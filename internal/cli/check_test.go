package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectledger"
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

func TestBuildCheckReportAddsReadOnlyEvidenceFreshnessInventoryWithoutFindings(
	t *testing.T,
) {
	fixture := newCheckTestProject(t)
	createdAt := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC).
		Format(time.RFC3339)
	rows := []struct {
		id         string
		validUntil any
	}{
		{id: "evid-dated", validUntil: "2099-01-01"},
		{id: "evid-expired", validUntil: "2020-01-01"},
		{id: "evid-legacy-blank", validUntil: nil},
	}
	for _, row := range rows {
		if _, err := fixture.db.Exec(
			`INSERT INTO evidence_items
			 (id, artifact_ref, type, content, verdict, valid_until, created_at)
			 VALUES (?, ?, 'test', 'fixture', 'supports', ?, ?)`,
			row.id,
			"dec-freshness-fixture",
			row.validUntil,
			createdAt,
		); err != nil {
			t.Fatalf("seed evidence freshness row %s: %v", row.id, err)
		}
	}
	before := checkEvidenceFreshnessCarrierSnapshot(t, fixture.db)
	report, err := buildCheckReport(
		context.Background(),
		fixture.store,
		fixture.root,
	)
	if err != nil {
		t.Fatalf("buildCheckReport returned error: %v", err)
	}
	after := checkEvidenceFreshnessCarrierSnapshot(t, fixture.db)
	if before != after {
		t.Fatalf("evidence freshness inventory mutated rows: before=%q after=%q", before, after)
	}
	if report.Summary.TotalFindings != 0 || report.hasFindings() {
		t.Fatalf("freshness inventory changed check findings: %#v", report)
	}
	inventory := report.EvidenceFreshness
	if inventory.Posture != artifact.EvidenceFreshnessInventoryPostureAvailable ||
		inventory.Diagnostic != "" ||
		inventory.TotalItems != 3 ||
		inventory.Dated != 1 ||
		inventory.Expired != 1 ||
		inventory.LegacyBlankUnknown != 1 ||
		inventory.FindingsAdded != 0 ||
		inventory.LegacyBlankUnknownIsCCED1Violation ||
		inventory.ScoringChanged ||
		inventory.AdmissionChanged ||
		inventory.MutationsPerformed {
		t.Fatalf("evidence freshness inventory = %#v", inventory)
	}
}

func TestBuildCheckReportKeepsEvidenceFreshnessFailureDiagnosticOnly(t *testing.T) {
	fixture := newCheckTestProject(t)
	dropCheckEvidenceItemsTable(t, fixture.db)

	report, err := buildCheckReport(
		context.Background(),
		fixture.store,
		fixture.root,
	)
	if err != nil {
		t.Fatalf("buildCheckReport returned error: %v", err)
	}
	if report.Summary.TotalFindings != 0 || report.hasFindings() {
		t.Fatalf("unavailable freshness changed findings: %#v", report)
	}
	inventory := report.EvidenceFreshness
	if inventory.Posture != artifact.EvidenceFreshnessInventoryPostureUnavailable ||
		!strings.Contains(inventory.Diagnostic, "read evidence freshness carriers") ||
		inventory.TotalItems != 0 ||
		inventory.FindingsAdded != 0 ||
		inventory.ScoringChanged ||
		inventory.AdmissionChanged ||
		inventory.MutationsPerformed {
		t.Fatalf("unavailable freshness inventory = %#v", inventory)
	}
}

func TestCollectCheckEvidenceFreshnessPropagatesContextTermination(t *testing.T) {
	tests := []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		want       error
	}{
		{
			name: "cancelled",
			newContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			want: context.Canceled,
		},
		{
			name: "deadline exceeded",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(
					context.Background(),
					time.Unix(0, 0),
				)
			},
			want: context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCheckTestProject(t)
			ctx, cancel := test.newContext()
			defer cancel()

			inventory, err := collectCheckEvidenceFreshness(
				ctx,
				fixture.store,
				time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC),
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if inventory.Posture !=
				artifact.EvidenceFreshnessInventoryPostureUnavailable {
				t.Fatalf("terminated inventory posture = %q", inventory.Posture)
			}
		})
	}
}

