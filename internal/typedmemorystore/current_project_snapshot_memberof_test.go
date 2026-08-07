package typedmemorystore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestGenericCurrentProjectSnapshotEvaluatesMemberOfWithStaticRuntimeAndDurableCatalog(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	engine := snapshotSelectingExactBasisEngine{
		delegate:  fixture.adapter.memberOfEngine.(exactBasisMemberOfEngine),
		selection: snapshotObservableSelectionExact{blobs: fixture.observableBlobs},
	}
	fixture.adapter.memberOfEngine = engine
	request := fixture.request(t, "current-snapshot-memberof")
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	evaluationRequest := exactBasisCurrentSnapshotRequest(t, fixture)
	judgement := loaded.Snapshot().EvaluateMemberOf(evaluationRequest)
	member, ok := judgement.(typedmemory.MemberOfMember)
	if !ok {
		t.Fatalf("EvaluateMemberOf = %T; want MemberOfMember", judgement)
	}
	if !typedmemory.MemberOfJudgementMatchesRequest(evaluationRequest, member) {
		t.Fatal("current snapshot MemberOf judgement is not request-correlated")
	}
	inputs := member.Basis().ObservableInputs()
	if len(inputs) != len(fixture.observableBlobs) {
		t.Fatalf(
			"current snapshot observable inputs = %d; want %d",
			len(inputs),
			len(fixture.observableBlobs),
		)
	}
}

func TestCurrentProjectSnapshotOffersExactPersistedUniverseToSelectedEvaluator(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "current-snapshot-persisted-universe")
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	fixture.adapter.memberOfEngine = snapshotSelectingExactBasisEngine{
		delegate:  fixture.adapter.memberOfEngine.(exactBasisMemberOfEngine),
		selection: snapshotObservableSelectionPersistedUniverse{},
	}
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	evaluationRequest := exactBasisCurrentSnapshotRequest(t, fixture)
	judgement := loaded.Snapshot().EvaluateMemberOf(evaluationRequest)
	member, ok := judgement.(typedmemory.MemberOfMember)
	if !ok {
		t.Fatalf("persisted-universe snapshot judgement = %T; want MemberOfMember", judgement)
	}
	inputs := member.Basis().ObservableInputs()
	if len(inputs) != 1 ||
		!strings.HasPrefix(
			inputs[0].Reference().String(),
			"persisted-entity-universe:",
		) {
		t.Fatalf("snapshot selected observable inputs = %#v; want exact persisted universe", inputs)
	}
}

func TestProjectExecutableCurrentSnapshotUsesExactSelectedMemberOfRuntime(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	input := activationAdapterTestInputForProject(
		t,
		fixture.project.String(),
		fixture.snapshot.Ref(),
	)
	activateProjectExecutableForSnapshot(t, fixture, input)
	selected := input.Request.Target().VerifiedComposite()
	environment, valueKind, contextSlice := selectedSnapshotTypeEnv(
		t,
		selected,
		fixture,
	)
	engine := &recordingSelectedSnapshotMemberOfEngine{}
	runtimeBasisDigest, registryCoordinateDigest := selectedRuntimeTestCoordinates(
		t,
		selected.Digest(),
		fixture.environment.Ref().Digest(),
	)
	exact, err := NewExactMemberOfRuntime(ExactMemberOfRuntimeInput{
		Engine:                   engine,
		RuntimeBasisDigest:       runtimeBasisDigest,
		RegistryCoordinateDigest: registryCoordinateDigest,
	})
	if err != nil {
		t.Fatalf("NewExactMemberOfRuntime: %v", err)
	}
	resolver := &recordingSelectedProjectTypeEnvRuntimeResolver{
		environment: environment,
		codecs:      fixture.registry,
		memberOf:    exact,
	}
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	reader, err := NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		loader,
		resolver,
	)
	if err != nil {
		t.Fatalf("NewProjectAwareSQLiteCurrentProjectSnapshotLoader: %v", err)
	}
	loaded, err := reader.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	entity := mustGenericEntityID(t, "entity:selected-snapshot-query")
	query, err := typedmemory.NewMemberOfQuery(entity, valueKind, contextSlice)
	if err != nil {
		t.Fatalf("NewMemberOfQuery: %v", err)
	}
	view, err := typedmemory.NewPersistedSnapshotView(
		selected,
		typedmemory.NewGraphRevision(1),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView: %v", err)
	}
	evaluationRequest, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest: %v", err)
	}
	judgement := loaded.Snapshot().EvaluateMemberOf(evaluationRequest)
	if _, ok := judgement.(typedmemory.MemberOfUndefined); !ok {
		t.Fatalf("selected-C snapshot judgement = %T; want MemberOfUndefined", judgement)
	}
	if engine.selectionCalls != 1 {
		t.Fatalf("selected-C source-selection calls = %d; want 1", engine.selectionCalls)
	}
	if engine.evaluationCalls != 0 {
		t.Fatalf("selected-C evaluation calls = %d; want 0 without exact source", engine.evaluationCalls)
	}
	if resolver.calls != 1 {
		t.Fatalf("selected-C runtime resolver calls = %d; want 1", resolver.calls)
	}
}

