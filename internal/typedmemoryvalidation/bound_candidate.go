package typedmemoryvalidation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

// EvaluateCandidate validates a freshly produced typed candidate under the
// current v2 admission contract against the same sealed basis service as the
// strict wire path. It never infers a version from candidate content. The
// caller supplies no TypeEnv, codec registry, or snapshot, and the
// server-resolved basis remains authoritative. Historical v1 admission is
// reachable only from an exact decoded request and its replay identity.
func (service Service) EvaluateCandidate(
	selector typedmemorywire.BasisSelector,
	candidate typedmemory.MemoryChangeSet,
) Outcome {
	request, err := newBoundCandidateRequest(
		typedmemorywire.ContractVersionV2,
		selector,
		candidate,
	)
	if err != nil {
		return invalidRequestOutcome(typedmemorywire.ContractVersionV2)
	}
	return service.evaluate(request)
}

type boundCandidateRequest struct {
	contractVersion string
	selector        typedmemorywire.BasisSelector
	candidate       typedmemory.MemoryChangeSet
}

func newBoundCandidateRequest(
	contractVersion string,
	selector typedmemorywire.BasisSelector,
	candidate typedmemory.MemoryChangeSet,
) (boundCandidateRequest, error) {
	if contractVersion != typedmemorywire.ContractVersionV2 {
		return boundCandidateRequest{}, fmt.Errorf(
			"bound candidate requires the current admission contract version",
		)
	}
	if selector == nil {
		return boundCandidateRequest{}, fmt.Errorf("bound candidate basis selector is required")
	}
	if _, err := candidate.Digest(); err != nil {
		return boundCandidateRequest{}, fmt.Errorf("bound candidate is invalid: %w", err)
	}
	return boundCandidateRequest{
		contractVersion: contractVersion,
		selector:        selector,
		candidate:       candidate,
	}, nil
}

func (request boundCandidateRequest) ContractVersion() string {
	return request.contractVersion
}

func (boundCandidateRequest) Action() string { return "validate" }

func (request boundCandidateRequest) Basis() typedmemorywire.BasisSelector {
	return request.selector
}

func (request boundCandidateRequest) ChangeCount() int {
	return len(request.candidate.Changes())
}

func (request boundCandidateRequest) BindChangeSet(
	_ typedmemory.TypeEnvRef,
) (typedmemory.MemoryChangeSet, error) {
	if _, err := request.candidate.Digest(); err != nil {
		return typedmemory.MemoryChangeSet{}, err
	}
	return request.candidate, nil
}

func (boundCandidateRequest) usesSemanticDiagnosticPaths() {}
