package artifact

import (
	"context"
	"encoding/json"
	"strconv"
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

func TestCompactDecisionReconciliationPlanPreservesSummaryAndOmitsAuditGroups(t *testing.T) {
	plan := DecisionReconciliationPlan{
		SchemaVersion: DecisionReconciliationSchemaVersion,
		Authority:     DecisionReconciliationAuthority,
		Summary: DecisionReconciliationSummary{
			Groups:            3,
			ReviewedDecisions: 3,
		},
		Groups: []DecisionReconciliationGroup{
			{
				GroupID:           "group-1",
				Category:          DecisionReconciliationMergeCandidate,
				SubjectRef:        "subject:store",
				SubjectResolution: "explicit_subject",
				BoundedContext:    "artifact",
				DecisionRefs:      []string{"dec-1", "dec-2"},
				OperatorRequired:  true,
				Preview: DecisionReconciliationPreview{
					Operation:      DecisionReconciliationOperationMergeThroughSuccessor,
					ApplyOperation: DecisionReconciliationOperationMergeThroughSuccessor,
					DownstreamImpact: &DecisionReconciliationDownstream{
						DependentRefs: []string{"evid-1"},
					},
				},
			},
			{
				GroupID:      "group-2",
				Category:     DecisionReconciliationKeep,
				DecisionRefs: []string{"dec-3"},
			},
			{
				GroupID:      "group-3",
				Category:     DecisionReconciliationKeep,
				DecisionRefs: []string{"dec-4"},
			},
		},
	}

	compact := CompactDecisionReconciliationPlan(plan, 2)

	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if compact.Summary.Groups != 3 || compact.Summary.ReviewedDecisions != 3 {
		t.Fatalf("summary = %#v, want preserved source summary", compact.Summary)
	}
	if len(compact.Groups) != 0 {
		t.Fatalf("compact groups should omit full audit groups: %#v", compact.Groups)
	}
	if len(compact.CompactGroups) != 2 || compact.OmittedGroups != 1 {
		t.Fatalf("compact group count = %d omitted = %d", len(compact.CompactGroups), compact.OmittedGroups)
	}
	first := compact.CompactGroups[0]
	if first.GroupID != "group-1" || first.Fanout != 2 || !first.OperatorRequired {
		t.Fatalf("compact first group = %#v", first)
	}
	if first.PreviewOperation != DecisionReconciliationOperationMergeThroughSuccessor {
		t.Fatalf("preview_operation = %q", first.PreviewOperation)
	}
	if first.DownstreamDependents != 1 {
		t.Fatalf("downstream_dependents = %d, want 1", first.DownstreamDependents)
	}
	if !strings.Contains(compact.FullAuditCommand, "full=true") {
		t.Fatalf("full audit command = %q", compact.FullAuditCommand)
	}
	if len(plan.Groups) != 3 {
		t.Fatalf("source plan mutated: %#v", plan.Groups)
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
	if draft.Summary.OperatorApprovalCandidates != 1 {
		t.Fatalf("operator_approval_candidates = %d, want 1", draft.Summary.OperatorApprovalCandidates)
	}
	if draft.Summary.SelectedCandidates != 0 {
		t.Fatalf("selected_candidates = %d, want 0 for low-confidence draft item", draft.Summary.SelectedCandidates)
	}
	if len(draft.Items) != 1 {
		t.Fatalf("items = %#v", draft.Items)
	}
	item := draft.Items[0]
	if item.Operation != DecisionReconciliationOperationEnrichScope {
		t.Fatalf("operation = %q", item.Operation)
	}
	if item.CandidatePosture != "needs_subject_and_fallback_target_repair" {
		t.Fatalf("candidate_posture = %q", item.CandidatePosture)
	}
	if item.Confidence != "low" {
		t.Fatalf("confidence = %q", item.Confidence)
	}
	if !containsString(item.BlockingQuestions, "What exact object does this decision govern now?") {
		t.Fatalf("blocking_questions = %#v", item.BlockingQuestions)
	}
	if !strings.Contains(item.SuggestedReviewAction, "replace whole-file fallback") {
		t.Fatalf("suggested_review_action = %q", item.SuggestedReviewAction)
	}
	if item.ApprovalReadiness.State != "operator_review_required" {
		t.Fatalf("approval_readiness.state = %q", item.ApprovalReadiness.State)
	}
	if item.ApprovalReadiness.ApplyReady {
		t.Fatalf("approval_readiness.apply_ready = true; draft item must stay advisory")
	}
	for _, want := range []string{
		"operator_approval_ref",
		"items[].decision_subject_ref",
		"items[].governance_targets[0].kind",
		"items[].governance_targets[0].ref",
		"items[].drift_watch_targets[0].target_ref",
		"items[].drift_watch_targets[0].trigger",
		"items[].reason",
	} {
		if !containsString(item.ApprovalReadiness.PlaceholderFields, want) {
			t.Fatalf("approval_readiness.placeholder_fields missing %q: %#v", want, item.ApprovalReadiness.PlaceholderFields)
		}
	}
	if !containsString(item.ApprovalReadiness.RequiredOperatorChecks, "replace whole-file fallback targets unless no semantic target can be recovered") {
		t.Fatalf("approval_readiness.required_operator_checks = %#v", item.ApprovalReadiness.RequiredOperatorChecks)
	}
	if !containsString(item.ApprovalReadiness.AuthorityBoundary, "approval_readiness does not create operator approval") {
		t.Fatalf("approval_readiness.authority_boundary = %#v", item.ApprovalReadiness.AuthorityBoundary)
	}
	if item.DecisionRef != "dec-fallback" {
		t.Fatalf("decision_ref = %q", item.DecisionRef)
	}
	if !strings.Contains(item.SelectionTemplate, "TODO_exact_decision_subject_ref") {
		t.Fatalf("selection_template lacks subject placeholder: %s", item.SelectionTemplate)
	}
	if item.ProposedSelection == nil {
		t.Fatal("proposed_selection missing")
	}
	if item.ProposedSelection.DecisionSubjectRef != "TODO_exact_decision_subject_ref" {
		t.Fatalf("proposed_selection decision_subject_ref = %q", item.ProposedSelection.DecisionSubjectRef)
	}
	if item.ProposedSelection.Reason != "TODO_operator_reviewed_scope_enrichment_reason" {
		t.Fatalf("proposed_selection reason = %q", item.ProposedSelection.Reason)
	}
	if !strings.Contains(strings.Join(draft.MutationBoundary, "\n"), "not an operator approval") {
		t.Fatalf("mutation_boundary = %#v", draft.MutationBoundary)
	}
}

func TestDecisionReconciliationSelectionDraftPrefillsKnownGovernanceTargets(t *testing.T) {
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{{
		DecisionID:        "dec-precise",
		DecisionTitle:     "Precise scope",
		Status:            StatusActive,
		BoundedContext:    "fpf-retrieval",
		GovernanceTargets: []string{"symbol:internal/fpf/specsearch.go:type:Route"},
		ScopeRepairHint:   "use enrich_scope to add decision_subject_ref",
		AffectedFiles:     []string{"internal/fpf/specsearch.go"},
	}})

	draft := BuildDecisionReconciliationSelectionDraft(plan)

	if len(draft.Items) != 1 {
		t.Fatalf("items = %#v", draft.Items)
	}
	if draft.Summary.SelectedCandidates != 0 {
		t.Fatalf("selected_candidates = %d, want 0 for medium-confidence draft item", draft.Summary.SelectedCandidates)
	}
	item := draft.Items[0]
	if strings.Contains(item.SelectionTemplate, "TODO_target_kind") {
		t.Fatalf("selection_template should reuse current governance target:\n%s", item.SelectionTemplate)
	}
	if item.CandidatePosture != "precise_target_prefilled_subject_needed" {
		t.Fatalf("candidate_posture = %q", item.CandidatePosture)
	}
	if item.Confidence != "medium" {
		t.Fatalf("confidence = %q", item.Confidence)
	}
	for _, unexpected := range []string{
		"items[].governance_targets[0].kind",
		"items[].governance_targets[0].ref",
		"items[].drift_watch_targets[0].target_ref",
	} {
		if containsString(item.ApprovalReadiness.PlaceholderFields, unexpected) {
			t.Fatalf("approval_readiness.placeholder_fields contains unexpected %q: %#v", unexpected, item.ApprovalReadiness.PlaceholderFields)
		}
	}
	for _, want := range []string{
		"operator_approval_ref",
		"items[].decision_subject_ref",
		"items[].reason",
	} {
		if !containsString(item.ApprovalReadiness.PlaceholderFields, want) {
			t.Fatalf("approval_readiness.placeholder_fields missing %q: %#v", want, item.ApprovalReadiness.PlaceholderFields)
		}
	}
	if !containsString(item.ApprovalReadiness.RequiredOperatorChecks, "confirm each prefilled governance target is a real falsification or preservation boundary, not only an implementation footprint") {
		t.Fatalf("approval_readiness.required_operator_checks = %#v", item.ApprovalReadiness.RequiredOperatorChecks)
	}
	if item.DecisionCarrierHint != ".haft/decisions/dec-precise.md" {
		t.Fatalf("decision_carrier_hint = %q", item.DecisionCarrierHint)
	}
	if !containsString(item.ReviewCommands, "sed -n '1,220p' .haft/decisions/dec-precise.md") ||
		!containsString(item.ReviewCommands, "haft decision reconcile selection-draft --decision-ref dec-precise --json") {
		t.Fatalf("review_commands = %#v", item.ReviewCommands)
	}
	if !containsString(item.ReviewNotes, "decision_carrier_hint and review_commands are discovery aids, not authority or apply approval") {
		t.Fatalf("review_notes = %#v", item.ReviewNotes)
	}
	if !containsString(item.DecisionSubjectRefSuggestions, "subject:fpf-retrieval:precise-scope") ||
		!containsString(item.DecisionSubjectRefSuggestions, "subject:precise-scope") {
		t.Fatalf("decision_subject_ref_suggestions = %#v", item.DecisionSubjectRefSuggestions)
	}
	var selection DecisionReconciliationSelection
	if err := json.Unmarshal([]byte(item.SelectionTemplate), &selection); err != nil {
		t.Fatalf("decode selection_template: %v\n%s", err, item.SelectionTemplate)
	}
	if selection.DecisionSubjectRef != "TODO_exact_decision_subject_ref" {
		t.Fatalf("decision_subject_ref = %q, want subject placeholder", selection.DecisionSubjectRef)
	}
	if item.ProposedSelection == nil {
		t.Fatal("proposed_selection missing")
	}
	if item.ProposedSelection.DecisionSubjectRef != selection.DecisionSubjectRef {
		t.Fatalf("proposed_selection subject = %q, selection_template subject = %q", item.ProposedSelection.DecisionSubjectRef, selection.DecisionSubjectRef)
	}
	if strings.Contains(item.SelectionTemplate, "decision_carrier_hint") ||
		strings.Contains(item.SelectionTemplate, "review_commands") {
		t.Fatalf("selection_template copied review-only hints:\n%s", item.SelectionTemplate)
	}
	proposedData, err := json.Marshal(item.ProposedSelection)
	if err != nil {
		t.Fatalf("marshal proposed_selection: %v", err)
	}
	proposedText := string(proposedData)
	if strings.Contains(proposedText, "decision_carrier_hint") ||
		strings.Contains(proposedText, "review_commands") {
		t.Fatalf("proposed_selection copied review-only hints:\n%s", proposedText)
	}
	if len(selection.GovernanceTargets) != 1 {
		t.Fatalf("governance_targets = %#v, want one prefilled target", selection.GovernanceTargets)
	}
	if len(item.ProposedSelection.GovernanceTargets) != 1 {
		t.Fatalf("proposed_selection governance_targets = %#v, want one prefilled target", item.ProposedSelection.GovernanceTargets)
	}
	if selection.GovernanceTargets[0].Kind != "symbol" || selection.GovernanceTargets[0].Ref != "symbol:internal/fpf/specsearch.go:type:Route" {
		t.Fatalf("governance_target = %#v", selection.GovernanceTargets[0])
	}
	if len(selection.DriftWatchTargets) != 0 {
		t.Fatalf("drift_watch_targets = %#v, want none when governance target is already known", selection.DriftWatchTargets)
	}
}

func TestDecisionReconciliationSelectionDraftPreservesExistingSubjectRef(t *testing.T) {
	plan := BuildDecisionReconciliationPlanFromItems([]DecisionReconciliationItem{{
		DecisionID:                "dec-mixed",
		DecisionTitle:             "Mixed precise and fallback scope",
		Status:                    StatusActive,
		DecisionSubjectRef:        "decision_reconciliation:r9_scope_enrichment",
		GovernanceTargets:         []string{"api_contract:haft decision reconcile apply operation=enrich_scope"},
		WholeFileFallbackTargets:  []string{"whole_file_fallback:.haft/solutions/sol-old.md"},
		AffectedFiles:             []string{"internal/artifact/reconciliation.go"},
		DecisionSubjectResolution: "explicit_decision_subject_ref",
	}})

	draft := BuildDecisionReconciliationSelectionDraft(plan)

	if len(draft.Items) != 1 {
		t.Fatalf("items = %#v", draft.Items)
	}
	item := draft.Items[0]
	if item.CandidatePosture != "mixed_precise_and_fallback_target_repair_needed" {
		t.Fatalf("candidate_posture = %q", item.CandidatePosture)
	}
	if item.Confidence != "low" {
		t.Fatalf("confidence = %q", item.Confidence)
	}
	if item.ProposedSelection == nil {
		t.Fatal("proposed_selection missing")
	}
	if item.ProposedSelection.DecisionSubjectRef != "decision_reconciliation:r9_scope_enrichment" {
		t.Fatalf("proposed_selection decision_subject_ref = %q", item.ProposedSelection.DecisionSubjectRef)
	}
	if !containsString(item.ProposedSelection.RemoveWholeFileFallbacks, "whole_file_fallback:.haft/solutions/sol-old.md") {
		t.Fatalf("proposed_selection remove_whole_file_fallback_targets = %#v", item.ProposedSelection.RemoveWholeFileFallbacks)
	}
	if containsString(item.ApprovalReadiness.PlaceholderFields, "items[].decision_subject_ref") {
		t.Fatalf("approval_readiness still asks for subject placeholder: %#v", item.ApprovalReadiness.PlaceholderFields)
	}
	if !containsString(item.ApprovalReadiness.PlaceholderFields, "operator_approval_ref") ||
		!containsString(item.ApprovalReadiness.PlaceholderFields, "items[].reason") {
		t.Fatalf("approval_readiness missing remaining placeholders: %#v", item.ApprovalReadiness.PlaceholderFields)
	}
	if !containsString(item.ApprovalReadiness.RequiredOperatorChecks, "replace whole-file fallback targets unless no semantic target can be recovered") {
		t.Fatalf("operator checks = %#v", item.ApprovalReadiness.RequiredOperatorChecks)
	}
	if item.ApprovalReadiness.ApplyReady {
		t.Fatal("approval_readiness.apply_ready = true; preserving subject must not create approval")
	}
}

func TestDecisionReconciliationSelectionDraftPrioritizesReviewableCandidates(t *testing.T) {
	plan := DecisionReconciliationPlan{
		SchemaVersion: DecisionReconciliationSchemaVersion,
		Authority:     DecisionReconciliationAuthority,
		Groups: []DecisionReconciliationGroup{
			{
				GroupID: "group-a-low",
				Preview: DecisionReconciliationPreview{
					ApplyOperation: DecisionReconciliationOperationEnrichScope,
				},
				Decisions: []DecisionReconciliationItem{{
					DecisionID:    "dec-low",
					DecisionTitle: "Low confidence broad scope",
				}},
			},
			{
				GroupID: "group-z-medium",
				Preview: DecisionReconciliationPreview{
					ApplyOperation: DecisionReconciliationOperationEnrichScope,
				},
				Decisions: []DecisionReconciliationItem{{
					DecisionID:        "dec-medium",
					DecisionTitle:     "Medium confidence precise target",
					GovernanceTargets: []string{"symbol:internal/fpf/specsearch.go:type:Route"},
				}},
			},
		},
	}

	draft := BuildDecisionReconciliationSelectionDraftFiltered(
		plan,
		DecisionReconciliationSelectionDraftFilter{Limit: 1},
	)

	if draft.Summary.OperatorApprovalCandidates != 2 {
		t.Fatalf("operator_approval_candidates = %d, want 2", draft.Summary.OperatorApprovalCandidates)
	}
	if draft.Summary.EmittedCandidates != 1 || draft.Summary.OmittedCandidates != 1 {
		t.Fatalf("bounded summary = %#v, want one emitted and one omitted", draft.Summary)
	}
	if len(draft.Items) != 1 {
		t.Fatalf("items = %#v, want one bounded candidate", draft.Items)
	}
	item := draft.Items[0]
	if item.DecisionRef != "dec-medium" {
		t.Fatalf("first bounded candidate = %#v, want medium-confidence precise-target candidate", item)
	}
	if item.Confidence != "medium" {
		t.Fatalf("confidence = %q, want medium", item.Confidence)
	}
}

func TestDecisionReconciliationSelectionDraftIncludesNonApprovedDocumentTemplate(t *testing.T) {
	plan := DecisionReconciliationPlan{
		SchemaVersion: DecisionReconciliationSchemaVersion,
		Authority:     DecisionReconciliationAuthority,
		Groups: []DecisionReconciliationGroup{{
			GroupID: "group-medium",
			Preview: DecisionReconciliationPreview{
				ApplyOperation: DecisionReconciliationOperationEnrichScope,
			},
			Decisions: []DecisionReconciliationItem{{
				DecisionID:        "dec-medium",
				DecisionTitle:     "Medium confidence precise target",
				GovernanceTargets: []string{"symbol:internal/fpf/specsearch.go:type:Route"},
			}},
		}},
	}

	draft := BuildDecisionReconciliationSelectionDraftFiltered(
		plan,
		DecisionReconciliationSelectionDraftFilter{Limit: 1},
	)

	template := draft.SelectionDocumentTemplate
	if template == nil {
		t.Fatal("selection_document_template missing")
	}
	if draft.OperatorApproved {
		t.Fatal("draft must remain not operator approved")
	}
	if template.Authority != DecisionReconciliationSelectionApplyAuthority {
		t.Fatalf("template authority = %q", template.Authority)
	}
	if template.OperatorApprovalRef != "" {
		t.Fatalf("operator_approval_ref = %q, want empty placeholder so apply rejects draft", template.OperatorApprovalRef)
	}
	if len(template.Items) != 1 || template.Items[0].DecisionRefs[0] != "dec-medium" {
		t.Fatalf("template items = %#v", template.Items)
	}

	store := setupTestDB(t)
	review := ReviewDecisionReconciliationSelectionDocument(
		context.Background(),
		store,
		*template,
		"selection-template.json",
	)
	if review.ApplyReady {
		t.Fatalf("template review must not be apply ready without operator approval: %#v", review)
	}
	if !containsString(review.ValidationErrors, "operator_approval_ref is required") {
		t.Fatalf("validation_errors = %#v, want missing operator approval", review.ValidationErrors)
	}
}

func TestDecisionReconciliationSelectionDraftFilterKeepsReportOnlyBatch(t *testing.T) {
	plan := DecisionReconciliationPlan{
		SchemaVersion: DecisionReconciliationSchemaVersion,
		Authority:     DecisionReconciliationAuthority,
		Groups: []DecisionReconciliationGroup{
			{
				GroupID: "group-1",
				Preview: DecisionReconciliationPreview{
					ApplyOperation: DecisionReconciliationOperationEnrichScope,
				},
				Decisions: []DecisionReconciliationItem{
					{
						DecisionID:    "dec-1",
						DecisionTitle: "First",
						AffectedFiles: []string{"one.go"},
					},
					{
						DecisionID:    "dec-2",
						DecisionTitle: "Second",
						AffectedFiles: []string{"two.go"},
					},
				},
			},
			{
				GroupID: "group-2",
				Preview: DecisionReconciliationPreview{
					ApplyOperation: DecisionReconciliationOperationEnrichScope,
				},
				Decisions: []DecisionReconciliationItem{{
					DecisionID:    "dec-3",
					DecisionTitle: "Third",
					AffectedFiles: []string{"three.go"},
				}},
			},
		},
	}

	draft := BuildDecisionReconciliationSelectionDraftFiltered(
		plan,
		DecisionReconciliationSelectionDraftFilter{
			Limit:   1,
			GroupID: "group-1",
		},
	)

	if draft.Authority != DecisionReconciliationSelectionDraftAuthority {
		t.Fatalf("authority = %q", draft.Authority)
	}
	if draft.OperatorApproved {
		t.Fatal("filtered draft must not become operator-approved")
	}
	if draft.Summary.ScopeEnrichmentCandidates != 3 {
		t.Fatalf("scope_enrichment_candidates = %d, want full source count 3", draft.Summary.ScopeEnrichmentCandidates)
	}
	if draft.Summary.OperatorApprovalCandidates != 2 {
		t.Fatalf("operator_approval_candidates = %d, want two matching group-1 candidates", draft.Summary.OperatorApprovalCandidates)
	}
	if draft.Summary.EmittedCandidates != 1 || draft.Summary.OmittedCandidates != 1 {
		t.Fatalf("bounded summary = %#v, want one emitted and one omitted", draft.Summary)
	}
	if draft.Summary.SelectedCandidates != 0 {
		t.Fatalf("selected_candidates = %d, want zero selected candidates", draft.Summary.SelectedCandidates)
	}
	if draft.OmittedItems != 1 {
		t.Fatalf("omitted_items = %d, want 1", draft.OmittedItems)
	}
	if !strings.Contains(draft.FullAuditCommand, "--full") {
		t.Fatalf("full_audit_command = %q", draft.FullAuditCommand)
	}
	if len(draft.Items) != 1 {
		t.Fatalf("items = %#v, want one filtered candidate", draft.Items)
	}
	item := draft.Items[0]
	if item.ReviewedGroupID != "group-1" || item.DecisionRef != "dec-1" {
		t.Fatalf("filtered item = %#v, want first candidate from group-1", item)
	}

	fullDraft := BuildDecisionReconciliationSelectionDraftFiltered(
		plan,
		DecisionReconciliationSelectionDraftFilter{
			Limit:   1,
			Full:    true,
			GroupID: "group-1",
		},
	)
	if fullDraft.Summary.EmittedCandidates != 2 || fullDraft.Summary.OmittedCandidates != 0 {
		t.Fatalf("full draft summary = %#v, want all matching candidates emitted", fullDraft.Summary)
	}
	if len(fullDraft.Items) != 2 {
		t.Fatalf("full draft items = %#v, want two matching candidates", fullDraft.Items)
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

func TestValidateDecisionReconciliationSelectionRejectsApprovalPlaceholder(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-placeholder-approval", StatusActive, "runtime", DecisionFields{}, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-placeholder-approval")

	err := ValidateDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "TODO_operator_approval_ref",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-placeholder-approval"},
			DecisionSubjectRef: "runtime:placeholder_approval",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/placeholder_approval",
			}},
			Reason: "Operator approved precise scope enrichment.",
		}},
	})

	if err == nil {
		t.Fatal("expected placeholder operator approval error")
	}
	if !strings.Contains(err.Error(), "operator_approval_ref contains placeholder") {
		t.Fatalf("error = %v, want operator approval placeholder diagnostic", err)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentReportsDraftNotApplyReady(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope-review", StatusActive, "runtime", DecisionFields{}, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-scope-review")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionDraftAuthority,
		OperatorApprovalRef: "",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-scope-review"},
			DecisionSubjectRef: "runtime:review_scope",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/review_scope",
			}},
			Reason: "Prepared for operator review.",
		}},
	}, "selection.json")

	if review.ApplyReady {
		t.Fatalf("apply_ready = true for draft review: %#v", review)
	}
	if review.OperatorApproved {
		t.Fatalf("operator_approved = true for draft review: %#v", review)
	}
	if !containsString(review.ValidationErrors, "authority must be operator_approved_reconciliation_selection") {
		t.Fatalf("validation_errors = %#v", review.ValidationErrors)
	}
	if !containsString(review.ValidationErrors, "operator_approval_ref is required") {
		t.Fatalf("validation_errors = %#v", review.ValidationErrors)
	}
	if len(review.Items) != 1 || !review.Items[0].ApplyReady {
		t.Fatalf("valid item should still be reviewed as item-apply-ready: %#v", review.Items)
	}
	if review.ApplyCommand != "" {
		t.Fatalf("apply_command = %q, want empty for draft", review.ApplyCommand)
	}

	decision, err := store.Get(ctx, "dec-scope-review")
	if err != nil {
		t.Fatalf("load decision: %v", err)
	}
	fields := decision.UnmarshalDecisionFields()
	if fields.DecisionSubjectRef != "" {
		t.Fatalf("review mutated decision_subject_ref = %q", fields.DecisionSubjectRef)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentRejectsTemplatePlaceholders(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-template-placeholder", StatusActive, "runtime", DecisionFields{}, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-template-placeholder")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:approved-template-placeholder",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-template-placeholder"},
			DecisionSubjectRef: "TODO_exact_decision_subject_ref",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/template_placeholder",
			}},
			Reason: "TODO_operator_reviewed_scope_enrichment_reason",
		}},
	}, "selection-template.json")

	if review.ApplyReady {
		t.Fatalf("apply_ready = true for placeholder selection: %#v", review)
	}
	if !containsString(review.ValidationErrors, "items[0].reason contains placeholder \"TODO_operator_reviewed_scope_enrichment_reason\"; replace it with an exact reviewed value") {
		t.Fatalf("top-level validation_errors = %#v, want mirrored placeholder failure", review.ValidationErrors)
	}
	if len(review.Items) != 1 || review.Items[0].ApplyReady {
		t.Fatalf("item review = %#v, want not apply-ready", review.Items)
	}
	if len(review.Items[0].ValidationErrors) != 1 ||
		!strings.Contains(review.Items[0].ValidationErrors[0], "reason contains placeholder") {
		t.Fatalf("item validation_errors = %#v", review.Items[0].ValidationErrors)
	}
	if review.ApplyCommand != "" {
		t.Fatalf("apply_command = %q, want empty for placeholder selection", review.ApplyCommand)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentReportsApprovedReady(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope-ready", StatusActive, "runtime", DecisionFields{}, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-scope-ready")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:approved-scope-ready",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-scope-ready"},
			DecisionSubjectRef: "runtime:ready_scope",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/ready_scope",
			}},
			Reason: "Operator approved precise scope enrichment.",
		}},
	}, "ready-selection.json")

	if !review.ApplyReady {
		t.Fatalf("apply_ready = false: %#v", review)
	}
	if !review.OperatorApproved {
		t.Fatalf("operator_approved = false: %#v", review)
	}
	if review.ApplyCommand != "haft decision reconcile apply ready-selection.json --json" {
		t.Fatalf("apply_command = %q", review.ApplyCommand)
	}
	if len(review.ValidationErrors) != 0 {
		t.Fatalf("validation_errors = %#v", review.ValidationErrors)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentReportsInvalidItem(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope-invalid-review", StatusActive, "runtime", DecisionFields{}, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-scope-invalid-review")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:approved-invalid-review",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-scope-invalid-review"},
			DecisionSubjectRef: "runtime:invalid_review_scope",
			Reason:             "Missing concrete target.",
		}},
	}, "invalid-selection.json")

	if review.ApplyReady {
		t.Fatalf("apply_ready = true for invalid item: %#v", review)
	}
	if len(review.Items) != 1 || review.Items[0].ApplyReady {
		t.Fatalf("item review = %#v", review.Items)
	}
	if len(review.Items[0].ValidationErrors) != 1 ||
		!strings.Contains(review.Items[0].ValidationErrors[0], "governance_targets or drift_watch_targets is required") {
		t.Fatalf("item validation_errors = %#v", review.Items[0].ValidationErrors)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentReportsStaleReviewedGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-stale-group", StatusActive, "runtime", DecisionFields{}, now)
	currentGroupID := reviewedGroupIDForDecisionRefs(t, store, "dec-stale-group")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:approved-stale-group",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    "decision-reconcile-stale",
			DecisionRefs:       []string{"dec-stale-group"},
			DecisionSubjectRef: "runtime:stale_group",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/stale_group",
			}},
			Reason: "Stale group id should not be apply-ready.",
		}},
	}, "stale-selection.json")

	if review.ApplyReady {
		t.Fatalf("apply_ready = true for stale group: %#v", review)
	}
	if len(review.Items) != 1 || review.Items[0].ApplyReady {
		t.Fatalf("item review = %#v", review.Items)
	}
	if len(review.Items[0].ValidationErrors) != 1 ||
		!strings.Contains(review.Items[0].ValidationErrors[0], "is not present in the current DecisionReconciliationPlan") {
		t.Fatalf("item validation_errors = %#v", review.Items[0].ValidationErrors)
	}
	if !strings.Contains(review.Items[0].ValidationErrors[0], "current decision_refs now match reviewed_group_id "+strconv.Quote(currentGroupID)) {
		t.Fatalf("item validation_errors = %#v, want current reviewed_group_id hint", review.Items[0].ValidationErrors)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentRejectsStaleEnrichScopeOperation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	fields := DecisionFields{
		DecisionSubjectRef: "subject:already-scoped",
		GovernanceTargets: []GovernanceTarget{{
			Kind: "api_contract",
			Ref:  "api_contract:haft/already_scoped",
		}},
	}
	createDecisionForReconciliation(t, store, "dec-already-scoped", StatusActive, "runtime", fields, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-already-scoped")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:approved-stale-enrich",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-already-scoped"},
			DecisionSubjectRef: "subject:already-scoped",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/already_scoped",
			}},
			Reason: "Old scope enrichment packet after the current plan no longer needs scope enrichment.",
		}},
	}, "stale-enrich-selection.json")

	if review.ApplyReady {
		t.Fatalf("apply_ready = true for stale enrich_scope selection: %#v", review)
	}
	if len(review.Items) != 1 || review.Items[0].ApplyReady {
		t.Fatalf("item review = %#v, want not apply-ready", review.Items)
	}
	if len(review.Items[0].ValidationErrors) != 1 ||
		!strings.Contains(review.Items[0].ValidationErrors[0], "does not match current reviewed_group_id") {
		t.Fatalf("item validation_errors = %#v", review.Items[0].ValidationErrors)
	}
	if !containsString(review.ValidationErrors, review.Items[0].ValidationErrors[0]) {
		t.Fatalf("top-level validation_errors = %#v, want mirrored item error", review.ValidationErrors)
	}
	if review.ApplyCommand != "" {
		t.Fatalf("apply_command = %q, want empty for stale enrich_scope selection", review.ApplyCommand)
	}
}

