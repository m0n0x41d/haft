package codeanchoradapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
)

const (
	codeAnchorDefinitionSignatureID = "Haft.CodeAnchorDefinition"
	codeAnchorDefinitionAnchorSlot  = "Haft.CodeAnchorDefinition.CodeAnchorSlot"
	codeAnchorDefinitionLocatorSlot = "Haft.CodeAnchorDefinition.CodeAnchorLocatorSlot"

	codeRealizesClaimSignatureID = "Haft.CodeRealizesClaim"
	codeRealizesClaimAnchorSlot  = "Haft.CodeRealizesClaim.CodeAnchorSlot"
	codeRealizesClaimTargetSlot  = "Haft.CodeRealizesClaim.TargetProjectClaimSlot"

	codeChangedByWorkSignatureID = "Haft.CodeChangedByWork"
	codeChangedByWorkAnchorSlot  = "Haft.CodeChangedByWork.CodeAnchorSlot"
	codeChangedByWorkTargetSlot  = "Haft.CodeChangedByWork.PerformedWorkOccurrenceSlot"

	codeAnchorKindID        = "Haft.CodeAnchor"
	codeAnchorRefKindID     = "Haft.CodeAnchorRef"
	codeAnchorLocatorKindID = "Haft.CodeAnchorLocator"
	projectClaimKindID      = "Haft.ProjectClaim"
	projectClaimRefKindID   = "Haft.ProjectClaimRef"
	performedWorkKindID     = "Haft.PerformedWorkOccurrence"
	performedWorkRefKindID  = "Haft.PerformedWorkOccurrenceRef"

	codeAnchorPayloadEdition = "1.0.0"
	codeAnchorPayloadSchema  = "haft.code-anchor-locator/v1"
)

type relationMapping struct {
	fragment  typedmemory.TypedRelationDeclarationFragmentRef
	anchor    typedmemory.SlotKindID
	target    typedmemory.SlotKindID
	targetRef typedmemory.RefKindRef
}

type resolvedMapping struct {
	anchorRefKind typedmemory.RefKindRef
	locatorSlot   typedmemory.SlotKindID
	locatorKind   typedmemory.ValueKindRef
	locatorShape  typedmemory.ValueShapeRef
	locatorCodec  typedmemory.CodecRef
	encoder       typedmemorycandidatecodec.CodeAnchorLocatorV1
	codec         typedmemory.CodecImplementation
	definition    typedmemory.TypedRelationDeclarationFragmentRef
	definitionRef typedmemory.SlotKindID
	claims        relationMapping
	work          relationMapping
}

// Adapt is a pure candidate producer. It defines one exact CodeAnchor and
// emits only caller-supplied claim/work links. It performs no repository scan,
// symbol resolution, evidence classification, work classification, admission,
// storage, authority, lifecycle, CLI, or MCP effect.
func Adapt(
	draft Draft,
	runtime RuntimeBasis,
) Result {
	locator, ready := draft.locator.(ExactLocator)
	if !ready {
		missing, ok := draft.locator.(MissingLocator)
		if !ok {
			return underdeterminedResult(mustMissingBasis(
				"code_anchor_locator",
				"repair:resolve-exact-code-anchor-locator",
			))
		}
		return underdetermined{missing: missing.MissingBasis()}
	}
	targets, missing, invalid := resolveSemanticTargets(
		draft.links,
		draft.contextSlice.Context(),
	)
	if invalid != nil {
		return invalid
	}
	if len(missing) > 0 {
		return underdetermined{missing: missing}
	}
	exactRuntime, ready := runtime.(ExactRuntimeBasis)
	if !ready {
		missingRuntime, ok := runtime.(MissingRuntimeBasis)
		if !ok {
			return underdeterminedResult(mustMissingBasis(
				"selected_type_environment",
				"repair:resolve-project-typeenv-head",
			))
		}
		return underdetermined{missing: missingRuntime.MissingBasis()}
	}
	if exactRuntime.project != draft.projectID {
		return invalidResult(
			"runtime_project_mismatch",
			"the selected runtime and CodeAnchor draft belong to different projects",
		)
	}
	manifest, err := CurrentMappingManifestV1()
	if err != nil {
		return underdeterminedResult(mustMissingBasis(
			"code_anchor_mapping_manifest",
			"repair:reload-code-anchor-mapping-manifest",
		))
	}
	if exactRuntime.sourceMode.IsHistoricalMembership() {
		policy, err := exactRuntime.registration.EvaluateMappingPolicy(
			manifest.Ref(),
			manifest.AdapterVersion(),
		)
		if err != nil ||
			policy.Kind() != recordmembershipregistration.MappingAccepted {
			return underdeterminedResult(mustMissingBasis(
				"code_anchor_mapping_registration",
				"repair:select-typeenv-with-code-anchor-adapter-registration",
			))
		}
	}
	mapping, mappingMissing := resolveMapping(
		exactRuntime.environment,
		exactRuntime.codecs,
		draft.contextSlice.Context(),
		targets,
	)
	if len(mappingMissing) > 0 {
		return underdetermined{missing: mappingMissing}
	}
	return buildCandidate(
		draft,
		locator.Value(),
		targets,
		mapping,
		manifest,
	)
}

