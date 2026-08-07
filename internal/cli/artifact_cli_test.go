package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/typedmemory"
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
		SelectionPolicy: "Preserve kernel validation and host-routed operator provenance, then reduce session context.",
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
		Invariants:    []string{"Only a direct unambiguous operator request reaches the effect sink."},
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
	decisionRecord := loadArtifactRecordForTest(t, root, decision.ID)
	fields := decisionRecord.UnmarshalDecisionFields()
	if fields.AuthorityProvenance != "host_routed_operator_request" {
		t.Fatalf("decision authority provenance = %q", fields.AuthorityProvenance)
	}
}

func TestArtifactCreateCLIProjectsTypedTaskRecordsAtExactConcern(t *testing.T) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	problem := runArtifactCreateForTest(
		t,
		fixture.root,
		"problem.frame",
		map[string]any{
			"title":            "Authorization ownership is unresolved",
			"signal":           "Refresh-token ownership differs across callers.",
			"scope":            "Authorization service token lifecycle.",
			"acceptance_probe": "Every token transition has one named owner.",
			"entity_ref": map[string]any{
				"ref_kind_id":  "U.EntityRef",
				"reference_id": taskMemoryTestConcern,
			},
			"bounded_context_ref": taskMemoryTestContext,
		},
	)
	assertArtifactCLITaskProjection(
		t,
		problem,
		"Haft.ProblemCardAtConcern",
	)
	first := runArtifactCreateForTest(
		t,
		fixture.root,
		"note.record",
		map[string]any{
			"title":        "Central token ownership",
			"observations": []string{"The authorization service owns refresh tokens."},
			"entity_ref": map[string]any{
				"ref_kind_id":  "U.EntityRef",
				"reference_id": taskMemoryTestConcern,
			},
			"bounded_context_ref": taskMemoryTestContext,
		},
	)
	second := runArtifactCreateForTest(
		t,
		fixture.root,
		"note.record",
		map[string]any{
			"title":        "Caller token ownership",
			"observations": []string{"Caller sessions own refresh tokens."},
			"entity_ref": map[string]any{
				"ref_kind_id":  "U.EntityRef",
				"reference_id": taskMemoryTestConcern,
			},
			"bounded_context_ref": taskMemoryTestContext,
		},
	)
	assertArtifactCLITaskProjection(t, first, "Haft.NoteAtConcern")
	assertArtifactCLITaskProjection(t, second, "Haft.NoteAtConcern")
	portfolioArgs := taskMemorySolutionArgs(
		first.TaskMemoryProjection.RecordReference.ReferenceID,
		second.TaskMemoryProjection.RecordReference.ReferenceID,
	)
	portfolioArgs["problem_ref"] = problem.ID
	portfolio := runArtifactCreateForTest(
		t,
		fixture.root,
		"solution.explore",
		portfolioArgs,
	)
	assertArtifactCLITaskProjection(
		t,
		portfolio,
		"Haft.SolutionPortfolioAtConcern",
	)
	comparison := runArtifactCreateForTest(
		t,
		fixture.root,
		"solution.compare",
		taskMemoryComparisonArgs(portfolio.ID),
	)
	assertArtifactCLITaskProjection(
		t,
		comparison,
		"Haft.PortfolioComparison",
	)
}

func assertArtifactCLITaskProjection(
	t *testing.T,
	result artifactCreateResult,
	relationSignatureID string,
) {
	t.Helper()
	projection := result.TaskMemoryProjection
	if projection == nil ||
		projection.AdapterResult != "valid" ||
		projection.AdmissionResult != "committed" ||
		projection.RelationDeclarationFragmentID != relationSignatureID ||
		projection.RelationDeclarationPosture !=
			typedmemory.RelationDeclarationTypedFragment.String() ||
		projection.RelationSignatureID != relationSignatureID ||
		projection.RecordReference == nil {
		t.Fatalf(
			"artifact CLI typed projection = %#v, want committed %s",
			projection,
			relationSignatureID,
		)
	}
}

func TestArtifactCreateCLIUsesHostRoutedDecisionBinder(t *testing.T) {
	root := newArtifactCLITestProject(t)
	binder := &fakeHostRoutedDecisionBinder{
		result: decisionBindingOutcome{
			DecisionRef: "dec-20260801-host-route-a1b2c3d4",
			Title:       "Host-routed operator request",
			FilePath:    filepath.Join(root, ".haft", "decisions", "host-route.md"),
		},
	}
	stubHostRoutedDecisionBinder(t, binder)

	result := runArtifactCreateForTest(t, root, "decision.decide", artifact.DecideInput{
		ProblemStatement: "The host needs a bounded decision effect sink.",
		SelectedTitle:    "Host-routed operator request",
		WhySelected:      "The host already received the direct operator request.",
	})

	if binder.input.SelectedTitle != "Host-routed operator request" {
		t.Fatalf("host-routed binding input title = %q", binder.input.SelectedTitle)
	}
	if result.ID != "dec-20260801-host-route-a1b2c3d4" {
		t.Fatalf("host-routed decision ID = %q", result.ID)
	}
}

