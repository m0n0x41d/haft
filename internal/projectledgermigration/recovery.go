package projectledgermigration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

const bindingRecoveryOutcome = "binding_recovered"

type RecoveryResult struct {
	Outcome       string
	ProjectRoot   string
	ProjectID     string
	DatabasePath  string
	BackupPath    string
	BackupDigest  string
	SchemaVersion int
	BoundAt       time.Time
}

func RecoverMissingBinding(
	ctx context.Context,
	request Request,
	at time.Time,
) (RecoveryResult, error) {
	if ctx == nil {
		return RecoveryResult{}, fmt.Errorf(
			"recover project ledger binding: context is required",
		)
	}
	if at.IsZero() {
		return RecoveryResult{}, fmt.Errorf(
			"recover project ledger binding: binding time is required",
		)
	}
	identity, err := projectledger.LoadIdentity(request.root.String())
	if err != nil {
		return RecoveryResult{}, fmt.Errorf(
			"load exact project identity for binding recovery: %w",
			err,
		)
	}
	if identity.ProjectID() != request.project {
		return RecoveryResult{}, fmt.Errorf(
			"project identity mismatch: root carries %s, expected %s",
			identity.ProjectID().String(),
			request.project.String(),
		)
	}
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		request.root.String(),
		projectledger.ReadWrite,
	)
	if err != nil {
		return RecoveryResult{}, fmt.Errorf(
			"open exact project ledger for binding recovery: %w",
			err,
		)
	}
	result, recoveryErr := prepareBindingRecovery(
		ctx,
		request,
		handle,
		at.UTC(),
	)
	closeErr := handle.Close()
	if err := errors.Join(recoveryErr, closeErr); err != nil {
		return RecoveryResult{}, err
	}
	if err := projectledger.BindInitialized(
		ctx,
		request.root.String(),
		at.UTC(),
	); err != nil {
		return RecoveryResult{}, fmt.Errorf(
			"bind recovered project ledger; consistent backup retained at %s (%s): %w",
			result.BackupPath,
			result.BackupDigest,
			err,
		)
	}
	if err := verifyRecoveredBinding(
		ctx,
		request,
		result.SchemaVersion,
	); err != nil {
		return RecoveryResult{}, fmt.Errorf(
			"verify recovered project ledger binding; consistent backup retained at %s (%s): %w",
			result.BackupPath,
			result.BackupDigest,
			err,
		)
	}
	return result, nil
}

func prepareBindingRecovery(
	ctx context.Context,
	request Request,
	handle *projectledger.Handle,
	at time.Time,
) (RecoveryResult, error) {
	database := handle.Database()
	frontier, err := observeSchemaFrontier(ctx, database)
	if err != nil {
		return RecoveryResult{}, err
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		return RecoveryResult{}, fmt.Errorf(
			"resolve compiled schema frontier for binding recovery: %w",
			err,
		)
	}
	if frontier < db.ProjectLedgerBindingSchemaVersion {
		return RecoveryResult{}, fmt.Errorf(
			"project ledger binding recovery requires binding-aware schema %d or newer; found %d",
			db.ProjectLedgerBindingSchemaVersion,
			frontier,
		)
	}
	if frontier > current {
		return RecoveryResult{}, fmt.Errorf(
			"project ledger schema %d is newer than this Haft binary schema %d",
			frontier,
			current,
		)
	}
	if err := db.RequireSchemaPrefixReadOnly(
		ctx,
		database,
		frontier,
	); err != nil {
		return RecoveryResult{}, fmt.Errorf(
			"verify project schema prefix for binding recovery: %w",
			err,
		)
	}
	bindingErr := handle.RequireAttachedIdentity(ctx)
	if bindingErr == nil {
		return RecoveryResult{}, fmt.Errorf(
			"project ledger already has its durable identity binding",
		)
	}
	if !errors.Is(bindingErr, projectledger.ErrBindingMissing) {
		return RecoveryResult{}, fmt.Errorf(
			"inspect missing project ledger binding: %w",
			bindingErr,
		)
	}
	if err := requireHealthyRecoveryDatabase(ctx, database); err != nil {
		return RecoveryResult{}, err
	}
	backupPath := bindingRecoveryBackupPath(handle.DatabasePath(), at)
	backupDigest, err := createBindingRecoveryBackup(
		ctx,
		database,
		backupPath,
		frontier,
	)
	if err != nil {
		return RecoveryResult{}, err
	}
	return RecoveryResult{
		Outcome:       bindingRecoveryOutcome,
		ProjectRoot:   request.root.String(),
		ProjectID:     request.project.String(),
		DatabasePath:  handle.DatabasePath(),
		BackupPath:    backupPath,
		BackupDigest:  backupDigest,
		SchemaVersion: frontier,
		BoundAt:       at,
	}, nil
}

