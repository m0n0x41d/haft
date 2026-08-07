package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// ProjectLedgerBindingSchemaVersion is the first kernel schema edition whose
// catalog creates the durable project-ledger binding table.
const ProjectLedgerBindingSchemaVersion = 37

// RunMigrationsThroughProjectLedgerBinding applies the kernel migration prefix
// through the durable project-ledger binding boundary. When version 37 is
// pending, its existing catalog statements and bind are committed in one
// transaction. When version 37 is already recorded, bind is not invoked; the
// caller must verify the existing binding rather than repair it.
func RunMigrationsThroughProjectLedgerBinding(
	connection *sql.DB,
	bind func(MigrationTransaction) error,
) error {
	if bind == nil {
		return fmt.Errorf("project ledger binding migration callback is required")
	}
	migrations, err := projectLedgerBindingMigrationPrefix(bind)
	if err != nil {
		return err
	}
	return Migrate(connection, "schema_version", migrations)
}

func projectLedgerBindingMigrationPrefix(
	bind func(MigrationTransaction) error,
) ([]Migration, error) {
	prefix := make([]Migration, 0, ProjectLedgerBindingSchemaVersion)
	targetIndex := -1
	for _, migration := range kernelMigrations {
		cloned := migration
		cloned.Statements = append([]string(nil), migration.Statements...)
		prefix = append(prefix, cloned)
		if migration.Version == ProjectLedgerBindingSchemaVersion {
			targetIndex = len(prefix) - 1
			break
		}
	}
	if targetIndex < 0 {
		return nil, fmt.Errorf(
			"project ledger binding migration %d is unavailable",
			ProjectLedgerBindingSchemaVersion,
		)
	}
	target := prefix[targetIndex]
	if target.Apply != nil ||
		len(target.Statements) == 0 ||
		target.ApplyBoundary != ForeignKeysEnforcedBoundary ||
		target.ForeignKeyVerifier != nil {
		return nil, fmt.Errorf(
			"project ledger binding migration %d does not have the expected statement boundary",
			ProjectLedgerBindingSchemaVersion,
		)
	}
	statements := append([]string(nil), target.Statements...)
	target.Statements = nil
	target.Apply = func(
		transaction MigrationTransaction,
		_ []Migration,
	) error {
		for _, statement := range statements {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := transaction.Exec(statement); err != nil {
				if !isIdempotentError(err) {
					return fmt.Errorf(
						"apply project ledger binding schema: %w",
						err,
					)
				}
			}
		}
		if err := bind(transaction); err != nil {
			return fmt.Errorf("bind project ledger identity: %w", err)
		}
		var bindingCount int
		if err := transaction.QueryRow(
			"SELECT COUNT(*) FROM project_ledger_binding",
		).Scan(&bindingCount); err != nil {
			return fmt.Errorf(
				"verify project ledger binding migration effect: %w",
				err,
			)
		}
		if bindingCount != 1 {
			return fmt.Errorf(
				"verify project ledger binding migration effect: found %d durable bindings, want exactly 1",
				bindingCount,
			)
		}
		return nil
	}
	prefix[targetIndex] = target
	return prefix, nil
}
