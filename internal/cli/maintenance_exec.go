package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/logger"
)

// Maintenance execute-phase (dec-20260611-overseer-maintenance-executor-b1a7a749).
//
// This is the SHELL of the maintenance loop: it consumes the kernel-compiled
// plan (artifact.BuildMaintenancePlan), enforces the config envelope at the
// mutation gate, runs allowlisted observables exec-direct (never a shell),
// and writes every act into the maintenance ledger with prior state for undo.
//
// Authority ladder (enforced here, not in prompts or spawn commands):
//
//	rung 1 — deterministic predicate (additive-only drift)  → may auto-rebaseline
//	rung 2 — allowlisted observable                         → may attach machine evidence;
//	          all-green decision_stale items may revalidate
//	rung 3 — judgment                                       → NEVER executed here
const (
	maintenanceActionRebaseline = "auto_rebaseline"
	maintenanceActionObservable = "observable_run"
	maintenanceActionRevalidate = "revalidate_stale"

	maxRebaselinesPerRun = 20
	maxObservablesPerRun = 10
	observableTimeout    = 2 * time.Minute
	revalidateExtension  = 30 * 24 * time.Hour
	machineEvidenceTTL   = 30 * 24 * time.Hour
)

// executeMaintenancePlan runs the execute-phase and returns the ledger.
// Per-item failures never abort the loop — they land as failed actions.
func executeMaintenancePlan(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	cfg overseer.Config,
) []overseer.MaintenanceAction {
	if !cfg.Enabled {
		return nil
	}

	rebaselineMode := overseer.NormalizeMaintenanceMode(cfg.MaintenanceRebaseline)
	revalidateMode := overseer.NormalizeMaintenanceMode(cfg.MaintenanceRevalidateStale)
	if rebaselineMode == overseer.MaintenanceModeOff && revalidateMode == overseer.MaintenanceModeOff {
		return nil
	}

	plan, err := artifact.BuildMaintenancePlan(ctx, store, projectRoot)
	if err != nil {
		logger.Warn().Err(err).Msg("maintenance: plan build failed — execute-phase skipped")
		return nil
	}

	var actions []overseer.MaintenanceAction
	actions = append(actions, executeRebaselines(ctx, store, projectRoot, rebaselineMode, plan)...)

	observableActions, greenByDecision := executeObservables(ctx, store, projectRoot, revalidateMode, plan)
	actions = append(actions, observableActions...)
	actions = append(actions, executeRevalidations(ctx, store, projectRoot, revalidateMode, plan, greenByDecision)...)

	for i := range actions {
		actions[i].ID = fmt.Sprintf("act-%03d", i+1)
	}
	return actions
}

// executeRebaselines acts on rung-1 dispositions: drift the conservative gate
// proved additive-only. auto → snapshot prior state, re-baseline; propose →
// ledger proposal only.
func executeRebaselines(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	mode string,
	plan *artifact.MaintenancePlan,
) []overseer.MaintenanceAction {
	if mode == overseer.MaintenanceModeOff {
		return nil
	}

	var actions []overseer.MaintenanceAction
	count := 0
	for _, task := range plan.Tasks {
		if task.Rung != artifact.RungDeterministic || task.Source != "drift" {
			continue
		}
		if count >= maxRebaselinesPerRun {
			break
		}
		count++

		action := overseer.MaintenanceAction{
			Kind:        maintenanceActionRebaseline,
			DecisionRef: task.DecisionRef,
			Title:       task.DecisionTitle,
			Rung:        task.Rung,
			Detail:      task.Reason,
		}
		if mode == overseer.MaintenanceModePropose {
			action.Outcome = "proposed"
			actions = append(actions, action)
			continue
		}

		prior, err := snapshotBaseline(ctx, store, task.DecisionRef)
		if err != nil {
			action.Outcome = "failed"
			action.Detail = "prior-state snapshot failed: " + err.Error()
			actions = append(actions, action)
			continue
		}
		action.PriorState = prior

		if _, err := artifact.Baseline(ctx, store, projectRoot, artifact.BaselineInput{DecisionRef: task.DecisionRef}); err != nil {
			action.Outcome = "failed"
			action.Detail = "re-baseline failed: " + err.Error()
			actions = append(actions, action)
			continue
		}
		action.Outcome = "applied"
		actions = append(actions, action)
	}
	return actions
}

