package projectmemory

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofc32"
	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

var (
	ErrCarrierFamilyMembershipRuntimeMissing = errors.New(
		"project-memory carrier-family membership runtime is missing",
	)
	ErrCarrierFamilyMembershipRuntimeInvalid = errors.New(
		"project-memory carrier-family membership runtime is invalid",
	)
)

type carrierFamilySourceSelection interface {
	carrierFamilySourceSelectionVariant()
}

type carrierFamilySourceSelected struct {
	delivery carrierfamily.TrustedMembershipSourceDeliveryV1
}

func (carrierFamilySourceSelected) carrierFamilySourceSelectionVariant() {}

type carrierFamilySourceNotApplicable struct{}

func (carrierFamilySourceNotApplicable) carrierFamilySourceSelectionVariant() {}

type carrierFamilySourceProblemKind uint8

const (
	carrierFamilySourceMalformed carrierFamilySourceProblemKind = iota + 1
	carrierFamilySourceUntrusted
	carrierFamilySourceAmbiguous
)

type carrierFamilySourceUnderdetermined struct {
	problem carrierFamilySourceProblemKind
}

func (carrierFamilySourceUnderdetermined) carrierFamilySourceSelectionVariant() {}

// CarrierFamilyMembershipAdmissionEngine is one exact, package-selected
// carrier-first family. Its constructors never accept a ValueKind, KindID, or
// RuleRef. The selected TypeEnv remains authoritative for the queried
// ValueKind; this engine owns only the callable rule and source grammar.
type CarrierFamilyMembershipAdmissionEngine struct {
	prerequisites memberofc32.Runtime
	rule          typedmemory.RuleRef
	identity      typedmemoryevaluation.MechanismIdentity
	policy        recordmembershipregistration.RegistrationArtifactV1
}

var _ memberofevaluation.MemberOfEvaluationEngine = CarrierFamilyMembershipAdmissionEngine{}
var _ memberofevaluation.SnapshotObservableInputSelector = CarrierFamilyMembershipAdmissionEngine{}

type CarrierFamilyMembershipAdmissionEngineBuilder struct {
	rule        typedmemory.RuleRef
	enumeration typedmemoryevaluation.EntitySetEnumerationRegistry
	visibility  typedmemoryevaluation.CandidateVisibilityRegistry
	definedness typedmemoryevaluation.KindDefinednessRegistry
	identity    typedmemoryevaluation.MechanismIdentity
	policy      recordmembershipregistration.RegistrationArtifactV1
}

func NewCarrierEditionMembershipAdmissionEngineBuilder() CarrierFamilyMembershipAdmissionEngineBuilder {
	return newCarrierFamilyMembershipAdmissionEngineBuilder(
		carrierfamily.CarrierEditionEvaluatorRuleV1(),
	)
}

func NewProjectClaimMembershipAdmissionEngineBuilder() CarrierFamilyMembershipAdmissionEngineBuilder {
	return newCarrierFamilyMembershipAdmissionEngineBuilder(
		carrierfamily.ProjectClaimEvaluatorRuleV1(),
	)
}

func NewPerformedWorkOccurrenceMembershipAdmissionEngineBuilder() CarrierFamilyMembershipAdmissionEngineBuilder {
	return newCarrierFamilyMembershipAdmissionEngineBuilder(
		carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1(),
	)
}

func NewCodeAnchorMembershipAdmissionEngineBuilder() CarrierFamilyMembershipAdmissionEngineBuilder {
	return newCarrierFamilyMembershipAdmissionEngineBuilder(
		carrierfamily.CodeAnchorEvaluatorRuleV1(),
	)
}

func newCarrierFamilyMembershipAdmissionEngineBuilder(
	rule typedmemory.RuleRef,
) CarrierFamilyMembershipAdmissionEngineBuilder {
	return CarrierFamilyMembershipAdmissionEngineBuilder{rule: rule}
}

func (builder CarrierFamilyMembershipAdmissionEngineBuilder) SetEntitySetEnumeration(
	registry typedmemoryevaluation.EntitySetEnumerationRegistry,
) CarrierFamilyMembershipAdmissionEngineBuilder {
	builder.enumeration = registry.Clone()
	return builder
}

