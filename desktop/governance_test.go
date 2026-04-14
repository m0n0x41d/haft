package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestResolveGovernanceScanIntervalDefaultsToFiveMinutes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(governanceScanIntervalEnvVar, "")

	interval := resolveGovernanceScanInterval()

	if interval != defaultGovernanceScanInterval {
		t.Fatalf("resolveGovernanceScanInterval = %s, want %s", interval, defaultGovernanceScanInterval)
	}
}

func TestResolveGovernanceScanIntervalUsesConfigMinutes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(governanceScanIntervalEnvVar, "")

	configPath, err := desktopConfigPath()
	if err != nil {
		t.Fatalf("desktopConfigPath: %v", err)
	}

	err = os.WriteFile(configPath, []byte(`{"governance_scan_interval_minutes":1}`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile desktop config: %v", err)
	}

	interval := resolveGovernanceScanInterval()

	if interval != time.Minute {
		t.Fatalf("resolveGovernanceScanInterval = %s, want %s", interval, time.Minute)
	}
}

func TestResolveGovernanceScanIntervalPrefersEnvOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(governanceScanIntervalEnvVar, "45s")

	configPath, err := desktopConfigPath()
	if err != nil {
		t.Fatalf("desktopConfigPath: %v", err)
	}

	err = os.WriteFile(configPath, []byte(`{"governance_scan_interval_minutes":1}`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile desktop config: %v", err)
	}

	interval := resolveGovernanceScanInterval()

	if interval != 45*time.Second {
		t.Fatalf("resolveGovernanceScanInterval = %s, want %s", interval, 45*time.Second)
	}
}

func TestDecisionDetailIncludesGovernanceScope(t *testing.T) {
	app := newGovernanceTestApp(t)
	defer app.shutdown(context.Background())

	decisionID := seedGovernanceDecision(t, app)

	detail, err := app.GetDecision(decisionID)
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}

	if len(detail.AffectedFiles) != 1 || detail.AffectedFiles[0] != "internal/auth/auth.go" {
		t.Fatalf("AffectedFiles = %#v, want internal/auth/auth.go", detail.AffectedFiles)
	}

	if len(detail.CoverageModules) == 0 {
		t.Fatal("expected impacted coverage modules")
	}

	if got := detail.CoverageModules[0].Path; got != "internal/auth" {
		t.Fatalf("CoverageModules[0].Path = %q, want %q", got, "internal/auth")
	}

	if len(detail.Claims) != 1 || detail.Claims[0].VerifyAfter == "" {
		t.Fatalf("Claims = %#v, want one claim with verify_after", detail.Claims)
	}
}

func TestGovernanceOverviewAndProblemCandidateLifecycle(t *testing.T) {
	app := newGovernanceTestApp(t)
	defer app.shutdown(context.Background())

	decisionID := seedGovernanceDecision(t, app)

	overview, err := app.GetGovernanceOverview()
	if err != nil {
		t.Fatalf("GetGovernanceOverview: %v", err)
	}

	if overview.Coverage.TotalModules == 0 {
		t.Fatal("expected module coverage to be populated")
	}

	if len(overview.Findings) == 0 {
		t.Fatal("expected governance findings")
	}

	var pendingCandidateID string
	var expiredCandidateID string

	for _, candidate := range overview.ProblemCandidates {
		switch candidate.Category {
		case "pending_verification":
			pendingCandidateID = candidate.ID
		case "evidence_expired":
			expiredCandidateID = candidate.ID
		}
	}

	if pendingCandidateID == "" {
		t.Fatalf(
			"expected pending verification candidate for %s, findings=%+v candidates=%+v",
			decisionID,
			overview.Findings,
			overview.ProblemCandidates,
		)
	}

	if expiredCandidateID == "" {
		t.Fatalf("expected expired evidence candidate for %s", decisionID)
	}

	problem, err := app.AdoptProblemCandidate(pendingCandidateID)
	if err != nil {
		t.Fatalf("AdoptProblemCandidate: %v", err)
	}

	if !strings.Contains(problem.Title, "Verify due claims") {
		t.Fatalf("problem title = %q, want verify wording", problem.Title)
	}

	if err := app.DismissProblemCandidate(expiredCandidateID); err != nil {
		t.Fatalf("DismissProblemCandidate: %v", err)
	}

	updated, err := app.GetGovernanceOverview()
	if err != nil {
		t.Fatalf("GetGovernanceOverview after updates: %v", err)
	}

	for _, candidate := range updated.ProblemCandidates {
		if candidate.ID == pendingCandidateID || candidate.ID == expiredCandidateID {
			t.Fatalf("candidate %s should not remain active after adopt/dismiss", candidate.ID)
		}
	}
}

