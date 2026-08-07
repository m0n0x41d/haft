package projecttypeenvheadstore

import (
	"context"
	"database/sql"
	"fmt"
)

// OpenReadOnly exposes existing head-state reads only when the exact current
// schema is already present. It never creates or migrates the head store.
func OpenReadOnly(
	ctx context.Context,
	database *sql.DB,
) (*Store, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if database == nil {
		return nil, ErrStoreRequired
	}
	if err := requireCurrentSchemaReadOnly(ctx, database); err != nil {
		return nil, err
	}
	return &Store{database: database}, nil
}

func requireCurrentSchemaReadOnly(
	ctx context.Context,
	database *sql.DB,
) error {
	version := 0
	err := database.
		QueryRowContext(
			ctx,
			`SELECT version
			 FROM project_typeenv_head_store_schema
			 WHERE singleton = 1`,
		).
		Scan(&version)
	if err != nil {
		return fmt.Errorf(
			"read project TypeEnv head schema without migration: %w",
			err,
		)
	}
	if version != CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv head schema version is %d; exact current version %d is required",
			version,
			CurrentSchemaVersion,
		)
	}
	return nil
}