func (builder CarrierFamilyMembershipAdmissionEngineBuilder) SetCandidateVisibility(
	registry typedmemoryevaluation.CandidateVisibilityRegistry,
) CarrierFamilyMembershipAdmissionEngineBuilder {
	builder.visibility = registry.Clone()
	return builder
}

func (builder CarrierFamilyMembershipAdmissionEngineBuilder) SetKindDefinedness(
	registry typedmemoryevaluation.KindDefinednessRegistry,
) CarrierFamilyMembershipAdmissionEngineBuilder {
	builder.definedness = registry.Clone()
	return builder
}

func (builder CarrierFamilyMembershipAdmissionEngineBuilder) SetMechanismIdentity(
	identity typedmemoryevaluation.MechanismIdentity,
) CarrierFamilyMembershipAdmissionEngineBuilder {
	builder.identity = identity
	return builder
}

func (builder CarrierFamilyMembershipAdmissionEngineBuilder) SetRegistrationPolicy(
	policy recordmembershipregistration.RegistrationArtifactV1,
) CarrierFamilyMembershipAdmissionEngineBuilder {
	builder.policy = policy
	return builder
}

func (builder CarrierFamilyMembershipAdmissionEngineBuilder) Build() (
	CarrierFamilyMembershipAdmissionEngine,
	error,
) {
	prerequisites, err := memberofc32.NewRuntime(
		builder.enumeration,
		builder.visibility,
		builder.definedness,
	)
	if err != nil {
		return CarrierFamilyMembershipAdmissionEngine{}, fmt.Errorf(
			"build carrier-family C.3.2 runtime: %w",
			err,
		)
	}
	engine := CarrierFamilyMembershipAdmissionEngine{
		prerequisites: prerequisites,
		rule:          builder.rule,
		identity:      builder.identity,
		policy:        builder.policy,
	}
	if err := engine.validate(); err != nil {
		return CarrierFamilyMembershipAdmissionEngine{}, err
	}
	return engine, nil
}

