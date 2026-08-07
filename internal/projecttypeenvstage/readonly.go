package projecttypeenvstage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
)

// OpenExisting exposes the existing Stage and artifact stores only when both
// current schema edition markers are present. It performs no create, migration,
// or repair. The caller must verify any stronger kernel-owned table/trigger
// footprint. The database handle determines whether later operations may write.
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
	artifacts, err := projecttypeenvstore.OpenExisting(ctx, database)
	if err != nil {
		return nil, fmt.Errorf(
			"open project TypeEnv artifact store read-only: %w",
			err,
		)
	}
	if err := requireCurrentSchemaReadOnly(ctx, database); err != nil {
		return nil, err
	}
	return &Store{database: database, artifacts: artifacts}, nil
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
			 FROM project_typeenv_stage_store_schema
			 WHERE singleton = 1`,
		).
		Scan(&version)
	if err != nil {
		return fmt.Errorf(
			"read project TypeEnv Stage schema without migration: %w",
			err,
		)
	}
	if version != CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv Stage schema version is %d; exact current version %d is required",
			version,
			CurrentSchemaVersion,
		)
	}
	return nil
}