type exactSemanticTarget struct {
	link      SemanticLink
	reference typedmemory.PersistedRef
}

func resolveSemanticTargets(
	links []SemanticLink,
	contextRef typedmemory.BoundedContextRef,
) ([]exactSemanticTarget, []MissingBasis, Invalid) {
	targets := make([]exactSemanticTarget, 0, len(links))
	missing := make([]MissingBasis, 0)
	for _, link := range links {
		switch binding := link.targetBinding().(type) {
		case ExactReferenceBinding:
			if binding.context != contextRef {
				return nil, nil, invalidResult(
					"semantic_target_context_mismatch",
					"the CodeAnchor and its semantic target use different bounded contexts",
				)
			}
			targets = append(targets, exactSemanticTarget{
				link:      link,
				reference: binding.reference,
			})
		case UnsettledReferenceBinding:
			missing = append(missing, binding.MissingBasis()...)
		default:
			missing = append(missing, mustMissingBasis(
				"semantic_target_resolution",
				"repair:resolve-code-anchor-semantic-target",
			))
		}
	}
	if len(missing) == 0 {
		return targets, nil, nil
	}
	normalized, err := normalizeMissingBasis(missing)
	if err != nil {
		return nil, nil, invalidResult(
			"semantic_target_missing_basis_invalid",
			err.Error(),
		)
	}
	return nil, normalized, nil
}

func resolveMapping(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	contextRef typedmemory.BoundedContextRef,
	targets []exactSemanticTarget,
) (resolvedMapping, []MissingBasis) {
	definition, missing := resolveDefinitionMapping(
		environment,
		codecs,
		contextRef,
	)
	if len(missing) > 0 {
		return resolvedMapping{}, missing
	}
	result := definition
	for _, target := range targets {
		switch target.link.(type) {
		case ClaimLink:
			if result.claims.fragment.ID().String() != "" {
				continue
			}
			link, linkMissing := resolveLinkMapping(
				environment,
				contextRef,
				codeRealizesClaimSignatureID,
				codeRealizesClaimAnchorSlot,
				codeRealizesClaimTargetSlot,
				projectClaimKindID,
				projectClaimRefKindID,
			)
			if len(linkMissing) > 0 {
				return resolvedMapping{}, linkMissing
			}
			result.claims = link
		case WorkLink:
			if result.work.fragment.ID().String() != "" {
				continue
			}
			link, linkMissing := resolveLinkMapping(
				environment,
				contextRef,
				codeChangedByWorkSignatureID,
				codeChangedByWorkAnchorSlot,
				codeChangedByWorkTargetSlot,
				performedWorkKindID,
				performedWorkRefKindID,
			)
			if len(linkMissing) > 0 {
				return resolvedMapping{}, linkMissing
			}
			result.work = link
		}
	}
	for _, target := range targets {
		expected := result.claims.targetRef
		switch target.link.(type) {
		case WorkLink:
			expected = result.work.targetRef
		}
		if target.reference.RefKind() != expected {
			return resolvedMapping{}, []MissingBasis{mustMissingBasis(
				"semantic_target_reference_kind",
				"repair:resolve-code-anchor-target-with-exact-reference-kind",
			)}
		}
	}
	return result, nil
}

