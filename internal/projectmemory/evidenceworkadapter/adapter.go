package evidenceworkadapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
)

const (
	supportingSignatureID = "Haft.SupportingEpistemeRecordAtConcern"
	workSignatureID       = "Haft.WorkOccurrenceRecord"
	evidenceSignatureID   = "Haft.EvidenceUse"
)

type resolvedValueMapping struct {
	kind    typedmemory.ValueKindRef
	shape   typedmemory.ValueShapeRef
	codec   typedmemory.CodecRef
	runtime typedmemory.CodecImplementation
}

type resolvedMapping struct {
	supporting    typedmemory.TypedRelationDeclarationFragmentRef
	work          typedmemory.TypedRelationDeclarationFragmentRef
	evidence      typedmemory.TypedRelationDeclarationFragmentRef
	evidenceRef   typedmemory.RefKindRef
	supportingRef typedmemory.RefKindRef
	workRef       typedmemory.RefKindRef
	occurrenceRef typedmemory.RefKindRef
	claimGraph    resolvedValueMapping
	qualifier     resolvedValueMapping
	interval      resolvedValueMapping
	suite         typedmemorycandidatecodec.Suite
}

// Adapt is a pure local Evidence/Work candidate producer. It requires exact
// pre-resolved subjects and emits no exact FPF Evidence/U.Work classification,
// persistence, truth, authority, lifecycle, repository, CLI, or MCP effect.
func Adapt(
	draft Draft,
	runtime RuntimeBasis,
) Result {
	exact, ready := runtime.(ExactRuntimeBasis)
	if !ready {
		missing, ok := runtime.(MissingRuntimeBasis)
		if ok {
			return underdetermined{missing: missing.MissingBasis()}
		}
		return underdeterminedResult(mustMissingBasis(
			"selected_type_environment",
			"repair:resolve-project-typeenv-head",
		))
	}
	if exact.project != draft.projectID {
		return invalidResult(
			"runtime_project_mismatch",
			"the selected runtime and Evidence/Work draft belong to different projects",
		)
	}
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		return underdeterminedResult(mustMissingBasis(
			"evidence_work_mapping_manifest",
			"repair:reload-evidence-work-mapping-manifest",
		))
	}
	occurrenceManifest, err :=
		carrierfamily.CurrentPerformedWorkOccurrenceMappingManifestV1()
	if err != nil {
		return underdeterminedResult(mustMissingBasis(
			"performed_work_occurrence_mapping_manifest",
			"repair:reload-performed-work-occurrence-mapping-manifest",
		))
	}
	if exact.sourceMode.IsHistoricalMembership() {
		if !mappingAccepted(
			exact.recordRegistration,
			manifest.Ref(),
			manifest.AdapterVersion(),
		) {
			return underdeterminedResult(mustMissingBasis(
				"evidence_work_record_mapping_registration",
				"repair:select-runtime-accepting-evidence-work-mapping",
			))
		}
		if !mappingAccepted(
			exact.workRegistration,
			occurrenceManifest.Ref(),
			occurrenceManifest.AdapterVersion(),
		) {
			return underdeterminedResult(mustMissingBasis(
				"performed_work_occurrence_mapping_registration",
				"repair:select-runtime-accepting-performed-work-occurrence-mapping",
			))
		}
	}
	mapping, missing := resolveMapping(
		exact.environment,
		exact.codecs,
		draft.contextSlice.Context(),
	)
	if len(missing) > 0 {
		return underdetermined{missing: missing}
	}
	return buildCandidate(
		draft,
		mapping,
		manifest,
		occurrenceManifest,
	)
}

func mappingAccepted(
	policy recordmembershipregistration.RegistrationArtifactV1,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
) bool {
	decision, err := policy.EvaluateMappingPolicy(manifest, adapter)
	return err == nil &&
		decision.Kind() == recordmembershipregistration.MappingAccepted
}

