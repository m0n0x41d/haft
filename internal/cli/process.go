package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

const (
	processTelemetryKind      = "haft_process_telemetry"
	processTelemetryAuthority = "read_only_process_telemetry_not_authority_gate_or_evidence_truth"
	processCheckKind          = "haft_process_check"
	processCheckAuthority     = "read_only_process_check_not_approval_authorization_evidence_truth_or_gate_passage"
)

const (
	processCheckStatusPass          = "pass"
	processCheckStatusFail          = "fail"
	processCheckStatusDegraded      = "degraded"
	processCheckStatusUnknown       = "unknown"
	processCheckStatusNotApplicable = "not_applicable"
)

var processTelemetryMutationBoundary = []string{
	"read_only_process_baseline",
	"does_not_mutate_method_runs_decisions_evidence_or_carriers",
	"does_not_create_process_authority_objects",
	"does_not_prove_code_correctness_gate_passage_or_operator_authorization",
}

var (
	processTelemetryJSON          bool
	processTelemetrySessionInputs []string
	processTelemetryLongOpenHours int
	processCheckJSON              bool
	processCheckProfile           string
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Inspect Haft process-method behavior",
}

var processTelemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Read-only MethodPack and session process telemetry",
	Long: `Build a read-only baseline report for MethodPack/MethodRun behavior.

The telemetry is an observation surface only. It does not create process
authority, does not mutate artifacts, and does not prove gate passage or
operator authorization.`,
	RunE: runProcessTelemetry,
}

var processCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Read-only process conformance checks",
	Long: `Aggregate current process invariants into stable read-only check results.

Process checks are observations. They do not approve work, authorize binding
actions, create evidence truth, pass gates, or mutate artifacts.`,
	RunE: runProcessCheck,
}

type processTelemetryOptions struct {
	SessionInputs []string
	Now           time.Time
	LongOpenAfter time.Duration
}

type processCheckOptions struct {
	Profile string
	Now     time.Time
}

type processCheckReport struct {
	Kind             string               `json:"kind"`
	SchemaVersion    int                  `json:"schema_version"`
	Authority        string               `json:"authority"`
	MutationBoundary []string             `json:"mutation_boundary"`
	Profile          string               `json:"profile"`
	ObservedAt       string               `json:"observed_at"`
	Summary          processCheckSummary  `json:"summary"`
	Results          []ProcessCheckResult `json:"results"`
	Notes            []string             `json:"notes,omitempty"`
}

type processCheckSummary struct {
	Total         int            `json:"total"`
	ByStatus      map[string]int `json:"by_status"`
	Failing       int            `json:"failing"`
	Degraded      int            `json:"degraded"`
	Unknown       int            `json:"unknown"`
	NotApplicable int            `json:"not_applicable"`
}

