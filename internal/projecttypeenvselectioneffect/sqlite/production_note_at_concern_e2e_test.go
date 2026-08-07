package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type productionNoteUnavailableObservableProvider struct{}

func (productionNoteUnavailableObservableProvider) LoadObservableInput(
	context.Context,
	projectidentity.ProjectID,
	typedmemory.ObservableInputRef,
	typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
		"production Note fixture has no unstaged observable input",
	)
}

func TestProductionNoteAtConcernAdmitsReplaysAndRereadsExactTypedMemory(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	fresh, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("production TypeEnv selection = %T, want FreshlyCommitted", selection)
	}
	if fresh.Closure().Target().Composite() != fixture.target.snapshot.TypeEnvRef() {
		t.Fatal("production TypeEnv selection did not activate the exact prepared C")
	}
	selectedRevision := fresh.Closure().CommittedGraphRevision()

	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseLoader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		projectmemory.NewBaseTypeEnvLoader(),
		resolver,
	)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 17, 13, 0, 0, 0, time.UTC),
	}
	baseAdapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		productionNoteUnavailableObservableProvider{},
	)
	baseSource := newGenesisE2ECurrentProjectBasisSource(t, baseLoader)
	baseRuntime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		baseSource,
		baseAdapter,
	)
	mustProductionNoteNoError(t, err)
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	concern := productionNoteConcernDeclaration(t, contextRef)
	concernCandidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{concern},
	)
	mustProductionNoteNoError(t, err)
	concernValid, err := baseRuntime.PrepareCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		concernCandidate,
	)
	mustProductionNoteNoError(t, err)
	concernKey, err := typedmemorystore.NewIdempotencyKey(
		"production-note-concern-admission",
	)
	mustProductionNoteNoError(t, err)
	concernReceipt, err := baseRuntime.AdmitValidated(
		ctx,
		concernValid,
		concernKey,
		concern.Provenance(),
	)
	mustProductionNoteNoError(t, err)
	if concernReceipt.Disposition() != typedmemorystore.CommitApplied ||
		concernReceipt.GraphRevision().Value() != selectedRevision.Value()+1 {
		t.Fatalf(
			"concern admission = %s/revision %d, want applied/revision %d",
			concernReceipt.Disposition(),
			concernReceipt.GraphRevision().Value(),
			selectedRevision.Value()+1,
		)
	}

	current, err := baseLoader.LoadCurrentProjectSnapshot(ctx, fixture.project)
	mustProductionNoteNoError(t, err)
	if current.Snapshot().GraphRevision() != concernReceipt.GraphRevision() {
		t.Fatal("current snapshot did not observe the persisted EntityOfConcern")
	}
	concernBinding := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	exactRuntime := productionNoteExactRuntime(t, fixture, current)
	draft, recordEntity, assertionID := productionNoteDraft(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
	)
	adapted := noteadapter.Adapt(draft, exactRuntime, concernBinding)
	noteCandidate, ok := adapted.(noteadapter.ValidCandidate)
	if !ok {
		t.Fatalf("Note adapter result = %T, want ValidCandidate", adapted)
	}
	stage, err := noteadapter.SealPreAdmissionSourceStage(noteCandidate)
	mustProductionNoteNoError(t, err)
	overlayLoader, err := typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
		baseLoader,
		stage,
	)
	mustProductionNoteNoError(t, err)
	noteSource := newGenesisE2ECurrentProjectBasisSource(t, overlayLoader)
	noteAdapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		stage,
	)
	noteRuntime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		noteSource,
		noteAdapter,
	)
	mustProductionNoteNoError(t, err)
	validationRuntime, err := projectmemory.NewValidationRuntime(
		fixture.project,
		noteSource,
	)
	mustProductionNoteNoError(t, err)
	noteOutcome, err := validationRuntime.EvaluateCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		noteCandidate.ChangeSet(),
	)
	mustProductionNoteNoError(t, err)
	noteValid, ok := noteOutcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		t.Fatalf(
			"production Note validation = %T/%s diagnostics=%#v",
			noteOutcome,
			noteOutcome.Verdict(),
			noteOutcome.Diagnostics(),
		)
	}
	assertProductionNoteAdmissionBasis(
		t,
		noteValid.AdmissionBasis(),
		current.Environment(),
		recordEntity,
		contextRef,
	)
	validatedAssertion := productionNoteValidatedAssertion(t, noteValid)

	noteKey, err := typedmemorystore.NewIdempotencyKey(
		"production-note-at-concern-admission",
	)
	mustProductionNoteNoError(t, err)
	noteProvenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-note-at-concern-admission",
	)
	mustProductionNoteNoError(t, err)
	receipt, err := noteRuntime.AdmitValidated(
		ctx,
		noteValid,
		noteKey,
		noteProvenance,
	)
	mustProductionNoteNoError(t, err)
	if receipt.Disposition() != typedmemorystore.CommitApplied ||
		receipt.GraphRevision().Value() != selectedRevision.Value()+2 {
		t.Fatalf(
			"Note admission = %s/revision %d, want applied/revision %d",
			receipt.Disposition(),
			receipt.GraphRevision().Value(),
			selectedRevision.Value()+2,
		)
	}
	replay, err := noteRuntime.AdmitValidated(
		ctx,
		noteValid,
		noteKey,
		noteProvenance,
	)
	mustProductionNoteNoError(t, err)
	if replay.Disposition() != typedmemorystore.CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.GraphRevision() != receipt.GraphRevision() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("Note replay = %#v, want exact replay of %#v", replay, receipt)
	}

	stored, err := noteAdapter.LoadEntity(ctx, fixture.project, recordEntity)
	mustProductionNoteNoError(t, err)
	if stored.Project() != fixture.project ||
		stored.Entity() != recordEntity ||
		stored.Context() != contextRef ||
		stored.Label().String() != "Production Note at concern" ||
		stored.Provenance() != validatedAssertion.Provenance() ||
		stored.Revision() != receipt.GraphRevision() {
		t.Fatalf("durable Note record = %#v, want exact admitted record", stored)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	mustProductionNoteNoError(t, err)
	observation, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	mustProductionNoteNoError(t, err)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit production Note durable read: %v", finish.Err())
	}
	relations := observation.ActiveAssertions().Relations()
	if len(relations) != 1 {
		t.Fatalf("durable active relations = %d, want exactly 1", len(relations))
	}
	durable := relations[0]
	carrier := productionFreshCurrentAssertionCarrier(t, durable)
	expectedBytes, err := validatedAssertion.CanonicalBytes()
	mustProductionNoteNoError(t, err)
	expectedDigest, err := validatedAssertion.Digest()
	mustProductionNoteNoError(t, err)
	if durable.AssertionID() != assertionID ||
		carrier.Signature() != validatedAssertion.Signature() ||
		!bytes.Equal(durable.CanonicalBytes(), expectedBytes) ||
		durable.Digest() != expectedDigest {
		t.Fatal("durable Note assertion differs from the exact validated assertion")
	}
}