func TestCurrentProjectSnapshotKeepsUnavailableSourceSelectionUndefined(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	fixture.adapter.memberOfEngine = snapshotSelectingExactBasisEngine{
		delegate:  fixture.adapter.memberOfEngine.(exactBasisMemberOfEngine),
		selection: snapshotObservableSelectionUnavailable{},
	}
	request := fixture.request(t, "current-snapshot-memberof-unavailable")
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	evaluationRequest := exactBasisCurrentSnapshotRequest(t, fixture)
	judgement := loaded.Snapshot().EvaluateMemberOf(evaluationRequest)
	undefined, ok := judgement.(typedmemory.MemberOfUndefined)
	if !ok {
		t.Fatalf("EvaluateMemberOf = %T; want MemberOfUndefined", judgement)
	}
	missing := undefined.MissingBasis()
	if len(missing) != 1 ||
		missing[0].Kind() != typedmemory.MissingMemberOfObservableInput {
		t.Fatalf(
			"unavailable selection missing basis = %#v; want observable input",
			missing,
		)
	}
	if missing[0].Subject() != "query:"+evaluationRequest.Query().Digest().String() {
		t.Fatalf(
			"unavailable selection subject = %q; want query digest",
			missing[0].Subject(),
		)
	}
}

func TestCurrentProjectSnapshotEvaluatesExplicitNotApplicableWithEmptySourceSet(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	engine := &snapshotNotApplicableMemberOfEngine{}
	request := fixture.request(t, "current-snapshot-memberof-not-applicable")
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	fixture.adapter.memberOfEngine = engine
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	evaluationRequest := exactBasisCurrentSnapshotRequest(t, fixture)
	judgement := loaded.Snapshot().EvaluateMemberOf(evaluationRequest)
	undefined, ok := judgement.(typedmemory.MemberOfUndefined)
	if !ok || !undefined.IsNoApplicableObservableSource() {
		t.Fatalf("not-applicable snapshot judgement = %T, want exact no-source Undefined", judgement)
	}
	if engine.evaluationCalls != 1 || engine.lastObservableCount != 0 {
		t.Fatalf(
			"not-applicable evaluator calls/source count = %d/%d, want 1/0",
			engine.evaluationCalls,
			engine.lastObservableCount,
		)
	}
}

func TestCurrentProjectSnapshotRejectsSelectorSourceInjection(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	_, injected := exactBasisObservable(
		t,
		"observable:outside-current-snapshot",
		[]byte("outside-current-snapshot"),
	)
	fixture.adapter.memberOfEngine = snapshotSelectingExactBasisEngine{
		delegate: fixture.adapter.memberOfEngine.(exactBasisMemberOfEngine),
		selection: snapshotObservableSelectionExact{blobs: []ObservableInputBlob{
			injected,
		}},
	}
	request := fixture.request(t, "current-snapshot-memberof-injection")
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	judgement := loaded.Snapshot().EvaluateMemberOf(
		exactBasisCurrentSnapshotRequest(t, fixture),
	)
	if _, ok := judgement.(typedmemory.MemberOfUndefined); !ok {
		t.Fatalf("injected source judgement = %T; want MemberOfUndefined", judgement)
	}
}

func TestImmutableObservableInputCatalogDeduplicatesExactReuseAndCopiesBytes(t *testing.T) {
	_, blob := exactBasisObservable(
		t,
		"observable:current-snapshot-catalog",
		[]byte("current-snapshot-catalog"),
	)
	catalog, err := newImmutableObservableInputCatalog([]ObservableInputBlob{blob, blob})
	if err != nil {
		t.Fatalf("newImmutableObservableInputCatalog: %v", err)
	}
	if catalog.Len() != 1 {
		t.Fatalf("deduplicated observable catalog size = %d; want 1", catalog.Len())
	}
	first := catalog.Blobs()
	firstBytes := first[0].Bytes()
	firstBytes[0] ^= 0xff
	second := catalog.Blobs()
	if string(second[0].Bytes()) != "current-snapshot-catalog" {
		t.Fatal("immutable observable catalog leaked mutable bytes")
	}
}

