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
	decisionGoverningSetCmd.Flags().BoolVar(&decisionGoverningSetJSON, "json", false, "print structured JSON output")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetQuery, "query", "", "filter governing sets by substring across subject, target, decision refs, and repair hints")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetSubjectRef, "subject-ref", "", "filter governing sets by exact subject ref")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetTargetRef, "target-ref", "", "filter governing sets by exact target ref")
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
	}
	if len(plan.Groups) > limit {
		_, err := fmt.Fprintf(output, "... and %d more; run `haft decision reconcile --json`\n", len(plan.Groups)-limit)
		return err
	}
	return nil
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
