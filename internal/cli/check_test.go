package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

type checkTestProject struct {
	root    string
	haftDir string
	store   *artifact.Store
	db      *sql.DB
}

type checkSeedData struct {
	staleID string
	driftID string
	gapID   string
}

func TestBuildCheckReport_CleanProject(t *testing.T) {
	fixture := newCheckTestProject(t)

	report, err := buildCheckReport(context.Background(), fixture.store, fixture.root)
	if err != nil {
		t.Fatalf("buildCheckReport returned error: %v", err)
	}

	if report.hasFindings() {
		t.Fatalf("expected clean report, got %+v", report)
	}
	if report.Summary.TotalFindings != 0 {
		t.Fatalf("total_findings = %d, want 0", report.Summary.TotalFindings)
	}
}

func TestBuildCheckReport_FindsGovernanceDebt(t *testing.T) {
	fixture := newCheckTestProject(t)
	seeded := seedGovernanceDebt(t, fixture)

	report, err := buildCheckReport(context.Background(), fixture.store, fixture.root)
	if err != nil {
		t.Fatalf("buildCheckReport returned error: %v", err)
	}

	if got := len(report.Stale); got != 1 {
		t.Fatalf("len(Stale) = %d, want 1", got)
	}
	if got := report.Stale[0].ID; got != seeded.staleID {
		t.Fatalf("stale ID = %q, want %q", got, seeded.staleID)
	}

	if got := len(report.Drifted); got != 1 {
		t.Fatalf("len(Drifted) = %d, want 1", got)
	}
	if got := report.Drifted[0].DecisionID; got != seeded.driftID {
		t.Fatalf("drift decision_id = %q, want %q", got, seeded.driftID)
	}
	if !strings.Contains(report.Drifted[0].Summary, "code drift") {
		t.Fatalf("drift summary = %q, want code drift summary", report.Drifted[0].Summary)
	}

	if got := len(report.Unassessed); got != 1 {
		t.Fatalf("len(Unassessed) = %d, want 1", got)
	}
	if got := report.Unassessed[0].DecisionID; got != seeded.gapID {
		t.Fatalf("unassessed decision_id = %q, want %q", got, seeded.gapID)
	}

	if got := len(report.CoverageGaps); got != 1 {
		t.Fatalf("len(CoverageGaps) = %d, want 1", got)
	}
	if got := report.CoverageGaps[0].DecisionID; got != seeded.gapID {
		t.Fatalf("coverage decision_id = %q, want %q", got, seeded.gapID)
	}

	wantGaps := []string{
		"latency stays below 50ms",
		"throughput stays above 100k events/sec",
	}
	gotGaps := strings.Join(report.CoverageGaps[0].Gaps, ",")
	if got := gotGaps; got != strings.Join(wantGaps, ",") {
		t.Fatalf("coverage gaps = %q, want %q", got, strings.Join(wantGaps, ","))
	}

	if got := report.Summary.TotalFindings; got != 4 {
		t.Fatalf("total_findings = %d, want 4", got)
	}
}

