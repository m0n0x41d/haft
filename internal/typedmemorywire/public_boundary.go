package typedmemorywire

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ValidateStrictJSON applies the bounded, duplicate-rejecting JSON scan shared
// by typed-memory wire decoders. Adjacent public adapters use it before their
// own closed-object decode so the transport boundary has one resource and
// duplicate-field policy.
func ValidateStrictJSON(payload []byte) error {
	return scanStrictJSON(payload)
}

// NewExactProjectSelector constructs the trusted internal selector used after
// a current project basis has been observed. Public agent-facing tools do not
// accept these coordinates; orchestration adapters derive them from the
// project snapshot to preserve compare-and-swap semantics.
func NewExactProjectSelector(
	typeEnvDigest typedmemory.SHA256Digest,
	graphRevision typedmemory.GraphRevision,
) (ExactProjectSelector, error) {
	if typeEnvDigest.String() == "" {
		return ExactProjectSelector{}, fmt.Errorf(
			"exact project selector requires a TypeEnv digest",
		)
	}
	return ExactProjectSelector{
		requestedTypeEnvDigest: typeEnvDigest,
		requestedGraphRevision: graphRevision,
	}, nil
}
