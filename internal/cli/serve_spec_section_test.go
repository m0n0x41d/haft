package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

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

	_, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{"action": "approve"})
	if err == nil {
		t.Fatalf("handleHaftSpecSection should reject unknown action")
	}
}
