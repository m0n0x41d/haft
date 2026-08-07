// Package specsectionadapter exposes the SpecSection-shaped façade over the
// source-exact record-at-concern computational core.
package specsectionadapter

import "github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"

// Adapt emits one specialized SpecSectionRecord at one exact concern. It does
// not approve, reopen, rebaseline, or otherwise change SpecSection lifecycle;
// it also performs no admission, storage, authority, CLI, or MCP effect.
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
	contract, err := recordatconcern.NewSpecSectionContract(
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	if err != nil {
		return recordatconcern.Contract{}
	}
	return contract
}
