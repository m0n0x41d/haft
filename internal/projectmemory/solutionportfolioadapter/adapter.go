// Package solutionportfolioadapter exposes the SolutionPortfolio-shaped
// facade over the source-exact project-record-at-concern computational core.
package solutionportfolioadapter

import "github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"

// Adapt emits the explicit portfolio relation and preserves every supplied
// option. It performs no ranking, comparison, selection, admission, storage,
// lifecycle, authority, CLI, or MCP effect.
func Adapt(
	draft Draft,
	runtime RuntimeBasis,
	concern ConcernBinding,
) Result {
	contract := currentContract()
	return recordatconcern.AdaptSolutionPortfolio(
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
	contract, err := recordatconcern.NewSolutionPortfolioContract(
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	if err != nil {
		return recordatconcern.Contract{}
	}
	return contract
}