func TestRunCheck_CleanProjectPrintsSummaryAndStaysZero(t *testing.T) {
	fixture := newCheckTestProject(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubCheckJSON(t, false)
	defer restoreJSON()

	exitCode := stubCheckExit(t)

	err := runCheck(cmd, nil)
	if err != nil {
		t.Fatalf("runCheck returned error: %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", *exitCode)
	}

	result := output.String()
	if !strings.Contains(result, "haft check: clean") {
		t.Fatalf("summary output = %q, want clean heading", result)
	}
}

func TestRunCheck_JSONExitsOneWhenFindingsExist(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedGovernanceDebt(t, fixture)

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubCheckJSON(t, true)
	defer restoreJSON()

	exitCode := stubCheckExit(t)

	err := runCheck(cmd, nil)
	if err != nil {
		t.Fatalf("runCheck returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}

	var report checkReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if got := report.Summary.TotalFindings; got != 4 {
		t.Fatalf("total_findings = %d, want 4", got)
	}
}

func TestWriteCheckJSON_ZeroValueUsesStableSchema(t *testing.T) {
	var output bytes.Buffer

	err := writeCheckJSON(&output, checkReport{})
	if err != nil {
		t.Fatalf("writeCheckJSON returned error: %v", err)
	}

	var payload map[string]json.RawMessage
	err = json.Unmarshal(output.Bytes(), &payload)
	if err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	wantArrays := map[string]string{
		"stale":         "[]",
		"drifted":       "[]",
		"unassessed":    "[]",
		"coverage_gaps": "[]",
	}

	for field, want := range wantArrays {
		got, ok := payload[field]
		if !ok {
			t.Fatalf("missing top-level field %q", field)
		}
		if string(got) != want {
			t.Fatalf("%s = %s, want %s", field, string(got), want)
		}
	}

	gotSummary, ok := payload["summary"]
	if !ok {
		t.Fatalf("missing top-level field %q", "summary")
	}

	var summary checkSummary
	err = json.Unmarshal(gotSummary, &summary)
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.TotalFindings != 0 {
		t.Fatalf("summary.total_findings = %d, want 0", summary.TotalFindings)
	}
}

func newCheckTestProject(t *testing.T) checkTestProject {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft dir: %v", err)
	}

	cfg, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("create project config: %v", err)
	}

	_ = cfg

	dbPath, err := cfg.DBPath()
	if err != nil {
		t.Fatalf("resolve DB path: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	createCheckSchema(t, db)

	// SpecSectionBaseline storage from migration v28 (slice 3). The bespoke
	// fixture schema doesn't run RunMigrations; declare the table inline so
	// writeCheckTestSpecCarriers can baseline its active sections.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS spec_section_baselines (
		project_id TEXT NOT NULL,
		section_id TEXT NOT NULL,
		hash TEXT NOT NULL,
		captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		approved_by TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (project_id, section_id)
	)`); err != nil {
		t.Fatalf("create spec_section_baselines: %v", err)
	}

	fixture := checkTestProject{
		root:    root,
		haftDir: haftDir,
		store:   artifact.NewStore(db),
		db:      db,
	}

	writeCheckTestSpecCarriers(t, fixture)

	return fixture
}

func createCheckSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active',
			context TEXT,
			mode TEXT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			file_path TEXT,
			valid_until TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			search_keywords TEXT DEFAULT '',
			structured_data TEXT DEFAULT ''
		)`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (source_id, target_id, link_type)
		)`,
		`CREATE TABLE evidence_items (
			id TEXT PRIMARY KEY,
			artifact_ref TEXT NOT NULL,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			verdict TEXT,
			carrier_ref TEXT,
			congruence_level INTEGER DEFAULT 3,
			formality_level INTEGER DEFAULT 5,
			claim_refs TEXT DEFAULT '[]',
			claim_scope TEXT DEFAULT '[]',
			valid_until TEXT,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path)
		)`,
		`CREATE TABLE affected_symbols (
			artifact_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			symbol_name TEXT NOT NULL,
			symbol_kind TEXT NOT NULL,
			symbol_line INTEGER,
			symbol_end_line INTEGER,
			symbol_hash TEXT,
			PRIMARY KEY (artifact_id, file_path, symbol_name)
		)`,
		`CREATE VIRTUAL TABLE artifacts_fts USING fts5(id, title, content, kind, search_keywords, tokenize='porter unicode61')`,
		`CREATE TRIGGER artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords)
			VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords)
			VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
		END`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create schema: %v\nSQL: %s", err, statement)
		}
	}
}

func seedGovernanceDebt(t *testing.T, fixture checkTestProject) checkSeedData {
	t.Helper()

	staleValidUntil := time.Now().Add(-72 * time.Hour).Format("2006-01-02")

	staleProblem := mustFrameProblem(t, fixture, artifact.ProblemFrameInput{
		Title:  "Expired problem framing",
		Signal: "Need one stale artifact that does not overlap with evidence freshness logic.",
	})
	mustSetValidUntil(t, fixture, staleProblem.Meta.ID, staleValidUntil)

	driftPath := filepath.Join(fixture.root, "drifted.go")
	if err := os.WriteFile(driftPath, []byte("package main\n\nfunc governed() {}\n"), 0o644); err != nil {
		t.Fatalf("write drift seed file: %v", err)
	}

	driftDecision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Protect drifted file",
		WhySelected:     "Need a baselined decision that will drift after the baseline.",
		SelectionPolicy: "Prefer a single-file drift case for deterministic output.",
		CounterArgument: "The file change may be too small to exercise diff reporting.",
		WeakestLink:     "Baseline and drift detection must both agree on the governed file.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "No drift fixture",
			Reason:  "Would miss the drift category entirely.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Drift findings stop reporting modified files."},
		},
		AffectedFiles: []string{"drifted.go"},
	})
	mustBaselineDecision(t, fixture, driftDecision.Meta.ID)
	mustMeasureDecision(t, fixture, driftDecision.Meta.ID)

	if err := os.WriteFile(driftPath, []byte("package main\n\nfunc governed() {\n\tprintln(\"drift\")\n}\n"), 0o644); err != nil {
		t.Fatalf("write drifted file: %v", err)
	}

	problem := mustFrameProblem(t, fixture, artifact.ProblemFrameInput{
		Title:      "Coverage gap problem",
		Signal:     "Decision evidence has not been attached yet.",
		Acceptance: "- latency stays below 50ms\n- throughput stays above 100k events/sec",
	})

	gapDecision := mustCreateDecision(t, fixture, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Record decision before measurement",
		WhySelected:     "Need one active decision with explicit acceptance coverage gaps.",
		SelectionPolicy: "Prefer the smallest decision that still links to a framed problem.",
		CounterArgument: "An empty evidence chain might be too synthetic.",
		WeakestLink:     "Coverage depends on the acceptance scope being linked through the problem.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Attach measurement immediately",
			Reason:  "Would remove the unassessed and coverage-gap findings.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Coverage gaps are no longer reported for acceptance criteria."},
		},
	})

	return checkSeedData{
		staleID: staleProblem.Meta.ID,
		driftID: driftDecision.Meta.ID,
		gapID:   gapDecision.Meta.ID,
	}
}

func mustFrameProblem(t *testing.T, fixture checkTestProject, input artifact.ProblemFrameInput) *artifact.Artifact {
	t.Helper()

	ctx := context.Background()
	problem, _, err := artifact.FrameProblem(ctx, fixture.store, fixture.haftDir, input)
	if err != nil {
		t.Fatalf("frame problem: %v", err)
	}

	return problem
}

func mustCreateDecision(t *testing.T, fixture checkTestProject, input artifact.DecideInput) *artifact.Artifact {
	t.Helper()

	ctx := context.Background()
	decision, _, err := artifact.Decide(ctx, fixture.store, fixture.haftDir, input)
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}

	return decision
}

func mustMeasureDecision(t *testing.T, fixture checkTestProject, decisionID string) {
	t.Helper()

	ctx := context.Background()
	_, err := artifact.Measure(ctx, fixture.store, fixture.haftDir, artifact.MeasureInput{
		DecisionRef:  decisionID,
		Findings:     "Verification completed successfully.",
		Measurements: []string{"p99 latency: 18ms"},
		Verdict:      "accepted",
	})
	if err != nil {
		t.Fatalf("measure decision %s: %v", decisionID, err)
	}
}

func mustBaselineDecision(t *testing.T, fixture checkTestProject, decisionID string) {
	t.Helper()

	ctx := context.Background()
	_, err := artifact.Baseline(ctx, fixture.store, fixture.root, artifact.BaselineInput{
		DecisionRef: decisionID,
	})
	if err != nil {
		t.Fatalf("baseline decision %s: %v", decisionID, err)
	}
}

func mustSetValidUntil(t *testing.T, fixture checkTestProject, artifactID string, validUntil string) {
	t.Helper()

	ctx := context.Background()
	item, err := fixture.store.Get(ctx, artifactID)
	if err != nil {
		t.Fatalf("load artifact %s: %v", artifactID, err)
	}

	item.Meta.ValidUntil = validUntil
	if err := fixture.store.Update(ctx, item); err != nil {
		t.Fatalf("update valid_until for %s: %v", artifactID, err)
	}
}

func enterTestProjectRoot(t *testing.T, dir string) func() {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	originalProjectRoot, hadProjectRoot := os.LookupEnv("HAFT_PROJECT_ROOT")
	if err := os.Setenv("HAFT_PROJECT_ROOT", dir); err != nil {
		t.Fatalf("set HAFT_PROJECT_ROOT: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}

	return func() {
		if hadProjectRoot {
			if err := os.Setenv("HAFT_PROJECT_ROOT", originalProjectRoot); err != nil {
				t.Fatalf("restore HAFT_PROJECT_ROOT: %v", err)
			}
		} else {
			if err := os.Unsetenv("HAFT_PROJECT_ROOT"); err != nil {
				t.Fatalf("unset HAFT_PROJECT_ROOT: %v", err)
			}
		}
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}
}

func stubCheckJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := checkJSON
	checkJSON = value

	return func() {
		checkJSON = previous
	}
}

func stubCheckExit(t *testing.T) *int {
	t.Helper()

	exitCode := new(int)
	previous := checkExit
	checkExit = func(code int) {
		*exitCode = code
	}
	t.Cleanup(func() {
		checkExit = previous
	})

	return exitCode
}

// writeCheckTestSpecCarriers writes the minimum-viable spec carriers
// (one active target section + one active enabling section + one term)
// so `haft check` is clean by default in tests. Tests that exercise
// spec_health findings explicitly should mutate these carriers.
func writeCheckTestSpecCarriers(t *testing.T, fixture checkTestProject) {
	t.Helper()

	haftDir := fixture.haftDir
	specsDir := filepath.Join(haftDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}

	target := "## TS.environment.001\n\n" +
		"```yaml spec-section\n" +
		"id: TS.environment.001\n" +
		"spec: target-system\n" +
		"kind: environment-change\n" +
		"title: Test environment change\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-12-31\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(specsDir, "target-system.md"), []byte(target), 0o644); err != nil {
		t.Fatal(err)
	}

	enabling := "## ES.creator.001\n\n" +
		"```yaml spec-section\n" +
		"id: ES.creator.001\n" +
		"spec: enabling-system\n" +
		"kind: creator-role\n" +
		"title: Test creator role\n" +
		"statement_type: explanation\n" +
		"claim_layer: carrier\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-12-31\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(specsDir, "enabling-system.md"), []byte(enabling), 0o644); err != nil {
		t.Fatal(err)
	}

	termMap := "```yaml term-map\n" +
		"entries:\n" +
		"  - term: TestProject\n" +
		"    domain: target\n" +
		"    definition: A project under check_test fixture.\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(specsDir, "term-map.md"), []byte(termMap), 0o644); err != nil {
		t.Fatal(err)
	}

	// Active sections need baselines so SpecSection drift detection
	// stays clean. Tests that exercise drift will overwrite carriers
	// after the fixture is built.
	cfg, err := project.Load(haftDir)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}

	store := specflow.NewSQLiteBaselineStore(fixture.db)
	specSet, err := project.LoadProjectSpecificationSet(fixture.root)
	if err != nil {
		t.Fatalf("load spec set: %v", err)
	}
	for _, section := range specSet.Sections {
		if section.Status != string(project.SpecSectionStateActive) {
			continue
		}
		baseline := specflow.SectionBaseline{
			ProjectID:  cfg.ID,
			SectionID:  section.ID,
			Hash:       specflow.HashSection(section),
			ApprovedBy: "check-test-fixture",
		}
		if err := store.Put(baseline); err != nil {
			t.Fatalf("put baseline %s: %v", section.ID, err)
		}
	}
}