func newProductionNoteSelectedFixture(t *testing.T) genesisE2EFixture {
	t.Helper()
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	database := harness.Database()
	project, err := projectidentity.ParseProjectID(harness.ProjectID())
	mustProductionNoteNoError(t, err)
	admission := harness.AdmitSoftwareRevision(
		t,
		"production-note-selected-e-e2e",
	)
	currentProfile, err := declaredProjectProfileBasis(admission)
	mustProductionNoteNoError(t, err)
	target := newProductionNoteE2ETarget(t)
	stageStore, err := projecttypeenvstage.New(ctx, database)
	mustProductionNoteNoError(t, err)
	mustProductionNoteNoError(
		t,
		stageStore.PutArtifactClosure(ctx, target.closure),
	)
	baseRef, ok := target.closure.Base().TypeEnvRef()
	if !ok {
		t.Fatal("production Note B has no executable TypeEnv reference")
	}
	clock := genesisE2EClock{
		value: time.Date(2026, 7, 17, 12, 30, 0, 0, time.UTC),
	}
	seedGenesisE2EBaseSnapshot(
		t,
		database,
		target.closure.Base(),
		clock.Now(),
	)
	seedGenesisE2EGraphHead(t, database, project, baseRef, clock.Now())
	stage := newGenesisE2EStage(
		t,
		database,
		project,
		currentProfile,
		target,
	)
	mustProductionNoteNoError(
		t,
		stageStore.Put(
			ctx,
			stage,
			target.verification.Record(),
			target.snapshot.Record(),
		),
	)
	ready, err := stageStore.LoadSelectionReady(ctx, stage.Ref())
	mustProductionNoteNoError(t, err)
	assertGenesisE2EStageCurrent(
		t,
		database,
		project,
		currentProfile,
		ready,
		target.registry,
	)
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		"production-note-selected-e-e2e-key",
	)
	mustProductionNoteNoError(t, err)
	request, err := projecttypeenvselection.SealGenesisProjectTypeEnvHeadSelectionRequest(
		projecttypeenvselection.GenesisProjectTypeEnvHeadSelectionRequestInput{
			Project:               project,
			Stage:                 stage,
			ExpectedGraphRevision: typedmemory.NewGraphRevision(0),
			IdempotencyKey:        key,
		},
	)
	mustProductionNoteNoError(t, err)
	description, err := authority.NewClaimIDDescriptionRef(
		"claim:project-typeenv-head-selection:production-note-e2e",
	)
	mustProductionNoteNoError(t, err)
	judgementContext, err := authority.NewBoundedContextRef(
		"bounded-context:project-typeenv-head-selection",
	)
	mustProductionNoteNoError(t, err)
	validity, err := authority.NewTimeWindow(
		clock.Now().Add(-time.Hour),
		clock.Now().Add(time.Hour),
	)
	mustProductionNoteNoError(t, err)
	content, err := projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
		projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
			DescriptionRef:   description,
			Request:          request,
			Stage:            stage,
			JudgementContext: judgementContext,
			ValidityWindow:   validity,
		},
	)
	mustProductionNoteNoError(t, err)
	service, err := NewGenesisService(
		ctx,
		database,
		harness.Root().String(),
		target.installed,
		&clock,
	)
	mustProductionNoteNoError(t, err)
	return genesisE2EFixture{
		database: database,
		project:  project,
		target:   target,
		stage:    stage,
		request:  request,
		content:  content,
		service:  service,
	}
}

