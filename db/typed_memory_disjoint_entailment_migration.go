package db

import (
	"bytes"
	"fmt"
	"strings"
)

const typedMemoryDisjointEntailmentSchemaVersion49 = 49

const typedMemoryDisjointEntailmentUsesTable49 = "typed_memory_relation_filler_disjoint_entailment_uses"

var typedMemoryDisjointEntailmentMigration49 = Migration{
	Version:     typedMemoryDisjointEntailmentSchemaVersion49,
	Description: "Add exact disjoint-entailment admission-use projection",
	Apply:       applyTypedMemoryDisjointEntailmentMigration49,
}

type typedMemoryDisjointEntailmentObject49 struct {
	kind string
	name string
	sql  string
}

func applyTypedMemoryDisjointEntailmentMigration49(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireTypedMemoryDisjointEntailmentSource49(tx); err != nil {
		return err
	}
	if err := requireAbsentTypedMemoryDisjointEntailmentFootprint49(tx); err != nil {
		return err
	}
	statements, err := typedMemoryDisjointEntailmentStatements49()
	if err != nil {
		return err
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install exact disjoint-entailment projection: %w", err)
	}
	if err := verifyTypedMemoryDisjointEntailmentFootprint49(tx); err != nil {
		return err
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify disjoint-entailment projection foreign keys: %w", err)
	}
	return nil
}

func requireTypedMemoryDisjointEntailmentSource49(
	tx MigrationTransaction,
) error {
	var maximumVersion int
	err := tx.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&maximumVersion)
	if err != nil {
		return fmt.Errorf("inspect disjoint-entailment source schema: %w", err)
	}
	if maximumVersion != 48 {
		return fmt.Errorf(
			"disjoint-entailment projection requires exact schema version 48, found %d",
			maximumVersion,
		)
	}
	if err := requireTypedMemoryWriterCapability46(tx); err != nil {
		return err
	}
	objects, err := typedMemoryDisjointEntailmentSourceObjects49()
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := requireExactTypedMemoryObject49(tx, object); err != nil {
			return err
		}
	}
	return nil
}

func requireTypedMemoryWriterCapability46(tx MigrationTransaction) error {
	var generation int
	var digest string
	var canonical []byte
	err := tx.QueryRow(
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_storage_capabilities
		WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		return fmt.Errorf("load exact v46 typed-memory writer capability: %w", err)
	}
	matches := generation == typedMemoryWriterGeneration46
	matches = matches && digest == typedMemoryWriterMarkerDigest46
	matches = matches && bytes.Equal(canonical, []byte(typedMemoryWriterMarkerBytes46))
	if !matches {
		return fmt.Errorf("disjoint-entailment projection requires the exact v46 writer capability")
	}
	return nil
}

func typedMemoryDisjointEntailmentSourceObjects49() (
	[]typedMemoryDisjointEntailmentObject49,
	error,
) {
	footprintView, err := typedMemoryEventMaterializationFootprintsView47()
	if err != nil {
		return nil, fmt.Errorf("derive exact v48 typed-memory footprint view: %w", err)
	}
	closureTrigger, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return nil, fmt.Errorf("derive exact v48 typed-memory closure trigger: %w", err)
	}
	return []typedMemoryDisjointEntailmentObject49{
		{
			kind: "view",
			name: "typed_memory_event_materialization_footprints_v46",
			sql:  footprintView,
		},
		{
			kind: "trigger",
			name: "typed_memory_commit_materialization_closures_v46_exact_footprint",
			sql:  closureTrigger,
		},
		{
			kind: "trigger",
			name: "typed_memory_graph_commits_exact_closure",
			sql:  typedMemoryGraphCommitExactClosureTrigger46(),
		},
	}, nil
}

func requireExactTypedMemoryObject49(
	tx MigrationTransaction,
	object typedMemoryDisjointEntailmentObject49,
) error {
	var actual string
	err := tx.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		object.kind,
		object.name,
	).Scan(&actual)
	if err != nil {
		return fmt.Errorf(
			"load exact v48 typed-memory %s %s: %w",
			object.kind,
			object.name,
			err,
		)
	}
	want := normalizeSQLiteDDL46(object.sql)
	got := normalizeSQLiteDDL46(actual)
	if got != want {
		return fmt.Errorf(
			"disjoint-entailment projection refused unknown %s %s",
			object.kind,
			object.name,
		)
	}
	return nil
}

func requireAbsentTypedMemoryDisjointEntailmentFootprint49(
	tx MigrationTransaction,
) error {
	objects := []typedMemoryDisjointEntailmentObject49{
		{kind: "table", name: typedMemoryDisjointEntailmentUsesTable49},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_exact_use",
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_open_event",
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_no_update",
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_no_delete",
		},
	}
	return requireAbsentTypedMemoryDisjointEntailmentObject49(tx, objects, 0)
}