func TestGovernanceDecisionRefreshActions(t *testing.T) {
	app := newGovernanceTestApp(t)
	defer app.shutdown(context.Background())

	decisionID := seedGovernanceDecision(t, app)

	waived, err := app.WaiveDecision(decisionID, "Need time to refresh the evidence without reopening yet.")
	if err != nil {
		t.Fatalf("WaiveDecision: %v", err)
	}

	if waived.Status != "active" {
		t.Fatalf("waived decision status = %q, want active", waived.Status)
	}

	validUntil, err := time.Parse(time.RFC3339, waived.ValidUntil)
	if err != nil {
		t.Fatalf("parse waived valid_until %q: %v", waived.ValidUntil, err)
	}

	if !validUntil.After(time.Now().UTC()) {
		t.Fatalf("waived valid_until = %s, want future timestamp", waived.ValidUntil)
	}

	reopened, err := app.ReopenDecision(decisionID, "The stale decision needs a new problem cycle.")
	if err != nil {
		t.Fatalf("ReopenDecision: %v", err)
	}

	if !strings.Contains(reopened.Title, "Revisit:") {
		t.Fatalf("reopened problem title = %q, want revisit wording", reopened.Title)
	}

	decision, err := app.GetDecision(decisionID)
	if err != nil {
		t.Fatalf("GetDecision after reopen: %v", err)
	}

	if decision.Status != "refresh_due" {
		t.Fatalf("decision status after reopen = %q, want refresh_due", decision.Status)
	}
}

func TestGovernanceDecisionBaselineMeasureAndDeprecateActions(t *testing.T) {
	app := newGovernanceTestApp(t)
	defer app.shutdown(context.Background())

	decisionID := seedGovernanceDecision(t, app)

	baselined, err := app.BaselineDecision(decisionID)
	if err != nil {
		t.Fatalf("BaselineDecision: %v", err)
	}

	if len(baselined.AffectedFiles) != 1 || baselined.AffectedFiles[0] != "internal/auth/auth.go" {
		t.Fatalf("baselined affected files = %#v, want internal/auth/auth.go", baselined.AffectedFiles)
	}

	affectedFiles, err := app.store.GetAffectedFiles(context.Background(), decisionID)
	if err != nil {
		t.Fatalf("GetAffectedFiles after baseline: %v", err)
	}
	if len(affectedFiles) != 1 || strings.TrimSpace(affectedFiles[0].Hash) == "" {
		t.Fatalf("affected files after baseline = %#v, want one hashed file", affectedFiles)
	}

	measured, err := app.MeasureDecision(
		decisionID,
		"Measured the desktop verification loop after re-baselining the governed file.",
		"accepted",
	)
	if err != nil {
		t.Fatalf("MeasureDecision: %v", err)
	}

	foundMeasurement := false
	for _, item := range measured.Evidence.Items {
		if item.Type == "measurement" && item.Verdict == "supports" {
			foundMeasurement = true
		}
	}
	if !foundMeasurement {
		t.Fatalf("measurement evidence not found after MeasureDecision: %#v", measured.Evidence.Items)
	}

	deprecated, err := app.DeprecateDecision(
		decisionID,
		"The desktop governance slice is no longer relevant for this seeded decision.",
	)
	if err != nil {
		t.Fatalf("DeprecateDecision: %v", err)
	}

	if deprecated.Status != "deprecated" {
		t.Fatalf("deprecated decision status = %q, want deprecated", deprecated.Status)
	}
	if !strings.Contains(deprecated.Body, "## Deprecated") {
		t.Fatalf("deprecated decision body missing section:\n%s", deprecated.Body)
	}
}

