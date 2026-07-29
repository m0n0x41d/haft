package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	basetypeenvartifacts "github.com/m0n0x41d/haft/data/haft/base-typeenv/artifacts"
	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvpreparation"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	selectionsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/spf13/cobra"
)

func TestMemoryTypeEnvCommandsExposeNoTechnicalTransitionCoordinates(
	t *testing.T,
) {
	for _, command := range []*cobra.Command{
		memoryTypeEnvPrepareCmd,
		memoryTypeEnvSelectCmd,
	} {
		if err := command.Args(command, []string{"unexpected"}); err == nil {
			t.Fatalf("%s accepted a positional technical coordinate", command.CommandPath())
		}
		for _, forbidden := range []string{
			"packet",
			"stage",
			"request",
			"digest",
			"hash",
			"speech",
			"phrase",
		} {
			if command.Flags().Lookup(forbidden) != nil {
				t.Fatalf("%s exposes forbidden --%s flag", command.CommandPath(), forbidden)
			}
		}
	}
}

func TestTransitionPriorFixtureBuildsDistinctExactTarget(t *testing.T) {
	runtime, err := loadEmbeddedMemoryRuntime(context.Background())
	if err != nil {
		t.Fatalf("load embedded TypeEnv base: %v", err)
	}
	prior := transitionPriorTarget(t)
	current, err := localpracticeruntime.Build(
		runtime.Artifact(),
		typedmemorycandidates.SourceV1_4(),
	)
	if err != nil {
		t.Fatalf("build current target: %v", err)
	}
	if prior.Composite().Ref() == current.Composite().Ref() {
		t.Fatal("prior fixture did not produce a distinct TypeEnv target")
	}
}