func resolveMapping(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	contextRef typedmemory.BoundedContextRef,
) (resolvedMapping, []MissingBasis) {
	supporting, supportingMissing := exactFragment(
		environment,
		contextRef,
		supportingSignatureID,
		3,
	)
	if len(supportingMissing) > 0 {
		return resolvedMapping{}, supportingMissing
	}
	work, workMissing := exactFragment(
		environment,
		contextRef,
		workSignatureID,
		7,
	)
	if len(workMissing) > 0 {
		return resolvedMapping{}, workMissing
	}
	evidence, evidenceMissing := exactFragment(
		environment,
		contextRef,
		evidenceSignatureID,
		6,
	)
	if len(evidenceMissing) > 0 {
		return resolvedMapping{}, evidenceMissing
	}
	suite, err := typedmemorycandidatecodec.NewSuite(
		environment.ValueShapes(),
	)
	if err != nil {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"evidence_work_input_codec_suite",
			"repair:install-source-exact-evidence-work-input-codecs",
		)}
	}
	claimGraph, ready := exactValueMapping(
		environment,
		codecs,
		"U.ClaimGraph",
	)
	if !ready {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"claim_graph_value_mapping",
			"repair:select-typeenv-with-claim-graph-binding",
		)}
	}
	qualifier, ready := exactValueMapping(
		environment,
		codecs,
		"Haft.EvidenceUseQualifier",
	)
	if !ready || qualifier.shape != suite.EvidenceUseQualifier().Shape() {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"evidence_use_qualifier_mapping",
			"repair:select-typeenv-with-evidence-use-qualifier-binding",
		)}
	}
	interval, ready := exactValueMapping(
		environment,
		codecs,
		"Haft.PerformedInterval",
	)
	if !ready || interval.shape != suite.PerformedInterval().Shape() {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"performed_interval_mapping",
			"repair:select-typeenv-with-performed-interval-binding",
		)}
	}
	references := []struct {
		fragment typedmemory.TypedRelationDeclarationFragment
		slot     string
		kind     string
		ref      string
	}{
		{supporting, "Haft.SupportingEpistemeRecordAtConcern.SupportingEpistemeRecordSlot", "Haft.SupportingEpistemeRecord", "Haft.SupportingEpistemeRecordRef"},
		{work, "Haft.WorkOccurrenceRecord.WorkRecordSlot", "Haft.WorkRecord", "Haft.WorkRecordRef"},
		{work, "Haft.WorkOccurrenceRecord.PerformedWorkOccurrenceSlot", "Haft.PerformedWorkOccurrence", "Haft.PerformedWorkOccurrenceRef"},
		{evidence, "Haft.EvidenceUse.EvidenceRecordSlot", "Haft.EvidenceRecord", "Haft.EvidenceRecordRef"},
	}
	resolved := make([]typedmemory.RefKindRef, 0, len(references))
	for _, item := range references {
		ref, ok := exactReferenceSlot(
			item.fragment,
			item.slot,
			item.kind,
			item.ref,
		)
		if !ok {
			return resolvedMapping{}, []MissingBasis{mustMissingBasis(
				"evidence_work_reference_slots",
				"repair:select-typeenv-with-exact-evidence-work-slots",
			)}
		}
		resolved = append(resolved, ref)
	}
	return resolvedMapping{
		supporting:    supporting.Ref(),
		work:          work.Ref(),
		evidence:      evidence.Ref(),
		supportingRef: resolved[0],
		workRef:       resolved[1],
		occurrenceRef: resolved[2],
		evidenceRef:   resolved[3],
		claimGraph:    claimGraph,
		qualifier:     qualifier,
		interval:      interval,
		suite:         suite,
	}, nil
}

