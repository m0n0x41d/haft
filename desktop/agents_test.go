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

	"github.com/m0n0x41d/haft/internal/artifact"
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

func TestBuildAgentArgsCodexUsesWorktreeApprovalGate(t *testing.T) {
	worktree := "/tmp/haft/worktree"
	args := buildAgentArgs(AgentCodex, "inspect the task", worktree)

	if len(args) == 0 {
		t.Fatal("expected codex args")
	}

	if strings.Join(args, " ") == "" {
		t.Fatal("expected non-empty codex args")
	}

	if !containsArgs(args, "--cd", worktree) {
		t.Fatalf("expected codex args to include worktree path, got %v", args)
	}

	if !containsArgs(args, "--ask-for-approval", "untrusted") {
		t.Fatalf("expected codex args to include checkpointed approval gate, got %v", args)
	}

	if containsArg(args, "--full-auto") {
		t.Fatalf("expected codex checkpointed args to avoid --full-auto, got %v", args)
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

func TestImplementDecisionPromptIncludesPortfolioWorkflowAndGraphContext(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'stub claude agent\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	workflowMarkdown := strings.Join([]string{
		"# Workflow",
		"",
		"## Intent",
		"",
		"Keep implementation prompts policy-complete for governed files.",
		"",
		"## Defaults",
		"",
		"```yaml",
		"mode: standard",
		"require_decision: true",
		"require_verify: true",
		"allow_autonomy: false",
		"```",
	}, "\n")

	if err := os.WriteFile(filepath.Join(app.projectRoot, ".haft", "workflow.md"), []byte(workflowMarkdown), 0o644); err != nil {
		t.Fatalf("WriteFile workflow.md: %v", err)
	}

	governingProblem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Workflow governance problem",
		Signal:      "A governing invariant should reach implementation prompts for shared files.",
		Acceptance:  "Implement sees cross-decision invariants.",
		BlastRadius: "Desktop implementation prompt only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem governing: %v", err)
	}

	governingDecision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      governingProblem.ID,
		SelectedTitle:   "Protect workflow policy propagation",
		WhySelected:     "Shared files should inherit governing invariants.",
		SelectionPolicy: "Prefer the narrowest governing rule that still protects shared implementation context.",
		CounterArgument: "Keeping this invariant local to one decision avoids prompt growth for unrelated work.",
		WeakestLink:     "A shared-file invariant can overreach when the affected file list is too broad.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Keep the invariant private to one decision",
				Reason:  "That leaves later implementation prompts blind to a governing constraint on the same file.",
			},
		},
		Invariants: []string{
			"Keep workflow policy embedded verbatim.",
		},
		AffectedFiles: []string{"README.md"},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Shared-file invariants prove too noisy for routine implementation prompts.",
			},
		},
		Mode: "tactical",
	})
	if err != nil {
		t.Fatalf("CreateDecision governing: %v", err)
	}

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Prompt context gap",
		Signal:      "Implement prompts miss portfolio and workflow guidance.",
		Acceptance:  "Implement prompts include decision, portfolio, workflow, and governing invariant context.",
		BlastRadius: "Desktop implementation prompt only",
		Mode:        "standard",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	portfolio, err := app.CreatePortfolio(PortfolioCreateInput{
		ProblemRef: problem.ID,
		Variants: []PortfolioVariantInput{
			{
				ID:                 "var-1",
				Title:              "Policy-complete prompt",
				Description:        "Inject decision, workflow, and knowledge-graph context into the implement prompt.",
				WeakestLink:        "Prompt bloat can bury the core task if the sections are not ordered tightly.",
				NoveltyMarker:      "Keeps the implementation context decision-anchored and policy-complete.",
				SteppingStone:      true,
				SteppingStoneBasis: "Adds the missing context without changing task execution or verification semantics.",
			},
			{
				ID:            "var-2",
				Title:         "Decision-only prompt",
				Description:   "Send only the current decision body and let the operator fill the gaps manually.",
				WeakestLink:   "The agent can violate neighboring invariants and workflow policy.",
				NoveltyMarker: "Minimizes prompt size at the cost of missing governance context.",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}

	_, err = app.ComparePortfolio(PortfolioCompareInput{
		PortfolioRef: portfolio.ID,
		Dimensions:   []string{"operator confidence"},
		Scores: map[string]map[string]string{
			"var-1": {"operator confidence": "High"},
			"var-2": {"operator confidence": "Low"},
		},
		PolicyApplied: "Prefer the option that makes governance context explicit before editing.",
		SelectedRef:   "var-1",
		DominatedNotes: []DominatedNoteInput{
			{
				Variant:     "var-2",
				DominatedBy: []string{"var-1"},
				Summary:     "A decision-only prompt leaves the agent blind to workflow policy and neighboring decisions.",
			},
		},
		ParetoTradeoffs: []TradeoffNoteInput{
			{
				Variant: "var-1",
				Summary: "Carries more context, which slightly increases prompt size, but keeps governance visible before edits start.",
			},
			{
				Variant: "var-2",
				Summary: "Keeps the prompt short, but hides workflow and cross-decision constraints from the agent.",
			},
		},
		Recommendation: "Prefer the prompt that includes workflow policy and governing invariants before implementation starts.",
	})
	if err != nil {
		t.Fatalf("ComparePortfolio: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		PortfolioRef:    portfolio.ID,
		SelectedRef:     "var-1",
		WhySelected:     "The implementation prompt should stay complete even if the portfolio rationale is unavailable.",
		SelectionPolicy: "Prefer the smallest reversible prompt expansion.",
		CounterArgument: "Loading workflow and graph context can make the implementation prompt heavier than necessary.",
		WeakestLink:     "Prompt structure can decay if the extra context is appended without clear sections.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Decision-only prompt",
				Reason:  "It omits workflow policy and governing invariants that still apply to the same files.",
			},
		},
		Invariants: []string{
			"Keep implementation prompt assembly pure.",
		},
		AffectedFiles: []string{"README.md"},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"The extra context materially slows agents without improving implementation quality.",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	task, err := app.ImplementDecision(decision.ID, "claude", false, "")
	if err != nil {
		t.Fatalf("ImplementDecision: %v", err)
	}

	expectedSnippets := []string{
		"## Solution Portfolio Rationale",
		"Prefer the prompt that includes workflow policy and governing invariants before implementation starts.",
		"## Workflow Policy (.haft/workflow.md)",
		"Keep implementation prompts policy-complete for governed files.",
		"## Governing Invariants (knowledge graph)",
		fmt.Sprintf("[%s] Keep workflow policy embedded verbatim.", governingDecision.ID),
		"## Affected Files",
		"- README.md",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(task.Prompt, snippet) {
			t.Fatalf("implementation prompt missing %q:\n%s", snippet, task.Prompt)
		}
	}

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "Ready for PR" {
		t.Fatalf("task status = %q, want Ready for PR", final.Status)
	}
}

func TestImplementDecisionWaitsForReviewUntilAutoRunEnabled(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'cwd=%s\\n' \"$PWD\"\nprintf 'waiting-for-review\\n'\nread answer\nprintf 'approved=%s\\n' \"$answer\"\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Checkpointed review problem",
		Signal:      "Implement should wait for operator review between tool steps.",
		Acceptance:  "The desktop task pauses for review and resumes in the worktree when auto-run is enabled.",
		BlastRadius: "Desktop implementation flow only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Review gated worktree",
		WhySelected:     "The operator needs a review checkpoint inside the implementation task.",
		SelectionPolicy: "Prefer the smallest launcher change that keeps the task reviewable.",
		CounterArgument: "A fully automatic launcher is simpler to wire.",
		WeakestLink:     "The agent backend must honor stdin-driven review prompts.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Full auto",
				Reason:  "It skips the review checkpoint required by the execution contract.",
			},
		},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Checkpointed desktop tasks become unreliable across supported agent backends.",
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

	if task.AutoRun {
		t.Fatal("expected implement task to start checkpointed")
	}

	expectedWorktree := filepath.Join(app.projectRoot, ".haft", "worktrees", "feat/review-gated-worktree")
	liveOutput := waitForTaskOutputContains(t, app, task.ID, "waiting-for-review")

	if !strings.Contains(liveOutput, expectedWorktree) {
		t.Fatalf("expected live task output to stream the worktree cwd %q, got %q", expectedWorktree, liveOutput)
	}

	liveState, err := app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("task status = %q, want running while awaiting review", liveState.Status)
	}

	if err := app.SetTaskAutoRun(task.ID, true); err != nil {
		t.Fatalf("SetTaskAutoRun: %v", err)
	}

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "completed" {
		t.Fatalf("task status = %q, want completed", final.Status)
	}

	if !strings.Contains(final.Output, "approved=y") {
		t.Fatalf("expected approval marker in final output, got %q", final.Output)
	}
}

