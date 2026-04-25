package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/spf13/cobra"
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
	if plan.DeliveryPolicy != defaultDeliveryPolicy {
		t.Fatalf("delivery policy = %q, want %s", plan.DeliveryPolicy, defaultDeliveryPolicy)
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

func TestHarnessRunReadinessGateCoversReadyNeedsInitNeedsOnboard(t *testing.T) {
	cases := []struct {
		name            string
		facts           project.ReadinessFacts
		overrideReason  string
		wantKind        harnessRunReadinessKind
		wantReasonPiece string
	}{
		{
			name:     "ready",
			facts:    project.ReadinessFacts{Status: project.ReadinessReady, Exists: true, HasHaft: true, HasSpecs: true},
			wantKind: harnessRunReadinessAdmissible,
		},
		{
			name:            "needs_init",
			facts:           project.ReadinessFacts{Status: project.ReadinessNeedsInit, Exists: true},
			wantKind:        harnessRunReadinessBlocked,
			wantReasonPiece: "haft init",
		},
		{
			name:            "needs_onboard",
			facts:           project.ReadinessFacts{Status: project.ReadinessNeedsOnboard, Exists: true, HasHaft: true},
			wantKind:        harnessRunReadinessBlocked,
			wantReasonPiece: "ProjectSpecificationSet",
		},
		{
			name:           "needs_onboard_with_tactical_override",
			facts:          project.ReadinessFacts{Status: project.ReadinessNeedsOnboard, Exists: true, HasHaft: true},
			overrideReason: "legacy bootstrap",
			wantKind:       harnessRunReadinessTacticalAllowed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := harnessRunReadinessGateFor(tc.facts, tc.overrideReason)
			if got.Kind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if tc.wantReasonPiece == "" {
				return
			}
			if !strings.Contains(got.BlockReason, tc.wantReasonPiece) {
				t.Fatalf("reason = %q, want fragment %q", got.BlockReason, tc.wantReasonPiece)
			}
		})
	}
}

func TestRunHarnessRunBlocksNeedsOnboardWithClearReason(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(haftDir, "project.yaml"), []byte("id: qnt_test\nname: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()

	err := runHarnessRun(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("error = nil, want needs_onboard block")
	}

	message := err.Error()
	for _, fragment := range []string{"needs_onboard", "ProjectSpecificationSet", "--tactical-override-reason"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("block reason missing %q:\n%s", fragment, message)
		}
	}
}

func TestRunHarnessRunBlocksNeedsInitWithClearReason(t *testing.T) {
	restore := overrideHarnessTestFlags()
	defer restore()

	root := t.TempDir()
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()

	err := runHarnessRun(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("error = nil, want needs_init block")
	}

	message := err.Error()
	for _, fragment := range []string{"needs_init", "haft init"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("block reason missing %q:\n%s", fragment, message)
		}
	}
}