func TestMemoryTypeEnvPrepareAndSelectTransitionWithoutTechnicalArguments(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "memory-typeenv-transition")
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())

	priorTarget := seedPriorProjectTypeEnvHead(t, harness)
	priorComposite := priorTarget.Composite().Ref().String()
	projectID := mustProjectMemoryRuntimeProjectID(t, harness.ProjectID())
	currentOnly, err := buildProjectMemoryRuntimeBasisAtSources(
		context.Background(),
		projectID,
		harness.Database(),
		nil,
	)
	if err != nil {
		t.Fatalf("build current-only project-memory runtime: %v", err)
	}
	_, err = currentOnly.readOnly.Validate(
		context.Background(),
		projectCurrentValidationPayload(),
	)
	if !errors.Is(err, projectmemory.ErrProjectTypeEnvRuntimeUnavailable) {
		t.Fatalf(
			"current-only runtime at predecessor X error = %v, want ErrProjectTypeEnvRuntimeUnavailable",
			err,
		)
	}
	dualRuntime, err := buildProjectMemoryRuntimeBasisAtSources(
		context.Background(),
		projectID,
		harness.Database(),
		[][]byte{transitionPriorSource(t)},
	)
	if err != nil {
		t.Fatalf("build exact-X project-memory runtime catalog: %v", err)
	}
	assertProjectCurrentValidationUsesComposite(
		t,
		dualRuntime.readOnly,
		priorComposite,
	)
	prepareOutput := &bytes.Buffer{}
	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(prepareOutput),
		nil,
	); err != nil {
		t.Fatalf("runMemoryTypeEnvPrepare(Transition): %v", err)
	}
	prepared := transitionPrepareTestResponse{}
	if err := json.Unmarshal(prepareOutput.Bytes(), &prepared); err != nil {
		t.Fatalf("decode Transition prepare response: %v", err)
	}
	if prepared.ContractVersion != projectTypeEnvTransitionReviewSchema ||
		prepared.Result != "prepared_successor" ||
		prepared.ProjectID != harness.ProjectID() ||
		prepared.Candidate.PriorHeadRevision != 1 ||
		prepared.Candidate.PriorCompositeTypeEnvRef != priorComposite ||
		prepared.Candidate.GraphRevision != 1 ||
		prepared.Review.Readiness.Posture != "selectable" ||
		prepared.Interpretation.NextHumanGate == "" ||
		prepared.ReviewCarrier.Path != projectTypeEnvGenesisReviewRelativePath() {
		t.Fatalf("Transition prepare response = %#v", prepared)
	}
	if strings.Contains(prepareOutput.String(), "DECIDE THIS") ||
		strings.Contains(prepareOutput.String(), "--packet") {
		t.Fatalf("Transition prepare leaked legacy operator ceremony: %s", prepareOutput.String())
	}
	observed, err := observeProjectTypeEnvTransitionReview(harness.Root().String())
	if err != nil {
		t.Fatalf("observe readable Transition review: %v", err)
	}
	if observed.carrier.Review.Choice != prepared.Review.Choice ||
		observed.carrier.Candidate.StageRef != prepared.Candidate.StageRef {
		t.Fatal("known review carrier differs from the readable prepare response")
	}

	selectOutput := &bytes.Buffer{}
	if err := runMemoryTypeEnvSelect(
		genesisTestCommand(selectOutput),
		nil,
	); err != nil {
		t.Fatalf("runMemoryTypeEnvSelect(Transition): %v", err)
	}
	selectedEnvelope := decodeGenesisSelectionEnvelope(t, selectOutput.Bytes())
	selected := projectTypeEnvGenesisFreshlyCommitted{}
	if err := json.Unmarshal(selectedEnvelope.Outcome, &selected); err != nil {
		t.Fatalf("decode Transition selection outcome: %v", err)
	}
	if selectedEnvelope.ContractVersion != "haft.project-typeenv.transition-selection/v1" ||
		selectedEnvelope.AuthorityIngress != "explicit_h_decide" ||
		selected.Kind != "freshly_committed" ||
		selected.CommittedClosure.HeadRevision != 2 ||
		selected.CommittedClosure.CommittedGraphRevision != 2 ||
		selected.CommittedClosure.SelectedCompositeRef != prepared.Candidate.CompositeTypeEnvRef {
		t.Fatalf("Transition selection response = %#v / %#v", selectedEnvelope, selected)
	}
	if current := dualRuntime.target.Composite().Ref().String(); current != prepared.Candidate.CompositeTypeEnvRef {
		t.Fatalf(
			"catalog current target C = %s, want prepared target %s",
			current,
			prepared.Candidate.CompositeTypeEnvRef,
		)
	}
	assertProjectCurrentValidationUsesComposite(
		t,
		dualRuntime.readOnly,
		prepared.Candidate.CompositeTypeEnvRef,
	)

	replayOutput := &bytes.Buffer{}
	if err := runMemoryTypeEnvSelect(
		genesisTestCommand(replayOutput),
		nil,
	); err != nil {
		t.Fatalf("runMemoryTypeEnvSelect(Transition replay): %v", err)
	}
	replayEnvelope := decodeGenesisSelectionEnvelope(t, replayOutput.Bytes())
	replayed := projectTypeEnvGenesisReplayedExisting{}
	if err := json.Unmarshal(replayEnvelope.Outcome, &replayed); err != nil {
		t.Fatalf("decode Transition replay: %v", err)
	}
	if replayed.Kind != "replayed_existing" ||
		replayed.CommittedClosure.ReceiptRef != selected.CommittedClosure.ReceiptRef {
		t.Fatalf("Transition replay = %#v", replayEnvelope)
	}

	assertGenesisSelectionTableCount(t, harness, "project_typeenv_stages", 2)
	prepareAgain := &bytes.Buffer{}
	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(prepareAgain),
		nil,
	); err != nil {
		t.Fatalf("runMemoryTypeEnvPrepare(already selected): %v", err)
	}
	already := projectTypeEnvTransitionAlreadySelectedResponse{}
	if err := json.Unmarshal(prepareAgain.Bytes(), &already); err != nil {
		t.Fatalf("decode already-selected response: %v", err)
	}
	if already.Result != "already_selected" ||
		already.Current.HeadRevision != 2 ||
		already.Current.CompositeTypeEnv == priorComposite {
		t.Fatalf("already-selected response = %#v", already)
	}
	assertGenesisSelectionTableCount(t, harness, "project_typeenv_stages", 2)
	assertGenesisSelectionTableCount(t, harness, "project_typeenv_heads", 1)
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		2,
	)
}