func newProductionNoteCommitAdapter(
	t *testing.T,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	clock typedmemorystore.Clock,
	provider typedmemorystore.ObservableInputContentProvider,
) *typedmemorystore.SQLiteAdapter {
	t.Helper()
	adapter, err := typedmemorystore.NewProjectExecutableGenericSQLiteAdapterBuilder(
		fixture.database,
	).
		SetTypeEnvLoader(projectmemory.NewBaseTypeEnvLoader()).
		SetClock(clock).
		SetReferenceEngine(typedmemorystore.NewExactPersistedStrongReferenceEngine()).
		SetObservableInputs(provider).
		SetSelectedProjectRuntime(resolver).
		Build()
	mustProductionNoteNoError(t, err)
	return adapter
}

func productionNoteConcernDeclaration(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.DeclareEntity {
	t.Helper()
	entity, err := typedmemory.NewEntityID("entity:production-note-concern")
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef("local:production-note-concern")
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel("Production Note concern")
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-note-concern",
	)
	mustProductionNoteNoError(t, err)
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		local,
		contextRef,
		label,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	return declaration
}

func productionNoteConcernBinding(
	t *testing.T,
	current typedmemorystore.CurrentProjectSnapshot,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) noteadapter.ExactConcernBinding {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	mustProductionNoteNoError(t, err)
	refKind, err := typedmemory.NewRefKindRef(
		current.Environment().Ref(),
		refKindID,
	)
	mustProductionNoteNoError(t, err)
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	mustProductionNoteNoError(t, err)
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	mustProductionNoteNoError(t, err)
	resolution := current.Snapshot().ResolveReference(reference, contextRef)
	resolved, ok := resolution.(typedmemory.ResolvedStrongReference)
	if !ok {
		t.Fatalf("persisted EntityOfConcern resolution = %T, want ResolvedStrongReference", resolution)
	}
	binding, err := noteadapter.NewExactConcernBinding(resolved)
	mustProductionNoteNoError(t, err)
	return binding
}

func productionNoteExactRuntime(
	t *testing.T,
	fixture genesisE2EFixture,
	current typedmemorystore.CurrentProjectSnapshot,
) noteadapter.ExactRuntimeBasis {
	t.Helper()
	runtimeDigest, err := typedmemorystore.NewSelectedRuntimeBasisDigest(
		fixture.target.runtime.Digest(),
	)
	mustProductionNoteNoError(t, err)
	coordinate, found := fixture.target.registry.CoordinateDigest()
	if !found {
		t.Fatal("production Note selected runtime has no exact registry coordinate")
	}
	registryDigest, err := typedmemorystore.NewExactTargetRegistryCoordinateDigest(
		coordinate,
	)
	mustProductionNoteNoError(t, err)
	registration := productionPolicyForRule(
		t,
		fixture.target.installed.RegistrationPolicies,
		recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
	)
	runtime, err := noteadapter.NewExactRuntimeBasisBuilder(fixture.project).
		SetGraphRevision(current.Snapshot().GraphRevision()).
		SetEnvironment(current.Environment()).
		SetCodecs(current.Codecs()).
		SetSelectedRuntimeCoordinates(runtimeDigest, registryDigest).
		SetRegistrationPolicy(registration).
		Build()
	mustProductionNoteNoError(t, err)
	return runtime
}

