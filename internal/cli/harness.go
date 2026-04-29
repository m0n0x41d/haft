package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/implementationplan"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/scopeauth"
	"github.com/m0n0x41d/haft/internal/workcommission"
)

type harnessPlanFile struct {
	ID               string                `yaml:"id"`
	Revision         string                `yaml:"revision"`
	Title            string                `yaml:"title,omitempty"`
	RepoRef          string                `yaml:"repo_ref"`
	BaseSHA          string                `yaml:"base_sha"`
	TargetBranch     string                `yaml:"target_branch"`
	ProjectionPolicy string                `yaml:"projection_policy"`
	DeliveryPolicy   string                `yaml:"delivery_policy"`
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
	PlanPath               string
	GeneratedPlanPath      string
	PrepareOnly            bool
	ForceCreate            bool
	Drain                  bool
	Once                   bool
	OnceTimeoutMS          int
	Detach                 bool
	Mock                   bool
	MockAgent              bool
	MockJudge              bool
	Concurrency            int
	MaxClaims              int
	PollIntervalMS         int
	StaleLeaseMaxAgeS      int
	StatusPath             string
	LogPath                string
	WorkspaceRoot          string
	RuntimePath            string
	RepoURL                string
	AgentMaxTurns          int
	AgentTurnTimeoutMS     int
	AgentWallClockS        int
	JudgeWallClockS        int
	ApprovalPolicy         string
	ThreadSandbox          string
	ProjectionMode         string
	ProjectionProfile      string
	ExternalApprover       string
	StatusHTTPEnabled      bool
	StatusHTTPPort         int
	CommissionRunnerID     string
	CommissionPlanRef      string
	CommissionPlanRevision string
	CommissionQueueName    string
}

type harnessRunReadinessKind string

const (
	harnessRunReadinessAdmissible      harnessRunReadinessKind = "admissible"
	harnessRunReadinessBlocked         harnessRunReadinessKind = "blocked"
	harnessRunReadinessTacticalAllowed harnessRunReadinessKind = "tactical_allowed"
)

type harnessRunReadinessGate struct {
	Kind           harnessRunReadinessKind
	ProjectStatus  project.Readiness
	BlockReason    string
	OverrideReason string
}

type harnessRunSelection struct {
	CommissionIDs []string
	DecisionRefs  []string
	PlanRef       string
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

type harnessTerminalCommissionSummary struct {
	CommissionID string
	State        string
	DecisionRef  string
	PlanRef      string
	LastEvent    string
	LastVerdict  string
	RecordedAt   string
	Workspace    string
	Preview      string
}

type harnessWorkspaceDiffState string

const (
	harnessWorkspaceDiffUnavailable harnessWorkspaceDiffState = "workspace_unavailable"
	harnessWorkspaceDiffClean       harnessWorkspaceDiffState = "clean"
	harnessWorkspaceDiffChanged     harnessWorkspaceDiffState = "changed"
)

type harnessWorkspaceGitSummary struct {
	State    harnessWorkspaceDiffState
	Status   string
	DiffStat string
	Changed  []string
	Error    string
}

type harnessOperatorActionKind string

const (
	harnessOperatorActionWait          harnessOperatorActionKind = "wait"
	harnessOperatorActionInspect       harnessOperatorActionKind = "inspect"
	harnessOperatorActionApply         harnessOperatorActionKind = "apply"
	harnessOperatorActionRerunEvidence harnessOperatorActionKind = "rerun_evidence"
)

type harnessOperatorAction struct {
	Kind                harnessOperatorActionKind
	Lines               []string
	Reason              string
	ApplyDisabledReason scopeauth.BlockingReason
}

type harnessApplySummary struct {
	CommissionID string
	Workspace    string
	ProjectRoot  string
	Files        []string
	Commit       string
}

type harnessStaleLeaseSummary struct {
	CommissionID string
	State        string
	FetchedAt    string
	Age          string
	MaxAgeS      int
}

type harnessDrainMonitor struct {
	OpenIDs      []string
	ObservedIDs  []string
	StaleLeases  []harnessStaleLeaseSummary
	RunnableIDs  []string
	ExecutingIDs []string
}

const defaultHarnessStaleLeaseMaxAgeS = 24 * 60 * 60

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
	harnessRunDrain             bool
	harnessRunOnce              bool
	harnessRunOnceTimeoutMS     int
	harnessRunMock              bool
	harnessRunMockAgent         bool
	harnessRunMockJudge         bool
	harnessRunDetach            bool
	harnessRunConcurrency       int
	harnessRunMaxClaims         int
	harnessRunPollIntervalMS    int
	harnessRunStaleLeaseMaxAgeS int
	harnessRunStatusPath        string
	harnessRunLogPath           string
	harnessRunWorkspaceRoot     string
	harnessRunRuntimePath       string
	harnessRunRepoURL           string
	harnessRunTacticalReason    string

	harnessStatusPath    string
	harnessStatusLogPath string
	harnessStatusJSON    bool
	harnessStatusTail    int

	harnessWatchStatusPath string
	harnessWatchLogPath    string
	harnessWatchTail       int
	harnessWatchIntervalMS int

	harnessTailStatusPath string
	harnessTailLogPath    string
	harnessTailFollow     bool
	harnessTailJSON       bool
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

By default, run opens an append-only operator stream in this terminal and exits
when the selected WorkCommissions reach terminal states. Use --detach to start
the runtime and return immediately.

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
	Use:   "status [commission-id]",
	Short: "Show the latest Haft Harness status",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runHarnessStatus,
}

var harnessWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch Haft Harness status live",
	RunE:  runHarnessWatch,
}

var harnessTailCmd = &cobra.Command{
	Use:          "tail [commission-id]",
	Short:        "Show harness runtime events for a commission",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runHarnessTail,
}

var harnessResultCmd = &cobra.Command{
	Use:          "result [commission-id]",
	Short:        "Show the latest harness result and workspace diff",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runHarnessResult,
}

var harnessApplyCmd = &cobra.Command{
	Use:          "apply <commission-id>",
	Short:        "Apply a completed harness workspace diff to the current project",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runHarnessApply,
}

func init() {
	registerCommissionFromDecisionFlags(harnessPlanCmd)
	registerHarnessPlanFlags(harnessPlanCmd)

	registerCommissionFromDecisionFlags(harnessRunCmd)
	registerHarnessPlanFlags(harnessRunCmd)
	registerHarnessRunFlags(harnessRunCmd)
	registerHarnessStatusFlags(harnessStatusCmd)
	registerHarnessWatchFlags(harnessWatchCmd)
	registerHarnessTailFlags(harnessTailCmd)

	harnessCmd.AddCommand(harnessPlanCmd)
	harnessCmd.AddCommand(harnessRunCmd)
	harnessCmd.AddCommand(harnessStatusCmd)
	harnessCmd.AddCommand(harnessWatchCmd)
	harnessCmd.AddCommand(harnessTailCmd)
	harnessCmd.AddCommand(harnessResultCmd)
	harnessCmd.AddCommand(harnessApplyCmd)
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
	cmd.Flags().BoolVar(&harnessRunDrain, "drain", false, "Keep running until no runnable WorkCommissions remain")
	cmd.Flags().BoolVar(&harnessRunOnce, "once", false, "Run one Open-Sleigh polling pass")
	cmd.Flags().IntVar(&harnessRunOnceTimeoutMS, "once-timeout-ms", 8000, "Open-Sleigh --once timeout in milliseconds")
	cmd.Flags().BoolVar(&harnessRunMock, "mock", false, "Use mock agent and mock judge")
	cmd.Flags().BoolVar(&harnessRunMockAgent, "mock-agent", false, "Use mock agent")
	cmd.Flags().BoolVar(&harnessRunMockJudge, "mock-judge", false, "Use mock judge")
	cmd.Flags().BoolVar(&harnessRunDetach, "detach", false, "Start Open-Sleigh and return without the operator stream")
	cmd.Flags().IntVar(&harnessRunConcurrency, "concurrency", 2, "Open-Sleigh engine concurrency")
	cmd.Flags().IntVar(&harnessRunMaxClaims, "max-claims", 50, "Maximum commission claims per poll")
	cmd.Flags().IntVar(&harnessRunPollIntervalMS, "poll-interval-ms", 30000, "Open-Sleigh poll interval")
	cmd.Flags().IntVar(&harnessRunStaleLeaseMaxAgeS, "stale-lease-max-age-s", defaultHarnessStaleLeaseMaxAgeS, "Skip claimed WorkCommission leases older than this age in seconds; 0 disables the cap")
	cmd.Flags().StringVar(&harnessRunStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessRunLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().StringVar(&harnessRunWorkspaceRoot, "workspace-root", "", "Open-Sleigh workspace root")
	cmd.Flags().StringVar(&harnessRunRuntimePath, "runtime", "", "Open-Sleigh runtime directory (default: project open-sleigh or installed ~/.haft runtime)")
	cmd.Flags().StringVar(&harnessRunRepoURL, "repo-url", "", "Repository URL/path cloned into workspaces (default: project root)")
	cmd.Flags().StringVar(&harnessRunTacticalReason, "tactical-override-reason", "", "Allow needs_onboard harness work and record each selected commission as out-of-spec tactical with this reason")
}

func registerHarnessStatusFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessStatusLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().BoolVar(&harnessStatusJSON, "json", false, "Print raw status JSON")
	cmd.Flags().IntVar(&harnessStatusTail, "tail", 0, "Print the last N runtime log events")
}

func registerHarnessWatchFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessWatchStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessWatchLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().IntVar(&harnessWatchTail, "tail", 10, "Print the last N runtime log events")
	cmd.Flags().IntVar(&harnessWatchIntervalMS, "interval-ms", 1000, "Refresh interval in milliseconds")
}

func registerHarnessTailFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&harnessTailStatusPath, "status-path", "", "Status JSON path")
	cmd.Flags().StringVar(&harnessTailLogPath, "log-path", "", "Runtime JSONL log path")
	cmd.Flags().BoolVar(&harnessTailFollow, "follow", false, "Follow new runtime events")
	cmd.Flags().BoolVar(&harnessTailJSON, "json", false, "Print raw JSONL events")
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

func runHarnessStatus(cmd *cobra.Command, args []string) error {
	if len(args) == 1 && !harnessStatusJSON {
		return runHarnessResult(cmd, args)
	}

	statusPath := selectedHarnessStatusPath()
	logPath := selectedHarnessLogPath()

	encoded, status, err := readHarnessStatus(statusPath)
	if err != nil {
		return err
	}

	if harnessStatusJSON {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(string(encoded)))
		return err
	}

	store, cleanup := openOptionalHarnessStore()
	defer cleanup()

	recent := loadHarnessRecentTerminalSummaries(context.Background(), store, logPath, 5)
	for _, line := range formatHarnessStatus(status, statusPath, logPath, harnessSessionLogSummaries(logPath), recent) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}

	if harnessStatusTail <= 0 {
		return nil
	}

	return printHarnessRuntimeTail(cmd, status, logPath, harnessStatusTail)
}

func runHarnessWatch(cmd *cobra.Command, _args []string) error {
	statusPath := selectedHarnessStatusPathValue(harnessWatchStatusPath)
	logPath := selectedHarnessLogPathValue(harnessWatchLogPath)
	intervalMS := harnessWatchIntervalMS
	if intervalMS < 200 {
		intervalMS = 200
	}

	store, cleanup := openOptionalHarnessStore()
	defer cleanup()

	for {
		lines, err := renderHarnessWatchFrame(statusPath, logPath, harnessWatchTail, store)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprint(cmd.OutOrStdout(), "\x1b[H\x1b[2J"); err != nil {
			return err
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
				return err
			}
		}
		time.Sleep(time.Duration(intervalMS) * time.Millisecond)
	}
}

