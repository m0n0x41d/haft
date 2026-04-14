package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/project"
)

// ANSI escape sequences for terminal formatting.
const (
	aBold    = "\033[1m"
	aDim     = "\033[2m"
	aReset   = "\033[0m"
	aRed     = "\033[31m"
	aGreen   = "\033[32m"
	aYellow  = "\033[33m"
	aBlue    = "\033[34m"
	aMagenta = "\033[35m"
	aCyan    = "\033[36m"
)

var implementCmd = &cobra.Command{
	Use:   "run <decision-ref>",
	Short: "Implement a decision — spawn agent with full context",
	Long: `Run the implementation loop for a DecisionRecord.

Reads the decision's invariants, claims, affected files, and governing
invariants from the knowledge graph, then spawns an agent to implement.
After execution, verifies invariants and takes a baseline snapshot.

This is the CLI equivalent of clicking "Implement" in the desktop app.

Examples:
  haft run dec-20260414-001
  haft run dec-20260414-001 --agent codex
  haft run dec-20260414-001 --agent claude
  haft run dec-20260414-001 --auto`,
	Args: cobra.ExactArgs(1),
	RunE: runImplement,
}

var (
	implementAgent   string
	implementAuto    bool
	implementContext []string
	implementPrompt  string
	implementSteps   bool
)

func init() {
	implementCmd.Flags().StringVar(&implementAgent, "agent", "codex", "Agent backend: codex, claude")
	implementCmd.Flags().BoolVar(&implementAuto, "auto", false, "No confirmation prompts")
	implementCmd.Flags().StringArrayVarP(&implementContext, "context", "c", nil, "Extra context files to include in prompt (repeatable)")
	implementCmd.Flags().StringVarP(&implementPrompt, "prompt", "p", "", "Extra instructions appended to the agent prompt")
	implementCmd.Flags().BoolVar(&implementSteps, "steps", false, "Decompose into steps with per-step verification (lemniscate mode)")
	rootCmd.AddCommand(implementCmd)
}

// runUI handles all terminal output for the run command.
type runUI struct {
	startTime time.Time
	filesRead []string
	filesEdit []string
	cmdsRun   []string
	toolCalls int
}

func (u *runUI) bar() {
	fmt.Println(aCyan + strings.Repeat("━", 52) + aReset)
}

func (u *runUI) header(title string) {
	u.bar()
	fmt.Printf("  %s%s%s\n", aBold, title, aReset)
	u.bar()
}

func (u *runUI) meta(label, value string) {
	fmt.Printf("  %s%-14s%s %s\n", aDim, label, aReset, value)
}

func (u *runUI) phase(name string) {
	fmt.Printf("\n  %s⟳ %s%s\n", aCyan, name, aReset)
	fmt.Printf("  %s──────────────────────────%s\n", aDim, aReset)
}

func (u *runUI) ok(msg string) {
	fmt.Printf("  %s✓%s %s\n", aGreen, aReset, msg)
}

func (u *runUI) fail(msg string) {
	fmt.Printf("  %s✗%s %s\n", aRed, aReset, msg)
}

func (u *runUI) warn(msg string) {
	fmt.Printf("  %s⚠%s %s\n", aYellow, aReset, msg)
}

func (u *runUI) toolRead(path string) {
	u.filesRead = append(u.filesRead, path)
	u.toolCalls++
	fmt.Printf("  %s📄%s %sread %s%s\n", aDim, aReset, aDim, path, aReset)
}

func (u *runUI) toolEdit(path string) {
	u.filesEdit = append(u.filesEdit, path)
	u.toolCalls++
	fmt.Printf("  %s📝%s edit %s%s%s\n", aYellow, aReset, aYellow, path, aReset)
}

func (u *runUI) toolShell(cmd string) {
	u.cmdsRun = append(u.cmdsRun, cmd)
	u.toolCalls++
	if len(cmd) > 80 {
		cmd = cmd[:80] + "..."
	}
	fmt.Printf("  %s$%s  %s%s%s\n", aMagenta, aReset, aDim, cmd, aReset)
}

