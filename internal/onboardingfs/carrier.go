package onboardingfs

import "github.com/m0n0x41d/haft/internal/onboarding"

const (
	DirectoryName = ".haft"
	FileName      = "onboarding-memory-deferral.json"
	MaximumBytes  = 16 << 10
)

// ReadResult makes absence distinct from a present, strictly decoded carrier.
type ReadResult interface {
	readResultVariant()
}

type Absent struct{}

func (Absent) readResultVariant() {}

type Present struct {
	Deferral onboarding.MemoryDeferral
}

func (Present) readResultVariant() {}

// InstallResult closes the namespace effect. OutcomeUnknown is returned only
// after the canonical link may have been created; retrying Install with the
// exact same deferral resolves it as Reused when that effect occurred.
type InstallResult interface {
	installResultVariant()
}

type Created struct {
	Deferral onboarding.MemoryDeferral
}

func (Created) installResultVariant() {}

type Reused struct {
	Deferral onboarding.MemoryDeferral
}

func (Reused) installResultVariant() {}

type Conflict struct {
	Current  onboarding.MemoryDeferral
	Proposed onboarding.MemoryDeferral
}

func (Conflict) installResultVariant() {}

type OutcomeUnknown struct {
	Proposed onboarding.MemoryDeferral
	Failure  string
}

func (OutcomeUnknown) installResultVariant() {}

// ReopenResult closes removal of one exact reviewed disposition. A different
// current carrier is retained as a conflict. OutcomeUnknown is recoverable by
// retrying Reopen with the same expected deferral.
type ReopenResult interface {
	reopenResultVariant()
}

type AlreadyOpen struct{}

func (AlreadyOpen) reopenResultVariant() {}

type Reopened struct {
	Deferral onboarding.MemoryDeferral
}

func (Reopened) reopenResultVariant() {}

type ReopenConflict struct {
	Current  onboarding.MemoryDeferral
	Expected onboarding.MemoryDeferral
}

func (ReopenConflict) reopenResultVariant() {}

type ReopenOutcomeUnknown struct {
	Expected onboarding.MemoryDeferral
	Failure  string
}

func (ReopenOutcomeUnknown) reopenResultVariant() {}