func TestRecordHarnessRunTacticalOverrideAnnotatesCommission(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	commissionID := "wc-tactical-override"

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture(commissionID, "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	gate := harnessRunReadinessGateFor(
		project.ReadinessFacts{Status: project.ReadinessNeedsOnboard, Exists: true, HasHaft: true},
		"incident bootstrap",
	)
	err = recordHarnessRunTacticalOverride(
		ctx,
		store,
		harnessRunSelection{CommissionIDs: []string{commissionID}},
		gate,
	)
	if err != nil {
		t.Fatal(err)
	}

	stored, err := loadWorkCommissionPayload(ctx, store, commissionID)
	if err != nil {
		t.Fatal(err)
	}

	override := mapField(stored, "spec_readiness_override")
	if override["kind"] != "tactical" {
		t.Fatalf("override kind = %#v, want tactical", override["kind"])
	}
	if override["out_of_spec"] != true {
		t.Fatalf("override out_of_spec = %#v, want true", override["out_of_spec"])
	}
	if override["project_readiness"] != string(project.ReadinessNeedsOnboard) {
		t.Fatalf("override readiness = %#v, want needs_onboard", override["project_readiness"])
	}
	if override["reason"] != "incident bootstrap" {
		t.Fatalf("override reason = %#v, want incident bootstrap", override["reason"])
	}

	events := mapSliceField(stored, "events")
	if len(events) == 0 {
		t.Fatal("events = empty, want tactical override audit event")
	}
	last := events[len(events)-1]
	if last["event"] != "tactical_override" {
		t.Fatalf("last event = %#v, want tactical_override", last["event"])
	}
	if last["reason"] != "incident bootstrap" {
		t.Fatalf("last reason = %#v, want incident bootstrap", last["reason"])
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

	created, result, err := ensureHarnessCommissions(
		ctx,
		store,
		t.TempDir(),
		map[string]any{
			"id":       "plan-existing",
			"revision": "p1",
		},
		harnessRunReadinessGate{},
	)
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

	gotPath, plan, result, selection, found, err := existingRunnableHarnessPlan(ctx, store, projectRoot)
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
	if len(selection.CommissionIDs) != 1 || selection.CommissionIDs[0] != "wc-existing-runnable" {
		t.Fatalf("selection commissions = %#v, want [wc-existing-runnable]", selection.CommissionIDs)
	}
	if len(selection.DecisionRefs) != 1 || selection.DecisionRefs[0] != "dec-20260422-001" {
		t.Fatalf("selection decisions = %#v, want [dec-20260422-001]", selection.DecisionRefs)
	}
}

func TestPrintHarnessRunSummaryIncludesSelectedCommissionAndObservationCommands(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := printHarnessRunSummary(
		cmd,
		"/tmp/.haft/plans/plan.yaml",
		"/tmp/sleigh.md",
		false,
		"using 1 existing runnable commission(s)",
		harnessRunSelection{
			CommissionIDs: []string{"wc-1"},
			DecisionRefs:  []string{"dec-1"},
		},
		harnessRunOptions{
			StatusPath:    "/tmp/status.json",
			LogPath:       "/tmp/runtime.jsonl",
			WorkspaceRoot: "/tmp/workspaces",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	joined := out.String()
	for _, fragment := range []string{
		"Commissions: using 1 existing runnable commission(s)",
		"Selected commission: wc-1",
		"Selected decision: dec-1",
		"Operator stream: live in this terminal",
		"Stop: Ctrl-C stops the stream and the Open-Sleigh process",
		"Observe result: haft harness result wc-1",
		"Workspace: /tmp/workspaces/wc-1",
		"workspace changes usually appear only after execute starts editing files",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("summary output missing %q:\n%s", fragment, joined)
		}
	}
}

func TestHarnessRunObservationLinesKeepsCommandsForDetachedRun(t *testing.T) {
	lines := harnessRunObservationLines(harnessRunOptions{
		Detach:  true,
		LogPath: "/tmp/runtime.jsonl",
	})
	joined := strings.Join(lines, "\n")

	for _, fragment := range []string{
		"Observe status: haft harness status --tail 20",
		"Observe log: tail -f /tmp/runtime.jsonl",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("detached observation output missing %q:\n%s", fragment, joined)
		}
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

	lines := formatHarnessStatus(status, "/tmp/status.json", logPath, harnessSessionLogSummaries(logPath), nil)
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

func TestFormatHarnessStatusIncludesRecentTerminalCommissions(t *testing.T) {
	status := map[string]any{
		"updated_at": "2026-04-24T05:11:34Z",
		"metadata": map[string]any{
			"agent_kind":     "codex",
			"tracker_kind":   "commission_source:haft",
			"config_path":    "/tmp/sleigh.md",
			"workspace_root": "/tmp/workspaces",
		},
		"orchestrator": map[string]any{
			"claimed":         []any{},
			"running":         []any{},
			"pending_human":   []any{},
			"running_details": []any{},
		},
		"failures": []any{},
	}

	lines := formatHarnessStatus(
		status,
		"/tmp/status.json",
		"/tmp/runtime.jsonl",
		nil,
		[]harnessTerminalCommissionSummary{
			{
				CommissionID: "wc-1",
				State:        "completed",
				DecisionRef:  "dec-1",
				LastEvent:    "workflow_terminal",
				LastVerdict:  "pass",
				RecordedAt:   "2026-04-24T05:08:35Z",
				Workspace:    "/tmp/workspaces/wc-1",
				Preview:      "Measurement pass: tests completed.",
			},
		},
	)
	joined := strings.Join(lines, "\n")

	for _, fragment := range []string{
		"recent_terminal:",
		"commission=wc-1 state=completed decision=dec-1 last_event=workflow_terminal verdict=pass",
		"result=haft harness result wc-1",
		"tail=haft harness tail wc-1",
		"terminal_next=inspect result, then apply or rerun evidence",
		"workspace=/tmp/workspaces/wc-1",
		"preview=Measurement pass: tests completed.",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("status output missing %q:\n%s", fragment, joined)
		}
	}
}

func TestReadHarnessStatusMissingFileReturnsUnavailableDashboard(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "missing-status.json")

	_, status, err := readHarnessStatus(statusPath)
	if err != nil {
		t.Fatal(err)
	}

	lines := formatHarnessStatus(status, statusPath, "/tmp/missing-runtime.jsonl", nil, nil)
	joined := strings.Join(lines, "\n")
	for _, fragment := range []string{
		"runtime_state: unavailable",
		"operator_next:",
		"no active harness run detected",
		"create a commission: haft commission create-from-decision <decision-id>",
		"create a plan and commissions: haft harness run <decision-id> --prepare-only",
		"run queued commissions: haft harness run",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("missing status dashboard lacks %q:\n%s", fragment, joined)
		}
	}
}

