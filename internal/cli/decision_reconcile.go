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
	decisionReconcileDraftFull     bool
	decisionReconcileReviewJSON    bool
	decisionReconcileLimit         int
	decisionReconcileDraftLimit    int
	decisionReconcileDraftGroupID  string
	decisionReconcileDraftDecision string
	decisionReconcileDraftWrite    string
	decisionReconcileDraftReview   string
	decisionGoverningSetJSON       bool
	decisionGoverningSetLimit      int
	decisionGoverningSetQuery      string
	decisionGoverningSetSubjectRef string
	decisionGoverningSetTargetRef  string
	decisionGoverningSetWrite      string
	decisionGoverningSetCheck      string
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
	decisionReconcileCmd.Flags().IntVar(&decisionReconcileLimit, "limit", 0, "limit compact JSON groups without approving or applying anything; default 0 emits the full audit JSON")
	decisionReconcileApplyCmd.Flags().BoolVar(&decisionReconcileApplyJSON, "json", false, "print structured JSON output")
	decisionReconcileMetricsCmd.Flags().BoolVar(&decisionReconcileMetricsJSON, "json", false, "print structured JSON output")
	decisionReconcileSelectionDraftCmd.Flags().BoolVar(&decisionReconcileDraftJSON, "json", false, "print structured JSON output")
	decisionReconcileSelectionDraftCmd.Flags().BoolVar(&decisionReconcileDraftFull, "full", false, "emit every draft candidate; default output is bounded for review")
	decisionReconcileSelectionReviewCmd.Flags().BoolVar(&decisionReconcileReviewJSON, "json", false, "print structured JSON output")
	decisionReconcileSelectionDraftCmd.Flags().IntVar(&decisionReconcileDraftLimit, "limit", 0, "limit emitted draft candidates without approving or applying them")
	decisionReconcileSelectionDraftCmd.Flags().StringVar(&decisionReconcileDraftGroupID, "group-id", "", "emit draft candidates only for this reconciliation group id")
	decisionReconcileSelectionDraftCmd.Flags().StringVar(&decisionReconcileDraftDecision, "decision-ref", "", "emit draft candidates only for this decision ref")
	decisionReconcileSelectionDraftCmd.Flags().StringVar(&decisionReconcileDraftWrite, "write-template", "", "write the bounded selection document template to this JSON path without approving it")
	decisionReconcileSelectionDraftCmd.Flags().StringVar(&decisionReconcileDraftReview, "write-review-packet", "", "write the bounded report-only selection draft with review hints to this JSON path without approving it")
	decisionGoverningSetCmd.Flags().BoolVar(&decisionGoverningSetJSON, "json", false, "print structured JSON output")
	decisionGoverningSetCmd.Flags().IntVar(&decisionGoverningSetLimit, "limit", 0, "limit compact JSON sets without changing governing authority; default 0 emits the full audit JSON")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetQuery, "query", "", "filter governing sets by substring across subject, target, decision refs, and repair hints")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetSubjectRef, "subject-ref", "", "filter governing sets by exact subject ref")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetTargetRef, "target-ref", "", "filter governing sets by exact target ref")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetWrite, "write-snapshot", "", "write the current governing-set report to a JSON snapshot carrier")
	decisionGoverningSetCmd.Flags().StringVar(&decisionGoverningSetCheck, "check-snapshot", "", "compare the current governing-set snapshot digest with a JSON snapshot carrier")
	decisionReconcileCmd.AddCommand(decisionReconcileMetricsCmd)
	decisionReconcileCmd.AddCommand(decisionReconcileSelectionDraftCmd)
	decisionReconcileCmd.AddCommand(decisionReconcileSelectionReviewCmd)
	decisionReconcileCmd.AddCommand(decisionReconcileApplyCmd)
	decisionCmd.AddCommand(decisionGoverningSetCmd)
	decisionCmd.AddCommand(decisionReconcileCmd)
	rootCmd.AddCommand(decisionCmd)
}

