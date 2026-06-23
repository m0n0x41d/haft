package artifact

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	DecisionReconciliationSchemaVersion = 1
	DecisionReconciliationAuthority     = "report_only_not_binding_authority"

	DecisionReconciliationKeep                     = "keep"
	DecisionReconciliationReopenCandidate          = "reopen_candidate"
	DecisionReconciliationMergeCandidate           = "merge_candidate"
	DecisionReconciliationSupersedeCandidate       = "supersede_candidate"
	DecisionReconciliationRetireWithoutSuccessor   = "retire_without_successor_candidate"
	DecisionReconciliationConflictRequiresOperator = "conflict_requires_operator"

	DecisionReconciliationOperationSupersede              = "supersede"
	DecisionReconciliationOperationMergeThroughSuccessor  = "merge_through_successor"
	DecisionReconciliationOperationRetireWithoutSuccessor = "retire_without_successor"
	DecisionReconciliationOperationReopen                 = "reopen"
	DecisionReconciliationOperationEnrichScope            = "enrich_scope"
	DecisionReconciliationOperationClaimLifecycleUpdate   = "claim_lifecycle_update"

	DecisionReconciliationSelectionDraftAuthority  = "report_only_selection_draft_not_operator_approval"
	DecisionReconciliationSelectionReviewAuthority = "read_only_selection_review_not_apply_authority"
	DecisionReconciliationSelectionApplyAuthority  = "operator_approved_reconciliation_selection"
)

type DecisionReconciliationPlan struct {
	SchemaVersion     int                                  `json:"schema_version"`
	Authority         string                               `json:"authority"`
	View              string                               `json:"view,omitempty"`
	FileOverlapPolicy string                               `json:"file_overlap_policy"`
	Summary           DecisionReconciliationSummary        `json:"summary"`
	CompactGroups     []DecisionReconciliationCompactGroup `json:"compact_groups,omitempty"`
	OmittedGroups     int                                  `json:"omitted_groups,omitempty"`
	FullAuditCommand  string                               `json:"full_audit_command,omitempty"`
	Groups            []DecisionReconciliationGroup        `json:"groups,omitempty"`
}

type DecisionReconciliationSummary struct {
	ReviewedDecisions                int `json:"reviewed_decisions"`
	Groups                           int `json:"groups"`
	Keep                             int `json:"keep"`
	ReopenCandidates                 int `json:"reopen_candidates"`
	MergeCandidates                  int `json:"merge_candidates"`
	SupersedeCandidates              int `json:"supersede_candidates"`
	RetireWithoutSuccessorCandidates int `json:"retire_without_successor_candidates"`
	ConflictRequiresOperator         int `json:"conflict_requires_operator"`
	MissingExplicitSubject           int `json:"missing_explicit_subject"`
	WholeFileFallbackOnly            int `json:"whole_file_fallback_only"`
	ScopeEnrichmentCandidates        int `json:"scope_enrichment_candidates"`
}

type DecisionReconciliationGroup struct {
	GroupID           string                        `json:"group_id"`
	Category          string                        `json:"category"`
	SubjectRef        string                        `json:"subject_ref"`
	SubjectResolution string                        `json:"subject_resolution"`
	BoundedContext    string                        `json:"bounded_context"`
	GovernanceTargets []string                      `json:"governance_targets,omitempty"`
	AffectedFiles     []string                      `json:"affected_files,omitempty"`
	DecisionRefs      []string                      `json:"decision_refs"`
	Decisions         []DecisionReconciliationItem  `json:"decisions"`
	Basis             []string                      `json:"basis"`
	ScopeRepairHints  []string                      `json:"scope_repair_hints,omitempty"`
	Confidence        string                        `json:"confidence"`
	OperatorRequired  bool                          `json:"operator_required"`
	Preview           DecisionReconciliationPreview `json:"preview"`
}

type DecisionReconciliationCompactGroup struct {
	GroupID                     string   `json:"group_id"`
	Category                    string   `json:"category"`
	SubjectRef                  string   `json:"subject_ref"`
	SubjectResolution           string   `json:"subject_resolution"`
	BoundedContext              string   `json:"bounded_context,omitempty"`
	DecisionRefs                []string `json:"decision_refs"`
	Fanout                      int      `json:"fanout"`
	OperatorRequired            bool     `json:"operator_required"`
	PreviewOperation            string   `json:"preview_operation,omitempty"`
	ApplyOperation              string   `json:"apply_operation,omitempty"`
	DownstreamDependents        int      `json:"downstream_dependents,omitempty"`
	DownstreamMigrationRequired bool     `json:"downstream_migration_required,omitempty"`
	SuccessorWorkflowRequired   bool     `json:"successor_workflow_required,omitempty"`
	ScopeRepairHints            []string `json:"scope_repair_hints,omitempty"`
}

type DecisionReconciliationItem struct {
	DecisionID                string                 `json:"decision_id"`
	DecisionTitle             string                 `json:"decision_title,omitempty"`
	Status                    Status                 `json:"status"`
	BoundedContext            string                 `json:"bounded_context,omitempty"`
	DecisionSubjectRef        string                 `json:"decision_subject_ref,omitempty"`
	DecisionSubjectResolution string                 `json:"decision_subject_resolution"`
	GovernanceTargets         []string               `json:"governance_targets,omitempty"`
	WholeFileFallbackTargets  []string               `json:"whole_file_fallback_targets,omitempty"`
	ScopeRepairHint           string                 `json:"scope_repair_hint,omitempty"`
	AffectedFiles             []string               `json:"affected_files,omitempty"`
	Links                     []Link                 `json:"links,omitempty"`
	Backlinks                 []Link                 `json:"backlinks,omitempty"`
	ClaimLifecycle            *ClaimLifecycleSummary `json:"claim_lifecycle,omitempty"`
}

type DecisionReconciliationPreview struct {
	Authority               string                             `json:"authority"`
	ReadOnly                bool                               `json:"read_only"`
	Operation               string                             `json:"operation"`
	ApplyOperation          string                             `json:"apply_operation,omitempty"`
	Current                 DecisionReconciliationPreviewState `json:"current"`
	Proposed                DecisionReconciliationPreviewState `json:"proposed"`
	RequiredSelectionFields []string                           `json:"required_selection_fields,omitempty"`
	ValidationNotes         []string                           `json:"validation_notes,omitempty"`
	DownstreamImpact        *DecisionReconciliationDownstream  `json:"downstream_impact,omitempty"`
	DownstreamMigration     *DecisionReconciliationMigration   `json:"downstream_migration_report,omitempty"`
	SuccessorWorkflow       *DecisionReconciliationSuccessor   `json:"consolidated_successor_workflow,omitempty"`
	DownstreamReview        []string                           `json:"downstream_review,omitempty"`
	MutationBoundary        []string                           `json:"mutation_boundary"`
	ApprovalCue             string                             `json:"approval_cue,omitempty"`
}

type DecisionReconciliationDownstream struct {
	InternalEdges int                                    `json:"internal_edges"`
	ExternalEdges int                                    `json:"external_edges"`
	IncomingEdges int                                    `json:"incoming_edges"`
	OutgoingEdges int                                    `json:"outgoing_edges"`
	DependentRefs []string                               `json:"dependent_refs,omitempty"`
	Edges         []DecisionReconciliationDownstreamEdge `json:"edges,omitempty"`
	ReviewCue     string                                 `json:"review_cue,omitempty"`
}

type DecisionReconciliationDownstreamEdge struct {
	Direction   string `json:"direction"`
	DecisionRef string `json:"decision_ref"`
	LinkType    string `json:"link_type"`
	Ref         string `json:"ref"`
	Scope       string `json:"scope"`
}

type DecisionReconciliationMigration struct {
	RequiredBeforeApply bool     `json:"required_before_apply"`
	AutoRelink          bool     `json:"auto_relink"`
	Policy              string   `json:"policy"`
	DependentRefs       []string `json:"dependent_refs,omitempty"`
	ReviewSteps         []string `json:"review_steps,omitempty"`
	SelectionImpact     []string `json:"selection_impact,omitempty"`
}

type DecisionReconciliationSuccessor struct {
	Required                     bool     `json:"required"`
	Authority                    string   `json:"authority"`
	BindingPath                  string   `json:"binding_path"`
	ExistingSuccessorRefRequired bool     `json:"existing_successor_ref_required"`
	RequiredPacketFields         []string `json:"required_packet_fields,omitempty"`
	ReviewSteps                  []string `json:"review_steps,omitempty"`
	MutationBoundary             []string `json:"mutation_boundary,omitempty"`
}

type DecisionReconciliationPreviewState struct {
	DecisionRefs          []string                                `json:"decision_refs,omitempty"`
	Statuses              []DecisionReconciliationPreviewStatus   `json:"statuses,omitempty"`
	SubjectRef            string                                  `json:"subject_ref,omitempty"`
	BoundedContext        string                                  `json:"bounded_context,omitempty"`
	GovernanceTargets     []string                                `json:"governance_targets,omitempty"`
	ScopeRepairHints      []string                                `json:"scope_repair_hints,omitempty"`
	LineageRefs           []string                                `json:"lineage_refs,omitempty"`
	LineageRelations      []DecisionReconciliationLineageRelation `json:"lineage_relations,omitempty"`
	Effects               []string                                `json:"effects,omitempty"`
	RequiredSuccessorRef  bool                                    `json:"required_successor_ref,omitempty"`
	RequiresProblemReview bool                                    `json:"requires_problem_review,omitempty"`
}

type DecisionReconciliationPreviewStatus struct {
	DecisionRef string `json:"decision_ref"`
	Status      string `json:"status"`
}

type DecisionReconciliationLineageRelation struct {
	Relation             string `json:"relation"`
	SourceRef            string `json:"source_ref"`
	TargetRef            string `json:"target_ref,omitempty"`
	RequiresSuccessorRef bool   `json:"requires_successor_ref,omitempty"`
	Note                 string `json:"note,omitempty"`
}

type DecisionReconciliationSelectionDocument struct {
	SchemaVersion       int                               `json:"schema_version"`
	Authority           string                            `json:"authority"`
	OperatorApprovalRef string                            `json:"operator_approval_ref"`
	Items               []DecisionReconciliationSelection `json:"items"`
}

type DecisionReconciliationSelectionDraft struct {
	SchemaVersion             int                                      `json:"schema_version"`
	Authority                 string                                   `json:"authority"`
	OperatorApproved          bool                                     `json:"operator_approved"`
	ApplyAuthorityRequired    string                                   `json:"apply_authority_required"`
	SourcePlanAuthority       string                                   `json:"source_plan_authority"`
	Summary                   DecisionReconciliationDraftSummary       `json:"summary"`
	OmittedItems              int                                      `json:"omitted_items,omitempty"`
	FullAuditCommand          string                                   `json:"full_audit_command,omitempty"`
	Items                     []DecisionReconciliationDraftItem        `json:"items,omitempty"`
	SelectionDocumentTemplate *DecisionReconciliationSelectionDocument `json:"selection_document_template,omitempty"`
	MutationBoundary          []string                                 `json:"mutation_boundary"`
	NextSteps                 []string                                 `json:"next_steps"`
}

type DecisionReconciliationSelectionReview struct {
	SchemaVersion       int                                         `json:"schema_version"`
	Authority           string                                      `json:"authority"`
	ApplyReady          bool                                        `json:"apply_ready"`
	OperatorApproved    bool                                        `json:"operator_approved"`
	DocumentAuthority   string                                      `json:"document_authority"`
	OperatorApprovalRef string                                      `json:"operator_approval_ref,omitempty"`
	RequiredAuthority   string                                      `json:"required_authority"`
	ItemCount           int                                         `json:"item_count"`
	ValidationErrors    []string                                    `json:"validation_errors,omitempty"`
	Items               []DecisionReconciliationSelectionReviewItem `json:"items,omitempty"`
	ApplyCommand        string                                      `json:"apply_command,omitempty"`
	MutationBoundary    []string                                    `json:"mutation_boundary"`
	NextSteps           []string                                    `json:"next_steps"`
}

type DecisionReconciliationSelectionReviewItem struct {
	Index            int      `json:"index"`
	Operation        string   `json:"operation,omitempty"`
	ReviewedGroupID  string   `json:"reviewed_group_id,omitempty"`
	DecisionRefs     []string `json:"decision_refs,omitempty"`
	ApplyReady       bool     `json:"apply_ready"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
}

type DecisionReconciliationSelectionDraftFilter struct {
	Limit       int    `json:"limit,omitempty"`
	Full        bool   `json:"full,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	DecisionRef string `json:"decision_ref,omitempty"`
}

type DecisionReconciliationDraftSummary struct {
	ReviewedGroups             int `json:"reviewed_groups"`
	ScopeEnrichmentCandidates  int `json:"scope_enrichment_candidates"`
	OperatorApprovalCandidates int `json:"operator_approval_candidates"`
	EmittedCandidates          int `json:"emitted_candidates"`
	OmittedCandidates          int `json:"omitted_candidates"`
	SelectedCandidates         int `json:"selected_candidates"`
}