func productionNoteDraft(
	t *testing.T,
	project projectidentity.ProjectID,
	environment typedmemory.TypeEnv,
	contextRef typedmemory.BoundedContextRef,
) (noteadapter.Draft, typedmemory.EntityID, typedmemory.AssertionID) {
	t.Helper()
	textKindID, err := typedmemory.NewKindID("Haft.Text")
	mustProductionNoteNoError(t, err)
	textKind, err := typedmemory.NewValueKindRef(environment.Ref(), textKindID)
	mustProductionNoteNoError(t, err)
	firstID, err := typedmemory.NewClaimNodeID("claim:production-note-current")
	mustProductionNoteNoError(t, err)
	first, err := typedmemory.NewClaimNode(
		firstID,
		textKind,
		typedmemory.NewTextValue("The Note is attached to one exact EntityOfConcern"),
	)
	mustProductionNoteNoError(t, err)
	secondID, err := typedmemory.NewClaimNodeID("claim:production-note-next")
	mustProductionNoteNoError(t, err)
	second, err := typedmemory.NewClaimNode(
		secondID,
		textKind,
		typedmemory.NewTextValue("The durable graph retains the typed relation"),
	)
	mustProductionNoteNoError(t, err)
	graph, err := typedmemory.NewClaimGraphValue(
		[]typedmemory.ClaimNode{first, second},
		nil,
	)
	mustProductionNoteNoError(t, err)
	exactGraph, err := noteadapter.NewExactClaimGraph(graph)
	mustProductionNoteNoError(t, err)
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 13, 5, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   contextRef,
		GammaTime: gamma,
	})
	mustProductionNoteNoError(t, err)
	recordEntity, err := typedmemory.NewEntityID("record:production-note-1")
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef("record:production-note-1")
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel("Production Note at concern")
	mustProductionNoteNoError(t, err)
	assertion, err := typedmemory.NewAssertionID(
		"assertion:production-note-1-at-concern",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-note-at-concern",
	)
	mustProductionNoteNoError(t, err)
	draft, err := noteadapter.NewDraft(noteadapter.DraftInput{
		ProjectID:      project,
		RecordEntity:   recordEntity,
		RecordLocalRef: local,
		RecordLabel:    label,
		AssertionID:    assertion,
		ContextSlice:   contextSlice,
		ClaimGraph:     exactGraph,
		Provenance:     provenance,
	})
	mustProductionNoteNoError(t, err)
	return draft, recordEntity, assertion
}

func assertProductionNoteAdmissionBasis(
	t *testing.T,
	admission typedmemory.AdmissionBasis,
	environment typedmemory.TypeEnv,
	recordEntity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) {
	t.Helper()
	basis, ok := admission.(typedmemory.ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf("production Note admission basis = %T, want ContextSliceMembershipBasis", admission)
	}
	var noteUse typedmemory.ReferenceFillerAdmissionUse
	for _, use := range basis.ReferenceFillerAdmissionUses() {
		if use.Coordinate().Slot().String() == "Haft.NoteAtConcern.NoteSlot" {
			noteUse = use
			break
		}
	}
	if noteUse == nil {
		t.Fatal("production Note admission basis omitted the NoteSlot reference use")
	}
	required := noteUse.RequiredMembership()
	if required.Query().EntityID() != recordEntity ||
		required.Query().ContextSlice().Context() != contextRef {
		t.Fatal("NoteSlot required membership lost the record Entity or ContextSlice")
	}
	if _, ok := required.EvaluationView().(typedmemory.ProspectiveBatchView); !ok {
		t.Fatalf("NoteSlot evaluation view = %T, want ProspectiveBatchView", required.EvaluationView())
	}
	posture, ok := required.Basis().Posture().(typedmemory.C32PrerequisiteMemberOfBasisV3)
	if !ok {
		t.Fatalf("NoteSlot MemberOf basis = %T, want C.3.2 prerequisite v3", required.Basis().Posture())
	}
	if _, ok := posture.Certificate().CandidateVisibility().(typedmemory.C32ProspectiveVisibilityCoordinate); !ok {
		t.Fatalf(
			"NoteSlot candidate visibility = %T, want prospective C.3.2 coordinate",
			posture.Certificate().CandidateVisibility(),
		)
	}
	want := map[string]string{
		"Haft.Constraint.ProjectRecordCarrierEditionDisjointV1":          "Haft.CarrierEdition",
		"Haft.Constraint.ProjectRecordPerformedWorkOccurrenceDisjointV1": "Haft.PerformedWorkOccurrence",
		"Haft.Constraint.ProjectRecordCodeAnchorDisjointV1":              "Haft.CodeAnchor",
	}
	disjoint := noteUse.DisjointMemberships()
	if len(disjoint) != len(want) {
		t.Fatalf("NoteSlot disjoint uses = %d, want exactly %d", len(disjoint), len(want))
	}
	for _, counter := range disjoint {
		proof, ok := counter.(typedmemory.DisjointEntailmentUse)
		if !ok || counter.Kind() != typedmemory.EntailedDisjointCounterUse {
			t.Fatalf("NoteSlot disjoint use = %T/%s, want DisjointEntailmentUse", counter, counter.Kind())
		}
		excluded, found := want[proof.Constraint().String()]
		if !found {
			t.Fatalf("unexpected ProjectRecord disjoint constraint %q", proof.Constraint().String())
		}
		delete(want, proof.Constraint().String())
		if proof.MatchedOperand().String() != "Haft.ProjectRecord" ||
			proof.ExcludedOperand().String() != excluded ||
			proof.SupportingMembership().Digest() != required.Digest() ||
			!bytes.Equal(
				proof.SupportingMembership().CanonicalBytes(),
				required.CanonicalBytes(),
			) ||
			proof.CounterQuery().EntityID() != recordEntity ||
			proof.CounterQuery().ContextSlice().Context() != contextRef {
			t.Fatalf("disjoint entailment for %q lost its exact support or operands", proof.Constraint())
		}
		exact := productionNoteConstraint(t, environment, proof.Constraint())
		digest := sha256.Sum256(exact.CanonicalBytes())
		expectedDigest, err := typedmemory.NewSHA256Digest(fmt.Sprintf("sha256:%x", digest[:]))
		mustProductionNoteNoError(t, err)
		if !bytes.Equal(proof.ConstraintRule().CanonicalBytes(), exact.CanonicalBytes()) ||
			proof.ConstraintDigest() != expectedDigest {
			t.Fatalf("disjoint entailment for %q lost the exact TypeEnv constraint", proof.Constraint())
		}
	}
	if len(want) != 0 {
		t.Fatalf("production Note admission omitted disjoint constraints: %#v", want)
	}
}

