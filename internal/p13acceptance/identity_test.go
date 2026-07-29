package p13acceptance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profileprojection"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	projecttypeenvselectioneffectsqlite "github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

type acceptanceIdentity struct {
	Schema      string            `json:"schema"`
	ProjectID   string            `json:"project_id"`
	ProjectRoot string            `json:"project_root"`
	Source      byteIdentity      `json:"source"`
	Git         gitIdentity       `json:"git"`
	FPF         fpfIdentity       `json:"fpf"`
	Toolchain   toolchainIdentity `json:"toolchain"`
	Profile     profileIdentity   `json:"profile"`
	TypeEnv     typeEnvIdentity   `json:"type_env"`
	Graph       graphIdentity     `json:"graph"`
	SchemaState schemaIdentity    `json:"schema_state"`
	Digest      string            `json:"digest,omitempty"`
}

type byteIdentity struct {
	Digest    string `json:"digest"`
	FileCount int    `json:"file_count"`
	ByteCount int64  `json:"byte_count"`
}

type gitIdentity struct {
	Head         string `json:"head"`
	StatusDigest string `json:"status_digest"`
	StatusBytes  int    `json:"status_bytes"`
}

type fpfIdentity struct {
	Head         string              `json:"head"`
	StatusDigest string              `json:"status_digest"`
	StatusBytes  int                 `json:"status_bytes"`
	SpecDigest   string              `json:"spec_digest"`
	ReadmeDigest string              `json:"readme_digest"`
	Source       byteIdentity        `json:"source"`
	Embedded     embeddedFPFIdentity `json:"embedded"`
}

type embeddedFPFIdentity struct {
	Revision                  string `json:"revision"`
	SpecDigest                string `json:"spec_digest"`
	ReadmeDigest              string `json:"readme_digest"`
	BaseTypeEnvRef            string `json:"base_type_env_ref"`
	BaseTypeEnvDigest         string `json:"base_type_env_digest"`
	BaseTypeEnvSourceRevision string `json:"base_type_env_source_revision"`
	CompilerSchema            string `json:"compiler_schema"`
}

type toolchainIdentity struct {
	GoVersion         string `json:"go_version"`
	GoEnvironment     string `json:"go_environment"`
	GitVersion        string `json:"git_version"`
	BashVersion       string `json:"bash_version"`
	PythonRuntime     string `json:"python_runtime"`
	NodeVersion       string `json:"node_version"`
	PNPMVersion       string `json:"pnpm_version"`
	MixVersion        string `json:"mix_version"`
	ElixirVersion     string `json:"elixir_version"`
	Platform          string `json:"platform"`
	ExecutableDigest  string `json:"executable_digest"`
	EnvironmentDigest string `json:"environment_digest"`
	ModuleGraphDigest string `json:"module_graph_digest"`
	DependencyDigest  string `json:"dependency_digest"`
	DependencyFiles   int    `json:"dependency_files"`
	DependencyBytes   int64  `json:"dependency_bytes"`
}

type profileIdentity struct {
	Generation           string `json:"generation"`
	LedgerRevision       int64  `json:"ledger_revision"`
	PayloadDigest        string `json:"payload_digest"`
	AdmissionRef         string `json:"admission_ref"`
	AdmissionDigest      string `json:"admission_digest"`
	ProjectionDigest     string `json:"projection_digest"`
	ProjectionSchema     string `json:"projection_schema"`
	ProjectionLedgerHead int64  `json:"projection_ledger_head"`
	BasisRef             string `json:"basis_ref"`
	BasisDigest          string `json:"basis_digest"`
	LedgerDigest         string `json:"ledger_digest"`
	SupportDAGDigest     string `json:"support_dag_digest"`
}

type predecessorIdentity struct {
	HeadRevision             int64  `json:"head_revision"`
	CompositeRef             string `json:"composite_ref"`
	CompositeDigest          string `json:"composite_digest"`
	BaseTypeEnvRef           string `json:"base_type_env_ref"`
	BaseTypeEnvDigest        string `json:"base_type_env_digest"`
	FPFRevision              string `json:"fpf_revision"`
	CompilerSchema           string `json:"compiler_schema"`
	ExecutableSnapshotDigest string `json:"executable_snapshot_digest"`
	LoweredEnvironmentDigest string `json:"lowered_environment_digest"`
}

type stageProfileIdentity struct {
	SchemaEdition            string `json:"schema_edition"`
	LedgerRevision           int64  `json:"ledger_revision"`
	LedgerDigest             string `json:"ledger_digest"`
	FitRef                   string `json:"fit_ref"`
	FitDigest                string `json:"fit_digest"`
	FitRuleEdition           string `json:"fit_rule_edition"`
	FitPosture               string `json:"fit_posture"`
	TransitionProfilesRef    string `json:"transition_profiles_ref"`
	TransitionProfilesDigest string `json:"transition_profiles_digest"`
	TransitionProfileSet     string `json:"transition_profile_set_digest"`
	TransitionProfileCount   int    `json:"transition_profile_count"`
	TransitionPosturesDigest string `json:"transition_postures_digest"`
}

type typeEnvIdentity struct {
	HeadRef                       string `json:"head_ref"`
	HeadRevision                  int64  `json:"head_revision"`
	SelectedCompositeRef          string `json:"selected_composite_ref"`
	StateDigest                   string `json:"state_digest"`
	SelectionClosureRef           string `json:"selection_closure_ref"`
	SelectionClosureDigest        string `json:"selection_closure_digest"`
	SelectionRequestSchema        string `json:"selection_request_schema"`
	SelectionRequestRef           string `json:"selection_request_ref"`
	SelectionRequestDigest        string `json:"selection_request_digest"`
	SelectionStageRef             string `json:"selection_stage_ref"`
	SelectionStageDigest          string `json:"selection_stage_digest"`
	SelectionReadyClosureDigest   string `json:"selection_ready_closure_digest"`
	SelectionPredecessorKind      string `json:"selection_predecessor_kind"`
	PriorHeadRef                  string `json:"prior_head_ref"`
	PriorHeadRevision             int64  `json:"prior_head_revision"`
	PriorSelectedCompositeRef     string `json:"prior_selected_composite_ref"`
	PriorHeadStateDigest          string `json:"prior_head_state_digest"`
	PriorCompositeDigest          string `json:"prior_composite_digest"`
	PriorBaseRef                  string `json:"prior_base_ref"`
	PriorBaseDigest               string `json:"prior_base_digest"`
	PriorFPFRevision              string `json:"prior_fpf_revision"`
	PriorCompilerSchema           string `json:"prior_compiler_schema"`
	PriorExecutableDigest         string `json:"prior_executable_snapshot_digest"`
	PriorLoweredDigest            string `json:"prior_lowered_environment_digest"`
	TargetBaseRef                 string `json:"target_base_ref"`
	TargetOrderedExtensionsDigest string `json:"target_ordered_extensions_digest"`
	TargetRuntimeBasisRef         string `json:"target_runtime_basis_ref"`
	TargetCompositeRef            string `json:"target_composite_ref"`
	SelectionReceiptRef           string `json:"selection_receipt_ref"`
	SelectionReceiptDigest        string `json:"selection_receipt_digest"`
	SelectionAuthorityUseRef      string `json:"selection_authority_use_ref"`
	SelectionAuthorityUseDigest   string `json:"selection_authority_use_digest"`
	SelectionGraphRevision        int64  `json:"selection_graph_revision"`
	SelectionGraphEventRef        string `json:"selection_graph_event_ref"`
	SelectionGraphCommitRef       string `json:"selection_graph_commit_ref"`
	StageSchemaEdition            string `json:"stage_schema_edition"`
	StageProfileLedgerRevision    int64  `json:"stage_profile_ledger_revision"`
	StageProfileLedgerDigest      string `json:"stage_profile_ledger_digest"`
	StageProfileFitRef            string `json:"stage_profile_fit_ref"`
	StageProfileFitDigest         string `json:"stage_profile_fit_digest"`
	StageProfileFitRuleEdition    string `json:"stage_profile_fit_rule_edition"`
	StageProfileFitPosture        string `json:"stage_profile_fit_posture"`
	TransitionProfilesRef         string `json:"transition_profiles_ref"`
	TransitionProfilesDigest      string `json:"transition_profiles_digest"`
	TransitionProfileSetDigest    string `json:"transition_profile_set_digest"`
	TransitionProfileCount        int    `json:"transition_profile_count"`
	TransitionPosturesDigest      string `json:"transition_postures_digest"`
}

type graphIdentity struct {
	Revision              int64  `json:"revision"`
	ActiveTypeEnvRef      string `json:"active_type_env_ref"`
	SnapshotBasisRef      string `json:"snapshot_basis_ref"`
	SnapshotBasisDigest   string `json:"snapshot_basis_digest"`
	LastEventRef          string `json:"last_event_ref"`
	LastCommitRef         string `json:"last_commit_ref"`
	MaterializationDigest string `json:"materialization_digest"`
}

type schemaIdentity struct {
	MaximumVersion         int    `json:"maximum_version"`
	VersionCount           int    `json:"version_count"`
	VersionsDigest         string `json:"versions_digest"`
	WriterGeneration       int    `json:"writer_generation"`
	WriterCapabilityDigest string `json:"writer_capability_digest"`
	CatalogObjectCount     int    `json:"catalog_object_count"`
	CatalogDigest          string `json:"catalog_digest"`
}

type schemaCatalogRow struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	TableName string `json:"table_name"`
	SQL       string `json:"sql"`
}

type projectCarrier struct {
	ID string `yaml:"id"`
}

type resolvedProfileCoordinates struct {
	ProjectRoot     string
	LedgerRevision  int64
	PayloadDigest   string
	AdmissionRef    string
	AdmissionDigest string
}

type goListPackage struct {
	Dir             string   `json:"Dir"`
	GoFiles         []string `json:"GoFiles"`
	CgoFiles        []string `json:"CgoFiles"`
	CFiles          []string `json:"CFiles"`
	CXXFiles        []string `json:"CXXFiles"`
	MFiles          []string `json:"MFiles"`
	HFiles          []string `json:"HFiles"`
	FFiles          []string `json:"FFiles"`
	SFiles          []string `json:"SFiles"`
	SwigFiles       []string `json:"SwigFiles"`
	SwigCXXFiles    []string `json:"SwigCXXFiles"`
	SysoFiles       []string `json:"SysoFiles"`
	EmbedFiles      []string `json:"EmbedFiles"`
	TestGoFiles     []string `json:"TestGoFiles"`
	TestEmbedFiles  []string `json:"TestEmbedFiles"`
	XTestGoFiles    []string `json:"XTestGoFiles"`
	XTestEmbedFiles []string `json:"XTestEmbedFiles"`
}

type goEnvironmentCoordinates struct {
	CGOEnabled string `json:"CGO_ENABLED"`
	CC         string `json:"CC"`
	CXX        string `json:"CXX"`
	GoToolDir  string `json:"GOTOOLDIR"`
}

type identityReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const p13FPFReadmeFileName = "Readme.md"

var canonicalProjectID = regexp.MustCompile(`^qnt_[0-9a-f]{8}$`)

func TestP13FPFReadmePathMatchesTrackedSourceCase(t *testing.T) {
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	gitProgram, err := resolveExecutable("git")
	if err != nil {
		t.Fatal(err)
	}
	fpfRoot := filepath.Join(root, "data", "FPF")
	_, err = runIdentityCommand(
		fpfRoot,
		gitProgram,
		"ls-files",
		"--error-unmatch",
		"--",
		p13FPFReadmeFileName,
	)
	if err != nil {
		t.Fatalf(
			"P13 FPF README path %q does not match the tracked source case: %v",
			p13FPFReadmeFileName,
			err,
		)
	}
}

