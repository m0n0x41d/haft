package typedmemorystore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func loadExactPersistedEntityUniverseTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
) (ExactPersistedEntityUniverse, error) {
	if transaction == nil {
		return ExactPersistedEntityUniverse{}, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return ExactPersistedEntityUniverse{}, err
	}
	sqliteRevision, exact := sqliteIntegerFromUint64(revision.Value())
	if !exact {
		return ExactPersistedEntityUniverse{}, ErrRevisionOverflow
	}
	var encoded string
	err := transaction.ScanOne(
		ctx,
		`SELECT COALESCE(json_group_array(entity_id), '[]')
		FROM (
			SELECT entity_context.entity_id
			FROM typed_memory_entity_contexts entity_context
			JOIN typed_memory_graph_events event
				ON event.project_id = entity_context.project_id
				AND event.event_ref = entity_context.declared_event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE entity_context.project_id = ?
				AND entity_context.bounded_context_ref = ?
				AND entity_context.declared_revision <= ?
				AND event.graph_revision <= ?
			ORDER BY entity_context.entity_id
		)`,
		[]any{
			project.String(),
			contextRef.String(),
			sqliteRevision,
			sqliteRevision,
		},
		[]any{&encoded},
	)
	if err != nil {
		return ExactPersistedEntityUniverse{}, fmt.Errorf(
			"load persisted entity universe: %w",
			err,
		)
	}
	rows := []string{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return ExactPersistedEntityUniverse{}, storedAdmissionIntegrity(
			"decode persisted entity universe",
			err,
		)
	}
	members := make([]typedmemory.EntityID, 0, len(rows))
	for _, raw := range rows {
		entity, err := typedmemory.NewEntityID(raw)
		if err != nil {
			return ExactPersistedEntityUniverse{}, storedAdmissionIntegrity(
				"persisted entity universe contains a malformed EntityID",
				err,
			)
		}
		members = append(members, entity)
	}
	universe, err := newExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		members,
	)
	if err != nil {
		return ExactPersistedEntityUniverse{}, storedAdmissionIntegrity(
			"construct persisted entity universe",
			err,
		)
	}
	return universe, nil
}

func exactPersistedEntityUniverseFromSnapshot(
	project projectledger.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
	entityContexts map[entityContextKey]struct{},
) (ExactPersistedEntityUniverse, error) {
	members := make([]typedmemory.EntityID, 0)
	for key := range entityContexts {
		if key.context == contextRef {
			members = append(members, key.entity)
		}
	}
	return newExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		members,
	)
}