func productionNoteConstraint(
	t *testing.T,
	environment typedmemory.TypeEnv,
	id typedmemory.ConstraintID,
) typedmemory.KindDisjointConstraint {
	t.Helper()
	for _, rule := range environment.Constraints() {
		constraint, ok := rule.(typedmemory.KindDisjointConstraint)
		if ok && constraint.ID() == id {
			return constraint
		}
	}
	t.Fatalf("selected production TypeEnv omitted KindDisjoint %q", id.String())
	return typedmemory.KindDisjointConstraint{}
}

func productionNoteValidatedAssertion(
	t *testing.T,
	valid typedmemoryvalidation.ValidOutcome,
) typedmemory.RelationalAssertion {
	t.Helper()
	for _, change := range valid.AdmissionBatch().ChangeSet().Changes() {
		if assertion, ok := change.(typedmemory.ValidatedRelationalAssertion); ok {
			return assertion.Assertion()
		}
	}
	t.Fatal("production Note validated change set omitted the relational assertion")
	return typedmemory.RelationalAssertion{}
}

func TestProductionNoteAtConcernSelectedRuntimeFixtureIsExecutable(t *testing.T) {
	t.Parallel()

	target := newProductionNoteE2ETarget(t)
	memberOf, found := target.registry.MemberOfRegistry()
	if !found || memberOf.Len() != 6 {
		t.Fatalf("production selected runtime MemberOf families = %d, found=%v; want 6", memberOf.Len(), found)
	}
	actual := make(map[string]struct{}, memberOf.Len())
	for _, registration := range memberOf.Registrations() {
		actual[registration.RuleRef().String()] = struct{}{}
	}
	for _, rule := range productionMemberOfRules(t) {
		if _, ok := actual[rule.String()]; !ok {
			t.Fatalf("production selected runtime omitted MemberOf family %q", rule.String())
		}
		productionPolicyForRule(t, target.installed.RegistrationPolicies, rule)
	}
}