func TestMemoryTypeEnvTransitionRejectsTamperedReadableReview(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "memory-typeenv-transition-tamper")
	t.Setenv(envProjectRoot, harness.Root().String())
	t.Setenv(envExpectedProjectID, harness.ProjectID())
	seedPriorProjectTypeEnvHead(t, harness)

	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err != nil {
		t.Fatalf("prepare Transition review: %v", err)
	}
	observed, err := observeProjectTypeEnvTransitionReview(harness.Root().String())
	if err != nil {
		t.Fatalf("observe Transition review: %v", err)
	}
	tampered := observed.carrier
	tampered.Review.Choice = "Choose a different, unreviewed ontology"
	if _, err := installProjectTypeEnvTransitionReview(
		harness.Root().String(),
		tampered,
		true,
	); err != nil {
		t.Fatalf("install tampered Transition review: %v", err)
	}
	if err := runMemoryTypeEnvSelect(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err == nil {
		t.Fatal("tampered readable Transition review reached selection")
	}
	assertGenesisSelectionTableCount(
		t,
		harness,
		"project_typeenv_head_selection_receipts",
		1,
	)
}

type transitionPrepareTestResponse struct {
	ContractVersion string                                    `json:"contract_version"`
	Result          string                                    `json:"result"`
	ProjectID       string                                    `json:"project_id"`
	Review          projectTypeEnvTransitionHumanReview       `json:"review"`
	Candidate       projectTypeEnvTransitionCandidateResponse `json:"candidate"`
	ReviewCarrier   projectTypeEnvGenesisReviewCarrierRef     `json:"review_carrier"`
	Interpretation  projectTypeEnvGenesisReviewInterpretation `json:"interpretation"`
}

