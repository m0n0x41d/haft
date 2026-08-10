//go:build linux

package projectledgermigration

import "golang.org/x/sys/unix"

func publishServeMigrationSnapshot(
	partialPath string,
	finalPath string,
) error {
	return unix.Renameat2(
		unix.AT_FDCWD,
		partialPath,
		unix.AT_FDCWD,
		finalPath,
		unix.RENAME_NOREPLACE,
	)
}
