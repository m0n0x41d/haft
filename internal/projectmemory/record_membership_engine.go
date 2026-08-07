package projectmemory

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorykindruntime"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrRecordMembershipRuntimeMissing = errors.New(
		"project-memory record membership runtime is missing",
	)
	ErrRecordMembershipRuntimeInvalid = errors.New(
		"project-memory record membership runtime is invalid",
	)
	ErrRecordMembershipBasisMissing = errors.New(
		"project-memory record membership basis is missing",
	)
)

// RecordMembershipAdmissionEngine is the store-facing executable boundary for
// transaction-time MemberOf revalidation of project-record carrier sources.
// It is constructed from one exact runtime registry already matched to the
// selected project TypeEnv. It does not select a head, trust caller bytes, or
// open public admission.
type RecordMembershipAdmissionEngine struct {
	entitySetEnumeration projecttypeenvruntime.EntitySetEnumerationEvaluatorRegistry
	candidateVisibility  projecttypeenvruntime.CandidateVisibilityEvaluatorRegistry
	kindDefinedness      projecttypeenvruntime.KindDefinednessEvaluatorRegistry
	recordMembership     typedmemoryevaluation.Registry[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	]
	policy recordmembershipregistration.RegistrationArtifactV1
}

var _ typedmemorystore.MemberOfEvaluationEngine = RecordMembershipAdmissionEngine{}