func TestDecodeStoredObservableInputRowRejectsCorruptContent(t *testing.T) {
	_, blob := exactBasisObservable(
		t,
		"observable:current-snapshot-corrupt",
		[]byte("current-snapshot-corrupt"),
	)
	row := storedObservableInputRow{
		EventRevision: 1,
		EventRef:      "event:current-snapshot-corrupt",
		Reference:     blob.Reference().String(),
		Digest:        blob.Digest().String(),
		CanonicalHex:  "00",
	}
	_, err := decodeStoredObservableInputRow(row)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("corrupt observable row error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func exactBasisCurrentSnapshotRequest(
	t *testing.T,
	fixture exactBasisStoreFixture,
) typedmemory.MemberOfEvaluationRequest {
	t.Helper()
	entity := mustGenericEntityID(t, "entity:exact-basis")
	query, err := typedmemory.NewMemberOfQuery(
		entity,
		fixture.entityKind,
		fixture.contextSlice,
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery: %v", err)
	}
	view, err := typedmemory.NewPersistedSnapshotView(
		fixture.environment.Ref(),
		typedmemory.NewGraphRevision(1),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView: %v", err)
	}
	request, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest: %v", err)
	}
	return request
}

func activateProjectExecutableForSnapshot(
	t *testing.T,
	fixture sqliteStoreFixture,
	input ProjectTypeEnvActivationGraphInput,
) {
	t.Helper()
	ctx := context.Background()
	prepared := prepareActivationIntegrationGraph(t, input)
	effect := newActivationIntegrationEffect(
		t,
		input,
		canonicalTime(fixture.clock.Now()),
	)
	installActivationIntegrationCandidate(t, fixture.database, input)
	stages, err := projecttypeenvstage.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New: %v", err)
	}
	adapter, err := NewProjectTypeEnvActivationAdapter(fixture.clock, stages)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvActivationAdapter: %v", err)
	}
	adapter.verifyStage = func(
		context.Context,
		*sqlitetransaction.Transaction,
		PreparedProjectTypeEnvActivationGraph,
	) error {
		return nil
	}
	heads, err := projecttypeenvheadstore.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvheadstore.New: %v", err)
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	insertActivationIntegrationSources(t, ctx, transaction, input, effect)
	successor := activationIntegrationSuccessor(t, input)
	if err := heads.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		successor,
	); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("CompareAndSwapGenesisProjectTypeEnvHeadTx: %v", err)
	}
	_, err = adapter.WritePreparedProjectTypeEnvActivationGraphTx(
		ctx,
		transaction,
		prepared,
		func(
			callbackContext context.Context,
			callbackTransaction *sqlitetransaction.Transaction,
			write ProjectTypeEnvActivationWriteContext,
		) error {
			return insertActivationIntegrationClosure(
				callbackContext,
				callbackTransaction,
				input,
				effect,
				successor,
				write,
			)
		},
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("WritePreparedProjectTypeEnvActivationGraphTx: %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit project-executable snapshot fixture: %v", finish.Err())
	}
}

func selectedSnapshotTypeEnv(
	t *testing.T,
	selected typedmemory.TypeEnvRef,
	fixture sqliteStoreFixture,
) (typedmemory.TypeEnv, typedmemory.ValueKindRef, typedmemory.ContextSlice) {
	t.Helper()
	boundedContext, found := fixture.environment.BoundedContext(fixture.context)
	if !found {
		t.Fatal("selected snapshot fixture context is missing")
	}
	provenance := mustFPFProvenance(t, fixture.snapshot.SourceRevision())
	kindID := mustGenericKindID(t, "U.SelectedSnapshotEntity")
	kindDefinition, err := typedmemory.NewKindDefinition(kindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition: %v", err)
	}
	valueKind, err := typedmemory.NewValueKindRef(selected, kindID)
	if err != nil {
		t.Fatalf("NewValueKindRef: %v", err)
	}
	availability := mustLocalContextKindAvailability(
		t,
		selected,
		fixture.context,
		kindID,
		provenance,
		"selected-snapshot.availability",
	)
	entitySet, err := typedmemory.NewEntitySetDefinition(
		typedmemory.EntitySetDefinitionInput{
			TypeEnv:         selected,
			Context:         fixture.context,
			EnumerationRule: exactBasisRuleRef(t, "test:selected-snapshot/enumeration/v1"),
			CandidatePolicy: typedmemory.PersistedEntitiesOnly{},
			Provenance:      provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewEntitySetDefinition: %v", err)
	}
	signature, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       valueKind,
			Formality:       typedmemory.SignatureF4,
			DefinednessRule: exactBasisRuleRef(t, "test:selected-snapshot/definedness/v1"),
			Evaluator:       exactBasisRuleRef(t, "test:selected-snapshot/memberof/v1"),
			EntitySet:       entitySet.Ref(),
			Provenance:      provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition: %v", err)
	}
	environment, err := typedmemory.NewTypeEnvBuilder(selected).
		SetSourceRevision(fixture.snapshot.SourceRevision()).
		SetCompilerSchemaVersion(fixture.snapshot.CompilerSchemaVersion()).
		SetCoverageManifest(fixture.environment.CoverageManifest()).
		AddBoundedContext(boundedContext).
		AddKindDefinition(kindDefinition).
		AddEntitySetDefinition(entitySet).
		AddKindSignatureDefinition(signature).
		AddContextKindAvailability(availability).
		Build()
	if err != nil {
		t.Fatalf("build selected snapshot TypeEnv: %v", err)
	}
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewGammaPoint: %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   fixture.context,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice: %v", err)
	}
	return environment, valueKind, contextSlice
}

