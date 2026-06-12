package overseer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitContext struct {
	CreatedAt    string
	Subject      Subject
	RepoState    RepoState
	ChangedFiles []ChangedFile
}

type GitRunner interface {
	RunGit(ctx context.Context, args ...string) (string, error)
}

type CommandGitRunner struct {
	ProjectRoot string
}

func (runner CommandGitRunner) RunGit(ctx context.Context, args ...string) (string, error) {
	gitArgs := append([]string{"-C", runner.ProjectRoot}, args...)
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func CollectGitContext(ctx context.Context, projectRoot string, commitRef string) (GitContext, error) {
	return CollectGitContextWithRunner(ctx, CommandGitRunner{ProjectRoot: projectRoot}, projectRoot, commitRef)
}

func CollectGitContextWithRunner(
	ctx context.Context,
	runner GitRunner,
	projectRoot string,
	commitRef string,
) (GitContext, error) {
	ref := strings.TrimSpace(commitRef)
	if ref == "" {
		ref = "HEAD"
	}

	sha, err := runner.RunGit(ctx, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return GitContext{}, fmt.Errorf("resolve commit %q: %w", ref, err)
	}

	parentSHA, _ := runner.RunGit(ctx, "rev-parse", "--verify", sha+"^")
	diffOutput, err := diffTree(ctx, runner, sha, "--name-status")
	if err != nil {
		return GitContext{}, err
	}

	numstatOutput, err := diffTree(ctx, runner, sha, "--numstat")
	if err != nil {
		return GitContext{}, err
	}

	patchOutput, err := diffTree(ctx, runner, sha, "--patch")
	if err != nil {
		return GitContext{}, err
	}

	createdAt, _ := runner.RunGit(ctx, "show", "-s", "--format=%cI", sha)
	branch, _ := runner.RunGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	status, _ := runner.RunGit(ctx, "status", "--porcelain")

	diffHash := sha256.Sum256([]byte(patchOutput))
	changedFiles := parseChangedFiles(diffOutput, parseNumstat(numstatOutput))

	return GitContext{
		CreatedAt: createdAt,
		Subject: Subject{
			Kind:      "commit",
			Ref:       ref,
			SHA:       sha,
			ParentSHA: parentSHA,
			DiffHash:  "sha256:" + hex.EncodeToString(diffHash[:]),
		},
		RepoState: RepoState{
			GitRoot:                  ".",
			Branch:                   branch,
			WorktreeDirtyAfterCommit: strings.TrimSpace(status) != "",
			UntrackedFilesCount:      countUntracked(status),
		},
		ChangedFiles: attachDiffRefs(changedFiles, projectRoot),
	}, nil
}

func diffTree(ctx context.Context, runner GitRunner, sha string, mode string) (string, error) {
	output, err := runner.RunGit(ctx, "diff-tree", "--root", "--no-commit-id", "-r", "-m", mode, sha)
	if err != nil {
		return "", fmt.Errorf("read commit diff %s: %w", mode, err)
	}
	return output, nil
}

func parseChangedFiles(nameStatusOutput string, stats map[string]DiffStats) []ChangedFile {
	lines := strings.Split(strings.TrimSpace(nameStatusOutput), "\n")
	files := make([]ChangedFile, 0, len(lines))

	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		status := fields[0]
		path := fields[len(fields)-1]
		stat := stats[path]

		files = append(files, ChangedFile{
			Path:      normalizePath(path),
			Status:    normalizeGitStatus(status),
			Language:  languageForPath(path),
			DiffStats: stat,
			Governance: ChangedFileGovernance{
				ModuleState: "blind",
			},
		})
	}

	return files
}

func parseNumstat(output string) map[string]DiffStats {
	stats := make(map[string]DiffStats)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}

		path := fields[len(fields)-1]
		stats[normalizePath(path)] = DiffStats{
			Added:   parseGitNumstat(fields[0]),
			Deleted: parseGitNumstat(fields[1]),
		}
	}

	return stats
}

func parseGitNumstat(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func normalizeGitStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return ""
	}
	switch trimmed[0] {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'M':
		return "modified"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	default:
		return strings.ToLower(trimmed)
	}
}

func languageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	default:
		return ""
	}
}

func attachDiffRefs(files []ChangedFile, projectRoot string) []ChangedFile {
	for index := range files {
		path := files[index].Path
		files[index].InlineDiffRef = "diff://" + path + "#compact"
		files[index].FullDiffHandle = "haft://packet/diff/" + path
	}
	return files
}

func countUntracked(status string) int {
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if strings.HasPrefix(line, "??") {
			count++
		}
	}
	return count
}
