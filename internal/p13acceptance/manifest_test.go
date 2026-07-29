package p13acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	manifestRelativePath             = "internal/p13acceptance/manifest.json"
	manifestSchema                   = "haft.p13.acceptance-manifest/v3"
	manifestStatusPendingFinalSource = "pending_final_source"
	manifestStatusPendingSelection   = "pending_selected_basis"
	manifestStatusFrozen             = "frozen_for_execution"
	freezePosturePendingFinalSource  = "pending_final_source"
	freezePosturePendingSelection    = "pending_manual_selection"
	freezePostureSelectedAndFrozen   = "selected_and_frozen"
	modulePath                       = "github.com/m0n0x41d/haft"
	consolidatedOuterTimeoutSeconds  = 12 * 60 * 60
	consolidatedCommand              = "HAFT_P13_RUN_CONSOLIDATED=1 go test -count=1 -timeout=12h -v ./internal/p13acceptance -run '^TestP13ConsolidatedAcceptance$'"
	transitionSelectionRequestSchema = "haft.project-typeenv.head-selection-request.v2"
	p13PlanRelativePath              = ".context/haft-v9-deterministic-closeout.plan.md"
	supersededP13PlanRelativePath    = ".context/haft-v9-typed-memory-e2e-master-plan.md"
	p13ReadmeRelativePath            = "internal/p13acceptance/README.md"
	p13WorkflowRelativePath          = ".github/workflows/ci.yml"
	p13PlanSpan                      = "heading:### D2 — freeze and run one P13"
	p13PlanHeading                   = "### D2 — freeze and run one P13"
	acceptanceMatrixSpan             = "manifest:gates"
	p13RemoteCloseoutRequirement     = `test -f "$repository_tree/.context/haft-v9-deterministic-closeout.plan.md"`
	releaseBlockerRelativePath       = ".context/current-plan-issue-report.md"
	releaseBlockerSpan               = "document:full"
	releaseBlockerHeading            = "# V9 release blocker: internal FPF Query provenance leaks into MCP working responses"
	requiredPriorHeadRevision        = int64(1)
	requiredPriorCompositeRef        = "typeenv:sha256:d6097b7231aee200a0b998bd4146496b796222917e1e16505ac897079b7f29c2"
	requiredPriorBaseRef             = "typeenv:sha256:aa1eec077868e611108810f1e4bc187d55eb38e3bc705cc149a098008b58cd1a"
	requiredPriorBaseDigest          = "sha256:aa1eec077868e611108810f1e4bc187d55eb38e3bc705cc149a098008b58cd1a"
	requiredPriorFPFRevision         = "44dd88188a07646ef23aca32627a3f670525853f"
	requiredPriorCompilerSchema      = "fpf-base-typeenv.cov2.v2"
	requiredPriorSnapshotDigest      = "sha256:a38f3d1ab0eb3674a0153b5c38e705c69d68f5afed98bb928fa88019d527b6b5"
	requiredPriorLoweredDigest       = "sha256:a553e9edd919475393819b8eae6fed2129313ce9c5ee70f2fc4e23d1d8e73d73"
	requiredStageSchemaEdition       = "haft.project-typeenv.stage-schema/v5"
)

type privateP13BasisPosture string

const (
	privateP13BasisAbsent   privateP13BasisPosture = "absent"
	privateP13BasisPartial  privateP13BasisPosture = "partial"
	privateP13BasisComplete privateP13BasisPosture = "complete"
)

type privateP13BasisState struct {
	Posture privateP13BasisPosture
	Missing []string
}

var fullGitRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)
var goTestFunctionName = regexp.MustCompile(`^Test[A-Za-z0-9_]+$`)

var requiredPrivateP13BasisPaths = []string{
	".agents/skills",
	".context/current-plan-issue-report.md",
	p13PlanRelativePath,
	".haft/config.yaml",
	".haft/project-profile.yaml",
	".haft/project.yaml",
}

var expectedCriticalRaceCases = []goRaceCase{
	{
		Package: modulePath + "/db",
		Tests: []string{
			"TestAuthorityProfileReconciliationConvergesAcrossConcurrentConnections",
			"TestForeignKeyTableRebuildMigrationConcurrentRecheckAppliesOnce",
		},
	},
	{
		Package: modulePath + "/internal/codebase",
		Tests: []string{
			"TestConcurrentReaderObservesOnlyWholeEpochBasis",
			"TestIndexPublicationFailureRollsBackCandidateAndBasis",
			"TestRootAdmissionBudgetRetainsPriorEpoch",
		},
	},
	{
		Package: modulePath + "/internal/codeintel",
		Tests: []string{
			"TestDiscoverConcernRetriesAcrossConcurrentPublication",
			"TestPublishedExploreReplayMismatchNamesChangedBasis",
		},
	},
	{
		Package: modulePath + "/internal/initfs",
		Tests: []string{
			"TestHostPublisherConcurrentSameBindingReturnsBusyWithoutInterleaving",
			"TestHostPublisherRecoveryPreservesPostPartialLocalModification",
			"TestManifestStoreConcurrentFirstPersistPublishesExactlyOneManifest",
			"TestManifestStoreLeaseSerializesAWholePublicationWindow",
			"TestPublicationCoordinatorSerializesSharedRootAcrossProcesses",
		},
	},
	{
		Package: modulePath + "/internal/projectledger",
		Tests: []string{
			"TestConcurrentProjectLedgerBindingAdmitsExactlyOnePhysicalRoot",
			"TestProjectLedgerBindingReportsCommittedOutcomeWhenPathSwapsBeforePostCheck",
			"TestProjectLedgerBindingRollsBackWhenTopologyChangesBeforeCommit",
			"TestProjectLedgerPreservesSidecarGenerationAcrossIndependentWriterClose",
		},
	},
	{
		Package: modulePath + "/internal/projectmemory",
		Tests: []string{
			"TestAdmissionRuntimeClosesCommitUnknownAndSameKeyReplayWithoutSecondEffect",
			"TestCurrentReadRuntimeRejectsStaleProcessAcrossTypeEnvTransitionAndReusesRollbackSnapshot",
		},
	},
	{
		Package: modulePath + "/internal/projectmemory/identityreconciliation",
		Tests: []string{
			"TestReviewedIdentityConcurrentCASHasOneWinnerAndNoPartialLoser",
			"TestReviewedIdentityMergeCommitsReplaysResolvesAndLoadsCurrentSnapshot",
		},
	},
	{
		Package: modulePath + "/internal/projectmemory/neighborhoodcache",
		Tests: []string{
			"TestAtomicStoreConcurrentReadsRemainDeterministic",
			"TestCacheKeyInvalidatesEveryExactProjectionCoordinate",
			"TestReadThroughCacheHitAndMissAreByteIdentical",
			"TestTypeEnvTransitionMissesWhileRollbackCanReuseExactOldSnapshot",
		},
	},
	{
		Package: modulePath + "/internal/projecttypeenvheadstore",
		Tests: []string{
			"TestHeadCASIsCallerOwnedAtomicAndTransitionExact",
			"TestHeadCASRejectsReadTransactionAndDoesNotFinishIt",
		},
	},
	{
		Package: modulePath + "/internal/projecttypeenvselectioneffect/sqlite",
		Tests: []string{
			"TestGenesisServiceConcurrentSameKeyCommitsOnceAndReplaysExactly",
			"TestGenesisServiceMapsPriorHeadBeforeConcurrentGraphDrift",
			"TestTransitionServiceCommitsExactSuccessorAndReplays",
			"TestTransitionServiceRollbackSelectsPriorImmutableCWithoutDeletingAssertions",
		},
	},
	{
		Package: modulePath + "/internal/sqlitetransaction",
		Tests: []string{
			"TestFailedCommitCleansUpBeforeConnectionReuse",
			"TestImmediateTransactionOwnsEffectAndLifecycle",
			"TestReadTransactionRejectsMutationAndRollbackFinishes",
		},
	},
	{
		Package: modulePath + "/internal/typedmemorystore",
		Tests: []string{
			"TestCommitDeclareEntityRecoversAfterPhysicalCommitReportsFailure",
			"TestSQLiteAdapterTwoConnectionsCASOneExpectedRevisionZeroCommit",
			"TestSQLiteProjectGraphInitializerConcurrentExactCallsConverge",
		},
	},
}

type acceptanceManifest struct {
	Schema                string                 `json:"schema"`
	Status                string                 `json:"status"`
	PlanRef               string                 `json:"plan_ref"`
	P13PlanSpan           string                 `json:"p13_plan_span"`
	AcceptanceMatrixSpan  string                 `json:"acceptance_matrix_span"`
	ReleaseBlockerRef     string                 `json:"release_blocker_ref"`
	ReleaseBlockerSpan    string                 `json:"release_blocker_span"`
	ReleaseClaim          bool                   `json:"release_claim"`
	ResultSemantics       string                 `json:"result_semantics"`
	SingleCommand         string                 `json:"single_command"`
	Waivers               []manifestWaiver       `json:"waivers"`
	Identity              identitySpec           `json:"identity"`
	FreezeInput           freezeInputSpec        `json:"freeze_input"`
	Suites                []suiteSpec            `json:"suites"`
	PlannedGraphContracts []plannedGraphContract `json:"planned_graph_contracts"`
	Gates                 []gateSpec             `json:"gates"`
}

type manifestWaiver struct {
	GateID string `json:"gate_id"`
	Reason string `json:"reason"`
}

type identitySpec struct {
	RequiredSchemaVersion      int             `json:"required_schema_version"`
	RequiredWriterGeneration   int             `json:"required_writer_generation"`
	RequiredFPF                fpfIdentitySpec `json:"required_fpf"`
	RequiredPredecessor        predecessorSpec `json:"required_predecessor"`
	RequiredExcludedGoPackages []string        `json:"required_excluded_go_packages"`
	SourceRoots                []string        `json:"source_roots"`
	SourceFiles                []string        `json:"source_files"`
	IncludeAllFiles            bool            `json:"include_all_files"`
	IncludeGoBuildInputs       bool            `json:"include_go_build_inputs"`
	ExcludedDirectories        []string        `json:"excluded_directories"`
	DependencyRoots            []string        `json:"dependency_roots"`
}

type fpfIdentitySpec struct {
	Revision          string `json:"revision"`
	SpecDigest        string `json:"spec_digest"`
	ReadmeDigest      string `json:"readme_digest"`
	BaseTypeEnvRef    string `json:"base_type_env_ref"`
	BaseTypeEnvDigest string `json:"base_type_env_digest"`
	CompilerSchema    string `json:"compiler_schema"`
}

type predecessorSpec struct {
	HeadRevision             int64  `json:"head_revision"`
	CompositeRef             string `json:"composite_ref"`
	BaseTypeEnvRef           string `json:"base_type_env_ref"`
	BaseTypeEnvDigest        string `json:"base_type_env_digest"`
	FPFRevision              string `json:"fpf_revision"`
	CompilerSchema           string `json:"compiler_schema"`
	ExecutableSnapshotDigest string `json:"executable_snapshot_digest"`
	LoweredEnvironmentDigest string `json:"lowered_environment_digest"`
}