func requireAbsentTypedMemoryDisjointEntailmentObject49(
	tx MigrationTransaction,
	objects []typedMemoryDisjointEntailmentObject49,
	index int,
) error {
	if index >= len(objects) {
		return nil
	}
	object := objects[index]
	var count int
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		object.kind,
		object.name,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf(
			"inspect v49 typed-memory %s %s: %w",
			object.kind,
			object.name,
			err,
		)
	}
	if count != 0 {
		return fmt.Errorf(
			"disjoint-entailment projection refused unknown partial v49 footprint: %s %s already exists",
			object.kind,
			object.name,
		)
	}
	return requireAbsentTypedMemoryDisjointEntailmentObject49(
		tx,
		objects,
		index+1,
	)
}

func typedMemoryDisjointEntailmentStatements49() ([]string, error) {
	footprintView, err := typedMemoryEventMaterializationFootprintsView49()
	if err != nil {
		return nil, err
	}
	closureTrigger, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return nil, fmt.Errorf("derive v49 typed-memory closure trigger: %w", err)
	}
	return []string{
		"DROP TRIGGER typed_memory_graph_commits_exact_closure",
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_exact_footprint",
		"DROP VIEW typed_memory_event_materialization_footprints_v46",
		typedMemoryDisjointEntailmentUsesTableStatement49(),
		footprintView,
		typedMemoryDisjointEntailmentExactUseTrigger49(),
		typedMemoryDisjointEntailmentOpenEventTrigger49(),
		typedMemoryDisjointEntailmentImmutableTrigger49("update"),
		typedMemoryDisjointEntailmentImmutableTrigger49("delete"),
		closureTrigger,
		typedMemoryGraphCommitExactClosureTrigger46(),
	}, nil
}

