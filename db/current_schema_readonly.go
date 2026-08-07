package db

import (
	"context"
	"database/sql"
	"fmt"
)

// RequireCurrentSchemaReadOnly verifies the exact kernel migration edition
// without creating the version table or applying migrations.
func RequireCurrentSchemaReadOnly(
	ctx context.Context,
	database *sql.DB,
) error {
	if ctx == nil {
		return fmt.Errorf("verify current kernel schema: context is required")
	}
	if database == nil {
		return fmt.Errorf("verify current kernel schema: database is required")
	}
	expected, err := currentKernelSchemaVersions()
	if err != nil {
		return err
	}
	rows, err := database.QueryContext(
		ctx,
		`SELECT version FROM schema_version ORDER BY version`,
	)
	if err != nil {
		return fmt.Errorf("read kernel schema versions without migration: %w", err)
	}
	defer rows.Close()

	observed := make([]int, 0, len(expected))
	for rows.Next() {
		version := 0
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("decode kernel schema version: %w", err)
		}
		observed = append(observed, version)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read kernel schema versions: %w", err)
	}
	if len(observed) != len(expected) {
		return fmt.Errorf(
			"kernel schema is not current: found versions %v, want %v",
			observed,
			expected,
		)
	}
	for index := range expected {
		if observed[index] != expected[index] {
			return fmt.Errorf(
				"kernel schema is not current: found versions %v, want %v",
				observed,
				expected,
			)
		}
	}
	return nil
}

// RequireSchemaPrefixReadOnly verifies that schema_version contains exactly
// the immutable kernel migration prefix through frontier. It performs no
// migration and creates no schema objects.
func RequireSchemaPrefixReadOnly(
	ctx context.Context,
	database *sql.DB,
	frontier int,
) error {
	if ctx == nil {
		return fmt.Errorf(
			"verify kernel schema prefix: context is required",
		)
	}
	if database == nil {
		return fmt.Errorf(
			"verify kernel schema prefix: database is required",
		)
	}
	versions, err := currentKernelSchemaVersions()
	if err != nil {
		return err
	}
	if frontier < 1 || frontier > versions[len(versions)-1] {
		return fmt.Errorf(
			"verify kernel schema prefix: frontier %d is outside the compiled migration catalog",
			frontier,
		)
	}
	expected := versions[:frontier]
	rows, err := database.QueryContext(
		ctx,
		`SELECT version FROM schema_version ORDER BY version`,
	)
	if err != nil {
		return fmt.Errorf(
			"read kernel schema prefix without migration: %w",
			err,
		)
	}
	defer rows.Close()

	observed := make([]int, 0, len(expected))
	for rows.Next() {
		version := 0
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf(
				"decode kernel schema prefix version: %w",
				err,
			)
		}
		observed = append(observed, version)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read kernel schema prefix: %w", err)
	}
	if len(observed) != len(expected) {
		return fmt.Errorf(
			"kernel schema is not an exact prefix through version %d: found versions %v, want %v",
			frontier,
			observed,
			expected,
		)
	}
	for index := range expected {
		if observed[index] != expected[index] {
			return fmt.Errorf(
				"kernel schema is not an exact prefix through version %d: found versions %v, want %v",
				frontier,
				observed,
				expected,
			)
		}
	}
	return nil
}

// CurrentSchemaVersion returns the exact latest kernel migration edition.
// It observes only the compiled migration catalog and performs no IO.
func CurrentSchemaVersion() (int, error) {
	versions, err := currentKernelSchemaVersions()
	if err != nil {
		return 0, err
	}
	return versions[len(versions)-1], nil
}

func currentKernelSchemaVersions() ([]int, error) {
	if len(kernelMigrations) == 0 {
		return nil, fmt.Errorf("verify current kernel schema: migration catalog is empty")
	}
	versions := make([]int, len(kernelMigrations))
	for index, migration := range kernelMigrations {
		expected := index + 1
		if migration.Version != expected {
			return nil, fmt.Errorf(
				"verify current kernel schema: migration catalog is not contiguous at %d",
				expected,
			)
		}
		versions[index] = migration.Version
	}
	return versions, nil
}
