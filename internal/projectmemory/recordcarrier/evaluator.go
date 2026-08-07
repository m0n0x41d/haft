package recordcarrier

import (
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	recordMembershipEvaluatorRuleV1 = "haft.member-of.project-record-carrier/v1"

	repairMissingSourceV1      = "repair:record-membership-source/missing"
	repairUntrustedSourceV1    = "repair:record-membership-source/untrusted"
	repairInvalidSourceV1      = "repair:record-membership-source/invalid-or-substituted"
	repairProjectMismatchV1    = "repair:record-membership-source/project-mismatch"
	repairEntityMismatchV1     = "repair:record-membership-source/entity-mismatch"
	repairContextMismatchV1    = "repair:record-membership-source/context-mismatch"
	repairMappingMismatchV1    = "repair:record-membership-source/mapping-manifest-mismatch"
	repairAdapterMismatchV1    = "repair:record-membership-source/adapter-version-mismatch"
	repairUnsupportedKindV1    = "repair:record-membership-source/unsupported-kind"
	repairEvaluatorMismatchV1  = "repair:record-membership-source/evaluator-rule-mismatch"
	repairDefinitionMismatchV1 = "repair:record-membership-source/definition-basis-mismatch"
)

const (
	projectRecordKindV1            = "Haft.ProjectRecord"
	decisionRecordKindV1           = "Haft.DecisionRecord"
	specSectionRecordKindV1        = "Haft.SpecSectionRecord"
	evidenceRecordKindV1           = "Haft.EvidenceRecord"
	supportingEpistemeRecordKindV1 = "Haft.SupportingEpistemeRecord"
	workRecordKindV1               = "Haft.WorkRecord"
	workPlanRecordKindV1           = "Haft.WorkPlanRecord"
)

type RecordMembershipEvaluatorV1 struct {
	rule typedmemory.RuleRef
}

func NewRecordMembershipEvaluatorV1() RecordMembershipEvaluatorV1 {
	rule, _ := typedmemory.NewRuleRef(recordMembershipEvaluatorRuleV1)
	return RecordMembershipEvaluatorV1{rule: rule}
}

func (evaluator RecordMembershipEvaluatorV1) RuleRef() typedmemory.RuleRef {
	return evaluator.rule
}

type RecordMembershipEvaluationInputV1 struct {
	ProjectID               projectidentity.ProjectID
	Query                   typedmemory.MemberOfQuery
	EvaluationView          typedmemory.MemberOfEvaluationView
	KindSignature           typedmemory.KindSignatureDefinition
	EntitySet               typedmemory.EntitySetDefinition
	EvaluationProvenance    typedmemory.MemberOfEvaluationProvenance
	ExpectedMappingManifest MappingManifestRef
	ExpectedAdapterVersion  AdapterVersion
	SourceDelivery          RecordMembershipSourceDeliveryV1
}

type RecordMembershipEvaluationRequestV1 struct {
	project                 projectidentity.ProjectID
	query                   typedmemory.MemberOfQuery
	view                    typedmemory.MemberOfEvaluationView
	kindSignature           typedmemory.KindSignatureDefinition
	entitySet               typedmemory.EntitySetDefinition
	provenance              typedmemory.MemberOfEvaluationProvenance
	expectedMappingManifest MappingManifestRef
	expectedAdapterVersion  AdapterVersion
	sourceDelivery          RecordMembershipSourceDeliveryV1
	prerequisites           recordMembershipPrerequisitePosture
}

// recordMembershipPrerequisitePosture keeps historical V1 evaluation and
// selected-TypeEnv C.3.2 evaluation as distinct, sealed request variants. A
// zero or optional certificate cannot enter the registered evaluator.
type recordMembershipPrerequisitePosture interface {
	recordMembershipPrerequisiteVariant()
}

type legacyRecordMembershipPrerequisitesV1 struct{}

func (legacyRecordMembershipPrerequisitesV1) recordMembershipPrerequisiteVariant() {}

type exactC32RecordMembershipPrerequisitesV3 struct {
	certificate      typedmemory.C32PrerequisiteCertificate
	observableInputs []typedmemory.MemberOfObservableInput
}

func (exactC32RecordMembershipPrerequisitesV3) recordMembershipPrerequisiteVariant() {}