func requireHealthyRecoveryDatabase(
	ctx context.Context,
	database *sql.DB,
) error {
	var integrity string
	err := database.QueryRowContext(
		ctx,
		"PRAGMA integrity_check",
	).Scan(&integrity)
	if err != nil {
		return fmt.Errorf(
			"inspect project ledger integrity for binding recovery: %w",
			err,
		)
	}
	if integrity != "ok" {
		return fmt.Errorf(
			"project ledger integrity blocks binding recovery: %s",
			integrity,
		)
	}
	var foreignKeyViolations int
	err = database.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM pragma_foreign_key_check",
	).Scan(&foreignKeyViolations)
	if err != nil {
		return fmt.Errorf(
			"inspect project ledger foreign keys for binding recovery: %w",
			err,
		)
	}
	if foreignKeyViolations != 0 {
		return fmt.Errorf(
			"project ledger has %d foreign-key violation(s); binding recovery was not attempted",
			foreignKeyViolations,
		)
	}
	return nil
}

func bindingRecoveryBackupPath(databasePath string, at time.Time) string {
	stamp := at.UTC().Format("20060102T150405.000000000Z")
	name := filepath.Base(databasePath) + ".pre-binding-recovery-" + stamp + ".bak"
	return filepath.Join(filepath.Dir(databasePath), name)
}

func createBindingRecoveryBackup(
	ctx context.Context,
	database *sql.DB,
	backupPath string,
	expectedSchema int,
) (string, error) {
	_, err := os.Lstat(backupPath)
	if err == nil {
		return "", fmt.Errorf(
			"project ledger binding recovery backup already exists: %s",
			backupPath,
		)
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf(
			"inspect project ledger binding recovery backup: %w",
			err,
		)
	}
	literal := sqliteStringLiteral(backupPath)
	statement := "VACUUM INTO " + literal
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return "", fmt.Errorf(
			"create consistent project ledger binding recovery backup: %w",
			err,
		)
	}
	if err := verifyBindingRecoveryBackup(
		ctx,
		backupPath,
		expectedSchema,
	); err != nil {
		return "", err
	}
	digest, err := digestRecoveryFile(backupPath)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func verifyBindingRecoveryBackup(
	ctx context.Context,
	backupPath string,
	expectedSchema int,
) error {
	query := url.Values{}
	query.Set("mode", "ro")
	dsn := url.URL{
		Scheme:   "file",
		Path:     backupPath,
		RawQuery: query.Encode(),
	}
	database, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return fmt.Errorf(
			"open project ledger binding recovery backup: %w",
			err,
		)
	}
	defer database.Close()
	if err := requireHealthyRecoveryDatabase(ctx, database); err != nil {
		return fmt.Errorf(
			"verify project ledger binding recovery backup: %w",
			err,
		)
	}
	frontier, err := observeSchemaFrontier(ctx, database)
	if err != nil {
		return fmt.Errorf(
			"read project ledger binding recovery backup schema: %w",
			err,
		)
	}
	if frontier != expectedSchema {
		return fmt.Errorf(
			"project ledger binding recovery backup schema = %d, want %d",
			frontier,
			expectedSchema,
		)
	}
	var bindingCount int
	err = database.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM project_ledger_binding",
	).Scan(&bindingCount)
	if err != nil {
		return fmt.Errorf(
			"inspect project ledger binding recovery backup: %w",
			err,
		)
	}
	if bindingCount != 0 {
		return fmt.Errorf(
			"project ledger binding recovery backup unexpectedly contains %d binding row(s)",
			bindingCount,
		)
	}
	return nil
}

func verifyRecoveredBinding(
	ctx context.Context,
	request Request,
	expectedSchema int,
) error {
	handle, err := projectledger.OpenExisting(
		ctx,
		request.root.String(),
		projectledger.ReadOnly,
	)
	if err != nil {
		return err
	}
	defer handle.Close()
	if handle.ProjectID() != request.project {
		return fmt.Errorf(
			"recovered ledger carries project %s, expected %s",
			handle.ProjectID().String(),
			request.project.String(),
		)
	}
	return db.RequireSchemaPrefixReadOnly(
		ctx,
		handle.Database(),
		expectedSchema,
	)
}

func sqliteStringLiteral(raw string) string {
	escaped := strings.ReplaceAll(raw, "'", "''")
	return "'" + escaped + "'"
}

func digestRecoveryFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf(
			"open project ledger binding recovery backup for digest: %w",
			err,
		)
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf(
			"digest project ledger binding recovery backup: %w",
			err,
		)
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), nil
}
