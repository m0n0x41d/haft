package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopReasoningAuthoringFlow(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Desktop reasoning authoring gap",
		Signal:      "The desktop shell can view reasoning artifacts but cannot author them.",
		Acceptance:  "An operator can frame, compare, and decide from the desktop UI without dropping to CLI.",
		BlastRadius: "Desktop authoring workflows and the local project database",
		Mode:        "standard",
		Constraints: []string{
			"Use shared artifact logic as the single source of truth",
			"Keep the desktop layer reversible",
		},
		OptimizationTargets: []string{
			"Authoring loop completion time",
			"Parity with CLI validation rules",
		},
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	characterized, err := app.CharacterizeProblem(ProblemCharacterizationInput{
		ProblemRef: problem.ID,
		Dimensions: []ComparisonDimensionInput{
			{
				Name:         "operator load",
				ScaleType:    "ordinal",
				Polarity:     "lower_better",
				Role:         "target",
				HowToMeasure: "Estimate setup and editing effort for each flow",
			},
			{
				Name:         "implementation risk",
				ScaleType:    "ordinal",
				Polarity:     "lower_better",
				Role:         "constraint",
				HowToMeasure: "Can the desktop shell reuse existing backend logic without divergence?",
			},
		},
		ParityPlan: &ParityPlanInput{
			BaselineSet:       []string{"var-1", "var-2"},
			Window:            "single release step",
			Budget:            "one desktop iteration",
			MissingDataPolicy: "explicit_abstain",
			PinnedConditions: []string{
				"Use the same project and artifact store",
			},
		},
	})
	if err != nil {
		t.Fatalf("CharacterizeProblem: %v", err)
	}

	if characterized.LatestCharacterization == nil {
		t.Fatal("expected latest characterization")
	}

	portfolio, err := app.CreatePortfolio(PortfolioCreateInput{
		ProblemRef: problem.ID,
		Variants: []PortfolioVariantInput{
			{
				ID:                 "var-1",
				Title:              "Inline modal authoring",
				Description:        "Keep authoring forms inside the existing pages with backend persistence.",
				WeakestLink:        "Dense forms can become visually heavy.",
				NoveltyMarker:      "Preserves the current navigation model.",
				SteppingStone:      true,
				SteppingStoneBasis: "Adds the missing write path without redesigning the shell.",
				Strengths: []string{
					"Low blast radius",
					"Fastest to ship",
				},
				Risks: []string{
					"Can feel cramped on smaller screens",
				},
			},
			{
				ID:            "var-2",
				Title:         "Dedicated authoring workspace",
				Description:   "Introduce a separate composer flow for problems, compare, and decisions.",
				WeakestLink:   "Larger UI and navigation change.",
				NoveltyMarker: "Adds a focused workspace instead of page-local composition.",
				Strengths: []string{
					"More room for dense forms",
				},
				Risks: []string{
					"Higher implementation cost",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	compared, err := app.ComparePortfolio(PortfolioCompareInput{
		PortfolioRef: portfolio.ID,
		Dimensions: []string{
			"operator load",
			"implementation risk",
		},
		Scores: map[string]map[string]string{
			"var-1": {
				"operator load":       "Low",
				"implementation risk": "Low",
			},
			"var-2": {
				"operator load":       "Medium",
				"implementation risk": "Medium",
			},
		},
		PolicyApplied: "Prefer the option that closes the loop quickly without splitting validation logic.",
		SelectedRef:   "var-1",
		DominatedNotes: []DominatedNoteInput{
			{
				Variant:     "var-2",
				DominatedBy: []string{"var-1"},
				Summary:     "The dedicated workspace adds more UI and navigation work without beating the lighter inline flow on either dimension.",
			},
		},
		ParetoTradeoffs: []TradeoffNoteInput{
			{
				Variant: "var-1",
				Summary: "Wins on delivery speed and consistency, but still needs careful layout discipline to stay readable.",
			},
		},
		Recommendation: "Use the inline authoring flow first, then graduate to a dedicated workspace only if the interaction density outgrows the current shell.",
	})
	if err != nil {
		t.Fatalf("ComparePortfolio: %v", err)
	}

	if compared.Comparison == nil {
		t.Fatal("expected stored comparison")
	}

	if got := compared.Comparison.NonDominatedSet; len(got) != 1 || got[0] != "var-1" {
		t.Fatalf("unexpected Pareto front: %#v", got)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		PortfolioRef:    portfolio.ID,
		SelectedRef:     "var-1",
		WhySelected:     "It closes the missing authoring loop while preserving shared compare and decision validation in Go.",
		SelectionPolicy: "Choose the smallest reversible step that keeps the rules authoritative in one place.",
		CounterArgument: "Inline authoring could turn the existing pages into crowded multi-mode surfaces.",
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Form density blocks routine authoring work",
			},
			Steps: []string{
				"Extract the forms into a dedicated workspace route",
				"Keep the same backend bindings and artifact contracts",
			},
			BlastRadius: "Frontend composition only",
		},
		Invariants: []string{
			"Compare validation remains backend-authoritative",
			"Desktop authoring writes the same artifact structures as CLI/MCP flows",
		},
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	if decision.SelectedTitle != "Inline modal authoring" {
		t.Fatalf("expected selected title to resolve from variant, got %q", decision.SelectedTitle)
	}

	if len(decision.WhyNotOthers) == 0 {
		t.Fatal("expected rejected alternatives to be defaulted from the portfolio")
	}
}

func TestDesktopStartupUsesSharedProjectDatabaseForArtifactsAndTasks(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	if app.dbConn == nil {
		t.Fatal("expected desktop database connection after startup")
	}

	if app.tasks == nil || app.tasks.store == nil || app.tasks.store.db == nil {
		t.Fatal("expected desktop task store after startup")
	}

	rawDB := app.dbConn.GetRawDB()

	if app.store.DB() != rawDB {
		t.Fatal("expected artifact store to use the active project database")
	}

	if app.tasks.store.db != rawDB {
		t.Fatal("expected task store to use the active project database")
	}
}

func newAuthoringTestApp(t *testing.T) *App {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "desktop-authoring")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
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
