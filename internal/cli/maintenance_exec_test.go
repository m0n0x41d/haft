package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
)

const maintExecAffectedFile = "internal/app/maintflow.go"

// TestMaintenanceExecutePhase_GateModesAndUndo is the e2e proof of
// dec-20260611-overseer-maintenance-executor-b1a7a749: additive-only drift
// (a NEW file in a module-governed scope — the rebuild/init noise class) →
// plan → config gate (off / propose / auto) → auto-rebaseline with ledger →
// machine evidence for rung-2 observables with flake cooldown → disclosure →
// one-step undo restoring files, symbols, AND drift manifests.
func TestMaintenanceExecutePhase_GateModesAndUndo(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	if err := runInit(&cobra.Command{}, nil); err != nil {
		t.Fatalf("run init: %v", err)
	}

	ctx := context.Background()
	decisionID := createMaintExecDecision(t, root)

	// Additive-only drift: a NEW file appears in the governed module scope.
	// The governed file itself stays untouched — this is exactly the class
	// the conservative gate proves benign (every drift item is an addition).
	writeGoldenE2EFile(t, filepath.Join(root, "internal/app/extra.go"),
		"package app\n\nfunc Extra() string {\n\treturn \"added\"\n}\n")

	// Phase 1 — off: execute-phase fully disabled.
	saveMaintenanceModes(t, root, overseer.MaintenanceModeOff, overseer.MaintenanceModeOff)
	if actions := withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		cfg, _ := overseer.LoadConfig(root)
		return executeMaintenancePlan(ctx, store, root, cfg)
	}); len(actions) != 0 {
		t.Fatalf("mode=off produced %d action(s), want 0", len(actions))
	}

	// Phase 2 — propose: rebaseline and rung-2 observables are recorded as
	// proposals only. Viewing/planning must not mutate baselines or attach
	// machine evidence.
	saveMaintenanceModes(t, root, overseer.MaintenanceModePropose, overseer.MaintenanceModePropose)
	proposeActions := withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		cfg, _ := overseer.LoadConfig(root)
		return executeMaintenancePlan(ctx, store, root, cfg)
	})
	assertActionOutcome(t, proposeActions, maintenanceActionRebaseline, decisionID, "proposed")
	observable := assertActionOutcome(t, proposeActions, maintenanceActionObservable, decisionID, "proposed")
	if !strings.Contains(observable.Detail, "no evidence attached") {
		t.Errorf("observable detail = %q, want proposal without evidence attach", observable.Detail)
	}
	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		if !driftPresent(t, ctx, store, root, decisionID) {
			t.Fatal("propose mode must NOT re-baseline — drift should persist")
		}
		items, err := store.GetEvidenceItems(ctx, decisionID)
		if err != nil {
			t.Fatalf("get evidence: %v", err)
		}
		for _, item := range items {
			if item.Provenance == artifact.ProvenanceMachine {
				t.Fatalf("propose mode must not attach machine evidence: %+v", item)
			}
		}
		return nil
	})

	// Phase 3 — auto: rebaseline applies with prior state recorded; the
	// observable runs and attaches evidence; the stored maintenance run carries
	// the ledger.
	saveMaintenanceModes(t, root, overseer.MaintenanceModeAuto, overseer.MaintenanceModeAuto)
	var maintenanceRun overseer.MaintenanceRun
	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		run, err := buildAndStoreOverseerMaintenance(ctx, store, root)
		if err != nil {
			t.Fatalf("build and store maintenance: %v", err)
		}
		maintenanceRun = run
		return nil
	})

	rebaseline := assertActionOutcome(t, maintenanceRun.Executed, maintenanceActionRebaseline, decisionID, "applied")
	if rebaseline.PriorState == "" {
		t.Fatal("applied rebaseline must record prior state for undo")
	}
	assertActionOutcome(t, maintenanceRun.Executed, maintenanceActionObservable, decisionID, "evidence_attached")

	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		if driftPresent(t, ctx, store, root, decisionID) {
			t.Fatal("auto mode should have re-baselined the additive drift")
		}
		items, err := store.GetEvidenceItems(ctx, decisionID)
		if err != nil {
			t.Fatalf("get evidence: %v", err)
		}
		machine := 0
		for _, item := range items {
			if item.Provenance == artifact.ProvenanceMachine {
				machine++
			}
		}
		if machine == 0 {
			t.Fatal("expected provenance=machine evidence from the observable run")
		}
		return nil
	})

	// Disclosure: the status summary renders the ledger with the undo command.
	summary := overseer.BuildStatusSummary(overseer.StoredRun{}, false, maintenanceRun, true)
	disclosure := overseer.FormatStatusSignals(summary)
	if !strings.Contains(disclosure, "AUTONOMOUS MAINTENANCE") || !strings.Contains(disclosure, "haft overseer undo "+maintenanceRun.MaintenanceID) {
		t.Errorf("disclosure missing autonomy ledger or undo command:\n%s", disclosure)
	}

	// Phase 4 — undo restores files, symbols, and drift manifests: the
	// absorbed added-file drift must become visible again.
	undoCmd := &cobra.Command{}
	if err := runOverseerUndo(undoCmd, []string{maintenanceRun.MaintenanceID, rebaseline.ID}); err != nil {
		t.Fatalf("undo: %v", err)
	}
	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		if !driftPresent(t, ctx, store, root, decisionID) {
			t.Fatal("undo must restore the PRIOR baseline — added-file drift should be visible again")
		}
		return nil
	})
}

