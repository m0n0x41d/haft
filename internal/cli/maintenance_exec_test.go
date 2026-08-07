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

func TestOverseerDrainHelpNamesAuthorityBoundaries(t *testing.T) {
	t.Parallel()

	normalized := strings.ToLower(strings.Join(strings.Fields(overseerDrainCmd.Long), " "))
	for _, want := range []string{
		"opt-in",
		"machine-safe maintenance",
		"needs_operator",
		"never applies",
		"decisionreconciliationselection",
		"not semantic approval",
		"not evidence",
		"not gatedecision",
		"not claim truth",
		"not global truth",
		"not publication",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("overseer drain help missing %q:\n%s", want, overseerDrainCmd.Long)
		}
	}
}

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
	runTypedCoreInitForTest(t)

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

func TestMaintenanceDrainDryRunReportsProposalsAndNeedsOperatorWithoutMutation(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	runTypedCoreInitForTest(t)

	ctx := context.Background()
	safeDecisionID := createMaintExecDecision(t, root)
	createDrainJudgmentDecision(t, root)
	writeGoldenE2EFile(t, filepath.Join(root, "internal/app/extra.go"),
		"package app\n\nfunc Extra() string {\n\treturn \"added\"\n}\n")

	var report maintenanceDrainReport
	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		var err error
		report, err = buildMaintenanceDrainReport(ctx, store, root, true)
		if err != nil {
			t.Fatalf("build drain report: %v", err)
		}
		return nil
	})

	if !report.DryRun {
		t.Fatal("dry-run report should set dry_run=true")
	}
	assertMaintenanceDrainAuthorityBoundary(t, report.AuthorityBoundary)
	if report.AuthorityBoundary.Mutation != "not_mutation" || report.AuthorityBoundary.Evidence != "not_evidence" {
		t.Fatalf("dry-run authority boundary = %#v", report.AuthorityBoundary)
	}
	if report.Summary.ProposedActions == 0 {
		t.Fatalf("dry-run should propose safe actions: %+v", report.Summary)
	}
	if report.Summary.AppliedActions != 0 || report.Summary.EvidenceActions != 0 {
		t.Fatalf("dry-run must not apply or attach evidence: %+v", report.Summary)
	}
	if report.Summary.NeedsOperatorTasks == 0 {
		t.Fatalf("dry-run should surface needs_operator tasks: %+v", report)
	}

	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		if !driftPresent(t, ctx, store, root, safeDecisionID) {
			t.Fatal("dry-run must leave additive drift visible")
		}
		if machineEvidenceCount(t, ctx, store, safeDecisionID) != 0 {
			t.Fatal("dry-run must not attach machine evidence")
		}
		return nil
	})
}

