package projectledger

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

var ErrSQLiteSidecarGenerationChanged = errors.New(
	"project ledger SQLite sidecar generation changed; close and reopen the project ledger",
)

type sqliteSidecarIdentity struct {
	path    string
	present bool
	info    os.FileInfo
}

// sqliteSidecarGeneration observes the pathname identities of SQLite's WAL and
// shared-memory files. An absent sidecar may appear during ordinary first WAL
// activation and is adopted once. After a live handle has observed a sidecar,
// disappearance or inode replacement means that the handle and pathname no
// longer participate in one lock/journal generation. That exact observation is
// classified before another SQLite read; unrelated SQLite errors retain their
// original corruption or IO meaning.
type sqliteSidecarGeneration struct {
	mu       sync.Mutex
	captured bool
	wal      sqliteSidecarIdentity
	shm      sqliteSidecarIdentity
}

func newSQLiteSidecarGeneration(
	databasePath string,
) *sqliteSidecarGeneration {
	return &sqliteSidecarGeneration{
		wal: sqliteSidecarIdentity{
			path: databasePath + "-wal",
		},
		shm: sqliteSidecarIdentity{
			path: databasePath + "-shm",
		},
	}
}

func (generation *sqliteSidecarGeneration) Revalidate() error {
	if generation == nil {
		return fmt.Errorf(
			"project ledger SQLite sidecar generation is unavailable",
		)
	}
	observedWAL, err := observeSQLiteSidecar(generation.wal.path)
	if err != nil {
		return err
	}
	observedSHM, err := observeSQLiteSidecar(generation.shm.path)
	if err != nil {
		return err
	}

	generation.mu.Lock()
	defer generation.mu.Unlock()

	if !generation.captured {
		generation.wal = observedWAL
		generation.shm = observedSHM
		generation.captured = true
		return nil
	}
	if err := verifySQLiteSidecarIdentity(
		generation.wal,
		observedWAL,
	); err != nil {
		return err
	}
	if err := verifySQLiteSidecarIdentity(
		generation.shm,
		observedSHM,
	); err != nil {
		return err
	}
	generation.wal = adoptSQLiteSidecarIdentity(
		generation.wal,
		observedWAL,
	)
	generation.shm = adoptSQLiteSidecarIdentity(
		generation.shm,
		observedSHM,
	)
	return nil
}

func observeSQLiteSidecar(
	path string,
) (sqliteSidecarIdentity, error) {
	identity := sqliteSidecarIdentity{path: path}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return identity, nil
	}
	if err != nil {
		return sqliteSidecarIdentity{}, fmt.Errorf(
			"inspect project ledger SQLite sidecar %q: %w",
			path,
			err,
		)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return sqliteSidecarIdentity{}, fmt.Errorf(
			"%w: path %q no longer names a real regular file",
			ErrSQLiteSidecarGenerationChanged,
			path,
		)
	}
	identity.present = true
	identity.info = info
	return identity, nil
}

func verifySQLiteSidecarIdentity(
	captured sqliteSidecarIdentity,
	observed sqliteSidecarIdentity,
) error {
	if !captured.present {
		return nil
	}
	if !observed.present {
		return fmt.Errorf(
			"%w: captured path %q is now absent",
			ErrSQLiteSidecarGenerationChanged,
			captured.path,
		)
	}
	if os.SameFile(captured.info, observed.info) {
		return nil
	}
	return fmt.Errorf(
		"%w: captured path %q now names a different file",
		ErrSQLiteSidecarGenerationChanged,
		captured.path,
	)
}

func adoptSQLiteSidecarIdentity(
	captured sqliteSidecarIdentity,
	observed sqliteSidecarIdentity,
) sqliteSidecarIdentity {
	if captured.present || !observed.present {
		return captured
	}
	return observed
}
