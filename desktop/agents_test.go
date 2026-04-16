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

func TestBuildAgentArgsClaudeUsesStreamJSON(t *testing.T) {
	args := buildAgentArgs(AgentClaude, "inspect the task", "/tmp/ignored")

	if len(args) == 0 {
		t.Fatal("expected claude args")
	}

	if !containsArgs(args, "--output-format", "stream-json") {
		t.Fatalf("expected claude args to include stream-json output, got %v", args)
	}
}

func TestTaskTranscriptParsesClaudeStreamJSONAndStripsANSI(t *testing.T) {
	transcript := newTaskTranscript(AgentClaude)
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-1","cwd":"/tmp"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"\u001b[31mplan\u001b[0m"},{"type":"text","text":"\u001b[32mhello\u001b[0m"},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"echo hi"}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"\u001b[34mstdout\u001b[0m"}]}}`,
		"\x1b[35mraw fallback\x1b[0m",
	}, "\n") + "\n"

	display := transcript.AppendChunk(stream)
	blocks := transcript.Blocks()

	if len(blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(blocks))
	}

	if transcript.SessionID() != "sess-1" {
		t.Fatalf("expected transcript session id sess-1, got %q", transcript.SessionID())
	}

	if blocks[0].Type != "thinking" || blocks[0].Text != "plan" {
		t.Fatalf("unexpected thinking block: %#v", blocks[0])
	}

	if blocks[1].Type != "text" || blocks[1].Role != "assistant" || blocks[1].Text != "hello" {
		t.Fatalf("unexpected text block: %#v", blocks[1])
	}

	if blocks[2].Type != "tool_use" || blocks[2].Name != "Bash" || blocks[2].CallID != "toolu_1" {
		t.Fatalf("unexpected tool use block: %#v", blocks[2])
	}

	if blocks[3].Type != "tool_result" || blocks[3].Output != "stdout" || blocks[3].ParentID != blocks[2].ID {
		t.Fatalf("unexpected tool result block: %#v", blocks[3])
	}

	if blocks[4].Type != "text" || blocks[4].Text != "raw fallback" {
		t.Fatalf("unexpected raw fallback block: %#v", blocks[4])
	}

	assertNoANSI(t, display)
	assertNoANSI(t, blocks[0].Text)
	assertNoANSI(t, blocks[1].Text)
	assertNoANSI(t, blocks[3].Output)
	if !strings.Contains(display, "hello") || !strings.Contains(display, "stdout") || !strings.Contains(display, "raw fallback") {
		t.Fatalf("expected display output to include parsed content, got %q", display)
	}
}

func TestTaskTranscriptParsesClaudeMessageEnvelopeAndResultOutput(t *testing.T) {
	transcript := newTaskTranscript(AgentClaude)
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"sess-2","cwd":"/tmp"}`,
		`{"type":"message","message":{"role":"assistant","content":"\u001b[32mhello from message\u001b[0m"}}`,
		`{"type":"result","subtype":"success","session_id":"sess-2","result":"\u001b[34mfinal answer\u001b[0m"}`,
	}, "\n") + "\n"

	display := transcript.AppendChunk(stream)
	blocks := transcript.Blocks()

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	if transcript.SessionID() != "sess-2" {
		t.Fatalf("expected transcript session id sess-2, got %q", transcript.SessionID())
	}

	if blocks[0].Type != "text" || blocks[0].Role != "assistant" || blocks[0].Text != "hello from message" {
		t.Fatalf("unexpected message block: %#v", blocks[0])
	}

	if blocks[1].Type != "text" || blocks[1].Role != "assistant" || blocks[1].Text != "final answer" {
		t.Fatalf("unexpected result block: %#v", blocks[1])
	}

	assertNoANSI(t, display)
	assertNoANSI(t, blocks[0].Text)
	assertNoANSI(t, blocks[1].Text)
	if !strings.Contains(display, "hello from message") || !strings.Contains(display, "final answer") {
		t.Fatalf("expected display output to include parsed Claude message/result content, got %q", display)
	}
}

func TestTaskTranscriptParsesCodexJSONAndStripsANSI(t *testing.T) {
	transcript := newTaskTranscript(AgentCodex)
	stream := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-1"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.started","item":{"id":"cmd_1","type":"command_execution","command":"echo hi","status":"in_progress"}}`,
		`{"type":"item.completed","item":{"id":"reason_1","type":"reasoning","text":"\u001b[31mplan\u001b[0m"}}`,
		`{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"\u001b[32mDone\u001b[0m"}}`,
		`{"type":"item.completed","item":{"id":"cmd_1","type":"command_execution","status":"failed","aggregated_output":"\u001b[33mcommand failed\u001b[0m","exit_code":1}}`,
	}, "\n") + "\n"

	display := transcript.AppendChunk(stream)
	blocks := transcript.Blocks()

	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(blocks))
	}

	if transcript.SessionID() != "thread-1" {
		t.Fatalf("expected transcript session id thread-1, got %q", transcript.SessionID())
	}

	if blocks[0].Type != "tool_use" || blocks[0].CallID != "cmd_1" || blocks[0].Input != "echo hi" {
		t.Fatalf("unexpected tool use block: %#v", blocks[0])
	}

	if blocks[1].Type != "thinking" || blocks[1].Text != "plan" {
		t.Fatalf("unexpected thinking block: %#v", blocks[1])
	}

	if blocks[2].Type != "text" || blocks[2].Text != "Done" {
		t.Fatalf("unexpected text block: %#v", blocks[2])
	}

	if blocks[3].Type != "tool_result" || blocks[3].Output != "command failed" || !blocks[3].IsError || blocks[3].ParentID != blocks[0].ID {
		t.Fatalf("unexpected tool result block: %#v", blocks[3])
	}

	assertNoANSI(t, display)
	assertNoANSI(t, blocks[1].Text)
	assertNoANSI(t, blocks[2].Text)
	assertNoANSI(t, blocks[3].Output)
	if !strings.Contains(display, "Done") || !strings.Contains(display, "command failed") {
		t.Fatalf("expected display output to include parsed codex content, got %q", display)
	}
}