type freezeInputSpec struct {
	Posture                     string `json:"posture"`
	ProfileGeneration           string `json:"profile_generation"`
	ProfileLedgerRevision       int64  `json:"profile_ledger_revision"`
	ProfilePayloadDigest        string `json:"profile_payload_digest"`
	ProfileAdmissionRef         string `json:"profile_admission_ref"`
	ProfileAdmissionDigest      string `json:"profile_admission_digest"`
	ProfileProjectionDigest     string `json:"profile_projection_digest"`
	ProfileProjectionSchema     string `json:"profile_projection_schema"`
	ProfileProjectionLedgerHead int64  `json:"profile_projection_ledger_head"`
	ProfileBasisRef             string `json:"profile_basis_ref"`
	ProfileBasisDigest          string `json:"profile_basis_digest"`
	ProfileLedgerDigest         string `json:"profile_ledger_digest"`
	ProfileSupportDAGDigest     string `json:"profile_support_dag_digest"`
	HeadRef                     string `json:"head_ref"`
	HeadRevision                int64  `json:"head_revision"`
	SelectedCompositeRef        string `json:"selected_composite_ref"`
	HeadStateDigest             string `json:"head_state_digest"`
	SelectionClosureRef         string `json:"selection_closure_ref"`
	SelectionClosureDigest      string `json:"selection_closure_digest"`
	SelectionRequestSchema      string `json:"selection_request_schema"`
	SelectionRequestRef         string `json:"selection_request_ref"`
	SelectionRequestDigest      string `json:"selection_request_digest"`
	SelectionStageRef           string `json:"selection_stage_ref"`
	SelectionStageDigest        string `json:"selection_stage_digest"`
	SelectionReadyClosureDigest string `json:"selection_ready_closure_digest"`
	SelectionPredecessorKind    string `json:"selection_predecessor_kind"`
	PriorHeadRef                string `json:"prior_head_ref"`
	PriorHeadRevision           int64  `json:"prior_head_revision"`
	PriorSelectedCompositeRef   string `json:"prior_selected_composite_ref"`
	PriorHeadStateDigest        string `json:"prior_head_state_digest"`
	PriorCompositeDigest        string `json:"prior_composite_digest"`
	PriorBaseRef                string `json:"prior_base_ref"`
	PriorBaseDigest             string `json:"prior_base_digest"`
	PriorFPFRevision            string `json:"prior_fpf_revision"`
	PriorCompilerSchema         string `json:"prior_compiler_schema"`
	PriorExecutableDigest       string `json:"prior_executable_snapshot_digest"`
	PriorLoweredDigest          string `json:"prior_lowered_environment_digest"`
	TargetBaseRef               string `json:"target_base_ref"`
	TargetExtensionsDigest      string `json:"target_ordered_extensions_digest"`
	TargetRuntimeBasisRef       string `json:"target_runtime_basis_ref"`
	TargetCompositeRef          string `json:"target_composite_ref"`
	SelectionReceiptRef         string `json:"selection_receipt_ref"`
	SelectionReceiptDigest      string `json:"selection_receipt_digest"`
	SelectionAuthorityUseRef    string `json:"selection_authority_use_ref"`
	SelectionAuthorityUseDigest string `json:"selection_authority_use_digest"`
	SelectionGraphRevision      int64  `json:"selection_graph_revision"`
	SelectionGraphEventRef      string `json:"selection_graph_event_ref"`
	SelectionGraphCommitRef     string `json:"selection_graph_commit_ref"`
	StageSchemaEdition          string `json:"stage_schema_edition"`
	StageProfileLedgerRevision  int64  `json:"stage_profile_ledger_revision"`
	StageProfileLedgerDigest    string `json:"stage_profile_ledger_digest"`
	StageProfileFitRef          string `json:"stage_profile_fit_ref"`
	StageProfileFitDigest       string `json:"stage_profile_fit_digest"`
	StageProfileFitRuleEdition  string `json:"stage_profile_fit_rule_edition"`
	StageProfileFitPosture      string `json:"stage_profile_fit_posture"`
	TransitionProfilesRef       string `json:"transition_profiles_ref"`
	TransitionProfilesDigest    string `json:"transition_profiles_digest"`
	TransitionProfileSetDigest  string `json:"transition_profile_set_digest"`
	TransitionProfileCount      int    `json:"transition_profile_count"`
	TransitionPosturesDigest    string `json:"transition_postures_digest"`
	GraphRevision               int64  `json:"graph_revision"`
	GraphActiveTypeEnvRef       string `json:"graph_active_type_env_ref"`
	GraphSnapshotBasisRef       string `json:"graph_snapshot_basis_ref"`
	GraphSnapshotBasisDigest    string `json:"graph_snapshot_basis_digest"`
	GraphLastEventRef           string `json:"graph_last_event_ref"`
	GraphLastCommitRef          string `json:"graph_last_commit_ref"`
	GraphMaterializationDigest  string `json:"graph_materialization_digest"`
}

type suiteSpec struct {
	ID                   string       `json:"id"`
	Kind                 string       `json:"kind"`
	Program              string       `json:"program,omitempty"`
	Args                 []string     `json:"args,omitempty"`
	WorkingDirectory     string       `json:"working_directory,omitempty"`
	GoRaceCases          []goRaceCase `json:"go_race_cases,omitempty"`
	GoPackageParallelism int          `json:"go_package_parallelism,omitempty"`
	GoTestProcs          int          `json:"go_test_procs,omitempty"`
	TimeoutSeconds       int          `json:"timeout_seconds"`
}

type goRaceCase struct {
	Package string   `json:"package"`
	Tests   []string `json:"tests"`
}

type gateSpec struct {
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	PlanSpan string       `json:"plan_span"`
	Claims   []string     `json:"claims"`
	SuiteIDs []string     `json:"suite_ids"`
	Anchors  []testAnchor `json:"anchors"`
}

type plannedGraphContract struct {
	ID               string       `json:"id"`
	OwnerWorkPackage string       `json:"owner_work_package"`
	DetailedItems    []string     `json:"detailed_items"`
	ActivationGate   string       `json:"activation_gate"`
	Claim            string       `json:"claim"`
	IntendedAnchors  []testAnchor `json:"intended_anchors"`
	Status           string       `json:"status"`
	EvidenceRef      string       `json:"evidence_ref"`
}

type testAnchor struct {
	Package string `json:"package"`
	Test    string `json:"test"`
}

type gateContract struct {
	ID           string
	Title        string
	PlanSpan     string
	ClaimsDigest string
	SuiteIDs     []string
	AnchorKeys   []string
}