func TestCompactMaintenanceDrainReportPreservesSummaryAndOmitsAuditTail(t *testing.T) {
	t.Parallel()

	report := maintenanceDrainReport{
		SchemaVersion: maintenanceDrainSchemaVersion,
		DryRun:        true,
		Summary: maintenanceDrainSummary{
			ExecutedActions:             3,
			NeedsOperatorTasks:          4,
			ReconciliationProposalCount: 3,
		},
		Executed: []overseer.MaintenanceAction{
			{ID: "act-001"},
			{ID: "act-002"},
			{ID: "act-003"},
		},
		ReconciliationProposals: []overseer.MaintenanceReconciliationProposal{
			{ID: "proposal-001"},
			{ID: "proposal-002"},
			{ID: "proposal-003"},
		},
		AfterAction: overseer.MaintenanceAfterActionReport{
			RemainingOperatorJudgment: []overseer.MaintenanceAfterActionItem{
				{Ref: "finding-001"},
				{Ref: "finding-002"},
				{Ref: "finding-003"},
			},
			AuthorityBoundary: "after_action_report_only_not_binding_authority_not_mutation_not_evidence_not_approval_not_gate_decision_not_claim_truth_not_global_truth_not_publication",
		},
		NeedsOperator: []artifact.MaintenanceJudgmentGroup{
			{
				Recommendation: "review",
				Tasks: []artifact.MaintenanceJudgmentTaskReview{
					{DecisionRef: "dec-review-001"},
					{DecisionRef: "dec-review-002"},
				},
			},
			{
				Recommendation: "verify",
				Tasks: []artifact.MaintenanceJudgmentTaskReview{
					{DecisionRef: "dec-verify-001"},
				},
			},
			{
				Recommendation: "reopen",
				Tasks: []artifact.MaintenanceJudgmentTaskReview{
					{DecisionRef: "dec-reopen-001"},
				},
			},
		},
	}

	compact := compactMaintenanceDrainReport(report, 1)

	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if compact.Summary != report.Summary {
		t.Fatalf("summary changed: got %#v want %#v", compact.Summary, report.Summary)
	}
	if len(compact.Executed) != 1 || compact.OmittedExecuted != 2 {
		t.Fatalf("executed projection = len %d omitted %d", len(compact.Executed), compact.OmittedExecuted)
	}
	if len(compact.ReconciliationProposals) != 1 || compact.OmittedReconciliation != 2 {
		t.Fatalf("reconciliation projection = len %d omitted %d", len(compact.ReconciliationProposals), compact.OmittedReconciliation)
	}
	if len(compact.AfterAction.RemainingOperatorJudgment) != 1 || compact.OmittedAfterAction != 2 {
		t.Fatalf("after_action projection = len %d omitted %d", len(compact.AfterAction.RemainingOperatorJudgment), compact.OmittedAfterAction)
	}
	if len(compact.NeedsOperator) != 3 || compact.OmittedNeedsOperatorTasks != 3 {
		t.Fatalf("needs_operator projection = len %d omitted tasks %d", len(compact.NeedsOperator), compact.OmittedNeedsOperatorTasks)
	}
	if len(compact.NeedsOperator[0].Tasks) != 1 || compact.NeedsOperator[0].OmittedTasks != 1 {
		t.Fatalf("needs_operator nested projection = %#v", compact.NeedsOperator[0])
	}
	if compact.FullAuditCommand != "haft overseer drain --dry-run --json --full" {
		t.Fatalf("full_audit_command = %q", compact.FullAuditCommand)
	}
	if len(report.Executed) != 3 || len(report.ReconciliationProposals) != 3 || len(report.NeedsOperator) != 3 {
		t.Fatalf("source report mutated: %#v", report)
	}

	zero := compactMaintenanceDrainReport(report, 0)
	if len(zero.Executed) != 0 || zero.OmittedExecuted != 3 {
		t.Fatalf("zero-limit executed projection = len %d omitted %d", len(zero.Executed), zero.OmittedExecuted)
	}
	if len(zero.ReconciliationProposals) != 0 || zero.OmittedReconciliation != 3 {
		t.Fatalf("zero-limit reconciliation projection = len %d omitted %d", len(zero.ReconciliationProposals), zero.OmittedReconciliation)
	}
	if len(zero.AfterAction.RemainingOperatorJudgment) != 0 || zero.OmittedAfterAction != 3 {
		t.Fatalf("zero-limit after_action projection = len %d omitted %d", len(zero.AfterAction.RemainingOperatorJudgment), zero.OmittedAfterAction)
	}
	if len(zero.NeedsOperator) != 3 || zero.OmittedNeedsOperatorTasks != 4 {
		t.Fatalf("zero-limit needs_operator projection = len %d omitted tasks %d", len(zero.NeedsOperator), zero.OmittedNeedsOperatorTasks)
	}
	if len(zero.NeedsOperator[0].Tasks) != 0 || zero.NeedsOperator[0].OmittedTasks != 2 {
		t.Fatalf("zero-limit nested projection = %#v", zero.NeedsOperator[0])
	}
}

func TestMaintenanceDrainActionLinesRenderDecisionTitleBeforeRef(t *testing.T) {
	t.Parallel()

	lines := maintenanceDrainActionLines([]overseer.MaintenanceAction{{
		Kind:        maintenanceActionObservable,
		DecisionRef: "dec-20260620-134e8f83",
		Title:       "Adaptive stage gate",
		Outcome:     "evidence_attached",
		Detail:      "go test ./internal/cli — green",
	}})

	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}
	want := "**Adaptive stage gate** `dec-20260620-134e8f83`"
	if !strings.Contains(lines[0], want) {
		t.Fatalf("drain line should render decision title before ref, want %q in:\n%s", want, lines[0])
	}
	if strings.HasPrefix(lines[0], "`"+maintenanceActionObservable+"`") {
		t.Fatalf("drain line regressed to machine-marker-first format:\n%s", lines[0])
	}
}