func NewRecordMembershipAdmissionEngine(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (RecordMembershipAdmissionEngine, error) {
	if !runtime.Valid() {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeMissing
	}
	memberOf, ok := runtime.MemberOfRegistry()
	if !ok || memberOf.Len() == 0 {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeMissing
	}
	policy, ok := exactRecordMembershipPolicy(runtime)
	if !ok {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeMissing
	}
	identity, err := recordMembershipMechanismIdentity(policy.Evaluator())
	if err != nil {
		return RecordMembershipAdmissionEngine{}, err
	}
	lookup, err := memberOf.Lookup(policy.Evaluator().Rule(), identity)
	if err != nil {
		return RecordMembershipAdmissionEngine{}, err
	}
	found, ok := lookup.(memberofruntime.Found)
	if !ok {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeMissing
	}
	installed, ok := found.Registration().Engine().(RecordMembershipAdmissionEngine)
	if !ok || installed.policy.Ref() != policy.Ref() {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeInvalid
	}
	registration, err := installed.evaluatorRegistration(policy.Evaluator())
	if err != nil {
		return RecordMembershipAdmissionEngine{}, err
	}
	recordMembership, err := typedmemoryevaluation.NewRegistry(
		[]typedmemoryevaluation.Registration[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{registration},
	)
	if err != nil {
		return RecordMembershipAdmissionEngine{}, err
	}
	entitySetEnumeration, ok := runtime.EntitySetEnumerationRegistry()
	if !ok || entitySetEnumeration.Len() == 0 {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeMissing
	}
	kindDefinedness, ok := runtime.KindDefinednessRegistry()
	if !ok || kindDefinedness.Len() == 0 {
		return RecordMembershipAdmissionEngine{}, ErrRecordMembershipRuntimeMissing
	}
	candidateVisibility, _ := runtime.CandidateVisibilityRegistry()
	engine := RecordMembershipAdmissionEngine{
		entitySetEnumeration: entitySetEnumeration,
		candidateVisibility:  candidateVisibility,
		kindDefinedness:      kindDefinedness,
		recordMembership:     recordMembership,
		policy:               policy,
	}
	if err := engine.validate(); err != nil {
		return RecordMembershipAdmissionEngine{}, err
	}
	return engine, nil
}

func (engine RecordMembershipAdmissionEngine) EvaluateMemberOf(
	ctx context.Context,
	input typedmemorystore.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	if err := recordMembershipContextError(ctx); err != nil {
		return nil, err
	}
	if err := engine.validate(); err != nil {
		return nil, err
	}
	request := input.Request()
	query := request.Query()
	environment := input.Environment()
	contextRef := query.ContextSlice().Context()
	signature, found := environment.KindSignatureDefinition(
		query.ValueKind(),
		contextRef,
	)
	if !found {
		return nil, fmt.Errorf(
			"%w: KindSignature for %s in %s",
			ErrRecordMembershipBasisMissing,
			query.ValueKind().String(),
			contextRef.String(),
		)
	}
	entitySet, found := environment.EntitySetDefinition(signature.EntitySet())
	if !found {
		return nil, fmt.Errorf(
			"%w: EntitySet %s",
			ErrRecordMembershipBasisMissing,
			signature.EntitySet().String(),
		)
	}
	prerequisites, err := engine.evaluateKindPrerequisites(
		request,
		signature,
		entitySet,
		input.PersistedEntityUniverse(),
	)
	if err != nil {
		return nil, err
	}
	var satisfied recordMembershipKindPrerequisitesSatisfied
	switch result := prerequisites.(type) {
	case recordMembershipKindPrerequisitesSatisfied:
		satisfied = result
	case recordMembershipKindPrerequisitesUndefined:
		return result.judgement, nil
	default:
		return nil, ErrRecordMembershipRuntimeInvalid
	}
	delivery, manifest, adapter, err := engine.trustedDelivery(
		withoutPrerequisiteObservableBlobs(
			input.ObservableInputs(),
			satisfied.observableInputs,
		),
	)
	if err != nil {
		return nil, err
	}
	provenance, err := recordMembershipEvaluationProvenance(
		engine.policy.Evaluator(),
	)
	if err != nil {
		return nil, err
	}
	evaluation, err := recordcarrier.NewRecordMembershipEvaluationRequestV3(
		recordcarrier.RecordMembershipEvaluationInputV3{
			ProjectID:                    input.ProjectID(),
			Query:                        query,
			EvaluationView:               request.View(),
			KindSignature:                signature,
			EntitySet:                    entitySet,
			EvaluationProvenance:         provenance,
			ExpectedMappingManifest:      manifest,
			ExpectedAdapterVersion:       adapter,
			SourceDelivery:               delivery,
			Prerequisites:                satisfied.certificate,
			PrerequisiteObservableInputs: satisfied.observableInputs,
		},
	)
	if err != nil {
		return nil, err
	}
	registration, err := engine.evaluatorRegistration(engine.policy.Evaluator())
	if err != nil {
		return nil, err
	}
	return registration.Evaluator().Evaluate(evaluation.RegisteredRequest())
}

func withoutPrerequisiteObservableBlobs(
	blobs []typedmemorystore.ObservableInputBlob,
	prerequisites []typedmemory.MemberOfObservableInput,
) []typedmemorystore.ObservableInputBlob {
	excluded := make(map[typedmemory.MemberOfObservableInput]struct{}, len(prerequisites))
	for _, input := range prerequisites {
		excluded[input] = struct{}{}
	}
	result := make([]typedmemorystore.ObservableInputBlob, 0, len(blobs))
	for _, blob := range blobs {
		input, err := typedmemory.NewMemberOfObservableInput(
			blob.Reference(),
			blob.Digest(),
		)
		if err != nil {
			continue
		}
		if _, isPrerequisite := excluded[input]; isPrerequisite {
			continue
		}
		result = append(result, blob)
	}
	return result
}

func (engine RecordMembershipAdmissionEngine) validate() error {
	if engine.recordMembership.Len() == 0 ||
		engine.entitySetEnumeration.Len() == 0 ||
		engine.kindDefinedness.Len() == 0 {
		return ErrRecordMembershipRuntimeMissing
	}
	if err := engine.policy.Verify(); err != nil {
		return fmt.Errorf("%w: %v", ErrRecordMembershipRuntimeInvalid, err)
	}
	if len(engine.policy.AcceptedMappings()) == 0 {
		return ErrRecordMembershipRuntimeInvalid
	}
	if _, err := engine.evaluatorRegistration(engine.policy.Evaluator()); err != nil {
		return fmt.Errorf("%w: %v", ErrRecordMembershipRuntimeInvalid, err)
	}
	return nil
}

type recordMembershipKindPrerequisiteResult interface {
	recordMembershipKindPrerequisiteVariant()
}

type recordMembershipKindPrerequisitesSatisfied struct {
	enumeration      typedmemorykindruntime.EntitySetEnumerated
	definedness      typedmemorykindruntime.KindDefined
	certificate      typedmemory.C32PrerequisiteCertificate
	observableInputs []typedmemory.MemberOfObservableInput
}

func (recordMembershipKindPrerequisitesSatisfied) recordMembershipKindPrerequisiteVariant() {}

type recordMembershipKindPrerequisitesUndefined struct {
	judgement typedmemory.MemberOfUndefined
}

func (recordMembershipKindPrerequisitesUndefined) recordMembershipKindPrerequisiteVariant() {}

func (engine RecordMembershipAdmissionEngine) evaluateKindPrerequisites(
	request typedmemory.MemberOfEvaluationRequest,
	signature typedmemory.KindSignatureDefinition,
	entitySet typedmemory.EntitySetDefinition,
	universe typedmemorystore.PersistedEntityUniverse,
) (recordMembershipKindPrerequisiteResult, error) {
	exactUniverse, ok := universe.(typedmemorystore.ExactPersistedEntityUniverse)
	if !ok {
		return recordMembershipUndefinedForEntitySet(
			request,
			entitySet.Ref(),
			"repair:project-memory/entity-set-universe-unavailable",
		)
	}
	observable, err := exactUniverse.ObservableInput()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRecordMembershipBasisMissing, err)
	}
	return engine.evaluateExactKindPrerequisites(
		request,
		signature,
		entitySet,
		exactUniverse.Members(),
		observable,
	)
}

