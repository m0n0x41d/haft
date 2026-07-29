package projecttypeenvassertionrevalidation

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// Input contains only exact immutable observations. TargetRuntime is an
// in-process non-serializable capability minted by exact X comparison.
type Input struct {
	CurrentGraph                  projectgraphobservation.CurrentProjectGraphObservation
	TargetTypeEnv                 typedmemory.TypeEnv
	TargetRuntime                 projecttypeenvruntime.ExactTargetRuntimeRegistry
	ExactTargetReferenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView
}

// Revalidate derives every active-assertion outcome and the complete sealed
// report. It performs no IO and grants no Stage/head/authority capability.
func Revalidate(input Input) (Report, error) {
	if err := input.CurrentGraph.Verify(); err != nil {
		return Report{}, fmt.Errorf("current project graph observation: %w", err)
	}
	targetRef, err := typedmemory.ParseTypeEnvRef(
		input.TargetTypeEnv.Ref().String(),
	)
	if err != nil || targetRef != input.TargetTypeEnv.Ref() {
		return Report{}, fmt.Errorf("target executable TypeEnv is required")
	}
	if !input.TargetRuntime.Valid() {
		return Report{}, fmt.Errorf("exact target runtime registry is required")
	}
	runtimeBasis, runtimeBasisAvailable := input.TargetRuntime.RuntimeBasisRef()
	runtimeCoordinate, runtimeCoordinateAvailable :=
		input.TargetRuntime.CoordinateDigest()
	codecs, codecsAvailable := input.TargetRuntime.CodecRegistry()
	membership, membershipAvailable :=
		input.TargetRuntime.MemberOfRegistry()
	classification, classificationAvailable :=
		input.TargetRuntime.KindClassificationRegistry()
	if !runtimeBasisAvailable ||
		!runtimeCoordinateAvailable ||
		!codecsAvailable ||
		!membershipAvailable ||
		!classificationAvailable {
		return Report{}, fmt.Errorf(
			"exact target runtime registry did not expose its complete coordinates",
		)
	}

	assertions := input.CurrentGraph.ActiveAssertions().Relations()
	outcomes := make([]AssertionOutcome, 0, len(assertions))
	for _, assertion := range assertions {
		outcome, evaluateErr := evaluateAssertion(
			assertion,
			input.TargetTypeEnv,
			codecs,
			membership,
			classification,
			input.ExactTargetReferenceKindFacts,
			input.CurrentGraph.GraphSnapshotBasis(),
		)
		if evaluateErr != nil {
			return Report{}, fmt.Errorf(
				"revalidate assertion %s: %w",
				assertion.AssertionID().String(),
				evaluateErr,
			)
		}
		outcomes = append(outcomes, outcome)
	}
	return newReport(
		targetRef,
		input.CurrentGraph.GraphSnapshotBasis(),
		runtimeBasis,
		runtimeCoordinate,
		outcomes,
	)
}

