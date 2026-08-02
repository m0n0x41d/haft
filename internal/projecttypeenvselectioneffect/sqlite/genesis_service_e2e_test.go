package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionrevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionreadset"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

var (
	genesisE2EBaseOnce sync.Once
	genesisE2EBase     typeenv.BaseTypeEnvArtifact
	genesisE2EBaseErr  error
)

type genesisE2EClock struct {
	mu    sync.Mutex
	value time.Time
}

func (clock *genesisE2EClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	current := clock.value
	clock.value = clock.value.Add(time.Millisecond)
	return current
}

type genesisE2ERejectingCodec struct{}

func (genesisE2ERejectingCodec) Canonicalize(
	typedmemory.ValueShapeRef,
	[]byte,
) typedmemory.CodecCanonicalization {
	return typedmemory.RejectedCodecValue{}
}

type genesisE2EUnexpectedMemberOfEngine struct{}

func (genesisE2EUnexpectedMemberOfEngine) EvaluateMemberOf(
	context.Context,
	typedmemorystore.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, fmt.Errorf("Genesis E2E declare-entity admission should not evaluate MemberOf")
}

type genesisE2EUnexpectedObservableProvider struct{}

func (genesisE2EUnexpectedObservableProvider) LoadObservableInput(
	context.Context,
	projectidentity.ProjectID,
	typedmemory.ObservableInputRef,
	typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
		"Genesis E2E declare-entity admission should not load observable input",
	)
}

type genesisE2EUnexpectedReferenceEngine struct{}

func (genesisE2EUnexpectedReferenceEngine) ResolveStrongReference(
	context.Context,
	typedmemorystore.StrongReferenceResolutionInput,
) (typedmemory.StrongReferenceResolution, error) {
	return nil, fmt.Errorf(
		"Genesis E2E declare-entity admission should not resolve strong references",
	)
}

type genesisE2ETarget struct {
	closure      projecttypeenvstore.ArtifactClosure
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
	runtime      projecttypeenv.RuntimeEvaluationBasisArtifact
	mechanism    runtimemechanism.RuntimeMechanismArtifactV1
	installed    projecttypeenvruntime.InstalledRuntimeRegistryInput
	registry     projecttypeenvruntime.ExactTargetRuntimeRegistry
}

type genesisE2EFixture struct {
	database *sql.DB
	project  projectidentity.ProjectID
	target   genesisE2ETarget
	stage    projecttypeenvselection.ProjectTypeEnvStage
	request  projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content  projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent
	service  *GenesisService
}

func TestGenesisSelectionInputExposesOnlyImmutableProposalAndIngress(
	t *testing.T,
) {
	inputType := reflect.TypeOf(GenesisSelectionInput{})
	got := make([]string, inputType.NumField())
	for index := 0; index < inputType.NumField(); index++ {
		got[index] = inputType.Field(index).Name
	}
	want := []string{"Request", "Content", "Authority"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GenesisSelectionInput fields = %v, want exactly %v", got, want)
	}
}

func TestGenesisServiceCommitsEffectOwnedHeadAbsenceAndReplaysExactClosure(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	input := GenesisSelectionInput{
		Request:   fixture.request,
		Content:   fixture.content,
		Authority: hostRoutedIngressForTest(t, fixture.request, fixture.content),
	}

	result, err := fixture.service.SelectGenesis(ctx, input)
	if err != nil {
		t.Fatalf("SelectGenesis(fresh): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("fresh SelectGenesis result = %T, want FreshlyCommitted", result)
	}
	if _, ok := fresh.Delivery().(projecttypeenvselectioneffect.CommittedAndObserved); !ok {
		t.Fatalf("fresh delivery posture = %T, want CommittedAndObserved", fresh.Delivery())
	}
	closure := fresh.Closure()
	if err := closure.Verify(); err != nil {
		t.Fatalf("fresh closure Verify(): %v", err)
	}
	assertGenesisE2ECommittedFootprint(t, fixture, closure)
	beforeReplay := genesisE2EEffectCounts(t, fixture.database)

	replayedResult, err := fixture.service.SelectGenesis(ctx, input)
	if err != nil {
		t.Fatalf("SelectGenesis(replay): %v", err)
	}
	replayed, ok := replayedResult.(projecttypeenvselectioneffect.ReplayedExisting)
	if !ok {
		t.Fatalf("replay SelectGenesis result = %T, want ReplayedExisting", replayedResult)
	}
	if replayed.Closure().Ref() != closure.Ref() ||
		!bytes.Equal(replayed.Closure().CanonicalBytes(), closure.CanonicalBytes()) {
		t.Fatal("exact replay returned a different committed closure")
	}
	afterReplay := genesisE2EEffectCounts(t, fixture.database)
	if !reflect.DeepEqual(afterReplay, beforeReplay) {
		t.Fatalf(
			"exact replay changed effect row counts: before=%v after=%v",
			beforeReplay,
			afterReplay,
		)
	}
}

func TestGenesisSelectionFeedsProjectAwareCurrentSnapshot(t *testing.T) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	result, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("SelectGenesis() = %T, want FreshlyCommitted", result)
	}
	closure := fresh.Closure()

	stageStore, err := projecttypeenvstage.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New(): %v", err)
	}
	headStore, err := projecttypeenvheadstore.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvheadstore.New(): %v", err)
	}
	resolver, err := projectmemory.NewProjectTypeEnvRuntimeResolver(
		stageStore,
		headStore,
		NewCurrentCommittedClosureLoader(),
		genesisE2EInstalledRuntimeCatalog(t, fixture.target),
	)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvRuntimeResolver(): %v", err)
	}
	loader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		projectmemory.NewBaseTypeEnvLoader(),
		resolver,
	)
	if err != nil {
		t.Fatalf("NewProjectAwareSQLiteCurrentProjectSnapshotLoader(): %v", err)
	}

	current, err := loader.LoadCurrentProjectSnapshot(ctx, fixture.project)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(): %v", err)
	}
	if current.ProjectID() != fixture.project {
		t.Fatalf(
			"current project = %q, want %q",
			current.ProjectID().String(),
			fixture.project.String(),
		)
	}
	if current.Environment().Ref() != closure.Target().Composite() {
		t.Fatalf(
			"current TypeEnv = %q, want selected C %q",
			current.Environment().Ref().String(),
			closure.Target().Composite().String(),
		)
	}
	if current.Snapshot().GraphRevision() != closure.CommittedGraphRevision() {
		t.Fatalf(
			"current graph revision = %d, want committed revision %d",
			current.Snapshot().GraphRevision().Value(),
			closure.CommittedGraphRevision().Value(),
		)
	}
	if current.Snapshot().TypeEnvRef() != closure.Target().Composite() {
		t.Fatalf(
			"snapshot TypeEnv = %q, want selected C %q",
			current.Snapshot().TypeEnvRef().String(),
			closure.Target().Composite().String(),
		)
	}
	if current.Codecs().Len() == 0 {
		t.Fatal("project-aware current snapshot lost the selected runtime codecs")
	}
}

