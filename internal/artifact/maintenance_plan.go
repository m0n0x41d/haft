package artifact

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

// Maintenance loop core (dec-20260611-overseer-maintenance-executor-b1a7a749).
//
// This file is the PURE side of the maintenance executor: evidence provenance
// vocabulary, the allowlist command classifier, the mutation-ladder rungs, and
// the work-order compiler (BuildMaintenancePlan). Executing commands, mutating
// baselines, and config gating live in the shell (internal/cli) — this file
// never runs anything and never writes.

// Evidence provenance values. Machine-collected evidence must always be
// distinguishable from human-reviewed evidence (decision invariant).
const (
	ProvenanceHuman     = ""           // collected by a human/agent in session (default)
	ProvenanceMachine   = "machine"    // maintenance loop ran an allowlisted observable
	ProvenanceLLMReview = "llm-review" // overseer reviewer proposal — never auto-executed
)

func normalizeEvidenceProvenance(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == ProvenanceMachine || value == ProvenanceLLMReview {
		return value
	}
	return ProvenanceHuman
}

// Mutation-ladder rungs (note-20260611-dc539572). The rung decides HOW a
// maintenance task may be acted on; the config envelope decides WHETHER.
const (
	// RungDeterministic: a kernel predicate proves the action benign
	// (e.g. additive-only drift under the conservative gate). No judgment.
	RungDeterministic = 1
	// RungMachine: an allowlisted observable command produces the evidence.
	RungMachine = 2
	// RungJudgment: needs human or LLM judgment. NEVER auto-executed —
	// at most rendered as a proposal, regardless of config.
	RungJudgment = 3
)

// commandAllowlist maps permitted argv[0] binaries to their permitted
// subcommands (nil = first argument unrestricted). Stored observable commands
// are an injection surface: execution is exec-direct (never a shell), the
// binary must be allowlisted, and shell metacharacters disqualify outright.
var commandAllowlist = map[string]map[string]bool{
	"go":   {"test": true, "build": true, "vet": true},
	"grep": nil,
	"rg":   nil,
}

const commandShellMeta = ";|&$`<>(){}*?~#\\\"'"

// ClassifyCommand validates a stored observable command against the
// allowlist. Returns the command class ("go test", "grep", ...) and whether
// execution is permitted. Pure.
func ClassifyCommand(command string) (string, bool) {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsAny(command, commandShellMeta) {
		return "", false
	}
	fields := strings.Fields(command)
	allowedSubs, ok := commandAllowlist[fields[0]]
	if !ok {
		return "", false
	}
	if allowedSubs == nil {
		if !grepLikeCommandConfined(fields[1:]) {
			return "", false
		}
		return fields[0], true
	}
	if len(fields) < 2 || !allowedSubs[fields[1]] {
		return "", false
	}
	if !goCommandConfined(fields[2:]) {
		return "", false
	}
	return fields[0] + " " + fields[1], true
}

func goCommandConfined(args []string) bool {
	for _, arg := range args {
		if commandArgEscapesProject(arg) {
			return false
		}
	}

	return true
}

func grepLikeCommandConfined(args []string) bool {
	for _, arg := range args {
		if commandArgEscapesProject(arg) {
			return false
		}
	}

	return true
}

func commandArgEscapesProject(arg string) bool {
	values := []string{arg}
	if _, value, ok := strings.Cut(arg, "="); ok {
		values = append(values, value)
	}

	for _, value := range values {
		if pathTokenEscapesProject(value) {
			return true
		}
	}

	return false
}

func pathTokenEscapesProject(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if filepath.IsAbs(value) {
		return true
	}

	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return true
		}
	}

	return false
}

// BaselineSnapshot is the prior state captured before an autonomous
// re-baseline — the undo payload of the maintenance ledger. Manifests are
// included because re-baselining rebuilds drift-scope manifests; restoring
// files+symbols alone would leave added-file drift silently absorbed.
type BaselineSnapshot struct {
	Files     []AffectedFile       `json:"files"`
	Symbols   []AffectedSymbol     `json:"symbols,omitempty"`
	Manifests []DriftScopeManifest `json:"manifests,omitempty"`
}