func runHarnessTail(cmd *cobra.Command, args []string) error {
	statusPath := selectedHarnessStatusPathValue(harnessTailStatusPath)
	logPath := selectedHarnessLogPathValue(harnessTailLogPath)
	commissionID, err := harnessTailCommissionID(args, statusPath)
	if err != nil {
		return err
	}

	offset, err := printHarnessTailSnapshot(cmd, logPath, commissionID, harnessTailJSON)
	if err != nil {
		return err
	}
	if !harnessTailFollow {
		return nil
	}

	store, cleanup := openOptionalHarnessStore()
	defer cleanup()

	for {
		time.Sleep(time.Second)
		nextOffset, _, err := printHarnessTailSince(cmd, logPath, commissionID, offset, harnessTailJSON)
		if err != nil {
			return err
		}
		offset = nextOffset
		if harnessTailJSON {
			continue
		}

		line, terminal, err := harnessTailFollowCompletionLine(context.Background(), store, commissionID)
		if err != nil {
			return err
		}
		if terminal {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), line)
			return err
		}
	}
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

func runHarnessApply(cmd *cobra.Command, args []string) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		commissionID := strings.TrimSpace(args[0])
		commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
		if err != nil {
			return err
		}

		summary, err := applyHarnessWorkspaceDiff(
			projectRoot,
			filepath.Join(defaultHarnessWorkspaceRoot(), commissionID),
			harnessCommissionScope(commission),
		)
		if err != nil {
			return err
		}

		for _, line := range formatHarnessApplySummary(summary) {
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

func harnessTailCommissionID(args []string, statusPath string) (string, error) {
	if len(args) == 1 {
		return strings.TrimSpace(args[0]), nil
	}

	statusIDs := harnessStatusCommissionIDs(statusPath)
	if len(statusIDs) == 1 {
		return statusIDs[0], nil
	}
	return "", fmt.Errorf("commission id is required when no single harness run is active")
}

func renderHarnessWatchFrame(
	statusPath string,
	logPath string,
	tail int,
	store *artifact.Store,
) ([]string, error) {
	_, status, err := readHarnessStatus(statusPath)
	if err != nil {
		return nil, err
	}

	recent := loadHarnessRecentTerminalSummaries(context.Background(), store, logPath, 5)
	lines := formatHarnessStatus(status, statusPath, logPath, harnessSessionLogSummaries(logPath), recent)
	lines = append(lines, "", "watching: press Ctrl-C to stop")

	if tail > 0 {
		runtimeLines, err := recentHarnessEventLines(status, logPath, tail)
		if err != nil {
			return nil, err
		}
		lines = append(lines, "", "recent_events:")
		lines = append(lines, runtimeLines...)
	}
	return lines, nil
}

func readHarnessStatus(statusPath string) ([]byte, map[string]any, error) {
	var lastDecodeErr error
	for attempt := 0; attempt < 8; attempt++ {
		encoded, err := os.ReadFile(statusPath)
		if err != nil {
			if os.IsNotExist(err) {
				status := emptyHarnessStatus()
				payload, marshalErr := json.Marshal(status)
				if marshalErr != nil {
					return nil, nil, marshalErr
				}
				return payload, status, nil
			}
			return nil, nil, fmt.Errorf("read harness status %s: %w", statusPath, err)
		}

		status := map[string]any{}
		if err := json.Unmarshal(encoded, &status); err == nil {
			return encoded, status, nil
		} else {
			lastDecodeErr = err
		}

		if !harnessStatusDecodeRetryable(lastDecodeErr) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	return nil, nil, fmt.Errorf("decode harness status %s: %w", statusPath, lastDecodeErr)
}

func harnessStatusDecodeRetryable(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "unexpected end of JSON input") ||
		strings.Contains(message, "unexpected EOF")
}

func emptyHarnessStatus() map[string]any {
	return map[string]any{
		"updated_at": "",
		"metadata": map[string]any{
			"agent_kind":     "",
			"tracker_kind":   "commission_source:haft",
			"config_path":    "",
			"workspace_root": defaultHarnessWorkspaceRoot(),
		},
		"orchestrator": map[string]any{
			"claimed":         []any{},
			"running":         []any{},
			"pending_human":   []any{},
			"running_details": []any{},
			"skipped":         []any{},
		},
		"failures":           []any{},
		"status_unavailable": true,
	}
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
	workspaceSummary := inspectHarnessWorkspaceGit(workspacePath)
	authorization := harnessWorkspaceApplyAuthorization(
		commission,
		workspacePath,
		"",
		workspaceSummary,
	)

	lines := []string{
		"Open-Sleigh harness result",
		"commission: " + presentOrUnknown(commissionID),
		"state: " + presentOrUnknown(stringField(commission, "state")),
		"decision: " + presentOrUnknown(stringField(commission, "decision_ref")),
		"plan: " + presentOrUnknown(stringField(commission, "implementation_plan_ref")),
		"delivery_policy: " + presentOrUnknown(stringField(commission, "delivery_policy")),
		"workspace: " + workspacePath,
	}

	lines = append(lines, formatHarnessCurrentRuntime(runtimeDetail, statusUpdatedAt, runtimeSummary)...)
	lines = append(lines, formatHarnessLatestAgentTurn(latestTurn)...)
	lines = append(lines, formatHarnessResultEvents(mapSliceField(commission, "events"))...)
	lines = append(lines, formatHarnessEvidenceSummary(commission)...)
	lines = append(lines, formatHarnessWorkspaceGitSummary(workspaceSummary)...)
	lines = append(lines, formatHarnessOperatorNext(commission, workspaceSummary, authorization)...)
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
	payload := mapField(event, "payload")
	parts := []string{
		presentOrUnknown(stringField(event, "event")),
		"action=" + presentOrUnknown(stringField(event, "action")),
		"verdict=" + presentOrUnknown(stringField(event, "verdict")),
		"reason=" + presentOrUnknown(stringField(event, "reason")),
		"at=" + presentOrUnknown(stringField(event, "recorded_at")),
	}
	if outOfScope := stringSliceField(payload, "out_of_scope_paths"); len(outOfScope) > 0 {
		parts = append(parts, "out_of_scope="+strings.Join(outOfScope, ","))
	}
	return strings.Join(parts, " ")
}

func formatHarnessEvidenceSummary(commission map[string]any) []string {
	requirements := harnessEvidenceRequirements(commission)
	events := currentHarnessAttemptEvents(mapSliceField(commission, "events"))

	lines := []string{"evidence_summary:"}
	lines = append(lines, "- required="+strconv.Itoa(len(requirements)))
	if len(requirements) == 0 {
		lines = append(lines, "- requirements: none declared")
	} else {
		visible := requirements
		if len(visible) > 5 {
			visible = visible[:5]
		}
		for _, requirement := range visible {
			lines = append(lines, "- requirement: "+formatHarnessEvidenceRequirement(requirement))
		}
		if len(requirements) > len(visible) {
			lines = append(lines, fmt.Sprintf("- requirement: +%d more", len(requirements)-len(visible)))
		}
	}

	measure := harnessLatestPhaseOutcome(events, "measure")
	if len(measure) > 0 {
		lines = append(lines, "- latest_measure: "+harnessPhaseEventSummary(measure))
	}

	terminal := harnessLatestPhaseOutcome(events, "terminal")
	if len(terminal) > 0 {
		lines = append(lines, "- terminal: "+harnessPhaseEventSummary(terminal))
	}
	return lines
}

func harnessEvidenceRequirements(commission map[string]any) []any {
	switch value := commission["evidence_requirements"].(type) {
	case []any:
		return value
	case []string:
		result := make([]any, 0, len(value))
		for _, item := range value {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func formatHarnessEvidenceRequirement(requirement any) string {
	switch value := requirement.(type) {
	case string:
		return "command=" + presentOrUnknown(value)
	case map[string]any:
		return formatHarnessEvidenceRequirementMap(value)
	default:
		return "kind=" + presentOrUnknown(fmt.Sprint(value))
	}
}

func formatHarnessEvidenceRequirementMap(requirement map[string]any) string {
	parts := []string{}
	for _, key := range []string{"kind", "command", "description", "claim_ref", "section_ref"} {
		value := strings.TrimSpace(stringField(requirement, key))
		if value == "" {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	if len(parts) == 0 {
		return "kind=unspecified"
	}
	return strings.Join(parts, " ")
}

func harnessLatestPhaseOutcome(events []map[string]any, phase string) map[string]any {
	latest := map[string]any{}
	for _, event := range events {
		if stringField(event, "event") != "phase_outcome" {
			continue
		}
		if stringField(mapField(event, "payload"), "phase") != phase {
			continue
		}
		latest = event
	}
	return latest
}

func harnessPhaseEventSummary(event map[string]any) string {
	payload := mapField(event, "payload")
	fields := []string{
		"verdict=" + presentOrUnknown(stringField(event, "verdict")),
		"next=" + presentOrUnknown(stringField(payload, "next")),
		"at=" + presentOrUnknown(stringField(event, "recorded_at")),
	}
	return strings.Join(fields, " ")
}

func inspectHarnessWorkspaceGit(workspacePath string) harnessWorkspaceGitSummary {
	if _, err := os.Stat(filepath.Join(workspacePath, ".git")); err != nil {
		return harnessWorkspaceGitSummary{
			State: harnessWorkspaceDiffUnavailable,
			Error: "workspace not found or not a git repository",
		}
	}

	tracked, trackedErr := gitChangedTrackedFiles(workspacePath)
	if trackedErr != nil {
		return harnessWorkspaceGitSummary{
			State: harnessWorkspaceDiffUnavailable,
			Error: strings.TrimSpace(trackedErr.Error()),
		}
	}

	untracked, untrackedErr := gitUntrackedFiles(workspacePath)
	if untrackedErr != nil {
		return harnessWorkspaceGitSummary{
			State: harnessWorkspaceDiffUnavailable,
			Error: strings.TrimSpace(untrackedErr.Error()),
		}
	}

	changed := uniqueStringsPreserveOrder(append(tracked, untracked...))
	state := harnessWorkspaceDiffClean
	if len(changed) > 0 {
		state = harnessWorkspaceDiffChanged
	}

	return harnessWorkspaceGitSummary{
		State:    state,
		Status:   trimmedCommandOutput(workspacePath, "git", "status", "--short"),
		DiffStat: trimmedCommandOutput(workspacePath, "git", "diff", "--stat"),
		Changed:  changed,
	}
}

func formatHarnessWorkspaceGitSummary(summary harnessWorkspaceGitSummary) []string {
	lines := []string{"diff_status: " + string(summary.State)}
	if strings.TrimSpace(summary.Error) != "" {
		lines = append(lines, "diff_error: "+summary.Error)
	}

	lines = append(lines, "git_status:")
	if strings.TrimSpace(summary.Status) == "" {
		lines = append(lines, "- clean")
	} else {
		lines = append(lines, indentLines(summary.Status)...)
	}

	lines = append(lines, "diff_stat:")
	if strings.TrimSpace(summary.DiffStat) == "" {
		lines = append(lines, "- empty")
	} else {
		lines = append(lines, indentLines(summary.DiffStat)...)
	}
	return lines
}

func formatHarnessOperatorNext(
	commission map[string]any,
	workspaceSummary harnessWorkspaceGitSummary,
	authorization scopeauth.Summary,
) []string {
	action := harnessOperatorActionFor(commission, workspaceSummary, authorization)
	lines := []string{"operator_next:", "- next_action=" + string(action.Kind)}
	if strings.TrimSpace(action.Reason) != "" {
		lines = append(lines, "- reason="+action.Reason)
	}
	if action.ApplyDisabledReason.Code != "" {
		lines = append(lines, "- apply_disabled_reason="+formatHarnessApplyDisabledReason(action.ApplyDisabledReason))
	}
	lines = append(lines, action.Lines...)
	return lines
}

func harnessOperatorActionFor(
	commission map[string]any,
	workspaceSummary harnessWorkspaceGitSummary,
	authorization scopeauth.Summary,
) harnessOperatorAction {
	state := stringField(commission, "state")
	commissionID := stringField(commission, "id")

	switch {
	case workcommission.IsRunnableState(state) || state == string(workcommission.StateDraft):
		return harnessQueuedOperatorAction(commissionID)
	case workcommission.IsExecutingState(state):
		return harnessRunningOperatorAction(commissionID)
	case workcommission.IsCompletionState(state):
		return harnessCompletedOperatorAction(commission, workspaceSummary, authorization)
	case workcommission.RequiresOperatorDecisionState(state):
		return harnessBlockedOperatorAction(commissionID, state)
	case workcommission.IsTerminalState(state):
		return harnessInspectOperatorAction(commissionID, "commission is "+state)
	default:
		return harnessInspectOperatorAction(commissionID, "commission state is "+presentOrUnknown(state))
	}
}

func harnessQueuedOperatorAction(commissionID string) harnessOperatorAction {
	return harnessOperatorAction{
		Kind:   harnessOperatorActionWait,
		Reason: "commission is runnable but not active",
		Lines: []string{
			"- start or continue runtime: haft harness run",
			"- inspect queued commission: haft commission show " + commissionID,
		},
	}
}

func harnessRunningOperatorAction(commissionID string) harnessOperatorAction {
	return harnessOperatorAction{
		Kind:   harnessOperatorActionWait,
		Reason: "runtime is still active",
		Lines: []string{
			"- wait for terminal state: haft harness tail " + commissionID + " --follow",
			"- inspect current state: haft harness result " + commissionID,
		},
	}
}

func harnessCompletedOperatorAction(
	commission map[string]any,
	workspaceSummary harnessWorkspaceGitSummary,
	authorization scopeauth.Summary,
) harnessOperatorAction {
	commissionID := stringField(commission, "id")
	if workspaceSummary.State == harnessWorkspaceDiffChanged {
		if !authorization.CanApply() {
			reason := authorization.BlockingReason()
			message := harnessScopeAuthorizationMessage(authorization)
			return harnessOperatorAction{
				Kind:                harnessOperatorActionInspect,
				Reason:              message,
				ApplyDisabledReason: reason,
				Lines: []string{
					"- inspect commission scope: haft commission show " + commissionID,
					"- inspect current state: haft harness result " + commissionID,
				},
			}
		}
		lines := []string{
			"- apply completed workspace diff: haft harness apply " + commissionID,
			"- then rerun required evidence in the project checkout",
		}
		lines = append(lines, harnessEvidenceCommandLines(commission)...)
		return harnessOperatorAction{
			Kind:   harnessOperatorActionApply,
			Reason: "completed workspace has unapplied changes",
			Lines:  lines,
		}
	}
	if workspaceSummary.State == harnessWorkspaceDiffClean {
		lines := []string{"- rerun required evidence in the project checkout before relying on the result"}
		lines = append(lines, harnessEvidenceCommandLines(commission)...)
		return harnessOperatorAction{
			Kind:   harnessOperatorActionRerunEvidence,
			Reason: "completed workspace has no unapplied diff",
			Lines:  lines,
		}
	}
	return harnessInspectOperatorAction(commissionID, "completed workspace is unavailable; inspect before applying")
}

func canApplyHarnessWorkspaceDiff(
	commission map[string]any,
	workspacePath string,
	projectRoot string,
	workspaceSummary harnessWorkspaceGitSummary,
) bool {
	authorization := harnessWorkspaceApplyAuthorization(
		commission,
		workspacePath,
		projectRoot,
		workspaceSummary,
	)

	return canApplyAuthorizedHarnessWorkspaceDiff(commission, workspaceSummary, authorization)
}

func harnessWorkspaceApplyAuthorization(
	commission map[string]any,
	workspacePath string,
	projectRoot string,
	workspaceSummary harnessWorkspaceGitSummary,
) scopeauth.Summary {
	if workspaceSummary.State != harnessWorkspaceDiffChanged {
		return scopeauth.Summary{}
	}

	return scopeauth.AuthorizeWorkspaceDiff(
		harnessCommissionScope(commission),
		workspaceSummary.Changed,
		scopeauth.PathFacts{WorkspaceRoot: workspacePath, ProjectRoot: projectRoot},
	)
}

func canApplyAuthorizedHarnessWorkspaceDiff(
	commission map[string]any,
	workspaceSummary harnessWorkspaceGitSummary,
	authorization scopeauth.Summary,
) bool {
	if !isHarnessApplyResultState(stringField(commission, "state")) {
		return false
	}
	if workspaceSummary.State != harnessWorkspaceDiffChanged {
		return false
	}

	return authorization.CanApply()
}

func isHarnessApplyResultState(state string) bool {
	return workcommission.IsCompletionState(state)
}

func harnessBlockedOperatorAction(commissionID string, state string) harnessOperatorAction {
	return harnessOperatorAction{
		Kind:   harnessOperatorActionInspect,
		Reason: "commission requires operator decision: " + state,
		Lines: []string{
			"- inspect result and phase events: haft harness result " + commissionID,
			"- requeue after fixing the cause: haft commission requeue " + commissionID + " --reason operator_recovered",
			"- cancel if obsolete: haft commission cancel " + commissionID + " --reason obsolete",
		},
	}
}

func harnessInspectOperatorAction(commissionID string, reason string) harnessOperatorAction {
	return harnessOperatorAction{
		Kind:   harnessOperatorActionInspect,
		Reason: reason,
		Lines: []string{
			"- inspect result and phase events: haft harness result " + commissionID,
		},
	}
}

func harnessEvidenceCommandLines(commission map[string]any) []string {
	lines := []string{}
	for _, requirement := range harnessEvidenceRequirements(commission) {
		command := harnessEvidenceRequirementCommand(requirement)
		if command == "" {
			continue
		}
		lines = append(lines, "- evidence: "+command)
	}
	return lines
}

func harnessEvidenceRequirementCommand(requirement any) string {
	switch value := requirement.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		return strings.TrimSpace(stringField(value, "command"))
	default:
		return ""
	}
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

func applyHarnessWorkspaceDiff(
	projectRoot string,
	workspacePath string,
	scope scopeauth.CommissionScope,
) (harnessApplySummary, error) {
	summary := harnessApplySummary{
		CommissionID: filepath.Base(workspacePath),
		Workspace:    workspacePath,
		ProjectRoot:  projectRoot,
	}

	if _, err := os.Stat(filepath.Join(workspacePath, ".git")); err != nil {
		return summary, fmt.Errorf("workspace is not a git repository: %s", workspacePath)
	}

	tracked, err := gitChangedTrackedFiles(workspacePath)
	if err != nil {
		return summary, err
	}
	untracked, err := gitUntrackedFiles(workspacePath)
	if err != nil {
		return summary, err
	}
	changed := uniqueStringsPreserveOrder(append(tracked, untracked...))
	if len(changed) == 0 {
		return summary, fmt.Errorf("workspace has no diff to apply")
	}
	authorization := scopeauth.AuthorizeWorkspaceDiff(
		scope,
		changed,
		scopeauth.PathFacts{WorkspaceRoot: workspacePath, ProjectRoot: projectRoot},
	)
	if !authorization.CanApply() {
		return summary, harnessScopeAuthorizationError(authorization)
	}
	if dirty, err := gitDirtyTargetPaths(projectRoot, changed); err != nil {
		return summary, err
	} else if len(dirty) > 0 {
		return summary, fmt.Errorf("target checkout has existing changes in scoped paths: %s", strings.Join(dirty, ", "))
	}

	if len(tracked) > 0 {
		patch, err := gitDiffBinary(workspacePath, tracked)
		if err != nil {
			return summary, err
		}
		if len(bytes.TrimSpace(patch)) == 0 {
			return summary, fmt.Errorf("workspace tracked diff is empty")
		}
		if err := gitApplyPatch(projectRoot, patch); err != nil {
			return summary, err
		}
	}
	if len(untracked) > 0 {
		if err := copyHarnessWorkspaceFiles(projectRoot, workspacePath, untracked); err != nil {
			return summary, err
		}
	}

	summary.Files = changed
	return summary, nil
}

func formatHarnessApplySummary(summary harnessApplySummary) []string {
	lines := []string{
		"Applied harness workspace diff",
		"commission: " + presentOrUnknown(summary.CommissionID),
		"workspace: " + presentOrUnknown(summary.Workspace),
		"project: " + presentOrUnknown(summary.ProjectRoot),
		"files:",
	}
	for _, file := range summary.Files {
		lines = append(lines, "- "+file)
	}
	if strings.TrimSpace(summary.Commit) != "" {
		lines = append(lines, "commit: "+summary.Commit)
		lines = append(lines, "next: review with git show "+summary.Commit+", then run required evidence")
		return lines
	}
	lines = append(lines, "next: review with git diff, then run required evidence")
	return lines
}

func harnessCommissionScope(commission map[string]any) scopeauth.CommissionScope {
	scope := mapField(commission, "scope")
	return scopeauth.CommissionScope{
		AllowedPaths:   uniqueStringsPreserveOrder(stringSliceField(scope, "allowed_paths")),
		ForbiddenPaths: uniqueStringsPreserveOrder(stringSliceField(scope, "forbidden_paths")),
		AffectedFiles:  uniqueStringsPreserveOrder(stringSliceField(scope, "affected_files")),
		Lockset: uniqueStringsPreserveOrder(
			append(
				stringSliceField(scope, "lockset"),
				stringSliceField(commission, "lockset")...,
			),
		),
	}
}

func harnessScopeAuthorizationError(summary scopeauth.Summary) error {
	return fmt.Errorf("%s", harnessScopeAuthorizationMessage(summary))
}

func harnessScopeAuthorizationMessage(summary scopeauth.Summary) string {
	switch summary.Verdict {
	case scopeauth.Forbidden:
		return fmt.Sprintf(
			"workspace diff contains paths forbidden by commission scope: %s",
			strings.Join(summary.Forbidden, ", "),
		)
	case scopeauth.UnknownScope:
		return fmt.Sprintf(
			"workspace diff cannot be applied because commission scope is unknown for paths: %s",
			strings.Join(summary.UnknownScope, ", "),
		)
	case scopeauth.OutOfScope:
		return fmt.Sprintf(
			"workspace diff contains paths outside commission scope: %s",
			strings.Join(summary.OutOfScope, ", "),
		)
	default:
		return "workspace diff is not authorized by commission scope"
	}
}

func formatHarnessApplyDisabledReason(reason scopeauth.BlockingReason) string {
	parts := []string{
		"code=" + string(reason.Code),
		"verdict=" + string(reason.Verdict),
	}
	if len(reason.Paths) > 0 {
		parts = append(parts, "paths="+strings.Join(reason.Paths, ","))
	}
	return strings.Join(parts, " ")
}

func harnessScopeAuthorizationPayload(summary scopeauth.Summary) map[string]any {
	if summary.Verdict == "" {
		return nil
	}

	return map[string]any{
		"verdict":             string(summary.Verdict),
		"can_apply":           summary.CanApply(),
		"allowed_paths":       summary.Allowed,
		"out_of_scope_paths":  summary.OutOfScope,
		"forbidden_paths":     summary.Forbidden,
		"unknown_scope_paths": summary.UnknownScope,
		"operator_reason":     harnessBlockingReasonPayload(summary.BlockingReason(), summary),
	}
}

func harnessBlockingReasonPayload(
	reason scopeauth.BlockingReason,
	summary scopeauth.Summary,
) map[string]any {
	if reason.Code == "" {
		return nil
	}

	return map[string]any{
		"code":    string(reason.Code),
		"verdict": string(reason.Verdict),
		"paths":   reason.Paths,
		"message": harnessScopeAuthorizationMessage(summary),
	}
}

func gitChangedTrackedFiles(workdir string) ([]string, error) {
	output, err := harnessGitOutput(workdir, "diff", "--name-only")
	if err != nil {
		return nil, err
	}
	return uniqueStringsPreserveOrder(cleanStringSlice(strings.Split(string(output), "\n"))), nil
}

func gitUntrackedFiles(workdir string) ([]string, error) {
	output, err := harnessGitOutput(workdir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return uniqueStringsPreserveOrder(cleanStringSlice(strings.Split(string(output), "\n"))), nil
}

func gitDirtyTargetPaths(workdir string, paths []string) ([]string, error) {
	args := append([]string{"status", "--porcelain", "--"}, paths...)
	output, err := harnessGitOutput(workdir, args...)
	if err != nil {
		return nil, err
	}

	dirty := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > 3 {
			dirty = append(dirty, strings.TrimSpace(line[3:]))
		}
	}
	return uniqueStringsPreserveOrder(dirty), nil
}

func gitDiffBinary(workdir string, paths []string) ([]byte, error) {
	args := append([]string{"diff", "--binary", "--"}, paths...)
	return harnessGitOutput(workdir, args...)
}

func copyHarnessWorkspaceFiles(projectRoot string, workspacePath string, paths []string) error {
	for _, path := range paths {
		if err := copyHarnessWorkspaceFile(projectRoot, workspacePath, path); err != nil {
			return err
		}
	}
	return nil
}

func copyHarnessWorkspaceFile(projectRoot string, workspacePath string, path string) error {
	source := filepath.Join(workspacePath, path)
	target := filepath.Join(projectRoot, path)

	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("stat workspace file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("workspace path is a directory, not a file: %s", path)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("workspace symlink copy is not supported: %s", path)
	}

	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read workspace file %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create target directory for %s: %w", path, err)
	}
	if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write target file %s: %w", path, err)
	}
	return nil
}

func gitApplyPatch(workdir string, patch []byte) error {
	cmd := exec.Command("git", "-C", workdir, "apply", "--whitespace=nowarn", "-")
	cmd.Stdin = bytes.NewReader(patch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply workspace diff: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func harnessGitOutput(workdir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", workdir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return output, nil
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
	return selectedHarnessStatusPathValue(harnessStatusPath)
}

func selectedHarnessStatusPathValue(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	if env := strings.TrimSpace(os.Getenv("OPEN_SLEIGH_STATUS_PATH")); env != "" {
		return env
	}
	return defaultHarnessStatusPath()
}

func selectedHarnessLogPath() string {
	return selectedHarnessLogPathValue(harnessStatusLogPath)
}

func selectedHarnessLogPathValue(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
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
	recentTerminal []harnessTerminalCommissionSummary,
) []string {
	metadata := mapField(status, "metadata")
	orchestrator := mapField(status, "orchestrator")
	claimed := stringSliceField(orchestrator, "claimed")
	running := stringSliceField(orchestrator, "running")
	pendingHuman := stringSliceField(orchestrator, "pending_human")
	skipped := mapSliceField(orchestrator, "skipped")
	failures := mapSliceField(status, "failures")

	lines := []string{
		"Open-Sleigh harness status",
		"updated_at: " + stringField(status, "updated_at"),
		"runtime_state: " + harnessRuntimeState(status, claimed, running, pendingHuman, time.Now().UTC()),
		"status_path: " + statusPath,
		"runtime_log: " + logPath,
		"agent: " + presentOrUnknown(stringField(metadata, "agent_kind")),
		"tracker: " + presentOrUnknown(stringField(metadata, "tracker_kind")),
		"config: " + presentOrUnknown(stringField(metadata, "config_path")),
		"workspace_root: " + presentOrUnknown(stringField(metadata, "workspace_root")),
		"claimed: " + strconv.Itoa(len(claimed)),
		"running: " + strconv.Itoa(len(running)),
		"pending_human: " + strconv.Itoa(len(pendingHuman)),
		"skipped: " + strconv.Itoa(len(skipped)),
		"failures: " + strconv.Itoa(len(failures)),
	}

	lines = append(
		lines,
		formatRunningDetails(mapSliceField(orchestrator, "running_details"), sessionSummaries, time.Now().UTC())...,
	)
	lines = append(lines, formatSkippedHarnessCommissions(skipped)...)
	lines = append(lines, formatFailures(failures)...)
	lines = append(lines, formatRecentTerminalCommissions(recentTerminal)...)
	lines = append(lines, formatHarnessOperatorHints(claimed, running, pendingHuman, skipped, recentTerminal)...)
	return lines
}

func harnessRuntimeState(
	status map[string]any,
	claimed []string,
	running []string,
	pendingHuman []string,
	now time.Time,
) string {
	if len(claimed)+len(running)+len(pendingHuman) > 0 {
		return "active"
	}
	if boolField(status, "status_unavailable") {
		return "unavailable"
	}

	updatedAt := parseHarnessTimestamp(stringField(status, "updated_at"))
	if updatedAt.IsZero() {
		return "unknown"
	}
	if now.Sub(updatedAt) > 2*time.Minute {
		return "stale"
	}
	return "idle"
}

func boolField(payload map[string]any, key string) bool {
	value, ok := payload[key].(bool)
	return ok && value
}

func intField(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, err := strconv.Atoi(value.String())
		if err == nil {
			return parsed
		}
	}
	return 0
}

func formatHarnessOperatorHints(
	claimed []string,
	running []string,
	pendingHuman []string,
	skipped []map[string]any,
	recent []harnessTerminalCommissionSummary,
) []string {
	if len(claimed)+len(running)+len(pendingHuman) > 0 {
		return []string{
			"operator_next:",
			"- watch live: haft harness watch",
			"- inspect current run: haft harness status <commission-id>",
			"- tail current run: haft harness tail <commission-id> --follow",
		}
	}

	if len(skipped) > 0 {
		commissionID := presentOrUnknown(stringField(skipped[0], "commission_id"))
		return []string{
			"operator_next:",
			"- inspect skipped lease: haft harness result " + commissionID,
			"- requeue after verifying it is still wanted: haft commission requeue " + commissionID + " --reason stale_lease_recovered",
			"- cancel if obsolete: haft commission cancel " + commissionID + " --reason stale_lease_obsolete",
		}
	}

	if len(recent) > 0 {
		latest := recent[0].CommissionID
		return []string{
			"operator_next:",
			"- inspect latest result: haft harness result " + latest,
			"- tail latest run: haft harness tail " + latest,
			"- run queued commissions: haft harness run",
		}
	}

	return []string{
		"operator_next:",
		"- no active harness run detected",
		"- create a commission: haft commission create-from-decision <decision-id>",
		"- create a plan and commissions: haft harness run <decision-id> --prepare-only",
		"- run queued commissions: haft harness run",
	}
}

func formatSkippedHarnessCommissions(skipped []map[string]any) []string {
	if len(skipped) == 0 {
		return nil
	}

	lines := []string{"skipped_leases:"}
	for _, item := range skipped {
		fields := []string{
			"commission=" + presentOrUnknown(stringField(item, "commission_id")),
			"reason=" + presentOrUnknown(stringField(item, "reason")),
		}
		if state := strings.TrimSpace(stringField(item, "state")); state != "" {
			fields = append(fields, "state="+state)
		}
		if fetchedAt := strings.TrimSpace(stringField(item, "fetched_at")); fetchedAt != "" {
			fields = append(fields, "fetched_at="+fetchedAt)
		}
		if age := strings.TrimSpace(stringField(item, "age")); age != "" {
			fields = append(fields, "age="+age)
		}
		if maxAgeS := intField(item, "max_age_s"); maxAgeS > 0 {
			fields = append(fields, "max_age_s="+strconv.Itoa(maxAgeS))
		}
		lines = append(lines, "- "+strings.Join(fields, " "))
	}
	return lines
}

func formatRecentTerminalCommissions(recent []harnessTerminalCommissionSummary) []string {
	if len(recent) == 0 {
		return nil
	}

	lines := []string{"recent_terminal:"}
	for _, summary := range recent {
		fields := []string{
			"commission=" + presentOrUnknown(summary.CommissionID),
			"state=" + presentOrUnknown(summary.State),
			"decision=" + presentOrUnknown(summary.DecisionRef),
		}
		if strings.TrimSpace(summary.PlanRef) != "" {
			fields = append(fields, "plan="+summary.PlanRef)
		}
		if strings.TrimSpace(summary.LastEvent) != "" {
			fields = append(fields, "last_event="+summary.LastEvent)
		}
		if strings.TrimSpace(summary.LastVerdict) != "" {
			fields = append(fields, "verdict="+summary.LastVerdict)
		}
		if strings.TrimSpace(summary.RecordedAt) != "" {
			fields = append(fields, "at="+summary.RecordedAt)
		}

		lines = append(lines, "- "+strings.Join(fields, " "))
		lines = append(lines, "  result=haft harness result "+summary.CommissionID)
		lines = append(lines, "  tail=haft harness tail "+summary.CommissionID)
		lines = append(lines, "  terminal_next="+harnessRecentTerminalNext(summary))
		if strings.TrimSpace(summary.Workspace) != "" {
			lines = append(lines, "  workspace="+summary.Workspace)
		}
		if strings.TrimSpace(summary.Preview) != "" {
			lines = append(lines, "  preview="+summary.Preview)
		}
	}
	return lines
}

func harnessRecentTerminalNext(summary harnessTerminalCommissionSummary) string {
	state := strings.TrimSpace(summary.State)

	switch {
	case workcommission.IsCompletionState(state):
		return "inspect result, then apply or rerun evidence"
	case workcommission.RequiresOperatorDecisionState(state):
		return "inspect result, then requeue or cancel"
	case workcommission.IsTerminalState(state):
		return "inspect audit trail"
	default:
		return "inspect result"
	}
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

func openOptionalHarnessStore() (*artifact.Store, func()) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, func() {}
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil || projCfg == nil {
		return nil, func() {}
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return nil, func() {}
	}

	database, err := db.NewStore(dbPath)
	if err != nil {
		return nil, func() {}
	}

	return artifact.NewStore(database.GetRawDB()), func() {
		_ = database.Close()
	}
}

func loadHarnessRecentTerminalSummaries(
	ctx context.Context,
	store *artifact.Store,
	logPath string,
	limit int,
) []harnessTerminalCommissionSummary {
	if store == nil || limit <= 0 {
		return nil
	}

	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return nil
	}

	summaries := make([]harnessTerminalCommissionSummary, 0, len(records))
	for _, commission := range records {
		state := strings.TrimSpace(stringField(commission, "state"))
		if !isHarnessTerminalState(state) {
			continue
		}

		events := currentHarnessAttemptEvents(mapSliceField(commission, "events"))
		lastEvent := map[string]any{}
		if len(events) > 0 {
			lastEvent = events[len(events)-1]
		}

		commissionID := stringField(commission, "id")
		latestTurn := harnessLatestCommissionLogSummary(logPath, commissionID)
		recordedAt := strings.TrimSpace(stringField(lastEvent, "recorded_at"))
		if recordedAt == "" {
			recordedAt = latestTurn.At
		}

		summaries = append(summaries, harnessTerminalCommissionSummary{
			CommissionID: commissionID,
			State:        state,
			DecisionRef:  stringField(commission, "decision_ref"),
			PlanRef:      stringField(commission, "implementation_plan_ref"),
			LastEvent:    stringField(lastEvent, "event"),
			LastVerdict:  stringField(lastEvent, "verdict"),
			RecordedAt:   recordedAt,
			Workspace:    filepath.Join(defaultHarnessWorkspaceRoot(), commissionID),
			Preview:      latestTurn.TextPreview,
		})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		left := parseHarnessTimestamp(summaries[i].RecordedAt)
		right := parseHarnessTimestamp(summaries[j].RecordedAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		return summaries[i].CommissionID > summaries[j].CommissionID
	})

	if len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries
}

func isHarnessTerminalState(state string) bool {
	return harnessStateStopsOperatorWait(state)
}

func isHarnessOperatorStopState(state string) bool {
	return harnessStateStopsOperatorWait(state)
}

func harnessStateStopsOperatorWait(state string) bool {
	cleanState := strings.TrimSpace(state)

	return workcommission.IsCompletionState(cleanState) ||
		workcommission.RequiresOperatorDecisionState(cleanState) ||
		workcommission.IsTerminalState(cleanState)
}

func parseHarnessTimestamp(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	for _, format := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func printHarnessRuntimeTail(
	cmd *cobra.Command,
	status map[string]any,
	logPath string,
	lineCount int,
) error {
	lines, err := recentHarnessEventLines(status, logPath, lineCount)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "recent_events:"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func recentHarnessEventLines(status map[string]any, logPath string, lineCount int) ([]string, error) {
	lines, err := recentHarnessLogLines(status, logPath, lineCount)
	if err != nil {
		return nil, err
	}

	formatted := make([]string, 0, len(lines))
	for _, line := range lines {
		eventLine, ok := formatHarnessRuntimeLogLine(line)
		if !ok {
			continue
		}
		formatted = append(formatted, eventLine)
	}
	if len(formatted) == 0 {
		return []string{"- no operator runtime events yet"}, nil
	}
	return formatted, nil
}

func recentHarnessLogLines(status map[string]any, logPath string, lineCount int) ([]string, error) {
	lines, err := readHarnessRuntimeLogLines(logPath)
	if err != nil {
		return nil, err
	}

	filtered := filterHarnessLogLines(status, lines)
	if len(filtered) == 0 {
		filtered = lines
	}
	if len(filtered) <= lineCount {
		return filtered, nil
	}
	return filtered[len(filtered)-lineCount:], nil
}

func printHarnessTailSnapshot(
	cmd *cobra.Command,
	logPath string,
	commissionID string,
	rawJSON bool,
) (int, error) {
	offset, printed, err := printHarnessTailSince(cmd, logPath, commissionID, 0, rawJSON)
	if err != nil {
		return offset, err
	}
	if printed == 0 && !rawJSON {
		_, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"No runtime events for commission %s yet. Use `haft harness tail %s --follow` to wait.\n",
			commissionID,
			commissionID,
		)
		return offset, err
	}
	return offset, nil
}

func printHarnessTailSince(
	cmd *cobra.Command,
	logPath string,
	commissionID string,
	offset int,
	rawJSON bool,
) (int, int, error) {
	lines, err := readHarnessRuntimeLogLines(logPath)
	if err != nil {
		return offset, 0, err
	}
	if offset > len(lines) {
		offset = 0
	}

	printed := 0
	for _, line := range lines[offset:] {
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if strings.TrimSpace(stringField(event, "commission_id")) != commissionID {
			continue
		}

		output := line
		if !rawJSON {
			formatted, ok := formatHarnessRuntimeEventLineForOperator(event)
			if !ok {
				continue
			}
			output = formatted
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), output); err != nil {
			return len(lines), printed, err
		}
		printed++
	}
	return len(lines), printed, nil
}

func harnessTailFollowCompletionLine(
	ctx context.Context,
	store *artifact.Store,
	commissionID string,
) (string, bool, error) {
	if store == nil {
		return "", false, nil
	}

	commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
	if err != nil {
		return "", false, err
	}
	state := stringField(commission, "state")
	if !isHarnessOperatorStopState(state) {
		return "", false, nil
	}

	workspacePath := filepath.Join(defaultHarnessWorkspaceRoot(), commissionID)
	workspaceSummary := inspectHarnessWorkspaceGit(workspacePath)
	authorization := harnessWorkspaceApplyAuthorization(
		commission,
		workspacePath,
		"",
		workspaceSummary,
	)
	action := harnessOperatorActionFor(commission, workspaceSummary, authorization)
	line := strings.Join([]string{
		"terminal:",
		"commission=" + presentOrUnknown(commissionID),
		"state=" + presentOrUnknown(state),
		"next_action=" + string(action.Kind),
		"result=haft harness result " + commissionID,
	}, " ")
	return line, true, nil
}

func readHarnessRuntimeLogLines(logPath string) ([]string, error) {
	encoded, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read harness runtime log %s: %w", logPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(encoded)), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}
	return result, nil
}

func formatHarnessRuntimeLogLine(line string) (string, bool) {
	event := map[string]any{}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", false
	}
	return formatHarnessRuntimeEventLineForOperator(event)
}

func formatHarnessRuntimeEventLine(event map[string]any) string {
	line, ok := formatHarnessRuntimeEventLineForOperator(event)
	if ok {
		return line
	}

	data := mapField(event, "data")
	fields := []string{
		presentOrUnknown(stringField(event, "at")),
		presentOrUnknown(stringField(event, "event")),
	}

	if phase := strings.TrimSpace(stringField(data, "phase")); phase != "" {
		fields = append(fields, "phase="+phase)
	}
	if status := strings.TrimSpace(stringField(data, "status")); status != "" {
		fields = append(fields, "status="+status)
	}
	if sessionID := strings.TrimSpace(stringField(data, "session_id")); sessionID != "" {
		fields = append(fields, "session="+sessionID)
	}
	if turnID := strings.TrimSpace(stringField(data, "turn_id")); turnID != "" {
		fields = append(fields, "turn="+turnID)
	}

	if preview := harnessRuntimeEventPreview(event); preview != "" {
		fields = append(fields, "preview="+preview)
	}
	return strings.Join(fields, " ")
}

func formatHarnessRuntimeEventLineForOperator(event map[string]any) (string, bool) {
	action, ok := harnessRuntimeEventAction(stringField(event, "event"))
	if !ok {
		return "", false
	}

	data := mapField(event, "data")
	fields := []string{
		presentOrUnknown(stringField(event, "at")),
		action + ":",
	}
	if commissionID := strings.TrimSpace(stringField(event, "commission_id")); commissionID != "" {
		fields = append(fields, "commission="+commissionID)
	}
	if phase := strings.TrimSpace(stringField(data, "phase")); phase != "" {
		fields = append(fields, "phase="+phase)
	}
	if status := strings.TrimSpace(stringField(data, "status")); status != "" {
		fields = append(fields, "status="+status)
	}
	if sessionID := strings.TrimSpace(stringField(data, "session_id")); sessionID != "" {
		fields = append(fields, "session="+sessionID)
	}
	if turnID := strings.TrimSpace(stringField(data, "turn_id")); turnID != "" {
		fields = append(fields, "turn="+turnID)
	}
	if workspace := strings.TrimSpace(stringField(data, "workspace_path")); workspace != "" {
		fields = append(fields, "workspace="+workspace)
	}
	if reason := harnessCompactPreview(stringField(data, "reason")); reason != "" {
		fields = append(fields, "reason="+reason)
	}
	if preview := harnessRuntimeEventPreview(event); preview != "" {
		fields = append(fields, "preview="+preview)
	}
	return strings.Join(fields, " "), true
}

func harnessRuntimeEventAction(eventName string) (string, bool) {
	switch eventName {
	case "session_dispatched":
		return "dispatched", true
	case "session_started":
		return "started", true
	case "workspace_reset_started":
		return "workspace_reset_started", true
	case "workspace_reset_completed":
		return "workspace_reset_completed", true
	case "workspace_reset_failed":
		return "workspace_reset_failed", true
	case "agent_turn_started":
		return "agent_started", true
	case "agent_turn_completed":
		return "agent_completed", true
	case "agent_turn_failed":
		return "agent_failed", true
	case "gate_evaluation_completed":
		return "gate_checked", true
	case "gate_evaluation_failed":
		return "gate_failed", true
	case "terminal_diff_validation_failed":
		return "diff_blocked", true
	case "haft_write_completed":
		return "recorded", true
	case "haft_write_failed":
		return "record_failed", true
	case "session_failed":
		return "failed", true
	case "session_waiting_human":
		return "waiting_human", true
	case "session_aborted":
		return "aborted", true
	case "phase_blocked":
		return "blocked", true
	case "workflow_terminal":
		return "terminal", true
	default:
		return "", false
	}
}

func harnessRuntimeEventPreview(event map[string]any) string {
	data := mapField(event, "data")
	for _, key := range []string{"text_preview", "reason"} {
		preview := harnessCompactPreview(stringField(data, key))
		if preview != "" {
			return preview
		}
	}
	return ""
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

func inspectHarnessRunReadiness() (string, project.ReadinessFacts, error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", project.ReadinessFacts{}, cwdErr
		}

		facts, readinessErr := project.InspectReadiness(cwd)
		if readinessErr != nil {
			return "", project.ReadinessFacts{}, readinessErr
		}
		return cwd, facts, nil
	}

	facts, err := project.InspectReadiness(projectRoot)
	if err != nil {
		return "", project.ReadinessFacts{}, err
	}
	return projectRoot, facts, nil
}

func harnessRunReadinessGateFor(
	facts project.ReadinessFacts,
	tacticalReason string,
) harnessRunReadinessGate {
	reason := harnessRunReadinessBlockReason(facts)
	if reason == "" {
		return harnessRunReadinessGate{
			Kind:          harnessRunReadinessAdmissible,
			ProjectStatus: facts.Status,
		}
	}

	cleanReason := strings.TrimSpace(tacticalReason)
	if facts.Status == project.ReadinessNeedsOnboard && cleanReason != "" {
		return harnessRunReadinessGate{
			Kind:           harnessRunReadinessTacticalAllowed,
			ProjectStatus:  facts.Status,
			BlockReason:    reason,
			OverrideReason: cleanReason,
		}
	}

	return harnessRunReadinessGate{
		Kind:          harnessRunReadinessBlocked,
		ProjectStatus: facts.Status,
		BlockReason:   reason,
	}
}

func harnessRunReadinessBlockReason(facts project.ReadinessFacts) string {
	switch facts.Status {
	case project.ReadinessReady:
		return ""
	case project.ReadinessNeedsInit:
		return "haft harness run blocked: project readiness is needs_init; run `haft init` before harness execution."
	case project.ReadinessNeedsOnboard:
		return "haft harness run blocked: project readiness is needs_onboard; .haft exists but the ProjectSpecificationSet is missing or incomplete. Run onboarding and `haft spec check`, or pass `--tactical-override-reason \"...\"` to record out-of-spec tactical WorkCommissions."
	case project.ReadinessMissing:
		return "haft harness run blocked: project readiness is missing; select an existing project before harness execution."
	default:
		return "haft harness run blocked: project readiness is unknown."
	}
}

func runHarnessRun(cmd *cobra.Command, decisionRefs []string) error {
	if harnessRunPlanPath != "" && harnessDecisionSelectorsPresent(decisionRefs) {
		return fmt.Errorf("use either decision selectors or --plan, not both")
	}

	_, readiness, err := inspectHarnessRunReadiness()
	if err != nil {
		return err
	}

	readinessGate := harnessRunReadinessGateFor(readiness, harnessRunTacticalReason)
	if readinessGate.Kind == harnessRunReadinessBlocked {
		return fmt.Errorf("%s", readinessGate.BlockReason)
	}

	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		if shouldRunExistingHarnessCommissions(decisionRefs) {
			handled, err := runExistingHarnessCommissions(ctx, cmd, store, projectRoot, readinessGate)
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

		created, result, err := ensureHarnessCommissions(ctx, store, projectRoot, plan, readinessGate)
		if err != nil {
			return err
		}
		selection, err := loadHarnessRunSelectionForPlan(ctx, store, plan)
		if err != nil {
			return err
		}
		if err := recordHarnessRunTacticalOverride(ctx, store, selection, readinessGate); err != nil {
			return err
		}

		opts := defaultHarnessRunOptions(projectRoot, planPath, plan)
		if err := validateHarnessRunOptions(opts); err != nil {
			return err
		}

		configPath, err := writeHarnessRuntimeConfig(projectRoot, opts)
		if err != nil {
			return err
		}

		if err := printHarnessRunSummary(cmd, planPath, configPath, created, result, selection, opts); err != nil {
			return err
		}
		if opts.PrepareOnly {
			return nil
		}

		return startOpenSleighAndDeliver(ctx, cmd, store, projectRoot, selection, opts, configPath)
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
	readinessGate harnessRunReadinessGate,
) (bool, error) {
	planPath, plan, result, selection, found, err := existingRunnableHarnessPlan(ctx, store, projectRoot)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	opts := defaultHarnessRunOptions(projectRoot, planPath, plan)
	if err := validateHarnessRunOptions(opts); err != nil {
		return false, err
	}

	configPath, err := writeHarnessRuntimeConfig(projectRoot, opts)
	if err != nil {
		return false, err
	}
	if err := recordHarnessRunTacticalOverride(ctx, store, selection, readinessGate); err != nil {
		return false, err
	}

	if err := printHarnessRunSummary(cmd, planPath, configPath, false, result, selection, opts); err != nil {
		return false, err
	}
	if opts.PrepareOnly {
		return true, nil
	}

	return true, startOpenSleighAndDeliver(ctx, cmd, store, projectRoot, selection, opts, configPath)
}

func startOpenSleighAndDeliver(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	projectRoot string,
	selection harnessRunSelection,
	opts harnessRunOptions,
	configPath string,
) error {
	if !opts.Once && !opts.Detach {
		return startOpenSleighOperatorRun(ctx, cmd, store, projectRoot, selection, opts, configPath)
	}

	if err := startOpenSleigh(cmd, opts, configPath); err != nil {
		return err
	}

	return deliverHarnessRunCommissions(ctx, cmd, store, projectRoot, selection, opts)
}

func startOpenSleighOperatorRun(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	projectRoot string,
	selection harnessRunSelection,
	opts harnessRunOptions,
	configPath string,
) error {
	output := &bytes.Buffer{}
	process, err := startOpenSleighProcess(opts, configPath, output, output, nil)
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- process.Wait()
	}()

	runCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	waitErr := watchHarnessRunUntilTerminal(runCtx, cmd, store, projectRoot, selection, opts, done, output)
	if waitErr != nil {
		stopOpenSleighProcess(process, done)
		return waitErr
	}

	stopOpenSleighProcess(process, done)
	if err := printHarnessRunResults(ctx, cmd, store, selection, opts); err != nil {
		return err
	}
	return deliverHarnessRunCommissions(ctx, cmd, store, projectRoot, selection, opts)
}

func watchHarnessRunUntilTerminal(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	projectRoot string,
	selection harnessRunSelection,
	opts harnessRunOptions,
	done <-chan error,
	processOutput *bytes.Buffer,
) error {
	if opts.Drain {
		return watchHarnessDrainUntilIdle(ctx, cmd, store, projectRoot, selection, opts, done, processOutput)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	if err := printHarnessRunOperatorHeader(cmd, selection, opts); err != nil {
		return err
	}

	offset := 0
	lastProgressAt := time.Time{}
	for {
		nextOffset, printed, err := printHarnessSelectedTailSince(
			cmd,
			opts.LogPath,
			selection.CommissionIDs,
			offset,
			false,
		)
		if err != nil {
			return err
		}
		offset = nextOffset
		if printed > 0 {
			lastProgressAt = time.Now()
		}

		terminal, err := selectedHarnessCommissionsTerminal(ctx, store, selection.CommissionIDs)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}

		if printed == 0 && time.Since(lastProgressAt) >= 30*time.Second {
			if err := printHarnessSelectedProgress(cmd, opts, selection.CommissionIDs); err != nil {
				return err
			}
			lastProgressAt = time.Now()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			nextOffset, _, printErr := printHarnessSelectedTailSince(
				cmd,
				opts.LogPath,
				selection.CommissionIDs,
				offset,
				false,
			)
			if printErr != nil {
				return printErr
			}
			offset = nextOffset

			terminal, terminalErr := selectedHarnessCommissionsTerminal(ctx, store, selection.CommissionIDs)
			if terminalErr != nil {
				return terminalErr
			}
			if terminal {
				return nil
			}
			return openSleighExitedEarlyError(err, processOutput.String())
		case <-ticker.C:
		}
	}
}

func watchHarnessDrainUntilIdle(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	projectRoot string,
	selection harnessRunSelection,
	opts harnessRunOptions,
	done <-chan error,
	processOutput *bytes.Buffer,
) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	if err := printHarnessRunOperatorHeader(cmd, selection, opts); err != nil {
		return err
	}

	offset := 0
	lastProgressAt := time.Time{}
	seenIDs := stringSet(selection.CommissionIDs)
	autoApplyAttempted := map[string]struct{}{}
	workspaceRoot := opts.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = defaultHarnessWorkspaceRoot()
	}
	for {
		monitor, err := loadHarnessDrainMonitor(ctx, store, opts, time.Now().UTC())
		if err != nil {
			return err
		}
		for _, commissionID := range monitor.ObservedIDs {
			seenIDs[commissionID] = struct{}{}
		}
		// Auto-apply hook: any commission that's left the open set with
		// auto_apply.allowed=true gets the workspace diff applied as a
		// discrete revertable commit. Best-effort — failures are logged but
		// don't abort the drain loop. Each commission is attempted at most
		// once per drain run.
		openSet := stringSet(monitor.OpenIDs)
		for _, commissionID := range monitor.ObservedIDs {
			if _, attempted := autoApplyAttempted[commissionID]; attempted {
				continue
			}
			if _, stillOpen := openSet[commissionID]; stillOpen {
				continue
			}
			commission, loadErr := loadWorkCommissionPayload(ctx, store, commissionID)
			if loadErr != nil {
				continue
			}
			eligible, _ := attemptHarnessAutoApply(cmd, commission, projectRoot, workspaceRoot)
			if eligible {
				autoApplyAttempted[commissionID] = struct{}{}
				lastProgressAt = time.Now()
			} else {
				// not eligible — never need to re-check
				autoApplyAttempted[commissionID] = struct{}{}
			}
		}
		tailIDs := sortedSetValues(seenIDs)

		nextOffset, printed, err := printHarnessSelectedTailSince(
			cmd,
			opts.LogPath,
			tailIDs,
			offset,
			false,
		)
		if err != nil {
			return err
		}
		offset = nextOffset
		if printed > 0 {
			lastProgressAt = time.Now()
		}

		if len(monitor.OpenIDs) == 0 {
			return printHarnessDrainFinished(cmd, monitor)
		}

		if printed == 0 && time.Since(lastProgressAt) >= 30*time.Second {
			if err := printHarnessSelectedProgress(cmd, opts, tailIDs); err != nil {
				return err
			}
			lastProgressAt = time.Now()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			nextOffset, _, printErr := printHarnessSelectedTailSince(
				cmd,
				opts.LogPath,
				tailIDs,
				offset,
				false,
			)
			if printErr != nil {
				return printErr
			}
			offset = nextOffset

			monitor, monitorErr := loadHarnessDrainMonitor(ctx, store, opts, time.Now().UTC())
			if monitorErr != nil {
				return monitorErr
			}
			if len(monitor.OpenIDs) == 0 {
				return printHarnessDrainFinished(cmd, monitor)
			}
			return openSleighExitedEarlyError(err, processOutput.String())
		case <-ticker.C:
		}
	}
}

