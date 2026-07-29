package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var (
	commissionJSONPath                     string
	commissionRunnerID                     string
	commissionFromDecisionRepoRef          string
	commissionFromDecisionBaseSHA          string
	commissionFromDecisionTargetBranch     string
	commissionFromDecisionAllowedPaths     []string
	commissionFromDecisionForbiddenPaths   []string
	commissionFromDecisionAllowedActions   []string
	commissionFromDecisionAffectedFiles    []string
	commissionFromDecisionAllowedModules   []string
	commissionFromDecisionLockset          []string
	commissionFromDecisionEvidence         []string
	commissionFromDecisionProjectionPolicy string
	commissionFromDecisionDeliveryPolicy   string
	commissionFromDecisionState            string
	commissionFromDecisionValidFor         string
	commissionFromDecisionValidUntil       string
	commissionFromDecisionSliceDescription string
	commissionScopeID                      string
	commissionLifecycleEvent               string
	commissionLifecycleVerdict             string
	commissionLifecycleReason              string
	commissionLifecyclePayload             string
	commissionLifecyclePayloadFile         string
)

var commissionCmd = &cobra.Command{
	Use:   "commission",
	Short: "Manage WorkCommissions for execution harnesses",
	Long: `Manage WorkCommissions for execution harnesses.

WorkCommission is the bounded authorization boundary between a DecisionRecord
and a runtime such as Open-Sleigh. Normal operator lifecycle actions inspect,
requeue, or cancel the record; they do not physically delete it.`,
}

var commissionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a WorkCommission from a JSON payload",
	RunE:  runCommissionCreate,
}

var commissionCreateFromDecisionCmd = &cobra.Command{
	Use:   "create-from-decision <decision-id>",
	Short: "Create a runnable WorkCommission from an active DecisionRecord",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionCreateFromDecision,
}

var commissionCreateBatchCmd = &cobra.Command{
	Use:   "create-batch <decision-id>...",
	Short: "Create runnable WorkCommissions from active DecisionRecords",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCommissionCreateBatch,
}

var commissionCreateFromPlanCmd = &cobra.Command{
	Use:   "create-from-plan <plan-file>",
	Short: "Create runnable WorkCommissions from an ImplementationPlan file",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionCreateFromPlan,
}

var commissionListRunnableCmd = &cobra.Command{
	Use:   "list-runnable",
	Short: "List queued or ready dependency-satisfied WorkCommissions",
	RunE:  runCommissionListRunnable,
}

var commissionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List WorkCommissions by lifecycle selector",
	RunE:  runCommissionList,
}

var commissionShowCmd = &cobra.Command{
	Use:   "show <commission-id>",
	Short: "Show one WorkCommission",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionShow,
}

var commissionClaimCmd = &cobra.Command{
	Use:   "claim <commission-id>",
	Short: "Claim a WorkCommission for preflight",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionClaim,
}

var commissionRequeueCmd = &cobra.Command{
	Use:   "requeue <commission-id>",
	Short: "Return a recoverable WorkCommission to the runnable queue",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionRequeue,
}

var commissionCancelCmd = &cobra.Command{
	Use:   "cancel <commission-id>",
	Short: "Cancel an unfinished WorkCommission without deleting its audit record",
	Args:  cobra.ExactArgs(1),
	RunE:  runCommissionCancel,
}

var commissionCompleteExternalCmd = &cobra.Command{
	Use:   "complete-external <commission-id>",
	Short: "Record terminal success/failure for an externally-run WorkCommission",
	Long: `Record terminal success/failure for an externally-run WorkCommission.

This is the operator path for external runners that already wrote local runtime
evidence outside Haft Harness. If the commission is still preflighting, this
command first records a passed preflight/start event, then records the terminal
complete_or_block event. It does not apply, merge, or publish any workspace diff.`,
	Args: cobra.ExactArgs(1),
	RunE: runCommissionCompleteExternal,
}

