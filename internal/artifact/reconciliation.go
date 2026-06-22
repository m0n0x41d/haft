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
)

type DecisionReconciliationPlan struct {
	SchemaVersion     int                           `json:"schema_version"`
	Authority         string                        `json:"authority"`
	FileOverlapPolicy string                        `json:"file_overlap_policy"`
	Summary           DecisionReconciliationSummary `json:"summary"`
	Groups            []DecisionReconciliationGroup `json:"groups"`
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

type DecisionReconciliationSelection struct {
	Operation                 string              `json:"operation"`
	ReviewedGroupID           string              `json:"reviewed_group_id"`
	DecisionRefs              []string            `json:"decision_refs"`
	SuccessorRef              string              `json:"successor_ref,omitempty"`
	DecisionSubjectRef        string              `json:"decision_subject_ref,omitempty"`
	GovernanceTargets         []GovernanceTarget  `json:"governance_targets,omitempty"`
	DriftWatchTargets         []DriftWatchTarget  `json:"drift_watch_targets,omitempty"`
	ClaimGovernanceTargetRefs map[string][]string `json:"claim_governance_target_refs,omitempty"`
	Reason                    string              `json:"reason"`
}

type DecisionReconciliationApplyResult struct {
	SchemaVersion int                                  `json:"schema_version"`
	Authority     string                               `json:"authority"`
	Applied       []DecisionReconciliationApplyOutcome `json:"applied"`
}

type DecisionReconciliationApplyOutcome struct {
	Operation        string                                  `json:"operation"`
	ReviewedGroupID  string                                  `json:"reviewed_group_id,omitempty"`
	DecisionRefs     []string                                `json:"decision_refs"`
	SuccessorRef     string                                  `json:"successor_ref,omitempty"`
	ProblemRefs      []string                                `json:"problem_refs,omitempty"`
	UpdatedFields    []string                                `json:"updated_fields,omitempty"`
	LineageRelations []DecisionReconciliationLineageRelation `json:"lineage_relations,omitempty"`
	Status           string                                  `json:"status"`
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
	if strings.TrimSpace(document.Authority) != "operator_approved_reconciliation_selection" {
		return errors.New("authority must be operator_approved_reconciliation_selection")
	}
	if strings.TrimSpace(document.OperatorApprovalRef) == "" {
		return errors.New("operator_approval_ref is required")
	}
	if len(document.Items) == 0 {
		return errors.New("items must contain at least one selection")
	}
	for index, item := range document.Items {
		if err := validateDecisionReconciliationSelection(ctx, store, index, item); err != nil {
			return err
		}
	}
	return nil
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

func validateDecisionReconciliationSelection(
	ctx context.Context,
	store ArtifactStore,
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
	if strings.TrimSpace(item.Reason) == "" {
		return fmt.Errorf("%s.reason is required", prefix)
	}
	refs := compactSortedStrings(item.DecisionRefs)
	if len(refs) == 0 {
		return fmt.Errorf("%s.decision_refs must contain at least one decision", prefix)
	}
	if operation == DecisionReconciliationOperationReopen && len(refs) != 1 {
		return fmt.Errorf("%s.decision_refs must contain exactly one decision for reopen", prefix)
	}
	if operation == DecisionReconciliationOperationEnrichScope && len(refs) != 1 {
		return fmt.Errorf("%s.decision_refs must contain exactly one decision for enrich_scope", prefix)
	}
	if operation == DecisionReconciliationOperationSupersede || operation == DecisionReconciliationOperationMergeThroughSuccessor {
		if strings.TrimSpace(item.SuccessorRef) == "" {
			return fmt.Errorf("%s.successor_ref is required for %s", prefix, operation)
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

	governanceTargets := normalizeGovernanceTargets(item.GovernanceTargets)
	driftWatchTargets := normalizeDriftWatchTargets(item.DriftWatchTargets)
	if len(governanceTargets) == 0 && len(driftWatchTargets) == 0 {
		return fmt.Errorf("%s.governance_targets or drift_watch_targets is required for enrich_scope", prefix)
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
		if _, ok := claimsByID[claimID]; !ok {
			return fmt.Errorf("%s.claim_governance_target_refs[%q] does not match an explicit claim", prefix, claimID)
		}
		if len(targetRefs) == 0 {
			return fmt.Errorf("%s.claim_governance_target_refs[%q] must contain at least one target ref", prefix, claimID)
		}
	}
	return nil
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

func isDecisionReconciliationApplyOperation(operation string) bool {
	switch strings.TrimSpace(operation) {
	case DecisionReconciliationOperationSupersede,
		DecisionReconciliationOperationMergeThroughSuccessor,
		DecisionReconciliationOperationRetireWithoutSuccessor,
		DecisionReconciliationOperationReopen,
		DecisionReconciliationOperationEnrichScope:
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

	return DecisionReconciliationPreview{
		Authority:               "report_only_preview_not_binding_authority",
		ReadOnly:                true,
		Operation:               operation,
		ApplyOperation:          applyOperation,
		Current:                 current,
		Proposed:                proposed,
		RequiredSelectionFields: decisionReconciliationPreviewRequiredFields(operation),
		ValidationNotes:         decisionReconciliationPreviewValidationNotes(operation, group, items),
		DownstreamImpact:        decisionReconciliationPreviewDownstreamImpact(items),
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
