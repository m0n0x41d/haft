//go:build !windows

package specmigrationv2

import (
	"fmt"
	"os"
	"syscall"
)

type migrationLock struct {
	file *os.File
}

func acquireMigrationLock(path string) (migrationLock, error) {
	file, err := openDirectoryNoFollow(path)
	if err != nil {
		return migrationLock{}, fmt.Errorf("open migration lock directory: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return migrationLock{}, fmt.Errorf("inspect migration lock directory: %w", err)
	}
	if !info.IsDir() {
		_ = file.Close()
		return migrationLock{}, fmt.Errorf("migration lock carrier is not a directory")
	}
	fd := file.Fd()
	if fd > (^uintptr(0) >> 1) {
		_ = file.Close()
		return migrationLock{}, fmt.Errorf("migration lock file descriptor exceeds int range")
	}
	err = syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB) // #nosec G115 -- range checked above.
	if err != nil {
		_ = file.Close()
		return migrationLock{}, fmt.Errorf("migration apply is already locked: %w", err)
	}
	return migrationLock{file: file}, nil
}

func (lock migrationLock) close() error {
	if lock.file == nil {
		return nil
	}
	fd := lock.file.Fd()
	if fd <= (^uintptr(0) >> 1) {
		_ = syscall.Flock(int(fd), syscall.LOCK_UN) // #nosec G115 -- range checked above.
	}
	return lock.file.Close()
}
