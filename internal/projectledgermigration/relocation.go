package projectledgermigration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

const rootRelocationOutcome = "root_relocated"

type RootRelocationRequest struct {
	request  Request
	fromRoot string
}

type RootRelocationResult struct {
	Outcome          string
	ProjectID        string
	FromRoot         string
	ToRoot           string
	DatabasePath     string
	BeforeSchema     int
	AfterSchema      int
	RelocationNumber int
	RelocationDigest string
	BackupPath       string
	BackupDigest     string
	RelocatedAt      time.Time
}

func NewRootRelocationRequest(
	fromRoot string,
	toRoot string,
	projectID string,
) (RootRelocationRequest, error) {
	request, err := NewRequest(toRoot, projectID)
	if err != nil {
		return RootRelocationRequest{}, err
	}
	canonicalFrom, err := canonicalRecordedRoot(fromRoot)
	if err != nil {
		return RootRelocationRequest{}, fmt.Errorf(
			"predecessor project root: %w",
			err,
		)
	}
	if canonicalFrom == request.root.String() {
		return RootRelocationRequest{}, fmt.Errorf(
			"predecessor and successor project roots must be distinct",
		)
	}
	return RootRelocationRequest{
		request:  request,
		fromRoot: canonicalFrom,
	}, nil
}

func RelocateRoot(
	ctx context.Context,
	request RootRelocationRequest,
	at time.Time,
) (RootRelocationResult, error) {
	if ctx == nil {
		return RootRelocationResult{}, fmt.Errorf(
			"relocate project root: context is required",
		)
	}
	if at.IsZero() {
		return RootRelocationResult{}, fmt.Errorf(
			"relocate project root: timestamp is required",
		)
	}
	observation, state, err := observeRelocation(ctx, request)
	if err != nil {
		return RootRelocationResult{}, err
	}
	if err := requireRelocationPredecessor(request, state); err != nil {
		return RootRelocationResult{}, err
	}
	coordinator, err := newMigrationCoordinator(observation)
	if err != nil {
		return RootRelocationResult{}, err
	}
	waitContext, cancel := boundedMigrationWaitContext(ctx)
	defer cancel()
	lease, _, err := coordinator.acquire(waitContext)
	if err != nil {
		return RootRelocationResult{}, err
	}
	result, relocationErr := relocateRootUnderLease(ctx, request, at.UTC())
	releaseErr := lease.release()
	if err := errors.Join(relocationErr, releaseErr); err != nil {
		return result, err
	}
	return result, nil
}

func observeRelocation(
	ctx context.Context,
	request RootRelocationRequest,
) (SchemaObservation, projectledger.PersistedRootState, error) {
	identity, err := projectledger.LoadIdentity(request.request.root.String())
	if err != nil {
		return SchemaObservation{}, projectledger.PersistedRootState{}, fmt.Errorf(
			"load successor project identity: %w",
			err,
		)
	}
	if identity.ProjectID() != request.request.project {
		return SchemaObservation{}, projectledger.PersistedRootState{}, fmt.Errorf(
			"successor root carries project %s, expected %s",
			identity.ProjectID().String(),
			request.request.project.String(),
		)
	}
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		request.request.root.String(),
		projectledger.ReadOnly,
	)
	if err != nil {
		return SchemaObservation{}, projectledger.PersistedRootState{}, fmt.Errorf(
			"open project ledger for root relocation observation: %w",
			err,
		)
	}
	frontier, frontierErr := observeSchemaFrontier(ctx, handle.Database())
	current, currentErr := db.CurrentSchemaVersion()
	var prefixErr error
	if frontierErr == nil && currentErr == nil && frontier <= current {
		prefixErr = db.RequireSchemaPrefixReadOnly(
			ctx,
			handle.Database(),
			frontier,
		)
	}
	state, stateErr := handle.InspectPersistedRootState(ctx)
	databasePath := handle.DatabasePath()
	closeErr := handle.Close()
	if err := errors.Join(
		frontierErr,
		currentErr,
		prefixErr,
		stateErr,
		closeErr,
	); err != nil {
		return SchemaObservation{}, projectledger.PersistedRootState{}, err
	}
	if frontier < db.ProjectLedgerBindingSchemaVersion {
		return SchemaObservation{}, projectledger.PersistedRootState{}, fmt.Errorf(
			"project-root relocation requires binding-aware schema %d or newer; found %d",
			db.ProjectLedgerBindingSchemaVersion,
			frontier,
		)
	}
	if frontier > current {
		return SchemaObservation{}, projectledger.PersistedRootState{}, fmt.Errorf(
			"project schema %d is newer than this Haft binary schema %d",
			frontier,
			current,
		)
	}
	return SchemaObservation{
		ProjectRoot:    request.request.root.String(),
		ProjectID:      request.request.project.String(),
		DatabasePath:   databasePath,
		ObservedSchema: frontier,
		CompiledSchema: current,
	}, state, nil
}