type DecisionReconciliationDraftItem struct {
	Operation                     string   `json:"operation"`
	ReviewedGroupID               string   `json:"reviewed_group_id"`
	DecisionRef                   string   `json:"decision_ref"`
	DecisionTitle                 string   `json:"decision_title,omitempty"`
	DecisionCarrierHint           string   `json:"decision_carrier_hint,omitempty"`
	CandidatePosture              string   `json:"candidate_posture,omitempty"`
	Confidence                    string   `json:"confidence,omitempty"`
	CurrentSubjectRef             string   `json:"current_subject_ref,omitempty"`
	DecisionSubjectRefSuggestions []string `json:"decision_subject_ref_suggestions,omitempty"`
	CurrentGovernanceTargets      []string `json:"current_governance_targets,omitempty"`
	WholeFileFallbackTargets      []string `json:"whole_file_fallback_targets,omitempty"`
	AffectedFiles                 []string `json:"affected_files,omitempty"`
	ScopeRepairHint               string   `json:"scope_repair_hint,omitempty"`
	SuggestedReviewAction         string   `json:"suggested_review_action,omitempty"`
	BlockingQuestions             []string `json:"blocking_questions,omitempty"`
	RequiredSelectionFields       []string `json:"required_selection_fields,omitempty"`
	ReviewCommands                []string `json:"review_commands,omitempty"`
	SelectionTemplate             string   `json:"selection_template"`
	ReviewNotes                   []string `json:"review_notes"`
}

type DecisionReconciliationSelection struct {
	Operation                 string                                       `json:"operation"`
	ReviewedGroupID           string                                       `json:"reviewed_group_id"`
	DecisionRefs              []string                                     `json:"decision_refs"`
	SuccessorRef              string                                       `json:"successor_ref,omitempty"`
	DecisionSubjectRef        string                                       `json:"decision_subject_ref,omitempty"`
	GovernanceTargets         []GovernanceTarget                           `json:"governance_targets,omitempty"`
	DriftWatchTargets         []DriftWatchTarget                           `json:"drift_watch_targets,omitempty"`
	ClaimGovernanceTargetRefs map[string][]string                          `json:"claim_governance_target_refs,omitempty"`
	ClaimLifecycleUpdates     []DecisionReconciliationClaimLifecycleUpdate `json:"claim_lifecycle_updates,omitempty"`
	Reason                    string                                       `json:"reason"`
}

type decisionReconciliationPlanIndex struct {
	groups map[string]DecisionReconciliationGroup
}

func BuildDecisionReconciliationSelectionDraft(
	plan DecisionReconciliationPlan,
) DecisionReconciliationSelectionDraft {
	return BuildDecisionReconciliationSelectionDraftFiltered(plan, DecisionReconciliationSelectionDraftFilter{})
}

func BuildDecisionReconciliationSelectionDraftFiltered(
	plan DecisionReconciliationPlan,
	filter DecisionReconciliationSelectionDraftFilter,
) DecisionReconciliationSelectionDraft {
	allItems := decisionReconciliationDraftItems(plan.Groups)
	filteredItems := filterDecisionReconciliationDraftItems(allItems, filter)
	items := limitDecisionReconciliationDraftItems(filteredItems, filter)
	omittedItems := len(filteredItems) - len(items)
	return DecisionReconciliationSelectionDraft{
		SchemaVersion:          DecisionReconciliationSchemaVersion,
		Authority:              DecisionReconciliationSelectionDraftAuthority,
		OperatorApproved:       false,
		ApplyAuthorityRequired: "operator_approved_reconciliation_selection",
		SourcePlanAuthority:    plan.Authority,
		Summary: DecisionReconciliationDraftSummary{
			ReviewedGroups:             len(plan.Groups),
			ScopeEnrichmentCandidates:  len(allItems),
			OperatorApprovalCandidates: len(filteredItems),
			EmittedCandidates:          len(items),
			OmittedCandidates:          omittedItems,
			SelectedCandidates:         countDecisionReconciliationDraftSelectedCandidates(filteredItems),
		},
		OmittedItems:              omittedItems,
		FullAuditCommand:          "haft decision reconcile selection-draft --json --full",
		Items:                     items,
		SelectionDocumentTemplate: decisionReconciliationDraftSelectionDocumentTemplate(items),
		MutationBoundary: []string{
			"selection draft is read-only",
			"draft output is not an operator approval",
			"draft output is not accepted by apply as authority",
			"apply still requires an operator-approved selection document",
			"enrich_scope does not change decision status, lineage, evidence, baselines, or gates",
		},
		NextSteps: []string{
			"review each candidate and replace template placeholders with exact subject and target refs",
			"remove candidates that are uncertain or too broad",
			"copy selection_document_template only after operator review and fill operator_approval_ref after explicit approval",
			"apply with haft decision reconcile apply SELECTION.json --json",
			"rerun haft decision reconcile metrics --json before and after apply",
		},
	}
}

func countDecisionReconciliationDraftSelectedCandidates(items []DecisionReconciliationDraftItem) int {
	total := 0
	for _, item := range items {
		if item.Confidence == "high" {
			total++
		}
	}
	return total
}

type DecisionReconciliationClaimLifecycleUpdate struct {
	DecisionRef     string               `json:"decision_ref"`
	ClaimID         string               `json:"claim_id"`
	LifecycleStatus ClaimLifecycleStatus `json:"lifecycle_status"`
	SuccessorRef    string               `json:"successor_ref,omitempty"`
	Reason          string               `json:"reason"`
}

type DecisionReconciliationApplyResult struct {
	SchemaVersion int                                  `json:"schema_version"`
	Authority     string                               `json:"authority"`
	Applied       []DecisionReconciliationApplyOutcome `json:"applied"`
}

type DecisionReconciliationApplyOutcome struct {
	Operation        string                                       `json:"operation"`
	ReviewedGroupID  string                                       `json:"reviewed_group_id,omitempty"`
	DecisionRefs     []string                                     `json:"decision_refs"`
	SuccessorRef     string                                       `json:"successor_ref,omitempty"`
	ProblemRefs      []string                                     `json:"problem_refs,omitempty"`
	UpdatedFields    []string                                     `json:"updated_fields,omitempty"`
	ClaimUpdates     []DecisionReconciliationClaimLifecycleUpdate `json:"claim_lifecycle_updates,omitempty"`
	LineageRelations []DecisionReconciliationLineageRelation      `json:"lineage_relations,omitempty"`
	Status           string                                       `json:"status"`
}

type decisionReconciliationBucket struct {
	key               string
	subjectRef        string
	subjectResolution string
	boundedContext    string
	targets           map[string]struct{}
	files             map[string]struct{}
	items             []DecisionReconciliationItem
}

func ValidateDecisionReconciliationSelectionDocument(
	ctx context.Context,
	store ArtifactStore,
	document DecisionReconciliationSelectionDocument,
) error {
	if document.SchemaVersion != DecisionReconciliationSchemaVersion {
		return fmt.Errorf("schema_version must be %d", DecisionReconciliationSchemaVersion)
	}
	if strings.TrimSpace(document.Authority) != DecisionReconciliationSelectionApplyAuthority {
		return fmt.Errorf("authority must be %s", DecisionReconciliationSelectionApplyAuthority)
	}
	if strings.TrimSpace(document.OperatorApprovalRef) == "" {
		return errors.New("operator_approval_ref is required")
	}
	if err := validateDecisionReconciliationNoPlaceholder("operator_approval_ref", document.OperatorApprovalRef); err != nil {
		return err
	}
	if len(document.Items) == 0 {
		return errors.New("items must contain at least one selection")
	}
	planIndex, err := buildDecisionReconciliationPlanIndex(ctx, store)
	if err != nil {
		return err
	}
	for index, item := range document.Items {
		if err := validateDecisionReconciliationSelection(ctx, store, planIndex, index, item); err != nil {
			return err
		}
	}
	return nil
}

func ReviewDecisionReconciliationSelectionDocument(
	ctx context.Context,
	store ArtifactStore,
	document DecisionReconciliationSelectionDocument,
	sourcePath string,
) DecisionReconciliationSelectionReview {
	review := DecisionReconciliationSelectionReview{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionReviewAuthority,
		DocumentAuthority:   strings.TrimSpace(document.Authority),
		OperatorApprovalRef: strings.TrimSpace(document.OperatorApprovalRef),
		RequiredAuthority:   DecisionReconciliationSelectionApplyAuthority,
		ItemCount:           len(document.Items),
		MutationBoundary: []string{
			"selection review is read-only",
			"review does not create operator approval",
			"review does not apply reconciliation selections",
			"apply remains a separate operator-approved command",
		},
	}

	review.OperatorApproved = review.DocumentAuthority == review.RequiredAuthority &&
		review.OperatorApprovalRef != ""
	review.ValidationErrors = decisionReconciliationDocumentReviewErrors(document)
	planIndex, err := buildDecisionReconciliationPlanIndex(ctx, store)
	if err != nil {
		review.ValidationErrors = appendMissingString(review.ValidationErrors, err.Error())
	}
	review.Items = decisionReconciliationItemReviews(ctx, store, planIndex, document.Items)
	review.ValidationErrors = appendMissingString(
		review.ValidationErrors,
		decisionReconciliationStrictValidationError(ctx, store, document),
	)
	review.ApplyReady = len(review.ValidationErrors) == 0 &&
		decisionReconciliationReviewItemsReady(review.Items)
	if review.ApplyReady {
		review.ApplyCommand = decisionReconciliationApplyCommand(sourcePath)
	}
	review.NextSteps = decisionReconciliationReviewNextSteps(review, sourcePath)
	return review
}

func ApplyDecisionReconciliationSelections(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	document DecisionReconciliationSelectionDocument,
) (DecisionReconciliationApplyResult, error) {
	if err := ValidateDecisionReconciliationSelectionDocument(ctx, store, document); err != nil {
		return DecisionReconciliationApplyResult{}, err
	}

	result := DecisionReconciliationApplyResult{
		SchemaVersion: DecisionReconciliationSchemaVersion,
		Authority:     "operator_approved_lineage_mutation",
	}
	for _, item := range document.Items {
		outcome, err := applyDecisionReconciliationSelection(ctx, store, haftDir, item)
		if err != nil {
			return DecisionReconciliationApplyResult{}, err
		}
		result.Applied = append(result.Applied, outcome)
	}
	return result, nil
}

func decisionReconciliationDocumentReviewErrors(
	document DecisionReconciliationSelectionDocument,
) []string {
	errs := make([]string, 0, 4)
	if document.SchemaVersion != DecisionReconciliationSchemaVersion {
		errs = append(errs, fmt.Sprintf("schema_version must be %d", DecisionReconciliationSchemaVersion))
	}
	if strings.TrimSpace(document.Authority) != DecisionReconciliationSelectionApplyAuthority {
		errs = append(errs, fmt.Sprintf("authority must be %s", DecisionReconciliationSelectionApplyAuthority))
	}
	if strings.TrimSpace(document.OperatorApprovalRef) == "" {
		errs = append(errs, "operator_approval_ref is required")
	} else if err := validateDecisionReconciliationNoPlaceholder("operator_approval_ref", document.OperatorApprovalRef); err != nil {
		errs = append(errs, err.Error())
	}
	if len(document.Items) == 0 {
		errs = append(errs, "items must contain at least one selection")
	}
	return errs
}

func decisionReconciliationItemReviews(
	ctx context.Context,
	store ArtifactStore,
	planIndex decisionReconciliationPlanIndex,
	items []DecisionReconciliationSelection,
) []DecisionReconciliationSelectionReviewItem {
	reviews := make([]DecisionReconciliationSelectionReviewItem, 0, len(items))
	for index, item := range items {
		review := DecisionReconciliationSelectionReviewItem{
			Index:           index,
			Operation:       strings.TrimSpace(item.Operation),
			ReviewedGroupID: strings.TrimSpace(item.ReviewedGroupID),
			DecisionRefs:    compactSortedStrings(item.DecisionRefs),
		}
		if err := validateDecisionReconciliationSelection(ctx, store, planIndex, index, item); err != nil {
			review.ValidationErrors = []string{err.Error()}
		}
		review.ApplyReady = len(review.ValidationErrors) == 0
		reviews = append(reviews, review)
	}
	return reviews
}

func buildDecisionReconciliationPlanIndex(
	ctx context.Context,
	store ArtifactStore,
) (decisionReconciliationPlanIndex, error) {
	plan, err := BuildDecisionReconciliationPlan(ctx, store)
	if err != nil {
		return decisionReconciliationPlanIndex{}, fmt.Errorf("build current reconciliation plan: %w", err)
	}
	groups := make(map[string]DecisionReconciliationGroup, len(plan.Groups))
	for _, group := range plan.Groups {
		groups[group.GroupID] = group
	}
	return decisionReconciliationPlanIndex{groups: groups}, nil
}

func decisionReconciliationStrictValidationError(
	ctx context.Context,
	store ArtifactStore,
	document DecisionReconciliationSelectionDocument,
) string {
	if err := ValidateDecisionReconciliationSelectionDocument(ctx, store, document); err != nil {
		return err.Error()
	}
	return ""
}

func decisionReconciliationReviewItemsReady(
	items []DecisionReconciliationSelectionReviewItem,
) bool {
	for _, item := range items {
		if !item.ApplyReady {
			return false
		}
	}
	return true
}

func decisionReconciliationApplyCommand(sourcePath string) string {
	path := strings.TrimSpace(sourcePath)
	if path == "" {
		path = "SELECTION.json"
	}
	return "haft decision reconcile apply " + path + " --json"
}

func decisionReconciliationReviewNextSteps(
	review DecisionReconciliationSelectionReview,
	sourcePath string,
) []string {
	if review.ApplyReady {
		return []string{
			"capture before metrics with haft decision reconcile metrics --json",
			"apply with " + decisionReconciliationApplyCommand(sourcePath),
			"capture after metrics with haft decision reconcile metrics --json",
		}
	}
	steps := []string{}
	if review.DocumentAuthority != review.RequiredAuthority {
		steps = append(steps, "create a separate selection document with authority="+review.RequiredAuthority+" only after operator approval")
	}
	if review.OperatorApprovalRef == "" {
		steps = append(steps, "add operator_approval_ref that names the explicit approval event")
	}
	if !decisionReconciliationReviewItemsReady(review.Items) {
		steps = append(steps, "fix or remove invalid selection items before apply")
	}
	if len(steps) == 0 {
		steps = append(steps, "fix validation_errors before apply")
	}
	return steps
}

