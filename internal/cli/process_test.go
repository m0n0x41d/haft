package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

func TestBuildProcessTelemetryReportCoversMethodRunsSessionsAndBindingRisk(t *testing.T) {
	fixture := newCheckTestProject(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	sessionPath := writeProcessTelemetrySessionFixture(t, fixture.root)

	seedProcessMethodRun(t, fixture, methodpkg.MethodRun{
		ID:             "mpull-open-old",
		CatalogID:      methodpkg.CatalogID,
		CatalogVersion: methodpkg.CatalogVersion,
		Status:         "open",
		TaskSignature: methodpkg.TaskSignature{
			Task:               "Long open task",
			NormalizedTaskKind: "feature",
			Ceremony:           "medium",
		},
		OpenedAt: now.Add(-30 * time.Hour).Format(time.RFC3339),
		CarryThrough: []methodpkg.CarryThroughItem{{
			SourceRef:     "review:external",
			SourceItemRef: "pending-finding",
			AcceptanceRef: "operator:accepted",
			Disposition:   methodpkg.CarryDispositionPending,
		}},
	})
	seedProcessMethodRun(t, fixture, methodpkg.MethodRun{
		ID:             "mpull-closed-waived",
		CatalogID:      methodpkg.CatalogID,
		CatalogVersion: methodpkg.CatalogVersion,
		Status:         "closed",
		TaskSignature: methodpkg.TaskSignature{
			Task:               "Closed with waiver",
			NormalizedTaskKind: "feature",
			Ceremony:           "medium",
		},
		OpenedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
		ClosedAt: now.Add(-time.Hour).Format(time.RFC3339),
		Closeout: &methodpkg.Closeout{
			Verification: methodpkg.Verification{
				Commands:  []string{"go test ./internal/cli"},
				Result:    "pass",
				OutputRef: "test output",
			},
			Waivers: []methodpkg.Waiver{{
				GateID: "manual_followup",
				Reason: "operator accepted residual review",
			}},
			ClosedAt: now.Add(-time.Hour).Format(time.RFC3339),
		},
	})
	seedProcessBroadBindingDecision(t, fixture, "dec-broad", "Broad carrier decision", artifact.DecisionFields{
		SelectedTitle: "Broad carrier decision",
		WhySelected:   "Need a fixture for affected_files without semantic targets.",
	}, []artifact.AffectedFile{{Path: "internal/cli/serve.go"}})
	seedProcessBroadBindingDecision(t, fixture, "dec-footprint", "Footprint only decision", artifact.DecisionFields{
		SelectedTitle: "Footprint only decision",
		WhySelected:   "Need a fixture that should not become governance risk.",
		ImplementationFootprint: artifact.ImplementationFootprint{
			Files: []string{"internal/cli/interface.go"},
		},
	}, []artifact.AffectedFile{{Path: "internal/cli/interface.go"}})

	report, err := buildProcessTelemetryReport(context.Background(), fixture.store, fixture.root, processTelemetryOptions{
		SessionInputs: []string{sessionPath},
		Now:           now,
		LongOpenAfter: 12 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	if report.Kind != processTelemetryKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.MethodRuns.Total != 2 || report.MethodRuns.Open != 1 || report.MethodRuns.Closed != 1 {
		t.Fatalf("method run counts = %+v", report.MethodRuns)
	}
	if got := len(report.MethodRuns.LongOpen); got != 1 {
		t.Fatalf("long open = %d, want 1", got)
	}
	if got := len(report.MethodRuns.PendingCarryThrough); got != 1 {
		t.Fatalf("pending carry-through runs = %d, want 1", got)
	}
	if got := len(report.MethodRuns.ClosedWithWaivers); got != 1 {
		t.Fatalf("closed with waivers = %d, want 1", got)
	}
	if got := len(report.BroadBindingRisk.Risks); got != 1 {
		t.Fatalf("broad binding risks = %d, want 1: %+v", got, report.BroadBindingRisk)
	}
	if report.BroadBindingRisk.ImplementationFootprintOnly != 1 {
		t.Fatalf("implementation footprint only = %d, want 1", report.BroadBindingRisk.ImplementationFootprintOnly)
	}

	if got := len(report.Sessions); got != 1 {
		t.Fatalf("sessions = %d, want 1", got)
	}
	session := report.Sessions[0]
	if session.MethodActionCounts["pull"] != 2 || session.MethodActionCounts["close"] != 3 {
		t.Fatalf("method action counts = %+v", session.MethodActionCounts)
	}
	if got := len(session.DuplicatePullTasks); got != 1 {
		t.Fatalf("duplicate pull tasks = %d, want 1", got)
	}
	if got := len(session.DuplicateCloseIDs); got != 1 {
		t.Fatalf("duplicate close ids = %d, want 1", got)
	}
	if session.CloseFailures != 1 {
		t.Fatalf("close failures = %d, want 1", session.CloseFailures)
	}
	if session.ClosesWithWaivers != 1 {
		t.Fatalf("closes with waivers = %d, want 1", session.ClosesWithWaivers)
	}
	if session.WaiverItemsOnClose != 1 {
		t.Fatalf("waiver items on close = %d, want 1", session.WaiverItemsOnClose)
	}
	if session.CloseRepairRounds != 1 {
		t.Fatalf("close repair rounds = %d, want 1", session.CloseRepairRounds)
	}
	if session.CarryThroughItemsOnPull != 1 || session.CarryThroughItemsOnClose != 1 {
		t.Fatalf("carry-through item counts pull=%d close=%d, want 1/1", session.CarryThroughItemsOnPull, session.CarryThroughItemsOnClose)
	}
	if session.MissingGateResultsOnClose != 1 {
		t.Fatalf("missing gate results = %d, want 1", session.MissingGateResultsOnClose)
	}
	if session.CloseWithoutPriorPull != 1 {
		t.Fatalf("close without pull = %d, want 1", session.CloseWithoutPriorPull)
	}
	if session.InvocationLanes.CLIRunCommands != 1 || session.InvocationLanes.CLIHelpOnlyCommands != 1 {
		t.Fatalf("invocation lanes = %+v", session.InvocationLanes)
	}
	if session.PullToCloseMinutes.Count != 1 || session.PullToCloseMinutes.Max != 5.0 {
		t.Fatalf("duration stats = %+v, want one 5.0 minute pair", session.PullToCloseMinutes)
	}
	if report.OperatorBurden.AuthorityBoundary == "" {
		t.Fatal("operator burden proxy authority boundary missing")
	}
	if !strings.Contains(strings.Join(report.OperatorBurden.SourcePolicy, " "), "no_arbitrary_prose_classification") {
		t.Fatalf("operator burden source policy = %#v", report.OperatorBurden.SourcePolicy)
	}
	if report.OperatorBurden.SessionWaiverItems != 1 ||
		report.OperatorBurden.SessionCloseRepairRounds != 1 ||
		report.OperatorBurden.SessionCarryThroughItems != 2 ||
		report.OperatorBurden.MethodRunsPendingCarryThroughItems != 1 {
		t.Fatalf("operator burden proxies = %#v", report.OperatorBurden)
	}
	if report.Summary.SessionWaiverItems != 1 ||
		report.Summary.SessionCloseRepairRounds != 1 ||
		report.Summary.SessionCarryThroughItems != 2 ||
		report.Summary.MethodRunsPendingCarryThroughItems != 1 {
		t.Fatalf("summary operator burden counts = %#v", report.Summary)
	}
}

func TestRunProcessTelemetryJSON(t *testing.T) {
	fixture := newCheckTestProject(t)
	sessionPath := writeProcessTelemetrySessionFixture(t, fixture.root)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	oldJSON := processTelemetryJSON
	oldSessionInputs := processTelemetrySessionInputs
	oldLongOpenHours := processTelemetryLongOpenHours
	t.Cleanup(func() {
		processTelemetryJSON = oldJSON
		processTelemetrySessionInputs = oldSessionInputs
		processTelemetryLongOpenHours = oldLongOpenHours
	})

	processTelemetryJSON = true
	processTelemetrySessionInputs = []string{sessionPath}
	processTelemetryLongOpenHours = 1

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runProcessTelemetry(cmd, nil); err != nil {
		t.Fatal(err)
	}

	report := processTelemetryReport{}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, output.String())
	}
	if report.Kind != processTelemetryKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Summary.SessionMethodPulls != 2 {
		t.Fatalf("session pulls = %d, want 2", report.Summary.SessionMethodPulls)
	}
	if strings.Contains(output.String(), "transcript body that should not be in telemetry") {
		t.Fatalf("telemetry output inlined transcript prose:\n%s", output.String())
	}
}

func TestBuildProcessCheckReportCoreReturnsStableResults(t *testing.T) {
	fixture := newCheckTestProject(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	seedProcessMethodRun(t, fixture, validProcessMethodRunFixture(now))

	report, err := buildProcessCheckReport(context.Background(), fixture.store, fixture.root, processCheckOptions{
		Profile: "core",
		Now:     now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Kind != processCheckKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Authority != processCheckAuthority {
		t.Fatalf("authority = %q", report.Authority)
	}
	if len(report.Results) != 5 {
		t.Fatalf("results = %d, want 5: %#v", len(report.Results), report.Results)
	}
	if report.Summary.Total != 5 {
		t.Fatalf("summary total = %d, want 5", report.Summary.Total)
	}
	for _, result := range report.Results {
		if result.CheckID == "" || result.CheckVersion != "v0" {
			t.Fatalf("unstable check identity: %#v", result)
		}
		if result.NextAction == "" {
			t.Fatalf("next_action missing: %#v", result)
		}
		if !strings.Contains(result.AuthorityBoundary, "not_approval") ||
			!strings.Contains(result.AuthorityBoundary, "not_operator_authorization") {
			t.Fatalf("authority boundary missing: %#v", result)
		}
	}
	for _, want := range []string{
		"method_run_hard_gates",
		"generated_contract_runtime_schema",
		"binding_actions_fail_closed",
		"default_status_compact",
		"interface_discovery_compact",
	} {
		if !processCheckTestHasResult(report.Results, want) {
			t.Fatalf("missing check %q in %#v", want, report.Results)
		}
	}
	statusResult := processCheckTestResult(t, report.Results, "default_status_compact")
	statusEvidence := strings.Join(statusResult.EvidenceRefs, " ")
	for _, want := range []string{"bytes=", "action_lines="} {
		if !strings.Contains(statusEvidence, want) {
			t.Fatalf("default status compact evidence missing %q: %#v", want, statusResult)
		}
	}
}

func TestBuildProcessCheckReportDetectsMethodRunHardGateGap(t *testing.T) {
	fixture := newCheckTestProject(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	run := validProcessMethodRunFixture(now)
	run.ID = "mpull-missing-gate-evidence"
	run.Closeout.GateResults = nil
	seedProcessMethodRun(t, fixture, run)

	report, err := buildProcessCheckReport(context.Background(), fixture.store, fixture.root, processCheckOptions{
		Profile: "core",
		Now:     now,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(t, report.Results, "method_run_hard_gates")
	if result.Status != processCheckStatusFail {
		t.Fatalf("status = %q, want fail: %#v", result.Status, result)
	}
	if !strings.Contains(result.Finding, "hard-gate closeout gaps") {
		t.Fatalf("finding should name hard-gate gaps: %#v", result)
	}
	if len(result.EvidenceRefs) == 0 || !strings.Contains(strings.Join(result.EvidenceRefs, " "), run.ID) {
		t.Fatalf("evidence refs should name failing run: %#v", result)
	}
}

func TestBuildProcessCheckReportReplaysCarryThroughCloseout(t *testing.T) {
	fixture := newCheckTestProject(t)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	run := validProcessMethodRunFixture(now)
	run.ID = "mpull-applied-carry-through"
	run.CarryThrough = []methodpkg.CarryThroughItem{{
		SourceRef:     "review:external",
		SourceItemRef: "finding-1",
		AcceptanceRef: "operator:accepted",
		Disposition:   methodpkg.CarryDispositionPending,
	}}
	run.Closeout.CarryThrough = []methodpkg.CarryThroughItem{{
		SourceRef:     "review:external",
		SourceItemRef: "finding-1",
		AcceptanceRef: "operator:accepted",
		Disposition:   methodpkg.CarryDispositionApplied,
		TargetRefs:    []string{"internal/cli/process.go::processMethodRunHardGateIssues"},
		EvidenceRefs:  []string{"go test ./internal/cli -run TestBuildProcessCheckReportReplaysCarryThroughCloseout"},
		UpdatedAt:     run.Closeout.ClosedAt,
	}}
	seedProcessMethodRun(t, fixture, run)

	report, err := buildProcessCheckReport(context.Background(), fixture.store, fixture.root, processCheckOptions{
		Profile: "core",
		Now:     now,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(t, report.Results, "method_run_hard_gates")
	if result.Status != processCheckStatusPass {
		t.Fatalf("status = %q, want pass: %#v", result.Status, result)
	}
	if strings.Contains(strings.Join(result.EvidenceRefs, " "), run.ID) {
		t.Fatalf("valid carry-through closeout should not appear as failure evidence: %#v", result)
	}
}

func TestProcessCheckClosureDisciplineCarryThroughEndToEnd(t *testing.T) {
	fixture := newCheckTestProject(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	_, ref, err := handleHaftMethod(ctx, fixture.store, fixture.haftDir, map[string]any{
		"action":             "pull",
		"task":               "Apply accepted external review finding",
		"declared_task_kind": "bugfix",
		"change_intent":      "fix_bug",
		"carry_through": []any{map[string]any{
			"source_ref":      "review:external",
			"source_item_ref": "finding-1",
			"acceptance_ref":  "operator:accepted",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	run := decodeStoredMethodRun(t, ctx, fixture.store, ref)
	gateResults := methodGateResultsWithEvidence(run, "test:closure-discipline-e2e")
	_, _, err = handleHaftMethod(ctx, fixture.store, fixture.haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"gate_results":  gateResults,
		"verification":  map[string]any{"result": "pass"},
		"changed_files": []any{"internal/cli/process.go"},
	})
	if err == nil {
		t.Fatal("close accepted missing carry-through disposition")
	}
	if !strings.Contains(err.Error(), "carry_through close disposition") {
		t.Fatalf("close error = %v, want carry-through disposition failure", err)
	}

	_, _, err = handleHaftMethod(ctx, fixture.store, fixture.haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"gate_results":  gateResults,
		"verification":  map[string]any{"result": "pass"},
		"changed_files": []any{"internal/cli/process.go"},
		"carry_through": []any{map[string]any{
			"source_ref":      "review:external",
			"source_item_ref": "finding-1",
			"acceptance_ref":  "operator:accepted",
			"disposition":     "applied",
			"target_refs":     []any{"internal/cli/process.go::processMethodRunHardGateIssues"},
			"evidence_refs":   []any{"go test ./internal/cli -run TestProcessCheckClosureDisciplineCarryThroughEndToEnd"},
		}},
	})
	if err != nil {
		t.Fatalf("close rejected applied carry-through disposition: %v", err)
	}

	report, err := buildProcessCheckReport(ctx, fixture.store, fixture.root, processCheckOptions{
		Profile: "core",
		Now:     now,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := processCheckTestResult(t, report.Results, "method_run_hard_gates")
	if result.Status != processCheckStatusPass {
		t.Fatalf("status = %q, want pass: %#v", result.Status, result)
	}
	if strings.Contains(strings.Join(result.EvidenceRefs, " "), ref) {
		t.Fatalf("closed carry-through run should not appear as failure evidence: %#v", result)
	}

	status, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{"action": "status"})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{
		processCheckKind,
		"ProcessCheckResult",
		"method_run_hard_gates",
		"carry_through",
	} {
		if strings.Contains(status, absent) {
			t.Fatalf("default status should not inline closure-discipline fragment %q:\n%s", absent, status)
		}
	}
}

func TestRunProcessCheckJSON(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedProcessMethodRun(t, fixture, validProcessMethodRunFixture(time.Now().UTC()))
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	oldJSON := processCheckJSON
	oldProfile := processCheckProfile
	t.Cleanup(func() {
		processCheckJSON = oldJSON
		processCheckProfile = oldProfile
	})

	processCheckJSON = true
	processCheckProfile = "core"

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := runProcessCheck(cmd, nil); err != nil {
		t.Fatal(err)
	}

	report := processCheckReport{}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode process check report: %v\n%s", err, output.String())
	}
	if report.Kind != processCheckKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if len(report.Results) != 5 {
		t.Fatalf("results = %d, want 5", len(report.Results))
	}
	if strings.Contains(output.String(), "generated_schema_fragments") {
		t.Fatalf("process check should not inline generated schema fragments:\n%s", output.String())
	}
}

func TestProcessCheckDoesNotInlineIntoDefaultStatus(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedProcessMethodRun(t, fixture, validProcessMethodRunFixture(time.Now().UTC()))

	status, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{"action": "status"})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{
		processCheckKind,
		"ProcessCheckResult",
		"method_run_hard_gates",
		"generated_contract_runtime_schema",
	} {
		if strings.Contains(status, absent) {
			t.Fatalf("default status should not inline process check fragment %q:\n%s", absent, status)
		}
	}
	if got := len(processDefaultStatusActionLines(status)); got > 20 {
		t.Fatalf("default status action lines = %d, want <= 20:\n%s", got, status)
	}
}

func TestProcessDefaultStatusActionLinesCountsOperatorDrilldowns(t *testing.T) {
	status := strings.Join([]string{
		"## Haft Status",
		"- **HIGH** Current drift needs operator review: inspect exact items with `haft_query(action=\"drift_events\")`",
		"- **Refresh due**: run `haft_refresh(action=\"scan\", verbose=true)`.",
		"### Drill-down",
		"- Full status: `haft_query(action=\"status\", full=true)`.",
		"- Coverage: `haft_query(action=\"coverage\")`.",
		"- Default descriptive sentence with no operator command.",
		"Available: /h-verify (review stale claims)",
		"↑ Present to user — do not auto-execute.",
	}, "\n")

	lines := processDefaultStatusActionLines(status)
	if len(lines) != 6 {
		t.Fatalf("action lines = %d, want 6: %#v", len(lines), lines)
	}
	for _, unexpected := range []string{
		"## Haft Status",
		"Default descriptive sentence",
	} {
		if strings.Contains(strings.Join(lines, "\n"), unexpected) {
			t.Fatalf("non-action line counted %q in %#v", unexpected, lines)
		}
	}
}

func TestSummarizeProcessCheckResultsClassifiesStatuses(t *testing.T) {
	results := []ProcessCheckResult{
		{Status: processCheckStatusPass},
		{Status: processCheckStatusFail},
		{Status: processCheckStatusDegraded},
		{Status: processCheckStatusUnknown},
		{Status: processCheckStatusNotApplicable},
	}
	summary := summarizeProcessCheckResults(results)
	if summary.Total != 5 ||
		summary.Failing != 1 ||
		summary.Degraded != 1 ||
		summary.Unknown != 1 ||
		summary.NotApplicable != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.ByStatus[processCheckStatusPass] != 1 {
		t.Fatalf("pass count = %d", summary.ByStatus[processCheckStatusPass])
	}
}

func writeProcessTelemetrySessionFixture(t *testing.T, root string) string {
	t.Helper()

	path := filepath.Join(root, "codex-process.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"timestamp":"2026-06-25T00:00:00Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","call_id":"call_a","arguments":"{\"action\":\"pull\",\"task\":\"Implement semantic spine v3 first slice\",\"carry_through\":[{\"source_ref\":\"review:external\",\"source_item_ref\":\"finding-1\",\"acceptance_ref\":\"operator:accepted\"}]}"}}`,
		`{"timestamp":"2026-06-25T00:00:00Z","type":"event_msg","payload":{"type":"mcp_tool_call_end","call_id":"call_a","invocation":{"server":"haft","tool":"haft_method","arguments":{"action":"pull"}},"result":{"Ok":{"content":[{"type":"text","text":"Method pull recorded: ` + "`mpull-a`" + `"}]}}}}`,
		`{"timestamp":"2026-06-25T00:01:00Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","call_id":"call_b","arguments":"{\"action\":\"pull\",\"task\":\"Implement semantic spine v3 first slice\"}"}}`,
		`{"timestamp":"2026-06-25T00:01:00Z","type":"event_msg","payload":{"type":"mcp_tool_call_end","call_id":"call_b","invocation":{"server":"haft","tool":"haft_method","arguments":{"action":"pull"}},"result":{"Ok":{"content":[{"type":"text","text":"Method pull recorded: ` + "`mpull-b`" + `"}]}}}}`,
		`{"timestamp":"2026-06-25T00:05:00Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"pull_id\":\"mpull-a\",\"gate_results\":[{\"gate_id\":\"fresh_verification_before_completion\",\"status\":\"satisfied\",\"evidence_refs\":[\"go test ./internal/cli\"]}],\"verification\":{\"result\":\"pass\",\"commands\":[\"go test ./internal/cli\"],\"output_ref\":\"test output\"},\"carry_through\":[{\"source_ref\":\"review:external\",\"source_item_ref\":\"finding-1\",\"acceptance_ref\":\"operator:accepted\",\"disposition\":\"applied\",\"target_refs\":[\"internal/cli/process.go\"],\"evidence_refs\":[\"go test ./internal/cli\"]}]}"}}`,
		`{"timestamp":"2026-06-25T00:06:00Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"pull_id\":\"mpull-a\",\"gate_results\":[{\"gate_id\":\"fresh_verification_before_completion\",\"status\":\"waived\",\"waiver_reason\":\"operator accepted residual risk\"}],\"verification\":{\"result\":\"failed\",\"commands\":[\"go test ./internal/cli\"],\"output_ref\":\"test output\"}}"}}`,
		`{"timestamp":"2026-06-25T00:07:00Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"pull_id\":\"mpull-missing\",\"verification\":{\"result\":\"pass\"}}"}}`,
		`{"timestamp":"2026-06-25T00:08:00Z","type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"exec_command","arguments":"{\"cmd\":\"go run ./cmd/haft run --help\"}"}}`,
		`{"timestamp":"2026-06-25T00:09:00Z","type":"response_item","payload":{"type":"message","content":[{"type":"output_text","text":"transcript body that should not be in telemetry, with haft_method prose mention"}]}}`,
	})
	return path
}

func seedProcessMethodRun(t *testing.T, fixture checkTestProject, run methodpkg.MethodRun) {
	t.Helper()

	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	status := artifact.StatusActive
	if run.Status == "closed" {
		status = artifact.StatusAddressed
	}
	now := time.Now().UTC()
	item := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        run.ID,
			Kind:      artifact.KindMethodRun,
			Status:    status,
			Title:     "Method pull: " + run.TaskSignature.Task,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "method run fixture",
		StructuredData: string(encoded),
	}
	if err := fixture.store.Create(context.Background(), item); err != nil {
		t.Fatalf("create method run fixture: %v", err)
	}
}

func validProcessMethodRunFixture(now time.Time) methodpkg.MethodRun {
	gate := methodpkg.Gate{
		ID:            "fresh_verification_before_completion",
		Kind:          "verification",
		CheckLevel:    "hard",
		PassCondition: "fresh verification recorded",
		RequiredEvidence: []string{
			"test command",
		},
		Waiver: methodpkg.WaiverPolicy{
			Allowed:        true,
			RequiresReason: true,
		},
	}
	return methodpkg.MethodRun{
		ID:             "mpull-valid-process-check",
		CatalogID:      methodpkg.CatalogID,
		CatalogVersion: methodpkg.CatalogVersion,
		Status:         "closed",
		TaskSignature: methodpkg.TaskSignature{
			Task:               "Valid process check fixture",
			NormalizedTaskKind: "feature",
			Ceremony:           "medium",
		},
		Methods: []methodpkg.MethodCard{{
			ID:               "verification-before-completion",
			Version:          "1.0.0",
			Title:            "Verification before completion",
			WhyApplies:       "fixture",
			Intent:           "fixture",
			HardGates:        []methodpkg.Gate{gate},
			RequiredCloseout: true,
		}},
		OpenedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
		ClosedAt: now.Add(-time.Hour).Format(time.RFC3339),
		Closeout: &methodpkg.Closeout{
			GateResults: []methodpkg.GateResult{{
				GateID:       gate.ID,
				Status:       "satisfied",
				EvidenceRefs: []string{"go test ./internal/cli"},
			}},
			Verification: methodpkg.Verification{
				Commands:  []string{"go test ./internal/cli"},
				Result:    "pass",
				OutputRef: "test output",
			},
			ClosedAt: now.Add(-time.Hour).Format(time.RFC3339),
		},
	}
}

func processCheckTestHasResult(results []ProcessCheckResult, checkID string) bool {
	for _, result := range results {
		if result.CheckID == checkID {
			return true
		}
	}
	return false
}

func processCheckTestResult(t *testing.T, results []ProcessCheckResult, checkID string) ProcessCheckResult {
	t.Helper()
	for _, result := range results {
		if result.CheckID == checkID {
			return result
		}
	}
	t.Fatalf("missing check %q in %#v", checkID, results)
	return ProcessCheckResult{}
}

func seedProcessBroadBindingDecision(
	t *testing.T,
	fixture checkTestProject,
	id string,
	title string,
	fields artifact.DecisionFields,
	files []artifact.AffectedFile,
) {
	t.Helper()

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	item := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        id,
			Kind:      artifact.KindDecisionRecord,
			Status:    artifact.StatusActive,
			Title:     title,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "decision fixture",
		StructuredData: string(encoded),
	}
	if err := fixture.store.Create(context.Background(), item); err != nil {
		t.Fatalf("create decision fixture: %v", err)
	}
	if err := fixture.store.SetAffectedFiles(context.Background(), id, files); err != nil {
		t.Fatalf("set affected files: %v", err)
	}
}