func resolveDefinitionMapping(
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	contextRef typedmemory.BoundedContextRef,
) (resolvedMapping, []MissingBasis) {
	fragment, missing := exactFragment(
		environment,
		contextRef,
		codeAnchorDefinitionSignatureID,
		2,
	)
	if len(missing) > 0 {
		return resolvedMapping{}, missing
	}
	anchorSlot := mustSlotKindID(codeAnchorDefinitionAnchorSlot)
	locatorSlot := mustSlotKindID(codeAnchorDefinitionLocatorSlot)
	anchorRef, anchorReady := exactReferenceSlot(
		fragment,
		anchorSlot,
		codeAnchorKindID,
		codeAnchorRefKindID,
	)
	locatorKind, locatorReady := exactValueSlot(
		fragment,
		locatorSlot,
		codeAnchorLocatorKindID,
	)
	if !anchorReady || !locatorReady {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"code_anchor_definition_slots",
			"repair:select-typeenv-with-exact-code-anchor-definition",
		)}
	}
	binding, found := environment.ValueBinding(locatorKind)
	if !found {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"code_anchor_locator_binding",
			"repair:select-typeenv-with-code-anchor-locator-binding",
		)}
	}
	codec, found := codecs.Resolve(binding.Codec())
	if !found {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"code_anchor_locator_codec",
			"repair:install-selected-code-anchor-locator-codec",
		)}
	}
	suite, err := typedmemorycandidatecodec.NewSuite(
		environment.ValueShapes(),
	)
	if err != nil ||
		suite.CodeAnchorLocator().Shape() != binding.ValueShape() {
		return resolvedMapping{}, []MissingBasis{mustMissingBasis(
			"code_anchor_locator_input_encoder",
			"repair:install-source-exact-code-anchor-locator-input-encoder",
		)}
	}
	return resolvedMapping{
		anchorRefKind: anchorRef,
		locatorSlot:   locatorSlot,
		locatorKind:   locatorKind,
		locatorShape:  binding.ValueShape(),
		locatorCodec:  binding.Codec(),
		encoder:       suite.CodeAnchorLocator(),
		codec:         codec,
		definition:    fragment.Ref(),
		definitionRef: anchorSlot,
	}, nil
}

func resolveLinkMapping(
	environment typedmemory.TypeEnv,
	contextRef typedmemory.BoundedContextRef,
	signatureID string,
	anchorSlotID string,
	targetSlotID string,
	targetKindID string,
	targetRefKindID string,
) (relationMapping, []MissingBasis) {
	fragment, missing := exactFragment(
		environment,
		contextRef,
		signatureID,
		2,
	)
	if len(missing) > 0 {
		return relationMapping{}, missing
	}
	anchorSlot := mustSlotKindID(anchorSlotID)
	targetSlot := mustSlotKindID(targetSlotID)
	anchorRef, anchorReady := exactReferenceSlot(
		fragment,
		anchorSlot,
		codeAnchorKindID,
		codeAnchorRefKindID,
	)
	targetRef, targetReady := exactReferenceSlot(
		fragment,
		targetSlot,
		targetKindID,
		targetRefKindID,
	)
	if !anchorReady || !targetReady ||
		anchorRef.ID().String() != codeAnchorRefKindID {
		return relationMapping{}, []MissingBasis{mustMissingBasis(
			"code_anchor_link_slots",
			"repair:select-typeenv-with-exact-"+signatureID,
		)}
	}
	return relationMapping{
		fragment:  fragment.Ref(),
		anchor:    anchorSlot,
		target:    targetSlot,
		targetRef: targetRef,
	}, nil
}