func TestImplementDecisionRecordsVerificationPassOnSuccess(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'verification task complete\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Verification pass problem",
		Signal:      "Successful implement tasks should capture a baseline and evidence before PR handoff.",
		Acceptance:  "A completed verification pass marks the task ready for PR and records CL3 evidence.",
		BlastRadius: "Desktop implement flow only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Record verification pass",
		WhySelected:     "The desktop flow needs a concrete close-the-loop step after implementation.",
		SelectionPolicy: "Prefer a minimal reversible post-run hook.",
		CounterArgument: "Leaving the task as completed is simpler and avoids another persistence step.",
		WeakestLink:     "A false-positive verification pass would create misleading evidence.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Manual baseline after task completion",
				Reason:  "It leaves the happy path incomplete and makes the PR handoff easy to forget.",
			},
		},
		Invariants: []string{
			"Verification evidence is recorded only after a successful implement task.",
		},
		AffectedFiles: []string{"README.md"},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Desktop verification evidence proves noisy or misleading in regular implement flows.",
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

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "Ready for PR" {
		t.Fatalf("task status = %q, want Ready for PR", final.Status)
	}
	if !strings.Contains(final.Output, "Post-execution verification passed") {
		t.Fatalf("final output missing verification note: %q", final.Output)
	}

	files, err := app.store.GetAffectedFiles(context.Background(), decision.ID)
	if err != nil {
		t.Fatalf("GetAffectedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("affected files = %d, want 1", len(files))
	}
	if files[0].Hash == "" {
		t.Fatal("expected README.md baseline hash to be recorded")
	}

	items, err := app.store.GetEvidenceItems(context.Background(), decision.ID)
	if err != nil {
		t.Fatalf("GetEvidenceItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("evidence items = %d, want 1", len(items))
	}
	if items[0].Type != "audit" {
		t.Fatalf("evidence type = %q, want audit", items[0].Type)
	}
	if items[0].Verdict != "supports" {
		t.Fatalf("evidence verdict = %q, want supports", items[0].Verdict)
	}
	if items[0].CongruenceLevel != 3 {
		t.Fatalf("evidence congruence = %d, want 3", items[0].CongruenceLevel)
	}
	if items[0].CarrierRef != "desktop-task:"+task.ID {
		t.Fatalf("evidence carrier_ref = %q, want %q", items[0].CarrierRef, "desktop-task:"+task.ID)
	}
}

func TestBuildPullRequestBodyIncludesRationaleInvariantAndVerification(t *testing.T) {
	task := TaskState{
		ID:     "task-42",
		Title:  "Implement: Create PR action",
		Branch: "feat/create-pr-action",
	}

	detail := DecisionDetailView{
		ID:              "dec-42",
		Title:           "Create PR action",
		SelectedTitle:   "Create PR action",
		WhySelected:     "It closes the execution loop without widening the artifact model.",
		SelectionPolicy: "Prefer the smallest reversible operator action.",
		Invariants: []string{
			"Generate PR body from decision rationale and verification evidence.",
		},
	}

	verification := &artifact.EvidenceItem{
		ID:              "ev-42",
		Verdict:         "supports",
		CongruenceLevel: 3,
		Content: strings.Join([]string{
			"Desktop post-execution verification pass recorded.",
			"Decision: dec-42",
			"Baselined files (1): desktop/agents.go",
			"Task: task-42",
			"Worktree: /tmp/worktree",
		}, "\n"),
	}

	body := buildPullRequestBody(
		task,
		detail,
		"Use the stored decision rationale rather than rebuilding PR text from task output.",
		verification,
	)

	expectedSnippets := []string{
		"## Summary: Create PR action",
		"## Decision Rationale",
		"Use the stored decision rationale rather than rebuilding PR text from task output.",
		"It closes the execution loop without widening the artifact model.",
		"Selection policy: Prefer the smallest reversible operator action.",
		"## Invariants",
		"Generate PR body from decision rationale and verification evidence.",
		"## Verification Result",
		"Post-execution verification passed.",
		"Evidence: ev-42",
		"Baselined files (1): desktop/agents.go",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(body, snippet) {
			t.Fatalf("PR body missing %q:\n%s", snippet, body)
		}
	}

	if strings.Contains(body, "Worktree: /tmp/worktree") {
		t.Fatalf("PR body should not expose worktree paths:\n%s", body)
	}
}

func TestCreatePullRequestCreatesDraftForReadyTask(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'verification task complete\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}

	pushLog := filepath.Join(t.TempDir(), "git-push.log")
	ghLog := filepath.Join(t.TempDir(), "gh.log")

	installStubAgentBinary(
		t,
		"git",
		fmt.Sprintf(
			"#!/bin/sh\nif [ \"$1\" = \"push\" ]; then\n  printf '%%s\\n' \"$*\" >> %q\n  exit 0\nfi\nexec %q \"$@\"\n",
			pushLog,
			realGit,
		),
	)
	installStubAgentBinary(
		t,
		"gh",
		fmt.Sprintf(
			"#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %q\nprintf 'https://example.com/pr/123\\n'\n",
			ghLog,
		),
	)

	decision, task := createReadyForPRTask(t, app)

	result, err := app.CreatePullRequest(task.ID)
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}

	if !result.Pushed {
		t.Fatal("expected branch push to succeed")
	}
	if !result.DraftCreated {
		t.Fatalf("expected draft PR creation, warnings=%v", result.Warnings)
	}
	if result.CopiedToClipboard {
		t.Fatal("did not expect clipboard fallback when draft creation succeeds")
	}
	if result.URL != "https://example.com/pr/123" {
		t.Fatalf("result URL = %q, want %q", result.URL, "https://example.com/pr/123")
	}
	if !strings.Contains(result.Body, decision.WhySelected) {
		t.Fatalf("PR body missing decision rationale:\n%s", result.Body)
	}
	if !strings.Contains(result.Body, decision.Invariants[0]) {
		t.Fatalf("PR body missing invariant:\n%s", result.Body)
	}
	if !strings.Contains(result.Body, "Post-execution verification passed.") {
		t.Fatalf("PR body missing verification result:\n%s", result.Body)
	}

	pushArgs, err := os.ReadFile(pushLog)
	if err != nil {
		t.Fatalf("ReadFile push log: %v", err)
	}
	if !strings.Contains(string(pushArgs), "push --set-upstream origin HEAD:"+task.Branch) {
		t.Fatalf("unexpected push args: %q", string(pushArgs))
	}

	ghArgs, err := os.ReadFile(ghLog)
	if err != nil {
		t.Fatalf("ReadFile gh log: %v", err)
	}
	if !strings.Contains(string(ghArgs), "pr create --draft") {
		t.Fatalf("unexpected gh args: %q", string(ghArgs))
	}
	if !strings.Contains(string(ghArgs), "--head "+task.Branch) {
		t.Fatalf("expected gh args to include head branch, got %q", string(ghArgs))
	}
}

