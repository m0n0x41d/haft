package overseer

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCollectGitContextRequestsMergeDiffs(t *testing.T) {
	runner := &recordingGitRunner{}

	ctx, err := CollectGitContextWithRunner(context.Background(), runner, ".", "HEAD")
	if err != nil {
		t.Fatalf("CollectGitContextWithRunner returned error: %v", err)
	}

	if got := len(ctx.ChangedFiles); got != 1 {
		t.Fatalf("changed files = %d, want 1", got)
	}
	for _, call := range runner.diffTreeCalls {
		if !containsString(call, "-m") {
			t.Fatalf("diff-tree call missing -m: %v", call)
		}
	}
}

type recordingGitRunner struct {
	diffTreeCalls [][]string
}

func (runner *recordingGitRunner) RunGit(_ context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("missing git args")
	}

	if args[0] == "diff-tree" {
		call := append([]string(nil), args...)
		runner.diffTreeCalls = append(runner.diffTreeCalls, call)
		return diffTreeOutputFor(args), nil
	}
	if args[0] == "rev-parse" && containsString(args, "--verify") {
		if strings.HasSuffix(args[len(args)-1], "^{commit}") {
			return "merge-sha", nil
		}
		return "parent-sha", nil
	}
	if args[0] == "show" {
		return "2026-06-12T00:00:00Z", nil
	}
	if args[0] == "rev-parse" && containsString(args, "--abbrev-ref") {
		return "main", nil
	}
	if args[0] == "status" {
		return "", nil
	}
	return "", fmt.Errorf("unexpected git args: %v", args)
}

func diffTreeOutputFor(args []string) string {
	if containsString(args, "--name-status") {
		return "M\tinternal/overseer/context.go"
	}
	if containsString(args, "--numstat") {
		return "2\t1\tinternal/overseer/context.go"
	}
	if containsString(args, "--patch") {
		return "diff --git a/internal/overseer/context.go b/internal/overseer/context.go"
	}
	return ""
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