func exactFragment(
	environment typedmemory.TypeEnv,
	contextRef typedmemory.BoundedContextRef,
	raw string,
	slotCount int,
) (typedmemory.TypedRelationDeclarationFragment, []MissingBasis) {
	id, err := typedmemory.NewSignatureID(raw)
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{mustMissingBasis(
			"typed_relation_declaration_fragment",
			"repair:select-typeenv-with-"+raw,
		)}
	}
	ref, err := typedmemory.NewTypedRelationDeclarationFragmentRef(environment.Ref(), id)
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{mustMissingBasis(
			"typed_relation_declaration_fragment",
			"repair:select-typeenv-with-"+raw,
		)}
	}
	fragment, found := environment.TypedRelationDeclarationFragment(ref)
	if !found || len(fragment.Slots()) != slotCount {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{mustMissingBasis(
			"typed_relation_declaration_fragment",
			"repair:select-typeenv-with-"+raw,
		)}
	}
	allowed := false
	for _, context := range fragment.Contexts() {
		if context == contextRef {
			allowed = true
		}
	}
	if !allowed {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{mustMissingBasis(
			"relation_context",
			"repair:select-typeenv-allowing-evidence-work-context",
		)}
	}
	return fragment, nil
}

func exactReferenceSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotRaw string,
	kindRaw string,
	refRaw string,
) (typedmemory.RefKindRef, bool) {
	slotID, err := typedmemory.NewSlotKindID(slotRaw)
	if err != nil {
		return typedmemory.RefKindRef{}, false
	}
	slot, found := fragment.Slot(slotID)
	if !found {
		return typedmemory.RefKindRef{}, false
	}
	target, ok := slot.Target().(typedmemory.ReferenceSlotTarget)
	if !ok {
		return typedmemory.RefKindRef{}, false
	}
	kindID, err := typedmemory.NewKindID(kindRaw)
	if err != nil {
		return typedmemory.RefKindRef{}, false
	}
	kind, err := typedmemory.NewValueKindRef(
		fragment.Ref().TypeEnv(),
		kindID,
	)
	if err != nil || target.ValueKind() != kind {
		return typedmemory.RefKindRef{}, false
	}
	refID, err := typedmemory.NewRefKindID(refRaw)
	if err != nil {
		return typedmemory.RefKindRef{}, false
	}
	ref, err := typedmemory.NewRefKindRef(
		fragment.Ref().TypeEnv(),
		refID,
	)
	if err != nil || target.ReferenceKind() != ref {
		return typedmemory.RefKindRef{}, false
	}
	return ref, true
}

func exactValueMapping(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	kindRaw string,
) (resolvedValueMapping, bool) {
	kindID, err := typedmemory.NewKindID(kindRaw)
	if err != nil {
		return resolvedValueMapping{}, false
	}
	kind, err := typedmemory.NewValueKindRef(environment.Ref(), kindID)
	if err != nil {
		return resolvedValueMapping{}, false
	}
	binding, found := environment.ValueBinding(kind)
	if !found {
		return resolvedValueMapping{}, false
	}
	codec, found := codecs.Resolve(binding.Codec())
	if !found {
		return resolvedValueMapping{}, false
	}
	return resolvedValueMapping{
		kind:    kind,
		shape:   binding.ValueShape(),
		codec:   binding.Codec(),
		runtime: codec,
	}, true
}