func TestP13ManifestStructureAndAnchors(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	privateBasis, err := inspectPrivateP13Basis(root)
	if err != nil {
		t.Fatal(err)
	}
	switch privateBasis.Posture {
	case privateP13BasisAbsent:
		t.Skip("private P13 basis is absent; use the frozen-basis preflight")
	case privateP13BasisPartial:
		t.Fatalf("private P13 basis is partial; missing %v", privateBasis.Missing)
	case privateP13BasisComplete:
	default:
		t.Fatalf("unknown private P13 basis posture %q", privateBasis.Posture)
	}
	manifest, raw, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAcceptanceManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := validateP13ReadmeCurrentness(root, manifest.Identity); err != nil {
		t.Fatal(err)
	}
	if err := validateP13WorkflowCurrentness(root); err != nil {
		t.Fatal(err)
	}
	relevantPaths, err := collectRelevantPaths(root, manifest.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRequiredIdentityPaths(relevantPaths); err != nil {
		t.Fatal(err)
	}
	digest := sha256Prefixed(raw)
	t.Logf("P13 manifest status=%s digest=%s", manifest.Status, digest)
}

func TestP13ActiveProvenanceUsesDeterministicCloseout(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.PlanRef != p13PlanRelativePath ||
		manifest.P13PlanSpan != p13PlanSpan ||
		manifest.AcceptanceMatrixSpan != acceptanceMatrixSpan {
		t.Fatalf(
			"P13 active provenance = %q %q %q",
			manifest.PlanRef,
			manifest.P13PlanSpan,
			manifest.AcceptanceMatrixSpan,
		)
	}
	if slices.Contains(
		manifest.Identity.SourceFiles,
		supersededP13PlanRelativePath,
	) {
		t.Fatal("P13 identity treats the superseded master plan as active")
	}
	if !slices.Contains(manifest.Identity.SourceFiles, p13PlanRelativePath) {
		t.Fatal("P13 identity omits the deterministic closeout carrier")
	}
	for _, gate := range manifest.Gates {
		if gate.PlanSpan != p13PlanSpan {
			t.Fatalf(
				"P13 gate %q uses stale plan span %q",
				gate.ID,
				gate.PlanSpan,
			)
		}
	}
	if err := validateP13WorkflowCurrentness(root); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateP13BasisPostureSeparatesAbsentPartialAndComplete(t *testing.T) {
	root := t.TempDir()
	absent, err := inspectPrivateP13Basis(root)
	if err != nil {
		t.Fatal(err)
	}
	if absent.Posture != privateP13BasisAbsent {
		t.Fatalf("empty basis posture = %q", absent.Posture)
	}
	if err := materializePrivateP13BasisFixture(
		root,
		requiredPrivateP13BasisPaths[:1],
	); err != nil {
		t.Fatal(err)
	}
	partial, err := inspectPrivateP13Basis(root)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Posture != privateP13BasisPartial {
		t.Fatalf("partial basis posture = %q", partial.Posture)
	}
	if err := materializePrivateP13BasisFixture(
		root,
		requiredPrivateP13BasisPaths[1:],
	); err != nil {
		t.Fatal(err)
	}
	complete, err := inspectPrivateP13Basis(root)
	if err != nil {
		t.Fatal(err)
	}
	if complete.Posture != privateP13BasisComplete {
		t.Fatalf("complete basis posture = %q", complete.Posture)
	}
	if len(complete.Missing) != 0 {
		t.Fatalf("complete basis has missing paths %v", complete.Missing)
	}
}

func inspectPrivateP13Basis(root string) (privateP13BasisState, error) {
	missing := make([]string, 0, len(requiredPrivateP13BasisPaths))
	for _, relativePath := range requiredPrivateP13BasisPaths {
		_, err := os.Stat(filepath.Join(root, relativePath))
		switch {
		case err == nil:
		case os.IsNotExist(err):
			missing = append(missing, relativePath)
		default:
			return privateP13BasisState{}, fmt.Errorf(
				"inspect private P13 basis %s: %w",
				relativePath,
				err,
			)
		}
	}
	switch len(missing) {
	case 0:
		return privateP13BasisState{
			Posture: privateP13BasisComplete,
		}, nil
	case len(requiredPrivateP13BasisPaths):
		return privateP13BasisState{
			Posture: privateP13BasisAbsent,
			Missing: missing,
		}, nil
	default:
		return privateP13BasisState{
			Posture: privateP13BasisPartial,
			Missing: missing,
		}, nil
	}
}

func materializePrivateP13BasisFixture(
	root string,
	relativePaths []string,
) error {
	for _, relativePath := range relativePaths {
		path := filepath.Join(root, relativePath)
		if filepath.Ext(relativePath) == "" {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("fixture\n"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func TestP13ExactGateContractRejectsSameCountAnchorSubstitution(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	gates := slices.Clone(manifest.Gates)
	gates[0].Anchors = slices.Clone(gates[0].Anchors)
	originalCount := len(gates[0].Anchors)
	substitute := manifest.Gates[1].Anchors[0]
	gates[0].Anchors[0] = substitute
	if len(gates[0].Anchors) != originalCount {
		t.Fatal("test setup changed the gate anchor count")
	}
	err = validateExactGateContract(gates)
	if err == nil {
		t.Fatal("same-count anchor substitution passed the exact gate contract")
	}
	if !strings.Contains(err.Error(), "anchor keys") {
		t.Fatalf("same-count substitution returned unrelated error: %v", err)
	}
}

func TestP13PlannedGraphContractsRejectSameCountAnchorSubstitution(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	contracts := slices.Clone(manifest.PlannedGraphContracts)
	contracts[0].IntendedAnchors = slices.Clone(contracts[0].IntendedAnchors)
	contracts[0].IntendedAnchors[0] = contracts[1].IntendedAnchors[0]
	if err := validatePlannedGraphContracts(
		root,
		contracts,
		manifest.Gates,
	); err == nil {
		t.Fatal("P13 planned graph contract accepted an anchor substitution")
	}
}

func TestP13ManifestStateSeparatesSourceSelectionFromExecutionFreeze(t *testing.T) {
	pendingSource := acceptanceManifest{
		Status: manifestStatusPendingFinalSource,
		FreezeInput: freezeInputSpec{
			Posture: freezePosturePendingFinalSource,
		},
	}
	if err := validateManifestState(pendingSource); err != nil {
		t.Fatalf("pending final source state rejected: %v", err)
	}

	finalSource := exactFPFIdentityForStateTest()
	pendingSelection := acceptanceManifest{
		Status: manifestStatusPendingSelection,
		Identity: identitySpec{
			RequiredFPF: finalSource,
		},
		FreezeInput: freezeInputSpec{
			Posture: freezePosturePendingSelection,
		},
	}
	if err := validateManifestState(pendingSelection); err != nil {
		t.Fatalf("pending selected basis state rejected: %v", err)
	}

	pendingSource.Identity.RequiredFPF = finalSource
	if err := validateManifestState(pendingSource); err == nil {
		t.Fatal("pending final source state retained a temporary target basis")
	}

	pendingSelection.Status = manifestStatusFrozen
	if err := validateManifestState(pendingSelection); err == nil {
		t.Fatal("unselected basis was labelled frozen for execution")
	}
}

func exactFPFIdentityForStateTest() fpfIdentitySpec {
	baseDigest := "sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6"
	return fpfIdentitySpec{
		Revision:          "0990ff1d1ccee4587b8f7e16e7a725a8edbe66b4",
		SpecDigest:        "sha256:1093a25640c61a2674f56443bffb8e27f33ac2cdf95f09af2c0cf67c68913eac",
		ReadmeDigest:      "sha256:6c8d87a641f36d34a9d84aa0ab8e7565dcca2a691482a0cee31bd28a743eb3fd",
		BaseTypeEnvRef:    "typeenv:" + baseDigest,
		BaseTypeEnvDigest: baseDigest,
		CompilerSchema:    "fpf-base-typeenv.cov2.v4",
	}
}

func validateRequiredIdentityPaths(paths []string) error {
	required := []string{
		"assurance/calculator.go",
		".context/current-plan-issue-report.md",
		p13PlanRelativePath,
		"data/FPF/FPF-Spec.md",
		"db/typed_memory_identity_reconciliation_migration_test.go",
		"docs/src/pages/docs/agent-prompt.astro",
		"internal/cli/fpf.db",
		"internal/p13acceptance/manifest.json",
		"internal/p13acceptance/source_naming_purity_test.go",
		"internal/projectmemory/architecturep2s/model.go",
		"internal/projectmemory/current_architecture_p2s.go",
		"internal/projectmemory/identityreconciliation/service_e2e_test.go",
		"internal/projecttypeenvselectioneffect/sqlite/production_golden_concern_bundle_e2e_test.go",
		"internal/typedmemory/genericity_acceptance_external_test.go",
		"internal/typedmemory/fpf_category_error_test.go",
		"logger/logger.go",
		"open-sleigh/lib/open_sleigh.ex",
		"packages/haft-pi/tests/pure.test.ts",
	}
	for _, path := range required {
		if !slices.Contains(paths, filepath.FromSlash(path)) {
			return fmt.Errorf("P13 identity scope omits required path %s", path)
		}
	}
	return nil
}

func repositoryRoot() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("read working directory: %w", err)
	}
	current := workingDirectory
	for {
		candidate := filepath.Join(current, "go.mod")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.Mode().IsRegular() {
			canonical, evalErr := filepath.EvalSymlinks(current)
			if evalErr != nil {
				return "", fmt.Errorf("canonicalize repository root: %w", evalErr)
			}
			return canonical, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("repository root with go.mod was not found")
		}
		current = parent
	}
}

func loadAcceptanceManifest(
	root string,
) (acceptanceManifest, []byte, error) {
	path := filepath.Join(root, manifestRelativePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return acceptanceManifest{}, nil, fmt.Errorf(
			"read P13 manifest: %w",
			err,
		)
	}
	manifest := acceptanceManifest{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return acceptanceManifest{}, nil, fmt.Errorf(
			"decode P13 manifest: %w",
			err,
		)
	}
	extra := json.RawMessage{}
	err = decoder.Decode(&extra)
	if err != io.EOF {
		return acceptanceManifest{}, nil, fmt.Errorf(
			"P13 manifest contains trailing JSON content",
		)
	}
	return manifest, raw, nil
}

func validateAcceptanceManifest(
	root string,
	manifest acceptanceManifest,
) error {
	if manifest.Schema != manifestSchema {
		return fmt.Errorf("P13 manifest schema = %q", manifest.Schema)
	}
	if manifest.ReleaseClaim {
		return fmt.Errorf("P13 manifest makes a release claim")
	}
	if len(manifest.Waivers) != 0 {
		return fmt.Errorf("P13 manifest contains %d waiver(s)", len(manifest.Waivers))
	}
	if manifest.PlanRef == "" ||
		manifest.P13PlanSpan == "" ||
		manifest.AcceptanceMatrixSpan == "" ||
		manifest.ReleaseBlockerRef == "" ||
		manifest.ReleaseBlockerSpan == "" ||
		manifest.ResultSemantics == "" ||
		manifest.SingleCommand == "" {
		return fmt.Errorf("P13 manifest provenance or semantics are incomplete")
	}
	if manifest.PlanRef != p13PlanRelativePath ||
		manifest.P13PlanSpan != p13PlanSpan ||
		manifest.AcceptanceMatrixSpan != acceptanceMatrixSpan ||
		manifest.ReleaseBlockerRef != releaseBlockerRelativePath ||
		manifest.ReleaseBlockerSpan != releaseBlockerSpan {
		return fmt.Errorf("P13 manifest plan coordinates differ from the acceptance contract")
	}
	if err := validateP13ProvenanceAnchors(root, manifest); err != nil {
		return err
	}
	if manifest.SingleCommand != consolidatedCommand {
		return fmt.Errorf(
			"P13 single command = %q, want %q",
			manifest.SingleCommand,
			consolidatedCommand,
		)
	}
	if err := validateManifestState(manifest); err != nil {
		return err
	}
	if err := validateIdentitySpec(manifest.Identity); err != nil {
		return err
	}
	suiteByID, err := validateSuites(manifest.Suites)
	if err != nil {
		return err
	}
	if err := validateCriticalRaceCaseTests(root, suiteByID["go_race"]); err != nil {
		return err
	}
	if err := validateConsolidatedOuterTimeout(manifest.Suites); err != nil {
		return err
	}
	if err := validateGates(root, manifest.Gates, suiteByID); err != nil {
		return err
	}
	if err := validatePlannedGraphContracts(
		root,
		manifest.PlannedGraphContracts,
		manifest.Gates,
	); err != nil {
		return err
	}
	return validateEverySuiteIsMapped(manifest.Gates, manifest.Suites)
}

func validateConsolidatedOuterTimeout(suites []suiteSpec) error {
	totalSuiteTimeoutSeconds := 0
	for _, suite := range suites {
		totalSuiteTimeoutSeconds += suite.TimeoutSeconds
	}
	if totalSuiteTimeoutSeconds >= consolidatedOuterTimeoutSeconds {
		return fmt.Errorf(
			"P13 consolidated outer timeout = %d seconds, want greater than suite budget %d",
			consolidatedOuterTimeoutSeconds,
			totalSuiteTimeoutSeconds,
		)
	}
	return nil
}

func validatePlannedGraphContracts(
	root string,
	contracts []plannedGraphContract,
	gates []gateSpec,
) error {
	expected := expectedPlannedGraphContracts()
	if len(contracts) != len(expected) {
		return fmt.Errorf(
			"P13 planned graph contract count = %d, want %d",
			len(contracts),
			len(expected),
		)
	}
	seen := make(map[string]struct{}, len(contracts))
	for index, contract := range contracts {
		want := expected[index]
		if _, duplicate := seen[contract.ID]; duplicate {
			return fmt.Errorf("P13 planned graph contract %q is duplicated", contract.ID)
		}
		seen[contract.ID] = struct{}{}
		if contract.ID != want.ID ||
			contract.OwnerWorkPackage != want.OwnerWorkPackage ||
			contract.ActivationGate != want.ActivationGate ||
			contract.Claim != want.Claim ||
			contract.Status != want.Status ||
			contract.EvidenceRef != want.EvidenceRef {
			return fmt.Errorf(
				"P13 planned graph contract %d identity or policy differs",
				index,
			)
		}
		if !slices.Equal(contract.DetailedItems, want.DetailedItems) {
			return fmt.Errorf(
				"P13 planned graph contract %q detailed items differ",
				contract.ID,
			)
		}
		observedAnchors := anchorKeys(contract.IntendedAnchors)
		expectedAnchors := anchorKeys(want.IntendedAnchors)
		if !slices.Equal(observedAnchors, expectedAnchors) {
			return fmt.Errorf(
				"P13 planned graph contract %q intended anchors differ",
				contract.ID,
			)
		}
		for _, anchor := range contract.IntendedAnchors {
			if !strings.HasPrefix(anchor.Package, modulePath+"/internal/") ||
				!strings.HasPrefix(anchor.Test, "Test") {
				return fmt.Errorf(
					"P13 planned graph contract %q has invalid intended anchor",
					contract.ID,
				)
			}
		}
		if err := validatePlannedGraphContractMaterialization(
			root,
			contract,
			gates,
		); err != nil {
			return err
		}
	}
	return nil
}

func validatePlannedGraphContractMaterialization(
	root string,
	contract plannedGraphContract,
	gates []gateSpec,
) error {
	if contract.Status == "planned_not_executable" {
		for _, anchor := range contract.IntendedAnchors {
			exists, err := testAnchorExists(root, anchor)
			if err != nil {
				return fmt.Errorf(
					"P13 planned graph contract %q anchor %s: %w",
					contract.ID,
					anchorKey(anchor),
					err,
				)
			}
			if exists {
				return fmt.Errorf(
					"P13 planned graph contract %q anchor %s exists but status remains planned_not_executable",
					contract.ID,
					anchorKey(anchor),
				)
			}
		}
		return nil
	}
	if contract.Status != "implemented_anchored" {
		return fmt.Errorf(
			"P13 graph contract %q has unsupported status %q",
			contract.ID,
			contract.Status,
		)
	}
	if err := validateGraphContractEvidenceCarrier(
		root,
		contract,
	); err != nil {
		return err
	}
	var activationGate *gateSpec
	for index := range gates {
		if gates[index].ID == contract.ActivationGate {
			activationGate = &gates[index]
			break
		}
	}
	if activationGate == nil {
		return fmt.Errorf(
			"P13 graph contract %q names missing activation gate %q",
			contract.ID,
			contract.ActivationGate,
		)
	}
	activationAnchors := anchorKeys(activationGate.Anchors)
	for _, anchor := range contract.IntendedAnchors {
		key := anchorKey(anchor)
		exists, err := testAnchorExists(root, anchor)
		if err != nil {
			return fmt.Errorf(
				"P13 graph contract %q anchor %s: %w",
				contract.ID,
				key,
				err,
			)
		}
		if !exists {
			return fmt.Errorf(
				"P13 graph contract %q implemented anchor %s does not exist",
				contract.ID,
				key,
			)
		}
		if !slices.Contains(activationAnchors, key) {
			return fmt.Errorf(
				"P13 graph contract %q implemented anchor %s is absent from activation gate %q",
				contract.ID,
				key,
				contract.ActivationGate,
			)
		}
	}
	return nil
}

func validateGraphContractEvidenceCarrier(
	root string,
	contract plannedGraphContract,
) error {
	clean := filepath.Clean(contract.EvidenceRef)
	parentPrefix := ".." + string(filepath.Separator)
	if filepath.IsAbs(clean) ||
		clean == "." ||
		clean == ".." ||
		strings.HasPrefix(clean, parentPrefix) {
		return fmt.Errorf(
			"P13 graph contract %q evidence ref is outside the repository",
			contract.ID,
		)
	}
	path := filepath.Join(root, clean)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf(
			"P13 graph contract %q evidence carrier: %w",
			contract.ID,
			err,
		)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf(
			"P13 graph contract %q evidence carrier is not a regular file",
			contract.ID,
		)
	}
	return nil
}

func expectedPlannedGraphContracts() []plannedGraphContract {
	const (
		implementedStatus      = "implemented_anchored"
		implementedEvidenceRef = ".context/v9-remaining-evidence/r7-p13/graph-anchor-closure.md"
		hg8EvidenceRef         = ".context/v9-remaining-evidence/r7-p13/hg8b.md"
	)
	return []plannedGraphContract{
		{
			ID:               "hg1_hg2_typed_traversal",
			OwnerWorkPackage: "R3",
			DetailedItems:    []string{"HG1", "HG2"},
			ActivationGate:   "G7",
			Claim:            "seed and traversal outcomes remain typed, deterministic, and honest under faults and budgets",
			IntendedAnchors: []testAnchor{
				{Package: modulePath + "/internal/codeintel", Test: "TestHGTraversalOutcomeCorpus"},
				{Package: modulePath + "/internal/codeintel", Test: "TestExploreBagDirection_MixedOutcomes"},
			},
			Status:      implementedStatus,
			EvidenceRef: implementedEvidenceRef,
		},
		{
			ID:               "hg3_hg4_honest_index",
			OwnerWorkPackage: "R3",
			DetailedItems:    []string{"HG3", "HG4"},
			ActivationGate:   "G7",
			Claim:            "source admission, file disposition, coverage, and epoch publication cannot turn incomplete roots into absence",
			IntendedAnchors: []testAnchor{
				{Package: modulePath + "/internal/codebase", Test: "TestSourceAdmissionPureCorpus"},
				{Package: modulePath + "/internal/codebase", Test: "TestIndexPublicationFailureRollsBackCandidateAndBasis"},
			},
			Status:      implementedStatus,
			EvidenceRef: implementedEvidenceRef,
		},
		{
			ID:               "hg5_hg6_concern_discovery",
			OwnerWorkPackage: "R4",
			DetailedItems:    []string{"HG5", "HG6"},
			ActivationGate:   "G7R",
			Claim:            "concern discovery is deterministic, coverage-aware, fused with exact reasoning context, and allowed to abstain",
			IntendedAnchors: []testAnchor{
				{Package: modulePath + "/internal/codebase", Test: "TestConcernDiscoveryMeetsFrozenCodeNativeCorpus"},
				{Package: modulePath + "/internal/codeintel", Test: "TestConcernFusionMeetsFrozenReasoningToCodeCorpus"},
			},
			Status:      implementedStatus,
			EvidenceRef: implementedEvidenceRef,
		},
		{
			ID:               "hg7_projection_and_agent_use",
			OwnerWorkPackage: "R5",
			DetailedItems:    []string{"HG7", "HG8a"},
			ActivationGate:   "G7",
			Claim:            "one Explore envelope feeds bounded working, trace, and diagnostic projections with CLI, MCP, and carrier parity",
			IntendedAnchors: []testAnchor{
				{Package: modulePath + "/internal/codeintel", Test: "TestPublishedExploreTraceIsDeterministicAndDiagnosticIsExplicit"},
				{Package: modulePath + "/internal/cli", Test: "TestHandleQuintQueryExploreConcernReturnsEvidenceBearingCandidates"},
				{Package: modulePath + "/internal/initplanning", Test: "TestSkillComponentRendererDerivesHostCarriersFromOneBundle"},
			},
			Status:      implementedStatus,
			EvidenceRef: implementedEvidenceRef,
		},
		{
			ID:               "hg8_adversarial_acceptance",
			OwnerWorkPackage: "R7",
			DetailedItems:    []string{"HG8b"},
			ActivationGate:   "G8",
			Claim:            "the frozen graph corpus, fault matrix, determinism, and performance budgets pass on one source identity",
			IntendedAnchors: []testAnchor{
				{Package: modulePath + "/internal/p13acceptance", Test: "TestHGAdversarialAcceptance"},
			},
			Status:      implementedStatus,
			EvidenceRef: hg8EvidenceRef,
		},
	}
}

func validateP13ProvenanceAnchors(
	root string,
	manifest acceptanceManifest,
) error {
	plan, err := readP13Carrier(root, manifest.PlanRef, "plan")
	if err != nil {
		return err
	}
	if err := requireOneP13Anchor(plan, p13PlanHeading, "P13 plan section"); err != nil {
		return err
	}
	for _, gate := range manifest.Gates {
		if gate.PlanSpan != p13PlanSpan {
			return fmt.Errorf(
				"P13 gate %q plan anchor = %q, want %q",
				gate.ID,
				gate.PlanSpan,
				p13PlanSpan,
			)
		}
	}
	releaseBlocker, err := readP13Carrier(
		root,
		manifest.ReleaseBlockerRef,
		"release blocker",
	)
	if err != nil {
		return err
	}
	return requireOneP13Anchor(
		releaseBlocker,
		releaseBlockerHeading,
		"release blocker heading",
	)
}

func readP13Carrier(
	root string,
	relativePath string,
	label string,
) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read P13 %s carrier: %w", label, err)
	}
	return raw, nil
}

func validateP13ReadmeCurrentness(root string, identity identitySpec) error {
	readme, err := readP13Carrier(
		root,
		p13ReadmeRelativePath,
		"README",
	)
	if err != nil {
		return err
	}
	marker := fmt.Sprintf(
		"schema %d with its exact writer-%d marker",
		identity.RequiredSchemaVersion,
		identity.RequiredWriterGeneration,
	)
	readmeText := string(readme)
	fields := strings.Fields(readmeText)
	normalized := strings.Join(fields, " ")
	if !strings.Contains(normalized, marker) {
		return fmt.Errorf(
			"P13 README does not name current manifest identity %q",
			marker,
		)
	}
	if !strings.Contains(readmeText, p13PlanRelativePath) {
		return fmt.Errorf(
			"P13 README omits deterministic closeout carrier %q",
			p13PlanRelativePath,
		)
	}
	if strings.Contains(readmeText, supersededP13PlanRelativePath) {
		return fmt.Errorf(
			"P13 README treats superseded plan %q as active",
			supersededP13PlanRelativePath,
		)
	}
	return nil
}

func validateP13WorkflowCurrentness(root string) error {
	workflow, err := readP13Carrier(
		root,
		p13WorkflowRelativePath,
		"CI workflow",
	)
	if err != nil {
		return err
	}
	if bytes.Contains(workflow, []byte(supersededP13PlanRelativePath)) {
		return fmt.Errorf(
			"P13 workflow requires superseded plan %q",
			supersededP13PlanRelativePath,
		)
	}
	return requireOneP13Anchor(
		workflow,
		p13RemoteCloseoutRequirement,
		"remote deterministic-closeout requirement",
	)
}

func requireOneP13Anchor(
	carrier []byte,
	anchor string,
	label string,
) error {
	occurrences := bytes.Count(carrier, []byte(anchor))
	if occurrences != 1 {
		return fmt.Errorf(
			"P13 %s occurs %d times",
			label,
			occurrences,
		)
	}
	return nil
}

func validateIdentitySpec(spec identitySpec) error {
	if spec.RequiredSchemaVersion != 54 {
		return fmt.Errorf(
			"P13 schema version = %d, want exactly 54",
			spec.RequiredSchemaVersion,
		)
	}
	if spec.RequiredWriterGeneration != 54 {
		return fmt.Errorf(
			"P13 writer generation = %d, want exactly 54",
			spec.RequiredWriterGeneration,
		)
	}
	if err := validateRequiredPredecessorSpec(spec.RequiredPredecessor); err != nil {
		return err
	}
	if len(spec.RequiredExcludedGoPackages) != 0 {
		return fmt.Errorf(
			"P13 requires %d excluded Go package(s), want none",
			len(spec.RequiredExcludedGoPackages),
		)
	}
	if len(spec.SourceRoots) == 0 ||
		len(spec.SourceFiles) == 0 ||
		!spec.IncludeAllFiles ||
		!spec.IncludeGoBuildInputs ||
		len(spec.ExcludedDirectories) == 0 ||
		len(spec.DependencyRoots) == 0 {
		return fmt.Errorf("P13 relevant-byte scope is incomplete")
	}
	wantRoots := []string{
		".agents/skills",
		".github",
		".haft/specs",
		"assurance",
		"cmd",
		"data/FPF",
		"data/haft",
		"db",
		"docs/src",
		"internal",
		"logger",
		"open-sleigh",
		"packages/haft-pi",
		"scripts",
		"skills",
		"spec",
	}
	wantFiles := []string{
		".context/current-plan-issue-report.md",
		p13PlanRelativePath,
		".haft/config.yaml",
		".haft/project-profile.yaml",
		".haft/project.yaml",
		"AGENTS.md",
		"CHANGELOG.md",
		"CLAUDE.md",
		"MIGRATION-v8.md",
		"README.md",
		"Taskfile.yaml",
		"go.mod",
		"go.sum",
		"internal/cli/fpf.db",
		"open-sleigh/mix.exs",
		"open-sleigh/mix.lock",
		"package-lock.json",
		"package.json",
	}
	wantExcluded := []string{
		".git",
		"_build",
		"build",
		"deps",
		"dist",
		"node_modules",
		"target",
		"tmp",
	}
	wantDependencyRoots := []string{
		"open-sleigh/deps",
		"packages/haft-pi/node_modules",
	}
	if !slices.Equal(spec.SourceRoots, wantRoots) ||
		!slices.Equal(spec.SourceFiles, wantFiles) ||
		!slices.Equal(spec.ExcludedDirectories, wantExcluded) ||
		!slices.Equal(spec.DependencyRoots, wantDependencyRoots) {
		return fmt.Errorf("P13 relevant-byte scope differs from its closed contract")
	}
	return nil
}

func validateRequiredPredecessorSpec(spec predecessorSpec) error {
	want := predecessorSpec{
		HeadRevision:             requiredPriorHeadRevision,
		CompositeRef:             requiredPriorCompositeRef,
		BaseTypeEnvRef:           requiredPriorBaseRef,
		BaseTypeEnvDigest:        requiredPriorBaseDigest,
		FPFRevision:              requiredPriorFPFRevision,
		CompilerSchema:           requiredPriorCompilerSchema,
		ExecutableSnapshotDigest: requiredPriorSnapshotDigest,
		LoweredEnvironmentDigest: requiredPriorLoweredDigest,
	}
	if spec != want {
		return fmt.Errorf(
			"P13 predecessor identity differs from exact 44dd881 project-head closure",
		)
	}
	return nil
}

func validateRequiredFPFSpec(spec fpfIdentitySpec) error {
	if !fullGitRevision.MatchString(spec.Revision) {
		return fmt.Errorf("P13 required FPF revision is not one full lowercase Git revision")
	}
	digests := []string{
		spec.SpecDigest,
		spec.ReadmeDigest,
		spec.BaseTypeEnvDigest,
	}
	if !everyCanonicalSHA256(digests) {
		return fmt.Errorf("P13 required FPF digests are not canonical sha256 values")
	}
	if spec.BaseTypeEnvRef != "typeenv:"+spec.BaseTypeEnvDigest {
		return fmt.Errorf("P13 required Base TypeEnv ref/digest are inconsistent")
	}
	if spec.CompilerSchema == "" || strings.TrimSpace(spec.CompilerSchema) != spec.CompilerSchema {
		return fmt.Errorf("P13 required FPF compiler schema is not canonical")
	}
	return nil
}

func everyCanonicalSHA256(values []string) bool {
	for _, value := range values {
		raw := strings.TrimPrefix(value, "sha256:")
		if !validPrefixedSHA256(value) || raw != strings.ToLower(raw) {
			return false
		}
	}
	return true
}

func validateManifestState(manifest acceptanceManifest) error {
	switch manifest.Status {
	case manifestStatusPendingFinalSource:
		return validatePendingFinalSourceState(
			manifest.Identity.RequiredFPF,
			manifest.FreezeInput,
		)
	case manifestStatusPendingSelection:
		return validatePendingSelectionState(
			manifest.Identity.RequiredFPF,
			manifest.FreezeInput,
		)
	case manifestStatusFrozen:
		return validateFrozenManifestState(
			manifest.Identity.RequiredFPF,
			manifest.FreezeInput,
		)
	default:
		return fmt.Errorf("P13 manifest status = %q", manifest.Status)
	}
}

func validatePendingFinalSourceState(
	requiredFPF fpfIdentitySpec,
	freezeInput freezeInputSpec,
) error {
	if requiredFPF != (fpfIdentitySpec{}) {
		return fmt.Errorf("pending-final-source P13 manifest carries a selected FPF target")
	}
	want := freezeInputSpec{Posture: freezePosturePendingFinalSource}
	if freezeInput != want {
		return fmt.Errorf("pending-final-source P13 manifest carries selection coordinates")
	}
	return nil
}

func validatePendingSelectionState(
	requiredFPF fpfIdentitySpec,
	freezeInput freezeInputSpec,
) error {
	if err := validateRequiredFPFSpec(requiredFPF); err != nil {
		return err
	}
	want := freezeInputSpec{Posture: freezePosturePendingSelection}
	if freezeInput != want {
		return fmt.Errorf("pending-selection P13 manifest carries unaccepted coordinates")
	}
	return nil
}

func validateFrozenManifestState(
	requiredFPF fpfIdentitySpec,
	freezeInput freezeInputSpec,
) error {
	if err := validateRequiredFPFSpec(requiredFPF); err != nil {
		return err
	}
	if err := validateFrozenExecutionInput(freezeInput); err != nil {
		return err
	}
	if freezeInput.TargetBaseRef != requiredFPF.BaseTypeEnvRef {
		return fmt.Errorf("P13 frozen target base differs from the selected FPF basis")
	}
	return nil
}

func validateManifestExecutionState(manifest acceptanceManifest) error {
	if manifest.Status != manifestStatusFrozen {
		return fmt.Errorf(
			"P13 execution is blocked while manifest status is %q",
			manifest.Status,
		)
	}
	return validateFrozenManifestState(
		manifest.Identity.RequiredFPF,
		manifest.FreezeInput,
	)
}

func validateFrozenExecutionInput(spec freezeInputSpec) error {
	if spec.Posture != freezePostureSelectedAndFrozen {
		return fmt.Errorf(
			"P13 execution is blocked until the manually selected head, receipt, and graph coordinates are frozen",
		)
	}
	if spec.ProfileLedgerRevision <= 0 ||
		spec.ProfileProjectionLedgerHead != spec.ProfileLedgerRevision ||
		spec.PriorHeadRevision != requiredPriorHeadRevision ||
		spec.HeadRevision != spec.PriorHeadRevision+1 ||
		spec.StageProfileLedgerRevision != spec.ProfileLedgerRevision ||
		spec.TransitionProfileCount <= 0 ||
		spec.SelectionGraphRevision <= 0 ||
		spec.GraphRevision < spec.SelectionGraphRevision {
		return fmt.Errorf("P13 frozen revision coordinates are incomplete")
	}
	values := []string{
		spec.ProfileGeneration,
		spec.ProfilePayloadDigest,
		spec.ProfileAdmissionRef,
		spec.ProfileAdmissionDigest,
		spec.ProfileProjectionDigest,
		spec.ProfileProjectionSchema,
		spec.ProfileBasisRef,
		spec.ProfileBasisDigest,
		spec.ProfileLedgerDigest,
		spec.ProfileSupportDAGDigest,
		spec.HeadRef,
		spec.SelectedCompositeRef,
		spec.HeadStateDigest,
		spec.SelectionClosureRef,
		spec.SelectionClosureDigest,
		spec.SelectionRequestSchema,
		spec.SelectionRequestRef,
		spec.SelectionRequestDigest,
		spec.SelectionStageRef,
		spec.SelectionStageDigest,
		spec.SelectionReadyClosureDigest,
		spec.SelectionPredecessorKind,
		spec.PriorHeadRef,
		spec.PriorSelectedCompositeRef,
		spec.PriorHeadStateDigest,
		spec.PriorCompositeDigest,
		spec.PriorBaseRef,
		spec.PriorBaseDigest,
		spec.PriorFPFRevision,
		spec.PriorCompilerSchema,
		spec.PriorExecutableDigest,
		spec.PriorLoweredDigest,
		spec.TargetBaseRef,
		spec.TargetExtensionsDigest,
		spec.TargetRuntimeBasisRef,
		spec.TargetCompositeRef,
		spec.SelectionReceiptRef,
		spec.SelectionReceiptDigest,
		spec.SelectionAuthorityUseRef,
		spec.SelectionAuthorityUseDigest,
		spec.SelectionGraphEventRef,
		spec.SelectionGraphCommitRef,
		spec.StageSchemaEdition,
		spec.StageProfileLedgerDigest,
		spec.StageProfileFitRef,
		spec.StageProfileFitDigest,
		spec.StageProfileFitRuleEdition,
		spec.StageProfileFitPosture,
		spec.TransitionProfilesRef,
		spec.TransitionProfilesDigest,
		spec.TransitionProfileSetDigest,
		spec.TransitionPosturesDigest,
		spec.GraphActiveTypeEnvRef,
		spec.GraphSnapshotBasisRef,
		spec.GraphSnapshotBasisDigest,
		spec.GraphLastEventRef,
		spec.GraphLastCommitRef,
		spec.GraphMaterializationDigest,
	}
	if slices.Contains(values, "") {
		return fmt.Errorf("P13 frozen head, receipt, or graph coordinates are incomplete")
	}
	if spec.SelectionPredecessorKind != "transition" {
		return fmt.Errorf("P13 frozen selection is not an exact post-P12E Transition")
	}
	if spec.SelectionRequestSchema != transitionSelectionRequestSchema {
		return fmt.Errorf("P13 frozen Transition request schema is not exact v2")
	}
	if spec.SelectionRequestRef !=
		"project-typeenv-head-selection-request:"+spec.SelectionRequestDigest {
		return fmt.Errorf("P13 frozen Transition request ref/digest are inconsistent")
	}
	if spec.SelectionStageRef != "project-typeenv-stage:"+spec.SelectionStageDigest {
		return fmt.Errorf("P13 frozen Transition Stage ref/digest are inconsistent")
	}
	if spec.HeadRef != spec.PriorHeadRef {
		return fmt.Errorf("P13 frozen Transition changed the stable project head ref")
	}
	if spec.TargetCompositeRef != spec.SelectedCompositeRef ||
		spec.GraphActiveTypeEnvRef != spec.SelectedCompositeRef {
		return fmt.Errorf("P13 frozen graph TypeEnv does not match the selected head")
	}
	if spec.ProfileProjectionSchema != "haft.project-profile-projection/v1" {
		return fmt.Errorf("P13 frozen profile projection is not exact final v1")
	}
	if spec.ProfileBasisRef != "project-profile-basis:"+spec.ProfileBasisDigest ||
		spec.ProfileLedgerDigest != spec.StageProfileLedgerDigest {
		return fmt.Errorf("P13 frozen profile basis does not match the Transition Stage")
	}
	if spec.StageSchemaEdition != requiredStageSchemaEdition ||
		spec.StageProfileFitPosture != "compatible" {
		return fmt.Errorf("P13 frozen Transition Stage is not exact v5 compatible profile-fit")
	}
	if spec.StageProfileFitRef != "project-typeenv-profile-fit:"+spec.StageProfileFitDigest {
		return fmt.Errorf("P13 frozen Stage profile-fit ref/digest are inconsistent")
	}
	if spec.TransitionProfilesRef !=
		"transition-projection-profile-compatibility-set:"+spec.TransitionProfilesDigest {
		return fmt.Errorf("P13 frozen Transition projection-profile ref/digest are inconsistent")
	}
	if spec.PriorSelectedCompositeRef != requiredPriorCompositeRef ||
		spec.PriorCompositeDigest != strings.TrimPrefix(requiredPriorCompositeRef, "typeenv:") ||
		spec.PriorBaseRef != requiredPriorBaseRef ||
		spec.PriorBaseDigest != requiredPriorBaseDigest ||
		spec.PriorFPFRevision != requiredPriorFPFRevision ||
		spec.PriorCompilerSchema != requiredPriorCompilerSchema ||
		spec.PriorExecutableDigest != requiredPriorSnapshotDigest ||
		spec.PriorLoweredDigest != requiredPriorLoweredDigest {
		return fmt.Errorf("P13 frozen predecessor is not the exact 44dd881 project-head closure")
	}
	return nil
}

func TestFrozenExecutionInputRejectsGenesisWithPositiveHead(t *testing.T) {
	spec := completeFrozenInputForTest()
	spec.SelectionPredecessorKind = "genesis"
	spec.PriorHeadRef = ""
	spec.PriorHeadRevision = 0
	spec.PriorSelectedCompositeRef = ""
	spec.PriorHeadStateDigest = ""
	if err := validateFrozenExecutionInput(spec); err == nil {
		t.Fatal("positive Genesis head unexpectedly satisfied post-P12E freeze")
	}
}

func TestFrozenExecutionInputRejectsRequestOrStageIdentityDrift(t *testing.T) {
	spec := completeFrozenInputForTest()
	spec.SelectionRequestSchema = "haft.project-typeenv.head-selection-request.v1"
	if err := validateFrozenExecutionInput(spec); err == nil {
		t.Fatal("wrong Transition request schema unexpectedly passed")
	}
	spec = completeFrozenInputForTest()
	spec.SelectionStageDigest = "sha256:different-stage"
	if err := validateFrozenExecutionInput(spec); err == nil {
		t.Fatal("mismatched Transition Stage ref/digest unexpectedly passed")
	}
}

func completeFrozenInputForTest() freezeInputSpec {
	return freezeInputSpec{
		Posture:                     freezePostureSelectedAndFrozen,
		ProfileGeneration:           "v1",
		ProfileLedgerRevision:       1,
		ProfilePayloadDigest:        "sha256:profile",
		ProfileAdmissionRef:         "profile-admission:one",
		ProfileAdmissionDigest:      "sha256:admission",
		ProfileProjectionDigest:     "sha256:projection",
		ProfileProjectionSchema:     "haft.project-profile-projection/v1",
		ProfileProjectionLedgerHead: 1,
		ProfileBasisRef:             "project-profile-basis:sha256:profile-basis",
		ProfileBasisDigest:          "sha256:profile-basis",
		ProfileLedgerDigest:         "sha256:profile-ledger",
		ProfileSupportDAGDigest:     "sha256:profile-support",
		HeadRef:                     "project-typeenv-head:qnt_01234567",
		HeadRevision:                2,
		SelectedCompositeRef:        "typeenv:sha256:target",
		HeadStateDigest:             "sha256:head",
		SelectionClosureRef:         "project-typeenv-head-selection-closure:sha256:closure",
		SelectionClosureDigest:      "sha256:closure",
		SelectionRequestSchema:      transitionSelectionRequestSchema,
		SelectionRequestRef:         "project-typeenv-head-selection-request:sha256:request",
		SelectionRequestDigest:      "sha256:request",
		SelectionStageRef:           "project-typeenv-stage:sha256:stage",
		SelectionStageDigest:        "sha256:stage",
		SelectionReadyClosureDigest: "sha256:selection-ready-closure",
		SelectionPredecessorKind:    "transition",
		PriorHeadRef:                "project-typeenv-head:qnt_01234567",
		PriorHeadRevision:           1,
		PriorSelectedCompositeRef:   requiredPriorCompositeRef,
		PriorHeadStateDigest:        "sha256:prior-head",
		PriorCompositeDigest:        strings.TrimPrefix(requiredPriorCompositeRef, "typeenv:"),
		PriorBaseRef:                requiredPriorBaseRef,
		PriorBaseDigest:             requiredPriorBaseDigest,
		PriorFPFRevision:            requiredPriorFPFRevision,
		PriorCompilerSchema:         requiredPriorCompilerSchema,
		PriorExecutableDigest:       requiredPriorSnapshotDigest,
		PriorLoweredDigest:          requiredPriorLoweredDigest,
		TargetBaseRef:               "typeenv:sha256:base",
		TargetExtensionsDigest:      "sha256:extensions",
		TargetRuntimeBasisRef:       "runtime-evaluation-basis:sha256:runtime",
		TargetCompositeRef:          "typeenv:sha256:target",
		SelectionReceiptRef:         "project-typeenv-head-selection-receipt:sha256:receipt",
		SelectionReceiptDigest:      "sha256:receipt",
		SelectionAuthorityUseRef:    "project-typeenv-head-selection-authority-use:sha256:authority",
		SelectionAuthorityUseDigest: "sha256:authority",
		SelectionGraphRevision:      4,
		SelectionGraphEventRef:      "typed-memory-event:selection",
		SelectionGraphCommitRef:     "typed-memory-commit:selection",
		StageSchemaEdition:          requiredStageSchemaEdition,
		StageProfileLedgerRevision:  1,
		StageProfileLedgerDigest:    "sha256:profile-ledger",
		StageProfileFitRef:          "project-typeenv-profile-fit:sha256:profile-fit",
		StageProfileFitDigest:       "sha256:profile-fit",
		StageProfileFitRuleEdition:  "haft.project-typeenv.profile-fit-rules/v1",
		StageProfileFitPosture:      "compatible",
		TransitionProfilesRef:       "transition-projection-profile-compatibility-set:sha256:transition-profiles",
		TransitionProfilesDigest:    "sha256:transition-profiles",
		TransitionProfileSetDigest:  "sha256:profile-set",
		TransitionProfileCount:      2,
		TransitionPosturesDigest:    "sha256:profile-postures",
		GraphRevision:               4,
		GraphActiveTypeEnvRef:       "typeenv:sha256:target",
		GraphSnapshotBasisRef:       "project-graph-snapshot-basis:sha256:graph",
		GraphSnapshotBasisDigest:    "sha256:graph",
		GraphLastEventRef:           "typed-memory-event:selection",
		GraphLastCommitRef:          "typed-memory-commit:selection",
		GraphMaterializationDigest:  "sha256:materialization",
	}
}

func validateSuites(suites []suiteSpec) (map[string]suiteSpec, error) {
	if len(suites) == 0 {
		return nil, fmt.Errorf("P13 manifest has no suites")
	}
	allowedKinds := map[string]struct{}{
		"exec":                    {},
		"fpf_index_verify":        {},
		"go_test_all_non_desktop": {},
		"go_test_race_critical":   {},
		"go_vet_all_non_desktop":  {},
		"gofmt_check":             {},
	}
	suiteByID := make(map[string]suiteSpec, len(suites))
	for _, suite := range suites {
		if suite.ID == "" || suite.Kind == "" || suite.TimeoutSeconds <= 0 {
			return nil, fmt.Errorf("P13 suite is incomplete: %#v", suite)
		}
		if _, known := allowedKinds[suite.Kind]; !known {
			return nil, fmt.Errorf(
				"P13 suite %q has unknown kind %q",
				suite.ID,
				suite.Kind,
			)
		}
		if _, duplicate := suiteByID[suite.ID]; duplicate {
			return nil, fmt.Errorf("P13 suite %q is duplicated", suite.ID)
		}
		if suite.Kind == "exec" && suite.Program == "" {
			return nil, fmt.Errorf("P13 exec suite %q has no program", suite.ID)
		}
		if suite.Kind != "exec" &&
			(suite.Program != "" || len(suite.Args) != 0 || suite.WorkingDirectory != "") {
			return nil, fmt.Errorf(
				"P13 derived suite %q carries an executable override",
				suite.ID,
			)
		}
		if suite.Kind == "go_test_race_critical" {
			if err := validateCriticalRaceSuite(suite); err != nil {
				return nil, err
			}
			suiteByID[suite.ID] = suite
			continue
		}
		if len(suite.GoRaceCases) != 0 ||
			suite.GoPackageParallelism != 0 ||
			suite.GoTestProcs != 0 {
			return nil, fmt.Errorf(
				"P13 non-race suite %q carries a race execution policy",
				suite.ID,
			)
		}
		suiteByID[suite.ID] = suite
	}
	if err := validateExactSuiteContract(suites); err != nil {
		return nil, err
	}
	return suiteByID, nil
}

func validateCriticalRaceSuite(suite suiteSpec) error {
	if len(suite.GoRaceCases) == 0 {
		return fmt.Errorf("P13 critical-race suite %q has no test cases", suite.ID)
	}
	if !slices.IsSortedFunc(
		suite.GoRaceCases,
		func(left goRaceCase, right goRaceCase) int {
			return strings.Compare(left.Package, right.Package)
		},
	) {
		return fmt.Errorf("P13 critical-race suite %q cases are not sorted", suite.ID)
	}
	seenPackages := make(map[string]struct{}, len(suite.GoRaceCases))
	for _, raceCase := range suite.GoRaceCases {
		if !strings.HasPrefix(raceCase.Package, modulePath+"/") {
			return fmt.Errorf(
				"P13 critical-race suite %q package %q is outside the module",
				suite.ID,
				raceCase.Package,
			)
		}
		if _, duplicate := seenPackages[raceCase.Package]; duplicate {
			return fmt.Errorf(
				"P13 critical-race suite %q repeats package %q",
				suite.ID,
				raceCase.Package,
			)
		}
		seenPackages[raceCase.Package] = struct{}{}
		if len(raceCase.Tests) == 0 {
			return fmt.Errorf(
				"P13 critical-race suite %q package %q has no tests",
				suite.ID,
				raceCase.Package,
			)
		}
		if !slices.IsSorted(raceCase.Tests) {
			return fmt.Errorf(
				"P13 critical-race suite %q package %q tests are not sorted",
				suite.ID,
				raceCase.Package,
			)
		}
		if len(slices.Compact(slices.Clone(raceCase.Tests))) != len(raceCase.Tests) {
			return fmt.Errorf(
				"P13 critical-race suite %q package %q tests are not unique",
				suite.ID,
				raceCase.Package,
			)
		}
		for _, testName := range raceCase.Tests {
			if !goTestFunctionName.MatchString(testName) {
				return fmt.Errorf(
					"P13 critical-race suite %q package %q has invalid test %q",
					suite.ID,
					raceCase.Package,
					testName,
				)
			}
		}
	}
	if suite.GoPackageParallelism <= 0 || suite.GoTestProcs <= 0 {
		return fmt.Errorf(
			"P13 critical-race suite %q has no positive resource bound",
			suite.ID,
		)
	}
	return nil
}

func validateCriticalRaceCaseTests(root string, suite suiteSpec) error {
	if suite.Kind != "go_test_race_critical" {
		return fmt.Errorf("P13 critical-race suite is absent")
	}
	for _, raceCase := range suite.GoRaceCases {
		for _, testName := range raceCase.Tests {
			anchor := testAnchor{
				Package: raceCase.Package,
				Test:    testName,
			}
			found, err := testAnchorExists(root, anchor)
			if err != nil {
				return fmt.Errorf(
					"P13 critical-race test %s: %w",
					anchorKey(anchor),
					err,
				)
			}
			if !found {
				return fmt.Errorf(
					"P13 critical-race test %s does not exist",
					anchorKey(anchor),
				)
			}
		}
	}
	return nil
}

func equalGoRaceCases(left []goRaceCase, right []goRaceCase) bool {
	return slices.EqualFunc(
		left,
		right,
		func(leftCase goRaceCase, rightCase goRaceCase) bool {
			return leftCase.Package == rightCase.Package &&
				slices.Equal(leftCase.Tests, rightCase.Tests)
		},
	)
}

func validateExactSuiteContract(suites []suiteSpec) error {
	want := []suiteSpec{
		{ID: "fpf_index_exact", Kind: "fpf_index_verify", TimeoutSeconds: 900},
		{
			ID:               "query_token_gate",
			Kind:             "exec",
			Program:          "bash",
			Args:             []string{"scripts/fpf_query_token_gate.sh"},
			WorkingDirectory: ".",
			TimeoutSeconds:   1800,
		},
		{ID: "go_normal", Kind: "go_test_all_non_desktop", TimeoutSeconds: 7200},
		{
			ID:                   "go_race",
			Kind:                 "go_test_race_critical",
			GoRaceCases:          slices.Clone(expectedCriticalRaceCases),
			GoPackageParallelism: 1,
			GoTestProcs:          2,
			TimeoutSeconds:       10800,
		},
		{ID: "go_vet", Kind: "go_vet_all_non_desktop", TimeoutSeconds: 3600},
		{
			ID:               "pi_test",
			Kind:             "exec",
			Program:          "pnpm",
			Args:             []string{"test"},
			WorkingDirectory: "packages/haft-pi",
			TimeoutSeconds:   1200,
		},
		{
			ID:               "pi_typecheck",
			Kind:             "exec",
			Program:          "pnpm",
			Args:             []string{"typecheck"},
			WorkingDirectory: "packages/haft-pi",
			TimeoutSeconds:   1200,
		},
		{
			ID:               "open_sleigh_format",
			Kind:             "exec",
			Program:          "mix",
			Args:             []string{"format", "--check-formatted"},
			WorkingDirectory: "open-sleigh",
			TimeoutSeconds:   1200,
		},
		{
			ID:               "open_sleigh_test",
			Kind:             "exec",
			Program:          "mix",
			Args:             []string{"test"},
			WorkingDirectory: "open-sleigh",
			TimeoutSeconds:   3600,
		},
		{ID: "gofmt_check", Kind: "gofmt_check", TimeoutSeconds: 900},
		{
			ID:               "git_diff_check",
			Kind:             "exec",
			Program:          "git",
			Args:             []string{"diff", "--check"},
			WorkingDirectory: ".",
			TimeoutSeconds:   900,
		},
	}
	if len(suites) != len(want) {
		return fmt.Errorf("P13 suite count = %d, want %d", len(suites), len(want))
	}
	for index := range want {
		observed := suites[index]
		expected := want[index]
		if observed.ID != expected.ID ||
			observed.Kind != expected.Kind ||
			observed.Program != expected.Program ||
			!slices.Equal(observed.Args, expected.Args) ||
			observed.WorkingDirectory != expected.WorkingDirectory ||
			!equalGoRaceCases(observed.GoRaceCases, expected.GoRaceCases) ||
			observed.GoPackageParallelism != expected.GoPackageParallelism ||
			observed.GoTestProcs != expected.GoTestProcs ||
			observed.TimeoutSeconds != expected.TimeoutSeconds {
			return fmt.Errorf(
				"P13 suite %d = %#v, want %#v",
				index,
				observed,
				expected,
			)
		}
	}
	return nil
}

func validateGates(
	root string,
	gates []gateSpec,
	suiteByID map[string]suiteSpec,
) error {
	if err := validateExactGateContract(gates); err != nil {
		return err
	}
	for _, gate := range gates {
		if gate.Title == "" || gate.PlanSpan == "" || len(gate.Claims) == 0 {
			return fmt.Errorf("P13 gate %q is not described", gate.ID)
		}
		if len(gate.SuiteIDs) == 0 || len(gate.Anchors) == 0 {
			return fmt.Errorf("P13 gate %q has no executable mapping", gate.ID)
		}
		if !slices.Contains(gate.SuiteIDs, "go_normal") {
			return fmt.Errorf("P13 gate %q has no normal Go proof suite", gate.ID)
		}
		for _, suiteID := range gate.SuiteIDs {
			if _, found := suiteByID[suiteID]; !found {
				return fmt.Errorf(
					"P13 gate %q names unknown suite %q",
					gate.ID,
					suiteID,
				)
			}
		}
		if err := validateGateAnchors(root, gate); err != nil {
			return err
		}
	}
	return nil
}

func validateExactGateContract(gates []gateSpec) error {
	want := exactGateContract()
	if len(gates) != len(want) {
		return fmt.Errorf("P13 gate count = %d, want %d", len(gates), len(want))
	}
	for index, expected := range want {
		observed := gates[index]
		if observed.ID != expected.ID {
			return fmt.Errorf(
				"P13 gate %d ID = %q, want %q",
				index,
				observed.ID,
				expected.ID,
			)
		}
		if observed.Title != expected.Title {
			return fmt.Errorf(
				"P13 gate %q title = %q, want %q",
				observed.ID,
				observed.Title,
				expected.Title,
			)
		}
		if observed.PlanSpan != expected.PlanSpan {
			return fmt.Errorf(
				"P13 gate %q plan span = %q, want %q",
				observed.ID,
				observed.PlanSpan,
				expected.PlanSpan,
			)
		}
		claimsDigest, err := exactClaimsDigest(observed.Claims)
		if err != nil {
			return fmt.Errorf("P13 gate %q claims: %w", observed.ID, err)
		}
		if claimsDigest != expected.ClaimsDigest {
			return fmt.Errorf(
				"P13 gate %q claims digest = %q, want %q",
				observed.ID,
				claimsDigest,
				expected.ClaimsDigest,
			)
		}
		if !slices.Equal(observed.SuiteIDs, expected.SuiteIDs) {
			return fmt.Errorf(
				"P13 gate %q suite IDs = %v, want %v",
				observed.ID,
				observed.SuiteIDs,
				expected.SuiteIDs,
			)
		}
		observedAnchorKeys := anchorKeys(observed.Anchors)
		if !slices.Equal(observedAnchorKeys, expected.AnchorKeys) {
			return fmt.Errorf(
				"P13 gate %q anchor keys = %v, want %v",
				observed.ID,
				observedAnchorKeys,
				expected.AnchorKeys,
			)
		}
	}
	return nil
}

func exactClaimsDigest(claims []string) (string, error) {
	encoded, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode exact claims: %w", err)
	}
	return sha256Prefixed(encoded), nil
}