func TestReadHarnessStatusRetriesPartialWrite(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(statusPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(statusPath, []byte(`{"updated_at":"2026-04-24T08:03:39Z","orchestrator":{}}`), 0o644)
	}()

	_, status, err := readHarnessStatus(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if stringField(status, "updated_at") != "2026-04-24T08:03:39Z" {
		t.Fatalf("status = %#v, want rewritten status", status)
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

func TestFormatHarnessResultShowsCompletedDiffEvidenceAndApplyAction(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspaces")
	commissionID := "wc-result-success"
	trackedPath := filepath.Join("internal", "cli", "harness.go")
	workspacePath := filepath.Join(workspaceRoot, commissionID)

	initHarnessApplyRepo(t, workspacePath, trackedPath, "package cli\n\nconst value = \"old\"\n")
	if err := os.WriteFile(filepath.Join(workspacePath, trackedPath), []byte("package cli\n\nconst value = \"new\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	commission := workCommissionFixture(commissionID, "completed", "2099-01-01T00:00:00Z")
	commission["events"] = []any{
		map[string]any{
			"event":       "phase_outcome",
			"verdict":     "pass",
			"recorded_at": "2026-04-24T05:08:35Z",
			"payload": map[string]any{
				"phase": "measure",
				"next":  "terminal:pass",
			},
		},
	}

	lines := formatHarnessResult(
		commission,
		workspaceRoot,
		nil,
		"",
		harnessSessionLogSummary{},
		harnessCommissionLogSummary{},
	)
	joined := strings.Join(lines, "\n")

	for _, fragment := range []string{
		"state: completed",
		"workspace: " + workspacePath,
		"evidence_summary:",
		"required=1",
		"requirement: kind=go_test command=go test ./...",
		"latest_measure: verdict=pass next=terminal:pass",
		"diff_status: changed",
		"operator_next:",
		"next_action=apply",
		"haft harness apply " + commissionID,
		"evidence: go test ./...",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("result output missing %q:\n%s", fragment, joined)
		}
	}
	if strings.Contains(joined, `{"`) {
		t.Fatalf("result output leaked raw JSON:\n%s", joined)
	}
}

func TestFormatHarnessResultShowsBlockedPolicyOperatorActions(t *testing.T) {
	commissionID := "wc-result-blocked"
	commission := workCommissionFixture(commissionID, "blocked_policy", "2099-01-01T00:00:00Z")
	commission["events"] = []any{
		map[string]any{
			"event":       "phase_blocked",
			"action":      "complete_or_block",
			"verdict":     "blocked",
			"reason":      "mutation_outside_commission_scope",
			"recorded_at": "2026-04-24T09:57:53Z",
			"payload": map[string]any{
				"phase":              "preflight",
				"out_of_scope_paths": []any{"outside.txt"},
			},
		},
	}

	lines := formatHarnessResult(
		commission,
		t.TempDir(),
		nil,
		"",
		harnessSessionLogSummary{},
		harnessCommissionLogSummary{},
	)
	joined := strings.Join(lines, "\n")

	for _, fragment := range []string{
		"state: blocked_policy",
		"diff_status: workspace_unavailable",
		"evidence_summary:",
		"last_event: phase_blocked",
		"out_of_scope=outside.txt",
		"next_action=inspect",
		"requires operator decision: blocked_policy",
		"haft commission requeue " + commissionID,
		"haft commission cancel " + commissionID,
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("blocked result output missing %q:\n%s", fragment, joined)
		}
	}
	if strings.Contains(joined, `{"`) {
		t.Fatalf("blocked result output leaked raw JSON:\n%s", joined)
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

func TestFormatHarnessResultEventsShowsOutOfScopePaths(t *testing.T) {
	lines := formatHarnessResultEvents([]map[string]any{
		{
			"event":       "phase_blocked",
			"action":      "complete_or_block",
			"verdict":     "blocked",
			"reason":      "mutation_outside_commission_scope",
			"recorded_at": "2026-04-24T09:57:53Z",
			"payload": map[string]any{
				"phase":              "preflight",
				"out_of_scope_paths": []any{"erl_crash.dump"},
			},
		},
	})
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "out_of_scope=erl_crash.dump") {
		t.Fatalf("result output missing out-of-scope paths:\n%s", joined)
	}
}

func TestHarnessRuntimeCrashDumpPathExpandsEnvDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(t.TempDir(), "crash_dumps")
	relativeRoot, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPEN_SLEIGH_CRASH_DUMP_DIR", relativeRoot)

	path := harnessRuntimeCrashDumpPath()
	if !filepath.IsAbs(path) {
		t.Fatalf("crash dump path = %s, want absolute", path)
	}
	if filepath.Dir(path) != root {
		t.Fatalf("crash dump dir = %s, want %s", filepath.Dir(path), root)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("crash dump dir not created: %v", err)
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

func TestRecentHarnessEventLinesHumanizesCurrentRun(t *testing.T) {
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

	writeHarnessRuntimeEvents(t, logPath, []map[string]any{
		{
			"at":            "2026-04-24T05:01:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-current",
			"metadata":      map[string]any{"config_path": "/tmp/current-sleigh.md"},
			"data": map[string]any{
				"phase":        "execute",
				"session_id":   "session-1",
				"turn_id":      "turn-1",
				"status":       "completed",
				"text_preview": "Implemented the scoped desktop RPC change.",
			},
		},
	})

	lines, err := recentHarnessEventLines(status, logPath, 10)
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, `{"`) {
		t.Fatalf("event lines leaked raw JSON:\n%s", joined)
	}
	for _, fragment := range []string{
		"2026-04-24T05:01:00Z agent_completed: commission=wc-current phase=execute status=completed session=session-1 turn=turn-1",
		"preview=Implemented the scoped desktop RPC change.",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("event lines missing %q:\n%s", fragment, joined)
		}
	}
}

func TestRecentHarnessLogLinesMissingFileReturnsEmpty(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "missing-runtime.jsonl")

	lines, err := recentHarnessLogLines(map[string]any{}, logPath, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("lines = %#v, want empty", lines)
	}
}

func TestRecentHarnessEventLinesShowsEmptyRuntimeLog(t *testing.T) {
	lines, err := recentHarnessEventLines(map[string]any{}, filepath.Join(t.TempDir(), "missing-runtime.jsonl"), 10)
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, `{"`) {
		t.Fatalf("empty event lines leaked raw JSON:\n%s", joined)
	}
	if !strings.Contains(joined, "no operator runtime events yet") {
		t.Fatalf("empty event lines missing operator message:\n%s", joined)
	}
}

func TestPrintHarnessSelectedTailSincePrintsOnlySelectedHumanizedEvents(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime.jsonl")
	writeHarnessRuntimeEvents(t, logPath, []map[string]any{
		{
			"at":            "2026-04-24T05:00:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-other",
			"data": map[string]any{
				"phase":        "execute",
				"text_preview": "other commission",
			},
		},
		{
			"at":            "2026-04-24T05:01:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-selected",
			"data": map[string]any{
				"phase":        "execute",
				"status":       "completed",
				"text_preview": "selected commission changed only scoped files",
			},
		},
	})

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	offset, printed, err := printHarnessSelectedTailSince(cmd, logPath, []string{"wc-selected"}, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	joined := out.String()
	if offset != 2 {
		t.Fatalf("offset = %d, want 2", offset)
	}
	if printed != 1 {
		t.Fatalf("printed = %d, want 1", printed)
	}
	if strings.Contains(joined, "other commission") {
		t.Fatalf("selected tail leaked other commission:\n%s", joined)
	}
	if strings.Contains(joined, `{"`) {
		t.Fatalf("selected tail leaked raw JSON:\n%s", joined)
	}
	if !strings.Contains(joined, "selected commission changed only scoped files") {
		t.Fatalf("selected tail missing selected preview:\n%s", joined)
	}
}

func TestPrintHarnessSelectedTailSinceShowsFailedResetLoop(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime.jsonl")
	writeHarnessRuntimeEvents(t, logPath, []map[string]any{
		{
			"at":            "2026-04-24T05:00:00Z",
			"event":         "workspace_reset_failed",
			"commission_id": "wc-reset",
			"data": map[string]any{
				"phase":          "preflight",
				"session_id":     "session-reset",
				"workspace_path": "/tmp/workspaces/wc-reset",
				"reason":         "dirty_workspace",
			},
		},
		{
			"at":            "2026-04-24T05:00:01Z",
			"event":         "session_failed",
			"commission_id": "wc-reset",
			"data": map[string]any{
				"phase":      "preflight",
				"session_id": "session-reset",
				"reason":     "dirty_workspace",
			},
		},
	})

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	_, printed, err := printHarnessSelectedTailSince(cmd, logPath, []string{"wc-reset"}, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	joined := out.String()
	if printed != 2 {
		t.Fatalf("printed = %d, want 2", printed)
	}
	for _, fragment := range []string{
		"workspace_reset_failed: commission=wc-reset phase=preflight session=session-reset workspace=/tmp/workspaces/wc-reset reason=dirty_workspace",
		"failed: commission=wc-reset phase=preflight session=session-reset reason=dirty_workspace",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("reset-loop output missing %q:\n%s", fragment, joined)
		}
	}
	if strings.Contains(joined, `{"`) {
		t.Fatalf("reset-loop output leaked raw JSON:\n%s", joined)
	}
}

func TestHarnessTailFollowCompletionLineShowsTerminalSuccess(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)

	commissionID := "wc-tail-completed"
	trackedPath := filepath.Join("internal", "cli", "harness.go")
	workspacePath := filepath.Join(defaultHarnessWorkspaceRoot(), commissionID)
	initHarnessApplyRepo(t, workspacePath, trackedPath, "package cli\n\nconst value = \"old\"\n")
	if err := os.WriteFile(filepath.Join(workspacePath, trackedPath), []byte("package cli\n\nconst value = \"new\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := persistWorkCommission(ctx, store, workCommissionFixture(commissionID, "completed", "2099-01-01T00:00:00Z"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	line, terminal, err := harnessTailFollowCompletionLine(ctx, store, commissionID)
	if err != nil {
		t.Fatal(err)
	}
	if !terminal {
		t.Fatal("terminal = false, want true")
	}
	for _, fragment := range []string{
		"terminal:",
		"commission=" + commissionID,
		"state=completed",
		"next_action=apply",
		"result=haft harness result " + commissionID,
	} {
		if !strings.Contains(line, fragment) {
			t.Fatalf("completion line missing %q: %s", fragment, line)
		}
	}
}

func TestPrintHarnessTailSnapshotFiltersAndHumanizesCommissionEvents(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "runtime.jsonl")
	events := []map[string]any{
		{
			"at":            "2026-04-24T05:00:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-other",
			"data": map[string]any{
				"phase":        "execute",
				"status":       "completed",
				"text_preview": "other commission",
			},
		},
		{
			"at":            "2026-04-24T05:01:00Z",
			"event":         "agent_turn_completed",
			"commission_id": "wc-1",
			"data": map[string]any{
				"phase":        "execute",
				"session_id":   "session-1",
				"turn_id":      "turn-1",
				"status":       "completed",
				"text_preview": "Implemented the scoped MCP config portability change.",
			},
		},
	}
	writeHarnessRuntimeEvents(t, logPath, events)

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	offset, err := printHarnessTailSnapshot(cmd, logPath, "wc-1", false)
	if err != nil {
		t.Fatal(err)
	}

	joined := out.String()
	if offset != 2 {
		t.Fatalf("offset = %d, want 2", offset)
	}
	if strings.Contains(joined, "other commission") {
		t.Fatalf("tail output leaked other commission:\n%s", joined)
	}
	for _, fragment := range []string{
		"2026-04-24T05:01:00Z agent_completed: commission=wc-1 phase=execute status=completed session=session-1 turn=turn-1",
		"preview=Implemented the scoped MCP config portability change.",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("tail output missing %q:\n%s", fragment, joined)
		}
	}
}

func TestPrintHarnessTailSnapshotShowsEmptyState(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	offset, err := printHarnessTailSnapshot(cmd, filepath.Join(t.TempDir(), "missing.jsonl"), "wc-empty", false)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Fatalf("offset = %d, want 0", offset)
	}

	joined := out.String()
	if !strings.Contains(joined, "No runtime events for commission wc-empty yet") {
		t.Fatalf("tail empty state missing message:\n%s", joined)
	}
	if !strings.Contains(joined, "haft harness tail wc-empty --follow") {
		t.Fatalf("tail empty state missing follow command:\n%s", joined)
	}
}