func buildCandidate(
	draft Draft,
	mapping resolvedMapping,
	manifest MappingManifestV1,
	occurrenceManifest carrierfamily.MappingManifestV1,
) Result {
	supportingGraph, err := claimGraphCandidate(
		draft.supportingClaimGraph.Value(),
		mapping.claimGraph,
	)
	if err != nil {
		return invalidResult("supporting_claim_graph_invalid", err.Error())
	}
	workGraph, err := claimGraphCandidate(
		draft.workClaimGraph.Value(),
		mapping.claimGraph,
	)
	if err != nil {
		return invalidResult("work_claim_graph_invalid", err.Error())
	}
	qualifier, err := candidateValue(
		mapping.suite.EvidenceUseQualifier().EncodeInput(draft.qualifier),
		mapping.qualifier,
	)
	if err != nil {
		return invalidResult("evidence_use_qualifier_invalid", err.Error())
	}
	interval, err := candidateValue(
		mapping.suite.PerformedInterval().EncodeInput(draft.interval),
		mapping.interval,
	)
	if err != nil {
		return invalidResult("performed_interval_invalid", err.Error())
	}
	refs, err := newLocalReferences(draft, mapping)
	if err != nil {
		return invalidResult("evidence_work_local_reference_invalid", err.Error())
	}
	declarations, err := newDeclarations(draft)
	if err != nil {
		return invalidResult("evidence_work_declaration_invalid", err.Error())
	}
	supporting, err := supportingRelation(
		draft,
		mapping,
		refs,
		supportingGraph,
	)
	if err != nil {
		return invalidResult("supporting_episteme_relation_invalid", err.Error())
	}
	work, err := workRelation(
		draft,
		mapping,
		refs,
		workGraph,
		interval,
	)
	if err != nil {
		return invalidResult("work_occurrence_relation_invalid", err.Error())
	}
	evidence, err := evidenceRelation(
		draft,
		mapping,
		refs,
		qualifier,
	)
	if err != nil {
		return invalidResult("evidence_use_relation_invalid", err.Error())
	}
	changes := append(
		[]typedmemory.MemoryChange(nil),
		declarations...,
	)
	changes = append(changes, supporting, work, evidence)
	changeSet, err := typedmemory.NewMemoryChangeSet(changes)
	if err != nil {
		return invalidResult("evidence_work_change_set_invalid", err.Error())
	}
	recordSources, recordClassificationSources, err := recordCandidateSources(
		draft,
		manifest,
	)
	if err != nil {
		return invalidResult("evidence_work_record_source_invalid", err.Error())
	}
	occurrenceSource, occurrenceClassificationSource, err := occurrenceCandidateSources(
		draft,
		occurrenceManifest,
		manifest,
		interval.InputBytes(),
	)
	if err != nil {
		return invalidResult("performed_work_occurrence_source_invalid", err.Error())
	}
	return validCandidateResult{
		changeSet:                      changeSet,
		recordSources:                  recordSources,
		occurrenceSource:               occurrenceSource,
		recordClassificationSources:    recordClassificationSources,
		occurrenceClassificationSource: occurrenceClassificationSource,
		manifest:                       manifest.Ref(),
		adapter:                        manifest.AdapterVersion(),
		signatures: []typedmemory.SignatureID{
			mapping.supporting.ID(),
			mapping.work.ID(),
			mapping.evidence.ID(),
		},
	}
}

func claimGraphCandidate(
	graph typedmemory.ClaimGraphValue,
	mapping resolvedValueMapping,
) (typedmemory.TypedValueCandidate, error) {
	codec, err := typedmemory.NewClaimGraphCodecV1(mapping.shape)
	if err != nil {
		return typedmemory.TypedValueCandidate{}, err
	}
	return candidateValue(codec.EncodeInput(graph), mapping)
}

func candidateValue(
	encoded typedmemory.CodecCanonicalization,
	mapping resolvedValueMapping,
) (typedmemory.TypedValueCandidate, error) {
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return typedmemory.TypedValueCandidate{}, fmt.Errorf(
			"candidate input codec rejected the value",
		)
	}
	selected := mapping.runtime.Canonicalize(
		mapping.shape,
		canonical.CanonicalBytes(),
	)
	replayed, ok := selected.(typedmemory.CanonicalizedCodecValue)
	if !ok ||
		!bytes.Equal(
			replayed.CanonicalBytes(),
			canonical.CanonicalBytes(),
		) {
		return typedmemory.TypedValueCandidate{}, fmt.Errorf(
			"selected runtime codec did not replay exact canonical bytes",
		)
	}
	return typedmemory.NewTypedValueCandidate(
		mapping.kind,
		mapping.shape,
		mapping.codec,
		replayed.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
}