func TestGenesisSelectionFeedsProjectCurrentValidationAdmissionAndReplay(
	t *testing.T,
) {
	fixture := newGenesisE2EFixture(t)
	ctx := context.Background()
	result, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	if err != nil {
		t.Fatalf("SelectGenesis(): %v", err)
	}
	fresh, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf("SelectGenesis() = %T, want FreshlyCommitted", result)
	}
	closure := fresh.Closure()
	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	loader, err := typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
		fixture.database,
		projectmemory.NewBaseTypeEnvLoader(),
		resolver,
	)
	if err != nil {
		t.Fatalf("NewProjectAwareSQLiteCurrentProjectSnapshotLoader(): %v", err)
	}
	currentBeforeAdmission, err := loader.LoadCurrentProjectSnapshot(
		ctx,
		fixture.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(before admit): %v", err)
	}
	contexts := currentBeforeAdmission.Environment().BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("selected C exposes no bounded contexts for declaration")
	}
	contextRef := contexts[0].Ref()
	source := newGenesisE2ECurrentProjectBasisSource(t, loader)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 17, 10, 15, 0, 0, time.UTC),
	}
	adapter, err := typedmemorystore.NewProjectExecutableGenericSQLiteAdapterBuilder(
		fixture.database,
	).
		SetTypeEnvLoader(projectmemory.NewBaseTypeEnvLoader()).
		SetClock(clock).
		SetReferenceEngine(genesisE2EUnexpectedReferenceEngine{}).
		SetObservableInputs(genesisE2EUnexpectedObservableProvider{}).
		SetSelectedProjectRuntime(resolver).
		Build()
	if err != nil {
		t.Fatalf("project-executable adapter builder: %v", err)
	}
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	if err != nil {
		t.Fatalf("NewAdmissionRuntime(): %v", err)
	}
	request := genesisE2EProjectCurrentDeclareRequest(t, contextRef)
	validationRuntime, err := projectmemory.NewValidationRuntime(
		fixture.project,
		source,
	)
	if err != nil {
		t.Fatalf("NewValidationRuntime(): %v", err)
	}
	response, err := validationRuntime.Validate(ctx, request)
	if err != nil {
		t.Fatalf("Validate(project_current): %v", err)
	}
	if response.Verdict() != typedmemory.ValidationValid {
		t.Fatalf(
			"Validate(project_current) verdict = %s diagnostics=%#v",
			response.Verdict(),
			response.Diagnostics(),
		)
	}
	prepared, err := runtime.PrepareAdmission(ctx, request)
	if err != nil {
		t.Fatalf("PrepareAdmission(): %v", err)
	}
	key, err := typedmemorystore.NewIdempotencyKey(
		"genesis-e2e-project-current-admit",
	)
	if err != nil {
		t.Fatalf("NewIdempotencyKey(): %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:genesis-e2e-project-current-admit",
	)
	if err != nil {
		t.Fatalf("NewProvenanceRef(): %v", err)
	}

	receipt, err := runtime.AdmitValidated(ctx, prepared, key, provenance)
	if err != nil {
		t.Fatalf("AdmitValidated(fresh): %v", err)
	}
	if receipt.Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf("fresh disposition = %s, want applied", receipt.Disposition())
	}
	if receipt.GraphRevision().Value() != closure.CommittedGraphRevision().Value()+1 {
		t.Fatalf(
			"fresh graph revision = %d, want %d",
			receipt.GraphRevision().Value(),
			closure.CommittedGraphRevision().Value()+1,
		)
	}
	replay, err := runtime.AdmitValidated(ctx, prepared, key, provenance)
	if err != nil {
		t.Fatalf("AdmitValidated(replay): %v", err)
	}
	if replay.Disposition() != typedmemorystore.CommitReplay {
		t.Fatalf("replay disposition = %s, want replay", replay.Disposition())
	}
	if replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("replay = %#v; want original receipt %#v", replay, receipt)
	}
	current, err := loader.LoadCurrentProjectSnapshot(ctx, fixture.project)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(after admit): %v", err)
	}
	if current.Snapshot().GraphRevision() != receipt.GraphRevision() {
		t.Fatalf(
			"current graph revision after admit = %d, want receipt revision %d",
			current.Snapshot().GraphRevision().Value(),
			receipt.GraphRevision().Value(),
		)
	}
	if current.Snapshot().TypeEnvRef() != closure.Target().Composite() {
		t.Fatalf(
			"current TypeEnv after admit = %q, want selected C %q",
			current.Snapshot().TypeEnvRef().String(),
			closure.Target().Composite().String(),
		)
	}
}