func printHarnessDrainFinished(cmd *cobra.Command, monitor harnessDrainMonitor) error {
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "drain: runnable WorkCommission queue is empty"); err != nil {
		return err
	}
	for _, stale := range monitor.StaleLeases {
		line := strings.Join([]string{
			"drain_skipped:",
			"commission=" + presentOrUnknown(stale.CommissionID),
			"reason=lease_too_old",
			"state=" + presentOrUnknown(stale.State),
			"age=" + presentOrUnknown(stale.Age),
			"max_age_s=" + strconv.Itoa(stale.MaxAgeS),
		}, " ")
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func printHarnessRunOperatorHeader(
	cmd *cobra.Command,
	selection harnessRunSelection,
	opts harnessRunOptions,
) error {
	lines := []string{
		"Harness run started",
		"status: " + opts.StatusPath,
		"log: " + opts.LogPath,
		"workspace_root: " + opts.WorkspaceRoot,
	}
	if len(selection.CommissionIDs) > 0 {
		lines = append(lines, formatHarnessSelectionLine("Selected commission", selection.CommissionIDs))
		first := selection.CommissionIDs[0]
		lines = append(
			lines,
			"result: haft harness result "+first,
			"tail: haft harness tail "+first+" --follow",
			"workspace: "+filepath.Join(opts.WorkspaceRoot, first),
		)
	}
	if len(selection.DecisionRefs) > 0 {
		lines = append(lines, formatHarnessSelectionLine("Selected decision", selection.DecisionRefs))
	}
	lines = append(lines, "", "events:")

	for _, line := range lines {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func printHarnessSelectedTailSince(
	cmd *cobra.Command,
	logPath string,
	commissionIDs []string,
	offset int,
	rawJSON bool,
) (int, int, error) {
	lines, err := readHarnessRuntimeLogLines(logPath)
	if err != nil {
		return offset, 0, err
	}
	if offset > len(lines) {
		offset = 0
	}

	selected := stringSet(commissionIDs)
	printed := 0
	for _, line := range lines[offset:] {
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if _, ok := selected[strings.TrimSpace(stringField(event, "commission_id"))]; !ok {
			continue
		}

		output := line
		if !rawJSON {
			formatted, ok := formatHarnessRuntimeEventLineForOperator(event)
			if !ok {
				continue
			}
			output = formatted
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), output); err != nil {
			return len(lines), printed, err
		}
		printed++
	}
	return len(lines), printed, nil
}

func printHarnessSelectedProgress(
	cmd *cobra.Command,
	opts harnessRunOptions,
	commissionIDs []string,
) error {
	_, status, err := readHarnessStatus(opts.StatusPath)
	if err != nil {
		_, printErr := fmt.Fprintf(cmd.OutOrStdout(), "progress: status unavailable: %v\n", err)
		return printErr
	}

	details := selectedHarnessRunningDetails(status, commissionIDs)
	if len(details) == 0 {
		_, printErr := fmt.Fprintln(cmd.OutOrStdout(), "progress: waiting for selected commissions to start or finish")
		return printErr
	}

	sessionSummaries := harnessSessionLogSummaries(opts.LogPath)
	for _, detail := range details {
		line := formatHarnessRunningProgressLine(detail, sessionSummaries, time.Now().UTC())
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func selectedHarnessRunningDetails(status map[string]any, commissionIDs []string) []map[string]any {
	selected := stringSet(commissionIDs)
	details := mapSliceField(mapField(status, "orchestrator"), "running_details")
	result := make([]map[string]any, 0, len(details))
	for _, detail := range details {
		if _, ok := selected[strings.TrimSpace(stringField(detail, "commission_id"))]; ok {
			result = append(result, detail)
		}
	}
	return result
}

func formatHarnessRunningProgressLine(
	detail map[string]any,
	sessionSummaries map[string]harnessSessionLogSummary,
	now time.Time,
) string {
	sessionID := presentOrUnknown(stringField(detail, "session_id"))
	sessionSummary := sessionSummaries[sessionID]
	fields := []string{
		"progress:",
		"commission=" + presentOrUnknown(stringField(detail, "commission_id")),
		"phase=" + presentOrUnknown(stringField(detail, "phase")),
		"sub_state=" + presentOrUnknown(stringField(detail, "sub_state")),
	}
	if startedAt, elapsed := harnessSessionTiming(sessionSummary.StartedAt, now); startedAt != "" {
		fields = append(fields, "elapsed="+elapsed)
	}
	if strings.TrimSpace(sessionSummary.LastEvent) != "" {
		fields = append(fields, "last_event="+sessionSummary.LastEvent)
	}
	if strings.TrimSpace(sessionSummary.LastTurnStatus) != "" {
		fields = append(fields, "last_turn="+sessionSummary.LastTurnStatus)
	}
	if workspace := strings.TrimSpace(stringField(detail, "workspace_path")); workspace != "" {
		fields = append(fields, "workspace="+workspace)
	}
	return strings.Join(fields, " ")
}

func selectedHarnessCommissionsTerminal(
	ctx context.Context,
	store *artifact.Store,
	commissionIDs []string,
) (bool, error) {
	if len(commissionIDs) == 0 {
		return false, nil
	}

	for _, commissionID := range commissionIDs {
		commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
		if err != nil {
			return false, err
		}
		if !isHarnessOperatorStopState(stringField(commission, "state")) {
			return false, nil
		}
	}
	return true, nil
}

func openSleighExitedEarlyError(err error, output string) error {
	output = strings.TrimSpace(output)
	if err == nil {
		if output == "" {
			return fmt.Errorf("Open-Sleigh exited before selected WorkCommissions reached a terminal state")
		}
		return fmt.Errorf("Open-Sleigh exited before selected WorkCommissions reached a terminal state:\n%s", output)
	}
	if output == "" {
		return fmt.Errorf("Open-Sleigh exited before selected WorkCommissions reached a terminal state: %w", err)
	}
	return fmt.Errorf("Open-Sleigh exited before selected WorkCommissions reached a terminal state: %w\n%s", err, output)
}

func stopOpenSleighProcess(process *exec.Cmd, done <-chan error) {
	if process == nil || process.Process == nil {
		return
	}

	_ = process.Process.Signal(os.Interrupt)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = process.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
}

func printHarnessRunResults(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	selection harnessRunSelection,
	opts harnessRunOptions,
) error {
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nHarness run finished"); err != nil {
		return err
	}

	logPath := opts.LogPath
	sessionSummaries := harnessSessionLogSummaries(logPath)
	for index, commissionID := range selection.CommissionIDs {
		commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
		if err != nil {
			return err
		}
		runtimeDetail, statusUpdatedAt := currentHarnessRuntimeDetail(opts.StatusPath, commissionID)
		runtimeSummary := sessionSummaries[stringField(runtimeDetail, "session_id")]
		latestTurn := harnessLatestCommissionLogSummary(logPath, commissionID)

		if index > 0 {
			if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
				return err
			}
		}
		for _, line := range formatHarnessResult(
			commission,
			opts.WorkspaceRoot,
			runtimeDetail,
			statusUpdatedAt,
			runtimeSummary,
			latestTurn,
		) {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
				return err
			}
		}
	}
	return nil
}

func deliverHarnessRunCommissions(
	ctx context.Context,
	cmd *cobra.Command,
	store *artifact.Store,
	projectRoot string,
	selection harnessRunSelection,
	opts harnessRunOptions,
) error {
	for _, commissionID := range selection.CommissionIDs {
		commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
		if err != nil {
			return err
		}
		if !harnessCommissionAutoDeliveryAllowed(commission) {
			continue
		}

		summary, err := applyHarnessWorkspaceDiff(
			projectRoot,
			filepath.Join(opts.WorkspaceRoot, commissionID),
			harnessCommissionScope(commission),
		)
		if err != nil {
			return fmt.Errorf("auto delivery for %s: %w", commissionID, err)
		}
		commit, err := commitHarnessAutoAppliedDiff(
			projectRoot,
			commissionID,
			stringField(commission, "decision_ref"),
			summary.Files,
		)
		if err != nil {
			return fmt.Errorf("auto delivery commit for %s: %w", commissionID, err)
		}
		summary.Commit = commit

		for _, line := range formatHarnessApplySummary(summary) {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
				return err
			}
		}
	}
	return nil
}