func TestRequiredFPFIdentityRejectsCheckoutOrEmbeddedDrift(t *testing.T) {
	required := fpfIdentitySpec{
		Revision:          "0990ff1d1ccee4587b8f7e16e7a725a8edbe66b4",
		SpecDigest:        "sha256:1093a25640c61a2674f56443bffb8e27f33ac2cdf95f09af2c0cf67c68913eac",
		ReadmeDigest:      "sha256:6c8d87a641f36d34a9d84aa0ab8e7565dcca2a691482a0cee31bd28a743eb3fd",
		BaseTypeEnvRef:    "typeenv:sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6",
		BaseTypeEnvDigest: "sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6",
		CompilerSchema:    "fpf-base-typeenv.cov2.v4",
	}
	observed := fpfIdentity{
		Head:         required.Revision,
		SpecDigest:   required.SpecDigest,
		ReadmeDigest: required.ReadmeDigest,
		Embedded: embeddedFPFIdentity{
			Revision:                  required.Revision,
			SpecDigest:                required.SpecDigest,
			ReadmeDigest:              required.ReadmeDigest,
			BaseTypeEnvRef:            required.BaseTypeEnvRef,
			BaseTypeEnvDigest:         required.BaseTypeEnvDigest,
			BaseTypeEnvSourceRevision: required.Revision,
			CompilerSchema:            required.CompilerSchema,
		},
	}
	if err := validateRequiredFPFIdentity(required, observed); err != nil {
		t.Fatalf("exact FPF identity rejected: %v", err)
	}
	observed.Embedded.CompilerSchema = "drifted"
	if err := validateRequiredFPFIdentity(required, observed); err == nil {
		t.Fatal("embedded FPF identity drift unexpectedly passed")
	}
}

func TestRequiredFPFBaseRejectsDifferentTransitionBase(t *testing.T) {
	err := validateRequiredTransitionBase(
		"typeenv:sha256:required",
		"typeenv:sha256:different",
	)
	if err == nil {
		t.Fatal("Transition on a different base unexpectedly passed P13 identity")
	}
}

func TestRequiredTransitionPredecessorRejectsArbitraryAndSecondTransition(t *testing.T) {
	required := exactRequiredPredecessorSpec()
	observed := predecessorIdentityFromSpec(required)
	if err := validateRequiredPredecessorIdentity(required, observed); err != nil {
		t.Fatalf("exact 44dd881 predecessor rejected: %v", err)
	}
	cases := map[string]func(*predecessorIdentity){
		"arbitrary C": func(value *predecessorIdentity) {
			value.CompositeRef = "typeenv:sha256:arbitrary"
			value.CompositeDigest = "sha256:arbitrary"
		},
		"historical installed Q+B base": func(value *predecessorIdentity) {
			value.BaseTypeEnvRef = "typeenv:sha256:29cf6c6badbdef447e267a97617f74e7f826bddc1f8fdfcca73b46318d0e2e41"
			value.BaseTypeEnvDigest = "sha256:29cf6c6badbdef447e267a97617f74e7f826bddc1f8fdfcca73b46318d0e2e41"
		},
		"second current transition": func(value *predecessorIdentity) {
			value.HeadRevision = 2
			value.FPFRevision = "6e7eeb93d7d6208877649ac999d52ab845640817"
			value.CompilerSchema = "fpf-base-typeenv.cov2.v3"
		},
		"different executable snapshot": func(value *predecessorIdentity) {
			value.ExecutableSnapshotDigest = "sha256:different"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			drifted := observed
			mutate(&drifted)
			if err := validateRequiredPredecessorIdentity(required, drifted); err == nil {
				t.Fatal("drifted predecessor unexpectedly passed P13 identity")
			}
		})
	}
}

func TestStageProfileBindingRejectsPostStageProfileDrift(t *testing.T) {
	profile := profileIdentity{
		LedgerRevision:   1,
		BasisRef:         "project-profile-basis:sha256:basis-one",
		BasisDigest:      "sha256:basis-one",
		LedgerDigest:     "sha256:ledger-one",
		SupportDAGDigest: "sha256:support-one",
	}
	stage := stageProfileIdentity{
		SchemaEdition:            requiredStageSchemaEdition,
		LedgerRevision:           profile.LedgerRevision,
		LedgerDigest:             profile.LedgerDigest,
		FitRef:                   "project-typeenv-profile-fit:sha256:fit",
		FitDigest:                "sha256:fit",
		FitRuleEdition:           projecttypeenvprofilefit.AssessmentRuleEditionV1,
		FitPosture:               "compatible",
		TransitionProfilesRef:    "transition-projection-profile-compatibility-set:sha256:profiles",
		TransitionProfilesDigest: "sha256:profiles",
		TransitionProfileSet:     "sha256:profile-set",
		TransitionProfileCount:   2,
		TransitionPosturesDigest: "sha256:postures",
	}
	if err := validateStageProfileBinding(profile, stage); err != nil {
		t.Fatalf("exact profile/Stage binding rejected: %v", err)
	}
	profile.LedgerRevision = 2
	profile.BasisRef = "project-profile-basis:sha256:basis-two"
	profile.BasisDigest = "sha256:basis-two"
	profile.LedgerDigest = "sha256:ledger-two"
	profile.SupportDAGDigest = "sha256:support-two"
	if err := validateStageProfileBinding(profile, stage); err == nil {
		t.Fatal("profile admitted after Stage creation unexpectedly passed P13 identity")
	}
}