func requireRelocationPredecessor(
	request RootRelocationRequest,
	state projectledger.PersistedRootState,
) error {
	if state.ProjectID != request.request.project.String() {
		return fmt.Errorf(
			"project ledger carries id %s, expected %s",
			state.ProjectID,
			request.request.project.String(),
		)
	}
	if state.CurrentRoot != request.fromRoot {
		return fmt.Errorf(
			"project ledger current root is %q, not declared predecessor %q",
			state.CurrentRoot,
			request.fromRoot,
		)
	}
	if _, err := os.Lstat(request.fromRoot); err == nil {
		return fmt.Errorf(
			"predecessor project root still exists at %s; refusing to select between two live roots",
			request.fromRoot,
		)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect predecessor project root: %w", err)
	}
	return nil
}

func relocateRootUnderLease(
	ctx context.Context,
	request RootRelocationRequest,
	at time.Time,
) (RootRelocationResult, error) {
	handle, err := projectledger.OpenForExplicitMigration(
		ctx,
		request.request.root.String(),
		projectledger.ReadWrite,
	)
	if err != nil {
		return RootRelocationResult{}, fmt.Errorf(
			"open project ledger for root relocation: %w",
			err,
		)
	}
	result, relocationErr := relocateRootHandle(
		ctx,
		request,
		handle,
		at,
	)
	closeErr := handle.Close()
	if err := errors.Join(relocationErr, closeErr); err != nil {
		return result, err
	}
	verification, err := projectledger.OpenExisting(
		ctx,
		request.request.root.String(),
		projectledger.ReadOnly,
	)
	if err != nil {
		return result, fmt.Errorf(
			"verify relocated project attachment; backup retained at %s (%s): %w",
			result.BackupPath,
			result.BackupDigest,
			err,
		)
	}
	verifySchemaErr := db.RequireCurrentSchemaReadOnly(
		ctx,
		verification.Database(),
	)
	verifyCloseErr := verification.Close()
	if err := errors.Join(verifySchemaErr, verifyCloseErr); err != nil {
		return result, fmt.Errorf(
			"verify relocated project ledger; backup retained at %s (%s): %w",
			result.BackupPath,
			result.BackupDigest,
			err,
		)
	}
	return result, nil
}

func relocateRootHandle(
	ctx context.Context,
	request RootRelocationRequest,
	handle *projectledger.Handle,
	at time.Time,
) (RootRelocationResult, error) {
	before, err := observeSchemaFrontier(ctx, handle.Database())
	if err != nil {
		return RootRelocationResult{}, err
	}
	current, err := db.CurrentSchemaVersion()
	if err != nil {
		return RootRelocationResult{}, err
	}
	if before < db.ProjectLedgerBindingSchemaVersion || before > current {
		return RootRelocationResult{}, fmt.Errorf(
			"project-root relocation cannot cross schema %d with compiled schema %d",
			before,
			current,
		)
	}
	if err := db.RequireSchemaPrefixReadOnly(
		ctx,
		handle.Database(),
		before,
	); err != nil {
		return RootRelocationResult{}, err
	}
	state, err := handle.InspectPersistedRootState(ctx)
	if err != nil {
		return RootRelocationResult{}, err
	}
	if err := requireRelocationPredecessor(request, state); err != nil {
		return RootRelocationResult{}, err
	}
	witnesses, err := healthyProjectDatabaseWitnesses(
		ctx,
		handle.Database(),
		"project-root relocation",
	)
	if err != nil {
		return RootRelocationResult{}, err
	}
	snapshot, err := createRootRelocationSnapshot(
		ctx,
		handle.Database(),
		handle.DatabasePath(),
		request,
		before,
		current,
		at,
		witnesses,
	)
	partial := RootRelocationResult{
		ProjectID:    request.request.project.String(),
		FromRoot:     request.fromRoot,
		ToRoot:       request.request.root.String(),
		DatabasePath: handle.DatabasePath(),
		BeforeSchema: before,
		AfterSchema:  before,
		BackupPath:   snapshot.path,
		BackupDigest: snapshot.digest,
		RelocatedAt:  at,
	}
	if err != nil {
		return partial, err
	}
	if err := db.RunMigrations(handle.Database()); err != nil {
		return partial, fmt.Errorf(
			"upgrade project ledger for root relocation; backup retained at %s (%s): %w",
			snapshot.path,
			snapshot.digest,
			err,
		)
	}
	partial.AfterSchema = current
	if err := db.RequireCurrentSchemaReadOnly(ctx, handle.Database()); err != nil {
		return partial, fmt.Errorf(
			"verify relocation-capable schema; backup retained at %s (%s): %w",
			snapshot.path,
			snapshot.digest,
			err,
		)
	}
	relocation, err := handle.RelocateRoot(ctx, request.fromRoot, at)
	if err != nil {
		return partial, fmt.Errorf(
			"commit project-root relocation; backup retained at %s (%s): %w",
			snapshot.path,
			snapshot.digest,
			err,
		)
	}
	partial.Outcome = rootRelocationOutcome
	partial.RelocationNumber = relocation.Sequence
	partial.RelocationDigest = relocation.Digest
	if err := requirePreservedProjectDatabaseWitnesses(
		ctx,
		handle.Database(),
		"project-root relocation verification",
		witnesses,
	); err != nil {
		return partial, fmt.Errorf(
			"verify relocated project ledger; backup retained at %s (%s): %w",
			snapshot.path,
			snapshot.digest,
			err,
		)
	}
	return partial, nil
}

