package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTaskOutputBufferKeepsNewestLongSingleLine(t *testing.T) {
	buffer := newTaskOutputBuffer(taskOutputMaxLines, "")
	head := "STARTMARKER"
	tail := strings.Repeat("tail", 2000) + "ENDMARKER"
	body := strings.Repeat("H", taskOutputMaxChars)
	longLine := head + body + tail

	got := buffer.Append(longLine)

	if utf8.RuneCountInString(got) > taskOutputMaxChars {
		t.Fatalf("expected output <= %d runes, got %d", taskOutputMaxChars, utf8.RuneCountInString(got))
	}

	if strings.Contains(got, "STARTMARKER") {
		t.Fatalf("expected oldest prefix marker to be trimmed from output")
	}

	if !strings.HasSuffix(got, "ENDMARKER") {
		t.Fatalf("expected newest output tail to be preserved, got suffix %q", got[maxInt(len(got)-32, 0):])
	}
}

func TestNormalizeTaskOutputKeepsNewestLines(t *testing.T) {
	lines := make([]string, 0, taskOutputMaxLines+25)

	for i := range taskOutputMaxLines + 25 {
		lines = append(lines, fmt.Sprintf("line-%03d", i))
	}

	output := strings.Join(lines, "\n")
	got := normalizeTaskOutput(output)
	gotLines := strings.Split(got, "\n")

	if len(gotLines) != taskOutputMaxLines {
		t.Fatalf("expected %d lines after normalization, got %d", taskOutputMaxLines, len(gotLines))
	}

	if gotLines[0] != "line-025" {
		t.Fatalf("expected first retained line line-025, got %q", gotLines[0])
	}

	if gotLines[len(gotLines)-1] != "line-524" {
		t.Fatalf("expected last retained line line-524, got %q", gotLines[len(gotLines)-1])
	}
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}

	return b
}

func TestDecisionFeatureBranchNameDefaultsToFeatureSlug(t *testing.T) {
	branch := decisionFeatureBranchName("", "Runtime foundation", "dec-001")

	if branch != "feat/runtime-foundation" {
		t.Fatalf("branch = %q, want %q", branch, "feat/runtime-foundation")
	}
}

func TestImplementDecisionCreatesFeatureWorktree(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'stub claude agent\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Runtime foundation problem",
		Signal:      "Implement needs an isolated branch per decision.",
		Acceptance:  "Decision implementation creates a dedicated worktree.",
		BlastRadius: "Desktop implementation flow only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Runtime foundation",
		WhySelected:     "A dedicated worktree keeps decision execution isolated.",
		SelectionPolicy: "Prefer the smallest reversible execution step.",
		CounterArgument: "A shared working tree is simpler when only one task runs.",
		WeakestLink:     "Git worktree setup can fail when the repository is not initialized.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Reuse the main checkout",
				Reason:  "It leaks decision implementation changes into the shared workspace.",
			},
		},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Worktree setup proves unreliable for routine decision execution.",
			},
		},
		Mode: "tactical",
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	task, err := app.ImplementDecision(decision.ID, "claude", false, "")
	if err != nil {
		t.Fatalf("ImplementDecision: %v", err)
	}

	expectedBranch := "feat/runtime-foundation"
	expectedWorktree := filepath.Join(app.projectRoot, ".haft", "worktrees", filepath.FromSlash(expectedBranch))

	if !task.Worktree {
		t.Fatal("expected ImplementDecision to always use a worktree")
	}

	if task.Branch != expectedBranch {
		t.Fatalf("Branch = %q, want %q", task.Branch, expectedBranch)
	}

	if task.WorktreePath != expectedWorktree {
		t.Fatalf("WorktreePath = %q, want %q", task.WorktreePath, expectedWorktree)
	}

	if _, err := os.Stat(expectedWorktree); err != nil {
		t.Fatalf("Stat worktree: %v", err)
	}

	branchOutput, err := exec.Command("git", "-C", expectedWorktree, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse worktree branch: %v (%s)", err, strings.TrimSpace(string(branchOutput)))
	}

	if got := strings.TrimSpace(string(branchOutput)); got != expectedBranch {
		t.Fatalf("worktree branch = %q, want %q", got, expectedBranch)
	}

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "completed" {
		t.Fatalf("task status = %q, want completed", final.Status)
	}
}

func installStubAgentBinary(t *testing.T, name string, body string) {
	t.Helper()

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll bin dir: %v", err)
	}

	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func initTestGitRepository(t *testing.T, root string) {
	t.Helper()

	readmePath := filepath.Join(root, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test repo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README.md: %v", err)
	}

	runTestCommand(t, root, "git", "init")
	runTestCommand(t, root, "git", "config", "user.email", "tests@example.com")
	runTestCommand(t, root, "git", "config", "user.name", "Desktop Tests")
	runTestCommand(t, root, "git", "add", "README.md", ".haft")
	runTestCommand(t, root, "git", "commit", "-m", "initial commit")
}

func runTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func waitForTaskState(t *testing.T, app *App, taskID string) TaskState {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		state, err := app.loadTaskState(taskID)
		if err == nil && state.Status != "running" {
			return *state
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("task %s did not finish before timeout", taskID)
	return TaskState{}
}
