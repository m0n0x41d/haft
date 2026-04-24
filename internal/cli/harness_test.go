package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestBuildHarnessPlanSequentialDependencies(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	harnessPlanID = "plan-test"
	harnessPlanTitle = "Test harness plan"
	harnessPlanSequential = true
	commissionFromDecisionRepoRef = "local:test"
	commissionFromDecisionBaseSHA = "base-r1"
	commissionFromDecisionTargetBranch = "dev"
	commissionFromDecisionEvidence = []string{"go test ./internal/cli"}

	plan, err := buildHarnessPlan(t.TempDir(), []string{"dec-a", "dec-b", "dec-c"})
	if err != nil {
		t.Fatal(err)
	}

	if plan.ID != "plan-test" {
		t.Fatalf("plan id = %s, want plan-test", plan.ID)
	}
	if plan.Decisions[0].Ref != "dec-a" {
		t.Fatalf("first decision = %#v, want dec-a", plan.Decisions[0])
	}
	if len(plan.Decisions[0].DependsOn) != 0 {
		t.Fatalf("first depends_on = %#v, want empty", plan.Decisions[0].DependsOn)
	}
	if !stringSliceContains(plan.Decisions[1].DependsOn, "dec-a") {
		t.Fatalf("second depends_on = %#v, want dec-a", plan.Decisions[1].DependsOn)
	}
	if !stringSliceContains(plan.Decisions[2].DependsOn, "dec-b") {
		t.Fatalf("third depends_on = %#v, want dec-b", plan.Decisions[2].DependsOn)
	}
	if !stringSliceContains(plan.Defaults.EvidenceRequirements, "go test ./internal/cli") {
		t.Fatalf("evidence = %#v, want go test", plan.Defaults.EvidenceRequirements)
	}
}

func TestBuildHarnessPlanRejectsDependencyOutsidePlan(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	harnessPlanDependencies = []string{"dec-b:dec-missing"}
	commissionFromDecisionRepoRef = "local:test"
	commissionFromDecisionBaseSHA = "base-r1"
	commissionFromDecisionTargetBranch = "dev"

	_, err := buildHarnessPlan(t.TempDir(), []string{"dec-a", "dec-b"})
	if err == nil || !strings.Contains(err.Error(), "dependency source dec-missing") {
		t.Fatalf("error = %v, want missing dependency rejection", err)
	}
}

