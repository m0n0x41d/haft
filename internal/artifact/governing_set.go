package artifact

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	CurrentGoverningSetSchemaVersion = 1
	CurrentGoverningSetAuthority     = "read_only_current_authority_frontier"

	GoverningSetPostureSingle   = "single_current_authority"
	GoverningSetPostureOverlap  = "overlap_needs_review"
	GoverningSetPostureConflict = "conflict_requires_operator"
)

type CurrentGoverningSetReport struct {
	SchemaVersion int                         `json:"schema_version"`
	Authority     string                      `json:"authority"`
	Snapshot      CurrentGoverningSetSnapshot `json:"snapshot"`
	Filter        *CurrentGoverningSetFilter  `json:"filter,omitempty"`
	Summary       CurrentGoverningSetSummary  `json:"summary"`
	Sets          []CurrentGoverningSet       `json:"sets"`
}

type CurrentGoverningSetSnapshot struct {
	GeneratedAt           string   `json:"generated_at"`
	Source                string   `json:"source"`
	Projection            string   `json:"projection"`
	AuthorityBoundary     string   `json:"authority_boundary"`
	CurrentStatusPolicy   []string `json:"current_status_policy"`
	TerminalStatusPolicy  []string `json:"terminal_status_policy"`
	TerminalHistoryPolicy string   `json:"terminal_history_policy"`
	FilterApplied         bool     `json:"filter_applied"`
}

type CurrentGoverningSetFilter struct {
	Query      string `json:"query,omitempty"`
	SubjectRef string `json:"subject_ref,omitempty"`
	TargetRef  string `json:"target_ref,omitempty"`
}

type CurrentGoverningSetSummary struct {
	CurrentDecisions       int `json:"current_decisions"`
	GoverningSets          int `json:"governing_sets"`
	ConflictSets           int `json:"conflict_sets"`
	OverlapReviewSets      int `json:"overlap_review_sets"`
	MissingExplicitSubject int `json:"missing_explicit_subject"`
	FallbackTargetSets     int `json:"fallback_target_sets"`
	ScopeEnrichmentSets    int `json:"scope_enrichment_sets"`
	TerminalHistoryRefs    int `json:"terminal_history_refs"`
}

type CurrentGoverningSet struct {
	SetID                    string                       `json:"set_id"`
	SubjectRef               string                       `json:"subject_ref"`
	SubjectResolution        string                       `json:"subject_resolution"`
	BoundedContext           string                       `json:"bounded_context"`
	TargetRef                string                       `json:"target_ref"`
	TargetResolution         string                       `json:"target_resolution"`
	WholeFileFallbackTargets []string                     `json:"whole_file_fallback_targets,omitempty"`
	Posture                  string                       `json:"posture"`
	CurrentDecisionRefs      []string                     `json:"current_decision_refs"`
	CurrentDecisions         []DecisionReconciliationItem `json:"current_decisions"`
	TerminalHistoryRefs      []string                     `json:"terminal_history_refs,omitempty"`
	OperatorRequired         bool                         `json:"operator_required"`
	AuthorityBoundary        string                       `json:"authority_boundary"`
	Basis                    []string                     `json:"basis"`
	ScopeRepairHints         []string                     `json:"scope_repair_hints,omitempty"`
}

type currentGoverningSetBucket struct {
	subjectRef        string
	subjectResolution string
	boundedContext    string
	targetRef         string
	targetResolution  string
	items             []DecisionReconciliationItem
	historyRefs       []string
}

func BuildCurrentGoverningSetReport(
	ctx context.Context,
	store ArtifactStore,
) (CurrentGoverningSetReport, error) {
	return BuildCurrentGoverningSetReportFiltered(ctx, store, CurrentGoverningSetFilter{})
}