func TestTaskTranscriptParsesCodexAssistantItemsAndMCPToolCalls(t *testing.T) {
	transcript := newTaskTranscript(AgentCodex)
	stream := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread-2"}`,
		`{"type":"item.started","item":{"id":"msg_0","type":"assistant_message","status":"in_progress"}}`,
		`{"type":"item.started","item":{"id":"mcp_1","type":"mcp_tool_call","server":"github","tool":"search","arguments":{"query":"repo"}}}`,
		`{"type":"item.completed","item":{"id":"mcp_1","type":"mcp_tool_call","server":"github","tool":"search","result":{"content":[{"type":"text","text":"\u001b[33mfound\u001b[0m"}]}}}`,
		`{"type":"item.completed","item":{"id":"msg_1","type":"assistant_message","text":"\u001b[32mDone\u001b[0m"}}`,
		`{"type":"item.completed","item":{"id":"err_1","type":"error","text":"\u001b[31mboom\u001b[0m"}}`,
	}, "\n") + "\n"

	display := transcript.AppendChunk(stream)
	blocks := transcript.Blocks()

	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(blocks))
	}

	if blocks[0].Type != "tool_use" || blocks[0].Name != "github:search" || blocks[0].Input != `{"query":"repo"}` {
		t.Fatalf("unexpected codex tool use block: %#v", blocks[0])
	}

	if blocks[1].Type != "tool_result" || blocks[1].Output != "found" || blocks[1].ParentID != blocks[0].ID {
		t.Fatalf("unexpected codex tool result block: %#v", blocks[1])
	}

	if blocks[2].Type != "text" || blocks[2].Role != "assistant" || blocks[2].Text != "Done" {
		t.Fatalf("unexpected codex assistant message block: %#v", blocks[2])
	}

	if blocks[3].Type != "text" || blocks[3].Role != "system" || blocks[3].Text != "boom" || !blocks[3].IsError {
		t.Fatalf("unexpected codex error block: %#v", blocks[3])
	}

	assertNoANSI(t, display)
	assertNoANSI(t, blocks[0].Input)
	assertNoANSI(t, blocks[1].Output)
	assertNoANSI(t, blocks[2].Text)
	assertNoANSI(t, blocks[3].Text)
	if !strings.Contains(display, "found") || !strings.Contains(display, "Done") || !strings.Contains(display, "boom") {
		t.Fatalf("expected display output to include parsed Codex content, got %q", display)
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

	if !containsArg(args, "--json") {
		t.Fatalf("expected codex args to include JSON output, got %v", args)
	}

	if containsArg(args, "--full-auto") {
		t.Fatalf("expected codex checkpointed args to avoid --full-auto, got %v", args)
	}
}