func TestRelevantByteIdentityIncludesUntrackedRootGoBuildInput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/p13-root-input\n\ngo 1.25\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, "root_input.go")
	if err := os.WriteFile(goFile, []byte("package rootinput\nconst Value = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := identitySpec{
		SourceFiles:          []string{"go.mod"},
		IncludeAllFiles:      true,
		IncludeGoBuildInputs: true,
	}
	first, err := digestRelevantBytes(root, spec)
	if err != nil {
		t.Fatalf("digest first build input: %v", err)
	}
	if err := os.WriteFile(goFile, []byte("package rootinput\nconst Value = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := digestRelevantBytes(root, spec)
	if err != nil {
		t.Fatalf("digest changed build input: %v", err)
	}
	if first.Digest == second.Digest {
		t.Fatal("root Go build-input byte drift did not change P13 identity")
	}
}

func TestDependencyIdentityBindsInstalledTreeBytes(t *testing.T) {
	root := t.TempDir()
	dependencyRoot := filepath.Join(root, "deps")
	if err := os.MkdirAll(dependencyRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(dependencyRoot, "runtime.js")
	if err := os.WriteFile(dependency, []byte("export const value = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := digestDependencyRoots(root, []string{"deps"})
	if err != nil {
		t.Fatalf("digest dependency tree: %v", err)
	}
	if err := os.WriteFile(dependency, []byte("export const value = 2;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := digestDependencyRoots(root, []string{"deps"})
	if err != nil {
		t.Fatalf("digest changed dependency tree: %v", err)
	}
	if first.Digest == second.Digest {
		t.Fatal("installed dependency-tree byte drift did not change P13 identity")
	}
}

func TestResolvedProfileGenerationPreservesExactV1(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open profile generation fixture: %v", err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	for _, table := range []string{
		"project_profile_revisions",
		"project_profile_revisions_v2",
		"project_profile_revisions_v3",
	} {
		statement := fmt.Sprintf(
			`CREATE TABLE %s (
				project_root TEXT NOT NULL,
				ledger_revision INTEGER NOT NULL,
				profile_payload_digest TEXT NOT NULL,
				admission_id TEXT NOT NULL,
				admission_digest TEXT NOT NULL
			)`,
			table,
		)
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
	}
	coordinates := resolvedProfileCoordinates{
		ProjectRoot:     "/project",
		LedgerRevision:  1,
		PayloadDigest:   "sha256:payload",
		AdmissionRef:    "profile-admission:one",
		AdmissionDigest: "sha256:admission",
	}
	_, err = database.Exec(
		`INSERT INTO project_profile_revisions (
			project_root, ledger_revision, profile_payload_digest,
			admission_id, admission_digest
		) VALUES (?, ?, ?, ?, ?)`,
		coordinates.ProjectRoot,
		coordinates.LedgerRevision,
		coordinates.PayloadDigest,
		coordinates.AdmissionRef,
		coordinates.AdmissionDigest,
	)
	if err != nil {
		t.Fatalf("insert exact v1 profile: %v", err)
	}
	transaction, err := sqlitetransaction.BeginRead(context.Background(), database)
	if err != nil {
		t.Fatalf("begin profile generation fixture: %v", err)
	}
	defer transaction.Rollback(context.Background())
	generation, err := captureExactProfileGeneration(
		context.Background(),
		transaction,
		coordinates,
	)
	if err != nil {
		t.Fatalf("capture exact v1 generation: %v", err)
	}
	if generation != "v1" {
		t.Fatalf("exact v1 generation = %q, want v1", generation)
	}
}

func TestExecutableIdentityBindsBytesModeAndSymlinkChain(t *testing.T) {
	directory := t.TempDir()
	firstTarget := filepath.Join(directory, "tool-v1")
	secondTarget := filepath.Join(directory, "tool-v2")
	link := filepath.Join(directory, "tool")
	if err := os.WriteFile(firstTarget, []byte("same executable bytes"), 0o755); err != nil {
		t.Fatalf("write first executable: %v", err)
	}
	if err := os.WriteFile(secondTarget, []byte("same executable bytes"), 0o755); err != nil {
		t.Fatalf("write second executable: %v", err)
	}
	if err := os.Symlink(filepath.Base(firstTarget), link); err != nil {
		t.Fatalf("create executable symlink: %v", err)
	}
	initial, err := digestExecutableSet(map[string]string{"tool": link})
	if err != nil {
		t.Fatalf("digest initial executable: %v", err)
	}
	if err := os.WriteFile(firstTarget, []byte("changed executable bytes"), 0o755); err != nil {
		t.Fatalf("change executable bytes: %v", err)
	}
	changedBytes, err := digestExecutableSet(map[string]string{"tool": link})
	if err != nil {
		t.Fatalf("digest changed executable: %v", err)
	}
	if changedBytes == initial {
		t.Fatal("executable byte drift did not change identity")
	}
	if err := os.WriteFile(firstTarget, []byte("same executable bytes"), 0o755); err != nil {
		t.Fatalf("restore executable bytes: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove executable symlink: %v", err)
	}
	if err := os.Symlink(filepath.Base(secondTarget), link); err != nil {
		t.Fatalf("replace executable symlink: %v", err)
	}
	changedLink, err := digestExecutableSet(map[string]string{"tool": link})
	if err != nil {
		t.Fatalf("digest retargeted executable: %v", err)
	}
	if changedLink == initial {
		t.Fatal("executable symlink drift did not change identity")
	}
	if err := os.Chmod(secondTarget, 0o744); err != nil {
		t.Fatalf("change executable mode: %v", err)
	}
	changedMode, err := digestExecutableSet(map[string]string{"tool": link})
	if err != nil {
		t.Fatalf("digest mode-changed executable: %v", err)
	}
	if changedMode == changedLink {
		t.Fatal("executable mode drift did not change identity")
	}
}

func TestExecutableIdentityBindsGoToolDirectoryBytes(t *testing.T) {
	directory := t.TempDir()
	tool := filepath.Join(directory, "compile")
	if err := os.WriteFile(tool, []byte("compile-v1"), 0o755); err != nil {
		t.Fatalf("write Go tool: %v", err)
	}
	initial, err := digestExecutableInputs(map[string]string{}, directory)
	if err != nil {
		t.Fatalf("digest Go tool directory: %v", err)
	}
	if err := os.WriteFile(tool, []byte("compile-v2"), 0o755); err != nil {
		t.Fatalf("change Go tool bytes: %v", err)
	}
	changed, err := digestExecutableInputs(map[string]string{}, directory)
	if err != nil {
		t.Fatalf("digest changed Go tool directory: %v", err)
	}
	if changed == initial {
		t.Fatal("Go tool byte drift did not change executable identity")
	}
}

func captureAcceptanceIdentity(
	root string,
	spec identitySpec,
) (acceptanceIdentity, error) {
	project, err := readProjectCarrier(root)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	projectionBytes, err := readProfileProjectionBytes(root)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	fpfState, err := captureFPFIdentity(root)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	if err := validateRequiredFPFIdentity(spec.RequiredFPF, fpfState); err != nil {
		return acceptanceIdentity{}, err
	}
	source, err := digestRelevantBytes(root, spec)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	gitState, err := captureGitIdentity(root)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	database, err := openProjectDatabaseReadOnly(project.ID)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	defer database.Close()
	databaseContext, cancelDatabase := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancelDatabase()
	stageStore, err := projecttypeenvstage.OpenExisting(databaseContext, database)
	if err != nil {
		return acceptanceIdentity{}, fmt.Errorf(
			"open exact project TypeEnv Stage store: %w",
			err,
		)
	}
	transaction, err := sqlitetransaction.BeginRead(databaseContext, database)
	if err != nil {
		return acceptanceIdentity{}, fmt.Errorf(
			"begin P13 identity read snapshot: %w",
			err,
		)
	}
	defer transaction.Rollback(context.Background())
	projectRoot, err := projectprofile.NewProjectRootV1(root)
	if err != nil {
		return acceptanceIdentity{}, fmt.Errorf("parse project root: %w", err)
	}
	projectID, err := projectidentity.ParseProjectID(project.ID)
	if err != nil {
		return acceptanceIdentity{}, fmt.Errorf("parse project ID: %w", err)
	}
	profile, currentProfile, err := captureProfileIdentity(
		transaction,
		projectRoot,
		projectionBytes,
	)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	graphObservation, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		databaseContext,
		transaction,
		projectID,
	)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	typeEnvironment, err := captureTypeEnvIdentity(
		databaseContext,
		transaction,
		stageStore,
		projectID,
		graphObservation,
		currentProfile,
		spec.RequiredPredecessor,
	)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	if err := validateRequiredTransitionBase(
		spec.RequiredFPF.BaseTypeEnvRef,
		typeEnvironment.TargetBaseRef,
	); err != nil {
		return acceptanceIdentity{}, err
	}
	graph, err := captureGraphIdentity(graphObservation, typeEnvironment)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	finish := transaction.Commit(databaseContext)
	if !finish.Succeeded() {
		return acceptanceIdentity{}, fmt.Errorf(
			"close P13 identity read snapshot: %w",
			finish.Err(),
		)
	}
	schemaTransaction, err := database.BeginTx(
		databaseContext,
		&sql.TxOptions{ReadOnly: true},
	)
	if err != nil {
		return acceptanceIdentity{}, fmt.Errorf(
			"begin P13 schema identity read snapshot: %w",
			err,
		)
	}
	defer schemaTransaction.Rollback()
	schemaState, err := captureSchemaIdentity(
		schemaTransaction,
		spec.RequiredSchemaVersion,
		spec.RequiredWriterGeneration,
	)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	if err := schemaTransaction.Commit(); err != nil {
		return acceptanceIdentity{}, fmt.Errorf(
			"close P13 schema identity read snapshot: %w",
			err,
		)
	}
	toolchain, err := captureToolchainIdentity(root, spec.DependencyRoots)
	if err != nil {
		return acceptanceIdentity{}, err
	}
	identity := acceptanceIdentity{
		Schema:      "haft.p13.acceptance-identity/v2",
		ProjectID:   project.ID,
		ProjectRoot: root,
		Source:      source,
		Git:         gitState,
		FPF:         fpfState,
		Toolchain:   toolchain,
		Profile:     profile,
		TypeEnv:     typeEnvironment,
		Graph:       graph,
		SchemaState: schemaState,
	}
	digest, err := digestCanonicalJSON(identity)
	if err != nil {
		return acceptanceIdentity{}, fmt.Errorf("digest P13 identity: %w", err)
	}
	identity.Digest = digest
	return identity, nil
}

func validateRequiredTransitionBase(required string, observed string) error {
	if observed != required {
		return fmt.Errorf(
			"post-P12E Transition base %q does not match required FPF base %q",
			observed,
			required,
		)
	}
	return nil
}

func exactRequiredPredecessorSpec() predecessorSpec {
	return predecessorSpec{
		HeadRevision:             requiredPriorHeadRevision,
		CompositeRef:             requiredPriorCompositeRef,
		BaseTypeEnvRef:           requiredPriorBaseRef,
		BaseTypeEnvDigest:        requiredPriorBaseDigest,
		FPFRevision:              requiredPriorFPFRevision,
		CompilerSchema:           requiredPriorCompilerSchema,
		ExecutableSnapshotDigest: requiredPriorSnapshotDigest,
		LoweredEnvironmentDigest: requiredPriorLoweredDigest,
	}
}

func predecessorIdentityFromSpec(spec predecessorSpec) predecessorIdentity {
	return predecessorIdentity{
		HeadRevision:             spec.HeadRevision,
		CompositeRef:             spec.CompositeRef,
		CompositeDigest:          strings.TrimPrefix(spec.CompositeRef, "typeenv:"),
		BaseTypeEnvRef:           spec.BaseTypeEnvRef,
		BaseTypeEnvDigest:        spec.BaseTypeEnvDigest,
		FPFRevision:              spec.FPFRevision,
		CompilerSchema:           spec.CompilerSchema,
		ExecutableSnapshotDigest: spec.ExecutableSnapshotDigest,
		LoweredEnvironmentDigest: spec.LoweredEnvironmentDigest,
	}
}

func validateRequiredPredecessorIdentity(
	required predecessorSpec,
	observed predecessorIdentity,
) error {
	want := predecessorIdentityFromSpec(required)
	if observed != want {
		return fmt.Errorf(
			"Transition predecessor is not the exact selected 44dd881 project-head closure",
		)
	}
	return nil
}

func validateStageProfileBinding(
	profile profileIdentity,
	stage stageProfileIdentity,
) error {
	values := []string{
		profile.BasisRef,
		profile.BasisDigest,
		profile.LedgerDigest,
		profile.SupportDAGDigest,
		stage.LedgerDigest,
		stage.FitRef,
		stage.FitDigest,
		stage.FitRuleEdition,
		stage.TransitionProfilesRef,
		stage.TransitionProfilesDigest,
		stage.TransitionProfileSet,
		stage.TransitionPosturesDigest,
	}
	if slices.Contains(values, "") {
		return fmt.Errorf("profile/Stage identity is incomplete")
	}
	if profile.LedgerRevision <= 0 ||
		stage.LedgerRevision != profile.LedgerRevision ||
		stage.LedgerDigest != profile.LedgerDigest {
		return fmt.Errorf("current profile ledger differs from the Transition Stage")
	}
	if profile.BasisRef != "project-profile-basis:"+profile.BasisDigest {
		return fmt.Errorf("current project-profile basis ref/digest are inconsistent")
	}
	if stage.SchemaEdition != requiredStageSchemaEdition ||
		stage.FitPosture != "compatible" ||
		stage.FitRuleEdition != projecttypeenvprofilefit.AssessmentRuleEditionV1 {
		return fmt.Errorf("Transition Stage profile-fit is not exact v5 compatible")
	}
	if stage.FitRef != "project-typeenv-profile-fit:"+stage.FitDigest {
		return fmt.Errorf("Transition Stage profile-fit ref/digest are inconsistent")
	}
	if stage.TransitionProfilesRef !=
		"transition-projection-profile-compatibility-set:"+stage.TransitionProfilesDigest ||
		stage.TransitionProfileCount <= 0 {
		return fmt.Errorf("Transition projection-profile identity is inconsistent")
	}
	return nil
}

func readProjectCarrier(root string) (projectCarrier, error) {
	path := filepath.Join(root, ".haft", "project.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return projectCarrier{}, fmt.Errorf("read project carrier: %w", err)
	}
	carrier := projectCarrier{}
	if err := yaml.Unmarshal(raw, &carrier); err != nil {
		return projectCarrier{}, fmt.Errorf("decode project carrier: %w", err)
	}
	if carrier.ID == "" {
		return projectCarrier{}, fmt.Errorf("project carrier has no project ID")
	}
	if !canonicalProjectID.MatchString(carrier.ID) {
		return projectCarrier{}, fmt.Errorf(
			"project carrier has non-canonical project ID %q",
			carrier.ID,
		)
	}
	return carrier, nil
}

func readProfileProjectionBytes(root string) ([]byte, error) {
	path := filepath.Join(root, ".haft", "project-profile.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"read profile projection: %w",
			err,
		)
	}
	return raw, nil
}

func digestRelevantBytes(root string, spec identitySpec) (byteIdentity, error) {
	paths, err := collectRelevantPaths(root, spec)
	if err != nil {
		return byteIdentity{}, err
	}
	return digestPaths(root, paths)
}

func collectRelevantPaths(root string, spec identitySpec) ([]string, error) {
	excluded := make(map[string]struct{}, len(spec.ExcludedDirectories))
	for _, directory := range spec.ExcludedDirectories {
		excluded[directory] = struct{}{}
	}
	pathSet := make(map[string]struct{})
	for _, relative := range spec.SourceFiles {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect source file %s: %w", relative, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("source file %s is a directory", relative)
		}
		pathSet[filepath.Clean(relative)] = struct{}{}
	}
	for _, relativeRoot := range spec.SourceRoots {
		absoluteRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		walkErr := filepath.WalkDir(
			absoluteRoot,
			func(path string, entry fs.DirEntry, walkErr error) error {
				return collectRelevantPath(
					root,
					path,
					entry,
					walkErr,
					excluded,
					pathSet,
				)
			},
		)
		if walkErr != nil {
			return nil, fmt.Errorf("walk source root %s: %w", relativeRoot, walkErr)
		}
	}
	if spec.IncludeGoBuildInputs {
		buildPaths, err := collectGoBuildInputPaths(root)
		if err != nil {
			return nil, err
		}
		for _, path := range buildPaths {
			pathSet[path] = struct{}{}
		}
	}
	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths, nil
}

func collectGoBuildInputPaths(root string) ([]string, error) {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return nil, err
	}
	raw, err := runIdentityCommand(root, goProgram, "list", "-json", "./...")
	if err != nil {
		return nil, fmt.Errorf("enumerate exact Go build inputs: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	pathSet := make(map[string]struct{})
	for {
		listed := goListPackage{}
		decodeErr := decoder.Decode(&listed)
		if decodeErr == io.EOF {
			break
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode Go build-input package: %w", decodeErr)
		}
		files := goListPackageFiles(listed)
		for _, file := range files {
			relative, err := repositoryRelativeBuildInput(root, listed.Dir, file)
			if err != nil {
				return nil, err
			}
			pathSet[relative] = struct{}{}
		}
	}
	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("Go build-input set is empty")
	}
	return paths, nil
}

func goListPackageFiles(listed goListPackage) []string {
	groups := [][]string{
		listed.GoFiles,
		listed.CgoFiles,
		listed.CFiles,
		listed.CXXFiles,
		listed.MFiles,
		listed.HFiles,
		listed.FFiles,
		listed.SFiles,
		listed.SwigFiles,
		listed.SwigCXXFiles,
		listed.SysoFiles,
		listed.EmbedFiles,
		listed.TestGoFiles,
		listed.TestEmbedFiles,
		listed.XTestGoFiles,
		listed.XTestEmbedFiles,
	}
	files := make([]string, 0)
	for _, group := range groups {
		files = append(files, group...)
	}
	return files
}

func repositoryRelativeBuildInput(
	root string,
	directory string,
	file string,
) (string, error) {
	if directory == "" || file == "" {
		return "", fmt.Errorf("Go build input has an empty directory or file")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("canonicalize Go build-input root: %w", err)
	}
	absolute := filepath.Join(directory, file)
	relative, err := filepath.Rel(canonicalRoot, absolute)
	if err != nil {
		return "", fmt.Errorf("relativize Go build input %s: %w", absolute, err)
	}
	relative = filepath.Clean(relative)
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("Go build input %s is outside repository root", absolute)
	}
	info, err := os.Lstat(filepath.Join(root, relative))
	if err != nil {
		return "", fmt.Errorf("inspect Go build input %s: %w", relative, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("Go build input %s is a directory", relative)
	}
	return relative, nil
}

func digestDependencyRoots(root string, roots []string) (byteIdentity, error) {
	pathSet := make(map[string]struct{})
	for _, relativeRoot := range roots {
		absoluteRoot := filepath.Join(root, filepath.FromSlash(relativeRoot))
		walkErr := filepath.WalkDir(
			absoluteRoot,
			func(path string, entry fs.DirEntry, walkErr error) error {
				return collectRelevantPath(
					root,
					path,
					entry,
					walkErr,
					map[string]struct{}{},
					pathSet,
				)
			},
		)
		if walkErr != nil {
			return byteIdentity{}, fmt.Errorf(
				"walk installed dependency root %s: %w",
				relativeRoot,
				walkErr,
			)
		}
	}
	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		return byteIdentity{}, fmt.Errorf("installed dependency-tree identity is empty")
	}
	return digestPaths(root, paths)
}

func collectRelevantPath(
	root string,
	path string,
	entry fs.DirEntry,
	walkErr error,
	excluded map[string]struct{},
	pathSet map[string]struct{},
) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() {
		if path == root {
			return nil
		}
		if _, skip := excluded[entry.Name()]; skip {
			return filepath.SkipDir
		}
		return nil
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("relativize source path %s: %w", path, err)
	}
	pathSet[relative] = struct{}{}
	return nil
}

func digestPaths(root string, paths []string) (byteIdentity, error) {
	hash := sha256.New()
	var byteCount int64
	for _, relative := range paths {
		absolute := filepath.Join(root, relative)
		content, mode, err := readIdentityPath(absolute)
		if err != nil {
			return byteIdentity{}, fmt.Errorf("read identity path %s: %w", relative, err)
		}
		if err := writeIdentityRecord(hash, relative, mode, content); err != nil {
			return byteIdentity{}, err
		}
		byteCount += int64(len(content))
	}
	return byteIdentity{
		Digest:    "sha256:" + hex.EncodeToString(hash.Sum(nil)),
		FileCount: len(paths),
		ByteCount: byteCount,
	}, nil
}

func readIdentityPath(path string) ([]byte, fs.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(path)
		return []byte(target), info.Mode(), readErr
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("identity path is not a regular file or symlink")
	}
	content, err := os.ReadFile(path)
	return content, info.Mode(), err
}

