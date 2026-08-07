package specsectionadapter

import (
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type DraftInput = recordatconcern.DraftInput
type Draft = recordatconcern.Draft

func NewDraft(input DraftInput) (Draft, error) {
	return recordatconcern.NewDraft(input)
}

type ClaimGraphBasis = recordatconcern.ClaimGraphBasis
type ExactClaimGraph = recordatconcern.ExactClaimGraph
type MissingClaimGraph = recordatconcern.MissingClaimGraph

func NewExactClaimGraph(
	graph typedmemory.ClaimGraphValue,
) (ExactClaimGraph, error) {
	return recordatconcern.NewExactClaimGraph(graph)
}

func NewMissingClaimGraph(
	missing []MissingBasis,
) (MissingClaimGraph, error) {
	return recordatconcern.NewMissingClaimGraph(missing)
}

type RuntimeBasis = recordatconcern.RuntimeBasis
type ExactRuntimeBasis = recordatconcern.ExactRuntimeBasis
type ExactRuntimeBasisBuilder = recordatconcern.ExactRuntimeBasisBuilder
type MissingRuntimeBasis = recordatconcern.MissingRuntimeBasis

func NewExactRuntimeBasisBuilder(
	project projectidentity.ProjectID,
) ExactRuntimeBasisBuilder {
	return recordatconcern.NewExactRuntimeBasisBuilder(project)
}

func NewMissingRuntimeBasis(
	missing []MissingBasis,
) (MissingRuntimeBasis, error) {
	return recordatconcern.NewMissingRuntimeBasis(missing)
}

type ConcernBinding = recordatconcern.ConcernBinding
type ExactConcernBinding = recordatconcern.ExactConcernBinding
type UnsettledConcernBinding = recordatconcern.UnsettledConcernBinding

func NewExactConcernBinding(
	resolution typedmemory.ResolvedStrongReference,
) (ExactConcernBinding, error) {
	return recordatconcern.NewExactConcernBinding(resolution)
}

func NewUnsettledConcernBinding(
	missing []MissingBasis,
) (UnsettledConcernBinding, error) {
	return recordatconcern.NewUnsettledConcernBinding(missing)
}

type MissingBasis = recordatconcern.MissingBasis

func NewMissingBasis(
	name string,
	repair typedmemory.RepairPointer,
) (MissingBasis, error) {
	return recordatconcern.NewMissingBasis(name, repair)
}

type Violation = recordatconcern.Violation
type Result = recordatconcern.Result
type ValidCandidate = recordatconcern.ValidCandidate
type Invalid = recordatconcern.Invalid
type Underdetermined = recordatconcern.Underdetermined