func TestReviewDecisionReconciliationSelectionDocumentRejectsUnknownFallbackRemoval(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	fields := DecisionFields{
		DecisionSubjectRef: "subject:fallback-cleanup",
		GovernanceTargets: []GovernanceTarget{{
			Kind: "api_contract",
			Ref:  "api_contract:haft/fallback_cleanup",
		}},
		BindingTargets: []BindingTarget{{
			Kind:     BindingTargetWholeFileFallback,
			FilePath: "docs/old.md",
			Reason:   "legacy fallback",
		}},
	}
	createDecisionForReconciliation(t, store, "dec-fallback-cleanup", StatusActive, "runtime", fields, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-fallback-cleanup")

	review := ReviewDecisionReconciliationSelectionDocument(ctx, store, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:approved-fallback-cleanup",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-fallback-cleanup"},
			DecisionSubjectRef: "subject:fallback-cleanup",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/fallback_cleanup",
			}},
			RemoveWholeFileFallbacks: []string{"whole_file_fallback:docs/missing.md"},
			Reason:                   "Attempt to remove a non-existing fallback should fail closed.",
		}},
	}, "unknown-fallback-removal.json")

	if review.ApplyReady {
		t.Fatalf("apply_ready = true for unknown fallback removal: %#v", review)
	}
	if len(review.Items) != 1 || review.Items[0].ApplyReady {
		t.Fatalf("item review = %#v, want not apply-ready", review.Items)
	}
	if len(review.Items[0].ValidationErrors) != 1 ||
		!strings.Contains(review.Items[0].ValidationErrors[0], "is not an existing whole-file fallback binding_target") {
		t.Fatalf("item validation_errors = %#v", review.Items[0].ValidationErrors)
	}
}

