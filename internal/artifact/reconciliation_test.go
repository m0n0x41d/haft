package artifact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecisionReconciliationDoesNotMergeSharedFileWithoutExplicitSubject(t *testing.T) {
	items := []DecisionReconciliationItem{
		{
			DecisionID:     "dec-1",
			DecisionTitle:  "First",
			Status:         StatusActive,
			BoundedContext: "runtime",
			AffectedFiles:  []string{"internal/shared.go"},
		},
		{
			DecisionID:     "dec-2",
			DecisionTitle:  "Second",
			Status:         StatusActive,
			BoundedContext: "runtime",
			AffectedFiles:  []string{"internal/shared.go"},
		},
	}

	plan := BuildDecisionReconciliationPlanFromItems(items)

	if plan.Summary.MergeCandidates != 0 {
		t.Fatalf("merge_candidates = %d, want 0", plan.Summary.MergeCandidates)
	}
	if plan.Summary.Keep != 2 {
		t.Fatalf("keep = %d, want 2", plan.Summary.Keep)
	}
	if plan.Summary.MissingExplicitSubject != 2 {
		t.Fatalf("missing_explicit_subject = %d, want 2", plan.Summary.MissingExplicitSubject)
	}
	for _, group := range plan.Groups {
		if len(group.DecisionRefs) != 1 {
			t.Fatalf("shared file produced multi-decision group: %#v", group)
		}
	}
}

func TestDecisionReconciliationSurfacesScopeEnrichmentRepairHints(t *testing.T) {
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{{
		DecisionID:               "dec-fallback",
		Status:                   StatusActive,
		BoundedContext:           "drift",
		WholeFileFallbackTargets: []string{"whole_file_fallback:internal/shared.go"},
		AffectedFiles:            []string{"internal/shared.go"},
	}})

	if plan.Summary.ScopeEnrichmentCandidates != 1 {
		t.Fatalf("scope_enrichment_candidates = %d, want 1", plan.Summary.ScopeEnrichmentCandidates)
	}
	if plan.Summary.WholeFileFallbackOnly != 1 {
		t.Fatalf("whole_file_fallback_only = %d, want 1", plan.Summary.WholeFileFallbackOnly)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("groups = %#v", plan.Groups)
	}
	group := plan.Groups[0]
	if len(group.ScopeRepairHints) != 1 {
		t.Fatalf("scope_repair_hints = %#v", group.ScopeRepairHints)
	}
	if !strings.Contains(group.ScopeRepairHints[0], "enrich_scope") {
		t.Fatalf("scope_repair_hints = %#v, want enrich_scope hint", group.ScopeRepairHints)
	}
	item := group.Decisions[0]
	if !strings.Contains(item.ScopeRepairHint, "whole-file fallback") {
		t.Fatalf("item scope_repair_hint = %q", item.ScopeRepairHint)
	}
}

func TestDecisionReconciliationSelectionDraftIsReportOnly(t *testing.T) {
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{{
		DecisionID:               "dec-fallback",
		DecisionTitle:            "Fallback scope",
		Status:                   StatusActive,
		BoundedContext:           "drift",
		WholeFileFallbackTargets: []string{"whole_file_fallback:internal/shared.go"},
		AffectedFiles:            []string{"internal/shared.go"},
	}})

	draft := BuildDecisionReconciliationSelectionDraft(plan)

	if draft.Authority != DecisionReconciliationSelectionDraftAuthority {
		t.Fatalf("authority = %q", draft.Authority)
	}
	if draft.OperatorApproved {
		t.Fatal("operator_approved = true; draft must stay report-only")
	}
	if draft.ApplyAuthorityRequired != "operator_approved_reconciliation_selection" {
		t.Fatalf("apply authority = %q", draft.ApplyAuthorityRequired)
	}
	if draft.Summary.ScopeEnrichmentCandidates != 1 {
		t.Fatalf("scope_enrichment_candidates = %d, want 1", draft.Summary.ScopeEnrichmentCandidates)
	}
	if len(draft.Items) != 1 {
		t.Fatalf("items = %#v", draft.Items)
	}
	item := draft.Items[0]
	if item.Operation != DecisionReconciliationOperationEnrichScope {
		t.Fatalf("operation = %q", item.Operation)
	}
	if item.DecisionRef != "dec-fallback" {
		t.Fatalf("decision_ref = %q", item.DecisionRef)
	}
	if !strings.Contains(item.SelectionTemplate, "TODO_exact_decision_subject_ref") {
		t.Fatalf("selection_template lacks subject placeholder: %s", item.SelectionTemplate)
	}
	if !strings.Contains(strings.Join(draft.MutationBoundary, "\n"), "not an operator approval") {
		t.Fatalf("mutation_boundary = %#v", draft.MutationBoundary)
	}
}

