package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

// implementationStep represents one unit of work in the lemniscate loop.
type implementationStep struct {
	index       int
	total       int
	title       string
	files       []string
	invariants  []string // invariants relevant to this step
	prompt      string
}

// decomposeDecision breaks a DecisionRecord into implementation steps.
// Strategy: one step per affected file (or group of related files).
// Each step carries only the invariants relevant to its files.
func decomposeDecision(
	decision *artifact.Artifact,
	files []artifact.AffectedFile,
	graphStore *graph.Store,
	projectRoot string,
	contextFiles []string,
	extraPrompt string,
) []implementationStep {
	if len(files) == 0 {
		// No affected files — single step with full decision
		return []implementationStep{{
			index: 1,
			total: 1,
			title: decision.Meta.Title,
			prompt: buildRunPrompt(decision, files, nil, projectRoot, contextFiles, extraPrompt),
		}}
	}

	// Group files by directory (module boundary)
	groups := groupFilesByDir(files)

	steps := make([]implementationStep, 0, len(groups))
	for i, group := range groups {
		// Find invariants specific to these files
		var stepInvariants []string
		for _, f := range group.files {
			invs, err := graphStore.FindInvariantsForFile(nil, f)
			if err == nil {
				for _, inv := range invs {
					stepInvariants = append(stepInvariants, fmt.Sprintf("[%s] %s", inv.DecisionID, inv.Text))
				}
			}
		}
		stepInvariants = uniqueStrings(stepInvariants)

		// Build scoped affected files
		var scopedFiles []artifact.AffectedFile
		for _, f := range group.files {
			scopedFiles = append(scopedFiles, artifact.AffectedFile{Path: f})
		}

		step := implementationStep{
			index:      i + 1,
			total:      len(groups),
			title:      fmt.Sprintf("%s — %s", decision.Meta.Title, group.dir),
			files:      group.files,
			invariants: stepInvariants,
			prompt:     buildStepPrompt(decision, i+1, len(groups), group.dir, scopedFiles, stepInvariants, projectRoot, contextFiles, extraPrompt),
		}
		steps = append(steps, step)
	}

	return steps
}

type fileGroup struct {
	dir   string
	files []string
}

func groupFilesByDir(files []artifact.AffectedFile) []fileGroup {
	dirMap := map[string][]string{}
	var dirOrder []string

	for _, f := range files {
		dir := f.Path
		if idx := strings.LastIndex(f.Path, "/"); idx >= 0 {
			dir = f.Path[:idx]
		}
		if _, exists := dirMap[dir]; !exists {
			dirOrder = append(dirOrder, dir)
		}
		dirMap[dir] = append(dirMap[dir], f.Path)
	}

	groups := make([]fileGroup, 0, len(dirOrder))
	for _, dir := range dirOrder {
		groups = append(groups, fileGroup{dir: dir, files: dirMap[dir]})
	}
	return groups
}

func buildStepPrompt(
	decision *artifact.Artifact,
	stepNum, totalSteps int,
	scopeDir string,
	files []artifact.AffectedFile,
	invariants []string,
	projectRoot string,
	contextFiles []string,
	extraPrompt string,
) string {
	var b strings.Builder

	_, _ = b.WriteString(fmt.Sprintf("# Step %d/%d: %s\n\n", stepNum, totalSteps, decision.Meta.Title))
	_, _ = b.WriteString(fmt.Sprintf("Decision ID: %s\n", decision.Meta.ID))
	_, _ = b.WriteString(fmt.Sprintf("Scope: %s\n\n", scopeDir))

	_, _ = b.WriteString("## Decision Context\n\n")
	_, _ = b.WriteString(decision.Body)
	_, _ = b.WriteString("\n\n")

	_, _ = b.WriteString(fmt.Sprintf("## Scope for This Step (%s)\n\n", scopeDir))
	_, _ = b.WriteString("Focus ONLY on these files:\n")
	for _, f := range files {
		_, _ = b.WriteString(fmt.Sprintf("- %s\n", f.Path))
	}
	_, _ = b.WriteString("\nDo NOT modify files outside this scope.\n\n")

	if len(invariants) > 0 {
		_, _ = b.WriteString("## Invariants for This Step (must hold)\n\n")
		for _, inv := range invariants {
			_, _ = b.WriteString(fmt.Sprintf("- %s\n", inv))
		}
		_, _ = b.WriteString("\n")
	}

	// Include context files and extra prompt
	for _, cf := range contextFiles {
		absPath := cf
		if !strings.HasPrefix(cf, "/") {
			absPath = projectRoot + "/" + cf
		}
		if data, err := os.ReadFile(absPath); err == nil {
			_, _ = b.WriteString(fmt.Sprintf("## Context: %s\n\n", cf))
			_, _ = b.Write(data)
			_, _ = b.WriteString("\n\n")
		}
	}

	if extraPrompt != "" {
		_, _ = b.WriteString("## Additional Instructions\n\n")
		_, _ = b.WriteString(extraPrompt)
		_, _ = b.WriteString("\n\n")
	}

	_, _ = b.WriteString("## Instructions\n\n")
	_, _ = b.WriteString(fmt.Sprintf("This is step %d of %d. Stay within scope.\n", stepNum, totalSteps))
	_, _ = b.WriteString("1. Read the scoped files before editing.\n")
	_, _ = b.WriteString("2. Preserve every invariant listed above.\n")
	_, _ = b.WriteString("3. Run `go build ./cmd/haft/` to verify compilation.\n")
	_, _ = b.WriteString("4. Commit with conventional commit message.\n")

	return b.String()
}

