package identityreconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type storedRedirect struct {
	Kind              string `json:"kind"`
	Target            string `json:"target"`
	ReconciliationRef string `json:"reconciliation_ref"`
	Revision          int64  `json:"revision"`
}

// ResolveHistorical preserves the queried historical EntityID. A reviewed
// merge follows only exact durable redirects; a reviewed split returns the
// complete candidate set and deliberately refuses to guess a successor.
func (service *SQLiteService) ResolveHistorical(
	ctx context.Context,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (HistoricalResolution, error) {
	if ctx == nil {
		return nil, fmt.Errorf("resolve historical identity: context is required")
	}
	if service == nil || service.database == nil || service.schema == nil {
		return nil, ErrDatabaseRequired
	}
	if err := service.schema.RequireCompatible(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaUnavailable, err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, service.database)
	if err != nil {
		return nil, fmt.Errorf("begin historical identity resolution: %w", err)
	}
	resolution, resolveErr := resolveHistoricalTx(
		ctx,
		transaction,
		project,
		entity,
		contextRef,
	)
	finish := transaction.Rollback(ctx)
	if resolveErr != nil {
		return nil, errors.Join(resolveErr, finish.Err())
	}
	if !finish.Succeeded() {
		return nil, finish.Err()
	}
	return resolution, nil
}

func resolveHistoricalTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	requested typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (HistoricalResolution, error) {
	var headRevision int64
	err := transaction.ScanOne(
		ctx,
		`SELECT graph_revision FROM typed_memory_graph_heads WHERE project_id = ?`,
		[]any{project.String()},
		[]any{&headRevision},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return IdentityAbsent{entity: requested, context: contextRef}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load identity-resolution graph head: %w", err)
	}
	present, err := exactEntityContextExists(
		ctx,
		transaction,
		project,
		requested,
		contextRef,
		headRevision,
	)
	if err != nil {
		return nil, err
	}
	if !present {
		return IdentityAbsent{entity: requested, context: contextRef}, nil
	}
	current := requested
	history := make([]string, 0)
	visited := map[string]struct{}{requested.String(): {}}
	for {
		redirects, err := loadCommittedRedirects(
			ctx,
			transaction,
			project,
			current,
			contextRef,
			headRevision,
		)
		if err != nil {
			return nil, err
		}
		if len(redirects) == 0 {
			if current == requested {
				return CurrentIdentity{entity: requested, context: contextRef}, nil
			}
			return MergedIdentity{
				requested: requested,
				current:   current,
				context:   contextRef,
				history:   append([]string(nil), history...),
			}, nil
		}
		kind := redirects[0].Kind
		reconciliationRef := redirects[0].ReconciliationRef
		revision := redirects[0].Revision
		for _, redirect := range redirects {
			if redirect.Kind != kind ||
				redirect.ReconciliationRef != reconciliationRef ||
				redirect.Revision != revision {
				return nil, fmt.Errorf("%w: one historical identity has conflicting redirect events", ErrStoredIntegrity)
			}
		}
		switch kind {
		case "merge_redirect":
			if len(redirects) != 1 {
				return nil, fmt.Errorf("%w: merge redirect has %d targets", ErrStoredIntegrity, len(redirects))
			}
			target, err := typedmemory.NewEntityID(redirects[0].Target)
			if err != nil {
				return nil, fmt.Errorf("%w: parse merge target: %v", ErrStoredIntegrity, err)
			}
			if _, seen := visited[target.String()]; seen {
				return nil, fmt.Errorf("%w: identity redirect cycle", ErrStoredIntegrity)
			}
			visited[target.String()] = struct{}{}
			history = append(history, reconciliationRef)
			current = target
		case "split_candidate":
			if len(redirects) < 2 {
				return nil, fmt.Errorf("%w: reviewed split has fewer than two candidates", ErrStoredIntegrity)
			}
			candidates := make([]typedmemory.EntityID, 0, len(redirects))
			seen := make(map[string]struct{}, len(redirects))
			for _, redirect := range redirects {
				candidate, err := typedmemory.NewEntityID(redirect.Target)
				if err != nil {
					return nil, fmt.Errorf("%w: parse split candidate: %v", ErrStoredIntegrity, err)
				}
				if _, duplicate := seen[candidate.String()]; duplicate {
					return nil, fmt.Errorf("%w: reviewed split repeats a candidate", ErrStoredIntegrity)
				}
				seen[candidate.String()] = struct{}{}
				candidates = append(candidates, candidate)
			}
			return SplitIdentityCandidates{
				source:     requested,
				candidates: candidates,
				context:    contextRef,
				history:    append(append([]string(nil), history...), reconciliationRef),
			}, nil
		default:
			return nil, fmt.Errorf("%w: unknown identity redirect kind %q", ErrStoredIntegrity, kind)
		}
	}
}

func exactEntityContextExists(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	headRevision int64,
) (bool, error) {
	var count int64
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*)
		FROM typed_memory_entity_contexts entity_context
		JOIN typed_memory_graph_events event
			ON event.project_id = entity_context.project_id
			AND event.event_ref = entity_context.declared_event_ref
		JOIN typed_memory_graph_commits commit_record
			ON commit_record.project_id = event.project_id
			AND commit_record.event_ref = event.event_ref
		WHERE entity_context.project_id = ?
			AND entity_context.entity_id = ?
			AND entity_context.bounded_context_ref = ?
			AND event.graph_revision <= ?`,
		[]any{project.String(), entity.String(), contextRef.String(), headRevision},
		[]any{&count},
	)
	if err != nil {
		return false, fmt.Errorf("inspect historical identity: %w", err)
	}
	if count > 1 {
		return false, fmt.Errorf("%w: repeated entity/context declaration", ErrStoredIntegrity)
	}
	return count == 1, nil
}

func loadCommittedRedirects(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	headRevision int64,
) ([]storedRedirect, error) {
	var encoded string
	err := transaction.ScanOne(
		ctx,
		`SELECT COALESCE(json_group_array(json_object(
			'kind', resolution_kind,
			'target', target_entity_id,
			'reconciliation_ref', reconciliation_ref,
			'revision', graph_revision
		)), '[]')
		FROM (
			SELECT redirect.resolution_kind, redirect.target_entity_id,
				reconciliation.reconciliation_ref, event.graph_revision,
				redirect.redirect_ordinal
			FROM typed_memory_identity_redirects redirect
			JOIN typed_memory_identity_reconciliations reconciliation
				ON reconciliation.project_id = redirect.project_id
				AND reconciliation.event_ref = redirect.event_ref
			JOIN typed_memory_graph_events event
				ON event.project_id = redirect.project_id
				AND event.event_ref = redirect.event_ref
			JOIN typed_memory_graph_commits commit_record
				ON commit_record.project_id = event.project_id
				AND commit_record.event_ref = event.event_ref
			WHERE redirect.project_id = ?
				AND redirect.bounded_context_ref = ?
				AND redirect.source_entity_id = ?
				AND event.graph_revision <= ?
			ORDER BY event.graph_revision, redirect.redirect_ordinal
		)`,
		[]any{project.String(), contextRef.String(), entity.String(), headRevision},
		[]any{&encoded},
	)
	if err != nil {
		return nil, fmt.Errorf("load reviewed identity redirects: %w", err)
	}
	redirects := []storedRedirect{}
	if err := json.Unmarshal([]byte(encoded), &redirects); err != nil {
		return nil, fmt.Errorf("%w: decode reviewed identity redirects: %v", ErrStoredIntegrity, err)
	}
	return redirects, nil
}
