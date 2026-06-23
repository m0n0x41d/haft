package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

var (
	decisionReconcileJSON          bool
	decisionReconcileApplyJSON     bool
	decisionReconcileMetricsJSON   bool
	decisionReconcileDraftJSON     bool
	decisionReconcileReviewJSON    bool
	decisionReconcileDraftLimit    int
	decisionReconcileDraftGroupID  string
	decisionReconcileDraftDecision string
	decisionGoverningSetJSON       bool
	decisionGoverningSetQuery      string
	decisionGoverningSetSubjectRef string
	decisionGoverningSetTargetRef  string
)

var decisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Inspect decision authority and reconciliation projections",
}

var decisionReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Build a read-only decision reconciliation plan",
	Long: `Build a deterministic report-only DecisionReconciliationPlan.

The plan groups current DecisionRecords by explicit decision subject, bounded
context, and governance-target overlap. File overlap alone is never merge or
supersede evidence. This command does not mutate decisions, links, evidence,
baselines, or carriers.`,
	RunE: runDecisionReconcile,
}

var decisionReconcileApplyCmd = &cobra.Command{
	Use:   "apply SELECTION_JSON",
	Short: "Apply an operator-approved decision reconciliation selection",
	Long: `Apply an explicit operator-approved reconciliation selection document.

This mutates decision lineage/status using existing lifecycle APIs. The
selection document must include authority=operator_approved_reconciliation_selection,
operator_approval_ref, reviewed_group_id, decision_refs, operation, and reason.
operation=enrich_scope may add explicit decision_subject_ref,
governance_targets, drift_watch_targets, and claim_governance_target_refs
without changing decision status or lineage. operation=claim_lifecycle_update
may update explicit claims to refresh_due, superseded, or deprecated while the
DecisionRecord remains current. MCP does not get an auto-apply path in this
slice.`,
	Args: cobra.ExactArgs(1),
	RunE: runDecisionReconcileApply,
}

var decisionReconcileMetricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Capture read-only reconciliation before/after metrics",
	Long: `Capture the current read-only metrics packet used to compare old-decision
scope-enrichment cleanup before and after an operator-approved reconciliation
apply. This command does not mutate decisions, links, evidence, baselines, or
carriers.`,
	RunE: runDecisionReconcileMetrics,
}

var decisionReconcileSelectionDraftCmd = &cobra.Command{
	Use:   "selection-draft",
	Short: "Build a read-only reconciliation selection draft",
	Long: `Build a read-only operator-review draft from reconciliation scope-enrichment
candidates. The draft is not an approval document and cannot be applied as-is;
apply still requires a separate selection with
authority=operator_approved_reconciliation_selection.`,
	RunE: runDecisionReconcileSelectionDraft,
}

var decisionReconcileSelectionReviewCmd = &cobra.Command{
	Use:   "selection-review SELECTION_JSON",
	Short: "Review a reconciliation selection document without applying it",
	Long: `Review a reconciliation selection document against the same core validation
used by apply. This command is read-only: it does not create operator approval
and does not mutate decisions, links, evidence, baselines, gates, or carriers.`,
	Args: cobra.ExactArgs(1),
	RunE: runDecisionReconcileSelectionReview,
}

var decisionGoverningSetCmd = &cobra.Command{
	Use:   "governing-set",
	Short: "Build a read-only current governing set projection",
	Long: `Build the current governing authority frontier.

The projection groups active/refresh_due DecisionRecords by explicit subject,
bounded context, and effective governance/drift target. Superseded/deprecated
records remain lineage history, not current authority. This command is
read-only and does not resolve conflicts.

Use --query, --subject-ref, or --target-ref to answer "what currently governs
this symbol / contract / spec section" without expanding default status.`,
	RunE: runDecisionGoverningSet,
}