func TestApplyHarnessWorkspaceDiffAppliesScopedTrackedDiff(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	workspaceRoot := filepath.Join(root, "workspace")
	trackedPath := filepath.Join("internal", "cli", "init.go")

	initHarnessApplyRepo(t, projectRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")
	initHarnessApplyRepo(t, workspaceRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")

	updated := "package cli\n\nconst value = \"new\"\n"
	if err := os.WriteFile(filepath.Join(workspaceRoot, trackedPath), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := applyHarnessWorkspaceDiff(projectRoot, workspaceRoot, []string{trackedPath})
	if err != nil {
		t.Fatal(err)
	}

	if len(summary.Files) != 1 || summary.Files[0] != trackedPath {
		t.Fatalf("summary files = %#v, want [%s]", summary.Files, trackedPath)
	}

	got, err := os.ReadFile(filepath.Join(projectRoot, trackedPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != updated {
		t.Fatalf("applied file = %q, want %q", string(got), updated)
	}
}

func TestApplyHarnessWorkspaceDiffAppliesScopedUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	workspaceRoot := filepath.Join(root, "workspace")
	trackedPath := filepath.Join("internal", "cli", "init.go")
	newPath := filepath.Join("internal", "cli", "desktop_rpc_handlers_test.go")

	initHarnessApplyRepo(t, projectRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")
	initHarnessApplyRepo(t, workspaceRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")

	newContent := "package cli\n\nfunc TestGenerated(t *testing.T) {}\n"
	if err := os.MkdirAll(filepath.Join(workspaceRoot, filepath.Dir(newPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, newPath), []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := applyHarnessWorkspaceDiff(projectRoot, workspaceRoot, []string{newPath})
	if err != nil {
		t.Fatal(err)
	}

	if len(summary.Files) != 1 || summary.Files[0] != newPath {
		t.Fatalf("summary files = %#v, want [%s]", summary.Files, newPath)
	}

	got, err := os.ReadFile(filepath.Join(projectRoot, newPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newContent {
		t.Fatalf("applied untracked file = %q, want %q", string(got), newContent)
	}
}

func TestApplyHarnessWorkspaceDiffRejectsOutOfScopeDiff(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	workspaceRoot := filepath.Join(root, "workspace")
	trackedPath := filepath.Join("internal", "cli", "init.go")

	initHarnessApplyRepo(t, projectRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")
	initHarnessApplyRepo(t, workspaceRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")

	if err := os.WriteFile(filepath.Join(workspaceRoot, trackedPath), []byte("package cli\n\nconst value = \"new\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := applyHarnessWorkspaceDiff(projectRoot, workspaceRoot, []string{"README.md"})
	if err == nil || !strings.Contains(err.Error(), "outside commission scope") {
		t.Fatalf("error = %v, want out-of-scope rejection", err)
	}
}

func TestDeliverHarnessRunCommissionsAppliesAutoPolicy(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	workspaceRoot := filepath.Join(root, "workspaces")
	commissionID := "wc-auto-delivery"
	trackedPath := filepath.Join("internal", "cli", "init.go")

	initHarnessApplyRepo(t, projectRoot, trackedPath, "package cli\n\nconst value = \"old\"\n")
	initHarnessApplyRepo(
		t,
		filepath.Join(workspaceRoot, commissionID),
		trackedPath,
		"package cli\n\nconst value = \"old\"\n",
	)

	updated := "package cli\n\nconst value = \"new\"\n"
	workspaceFile := filepath.Join(workspaceRoot, commissionID, trackedPath)
	if err := os.WriteFile(workspaceFile, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	commission := workCommissionFixture(commissionID, "completed", "2099-01-01T00:00:00Z")
	commission["delivery_policy"] = "workspace_patch_auto_on_pass"
	scope := mapField(commission, "scope")
	scope["allowed_paths"] = []any{trackedPath}
	scope["affected_files"] = []any{trackedPath}
	scope["lockset"] = []any{trackedPath}
	commission["lockset"] = []any{trackedPath}

	if _, err := persistWorkCommission(ctx, store, commission, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err := deliverHarnessRunCommissions(
		ctx,
		cmd,
		store,
		projectRoot,
		harnessRunSelection{CommissionIDs: []string{commissionID}},
		harnessRunOptions{WorkspaceRoot: workspaceRoot},
	)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(projectRoot, trackedPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != updated {
		t.Fatalf("delivered file = %q, want %q", string(got), updated)
	}
	if !strings.Contains(out.String(), "Applied harness workspace diff") {
		t.Fatalf("output = %q, want apply summary", out.String())
	}
}

func initHarnessApplyRepo(t *testing.T, root string, trackedPath string, content string) {
	t.Helper()

	fullPath := filepath.Join(root, trackedPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	runHarnessApplyGit(t, root, "init")
	runHarnessApplyGit(t, root, "config", "user.email", "test@example.com")
	runHarnessApplyGit(t, root, "config", "user.name", "Test User")
	runHarnessApplyGit(t, root, "add", trackedPath)
	runHarnessApplyGit(t, root, "commit", "-m", "initial")
}

func runHarnessApplyGit(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}

func writeHarnessRuntimeEvents(t *testing.T, logPath string, events []map[string]any) {
	t.Helper()

	encoded := make([]string, 0, len(events))
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, string(payload))
	}

	if err := os.WriteFile(logPath, []byte(strings.Join(encoded, "\n")), 0o644); err != nil {
		t.Fatal(err)
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
	oldHarnessRunDetach := harnessRunDetach
	oldHarnessRunTacticalReason := harnessRunTacticalReason
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
	oldCommissionDeliveryPolicy := commissionFromDecisionDeliveryPolicy
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
	harnessRunDetach = false
	harnessRunTacticalReason = ""
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
	commissionFromDecisionDeliveryPolicy = defaultDeliveryPolicy
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
		harnessRunDetach = oldHarnessRunDetach
		harnessRunTacticalReason = oldHarnessRunTacticalReason
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
		commissionFromDecisionDeliveryPolicy = oldCommissionDeliveryPolicy
		commissionFromDecisionState = oldCommissionState
		commissionFromDecisionValidFor = oldCommissionValidFor
		commissionFromDecisionValidUntil = oldCommissionValidUntil
	}
}