func harnessCommissionAutoDeliveryAllowed(commission map[string]any) bool {
	if stringField(commission, "delivery_policy") != "workspace_patch_auto_on_pass" {
		return false
	}
	if stringField(commission, "state") != "completed" {
		return false
	}
	if !harnessCommissionTerminalPass(commission) {
		return false
	}
	return harnessCommissionAutonomyAllowed(commission)
}

func harnessCommissionTerminalPass(commission map[string]any) bool {
	events := currentHarnessAttemptEvents(mapSliceField(commission, "events"))
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		verdict := strings.TrimSpace(stringField(event, "verdict"))
		if stringField(event, "event") == "workflow_terminal" {
			return verdict == "pass"
		}

		payload := mapField(event, "payload")
		if stringField(payload, "next") == "terminal:pass" {
			return verdict == "pass"
		}
	}
	return false
}

func harnessCommissionAutonomyAllowed(commission map[string]any) bool {
	decision := harnessCommissionAutonomyDecision(commission)
	return strings.EqualFold(decision, "allowed")
}

func harnessCommissionAutonomyDecision(commission map[string]any) string {
	events := currentHarnessAttemptEvents(mapSliceField(commission, "events"))
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		payload := mapField(event, "payload")
		decision := firstCleanString([]string{
			stringField(payload, "autonomy_envelope_decision"),
			stringField(payload, "autonomy_decision"),
			stringField(mapField(payload, "autonomy_envelope"), "decision"),
		})
		if decision != "" {
			return decision
		}
	}

	return firstCleanString([]string{
		stringField(commission, "autonomy_envelope_decision"),
		stringField(commission, "autonomy_decision"),
		stringField(mapField(commission, "autonomy_envelope"), "decision"),
	})
}

