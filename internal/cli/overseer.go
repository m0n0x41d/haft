package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/present"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/logger"
)

var (
	overseerPacketCommit      = "HEAD"
	overseerPacketJSON        bool
	overseerRunCommit         = "HEAD"
	overseerRunJSON           bool
	overseerRunQuiet          bool
	overseerInitAgent         = overseer.ReviewerAuto
	overseerInitCommand       string
	overseerInitReviewOnHook  bool
	overseerInitTimeout       = 180
	overseerInitJSON          bool
	overseerHookCommit        = "HEAD"
	overseerHookJSON          bool
	overseerHookQuiet         bool
	overseerHookAsync         bool
	overseerShowJSON          bool
	overseerRemindJSON        bool
	overseerMaintainJSON      bool
	overseerJudgmentJSON      bool
	overseerDrainJSON         bool
	overseerDrainDryRun       bool
	overseerDaemonJSON        bool
	overseerDaemonIdleTimeout = 300
	overseerDaemonPollSeconds = 2
	overseerReviewRun         = "latest"
	overseerReviewJSON        bool
	overseerReviewQuiet       bool
	overseerIngestRun         = "latest"
	overseerIngestFile        string
	overseerIngestJSON        bool
	overseerDispositionRun    = "latest"
	overseerDispositionStatus string
	overseerDispositionActor  string
	overseerDispositionReason string
	overseerDispositionCommit string
	overseerDispositionJSON   bool
)

var overseerCmd = &cobra.Command{
	Use:   "overseer",
	Short: "Run the advisory overseer review loop",
	Long: `Run deterministic overseer packet, maintenance, reviewer, ingest, and
disposition flows.

All reviewer output is advisory: overseer never approves, merges, deploys,
decides, commissions, rebaselines, or contributes directly to R_eff.`,
}

var overseerInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize overseer config and soft post-commit hook",
	Long: `Initialize the advisory overseer review loop for an existing haft
project.

By default, Haft auto-detects the project's configured agent from project
carriers such as .codex/config.toml, .mcp.json, or CLAUDE.md. Explicit flags
override detection:

  haft overseer init
  haft overseer init --agent codex
  haft overseer init --agent claude
  haft overseer init --agent command --command ./reviewer.sh

The installed git hook is soft: reviewer failures never block the commit.`,
	RunE: runOverseerInit,
}

var overseerPacketCmd = &cobra.Command{
	Use:   "packet",
	Short: "Build a deterministic ReviewPacket for a commit",
	RunE:  runOverseerPacket,
}

var overseerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Build and store a soft overseer ReviewRun for a commit",
	RunE:  runOverseerRun,
}

var overseerHookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Run the soft post-commit overseer pipeline",
	RunE:  runOverseerHook,
}

var overseerShowCmd = &cobra.Command{
	Use:   "show [run-id|latest]",
	Short: "Show a stored overseer ReviewRun",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runOverseerShow,
}

var overseerRemindCmd = &cobra.Command{
	Use:   "remind",
	Short: "Print an agent-facing reminder for the latest overseer run",
	RunE:  runOverseerRemind,
}

var overseerMaintainCmd = &cobra.Command{
	Use:   "maintain",
	Short: "Scan governance maintenance debt and store advisory overseer signals",
	RunE:  runOverseerMaintain,
}

var overseerJudgmentCmd = &cobra.Command{
	Use:   "judgment",
	Short: "Build a read-only judgment packet for maintenance tasks that need human judgment",
	RunE:  runOverseerJudgment,
}

var overseerDrainCmd = &cobra.Command{
	Use:   "drain",
	Short: "Explicitly drain machine-safe maintenance tasks and report the rest",
	RunE:  runOverseerDrain,
}

var overseerUndoCmd = &cobra.Command{
	Use:   "undo <maintenance-id> <action-id>",
	Short: "Restore the prior baseline recorded by an autonomous maintenance action",
	Args:  cobra.ExactArgs(2),
	RunE:  runOverseerUndo,
}

var overseerStatusJSON bool

var overseerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the latest overseer signals and autonomous-maintenance ledger (read-only, never re-runs the loop)",
	RunE:  runOverseerStatus,
}

var overseerDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the overseer background review daemon",
}

var overseerDaemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the overseer background review daemon",
	RunE:  runOverseerDaemonStart,
}

var overseerDaemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the overseer background review daemon in the foreground",
	RunE:  runOverseerDaemonRun,
}

var overseerDaemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show overseer daemon and queue status",
	RunE:  runOverseerDaemonStatus,
}

var overseerDaemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the overseer background review daemon",
	RunE:  runOverseerDaemonStop,
}

var overseerReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run the configured advisory reviewer and ingest its findings",
	RunE:  runOverseerReview,
}

var overseerIngestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest advisory reviewer findings into a stored overseer run",
	RunE:  runOverseerIngest,
}

var overseerDispositionCmd = &cobra.Command{
	Use:   "disposition FINDING_ID",
	Short: "Record an explicit disposition for an overseer review finding",
	Args:  cobra.ExactArgs(1),
	RunE:  runOverseerDisposition,
}