func BuildCurrentGoverningSetReportFiltered(
	ctx context.Context,
	store ArtifactStore,
	filter CurrentGoverningSetFilter,
) (CurrentGoverningSetReport, error) {
	artifacts, err := store.ListByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		return CurrentGoverningSetReport{}, err
	}

	buckets := map[string]*currentGoverningSetBucket{}
	for _, candidate := range artifacts {
		if candidate == nil {
			continue
		}
		if !reconciliationStatusInScope(candidate.Meta.Status) {
			continue
		}
		decision, err := store.Get(ctx, candidate.Meta.ID)
		if err != nil {
			return CurrentGoverningSetReport{}, err
		}
		item, err := buildDecisionReconciliationItem(ctx, store, decision)
		if err != nil {
			return CurrentGoverningSetReport{}, err
		}
		for _, targetRef := range currentGoverningTargetRefs(item) {
			key := currentGoverningSetKey(item, targetRef)
			bucket := buckets[key]
			if bucket == nil {
				bucket = &currentGoverningSetBucket{
					subjectRef:        decisionReconciliationSubject(item),
					subjectResolution: item.DecisionSubjectResolution,
					boundedContext:    item.BoundedContext,
					targetRef:         targetRef,
					targetResolution:  currentGoverningTargetResolution(item, targetRef),
				}
				buckets[key] = bucket
			}
			bucket.items = append(bucket.items, item)
			historyRefs, err := currentGoverningTerminalHistoryRefs(ctx, store, item)
			if err != nil {
				return CurrentGoverningSetReport{}, err
			}
			bucket.historyRefs = append(bucket.historyRefs, historyRefs...)
		}
	}

	sets := make([]CurrentGoverningSet, 0, len(buckets))
	for _, bucket := range buckets {
		sets = append(sets, currentGoverningSetFromBucket(bucket))
	}
	sort.SliceStable(sets, func(i, j int) bool {
		return currentGoverningSetLess(sets[i], sets[j])
	})

	summary := currentGoverningSetSummary(sets)
	report := CurrentGoverningSetReport{
		SchemaVersion: CurrentGoverningSetSchemaVersion,
		Authority:     CurrentGoverningSetAuthority,
		Snapshot:      newCurrentGoverningSetSnapshot(false),
		Summary:       summary,
		Sets:          sets,
	}
	return FilterCurrentGoverningSetReport(report, filter), nil
}

func FilterCurrentGoverningSetReport(
	report CurrentGoverningSetReport,
	filter CurrentGoverningSetFilter,
) CurrentGoverningSetReport {
	normalized := normalizeCurrentGoverningSetFilter(filter)
	if currentGoverningSetFilterEmpty(normalized) {
		return report
	}
	sets := make([]CurrentGoverningSet, 0, len(report.Sets))
	for _, set := range report.Sets {
		if currentGoverningSetMatchesFilter(set, normalized) {
			sets = append(sets, set)
		}
	}
	report.Filter = &normalized
	report.Snapshot.FilterApplied = true
	report.Sets = sets
	report.Summary = currentGoverningSetSummary(sets)
	return report
}

func newCurrentGoverningSetSnapshot(filterApplied bool) CurrentGoverningSetSnapshot {
	return CurrentGoverningSetSnapshot{
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
		Source:                "artifact_store_decision_records",
		Projection:            "refreshable_current_governing_frontier",
		AuthorityBoundary:     "derived_read_only_not_gate_decision",
		CurrentStatusPolicy:   []string{string(StatusActive), string(StatusRefreshDue)},
		TerminalStatusPolicy:  []string{string(StatusSuperseded), string(StatusDeprecated)},
		TerminalHistoryPolicy: "terminal decisions stay searchable history and are excluded from current authority",
		FilterApplied:         filterApplied,
	}
}

