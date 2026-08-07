package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/m0n0x41d/haft/internal/projecttypeenvactivation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ProjectTypeEnvActivationGraphInput is the storage-owned part of one
// activation event. Authority, head, receipt, and selection-closure effects
// remain outside this package and are written by the interleaved callback.
type ProjectTypeEnvActivationGraphInput struct {
	Request               projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	BasisTypeEnv          typedmemory.TypeEnvRef
	StorageIdempotencyKey string
	Delta                 projecttypeenvactivation.Delta
}

// ProjectTypeEnvActivationGraphIdentity is the first pure preparation result.
// Its coordinates let the authority-effect layer seal the manifest without
// granting a transaction or mutation capability.
type ProjectTypeEnvActivationGraphIdentity struct {
	request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	basisTypeEnv   typedmemory.TypeEnvRef
	idempotencyKey IdempotencyKey
	delta          projecttypeenvactivation.Delta
	identity       genericEventIdentity
	event          projecttypeenvselection.GraphEventRef
	commit         projecttypeenvselection.GraphCommitRef
}

func (value ProjectTypeEnvActivationGraphIdentity) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value ProjectTypeEnvActivationGraphIdentity) EventDigest() typedmemory.SHA256Digest {
	return value.identity.eventDigest
}

func (value ProjectTypeEnvActivationGraphIdentity) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func (value ProjectTypeEnvActivationGraphIdentity) ProjectionJobRef() string {
	return value.identity.projectionJobRef
}

func (value ProjectTypeEnvActivationGraphIdentity) GraphRevision() typedmemory.GraphRevision {
	return value.identity.nextRevision
}

func (value ProjectTypeEnvActivationGraphIdentity) StorageIdempotencyKey() string {
	return value.idempotencyKey.String()
}

// PrepareProjectTypeEnvActivationGraph derives the generic event/commit
// identity from the exact activation delta. It performs no I/O.
func PrepareProjectTypeEnvActivationGraph(
	input ProjectTypeEnvActivationGraphInput,
) (ProjectTypeEnvActivationGraphIdentity, error) {
	if err := input.Request.Verify(); err != nil {
		return ProjectTypeEnvActivationGraphIdentity{},
			fmt.Errorf("prepare ProjectTypeEnv activation graph request: %w", err)
	}
	if err := input.Delta.Verify(); err != nil {
		return ProjectTypeEnvActivationGraphIdentity{},
			fmt.Errorf("prepare ProjectTypeEnv activation graph delta: %w", err)
	}
	basis, err := typedmemory.ParseTypeEnvRef(input.BasisTypeEnv.String())
	if err != nil || basis != input.BasisTypeEnv {
		return ProjectTypeEnvActivationGraphIdentity{},
			fmt.Errorf("ProjectTypeEnv activation graph basis TypeEnv is required")
	}
	key, err := NewIdempotencyKey(input.StorageIdempotencyKey)
	if err != nil {
		return ProjectTypeEnvActivationGraphIdentity{}, err
	}
	if err := verifyActivationRequestDelta(input.Request, basis, input.Delta); err != nil {
		return ProjectTypeEnvActivationGraphIdentity{}, err
	}
	expected := input.Request.ExpectedGraphRevision()
	if expected.Value() >= mathMaxSQLiteRevision ||
		expected.Value() == math.MaxUint64 ||
		input.Delta.SuccessorHeadRevision().Value() > mathMaxSQLiteRevision {
		return ProjectTypeEnvActivationGraphIdentity{}, ErrRevisionOverflow
	}
	next := typedmemory.NewGraphRevision(expected.Value() + 1)
	expectedSQLiteRevision, exactExpectedRevision := sqliteIntegerFromUint64(expected.Value())
	if !exactExpectedRevision {
		return ProjectTypeEnvActivationGraphIdentity{}, ErrRevisionOverflow
	}
	nextSQLiteRevision, exactNextRevision := sqliteIntegerFromUint64(next.Value())
	if !exactNextRevision {
		return ProjectTypeEnvActivationGraphIdentity{}, ErrRevisionOverflow
	}
	commitText := derivedRef(
		"typed-memory-commit",
		input.Request.Project().String(),
		key.String(),
		input.Delta.Digest().String(),
		strconv.FormatUint(next.Value(), 10),
	)
	eventDigest, err := digestFields(
		"typed-memory-graph-event.v1",
		input.Request.Project().String(),
		commitText,
		strconv.FormatUint(expected.Value(), 10),
		strconv.FormatUint(next.Value(), 10),
		basis.String(),
		input.Delta.Digest().String(),
		string(input.Delta.CanonicalBytes()),
		input.Delta.EventKind(),
		input.Delta.AuthorityClass(),
		input.Request.Ref().String(),
	)
	if err != nil {
		return ProjectTypeEnvActivationGraphIdentity{}, err
	}
	eventText := derivedRef("typed-memory-event", eventDigest.String())
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return ProjectTypeEnvActivationGraphIdentity{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return ProjectTypeEnvActivationGraphIdentity{}, err
	}
	return ProjectTypeEnvActivationGraphIdentity{
		request:        input.Request,
		basisTypeEnv:   basis,
		idempotencyKey: key,
		delta:          input.Delta,
		identity: genericEventIdentity{
			nextRevision:           next,
			expectedSQLiteRevision: expectedSQLiteRevision,
			nextSQLiteRevision:     nextSQLiteRevision,
			commitRef:              commitText,
			eventRef:               eventText,
			eventDigest:            eventDigest,
			projectionJobRef:       derivedRef("typed-memory-projection-job", commitText),
		},
		event:  event,
		commit: commit,
	}, nil
}