// ProcessCheckResult is the stable read-only process conformance result shape.
type ProcessCheckResult struct {
	CheckID           string   `json:"check_id"`
	CheckVersion      string   `json:"check_version"`
	BearerRef         string   `json:"bearer_ref"`
	Scope             string   `json:"scope"`
	Status            string   `json:"status"`
	Severity          string   `json:"severity"`
	ObservedAt        string   `json:"observed_at"`
	ValidUntil        string   `json:"valid_until"`
	Finding           string   `json:"finding"`
	EvidenceRefs      []string `json:"evidence_refs,omitempty"`
	NextAction        string   `json:"next_action"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type processTelemetryReport struct {
	Kind             string                           `json:"kind"`
	SchemaVersion    int                              `json:"schema_version"`
	Authority        string                           `json:"authority"`
	MutationBoundary []string                         `json:"mutation_boundary"`
	ScanPolicy       processTelemetryScanPolicy       `json:"scan_policy"`
	Summary          processTelemetrySummary          `json:"summary"`
	MethodRuns       processMethodRunTelemetry        `json:"method_runs"`
	Sessions         []processSessionTelemetryFile    `json:"sessions,omitempty"`
	OperatorBurden   processOperatorBurdenTelemetry   `json:"operator_burden_proxies"`
	BroadBindingRisk processBroadBindingRiskTelemetry `json:"broad_binding_risk"`
	Diagnostics      []processTelemetryDiagnostic     `json:"diagnostics,omitempty"`
}

type processTelemetryScanPolicy struct {
	ProjectRoot        string   `json:"project_root"`
	SessionInputs      []string `json:"session_inputs,omitempty"`
	MethodRunSource    string   `json:"method_run_source"`
	DecisionRiskSource string   `json:"decision_risk_source"`
	EventPolicy        []string `json:"event_policy"`
}

type processTelemetrySummary struct {
	MethodRunsTotal                    int `json:"method_runs_total"`
	MethodRunsOpen                     int `json:"method_runs_open"`
	MethodRunsClosed                   int `json:"method_runs_closed"`
	MethodRunsLongOpen                 int `json:"method_runs_long_open"`
	MethodRunsWithWaivers              int `json:"method_runs_with_waivers"`
	MethodRunsMissingVerify            int `json:"method_runs_missing_verification"`
	SessionFilesScanned                int `json:"session_files_scanned"`
	SessionMethodPulls                 int `json:"session_method_pulls"`
	SessionMethodCloses                int `json:"session_method_closes"`
	SessionDuplicatePulls              int `json:"session_duplicate_pulls"`
	SessionDuplicateCloses             int `json:"session_duplicate_closes"`
	SessionCloseFailures               int `json:"session_close_failures"`
	SessionWaiverItems                 int `json:"session_waiver_items"`
	SessionCloseRepairRounds           int `json:"session_close_repair_rounds"`
	SessionCarryThroughItems           int `json:"session_carry_through_items"`
	MethodRunsPendingCarryThroughItems int `json:"method_runs_pending_carry_through_items"`
	SessionCLIRunCommands              int `json:"session_cli_run_commands"`
	BroadBindingRisks                  int `json:"broad_binding_risks"`
}

type processMethodRunTelemetry struct {
	Total                        int                          `json:"total"`
	ByStatus                     map[string]int               `json:"by_status,omitempty"`
	Open                         int                          `json:"open"`
	Closed                       int                          `json:"closed"`
	LongOpen                     []processMethodRunAge        `json:"long_open,omitempty"`
	ClosedWithWaivers            []processMethodRunCloseIssue `json:"closed_with_waivers,omitempty"`
	ClosedMissingVerification    []processMethodRunCloseIssue `json:"closed_missing_verification,omitempty"`
	PendingCarryThrough          []processMethodRunCloseIssue `json:"pending_carry_through,omitempty"`
	Unreadable                   []processTelemetryDiagnostic `json:"unreadable,omitempty"`
	LongOpenThresholdHours       float64                      `json:"long_open_threshold_hours"`
	CloseFailuresRecoverable     bool                         `json:"close_failures_recoverable"`
	CloseFailuresRecoverableNote string                       `json:"close_failures_recoverable_note"`
}

type processMethodRunAge struct {
	RunRef     string  `json:"run_ref"`
	Task       string  `json:"task,omitempty"`
	OpenedAt   string  `json:"opened_at,omitempty"`
	AgeHours   float64 `json:"age_hours"`
	NextAction string  `json:"next_action"`
}

type processMethodRunCloseIssue struct {
	RunRef     string   `json:"run_ref"`
	Task       string   `json:"task,omitempty"`
	Reason     string   `json:"reason"`
	Details    []string `json:"details,omitempty"`
	NextAction string   `json:"next_action"`
}

type processSessionTelemetryFile struct {
	Path                      string                         `json:"path"`
	Format                    string                         `json:"format"`
	EventsScanned             int                            `json:"events_scanned"`
	ParseErrors               int                            `json:"parse_errors,omitempty"`
	ResponseSpan              processSessionResponseSpan     `json:"response_span,omitempty"`
	MethodActionCounts        map[string]int                 `json:"method_action_counts,omitempty"`
	PullToCloseMinutes        processDurationStats           `json:"pull_to_close_minutes,omitempty"`
	DuplicatePullTasks        []processDuplicatePullTask     `json:"duplicate_pull_tasks,omitempty"`
	DuplicateCloseIDs         []processDuplicateCloseID      `json:"duplicate_close_ids,omitempty"`
	CloseFailures             int                            `json:"close_failures"`
	ClosesWithWaivers         int                            `json:"closes_with_waivers"`
	WaiverItemsOnClose        int                            `json:"waiver_items_on_close"`
	CloseRepairRounds         int                            `json:"close_repair_rounds"`
	CarryThroughItemsOnPull   int                            `json:"carry_through_items_on_pull"`
	CarryThroughItemsOnClose  int                            `json:"carry_through_items_on_close"`
	MissingGateResultsOnClose int                            `json:"missing_gate_results_on_close"`
	CloseWithoutPriorPull     int                            `json:"close_without_prior_pull"`
	PullWithoutClose          int                            `json:"pull_without_close"`
	InvocationLanes           processInvocationLaneTelemetry `json:"invocation_lanes"`
	ExampleRunCommands        []string                       `json:"example_run_commands,omitempty"`
}

type processSessionResponseSpan struct {
	First string  `json:"first,omitempty"`
	Last  string  `json:"last,omitempty"`
	Hours float64 `json:"hours,omitempty"`
}

type processDurationStats struct {
	Count int     `json:"count"`
	Min   float64 `json:"min,omitempty"`
	P50   float64 `json:"p50,omitempty"`
	P90   float64 `json:"p90,omitempty"`
	Max   float64 `json:"max,omitempty"`
}

type processDuplicatePullTask struct {
	NormalizedTask string `json:"normalized_task"`
	Count          int    `json:"count"`
}

type processDuplicateCloseID struct {
	PullID string `json:"pull_id"`
	Count  int    `json:"count"`
}

type processInvocationLaneTelemetry struct {
	MCPMethodCalls      int `json:"mcp_method_calls"`
	CLIRunCommands      int `json:"cli_run_commands"`
	CLIHelpOnlyCommands int `json:"cli_help_only_commands"`
	ProseMentions       int `json:"prose_mentions"`
}

type processOperatorBurdenTelemetry struct {
	AuthorityBoundary                  string   `json:"authority_boundary"`
	SourcePolicy                       []string `json:"source_policy"`
	SessionWaiverItems                 int      `json:"session_waiver_items"`
	SessionCloseRepairRounds           int      `json:"session_close_repair_rounds"`
	SessionCarryThroughItems           int      `json:"session_carry_through_items"`
	MethodRunsPendingCarryThrough      int      `json:"method_runs_pending_carry_through"`
	MethodRunsPendingCarryThroughItems int      `json:"method_runs_pending_carry_through_items"`
}

type processBroadBindingRiskTelemetry struct {
	ProblemRef                   string                        `json:"problem_ref"`
	AuthorityBoundary            string                        `json:"authority_boundary"`
	DecisionsWithAffectedFiles   int                           `json:"decisions_with_affected_files"`
	DecisionsWithExplicitTargets int                           `json:"decisions_with_explicit_targets"`
	ImplementationFootprintOnly  int                           `json:"implementation_footprint_only"`
	Risks                        []processBroadBindingRiskItem `json:"risks,omitempty"`
}

type processBroadBindingRiskItem struct {
	DecisionRef        string   `json:"decision_ref"`
	Title              string   `json:"title,omitempty"`
	AffectedFilesCount int      `json:"affected_files_count"`
	SampleFiles        []string `json:"sample_files,omitempty"`
	Classification     string   `json:"classification"`
	NextAction         string   `json:"next_action"`
}

type processTelemetryDiagnostic struct {
	Level      string   `json:"level"`
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	NextAction string   `json:"next_action"`
	Examples   []string `json:"examples,omitempty"`
}

type processSessionMethodEvent struct {
	Ordinal            int
	CallID             string
	Timestamp          time.Time
	Tool               string
	Action             string
	PullID             string
	Task               string
	VerificationResult string
	GateCount          int
	WaiverCount        int
	CarryThroughCount  int
	Payload            string
}

var processCLIRunCommandPattern = regexp.MustCompile(`(^|\s)(go\s+run\s+\./cmd/haft\s+run|haft\s+run)(\s|$)`)

func init() {
	processTelemetryCmd.Flags().BoolVar(&processTelemetryJSON, "json", false, "print structured JSON output")
	processTelemetryCmd.Flags().StringArrayVar(&processTelemetrySessionInputs, "session", nil, "Codex or Claude JSONL session file or directory; repeatable")
	processTelemetryCmd.Flags().IntVar(&processTelemetryLongOpenHours, "long-open-hours", 12, "open MethodRun age threshold in hours")
	processCheckCmd.Flags().BoolVar(&processCheckJSON, "json", false, "print structured JSON output")
	processCheckCmd.Flags().StringVar(&processCheckProfile, "profile", "core", "check profile to run; currently only core")
	processCmd.AddCommand(processTelemetryCmd)
	processCmd.AddCommand(processCheckCmd)
	rootCmd.AddCommand(processCmd)
}

func runProcessTelemetry(cmd *cobra.Command, _ []string) error {
	if processTelemetryLongOpenHours < 0 {
		return fmt.Errorf("long-open-hours must be >= 0")
	}

	projectRoot, store, closeStore, err := openArtifactCLIStore()
	if err != nil {
		return err
	}
	defer closeStore()

	options := processTelemetryOptions{
		SessionInputs: append([]string(nil), processTelemetrySessionInputs...),
		Now:           time.Now().UTC(),
		LongOpenAfter: time.Duration(processTelemetryLongOpenHours) * time.Hour,
	}
	report, err := buildProcessTelemetryReport(context.Background(), store, projectRoot, options)
	if err != nil {
		return err
	}
	if processTelemetryJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writeProcessTelemetryText(cmd.OutOrStdout(), report)
}

func runProcessCheck(cmd *cobra.Command, _ []string) error {
	projectRoot, store, closeStore, err := openArtifactCLIStore()
	if err != nil {
		return err
	}
	defer closeStore()

	report, err := buildProcessCheckReport(context.Background(), store, projectRoot, processCheckOptions{
		Profile: processCheckProfile,
		Now:     time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if processCheckJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writeProcessCheckText(cmd.OutOrStdout(), report)
}

func buildProcessCheckReport(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	options processCheckOptions,
) (processCheckReport, error) {
	options = normalizeProcessCheckOptions(options)
	if options.Profile != "core" {
		return processCheckReport{}, fmt.Errorf("unsupported process check profile %q; supported profiles: core", options.Profile)
	}
	observedAt := options.Now.UTC().Format(time.RFC3339)
	validUntil := options.Now.UTC().Add(24 * time.Hour).Format(time.RFC3339)
	report := processCheckReport{
		Kind:             processCheckKind,
		SchemaVersion:    1,
		Authority:        processCheckAuthority,
		MutationBoundary: append([]string(nil), processTelemetryMutationBoundary...),
		Profile:          options.Profile,
		ObservedAt:       observedAt,
		Notes: []string{
			"Process checks are read-only observations, not approval, operator authorization, evidence truth, gate passage, global truth, or publication.",
			"Default status is not modified by this report; use haft process check --json for details.",
		},
	}

	results := []ProcessCheckResult{
		processCheckMethodRunHardGates(ctx, store, observedAt, validUntil),
		processCheckGeneratedContractRuntime(observedAt, validUntil),
		processCheckBindingActionsFailClosed(observedAt, validUntil),
		processCheckDefaultStatusCompact(ctx, store, projectRoot, observedAt, validUntil),
		processCheckInterfaceDiscoveryCompact(observedAt, validUntil),
	}
	report.Results = results
	report.Summary = summarizeProcessCheckResults(results)
	return report, nil
}

func normalizeProcessCheckOptions(options processCheckOptions) processCheckOptions {
	if strings.TrimSpace(options.Profile) == "" {
		options.Profile = "core"
	}
	options.Profile = processNormalizeStatus(options.Profile)
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	return options
}

func processCheckMethodRunHardGates(
	ctx context.Context,
	store *artifact.Store,
	observedAt string,
	validUntil string,
) ProcessCheckResult {
	checked, issues, err := processMethodRunHardGateIssues(ctx, store)
	if err != nil {
		return processCheckResult(
			"method_run_hard_gates",
			"MethodRun",
			"project_method_runs",
			processCheckStatusUnknown,
			"medium",
			observedAt,
			validUntil,
			"Could not inspect MethodRun hard-gate closeouts: "+err.Error(),
			nil,
			"Fix artifact store access, then rerun haft process check.",
		)
	}
	if checked == 0 {
		return processCheckResult(
			"method_run_hard_gates",
			"MethodRun",
			"project_method_runs",
			processCheckStatusNotApplicable,
			"info",
			observedAt,
			validUntil,
			"No closed MethodRun artifacts were found.",
			nil,
			"No action unless MethodPack is expected to be in use for this project.",
		)
	}
	if len(issues) > 0 {
		return processCheckResult(
			"method_run_hard_gates",
			"MethodRun",
			"project_method_runs",
			processCheckStatusFail,
			"high",
			observedAt,
			validUntil,
			fmt.Sprintf("%d closed MethodRun(s) have hard-gate closeout gaps.", len(issues)),
			processSampleStrings(issues, 5),
			"Inspect listed MethodRuns; add evidence refs or valid waiver reasons before relying on them.",
		)
	}
	return processCheckResult(
		"method_run_hard_gates",
		"MethodRun",
		"project_method_runs",
		processCheckStatusPass,
		"low",
		observedAt,
		validUntil,
		fmt.Sprintf("%d closed MethodRun(s) satisfy hard-gate evidence or waiver requirements.", checked),
		[]string{"method.ValidateClose over MethodRun structured_data"},
		"No action.",
	)
}

func processMethodRunHardGateIssues(
	ctx context.Context,
	store *artifact.Store,
) (int, []string, error) {
	heads, err := store.ListByKind(ctx, artifact.KindMethodRun, 0)
	if err != nil {
		return 0, nil, err
	}
	var checked int
	var issues []string
	for _, head := range heads {
		full, err := store.Get(ctx, head.Meta.ID)
		if err != nil {
			issues = append(issues, head.Meta.ID+": unreadable")
			continue
		}
		run, err := methodpkg.DecodeRun(full)
		if err != nil {
			issues = append(issues, head.Meta.ID+": undecodable")
			continue
		}
		if processNormalizeStatus(run.Status) != "closed" {
			continue
		}
		checked++
		if run.Closeout == nil {
			issues = append(issues, run.ID+": missing closeout")
			continue
		}
		input := methodpkg.CloseInput{
			PullID:       run.ID,
			ChangedFiles: run.Closeout.ChangedFiles,
			GateResults:  run.Closeout.GateResults,
			Verification: run.Closeout.Verification,
			Waivers:      run.Closeout.Waivers,
			CarryThrough: run.Closeout.CarryThrough,
		}
		if err := methodpkg.ValidateClose(run, input); err != nil {
			issues = append(issues, run.ID+": "+err.Error())
		}
	}
	return checked, issues, nil
}

func processCheckGeneratedContractRuntime(
	observedAt string,
	validUntil string,
) ProcessCheckResult {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	audit := report.RuntimeAudit
	if audit.Status != "clean" || audit.RuntimeSchemaDrift != 0 {
		return processCheckResult(
			"generated_contract_runtime_schema",
			"kernel_interface_catalog",
			"mcp_tools_list_runtime_schema",
			processCheckStatusFail,
			"high",
			observedAt,
			validUntil,
			fmt.Sprintf("Generated contract schema fragments drift from runtime MCP schema: drift=%d.", audit.RuntimeSchemaDrift),
			processContractRuntimeAuditEvidence(audit),
			"Run haft interface contract-generation --json and repair the missing runtime schema fragments before relying on generated contracts.",
		)
	}
	return processCheckResult(
		"generated_contract_runtime_schema",
		"kernel_interface_catalog",
		"mcp_tools_list_runtime_schema",
		processCheckStatusPass,
		"low",
		observedAt,
		validUntil,
		fmt.Sprintf("%d generated schema fragments mirror the live runtime MCP schema.", audit.RuntimeSchemaMirrors),
		processContractRuntimeAuditEvidence(audit),
		"No action.",
	)
}

func processContractRuntimeAuditEvidence(audit interfaceContractRuntimeSchemaAudit) []string {
	refs := append([]string{}, audit.ValidationRefs...)
	refs = append(refs,
		fmt.Sprintf("runtime_schema_status=%s", audit.Status),
		fmt.Sprintf("runtime_schema_drift=%d", audit.RuntimeSchemaDrift),
	)
	return refs
}

func processCheckBindingActionsFailClosed(
	observedAt string,
	validUntil string,
) ProcessCheckResult {
	cases := []struct {
		tool   string
		action string
		args   map[string]any
	}{
		{tool: "haft_decision", action: "decide", args: map[string]any{"action": "decide"}},
		{tool: "haft_commission", action: "create", args: map[string]any{"action": "create"}},
		{tool: "haft_spec_section", action: "approve", args: map[string]any{"action": "approve", "section_id": "TS.example.001"}},
		{tool: "haft_refresh", action: "supersede", args: map[string]any{"action": "supersede", "artifact_ref": "dec-example"}},
	}
	var failures []string
	var evidence []string
	for _, tc := range cases {
		err := rejectMCPBindingAction(tc.tool, tc.args)
		if err == nil {
			failures = append(failures, tc.tool+"."+tc.action+": accepted without operator confirmation")
			continue
		}
		var payload operatorConfirmationRequired
		if decodeErr := json.Unmarshal([]byte(err.Error()), &payload); decodeErr != nil {
			failures = append(failures, tc.tool+"."+tc.action+": unstructured rejection")
			continue
		}
		if payload.Code != "operator_confirmation_required" ||
			payload.Tool != tc.tool ||
			payload.Action != tc.action ||
			payload.BindingMode != mcpBindingModeCLIOnly {
			failures = append(failures, tc.tool+"."+tc.action+": wrong rejection payload")
			continue
		}
		evidence = append(evidence, tc.tool+"."+tc.action+" -> operator_confirmation_required")
	}
	if len(failures) > 0 {
		return processCheckResult(
			"binding_actions_fail_closed",
			"mcp_binding_surface",
			"default_mcp_cli_only_binding_mode",
			processCheckStatusFail,
			"high",
			observedAt,
			validUntil,
			fmt.Sprintf("%d binding action(s) did not fail closed with operator_confirmation_required.", len(failures)),
			failures,
			"Restore rejectMCPBindingAction coverage before exposing binding mutations through MCP.",
		)
	}
	return processCheckResult(
		"binding_actions_fail_closed",
		"mcp_binding_surface",
		"default_mcp_cli_only_binding_mode",
		processCheckStatusPass,
		"low",
		observedAt,
		validUntil,
		"Binding MCP actions fail closed with operator_confirmation_required in default cli-only mode.",
		evidence,
		"No action.",
	)
}

func processCheckDefaultStatusCompact(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	observedAt string,
	validUntil string,
) ProcessCheckResult {
	status, err := handleQuintQuery(ctx, store, nil, filepath.Join(projectRoot, ".haft"), map[string]any{"action": "status"})
	if err != nil {
		return processCheckResult(
			"default_status_compact",
			"haft_query.status",
			"default_operator_cockpit",
			processCheckStatusUnknown,
			"medium",
			observedAt,
			validUntil,
			"Could not render default status: "+err.Error(),
			nil,
			"Fix status rendering before treating compactness as proven.",
		)
	}
	const maxDefaultStatusBytes = 20000
	const maxDefaultStatusActionLines = 20
	forbidden := processForbiddenStatusFragments(status)
	actionLines := processDefaultStatusActionLines(status)
	if len(status) > maxDefaultStatusBytes || len(forbidden) > 0 || len(actionLines) > maxDefaultStatusActionLines {
		evidence := []string{fmt.Sprintf("bytes=%d", len(status)), fmt.Sprintf("action_lines=%d", len(actionLines))}
		evidence = append(evidence, forbidden...)
		evidence = append(evidence, actionLines...)
		return processCheckResult(
			"default_status_compact",
			"haft_query.status",
			"default_operator_cockpit",
			processCheckStatusFail,
			"high",
			observedAt,
			validUntil,
			fmt.Sprintf("Default status is not compact enough: bytes=%d forbidden_fragments=%d action_lines=%d.", len(status), len(forbidden), len(actionLines)),
			evidence,
			"Move detailed process/contract data behind drill-down commands and keep default status as cockpit cues only.",
		)
	}
	return processCheckResult(
		"default_status_compact",
		"haft_query.status",
		"default_operator_cockpit",
		processCheckStatusPass,
		"low",
		observedAt,
		validUntil,
		fmt.Sprintf("Default status is compact and does not inline process or generated-contract reports: bytes=%d action_lines=%d.", len(status), len(actionLines)),
		[]string{fmt.Sprintf("bytes=%d <= %d", len(status), maxDefaultStatusBytes), fmt.Sprintf("action_lines=%d <= %d", len(actionLines), maxDefaultStatusActionLines)},
		"No action.",
	)
}

func processForbiddenStatusFragments(status string) []string {
	forbiddenMarkers := []string{
		"haft_interface_contract_generation_manifest",
		"generated_schema_fragments",
		"generated/contract-generation/schema/",
		"ProcessCheckResult",
		"ProcessAuthorityEntry",
		"ProcessReconcile",
		"CheckpointRecord",
		"haft_process_check",
		"haft_process_authority",
		"haft_process_reconcile",
		"method_checkpoint",
		"checkpoint_trace",
	}
	var found []string
	for _, marker := range forbiddenMarkers {
		if strings.Contains(status, marker) {
			found = append(found, marker)
		}
	}
	return found
}

func processDefaultStatusActionLines(status string) []string {
	var actionLines []string
	for _, line := range strings.Split(status, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if processDefaultStatusLineIsAction(trimmed) {
			actionLines = append(actionLines, trimmed)
		}
	}
	return actionLines
}

func processDefaultStatusLineIsAction(line string) bool {
	lower := strings.ToLower(line)
	actionMarkers := []string{
		"inspect exact items with",
		"drill down",
		"full status:",
		"coverage:",
		"drift/stale detail:",
		"drift events:",
		"maintenance plan:",
		"judgment review:",
		"safe drain preview:",
		"details: `",
		"available:",
		"↑ present",
		"run `haft_",
		"run `haft ",
	}
	for _, marker := range actionMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func processCheckInterfaceDiscoveryCompact(
	observedAt string,
	validUntil string,
) ProcessCheckResult {
	var output bytes.Buffer
	if err := writeInterfaceCatalogJSON(&output, haftInterfaceCatalog()); err != nil {
		return processCheckResult(
			"interface_discovery_compact",
			"haft_interface_catalog",
			"cli_interface_catalog_default_json",
			processCheckStatusUnknown,
			"medium",
			observedAt,
			validUntil,
			"Could not render interface catalog: "+err.Error(),
			nil,
			"Fix interface catalog rendering before treating compact discovery as proven.",
		)
	}
	const maxInterfaceCatalogBytes = 30000
	forbidden := processForbiddenInterfaceCatalogFragments(output.String())
	if output.Len() > maxInterfaceCatalogBytes || len(forbidden) > 0 {
		return processCheckResult(
			"interface_discovery_compact",
			"haft_interface_catalog",
			"cli_interface_catalog_default_json",
			processCheckStatusFail,
			"high",
			observedAt,
			validUntil,
			fmt.Sprintf("Interface catalog default JSON is not compact enough: bytes=%d forbidden_fragments=%d.", output.Len(), len(forbidden)),
			append([]string{fmt.Sprintf("bytes=%d", output.Len())}, forbidden...),
			"Keep default interface discovery to capability summaries; move schemas and generated fragments to explicit capability drill-downs.",
		)
	}
	return processCheckResult(
		"interface_discovery_compact",
		"haft_interface_catalog",
		"cli_interface_catalog_default_json",
		processCheckStatusPass,
		"low",
		observedAt,
		validUntil,
		fmt.Sprintf("Interface catalog default JSON is compact and summary-only: bytes=%d.", output.Len()),
		[]string{fmt.Sprintf("bytes=%d <= %d", output.Len(), maxInterfaceCatalogBytes)},
		"No action.",
	)
}