func runDecisionReconcile(cmd *cobra.Command, _ []string) error {
	if decisionReconcileLimit < 0 {
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

	if decisionReconcileJSON {
		outputPlan := decisionReconciliationJSONProjection(plan)
		return writeJSON(cmd.OutOrStdout(), outputPlan)
	}

	return writeDecisionReconciliationSummary(cmd.OutOrStdout(), plan)
}

func decisionReconciliationJSONProjection(
	plan artifact.DecisionReconciliationPlan,
) artifact.DecisionReconciliationPlan {
	if decisionReconcileLimit <= 0 {
		return plan
	}
	return artifact.CompactDecisionReconciliationPlan(plan, decisionReconcileLimit)
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
			Limit:       decisionReconcileSelectionDraftLimit(),
			Full:        decisionReconcileDraftFull,
			GroupID:     decisionReconcileDraftGroupID,
			DecisionRef: decisionReconcileDraftDecision,
		},
	)
	if decisionReconcileDraftWrite != "" {
		if err := writeDecisionReconciliationSelectionTemplateFile(decisionReconcileDraftWrite, draft); err != nil {
			return err
		}
	}
	if decisionReconcileDraftReview != "" {
		if err := writeDecisionReconciliationSelectionDraftFile(decisionReconcileDraftReview, draft); err != nil {
			return err
		}
	}
	if decisionReconcileDraftJSON {
		return writeJSON(cmd.OutOrStdout(), draft)
	}
	if decisionReconcileDraftWrite != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "selection_template_written: %s authority=%s operator_approved=false\n",
			decisionReconcileDraftWrite,
			artifact.DecisionReconciliationSelectionApplyAuthority,
		); err != nil {
			return err
		}
	}
	if decisionReconcileDraftReview != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "selection_review_packet_written: %s authority=%s operator_approved=false\n",
			decisionReconcileDraftReview,
			artifact.DecisionReconciliationSelectionDraftAuthority,
		); err != nil {
			return err
		}
	}
	return writeDecisionReconciliationSelectionDraftSummary(cmd.OutOrStdout(), draft)
}

func decisionReconcileSelectionDraftLimit() int {
	if decisionReconcileDraftFull {
		return 0
	}
	if decisionReconcileDraftLimit > 0 {
		return decisionReconcileDraftLimit
	}
	return 5
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
	if decisionGoverningSetLimit < 0 {
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

	report, err := artifact.BuildCurrentGoverningSetReportFiltered(context.Background(), store, artifact.CurrentGoverningSetFilter{
		Query:      decisionGoverningSetQuery,
		SubjectRef: decisionGoverningSetSubjectRef,
		TargetRef:  decisionGoverningSetTargetRef,
	})
	if err != nil {
		return fmt.Errorf("build current governing set: %w", err)
	}
	if decisionGoverningSetWrite != "" {
		if err := writeCurrentGoverningSetSnapshotFile(decisionGoverningSetWrite, report); err != nil {
			return err
		}
	}
	if decisionGoverningSetCheck != "" {
		check, err := checkCurrentGoverningSetSnapshotFile(decisionGoverningSetCheck, report)
		if err != nil {
			return err
		}
		if decisionGoverningSetJSON {
			if err := writeJSON(cmd.OutOrStdout(), check); err != nil {
				return err
			}
		} else if err := writeCurrentGoverningSetSnapshotCheckSummary(cmd.OutOrStdout(), check); err != nil {
			return err
		}
		if !check.Match {
			return fmt.Errorf("current governing-set snapshot digest mismatch: current=%s recorded=%s", check.CurrentSnapshotDigest, check.RecordedSnapshotDigest)
		}
		return nil
	}
	if decisionGoverningSetJSON {
		outputReport := currentGoverningSetJSONProjection(report)
		return writeJSON(cmd.OutOrStdout(), outputReport)
	}
	if decisionGoverningSetWrite != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "governing_set_snapshot_written: %s digest=%s authority=%s\n",
			decisionGoverningSetWrite,
			report.Snapshot.SnapshotDigest,
			artifact.CurrentGoverningSetAuthority,
		); err != nil {
			return err
		}
	}
	return writeCurrentGoverningSetSummary(cmd.OutOrStdout(), report)
}

func currentGoverningSetJSONProjection(
	report artifact.CurrentGoverningSetReport,
) artifact.CurrentGoverningSetReport {
	if decisionGoverningSetLimit <= 0 {
		return report
	}
	return artifact.CompactCurrentGoverningSetReport(report, decisionGoverningSetLimit)
}

type currentGoverningSetSnapshotCheck struct {
	SchemaVersion          int      `json:"schema_version"`
	Authority              string   `json:"authority"`
	SnapshotPath           string   `json:"snapshot_path"`
	Match                  bool     `json:"match"`
	CurrentSnapshotDigest  string   `json:"current_snapshot_digest"`
	RecordedSnapshotDigest string   `json:"recorded_snapshot_digest"`
	CurrentGeneratedAt     string   `json:"current_generated_at,omitempty"`
	RecordedGeneratedAt    string   `json:"recorded_generated_at,omitempty"`
	MutationBoundary       []string `json:"mutation_boundary"`
}

func writeCurrentGoverningSetSnapshotFile(
	path string,
	report artifact.CurrentGoverningSetReport,
) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create governing-set snapshot directory %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode governing-set snapshot: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write governing-set snapshot %s: %w", path, err)
	}
	return nil
}

