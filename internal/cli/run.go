package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"
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
	implementNoTUI   bool
)

// useTUI indicates whether the BubbleTea TUI should be used for rendering.
// Computed at the start of runImplement: true when stdout is a terminal and --no-tui is not set.
var useTUI bool

func init() {
	implementCmd.Flags().StringVar(&implementAgent, "agent", "codex", "Agent backend: codex, claude")
	implementCmd.Flags().BoolVar(&implementAuto, "auto", false, "Run full pipeline without pausing between tasks")
	implementCmd.Flags().StringArrayVarP(&implementContext, "context", "c", nil, "Extra context files to include in prompts (repeatable)")
	implementCmd.Flags().StringVarP(&implementPrompt, "prompt", "p", "", "Extra instructions for the agent")
	implementCmd.Flags().BoolVar(&implementNoTUI, "no-tui", false, "Disable TUI rendering, use plain text output")
	rootCmd.AddCommand(implementCmd)
}

func runImplement(cmd *cobra.Command, args []string) error {
	useTUI = isatty.IsTerminal(os.Stdout.Fd()) && !implementNoTUI

	decisionRef := args[0]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Event channel ────────────────────────────────────────────
	ch := make(chan RunEvent, 256)
	ev := NewEventSender(ch)
	startTime := time.Now()

	// pipeline runs the full implementation flow, emitting events via ev.
	// It does NOT close the channel — the caller handles that so the
	// channel is closed even when the pipeline returns an error before
	// sending PipelineDone.
	pipeline := func() error {
		// ── Setup ────────────────────────────────────────────────────
		projectRoot, err := findProjectRoot()
		if err != nil {
			return fmt.Errorf("not a haft project: %w", err)
		}
		tc := detectToolchain(projectRoot)
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
		auto := implementAuto || useTUI
		mode := "interactive"
		if auto {
			mode = "auto"
		}

		ev.Phase(fmt.Sprintf("Implement: %s", decision.Meta.Title))
		ev.Meta("Decision", decisionRef)
		ev.Meta("Files", fmt.Sprintf("%d affected", len(affectedFiles)))
		ev.Meta("Invariants", fmt.Sprintf("%d governing", len(allInvariants)))
		ev.Meta("Agent", implementAgent)
		tcInfo := tc.lang
		if tc.buildCmd != "" {
			tcInfo += " (" + tc.buildCmd + ")"
		}
		ev.Meta("Toolchain", tcInfo)
		ev.Meta("Mode", mode)

		if !auto {
			fmt.Printf("\n  %sProceed? [Y/n] %s", aBold, aReset)
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
			ev.Phase("Plan (loaded)")
			ev.OK(fmt.Sprintf("Using existing plan: %s", planPath))
			var pErr error
			plan, pErr = parsePlanFile(planPath, decisionRef, decision.Meta.Title)
			if pErr != nil {
				return fmt.Errorf("parse plan: %w", pErr)
			}
			emitPlanLoaded(ev, plan)
		} else {
			plan, err = generatePlan(decision, affectedFiles, allInvariants, projectRoot, implementAgent, tc, implementContext, implementPrompt, ev)
			if err != nil {
				return fmt.Errorf("plan generation: %w", err)
			}
		}

		if !auto {
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
		allPassed := executePlan(ctx, plan, decision, artStore, graphStore, projectRoot, implementAgent, auto, tc, implementContext, implementPrompt, ev)

		if !allPassed {
			ev.Fail("Some tasks failed")
			ev.Meta("Plan", plan.planFile)
			ev.OK(fmt.Sprintf("Fix issues and re-run: haft run %s", decisionRef))
			ev.Done(time.Since(startTime), false)
			return nil
		}

		// ── Phase 3: Review ──────────────────────────────────────────
		reviewOk := finalReview(ctx, decisionRef, graphStore, artStore, projectRoot, implementAgent, auto, tc, ev)

		if !reviewOk {
			ev.Fail("Review failed — fix issues before committing")
			ev.Done(time.Since(startTime), false)
			return nil
		}

		// ── Phase 4: Baseline ────────────────────────────────────────
		ev.Phase("Baseline")
		if len(affectedFiles) > 0 {
			baselined, blErr := artifact.Baseline(ctx, artStore, projectRoot, artifact.BaselineInput{
				DecisionRef: decisionRef,
			})
			if blErr != nil {
				ev.Warn(fmt.Sprintf("Baseline failed: %v", blErr))
			} else {
				ev.OK(fmt.Sprintf("%d file(s) snapshotted", len(baselined)))
			}
		}

		// ── Done ─────────────────────────────────────────────────────
		ev.OK("Implementation complete — ready for human review")
		ev.OK("Next: git diff, git commit, Create PR")
		ev.Done(time.Since(startTime), true)

		return nil
	}

	// ── Renderer selection ───────────────────────────────────────
	if useTUI {
		// TUI mode: pipeline runs in a background goroutine; BubbleTea
		// blocks the main goroutine (required for terminal control).
		pipelineErr := make(chan error, 1)
		go func() {
			err := pipeline()
			if err != nil {
				// Pipeline hit a setup error before sending PipelineDone.
				// Emit one so the TUI knows the pipeline is finished.
				ev.Fail(err.Error())
				ev.Done(time.Since(startTime), false)
			}
			ev.Close()
			pipelineErr <- err
		}()

		model := newTUIModel(ch)
		p := tea.NewProgram(model)
		if _, tuiErr := p.Run(); tuiErr != nil {
			cancel()
			<-pipelineErr
			return fmt.Errorf("TUI: %w", tuiErr)
		}

		// TUI exited (q or Ctrl+C). Cancel pipeline context so
		// context-aware operations (DB calls, future subprocess mgmt) stop.
		cancel()
		return <-pipelineErr
	}

	// Plain-text mode: renderer in background, pipeline on main goroutine.
	consumerDone := make(chan struct{})
	go func() {
		r := &plainRenderer{}
		r.Run(ch)
		close(consumerDone)
	}()

	err := pipeline()
	ev.Close()
	<-consumerDone
	return err
}

// emitPlanLoaded sends a PlanLoaded event from an executionPlan.
func emitPlanLoaded(ev *EventSender, plan *executionPlan) {
	summaries := make([]PlanTaskSummary, len(plan.tasks))
	for i, t := range plan.tasks {
		summaries[i] = PlanTaskSummary{
			ID:         t.id,
			Title:      t.title,
			Acceptance: t.acceptance,
			Files:      t.files,
		}
	}
	ev.Plan(plan.decisionRef, plan.title, plan.planFile, summaries)
}

// spawnAgentWithEvents runs an agent, routing output through events when TUI mode is active.
// In plain-text mode, agent stdout goes directly to the terminal.
func spawnAgentWithEvents(agent, prompt, projectRoot string, ev *EventSender) error {
	if !useTUI {
		return spawnAgent(agent, prompt, projectRoot)
	}

	pr, pw := io.Pipe()
	done := make(chan struct{})
	go func() {
		parseAgentJSONL(pr, ev)
		close(done)
	}()
	err := spawnAgent(agent, prompt, projectRoot, pw)
	pw.Close()
	<-done
	return err
}