func evaluateAssertion(
	assertion projectgraphobservation.CurrentActiveAssertion,
	target typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	membership projecttypeenvruntime.MemberOfEvaluatorRegistry,
	classification projecttypeenvruntime.KindClassificationEvaluatorRegistry,
	referenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
) (AssertionOutcome, error) {
	if err := assertion.Verify(); err != nil {
		return nil, err
	}
	carrier := assertion.Carrier()
	path := relationPath(carrier)
	grounds := make([]Ground, 0)

	targetFragmentRef, err := typedmemory.NewTypedRelationDeclarationFragmentRef(
		target.Ref(),
		carrier.RelationDeclarationFragmentRef().ID(),
	)
	if err != nil {
		return nil, err
	}
	fragment, found := target.TypedRelationDeclarationFragment(targetFragmentRef)
	if !found {
		grounds = append(grounds, mustMissingGround(
			CodeTargetRelationFragmentUnavailable,
			path+".signature",
			"the target TypeEnv does not contain the persisted typed relation declaration fragment",
			"target_relation_declaration_fragment",
			targetFragmentRef.String(),
			"inspect-or-stage-the-required-typed-relation-declaration-fragment",
		))
		return newAssertionOutcome(
			assertion.AssertionID(),
			assertion.Digest(),
			grounds,
		)
	}

	context := carrier.Context()
	_, contextFound := target.BoundedContext(context)
	if !contextFound {
		grounds = append(grounds, mustMissingGround(
			CodeTargetContextUnavailable,
			path+".context",
			"the persisted relation context is absent from the target TypeEnv",
			"target_context",
			context.String(),
			"inspect-or-stage-the-required-bounded-context",
		))
	} else if !fragmentDeclaresContext(fragment, context) {
		grounds = append(grounds, mustInvalidGround(
			CodeRelationFragmentContextMismatch,
			path+".context",
			"the persisted relation context is outside the target fragment's declared contexts",
			"relation_fragment_contexts",
			boundedContextStrings(fragment.Contexts()),
			"actual_context",
			[]string{context.String()},
		))
	}

	bindings := relationBindingMap(carrier)
	for _, slot := range fragment.Slots() {
		binding, present := bindings[slot.SlotKind().String()]
		count := uint64(0)
		if present {
			count = uint64(len(binding.Fillers()))
		}
		slotPath := path + ".slots." + slot.SlotKind().String()
		if !present && slot.Cardinality().Minimum() > 0 {
			grounds = append(grounds, mustInvalidGround(
				CodeMissingSlot,
				slotPath,
				"a slot required by the target typed relation declaration fragment is absent",
				"minimum",
				[]string{strconv.FormatUint(slot.Cardinality().Minimum(), 10)},
				"actual",
				[]string{"0"},
			))
			continue
		}
		if !slot.Cardinality().Allows(count) {
			grounds = append(grounds, mustInvalidGround(
				CodeCardinalityMismatch,
				slotPath,
				"persisted filler count is outside target SlotSpec cardinality",
				"cardinality",
				[]string{cardinalityCoordinate(slot.Cardinality())},
				"actual",
				[]string{strconv.FormatUint(count, 10)},
			))
			continue
		}
		if !present {
			continue
		}
		slotGrounds, evaluateErr := evaluateSlot(
			target,
			codecs,
			membership,
			classification,
			referenceKindFacts,
			graphBasis,
			carrier,
			slot,
			binding,
			slotPath,
		)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		grounds = append(grounds, slotGrounds...)
	}

	constraintView, err := currentAssertionConstraintView(carrier, fragment)
	if err != nil {
		return nil, err
	}
	constraintOutcome := typedmemory.EvaluateRelationConstraints(
		target,
		constraintView,
	)
	constraintGrounds, err := groundsFromDiagnostics(
		constraintOutcome.Diagnostics(),
	)
	if err != nil {
		return nil, err
	}
	grounds = append(grounds, constraintGrounds...)
	grounds = append(grounds, evaluateSlotGroups(target, fragment, bindings, path)...)
	return newAssertionOutcome(
		assertion.AssertionID(),
		assertion.Digest(),
		grounds,
	)
}

func evaluateSlot(
	target typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	membership projecttypeenvruntime.MemberOfEvaluatorRegistry,
	classification projecttypeenvruntime.KindClassificationEvaluatorRegistry,
	referenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	relation projectgraphobservation.CurrentAssertionCarrier,
	slot typedmemory.SlotSpec,
	binding typedmemory.SlotBinding,
	path string,
) ([]Ground, error) {
	fillers := binding.Fillers()
	switch targetSlot := slot.Target().(type) {
	case typedmemory.ValueSlotTarget:
		valueGrounds := make([]Ground, 0)
		for index, filler := range fillers {
			value, isValue := filler.(typedmemory.ValueFiller)
			if !isValue {
				continue
			}
			fillerGrounds, err := evaluateValueFiller(
				target,
				codecs,
				relation.Context(),
				targetSlot,
				value,
				fmt.Sprintf("%s.fillers.%d", path, index),
			)
			if err != nil {
				return nil, err
			}
			valueGrounds = append(valueGrounds, fillerGrounds...)
		}
		return valueGrounds, nil
	case typedmemory.ReferenceSlotTarget:
		referenceGrounds := make([]Ground, 0)
		for index, filler := range fillers {
			reference, isReference := filler.(typedmemory.ReferenceFiller)
			if !isReference {
				continue
			}
			fillerGrounds, err := evaluateReferenceFiller(
				target,
				membership,
				classification,
				referenceKindFacts,
				graphBasis,
				relation,
				targetSlot,
				reference,
				fmt.Sprintf("%s.fillers.%d", path, index),
			)
			if err != nil {
				return nil, err
			}
			referenceGrounds = append(referenceGrounds, fillerGrounds...)
		}
		return referenceGrounds, nil
	default:
		return nil, fmt.Errorf("unsupported target SlotSpec variant %T", slot.Target())
	}
}

