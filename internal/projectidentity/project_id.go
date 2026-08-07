// Package projectidentity owns the effect-free canonical identity of one Haft
// project. Project-ledger effects establish and verify this identity; pure
// consumers may carry it without importing the SQLite-backed ledger package.
package projectidentity

import (
	"fmt"
	"regexp"
	"strings"
)

var projectIDPattern = regexp.MustCompile(`^qnt_[0-9a-f]{8}$`)

// ProjectID is the one canonical project identity value used across Haft.
// It is deliberately defined outside projectledger so importing the identity
// does not also import ledger I/O or a database driver.
type ProjectID struct {
	value string
}

func ParseProjectID(raw string) (ProjectID, error) {
	if raw != strings.TrimSpace(raw) || !projectIDPattern.MatchString(raw) {
		return ProjectID{}, fmt.Errorf("project ID must match canonical qnt_<8 lowercase hex digits> syntax")
	}
	return ProjectID{value: raw}, nil
}

func (id ProjectID) String() string {
	return id.value
}
