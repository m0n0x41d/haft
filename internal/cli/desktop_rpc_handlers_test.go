package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/spf13/cobra"
)

type testProjectRPCHandler func(*rpcEnv, io.Writer) error

func TestHandleAddProject(t *testing.T) {
	t.Run("rejects normal directory without haft metadata", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		err := runProjectRPCHandlerError(t, handleAddProject, targetPath)
		if err == nil {
			t.Fatal("handleAddProject succeeded for a directory without .haft/")
		}
		if !strings.Contains(err.Error(), "no .haft/ directory") {
			t.Fatalf("handleAddProject error = %q, want missing .haft/", err.Error())
		}

		_, statErr := os.Stat(filepath.Join(targetPath, ".haft"))
		if !os.IsNotExist(statErr) {
			t.Fatalf(".haft/ stat error = %v, want not exists", statErr)
		}
	})

	t.Run("registers existing haft project", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		cfg := createProjectConfig(t, targetPath)

		got := runProjectRPCHandler(t, handleAddProject, targetPath)
		if got.Path != targetPath {
			t.Fatalf("Path = %q, want %q", got.Path, targetPath)
		}
		if got.ID != cfg.ID {
			t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
		}
		if got.Name != cfg.Name {
			t.Fatalf("Name = %q, want %q", got.Name, cfg.Name)
		}
		if got.Status != string(project.ReadinessNeedsOnboard) {
			t.Fatalf("Status = %q, want %q", got.Status, project.ReadinessNeedsOnboard)
		}
		if !got.Exists || !got.HasHaft || got.HasSpecs {
			t.Fatalf("readiness fields = exists:%t has_haft:%t has_specs:%t, want true true false", got.Exists, got.HasHaft, got.HasSpecs)
		}

		requireRegisteredProject(t, targetPath, cfg.ID)
	})
}

func TestHandleAddProjectSmart(t *testing.T) {
	t.Run("initializes and registers normal directory", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		got := runProjectRPCHandler(t, handleAddProjectSmart, targetPath)

		cfg := requireProjectConfig(t, targetPath)
		if got.Path != targetPath {
			t.Fatalf("Path = %q, want %q", got.Path, targetPath)
		}
		if got.ID != cfg.ID {
			t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
		}
		if got.Status != string(project.ReadinessNeedsOnboard) {
			t.Fatalf("Status = %q, want %q", got.Status, project.ReadinessNeedsOnboard)
		}
		if !got.Exists || !got.HasHaft || got.HasSpecs {
			t.Fatalf("readiness fields = exists:%t has_haft:%t has_specs:%t, want true true false", got.Exists, got.HasHaft, got.HasSpecs)
		}

		dbPath, err := cfg.DBPath()
		if err != nil {
			t.Fatalf("DBPath: %v", err)
		}
		if _, err := os.Stat(dbPath); err != nil {
			t.Fatalf("database stat: %v", err)
		}
		requireOnboardingCarriers(t, targetPath)

		requireRegisteredProject(t, targetPath, cfg.ID)
	})

	t.Run("registers existing project without initialization", func(t *testing.T) {
		setRPCProjectHome(t)

		targetPath := t.TempDir()
		cfg := createProjectConfig(t, targetPath)

		got := runProjectRPCHandler(t, handleAddProjectSmart, targetPath)
		if got.Path != targetPath {
			t.Fatalf("Path = %q, want %q", got.Path, targetPath)
		}
		if got.ID != cfg.ID {
			t.Fatalf("ID = %q, want existing ID %q", got.ID, cfg.ID)
		}

		dbPath, err := cfg.DBPath()
		if err != nil {
			t.Fatalf("DBPath: %v", err)
		}
		if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
			t.Fatalf("database stat error = %v, want not exists", err)
		}

		requireRegisteredProject(t, targetPath, cfg.ID)
	})
}