func init() {
	overseerInitCmd.Flags().StringVar(&overseerInitAgent, "agent", overseer.ReviewerAuto, "reviewer agent: auto, manual, command, codex, or claude")
	overseerInitCmd.Flags().StringVar(&overseerInitAgent, "reviewer", overseer.ReviewerAuto, "alias for --agent")
	overseerInitCmd.Flags().StringVar(&overseerInitCommand, "command", "", "custom reviewer command; receives HAFT_OVERSEER_* env vars")
	overseerInitCmd.Flags().BoolVar(&overseerInitReviewOnHook, "review-on-hook", false, "force configured reviewer to run from the post-commit hook")
	overseerInitCmd.Flags().IntVar(&overseerInitTimeout, "timeout", 180, "reviewer timeout in seconds")
	overseerInitCmd.Flags().BoolVar(&overseerInitJSON, "json", false, "print structured JSON output")
	overseerPacketCmd.Flags().StringVar(&overseerPacketCommit, "commit", "HEAD", "commit ref to packetize")
	overseerPacketCmd.Flags().BoolVar(&overseerPacketJSON, "json", false, "print structured JSON output")
	overseerRunCmd.Flags().StringVar(&overseerRunCommit, "commit", "HEAD", "commit ref to packetize and store")
	overseerRunCmd.Flags().BoolVar(&overseerRunJSON, "json", false, "print structured JSON output")
	overseerRunCmd.Flags().BoolVar(&overseerRunQuiet, "quiet", false, "suppress output; used by git hooks")
	overseerHookCmd.Flags().StringVar(&overseerHookCommit, "commit", "HEAD", "commit ref to packetize from a git hook")
	overseerHookCmd.Flags().BoolVar(&overseerHookJSON, "json", false, "print structured JSON output")
	overseerHookCmd.Flags().BoolVar(&overseerHookQuiet, "quiet", false, "suppress output; used by git hooks")
	overseerHookCmd.Flags().BoolVar(&overseerHookAsync, "async", false, "store packet now and schedule configured reviewer in the background")
	overseerShowCmd.Flags().BoolVar(&overseerShowJSON, "json", false, "print structured JSON output")
	overseerRemindCmd.Flags().BoolVar(&overseerRemindJSON, "json", false, "print structured JSON output")
	overseerMaintainCmd.Flags().BoolVar(&overseerMaintainJSON, "json", false, "print structured JSON output")
	overseerJudgmentCmd.Flags().BoolVar(&overseerJudgmentJSON, "json", false, "print structured JSON output")
	overseerDrainCmd.Flags().BoolVar(&overseerDrainJSON, "json", false, "print structured JSON output")
	overseerDrainCmd.Flags().BoolVar(&overseerDrainDryRun, "dry-run", false, "propose safe actions without mutating baselines, evidence, or stored maintenance runs")
	overseerDaemonStartCmd.Flags().BoolVar(&overseerDaemonJSON, "json", false, "print structured JSON output")
	overseerDaemonRunCmd.Flags().IntVar(&overseerDaemonIdleTimeout, "idle-timeout", 300, "exit after this many idle seconds; 0 waits forever")
	overseerDaemonRunCmd.Flags().IntVar(&overseerDaemonPollSeconds, "poll-interval", 2, "queue poll interval in seconds")
	overseerDaemonRunCmd.Flags().BoolVar(&overseerDaemonJSON, "json", false, "print structured JSON output")
	overseerDaemonStatusCmd.Flags().BoolVar(&overseerDaemonJSON, "json", false, "print structured JSON output")
	overseerDaemonStopCmd.Flags().BoolVar(&overseerDaemonJSON, "json", false, "print structured JSON output")
	overseerReviewCmd.Flags().StringVar(&overseerReviewRun, "run", "latest", "review run id to review")
	overseerReviewCmd.Flags().BoolVar(&overseerReviewJSON, "json", false, "print structured JSON output")
	overseerReviewCmd.Flags().BoolVar(&overseerReviewQuiet, "quiet", false, "suppress output; used by background hook workers")
	overseerIngestCmd.Flags().StringVar(&overseerIngestRun, "run", "latest", "review run id to update")
	overseerIngestCmd.Flags().StringVar(&overseerIngestFile, "input-file", "", "JSON review result file to ingest")
	overseerIngestCmd.Flags().BoolVar(&overseerIngestJSON, "json", false, "print structured JSON output")
	overseerDispositionCmd.Flags().StringVar(&overseerDispositionRun, "run", "latest", "review run id to update")
	overseerDispositionCmd.Flags().StringVar(&overseerDispositionStatus, "status", "", "disposition status: fixed_by_commit, false_positive, waived_by_human, escalated_to_decision, superseded_by_rewrite")
	overseerDispositionCmd.Flags().StringVar(&overseerDispositionActor, "actor", "agent", "actor recording the disposition")
	overseerDispositionCmd.Flags().StringVar(&overseerDispositionReason, "reason", "", "audit-trail reason for the disposition")
	overseerDispositionCmd.Flags().StringVar(&overseerDispositionCommit, "commit", "", "commit sha associated with the disposition, if any")
	overseerDispositionCmd.Flags().BoolVar(&overseerDispositionJSON, "json", false, "print structured JSON output")
	overseerCmd.AddCommand(overseerInitCmd)
	overseerCmd.AddCommand(overseerPacketCmd)
	overseerCmd.AddCommand(overseerRunCmd)
	overseerCmd.AddCommand(overseerHookCmd)
	overseerCmd.AddCommand(overseerShowCmd)
	overseerCmd.AddCommand(overseerRemindCmd)
	overseerStatusCmd.Flags().BoolVar(&overseerStatusJSON, "json", false, "print structured JSON output")
	overseerCmd.AddCommand(overseerMaintainCmd)
	overseerCmd.AddCommand(overseerJudgmentCmd)
	overseerCmd.AddCommand(overseerDrainCmd)
	overseerCmd.AddCommand(overseerUndoCmd)
	overseerCmd.AddCommand(overseerStatusCmd)
	overseerDaemonCmd.AddCommand(overseerDaemonStartCmd)
	overseerDaemonCmd.AddCommand(overseerDaemonRunCmd)
	overseerDaemonCmd.AddCommand(overseerDaemonStatusCmd)
	overseerDaemonCmd.AddCommand(overseerDaemonStopCmd)
	overseerCmd.AddCommand(overseerDaemonCmd)
	overseerCmd.AddCommand(overseerReviewCmd)
	overseerCmd.AddCommand(overseerIngestCmd)
	overseerCmd.AddCommand(overseerDispositionCmd)
	rootCmd.AddCommand(overseerCmd)
}

func runOverseerInit(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	factory := func() (overseer.Config, error) {
		return buildOverseerConfigForProject(projectRoot, overseerSetupOptions{
			reviewer:     overseerInitAgent,
			command:      overseerInitCommand,
			reviewOnHook: overseerInitReviewOnHook,
			timeout:      overseerInitTimeout,
		})
	}
	config, result, err := configureOverseerWithConfig(projectRoot, "haft", factory)
	if err != nil {
		return err
	}

	if overseerInitJSON {
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"config": config,
			"hook":   result,
		})
	}
	return writeOverseerInitSummary(cmd.OutOrStdout(), config, result)
}

func runOverseerPacket(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return fmt.Errorf("get DB path: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)")
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer sqlDB.Close()

	store := artifact.NewStore(sqlDB)
	packet, err := buildOverseerPacket(ctx, store, projectRoot, overseerPacketCommit, Version)
	if err != nil {
		return err
	}

	if overseerPacketJSON {
		return writeOverseerPacketJSON(cmd.OutOrStdout(), packet)
	}

	return writeOverseerPacketSummary(cmd.OutOrStdout(), packet)
}

func runOverseerRun(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, store, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	packet, err := buildOverseerPacket(ctx, store, projectRoot, overseerRunCommit, Version)
	if err != nil {
		return err
	}

	run := overseer.NewDeterministicReviewRun(packet, packet.CreatedAt)
	if err := overseer.StoreRun(projectRoot, packet, run); err != nil {
		return err
	}

	stored := overseer.StoredRun{Run: run, Packet: packet}
	if overseerRunQuiet {
		return nil
	}
	if overseerRunJSON {
		return writeOverseerRunJSON(cmd.OutOrStdout(), stored)
	}
	return writeOverseerRunSummary(cmd.OutOrStdout(), stored)
}

func runOverseerHook(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, store, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	config, err := overseer.LoadConfig(projectRoot)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if overseerHookAsync {
		stored, maintenance, schedule, err := runOverseerHookAsyncPipeline(
			ctx,
			projectRoot,
			store,
			config,
			overseerHookCommit,
		)
		if err != nil {
			return err
		}
		if overseerHookQuiet {
			return nil
		}
		if overseerHookJSON {
			return writeJSON(output, map[string]any{
				"run":             stored.Run,
				"packet":          stored.Packet,
				"maintenance":     maintenance,
				"review_schedule": schedule,
			})
		}
		return writeOverseerHookAsyncSummary(output, stored, maintenance, schedule)
	}

	if !overseerHookQuiet && !overseerHookJSON {
		if err := writeOverseerHookStart(output, config); err != nil {
			return err
		}
	}

	stored, maintenance, reviewErr, err := runOverseerHookPipeline(
		ctx,
		projectRoot,
		store,
		config,
		overseerHookCommit,
	)
	if err != nil {
		return err
	}

	if overseerHookQuiet {
		return nil
	}
	if overseerHookJSON {
		return writeJSON(output, map[string]any{
			"run":          stored.Run,
			"packet":       stored.Packet,
			"maintenance":  maintenance,
			"review_error": reviewErrString(reviewErr),
		})
	}
	return writeOverseerHookSummary(output, stored, maintenance, reviewErr)
}

func runOverseerShow(cmd *cobra.Command, args []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	runID := "latest"
	if len(args) > 0 {
		runID = args[0]
	}

	stored, err := overseer.LoadRun(projectRoot, runID)
	if err != nil {
		return err
	}
	if overseerShowJSON {
		return writeOverseerRunJSON(cmd.OutOrStdout(), stored)
	}
	return writeOverseerRunSummary(cmd.OutOrStdout(), stored)
}