func NewRecordMembershipEvaluationRequestV1(
	input RecordMembershipEvaluationInputV1,
) (RecordMembershipEvaluationRequestV1, error) {
	if err := validateProjectID(input.ProjectID); err != nil {
		return RecordMembershipEvaluationRequestV1{}, err
	}
	if _, err := typedmemory.NewMemberOfEvaluationRequest(
		input.Query,
		input.EvaluationView,
	); err != nil {
		return RecordMembershipEvaluationRequestV1{}, fmt.Errorf(
			"record membership query/view: %w",
			err,
		)
	}
	if len(input.KindSignature.CanonicalBytes()) == 0 {
		return RecordMembershipEvaluationRequestV1{}, fmt.Errorf("record membership KindSignature is required")
	}
	if len(input.EntitySet.CanonicalBytes()) == 0 {
		return RecordMembershipEvaluationRequestV1{}, fmt.Errorf("record membership EntitySet is required")
	}
	if len(input.EvaluationProvenance.CanonicalBytes()) == 0 {
		return RecordMembershipEvaluationRequestV1{}, fmt.Errorf("record membership evaluation provenance is required")
	}
	if err := input.ExpectedMappingManifest.Verify(); err != nil {
		return RecordMembershipEvaluationRequestV1{}, fmt.Errorf("expected mapping manifest is invalid")
	}
	if err := input.ExpectedAdapterVersion.Verify(); err != nil {
		return RecordMembershipEvaluationRequestV1{}, fmt.Errorf("expected adapter version is invalid")
	}
	if err := validateRecordMembershipSourceDeliveryV1(input.SourceDelivery); err != nil {
		return RecordMembershipEvaluationRequestV1{}, err
	}
	return RecordMembershipEvaluationRequestV1{
		project:                 input.ProjectID,
		query:                   input.Query,
		view:                    input.EvaluationView,
		kindSignature:           input.KindSignature,
		entitySet:               input.EntitySet,
		provenance:              input.EvaluationProvenance,
		expectedMappingManifest: input.ExpectedMappingManifest,
		expectedAdapterVersion:  input.ExpectedAdapterVersion,
		sourceDelivery:          input.SourceDelivery,
		prerequisites:           legacyRecordMembershipPrerequisitesV1{},
	}, nil
}

// RecordMembershipEvaluationInputV3 is the explicit selected-TypeEnv request.
// It requires the content-addressed result of the complete C.3.2 prerequisite
// chain and cannot be constructed as a legacy request by omission.
type RecordMembershipEvaluationInputV3 struct {
	ProjectID                    projectidentity.ProjectID
	Query                        typedmemory.MemberOfQuery
	EvaluationView               typedmemory.MemberOfEvaluationView
	KindSignature                typedmemory.KindSignatureDefinition
	EntitySet                    typedmemory.EntitySetDefinition
	EvaluationProvenance         typedmemory.MemberOfEvaluationProvenance
	ExpectedMappingManifest      MappingManifestRef
	ExpectedAdapterVersion       AdapterVersion
	SourceDelivery               RecordMembershipSourceDeliveryV1
	Prerequisites                typedmemory.C32PrerequisiteCertificate
	PrerequisiteObservableInputs []typedmemory.MemberOfObservableInput
}

// RecordMembershipEvaluationRequestV3 is a typestate wrapper around the exact
// registered request. RegisteredRequest is available only after construction
// has validated the required C.3.2 certificate.
type RecordMembershipEvaluationRequestV3 struct {
	registered RecordMembershipEvaluationRequestV1
}