func TestDesktopRPCAddProjectSmart(t *testing.T) {
	setRPCProjectHome(t)

	activePath := createInitializedProject(t)
	t.Setenv("HAFT_PROJECT_ROOT", activePath)

	targetPath := t.TempDir()
	cmd := desktopRPCSubcommand(t, "add-project-smart")
	output := bytes.Buffer{}
	cmd.SetOut(&output)

	restore := setRPCInput(t, map[string]string{"path": targetPath})
	defer restore()

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("add-project-smart command: %v", err)
	}

	got := decodeProjectRPCResult(t, output.Bytes())
	cfg := requireProjectConfig(t, targetPath)
	if got.Path != targetPath {
		t.Fatalf("Path = %q, want %q", got.Path, targetPath)
	}
	if got.ID != cfg.ID {
		t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
	}
}

func TestHandleProjectReadinessUsesCoreSpecCheck(t *testing.T) {
	setRPCProjectHome(t)

	rootPath := t.TempDir()
	haftDir := filepath.Join(rootPath, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), []byte("id: qnt_test\nname: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(haftDir, "workflow.md"), []byte("# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "target-system.md"), []byte(malformedDesktopRPCSpecSection("TS.use.001", "environment-change")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "enabling-system.md"), []byte(desktopRPCSpecSection("ES.creator.001", "creator-role")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "term-map.md"), []byte("```yaml\nterm: HarnessableProject\ndomain: enabling\ndefinition: A project with active specs.\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runReadinessRPCHandler(t, rootPath)
	if got.Status != string(project.ReadinessNeedsOnboard) {
		t.Fatalf("Status = %q, want %q", got.Status, project.ReadinessNeedsOnboard)
	}
	if !got.Exists || !got.HasHaft || got.HasSpecs {
		t.Fatalf("readiness fields = exists:%t has_haft:%t has_specs:%t, want true true false", got.Exists, got.HasHaft, got.HasSpecs)
	}
	if got.ReadinessSource != "core" || got.ReadinessError != "" {
		t.Fatalf("readiness source/error = %q/%q, want core/empty", got.ReadinessSource, got.ReadinessError)
	}
}

func TestHandleProjectReadinessClassifiesMissingTermMapAsNeedsOnboard(t *testing.T) {
	setRPCProjectHome(t)

	rootPath := t.TempDir()
	haftDir := filepath.Join(rootPath, ".haft")
	specDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), []byte("id: qnt_test\nname: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(haftDir, "workflow.md"), []byte("# Workflow\n\n## Defaults\n\n```yaml\nmode: standard\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "target-system.md"), []byte(desktopRPCSpecSection("TS.use.001", "environment-change")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "enabling-system.md"), []byte(desktopRPCSpecSection("ES.creator.001", "creator-role")), 0o644); err != nil {
		t.Fatal(err)
	}

	got := runReadinessRPCHandler(t, rootPath)
	if got.Status != string(project.ReadinessNeedsOnboard) {
		t.Fatalf("Status = %q, want %q", got.Status, project.ReadinessNeedsOnboard)
	}
	if !got.Exists || !got.HasHaft || got.HasSpecs {
		t.Fatalf("readiness fields = exists:%t has_haft:%t has_specs:%t, want true true false", got.Exists, got.HasHaft, got.HasSpecs)
	}
}

func TestHandleSpecCheckReturnsCoreFindings(t *testing.T) {
	setRPCProjectHome(t)

	rootPath := t.TempDir()
	specDir := filepath.Join(rootPath, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "target-system.md"), []byte(malformedDesktopRPCSpecSection("TS.use.001", "environment-change")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "enabling-system.md"), []byte(desktopRPCSpecSection("ES.creator.001", "creator-role")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "term-map.md"), []byte("```yaml\nterm: HarnessableProject\ndomain: enabling\ndefinition: A project with active specs.\n```\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HAFT_PROJECT_ROOT", rootPath)

	output := bytes.Buffer{}
	if err := handleSpecCheck(&output); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	var report project.SpecCheckReport
	decodeRPCData(t, output.Bytes(), &report)
	if report.Summary.TotalFindings == 0 {
		t.Fatalf("TotalFindings = 0, want core spec-check findings")
	}
	if len(report.Documents) != 3 {
		t.Fatalf("Documents = %d, want 3", len(report.Documents))
	}
	if !specCheckReportHasCode(report, "spec_section_invalid_yaml") {
		t.Fatalf("findings missing spec_section_invalid_yaml: %#v", report.Findings)
	}
}