func exactFragment(
	environment typedmemory.TypeEnv,
	contextRef typedmemory.BoundedContextRef,
	signatureID string,
	slotCount int,
) (typedmemory.TypedRelationDeclarationFragment, []MissingBasis) {
	ref, err := typedmemory.NewTypedRelationDeclarationFragmentRef(
		environment.Ref(),
		mustSignatureID(signatureID),
	)
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{mustMissingBasis(
			"typed_relation_declaration_fragment",
			"repair:select-typeenv-with-"+signatureID,
		)}
	}
	fragment, found := environment.TypedRelationDeclarationFragment(ref)
	if !found ||
		len(fragment.Slots()) != slotCount ||
		!fragmentAllowsContext(fragment, contextRef) {
		return typedmemory.TypedRelationDeclarationFragment{}, []MissingBasis{mustMissingBasis(
			"typed_relation_declaration_fragment",
			"repair:select-typeenv-with-"+signatureID,
		)}
	}
	return fragment, nil
}

func buildCandidate(
	draft Draft,
	locator typedmemorycandidatecodec.CodeAnchorLocator,
	targets []exactSemanticTarget,
	mapping resolvedMapping,
	manifest MappingManifestV1,
) Result {
	locatorValue, err := canonicalLocatorCandidate(
		locator,
		mapping,
	)
	if err != nil {
		return invalidResult("code_anchor_locator_invalid", err.Error())
	}
	anchorReference, err := typedmemory.NewLocalRef(
		mapping.anchorRefKind,
		draft.anchorLocalRef,
	)
	if err != nil {
		return invalidResult("code_anchor_reference_invalid", err.Error())
	}
	declaration, err := typedmemory.NewDeclareEntity(
		draft.anchorEntity,
		draft.anchorLocalRef,
		draft.contextSlice.Context(),
		draft.anchorLabel,
		draft.provenance,
	)
	if err != nil {
		return invalidResult("code_anchor_declaration_invalid", err.Error())
	}
	definition, err := buildDefinitionRelation(
		draft,
		anchorReference,
		locatorValue,
		mapping,
	)
	if err != nil {
		return invalidResult("code_anchor_definition_invalid", err.Error())
	}
	changes := []typedmemory.MemoryChange{declaration, definition}
	signatures := []typedmemory.SignatureID{mapping.definition.ID()}
	for _, target := range targets {
		change, signature, linkErr := buildSemanticLinkRelation(
			draft,
			anchorReference,
			target,
			mapping,
		)
		if linkErr != nil {
			return invalidResult(
				"code_anchor_semantic_link_invalid",
				linkErr.Error(),
			)
		}
		changes = append(changes, change)
		signatures = append(signatures, signature)
	}
	changeSet, err := typedmemory.NewMemoryChangeSet(changes)
	if err != nil {
		return invalidResult("code_anchor_change_set_invalid", err.Error())
	}
	payload, err := locatorSourcePayload(locatorValue.InputBytes())
	if err != nil {
		return invalidResult("code_anchor_payload_invalid", err.Error())
	}
	carrier, err := carrierfamily.SealCodeAnchorCarrierV1(
		draft.anchorEntity,
		draft.contextSlice.Context(),
		payload,
	)
	if err != nil {
		return invalidResult("code_anchor_carrier_invalid", err.Error())
	}
	binding, err := carrierfamily.SealEntityCarrierBindingV1(
		draft.projectID,
		carrier,
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	if err != nil {
		return invalidResult("code_anchor_binding_invalid", err.Error())
	}
	source, err := carrierfamily.SealMembershipSourceV1(
		draft.projectID,
		draft.anchorEntity,
		draft.contextSlice.Context(),
		carrier,
		binding,
	)
	if err != nil {
		return invalidResult(
			"code_anchor_membership_source_invalid",
			err.Error(),
		)
	}
	classificationSource, err := carrierfamily.SealClassificationSourceV1(
		draft.projectID,
		draft.anchorEntity,
		draft.contextSlice.Context(),
		carrier,
		binding,
	)
	if err != nil {
		return invalidResult(
			"code_anchor_classification_source_invalid",
			err.Error(),
		)
	}
	return validCandidateResult{
		changeSet:            changeSet,
		carrier:              carrier,
		binding:              binding,
		membershipSource:     source,
		classificationSource: classificationSource,
		manifest:             manifest.Ref(),
		adapter:              manifest.AdapterVersion(),
		signatures:           signatures,
	}
}

func canonicalLocatorCandidate(
	locator typedmemorycandidatecodec.CodeAnchorLocator,
	mapping resolvedMapping,
) (typedmemory.TypedValueCandidate, error) {
	encoded := mapping.encoder.EncodeInput(locator)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return typedmemory.TypedValueCandidate{}, fmt.Errorf(
			"CodeAnchor locator codec rejected the locator",
		)
	}
	verified := mapping.codec.Canonicalize(
		mapping.locatorShape,
		canonical.CanonicalBytes(),
	)
	replayed, ok := verified.(typedmemory.CanonicalizedCodecValue)
	if !ok ||
		!bytes.Equal(
			replayed.CanonicalBytes(),
			canonical.CanonicalBytes(),
		) {
		return typedmemory.TypedValueCandidate{}, fmt.Errorf(
			"selected CodeAnchor locator codec did not replay exact canonical bytes",
		)
	}
	return typedmemory.NewTypedValueCandidate(
		mapping.locatorKind,
		mapping.locatorShape,
		mapping.locatorCodec,
		replayed.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
}

func buildDefinitionRelation(
	draft Draft,
	anchor typedmemory.LocalRef,
	locator typedmemory.TypedValueCandidate,
	mapping resolvedMapping,
) (typedmemory.AssertRelation, error) {
	anchorFiller, err := typedmemory.NewByReferenceCandidate(anchor)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	locatorFiller, err := typedmemory.NewByValueCandidate(locator)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	anchorBinding, err := typedmemory.NewCandidateSlotBinding(
		mapping.definitionRef,
		[]typedmemory.CandidateSlotFiller{anchorFiller},
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	locatorBinding, err := typedmemory.NewCandidateSlotBinding(
		mapping.locatorSlot,
		[]typedmemory.CandidateSlotFiller{locatorFiller},
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	modality := typedmemory.NewAffirmsObtaining()
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion: draft.definitionAssertionID,
			Signature: mapping.definition,
			Slice:     draft.contextSlice,
			Modality:  modality,
			Bindings: []typedmemory.CandidateSlotBinding{
				anchorBinding,
				locatorBinding,
			},
			Provenance: draft.provenance,
		},
	)
	if err != nil {
		return typedmemory.AssertRelation{}, err
	}
	return typedmemory.NewAssertRelation(relation)
}

func buildSemanticLinkRelation(
	draft Draft,
	anchor typedmemory.LocalRef,
	target exactSemanticTarget,
	mapping resolvedMapping,
) (
	typedmemory.AssertRelation,
	typedmemory.SignatureID,
	error,
) {
	linkMapping := mapping.claims
	switch target.link.(type) {
	case WorkLink:
		linkMapping = mapping.work
	}
	anchorFiller, err := typedmemory.NewByReferenceCandidate(anchor)
	if err != nil {
		return typedmemory.AssertRelation{},
			typedmemory.SignatureID{},
			err
	}
	targetFiller, err := typedmemory.NewByReferenceCandidate(target.reference)
	if err != nil {
		return typedmemory.AssertRelation{},
			typedmemory.SignatureID{},
			err
	}
	anchorBinding, err := typedmemory.NewCandidateSlotBinding(
		linkMapping.anchor,
		[]typedmemory.CandidateSlotFiller{anchorFiller},
	)
	if err != nil {
		return typedmemory.AssertRelation{},
			typedmemory.SignatureID{},
			err
	}
	targetBinding, err := typedmemory.NewCandidateSlotBinding(
		linkMapping.target,
		[]typedmemory.CandidateSlotFiller{targetFiller},
	)
	if err != nil {
		return typedmemory.AssertRelation{},
			typedmemory.SignatureID{},
			err
	}
	assertionID := target.link.assertionID()
	modality := typedmemory.NewAffirmsObtaining()
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion: assertionID,
			Signature: linkMapping.fragment,
			Slice:     draft.contextSlice,
			Modality:  modality,
			Bindings: []typedmemory.CandidateSlotBinding{
				anchorBinding,
				targetBinding,
			},
			Provenance: draft.provenance,
		},
	)
	if err != nil {
		return typedmemory.AssertRelation{},
			typedmemory.SignatureID{},
			err
	}
	change, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		return typedmemory.AssertRelation{},
			typedmemory.SignatureID{},
			err
	}
	return change, linkMapping.fragment.ID(), nil
}

