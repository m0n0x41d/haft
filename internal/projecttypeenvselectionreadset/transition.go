package projecttypeenvselectionreadset

import (
	"context"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

var ErrTransitionContextRequired = errors.New(
	"transition head-observation context is required",
)

// TransitionHeadObservationInput contains the exact immutable proposal and
// graph coordinates whose predecessor must still be current in one caller-
// owned BEGIN IMMEDIATE transaction.
type TransitionHeadObservationInput struct {
	Request      projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Stage        projecttypeenvselection.ProjectTypeEnvStage
	CurrentGraph projecttypeenvselection.ProjectGraphSnapshotBasis
}

// ObserveTransitionHeadTx compares the dedicated current head with the exact
// predecessor embedded in Request and Stage. It never turns absence into
// Genesis and performs no write or transaction finish.
func ObserveTransitionHeadTx(
	ctx context.Context,
	store *projecttypeenvheadstore.Store,
	transaction *sqlitetransaction.Transaction,
	input TransitionHeadObservationInput,
) (TransitionHeadObservation, error) {
	if ctx == nil {
		return nil, ErrTransitionContextRequired
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, err
	}
	if err := verifyTransitionInput(input); err != nil {
		return nil, err
	}
	observation, err := store.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		input.Request.Project(),
	)
	if errors.Is(err, projecttypeenvheadstore.ErrStoredHeadIntegrity) {
		return HeadStorageCorrupt{cause: err}, nil
	}
	if err != nil {
		return nil, err
	}
	switch current := observation.(type) {
	case projecttypeenvstagerevalidation.ObservedNoProjectTypeEnvHead:
		head, headErr := input.Request.Head()
		if headErr != nil {
			return nil, headErr
		}
		return TransitionHeadAbsent{
			project: input.Request.Project(),
			head:    head,
		}, nil
	case projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead:
		return observeTransitionHead(transaction, input, current.State())
	default:
		return nil, fmt.Errorf(
			"dedicated project TypeEnv head observation variant is invalid",
		)
	}
}

func observeTransitionHead(
	transaction *sqlitetransaction.Transaction,
	input TransitionHeadObservationInput,
	current projecttypeenvselection.ProjectTypeEnvHeadState,
) (TransitionHeadObservation, error) {
	prior, err := transitionPriorHead(input.Request)
	if err != nil {
		return nil, err
	}
	if !sameHeadState(current, prior) {
		return TransitionPredecessorConflict{
			expected: prior,
			current:  current,
		}, nil
	}
	successor, err := projecttypeenvselection.DeriveTransitionProjectTypeEnvHeadSuccessorCandidate(
		input.Request,
		current,
		input.Stage,
	)
	if err != nil {
		return nil, err
	}
	committedGraphRevision, err := deriveCommittedGraphRevision(
		input.Request.ExpectedGraphRevision(),
	)
	if err != nil {
		return nil, err
	}
	readSet := TransitionHeadSelectionReadSet{
		state: &transitionHeadSelectionReadSetState{
			request:                input.Request,
			stage:                  input.Stage,
			currentGraph:           input.CurrentGraph,
			prior:                  current,
			successor:              successor,
			committedGraphRevision: committedGraphRevision,
		},
		capability: &transitionHeadMatchedInTransaction{
			transaction:           transaction,
			project:               current.Project(),
			head:                  current.Ref(),
			headRevision:          current.Revision(),
			selectedComposite:     current.SelectedComposite(),
			graphSnapshot:         input.CurrentGraph.Ref(),
			graphSnapshotDigest:   input.CurrentGraph.Ref().Digest(),
			expectedGraphRevision: input.CurrentGraph.GraphRevision(),
		},
	}
	if err := readSet.VerifyForTransaction(transaction); err != nil {
		return nil, err
	}
	return readSet, nil
}

func verifyTransitionInput(input TransitionHeadObservationInput) error {
	prior, err := transitionPriorHead(input.Request)
	if err != nil {
		return err
	}
	if err := projecttypeenvselection.VerifyTransitionProjectTypeEnvHeadSelectionRequestStructure(
		input.Request,
		prior,
		input.Stage,
	); err != nil {
		return fmt.Errorf("verify Transition head-selection structure: %w", err)
	}
	if err := input.CurrentGraph.Verify(); err != nil {
		return fmt.Errorf("verify Transition current graph: %w", err)
	}
	checks := []struct {
		matches bool
		label   string
	}{
		{input.Request.Project() == input.CurrentGraph.Project(), "project"},
		{input.Request.ExpectedGraphRevision() == input.CurrentGraph.GraphRevision(), "expected graph revision"},
		{input.Stage.GraphSnapshotBasis() == input.CurrentGraph.Ref(), "Stage graph snapshot"},
		{input.Stage.GraphSnapshotBasisDigest() == input.CurrentGraph.Ref().Digest(), "Stage graph snapshot digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("transition head-observation %s mismatch", check.label)
		}
	}
	return nil
}

func transitionPriorHead(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) (projecttypeenvselection.ProjectTypeEnvHeadState, error) {
	predecessor, ok := request.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return projecttypeenvselection.ProjectTypeEnvHeadState{},
			fmt.Errorf("transition request requires an exact prior head")
	}
	return projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           predecessor.Project(),
			SelectedComposite: predecessor.SelectedComposite(),
			Revision:          predecessor.HeadRevision(),
		},
	)
}