func TestHandleInitProjectCreatesOnboardingCarriers(t *testing.T) {
	setRPCProjectHome(t)

	targetPath := t.TempDir()
	got := runProjectRPCHandler(t, handleInitProject, targetPath)
	cfg := requireProjectConfig(t, targetPath)

	if got.ID != cfg.ID {
		t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
	}
	if got.Status != string(project.ReadinessNeedsOnboard) {
		t.Fatalf("Status = %q, want %q", got.Status, project.ReadinessNeedsOnboard)
	}
	requireOnboardingCarriers(t, targetPath)
}

func TestHandleInitProjectDoesNotOverwriteExistingSpecCarriers(t *testing.T) {
	setRPCProjectHome(t)

	targetPath := t.TempDir()
	targetCarrier := filepath.Join(targetPath, ".haft", "specs", "target-system.md")
	customContent := "# Existing Target Spec\n\nHuman-authored target sections.\n"
	if err := os.MkdirAll(filepath.Dir(targetCarrier), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetCarrier, []byte(customContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = runProjectRPCHandler(t, handleInitProject, targetPath)

	data, err := os.ReadFile(targetCarrier)
	if err != nil {
		t.Fatalf("read target carrier: %v", err)
	}
	if string(data) != customContent {
		t.Fatalf("target carrier overwritten:\n%s", string(data))
	}
}

func TestHandleSwitchProjectInitializesMissingHaftProject(t *testing.T) {
	setRPCProjectHome(t)

	targetPath := t.TempDir()
	got := runProjectRPCHandler(t, handleSwitchProject, targetPath)

	cfg := requireProjectConfig(t, targetPath)
	if got.Path != targetPath {
		t.Fatalf("Path = %q, want %q", got.Path, targetPath)
	}
	if got.ID != cfg.ID {
		t.Fatalf("ID = %q, want %q", got.ID, cfg.ID)
	}

	reg, err := rpcLoadRegistry()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if reg.ActivePath != targetPath {
		t.Fatalf("ActivePath = %q, want %q", reg.ActivePath, targetPath)
	}
}

func TestHandleAddProjectSmartActivatesProject(t *testing.T) {
	setRPCProjectHome(t)

	targetPath := t.TempDir()
	got := runProjectRPCHandler(t, handleAddProjectSmart, targetPath)

	reg, err := rpcLoadRegistry()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if reg.ActivePath != got.Path {
		t.Fatalf("ActivePath = %q, want %q", reg.ActivePath, got.Path)
	}
}

func TestHandleAddProjectSmartRepairsNameAndPrunesDuplicateIdentity(t *testing.T) {
	setRPCProjectHome(t)

	stalePath := filepath.Join(t.TempDir(), "old-name")
	targetPath := filepath.Join(t.TempDir(), "new-name")
	if err := os.MkdirAll(filepath.Join(targetPath, ".haft"), 0o755); err != nil {
		t.Fatalf("mkdir target .haft: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(targetPath, ".haft", "project.yaml"),
		[]byte("id: qnt_same\nname: old-name\n"),
		0o644,
	); err != nil {
		t.Fatalf("write stale config: %v", err)
	}

	if err := rpcSaveRegistry(&rpcProjectRegistry{
		Projects: []rpcRegisteredProject{{
			Path: stalePath,
			Name: "old-name",
			ID:   "qnt_same",
		}},
		ActivePath: stalePath,
	}); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	got := runProjectRPCHandler(t, handleAddProjectSmart, targetPath)
	if got.Name != "new-name" {
		t.Fatalf("Name = %q, want repaired name", got.Name)
	}

	cfg := requireProjectConfig(t, targetPath)
	if cfg.Name != "new-name" {
		t.Fatalf("persisted Name = %q, want repaired name", cfg.Name)
	}

	reg, err := rpcLoadRegistry()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if len(reg.Projects) != 1 {
		t.Fatalf("registry projects = %#v, want only canonical project", reg.Projects)
	}
	if reg.Projects[0].Path != targetPath {
		t.Fatalf("registry path = %q, want %q", reg.Projects[0].Path, targetPath)
	}
}

func TestSupportedDesktopAgentSpecsAreV7Hosts(t *testing.T) {
	specs := rpcSupportedDesktopAgentSpecs()
	kinds := make([]string, 0, len(specs))
	for _, spec := range specs {
		kinds = append(kinds, spec.kind)
	}

	if strings.Join(kinds, ",") != "claude,codex" {
		t.Fatalf("desktop agent kinds = %v, want claude,codex", kinds)
	}
}

func TestHandleListCommissionsReturnsOperatorFields(t *testing.T) {
	env, cleanup := createCommissionRPCEnv(t)
	defer cleanup()

	commission := workCommissionFixture("wc-desktop-rpc-stale", "blocked_policy", "2099-01-01T00:00:00Z")
	if _, err := persistWorkCommission(env.ctx, env.store, commission, time.Now().UTC()); err != nil {
		t.Fatalf("persist commission: %v", err)
	}

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"selector": "stale"})
	defer restore()

	if err := handleListCommissions(env, &output); err != nil {
		t.Fatalf("handleListCommissions: %v", err)
	}

	var decoded struct {
		Commissions []map[string]any `json:"commissions"`
	}
	decodeRPCData(t, output.Bytes(), &decoded)

	if len(decoded.Commissions) != 1 {
		t.Fatalf("commissions len = %d, want 1", len(decoded.Commissions))
	}

	operator, ok := decoded.Commissions[0]["operator"].(map[string]any)
	if !ok {
		t.Fatalf("operator missing in %#v", decoded.Commissions[0])
	}
	if operator["attention"] != true {
		t.Fatalf("operator attention = %#v, want true", operator["attention"])
	}
}

func TestHandleCommissionOperatorActions(t *testing.T) {
	env, cleanup := createCommissionRPCEnv(t)
	defer cleanup()

	commission := workCommissionFixture("wc-desktop-rpc-action", "blocked_policy", "2099-01-01T00:00:00Z")
	if _, err := persistWorkCommission(env.ctx, env.store, commission, time.Now().UTC()); err != nil {
		t.Fatalf("persist commission: %v", err)
	}

	requeueOutput := bytes.Buffer{}
	restoreRequeue := setRPCInput(t, map[string]string{
		"commission_id": "wc-desktop-rpc-action",
		"reason":        "test requeue",
	})
	defer restoreRequeue()

	if err := handleRequeueCommission(env, &requeueOutput); err != nil {
		t.Fatalf("handleRequeueCommission: %v", err)
	}

	var requeueDecoded struct {
		Commission map[string]any `json:"commission"`
	}
	decodeRPCData(t, requeueOutput.Bytes(), &requeueDecoded)
	if requeueDecoded.Commission["state"] != "queued" {
		t.Fatalf("requeued state = %#v, want queued", requeueDecoded.Commission["state"])
	}

	cancelOutput := bytes.Buffer{}
	restoreCancel := setRPCInput(t, map[string]string{
		"commission_id": "wc-desktop-rpc-action",
		"reason":        "test cancel",
	})
	defer restoreCancel()

	if err := handleCancelCommission(env, &cancelOutput); err != nil {
		t.Fatalf("handleCancelCommission: %v", err)
	}

	var cancelDecoded struct {
		Commission map[string]any `json:"commission"`
	}
	decodeRPCData(t, cancelOutput.Bytes(), &cancelDecoded)
	if cancelDecoded.Commission["state"] != "cancelled" {
		t.Fatalf("cancelled state = %#v, want cancelled", cancelDecoded.Commission["state"])
	}
}

func TestHandleHarnessResultReturnsStructuredDesktopFacts(t *testing.T) {
	homePath := setRPCProjectHome(t)
	env, cleanup := createCommissionRPCEnv(t)
	defer cleanup()

	commissionID := "wc-desktop-rpc-result"
	trackedPath := filepath.Join("internal", "cli", "serve_commission.go")
	workspacePath := filepath.Join(homePath, ".open-sleigh", "workspaces", commissionID)
	initHarnessApplyRepo(t, workspacePath, trackedPath, "package cli\n\nconst value = \"old\"\n")
	if err := os.WriteFile(filepath.Join(workspacePath, trackedPath), []byte("package cli\n\nconst value = \"new\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	commission := workCommissionFixture(commissionID, "completed", "2099-01-01T00:00:00Z")
	commission["events"] = []any{
		map[string]any{
			"event":       "phase_outcome",
			"verdict":     "pass",
			"recorded_at": "2026-04-24T05:08:35Z",
			"payload": map[string]any{
				"phase": "measure",
				"next":  "terminal:pass",
			},
		},
	}
	if _, err := persistWorkCommission(env.ctx, env.store, commission, time.Now().UTC()); err != nil {
		t.Fatalf("persist commission: %v", err)
	}

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"commission_id": commissionID})
	defer restore()

	if err := handleHarnessResult(env, &output); err != nil {
		t.Fatalf("handleHarnessResult: %v", err)
	}

	var decoded struct {
		CanApply       bool `json:"can_apply"`
		WorkspaceFacts struct {
			Path         string   `json:"path"`
			DiffState    string   `json:"diff_state"`
			ChangedFiles []string `json:"changed_files"`
		} `json:"workspace_facts"`
		EvidenceFacts struct {
			RequiredCount int `json:"required_count"`
			LatestMeasure struct {
				Phase   string `json:"phase"`
				Verdict string `json:"verdict"`
			} `json:"latest_measure"`
		} `json:"evidence_facts"`
		OperatorNext struct {
			Kind string `json:"kind"`
		} `json:"operator_next"`
	}
	decodeRPCData(t, output.Bytes(), &decoded)

	if !decoded.CanApply {
		t.Fatalf("CanApply = false, want true")
	}
	if decoded.WorkspaceFacts.Path != workspacePath {
		t.Fatalf("workspace path = %q, want %q", decoded.WorkspaceFacts.Path, workspacePath)
	}
	if decoded.WorkspaceFacts.DiffState != "changed" {
		t.Fatalf("diff state = %q, want changed", decoded.WorkspaceFacts.DiffState)
	}
	if strings.Join(decoded.WorkspaceFacts.ChangedFiles, ",") != trackedPath {
		t.Fatalf("changed files = %#v, want %s", decoded.WorkspaceFacts.ChangedFiles, trackedPath)
	}
	if decoded.EvidenceFacts.RequiredCount != 1 {
		t.Fatalf("required evidence = %d, want 1", decoded.EvidenceFacts.RequiredCount)
	}
	if decoded.EvidenceFacts.LatestMeasure.Phase != "measure" || decoded.EvidenceFacts.LatestMeasure.Verdict != "pass" {
		t.Fatalf("latest measure = %#v, want measure pass", decoded.EvidenceFacts.LatestMeasure)
	}
	if decoded.OperatorNext.Kind != "apply" {
		t.Fatalf("operator next = %q, want apply", decoded.OperatorNext.Kind)
	}
}

func TestHandleHarnessResultReturnsScopeAuthorizationFactsWhenApplyDisabled(t *testing.T) {
	homePath := setRPCProjectHome(t)
	env, cleanup := createCommissionRPCEnv(t)
	defer cleanup()

	commissionID := "wc-desktop-rpc-result-forbidden"
	trackedPath := filepath.Join("internal", "cli", "serve_commission.go")
	workspacePath := filepath.Join(homePath, ".open-sleigh", "workspaces", commissionID)
	initHarnessApplyRepo(t, workspacePath, trackedPath, "package cli\n\nconst value = \"old\"\n")
	if err := os.WriteFile(filepath.Join(workspacePath, trackedPath), []byte("package cli\n\nconst value = \"new\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	commission := workCommissionFixture(commissionID, "completed", "2099-01-01T00:00:00Z")
	scope := mapField(commission, "scope")
	scope["allowed_paths"] = []any{"**/*"}
	scope["forbidden_paths"] = []any{trackedPath}
	if _, err := persistWorkCommission(env.ctx, env.store, commission, time.Now().UTC()); err != nil {
		t.Fatalf("persist commission: %v", err)
	}

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"commission_id": commissionID})
	defer restore()

	if err := handleHarnessResult(env, &output); err != nil {
		t.Fatalf("handleHarnessResult: %v", err)
	}

	var decoded struct {
		CanApply       bool `json:"can_apply"`
		WorkspaceFacts struct {
			Authorization struct {
				Verdict        string   `json:"verdict"`
				CanApply       bool     `json:"can_apply"`
				ForbiddenPaths []string `json:"forbidden_paths"`
				OperatorReason struct {
					Code    string   `json:"code"`
					Verdict string   `json:"verdict"`
					Paths   []string `json:"paths"`
					Message string   `json:"message"`
				} `json:"operator_reason"`
			} `json:"authorization"`
		} `json:"workspace_facts"`
		OperatorNext struct {
			Kind                string `json:"kind"`
			Reason              string `json:"reason"`
			ApplyDisabledReason struct {
				Code    string   `json:"code"`
				Verdict string   `json:"verdict"`
				Paths   []string `json:"paths"`
				Message string   `json:"message"`
			} `json:"apply_disabled_reason"`
		} `json:"operator_next"`
	}
	decodeRPCData(t, output.Bytes(), &decoded)

	if decoded.CanApply {
		t.Fatal("CanApply = true, want false")
	}
	if decoded.WorkspaceFacts.Authorization.CanApply {
		t.Fatal("workspace authorization can_apply = true, want false")
	}
	if decoded.WorkspaceFacts.Authorization.Verdict != "forbidden" {
		t.Fatalf("authorization verdict = %q, want forbidden", decoded.WorkspaceFacts.Authorization.Verdict)
	}
	if strings.Join(decoded.WorkspaceFacts.Authorization.ForbiddenPaths, ",") != trackedPath {
		t.Fatalf("forbidden paths = %#v, want %s", decoded.WorkspaceFacts.Authorization.ForbiddenPaths, trackedPath)
	}
	if decoded.OperatorNext.Kind != "inspect" {
		t.Fatalf("operator next = %q, want inspect", decoded.OperatorNext.Kind)
	}
	if decoded.OperatorNext.ApplyDisabledReason.Code != "forbidden_paths" {
		t.Fatalf("disabled reason code = %q, want forbidden_paths", decoded.OperatorNext.ApplyDisabledReason.Code)
	}
	if strings.Join(decoded.OperatorNext.ApplyDisabledReason.Paths, ",") != trackedPath {
		t.Fatalf("disabled reason paths = %#v, want %s", decoded.OperatorNext.ApplyDisabledReason.Paths, trackedPath)
	}
	if !strings.Contains(decoded.OperatorNext.Reason, "forbidden by commission scope") {
		t.Fatalf("operator reason = %q, want forbidden scope message", decoded.OperatorNext.Reason)
	}
	if decoded.WorkspaceFacts.Authorization.OperatorReason.Message != decoded.OperatorNext.ApplyDisabledReason.Message {
		t.Fatalf(
			"operator reason message = %q, disabled reason message = %q",
			decoded.WorkspaceFacts.Authorization.OperatorReason.Message,
			decoded.OperatorNext.ApplyDisabledReason.Message,
		)
	}
}

func TestHandleHarnessTailUsesSnakeCaseAndHumanizedEvents(t *testing.T) {
	homePath := setRPCProjectHome(t)
	env, cleanup := createCommissionRPCEnv(t)
	defer cleanup()

	logPath := filepath.Join(homePath, ".open-sleigh", "runtime.jsonl")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHarnessRuntimeEvents(t, logPath, []map[string]any{
		{
			"at":            "2026-04-24T05:00:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-other",
			"data": map[string]any{
				"phase":        "execute",
				"text_preview": "other commission",
			},
		},
		{
			"at":            "2026-04-24T05:01:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-tail-rpc",
			"data": map[string]any{
				"phase":        "execute",
				"status":       "completed",
				"text_preview": "selected commission done",
			},
		},
	})

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]any{
		"commission_id": "wc-tail-rpc",
		"line_count":    20,
	})
	defer restore()

	if err := handleHarnessTail(env, &output); err != nil {
		t.Fatalf("handleHarnessTail: %v", err)
	}

	var decoded struct {
		CommissionID  string   `json:"commission_id"`
		LineCount     int      `json:"line_count"`
		Lines         []string `json:"lines"`
		HasEvents     bool     `json:"has_events"`
		FollowCommand string   `json:"follow_command"`
	}
	decodeRPCData(t, output.Bytes(), &decoded)

	joined := strings.Join(decoded.Lines, "\n")
	if decoded.CommissionID != "wc-tail-rpc" {
		t.Fatalf("commission id = %q, want wc-tail-rpc", decoded.CommissionID)
	}
	if decoded.LineCount != 20 {
		t.Fatalf("line count = %d, want 20", decoded.LineCount)
	}
	if !decoded.HasEvents {
		t.Fatalf("HasEvents = false, want true")
	}
	if strings.Contains(joined, "other commission") || strings.Contains(joined, `{"`) {
		t.Fatalf("tail output leaked wrong/raw event:\n%s", joined)
	}
	if !strings.Contains(joined, "selected commission done") {
		t.Fatalf("tail output missing selected preview:\n%s", joined)
	}
	if decoded.FollowCommand != "haft harness tail wc-tail-rpc --follow" {
		t.Fatalf("follow command = %q", decoded.FollowCommand)
	}
}