type localReferences struct {
	evidence   typedmemory.LocalRef
	supporting typedmemory.LocalRef
	work       typedmemory.LocalRef
	occurrence typedmemory.LocalRef
}

func newLocalReferences(
	draft Draft,
	mapping resolvedMapping,
) (localReferences, error) {
	evidence, err := typedmemory.NewLocalRef(
		mapping.evidenceRef,
		draft.evidenceRecord.local,
	)
	if err != nil {
		return localReferences{}, err
	}
	supporting, err := typedmemory.NewLocalRef(
		mapping.supportingRef,
		draft.supportingRecord.local,
	)
	if err != nil {
		return localReferences{}, err
	}
	work, err := typedmemory.NewLocalRef(
		mapping.workRef,
		draft.workRecord.local,
	)
	if err != nil {
		return localReferences{}, err
	}
	occurrence, err := typedmemory.NewLocalRef(
		mapping.occurrenceRef,
		draft.occurrence.local,
	)
	if err != nil {
		return localReferences{}, err
	}
	return localReferences{
		evidence:   evidence,
		supporting: supporting,
		work:       work,
		occurrence: occurrence,
	}, nil
}

func newDeclarations(
	draft Draft,
) ([]typedmemory.MemoryChange, error) {
	identities := []NewEntityIdentity{
		draft.evidenceRecord,
		draft.supportingRecord,
		draft.workRecord,
		draft.occurrence,
	}
	changes := make([]typedmemory.MemoryChange, 0, len(identities))
	for _, identity := range identities {
		declaration, err := typedmemory.NewDeclareEntity(
			identity.entity,
			identity.local,
			draft.contextSlice.Context(),
			identity.label,
			draft.provenance,
		)
		if err != nil {
			return nil, err
		}
		changes = append(changes, declaration)
	}
	return changes, nil
}

