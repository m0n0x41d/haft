package overseer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	hookStartMarker = "# BEGIN HAFT OVERSEER"
	hookEndMarker   = "# END HAFT OVERSEER"
)

type HookInstallResult struct {
	HookPath  string
	Installed bool
	Updated   bool
	Skipped   bool
	Reason    string
}

func InstallPostCommitHook(projectRoot string, command string) (HookInstallResult, error) {
	hookPath := filepath.Join(projectRoot, ".git", "hooks", "post-commit")
	result := HookInstallResult{HookPath: hookPath}

	if _, err := os.Stat(filepath.Join(projectRoot, ".git")); err != nil {
		if os.IsNotExist(err) {
			result.Skipped = true
			result.Reason = "no .git directory"
			return result, nil
		}
		return result, fmt.Errorf("inspect .git directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return result, fmt.Errorf("create hooks dir: %w", err)
	}

	existingBytes, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return result, fmt.Errorf("read post-commit hook: %w", err)
	}

	existing := string(existingBytes)
	block := RenderPostCommitHookBlock(command)
	next := mergeHookBlock(existing, block)
	if existing == next {
		result.Installed = true
		return result, nil
	}

	if err := os.WriteFile(hookPath, []byte(next), 0o755); err != nil {
		return result, fmt.Errorf("write post-commit hook: %w", err)
	}

	result.Installed = true
	result.Updated = existing != ""
	return result, nil
}

func RenderPostCommitHookBlock(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		command = DefaultToolName
	}

	return strings.Join([]string{
		hookStartMarker,
		"# Soft carrier only: never blocks the commit and never creates binding authority.",
		fmt.Sprintf("if command -v %s >/dev/null 2>&1; then", shellWord(command)),
		fmt.Sprintf("  HAFT_PROJECT_ROOT=\"$(git rev-parse --show-toplevel 2>/dev/null || pwd)\" %s overseer hook --commit HEAD --async || true", shellWord(command)),
		"fi",
		hookEndMarker,
		"",
	}, "\n")
}

func mergeHookBlock(existing string, block string) string {
	trimmed := strings.TrimRight(existing, " \t\n")
	if trimmed == "" {
		return "#!/bin/sh\n\n" + block
	}

	start := strings.Index(trimmed, hookStartMarker)
	end := strings.Index(trimmed, hookEndMarker)
	if start >= 0 && end > start {
		end += len(hookEndMarker)
		merged := strings.TrimRight(trimmed[:start], " \t\n")
		trailing := strings.TrimLeft(trimmed[end:], " \t\n")
		parts := []string{}
		if merged != "" {
			parts = append(parts, merged)
		}
		parts = append(parts, strings.TrimRight(block, "\n"))
		if trailing != "" {
			parts = append(parts, trailing)
		}
		return strings.Join(parts, "\n\n") + "\n"
	}

	return trimmed + "\n\n" + block
}

func shellWord(command string) string {
	if command != "" && strings.IndexFunc(command, func(r rune) bool {
		return r != '/' && r != '-' && r != '_' && r != '.' && r != ':' && r != '+' && r != '=' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) == -1 {
		return command
	}
	return "'" + strings.ReplaceAll(command, "'", "'\"'\"'") + "'"
}