func writeIdentityRecord(
	writer io.Writer,
	relative string,
	mode fs.FileMode,
	content []byte,
) error {
	header := fmt.Sprintf("%s\x00%#o\x00%d\x00", filepath.ToSlash(relative), mode, len(content))
	if _, err := io.WriteString(writer, header); err != nil {
		return fmt.Errorf("hash identity header: %w", err)
	}
	if _, err := writer.Write(content); err != nil {
		return fmt.Errorf("hash identity content: %w", err)
	}
	if _, err := io.WriteString(writer, "\x00"); err != nil {
		return fmt.Errorf("hash identity delimiter: %w", err)
	}
	return nil
}

func captureGitIdentity(root string) (gitIdentity, error) {
	gitProgram, err := resolveExecutable("git")
	if err != nil {
		return gitIdentity{}, err
	}
	head, err := runIdentityCommand(root, gitProgram, "rev-parse", "HEAD")
	if err != nil {
		return gitIdentity{}, err
	}
	status, err := runIdentityCommand(
		root,
		gitProgram,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return gitIdentity{}, err
	}
	return gitIdentity{
		Head:         strings.TrimSpace(string(head)),
		StatusDigest: sha256Prefixed(status),
		StatusBytes:  len(status),
	}, nil
}

func captureFPFIdentity(root string) (fpfIdentity, error) {
	fpfRoot := filepath.Join(root, "data", "FPF")
	gitProgram, err := resolveExecutable("git")
	if err != nil {
		return fpfIdentity{}, err
	}
	head, err := runIdentityCommand(fpfRoot, gitProgram, "rev-parse", "HEAD")
	if err != nil {
		return fpfIdentity{}, err
	}
	status, err := runIdentityCommand(
		fpfRoot,
		gitProgram,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return fpfIdentity{}, err
	}
	paths, err := collectFPFPaths(root, fpfRoot)
	if err != nil {
		return fpfIdentity{}, err
	}
	source, err := digestPaths(root, paths)
	if err != nil {
		return fpfIdentity{}, err
	}
	specBytes, err := os.ReadFile(filepath.Join(fpfRoot, "FPF-Spec.md"))
	if err != nil {
		return fpfIdentity{}, fmt.Errorf("read bundled FPF specification: %w", err)
	}
	readmeBytes, err := os.ReadFile(
		filepath.Join(fpfRoot, p13FPFReadmeFileName),
	)
	if err != nil {
		return fpfIdentity{}, fmt.Errorf("read bundled FPF README: %w", err)
	}
	embedded, err := captureEmbeddedFPFIdentity(root)
	if err != nil {
		return fpfIdentity{}, err
	}
	return fpfIdentity{
		Head:         strings.TrimSpace(string(head)),
		StatusDigest: sha256Prefixed(status),
		StatusBytes:  len(status),
		SpecDigest:   sha256Prefixed(specBytes),
		ReadmeDigest: sha256Prefixed(readmeBytes),
		Source:       source,
		Embedded:     embedded,
	}, nil
}

func captureEmbeddedFPFIdentity(root string) (embeddedFPFIdentity, error) {
	path := filepath.Join(root, "internal", "cli", "fpf.db")
	database, err := openSQLiteReadOnly(path)
	if err != nil {
		return embeddedFPFIdentity{}, fmt.Errorf("open embedded FPF index: %w", err)
	}
	defer database.Close()
	keys := []string{
		"fpf_commit",
		"spec_document_digest",
		"readme_document_digest",
		"typeenv_ref",
		"typeenv_artifact_digest",
		"typeenv_source_revision",
		"typeenv_compiler_schema_version",
	}
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		value := ""
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := database.QueryRowContext(
			ctx,
			"SELECT value FROM meta WHERE key = ?",
			key,
		).Scan(&value)
		cancel()
		if err != nil {
			return embeddedFPFIdentity{}, fmt.Errorf(
				"read embedded FPF metadata %q: %w",
				key,
				err,
			)
		}
		values = append(values, value)
	}
	return embeddedFPFIdentity{
		Revision:                  values[0],
		SpecDigest:                values[1],
		ReadmeDigest:              values[2],
		BaseTypeEnvRef:            values[3],
		BaseTypeEnvDigest:         values[4],
		BaseTypeEnvSourceRevision: values[5],
		CompilerSchema:            values[6],
	}, nil
}

func validateRequiredFPFIdentity(
	required fpfIdentitySpec,
	observed fpfIdentity,
) error {
	embedded := embeddedFPFIdentity{
		Revision:                  required.Revision,
		SpecDigest:                required.SpecDigest,
		ReadmeDigest:              required.ReadmeDigest,
		BaseTypeEnvRef:            required.BaseTypeEnvRef,
		BaseTypeEnvDigest:         required.BaseTypeEnvDigest,
		BaseTypeEnvSourceRevision: required.Revision,
		CompilerSchema:            required.CompilerSchema,
	}
	if observed.Head != required.Revision ||
		observed.SpecDigest != required.SpecDigest ||
		observed.ReadmeDigest != required.ReadmeDigest ||
		observed.Embedded != embedded {
		return fmt.Errorf(
			"P13 FPF identity is not the exact accepted 1d5 checkout and embedded index",
		)
	}
	return nil
}

func collectFPFPaths(root string, fpfRoot string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(
		fpfRoot,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Name() == ".git" {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			relative, relativeErr := filepath.Rel(root, path)
			if relativeErr != nil {
				return relativeErr
			}
			paths = append(paths, relative)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("walk bundled FPF source: %w", err)
	}
	slices.Sort(paths)
	return paths, nil
}

func captureToolchainIdentity(
	root string,
	dependencyRoots []string,
) (toolchainIdentity, error) {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return toolchainIdentity{}, err
	}
	gofmtProgram := filepath.Join(filepath.Dir(goProgram), "gofmt")
	if !isExecutableFile(gofmtProgram) {
		return toolchainIdentity{}, fmt.Errorf(
			"resolve gofmt executable %q",
			gofmtProgram,
		)
	}
	gitProgram, err := resolveExecutable("git")
	if err != nil {
		return toolchainIdentity{}, err
	}
	bashProgram, err := resolveExecutable("bash")
	if err != nil {
		return toolchainIdentity{}, err
	}
	pythonSelection, err := runIdentityCommand(
		root,
		bashProgram,
		"scripts/fpf_query_token_gate.sh",
		"--print-bootstrap-python",
	)
	if err != nil {
		return toolchainIdentity{}, fmt.Errorf(
			"resolve exact Query token-gate Python: %w",
			err,
		)
	}
	pythonProgram := normalizedCommandText(pythonSelection)
	if !filepath.IsAbs(pythonProgram) || !isExecutableFile(pythonProgram) {
		return toolchainIdentity{}, fmt.Errorf(
			"Query token-gate Python %q is not an absolute executable",
			pythonProgram,
		)
	}
	pnpmProgram, err := resolveExecutable("pnpm")
	if err != nil {
		return toolchainIdentity{}, err
	}
	nodeProgram, err := resolveNodeExecutable(pnpmProgram)
	if err != nil {
		return toolchainIdentity{}, err
	}
	mixProgram, err := resolveExecutable("mix")
	if err != nil {
		return toolchainIdentity{}, err
	}
	elixirProgram, err := resolveExecutable("elixir")
	if err != nil {
		return toolchainIdentity{}, err
	}
	erlProgram, err := resolveExecutable("erl")
	if err != nil {
		return toolchainIdentity{}, err
	}
	unameProgram, err := resolveExecutable("uname")
	if err != nil {
		return toolchainIdentity{}, err
	}
	goVersion, err := runIdentityCommand(root, goProgram, "version")
	if err != nil {
		return toolchainIdentity{}, err
	}
	goEnvironment, err := runIdentityCommand(
		root,
		goProgram,
		"env",
		"-json",
		"GOVERSION",
		"GOOS",
		"GOARCH",
		"GOAMD64",
		"CGO_ENABLED",
		"CC",
		"CXX",
		"CGO_CFLAGS",
		"CGO_CPPFLAGS",
		"CGO_CXXFLAGS",
		"CGO_LDFLAGS",
		"GOEXPERIMENT",
		"GOFLAGS",
		"GOTOOLCHAIN",
		"GOWORK",
		"GOMOD",
		"GOMODCACHE",
		"GOCACHE",
		"GOTOOLDIR",
	)
	if err != nil {
		return toolchainIdentity{}, err
	}
	goCoordinates := goEnvironmentCoordinates{}
	if err := json.Unmarshal(goEnvironment, &goCoordinates); err != nil {
		return toolchainIdentity{}, fmt.Errorf("decode Go environment coordinates: %w", err)
	}
	gitVersion, err := runIdentityCommand(root, gitProgram, "--version")
	if err != nil {
		return toolchainIdentity{}, err
	}
	bashVersion, err := runIdentityCommand(root, bashProgram, "--version")
	if err != nil {
		return toolchainIdentity{}, err
	}
	pythonRuntime, err := runIdentityCommand(
		root,
		pythonProgram,
		"-c",
		`import json, platform, ssl, sys, sysconfig
print(json.dumps({
    "base_prefix": sys.base_prefix,
    "executable": sys.executable,
    "implementation": platform.python_implementation(),
    "openssl": ssl.OPENSSL_VERSION,
    "platform": platform.platform(),
    "stdlib": sysconfig.get_paths()["stdlib"],
    "version": platform.python_version(),
}, sort_keys=True, separators=(",", ":")))`,
	)
	if err != nil {
		return toolchainIdentity{}, err
	}
	nodeVersion, err := runIdentityCommand(
		root,
		nodeProgram,
		"--version",
	)
	if err != nil {
		return toolchainIdentity{}, err
	}
	pnpmVersion, err := runIdentityCommand(root, pnpmProgram, "--version")
	if err != nil {
		return toolchainIdentity{}, err
	}
	mixVersion, err := runIdentityCommand(root, mixProgram, "--version")
	if err != nil {
		return toolchainIdentity{}, err
	}
	elixirVersion, err := runIdentityCommand(root, elixirProgram, "--version")
	if err != nil {
		return toolchainIdentity{}, err
	}
	platform, err := runIdentityCommand(root, unameProgram, "-a")
	if err != nil {
		return toolchainIdentity{}, err
	}
	moduleGraph, err := runIdentityCommand(
		root,
		goProgram,
		"list",
		"-m",
		"-json",
		"all",
	)
	if err != nil {
		return toolchainIdentity{}, err
	}
	executables := map[string]string{
		"bash":   bashProgram,
		"elixir": elixirProgram,
		"erl":    erlProgram,
		"git":    gitProgram,
		"go":     goProgram,
		"gofmt":  gofmtProgram,
		"mix":    mixProgram,
		"node":   nodeProgram,
		"pnpm":   pnpmProgram,
		"python": pythonProgram,
		"uname":  unameProgram,
	}
	for _, utility := range []string{"dirname", "env", "grep", "mktemp", "rm", "tee"} {
		program, err := resolveAcceptanceExecutable(root, utility)
		if err != nil {
			return toolchainIdentity{}, err
		}
		executables[utility] = program
	}
	if goCoordinates.CGOEnabled == "1" {
		ccProgram, err := resolveConfiguredExecutable("CC", goCoordinates.CC)
		if err != nil {
			return toolchainIdentity{}, err
		}
		cxxProgram, err := resolveConfiguredExecutable("CXX", goCoordinates.CXX)
		if err != nil {
			return toolchainIdentity{}, err
		}
		executables["cc"] = ccProgram
		executables["cxx"] = cxxProgram
	}
	executableDigest, err := digestExecutableInputs(executables, goCoordinates.GoToolDir)
	if err != nil {
		return toolchainIdentity{}, fmt.Errorf("digest executable identities: %w", err)
	}
	dependencies, err := digestDependencyRoots(root, dependencyRoots)
	if err != nil {
		return toolchainIdentity{}, fmt.Errorf("digest installed dependency trees: %w", err)
	}
	environmentDigest, err := digestCanonicalJSON(capturedEnvironment())
	if err != nil {
		return toolchainIdentity{}, fmt.Errorf("digest execution environment: %w", err)
	}
	return toolchainIdentity{
		GoVersion:         normalizedCommandText(goVersion),
		GoEnvironment:     normalizedCommandText(goEnvironment),
		GitVersion:        normalizedCommandText(gitVersion),
		BashVersion:       normalizedCommandText(bashVersion),
		PythonRuntime:     normalizedCommandText(pythonRuntime),
		NodeVersion:       normalizedCommandText(nodeVersion),
		PNPMVersion:       normalizedCommandText(pnpmVersion),
		MixVersion:        normalizedCommandText(mixVersion),
		ElixirVersion:     normalizedCommandText(elixirVersion),
		Platform:          normalizedCommandText(platform),
		ExecutableDigest:  executableDigest,
		EnvironmentDigest: environmentDigest,
		ModuleGraphDigest: sha256Prefixed(moduleGraph),
		DependencyDigest:  dependencies.Digest,
		DependencyFiles:   dependencies.FileCount,
		DependencyBytes:   dependencies.ByteCount,
	}, nil
}