// PreparedProjectTypeEnvActivationGraph is the closed storage write intent.
// It authenticates the caller-sealed activation envelope/basis/manifest and
// the generic materialization closure before any SQL effect begins.
type PreparedProjectTypeEnvActivationGraph struct {
	graph                  ProjectTypeEnvActivationGraphIdentity
	envelope               projecttypeenvactivation.AdmissionEnvelope
	basis                  projecttypeenvactivation.AdmissionBasis
	manifest               projecttypeenvactivation.MaterializationManifest
	materializationBytes   []byte
	materializationDigest  typedmemory.SHA256Digest
	materializationDigests []string
}

func (value PreparedProjectTypeEnvActivationGraph) EventRef() projecttypeenvselection.GraphEventRef {
	return value.graph.EventRef()
}

func (value PreparedProjectTypeEnvActivationGraph) EventDigest() typedmemory.SHA256Digest {
	return value.graph.EventDigest()
}

func (value PreparedProjectTypeEnvActivationGraph) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.graph.CommitRef()
}

func (value PreparedProjectTypeEnvActivationGraph) ProjectionJobRef() string {
	return value.graph.ProjectionJobRef()
}

func (value PreparedProjectTypeEnvActivationGraph) GraphRevision() typedmemory.GraphRevision {
	return value.graph.GraphRevision()
}

func (value PreparedProjectTypeEnvActivationGraph) MaterializationDigest() typedmemory.SHA256Digest {
	return value.materializationDigest
}

func (value PreparedProjectTypeEnvActivationGraph) Delta() projecttypeenvactivation.Delta {
	decoded, _ := projecttypeenvactivation.DecodeDelta(value.graph.delta.CanonicalBytes())
	return decoded
}