func locatorSourcePayload(
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
		"code-anchor-locator:" + digest.String(),
	)
	if err != nil {
		return carrierfamily.SourcePayloadV1{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(codeAnchorPayloadEdition)
	if err != nil {
		return carrierfamily.SourcePayloadV1{}, err
	}
	return carrierfamily.NewSourcePayloadV1(
		ref,
		edition,
		digest,
		codeAnchorPayloadSchema,
		canonical,
	)
}

func exactReferenceSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotID typedmemory.SlotKindID,
	kindID string,
	refKindID string,
) (typedmemory.RefKindRef, bool) {
	slot, found := fragment.Slot(slotID)
	if !found {
		return typedmemory.RefKindRef{}, false
	}
	target, ok := slot.Target().(typedmemory.ReferenceSlotTarget)
	if !ok {
		return typedmemory.RefKindRef{}, false
	}
	expectedKind, err := typedmemory.NewValueKindRef(
		fragment.Ref().TypeEnv(),
		mustKindID(kindID),
	)
	if err != nil || target.ValueKind() != expectedKind {
		return typedmemory.RefKindRef{}, false
	}
	expectedRef, err := typedmemory.NewRefKindRef(
		fragment.Ref().TypeEnv(),
		mustRefKindID(refKindID),
	)
	if err != nil || target.ReferenceKind() != expectedRef {
		return typedmemory.RefKindRef{}, false
	}
	return expectedRef, true
}

