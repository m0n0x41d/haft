package recordatconcern

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type resolvedMapping struct {
	fragment        typedmemory.TypedRelationDeclarationFragmentRef
	recordSlot      typedmemory.SlotKindID
	recordRefKind   typedmemory.RefKindRef
	concernSlot     typedmemory.SlotKindID
	concernRefKind  typedmemory.RefKindRef
	claimGraphSlot  typedmemory.SlotKindID
	claimGraphKind  typedmemory.ValueKindRef
	claimGraphShape typedmemory.ValueShapeRef
	claimGraphCodec typedmemory.CodecRef
	codec           typedmemory.CodecImplementation
}

// Adapt is a pure candidate producer. It never chooses or mints an
// EntityOfConcern, never validates/admit/stores the candidate, and returns no
// candidate bytes when the exact mapping or concern basis is absent.
func Adapt(
	contract Contract,
	draft Draft,
	runtime RuntimeBasis,
	concern ConcernBinding,
) Result {
	if !contract.valid() {
		return underdeterminedFor(
			"mapping_contract",
			"repair:reload-record-at-concern-contract",
		)
	}
	claimGraph, graphReady := draft.claimGraph.(ExactClaimGraph)
	if !graphReady {
		missing, ok := draft.claimGraph.(MissingClaimGraph)
		if !ok {
			return underdeterminedFor(
				"claim_graph",
				contract.definition.claimGraphRepair,
			)
		}
		return underdetermined{missing: missing.MissingBasis()}
	}
	exactRuntime, runtimeReady := runtime.(ExactRuntimeBasis)
	if !runtimeReady {
		missing, ok := runtime.(MissingRuntimeBasis)
		if !ok {
			return underdeterminedFor(
				"selected_type_environment",
				"repair:resolve-project-typeenv-head",
			)
		}
		return underdetermined{missing: missing.MissingBasis()}
	}
	if exactRuntime.project != draft.projectID {
		return invalidResult(
			"runtime_project_mismatch",
			"the selected runtime and record-at-concern draft belong to different projects",
		)
	}
	if exactRuntime.sourceMode.IsHistoricalMembership() {
		policy, err := exactRuntime.registration.EvaluateMappingPolicy(
			contract.manifest,
			contract.adapter,
		)
		if err != nil {
			return underdeterminedFor(
				"record_membership_registration",
				"repair:resolve-selected-record-membership-registration",
			)
		}
		if policy.Kind() != recordmembershipregistration.MappingAccepted {
			return underdeterminedFor(
				contract.definition.mappingRegistration,
				contract.definition.registrationRepair,
			)
		}
	}
	mapping, missing := resolveMapping(
		contract,
		exactRuntime.environment,
		exactRuntime.codecs,
		draft.contextSlice.Context(),
	)
	if len(missing) > 0 {
		return underdetermined{missing: missing}
	}
	exactConcern, concernReady := concern.(ExactConcernBinding)
	if !concernReady {
		unsettled, ok := concern.(UnsettledConcernBinding)
		if !ok {
			return underdeterminedFor(
				"entity_of_concern_resolution",
				"repair:resolve-entity-of-concern",
			)
		}
		return underdetermined{missing: unsettled.MissingBasis()}
	}
	if exactConcern.context != draft.contextSlice.Context() {
		return invalidResult(
			"concern_context_mismatch",
			"the exact EntityOfConcern resolution and record ContextSlice use different bounded contexts",
		)
	}
	if exactConcern.reference.RefKind() != mapping.concernRefKind {
		return invalidResult(
			"concern_reference_kind_mismatch",
			"the exact EntityOfConcern reference does not use the U.EntityRef required by the selected record-at-concern relation fragment",
		)
	}
	if exactConcern.reference.ReferenceID().String() != exactConcern.entity.String() {
		return invalidResult(
			"concern_reference_identity_mismatch",
			"the exact EntityOfConcern reference and stable EntityID do not name one identity",
		)
	}
	return buildCandidate(
		draft,
		claimGraph.Value(),
		exactConcern,
		mapping,
		contract,
	)
}