func TestDecisionReconciliationGroupsExplicitSubjectAndTargetAsMergeCandidate(t *testing.T) {
	target := "symbol:internal/store.go:func::Save"
	items := []DecisionReconciliationItem{
		{
			DecisionID:         "dec-1",
			Status:             StatusActive,
			BoundedContext:     "artifact-store",
			DecisionSubjectRef: "subject:artifact-store-write-path",
			GovernanceTargets:  []string{target},
			AffectedFiles:      []string{"internal/store.go"},
		},
		{
			DecisionID:         "dec-2",
			Status:             StatusActive,
			BoundedContext:     "artifact-store",
			DecisionSubjectRef: "subject:artifact-store-write-path",
			GovernanceTargets:  []string{target},
			AffectedFiles:      []string{"internal/store.go"},
		},
	}

	plan := BuildDecisionReconciliationPlanFromItems(items)

	if plan.Summary.MergeCandidates != 1 {
		t.Fatalf("merge_candidates = %d, want 1", plan.Summary.MergeCandidates)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(plan.Groups))
	}
	group := plan.Groups[0]
	if group.Category != DecisionReconciliationMergeCandidate {
		t.Fatalf("category = %q", group.Category)
	}
	if group.OperatorRequired != true {
		t.Fatalf("operator_required = false, want true")
	}
	if len(group.GovernanceTargets) != 1 || group.GovernanceTargets[0] != target {
		t.Fatalf("governance_targets = %#v", group.GovernanceTargets)
	}
	if group.Preview.Authority != "report_only_preview_not_binding_authority" {
		t.Fatalf("preview authority = %#v", group.Preview)
	}
	if !group.Preview.ReadOnly {
		t.Fatalf("preview read_only = false")
	}
	if group.Preview.Operation != DecisionReconciliationOperationMergeThroughSuccessor {
		t.Fatalf("preview operation = %q", group.Preview.Operation)
	}
	if !containsString(group.Preview.RequiredSelectionFields, "items[].successor_ref") {
		t.Fatalf("preview required fields = %#v", group.Preview.RequiredSelectionFields)
	}
	if !containsString(group.Preview.ValidationNotes, "apply-ready only after an existing successor_ref is selected") {
		t.Fatalf("preview validation notes = %#v", group.Preview.ValidationNotes)
	}
	workflow := group.Preview.SuccessorWorkflow
	if workflow == nil {
		t.Fatal("consolidated successor workflow missing")
	}
	if !workflow.Required || !workflow.ExistingSuccessorRefRequired {
		t.Fatalf("successor workflow = %#v, want required existing successor", workflow)
	}
	if !containsString(workflow.RequiredPacketFields, "retained_claims") ||
		!containsString(workflow.RequiredPacketFields, "withdrawn_claims") ||
		!containsString(workflow.RequiredPacketFields, "drift_watch_targets") ||
		!containsString(workflow.RequiredPacketFields, "valid_until") {
		t.Fatalf("successor required_packet_fields = %#v", workflow.RequiredPacketFields)
	}
	if !strings.Contains(strings.Join(workflow.MutationBoundary, "\n"), "does not create the successor") {
		t.Fatalf("successor mutation_boundary = %#v", workflow.MutationBoundary)
	}
	if len(group.Preview.Proposed.Statuses) != 2 {
		t.Fatalf("preview proposed statuses = %#v", group.Preview.Proposed.Statuses)
	}
	for _, status := range group.Preview.Proposed.Statuses {
		if status.Status != string(StatusSuperseded) {
			t.Fatalf("preview proposed status = %#v, want superseded", status)
		}
	}
	if !hasLineageRelation(group.Preview.Proposed.LineageRelations, "mergedFrom", "$successor_ref", "dec-1") {
		t.Fatalf("preview lineage_relations = %#v, want mergedFrom placeholder", group.Preview.Proposed.LineageRelations)
	}
	if !hasLineageRelation(group.Preview.Proposed.LineageRelations, "retiredWithSuccessor", "dec-2", "$successor_ref") {
		t.Fatalf("preview lineage_relations = %#v, want retiredWithSuccessor placeholder", group.Preview.Proposed.LineageRelations)
	}
}