// executeObservables runs rung-2 allowlisted commands and attaches
// provenance=machine evidence. Returns the ledger slice plus, per decision,
// whether every planned machine claim ran green (the revalidation predicate).
func executeObservables(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	mode string,
	plan *artifact.MaintenancePlan,
) ([]overseer.MaintenanceAction, map[string]bool) {
	var actions []overseer.MaintenanceAction
	greenByDecision := make(map[string]bool)

	if mode == overseer.MaintenanceModeOff {
		return actions, greenByDecision
	}

	count := 0

	tasks := executableObservableTasks(plan)
	for index, task := range tasks {
		if count >= maxObservablesPerRun {
			markUnrunObservables(greenByDecision, tasks[index:])
			break
		}
		count++

		action := overseer.MaintenanceAction{
			Kind:        maintenanceActionObservable,
			DecisionRef: task.DecisionRef,
			Title:       task.DecisionTitle,
			Rung:        task.Rung,
			Detail:      task.Command,
		}

		if mode == overseer.MaintenanceModePropose {
			action.Outcome = "proposed"
			action.Detail = fmt.Sprintf("%s — proposal only (maintenance_revalidate_stale=propose; no command executed, no evidence attached)", task.Command)
			actions = append(actions, action)
			continue
		}

		output, runErr := runAllowlistedObservable(ctx, projectRoot, task.Command)
		verdict := "supports"
		if runErr != nil {
			verdict = "weakens"
			greenByDecision[task.DecisionRef] = false
		} else if _, seen := greenByDecision[task.DecisionRef]; !seen {
			greenByDecision[task.DecisionRef] = true
		}

		content := fmt.Sprintf("Maintenance loop ran `%s` for %s (%s). Exit: %s. Output tail:\n%s",
			task.Command, task.ClaimID, task.Threshold, exitLabel(runErr), output)
		evidence, evidenceErr := artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
			ArtifactRef:        task.DecisionRef,
			Content:            content,
			Type:               "test",
			Verdict:            verdict,
			CarrierRef:         "overseer maintenance run",
			CongruenceLevel:    3,
			FormalityLevel:     -1,
			ClaimRefs:          claimRefsFor(task),
			ValidUntil:         time.Now().UTC().Add(machineEvidenceTTL).Format("2006-01-02"),
			CausalSupportBasis: "observational",
			Provenance:         artifact.ProvenanceMachine,
		})
		switch {
		case evidenceErr != nil:
			action.Outcome = "failed"
			action.Detail = fmt.Sprintf("%s — evidence attach failed: %v", task.Command, evidenceErr)
			greenByDecision[task.DecisionRef] = false
		case runErr != nil:
			action.Outcome = "evidence_attached"
			action.Detail = fmt.Sprintf("%s — observable FAILED (weakens evidence attached)", task.Command)
		default:
			action.Outcome = "evidence_attached"
			action.Detail = fmt.Sprintf("%s — green (supports evidence attached)", task.Command)
		}
		if evidence != nil && evidence.ID != "" {
			action.EvidenceRefs = []string{evidence.ID}
		}
		actions = append(actions, action)
	}
	return actions, greenByDecision
}

