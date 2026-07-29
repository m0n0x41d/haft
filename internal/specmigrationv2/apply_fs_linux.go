//go:build linux

package specmigrationv2

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func renameNoReplace(source string, destination string) error {
	err := unix.Renameat2(
		unix.AT_FDCWD,
		source,
		unix.AT_FDCWD,
		destination,
		unix.RENAME_NOREPLACE,
	)
	if err != nil {
		return fmt.Errorf("install migration carrier without replacement: %w", err)
	}
	return nil
}