func digestExecutableSet(executables map[string]string) (string, error) {
	return digestExecutableInputs(executables, "")
}

func digestExecutableInputs(
	executables map[string]string,
	goToolDirectory string,
) (string, error) {
	names := make([]string, 0, len(executables))
	for name := range executables {
		names = append(names, name)
	}
	slices.Sort(names)
	hash := sha256.New()
	for _, name := range names {
		if err := writeExecutableIdentity(hash, name, executables[name]); err != nil {
			return "", err
		}
	}
	if goToolDirectory != "" {
		if err := writeGoToolDirectoryIdentity(hash, goToolDirectory); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func writeGoToolDirectoryIdentity(writer io.Writer, directory string) error {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("absolutize Go tool directory: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return fmt.Errorf("inspect Go tool directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("Go tool directory is not a directory")
	}
	directoryLabel := "go-tool-directory/" + filepath.ToSlash(absolute)
	if err := writeIdentityRecord(writer, directoryLabel, info.Mode(), nil); err != nil {
		return err
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return fmt.Errorf("read Go tool directory: %w", err)
	}
	executableCount := 0
	for _, entry := range entries {
		path := filepath.Join(absolute, entry.Name())
		entryInfo, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect Go tool %s: %w", entry.Name(), err)
		}
		isSymlink := entryInfo.Mode()&os.ModeSymlink != 0
		isExecutable := entryInfo.Mode().IsRegular() && entryInfo.Mode().Perm()&0o111 != 0
		if !isSymlink && !isExecutable {
			continue
		}
		if err := writeExecutableIdentity(writer, "go-tool/"+entry.Name(), path); err != nil {
			return err
		}
		executableCount++
	}
	if executableCount == 0 {
		return fmt.Errorf("Go tool directory contains no executable tools")
	}
	return nil
}

func writeExecutableIdentity(
	writer io.Writer,
	name string,
	path string,
) error {
	current, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("absolutize executable %s: %w", name, err)
	}
	seen := make(map[string]struct{})
	for hop := 0; hop < 64; hop++ {
		current = filepath.Clean(current)
		if _, exists := seen[current]; exists {
			return fmt.Errorf("executable %s has a symlink cycle", name)
		}
		seen[current] = struct{}{}
		content, mode, err := readIdentityPath(current)
		if err != nil {
			return fmt.Errorf("read executable %s hop %d: %w", name, hop, err)
		}
		label := fmt.Sprintf("%s/hop-%02d/%s", name, hop, filepath.ToSlash(current))
		if err := writeIdentityRecord(writer, label, mode, content); err != nil {
			return err
		}
		if mode&os.ModeSymlink == 0 {
			if !mode.IsRegular() || mode.Perm()&0o111 == 0 {
				return fmt.Errorf("executable %s target is not an executable regular file", name)
			}
			return nil
		}
		target := string(content)
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		current = target
	}
	return fmt.Errorf("executable %s exceeds the symlink traversal limit", name)
}

func capturedEnvironment() map[string]string {
	keys := []string{
		"CGO_ENABLED",
		"CI",
		"GOFLAGS",
		"GOMAXPROCS",
		"GOTOOLCHAIN",
		"HAFT_QUERY_TOKEN_GATE_BOOTSTRAP_PYTHON",
		"HOME",
		"LANG",
		"LC_ALL",
		"ERL_AFLAGS",
		"ERL_FLAGS",
		"HEX_HOME",
		"MIX_ENV",
		"MIX_HOME",
		"NODE_ENV",
		"NODE_OPTIONS",
		"PATH",
		"PNPM_HOME",
		"TMPDIR",
		"TZ",
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = os.Getenv(key)
	}
	return result
}

func runIdentityCommand(
	workingDirectory string,
	program string,
	args ...string,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = workingDirectory
	command.Env = acceptanceEnvironment(false)
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("identity command %s timed out: %w", program, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf(
			"identity command %s failed: %w: %s",
			program,
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return stdout.Bytes(), nil
}

func normalizedCommandText(raw []byte) string {
	return strings.TrimSpace(strings.ReplaceAll(string(raw), "\r\n", "\n"))
}

func resolveExecutable(program string) (string, error) {
	if program == "go" {
		candidate := filepath.Join(runtime.GOROOT(), "bin", "go")
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	resolved, err := exec.LookPath(program)
	if err == nil {
		return resolved, nil
	}
	candidates := []string{
		filepath.Join("/opt/homebrew/bin", program),
		filepath.Join("/usr/local/bin", program),
	}
	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("resolve executable %q: %w", program, err)
}

func resolveConfiguredExecutable(label string, command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("Go environment %s executable is empty", label)
	}
	program := strings.Trim(parts[0], "\"'")
	if filepath.IsAbs(program) {
		if !isExecutableFile(program) {
			return "", fmt.Errorf("Go environment %s executable %q is unavailable", label, program)
		}
		return program, nil
	}
	resolved, err := resolveExecutable(program)
	if err != nil {
		return "", fmt.Errorf("resolve Go environment %s executable: %w", label, err)
	}
	return resolved, nil
}

func acceptanceSearchPath() string {
	entries := []string{
		filepath.Join(runtime.GOROOT(), "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		os.Getenv("PATH"),
	}
	return strings.Join(entries, string(os.PathListSeparator))
}

func resolveAcceptanceExecutable(root string, program string) (string, error) {
	if strings.ContainsRune(program, os.PathSeparator) {
		candidate := program
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(root, candidate)
		}
		if !isExecutableFile(candidate) {
			return "", fmt.Errorf("resolve acceptance executable %q", program)
		}
		return candidate, nil
	}
	for _, directory := range filepath.SplitList(acceptanceSearchPath()) {
		if directory == "" {
			directory = root
		}
		candidate := filepath.Join(directory, program)
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("resolve acceptance executable %q", program)
}

func resolveNodeExecutable(pnpmProgram string) (string, error) {
	resolved, err := exec.LookPath("node")
	if err == nil {
		return resolved, nil
	}
	pnpmDirectory := filepath.Dir(pnpmProgram)
	candidates := []string{
		filepath.Join("/opt/homebrew/opt/node/bin", "node"),
		filepath.Join("/usr/local/opt/node/bin", "node"),
		filepath.Clean(filepath.Join(pnpmDirectory, "..", "..", "node", "bin", "node")),
	}
	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("resolve Node.js executable: %w", err)
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func openProjectDatabaseReadOnly(projectID string) (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home for project database: %w", err)
	}
	path := filepath.Join(home, ".haft", "projects", projectID, "haft.db")
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect project database %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project database %s is not a regular file", path)
	}
	database, err := openSQLiteReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("open project database read-only: %w", err)
	}
	return database, nil
}

func openSQLiteReadOnly(path string) (*sql.DB, error) {
	dsnURL := url.URL{Scheme: "file", Path: path}
	query := dsnURL.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "query_only(1)")
	dsnURL.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", dsnURL.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite database read-only: %w", err)
	}
	database.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping SQLite database read-only: %w", err)
	}
	return database, nil
}

func captureProfileIdentity(
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
	projectionBytes []byte,
) (
	profileIdentity,
	projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile,
	error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	current, err := profileadmissionsqlite.ResolveCurrentWithin(
		ctx,
		transaction,
		root,
	)
	if err != nil {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, fmt.Errorf(
			"resolve canonical profile in P13 snapshot: %w",
			err,
		)
	}
	declared, ok := current.(profileadmissionsqlite.DeclaredCurrentCanonicalProfile)
	if !ok {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, fmt.Errorf(
			"P13 requires one declared canonical profile, got %T",
			current,
		)
	}
	admission := declared.Admission()
	if !admission.Valid() {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, fmt.Errorf("resolved canonical profile is invalid")
	}
	projectionDigest, err := profileprojection.VerifyExactProjectionBytes(
		admission,
		projectionBytes,
	)
	if err != nil {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, fmt.Errorf("verify exact profile projection: %w", err)
	}
	basis, err := profilebasissqlite.FromCanonicalAdmission(admission)
	if err != nil {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, fmt.Errorf(
			"derive current project-profile basis: %w",
			err,
		)
	}
	if err := basis.Verify(); err != nil {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, fmt.Errorf(
			"verify current project-profile basis: %w",
			err,
		)
	}
	generation, err := captureResolvedProfileGeneration(ctx, transaction, admission)
	if err != nil {
		return profileIdentity{}, projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile{}, err
	}
	identity := profileIdentity{
		Generation:           generation,
		LedgerRevision:       int64(admission.LedgerRevision().Value()),
		PayloadDigest:        admission.PayloadDigest().String(),
		AdmissionRef:         admission.AdmissionRecordRef().String(),
		AdmissionDigest:      admission.AdmissionRecordDigest().String(),
		ProjectionDigest:     projectionDigest.String(),
		ProjectionSchema:     profileprojection.ProjectionSchemaV1,
		ProjectionLedgerHead: int64(admission.LedgerRevision().Value()),
		BasisRef:             basis.ProfileBasisRef().String(),
		BasisDigest:          basis.Digest().String(),
		LedgerDigest:         basis.ProfileLedgerDigest().String(),
		SupportDAGDigest:     basis.SupportDAGDigest().String(),
	}
	return identity, basis, nil
}