// CaptureBaselineSnapshot reads the full restorable baseline state of a decision.
func CaptureBaselineSnapshot(ctx context.Context, store ArtifactStore, decisionRef string) (BaselineSnapshot, error) {
	files, err := store.GetAffectedFiles(ctx, decisionRef)
	if err != nil {
		return BaselineSnapshot{}, fmt.Errorf("get affected files: %w", err)
	}
	symbols, err := store.GetAffectedSymbols(ctx, decisionRef)
	if err != nil {
		return BaselineSnapshot{}, fmt.Errorf("get affected symbols: %w", err)
	}
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return BaselineSnapshot{}, fmt.Errorf("get decision: %w", err)
	}
	return BaselineSnapshot{
		Files:     files,
		Symbols:   symbols,
		Manifests: a.UnmarshalDecisionFields().DriftManifests,
	}, nil
}

// RestoreBaselineSnapshot puts a previously captured baseline state back —
// the one-step undo for autonomous re-baselines.
func RestoreBaselineSnapshot(ctx context.Context, store ArtifactStore, decisionRef string, snapshot BaselineSnapshot) error {
	if err := store.SetAffectedFiles(ctx, decisionRef, snapshot.Files); err != nil {
		return fmt.Errorf("restore affected files: %w", err)
	}
	if err := store.SetAffectedSymbols(ctx, decisionRef, snapshot.Symbols); err != nil {
		return fmt.Errorf("restore affected symbols: %w", err)
	}
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return fmt.Errorf("get decision: %w", err)
	}
	if err := persistDriftManifests(ctx, store, a, snapshot.Manifests); err != nil {
		return fmt.Errorf("restore drift manifests: %w", err)
	}
	return nil
}

// MaintenanceTask is one typed micro-task in the compiled work order.
type MaintenanceTask struct {
	DecisionRef   string `json:"decision_ref"`
	DecisionTitle string `json:"decision_title"`
	Source        string `json:"source"`   // "stale" | "drift"
	Category      string `json:"category"` // StaleCategory or AutoBaselineAction
	ClaimID       string `json:"claim_id,omitempty"`
	Observable    string `json:"observable,omitempty"`
	Threshold     string `json:"threshold,omitempty"`
	Command       string `json:"command,omitempty"`
	CommandClass  string `json:"command_class,omitempty"`
	Rung          int    `json:"rung"`
	Reason        string `json:"reason"`
	EstCost       string `json:"est_cost"` // "seconds" | "judgment"
}

// MaintenancePlan is the kernel-compiled work order: what the maintenance
// loop (interactive agent or overseer execute-phase) can act on right now.
type MaintenancePlan struct {
	GeneratedAt            string            `json:"generated_at"`
	Tasks                  []MaintenanceTask `json:"tasks"`
	AutoBaselineCandidates int               `json:"auto_baseline_candidates"`
	MachineCheckable       int               `json:"machine_checkable"`
	JudgmentNeeded         int               `json:"judgment_needed"`
	CooldownSkipped        int               `json:"cooldown_skipped"`
}

// BuildMaintenancePlan compiles the typed work order from the stale scan and
// the drift dispositions. Read-only and deterministic given store state: the
// ranking is rung-ascending (cheapest, safest first), then decision ref.
func BuildMaintenancePlan(ctx context.Context, store ArtifactStore, projectRoot string) (*MaintenancePlan, error) {
	plan := &MaintenancePlan{GeneratedAt: time.Now().UTC().Format(time.RFC3339)}

	staleTasks, skipped, err := staleClaimTasks(ctx, store, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("stale scan for plan: %w", err)
	}
	plan.Tasks = append(plan.Tasks, staleTasks...)
	plan.CooldownSkipped = skipped

	driftTasks, err := driftDispositionTasks(ctx, store, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("drift scan for plan: %w", err)
	}
	plan.Tasks = append(plan.Tasks, driftTasks...)

	sort.SliceStable(plan.Tasks, func(i, j int) bool {
		if plan.Tasks[i].Rung != plan.Tasks[j].Rung {
			return plan.Tasks[i].Rung < plan.Tasks[j].Rung
		}
		return plan.Tasks[i].DecisionRef < plan.Tasks[j].DecisionRef
	})

	for _, t := range plan.Tasks {
		switch t.Rung {
		case RungDeterministic:
			plan.AutoBaselineCandidates++
		case RungMachine:
			plan.MachineCheckable++
		default:
			plan.JudgmentNeeded++
		}
	}
	return plan, nil
}

