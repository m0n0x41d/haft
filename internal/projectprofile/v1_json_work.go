package projectprofile

import (
	"bytes"
	"fmt"
)

const (
	profileOnboardingWorkRecordJSONSchemaV1 = "haft.project-profile.onboarding-work-record/v1"
	profileOnboardingWorkRecordJSONSchemaV2 = "haft.project-profile.onboarding-work-record/v2"
)

type methodParameterBindingJSONV1 struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type workStateTransitionJSONV1 struct {
	Kind              string `json:"kind"`
	PreStateRef       string `json:"pre_state_ref"`
	PostStateRef      string `json:"post_state_ref,omitempty"`
	DeltaPredicateRef string `json:"delta_predicate_ref,omitempty"`
}

type profileOnboardingWorkOutcomeJSONV1 struct {
	Kind                string `json:"kind"`
	PayloadDigest       string `json:"payload_digest,omitempty"`
	ObservedBasisDigest string `json:"observed_basis_digest,omitempty"`
	MissingBasisDigest  string `json:"missing_basis_digest,omitempty"`
}

type profileOnboardingWorkRecordJSONV1 struct {
	Schema                  string                         `json:"schema"`
	RecordRef               string                         `json:"record_ref"`
	WorkRef                 string                         `json:"work_ref"`
	EnactsMethodRef         string                         `json:"enacts_method_ref"`
	MethodDescriptionRef    string                         `json:"method_description_ref"`
	MethodDescriptionDigest string                         `json:"method_description_digest"`
	MethodContractRef       string                         `json:"method_contract_ref"`
	MethodContractDigest    string                         `json:"method_contract_digest"`
	ParameterBindings       []methodParameterBindingJSONV1 `json:"parameter_bindings"`
	// PerformedBy is a frozen v1 wire name. Its value is the covering
	// RoleAssignment, not the actual performer system.
	PerformedBy                       string `json:"performed_by_role_assignment_ref"`
	ProfileAuthorRoleAssignmentRef    string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorRoleAssignmentDigest string `json:"profile_author_role_assignment_digest"`
	// ExecutedWithin is a frozen v1 wire name. In this record it carries the
	// actual performer system.
	ExecutedWithin             string                             `json:"executed_within_system_ref"`
	BoundedContextRef          string                             `json:"bounded_context_ref"`
	WorkInterval               closedIntervalJSONV1               `json:"work_interval"`
	BasisObservationWindow     closedIntervalJSONV1               `json:"basis_observation_window"`
	ObservedProjectBasisRef    string                             `json:"observed_project_basis_ref"`
	ObservedProjectBasisDigest string                             `json:"observed_project_basis_digest"`
	InputRefs                  []string                           `json:"input_refs"`
	OutputRefs                 []string                           `json:"output_refs"`
	ResourceRefs               []string                           `json:"resource_refs"`
	AffectedRefKind            string                             `json:"affected_ref_kind"`
	AffectedRefs               []string                           `json:"affected_refs"`
	StatePlaneRef              string                             `json:"state_plane_ref"`
	StateTransition            workStateTransitionJSONV1          `json:"state_transition"`
	Outcome                    profileOnboardingWorkOutcomeJSONV1 `json:"outcome"`
}

func EncodeProfileOnboardingWorkRecordCanonicalJSON(
	record ProfileOnboardingWorkRecord,
) ([]byte, error) {
	dto, err := profileOnboardingWorkRecordToJSONV1(record)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingWorkRecordCanonicalJSON(
	data []byte,
) (ProfileOnboardingWorkRecord, error) {
	var dto profileOnboardingWorkRecordJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	record, err := profileOnboardingWorkRecordFromJSONV1(dto)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	canonical, err := EncodeProfileOnboardingWorkRecordCanonicalJSON(record)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("profile onboarding Work-record JSON is not canonical")
	}
	return record, nil
}