// SealPreparedProjectTypeEnvActivationGraph closes the activation carriers
// over the storage-owned event and commit coordinates. It performs no I/O.
func SealPreparedProjectTypeEnvActivationGraph(
	graph ProjectTypeEnvActivationGraphIdentity,
	envelope projecttypeenvactivation.AdmissionEnvelope,
	basis projecttypeenvactivation.AdmissionBasis,
	manifest projecttypeenvactivation.MaterializationManifest,
) (PreparedProjectTypeEnvActivationGraph, error) {
	if err := verifyPreparedActivationGraphIdentity(graph); err != nil {
		return PreparedProjectTypeEnvActivationGraph{}, err
	}
	if err := projecttypeenvactivation.VerifyClosure(
		graph.delta,
		envelope,
		basis,
		manifest,
	); err != nil {
		return PreparedProjectTypeEnvActivationGraph{},
			fmt.Errorf("seal ProjectTypeEnv activation storage closure: %w", err)
	}
	if envelope.GraphIdempotencyKey() != graph.idempotencyKey.String() ||
		manifest.EventRef() != graph.event ||
		manifest.CommitRef() != graph.commit {
		return PreparedProjectTypeEnvActivationGraph{},
			fmt.Errorf("activation storage coordinates differ from sealed carriers")
	}
	rowDigests := []string{
		"type-env-activation:" + graph.delta.Digest().String(),
	}
	closureInput := materializationClosureInput{
		identity:       graph.identity,
		basisKind:      typedmemory.SnapshotOnlyAdmissionBasis,
		requestDigest:  graph.request.Ref().Digest(),
		semanticDigest: graph.delta.Digest(),
		envelopeDigest: envelope.Digest(),
		basisDigest:    basis.Digest(),
		manifestDigest: manifest.Digest(),
		footprint:      genericMaterializationFootprint{},
		rowDigests:     rowDigests,
	}
	canonical := canonicalMaterializationClosureFromInput(closureInput)
	digest, err := digestBytes(canonical)
	if err != nil {
		return PreparedProjectTypeEnvActivationGraph{}, err
	}
	return PreparedProjectTypeEnvActivationGraph{
		graph:                  graph,
		envelope:               envelope,
		basis:                  basis,
		manifest:               manifest,
		materializationBytes:   canonical,
		materializationDigest:  digest,
		materializationDigests: rowDigests,
	}, nil
}

// ProjectTypeEnvActivationWriteContext is the exact storage coordinate passed
// to the effect callback. RecordedAt is the one canonical timestamp shared by
// all store-owned rows; callers may use it for their interleaved effect rows.
type ProjectTypeEnvActivationWriteContext struct {
	prepared   PreparedProjectTypeEnvActivationGraph
	recordedAt string
}

func (value ProjectTypeEnvActivationWriteContext) EventRef() projecttypeenvselection.GraphEventRef {
	return value.prepared.EventRef()
}

func (value ProjectTypeEnvActivationWriteContext) EventDigest() typedmemory.SHA256Digest {
	return value.prepared.EventDigest()
}

func (value ProjectTypeEnvActivationWriteContext) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.prepared.CommitRef()
}

func (value ProjectTypeEnvActivationWriteContext) ProjectionJobRef() string {
	return value.prepared.ProjectionJobRef()
}

func (value ProjectTypeEnvActivationWriteContext) GraphRevision() typedmemory.GraphRevision {
	return value.prepared.GraphRevision()
}

func (value ProjectTypeEnvActivationWriteContext) MaterializationDigest() typedmemory.SHA256Digest {
	return value.prepared.MaterializationDigest()
}

func (value ProjectTypeEnvActivationWriteContext) RecordedAt() string {
	return value.recordedAt
}

type ProjectTypeEnvActivationEffectWriter func(
	context.Context,
	*sqlitetransaction.Transaction,
	ProjectTypeEnvActivationWriteContext,
) error

// ProjectTypeEnvActivationAdapter owns only generic graph storage effects.
// The stage store is injected pre-opened so schema setup cannot occur inside a
// caller-owned activation transaction.
type ProjectTypeEnvActivationAdapter struct {
	clock       Clock
	stages      *projecttypeenvstage.Store
	verifyStage projectTypeEnvActivationStageVerifier
}

type projectTypeEnvActivationStageVerifier func(
	context.Context,
	*sqlitetransaction.Transaction,
	PreparedProjectTypeEnvActivationGraph,
) error

func NewProjectTypeEnvActivationAdapter(
	clock Clock,
	stages *projecttypeenvstage.Store,
) (*ProjectTypeEnvActivationAdapter, error) {
	if clock == nil {
		return nil, ErrClockRequired
	}
	if stages == nil {
		return nil, projecttypeenvstage.ErrStoreRequired
	}
	adapter := &ProjectTypeEnvActivationAdapter{
		clock:  clock,
		stages: stages,
	}
	adapter.verifyStage = adapter.verifySelectionReadyStageTx
	return adapter, nil
}