type recordingSelectedSnapshotMemberOfEngine struct {
	selectionCalls  int
	evaluationCalls int
}

type snapshotNotApplicableMemberOfEngine struct {
	evaluationCalls     int
	lastObservableCount int
}

func (*snapshotNotApplicableMemberOfEngine) SelectSnapshotObservableInputs(
	MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	return NewSnapshotObservableInputsNotApplicable()
}

func (engine *snapshotNotApplicableMemberOfEngine) EvaluateMemberOf(
	_ context.Context,
	input MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	engine.evaluationCalls++
	engine.lastObservableCount = len(input.ObservableInputs())
	query := input.Request().Query()
	missing, err := typedmemory.NoApplicableObservableSourceForMemberOf(query)
	if err != nil {
		return nil, err
	}
	repair, err := typedmemory.NewRepairPointer(
		"repair:test/current-snapshot-no-applicable-source",
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewMemberOfUndefined(
		input.Request(),
		[]typedmemory.MemberOfMissingBasis{missing},
		repair,
	)
}

func (engine *recordingSelectedSnapshotMemberOfEngine) SelectSnapshotObservableInputs(
	MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	engine.selectionCalls++
	return NewSnapshotObservableInputsUnavailable()
}

func (engine *recordingSelectedSnapshotMemberOfEngine) EvaluateMemberOf(
	context.Context,
	MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	engine.evaluationCalls++
	return nil, errors.New("selected snapshot engine received no exact source")
}

type snapshotObservableSelection interface {
	selectSnapshotObservables(MemberOfEvaluationInput) SnapshotObservableInputSelection
}

type snapshotSelectingExactBasisEngine struct {
	delegate  exactBasisMemberOfEngine
	selection snapshotObservableSelection
}

func (engine snapshotSelectingExactBasisEngine) EvaluateMemberOf(
	ctx context.Context,
	input MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return engine.delegate.EvaluateMemberOf(ctx, input)
}

func (engine snapshotSelectingExactBasisEngine) SelectSnapshotObservableInputs(
	input MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	return engine.selection.selectSnapshotObservables(input)
}

type snapshotObservableSelectionUnavailable struct{}

func (snapshotObservableSelectionUnavailable) selectSnapshotObservables(
	MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	return NewSnapshotObservableInputsUnavailable()
}

type snapshotObservableSelectionPersistedUniverse struct{}

func (snapshotObservableSelectionPersistedUniverse) selectSnapshotObservables(
	input MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	universe, ok := input.PersistedEntityUniverse().(ExactPersistedEntityUniverse)
	if !ok || !universe.Valid() {
		return NewSnapshotObservableInputsUnavailable()
	}
	blob, err := universe.ObservableBlob()
	if err != nil {
		return NewSnapshotObservableInputsUnavailable()
	}
	selected, err := NewSnapshotObservableInputsSelected(
		[]ObservableInputBlob{blob},
	)
	if err != nil {
		return NewSnapshotObservableInputsUnavailable()
	}
	return selected
}

type snapshotObservableSelectionExact struct {
	blobs []ObservableInputBlob
}

func (selection snapshotObservableSelectionExact) selectSnapshotObservables(
	MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	selected, err := NewSnapshotObservableInputsSelected(selection.blobs)
	if err != nil {
		return NewSnapshotObservableInputsUnavailable()
	}
	return selected
}

var _ MemberOfEvaluationEngine = snapshotSelectingExactBasisEngine{}
var _ SnapshotObservableInputSelector = snapshotSelectingExactBasisEngine{}
