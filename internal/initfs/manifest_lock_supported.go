//go:build darwin || linux

package initfs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type manifestLock struct {
	file *os.File
}

func tryAcquireManifestLock(
	path string,
) (*manifestLock, bool, error) {
	flags := unix.O_RDWR |
		unix.O_CREAT |
		unix.O_CLOEXEC |
		unix.O_NOFOLLOW |
		unix.O_NONBLOCK
	fd, err := unix.Open(path, flags, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open manifest lock without following symlinks: %w", err)
	}
	file := os.NewFile(uintptr(fd), path) // #nosec G115 -- unix.Open returned a valid nonnegative descriptor.
	if err := requireRegularAttachedManifestLock(path, file); err != nil {
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
		return nil, false, fmt.Errorf("lock manifest publication: %w", err)
	}
	if err := requireRegularAttachedManifestLock(path, file); err != nil {
		_ = unix.Flock(fd, unix.LOCK_UN)
		_ = file.Close()
		return nil, false, err
	}
	return &manifestLock{file: file}, true, nil
}

func requireRegularAttachedManifestLock(
	path string,
	file *os.File,
) error {
	if file == nil {
		return fmt.Errorf("manifest lock file is nil")
	}
	var held unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &held); err != nil { // #nosec G115 -- file wraps the descriptor returned by unix.Open above.
		return fmt.Errorf("inspect held manifest lock: %w", err)
	}
	if held.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("held manifest lock is not a regular file")
	}
	effectiveUID := unix.Geteuid()
	if effectiveUID < 0 {
		return fmt.Errorf("effective user ID is outside the supported range")
	}
	expectedUID := uint32(effectiveUID) // #nosec G115 -- Unix effective UIDs are nonnegative uid_t values.
	if held.Mode&0o777 != 0o600 ||
		held.Mode&(unix.S_ISUID|unix.S_ISGID|unix.S_ISVTX) != 0 ||
		held.Nlink != 1 ||
		held.Uid != expectedUID {
		return fmt.Errorf("held manifest lock ownership or mode is unsafe")
	}
	var attached unix.Stat_t
	if err := unix.Lstat(path, &attached); err != nil {
		return fmt.Errorf("inspect attached manifest lock: %w", err)
	}
	if attached.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("attached manifest lock is not a regular file")
	}
	if held.Dev != attached.Dev || held.Ino != attached.Ino {
		return fmt.Errorf("manifest lock identity changed while opening")
	}
	return nil
}

func (lock *manifestLock) release() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	unlockErr := unix.Flock(int(lock.file.Fd()), unix.LOCK_UN) // #nosec G115 -- file wraps the descriptor returned by unix.Open.
	closeErr := lock.file.Close()
	lock.file = nil
	return errors.Join(unlockErr, closeErr)
}