func TestDecisionReconciliationPreviewReportsDownstreamImpact(t *testing.T) {
	target := "symbol:internal/store.go:func::Save"
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{
		{
			DecisionID:         "dec-1",
			Status:             StatusActive,
			BoundedContext:     "artifact-store",
			DecisionSubjectRef: "subject:artifact-store-write-path",
			GovernanceTargets:  []string{target},
			Links: []Link{
				{Ref: "dec-2", Type: "supersedes"},
				{Ref: "evid-1", Type: "supported_by"},
			},
		},
		{
			DecisionID:         "dec-2",
			Status:             StatusActive,
			BoundedContext:     "artifact-store",
			DecisionSubjectRef: "subject:artifact-store-write-path",
			GovernanceTargets:  []string{target},
			Backlinks:          []Link{{Ref: "work-1", Type: "depends_on"}},
		},
	})

	if len(plan.Groups) != 1 {
		t.Fatalf("groups = %#v", plan.Groups)
	}
	impact := plan.Groups[0].Preview.DownstreamImpact
	if impact == nil {
		t.Fatal("downstream impact missing")
	}
	if impact.InternalEdges != 1 {
		t.Fatalf("internal_edges = %d, want 1", impact.InternalEdges)
	}
	if impact.ExternalEdges != 2 {
		t.Fatalf("external_edges = %d, want 2", impact.ExternalEdges)
	}
	if !containsString(impact.DependentRefs, "evid-1") || !containsString(impact.DependentRefs, "work-1") {
		t.Fatalf("dependent_refs = %#v", impact.DependentRefs)
	}
	if !strings.Contains(impact.ReviewCue, "does not relink") {
		t.Fatalf("review_cue = %q", impact.ReviewCue)
	}
	migration := plan.Groups[0].Preview.DownstreamMigration
	if migration == nil {
		t.Fatal("downstream migration report missing")
	}
	if !migration.RequiredBeforeApply {
		t.Fatalf("required_before_apply = false, want true for external dependents")
	}
	if migration.AutoRelink {
		t.Fatalf("auto_relink = true, want false")
	}
	if !containsString(migration.DependentRefs, "evid-1") || !containsString(migration.DependentRefs, "work-1") {
		t.Fatalf("migration dependent_refs = %#v", migration.DependentRefs)
	}
	if !strings.Contains(strings.Join(migration.ReviewSteps, "\n"), "does not relink") {
		t.Fatalf("migration review_steps = %#v, want no auto relink boundary", migration.ReviewSteps)
	}
}

func TestDecisionReconciliationPreviewForScopeEnrichmentIsReadOnly(t *testing.T) {
	ctx := context.Background()
	store := setupTestDB(t)
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope-preview", StatusActive, "runtime", DecisionFields{
		SelectedTitle: "Keep runtime boundary",
	}, now)

	plan, err := BuildDecisionReconciliationPlan(ctx, store)
	if err != nil {
		t.Fatalf("BuildDecisionReconciliationPlan: %v", err)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("groups = %#v", plan.Groups)
	}
	preview := plan.Groups[0].Preview
	if preview.Operation != DecisionReconciliationOperationEnrichScope {
		t.Fatalf("preview operation = %q, want enrich_scope", preview.Operation)
	}
	if preview.ApplyOperation != DecisionReconciliationOperationEnrichScope {
		t.Fatalf("preview apply operation = %q", preview.ApplyOperation)
	}
	if !containsString(preview.RequiredSelectionFields, "items[].decision_subject_ref") {
		t.Fatalf("preview required fields = %#v", preview.RequiredSelectionFields)
	}
	if !strings.Contains(strings.Join(preview.MutationBoundary, "\n"), "read-only") {
		t.Fatalf("preview mutation boundary = %#v", preview.MutationBoundary)
	}

	decision, err := store.Get(ctx, "dec-scope-preview")
	if err != nil {
		t.Fatalf("load decision: %v", err)
	}
	fields := decision.UnmarshalDecisionFields()
	if fields.DecisionSubjectRef != "" {
		t.Fatalf("decision_subject_ref = %q, preview mutated the decision", fields.DecisionSubjectRef)
	}
	if decision.Meta.Status != StatusActive {
		t.Fatalf("status = %q, preview mutated the decision", decision.Meta.Status)
	}
}