func processForbiddenInterfaceCatalogFragments(text string) []string {
	forbiddenMarkers := []string{
		"input_contract",
		"generated_schema_fragments",
		"haft_interface_contract_generation_manifest",
		"generated/contract-generation/schema/",
	}
	var found []string
	for _, marker := range forbiddenMarkers {
		if strings.Contains(text, marker) {
			found = append(found, marker)
		}
	}
	return found
}

func processCheckResult(
	checkID string,
	bearerRef string,
	scope string,
	status string,
	severity string,
	observedAt string,
	validUntil string,
	finding string,
	evidenceRefs []string,
	nextAction string,
) ProcessCheckResult {
	return ProcessCheckResult{
		CheckID:           checkID,
		CheckVersion:      "v0",
		BearerRef:         bearerRef,
		Scope:             scope,
		Status:            status,
		Severity:          severity,
		ObservedAt:        observedAt,
		ValidUntil:        validUntil,
		Finding:           finding,
		EvidenceRefs:      compactProcessStrings(evidenceRefs),
		NextAction:        nextAction,
		AuthorityBoundary: "read_only_process_observation_not_approval_not_operator_authorization_not_evidence_truth_not_gate_passage",
	}
}

func summarizeProcessCheckResults(results []ProcessCheckResult) processCheckSummary {
	summary := processCheckSummary{ByStatus: map[string]int{}}
	for _, result := range results {
		status := processNormalizeStatus(result.Status)
		if status == "" {
			status = processCheckStatusUnknown
		}
		summary.Total++
		summary.ByStatus[status]++
		switch status {
		case processCheckStatusFail:
			summary.Failing++
		case processCheckStatusDegraded:
			summary.Degraded++
		case processCheckStatusUnknown:
			summary.Unknown++
		case processCheckStatusNotApplicable:
			summary.NotApplicable++
		}
	}
	return summary
}

