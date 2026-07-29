package projecttypeenvselectionreadset

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	ErrGenesisHeadSelectionReadSetInvalid = errors.New(
		"genesis head-selection read set is invalid",
	)
	ErrGenesisHeadSelectionTransactionMismatch = errors.New(
		"genesis head-selection read set belongs to a different transaction",
	)
	ErrGenesisHeadSelectionReadSetNotSerializable = errors.New(
		"genesis head-selection read set is an in-process transaction capability and cannot be serialized",
	)
	ErrTransitionHeadSelectionReadSetInvalid = errors.New(
		"transition head-selection read set is invalid",
	)
	ErrTransitionHeadSelectionTransactionMismatch = errors.New(
		"transition head-selection read set belongs to a different transaction",
	)
	ErrTransitionHeadSelectionReadSetNotSerializable = errors.New(
		"transition head-selection read set is an in-process transaction capability and cannot be serialized",
	)
)

// GenesisHeadObservation is a closed outcome of one dedicated-head read.
// A transaction-local read set, an exact current-head conflict, and a corrupt
// positive footprint stay distinct. Storage transport failures are returned
// as errors by ObserveGenesisHeadTx.
type GenesisHeadObservation interface {
	genesisHeadObservationVariant()
}

// genesisHeadAbsentInTransaction is the package-owned capability proving that
// project/head absence was read through one exact active transaction pointer.
// It has no constructor, codec, or serialized representation.
type genesisHeadAbsentInTransaction struct {
	transaction           *sqlitetransaction.Transaction
	project               projectidentity.ProjectID
	head                  projecttypeenvselection.ProjectTypeEnvHeadRef
	proof                 projecttypeenvselection.NoPriorHeadProofRef
	graphSnapshot         projecttypeenvselection.ProjectGraphSnapshotBasisRef
	graphSnapshotDigest   typedmemory.SHA256Digest
	expectedGraphRevision typedmemory.GraphRevision
}

type genesisHeadSelectionReadSetState struct {
	request                projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	proof                  NoPriorHeadProofRecord
	stage                  projecttypeenvselection.ProjectTypeEnvStage
	currentGraph           projecttypeenvselection.ProjectGraphSnapshotBasis
	successor              projecttypeenvselection.ProjectTypeEnvHeadState
	committedGraphRevision typedmemory.GraphRevision
}

// GenesisHeadSelectionReadSet binds exact immutable source values to one
// transaction-local absence observation and to separately derived head and
// graph successors. It is intentionally opaque and non-serializable.
//
// This value does not prove that Stage mutable bases are current. A later
// service must separately require CurrentSelectionStage and authority before
// any write.
type GenesisHeadSelectionReadSet struct {
	state      *genesisHeadSelectionReadSetState
	capability *genesisHeadAbsentInTransaction
}

func (GenesisHeadSelectionReadSet) genesisHeadObservationVariant() {}