func firstCleanString(values []string) string {
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned != "" {
			return cleaned
		}
	}
	return ""
}

func commitHarnessAutoAppliedDiff(
	projectRoot string,
	commissionID string,
	decisionRef string,
	files []string,
) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	addArgs := append([]string{"add", "--"}, files...)
	if _, err := harnessGitOutput(projectRoot, addArgs...); err != nil {
		return "", err
	}

	message := "Auto-apply WorkCommission " + commissionID
	body := "Decision: " + presentOrUnknown(decisionRef)
	if _, err := harnessGitOutput(projectRoot, "commit", "-m", message, "-m", body); err != nil {
		return "", err
	}

	output, err := harnessGitOutput(projectRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func existingRunnableHarnessPlan(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) (string, map[string]any, string, harnessRunSelection, bool, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return "", nil, "", harnessRunSelection{}, false, err
	}

	runnable := runnableHarnessCommissions(records)
	if len(runnable) == 0 {
		return "", nil, "", harnessRunSelection{}, false, nil
	}

	plan := existingRunnablePlan(runnable)
	planPath := existingRunnablePlanPath(projectRoot, plan)
	result := fmt.Sprintf("using %d existing runnable commission(s)", len(runnable))
	selection := harnessRunSelectionFromCommissions(runnable)
	return planPath, plan, result, selection, true, nil
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

func loadHarnessDrainMonitor(
	ctx context.Context,
	store *artifact.Store,
	opts harnessRunOptions,
	now time.Time,
) (harnessDrainMonitor, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return harnessDrainMonitor{}, err
	}

	return harnessDrainMonitorFromRecords(records, opts, now), nil
}

