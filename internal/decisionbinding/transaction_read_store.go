package decisionbinding

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// decisionTransactionReadStore redirects the two read operations used by
// artifact.PrepareDecision onto the caller-owned BEGIN IMMEDIATE connection.
// The embedded store supplies the wider interface shape only; preparation's
// source-pin adapter calls Get and ListByKind exclusively.
type decisionTransactionReadStore struct {
	artifact.ArtifactStore
	transaction *sqlitetransaction.Transaction
}

func newDecisionTransactionReadStore(
	store artifact.ArtifactStore,
	transaction *sqlitetransaction.Transaction,
) (*decisionTransactionReadStore, error) {
	if store == nil || transaction == nil {
		return nil, fmt.Errorf("decision transaction read store requires exact dependencies")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, err
	}
	return &decisionTransactionReadStore{
		ArtifactStore: store,
		transaction:   transaction,
	}, nil
}

func (store *decisionTransactionReadStore) Get(
	ctx context.Context,
	id string,
) (*artifact.Artifact, error) {
	if store == nil || store.transaction == nil || ctx == nil {
		return nil, fmt.Errorf("transaction-local artifact get requires an active context")
	}
	row := transactionArtifactRow{}
	err := store.transaction.ScanOne(
		ctx,
		`SELECT id, kind, version, status, context, mode, title, content,
			valid_until, created_at, updated_at,
			COALESCE(search_keywords, ''), COALESCE(structured_data, '')
		 FROM artifacts WHERE id = ?`,
		[]any{id},
		row.scanTargetsWithExtendedFields(),
	)
	if err != nil {
		return nil, fmt.Errorf("get artifact %s: %w", id, err)
	}
	value, err := row.artifact(true)
	if err != nil {
		return nil, fmt.Errorf("decode artifact %s: %w", id, err)
	}
	links, err := store.getLinks(ctx, id)
	if err != nil {
		return nil, err
	}
	value.Meta.Links = links
	return value, nil
}

func (store *decisionTransactionReadStore) ListByKind(
	ctx context.Context,
	kind artifact.Kind,
	limit int,
) ([]*artifact.Artifact, error) {
	if store == nil || store.transaction == nil || ctx == nil {
		return nil, fmt.Errorf("transaction-local artifact list requires an active context")
	}
	if limit < 0 {
		return nil, fmt.Errorf("artifact list limit must not be negative")
	}
	result := make([]*artifact.Artifact, 0)
	for offset := 0; limit == 0 || offset < limit; offset++ {
		value, found, err := store.artifactByKindAtOffset(ctx, kind, offset)
		if err != nil {
			return nil, err
		}
		if !found {
			return result, nil
		}
		result = append(result, value)
	}
	return result, nil
}

func (store *decisionTransactionReadStore) artifactByKindAtOffset(
	ctx context.Context,
	kind artifact.Kind,
	offset int,
) (*artifact.Artifact, bool, error) {
	statement := `SELECT id, kind, version, status, context, mode, title, content,
		valid_until, created_at, updated_at
		FROM artifacts
		WHERE (? = '' OR kind = ?)
		ORDER BY created_at DESC
		LIMIT 1 OFFSET ?`
	row := transactionArtifactRow{}
	err := store.transaction.ScanOne(
		ctx,
		statement,
		[]any{string(kind), string(kind), offset},
		row.scanTargets(),
	)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("list artifacts by kind at offset %d: %w", offset, err)
	}
	value, err := row.artifact(false)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (store *decisionTransactionReadStore) getLinks(
	ctx context.Context,
	artifactID string,
) ([]artifact.Link, error) {
	result := make([]artifact.Link, 0)
	for offset := 0; ; offset++ {
		link := artifact.Link{}
		err := store.transaction.ScanOne(
			ctx,
			`SELECT target_id, link_type
			 FROM artifact_links
			 WHERE source_id = ?
			 ORDER BY target_id, link_type
			 LIMIT 1 OFFSET ?`,
			[]any{artifactID, offset},
			[]any{&link.Ref, &link.Type},
		)
		if err == sql.ErrNoRows {
			return result, nil
		}
		if err != nil {
			return nil, fmt.Errorf("get artifact %s links: %w", artifactID, err)
		}
		result = append(result, link)
	}
}

type transactionArtifactRow struct {
	id             string
	kind           string
	version        int
	status         string
	context        string
	mode           string
	title          string
	body           string
	validUntil     string
	createdAt      string
	updatedAt      string
	searchKeywords string
	structuredData string
}

func (row *transactionArtifactRow) scanTargets() []any {
	return []any{
		&row.id,
		&row.kind,
		&row.version,
		&row.status,
		&row.context,
		&row.mode,
		&row.title,
		&row.body,
		&row.validUntil,
		&row.createdAt,
		&row.updatedAt,
	}
}

func (row *transactionArtifactRow) scanTargetsWithExtendedFields() []any {
	result := row.scanTargets()
	result = append(result, &row.searchKeywords)
	result = append(result, &row.structuredData)
	return result
}

func (row transactionArtifactRow) artifact(
	includeExtendedFields bool,
) (*artifact.Artifact, error) {
	kind, err := artifact.ParseKind(row.kind)
	if err != nil {
		return nil, err
	}
	status, err := artifact.ParseStatus(row.status)
	if err != nil {
		return nil, err
	}
	mode, err := artifact.ParseMode(row.mode)
	if err != nil {
		return nil, err
	}
	createdAt, err := time.Parse(time.RFC3339, row.createdAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := time.Parse(time.RFC3339, row.updatedAt)
	if err != nil {
		return nil, err
	}
	value := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         row.id,
			Kind:       kind,
			Version:    row.version,
			Status:     status,
			Context:    row.context,
			Mode:       mode,
			Title:      row.title,
			ValidUntil: row.validUntil,
			CreatedAt:  createdAt,
			UpdatedAt:  updatedAt,
		},
		Body: row.body,
	}
	if includeExtendedFields {
		value.SearchKeywords = row.searchKeywords
		value.StructuredData = row.structuredData
	}
	return value, nil
}
