package projecttypeenvselectionreadset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	ErrContextRequired = errors.New(
		"genesis head-observation context is required",
	)
	ErrCommittedGraphRevisionOverflow = errors.New(
		"committed graph revision overflow",
	)
)

// GenesisHeadObservationInput contains only immutable structural values.
// CurrentGraph is an exact basis obtained by an upstream transaction adapter;
// this package correlates it but does not reread the generic graph or mint its
// storage provenance. ObservedAt is supplied by the effect shell's clock; it is
// audit time, not caller-supplied proof. Stage is correlated with
// request/current graph, but this package does not assert that Stage profile,
// assertion, compatibility, edition, or runtime bases remain current.
type GenesisHeadObservationInput struct {
	Request      projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Stage        projecttypeenvselection.ProjectTypeEnvStage
	CurrentGraph projecttypeenvselection.ProjectGraphSnapshotBasis
	ObservedAt   time.Time
}

// ObserveGenesisHeadTx reads the exact dedicated head under one caller-owned
// BEGIN IMMEDIATE transaction. It performs no write and never commits or
// rolls back the transaction.
//
// Exact absence returns GenesisHeadSelectionReadSet. Exact current state
// returns GenesisPredecessorConflict. A positive corrupt footprint returns
// HeadStorageCorrupt. Only transport/lifecycle/invalid-input failures use the
// error return.
func ObserveGenesisHeadTx(
	ctx context.Context,
	store *projecttypeenvheadstore.Store,
	transaction *sqlitetransaction.Transaction,
	input GenesisHeadObservationInput,
) (GenesisHeadObservation, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := transaction.RequireImmediate(); err != nil {
		return nil, err
	}
	if err := verifyGenesisInput(input); err != nil {
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
	case projecttypeenvstagerevalidation.ObservedProjectTypeEnvHead:
		return GenesisPredecessorConflict{current: current.State()}, nil
	case projecttypeenvstagerevalidation.ObservedNoProjectTypeEnvHead:
		return observeAbsentHead(transaction, input, current)
	default:
		return nil, fmt.Errorf(
			"dedicated project TypeEnv head observation variant is invalid",
		)
	}
}

func observeAbsentHead(
	transaction *sqlitetransaction.Transaction,
	input GenesisHeadObservationInput,
	absence projecttypeenvstagerevalidation.ObservedNoProjectTypeEnvHead,
) (GenesisHeadObservation, error) {
	expectedHead, err := input.Request.Head()
	if err != nil {
		return nil, err
	}
	if absence.Project() != input.Request.Project() ||
		absence.Head() != expectedHead {
		return nil, fmt.Errorf(
			"dedicated project TypeEnv head absence coordinate mismatch",
		)
	}
	committedGraphRevision, err := deriveCommittedGraphRevision(
		input.Request.ExpectedGraphRevision(),
	)
	if err != nil {
		return nil, err
	}
	proof, err := sealNoPriorHeadProofRecord(noPriorHeadProofInput{
		project:      input.Request.Project(),
		head:         expectedHead,
		currentGraph: input.CurrentGraph,
		observedAt:   input.ObservedAt,
	})
	if err != nil {
		return nil, err
	}
	capability := &genesisHeadAbsentInTransaction{
		transaction:           transaction,
		project:               input.Request.Project(),
		head:                  expectedHead,
		proof:                 proof.Ref(),
		graphSnapshot:         input.CurrentGraph.Ref(),
		graphSnapshotDigest:   input.CurrentGraph.Ref().Digest(),
		expectedGraphRevision: input.Request.ExpectedGraphRevision(),
	}
	successor, err := projecttypeenvselection.DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(
		input.Request,
		input.Stage,
	)
	if err != nil {
		return nil, err
	}
	readSet := GenesisHeadSelectionReadSet{
		state: &genesisHeadSelectionReadSetState{
			request:                input.Request,
			proof:                  proof,
			stage:                  input.Stage,
			currentGraph:           input.CurrentGraph,
			successor:              successor,
			committedGraphRevision: committedGraphRevision,
		},
		capability: capability,
	}
	if err := readSet.VerifyForTransaction(transaction); err != nil {
		return nil, err
	}
	return readSet, nil
}

func verifyGenesisInput(input GenesisHeadObservationInput) error {
	if err := projecttypeenvselection.VerifyGenesisProjectTypeEnvHeadSelectionRequestStructure(
		input.Request,
		input.Stage,
	); err != nil {
		return fmt.Errorf("verify Genesis head-selection structure: %w", err)
	}
	if err := input.CurrentGraph.Verify(); err != nil {
		return fmt.Errorf("verify Genesis current graph: %w", err)
	}
	if canonicalNoPriorHeadProofTime(input.ObservedAt).IsZero() {
		return fmt.Errorf("genesis head-observation observed_at is required")
	}
	checks := []struct {
		matches bool
		label   string
	}{
		{input.Request.Project() == input.CurrentGraph.Project(), "project"},
		{
			input.Request.ExpectedGraphRevision() == input.CurrentGraph.GraphRevision(),
			"expected graph revision",
		},
		{
			input.Stage.GraphSnapshotBasis() == input.CurrentGraph.Ref(),
			"Stage graph snapshot",
		},
		{
			input.Stage.GraphSnapshotBasisDigest() == input.CurrentGraph.Ref().Digest(),
			"Stage graph snapshot digest",
		},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("genesis head-observation %s mismatch", check.label)
		}
	}
	return nil
}

func verifyNoPriorHeadProofAgainstGenesisInput(
	proof NoPriorHeadProofRecord,
	input GenesisHeadObservationInput,
) error {
	if err := VerifyNoPriorHeadProofAgainstGraphSnapshot(
		proof,
		input.CurrentGraph,
	); err != nil {
		return err
	}
	head, err := input.Request.Head()
	if err != nil {
		return err
	}
	observedAt := canonicalNoPriorHeadProofTime(input.ObservedAt)
	checks := []struct {
		matches bool
		label   string
	}{
		{proof.Project() == input.Request.Project(), "project"},
		{proof.Head() == head, "head"},
		{
			proof.GraphRevision() == input.Request.ExpectedGraphRevision(),
			"graph revision",
		},
		{proof.ObservedAt().Equal(observedAt), "observed_at"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf(
				"no-prior-head proof %s mismatch",
				check.label,
			)
		}
	}
	return nil
}

func deriveCommittedGraphRevision(
	expected typedmemory.GraphRevision,
) (typedmemory.GraphRevision, error) {
	if expected.Value() == math.MaxUint64 {
		return typedmemory.GraphRevision{}, ErrCommittedGraphRevisionOverflow
	}
	return typedmemory.NewGraphRevision(expected.Value() + 1), nil
}