func harnessDrainMonitorFromRecords(
	records []map[string]any,
	opts harnessRunOptions,
	now time.Time,
) harnessDrainMonitor {
	monitor := harnessDrainMonitor{}

	for _, commission := range records {
		commissionID := strings.TrimSpace(stringField(commission, "id"))
		if commissionID == "" {
			continue
		}

		stale, summary := harnessStaleLeaseSummaryFor(commission, opts.StaleLeaseMaxAgeS, now)
		if stale {
			monitor.StaleLeases = append(monitor.StaleLeases, summary)
			monitor.ObservedIDs = append(monitor.ObservedIDs, commissionID)
			continue
		}

		if workCommissionRunnableForRequest(commission, records, nil, now) {
			monitor.RunnableIDs = append(monitor.RunnableIDs, commissionID)
			monitor.OpenIDs = append(monitor.OpenIDs, commissionID)
			monitor.ObservedIDs = append(monitor.ObservedIDs, commissionID)
			continue
		}

		if workcommission.IsExecutingState(stringField(commission, "state")) {
			monitor.ExecutingIDs = append(monitor.ExecutingIDs, commissionID)
			monitor.OpenIDs = append(monitor.OpenIDs, commissionID)
			monitor.ObservedIDs = append(monitor.ObservedIDs, commissionID)
		}
	}

	monitor.OpenIDs = uniqueStringsPreserveOrder(monitor.OpenIDs)
	monitor.ObservedIDs = uniqueStringsPreserveOrder(monitor.ObservedIDs)
	monitor.RunnableIDs = uniqueStringsPreserveOrder(monitor.RunnableIDs)
	monitor.ExecutingIDs = uniqueStringsPreserveOrder(monitor.ExecutingIDs)
	return monitor
}