func runProjectRPCHandler(t *testing.T, handler testProjectRPCHandler, path string) rpcProjectInfo {
	t.Helper()

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"path": path})
	defer restore()

	if err := handler(&rpcEnv{}, &output); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	return decodeProjectRPCResult(t, output.Bytes())
}

func runProjectRPCHandlerError(t *testing.T, handler testProjectRPCHandler, path string) error {
	t.Helper()

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"path": path})
	defer restore()

	return handler(&rpcEnv{}, &output)
}

func runReadinessRPCHandler(t *testing.T, path string) rpcProjectReadiness {
	t.Helper()

	output := bytes.Buffer{}
	restore := setRPCInput(t, map[string]string{"path": path})
	defer restore()

	if err := handleProjectReadiness(&output); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	var facts rpcProjectReadiness
	decodeRPCData(t, output.Bytes(), &facts)
	return facts
}

func decodeProjectRPCResult(t *testing.T, data []byte) rpcProjectInfo {
	t.Helper()

	var info rpcProjectInfo
	decodeRPCData(t, data, &info)
	return info
}

func specCheckReportHasCode(report project.SpecCheckReport, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}

	return false
}

func decodeRPCData(t *testing.T, data []byte, target any) {
	t.Helper()

	var result rpcResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode rpc result: %v\n%s", err, string(data))
	}
	if !result.OK {
		t.Fatalf("rpc result error: %s", result.Error)
	}

	if err := json.Unmarshal(result.Data, target); err != nil {
		t.Fatalf("decode rpc data: %v", err)
	}
}