func newGenesisE2EFixture(t *testing.T) genesisE2EFixture {
	t.Helper()
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	database := harness.Database()
	project, err := projectidentity.ParseProjectID(harness.ProjectID())
	if err != nil {
		t.Fatalf("ParseProjectID(%q): %v", harness.ProjectID(), err)
	}
	admission := harness.AdmitSoftwareRevision(t, "genesis-selection-e2e")
	currentProfile, err := declaredProjectProfileBasis(admission)
	if err != nil {
		t.Fatalf("declaredProjectProfileBasis(): %v", err)
	}
	target := newGenesisE2ETarget(t)
	stageStore, err := projecttypeenvstage.New(ctx, database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New(): %v", err)
	}
	if err := stageStore.PutArtifactClosure(ctx, target.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	clock := genesisE2EClock{
		value: time.Date(2026, 7, 17, 9, 30, 0, 0, time.UTC),
	}
	initializeGenesisE2EProjectGraph(
		t,
		database,
		project,
		target.closure.Base(),
		&clock,
	)
	stage := newGenesisE2EStage(
		t,
		database,
		project,
		currentProfile,
		target,
	)
	if err := stageStore.Put(
		ctx,
		stage,
		target.verification.Record(),
		target.snapshot.Record(),
	); err != nil {
		t.Fatalf("Stage Put(): %v", err)
	}
	ready, err := stageStore.LoadSelectionReady(ctx, stage.Ref())
	if err != nil {
		t.Fatalf("LoadSelectionReady(): %v", err)
	}
	if ready.Stage().Ref() != stage.Ref() ||
		ready.ExecutableSnapshot().TypeEnvRef() != target.verification.CompositeRef() {
		t.Fatal("selection-ready Stage reload changed Stage or target C")
	}
	assertGenesisE2EStageCurrent(
		t,
		database,
		project,
		currentProfile,
		ready,
		target.registry,
	)
	key, err :=
		projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
			"genesis-e2e-key",
		)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey(): %v", err)
	}
	request, err :=
		projecttypeenvselection.SealGenesisProjectTypeEnvHeadSelectionRequest(
			projecttypeenvselection.GenesisProjectTypeEnvHeadSelectionRequestInput{
				Project:               project,
				Stage:                 stage,
				ExpectedGraphRevision: typedmemory.NewGraphRevision(0),
				IdempotencyKey:        key,
			},
		)
	if err != nil {
		t.Fatalf("SealGenesisProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	description, err := authority.NewClaimIDDescriptionRef(
		"claim:project-typeenv-head-selection:genesis-e2e",
	)
	if err != nil {
		t.Fatalf("NewClaimIDDescriptionRef(): %v", err)
	}
	judgementContext, err := authority.NewBoundedContextRef(
		"bounded-context:project-typeenv-head-selection",
	)
	if err != nil {
		t.Fatalf("NewBoundedContextRef(): %v", err)
	}
	validity, err := authority.NewTimeWindow(
		clock.Now().Add(-time.Hour),
		clock.Now().Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow(): %v", err)
	}
	content, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionAuthorizationContent(
			projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContentInput{
				DescriptionRef:   description,
				Request:          request,
				Stage:            stage,
				JudgementContext: judgementContext,
				ValidityWindow:   validity,
			},
		)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadSelectionAuthorizationContent(): %v", err)
	}
	service, err := NewGenesisService(
		ctx,
		database,
		harness.Root().String(),
		target.installed,
		&clock,
	)
	if err != nil {
		t.Fatalf("NewGenesisService(): %v", err)
	}
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

