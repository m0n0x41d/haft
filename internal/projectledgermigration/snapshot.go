package projectledgermigration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

type serveMigrationSnapshot struct {
	path   string
	digest string
}

func createServeMigrationSnapshot(
	ctx context.Context,
	database *sql.DB,
	databasePath string,
	request Request,
	beforeSchema int,
	afterSchema int,
	at time.Time,
) (serveMigrationSnapshot, error) {
	if at.IsZero() {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"create serve migration snapshot: timestamp is required",
		)
	}
	partialPath, finalPath := serveMigrationSnapshotPaths(
		databasePath,
		beforeSchema,
		afterSchema,
		at,
	)
	if err := requireSnapshotPathAbsent(partialPath); err != nil {
		return serveMigrationSnapshot{}, err
	}
	if err := requireSnapshotPathAbsent(finalPath); err != nil {
		return serveMigrationSnapshot{}, err
	}
	literal := sqliteStringLiteral(partialPath)
	// #nosec G202 -- SQLite VACUUM INTO does not accept a bind parameter; sqliteStringLiteral escapes the exact path as one SQL string literal.
	statement := "VACUUM INTO " + literal
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"create consistent serve migration snapshot at %s: %w",
			partialPath,
			err,
		)
	}
	if err := os.Chmod(partialPath, 0o600); err != nil {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"secure serve migration snapshot %s: %w",
			partialPath,
			err,
		)
	}
	if err := syncSnapshotFile(partialPath); err != nil {
		return serveMigrationSnapshot{}, err
	}
	if err := verifyServeMigrationSnapshot(
		ctx,
		partialPath,
		request,
		beforeSchema,
	); err != nil {
		return serveMigrationSnapshot{}, err
	}
	digest, err := digestRecoveryFile(partialPath)
	if err != nil {
		return serveMigrationSnapshot{}, err
	}
	if err := publishServeMigrationSnapshot(partialPath, finalPath); err != nil {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"publish verified serve migration snapshot %s: %w",
			finalPath,
			err,
		)
	}
	if err := syncSnapshotDirectory(filepath.Dir(finalPath)); err != nil {
		return serveMigrationSnapshot{}, err
	}
	if err := requireSecureSnapshotFile(finalPath); err != nil {
		return serveMigrationSnapshot{}, err
	}
	finalDigest, err := digestRecoveryFile(finalPath)
	if err != nil {
		return serveMigrationSnapshot{}, err
	}
	if finalDigest != digest {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"published serve migration snapshot digest changed: found %s, want %s",
			finalDigest,
			digest,
		)
	}
	return serveMigrationSnapshot{
		path:   finalPath,
		digest: digest,
	}, nil
}

func serveMigrationSnapshotPaths(
	databasePath string,
	beforeSchema int,
	afterSchema int,
	at time.Time,
) (string, string) {
	stamp := at.UTC().Format("20060102T150405.000000000Z")
	base := fmt.Sprintf(
		"%s.pre-serve-migration-v%d-to-v%d-%s",
		filepath.Base(databasePath),
		beforeSchema,
		afterSchema,
		stamp,
	)
	directory := filepath.Dir(databasePath)
	return filepath.Join(directory, base+".partial"),
		filepath.Join(directory, base+".bak")
}

func requireSnapshotPathAbsent(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf("serve migration snapshot path already exists: %s", path)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect serve migration snapshot path %s: %w", path, err)
	}
	return nil
}

func syncSnapshotFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open serve migration snapshot for sync: %w", err)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return fmt.Errorf("sync serve migration snapshot: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close synced serve migration snapshot: %w", closeErr)
	}
	return nil
}

func syncSnapshotDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open serve migration snapshot directory: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("sync serve migration snapshot directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close serve migration snapshot directory: %w", closeErr)
	}
	return nil
}

func verifyServeMigrationSnapshot(
	ctx context.Context,
	path string,
	request Request,
	expectedSchema int,
) error {
	if err := requireSecureSnapshotFile(path); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("mode", "ro")
	dsn := url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: query.Encode(),
	}
	database, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return fmt.Errorf("open serve migration snapshot: %w", err)
	}
	defer database.Close()
	if err := requireHealthyProjectDatabase(
		ctx,
		database,
		"serve migration snapshot verification",
	); err != nil {
		return err
	}
	if err := db.RequireSchemaPrefixReadOnly(
		ctx,
		database,
		expectedSchema,
	); err != nil {
		return fmt.Errorf("verify serve migration snapshot schema: %w", err)
	}
	reader := databasePersistedBindingReader{database: database}
	if err := projectledger.RequireExactPersistedBinding(
		ctx,
		reader,
		request.project,
	); err != nil {
		return fmt.Errorf("verify serve migration snapshot binding: %w", err)
	}
	var projectRoot string
	if err := database.QueryRowContext(
		ctx,
		`SELECT project_root FROM project_ledger_binding
		 WHERE binding_slot = 1`,
	).Scan(&projectRoot); err != nil {
		return fmt.Errorf("read serve migration snapshot project root: %w", err)
	}
	if projectRoot != request.root.String() {
		return fmt.Errorf(
			"serve migration snapshot is bound to project root %q, want %q",
			projectRoot,
			request.root.String(),
		)
	}
	return nil
}

type databasePersistedBindingReader struct {
	database *sql.DB
}

func (reader databasePersistedBindingReader) ScanOne(
	ctx context.Context,
	query string,
	arguments []any,
	destinations []any,
) error {
	return reader.database.QueryRowContext(
		ctx,
		query,
		arguments...,
	).Scan(destinations...)
}

func requireSecureSnapshotFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect serve migration snapshot: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("serve migration snapshot is not a regular file")
	}
	if info.Mode().Perm() != 0o600 {
		return fmt.Errorf(
			"serve migration snapshot mode = %04o, want 0600",
			info.Mode().Perm(),
		)
	}
	return nil
}
