package typedmemorystore

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// rebuildAdmissionObservations reconstructs every snapshot observation from
// the exact committed graph revision held by an immediate transaction. The
// supplied observations identify only the addressed entity, alias, assertion,
// context, and change ordinal. Their claimed state and basis are never trusted.
func rebuildAdmissionObservations(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	expected []typedmemory.AdmissionSnapshotObservation,
) ([]typedmemory.AdmissionSnapshotObservation, error) {
	if ctx == nil {
		return nil, fmt.Errorf("rebuild typed-memory admission observations: context is required")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, fmt.Errorf("rebuild typed-memory admission observations: %w", err)
	}
	if revision.Value() > mathMaxSQLiteRevision {
		return nil, ErrRevisionOverflow
	}
	resolutionBasis, err := snapshotResolutionBasis(project, revision)
	if err != nil {
		return nil, err
	}
	assertionRule, err := snapshotAssertionRule(project, revision)
	if err != nil {
		return nil, err
	}
	observations := make([]typedmemory.AdmissionSnapshotObservation, 0, len(expected))
	for _, value := range expected {
		observation, err := rebuildAdmissionObservation(
			ctx,
			transaction,
			project,
			revision,
			resolutionBasis,
			assertionRule,
			value,
		)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func snapshotResolutionBasis(
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
) (typedmemory.ResolutionBasisRef, error) {
	revisionText := strconv.FormatUint(revision.Value(), 10)
	ref := derivedRef(
		"typed-memory-snapshot-resolution-basis.v1",
		project.String(),
		revisionText,
	)
	basis, err := typedmemory.NewResolutionBasisRef(ref)
	if err != nil {
		return typedmemory.ResolutionBasisRef{}, fmt.Errorf(
			"build typed-memory snapshot resolution basis: %w",
			err,
		)
	}
	return basis, nil
}

func snapshotAssertionRule(
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
) (typedmemory.RuleRef, error) {
	revisionText := strconv.FormatUint(revision.Value(), 10)
	ref := derivedRef(
		"typed-memory-snapshot-assertion-rule.v1",
		project.String(),
		revisionText,
	)
	rule, err := typedmemory.NewRuleRef(ref)
	if err != nil {
		return typedmemory.RuleRef{}, fmt.Errorf(
			"build typed-memory snapshot assertion rule: %w",
			err,
		)
	}
	return rule, nil
}

func rebuildAdmissionObservation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	resolutionBasis typedmemory.ResolutionBasisRef,
	assertionRule typedmemory.RuleRef,
	expected typedmemory.AdmissionSnapshotObservation,
) (typedmemory.AdmissionSnapshotObservation, error) {
	switch value := expected.(type) {
	case typedmemory.EntityAbsentObservation:
		resolution := value.Resolution()
		return rebuildEntityObservation(
			ctx,
			transaction,
			project,
			revision,
			value.ChangeOrdinal(),
			resolution.Entity(),
			resolution.Context(),
			resolutionBasis,
		)
	case typedmemory.EntityExactObservation:
		resolution := value.Resolution()
		return rebuildEntityObservation(
			ctx,
			transaction,
			project,
			revision,
			value.ChangeOrdinal(),
			resolution.Entity(),
			resolution.Context(),
			resolutionBasis,
		)
	case typedmemory.AliasUnboundObservation:
		resolution := value.Resolution()
		return rebuildAliasObservation(
			ctx,
			transaction,
			project,
			revision,
			value.ChangeOrdinal(),
			resolution.Alias(),
			resolution.Context(),
			resolutionBasis,
		)
	case typedmemory.AliasBoundObservation:
		resolution := value.Resolution()
		return rebuildAliasObservation(
			ctx,
			transaction,
			project,
			revision,
			value.ChangeOrdinal(),
			resolution.Alias(),
			resolution.Context(),
			resolutionBasis,
		)
	case typedmemory.AssertionAbsentObservation:
		state := value.State()
		return rebuildAssertionObservation(
			ctx,
			transaction,
			project,
			revision,
			value.ChangeOrdinal(),
			state.Assertion(),
			assertionRule,
		)
	case typedmemory.AssertionActiveObservation:
		state := value.State()
		return rebuildAssertionObservation(
			ctx,
			transaction,
			project,
			revision,
			value.ChangeOrdinal(),
			state.Assertion(),
			assertionRule,
		)
	default:
		return nil, fmt.Errorf(
			"%w: unsupported admission observation %T",
			ErrInvalidAdmissionBatch,
			expected,
		)
	}
}

func rebuildEntityObservation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	changeOrdinal uint64,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
	basis typedmemory.ResolutionBasisRef,
) (typedmemory.AdmissionSnapshotObservation, error) {
	sqliteRevision, exact := sqliteIntegerFromUint64(revision.Value())
	if !exact {
		return nil, fmt.Errorf(
			"%w: entity observation revision exceeds SQLite INTEGER",
			ErrRevalidationRejected,
		)
	}
	var matches int64
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
			AND entity_context.declared_revision <= ?
			AND event.graph_revision <= ?`,
		[]any{
			project.String(),
			entity.String(),
			contextRef.String(),
			sqliteRevision,
			sqliteRevision,
		},
		[]any{&matches},
	)
	if err != nil {
		return nil, fmt.Errorf("rebuild entity admission observation: %w", err)
	}
	if matches > 1 {
		return nil, fmt.Errorf(
			"%w: entity %s has %d exact context declarations at revision %d",
			ErrRevalidationRejected,
			entity.String(),
			matches,
			revision.Value(),
		)
	}
	if matches == 1 {
		resolution, err := typedmemory.NewExactEntityResolution(entity, contextRef, basis)
		if err != nil {
			return nil, err
		}
		return typedmemory.NewEntityExactObservation(changeOrdinal, resolution)
	}
	resolution, err := typedmemory.NewAbsentEntityResolution(entity, contextRef, basis)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewEntityAbsentObservation(changeOrdinal, resolution)
}

func rebuildAliasObservation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	changeOrdinal uint64,
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
	basis typedmemory.ResolutionBasisRef,
) (typedmemory.AdmissionSnapshotObservation, error) {
	sqliteRevision, exact := sqliteIntegerFromUint64(revision.Value())
	if !exact {
		return nil, fmt.Errorf(
			"%w: alias observation revision exceeds SQLite INTEGER",
			ErrRevalidationRejected,
		)
	}
	var matches int64
	var entityText sql.NullString
	err := transaction.ScanOne(
		ctx,
		`WITH visible_alias_changes AS (
			SELECT
				alias_change.alias_change_ref,
				alias_change.supersedes_alias_change_ref,
				alias_change.change_kind,
				alias_change.alias,
				alias_change.replacement_alias,
				alias_change.entity_id
			FROM typed_memory_alias_changes alias_change
			JOIN typed_memory_graph_events event
				ON event.project_id = alias_change.project_id
				AND event.event_ref = alias_change.event_ref
			JOIN typed_memory_graph_commits commit_record
				ON commit_record.project_id = event.project_id
				AND commit_record.event_ref = event.event_ref
			WHERE alias_change.project_id = ?
				AND alias_change.bounded_context_ref = ?
				AND event.graph_revision <= ?
		), active_alias_bindings AS (
			SELECT
				CASE current.change_kind
					WHEN 'admit_alias' THEN current.alias
					ELSE current.replacement_alias
				END AS effective_alias,
				current.entity_id
			FROM visible_alias_changes current
			WHERE NOT EXISTS (
				SELECT 1
				FROM visible_alias_changes successor
				WHERE successor.supersedes_alias_change_ref = current.alias_change_ref
			)
		)
		SELECT COUNT(*), MIN(entity_id)
		FROM active_alias_bindings
		WHERE effective_alias = ?`,
		[]any{
			project.String(),
			contextRef.String(),
			sqliteRevision,
			alias.String(),
		},
		[]any{&matches, &entityText},
	)
	if err != nil {
		return nil, fmt.Errorf("rebuild alias admission observation: %w", err)
	}
	if matches > 1 {
		return nil, fmt.Errorf(
			"%w: alias %s has %d active bindings at revision %d",
			ErrRevalidationRejected,
			alias.String(),
			matches,
			revision.Value(),
		)
	}
	if matches == 1 && !entityText.Valid {
		return nil, fmt.Errorf(
			"%w: alias %s has an active binding without an entity",
			ErrRevalidationRejected,
			alias.String(),
		)
	}
	if matches == 1 {
		entity, err := typedmemory.NewEntityID(entityText.String)
		if err != nil {
			return nil, fmt.Errorf("parse active alias entity: %w", err)
		}
		resolution, err := typedmemory.NewBoundAliasResolution(alias, entity, contextRef, basis)
		if err != nil {
			return nil, err
		}
		return typedmemory.NewAliasBoundObservation(changeOrdinal, resolution)
	}
	resolution, err := typedmemory.NewUnboundAliasResolution(alias, contextRef, basis)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewAliasUnboundObservation(changeOrdinal, resolution)
}

func rebuildAssertionObservation(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	changeOrdinal uint64,
	assertion typedmemory.AssertionID,
	rule typedmemory.RuleRef,
) (typedmemory.AdmissionSnapshotObservation, error) {
	sqliteRevision, exact := sqliteIntegerFromUint64(revision.Value())
	if !exact {
		return nil, fmt.Errorf(
			"%w: assertion observation revision exceeds SQLite INTEGER",
			ErrRevalidationRejected,
		)
	}
	var assertionCount int64
	var assertionWriterMismatchCount int64
	var retractionCount int64
	var retractionWriterMismatchCount int64
	err := transaction.ScanOne(
		ctx,
		`WITH assertion_origins AS (
			SELECT
				'legacy' AS storage_lane,
				generation.writer_generation,
				generation.provenance_kind
			FROM typed_memory_relation_instances relation
			JOIN typed_memory_graph_events relation_event
				ON relation_event.project_id = relation.project_id
				AND relation_event.event_ref = relation.event_ref
			JOIN typed_memory_graph_commits relation_commit
				ON relation_commit.project_id = relation_event.project_id
				AND relation_commit.event_ref = relation_event.event_ref
			LEFT JOIN typed_memory_event_writer_generations generation
				ON generation.project_id = relation_event.project_id
				AND generation.event_ref = relation_event.event_ref
			WHERE relation.project_id = ?
				AND relation.assertion_id = ?
				AND relation_event.graph_revision <= ?
			UNION ALL
			SELECT
				'v3' AS storage_lane,
				generation.writer_generation,
				generation.provenance_kind
			FROM typed_memory_relational_assertions_v3 assertion
			JOIN typed_memory_graph_events assertion_event
				ON assertion_event.project_id = assertion.project_id
				AND assertion_event.event_ref = assertion.event_ref
			JOIN typed_memory_graph_commits assertion_commit
				ON assertion_commit.project_id = assertion_event.project_id
				AND assertion_commit.event_ref = assertion_event.event_ref
			LEFT JOIN typed_memory_event_writer_generations generation
				ON generation.project_id = assertion_event.project_id
				AND generation.event_ref = assertion_event.event_ref
			WHERE assertion.project_id = ?
				AND assertion.assertion_id = ?
				AND assertion_event.graph_revision <= ?
		), assertion_origin_counts AS (
			SELECT
				COUNT(*) AS assertion_count,
				COALESCE(SUM(CASE
					WHEN storage_lane = 'legacy'
						AND writer_generation = 46
						AND provenance_kind = 'writer_v46' THEN 0
					WHEN storage_lane = 'v3'
						AND writer_generation = 53
						AND provenance_kind = 'writer_v53' THEN 0
					WHEN storage_lane = 'v3'
						AND writer_generation = 54
						AND provenance_kind = 'writer_v54' THEN 0
					ELSE 1
				END), 0) AS writer_mismatch_count
			FROM assertion_origins
		), retractions AS (
			SELECT
				generation.writer_generation,
				generation.provenance_kind
			FROM typed_memory_assertion_retractions retraction
			JOIN typed_memory_graph_events retraction_event
				ON retraction_event.project_id = retraction.project_id
				AND retraction_event.event_ref = retraction.event_ref
			JOIN typed_memory_graph_commits retraction_commit
				ON retraction_commit.project_id = retraction_event.project_id
				AND retraction_commit.event_ref = retraction_event.event_ref
			LEFT JOIN typed_memory_event_writer_generations generation
				ON generation.project_id = retraction_event.project_id
				AND generation.event_ref = retraction_event.event_ref
			WHERE retraction.project_id = ?
				AND retraction.assertion_id = ?
				AND retraction_event.graph_revision <= ?
		), retraction_counts AS (
			SELECT
				COUNT(*) AS retraction_count,
				COALESCE(SUM(CASE
					WHEN writer_generation = 46
						AND provenance_kind = 'writer_v46' THEN 0
					WHEN writer_generation = 53
						AND provenance_kind = 'writer_v53' THEN 0
					WHEN writer_generation = 54
						AND provenance_kind = 'writer_v54' THEN 0
					ELSE 1
				END), 0) AS writer_mismatch_count
			FROM retractions
		)
		SELECT
			assertion_origin_counts.assertion_count,
			assertion_origin_counts.writer_mismatch_count,
			retraction_counts.retraction_count,
			retraction_counts.writer_mismatch_count
		FROM assertion_origin_counts
		CROSS JOIN retraction_counts`,
		[]any{
			project.String(),
			assertion.String(),
			sqliteRevision,
			project.String(),
			assertion.String(),
			sqliteRevision,
			project.String(),
			assertion.String(),
			sqliteRevision,
		},
		[]any{
			&assertionCount,
			&assertionWriterMismatchCount,
			&retractionCount,
			&retractionWriterMismatchCount,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("rebuild assertion admission observation: %w", err)
	}
	if assertionWriterMismatchCount != 0 || retractionWriterMismatchCount != 0 {
		return nil, storedAdmissionIntegrity(
			fmt.Sprintf(
				"assertion %s crosses its sealed writer-generation lane at revision %d",
				assertion.String(),
				revision.Value(),
			),
			nil,
		)
	}
	// Assertion activity is identity state only. The v3 row keeps its explicit
	// modality and provenance; this union neither filters by modality nor
	// reinterprets an affirmed assertion as an obtaining relation occurrence.
	if assertionCount == 0 && retractionCount == 0 {
		state, err := typedmemory.NewAbsentAssertionState(assertion, rule)
		if err != nil {
			return nil, err
		}
		return typedmemory.NewAssertionAbsentObservation(changeOrdinal, state)
	}
	if assertionCount == 1 && retractionCount == 0 {
		state, err := typedmemory.NewActiveAssertion(assertion, rule)
		if err != nil {
			return nil, err
		}
		return typedmemory.NewAssertionActiveObservation(changeOrdinal, state)
	}
	if assertionCount == 1 && retractionCount == 1 {
		return nil, fmt.Errorf(
			"%w: assertion %s is retracted at revision %d",
			ErrRevalidationRejected,
			assertion.String(),
			revision.Value(),
		)
	}
	return nil, fmt.Errorf(
		"%w: assertion %s has an inconsistent history at revision %d (assertion_origins=%d, retractions=%d)",
		ErrRevalidationRejected,
		assertion.String(),
		revision.Value(),
		assertionCount,
		retractionCount,
	)
}