func appendMissingString(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}

func validateDecisionReconciliationSelection(
	ctx context.Context,
	store ArtifactStore,
	planIndex decisionReconciliationPlanIndex,
	index int,
	item DecisionReconciliationSelection,
) error {
	prefix := fmt.Sprintf("items[%d]", index)
	operation := strings.TrimSpace(item.Operation)
	if !isDecisionReconciliationApplyOperation(operation) {
		return fmt.Errorf("%s.operation %q is unsupported", prefix, item.Operation)
	}
	if strings.TrimSpace(item.ReviewedGroupID) == "" {
		return fmt.Errorf("%s.reviewed_group_id is required", prefix)
	}
	if err := validateDecisionReconciliationNoPlaceholder(prefix+".reviewed_group_id", item.ReviewedGroupID); err != nil {
		return err
	}
	if strings.TrimSpace(item.Reason) == "" {
		return fmt.Errorf("%s.reason is required", prefix)
	}
	if err := validateDecisionReconciliationNoPlaceholder(prefix+".reason", item.Reason); err != nil {
		return err
	}
	refs := compactSortedStrings(item.DecisionRefs)
	if len(refs) == 0 {
		return fmt.Errorf("%s.decision_refs must contain at least one decision", prefix)
	}
	for refIndex, ref := range refs {
		if err := validateDecisionReconciliationNoPlaceholder(
			fmt.Sprintf("%s.decision_refs[%d]", prefix, refIndex),
			ref,
		); err != nil {
			return err
		}
	}
	if err := validateDecisionReconciliationReviewedGroup(planIndex, prefix, item.ReviewedGroupID, refs); err != nil {
		return err
	}
	if operation == DecisionReconciliationOperationReopen && len(refs) != 1 {
		return fmt.Errorf("%s.decision_refs must contain exactly one decision for reopen", prefix)
	}
	if operation == DecisionReconciliationOperationEnrichScope && len(refs) != 1 {
		return fmt.Errorf("%s.decision_refs must contain exactly one decision for enrich_scope", prefix)
	}
	if operation == DecisionReconciliationOperationClaimLifecycleUpdate {
		if err := validateDecisionReconciliationClaimLifecycleUpdates(ctx, store, prefix, item, refs); err != nil {
			return err
		}
	}
	if operation == DecisionReconciliationOperationSupersede || operation == DecisionReconciliationOperationMergeThroughSuccessor {
		if strings.TrimSpace(item.SuccessorRef) == "" {
			return fmt.Errorf("%s.successor_ref is required for %s", prefix, operation)
		}
		if err := validateDecisionReconciliationNoPlaceholder(prefix+".successor_ref", item.SuccessorRef); err != nil {
			return err
		}
		if err := validateDecisionReconciliationSuccessor(ctx, store, prefix, item.SuccessorRef); err != nil {
			return err
		}
	}
	if operation == DecisionReconciliationOperationRetireWithoutSuccessor && strings.TrimSpace(item.SuccessorRef) != "" {
		return fmt.Errorf("%s.successor_ref must be empty for retire_without_successor", prefix)
	}
	if operation == DecisionReconciliationOperationEnrichScope {
		if err := validateDecisionReconciliationScopeEnrichment(ctx, store, prefix, item); err != nil {
			return err
		}
	}
	if operation != DecisionReconciliationOperationEnrichScope && selectionHasScopeEnrichment(item) {
		return fmt.Errorf("%s scope enrichment fields are only valid for enrich_scope", prefix)
	}
	if operation != DecisionReconciliationOperationClaimLifecycleUpdate && len(item.ClaimLifecycleUpdates) > 0 {
		return fmt.Errorf("%s claim_lifecycle_updates are only valid for claim_lifecycle_update", prefix)
	}
	for _, ref := range refs {
		if err := validateDecisionReconciliationCurrentDecision(ctx, store, prefix, ref); err != nil {
			return err
		}
		if ref == strings.TrimSpace(item.SuccessorRef) {
			return fmt.Errorf("%s.decision_refs must not include successor_ref %q", prefix, ref)
		}
	}
	return nil
}

func validateDecisionReconciliationReviewedGroup(
	planIndex decisionReconciliationPlanIndex,
	prefix string,
	reviewedGroupID string,
	decisionRefs []string,
) error {
	groupID := strings.TrimSpace(reviewedGroupID)
	group, ok := planIndex.groups[groupID]
	if !ok {
		return fmt.Errorf("%s.reviewed_group_id %q is not present in the current DecisionReconciliationPlan; rerun `haft decision reconcile --json` and rebuild the selection", prefix, groupID)
	}
	groupRefs := stringSet(compactSortedStrings(group.DecisionRefs))
	for _, ref := range decisionRefs {
		if _, ok := groupRefs[ref]; !ok {
			return fmt.Errorf("%s.decision_refs contains %q, which is not in reviewed_group_id %q", prefix, ref, groupID)
		}
	}
	return nil
}

func validateDecisionReconciliationClaimLifecycleUpdates(
	ctx context.Context,
	store ArtifactStore,
	prefix string,
	item DecisionReconciliationSelection,
	refs []string,
) error {
	if strings.TrimSpace(item.SuccessorRef) != "" {
		return fmt.Errorf("%s.successor_ref must be empty for claim_lifecycle_update; use claim_lifecycle_updates[].successor_ref", prefix)
	}
	if len(item.ClaimLifecycleUpdates) == 0 {
		return fmt.Errorf("%s.claim_lifecycle_updates must contain at least one claim update", prefix)
	}
	selectedRefs := stringSet(refs)
	claimsByDecision, err := decisionReconciliationClaimsByDecision(ctx, store, refs)
	if err != nil {
		return err
	}
	for updateIndex, update := range item.ClaimLifecycleUpdates {
		updatePrefix := fmt.Sprintf("%s.claim_lifecycle_updates[%d]", prefix, updateIndex)
		decisionRef := strings.TrimSpace(update.DecisionRef)
		claimID := strings.TrimSpace(update.ClaimID)
		if decisionRef == "" {
			return fmt.Errorf("%s.decision_ref is required", updatePrefix)
		}
		if err := validateDecisionReconciliationNoPlaceholder(updatePrefix+".decision_ref", decisionRef); err != nil {
			return err
		}
		if _, ok := selectedRefs[decisionRef]; !ok {
			return fmt.Errorf("%s.decision_ref %q must be listed in %s.decision_refs", updatePrefix, decisionRef, prefix)
		}
		if claimID == "" {
			return fmt.Errorf("%s.claim_id is required", updatePrefix)
		}
		if err := validateDecisionReconciliationNoPlaceholder(updatePrefix+".claim_id", claimID); err != nil {
			return err
		}
		if _, ok := claimsByDecision[decisionRef][claimID]; !ok {
			return fmt.Errorf("%s.claim_id %q does not match an explicit claim on %q", updatePrefix, claimID, decisionRef)
		}
		status := normalizeClaimLifecycleStatus(update.LifecycleStatus)
		if status == "" || status == ClaimLifecycleActive {
			return fmt.Errorf("%s.lifecycle_status must be refresh_due, superseded, or deprecated", updatePrefix)
		}
		if status == ClaimLifecycleSuperseded && strings.TrimSpace(update.SuccessorRef) == "" {
			return fmt.Errorf("%s.successor_ref is required when lifecycle_status is superseded", updatePrefix)
		}
		if err := validateDecisionReconciliationNoPlaceholder(updatePrefix+".successor_ref", update.SuccessorRef); err != nil {
			return err
		}
		if strings.TrimSpace(update.Reason) == "" {
			return fmt.Errorf("%s.reason is required", updatePrefix)
		}
		if err := validateDecisionReconciliationNoPlaceholder(updatePrefix+".reason", update.Reason); err != nil {
			return err
		}
	}
	return nil
}

