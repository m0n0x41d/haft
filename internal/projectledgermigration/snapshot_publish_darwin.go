//go:build darwin

package projectledgermigration

import "golang.org/x/sys/unix"

func publishServeMigrationSnapshot(
	partialPath string,
	finalPath string,
) error {
	return unix.RenamexNp(
		partialPath,
		finalPath,
		unix.RENAME_EXCL,
	)
}
