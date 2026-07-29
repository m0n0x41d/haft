package projecttypeenvselectionreadset

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestObserveMintsSameTransactionReadSetAndDerivesIndependentRevisions(
	t *testing.T,
) {
	fixture := newGenesisReadSetFixture(t, 17)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.observedInput,
	)
	if err != nil {
		t.Fatalf("Observe(): %v", err)
	}
	readSet, ok := outcome.(GenesisHeadSelectionReadSet)
	if !ok {
		t.Fatalf("outcome = %T; want GenesisHeadSelectionReadSet", outcome)
	}
	if err := readSet.VerifyForTransaction(transaction); err != nil {
		t.Fatalf("VerifyForTransaction(same): %v", err)
	}
	successor, exists := readSet.SuccessorHead()
	if !exists {
		t.Fatal("read set has no Genesis head successor")
	}
	if successor.Revision().Value() != 1 {
		t.Fatalf(
			"Genesis HeadRevision = %d; want 1",
			successor.Revision().Value(),
		)
	}
	if successor.Project() != fixture.project ||
		successor.SelectedComposite() != fixture.stage.VerifiedComposite() {
		t.Fatal("Genesis head successor lost project or target C")
	}
	committedGraphRevision, exists := readSet.CommittedGraphRevision()
	if !exists {
		t.Fatal("read set has no committed GraphRevision")
	}
	if committedGraphRevision.Value() != 18 {
		t.Fatalf(
			"committed GraphRevision = %d; want 18",
			committedGraphRevision.Value(),
		)
	}
	assertReadSetCoordinates(t, readSet, fixture)
	if _, err := json.Marshal(readSet); !errors.Is(
		err,
		ErrGenesisHeadSelectionReadSetNotSerializable,
	) {
		t.Fatalf("MarshalJSON error = %v", err)
	}
	var decoded GenesisHeadSelectionReadSet
	if err := json.Unmarshal([]byte(`{}`), &decoded); !errors.Is(
		err,
		ErrGenesisHeadSelectionReadSetNotSerializable,
	) {
		t.Fatalf("UnmarshalJSON error = %v", err)
	}
	assertHeadRowsTx(t, ctx, transaction, 0, 0)
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("Observe finished caller transaction: %v", err)
	}
	copiedTransaction := *transaction
	if err := readSet.VerifyForTransaction(&copiedTransaction); !errors.Is(
		err,
		ErrGenesisHeadSelectionTransactionMismatch,
	) {
		t.Fatalf("copied transaction verification error = %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback read-set transaction: %v", finish.Err())
	}
	if err := readSet.VerifyForTransaction(transaction); !errors.Is(
		err,
		sqlitetransaction.ErrTransactionFinished,
	) {
		t.Fatalf("finished transaction verification error = %v", err)
	}
}

func TestObserveRejectsReadTransactionWithoutFinishingIt(t *testing.T) {
	fixture := newGenesisReadSetFixture(t, 0)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginRead(): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.observedInput,
	)
	if outcome != nil {
		t.Fatalf("read transaction outcome = %T; want nil", outcome)
	}
	if !errors.Is(err, sqlitetransaction.ErrImmediateRequired) {
		t.Fatalf("read transaction error = %v", err)
	}
	if err := transaction.RequireActive(); err != nil {
		t.Fatalf("rejected read transaction was finished: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback read transaction: %v", finish.Err())
	}
}

func TestObserveReturnsExactCurrentHeadConflict(t *testing.T) {
	fixture := newGenesisReadSetFixture(t, 9)
	ctx := context.Background()
	current := readSetHeadState(
		t,
		fixture.project,
		fixture.stage.VerifiedComposite(),
		1,
	)
	write, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(seed): %v", err)
	}
	if err := fixture.store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		write,
		current,
	); err != nil {
		t.Fatalf("seed current head: %v", err)
	}
	if finish := write.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit current head: %v", finish.Err())
	}

	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(observe): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.observedInput,
	)
	if err != nil {
		t.Fatalf("Observe(): %v", err)
	}
	conflict, ok := outcome.(GenesisPredecessorConflict)
	if !ok {
		t.Fatalf("outcome = %T; want GenesisPredecessorConflict", outcome)
	}
	observed, exists := conflict.CurrentHead()
	if !exists || !sameHeadState(observed, current) {
		t.Fatal("Genesis conflict lost the exact current head state")
	}
	assertHeadRowsTx(t, ctx, transaction, 1, 1)
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("conflict observation finished transaction: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback conflict transaction: %v", finish.Err())
	}
}