func captureResolvedProfileGeneration(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (string, error) {
	coordinates := resolvedProfileCoordinates{
		ProjectRoot:     admission.ProjectRoot().String(),
		LedgerRevision:  int64(admission.LedgerRevision().Value()),
		PayloadDigest:   admission.PayloadDigest().String(),
		AdmissionRef:    admission.AdmissionRecordRef().String(),
		AdmissionDigest: admission.AdmissionRecordDigest().String(),
	}
	return captureExactProfileGeneration(ctx, transaction, coordinates)
}

func captureExactProfileGeneration(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	coordinates resolvedProfileCoordinates,
) (string, error) {
	const query = `WITH exact_generation AS (
		SELECT 'v1' AS generation FROM project_profile_revisions
		WHERE project_root = ? AND ledger_revision = ?
			AND profile_payload_digest = ? AND admission_id = ?
			AND admission_digest = ?
		UNION ALL
		SELECT 'v2' AS generation FROM project_profile_revisions_v2
		WHERE project_root = ? AND ledger_revision = ?
			AND profile_payload_digest = ? AND admission_id = ?
			AND admission_digest = ?
		UNION ALL
		SELECT 'v3' AS generation FROM project_profile_revisions_v3
		WHERE project_root = ? AND ledger_revision = ?
			AND profile_payload_digest = ? AND admission_id = ?
			AND admission_digest = ?
	)
	SELECT COALESCE(MIN(generation), ''), COUNT(*) FROM exact_generation`
	coordinateArguments := []any{
		coordinates.ProjectRoot,
		coordinates.LedgerRevision,
		coordinates.PayloadDigest,
		coordinates.AdmissionRef,
		coordinates.AdmissionDigest,
	}
	arguments := make([]any, 0, len(coordinateArguments)*3)
	arguments = append(arguments, coordinateArguments...)
	arguments = append(arguments, coordinateArguments...)
	arguments = append(arguments, coordinateArguments...)
	generation := ""
	count := int64(0)
	err := transaction.ScanOne(
		ctx,
		query,
		arguments,
		[]any{&generation, &count},
	)
	if err != nil {
		return "", fmt.Errorf("read resolved profile storage generation: %w", err)
	}
	if count != 1 || generation == "" {
		return "", fmt.Errorf(
			"resolved profile admission has %d exact storage generations",
			count,
		)
	}
	return generation, nil
}

func captureTypeEnvIdentity(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	stageStore *projecttypeenvstage.Store,
	projectID projectidentity.ProjectID,
	graphObservation typedmemorystore.CurrentProjectGraphObservation,
	currentProfile projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile,
	requiredPredecessor predecessorSpec,
) (typeEnvIdentity, error) {
	if err := graphObservation.Verify(); err != nil {
		return typeEnvIdentity{}, fmt.Errorf("verify current graph observation: %w", err)
	}
	if stageStore == nil {
		return typeEnvIdentity{}, fmt.Errorf("exact project TypeEnv Stage store is required")
	}
	headRevisionRaw := int64(0)
	selectedRaw := ""
	err := transaction.ScanOne(
		ctx,
		`SELECT head_revision, selected_composite_ref
		FROM project_typeenv_heads WHERE project_id = ?`,
		[]any{projectID.String()},
		[]any{&headRevisionRaw, &selectedRaw},
	)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf("read current project TypeEnv coordinate: %w", err)
	}
	if headRevisionRaw <= 0 {
		return typeEnvIdentity{}, fmt.Errorf("current project TypeEnv head is not committed")
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(uint64(headRevisionRaw))
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf("parse current TypeEnv head revision: %w", err)
	}
	selected, err := typedmemory.ParseTypeEnvRef(selectedRaw)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf("parse current selected TypeEnv: %w", err)
	}
	if selected != graphObservation.ActiveTypeEnv() {
		return typeEnvIdentity{}, fmt.Errorf(
			"current TypeEnv head differs from verified graph observation",
		)
	}
	head, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(projectID)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf("derive current TypeEnv head ref: %w", err)
	}
	loader := projecttypeenvselectioneffectsqlite.NewCurrentCommittedClosureLoader()
	closure, err := loader.LoadCommittedClosureForCurrentHeadTx(
		ctx,
		transaction,
		projectID,
		graphObservation.GraphSnapshotBasis().GraphRevision(),
		head,
		headRevision,
		selected,
	)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf(
			"load verified current TypeEnv selection closure: %w",
			err,
		)
	}
	ready, err := stageStore.LoadSelectionReadyTx(
		ctx,
		transaction,
		closure.Target().Stage(),
	)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf(
			"load exact selection-ready TypeEnv closure: %w",
			err,
		)
	}
	prior, transition := closure.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !transition {
		return typeEnvIdentity{}, fmt.Errorf(
			"P13 requires an exact post-P12E Transition closure; Genesis is not sufficient",
		)
	}
	predecessor, priorSnapshot, err := captureRequiredTransitionPredecessor(
		ctx,
		transaction,
		stageStore,
		prior,
		requiredPredecessor,
	)
	if err != nil {
		return typeEnvIdentity{}, err
	}
	profileStage, err := verifySelectionReadyTransitionProfileClosure(
		currentProfile,
		priorSnapshot,
		ready,
	)
	if err != nil {
		return typeEnvIdentity{}, err
	}
	readyClosureDigest, err := verifySelectionReadyTransitionClosure(
		closure,
		ready,
		predecessor,
		profileStage,
	)
	if err != nil {
		return typeEnvIdentity{}, err
	}
	return typeEnvIdentityFromVerifiedTransitionClosure(
		closure,
		readyClosureDigest,
		predecessor,
		profileStage,
	)
}

func captureRequiredTransitionPredecessor(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	stageStore *projecttypeenvstage.Store,
	prior projecttypeenvselection.TransitionStagePredecessor,
	required predecessorSpec,
) (
	predecessorIdentity,
	projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	error,
) {
	priorSnapshot, err := stageStore.LoadExecutableSnapshotTx(
		ctx,
		transaction,
		prior.SelectedComposite(),
	)
	if err != nil {
		return predecessorIdentity{}, projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"load exact predecessor B/E/X/C executable closure: %w",
			err,
		)
	}
	if err := priorSnapshot.Verify(); err != nil {
		return predecessorIdentity{}, projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"verify exact predecessor executable closure: %w",
			err,
		)
	}
	record := priorSnapshot.Record()
	if record.TypeEnvRef() != prior.SelectedComposite() {
		return predecessorIdentity{}, projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, fmt.Errorf(
			"predecessor executable closure resolves another C",
		)
	}
	observed := predecessorIdentity{
		HeadRevision:             int64(prior.HeadRevision().Value()),
		CompositeRef:             prior.SelectedComposite().String(),
		CompositeDigest:          prior.SelectedComposite().Digest().String(),
		BaseTypeEnvRef:           record.BaseTypeEnvRef().String(),
		BaseTypeEnvDigest:        record.BaseTypeEnvRef().Digest().String(),
		FPFRevision:              record.SourceRevision().String(),
		CompilerSchema:           record.CompilerSchemaVersion().String(),
		ExecutableSnapshotDigest: priorSnapshot.Digest().String(),
		LoweredEnvironmentDigest: priorSnapshot.LoweredEnvironmentDigest().String(),
	}
	if err := validateRequiredPredecessorIdentity(required, observed); err != nil {
		return predecessorIdentity{}, projecttypeenv.ProjectTypeEnvExecutableSnapshot{}, err
	}
	return observed, priorSnapshot, nil
}

func verifySelectionReadyTransitionProfileClosure(
	currentProfile projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile,
	priorSnapshot projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	ready projecttypeenvstage.SelectionReadyStage,
) (stageProfileIdentity, error) {
	if err := currentProfile.Verify(); err != nil {
		return stageProfileIdentity{}, fmt.Errorf("verify current profile basis: %w", err)
	}
	if err := priorSnapshot.Verify(); err != nil {
		return stageProfileIdentity{}, fmt.Errorf("verify predecessor executable snapshot: %w", err)
	}
	if err := ready.Verify(); err != nil {
		return stageProfileIdentity{}, fmt.Errorf("verify selection-ready Stage: %w", err)
	}
	stage := ready.Stage()
	if stage.SchemaEdition() != projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV5 {
		return stageProfileIdentity{}, fmt.Errorf("P13 requires exact Transition Stage schema v5")
	}
	if stage.ProfileLedgerRevision() != currentProfile.LedgerRevision() ||
		stage.ProfileLedgerDigest() != currentProfile.ProfileLedgerDigest() {
		return stageProfileIdentity{}, fmt.Errorf(
			"current canonical profile was admitted after the Transition Stage basis",
		)
	}
	currentFit, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		currentProfile,
		ready.ExecutableSnapshot(),
	)
	if err != nil {
		return stageProfileIdentity{}, fmt.Errorf("recompute current Stage profile-fit: %w", err)
	}
	if _, compatible := currentFit.(projecttypeenvprofilefit.Compatible); !compatible {
		return stageProfileIdentity{}, fmt.Errorf(
			"current project profile is not compatible with the selected TypeEnv",
		)
	}
	storedFit := stage.ProfileCompatibility()
	if err := storedFit.Verify(); err != nil {
		return stageProfileIdentity{}, fmt.Errorf("verify stored Stage profile-fit: %w", err)
	}
	if storedFit.BasisRef() != currentProfile.ProfileBasisRef() ||
		storedFit.BasisDigest() != currentProfile.Digest() ||
		storedFit.TargetTypeEnvRef() != ready.ExecutableSnapshot().TypeEnvRef() ||
		storedFit.TargetSnapshotDigest() != ready.ExecutableSnapshot().Digest() ||
		storedFit.RuleEdition() != projecttypeenvprofilefit.CurrentRuleEdition() ||
		storedFit.FitRef() != stage.ProfileFitRef() ||
		storedFit.Digest() != stage.ProfileFitDigest() ||
		storedFit.FitRef() != currentFit.FitRef() ||
		storedFit.Digest() != currentFit.Digest() ||
		!bytes.Equal(storedFit.CanonicalBytes(), currentFit.CanonicalBytes()) {
		return stageProfileIdentity{}, fmt.Errorf(
			"Transition Stage profile-fit differs from the exact current profile basis",
		)
	}
	diff, err := projecttypeenvcompatibility.CompareSuccessor(
		priorSnapshot.Environment(),
		ready.ExecutableTypeEnv(),
	)
	if err != nil {
		return stageProfileIdentity{}, fmt.Errorf(
			"recompute exact predecessor-to-target successor diff: %w",
			err,
		)
	}
	currentProfiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(diff)
	if err != nil {
		return stageProfileIdentity{}, fmt.Errorf(
			"recompute installed projection-profile compatibility: %w",
			err,
		)
	}
	storedProfiles, exists := stage.TransitionProjectionProfileCompatibility()
	if !exists {
		return stageProfileIdentity{}, fmt.Errorf(
			"Transition Stage v5 has no projection-profile compatibility closure",
		)
	}
	storedProfilesRef, refExists := stage.TransitionProjectionProfileCompatibilityRef()
	storedProfilesDigest, digestExists := stage.TransitionProjectionProfileCompatibilityDigest()
	if !refExists ||
		!digestExists ||
		storedProfilesRef != storedProfiles.Ref() ||
		storedProfilesDigest != storedProfiles.Digest() ||
		storedProfiles.Ref() != currentProfiles.Ref() ||
		storedProfiles.Digest() != currentProfiles.Digest() ||
		!bytes.Equal(storedProfiles.CanonicalBytes(), currentProfiles.CanonicalBytes()) {
		return stageProfileIdentity{}, fmt.Errorf(
			"Transition Stage projection-profile closure differs from the installed catalog",
		)
	}
	profiles, err := projecttypeenvprofilecompatibility.DecodeTransitionProjectionProfiles(
		currentProfiles,
	)
	if err != nil {
		return stageProfileIdentity{}, fmt.Errorf(
			"decode current Transition projection-profile closure: %w",
			err,
		)
	}
	if profiles.HasBlockedProfile() {
		return stageProfileIdentity{}, fmt.Errorf(
			"Transition blocks one or more installed projection profiles",
		)
	}
	postures := make([]string, 0, len(profiles.Profiles()))
	for _, profile := range profiles.Profiles() {
		postures = append(postures, profile.Kind().String())
	}
	posturesDigest, err := digestCanonicalJSON(postures)
	if err != nil {
		return stageProfileIdentity{}, fmt.Errorf(
			"digest Transition projection-profile postures: %w",
			err,
		)
	}
	identity := stageProfileIdentity{
		SchemaEdition:            stage.SchemaEdition(),
		LedgerRevision:           int64(stage.ProfileLedgerRevision().Value()),
		LedgerDigest:             stage.ProfileLedgerDigest().String(),
		FitRef:                   storedFit.FitRef().String(),
		FitDigest:                storedFit.Digest().String(),
		FitRuleEdition:           storedFit.RuleEdition().String(),
		FitPosture:               "compatible",
		TransitionProfilesRef:    currentProfiles.Ref().String(),
		TransitionProfilesDigest: currentProfiles.Digest().String(),
		TransitionProfileSet:     currentProfiles.ProjectionProfilesDigest().String(),
		TransitionProfileCount:   len(profiles.Profiles()),
		TransitionPosturesDigest: posturesDigest,
	}
	profileIdentity := profileIdentity{
		LedgerRevision:   int64(currentProfile.LedgerRevision().Value()),
		BasisRef:         currentProfile.ProfileBasisRef().String(),
		BasisDigest:      currentProfile.Digest().String(),
		LedgerDigest:     currentProfile.ProfileLedgerDigest().String(),
		SupportDAGDigest: currentProfile.SupportDAGDigest().String(),
	}
	if err := validateStageProfileBinding(profileIdentity, identity); err != nil {
		return stageProfileIdentity{}, err
	}
	return identity, nil
}