func decisionReconciliationClaimsByDecision(
	ctx context.Context,
	store ArtifactStore,
	refs []string,
) (map[string]map[string]struct{}, error) {
	out := make(map[string]map[string]struct{}, len(refs))
	for _, ref := range refs {
		artifact, err := store.Get(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("decision_ref %q not found: %w", ref, err)
		}
		fields := artifact.UnmarshalDecisionFields()
		claims := make(map[string]struct{}, len(fields.Claims))
		for _, claim := range fields.Claims {
			claimID := strings.TrimSpace(claim.ID)
			if claimID != "" {
				claims[claimID] = struct{}{}
			}
		}
		out[ref] = claims
	}
	return out, nil
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func validateDecisionReconciliationScopeEnrichment(
	ctx context.Context,
	store ArtifactStore,
	prefix string,
	item DecisionReconciliationSelection,
) error {
	if strings.TrimSpace(item.SuccessorRef) != "" {
		return fmt.Errorf("%s.successor_ref must be empty for enrich_scope", prefix)
	}
	if strings.TrimSpace(item.DecisionSubjectRef) == "" {
		return fmt.Errorf("%s.decision_subject_ref is required for enrich_scope", prefix)
	}
	if err := validateDecisionReconciliationNoPlaceholder(prefix+".decision_subject_ref", item.DecisionSubjectRef); err != nil {
		return err
	}

	governanceTargets := normalizeGovernanceTargets(item.GovernanceTargets)
	driftWatchTargets := normalizeDriftWatchTargets(item.DriftWatchTargets)
	if len(governanceTargets) == 0 && len(driftWatchTargets) == 0 {
		return fmt.Errorf("%s.governance_targets or drift_watch_targets is required for enrich_scope", prefix)
	}
	if err := validateDecisionReconciliationTargetPlaceholders(prefix, governanceTargets, driftWatchTargets); err != nil {
		return err
	}

	ref := compactSortedStrings(item.DecisionRefs)[0]
	decision, err := store.Get(ctx, ref)
	if err != nil {
		return fmt.Errorf("%s.decision_ref %q not found: %w", prefix, ref, err)
	}
	fields := decision.UnmarshalDecisionFields()
	existingSubject := strings.TrimSpace(fields.DecisionSubjectRef)
	nextSubject := strings.TrimSpace(item.DecisionSubjectRef)
	if existingSubject != "" && existingSubject != nextSubject {
		return fmt.Errorf("%s.decision_subject_ref would retarget %q from %q to %q", prefix, ref, existingSubject, nextSubject)
	}

	claimRefs := normalizeScopeEnrichmentClaimRefs(item.ClaimGovernanceTargetRefs)
	if len(claimRefs) == 0 {
		return nil
	}

	claimsByID := map[string]struct{}{}
	for _, claim := range fields.Claims {
		claimsByID[claim.ID] = struct{}{}
	}
	for claimID, targetRefs := range claimRefs {
		if err := validateDecisionReconciliationNoPlaceholder(
			fmt.Sprintf("%s.claim_governance_target_refs[%q]", prefix, claimID),
			claimID,
		); err != nil {
			return err
		}
		if _, ok := claimsByID[claimID]; !ok {
			return fmt.Errorf("%s.claim_governance_target_refs[%q] does not match an explicit claim", prefix, claimID)
		}
		if len(targetRefs) == 0 {
			return fmt.Errorf("%s.claim_governance_target_refs[%q] must contain at least one target ref", prefix, claimID)
		}
		for targetIndex, targetRef := range targetRefs {
			if err := validateDecisionReconciliationNoPlaceholder(
				fmt.Sprintf("%s.claim_governance_target_refs[%q][%d]", prefix, claimID, targetIndex),
				targetRef,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDecisionReconciliationTargetPlaceholders(
	prefix string,
	governanceTargets []GovernanceTarget,
	driftWatchTargets []DriftWatchTarget,
) error {
	for index, target := range governanceTargets {
		targetPrefix := fmt.Sprintf("%s.governance_targets[%d]", prefix, index)
		if err := validateDecisionReconciliationNoPlaceholder(targetPrefix+".kind", target.Kind); err != nil {
			return err
		}
		if err := validateDecisionReconciliationNoPlaceholder(targetPrefix+".ref", target.Ref); err != nil {
			return err
		}
	}
	for index, target := range driftWatchTargets {
		targetPrefix := fmt.Sprintf("%s.drift_watch_targets[%d]", prefix, index)
		if err := validateDecisionReconciliationNoPlaceholder(targetPrefix+".target_ref", target.TargetRef); err != nil {
			return err
		}
		if err := validateDecisionReconciliationNoPlaceholder(targetPrefix+".trigger", target.Trigger); err != nil {
			return err
		}
	}
	return nil
}

func validateDecisionReconciliationNoPlaceholder(field string, value string) error {
	if !isDecisionReconciliationPlaceholder(value) {
		return nil
	}
	return fmt.Errorf("%s contains placeholder %q; replace it with an exact reviewed value", field, strings.TrimSpace(value))
}

func isDecisionReconciliationPlaceholder(value string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	return normalized == "TODO" || strings.Contains(normalized, "TODO_")
}

func selectionHasScopeEnrichment(item DecisionReconciliationSelection) bool {
	if strings.TrimSpace(item.DecisionSubjectRef) != "" {
		return true
	}
	if len(normalizeGovernanceTargets(item.GovernanceTargets)) > 0 {
		return true
	}
	if len(normalizeDriftWatchTargets(item.DriftWatchTargets)) > 0 {
		return true
	}
	return len(normalizeScopeEnrichmentClaimRefs(item.ClaimGovernanceTargetRefs)) > 0
}

func validateDecisionReconciliationSuccessor(
	ctx context.Context,
	store ArtifactStore,
	prefix string,
	ref string,
) error {
	artifact, err := store.Get(ctx, strings.TrimSpace(ref))
	if err != nil {
		return fmt.Errorf("%s.successor_ref %q not found: %w", prefix, ref, err)
	}
	if artifact.Meta.Kind != KindDecisionRecord {
		return fmt.Errorf("%s.successor_ref %q is %s, want DecisionRecord", prefix, ref, artifact.Meta.Kind)
	}
	if !reconciliationStatusInScope(artifact.Meta.Status) {
		return fmt.Errorf("%s.successor_ref %q status %q is not current", prefix, ref, artifact.Meta.Status)
	}
	return nil
}

func validateDecisionReconciliationCurrentDecision(
	ctx context.Context,
	store ArtifactStore,
	prefix string,
	ref string,
) error {
	artifact, err := store.Get(ctx, ref)
	if err != nil {
		return fmt.Errorf("%s.decision_ref %q not found: %w", prefix, ref, err)
	}
	if artifact.Meta.Kind != KindDecisionRecord {
		return fmt.Errorf("%s.decision_ref %q is %s, want DecisionRecord", prefix, ref, artifact.Meta.Kind)
	}
	if !reconciliationStatusInScope(artifact.Meta.Status) {
		return fmt.Errorf("%s.decision_ref %q status %q is not active/refresh_due", prefix, ref, artifact.Meta.Status)
	}
	return nil
}

func applyDecisionReconciliationSelection(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	item DecisionReconciliationSelection,
) (DecisionReconciliationApplyOutcome, error) {
	operation := strings.TrimSpace(item.Operation)
	switch operation {
	case DecisionReconciliationOperationSupersede, DecisionReconciliationOperationMergeThroughSuccessor:
		return applyDecisionReconciliationSupersede(ctx, store, haftDir, item)
	case DecisionReconciliationOperationRetireWithoutSuccessor:
		return applyDecisionReconciliationRetire(ctx, store, haftDir, item)
	case DecisionReconciliationOperationReopen:
		return applyDecisionReconciliationReopen(ctx, store, haftDir, item)
	case DecisionReconciliationOperationEnrichScope:
		return applyDecisionReconciliationEnrichScope(ctx, store, item)
	case DecisionReconciliationOperationClaimLifecycleUpdate:
		return applyDecisionReconciliationClaimLifecycleUpdate(ctx, store, item)
	default:
		return DecisionReconciliationApplyOutcome{}, fmt.Errorf("unsupported operation %q", item.Operation)
	}
}

func applyDecisionReconciliationSupersede(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	item DecisionReconciliationSelection,
) (DecisionReconciliationApplyOutcome, error) {
	refs := compactSortedStrings(item.DecisionRefs)
	for _, ref := range refs {
		if _, err := SupersedeArtifact(ctx, store, haftDir, ref, strings.TrimSpace(item.SuccessorRef), strings.TrimSpace(item.Reason)); err != nil {
			return DecisionReconciliationApplyOutcome{}, err
		}
	}
	return DecisionReconciliationApplyOutcome{
		Operation:        strings.TrimSpace(item.Operation),
		ReviewedGroupID:  strings.TrimSpace(item.ReviewedGroupID),
		DecisionRefs:     refs,
		SuccessorRef:     strings.TrimSpace(item.SuccessorRef),
		LineageRelations: decisionReconciliationLineageRelations(strings.TrimSpace(item.Operation), refs, strings.TrimSpace(item.SuccessorRef)),
		Status:           "applied",
	}, nil
}

func applyDecisionReconciliationRetire(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	item DecisionReconciliationSelection,
) (DecisionReconciliationApplyOutcome, error) {
	refs := compactSortedStrings(item.DecisionRefs)
	for _, ref := range refs {
		if _, err := DeprecateArtifact(ctx, store, haftDir, ref, strings.TrimSpace(item.Reason)); err != nil {
			return DecisionReconciliationApplyOutcome{}, err
		}
	}
	return DecisionReconciliationApplyOutcome{
		Operation:        DecisionReconciliationOperationRetireWithoutSuccessor,
		ReviewedGroupID:  strings.TrimSpace(item.ReviewedGroupID),
		DecisionRefs:     refs,
		LineageRelations: decisionReconciliationLineageRelations(DecisionReconciliationOperationRetireWithoutSuccessor, refs, ""),
		Status:           "applied",
	}, nil
}

func applyDecisionReconciliationReopen(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	item DecisionReconciliationSelection,
) (DecisionReconciliationApplyOutcome, error) {
	refs := compactSortedStrings(item.DecisionRefs)
	_, problem, err := ReopenDecision(ctx, store, haftDir, refs[0], strings.TrimSpace(item.Reason))
	if err != nil {
		return DecisionReconciliationApplyOutcome{}, err
	}
	outcome := DecisionReconciliationApplyOutcome{
		Operation:       DecisionReconciliationOperationReopen,
		ReviewedGroupID: strings.TrimSpace(item.ReviewedGroupID),
		DecisionRefs:    refs,
		Status:          "applied",
	}
	if problem != nil {
		outcome.ProblemRefs = []string{problem.Meta.ID}
	}
	return outcome, nil
}

func applyDecisionReconciliationEnrichScope(
	ctx context.Context,
	store ArtifactStore,
	item DecisionReconciliationSelection,
) (DecisionReconciliationApplyOutcome, error) {
	refs := compactSortedStrings(item.DecisionRefs)
	decision, err := store.Get(ctx, refs[0])
	if err != nil {
		return DecisionReconciliationApplyOutcome{}, err
	}

	fields := decision.UnmarshalDecisionFields()
	fields.DecisionSubjectRef = strings.TrimSpace(item.DecisionSubjectRef)
	fields.GovernanceTargets = mergeGovernanceTargets(fields.GovernanceTargets, item.GovernanceTargets)
	fields.DriftWatchTargets = mergeDriftWatchTargets(fields.DriftWatchTargets, item.DriftWatchTargets)
	fields.Claims = applyScopeEnrichmentClaimRefs(fields.Claims, item.ClaimGovernanceTargetRefs)

	if err := persistDecisionFields(ctx, store, decision, fields); err != nil {
		return DecisionReconciliationApplyOutcome{}, err
	}

	return DecisionReconciliationApplyOutcome{
		Operation:       DecisionReconciliationOperationEnrichScope,
		ReviewedGroupID: strings.TrimSpace(item.ReviewedGroupID),
		DecisionRefs:    refs,
		UpdatedFields:   decisionScopeEnrichmentUpdatedFields(item),
		Status:          "applied",
	}, nil
}

func applyDecisionReconciliationClaimLifecycleUpdate(
	ctx context.Context,
	store ArtifactStore,
	item DecisionReconciliationSelection,
) (DecisionReconciliationApplyOutcome, error) {
	refs := compactSortedStrings(item.DecisionRefs)
	updatesByDecision := decisionReconciliationClaimUpdatesByDecision(item.ClaimLifecycleUpdates)
	appliedUpdates := make([]DecisionReconciliationClaimLifecycleUpdate, 0, len(item.ClaimLifecycleUpdates))

	for _, ref := range refs {
		updates := updatesByDecision[ref]
		if len(updates) == 0 {
			continue
		}
		decision, err := store.Get(ctx, ref)
		if err != nil {
			return DecisionReconciliationApplyOutcome{}, err
		}
		fields := decision.UnmarshalDecisionFields()
		fields.Claims = applyDecisionReconciliationClaimUpdates(fields.Claims, updates)
		if err := persistDecisionFields(ctx, store, decision, fields); err != nil {
			return DecisionReconciliationApplyOutcome{}, err
		}
		appliedUpdates = append(appliedUpdates, normalizeDecisionReconciliationClaimUpdates(updates)...)
	}

	return DecisionReconciliationApplyOutcome{
		Operation:       DecisionReconciliationOperationClaimLifecycleUpdate,
		ReviewedGroupID: strings.TrimSpace(item.ReviewedGroupID),
		DecisionRefs:    refs,
		UpdatedFields: []string{
			"claims[].lifecycle_status",
			"claims[].successor_ref",
			"claims[].retired_reason",
		},
		ClaimUpdates: appliedUpdates,
		Status:       "applied",
	}, nil
}

func decisionReconciliationClaimUpdatesByDecision(
	updates []DecisionReconciliationClaimLifecycleUpdate,
) map[string][]DecisionReconciliationClaimLifecycleUpdate {
	out := make(map[string][]DecisionReconciliationClaimLifecycleUpdate, len(updates))
	for _, update := range updates {
		decisionRef := strings.TrimSpace(update.DecisionRef)
		if decisionRef == "" {
			continue
		}
		out[decisionRef] = append(out[decisionRef], update)
	}
	return out
}

func applyDecisionReconciliationClaimUpdates(
	claims []DecisionClaim,
	updates []DecisionReconciliationClaimLifecycleUpdate,
) []DecisionClaim {
	updatesByClaim := make(map[string]DecisionReconciliationClaimLifecycleUpdate, len(updates))
	for _, update := range updates {
		updatesByClaim[strings.TrimSpace(update.ClaimID)] = update
	}
	out := make([]DecisionClaim, 0, len(claims))
	for _, claim := range claims {
		update, ok := updatesByClaim[claim.ID]
		if !ok {
			out = append(out, claim)
			continue
		}
		claim.LifecycleStatus = normalizeClaimLifecycleStatus(update.LifecycleStatus)
		claim.SuccessorRef = strings.TrimSpace(update.SuccessorRef)
		claim.RetiredReason = strings.TrimSpace(update.Reason)
		out = append(out, claim)
	}
	return out
}

func normalizeDecisionReconciliationClaimUpdates(
	updates []DecisionReconciliationClaimLifecycleUpdate,
) []DecisionReconciliationClaimLifecycleUpdate {
	out := make([]DecisionReconciliationClaimLifecycleUpdate, 0, len(updates))
	for _, update := range updates {
		out = append(out, DecisionReconciliationClaimLifecycleUpdate{
			DecisionRef:     strings.TrimSpace(update.DecisionRef),
			ClaimID:         strings.TrimSpace(update.ClaimID),
			LifecycleStatus: normalizeClaimLifecycleStatus(update.LifecycleStatus),
			SuccessorRef:    strings.TrimSpace(update.SuccessorRef),
			Reason:          strings.TrimSpace(update.Reason),
		})
	}
	return out
}

func isDecisionReconciliationApplyOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case DecisionReconciliationOperationSupersede,
		DecisionReconciliationOperationMergeThroughSuccessor,
		DecisionReconciliationOperationRetireWithoutSuccessor,
		DecisionReconciliationOperationReopen,
		DecisionReconciliationOperationEnrichScope,
		DecisionReconciliationOperationClaimLifecycleUpdate:
		return true
	default:
		return false
	}
}

func BuildDecisionReconciliationPlan(
	ctx context.Context,
	store ArtifactStore,
) (DecisionReconciliationPlan, error) {
	artifacts, err := store.ListByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		return DecisionReconciliationPlan{}, err
	}

	items := make([]DecisionReconciliationItem, 0, len(artifacts))
	for _, candidate := range artifacts {
		if candidate == nil {
			continue
		}
		if !reconciliationStatusInScope(candidate.Meta.Status) {
			continue
		}
		decision, err := store.Get(ctx, candidate.Meta.ID)
		if err != nil {
			return DecisionReconciliationPlan{}, err
		}
		item, err := buildDecisionReconciliationItem(ctx, store, decision)
		if err != nil {
			return DecisionReconciliationPlan{}, err
		}
		items = append(items, item)
	}

	return BuildDecisionReconciliationPlanFromItems(items), nil
}

func BuildDecisionReconciliationPlanFromItems(
	items []DecisionReconciliationItem,
) DecisionReconciliationPlan {
	normalized := normalizeDecisionReconciliationItems(items)
	conflictGroups, conflictIDs := decisionReconciliationConflictGroups(normalized)
	regularGroups := decisionReconciliationRegularGroups(normalized, conflictIDs)

	groups := append([]DecisionReconciliationGroup{}, conflictGroups...)
	groups = append(groups, regularGroups...)
	sort.SliceStable(groups, func(i, j int) bool {
		return decisionReconciliationGroupLess(groups[i], groups[j])
	})

	summary := decisionReconciliationSummary(normalized, groups)
	return DecisionReconciliationPlan{
		SchemaVersion:     DecisionReconciliationSchemaVersion,
		Authority:         DecisionReconciliationAuthority,
		FileOverlapPolicy: "affected_files are implementation-footprint hints; file overlap alone is never merge/supersede evidence",
		Summary:           summary,
		Groups:            groups,
	}
}

func CompactDecisionReconciliationPlan(
	plan DecisionReconciliationPlan,
	groupLimit int,
) DecisionReconciliationPlan {
	compact := plan
	compact.View = "compact"
	compact.FullAuditCommand = `haft_query(action="decision_reconcile", full=true)`

	groups := plan.Groups
	if groupLimit > 0 && len(groups) > groupLimit {
		compact.OmittedGroups = len(groups) - groupLimit
		groups = groups[:groupLimit]
	}
	compact.CompactGroups = decisionReconciliationCompactGroups(groups)
	compact.Groups = nil

	return compact
}

func decisionReconciliationCompactGroups(
	groups []DecisionReconciliationGroup,
) []DecisionReconciliationCompactGroup {
	out := make([]DecisionReconciliationCompactGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, decisionReconciliationCompactGroup(group))
	}
	return out
}

func decisionReconciliationCompactGroup(
	group DecisionReconciliationGroup,
) DecisionReconciliationCompactGroup {
	return DecisionReconciliationCompactGroup{
		GroupID:                     group.GroupID,
		Category:                    group.Category,
		SubjectRef:                  group.SubjectRef,
		SubjectResolution:           group.SubjectResolution,
		BoundedContext:              group.BoundedContext,
		DecisionRefs:                append([]string(nil), group.DecisionRefs...),
		Fanout:                      len(group.DecisionRefs),
		OperatorRequired:            group.OperatorRequired,
		PreviewOperation:            group.Preview.Operation,
		ApplyOperation:              group.Preview.ApplyOperation,
		DownstreamDependents:        decisionReconciliationPreviewDependentCount(group.Preview),
		DownstreamMigrationRequired: decisionReconciliationPreviewMigrationRequired(group.Preview),
		SuccessorWorkflowRequired:   decisionReconciliationPreviewSuccessorRequired(group.Preview),
		ScopeRepairHints:            append([]string(nil), group.ScopeRepairHints...),
	}
}

func decisionReconciliationPreviewDependentCount(
	preview DecisionReconciliationPreview,
) int {
	if preview.DownstreamImpact != nil {
		return len(preview.DownstreamImpact.DependentRefs)
	}
	if preview.DownstreamMigration != nil {
		return len(preview.DownstreamMigration.DependentRefs)
	}
	return 0
}

func decisionReconciliationPreviewMigrationRequired(
	preview DecisionReconciliationPreview,
) bool {
	return preview.DownstreamMigration != nil && preview.DownstreamMigration.RequiredBeforeApply
}

func decisionReconciliationPreviewSuccessorRequired(
	preview DecisionReconciliationPreview,
) bool {
	return preview.SuccessorWorkflow != nil && preview.SuccessorWorkflow.Required
}

func decisionReconciliationDraftItems(
	groups []DecisionReconciliationGroup,
) []DecisionReconciliationDraftItem {
	out := []DecisionReconciliationDraftItem{}
	for _, group := range groups {
		if group.Preview.ApplyOperation != DecisionReconciliationOperationEnrichScope {
			continue
		}
		for _, item := range group.Decisions {
			out = append(out, decisionReconciliationDraftItem(group, item))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftScore := decisionReconciliationDraftReviewabilityScore(out[i])
		rightScore := decisionReconciliationDraftReviewabilityScore(out[j])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if out[i].ReviewedGroupID != out[j].ReviewedGroupID {
			return out[i].ReviewedGroupID < out[j].ReviewedGroupID
		}
		return out[i].DecisionRef < out[j].DecisionRef
	})
	return out
}

func decisionReconciliationDraftReviewabilityScore(item DecisionReconciliationDraftItem) int {
	switch item.Confidence {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func filterDecisionReconciliationDraftItems(
	items []DecisionReconciliationDraftItem,
	filter DecisionReconciliationSelectionDraftFilter,
) []DecisionReconciliationDraftItem {
	groupID := strings.TrimSpace(filter.GroupID)
	decisionRef := strings.TrimSpace(filter.DecisionRef)
	filtered := make([]DecisionReconciliationDraftItem, 0, len(items))
	for _, item := range items {
		if groupID != "" && item.ReviewedGroupID != groupID {
			continue
		}
		if decisionRef != "" && item.DecisionRef != decisionRef {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func limitDecisionReconciliationDraftItems(
	items []DecisionReconciliationDraftItem,
	filter DecisionReconciliationSelectionDraftFilter,
) []DecisionReconciliationDraftItem {
	if filter.Full || filter.Limit <= 0 || len(items) <= filter.Limit {
		return append([]DecisionReconciliationDraftItem(nil), items...)
	}
	return append([]DecisionReconciliationDraftItem(nil), items[:filter.Limit]...)
}

func decisionReconciliationDraftItem(
	group DecisionReconciliationGroup,
	item DecisionReconciliationItem,
) DecisionReconciliationDraftItem {
	posture := decisionReconciliationDraftCandidatePosture(item)
	return DecisionReconciliationDraftItem{
		Operation:                     DecisionReconciliationOperationEnrichScope,
		ReviewedGroupID:               group.GroupID,
		DecisionRef:                   item.DecisionID,
		DecisionTitle:                 item.DecisionTitle,
		DecisionCarrierHint:           decisionReconciliationDraftCarrierHint(item.DecisionID),
		CandidatePosture:              posture,
		Confidence:                    decisionReconciliationDraftConfidence(posture),
		CurrentSubjectRef:             item.DecisionSubjectRef,
		DecisionSubjectRefSuggestions: decisionReconciliationDraftSubjectSuggestions(item),
		CurrentGovernanceTargets:      compactSortedStrings(item.GovernanceTargets),
		WholeFileFallbackTargets:      compactSortedStrings(item.WholeFileFallbackTargets),
		AffectedFiles:                 compactSortedStrings(item.AffectedFiles),
		ScopeRepairHint:               item.ScopeRepairHint,
		SuggestedReviewAction:         decisionReconciliationDraftReviewAction(posture),
		BlockingQuestions:             decisionReconciliationDraftBlockingQuestions(item),
		RequiredSelectionFields:       decisionReconciliationPreviewRequiredFields(DecisionReconciliationOperationEnrichScope),
		ReviewCommands:                decisionReconciliationDraftReviewCommands(item.DecisionID),
		SelectionTemplate:             decisionReconciliationDraftSelectionTemplate(group, item),
		ReviewNotes: []string{
			"file overlap is implementation footprint only; select exact governance targets before approval",
			"use symbol/api_contract/invariant/spec_section targets when possible; whole-file fallback only when no better target exists",
			"leave the candidate out if subject or target cannot be stated with high confidence",
			"decision_carrier_hint and review_commands are discovery aids, not authority or apply approval",
		},
	}
}

func decisionReconciliationDraftCarrierHint(decisionRef string) string {
	ref := strings.TrimSpace(decisionRef)
	if ref == "" {
		return ""
	}
	return ".haft/" + KindDecisionRecord.Dir() + "/" + ref + ".md"
}

func decisionReconciliationDraftReviewCommands(decisionRef string) []string {
	ref := strings.TrimSpace(decisionRef)
	if ref == "" {
		return nil
	}
	carrier := decisionReconciliationDraftCarrierHint(ref)
	return []string{
		"sed -n '1,220p' " + carrier,
		"haft decision reconcile selection-draft --decision-ref " + ref + " --json",
	}
}

func decisionReconciliationDraftSubjectSuggestions(
	item DecisionReconciliationItem,
) []string {
	suggestions := []string{}
	if subject := strings.TrimSpace(item.DecisionSubjectRef); subject != "" {
		suggestions = appendMissingString(suggestions, subject)
	}
	titleSlug := sanitizeIDSlug(item.DecisionTitle)
	contextSlug := sanitizeIDSlug(item.BoundedContext)
	if titleSlug != "" && contextSlug != "" {
		suggestions = appendMissingString(suggestions, "subject:"+contextSlug+":"+titleSlug)
	}
	if titleSlug != "" {
		suggestions = appendMissingString(suggestions, "subject:"+titleSlug)
	}
	return suggestions
}

func decisionReconciliationDraftCandidatePosture(item DecisionReconciliationItem) string {
	missingSubject := strings.TrimSpace(item.DecisionSubjectRef) == ""
	missingTargets := len(item.GovernanceTargets) == 0
	hasWholeFileFallback := len(item.WholeFileFallbackTargets) > 0
	switch {
	case missingSubject && missingTargets && hasWholeFileFallback:
		return "needs_subject_and_fallback_target_repair"
	case missingSubject && missingTargets:
		return "needs_subject_and_target_review"
	case missingSubject:
		return "precise_target_prefilled_subject_needed"
	case missingTargets && hasWholeFileFallback:
		return "whole_file_fallback_target_repair_needed"
	case missingTargets:
		return "needs_target_review"
	case hasWholeFileFallback:
		return "mixed_precise_and_fallback_target_repair_needed"
	default:
		return "scope_enrichment_not_needed"
	}
}

func decisionReconciliationDraftConfidence(posture string) string {
	switch posture {
	case "precise_target_prefilled_subject_needed":
		return "medium"
	case "needs_subject_and_fallback_target_repair", "whole_file_fallback_target_repair_needed", "mixed_precise_and_fallback_target_repair_needed":
		return "low"
	case "needs_subject_and_target_review", "needs_target_review":
		return "low"
	default:
		return "not_applicable"
	}
}

func decisionReconciliationDraftReviewAction(posture string) string {
	switch posture {
	case "precise_target_prefilled_subject_needed":
		return "review decision carrier and fill exact decision_subject_ref; keep prefilled target only if it is governance scope"
	case "needs_subject_and_fallback_target_repair", "whole_file_fallback_target_repair_needed", "mixed_precise_and_fallback_target_repair_needed":
		return "replace whole-file fallback with symbol/api_contract/invariant/spec_section target before approval"
	case "needs_subject_and_target_review":
		return "recover decision carrier, identify exact subject and target, or leave candidate out"
	case "needs_target_review":
		return "identify exact governance target or leave candidate out"
	default:
		return "no selection needed"
	}
}

func decisionReconciliationDraftBlockingQuestions(item DecisionReconciliationItem) []string {
	out := []string{}
	if strings.TrimSpace(item.DecisionSubjectRef) == "" {
		out = append(out, "What exact object does this decision govern now?")
	}
	if len(item.GovernanceTargets) == 0 {
		out = append(out, "Which symbol, API contract, invariant, or spec section would falsify or preserve this decision?")
	}
	if len(item.WholeFileFallbackTargets) > 0 {
		out = append(out, "Can the whole-file fallback be narrowed to a semantic target?")
	}
	return out
}

func decisionReconciliationDraftSelectionTemplate(
	group DecisionReconciliationGroup,
	item DecisionReconciliationItem,
) string {
	selection := decisionReconciliationDraftSelection(group, item)
	data, err := json.Marshal(selection)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func decisionReconciliationDraftSelection(
	group DecisionReconciliationGroup,
	item DecisionReconciliationItem,
) DecisionReconciliationSelection {
	selection := DecisionReconciliationSelection{
		Operation:          DecisionReconciliationOperationEnrichScope,
		ReviewedGroupID:    group.GroupID,
		DecisionRefs:       []string{item.DecisionID},
		DecisionSubjectRef: "TODO_exact_decision_subject_ref",
		GovernanceTargets:  draftGovernanceTargets(item.GovernanceTargets),
		Reason:             "TODO_operator_reviewed_scope_enrichment_reason",
	}
	if len(selection.GovernanceTargets) == 0 {
		selection.GovernanceTargets = []GovernanceTarget{{
			Kind: "TODO_target_kind",
			Ref:  "TODO_exact_target_ref",
		}}
		selection.DriftWatchTargets = []DriftWatchTarget{{
			TargetRef: "TODO_exact_target_ref",
			Trigger:   "TODO_trigger",
		}}
	}
	return selection
}

func decisionReconciliationDraftSelectionDocumentTemplate(
	items []DecisionReconciliationDraftItem,
) *DecisionReconciliationSelectionDocument {
	if len(items) == 0 {
		return nil
	}
	selections := make([]DecisionReconciliationSelection, 0, len(items))
	for _, item := range items {
		var selection DecisionReconciliationSelection
		if err := json.Unmarshal([]byte(item.SelectionTemplate), &selection); err != nil {
			continue
		}
		selections = append(selections, selection)
	}
	if len(selections) == 0 {
		return nil
	}
	return &DecisionReconciliationSelectionDocument{
		SchemaVersion:       DecisionReconciliationSchemaVersion,
		Authority:           DecisionReconciliationSelectionApplyAuthority,
		OperatorApprovalRef: "",
		Items:               selections,
	}
}

func draftGovernanceTargets(refs []string) []GovernanceTarget {
	out := make([]GovernanceTarget, 0, len(refs))
	for _, ref := range compactSortedStrings(refs) {
		out = append(out, GovernanceTarget{
			Kind: draftGovernanceTargetKind(ref),
			Ref:  ref,
		})
	}
	return out
}

func draftGovernanceTargetKind(ref string) string {
	kind, _, ok := strings.Cut(strings.TrimSpace(ref), ":")
	if ok && strings.TrimSpace(kind) != "" {
		return strings.TrimSpace(kind)
	}
	return "target_ref"
}

func buildDecisionReconciliationItem(
	ctx context.Context,
	store ArtifactStore,
	decision *Artifact,
) (DecisionReconciliationItem, error) {
	fields := decision.UnmarshalDecisionFields()
	files, err := store.GetAffectedFiles(ctx, decision.Meta.ID)
	if err != nil {
		return DecisionReconciliationItem{}, err
	}
	links, err := store.GetLinks(ctx, decision.Meta.ID)
	if err != nil {
		return DecisionReconciliationItem{}, err
	}
	backlinks, err := store.GetBacklinks(ctx, decision.Meta.ID)
	if err != nil {
		return DecisionReconciliationItem{}, err
	}

	item := DecisionReconciliationItem{
		DecisionID:                decision.Meta.ID,
		DecisionTitle:             decision.Meta.Title,
		Status:                    decision.Meta.Status,
		BoundedContext:            strings.TrimSpace(decision.Meta.Context),
		DecisionSubjectRef:        strings.TrimSpace(fields.DecisionSubjectRef),
		DecisionSubjectResolution: decisionSubjectResolution(fields.DecisionSubjectRef),
		GovernanceTargets:         decisionReconciliationGovernanceTargetRefs(fields),
		WholeFileFallbackTargets:  decisionReconciliationWholeFileTargets(fields.EffectiveDriftBindingTargets()),
		AffectedFiles:             reconciliationAffectedFilePaths(files),
		Links:                     normalizeLinks(links),
		Backlinks:                 normalizeLinks(backlinks),
		ClaimLifecycle:            buildClaimLifecycleSummary(fields.Claims),
	}
	item.ScopeRepairHint = decisionReconciliationScopeRepairHint(item)
	return item, nil
}

func reconciliationStatusInScope(status Status) bool {
	return status == StatusActive || status == StatusRefreshDue
}

func normalizeDecisionReconciliationItems(
	items []DecisionReconciliationItem,
) []DecisionReconciliationItem {
	out := make([]DecisionReconciliationItem, 0, len(items))
	for _, item := range items {
		normalized := item
		normalized.DecisionID = strings.TrimSpace(item.DecisionID)
		if normalized.DecisionID == "" {
			continue
		}
		normalized.DecisionTitle = strings.TrimSpace(item.DecisionTitle)
		normalized.BoundedContext = strings.TrimSpace(item.BoundedContext)
		normalized.DecisionSubjectRef = strings.TrimSpace(item.DecisionSubjectRef)
		normalized.DecisionSubjectResolution = decisionSubjectResolution(normalized.DecisionSubjectRef)
		normalized.GovernanceTargets = compactSortedStrings(item.GovernanceTargets)
		normalized.WholeFileFallbackTargets = compactSortedStrings(item.WholeFileFallbackTargets)
		normalized.ScopeRepairHint = decisionReconciliationScopeRepairHint(normalized)
		normalized.AffectedFiles = compactSortedStrings(item.AffectedFiles)
		normalized.Links = normalizeLinks(item.Links)
		normalized.Backlinks = normalizeLinks(item.Backlinks)
		if item.ClaimLifecycle != nil {
			claimLifecycle := *item.ClaimLifecycle
			claimLifecycle.GovernanceTargetRefs = normalizeClaimRefs(claimLifecycle.GovernanceTargetRefs)
			normalized.ClaimLifecycle = &claimLifecycle
		}
		out = append(out, normalized)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DecisionID < out[j].DecisionID
	})
	return out
}

func decisionReconciliationConflictGroups(
	items []DecisionReconciliationItem,
) ([]DecisionReconciliationGroup, map[string]struct{}) {
	byID := map[string]DecisionReconciliationItem{}
	for _, item := range items {
		byID[item.DecisionID] = item
	}

	pairs := map[string][]DecisionReconciliationItem{}
	for _, item := range items {
		for _, link := range append(item.Links, item.Backlinks...) {
			if link.Type != "contradicts" {
				continue
			}
			other, ok := byID[link.Ref]
			if !ok {
				continue
			}
			key := orderedPairKey(item.DecisionID, other.DecisionID)
			pairs[key] = []DecisionReconciliationItem{item, other}
		}
	}

	groups := make([]DecisionReconciliationGroup, 0, len(pairs))
	conflictIDs := map[string]struct{}{}
	for key, pair := range pairs {
		items := normalizeDecisionReconciliationItems(pair)
		for _, item := range items {
			conflictIDs[item.DecisionID] = struct{}{}
		}
		group := decisionReconciliationGroup(
			DecisionReconciliationConflictRequiresOperator,
			"conflict:"+key,
			items,
			[]string{"explicit contradicts link between current decisions"},
		)
		groups = append(groups, group)
	}
	return groups, conflictIDs
}

func decisionReconciliationRegularGroups(
	items []DecisionReconciliationItem,
	exclude map[string]struct{},
) []DecisionReconciliationGroup {
	buckets := map[string]*decisionReconciliationBucket{}
	for _, item := range items {
		if _, ok := exclude[item.DecisionID]; ok {
			continue
		}
		key := decisionReconciliationBucketKey(item)
		bucket := buckets[key]
		if bucket == nil {
			bucket = &decisionReconciliationBucket{
				key:               key,
				subjectRef:        decisionReconciliationSubject(item),
				subjectResolution: item.DecisionSubjectResolution,
				boundedContext:    item.BoundedContext,
				targets:           map[string]struct{}{},
				files:             map[string]struct{}{},
			}
			buckets[key] = bucket
		}
		for _, target := range item.GovernanceTargets {
			bucket.targets[target] = struct{}{}
		}
		for _, file := range item.AffectedFiles {
			bucket.files[file] = struct{}{}
		}
		bucket.items = append(bucket.items, item)
	}

	groups := make([]DecisionReconciliationGroup, 0, len(buckets))
	for _, bucket := range buckets {
		items := normalizeDecisionReconciliationItems(bucket.items)
		category := decisionReconciliationCategory(items)
		basis := decisionReconciliationBasis(category, items)
		groups = append(groups, decisionReconciliationGroup(category, bucket.key, items, basis))
	}
	return groups
}

func decisionReconciliationGroup(
	category string,
	key string,
	items []DecisionReconciliationItem,
	basis []string,
) DecisionReconciliationGroup {
	subject := decisionReconciliationSubject(items[0])
	targets := decisionReconciliationGroupTargets(items)
	files := decisionReconciliationGroupAffectedFiles(items)
	refs := make([]string, 0, len(items))
	for _, item := range items {
		refs = append(refs, item.DecisionID)
	}
	group := DecisionReconciliationGroup{
		GroupID:           decisionReconciliationGroupID(category + "|" + key),
		Category:          category,
		SubjectRef:        subject,
		SubjectResolution: items[0].DecisionSubjectResolution,
		BoundedContext:    items[0].BoundedContext,
		GovernanceTargets: targets,
		AffectedFiles:     files,
		DecisionRefs:      refs,
		Decisions:         items,
		Basis:             basis,
		ScopeRepairHints:  decisionReconciliationGroupRepairHints(items),
		Confidence:        decisionReconciliationConfidence(category, items),
		OperatorRequired:  decisionReconciliationOperatorRequired(category),
	}
	group.Preview = decisionReconciliationPreview(group, items)
	return group
}

func decisionReconciliationBucketKey(item DecisionReconciliationItem) string {
	subject := decisionReconciliationSubject(item)
	targetKey := "no-explicit-governance-target:" + item.DecisionID
	if len(item.GovernanceTargets) > 0 && item.DecisionSubjectRef != "" {
		targetKey = strings.Join(item.GovernanceTargets, ",")
	}
	return strings.Join([]string{
		subject,
		item.BoundedContext,
		targetKey,
	}, "|")
}

func decisionReconciliationSubject(item DecisionReconciliationItem) string {
	if item.DecisionSubjectRef != "" {
		return item.DecisionSubjectRef
	}
	return "decision:" + item.DecisionID
}

func decisionSubjectResolution(subjectRef string) string {
	if strings.TrimSpace(subjectRef) == "" {
		return "missing_explicit_subject_unique_decision_scope"
	}
	return "explicit_decision_subject_ref"
}

func decisionReconciliationCategory(items []DecisionReconciliationItem) string {
	if len(items) > 1 && decisionReconciliationMergeReady(items) {
		return DecisionReconciliationMergeCandidate
	}
	groupIDs := map[string]struct{}{}
	for _, item := range items {
		groupIDs[item.DecisionID] = struct{}{}
	}
	for _, item := range items {
		if decisionReconciliationHasSupersedesLink(item, groupIDs) {
			return DecisionReconciliationSupersedeCandidate
		}
	}
	for _, item := range items {
		if item.Status == StatusRefreshDue {
			return DecisionReconciliationReopenCandidate
		}
	}
	return DecisionReconciliationKeep
}

func decisionReconciliationMergeReady(items []DecisionReconciliationItem) bool {
	if len(items) < 2 {
		return false
	}
	if items[0].DecisionSubjectRef == "" || len(items[0].GovernanceTargets) == 0 {
		return false
	}
	for _, item := range items[1:] {
		if item.DecisionSubjectRef != items[0].DecisionSubjectRef {
			return false
		}
		if item.BoundedContext != items[0].BoundedContext {
			return false
		}
		if len(intersectStrings(item.GovernanceTargets, items[0].GovernanceTargets)) == 0 {
			return false
		}
	}
	return true
}

func decisionReconciliationBasis(category string, items []DecisionReconciliationItem) []string {
	switch category {
	case DecisionReconciliationMergeCandidate:
		return []string{"same explicit decision_subject_ref", "same bounded context", "overlapping explicit governance targets"}
	case DecisionReconciliationSupersedeCandidate:
		return []string{"active/refresh_due decision has explicit supersedes lineage but remains in current review scope"}
	case DecisionReconciliationReopenCandidate:
		return []string{"decision is refresh_due and needs semantic review before authority is extended"}
	case DecisionReconciliationKeep:
		if len(items) == 1 && len(items[0].GovernanceTargets) == 0 {
			return []string{"no explicit governance-target overlap; affected_files are footprint hints only"}
		}
		return []string{"no conflict, no refresh_due status, no merge-ready overlap"}
	default:
		return []string{"report-only classification"}
	}
}

func decisionReconciliationConfidence(category string, items []DecisionReconciliationItem) string {
	switch category {
	case DecisionReconciliationMergeCandidate:
		return "medium_requires_operator_review"
	case DecisionReconciliationConflictRequiresOperator:
		return "high_conflict_link_present"
	case DecisionReconciliationKeep:
		if len(items) == 1 && items[0].DecisionSubjectRef == "" {
			return "high_no_explicit_subject_for_merge"
		}
		return "medium"
	default:
		return "low_report_only_candidate"
	}
}

func decisionReconciliationOperatorRequired(category string) bool {
	return category != DecisionReconciliationKeep
}

func decisionReconciliationPreview(
	group DecisionReconciliationGroup,
	items []DecisionReconciliationItem,
) DecisionReconciliationPreview {
	operation := decisionReconciliationPreviewOperation(group.Category, items)
	applyOperation := decisionReconciliationPreviewApplyOperation(operation)
	current := decisionReconciliationPreviewCurrent(group, items)
	proposed := decisionReconciliationPreviewProposed(operation, group, items)
	downstreamImpact := decisionReconciliationPreviewDownstreamImpact(items)

	return DecisionReconciliationPreview{
		Authority:               "report_only_preview_not_binding_authority",
		ReadOnly:                true,
		Operation:               operation,
		ApplyOperation:          applyOperation,
		Current:                 current,
		Proposed:                proposed,
		RequiredSelectionFields: decisionReconciliationPreviewRequiredFields(operation),
		ValidationNotes:         decisionReconciliationPreviewValidationNotes(operation, group, items),
		DownstreamImpact:        downstreamImpact,
		DownstreamMigration:     decisionReconciliationPreviewMigration(operation, downstreamImpact),
		SuccessorWorkflow:       decisionReconciliationPreviewSuccessorWorkflow(operation),
		DownstreamReview:        decisionReconciliationPreviewDownstreamReview(operation),
		MutationBoundary:        decisionReconciliationPreviewMutationBoundary(operation),
		ApprovalCue:             decisionReconciliationPreviewApprovalCue(operation),
	}
}

func decisionReconciliationPreviewOperation(
	category string,
	items []DecisionReconciliationItem,
) string {
	switch category {
	case DecisionReconciliationMergeCandidate:
		return DecisionReconciliationOperationMergeThroughSuccessor
	case DecisionReconciliationSupersedeCandidate:
		return DecisionReconciliationOperationSupersede
	case DecisionReconciliationReopenCandidate:
		return DecisionReconciliationOperationReopen
	case DecisionReconciliationRetireWithoutSuccessor:
		return DecisionReconciliationOperationRetireWithoutSuccessor
	case DecisionReconciliationConflictRequiresOperator:
		return "operator_judgment_required"
	case DecisionReconciliationKeep:
		if decisionReconciliationGroupNeedsScopeEnrichment(items) {
			return DecisionReconciliationOperationEnrichScope
		}
		return DecisionReconciliationKeep
	default:
		return "review_only"
	}
}

func decisionReconciliationPreviewApplyOperation(operation string) string {
	if isDecisionReconciliationApplyOperation(operation) {
		return operation
	}
	return ""
}

func decisionReconciliationPreviewCurrent(
	group DecisionReconciliationGroup,
	items []DecisionReconciliationItem,
) DecisionReconciliationPreviewState {
	return DecisionReconciliationPreviewState{
		DecisionRefs:      compactSortedStrings(group.DecisionRefs),
		Statuses:          decisionReconciliationPreviewStatuses(items, ""),
		SubjectRef:        group.SubjectRef,
		BoundedContext:    group.BoundedContext,
		GovernanceTargets: compactSortedStrings(group.GovernanceTargets),
		ScopeRepairHints:  compactSortedStrings(group.ScopeRepairHints),
		LineageRefs:       decisionReconciliationPreviewLineageRefs(items),
	}
}

func decisionReconciliationPreviewProposed(
	operation string,
	group DecisionReconciliationGroup,
	items []DecisionReconciliationItem,
) DecisionReconciliationPreviewState {
	state := DecisionReconciliationPreviewState{
		DecisionRefs:      compactSortedStrings(group.DecisionRefs),
		SubjectRef:        group.SubjectRef,
		BoundedContext:    group.BoundedContext,
		GovernanceTargets: compactSortedStrings(group.GovernanceTargets),
	}
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor:
		state.Statuses = decisionReconciliationPreviewStatuses(items, string(StatusSuperseded))
		state.LineageRelations = decisionReconciliationLineageRelations(operation, group.DecisionRefs, "$successor_ref")
		state.Effects = []string{
			"requires an existing successor DecisionRecord",
			"selected decisions would become superseded by the successor",
			"successor would preserve lineage to old decision IDs",
		}
		state.RequiredSuccessorRef = true
	case DecisionReconciliationOperationSupersede:
		state.Statuses = decisionReconciliationPreviewStatuses(items, string(StatusSuperseded))
		state.LineageRelations = decisionReconciliationLineageRelations(operation, group.DecisionRefs, "$successor_ref")
		state.Effects = []string{
			"requires an existing successor DecisionRecord",
			"selected decisions would become superseded by the successor",
		}
		state.RequiredSuccessorRef = true
	case DecisionReconciliationOperationRetireWithoutSuccessor:
		state.Statuses = decisionReconciliationPreviewStatuses(items, string(StatusDeprecated))
		state.LineageRelations = decisionReconciliationLineageRelations(operation, group.DecisionRefs, "")
		state.Effects = []string{"selected decisions would be deprecated without a successor"}
	case DecisionReconciliationOperationReopen:
		state.Statuses = decisionReconciliationPreviewStatuses(items, string(StatusRefreshDue))
		state.Effects = []string{"creates or reuses a ProblemCard for semantic review"}
		state.RequiresProblemReview = true
	case DecisionReconciliationOperationEnrichScope:
		state.Statuses = decisionReconciliationPreviewStatuses(items, "")
		state.ScopeRepairHints = compactSortedStrings(group.ScopeRepairHints)
		state.Effects = []string{
			"adds explicit decision_subject_ref and governance_targets or drift_watch_targets",
			"may attach claim governance target refs when explicit claims exist",
			"status, lineage, evidence, baselines, and gates remain unchanged",
		}
	default:
		state.Statuses = decisionReconciliationPreviewStatuses(items, "")
		state.Effects = []string{"no mutation proposed by this report"}
	}
	return state
}

func decisionReconciliationLineageRelations(
	operation string,
	decisionRefs []string,
	successorRef string,
) []DecisionReconciliationLineageRelation {
	refs := compactSortedStrings(decisionRefs)
	relations := make([]DecisionReconciliationLineageRelation, 0, len(refs)*2)
	requiresSuccessor := strings.TrimSpace(successorRef) == "$successor_ref"
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor:
		for _, ref := range refs {
			relations = append(relations,
				DecisionReconciliationLineageRelation{
					Relation:             "mergedFrom",
					SourceRef:            successorRef,
					TargetRef:            ref,
					RequiresSuccessorRef: requiresSuccessor,
					Note:                 "successor keeps lineage to merged historical decision",
				},
				DecisionReconciliationLineageRelation{
					Relation:             "retiredWithSuccessor",
					SourceRef:            ref,
					TargetRef:            successorRef,
					RequiresSuccessorRef: requiresSuccessor,
					Note:                 "historical decision authority ends through selected successor",
				},
			)
		}
	case DecisionReconciliationOperationSupersede:
		for _, ref := range refs {
			relations = append(relations,
				DecisionReconciliationLineageRelation{
					Relation:             "supersedes",
					SourceRef:            successorRef,
					TargetRef:            ref,
					RequiresSuccessorRef: requiresSuccessor,
					Note:                 "successor replaces historical decision authority",
				},
				DecisionReconciliationLineageRelation{
					Relation:             "retiredWithSuccessor",
					SourceRef:            ref,
					TargetRef:            successorRef,
					RequiresSuccessorRef: requiresSuccessor,
					Note:                 "historical decision authority ends through selected successor",
				},
			)
		}
	case DecisionReconciliationOperationRetireWithoutSuccessor:
		for _, ref := range refs {
			relations = append(relations, DecisionReconciliationLineageRelation{
				Relation:  "retiredWithoutSuccessor",
				SourceRef: ref,
				Note:      "historical decision authority ends without replacement",
			})
		}
	}
	return relations
}

func decisionReconciliationPreviewStatuses(
	items []DecisionReconciliationItem,
	override string,
) []DecisionReconciliationPreviewStatus {
	out := make([]DecisionReconciliationPreviewStatus, 0, len(items))
	for _, item := range items {
		status := string(item.Status)
		if override != "" {
			status = override
		}
		out = append(out, DecisionReconciliationPreviewStatus{
			DecisionRef: item.DecisionID,
			Status:      status,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DecisionRef < out[j].DecisionRef
	})
	return out
}

func decisionReconciliationPreviewLineageRefs(
	items []DecisionReconciliationItem,
) []string {
	out := []string{}
	for _, item := range items {
		for _, link := range append(item.Links, item.Backlinks...) {
			out = append(out, link.Type+":"+link.Ref)
		}
	}
	return compactSortedStrings(out)
}

func decisionReconciliationPreviewDownstreamImpact(
	items []DecisionReconciliationItem,
) *DecisionReconciliationDownstream {
	groupRefs := map[string]struct{}{}
	for _, item := range items {
		groupRefs[item.DecisionID] = struct{}{}
	}

	impact := DecisionReconciliationDownstream{}
	dependentRefs := []string{}
	for _, item := range items {
		for _, link := range item.Links {
			edge := decisionReconciliationDownstreamEdge("outgoing", item.DecisionID, link, groupRefs)
			impact = appendDecisionReconciliationDownstreamEdge(impact, edge)
			if edge.Scope == "external" {
				dependentRefs = append(dependentRefs, edge.Ref)
			}
		}
		for _, link := range item.Backlinks {
			edge := decisionReconciliationDownstreamEdge("incoming", item.DecisionID, link, groupRefs)
			impact = appendDecisionReconciliationDownstreamEdge(impact, edge)
			if edge.Scope == "external" {
				dependentRefs = append(dependentRefs, edge.Ref)
			}
		}
	}
	if impact.InternalEdges == 0 && impact.ExternalEdges == 0 {
		return nil
	}
	impact.DependentRefs = compactSortedStrings(dependentRefs)
	impact.ReviewCue = decisionReconciliationDownstreamReviewCue(impact)
	sort.SliceStable(impact.Edges, func(i, j int) bool {
		left := impact.Edges[i]
		right := impact.Edges[j]
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		if left.Direction != right.Direction {
			return left.Direction < right.Direction
		}
		if left.DecisionRef != right.DecisionRef {
			return left.DecisionRef < right.DecisionRef
		}
		if left.Ref != right.Ref {
			return left.Ref < right.Ref
		}
		return left.LinkType < right.LinkType
	})
	return &impact
}

func decisionReconciliationPreviewMigration(
	operation string,
	impact *DecisionReconciliationDownstream,
) *DecisionReconciliationMigration {
	if impact == nil || !isDecisionReconciliationApplyOperation(operation) {
		return nil
	}
	report := DecisionReconciliationMigration{
		RequiredBeforeApply: impact.ExternalEdges > 0,
		AutoRelink:          false,
		Policy:              "review_dependents_before_apply_no_auto_relink",
		DependentRefs:       compactSortedStrings(impact.DependentRefs),
		ReviewSteps: []string{
			"inspect every external dependent ref before approving the selection",
			"decide whether each dependent should point to the successor, remain historical, or be reopened separately",
			"record follow-up relinks as explicit operator-approved work; this report does not relink dependencies",
		},
		SelectionImpact: decisionReconciliationMigrationImpact(operation),
	}
	if !report.RequiredBeforeApply {
		report.Policy = "internal_group_edges_only_no_external_relink_required"
		report.ReviewSteps = []string{
			"internal group edges are preserved as lineage context",
			"no external dependent refs require migration before this selection",
		}
	}
	return &report
}

func decisionReconciliationMigrationImpact(operation string) []string {
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor,
		DecisionReconciliationOperationSupersede:
		return []string{
			"downstream dependents may need to move from old decision IDs to the selected successor",
			"old decision IDs remain searchable history after apply",
		}
	case DecisionReconciliationOperationRetireWithoutSuccessor:
		return []string{
			"downstream dependents may need explicit retirement or reopen follow-up because no successor is selected",
			"old decision IDs remain searchable history after apply",
		}
	case DecisionReconciliationOperationReopen:
		return []string{
			"downstream dependents may need review after the reopened problem is resolved",
		}
	case DecisionReconciliationOperationEnrichScope:
		return []string{
			"scope enrichment does not relink dependents but may change future drift/current-authority grouping",
		}
	default:
		return nil
	}
}

func decisionReconciliationPreviewSuccessorWorkflow(
	operation string,
) *DecisionReconciliationSuccessor {
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor,
		DecisionReconciliationOperationSupersede:
		return &DecisionReconciliationSuccessor{
			Required:                     true,
			Authority:                    "review_contract_not_binding_authority",
			BindingPath:                  "create_or_select_successor_decision_then_apply_operator_approved_selection",
			ExistingSuccessorRefRequired: true,
			RequiredPacketFields: []string{
				"decision_subject_ref",
				"bounded_context_ref",
				"merged_from",
				"retained_claims",
				"withdrawn_claims",
				"changed_assumptions",
				"resolved_conflicts",
				"remaining_evidence",
				"governance_scope",
				"drift_watch_targets",
				"valid_until",
			},
			ReviewSteps: []string{
				"confirm the successor DecisionRecord is current and covers the same bounded context",
				"review retained_claims and withdrawn_claims before approving lineage mutation",
				"confirm remaining_evidence and valid_until are sufficient for the intended use",
				"confirm governance_scope and drift_watch_targets are narrower than implementation footprint",
			},
			MutationBoundary: []string{
				"this preview does not create the successor",
				"this preview does not change old decision status",
				"this preview does not relink downstream dependencies",
			},
		}
	default:
		return nil
	}
}

func decisionReconciliationDownstreamEdge(
	direction string,
	decisionRef string,
	link Link,
	groupRefs map[string]struct{},
) DecisionReconciliationDownstreamEdge {
	ref := strings.TrimSpace(link.Ref)
	scope := "external"
	if _, ok := groupRefs[ref]; ok {
		scope = "internal"
	}
	return DecisionReconciliationDownstreamEdge{
		Direction:   direction,
		DecisionRef: strings.TrimSpace(decisionRef),
		LinkType:    strings.TrimSpace(link.Type),
		Ref:         ref,
		Scope:       scope,
	}
}

func appendDecisionReconciliationDownstreamEdge(
	impact DecisionReconciliationDownstream,
	edge DecisionReconciliationDownstreamEdge,
) DecisionReconciliationDownstream {
	if edge.Ref == "" || edge.LinkType == "" {
		return impact
	}
	if edge.Scope == "internal" {
		impact.InternalEdges++
	} else {
		impact.ExternalEdges++
	}
	if edge.Direction == "incoming" {
		impact.IncomingEdges++
	}
	if edge.Direction == "outgoing" {
		impact.OutgoingEdges++
	}
	impact.Edges = append(impact.Edges, edge)
	return impact
}

func decisionReconciliationDownstreamReviewCue(
	impact DecisionReconciliationDownstream,
) string {
	if impact.ExternalEdges > 0 {
		return "external dependent refs require review before apply; this report does not relink downstream artifacts"
	}
	return "only internal group links detected; old IDs remain searchable history"
}

func decisionReconciliationPreviewRequiredFields(operation string) []string {
	common := []string{
		"schema_version",
		"authority=operator_approved_reconciliation_selection",
		"operator_approval_ref",
		"items[].operation",
		"items[].reviewed_group_id",
		"items[].decision_refs",
		"items[].reason",
	}
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor,
		DecisionReconciliationOperationSupersede:
		return append(common, "items[].successor_ref")
	case DecisionReconciliationOperationEnrichScope:
		return append(common,
			"items[].decision_subject_ref",
			"items[].governance_targets_or_drift_watch_targets",
		)
	case DecisionReconciliationOperationRetireWithoutSuccessor,
		DecisionReconciliationOperationReopen:
		return common
	default:
		return nil
	}
}

func decisionReconciliationPreviewValidationNotes(
	operation string,
	group DecisionReconciliationGroup,
	items []DecisionReconciliationItem,
) []string {
	notes := []string{"preview is advisory and cannot authorize apply"}
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor:
		notes = append(notes,
			"apply-ready only after an existing successor_ref is selected",
			"successor DecisionRecord must already exist and remain current",
			"review retained and withdrawn claims before approval",
		)
	case DecisionReconciliationOperationSupersede:
		notes = append(notes,
			"apply-ready only after an existing successor_ref is selected",
			"successor DecisionRecord must already exist and remain current",
		)
	case DecisionReconciliationOperationRetireWithoutSuccessor:
		notes = append(notes, "apply-ready only with explicit no-successor retirement reason")
	case DecisionReconciliationOperationReopen:
		notes = append(notes, "apply-ready for exactly one decision and an explicit review reason")
	case DecisionReconciliationOperationEnrichScope:
		notes = append(notes,
			"apply-ready only after decision_subject_ref and governance_targets or drift_watch_targets are reviewed",
			"scope enrichment cannot retarget an existing explicit decision_subject_ref",
			"scope enrichment must describe governance scope, not implementation footprint",
		)
	case "operator_judgment_required":
		notes = append(notes,
			"not apply-ready: conflict requires operator judgment before choosing a concrete operation",
			"create a successor, reopen, or waive only through a separate approved selection",
		)
	case DecisionReconciliationKeep:
		notes = append(notes, "no apply selection is needed for keep")
	default:
		notes = append(notes, "not apply-ready: no supported apply operation selected")
	}
	if group.Category == DecisionReconciliationKeep && decisionReconciliationGroupNeedsScopeEnrichment(items) {
		notes = append(notes, "keep classification still has scope enrichment hints for optional precision repair")
	}
	if len(group.ScopeRepairHints) > 0 {
		notes = append(notes, "scope repair hints are prompts, not automatic mutations")
	}
	return compactSortedStrings(notes)
}

func decisionReconciliationPreviewDownstreamReview(operation string) []string {
	switch operation {
	case DecisionReconciliationOperationMergeThroughSuccessor,
		DecisionReconciliationOperationSupersede,
		DecisionReconciliationOperationRetireWithoutSuccessor:
		return []string{
			"review downstream links and dependent current-authority groups before apply",
			"old IDs remain searchable history after apply",
		}
	case DecisionReconciliationOperationReopen:
		return []string{"review whether the reopened problem needs a successor decision or evidence refresh"}
	case DecisionReconciliationOperationEnrichScope:
		return []string{"verify proposed subject and targets describe governance scope, not implementation footprint"}
	default:
		return nil
	}
}

func decisionReconciliationPreviewMutationBoundary(operation string) []string {
	boundary := []string{
		"preview generation is read-only",
		"does not create or modify DecisionRecords",
		"does not create evidence, baselines, gates, or admissions",
	}
	if operation == DecisionReconciliationOperationEnrichScope {
		return append(boundary, "apply may update only explicit scope fields after operator approval")
	}
	if isDecisionReconciliationApplyOperation(operation) {
		return append(boundary, "apply requires a separate operator-approved selection document")
	}
	return boundary
}

func decisionReconciliationPreviewApprovalCue(operation string) string {
	switch operation {
	case DecisionReconciliationKeep:
		return "no approval required for keep"
	case "operator_judgment_required":
		return "operator judgment is required before a concrete selection can be produced"
	case "review_only":
		return "review-only classification; no apply operation selected"
	default:
		return "operator approval is required before apply"
	}
}

func decisionReconciliationGroupNeedsScopeEnrichment(
	items []DecisionReconciliationItem,
) bool {
	for _, item := range items {
		if decisionReconciliationNeedsScopeEnrichment(item) {
			return true
		}
	}
	return false
}

func decisionReconciliationGroupRepairHints(items []DecisionReconciliationItem) []string {
	hints := make([]string, 0, len(items))
	for _, item := range items {
		hints = append(hints, item.ScopeRepairHint)
	}
	return compactSortedStrings(hints)
}

func decisionReconciliationNeedsScopeEnrichment(item DecisionReconciliationItem) bool {
	return strings.TrimSpace(item.DecisionSubjectRef) == "" ||
		len(item.GovernanceTargets) == 0 ||
		len(item.WholeFileFallbackTargets) > 0
}

func decisionReconciliationScopeRepairHint(item DecisionReconciliationItem) string {
	missingSubject := strings.TrimSpace(item.DecisionSubjectRef) == ""
	missingTargets := len(item.GovernanceTargets) == 0
	hasWholeFileFallback := len(item.WholeFileFallbackTargets) > 0
	switch {
	case missingSubject && missingTargets && hasWholeFileFallback:
		return "use enrich_scope to add decision_subject_ref and replace whole-file fallback with explicit governance_targets or drift_watch_targets"
	case missingSubject && missingTargets:
		return "use enrich_scope to add decision_subject_ref and explicit governance_targets or drift_watch_targets"
	case missingSubject:
		return "use enrich_scope to add decision_subject_ref"
	case missingTargets && hasWholeFileFallback:
		return "replace whole-file fallback with explicit governance_targets or drift_watch_targets before merge/supersede review"
	case missingTargets:
		return "use enrich_scope to add explicit governance_targets or drift_watch_targets"
	case hasWholeFileFallback:
		return "replace whole-file fallback with symbol, range, module, api_contract, or invariant target"
	default:
		return ""
	}
}

func decisionReconciliationHasSupersedesLink(
	item DecisionReconciliationItem,
	groupIDs map[string]struct{},
) bool {
	for _, link := range item.Links {
		_, targetInGroup := groupIDs[link.Ref]
		if link.Type == "supersedes" && targetInGroup {
			return true
		}
	}
	return false
}

func decisionReconciliationGovernanceTargets(targets []BindingTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.Kind == BindingTargetWholeFileFallback {
			continue
		}
		key := decisionReconciliationBindingTargetKey(target)
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return compactSortedStrings(out)
}

func decisionReconciliationGovernanceTargetRefs(fields DecisionFields) []string {
	watchRefs := decisionReconciliationWatchTargetRefs(fields.DriftWatchTargets)
	if len(watchRefs) > 0 {
		return watchRefs
	}

	targetRefs := decisionReconciliationSemanticGovernanceTargetRefs(fields.GovernanceTargets)
	if len(targetRefs) > 0 {
		return targetRefs
	}

	return decisionReconciliationGovernanceTargets(fields.EffectiveDriftBindingTargets())
}

func decisionReconciliationWatchTargetRefs(targets []DriftWatchTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range normalizeDriftWatchTargets(targets) {
		if target.TargetRef != "" {
			out = append(out, target.TargetRef)
			continue
		}
		if target.BindingTarget == nil {
			continue
		}
		key := decisionReconciliationBindingTargetKey(*target.BindingTarget)
		if key != "" {
			out = append(out, key)
		}
	}
	return compactSortedStrings(out)
}

func decisionReconciliationSemanticGovernanceTargetRefs(targets []GovernanceTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range normalizeGovernanceTargets(targets) {
		if target.Ref != "" {
			out = append(out, target.Ref)
			continue
		}
		if target.BindingTarget == nil {
			continue
		}
		key := decisionReconciliationBindingTargetKey(*target.BindingTarget)
		if key != "" {
			out = append(out, key)
		}
	}
	return compactSortedStrings(out)
}

func decisionReconciliationWholeFileTargets(targets []BindingTarget) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.Kind != BindingTargetWholeFileFallback {
			continue
		}
		key := decisionReconciliationBindingTargetKey(target)
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return compactSortedStrings(out)
}