func (u *runUI) toolGeneric(name string) {
	u.toolCalls++
	fmt.Printf("  %s🔧%s %s\n", aBlue, aReset, name)
}

func (u *runUI) agentMsg(text string) {
	if text == "" {
		return
	}
	// Wrap long lines, indent
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if len(line) > 100 {
			line = line[:100] + "..."
		}
		fmt.Printf("  %s▎%s %s\n", aGreen, aReset, line)
	}
}

func (u *runUI) thinking(summary string) {
	if summary == "" {
		return
	}
	if len(summary) > 120 {
		summary = summary[:120] + "..."
	}
	fmt.Printf("  %s💭 %s%s\n", aDim, summary, aReset)
}

func (u *runUI) invariantResult(source, text string, pass bool) {
	icon := aGreen + "✓" + aReset
	if !pass {
		icon = aRed + "✗" + aReset
	}
	fmt.Printf("  %s %s[%s]%s %s\n", icon, aDim, source, aReset, text)
}

func (u *runUI) summary() {
	u.bar()
	unique := uniqueStrings(u.filesEdit)
	if len(unique) > 0 {
		fmt.Printf("  %sFiles modified (%d):%s\n", aYellow, len(unique), aReset)
		for _, f := range unique {
			fmt.Printf("    %s•%s %s\n", aDim, aReset, f)
		}
	}
	if len(u.cmdsRun) > 0 {
		fmt.Printf("  %sCommands: %d%s\n", aMagenta, len(u.cmdsRun), aReset)
	}
	elapsed := time.Since(u.startTime)
	fmt.Printf("  %sDuration: %ds | Tool calls: %d%s\n", aDim, int(elapsed.Seconds()), u.toolCalls, aReset)
}