func genesisE2EProjectRuntimeResolver(
	t *testing.T,
	fixture genesisE2EFixture,
) *projectmemory.ProjectTypeEnvRuntimeResolver {
	t.Helper()
	ctx := context.Background()
	stageStore, err := projecttypeenvstage.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvstage.New(): %v", err)
	}
	headStore, err := projecttypeenvheadstore.New(ctx, fixture.database)
	if err != nil {
		t.Fatalf("projecttypeenvheadstore.New(): %v", err)
	}
	resolver, err := projectmemory.NewProjectTypeEnvRuntimeResolver(
		stageStore,
		headStore,
		NewCurrentCommittedClosureLoader(),
		genesisE2EInstalledRuntimeCatalog(t, fixture.target),
	)
	if err != nil {
		t.Fatalf("NewProjectTypeEnvRuntimeResolver(): %v", err)
	}
	return resolver
}

func genesisE2EInstalledRuntimeCatalog(
	t *testing.T,
	target genesisE2ETarget,
) projectmemory.InstalledProjectTypeEnvRuntimeCatalog {
	t.Helper()
	entry, err := projectmemory.NewInstalledProjectTypeEnvRuntimeEntry(
		target.runtime,
		target.installed,
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeEntry(): %v", err)
	}
	catalog, err := projectmemory.NewInstalledProjectTypeEnvRuntimeCatalog(
		[]projectmemory.InstalledProjectTypeEnvRuntimeEntry{entry},
	)
	if err != nil {
		t.Fatalf("NewInstalledProjectTypeEnvRuntimeCatalog(): %v", err)
	}
	return catalog
}

func newGenesisE2ECurrentProjectBasisSource(
	t *testing.T,
	loader typedmemorystore.CurrentProjectSnapshotLoader,
) *projectmemory.CurrentProjectBasisSource {
	t.Helper()
	source, err := projectmemory.NewCurrentProjectBasisSource(loader)
	if err != nil {
		t.Fatalf("NewCurrentProjectBasisSource(): %v", err)
	}
	return source
}