func evaluateValueFiller(
	target typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	context typedmemory.BoundedContextRef,
	slot typedmemory.ValueSlotTarget,
	filler typedmemory.ValueFiller,
	path string,
) ([]Ground, error) {
	stored := filler.Value()
	actualKind, err := typedmemory.NewValueKindRef(
		target.Ref(),
		stored.ValueKind().ID(),
	)
	if err != nil {
		return nil, err
	}
	grounds := make([]Ground, 0)
	if !target.IsSubkind(actualKind.ID(), slot.ValueKind().ID()) {
		return []Ground{mustInvalidGround(
			CodeValueKindMismatch,
			path+".value_kind",
			"the persisted ValueKind is neither the target SlotSpec kind nor its subkind",
			"target",
			[]string{slot.ValueKind().String()},
			"actual",
			[]string{actualKind.String()},
		)}, nil
	}
	if _, found := target.KindDefinition(actualKind.ID()); !found {
		return []Ground{mustMissingGround(
			CodeTargetKindUnavailable,
			path+".value_kind",
			"the persisted ValueKind is absent from the target TypeEnv",
			"target_value_kind",
			actualKind.String(),
			"inspect-or-stage-the-required-kind-declaration",
		)}, nil
	}
	if !target.HasKindInContext(context, actualKind.ID()) {
		grounds = append(grounds, mustMissingGround(
			CodeTargetKindUnavailable,
			path+".value_kind",
			"the persisted ValueKind is unavailable in the relation context",
			"context_and_kind",
			context.String()+"|"+actualKind.String(),
			"inspect-or-compile-the-context-kind-availability",
		))
	}
	grounds = append(
		grounds,
		staticKindDisjointnessGrounds(target, actualKind, path+".value_kind")...,
	)
	binding, found := target.ValueBinding(actualKind)
	if !found {
		grounds = append(grounds, mustMissingGround(
			CodeValueBindingUnavailable,
			path+".value_kind",
			"the persisted ValueKind has no target shape/codec binding",
			"target_value_binding",
			actualKind.String(),
			"inspect-or-stage-the-required-value-binding",
		))
		return grounds, nil
	}
	if stored.ValueShape() != binding.ValueShape() ||
		stored.Codec() != binding.Codec() {
		coordinate := strings.Join([]string{
			"stored_shape=" + stored.ValueShape().String(),
			"stored_codec=" + stored.Codec().String(),
			"target_shape=" + binding.ValueShape().String(),
			"target_codec=" + binding.Codec().String(),
		}, "|")
		grounds = append(grounds, mustMissingGround(
			CodeValueMigrationRequired,
			path+".value_binding",
			"the target shape or codec differs from the persisted value envelope; historical bytes cannot be reinterpreted as a migrated value",
			"stored_and_target_binding",
			coordinate,
			"append-a-new-value-and-assertion-under-the-target-binding",
		))
		return grounds, nil
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		actualKind,
		stored.ValueShape(),
		stored.Codec(),
		stored.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		return nil, err
	}
	switch result := typedmemory.VerifyTypedValue(
		codecs,
		binding,
		candidate,
	).(type) {
	case typedmemory.ValidTypedValue:
		if !bytes.Equal(
			result.Value().CanonicalBytes(),
			stored.CanonicalBytes(),
		) {
			grounds = append(grounds, mustInvalidGround(
				CodeValueCanonicalBytesChanged,
				path+".canonical_bytes",
				"the target codec reinterprets persisted canonical bytes",
				"persisted_sha256",
				[]string{digestCoordinate(stored.CanonicalBytes())},
				"target_sha256",
				[]string{digestCoordinate(result.Value().CanonicalBytes())},
			))
		}
	case typedmemory.InvalidTypedValue:
		projected, projectErr := groundsFromDiagnostics(result.Diagnostics())
		if projectErr != nil {
			return nil, projectErr
		}
		grounds = append(grounds, projected...)
	case typedmemory.UnderdeterminedTypedValue:
		projected, projectErr := groundsFromDiagnostics(result.Diagnostics())
		if projectErr != nil {
			return nil, projectErr
		}
		grounds = append(grounds, projected...)
	default:
		return nil, fmt.Errorf("typed-value verifier returned unknown result %T", result)
	}
	return grounds, nil
}

