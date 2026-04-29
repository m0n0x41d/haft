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

func TestRunSpecOnboardJSONReturnsFirstPhaseOnEmptyProject(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := enterTestProjectRoot(t, root)
	defer restore()

	restoreJSON := stubSpecOnboardJSON(t, true)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	var intent specflow.WorkflowIntent
	if err := json.Unmarshal(output.Bytes(), &intent); err != nil {
		t.Fatalf("decode JSON: %v\nraw: %s", err, output.String())
	}

	if intent.Terminal {
		t.Fatalf("intent.Terminal = true on empty project; want first phase")
	}
	if intent.Phase != specflow.PhaseTargetEnvironmentDraft {
		t.Fatalf("intent.Phase = %q, want %q", intent.Phase, specflow.PhaseTargetEnvironmentDraft)
	}
	if intent.PromptForUser == "" {
		t.Fatalf("PromptForUser is empty")
	}
	if len(intent.Checks) == 0 {
		t.Fatalf("Checks is empty; want SoTA list")
	}
}

func TestRunSpecOnboardSummaryRendersHumanLines(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".haft", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := enterTestProjectRoot(t, root)
	defer restore()

	restoreJSON := stubSpecOnboardJSON(t, false)
	defer restoreJSON()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	got := output.String()
	for _, want := range []string{
		"Phase:",
		"Audience:",
		"Document:",
		"For the operator:",
		"For the host agent:",
		"Structural checks:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func stubSpecOnboardJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specOnboardJSON
	specOnboardJSON = value
	return func() { specOnboardJSON = prev }
}
