//go:build !darwin && !linux

package projectledgermigration

import (
	"fmt"
	"runtime"
)

type migrationFileLease interface {
	release() error
}

func tryAcquireMigrationFileLease(
	directory string,
	name string,
) (migrationFileLease, bool, error) {
	return nil, false, fmt.Errorf(
		"project ledger migration lease is unsupported on %s for %s/%s",
		runtime.GOOS,
		directory,
		name,
	)
}