func createRootRelocationSnapshot(
	ctx context.Context,
	database *sql.DB,
	databasePath string,
	request RootRelocationRequest,
	beforeSchema int,
	afterSchema int,
	at time.Time,
	expectedWitnesses []db.LegacyDecisionSpecSectionForeignKeyWitness,
) (serveMigrationSnapshot, error) {
	partialPath, finalPath := rootRelocationSnapshotPaths(
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
	// #nosec G202 -- SQLite VACUUM INTO does not accept a bind parameter;
	// sqliteStringLiteral escapes the exact path as one SQL string literal.
	statement := "VACUUM INTO " + sqliteStringLiteral(partialPath)
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"create consistent project-root relocation snapshot: %w",
			err,
		)
	}
	if err := os.Chmod(partialPath, 0o600); err != nil {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"secure project-root relocation snapshot: %w",
			err,
		)
	}
	if err := syncSnapshotFile(partialPath); err != nil {
		return serveMigrationSnapshot{}, err
	}
	if err := verifyRootRelocationSnapshot(
		ctx,
		partialPath,
		request,
		beforeSchema,
		expectedWitnesses,
	); err != nil {
		return serveMigrationSnapshot{}, err
	}
	digest, err := digestRecoveryFile(partialPath)
	if err != nil {
		return serveMigrationSnapshot{}, err
	}
	if err := publishServeMigrationSnapshot(partialPath, finalPath); err != nil {
		return serveMigrationSnapshot{}, fmt.Errorf(
			"publish verified project-root relocation snapshot: %w",
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
			"published project-root relocation snapshot digest changed: found %s, want %s",
			finalDigest,
			digest,
		)
	}
	return serveMigrationSnapshot{path: finalPath, digest: digest}, nil
}

func rootRelocationSnapshotPaths(
	databasePath string,
	beforeSchema int,
	afterSchema int,
	at time.Time,
) (string, string) {
	stamp := at.UTC().Format("20060102T150405.000000000Z")
	base := fmt.Sprintf(
		"%s.pre-root-relocation-v%d-to-v%d-%s",
		filepath.Base(databasePath),
		beforeSchema,
		afterSchema,
		stamp,
	)
	directory := filepath.Dir(databasePath)
	return filepath.Join(directory, base+".partial"),
		filepath.Join(directory, base+".bak")
}

func verifyRootRelocationSnapshot(
	ctx context.Context,
	path string,
	request RootRelocationRequest,
	expectedSchema int,
	expectedWitnesses []db.LegacyDecisionSpecSectionForeignKeyWitness,
) error {
	if err := requireSecureSnapshotFile(path); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("mode", "ro")
	dsn := url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}
	database, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return fmt.Errorf("open project-root relocation snapshot: %w", err)
	}
	defer database.Close()
	if err := requirePreservedProjectDatabaseWitnesses(
		ctx,
		database,
		"project-root relocation snapshot verification",
		expectedWitnesses,
	); err != nil {
		return err
	}
	if err := db.RequireSchemaPrefixReadOnly(
		ctx,
		database,
		expectedSchema,
	); err != nil {
		return fmt.Errorf(
			"verify project-root relocation snapshot schema: %w",
			err,
		)
	}
	state, err := projectledger.InspectPersistedRootStateDatabase(ctx, database)
	if err != nil {
		return fmt.Errorf(
			"verify project-root relocation snapshot binding: %w",
			err,
		)
	}
	if state.ProjectID != request.request.project.String() ||
		state.CurrentRoot != request.fromRoot {
		return fmt.Errorf(
			"project-root relocation snapshot carries id %q root %q, want id %q root %q",
			state.ProjectID,
			state.CurrentRoot,
			request.request.project.String(),
			request.fromRoot,
		)
	}
	return nil
}

func canonicalRecordedRoot(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if raw != trimmed || !filepath.IsAbs(raw) || filepath.Clean(raw) != raw {
		return "", fmt.Errorf("must be a canonical absolute path")
	}
	return raw, nil
}
