//go:build !darwin && !linux

package specmigrationv2

import (
	"fmt"
	"os"
)

func openExclusiveNoFollow(_ string, _ os.FileMode) (*os.File, error) {
	return nil, fmt.Errorf("migration no-follow effects are unsupported on this platform")
}

func openReadOnlyNoFollow(_ string) (*os.File, error) {
	return nil, fmt.Errorf("migration no-follow effects are unsupported on this platform")
}

func openDirectoryNoFollow(_ string) (*os.File, error) {
	return nil, fmt.Errorf("migration no-follow directory access is unsupported on this platform")
}

func syncDirectoryNoFollow(_ string) error {
	return fmt.Errorf("migration durable directory sync is unsupported on this platform")
}

func sameFilesystem(_ []string) error {
	return fmt.Errorf("migration filesystem topology verification is unsupported on this platform")
}

func renameNoReplace(_ string, _ string) error {
	return fmt.Errorf("migration no-replace install is unsupported on this platform")
}