func TestResolveHarnessDecisionRefsByProblem(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Harness selector",
		Signal:     "The operator should run all decisions for one problem.",
		Acceptance: "Problem selector returns linked decisions.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Problem selector decision",
		WhySelected:     "It belongs to the selected problem.",
		SelectionPolicy: "Keep problem-scoped harness runs explicit.",
		CounterArgument: "Listing ids is simpler.",
		WeakestLink:     "Selection must not pull unrelated decisions.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual ids",
			Reason:  "Too much operator ceremony.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Problem selector regresses."}},
		AffectedFiles: []string{"internal/cli/harness.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = createCommissionDecisionFixture(t, ctx, store, haftDir, "Unrelated selector decision", "internal/cli/run.go")

	harnessPlanProblems = []string{problem.Meta.ID}

	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one problem-linked decision", refs)
	}
	if refs[0] != decision.Meta.ID {
		t.Fatalf("refs = %#v, want [%s]", refs, decision.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsByContext(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	firstProblem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Context:    "mvp-harness",
		Title:      "Harness context first",
		Signal:     "The operator should run a context-scoped workset.",
		Acceptance: "Context selector returns linked decisions.",
	})
	if err != nil {
		t.Fatal(err)
	}

	secondProblem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Context:    "mvp-harness",
		Title:      "Harness context second",
		Signal:     "The operator should run a context-scoped workset.",
		Acceptance: "Context selector returns linked decisions.",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      firstProblem.Meta.ID,
		SelectedTitle:   "First context decision",
		WhySelected:     "It belongs to the selected context.",
		SelectionPolicy: "Keep problem-scoped harness runs explicit.",
		CounterArgument: "Listing ids is simpler.",
		WeakestLink:     "Selection must not pull unrelated decisions.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual ids",
			Reason:  "Too much operator ceremony.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Problem selector regresses."}},
		AffectedFiles: []string{"internal/cli/harness.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	second, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      secondProblem.Meta.ID,
		SelectedTitle:   "Second context decision",
		WhySelected:     "It belongs to the selected context.",
		SelectionPolicy: "Keep context-scoped harness runs explicit.",
		CounterArgument: "Listing ids is simpler.",
		WeakestLink:     "Selection must not pull unrelated decisions.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual ids",
			Reason:  "Too much operator ceremony.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Context selector regresses."}},
		AffectedFiles: []string{"internal/cli/harness_test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = createCommissionDecisionFixture(t, ctx, store, haftDir, "Unrelated context decision", "internal/cli/run.go")

	harnessPlanContext = "mvp-harness"
	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want two context decisions", refs)
	}
	if !stringSliceContains(refs, first.Meta.ID) || !stringSliceContains(refs, second.Meta.ID) {
		t.Fatalf("refs = %#v, want %s and %s", refs, first.Meta.ID, second.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsDefaultsToUncommissionedActive(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	commissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "Commissioned decision", "internal/cli/harness.go")
	uncommissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "Uncommissioned decision", "internal/cli/run.go")

	commission := workCommissionFixture("wc-commissioned-decision", "queued", "2099-01-01T00:00:00Z")
	commission["decision_ref"] = commissioned.Meta.ID

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one uncommissioned decision", refs)
	}
	if refs[0] != uncommissioned.Meta.ID {
		t.Fatalf("refs = %#v, want [%s]", refs, uncommissioned.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsDefaultSkipsDecisionsWithoutAffectedFiles(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	unscoped := createCommissionDecisionWithoutAffectedFiles(t, ctx, store, haftDir, "Unscoped decision")
	scoped := createCommissionDecisionFixture(t, ctx, store, haftDir, "Scoped decision", "internal/cli/harness.go")

	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one scoped decision", refs)
	}
	if refs[0] != scoped.Meta.ID {
		t.Fatalf("refs = %#v, want [%s], not %s", refs, scoped.Meta.ID, unscoped.Meta.ID)
	}
}

func TestResolveHarnessDecisionRefsAllActiveIncludesCommissioned(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	commissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "All active commissioned", "internal/cli/harness.go")
	uncommissioned := createCommissionDecisionFixture(t, ctx, store, haftDir, "All active uncommissioned", "internal/cli/run.go")

	commission := workCommissionFixture("wc-all-active-commissioned", "queued", "2099-01-01T00:00:00Z")
	commission["decision_ref"] = commissioned.Meta.ID

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	harnessPlanAllActive = true
	refs, err := resolveHarnessDecisionRefs(ctx, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs) != 2 {
		t.Fatalf("refs = %#v, want both active decisions", refs)
	}
	if !stringSliceContains(refs, commissioned.Meta.ID) || !stringSliceContains(refs, uncommissioned.Meta.ID) {
		t.Fatalf("refs = %#v, want %s and %s", refs, commissioned.Meta.ID, uncommissioned.Meta.ID)
	}
}

func TestEnsureHarnessCommissionsSkipsExistingPlan(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	commission := workCommissionFixture("wc-existing-plan", "queued", "2099-01-01T00:00:00Z")
	commission["implementation_plan_ref"] = "plan-existing"
	commission["implementation_plan_revision"] = "p1"

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	created, result, err := ensureHarnessCommissions(ctx, store, t.TempDir(), map[string]any{
		"id":       "plan-existing",
		"revision": "p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatalf("created = true, want existing commissions reused")
	}
	if !strings.Contains(result, "reused 1 existing commission") {
		t.Fatalf("result = %q, want reuse message", result)
	}

	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want no duplicate commissions", len(records))
	}
}