func TestProductionProjectEntityMemberOfUsesPersistedC32Prerequisites(
	t *testing.T,
) {
	t.Parallel()

	production := newProductionLocalPracticeETarget(t)
	environment, ok := production.preparation.Environment()
	if !ok {
		t.Fatal("prepared production target has no executable environment")
	}
	installed := productionNoteInstalledRuntime(t, production, environment)
	rule, err := typedmemory.NewRuleRef("haft.member-of.project-entity/v1")
	mustProductionNoteNoError(t, err)
	lookup, err := installed.MemberOfEvaluators.Lookup(
		rule,
		productionNoteMechanismIdentity(t, production.mechanism),
	)
	mustProductionNoteNoError(t, err)
	found, ok := lookup.(memberofruntime.Found)
	if !ok {
		t.Fatalf("project-entity evaluator lookup = %T, want Found", lookup)
	}
	project, err := projectidentity.ParseProjectID("qnt_a71e9e22")
	mustProductionNoteNoError(t, err)
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   contextRef,
		GammaTime: gamma,
	})
	mustProductionNoteNoError(t, err)
	kindID, err := typedmemory.NewKindID("U.Entity")
	mustProductionNoteNoError(t, err)
	valueKind, err := typedmemory.NewValueKindRef(environment.Ref(), kindID)
	mustProductionNoteNoError(t, err)
	concern, err := typedmemory.NewEntityID("entity:production-note-concern")
	mustProductionNoteNoError(t, err)
	query, err := typedmemory.NewMemberOfQuery(concern, valueKind, contextSlice)
	mustProductionNoteNoError(t, err)
	revision := typedmemory.NewGraphRevision(2)
	view, err := typedmemory.NewPersistedSnapshotView(environment.Ref(), revision)
	mustProductionNoteNoError(t, err)
	request, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	mustProductionNoteNoError(t, err)
	universe, err := memberofevaluation.NewExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		[]typedmemory.EntityID{concern},
	)
	mustProductionNoteNoError(t, err)
	blob, err := universe.ObservableBlob()
	mustProductionNoteNoError(t, err)
	input, err := memberofevaluation.NewMemberOfEvaluationInput(
		project,
		environment,
		request,
		[]memberofevaluation.ObservableInputBlob{blob},
		universe,
	)
	mustProductionNoteNoError(t, err)
	engine := found.Registration().Engine()
	selector, ok := engine.(memberofevaluation.SnapshotObservableInputSelector)
	if !ok {
		t.Fatalf("project-entity engine = %T, want snapshot selector", engine)
	}
	selection := selector.SelectSnapshotObservableInputs(input)
	selected, ok := selection.(memberofevaluation.SnapshotObservableInputsSelected)
	if !ok {
		t.Fatalf("project-entity snapshot source selection = %T, want selected", selection)
	}
	selectedBlobs := selected.ObservableInputs()
	if len(selectedBlobs) != 1 ||
		selectedBlobs[0].Reference() != blob.Reference() ||
		selectedBlobs[0].Digest() != blob.Digest() {
		t.Fatal("project-entity selector did not retain the exact persisted-universe coordinate")
	}
	judgement, err := engine.EvaluateMemberOf(
		context.Background(),
		input,
	)
	mustProductionNoteNoError(t, err)
	member, ok := judgement.(typedmemory.MemberOfMember)
	if !ok {
		t.Fatalf("project-entity judgement = %T, want MemberOfMember", judgement)
	}
	if member.Query().EntityID() != concern {
		t.Fatal("project-entity judgement changed the persisted concern identity")
	}
	posture, ok := member.Basis().Posture().(typedmemory.C32PrerequisiteMemberOfBasisV3)
	if !ok {
		t.Fatalf("project-entity basis posture = %T, want C.3.2 v3", member.Basis().Posture())
	}
	if _, ok := posture.Certificate().CandidateVisibility().(typedmemory.C32PersistedVisibilityCoordinate); !ok {
		t.Fatalf(
			"project-entity candidate visibility = %T, want persisted coordinate",
			posture.Certificate().CandidateVisibility(),
		)
	}
	absentEntity, err := typedmemory.NewEntityID("entity:absent-production-note-concern")
	mustProductionNoteNoError(t, err)
	absentQuery, err := typedmemory.NewMemberOfQuery(absentEntity, valueKind, contextSlice)
	mustProductionNoteNoError(t, err)
	absentRequest, err := typedmemory.NewMemberOfEvaluationRequest(absentQuery, view)
	mustProductionNoteNoError(t, err)
	absentInput, err := memberofevaluation.NewMemberOfEvaluationInput(
		project,
		environment,
		absentRequest,
		[]memberofevaluation.ObservableInputBlob{blob},
		universe,
	)
	mustProductionNoteNoError(t, err)
	absentJudgement, err := found.Registration().Engine().EvaluateMemberOf(
		context.Background(),
		absentInput,
	)
	mustProductionNoteNoError(t, err)
	if _, ok := absentJudgement.(typedmemory.MemberOfUndefined); !ok {
		t.Fatalf("absent project-entity judgement = %T, want fail-closed MemberOfUndefined", absentJudgement)
	}
}