func TestMaintenanceDrainClosesSafeItemsAndReportsNeedsOperator(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	runTypedCoreInitForTest(t)

	ctx := context.Background()
	safeDecisionID := createMaintExecDecision(t, root)
	judgmentDecisionID := createDrainJudgmentDecision(t, root)
	writeGoldenE2EFile(t, filepath.Join(root, "internal/app/extra.go"),
		"package app\n\nfunc Extra() string {\n\treturn \"added\"\n}\n")

	var report maintenanceDrainReport
	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		var err error
		report, err = buildMaintenanceDrainReport(ctx, store, root, false)
		if err != nil {
			t.Fatalf("build drain report: %v", err)
		}
		return nil
	})

	if report.DryRun {
		t.Fatal("real drain should set dry_run=false")
	}
	if report.MaintenanceRunID == "" {
		t.Fatal("real drain should store a maintenance run for audit/undo")
	}
	assertMaintenanceDrainAuthorityBoundary(t, report.AuthorityBoundary)
	if report.AuthorityBoundary.Mutation != "machine_safe_only" || report.AuthorityBoundary.Evidence != "machine_evidence_only" {
		t.Fatalf("real drain authority boundary = %#v", report.AuthorityBoundary)
	}
	if report.Summary.AppliedActions == 0 || report.Summary.EvidenceActions == 0 {
		t.Fatalf("real drain should apply safe actions and attach evidence: %+v", report.Summary)
	}
	if report.Summary.NeedsOperatorTasks == 0 {
		t.Fatalf("real drain should keep judgment tasks operator-facing: %+v", report)
	}

	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		if driftPresent(t, ctx, store, root, safeDecisionID) {
			t.Fatal("real drain should rebaseline additive-only drift")
		}
		if machineEvidenceCount(t, ctx, store, safeDecisionID) == 0 {
			t.Fatal("real drain should attach machine evidence for allowlisted observable")
		}
		refreshed, err := store.Get(ctx, judgmentDecisionID)
		if err != nil {
			t.Fatalf("get judgment decision: %v", err)
		}
		if refreshed.Meta.ValidUntil == "" || !strings.HasPrefix(refreshed.Meta.ValidUntil, "20") {
			t.Fatalf("judgment decision valid_until unexpectedly blank: %#v", refreshed.Meta)
		}
		return nil
	})
}

