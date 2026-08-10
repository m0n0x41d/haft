package projectledgermigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
)

const (
	defaultMigrationLeaseWait  = 30 * time.Second
	migrationLeasePollInterval = 10 * time.Millisecond
	migrationLeaseFilename     = "project-ledger-migration.lock"
)

var ErrMigrationLeaseTimeout = errors.New(
	"project ledger migration lease was not acquired before the retry deadline",
)

type migrationProcessState struct {
	gate chan struct{}
}

var migrationProcessStates sync.Map

type migrationCoordinator struct {
	directory string
	state     *migrationProcessState
}

type heldMigrationLease struct {
	file     migrationFileLease
	state    *migrationProcessState
	released bool
}

func newMigrationCoordinator(
	observation SchemaObservation,
) (*migrationCoordinator, error) {
	projectID, err := projectledger.ParseProjectID(observation.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("migration project coordinate: %w", err)
	}
	root, err := requireCanonicalMigrationDirectory(
		"migration project root",
		observation.ProjectRoot,
	)
	if err != nil {
		return nil, err
	}
	databasePath, err := requireCanonicalMigrationDatabase(
		observation.DatabasePath,
	)
	if err != nil {
		return nil, err
	}
	directory, err := requireCanonicalMigrationDirectory(
		"migration ledger directory",
		filepath.Dir(databasePath),
	)
	if err != nil {
		return nil, err
	}
	key := migrationCoordinateKey(
		projectID.String(),
		root,
		databasePath,
	)
	stateValue, _ := migrationProcessStates.LoadOrStore(
		key,
		newMigrationProcessState(),
	)
	return &migrationCoordinator{
		directory: directory,
		state:     stateValue.(*migrationProcessState),
	}, nil
}

func newMigrationProcessState() *migrationProcessState {
	state := &migrationProcessState{gate: make(chan struct{}, 1)}
	state.gate <- struct{}{}
	return state
}

func migrationCoordinateKey(
	projectID string,
	projectRoot string,
	databasePath string,
) string {
	digest := sha256.Sum256([]byte(
		projectID + "\x00" + projectRoot + "\x00" + databasePath,
	))
	return hex.EncodeToString(digest[:])
}

func requireCanonicalMigrationDirectory(
	label string,
	raw string,
) (string, error) {
	if raw != strings.TrimSpace(raw) ||
		!filepath.IsAbs(raw) ||
		filepath.Clean(raw) != raw {
		return "", fmt.Errorf("%s must be a canonical absolute path", label)
	}
	info, err := os.Lstat(raw)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s must be a real directory", label)
	}
	physical, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", fmt.Errorf("resolve physical %s: %w", label, err)
	}
	if filepath.Clean(physical) != raw {
		return "", fmt.Errorf("%s must use canonical physical form", label)
	}
	return raw, nil
}

func requireCanonicalMigrationDatabase(raw string) (string, error) {
	if raw != strings.TrimSpace(raw) ||
		!filepath.IsAbs(raw) ||
		filepath.Clean(raw) != raw {
		return "", fmt.Errorf(
			"migration ledger database must be a canonical absolute path",
		)
	}
	info, err := os.Lstat(raw)
	if err != nil {
		return "", fmt.Errorf("inspect migration ledger database: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf(
			"migration ledger database must be a real regular file",
		)
	}
	physical, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", fmt.Errorf(
			"resolve physical migration ledger database: %w",
			err,
		)
	}
	if filepath.Clean(physical) != raw {
		return "", fmt.Errorf(
			"migration ledger database must use canonical physical form",
		)
	}
	return raw, nil
}

func boundedMigrationWaitContext(
	parent context.Context,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, defaultMigrationLeaseWait)
}

func (coordinator *migrationCoordinator) acquire(
	ctx context.Context,
) (*heldMigrationLease, time.Duration, error) {
	if coordinator == nil || coordinator.state == nil {
		return nil, 0, fmt.Errorf("project ledger migration coordinator is unavailable")
	}
	started := time.Now()
	select {
	case <-coordinator.state.gate:
	case <-ctx.Done():
		return nil, time.Since(started), migrationLeaseContextError(ctx)
	}
	releaseProcessGate := func() {
		coordinator.state.gate <- struct{}{}
	}
	for {
		lease, acquired, err := tryAcquireMigrationFileLease(
			coordinator.directory,
			migrationLeaseFilename,
		)
		if err != nil {
			releaseProcessGate()
			return nil, time.Since(started), err
		}
		if acquired {
			return &heldMigrationLease{
				file:  lease,
				state: coordinator.state,
			}, time.Since(started), nil
		}
		timer := time.NewTimer(migrationLeasePollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			releaseProcessGate()
			return nil, time.Since(started), migrationLeaseContextError(ctx)
		case <-timer.C:
		}
	}
}

func migrationLeaseContextError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.Join(ErrMigrationLeaseTimeout, ctx.Err())
	}
	return fmt.Errorf("wait for project ledger migration lease: %w", ctx.Err())
}

func (lease *heldMigrationLease) release() error {
	if lease == nil || lease.released {
		return nil
	}
	lease.released = true
	fileErr := lease.file.release()
	lease.state.gate <- struct{}{}
	lease.file = nil
	lease.state = nil
	return fileErr
}