func NewRecordMembershipEvaluationRequestV3(
	input RecordMembershipEvaluationInputV3,
) (RecordMembershipEvaluationRequestV3, error) {
	posture, err := typedmemory.NewC32PrerequisiteMemberOfBasisV3(
		input.Prerequisites,
	)
	if err != nil {
		return RecordMembershipEvaluationRequestV3{}, err
	}
	prerequisiteInputs, err := normalizeRecordMembershipPrerequisiteInputs(
		input.PrerequisiteObservableInputs,
	)
	if err != nil {
		return RecordMembershipEvaluationRequestV3{}, err
	}
	legacy, err := NewRecordMembershipEvaluationRequestV1(
		RecordMembershipEvaluationInputV1{
			ProjectID:               input.ProjectID,
			Query:                   input.Query,
			EvaluationView:          input.EvaluationView,
			KindSignature:           input.KindSignature,
			EntitySet:               input.EntitySet,
			EvaluationProvenance:    input.EvaluationProvenance,
			ExpectedMappingManifest: input.ExpectedMappingManifest,
			ExpectedAdapterVersion:  input.ExpectedAdapterVersion,
			SourceDelivery:          input.SourceDelivery,
		},
	)
	if err != nil {
		return RecordMembershipEvaluationRequestV3{}, err
	}
	legacy.prerequisites = exactC32RecordMembershipPrerequisitesV3{
		certificate:      posture.Certificate(),
		observableInputs: prerequisiteInputs,
	}
	if err := validateRecordMembershipEvaluationRequestV1(legacy); err != nil {
		return RecordMembershipEvaluationRequestV3{}, err
	}
	return RecordMembershipEvaluationRequestV3{registered: legacy}, nil
}

func (request RecordMembershipEvaluationRequestV3) RegisteredRequest() RecordMembershipEvaluationRequestV1 {
	return request.registered
}

func (evaluator RecordMembershipEvaluatorV1) Evaluate(
	request RecordMembershipEvaluationRequestV1,
) (typedmemory.MemberOfJudgement, error) {
	if err := validateRecordMembershipEvaluatorV1(evaluator); err != nil {
		return nil, err
	}
	if err := validateRecordMembershipEvaluationRequestV1(request); err != nil {
		return nil, err
	}
	source, outcome := evaluator.verifySource(request)
	if outcome != "" {
		return evaluator.undefinedForSource(request, outcome)
	}
	if source.ProjectID() != request.project {
		return evaluator.undefinedForObservable(request, repairProjectMismatchV1)
	}
	if source.EntityID() != request.query.EntityID() {
		return evaluator.undefinedForObservable(request, repairEntityMismatchV1)
	}
	context := request.query.ContextSlice().Context()
	if source.BoundedContext() != context {
		return evaluator.undefinedForObservable(request, repairContextMismatchV1)
	}
	if source.Binding().MappingManifestRef() != request.expectedMappingManifest {
		return evaluator.undefinedForObservable(request, repairMappingMismatchV1)
	}
	if source.Binding().AdapterVersion() != request.expectedAdapterVersion {
		return evaluator.undefinedForObservable(request, repairAdapterMismatchV1)
	}
	requestedKind := request.query.ValueKind().ID().String()
	member, supported := recordCarrierMembership(requestedKind, source.RecordVariant())
	if !supported {
		return evaluator.undefinedForKind(request, repairUnsupportedKindV1)
	}
	if request.kindSignature.Evaluator() != evaluator.rule {
		return evaluator.undefinedForEvaluator(request, repairEvaluatorMismatchV1)
	}
	observableInputs := []typedmemory.MemberOfObservableInput{
		source.ObservableInput(),
	}
	if posture, exact := request.prerequisites.(exactC32RecordMembershipPrerequisitesV3); exact {
		observableInputs = make(
			[]typedmemory.MemberOfObservableInput,
			0,
			len(posture.observableInputs)+1,
		)
		observableInputs = append(observableInputs, posture.observableInputs...)
		observableInputs = append(observableInputs, source.ObservableInput())
	}
	basisInput := typedmemory.MemberOfBasisInput{
		Query:                request.query,
		EvaluationView:       request.view,
		KindSignature:        request.kindSignature,
		EntitySet:            request.entitySet,
		ObservableInputs:     observableInputs,
		EvaluationProvenance: request.provenance,
	}
	basis, err := recordMembershipBasis(request.prerequisites, basisInput)
	if err != nil {
		return evaluator.undefinedForKind(request, repairDefinitionMismatchV1)
	}
	if member {
		judgement, err := typedmemory.NewMemberOfMember(request.query, basis)
		if err != nil {
			return nil, err
		}
		return judgement, nil
	}
	judgement, err := typedmemory.NewMemberOfNotMember(request.query, basis)
	if err != nil {
		return nil, err
	}
	return judgement, nil
}