func TestMaintenanceExecutePhaseDisabledConfigSkipsMutations(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	runTypedCoreInitForTest(t)

	ctx := context.Background()
	decisionID := createMaintExecDecision(t, root)
	writeGoldenE2EFile(t, filepath.Join(root, "internal/app/extra.go"),
		"package app\n\nfunc Extra() string {\n\treturn \"added\"\n}\n")

	saveMaintenanceModes(t, root, overseer.MaintenanceModeAuto, overseer.MaintenanceModeAuto)
	cfg, err := overseer.LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	cfg.Enabled = false
	if err := overseer.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig returned error: %v", err)
	}

	actions := withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		return executeMaintenancePlan(ctx, store, root, cfg)
	})
	if len(actions) != 0 {
		t.Fatalf("enabled=false produced %d action(s), want 0", len(actions))
	}

	withMaintStore(t, root, func(store *artifact.Store) []overseer.MaintenanceAction {
		if !driftPresent(t, ctx, store, root, decisionID) {
			t.Fatal("enabled=false must leave drift unmodified")
		}
		items, err := store.GetEvidenceItems(ctx, decisionID)
		if err != nil {
			t.Fatalf("get evidence: %v", err)
		}
		for _, item := range items {
			if item.Provenance == artifact.ProvenanceMachine {
				t.Fatalf("enabled=false must not attach machine evidence: %+v", item)
			}
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
	runTypedCoreInitForTest(t)

	ctx := context.Background()
	writeGoldenE2EFile(t, filepath.Join(root, maintExecAffectedFile),
		"package app\n\nfunc Flow() string {\n\treturn \"ready\"\n}\n")

	database, store := openGoldenE2EStore(t, root)
	haftDir := filepath.Join(root, ".haft")
	stale := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Stale revalidation fixture decision",
		ProblemStatement: "A stale governed decision needs evidence-backed revalidation.",
		WhySelected:      "E2E fixture for evidence-backed revalidation.",
		SelectionPolicy:  "Fixture.",
		CounterArgument:  "Fixture.",
		WeakestLink:      "Fixture.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:       []string{"Flow stays present"},
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

func TestMaintenanceExecutePhase_JudgmentStaleClaimBlocksRevalidation(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	runTypedCoreInitForTest(t)

	ctx := context.Background()
	writeGoldenE2EFile(t, filepath.Join(root, maintExecAffectedFile),
		"package app\n\nfunc Flow() string {\n\treturn \"ready\"\n}\n")

	database, store := openGoldenE2EStore(t, root)
	haftDir := filepath.Join(root, ".haft")
	stale := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Mixed stale claims fixture decision",
		ProblemStatement: "Mixed machine and judgment claims need a bounded revalidation gate.",
		WhySelected:      "E2E fixture for the all-stale-tasks revalidation gate.",
		SelectionPolicy:  "Fixture.",
		CounterArgument:  "Fixture.",
		WeakestLink:      "Fixture.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:       []string{"Flow stays present"},
		Predictions: []artifact.PredictionInput{
			{
				Claim:      "Flow function stays present",
				Observable: "grep for func in the governed file",
				Threshold:  "exit 0",
				Command:    "grep -n func internal/app/maintflow.go",
			},
			{
				Claim:      "Operator confidence remains high",
				Observable: "operator interview",
				Threshold:  "human judgment supports",
			},
		},
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
	for _, action := range actions {
		if action.Kind == maintenanceActionRevalidate && action.DecisionRef == decision.Meta.ID {
			t.Fatalf("judgment-only stale claim must block revalidation: %+v", action)
		}
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

func TestMaintenanceExecutePhase_CappedObservablesPreventRevalidation(t *testing.T) {
	restore := overrideGoldenE2EFlags(t)
	defer restore()

	root := newGoldenE2ERepo(t)
	restoreCwd := enterTestProjectRoot(t, root)
	defer restoreCwd()
	t.Setenv("HOME", filepath.Join(root, ".test-home"))

	initLocal = true
	runTypedCoreInitForTest(t)

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
		SelectedTitle:    "Capped observable fixture decision",
		ProblemStatement: "A capped evidence run must not overstate partial machine verification.",
		WhySelected:      "E2E fixture for partial machine evidence.",
		SelectionPolicy:  "Fixture.",
		CounterArgument:  "Fixture.",
		WeakestLink:      "Fixture.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:       []string{"Flow stays present"},
		Predictions:      predictions,
		AffectedFiles:    []string{maintExecAffectedFile},
		ValidUntil:       stale,
		GovernanceMode:   "exact",
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
	ripe := time.Now().Add(-48 * time.Hour).Format("2006-01-02")

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Maintenance executor fixture decision",
		ProblemStatement: "The governed flow needs a machine-checkable maintenance path.",
		WhySelected:      "E2E fixture for the maintenance execute-phase.",
		SelectionPolicy:  "Fixture.",
		CounterArgument:  "Fixture.",
		WeakestLink:      "Fixture.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:       []string{"Flow stays present"},
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

func createDrainJudgmentDecision(t *testing.T, root string) string {
	t.Helper()

	writeGoldenE2EFile(t, filepath.Join(root, "internal/app/judgment.go"),
		"package app\n\nfunc Judgment() string {\n\treturn \"operator\"\n}\n")

	database, store := openGoldenE2EStore(t, root)
	defer database.Close()

	ctx := context.Background()
	haftDir := filepath.Join(root, ".haft")
	ripe := time.Now().Add(-48 * time.Hour).Format("2006-01-02")
	stale := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		SelectedTitle:    "Drain needs operator fixture decision",
		ProblemStatement: "Judgment-only maintenance must remain an explicit operator task.",
		WhySelected:      "E2E fixture for operator-only maintenance tasks.",
		SelectionPolicy:  "Fixture.",
		CounterArgument:  "Fixture.",
		WeakestLink:      "Fixture.",
		WhyNotOthers:     []artifact.RejectionReason{{Variant: "none", Reason: "fixture"}},
		Rollback:         &artifact.RollbackSpec{Triggers: []string{"fixture"}},
		Invariants:       []string{"Judgment remains operator-reviewed"},
		Predictions: []artifact.PredictionInput{{
			Claim:       "Operator confidence remains high",
			Observable:  "operator interview",
			Threshold:   "human judgment supports",
			VerifyAfter: ripe,
		}},
		AffectedFiles:  []string{"internal/app/judgment.go"},
		ValidUntil:     stale,
		GovernanceMode: "exact",
	})
	if err != nil {
		t.Fatalf("decide judgment fixture: %v", err)
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

func assertMaintenanceDrainAuthorityBoundary(t *testing.T, boundary maintenanceDrainAuthority) {
	t.Helper()
	if boundary.Trigger != "explicit_h_verify_or_overseer_drain" {
		t.Fatalf("drain trigger boundary = %#v", boundary)
	}
	if boundary.Approval != "not_semantic_approval" {
		t.Fatalf("drain approval boundary = %#v", boundary)
	}
	if boundary.GateDecision != "not_gate_decision" {
		t.Fatalf("drain gate boundary = %#v", boundary)
	}
	if boundary.ClaimTruth != "not_claim_truth" {
		t.Fatalf("drain claim truth boundary = %#v", boundary)
	}
	if boundary.GlobalTruth != "not_global_truth" {
		t.Fatalf("drain global truth boundary = %#v", boundary)
	}
	if boundary.Publication != "not_publication" {
		t.Fatalf("drain publication boundary = %#v", boundary)
	}
}

func machineEvidenceCount(t *testing.T, ctx context.Context, store *artifact.Store, decisionID string) int {
	t.Helper()
	items, err := store.GetEvidenceItems(ctx, decisionID)
	if err != nil {
		t.Fatalf("get evidence: %v", err)
	}
	count := 0
	for _, item := range items {
		if item.Provenance == artifact.ProvenanceMachine {
			count++
		}
	}
	return count
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
