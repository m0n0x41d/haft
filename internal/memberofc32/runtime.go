// Package memberofc32 evaluates the prerequisite chain required before one
// selected-TypeEnv MemberOf rule may produce a defined judgement.
package memberofc32

import (
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorykindruntime"
)

var (
	ErrRuntimeMissing = errors.New("C.3.2 prerequisite runtime is missing")
	ErrRuntimeInvalid = errors.New("C.3.2 prerequisite runtime is invalid")
)

// Runtime is the exact callable prerequisite capability. It does not own a
// MemberOf evaluator, source-delivery boundary, store, or project head.
type Runtime struct {
	enumeration typedmemoryevaluation.EntitySetEnumerationRegistry
	visibility  typedmemoryevaluation.CandidateVisibilityRegistry
	definedness typedmemoryevaluation.KindDefinednessRegistry
}

func NewRuntime(
	enumeration typedmemoryevaluation.EntitySetEnumerationRegistry,
	visibility typedmemoryevaluation.CandidateVisibilityRegistry,
	definedness typedmemoryevaluation.KindDefinednessRegistry,
) (Runtime, error) {
	runtime := Runtime{
		enumeration: enumeration.Clone(),
		visibility:  visibility.Clone(),
		definedness: definedness.Clone(),
	}
	if err := runtime.validate(); err != nil {
		return Runtime{}, err
	}
	return runtime, nil
}

func (runtime Runtime) validate() error {
	if runtime.enumeration.Len() == 0 || runtime.definedness.Len() == 0 {
		return ErrRuntimeMissing
	}
	return nil
}

type Input struct {
	Request   typedmemory.MemberOfEvaluationRequest
	Signature typedmemory.KindSignatureDefinition
	EntitySet typedmemory.EntitySetDefinition
	Universe  memberofevaluation.PersistedEntityUniverse
}

type Result interface {
	resultVariant()
}

type Satisfied struct {
	certificate typedmemory.C32PrerequisiteCertificate
	inputs      []typedmemory.MemberOfObservableInput
}

func (result Satisfied) Certificate() typedmemory.C32PrerequisiteCertificate {
	return result.certificate
}

func (result Satisfied) ObservableInputs() []typedmemory.MemberOfObservableInput {
	return append([]typedmemory.MemberOfObservableInput(nil), result.inputs...)
}

func (Satisfied) resultVariant() {}

type Undefined struct {
	judgement typedmemory.MemberOfUndefined
}

func (result Undefined) Judgement() typedmemory.MemberOfUndefined {
	return result.judgement
}

func (Undefined) resultVariant() {}