func executableObservableTasks(plan *artifact.MaintenancePlan) []artifact.MaintenanceTask {
	if plan == nil {
		return nil
	}

	tasks := make([]artifact.MaintenanceTask, 0, len(plan.Tasks))
	for _, task := range plan.Tasks {
		if task.Rung != artifact.RungMachine || task.Command == "" {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func markUnrunObservables(greenByDecision map[string]bool, tasks []artifact.MaintenanceTask) {
	for _, task := range tasks {
		greenByDecision[task.DecisionRef] = false
	}
}

// executeRevalidations extends valid_until on decision_stale items whose
// machine observables ALL ran green in this same run — the evidence-backed
// revalidation gate: extension without fresh machine evidence never happens
// here (that is can-kicking, manual-only).
func executeRevalidations(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	mode string,
	plan *artifact.MaintenancePlan,
	greenByDecision map[string]bool,
) []overseer.MaintenanceAction {
	if mode == overseer.MaintenanceModeOff {
		return nil
	}

	staleDecisions := revalidatableStaleDecisions(plan, greenByDecision)

	var actions []overseer.MaintenanceAction
	for ref, task := range staleDecisions {
		action := overseer.MaintenanceAction{
			Kind:        maintenanceActionRevalidate,
			DecisionRef: ref,
			Title:       task.DecisionTitle,
			Rung:        artifact.RungMachine,
		}
		if mode == overseer.MaintenanceModePropose {
			action.Outcome = "proposed"
			action.Detail = "all machine observables green this run — extension proposed"
			actions = append(actions, action)
			continue
		}

		newValidUntil := time.Now().UTC().Add(revalidateExtension).Format("2006-01-02")
		haftDir := filepath.Join(projectRoot, ".haft")
		_, err := artifact.WaiveArtifact(ctx, store, haftDir, ref,
			"maintenance loop: all machine-checkable observables re-ran green in this run (provenance=machine evidence attached)",
			newValidUntil,
			"fresh machine evidence attached by the same maintenance run; see ledger")
		if err != nil {
			action.Outcome = "failed"
			action.Detail = "revalidation failed: " + err.Error()
			actions = append(actions, action)
			continue
		}
		action.Outcome = "applied"
		action.Detail = "valid_until extended to " + newValidUntil + " on green machine evidence"
		actions = append(actions, action)
	}
	return actions
}

func revalidatableStaleDecisions(
	plan *artifact.MaintenancePlan,
	greenByDecision map[string]bool,
) map[string]artifact.MaintenanceTask {
	candidates := make(map[string]artifact.MaintenanceTask)
	blocked := make(map[string]bool)
	if plan == nil {
		return candidates
	}

	for _, task := range plan.Tasks {
		if !isEvidenceExpiredStaleTask(task) {
			continue
		}
		candidates[task.DecisionRef] = task
		if task.Rung != artifact.RungMachine || task.Command == "" {
			blocked[task.DecisionRef] = true
		}
	}

	for ref := range candidates {
		if blocked[ref] || !greenByDecision[ref] {
			delete(candidates, ref)
		}
	}
	return candidates
}

func isEvidenceExpiredStaleTask(task artifact.MaintenanceTask) bool {
	if task.Source != "stale" {
		return false
	}
	return task.Category == string(artifact.StaleCategoryEvidenceExpired)
}

// runAllowlistedObservable executes a stored observable command exec-direct:
// the allowlist is re-checked at the last moment (defense in depth), argv is
// field-split (no shell, no expansion), and the run is time-bounded.
func runAllowlistedObservable(ctx context.Context, projectRoot, command string) (string, error) {
	if _, ok := artifact.ClassifyCommand(command); !ok {
		return "", fmt.Errorf("command not allowlisted: %q", command)
	}
	fields := strings.Fields(command)

	cctx, cancel := context.WithTimeout(ctx, observableTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, fields[0], fields[1:]...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	return tailString(string(out), 1500), err
}

func snapshotBaseline(ctx context.Context, store *artifact.Store, decisionRef string) (string, error) {
	snapshot, err := artifact.CaptureBaselineSnapshot(ctx, store, decisionRef)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func claimRefsFor(task artifact.MaintenanceTask) []string {
	if task.ClaimID == "" {
		return nil
	}
	return []string{task.ClaimID}
}

func exitLabel(err error) string {
	if err == nil {
		return "0 (green)"
	}
	return err.Error()
}

func tailString(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return "…" + s[len(s)-limit:]
}