func TestGovernanceTimerRefreshesSnapshotWithoutManualRefresh(t *testing.T) {
	app := newGovernanceTestApp(t)
	defer app.shutdown(context.Background())

	decisionID := seedGovernanceDecision(t, app)

	_, err := app.BaselineDecision(decisionID)
	if err != nil {
		t.Fatalf("BaselineDecision: %v", err)
	}

	overview, err := app.GetGovernanceOverview()
	if err != nil {
		t.Fatalf("GetGovernanceOverview before drift: %v", err)
	}

	if hasGovernanceDriftFinding(overview.Findings, decisionID) {
		t.Fatalf("unexpected drift finding before file change: %+v", overview.Findings)
	}

	app.governance.interval = 20 * time.Millisecond
	app.governance.start(context.Background())

	err = os.WriteFile(
		filepath.Join(app.projectRoot, "internal", "auth", "auth.go"),
		[]byte("package auth\n\nfunc Enabled() bool { return false }\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile auth.go: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		overview, err = app.GetGovernanceOverview()
		if err != nil {
			t.Fatalf("GetGovernanceOverview during timer refresh: %v", err)
		}

		if hasGovernanceDriftFinding(overview.Findings, decisionID) {
			return
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("expected timer refresh to surface drift for %s, findings=%+v", decisionID, overview.Findings)
}

func TestAdoptCreatesDriftTaskWithDecisionContext(t *testing.T) {
	app := newGovernanceTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'adopt drift task\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	decisionID := seedGovernanceDecision(t, app)
	detail, err := app.GetDecision(decisionID)
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}

	_, err = artifact.Baseline(context.Background(), app.store, app.projectRoot, artifact.BaselineInput{
		DecisionRef: decisionID,
	})
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}

	err = os.WriteFile(
		filepath.Join(app.projectRoot, "internal", "auth", "auth.go"),
		[]byte("package auth\n\nfunc Enabled() bool { return false }\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile auth.go: %v", err)
	}

	overview, err := app.GetGovernanceOverview()
	if err != nil {
		t.Fatalf("GetGovernanceOverview: %v", err)
	}

	var findingID string
	for _, finding := range overview.Findings {
		if finding.ArtifactRef != decisionID {
			continue
		}
		if finding.Category != string(artifact.StaleCategoryDecisionStale) {
			continue
		}
		findingID = finding.ID
	}

	if findingID == "" {
		t.Fatalf("expected drift finding for %s, findings=%+v", decisionID, overview.Findings)
	}

	task, err := app.Adopt(findingID)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}

	if task.AutoRun {
		t.Fatal("expected adopt task to start checkpointed")
	}

	expectedSnippets := []string{
		"## Adopt Drift Finding: Desktop governance execution loop",
		"Finding category: decision_stale",
		"Decision ID: " + decisionID,
		"## Decision Record Body",
		detail.Body,
		"## Decision Invariants",
		"Desktop uses shared artifact logic as the single source of truth.",
		"## Drift Report",
		"- internal/auth/auth.go status=modified",
		"## Diffs",
		"internal/auth/auth.go (modified)",
		"func Enabled() bool { return false }",
		"## Impacted Modules",
		"- internal/auth [go] status=",
		"## Instructions",
		"Present options. Do not execute resolution without user confirmation.",
		"Do not execute re-baseline, reopen, waive, or any other lifecycle action without explicit user confirmation.",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(task.Prompt, snippet) {
			t.Fatalf("adopt prompt missing %q:\n%s", snippet, task.Prompt)
		}
	}

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "completed" {
		t.Fatalf("task status = %q, want completed", final.Status)
	}
}