func TestCreatePullRequestCopiesBodyToClipboardWhenDraftCreationFails(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'verification task complete\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")
	initTestGitRepository(t, app.projectRoot)

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}

	installStubAgentBinary(
		t,
		"git",
		fmt.Sprintf(
			"#!/bin/sh\nif [ \"$1\" = \"push\" ]; then\n  exit 0\nfi\nexec %q \"$@\"\n",
			realGit,
		),
	)
	installStubAgentBinary(
		t,
		"gh",
		"#!/bin/sh\nprintf 'draft creation unavailable\\n' >&2\nexit 1\n",
	)

	decision, task := createReadyForPRTask(t, app)

	originalClipboardWriter := desktopClipboardWriter
	copiedBody := ""
	desktopClipboardWriter = func(_ context.Context, text string) error {
		copiedBody = text
		return nil
	}
	defer func() {
		desktopClipboardWriter = originalClipboardWriter
	}()

	result, err := app.CreatePullRequest(task.ID)
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}

	if !result.Pushed {
		t.Fatal("expected branch push to succeed before fallback")
	}
	if result.DraftCreated {
		t.Fatal("did not expect draft PR creation when gh fails")
	}
	if !result.CopiedToClipboard {
		t.Fatalf("expected clipboard fallback, warnings=%v", result.Warnings)
	}
	if copiedBody != result.Body {
		t.Fatalf("clipboard body mismatch:\nclipboard=%q\nresult=%q", copiedBody, result.Body)
	}
	if !strings.Contains(result.Body, decision.WhySelected) {
		t.Fatalf("clipboard body missing rationale:\n%s", result.Body)
	}
	if !containsWarning(result.Warnings, "Draft PR creation failed") {
		t.Fatalf("expected draft creation warning, got %v", result.Warnings)
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

func waitForTaskOutputContains(t *testing.T, app *App, taskID string, want string) string {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		output, err := app.GetTaskOutput(taskID)
		if err == nil && strings.Contains(output, want) {
			return output
		}

		time.Sleep(25 * time.Millisecond)
	}

	output, _ := app.GetTaskOutput(taskID)
	t.Fatalf("task %s output did not contain %q before timeout: %q", taskID, want, output)
	return ""
}