func exactValueSlot(
	fragment typedmemory.TypedRelationDeclarationFragment,
	slotID typedmemory.SlotKindID,
	kindID string,
) (typedmemory.ValueKindRef, bool) {
	slot, found := fragment.Slot(slotID)
	if !found {
		return typedmemory.ValueKindRef{}, false
	}
	target, ok := slot.Target().(typedmemory.ValueSlotTarget)
	if !ok {
		return typedmemory.ValueKindRef{}, false
	}
	expected, err := typedmemory.NewValueKindRef(
		fragment.Ref().TypeEnv(),
		mustKindID(kindID),
	)
	if err != nil || target.ValueKind() != expected {
		return typedmemory.ValueKindRef{}, false
	}
	return expected, true
}

func fragmentAllowsContext(
	fragment typedmemory.TypedRelationDeclarationFragment,
	contextRef typedmemory.BoundedContextRef,
) bool {
	for _, allowed := range fragment.Contexts() {
		if allowed == contextRef {
			return true
		}
	}
	return false
}

func mustSignatureID(raw string) typedmemory.SignatureID {
	value, _ := typedmemory.NewSignatureID(raw)
	return value
}

func mustSlotKindID(raw string) typedmemory.SlotKindID {
	value, _ := typedmemory.NewSlotKindID(raw)
	return value
}

func mustKindID(raw string) typedmemory.KindID {
	value, _ := typedmemory.NewKindID(raw)
	return value
}

func mustRefKindID(raw string) typedmemory.RefKindID {
	value, _ := typedmemory.NewRefKindID(raw)
	return value
}