func checkEvidenceFreshnessCarrierSnapshot(t *testing.T, database *sql.DB) string {
	t.Helper()
	var snapshot sql.NullString
	if err := database.QueryRow(`
		SELECT group_concat(id || ':' || coalesce(valid_until, '<null>'), '|')
		FROM (SELECT id, valid_until FROM evidence_items ORDER BY id)
	`).Scan(&snapshot); err != nil {
		t.Fatalf("snapshot evidence freshness carriers: %v", err)
	}
	return snapshot.String
}

func dropCheckEvidenceItemsTable(t *testing.T, database *sql.DB) {
	t.Helper()
	if _, err := database.Exec(`DROP TABLE evidence_items`); err != nil {
		t.Fatalf("drop evidence_items: %v", err)
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
	if report.Drifted[0].BaselineKind != artifact.BaselineKindVerifiedStateSnapshot {
		t.Fatalf("baseline_kind = %q, want verified-state snapshot", report.Drifted[0].BaselineKind)
	}
	if report.Drifted[0].BaselineProfile == nil {
		t.Fatal("baseline_profile missing from drift finding")
	}
	if report.Drifted[0].BaselineProfile.AuthorityBoundary != "drift_detection_snapshot_not_spec_approval_or_pre_work_reference" {
		t.Fatalf("baseline_profile = %+v", report.Drifted[0].BaselineProfile)
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

func TestAppendSpecHealthFindingsFromSetReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.health.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "SQL health section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	specificationSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("load SQL-first specification set: %v", err)
	}
	report := appendSpecHealthFindingsFromSet(
		project.SpecCheckReport{},
		specificationSet,
		root,
	)
	for _, finding := range report.Findings {
		if finding.SectionID == "TS.sql.health.001" && finding.Code == "spec_section_needs_baseline" {
			return
		}
	}
	t.Fatalf("spec health should use SQL edition section; findings = %#v", report.Findings)
}

func TestAppendSpecHealthFindingsFromSetReportsSingleDriftedSectionID(t *testing.T) {
	root, haftDir := newBaselineTestProject(t)

	_ = callHandleSpecSection(t, haftDir, map[string]any{
		"action":       "approve",
		"project_root": root,
		"section_id":   baselineTestSectionID,
		"approved_by":  "human",
	})
	mutateCarrierTitle(t, root)

	specificationSet, err := loadProjectSpecificationSetSQLFirst(root)
	if err != nil {
		t.Fatalf("load SQL-first specification set: %v", err)
	}
	report := appendSpecHealthFindingsFromSet(
		project.SpecCheckReport{},
		specificationSet,
		root,
	)

	driftFindings := make([]project.SpecCheckFinding, 0)
	for _, finding := range report.Findings {
		if finding.Code == "spec_section_drifted" {
			driftFindings = append(driftFindings, finding)
		}
	}
	if len(driftFindings) != 1 {
		t.Fatalf("drift findings = %d, want 1; all findings = %#v", len(driftFindings), report.Findings)
	}
	if driftFindings[0].SectionID != baselineTestSectionID {
		t.Fatalf("section_id = %q, want %q", driftFindings[0].SectionID, baselineTestSectionID)
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

func TestRunCheckEvidenceFreshnessUnavailablePrintsPostureAndStaysZero(
	t *testing.T,
) {
	fixture := newCheckTestProject(t)
	dropCheckEvidenceItemsTable(t, fixture.db)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	restoreJSON := stubCheckJSON(t, false)
	defer restoreJSON()
	exitCode := stubCheckExit(t)

	if err := runCheck(cmd, nil); err != nil {
		t.Fatalf("runCheck returned error: %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", *exitCode)
	}
	result := output.String()
	if !strings.Contains(result, "haft check: clean") ||
		!strings.Contains(result, "evidence freshness (diagnostic only): unavailable") ||
		strings.Contains(result, "evidence freshness (diagnostic only): total=0") {
		t.Fatalf("summary output = %q", result)
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
	t.Parallel()

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
	if _, ok := payload["evidence_freshness"]; !ok {
		t.Fatal("missing top-level field \"evidence_freshness\"")
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

func TestWriteCheckJSONEvidenceFreshnessPostureContract(t *testing.T) {
	t.Parallel()
	now := "2026-08-09T12:00:00Z"
	tests := []struct {
		name           string
		inventory      artifact.EvidenceFreshnessInventory
		wantPosture    string
		wantDiagnostic bool
	}{
		{
			name: "available omits diagnostic",
			inventory: artifact.EvidenceFreshnessInventory{
				Posture:   artifact.EvidenceFreshnessInventoryPostureAvailable,
				Authority: artifact.EvidenceFreshnessDiagnosticAuthority,
				CheckedAt: now,
			},
			wantPosture: "available",
		},
		{
			name: "unavailable includes diagnostic",
			inventory: artifact.EvidenceFreshnessInventory{
				Posture:    artifact.EvidenceFreshnessInventoryPostureUnavailable,
				Authority:  artifact.EvidenceFreshnessDiagnosticAuthority,
				CheckedAt:  now,
				Diagnostic: "read evidence freshness carriers: fixture failure",
			},
			wantPosture:    "unavailable",
			wantDiagnostic: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writeCheckJSON(&output, checkReport{
				EvidenceFreshness: test.inventory,
			}); err != nil {
				t.Fatalf("writeCheckJSON returned error: %v", err)
			}

			var payload struct {
				EvidenceFreshness map[string]json.RawMessage `json:"evidence_freshness"`
			}
			if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
				t.Fatalf("decode JSON output: %v", err)
			}
			var posture string
			if err := json.Unmarshal(
				payload.EvidenceFreshness["posture"],
				&posture,
			); err != nil {
				t.Fatalf("decode posture: %v", err)
			}
			if posture != test.wantPosture {
				t.Fatalf("posture = %q, want %q", posture, test.wantPosture)
			}
			_, hasDiagnostic := payload.EvidenceFreshness["diagnostic"]
			if hasDiagnostic != test.wantDiagnostic {
				t.Fatalf(
					"diagnostic presence = %t, want %t",
					hasDiagnostic,
					test.wantDiagnostic,
				)
			}
		})
	}
}

func newCheckTestProject(t *testing.T) checkTestProject {
	t.Helper()

	homeDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve physical test home: %v", err)
	}
	t.Setenv("HOME", homeDir)

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve physical project root: %v", err)
	}
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

	kernelStore, err := kerneldb.NewStore(dbPath)
	if err != nil {
		t.Fatalf("initialize current kernel store: %v", err)
	}
	t.Cleanup(func() { _ = kernelStore.Close() })
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("bind current kernel store: %v", err)
	}
	database := kernelStore.GetRawDB()

	fixture := checkTestProject{
		root:    root,
		haftDir: haftDir,
		store:   artifact.NewStore(database),
		db:      database,
	}

	writeCheckTestSpecCarriers(t, fixture)

	return fixture
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

	if input.ProblemRef == "" && len(input.ProblemRefs) == 0 && input.PortfolioRef == "" && input.ProblemStatement == "" {
		input.ProblemStatement = "The CLI test fixture requires an explicit problem basis without a linked ProblemCard."
	}

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

func mustSetArtifactStatus(t *testing.T, fixture checkTestProject, artifactID string, status artifact.Status) {
	t.Helper()

	ctx := context.Background()
	item, err := fixture.store.Get(ctx, artifactID)
	if err != nil {
		t.Fatalf("load artifact %s: %v", artifactID, err)
	}

	item.Meta.Status = status
	if err := fixture.store.Update(ctx, item); err != nil {
		t.Fatalf("update status for %s: %v", artifactID, err)
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

	software := "## SS.role.001\n\n" +
		"```yaml spec-section\n" +
		"id: SS.role.001\n" +
		"spec: software-system\n" +
		"kind: software.role\n" +
		"title: Test software role\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-12-31\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(specsDir, "software-system.md"), []byte(software), 0o644); err != nil {
		t.Fatal(err)
	}

	termMap := "```yaml term-map\n" +
		"entries:\n" +
		"  - term: TestProject\n" +
		"    category: target\n" +
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
