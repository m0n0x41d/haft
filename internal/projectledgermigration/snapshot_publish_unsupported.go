//go:build !darwin && !linux

package projectledgermigration

import "fmt"

func publishServeMigrationSnapshot(
	string,
	string,
) error {
	return fmt.Errorf(
		"atomic no-replace snapshot publication is unavailable on this platform",
	)
}
