package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestWriteDecisionReconciliationSummary(t *testing.T) {
	var output bytes.Buffer
	plan := artifact.BuildDecisionReconciliationPlanFromItems([]artifact.DecisionReconciliationItem{{
		DecisionID:         "dec-1",
		Status:             artifact.StatusRefreshDue,
		BoundedContext:     "status",
		DecisionSubjectRef: "subject:status-policy",
		GovernanceTargets:  []string{"api_contract:haft_query/status"},
	}})

	if err := writeDecisionReconciliationSummary(&output, plan); err != nil {
		t.Fatalf("writeDecisionReconciliationSummary returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Decision reconciliation plan v1",
		"authority: report_only_not_binding_authority",
		"reopen=1",
		"scope_enrichment_candidates: 0",
		"top_groups:",
		"preview=reopen",
		"preview_cues: read_only=true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestWriteDecisionReconciliationSummaryShowsPreviewCues(t *testing.T) {
	var output bytes.Buffer
	target := "symbol:internal/store.go:func::Save"
	plan := artifact.BuildDecisionReconciliationPlanFromItems([]artifact.DecisionReconciliationItem{
		{
			DecisionID:         "dec-1",
			Status:             artifact.StatusActive,
			BoundedContext:     "artifact-store",
			DecisionSubjectRef: "subject:artifact-store-write-path",
			GovernanceTargets:  []string{target},
			Links:              []artifact.Link{{Ref: "evid-1", Type: "supported_by"}},
		},
		{
			DecisionID:         "dec-2",
			Status:             artifact.StatusActive,
			BoundedContext:     "artifact-store",
			DecisionSubjectRef: "subject:artifact-store-write-path",
			GovernanceTargets:  []string{target},
		},
	})

	if err := writeDecisionReconciliationSummary(&output, plan); err != nil {
		t.Fatalf("writeDecisionReconciliationSummary returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"preview=merge_through_successor",
		"preview_cues: read_only=true",
		"lineage_relations=",
		"downstream_dependents=1",
		"downstream_migration_required=true",
		"successor_workflow=required_existing_ref",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestWriteCurrentGoverningSetSummary(t *testing.T) {
	var output bytes.Buffer
	report := artifact.CurrentGoverningSetReport{
		SchemaVersion: 1,
		Authority:     artifact.CurrentGoverningSetAuthority,
		Summary: artifact.CurrentGoverningSetSummary{
			CurrentDecisions:    1,
			GoverningSets:       1,
			FallbackTargetSets:  1,
			ScopeEnrichmentSets: 1,
			TerminalHistoryRefs: 1,
		},
		AuthorityFrontier: artifact.CurrentGoverningAuthorityFrontier{
			AuthorityBoundary:   "current_decision_refs_are_governing_authority_terminal_history_refs_are_not",
			CurrentDecisionRefs: []string{"dec-current"},
			TerminalHistoryRefs: []string{"dec-old"},
		},
		Sets: []artifact.CurrentGoverningSet{{
			Posture:             artifact.GoverningSetPostureSingle,
			SubjectRef:          "subject:store",
			TargetRef:           "symbol:store.go::Save",
			CurrentDecisionRefs: []string{"dec-current"},
		}},
	}

	if err := writeCurrentGoverningSetSummary(&output, report); err != nil {
		t.Fatalf("writeCurrentGoverningSetSummary: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Current governing set v1",
		"authority: read_only_current_authority_frontier",
		"current_decisions: 1",
		"authority_frontier: current_refs=1 terminal_history_refs=1",
		"current_decision_refs_are_governing_authority_terminal_history_refs_are_not",
		"fallback_sets=1",
		"scope_enrichment_sets=1",
		"top_sets:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestDecisionReconciliationJSONProjectionHonorsLimit(t *testing.T) {
	restore := stubDecisionReconcileReportLimits(t, 2, 0)
	defer restore()

	plan := artifact.BuildDecisionReconciliationPlanFromItems([]artifact.DecisionReconciliationItem{
		{
			DecisionID:         "dec-save",
			Status:             artifact.StatusActive,
			BoundedContext:     "artifact",
			DecisionSubjectRef: "subject:save",
			GovernanceTargets:  []string{"symbol:store.go::Save"},
		},
		{
			DecisionID:         "dec-load",
			Status:             artifact.StatusActive,
			BoundedContext:     "artifact",
			DecisionSubjectRef: "subject:load",
			GovernanceTargets:  []string{"symbol:store.go::Load"},
		},
		{
			DecisionID:         "dec-delete",
			Status:             artifact.StatusActive,
			BoundedContext:     "artifact",
			DecisionSubjectRef: "subject:delete",
			GovernanceTargets:  []string{"symbol:store.go::Delete"},
		},
	})

	compact := decisionReconciliationJSONProjection(plan)
	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if len(compact.CompactGroups) != 2 {
		t.Fatalf("compact groups = %d, want 2: %#v", len(compact.CompactGroups), compact.CompactGroups)
	}
	if compact.OmittedGroups == 0 {
		t.Fatalf("omitted_groups = 0, want hidden groups")
	}
	if len(compact.Groups) != 0 {
		t.Fatalf("compact projection leaked audit groups: %#v", compact.Groups)
	}

	restore()
	restore = stubDecisionReconcileReportLimits(t, 0, 0)
	defer restore()

	full := decisionReconciliationJSONProjection(plan)
	if full.View != "" {
		t.Fatalf("full view = %q, want empty", full.View)
	}
	if len(full.Groups) != len(plan.Groups) {
		t.Fatalf("full groups = %d, want %d", len(full.Groups), len(plan.Groups))
	}
}

func TestCurrentGoverningSetJSONProjectionHonorsLimit(t *testing.T) {
	restore := stubDecisionReconcileReportLimits(t, 0, 1)
	defer restore()

	report := artifact.CurrentGoverningSetReport{
		SchemaVersion: artifact.CurrentGoverningSetSchemaVersion,
		Authority:     artifact.CurrentGoverningSetAuthority,
		Summary: artifact.CurrentGoverningSetSummary{
			CurrentDecisions: 2,
			GoverningSets:    2,
		},
		Sets: []artifact.CurrentGoverningSet{
			{
				SetID:               "governing-set-save",
				SubjectRef:          "subject:save",
				TargetRef:           "symbol:store.go::Save",
				Posture:             artifact.GoverningSetPostureSingle,
				CurrentDecisionRefs: []string{"dec-save"},
			},
			{
				SetID:               "governing-set-load",
				SubjectRef:          "subject:load",
				TargetRef:           "symbol:store.go::Load",
				Posture:             artifact.GoverningSetPostureSingle,
				CurrentDecisionRefs: []string{"dec-load"},
			},
		},
	}

	compact := currentGoverningSetJSONProjection(report)
	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if len(compact.CompactSets) != 1 {
		t.Fatalf("compact sets = %d, want 1: %#v", len(compact.CompactSets), compact.CompactSets)
	}
	if compact.OmittedSets == 0 {
		t.Fatalf("omitted_sets = 0, want hidden sets")
	}
	if len(compact.Sets) != 0 {
		t.Fatalf("compact projection leaked audit sets: %#v", compact.Sets)
	}

	restore()
	restore = stubDecisionReconcileReportLimits(t, 0, 0)
	defer restore()

	full := currentGoverningSetJSONProjection(report)
	if full.View != "" {
		t.Fatalf("full view = %q, want empty", full.View)
	}
	if len(full.Sets) != len(report.Sets) {
		t.Fatalf("full sets = %d, want %d", len(full.Sets), len(report.Sets))
	}
}

func TestCurrentGoverningSetSnapshotWriteAndCheck(t *testing.T) {
	report := artifact.CurrentGoverningSetReport{
		SchemaVersion: artifact.CurrentGoverningSetSchemaVersion,
		Authority:     artifact.CurrentGoverningSetAuthority,
		Snapshot: artifact.CurrentGoverningSetSnapshot{
			GeneratedAt:    "2026-06-23T19:00:00Z",
			SnapshotDigest: "sha256:frontier",
		},
	}
	path := filepath.Join(t.TempDir(), "snapshots", "governing-set.json")

	if err := writeCurrentGoverningSetSnapshotFile(path, report); err != nil {
		t.Fatalf("writeCurrentGoverningSetSnapshotFile: %v", err)
	}
	check, err := checkCurrentGoverningSetSnapshotFile(path, report)
	if err != nil {
		t.Fatalf("checkCurrentGoverningSetSnapshotFile: %v", err)
	}
	if !check.Match {
		t.Fatalf("match = false: %#v", check)
	}
	if check.Authority != "read_only_current_governing_frontier_snapshot_check" {
		t.Fatalf("authority = %q", check.Authority)
	}
	if !containsString(check.MutationBoundary, "snapshot check is read-only") {
		t.Fatalf("mutation_boundary = %#v", check.MutationBoundary)
	}

	mutated := report
	mutated.Snapshot.SnapshotDigest = "sha256:changed"
	mismatch, err := checkCurrentGoverningSetSnapshotFile(path, mutated)
	if err != nil {
		t.Fatalf("checkCurrentGoverningSetSnapshotFile mismatch: %v", err)
	}
	if mismatch.Match {
		t.Fatalf("mismatch match = true: %#v", mismatch)
	}
	if mismatch.CurrentSnapshotDigest != "sha256:changed" || mismatch.RecordedSnapshotDigest != "sha256:frontier" {
		t.Fatalf("mismatch digests = %#v", mismatch)
	}
}

func TestCurrentGoverningSetSnapshotRejectsWrongAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "governing-set.json")
	data := []byte(`{"schema_version":1,"authority":"not_the_frontier","snapshot":{"snapshot_digest":"sha256:x"}}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := checkCurrentGoverningSetSnapshotFile(path, artifact.CurrentGoverningSetReport{})
	if err == nil {
		t.Fatal("expected wrong authority to fail")
	}
	if !strings.Contains(err.Error(), "authority") {
		t.Fatalf("error = %v, want authority diagnostic", err)
	}
}

func TestWriteDecisionReconciliationMetricsSummary(t *testing.T) {
	var output bytes.Buffer
	packet := artifact.ReconciliationMetricsPacket{
		SchemaVersion: 1,
		Authority:     artifact.ReconciliationMetricsAuthority,
		CapturePolicy: "capture_before_and_after_operator_approved_reconciliation_apply",
		Reconciliation: artifact.ReconciliationPlanMetrics{
			ReviewedDecisions:         10,
			Groups:                    6,
			WholeFileFallbackOnly:     3,
			MissingExplicitSubject:    4,
			ScopeEnrichmentCandidates: 5,
			ConflictRequiresOperator:  1,
		},
		GoverningSet: artifact.ReconciliationGoverningMetrics{
			CurrentDecisions:    9,
			GoverningSets:       7,
			FallbackTargetSets:  2,
			ScopeEnrichmentSets: 3,
			ConflictSets:        1,
			OverlapReviewSets:   2,
			TerminalHistoryRefs: 8,
		},
		DriftEvents: artifact.ReconciliationDriftMetrics{
			UniqueEvents:                 11,
			ImpactedDecisions:            13,
			MaterialEvents:               6,
			AuditOnlyEvents:              4,
			NeedsBindingResolutionEvents: 5,
			SemanticTargetEvents:         7,
			FileFallbackEvents:           2,
			UnknownHighRiskEvents:        1,
			MaxFanout:                    9,
		},
	}

	if err := writeDecisionReconciliationMetricsSummary(&output, packet); err != nil {
		t.Fatalf("writeDecisionReconciliationMetricsSummary: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Decision reconciliation metrics v1",
		"authority: read_only_reconciliation_metrics_not_binding_authority",
		"whole_file_fallback_only=3",
		"scope_enrichment=5",
		"fallback_sets=2",
		"conflicts=1",
		"unique=11",
		"max_fanout=9",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestWriteDecisionReconciliationSelectionDraftSummary(t *testing.T) {
	var output bytes.Buffer
	draft := artifact.DecisionReconciliationSelectionDraft{
		SchemaVersion:          1,
		Authority:              artifact.DecisionReconciliationSelectionDraftAuthority,
		OperatorApproved:       false,
		ApplyAuthorityRequired: "operator_approved_reconciliation_selection",
		Summary: artifact.DecisionReconciliationDraftSummary{
			ScopeEnrichmentCandidates:  1,
			OperatorApprovalCandidates: 1,
			EmittedCandidates:          1,
			OmittedCandidates:          0,
			SelectedCandidates:         0,
		},
		CurrentMetrics: &artifact.ReconciliationMetricsPacket{
			Authority: artifact.ReconciliationMetricsAuthority,
			Reconciliation: artifact.ReconciliationPlanMetrics{
				ReviewedDecisions: 10,
				Groups:            6,
			},
			GoverningSet: artifact.ReconciliationGoverningMetrics{
				FallbackTargetSets:  2,
				ScopeEnrichmentSets: 3,
				ConflictSets:        1,
				TerminalHistoryRefs: 8,
			},
			DriftEvents: artifact.ReconciliationDriftMetrics{
				UniqueEvents:                 11,
				NeedsBindingResolutionEvents: 5,
				MaxFanout:                    9,
			},
			BeforeAfterUse: artifact.ReconciliationBeforeAfterUse{
				RequiredAuthority: artifact.DecisionReconciliationSelectionApplyAuthority,
			},
		},
		Items: []artifact.DecisionReconciliationDraftItem{{
			DecisionRef:       "dec-fallback",
			ReviewedGroupID:   "decision-reconcile-1",
			CandidatePosture:  "needs_subject_and_target_review",
			Confidence:        "low",
			AffectedFiles:     []string{"internal/shared.go"},
			ScopeRepairHint:   "use enrich_scope to add decision_subject_ref",
			BlockingQuestions: []string{"What exact object does this decision govern now?"},
			ApprovalReadiness: artifact.DecisionReconciliationReadiness{
				State:                "operator_review_required",
				NotApplyReadyReasons: []string{"selection document still contains missing or placeholder fields"},
			},
		}},
	}

	if err := writeDecisionReconciliationSelectionDraftSummary(&output, draft); err != nil {
		t.Fatalf("writeDecisionReconciliationSelectionDraftSummary: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Decision reconciliation selection draft v1",
		"authority: report_only_selection_draft_not_operator_approval",
		"operator_approved: false",
		"apply_authority_required: operator_approved_reconciliation_selection",
		"scope_enrichment_candidates: 1",
		"operator_approval_candidates: 1",
		"emitted_candidates: 1",
		"omitted_candidates: 0",
		"selected_candidates: 0",
		"current_metrics_authority: read_only_reconciliation_metrics_not_binding_authority",
		"current_metrics: reviewed=10 groups=6 fallback_sets=2 scope_enrichment_sets=3 conflicts=1 terminal_history=8 unique_drift=11 needs_binding=5 max_fanout=9",
		"before_after_required_authority: operator_approved_reconciliation_selection",
		"dec-fallback",
		"confidence=low",
		"readiness=operator_review_required",
		"blockers=1",
		"posture=needs_subject_and_target_review",
		"subject_suggestions=0",
		"carrier=",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestDecisionReconcileSelectionDraftLimitDefaultsToCompactUnlessFull(t *testing.T) {
	previousLimit := decisionReconcileDraftLimit
	previousFull := decisionReconcileDraftFull
	defer func() {
		decisionReconcileDraftLimit = previousLimit
		decisionReconcileDraftFull = previousFull
	}()

	decisionReconcileDraftLimit = 0
	decisionReconcileDraftFull = false
	if got := decisionReconcileSelectionDraftLimit(); got != 5 {
		t.Fatalf("default draft limit = %d, want 5", got)
	}

	decisionReconcileDraftLimit = 2
	decisionReconcileDraftFull = false
	if got := decisionReconcileSelectionDraftLimit(); got != 2 {
		t.Fatalf("explicit draft limit = %d, want 2", got)
	}

	decisionReconcileDraftLimit = 2
	decisionReconcileDraftFull = true
	if got := decisionReconcileSelectionDraftLimit(); got != 0 {
		t.Fatalf("full draft limit = %d, want 0", got)
	}
}

func stubDecisionReconcileReportLimits(t *testing.T, reconcileLimit int, governingSetLimit int) func() {
	t.Helper()

	previousReconcileLimit := decisionReconcileLimit
	previousGoverningSetLimit := decisionGoverningSetLimit
	decisionReconcileLimit = reconcileLimit
	decisionGoverningSetLimit = governingSetLimit

	return func() {
		decisionReconcileLimit = previousReconcileLimit
		decisionGoverningSetLimit = previousGoverningSetLimit
	}
}

func TestWriteDecisionReconciliationSelectionDraftSummaryShowsOmittedAuditRoute(t *testing.T) {
	var output bytes.Buffer
	draft := artifact.DecisionReconciliationSelectionDraft{
		SchemaVersion:          1,
		Authority:              artifact.DecisionReconciliationSelectionDraftAuthority,
		OperatorApproved:       false,
		ApplyAuthorityRequired: "operator_approved_reconciliation_selection",
		Summary: artifact.DecisionReconciliationDraftSummary{
			ScopeEnrichmentCandidates:  3,
			OperatorApprovalCandidates: 3,
			EmittedCandidates:          1,
			OmittedCandidates:          2,
			SelectedCandidates:         0,
		},
		OmittedItems:     2,
		FullAuditCommand: "haft decision reconcile selection-draft --json --full",
		Items: []artifact.DecisionReconciliationDraftItem{{
			DecisionRef:      "dec-1",
			ReviewedGroupID:  "decision-reconcile-1",
			CandidatePosture: "needs_subject_and_target_review",
			Confidence:       "low",
			ScopeRepairHint:  "use enrich_scope to add decision_subject_ref",
		}},
	}

	if err := writeDecisionReconciliationSelectionDraftSummary(&output, draft); err != nil {
		t.Fatalf("writeDecisionReconciliationSelectionDraftSummary: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"emitted_candidates: 1",
		"omitted_candidates: 2",
		"... and 2 more; run `haft decision reconcile selection-draft --json --full`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func TestWriteDecisionReconciliationSelectionReviewSummary(t *testing.T) {
	var output bytes.Buffer
	review := artifact.DecisionReconciliationSelectionReview{
		SchemaVersion:     1,
		Authority:         artifact.DecisionReconciliationSelectionReviewAuthority,
		DocumentAuthority: artifact.DecisionReconciliationSelectionDraftAuthority,
		RequiredAuthority: artifact.DecisionReconciliationSelectionApplyAuthority,
		OperatorApproved:  false,
		ApplyReady:        false,
		ItemCount:         1,
		ValidationErrors: []string{
			"authority must be operator_approved_reconciliation_selection",
			"operator_approval_ref is required",
		},
		Items: []artifact.DecisionReconciliationSelectionReviewItem{{
			Index:            0,
			Operation:        artifact.DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:  "decision-reconcile-1",
			DecisionRefs:     []string{"dec-fallback"},
			ApplyReady:       false,
			ValidationErrors: []string{"items[0].governance_targets or drift_watch_targets is required for enrich_scope"},
		}},
		NextSteps: []string{
			"create a separate selection document with authority=operator_approved_reconciliation_selection only after operator approval",
			"add operator_approval_ref that names the explicit approval event",
		},
		MutationBoundary: []string{
			"selection review is read-only",
			"review does not apply reconciliation selections",
		},
	}

	if err := writeDecisionReconciliationSelectionReviewSummary(&output, review); err != nil {
		t.Fatalf("writeDecisionReconciliationSelectionReviewSummary: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Decision reconciliation selection review v1",
		"authority: read_only_selection_review_not_apply_authority",
		"document_authority: report_only_selection_draft_not_operator_approval",
		"operator_approved: false",
		"apply_ready: false",
		"validation_errors:",
		"item[0] errors:",
		"mutation_boundary:",
		"review does not apply reconciliation selections",
		"next_steps:",
		"operator approval",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "apply_command:") {
		t.Fatalf("non-ready review should not print apply_command:\n%s", text)
	}
}

func TestHandleQuintQueryDecisionReconcileDefaultsToCompactReportOnlyPlan(t *testing.T) {
	store := setupCLIArtifactStore(t)
	seedDecisionReconcileDecision(t, store, "dec-1", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")
	seedDecisionReconcileDecision(t, store, "dec-2", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "decision_reconcile",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery decision_reconcile returned error: %v", err)
	}

	var plan artifact.DecisionReconciliationPlan
	if err := json.Unmarshal([]byte(result), &plan); err != nil {
		t.Fatalf("decode decision reconciliation plan: %v\n%s", err, result)
	}
	if plan.Authority != artifact.DecisionReconciliationAuthority {
		t.Fatalf("authority = %q", plan.Authority)
	}
	if plan.Summary.MergeCandidates != 1 {
		t.Fatalf("merge_candidates = %d, want 1", plan.Summary.MergeCandidates)
	}
	if plan.View != "compact" {
		t.Fatalf("default decision_reconcile view = %q, want compact", plan.View)
	}
	if len(plan.Groups) != 0 {
		t.Fatalf("default decision_reconcile should omit full groups: %#v", plan.Groups)
	}
	if len(plan.CompactGroups) != 1 || plan.CompactGroups[0].PreviewOperation != artifact.DecisionReconciliationOperationMergeThroughSuccessor {
		t.Fatalf("compact_groups = %#v", plan.CompactGroups)
	}
	if !strings.Contains(plan.FullAuditCommand, "full=true") {
		t.Fatalf("full audit command = %q", plan.FullAuditCommand)
	}

	fullResult, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "decision_reconcile",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery decision_reconcile full returned error: %v", err)
	}
	var fullPlan artifact.DecisionReconciliationPlan
	if err := json.Unmarshal([]byte(fullResult), &fullPlan); err != nil {
		t.Fatalf("decode full decision reconciliation plan: %v\n%s", err, fullResult)
	}
	if fullPlan.View != "" {
		t.Fatalf("full decision_reconcile view = %q, want empty audit view", fullPlan.View)
	}
	if len(fullPlan.Groups) != 1 || fullPlan.Groups[0].Preview.Operation != artifact.DecisionReconciliationOperationMergeThroughSuccessor {
		t.Fatalf("full groups = %#v", fullPlan.Groups)
	}
}

func TestHandleQuintQueryDecisionReconcileHonorsCompactLimit(t *testing.T) {
	store := setupCLIArtifactStore(t)
	seedDecisionReconcileDecision(t, store, "dec-save-1", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")
	seedDecisionReconcileDecision(t, store, "dec-save-2", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")
	seedDecisionReconcileDecision(t, store, "dec-load", artifact.StatusActive, "artifact", "subject:artifact-store-load", "Load")
	seedDecisionReconcileDecision(t, store, "dec-delete", artifact.StatusActive, "artifact", "subject:artifact-store-delete", "Delete")

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "decision_reconcile",
		"limit":  float64(2),
	})
	if err != nil {
		t.Fatalf("handleQuintQuery decision_reconcile returned error: %v", err)
	}

	var plan artifact.DecisionReconciliationPlan
	if err := json.Unmarshal([]byte(result), &plan); err != nil {
		t.Fatalf("decode decision reconciliation plan: %v\n%s", err, result)
	}
	if plan.View != "compact" {
		t.Fatalf("view = %q, want compact", plan.View)
	}
	if len(plan.CompactGroups) != 2 {
		t.Fatalf("compact groups = %d, want 2: %#v", len(plan.CompactGroups), plan.CompactGroups)
	}
	if plan.OmittedGroups == 0 {
		t.Fatalf("omitted_groups = 0, want limit to omit at least one group")
	}

	fullResult, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "decision_reconcile",
		"full":   true,
		"limit":  float64(2),
	})
	if err != nil {
		t.Fatalf("handleQuintQuery decision_reconcile full returned error: %v", err)
	}
	var fullPlan artifact.DecisionReconciliationPlan
	if err := json.Unmarshal([]byte(fullResult), &fullPlan); err != nil {
		t.Fatalf("decode full decision reconciliation plan: %v\n%s", err, fullResult)
	}
	if fullPlan.View != "" {
		t.Fatalf("full view = %q, want empty audit view", fullPlan.View)
	}
	if len(fullPlan.Groups) <= len(plan.CompactGroups) {
		t.Fatalf("full groups = %d, compact groups = %d; full view should ignore compact limit", len(fullPlan.Groups), len(plan.CompactGroups))
	}
}

func TestHandleQuintQueryGoverningSetDefaultsToCompactCurrentAuthorityFrontier(t *testing.T) {
	store := setupCLIArtifactStore(t)
	seedDecisionReconcileDecision(t, store, "dec-current", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "governing_set",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set returned error: %v", err)
	}

	var report artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode current governing set: %v\n%s", err, result)
	}
	if report.Authority != artifact.CurrentGoverningSetAuthority {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Summary.CurrentDecisions != 1 {
		t.Fatalf("current_decisions = %d, want 1", report.Summary.CurrentDecisions)
	}
	if report.View != "compact" {
		t.Fatalf("default governing_set view = %q, want compact", report.View)
	}
	if len(report.Sets) != 0 {
		t.Fatalf("default governing_set should omit full sets: %#v", report.Sets)
	}
	if len(report.CompactSets) != 1 || report.CompactSets[0].Posture != artifact.GoverningSetPostureSingle {
		t.Fatalf("compact_sets = %#v", report.CompactSets)
	}
	if !strings.Contains(report.FullAuditCommand, "full=true") {
		t.Fatalf("full audit command = %q", report.FullAuditCommand)
	}

	fullResult, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "governing_set",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set full returned error: %v", err)
	}
	var fullReport artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(fullResult), &fullReport); err != nil {
		t.Fatalf("decode full current governing set: %v\n%s", err, fullResult)
	}
	if fullReport.View != "" {
		t.Fatalf("full governing_set view = %q, want empty audit view", fullReport.View)
	}
	if len(fullReport.Sets) != 1 || fullReport.Sets[0].Posture != artifact.GoverningSetPostureSingle {
		t.Fatalf("full sets = %#v", fullReport.Sets)
	}
}

func TestHandleQuintQueryGoverningSetHonorsCompactLimit(t *testing.T) {
	store := setupCLIArtifactStore(t)
	seedDecisionReconcileDecision(t, store, "dec-save", artifact.StatusActive, "artifact", "subject:artifact-store-save", "Save")
	seedDecisionReconcileDecision(t, store, "dec-load", artifact.StatusActive, "artifact", "subject:artifact-store-load", "Load")
	seedDecisionReconcileDecision(t, store, "dec-delete", artifact.StatusActive, "artifact", "subject:artifact-store-delete", "Delete")

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "governing_set",
		"limit":  float64(2),
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set returned error: %v", err)
	}

	var report artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode current governing set: %v\n%s", err, result)
	}
	if report.View != "compact" {
		t.Fatalf("view = %q, want compact", report.View)
	}
	if len(report.CompactSets) != 2 {
		t.Fatalf("compact sets = %d, want 2: %#v", len(report.CompactSets), report.CompactSets)
	}
	if report.OmittedSets == 0 {
		t.Fatalf("omitted_sets = 0, want limit to omit at least one set")
	}

	fullResult, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "governing_set",
		"full":   true,
		"limit":  float64(2),
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set full returned error: %v", err)
	}
	var fullReport artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(fullResult), &fullReport); err != nil {
		t.Fatalf("decode full current governing set: %v\n%s", err, fullResult)
	}
	if fullReport.View != "" {
		t.Fatalf("full view = %q, want empty audit view", fullReport.View)
	}
	if len(fullReport.Sets) <= len(report.CompactSets) {
		t.Fatalf("full sets = %d, compact sets = %d; full view should ignore compact limit", len(fullReport.Sets), len(report.CompactSets))
	}
}

func TestHandleQuintQueryGoverningSetFiltersByQuery(t *testing.T) {
	store := setupCLIArtifactStore(t)
	seedDecisionReconcileDecision(t, store, "dec-save", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")
	seedDecisionReconcileDecision(t, store, "dec-load", artifact.StatusActive, "artifact", "subject:artifact-store", "Load")

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "governing_set",
		"query":  "Load",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set returned error: %v", err)
	}

	var report artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode current governing set: %v\n%s", err, result)
	}
	if report.Filter == nil || report.Filter.Query != "Load" {
		t.Fatalf("filter = %#v", report.Filter)
	}
	if report.Summary.GoverningSets != 1 || report.Summary.CurrentDecisions != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if report.View != "compact" {
		t.Fatalf("default filtered governing_set view = %q, want compact", report.View)
	}
	if len(report.CompactSets) != 1 || !strings.Contains(report.CompactSets[0].TargetRef, "Load") {
		t.Fatalf("compact_sets = %#v", report.CompactSets)
	}
}

func TestHandleQuintQueryGoverningSetAnswerPathSourceRefsFilterExactTarget(t *testing.T) {
	store := setupCLIArtifactStore(t)
	seedDecisionReconcileDecision(t, store, "dec-save", artifact.StatusActive, "artifact", "subject:artifact-store", "Save")
	seedDecisionReconcileDecision(t, store, "dec-load", artifact.StatusActive, "artifact", "subject:artifact-store", "Load")

	fullResult, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "governing_set",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set full returned error: %v", err)
	}
	var fullReport artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(fullResult), &fullReport); err != nil {
		t.Fatalf("decode full current governing set: %v\n%s", err, fullResult)
	}
	answerPath := governingSetAnswerPathForTarget(t, fullReport, "Load")
	if !strings.Contains(answerPath.MCPCall, "source_refs") {
		t.Fatalf("answer path MCP call = %q, want source_refs drill-down", answerPath.MCPCall)
	}

	filteredResult, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":      "governing_set",
		"source_refs": []any{answerPath.TargetRef},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery governing_set source_refs returned error: %v", err)
	}
	var filteredReport artifact.CurrentGoverningSetReport
	if err := json.Unmarshal([]byte(filteredResult), &filteredReport); err != nil {
		t.Fatalf("decode filtered current governing set: %v\n%s", err, filteredResult)
	}

	if filteredReport.Filter == nil || filteredReport.Filter.TargetRef != answerPath.TargetRef {
		t.Fatalf("filter = %#v, want target_ref %q", filteredReport.Filter, answerPath.TargetRef)
	}
	if filteredReport.Summary.GoverningSets != 1 || filteredReport.Summary.CurrentDecisions != 1 {
		t.Fatalf("summary = %#v, want one exact target set", filteredReport.Summary)
	}
	if len(filteredReport.CompactSets) != 1 || filteredReport.CompactSets[0].TargetRef != answerPath.TargetRef {
		t.Fatalf("compact_sets = %#v, want target_ref %q", filteredReport.CompactSets, answerPath.TargetRef)
	}
}

func TestDecisionReconcileJSONWriterKeepsReportShape(t *testing.T) {
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	plan := artifact.BuildDecisionReconciliationPlanFromItems([]artifact.DecisionReconciliationItem{{
		DecisionID:    "dec-1",
		Status:        artifact.StatusActive,
		AffectedFiles: []string{"shared.go"},
	}})

	if err := writeJSON(cmd.OutOrStdout(), plan); err != nil {
		t.Fatalf("writeJSON returned error: %v", err)
	}

	var decoded artifact.DecisionReconciliationPlan
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, output.String())
	}
	if decoded.Summary.Keep != 1 {
		t.Fatalf("keep = %d, want 1", decoded.Summary.Keep)
	}
	if decoded.Summary.MergeCandidates != 0 {
		t.Fatalf("merge_candidates = %d, want 0", decoded.Summary.MergeCandidates)
	}
	if len(decoded.Groups) != 1 || decoded.Groups[0].Preview.Operation != artifact.DecisionReconciliationOperationEnrichScope {
		t.Fatalf("preview = %#v", decoded.Groups)
	}
	if !decoded.Groups[0].Preview.ReadOnly {
		t.Fatalf("preview read_only = false")
	}
}

func TestReadDecisionReconciliationSelectionDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	data := []byte(`{
  "schema_version": 1,
  "authority": "operator_approved_reconciliation_selection",
  "operator_approval_ref": "chat:approved",
  "items": [
    {
      "operation": "enrich_scope",
      "reviewed_group_id": "decision-reconcile-001",
      "decision_refs": ["dec-1"],
      "decision_subject_ref": "runtime:explicit_scope",
      "governance_targets": [{"kind":"api_contract","ref":"api_contract:haft/runtime"}],
      "drift_watch_targets": [{"target_ref":"api_contract:haft/runtime","trigger":"schema_or_behavior_changed"}],
      "claim_governance_target_refs": {"claim-1":["api_contract:haft/runtime"]},
      "reason": "precision enrichment"
    }
  ]
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write selection: %v", err)
	}

	document, err := readDecisionReconciliationSelectionDocument(path)
	if err != nil {
		t.Fatalf("readDecisionReconciliationSelectionDocument: %v", err)
	}
	if document.OperatorApprovalRef != "chat:approved" {
		t.Fatalf("operator_approval_ref = %q", document.OperatorApprovalRef)
	}
	if len(document.Items) != 1 || document.Items[0].Operation != artifact.DecisionReconciliationOperationEnrichScope {
		t.Fatalf("items = %#v", document.Items)
	}
	if document.Items[0].DecisionSubjectRef != "runtime:explicit_scope" {
		t.Fatalf("decision_subject_ref = %q", document.Items[0].DecisionSubjectRef)
	}
	if len(document.Items[0].GovernanceTargets) != 1 {
		t.Fatalf("governance_targets = %#v", document.Items[0].GovernanceTargets)
	}
}

func TestWriteDecisionReconciliationSelectionTemplateFileKeepsApprovalEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review", "selection.json")
	draft := artifact.DecisionReconciliationSelectionDraft{
		SelectionDocumentTemplate: &artifact.DecisionReconciliationSelectionDocument{
			SchemaVersion:       artifact.DecisionReconciliationSchemaVersion,
			Authority:           artifact.DecisionReconciliationSelectionApplyAuthority,
			OperatorApprovalRef: "",
			Items: []artifact.DecisionReconciliationSelection{{
				Operation:          artifact.DecisionReconciliationOperationEnrichScope,
				ReviewedGroupID:    "decision-reconcile-001",
				DecisionRefs:       []string{"dec-1"},
				DecisionSubjectRef: "subject:exact",
				GovernanceTargets: []artifact.GovernanceTarget{{
					Kind: "api_contract",
					Ref:  "api_contract:haft/runtime",
				}},
				Reason: "TODO_operator_reviewed_scope_enrichment_reason",
			}},
		},
	}

	if err := writeDecisionReconciliationSelectionTemplateFile(path, draft); err != nil {
		t.Fatalf("writeDecisionReconciliationSelectionTemplateFile: %v", err)
	}
	document, err := readDecisionReconciliationSelectionDocument(path)
	if err != nil {
		t.Fatalf("read written selection template: %v", err)
	}
	if document.Authority != artifact.DecisionReconciliationSelectionApplyAuthority {
		t.Fatalf("authority = %q", document.Authority)
	}
	if document.OperatorApprovalRef != "" {
		t.Fatalf("operator_approval_ref = %q, want empty", document.OperatorApprovalRef)
	}
	if len(document.Items) != 1 || document.Items[0].Operation != artifact.DecisionReconciliationOperationEnrichScope {
		t.Fatalf("items = %#v", document.Items)
	}

	store := setupCLIArtifactStore(t)
	review := artifact.ReviewDecisionReconciliationSelectionDocument(
		context.Background(),
		store,
		document,
		path,
	)
	if review.ApplyReady {
		t.Fatalf("review apply_ready = true: %#v", review)
	}
	if review.OperatorApproved {
		t.Fatalf("review operator_approved = true: %#v", review)
	}
	if !containsString(review.ValidationErrors, "operator_approval_ref is required") {
		t.Fatalf("validation_errors = %#v, want missing operator_approval_ref", review.ValidationErrors)
	}
}

func TestWriteDecisionReconciliationSelectionTemplateFileRejectsMissingTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")

	err := writeDecisionReconciliationSelectionTemplateFile(
		path,
		artifact.DecisionReconciliationSelectionDraft{},
	)
	if err == nil {
		t.Fatal("expected missing template error")
	}
	if !strings.Contains(err.Error(), "selection_document_template") {
		t.Fatalf("error = %v, want selection_document_template diagnostic", err)
	}
}

func TestWriteDecisionReconciliationSelectionDraftFileKeepsReportOnlyAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review", "selection-draft.json")
	draft := artifact.DecisionReconciliationSelectionDraft{
		SchemaVersion:          artifact.DecisionReconciliationSchemaVersion,
		Authority:              artifact.DecisionReconciliationSelectionDraftAuthority,
		OperatorApproved:       false,
		ApplyAuthorityRequired: artifact.DecisionReconciliationSelectionApplyAuthority,
		SourcePlanAuthority:    artifact.DecisionReconciliationAuthority,
		FullAuditCommand:       "haft decision reconcile selection-draft --json --full",
		SelectionDocumentTemplate: &artifact.DecisionReconciliationSelectionDocument{
			SchemaVersion:       artifact.DecisionReconciliationSchemaVersion,
			Authority:           artifact.DecisionReconciliationSelectionApplyAuthority,
			OperatorApprovalRef: "",
			Items: []artifact.DecisionReconciliationSelection{{
				Operation:          artifact.DecisionReconciliationOperationEnrichScope,
				ReviewedGroupID:    "decision-reconcile-001",
				DecisionRefs:       []string{"dec-1"},
				DecisionSubjectRef: "TODO_exact_decision_subject_ref",
				Reason:             "TODO_operator_reviewed_scope_enrichment_reason",
			}},
		},
		MutationBoundary: []string{
			"selection draft is read-only",
			"draft output is not an operator approval",
		},
	}

	if err := writeDecisionReconciliationSelectionDraftFile(path, draft); err != nil {
		t.Fatalf("writeDecisionReconciliationSelectionDraftFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written draft: %v", err)
	}
	var decoded artifact.DecisionReconciliationSelectionDraft
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode written draft: %v\n%s", err, string(data))
	}
	if decoded.Authority != artifact.DecisionReconciliationSelectionDraftAuthority {
		t.Fatalf("authority = %q", decoded.Authority)
	}
	if decoded.OperatorApproved {
		t.Fatal("operator_approved = true")
	}
	if decoded.SelectionDocumentTemplate == nil {
		t.Fatal("selection_document_template missing")
	}
	if decoded.SelectionDocumentTemplate.OperatorApprovalRef != "" {
		t.Fatalf("operator_approval_ref = %q", decoded.SelectionDocumentTemplate.OperatorApprovalRef)
	}
	if !containsString(decoded.MutationBoundary, "draft output is not an operator approval") {
		t.Fatalf("mutation_boundary = %#v", decoded.MutationBoundary)
	}
}

func TestWriteDecisionReconciliationSelectionDraftFileRejectsApplyAuthorityDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection-draft.json")

	err := writeDecisionReconciliationSelectionDraftFile(
		path,
		artifact.DecisionReconciliationSelectionDraft{
			Authority: artifact.DecisionReconciliationSelectionApplyAuthority,
		},
	)
	if err == nil {
		t.Fatal("expected authority mismatch error")
	}
	if !strings.Contains(err.Error(), "report_only_selection_draft_not_operator_approval") {
		t.Fatalf("error = %v, want report-only authority diagnostic", err)
	}
}

func TestWriteDecisionReconciliationApplySummary(t *testing.T) {
	var output bytes.Buffer
	result := artifact.DecisionReconciliationApplyResult{
		SchemaVersion: 1,
		Authority:     "operator_approved_lineage_mutation",
		Applied: []artifact.DecisionReconciliationApplyOutcome{{
			Operation:     artifact.DecisionReconciliationOperationMergeThroughSuccessor,
			DecisionRefs:  []string{"dec-old"},
			SuccessorRef:  "dec-new",
			UpdatedFields: []string{"decision_subject_ref"},
			LineageRelations: []artifact.DecisionReconciliationLineageRelation{{
				Relation:  "mergedFrom",
				SourceRef: "dec-new",
				TargetRef: "dec-old",
			}},
			ClaimUpdates: []artifact.DecisionReconciliationClaimLifecycleUpdate{{
				DecisionRef:     "dec-old",
				ClaimID:         "claim-1",
				LifecycleStatus: artifact.ClaimLifecycleSuperseded,
				SuccessorRef:    "dec-new#claim-2",
			}},
			Status: "applied",
		}},
	}

	if err := writeDecisionReconciliationApplySummary(&output, result); err != nil {
		t.Fatalf("writeDecisionReconciliationApplySummary: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Decision reconciliation apply v1",
		"authority: operator_approved_lineage_mutation",
		"merge_through_successor",
		"dec-new",
		"updated=[decision_subject_ref]",
		"lineage_relation: mergedFrom dec-new -> dec-old",
		"claim_update: decision=dec-old claim=claim-1 lifecycle=superseded successor=dec-new#claim-2",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}

func governingSetAnswerPathForTarget(
	t *testing.T,
	report artifact.CurrentGoverningSetReport,
	targetFragment string,
) artifact.CurrentGoverningSetAnswerPath {
	t.Helper()

	for _, set := range report.Sets {
		if !strings.Contains(set.TargetRef, targetFragment) {
			continue
		}
		if len(set.AnswerPaths) != 1 {
			t.Fatalf("answer_paths for %s = %#v", set.TargetRef, set.AnswerPaths)
		}
		return set.AnswerPaths[0]
	}
	t.Fatalf("target fragment %q not found in governing sets: %#v", targetFragment, report.Sets)
	return artifact.CurrentGoverningSetAnswerPath{}
}

func seedDecisionReconcileDecision(
	t *testing.T,
	store *artifact.Store,
	id string,
	status artifact.Status,
	contextName string,
	subject string,
	symbol string,
) {
	t.Helper()

	fields := artifact.DecisionFields{
		DecisionSubjectRef: subject,
		BindingTargets: []artifact.BindingTarget{{
			Kind:       artifact.BindingTargetSymbol,
			FilePath:   "internal/store.go",
			SymbolKind: "func",
			SymbolName: symbol,
		}},
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal fields: %v", err)
	}
	now := time.Now().UTC()
	if err := store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        id,
			Kind:      artifact.KindDecisionRecord,
			Version:   1,
			Status:    status,
			Context:   contextName,
			Title:     "Decision " + id,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "decision body",
		StructuredData: string(payload),
	}); err != nil {
		t.Fatalf("create decision %s: %v", id, err)
	}
}