// WritePreparedProjectTypeEnvActivationGraphTx writes into one caller-owned
// BEGIN IMMEDIATE transaction. It never commits or rolls back. A callback
// error deliberately leaves the prelude rows in the transaction so the owner
// can prove and perform rollback.
func (adapter *ProjectTypeEnvActivationAdapter) WritePreparedProjectTypeEnvActivationGraphTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared PreparedProjectTypeEnvActivationGraph,
	writeEffect ProjectTypeEnvActivationEffectWriter,
) (CurrentProjectGraphObservation, error) {
	if ctx == nil {
		return CurrentProjectGraphObservation{},
			fmt.Errorf("write ProjectTypeEnv activation graph: context is required")
	}
	if adapter == nil ||
		adapter.clock == nil ||
		adapter.stages == nil ||
		adapter.verifyStage == nil {
		return CurrentProjectGraphObservation{}, projecttypeenvstage.ErrStoreRequired
	}
	if err := transaction.RequireImmediate(); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	if writeEffect == nil {
		return CurrentProjectGraphObservation{},
			fmt.Errorf("ProjectTypeEnv activation effect writer is required")
	}
	if err := verifyPreparedProjectTypeEnvActivationGraph(prepared); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	if err := verifyActivationCoordinateOwnersTx(ctx, transaction, prepared); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	if err := adapter.verifyStage(ctx, transaction, prepared); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	recordedAt := canonicalTime(adapter.clock.Now())
	prelude := activationPreludeStatements(prepared, recordedAt)
	if err := executeStatements(ctx, transaction, prelude, 0); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	writeContext := ProjectTypeEnvActivationWriteContext{
		prepared:   prepared,
		recordedAt: recordedAt,
	}
	if err := writeEffect(ctx, transaction, writeContext); err != nil {
		return CurrentProjectGraphObservation{},
			fmt.Errorf("write ProjectTypeEnv activation effect closure: %w", err)
	}
	if err := verifyInterleavedActivationEffectTx(
		ctx,
		transaction,
		prepared,
		recordedAt,
	); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	closure := activationMaterializationClosureStatement(prepared, recordedAt)
	if err := executeStatements(ctx, transaction, []statement{closure}, 0); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	commit := activationGraphCommitStatement(prepared, recordedAt)
	if err := executeStatements(ctx, transaction, []statement{commit}, 0); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	observation, err := LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		prepared.graph.request.Project(),
	)
	if err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	if err := verifyActivationObservation(prepared, observation); err != nil {
		return CurrentProjectGraphObservation{}, err
	}
	return observation, nil
}

func verifyActivationRequestDelta(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	basis typedmemory.TypeEnvRef,
	delta projecttypeenvactivation.Delta,
) error {
	target := request.Target()
	deltaTarget := delta.Target()
	head, err := request.Head()
	if err != nil {
		return err
	}
	matches := request.Project() == delta.Project() &&
		head == delta.Head() &&
		request.Ref() == delta.RequestRef() &&
		request.Ref().Digest() == delta.RequestDigest() &&
		request.ExpectedGraphRevision() == delta.ExpectedGraphRevision() &&
		delta.CommittedGraphRevision().Value() == request.ExpectedGraphRevision().Value()+1 &&
		target.Base() == deltaTarget.Base() &&
		target.RuntimeBasis() == deltaTarget.RuntimeBasis() &&
		target.VerifiedComposite() == deltaTarget.Composite() &&
		target.Stage() == deltaTarget.Stage() &&
		orderedActivationExtensionsEqual(
			target.OrderedExtensions(),
			deltaTarget.OrderedExtensions(),
		) &&
		activationPredecessorsEqual(request.Predecessor(), delta.Predecessor())
	if !matches {
		return fmt.Errorf("activation request and delta coordinates differ")
	}
	if basis == target.VerifiedComposite() {
		return fmt.Errorf("activation basis and target TypeEnv must differ")
	}
	switch predecessor := request.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		if basis != target.Base() {
			return fmt.Errorf("genesis activation basis differs from target base B")
		}
		_ = predecessor
	case projecttypeenvselection.TransitionStagePredecessor:
		if predecessor.SelectedComposite() != basis {
			return fmt.Errorf("transition activation basis differs from prior selected TypeEnv")
		}
	default:
		return fmt.Errorf("activation predecessor posture is invalid")
	}
	return nil
}