func buildCandidate(
	draft Draft,
	claimGraph typedmemory.ClaimGraphValue,
	concern ExactConcernBinding,
	mapping resolvedMapping,
	contract Contract,
) Result {
	recordReference, err := typedmemory.NewLocalRef(
		mapping.recordRefKind,
		draft.recordLocalRef,
	)
	if err != nil {
		return invalidResult("record_reference_invalid", err.Error())
	}
	claimResult := buildClaimGraphCandidate(contract, claimGraph, mapping)
	claimCandidate, ready := claimResult.(claimGraphCandidateReady)
	if !ready {
		rejected := claimResult.(claimGraphCandidateRejected)
		return rejected.result
	}
	bindings, err := buildRelationBindings(
		recordReference,
		concern.reference,
		claimCandidate.candidate,
		mapping,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "relation_binding_invalid"),
			err.Error(),
		)
	}
	return buildCandidateWithBindings(
		draft,
		mapping.fragment,
		bindings,
		contract,
	)
}

func buildCandidateWithBindings(
	draft Draft,
	fragment typedmemory.TypedRelationDeclarationFragmentRef,
	bindings []typedmemory.CandidateSlotBinding,
	contract Contract,
) Result {
	declaration, err := typedmemory.NewDeclareEntity(
		draft.recordEntity,
		draft.recordLocalRef,
		draft.contextSlice.Context(),
		draft.recordLabel,
		draft.provenance,
	)
	if err != nil {
		return invalidResult("record_declaration_invalid", err.Error())
	}
	modality := typedmemory.NewAffirmsObtaining()
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  draft.assertionID,
			Signature:  fragment,
			Slice:      draft.contextSlice,
			Modality:   modality,
			Bindings:   bindings,
			Provenance: draft.provenance,
		},
	)
	if err != nil {
		return invalidResult(diagnosticCode(contract, "relation_invalid"), err.Error())
	}
	assertRelation, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "relation_change_invalid"),
			err.Error(),
		)
	}
	changeSet, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration, assertRelation},
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "change_set_invalid"),
			err.Error(),
		)
	}
	carrier, err := recordcarrier.SealProjectRecordCarrierV1(
		draft.recordEntity,
		draft.contextSlice.Context(),
		mustRecordCarrierVariant(contract),
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "record_carrier_invalid"),
			err.Error(),
		)
	}
	binding, err := recordcarrier.SealEntityRecordCarrierBindingV1(
		draft.projectID,
		carrier,
		contract.manifest,
		contract.adapter,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "record_carrier_binding_invalid"),
			err.Error(),
		)
	}
	membershipSource, err := recordcarrier.SealRecordMembershipSourceV1(
		draft.projectID,
		draft.recordEntity,
		draft.contextSlice.Context(),
		carrier,
		binding,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "record_membership_source_invalid"),
			err.Error(),
		)
	}
	classificationSource, err := recordcarrier.SealRecordClassificationSourceV1(
		draft.projectID,
		draft.recordEntity,
		draft.contextSlice.Context(),
		carrier,
		binding,
	)
	if err != nil {
		return invalidResult(
			diagnosticCode(contract, "record_classification_source_invalid"),
			err.Error(),
		)
	}
	return validCandidateResult{
		changeSet:            changeSet,
		carrier:              carrier,
		binding:              binding,
		membershipSource:     membershipSource,
		classificationSource: classificationSource,
		manifest:             contract.manifest,
		adapter:              contract.adapter,
		signature:            fragment.ID(),
	}
}

type claimGraphCandidateResult interface {
	claimGraphCandidateResultVariant()
}

type claimGraphCandidateReady struct {
	candidate typedmemory.TypedValueCandidate
}

func (claimGraphCandidateReady) claimGraphCandidateResultVariant() {}