func evaluateReferenceFiller(
	target typedmemory.TypeEnv,
	membership projecttypeenvruntime.MemberOfEvaluatorRegistry,
	classification projecttypeenvruntime.KindClassificationEvaluatorRegistry,
	referenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	relation projectgraphobservation.CurrentAssertionCarrier,
	slot typedmemory.ReferenceSlotTarget,
	filler typedmemory.ReferenceFiller,
	path string,
) ([]Ground, error) {
	if filler.Reference().RefKind().ID() != slot.ReferenceKind().ID() {
		return nil, nil
	}
	grounds := make([]Ground, 0)
	definition, found := target.RefKindDefinition(slot.ReferenceKind())
	if !found {
		return []Ground{mustMissingGround(
			CodeRefKindDefinitionMissing,
			path+".ref_kind",
			"the target RefKind has no active referent definition",
			"target_refkind",
			slot.ReferenceKind().String(),
			"inspect-or-stage-the-required-refkind-definition",
		)}, nil
	}
	if !target.IsSubkind(
		slot.ValueKind().ID(),
		definition.ValueKind().ID(),
	) {
		return []Ground{mustInvalidGround(
			CodeRefKindReferentMismatch,
			path+".ref_kind",
			"the target SlotSpec kind is incompatible with its RefKind referent",
			"refkind_referent",
			[]string{definition.ValueKind().String()},
			"slot_value_kind",
			[]string{slot.ValueKind().String()},
		)}, nil
	}
	if !target.HasKindInContext(
		relation.Context(),
		slot.ValueKind().ID(),
	) {
		grounds = append(grounds, mustMissingGround(
			CodeTargetKindUnavailable,
			path+".value_kind",
			"the target reference ValueKind is unavailable in the relation context",
			"context_and_kind",
			relation.Context().String()+"|"+slot.ValueKind().String(),
			"inspect-or-compile-the-context-kind-availability",
		))
	}
	grounds = append(
		grounds,
		staticKindDisjointnessGrounds(
			target,
			slot.ValueKind(),
			path+".value_kind",
		)...,
	)
	localKind, err := typedmemory.NewLocalKindRef(
		slot.ValueKind(),
		relation.Context(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct target local kind for reference revalidation: %w",
			err,
		)
	}
	classificationSignature, classified :=
		target.KindClassificationSignatureDefinition(localKind)
	if classified {
		if !kindClassificationRegistryContainsRule(
			classification,
			classificationSignature.Criterion(),
		) {
			grounds = append(grounds, mustMissingGround(
				CodeKindClassificationEvaluatorMissing,
				path+".value_kind",
				"the exact target runtime has no evaluator for the current KindClassification signature",
				"kind_classification_evaluator",
				classificationSignature.Criterion().String(),
				"install-the-exact-target-x-kind-classification-evaluator",
			))
			return grounds, nil
		}
		classificationGrounds, evaluateErr :=
			evaluateExactTargetKindClassification(
				target,
				referenceKindFacts,
				graphBasis,
				relation,
				localKind,
				classificationSignature,
				filler,
				path,
			)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		grounds = append(grounds, classificationGrounds...)
		return grounds, nil
	}
	kindSignature, found := target.KindSignatureDefinition(
		slot.ValueKind(),
		relation.Context(),
	)
	if !found {
		grounds = append(grounds, mustMissingGround(
			CodeKindSignatureUnavailable,
			path+".value_kind",
			"target C has no executable C.3.2 KindSignature for this context",
			"kind_signature",
			slot.ValueKind().String()+"|"+relation.Context().String(),
			"inspect-or-stage-the-required-kind-signature",
		))
		return grounds, nil
	}
	if _, found := target.EntitySetDefinition(
		kindSignature.EntitySet(),
	); !found {
		grounds = append(grounds, mustMissingGround(
			CodeEntitySetUnavailable,
			path+".value_kind",
			"target C has no EntitySet required by the KindSignature",
			"entity_set",
			kindSignature.EntitySet().String(),
			"inspect-or-stage-the-required-entity-set",
		))
		return grounds, nil
	}
	if !registryContainsRule(membership, kindSignature.Evaluator()) {
		grounds = append(grounds, mustMissingGround(
			CodeMemberOfEvaluatorMissing,
			path+".value_kind",
			"the exact target runtime has no evaluator for the KindSignature",
			"member_of_evaluator",
			kindSignature.Evaluator().String(),
			"install-the-exact-target-x-evaluator",
		))
		return grounds, nil
	}
	memberOfGrounds, err := evaluateExactTargetMemberOf(
		target,
		referenceKindFacts,
		graphBasis,
		relation,
		slot,
		filler,
		path,
	)
	if err != nil {
		return nil, err
	}
	grounds = append(grounds, memberOfGrounds...)
	return grounds, nil
}