func (engine CarrierFamilyMembershipAdmissionEngine) EvaluateMemberOf(
	ctx context.Context,
	input memberofevaluation.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	if ctx == nil {
		return nil, fmt.Errorf("carrier-family MemberOf context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := engine.validate(); err != nil || !input.Valid() {
		return nil, ErrCarrierFamilyMembershipRuntimeInvalid
	}
	request := input.Request()
	query := request.Query()
	signature, found := input.Environment().KindSignatureDefinition(
		query.ValueKind(),
		query.ContextSlice().Context(),
	)
	if !found || signature.Evaluator() != engine.rule {
		return carrierFamilyUndefinedForSignature(request)
	}
	entitySet, found := input.Environment().EntitySetDefinition(signature.EntitySet())
	if !found {
		return carrierFamilyUndefinedForEntitySet(request, signature.EntitySet())
	}
	prerequisite, err := engine.prerequisites.Evaluate(memberofc32.Input{
		Request:   request,
		Signature: signature,
		EntitySet: entitySet,
		Universe:  input.PersistedEntityUniverse(),
	})
	if err != nil {
		return nil, err
	}
	satisfied, ok := prerequisite.(memberofc32.Satisfied)
	if !ok {
		undefined, exact := prerequisite.(memberofc32.Undefined)
		if !exact {
			return nil, ErrCarrierFamilyMembershipRuntimeInvalid
		}
		return undefined.Judgement(), nil
	}
	selection := engine.selectTrustedSource(
		input,
		withoutPrerequisiteObservableBlobs(
			input.ObservableInputs(),
			satisfied.ObservableInputs(),
		),
	)
	selected, ok := selection.(carrierFamilySourceSelected)
	if !ok {
		switch selection.(type) {
		case carrierFamilySourceNotApplicable:
			return carrierFamilyUndefinedForNoApplicableSource(request)
		case carrierFamilySourceUnderdetermined:
			return carrierFamilyUndefinedForUnusableSource(request)
		default:
			return nil, ErrCarrierFamilyMembershipRuntimeInvalid
		}
	}
	source := selected.delivery.Source()
	provenance, err := carrierFamilyMembershipProvenance(
		engine.rule,
		engine.identity,
	)
	if err != nil {
		return nil, err
	}
	inputs := append(
		satisfied.ObservableInputs(),
		source.ObservableInput(),
	)
	basis, err := typedmemory.NewMemberOfBasisV3(
		typedmemory.MemberOfBasisV3Input{
			Basis: typedmemory.MemberOfBasisInput{
				Query:                query,
				EvaluationView:       request.View(),
				KindSignature:        signature,
				EntitySet:            entitySet,
				ObservableInputs:     inputs,
				EvaluationProvenance: provenance,
			},
			Prerequisites: satisfied.Certificate(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct carrier-family MemberOf basis: %w", err)
	}
	return typedmemory.NewMemberOfMember(query, basis)
}

func (engine CarrierFamilyMembershipAdmissionEngine) SelectSnapshotObservableInputs(
	input memberofevaluation.MemberOfEvaluationInput,
) memberofevaluation.SnapshotObservableInputSelection {
	if engine.validate() != nil || !input.Valid() {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	selection := engine.selectTrustedSource(input, input.ObservableInputs())
	switch selected := selection.(type) {
	case carrierFamilySourceNotApplicable:
		return memberofevaluation.NewSnapshotObservableInputsNotApplicable()
	case carrierFamilySourceSelected:
		source := selected.delivery.Source()
		for _, blob := range input.ObservableInputs() {
			if blob.Reference() != source.ObservableInput().Reference() ||
				blob.Digest() != source.ObservableInput().Digest() {
				continue
			}
			result, err := memberofevaluation.NewSnapshotObservableInputsSelected(
				[]memberofevaluation.ObservableInputBlob{blob},
			)
			if err == nil {
				return result
			}
		}
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	default:
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
}

func (engine CarrierFamilyMembershipAdmissionEngine) selectTrustedSource(
	input memberofevaluation.MemberOfEvaluationInput,
	blobs []memberofevaluation.ObservableInputBlob,
) carrierFamilySourceSelection {
	query := input.Request().Query()
	selected := make([]carrierfamily.TrustedMembershipSourceDeliveryV1, 0, 1)
	selectedIdentities := make(map[string]struct{})
	problem := carrierFamilySourceProblemKind(0)
	for _, blob := range blobs {
		if !carrierfamily.IsMembershipSourceReference(blob.Reference()) {
			continue
		}
		expected, err := typedmemory.NewMemberOfObservableInput(
			blob.Reference(),
			blob.Digest(),
		)
		if err != nil {
			problem = strongerCarrierFamilySourceProblem(
				problem,
				carrierFamilySourceMalformed,
			)
			continue
		}
		source, err := carrierfamily.VerifyMembershipSourceV1(
			expected,
			blob.Bytes(),
		)
		if err != nil {
			problem = strongerCarrierFamilySourceProblem(
				problem,
				carrierFamilySourceMalformed,
			)
			continue
		}
		if source.ProjectID() != input.ProjectID() ||
			source.EntityID() != query.EntityID() ||
			source.BoundedContext() != query.ContextSlice().Context() ||
			source.EvaluatorRule() != engine.rule {
			continue
		}
		delivery, err := carrierfamily.NewTrustedMembershipSourceDeliveryV1(
			engine.policy,
			expected,
			blob.Bytes(),
		)
		if err != nil {
			problem = strongerCarrierFamilySourceProblem(
				problem,
				carrierFamilySourceUntrusted,
			)
			continue
		}
		identity := expected.Reference().String() + "\x00" + expected.Digest().String()
		if _, duplicate := selectedIdentities[identity]; duplicate {
			continue
		}
		selectedIdentities[identity] = struct{}{}
		selected = append(selected, delivery)
	}
	if len(selected) > 1 {
		problem = strongerCarrierFamilySourceProblem(
			problem,
			carrierFamilySourceAmbiguous,
		)
	}
	if problem != 0 {
		return carrierFamilySourceUnderdetermined{problem: problem}
	}
	if len(selected) == 0 {
		return carrierFamilySourceNotApplicable{}
	}
	return carrierFamilySourceSelected{delivery: selected[0]}
}

func strongerCarrierFamilySourceProblem(
	left carrierFamilySourceProblemKind,
	right carrierFamilySourceProblemKind,
) carrierFamilySourceProblemKind {
	if right > left {
		return right
	}
	return left
}

func (engine CarrierFamilyMembershipAdmissionEngine) validate() error {
	if engine.rule.String() == "" ||
		engine.identity.Role() != typedmemoryevaluation.EvaluatorRole ||
		engine.identity.ArtifactRef().String() == "" ||
		engine.identity.Edition().String() == "" ||
		engine.identity.Digest().String() == "" {
		return ErrCarrierFamilyMembershipRuntimeMissing
	}
	if err := engine.policy.Verify(); err != nil {
		return fmt.Errorf("%w: %v", ErrCarrierFamilyMembershipRuntimeInvalid, err)
	}
	evaluator := engine.policy.Evaluator()
	delivery := engine.policy.SourceDeliveryBoundary()
	coordinatesMatch := evaluator.Rule() == engine.rule &&
		delivery.Rule() == engine.rule &&
		evaluator.Artifact() == engine.identity.ArtifactRef() &&
		evaluator.Edition() == engine.identity.Edition() &&
		evaluator.Digest() == engine.identity.Digest() &&
		delivery.Artifact() == engine.identity.ArtifactRef() &&
		delivery.Edition() == engine.identity.Edition() &&
		delivery.Digest() == engine.identity.Digest()
	if !coordinatesMatch {
		return ErrCarrierFamilyMembershipRuntimeInvalid
	}
	return nil
}

func carrierFamilyMembershipProvenance(
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) (typedmemory.MemberOfEvaluationProvenance, error) {
	reference, err := typedmemory.NewProvenanceRef(
		"prov:haft-carrier-family-membership:" +
			rule.String() + ":" + identity.Digest().String(),
	)
	if err != nil {
		return typedmemory.MemberOfEvaluationProvenance{}, err
	}
	return typedmemory.NewMemberOfEvaluationProvenance(
		typedmemory.MemberOfEvaluationProvenanceInput{
			Reference:         reference,
			EvaluatorArtifact: identity.ArtifactRef(),
			EvaluatorEdition:  identity.Edition(),
			EvaluatorDigest:   identity.Digest(),
		},
	)
}

func carrierFamilyUndefinedForSignature(
	request typedmemory.MemberOfEvaluationRequest,
) (typedmemory.MemberOfJudgement, error) {
	query := request.Query()
	missing, err := typedmemory.MissingKindSignatureForMemberOf(query)
	if err != nil {
		return nil, err
	}
	return carrierFamilyUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/carrier-family-kind-signature",
	)
}

func carrierFamilyUndefinedForEntitySet(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinitionRef,
) (typedmemory.MemberOfJudgement, error) {
	missing, err := typedmemory.MissingEntitySetForMemberOf(entitySet)
	if err != nil {
		return nil, err
	}
	return carrierFamilyUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/carrier-family-entity-set",
	)
}

func carrierFamilyUndefinedForNoApplicableSource(
	request typedmemory.MemberOfEvaluationRequest,
) (typedmemory.MemberOfJudgement, error) {
	query := request.Query()
	missing, err := typedmemory.NoApplicableObservableSourceForMemberOf(query)
	if err != nil {
		return nil, err
	}
	return carrierFamilyUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/carrier-family-source",
	)
}

func carrierFamilyUndefinedForUnusableSource(
	request typedmemory.MemberOfEvaluationRequest,
) (typedmemory.MemberOfJudgement, error) {
	query := request.Query()
	missing, err := typedmemory.MissingUniqueTrustedObservableSourceForMemberOf(query)
	if err != nil {
		return nil, err
	}
	return carrierFamilyUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/carrier-family-trusted-source",
	)
}

func carrierFamilyUndefined(
	request typedmemory.MemberOfEvaluationRequest,
	missing []typedmemory.MemberOfMissingBasis,
	repairRaw string,
) (typedmemory.MemberOfJudgement, error) {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewMemberOfUndefined(request, missing, repair)
}