func TestExistingRunnableHarnessPlanFindsPreparedCommissions(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	planPath := filepath.Join(projectRoot, ".haft", "plans", "plan-20260422-001.yaml")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("id: plan-20260422-001\nrevision: plan-r1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	commission := workCommissionFixture("wc-existing-runnable", "queued", "2099-01-01T00:00:00Z")
	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	gotPath, plan, result, found, err := existingRunnableHarnessPlan(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("found = false, want existing runnable commission plan")
	}
	if gotPath != planPath {
		t.Fatalf("plan path = %q, want %q", gotPath, planPath)
	}
	if stringField(plan, "id") != "plan-20260422-001" {
		t.Fatalf("plan id = %q, want plan-20260422-001", stringField(plan, "id"))
	}
	if !strings.Contains(result, "using 1 existing runnable commission") {
		t.Fatalf("result = %q, want existing runnable commission summary", result)
	}
}

func TestHarnessPhaseConfigMeasureIncludesEvidenceTools(t *testing.T) {
	phases := harnessPhaseConfig()
	measure, ok := phases["measure"].(map[string]any)
	if !ok {
		t.Fatalf("measure config = %#v, want map", phases["measure"])
	}

	tools, ok := measure["tools"].([]string)
	if !ok {
		t.Fatalf("measure tools = %#v, want []string", measure["tools"])
	}

	for _, tool := range []string{"bash", "read", "grep", "haft_query", "haft_decision", "haft_refresh"} {
		if !stringSliceContains(tools, tool) {
			t.Fatalf("measure tools = %#v, want %q", tools, tool)
		}
	}
}

func TestHarnessPromptTemplatesUseAuthoritativeCommissionSnapshot(t *testing.T) {
	templates := harnessPromptTemplates()

	for _, fragment := range []string{
		"Use this authoritative WorkCommission snapshot.",
		"{{commission.json}}",
		"Do not stop at analysis, narration, or a plan.",
		"Run or inspect the required evidence listed in the commission snapshot.",
	} {
		if !strings.Contains(templates, fragment) {
			t.Fatalf("prompt templates missing %q:\n%s", fragment, templates)
		}
	}
}

func TestFormatHarnessStatusIncludesRunningDetails(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime.jsonl")
	logLines := []map[string]any{
		{
			"at":    "2026-04-23T14:00:40Z",
			"event": "session_started",
			"data": map[string]any{
				"session_id": "session-1",
			},
		},
		{
			"at":    "2026-04-23T14:00:42Z",
			"event": "agent_turn_completed",
			"data": map[string]any{
				"session_id":   "session-1",
				"status":       "completed",
				"turn_id":      "turn-1",
				"text_preview": "executor inspected the scoped files and is ready to edit the portable MCP config",
			},
		},
	}

	encoded := make([]string, 0, len(logLines))
	for _, line := range logLines {
		payload, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, string(payload))
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(encoded, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	status := map[string]any{
		"updated_at": "2026-04-23T14:00:42Z",
		"metadata": map[string]any{
			"agent_kind":     "codex",
			"tracker_kind":   "commission_source:haft",
			"config_path":    "/tmp/sleigh.md",
			"workspace_root": "/tmp/workspaces",
		},
		"orchestrator": map[string]any{
			"claimed":       []any{"wc-1"},
			"running":       []any{"session-1"},
			"pending_human": []any{},
			"running_details": []any{
				map[string]any{
					"session_id":     "session-1",
					"commission_id":  "wc-1",
					"phase":          "frame",
					"sub_state":      "preparing_workspace",
					"task_pid":       "#PID<0.1.0>",
					"workspace_path": "/tmp/workspaces/wc-1",
				},
			},
		},
		"failures": []any{},
	}

	lines := formatHarnessStatus(status, "/tmp/status.json", logPath, harnessSessionLogSummaries(logPath))
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "agent: codex") {
		t.Fatalf("status output missing agent:\n%s", joined)
	}
	if !strings.Contains(joined, "claimed: 1") {
		t.Fatalf("status output missing claimed count:\n%s", joined)
	}
	if !strings.Contains(joined, "commission=wc-1") {
		t.Fatalf("status output missing running commission:\n%s", joined)
	}
	if !strings.Contains(joined, "workspace=/tmp/workspaces/wc-1") {
		t.Fatalf("status output missing workspace:\n%s", joined)
	}
	if !strings.Contains(joined, "started_at=2026-04-23T14:00:40Z") {
		t.Fatalf("status output missing started_at:\n%s", joined)
	}
	if !strings.Contains(joined, "elapsed=") {
		t.Fatalf("status output missing elapsed:\n%s", joined)
	}
	if !strings.Contains(joined, "last_event=agent_turn_completed") {
		t.Fatalf("status output missing last_event:\n%s", joined)
	}
	if !strings.Contains(joined, "last_turn=completed") {
		t.Fatalf("status output missing last_turn:\n%s", joined)
	}
	if !strings.Contains(joined, "turn_id=turn-1") {
		t.Fatalf("status output missing turn_id:\n%s", joined)
	}
	if !strings.Contains(joined, "preview=executor inspected the scoped files") {
		t.Fatalf("status output missing preview:\n%s", joined)
	}
}

func TestFormatHarnessResultIncludesCurrentRuntime(t *testing.T) {
	commission := map[string]any{
		"id":                      "wc-1",
		"state":                   "running",
		"decision_ref":            "dec-1",
		"implementation_plan_ref": "plan-1",
		"events":                  []any{},
	}
	runtimeDetail := map[string]any{
		"phase":          "frame",
		"sub_state":      "preparing_workspace",
		"session_id":     "session-1",
		"task_pid":       "#PID<0.1.0>",
		"workspace_path": "/tmp/workspaces/wc-1",
	}

	lines := formatHarnessResult(
		commission,
		"/tmp/workspaces",
		runtimeDetail,
		"2026-04-23T16:35:53Z",
		harnessSessionLogSummary{
			LastEvent:      "agent_turn_started",
			LastEventAt:    "2026-04-23T16:35:50Z",
			LastTurnStatus: "started",
		},
		harnessCommissionLogSummary{},
	)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "current_runtime:") {
		t.Fatalf("result output missing current_runtime section:\n%s", joined)
	}
	if !strings.Contains(joined, "phase=frame") {
		t.Fatalf("result output missing runtime phase:\n%s", joined)
	}
	if !strings.Contains(joined, "status_updated_at=2026-04-23T16:35:53Z") {
		t.Fatalf("result output missing runtime status timestamp:\n%s", joined)
	}
	if !strings.Contains(joined, "last_event=agent_turn_started") {
		t.Fatalf("result output missing runtime last_event:\n%s", joined)
	}
	if !strings.Contains(joined, "last_turn=started") {
		t.Fatalf("result output missing runtime last_turn:\n%s", joined)
	}
}

