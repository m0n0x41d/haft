package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