func anchorKeys(anchors []testAnchor) []string {
	keys := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		keys = append(keys, anchorKey(anchor))
	}
	return keys
}

func exactGateContract() []gateContract {
	return []gateContract{
		{
			ID:           "G0",
			Title:        "Authority and specification",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:ca55d7e091e52f8fddb7c42e7ca12a9392e91b9fad9ce1f3821cef275c12aac1",
			SuiteIDs:     []string{"go_normal", "go_vet", "pi_test", "pi_typecheck", "open_sleigh_test"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/specmigrationv2::TestMigrationEffectSagaArchivesExactBytesWritesReceiptAndReplays",
				"github.com/m0n0x41d/haft/internal/specmigrationv2::TestAuditPacketCandidateVerifiesExactEightSectionPartition",
				"github.com/m0n0x41d/haft/internal/authority::TestEvaluateReceiptRejectsModelSuppliedArguments",
				"github.com/m0n0x41d/haft/internal/decisionbinding::TestDecisionBindingServicePersistsExactTwoPhaseClosure",
				"github.com/m0n0x41d/haft/internal/decisionbinding::TestDecisionContextPolicyRejectsProfileAndMigrationCrossBinding",
				"github.com/m0n0x41d/haft/internal/cli::TestArtifactCreateCLIUnknownDecisionBindingModeWritesNothing",
				"github.com/m0n0x41d/haft/internal/cli::TestDispatchToolRejectsMCPBindingDecisionWithoutCreatingArtifact",
			},
		},
		{
			ID:           "G0P",
			Title:        "Project-profile and applicability",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:869752aee38d916b36c1f34d7505e3482c7321468800fe5cf7d0e009a6363df2",
			SuiteIDs:     []string{"go_normal", "go_vet", "pi_test", "pi_typecheck"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/profileonboarding::TestRunProfileDeclarationExplicitPolicyAdmitsAndReplaysAfterRestart",
				"github.com/m0n0x41d/haft/internal/profileadmission/sqlite::TestResolveCurrentWithinRehashesV3AuthorityUseInsideCallerSnapshot",
				"github.com/m0n0x41d/haft/internal/profileadmission/sqlite::TestCapabilityApplicabilityMatrixResolverIsUnderdeterminedWithoutAdmission",
				"github.com/m0n0x41d/haft/internal/projectprofile::TestAdmittedScopeEntityRelationMakesTargetSystemSpecApplicable",
				"github.com/m0n0x41d/haft/internal/profileprojection::TestHistoricalV2OpenDebtResolvesThroughV3TaggedEvent",
				"github.com/m0n0x41d/haft/internal/cli::TestCanonicalProjectSpecificationApplicabilityIsUnderdeterminedWithoutAdmission",
				"github.com/m0n0x41d/haft/internal/cli::TestCanonicalProjectReadinessKeepsNonSoftwareHarnessFreeOfSWEGate",
				"github.com/m0n0x41d/haft/internal/cli::TestProfileDeclarationFreshReviewedCandidateReplaysAfterLedgerRestart",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestStageProfileBindingRejectsPostStageProfileDrift",
			},
		},
		{
			ID:           "G0I",
			Title:        "Core initialization and host-adapter publication",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:bc9e10e15346c7b84fcec5f894496e5c257a398e6ab0e3c60718210869d285df",
			SuiteIDs:     []string{"go_normal", "go_race", "go_vet", "pi_test", "pi_typecheck"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/cli::TestProjectMigrateCommandExposesNoHostSelectionFlags",
				"github.com/m0n0x41d/haft/internal/projectledgermigration::TestApplyExactRejectsStaleSchemaPlanBeforeMigration",
				"github.com/m0n0x41d/haft/internal/initplanning::TestCompileInitPlanSeparatesCoreAndHostEffectsAndProjectsExactPreview",
				"github.com/m0n0x41d/haft/internal/cli::TestInitSelectionTUIMapsCancelAndEOFToDistinctNoWriteOutcomes",
				"github.com/m0n0x41d/haft/internal/initexecution::TestPreparedCoreOnlyInitOperationCreatesNoHostPublicationCarrier",
				"github.com/m0n0x41d/haft/internal/initexecution::TestPreparedHostInitOperationAppliesOnlyItsExactReviewedPreview",
				"github.com/m0n0x41d/haft/internal/initexecution::TestExecutorCoreFailureLeavesEveryHostEffectUnperformed",
				"github.com/m0n0x41d/haft/internal/initexecution::TestExecutorReturnsExactPartialBoundaryAfterCoreAndHostDrift",
				"github.com/m0n0x41d/haft/internal/initplanning::TestSkillSourceBundleIsCanonicalContentAddressedAndStrict",
				"github.com/m0n0x41d/haft/internal/cli::TestCurrentCoherentHostApplicabilityIsCompleteAndHookFree",
				"github.com/m0n0x41d/haft/internal/cli::TestCurrentPiCondensedParityDeclaresExactSourceAndLossBoundary",
				"github.com/m0n0x41d/haft/internal/initplanning::TestClassifyInstallationCurrentnessProducesSevenExactOwnershipStates",
				"github.com/m0n0x41d/haft/internal/initfs::TestHostPublisherRecoveryPreservesPostPartialLocalModification",
				"github.com/m0n0x41d/haft/internal/initfs::TestPublicationCoordinatorSerializesSharedRootAcrossProcesses",
				"github.com/m0n0x41d/haft/internal/cli::TestHostStatusCommandEvaluatesEveryRegisteredCoherentProjection",
				"github.com/m0n0x41d/haft/internal/cli::TestCurrentCodexProjectionRunsThroughExactPreviewAndIdempotentApply",
			},
		},
		{
			ID:           "G1",
			Title:        "Source compiler and FPF upgrades",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:8f5a36143052e4c6d1340cda148c9db43f7de69483399824f5b861b1a8e41421",
			SuiteIDs:     []string{"fpf_index_exact", "query_token_gate", "go_normal", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/fpf::TestSQLiteQueryIndex_SourceNativeTiersAndExactHydration",
				"github.com/m0n0x41d/haft/internal/fpf::TestRebuildSourceQueryIndexAtomic_PreservesPreviousFileOnFailure",
				"github.com/m0n0x41d/haft/internal/fpf::TestWorkingProjectionRecursivelyExcludesInternalQueryFields",
				"github.com/m0n0x41d/haft/internal/fpf::TestWorkingExactProjectionKeepsInspectBodyAndSeparatesLookupIdentity",
				"github.com/m0n0x41d/haft/internal/fpf::TestTraceProjectionDeduplicatesAndReconstructsCanonicalProvenance",
				"github.com/m0n0x41d/haft/internal/fpf::TestDiagnosticProjectionAloneExposesRetrievalInternals",
				"github.com/m0n0x41d/haft/internal/fpf::TestCanonicalProvenanceValidationPrecedesProjection",
				"github.com/m0n0x41d/haft/internal/fpf::TestPublishedEncoderRejectsForgedWorkingLabelsOnInternalCarriers",
				"github.com/m0n0x41d/haft/internal/fpf::TestTraceReplayFailsClosedBeforeRetrievalOnSnapshotOrRequestDrift",
				"github.com/m0n0x41d/haft/internal/fpf::TestTraceReplayDetectsCanonicalResultDriftAfterPreflight",
				"github.com/m0n0x41d/haft/internal/fpf::TestWorkingPayloadDoesNotGrowWithConcernOrRawGroundCount",
				"github.com/m0n0x41d/haft/internal/fpf::TestWorkingExactLookupBudgetsCuesRelationsAndReferencesButInspectDoesNot",
				"github.com/m0n0x41d/haft/internal/fpf/typeenv::TestPinnedPublicationCompilesArtifactAndRuntimeTypeEnv",
				"github.com/m0n0x41d/haft/internal/fpf/localpractice::TestDigestCommitsToExactCarrierBytesRatherThanOnlyTheAST",
				"github.com/m0n0x41d/haft/internal/fpf/localpractice::TestParseCurrentKindClassificationSignatureHasNoEntitySetOrMemberOfFields",
				"github.com/m0n0x41d/haft/internal/fpf/localpractice::TestCurrentCandidateCarrierV1_4RemainsByteStableAndUsesKindClassification",
				"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv::TestResolveManifestGraphRejectsMissingSelfAndCyclicImports",
				"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv::TestPrepareProjectTypeEnvCompositeLowersEverySourceFamilyAtC",
				"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv::TestCurrentKindClassificationCompilesLinksLowersAndDiscoversOnlyCriterionRuntime",
				"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility::TestSuccessorDiffClosedTaxonomy",
				"github.com/m0n0x41d/haft/internal/projecttypeenvstage::TestLoadExecutableSnapshotTxRestoresExactImmutableCWithoutStageLookup",
				"github.com/m0n0x41d/haft/internal/projecttypeenvstage::TestStoreLoadSelectionReadyFailsClosedOnPersistedCorruption",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestRequiredTransitionPredecessorRejectsArbitraryAndSecondTransition",
			},
		},
		{
			ID:           "G2",
			Title:        "Algebraic typed core",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:1620bf0ecd0dedd7abe7efc073c55381df91c8ce27d0059a5283a7850304194c",
			SuiteIDs:     []string{"go_normal", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/typedmemory::TestTypeEnvRetainsA65SlotKindValueKindAndRefKind",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestExternalCodeCannotPutSchemaChangeInMemoryChangeSet",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestValueShapeAlgebraContainsOnlyClosedVariants",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestVerifyTypedValueRequiresBindingAndCodecThenSealsCanonicalValue",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestClaimGraphCodecV1GoldenCanonicalBytes",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestDiagnosticSeparatesContradictionFromMissingBasis",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestKindIdentifierDoesNotUseEntityOfConcernWordBan",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestKindClassificationRequestHasFourExactInputsAndNoEntitySet",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestKindClassificationKeepsTrueFalseUnknownEvidenceAndGuardSeparate",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestKindExtensionIsNamedOptionalProjectionOfTrueJudgementsOnly",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestCurrentSubkindOrderUsesObtainingFactsAndSeparateAssertion",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestKindBridgeCreatesFreshTargetRequestWithoutSourceTruth",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestRoleMaskKeepsFeatureJudgementAndScopeExpectationSeparate",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestClassificationAdmissionBasisCarriesOnlyDirectCurrentJudgements",
				"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime::TestCurrentCandidateBuildsAllFiveExactReferenceSchemeRegistries",
				"github.com/m0n0x41d/haft/internal/projecttypeenvruntime::TestObserveCurrentTargetRuntimeMatchesAllReferenceSchemeCallables",
				"github.com/m0n0x41d/haft/internal/projectmemoryconstitution::TestEachC21DiscriminatorChangesEpistemeCoordinate",
				"github.com/m0n0x41d/haft/internal/projectmemoryconstitution::TestEpistemeCoordinateExcludesGroundingTimeTypeEnvAndResolutionBasis",
				"github.com/m0n0x41d/haft/internal/projectmemoryconstitution::TestEvaluateKeepsMissingRuntimeBasisUnderdetermined",
			},
		},
		{
			ID:           "G3",
			Title:        "FPF source category-error corpus",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:da7f0a46afcf5bb3d06a9b9622598fbe41357218c8445a0142728600622b73d0",
			SuiteIDs:     []string{"go_normal", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/typedmemory::TestSourceConformanceCategoryErrorCorpus",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestSourceConformanceCategoryErrorSourceSnapshotIsCurrent",
				"github.com/m0n0x41d/haft/internal/projectmemory/architecturep2s::TestComposePreservesDistinctPositionsAndIsPermutationStable",
				"github.com/m0n0x41d/haft/internal/projectmemory/architecturep2s::TestRecursiveNaryFlowRoundTripsWithoutDirectionalPromotion",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestProductionGoldenConcernBundleRecoversExactHaftSoftwareSystemMemory",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestProductCarriersUseSourceNeutralNames",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestTrackedCarriersUseSourceNeutralNames",
			},
		},
		{
			ID:           "G4",
			Title:        "MCP validation and admission",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:ca09580608bf336099b98ab71aa03d7d9fbd819b8ff0eba7bae0a867e264ec27",
			SuiteIDs:     []string{"go_normal", "go_vet", "pi_test", "pi_typecheck"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/typedmemory::TestGenericKernelHasNoHaftProductCarrierDependency",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestIndependentProductAdaptersConsumeGenericKernelPublicTypes",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestGenericityGuardTargetsExactProductVocabulary",
				"github.com/m0n0x41d/haft/internal/typedmemory::TestValidateMemoryChangeSetUsesCurrentClassificationWithoutMemberOf",
				"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime::TestCurrentKindClassificationRuntimeCoversAllCarrierFamilies",
				"github.com/m0n0x41d/haft/internal/cli::TestMemoryValidationCLIAndMCPShareStableReadOnlyPresentation",
				"github.com/m0n0x41d/haft/internal/cli::TestSealedProjectMemorySplitSurfaceImplementsEveryAdvertisedAction",
				"github.com/m0n0x41d/haft/internal/cli::TestProjectMemoryCommitUnknownHasCLIAndMCPDeliveryParity",
				"github.com/m0n0x41d/haft/internal/cli::TestSourceNativeFPFQueryIntegration",
				"github.com/m0n0x41d/haft/internal/cli::TestEmbeddedFPFQueryWorksFromEmptyDownstreamProject",
			},
		},
		{
			ID:           "G5",
			Title:        "Transactional durability",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:5b857e3210fbc69af4902bbff87b9d7731fa5dc3ce86dfc97dad061d89ab6114",
			SuiteIDs:     []string{"go_normal", "go_race", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestSQLiteAdapterCommitDeclareEntityPersistsExactClosureAndReopens",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestSQLiteAdapterTwoConnectionsCASOneExpectedRevisionZeroCommit",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestGenericCommitFailureRollsBackWholeMixedBatch",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestCommitDeclareEntityRecoversAfterPhysicalCommitReportsFailure",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestTransitionServiceCommitsExactSuccessorAndReplays",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestTransitionServiceRollbackSelectsPriorImmutableCWithoutDeletingAssertions",
				"github.com/m0n0x41d/haft/db::TestTypedMemoryRelationalAssertionMigration53PreservesV52HistoryByteExactly",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestFreshV2AdmissionsUseWriter53AcrossConsecutiveCommitsAndSnapshotRead",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestV2RelationalAssertionSemanticFootprintMatchesSchemaV53View",
				"github.com/m0n0x41d/haft/db::TestTypedMemoryKindClassificationMigration54PreservesV53HistoryByteExactly",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestCurrentKindClassificationCommitPersistsExactV54ClosureAndReplay",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestCurrentKindClassificationSourceCatalogRoundTripsAfterRestart",
			},
		},
		{
			ID:           "G6",
			Title:        "Legacy migration",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:fb87ae71a2bb54b58ab142bae9e0b9be64b8f9087e86df973ec53cf105355347",
			SuiteIDs:     []string{"go_normal", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/db::TestReleasedV6AndV8DatabasesUpgradeAdditivelyToCurrentSchema",
				"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport::TestImportPlanDoesNotFabricateTypedSemanticsOrAuthority",
				"github.com/m0n0x41d/haft/internal/projectmemory/legacyimportsqlite::TestCoreSnapshotDryRunReadsWithoutWritingAndDoesNotGuessRelations",
				"github.com/m0n0x41d/haft/internal/projectmemory/legacydualread::TestCoalesceExposesIdentityCollisionWithoutChoosingWinner",
				"github.com/m0n0x41d/haft/internal/projectmemory/legacyimporteffect::TestApplyPersistsOneAtomicOpaqueBatchAndReplaysWithoutCurrentHead",
				"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime::TestCurrentClassificationAdaptsSealedHistoricalRecordDeliveryWithoutUsingMembershipJudgement",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestSealedHistoricalClassificationSourcesRemainEphemeralAndDeduplicated",
				"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv::TestHistoricalV1ExecutableSnapshotReplaysWithoutCurrentClassificationField",
				"github.com/m0n0x41d/haft/internal/projecttypeenvstore::TestPrepareArtifactClosurePreservesSealedHistoricalLowererRecipe",
				"github.com/m0n0x41d/haft/db::TestSchema50DatabaseCopyUpgradesToCurrentSchemaWithoutInventingState",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselection::TestFrozenHistoricalV3GenesisStageDecodesAndReencodesExactly",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselection::TestFrozenReconstructedV2GenesisStageDecodesAndReencodesExactly",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselection::TestFrozenRealTransitionStageV5DecodesAndReencodesExactly",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestCurrentProjectGraphObservationPreservesLegacyAndV3AssertionCarriers",
				"github.com/m0n0x41d/haft/internal/typedmemorystore::TestHistoricalWriter46V1ExactReplaySurvivesRestart",
				"github.com/m0n0x41d/haft/db::TestProfileAuthorityUnionMigration51UpgradesHistoryWithoutBackfill",
				"github.com/m0n0x41d/haft/internal/authority::TestAuthorityBasisWriterStoresLoadsAndReplaysExactGraph",
				"github.com/m0n0x41d/haft/db::TestProfileAuthorityAdmissionV2Migration44InstallsExactAdditiveContract",
				"github.com/m0n0x41d/haft/internal/authority::TestResolveHistoricalProfileAdmissionAuthorityV1ExactAndReadOnly",
				"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter::TestLoadExistingDecisionChoiceSourcePreservesIndependentLegacyChoiceFields",
			},
		},
		{
			ID:           "G7",
			Title:        "Graph, specification, and code projection",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:1b43812c001573272a3c0ee0fb37eb56f674a137f2adcf3f2067b91ff4145272",
			SuiteIDs:     []string{"go_normal", "go_race", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestProductionGoldenConcernBundleRecoversExactHaftSoftwareSystemMemory",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestProductionCodeAnchorAdapterAdmitsExplicitClaimLinkWithoutInference",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestProductionEvidenceWorkAdapterAdmitsClaimBoundLocalRelations",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestReviewedIdentityMergeCommitsReplaysResolvesAndLoadsCurrentSnapshot",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestReviewedIdentitySplitReturnsCandidatesAndProjectionDebtDoesNotMutateGraph",
				"github.com/m0n0x41d/haft/db::TestTypedMemoryIdentityReconciliationMigration52FootprintIsImmutable",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestCurrentPublicResolutionConsumesOnlyExactCommittedMergeState",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestCurrentPublicResolutionPreservesReviewedSplitAsUnsettledCandidates",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestCurrentPublicResolutionPreservesMergeToSplitHistoryChain",
				"github.com/m0n0x41d/haft/internal/codeintel::TestHGTraversalOutcomeCorpus",
				"github.com/m0n0x41d/haft/internal/codeintel::TestHGOutcomeSerializationDeterministic",
				"github.com/m0n0x41d/haft/internal/codeintel::TestExploreBagDirection_MixedOutcomes",
				"github.com/m0n0x41d/haft/internal/codebase::TestSourceAdmissionPureCorpus",
				"github.com/m0n0x41d/haft/internal/codebase::TestSourceAdmissionRootBudgetsAreTyped",
				"github.com/m0n0x41d/haft/internal/codebase::TestRefreshIncrementalDoesNotFollowSymlinkCycle",
				"github.com/m0n0x41d/haft/internal/codebase::TestRootAdmissionBudgetRetainsPriorEpoch",
				"github.com/m0n0x41d/haft/internal/codebase::TestIndexPublicationFailureRollsBackCandidateAndBasis",
				"github.com/m0n0x41d/haft/internal/codebase::TestIndexBasisSurvivesProcessReopen",
				"github.com/m0n0x41d/haft/internal/codebase::TestConcurrentReaderObservesOnlyWholeEpochBasis",
				"github.com/m0n0x41d/haft/internal/codebase::TestCodeGraphTypeScriptParityQualificationCarrierIsCurrent",
				"github.com/m0n0x41d/haft/internal/codeintel::TestPublishedExploreWorkingConcernIsBoundedAndScoreFree",
				"github.com/m0n0x41d/haft/internal/codeintel::TestPublishedExploreWorkingHostileConcernIsEscapedDeterministicAndBounded",
				"github.com/m0n0x41d/haft/internal/codeintel::TestPublishedExploreTraceIsDeterministicAndDiagnosticIsExplicit",
				"github.com/m0n0x41d/haft/internal/codeintel::TestPublishedExploreReplayMismatchNamesChangedBasis",
				"github.com/m0n0x41d/haft/internal/cli::TestHandleQuintQueryExploreConcernReturnsEvidenceBearingCandidates",
				"github.com/m0n0x41d/haft/internal/initplanning::TestSkillComponentRendererDerivesHostCarriersFromOneBundle",
				"github.com/m0n0x41d/haft/internal/cli::TestSessionAuditSeparatesTypedMemoryHydrationFromGraphPreflight",
				"github.com/m0n0x41d/haft/internal/cli::TestSessionAuditTreatsUnavailableTypedMemoryBasisAsNonBlocking",
				"github.com/m0n0x41d/haft/internal/cli::TestSessionAuditFlagsContextHeavyWorkWithoutTypedMemoryOrientation",
				"github.com/m0n0x41d/haft/internal/cli::TestSessionAuditClassifiesMechanicalEditAsGraphNotApplicable",
			},
		},
		{
			ID:           "G7R",
			Title:        "EntityOfConcern recall",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:23c239149e7bcd9e32193b7644e047d77ac352f50a49092535690675b6ee13db",
			SuiteIDs:     []string{"go_normal", "go_race", "go_vet"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve::TestExactIdentifierAndContextResolveBeforeRanking",
				"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve::TestExactAliasConflictFailsUnsettled",
				"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall::TestLexicalRecallScopesBeforeRankingAcrossRussianText",
				"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood::TestFacetCoverageKeepsFivePosturesDistinct",
				"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood::TestExactInterpretationIsDerivedFromIndependentItemPostures",
				"github.com/m0n0x41d/haft/internal/projectmemory/neighborhoodcache::TestReadThroughCacheHitAndMissAreByteIdentical",
				"github.com/m0n0x41d/haft/internal/projectmemory::TestCurrentReadRuntimeRejectsStaleProcessAcrossTypeEnvTransitionAndReusesRollbackSnapshot",
				"github.com/m0n0x41d/haft/internal/projectmemory::TestMemoryReadOutputContractNamesClosedRuntimeFamilies",
				"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite::TestProductionGoldenConcernBundleRecoversExactHaftSoftwareSystemMemory",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestCurrentPublicResolutionConsumesOnlyExactCommittedMergeState",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestCurrentPublicResolutionPreservesReviewedSplitAsUnsettledCandidates",
				"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation::TestCurrentPublicResolutionPreservesMergeToSplitHistoryChain",
				"github.com/m0n0x41d/haft/internal/projectmemory::TestCurrentNeighborhoodClassifiesNoteAtConcernRecordExactly",
				"github.com/m0n0x41d/haft/internal/cli::TestProjectMemoryFullSurfaceIsInstalledOnlyAfterExactReadiness",
				"github.com/m0n0x41d/haft/internal/codebase::TestConcernDiscoveryMeetsFrozenCodeNativeCorpus",
				"github.com/m0n0x41d/haft/internal/codebase::TestConcernDiscoveryNeverSelectsFrozenNegativeCorpus",
				"github.com/m0n0x41d/haft/internal/codeintel::TestConcernFusionMeetsFrozenReasoningToCodeCorpus",
				"github.com/m0n0x41d/haft/internal/projectmemory/existingrecordprojection::TestBuildOrdersSupportedRoutesByDependency",
				"github.com/m0n0x41d/haft/internal/projectmemory/existingrecordprojection::TestBuildKeepsUnsupportedCarrierMeaningsExplicit",
				"github.com/m0n0x41d/haft/internal/cli::TestMemoryBackfillDryRunThenApplyUsesSourceOwnedProjection",
			},
		},
		{
			ID:           "G8",
			Title:        "Stream and release truth",
			PlanSpan:     p13PlanSpan,
			ClaimsDigest: "sha256:5923088099b8e3032714734f7b032db13ffc023eea0eaf11065e687a34fc3b1c",
			SuiteIDs:     []string{"fpf_index_exact", "query_token_gate", "go_normal", "go_vet", "pi_test", "pi_typecheck", "open_sleigh_format", "open_sleigh_test", "gofmt_check", "git_diff_check"},
			AnchorKeys: []string{
				"github.com/m0n0x41d/haft/internal/streamtruth::TestREADMEDeclaresExactlyTheFourStreamTruthLabels",
				"github.com/m0n0x41d/haft/internal/streamtruth::TestCurrentFacingV9ProseHasNoUnsupportedTruthClaim",
				"github.com/m0n0x41d/haft/internal/streamtruth::TestCurrentFacingCarriersMakeNoRetrievalSuperiorityClaim",
				"github.com/m0n0x41d/haft/internal/streamtruth::TestAgentDisciplineTemplateKeepsOneTruthContract",
				"github.com/m0n0x41d/haft/internal/cli::TestHReasonSkill_IsSourceFirstUmbrella",
				"github.com/m0n0x41d/haft/internal/cli::TestFreshHostAndPiCarriersPreserveIndependentSourceFirstSemantics",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestRelevantByteIdentityIncludesUntrackedRootGoBuildInput",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestDependencyIdentityBindsInstalledTreeBytes",
				"github.com/m0n0x41d/haft/internal/p13acceptance::TestHGAdversarialAcceptance",
			},
		},
	}
}