// shouldAutoApplyCommission reports whether a terminal commission's
// delivery_decision says it should be auto-applied. Pure: examines the
// commission payload, performs no I/O.
func shouldAutoApplyCommission(commission map[string]any) bool {
	if !isHarnessApplyResultState(stringField(commission, "state")) {
		return false
	}
	autoApply := mapField(commission, "auto_apply")
	if len(autoApply) == 0 {
		return false
	}
	allowed, _ := autoApply["allowed"].(bool)
	return allowed
}

// attemptHarnessAutoApply runs the standard harness apply path for a single
// terminal commission when its delivery_decision marks auto_apply.allowed.
// Best-effort: failures emit a typed line on cmd output but do not abort the
// surrounding drain/run loop. Returns whether the commission was eligible and
// any apply error (regardless of eligibility, the drain caller should treat
// the commission as "auto-apply attempt complete" and move on).
func attemptHarnessAutoApply(
	cmd *cobra.Command,
	commission map[string]any,
	projectRoot string,
	workspaceRoot string,
) (bool, error) {
	if !shouldAutoApplyCommission(commission) {
		return false, nil
	}
	commissionID := stringField(commission, "id")
	summary, err := applyHarnessWorkspaceDiff(
		projectRoot,
		filepath.Join(workspaceRoot, commissionID),
		harnessCommissionScope(commission),
	)
	if err != nil {
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			"auto_apply_failed: commission=%s reason=%q\n",
			commissionID,
			err.Error(),
		)
		return true, err
	}
	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"auto_apply_succeeded: commission=%s files=%d\n",
		commissionID,
		len(summary.Files),
	)
	return true, nil
}

func harnessStaleLeaseSummaryFor(
	commission map[string]any,
	maxAgeS int,
	now time.Time,
) (bool, harnessStaleLeaseSummary) {
	if maxAgeS <= 0 {
		return false, harnessStaleLeaseSummary{}
	}
	state := stringField(commission, "state")
	if !workcommission.IsExecutingState(state) {
		return false, harnessStaleLeaseSummary{}
	}

	fetchedAt := parseHarnessTimestamp(stringField(commission, "fetched_at"))
	if fetchedAt.IsZero() {
		return false, harnessStaleLeaseSummary{}
	}

	age := now.Sub(fetchedAt)
	if age <= time.Duration(maxAgeS)*time.Second {
		return false, harnessStaleLeaseSummary{}
	}

	return true, harnessStaleLeaseSummary{
		CommissionID: stringField(commission, "id"),
		State:        state,
		FetchedAt:    fetchedAt.Format(time.RFC3339),
		Age:          age.Round(time.Second).String(),
		MaxAgeS:      maxAgeS,
	}
}

func loadHarnessRunSelectionForPlan(
	ctx context.Context,
	store *artifact.Store,
	plan map[string]any,
) (harnessRunSelection, error) {
	records, err := loadWorkCommissionPayloads(ctx, store)
	if err != nil {
		return harnessRunSelection{}, err
	}

	planRef := strings.TrimSpace(stringField(plan, "id"))
	planRevision := strings.TrimSpace(stringField(plan, "revision"))
	planCommissions := harnessPlanCommissions(records, planRef, planRevision)
	runnable := runnablePlanCommissions(records, planCommissions)
	if len(runnable) > 0 {
		return harnessRunSelectionFromCommissions(runnable), nil
	}
	return harnessRunSelectionFromCommissions(planCommissions), nil
}

func harnessPlanCommissions(records []map[string]any, planRef string, planRevision string) []map[string]any {
	filtered := make([]map[string]any, 0, len(records))
	for _, commission := range records {
		if stringField(commission, "implementation_plan_ref") != planRef {
			continue
		}
		if planRevision != "" && stringField(commission, "implementation_plan_revision") != planRevision {
			continue
		}
		filtered = append(filtered, commission)
	}
	return filtered
}

func runnablePlanCommissions(records []map[string]any, commissions []map[string]any) []map[string]any {
	runnable := make([]map[string]any, 0, len(commissions))
	now := time.Now().UTC()
	for _, commission := range commissions {
		if !workCommissionRunnableForRequest(commission, records, nil, now) {
			continue
		}
		runnable = append(runnable, commission)
	}
	return runnable
}

func harnessRunSelectionFromCommissions(commissions []map[string]any) harnessRunSelection {
	commissionIDs := make([]string, 0, len(commissions))
	decisionRefs := make([]string, 0, len(commissions))
	planRef := ""

	for _, commission := range commissions {
		commissionIDs = append(commissionIDs, stringField(commission, "id"))
		decisionRefs = append(decisionRefs, stringField(commission, "decision_ref"))
		if planRef == "" {
			planRef = stringField(commission, "implementation_plan_ref")
		}
	}

	commissionIDs = uniqueStringsPreserveOrder(cleanStringSlice(commissionIDs))
	decisionRefs = uniqueStringsPreserveOrder(cleanStringSlice(decisionRefs))
	return harnessRunSelection{
		CommissionIDs: commissionIDs,
		DecisionRefs:  decisionRefs,
		PlanRef:       strings.TrimSpace(planRef),
	}
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

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		set[cleaned] = struct{}{}
	}
	return set
}