func init() {
	decisionReconcileCmd.Flags().BoolVar(&decisionReconcileJSON, "json", false, "print structured JSON output")
	decisionReconcileApplyCmd.Flags().BoolVar(&decisionReconcileApplyJSON, "json", false, "print structured JSON output")
	decisionReconcileMetricsCmd.Flags().BoolVar(&decisionReconcileMetricsJSON, "json", false, "print structured JSON output")
	decisionReconcileSelectionDraftCmd.Flags().BoolVar(&decisionReconcileDraftJSON, "json", false, "print structured JSON output")
	decisionReconcileSelectionReviewCmd.Flags().BoolVar(&decisionReconcileReviewJSON, "json", false, "print structured JSON output")
	decisionReconcileSelectionDraftCmd.Flags().IntVar(&decisionReconcileDraftLimit, "limit", 0, "limit emitted draft candidates without approving or applying them")
	decisionReconcileSelectionDraftCmd.Flags().StringVar(&decisionReconcileDraftGroupID, "group-id", "", "emit draft candidates only for this reconciliation group id")
	decisionReconcileSelectionDraftCmd.Flags().StringVar(&decisionReconcileDraftDecision, "decision-ref", "", "emit draft candidates only for this decision ref")
	decisionGoverningSetCmd.Flags().BoolVar(&decisionGoverningSetJSON, "json", false, "print structured JSON output")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetQuery, "query", "", "filter governing sets by substring across subject, target, decision refs, and repair hints")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetSubjectRef, "subject-ref", "", "filter governing sets by exact subject ref")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetTargetRef, "target-ref", "", "filter governing sets by exact target ref")
	decisionReconcileCmd.AddCommand(decisionReconcileMetricsCmd)
	decisionReconcileCmd.AddCommand(decisionReconcileSelectionDraftCmd)
	decisionReconcileCmd.AddCommand(decisionReconcileSelectionReviewCmd)
	decisionReconcileCmd.AddCommand(decisionReconcileApplyCmd)
	decisionCmd.AddCommand(decisionGoverningSetCmd)
	decisionCmd.AddCommand(decisionReconcileCmd)
	rootCmd.AddCommand(decisionCmd)
}

func runDecisionReconcile(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	plan, err := artifact.BuildDecisionReconciliationPlan(context.Background(), store)
	if err != nil {
		return fmt.Errorf("build decision reconciliation plan: %w", err)
	}

	if decisionReconcileJSON {
		return writeJSON(cmd.OutOrStdout(), plan)
	}

	return writeDecisionReconciliationSummary(cmd.OutOrStdout(), plan)
}

func runDecisionReconcileSelectionDraft(cmd *cobra.Command, _ []string) error {
	if decisionReconcileDraftLimit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	plan, err := artifact.BuildDecisionReconciliationPlan(context.Background(), store)
	if err != nil {
		return fmt.Errorf("build decision reconciliation plan: %w", err)
	}
	draft := artifact.BuildDecisionReconciliationSelectionDraftFiltered(
		plan,
		artifact.DecisionReconciliationSelectionDraftFilter{
			Limit:       decisionReconcileDraftLimit,
			GroupID:     decisionReconcileDraftGroupID,
			DecisionRef: decisionReconcileDraftDecision,
		},
	)
	if decisionReconcileDraftJSON {
		return writeJSON(cmd.OutOrStdout(), draft)
	}
	return writeDecisionReconciliationSelectionDraftSummary(cmd.OutOrStdout(), draft)
}

func runDecisionReconcileSelectionReview(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	document, err := readDecisionReconciliationSelectionDocument(args[0])
	if err != nil {
		return err
	}
	review := artifact.ReviewDecisionReconciliationSelectionDocument(
		context.Background(),
		store,
		document,
		args[0],
	)
	if decisionReconcileReviewJSON {
		return writeJSON(cmd.OutOrStdout(), review)
	}
	return writeDecisionReconciliationSelectionReviewSummary(cmd.OutOrStdout(), review)
}

func runDecisionGoverningSet(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	report, err := artifact.BuildCurrentGoverningSetReportFiltered(context.Background(), store, artifact.CurrentGoverningSetFilter{
		Query:      decisionGoverningSetQuery,
		SubjectRef: decisionGoverningSetSubjectRef,
		TargetRef:  decisionGoverningSetTargetRef,
	})
	if err != nil {
		return fmt.Errorf("build current governing set: %w", err)
	}
	if decisionGoverningSetJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}
	return writeCurrentGoverningSetSummary(cmd.OutOrStdout(), report)
}

func runDecisionReconcileApply(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	document, err := readDecisionReconciliationSelectionDocument(args[0])
	if err != nil {
		return err
	}
	result, err := artifact.ApplyDecisionReconciliationSelections(
		context.Background(),
		store,
		filepath.Join(projectRoot, ".haft"),
		document,
	)
	if err != nil {
		return fmt.Errorf("apply decision reconciliation selection: %w", err)
	}

	if decisionReconcileApplyJSON {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	return writeDecisionReconciliationApplySummary(cmd.OutOrStdout(), result)
}

func runDecisionReconcileMetrics(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return err
	}
	defer closeFn()

	ctx := context.Background()
	plan, err := artifact.BuildDecisionReconciliationPlan(ctx, store)
	if err != nil {
		return fmt.Errorf("build decision reconciliation plan: %w", err)
	}
	governing, err := artifact.BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		return fmt.Errorf("build current governing set: %w", err)
	}
	driftReports, err := artifact.CheckDrift(ctx, store, projectRoot)
	if err != nil {
		return fmt.Errorf("check drift for reconciliation metrics: %w", err)
	}
	packet := artifact.BuildReconciliationMetricsPacket(
		plan,
		governing,
		artifact.BuildDriftEventReport(driftReports),
	)

	if decisionReconcileMetricsJSON {
		return writeJSON(cmd.OutOrStdout(), packet)
	}
	return writeDecisionReconciliationMetricsSummary(cmd.OutOrStdout(), packet)
}