func TestObserveReturnsCorruptPositiveFootprintNotAbsence(t *testing.T) {
	fixture := newGenesisReadSetFixture(t, 4)
	head, err := fixture.request.Head()
	if err != nil {
		t.Fatalf("request.Head(): %v", err)
	}
	_, err = fixture.database.Exec(
		`INSERT INTO project_typeenv_heads (
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.project.String(),
		head.String(),
		1,
		fixture.stage.VerifiedComposite().String(),
		"sha256:"+strings.Repeat("d", 64),
		[]byte("not-a-canonical-head-state"),
	)
	if err != nil {
		t.Fatalf("insert corrupt positive footprint: %v", err)
	}
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.observedInput,
	)
	if err != nil {
		t.Fatalf("Observe transport error: %v", err)
	}
	corrupt, ok := outcome.(HeadStorageCorrupt)
	if !ok {
		t.Fatalf("outcome = %T; want HeadStorageCorrupt", outcome)
	}
	if !errors.Is(corrupt.Err(), projecttypeenvheadstore.ErrStoredHeadIntegrity) {
		t.Fatalf("corrupt outcome error = %v", corrupt.Err())
	}
	assertHeadRowsTx(t, ctx, transaction, 1, 1)
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("corrupt observation finished transaction: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback corrupt transaction: %v", finish.Err())
	}
}

func TestObserveRejectsGraphBasisSubstitutionBeforeMint(t *testing.T) {
	fixture := newGenesisReadSetFixture(t, 7)
	substitutedGraph := readSetGraphBasis(
		t,
		fixture.project,
		7,
		"b",
	)
	input := fixture.observedInput
	input.CurrentGraph = substitutedGraph
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(ctx, fixture.store, transaction, input)
	if outcome != nil {
		t.Fatalf("substituted graph outcome = %T; want nil", outcome)
	}
	if err == nil || !strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("substituted graph error = %v", err)
	}
	assertHeadRowsTx(t, ctx, transaction, 0, 0)
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("rejected graph substitution finished transaction: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback graph-substitution transaction: %v", finish.Err())
	}
}

func TestObserveRejectsMissingObservationTimeBeforeHeadRead(t *testing.T) {
	fixture := newGenesisReadSetFixture(t, 7)
	input := fixture.observedInput
	input.ObservedAt = time.Time{}
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(ctx, fixture.store, transaction, input)
	if outcome != nil {
		t.Fatalf("missing observed_at outcome = %T; want nil", outcome)
	}
	if err == nil || !strings.Contains(err.Error(), "observed_at") {
		t.Fatalf("missing observed_at error = %v", err)
	}
	assertHeadRowsTx(t, ctx, transaction, 0, 0)
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("rejected observation time finished transaction: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback missing-observation-time transaction: %v", finish.Err())
	}
}

func TestObserveRejectsCommittedGraphRevisionOverflowWithoutMint(t *testing.T) {
	fixture := newGenesisReadSetFixture(t, math.MaxUint64)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate(): %v", err)
	}
	outcome, err := ObserveGenesisHeadTx(
		ctx,
		fixture.store,
		transaction,
		fixture.observedInput,
	)
	if outcome != nil {
		t.Fatalf("overflow outcome = %T; want nil", outcome)
	}
	if !errors.Is(err, ErrCommittedGraphRevisionOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
	assertHeadRowsTx(t, ctx, transaction, 0, 0)
	if err := transaction.RequireImmediate(); err != nil {
		t.Fatalf("overflow rejection finished transaction: %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback overflow transaction: %v", finish.Err())
	}
}

func TestZeroReadSetCannotBeUsedOrSerialized(t *testing.T) {
	var readSet GenesisHeadSelectionReadSet
	if _, exists := readSet.SuccessorHead(); exists {
		t.Fatal("zero read set exposed a successor")
	}
	if _, exists := readSet.CommittedGraphRevision(); exists {
		t.Fatal("zero read set exposed a committed GraphRevision")
	}
	if err := readSet.VerifyForTransaction(nil); !errors.Is(
		err,
		ErrGenesisHeadSelectionReadSetInvalid,
	) {
		t.Fatalf("zero read-set verification error = %v", err)
	}
	if _, err := json.Marshal(readSet); !errors.Is(
		err,
		ErrGenesisHeadSelectionReadSetNotSerializable,
	) {
		t.Fatalf("zero read-set MarshalJSON error = %v", err)
	}
}

func assertReadSetCoordinates(
	t *testing.T,
	readSet GenesisHeadSelectionReadSet,
	fixture genesisReadSetFixture,
) {
	t.Helper()
	request, exists := readSet.Request()
	if !exists || request.Ref() != fixture.request.Ref() {
		t.Fatal("read set lost the exact request")
	}
	proof, exists := readSet.Proof()
	if !exists {
		t.Fatal("read set has no transaction-issued no-prior-head proof")
	}
	if err := proof.Verify(); err != nil {
		t.Fatalf("read-set proof Verify(): %v", err)
	}
	head, err := fixture.request.Head()
	if err != nil {
		t.Fatalf("request.Head(): %v", err)
	}
	expectedObservedAt := fixture.observedAt.Round(0).UTC()
	if proof.Project() != fixture.project ||
		proof.Head() != head ||
		proof.GraphSnapshotBasis() != fixture.graph.Ref() ||
		proof.GraphSnapshotBasisDigest() != fixture.graph.Ref().Digest() ||
		proof.GraphRevision() != fixture.graph.GraphRevision() ||
		!proof.ObservedAt().Equal(expectedObservedAt) {
		t.Fatal("read set lost transaction-issued proof coordinates")
	}
	decoded, err := VerifyNoPriorHeadProof(
		proof.Ref(),
		proof.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyNoPriorHeadProof(): %v", err)
	}
	if decoded.Ref() != proof.Ref() {
		t.Fatal("decoded proof lost its content address")
	}
	stage, exists := readSet.Stage()
	if !exists || stage.Ref() != fixture.stage.Ref() {
		t.Fatal("read set lost the exact immutable Stage")
	}
	graph, exists := readSet.CurrentGraph()
	if !exists || graph.Ref() != fixture.graph.Ref() {
		t.Fatal("read set lost the exact current graph basis")
	}
}

func assertHeadRowsTx(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	currentWant int64,
	historyWant int64,
) {
	t.Helper()
	var current int64
	if err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM project_typeenv_heads`,
		nil,
		[]any{&current},
	); err != nil {
		t.Fatalf("count current heads in transaction: %v", err)
	}
	var history int64
	if err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*) FROM project_typeenv_head_states`,
		nil,
		[]any{&history},
	); err != nil {
		t.Fatalf("count head history in transaction: %v", err)
	}
	if current != currentWant || history != historyWant {
		t.Fatalf(
			"head rows current/history = %d/%d; want %d/%d",
			current,
			history,
			currentWant,
			historyWant,
		)
	}
}