func sortedSetValues(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
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
		DeliveryPolicy:   stringField(base, "delivery_policy"),
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

	if err := validateHarnessPlanFile(plan); err != nil {
		return harnessPlanFile{}, err
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

func validateHarnessPlanFile(plan harnessPlanFile) error {
	payload := map[string]any{
		"id":        plan.ID,
		"revision":  plan.Revision,
		"decisions": harnessPlanDecisionPayloads(plan.Decisions),
	}
	_, err := implementationplan.ParsePayload(payload)
	return err
}

func harnessPlanDecisionPayloads(decisions []harnessPlanDecision) []any {
	payloads := make([]any, 0, len(decisions))
	for _, decision := range decisions {
		payloads = append(payloads, map[string]any{
			"ref":        decision.Ref,
			"depends_on": stringSliceToAny(decision.DependsOn),
		})
	}
	return payloads
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
	readinessGate harnessRunReadinessGate,
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
	args = withHarnessRunSpecReadinessOverride(args, readinessGate)

	result, err := handleHaftCommission(ctx, store, args)
	if err != nil {
		return false, "", err
	}
	return true, result, nil
}

func withHarnessRunSpecReadinessOverride(
	args map[string]any,
	readinessGate harnessRunReadinessGate,
) map[string]any {
	override := harnessRunSpecReadinessOverride(readinessGate)
	if override == nil {
		return args
	}

	next := copyStringAnyMap(args)
	next["spec_readiness_override"] = override
	return next
}

func harnessRunSpecReadinessOverride(readinessGate harnessRunReadinessGate) map[string]any {
	if readinessGate.Kind != harnessRunReadinessTacticalAllowed {
		return nil
	}

	return map[string]any{
		"kind":              "tactical",
		"out_of_spec":       true,
		"project_readiness": string(readinessGate.ProjectStatus),
		"reason":            readinessGate.OverrideReason,
	}
}

func recordHarnessRunTacticalOverride(
	ctx context.Context,
	store *artifact.Store,
	selection harnessRunSelection,
	readinessGate harnessRunReadinessGate,
) error {
	override := harnessRunSpecReadinessOverride(readinessGate)
	if override == nil || len(selection.CommissionIDs) == 0 {
		return nil
	}

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tactical override record: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	for _, commissionID := range selection.CommissionIDs {
		commission, err := loadWorkCommissionPayloadForUpdate(ctx, tx, commissionID)
		if err != nil {
			return err
		}

		commission = withCommissionSpecReadinessOverride(commission, override, now)
		commission = appendLifecycleEvent(commission, map[string]any{
			"event":     "tactical_override",
			"runner_id": "haft-cli",
			"reason":    readinessGate.OverrideReason,
			"payload":   copyStringAnyMap(override),
		})

		if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tactical override record: %w", err)
	}
	return nil
}

func withCommissionSpecReadinessOverride(
	commission map[string]any,
	override map[string]any,
	now time.Time,
) map[string]any {
	record := copyStringAnyMap(override)
	if stringField(record, "recorded_at") == "" {
		record["recorded_at"] = now.Format(time.RFC3339)
	}

	commission["spec_readiness_override"] = record
	return commission
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
		PlanPath:               planPath,
		PrepareOnly:            harnessRunPrepareOnly,
		ForceCreate:            harnessRunForceCreate,
		Drain:                  harnessRunDrain,
		Once:                   harnessRunOnce,
		OnceTimeoutMS:          harnessRunOnceTimeoutMS,
		Detach:                 harnessRunDetach,
		Mock:                   harnessRunMock,
		MockAgent:              harnessRunMockAgent || harnessRunMock,
		MockJudge:              harnessRunMockJudge || harnessRunMock,
		Concurrency:            positiveOrDefault(harnessRunConcurrency, 2),
		MaxClaims:              positiveOrDefault(harnessRunMaxClaims, 50),
		PollIntervalMS:         positiveOrDefault(harnessRunPollIntervalMS, 30000),
		StaleLeaseMaxAgeS:      harnessRunStaleLeaseMaxAgeS,
		StatusPath:             statusPath,
		LogPath:                logPath,
		WorkspaceRoot:          workspaceRoot,
		RuntimePath:            runtimePath,
		RepoURL:                repoURL,
		AgentMaxTurns:          20,
		AgentTurnTimeoutMS:     3600000,
		AgentWallClockS:        600,
		JudgeWallClockS:        120,
		ApprovalPolicy:         "never",
		ThreadSandbox:          "workspace-write",
		ProjectionMode:         "local_only",
		ProjectionProfile:      "manager_plain",
		ExternalApprover:       "ivan@weareocta.com",
		StatusHTTPPort:         4767,
		CommissionPlanRef:      stringField(plan, "id"),
		CommissionPlanRevision: stringField(plan, "revision"),
	}
}

func validateHarnessRunOptions(opts harnessRunOptions) error {
	if opts.StaleLeaseMaxAgeS < 0 {
		return fmt.Errorf("--stale-lease-max-age-s must be >= 0")
	}
	if !opts.Drain {
		return nil
	}
	if opts.Once {
		return fmt.Errorf("--drain cannot be combined with --once")
	}
	if opts.Detach {
		return fmt.Errorf("--drain requires the operator stream; omit --detach")
	}
	return nil
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
			"kind":                  "haft",
			"selector":              "runnable",
			"max_claims":            opts.MaxClaims,
			"lease_timeout_s":       300,
			"stale_lease_max_age_s": opts.StaleLeaseMaxAgeS,
			"plan_ref":              opts.CommissionPlanRef,
			"plan_revision":         opts.CommissionPlanRevision,
			"queue":                 nil,
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
	selection harnessRunSelection,
	opts harnessRunOptions,
) error {
	mode := harnessRunMode(opts)
	if opts.MockAgent && opts.MockJudge {
		mode += " mock"
	}

	lines := []string{
		"Plan: " + planPath,
		"Open-Sleigh config: " + configPath,
		"Status: " + opts.StatusPath,
		"Runtime log: " + opts.LogPath,
		"Mode: " + mode,
		"Stale lease max age: " + strconv.Itoa(opts.StaleLeaseMaxAgeS) + "s",
	}

	if created {
		lines = append(lines, "Commissions: created")
	} else {
		lines = append(lines, "Commissions: "+commissionResult)
	}
	if !opts.PrepareOnly {
		lines = append(lines, formatHarnessRunSelectionLines(selection, opts)...)
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), line); err != nil {
			return err
		}
	}
	return nil
}

func harnessRunMode(opts harnessRunOptions) string {
	if opts.Once {
		return "once"
	}
	if opts.Drain {
		return "drain"
	}
	return "long-running"
}

func formatHarnessRunSelectionLines(
	selection harnessRunSelection,
	opts harnessRunOptions,
) []string {
	lines := harnessRunObservationLines(opts)

	if len(selection.CommissionIDs) == 0 {
		return lines
	}

	lines = append(lines, formatHarnessSelectionLine("Selected commission", selection.CommissionIDs))
	lines = append(lines, "Observe result: haft harness result "+selection.CommissionIDs[0])
	lines = append(lines, "Workspace: "+filepath.Join(opts.WorkspaceRoot, selection.CommissionIDs[0]))

	if len(selection.DecisionRefs) > 0 {
		lines = append(lines, formatHarnessSelectionLine("Selected decision", selection.DecisionRefs))
	}

	lines = append(lines, "Note: workspace changes usually appear only after execute starts editing files")
	return lines
}

func harnessRunObservationLines(opts harnessRunOptions) []string {
	if opts.Detach || opts.Once {
		return []string{
			"Observe status: haft harness status --tail 20",
			"Observe log: tail -f " + opts.LogPath,
		}
	}

	lines := []string{
		"Operator stream: live in this terminal",
		"Stop: Ctrl-C stops the stream and the Open-Sleigh process",
	}
	if opts.Drain {
		lines = append(lines, "Drain: exits when the runnable WorkCommission queue is empty")
	}
	return lines
}

func formatHarnessSelectionLine(label string, values []string) string {
	cleaned := uniqueStringsPreserveOrder(cleanStringSlice(values))
	if len(cleaned) == 0 {
		return label + ": none"
	}

	visible := cleaned
	extra := 0
	if len(cleaned) > 3 {
		visible = cleaned[:3]
		extra = len(cleaned) - len(visible)
	}

	summary := strings.Join(visible, ", ")
	if extra > 0 {
		summary += fmt.Sprintf(" (+%d more)", extra)
	}
	if len(cleaned) > 1 {
		return label + "s: " + summary
	}
	return label + ": " + summary
}

func startOpenSleigh(cmd *cobra.Command, opts harnessRunOptions, configPath string) error {
	process, err := openSleighProcess(opts, configPath)
	if err != nil {
		return err
	}

	configureOpenSleighProcess(process, opts, cmd.OutOrStdout(), cmd.ErrOrStderr(), cmd.InOrStdin())
	return process.Run()
}

func startOpenSleighProcess(
	opts harnessRunOptions,
	configPath string,
	stdout io.Writer,
	stderr io.Writer,
	stdin io.Reader,
) (*exec.Cmd, error) {
	process, err := openSleighProcess(opts, configPath)
	if err != nil {
		return nil, err
	}

	configureOpenSleighProcess(process, opts, stdout, stderr, stdin)
	if err := process.Start(); err != nil {
		return nil, err
	}
	return process, nil
}

func openSleighProcess(opts harnessRunOptions, configPath string) (*exec.Cmd, error) {
	args := openSleighStartArgs(opts, configPath)
	kind, err := openSleighRuntimeKind(opts.RuntimePath)
	if err != nil {
		return nil, err
	}
	if kind == "release" {
		return openSleighReleaseProcess(opts, args), nil
	}
	return openSleighSourceProcess(args), nil
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

func openSleighSourceProcess(args []string) *exec.Cmd {
	mixArgs := append([]string{"open_sleigh.start"}, args...)
	return exec.Command("mix", mixArgs...)
}

func openSleighReleaseProcess(opts harnessRunOptions, args []string) *exec.Cmd {
	expression := "Application.ensure_all_started(:mix); Mix.Tasks.OpenSleigh.Start.run(" +
		elixirStringListLiteral(args) +
		")"

	return exec.Command(openSleighReleaseExecutable(opts.RuntimePath), "eval", expression)
}

func openSleighReleaseExecutable(runtimePath string) string {
	return filepath.Join(runtimePath, "bin", "open_sleigh")
}

func configureOpenSleighProcess(
	process *exec.Cmd,
	opts harnessRunOptions,
	stdout io.Writer,
	stderr io.Writer,
	stdin io.Reader,
) {
	process.Dir = opts.RuntimePath
	process.Env = append(
		os.Environ(),
		"REPO_URL="+opts.RepoURL,
		"ERL_CRASH_DUMP="+harnessRuntimeCrashDumpPath(),
		"OPEN_SLEIGH_STALE_LEASE_MAX_AGE_S="+strconv.Itoa(opts.StaleLeaseMaxAgeS),
	)
	process.Stdout = stdout
	process.Stderr = stderr
	process.Stdin = stdin
}

func harnessRuntimeCrashDumpPath() string {
	root := strings.TrimSpace(os.Getenv("OPEN_SLEIGH_CRASH_DUMP_DIR"))
	if root == "" {
		root = defaultHarnessCrashDumpDir()
	}
	if absoluteRoot, err := filepath.Abs(root); err == nil {
		root = absoluteRoot
	}
	_ = os.MkdirAll(root, 0o700)
	return filepath.Join(root, fmt.Sprintf("runtime-%d-%d.dump", os.Getpid(), time.Now().UTC().UnixNano()))
}

func defaultHarnessCrashDumpDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(os.TempDir(), "open-sleigh-crash-dumps")
	}
	return filepath.Join(home, ".open-sleigh", "crash_dumps")
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
