package memberofevaluation

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// MemberOfEvaluationInput is the complete evaluator input for one exact
// transaction-time judgement. It deliberately omits the caller's expected
// judgement and basis so an engine must recompute from these inputs.
type MemberOfEvaluationInput struct {
	project     projectledger.ProjectID
	environment typedmemory.TypeEnv
	request     typedmemory.MemberOfEvaluationRequest
	observables []ObservableInputBlob
	universe    PersistedEntityUniverse
}

func NewMemberOfEvaluationInput(
	project projectledger.ProjectID,
	environment typedmemory.TypeEnv,
	request typedmemory.MemberOfEvaluationRequest,
	observables []ObservableInputBlob,
	universe PersistedEntityUniverse,
) (MemberOfEvaluationInput, error) {
	canonicalProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return MemberOfEvaluationInput{}, fmt.Errorf(
			"MemberOf evaluation project is invalid",
		)
	}
	if err := validatePersistedEntityUniverseForRequest(
		project,
		request,
		universe,
	); err != nil {
		return MemberOfEvaluationInput{}, err
	}
	copied := cloneObservableInputBlobs(observables)
	for _, blob := range copied {
		if !blob.Valid() {
			return MemberOfEvaluationInput{}, fmt.Errorf(
				"MemberOf evaluation observable input is invalid",
			)
		}
	}
	return MemberOfEvaluationInput{
		project:     project,
		environment: environment,
		request:     request,
		observables: copied,
		universe:    universe,
	}, nil
}

func (input MemberOfEvaluationInput) ProjectID() projectledger.ProjectID {
	return input.project
}

func (input MemberOfEvaluationInput) Environment() typedmemory.TypeEnv {
	return input.environment
}

func (input MemberOfEvaluationInput) Request() typedmemory.MemberOfEvaluationRequest {
	return input.request
}

func (input MemberOfEvaluationInput) ObservableInputs() []ObservableInputBlob {
	return cloneObservableInputBlobs(input.observables)
}

func (input MemberOfEvaluationInput) PersistedEntityUniverse() PersistedEntityUniverse {
	return input.universe
}

func (input MemberOfEvaluationInput) Valid() bool {
	rebuilt, err := NewMemberOfEvaluationInput(
		input.project,
		input.environment,
		input.request,
		input.observables,
		input.universe,
	)
	return err == nil && sameEvaluationInput(rebuilt, input)
}

func validatePersistedEntityUniverseForRequest(
	project projectledger.ProjectID,
	request typedmemory.MemberOfEvaluationRequest,
	universe PersistedEntityUniverse,
) error {
	switch value := universe.(type) {
	case ExactPersistedEntityUniverse:
		query := request.Query()
		matches := value.Valid() &&
			value.ProjectID() == project &&
			value.BoundedContext() == query.ContextSlice().Context() &&
			value.GraphRevision() == request.View().PreStateGraphRevision()
		if !matches {
			return fmt.Errorf(
				"persisted entity universe does not match the MemberOf request",
			)
		}
		return nil
	case PersistedEntityUniverseUnavailable:
		return nil
	default:
		return fmt.Errorf("persisted entity universe posture is required")
	}
}

func sameEvaluationInput(left, right MemberOfEvaluationInput) bool {
	if left.project != right.project ||
		left.environment.Ref() != right.environment.Ref() ||
		left.request.Digest() != right.request.Digest() ||
		len(left.observables) != len(right.observables) {
		return false
	}
	for index := range left.observables {
		leftBlob := left.observables[index]
		rightBlob := right.observables[index]
		if leftBlob.Reference() != rightBlob.Reference() ||
			leftBlob.Digest() != rightBlob.Digest() ||
			string(leftBlob.Bytes()) != string(rightBlob.Bytes()) {
			return false
		}
	}
	switch leftUniverse := left.universe.(type) {
	case ExactPersistedEntityUniverse:
		rightUniverse, ok := right.universe.(ExactPersistedEntityUniverse)
		return ok && leftUniverse.Digest() == rightUniverse.Digest()
	case PersistedEntityUniverseUnavailable:
		_, ok := right.universe.(PersistedEntityUniverseUnavailable)
		return ok
	default:
		return false
	}
}

type MemberOfEvaluationEngine interface {
	EvaluateMemberOf(
		context.Context,
		MemberOfEvaluationInput,
	) (typedmemory.MemberOfJudgement, error)
}

type SnapshotObservableInputSelector interface {
	SelectSnapshotObservableInputs(
		MemberOfEvaluationInput,
	) SnapshotObservableInputSelection
}

type SnapshotObservableInputSelection interface {
	snapshotObservableInputSelectionVariant()
}

type SnapshotObservableInputsSelected struct {
	blobs []ObservableInputBlob
}

func NewSnapshotObservableInputsSelected(
	blobs []ObservableInputBlob,
) (SnapshotObservableInputsSelected, error) {
	verified, err := newImmutableObservableInputCatalog(blobs)
	if err != nil {
		return SnapshotObservableInputsSelected{}, err
	}
	if verified.Len() == 0 {
		return SnapshotObservableInputsSelected{}, fmt.Errorf(
			"snapshot MemberOf observable selection requires at least one exact source",
		)
	}
	return SnapshotObservableInputsSelected{blobs: verified.Blobs()}, nil
}

func (selection SnapshotObservableInputsSelected) ObservableInputs() []ObservableInputBlob {
	return cloneObservableInputBlobs(selection.blobs)
}

func (selection SnapshotObservableInputsSelected) Valid() bool {
	rebuilt, err := NewSnapshotObservableInputsSelected(selection.blobs)
	return err == nil && len(rebuilt.blobs) == len(selection.blobs)
}

func (SnapshotObservableInputsSelected) snapshotObservableInputSelectionVariant() {}

// SnapshotObservableInputsNotApplicable records that the exact selected
// evaluator inspected the available snapshot catalog and found no observable
// source whose grammar applies to this query. It is not source unavailability:
// the evaluator may be invoked with the exact empty source set so it can return
// its typed no-applicable-source judgement.
type SnapshotObservableInputsNotApplicable struct{}

func NewSnapshotObservableInputsNotApplicable() SnapshotObservableInputsNotApplicable {
	return SnapshotObservableInputsNotApplicable{}
}

func (SnapshotObservableInputsNotApplicable) Valid() bool { return true }

func (SnapshotObservableInputsNotApplicable) snapshotObservableInputSelectionVariant() {}

type SnapshotObservableInputsUnavailable struct{}

func NewSnapshotObservableInputsUnavailable() SnapshotObservableInputsUnavailable {
	return SnapshotObservableInputsUnavailable{}
}

func (SnapshotObservableInputsUnavailable) Valid() bool { return true }

func (SnapshotObservableInputsUnavailable) snapshotObservableInputSelectionVariant() {}