func TestAdoptCreatesStaleTaskWithEvidenceHistory(t *testing.T) {
	categories := []artifact.StaleCategory{
		artifact.StaleCategoryREffDegraded,
		artifact.StaleCategoryEvidenceExpired,
	}

	for _, category := range categories {
		t.Run(string(category), func(t *testing.T) {
			app := newGovernanceTestApp(t)
			defer app.shutdown(context.Background())

			installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'adopt stale task\\n'\n")
			installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")

			decisionID := seedGovernanceDecision(t, app)
			addGovernanceEvidenceHistory(t, app, decisionID)

			if _, err := app.governance.scan(context.Background(), false); err != nil {
				t.Fatalf("governance scan: %v", err)
			}

			detail, err := app.GetDecision(decisionID)
			if err != nil {
				t.Fatalf("GetDecision: %v", err)
			}

			overview, err := app.GetGovernanceOverview()
			if err != nil {
				t.Fatalf("GetGovernanceOverview: %v", err)
			}

			findingID := findGovernanceFindingID(t, overview.Findings, decisionID, category)
			task, err := app.Adopt(findingID)
			if err != nil {
				t.Fatalf("Adopt: %v", err)
			}

			if task.AutoRun {
				t.Fatal("expected stale adopt task to start checkpointed")
			}

			expectedSnippets := []string{
				"## Adopt Stale Finding: Desktop governance execution loop",
				"Finding category: " + string(category),
				"Decision ID: " + decisionID,
				"## Decision Record Body",
				detail.Body,
				"## R_eff Computation",
				"- Decision R_eff: 0.10",
				"weakest-link rule: min(active evidence scores), never average",
				"## Evidence Timeline",
				"ev-expired [measurement] verdict=supports",
				"score=0.10",
				"ev-recent [benchmark] verdict=weakens",
				"score=0.40",
				"## Expired Items",
				"DecisionRecord valid_until expired",
				"ev-expired [measurement] verdict=supports",
				"## Instructions",
				"Present options. Do not execute resolution without user confirmation.",
				"Do not execute measure, waive, deprecate, reopen, or any other lifecycle action without explicit user confirmation.",
			}

			for _, snippet := range expectedSnippets {
				if !strings.Contains(task.Prompt, snippet) {
					t.Fatalf("adopt stale prompt missing %q:\n%s", snippet, task.Prompt)
				}
			}

			final := waitForTaskState(t, app, task.ID)
			if final.Status != "completed" {
				t.Fatalf("task status = %q, want completed", final.Status)
			}
		})
	}
}

func TestAdoptDriftResolutionsCreateRefreshReport(t *testing.T) {
	testCases := []struct {
		name    string
		action  string
		resolve func(t *testing.T, app *App, findingID string)
	}{
		{
			name:   "rebaseline",
			action: "re-baseline",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptBaseline(findingID)
				if err != nil {
					t.Fatalf("ResolveAdoptBaseline: %v", err)
				}
			},
		},
		{
			name:   "waive",
			action: "waive",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptWaive(
					findingID,
					"Need a documented waiver while the drift is investigated further.",
				)
				if err != nil {
					t.Fatalf("ResolveAdoptWaive: %v", err)
				}
			},
		},
		{
			name:   "reopen",
			action: "reopen",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptReopen(
					findingID,
					"The drift changes the intent enough to require a new problem cycle.",
				)
				if err != nil {
					t.Fatalf("ResolveAdoptReopen: %v", err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			app := newGovernanceTestApp(t)
			defer app.shutdown(context.Background())

			decisionID, findingID := seedDriftAdoptFinding(t, app)

			testCase.resolve(t, app, findingID)
			assertAdoptRefreshReport(t, app, decisionID, findingID, testCase.action)
		})
	}
}

func TestAdoptStaleResolutionsCreateRefreshReport(t *testing.T) {
	testCases := []struct {
		name    string
		action  string
		resolve func(t *testing.T, app *App, findingID string)
	}{
		{
			name:   "measure",
			action: "measure",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptMeasure(
					findingID,
					"Re-measured the stale decision after reviewing the evidence trail.",
					"accepted",
				)
				if err != nil {
					t.Fatalf("ResolveAdoptMeasure: %v", err)
				}
			},
		},
		{
			name:   "waive",
			action: "waive",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptWaive(
					findingID,
					"Need more time before rotating this decision into a new cycle.",
				)
				if err != nil {
					t.Fatalf("ResolveAdoptWaive: %v", err)
				}
			},
		},
		{
			name:   "deprecate",
			action: "deprecate",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptDeprecate(
					findingID,
					"The stale desktop slice is intentionally retired.",
				)
				if err != nil {
					t.Fatalf("ResolveAdoptDeprecate: %v", err)
				}
			},
		},
		{
			name:   "reopen",
			action: "reopen",
			resolve: func(t *testing.T, app *App, findingID string) {
				t.Helper()

				_, err := app.ResolveAdoptReopen(
					findingID,
					"The stale decision needs a fresh problem framing.",
				)
				if err != nil {
					t.Fatalf("ResolveAdoptReopen: %v", err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			app := newGovernanceTestApp(t)
			defer app.shutdown(context.Background())

			decisionID, findingID := seedStaleAdoptFinding(
				t,
				app,
				artifact.StaleCategoryEvidenceExpired,
			)

			testCase.resolve(t, app, findingID)
			assertAdoptRefreshReport(t, app, decisionID, findingID, testCase.action)
		})
	}
}

