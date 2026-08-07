package projectmemory

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/architecturep2s"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// BuildCurrentArchitectureP2S composes the internal P12A read model from one
// already-correlated current project read frame. It performs no I/O and has no
// public memory-response, persistence, lifecycle, TypeEnv-head, or authority
// effect. The boolean is false only when the exact concern is absent from the
// requested bounded context.
func BuildCurrentArchitectureP2S(
	frame typedmemorystore.CurrentProjectReadFrame,
	entityOfConcern typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
) (architecturep2s.ReadModel, bool, error) {
	correlated, err := typedmemorystore.NewCurrentProjectReadFrame(
		frame.Snapshot(),
		frame.EntityDirectory(),
		frame.GraphObservation(),
	)
	if err != nil {
		return architecturep2s.ReadModel{}, false, fmt.Errorf(
			"current architecture P2S frame: %w",
			err,
		)
	}
	if entityOfConcern.RefKind().ID().String() != "U.EntityRef" {
		return architecturep2s.ReadModel{}, false, fmt.Errorf(
			"current architecture P2S requires an exact U.EntityRef concern",
		)
	}
	if !currentArchitectureP2SConcernExists(
		correlated.EntityDirectory(),
		entityOfConcern,
		context,
	) {
		return architecturep2s.ReadModel{}, false, nil
	}
	concern, err := architecturep2s.NewReference(
		entityOfConcern.RefKind().ID().String(),
		entityOfConcern.ReferenceID().String(),
	)
	if err != nil {
		return architecturep2s.ReadModel{}, false, err
	}
	graph := correlated.GraphObservation()
	basis, err := architecturep2s.NewProjectionBasis(
		architecturep2s.ProjectionBasisInput{
			Project:         correlated.Snapshot().ProjectID().String(),
			EntityOfConcern: concern,
			Context:         context.String(),
			TypeEnv:         graph.ActiveTypeEnv().String(),
			GraphSnapshot:   graph.GraphSnapshotBasis().Ref().String(),
			GraphRevision: graph.GraphSnapshotBasis().
				GraphRevision().Value(),
		},
	)
	if err != nil {
		return architecturep2s.ReadModel{}, false, err
	}
	claims, err := currentArchitectureP2SClaims(graph, context)
	if err != nil {
		return architecturep2s.ReadModel{}, false, err
	}
	model, err := architecturep2s.Compose(
		architecturep2s.ComposeInput{
			Basis:  basis,
			Claims: claims,
		},
		architecturep2s.HaftV9RuleSet(),
	)
	if err != nil {
		return architecturep2s.ReadModel{}, false, err
	}
	return model, true, nil
}

func currentArchitectureP2SConcernExists(
	directory typedmemorystore.CurrentEntityDirectory,
	entityOfConcern typedmemory.PersistedRef,
	context typedmemory.BoundedContextRef,
) bool {
	for _, entry := range directory.Entries() {
		if entry.Entity().String() !=
			entityOfConcern.ReferenceID().String() {
			continue
		}
		if entry.Context() == context {
			return true
		}
	}
	return false
}

func currentArchitectureP2SClaims(
	graph typedmemorystore.CurrentProjectGraphObservation,
	context typedmemory.BoundedContextRef,
) ([]architecturep2s.ObservedClaim, error) {
	activeAssertions := graph.ActiveAssertions().Relations()
	result := make([]architecturep2s.ObservedClaim, 0, len(activeAssertions))
	for _, active := range activeAssertions {
		carrier := active.Carrier()
		if carrier.Context() != context {
			continue
		}
		references, err := currentArchitectureP2SReferences(
			carrier.Bindings(),
		)
		if err != nil {
			return nil, err
		}
		if len(references) == 0 {
			continue
		}
		modality, err := currentArchitectureP2SModality(carrier)
		if err != nil {
			return nil, err
		}
		claim, err := architecturep2s.NewObservedClaim(
			architecturep2s.ObservedClaimInput{
				AssertionID: active.AssertionID().String(),
				Signature:   carrier.Signature().ID().String(),
				Context:     carrier.Context().String(),
				TypeEnv:     carrier.Signature().TypeEnv().String(),
				Modality:    modality,
				Provenance:  carrier.Provenance().String(),
				OriginEvent: active.OriginEvent().String(),
				References:  references,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"current architecture P2S assertion %q: %w",
				active.AssertionID().String(),
				err,
			)
		}
		result = append(result, claim)
	}
	return result, nil
}

func currentArchitectureP2SReferences(
	bindings []typedmemory.SlotBinding,
) ([]architecturep2s.Reference, error) {
	byKey := make(map[string]architecturep2s.Reference)
	for _, binding := range bindings {
		for _, filler := range binding.Fillers() {
			referenceFiller, ok := filler.(typedmemory.ReferenceFiller)
			if !ok {
				continue
			}
			persisted := referenceFiller.Reference()
			reference, err := architecturep2s.NewReference(
				persisted.RefKind().ID().String(),
				persisted.ReferenceID().String(),
			)
			if err != nil {
				return nil, err
			}
			byKey[reference.Key()] = reference
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]architecturep2s.Reference, 0, len(keys))
	for _, key := range keys {
		result = append(result, byKey[key])
	}
	return result, nil
}

func currentArchitectureP2SModality(
	carrier typedmemorystore.CurrentAssertionCarrier,
) (architecturep2s.ClaimModality, error) {
	switch value := carrier.(type) {
	case typedmemorystore.CurrentLegacyRelation:
		return architecturep2s.ClaimLegacyUnqualified, nil
	case typedmemorystore.CurrentRelationalAssertion:
		return currentArchitectureP2SExplicitModality(
			value.Assertion().Modality().Kind(),
		)
	default:
		return "", fmt.Errorf(
			"current architecture P2S assertion carrier %T is unsupported",
			carrier,
		)
	}
}

func currentArchitectureP2SExplicitModality(
	modality typedmemory.AssertionModalityKind,
) (architecturep2s.ClaimModality, error) {
	switch modality {
	case typedmemory.AssertionModalityAffirmsObtaining:
		return architecturep2s.ClaimAffirmsObtaining, nil
	case typedmemory.AssertionModalityDeniesObtaining:
		return architecturep2s.ClaimDeniesObtaining, nil
	case typedmemory.AssertionModalityObtainingUnknown:
		return architecturep2s.ClaimObtainingUnknown, nil
	default:
		return "", fmt.Errorf(
			"current architecture P2S assertion modality %q is unsupported",
			modality,
		)
	}
}
