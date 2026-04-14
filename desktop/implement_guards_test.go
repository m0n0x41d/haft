package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestDashboardDecisionImplementGuardWarnsForParitySubjectiveAndInvariantGaps(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Implement guard warning coverage",
		Signal:      "Dashboard Implement currently misses fairness and verification warnings.",
		Acceptance:  "Operators see the same execution warnings before spawning work.",
		BlastRadius: "Desktop dashboard only",
		Mode:        "standard",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	_, err = app.CharacterizeProblem(ProblemCharacterizationInput{
		ProblemRef: problem.ID,
		Dimensions: []ComparisonDimensionInput{
			{
				Name:         "maintainable",
				ScaleType:    "ordinal",
				Polarity:     "higher_better",
				Role:         "target",
				HowToMeasure: "Team judgment",
			},
		},
	})
	if err != nil {
		t.Fatalf("CharacterizeProblem: %v", err)
	}

	portfolio, err := app.CreatePortfolio(PortfolioCreateInput{
		ProblemRef: problem.ID,
		Mode:       "standard",
		Variants: []PortfolioVariantInput{
			{
				ID:                 "var-1",
				Title:              "Guard in dashboard",
				Description:        "Surface warnings in the dashboard before implementation.",
				WeakestLink:        "Needs careful UI copy to stay precise.",
				NoveltyMarker:      "Keeps execution review in the existing dashboard.",
				SteppingStone:      true,
				SteppingStoneBasis: "Adds the missing warnings without changing the execution backend.",
			},
			{
				ID:            "var-2",
				Title:         "Guard in task view",
				Description:   "Let implementation start and warn later from the task screen.",
				WeakestLink:   "Fails too late in the operator flow.",
				NoveltyMarker: "Defers execution review into the task surface.",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	_, err = app.ComparePortfolio(PortfolioCompareInput{
		PortfolioRef: portfolio.ID,
		Dimensions:   []string{"maintainable"},
		Scores: map[string]map[string]string{
			"var-1": {"maintainable": "High"},
			"var-2": {"maintainable": "Medium"},
		},
		PolicyApplied: "Prefer the tighter execution loop.",
		SelectedRef:   "var-1",
		DominatedNotes: []DominatedNoteInput{
			{
				Variant:     "var-2",
				DominatedBy: []string{"var-1"},
				Summary:     "The later warning flow loses because it delays the operator feedback.",
			},
		},
		ParetoTradeoffs: []TradeoffNoteInput{
			{
				Variant: "var-1",
				Summary: "Keeps the operator on the dashboard path but adds an extra pre-flight review step.",
			},
		},
		Recommendation: "Keep the warning at the execution entry point.",
	})
	if err != nil {
		t.Fatalf("ComparePortfolio: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		PortfolioRef:    portfolio.ID,
		SelectedRef:     "var-1",
		WhySelected:     "Warnings belong exactly where the operator starts implementation.",
		SelectionPolicy: "Prefer the smallest reversible execution guard.",
		CounterArgument: "Prompting before execution can feel like friction when the operator already trusts the decision.",
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Operators consistently bypass the warnings without learning anything new.",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	dashboard, err := app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	summary := findDashboardDecision(t, dashboard, decision.ID)

	if summary.ImplementGuard.BlockedReason != "" {
		t.Fatalf("BlockedReason = %q, want no hard block", summary.ImplementGuard.BlockedReason)
	}

	if len(summary.ImplementGuard.WarningMessages) != 1 {
		t.Fatalf("WarningMessages = %#v, want one invariant warning", summary.ImplementGuard.WarningMessages)
	}

	if got := summary.ImplementGuard.WarningMessages[0]; got != "No invariants defined — post-execution verification will be skipped" {
		t.Fatalf("warning = %q", got)
	}

	if len(summary.ImplementGuard.ConfirmationMessages) != 2 {
		t.Fatalf("ConfirmationMessages = %#v, want parity + subjective warnings", summary.ImplementGuard.ConfirmationMessages)
	}

	if !containsText(summary.ImplementGuard.ConfirmationMessages, "No parity plan recorded") {
		t.Fatalf("expected parity warning in %#v", summary.ImplementGuard.ConfirmationMessages)
	}

	if !containsText(summary.ImplementGuard.ConfirmationMessages, "unresolved subjective dimensions") {
		t.Fatalf("expected subjective warning in %#v", summary.ImplementGuard.ConfirmationMessages)
	}
}

func TestDashboardDecisionImplementGuardBlocksConflictingActiveDecisions(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Conflicting active decisions",
		Signal:      "Legacy data can leave two active decisions on one problem.",
		Acceptance:  "Implement blocks until one decision is superseded.",
		BlastRadius: "Desktop dashboard only",
		Mode:        "standard",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Keep the first decision active",
		WhySelected:     "Provides the baseline path for the operator.",
		SelectionPolicy: "Single governing decision per problem.",
		CounterArgument: "A legacy pair of active decisions might represent an unresolved migration rather than a broken state.",
		WeakestLink:     "Legacy data can still bypass creation-time validation.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Allow both decisions to run",
				Reason:  "Violates the single governing decision invariant.",
			},
		},
		Invariants: []string{
			"Exactly one active decision governs a problem",
		},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Conflict detection proves too noisy for valid decision chains.",
			},
		},
		Mode: "standard",
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	conflictingFields, err := json.Marshal(artifact.DecisionFields{
		ProblemRefs:   []string{problem.ID},
		SelectedTitle: "Conflicting decision",
		WhySelected:   "Bypasses validation to simulate legacy drift.",
		Invariants: []string{
			"Exactly one active decision governs a problem",
		},
	})
	if err != nil {
		t.Fatalf("Marshal conflicting DecisionFields: %v", err)
	}

	err = app.store.Create(app.ctx, &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     "dec-conflict-legacy-001",
			Kind:   artifact.KindDecisionRecord,
			Status: artifact.StatusActive,
			Mode:   artifact.ModeStandard,
			Title:  "Conflicting decision",
			Links: []artifact.Link{
				{Ref: problem.ID, Type: "based_on"},
			},
		},
		StructuredData: string(conflictingFields),
	})
	if err != nil {
		t.Fatalf("Create conflicting decision: %v", err)
	}

	dashboard, err := app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	summary := findDashboardDecision(t, dashboard, decision.ID)

	if got := summary.ImplementGuard.BlockedReason; got != "Multiple active decisions for this problem — supersede one first" {
		t.Fatalf("BlockedReason = %q", got)
	}

	if _, err := app.ImplementDecision(decision.ID, "claude", false, ""); err == nil {
		t.Fatal("ImplementDecision should reject conflicting active decisions")
	} else if !strings.Contains(err.Error(), "Multiple active decisions for this problem") {
		t.Fatalf("ImplementDecision error = %v", err)
	}
}

func findDashboardDecision(t *testing.T, dashboard *DashboardView, decisionID string) DecisionView {
	t.Helper()

	for _, decision := range dashboard.HealthyDecisions {
		if decision.ID == decisionID {
			return decision
		}
	}

	for _, decision := range dashboard.PendingDecisions {
		if decision.ID == decisionID {
			return decision
		}
	}

	for _, decision := range dashboard.UnassessedDecisions {
		if decision.ID == decisionID {
			return decision
		}
	}

	t.Fatalf("decision %s not found in dashboard", decisionID)
	return DecisionView{}
}

func containsText(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}

	return false
}