func decisionReconciliationBindingTargetKey(target BindingTarget) string {
	kind := strings.TrimSpace(target.Kind)
	if kind == "" {
		return ""
	}
	parts := []string{kind}
	switch kind {
	case BindingTargetSymbol:
		parts = append(parts,
			strings.TrimSpace(target.FilePath),
			strings.TrimSpace(target.SymbolKind),
			strings.TrimSpace(target.Receiver),
			strings.TrimSpace(target.SymbolName),
		)
	case BindingTargetRange:
		parts = append(parts,
			strings.TrimSpace(target.FilePath),
			fmt.Sprintf("%d-%d", target.Line, target.EndLine),
		)
	case BindingTargetModule:
		parts = append(parts, strings.TrimSpace(target.ModulePath))
	case BindingTargetSpecSection, BindingTargetAPIContract, BindingTargetInvariant:
		targetRef := strings.TrimSpace(target.TargetRef)
		if strings.HasPrefix(targetRef, kind+":") {
			return targetRef
		}
		parts = append(parts, targetRef)
	default:
		parts = append(parts, strings.TrimSpace(target.FilePath))
	}
	return strings.Join(compactStrings(parts), ":")
}

func mergeGovernanceTargets(
	current []GovernanceTarget,
	next []GovernanceTarget,
) []GovernanceTarget {
	merged := append(normalizeGovernanceTargets(current), normalizeGovernanceTargets(next)...)
	return uniqueGovernanceTargets(merged)
}