func validateRecordMembershipEvaluatorV1(
	evaluator RecordMembershipEvaluatorV1,
) error {
	expected, err := typedmemory.NewRuleRef(recordMembershipEvaluatorRuleV1)
	if err != nil {
		return fmt.Errorf("record membership evaluator rule is invalid: %w", err)
	}
	if evaluator.rule != expected {
		return fmt.Errorf("record membership evaluator is invalid")
	}
	return nil
}

func (evaluator RecordMembershipEvaluatorV1) verifySource(
	request RecordMembershipEvaluationRequestV1,
) (RecordMembershipSourceV1, string) {
	switch delivery := request.sourceDelivery.(type) {
	case *trustedRecordMembershipSourceDeliveryV1:
		source, err := VerifyRecordMembershipSourceV1(
			delivery.expected,
			delivery.canonical,
		)
		if err != nil {
			return RecordMembershipSourceV1{}, repairInvalidSourceV1
		}
		return source, ""
	case *untrustedRecordMembershipSourceDeliveryV1:
		_, err := VerifyRecordMembershipSourceV1(
			delivery.expected,
			delivery.canonical,
		)
		if err != nil {
			return RecordMembershipSourceV1{}, repairInvalidSourceV1
		}
		return RecordMembershipSourceV1{}, repairUntrustedSourceV1
	case *missingRecordMembershipSourceDeliveryV1:
		return RecordMembershipSourceV1{}, repairMissingSourceV1
	default:
		return RecordMembershipSourceV1{}, repairMissingSourceV1
	}
}

func validateRecordMembershipEvaluationRequestV1(
	request RecordMembershipEvaluationRequestV1,
) error {
	legacy, err := NewRecordMembershipEvaluationRequestV1(
		RecordMembershipEvaluationInputV1{
			ProjectID:               request.project,
			Query:                   request.query,
			EvaluationView:          request.view,
			KindSignature:           request.kindSignature,
			EntitySet:               request.entitySet,
			EvaluationProvenance:    request.provenance,
			ExpectedMappingManifest: request.expectedMappingManifest,
			ExpectedAdapterVersion:  request.expectedAdapterVersion,
			SourceDelivery:          request.sourceDelivery,
		},
	)
	if err != nil {
		return fmt.Errorf("record membership evaluation request is invalid: %w", err)
	}
	switch posture := request.prerequisites.(type) {
	case legacyRecordMembershipPrerequisitesV1:
		_ = legacy
	case exactC32RecordMembershipPrerequisitesV3:
		if _, err := typedmemory.NewC32PrerequisiteMemberOfBasisV3(
			posture.certificate,
		); err != nil {
			return fmt.Errorf(
				"record membership C.3.2 prerequisite certificate is invalid: %w",
				err,
			)
		}
		if _, err := normalizeRecordMembershipPrerequisiteInputs(
			posture.observableInputs,
		); err != nil {
			return fmt.Errorf(
				"record membership C.3.2 observable basis is invalid: %w",
				err,
			)
		}
	default:
		return fmt.Errorf("record membership prerequisite posture is invalid")
	}
	return nil
}

func normalizeRecordMembershipPrerequisiteInputs(
	values []typedmemory.MemberOfObservableInput,
) ([]typedmemory.MemberOfObservableInput, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf(
			"record membership V3 requires C.3.2 prerequisite observable inputs",
		)
	}
	result := append([]typedmemory.MemberOfObservableInput(nil), values...)
	for _, input := range result {
		rebuilt, err := typedmemory.NewMemberOfObservableInput(
			input.Reference(),
			input.Digest(),
		)
		if err != nil || rebuilt != input {
			return nil, fmt.Errorf(
				"record membership V3 contains an invalid prerequisite observable input",
			)
		}
	}
	slices.SortFunc(
		result,
		func(left, right typedmemory.MemberOfObservableInput) int {
			if left.Reference().String() < right.Reference().String() {
				return -1
			}
			if left.Reference().String() > right.Reference().String() {
				return 1
			}
			if left.Digest().String() < right.Digest().String() {
				return -1
			}
			if left.Digest().String() > right.Digest().String() {
				return 1
			}
			return 0
		},
	)
	for index := 1; index < len(result); index++ {
		if result[index-1] == result[index] {
			return nil, fmt.Errorf(
				"record membership V3 contains duplicate prerequisite observable input",
			)
		}
	}
	return result, nil
}