type claimGraphCandidateRejected struct {
	result Result
}

func (claimGraphCandidateRejected) claimGraphCandidateResultVariant() {}

func buildClaimGraphCandidate(
	contract Contract,
	graph typedmemory.ClaimGraphValue,
	mapping resolvedMapping,
) claimGraphCandidateResult {
	coreCodec, err := typedmemory.NewClaimGraphCodecV1(mapping.claimGraphShape)
	if err != nil {
		return claimGraphCandidateRejected{result: underdeterminedFor(
			"claim_graph_shape",
			contract.definition.mappingRepair,
		)}
	}
	encoded := coreCodec.EncodeInput(graph)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return claimGraphCandidateRejected{result: invalidResult(
			"claim_graph_invalid",
			"the project-record ClaimGraph is not accepted by the closed ClaimGraphCodecV1 algebra",
		)}
	}
	selected := mapping.codec.Canonicalize(
		mapping.claimGraphShape,
		canonical.CanonicalBytes(),
	)
	roundTrip, ok := selected.(typedmemory.CanonicalizedCodecValue)
	if !ok || !bytes.Equal(roundTrip.CanonicalBytes(), canonical.CanonicalBytes()) {
		return claimGraphCandidateRejected{result: underdeterminedFor(
			"selected_claim_graph_codec",
			"repair:reconcile-selected-claim-graph-codec",
		)}
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		mapping.claimGraphKind,
		mapping.claimGraphShape,
		mapping.claimGraphCodec,
		canonical.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		return claimGraphCandidateRejected{result: invalidResult(
			"claim_graph_candidate_invalid",
			err.Error(),
		)}
	}
	return claimGraphCandidateReady{candidate: candidate}
}

func buildRelationBindings(
	record typedmemory.LocalRef,
	concern typedmemory.PersistedRef,
	claim typedmemory.TypedValueCandidate,
	mapping resolvedMapping,
) ([]typedmemory.CandidateSlotBinding, error) {
	recordFiller, err := typedmemory.NewByReferenceCandidate(record)
	if err != nil {
		return nil, err
	}
	concernFiller, err := typedmemory.NewByReferenceCandidate(concern)
	if err != nil {
		return nil, err
	}
	claimFiller, err := typedmemory.NewByValueCandidate(claim)
	if err != nil {
		return nil, err
	}
	recordBinding, err := typedmemory.NewCandidateSlotBinding(
		mapping.recordSlot,
		[]typedmemory.CandidateSlotFiller{recordFiller},
	)
	if err != nil {
		return nil, err
	}
	concernBinding, err := typedmemory.NewCandidateSlotBinding(
		mapping.concernSlot,
		[]typedmemory.CandidateSlotFiller{concernFiller},
	)
	if err != nil {
		return nil, err
	}
	claimBinding, err := typedmemory.NewCandidateSlotBinding(
		mapping.claimGraphSlot,
		[]typedmemory.CandidateSlotFiller{claimFiller},
	)
	if err != nil {
		return nil, err
	}
	return []typedmemory.CandidateSlotBinding{
		recordBinding,
		concernBinding,
		claimBinding,
	}, nil
}