func TestBuildClaudeResumeArgsUsesSessionID(t *testing.T) {
	args := buildAgentTurnArgs(AgentClaude, "follow up", "/tmp/ignored", "sess-42")

	if !containsArgs(args, "--resume", "sess-42") {
		t.Fatalf("expected claude resume args to include session id, got %v", args)
	}

	if !containsArg(args, "-p") {
		t.Fatalf("expected claude resume args to stay in print mode, got %v", args)
	}
}

func TestBuildCodexResumeArgsUsesSessionID(t *testing.T) {
	args := buildAgentTurnArgs(AgentCodex, "follow up", "/tmp/ignored", "thread-42")

	if len(args) < 4 || args[0] != "codex" || args[1] != "exec" || args[2] != "resume" {
		t.Fatalf("expected codex resume command, got %v", args)
	}

	if !containsArg(args, "--json") {
		t.Fatalf("expected codex resume args to include JSON output, got %v", args)
	}

	foundSession := false
	for _, arg := range args {
		if arg == "thread-42" {
			foundSession = true
			break
		}
	}

	if !foundSession {
		t.Fatalf("expected codex resume args to include session id, got %v", args)
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

func TestSpawnTaskFollowUpPersistsConversationAcrossRestart(t *testing.T) {
	app := newAuthoringTestApp(t)

	installStubAgentBinary(
		t,
		"claude",
		`#!/bin/sh
resume=""
while [ $# -gt 0 ]; do
  case "$1" in
    -p|--print|--verbose)
      shift
      ;;
    --output-format|--permission-mode|--resume)
      if [ "$1" = "--resume" ]; then
        resume="$2"
      fi
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [ -n "$resume" ]; then
  printf '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"follow-up reply"}]}}\n'
  exit 0
fi

printf '{"type":"system","subtype":"init","session_id":"sess-follow-up"}\n'
printf '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"initial reply"}]}}\n'
`,
	)

	task, err := app.SpawnTask("claude", "Inspect the runtime state", false, "")
	if err != nil {
		t.Fatalf("SpawnTask: %v", err)
	}

	waitForTaskOutputContains(t, app, task.ID, "initial reply")

	liveState, err := app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState initial: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("task status after initial reply = %q, want running", liveState.Status)
	}

	if len(liveState.ChatBlocks) != 2 {
		t.Fatalf("expected 2 chat blocks after initial reply, got %d", len(liveState.ChatBlocks))
	}

	if liveState.ChatBlocks[0].Role != "user" || liveState.ChatBlocks[0].Text != "Inspect the runtime state" {
		t.Fatalf("unexpected initial user block: %#v", liveState.ChatBlocks[0])
	}

	if liveState.ChatBlocks[1].Role != "assistant" || liveState.ChatBlocks[1].Text != "initial reply" {
		t.Fatalf("unexpected initial chat blocks: %#v", liveState.ChatBlocks)
	}

	if err := app.WriteTaskInput(task.ID, "Please continue"); err != nil {
		t.Fatalf("WriteTaskInput: %v", err)
	}

	liveOutput := waitForTaskOutputContains(t, app, task.ID, "follow-up reply")
	if !strings.Contains(liveOutput, "[user] Please continue") {
		t.Fatalf("expected user follow-up in live output, got %q", liveOutput)
	}

	liveState, err = app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState follow-up: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("task status after follow-up = %q, want running", liveState.Status)
	}

	if len(liveState.ChatBlocks) != 4 {
		t.Fatalf("expected 4 chat blocks after follow-up, got %d", len(liveState.ChatBlocks))
	}

	if liveState.ChatBlocks[2].Role != "user" || liveState.ChatBlocks[2].Text != "Please continue" {
		t.Fatalf("unexpected user follow-up block: %#v", liveState.ChatBlocks[2])
	}

	if liveState.ChatBlocks[3].Role != "assistant" || liveState.ChatBlocks[3].Text != "follow-up reply" {
		t.Fatalf("unexpected follow-up reply block: %#v", liveState.ChatBlocks[3])
	}

	projectRoot := app.projectRoot
	app.shutdown(context.Background())

	reopened := NewApp()
	reopened.projectRoot = projectRoot
	reopened.startup(context.Background())
	defer reopened.shutdown(context.Background())

	restored, err := reopened.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState reopened: %v", err)
	}

	if restored.Status != "interrupted" {
		t.Fatalf("restored task status = %q, want interrupted", restored.Status)
	}

	if len(restored.ChatBlocks) != 4 {
		t.Fatalf("expected reopened task to keep 4 chat blocks, got %d", len(restored.ChatBlocks))
	}

	if restored.ChatBlocks[0].Role != "user" || restored.ChatBlocks[0].Text != "Inspect the runtime state" {
		t.Fatalf("unexpected reopened initial user block: %#v", restored.ChatBlocks[0])
	}

	if restored.ChatBlocks[2].Role != "user" || restored.ChatBlocks[2].Text != "Please continue" {
		t.Fatalf("unexpected reopened user block: %#v", restored.ChatBlocks[2])
	}

	if restored.ChatBlocks[3].Text != "follow-up reply" {
		t.Fatalf("unexpected reopened assistant block: %#v", restored.ChatBlocks[3])
	}

	if !strings.Contains(restored.Output, "[user] Inspect the runtime state") ||
		!strings.Contains(restored.Output, "initial reply") ||
		!strings.Contains(restored.Output, "follow-up reply") {
		t.Fatalf("expected reopened output to preserve transcript, got %q", restored.Output)
	}
}