func TestDecisionReconciliationClassifiesRefreshDueAsReopenCandidate(t *testing.T) {
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{{
		DecisionID:         "dec-refresh",
		Status:             StatusRefreshDue,
		BoundedContext:     "status",
		DecisionSubjectRef: "subject:status-policy",
		GovernanceTargets:  []string{"api_contract:haft_query/status"},
	}})

	if plan.Summary.ReopenCandidates != 1 {
		t.Fatalf("reopen_candidates = %d, want 1", plan.Summary.ReopenCandidates)
	}
	if plan.Groups[0].Category != DecisionReconciliationReopenCandidate {
		t.Fatalf("category = %q", plan.Groups[0].Category)
	}
}

func TestDecisionReconciliationClassifiesContradictsLinkAsConflict(t *testing.T) {
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{
		{
			DecisionID:         "dec-a",
			Status:             StatusActive,
			BoundedContext:     "authority",
			DecisionSubjectRef: "subject:binding-mode",
			GovernanceTargets:  []string{"api_contract:haft_decision/decide"},
			Links:              []Link{{Ref: "dec-b", Type: "contradicts"}},
		},
		{
			DecisionID:         "dec-b",
			Status:             StatusActive,
			BoundedContext:     "authority",
			DecisionSubjectRef: "subject:binding-mode",
			GovernanceTargets:  []string{"api_contract:haft_decision/decide"},
		},
	})

	if plan.Summary.ConflictRequiresOperator != 1 {
		t.Fatalf("conflict_requires_operator = %d, want 1", plan.Summary.ConflictRequiresOperator)
	}
	if plan.Groups[0].Category != DecisionReconciliationConflictRequiresOperator {
		t.Fatalf("category = %q", plan.Groups[0].Category)
	}
	if plan.Groups[0].Preview.Operation != "operator_judgment_required" {
		t.Fatalf("preview operation = %q", plan.Groups[0].Preview.Operation)
	}
	if plan.Groups[0].Preview.ApplyOperation != "" {
		t.Fatalf("preview apply operation = %q, want empty", plan.Groups[0].Preview.ApplyOperation)
	}
	if !containsString(plan.Groups[0].Preview.ValidationNotes, "not apply-ready: conflict requires operator judgment before choosing a concrete operation") {
		t.Fatalf("preview validation notes = %#v", plan.Groups[0].Preview.ValidationNotes)
	}
}

func TestBuildDecisionReconciliationPlanReadsStoreWithoutMutating(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	createdAt := time.Now().UTC()

	createDecisionForReconciliation(t, store, "dec-1", StatusActive, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:artifact-store-write-path",
		BindingTargets: []BindingTarget{{
			Kind:       BindingTargetSymbol,
			FilePath:   "internal/store.go",
			SymbolKind: "func",
			SymbolName: "Save",
		}},
	}, createdAt)
	createDecisionForReconciliation(t, store, "dec-2", StatusActive, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:artifact-store-write-path",
		BindingTargets: []BindingTarget{{
			Kind:       BindingTargetSymbol,
			FilePath:   "internal/store.go",
			SymbolKind: "func",
			SymbolName: "Save",
		}},
	}, createdAt)
	if err := store.SetAffectedFiles(ctx, "dec-1", []AffectedFile{{Path: "internal/store.go"}}); err != nil {
		t.Fatalf("SetAffectedFiles dec-1: %v", err)
	}
	if err := store.SetAffectedFiles(ctx, "dec-2", []AffectedFile{{Path: "internal/store.go"}}); err != nil {
		t.Fatalf("SetAffectedFiles dec-2: %v", err)
	}

	plan, err := BuildDecisionReconciliationPlan(ctx, store)
	if err != nil {
		t.Fatalf("BuildDecisionReconciliationPlan: %v", err)
	}

	if plan.Summary.ReviewedDecisions != 2 {
		t.Fatalf("reviewed_decisions = %d, want 2", plan.Summary.ReviewedDecisions)
	}
	if plan.Summary.MergeCandidates != 1 {
		t.Fatalf("merge_candidates = %d, want 1", plan.Summary.MergeCandidates)
	}
}