func evaluateExactTargetMemberOf(
	target typedmemory.TypeEnv,
	referenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	relation projectgraphobservation.CurrentAssertionCarrier,
	slot typedmemory.ReferenceSlotTarget,
	filler typedmemory.ReferenceFiller,
	path string,
) ([]Ground, error) {
	query, err := typedmemory.NewMemberOfQuery(
		filler.Entity(),
		slot.ValueKind(),
		relation.Slice(),
	)
	if err != nil {
		return nil, fmt.Errorf("construct exact target MemberOf query: %w", err)
	}
	queryCoordinate := strings.Join([]string{
		filler.Entity().String(),
		slot.ValueKind().String(),
		relation.Slice().Ref().String(),
	}, "|")
	if referenceKindFacts == nil {
		return []Ground{missingMemberOfObservableGround(
			path,
			queryCoordinate,
			"load-the-exact-target-c-membership-fact-view",
		)}, nil
	}
	memberOfVariant, historical :=
		referenceKindFacts.(projectgraphobservation.ExactTargetMemberOfReferenceKindFacts)
	if !historical {
		return nil, fmt.Errorf(
			"historical target C received non-MemberOf reference-kind facts",
		)
	}
	facts := memberOfVariant.MemberOfFacts()
	if facts.TargetTypeEnv() != target.Ref() {
		return nil, fmt.Errorf(
			"exact target MemberOf fact view uses %s, want %s",
			facts.TargetTypeEnv().String(),
			target.Ref().String(),
		)
	}
	factBasis := facts.GraphSnapshotBasis()
	if err := factBasis.Verify(); err != nil {
		return nil, fmt.Errorf("exact target MemberOf graph basis: %w", err)
	}
	if factBasis.Ref() != graphBasis.Ref() {
		return nil, fmt.Errorf(
			"exact target MemberOf fact view graph basis differs from current graph",
		)
	}
	view, err := typedmemory.NewPersistedSnapshotView(
		target.Ref(),
		graphBasis.GraphRevision(),
	)
	if err != nil {
		return nil, fmt.Errorf("construct exact target MemberOf view: %w", err)
	}
	request, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		return nil, fmt.Errorf("construct exact target MemberOf request: %w", err)
	}
	judgement := facts.EvaluateMemberOf(request)
	if !typedmemory.MemberOfJudgementMatchesRequest(request, judgement) {
		return nil, fmt.Errorf("exact target MemberOf judgement is malformed or mismatched")
	}
	switch result := judgement.(type) {
	case typedmemory.MemberOfMember:
		return nil, nil
	case typedmemory.MemberOfNotMember:
		return []Ground{mustInvalidGround(
			CodeMemberOfNotMember,
			path+".value_kind",
			"the persisted entity is not a member of the target reference ValueKind",
			"member_of_query",
			[]string{queryCoordinate},
			"member_of_judgement",
			[]string{result.Digest().String()},
		)}, nil
	case typedmemory.MemberOfUndefined:
		return []Ground{missingMemberOfObservableGround(
			path,
			queryCoordinate,
			result.Repair().String(),
		)}, nil
	default:
		return nil, fmt.Errorf("exact target MemberOf returned unknown judgement %T", judgement)
	}
}