func seedPriorProjectTypeEnvHead(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
) localpracticeruntime.Target {
	t.Helper()
	ctx := context.Background()
	ledger, _, err := openProjectTypeEnvGenesisLedger(ctx, projectledger.ReadWrite)
	if err != nil {
		t.Fatalf("open seed project ledger: %v", err)
	}
	defer func() {
		if err := ledger.Close(); err != nil {
			t.Fatalf("close seed project ledger: %v", err)
		}
	}()
	priorTarget := transitionPriorTarget(t)
	baseSnapshot, err := projectmemory.NewBaseTypeEnvSnapshot(priorTarget.Base())
	if err != nil {
		t.Fatalf("construct seed base snapshot: %v", err)
	}
	clock := typedmemorystore.SystemClock{}
	initializer, err := typedmemorystore.NewSQLiteProjectGraphInitializer(
		ledger.Database(),
		projectmemory.NewBaseTypeEnvLoader(),
		clock,
	)
	if err != nil {
		t.Fatalf("open seed graph initializer: %v", err)
	}
	if _, err := initializer.InitializeProjectGraphAtBaseTypeEnv(
		ctx,
		ledger.ProjectID(),
		baseSnapshot,
	); err != nil {
		t.Fatalf("initialize seed graph: %v", err)
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, ledger.Database())
	if err != nil {
		t.Fatalf("begin seed basis read: %v", err)
	}
	graph, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		ledger.ProjectID(),
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load seed graph basis: %v", err)
	}
	profile, err := profilebasissqlite.LoadCurrentWithin(
		ctx,
		transaction,
		harness.Root(),
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load seed profile basis: %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit seed basis read: %v", finish.Err())
	}
	candidate, err := projecttypeenvpreparation.PrepareGenesisCandidate(
		projecttypeenvpreparation.GenesisCandidateInput{
			Project:        ledger.ProjectID(),
			ProjectRoot:    harness.Root(),
			Target:         priorTarget,
			CurrentGraph:   graph,
			CurrentProfile: profile,
		},
	)
	if err != nil {
		t.Fatalf("prepare seed Genesis candidate: %v", err)
	}
	stageStore, err := projecttypeenvstage.OpenExisting(ctx, ledger.Database())
	if err != nil {
		t.Fatalf("open seed Stage store: %v", err)
	}
	if err := stageStore.PutArtifactClosure(ctx, candidate.ArtifactClosure()); err != nil {
		t.Fatalf("persist seed closure: %v", err)
	}
	if err := stageStore.Put(
		ctx,
		candidate.Stage(),
		candidate.Verification().Record(),
		candidate.ExecutableSnapshot().Record(),
	); err != nil {
		t.Fatalf("persist seed Stage: %v", err)
	}
	prepared, err := sealProjectTypeEnvGenesisReview(
		candidate,
		"prepared_at_new_base",
		clock.Now(),
	)
	if err != nil {
		t.Fatalf("seal seed Genesis review: %v", err)
	}
	request, content, _, err := decodeProjectTypeEnvGenesisReview(
		ctx,
		ledger,
		prepared.carrier,
	)
	if err != nil {
		t.Fatalf("decode seed Genesis review: %v", err)
	}
	service, err := selectionsqlite.NewGenesisService(
		ctx,
		ledger.Database(),
		harness.Root().String(),
		priorTarget.InstalledRuntime(),
		clock,
	)
	if err != nil {
		t.Fatalf("open seed Genesis selection service: %v", err)
	}
	result, err := service.SelectGenesis(
		ctx,
		selectionsqlite.GenesisSelectionInput{
			Request:   request,
			Content:   content,
			Authority: selectionsqlite.NewDedicatedCLIInvocation(),
		},
	)
	if err != nil {
		t.Fatalf("select seed Genesis head: %v", err)
	}
	if _, ok := result.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf("seed Genesis selection = %T", result)
	}
	return priorTarget
}

func transitionPriorSource(t *testing.T) []byte {
	t.Helper()
	return typedmemorycandidates.SourceV1_2()
}

func transitionPriorTarget(t *testing.T) localpracticeruntime.Target {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef(basetypeenvartifacts.HistoricalV3Ref)
	if err != nil {
		t.Fatalf("parse historical transition Base reference: %v", err)
	}
	base, err := basetypeenvartifacts.LoadExact(ref)
	if err != nil {
		t.Fatalf("load historical transition Base: %v", err)
	}
	target, err := localpracticeruntime.Build(base, transitionPriorSource(t))
	if err != nil {
		t.Fatalf("build historical transition target: %v", err)
	}
	return target
}

func projectCurrentValidationPayload() []byte {
	return bytes.Replace(
		bundledMemoryValidationFixture(),
		[]byte(`{"kind":"bundled_candidate_open_world"}`),
		[]byte(`{"kind":"project_current"}`),
		1,
	)
}

func assertProjectCurrentValidationUsesComposite(
	t *testing.T,
	runtime projectMemoryReadRuntime,
	want string,
) {
	t.Helper()
	result, err := runtime.Validate(
		context.Background(),
		projectCurrentValidationPayload(),
	)
	if err != nil {
		t.Fatalf("Validate(project_current at %s) error = %v", want, err)
	}
	response := struct {
		Basis struct {
			ResolutionKind string `json:"resolution_kind"`
			TypeEnvRef     string `json:"type_env_ref"`
		} `json:"basis"`
	}{}
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("decode project_current validation at %s: %v", want, err)
	}
	if response.Basis.ResolutionKind != "resolved_project_basis" ||
		response.Basis.TypeEnvRef != want {
		t.Fatalf(
			"project_current basis = %#v, want resolved C %s",
			response.Basis,
			want,
		)
	}
}