func (readSet GenesisHeadSelectionReadSet) VerifyForTransaction(
	transaction *sqlitetransaction.Transaction,
) error {
	if readSet.state == nil || readSet.capability == nil {
		return ErrGenesisHeadSelectionReadSetInvalid
	}
	if transaction == nil || readSet.capability.transaction != transaction {
		return ErrGenesisHeadSelectionTransactionMismatch
	}
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	state := readSet.state
	input := GenesisHeadObservationInput{
		Request:      state.request,
		Stage:        state.stage,
		CurrentGraph: state.currentGraph,
		ObservedAt:   state.proof.ObservedAt(),
	}
	if err := verifyGenesisInput(input); err != nil {
		return err
	}
	if err := verifyNoPriorHeadProofAgainstGenesisInput(
		state.proof,
		input,
	); err != nil {
		return fmt.Errorf(
			"verify Genesis proof against current graph: %w",
			err,
		)
	}
	successor, err := projecttypeenvselection.DeriveGenesisProjectTypeEnvHeadSuccessorCandidate(
		state.request,
		state.stage,
	)
	if err != nil {
		return err
	}
	if !sameHeadState(successor, state.successor) {
		return fmt.Errorf("genesis head-selection successor differs from the structural request")
	}
	committedGraphRevision, err := deriveCommittedGraphRevision(
		state.request.ExpectedGraphRevision(),
	)
	if err != nil {
		return err
	}
	if committedGraphRevision != state.committedGraphRevision {
		return fmt.Errorf("genesis committed graph revision differs from the request")
	}
	expectedHead, err := state.request.Head()
	if err != nil {
		return err
	}
	capability := readSet.capability
	checks := []struct {
		matches bool
		label   string
	}{
		{capability.project == state.request.Project(), "project"},
		{capability.head == expectedHead, "head"},
		{capability.proof == state.proof.Ref(), "proof"},
		{capability.graphSnapshot == state.currentGraph.Ref(), "graph snapshot"},
		{
			capability.graphSnapshotDigest == state.currentGraph.Ref().Digest(),
			"graph snapshot digest",
		},
		{
			capability.expectedGraphRevision == state.currentGraph.GraphRevision(),
			"expected graph revision",
		},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf(
				"genesis transaction-local head absence %s mismatch",
				check.label,
			)
		}
	}
	return nil
}

func (readSet GenesisHeadSelectionReadSet) Request() (
	projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{}, false
	}
	return readSet.state.request, true
}

func (readSet GenesisHeadSelectionReadSet) Proof() (
	NoPriorHeadProofRecord,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return NoPriorHeadProofRecord{}, false
	}
	return readSet.state.proof, true
}

func (readSet GenesisHeadSelectionReadSet) Stage() (
	projecttypeenvselection.ProjectTypeEnvStage,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvStage{}, false
	}
	return readSet.state.stage, true
}

func (readSet GenesisHeadSelectionReadSet) CurrentGraph() (
	projecttypeenvselection.ProjectGraphSnapshotBasis,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectGraphSnapshotBasis{}, false
	}
	return readSet.state.currentGraph, true
}

func (readSet GenesisHeadSelectionReadSet) SuccessorHead() (
	projecttypeenvselection.ProjectTypeEnvHeadState,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, false
	}
	return readSet.state.successor, true
}

func (readSet GenesisHeadSelectionReadSet) CommittedGraphRevision() (
	typedmemory.GraphRevision,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return typedmemory.GraphRevision{}, false
	}
	return readSet.state.committedGraphRevision, true
}

func (GenesisHeadSelectionReadSet) MarshalJSON() ([]byte, error) {
	return nil, ErrGenesisHeadSelectionReadSetNotSerializable
}

func (*GenesisHeadSelectionReadSet) UnmarshalJSON([]byte) error {
	return ErrGenesisHeadSelectionReadSetNotSerializable
}

var (
	_ json.Marshaler   = GenesisHeadSelectionReadSet{}
	_ json.Unmarshaler = (*GenesisHeadSelectionReadSet)(nil)
)

// GenesisPredecessorConflict is a verified exact current dedicated head. It is
// not corruption and cannot be weakened into Genesis absence.
type GenesisPredecessorConflict struct {
	current projecttypeenvselection.ProjectTypeEnvHeadState
}

func (GenesisPredecessorConflict) genesisHeadObservationVariant() {}

func (conflict GenesisPredecessorConflict) CurrentHead() (
	projecttypeenvselection.ProjectTypeEnvHeadState,
	bool,
) {
	if err := conflict.current.Verify(); err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, false
	}
	return conflict.current, true
}

// HeadStorageCorrupt means the dedicated current/history footprint is
// positive but incomplete, non-canonical, or internally inconsistent. It is a
// closed non-ready outcome and never an absence witness.
type HeadStorageCorrupt struct {
	cause error
}

func (HeadStorageCorrupt) genesisHeadObservationVariant() {}