func buildProcessTelemetryReport(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	options processTelemetryOptions,
) (processTelemetryReport, error) {
	options = normalizeProcessTelemetryOptions(options)
	report := processTelemetryReport{
		Kind:             processTelemetryKind,
		SchemaVersion:    1,
		Authority:        processTelemetryAuthority,
		MutationBoundary: append([]string(nil), processTelemetryMutationBoundary...),
		ScanPolicy: processTelemetryScanPolicy{
			ProjectRoot:        filepath.Clean(projectRoot),
			SessionInputs:      compactProcessStrings(options.SessionInputs),
			MethodRunSource:    "artifacts.kind=MethodRun structured_data",
			DecisionRiskSource: "DecisionRecord structured_data plus affected_files rows",
			EventPolicy: []string{
				"method_run_records_are_project_truth_for_open_closed_state",
				"session_jsonl_is_observation_source_for_duplicate_calls_failed_close_results_and_invocation_lanes",
				"operator_burden_proxies_use_only_structured_methodrun_and_session_tool_arguments",
				"prose_mentions_are_counted_separately_and_do_not_count_as_invocation",
				"read_only_no_artifact_mutation",
			},
		},
	}

	methodRuns, err := buildProcessMethodRunTelemetry(ctx, store, options.Now, options.LongOpenAfter)
	if err != nil {
		return processTelemetryReport{}, err
	}
	report.MethodRuns = methodRuns

	sessions, err := buildProcessSessionTelemetry(options.SessionInputs)
	if err != nil {
		return processTelemetryReport{}, err
	}
	report.Sessions = sessions

	risk, err := buildProcessBroadBindingRiskTelemetry(ctx, store)
	if err != nil {
		return processTelemetryReport{}, err
	}
	report.BroadBindingRisk = risk
	report.OperatorBurden = buildProcessOperatorBurdenTelemetry(report.MethodRuns, report.Sessions)
	report.Summary = summarizeProcessTelemetry(report)
	report.Diagnostics = processTelemetryDiagnostics(report)
	return report, nil
}

func normalizeProcessTelemetryOptions(options processTelemetryOptions) processTelemetryOptions {
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	if options.LongOpenAfter == 0 {
		options.LongOpenAfter = 12 * time.Hour
	}
	options.SessionInputs = compactProcessStrings(options.SessionInputs)
	return options
}