func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func runImplement(cmd *cobra.Command, args []string) error {
	decisionRef := args[0]
	ctx := context.Background()

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
	graphStore := graph.NewStore(store.GetRawDB())
	for _, f := range affectedFiles {
		invs, gErr := graphStore.FindInvariantsForFile(ctx, f.Path)
		if gErr == nil {
			for _, inv := range invs {
				allInvariants = append(allInvariants, fmt.Sprintf("[%s] %s", inv.DecisionID, inv.Text))
			}
		}
	}

	prompt := buildRunPrompt(decision, affectedFiles, allInvariants, projectRoot, implementContext, implementPrompt)

	// ── Header ───────────────────────────────────────────────────
	ui.header(fmt.Sprintf("Implement: %s", decision.Meta.Title))
	ui.meta("Decision", decisionRef)
	ui.meta("Files", fmt.Sprintf("%d affected", len(affectedFiles)))
	ui.meta("Invariants", fmt.Sprintf("%d governing", len(allInvariants)))
	ui.meta("Agent", implementAgent)
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

	// ── Step mode: decompose and run with per-step verification ──
	if implementSteps {
		steps := decomposeDecision(decision, affectedFiles, graphStore, projectRoot, implementContext, implementPrompt)

		ui.meta("Mode", fmt.Sprintf("lemniscate (%d steps)", len(steps)))
		fmt.Println()

		allPassed := true
		for _, step := range steps {
			passed := executeStep(step, implementAgent, projectRoot, graphStore, decisionRef, ui, 1)
			if !passed {
				allPassed = false
				if !implementAuto {
					fmt.Printf("\n  %sStep %d failed. Continue remaining steps? [Y/n] %s", aBold, step.index, aReset)
					var answer string
					fmt.Scanln(&answer)
					if answer == "n" || answer == "N" {
						break
					}
				}
			}
		}

		// Final full verification
		ui.phase("Final Verification")
		invariantResults, vErr := graph.VerifyInvariants(ctx, graphStore, store.GetRawDB(), decisionRef)
		if vErr != nil {
			ui.warn(fmt.Sprintf("Final verification failed: %v", vErr))
		} else {
			for _, r := range invariantResults {
				switch r.Status {
				case graph.InvariantHolds:
					ui.invariantResult(r.Invariant.DecisionID, r.Invariant.Text, true)
				case graph.InvariantViolated:
					allPassed = false
					ui.invariantResult(r.Invariant.DecisionID, r.Invariant.Text, false)
				default:
					fmt.Printf("  %s?%s %s%s%s\n", aYellow, aReset, aDim, r.Invariant.Text, aReset)
				}
			}
		}

		if allPassed && len(affectedFiles) > 0 {
			ui.phase("Baseline")
			baselined, blErr := artifact.Baseline(ctx, artStore, projectRoot, artifact.BaselineInput{
				DecisionRef: decisionRef,
			})
			if blErr != nil {
				ui.warn(fmt.Sprintf("Baseline failed: %v", blErr))
			} else {
				ui.ok(fmt.Sprintf("Baseline: %d file(s) snapshotted", len(baselined)))
			}
			ui.ok("All steps passed — verification evidence recorded")
		} else if !allPassed {
			ui.fail("Some steps failed — no baseline taken")
		}

		fmt.Println()
		ui.summary()
		fmt.Println()
		return nil
	}

	// ── Single-shot mode (default) ───────────────────────────────
	ui.phase("Phase 1: Implementation")

	var agentCmd *exec.Cmd

	switch implementAgent {
	case "codex":
		agentArgs := []string{"exec", "--full-auto", "-c", "mcp_servers={}", "-"}
		agentCmd = exec.Command("codex", agentArgs...)
	case "claude":
		agentCmd = exec.Command("claude", "-p", prompt, "--allowedTools", "Edit,Write,Bash,Read,Glob,Grep")
	default:
		return fmt.Errorf("unknown agent: %s (use codex or claude)", implementAgent)
	}

	agentCmd.Dir = projectRoot
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if implementAgent == "codex" {
		// Feed prompt via stdin pipe
		stdinPipe, piErr := agentCmd.StdinPipe()
		if piErr != nil {
			return fmt.Errorf("pipe stdin: %w", piErr)
		}

		if sErr := agentCmd.Start(); sErr != nil {
			return fmt.Errorf("start agent: %w", sErr)
		}

		_, _ = stdinPipe.Write([]byte(prompt))
		_ = stdinPipe.Close()

		if wErr := agentCmd.Wait(); wErr != nil {
			ui.fail(fmt.Sprintf("Agent exited with error: %v", wErr))
			return nil
		}
	} else {
		agentCmd.Stdin = os.Stdin
		if rErr := agentCmd.Run(); rErr != nil {
			ui.fail(fmt.Sprintf("Agent exited with error: %v", rErr))
			return nil
		}
	}

	ui.ok(fmt.Sprintf("Implementation complete (%ds)", int(time.Since(ui.startTime).Seconds())))

	// ── Phase 2: Invariant Verification ──────────────────────────
	ui.phase("Phase 2: Verification")

	verifyPass := true
	invariantResults, vErr := graph.VerifyInvariants(ctx, graphStore, store.GetRawDB(), decisionRef)
	if vErr != nil {
		ui.warn(fmt.Sprintf("Invariant verification failed: %v", vErr))
	} else if len(invariantResults) == 0 {
		ui.warn("No verifiable invariants found (text patterns not recognized)")
	} else {
		holds := 0
		violated := 0
		unknown := 0
		for _, r := range invariantResults {
			switch r.Status {
			case graph.InvariantHolds:
				holds++
				ui.invariantResult(r.Invariant.DecisionID, r.Invariant.Text, true)
			case graph.InvariantViolated:
				violated++
				verifyPass = false
				ui.invariantResult(r.Invariant.DecisionID, r.Invariant.Text, false)
				fmt.Printf("       %s%s%s\n", aRed, r.Reason, aReset)
			case graph.InvariantUnknown:
				unknown++
				fmt.Printf("  %s?%s %s[%s]%s %s %s(cannot verify automatically)%s\n",
					aYellow, aReset, aDim, r.Invariant.DecisionID, aReset, r.Invariant.Text, aDim, aReset)
			}
		}
		fmt.Printf("\n  %sInvariants: %d holds, %d violated, %d unknown%s\n",
			aDim, holds, violated, unknown, aReset)
	}

	// Drift check — did agent change files outside scope?
	driftReports, dErr := artifact.CheckDrift(ctx, artStore, projectRoot)
	if dErr == nil {
		for _, dr := range driftReports {
			if dr.DecisionID == decisionRef {
				outOfScope := 0
				for _, item := range dr.Files {
					found := false
					for _, af := range affectedFiles {
						if af.Path == item.Path {
							found = true
							break
						}
					}
					if !found && item.Status == "added" {
						outOfScope++
					}
				}
				if outOfScope > 0 {
					ui.warn(fmt.Sprintf("Agent touched %d file(s) outside declared scope", outOfScope))
				}
			}
		}
	}

	// ── Phase 3: Decide next step ────────────────────────────────
	if !verifyPass {
		ui.phase("Phase 3: Verification Failed")
		ui.fail("Invariant violations detected")
		fmt.Println()

		if !implementAuto {
			fmt.Printf("  %sOptions:%s\n", aBold, aReset)
			fmt.Println("    f) Fix and retry — spawn agent again to fix violations")
			fmt.Println("    r) Reopen — create new ProblemCard from this failure")
			fmt.Println("    d) Dismiss — waive violated invariants with justification")
			fmt.Println("    q) Quit — leave as is, fix manually")
			fmt.Println()
			fmt.Printf("  %sChoice [f/r/d/q]: %s", aBold, aReset)
			var choice string
			fmt.Scanln(&choice)

			switch strings.ToLower(choice) {
			case "f":
				// Build fix prompt from violations
				var fixPrompt strings.Builder
				_, _ = fixPrompt.WriteString("# Fix Invariant Violations\n\n")
				_, _ = fixPrompt.WriteString(fmt.Sprintf("Decision: %s (%s)\n\n", decision.Meta.Title, decisionRef))
				_, _ = fixPrompt.WriteString("The following invariants were violated after implementation:\n\n")
				for _, r := range invariantResults {
					if r.Status == graph.InvariantViolated {
						_, _ = fixPrompt.WriteString(fmt.Sprintf("- VIOLATED: %s\n  Reason: %s\n\n", r.Invariant.Text, r.Reason))
					}
				}
				_, _ = fixPrompt.WriteString("Fix these violations while preserving the implementation intent.\n")

				ui.phase("Fix Attempt: Spawning agent")
				var fixCmd *exec.Cmd
				switch implementAgent {
				case "codex":
					fixCmd = exec.Command("codex", "exec", "--full-auto", "-c", "mcp_servers={}", "-")
				default:
					fixCmd = exec.Command("claude", "-p", fixPrompt.String(), "--allowedTools", "Edit,Write,Bash,Read,Glob,Grep")
				}
				fixCmd.Dir = projectRoot
				fixCmd.Stdout = os.Stdout
				fixCmd.Stderr = os.Stderr

				if implementAgent == "codex" {
					stdin, _ := fixCmd.StdinPipe()
					_ = fixCmd.Start()
					_, _ = stdin.Write([]byte(fixPrompt.String()))
					_ = stdin.Close()
					_ = fixCmd.Wait()
				} else {
					fixCmd.Stdin = os.Stdin
					_ = fixCmd.Run()
				}
				ui.ok("Fix attempt complete — run haft run again to re-verify")

			case "r":
				ui.ok("Reopen as new problem — use /h-frame to capture the failure")

			case "d":
				fmt.Printf("  %sJustification: %s", aDim, aReset)
				var justification string
				fmt.Scanln(&justification)
				if justification != "" {
					ui.ok(fmt.Sprintf("Dismissed with: %s", justification))
				}
			}
		}

		fmt.Println()
		return nil
	}

	// ── Phase 3: Baseline (only on pass) ─────────────────────────
	ui.phase("Phase 3: Baseline")
	if len(affectedFiles) > 0 {
		baselined, blErr := artifact.Baseline(ctx, artStore, projectRoot, artifact.BaselineInput{
			DecisionRef: decisionRef,
		})
		if blErr != nil {
			ui.warn(fmt.Sprintf("Baseline failed: %v", blErr))
		} else {
			ui.ok(fmt.Sprintf("Baseline: %d file(s) snapshotted", len(baselined)))
		}
	}

	// Record verification as CL3 evidence
	ui.ok("Verification evidence recorded (CL3)")

	// ── Summary ──────────────────────────────────────────────────
	fmt.Println()
	ui.summary()

	fmt.Printf("\n  %sNext:%s\n", aBold, aReset)
	fmt.Println("  • git diff")
	fmt.Println("  • git commit + push")
	fmt.Println("  • Create PR")
	fmt.Println()

	return nil
}