func init() {
	commissionCreateCmd.Flags().StringVar(&commissionJSONPath, "json", "", "JSON payload path, or '-' for stdin")
	commissionCreateCmd.Flags().StringVar(
		&commissionScopeID,
		"scope-id",
		"",
		"Exact admitted ScopeID when the canonical project profile has multiple scopes",
	)
	registerCommissionFromDecisionFlags(commissionCreateFromDecisionCmd)
	registerCommissionFromDecisionFlags(commissionCreateBatchCmd)
	registerCommissionFromDecisionFlags(commissionCreateFromPlanCmd)
	commissionListCmd.Flags().String("selector", "open", "commission selector: open, stale, terminal, runnable, all")
	commissionListCmd.Flags().String("state", "", "exact WorkCommission state filter")
	commissionListCmd.Flags().String("older-than", "", "duration threshold for stale open commissions, e.g. 24h")
	commissionClaimCmd.Flags().StringVar(&commissionRunnerID, "runner", "haft-cli", "runner id for the lease")
	commissionRequeueCmd.Flags().StringVar(&commissionRunnerID, "runner", "haft-cli", "runner id for the recovery event")
	commissionRequeueCmd.Flags().String("reason", "operator_requested_requeue", "reason recorded on the recovery event")
	commissionCancelCmd.Flags().StringVar(&commissionRunnerID, "runner", "haft-cli", "runner id for the cancellation event")
	commissionCancelCmd.Flags().String("reason", "operator_cancelled", "reason recorded on the cancellation event")
	commissionCompleteExternalCmd.Flags().StringVar(&commissionRunnerID, "runner", "haft-cli", "runner id for the lifecycle events")
	commissionCompleteExternalCmd.Flags().StringVar(&commissionLifecycleVerdict, "verdict", "completed", "terminal verdict: completed, pass, failed, or blocked")
	commissionCompleteExternalCmd.Flags().StringVar(&commissionLifecycleReason, "reason", "external_runtime_completed", "reason recorded on lifecycle events")
	commissionCompleteExternalCmd.Flags().StringVar(&commissionLifecycleEvent, "event", "external_runtime_terminal", "terminal event name recorded on completion")
	commissionCompleteExternalCmd.Flags().StringVar(&commissionLifecyclePayload, "payload", "", "JSON object payload, or @path to read JSON from a file")
	commissionCompleteExternalCmd.Flags().StringVar(&commissionLifecyclePayloadFile, "payload-file", "", "JSON object payload file")

	commissionCmd.AddCommand(commissionCreateCmd)
	commissionCmd.AddCommand(commissionCreateFromDecisionCmd)
	commissionCmd.AddCommand(commissionCreateBatchCmd)
	commissionCmd.AddCommand(commissionCreateFromPlanCmd)
	commissionCmd.AddCommand(commissionListCmd)
	commissionCmd.AddCommand(commissionListRunnableCmd)
	commissionCmd.AddCommand(commissionShowCmd)
	commissionCmd.AddCommand(commissionClaimCmd)
	commissionCmd.AddCommand(commissionRequeueCmd)
	commissionCmd.AddCommand(commissionCancelCmd)
	commissionCmd.AddCommand(commissionCompleteExternalCmd)
	rootCmd.AddCommand(commissionCmd)
}

func registerCommissionFromDecisionFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&commissionFromDecisionRepoRef, "repo-ref", "", "repo ref recorded in commission scope (default: local:<project-dir>)")
	cmd.Flags().StringVar(&commissionFromDecisionBaseSHA, "base-sha", "", "git base SHA for the commission scope (default: current HEAD)")
	cmd.Flags().StringVar(&commissionFromDecisionTargetBranch, "target-branch", "", "target branch for runner work (default: current branch)")
	cmd.Flags().StringSliceVar(&commissionFromDecisionAllowedPaths, "allowed-path", nil, "path the runner may edit; repeatable (default: decision affected_files)")
	cmd.Flags().StringSliceVar(&commissionFromDecisionForbiddenPaths, "forbidden-path", nil, "path the runner must not edit; repeatable")
	cmd.Flags().StringSliceVar(&commissionFromDecisionAllowedActions, "allowed-action", []string{"edit_files", "run_tests"}, "allowed runner action; repeatable")
	cmd.Flags().StringSliceVar(&commissionFromDecisionAffectedFiles, "affected-file", nil, "affected file/path for commission scope; repeatable (default: allowed paths)")
	cmd.Flags().StringSliceVar(&commissionFromDecisionAllowedModules, "allowed-module", nil, "allowed module name/path; repeatable")
	cmd.Flags().StringSliceVar(&commissionFromDecisionLockset, "lock", nil, "lockset path/pattern; repeatable (default: affected files)")
	cmd.Flags().StringSliceVar(&commissionFromDecisionEvidence, "evidence", nil, "required evidence command; repeatable (default: decision evidence_requirements)")
	cmd.Flags().StringVar(&commissionFromDecisionProjectionPolicy, "projection-policy", "local_only", "projection policy: local_only, external_optional, external_required")
	cmd.Flags().StringVar(&commissionFromDecisionDeliveryPolicy, "delivery-policy", defaultDeliveryPolicy, "delivery policy: workspace_patch_manual, workspace_patch_auto_on_pass")
	cmd.Flags().StringVar(&commissionFromDecisionState, "state", "queued", "initial commission state")
	cmd.Flags().StringVar(&commissionFromDecisionValidFor, "valid-for", "168h", "commission validity duration when --valid-until is omitted")
	cmd.Flags().StringVar(&commissionFromDecisionValidUntil, "valid-until", "", "explicit commission expiry timestamp (RFC3339)")
	cmd.Flags().StringVar(&commissionFromDecisionSliceDescription, "slice-description", "", "names which slice of the decision THIS commission implements; REQUIRED when the decision already has a non-terminal commission. Multi-commission decisions without per-slice scope text leak the decision body to every codex session.")
	cmd.Flags().StringVar(
		&commissionScopeID,
		"scope-id",
		"",
		"Exact admitted ScopeID when the canonical project profile has multiple scopes",
	)
}