func buildProcessMethodRunTelemetry(
	ctx context.Context,
	store *artifact.Store,
	now time.Time,
	longOpenAfter time.Duration,
) (processMethodRunTelemetry, error) {
	heads, err := store.ListByKind(ctx, artifact.KindMethodRun, 0)
	if err != nil {
		return processMethodRunTelemetry{}, err
	}

	telemetry := processMethodRunTelemetry{
		ByStatus:                     map[string]int{},
		LongOpenThresholdHours:       longOpenAfter.Hours(),
		CloseFailuresRecoverable:     false,
		CloseFailuresRecoverableNote: "failed close attempts are not stored in MethodRun artifacts; use session telemetry for verification.result=failed and duplicate close attempts",
	}
	for _, head := range heads {
		full, err := store.Get(ctx, head.Meta.ID)
		if err != nil {
			telemetry.Unreadable = append(telemetry.Unreadable, processUnreadableDiagnostic(head.Meta.ID, err))
			continue
		}
		run, err := methodpkg.DecodeRun(full)
		if err != nil {
			telemetry.Unreadable = append(telemetry.Unreadable, processUnreadableDiagnostic(head.Meta.ID, err))
			continue
		}
		telemetry = addProcessMethodRun(telemetry, run, now, longOpenAfter)
	}
	sortProcessMethodRunTelemetry(&telemetry)
	return telemetry, nil
}

func addProcessMethodRun(
	telemetry processMethodRunTelemetry,
	run methodpkg.MethodRun,
	now time.Time,
	longOpenAfter time.Duration,
) processMethodRunTelemetry {
	status := processNormalizeStatus(run.Status)
	telemetry.Total++
	telemetry.ByStatus[status]++
	switch status {
	case "open":
		telemetry.Open++
		if item, ok := processLongOpenMethodRun(run, now, longOpenAfter); ok {
			telemetry.LongOpen = append(telemetry.LongOpen, item)
		}
	case "closed":
		telemetry.Closed++
		if issue, ok := processClosedRunWaiverIssue(run); ok {
			telemetry.ClosedWithWaivers = append(telemetry.ClosedWithWaivers, issue)
		}
		if issue, ok := processClosedRunVerificationIssue(run); ok {
			telemetry.ClosedMissingVerification = append(telemetry.ClosedMissingVerification, issue)
		}
	}
	if issue, ok := processPendingCarryThroughIssue(run); ok {
		telemetry.PendingCarryThrough = append(telemetry.PendingCarryThrough, issue)
	}
	return telemetry
}

func processLongOpenMethodRun(
	run methodpkg.MethodRun,
	now time.Time,
	longOpenAfter time.Duration,
) (processMethodRunAge, bool) {
	openedAt, err := time.Parse(time.RFC3339, run.OpenedAt)
	if err != nil {
		return processMethodRunAge{}, false
	}
	age := now.Sub(openedAt)
	if age < longOpenAfter {
		return processMethodRunAge{}, false
	}
	return processMethodRunAge{
		RunRef:     run.ID,
		Task:       run.TaskSignature.Task,
		OpenedAt:   run.OpenedAt,
		AgeHours:   roundProcessFloat(age.Hours()),
		NextAction: "close with evidence, recover with haft_method(action=\"show\"), or explicitly supersede/reopen the work",
	}, true
}

func processClosedRunWaiverIssue(run methodpkg.MethodRun) (processMethodRunCloseIssue, bool) {
	if run.Closeout == nil {
		return processMethodRunCloseIssue{}, false
	}
	details := processCloseoutWaiverDetails(*run.Closeout)
	if len(details) == 0 {
		return processMethodRunCloseIssue{}, false
	}
	return processMethodRunCloseIssue{
		RunRef:     run.ID,
		Task:       run.TaskSignature.Task,
		Reason:     "closeout used waiver",
		Details:    details,
		NextAction: "review waiver reason before relying on the MethodRun as process-complete evidence",
	}, true
}

func processClosedRunVerificationIssue(run methodpkg.MethodRun) (processMethodRunCloseIssue, bool) {
	if run.Closeout == nil {
		return processMethodRunCloseIssue{
			RunRef:     run.ID,
			Task:       run.TaskSignature.Task,
			Reason:     "closed MethodRun has no closeout",
			NextAction: "inspect carrier and repair MethodRun structured_data if this is not a legacy artifact",
		}, true
	}
	verification := run.Closeout.Verification
	if strings.TrimSpace(verification.Result) != "" &&
		(len(verification.Commands) > 0 || strings.TrimSpace(verification.OutputRef) != "") {
		return processMethodRunCloseIssue{}, false
	}
	return processMethodRunCloseIssue{
		RunRef:     run.ID,
		Task:       run.TaskSignature.Task,
		Reason:     "verification result, command, or output_ref is missing",
		NextAction: "record fresh verification evidence before treating the run as complete",
	}, true
}

func processPendingCarryThroughIssue(run methodpkg.MethodRun) (processMethodRunCloseIssue, bool) {
	pending := processPendingCarryThroughDetails(run)
	if len(pending) == 0 {
		return processMethodRunCloseIssue{}, false
	}
	return processMethodRunCloseIssue{
		RunRef:     run.ID,
		Task:       run.TaskSignature.Task,
		Reason:     "carry-through items lack terminal disposition",
		Details:    pending,
		NextAction: "close with applied/rejected/deferred/superseded carry_through disposition or explicitly waive carry_through_disposition_recorded",
	}, true
}

func processPendingCarryThroughDetails(run methodpkg.MethodRun) []string {
	disposed := map[string]bool{}
	if run.Closeout != nil {
		for _, item := range run.Closeout.CarryThrough {
			if processNormalizeStatus(item.Disposition) == methodpkg.CarryDispositionPending {
				continue
			}
			disposed[processCarryThroughKey(item)] = true
		}
	}
	var pending []string
	for _, item := range run.CarryThrough {
		if disposed[processCarryThroughKey(item)] {
			continue
		}
		pending = append(pending, processCarryThroughKey(item))
	}
	return compactProcessStrings(pending)
}

func processCarryThroughKey(item methodpkg.CarryThroughItem) string {
	parts := []string{item.SourceRef, item.SourceItemRef, item.AcceptanceRef}
	return strings.Join(compactProcessStrings(parts), "#")
}

func processCloseoutWaiverDetails(closeout methodpkg.Closeout) []string {
	var details []string
	for _, waiver := range closeout.Waivers {
		detail := strings.TrimSpace(waiver.GateID)
		reason := strings.TrimSpace(waiver.Reason)
		if reason != "" {
			detail += ": " + reason
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	for _, result := range closeout.GateResults {
		if processNormalizeStatus(result.Status) != "waived" && strings.TrimSpace(result.WaiverReason) == "" {
			continue
		}
		detail := strings.TrimSpace(result.GateID)
		reason := strings.TrimSpace(result.WaiverReason)
		if reason != "" {
			detail += ": " + reason
		}
		if detail != "" {
			details = append(details, detail)
		}
	}
	return compactProcessStrings(details)
}

func processUnreadableDiagnostic(ref string, err error) processTelemetryDiagnostic {
	return processTelemetryDiagnostic{
		Level:      "medium",
		Code:       "unreadable_method_run",
		Message:    fmt.Sprintf("MethodRun %s could not be decoded: %v", ref, err),
		NextAction: "inspect the MethodRun carrier and structured_data block",
		Examples:   []string{ref},
	}
}

func sortProcessMethodRunTelemetry(telemetry *processMethodRunTelemetry) {
	sort.Slice(telemetry.LongOpen, func(i, j int) bool {
		return telemetry.LongOpen[i].AgeHours > telemetry.LongOpen[j].AgeHours
	})
	sort.Slice(telemetry.ClosedWithWaivers, func(i, j int) bool {
		return telemetry.ClosedWithWaivers[i].RunRef < telemetry.ClosedWithWaivers[j].RunRef
	})
	sort.Slice(telemetry.ClosedMissingVerification, func(i, j int) bool {
		return telemetry.ClosedMissingVerification[i].RunRef < telemetry.ClosedMissingVerification[j].RunRef
	})
	sort.Slice(telemetry.PendingCarryThrough, func(i, j int) bool {
		return telemetry.PendingCarryThrough[i].RunRef < telemetry.PendingCarryThrough[j].RunRef
	})
}

func buildProcessSessionTelemetry(inputs []string) ([]processSessionTelemetryFile, error) {
	var sessions []processSessionTelemetryFile
	for _, input := range compactProcessStrings(inputs) {
		files, err := sessionAuditFiles(input)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			session, err := processSessionTelemetryFileFromPath(file)
			if err != nil {
				return nil, err
			}
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Path < sessions[j].Path
	})
	return sessions, nil
}

func processSessionTelemetryFileFromPath(path string) (processSessionTelemetryFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return processSessionTelemetryFile{}, err
	}
	defer file.Close()

	session := processSessionTelemetryFile{
		Path:               filepath.Clean(path),
		Format:             "jsonl",
		MethodActionCounts: map[string]int{},
	}
	events := []processSessionMethodEvent{}
	pullIDsByCallID := map[string]string{}
	runCommands := []string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024*32)
	for scanner.Scan() {
		session.EventsScanned++
		line := append([]byte(nil), scanner.Bytes()...)
		record, parseErr := processDecodeSessionLine(line)
		if parseErr != nil {
			session.ParseErrors++
			continue
		}
		session.ResponseSpan = processSessionResponseSpanWith(record, session.ResponseSpan)
		if processLineHasProseMention(line) && !processLineHasToolInvocation(record) {
			session.InvocationLanes.ProseMentions++
		}
		nextEvents := processMethodInvocationEvents(record, len(events)+1)
		events = append(events, nextEvents...)
		processCollectPullResultIDs(record, pullIDsByCallID)
		commands := processCLIRunCommands(record)
		runCommands = append(runCommands, commands...)
	}
	if err := scanner.Err(); err != nil {
		return processSessionTelemetryFile{}, err
	}

	for index := range events {
		if events[index].PullID != "" {
			continue
		}
		if pullID := pullIDsByCallID[events[index].CallID]; pullID != "" {
			events[index].PullID = pullID
		}
	}
	return summarizeProcessSessionTelemetry(session, events, runCommands), nil
}