func supportingRelation(
	draft Draft,
	mapping resolvedMapping,
	refs localReferences,
	graph typedmemory.TypedValueCandidate,
) (typedmemory.AssertRelation, error) {
	bindings, err := bindings(
		referenceSlot("Haft.SupportingEpistemeRecordAtConcern.SupportingEpistemeRecordSlot", refs.supporting),
		referenceSlot("Haft.SupportingEpistemeRecordAtConcern.EntityOfConcernSlot", draft.concern.reference),
		valueSlot("Haft.SupportingEpistemeRecordAtConcern.ClaimGraphSlot", graph),
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	return relation(
		draft.supportingAssertion,
		mapping.supporting,
		draft,
		bindings,
	)
}

func workRelation(
	draft Draft,
	mapping resolvedMapping,
	refs localReferences,
	graph typedmemory.TypedValueCandidate,
	interval typedmemory.TypedValueCandidate,
) (typedmemory.AssertRelation, error) {
	bindings, err := bindings(
		referenceSlot("Haft.WorkOccurrenceRecord.WorkRecordSlot", refs.work),
		referenceSlot("Haft.WorkOccurrenceRecord.PerformedWorkOccurrenceSlot", refs.occurrence),
		referenceSlot("Haft.WorkOccurrenceRecord.EntityOfConcernSlot", draft.concern.reference),
		referenceSlot("Haft.WorkOccurrenceRecord.PerformerSlot", draft.performer.reference),
		referenceSlot("Haft.WorkOccurrenceRecord.ProducedSupportingEpistemeRecordSlot", refs.supporting),
		valueSlot("Haft.WorkOccurrenceRecord.PerformedIntervalSlot", interval),
		valueSlot("Haft.WorkOccurrenceRecord.ClaimGraphSlot", graph),
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	return relation(draft.workAssertion, mapping.work, draft, bindings)
}

func evidenceRelation(
	draft Draft,
	mapping resolvedMapping,
	refs localReferences,
	qualifier typedmemory.TypedValueCandidate,
) (typedmemory.AssertRelation, error) {
	bindings, err := bindings(
		referenceSlot("Haft.EvidenceUse.EvidenceRecordSlot", refs.evidence),
		referenceSlot("Haft.EvidenceUse.SupportingEpistemeRecordSlot", refs.supporting),
		referenceSlot("Haft.EvidenceUse.TargetProjectClaimSlot", draft.targetClaim.reference),
		valueSlot("Haft.EvidenceUse.EvidenceUseQualifierSlot", qualifier),
		referenceSlot("Haft.EvidenceUse.ProvenanceCarrierEditionSlot", draft.provenanceCarrierEdition.reference),
		referenceSlot("Haft.EvidenceUse.ProducedByPerformedWorkOccurrenceSlot", refs.occurrence),
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	return relation(
		draft.evidenceAssertion,
		mapping.evidence,
		draft,
		bindings,
	)
}

type pendingSlot struct {
	name      string
	reference typedmemory.StrongRef
	value     typedmemory.TypedValueCandidate
	byValue   bool
}

func referenceSlot(
	name string,
	reference typedmemory.StrongRef,
) pendingSlot {
	return pendingSlot{name: name, reference: reference}
}

func valueSlot(
	name string,
	value typedmemory.TypedValueCandidate,
) pendingSlot {
	return pendingSlot{name: name, value: value, byValue: true}
}

func bindings(
	slots ...pendingSlot,
) ([]typedmemory.CandidateSlotBinding, error) {
	result := make([]typedmemory.CandidateSlotBinding, 0, len(slots))
	for _, slot := range slots {
		id, err := typedmemory.NewSlotKindID(slot.name)
		if err != nil {
			return nil, err
		}
		var filler typedmemory.CandidateSlotFiller
		if slot.byValue {
			filler, err = typedmemory.NewByValueCandidate(slot.value)
		} else {
			filler, err = typedmemory.NewByReferenceCandidate(slot.reference)
		}
		if err != nil {
			return nil, err
		}
		binding, err := typedmemory.NewCandidateSlotBinding(
			id,
			[]typedmemory.CandidateSlotFiller{filler},
		)
		if err != nil {
			return nil, err
		}
		result = append(result, binding)
	}
	return result, nil
}

func relation(
	assertion typedmemory.AssertionID,
	fragment typedmemory.TypedRelationDeclarationFragmentRef,
	draft Draft,
	bindings []typedmemory.CandidateSlotBinding,
) (typedmemory.AssertRelation, error) {
	modality := typedmemory.NewAffirmsObtaining()
	relationalAssertion, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  fragment,
			Slice:      draft.contextSlice,
			Modality:   modality,
			Bindings:   bindings,
			Provenance: draft.provenance,
		},
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	return typedmemory.NewAssertRelation(relationalAssertion)
}

func recordCandidateSources(
	draft Draft,
	manifest MappingManifestV1,
) (
	[]recordcarrier.RecordMembershipSourceV1,
	[]recordcarrier.RecordClassificationSourceV1,
	error,
) {
	items := []struct {
		identity NewEntityIdentity
		variant  recordcarrier.ProjectRecordCarrierVariantV1
	}{
		{draft.evidenceRecord, recordcarrier.EvidenceRecordVariantV1{}},
		{draft.supportingRecord, recordcarrier.SupportingEpistemeRecordVariantV1{}},
		{draft.workRecord, recordcarrier.WorkRecordVariantV1{}},
	}
	membershipSources := make(
		[]recordcarrier.RecordMembershipSourceV1,
		0,
		len(items),
	)
	classificationSources := make(
		[]recordcarrier.RecordClassificationSourceV1,
		0,
		len(items),
	)
	for _, item := range items {
		carrier, err := recordcarrier.SealProjectRecordCarrierV1(
			item.identity.entity,
			draft.contextSlice.Context(),
			item.variant,
		)
		if err != nil {
			return nil, nil, err
		}
		binding, err := recordcarrier.SealEntityRecordCarrierBindingV1(
			draft.projectID,
			carrier,
			manifest.Ref(),
			manifest.AdapterVersion(),
		)
		if err != nil {
			return nil, nil, err
		}
		membershipSource, err := recordcarrier.SealRecordMembershipSourceV1(
			draft.projectID,
			item.identity.entity,
			draft.contextSlice.Context(),
			carrier,
			binding,
		)
		if err != nil {
			return nil, nil, err
		}
		classificationSource, err := recordcarrier.SealRecordClassificationSourceV1(
			draft.projectID,
			item.identity.entity,
			draft.contextSlice.Context(),
			carrier,
			binding,
		)
		if err != nil {
			return nil, nil, err
		}
		membershipSources = append(membershipSources, membershipSource)
		classificationSources = append(classificationSources, classificationSource)
	}
	return membershipSources, classificationSources, nil
}

func occurrenceCandidateSources(
	draft Draft,
	historicalManifest carrierfamily.MappingManifestV1,
	currentManifest MappingManifestV1,
	canonicalInterval []byte,
) (
	carrierfamily.MembershipSourceV1,
	carrierfamily.ClassificationSourceV1,
	error,
) {
	payload, err := occurrenceSourcePayload(canonicalInterval)
	if err != nil {
		return carrierfamily.MembershipSourceV1{}, carrierfamily.ClassificationSourceV1{}, err
	}
	carrier, err := carrierfamily.SealPerformedWorkOccurrenceCarrierV1(
		draft.occurrence.entity,
		draft.contextSlice.Context(),
		payload,
	)
	if err != nil {
		return carrierfamily.MembershipSourceV1{}, carrierfamily.ClassificationSourceV1{}, err
	}
	historicalBinding, err := carrierfamily.SealEntityCarrierBindingV1(
		draft.projectID,
		carrier,
		historicalManifest.Ref(),
		historicalManifest.AdapterVersion(),
	)
	if err != nil {
		return carrierfamily.MembershipSourceV1{}, carrierfamily.ClassificationSourceV1{}, err
	}
	membershipSource, err := carrierfamily.SealMembershipSourceV1(
		draft.projectID,
		draft.occurrence.entity,
		draft.contextSlice.Context(),
		carrier,
		historicalBinding,
	)
	if err != nil {
		return carrierfamily.MembershipSourceV1{}, carrierfamily.ClassificationSourceV1{}, err
	}
	currentBinding, err := carrierfamily.SealEntityCarrierBindingV1(
		draft.projectID,
		carrier,
		currentManifest.Ref(),
		currentManifest.AdapterVersion(),
	)
	if err != nil {
		return carrierfamily.MembershipSourceV1{}, carrierfamily.ClassificationSourceV1{}, err
	}
	classificationSource, err := carrierfamily.SealClassificationSourceV1(
		draft.projectID,
		draft.occurrence.entity,
		draft.contextSlice.Context(),
		carrier,
		currentBinding,
	)
	if err != nil {
		return carrierfamily.MembershipSourceV1{}, carrierfamily.ClassificationSourceV1{}, err
	}
	return membershipSource, classificationSource, nil
}

func occurrenceSourcePayload(
	canonical []byte,
) (carrierfamily.SourcePayloadV1, error) {
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		return carrierfamily.SourcePayloadV1{}, err
	}
	ref, err := typedmemory.NewCarrierRef(
		"performed-work-occurrence:" + digest.String(),
	)
	if err != nil {
		return carrierfamily.SourcePayloadV1{}, err
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		return carrierfamily.SourcePayloadV1{}, err
	}
	return carrierfamily.NewSourcePayloadV1(
		ref,
		edition,
		digest,
		"haft.performed-work-occurrence/v1",
		canonical,
	)
}