func verifyPreparedActivationGraphIdentity(
	value ProjectTypeEnvActivationGraphIdentity,
) error {
	rebuilt, err := PrepareProjectTypeEnvActivationGraph(
		ProjectTypeEnvActivationGraphInput{
			Request:               value.request,
			BasisTypeEnv:          value.basisTypeEnv,
			StorageIdempotencyKey: value.idempotencyKey.String(),
			Delta:                 value.delta,
		},
	)
	if err != nil {
		return err
	}
	matches := rebuilt.identity == value.identity &&
		rebuilt.event == value.event &&
		rebuilt.commit == value.commit
	if !matches {
		return fmt.Errorf("prepared activation graph identity differs from canonical inputs")
	}
	return nil
}

func verifyPreparedProjectTypeEnvActivationGraph(
	value PreparedProjectTypeEnvActivationGraph,
) error {
	rebuilt, err := SealPreparedProjectTypeEnvActivationGraph(
		value.graph,
		value.envelope,
		value.basis,
		value.manifest,
	)
	if err != nil {
		return err
	}
	matches := rebuilt.materializationDigest == value.materializationDigest &&
		bytes.Equal(rebuilt.materializationBytes, value.materializationBytes) &&
		orderedStringsEqual(rebuilt.materializationDigests, value.materializationDigests)
	if !matches {
		return fmt.Errorf("prepared activation graph differs from canonical carriers")
	}
	return nil
}

func activationPredecessorsEqual(
	left projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
	right projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) bool {
	switch value := left.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		_, ok := right.(projecttypeenvselection.GenesisStagePredecessor)
		return ok
	case projecttypeenvselection.TransitionStagePredecessor:
		candidate, ok := right.(projecttypeenvselection.TransitionStagePredecessor)
		return ok &&
			value.Project() == candidate.Project() &&
			value.Head() == candidate.Head() &&
			value.HeadRevision() == candidate.HeadRevision() &&
			value.SelectedComposite() == candidate.SelectedComposite()
	default:
		return false
	}
}

func orderedActivationExtensionsEqual(
	left []typedmemory.TypeEnvExtensionRef,
	right []typedmemory.TypeEnvExtensionRef,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func orderedStringsEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func verifyActivationCoordinateOwnersTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared PreparedProjectTypeEnvActivationGraph,
) error {
	head, err := loadHeadWithScanner(ctx, transaction, prepared.graph.request.Project())
	if err != nil {
		return err
	}
	if head.Revision() != prepared.graph.request.ExpectedGraphRevision() {
		return ErrStaleGraphRevision
	}
	if head.ActiveTypeEnv() != prepared.graph.basisTypeEnv {
		return ErrActiveTypeEnvMismatch
	}
	if err := requireTypeEnvCoordinateTx(
		ctx,
		transaction,
		prepared.graph.basisTypeEnv,
	); err != nil {
		return fmt.Errorf("resolve activation basis TypeEnv coordinate: %w", err)
	}
	target := prepared.graph.request.Target()
	if err := requireProjectExecutableCoordinateTx(
		ctx,
		transaction,
		target.VerifiedComposite(),
	); err != nil {
		return fmt.Errorf("resolve activation target TypeEnv coordinate: %w", err)
	}
	return nil
}

func (adapter *ProjectTypeEnvActivationAdapter) verifySelectionReadyStageTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared PreparedProjectTypeEnvActivationGraph,
) error {
	target := prepared.graph.request.Target()
	ready, err := adapter.stages.LoadSelectionReadyTx(
		ctx,
		transaction,
		target.Stage(),
	)
	if err != nil {
		return err
	}
	if err := projecttypeenvselection.VerifyProjectTypeEnvHeadSelectionRequestAgainstStage(
		prepared.graph.request,
		ready.Stage(),
	); err != nil {
		return fmt.Errorf("activation request differs from transaction-reloaded Stage: %w", err)
	}
	if ready.ExecutableSnapshot().TypeEnvRef() != target.VerifiedComposite() ||
		ready.ExecutableSnapshot().Record().TypeEnvRef() != target.VerifiedComposite() {
		return fmt.Errorf("transaction-reloaded Stage executable snapshot differs from target C")
	}
	return nil
}

func requireTypeEnvCoordinateTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvRef,
) error {
	var representation string
	var genericRef sql.NullString
	var projectRef sql.NullString
	err := transaction.ScanOne(
		ctx,
		`SELECT representation_kind, generic_snapshot_ref, project_executable_ref
		FROM typed_memory_type_env_coordinates
		WHERE type_env_ref = ?`,
		[]any{ref.String()},
		[]any{&representation, &genericRef, &projectRef},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("TypeEnv coordinate %q is not registered", ref.String())
	}
	if err != nil {
		return err
	}
	switch representation {
	case "generic_snapshot":
		if !genericRef.Valid || genericRef.String != ref.String() || projectRef.Valid {
			return fmt.Errorf("generic TypeEnv coordinate ownership is malformed")
		}
		snapshot, found, loadErr := loadTypeEnvSnapshotWithScanner(ctx, transaction, ref)
		if loadErr != nil {
			return loadErr
		}
		if !found || snapshot.Ref() != ref {
			return fmt.Errorf("generic TypeEnv coordinate owner is missing")
		}
	case "project_executable":
		if genericRef.Valid || !projectRef.Valid || projectRef.String != ref.String() {
			return fmt.Errorf("project executable TypeEnv coordinate ownership is malformed")
		}
	default:
		return fmt.Errorf("TypeEnv coordinate representation %q is unsupported", representation)
	}
	return nil
}

func requireProjectExecutableCoordinateTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref typedmemory.TypeEnvRef,
) error {
	if err := requireTypeEnvCoordinateTx(ctx, transaction, ref); err != nil {
		return err
	}
	var representation string
	err := transaction.ScanOne(
		ctx,
		`SELECT representation_kind
		FROM typed_memory_type_env_coordinates
		WHERE type_env_ref = ?`,
		[]any{ref.String()},
		[]any{&representation},
	)
	if err != nil {
		return err
	}
	if representation != "project_executable" {
		return fmt.Errorf("activation target C is not a project executable TypeEnv")
	}
	return nil
}

