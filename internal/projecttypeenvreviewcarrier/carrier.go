package projecttypeenvreviewcarrier

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	DirectoryName = ".haft"
	FileName      = "project-typeenv-genesis-review.json"
	MaximumBytes  = 4 << 20
)

// Digest is the exact-byte identity of a Genesis review carrier.
type Digest struct {
	value       [sha256.Size]byte
	initialized bool
}

func ParseDigest(value string) (Digest, error) {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return Digest{}, fmt.Errorf("genesis review digest must use the sha256: prefix")
	}
	encoded := strings.TrimPrefix(value, prefix)
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return Digest{}, fmt.Errorf("decode Genesis review digest: %w", err)
	}
	if len(decoded) != sha256.Size {
		return Digest{}, fmt.Errorf(
			"genesis review digest must contain %d bytes",
			sha256.Size,
		)
	}
	var digest Digest
	copy(digest.value[:], decoded)
	digest.initialized = true
	return digest, nil
}

func (digest Digest) String() string {
	if !digest.initialized {
		return ""
	}
	return "sha256:" + hex.EncodeToString(digest.value[:])
}

func (digest Digest) valid() bool {
	return digest.initialized
}

// Carrier is an immutable, size-checked exact-byte review carrier.
type Carrier struct {
	content []byte
	digest  Digest
}

func NewCarrier(content []byte) (Carrier, error) {
	if len(content) > MaximumBytes {
		return Carrier{}, fmt.Errorf(
			"genesis review carrier exceeds %d bytes",
			MaximumBytes,
		)
	}
	owned := append([]byte(nil), content...)
	sum := sha256.Sum256(owned)
	return Carrier{
		content: owned,
		digest: Digest{
			value:       sum,
			initialized: true,
		},
	}, nil
}

func (carrier Carrier) Bytes() []byte {
	return append([]byte(nil), carrier.content...)
}

func (carrier Carrier) Digest() Digest {
	return carrier.digest
}

func (carrier Carrier) valid() error {
	if len(carrier.content) > MaximumBytes {
		return fmt.Errorf("genesis review carrier is invalid")
	}
	sum := sha256.Sum256(carrier.content)
	expected := Digest{
		value:       sum,
		initialized: true,
	}
	if carrier.digest != expected {
		return fmt.Errorf("genesis review carrier digest does not match its bytes")
	}
	return nil
}

// InstallationResult is a closed effect result. Callers must distinguish a
// newly installed carrier, an exact idempotent reuse, a retained conflict, and
// a post-linearization outcome that requires an exact-same-proposal retry.
type InstallationResult interface {
	isInstallationResult()
}

type Created struct {
	Carrier Carrier
}

func (Created) isInstallationResult() {}

type Reused struct {
	Carrier Carrier
}

func (Reused) isInstallationResult() {}

type Conflict struct {
	Current     CurrentState
	Proposed    Digest
	Expectation Expectation
}

func (Conflict) isInstallationResult() {}

// OutcomeUnknown means the atomic namespace effect may already have installed
// Proposed, but the package could not prove its durable exact-byte outcome.
// Retry identifies the only idempotent recovery operation.
type OutcomeUnknown struct {
	Proposed Digest
	Failure  string
	Retry    ExactSameProposalRetry
	Cleanup  CleanupDisposition
}

func (OutcomeUnknown) isInstallationResult() {}

// ExactSameProposalRetry is a closed retry coordinate. It preserves whether
// the unresolved namespace effect was an Install or an expected-digest Replace.
type ExactSameProposalRetry interface {
	isExactSameProposalRetry()
	ProposedDigest() Digest
	Instruction() string
}

type ExactInstallRetry struct {
	Proposed Digest
}

func (ExactInstallRetry) isExactSameProposalRetry() {}

func (retry ExactInstallRetry) ProposedDigest() Digest {
	return retry.Proposed
}

func (retry ExactInstallRetry) Instruction() string {
	return "retry Install with the exact same proposal bytes identified by " +
		retry.Proposed.String()
}

type ExactReplaceRetry struct {
	Expected Digest
	Proposed Digest
}

func (ExactReplaceRetry) isExactSameProposalRetry() {}

func (retry ExactReplaceRetry) ProposedDigest() Digest {
	return retry.Proposed
}

func (retry ExactReplaceRetry) Instruction() string {
	return "retry Replace with the original expected digest " +
		retry.Expected.String() +
		" and the exact same proposal bytes identified by " +
		retry.Proposed.String()
}

// CleanupDisposition keeps non-canonical stage residue separate from the
// canonical carrier outcome.
type CleanupDisposition interface {
	isCleanupDisposition()
}

type NoKnownCleanupDebt struct{}

func (NoKnownCleanupDebt) isCleanupDisposition() {}

type PossibleOrphanStage struct {
	Name string
}

func (PossibleOrphanStage) isCleanupDisposition() {}

// CurrentState makes absence explicit in a conflict instead of using null.
type CurrentState interface {
	isCurrentState()
}

type Missing struct{}

func (Missing) isCurrentState() {}

type Present struct {
	Carrier Carrier
}

func (Present) isCurrentState() {}

// Expectation records the condition that prevented a silent overwrite.
type Expectation interface {
	isExpectation()
}

type MustBeAbsent struct{}

func (MustBeAbsent) isExpectation() {}

type MustMatch struct {
	Digest Digest
}

func (MustMatch) isExpectation() {}
