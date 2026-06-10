package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestArtifactCreateCLI_CreatesProblemPortfolioAndDecision(t *testing.T) {
	root := newArtifactCLITestProject(t)

	problem := runArtifactCreateForTest(t, root, "problem.frame", artifact.ProblemFrameInput{
		Title:       "CLI artifact creation",
		Signal:      "Agents need an input-file CLI path for artifact writes.",
		Acceptance:  "A decision can be created through haft artifact create.",
		Context:     "cli-artifact-test",
		TaskContext: "cli-artifact-test",
	})

	portfolio := runArtifactCreateForTest(t, root, "solution.explore", artifact.ExploreInput{
		ProblemRef:  problem.ID,
		Context:     "cli-artifact-test",
		TaskContext: "cli-artifact-test",
		Variants: []artifact.Variant{
			{
				ID:                 "V1",
				Title:              "Input-file artifact CLI",
				Description:        "Create artifacts from JSON files through the CLI.",
				WeakestLink:        "Shell/file workflow must remain discoverable.",
				NoveltyMarker:      "Moves large payloads out of MCP call arguments.",
				SteppingStone:      true,
				SteppingStoneBasis: "Opens later MCP resource-linked input generation.",
			},
			{
				ID:            "V2",
				Title:         "MCP-only writes",
				Description:   "Keep all artifact writes in MCP tools.",
				WeakestLink:   "Large arguments stay in session history.",
				NoveltyMarker: "No CLI input file.",
			},
		},
	})

	decision := runArtifactCreateForTest(t, root, "decision.decide", artifact.DecideInput{
		ProblemRef:      problem.ID,
		PortfolioRef:    portfolio.ID,
		SelectedTitle:   "Input-file artifact CLI",
		WhySelected:     "It moves large write payloads out of MCP tool-call arguments while using the same artifact core.",
		SelectionPolicy: "Preserve kernel validation and manual binding, then reduce session context.",
		WeakestLink:     "Agents must retrieve the compact interface before writing the input file.",
		CounterArgument: "MCP-only writes are simpler and already typed.",
		WhyNotOthers: []artifact.RejectionReason{
			{Variant: "MCP-only writes", Reason: "Leaves large creation payloads in the agent transcript."},
		},
		Rollback: &artifact.RollbackSpec{
			Triggers:    []string{"CLI create rejects valid MCP-shaped inputs."},
			Steps:       []string{"Use existing MCP write path."},
			BlastRadius: "CLI artifact creation surface.",
		},
		Predictions: []artifact.PredictionInput{
			{Claim: "CLI artifact create works", Observable: "CLI e2e test", Threshold: "test passes"},
		},
		Invariants:    []string{"Manual binding remains explicit."},
		AffectedFiles: []string{"internal/cli/artifact_cli.go"},
		ValidUntil:    "2026-08-08T00:00:00+04:00",
		Context:       "cli-artifact-test",
		TaskContext:   "cli-artifact-test",
	})

	if decision.Kind != string(artifact.KindDecisionRecord) {
		t.Fatalf("decision kind = %q, want DecisionRecord", decision.Kind)
	}
	if !strings.Contains(decision.File, ".haft/decisions/") {
		t.Fatalf("decision file should be a decision projection path, got %q", decision.File)
	}
}

func TestArtifactCreateCLI_CreatesNote(t *testing.T) {
	root := newArtifactCLITestProject(t)

	note := runArtifactCreateForTest(t, root, "note.record", artifact.NoteInput{
		Title:        "CLI note create works",
		Observations: []string{"The artifact CLI can write Note carriers from an input file."},
		Context:      "cli-artifact-test",
		TaskContext:  "cli-artifact-test",
	})

	if note.Kind != string(artifact.KindNote) {
		t.Fatalf("note kind = %q, want Note", note.Kind)
	}
	if !strings.Contains(note.File, ".haft/notes/") {
		t.Fatalf("note file should be a note projection path, got %q", note.File)
	}
}

func newArtifactCLITestProject(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	oldInitClaude := initClaude
	oldInitCursor := initCursor
	oldInitGemini := initGemini
	oldInitCodex := initCodex
	oldInitAir := initAir
	oldInitOpencode := initOpencode
	oldInitAll := initAll
	oldInitLocal := initLocal
	oldInitNoFileInstructions := initNoFileInstructions
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
		initClaude = oldInitClaude
		initCursor = oldInitCursor
		initGemini = oldInitGemini
		initCodex = oldInitCodex
		initAir = oldInitAir
		initOpencode = oldInitOpencode
		initAll = oldInitAll
		initLocal = oldInitLocal
		initNoFileInstructions = oldInitNoFileInstructions
	})

	root := t.TempDir()
	home := filepath.Join(root, ".home")
	t.Setenv("HOME", home)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	initClaude = true
	initCursor = false
	initGemini = false
	initCodex = false
	initAir = false
	initOpencode = false
	initAll = false
	initLocal = true
	initNoFileInstructions = true

	if err := runInit(&cobra.Command{}, nil); err != nil {
		t.Fatalf("run init: %v", err)
	}

	return root
}

func runArtifactCreateForTest(t *testing.T, root string, capability string, input any) artifactCreateResult {
	t.Helper()

	inputBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	inputPath := filepath.Join(root, capability+".json")
	if err := os.WriteFile(inputPath, inputBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	oldInputFile := artifactCreateInputFile
	oldJSON := artifactCreateJSON
	t.Cleanup(func() {
		artifactCreateInputFile = oldInputFile
		artifactCreateJSON = oldJSON
	})

	artifactCreateInputFile = inputPath
	artifactCreateJSON = true

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runArtifactCreate(cmd, []string{capability}); err != nil {
		t.Fatalf("run artifact create %s: %v", capability, err)
	}

	result := artifactCreateResult{}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode artifact create result: %v\n%s", err, output.String())
	}

	if result.ID == "" {
		t.Fatalf("artifact create result missing ID:\n%s", output.String())
	}

	return result
}
