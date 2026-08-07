package projectmemory

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorykindruntime"
)

const projectEntityMemberOfRule = "haft.member-of.project-entity/v1"

var (
	ErrProjectEntityMembershipRuntimeMissing = errors.New(
		"project-memory project-entity membership runtime is missing",
	)
	ErrProjectEntityMembershipRuntimeInvalid = errors.New(
		"project-memory project-entity membership runtime is invalid",
	)
)

// ProjectEntityMembershipAdmissionEngine evaluates the local-practice
// project-entity family from one exact persisted-entity universe. Entity
// presence is not itself the final judgement: the selected EntitySet and
// KindSignature evaluators still run and their successful results are sealed
// into the C.3.2 v3 prerequisite certificate.
//
// A persisted concern uses the persisted-universe posture. A genuinely
// prospective U.Entity declaration is visible only through the selected
// PriorBatchDeclarationsVisible evaluator and carries that exact performed
// visibility result into the C.3.2 certificate. A task adapter that needs an
// already established concern must still resolve one persisted identity before
// constructing its relation candidate.
type ProjectEntityMembershipAdmissionEngine struct {
	entitySetEnumeration projecttypeenvruntime.EntitySetEnumerationEvaluatorRegistry
	candidateVisibility  projecttypeenvruntime.CandidateVisibilityEvaluatorRegistry
	kindDefinedness      projecttypeenvruntime.KindDefinednessEvaluatorRegistry
	rule                 typedmemory.RuleRef
	identity             typedmemoryevaluation.MechanismIdentity
}

var _ memberofevaluation.MemberOfEvaluationEngine = ProjectEntityMembershipAdmissionEngine{}
var _ memberofevaluation.SnapshotObservableInputSelector = ProjectEntityMembershipAdmissionEngine{}

type ProjectEntityMembershipAdmissionEngineBuilder struct {
	entitySetEnumeration projecttypeenvruntime.EntitySetEnumerationEvaluatorRegistry
	candidateVisibility  projecttypeenvruntime.CandidateVisibilityEvaluatorRegistry
	kindDefinedness      projecttypeenvruntime.KindDefinednessEvaluatorRegistry
	identity             typedmemoryevaluation.MechanismIdentity
}

func (builder ProjectEntityMembershipAdmissionEngineBuilder) SetCandidateVisibility(
	registry projecttypeenvruntime.CandidateVisibilityEvaluatorRegistry,
) ProjectEntityMembershipAdmissionEngineBuilder {
	builder.candidateVisibility = registry.Clone()
	return builder
}

func NewProjectEntityMembershipAdmissionEngineBuilder() ProjectEntityMembershipAdmissionEngineBuilder {
	return ProjectEntityMembershipAdmissionEngineBuilder{}
}

func (builder ProjectEntityMembershipAdmissionEngineBuilder) SetEntitySetEnumeration(
	registry projecttypeenvruntime.EntitySetEnumerationEvaluatorRegistry,
) ProjectEntityMembershipAdmissionEngineBuilder {
	builder.entitySetEnumeration = registry.Clone()
	return builder
}

func (builder ProjectEntityMembershipAdmissionEngineBuilder) SetKindDefinedness(
	registry projecttypeenvruntime.KindDefinednessEvaluatorRegistry,
) ProjectEntityMembershipAdmissionEngineBuilder {
	builder.kindDefinedness = registry.Clone()
	return builder
}

func (builder ProjectEntityMembershipAdmissionEngineBuilder) SetMechanismIdentity(
	identity typedmemoryevaluation.MechanismIdentity,
) ProjectEntityMembershipAdmissionEngineBuilder {
	builder.identity = identity
	return builder
}

func (builder ProjectEntityMembershipAdmissionEngineBuilder) Build() (
	ProjectEntityMembershipAdmissionEngine,
	error,
) {
	rule, err := typedmemory.NewRuleRef(projectEntityMemberOfRule)
	if err != nil {
		return ProjectEntityMembershipAdmissionEngine{}, err
	}
	engine := ProjectEntityMembershipAdmissionEngine{
		entitySetEnumeration: builder.entitySetEnumeration.Clone(),
		candidateVisibility:  builder.candidateVisibility.Clone(),
		kindDefinedness:      builder.kindDefinedness.Clone(),
		rule:                 rule,
		identity:             builder.identity,
	}
	if err := engine.validate(); err != nil {
		return ProjectEntityMembershipAdmissionEngine{}, fmt.Errorf(
			"build project-entity MemberOf admission engine: %w",
			err,
		)
	}
	return engine, nil
}

