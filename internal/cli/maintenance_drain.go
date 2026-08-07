package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/present"
)

const maintenanceDrainSchemaVersion = "maintenance_drain.v1"

type maintenanceDrainReport struct {
	SchemaVersion             string                                       `json:"schema_version"`
	View                      string                                       `json:"view,omitempty"`
	CreatedAt                 string                                       `json:"created_at"`
	DryRun                    bool                                         `json:"dry_run"`
	MaintenanceRunID          string                                       `json:"maintenance_run_id,omitempty"`
	AuthorityBoundary         maintenanceDrainAuthority                    `json:"authority_boundary"`
	Summary                   maintenanceDrainSummary                      `json:"summary"`
	Executed                  []overseer.MaintenanceAction                 `json:"executed,omitempty"`
	OmittedExecuted           int                                          `json:"omitted_executed,omitempty"`
	ReconciliationProposals   []overseer.MaintenanceReconciliationProposal `json:"reconciliation_proposals,omitempty"`
	OmittedReconciliation     int                                          `json:"omitted_reconciliation_proposals,omitempty"`
	ReconciliationSummary     *overseer.MaintenanceReconciliationSummary   `json:"reconciliation_summary,omitempty"`
	AfterAction               overseer.MaintenanceAfterActionReport        `json:"after_action"`
	OmittedAfterAction        int                                          `json:"omitted_after_action_remaining_operator_judgment,omitempty"`
	NeedsOperator             []artifact.MaintenanceJudgmentGroup          `json:"needs_operator,omitempty"`
	OmittedNeedsOperatorTasks int                                          `json:"omitted_needs_operator_tasks,omitempty"`
	NextOperatorAction        []string                                     `json:"next_operator_action,omitempty"`
	FullAuditCommand          string                                       `json:"full_audit_command,omitempty"`
}

type maintenanceDrainAuthority struct {
	Trigger      string `json:"trigger"`
	Mutation     string `json:"mutation"`
	Approval     string `json:"approval"`
	Evidence     string `json:"evidence"`
	GateDecision string `json:"gate_decision"`
	ClaimTruth   string `json:"claim_truth"`
	GlobalTruth  string `json:"global_truth"`
	Publication  string `json:"publication"`
}

type maintenanceDrainSummary struct {
	BeforeTasks                 int `json:"before_tasks"`
	BeforeJudgment              int `json:"before_judgment"`
	ExecutedActions             int `json:"executed_actions"`
	AppliedActions              int `json:"applied_actions"`
	EvidenceActions             int `json:"evidence_actions"`
	ProposedActions             int `json:"proposed_actions"`
	FailedActions               int `json:"failed_actions"`
	NeedsOperatorTasks          int `json:"needs_operator_tasks"`
	ReconciliationProposalCount int `json:"reconciliation_proposal_count,omitempty"`
}

func buildMaintenanceDrainReport(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	dryRun bool,
) (maintenanceDrainReport, error) {
	beforePlan, err := artifact.BuildMaintenancePlan(ctx, store, projectRoot)
	if err != nil {
		return maintenanceDrainReport{}, err
	}

	cfg := explicitDrainConfig(dryRun)
	executed := executeMaintenancePlan(ctx, store, projectRoot, cfg)

	run, err := buildMaintenanceRunAfterDrain(ctx, store, projectRoot, executed)
	if err != nil {
		return maintenanceDrainReport{}, err
	}
	if !dryRun {
		if err := overseer.StoreMaintenanceRun(projectRoot, run); err != nil {
			return maintenanceDrainReport{}, err
		}
	}

	afterPlan, err := artifact.BuildMaintenancePlan(ctx, store, projectRoot)
	if err != nil {
		return maintenanceDrainReport{}, err
	}
	review := artifact.BuildMaintenanceJudgmentReview(afterPlan)

	report := maintenanceDrainReport{
		SchemaVersion:           maintenanceDrainSchemaVersion,
		CreatedAt:               time.Now().UTC().Format(time.RFC3339),
		DryRun:                  dryRun,
		AuthorityBoundary:       maintenanceDrainAuthorityFor(dryRun),
		Summary:                 maintenanceDrainSummaryFor(beforePlan, run, review),
		Executed:                maintenanceDrainActionsWithUndo(run),
		ReconciliationProposals: run.ReconciliationProposals,
		ReconciliationSummary:   run.ReconciliationSummary,
		AfterAction:             run.AfterAction,
		NeedsOperator:           review.Groups,
		NextOperatorAction:      maintenanceDrainNextActions(review),
	}
	if !dryRun {
		report.MaintenanceRunID = run.MaintenanceID
	}
	return report, nil
}