// executeStep runs one step of the lemniscate loop.
// Returns true if invariants pass, false if violated.
func executeStep(
	step implementationStep,
	agent string,
	projectRoot string,
	graphStore *graph.Store,
	decisionRef string,
	ui *runUI,
	maxRetries int,
) bool {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			ui.phase(fmt.Sprintf("Step %d/%d — Retry %d", step.index, step.total, attempt))
		} else {
			ui.phase(fmt.Sprintf("Step %d/%d: %s", step.index, step.total, step.title))
		}

		prompt := step.prompt
		if attempt > 0 {
			// On retry, use fix prompt instead
			prompt = step.prompt // will be overridden below
		}

		// Run agent
		if err := spawnAgent(agent, prompt, projectRoot); err != nil {
			ui.fail(fmt.Sprintf("Agent error: %v", err))
			return false
		}

		// Verify invariants for this step
		results, err := graph.VerifyInvariants(nil, graphStore, nil, decisionRef)
		if err != nil {
			ui.warn(fmt.Sprintf("Verification error: %v — skipping check", err))
			return true // can't verify, proceed optimistically
		}

		// Check only invariants relevant to this step
		allPass := true
		var violations []graph.InvariantResult
		for _, r := range results {
			invKey := fmt.Sprintf("[%s] %s", r.Invariant.DecisionID, r.Invariant.Text)
			relevant := false
			for _, si := range step.invariants {
				if si == invKey {
					relevant = true
					break
				}
			}
			if !relevant {
				continue
			}

			switch r.Status {
			case graph.InvariantHolds:
				ui.invariantResult(r.Invariant.DecisionID, r.Invariant.Text, true)
			case graph.InvariantViolated:
				allPass = false
				violations = append(violations, r)
				ui.invariantResult(r.Invariant.DecisionID, r.Invariant.Text, false)
				fmt.Printf("       %s%s%s\n", aRed, r.Reason, aReset)
			case graph.InvariantUnknown:
				fmt.Printf("  %s?%s %s%s%s\n", aYellow, aReset, aDim, r.Invariant.Text, aReset)
			}
		}

		if allPass {
			ui.ok(fmt.Sprintf("Step %d passed", step.index))
			return true
		}

		if attempt < maxRetries {
			ui.warn(fmt.Sprintf("Step %d failed — retrying with fix prompt (%d/%d)", step.index, attempt+1, maxRetries))
			// Rebuild prompt with violation context
			step.prompt = buildFixPrompt(step, violations)
		}
	}

	ui.fail(fmt.Sprintf("Step %d failed after %d retries", step.index, maxRetries))
	return false
}

func buildFixPrompt(step implementationStep, violations []graph.InvariantResult) string {
	var b strings.Builder
	_, _ = b.WriteString(fmt.Sprintf("# Fix Invariant Violations — Step %d\n\n", step.index))
	_, _ = b.WriteString("The previous implementation attempt violated these invariants:\n\n")
	for _, v := range violations {
		_, _ = b.WriteString(fmt.Sprintf("- VIOLATED: %s\n  Reason: %s\n\n", v.Invariant.Text, v.Reason))
	}
	_, _ = b.WriteString("Fix the violations while preserving the implementation intent.\n")
	_, _ = b.WriteString("Focus only on these files:\n")
	for _, f := range step.files {
		_, _ = b.WriteString(fmt.Sprintf("- %s\n", f))
	}
	return b.String()
}

func spawnAgent(agent, prompt, projectRoot string) error {
	var cmd *exec.Cmd
	switch agent {
	case "codex":
		cmd = exec.Command("codex", "exec", "--full-auto", "-c", "mcp_servers={}", "-")
	case "claude":
		cmd = exec.Command("claude", "-p", prompt, "--allowedTools", "Edit,Write,Bash,Read,Glob,Grep")
	default:
		return fmt.Errorf("unknown agent: %s", agent)
	}

	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if agent == "codex" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		_, _ = stdin.Write([]byte(prompt))
		_ = stdin.Close()
		return cmd.Wait()
	}

	cmd.Stdin = os.Stdin
	return cmd.Run()
}