func profileOnboardingWorkRecordToJSONV1(
	record ProfileOnboardingWorkRecord,
) (profileOnboardingWorkRecordJSONV1, error) {
	validated, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return profileOnboardingWorkRecordJSONV1{}, err
	}
	transition, err := workStateTransitionToJSONV1(validated.stateTransition)
	if err != nil {
		return profileOnboardingWorkRecordJSONV1{}, err
	}
	outcome, err := profileOnboardingWorkOutcomeToJSONV1(validated.outcome)
	if err != nil {
		return profileOnboardingWorkRecordJSONV1{}, err
	}
	recordRef := validated.recordRef.String()
	workRef := validated.workRef.String()
	methodRef := validated.enactsMethodRef.String()
	methodDescriptionRef := validated.methodDescriptionRef.String()
	methodDescriptionDigest := validated.methodDescriptionDigest.String()
	methodContractRef := validated.methodContractRef.String()
	methodContractDigest := validated.methodContractDigest.String()
	parameterBindings := methodParameterBindingsToJSONV1(validated.parameterBindings)
	coveringRoleAssignment := validated.coveringRoleAssignment.String()
	assignmentRef := validated.profileAuthorRoleAssignmentRef.String()
	assignmentDigest := validated.profileAuthorRoleAssignmentDigest.String()
	actualPerformerSystem := validated.actualPerformerSystem.String()
	boundedContextRef := validated.boundedContextRef.String()
	workInterval := closedIntervalToJSONV1(validated.workInterval.closedIntervalV1)
	basisWindow := closedIntervalToJSONV1(validated.basisObservationWindow.closedIntervalV1)
	basisRef := validated.observedProjectBasisRef.String()
	basisDigest := validated.observedProjectBasisDigest.String()
	inputRefs := workInputStrings(validated.inputRefs)
	outputRefs := workOutputStrings(validated.outputRefs)
	resourceRefs := workResourceStrings(validated.resourceRefs)
	affectedRefKind := validated.affectedRefKind.String()
	affectedRefs := affectedReferentStrings(validated.affectedRefs)
	statePlaneRef := validated.statePlaneRef.String()
	edition, err := resolveProfileOnboardingWorkMethodEdition(validated)
	if err != nil {
		return profileOnboardingWorkRecordJSONV1{}, err
	}
	schema := profileOnboardingWorkRecordJSONSchema(edition)
	return profileOnboardingWorkRecordJSONV1{
		Schema:                            schema,
		RecordRef:                         recordRef,
		WorkRef:                           workRef,
		EnactsMethodRef:                   methodRef,
		MethodDescriptionRef:              methodDescriptionRef,
		MethodDescriptionDigest:           methodDescriptionDigest,
		MethodContractRef:                 methodContractRef,
		MethodContractDigest:              methodContractDigest,
		ParameterBindings:                 parameterBindings,
		PerformedBy:                       coveringRoleAssignment,
		ProfileAuthorRoleAssignmentRef:    assignmentRef,
		ProfileAuthorRoleAssignmentDigest: assignmentDigest,
		ExecutedWithin:                    actualPerformerSystem,
		BoundedContextRef:                 boundedContextRef,
		WorkInterval:                      workInterval,
		BasisObservationWindow:            basisWindow,
		ObservedProjectBasisRef:           basisRef,
		ObservedProjectBasisDigest:        basisDigest,
		InputRefs:                         inputRefs,
		OutputRefs:                        outputRefs,
		ResourceRefs:                      resourceRefs,
		AffectedRefKind:                   affectedRefKind,
		AffectedRefs:                      affectedRefs,
		StatePlaneRef:                     statePlaneRef,
		StateTransition:                   transition,
		Outcome:                           outcome,
	}, nil
}