// staleClaimTasks maps ripe unverified claims of stale decisions to tasks.
// Cooldown: a claim that already received evidence today is skipped — the
// flake guard that keeps oscillating observables from re-running every cycle.
func staleClaimTasks(ctx context.Context, store ArtifactStore, projectRoot string) ([]MaintenanceTask, int, error) {
	items, err := ScanStale(ctx, store, projectRoot)
	if err != nil {
		return nil, 0, err
	}

	seen := make(map[string]bool)
	tasks := make([]MaintenanceTask, 0, len(items))
	skipped := 0
	for _, item := range items {
		if item.Kind != string(KindDecisionRecord) || item.ID == "" || seen[item.ID] {
			continue
		}
		if !staleCategoryPlannable(item.Category) {
			continue
		}
		seen[item.ID] = true

		full, err := store.Get(ctx, item.ID)
		if err != nil || full == nil {
			continue
		}
		evidenceToday := evidenceDatesToday(ctx, store, item.ID)
		for _, claim := range full.UnmarshalDecisionFields().Claims {
			if claim.Status != ClaimStatusUnverified || !claimRipe(claim) {
				continue
			}
			if evidenceToday[claim.ID] {
				skipped++
				continue
			}
			tasks = append(tasks, claimTask(item, claim))
		}
	}
	return tasks, skipped, nil
}

func staleCategoryPlannable(category StaleCategory) bool {
	switch category {
	case StaleCategoryREffDegraded, StaleCategoryPendingVerification, StaleCategoryEvidenceExpired, StaleCategoryDecisionStale:
		return true
	default:
		return false
	}
}

func claimRipe(claim DecisionClaim) bool {
	if claim.VerifyAfter == "" {
		return true
	}
	verifyTime, ok := reff.ParseValidUntil(claim.VerifyAfter)
	if !ok {
		return true
	}
	return !time.Now().Before(verifyTime)
}

func claimTask(item StaleItem, claim DecisionClaim) MaintenanceTask {
	task := MaintenanceTask{
		DecisionRef:   item.ID,
		DecisionTitle: item.Title,
		Source:        "stale",
		Category:      string(item.Category),
		ClaimID:       claim.ID,
		Observable:    claim.Observable,
		Threshold:     claim.Threshold,
		Rung:          RungJudgment,
		Reason:        item.Reason,
		EstCost:       "judgment",
	}
	class, ok := ClassifyCommand(claim.Command)
	if ok {
		task.Command = strings.TrimSpace(claim.Command)
		task.CommandClass = class
		task.Rung = RungMachine
		task.EstCost = "seconds"
	}
	return task
}

// evidenceDatesToday returns the claim refs that already received evidence
// today (UTC), keyed by claim ID. Evidence IDs carry their date prefix
// (evid-YYYYMMDD-...), so no extra storage is needed for the cooldown.
func evidenceDatesToday(ctx context.Context, store ArtifactStore, decisionRef string) map[string]bool {
	out := make(map[string]bool)
	items, err := store.GetEvidenceItems(ctx, decisionRef)
	if err != nil {
		return out
	}
	today := time.Now().UTC().Format("20060102")
	for _, item := range items {
		if !strings.HasPrefix(item.ID, "evid-"+today+"-") {
			continue
		}
		for _, ref := range item.ClaimRefs {
			out[ref] = true
		}
	}
	return out
}

// driftDispositionTasks maps the conservative-gate dispositions to tasks:
// additive-only drift becomes a rung-1 auto-rebaseline candidate; anything
// touching a governed symbol or unprovable stays rung-3 judgment.
func driftDispositionTasks(ctx context.Context, store ArtifactStore, projectRoot string) ([]MaintenanceTask, error) {
	reports, err := CheckDrift(ctx, store, projectRoot)
	if err != nil {
		return nil, err
	}

	tasks := make([]MaintenanceTask, 0, len(reports))
	for _, d := range ClassifyAutoBaseline(driftedOnly(reports)) {
		task := MaintenanceTask{
			DecisionRef:   d.Report.DecisionID,
			DecisionTitle: d.Report.DecisionTitle,
			Source:        "drift",
			Category:      string(d.Action),
			Reason:        d.Reason,
			Rung:          RungJudgment,
			EstCost:       "judgment",
		}
		if d.Action == AutoResolveSilent {
			task.Rung = RungDeterministic
			task.EstCost = "seconds"
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func driftedOnly(reports []DriftReport) []DriftReport {
	out := make([]DriftReport, 0, len(reports))
	for _, r := range reports {
		if len(r.Files) > 0 {
			out = append(out, r)
		}
	}
	return out
}
