package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
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

func TestRunSpecOnboardReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.onboard.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "SQL onboard section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_spec_sync_test", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()
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
	if len(intent.BlockingFindings) == 0 {
		t.Fatalf("expected missing-baseline finding for SQL edition: %#v", intent)
	}
	if intent.BlockingFindings[0].SectionID != "TS.sql.onboard.001" {
		t.Fatalf("onboard read carrier section instead of SQL edition: %#v", intent.BlockingFindings)
	}
}

func TestRunSpecOnboardJSONMatchesMCPNextStepKeys(t *testing.T) {
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(filepath.Join(haftDir, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}

	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()
	restoreJSON := stubSpecOnboardJSON(t, true)
	defer restoreJSON()

	var cliOutput bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&cliOutput)
	if err := runSpecOnboard(cmd, nil); err != nil {
		t.Fatalf("runSpecOnboard returned error: %v", err)
	}

	mcpOutput, err := handleHaftSpecSection(context.Background(), nil, haftDir, map[string]any{
		"action":       "next_step",
		"project_root": root,
	})
	if err != nil {
		t.Fatalf("next_step returned error: %v", err)
	}

	cliKeys := jsonObjectKeys(t, cliOutput.Bytes())
	mcpKeys := jsonObjectKeys(t, []byte(mcpOutput))
	if !reflect.DeepEqual(cliKeys, mcpKeys) {
		t.Fatalf("CLI/MCP WorkflowIntent keys differ:\nCLI=%v\nMCP=%v", cliKeys, mcpKeys)
	}
}

func jsonObjectKeys(t *testing.T, raw []byte) []string {
	t.Helper()

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode JSON object: %v\nraw: %s", err, string(raw))
	}

	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stubSpecOnboardJSON(t *testing.T, value bool) func() {
	t.Helper()
	prev := specOnboardJSON
	specOnboardJSON = value
	return func() { specOnboardJSON = prev }
}