func checkCurrentGoverningSetSnapshotFile(
	path string,
	current artifact.CurrentGoverningSetReport,
) (currentGoverningSetSnapshotCheck, error) {
	recorded, err := readCurrentGoverningSetSnapshotFile(path)
	if err != nil {
		return currentGoverningSetSnapshotCheck{}, err
	}
	currentDigest := current.Snapshot.SnapshotDigest
	recordedDigest := recorded.Snapshot.SnapshotDigest
	return currentGoverningSetSnapshotCheck{
		SchemaVersion:          1,
		Authority:              "read_only_current_governing_frontier_snapshot_check",
		SnapshotPath:           path,
		Match:                  currentDigest == recordedDigest,
		CurrentSnapshotDigest:  currentDigest,
		RecordedSnapshotDigest: recordedDigest,
		CurrentGeneratedAt:     current.Snapshot.GeneratedAt,
		RecordedGeneratedAt:    recorded.Snapshot.GeneratedAt,
		MutationBoundary: []string{
			"snapshot check is read-only",
			"check does not create approval, evidence truth, gate passage, or decision authority",
			"mismatch requires operator review; it is not automatic reconciliation",
		},
	}, nil
}

func readCurrentGoverningSetSnapshotFile(path string) (artifact.CurrentGoverningSetReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifact.CurrentGoverningSetReport{}, fmt.Errorf("read governing-set snapshot %s: %w", path, err)
	}
	var report artifact.CurrentGoverningSetReport
	if err := json.Unmarshal(data, &report); err != nil {
		return artifact.CurrentGoverningSetReport{}, fmt.Errorf("parse governing-set snapshot %s: %w", path, err)
	}
	if report.Authority != artifact.CurrentGoverningSetAuthority {
		return artifact.CurrentGoverningSetReport{}, fmt.Errorf("governing-set snapshot %s authority = %q, want %q", path, report.Authority, artifact.CurrentGoverningSetAuthority)
	}
	if report.Snapshot.SnapshotDigest == "" {
		return artifact.CurrentGoverningSetReport{}, fmt.Errorf("governing-set snapshot %s missing snapshot.snapshot_digest", path)
	}
	return report, nil
}

func writeCurrentGoverningSetSnapshotCheckSummary(
	output io.Writer,
	check currentGoverningSetSnapshotCheck,
) error {
	_, err := fmt.Fprintf(
		output,
		"governing_set_snapshot_check: match=%t path=%s current=%s recorded=%s authority=%s\n",
		check.Match,
		check.SnapshotPath,
		check.CurrentSnapshotDigest,
		check.RecordedSnapshotDigest,
		check.Authority,
	)
	return err
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

func writeDecisionReconciliationSelectionTemplateFile(
	path string,
	draft artifact.DecisionReconciliationSelectionDraft,
) error {
	if draft.SelectionDocumentTemplate == nil {
		return fmt.Errorf("selection draft has no selection_document_template to write")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create decision reconciliation selection template directory %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(draft.SelectionDocumentTemplate, "", "  ")
	if err != nil {
		return fmt.Errorf("encode decision reconciliation selection template: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write decision reconciliation selection template %s: %w", path, err)
	}
	return nil
}

func writeDecisionReconciliationSelectionDraftFile(
	path string,
	draft artifact.DecisionReconciliationSelectionDraft,
) error {
	if draft.Authority != artifact.DecisionReconciliationSelectionDraftAuthority {
		return fmt.Errorf("selection draft authority = %q, want %q", draft.Authority, artifact.DecisionReconciliationSelectionDraftAuthority)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create decision reconciliation selection draft directory %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return fmt.Errorf("encode decision reconciliation selection draft: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write decision reconciliation selection draft %s: %w", path, err)
	}
	return nil
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
	if _, err := fmt.Fprintf(output, "emitted_candidates: %d\n", draft.Summary.EmittedCandidates); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "omitted_candidates: %d\n", draft.Summary.OmittedCandidates); err != nil {
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
			"- %s group=%s confidence=%s posture=%s subject_suggestions=%d carrier=%s files=%d hint=%s\n",
			item.DecisionRef,
			item.ReviewedGroupID,
			item.Confidence,
			item.CandidatePosture,
			len(item.DecisionSubjectRefSuggestions),
			item.DecisionCarrierHint,
			len(item.AffectedFiles),
			truncateDecisionReconciliationSummaryField(item.ScopeRepairHint),
		); err != nil {
			return err
		}
	}
	if draft.OmittedItems > 0 {
		_, err := fmt.Fprintf(output, "... and %d more; run `%s`\n", draft.OmittedItems, draft.FullAuditCommand)
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