func TestProductionProjectEntityMemberOfUsesProspectiveVisibilityCertificate(
	t *testing.T,
) {
	t.Parallel()

	production := newProductionLocalPracticeETarget(t)
	environment, ok := production.preparation.Environment()
	if !ok {
		t.Fatal("prepared production target has no executable environment")
	}
	installed := productionNoteInstalledRuntime(t, production, environment)
	rule, err := typedmemory.NewRuleRef("haft.member-of.project-entity/v1")
	mustProductionNoteNoError(t, err)
	lookup, err := installed.MemberOfEvaluators.Lookup(
		rule,
		productionNoteMechanismIdentity(t, production.mechanism),
	)
	mustProductionNoteNoError(t, err)
	found, ok := lookup.(memberofruntime.Found)
	if !ok {
		t.Fatalf("project-entity evaluator lookup = %T, want Found", lookup)
	}
	project, err := projectidentity.ParseProjectID("qnt_b71e9e22")
	mustProductionNoteNoError(t, err)
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 12, 5, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   contextRef,
		GammaTime: gamma,
	})
	mustProductionNoteNoError(t, err)
	kindID, err := typedmemory.NewKindID("U.Entity")
	mustProductionNoteNoError(t, err)
	valueKind, err := typedmemory.NewValueKindRef(environment.Ref(), kindID)
	mustProductionNoteNoError(t, err)
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	mustProductionNoteNoError(t, err)
	refKind, err := typedmemory.NewRefKindRef(environment.Ref(), refKindID)
	mustProductionNoteNoError(t, err)
	entity, err := typedmemory.NewEntityID("entity:prospective-project-entity")
	mustProductionNoteNoError(t, err)
	localID, err := typedmemory.NewBatchLocalRef("local:prospective-project-entity")
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel("Prospective project entity")
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef("memory:test:prospective-project-entity")
	mustProductionNoteNoError(t, err)
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		localID,
		contextRef,
		label,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	changeSet, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration},
	)
	mustProductionNoteNoError(t, err)
	prefix, err := typedmemory.ComputeOrderedCandidatePrefix(changeSet, 1)
	mustProductionNoteNoError(t, err)
	localRef, err := typedmemory.NewLocalRef(refKind, localID)
	mustProductionNoteNoError(t, err)
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	mustProductionNoteNoError(t, err)
	persistedRef, err := typedmemory.NewPersistedRef(
		refKind,
		referenceID,
	)
	mustProductionNoteNoError(t, err)
	revision := typedmemory.NewGraphRevision(2)
	view, err := typedmemory.NewProspectiveBatchView(
		typedmemory.ProspectiveBatchViewInput{
			TypeEnv:                  environment.Ref(),
			PreStateGraphRevision:    revision,
			EvaluationChangeOrdinal:  1,
			DeclarationChangeOrdinal: 0,
			Declaration:              declaration,
			LocalReference:           localRef,
			PersistedReference:       persistedRef,
			OrderedCandidatePrefix:   prefix,
		},
	)
	mustProductionNoteNoError(t, err)
	query, err := typedmemory.NewMemberOfQuery(entity, valueKind, contextSlice)
	mustProductionNoteNoError(t, err)
	request, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	mustProductionNoteNoError(t, err)
	universe, err := memberofevaluation.NewExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		nil,
	)
	mustProductionNoteNoError(t, err)
	blob, err := universe.ObservableBlob()
	mustProductionNoteNoError(t, err)
	input, err := memberofevaluation.NewMemberOfEvaluationInput(
		project,
		environment,
		request,
		[]memberofevaluation.ObservableInputBlob{blob},
		universe,
	)
	mustProductionNoteNoError(t, err)
	judgement, err := found.Registration().Engine().EvaluateMemberOf(
		context.Background(),
		input,
	)
	mustProductionNoteNoError(t, err)
	member, ok := judgement.(typedmemory.MemberOfMember)
	if !ok {
		t.Fatalf("prospective project-entity judgement = %T, want MemberOfMember", judgement)
	}
	posture, ok := member.Basis().Posture().(typedmemory.C32PrerequisiteMemberOfBasisV3)
	if !ok {
		t.Fatalf("prospective project-entity basis = %T, want C.3.2 v3", member.Basis().Posture())
	}
	if _, ok := posture.Certificate().CandidateVisibility().(typedmemory.C32ProspectiveVisibilityCoordinate); !ok {
		t.Fatalf(
			"prospective project-entity visibility = %T, want prospective coordinate",
			posture.Certificate().CandidateVisibility(),
		)
	}
}

func newProductionNoteE2ETarget(t *testing.T) genesisE2ETarget {
	t.Helper()
	production := newProductionLocalPracticeETarget(t)
	verification, ok := production.preparation.Verification()
	if !ok {
		t.Fatal("prepared production Note target has no final verification")
	}
	snapshot, ok := production.preparation.ExecutableSnapshot()
	if !ok {
		t.Fatal("prepared production Note target has no executable snapshot")
	}
	closure, err := projecttypeenvstore.PrepareArtifactClosureWithRuntimeClosure(
		production.base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{production.extension},
		production.runtime,
		production.composite,
		[]runtimemechanism.RuntimeMechanismArtifactV1{production.mechanism},
		production.policies,
	)
	if err != nil {
		t.Fatalf("prepare production Note B/E/X/C closure: %v", err)
	}
	installed := productionNoteInstalledRuntime(t, production, snapshot.Environment())
	observation := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: production.runtime,
			Installed:    installed,
		},
	)
	matched, ok := observation.(projecttypeenvruntime.Matched)
	if !ok {
		withIssues, _ := observation.(interface {
			Issues() []projecttypeenvruntime.Issue
		})
		issues := []projecttypeenvruntime.Issue(nil)
		if withIssues != nil {
			issues = withIssues.Issues()
		}
		t.Fatalf(
			"production Note installed runtime = %T (%s), want Matched; issues=%#v",
			observation,
			observation.Kind(),
			issues,
		)
	}
	registry, found := matched.Registry()
	if !found || !registry.Valid() {
		t.Fatal("production Note installed runtime exposed no exact registry")
	}
	return genesisE2ETarget{
		closure:      closure,
		verification: verification,
		snapshot:     snapshot,
		runtime:      production.runtime,
		mechanism:    production.mechanism,
		installed:    installed,
		registry:     registry,
	}
}

