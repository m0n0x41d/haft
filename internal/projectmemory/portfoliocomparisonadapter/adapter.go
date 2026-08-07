// Package portfoliocomparisonadapter exposes the PortfolioComparison-shaped
// facade over the source-exact project-record-at-concern computational core.
package portfoliocomparisonadapter

import "github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"

// Adapt emits an explicit comparison relation with compared and
// non-dominated option sets. It performs no winner selection, DecisionRecord
// creation, admission, storage, lifecycle, authority, CLI, or MCP effect.
func Adapt(
	draft Draft,
	runtime RuntimeBasis,
	concern ConcernBinding,
) Result {
	contract := currentContract()
	return recordatconcern.AdaptPortfolioComparison(
		contract,
		draft,
		runtime,
		concern,
	)
}

func currentContract() recordatconcern.Contract {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		return recordatconcern.Contract{}
	}
	contract, err := recordatconcern.NewPortfolioComparisonContract(
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	if err != nil {
		return recordatconcern.Contract{}
	}
	return contract
}