func typedMemoryDisjointEntailmentUsesTableStatement49() string {
	return `CREATE TABLE typed_memory_relation_filler_disjoint_entailment_uses (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		slot_ordinal INTEGER NOT NULL CHECK(slot_ordinal >= 0),
		filler_ordinal INTEGER NOT NULL CHECK(filler_ordinal >= 0),
		filler_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("filler_digest") + `),
		constraint_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("constraint_id") + `),
		constraint_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("constraint_digest") + `),
		canonical_constraint_bytes BLOB NOT NULL CHECK(length(canonical_constraint_bytes) > 0),
		matched_operand_kind_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("matched_operand_kind_id") + `),
		excluded_operand_kind_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("excluded_operand_kind_id") + `),
		counter_value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("counter_value_kind_ref") + `),
		counter_query_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("counter_query_digest") + `),
		canonical_counter_query_bytes BLOB NOT NULL CHECK(length(canonical_counter_query_bytes) > 0),
		supporting_evaluation_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("supporting_evaluation_ref") + `),
		use_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("use_digest") + `),
		canonical_use_bytes BLOB NOT NULL CHECK(length(canonical_use_bytes) > 0),
		PRIMARY KEY(
			project_id, event_ref, change_ordinal, slot_ordinal, filler_ordinal,
			constraint_id, counter_value_kind_ref, counter_query_digest
		),
		UNIQUE(
			project_id, event_ref, change_ordinal, slot_ordinal, filler_ordinal,
			constraint_id, excluded_operand_kind_id
		),
		FOREIGN KEY(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		) REFERENCES typed_memory_relation_fillers(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		),
		FOREIGN KEY(project_id, event_ref, supporting_evaluation_ref)
			REFERENCES typed_memory_memberof_evaluations(
				project_id, event_ref, evaluation_ref
			),
		CHECK(matched_operand_kind_id != excluded_operand_kind_id)
	) WITHOUT ROWID`
}

func typedMemoryEventMaterializationFootprintsView49() (string, error) {
	base, err := typedMemoryEventMaterializationFootprintsView47()
	if err != nil {
		return "", fmt.Errorf("derive v49 typed-memory footprint view source: %w", err)
	}
	oldCount := `(SELECT COUNT(*) FROM typed_memory_relation_filler_memberof_uses member_use
			WHERE member_use.project_id = event.project_id
				AND member_use.event_ref = event.event_ref) AS memberof_use_count,`
	newCount := `(SELECT COUNT(*) FROM typed_memory_relation_filler_memberof_uses member_use
			WHERE member_use.project_id = event.project_id
				AND member_use.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relation_filler_disjoint_entailment_uses entailment_use
			WHERE entailment_use.project_id = event.project_id
				AND entailment_use.event_ref = event.event_ref) AS memberof_use_count,`
	if strings.Count(base, oldCount) != 1 {
		return "", fmt.Errorf(
			"v49 footprint view cannot locate exact v48 member-use-count seam",
		)
	}
	return strings.Replace(base, oldCount, newCount, 1), nil
}

func typedMemoryDisjointEntailmentExactUseTrigger49() string {
	return `CREATE TRIGGER typed_memory_relation_filler_disjoint_entailment_uses_v49_exact_use
	BEFORE INSERT ON typed_memory_relation_filler_disjoint_entailment_uses
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_relation_fillers filler
		JOIN typed_memory_relation_instances relation
			ON relation.project_id = filler.project_id
			AND relation.event_ref = filler.event_ref
			AND relation.change_ordinal = filler.change_ordinal
			AND relation.assertion_id = filler.assertion_id
		JOIN typed_memory_reference_resolution_uses resolution_use
			ON resolution_use.project_id = filler.project_id
			AND resolution_use.event_ref = filler.event_ref
			AND resolution_use.change_ordinal = filler.change_ordinal
			AND resolution_use.assertion_id = filler.assertion_id
			AND resolution_use.slot_ordinal = filler.slot_ordinal
			AND resolution_use.filler_ordinal = filler.filler_ordinal
			AND resolution_use.filler_digest = filler.filler_digest
		JOIN typed_memory_relation_filler_memberof_uses required_use
			ON required_use.project_id = filler.project_id
			AND required_use.event_ref = filler.event_ref
			AND required_use.change_ordinal = filler.change_ordinal
			AND required_use.assertion_id = filler.assertion_id
			AND required_use.slot_ordinal = filler.slot_ordinal
			AND required_use.filler_ordinal = filler.filler_ordinal
			AND required_use.filler_digest = filler.filler_digest
			AND required_use.use_kind = 'required_member'
		JOIN typed_memory_memberof_evaluations supporting_evaluation
			ON supporting_evaluation.project_id = required_use.project_id
			AND supporting_evaluation.event_ref = required_use.event_ref
			AND supporting_evaluation.evaluation_ref = required_use.evaluation_ref
		WHERE filler.project_id = NEW.project_id
			AND filler.event_ref = NEW.event_ref
			AND filler.change_ordinal = NEW.change_ordinal
			AND filler.assertion_id = NEW.assertion_id
			AND filler.slot_ordinal = NEW.slot_ordinal
			AND filler.filler_ordinal = NEW.filler_ordinal
			AND filler.filler_digest = NEW.filler_digest
			AND filler.filler_kind = 'by_reference'
			AND required_use.evaluation_ref = NEW.supporting_evaluation_ref
			AND required_use.expected_judgement_kind = 'member'
			AND required_use.queried_value_kind_ref = filler.required_value_kind_ref
			AND supporting_evaluation.judgement_kind = 'member'
			AND supporting_evaluation.entity_id = filler.entity_id
			AND supporting_evaluation.value_kind_ref = filler.required_value_kind_ref
			AND supporting_evaluation.query_digest = required_use.query_digest
			AND supporting_evaluation.context_slice_ref = relation.context_slice_ref
			AND NEW.counter_value_kind_ref != filler.required_value_kind_ref
			AND NEW.matched_operand_kind_id != NEW.excluded_operand_kind_id
			AND (
				(resolution_use.resolution_kind = 'snapshot_reference'
					AND supporting_evaluation.evaluation_view_kind = 'persisted_snapshot')
				OR (resolution_use.resolution_kind = 'same_batch_declaration'
					AND supporting_evaluation.evaluation_view_kind = 'prospective_batch'
					AND supporting_evaluation.view_declaration_change_ordinal = resolution_use.declaration_change_ordinal
					AND supporting_evaluation.view_local_reference_kind_ref = resolution_use.local_reference_kind_ref
					AND supporting_evaluation.view_batch_local_ref = resolution_use.batch_local_ref
					AND supporting_evaluation.view_declaration_digest = resolution_use.declaration_digest
					AND supporting_evaluation.view_prefix_end_ordinal = filler.change_ordinal
					AND supporting_evaluation.view_ordered_candidate_prefix_digest = resolution_use.ordered_candidate_prefix_digest)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory disjoint entailment does not bind its exact filler and positive required-member support');
	END`
}

func typedMemoryDisjointEntailmentOpenEventTrigger49() string {
	return `CREATE TRIGGER typed_memory_relation_filler_disjoint_entailment_uses_v49_open_event
	BEFORE INSERT ON typed_memory_relation_filler_disjoint_entailment_uses
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory v49 disjoint entailment requires its exact open event');
	END`
}

func typedMemoryDisjointEntailmentImmutableTrigger49(operation string) string {
	return `CREATE TRIGGER typed_memory_relation_filler_disjoint_entailment_uses_v49_no_` + operation + `
	BEFORE ` + operation + ` ON typed_memory_relation_filler_disjoint_entailment_uses BEGIN
		SELECT RAISE(ABORT, 'typed-memory v49 disjoint-entailment history is append-only');
	END`
}

func verifyTypedMemoryDisjointEntailmentFootprint49(
	tx MigrationTransaction,
) error {
	footprintView, err := typedMemoryEventMaterializationFootprintsView49()
	if err != nil {
		return err
	}
	closureTrigger, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return fmt.Errorf("derive exact v49 typed-memory closure trigger: %w", err)
	}
	objects := []typedMemoryDisjointEntailmentObject49{
		{
			kind: "table",
			name: typedMemoryDisjointEntailmentUsesTable49,
			sql:  typedMemoryDisjointEntailmentUsesTableStatement49(),
		},
		{
			kind: "view",
			name: "typed_memory_event_materialization_footprints_v46",
			sql:  footprintView,
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_exact_use",
			sql:  typedMemoryDisjointEntailmentExactUseTrigger49(),
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_open_event",
			sql:  typedMemoryDisjointEntailmentOpenEventTrigger49(),
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_no_update",
			sql:  typedMemoryDisjointEntailmentImmutableTrigger49("update"),
		},
		{
			kind: "trigger",
			name: typedMemoryDisjointEntailmentUsesTable49 + "_v49_no_delete",
			sql:  typedMemoryDisjointEntailmentImmutableTrigger49("delete"),
		},
		{
			kind: "trigger",
			name: "typed_memory_commit_materialization_closures_v46_exact_footprint",
			sql:  closureTrigger,
		},
		{
			kind: "trigger",
			name: "typed_memory_graph_commits_exact_closure",
			sql:  typedMemoryGraphCommitExactClosureTrigger46(),
		},
	}
	for _, object := range objects {
		if err := requireExactTypedMemoryObject49(tx, object); err != nil {
			return err
		}
	}
	var entailmentCount int
	err = tx.QueryRow(
		"SELECT COUNT(*) FROM " + typedMemoryDisjointEntailmentUsesTable49,
	).Scan(&entailmentCount)
	if err != nil {
		return fmt.Errorf("count migrated disjoint-entailment rows: %w", err)
	}
	if entailmentCount != 0 {
		return fmt.Errorf("v49 migration unexpectedly materialized disjoint-entailment rows")
	}
	return verifyHistoricalTypedMemoryFootprints49(tx)
}

func verifyHistoricalTypedMemoryFootprints49(tx MigrationTransaction) error {
	var mismatchCount int
	err := tx.QueryRow(`SELECT COUNT(*)
		FROM typed_memory_commit_materialization_closures closure
		LEFT JOIN typed_memory_event_materialization_footprints_v46 footprint
			ON footprint.project_id = closure.project_id
			AND footprint.event_ref = closure.event_ref
		WHERE footprint.event_ref IS NULL
			OR closure.entity_count != footprint.entity_count
			OR closure.entity_context_count != footprint.entity_context_count
			OR closure.entity_declaration_count != footprint.entity_declaration_count
			OR closure.context_slice_catalog_count != footprint.context_slice_catalog_count
			OR closure.context_slice_count != footprint.context_slice_count
			OR closure.value_blob_count != footprint.value_blob_count
			OR closure.observable_input_blob_count != footprint.observable_input_blob_count
			OR closure.relation_count != footprint.relation_count
			OR closure.relation_slot_count != footprint.relation_slot_count
			OR closure.relation_filler_count != footprint.relation_filler_count
			OR closure.ordered_candidate_prefix_count != footprint.ordered_candidate_prefix_count
			OR closure.reference_resolution_use_count != footprint.reference_resolution_use_count
			OR closure.memberof_evaluation_count != footprint.memberof_evaluation_count
			OR closure.memberof_input_count != footprint.memberof_input_count
			OR closure.memberof_use_count != footprint.memberof_use_count
			OR closure.alias_change_count != footprint.alias_change_count
			OR closure.retraction_count != footprint.retraction_count
			OR closure.type_env_activation_count != footprint.type_env_activation_count`,
	).Scan(&mismatchCount)
	if err != nil {
		return fmt.Errorf("verify historical typed-memory footprints after v49: %w", err)
	}
	if mismatchCount != 0 {
		return fmt.Errorf(
			"v49 disjoint-entailment projection found %d historical materialization footprint mismatches",
			mismatchCount,
		)
	}
	return nil
}