func setRPCInput(t *testing.T, payload any) func() {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rpc input: %v", err)
	}

	inputFile, err := os.CreateTemp(t.TempDir(), "desktop-rpc-input-*.json")
	if err != nil {
		t.Fatalf("create rpc input: %v", err)
	}
	if _, err := inputFile.Write(data); err != nil {
		t.Fatalf("write rpc input: %v", err)
	}
	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek rpc input: %v", err)
	}

	original := os.Stdin
	os.Stdin = inputFile

	return func() {
		os.Stdin = original
		_ = inputFile.Close()
	}
}

func setRPCProjectHome(t *testing.T) string {
	t.Helper()

	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	t.Setenv("USERPROFILE", homePath)

	return homePath
}

func createProjectConfig(t *testing.T, rootPath string) *project.Config {
	t.Helper()

	haftDir := filepath.Join(rootPath, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft/: %v", err)
	}

	cfg, err := project.Create(haftDir, rootPath)
	if err != nil {
		t.Fatalf("create project config: %v", err)
	}

	return cfg
}

func createInitializedProject(t *testing.T) string {
	t.Helper()

	rootPath := t.TempDir()
	cfg := createProjectConfig(t, rootPath)

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	_ = database.Close()

	return rootPath
}

func createCommissionRPCEnv(t *testing.T) (*rpcEnv, func()) {
	t.Helper()

	rootPath := createInitializedProject(t)
	cfg := requireProjectConfig(t, rootPath)
	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	env := &rpcEnv{
		ctx:         context.Background(),
		store:       artifact.NewStore(database.GetRawDB()),
		rawDB:       database.GetRawDB(),
		dbStore:     database,
		projectRoot: rootPath,
		haftDir:     filepath.Join(rootPath, ".haft"),
	}

	return env, env.close
}

