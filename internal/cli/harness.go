package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/m0n0x41d/haft/internal/artifact"
)

type harnessPlanFile struct {
	ID               string                `yaml:"id"`
	Revision         string                `yaml:"revision"`
	Title            string                `yaml:"title,omitempty"`
	RepoRef          string                `yaml:"repo_ref"`
	BaseSHA          string                `yaml:"base_sha"`
	TargetBranch     string                `yaml:"target_branch"`
	ProjectionPolicy string                `yaml:"projection_policy"`
	ValidFor         string                `yaml:"valid_for,omitempty"`
	ValidUntil       string                `yaml:"valid_until,omitempty"`
	Queue            string                `yaml:"queue,omitempty"`
	Defaults         harnessPlanDefaults   `yaml:"defaults"`
	Decisions        []harnessPlanDecision `yaml:"decisions"`
}

type harnessPlanDefaults struct {
	AllowedActions       []string `yaml:"allowed_actions"`
	EvidenceRequirements []string `yaml:"evidence_requirements,omitempty"`
}

type harnessPlanDecision struct {
	Ref       string   `yaml:"ref"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

type harnessRunOptions struct {
	PlanPath            string
	GeneratedPlanPath   string
	PrepareOnly         bool
	ForceCreate         bool
	Once                bool
	OnceTimeoutMS       int
	Mock                bool
	MockAgent           bool
	MockJudge           bool
	Concurrency         int
	MaxClaims           int
	PollIntervalMS      int
	StatusPath          string
	LogPath             string
	WorkspaceRoot       string
	RuntimePath         string
	RepoURL             string
	AgentMaxTurns       int
	AgentTurnTimeoutMS  int
	AgentWallClockS     int
	JudgeWallClockS     int
	ApprovalPolicy      string
	ThreadSandbox       string
	ProjectionMode      string
	ProjectionProfile   string
	ExternalApprover    string
	StatusHTTPEnabled   bool
	StatusHTTPPort      int
	CommissionRunnerID  string
	CommissionPlanRef   string
	CommissionQueueName string
}

type harnessSessionLogSummary struct {
	StartedAt       string
	LastEvent       string
	LastEventAt     string
	LastTurnID      string
	LastTurnStatus  string
	LastTextPreview string
}

type harnessCommissionLogSummary struct {
	Phase       string
	Event       string
	At          string
	TurnID      string
	TurnStatus  string
	TextPreview string
}

var (
	harnessPlanOut          string
	harnessPlanID           string
	harnessPlanTitle        string
	harnessPlanRevision     string
	harnessPlanSequential   bool
	harnessPlanDependencies []string
	harnessPlanProblems     []string
	harnessPlanContext      string
	harnessPlanAllActive    bool

	harnessRunPlanPath          string
	harnessRunGeneratedPlanPath string
	harnessRunPrepareOnly       bool
	harnessRunForceCreate       bool
	harnessRunOnce              bool
	harnessRunOnceTimeoutMS     int
	harnessRunMock              bool
	harnessRunMockAgent         bool
	harnessRunMockJudge         bool
	harnessRunConcurrency       int
	harnessRunMaxClaims         int
	harnessRunPollIntervalMS    int
	harnessRunStatusPath        string
	harnessRunLogPath           string
	harnessRunWorkspaceRoot     string
	harnessRunRuntimePath       string
	harnessRunRepoURL           string

	harnessStatusPath    string
	harnessStatusLogPath string
	harnessStatusJSON    bool
	harnessStatusTail    int
)

var harnessCmd = &cobra.Command{
	Use:   "harness",
	Short: "Operate Haft Harness over WorkCommissions",
	Long: `Operate Haft Harness over WorkCommissions.

Haft is the product and semantic authority. ` + "`haft harness`" + ` is the
commissioned execution surface, backed today by the Open-Sleigh runtime.

The harness executes runnable WorkCommissions. Planning, decision slicing, and
commission creation stay upstream in Haft via ProblemCards, DecisionRecords,
plans, and commission commands.`,
}

var harnessPlanCmd = &cobra.Command{
	Use:   "plan [decision-id]...",
	Short: "Create an ImplementationPlan file from DecisionRecords",
	Args:  cobra.ArbitraryArgs,
	RunE:  runHarnessPlan,
}

var harnessRunCmd = &cobra.Command{
	Use:   "run [decision-id]...",
	Short: "Create commissions and start the Open-Sleigh harness",
	Long: `Create commissions and start the Open-Sleigh harness.

Run existing runnable WorkCommissions:
  haft harness run

Create commissions from explicit decision ids:
  haft harness run dec-a dec-b --sequential

Or select decisions from Haft:
  haft harness run --problem prob-...
  haft harness run --context mvp-harness
  haft harness run --all-active-decisions

Or pass an existing plan:
  haft harness run --plan .haft/plans/mvp.yaml`,
	Args:         cobra.ArbitraryArgs,
	SilenceUsage: true,
	RunE:         runHarnessRun,
}

var harnessStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the latest Haft Harness status",
	RunE:  runHarnessStatus,
}

var harnessResultCmd = &cobra.Command{
	Use:          "result [commission-id]",
	Short:        "Show the latest harness result and workspace diff",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runHarnessResult,
}

func init() {
	registerCommissionFromDecisionFlags(harnessPlanCmd)
	registerHarnessPlanFlags(harnessPlanCmd)

	registerCommissionFromDecisionFlags(harnessRunCmd)
	registerHarnessPlanFlags(harnessRunCmd)
	registerHarnessRunFlags(harnessRunCmd)
	registerHarnessStatusFlags(harnessStatusCmd)

	harnessCmd.AddCommand(harnessPlanCmd)
	harnessCmd.AddCommand(harnessRunCmd)
	harnessCmd.AddCommand(harnessStatusCmd)
	harnessCmd.AddCommand(harnessResultCmd)
	rootCmd.AddCommand(harnessCmd)
}

func registerHarnessPlanFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessPlanOut, "out", "", "Plan file path (default: .haft/plans/<id>.yaml)")
	cmd.Flags().StringVar(&harnessPlanID, "id", "", "Plan id (default: generated from current UTC time)")
	cmd.Flags().StringVar(&harnessPlanTitle, "title", "", "Plan title")
	cmd.Flags().StringVar(&harnessPlanRevision, "revision", "p1", "Plan revision")
	cmd.Flags().BoolVar(&harnessPlanSequential, "sequential", false, "Make each decision depend on the previous command-line decision")
	cmd.Flags().StringArrayVar(&harnessPlanDependencies, "depend", nil, "Dependency edge target:source[,source]. Example: dec-b:dec-a")
	cmd.Flags().StringArrayVar(&harnessPlanProblems, "problem", nil, "Select active decisions linked to this ProblemCard; repeatable")
	cmd.Flags().StringVar(&harnessPlanContext, "context", "", "Select uncommissioned active decisions in this optional Haft context")
	cmd.Flags().BoolVar(&harnessPlanAllActive, "all-active-decisions", false, "Select all active DecisionRecords, including already commissioned decisions")
}

func registerHarnessRunFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessRunPlanPath, "plan", "", "Existing ImplementationPlan YAML/JSON file")
	cmd.Flags().StringVar(&harnessRunGeneratedPlanPath, "generated-plan", "", "Path for generated plan when decision ids are passed")
	cmd.Flags().BoolVar(&harnessRunPrepareOnly, "prepare-only", false, "Create/reuse plan and commissions, then stop before starting Open-Sleigh")
	cmd.Flags().BoolVar(&harnessRunForceCreate, "force-create-commissions", false, "Create another commission set even when this plan already has commissions")
	cmd.Flags().BoolVar(&harnessRunOnce, "once", false, "Run one Open-Sleigh polling pass")
	cmd.Flags().IntVar(&harnessRunOnceTimeoutMS, "once-timeout-ms", 8000, "Open-Sleigh --once timeout in milliseconds")
	cmd.Flags().BoolVar(&harnessRunMock, "mock", false, "Use mock agent and mock judge")
	cmd.Flags().BoolVar(&harnessRunMockAgent, "mock-agent", false, "Use mock agent")
	cmd.Flags().BoolVar(&harnessRunMockJudge, "mock-judge", false, "Use mock judge")
	cmd.Flags().IntVar(&harnessRunConcurrency, "concurrency", 2, "Open-Sleigh engine concurrency")
	cmd.Flags().IntVar(&harnessRunMaxClaims, "max-claims", 50, "Maximum commission claims per poll")
	cmd.Flags().IntVar(&harnessRunPollIntervalMS, "poll-interval-ms", 30000, "Open-Sleigh poll interval")
	cmd.Flags().StringVar(&harnessRunStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessRunLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().StringVar(&harnessRunWorkspaceRoot, "workspace-root", "", "Open-Sleigh workspace root")
	cmd.Flags().StringVar(&harnessRunRuntimePath, "runtime", "", "Open-Sleigh runtime directory (default: project open-sleigh or installed ~/.haft runtime)")
	cmd.Flags().StringVar(&harnessRunRepoURL, "repo-url", "", "Repository URL/path cloned into workspaces (default: project root)")
}

func registerHarnessStatusFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessStatusLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().BoolVar(&harnessStatusJSON, "json", false, "Print raw status JSON")
	cmd.Flags().IntVar(&harnessStatusTail, "tail", 0, "Print the last N runtime log events")
}

func runHarnessPlan(cmd *cobra.Command, decisionRefs []string) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		selectedRefs, err := resolveHarnessDecisionRefs(ctx, store, decisionRefs)
		if err != nil {
			return err
		}

		plan, err := buildHarnessPlan(projectRoot, selectedRefs)
		if err != nil {
			return err
		}

		path, err := writeHarnessPlan(projectRoot, plan, harnessPlanOut)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Plan: %s\n", path)
		return err
	})
}

func runHarnessStatus(cmd *cobra.Command, _args []string) error {
	statusPath := selectedHarnessStatusPath()
	logPath := selectedHarnessLogPath()

	encoded, err := os.ReadFile(statusPath)
	if err != nil {
		return fmt.Errorf("read harness status %s: %w", statusPath, err)
	}

	if harnessStatusJSON {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(string(encoded)))
		return err
	}

	status := map[string]any{}
	if err := json.Unmarshal(encoded, &status); err != nil {
		return fmt.Errorf("decode harness status %s: %w", statusPath, err)
	}

	for _, line := range formatHarnessStatus(status, statusPath, logPath, harnessSessionLogSummaries(logPath)) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}

	if harnessStatusTail <= 0 {
		return nil
	}

	return printHarnessRuntimeTail(cmd, status, logPath, harnessStatusTail)
}

func runHarnessResult(cmd *cobra.Command, args []string) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, _projectRoot string) error {
		commissionID, err := harnessResultCommissionID(ctx, store, args)
		if err != nil {
			return err
		}

		commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
		if err != nil {
			return err
		}

		logPath := selectedHarnessLogPath()
		runtimeDetail, statusUpdatedAt := currentHarnessRuntimeDetail(selectedHarnessStatusPath(), commissionID)
		summary := harnessSessionLogSummaries(logPath)[stringField(runtimeDetail, "session_id")]
		latestTurn := harnessLatestCommissionLogSummary(logPath, commissionID)
		for _, line := range formatHarnessResult(
			commission,
			defaultHarnessWorkspaceRoot(),
			runtimeDetail,
			statusUpdatedAt,
			summary,
			latestTurn,
		) {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
				return err
			}
		}
		return nil
	})
}

func harnessResultCommissionID(
	ctx context.Context,
	store *artifact.Store,
	args []string,
) (string, error) {
	if len(args) == 1 {
		return strings.TrimSpace(args[0]), nil
	}

	statusIDs := harnessStatusCommissionIDs(selectedHarnessStatusPath())
	if len(statusIDs) == 1 {
		return statusIDs[0], nil
	}

	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", err
	}

	runnable := runnableHarnessCommissions(records)
	if len(runnable) == 1 {
		return stringField(runnable[0], "id"), nil
	}

	return "", fmt.Errorf("commission id is required when the current harness result is ambiguous")
}

func harnessStatusCommissionIDs(statusPath string) []string {
	encoded, err := os.ReadFile(statusPath)
	if err != nil {
		return nil
	}

	status := map[string]any{}
	if err := json.Unmarshal(encoded, &status); err != nil {
		return nil
	}

	orchestrator := mapField(status, "orchestrator")
	ids := []string{}
	for _, detail := range mapSliceField(orchestrator, "running_details") {
		ids = append(ids, stringField(detail, "commission_id"))
	}

	return uniqueStringsPreserveOrder(cleanStringSlice(ids))
}

func currentHarnessRuntimeDetail(statusPath string, commissionID string) (map[string]any, string) {
	encoded, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, ""
	}

	status := map[string]any{}
	if err := json.Unmarshal(encoded, &status); err != nil {
		return nil, ""
	}

	orchestrator := mapField(status, "orchestrator")
	for _, detail := range mapSliceField(orchestrator, "running_details") {
		if stringField(detail, "commission_id") == commissionID {
			return detail, stringField(status, "updated_at")
		}
	}

	return nil, stringField(status, "updated_at")
}

func formatHarnessResult(
	commission map[string]any,
	workspaceRoot string,
	runtimeDetail map[string]any,
	statusUpdatedAt string,
	runtimeSummary harnessSessionLogSummary,
	latestTurn harnessCommissionLogSummary,
) []string {
	commissionID := stringField(commission, "id")
	workspacePath := filepath.Join(workspaceRoot, commissionID)

	lines := []string{
		"Open-Sleigh harness result",
		"commission: " + presentOrUnknown(commissionID),
		"state: " + presentOrUnknown(stringField(commission, "state")),
		"decision: " + presentOrUnknown(stringField(commission, "decision_ref")),
		"plan: " + presentOrUnknown(stringField(commission, "implementation_plan_ref")),
		"workspace: " + workspacePath,
	}

	lines = append(lines, formatHarnessCurrentRuntime(runtimeDetail, statusUpdatedAt, runtimeSummary)...)
	lines = append(lines, formatHarnessLatestAgentTurn(latestTurn)...)
	lines = append(lines, formatHarnessResultEvents(mapSliceField(commission, "events"))...)
	lines = append(lines, formatHarnessWorkspaceGit(workspacePath)...)
	return lines
}

func formatHarnessCurrentRuntime(
	runtimeDetail map[string]any,
	statusUpdatedAt string,
	runtimeSummary harnessSessionLogSummary,
) []string {
	if len(runtimeDetail) == 0 {
		return nil
	}

	fields := []string{
		"phase=" + presentOrUnknown(stringField(runtimeDetail, "phase")),
		"sub_state=" + presentOrUnknown(stringField(runtimeDetail, "sub_state")),
		"session=" + presentOrUnknown(stringField(runtimeDetail, "session_id")),
		"task_pid=" + presentOrUnknown(stringField(runtimeDetail, "task_pid")),
		"workspace=" + presentOrUnknown(stringField(runtimeDetail, "workspace_path")),
	}

	if strings.TrimSpace(statusUpdatedAt) != "" {
		fields = append(fields, "status_updated_at="+statusUpdatedAt)
	}
	if strings.TrimSpace(runtimeSummary.LastEvent) != "" {
		fields = append(fields, "last_event="+runtimeSummary.LastEvent)
	}
	if strings.TrimSpace(runtimeSummary.LastEventAt) != "" {
		fields = append(fields, "last_event_at="+runtimeSummary.LastEventAt)
	}
	if strings.TrimSpace(runtimeSummary.LastTurnStatus) != "" {
		fields = append(fields, "last_turn="+runtimeSummary.LastTurnStatus)
	}
	if strings.TrimSpace(runtimeSummary.LastTurnID) != "" {
		fields = append(fields, "turn_id="+runtimeSummary.LastTurnID)
	}

	lines := []string{
		"current_runtime:",
		"- " + strings.Join(fields, " "),
	}
	if preview := strings.TrimSpace(runtimeSummary.LastTextPreview); preview != "" {
		lines = append(lines, "  preview="+preview)
	}
	return lines
}

func formatHarnessLatestAgentTurn(summary harnessCommissionLogSummary) []string {
	if strings.TrimSpace(summary.Event) == "" {
		return nil
	}

	fields := []string{
		"phase=" + presentOrUnknown(summary.Phase),
		"event=" + presentOrUnknown(summary.Event),
		"at=" + presentOrUnknown(summary.At),
	}
	if strings.TrimSpace(summary.TurnStatus) != "" {
		fields = append(fields, "status="+summary.TurnStatus)
	}
	if strings.TrimSpace(summary.TurnID) != "" {
		fields = append(fields, "turn_id="+summary.TurnID)
	}

	lines := []string{
		"last_agent_turn:",
		"- " + strings.Join(fields, " "),
	}
	if preview := strings.TrimSpace(summary.TextPreview); preview != "" {
		lines = append(lines, "  preview="+preview)
	}
	return lines
}

func formatHarnessResultEvents(events []map[string]any) []string {
	attemptEvents := currentHarnessAttemptEvents(events)

	lines := []string{"phase_events:"}
	outcomes := harnessPhaseOutcomeLines(attemptEvents)
	if len(outcomes) == 0 {
		lines = append(lines, "- none")
	} else {
		lines = append(lines, outcomes...)
	}

	lines = append(lines, "last_event: "+harnessLastEventLine(attemptEvents))
	return lines
}

func currentHarnessAttemptEvents(events []map[string]any) []map[string]any {
	if len(events) == 0 {
		return nil
	}

	start := 0
	for i, event := range events {
		if stringField(event, "event") == "commission_requeued" {
			start = i
		}
	}

	return events[start:]
}

func harnessPhaseOutcomeLines(events []map[string]any) []string {
	latest := map[string]string{}
	for _, event := range events {
		if stringField(event, "event") != "phase_outcome" {
			continue
		}

		payload := mapField(event, "payload")
		phase := stringField(payload, "phase")
		line := fmt.Sprintf(
			"- %s verdict=%s next=%s at=%s",
			presentOrUnknown(phase),
			presentOrUnknown(stringField(event, "verdict")),
			presentOrUnknown(stringField(payload, "next")),
			presentOrUnknown(stringField(event, "recorded_at")),
		)
		latest[phase] = line
	}

	lines := []string{}
	for _, phase := range []string{"preflight", "frame", "execute", "measure", "terminal"} {
		line, ok := latest[phase]
		if !ok {
			continue
		}
		lines = append(lines, line)
		delete(latest, phase)
	}

	extras := make([]string, 0, len(latest))
	for phase := range latest {
		extras = append(extras, phase)
	}
	sort.Strings(extras)
	for _, phase := range extras {
		lines = append(lines, latest[phase])
	}
	return lines
}

func harnessLastEventLine(events []map[string]any) string {
	if len(events) == 0 {
		return "none"
	}

	event := events[len(events)-1]
	parts := []string{
		presentOrUnknown(stringField(event, "event")),
		"action=" + presentOrUnknown(stringField(event, "action")),
		"verdict=" + presentOrUnknown(stringField(event, "verdict")),
		"reason=" + presentOrUnknown(stringField(event, "reason")),
		"at=" + presentOrUnknown(stringField(event, "recorded_at")),
	}
	return strings.Join(parts, " ")
}

func formatHarnessWorkspaceGit(workspacePath string) []string {
	if _, err := os.Stat(filepath.Join(workspacePath, ".git")); err != nil {
		return []string{"git_status: workspace not found or not a git repository"}
	}

	status := trimmedCommandOutput(workspacePath, "git", "status", "--short")
	diffStat := trimmedCommandOutput(workspacePath, "git", "diff", "--stat")

	lines := []string{"git_status:"}
	if status == "" {
		lines = append(lines, "- clean")
	} else {
		lines = append(lines, indentLines(status)...)
	}

	lines = append(lines, "diff_stat:")
	if diffStat == "" {
		lines = append(lines, "- empty")
	} else {
		lines = append(lines, indentLines(diffStat)...)
	}
	return lines
}

func trimmedCommandOutput(workdir string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output))
	}
	return strings.TrimSpace(string(output))
}

func indentLines(value string) []string {
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, "- "+line)
	}
	return lines
}

func selectedHarnessStatusPath() string {
	if strings.TrimSpace(harnessStatusPath) != "" {
		return harnessStatusPath
	}
	if env := strings.TrimSpace(os.Getenv("OPEN_SLEIGH_STATUS_PATH")); env != "" {
		return env
	}
	return defaultHarnessStatusPath()
}

func selectedHarnessLogPath() string {
	if strings.TrimSpace(harnessStatusLogPath) != "" {
		return harnessStatusLogPath
	}
	if env := strings.TrimSpace(os.Getenv("OPEN_SLEIGH_LOG_PATH")); env != "" {
		return env
	}
	return defaultHarnessLogPath()
}

func defaultHarnessStatusPath() string {
	return filepath.Join(os.Getenv("HOME"), ".open-sleigh", "status.json")
}

func defaultHarnessLogPath() string {
	return filepath.Join(os.Getenv("HOME"), ".open-sleigh", "runtime.jsonl")
}

func defaultHarnessWorkspaceRoot() string {
	return filepath.Join(os.Getenv("HOME"), ".open-sleigh", "workspaces")
}

func formatHarnessStatus(
	status map[string]any,
	statusPath string,
	logPath string,
	sessionSummaries map[string]harnessSessionLogSummary,
) []string {
	metadata := mapField(status, "metadata")
	orchestrator := mapField(status, "orchestrator")
	claimed := stringSliceField(orchestrator, "claimed")
	running := stringSliceField(orchestrator, "running")
	pendingHuman := stringSliceField(orchestrator, "pending_human")
	failures := mapSliceField(status, "failures")

	lines := []string{
		"Open-Sleigh harness status",
		"updated_at: " + stringField(status, "updated_at"),
		"status_path: " + statusPath,
		"runtime_log: " + logPath,
		"agent: " + presentOrUnknown(stringField(metadata, "agent_kind")),
		"tracker: " + presentOrUnknown(stringField(metadata, "tracker_kind")),
		"config: " + presentOrUnknown(stringField(metadata, "config_path")),
		"workspace_root: " + presentOrUnknown(stringField(metadata, "workspace_root")),
		"claimed: " + strconv.Itoa(len(claimed)),
		"running: " + strconv.Itoa(len(running)),
		"pending_human: " + strconv.Itoa(len(pendingHuman)),
		"failures: " + strconv.Itoa(len(failures)),
	}

	lines = append(
		lines,
		formatRunningDetails(mapSliceField(orchestrator, "running_details"), sessionSummaries, time.Now().UTC())...,
	)
	lines = append(lines, formatFailures(failures)...)
	return lines
}

func formatRunningDetails(
	details []map[string]any,
	sessionSummaries map[string]harnessSessionLogSummary,
	now time.Time,
) []string {
	if len(details) == 0 {
		return nil
	}

	lines := []string{"running_details:"}
	for _, detail := range details {
		lines = append(lines, formatRunningDetailLines(detail, sessionSummaries, now)...)
	}
	return lines
}

func formatRunningDetailLines(
	detail map[string]any,
	sessionSummaries map[string]harnessSessionLogSummary,
	now time.Time,
) []string {
	sessionID := presentOrUnknown(stringField(detail, "session_id"))
	sessionSummary := sessionSummaries[sessionID]
	fields := []string{
		"session=" + sessionID,
		"commission=" + presentOrUnknown(stringField(detail, "commission_id")),
		"phase=" + presentOrUnknown(stringField(detail, "phase")),
		"sub_state=" + presentOrUnknown(stringField(detail, "sub_state")),
		"task_pid=" + presentOrUnknown(stringField(detail, "task_pid")),
		"workspace=" + presentOrUnknown(stringField(detail, "workspace_path")),
	}

	if startedAt, elapsed := harnessSessionTiming(sessionSummary.StartedAt, now); startedAt != "" {
		fields = append(fields, "started_at="+startedAt, "elapsed="+elapsed)
	}
	if strings.TrimSpace(sessionSummary.LastEvent) != "" {
		fields = append(fields, "last_event="+sessionSummary.LastEvent)
	}
	if strings.TrimSpace(sessionSummary.LastEventAt) != "" {
		fields = append(fields, "last_event_at="+sessionSummary.LastEventAt)
	}
	if strings.TrimSpace(sessionSummary.LastTurnStatus) != "" {
		fields = append(fields, "last_turn="+sessionSummary.LastTurnStatus)
	}
	if strings.TrimSpace(sessionSummary.LastTurnID) != "" {
		fields = append(fields, "turn_id="+sessionSummary.LastTurnID)
	}

	lines := []string{"- " + strings.Join(fields, " ")}
	if preview := strings.TrimSpace(sessionSummary.LastTextPreview); preview != "" {
		lines = append(lines, "  preview="+preview)
	}
	return lines
}

func harnessSessionLogSummaries(logPath string) map[string]harnessSessionLogSummary {
	encoded, err := os.ReadFile(logPath)
	if err != nil {
		return map[string]harnessSessionLogSummary{}
	}

	lines := strings.Split(strings.TrimSpace(string(encoded)), "\n")
	summaries := map[string]harnessSessionLogSummary{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		data := mapField(event, "data")
		sessionID := strings.TrimSpace(stringField(data, "session_id"))
		if sessionID == "" {
			continue
		}

		at := strings.TrimSpace(stringField(event, "at"))
		eventName := strings.TrimSpace(stringField(event, "event"))
		summary := summaries[sessionID]

		if eventName == "session_started" && summary.StartedAt == "" {
			summary.StartedAt = at
		}
		if eventName != "" {
			summary.LastEvent = eventName
			summary.LastEventAt = at
		}

		switch eventName {
		case "agent_turn_started":
			summary.LastTurnStatus = "started"
			summary.LastTextPreview = ""
		case "agent_turn_completed":
			summary.LastTurnStatus = presentOrUnknown(stringField(data, "status"))
			summary.LastTurnID = strings.TrimSpace(stringField(data, "turn_id"))
			summary.LastTextPreview = harnessCompactPreview(stringField(data, "text_preview"))
		case "agent_turn_failed":
			summary.LastTurnStatus = "failed"
			summary.LastTextPreview = harnessCompactPreview(stringField(data, "reason"))
		}

		summaries[sessionID] = summary
	}
	return summaries
}

func harnessLatestCommissionLogSummary(logPath string, commissionID string) harnessCommissionLogSummary {
	if strings.TrimSpace(commissionID) == "" {
		return harnessCommissionLogSummary{}
	}

	encoded, err := os.ReadFile(logPath)
	if err != nil {
		return harnessCommissionLogSummary{}
	}

	lines := strings.Split(strings.TrimSpace(string(encoded)), "\n")
	summary := harnessCommissionLogSummary{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if strings.TrimSpace(stringField(event, "commission_id")) != commissionID {
			continue
		}

		eventName := strings.TrimSpace(stringField(event, "event"))
		if !isHarnessAgentTurnEvent(eventName) {
			continue
		}

		data := mapField(event, "data")
		summary = harnessCommissionLogSummary{
			Phase:       strings.TrimSpace(stringField(data, "phase")),
			Event:       eventName,
			At:          strings.TrimSpace(stringField(event, "at")),
			TurnID:      strings.TrimSpace(stringField(data, "turn_id")),
			TurnStatus:  harnessCommissionTurnStatus(eventName, data),
			TextPreview: harnessCommissionPreview(eventName, data),
		}
	}
	return summary
}

func isHarnessAgentTurnEvent(event string) bool {
	switch event {
	case "agent_turn_started", "agent_turn_completed", "agent_turn_failed":
		return true
	default:
		return false
	}
}

func harnessCommissionTurnStatus(eventName string, data map[string]any) string {
	switch eventName {
	case "agent_turn_started":
		return "started"
	case "agent_turn_failed":
		return "failed"
	default:
		return strings.TrimSpace(stringField(data, "status"))
	}
}

func harnessCommissionPreview(eventName string, data map[string]any) string {
	switch eventName {
	case "agent_turn_failed":
		return harnessCompactPreview(stringField(data, "reason"))
	default:
		return harnessCompactPreview(stringField(data, "text_preview"))
	}
}

func harnessSessionTiming(startedAt string, now time.Time) (string, string) {
	if strings.TrimSpace(startedAt) == "" {
		return "", ""
	}

	startTime, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return startedAt, "unknown"
	}

	return startedAt, now.Sub(startTime).Round(time.Second).String()
}

func harnessCompactPreview(value string) string {
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if cleaned == "" {
		return ""
	}
	if len(cleaned) <= 180 {
		return cleaned
	}
	return cleaned[:177] + "..."
}

func formatFailures(failures []map[string]any) []string {
	if len(failures) == 0 {
		return nil
	}

	lines := []string{"recent_failures:"}
	for _, failure := range failures {
		lines = append(lines, "- "+formatFailure(failure))
	}
	return lines
}

func formatFailure(failure map[string]any) string {
	fields := []string{
		"metric=" + presentOrUnknown(stringField(failure, "metric")),
		"reason=" + presentOrUnknown(stringField(failure, "reason")),
		"commission=" + presentOrUnknown(stringField(failure, "commission_id")),
		"session=" + presentOrUnknown(stringField(failure, "session_id")),
		"phase=" + presentOrUnknown(stringField(failure, "phase")),
	}
	return strings.Join(fields, " ")
}

func printHarnessRuntimeTail(
	cmd *cobra.Command,
	status map[string]any,
	logPath string,
	lineCount int,
) error {
	lines, err := recentHarnessLogLines(status, logPath, lineCount)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "runtime_events:"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func recentHarnessLogLines(status map[string]any, logPath string, lineCount int) ([]string, error) {
	encoded, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("read harness runtime log %s: %w", logPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(encoded)), "\n")
	filtered := filterHarnessLogLines(status, lines)
	if len(filtered) == 0 {
		filtered = lines
	}
	if len(filtered) <= lineCount {
		return filtered, nil
	}
	return filtered[len(filtered)-lineCount:], nil
}

func filterHarnessLogLines(status map[string]any, lines []string) []string {
	configPath := strings.TrimSpace(stringField(mapField(status, "metadata"), "config_path"))
	commissionSet := harnessRunningCommissionSet(status)
	if configPath == "" && len(commissionSet) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if harnessLogLineMatchesCurrentRun(event, configPath, commissionSet) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func harnessRunningCommissionSet(status map[string]any) map[string]struct{} {
	orchestrator := mapField(status, "orchestrator")
	details := mapSliceField(orchestrator, "running_details")
	set := make(map[string]struct{}, len(details))
	for _, detail := range details {
		commissionID := strings.TrimSpace(stringField(detail, "commission_id"))
		if commissionID == "" {
			continue
		}
		set[commissionID] = struct{}{}
	}
	return set
}

func harnessLogLineMatchesCurrentRun(
	event map[string]any,
	configPath string,
	commissionSet map[string]struct{},
) bool {
	if configPath != "" && stringField(mapField(event, "metadata"), "config_path") == configPath {
		return true
	}

	commissionID := strings.TrimSpace(stringField(event, "commission_id"))
	if commissionID == "" {
		return false
	}

	_, exists := commissionSet[commissionID]
	return exists
}

func mapField(payload map[string]any, key string) map[string]any {
	if value, ok := payload[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func mapSliceField(payload map[string]any, key string) []map[string]any {
	values, ok := payload[key].([]any)
	if !ok {
		return nil
	}

	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}

func presentOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func runHarnessRun(cmd *cobra.Command, decisionRefs []string) error {
	if harnessRunPlanPath != "" && harnessDecisionSelectorsPresent(decisionRefs) {
		return fmt.Errorf("use either decision selectors or --plan, not both")
	}

	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		if shouldRunExistingHarnessCommissions(decisionRefs) {
			handled, err := runExistingHarnessCommissions(ctx, cmd, store, projectRoot)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			return fmt.Errorf("no runnable WorkCommissions; create commissions explicitly with `haft harness run <decision-id> --prepare-only`, `haft harness run --problem <problem-id> --prepare-only`, or `haft commission create-from-decision <decision-id>`")
		}

		selectedRefs, err := harnessRunDecisionRefs(ctx, store, decisionRefs)
		if err != nil {
			return err
		}

		planPath, plan, err := harnessPlanForRun(projectRoot, selectedRefs)
		if err != nil {
			return err
		}

		created, result, err := ensureHarnessCommissions(ctx, store, projectRoot, plan)
		if err != nil {
			return err
		}

		opts := defaultHarnessRunOptions(projectRoot, planPath, plan)
		configPath, err := writeHarnessRuntimeConfig(projectRoot, opts)
		if err != nil {
			return err
		}

		if err := printHarnessRunSummary(cmd, planPath, configPath, created, result, opts); err != nil {
			return err
		}
		if opts.PrepareOnly {
			return nil
		}

		return startOpenSleigh(cmd, opts, configPath)
	})
}

func shouldRunExistingHarnessCommissions(decisionRefs []string) bool {
	return harnessRunPlanPath == "" && !harnessDecisionSelectorsPresent(decisionRefs)
}

func runExistingHarnessCommissions(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	projectRoot string,
) (bool, error) {
	planPath, plan, result, found, err := existingRunnableHarnessPlan(ctx, store, projectRoot)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	opts := defaultHarnessRunOptions(projectRoot, planPath, plan)
	configPath, err := writeHarnessRuntimeConfig(projectRoot, opts)
	if err != nil {
		return false, err
	}

	if err := printHarnessRunSummary(cmd, planPath, configPath, false, result, opts); err != nil {
		return false, err
	}
	if opts.PrepareOnly {
		return true, nil
	}

	return true, startOpenSleigh(cmd, opts, configPath)
}

func existingRunnableHarnessPlan(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) (string, map[string]any, string, bool, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", nil, "", false, err
	}

	runnable := runnableHarnessCommissions(records)
	if len(runnable) == 0 {
		return "", nil, "", false, nil
	}

	plan := existingRunnablePlan(runnable)
	planPath := existingRunnablePlanPath(projectRoot, plan)
	result := fmt.Sprintf("using %d existing runnable commission(s)", len(runnable))
	return planPath, plan, result, true, nil
}

func runnableHarnessCommissions(records []map[string]any) []map[string]any {
	runnable := make([]map[string]any, 0, len(records))
	now := time.Now().UTC()

	for _, commission := range records {
		if workCommissionRunnableForRequest(commission, records, nil, now) {
			runnable = append(runnable, commission)
		}
	}
	return runnable
}

func existingRunnablePlan(commissions []map[string]any) map[string]any {
	planRef, planRevision, ok := commonCommissionPlan(commissions)
	if !ok {
		return map[string]any{}
	}

	return map[string]any{
		"id":       planRef,
		"revision": planRevision,
	}
}

func commonCommissionPlan(commissions []map[string]any) (string, string, bool) {
	if len(commissions) == 0 {
		return "", "", false
	}

	firstPlanRef := stringField(commissions[0], "implementation_plan_ref")
	firstRevision := stringField(commissions[0], "implementation_plan_revision")
	if firstPlanRef == "" {
		return "", "", false
	}

	for _, commission := range commissions[1:] {
		if stringField(commission, "implementation_plan_ref") != firstPlanRef {
			return "", "", false
		}
		if stringField(commission, "implementation_plan_revision") != firstRevision {
			return "", "", false
		}
	}

	return firstPlanRef, firstRevision, true
}

func existingRunnablePlanPath(projectRoot string, plan map[string]any) string {
	planRef := stringField(plan, "id")
	if planRef == "" {
		return "(existing runnable commissions)"
	}

	path := filepath.Join(projectRoot, ".haft", "plans", planRef+".yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return "plan " + planRef + " (existing runnable commissions)"
}

func harnessRunDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	decisionRefs []string,
) ([]string, error) {
	if harnessRunPlanPath != "" {
		return nil, nil
	}
	return resolveHarnessDecisionRefs(ctx, store, decisionRefs)
}

func harnessDecisionSelectorsPresent(decisionRefs []string) bool {
	return len(decisionRefs) > 0 ||
		len(harnessPlanProblems) > 0 ||
		strings.TrimSpace(harnessPlanContext) != "" ||
		harnessPlanAllActive
}

func harnessPlanForRun(
	projectRoot string,
	decisionRefs []string,
) (string, map[string]any, error) {
	if harnessRunPlanPath != "" {
		plan, err := readCommissionPlanPayload(bytes.NewReader(nil), harnessRunPlanPath)
		return harnessRunPlanPath, plan, err
	}

	plan, err := buildHarnessPlan(projectRoot, decisionRefs)
	if err != nil {
		return "", nil, err
	}

	path, err := writeHarnessPlan(projectRoot, plan, harnessRunGeneratedPlanPath)
	if err != nil {
		return "", nil, err
	}

	mapped, err := harnessPlanFileMap(plan)
	if err != nil {
		return "", nil, err
	}
	return path, mapped, nil
}

func resolveHarnessDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	explicitRefs []string,
) ([]string, error) {
	selected := cleanStringSlice(explicitRefs)
	if len(selected) == 0 && !harnessPlanSelectorFlagsPresent() {
		refs, err := uncommissionedActiveDecisionRefs(ctx, store)
		if err != nil {
			return nil, err
		}
		if len(refs) == 0 {
			return nil, fmt.Errorf("no active decisions without WorkCommissions")
		}
		return defaultRunnableDecisionRefs(ctx, store, refs)
	}

	if harnessPlanAllActive {
		refs, err := activeDecisionRefs(ctx, store)
		if err != nil {
			return nil, err
		}
		selected = append(selected, refs...)
	}

	if strings.TrimSpace(harnessPlanContext) != "" {
		refs, err := decisionRefsForContext(ctx, store, harnessPlanContext)
		if err != nil {
			return nil, err
		}
		selected = append(selected, refs...)
	}

	for _, problemRef := range cleanStringSlice(harnessPlanProblems) {
		refs, err := decisionRefsForProblem(ctx, store, problemRef)
		if err != nil {
			return nil, err
		}
		selected = append(selected, refs...)
	}

	selected = uniqueStringsPreserveOrder(selected)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no active decisions matched the harness selectors")
	}

	if len(explicitRefs) > 0 || harnessPlanAllActive {
		return selected, nil
	}

	filtered, err := filterUncommissionedDecisionRefs(ctx, store, selected)
	if err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no selected active decisions without WorkCommissions")
	}
	if len(explicitRefs) == 0 && len(commissionFromDecisionAllowedPaths) == 0 {
		return defaultRunnableDecisionRefs(ctx, store, filtered)
	}
	return filtered, nil
}

func defaultRunnableDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	decisionRefs []string,
) ([]string, error) {
	if len(commissionFromDecisionAllowedPaths) > 0 {
		return decisionRefs, nil
	}

	refs, skipped, err := decisionRefsWithAffectedFiles(ctx, store, decisionRefs)
	if err != nil {
		return nil, err
	}
	if len(refs) > 0 {
		return refs, nil
	}

	return nil, fmt.Errorf(
		"no runnable commissions and no selected active decisions with affected_files; skipped %s. Add affected_files to a DecisionRecord or pass --allowed-path",
		strings.Join(skipped, ", "),
	)
}

func decisionRefsWithAffectedFiles(
	ctx context.Context,
	store *artifact.Store,
	decisionRefs []string,
) ([]string, []string, error) {
	refs := make([]string, 0, len(decisionRefs))
	skipped := make([]string, 0)

	for _, ref := range decisionRefs {
		files, err := store.GetAffectedFiles(ctx, ref)
		if err != nil {
			return nil, nil, fmt.Errorf("load decision affected files for %s: %w", ref, err)
		}
		if len(files) == 0 {
			skipped = append(skipped, ref)
			continue
		}
		refs = append(refs, ref)
	}

	return refs, skipped, nil
}

func harnessPlanSelectorFlagsPresent() bool {
	return len(harnessPlanProblems) > 0 ||
		strings.TrimSpace(harnessPlanContext) != "" ||
		harnessPlanAllActive
}

func activeDecisionRefs(ctx context.Context, store *artifact.Store) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list active decisions: %w", err)
	}

	return decisionRefsInCreationOrder(decisions), nil
}

func uncommissionedActiveDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list active decisions: %w", err)
	}

	refs := decisionRefsInCreationOrder(decisions)
	return filterUncommissionedDecisionRefs(ctx, store, refs)
}

func filterUncommissionedDecisionRefs(
	ctx context.Context,
	store *artifact.Store,
	decisionRefs []string,
) ([]string, error) {
	commissioned, err := commissionedDecisionRefSet(ctx, store)
	if err != nil {
		return nil, err
	}

	filtered := make([]string, 0, len(decisionRefs))
	for _, ref := range decisionRefs {
		if _, exists := commissioned[ref]; exists {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered, nil
}

func commissionedDecisionRefSet(
	ctx context.Context,
	store *artifact.Store,
) (map[string]struct{}, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return nil, err
	}

	commissioned := make(map[string]struct{}, len(records))
	for _, commission := range records {
		ref := stringField(commission, "decision_ref")
		if ref == "" {
			continue
		}
		commissioned[ref] = struct{}{}
	}
	return commissioned, nil
}

func decisionRefsForContext(
	ctx context.Context,
	store *artifact.Store,
	contextName string,
) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions for context %s: %w", contextName, err)
	}

	matching := make([]*artifact.Artifact, 0, len(decisions))
	for _, decision := range decisions {
		if decision.Meta.Context == strings.TrimSpace(contextName) {
			matching = append(matching, decision)
		}
	}

	refs := decisionRefsInCreationOrder(matching)
	if len(refs) == 0 {
		return nil, fmt.Errorf("no active decisions in context %s", contextName)
	}
	return refs, nil
}

func decisionRefsForProblem(
	ctx context.Context,
	store *artifact.Store,
	problemRef string,
) ([]string, error) {
	decisions, err := store.ListActiveByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions for problem %s: %w", problemRef, err)
	}

	matching := make([]*artifact.Artifact, 0, len(decisions))
	for _, decision := range decisions {
		fullDecision, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load decision %s: %w", decision.Meta.ID, err)
		}
		if decisionReferencesProblem(ctx, store, fullDecision, problemRef) {
			matching = append(matching, fullDecision)
		}
	}

	refs := decisionRefsInCreationOrder(matching)
	if len(refs) == 0 {
		return nil, fmt.Errorf("no active decisions linked to problem %s", problemRef)
	}
	return refs, nil
}

func decisionReferencesProblem(
	ctx context.Context,
	store *artifact.Store,
	decision *artifact.Artifact,
	problemRef string,
) bool {
	fields := decision.UnmarshalDecisionFields()
	refs := decisionProblemRefs(ctx, store, decision, fields)
	return stringSliceContains(refs, problemRef)
}

func decisionRefsInCreationOrder(decisions []*artifact.Artifact) []string {
	sorted := append([]*artifact.Artifact(nil), decisions...)
	sortArtifactsByCreatedAt(sorted)

	refs := make([]string, 0, len(sorted))
	for _, decision := range sorted {
		refs = append(refs, decision.Meta.ID)
	}
	return refs
}

func sortArtifactsByCreatedAt(items []*artifact.Artifact) {
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].Meta.CreatedAt
		right := items[j].Meta.CreatedAt
		if left.Equal(right) {
			return items[i].Meta.ID < items[j].Meta.ID
		}
		return left.Before(right)
	})
}

func uniqueStringsPreserveOrder(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func buildHarnessPlan(projectRoot string, decisionRefs []string) (harnessPlanFile, error) {
	cleanRefs := sortedUniqueDecisionRefs(decisionRefs)
	if len(cleanRefs) == 0 {
		return harnessPlanFile{}, fmt.Errorf("at least one decision id is required")
	}

	base, err := commissionPlanBaseParams(projectRoot, map[string]any{})
	if err != nil {
		return harnessPlanFile{}, err
	}

	planID := harnessPlanID
	if strings.TrimSpace(planID) == "" {
		planID = generatedHarnessPlanID(time.Now().UTC())
	}

	plan := harnessPlanFile{
		ID:               strings.TrimSpace(planID),
		Revision:         strings.TrimSpace(harnessPlanRevision),
		Title:            strings.TrimSpace(harnessPlanTitle),
		RepoRef:          stringField(base, "repo_ref"),
		BaseSHA:          stringField(base, "base_sha"),
		TargetBranch:     stringField(base, "target_branch"),
		ProjectionPolicy: stringField(base, "projection_policy"),
		ValidFor:         stringField(base, "valid_for"),
		ValidUntil:       stringField(base, "valid_until"),
		Defaults: harnessPlanDefaults{
			AllowedActions:       anyStrings(base["allowed_actions"]),
			EvidenceRequirements: anyStrings(base["evidence_requirements"]),
		},
		Decisions: harnessPlanDecisions(cleanRefs),
	}

	if plan.Title == "" {
		plan.Title = "Harness run " + plan.ID
	}

	return applyHarnessPlanDependencies(plan, harnessPlanDependencies, harnessPlanSequential)
}

func sortedUniqueDecisionRefs(decisionRefs []string) []string {
	refs := cleanStringSlice(decisionRefs)
	if harnessPlanSequential {
		return refs
	}
	return sortedUniqueStrings(refs)
}

func harnessPlanDecisions(decisionRefs []string) []harnessPlanDecision {
	decisions := make([]harnessPlanDecision, 0, len(decisionRefs))
	for _, ref := range decisionRefs {
		decisions = append(decisions, harnessPlanDecision{Ref: ref})
	}
	return decisions
}

func applyHarnessPlanDependencies(
	plan harnessPlanFile,
	edges []string,
	sequential bool,
) (harnessPlanFile, error) {
	indexByRef := make(map[string]int, len(plan.Decisions))
	for index, decision := range plan.Decisions {
		indexByRef[decision.Ref] = index
	}

	if sequential {
		for index := 1; index < len(plan.Decisions); index++ {
			previous := plan.Decisions[index-1].Ref
			plan.Decisions[index].DependsOn = append(plan.Decisions[index].DependsOn, previous)
		}
	}

	for _, edge := range edges {
		target, dependencies, err := parseHarnessDependency(edge)
		if err != nil {
			return harnessPlanFile{}, err
		}

		index, exists := indexByRef[target]
		if !exists {
			return harnessPlanFile{}, fmt.Errorf("dependency target %s is not in decisions", target)
		}
		for _, dependency := range dependencies {
			if _, exists := indexByRef[dependency]; !exists {
				return harnessPlanFile{}, fmt.Errorf("dependency source %s is not in decisions", dependency)
			}
		}

		plan.Decisions[index].DependsOn = append(plan.Decisions[index].DependsOn, dependencies...)
	}

	for index, decision := range plan.Decisions {
		plan.Decisions[index].DependsOn = sortedUniqueStrings(decision.DependsOn)
	}

	return plan, nil
}

func parseHarnessDependency(edge string) (string, []string, error) {
	parts := strings.SplitN(edge, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("--depend must look like target:source[,source]")
	}

	target := strings.TrimSpace(parts[0])
	dependencies := strings.Split(parts[1], ",")
	dependencies = cleanStringSlice(dependencies)
	if target == "" || len(dependencies) == 0 {
		return "", nil, fmt.Errorf("--depend must include target and at least one source")
	}

	return target, dependencies, nil
}

func writeHarnessPlan(projectRoot string, plan harnessPlanFile, out string) (string, error) {
	path := strings.TrimSpace(out)
	if path == "" {
		path = filepath.Join(projectRoot, ".haft", "plans", plan.ID+".yaml")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}

	data, err := yaml.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("encode plan YAML: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create plan directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write plan: %w", err)
	}

	return path, nil
}

func harnessPlanFileMap(plan harnessPlanFile) (map[string]any, error) {
	encoded, err := yaml.Marshal(plan)
	if err != nil {
		return nil, err
	}

	decoded := map[string]any{}
	if err := yaml.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}

	result := map[string]any{}
	if err := json.Unmarshal(normalized, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ensureHarnessCommissions(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	plan map[string]any,
) (bool, string, error) {
	count, err := countExistingHarnessPlanCommissions(ctx, store, plan)
	if err != nil {
		return false, "", err
	}
	if count > 0 && !harnessRunForceCreate {
		return false, fmt.Sprintf("reused %d existing commission(s)", count), nil
	}

	args, err := commissionFromPlanCLIParams(projectRoot, plan)
	if err != nil {
		return false, "", err
	}

	result, err := handleHaftCommission(ctx, store, args)
	if err != nil {
		return false, "", err
	}
	return true, result, nil
}

func countExistingHarnessPlanCommissions(
	ctx context.Context,
	store *artifact.Store,
	plan map[string]any,
) (int, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return 0, err
	}

	count := 0
	planRef := stringField(plan, "id")
	planRevision := stringField(plan, "revision")
	for _, commission := range records {
		if stringField(commission, "implementation_plan_ref") != planRef {
			continue
		}
		if stringField(commission, "implementation_plan_revision") != planRevision {
			continue
		}
		count++
	}
	return count, nil
}

func defaultHarnessRunOptions(
	projectRoot string,
	planPath string,
	plan map[string]any,
) harnessRunOptions {
	statusPath := harnessRunStatusPath
	if strings.TrimSpace(statusPath) == "" {
		statusPath = defaultHarnessStatusPath()
	}

	logPath := harnessRunLogPath
	if strings.TrimSpace(logPath) == "" {
		logPath = defaultHarnessLogPath()
	}

	workspaceRoot := harnessRunWorkspaceRoot
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = defaultHarnessWorkspaceRoot()
	}

	repoURL := harnessRunRepoURL
	if strings.TrimSpace(repoURL) == "" {
		repoURL = projectRoot
	}

	runtimePath := harnessRunRuntimePath
	if strings.TrimSpace(runtimePath) == "" {
		runtimePath = os.Getenv("HAFT_OPEN_SLEIGH_RUNTIME")
	}
	if strings.TrimSpace(runtimePath) == "" {
		runtimePath = os.Getenv("OPEN_SLEIGH_RUNTIME")
	}
	if strings.TrimSpace(runtimePath) == "" {
		runtimePath = defaultOpenSleighRuntimePath(projectRoot)
	}

	return harnessRunOptions{
		PlanPath:           planPath,
		PrepareOnly:        harnessRunPrepareOnly,
		ForceCreate:        harnessRunForceCreate,
		Once:               harnessRunOnce,
		OnceTimeoutMS:      harnessRunOnceTimeoutMS,
		Mock:               harnessRunMock,
		MockAgent:          harnessRunMockAgent || harnessRunMock,
		MockJudge:          harnessRunMockJudge || harnessRunMock,
		Concurrency:        positiveOrDefault(harnessRunConcurrency, 2),
		MaxClaims:          positiveOrDefault(harnessRunMaxClaims, 50),
		PollIntervalMS:     positiveOrDefault(harnessRunPollIntervalMS, 30000),
		StatusPath:         statusPath,
		LogPath:            logPath,
		WorkspaceRoot:      workspaceRoot,
		RuntimePath:        runtimePath,
		RepoURL:            repoURL,
		AgentMaxTurns:      20,
		AgentTurnTimeoutMS: 3600000,
		AgentWallClockS:    600,
		JudgeWallClockS:    120,
		ApprovalPolicy:     "never",
		ThreadSandbox:      "workspace-write",
		ProjectionMode:     "local_only",
		ProjectionProfile:  "manager_plain",
		ExternalApprover:   "ivan@weareocta.com",
		StatusHTTPPort:     4767,
		CommissionPlanRef:  stringField(plan, "id"),
	}
}

func writeHarnessRuntimeConfig(
	projectRoot string,
	opts harnessRunOptions,
) (string, error) {
	if err := validateOpenSleighRuntime(opts.RuntimePath); err != nil {
		return "", err
	}

	frontmatter, err := yaml.Marshal(harnessRuntimeConfig(projectRoot, opts))
	if err != nil {
		return "", fmt.Errorf("encode Open-Sleigh config: %w", err)
	}

	file, err := os.CreateTemp("", "sleigh.harness-*.md")
	if err != nil {
		return "", fmt.Errorf("create Open-Sleigh config: %w", err)
	}
	defer file.Close()

	content := "---\n" + string(frontmatter) + "---\n\n" + harnessPromptTemplates()
	if _, err := file.WriteString(content); err != nil {
		return "", fmt.Errorf("write Open-Sleigh config: %w", err)
	}

	return file.Name(), nil
}

func defaultOpenSleighRuntimePath(projectRoot string) string {
	projectRuntime := filepath.Join(projectRoot, "open-sleigh")
	if openSleighRuntimeExists(projectRuntime) {
		return projectRuntime
	}

	installedRuntime := installedOpenSleighRuntimePath()
	if openSleighRuntimeExists(installedRuntime) {
		return installedRuntime
	}

	return projectRuntime
}

func installedOpenSleighRuntimePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return ""
	}
	return filepath.Join(homeDir, ".haft", "runtimes", "open-sleigh", "current")
}

func openSleighRuntimeExists(runtimePath string) bool {
	_, err := openSleighRuntimeKind(runtimePath)
	return err == nil
}

func openSleighRuntimeKind(runtimePath string) (string, error) {
	if strings.TrimSpace(runtimePath) == "" {
		return "", fmt.Errorf("open-sleigh runtime path is empty")
	}

	info, err := os.Stat(runtimePath)
	if err != nil {
		return "", fmt.Errorf(
			"open-sleigh runtime not found at %s; run from the Haft monorepo, pass --runtime, or set HAFT_OPEN_SLEIGH_RUNTIME",
			runtimePath,
		)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("open-sleigh runtime path is not a directory: %s", runtimePath)
	}

	if _, err := os.Stat(openSleighReleaseExecutable(runtimePath)); err == nil {
		return "release", nil
	}
	if _, err := os.Stat(filepath.Join(runtimePath, "mix.exs")); err == nil {
		return "source", nil
	}

	return "", fmt.Errorf("open-sleigh runtime at %s is missing bin/open_sleigh or mix.exs", runtimePath)
}

func validateOpenSleighRuntime(runtimePath string) error {
	kind, err := openSleighRuntimeKind(runtimePath)
	if err != nil {
		return err
	}

	if kind == "source" {
		return validateOpenSleighSourceRuntime()
	}
	return nil
}

func validateOpenSleighSourceRuntime() error {
	if _, err := exec.LookPath("mix"); err != nil {
		return fmt.Errorf("mix is required to run Open-Sleigh; install Elixir 1.18+ or use --prepare-only")
	}
	return nil
}

func harnessRuntimeConfig(projectRoot string, opts harnessRunOptions) map[string]any {
	haftCommand := harnessHaftCommand(projectRoot)

	return map[string]any{
		"engine": map[string]any{
			"poll_interval_ms":   opts.PollIntervalMS,
			"status_path":        opts.StatusPath,
			"status_interval_ms": 5000,
			"log_path":           opts.LogPath,
			"concurrency":        opts.Concurrency,
			"status_http": map[string]any{
				"enabled": opts.StatusHTTPEnabled,
				"host":    "127.0.0.1",
				"port":    opts.StatusHTTPPort,
			},
		},
		"commission_source": map[string]any{
			"kind":            "haft",
			"selector":        "runnable",
			"max_claims":      opts.MaxClaims,
			"lease_timeout_s": 300,
			"plan_ref":        opts.CommissionPlanRef,
			"queue":           nil,
		},
		"projection": map[string]any{
			"mode":           opts.ProjectionMode,
			"targets":        []any{},
			"writer_profile": opts.ProjectionProfile,
		},
		"agent": map[string]any{
			"kind":                  "codex",
			"version_pin":           "local",
			"command":               "codex app-server",
			"max_turns":             opts.AgentMaxTurns,
			"max_tokens_per_turn":   80000,
			"wall_clock_timeout_s":  opts.AgentWallClockS,
			"max_retry_backoff_ms":  300000,
			"max_concurrent_agents": opts.Concurrency,
		},
		"codex": map[string]any{
			"approval_policy": opts.ApprovalPolicy,
			"thread_sandbox":  opts.ThreadSandbox,
			"turn_sandbox_policy": map[string]any{
				"type": "workspaceWrite",
			},
			"read_timeout_ms":  5000,
			"turn_timeout_ms":  opts.AgentTurnTimeoutMS,
			"stall_timeout_ms": 300000,
		},
		"judge": map[string]any{
			"kind":                 "codex",
			"adapter_version":      "mvp1-judge",
			"max_tokens_per_turn":  4000,
			"wall_clock_timeout_s": opts.JudgeWallClockS,
		},
		"workspace": map[string]any{
			"root":           opts.WorkspaceRoot,
			"cleanup_policy": "keep",
		},
		"hooks": map[string]any{
			"timeout_ms": 60000,
			"failure_policy": map[string]any{
				"after_create": "blocking",
				"before_run":   "blocking",
				"after_run":    "warning",
			},
			"after_create":  "git clone --depth 1 " + shellEscape(opts.RepoURL) + " .\nmix deps.get || true",
			"before_run":    nil,
			"after_run":     nil,
			"before_remove": nil,
		},
		"haft": map[string]any{
			"command": haftCommand,
			"version": "local",
		},
		"external_publication": map[string]any{
			"branch_regex":          "^(main|master|release/.*)$",
			"tracker_transition_to": []any{},
			"approvers":             []string{opts.ExternalApprover},
			"timeout_h":             24,
		},
		"phases": harnessPhaseConfig(),
	}
}

func harnessPhaseConfig() map[string]any {
	return map[string]any{
		"preflight": map[string]any{
			"agent_role": "preflight_checker",
			"tools":      []string{"haft_query", "read", "grep", "bash"},
			"gates": map[string]any{
				"structural": []string{"commission_runnable", "decision_fresh", "scope_snapshot_fresh"},
				"semantic":   []string{},
			},
		},
		"frame": map[string]any{
			"agent_role": "frame_verifier",
			"tools":      []string{"haft_query", "read", "grep"},
			"gates": map[string]any{
				"structural": []string{"problem_card_ref_present", "valid_until_field_present"},
				"semantic":   []string{},
			},
		},
		"execute": map[string]any{
			"agent_role": "executor",
			"tools":      []string{"read", "write", "edit", "bash", "haft_note"},
			"gates": map[string]any{
				"structural": []string{"design_runtime_split_ok"},
				"semantic":   []string{"lade_quadrants_split_ok"},
			},
		},
		"measure": map[string]any{
			"agent_role": "measurer",
			"tools":      []string{"bash", "read", "grep", "haft_query", "haft_decision", "haft_refresh"},
			"gates": map[string]any{
				"structural": []string{"evidence_ref_not_self", "valid_until_field_present"},
				"semantic":   []string{"no_self_evidence_semantic"},
			},
		},
	}
}

func harnessPromptTemplates() string {
	return strings.Join([]string{
		"# Prompt templates",
		"",
		"## Preflight",
		strings.Join([]string{
			"You are the Commission Preflight checker for WorkCommission {{commission.id}}.",
			"",
			"Use this authoritative WorkCommission snapshot. Do not discover the commission by scanning the repository.",
			"",
			"```json",
			"{{commission.json}}",
			"```",
			"",
			"Task:",
			"- Inspect only the linked ProblemCard, linked DecisionRecord, current git status, current HEAD, and the allowed_paths in the scope.",
			"- Do not implement, edit files, create commits, or widen scope.",
			"- Do not search the whole repository unless a listed allowed_path or linked artifact is missing.",
			"- Report a concise PreflightReport with: commission_id, decision_ref, problem_card_ref, current_head, workspace_dirty, material_context_change, reason.",
			"- If the deterministic snapshot looks stale or uncertain, say so directly. You do not authorize execution; Haft decides after validating deterministic preflight facts.",
		}, "\n"),
		"",
		"## Frame",
		strings.Join([]string{
			"You are the Frame verifier for WorkCommission {{commission.id}}.",
			"",
			"Linked ProblemCardRef: {{commission.problem_card_ref}}",
			"",
			"ProblemCard:",
			"{{problem_card.body}}",
			"",
			"Verify that this upstream problem frame is present and still compatible with the WorkCommission. Do not implement or edit files in this phase.",
		}, "\n"),
		"",
		"## Execute",
		strings.Join([]string{
			"You are the Executor for WorkCommission {{commission.id}}.",
			"",
			"Use this authoritative WorkCommission snapshot. Do not rediscover the commission by scanning the repository.",
			"",
			"```json",
			"{{commission.json}}",
			"```",
			"",
			"Linked ProblemCard:",
			"{{problem_card.body}}",
			"",
			"Task:",
			"- Implement DecisionRecord {{decision.id}} inside the bounded scope only.",
			"- Do not stop at analysis, narration, or a plan. Make the actual file changes now if the commission is runnable.",
			"- Edit only files inside `scope.allowed_paths` and do not widen scope.",
			"- Leave final evidence adjudication to the Measure phase.",
		}, "\n"),
		"",
		"## Measure",
		strings.Join([]string{
			"You are the Measurer for WorkCommission {{commission.id}}.",
			"",
			"Use this authoritative WorkCommission snapshot.",
			"",
			"```json",
			"{{commission.json}}",
			"```",
			"",
			"Linked ProblemCard:",
			"{{problem_card.body}}",
			"",
			"Task:",
			"- Run or inspect the required evidence listed in the commission snapshot.",
			"- Do not edit product files in this phase.",
			"- Record an honest measurement with concrete evidence: pass, partial, or failed.",
		}, "\n"),
		"",
	}, "\n")
}

