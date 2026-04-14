package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/project"
)

var implementCmd = &cobra.Command{
	Use:   "run <decision-ref>",
	Short: "Implement a decision — spawn agent with full context",
	Long: `Run the implementation loop for a DecisionRecord.

Reads the decision's invariants, claims, affected files, and governing
invariants from the knowledge graph, then spawns an agent to implement.
After execution, takes a baseline snapshot.

This is the CLI equivalent of clicking "Implement" in the desktop app.

Examples:
  haft run dec-20260414-001
  haft run dec-20260414-001 --agent codex
  haft run dec-20260414-001 --auto`,
	Args: cobra.ExactArgs(1),
	RunE: runImplement,
}

var (
	implementAgent string
	implementAuto  bool
)

func init() {
	implementCmd.Flags().StringVar(&implementAgent, "agent", "codex", "Agent backend: codex, claude")
	implementCmd.Flags().BoolVar(&implementAuto, "auto", false, "No confirmation prompts")
	rootCmd.AddCommand(implementCmd)
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

	// Load decision
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

	// Load affected files
	affectedFiles, err := artStore.GetAffectedFiles(ctx, decisionRef)
	if err != nil {
		return fmt.Errorf("get affected files: %w", err)
	}

	// Load governing invariants from knowledge graph
	var allInvariants []string
	graphStore := graph.NewStore(store.GetRawDB())
	for _, f := range affectedFiles {
		invs, err := graphStore.FindInvariantsForFile(ctx, f.Path)
		if err == nil {
			for _, inv := range invs {
				allInvariants = append(allInvariants, fmt.Sprintf("[%s] %s", inv.DecisionID, inv.Text))
			}
		}
	}

	// Build prompt
	prompt := buildRunPrompt(decision, affectedFiles, allInvariants, projectRoot)

	// Print summary
	fmt.Printf("\n\033[1m\033[36m Implement: %s\033[0m\n", decision.Meta.Title)
	fmt.Printf("  \033[2mDecision:\033[0m %s\n", decisionRef)
	fmt.Printf("  \033[2mFiles:\033[0m %d affected\n", len(affectedFiles))
	fmt.Printf("  \033[2mInvariants:\033[0m %d governing\n", len(allInvariants))
	fmt.Printf("  \033[2mAgent:\033[0m %s\n\n", implementAgent)

	if !implementAuto {
		fmt.Printf("  \033[1mProceed? [Y/n] \033[0m")
		var answer string
		fmt.Scanln(&answer)
		if answer == "n" || answer == "N" {
			fmt.Println("  Cancelled.")
			return nil
		}
	}

	fmt.Printf("  \033[36m⟳ Spawning agent...\033[0m\n\n")

	// Execute via agent
	var agentCmd *exec.Cmd
	switch implementAgent {
	case "codex":
		agentCmd = exec.Command("codex", "exec", "--full-auto", "-c", "mcp_servers={}", prompt)
	case "claude":
		agentCmd = exec.Command("claude", "-p", prompt, "--allowedTools", "Edit,Write,Bash,Read,Glob,Grep")
	default:
		return fmt.Errorf("unknown agent: %s (use codex or claude)", implementAgent)
	}

	agentCmd.Dir = projectRoot
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr
	agentCmd.Stdin = os.Stdin

	if err := agentCmd.Run(); err != nil {
		fmt.Printf("\n  \033[31m✗ Agent exited with error: %v\033[0m\n", err)
		return nil
	}

	fmt.Printf("\n  \033[32m✓ Agent completed\033[0m\n")

	// Post-execution: baseline
	if len(affectedFiles) > 0 {
		fmt.Printf("  \033[2m⟳ Taking baseline...\033[0m\n")
		_, blErr := artifact.Baseline(ctx, artStore, projectRoot, artifact.BaselineInput{
			DecisionRef: decisionRef,
		})
		if blErr != nil {
			fmt.Printf("  \033[33m⚠ Baseline failed: %v\033[0m\n", blErr)
		} else {
			fmt.Printf("  \033[32m✓ Baseline updated\033[0m\n")
		}
	}

	fmt.Printf("\n  \033[1mNext:\033[0m\n")
	fmt.Printf("  • git diff\n")
	fmt.Printf("  • haft check\n")
	fmt.Printf("  • git commit + push\n\n")

	return nil
}

func buildRunPrompt(
	decision *artifact.Artifact,
	files []artifact.AffectedFile,
	invariants []string,
	projectRoot string,
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

	_, _ = b.WriteString("## Instructions\n\n")
	_, _ = b.WriteString("1. Read spec/AGENT_CONTRACT.md for architecture rules.\n")
	_, _ = b.WriteString("2. Inspect current code before editing — keep changes scoped.\n")
	_, _ = b.WriteString("3. Preserve every invariant listed above.\n")
	_, _ = b.WriteString("4. After implementation, verify each post-condition.\n")
	_, _ = b.WriteString("5. Run `go build ./cmd/haft/` and tests to verify.\n")
	_, _ = b.WriteString("6. Commit with conventional commit message.\n")

	return b.String()
}