func mergeDriftWatchTargets(
	current []DriftWatchTarget,
	next []DriftWatchTarget,
) []DriftWatchTarget {
	merged := append(normalizeDriftWatchTargets(current), normalizeDriftWatchTargets(next)...)
	return uniqueDriftWatchTargets(merged)
}

func uniqueGovernanceTargets(targets []GovernanceTarget) []GovernanceTarget {
	out := make([]GovernanceTarget, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range normalizeGovernanceTargets(targets) {
		key := stableJSONKey(target)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out
}

func uniqueDriftWatchTargets(targets []DriftWatchTarget) []DriftWatchTarget {
	out := make([]DriftWatchTarget, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range normalizeDriftWatchTargets(targets) {
		key := stableJSONKey(target)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out
}

func stableJSONKey(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func normalizeScopeEnrichmentClaimRefs(values map[string][]string) map[string][]string {
	out := map[string][]string{}
	for claimID, refs := range values {
		cleanClaimID := strings.TrimSpace(claimID)
		if cleanClaimID == "" {
			continue
		}
		out[cleanClaimID] = normalizeClaimRefs(refs)
	}
	return out
}

func applyScopeEnrichmentClaimRefs(
	claims []DecisionClaim,
	values map[string][]string,
) []DecisionClaim {
	normalizedClaims := normalizeDecisionClaims(claims)
	refsByClaim := normalizeScopeEnrichmentClaimRefs(values)
	if len(refsByClaim) == 0 {
		return normalizedClaims
	}

	out := make([]DecisionClaim, 0, len(normalizedClaims))
	for _, claim := range normalizedClaims {
		targetRefs := refsByClaim[claim.ID]
		if len(targetRefs) > 0 {
			mergedRefs := append(claim.GovernanceTargetRefs, targetRefs...)
			claim.GovernanceTargetRefs = normalizeClaimRefs(mergedRefs)
		}
		out = append(out, claim)
	}
	return out
}

func decisionScopeEnrichmentUpdatedFields(item DecisionReconciliationSelection) []string {
	fields := []string{"decision_subject_ref"}
	if len(normalizeGovernanceTargets(item.GovernanceTargets)) > 0 {
		fields = append(fields, "governance_targets")
	}
	if len(normalizeDriftWatchTargets(item.DriftWatchTargets)) > 0 {
		fields = append(fields, "drift_watch_targets")
	}
	if len(normalizeScopeEnrichmentClaimRefs(item.ClaimGovernanceTargetRefs)) > 0 {
		fields = append(fields, "claim_governance_target_refs")
	}
	return compactSortedStrings(fields)
}

func reconciliationAffectedFilePaths(files []AffectedFile) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.Path)
	}
	return compactSortedStrings(out)
}

func decisionReconciliationGroupTargets(items []DecisionReconciliationItem) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.GovernanceTargets...)
	}
	return compactSortedStrings(out)
}