func TestArtifactCreateCLIIgnoresAndPreservesLegacyProjectConfig(t *testing.T) {
	root := newArtifactCLITestProject(t)
	configPath := filepath.Join(root, ".haft", "config.yaml")
	legacyConfig := []byte("operator-owned: true\n# byte-for-byte preservation\n")
	if err := os.WriteFile(configPath, legacyConfig, 0o644); err != nil {
		t.Fatalf("write legacy project config: %v", err)
	}
	binder := &fakeHostRoutedDecisionBinder{
		result: decisionBindingOutcome{
			DecisionRef: "dec-20260801-config-ignored-a1b2c3d4",
			Title:       "Ignore project authority config",
			FilePath:    filepath.Join(root, ".haft", "decisions", "config-ignored.md"),
		},
	}
	stubHostRoutedDecisionBinder(t, binder)

	result := runArtifactCreateForTest(t, root, "decision.decide", artifact.DecideInput{
		ProblemStatement: "Project-local authority switches are retired.",
		SelectedTitle:    "Ignore project authority config",
		WhySelected:      "The host route owns request classification.",
	})

	if result.ID != "dec-20260801-config-ignored-a1b2c3d4" {
		t.Fatalf("decision ID = %q", result.ID)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read preserved legacy project config: %v", err)
	}
	if !bytes.Equal(after, legacyConfig) {
		t.Fatalf("legacy project config changed:\n%s", after)
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

func TestArtifactCreateCLI_RejectsLegacyProjectDBWithoutMigration(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_7fac7fac")
	executeReadOnlyProjectValidationFixtureSQL(
		t,
		fixture.database,
		"DELETE FROM schema_version WHERE version = (SELECT MAX(version) FROM schema_version)",
	)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	aliasParent := canonicalReadOnlyProjectValidationDirectory(t, t.TempDir())
	projectAlias := filepath.Join(aliasParent, "project-alias")
	if err := os.Symlink(fixture.binding.ProjectRoot, projectAlias); err != nil {
		t.Fatal(err)
	}

	restore := enterTestProjectRoot(t, projectAlias)
	defer restore()
	inputBytes, err := json.Marshal(artifact.NoteInput{
		Title:        "Legacy artifact create",
		Observations: []string{"Artifact CLI must not migrate the project store."},
		Context:      "artifact-migration-test",
		TaskContext:  "artifact-migration-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(
		fixture.binding.ProjectRoot,
		"legacy-note.json",
	)
	if err := os.WriteFile(inputPath, inputBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	previousInputFile := artifactCreateInputFile
	previousJSON := artifactCreateJSON
	artifactCreateInputFile = inputPath
	artifactCreateJSON = true
	t.Cleanup(func() {
		artifactCreateInputFile = previousInputFile
		artifactCreateJSON = previousJSON
	})

	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	err = runArtifactCreate(command, []string{"note.record"})
	if err == nil {
		t.Fatal("artifact create accepted a legacy kernel schema")
	}
	for _, fragment := range []string{
		"kernel schema is not current",
		"haft project migrate",
		"no migration was attempted",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf(
				"artifact create error %q does not contain %q",
				err,
				fragment,
			)
		}
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatal("failed artifact create changed the legacy SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatal("failed artifact create changed legacy project-ledger files")
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
	oldInitZed := initZed
	oldInitAgy := initAgy
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
		initZed = oldInitZed
		initAgy = oldInitAgy
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
	initZed = false
	initAgy = false
	initAll = false
	initLocal = true
	initNoFileInstructions = true

	runTypedCoreInitForTest(t)

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

func countDecisionRecordsForTest(t *testing.T, projectRoot string) int {
	t.Helper()
	cfg, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil {
		t.Fatalf("load project identity: %v", err)
	}
	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("resolve project database: %v", err)
	}
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open project database: %v", err)
	}
	defer database.Close()

	row := database.QueryRow(
		`SELECT COUNT(*) FROM artifacts WHERE kind = ?`,
		string(artifact.KindDecisionRecord),
	)
	count := 0
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count DecisionRecords: %v", err)
	}
	return count
}

func loadArtifactRecordForTest(
	t *testing.T,
	projectRoot string,
	artifactID string,
) *artifact.Artifact {
	t.Helper()
	cfg, err := project.Load(filepath.Join(projectRoot, ".haft"))
	if err != nil {
		t.Fatalf("load project identity: %v", err)
	}
	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("resolve project database: %v", err)
	}
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open project database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	record, err := artifact.NewStore(database).Get(context.Background(), artifactID)
	if err != nil {
		t.Fatalf("load artifact %s: %v", artifactID, err)
	}
	return record
}