func activationPreludeStatements(
	prepared PreparedProjectTypeEnvActivationGraph,
	recordedAt string,
) []statement {
	graph := prepared.graph
	request := graph.request
	delta := graph.delta
	return []statement{
		{
			query: `INSERT INTO typed_memory_graph_events (
				project_id, event_ref, commit_ref, event_digest,
				expected_revision, graph_revision,
				basis_type_env_ref, result_type_env_ref,
				change_set_digest, canonical_change_set_bytes, change_count,
				event_kind, authority_class, request_provenance_ref, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)`,
			arguments: []any{
				request.Project().String(),
				graph.identity.eventRef,
				graph.identity.commitRef,
				graph.identity.eventDigest.String(),
				graph.identity.expectedSQLiteRevision,
				graph.identity.nextSQLiteRevision,
				graph.basisTypeEnv.String(),
				request.Target().VerifiedComposite().String(),
				delta.Digest().String(),
				delta.CanonicalBytes(),
				delta.EventKind(),
				delta.AuthorityClass(),
				request.Ref().String(),
				recordedAt,
			},
		},
		{
			query: `INSERT INTO typed_memory_event_writer_generations (
				project_id, event_ref, writer_generation, provenance_kind
			) VALUES (?, ?, 46, 'writer_v46')`,
			arguments: []any{request.Project().String(), graph.identity.eventRef},
		},
		{
			query: `INSERT INTO typed_memory_event_admission_bases (
				project_id, event_ref, event_digest, admission_basis_kind,
				type_env_ref, basis_graph_revision,
				request_digest, canonical_request_bytes,
				semantic_digest, canonical_semantic_bytes,
				admission_envelope_digest, canonical_admission_envelope_bytes,
				admission_basis_digest, canonical_admission_basis_bytes,
				materialization_manifest_digest,
				canonical_materialization_manifest_bytes, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			arguments: []any{
				request.Project().String(),
				graph.identity.eventRef,
				graph.identity.eventDigest.String(),
				projecttypeenvactivation.AdmissionKindSnapshotOnly,
				graph.basisTypeEnv.String(),
				graph.identity.expectedSQLiteRevision,
				request.Ref().Digest().String(),
				request.CanonicalBytes(),
				delta.Digest().String(),
				delta.CanonicalBytes(),
				prepared.envelope.Digest().String(),
				prepared.envelope.CanonicalBytes(),
				prepared.basis.Digest().String(),
				prepared.basis.CanonicalBytes(),
				prepared.manifest.Digest().String(),
				prepared.manifest.CanonicalBytes(),
				recordedAt,
			},
		},
		{
			query: `INSERT INTO typed_memory_idempotency_history (
				project_id, idempotency_key, change_set_digest,
				event_ref, graph_revision, result_digest, recorded_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			arguments: []any{
				request.Project().String(),
				graph.idempotencyKey.String(),
				delta.Digest().String(),
				graph.identity.eventRef,
				graph.identity.nextSQLiteRevision,
				graph.identity.eventDigest.String(),
				recordedAt,
			},
		},
		{
			query: `INSERT INTO typed_memory_projection_jobs (
				project_id, projection_job_ref, semantic_event_ref,
				graph_revision, target_kind, input_event_digest, recorded_at
			) VALUES (?, ?, ?, ?, 'project_carriers', ?, ?)`,
			arguments: []any{
				request.Project().String(),
				graph.identity.projectionJobRef,
				graph.identity.eventRef,
				graph.identity.nextSQLiteRevision,
				graph.identity.eventDigest.String(),
				recordedAt,
			},
		},
	}
}

func activationMaterializationClosureStatement(
	prepared PreparedProjectTypeEnvActivationGraph,
	recordedAt string,
) statement {
	graph := prepared.graph
	return statement{
		query: `INSERT INTO typed_memory_commit_materialization_closures (
			project_id, event_ref, commit_ref, event_digest,
			admission_basis_kind, request_digest, semantic_digest,
			admission_envelope_digest, admission_basis_digest,
			materialization_manifest_digest,
			materialization_digest, canonical_materialization_bytes,
			entity_count, entity_context_count, entity_declaration_count,
			context_slice_catalog_count, context_slice_count,
			value_blob_count, observable_input_blob_count, relation_count,
			relation_slot_count, relation_filler_count,
			ordered_candidate_prefix_count,
			reference_resolution_use_count, memberof_evaluation_count,
			memberof_input_count, memberof_use_count, alias_change_count,
			retraction_count, type_env_activation_count, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, ?)`,
		arguments: []any{
			graph.request.Project().String(),
			graph.identity.eventRef,
			graph.identity.commitRef,
			graph.identity.eventDigest.String(),
			projecttypeenvactivation.AdmissionKindSnapshotOnly,
			graph.request.Ref().Digest().String(),
			graph.delta.Digest().String(),
			prepared.envelope.Digest().String(),
			prepared.basis.Digest().String(),
			prepared.manifest.Digest().String(),
			prepared.materializationDigest.String(),
			prepared.materializationBytes,
			recordedAt,
		},
	}
}

func activationGraphCommitStatement(
	prepared PreparedProjectTypeEnvActivationGraph,
	recordedAt string,
) statement {
	graph := prepared.graph
	return statement{
		query: `INSERT INTO typed_memory_graph_commits (
			project_id, commit_ref, event_ref, event_digest,
			expected_revision, graph_revision, change_set_digest,
			idempotency_key, projection_job_ref,
			entity_count, entity_context_count, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?)`,
		arguments: []any{
			graph.request.Project().String(),
			graph.identity.commitRef,
			graph.identity.eventRef,
			graph.identity.eventDigest.String(),
			graph.identity.expectedSQLiteRevision,
			graph.identity.nextSQLiteRevision,
			graph.delta.Digest().String(),
			graph.idempotencyKey.String(),
			graph.identity.projectionJobRef,
			recordedAt,
		},
	}
}

func verifyInterleavedActivationEffectTx(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared PreparedProjectTypeEnvActivationGraph,
	recordedAt string,
) error {
	graph := prepared.graph
	delta := graph.delta
	var activationRef string
	var activationDigest string
	var activationBytes []byte
	var requestRef string
	var requestDigest string
	var basisRef string
	var resultRef string
	var stageRef string
	var expectedRevision int64
	var committedRevision int64
	var storedRecordedAt string
	err := transaction.ScanOne(
		ctx,
		`SELECT activation_ref, activation_digest, canonical_activation_bytes,
			request_ref, request_digest, basis_type_env_ref, result_type_env_ref,
			stage_ref, expected_graph_revision, committed_graph_revision, recorded_at
		FROM typed_memory_type_env_activations
		WHERE project_id = ? AND event_ref = ? AND change_ordinal = 0`,
		[]any{graph.request.Project().String(), graph.identity.eventRef},
		[]any{
			&activationRef,
			&activationDigest,
			&activationBytes,
			&requestRef,
			&requestDigest,
			&basisRef,
			&resultRef,
			&stageRef,
			&expectedRevision,
			&committedRevision,
			&storedRecordedAt,
		},
	)
	if err != nil {
		return fmt.Errorf("load interleaved ProjectTypeEnv activation row: %w", err)
	}
	matches := activationRef == delta.Ref().String() &&
		activationDigest == delta.Digest().String() &&
		bytes.Equal(activationBytes, delta.CanonicalBytes()) &&
		requestRef == graph.request.Ref().String() &&
		requestDigest == graph.request.Ref().Digest().String() &&
		basisRef == graph.basisTypeEnv.String() &&
		resultRef == graph.request.Target().VerifiedComposite().String() &&
		stageRef == graph.request.Target().Stage().String() &&
		expectedRevision == graph.identity.expectedSQLiteRevision &&
		committedRevision == graph.identity.nextSQLiteRevision &&
		storedRecordedAt == recordedAt
	if !matches {
		return storedAdmissionIntegrity("interleaved ProjectTypeEnv activation row", nil)
	}
	var historyCount int64
	var receiptCount int64
	var closureCount int64
	err = transaction.ScanOne(
		ctx,
		`SELECT
			(SELECT COUNT(*) FROM project_typeenv_head_history history
			 WHERE history.project_id = ? AND history.graph_event_ref = ?
			 AND history.recorded_at = ?),
			(SELECT COUNT(*) FROM project_typeenv_head_selection_receipts receipt
			 WHERE receipt.project_id = ? AND receipt.graph_event_ref = ?
			 AND receipt.recorded_at = ?),
			(SELECT COUNT(*) FROM project_typeenv_head_selection_closures closure
			 WHERE closure.project_id = ? AND closure.graph_event_ref = ?
			 AND closure.recorded_at = ?)`,
		[]any{
			graph.request.Project().String(),
			graph.identity.eventRef,
			recordedAt,
			graph.request.Project().String(),
			graph.identity.eventRef,
			recordedAt,
			graph.request.Project().String(),
			graph.identity.eventRef,
			recordedAt,
		},
		[]any{&historyCount, &receiptCount, &closureCount},
	)
	if err != nil {
		return fmt.Errorf("inspect interleaved ProjectTypeEnv effect closure: %w", err)
	}
	if historyCount != 1 || receiptCount != 1 || closureCount != 1 {
		return storedAdmissionIntegrity("interleaved ProjectTypeEnv effect closure", nil)
	}
	return nil
}

func verifyActivationObservation(
	prepared PreparedProjectTypeEnvActivationGraph,
	observation CurrentProjectGraphObservation,
) error {
	if err := observation.Verify(); err != nil {
		return err
	}
	basis := observation.GraphSnapshotBasis()
	closure, ok := basis.Closure().(projecttypeenvselection.CommittedProjectGraphClosure)
	matches := ok &&
		basis.Project() == prepared.graph.request.Project() &&
		basis.GraphRevision() == prepared.graph.identity.nextRevision &&
		observation.ActiveTypeEnv() == prepared.graph.request.Target().VerifiedComposite() &&
		closure.Event() == prepared.graph.event &&
		closure.Commit() == prepared.graph.commit &&
		closure.MaterializationDigest() == prepared.materializationDigest
	if !matches {
		return storedAdmissionIntegrity("committed ProjectTypeEnv activation graph observation", nil)
	}
	return nil
}