func decisionReconciliationGroupAffectedFiles(items []DecisionReconciliationItem) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.AffectedFiles...)
	}
	return compactSortedStrings(out)
}

func decisionReconciliationSummary(
	items []DecisionReconciliationItem,
	groups []DecisionReconciliationGroup,
) DecisionReconciliationSummary {
	summary := DecisionReconciliationSummary{
		ReviewedDecisions: len(items),
		Groups:            len(groups),
	}
	for _, item := range items {
		if item.DecisionSubjectRef == "" {
			summary.MissingExplicitSubject++
		}
		if len(item.GovernanceTargets) == 0 && len(item.WholeFileFallbackTargets) > 0 {
			summary.WholeFileFallbackOnly++
		}
		if decisionReconciliationNeedsScopeEnrichment(item) {
			summary.ScopeEnrichmentCandidates++
		}
	}
	for _, group := range groups {
		switch group.Category {
		case DecisionReconciliationKeep:
			summary.Keep++
		case DecisionReconciliationReopenCandidate:
			summary.ReopenCandidates++
		case DecisionReconciliationMergeCandidate:
			summary.MergeCandidates++
		case DecisionReconciliationSupersedeCandidate:
			summary.SupersedeCandidates++
		case DecisionReconciliationRetireWithoutSuccessor:
			summary.RetireWithoutSuccessorCandidates++
		case DecisionReconciliationConflictRequiresOperator:
			summary.ConflictRequiresOperator++
		}
	}
	return summary
}