func TestApplyDecisionReconciliationMergeThroughSuccessorPreservesLineage(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	now := time.Now().UTC()
	mergeFields := DecisionFields{
		DecisionSubjectRef: "subject:artifact-merge",
		GovernanceTargets: []GovernanceTarget{{
			Kind: "api_contract",
			Ref:  "api_contract:haft/artifact_merge",
		}},
	}
	createDecisionForReconciliation(t, store, "dec-old-a", StatusActive, "artifact", mergeFields, now)
	createDecisionForReconciliation(t, store, "dec-old-b", StatusActive, "artifact", mergeFields, now)
	createDecisionForReconciliation(t, store, "dec-successor", StatusActive, "artifact", mergeFields, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-old-a", "dec-old-b")

	result, err := ApplyDecisionReconciliationSelections(ctx, store, haftDir, DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-merge",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationMergeThroughSuccessor,
			ReviewedGroupID: groupID,
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
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-obsolete")

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-retire",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationRetireWithoutSuccessor,
			ReviewedGroupID: groupID,
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
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-stale")

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-reopen",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationReopen,
			ReviewedGroupID: groupID,
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
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-claims")

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-claim-lifecycle",
		Items: []DecisionReconciliationSelection{{
			Operation:       DecisionReconciliationOperationClaimLifecycleUpdate,
			ReviewedGroupID: groupID,
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
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-scope")

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-scope-enrichment",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
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

func TestApplyDecisionReconciliationEnrichScopeRemovesNamedWholeFileFallback(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-fallback-remove", StatusActive, "runtime", DecisionFields{
		DecisionSubjectRef: "subject:fallback-remove",
		GovernanceTargets: []GovernanceTarget{{
			Kind: "api_contract",
			Ref:  "api_contract:haft/fallback_remove",
		}},
		BindingTargets: []BindingTarget{
			{
				Kind:       BindingTargetSymbol,
				FilePath:   "internal/artifact/reconciliation.go",
				SymbolKind: "func",
				SymbolName: "ApplyDecisionReconciliationSelections",
			},
			{
				Kind:     BindingTargetWholeFileFallback,
				FilePath: ".haft/solutions/sol-old.md",
				Reason:   "legacy fallback",
			},
		},
	}, now)
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-fallback-remove")

	result, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "chat:operator-approved-fallback-removal",
		Items: []DecisionReconciliationSelection{{
			Operation:          DecisionReconciliationOperationEnrichScope,
			ReviewedGroupID:    groupID,
			DecisionRefs:       []string{"dec-fallback-remove"},
			DecisionSubjectRef: "subject:fallback-remove",
			GovernanceTargets: []GovernanceTarget{{
				Kind: "api_contract",
				Ref:  "api_contract:haft/fallback_remove",
			}},
			RemoveWholeFileFallbacks: []string{"whole_file_fallback:.haft/solutions/sol-old.md"},
			Reason:                   "Operator reviewed precise scope and removed the stale whole-file fallback.",
		}},
	})
	if err != nil {
		t.Fatalf("ApplyDecisionReconciliationSelections: %v", err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applied = %#v", result.Applied)
	}
	if !containsString(result.Applied[0].UpdatedFields, "binding_targets") {
		t.Fatalf("updated fields = %#v", result.Applied[0].UpdatedFields)
	}

	decision, err := store.Get(ctx, "dec-fallback-remove")
	if err != nil {
		t.Fatalf("load enriched decision: %v", err)
	}
	fields := decision.UnmarshalDecisionFields()
	if len(fields.BindingTargets) != 1 {
		t.Fatalf("binding_targets = %#v, want only the precise symbol target", fields.BindingTargets)
	}
	if fields.BindingTargets[0].Kind != BindingTargetSymbol {
		t.Fatalf("binding_targets = %#v, want symbol target retained", fields.BindingTargets)
	}
	if containsString(decisionReconciliationWholeFileTargets(fields.EffectiveDriftBindingTargets()), "whole_file_fallback:.haft/solutions/sol-old.md") {
		t.Fatalf("whole-file fallback target still present: %#v", fields.BindingTargets)
	}
}

func TestApplyDecisionReconciliationEnrichScopeValidatesWholeBatchBeforeMutation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-scope-valid", StatusActive, "runtime", DecisionFields{}, now)
	createDecisionForReconciliation(t, store, "dec-scope-invalid", StatusActive, "runtime", DecisionFields{}, now)
	validGroupID := reviewedGroupIDForDecisionRefs(t, store, "dec-scope-valid")
	invalidGroupID := reviewedGroupIDForDecisionRefs(t, store, "dec-scope-invalid")

	_, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-invalid-scope-batch",
		Items: []DecisionReconciliationSelection{
			{
				Operation:          DecisionReconciliationOperationEnrichScope,
				ReviewedGroupID:    validGroupID,
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
				ReviewedGroupID:    invalidGroupID,
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
	groupID := reviewedGroupIDForDecisionRefs(t, store, "dec-stays-active")

	_, err := ApplyDecisionReconciliationSelections(ctx, store, t.TempDir(), DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           "operator_approved_reconciliation_selection",
		OperatorApprovalRef: "chat:operator-approved-invalid-batch",
		Items: []DecisionReconciliationSelection{
			{
				Operation:       DecisionReconciliationOperationRetireWithoutSuccessor,
				ReviewedGroupID: groupID,
				DecisionRefs:    []string{"dec-stays-active"},
				Reason:          "Would be valid alone.",
			},
			{
				Operation:       DecisionReconciliationOperationSupersede,
				ReviewedGroupID: groupID,
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

func reviewedGroupIDForDecisionRefs(t *testing.T, store *Store, refs ...string) string {
	t.Helper()

	plan, err := BuildDecisionReconciliationPlan(context.Background(), store)
	if err != nil {
		t.Fatalf("BuildDecisionReconciliationPlan: %v", err)
	}
	wanted := stringSet(compactSortedStrings(refs))
	for _, group := range plan.Groups {
		groupRefs := stringSet(compactSortedStrings(group.DecisionRefs))
		matched := 0
		for ref := range wanted {
			if _, ok := groupRefs[ref]; ok {
				matched++
			}
		}
		if matched == len(wanted) {
			return group.GroupID
		}
	}
	t.Fatalf("no reconciliation group contains decision refs %v in plan %#v", refs, plan.Groups)
	return ""
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