func validateGateAnchors(root string, gate gateSpec) error {
	seen := make(map[string]struct{}, len(gate.Anchors))
	for _, anchor := range gate.Anchors {
		key := anchorKey(anchor)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf(
				"P13 gate %q repeats anchor %s",
				gate.ID,
				key,
			)
		}
		seen[key] = struct{}{}
		found, err := testAnchorExists(root, anchor)
		if err != nil {
			return fmt.Errorf("P13 gate %q anchor %s: %w", gate.ID, key, err)
		}
		if !found {
			return fmt.Errorf("P13 gate %q anchor %s does not exist", gate.ID, key)
		}
	}
	return nil
}

func validateEverySuiteIsMapped(gates []gateSpec, suites []suiteSpec) error {
	mapped := make(map[string]struct{}, len(suites))
	for _, gate := range gates {
		for _, suiteID := range gate.SuiteIDs {
			mapped[suiteID] = struct{}{}
		}
	}
	for _, suite := range suites {
		if _, found := mapped[suite.ID]; !found {
			return fmt.Errorf("P13 suite %q is not mapped to a gate", suite.ID)
		}
	}
	return nil
}

func testAnchorExists(root string, anchor testAnchor) (bool, error) {
	prefix := modulePath + "/"
	if anchor.Package != modulePath && !strings.HasPrefix(anchor.Package, prefix) {
		return false, fmt.Errorf("package %q is outside this module", anchor.Package)
	}
	relative := strings.TrimPrefix(anchor.Package, prefix)
	if anchor.Package == modulePath {
		relative = "."
	}
	directory := filepath.Join(root, filepath.FromSlash(relative))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, fmt.Errorf("read package directory: %w", err)
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		parsed, parseErr := parser.ParseFile(
			files,
			path,
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return false, fmt.Errorf("parse %s: %w", path, parseErr)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}
			if function.Name.Name == anchor.Test {
				return true, nil
			}
		}
	}
	return false, nil
}

func anchorKey(anchor testAnchor) string {
	return anchor.Package + "::" + anchor.Test
}