func resolveMapping(
	contract Contract,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	context typedmemory.BoundedContextRef,
) (resolvedMapping, []MissingBasis) {
	missing := make([]MissingBasis, 0)
	typeEnv := environment.Ref()
	fragmentID := mustSignatureID(contract.definition.signatureID)
	fragmentRef, err := typedmemory.NewTypedRelationDeclarationFragmentRef(
		typeEnv,
		fragmentID,
	)
	if err != nil {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			diagnosticCode(contract, "typed_relation_declaration_fragment"),
			contract.definition.mappingRepair,
		)}
	}
	fragment, found := environment.TypedRelationDeclarationFragment(fragmentRef)
	if !found || !fragmentAllowsContext(fragment, context) {
		missing = append(missing, mustMissingBasis(
			diagnosticCode(contract, "typed_relation_declaration_fragment"),
			"repair:select-typeenv-with-"+contract.definition.relationDiagnosticName+"-at-concern",
		))
		return resolvedMapping{}, missing
	}
	recordSlotIDValue := mustSlotKindID(contract.definition.recordSlotID)
	concernSlotIDValue := mustSlotKindID(contract.definition.concernSlotID)
	claimSlotIDValue := mustSlotKindID(contract.definition.claimGraphSlotID)
	recordTarget, recordReady := exactReferenceSlot(
		fragment,
		recordSlotIDValue,
		contract.definition.recordKindID,
		contract.definition.recordRefID,
	)
	concernTarget, concernReady := exactReferenceSlot(
		fragment,
		concernSlotIDValue,
		entityKindID,
		entityRefID,
	)
	claimTarget, claimReady := exactValueSlot(
		fragment,
		claimSlotIDValue,
		claimGraphKindID,
	)
	if len(fragment.Slots()) != 3 || !recordReady || !concernReady || !claimReady {
		missing = append(missing, mustMissingBasis(
			diagnosticCode(contract, "mapping_slots"),
			contract.definition.mappingRepair,
		))
		return resolvedMapping{}, missing
	}
	valueBinding, found := environment.ValueBinding(claimTarget.ValueKind())
	if !found {
		missing = append(missing, mustMissingBasis(
			"claim_graph_value_binding",
			"repair:select-typeenv-with-claim-graph-binding",
		))
		return resolvedMapping{}, missing
	}
	shape, found := environment.ValueShape(valueBinding.ValueShape())
	if !found || shape.Shape().Kind() != typedmemory.ValueShapeClaimGraph {
		missing = append(missing, mustMissingBasis(
			"claim_graph_shape",
			"repair:refresh-selected-claim-graph-shape",
		))
		return resolvedMapping{}, missing
	}
	codec, found := codecs.Resolve(valueBinding.Codec())
	if !found {
		missing = append(missing, mustMissingBasis(
			"claim_graph_codec",
			"repair:resolve-selected-claim-graph-codec",
		))
		return resolvedMapping{}, missing
	}
	return resolvedMapping{
		fragment:        fragment.Ref(),
		recordSlot:      recordSlotIDValue,
		recordRefKind:   recordTarget.ReferenceKind(),
		concernSlot:     concernSlotIDValue,
		concernRefKind:  concernTarget.ReferenceKind(),
		claimGraphSlot:  claimSlotIDValue,
		claimGraphKind:  claimTarget.ValueKind(),
		claimGraphShape: valueBinding.ValueShape(),
		claimGraphCodec: valueBinding.Codec(),
		codec:           codec,
	}, nil
}

func mustRecordCarrierVariant(
	contract Contract,
) recordcarrier.ProjectRecordCarrierVariantV1 {
	variant, _ := recordCarrierVariant(contract.definition.recordVariant)
	return variant
}

func diagnosticCode(contract Contract, suffix string) string {
	return contract.definition.relationDiagnosticName + "_" + suffix
}

func exactReferenceSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotID typedmemory.SlotKindID,
	wantKind string,
	wantRefKind string,
) (typedmemory.ReferenceSlotTarget, bool) {
	slot, found := fragment.Slot(slotID)
	if !found || slot.Cardinality() != typedmemory.ExactlyOneCardinality() {
		return typedmemory.ReferenceSlotTarget{}, false
	}
	target, ok := slot.Target().(typedmemory.ReferenceSlotTarget)
	if !ok || target.ValueKind().ID().String() != wantKind ||
		target.ReferenceKind().ID().String() != wantRefKind {
		return typedmemory.ReferenceSlotTarget{}, false
	}
	return target, true
}

func exactValueSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotID typedmemory.SlotKindID,
	wantKind string,
) (typedmemory.ValueSlotTarget, bool) {
	slot, found := fragment.Slot(slotID)
	if !found || slot.Cardinality() != typedmemory.ExactlyOneCardinality() {
		return typedmemory.ValueSlotTarget{}, false
	}
	target, ok := slot.Target().(typedmemory.ValueSlotTarget)
	if !ok || target.ValueKind().ID().String() != wantKind {
		return typedmemory.ValueSlotTarget{}, false
	}
	return target, true
}

func fragmentAllowsContext(
	fragment typedmemory.TypedRelationDeclarationFragment,
	context typedmemory.BoundedContextRef,
) bool {
	for _, candidate := range fragment.Contexts() {
		if candidate == context {
			return true
		}
	}
	return false
}

func underdeterminedFor(name string, repair string) Underdetermined {
	return underdeterminedResult(mustMissingBasis(name, repair))
}

func mustMissingBasis(name string, repair string) MissingBasis {
	pointer, _ := typedmemory.NewRepairPointer(repair)
	basis, _ := NewMissingBasis(name, pointer)
	return basis
}

func mustSignatureID(raw string) typedmemory.SignatureID {
	value, _ := typedmemory.NewSignatureID(raw)
	return value
}

func mustSlotKindID(raw string) typedmemory.SlotKindID {
	value, _ := typedmemory.NewSlotKindID(raw)
	return value
}

func requireExactEntityID(value typedmemory.EntityID) error {
	parsed, err := typedmemory.NewEntityID(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("EntityID is missing or noncanonical")
	}
	return nil
}

func requireExactBatchLocalRef(value typedmemory.BatchLocalRef) error {
	parsed, err := typedmemory.NewBatchLocalRef(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("BatchLocalRef is missing or noncanonical")
	}
	return nil
}

func requireExactEntityLabel(value typedmemory.EntityLabel) error {
	parsed, err := typedmemory.NewEntityLabel(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("EntityLabel is missing or noncanonical")
	}
	return nil
}

func requireExactAssertionID(value typedmemory.AssertionID) error {
	parsed, err := typedmemory.NewAssertionID(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("AssertionID is missing or noncanonical")
	}
	return nil
}

func requireExactProvenance(value typedmemory.ProvenanceRef) error {
	parsed, err := typedmemory.NewProvenanceRef(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("ProvenanceRef is missing or noncanonical")
	}
	return nil
}

func requireExactContextSlice(value typedmemory.ContextSlice) error {
	parsedRef, err := typedmemory.NewContextSliceRef(value.Digest())
	if err != nil || parsedRef != value.Ref() || len(value.CanonicalBytes()) == 0 {
		return fmt.Errorf("ContextSlice is missing or noncanonical")
	}
	return nil
}

func requireExactBoundedContext(value typedmemory.BoundedContextRef) error {
	parsed, err := typedmemory.NewBoundedContextRef(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("BoundedContextRef is missing or noncanonical")
	}
	return nil
}

func requireExactResolutionBasis(value typedmemory.ResolutionBasisRef) error {
	parsed, err := typedmemory.NewResolutionBasisRef(value.String())
	if err != nil || parsed != value {
		return fmt.Errorf("ResolutionBasisRef is missing or noncanonical")
	}
	return nil
}

func requireExactPersistedRef(value typedmemory.PersistedRef) error {
	refKindID, err := typedmemory.NewRefKindID(value.RefKind().ID().String())
	if err != nil || refKindID != value.RefKind().ID() {
		return fmt.Errorf("persisted reference has an invalid RefKind")
	}
	referenceID, err := typedmemory.NewReferenceID(value.ReferenceID().String())
	if err != nil || referenceID != value.ReferenceID() {
		return fmt.Errorf("persisted reference has an invalid ReferenceID")
	}
	parsed, err := typedmemory.NewPersistedRef(value.RefKind(), referenceID)
	if err != nil || parsed != value {
		return fmt.Errorf("persisted reference is missing or noncanonical")
	}
	return nil
}
