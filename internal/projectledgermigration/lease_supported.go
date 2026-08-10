//go:build darwin || linux

package projectledgermigration

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

type migrationFileLease interface {
	release() error
}

type unixMigrationFileLease struct {
	file *os.File
}

func tryAcquireMigrationFileLease(
	directory string,
	name string,
) (migrationFileLease, bool, error) {
	directoryFD, err := unix.Open(
		directory,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"open project ledger directory without following symlinks: %w",
			err,
		)
	}
	defer func() {
		_ = unix.Close(directoryFD)
	}()
	flags := unix.O_RDWR |
		unix.O_CREAT |
		unix.O_CLOEXEC |
		unix.O_NOFOLLOW |
		unix.O_NONBLOCK
	fd, err := unix.Openat(directoryFD, name, flags, 0o600)
	if runtime.GOOS == "darwin" && errors.Is(err, unix.ENOENT) {
		// Darwin can expose a transient negative pathname lookup while another
		// process attaches the newly created carrier. Treat it as contention:
		// the bounded coordinator retries and every successful open still passes
		// the regular-file, owner, mode, link-count, and inode checks below.
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf(
			"open project ledger migration lease without following symlinks: %w",
			err,
		)
	}
	path := directory + string(os.PathSeparator) + name
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
	if err := requireRegularAttachedMigrationLease(
		directoryFD,
		name,
		file,
	); err != nil {
		_ = file.Close()
		return nil, false, err
	}
	err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		_ = file.Close()
		return nil, false, nil
	}
	if err != nil {
		_ = file.Close()
		return nil, false, fmt.Errorf(
			"lock project ledger migration lease: %w",
			err,
		)
	}
	if err := requireRegularAttachedMigrationLease(
		directoryFD,
		name,
		file,
	); err != nil {
		_ = unix.Flock(fd, unix.LOCK_UN)
		_ = file.Close()
		return nil, false, err
	}
	return &unixMigrationFileLease{file: file}, true, nil
}

func requireRegularAttachedMigrationLease(
	directoryFD int,
	name string,
	file *os.File,
) error {
	if file == nil {
		return fmt.Errorf("project ledger migration lease file is nil")
	}
	var held unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &held); err != nil { // #nosec G115 -- file wraps the descriptor returned by unix.Openat.
		return fmt.Errorf("inspect held project ledger migration lease: %w", err)
	}
	effectiveUID := unix.Geteuid()
	if effectiveUID < 0 {
		return fmt.Errorf("effective user ID is outside the supported range")
	}
	expectedUID := uint32(effectiveUID) // #nosec G115 -- Unix effective UIDs are nonnegative uid_t values.
	if !safeMigrationLeaseMetadata(
		uint32(held.Mode),
		uint64(held.Nlink),
		held.Uid,
		expectedUID,
	) {
		return fmt.Errorf(
			"project ledger migration lease ownership or mode is unsafe",
		)
	}
	var attached unix.Stat_t
	if err := unix.Fstatat(
		directoryFD,
		name,
		&attached,
		unix.AT_SYMLINK_NOFOLLOW,
	); err != nil {
		return fmt.Errorf(
			"inspect attached project ledger migration lease: %w",
			err,
		)
	}
	if attached.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf(
			"attached project ledger migration lease is not a regular file",
		)
	}
	if held.Dev != attached.Dev || held.Ino != attached.Ino {
		return fmt.Errorf(
			"project ledger migration lease identity changed while opening",
		)
	}
	return nil
}

func safeMigrationLeaseMetadata(
	mode uint32,
	linkCount uint64,
	ownerUID uint32,
	expectedUID uint32,
) bool {
	return mode&unix.S_IFMT == unix.S_IFREG &&
		mode&0o777 == 0o600 &&
		mode&(unix.S_ISUID|unix.S_ISGID|unix.S_ISVTX) == 0 &&
		linkCount == 1 &&
		ownerUID == expectedUID
}

func (lease *unixMigrationFileLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	unlockErr := unix.Flock(int(lease.file.Fd()), unix.LOCK_UN) // #nosec G115 -- file wraps the descriptor returned by unix.Openat.
	closeErr := lease.file.Close()
	lease.file = nil
	return errors.Join(unlockErr, closeErr)
}