func (runtime Runtime) Evaluate(input Input) (Result, error) {
	if err := runtime.validate(); err != nil {
		return nil, err
	}
	if err := validateInput(input); err != nil {
		return nil, err
	}
	universe, exact := input.Universe.(memberofevaluation.ExactPersistedEntityUniverse)
	if !exact || !universe.Valid() {
		return undefinedForEntitySet(
			input.Request,
			input.EntitySet.Ref(),
			"repair:member-of-c32/persisted-entity-universe",
		)
	}
	observable, err := universe.ObservableInput()
	if err != nil {
		return nil, fmt.Errorf("C.3.2 persisted universe observable: %w", err)
	}
	projection, undefined, err := runtime.projectCandidates(
		input.Request,
		input.EntitySet,
		universe.Members(),
	)
	if err != nil {
		return nil, err
	}
	if undefined != nil {
		return *undefined, nil
	}
	observation, err := typedmemorykindruntime.NewExactEntitySetObservation(
		typedmemorykindruntime.ExactEntitySetObservationInput{
			Entities: projection.entities,
			ObservableInputs: []typedmemory.MemberOfObservableInput{
				observable,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct C.3.2 EntitySet observation: %w", err)
	}
	enumerationRequest, err := typedmemorykindruntime.NewEntitySetEnumerationRequest(
		typedmemorykindruntime.EntitySetEnumerationRequestInput{
			ContextSlice: input.Request.Query().ContextSlice(),
			View:         input.Request.View(),
			Definition:   input.EntitySet,
			Candidates:   projection.basis,
			Observation:  observation,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct C.3.2 EntitySet request: %w", err)
	}
	enumeration, err := exactRegistration(
		runtime.enumeration,
		input.EntitySet.EnumerationRule(),
		"EntitySet enumeration",
	)
	if err != nil {
		return nil, err
	}
	enumerationResult, err := enumeration.Evaluator().Evaluate(enumerationRequest)
	if err != nil {
		return nil, fmt.Errorf("evaluate C.3.2 EntitySet: %w", err)
	}
	enumerated, ok := enumerationResult.(typedmemorykindruntime.EntitySetEnumerated)
	if !ok {
		return undefinedForEnumeration(input.Request, enumerationResult)
	}
	definednessObservation, err := typedmemorykindruntime.NewExactKindDefinednessObservation(
		enumerated.Basis().ObservableInputs(),
	)
	if err != nil {
		return nil, fmt.Errorf("construct C.3.2 definedness observation: %w", err)
	}
	definednessRequest, err := typedmemorykindruntime.NewKindDefinednessRequest(
		typedmemorykindruntime.KindDefinednessRequestInput{
			MemberOfRequest: input.Request,
			Signature:       input.Signature,
			Enumeration:     enumerated,
			Observation:     definednessObservation,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("construct C.3.2 definedness request: %w", err)
	}
	definedness, err := exactRegistration(
		runtime.definedness,
		input.Signature.DefinednessRule(),
		"Kind definedness",
	)
	if err != nil {
		return nil, err
	}
	definednessResult, err := definedness.Evaluator().Evaluate(definednessRequest)
	if err != nil {
		return nil, fmt.Errorf("evaluate C.3.2 definedness: %w", err)
	}
	defined, ok := definednessResult.(typedmemorykindruntime.KindDefined)
	if !ok {
		return undefinedForDefinedness(input.Request, definednessResult)
	}
	certificate, err := typedmemorykindruntime.NewC32PrerequisiteCertificateFromResults(
		typedmemorykindruntime.C32PrerequisiteCertificateFromResultsInput{
			EnumerationRequest: enumerationRequest,
			EnumerationResult:  enumerated,
			DefinednessRequest: definednessRequest,
			DefinednessResult:  defined,
			CandidateEvidence:  projection.evidence,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("seal C.3.2 prerequisite certificate: %w", err)
	}
	return Satisfied{
		certificate: certificate,
		inputs:      enumerated.Basis().ObservableInputs(),
	}, nil
}

func validateInput(input Input) error {
	query := input.Request.Query()
	view := input.Request.View()
	if query.Digest().String() == "" || view.Digest().String() == "" {
		return fmt.Errorf("%w: exact MemberOf request is required", ErrRuntimeInvalid)
	}
	if input.Signature.Ref().String() == "" || input.EntitySet.Ref().String() == "" {
		return fmt.Errorf("%w: exact signature and EntitySet are required", ErrRuntimeInvalid)
	}
	if input.Signature.ValueKind() != query.ValueKind() ||
		input.Signature.Ref().Context() != query.ContextSlice().Context() ||
		input.Signature.EntitySet() != input.EntitySet.Ref() ||
		input.EntitySet.Ref().TypeEnv() != view.TypeEnv() {
		return fmt.Errorf("%w: C.3.2 input coordinates do not match", ErrRuntimeInvalid)
	}
	return nil
}

type candidateProjection struct {
	basis    typedmemorykindruntime.EntitySetCandidateBasis
	entities []typedmemory.EntityID
	evidence typedmemorykindruntime.C32CandidateEvaluationEvidence
}

func (runtime Runtime) projectCandidates(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinition,
	persisted []typedmemory.EntityID,
) (candidateProjection, *Undefined, error) {
	switch view := request.View().(type) {
	case typedmemory.PersistedSnapshotView:
		basis, err := typedmemorykindruntime.NewPersistedEntitySetCandidateBasis(view)
		if err != nil {
			return candidateProjection{}, nil, err
		}
		return candidateProjection{
			basis:    basis,
			entities: append([]typedmemory.EntityID(nil), persisted...),
			evidence: typedmemorykindruntime.NewPersistedC32CandidateEvidence(),
		}, nil, nil
	case typedmemory.ProspectiveBatchView:
		policy, visible := entitySet.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
		if !visible {
			result, err := undefinedForVisibility(request, entitySet.Ref())
			return candidateProjection{}, &result, err
		}
		registration, err := exactRegistration(
			runtime.visibility,
			policy.EvaluationRule(),
			"candidate visibility",
		)
		if err != nil {
			return candidateProjection{}, nil, err
		}
		visibilityRequest, err := typedmemorykindruntime.NewCandidateVisibilityRequest(
			typedmemorykindruntime.CandidateVisibilityRequestInput{
				Definition: entitySet,
				View:       view,
			},
		)
		if err != nil {
			return candidateProjection{}, nil, err
		}
		visibilityResult, err := registration.Evaluator().Evaluate(visibilityRequest)
		if err != nil {
			return candidateProjection{}, nil, err
		}
		candidate, ok := visibilityResult.(typedmemorykindruntime.CandidateVisible)
		if !ok {
			return candidateProjection{}, nil, ErrRuntimeInvalid
		}
		basis, err := typedmemorykindruntime.NewProspectiveEntitySetCandidateBasis(candidate)
		if err != nil {
			return candidateProjection{}, nil, err
		}
		evidence, err := typedmemorykindruntime.NewProspectiveC32CandidateEvidence(
			visibilityRequest,
			candidate,
		)
		if err != nil {
			return candidateProjection{}, nil, err
		}
		return candidateProjection{
			basis:    basis,
			entities: visibleEntities(persisted, view, entitySet.Ref().Context()),
			evidence: evidence,
		}, nil, nil
	default:
		return candidateProjection{}, nil, ErrRuntimeInvalid
	}
}

func visibleEntities(
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

func exactRegistration[Input, Output any](
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
				"%w: %s lookup returned %s",
				ErrRuntimeInvalid,
				contract,
				lookup.Kind().String(),
			)
		}
		return found.Registration(), nil
	}
	return typedmemoryevaluation.Registration[Input, Output]{}, fmt.Errorf(
		"%w: %s evaluator for %s is absent",
		ErrRuntimeMissing,
		contract,
		rule.String(),
	)
}

func undefinedForEntitySet(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinitionRef,
	repair string,
) (Result, error) {
	missing, err := typedmemory.MissingEntitySetForMemberOf(entitySet)
	if err != nil {
		return nil, err
	}
	return newUndefined(request, []typedmemory.MemberOfMissingBasis{missing}, repair)
}

func undefinedForVisibility(
	request typedmemory.MemberOfEvaluationRequest,
	entitySet typedmemory.EntitySetDefinitionRef,
) (Undefined, error) {
	missing, err := typedmemory.MissingCandidateVisibilityForMemberOf(entitySet)
	if err != nil {
		return Undefined{}, err
	}
	result, err := newUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		"repair:member-of-c32/candidate-visibility",
	)
	if err != nil {
		return Undefined{}, err
	}
	return result.(Undefined), nil
}

func undefinedForEnumeration(
	request typedmemory.MemberOfEvaluationRequest,
	result typedmemorykindruntime.EntitySetEnumerationResult,
) (Result, error) {
	undefined, ok := result.(typedmemorykindruntime.EntitySetEnumerationUndefined)
	if !ok {
		return nil, ErrRuntimeInvalid
	}
	missing := make([]typedmemory.MemberOfMissingBasis, 0)
	for _, reference := range undefined.MissingObservableInputs() {
		basis, err := typedmemory.MissingObservableInputForMemberOf(reference)
		if err != nil {
			return nil, err
		}
		missing = append(missing, basis)
	}
	if len(missing) == 0 {
		basis, err := typedmemory.MissingEntitySetForMemberOf(undefined.DefinitionRef())
		if err != nil {
			return nil, err
		}
		missing = append(missing, basis)
	}
	return newUndefined(request, missing, undefined.Repair().String())
}

func undefinedForDefinedness(
	request typedmemory.MemberOfEvaluationRequest,
	result typedmemorykindruntime.KindDefinednessResult,
) (Result, error) {
	undefined, ok := result.(typedmemorykindruntime.KindDefinednessUndefined)
	if !ok {
		return nil, ErrRuntimeInvalid
	}
	missing, err := missingForDefinedness(request.Query(), undefined.Failure())
	if err != nil {
		return nil, err
	}
	return newUndefined(
		request,
		missing,
		"repair:member-of-c32/kind-definedness/"+undefined.Failure().Kind().String(),
	)
}

func missingForDefinedness(
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
		return nil, ErrRuntimeInvalid
	}
}

func newUndefined(
	request typedmemory.MemberOfEvaluationRequest,
	missing []typedmemory.MemberOfMissingBasis,
	repairRaw string,
) (Result, error) {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return nil, err
	}
	judgement, err := typedmemory.NewMemberOfUndefined(request, missing, repair)
	if err != nil {
		return nil, err
	}
	return Undefined{judgement: judgement}, nil
}