func currentGoverningSetFromBucket(bucket *currentGoverningSetBucket) CurrentGoverningSet {
	items := normalizeDecisionReconciliationItems(bucket.items)
	decisionRefs := make([]string, 0, len(items))
	for _, item := range items {
		decisionRefs = append(decisionRefs, item.DecisionID)
	}
	historyRefs := compactSortedStrings(bucket.historyRefs)
	posture := currentGoverningSetPosture(items)
	return CurrentGoverningSet{
		SetID:                    currentGoverningSetID(bucket.subjectRef, bucket.boundedContext, bucket.targetRef),
		SubjectRef:               bucket.subjectRef,
		SubjectResolution:        bucket.subjectResolution,
		BoundedContext:           bucket.boundedContext,
		TargetRef:                bucket.targetRef,
		TargetResolution:         bucket.targetResolution,
		WholeFileFallbackTargets: currentGoverningWholeFileFallbackTargets(items),
		Posture:                  posture,
		CurrentDecisionRefs:      decisionRefs,
		CurrentDecisions:         items,
		TerminalHistoryRefs:      historyRefs,
		OperatorRequired:         posture != GoverningSetPostureSingle,
		AuthorityBoundary:        "derived_read_only_not_gate_decision",
		Basis:                    currentGoverningSetBasis(posture),
		ScopeRepairHints:         currentGoverningScopeRepairHints(items),
	}
}

func currentGoverningTargetRefs(item DecisionReconciliationItem) []string {
	if len(item.GovernanceTargets) > 0 {
		return compactSortedStrings(item.GovernanceTargets)
	}
	return []string{"unscoped:" + item.DecisionID}
}

func currentGoverningTargetResolution(item DecisionReconciliationItem, targetRef string) string {
	if strings.HasPrefix(targetRef, "unscoped:") {
		if len(item.WholeFileFallbackTargets) > 0 {
			return "whole_file_fallback_requires_scope_enrichment"
		}
		return "missing_explicit_target_unique_decision_scope"
	}
	return "explicit_governance_or_watch_target"
}

func currentGoverningWholeFileFallbackTargets(items []DecisionReconciliationItem) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.WholeFileFallbackTargets...)
	}
	return compactSortedStrings(out)
}

func currentGoverningScopeRepairHints(items []DecisionReconciliationItem) []string {
	hints := []string{}
	for _, item := range items {
		hints = append(hints, item.ScopeRepairHint)
	}
	return compactSortedStrings(hints)
}

func currentGoverningSetKey(item DecisionReconciliationItem, targetRef string) string {
	return strings.Join([]string{
		decisionReconciliationSubject(item),
		item.BoundedContext,
		targetRef,
	}, "|")
}

func currentGoverningTerminalHistoryRefs(
	ctx context.Context,
	store ArtifactStore,
	item DecisionReconciliationItem,
) ([]string, error) {
	refs := []string{}
	for _, link := range append(item.Links, item.Backlinks...) {
		if link.Type != "supersedes" {
			continue
		}
		artifact, err := store.Get(ctx, link.Ref)
		if err != nil {
			continue
		}
		if artifact.Meta.Status == StatusSuperseded || artifact.Meta.Status == StatusDeprecated {
			refs = append(refs, artifact.Meta.ID)
		}
	}
	return compactSortedStrings(refs), nil
}

func currentGoverningSetPosture(items []DecisionReconciliationItem) string {
	if currentGoverningSetHasContradiction(items) {
		return GoverningSetPostureConflict
	}
	if len(items) > 1 {
		return GoverningSetPostureOverlap
	}
	return GoverningSetPostureSingle
}

func currentGoverningSetHasContradiction(items []DecisionReconciliationItem) bool {
	ids := map[string]struct{}{}
	for _, item := range items {
		ids[item.DecisionID] = struct{}{}
	}
	for _, item := range items {
		for _, link := range append(item.Links, item.Backlinks...) {
			if link.Type != "contradicts" {
				continue
			}
			if _, ok := ids[link.Ref]; ok {
				return true
			}
		}
	}
	return false
}

func currentGoverningSetBasis(posture string) []string {
	switch posture {
	case GoverningSetPostureConflict:
		return []string{"explicit contradicts link among current governing decisions"}
	case GoverningSetPostureOverlap:
		return []string{"multiple current decisions share subject, bounded context, and target"}
	default:
		return []string{"single current decision governs this subject/context/target"}
	}
}

