package decisionbinding

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// stagePreparedDecisionArtifactInTransaction materializes the exact artifact
// projection already sealed inside durable DecisionBindingContent. It neither
// accepts a caller-built Artifact nor exposes the PreparedDecision timestamp
// adapter outside this institutional-effect boundary.
func stagePreparedDecisionArtifactInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	content DecisionBindingContent,
	occurredAt time.Time,
) error {
	if ctx == nil || transaction == nil {
		return fmt.Errorf("decision artifact staging requires a context and transaction")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	contentRef, contentRefOK := content.ContentRef()
	prepared, preparedOK := content.PreparedDecision()
	links, linksOK := prepared.Links()
	affectedFiles, affectedFilesOK := prepared.AffectedFiles()
	occurredAt = canonicalDecisionBindingTime(occurredAt)
	complete := contentRefOK && preparedOK && linksOK && affectedFilesOK
	complete = complete && !occurredAt.IsZero()
	if !complete {
		return fmt.Errorf("decision artifact staging requires exact prepared content and SpeechAct time")
	}
	instant := formatDecisionBindingTime(occurredAt)
	artifactResult, err := transaction.Execute(
		ctx,
		`INSERT INTO artifacts (
			id, kind, version, status, context, mode, title, content,
			valid_until, created_at, updated_at, search_keywords, structured_data
		)
		SELECT
			json_extract(canonical_json, '$.prepared_decision.artifact.id'),
			json_extract(canonical_json, '$.prepared_decision.artifact.kind'),
			json_extract(canonical_json, '$.prepared_decision.artifact.version'),
			json_extract(canonical_json, '$.prepared_decision.artifact.status'),
			json_extract(canonical_json, '$.prepared_decision.artifact.context'),
			json_extract(canonical_json, '$.prepared_decision.artifact.mode'),
			json_extract(canonical_json, '$.prepared_decision.artifact.title'),
			json_extract(canonical_json, '$.prepared_decision.artifact.body'),
			json_extract(canonical_json, '$.prepared_decision.artifact.valid_until'),
			?, ?,
			json_extract(canonical_json, '$.prepared_decision.artifact.search_keywords'),
			json_extract(canonical_json, '$.prepared_decision.artifact.structured_data')
		FROM decision_binding_contents
		WHERE decision_content_ref = ?`,
		[]any{instant, instant, contentRef.String()},
	)
	if err != nil {
		return fmt.Errorf("stage exact DecisionRecord artifact: %w", err)
	}
	if err := requireAffectedRows(artifactResult, 1, "DecisionRecord artifact"); err != nil {
		return err
	}
	linkResult, err := transaction.Execute(
		ctx,
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at)
		SELECT
			content.decision_ref,
			json_extract(item.value, '$.ref'),
			json_extract(item.value, '$.type'),
			?
		FROM decision_binding_contents content
		JOIN json_each(content.canonical_json, '$.prepared_decision.links') item
		WHERE content.decision_content_ref = ?`,
		[]any{instant, contentRef.String()},
	)
	if err != nil {
		return fmt.Errorf("stage exact DecisionRecord links: %w", err)
	}
	if err := requireAffectedRows(linkResult, int64(len(links)), "DecisionRecord links"); err != nil {
		return err
	}
	affectedResult, err := transaction.Execute(
		ctx,
		`INSERT INTO affected_files (artifact_id, file_path, file_hash)
		SELECT
			content.decision_ref,
			json_extract(item.value, '$.path'),
			''
		FROM decision_binding_contents content
		JOIN json_each(content.canonical_json, '$.prepared_decision.affected_files') item
		WHERE content.decision_content_ref = ?`,
		[]any{contentRef.String()},
	)
	if err != nil {
		return fmt.Errorf("stage exact DecisionRecord affected files: %w", err)
	}
	if err := requireAffectedRows(
		affectedResult,
		int64(len(affectedFiles)),
		"DecisionRecord affected files",
	); err != nil {
		return err
	}
	return verifyStagedDecisionArtifact(
		ctx,
		transaction,
		contentRef.String(),
		instant,
	)
}

type affectedRowsResult interface {
	RowsAffected() (int64, error)
}

func requireAffectedRows(
	result affectedRowsResult,
	want int64,
	subject string,
) error {
	if result == nil {
		return fmt.Errorf("%s staging returned no SQL result", subject)
	}
	got, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count staged %s: %w", subject, err)
	}
	if got != want {
		return fmt.Errorf("staged %s count = %d, want %d", subject, got, want)
	}
	return nil
}

func verifyStagedDecisionArtifact(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	contentRef string,
	instant string,
) error {
	exactCount := 0
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM decision_binding_contents content
		JOIN artifacts artifact ON artifact.id = content.decision_ref
		WHERE content.decision_content_ref = ?
		AND artifact.created_at = ?
		AND artifact.updated_at = ?
		AND artifact.kind = json_extract(content.canonical_json, '$.prepared_decision.artifact.kind')
		AND artifact.version = json_extract(content.canonical_json, '$.prepared_decision.artifact.version')
		AND artifact.status = json_extract(content.canonical_json, '$.prepared_decision.artifact.status')
		AND artifact.context IS json_extract(content.canonical_json, '$.prepared_decision.artifact.context')
		AND artifact.mode IS json_extract(content.canonical_json, '$.prepared_decision.artifact.mode')
		AND artifact.title = json_extract(content.canonical_json, '$.prepared_decision.artifact.title')
		AND artifact.content = json_extract(content.canonical_json, '$.prepared_decision.artifact.body')
		AND artifact.valid_until IS json_extract(content.canonical_json, '$.prepared_decision.artifact.valid_until')
		AND COALESCE(artifact.search_keywords, '') = json_extract(content.canonical_json, '$.prepared_decision.artifact.search_keywords')
		AND json(artifact.structured_data) = json(json_extract(content.canonical_json, '$.prepared_decision.artifact.structured_data'))
		AND (SELECT COUNT(*) FROM artifact_links link WHERE link.source_id = artifact.id)
			= json_array_length(content.canonical_json, '$.prepared_decision.links')
		AND NOT EXISTS (
			SELECT 1 FROM json_each(content.canonical_json, '$.prepared_decision.links') item
			WHERE NOT EXISTS (
				SELECT 1 FROM artifact_links link
				WHERE link.source_id = artifact.id
				AND link.target_id = json_extract(item.value, '$.ref')
				AND link.link_type = json_extract(item.value, '$.type')
			)
		)
		AND (SELECT COUNT(*) FROM affected_files file WHERE file.artifact_id = artifact.id)
			= json_array_length(content.canonical_json, '$.prepared_decision.affected_files')
		AND NOT EXISTS (
			SELECT 1 FROM json_each(content.canonical_json, '$.prepared_decision.affected_files') item
			WHERE NOT EXISTS (
				SELECT 1 FROM affected_files file
				WHERE file.artifact_id = artifact.id
				AND file.file_path = json_extract(item.value, '$.path')
				AND COALESCE(file.file_hash, '') = ''
			)
		)`,
		[]any{contentRef, instant, instant},
		[]any{&exactCount},
	)
	if err != nil {
		return fmt.Errorf("strictly reread staged DecisionRecord artifact: %w", err)
	}
	if exactCount != 1 {
		return fmt.Errorf("staged DecisionRecord artifact differs from exact reviewed content")
	}
	return nil
}