func profileOnboardingWorkRecordFromJSONV1(
	dto profileOnboardingWorkRecordJSONV1,
) (ProfileOnboardingWorkRecord, error) {
	if dto.Schema != profileOnboardingWorkRecordJSONSchemaV1 &&
		dto.Schema != profileOnboardingWorkRecordJSONSchemaV2 {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("unsupported Work-record JSON schema %q", dto.Schema)
	}
	recordRef, err := NewProfileOnboardingWorkRecordRef(dto.RecordRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	workRef, err := NewWorkRef(dto.WorkRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	methodRef, err := NewMethodRef(dto.EnactsMethodRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	methodDescriptionRef, err := NewMethodDescriptionRef(dto.MethodDescriptionRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	methodDescriptionDigest, err := NewContentDigest(dto.MethodDescriptionDigest)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	methodContractRef, err := profileOnboardingMethodContractRefFromString(dto.MethodContractRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	methodContractDigest, err := NewContentDigest(dto.MethodContractDigest)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	bindings, err := methodParameterBindingsFromJSONV1(dto.ParameterBindings)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	coveringRoleAssignment, err := NewRoleAssignmentRef(dto.PerformedBy)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	profileAuthorRoleAssignmentRef, err := NewRoleAssignmentRef(dto.ProfileAuthorRoleAssignmentRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	profileAuthorRoleAssignmentDigest, err := NewContentDigest(dto.ProfileAuthorRoleAssignmentDigest)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	actualPerformerSystem, err := NewSystemRef(dto.ExecutedWithin)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	boundedContextRef, err := NewBoundedContextRef(dto.BoundedContextRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	workInterval, err := workIntervalFromJSONV1(dto.WorkInterval)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	basisWindow, err := basisObservationWindowFromJSONV1(dto.BasisObservationWindow)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	observedProjectBasisRef, err := NewObservedProjectBasisRefV1(dto.ObservedProjectBasisRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	observedProjectBasisDigest, err := NewContentDigest(dto.ObservedProjectBasisDigest)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	inputRefs, err := refsFromStringsV1(dto.InputRefs, NewWorkInputRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work input refs: %w", err)
	}
	outputRefs, err := refsFromStringsV1(dto.OutputRefs, NewWorkOutputRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work output refs: %w", err)
	}
	resourceRefs, err := refsFromStringsV1(dto.ResourceRefs, NewWorkResourceRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("work resource refs: %w", err)
	}
	affectedRefKind, err := profileOnboardingAffectedKindFromJSONV1(dto.AffectedRefKind)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	affectedRefs, err := refsFromStringsV1(dto.AffectedRefs, NewAffectedReferentRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, fmt.Errorf("affected refs: %w", err)
	}
	statePlaneRef, err := NewStatePlaneRef(dto.StatePlaneRef)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	transition, err := workStateTransitionFromJSONV1(dto.StateTransition)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	outcome, err := profileOnboardingWorkOutcomeFromJSONV1(dto.Outcome)
	if err != nil {
		return ProfileOnboardingWorkRecord{}, err
	}
	builder := NewProfileOnboardingWorkRecordBuilder(recordRef, workRef)
	builder = builder.Enacts(methodRef, methodDescriptionRef, bindings)
	builder = builder.WithMethodDescriptionDigest(methodDescriptionDigest)
	builder = builder.GovernedByMethodContract(methodContractRef, methodContractDigest)
	builder = builder.PerformedUnderAssignment(coveringRoleAssignment)
	builder = builder.WithProfileAuthorRoleAssignment(
		profileAuthorRoleAssignmentRef,
		profileAuthorRoleAssignmentDigest,
	)
	builder = builder.ActualPerformer(actualPerformerSystem)
	builder = builder.InContext(boundedContextRef)
	builder = builder.During(workInterval, basisWindow)
	builder = builder.WithObservedProjectBasis(observedProjectBasisRef, observedProjectBasisDigest)
	builder = builder.WithInputs(inputRefs)
	builder = builder.WithOutputs(outputRefs)
	builder = builder.WithResources(resourceRefs)
	builder = builder.AffectingKind(affectedRefKind)
	builder = builder.Affecting(affectedRefs)
	builder = builder.OnStatePlane(statePlaneRef, transition)
	builder = builder.WithOutcome(outcome)
	return builder.Build()
}

func profileOnboardingWorkRecordJSONSchema(
	edition profileOnboardingWorkMethodEdition,
) string {
	switch edition.(type) {
	case profileOnboardingWorkMethodEditionV1:
		return profileOnboardingWorkRecordJSONSchemaV1
	case profileOnboardingWorkMethodEditionV2:
		return profileOnboardingWorkRecordJSONSchemaV2
	default:
		return ""
	}
}

func profileOnboardingAffectedKindFromJSONV1(
	raw string,
) (ProfileOnboardingAffectedKindV1, error) {
	if raw != profileOnboardingAffectedKindV1Value {
		return ProfileOnboardingAffectedKindV1{}, fmt.Errorf("unknown profile-onboarding affected kind %q", raw)
	}
	return ProfileOnboardingAffectedKindV1{value: raw}, nil
}

func methodParameterBindingsToJSONV1(
	bindings MethodParameterBindings,
) []methodParameterBindingJSONV1 {
	values := bindings.Values()
	return mapSliceV1Pure(values, func(value MethodParameterBinding) methodParameterBindingJSONV1 {
		return methodParameterBindingJSONV1{
			Name:  value.Name(),
			Value: value.Value(),
		}
	})
}

func methodParameterBindingsFromJSONV1(
	values []methodParameterBindingJSONV1,
) (MethodParameterBindings, error) {
	if values == nil {
		return MethodParameterBindings{}, fmt.Errorf("parameter bindings must be an explicit array")
	}
	result, err := mapSliceV1(values, func(index int, value methodParameterBindingJSONV1) (MethodParameterBinding, error) {
		binding, err := NewMethodParameterBinding(value.Name, value.Value)
		if err != nil {
			return MethodParameterBinding{}, fmt.Errorf("parameter binding %d: %w", index, err)
		}
		return binding, nil
	})
	if err != nil {
		return MethodParameterBindings{}, err
	}
	return NewMethodParameterBindings(result)
}

func workStateTransitionToJSONV1(
	transition WorkStateTransitionV1,
) (workStateTransitionJSONV1, error) {
	switch value := transition.(type) {
	case prePostStateTransitionV1:
		return workStateTransitionJSONV1{
			Kind:         "pre_post",
			PreStateRef:  value.preStateRef.String(),
			PostStateRef: value.postStateRef.String(),
		}, nil
	case deltaStateTransitionV1:
		return workStateTransitionJSONV1{
			Kind:              "delta_predicate",
			PreStateRef:       value.preStateRef.String(),
			DeltaPredicateRef: value.deltaPredicateRef.String(),
		}, nil
	default:
		return workStateTransitionJSONV1{}, fmt.Errorf("unknown Work state-transition variant")
	}
}

func workStateTransitionFromJSONV1(
	dto workStateTransitionJSONV1,
) (WorkStateTransitionV1, error) {
	preStateRef, err := NewStateRef(dto.PreStateRef)
	if err != nil {
		return nil, err
	}
	switch dto.Kind {
	case "pre_post":
		if dto.DeltaPredicateRef != "" {
			return nil, fmt.Errorf("pre/post transition contains delta-predicate ref")
		}
		postStateRef, postErr := NewStateRef(dto.PostStateRef)
		if postErr != nil {
			return nil, postErr
		}
		return NewPrePostStateTransitionV1(preStateRef, postStateRef)
	case "delta_predicate":
		if dto.PostStateRef != "" {
			return nil, fmt.Errorf("delta transition contains post-state ref")
		}
		deltaPredicateRef, deltaErr := NewDeltaPredicateRef(dto.DeltaPredicateRef)
		if deltaErr != nil {
			return nil, deltaErr
		}
		return NewDeltaStateTransitionV1(preStateRef, deltaPredicateRef)
	default:
		return nil, fmt.Errorf("unknown Work state-transition kind %q", dto.Kind)
	}
}

func profileOnboardingWorkOutcomeToJSONV1(
	outcome ProfileOnboardingWorkOutcomeV1,
) (profileOnboardingWorkOutcomeJSONV1, error) {
	operation, err := exactProfileOnboardingWorkOutcomeOperationV1(outcome)
	if err != nil {
		return profileOnboardingWorkOutcomeJSONV1{}, err
	}
	return profileOnboardingWorkOutcomeJSONV1{
		Kind:                operation.canonicalKind,
		PayloadDigest:       operation.payloadDigest.String(),
		ObservedBasisDigest: operation.observedBasisDigest.String(),
		MissingBasisDigest:  operation.missingBasisDigest.String(),
	}, nil
}

func profileOnboardingWorkOutcomeFromJSONV1(
	dto profileOnboardingWorkOutcomeJSONV1,
) (ProfileOnboardingWorkOutcomeV1, error) {
	switch dto.Kind {
	case "candidate_payload_produced":
		if dto.MissingBasisDigest != "" {
			return nil, fmt.Errorf("candidate outcome contains missing-basis digest")
		}
		payloadDigest, err := NewContentDigest(dto.PayloadDigest)
		if err != nil {
			return nil, err
		}
		basisDigest, err := NewContentDigest(dto.ObservedBasisDigest)
		if err != nil {
			return nil, err
		}
		return NewCandidatePayloadProduced(payloadDigest, basisDigest)
	case "classification_underdetermined":
		if dto.PayloadDigest != "" || dto.ObservedBasisDigest != "" {
			return nil, fmt.Errorf("underdetermined outcome contains candidate digests")
		}
		missingDigest, err := NewContentDigest(dto.MissingBasisDigest)
		if err != nil {
			return nil, err
		}
		return NewClassificationUnderdetermined(missingDigest)
	default:
		return nil, fmt.Errorf("unknown Work outcome kind %q", dto.Kind)
	}
}

func workIntervalFromJSONV1(dto closedIntervalJSONV1) (WorkIntervalV1, error) {
	interval, err := closedIntervalFromJSONV1("Work interval", dto)
	return WorkIntervalV1{closedIntervalV1: interval}, err
}

func basisObservationWindowFromJSONV1(
	dto closedIntervalJSONV1,
) (BasisObservationWindowV1, error) {
	interval, err := closedIntervalFromJSONV1("basis-observation window", dto)
	return BasisObservationWindowV1{closedIntervalV1: interval}, err
}

func refsFromStringsV1[T any](
	values []string,
	parse func(string) (T, error),
) ([]T, error) {
	if values == nil {
		return nil, fmt.Errorf("reference list must be an explicit array")
	}
	return mapSliceV1(values, func(index int, value string) (T, error) {
		ref, err := parse(value)
		if err != nil {
			var zero T
			return zero, fmt.Errorf("reference %d: %w", index, err)
		}
		return ref, nil
	})
}
