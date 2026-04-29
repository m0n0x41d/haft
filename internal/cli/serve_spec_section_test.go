package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

const (
	baselineTestSectionID = "TS.environment-change.001"
	baselineTestProjectID = "qnt_baseline_test"
)

func newBaselineTestProject(t *testing.T) (string, string) {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	specsDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configBody := "id: " + baselineTestProjectID + "\nname: baseline-test\n"
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}

	dbDir := filepath.Join(homeDir, ".haft", "projects", baselineTestProjectID)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := db.NewStore(filepath.Join(dbDir, "haft.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store.Close()

	writeBaselineTestSection(t, root, "Initial environment statement")

	termMap := "```yaml term-map\nentries:\n  - term: HarnessableProject\n    domain: target\n    definition: A repository ready for harness engineering.\n```\n"
	if err := os.WriteFile(filepath.Join(specsDir, "term-map.md"), []byte(termMap), 0o644); err != nil {
		t.Fatal(err)
	}

	return root, haftDir
}

func writeBaselineTestSection(t *testing.T, root, title string) {
	t.Helper()

	body := "## " + baselineTestSectionID + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + baselineTestSectionID + "\n" +
		"spec: target-system\n" +
		"kind: target.environment\n" +
		"title: " + title + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2026-12-31\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(root, ".haft", "specs", "target-system.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func overwriteSectionStatus(t *testing.T, root, status string) {
	t.Helper()

	path := filepath.Join(root, ".haft", "specs", "target-system.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "status: active", "status: "+status, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mutateCarrierTitle(t *testing.T, root string) {
	t.Helper()
	writeBaselineTestSection(t, root, "Sharper environment statement")
}

func callHandleSpecSection(t *testing.T, haftDir string, args map[string]any) SpecSectionBaselineResult {
	t.Helper()

	raw, err := handleHaftSpecSection(context.Background(), nil, haftDir, args)
	if err != nil {
		t.Fatalf("handleHaftSpecSection: %v", err)
	}

	var result SpecSectionBaselineResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode baseline result: %v\nraw: %s", err, raw)
	}
	return result
}

func TestHandleHaftSpecSection_NextStepReturnsFirstPhaseOnEmptyProject(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(filepath.Join(haftDir, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	args := map[string]any{
		"action":       "next_step",
		"project_root": root,
	}

	result, err := handleHaftSpecSection(context.Background(), nil, haftDir, args)
	if err != nil {
		t.Fatalf("handleHaftSpecSection returned error: %v", err)
	}

	var intent specflow.WorkflowIntent
	if err := json.Unmarshal([]byte(result), &intent); err != nil {
		t.Fatalf("decode intent: %v\nraw: %s", err, result)
	}

	if intent.Terminal {
		t.Fatalf("intent.Terminal = true; want first phase")
	}
	if intent.Phase != specflow.PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, specflow.PhaseTargetEnvironmentDraft)
	}
}

func TestHandleHaftSpecSection_DefaultsToServerBoundProjectRoot(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(filepath.Join(haftDir, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	// project_root not provided — handler should derive from haftDir parent.
	args := map[string]any{
		"action": "next_step",
	}

	result, err := handleHaftSpecSection(context.Background(), nil, haftDir, args)
	if err != nil {
		t.Fatalf("handleHaftSpecSection returned error: %v", err)
	}

	var intent specflow.WorkflowIntent
	if err := json.Unmarshal([]byte(result), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}

	if intent.Phase != specflow.PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, specflow.PhaseTargetEnvironmentDraft)
	}
}

func TestHandleHaftSpecSection_RejectsMissingAction(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{})
	if err == nil {
		t.Fatalf("handleHaftSpecSection should reject missing action")
	}
}

func TestHandleHaftSpecSection_RejectsUnknownAction(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{"action": "vibe-check"})
	if err == nil {
		t.Fatalf("handleHaftSpecSection should reject unknown action")
	}
}

func TestHandleHaftSpecSection_ApproveRecordsBaselineForActiveSection(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)

	result := callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"approved_by":  "human",
	})

	if result.SectionID != baselineTestSectionID {
		t.Fatalf("section_id = %q, want %q", result.SectionID, baselineTestSectionID)
	}
	if result.Hash == "" {
		t.Fatalf("hash should be recorded; got empty result: %#v", result)
	}
	if result.ApprovedBy != "human" {
		t.Fatalf("approved_by = %q, want human", result.ApprovedBy)
	}

	// next_step should now advance past the environment phase.
	intentRaw, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{
		"action":       "next_step",
		"project_root": root,
	})
	if err != nil {
		t.Fatalf("next_step: %v", err)
	}
	var intent specflow.WorkflowIntent
	if err := json.Unmarshal([]byte(intentRaw), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}
	if intent.Phase == specflow.PhaseTargetEnvironmentDraft && intent.Audience == "human" {
		t.Fatalf("environment phase still blocking after approve: %#v", intent)
	}
}

func TestHandleHaftSpecSection_ApproveRefusesDraftSection(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	overwriteSectionStatus(t, root, "draft")

	_, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("approve should refuse a draft section")
	}
}

func TestHandleHaftSpecSection_ApproveRefusesWhenBaselineDiffersAndNoRebaseline(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)

	// First approve to lay a baseline.
	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})

	// Mutate the carrier so the hash diverges.
	mutateCarrierTitle(t, root)

	_, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("approve should refuse when baseline already exists with a different hash")
	}
}

func TestHandleHaftSpecSection_RebaselineRequiresReason(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	_, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{
		"action":       "rebaseline",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})
	if err == nil {
		t.Fatalf("rebaseline should require reason")
	}
}

func TestHandleHaftSpecSection_RebaselineOverwritesAndReportsReason(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})

	mutateCarrierTitle(t, root)

	result := callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "rebaseline",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"reason":       "valid evolution: tightened title",
	})

	if result.Reason == "" {
		t.Fatalf("rebaseline result must echo reason: %#v", result)
	}
	if result.Hash == "" {
		t.Fatalf("rebaseline result must include new hash: %#v", result)
	}
}

func TestHandleHaftSpecSection_ReopenDeletesBaselineAndBlocksNextStep(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)
	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
	})

	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "reopen",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"reason":       "needs review",
	})

	intentRaw, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{
		"action":       "next_step",
		"project_root": root,
	})
	if err != nil {
		t.Fatalf("next_step: %v", err)
	}
	var intent specflow.WorkflowIntent
	if err := json.Unmarshal([]byte(intentRaw), &intent); err != nil {
		t.Fatalf("decode intent: %v", err)
	}
	if intent.Phase != specflow.PhaseTargetEnvironmentDraft || intent.Audience != "human" {
		t.Fatalf("expected environment phase to block after reopen; got %#v", intent)
	}
}
