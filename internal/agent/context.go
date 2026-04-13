package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LoadProjectContext gathers project-level context for the system prompt.
// Reads CLAUDE.md, .haft/ files, git info. Pure I/O — called once at session start.
func LoadProjectContext(projectRoot string) string {
	var sections []string

	// Cross-standard instruction files (same as Crush, industry convention)
	// AGENTS.md is the emerging standard. We read all of them.
	instructionFiles := []string{
		"AGENTS.md",                       // industry cross-standard (preferred)
		"HAFT.md",                         // haft-specific
		"CLAUDE.md",                       // Claude Code
		".github/copilot-instructions.md", // GitHub Copilot
		".cursorrules",                    // Cursor
		"GEMINI.md",                       // Gemini
	}
	for _, name := range instructionFiles {
		content := readFileIfExists(filepath.Join(projectRoot, name))
		if content != "" {
			sections = append(sections, "## Project instructions ("+name+")\n\n"+truncateContext(content, 3000))
		}
	}

	// Git branch and status
	if gitInfo := getGitInfo(projectRoot); gitInfo != "" {
		sections = append(sections, "## Git\n\n"+gitInfo)
	}

	// .haft/ summary (existing decisions count)
	if haftSummary := getQuintSummary(projectRoot); haftSummary != "" {
		sections = append(sections, "## Project decisions\n\n"+haftSummary)
	}

	if len(sections) == 0 {
		return ""
	}

	return "\n\n# Project Context\n\n" + strings.Join(sections, "\n\n")
}

func readFileIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func getGitInfo(root string) string {
	branch := runGitCmd(root, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" {
		return ""
	}

	status := runGitCmd(root, "status", "--porcelain", "--short")
	changedCount := 0
	if status != "" {
		changedCount = strings.Count(status, "\n")
		if !strings.HasSuffix(status, "\n") {
			changedCount++
		}
	}

	result := "Branch: " + branch
	if changedCount > 0 {
		result += "\nUncommitted changes: " + strings.SplitN(status, "\n", 6)[0]
		if changedCount > 5 {
			result += "\n... and more"
		}
	}
	return result
}

func getQuintSummary(root string) string {
	haftDir := filepath.Join(root, ".haft")
	if _, err := os.Stat(haftDir); err != nil {
		return ""
	}

	decisions, _ := filepath.Glob(filepath.Join(haftDir, "decisions", "*.md"))
	problems, _ := filepath.Glob(filepath.Join(haftDir, "problems", "*.md"))

	if len(decisions) == 0 && len(problems) == 0 {
		return ""
	}

	return fmt.Sprintf("Decisions: %d | Problems: %d\nUse haft_query(action='status') to see details.",
		len(decisions), len(problems))
}

func runGitCmd(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func truncateContext(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