func runOverseerRemind(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	stored, err := overseer.LoadLatestRun(projectRoot)
	if err != nil {
		if overseerRemindJSON {
			return writeJSON(cmd.OutOrStdout(), overseer.Reminder{
				HasReminder: false,
				Message:     "No overseer review run found.",
			})
		}
		_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), "No overseer review run found.")
		return writeErr
	}

	reminder := overseer.BuildReminder(stored)
	if overseerRemindJSON {
		return writeJSON(cmd.OutOrStdout(), reminder)
	}
	if !reminder.HasReminder {
		_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), reminder.Message)
		return writeErr
	}
	_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), reminder.Message)
	return writeErr
}

func runOverseerMaintain(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, store, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	run, err := buildAndStoreOverseerMaintenance(ctx, store, projectRoot)
	if err != nil {
		return err
	}

	if overseerMaintainJSON {
		return writeJSON(cmd.OutOrStdout(), run)
	}
	return writeOverseerMaintenanceSummary(cmd.OutOrStdout(), run)
}

func runOverseerJudgment(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, store, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	plan, err := artifact.BuildMaintenancePlan(ctx, store, projectRoot)
	if err != nil {
		return err
	}
	review := artifact.BuildMaintenanceJudgmentReview(plan)

	if overseerJudgmentJSON {
		return writeJSON(cmd.OutOrStdout(), review)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), present.MaintenanceJudgmentReviewResponse(review, ""))
	return err
}

func runOverseerDrain(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, store, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	report, err := buildMaintenanceDrainReport(ctx, store, projectRoot, overseerDrainDryRun)
	if err != nil {
		return err
	}
	if overseerDrainJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), present.MaintenanceDrainResponse(report, ""))
	return err
}