func compactMaintenanceDrainReport(
	report maintenanceDrainReport,
	limit int,
) maintenanceDrainReport {
	compact := report
	compact.View = "compact"
	compact.FullAuditCommand = "haft overseer drain --dry-run --json --full"
	if !compact.DryRun {
		compact.FullAuditCommand = "haft overseer drain --json --full"
	}
	compact.Executed, compact.OmittedExecuted = limitMaintenanceDrainSlice(compact.Executed, limit)
	compact.ReconciliationProposals, compact.OmittedReconciliation = limitMaintenanceDrainSlice(compact.ReconciliationProposals, limit)
	compact.NeedsOperator, compact.OmittedNeedsOperatorTasks = compactMaintenanceDrainNeedsOperator(compact.NeedsOperator, limit)
	compact.AfterAction.RemainingOperatorJudgment, compact.OmittedAfterAction = limitMaintenanceDrainSlice(
		compact.AfterAction.RemainingOperatorJudgment,
		limit,
	)
	return compact
}

func compactMaintenanceDrainNeedsOperator(
	groups []artifact.MaintenanceJudgmentGroup,
	limit int,
) ([]artifact.MaintenanceJudgmentGroup, int) {
	if limit <= 0 {
		compact := append([]artifact.MaintenanceJudgmentGroup(nil), groups...)
		omittedTasks := 0
		for index := range compact {
			group := &compact[index]
			group.OmittedTasks += len(group.Tasks)
			omittedTasks += group.OmittedTasks
			group.Tasks = []artifact.MaintenanceJudgmentTaskReview{}
		}
		return compact, omittedTasks
	}

	review := artifact.CompactMaintenanceJudgmentReview(
		&artifact.MaintenanceJudgmentReview{Groups: groups},
		limit,
	)
	if review == nil {
		return nil, 0
	}
	return review.Groups, review.OmittedJudgmentTasks
}

func limitMaintenanceDrainSlice[T any](
	items []T,
	limit int,
) ([]T, int) {
	if limit < 0 {
		limit = 0
	}
	if len(items) <= limit {
		return append([]T(nil), items...), 0
	}
	return append([]T(nil), items[:limit]...), len(items) - limit
}

func maintenanceDrainActionsWithUndo(run overseer.MaintenanceRun) []overseer.MaintenanceAction {
	actions := append([]overseer.MaintenanceAction(nil), run.Executed...)
	if run.MaintenanceID == "" {
		return actions
	}
	for i := range actions {
		if actions[i].PriorState == "" {
			continue
		}
		actions[i].Undo = "haft overseer undo " + run.MaintenanceID + " " + actions[i].ID
	}
	return actions
}

func explicitDrainConfig(dryRun bool) overseer.Config {
	cfg := overseer.DefaultConfig()
	cfg.Enabled = true
	if dryRun {
		cfg.MaintenanceRebaseline = overseer.MaintenanceModePropose
		cfg.MaintenanceRevalidateStale = overseer.MaintenanceModePropose
		return cfg
	}
	cfg.MaintenanceRebaseline = overseer.MaintenanceModeAuto
	cfg.MaintenanceRevalidateStale = overseer.MaintenanceModeAuto
	return cfg
}

func buildMaintenanceRunAfterDrain(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	executed []overseer.MaintenanceAction,
) (overseer.MaintenanceRun, error) {
	report, err := buildCheckReport(ctx, store, projectRoot)
	if err != nil {
		return overseer.MaintenanceRun{}, err
	}
	return overseer.BuildMaintenanceRun(overseer.MaintenanceInput{
		CreatedAt:               time.Now().UTC().Format(time.RFC3339),
		Stale:                   mapMaintenanceStale(report.Stale),
		Drift:                   mapMaintenanceDrift(report.Drifted),
		SpecHealth:              mapMaintenanceSpecHealth(report.SpecHealth),
		CoverageGaps:            mapMaintenanceCoverage(report.CoverageGaps),
		Executed:                executed,
		ReconciliationProposals: buildMaintenanceReconciliationProposals(ctx, store),
	})
}