func TestBuildDecisionReconciliationPlanIncludesClaimLifecycleSummary(t *testing.T) {
	ctx := context.Background()
	store := setupTestDB(t)
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-claims", StatusActive, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:claims",
		Claims: []DecisionClaim{
			{
				ID:                   "claim-active",
				Claim:                "Active claim",
				GovernanceTargetRefs: []string{"api_contract:haft/status"},
			},
			{
				ID:              "claim-old",
				Claim:           "Old claim",
				LifecycleStatus: ClaimLifecycleSuperseded,
				SuccessorRef:    "dec-new#claim-new",
			},
		},
	}, now)

	plan, err := BuildDecisionReconciliationPlan(ctx, store)
	if err != nil {
		t.Fatalf("BuildDecisionReconciliationPlan: %v", err)
	}
	if len(plan.Groups) != 1 || len(plan.Groups[0].Decisions) != 1 {
		t.Fatalf("plan groups = %#v", plan.Groups)
	}
	lifecycle := plan.Groups[0].Decisions[0].ClaimLifecycle
	if lifecycle == nil {
		t.Fatal("claim lifecycle summary missing")
	}
	if lifecycle.Active != 1 || lifecycle.Superseded != 1 {
		t.Fatalf("claim lifecycle summary = %#v", lifecycle)
	}
	if len(lifecycle.GovernanceTargetRefs) != 1 || lifecycle.GovernanceTargetRefs[0] != "api_contract:haft/status" {
		t.Fatalf("governance refs = %#v", lifecycle.GovernanceTargetRefs)
	}
}

func TestValidateDecisionReconciliationSelectionRequiresOperatorApproval(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	err := ValidateDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion: DecisionReconciliationSchemaVersion,
		Authority:     "operator_approved_reconciliation_selection",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationRetireWithoutSuccessor,
			ReviewedGroupID: "decision-reconcile-001",
			DecisionRefs:    []string{"dec-old"},
			Reason:          "obsolete",
		}},
	})

	if err == nil {
		t.Fatal("expected missing operator approval error")
	}
}

func TestApplyDecisionReconciliationMergeThroughSuccessorPreservesLineage(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-old-a", StatusActive, "artifact", DecisionFields{}, now)
	createDecisionForReconciliation(t, store, "dec-old-b", StatusActive, "artifact", DecisionFields{}, now)
	createDecisionForReconciliation(t, store, "dec-successor", StatusActive, "artifact", DecisionFields{}, now)

	result, err := ApplyDecisionReconciliationSelections(ctx, store, haftDir, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-merge",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationMergeThroughSuccessor,
			ReviewedGroupID: "decision-reconcile-merge",
			DecisionRefs:    []string{"dec-old-a", "dec-old-b"},
			SuccessorRef:    "dec-successor",
			Reason:          "Consolidated governing frontier.",
		}},
	})
	if err != nil {
		t.Fatalf("ApplyDecisionReconciliationSelections: %v", err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applied = %#v", result.Applied)
	}
	if !hasLineageRelation(result.Applied[0].LineageRelations, "mergedFrom", "dec-successor", "dec-old-a") {
		t.Fatalf("lineage_relations = %#v, want mergedFrom actual successor", result.Applied[0].LineageRelations)
	}
	if !hasLineageRelation(result.Applied[0].LineageRelations, "retiredWithSuccessor", "dec-old-b", "dec-successor") {
		t.Fatalf("lineage_relations = %#v, want retiredWithSuccessor actual successor", result.Applied[0].LineageRelations)
	}

	for _, ref := range []string{"dec-old-a", "dec-old-b"} {
		decision, err := store.Get(ctx, ref)
		if err != nil {
			t.Fatalf("load %s: %v", ref, err)
		}
		if decision.Meta.Status != StatusSuperseded {
			t.Fatalf("%s status = %q, want superseded", ref, decision.Meta.Status)
		}
	}
	links, err := store.GetLinks(ctx, "dec-successor")
	if err != nil {
		t.Fatalf("GetLinks successor: %v", err)
	}
	for _, ref := range []string{"dec-old-a", "dec-old-b"} {
		if !hasLink(links, ref, "supersedes") {
			t.Fatalf("successor missing supersedes link to %s: %#v", ref, links)
		}
	}
}