func productionNoteInstalledRuntime(
	t *testing.T,
	target productionLocalPracticeETarget,
	environment typedmemory.TypeEnv,
) projecttypeenvruntime.InstalledRuntimeRegistryInput {
	t.Helper()
	if environment.Ref() != target.composite.Ref() {
		t.Fatalf(
			"production installed runtime environment = %s, want target C %s",
			environment.Ref(),
			target.composite.Ref(),
		)
	}
	return target.installed
}

func productionCarrierFamilyEngine(
	t *testing.T,
	builder projectmemory.CarrierFamilyMembershipAdmissionEngineBuilder,
	rule typedmemory.RuleRef,
	policies []recordmembershipregistration.RegistrationArtifactV1,
	enumeration typedmemoryevaluation.EntitySetEnumerationRegistry,
	visibility typedmemoryevaluation.CandidateVisibilityRegistry,
	definedness typedmemoryevaluation.KindDefinednessRegistry,
	identity typedmemoryevaluation.MechanismIdentity,
) projectmemory.CarrierFamilyMembershipAdmissionEngine {
	t.Helper()
	engine, err := builder.
		SetEntitySetEnumeration(enumeration).
		SetCandidateVisibility(visibility).
		SetKindDefinedness(definedness).
		SetMechanismIdentity(identity).
		SetRegistrationPolicy(productionPolicyForRule(t, policies, rule)).
		Build()
	mustProductionNoteNoError(t, err)
	return engine
}

func productionPolicyForRule(
	t *testing.T,
	policies []recordmembershipregistration.RegistrationArtifactV1,
	rule typedmemory.RuleRef,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	var selected []recordmembershipregistration.RegistrationArtifactV1
	for _, policy := range policies {
		if policy.Evaluator().Rule() == rule {
			selected = append(selected, policy)
		}
	}
	if len(selected) != 1 {
		t.Fatalf(
			"production registration policies for %q = %d, want exactly 1",
			rule.String(),
			len(selected),
		)
	}
	return selected[0]
}

func productionMemberOfRules(t *testing.T) []typedmemory.RuleRef {
	t.Helper()
	projectEntity, err := typedmemory.NewRuleRef("haft.member-of.project-entity/v1")
	mustProductionNoteNoError(t, err)
	return []typedmemory.RuleRef{
		projectEntity,
		recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
		carrierfamily.CarrierEditionEvaluatorRuleV1(),
		carrierfamily.ProjectClaimEvaluatorRuleV1(),
		carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1(),
		carrierfamily.CodeAnchorEvaluatorRuleV1(),
	}
}

func productionNoteCodecRegistry(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	environment typedmemory.TypeEnv,
) typedmemory.CodecRegistry {
	t.Helper()
	_, baseCodecs, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		environment.Ref(),
	)
	if err != nil {
		t.Fatalf("lower production Note base codecs at C: %v", err)
	}
	suite, err := typedmemorycandidatecodec.NewSuite(environment.ValueShapes())
	if err != nil {
		t.Fatalf("construct production Note candidate codec suite: %v", err)
	}
	local := map[string]typedmemory.CodecImplementation{
		"Haft.Codec.TextV1":                 suite.Text(),
		"Haft.Codec.EvidencePolarityV1":     suite.EvidencePolarity(),
		"Haft.Codec.CanonicalInstantV1":     suite.CanonicalInstant(),
		"Haft.Codec.EvidenceUseQualifierV1": suite.EvidenceUseQualifier(),
		"Haft.Codec.PerformedIntervalV1":    suite.PerformedInterval(),
		"Haft.Codec.CodeAnchorLocatorV1":    suite.CodeAnchorLocator(),
	}
	result := typedmemory.NewCodecRegistry()
	for _, binding := range environment.ValueBindings() {
		implementation, found := baseCodecs.Resolve(binding.Codec())
		if !found {
			implementation, found = local[binding.Codec().ID().String()]
		}
		if !found {
			t.Fatalf("production Note codec %q has no installed implementation", binding.Codec().ID())
		}
		result, err = result.Register(binding.Codec(), implementation)
		if err != nil {
			t.Fatalf("register production Note codec %q: %v", binding.Codec().ID(), err)
		}
	}
	return result
}

func productionNoteMechanismIdentity(
	t *testing.T,
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	identity := mechanism.Identity()
	result, err := typedmemoryevaluation.NewMechanismIdentity(
		identity.Artifact(),
		identity.Edition(),
		identity.Digest(),
		typedmemoryevaluation.EvaluatorRole,
	)
	mustProductionNoteNoError(t, err)
	return result
}

func mustProductionNoteNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