func (HeadStorageCorrupt) transitionHeadObservationVariant() {}

func (corrupt HeadStorageCorrupt) Err() error { return corrupt.cause }

func sameHeadState(
	left projecttypeenvselection.ProjectTypeEnvHeadState,
	right projecttypeenvselection.ProjectTypeEnvHeadState,
) bool {
	return left.Project() == right.Project() &&
		left.Ref() == right.Ref() &&
		left.SelectedComposite() == right.SelectedComposite() &&
		left.Revision() == right.Revision() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

// TransitionHeadObservation is the closed outcome of one exact current-head
// comparison. Exact absence, a stale predecessor, a transaction-local read
// set, and corrupt storage remain distinct.
type TransitionHeadObservation interface {
	transitionHeadObservationVariant()
}

// transitionHeadMatchedInTransaction is an unforgeable same-transaction
// capability. It proves that the exact predecessor was current when read.
type transitionHeadMatchedInTransaction struct {
	transaction           *sqlitetransaction.Transaction
	project               projectidentity.ProjectID
	head                  projecttypeenvselection.ProjectTypeEnvHeadRef
	headRevision          projecttypeenvselection.HeadRevision
	selectedComposite     typedmemory.TypeEnvRef
	graphSnapshot         projecttypeenvselection.ProjectGraphSnapshotBasisRef
	graphSnapshotDigest   typedmemory.SHA256Digest
	expectedGraphRevision typedmemory.GraphRevision
}

type transitionHeadSelectionReadSetState struct {
	request                projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	stage                  projecttypeenvselection.ProjectTypeEnvStage
	currentGraph           projecttypeenvselection.ProjectGraphSnapshotBasis
	prior                  projecttypeenvselection.ProjectTypeEnvHeadState
	successor              projecttypeenvselection.ProjectTypeEnvHeadState
	committedGraphRevision typedmemory.GraphRevision
}

// TransitionHeadSelectionReadSet binds one exact prior head and target Stage
// to a caller-owned BEGIN IMMEDIATE transaction. It has no serialized form and
// grants neither authority nor a write by itself.
type TransitionHeadSelectionReadSet struct {
	state      *transitionHeadSelectionReadSetState
	capability *transitionHeadMatchedInTransaction
}

func (TransitionHeadSelectionReadSet) transitionHeadObservationVariant() {}

func (readSet TransitionHeadSelectionReadSet) VerifyForTransaction(
	transaction *sqlitetransaction.Transaction,
) error {
	if readSet.state == nil || readSet.capability == nil {
		return ErrTransitionHeadSelectionReadSetInvalid
	}
	if transaction == nil || readSet.capability.transaction != transaction {
		return ErrTransitionHeadSelectionTransactionMismatch
	}
	if err := transaction.RequireImmediate(); err != nil {
		return err
	}
	state := readSet.state
	input := TransitionHeadObservationInput{
		Request:      state.request,
		Stage:        state.stage,
		CurrentGraph: state.currentGraph,
	}
	if err := verifyTransitionInput(input); err != nil {
		return err
	}
	successor, err := projecttypeenvselection.DeriveTransitionProjectTypeEnvHeadSuccessorCandidate(
		state.request,
		state.prior,
		state.stage,
	)
	if err != nil {
		return err
	}
	if !sameHeadState(successor, state.successor) {
		return fmt.Errorf("transition head-selection successor differs from the structural request")
	}
	committedGraphRevision, err := deriveCommittedGraphRevision(
		state.request.ExpectedGraphRevision(),
	)
	if err != nil {
		return err
	}
	if committedGraphRevision != state.committedGraphRevision {
		return fmt.Errorf("transition committed graph revision differs from the request")
	}
	capability := readSet.capability
	checks := []struct {
		matches bool
		label   string
	}{
		{capability.project == state.prior.Project(), "project"},
		{capability.head == state.prior.Ref(), "head"},
		{capability.headRevision == state.prior.Revision(), "HeadRevision"},
		{capability.selectedComposite == state.prior.SelectedComposite(), "selected C"},
		{capability.graphSnapshot == state.currentGraph.Ref(), "graph snapshot"},
		{capability.graphSnapshotDigest == state.currentGraph.Ref().Digest(), "graph snapshot digest"},
		{capability.expectedGraphRevision == state.currentGraph.GraphRevision(), "expected graph revision"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("transition transaction-local predecessor %s mismatch", check.label)
		}
	}
	return nil
}

func (readSet TransitionHeadSelectionReadSet) Request() (
	projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest{}, false
	}
	return readSet.state.request, true
}

