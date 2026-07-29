// Package adaptersource defines the edition-sensitive source posture shared by
// project-memory candidate producers. It separates current C.3
// KindClassification delivery from sealed historical MemberOf delivery without
// making either posture an admission authority.
package adaptersource

import "fmt"

type modeKind uint8

const (
	modeInvalid modeKind = iota
	modeHistoricalMembership
	modeCurrentKindClassification
)

// Mode is a closed, non-zero adapter source posture. Its constructors are the
// only way to obtain a valid value, so an adapter cannot silently fall back
// from current classification to historical membership.
type Mode struct {
	kind modeKind
}

func HistoricalMembership() Mode {
	return Mode{kind: modeHistoricalMembership}
}

func CurrentKindClassification() Mode {
	return Mode{kind: modeCurrentKindClassification}
}

func (mode Mode) IsHistoricalMembership() bool {
	return mode.kind == modeHistoricalMembership
}

func (mode Mode) IsCurrentKindClassification() bool {
	return mode.kind == modeCurrentKindClassification
}

func (mode Mode) Verify() error {
	if mode.IsHistoricalMembership() || mode.IsCurrentKindClassification() {
		return nil
	}
	return fmt.Errorf("project-memory adapter source mode is invalid")
}
