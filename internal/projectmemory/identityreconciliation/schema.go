package identityreconciliation

import (
	"context"
	"database/sql"
	"fmt"
)

// schemaGate is capability-local: the effect depends on the immutable v52
// footprint, not on the whole kernel migration catalog. Keeping this check in
// the reconciliation layer avoids a storage-owner dependency cycle and lets a
// later additive kernel schema retain the v52 capability unchanged.
type schemaGate interface {
	RequireCompatible(context.Context) error
}

type sqliteSchemaGate struct {
	database *sql.DB
}

func newSQLiteSchemaGate(database *sql.DB) schemaGate {
	return sqliteSchemaGate{database: database}
}

func (gate sqliteSchemaGate) RequireCompatible(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("verify identity-reconciliation schema: context is required")
	}
	if gate.database == nil {
		return ErrDatabaseRequired
	}
	var versionCount int
	err := gate.database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM schema_version WHERE version = 52`,
	).Scan(&versionCount)
	if err != nil {
		return fmt.Errorf("read identity-reconciliation schema version: %w", err)
	}
	if versionCount != 1 {
		return fmt.Errorf("identity-reconciliation schema version 52 is unavailable")
	}
	for _, object := range identitySchemaObjects() {
		var count int
		err := gate.database.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`,
			object.kind,
			object.name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("inspect identity-reconciliation %s %s: %w", object.kind, object.name, err)
		}
		if count != 1 {
			return fmt.Errorf("identity-reconciliation %s %s is unavailable", object.kind, object.name)
		}
	}
	return nil
}

type identitySchemaObject struct {
	kind string
	name string
}

func identitySchemaObjects() []identitySchemaObject {
	return []identitySchemaObject{
		{kind: "table", name: "typed_memory_identity_reconciliations"},
		{kind: "table", name: "typed_memory_identity_reconciliation_participants"},
		{kind: "table", name: "typed_memory_identity_redirects"},
		{kind: "table", name: "typed_memory_identity_reconciliation_closures"},
		{kind: "trigger", name: "typed_memory_identity_reconciliations_v52_exact_event"},
		{kind: "trigger", name: "typed_memory_identity_participants_v52_exact_reconciliation"},
		{kind: "trigger", name: "typed_memory_identity_redirects_v52_exact_participant"},
		{kind: "trigger", name: "typed_memory_identity_closures_v52_exact_reconciliation"},
	}
}
