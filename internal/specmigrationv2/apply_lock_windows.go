//go:build windows

package specmigrationv2

import "fmt"

type migrationLock struct{}

func acquireMigrationLock(_ string) (migrationLock, error) {
	return migrationLock{}, fmt.Errorf("migration apply lock is unsupported on Windows")
}

func (migrationLock) close() error {
	return nil
}
