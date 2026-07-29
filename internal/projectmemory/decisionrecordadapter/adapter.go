package decisionrecordadapter

import "github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"

// Adapt projects an existing manual DecisionRecord into one typed
// DecisionChoiceAtConcern candidate. It performs no decision, supersession,
// admission, storage, authority, WorkCommission, CLI, or MCP effect.
func Adapt(
	draft Draft,
	runtime RuntimeBasis,
) Result {
	contract := currentContract()
	return recordatconcern.AdaptDecisionProjection(
		contract,
		draft,
		runtime,
	)
}

func currentContract() recordatconcern.Contract {
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		return recordatconcern.Contract{}
	}
	contract, err := recordatconcern.NewDecisionRecordContract(
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	if err != nil {
		return recordatconcern.Contract{}
	}
	return contract
}
