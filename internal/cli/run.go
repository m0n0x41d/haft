package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/project"
)

var implementCmd = &cobra.Command{
	Use:   "run <decision-ref>",
	Short: "Implement a decision — plan, execute, verify, baseline",
	Long: `Run the full implementation pipeline for a DecisionRecord.

1. PLAN  — agent decomposes the decision into ordered tasks
           (or loads existing plan from .haft/plans/)
2. EXECUTE — each task runs in sequence with build verification
3. REVIEW — invariants checked, tests run, drift detected
4. BASELINE — file snapshots taken on success

By default, pauses between tasks for human review.
Use --auto to run the full pipeline without stops.

Examples:
  haft run dec-20260414-001              # interactive — pauses between tasks
  haft run dec-20260414-001 --auto       # full pipeline, no stops
  haft run dec-20260414-001 --agent claude
  haft run dec-20260414-001 -c spec/EXECUTION_CONTRACT.md
  haft run dec-20260414-001 -p "Focus on error handling"`,
	Args: cobra.ExactArgs(1),
	RunE: runImplement,
}

var (
	implementAgent   string
	implementAuto    bool
	implementContext []string
	implementPrompt  string
)

func init() {
	implementCmd.Flags().StringVar(&implementAgent, "agent", "codex", "Agent backend: codex, claude")
	implementCmd.Flags().BoolVar(&implementAuto, "auto", false, "Run full pipeline without pausing between tasks")
	implementCmd.Flags().StringArrayVarP(&implementContext, "context", "c", nil, "Extra context files to include in prompts (repeatable)")
	implementCmd.Flags().StringVarP(&implementPrompt, "prompt", "p", "", "Extra instructions for the agent")
	rootCmd.AddCommand(implementCmd)
}

func runImplement(cmd *cobra.Command, args []string) error {
	decisionRef := args[0]
	ctx := context.Background()

	// ── Setup ────────────────────────────────────────────────────
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	haftDir := filepath.Join(projectRoot, ".haft")

	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("db path: %w", err)
	}

	store, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	artStore := artifact.NewStore(store.GetRawDB())
	graphStore := graph.NewStore(store.GetRawDB())
	ui := &runUI{startTime: time.Now()}

	// ── Load decision ────────────────────────────────────────────
	decision, err := artStore.Get(ctx, decisionRef)
	if err != nil {
		return fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if decision.Meta.Kind != artifact.KindDecisionRecord {
		return fmt.Errorf("%s is %s, not a DecisionRecord", decisionRef, decision.Meta.Kind)
	}
	if decision.Meta.Status != artifact.StatusActive {
		return fmt.Errorf("%s is %s — can only implement active decisions", decisionRef, decision.Meta.Status)
	}

	affectedFiles, err := artStore.GetAffectedFiles(ctx, decisionRef)
	if err != nil {
		return fmt.Errorf("get affected files: %w", err)
	}

	var allInvariants []string
	for _, f := range affectedFiles {
		invs, gErr := graphStore.FindInvariantsForFile(ctx, f.Path)
		if gErr == nil {
			for _, inv := range invs {
				allInvariants = append(allInvariants, fmt.Sprintf("[%s] %s", inv.DecisionID, inv.Text))
			}
		}
	}
	allInvariants = uniqueStrings(allInvariants)

	// ── Header ───────────────────────────────────────────────────
	mode := "interactive"
	if implementAuto {
		mode = "auto"
	}

	ui.header(fmt.Sprintf("Implement: %s", decision.Meta.Title))
	ui.meta("Decision", decisionRef)
	ui.meta("Files", fmt.Sprintf("%d affected", len(affectedFiles)))
	ui.meta("Invariants", fmt.Sprintf("%d governing", len(allInvariants)))
	ui.meta("Agent", implementAgent)
	ui.meta("Mode", mode)
	fmt.Println()

	if !implementAuto {
		fmt.Printf("  %sProceed? [Y/n] %s", aBold, aReset)
		var answer string
		fmt.Scanln(&answer)
		if answer == "n" || answer == "N" {
			fmt.Println("  Cancelled.")
			return nil
		}
	}

	// ── Phase 1: Plan ────────────────────────────────────────────
	planPath := planFilePath(haftDir, decisionRef)
	var plan *executionPlan

	if _, statErr := os.Stat(planPath); statErr == nil {
		ui.phase("Plan (loaded)")
		ui.ok(fmt.Sprintf("Using existing plan: %s", planPath))
		var pErr error
		plan, pErr = parsePlanFile(planPath, decisionRef, decision.Meta.Title)
		if pErr != nil {
			return fmt.Errorf("parse plan: %w", pErr)
		}
	} else {
		plan, err = generatePlan(decision, affectedFiles, allInvariants, projectRoot, implementAgent, ui)
		if err != nil {
			return fmt.Errorf("plan generation: %w", err)
		}
	}

	fmt.Printf("  %s%d tasks planned%s\n", aDim, len(plan.tasks), aReset)
	for _, t := range plan.tasks {
		fmt.Printf("  %s%-4s%s %s\n", aCyan, t.id, aReset, t.title)
	}
	fmt.Println()

	if !implementAuto {
		fmt.Printf("  %sExecute? [Y/n/e(dit plan)] %s", aBold, aReset)
		var answer string
		fmt.Scanln(&answer)
		if answer == "n" || answer == "N" {
			fmt.Printf("  Plan at: %s\n", plan.planFile)
			return nil
		}
		if answer == "e" || answer == "E" {
			fmt.Printf("  Edit: %s\n  Re-run: haft run %s\n", plan.planFile, decisionRef)
			return nil
		}
	}

	// ── Phase 2: Execute ─────────────────────────────────────────
	allPassed := executePlan(ctx, plan, decision, artStore, graphStore, projectRoot, implementAgent, implementAuto, ui)

	if !allPassed {
		ui.fail("Some tasks failed")
		fmt.Printf("\n  %sPlan at: %s%s\n", aDim, plan.planFile, aReset)
		fmt.Println("  Fix issues and re-run: haft run", decisionRef)
		fmt.Println()
		ui.summary()
		fmt.Println()
		return nil
	}

	// ── Phase 3: Review ──────────────────────────────────────────
	reviewOk := finalReview(ctx, decisionRef, graphStore, artStore, projectRoot, implementAgent, ui)

	if !reviewOk {
		ui.fail("Review failed — fix issues before committing")
		fmt.Println()
		ui.summary()
		fmt.Println()
		return nil
	}

	// ── Phase 4: Baseline ────────────────────────────────────────
	ui.phase("Baseline")
	if len(affectedFiles) > 0 {
		baselined, blErr := artifact.Baseline(ctx, artStore, projectRoot, artifact.BaselineInput{
			DecisionRef: decisionRef,
		})
		if blErr != nil {
			ui.warn(fmt.Sprintf("Baseline failed: %v", blErr))
		} else {
			ui.ok(fmt.Sprintf("%d file(s) snapshotted", len(baselined)))
		}
	}

	// ── Done ─────────────────────────────────────────────────────
	fmt.Println()
	ui.ok("Implementation complete — ready for human review")
	fmt.Println()
	ui.summary()

	fmt.Printf("\n  %sNext:%s\n", aBold, aReset)
	fmt.Println("  • git diff")
	fmt.Println("  • git commit")
	fmt.Println("  • Create PR")
	fmt.Println()

	return nil
}