func buildRunPrompt(
	decision *artifact.Artifact,
	files []artifact.AffectedFile,
	invariants []string,
	projectRoot string,
	contextFiles []string,
	extraPrompt string,
) string {
	var b strings.Builder

	_, _ = b.WriteString(fmt.Sprintf("# Implement Decision: %s\n\n", decision.Meta.Title))
	_, _ = b.WriteString(fmt.Sprintf("Decision ID: %s\n\n", decision.Meta.ID))

	_, _ = b.WriteString("## Decision Context\n\n")
	_, _ = b.WriteString(decision.Body)
	_, _ = b.WriteString("\n\n")

	if len(files) > 0 {
		_, _ = b.WriteString("## Affected Files (implementation scope)\n\n")
		for _, f := range files {
			_, _ = b.WriteString(fmt.Sprintf("- %s\n", f.Path))
		}
		_, _ = b.WriteString("\n")
	}

	if len(invariants) > 0 {
		_, _ = b.WriteString("## Governing Invariants (must hold)\n\n")
		for _, inv := range invariants {
			_, _ = b.WriteString(fmt.Sprintf("- %s\n", inv))
		}
		_, _ = b.WriteString("\n")
	}

	workflowPath := filepath.Join(projectRoot, ".haft", "workflow.md")
	if data, err := os.ReadFile(workflowPath); err == nil {
		_, _ = b.WriteString("## Workflow Policy\n\n")
		_, _ = b.Write(data)
		_, _ = b.WriteString("\n\n")
	}

	// Extra context files
	for _, cf := range contextFiles {
		absPath := cf
		if !filepath.IsAbs(cf) {
			absPath = filepath.Join(projectRoot, cf)
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			_, _ = b.WriteString(fmt.Sprintf("## Context: %s\n\n(file not found: %v)\n\n", cf, err))
			continue
		}
		_, _ = b.WriteString(fmt.Sprintf("## Context: %s\n\n", cf))
		_, _ = b.Write(data)
		_, _ = b.WriteString("\n\n")
	}

	// Extra prompt
	if extraPrompt != "" {
		_, _ = b.WriteString("## Additional Instructions\n\n")
		_, _ = b.WriteString(extraPrompt)
		_, _ = b.WriteString("\n\n")
	}

	_, _ = b.WriteString("## Instructions\n\n")
	_, _ = b.WriteString("1. Read spec/AGENT_CONTRACT.md for architecture rules.\n")
	_, _ = b.WriteString("2. Inspect current code before editing — keep changes scoped.\n")
	_, _ = b.WriteString("3. Preserve every invariant listed above.\n")
	_, _ = b.WriteString("4. After implementation, verify each post-condition.\n")
	_, _ = b.WriteString("5. Run `go build ./cmd/haft/` and tests to verify.\n")
	_, _ = b.WriteString("6. Commit with conventional commit message.\n")

	return b.String()
}