func readDecisionReconciliationSelectionDocument(
	path string,
) (artifact.DecisionReconciliationSelectionDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifact.DecisionReconciliationSelectionDocument{}, fmt.Errorf("read decision reconciliation selection %s: %w", path, err)
	}
	var document artifact.DecisionReconciliationSelectionDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return artifact.DecisionReconciliationSelectionDocument{}, fmt.Errorf("parse decision reconciliation selection %s: %w", path, err)
	}
	return document, nil
}

func writeDecisionReconciliationSummary(
	output io.Writer,
	plan artifact.DecisionReconciliationPlan,
) error {
	if _, err := fmt.Fprintf(output, "Decision reconciliation plan v%d\n", plan.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", plan.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "reviewed_decisions: %d\n", plan.Summary.ReviewedDecisions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "groups: %d keep=%d reopen=%d merge=%d supersede=%d retire=%d conflict=%d\n",
		plan.Summary.Groups,
		plan.Summary.Keep,
		plan.Summary.ReopenCandidates,
		plan.Summary.MergeCandidates,
		plan.Summary.SupersedeCandidates,
		plan.Summary.RetireWithoutSuccessorCandidates,
		plan.Summary.ConflictRequiresOperator,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "missing_explicit_subject: %d\n", plan.Summary.MissingExplicitSubject); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "whole_file_fallback_only: %d\n", plan.Summary.WholeFileFallbackOnly); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "scope_enrichment_candidates: %d\n", plan.Summary.ScopeEnrichmentCandidates); err != nil {
		return err
	}
	if len(plan.Groups) == 0 {
		_, err := fmt.Fprintln(output, "no current decisions to reconcile")
		return err
	}

	limit := len(plan.Groups)
	if limit > 5 {
		limit = 5
	}
	if _, err := fmt.Fprintf(output, "top_groups:\n"); err != nil {
		return err
	}
	for _, group := range plan.Groups[:limit] {
		if _, err := fmt.Fprintf(output,
			"- %s preview=%s fanout=%d subject=%s context=%s refs=%v\n",
			group.Category,
			group.Preview.Operation,
			len(group.DecisionRefs),
			truncateDecisionReconciliationSummaryField(group.SubjectRef),
			truncateDecisionReconciliationSummaryField(group.BoundedContext),
			group.DecisionRefs,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(output, "  preview_cues: %s\n", decisionReconciliationPreviewCueSummary(group)); err != nil {
			return err
		}
	}
	if len(plan.Groups) > limit {
		_, err := fmt.Fprintf(output, "... and %d more; run `haft decision reconcile --json`\n", len(plan.Groups)-limit)
		return err
	}
	return nil
}

func decisionReconciliationPreviewCueSummary(group artifact.DecisionReconciliationGroup) string {
	preview := group.Preview
	downstreamDependents := 0
	downstreamMigrationRequired := false
	if preview.DownstreamImpact != nil {
		downstreamDependents = len(preview.DownstreamImpact.DependentRefs)
	}
	if preview.DownstreamMigration != nil {
		downstreamMigrationRequired = preview.DownstreamMigration.RequiredBeforeApply
	}

	successorWorkflow := "none"
	if preview.SuccessorWorkflow != nil && preview.SuccessorWorkflow.Required {
		successorWorkflow = "required_existing_ref"
	}

	return fmt.Sprintf(
		"read_only=%t lineage_relations=%d downstream_dependents=%d downstream_migration_required=%t successor_workflow=%s claim_lifecycle=%d",
		preview.ReadOnly,
		len(preview.Proposed.LineageRelations),
		downstreamDependents,
		downstreamMigrationRequired,
		successorWorkflow,
		decisionReconciliationClaimLifecycleCount(group.Decisions),
	)
}

func decisionReconciliationClaimLifecycleCount(items []artifact.DecisionReconciliationItem) int {
	count := 0
	for _, item := range items {
		if item.ClaimLifecycle == nil {
			continue
		}
		count += item.ClaimLifecycle.Active
		count += item.ClaimLifecycle.RefreshDue
		count += item.ClaimLifecycle.Superseded
		count += item.ClaimLifecycle.Deprecated
	}
	return count
}