func TestSpawnTaskCodexFollowUpPersistsConversationAcrossRestart(t *testing.T) {
	app := newAuthoringTestApp(t)

	installStubAgentBinary(
		t,
		"codex",
		`#!/bin/sh
resume=""
thread=""
while [ $# -gt 0 ]; do
  case "$1" in
    exec)
      shift
      ;;
    resume)
      resume="1"
      shift
      ;;
    --cd|--ask-for-approval|-c)
      shift 2
      ;;
    --json)
      shift
      ;;
    *)
      if [ -n "$resume" ] && [ -z "$thread" ]; then
        thread="$1"
        shift
        continue
      fi
      shift
      ;;
  esac
done

if [ -n "$resume" ]; then
  printf '{"type":"item.completed","item":{"id":"msg_2","type":"agent_message","text":"follow-up reply"}}\n'
  exit 0
fi

printf '{"type":"thread.started","thread_id":"thread-follow-up"}\n'
printf '{"type":"item.started","item":{"id":"cmd_1","type":"command_execution","command":"echo hi","status":"in_progress"}}\n'
printf '{"type":"item.completed","item":{"id":"cmd_1","type":"command_execution","status":"completed","aggregated_output":"tool stdout"}}\n'
printf 'raw fallback line\n'
printf '{"type":"item.completed","item":{"id":"msg_1","type":"agent_message","text":"initial reply"}}\n'
`,
	)

	task, err := app.SpawnTask("codex", "Inspect the runtime state", false, "")
	if err != nil {
		t.Fatalf("SpawnTask: %v", err)
	}

	waitForTaskOutputContains(t, app, task.ID, "initial reply")

	liveState, err := app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState initial: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("task status after initial reply = %q, want running", liveState.Status)
	}

	if len(liveState.ChatBlocks) != 5 {
		t.Fatalf("expected 5 chat blocks after initial reply, got %d", len(liveState.ChatBlocks))
	}

	if liveState.ChatBlocks[0].Role != "user" || liveState.ChatBlocks[0].Text != "Inspect the runtime state" {
		t.Fatalf("unexpected codex initial user block: %#v", liveState.ChatBlocks[0])
	}

	if liveState.ChatBlocks[1].Type != "tool_use" || liveState.ChatBlocks[1].Input != "echo hi" {
		t.Fatalf("unexpected codex tool use block: %#v", liveState.ChatBlocks[1])
	}

	if liveState.ChatBlocks[2].Type != "tool_result" ||
		liveState.ChatBlocks[2].Output != "tool stdout" ||
		liveState.ChatBlocks[2].ParentID != liveState.ChatBlocks[1].ID {
		t.Fatalf("unexpected codex tool result block: %#v", liveState.ChatBlocks[2])
	}

	if liveState.ChatBlocks[3].Type != "text" || liveState.ChatBlocks[3].Text != "raw fallback line" {
		t.Fatalf("unexpected codex raw fallback block: %#v", liveState.ChatBlocks[3])
	}

	if liveState.ChatBlocks[4].Role != "assistant" || liveState.ChatBlocks[4].Text != "initial reply" {
		t.Fatalf("unexpected codex initial reply block: %#v", liveState.ChatBlocks[4])
	}

	if err := app.WriteTaskInput(task.ID, "Please continue"); err != nil {
		t.Fatalf("WriteTaskInput: %v", err)
	}

	liveOutput := waitForTaskOutputContains(t, app, task.ID, "follow-up reply")
	if !strings.Contains(liveOutput, "[user] Please continue") {
		t.Fatalf("expected user follow-up in live output, got %q", liveOutput)
	}

	liveState, err = app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState follow-up: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("task status after follow-up = %q, want running", liveState.Status)
	}

	if len(liveState.ChatBlocks) != 7 {
		t.Fatalf("expected 7 chat blocks after follow-up, got %d", len(liveState.ChatBlocks))
	}

	if liveState.ChatBlocks[5].Role != "user" || liveState.ChatBlocks[5].Text != "Please continue" {
		t.Fatalf("unexpected codex user follow-up block: %#v", liveState.ChatBlocks[5])
	}

	if liveState.ChatBlocks[6].Role != "assistant" || liveState.ChatBlocks[6].Text != "follow-up reply" {
		t.Fatalf("unexpected codex follow-up reply block: %#v", liveState.ChatBlocks[6])
	}

	projectRoot := app.projectRoot
	app.shutdown(context.Background())

	reopened := NewApp()
	reopened.projectRoot = projectRoot
	reopened.startup(context.Background())
	defer reopened.shutdown(context.Background())

	restored, err := reopened.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState reopened: %v", err)
	}

	if restored.Status != "interrupted" {
		t.Fatalf("restored task status = %q, want interrupted", restored.Status)
	}

	if len(restored.ChatBlocks) != 7 {
		t.Fatalf("expected reopened task to keep 7 chat blocks, got %d", len(restored.ChatBlocks))
	}

	if restored.ChatBlocks[0].Role != "user" || restored.ChatBlocks[0].Text != "Inspect the runtime state" {
		t.Fatalf("unexpected reopened codex initial user block: %#v", restored.ChatBlocks[0])
	}

	if restored.ChatBlocks[3].Text != "raw fallback line" {
		t.Fatalf("unexpected reopened codex raw fallback block: %#v", restored.ChatBlocks[3])
	}

	if restored.ChatBlocks[5].Role != "user" || restored.ChatBlocks[5].Text != "Please continue" {
		t.Fatalf("unexpected reopened codex user block: %#v", restored.ChatBlocks[5])
	}

	if restored.ChatBlocks[6].Text != "follow-up reply" {
		t.Fatalf("unexpected reopened codex assistant block: %#v", restored.ChatBlocks[6])
	}

	if !strings.Contains(restored.Output, "[user] Inspect the runtime state") ||
		!strings.Contains(restored.Output, "tool stdout") ||
		!strings.Contains(restored.Output, "raw fallback line") ||
		!strings.Contains(restored.Output, "initial reply") ||
		!strings.Contains(restored.Output, "follow-up reply") {
		t.Fatalf("expected reopened codex output to preserve transcript, got %q", restored.Output)
	}
}