func containsArgs(args []string, key string, value string) bool {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == key && args[index+1] == value {
			return true
		}
	}

	return false
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}

	return false
}

func createReadyForPRTask(t *testing.T, app *App) (*DecisionDetailView, TaskState) {
	t.Helper()

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Create PR action problem",
		Signal:      "Ready-for-PR tasks stop before branch publication and draft creation.",
		Acceptance:  "The desktop flow can generate a PR body and publish the verified branch.",
		BlastRadius: "Desktop create PR action only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Create PR action",
		WhySelected:     "The operator needs the PR handoff to reuse decision rationale and verification evidence.",
		SelectionPolicy: "Prefer the smallest reversible step that closes the verified execution loop.",
		CounterArgument: "Manual PR creation is simpler and avoids GitHub CLI dependencies.",
		WeakestLink:     "Publishing can fail when git or gh auth is unavailable.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Manual PR body drafting",
				Reason:  "It duplicates reasoning that already exists on the decision and verification evidence.",
			},
		},
		Invariants: []string{
			"Generate PR body from decision rationale + invariants + verification result.",
		},
		AffectedFiles: []string{"README.md"},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Desktop-driven PR creation proves unreliable across supported environments.",
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

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "Ready for PR" {
		t.Fatalf("task status = %q, want Ready for PR", final.Status)
	}

	return decision, final
}