func normalizeLinks(links []Link) []Link {
	out := make([]Link, 0, len(links))
	for _, link := range links {
		ref := strings.TrimSpace(link.Ref)
		linkType := strings.TrimSpace(link.Type)
		if ref == "" || linkType == "" {
			continue
		}
		out = append(out, Link{Ref: ref, Type: linkType})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ref != out[j].Ref {
			return out[i].Ref < out[j].Ref
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func compactSortedStrings(values []string) []string {
	out := compactStrings(values)
	sort.Strings(out)
	deduped := out[:0]
	var previous string
	for index, value := range out {
		if index > 0 && value == previous {
			continue
		}
		deduped = append(deduped, value)
		previous = value
	}
	return deduped
}

func intersectStrings(left []string, right []string) []string {
	rightSet := map[string]struct{}{}
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	out := []string{}
	for _, value := range left {
		if _, ok := rightSet[value]; ok {
			out = append(out, value)
		}
	}
	return compactSortedStrings(out)
}

func orderedPairKey(left string, right string) string {
	if left < right {
		return left + "::" + right
	}
	return right + "::" + left
}

func decisionReconciliationGroupID(key string) string {
	sum := sha1.Sum([]byte(key))
	return fmt.Sprintf("decision-reconcile-%x", sum[:6])
}

func decisionReconciliationGroupLess(
	left DecisionReconciliationGroup,
	right DecisionReconciliationGroup,
) bool {
	leftRank := decisionReconciliationCategoryRank(left.Category)
	rightRank := decisionReconciliationCategoryRank(right.Category)
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	if len(left.DecisionRefs) != len(right.DecisionRefs) {
		return len(left.DecisionRefs) > len(right.DecisionRefs)
	}
	if left.SubjectRef != right.SubjectRef {
		return left.SubjectRef < right.SubjectRef
	}
	return left.GroupID < right.GroupID
}

func decisionReconciliationCategoryRank(category string) int {
	switch category {
	case DecisionReconciliationConflictRequiresOperator:
		return 0
	case DecisionReconciliationMergeCandidate:
		return 1
	case DecisionReconciliationSupersedeCandidate:
		return 2
	case DecisionReconciliationReopenCandidate:
		return 3
	case DecisionReconciliationKeep:
		return 4
	default:
		return 9
	}
}