func recordMembershipBasis(
	posture recordMembershipPrerequisitePosture,
	input typedmemory.MemberOfBasisInput,
) (typedmemory.MemberOfBasis, error) {
	switch value := posture.(type) {
	case legacyRecordMembershipPrerequisitesV1:
		return typedmemory.NewLegacyMemberOfBasisV2(input)
	case exactC32RecordMembershipPrerequisitesV3:
		return typedmemory.NewMemberOfBasisV3(
			typedmemory.MemberOfBasisV3Input{
				Basis:         input,
				Prerequisites: value.certificate,
			},
		)
	default:
		return typedmemory.MemberOfBasis{}, fmt.Errorf(
			"record membership prerequisite posture is invalid",
		)
	}
}

func (evaluator RecordMembershipEvaluatorV1) undefinedForSource(
	request RecordMembershipEvaluationRequestV1,
	repair string,
) (typedmemory.MemberOfJudgement, error) {
	if repair == "" {
		repair = repairInvalidSourceV1
	}
	judgement, err := evaluator.undefinedForObservable(request, repair)
	if err != nil {
		return nil, err
	}
	return judgement, nil
}

func (evaluator RecordMembershipEvaluatorV1) undefinedForObservable(
	request RecordMembershipEvaluationRequestV1,
	repair string,
) (typedmemory.MemberOfJudgement, error) {
	missing, err := typedmemory.MissingObservableInputForMemberOf(
		request.sourceDelivery.expectedObservableRef(),
	)
	if err != nil {
		return nil, err
	}
	return newRecordMembershipUndefined(request, missing, repair)
}

func (evaluator RecordMembershipEvaluatorV1) undefinedForKind(
	request RecordMembershipEvaluationRequestV1,
	repair string,
) (typedmemory.MemberOfJudgement, error) {
	missing, err := typedmemory.MissingKindSignatureForMemberOf(request.query)
	if err != nil {
		return nil, err
	}
	return newRecordMembershipUndefined(request, missing, repair)
}

func (evaluator RecordMembershipEvaluatorV1) undefinedForEvaluator(
	request RecordMembershipEvaluationRequestV1,
	repair string,
) (typedmemory.MemberOfJudgement, error) {
	missing, err := typedmemory.MissingEvaluatorForMemberOf(evaluator.rule)
	if err != nil {
		return nil, err
	}
	return newRecordMembershipUndefined(request, missing, repair)
}

func newRecordMembershipUndefined(
	request RecordMembershipEvaluationRequestV1,
	missing typedmemory.MemberOfMissingBasis,
	repairRaw string,
) (typedmemory.MemberOfJudgement, error) {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return nil, err
	}
	evaluationRequest, err := typedmemory.NewMemberOfEvaluationRequest(
		request.query,
		request.view,
	)
	if err != nil {
		return nil, err
	}
	judgement, err := typedmemory.NewMemberOfUndefined(
		evaluationRequest,
		[]typedmemory.MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		return nil, err
	}
	return judgement, nil
}

func recordCarrierMembership(
	requestedKind string,
	variant ProjectRecordCarrierVariantV1,
) (bool, bool) {
	if requestedKind == projectRecordKindV1 {
		return true, true
	}
	expected, supported := specializationVariantForKind(requestedKind)
	if !supported {
		return false, false
	}
	return sameProjectRecordCarrierVariantV1(expected, variant), true
}

func specializationVariantForKind(
	requestedKind string,
) (ProjectRecordCarrierVariantV1, bool) {
	switch requestedKind {
	case decisionRecordKindV1:
		return DecisionRecordVariantV1{}, true
	case specSectionRecordKindV1:
		return SpecSectionRecordVariantV1{}, true
	case evidenceRecordKindV1:
		return EvidenceRecordVariantV1{}, true
	case supportingEpistemeRecordKindV1:
		return SupportingEpistemeRecordVariantV1{}, true
	case workRecordKindV1:
		return WorkRecordVariantV1{}, true
	case workPlanRecordKindV1:
		return WorkPlanRecordVariantV1{}, true
	default:
		return nil, false
	}
}