func TestApplyDecisionReconciliationRetireWithoutSuccessor(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-obsolete", StatusActive, "runtime", DecisionFields{}, now)

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-retire",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationRetireWithoutSuccessor,
			ReviewedGroupID: "decision-reconcile-retire",
			DecisionRefs:    []string{"dec-obsolete"},
			Reason:          "Surface removed.",
		}},
	})
	if err != nil {
		t.Fatalf("ApplyDecisionReconciliationSelections: %v", err)
	}
	if len(result.Applied) != 1 || !hasLineageRelation(result.Applied[0].LineageRelations, "retiredWithoutSuccessor", "dec-obsolete", "") {
		t.Fatalf("lineage_relations = %#v, want retiredWithoutSuccessor", result.Applied)
	}

	decision, err := store.Get(ctx, "dec-obsolete")
	if err != nil {
		t.Fatalf("load retired decision: %v", err)
	}
	if decision.Meta.Status != StatusDeprecated {
		t.Fatalf("status = %q, want deprecated", decision.Meta.Status)
	}
}

func TestApplyDecisionReconciliationReopenCreatesProblem(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-stale", StatusActive, "status", DecisionFields{}, now)

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-reopen",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationReopen,
			ReviewedGroupID: "decision-reconcile-reopen",
			DecisionRefs:    []string{"dec-stale"},
			Reason:          "Assumptions changed.",
		}},
	})
	if err != nil {
		t.Fatalf("ApplyDecisionReconciliationSelections: %v", err)
	}
	if len(result.Applied) != 1 || len(result.Applied[0].ProblemRefs) != 1 {
		t.Fatalf("result = %#v", result)
	}

	decision, err := store.Get(ctx, "dec-stale")
	if err != nil {
		t.Fatalf("load reopened decision: %v", err)
	}
	if decision.Meta.Status != StatusRefreshDue {
		t.Fatalf("status = %q, want refresh_due", decision.Meta.Status)
	}
	problem, err := store.Get(ctx, result.Applied[0].ProblemRefs[0])
	if err != nil {
		t.Fatalf("load created problem: %v", err)
	}
	if problem.Meta.Kind != KindProblemCard {
		t.Fatalf("problem kind = %s", problem.Meta.Kind)
	}
}

func TestApplyDecisionReconciliationClaimLifecycleUpdateKeepsDecisionActive(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-claims", StatusActive, "runtime", DecisionFields{
		Claims: []DecisionClaim{
			{
				ID:         "claim-old",
				Claim:      "Old runtime boundary holds.",
				Observable: "Boundary evidence exists.",
				Threshold:  "Evidence is current.",
			},
			{
				ID:         "claim-needs-review",
				Claim:      "Runtime prompt boundary is still sufficient.",
				Observable: "Review confirms the prompt boundary.",
				Threshold:  "No drift.",
			},
		},
	}, now)

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-claim-lifecycle",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationClaimLifecycleUpdate,
			ReviewedGroupID: "decision-reconcile-claims",
			DecisionRefs:    []string{"dec-claims"},
			ClaimLifecycleUpdates: []DecisionReconciliationClaimLifecycleUpdate{
				{
					DecisionRef:     "dec-claims",
					ClaimID:         "claim-old",
					LifecycleStatus: ClaimLifecycleSuperseded,
					SuccessorRef:    "dec-successor#claim-new",
					Reason:          "Narrower successor claim replaces this one.",
				},
				{
					DecisionRef:     "dec-claims",
					ClaimID:         "claim-needs-review",
					LifecycleStatus: ClaimLifecycleRefreshDue,
					Reason:          "Evidence window expired.",
				},
			},
			Reason: "Partial claim lifecycle update.",
		}},
	})
	if err != nil {
		t.Fatalf("ApplyDecisionReconciliationSelections: %v", err)
	}
	if len(result.Applied) != 1 || len(result.Applied[0].ClaimUpdates) != 2 {
		t.Fatalf("claim lifecycle result = %#v", result)
	}
	if !containsString(result.Applied[0].UpdatedFields, "claims[].lifecycle_status") {
		t.Fatalf("updated fields = %#v", result.Applied[0].UpdatedFields)
	}

	decision, err := store.Get(ctx, "dec-claims")
	if err != nil {
		t.Fatalf("load decision: %v", err)
	}
	if decision.Meta.Status != StatusActive {
		t.Fatalf("decision status = %q, want active", decision.Meta.Status)
	}
	fields := decision.UnmarshalDecisionFields()
	claims := map[string]DecisionClaim{}
	for _, claim := range fields.Claims {
		claims[claim.ID] = claim
	}
	if claims["claim-old"].LifecycleStatus != ClaimLifecycleSuperseded {
		t.Fatalf("claim-old lifecycle = %q", claims["claim-old"].LifecycleStatus)
	}
	if claims["claim-old"].SuccessorRef != "dec-successor#claim-new" {
		t.Fatalf("claim-old successor_ref = %q", claims["claim-old"].SuccessorRef)
	}
	if claims["claim-needs-review"].LifecycleStatus != ClaimLifecycleRefreshDue {
		t.Fatalf("claim-needs-review lifecycle = %q", claims["claim-needs-review"].LifecycleStatus)
	}
}