func writeDecisionReconciliationMetricsSummary(
	output io.Writer,
	packet artifact.ReconciliationMetricsPacket,
) error {
	if _, err := fmt.Fprintf(output, "Decision reconciliation metrics v%d\n", packet.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", packet.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "capture_policy: %s\n", packet.CapturePolicy); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output,
		"reconciliation: reviewed=%d groups=%d whole_file_fallback_only=%d missing_subject=%d scope_enrichment=%d conflicts=%d\n",
		packet.Reconciliation.ReviewedDecisions,
		packet.Reconciliation.Groups,
		packet.Reconciliation.WholeFileFallbackOnly,
		packet.Reconciliation.MissingExplicitSubject,
		packet.Reconciliation.ScopeEnrichmentCandidates,
		packet.Reconciliation.ConflictRequiresOperator,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output,
		"governing_set: current=%d sets=%d fallback_sets=%d scope_enrichment_sets=%d conflicts=%d overlap=%d terminal_history=%d\n",
		packet.GoverningSet.CurrentDecisions,
		packet.GoverningSet.GoverningSets,
		packet.GoverningSet.FallbackTargetSets,
		packet.GoverningSet.ScopeEnrichmentSets,
		packet.GoverningSet.ConflictSets,
		packet.GoverningSet.OverlapReviewSets,
		packet.GoverningSet.TerminalHistoryRefs,
	); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output,
		"drift_events: unique=%d impacted=%d material=%d audit_only=%d needs_binding=%d semantic=%d file_fallback=%d unknown_high_risk=%d max_fanout=%d\n",
		packet.DriftEvents.UniqueEvents,
		packet.DriftEvents.ImpactedDecisions,
		packet.DriftEvents.MaterialEvents,
		packet.DriftEvents.AuditOnlyEvents,
		packet.DriftEvents.NeedsBindingResolutionEvents,
		packet.DriftEvents.SemanticTargetEvents,
		packet.DriftEvents.FileFallbackEvents,
		packet.DriftEvents.UnknownHighRiskEvents,
		packet.DriftEvents.MaxFanout,
	)
	return err
}

func writeCurrentGoverningSetSummary(
	output io.Writer,
	report artifact.CurrentGoverningSetReport,
) error {
	if _, err := fmt.Fprintf(output, "Current governing set v%d\n", report.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", report.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "current_decisions: %d\n", report.Summary.CurrentDecisions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority_frontier: current_refs=%d terminal_history_refs=%d boundary=%s\n",
		len(report.AuthorityFrontier.CurrentDecisionRefs),
		len(report.AuthorityFrontier.TerminalHistoryRefs),
		truncateDecisionReconciliationSummaryField(report.AuthorityFrontier.AuthorityBoundary),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "sets: %d conflicts=%d overlaps=%d terminal_history=%d missing_subject=%d fallback_sets=%d scope_enrichment_sets=%d\n",
		report.Summary.GoverningSets,
		report.Summary.ConflictSets,
		report.Summary.OverlapReviewSets,
		report.Summary.TerminalHistoryRefs,
		report.Summary.MissingExplicitSubject,
		report.Summary.FallbackTargetSets,
		report.Summary.ScopeEnrichmentSets,
	); err != nil {
		return err
	}
	if len(report.Sets) == 0 {
		_, err := fmt.Fprintln(output, "no current governing decisions")
		return err
	}
	limit := len(report.Sets)
	if limit > 5 {
		limit = 5
	}
	if _, err := fmt.Fprintln(output, "top_sets:"); err != nil {
		return err
	}
	for _, set := range report.Sets[:limit] {
		if _, err := fmt.Fprintf(output,
			"- %s decisions=%d subject=%s target=%s refs=%v\n",
			set.Posture,
			len(set.CurrentDecisionRefs),
			truncateDecisionReconciliationSummaryField(set.SubjectRef),
			truncateDecisionReconciliationSummaryField(set.TargetRef),
			set.CurrentDecisionRefs,
		); err != nil {
			return err
		}
	}
	if len(report.Sets) > limit {
		_, err := fmt.Fprintf(output, "... and %d more; run `haft decision governing-set --json`\n", len(report.Sets)-limit)
		return err
	}
	return nil
}