func runCommissionCreate(cmd *cobra.Command, _ []string) error {
	payload, err := readCommissionJSONPayload(cmd.InOrStdin(), commissionJSONPath)
	if err != nil {
		return err
	}

	return withCommissionProject(func(
		ctx context.Context,
		store *artifact.Store,
		projectRoot string,
	) error {
		args := map[string]any{
			"action":       "create",
			"commission":   payload,
			"project_root": projectRoot,
			"scope_id":     commissionScopeID,
		}
		result, err := handleHaftCommissionForProject(ctx, store, args)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionCreateFromDecision(cmd *cobra.Command, args []string) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		params, err := commissionFromDecisionCLIParams(projectRoot, args[0])
		if err != nil {
			return err
		}

		result, err := handleHaftCommissionForProject(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionCreateBatch(cmd *cobra.Command, args []string) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		params, err := commissionBatchCLIParams(projectRoot, args)
		if err != nil {
			return err
		}

		result, err := handleHaftCommissionForProject(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionCreateFromPlan(cmd *cobra.Command, args []string) error {
	plan, err := readCommissionPlanPayload(cmd.InOrStdin(), args[0])
	if err != nil {
		return err
	}

	return withCommissionProject(func(ctx context.Context, store *artifact.Store, projectRoot string) error {
		params, err := commissionFromPlanCLIParams(projectRoot, plan)
		if err != nil {
			return err
		}

		result, err := handleHaftCommissionForProject(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionListRunnable(cmd *cobra.Command, _ []string) error {
	args := map[string]any{
		"action": "list_runnable",
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, args)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionList(cmd *cobra.Command, _ []string) error {
	selector, err := cmd.Flags().GetString("selector")
	if err != nil {
		return err
	}
	state, err := cmd.Flags().GetString("state")
	if err != nil {
		return err
	}
	olderThan, err := cmd.Flags().GetString("older-than")
	if err != nil {
		return err
	}

	params := map[string]any{
		"action":     "list",
		"selector":   selector,
		"state":      state,
		"older_than": olderThan,
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionShow(cmd *cobra.Command, args []string) error {
	params := map[string]any{
		"action":        "show",
		"commission_id": args[0],
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionClaim(cmd *cobra.Command, args []string) error {
	params := map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": args[0],
		"runner_id":     commissionRunnerID,
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionRequeue(cmd *cobra.Command, args []string) error {
	reason, err := cmd.Flags().GetString("reason")
	if err != nil {
		return err
	}

	params := map[string]any{
		"action":        "requeue",
		"commission_id": args[0],
		"runner_id":     commissionRunnerID,
		"reason":        reason,
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionCancel(cmd *cobra.Command, args []string) error {
	reason, err := cmd.Flags().GetString("reason")
	if err != nil {
		return err
	}

	params := map[string]any{
		"action":        "cancel",
		"commission_id": args[0],
		"runner_id":     commissionRunnerID,
		"reason":        reason,
	}

	return withCommissionStore(func(ctx context.Context, store *artifact.Store) error {
		result, err := handleHaftCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func runCommissionCompleteExternal(cmd *cobra.Command, args []string) error {
	payload, err := commissionLifecyclePayloadFromFlags(cmd)
	if err != nil {
		return err
	}

	params := map[string]any{
		"commission_id": args[0],
		"runner_id":     commissionRunnerID,
		"event":         commissionLifecycleEvent,
		"verdict":       commissionLifecycleVerdict,
		"reason":        commissionLifecycleReason,
	}
	if payload != nil {
		params["payload"] = payload
	}

	return withCommissionProject(func(
		ctx context.Context,
		store *artifact.Store,
		projectRoot string,
	) error {
		params["project_root"] = projectRoot
		result, err := completeExternalWorkCommission(ctx, store, params)
		return writeCommissionResult(cmd, result, err)
	})
}

func completeExternalWorkCommission(ctx context.Context, store *artifact.Store, params map[string]any) (string, error) {
	commissionID := strings.TrimSpace(stringArg(params, "commission_id"))
	if commissionID == "" {
		return "", fmt.Errorf("commission_id is required")
	}

	shown, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "show",
		"commission_id": commissionID,
	})
	if err != nil {
		return "", err
	}

	var decoded map[string]map[string]any
	if err := json.Unmarshal([]byte(shown), &decoded); err != nil {
		return "", fmt.Errorf("decode WorkCommission %s: %w", commissionID, err)
	}
	commission := decoded["commission"]
	state := strings.TrimSpace(stringField(commission, "state"))

	switch state {
	case "preflighting":
		_, err := handleHaftCommissionForProject(ctx, store, map[string]any{
			"action":        "start_after_preflight",
			"commission_id": commissionID,
			"runner_id":     stringArg(params, "runner_id"),
			"event":         "external_preflight_passed",
			"verdict":       "pass",
			"reason":        stringArg(params, "reason"),
			"project_root":  stringArg(params, "project_root"),
		})
		if err != nil {
			return "", err
		}
	case "running":
		// Already started; record only the terminal lifecycle event below.
	case "completed", "completed_with_projection_debt", "failed", "blocked_policy", "blocked_stale", "blocked_conflict":
		if externalCompletionVerdictMatchesState(state, stringArg(params, "verdict")) {
			return shown, nil
		}
		return "", fmt.Errorf("commission_complete_external_already_terminal: state=%s verdict=%s", state, stringArg(params, "verdict"))
	default:
		return "", fmt.Errorf("commission_complete_external_not_allowed: state=%s allowed_states=preflighting,running", state)
	}

	completeArgs := copyStringAnyMap(params)
	completeArgs["action"] = "complete_or_block"
	return handleHaftCommission(ctx, store, completeArgs)
}

func externalCompletionVerdictMatchesState(state string, verdict string) bool {
	switch strings.TrimSpace(verdict) {
	case "pass", "completed":
		return state == "completed" || state == "completed_with_projection_debt"
	case "failed":
		return state == "failed"
	case "blocked":
		return strings.HasPrefix(state, "blocked_")
	default:
		return false
	}
}

func commissionLifecyclePayloadFromFlags(cmd *cobra.Command) (map[string]any, error) {
	payload := strings.TrimSpace(commissionLifecyclePayload)
	payloadFile := strings.TrimSpace(commissionLifecyclePayloadFile)
	if payload != "" && payloadFile != "" {
		return nil, fmt.Errorf("use either --payload or --payload-file, not both")
	}
	if strings.HasPrefix(payload, "@") {
		payloadFile = strings.TrimSpace(strings.TrimPrefix(payload, "@"))
		payload = ""
	}
	if payloadFile != "" {
		data, err := os.ReadFile(payloadFile)
		if err != nil {
			return nil, fmt.Errorf("read payload file %s: %w", payloadFile, err)
		}
		payload = string(data)
	}
	if payload == "" {
		return nil, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return nil, fmt.Errorf("parse lifecycle payload JSON: %w", err)
	}
	return decoded, nil
}

func withCommissionStore(fn func(context.Context, *artifact.Store) error) error {
	return withCommissionProject(func(ctx context.Context, store *artifact.Store, _ string) error {
		return fn(ctx, store)
	})
}

func withCommissionProject(fn func(context.Context, *artifact.Store, string) error) error {
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

	database, err := db.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	defer database.Close()

	store := artifact.NewStore(database.GetRawDB())
	return fn(context.Background(), store, projectRoot)
}

func readCommissionJSONPayload(stdin io.Reader, path string) (map[string]any, error) {
	if path == "" {
		return nil, fmt.Errorf("--json is required")
	}

	data, err := readCommissionJSONBytes(stdin, path)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse commission JSON: %w", err)
	}

	return unwrapCommissionPayload(payload), nil
}

func readCommissionJSONBytes(stdin io.Reader, path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func unwrapCommissionPayload(payload map[string]any) map[string]any {
	commission, ok := payload["commission"].(map[string]any)
	if ok {
		return commission
	}
	return payload
}

func writeCommissionResult(cmd *cobra.Command, result string, err error) error {
	if err != nil {
		return err
	}

	_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), result)
	return writeErr
}

func commissionFromDecisionCLIParams(projectRoot string, decisionRef string) (map[string]any, error) {
	base, err := commissionFromDecisionBaseParams(projectRoot)
	if err != nil {
		return nil, err
	}

	base["action"] = "create_from_decision"
	base["decision_ref"] = decisionRef

	return base, nil
}

func commissionBatchCLIParams(projectRoot string, decisionRefs []string) (map[string]any, error) {
	base, err := commissionFromDecisionBaseParams(projectRoot)
	if err != nil {
		return nil, err
	}

	base["action"] = "create_batch_from_decisions"
	base["decision_refs"] = stringsToAnySlice(decisionRefs)

	return base, nil
}

func commissionFromPlanCLIParams(projectRoot string, plan map[string]any) (map[string]any, error) {
	base, err := commissionPlanBaseParams(projectRoot, plan)
	if err != nil {
		return nil, err
	}

	base["action"] = "create_from_plan"
	base["plan"] = plan

	return base, nil
}

func commissionPlanBaseParams(projectRoot string, plan map[string]any) (map[string]any, error) {
	scopeID := strings.TrimSpace(commissionScopeID)
	if scopeID == "" {
		scopeID = strings.TrimSpace(stringField(plan, "scope_id"))
	}
	repoRef := commissionFromDecisionRepoRef
	if strings.TrimSpace(repoRef) == "" {
		repoRef = stringField(plan, "repo_ref")
	}
	if strings.TrimSpace(repoRef) == "" {
		repoRef = "local:" + filepath.Base(projectRoot)
	}

	baseSHA := commissionFromDecisionBaseSHA
	if strings.TrimSpace(baseSHA) == "" {
		baseSHA = stringField(plan, "base_sha")
	}
	if strings.TrimSpace(baseSHA) == "" {
		value, err := gitOutput(projectRoot, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("resolve --base-sha from git: %w", err)
		}
		baseSHA = value
	}

	targetBranch := commissionFromDecisionTargetBranch
	if strings.TrimSpace(targetBranch) == "" {
		targetBranch = stringField(plan, "target_branch")
	}
	if strings.TrimSpace(targetBranch) == "" {
		value, err := gitOutput(projectRoot, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("resolve --target-branch from git: %w", err)
		}
		targetBranch = value
	}

	return map[string]any{
		"project_root":          projectRoot,
		"scope_id":              scopeID,
		"repo_ref":              repoRef,
		"base_sha":              baseSHA,
		"target_branch":         targetBranch,
		"allowed_paths":         stringsToAnySlice(commissionFromDecisionAllowedPaths),
		"forbidden_paths":       stringsToAnySlice(commissionFromDecisionForbiddenPaths),
		"allowed_actions":       stringsToAnySlice(commissionFromDecisionAllowedActions),
		"affected_files":        stringsToAnySlice(commissionFromDecisionAffectedFiles),
		"allowed_modules":       stringsToAnySlice(commissionFromDecisionAllowedModules),
		"lockset":               stringsToAnySlice(commissionFromDecisionLockset),
		"evidence_requirements": stringsToAnySlice(commissionFromDecisionEvidence),
		"projection_policy":     commissionFromDecisionProjectionPolicy,
		"delivery_policy":       commissionFromDecisionDeliveryPolicy,
		"state":                 commissionFromDecisionState,
		"valid_for":             commissionFromDecisionValidFor,
		"valid_until":           commissionFromDecisionValidUntil,
		"slice_description":     commissionFromDecisionSliceDescription,
	}, nil
}

func commissionFromDecisionBaseParams(projectRoot string) (map[string]any, error) {
	repoRef := commissionFromDecisionRepoRef
	if strings.TrimSpace(repoRef) == "" {
		repoRef = "local:" + filepath.Base(projectRoot)
	}

	baseSHA := commissionFromDecisionBaseSHA
	if strings.TrimSpace(baseSHA) == "" {
		value, err := gitOutput(projectRoot, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("resolve --base-sha from git: %w", err)
		}
		baseSHA = value
	}

	targetBranch := commissionFromDecisionTargetBranch
	if strings.TrimSpace(targetBranch) == "" {
		value, err := gitOutput(projectRoot, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("resolve --target-branch from git: %w", err)
		}
		targetBranch = value
	}

	return map[string]any{
		"project_root":          projectRoot,
		"scope_id":              strings.TrimSpace(commissionScopeID),
		"repo_ref":              repoRef,
		"base_sha":              baseSHA,
		"target_branch":         targetBranch,
		"allowed_paths":         stringsToAnySlice(commissionFromDecisionAllowedPaths),
		"forbidden_paths":       stringsToAnySlice(commissionFromDecisionForbiddenPaths),
		"allowed_actions":       stringsToAnySlice(commissionFromDecisionAllowedActions),
		"affected_files":        stringsToAnySlice(commissionFromDecisionAffectedFiles),
		"allowed_modules":       stringsToAnySlice(commissionFromDecisionAllowedModules),
		"lockset":               stringsToAnySlice(commissionFromDecisionLockset),
		"evidence_requirements": stringsToAnySlice(commissionFromDecisionEvidence),
		"projection_policy":     commissionFromDecisionProjectionPolicy,
		"delivery_policy":       commissionFromDecisionDeliveryPolicy,
		"state":                 commissionFromDecisionState,
		"valid_for":             commissionFromDecisionValidFor,
		"valid_until":           commissionFromDecisionValidUntil,
		"slice_description":     commissionFromDecisionSliceDescription,
	}, nil
}

func gitOutput(projectRoot string, args ...string) (string, error) {
	gitArgs := append([]string{"-C", projectRoot}, args...)
	output, err := exec.Command("git", gitArgs...).Output()
	if err != nil {
		return "", err
	}

	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("empty git output")
	}
	return value, nil
}

func stringsToAnySlice(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}