func processDecodeSessionLine(line []byte) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(line, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func processMethodInvocationEvents(record map[string]any, startOrdinal int) []processSessionMethodEvent {
	calls := sessionAuditToolCalls(record)
	events := make([]processSessionMethodEvent, 0, len(calls))
	for index, call := range calls {
		tool := sessionAuditToolName(call)
		if !strings.Contains(strings.ToLower(tool), "haft_method") {
			continue
		}
		args := processCallArguments(call)
		events = append(events, processSessionMethodEvent{
			Ordinal:            startOrdinal + index,
			CallID:             processCallID(call),
			Timestamp:          processRecordTimestamp(record),
			Tool:               tool,
			Action:             sessionAuditActionFromValue(args),
			PullID:             processStringValue(args["pull_id"]),
			Task:               processStringValue(args["task"]),
			VerificationResult: processVerificationResult(args),
			GateCount:          processArrayLength(args["gate_results"]),
			WaiverCount:        processArrayLength(args["waivers"]) + processWaivedGateResultCount(args["gate_results"]),
			CarryThroughCount:  processArrayLength(args["carry_through"]),
			Payload:            sessionAuditJSONText(call),
		})
	}
	return events
}

func processCollectPullResultIDs(record map[string]any, pullIDsByCallID map[string]string) {
	payload, ok := record["payload"].(map[string]any)
	if !ok {
		return
	}
	if sessionAuditStringField(payload, "type") != "mcp_tool_call_end" {
		return
	}
	invocation, ok := payload["invocation"].(map[string]any)
	if !ok {
		return
	}
	if !strings.Contains(strings.ToLower(sessionAuditStringField(invocation, "tool")), "haft_method") {
		return
	}
	arguments, _ := invocation["arguments"].(map[string]any)
	if sessionAuditActionFromValue(arguments) != "pull" {
		return
	}
	pullID := processFirstMethodRunID(processTextFragments(payload["result"]))
	if pullID == "" {
		return
	}
	callID := sessionAuditStringField(payload, "call_id")
	if callID == "" {
		callID = sessionAuditStringField(invocation, "call_id")
	}
	if callID == "" {
		return
	}
	pullIDsByCallID[callID] = pullID
}

func processCLIRunCommands(record map[string]any) []string {
	calls := sessionAuditToolCalls(record)
	var commands []string
	for _, call := range calls {
		tool := strings.ToLower(sessionAuditToolName(call))
		if !strings.Contains(tool, "exec_command") && !strings.Contains(tool, "bash") {
			continue
		}
		args := processCallArguments(call)
		command := processStringValue(args["cmd"])
		if command == "" {
			command = processStringValue(args["command"])
		}
		if !processCLIRunCommandPattern.MatchString(command) {
			continue
		}
		commands = append(commands, processCompactCommand(command))
	}
	return commands
}

func summarizeProcessSessionTelemetry(
	session processSessionTelemetryFile,
	events []processSessionMethodEvent,
	runCommands []string,
) processSessionTelemetryFile {
	pulls := processEventsByAction(events, "pull")
	closes := processEventsByAction(events, "close")
	session.InvocationLanes.MCPMethodCalls = len(events)
	session.InvocationLanes.CLIRunCommands = len(runCommands)
	session.InvocationLanes.CLIHelpOnlyCommands = processHelpOnlyCommandCount(runCommands)
	session.ExampleRunCommands = processSampleStrings(runCommands, 3)
	for _, event := range events {
		action := processNormalizeStatus(event.Action)
		if action == "" {
			action = "unknown"
		}
		session.MethodActionCounts[action]++
	}
	session.DuplicatePullTasks = processDuplicatePullTasks(pulls)
	session.DuplicateCloseIDs = processDuplicateCloseIDs(closes)
	session.PullToCloseMinutes = processPullToCloseDurations(pulls, closes)
	session.CloseFailures = processCloseFailureCount(closes)
	session.ClosesWithWaivers = processCloseWaiverCount(closes)
	session.WaiverItemsOnClose = processCloseWaiverItemCount(closes)
	session.CloseRepairRounds = processCloseRepairRounds(closes)
	session.CarryThroughItemsOnPull = processCarryThroughItemCount(pulls)
	session.CarryThroughItemsOnClose = processCarryThroughItemCount(closes)
	session.MissingGateResultsOnClose = processMissingGateResultsCount(closes)
	session.CloseWithoutPriorPull = processCloseWithoutPullCount(pulls, closes)
	session.PullWithoutClose = processPullWithoutCloseCount(pulls, closes)
	return session
}

func processEventsByAction(events []processSessionMethodEvent, action string) []processSessionMethodEvent {
	var out []processSessionMethodEvent
	for _, event := range events {
		if processNormalizeStatus(event.Action) != action {
			continue
		}
		out = append(out, event)
	}
	return out
}

func processDuplicatePullTasks(events []processSessionMethodEvent) []processDuplicatePullTask {
	counts := map[string]int{}
	for _, event := range events {
		key := processNormalizeTaskKey(event.Task)
		if key == "" {
			continue
		}
		counts[key]++
	}
	var out []processDuplicatePullTask
	for key, count := range counts {
		if count <= 1 {
			continue
		}
		out = append(out, processDuplicatePullTask{NormalizedTask: key, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].NormalizedTask < out[j].NormalizedTask
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func processDuplicateCloseIDs(events []processSessionMethodEvent) []processDuplicateCloseID {
	counts := map[string]int{}
	for _, event := range events {
		if event.PullID == "" {
			continue
		}
		counts[event.PullID]++
	}
	var out []processDuplicateCloseID
	for pullID, count := range counts {
		if count <= 1 {
			continue
		}
		out = append(out, processDuplicateCloseID{PullID: pullID, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].PullID < out[j].PullID
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func processPullToCloseDurations(
	pulls []processSessionMethodEvent,
	closes []processSessionMethodEvent,
) processDurationStats {
	firstPullByID := map[string]processSessionMethodEvent{}
	for _, event := range pulls {
		if event.PullID == "" || event.Timestamp.IsZero() {
			continue
		}
		if _, exists := firstPullByID[event.PullID]; exists {
			continue
		}
		firstPullByID[event.PullID] = event
	}
	var durations []float64
	closedIDs := map[string]bool{}
	for _, event := range closes {
		if event.PullID == "" || event.Timestamp.IsZero() || closedIDs[event.PullID] {
			continue
		}
		pull, exists := firstPullByID[event.PullID]
		if !exists {
			continue
		}
		durations = append(durations, event.Timestamp.Sub(pull.Timestamp).Minutes())
		closedIDs[event.PullID] = true
	}
	return processDurationStatsFromValues(durations)
}

func processDurationStatsFromValues(values []float64) processDurationStats {
	if len(values) == 0 {
		return processDurationStats{}
	}
	sort.Float64s(values)
	return processDurationStats{
		Count: len(values),
		Min:   roundProcessFloat(values[0]),
		P50:   roundProcessFloat(values[len(values)/2]),
		P90:   roundProcessFloat(values[processPercentileIndex(len(values), 0.9)]),
		Max:   roundProcessFloat(values[len(values)-1]),
	}
}

func processPercentileIndex(length int, percentile float64) int {
	index := int(float64(length) * percentile)
	if index >= length {
		return length - 1
	}
	return index
}

func processCloseFailureCount(events []processSessionMethodEvent) int {
	count := 0
	for _, event := range events {
		result := processNormalizeStatus(event.VerificationResult)
		if result == "fail" || result == "failed" {
			count++
		}
	}
	return count
}

func processCloseWaiverCount(events []processSessionMethodEvent) int {
	count := 0
	for _, event := range events {
		if event.WaiverCount > 0 {
			count++
		}
	}
	return count
}

func processCloseWaiverItemCount(events []processSessionMethodEvent) int {
	count := 0
	for _, event := range events {
		count += event.WaiverCount
	}
	return count
}

func processCarryThroughItemCount(events []processSessionMethodEvent) int {
	count := 0
	for _, event := range events {
		count += event.CarryThroughCount
	}
	return count
}

func processCloseRepairRounds(events []processSessionMethodEvent) int {
	rounds := 0
	for _, item := range processDuplicateCloseIDs(events) {
		rounds += item.Count - 1
	}
	return rounds
}

func processMissingGateResultsCount(events []processSessionMethodEvent) int {
	count := 0
	for _, event := range events {
		if event.GateCount == 0 {
			count++
		}
	}
	return count
}

func processCloseWithoutPullCount(
	pulls []processSessionMethodEvent,
	closes []processSessionMethodEvent,
) int {
	pullIDs := processEventPullIDSet(pulls)
	count := 0
	for _, event := range closes {
		if event.PullID == "" {
			continue
		}
		if pullIDs[event.PullID] {
			continue
		}
		count++
	}
	return count
}

func processPullWithoutCloseCount(
	pulls []processSessionMethodEvent,
	closes []processSessionMethodEvent,
) int {
	closeIDs := processEventPullIDSet(closes)
	count := 0
	for _, event := range pulls {
		if event.PullID == "" {
			continue
		}
		if closeIDs[event.PullID] {
			continue
		}
		count++
	}
	return count
}

func processEventPullIDSet(events []processSessionMethodEvent) map[string]bool {
	out := map[string]bool{}
	for _, event := range events {
		if event.PullID != "" {
			out[event.PullID] = true
		}
	}
	return out
}

func buildProcessBroadBindingRiskTelemetry(
	ctx context.Context,
	store *artifact.Store,
) (processBroadBindingRiskTelemetry, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return processBroadBindingRiskTelemetry{}, err
	}
	telemetry := processBroadBindingRiskTelemetry{
		ProblemRef:        "prob-20260625-75e0a8bc",
		AuthorityBoundary: "risk_observation_only_not_drift_verdict_not_binding_repair",
	}
	for _, head := range decisions {
		full, err := store.Get(ctx, head.Meta.ID)
		if err != nil {
			continue
		}
		files, err := store.GetAffectedFiles(ctx, full.Meta.ID)
		if err != nil || len(files) == 0 {
			continue
		}
		fields := full.UnmarshalDecisionFields()
		telemetry.DecisionsWithAffectedFiles++
		if fields.HasExplicitDriftAuthorityTargets() {
			telemetry.DecisionsWithExplicitTargets++
			continue
		}
		if fields.IsImplementationFootprintOnly() {
			telemetry.ImplementationFootprintOnly++
			continue
		}
		telemetry.Risks = append(telemetry.Risks, processBroadBindingRiskItem{
			DecisionRef:        full.Meta.ID,
			Title:              full.Meta.Title,
			AffectedFilesCount: len(files),
			SampleFiles:        processSampleAffectedFiles(files, 5),
			Classification:     "affected_files_without_explicit_governance_targets",
			NextAction:         "record governance_targets, drift_watch_targets, binding_targets, or move files to implementation_footprint before treating file churn as decision drift authority",
		})
	}
	sort.Slice(telemetry.Risks, func(i, j int) bool {
		return telemetry.Risks[i].DecisionRef < telemetry.Risks[j].DecisionRef
	})
	return telemetry, nil
}

func summarizeProcessTelemetry(report processTelemetryReport) processTelemetrySummary {
	summary := processTelemetrySummary{
		MethodRunsTotal:                    report.MethodRuns.Total,
		MethodRunsOpen:                     report.MethodRuns.Open,
		MethodRunsClosed:                   report.MethodRuns.Closed,
		MethodRunsLongOpen:                 len(report.MethodRuns.LongOpen),
		MethodRunsWithWaivers:              len(report.MethodRuns.ClosedWithWaivers),
		MethodRunsMissingVerify:            len(report.MethodRuns.ClosedMissingVerification),
		SessionFilesScanned:                len(report.Sessions),
		MethodRunsPendingCarryThroughItems: report.OperatorBurden.MethodRunsPendingCarryThroughItems,
		SessionWaiverItems:                 report.OperatorBurden.SessionWaiverItems,
		SessionCloseRepairRounds:           report.OperatorBurden.SessionCloseRepairRounds,
		SessionCarryThroughItems:           report.OperatorBurden.SessionCarryThroughItems,
		BroadBindingRisks:                  len(report.BroadBindingRisk.Risks),
	}
	for _, session := range report.Sessions {
		summary.SessionMethodPulls += session.MethodActionCounts["pull"]
		summary.SessionMethodCloses += session.MethodActionCounts["close"]
		summary.SessionDuplicatePulls += len(session.DuplicatePullTasks)
		summary.SessionDuplicateCloses += len(session.DuplicateCloseIDs)
		summary.SessionCloseFailures += session.CloseFailures
		summary.SessionCLIRunCommands += session.InvocationLanes.CLIRunCommands
	}
	return summary
}

func buildProcessOperatorBurdenTelemetry(
	methodRuns processMethodRunTelemetry,
	sessions []processSessionTelemetryFile,
) processOperatorBurdenTelemetry {
	telemetry := processOperatorBurdenTelemetry{
		AuthorityBoundary: "structured_process_proxy_not_quality_score_not_product_value_proof_not_operator_truth",
		SourcePolicy: []string{
			"methodrun_pending_carry_through_uses_MethodRun_structured_data_only",
			"session_counts_use_haft_method_tool_arguments_only",
			"no_arbitrary_prose_classification",
		},
		MethodRunsPendingCarryThrough: len(methodRuns.PendingCarryThrough),
	}
	for _, issue := range methodRuns.PendingCarryThrough {
		telemetry.MethodRunsPendingCarryThroughItems += len(issue.Details)
	}
	for _, session := range sessions {
		telemetry.SessionWaiverItems += session.WaiverItemsOnClose
		telemetry.SessionCloseRepairRounds += session.CloseRepairRounds
		telemetry.SessionCarryThroughItems += session.CarryThroughItemsOnPull
		telemetry.SessionCarryThroughItems += session.CarryThroughItemsOnClose
	}
	return telemetry
}

func processTelemetryDiagnostics(report processTelemetryReport) []processTelemetryDiagnostic {
	var diagnostics []processTelemetryDiagnostic
	if len(report.MethodRuns.LongOpen) > 0 {
		diagnostics = append(diagnostics, processTelemetryDiagnostic{
			Level:      "medium",
			Code:       "long_open_method_runs",
			Message:    "MethodRun(s) are open past the configured threshold",
			NextAction: "recover and close the run with evidence or explicitly abandon/supersede the work",
			Examples:   processMethodRunAgeRefs(report.MethodRuns.LongOpen, 3),
		})
	}
	if len(report.BroadBindingRisk.Risks) > 0 {
		diagnostics = append(diagnostics, processTelemetryDiagnostic{
			Level:      "medium",
			Code:       "affected_files_without_semantic_targets",
			Message:    "DecisionRecord(s) have affected_files but no explicit governance/drift/binding targets",
			NextAction: "repair with semantic targets or mark files as implementation footprint before relying on drift output",
			Examples:   processBroadBindingRiskRefs(report.BroadBindingRisk.Risks, 3),
		})
	}
	for _, session := range report.Sessions {
		if len(session.DuplicateCloseIDs) == 0 && len(session.DuplicatePullTasks) == 0 && session.CloseFailures == 0 {
			continue
		}
		diagnostics = append(diagnostics, processTelemetryDiagnostic{
			Level:      "low",
			Code:       "session_method_invocation_anomalies",
			Message:    "session transcript contains duplicate pull/close calls or failed verification close results",
			NextAction: "inspect the session path before designing checkpoint or handoff features",
			Examples:   []string{session.Path},
		})
	}
	return diagnostics
}

func writeProcessTelemetryText(output io.Writer, report processTelemetryReport) error {
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(output, format, args...)
		return err
	}
	if err := printf("Haft process telemetry\n"); err != nil {
		return err
	}
	if err := printf("authority: %s\n", report.Authority); err != nil {
		return err
	}
	if err := printf(
		"summary: method_runs=%d open=%d closed=%d long_open=%d sessions=%d session_pulls=%d session_closes=%d duplicate_pulls=%d duplicate_closes=%d waiver_items=%d close_repair_rounds=%d carry_through_items=%d pending_carry_through_items=%d cli_run_commands=%d broad_binding_risks=%d\n",
		report.Summary.MethodRunsTotal,
		report.Summary.MethodRunsOpen,
		report.Summary.MethodRunsClosed,
		report.Summary.MethodRunsLongOpen,
		report.Summary.SessionFilesScanned,
		report.Summary.SessionMethodPulls,
		report.Summary.SessionMethodCloses,
		report.Summary.SessionDuplicatePulls,
		report.Summary.SessionDuplicateCloses,
		report.Summary.SessionWaiverItems,
		report.Summary.SessionCloseRepairRounds,
		report.Summary.SessionCarryThroughItems,
		report.Summary.MethodRunsPendingCarryThroughItems,
		report.Summary.SessionCLIRunCommands,
		report.Summary.BroadBindingRisks,
	); err != nil {
		return err
	}
	for _, diagnostic := range report.Diagnostics {
		if err := printf("- %s %s: %s\n", diagnostic.Level, diagnostic.Code, diagnostic.Message); err != nil {
			return err
		}
	}
	return nil
}

func writeProcessCheckText(output io.Writer, report processCheckReport) error {
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(output, format, args...)
		return err
	}
	if err := printf("Haft process checks\n"); err != nil {
		return err
	}
	if err := printf("authority: %s\n", report.Authority); err != nil {
		return err
	}
	if err := printf(
		"summary: total=%d failing=%d degraded=%d unknown=%d not_applicable=%d profile=%s\n",
		report.Summary.Total,
		report.Summary.Failing,
		report.Summary.Degraded,
		report.Summary.Unknown,
		report.Summary.NotApplicable,
		report.Profile,
	); err != nil {
		return err
	}
	for _, result := range report.Results {
		if err := printf(
			"- %s status=%s severity=%s bearer=%s\n  finding: %s\n  next: %s\n",
			result.CheckID,
			result.Status,
			result.Severity,
			result.BearerRef,
			result.Finding,
			result.NextAction,
		); err != nil {
			return err
		}
	}
	return nil
}

func processRecordTimestamp(record map[string]any) time.Time {
	value := processStringValue(record["timestamp"])
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func processSessionResponseSpanWith(
	record map[string]any,
	span processSessionResponseSpan,
) processSessionResponseSpan {
	if sessionAuditStringField(record, "type") != "response_item" {
		return span
	}
	timestamp := processRecordTimestamp(record)
	if timestamp.IsZero() {
		return span
	}
	first, _ := time.Parse(time.RFC3339Nano, span.First)
	last, _ := time.Parse(time.RFC3339Nano, span.Last)
	if span.First == "" || timestamp.Before(first) {
		span.First = timestamp.Format(time.RFC3339Nano)
	}
	if span.Last == "" || timestamp.After(last) {
		span.Last = timestamp.Format(time.RFC3339Nano)
	}
	first, firstErr := time.Parse(time.RFC3339Nano, span.First)
	last, lastErr := time.Parse(time.RFC3339Nano, span.Last)
	if firstErr == nil && lastErr == nil && last.After(first) {
		span.Hours = roundProcessFloat(last.Sub(first).Hours())
	}
	return span
}

func processCallArguments(call map[string]any) map[string]any {
	for _, key := range []string{"arguments", "input"} {
		raw, exists := call[key]
		if !exists {
			continue
		}
		switch typed := raw.(type) {
		case map[string]any:
			return typed
		case string:
			var decoded map[string]any
			if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
				return decoded
			}
		}
	}
	return map[string]any{}
}

func processCallID(call map[string]any) string {
	for _, key := range []string{"call_id", "id"} {
		if value := sessionAuditStringField(call, key); value != "" {
			return value
		}
	}
	return ""
}

func processVerificationResult(args map[string]any) string {
	verification, ok := args["verification"].(map[string]any)
	if !ok {
		return ""
	}
	return processStringValue(verification["result"])
}

func processWaivedGateResultCount(raw any) int {
	items, ok := raw.([]any)
	if !ok {
		return 0
	}
	count := 0
	for _, item := range items {
		gate, ok := item.(map[string]any)
		if !ok {
			continue
		}
		status := processNormalizeStatus(processStringValue(gate["status"]))
		reason := strings.TrimSpace(processStringValue(gate["waiver_reason"]))
		if status == "waived" || reason != "" {
			count++
		}
	}
	return count
}

func processArrayLength(raw any) int {
	items, ok := raw.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func processTextFragments(raw any) []string {
	var out []string
	switch typed := raw.(type) {
	case string:
		out = append(out, typed)
	case []any:
		for _, item := range typed {
			out = append(out, processTextFragments(item)...)
		}
	case map[string]any:
		for _, item := range typed {
			out = append(out, processTextFragments(item)...)
		}
	}
	return out
}

func processFirstMethodRunID(values []string) string {
	for _, value := range values {
		for _, field := range strings.Fields(value) {
			trimmed := strings.Trim(field, "`\"'.,;()[]{}")
			if strings.HasPrefix(trimmed, "mpull-") {
				return trimmed
			}
		}
	}
	return ""
}

func processLineHasToolInvocation(record map[string]any) bool {
	if len(sessionAuditToolCalls(record)) > 0 {
		return true
	}
	payload, ok := record["payload"].(map[string]any)
	if !ok {
		return false
	}
	return sessionAuditStringField(payload, "type") == "mcp_tool_call_end"
}

func processLineHasProseMention(line []byte) bool {
	text := strings.ToLower(string(line))
	if !strings.Contains(text, "haft_method") && !strings.Contains(text, "haft run") {
		return false
	}
	return true
}

func processHelpOnlyCommandCount(commands []string) int {
	count := 0
	for _, command := range commands {
		if strings.Contains(command, "--help") {
			count++
		}
	}
	return count
}

func processNormalizeTaskKey(value string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(value)), " ")
	if len(normalized) > 120 {
		return normalized[:120]
	}
	return normalized
}

func processSampleAffectedFiles(files []artifact.AffectedFile, limit int) []string {
	values := make([]string, 0, len(files))
	for _, file := range files {
		values = append(values, file.Path)
	}
	sort.Strings(values)
	return processSampleStrings(values, limit)
}

func processSampleStrings(values []string, limit int) []string {
	values = compactProcessStrings(values)
	sort.Strings(values)
	if limit > 0 && len(values) > limit {
		return append([]string(nil), values[:limit]...)
	}
	return values
}

func processMethodRunAgeRefs(values []processMethodRunAge, limit int) []string {
	var refs []string
	for _, value := range values {
		refs = append(refs, value.RunRef)
	}
	return processSampleStrings(refs, limit)
}

func processBroadBindingRiskRefs(values []processBroadBindingRiskItem, limit int) []string {
	var refs []string
	for _, value := range values {
		refs = append(refs, value.DecisionRef)
	}
	return processSampleStrings(refs, limit)
}

func processCompactCommand(command string) string {
	compact := strings.Join(strings.Fields(command), " ")
	if len(compact) > 240 {
		return compact[:240]
	}
	return compact
}

func compactProcessStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func processNormalizeStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func processStringValue(raw any) string {
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func roundProcessFloat(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}
