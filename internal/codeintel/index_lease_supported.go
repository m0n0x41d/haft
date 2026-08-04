//go:build darwin || linux

package codeintel

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type unixCodeIndexLease struct {
	file *os.File
}

type codeIndexLease interface {
	release() error
}

func tryAcquireCodeIndexLease(
	directory string,
	name string,
) (codeIndexLease, bool, error) {
	directoryFD, err := unix.Open(
		directory,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		0,
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"open code-index runtime directory without following symlinks: %w",
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
	if err != nil {
		return nil, false, fmt.Errorf(
			"open code-index rebuild lease without following symlinks: %w",
			err,
		)
	}
	path := directory + string(os.PathSeparator) + name
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Openat returned a valid nonnegative descriptor.
	if err := requireRegularAttachedCodeIndexLease(directoryFD, name, file); err != nil {
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
		return nil, false, fmt.Errorf("lock code-index rebuild lease: %w", err)
	}
	if err := requireRegularAttachedCodeIndexLease(directoryFD, name, file); err != nil {
		_ = unix.Flock(fd, unix.LOCK_UN)
		_ = file.Close()
		return nil, false, err
	}
	return &unixCodeIndexLease{file: file}, true, nil
}

func requireRegularAttachedCodeIndexLease(
	directoryFD int,
	name string,
	file *os.File,
) error {
	if file == nil {
		return fmt.Errorf("code-index rebuild lease file is nil")
	}
	var held unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &held); err != nil { // #nosec G115 -- file wraps the descriptor returned by unix.Openat.
		return fmt.Errorf("inspect held code-index rebuild lease: %w", err)
	}
	if held.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("held code-index rebuild lease is not a regular file")
	}
	effectiveUID := unix.Geteuid()
	if effectiveUID < 0 {
		return fmt.Errorf("effective user ID is outside the supported range")
	}
	expectedUID := uint32(effectiveUID) // #nosec G115 -- Unix effective UIDs are nonnegative uid_t values.
	if !safeCodeIndexLeaseMetadata(
		uint32(held.Mode),
		uint64(held.Nlink),
		held.Uid,
		expectedUID,
	) {
		return fmt.Errorf("code-index rebuild lease ownership or mode is unsafe")
	}
	var attached unix.Stat_t
	if err := unix.Fstatat(
		directoryFD,
		name,
		&attached,
		unix.AT_SYMLINK_NOFOLLOW,
	); err != nil {
		return fmt.Errorf("inspect attached code-index rebuild lease: %w", err)
	}
	if attached.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("attached code-index rebuild lease is not a regular file")
	}
	if held.Dev != attached.Dev || held.Ino != attached.Ino {
		return fmt.Errorf("code-index rebuild lease identity changed while opening")
	}
	return nil
}

func safeCodeIndexLeaseMetadata(
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

func (lease *unixCodeIndexLease) release() error {
	if lease == nil || lease.file == nil {
		return nil
	}
	unlockErr := unix.Flock(int(lease.file.Fd()), unix.LOCK_UN) // #nosec G115 -- file wraps the descriptor returned by unix.Openat.
	closeErr := lease.file.Close()
	lease.file = nil
	return errors.Join(unlockErr, closeErr)
}