func TestFormatHarnessResultIncludesLatestAgentTurnForCompletedCommission(t *testing.T) {
	commission := map[string]any{
		"id":                      "wc-1",
		"state":                   "completed",
		"decision_ref":            "dec-1",
		"implementation_plan_ref": "plan-1",
		"events":                  []any{},
	}

	lines := formatHarnessResult(
		commission,
		"/tmp/workspaces",
		nil,
		"",
		harnessSessionLogSummary{},
		harnessCommissionLogSummary{
			Phase:       "measure",
			Event:       "agent_turn_completed",
			At:          "2026-04-23T19:17:09Z",
			TurnID:      "turn-7",
			TurnStatus:  "completed",
			TextPreview: "Measurement partial: both required commands passed and the portability assertions are covered in init smoke tests.",
		},
	)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "last_agent_turn:") {
		t.Fatalf("result output missing last_agent_turn section:\n%s", joined)
	}
	if !strings.Contains(joined, "phase=measure event=agent_turn_completed") {
		t.Fatalf("result output missing latest turn phase/event:\n%s", joined)
	}
	if !strings.Contains(joined, "status=completed") {
		t.Fatalf("result output missing latest turn status:\n%s", joined)
	}
	if !strings.Contains(joined, "turn_id=turn-7") {
		t.Fatalf("result output missing latest turn id:\n%s", joined)
	}
	if !strings.Contains(joined, "preview=Measurement partial: both required commands passed") {
		t.Fatalf("result output missing latest turn preview:\n%s", joined)
	}
}