func genesisE2EProjectCurrentDeclareRequest(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {"kind": "project_current"},
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:genesis-e2e-project-current",
      "local_ref": "local:genesis-e2e-project-current",
      "context": %q,
      "label": "Genesis E2E project-current entity",
      "provenance": "provenance:genesis-e2e-project-current"
    }]
  }
}`, typedmemorywire.ContractVersionV2, contextRef.String())
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest(): %v\npayload=%s", err, payload)
	}
	return request
}

func newGenesisE2ETarget(t *testing.T) genesisE2ETarget {
	return newGenesisE2ETargetWithRuntime(
		t,
		"artifact:genesis-e2e-runtime",
		"1.0.0",
	)
}

func newGenesisE2ETargetWithRuntime(
	t *testing.T,
	artifactRefText string,
	editionText string,
) genesisE2ETarget {
	t.Helper()
	base := genesisE2EBaseArtifact(t)
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, nil)
	if resolution.Rejected() {
		t.Fatalf("LinkProjectTypeEnvCompositeIR(B, empty E): %#v", resolution.Issues())
	}
	linked, ok := resolution.CompositeIR()
	if !ok {
		t.Fatal("accepted B/empty-E link produced no linked IR")
	}
	runtimeBasis, mechanism := newGenesisE2ERuntimeBasisWithIdentity(
		t,
		base,
		linked,
		artifactRefText,
		editionText,
	)
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(
		linked,
		runtimeBasis,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	preparation := projecttypeenv.PrepareProjectTypeEnvComposite(
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         base,
			Linked:       linked,
			RuntimeBasis: runtimeBasis,
			Composite:    composite,
		},
	)
	if preparation.Rejected() {
		t.Fatalf("PrepareProjectTypeEnvComposite(): %#v", preparation.Issues())
	}
	verification, ok := preparation.Verification()
	if !ok {
		t.Fatal("prepared Genesis target has no final verification")
	}
	snapshot, ok := preparation.ExecutableSnapshot()
	if !ok {
		t.Fatal("prepared Genesis target has no executable snapshot")
	}
	closure, err := projecttypeenvstore.PrepareArtifactClosureWithRuntimeMechanisms(
		base,
		nil,
		runtimeBasis,
		composite,
		[]runtimemechanism.RuntimeMechanismArtifactV1{mechanism},
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosureWithRuntimeMechanisms(): %v", err)
	}
	installed := genesisE2EInstalledRuntime(t, runtimeBasis, mechanism)
	observation := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: runtimeBasis,
			Installed:    installed,
		},
	)
	matched, ok := observation.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf(
			"Genesis E2E installed runtime = %T (%s), want Matched",
			observation,
			observation.Kind(),
		)
	}
	registry, exists := matched.Registry()
	if !exists || !registry.Valid() {
		t.Fatal("Genesis E2E installed runtime exposed no exact registry")
	}
	return genesisE2ETarget{
		closure:      closure,
		verification: verification,
		snapshot:     snapshot,
		runtime:      runtimeBasis,
		mechanism:    mechanism,
		installed:    installed,
		registry:     registry,
	}
}

func genesisE2EBaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	genesisE2EBaseOnce.Do(func() {
		path, err := filepath.Abs(filepath.Join("..", "..", "cli", "fpf.db"))
		if err != nil {
			genesisE2EBaseErr = err
			return
		}
		database, err := sql.Open(
			"sqlite",
			"file:"+filepath.ToSlash(path)+"?mode=ro&immutable=1",
		)
		if err != nil {
			genesisE2EBaseErr = err
			return
		}
		database.SetMaxOpenConns(1)
		defer func() { _ = database.Close() }()
		genesisE2EBase, genesisE2EBaseErr =
			typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	})
	if genesisE2EBaseErr != nil {
		t.Fatalf("load Genesis E2E B: %v", genesisE2EBaseErr)
	}
	return genesisE2EBase
}

func newGenesisE2ERuntimeBasis(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
) {
	return newGenesisE2ERuntimeBasisWithIdentity(
		t,
		base,
		linked,
		"artifact:genesis-e2e-runtime",
		"1.0.0",
	)
}

func newGenesisE2ERuntimeBasisWithIdentity(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	artifactRefText string,
	editionText string,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
) {
	t.Helper()
	empty, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(empty): %v", err)
	}
	provisional, err := projecttypeenv.SealProjectTypeEnvComposite(linked, empty)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(provisional): %v", err)
	}
	candidate, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		provisional.Ref(),
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef(): %v", err)
	}
	requirementResolution :=
		projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
			provisional,
			candidate,
			linked,
			empty,
		)
	requirements := requirementResolution.RequiredSet().Requirements()
	if len(requirements) == 0 {
		t.Fatal("Genesis E2E provisional C has no runtime requirements")
	}
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, entryErr := genesisE2EMechanismEntry(requirement)
		if entryErr != nil {
			t.Fatalf("build Genesis E2E runtime entry: %v", entryErr)
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef(artifactRefText)
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition(editionText)
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	mechanism, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		entries,
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(): %v", err)
	}
	mechanismPin, err :=
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(mechanism)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	pins := make([]projecttypeenv.RuntimeEvaluationMechanismPin, 0, len(requirements))
	for _, requirement := range requirements {
		pin, pinErr := genesisE2EMechanismPin(
			requirement,
			mechanismPin,
			mechanism,
		)
		if pinErr != nil {
			t.Fatalf("build Genesis E2E runtime pin: %v", pinErr)
		}
		pins = append(pins, pin)
	}
	runtimeBasis, err := projecttypeenv.SealRuntimeEvaluationBasis(
		pins,
		mechanism,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	return runtimeBasis, mechanism
}

func genesisE2EMechanismEntry(
	requirement projecttypeenv.CompositeRuntimeRequirement,
) (runtimemechanism.RuntimeMechanismEntryV1, error) {
	if codec, ok := requirement.Codec(); ok {
		return runtimemechanism.NewCodecCanonicalizationEntry(codec)
	}
	rule, ok := requirement.Rule()
	if !ok {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	constructors := map[projecttypeenv.RuntimeMechanismInvocationContract]func(
		typedmemory.RuleRef,
	) (runtimemechanism.RuntimeMechanismEntryV1, error){
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:      runtimemechanism.NewEntitySetEnumerationEntry,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility:       runtimemechanism.NewCandidateVisibilityEntry,
		projecttypeenv.RuntimeMechanismContractKindDefinedness:           runtimemechanism.NewKindDefinednessEntry,
		projecttypeenv.RuntimeMechanismContractMemberOf:                  runtimemechanism.NewMemberOfEntry,
		projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery: runtimemechanism.NewCarrierMembershipDeliveryEntry,
	}
	constructor, ok := constructors[requirement.InvocationContract()]
	if !ok {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"unsupported runtime invocation contract %q",
			requirement.InvocationContract(),
		)
	}
	return constructor(rule)
}

func genesisE2EMechanismPin(
	requirement projecttypeenv.CompositeRuntimeRequirement,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) (projecttypeenv.RuntimeEvaluationMechanismPin, error) {
	if codec, ok := requirement.Codec(); ok {
		return projecttypeenv.NewCodecRuntimeMechanismPin(
			projecttypeenv.CodecRuntimeMechanismPinInput{
				Codec:            codec,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	rule, ok := requirement.Rule()
	if !ok {
		return nil, fmt.Errorf(
			"runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	if requirement.Role() == projecttypeenv.RuntimeMechanismRoleCarrierMembership {
		return projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	return projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         requirement.InvocationContract(),
			Mechanism:        mechanism,
			ResolvedArtifact: &artifact,
		},
	)
}

func genesisE2EInstalledRuntime(
	t *testing.T,
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact,
	mechanism runtimemechanism.RuntimeMechanismArtifactV1,
) projecttypeenvruntime.InstalledRuntimeRegistryInput {
	t.Helper()
	codecs := typedmemory.NewCodecRegistry()
	for _, pin := range runtimeBasis.Pins() {
		codecPin, ok := pin.(projecttypeenv.CodecRuntimeMechanismPin)
		if !ok {
			t.Fatalf(
				"Genesis E2E X contains unsupported callable pin %T",
				pin,
			)
		}
		updated, err := codecs.Register(
			codecPin.Codec(),
			genesisE2ERejectingCodec{},
		)
		if err != nil {
			t.Fatalf("register Genesis E2E codec %s: %v", codecPin.Codec(), err)
		}
		codecs = updated
	}
	return projecttypeenvruntime.InstalledRuntimeRegistryInput{
		Codecs:            codecs,
		MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{mechanism},
	}
}

func newGenesisE2EStage(
	t *testing.T,
	database *sql.DB,
	project projectidentity.ProjectID,
	currentProfile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	target genesisE2ETarget,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("BeginRead(Stage graph basis): %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	currentGraph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx(Stage): %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit Stage graph-basis read: %v", finish.Err())
	}
	graphBasis := currentGraph.GraphSnapshotBasis()
	revalidation, err := projecttypeenvassertionrevalidation.Revalidate(
		projecttypeenvassertionrevalidation.Input{
			CurrentGraph:  currentGraph,
			TargetTypeEnv: target.snapshot.Environment(),
			TargetRuntime: target.registry,
		},
	)
	if err != nil {
		t.Fatalf("projecttypeenvassertionrevalidation.Revalidate(): %v", err)
	}
	profileFit, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		currentProfile,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(): %v", err)
	}
	compatibility, err := projecttypeenvselection.NewInitialStageCompatibility(
		target.verification.CompositeRef(),
	)
	if err != nil {
		t.Fatalf("NewInitialStageCompatibility(): %v", err)
	}
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                       project,
			Predecessor:                   projecttypeenvselection.NewGenesisStagePredecessor(),
			Base:                          target.verification.BaseTypeEnvRef(),
			OrderedExtensions:             target.verification.ExtensionRefs(),
			RuntimeBasis:                  target.verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:             target.verification,
			Composite:                     target.verification.CompositeRef(),
			GraphSnapshotBasis:            graphBasis,
			GraphSnapshotBasisRef:         graphBasis.Ref(),
			GraphSnapshotBasisDigest:      graphBasis.Ref().Digest(),
			GraphRevision:                 graphBasis.GraphRevision(),
			ProfileLedgerRevision:         currentProfile.LedgerRevision(),
			ProfileLedgerDigest:           currentProfile.ProfileLedgerDigest(),
			Compatibility:                 compatibility,
			ExistingAssertionRevalidation: revalidation,
			ProfileCompatibility:          profileFit,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(): %v", err)
	}
	return stage
}

func initializeGenesisE2EProjectGraph(
	t *testing.T,
	database *sql.DB,
	project projectidentity.ProjectID,
	base typeenv.BaseTypeEnvArtifact,
	clock *genesisE2EClock,
) {
	t.Helper()
	snapshot, err := projectmemory.NewBaseTypeEnvSnapshot(base)
	if err != nil {
		t.Fatalf("NewBaseTypeEnvSnapshot(Genesis E2E B): %v", err)
	}
	initializer, err := typedmemorystore.NewSQLiteProjectGraphInitializer(
		database,
		projectmemory.NewBaseTypeEnvLoader(),
		clock,
	)
	if err != nil {
		t.Fatalf("NewSQLiteProjectGraphInitializer(): %v", err)
	}
	result, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		context.Background(),
		project,
		snapshot,
	)
	if err != nil {
		t.Fatalf("InitializeProjectGraphAtBaseTypeEnv(): %v", err)
	}
	initialized, ok := result.(typedmemorystore.InitializedAtBase)
	if !ok {
		t.Fatalf(
			"InitializeProjectGraphAtBaseTypeEnv() = %T, want InitializedAtBase",
			result,
		)
	}
	if initialized.Project() != project ||
		initialized.BaseTypeEnv() != snapshot.Ref() {
		t.Fatal("Genesis E2E graph initializer lost project or exact B")
	}
}

// These seed helpers remain for the separate production-Note fixture. The
// Genesis service fixture above exercises the production initializer.
func seedGenesisE2EBaseSnapshot(
	t *testing.T,
	database *sql.DB,
	base typeenv.BaseTypeEnvArtifact,
	recordedAt time.Time,
) {
	t.Helper()
	ref, ok := base.TypeEnvRef()
	if !ok {
		t.Fatal("Genesis E2E B has no executable TypeEnv reference")
	}
	_, err := database.Exec(
		`INSERT INTO typed_memory_type_env_snapshots (
			type_env_ref,
			artifact_digest,
			snapshot_format,
			canonical_bytes,
			source_revision,
			compiler_schema_version,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ref.String(),
		base.Digest().String(),
		typedmemorystore.BaseTypeEnvSnapshotFormat,
		base.CanonicalBytes(),
		base.SourceRevision().String(),
		base.CompilerSchemaVersion().String(),
		recordedAt.Round(0).UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("seed generic B TypeEnv snapshot: %v", err)
	}
}