func (engine ProjectEntityMembershipAdmissionEngine) EvaluateMemberOf(
	ctx context.Context,
	input memberofevaluation.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	if err := projectEntityMembershipContextError(ctx); err != nil {
		return nil, err
	}
	if err := engine.validate(); err != nil {
		return nil, err
	}
	if !input.Valid() {
		return nil, ErrProjectEntityMembershipRuntimeInvalid
	}
	request := input.Request()
	query := request.Query()
	if query.ValueKind().ID().String() != "U.Entity" {
		return projectEntityMembershipUndefinedForSignature(request)
	}
	signature, found := input.Environment().KindSignatureDefinition(
		query.ValueKind(),
		query.ContextSlice().Context(),
	)
	if !found || signature.Evaluator() != engine.rule {
		return projectEntityMembershipUndefinedForSignature(request)
	}
	entitySet, found := input.Environment().EntitySetDefinition(
		signature.EntitySet(),
	)
	if !found {
		return projectEntityMembershipUndefinedForEntitySet(
			request,
			signature.EntitySet(),
		)
	}
	universe, exact := input.PersistedEntityUniverse().(memberofevaluation.ExactPersistedEntityUniverse)
	if !exact || !universe.Valid() {
		return projectEntityMembershipUndefinedForEntitySet(
			request,
			entitySet.Ref(),
		)
	}
	return engine.evaluateExactUniverse(
		request,
		signature,
		entitySet,
		universe,
	)
}

