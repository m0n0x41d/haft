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
	hookPath, ok, err := resolvePostCommitHookPath(projectRoot)
	result := HookInstallResult{HookPath: hookPath}
	if err != nil {
		return result, err
	}
	if !ok {
		result.Skipped = true
		result.Reason = "no .git directory"
		return result, nil
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

func PostCommitHookPath(projectRoot string) (string, bool, error) {
	return resolvePostCommitHookPath(projectRoot)
}

func resolvePostCommitHookPath(projectRoot string) (string, bool, error) {
	gitPath := filepath.Join(projectRoot, ".git")
	defaultHookPath := filepath.Join(gitPath, "hooks", "post-commit")

	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultHookPath, false, nil
		}
		return defaultHookPath, false, fmt.Errorf("inspect .git: %w", err)
	}
	if info.IsDir() {
		return defaultHookPath, true, nil
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return defaultHookPath, false, fmt.Errorf("read .git file: %w", err)
	}
	gitDir, ok := parseGitDirFile(string(data))
	if !ok {
		return defaultHookPath, false, fmt.Errorf("parse .git file: missing gitdir")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(projectRoot, gitDir)
	}
	return filepath.Join(filepath.Clean(gitDir), "hooks", "post-commit"), true, nil
}

func parseGitDirFile(content string) (string, bool) {
	line := strings.TrimSpace(content)
	prefix := "gitdir:"
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return "", false
	}

	gitDir := strings.TrimSpace(line[len(prefix):])
	if gitDir == "" {
		return "", false
	}
	return filepath.Clean(gitDir), true
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

func RenderPostCommitHook(existing string, command string) string {
	block := RenderPostCommitHookBlock(command)
	return mergeHookBlock(existing, block)
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
