package problemcardadapter

import "github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"

// Adapt is the pure ProblemCard candidate producer. It neither selects nor
// mints an EntityOfConcern, and it performs no validation, admission, storage,
// lifecycle, authority, CLI, or MCP effect.
func Adapt(
	draft Draft,
	runtime RuntimeBasis,
	concern ConcernBinding,
) Result {
	contract := currentContract()
	return recordatconcern.Adapt(
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
	contract, err := recordatconcern.NewProblemCardContract(
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	if err != nil {
		return recordatconcern.Contract{}
	}
	return contract
}
