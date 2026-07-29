//go:build darwin

package specmigrationv2

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func renameNoReplace(source string, destination string) error {
	if err := unix.RenamexNp(source, destination, unix.RENAME_EXCL); err != nil {
		return fmt.Errorf("install migration carrier without replacement: %w", err)
	}
	return nil
}