func printHarnessRunSummary(
	cmd *cobra.Command,
	planPath string,
	configPath string,
	created bool,
	commissionResult string,
	opts harnessRunOptions,
) error {
	mode := "long-running"
	if opts.Once {
		mode = "once"
	}
	if opts.MockAgent && opts.MockJudge {
		mode += " mock"
	}

	lines := []string{
		"Plan: " + planPath,
		"Open-Sleigh config: " + configPath,
		"Status: " + opts.StatusPath,
		"Runtime log: " + opts.LogPath,
		"Mode: " + mode,
	}

	if created {
		lines = append(lines, "Commissions: created")
	} else {
		lines = append(lines, "Commissions: "+commissionResult)
	}
	if !opts.PrepareOnly {
		lines = append(lines, "Observe: haft harness status --tail 20")
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func startOpenSleigh(cmd *cobra.Command, opts harnessRunOptions, configPath string) error {
	args := openSleighStartArgs(opts, configPath)
	kind, err := openSleighRuntimeKind(opts.RuntimePath)
	if err != nil {
		return err
	}
	if kind == "release" {
		return startOpenSleighRelease(cmd, opts, args)
	}
	return startOpenSleighSource(cmd, opts, args)
}

func openSleighStartArgs(opts harnessRunOptions, configPath string) []string {
	args := []string{"--path", configPath}
	if opts.Once {
		args = append(args, "--once", "--once-timeout-ms="+strconv.Itoa(opts.OnceTimeoutMS))
	}
	if opts.MockAgent {
		args = append(args, "--mock-agent")
	}
	if opts.MockJudge {
		args = append(args, "--mock-judge")
	}
	return args
}

func startOpenSleighSource(cmd *cobra.Command, opts harnessRunOptions, args []string) error {
	mixArgs := append([]string{"open_sleigh.start"}, args...)
	process := exec.Command("mix", mixArgs...)
	return runOpenSleighProcess(cmd, opts, process)
}

func startOpenSleighRelease(cmd *cobra.Command, opts harnessRunOptions, args []string) error {
	expression := "Application.ensure_all_started(:mix); Mix.Tasks.OpenSleigh.Start.run(" +
		elixirStringListLiteral(args) +
		")"

	process := exec.Command(openSleighReleaseExecutable(opts.RuntimePath), "eval", expression)
	return runOpenSleighProcess(cmd, opts, process)
}

func openSleighReleaseExecutable(runtimePath string) string {
	return filepath.Join(runtimePath, "bin", "open_sleigh")
}

func runOpenSleighProcess(cmd *cobra.Command, opts harnessRunOptions, process *exec.Cmd) error {
	process.Dir = opts.RuntimePath
	process.Env = append(os.Environ(), "REPO_URL="+opts.RepoURL)
	process.Stdout = cmd.OutOrStdout()
	process.Stderr = cmd.ErrOrStderr()
	process.Stdin = cmd.InOrStdin()

	return process.Run()
}

func elixirStringListLiteral(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, elixirStringLiteral(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func elixirStringLiteral(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return `"` + replacer.Replace(value) + `"`
}

func harnessHaftCommand(projectRoot string) string {
	executable, err := os.Executable()
	if err != nil || strings.TrimSpace(executable) == "" {
		executable = "haft"
	}

	return "HAFT_PROJECT_ROOT=" + shellEscape(projectRoot) + " " + shellEscape(executable) + " serve"
}

func generatedHarnessPlanID(now time.Time) string {
	return "plan-" + now.Format("20060102-150405")
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func positiveOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func anyStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return cleanStringSlice(typed)
	case []any:
		return anySliceToStrings(typed)
	default:
		return nil
	}
}
