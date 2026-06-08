package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecStatusSummaryShowsLifecycleAction(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := enterTestProjectRoot(t, root)
	defer restore()

	restoreJSON := stubSpecStatusJSON(t, false)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecStatus(cmd, nil); err != nil {
		t.Fatalf("runSpecStatus returned error: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"Spec status: needs_action",
		"Next action: draft",
		"Carrier:     .haft/specs/target-system.md",
		"Allowed next steps:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestRunSpecNextJSONReturnsLifecycleProjection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := enterTestProjectRoot(t, root)
	defer restore()

	restoreJSON := stubSpecNextJSON(t, true)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecNext(cmd, nil); err != nil {
		t.Fatalf("runSpecNext returned error: %v", err)
	}

	var projection specflow.SpecLifecycleProjection
	if err := json.Unmarshal(output.Bytes(), &projection); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, output.String())
	}
	if projection.Action != specflow.LifecycleActionDraft {
		t.Fatalf("Action = %q, want %q", projection.Action, specflow.LifecycleActionDraft)
	}
	if projection.WorkflowIntent.Phase != specflow.PhaseTargetEnvironmentDraft {
		t.Fatalf("WorkflowIntent.Phase = %q", projection.WorkflowIntent.Phase)
	}
}

func stubSpecStatusJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specStatusJSON
	specStatusJSON = value
	return func() { specStatusJSON = prev }
}

func stubSpecNextJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specNextJSON
	specNextJSON = value
	return func() { specNextJSON = prev }
}