func writeDecisionReconciliationSelectionDraftSummary(
	output io.Writer,
	draft artifact.DecisionReconciliationSelectionDraft,
) error {
	if _, err := fmt.Fprintf(output, "Decision reconciliation selection draft v%d\n", draft.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", draft.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "operator_approved: %t\n", draft.OperatorApproved); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "apply_authority_required: %s\n", draft.ApplyAuthorityRequired); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "scope_enrichment_candidates: %d\n", draft.Summary.ScopeEnrichmentCandidates); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "operator_approval_candidates: %d\n", draft.Summary.OperatorApprovalCandidates); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "selected_candidates: %d\n", draft.Summary.SelectedCandidates); err != nil {
		return err
	}
	if len(draft.Items) == 0 {
		_, err := fmt.Fprintln(output, "no scope enrichment candidates")
		return err
	}
	limit := len(draft.Items)
	if limit > 5 {
		limit = 5
	}
	if _, err := fmt.Fprintln(output, "top_candidates:"); err != nil {
		return err
	}
	for _, item := range draft.Items[:limit] {
		if _, err := fmt.Fprintf(output,
			"- %s group=%s confidence=%s posture=%s files=%d hint=%s\n",
			item.DecisionRef,
			item.ReviewedGroupID,
			item.Confidence,
			item.CandidatePosture,
			len(item.AffectedFiles),
			truncateDecisionReconciliationSummaryField(item.ScopeRepairHint),
		); err != nil {
			return err
		}
	}
	if len(draft.Items) > limit {
		_, err := fmt.Fprintf(output, "... and %d more; run `haft decision reconcile selection-draft --json`\n", len(draft.Items)-limit)
		return err
	}
	return nil
}

func writeDecisionReconciliationSelectionReviewSummary(
	output io.Writer,
	review artifact.DecisionReconciliationSelectionReview,
) error {
	if _, err := fmt.Fprintf(output, "Decision reconciliation selection review v%d\n", review.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", review.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "document_authority: %s\n", review.DocumentAuthority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "operator_approved: %t\n", review.OperatorApproved); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "apply_ready: %t\n", review.ApplyReady); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "items: %d\n", review.ItemCount); err != nil {
		return err
	}
	if len(review.ValidationErrors) > 0 {
		if _, err := fmt.Fprintln(output, "validation_errors:"); err != nil {
			return err
		}
		for _, validationError := range review.ValidationErrors {
			if _, err := fmt.Fprintf(output, "- %s\n", validationError); err != nil {
				return err
			}
		}
	}
	for _, item := range review.Items {
		if len(item.ValidationErrors) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(output, "item[%d] errors:\n", item.Index); err != nil {
			return err
		}
		for _, validationError := range item.ValidationErrors {
			if _, err := fmt.Fprintf(output, "- %s\n", validationError); err != nil {
				return err
			}
		}
	}
	if len(review.MutationBoundary) > 0 {
		if _, err := fmt.Fprintln(output, "mutation_boundary:"); err != nil {
			return err
		}
		for _, boundary := range review.MutationBoundary {
			if _, err := fmt.Fprintf(output, "- %s\n", boundary); err != nil {
				return err
			}
		}
	}
	if review.ApplyCommand != "" {
		if _, err := fmt.Fprintf(output, "apply_command: %s\n", review.ApplyCommand); err != nil {
			return err
		}
	}
	if len(review.NextSteps) > 0 {
		if _, err := fmt.Fprintln(output, "next_steps:"); err != nil {
			return err
		}
		for _, step := range review.NextSteps {
			if _, err := fmt.Fprintf(output, "- %s\n", step); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeDecisionReconciliationApplySummary(
	output io.Writer,
	result artifact.DecisionReconciliationApplyResult,
) error {
	if _, err := fmt.Fprintf(output, "Decision reconciliation apply v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "applied: %d\n", len(result.Applied)); err != nil {
		return err
	}
	for _, item := range result.Applied {
		if _, err := fmt.Fprintf(output,
			"- %s refs=%v successor=%s problems=%v updated=%v status=%s\n",
			item.Operation,
			item.DecisionRefs,
			item.SuccessorRef,
			item.ProblemRefs,
			item.UpdatedFields,
			item.Status,
		); err != nil {
			return err
		}
		for _, relation := range item.LineageRelations {
			if _, err := fmt.Fprintf(output,
				"  lineage_relation: %s %s -> %s\n",
				relation.Relation,
				relation.SourceRef,
				relation.TargetRef,
			); err != nil {
				return err
			}
		}
		for _, update := range item.ClaimUpdates {
			if _, err := fmt.Fprintf(output,
				"  claim_update: decision=%s claim=%s lifecycle=%s successor=%s\n",
				update.DecisionRef,
				update.ClaimID,
				update.LifecycleStatus,
				update.SuccessorRef,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func truncateDecisionReconciliationSummaryField(value string) string {
	const maxLen = 96
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen-3] + "..."
}