func currentGoverningSetSummary(sets []CurrentGoverningSet) CurrentGoverningSetSummary {
	currentIDs := map[string]struct{}{}
	historyIDs := map[string]struct{}{}
	summary := CurrentGoverningSetSummary{
		GoverningSets: len(sets),
	}
	for _, set := range sets {
		if set.Posture == GoverningSetPostureConflict {
			summary.ConflictSets++
		}
		if set.Posture == GoverningSetPostureOverlap {
			summary.OverlapReviewSets++
		}
		if len(set.WholeFileFallbackTargets) > 0 {
			summary.FallbackTargetSets++
		}
		if len(set.ScopeRepairHints) > 0 {
			summary.ScopeEnrichmentSets++
		}
		for _, item := range set.CurrentDecisions {
			currentIDs[item.DecisionID] = struct{}{}
			if item.DecisionSubjectRef == "" {
				summary.MissingExplicitSubject++
			}
		}
		for _, ref := range set.TerminalHistoryRefs {
			historyIDs[ref] = struct{}{}
		}
	}
	summary.CurrentDecisions = len(currentIDs)
	summary.TerminalHistoryRefs = len(historyIDs)
	return summary
}

func normalizeCurrentGoverningSetFilter(
	filter CurrentGoverningSetFilter,
) CurrentGoverningSetFilter {
	return CurrentGoverningSetFilter{
		Query:      strings.TrimSpace(filter.Query),
		SubjectRef: strings.TrimSpace(filter.SubjectRef),
		TargetRef:  strings.TrimSpace(filter.TargetRef),
	}
}

func currentGoverningSetFilterEmpty(filter CurrentGoverningSetFilter) bool {
	return filter.Query == "" && filter.SubjectRef == "" && filter.TargetRef == ""
}

func currentGoverningSetMatchesFilter(
	set CurrentGoverningSet,
	filter CurrentGoverningSetFilter,
) bool {
	if filter.SubjectRef != "" && set.SubjectRef != filter.SubjectRef {
		return false
	}
	if filter.TargetRef != "" && set.TargetRef != filter.TargetRef {
		return false
	}
	if filter.Query != "" && !currentGoverningSetMatchesQuery(set, filter.Query) {
		return false
	}
	return true
}

func currentGoverningSetMatchesQuery(
	set CurrentGoverningSet,
	query string,
) bool {
	values := []string{
		set.SetID,
		set.SubjectRef,
		set.SubjectResolution,
		set.BoundedContext,
		set.TargetRef,
		set.TargetResolution,
		set.Posture,
	}
	values = append(values, set.CurrentDecisionRefs...)
	values = append(values, set.TerminalHistoryRefs...)
	values = append(values, set.WholeFileFallbackTargets...)
	values = append(values, set.ScopeRepairHints...)
	return stringSliceContainsFold(values, query)
}

func stringSliceContainsFold(values []string, query string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

func currentGoverningSetID(subjectRef, boundedContext, targetRef string) string {
	sum := sha1.Sum([]byte(subjectRef + "|" + boundedContext + "|" + targetRef))
	return fmt.Sprintf("governing-set-%x", sum[:6])
}

func currentGoverningSetLess(left CurrentGoverningSet, right CurrentGoverningSet) bool {
	leftRank := currentGoverningPostureRank(left.Posture)
	rightRank := currentGoverningPostureRank(right.Posture)
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	if len(left.CurrentDecisionRefs) != len(right.CurrentDecisionRefs) {
		return len(left.CurrentDecisionRefs) > len(right.CurrentDecisionRefs)
	}
	if left.SubjectRef != right.SubjectRef {
		return left.SubjectRef < right.SubjectRef
	}
	if left.TargetRef != right.TargetRef {
		return left.TargetRef < right.TargetRef
	}
	return left.SetID < right.SetID
}

func currentGoverningPostureRank(posture string) int {
	switch posture {
	case GoverningSetPostureConflict:
		return 0
	case GoverningSetPostureOverlap:
		return 1
	default:
		return 2
	}
}