func runOverseerDaemonStart(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	result, err := startOverseerDaemon(projectRoot)
	if err != nil {
		return err
	}

	if overseerDaemonJSON {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	return writeOverseerDaemonStartSummary(cmd.OutOrStdout(), result)
}

func runOverseerDaemonRun(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	config, err := overseer.LoadConfig(projectRoot)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	summary, err := runOverseerDaemonLoop(
		ctx,
		projectRoot,
		config,
		time.Duration(overseerDaemonIdleTimeout)*time.Second,
		time.Duration(overseerDaemonPollSeconds)*time.Second,
		cmd.OutOrStdout(),
	)
	if err != nil {
		return err
	}

	if overseerDaemonJSON {
		return writeJSON(cmd.OutOrStdout(), summary)
	}
	return nil
}

func runOverseerDaemonStatus(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	status, err := overseer.LoadDaemonStatus(projectRoot)
	if err != nil {
		return err
	}
	if overseerDaemonJSON {
		return writeJSON(cmd.OutOrStdout(), status)
	}
	return writeOverseerDaemonStatusSummary(cmd.OutOrStdout(), status)
}

func runOverseerDaemonStop(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	if err := overseer.StopDaemon(projectRoot); err != nil {
		return err
	}
	status, err := overseer.LoadDaemonStatus(projectRoot)
	if err != nil {
		return err
	}
	if overseerDaemonJSON {
		return writeJSON(cmd.OutOrStdout(), status)
	}
	return writeOverseerDaemonStopSummary(cmd.OutOrStdout(), status)
}

func runOverseerReview(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	config, err := overseer.LoadConfig(projectRoot)
	if err != nil {
		return err
	}
	if !overseer.ReviewEnabled(config) {
		return fmt.Errorf("overseer llm_review is off; set llm_review: on and reviewer_command in .haft/overseer/config.yaml")
	}

	stored, err := overseer.LoadRun(projectRoot, overseerReviewRun)
	if err != nil {
		return err
	}

	result, err := overseer.RunConfiguredReviewer(ctx, projectRoot, config, stored)
	if err != nil {
		return err
	}

	stored, err = overseer.IngestReviewResult(
		stored,
		result.Input,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	if err := overseer.StoreRun(projectRoot, stored.Packet, stored.Run); err != nil {
		return err
	}

	if overseerReviewJSON {
		return writeOverseerRunJSON(cmd.OutOrStdout(), stored)
	}
	if overseerReviewQuiet {
		return nil
	}
	return writeOverseerReviewSummary(cmd.OutOrStdout(), stored, result)
}

func runOverseerIngest(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	if strings.TrimSpace(overseerIngestFile) == "" {
		return fmt.Errorf("--input-file is required")
	}

	stored, err := overseer.LoadRun(projectRoot, overseerIngestRun)
	if err != nil {
		return err
	}

	input, err := readReviewResultInput(overseerIngestFile)
	if err != nil {
		return err
	}

	stored, err = overseer.IngestReviewResult(
		stored,
		input,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}
	if err := overseer.StoreRun(projectRoot, stored.Packet, stored.Run); err != nil {
		return err
	}

	if overseerIngestJSON {
		return writeOverseerRunJSON(cmd.OutOrStdout(), stored)
	}
	return writeOverseerIngestSummary(cmd.OutOrStdout(), stored)
}

func runOverseerDisposition(cmd *cobra.Command, args []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	stored, err := overseer.LoadRun(projectRoot, overseerDispositionRun)
	if err != nil {
		return err
	}

	stored, err = overseer.ApplyDisposition(stored, overseer.ReviewDisposition{
		FindingID: args[0],
		Status:    overseerDispositionStatus,
		Actor:     overseerDispositionActor,
		Reason:    overseerDispositionReason,
		CommitSHA: overseerDispositionCommit,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if err := overseer.StoreRun(projectRoot, stored.Packet, stored.Run); err != nil {
		return err
	}

	if overseerDispositionJSON {
		return writeOverseerRunJSON(cmd.OutOrStdout(), stored)
	}
	return writeOverseerDispositionSummary(cmd.OutOrStdout(), stored, args[0], overseerDispositionStatus)
}

func readReviewResultInput(path string) (overseer.ReviewResultInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return overseer.ReviewResultInput{}, fmt.Errorf("read review result input: %w", err)
	}

	var input overseer.ReviewResultInput
	if err := json.Unmarshal(data, &input); err != nil {
		return overseer.ReviewResultInput{}, fmt.Errorf("decode review result input: %w", err)
	}
	return input, nil
}

func overseerStatusPrefix(projectRoot string) string {
	summary, err := overseer.LoadStatusSummary(projectRoot)
	if err != nil {
		return ""
	}
	return overseer.FormatStatusSignals(summary)
}

type overseerReviewSchedule struct {
	Queued      bool   `json:"queued"`
	JobID       string `json:"job_id,omitempty"`
	ReviewRunID string `json:"review_run_id,omitempty"`
	DaemonPID   int    `json:"daemon_pid,omitempty"`
	LogPath     string `json:"log_path,omitempty"`
	DaemonLog   string `json:"daemon_log,omitempty"`
	Skipped     string `json:"skipped,omitempty"`
	Error       string `json:"error,omitempty"`
}

type overseerDaemonStartResult struct {
	Started bool   `json:"started"`
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	LogPath string `json:"log_path,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

var startOverseerDaemon = startOverseerDaemonProcess

func runOverseerHookAsyncPipeline(
	ctx context.Context,
	projectRoot string,
	store *artifact.Store,
	config overseer.Config,
	commitRef string,
) (overseer.StoredRun, overseer.MaintenanceRun, overseerReviewSchedule, error) {
	stored, maintenance, err := prepareOverseerHookRun(ctx, projectRoot, store, commitRef)
	if err != nil {
		return overseer.StoredRun{}, overseer.MaintenanceRun{}, overseerReviewSchedule{}, err
	}

	if !overseer.ShouldReviewPacket(config, stored.Packet) {
		return stored, maintenance, overseerReviewSchedule{
			ReviewRunID: stored.Run.ReviewRunID,
			Skipped:     "packet_not_eligible_or_review_disabled",
		}, nil
	}

	job, err := overseer.EnqueueReviewJob(projectRoot, stored, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return stored, maintenance, overseerReviewSchedule{}, err
	}

	startResult, startErr := startOverseerDaemon(projectRoot)
	schedule := overseerReviewSchedule{
		Queued:      true,
		JobID:       job.JobID,
		ReviewRunID: stored.Run.ReviewRunID,
		DaemonPID:   startResult.PID,
		LogPath:     job.LogPath,
		DaemonLog:   startResult.LogPath,
	}

	if startErr != nil {
		schedule.Error = startErr.Error()
	}
	if strings.TrimSpace(startResult.Reason) != "" {
		schedule.Skipped = startResult.Reason
	}

	return stored, maintenance, schedule, nil
}

func prepareOverseerHookRun(
	ctx context.Context,
	projectRoot string,
	store *artifact.Store,
	commitRef string,
) (overseer.StoredRun, overseer.MaintenanceRun, error) {
	packet, err := buildOverseerPacket(ctx, store, projectRoot, commitRef, Version)
	if err != nil {
		return overseer.StoredRun{}, overseer.MaintenanceRun{}, err
	}

	run := overseer.NewDeterministicReviewRun(packet, packet.CreatedAt)
	if err := overseer.StoreRun(projectRoot, packet, run); err != nil {
		return overseer.StoredRun{}, overseer.MaintenanceRun{}, err
	}

	maintenance, err := buildAndStoreOverseerMaintenance(ctx, store, projectRoot)
	if err != nil {
		return overseer.StoredRun{Run: run, Packet: packet}, overseer.MaintenanceRun{}, err
	}

	return overseer.StoredRun{Run: run, Packet: packet}, maintenance, nil
}

func startOverseerDaemonProcess(projectRoot string) (overseerDaemonStartResult, error) {
	status, err := overseer.LoadDaemonStatus(projectRoot)
	if err != nil {
		return overseerDaemonStartResult{}, err
	}
	if status.Running {
		return overseerDaemonStartResult{
			Started: false,
			Running: true,
			PID:     status.PID,
			LogPath: overseer.DaemonLogPath(projectRoot),
			Reason:  "already_running",
		}, nil
	}

	logPath := overseer.DaemonLogPath(projectRoot)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return overseerDaemonStartResult{}, fmt.Errorf("create overseer log dir: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return overseerDaemonStartResult{}, fmt.Errorf("open overseer daemon log: %w", err)
	}
	defer logFile.Close()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return overseerDaemonStartResult{}, fmt.Errorf("open devnull: %w", err)
	}
	defer devNull.Close()

	binaryPath, err := os.Executable()
	if err != nil {
		binaryPath = overseer.DefaultToolName
	}

	cmd := exec.Command(binaryPath, "overseer", "daemon", "run", "--idle-timeout", strconv.Itoa(overseerDaemonIdleTimeout))
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "HAFT_PROJECT_ROOT="+projectRoot)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return overseerDaemonStartResult{}, fmt.Errorf("start overseer daemon: %w", err)
	}

	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return overseerDaemonStartResult{}, fmt.Errorf("release overseer daemon: %w", err)
	}

	return overseerDaemonStartResult{
		Started: true,
		Running: true,
		PID:     pid,
		LogPath: logPath,
	}, nil
}

func runOverseerDaemonLoop(
	ctx context.Context,
	projectRoot string,
	config overseer.Config,
	idleTimeout time.Duration,
	pollInterval time.Duration,
	output io.Writer,
) (overseer.QueueSummary, error) {
	lease, err := overseer.AcquireDaemonLease(projectRoot)
	if err != nil {
		if errors.Is(err, overseer.ErrDaemonAlreadyRunning) {
			jobs, listErr := overseer.ListReviewJobs(projectRoot)
			if listErr != nil {
				return overseer.QueueSummary{}, listErr
			}
			return overseer.BuildQueueSummary(jobs), nil
		}
		return overseer.QueueSummary{}, err
	}
	defer lease.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := overseer.RequeueRunningReviewJobs(projectRoot, "daemon restarted while job was running", now); err != nil {
		return overseer.QueueSummary{}, err
	}

	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}

	idleSince := time.Now()
	for {
		job, ok, err := overseer.NextRunnableReviewJob(projectRoot)
		if err != nil {
			return overseer.QueueSummary{}, err
		}
		if ok {
			idleSince = time.Now()
			if err := processOverseerReviewJob(ctx, projectRoot, config, job, output); err != nil {
				return overseer.QueueSummary{}, err
			}
			continue
		}

		jobs, err := overseer.ListReviewJobs(projectRoot)
		if err != nil {
			return overseer.QueueSummary{}, err
		}
		if idleTimeout > 0 && time.Since(idleSince) >= idleTimeout {
			return overseer.BuildQueueSummary(jobs), nil
		}

		select {
		case <-ctx.Done():
			return overseer.BuildQueueSummary(jobs), nil
		case <-time.After(pollInterval):
		}
	}
}

func processOverseerReviewJob(
	ctx context.Context,
	projectRoot string,
	config overseer.Config,
	job overseer.ReviewJob,
	output io.Writer,
) error {
	now := time.Now().UTC().Format(time.RFC3339)
	job.Status = overseer.JobStatusRunning
	job.Attempts++
	if strings.TrimSpace(job.StartedAt) == "" {
		job.StartedAt = now
	}
	job.UpdatedAt = now
	if err := overseer.StoreReviewJob(projectRoot, job); err != nil {
		return err
	}
	appendOverseerReviewLog(job.LogPath, fmt.Sprintf("started %s attempt=%d\n", job.JobID, job.Attempts))

	stored, err := overseer.LoadRun(projectRoot, job.ReviewRunID)
	if err != nil {
		return finishOverseerReviewJobFailure(projectRoot, config, overseer.StoredRun{}, job, err)
	}

	result, reviewErr := overseer.RunConfiguredReviewer(ctx, projectRoot, config, stored)
	if reviewErr != nil {
		appendOverseerReviewLog(job.LogPath, "reviewer error: "+reviewErr.Error()+"\n")
		if reviewContextCanceled(ctx, reviewErr) {
			return preserveOverseerReviewJobForRestart(projectRoot, job, reviewErr)
		}
		return finishOverseerReviewJobFailure(projectRoot, config, stored, job, reviewErr)
	}

	stored, err = overseer.IngestReviewResult(
		stored,
		result.Input,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return finishOverseerReviewJobFailure(projectRoot, config, stored, job, err)
	}
	if err := overseer.StoreRun(projectRoot, stored.Packet, stored.Run); err != nil {
		return finishOverseerReviewJobFailure(projectRoot, config, stored, job, err)
	}

	job.Status = overseer.JobStatusDone
	job.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	job.UpdatedAt = job.FinishedAt
	job.LastError = ""
	if err := overseer.StoreReviewJob(projectRoot, job); err != nil {
		return err
	}

	appendOverseerReviewLog(job.LogPath, fmt.Sprintf(
		"done %s verdict=%s findings=%d\n",
		job.JobID,
		stored.Run.Verdict,
		len(stored.Run.Findings),
	))
	if output != nil {
		_, _ = fmt.Fprintf(output, "haft overseer daemon: reviewed %s (%s)\n", job.ReviewRunID, stored.Run.Verdict)
	}
	return nil
}

func reviewContextCanceled(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return ctx.Err() != nil
}

func preserveOverseerReviewJobForRestart(
	projectRoot string,
	job overseer.ReviewJob,
	err error,
) error {
	now := time.Now().UTC().Format(time.RFC3339)
	job.Status = overseer.JobStatusRunning
	job.LastError = "daemon cancellation while reviewer was running: " + err.Error()
	job.UpdatedAt = now
	appendOverseerReviewLog(job.LogPath, fmt.Sprintf("preserved %s for daemon restart error=%s\n", job.JobID, err.Error()))
	return overseer.StoreReviewJob(projectRoot, job)
}

func finishOverseerReviewJobFailure(
	projectRoot string,
	config overseer.Config,
	stored overseer.StoredRun,
	job overseer.ReviewJob,
	err error,
) error {
	now := time.Now().UTC().Format(time.RFC3339)
	job.LastError = err.Error()
	job.UpdatedAt = now

	if job.Attempts < job.MaxAttempts {
		job.Status = overseer.JobStatusPending
		appendOverseerReviewLog(job.LogPath, fmt.Sprintf("retry %s error=%s\n", job.JobID, err.Error()))
		return overseer.StoreReviewJob(projectRoot, job)
	}

	job.Status = overseer.JobStatusFailed
	job.FinishedAt = now
	appendOverseerReviewLog(job.LogPath, fmt.Sprintf("failed %s error=%s\n", job.JobID, err.Error()))

	if stored.Run.ReviewRunID != "" {
		input := overseer.ReviewAbstention(config, stored, err.Error())
		updated, ingestErr := overseer.IngestReviewResult(stored, input, now)
		if ingestErr != nil {
			return ingestErr
		}
		if storeErr := overseer.StoreRun(projectRoot, updated.Packet, updated.Run); storeErr != nil {
			return storeErr
		}
	}

	return overseer.StoreReviewJob(projectRoot, job)
}

func appendOverseerReviewLog(path string, text string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(time.Now().UTC().Format(time.RFC3339) + " " + text)
}

func runOverseerHookPipeline(
	ctx context.Context,
	projectRoot string,
	store *artifact.Store,
	config overseer.Config,
	commitRef string,
) (overseer.StoredRun, overseer.MaintenanceRun, error, error) {
	stored, maintenance, err := prepareOverseerHookRun(ctx, projectRoot, store, commitRef)
	if err != nil {
		return stored, maintenance, nil, err
	}

	if !overseer.ShouldReviewPacket(config, stored.Packet) {
		return stored, maintenance, nil, nil
	}

	reviewResult, reviewErr := overseer.RunConfiguredReviewer(ctx, projectRoot, config, stored)
	if reviewErr != nil {
		reviewResult.Input = overseer.ReviewAbstention(config, stored, reviewErr.Error())
	}

	stored, err = overseer.IngestReviewResult(
		stored,
		reviewResult.Input,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return stored, maintenance, reviewErr, err
	}
	if err := overseer.StoreRun(projectRoot, stored.Packet, stored.Run); err != nil {
		return stored, maintenance, reviewErr, err
	}

	return stored, maintenance, reviewErr, nil
}

func buildAndStoreOverseerMaintenance(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) (overseer.MaintenanceRun, error) {
	config, err := overseer.LoadConfig(projectRoot)
	if err != nil {
		logger.Warn().Err(err).Msg("maintenance: config load failed — execute-phase disabled for this run")
		config = overseer.DefaultConfig()
		config.MaintenanceRebaseline = overseer.MaintenanceModeOff
		config.MaintenanceRevalidateStale = overseer.MaintenanceModeOff
	}

	// Execute-phase runs FIRST so the classification below reflects the world
	// after deterministic-gate rebaselines and machine evidence, not before.
	executed := executeMaintenancePlan(ctx, store, projectRoot, config)

	report, err := buildCheckReport(ctx, store, projectRoot)
	if err != nil {
		return overseer.MaintenanceRun{}, fmt.Errorf("build governance maintenance report: %w", err)
	}

	run, err := overseer.BuildMaintenanceRun(overseer.MaintenanceInput{
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		Stale:        mapMaintenanceStale(report.Stale),
		Drift:        mapMaintenanceDrift(report.Drifted),
		SpecHealth:   mapMaintenanceSpecHealth(report.SpecHealth),
		CoverageGaps: mapMaintenanceCoverage(report.CoverageGaps),
		Executed:     executed,
	})
	if err != nil {
		return overseer.MaintenanceRun{}, err
	}
	if err := overseer.StoreMaintenanceRun(projectRoot, run); err != nil {
		return overseer.MaintenanceRun{}, err
	}
	return run, nil
}

// runOverseerStatus is the read-only viewer: latest review signals plus the
// autonomous-maintenance ledger, WITHOUT re-running the loop (viewing must
// never mutate — `haft overseer maintain` is the act, this is the look).
func runOverseerStatus(cmd *cobra.Command, _ []string) error {
	projectRoot, _, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	summary, err := overseer.LoadStatusSummary(projectRoot)
	if err != nil {
		return fmt.Errorf("load overseer status: %w", err)
	}
	if overseerStatusJSON {
		return writeJSON(cmd.OutOrStdout(), summary)
	}

	rendered := overseer.FormatStatusSignals(summary)
	if strings.TrimSpace(rendered) == "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No overseer signals recorded yet — run `haft overseer maintain` or make a commit (post-commit hook).")
		return err
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), rendered); err != nil {
		return err
	}
	if summary.LatestMaintenanceID != "" {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "\nLatest maintenance run: %s (details: `haft overseer maintain --json` re-runs; this view is read-only)\n", summary.LatestMaintenanceID)
		return err
	}
	return nil
}

// runOverseerUndo restores the prior baseline recorded in a maintenance
// ledger entry — the one-step undo every autonomous mutation must carry.
func runOverseerUndo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	projectRoot, store, closeStore, err := openOverseerProjectStore()
	if err != nil {
		return err
	}
	defer closeStore()

	maintenanceID, actionID := args[0], args[1]
	run, err := overseer.LoadMaintenanceRun(projectRoot, maintenanceID)
	if err != nil {
		return fmt.Errorf("load maintenance run %s: %w", maintenanceID, err)
	}

	for _, action := range run.Executed {
		if action.ID != actionID {
			continue
		}
		if action.PriorState == "" {
			return fmt.Errorf("action %s has no prior state recorded (kind=%s, outcome=%s) — nothing to undo", actionID, action.Kind, action.Outcome)
		}
		var prior artifact.BaselineSnapshot
		if err := json.Unmarshal([]byte(action.PriorState), &prior); err != nil {
			return fmt.Errorf("decode prior state: %w", err)
		}
		if err := artifact.RestoreBaselineSnapshot(ctx, store, action.DecisionRef, prior); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Restored prior baseline for %s (%d file(s), %d symbol(s), %d manifest(s)) from %s/%s.\n",
			action.DecisionRef, len(prior.Files), len(prior.Symbols), len(prior.Manifests), maintenanceID, actionID); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("action %s not found in maintenance run %s", actionID, maintenanceID)
}

func openOverseerProjectStore() (string, *artifact.Store, func(), error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("not a haft project: %w", err)
	}

	haftDir := filepath.Join(projectRoot, ".haft")
	projCfg, err := project.Load(haftDir)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("load project config: %w", err)
	}
	if projCfg == nil {
		return "", nil, func() {}, fmt.Errorf("project not initialized — run 'haft init' first")
	}

	dbPath, err := projCfg.DBPath()
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("get DB path: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)")
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("open DB: %w", err)
	}

	closeStore := func() {
		_ = sqlDB.Close()
	}
	return projectRoot, artifact.NewStore(sqlDB), closeStore, nil
}

func buildOverseerPacket(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	commitRef string,
	version string,
) (overseer.Packet, error) {
	gitContext, err := overseer.CollectGitContext(ctx, projectRoot, commitRef)
	if err != nil {
		return overseer.Packet{}, err
	}

	workflow, err := project.LoadWorkflow(projectRoot)
	if err != nil {
		return overseer.Packet{}, fmt.Errorf("load workflow: %w", err)
	}

	checkReport, err := buildCheckReport(ctx, store, projectRoot)
	if err != nil {
		return overseer.Packet{}, fmt.Errorf("build governance check report: %w", err)
	}

	changedFiles, affectedDecisionIDs, err := enrichOverseerChangedFiles(
		ctx,
		store,
		workflow,
		gitContext.ChangedFiles,
	)
	if err != nil {
		return overseer.Packet{}, err
	}

	governance := mapOverseerGovernance(checkReport, changedFiles, affectedDecisionIDs)
	return overseer.BuildPacket(overseer.BuildInput{
		CreatedAt:    gitContext.CreatedAt,
		Producer:     overseer.DefaultProducer(version),
		Subject:      gitContext.Subject,
		RepoState:    gitContext.RepoState,
		ChangedFiles: changedFiles,
		Governance:   governance,
		Budget:       overseer.DefaultContextBudget(),
	})
}

func enrichOverseerChangedFiles(
	ctx context.Context,
	store *artifact.Store,
	workflow *project.Workflow,
	changedFiles []overseer.ChangedFile,
) ([]overseer.ChangedFile, map[string]bool, error) {
	allAffectedFiles, err := store.AllAffectedFiles(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list affected files: %w", err)
	}

	policies := overseerPathPolicies(workflow)
	affectedDecisionIDs := make(map[string]bool)
	enriched := make([]overseer.ChangedFile, 0, len(changedFiles))

	for _, changedFile := range changedFiles {
		decisions, err := decisionsForChangedPath(ctx, store, allAffectedFiles, changedFile.Path)
		if err != nil {
			return nil, nil, err
		}

		governance := overseer.ChangedFileGovernance{
			ModuleState:          "blind",
			AffectedDecisions:    make([]overseer.ArtifactRef, 0, len(decisions)),
			AffectedSpecSections: make([]string, 0),
			AffectedInvariants:   make([]overseer.InvariantRef, 0),
			PathPolicies:         overseer.MatchPathPolicies(changedFile.Path, policies),
		}

		if len(decisions) > 0 {
			governance.ModuleState = "covered"
		}

		for _, decision := range decisions {
			affectedDecisionIDs[decision.Meta.ID] = true
			fields := decodeDecisionFields(decision)
			governance.AffectedDecisions = append(governance.AffectedDecisions, overseer.ArtifactRef{
				ID:    decision.Meta.ID,
				Title: decision.Meta.Title,
			})
			governance.AffectedSpecSections = append(governance.AffectedSpecSections, fields.SectionRefs...)
			for index, invariant := range fields.Invariants {
				governance.AffectedInvariants = append(governance.AffectedInvariants, overseer.InvariantRef{
					ID:        fmt.Sprintf("%s#invariant-%d", decision.Meta.ID, index+1),
					Text:      invariant,
					SourceRef: decision.Meta.ID,
				})
			}
		}

		changedFile.Governance = governance
		enriched = append(enriched, changedFile)
	}

	return enriched, affectedDecisionIDs, nil
}

func decisionsForChangedPath(
	ctx context.Context,
	store *artifact.Store,
	allAffectedFiles []artifact.AffectedFileRef,
	changedPath string,
) ([]*artifact.Artifact, error) {
	byID := make(map[string]*artifact.Artifact)

	exactMatches, err := store.SearchByAffectedFile(ctx, changedPath)
	if err != nil {
		return nil, fmt.Errorf("search affected file %s: %w", changedPath, err)
	}

	for _, match := range exactMatches {
		decision, err := loadScopedDecision(ctx, store, match.Meta.ID)
		if err != nil {
			return nil, err
		}
		if decision != nil {
			byID[decision.Meta.ID] = decision
		}
	}

	for _, ref := range allAffectedFiles {
		if ref.FilePath == changedPath {
			continue
		}

		decision, err := loadScopedDecision(ctx, store, ref.ArtifactID)
		if err != nil {
			return nil, err
		}
		if decision == nil {
			continue
		}
		fields := decodeDecisionFields(decision)
		if fields.EffectiveGovernanceMode() != artifact.GovernanceModeModule {
			continue
		}
		if !sameGovernedModule(ref.FilePath, changedPath) {
			continue
		}
		byID[decision.Meta.ID] = decision
	}

	decisions := make([]*artifact.Artifact, 0, len(byID))
	for _, decision := range byID {
		decisions = append(decisions, decision)
	}

	sort.Slice(decisions, func(i, j int) bool {
		return decisions[i].Meta.ID < decisions[j].Meta.ID
	})

	return decisions, nil
}

func loadScopedDecision(ctx context.Context, store *artifact.Store, id string) (*artifact.Artifact, error) {
	item, err := store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load affected artifact %s: %w", id, err)
	}
	if item.Meta.Kind != artifact.KindDecisionRecord {
		return nil, nil
	}
	if item.Meta.Status != artifact.StatusActive && item.Meta.Status != artifact.StatusRefreshDue {
		return nil, nil
	}
	return item, nil
}

func decodeDecisionFields(decision *artifact.Artifact) artifact.DecisionFields {
	var fields artifact.DecisionFields
	if strings.TrimSpace(decision.StructuredData) == "" {
		return fields
	}
	_ = json.Unmarshal([]byte(decision.StructuredData), &fields)
	return fields
}

func sameGovernedModule(affectedPath string, changedPath string) bool {
	affected := strings.Trim(affectedPath, "/")
	changed := strings.Trim(changedPath, "/")
	if affected == changed {
		return true
	}

	dir := filepath.Dir(affected)
	if dir == "." || dir == "" {
		return false
	}
	return strings.HasPrefix(changed, dir+"/")
}

func overseerPathPolicies(workflow *project.Workflow) []overseer.PathPolicy {
	if workflow == nil {
		return nil
	}

	policies := make([]overseer.PathPolicy, 0, len(workflow.PathPolicies))
	for _, policy := range workflow.PathPolicies {
		policies = append(policies, overseer.PathPolicy{Path: policy.Path})
	}
	return policies
}

func mapOverseerGovernance(
	report checkReport,
	changedFiles []overseer.ChangedFile,
	affectedDecisionIDs map[string]bool,
) overseer.GovernanceInput {
	changedPathSet := changedPaths(changedFiles)
	affectedSpecSectionIDs := changedSpecSectionIDs(changedFiles)

	stale, unrelatedStale := mapScopedStale(report.Stale, affectedDecisionIDs)
	drift, unrelatedDrift := mapScopedDrift(report.Drifted, changedPathSet, affectedDecisionIDs)
	specHealth, unrelatedSpecHealth := mapScopedSpecHealth(report.SpecHealth, affectedSpecSectionIDs)
	coverage, unrelatedCoverage := mapScopedCoverage(report.CoverageGaps, affectedDecisionIDs)

	return overseer.GovernanceInput{
		Stale:        stale,
		Drift:        drift,
		SpecHealth:   specHealth,
		CoverageGaps: coverage,
		Suppressed: overseer.SuppressedDebt{
			UnrelatedStale:        unrelatedStale,
			UnrelatedDrift:        unrelatedDrift,
			UnrelatedSpecHealth:   unrelatedSpecHealth,
			UnrelatedCoverageGaps: unrelatedCoverage,
		},
	}
}

func mapScopedStale(
	findings []checkStaleFinding,
	affectedDecisionIDs map[string]bool,
) ([]overseer.FindingSummary, int) {
	out := make([]overseer.FindingSummary, 0, len(findings))
	suppressed := 0

	for _, finding := range findings {
		if !affectedDecisionIDs[finding.ID] {
			suppressed++
			continue
		}

		out = append(out, overseer.FindingSummary{
			ID:       finding.ID,
			Title:    finding.Title,
			Kind:     finding.Kind,
			Category: finding.Category,
			Reason:   finding.Reason,
		})
	}

	return out, suppressed
}

func mapScopedDrift(
	findings []checkDriftFinding,
	changedPathSet map[string]bool,
	affectedDecisionIDs map[string]bool,
) ([]overseer.FindingSummary, int) {
	out := make([]overseer.FindingSummary, 0, len(findings))
	suppressed := 0

	for _, finding := range findings {
		paths := driftPathsInChangedScope(finding, changedPathSet)
		if !affectedDecisionIDs[finding.DecisionID] && len(paths) == 0 {
			suppressed++
			continue
		}

		out = append(out, overseer.FindingSummary{
			ID:     finding.DecisionID,
			Title:  finding.DecisionTitle,
			Kind:   "DecisionRecord",
			Reason: finding.Summary,
			Paths:  paths,
		})
	}

	return out, suppressed
}

func mapScopedSpecHealth(
	findings []project.SpecCheckFinding,
	affectedSpecSectionIDs map[string]bool,
) ([]overseer.FindingSummary, int) {
	out := make([]overseer.FindingSummary, 0, len(findings))
	suppressed := 0

	for _, finding := range findings {
		sectionID := strings.TrimSpace(finding.SectionID)
		if sectionID == "" || !affectedSpecSectionIDs[sectionID] {
			suppressed++
			continue
		}

		out = append(out, overseer.FindingSummary{
			ID:       sectionID,
			Kind:     "SpecSection",
			Category: finding.Code,
			Reason:   finding.Message,
			Paths:    []string{finding.Path},
		})
	}

	return out, suppressed
}

func mapScopedCoverage(
	findings []checkCoverageGapFinding,
	affectedDecisionIDs map[string]bool,
) ([]overseer.FindingSummary, int) {
	out := make([]overseer.FindingSummary, 0, len(findings))
	suppressed := 0

	for _, finding := range findings {
		if !affectedDecisionIDs[finding.DecisionID] {
			suppressed++
			continue
		}

		out = append(out, overseer.FindingSummary{
			ID:     finding.DecisionID,
			Title:  finding.Title,
			Kind:   "DecisionRecord",
			Reason: strings.Join(finding.Gaps, "; "),
		})
	}

	return out, suppressed
}

func mapMaintenanceStale(findings []checkStaleFinding) []overseer.FindingSummary {
	out := make([]overseer.FindingSummary, 0, len(findings))
	for _, finding := range findings {
		out = append(out, overseer.FindingSummary{
			ID:       finding.ID,
			Title:    finding.Title,
			Kind:     finding.Kind,
			Category: finding.Category,
			Reason:   finding.Reason,
		})
	}
	return out
}

func changedSpecSectionIDs(changedFiles []overseer.ChangedFile) map[string]bool {
	ids := make(map[string]bool)
	for _, changedFile := range changedFiles {
		for _, sectionID := range changedFile.Governance.AffectedSpecSections {
			sectionID = strings.TrimSpace(sectionID)
			if sectionID == "" {
				continue
			}
			ids[sectionID] = true
		}
	}
	return ids
}

func mapMaintenanceDrift(findings []checkDriftFinding) []overseer.MaintenanceDriftFinding {
	reports := make([]artifact.DriftReport, 0, len(findings))
	for _, finding := range findings {
		reports = append(reports, artifact.DriftReport{
			DecisionID:        finding.DecisionID,
			DecisionTitle:     finding.DecisionTitle,
			HasBaseline:       finding.HasBaseline,
			LikelyImplemented: finding.LikelyImplemented,
			Files:             finding.Files,
		})
	}

	dispositions := artifact.ClassifyAutoBaseline(reports)
	out := make([]overseer.MaintenanceDriftFinding, 0, len(dispositions))
	for _, disposition := range dispositions {
		out = append(out, overseer.MaintenanceDriftFinding{
			ID:            disposition.Report.DecisionID,
			Title:         disposition.Report.DecisionTitle,
			Summary:       summarizeCheckDrift(disposition.Report),
			Paths:         driftReportPaths(disposition.Report),
			HasBaseline:   disposition.Report.HasBaseline,
			SymbolVerdict: disposition.Report.SymbolVerdict(),
			Materiality:   string(disposition.Report.EffectiveMateriality()),
			Action:        string(disposition.Action),
			Reason:        disposition.Reason,
		})
	}
	return out
}

func mapMaintenanceSpecHealth(findings []project.SpecCheckFinding) []overseer.FindingSummary {
	out := make([]overseer.FindingSummary, 0, len(findings))
	for _, finding := range findings {
		out = append(out, overseer.FindingSummary{
			ID:       finding.Code,
			Title:    finding.Code,
			Kind:     "SpecCheckFinding",
			Category: finding.Level,
			Reason:   finding.Message,
			Paths:    []string{finding.Path},
		})
	}
	return out
}

func mapMaintenanceCoverage(findings []checkCoverageGapFinding) []overseer.FindingSummary {
	out := make([]overseer.FindingSummary, 0, len(findings))
	for _, finding := range findings {
		out = append(out, overseer.FindingSummary{
			ID:     finding.DecisionID,
			Title:  finding.Title,
			Kind:   "DecisionRecord",
			Reason: strings.Join(finding.Gaps, "; "),
		})
	}
	return out
}

func driftReportPaths(report artifact.DriftReport) []string {
	paths := make([]string, 0, len(report.Files))
	for _, file := range report.Files {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	return paths
}

func changedPaths(files []overseer.ChangedFile) map[string]bool {
	paths := make(map[string]bool)
	for _, file := range files {
		paths[file.Path] = true
	}
	return paths
}

func driftPathsInChangedScope(finding checkDriftFinding, changedPathSet map[string]bool) []string {
	paths := make([]string, 0)
	for _, file := range finding.Files {
		if changedPathSet[file.Path] {
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func writeOverseerPacketSummary(output io.Writer, packet overseer.Packet) error {
	_, err := fmt.Fprintf(
		output,
		"haft overseer: %s risk packet generated: %s\nchanged files: %d\nreview modes: %s\n",
		packet.Risk.Level,
		packet.PacketID,
		len(packet.ChangedFiles),
		strings.Join(packet.ReviewRequest.Modes, ", "),
	)
	return err
}

func writeOverseerPacketJSON(output io.Writer, packet overseer.Packet) error {
	encoded, err := json.Marshal(packet)
	if err != nil {
		return err
	}
	_, err = output.Write(encoded)
	return err
}

func writeOverseerInitSummary(
	output io.Writer,
	config overseer.Config,
	result overseer.HookInstallResult,
) error {
	hookState := "installed"
	if result.Skipped {
		hookState = "skipped: " + result.Reason
	} else if result.Updated {
		hookState = "updated"
	}
	_, err := fmt.Fprintf(
		output,
		"haft overseer init: reviewer=%s llm_review=%s review_on_hook=%t\nhook: %s\nconfig: .haft/overseer/config.yaml\n",
		config.ReviewerAgent,
		config.LLMReview,
		config.ReviewOnHook,
		hookState,
	)
	return err
}

func writeOverseerRunJSON(output io.Writer, stored overseer.StoredRun) error {
	encoded, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	_, err = output.Write(encoded)
	return err
}

func writeOverseerRunSummary(output io.Writer, stored overseer.StoredRun) error {
	modes := strings.Join(stored.Packet.ReviewRequest.Modes, ", ")
	if modes == "" {
		modes = "(none)"
	}
	unresolved := len(overseer.UnresolvedFindings(stored.Run))
	_, err := fmt.Fprintf(
		output,
		"haft overseer: stored %s (%s risk, %s)\npacket: %s\nchanged files: %d\nreview modes: %s\nfindings: %d unresolved: %d\n",
		stored.Run.ReviewRunID,
		stored.Packet.Risk.Level,
		stored.Run.Verdict,
		stored.Packet.PacketID,
		len(stored.Packet.ChangedFiles),
		modes,
		len(stored.Run.Findings),
		unresolved,
	)
	return err
}

func writeOverseerMaintenanceSummary(output io.Writer, run overseer.MaintenanceRun) error {
	_, err := fmt.Fprintf(
		output,
		"haft overseer: maintenance stored %s (%s)\nsignals: %d\nsuppressed: %d\nauto-resolvable drift: %d\n",
		run.MaintenanceID,
		run.Verdict,
		run.Summary.SignalCount,
		run.Summary.SuppressedCount,
		run.Summary.AutoResolvableDrift,
	)
	return err
}

func writeOverseerReviewSummary(
	output io.Writer,
	stored overseer.StoredRun,
	result overseer.ReviewerRunResult,
) error {
	_, err := fmt.Fprintf(
		output,
		"haft overseer: reviewer ingested %s\nprompt: %s\nresult: %s\nfindings: %d\nunresolved: %d\n",
		stored.Run.ReviewRunID,
		result.PromptPath,
		result.ResultPath,
		len(stored.Run.Findings),
		len(overseer.UnresolvedFindings(stored.Run)),
	)
	return err
}

func writeOverseerDaemonStartSummary(output io.Writer, result overseerDaemonStartResult) error {
	state := "already running"
	if result.Started {
		state = "started"
	}
	_, err := fmt.Fprintf(
		output,
		"haft overseer daemon: %s pid=%d\nlog: %s\n",
		state,
		result.PID,
		displayProjectRelativePath(result.LogPath),
	)
	return err
}

func writeOverseerDaemonStatusSummary(output io.Writer, status overseer.DaemonStatus) error {
	state := "stopped"
	if status.Running {
		state = fmt.Sprintf("running pid=%d", status.PID)
	}
	if status.Stale {
		state = fmt.Sprintf("stale pid=%d", status.PID)
	}

	_, err := fmt.Fprintf(
		output,
		"haft overseer daemon: %s\nqueue: pending=%d running=%d done=%d failed=%d total=%d\n",
		state,
		status.Queue.Pending,
		status.Queue.Running,
		status.Queue.Done,
		status.Queue.Failed,
		status.Queue.Total,
	)
	return err
}

func writeOverseerDaemonStopSummary(output io.Writer, status overseer.DaemonStatus) error {
	_, err := fmt.Fprintf(
		output,
		"haft overseer daemon: stop signal sent\nrunning: %t\n",
		status.Running,
	)
	return err
}

func writeOverseerHookStart(output io.Writer, config overseer.Config) error {
	reviewState := "deterministic packet only"
	if overseer.ReviewHookEnabled(config) {
		reviewState = fmt.Sprintf("reviewer=%s timeout=%ds", config.ReviewerAgent, config.ReviewTimeoutSeconds)
	}

	_, err := fmt.Fprintf(
		output,
		"haft overseer: hook running (%s)\n",
		reviewState,
	)
	return err
}

func writeOverseerHookAsyncSummary(
	output io.Writer,
	stored overseer.StoredRun,
	maintenance overseer.MaintenanceRun,
	schedule overseerReviewSchedule,
) error {
	reviewState := "queued"
	if schedule.Queued {
		reviewState = fmt.Sprintf("queued job=%s daemon_pid=%d", schedule.JobID, schedule.DaemonPID)
	}
	if strings.TrimSpace(schedule.Skipped) != "" {
		reviewState = "skipped: " + schedule.Skipped
	}
	if strings.TrimSpace(schedule.Error) != "" {
		reviewState = "queued but daemon start failed: " + schedule.Error
	}

	_, err := fmt.Fprintf(
		output,
		"haft overseer: hook queued %s (%s risk)\nmaintenance: %s signals=%d suppressed=%d\nreview: %s\njob_log: %s\ndaemon_log: %s\n",
		stored.Run.ReviewRunID,
		stored.Packet.Risk.Level,
		maintenance.MaintenanceID,
		maintenance.Summary.SignalCount,
		maintenance.Summary.SuppressedCount,
		reviewState,
		displayOverseerLogPath(stored, schedule),
		displayProjectRelativePath(schedule.DaemonLog),
	)
	return err
}

func writeOverseerHookSummary(
	output io.Writer,
	stored overseer.StoredRun,
	maintenance overseer.MaintenanceRun,
	reviewErr error,
) error {
	reviewState := "skipped"
	if stored.Run.Verdict == "review_abstained" {
		reviewState = "abstained"
	}
	if stored.Run.Verdict == "findings_recorded" || stored.Run.Verdict == "reviewed_no_findings" {
		reviewState = "reviewed"
	}
	if reviewErr != nil {
		reviewState = "abstained: " + reviewErr.Error()
	}

	_, err := fmt.Fprintf(
		output,
		"haft overseer: hook stored %s (%s risk)\nmaintenance: %s signals=%d suppressed=%d\nreview: %s\nunresolved: %d\n",
		stored.Run.ReviewRunID,
		stored.Packet.Risk.Level,
		maintenance.MaintenanceID,
		maintenance.Summary.SignalCount,
		maintenance.Summary.SuppressedCount,
		reviewState,
		len(overseer.UnresolvedFindings(stored.Run)),
	)
	return err
}

func displayOverseerLogPath(stored overseer.StoredRun, schedule overseerReviewSchedule) string {
	if strings.TrimSpace(schedule.LogPath) == "" {
		return ".haft/overseer/logs/review-" + stored.Run.ReviewRunID + ".log"
	}
	return displayProjectRelativePath(schedule.LogPath)
}

func displayProjectRelativePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	projectRoot, err := findProjectRoot()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return path
	}
	return rel
}

func reviewErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeOverseerIngestSummary(output io.Writer, stored overseer.StoredRun) error {
	_, err := fmt.Fprintf(
		output,
		"haft overseer: ingested review for %s\nfindings: %d\nunresolved: %d\n",
		stored.Run.ReviewRunID,
		len(stored.Run.Findings),
		len(overseer.UnresolvedFindings(stored.Run)),
	)
	return err
}

func writeOverseerDispositionSummary(
	output io.Writer,
	stored overseer.StoredRun,
	findingID string,
	status string,
) error {
	_, err := fmt.Fprintf(
		output,
		"haft overseer: disposition recorded for %s (%s)\nrun: %s\nunresolved: %d\n",
		findingID,
		status,
		stored.Run.ReviewRunID,
		len(overseer.UnresolvedFindings(stored.Run)),
	)
	return err
}