// SelectSnapshotObservableInputs selects only the transaction-correlated
// persisted-entity universe already supplied by the storage snapshot. The
// engine does not infer that universe from arbitrary observable bytes, and it
// rejects a catalog that does not contain the exact store-owned blob.
func (engine ProjectEntityMembershipAdmissionEngine) SelectSnapshotObservableInputs(
	input memberofevaluation.MemberOfEvaluationInput,
) memberofevaluation.SnapshotObservableInputSelection {
	if engine.validate() != nil || !input.Valid() {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	query := input.Request().Query()
	signature, found := input.Environment().KindSignatureDefinition(
		query.ValueKind(),
		query.ContextSlice().Context(),
	)
	if !found ||
		query.ValueKind().ID().String() != "U.Entity" ||
		signature.Evaluator() != engine.rule {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	universe, exact := input.PersistedEntityUniverse().(memberofevaluation.ExactPersistedEntityUniverse)
	if !exact || !universe.Valid() {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	blob, err := universe.ObservableBlob()
	if err != nil || !observableCatalogContainsExact(input.ObservableInputs(), blob) {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	selected, err := memberofevaluation.NewSnapshotObservableInputsSelected(
		[]memberofevaluation.ObservableInputBlob{blob},
	)
	if err != nil {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	return selected
}

func observableCatalogContainsExact(
	catalog []memberofevaluation.ObservableInputBlob,
	want memberofevaluation.ObservableInputBlob,
) bool {
	for _, blob := range catalog {
		if blob.Reference() == want.Reference() &&
			blob.Digest() == want.Digest() &&
			bytes.Equal(blob.Bytes(), want.Bytes()) {
			return true
		}
	}
	return false
}

func (engine ProjectEntityMembershipAdmissionEngine) evaluateExactUniverse(
	request typedmemory.MemberOfEvaluationRequest,
	signature typedmemory.KindSignatureDefinition,
	entitySet typedmemory.EntitySetDefinition,
	universe memberofevaluation.ExactPersistedEntityUniverse,
) (typedmemory.MemberOfJudgement, error) {
	observable, err := universe.ObservableInput()
	if err != nil {
		return nil, err
	}
	candidates, entities, evidence, err := engine.projectEntityCandidates(
		request,
		entitySet,
		universe.Members(),
	)
	if err != nil {
		return nil, err
	}
	observation, err := typedmemorykindruntime.NewExactEntitySetObservation(
		typedmemorykindruntime.ExactEntitySetObservationInput{
			Entities: entities,
			ObservableInputs: []typedmemory.MemberOfObservableInput{
				observable,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	enumerationRequest, err := typedmemorykindruntime.NewEntitySetEnumerationRequest(
		typedmemorykindruntime.EntitySetEnumerationRequestInput{
			ContextSlice: request.Query().ContextSlice(),
			View:         request.View(),
			Definition:   entitySet,
			Candidates:   candidates,
			Observation:  observation,
		},
	)
	if err != nil {
		return nil, err
	}
	enumerationRegistration, err := exactEvaluatorRegistration(
		engine.entitySetEnumeration,
		entitySet.EnumerationRule(),
		"project-entity EntitySet enumeration",
	)
	if err != nil {
		return nil, err
	}
	enumerationResult, err := enumerationRegistration.Evaluator().Evaluate(
		enumerationRequest,
	)
	if err != nil {
		return nil, err
	}
	enumerated, exact := enumerationResult.(typedmemorykindruntime.EntitySetEnumerated)
	if !exact {
		return projectEntityMembershipUndefinedForEntitySet(
			request,
			entitySet.Ref(),
		)
	}
	if !entityIDsContain(enumerated.Entities(), request.Query().EntityID()) {
		return projectEntityMembershipUndefinedForPersistedIdentity(request)
	}
	definednessObservation, err := typedmemorykindruntime.NewExactKindDefinednessObservation(
		enumerated.Basis().ObservableInputs(),
	)
	if err != nil {
		return nil, err
	}
	definednessRequest, err := typedmemorykindruntime.NewKindDefinednessRequest(
		typedmemorykindruntime.KindDefinednessRequestInput{
			MemberOfRequest: request,
			Signature:       signature,
			Enumeration:     enumerated,
			Observation:     definednessObservation,
		},
	)
	if err != nil {
		return nil, err
	}
	definednessRegistration, err := exactEvaluatorRegistration(
		engine.kindDefinedness,
		signature.DefinednessRule(),
		"project-entity Kind definedness",
	)
	if err != nil {
		return nil, err
	}
	definednessResult, err := definednessRegistration.Evaluator().Evaluate(
		definednessRequest,
	)
	if err != nil {
		return nil, err
	}
	switch defined := definednessResult.(type) {
	case typedmemorykindruntime.KindDefined:
		certificate, err := typedmemorykindruntime.NewC32PrerequisiteCertificateFromResults(
			typedmemorykindruntime.C32PrerequisiteCertificateFromResultsInput{
				EnumerationRequest: enumerationRequest,
				EnumerationResult:  enumerated,
				DefinednessRequest: definednessRequest,
				DefinednessResult:  defined,
				CandidateEvidence:  evidence,
			},
		)
		if err != nil {
			return nil, err
		}
		return engine.memberFromExactCertificate(
			request,
			request.View(),
			signature,
			entitySet,
			observable,
			certificate,
		)
	case typedmemorykindruntime.KindDefinednessUndefined:
		result, err := recordMembershipUndefinedForDefinedness(
			request,
			defined,
		)
		if err != nil {
			return nil, err
		}
		undefined, ok := result.(recordMembershipKindPrerequisitesUndefined)
		if !ok {
			return nil, ErrProjectEntityMembershipRuntimeInvalid
		}
		return undefined.judgement, nil
	default:
		return nil, ErrProjectEntityMembershipRuntimeInvalid
	}
}

func (engine ProjectEntityMembershipAdmissionEngine) memberFromExactCertificate(
	request typedmemory.MemberOfEvaluationRequest,
	view typedmemory.MemberOfEvaluationView,
	signature typedmemory.KindSignatureDefinition,
	entitySet typedmemory.EntitySetDefinition,
	observable typedmemory.MemberOfObservableInput,
	certificate typedmemory.C32PrerequisiteCertificate,
) (typedmemory.MemberOfJudgement, error) {
	provenance, err := projectEntityMembershipProvenance(engine.identity)
	if err != nil {
		return nil, err
	}
	basis, err := typedmemory.NewMemberOfBasisV3(
		typedmemory.MemberOfBasisV3Input{
			Basis: typedmemory.MemberOfBasisInput{
				Query:                request.Query(),
				EvaluationView:       view,
				KindSignature:        signature,
				EntitySet:            entitySet,
				ObservableInputs:     []typedmemory.MemberOfObservableInput{observable},
				EvaluationProvenance: provenance,
			},
			Prerequisites: certificate,
		},
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewMemberOfMember(request.Query(), basis)
}

func entityIDsContain(
	entities []typedmemory.EntityID,
	want typedmemory.EntityID,
) bool {
	for _, entity := range entities {
		if entity == want {
			return true
		}
	}
	return false
}

func (engine ProjectEntityMembershipAdmissionEngine) projectEntityCandidates(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinition,
	persisted []typedmemory.EntityID,
) (
	typedmemorykindruntime.EntitySetCandidateBasis,
	[]typedmemory.EntityID,
	typedmemorykindruntime.C32CandidateEvaluationEvidence,
	error,
) {
	switch view := request.View().(type) {
	case typedmemory.PersistedSnapshotView:
		basis, err := typedmemorykindruntime.NewPersistedEntitySetCandidateBasis(view)
		return basis,
			append([]typedmemory.EntityID(nil), persisted...),
			typedmemorykindruntime.NewPersistedC32CandidateEvidence(),
			err
	case typedmemory.ProspectiveBatchView:
		policy, ok := entitySet.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
		if !ok {
			return nil, nil, nil, ErrProjectEntityMembershipRuntimeInvalid
		}
		visibilityRequest, err := typedmemorykindruntime.NewCandidateVisibilityRequest(
			typedmemorykindruntime.CandidateVisibilityRequestInput{
				Definition: entitySet,
				View:       view,
			},
		)
		if err != nil {
			return nil, nil, nil, err
		}
		registration, err := exactEvaluatorRegistration(
			engine.candidateVisibility,
			policy.EvaluationRule(),
			"project-entity candidate visibility",
		)
		if err != nil {
			return nil, nil, nil, err
		}
		visibilityResult, err := registration.Evaluator().Evaluate(visibilityRequest)
		if err != nil {
			return nil, nil, nil, err
		}
		visible, ok := visibilityResult.(typedmemorykindruntime.CandidateVisible)
		if !ok {
			return nil, nil, nil, ErrProjectEntityMembershipRuntimeInvalid
		}
		basis, err := typedmemorykindruntime.NewProspectiveEntitySetCandidateBasis(visible)
		if err != nil {
			return nil, nil, nil, err
		}
		evidence, err := typedmemorykindruntime.NewProspectiveC32CandidateEvidence(
			visibilityRequest,
			visible,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		entities := visibleEntitySetCandidates(
			persisted,
			view,
			entitySet.Ref().Context(),
		)
		return basis, entities, evidence, nil
	default:
		return nil, nil, nil, ErrProjectEntityMembershipRuntimeInvalid
	}
}

func (engine ProjectEntityMembershipAdmissionEngine) validate() error {
	if engine.entitySetEnumeration.Len() == 0 ||
		engine.candidateVisibility.Len() == 0 ||
		engine.kindDefinedness.Len() == 0 ||
		engine.rule.String() != projectEntityMemberOfRule ||
		engine.identity.Role() != typedmemoryevaluation.EvaluatorRole ||
		engine.identity.ArtifactRef().String() == "" ||
		engine.identity.Edition().String() == "" ||
		engine.identity.Digest().String() == "" {
		return ErrProjectEntityMembershipRuntimeMissing
	}
	return nil
}

func NewProjectEntityMembershipEvaluatorRegistry(
	engine ProjectEntityMembershipAdmissionEngine,
) (memberofruntime.Registry, error) {
	if err := engine.validate(); err != nil {
		return memberofruntime.Registry{}, err
	}
	registration, err := memberofruntime.NewRegistration(
		engine.rule,
		engine.identity,
		engine,
	)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	return memberofruntime.NewRegistry([]memberofruntime.Registration{registration})
}

func projectEntityMembershipProvenance(
	identity typedmemoryevaluation.MechanismIdentity,
) (typedmemory.MemberOfEvaluationProvenance, error) {
	reference, err := typedmemory.NewProvenanceRef(
		"prov:haft-project-entity-membership-evaluation:" +
			identity.Digest().String(),
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

func projectEntityMembershipUndefinedForSignature(
	request typedmemory.MemberOfEvaluationRequest,
) (typedmemory.MemberOfJudgement, error) {
	query := request.Query()
	missing, err := typedmemory.MissingKindSignatureForMemberOf(query)
	if err != nil {
		return nil, err
	}
	return projectEntityMembershipUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/project-entity-kind-signature",
	)
}

func projectEntityMembershipUndefinedForEntitySet(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinitionRef,
) (typedmemory.MemberOfJudgement, error) {
	missing, err := typedmemory.MissingEntitySetForMemberOf(entitySet)
	if err != nil {
		return nil, err
	}
	return projectEntityMembershipUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/persisted-entity-universe",
	)
}

func projectEntityMembershipUndefinedForPersistedIdentity(
	request typedmemory.MemberOfEvaluationRequest,
) (typedmemory.MemberOfJudgement, error) {
	query := request.Query()
	missing, err := typedmemory.MissingObservableSourceForMemberOf(query)
	if err != nil {
		return nil, err
	}
	return projectEntityMembershipUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:project-memory/resolve-persisted-project-entity",
	)
}

func projectEntityMembershipUndefined(
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

func projectEntityMembershipContextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("project-entity membership evaluation context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("project-entity membership evaluation context: %w", err)
	}
	return nil
}