func requireProjectConfig(t *testing.T, rootPath string) *project.Config {
	t.Helper()

	cfg, err := project.Load(filepath.Join(rootPath, ".haft"))
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if cfg == nil {
		t.Fatalf("project config missing for %s", rootPath)
	}

	return cfg
}

func requireOnboardingCarriers(t *testing.T, rootPath string) {
	t.Helper()

	workflowPath := filepath.Join(rootPath, ".haft", "workflow.md")
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("workflow carrier stat %s: %v", workflowPath, err)
	}

	for _, carrier := range project.MinimumSpecCarriers() {
		path := filepath.Join(rootPath, ".haft", carrier.RelativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("onboarding carrier read %s: %v", path, err)
		}
		if string(data) != carrier.Content {
			t.Fatalf("onboarding carrier %s content mismatch:\n%s", path, string(data))
		}
	}
}

func requireRegisteredProject(t *testing.T, path string, id string) {
	t.Helper()

	reg, err := rpcLoadRegistry()
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	for _, registered := range reg.Projects {
		if registered.Path == path && registered.ID == id {
			return
		}
	}

	t.Fatalf("registry missing project path=%q id=%q: %#v", path, id, reg.Projects)
}

func desktopRPCSubcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()

	for _, cmd := range desktopRPCCmd.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}

	t.Fatalf("desktop-rpc command %q is not registered", name)
	return nil
}

type rpcProjectReadiness struct {
	Status          string `json:"status"`
	Exists          bool   `json:"exists"`
	HasHaft         bool   `json:"has_haft"`
	HasSpecs        bool   `json:"has_specs"`
	ReadinessSource string `json:"readiness_source"`
	ReadinessError  string `json:"readiness_error"`
}

func desktopRPCSpecSection(id string, kind string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"```\n"
}

func malformedDesktopRPCSpecSection(id string, kind string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"terms: [\n" +
		"```\n"
}
