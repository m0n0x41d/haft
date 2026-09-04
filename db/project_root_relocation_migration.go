package db

import (
	"fmt"
	"strings"
)

const ProjectRootRelocationSchemaVersion = 60

const projectLedgerCurrentBindingView = "project_ledger_current_binding"

// projectRootRelocationMigration60 preserves the immutable genesis binding and
// adds an append-only current-root chain. A pre-9.2 binary sees schema 60 as a
// future schema, so every automatic activation takes a verified schema-59
// snapshot first.
var projectRootRelocationMigration60 = Migration{
	Version:         ProjectRootRelocationSchemaVersion,
	ServeActivation: ServeActivationAutomaticWithSnapshot,
	Description:     "Add append-only project-root relocation lineage",
	Apply:           applyProjectRootRelocationMigration60,
}

func applyProjectRootRelocationMigration60(
	transaction MigrationTransaction,
	_ []Migration,
) error {
	for _, statement := range projectRootRelocationSchemaStatements60() {
		if _, err := transaction.Exec(statement); err != nil {
			return fmt.Errorf("install project-root relocation schema: %w", err)
		}
	}

	type triggerDefinition struct {
		name  string
		table string
		sql   string
	}
	sealedTables := map[string]struct{}{}
	sealedRows, err := transaction.Query(
		`SELECT DISTINCT tbl_name
		 FROM sqlite_schema
		 WHERE type = 'trigger'
		 AND sql LIKE '%sealed historical%'`,
	)
	if err != nil {
		return fmt.Errorf("discover sealed historical writers: %w", err)
	}
	for sealedRows.Next() {
		var table string
		if err := sealedRows.Scan(&table); err != nil {
			_ = sealedRows.Close()
			return fmt.Errorf("read sealed historical writer: %w", err)
		}
		sealedTables[table] = struct{}{}
	}
	if err := sealedRows.Err(); err != nil {
		_ = sealedRows.Close()
		return fmt.Errorf("read sealed historical writers: %w", err)
	}
	if err := sealedRows.Close(); err != nil {
		return fmt.Errorf("close sealed historical writer scan: %w", err)
	}
	rows, err := transaction.Query(
		`SELECT name, tbl_name, sql
		 FROM sqlite_schema
		 WHERE type = 'trigger'
		 AND sql LIKE '%project_ledger_binding%'
		 ORDER BY name`,
	)
	if err != nil {
		return fmt.Errorf("discover project-root guard triggers: %w", err)
	}
	definitions := []triggerDefinition{}
	for rows.Next() {
		definition := triggerDefinition{}
		if err := rows.Scan(
			&definition.name,
			&definition.table,
			&definition.sql,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read project-root guard trigger: %w", err)
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read project-root guard triggers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close project-root guard trigger scan: %w", err)
	}

	rewritten := 0
	for _, definition := range definitions {
		if projectRootRelocationTriggerUsesGenesisBinding(definition.name) {
			continue
		}
		if _, sealed := sealedTables[definition.table]; sealed {
			continue
		}
		if !strings.Contains(definition.sql, "project_ledger_binding") {
			return fmt.Errorf(
				"project-root guard trigger %q lost its binding reference",
				definition.name,
			)
		}
		replacement := strings.ReplaceAll(
			definition.sql,
			"project_ledger_binding",
			projectLedgerCurrentBindingView,
		)
		if _, err := transaction.Exec(
			"DROP TRIGGER " + quoteMigrationIdentifier(definition.name),
		); err != nil {
			return fmt.Errorf(
				"drop predecessor project-root guard trigger %q: %w",
				definition.name,
				err,
			)
		}
		if _, err := transaction.Exec(replacement); err != nil {
			return fmt.Errorf(
				"recreate relocation-aware project-root guard trigger %q: %w",
				definition.name,
				err,
			)
		}
		rewritten++
	}
	if rewritten == 0 {
		var alreadyRewritten int
		if err := transaction.QueryRow(
			`SELECT COUNT(*) FROM sqlite_schema
			 WHERE type = 'trigger'
			 AND sql LIKE '%project_ledger_current_binding%'`,
		).Scan(&alreadyRewritten); err != nil {
			return fmt.Errorf(
				"inspect existing relocation-aware project-root guards: %w",
				err,
			)
		}
		if alreadyRewritten == 0 {
			return fmt.Errorf(
				"no predecessor or relocation-aware project-root guard triggers were found",
			)
		}
	}

	var staleGuards int
	if err := transaction.QueryRow(
		`SELECT COUNT(*)
		 FROM sqlite_schema
		 WHERE type = 'trigger'
		 AND sql LIKE '%project_ledger_binding%'
		 AND name NOT IN (
			'project_ledger_binding_no_replace',
			'project_ledger_binding_no_update',
			'project_ledger_binding_no_delete',
			'project_ledger_root_relocations_exact_chain',
			'project_ledger_root_relocations_no_cycle'
		 )
		 AND tbl_name NOT IN (
			SELECT DISTINCT tbl_name
			FROM sqlite_schema
			WHERE type = 'trigger'
			AND sql LIKE '%sealed historical%'
		 )`,
	).Scan(&staleGuards); err != nil {
		return fmt.Errorf("verify relocation-aware project-root guards: %w", err)
	}
	if staleGuards != 0 {
		return fmt.Errorf(
			"%d project-root guard trigger(s) still read only the genesis binding",
			staleGuards,
		)
	}
	return nil
}

func projectRootRelocationTriggerUsesGenesisBinding(name string) bool {
	switch name {
	case "project_ledger_binding_no_replace",
		"project_ledger_binding_no_update",
		"project_ledger_binding_no_delete",
		"project_ledger_root_relocations_exact_chain",
		"project_ledger_root_relocations_no_cycle":
		return true
	default:
		return false
	}
}

func quoteMigrationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func projectRootRelocationSchemaStatements60() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS project_ledger_root_relocations (
			relocation_sequence INTEGER PRIMARY KEY CHECK(relocation_sequence > 0),
			project_id TEXT NOT NULL CHECK(
				length(project_id) = 12
				AND substr(project_id, 1, 4) = 'qnt_'
				AND substr(project_id, 5) NOT GLOB '*[^0-9a-f]*'
			),
			from_root TEXT NOT NULL UNIQUE,
			to_root TEXT NOT NULL UNIQUE,
			predecessor_digest TEXT NOT NULL UNIQUE CHECK(
				length(predecessor_digest) = 71
				AND substr(predecessor_digest, 1, 7) = 'sha256:'
			),
			relocation_digest TEXT NOT NULL UNIQUE CHECK(
				length(relocation_digest) = 71
				AND substr(relocation_digest, 1, 7) = 'sha256:'
			),
			relocation_json TEXT NOT NULL UNIQUE CHECK(json_valid(relocation_json)),
			relocated_at TEXT NOT NULL,
			CHECK(from_root <> to_root),
			CHECK(json_extract(relocation_json, '$.schema') = 'haft.project-ledger-root-relocation/v1'),
			CHECK(json_extract(relocation_json, '$.relocation_sequence') = relocation_sequence),
			CHECK(json_extract(relocation_json, '$.project_id') = project_id),
			CHECK(json_extract(relocation_json, '$.from_root') = from_root),
			CHECK(json_extract(relocation_json, '$.to_root') = to_root),
			CHECK(json_extract(relocation_json, '$.predecessor_digest') = predecessor_digest),
			CHECK(json_extract(relocation_json, '$.relocated_at') = relocated_at)
		) WITHOUT ROWID`,
		`CREATE TRIGGER IF NOT EXISTS project_ledger_root_relocations_exact_chain
		 BEFORE INSERT ON project_ledger_root_relocations
		 WHEN NOT EXISTS (
			SELECT 1
			FROM project_ledger_binding binding
			WHERE binding.project_id = NEW.project_id
			AND NEW.relocation_sequence = COALESCE((
				SELECT MAX(relocation_sequence) + 1
				FROM project_ledger_root_relocations
			), 1)
			AND NEW.from_root = COALESCE((
				SELECT to_root
				FROM project_ledger_root_relocations
				ORDER BY relocation_sequence DESC
				LIMIT 1
			), binding.project_root)
			AND NEW.predecessor_digest = COALESCE((
				SELECT relocation_digest
				FROM project_ledger_root_relocations
				ORDER BY relocation_sequence DESC
				LIMIT 1
			), binding.binding_digest)
		 ) BEGIN
			SELECT RAISE(ABORT, 'project-root relocation does not extend the exact current binding');
		 END`,
		`CREATE TRIGGER IF NOT EXISTS project_ledger_root_relocations_no_cycle
		 BEFORE INSERT ON project_ledger_root_relocations
		 WHEN EXISTS (
			SELECT 1 FROM project_ledger_binding
			WHERE project_root = NEW.to_root
		 ) OR EXISTS (
			SELECT 1 FROM project_ledger_root_relocations
			WHERE from_root = NEW.to_root OR to_root = NEW.to_root
		 ) BEGIN
			SELECT RAISE(ABORT, 'project-root relocation cannot reuse an earlier root');
		 END`,
		`CREATE TRIGGER IF NOT EXISTS project_ledger_root_relocations_no_update
		 BEFORE UPDATE ON project_ledger_root_relocations BEGIN
			SELECT RAISE(ABORT, 'project_ledger_root_relocations is append-only');
		 END`,
		`CREATE TRIGGER IF NOT EXISTS project_ledger_root_relocations_no_delete
		 BEFORE DELETE ON project_ledger_root_relocations BEGIN
			SELECT RAISE(ABORT, 'project_ledger_root_relocations is append-only');
		 END`,
		`CREATE VIEW IF NOT EXISTS project_ledger_current_binding AS
		 SELECT
			binding.binding_slot,
			binding.project_id,
			COALESCE((
				SELECT relocation.to_root
				FROM project_ledger_root_relocations relocation
				ORDER BY relocation.relocation_sequence DESC
				LIMIT 1
			), binding.project_root) AS project_root,
			binding.project_root AS genesis_project_root,
			binding.binding_digest,
			binding.binding_json,
			binding.bound_at
		 FROM project_ledger_binding binding`,
	}
}
