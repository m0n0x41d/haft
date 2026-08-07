package projecttypeenvstore

import (
	"context"
	"database/sql"
	"fmt"
)

// OpenExisting exposes the existing artifact store only when its current schema
// edition marker is present. It never creates, migrates, or repairs storage.
// The caller must verify any stronger kernel-owned table/trigger footprint.
// The database handle determines whether later operations may write.
func OpenExisting(
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

// OpenReadOnly is the compatibility name used by read-only compositions. The
// caller remains responsible for opening the database itself read-only.
func OpenReadOnly(
	ctx context.Context,
	database *sql.DB,
) (*Store, error) {
	return OpenExisting(ctx, database)
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
			 FROM project_typeenv_artifact_store_schema
			 WHERE singleton = 1`,
		).
		Scan(&version)
	if err != nil {
		return fmt.Errorf(
			"read project TypeEnv artifact schema without migration: %w",
			err,
		)
	}
	if version != CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv artifact schema version is %d; exact current version %d is required",
			version,
			CurrentSchemaVersion,
		)
	}
	return nil
}