func TestFormatHarnessResultEventsShowsCurrentAttemptOnly(t *testing.T) {
	lines := formatHarnessResultEvents([]map[string]any{
		{
			"event":       "phase_outcome",
			"verdict":     "pass",
			"recorded_at": "2026-04-23T18:04:01Z",
			"payload": map[string]any{
				"phase": "frame",
				"next":  "advance:execute",
			},
		},
		{
			"event":       "phase_outcome",
			"verdict":     "pass",
			"recorded_at": "2026-04-23T18:08:20Z",
			"payload": map[string]any{
				"phase": "execute",
				"next":  "advance:measure",
			},
		},
		{
			"event":       "commission_requeued",
			"action":      "requeue",
			"reason":      "operator_requested_requeue",
			"recorded_at": "2026-04-23T18:14:03Z",
		},
		{
			"event":       "phase_outcome",
			"verdict":     "pass",
			"recorded_at": "2026-04-23T18:16:08Z",
			"payload": map[string]any{
				"phase": "preflight",
				"next":  "advance:frame",
			},
		},
	})
	joined := strings.Join(lines, "\n")

	if strings.Contains(joined, "frame verdict=pass next=advance:execute at=2026-04-23T18:04:01Z") {
		t.Fatalf("result output leaked prior-attempt frame outcome:\n%s", joined)
	}
	if strings.Contains(joined, "execute verdict=pass next=advance:measure at=2026-04-23T18:08:20Z") {
		t.Fatalf("result output leaked prior-attempt execute outcome:\n%s", joined)
	}
	if !strings.Contains(joined, "preflight verdict=pass next=advance:frame at=2026-04-23T18:16:08Z") {
		t.Fatalf("result output missing current attempt preflight outcome:\n%s", joined)
	}
	if !strings.Contains(joined, "last_event: phase_outcome") {
		t.Fatalf("result output missing current attempt last_event:\n%s", joined)
	}
}