// TestMaintenanceExecutePhase_RevalidateStale proves the evidence-backed
// revalidation gate: a calendar-stale decision whose machine observable
// re-runs green gets valid_until extended in the same run — and only then.
func TestMaintenanceExecutePhase_RevalidateStale(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	if err := runInit(&cobra.Command{}, nil); err != nil {
		t.Fatalf("run init: %v", err)
	}

	ctx := context.Background()
	writeGoldenE2EFile(t, filepath.Join(root, maintExecAffectedFile),
		"package app\n\nfunc Flow() string {\n\treturn \"ready\"\n}\n")

	database, store := openGoldenE2EStore(t, root)
	haftDir := filepath.Join(root, ".haft")
	stale := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Stale revalidation fixture decision",
		WhySelected:     "E2E fixture for evidence-backed revalidation.",
		SelectionPolicy: "Fixture.",
		CounterArgument: "Fixture.",
		WeakestLink:     "Fixture.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:      []string{"Flow stays present"},
		Predictions: []artifact.PredictionInput{{
			Claim:      "Flow function stays present",
			Observable: "grep for func in the governed file",
			Threshold:  "exit 0",
			Command:    "grep -n func internal/app/maintflow.go",
		}},
		AffectedFiles:  []string{maintExecAffectedFile},
		ValidUntil:     stale,
		GovernanceMode: "exact",
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	database.Close()

	saveMaintenanceModes(t, root, overseer.MaintenanceModeOff, overseer.MaintenanceModeAuto)
	actions := withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		cfg, _ := overseer.LoadConfig(root)
		return executeMaintenancePlan(ctx, store, root, cfg)
	})

	assertActionOutcome(t, actions, maintenanceActionObservable, decision.Meta.ID, "evidence_attached")
	revalidate := assertActionOutcome(t, actions, maintenanceActionRevalidate, decision.Meta.ID, "applied")
	if !strings.Contains(revalidate.Detail, "valid_until extended") {
		t.Errorf("revalidate detail = %q", revalidate.Detail)
	}

	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		refreshed, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			t.Fatalf("get decision: %v", err)
		}
		validUntil := refreshed.Meta.ValidUntil
		parsed, parseErr := time.Parse(time.RFC3339, validUntil)
		if parseErr != nil {
			parsed, parseErr = time.Parse("2006-01-02", validUntil)
		}
		if parseErr != nil {
			t.Fatalf("parse refreshed valid_until %q: %v", validUntil, parseErr)
		}
		if !parsed.After(time.Now()) {
			t.Errorf("valid_until = %s, want extended into the future", validUntil)
		}
		return nil
	})
}