func newGovernanceTestApp(t *testing.T) *App {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "desktop-governance")
	err := os.MkdirAll(filepath.Join(projectRoot, "internal", "auth"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	err = os.WriteFile(
		filepath.Join(projectRoot, "go.mod"),
		[]byte("module example.com/desktop-governance\n\ngo 1.25.0\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	err = os.WriteFile(
		filepath.Join(projectRoot, "internal", "auth", "auth.go"),
		[]byte("package auth\n\nfunc Enabled() bool { return true }\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("write auth.go: %v", err)
	}

	setup := NewApp()
	if _, err := setup.InitProject(projectRoot); err != nil {
		t.Fatalf("InitProject: %v", err)
	}

	app := NewApp()
	app.projectRoot = projectRoot
	app.startup(context.Background())

	if app.store == nil {
		t.Fatal("expected artifact store after startup")
	}

	return app
}

func addGovernanceEvidenceHistory(t *testing.T, app *App, decisionID string) {
	t.Helper()

	recentEvidence := artifact.EvidenceItem{
		ID:              "ev-recent",
		Type:            "benchmark",
		Content:         "Recent desktop operator benchmark weakened confidence in the current refresh loop.",
		Verdict:         "weakens",
		CongruenceLevel: 2,
		FormalityLevel:  2,
		ValidUntil:      time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339),
	}
	expiredEvidence := artifact.EvidenceItem{
		ID:              "ev-expired",
		Type:            "measurement",
		Content:         "Initial desktop verification supported the flow before the evidence went stale.",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
	}

	err := app.store.AddEvidenceItem(context.Background(), &recentEvidence, decisionID)
	if err != nil {
		t.Fatalf("AddEvidenceItem recent: %v", err)
	}

	err = app.store.AddEvidenceItem(context.Background(), &expiredEvidence, decisionID)
	if err != nil {
		t.Fatalf("AddEvidenceItem expired: %v", err)
	}
}

func findGovernanceFindingID(
	t *testing.T,
	findings []GovernanceFindingView,
	decisionID string,
	category artifact.StaleCategory,
) string {
	t.Helper()

	for _, finding := range findings {
		if finding.ArtifactRef != decisionID {
			continue
		}
		if finding.Category != string(category) {
			continue
		}
		return finding.ID
	}

	t.Fatalf("expected finding %s for %s, findings=%+v", category, decisionID, findings)
	return ""
}

func seedDriftAdoptFinding(t *testing.T, app *App) (string, string) {
	t.Helper()

	decisionID := seedGovernanceDecision(t, app)

	_, err := artifact.Baseline(context.Background(), app.store, app.projectRoot, artifact.BaselineInput{
		DecisionRef: decisionID,
	})
	if err != nil {
		t.Fatalf("Baseline: %v", err)
	}

	err = os.WriteFile(
		filepath.Join(app.projectRoot, "internal", "auth", "auth.go"),
		[]byte("package auth\n\nfunc Enabled() bool { return false }\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("WriteFile auth.go: %v", err)
	}

	overview, err := app.GetGovernanceOverview()
	if err != nil {
		t.Fatalf("GetGovernanceOverview: %v", err)
	}

	findingID := findGovernanceFindingID(
		t,
		overview.Findings,
		decisionID,
		artifact.StaleCategoryDecisionStale,
	)

	return decisionID, findingID
}

func hasGovernanceDriftFinding(findings []GovernanceFindingView, decisionID string) bool {
	for _, finding := range findings {
		if finding.ArtifactRef != decisionID {
			continue
		}
		if finding.DriftCount == 0 {
			continue
		}

		return true
	}

	return false
}

func seedStaleAdoptFinding(
	t *testing.T,
	app *App,
	category artifact.StaleCategory,
) (string, string) {
	t.Helper()

	decisionID := seedGovernanceDecision(t, app)
	addGovernanceEvidenceHistory(t, app, decisionID)

	if _, err := app.governance.scan(context.Background(), false); err != nil {
		t.Fatalf("governance scan: %v", err)
	}

	overview, err := app.GetGovernanceOverview()
	if err != nil {
		t.Fatalf("GetGovernanceOverview: %v", err)
	}

	findingID := findGovernanceFindingID(t, overview.Findings, decisionID, category)

	return decisionID, findingID
}

func assertAdoptRefreshReport(
	t *testing.T,
	app *App,
	decisionID string,
	findingID string,
	action string,
) {
	t.Helper()

	reports, err := app.store.ListByKind(context.Background(), artifact.KindRefreshReport, 20)
	if err != nil {
		t.Fatalf("ListByKind refresh reports: %v", err)
	}

	for _, summary := range reports {
		report, getErr := app.store.Get(context.Background(), summary.Meta.ID)
		if getErr != nil {
			t.Fatalf("Get refresh report %s: %v", summary.Meta.ID, getErr)
		}
		if !hasArtifactLink(report.Meta.Links, decisionID, "refreshes") {
			continue
		}
		if !hasArtifactLink(report.Meta.Links, findingID, "resolves") {
			continue
		}

		if !strings.Contains(report.Body, "## Finding\n\n"+findingID) {
			t.Fatalf("refresh report missing finding %s:\n%s", findingID, report.Body)
		}
		if !strings.Contains(report.Body, "## Action\n\n"+action) {
			t.Fatalf("refresh report missing action %q:\n%s", action, report.Body)
		}

		return
	}

	t.Fatalf(
		"expected RefreshReport linked to decision %s and finding %s",
		decisionID,
		findingID,
	)
}

func hasArtifactLink(links []artifact.Link, ref string, linkType string) bool {
	for _, link := range links {
		if link.Ref != ref {
			continue
		}
		if link.Type != linkType {
			continue
		}

		return true
	}

	return false
}

func seedGovernanceDecision(t *testing.T, app *App) string {
	t.Helper()

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:         "Desktop governance execution gap",
		Signal:        "Decision execution is missing coverage and stale-scan context in the desktop shell.",
		Acceptance:    "An operator can implement and verify from the desktop shell with visible governance context.",
		BlastRadius:   "Desktop governance surfaces",
		Reversibility: "medium",
		Mode:          "standard",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	validUntil := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	verifyAfter := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Desktop governance execution loop",
		WhySelected:     "It closes the missing operator loop with the smallest reversible slice.",
		SelectionPolicy: "Prefer the backend-authoritative path that keeps governance rules in Go.",
		CounterArgument: "The desktop shell could stay read-only and leave governance execution to the CLI.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Read-only governance dashboard",
				Reason:  "It would keep operators jumping back to the CLI for execution and refresh work.",
			},
		},
		WeakestLink:         "Background scan freshness depends on module detection and coverage indexing staying healthy.",
		Invariants:          []string{"Desktop uses shared artifact logic as the single source of truth."},
		Admissibility:       []string{"Do not duplicate stale or coverage rules in the frontend."},
		AffectedFiles:       []string{"internal/auth/auth.go"},
		FirstModuleCoverage: true,
		ValidUntil:          validUntil,
		Rollback: &DecisionRollbackInput{
			Triggers:    []string{"Coverage or stale scan results become misleading in the desktop shell."},
			Steps:       []string{"Fall back to the CLI-driven governance workflow while the desktop slice is repaired."},
			BlastRadius: "Desktop governance views only",
		},
		Predictions: []DecisionPredictionInput{
			{
				Claim:       "Operators can verify the decision from the desktop shell.",
				Observable:  "desktop verification task",
				Threshold:   "verification task records a measurement",
				VerifyAfter: verifyAfter,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	return decision.ID
}