func TestApplyDecisionReconciliationEnrichScopeUpdatesOnlyScopeFields(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope", StatusActive, "runtime", DecisionFields{
		SelectedTitle: "Keep runtime boundary",
		Claims: []DecisionClaim{{
			ID:         "claim-runtime-boundary",
			Claim:      "Runtime boundary remains explicit.",
			Observable: "Boundary is represented in decision scope.",
			Threshold:  "Scope refs exist.",
		}},
	}, now)

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-scope-enrichment",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    "decision-reconcile-scope",
			DecisionRefs:       []string{"dec-scope"},
			DecisionSubjectRef: "runtime:explicit_boundary",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/runtime_boundary",
			}},
			DriftWatchTargets: []DriftWatchTarget{{
				TargetRef: "api_contract:haft/runtime_boundary",
				Trigger:   "schema_or_behavior_changed",
			}},
			ClaimGovernanceTargetRefs: map[string][]string{
				"claim-runtime-boundary": []string{"api_contract:haft/runtime_boundary"},
			},
			Reason: "Dogfood precision enrichment for old decision scope.",
		}},
	})
	if err != nil {
		t.Fatalf("ApplyDecisionReconciliationSelections: %v", err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applied = %#v", result.Applied)
	}
	if result.Authority != "operator_approved_lineage_mutation" {
		t.Fatalf("authority = %q", result.Authority)
	}
	if !containsString(result.Applied[0].UpdatedFields, "decision_subject_ref") {
		t.Fatalf("updated fields = %#v", result.Applied[0].UpdatedFields)
	}

	decision, err := store.Get(ctx, "dec-scope")
	if err != nil {
		t.Fatalf("load enriched decision: %v", err)
	}
	if decision.Meta.Status != StatusActive {
		t.Fatalf("status = %q, want active", decision.Meta.Status)
	}
	fields := decision.UnmarshalDecisionFields()
	if fields.DecisionSubjectRef != "runtime:explicit_boundary" {
		t.Fatalf("decision_subject_ref = %q", fields.DecisionSubjectRef)
	}
	if len(fields.GovernanceTargets) != 1 || fields.GovernanceTargets[0].Ref != "api_contract:haft/runtime_boundary" {
		t.Fatalf("governance_targets = %#v", fields.GovernanceTargets)
	}
	if len(fields.DriftWatchTargets) != 1 || fields.DriftWatchTargets[0].TargetRef != "api_contract:haft/runtime_boundary" {
		t.Fatalf("drift_watch_targets = %#v", fields.DriftWatchTargets)
	}
	if len(fields.Claims) != 1 || !containsString(fields.Claims[0].GovernanceTargetRefs, "api_contract:haft/runtime_boundary") {
		t.Fatalf("claims = %#v", fields.Claims)
	}

	plan, err := BuildDecisionReconciliationPlan(ctx, store)
	if err != nil {
		t.Fatalf("BuildDecisionReconciliationPlan: %v", err)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("groups = %#v", plan.Groups)
	}
	if plan.Groups[0].SubjectRef != "runtime:explicit_boundary" {
		t.Fatalf("subject = %q", plan.Groups[0].SubjectRef)
	}
	if !containsString(plan.Groups[0].GovernanceTargets, "api_contract:haft/runtime_boundary") {
		t.Fatalf("group targets = %#v", plan.Groups[0].GovernanceTargets)
	}
}