func verifySelectionReadyTransitionClosure(
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	ready projecttypeenvstage.SelectionReadyStage,
	predecessor predecessorIdentity,
	profile stageProfileIdentity,
) (string, error) {
	if err := closure.Verify(); err != nil {
		return "", fmt.Errorf("verify selection closure before Stage correlation: %w", err)
	}
	if err := ready.Verify(); err != nil {
		return "", fmt.Errorf("verify selection-ready Stage closure: %w", err)
	}
	prior, transition := closure.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !transition {
		return "", fmt.Errorf("selection-ready closure requires Transition predecessor")
	}
	stage := ready.Stage()
	stagePrior, transition := stage.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !transition ||
		stagePrior.Project() != prior.Project() ||
		stagePrior.Head() != prior.Head() ||
		stagePrior.HeadRevision() != prior.HeadRevision() ||
		stagePrior.SelectedComposite() != prior.SelectedComposite() {
		return "", fmt.Errorf("selection-ready Stage differs from exact Transition predecessor")
	}
	target := closure.Target()
	if stage.Project() != closure.Project() ||
		stage.Ref() != target.Stage() ||
		stage.Base() != target.Base() ||
		!slices.Equal(stage.OrderedExtensions(), target.OrderedExtensions()) ||
		stage.RuntimeBasis() != target.RuntimeBasis() ||
		stage.VerifiedComposite() != target.Composite() ||
		stage.GraphRevision() != closure.ExpectedGraphRevision() {
		return "", fmt.Errorf("selection-ready Stage differs from exact request target")
	}
	verification := ready.VerificationRecord()
	if verification.Ref() != stage.CompositeVerificationRef() ||
		verification.Digest() != stage.CompositeVerificationDigest() ||
		verification.BaseTypeEnvRef() != target.Base() ||
		!slices.Equal(verification.ExtensionRefs(), target.OrderedExtensions()) ||
		verification.RuntimeEvaluationBasisRef() != target.RuntimeBasis() ||
		verification.CompositeRef() != target.Composite() ||
		verification.LoweredEnvironmentRef() != target.Composite() {
		return "", fmt.Errorf("selection-ready final-lowerer record differs from request B/E/X/C")
	}
	snapshot := ready.ExecutableSnapshot()
	snapshotRecord := snapshot.Record()
	if snapshot.TypeEnvRef() != target.Composite() ||
		ready.ExecutableTypeEnv().Ref() != target.Composite() ||
		snapshotRecord.TypeEnvRef() != target.Composite() ||
		snapshotRecord.VerificationRef() != verification.Ref() ||
		snapshotRecord.BaseTypeEnvRef() != target.Base() ||
		!slices.Equal(snapshotRecord.ExtensionRefs(), target.OrderedExtensions()) ||
		snapshotRecord.RuntimeEvaluationBasisRef() != target.RuntimeBasis() {
		return "", fmt.Errorf("selection-ready executable snapshot differs from request B/E/X/C")
	}
	coordinates := struct {
		Schema                        string               `json:"schema"`
		SelectionClosureDigest        string               `json:"selection_closure_digest"`
		StageRef                      string               `json:"stage_ref"`
		StageCanonicalDigest          string               `json:"stage_canonical_digest"`
		VerificationRef               string               `json:"verification_ref"`
		VerificationDigest            string               `json:"verification_digest"`
		VerificationCanonicalDigest   string               `json:"verification_canonical_digest"`
		ExecutableTypeEnvRef          string               `json:"executable_type_env_ref"`
		ExecutableSnapshotDigest      string               `json:"executable_snapshot_digest"`
		ExecutableSnapshotBytesDigest string               `json:"executable_snapshot_bytes_digest"`
		LoweredEnvironmentDigest      string               `json:"lowered_environment_digest"`
		Predecessor                   predecessorIdentity  `json:"predecessor"`
		Profile                       stageProfileIdentity `json:"profile"`
	}{
		Schema:                        "haft.p13.selection-ready-closure/v1",
		SelectionClosureDigest:        closure.Digest().String(),
		StageRef:                      stage.Ref().String(),
		StageCanonicalDigest:          sha256Prefixed(stage.CanonicalBytes()),
		VerificationRef:               verification.Ref().String(),
		VerificationDigest:            verification.Digest().String(),
		VerificationCanonicalDigest:   sha256Prefixed(verification.CanonicalBytes()),
		ExecutableTypeEnvRef:          snapshot.TypeEnvRef().String(),
		ExecutableSnapshotDigest:      snapshot.Digest().String(),
		ExecutableSnapshotBytesDigest: sha256Prefixed(snapshotRecord.CanonicalBytes()),
		LoweredEnvironmentDigest:      snapshot.LoweredEnvironmentDigest().String(),
		Predecessor:                   predecessor,
		Profile:                       profile,
	}
	digest, err := digestCanonicalJSON(coordinates)
	if err != nil {
		return "", fmt.Errorf("digest verified selection-ready closure: %w", err)
	}
	return digest, nil
}

func typeEnvIdentityFromVerifiedTransitionClosure(
	closure projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureV1,
	readyClosureDigest string,
	predecessor predecessorIdentity,
	profile stageProfileIdentity,
) (typeEnvIdentity, error) {
	if err := closure.Verify(); err != nil {
		return typeEnvIdentity{}, fmt.Errorf("verify TypeEnv selection closure: %w", err)
	}
	if readyClosureDigest == "" {
		return typeEnvIdentity{}, fmt.Errorf("verified selection-ready closure digest is required")
	}
	prior, ok := closure.Predecessor().(projecttypeenvselection.TransitionStagePredecessor)
	if !ok {
		return typeEnvIdentity{}, fmt.Errorf(
			"P13 requires an exact post-P12E Transition closure; Genesis is not sufficient",
		)
	}
	priorState, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           prior.Project(),
			SelectedComposite: prior.SelectedComposite(),
			Revision:          prior.HeadRevision(),
		},
	)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf("reconstruct exact Transition prior head: %w", err)
	}
	if priorState.Ref() != prior.Head() {
		return typeEnvIdentity{}, fmt.Errorf("reconstructed Transition prior head ref differs")
	}
	target := closure.Target()
	extensions := target.OrderedExtensions()
	extensionRefs := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		extensionRefs = append(extensionRefs, extension.String())
	}
	extensionsDigest, err := digestCanonicalJSON(extensionRefs)
	if err != nil {
		return typeEnvIdentity{}, fmt.Errorf("digest ordered target extensions: %w", err)
	}
	successor := closure.SuccessorHead()
	return typeEnvIdentity{
		HeadRef:                       successor.Ref().String(),
		HeadRevision:                  int64(successor.Revision().Value()),
		SelectedCompositeRef:          successor.SelectedComposite().String(),
		StateDigest:                   closure.SuccessorHeadDigest().String(),
		SelectionClosureRef:           closure.Ref().String(),
		SelectionClosureDigest:        closure.Digest().String(),
		SelectionRequestSchema:        transitionSelectionRequestSchema,
		SelectionRequestRef:           closure.RequestRef().String(),
		SelectionRequestDigest:        closure.RequestDigest().String(),
		SelectionStageRef:             target.Stage().String(),
		SelectionStageDigest:          target.Stage().Digest().String(),
		SelectionReadyClosureDigest:   readyClosureDigest,
		SelectionPredecessorKind:      "transition",
		PriorHeadRef:                  prior.Head().String(),
		PriorHeadRevision:             int64(prior.HeadRevision().Value()),
		PriorSelectedCompositeRef:     prior.SelectedComposite().String(),
		PriorHeadStateDigest:          sha256Prefixed(priorState.CanonicalBytes()),
		PriorCompositeDigest:          predecessor.CompositeDigest,
		PriorBaseRef:                  predecessor.BaseTypeEnvRef,
		PriorBaseDigest:               predecessor.BaseTypeEnvDigest,
		PriorFPFRevision:              predecessor.FPFRevision,
		PriorCompilerSchema:           predecessor.CompilerSchema,
		PriorExecutableDigest:         predecessor.ExecutableSnapshotDigest,
		PriorLoweredDigest:            predecessor.LoweredEnvironmentDigest,
		TargetBaseRef:                 target.Base().String(),
		TargetOrderedExtensionsDigest: extensionsDigest,
		TargetRuntimeBasisRef:         target.RuntimeBasis().String(),
		TargetCompositeRef:            target.Composite().String(),
		SelectionReceiptRef:           closure.ReceiptRef().String(),
		SelectionReceiptDigest:        closure.ReceiptDigest().String(),
		SelectionAuthorityUseRef:      closure.AuthorityUseRecordRef().String(),
		SelectionAuthorityUseDigest:   closure.AuthorityUseRecordDigest().String(),
		SelectionGraphRevision:        int64(closure.CommittedGraphRevision().Value()),
		SelectionGraphEventRef:        closure.EventRef().String(),
		SelectionGraphCommitRef:       closure.CommitRef().String(),
		StageSchemaEdition:            profile.SchemaEdition,
		StageProfileLedgerRevision:    profile.LedgerRevision,
		StageProfileLedgerDigest:      profile.LedgerDigest,
		StageProfileFitRef:            profile.FitRef,
		StageProfileFitDigest:         profile.FitDigest,
		StageProfileFitRuleEdition:    profile.FitRuleEdition,
		StageProfileFitPosture:        profile.FitPosture,
		TransitionProfilesRef:         profile.TransitionProfilesRef,
		TransitionProfilesDigest:      profile.TransitionProfilesDigest,
		TransitionProfileSetDigest:    profile.TransitionProfileSet,
		TransitionProfileCount:        profile.TransitionProfileCount,
		TransitionPosturesDigest:      profile.TransitionPosturesDigest,
	}, nil
}