func seedGenesisE2EGraphHead(
	t *testing.T,
	database *sql.DB,
	project projectidentity.ProjectID,
	activeTypeEnv typedmemory.TypeEnvRef,
	recordedAt time.Time,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO typed_memory_graph_heads (
			project_id,
			graph_revision,
			active_type_env_ref,
			last_event_ref,
			last_commit_ref,
			updated_at
		) VALUES (?, 0, ?, '', '', ?)`,
		project.String(),
		activeTypeEnv.String(),
		recordedAt.Round(0).UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("seed typed-memory graph head: %v", err)
	}
}

func assertGenesisE2EStageCurrent(
	t *testing.T,
	database *sql.DB,
	project projectidentity.ProjectID,
	currentProfile projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	ready projecttypeenvstage.SelectionReadyStage,
	registry projecttypeenvruntime.ExactTargetRuntimeRegistry,
) {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("BeginImmediate(Stage fixture validation): %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	currentGraph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx(): %v", err)
	}
	headStore, err := projecttypeenvheadstore.New(ctx, database)
	if err != nil {
		t.Fatalf("projecttypeenvheadstore.New(): %v", err)
	}
	currentHead, err := headStore.LoadCurrentProjectTypeEnvHeadTx(
		ctx,
		transaction,
		project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectTypeEnvHeadTx(): %v", err)
	}
	result := projecttypeenvstagerevalidation.Revalidate(
		projecttypeenvstagerevalidation.ProjectTypeEnvStageRevalidationInput{
			Stage:                 ready.Stage(),
			FinalVerification:     ready.FinalLowererVerification(),
			ExecutableTarget:      ready.ExecutableSnapshot(),
			TargetRuntimeRegistry: registry,
			CurrentGraph:          currentGraph,
			CurrentProfile:        currentProfile,
			CurrentHead:           currentHead,
		},
	)
	current, ok := result.(projecttypeenvstagerevalidation.CurrentSelectionStage)
	if ok && current.Valid() {
		return
	}
	type issueResult interface {
		Issues() []projecttypeenvstagerevalidation.StageRevalidationIssue
	}
	withIssues, ok := result.(issueResult)
	if !ok {
		t.Fatalf("fixture Stage revalidation = %T", result)
	}
	issues := withIssues.Issues()
	details := make([]string, 0, len(issues))
	for _, issue := range issues {
		details = append(
			details,
			fmt.Sprintf(
				"%s/%s subject=%q expected=%q actual=%q",
				issue.Kind(),
				issue.Code(),
				issue.Subject(),
				issue.Expected(),
				issue.Actual(),
			),
		)
	}
	t.Fatalf(
		"fixture Stage revalidation = %T: %s",
		result,
		strings.Join(details, "; "),
	)
}

type genesisE2ECounts struct {
	proofs         int
	requests       int
	authorityUses  int
	casWork        int
	activations    int
	headHistory    int
	receipts       int
	closures       int
	graphEvents    int
	graphCommits   int
	dedicatedHeads int
}

func genesisE2EEffectCounts(
	t *testing.T,
	database *sql.DB,
) genesisE2ECounts {
	t.Helper()
	count := func(table string) int {
		var value int
		if err := database.QueryRow(
			"SELECT COUNT(*) FROM " + table,
		).Scan(&value); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return value
	}
	return genesisE2ECounts{
		proofs:         count("project_typeenv_no_prior_head_proofs"),
		requests:       count("project_typeenv_head_selection_requests"),
		authorityUses:  count("project_typeenv_head_selection_authority_uses"),
		casWork:        count("project_typeenv_head_cas_work_records"),
		activations:    count("typed_memory_type_env_activations"),
		headHistory:    count("project_typeenv_head_history"),
		receipts:       count("project_typeenv_head_selection_receipts"),
		closures:       count("project_typeenv_head_selection_closures"),
		graphEvents:    count("typed_memory_graph_events"),
		graphCommits:   count("typed_memory_graph_commits"),
		dedicatedHeads: count("project_typeenv_heads"),
	}
}

func assertGenesisE2ECommittedFootprint(
	t *testing.T,
	fixture genesisE2EFixture,
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
) {
	t.Helper()
	var dedicatedRevision int64
	var dedicatedComposite string
	err := fixture.database.QueryRow(
		`SELECT head_revision, selected_composite_ref
		FROM project_typeenv_heads
		WHERE project_id = ?`,
		fixture.project.String(),
	).Scan(&dedicatedRevision, &dedicatedComposite)
	if err != nil {
		t.Fatalf("read committed dedicated head: %v", err)
	}
	if dedicatedRevision != 1 ||
		dedicatedComposite != fixture.target.verification.CompositeRef().String() {
		t.Fatalf(
			"dedicated head = revision %d composite %q",
			dedicatedRevision,
			dedicatedComposite,
		)
	}
	var graphRevision int64
	var activeTypeEnv string
	err = fixture.database.QueryRow(
		`SELECT graph_revision, active_type_env_ref
		FROM typed_memory_graph_heads
		WHERE project_id = ?`,
		fixture.project.String(),
	).Scan(&graphRevision, &activeTypeEnv)
	if err != nil {
		t.Fatalf("read committed graph head: %v", err)
	}
	if graphRevision != 1 ||
		activeTypeEnv != fixture.target.verification.CompositeRef().String() {
		t.Fatalf(
			"graph head = revision %d active TypeEnv %q",
			graphRevision,
			activeTypeEnv,
		)
	}
	var proofRef string
	var proofDigest string
	var observationSchema string
	var observedAt string
	var proofCanonical []byte
	err = fixture.database.QueryRow(
		`SELECT proof_ref, proof_digest, observation_schema,
			observed_at, canonical_bytes
		FROM project_typeenv_no_prior_head_proofs
		WHERE project_id = ?`,
		fixture.project.String(),
	).Scan(
		&proofRef,
		&proofDigest,
		&observationSchema,
		&observedAt,
		&proofCanonical,
	)
	if err != nil {
		t.Fatalf("read committed no-prior-head proof: %v", err)
	}
	if observationSchema != "effect_owned_head_absence_v1" || observedAt == "" {
		t.Fatalf(
			"proof observation = schema %q observed_at %q",
			observationSchema,
			observedAt,
		)
	}
	parsedProofRef, err :=
		projecttypeenvselection.ParseNoPriorHeadProofRef(proofRef)
	if err != nil {
		t.Fatalf("ParseNoPriorHeadProofRef(): %v", err)
	}
	proof, err := projecttypeenvselectionreadset.VerifyNoPriorHeadProof(
		parsedProofRef,
		proofCanonical,
	)
	if err != nil {
		t.Fatalf("VerifyNoPriorHeadProof(): %v", err)
	}
	if proof.Digest().String() != proofDigest ||
		proof.Project() != fixture.project ||
		proof.GraphRevision().Value() != 0 {
		t.Fatal("effect-owned no-prior-head proof lost its exact observation basis")
	}
	var requestSchema string
	var requestProofRef sql.NullString
	var requestProofDigest sql.NullString
	err = fixture.database.QueryRow(
		`SELECT request_schema, no_prior_head_proof_ref,
			no_prior_head_proof_digest
		FROM project_typeenv_head_selection_requests
		WHERE request_ref = ?`,
		fixture.request.Ref().String(),
	).Scan(&requestSchema, &requestProofRef, &requestProofDigest)
	if err != nil {
		t.Fatalf("read committed Genesis request: %v", err)
	}
	if requestSchema != "haft.project-typeenv.head-selection-request.v2" ||
		requestProofRef.Valid ||
		requestProofDigest.Valid {
		t.Fatalf(
			"current request = schema %q proof (%#v,%#v)",
			requestSchema,
			requestProofRef,
			requestProofDigest,
		)
	}
	assertGenesisE2EProofBinding(
		t,
		fixture.database,
		fixture.project.String(),
		proofRef,
		proofDigest,
	)
	var storedClosure []byte
	err = fixture.database.QueryRow(
		`SELECT canonical_bytes
		FROM project_typeenv_head_selection_closures
		WHERE closure_ref = ?`,
		closure.Ref().String(),
	).Scan(&storedClosure)
	if err != nil {
		t.Fatalf("read committed selection closure: %v", err)
	}
	if !bytes.Equal(storedClosure, closure.CanonicalBytes()) {
		t.Fatal("stored selection closure differs from returned closure")
	}
	counts := genesisE2EEffectCounts(t, fixture.database)
	want := genesisE2ECounts{
		proofs:         1,
		requests:       1,
		authorityUses:  1,
		casWork:        1,
		activations:    1,
		headHistory:    1,
		receipts:       1,
		closures:       1,
		graphEvents:    1,
		graphCommits:   1,
		dedicatedHeads: 1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("committed Genesis effect counts = %v, want %v", counts, want)
	}
}

func assertGenesisE2EProofBinding(
	t *testing.T,
	database *sql.DB,
	project string,
	proofRef string,
	proofDigest string,
) {
	t.Helper()
	tables := []string{
		"project_typeenv_head_cas_work_records",
		"typed_memory_type_env_activations",
		"project_typeenv_head_history",
		"project_typeenv_head_selection_receipts",
		"project_typeenv_head_selection_closures",
	}
	for _, table := range tables {
		var storedRef string
		var storedDigest string
		query := fmt.Sprintf(
			`SELECT no_prior_head_proof_ref, no_prior_head_proof_digest
			FROM %s
			WHERE project_id = ?`,
			table,
		)
		err := database.QueryRow(query, project).Scan(&storedRef, &storedDigest)
		if err != nil {
			t.Fatalf("read %s proof binding: %v", table, err)
		}
		if storedRef != proofRef || storedDigest != proofDigest {
			t.Fatalf(
				"%s proof binding = (%q,%q), want (%q,%q)",
				table,
				storedRef,
				storedDigest,
				proofRef,
				proofDigest,
			)
		}
	}
}