func TestRecentHarnessLogLinesFiltersToCurrentRun(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime.jsonl")
	status := map[string]any{
		"metadata": map[string]any{
			"config_path": "/tmp/current-sleigh.md",
		},
		"orchestrator": map[string]any{
			"running_details": []any{
				map[string]any{
					"commission_id": "wc-current",
				},
			},
		},
	}

	lines := []map[string]any{
		{
			"event":         "runtime_started",
			"metadata":      map[string]any{"config_path": "/tmp/old-sleigh.md"},
			"commission_id": "wc-old",
		},
		{
			"event":         "runtime_started",
			"metadata":      map[string]any{"config_path": "/tmp/current-sleigh.md"},
			"commission_id": "",
		},
		{
			"event":         "session_started",
			"metadata":      map[string]any{"config_path": "/tmp/current-sleigh.md"},
			"commission_id": "wc-current",
		},
		{
			"event":         "session_started",
			"metadata":      map[string]any{"config_path": "/tmp/old-sleigh.md"},
			"commission_id": "wc-old",
		},
	}

	encoded := make([]string, 0, len(lines))
	for _, line := range lines {
		payload, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, string(payload))
	}

	if err := os.WriteFile(logPath, []byte(strings.Join(encoded, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	filtered, err := recentHarnessLogLines(status, logPath, 10)
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(filtered, "\n")
	if strings.Contains(joined, "wc-old") {
		t.Fatalf("filtered log still contains old commission events:\n%s", joined)
	}
	if !strings.Contains(joined, "wc-current") {
		t.Fatalf("filtered log missing current commission event:\n%s", joined)
	}
	if !strings.Contains(joined, "/tmp/current-sleigh.md") {
		t.Fatalf("filtered log missing current runtime events:\n%s", joined)
	}
}

func TestDefaultHarnessRunOptionsUsesRuntimeEnv(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	t.Setenv("HAFT_OPEN_SLEIGH_RUNTIME", "/tmp/open-sleigh-runtime")

	opts := defaultHarnessRunOptions(t.TempDir(), ".haft/plans/p.yaml", map[string]any{
		"id": "plan-runtime-env",
	})

	if opts.RuntimePath != "/tmp/open-sleigh-runtime" {
		t.Fatalf("runtime path = %q, want env value", opts.RuntimePath)
	}
}

func TestValidateOpenSleighRuntimeRequiresMixExs(t *testing.T) {
	runtimePath := t.TempDir()

	err := validateOpenSleighRuntime(runtimePath)
	if err == nil || !strings.Contains(err.Error(), "missing bin/open_sleigh or mix.exs") {
		t.Fatalf("error = %v, want missing runtime marker", err)
	}
}

func TestValidateOpenSleighRuntimeAcceptsSourceRuntime(t *testing.T) {
	runtimePath := t.TempDir()
	mixPath := filepath.Join(runtimePath, "mix.exs")
	if err := os.WriteFile(mixPath, []byte("defmodule OpenSleigh.MixProject do\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := exec.LookPath("mix"); err != nil {
		t.Skipf("mix is not installed: %v", err)
	}

	if err := validateOpenSleighRuntime(runtimePath); err != nil {
		t.Fatalf("validateOpenSleighRuntime returned error: %v", err)
	}
}

func TestValidateOpenSleighRuntimeAcceptsReleaseRuntime(t *testing.T) {
	runtimePath := t.TempDir()
	binDir := filepath.Join(runtimePath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(binDir, "open_sleigh")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := validateOpenSleighRuntime(runtimePath); err != nil {
		t.Fatalf("validateOpenSleighRuntime returned error: %v", err)
	}
}

func TestElixirStringListLiteralEscapesArguments(t *testing.T) {
	got := elixirStringListLiteral([]string{
		"--path",
		`/tmp/a "quoted" path`,
		"line\nbreak",
	})
	want := `["--path", "/tmp/a \"quoted\" path", "line\nbreak"]`
	if got != want {
		t.Fatalf("literal = %q, want %q", got, want)
	}
}

func createCommissionDecisionWithoutAffectedFiles(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	title string,
) *artifact.Artifact {
	t.Helper()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      title + " problem",
		Signal:     "Harness should not infer repository-wide scope.",
		Acceptance: "Unscoped decisions are skipped by default harness selection.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   title,
		WhySelected:     "This fixture intentionally has no affected_files.",
		SelectionPolicy: "Default harness selection requires explicit scope.",
		CounterArgument: "The harness could infer a broad scope.",
		WeakestLink:     "Broad inferred scope would expand authority silently.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Infer repository scope",
			Reason:  "It makes the authorization boundary too wide.",
		}},
		Rollback:   &artifact.RollbackSpec{Triggers: []string{"Default harness selection regresses."}},
		ValidUntil: "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	return decision
}

func overrideHarnessTestFlags() func() {
	oldHarnessPlanID := harnessPlanID
	oldHarnessPlanTitle := harnessPlanTitle
	oldHarnessPlanRevision := harnessPlanRevision
	oldHarnessPlanSequential := harnessPlanSequential
	oldHarnessPlanDependencies := harnessPlanDependencies
	oldHarnessPlanProblems := harnessPlanProblems
	oldHarnessPlanContext := harnessPlanContext
	oldHarnessPlanAllActive := harnessPlanAllActive
	oldCommissionRepoRef := commissionFromDecisionRepoRef
	oldCommissionBaseSHA := commissionFromDecisionBaseSHA
	oldCommissionTargetBranch := commissionFromDecisionTargetBranch
	oldCommissionAllowedPaths := commissionFromDecisionAllowedPaths
	oldCommissionForbiddenPaths := commissionFromDecisionForbiddenPaths
	oldCommissionAllowedActions := commissionFromDecisionAllowedActions
	oldCommissionAffectedFiles := commissionFromDecisionAffectedFiles
	oldCommissionAllowedModules := commissionFromDecisionAllowedModules
	oldCommissionLockset := commissionFromDecisionLockset
	oldCommissionEvidence := commissionFromDecisionEvidence
	oldCommissionProjectionPolicy := commissionFromDecisionProjectionPolicy
	oldCommissionState := commissionFromDecisionState
	oldCommissionValidFor := commissionFromDecisionValidFor
	oldCommissionValidUntil := commissionFromDecisionValidUntil

	harnessPlanID = ""
	harnessPlanTitle = ""
	harnessPlanRevision = "p1"
	harnessPlanSequential = false
	harnessPlanDependencies = nil
	harnessPlanProblems = nil
	harnessPlanContext = ""
	harnessPlanAllActive = false
	commissionFromDecisionRepoRef = ""
	commissionFromDecisionBaseSHA = ""
	commissionFromDecisionTargetBranch = ""
	commissionFromDecisionAllowedPaths = nil
	commissionFromDecisionForbiddenPaths = nil
	commissionFromDecisionAllowedActions = []string{"edit_files", "run_tests"}
	commissionFromDecisionAffectedFiles = nil
	commissionFromDecisionAllowedModules = nil
	commissionFromDecisionLockset = nil
	commissionFromDecisionEvidence = nil
	commissionFromDecisionProjectionPolicy = "local_only"
	commissionFromDecisionState = "queued"
	commissionFromDecisionValidFor = "168h"
	commissionFromDecisionValidUntil = ""

	return func() {
		harnessPlanID = oldHarnessPlanID
		harnessPlanTitle = oldHarnessPlanTitle
		harnessPlanRevision = oldHarnessPlanRevision
		harnessPlanSequential = oldHarnessPlanSequential
		harnessPlanDependencies = oldHarnessPlanDependencies
		harnessPlanProblems = oldHarnessPlanProblems
		harnessPlanContext = oldHarnessPlanContext
		harnessPlanAllActive = oldHarnessPlanAllActive
		commissionFromDecisionRepoRef = oldCommissionRepoRef
		commissionFromDecisionBaseSHA = oldCommissionBaseSHA
		commissionFromDecisionTargetBranch = oldCommissionTargetBranch
		commissionFromDecisionAllowedPaths = oldCommissionAllowedPaths
		commissionFromDecisionForbiddenPaths = oldCommissionForbiddenPaths
		commissionFromDecisionAllowedActions = oldCommissionAllowedActions
		commissionFromDecisionAffectedFiles = oldCommissionAffectedFiles
		commissionFromDecisionAllowedModules = oldCommissionAllowedModules
		commissionFromDecisionLockset = oldCommissionLockset
		commissionFromDecisionEvidence = oldCommissionEvidence
		commissionFromDecisionProjectionPolicy = oldCommissionProjectionPolicy
		commissionFromDecisionState = oldCommissionState
		commissionFromDecisionValidFor = oldCommissionValidFor
		commissionFromDecisionValidUntil = oldCommissionValidUntil
	}
}