func evaluateExactTargetKindClassification(
	target typedmemory.TypeEnv,
	referenceKindFacts projectgraphobservation.ExactTargetReferenceKindFactView,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	relation projectgraphobservation.CurrentAssertionCarrier,
	localKind typedmemory.LocalKindRef,
	signature typedmemory.KindClassificationSignatureDefinition,
	filler typedmemory.ReferenceFiller,
	path string,
) ([]Ground, error) {
	requestCoordinate := strings.Join([]string{
		filler.Entity().String(),
		localKind.String(),
		signature.Ref().String(),
		relation.Slice().Ref().String(),
	}, "|")
	if referenceKindFacts == nil {
		return []Ground{mustMissingGround(
			CodeKindClassificationBasisMissing,
			path+".value_kind",
			"the exact target C has no current KindClassification fact view",
			"kind_classification_request",
			requestCoordinate,
			"load-the-exact-target-c-kind-classification-fact-view",
		)}, nil
	}
	classificationVariant, current :=
		referenceKindFacts.(projectgraphobservation.ExactTargetKindClassificationReferenceKindFacts)
	if !current {
		return nil, fmt.Errorf(
			"current target C received historical MemberOf reference-kind facts",
		)
	}
	facts := classificationVariant.KindClassificationFacts()
	if facts.TargetTypeEnv() != target.Ref() {
		return nil, fmt.Errorf(
			"exact target KindClassification fact view uses %s, want %s",
			facts.TargetTypeEnv().String(),
			target.Ref().String(),
		)
	}
	factBasis := facts.GraphSnapshotBasis()
	if err := factBasis.Verify(); err != nil {
		return nil, fmt.Errorf(
			"exact target KindClassification graph basis: %w",
			err,
		)
	}
	if factBasis.Ref() != graphBasis.Ref() {
		return nil, fmt.Errorf(
			"exact target KindClassification fact view graph basis differs from current graph",
		)
	}
	candidate, err := typedmemory.NewExactKindEntityCandidate(
		filler.Entity(),
		signature.CandidateValueKind(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct exact target KindClassification candidate: %w",
			err,
		)
	}
	request, err := typedmemory.NewKindClassificationRequest(
		typedmemory.KindClassificationRequestInput{
			Candidate:        candidate,
			LocalKind:        localKind,
			SignatureEdition: signature.Ref(),
			ContextSlice:     relation.Slice(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct exact target KindClassification request: %w",
			err,
		)
	}
	judgement := facts.EvaluateKindClassification(request)
	if !typedmemory.KindClassificationJudgementMatchesRequest(request, judgement) {
		return nil, fmt.Errorf(
			"exact target KindClassification judgement is malformed or mismatched",
		)
	}
	switch result := judgement.(type) {
	case typedmemory.TrueKindClassification:
		return nil, nil
	case typedmemory.FalseKindClassification:
		return []Ground{mustInvalidGround(
			CodeKindClassificationFalse,
			path+".value_kind",
			"the persisted entity is directly classified false for the target local kind",
			"kind_classification_request",
			[]string{requestCoordinate},
			"kind_classification_judgement",
			[]string{result.Digest().String()},
		)}, nil
	case typedmemory.UnknownKindClassification:
		return []Ground{missingKindClassificationBasisGround(
			path,
			requestCoordinate,
			result,
		)}, nil
	default:
		return nil, fmt.Errorf(
			"exact target KindClassification returned unknown judgement %T",
			judgement,
		)
	}
}

func missingKindClassificationBasisGround(
	path string,
	requestCoordinate string,
	judgement typedmemory.UnknownKindClassification,
) Ground {
	reasons := judgement.Reasons()
	coordinates := make([]string, 0, len(reasons))
	repair := "repair:kind-classification/reload-governed-features"
	for index, reason := range reasons {
		coordinate := reason.Kind().String() + "|" + reason.RepairPointer().String()
		coordinates = append(coordinates, coordinate)
		if index == 0 {
			repair = reason.RepairPointer().String()
		}
	}
	requestDetail, err := projecttypeenvassertionreport.NewGroundDetail(
		"kind_classification_request",
		[]string{requestCoordinate},
	)
	if err != nil {
		panic(err)
	}
	reasonDetail, err := projecttypeenvassertionreport.NewGroundDetail(
		"unknown_reasons",
		coordinates,
	)
	if err != nil {
		panic(err)
	}
	ground, err := projecttypeenvassertionreport.NewMissingBasisGround(
		CodeKindClassificationBasisMissing,
		path+".value_kind",
		"the exact target C cannot determine direct kind classification for this persisted entity",
		[]GroundDetail{requestDetail, reasonDetail},
		repair,
	)
	if err != nil {
		panic(err)
	}
	return ground
}

func missingMemberOfObservableGround(
	path string,
	queryCoordinate string,
	repair string,
) Ground {
	return mustMissingGround(
		CodeMemberOfObservableMissing,
		path+".value_kind",
		"the exact target C cannot determine membership for this persisted entity",
		"member_of_query",
		queryCoordinate,
		repair,
	)
}

func staticKindDisjointnessGrounds(
	target typedmemory.TypeEnv,
	valueKind typedmemory.ValueKindRef,
	path string,
) []Ground {
	grounds := make([]Ground, 0)
	for _, rule := range target.Constraints() {
		constraint, isDisjoint := rule.(typedmemory.KindDisjointConstraint)
		if !isDisjoint {
			continue
		}
		matches := make([]string, 0)
		for _, constrainedKind := range constraint.Kinds() {
			if target.IsSubkind(valueKind.ID(), constrainedKind) {
				matches = append(matches, constrainedKind.String())
			}
		}
		if len(matches) <= 1 {
			continue
		}
		sort.Strings(matches)
		grounds = append(grounds, mustInvalidGround(
			CodeStaticKindDisjointness,
			path,
			"the target ValueKind is a subkind of mutually disjoint kinds",
			"constraint",
			[]string{constraint.ID().String()},
			"matched_disjoint_kinds",
			matches,
		))
	}
	return grounds
}

func evaluateSlotGroups(
	target typedmemory.TypeEnv,
	fragment typedmemory.TypedRelationDeclarationFragment,
	bindings map[string]typedmemory.SlotBinding,
	path string,
) []Ground {
	grounds := make([]Ground, 0)
	for _, rule := range target.Constraints() {
		constraint, isGroup := rule.(typedmemory.SlotGroupConstraint)
		if !isGroup || constraint.Signature() != fragment.Ref() {
			continue
		}
		present := 0
		for _, slot := range constraint.Slots() {
			binding, found := bindings[slot.String()]
			if found && len(binding.Fillers()) > 0 {
				present++
			}
		}
		if slotGroupCountValid(
			constraint.Mode(),
			present,
			len(constraint.Slots()),
		) {
			continue
		}
		grounds = append(grounds, mustInvalidGround(
			CodeSlotGroupMismatch,
			path+".slots",
			"persisted slot presence does not satisfy the target slot-group law",
			"constraint_and_mode",
			[]string{
				constraint.ID().String(),
				constraint.Mode().String(),
			},
			"present",
			[]string{strconv.Itoa(present)},
		))
	}
	return grounds
}

func slotGroupCountValid(
	mode typedmemory.SlotGroupMode,
	present int,
	total int,
) bool {
	switch mode {
	case typedmemory.SlotGroupAllOrNone:
		return present == 0 || present == total
	case typedmemory.SlotGroupAtLeastOne:
		return present >= 1
	case typedmemory.SlotGroupExactlyOne:
		return present == 1
	default:
		return false
	}
}

func registryContainsRule(
	registry projecttypeenvruntime.MemberOfEvaluatorRegistry,
	rule typedmemory.RuleRef,
) bool {
	for _, registration := range registry.Registrations() {
		if registration.RuleRef() == rule {
			return true
		}
	}
	return false
}

func kindClassificationRegistryContainsRule(
	registry projecttypeenvruntime.KindClassificationEvaluatorRegistry,
	rule typedmemory.RuleRef,
) bool {
	_, found := registry.Registration(rule)
	return found
}

func groundsFromDiagnostics(
	diagnostics []typedmemory.Diagnostic,
) ([]Ground, error) {
	grounds := make([]Ground, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		ground, err :=
			projecttypeenvassertionreport.NewGroundFromDiagnostic(diagnostic)
		if err != nil {
			return nil, err
		}
		grounds = append(grounds, ground)
	}
	return grounds, nil
}

func mustInvalidGround(
	code GroundCode,
	path string,
	message string,
	expectedKey string,
	expected []string,
	actualKey string,
	actual []string,
) Ground {
	expectedDetail, err := projecttypeenvassertionreport.NewGroundDetail(
		expectedKey,
		expected,
	)
	if err != nil {
		panic(err)
	}
	actualDetail, err := projecttypeenvassertionreport.NewGroundDetail(
		actualKey,
		actual,
	)
	if err != nil {
		panic(err)
	}
	ground, err := projecttypeenvassertionreport.NewInvalidGround(
		code,
		path,
		message,
		[]GroundDetail{
			expectedDetail,
			actualDetail,
		},
	)
	if err != nil {
		panic(err)
	}
	return ground
}

func mustMissingGround(
	code GroundCode,
	path string,
	message string,
	requiredKey string,
	required string,
	repair string,
) Ground {
	detail, err := projecttypeenvassertionreport.NewGroundDetail(
		requiredKey,
		[]string{required},
	)
	if err != nil {
		panic(err)
	}
	ground, err := projecttypeenvassertionreport.NewMissingBasisGround(
		code,
		path,
		message,
		[]GroundDetail{detail},
		repair,
	)
	if err != nil {
		panic(err)
	}
	return ground
}

func relationPath(
	relation projectgraphobservation.CurrentAssertionCarrier,
) string {
	return "assertions." + relation.AssertionID().String()
}

func fragmentDeclaresContext(
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

func boundedContextStrings(
	contexts []typedmemory.BoundedContextRef,
) []string {
	result := make([]string, 0, len(contexts))
	for _, context := range contexts {
		result = append(result, context.String())
	}
	sort.Strings(result)
	return result
}

func relationBindingMap(
	relation projectgraphobservation.CurrentAssertionCarrier,
) map[string]typedmemory.SlotBinding {
	result := make(map[string]typedmemory.SlotBinding, len(relation.Bindings()))
	for _, binding := range relation.Bindings() {
		result[binding.Name().String()] = binding
	}
	return result
}

func currentAssertionConstraintView(
	carrier projectgraphobservation.CurrentAssertionCarrier,
	fragment typedmemory.TypedRelationDeclarationFragment,
) (typedmemory.RelationConstraintEvaluationView, error) {
	switch value := carrier.(type) {
	case projectgraphobservation.CurrentLegacyRelation:
		return typedmemory.NewRelationConstraintEvaluationViewFromInstanceForFragment(
			value.Relation(),
			fragment,
		)
	case projectgraphobservation.CurrentRelationalAssertion:
		return typedmemory.
			NewRelationConstraintEvaluationViewFromRelationalAssertionForFragment(
				value.Assertion(),
				fragment,
			)
	default:
		return typedmemory.RelationConstraintEvaluationView{}, fmt.Errorf(
			"unsupported current assertion carrier %T",
			carrier,
		)
	}
}

func cardinalityCoordinate(cardinality typedmemory.Cardinality) string {
	maximum, bounded := cardinality.Maximum().BoundedValue()
	if !bounded {
		return strconv.FormatUint(cardinality.Minimum(), 10) + "..*"
	}
	return strconv.FormatUint(cardinality.Minimum(), 10) +
		".." +
		strconv.FormatUint(maximum, 10)
}

func digestCoordinate(raw []byte) string {
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("%x", digest[:])
}