func TestApplyDecisionReconciliationEnrichScopeValidatesWholeBatchBeforeMutation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope-valid", StatusActive, "runtime", DecisionFields{}, now)
	createDecisionForReconciliation(t, store, "dec-scope-invalid", StatusActive, "runtime", DecisionFields{}, now)

	_, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-invalid-scope-batch",
		Items: []DecisionReconciliationSelection{
			{
				Operation:          DecisionReconciliationOperationEnrichScope,
				ReviewedGroupID:    "decision-reconcile-valid-scope",
				DecisionRefs:       []string{"dec-scope-valid"},
				DecisionSubjectRef: "runtime:valid_scope",
				GovernanceTargets: []GovernanceTarget{{
					Kind: "api_contract",
					Ref:  "api_contract:haft/valid_scope",
				}},
				Reason: "Would be valid alone.",
			},
			{
				Operation:          DecisionReconciliationOperationEnrichScope,
				ReviewedGroupID:    "decision-reconcile-invalid-scope",
				DecisionRefs:       []string{"dec-scope-invalid"},
				DecisionSubjectRef: "runtime:invalid_scope",
				Reason:             "Missing governance/drift target.",
			},
		},
	})
	if err == nil {
		t.Fatal("expected invalid batch error")
	}

	decision, err := store.Get(ctx, "dec-scope-valid")
	if err != nil {
		t.Fatalf("load decision: %v", err)
	}
	fields := decision.UnmarshalDecisionFields()
	if fields.DecisionSubjectRef != "" {
		t.Fatalf("decision_subject_ref = %q, want empty; first item mutated before validation failed", fields.DecisionSubjectRef)
	}
}

func TestApplyDecisionReconciliationValidatesWholeBatchBeforeMutation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-stays-active", StatusActive, "artifact", DecisionFields{}, now)

	_, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-invalid-batch",
		Items: []DecisionReconciliationSelection{
			{
				Operation:       DecisionReconciliationOperationRetireWithoutSuccessor,
				ReviewedGroupID: "decision-reconcile-retire",
				DecisionRefs:    []string{"dec-stays-active"},
				Reason:          "Would be valid alone.",
			},
			{
				Operation:       DecisionReconciliationOperationSupersede,
				ReviewedGroupID: "decision-reconcile-invalid",
				DecisionRefs:    []string{"dec-stays-active"},
				Reason:          "Missing successor.",
			},
		},
	})
	if err == nil {
		t.Fatal("expected invalid batch error")
	}

	decision, err := store.Get(ctx, "dec-stays-active")
	if err != nil {
		t.Fatalf("load decision: %v", err)
	}
	if decision.Meta.Status != StatusActive {
		t.Fatalf("status = %q, want active; first item mutated before validation failed", decision.Meta.Status)
	}
}

func hasLink(links []Link, ref string, linkType string) bool {
	for _, link := range links {
		if link.Ref == ref && link.Type == linkType {
			return true
		}
	}
	return false
}

func hasLineageRelation(
	relations []DecisionReconciliationLineageRelation,
	relation string,
	sourceRef string,
	targetRef string,
) bool {
	for _, candidate := range relations {
		if candidate.Relation != relation {
			continue
		}
		if candidate.SourceRef != sourceRef {
			continue
		}
		if candidate.TargetRef != targetRef {
			continue
		}
		return true
	}
	return false
}

func createDecisionForReconciliation(
	t *testing.T,
	store *Store,
	id string,
	status Status,
	contextName string,
	fields DecisionFields,
	createdAt time.Time,
) {
	t.Helper()

	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal decision fields: %v", err)
	}
	if err := store.Create(context.Background(), &Artifact{
		Meta: Meta{
			ID:        id,
			Kind:      KindDecisionRecord,
			Version:   1,
			Status:    status,
			Context:   contextName,
			Title:     "Decision " + id,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		Body:           "decision body",
		StructuredData: string(payload),
	}); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}
