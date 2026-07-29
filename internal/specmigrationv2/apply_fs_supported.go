//go:build darwin || linux

package specmigrationv2

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func openExclusiveNoFollow(path string, mode os.FileMode) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_CLOEXEC | unix.O_NOFOLLOW
	permissions := mode.Perm()
	unixMode := uint32(permissions)
	fd, err := unix.Open(path, flags, unixMode)
	if err != nil {
		return nil, fmt.Errorf("create exclusive migration file: %w", err)
	}
	return adoptOpenedFile(fd, path)
}

func openReadOnlyNoFollow(path string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open migration file without following symlinks: %w", err)
	}
	file, err := adoptOpenedFile(fd, path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect migration file: %w", err)
	}
	mode := info.Mode()
	if !mode.IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("migration carrier %s is not a regular file", path)
	}
	return file, nil
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open migration directory without following symlinks: %w", err)
	}
	return adoptOpenedFile(fd, path)
}

func adoptOpenedFile(fd int, path string) (*os.File, error) {
	return adoptOpenedFileWith(fd, path, os.NewFile, unix.Close)
}

func adoptOpenedFileWith(
	fd int,
	path string,
	newFile func(uintptr, string) *os.File,
	closeDescriptor func(int) error,
) (*os.File, error) {
	if fd < 0 {
		return nil, fmt.Errorf("adopt opened migration descriptor %d: descriptor is negative", fd)
	}
	// #nosec G115 -- unix.Open returned a non-negative descriptor, which is
	// representable as uintptr on every supported Unix target.
	fileDescriptor := uintptr(fd)
	file := newFile(fileDescriptor, path)
	if file != nil {
		return file, nil
	}
	if err := closeDescriptor(fd); err != nil {
		return nil, fmt.Errorf(
			"adopt opened migration descriptor %d: os.NewFile returned nil and close failed: %w",
			fd,
			err,
		)
	}
	return nil, fmt.Errorf("adopt opened migration descriptor %d: os.NewFile returned nil", fd)
}

func syncDirectoryNoFollow(path string) error {
	directory, err := openDirectoryNoFollow(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func sameFilesystem(paths []string) error {
	if len(paths) < 2 {
		return nil
	}
	var first unix.Stat_t
	if err := unix.Stat(paths[0], &first); err != nil {
		return fmt.Errorf("inspect migration filesystem topology: %w", err)
	}
	for _, path := range paths[1:] {
		var current unix.Stat_t
		if err := unix.Stat(path, &current); err != nil {
			return fmt.Errorf("inspect migration filesystem topology: %w", err)
		}
		if current.Dev != first.Dev {
			return fmt.Errorf("migration carriers cross filesystem devices")
		}
	}
	return nil
}