func (engine RecordMembershipAdmissionEngine) evaluateExactKindPrerequisites(
	request typedmemory.MemberOfEvaluationRequest,
	signature typedmemory.KindSignatureDefinition,
	entitySet typedmemory.EntitySetDefinition,
	persistedEntities []typedmemory.EntityID,
	persistedUniverseInput typedmemory.MemberOfObservableInput,
) (recordMembershipKindPrerequisiteResult, error) {
	projection, undefined, err := engine.projectEntitySetCandidates(
		request,
		entitySet,
		persistedEntities,
	)
	if err != nil {
		return nil, err
	}
	if undefined != nil {
		return recordMembershipKindPrerequisitesUndefined{
			judgement: *undefined,
		}, nil
	}
	observation, err := typedmemorykindruntime.NewExactEntitySetObservation(
		typedmemorykindruntime.ExactEntitySetObservationInput{
			Entities: projection.entities,
			ObservableInputs: []typedmemory.MemberOfObservableInput{
				persistedUniverseInput,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct exact EntitySet observation: %w", err)
	}
	enumerationRequest, err := typedmemorykindruntime.NewEntitySetEnumerationRequest(
		typedmemorykindruntime.EntitySetEnumerationRequestInput{
			ContextSlice: request.Query().ContextSlice(),
			View:         request.View(),
			Definition:   entitySet,
			Candidates:   projection.basis,
			Observation:  observation,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct EntitySet enumeration request: %w", err)
	}
	enumerationRegistration, err := exactEvaluatorRegistration(
		engine.entitySetEnumeration,
		entitySet.EnumerationRule(),
		"EntitySet enumeration",
	)
	if err != nil {
		return nil, err
	}
	enumerationResult, err := enumerationRegistration.Evaluator().Evaluate(
		enumerationRequest,
	)
	if err != nil {
		return nil, fmt.Errorf("evaluate exact EntitySet enumeration: %w", err)
	}
	definednessObservation, err := kindDefinednessObservation(enumerationResult)
	if err != nil {
		return nil, err
	}
	definednessRequest, err := typedmemorykindruntime.NewKindDefinednessRequest(
		typedmemorykindruntime.KindDefinednessRequestInput{
			MemberOfRequest: request,
			Signature:       signature,
			Enumeration:     enumerationResult,
			Observation:     definednessObservation,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct Kind definedness request: %w", err)
	}
	definednessRegistration, err := exactEvaluatorRegistration(
		engine.kindDefinedness,
		signature.DefinednessRule(),
		"Kind definedness",
	)
	if err != nil {
		return nil, err
	}
	definednessResult, err := definednessRegistration.Evaluator().Evaluate(
		definednessRequest,
	)
	if err != nil {
		return nil, fmt.Errorf("evaluate exact Kind definedness: %w", err)
	}
	switch result := definednessResult.(type) {
	case typedmemorykindruntime.KindDefined:
		enumerated, exact := enumerationResult.(typedmemorykindruntime.EntitySetEnumerated)
		if !exact {
			return nil, ErrRecordMembershipRuntimeInvalid
		}
		certificate, err := typedmemorykindruntime.NewC32PrerequisiteCertificateFromResults(
			typedmemorykindruntime.C32PrerequisiteCertificateFromResultsInput{
				EnumerationRequest: enumerationRequest,
				EnumerationResult:  enumerated,
				DefinednessRequest: definednessRequest,
				DefinednessResult:  result,
				CandidateEvidence:  projection.candidateEvidence,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"seal exact C.3.2 prerequisite certificate: %w",
				err,
			)
		}
		return recordMembershipKindPrerequisitesSatisfied{
			enumeration:      enumerated,
			definedness:      result,
			certificate:      certificate,
			observableInputs: enumerated.Basis().ObservableInputs(),
		}, nil
	case typedmemorykindruntime.KindDefinednessUndefined:
		return recordMembershipUndefinedForDefinedness(request, result)
	default:
		return nil, ErrRecordMembershipRuntimeInvalid
	}
}

type recordMembershipEntitySetProjection struct {
	basis             typedmemorykindruntime.EntitySetCandidateBasis
	entities          []typedmemory.EntityID
	candidateEvidence typedmemorykindruntime.C32CandidateEvaluationEvidence
}

func (engine RecordMembershipAdmissionEngine) projectEntitySetCandidates(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinition,
	persisted []typedmemory.EntityID,
) (recordMembershipEntitySetProjection, *typedmemory.MemberOfUndefined, error) {
	switch view := request.View().(type) {
	case typedmemory.PersistedSnapshotView:
		basis, err := typedmemorykindruntime.NewPersistedEntitySetCandidateBasis(view)
		if err != nil {
			return recordMembershipEntitySetProjection{}, nil, err
		}
		return recordMembershipEntitySetProjection{
			basis:             basis,
			entities:          append([]typedmemory.EntityID(nil), persisted...),
			candidateEvidence: typedmemorykindruntime.NewPersistedC32CandidateEvidence(),
		}, nil, nil
	case typedmemory.ProspectiveBatchView:
		policy, visible := entitySet.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
		if !visible {
			missing, err := typedmemory.MissingCandidateVisibilityForMemberOf(
				entitySet.Ref(),
			)
			if err != nil {
				return recordMembershipEntitySetProjection{}, nil, err
			}
			undefined, err := newRecordMembershipUndefined(
				request,
				[]typedmemory.MemberOfMissingBasis{missing},
				"repair:project-memory/prior-declaration-visibility-not-selected",
			)
			return recordMembershipEntitySetProjection{}, undefined, err
		}
		visibilityRequest, err := typedmemorykindruntime.NewCandidateVisibilityRequest(
			typedmemorykindruntime.CandidateVisibilityRequestInput{
				Definition: entitySet,
				View:       view,
			},
		)
		if err != nil {
			return recordMembershipEntitySetProjection{}, nil, err
		}
		visibilityRegistration, err := exactEvaluatorRegistration(
			engine.candidateVisibility,
			policy.EvaluationRule(),
			"candidate visibility",
		)
		if err != nil {
			return recordMembershipEntitySetProjection{}, nil, err
		}
		visibilityResult, err := visibilityRegistration.Evaluator().Evaluate(
			visibilityRequest,
		)
		if err != nil {
			return recordMembershipEntitySetProjection{}, nil, err
		}
		candidateVisible, ok := visibilityResult.(typedmemorykindruntime.CandidateVisible)
		if !ok {
			return recordMembershipEntitySetProjection{}, nil,
				ErrRecordMembershipRuntimeInvalid
		}
		basis, err := typedmemorykindruntime.NewProspectiveEntitySetCandidateBasis(
			candidateVisible,
		)
		if err != nil {
			return recordMembershipEntitySetProjection{}, nil, err
		}
		candidateEvidence, err := typedmemorykindruntime.NewProspectiveC32CandidateEvidence(
			visibilityRequest,
			candidateVisible,
		)
		if err != nil {
			return recordMembershipEntitySetProjection{}, nil, err
		}
		return recordMembershipEntitySetProjection{
			basis:             basis,
			entities:          visibleEntitySetCandidates(persisted, view, entitySet.Ref().Context()),
			candidateEvidence: candidateEvidence,
		}, nil, nil
	default:
		return recordMembershipEntitySetProjection{}, nil,
			ErrRecordMembershipRuntimeInvalid
	}
}

func visibleEntitySetCandidates(
	persisted []typedmemory.EntityID,
	view typedmemory.ProspectiveBatchView,
	contextRef typedmemory.BoundedContextRef,
) []typedmemory.EntityID {
	result := append([]typedmemory.EntityID(nil), persisted...)
	seen := make(map[typedmemory.EntityID]struct{}, len(result))
	for _, entity := range result {
		seen[entity] = struct{}{}
	}
	for _, change := range view.OrderedCandidatePrefix().Changes() {
		declaration, ok := change.(typedmemory.DeclareEntity)
		if !ok || declaration.Context() != contextRef {
			continue
		}
		if _, exists := seen[declaration.Entity()]; exists {
			continue
		}
		seen[declaration.Entity()] = struct{}{}
		result = append(result, declaration.Entity())
	}
	return result
}

func kindDefinednessObservation(
	result typedmemorykindruntime.EntitySetEnumerationResult,
) (typedmemorykindruntime.ExactKindDefinednessObservation, error) {
	enumerated, ok := result.(typedmemorykindruntime.EntitySetEnumerated)
	if !ok {
		return typedmemorykindruntime.ExactKindDefinednessObservation{}, fmt.Errorf(
			"%w: EntitySet enumeration is %s",
			ErrRecordMembershipBasisMissing,
			result.Kind().String(),
		)
	}
	return typedmemorykindruntime.NewExactKindDefinednessObservation(
		enumerated.Basis().ObservableInputs(),
	)
}

func recordMembershipUndefinedForDefinedness(
	request typedmemory.MemberOfEvaluationRequest,
	result typedmemorykindruntime.KindDefinednessUndefined,
) (recordMembershipKindPrerequisiteResult, error) {
	query := request.Query()
	missing, err := missingBasisForDefinedness(query, result.Failure())
	if err != nil {
		return nil, err
	}
	undefined, err := newRecordMembershipUndefined(
		request,
		missing,
		"repair:project-memory/kind-definedness-undefined/"+
			result.Failure().Kind().String(),
	)
	if err != nil {
		return nil, err
	}
	return recordMembershipKindPrerequisitesUndefined{
		judgement: *undefined,
	}, nil
}

func recordMembershipUndefinedForEntitySet(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinitionRef,
	repair string,
) (recordMembershipKindPrerequisiteResult, error) {
	missing, err := typedmemory.MissingEntitySetForMemberOf(entitySet)
	if err != nil {
		return nil, err
	}
	undefined, err := newRecordMembershipUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		return nil, err
	}
	return recordMembershipKindPrerequisitesUndefined{
		judgement: *undefined,
	}, nil
}

func missingBasisForDefinedness(
	query typedmemory.MemberOfQuery,
	failure typedmemorykindruntime.KindDefinednessFailure,
) ([]typedmemory.MemberOfMissingBasis, error) {
	switch value := failure.(type) {
	case typedmemorykindruntime.EntitySetUnavailableForDefinedness:
		missing, err := typedmemory.MissingEntitySetForMemberOf(
			value.Enumeration().DefinitionRef(),
		)
		return []typedmemory.MemberOfMissingBasis{missing}, err
	case typedmemorykindruntime.ObservationUnavailableForDefinedness:
		missing := make([]typedmemory.MemberOfMissingBasis, 0)
		for _, input := range value.ObservableInputs() {
			basis, err := typedmemory.MissingObservableInputForMemberOf(input)
			if err != nil {
				return nil, err
			}
			missing = append(missing, basis)
		}
		return missing, nil
	case typedmemorykindruntime.EntityOutsideSetForDefinedness,
		typedmemorykindruntime.AssumptionsUnavailableForDefinedness:
		missing, err := typedmemory.MissingKindSignatureForMemberOf(query)
		return []typedmemory.MemberOfMissingBasis{missing}, err
	default:
		return nil, ErrRecordMembershipRuntimeInvalid
	}
}

func newRecordMembershipUndefined(
	request typedmemory.MemberOfEvaluationRequest,
	missing []typedmemory.MemberOfMissingBasis,
	repairRaw string,
) (*typedmemory.MemberOfUndefined, error) {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return nil, err
	}
	undefined, err := typedmemory.NewMemberOfUndefined(request, missing, repair)
	if err != nil {
		return nil, err
	}
	return &undefined, nil
}

func exactEvaluatorRegistration[Input, Output any](
	registry typedmemoryevaluation.Registry[Input, Output],
	rule typedmemory.RuleRef,
	contract string,
) (typedmemoryevaluation.Registration[Input, Output], error) {
	for _, registration := range registry.Registrations() {
		if registration.RuleRef() != rule {
			continue
		}
		lookup, err := registry.Lookup(rule, registration.Identity())
		if err != nil {
			return typedmemoryevaluation.Registration[Input, Output]{}, err
		}
		found, ok := lookup.(typedmemoryevaluation.Found[Input, Output])
		if !ok {
			return typedmemoryevaluation.Registration[Input, Output]{}, fmt.Errorf(
				"%w: selected-X %s lookup returned %s",
				ErrRecordMembershipRuntimeInvalid,
				contract,
				lookup.Kind().String(),
			)
		}
		return found.Registration(), nil
	}
	return typedmemoryevaluation.Registration[Input, Output]{}, fmt.Errorf(
		"%w: selected-X %s evaluator for %s is absent",
		ErrRecordMembershipRuntimeMissing,
		contract,
		rule.String(),
	)
}

func (engine RecordMembershipAdmissionEngine) trustedDelivery(
	blobs []typedmemorystore.ObservableInputBlob,
) (
	recordcarrier.RecordMembershipSourceDeliveryV1,
	recordmapping.MappingManifestRef,
	recordmapping.AdapterVersion,
	error,
) {
	if len(blobs) != 1 {
		return nil, recordmapping.MappingManifestRef{}, recordmapping.AdapterVersion{}, fmt.Errorf(
			"%w: expected exactly one record-membership observable input, got %d",
			ErrRecordMembershipBasisMissing,
			len(blobs),
		)
	}
	blob := blobs[0]
	expected, err := typedmemory.NewMemberOfObservableInput(
		blob.Reference(),
		blob.Digest(),
	)
	if err != nil {
		return nil, recordmapping.MappingManifestRef{}, recordmapping.AdapterVersion{}, err
	}
	source, err := recordcarrier.VerifyRecordMembershipSourceV1(
		expected,
		blob.Bytes(),
	)
	if err != nil {
		return nil, recordmapping.MappingManifestRef{}, recordmapping.AdapterVersion{}, err
	}
	delivery, err := recordcarrier.NewTrustedRecordMembershipSourceDeliveryV1(
		engine.policy,
		expected,
		blob.Bytes(),
	)
	if err != nil {
		return nil, recordmapping.MappingManifestRef{}, recordmapping.AdapterVersion{}, err
	}
	return delivery,
		source.Binding().MappingManifestRef(),
		source.Binding().AdapterVersion(),
		nil
}

func (engine RecordMembershipAdmissionEngine) evaluatorRegistration(
	coordinate recordmembershipregistration.MechanismCoordinate,
) (
	typedmemoryevaluation.Registration[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	],
	error,
) {
	identity, err := recordMembershipMechanismIdentity(coordinate)
	if err != nil {
		return typedmemoryevaluation.Registration[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{}, err
	}
	lookup, err := engine.recordMembership.Lookup(coordinate.Rule(), identity)
	if err != nil {
		return typedmemoryevaluation.Registration[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{}, err
	}
	found, ok := lookup.(typedmemoryevaluation.Found[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	])
	if !ok {
		return typedmemoryevaluation.Registration[
				recordcarrier.RecordMembershipEvaluationRequestV1,
				typedmemory.MemberOfJudgement,
			]{}, fmt.Errorf(
				"%w: evaluator lookup returned %s",
				ErrRecordMembershipRuntimeInvalid,
				lookup.Kind().String(),
			)
	}
	return found.Registration(), nil
}

func recordMembershipMechanismIdentity(
	coordinate recordmembershipregistration.MechanismCoordinate,
) (typedmemoryevaluation.MechanismIdentity, error) {
	return typedmemoryevaluation.NewMechanismIdentity(
		coordinate.Artifact(),
		coordinate.Edition(),
		coordinate.Digest(),
		typedmemoryevaluation.EvaluatorRole,
	)
}

func exactRecordMembershipPolicy(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (recordmembershipregistration.RegistrationArtifactV1, bool) {
	policies, ok := runtime.RegistrationPolicies()
	if !ok {
		return recordmembershipregistration.RegistrationArtifactV1{}, false
	}
	registry, ok := policies.(projecttypeenvruntime.ExactTargetRegistrationPolicyRegistry)
	if !ok {
		return recordmembershipregistration.RegistrationArtifactV1{}, false
	}
	rule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	exact, ok := registry.Lookup(rule)
	if !ok {
		return recordmembershipregistration.RegistrationArtifactV1{}, false
	}
	return exact.Artifact()
}

func recordMembershipEvaluationProvenance(
	coordinate recordmembershipregistration.MechanismCoordinate,
) (typedmemory.MemberOfEvaluationProvenance, error) {
	reference, err := typedmemory.NewProvenanceRef(
		"prov:haft-record-membership-evaluation:" +
			coordinate.Digest().String(),
	)
	if err != nil {
		return typedmemory.MemberOfEvaluationProvenance{}, err
	}
	return typedmemory.NewMemberOfEvaluationProvenance(
		typedmemory.MemberOfEvaluationProvenanceInput{
			Reference:         reference,
			EvaluatorArtifact: coordinate.Artifact(),
			EvaluatorEdition:  coordinate.Edition(),
			EvaluatorDigest:   coordinate.Digest(),
		},
	)
}

func recordMembershipContextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("record membership evaluation context is nil")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("record membership evaluation context: %w", err)
	}
	return nil
}