func validateCapturedFrozenBasis(
	spec freezeInputSpec,
	identity acceptanceIdentity,
) error {
	want := freezeInputFromIdentity(identity)
	if spec != want {
		return fmt.Errorf("captured P13 head, receipt, or graph differs from the frozen execution input")
	}
	return nil
}

func freezeInputFromIdentity(identity acceptanceIdentity) freezeInputSpec {
	return freezeInputSpec{
		Posture:                     "selected_and_frozen",
		ProfileGeneration:           identity.Profile.Generation,
		ProfileLedgerRevision:       identity.Profile.LedgerRevision,
		ProfilePayloadDigest:        identity.Profile.PayloadDigest,
		ProfileAdmissionRef:         identity.Profile.AdmissionRef,
		ProfileAdmissionDigest:      identity.Profile.AdmissionDigest,
		ProfileProjectionDigest:     identity.Profile.ProjectionDigest,
		ProfileProjectionSchema:     identity.Profile.ProjectionSchema,
		ProfileProjectionLedgerHead: identity.Profile.ProjectionLedgerHead,
		ProfileBasisRef:             identity.Profile.BasisRef,
		ProfileBasisDigest:          identity.Profile.BasisDigest,
		ProfileLedgerDigest:         identity.Profile.LedgerDigest,
		ProfileSupportDAGDigest:     identity.Profile.SupportDAGDigest,
		HeadRef:                     identity.TypeEnv.HeadRef,
		HeadRevision:                identity.TypeEnv.HeadRevision,
		SelectedCompositeRef:        identity.TypeEnv.SelectedCompositeRef,
		HeadStateDigest:             identity.TypeEnv.StateDigest,
		SelectionClosureRef:         identity.TypeEnv.SelectionClosureRef,
		SelectionClosureDigest:      identity.TypeEnv.SelectionClosureDigest,
		SelectionRequestSchema:      identity.TypeEnv.SelectionRequestSchema,
		SelectionRequestRef:         identity.TypeEnv.SelectionRequestRef,
		SelectionRequestDigest:      identity.TypeEnv.SelectionRequestDigest,
		SelectionStageRef:           identity.TypeEnv.SelectionStageRef,
		SelectionStageDigest:        identity.TypeEnv.SelectionStageDigest,
		SelectionReadyClosureDigest: identity.TypeEnv.SelectionReadyClosureDigest,
		SelectionPredecessorKind:    identity.TypeEnv.SelectionPredecessorKind,
		PriorHeadRef:                identity.TypeEnv.PriorHeadRef,
		PriorHeadRevision:           identity.TypeEnv.PriorHeadRevision,
		PriorSelectedCompositeRef:   identity.TypeEnv.PriorSelectedCompositeRef,
		PriorHeadStateDigest:        identity.TypeEnv.PriorHeadStateDigest,
		PriorCompositeDigest:        identity.TypeEnv.PriorCompositeDigest,
		PriorBaseRef:                identity.TypeEnv.PriorBaseRef,
		PriorBaseDigest:             identity.TypeEnv.PriorBaseDigest,
		PriorFPFRevision:            identity.TypeEnv.PriorFPFRevision,
		PriorCompilerSchema:         identity.TypeEnv.PriorCompilerSchema,
		PriorExecutableDigest:       identity.TypeEnv.PriorExecutableDigest,
		PriorLoweredDigest:          identity.TypeEnv.PriorLoweredDigest,
		TargetBaseRef:               identity.TypeEnv.TargetBaseRef,
		TargetExtensionsDigest:      identity.TypeEnv.TargetOrderedExtensionsDigest,
		TargetRuntimeBasisRef:       identity.TypeEnv.TargetRuntimeBasisRef,
		TargetCompositeRef:          identity.TypeEnv.TargetCompositeRef,
		SelectionReceiptRef:         identity.TypeEnv.SelectionReceiptRef,
		SelectionReceiptDigest:      identity.TypeEnv.SelectionReceiptDigest,
		SelectionAuthorityUseRef:    identity.TypeEnv.SelectionAuthorityUseRef,
		SelectionAuthorityUseDigest: identity.TypeEnv.SelectionAuthorityUseDigest,
		SelectionGraphRevision:      identity.TypeEnv.SelectionGraphRevision,
		SelectionGraphEventRef:      identity.TypeEnv.SelectionGraphEventRef,
		SelectionGraphCommitRef:     identity.TypeEnv.SelectionGraphCommitRef,
		StageSchemaEdition:          identity.TypeEnv.StageSchemaEdition,
		StageProfileLedgerRevision:  identity.TypeEnv.StageProfileLedgerRevision,
		StageProfileLedgerDigest:    identity.TypeEnv.StageProfileLedgerDigest,
		StageProfileFitRef:          identity.TypeEnv.StageProfileFitRef,
		StageProfileFitDigest:       identity.TypeEnv.StageProfileFitDigest,
		StageProfileFitRuleEdition:  identity.TypeEnv.StageProfileFitRuleEdition,
		StageProfileFitPosture:      identity.TypeEnv.StageProfileFitPosture,
		TransitionProfilesRef:       identity.TypeEnv.TransitionProfilesRef,
		TransitionProfilesDigest:    identity.TypeEnv.TransitionProfilesDigest,
		TransitionProfileSetDigest:  identity.TypeEnv.TransitionProfileSetDigest,
		TransitionProfileCount:      identity.TypeEnv.TransitionProfileCount,
		TransitionPosturesDigest:    identity.TypeEnv.TransitionPosturesDigest,
		GraphRevision:               identity.Graph.Revision,
		GraphActiveTypeEnvRef:       identity.Graph.ActiveTypeEnvRef,
		GraphSnapshotBasisRef:       identity.Graph.SnapshotBasisRef,
		GraphSnapshotBasisDigest:    identity.Graph.SnapshotBasisDigest,
		GraphLastEventRef:           identity.Graph.LastEventRef,
		GraphLastCommitRef:          identity.Graph.LastCommitRef,
		GraphMaterializationDigest:  identity.Graph.MaterializationDigest,
	}
}

func captureGraphIdentity(
	observation typedmemorystore.CurrentProjectGraphObservation,
	typeEnvironment typeEnvIdentity,
) (graphIdentity, error) {
	if err := observation.Verify(); err != nil {
		return graphIdentity{}, fmt.Errorf("verify typed-memory graph observation: %w", err)
	}
	basis := observation.GraphSnapshotBasis()
	committed, ok := basis.Closure().(projecttypeenvselection.CommittedProjectGraphClosure)
	if !ok || basis.GraphRevision().Value() == 0 {
		return graphIdentity{}, fmt.Errorf("typed-memory graph head is not committed")
	}
	if observation.ActiveTypeEnv().String() != typeEnvironment.SelectedCompositeRef {
		return graphIdentity{}, fmt.Errorf(
			"graph active TypeEnv %q does not match selected composite %q",
			observation.ActiveTypeEnv().String(),
			typeEnvironment.SelectedCompositeRef,
		)
	}
	return graphIdentity{
		Revision:              int64(basis.GraphRevision().Value()),
		ActiveTypeEnvRef:      observation.ActiveTypeEnv().String(),
		SnapshotBasisRef:      basis.Ref().String(),
		SnapshotBasisDigest:   basis.Ref().Digest().String(),
		LastEventRef:          committed.Event().String(),
		LastCommitRef:         committed.Commit().String(),
		MaterializationDigest: committed.MaterializationDigest().String(),
	}, nil
}

func captureSchemaIdentity(
	database identityReader,
	requiredVersion int,
	requiredWriterGeneration int,
) (schemaIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := database.QueryContext(
		ctx,
		"SELECT version FROM schema_version ORDER BY version",
	)
	if err != nil {
		return schemaIdentity{}, fmt.Errorf("read schema versions: %w", err)
	}
	defer rows.Close()
	versions := make([]int, 0)
	for rows.Next() {
		version := 0
		if err := rows.Scan(&version); err != nil {
			return schemaIdentity{}, fmt.Errorf("scan schema version: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return schemaIdentity{}, fmt.Errorf("iterate schema versions: %w", err)
	}
	if err := rows.Close(); err != nil {
		return schemaIdentity{}, fmt.Errorf("close schema versions: %w", err)
	}
	if len(versions) == 0 {
		return schemaIdentity{}, fmt.Errorf("schema version ledger is empty")
	}
	maximum := versions[len(versions)-1]
	if maximum != requiredVersion {
		return schemaIdentity{}, fmt.Errorf(
			"schema version = %d, want exactly %d",
			maximum,
			requiredVersion,
		)
	}
	writerGeneration := 0
	writerCapabilityDigest := ""
	writerCapabilityBytes := []byte{}
	err = database.QueryRowContext(
		ctx,
		`SELECT writer_generation, capability_digest, canonical_bytes
			FROM typed_memory_writer_capabilities_v54
			WHERE capability_key = 'typed_memory_kind_classification_writer_generation'`,
	).Scan(
		&writerGeneration,
		&writerCapabilityDigest,
		&writerCapabilityBytes,
	)
	if err != nil {
		return schemaIdentity{}, fmt.Errorf("read writer-54 capability: %w", err)
	}
	wantWriterBytes := fmt.Sprintf(
		"haft.typed-memory.storage.kind-classification-writer-generation=%d",
		requiredWriterGeneration,
	)
	if writerGeneration != requiredWriterGeneration ||
		string(writerCapabilityBytes) != wantWriterBytes ||
		writerCapabilityDigest != sha256Prefixed(writerCapabilityBytes) {
		return schemaIdentity{}, fmt.Errorf(
			"writer generation capability is not the exact writer-%d marker",
			requiredWriterGeneration,
		)
	}
	digest, err := digestCanonicalJSON(versions)
	if err != nil {
		return schemaIdentity{}, fmt.Errorf("digest schema versions: %w", err)
	}
	catalog, err := captureSchemaCatalog(ctx, database)
	if err != nil {
		return schemaIdentity{}, err
	}
	catalogDigest, err := digestCanonicalJSON(catalog)
	if err != nil {
		return schemaIdentity{}, fmt.Errorf("digest SQLite schema catalog: %w", err)
	}
	return schemaIdentity{
		MaximumVersion:         maximum,
		VersionCount:           len(versions),
		VersionsDigest:         digest,
		WriterGeneration:       writerGeneration,
		WriterCapabilityDigest: writerCapabilityDigest,
		CatalogObjectCount:     len(catalog),
		CatalogDigest:          catalogDigest,
	}, nil
}

func captureSchemaCatalog(
	ctx context.Context,
	database identityReader,
) ([]schemaCatalogRow, error) {
	const query = `SELECT type, name, tbl_name, COALESCE(sql, '')
		FROM sqlite_schema
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name, tbl_name, COALESCE(sql, '')`
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read SQLite schema catalog: %w", err)
	}
	defer rows.Close()
	catalog := make([]schemaCatalogRow, 0)
	for rows.Next() {
		row := schemaCatalogRow{}
		if err := rows.Scan(&row.Type, &row.Name, &row.TableName, &row.SQL); err != nil {
			return nil, fmt.Errorf("scan SQLite schema catalog: %w", err)
		}
		catalog = append(catalog, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite schema catalog: %w", err)
	}
	return catalog, nil
}

func digestCanonicalJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return sha256Prefixed(encoded), nil
}

func sha256Prefixed(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}