func (readSet TransitionHeadSelectionReadSet) Stage() (
	projecttypeenvselection.ProjectTypeEnvStage,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvStage{}, false
	}
	return readSet.state.stage, true
}

func (readSet TransitionHeadSelectionReadSet) CurrentGraph() (
	projecttypeenvselection.ProjectGraphSnapshotBasis,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectGraphSnapshotBasis{}, false
	}
	return readSet.state.currentGraph, true
}

func (readSet TransitionHeadSelectionReadSet) PriorHead() (
	projecttypeenvselection.ProjectTypeEnvHeadState,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, false
	}
	return readSet.state.prior, true
}

func (readSet TransitionHeadSelectionReadSet) SuccessorHead() (
	projecttypeenvselection.ProjectTypeEnvHeadState,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, false
	}
	return readSet.state.successor, true
}

func (readSet TransitionHeadSelectionReadSet) CommittedGraphRevision() (
	typedmemory.GraphRevision,
	bool,
) {
	if readSet.state == nil || readSet.capability == nil {
		return typedmemory.GraphRevision{}, false
	}
	return readSet.state.committedGraphRevision, true
}

func (TransitionHeadSelectionReadSet) MarshalJSON() ([]byte, error) {
	return nil, ErrTransitionHeadSelectionReadSetNotSerializable
}

func (*TransitionHeadSelectionReadSet) UnmarshalJSON([]byte) error {
	return ErrTransitionHeadSelectionReadSetNotSerializable
}

var (
	_ json.Marshaler   = TransitionHeadSelectionReadSet{}
	_ json.Unmarshaler = (*TransitionHeadSelectionReadSet)(nil)
)

// TransitionHeadAbsent refuses the dangerous missing-head-to-Genesis
// reinterpretation. It carries no successor and no absence proof.
type TransitionHeadAbsent struct {
	project projectidentity.ProjectID
	head    projecttypeenvselection.ProjectTypeEnvHeadRef
}

func (TransitionHeadAbsent) transitionHeadObservationVariant() {}

func (absent TransitionHeadAbsent) Project() projectidentity.ProjectID {
	return absent.project
}

func (absent TransitionHeadAbsent) Head() projecttypeenvselection.ProjectTypeEnvHeadRef {
	return absent.head
}

// TransitionPredecessorConflict preserves the exact current head that differs
// from the request's predecessor. Callers can restage against it; they cannot
// weaken it into a generic CAS boolean.
type TransitionPredecessorConflict struct {
	expected projecttypeenvselection.ProjectTypeEnvHeadState
	current  projecttypeenvselection.ProjectTypeEnvHeadState
}

func (TransitionPredecessorConflict) transitionHeadObservationVariant() {}

func (conflict TransitionPredecessorConflict) ExpectedHead() (
	projecttypeenvselection.ProjectTypeEnvHeadState,
	bool,
) {
	if err := conflict.expected.Verify(); err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, false
	}
	return conflict.expected, true
}

func (conflict TransitionPredecessorConflict) CurrentHead() (
	projecttypeenvselection.ProjectTypeEnvHeadState,
	bool,
) {
	if err := conflict.current.Verify(); err != nil {
		return projecttypeenvselection.ProjectTypeEnvHeadState{}, false
	}
	return conflict.current, true
}