func maintenanceDrainAuthorityFor(dryRun bool) maintenanceDrainAuthority {
	mutation := "machine_safe_only"
	evidence := "machine_evidence_only"
	if dryRun {
		mutation = "not_mutation"
		evidence = "not_evidence"
	}
	return maintenanceDrainAuthority{
		Trigger:      "explicit_h_verify_or_overseer_drain",
		Mutation:     mutation,
		Approval:     "not_semantic_approval",
		Evidence:     evidence,
		GateDecision: "not_gate_decision",
		ClaimTruth:   "not_claim_truth",
		GlobalTruth:  "not_global_truth",
		Publication:  "not_publication",
	}
}

func maintenanceDrainSummaryFor(
	beforePlan *artifact.MaintenancePlan,
	run overseer.MaintenanceRun,
	review *artifact.MaintenanceJudgmentReview,
) maintenanceDrainSummary {
	summary := maintenanceDrainSummary{
		ExecutedActions:             len(run.Executed),
		ReconciliationProposalCount: len(run.ReconciliationProposals),
	}
	if beforePlan != nil {
		summary.BeforeTasks = len(beforePlan.Tasks)
		summary.BeforeJudgment = beforePlan.JudgmentNeeded
	}
	if review != nil {
		summary.NeedsOperatorTasks = review.JudgmentTasks
	}
	for _, action := range run.Executed {
		switch action.Outcome {
		case "applied":
			summary.AppliedActions++
		case "evidence_attached":
			summary.EvidenceActions++
		case "proposed":
			summary.ProposedActions++
		case "failed":
			summary.FailedActions++
		}
	}
	return summary
}

func maintenanceDrainNextActions(review *artifact.MaintenanceJudgmentReview) []string {
	if review == nil || review.JudgmentTasks == 0 {
		return []string{"No operator judgment tasks remain in the post-drain maintenance plan."}
	}
	return []string{
		"Review needs_operator groups before any baseline, waive, reopen, or supersede.",
		"Use `haft_refresh(action=\"review\")` or `haft overseer judgment --json --limit 20` for bounded task drill-down.",
	}
}

func (r maintenanceDrainReport) GetMaintenanceDrainFields() present.MaintenanceDrainFields {
	return present.MaintenanceDrainFields{
		DryRun:             r.DryRun,
		MaintenanceRunID:   r.MaintenanceRunID,
		Mutation:           r.AuthorityBoundary.Mutation,
		Approval:           r.AuthorityBoundary.Approval,
		Evidence:           r.AuthorityBoundary.Evidence,
		GateDecision:       r.AuthorityBoundary.GateDecision,
		ClaimTruth:         r.AuthorityBoundary.ClaimTruth,
		GlobalTruth:        r.AuthorityBoundary.GlobalTruth,
		Publication:        r.AuthorityBoundary.Publication,
		ExecutedActions:    r.Summary.ExecutedActions,
		AppliedActions:     r.Summary.AppliedActions,
		EvidenceActions:    r.Summary.EvidenceActions,
		ProposedActions:    r.Summary.ProposedActions,
		FailedActions:      r.Summary.FailedActions,
		NeedsOperatorTasks: r.Summary.NeedsOperatorTasks,
		ExecutedLines:      maintenanceDrainActionLines(r.Executed),
		NeedsOperatorLines: maintenanceDrainNeedLines(r.NeedsOperator),
		NextActions:        r.NextOperatorAction,
	}
}

func maintenanceDrainActionLines(actions []overseer.MaintenanceAction) []string {
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
		line := fmt.Sprintf("**%s** `%s` — `%s` %s: %s",
			maintenanceDrainDecisionTitle(action),
			action.DecisionRef,
			action.Kind,
			action.Outcome,
			action.Detail)
		if action.Undo != "" {
			line += " · undo: `" + action.Undo + "`"
		}
		lines = append(lines, line)
	}
	return lines
}

func maintenanceDrainDecisionTitle(action overseer.MaintenanceAction) string {
	if action.Title != "" {
		return action.Title
	}
	return "Untitled decision"
}

func maintenanceDrainNeedLines(groups []artifact.MaintenanceJudgmentGroup) []string {
	lines := make([]string, 0, len(groups))
	for _, group := range groups {
		lines = append(lines, fmt.Sprintf("%s/%s: %d task(s) — %s",
			group.Recommendation,
			group.Confidence,
			group.TaskCount,
			group.SuggestedAction))
	}
	return lines
}