func TestVerifyDecisionKeepsTaskConversational(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(
		t,
		"claude",
		`#!/bin/sh
resume=""
while [ $# -gt 0 ]; do
  case "$1" in
    -p|--print|--verbose)
      shift
      ;;
    --output-format|--permission-mode|--resume)
      if [ "$1" = "--resume" ]; then
        resume="$2"
      fi
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [ -n "$resume" ]; then
  printf '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"verification follow-up"}]}}\n'
  exit 0
fi

printf '{"type":"system","subtype":"init","session_id":"sess-verify"}\n'
printf '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"verification initial"}]}}\n'
`,
	)

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Keep verify tasks conversational",
		Signal:      "Verification tasks stop after the first assistant reply.",
		Acceptance:  "The operator can continue the same verification thread with follow-up input.",
		BlastRadius: "Desktop task runner only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Conversational verification",
		WhySelected:     "Verification needs iterative follow-up in the same task thread.",
		SelectionPolicy: "Prefer the smallest runner change that preserves task continuity.",
		CounterArgument: "Single-shot verification is simpler to implement.",
		WeakestLink:     "The agent backend must expose a resumable session identifier.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Single-shot verification",
				Reason:  "It blocks follow-up questions after the first response.",
			},
		},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"Verification follow-up cannot resume after the first reply.",
			},
		},
		Mode: "tactical",
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	task, err := app.VerifyDecision(decision.ID, "claude")
	if err != nil {
		t.Fatalf("VerifyDecision: %v", err)
	}

	waitForTaskOutputContains(t, app, task.ID, "verification initial")

	liveState, err := app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState initial: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("verify task status after initial reply = %q, want running", liveState.Status)
	}

	if len(liveState.ChatBlocks) != 2 {
		t.Fatalf("expected 2 verify chat blocks after initial reply, got %d", len(liveState.ChatBlocks))
	}

	if liveState.ChatBlocks[0].Role != "user" || liveState.ChatBlocks[0].Text != strings.TrimSpace(task.Prompt) {
		t.Fatalf("unexpected verify prompt block: %#v", liveState.ChatBlocks[0])
	}

	if liveState.ChatBlocks[1].Role != "assistant" || liveState.ChatBlocks[1].Text != "verification initial" {
		t.Fatalf("unexpected verify initial reply block: %#v", liveState.ChatBlocks[1])
	}

	if err := app.WriteTaskInput(task.ID, "Double-check the evidence links"); err != nil {
		t.Fatalf("WriteTaskInput: %v", err)
	}

	waitForTaskOutputContains(t, app, task.ID, "verification follow-up")

	liveState, err = app.loadTaskState(task.ID)
	if err != nil {
		t.Fatalf("loadTaskState follow-up: %v", err)
	}

	if liveState.Status != "running" {
		t.Fatalf("verify task status after follow-up = %q, want running", liveState.Status)
	}

	if len(liveState.ChatBlocks) != 4 {
		t.Fatalf("expected 4 verify chat blocks after follow-up, got %d", len(liveState.ChatBlocks))
	}

	if liveState.ChatBlocks[2].Role != "user" || liveState.ChatBlocks[2].Text != "Double-check the evidence links" {
		t.Fatalf("unexpected verify follow-up block: %#v", liveState.ChatBlocks[2])
	}

	if liveState.ChatBlocks[3].Role != "assistant" || liveState.ChatBlocks[3].Text != "verification follow-up" {
		t.Fatalf("unexpected verify follow-up reply block: %#v", liveState.ChatBlocks[3])
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

func TestImplementHappyPathSmoke(t *testing.T) {
	app := newAuthoringTestApp(t)
	defer app.shutdown(context.Background())

	installStubAgentBinary(t, "claude", "#!/bin/sh\nprintf 'happy-path implementation complete\\n'\n")
	installStubAgentBinary(t, "haft", "#!/bin/sh\nexit 0\n")

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
			"#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %q\nprintf 'https://example.com/pr/456\\n'\n",
			ghLog,
		),
	)

	initTestGitRepository(t, app.projectRoot)

	problem, err := app.CreateProblem(ProblemCreateInput{
		Title:       "Implement happy path smoke test",
		Signal:      "The desktop execution loop needs one explicit end-to-end smoke test.",
		Acceptance:  "Create a decision, implement it, verify it, baseline it, and generate a PR body from the resulting task.",
		BlastRadius: "Desktop implement flow only",
		Mode:        "tactical",
	})
	if err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	decision, err := app.CreateDecision(DecisionCreateInput{
		ProblemRef:      problem.ID,
		SelectedTitle:   "Exercise the desktop implement happy path",
		WhySelected:     "One explicit smoke test should prove the decision-to-PR loop holds end to end.",
		SelectionPolicy: "Prefer the narrowest automated proof that matches the dashboard contract.",
		CounterArgument: "The existing narrower tests already cover the same code paths individually.",
		WeakestLink:     "The smoke test depends on stable git and gh CLI stubs for branch publication.",
		WhyNotOthers: []DecisionRejectionInput{
			{
				Variant: "Manual dashboard walkthrough only",
				Reason:  "It leaves the release gate dependent on operator memory instead of a repeatable test.",
			},
		},
		Invariants: []string{
			"Generate PR body from decision rationale, invariants, and verification evidence.",
		},
		AffectedFiles: []string{"README.md"},
		Rollback: &DecisionRollbackInput{
			Triggers: []string{
				"The smoke test becomes brittle enough to slow routine desktop changes.",
			},
		},
		Mode: "tactical",
	})
	if err != nil {
		t.Fatalf("CreateDecision: %v", err)
	}

	task, err := app.Implement(decision.ID)
	if err != nil {
		t.Fatalf("Implement: %v", err)
	}

	final := waitForTaskState(t, app, task.ID)
	if final.Status != "Ready for PR" {
		t.Fatalf("task status = %q, want Ready for PR", final.Status)
	}
	if !strings.Contains(final.Output, "happy-path implementation complete") {
		t.Fatalf("task output missing agent marker: %q", final.Output)
	}
	if !strings.Contains(final.Output, "Post-execution verification passed") {
		t.Fatalf("task output missing verification note: %q", final.Output)
	}

	files, err := app.store.GetAffectedFiles(context.Background(), decision.ID)
	if err != nil {
		t.Fatalf("GetAffectedFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("affected files = %d, want 1", len(files))
	}
	if files[0].Hash == "" {
		t.Fatal("expected baseline hash to be recorded for README.md")
	}

	items, err := app.store.GetEvidenceItems(context.Background(), decision.ID)
	if err != nil {
		t.Fatalf("GetEvidenceItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("evidence items = %d, want 1", len(items))
	}
	if items[0].Verdict != "supports" {
		t.Fatalf("evidence verdict = %q, want supports", items[0].Verdict)
	}
	if items[0].CongruenceLevel != 3 {
		t.Fatalf("evidence congruence = %d, want 3", items[0].CongruenceLevel)
	}

	result, err := app.CreatePullRequest(final.ID)
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}

	if !result.Pushed {
		t.Fatal("expected branch push to succeed")
	}
	if !result.DraftCreated {
		t.Fatalf("expected draft PR creation, warnings=%v", result.Warnings)
	}
	if result.URL != "https://example.com/pr/456" {
		t.Fatalf("result URL = %q, want %q", result.URL, "https://example.com/pr/456")
	}

	expectedBodySnippets := []string{
		"## Summary: Exercise the desktop implement happy path",
		decision.WhySelected,
		decision.Invariants[0],
		"Post-execution verification passed.",
	}

	for _, snippet := range expectedBodySnippets {
		if !strings.Contains(result.Body, snippet) {
			t.Fatalf("PR body missing %q:\n%s", snippet, result.Body)
		}
	}

	pushArgs, err := os.ReadFile(pushLog)
	if err != nil {
		t.Fatalf("ReadFile push log: %v", err)
	}
	if !strings.Contains(string(pushArgs), "push --set-upstream origin HEAD:"+final.Branch) {
		t.Fatalf("unexpected push args: %q", string(pushArgs))
	}

	ghArgs, err := os.ReadFile(ghLog)
	if err != nil {
		t.Fatalf("ReadFile gh log: %v", err)
	}
	if !strings.Contains(string(ghArgs), "pr create --draft") {
		t.Fatalf("unexpected gh args: %q", string(ghArgs))
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

func assertNoANSI(t *testing.T, value string) {
	t.Helper()

	if strings.ContainsRune(value, '\x1b') {
		t.Fatalf("expected ANSI to be stripped, got %q", value)
	}
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