func TestMaintenanceExecutePhase_CappedObservablesPreventRevalidation(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	if err := runInit(&cobra.Command{}, nil); err != nil {
		t.Fatalf("run init: %v", err)
	}

	ctx := context.Background()
	writeGoldenE2EFile(t, filepath.Join(root, maintExecAffectedFile),
		"package app\n\nfunc Flow() string {\n\treturn \"ready\"\n}\n")

	predictions := make([]artifact.PredictionInput, 0, maxObservablesPerRun+1)
	for index := 0; index < maxObservablesPerRun+1; index++ {
		predictions = append(predictions, artifact.PredictionInput{
			Claim:      "Flow function stays present",
			Observable: "grep for Flow in the governed file",
			Threshold:  "exit 0",
			Command:    "grep -n Flow internal/app/maintflow.go",
		})
	}

	database, store := openGoldenE2EStore(t, root)
	haftDir := filepath.Join(root, ".haft")
	stale := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Capped observable fixture decision",
		WhySelected:     "E2E fixture for partial machine evidence.",
		SelectionPolicy: "Fixture.",
		CounterArgument: "Fixture.",
		WeakestLink:     "Fixture.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:      []string{"Flow stays present"},
		Predictions:     predictions,
		AffectedFiles:   []string{maintExecAffectedFile},
		ValidUntil:      stale,
		GovernanceMode:  "exact",
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	database.Close()

	saveMaintenanceModes(t, root, overseer.MaintenanceModeOff, overseer.MaintenanceModeAuto)
	actions := withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		cfg, _ := overseer.LoadConfig(root)
		return executeMaintenancePlan(ctx, store, root, cfg)
	})

	observableCount := 0
	for _, action := range actions {
		if action.Kind == maintenanceActionObservable && action.DecisionRef == decision.Meta.ID {
			observableCount++
		}
		if action.Kind == maintenanceActionRevalidate && action.DecisionRef == decision.Meta.ID {
			t.Fatalf("capped observable set must not revalidate on partial evidence: %+v", action)
		}
	}
	if observableCount != maxObservablesPerRun {
		t.Fatalf("observable actions = %d, want cap %d", observableCount, maxObservablesPerRun)
	}

	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		refreshed, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			t.Fatalf("get decision: %v", err)
		}
		if refreshed.Meta.ValidUntil != stale {
			t.Fatalf("valid_until = %q, want unchanged stale value %q", refreshed.Meta.ValidUntil, stale)
		}
		return nil
	})
}

func createMaintExecDecision(t *testing.T, root string) string {
	t.Helper()

	writeGoldenE2EFile(t, filepath.Join(root, maintExecAffectedFile),
		"package app\n\nfunc Flow() string {\n\treturn \"ready\"\n}\n")

	database, store := openGoldenE2EStore(t, root)
	defer database.Close()

	ctx := context.Background()
	haftDir := filepath.Join(root, ".haft")
	ripe := time.Now().Add(-24 * time.Hour).Format("2006-01-02")

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:   "Maintenance executor fixture decision",
		WhySelected:     "E2E fixture for the maintenance execute-phase.",
		SelectionPolicy: "Fixture.",
		CounterArgument: "Fixture.",
		WeakestLink:     "Fixture.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:      []string{"Flow stays present"},
		Predictions: []artifact.PredictionInput{{
			Claim:       "Flow function stays present",
			Observable:  "grep for func in the governed file",
			Threshold:   "exit 0",
			VerifyAfter: ripe,
			Command:     "grep -n func internal/app/maintflow.go",
		}},
		AffectedFiles:  []string{maintExecAffectedFile},
		ValidUntil:     "2099-01-01T00:00:00Z",
		GovernanceMode: "module",
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}

	if _, err := artifact.Baseline(ctx, store, root, artifact.BaselineInput{DecisionRef: decision.Meta.ID}); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	return decision.Meta.ID
}

func saveMaintenanceModes(t *testing.T, root, rebaseline, revalidate string) {
	t.Helper()
	cfg, err := overseer.LoadConfig(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.MaintenanceRebaseline = rebaseline
	cfg.MaintenanceRevalidateStale = revalidate
	if err := overseer.SaveConfig(root, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func withMaintStore(t *testing.T, root string, fn func(*artifact.Store) []overseer.MaintenanceAction) []overseer.MaintenanceAction {
	t.Helper()
	database, store := openGoldenE2EStore(t, root)
	defer database.Close()
	return fn(store)
}

func assertActionOutcome(t *testing.T, actions []overseer.MaintenanceAction, kind, decisionRef, outcome string) overseer.MaintenanceAction {
	t.Helper()
	for _, action := range actions {
		if action.Kind == kind && action.DecisionRef == decisionRef && action.Outcome == outcome {
			return action
		}
	}
	t.Fatalf("no action kind=%s decision=%s outcome=%s in ledger: %+v", kind, decisionRef, outcome, actions)
	return overseer.MaintenanceAction{}
}

func driftPresent(t *testing.T, ctx context.Context, store *artifact.Store, root, decisionID string) bool {
	t.Helper()
	reports, err := artifact.CheckDrift(ctx, store, root)
	if err != nil {
		t.Fatalf("check drift: %v", err)
	}
	for _, r := range reports {
		if r.DecisionID == decisionID && len(r.Files) > 0 {
			return true
		}
	}
	return false
}
