package artifact

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
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
	SchemaVersion     int                               `json:"schema_version"`
	Authority         string                            `json:"authority"`
	View              string                            `json:"view,omitempty"`
	Snapshot          CurrentGoverningSetSnapshot       `json:"snapshot"`
	Filter            *CurrentGoverningSetFilter        `json:"filter,omitempty"`
	Summary           CurrentGoverningSetSummary        `json:"summary"`
	AuthorityFrontier CurrentGoverningAuthorityFrontier `json:"authority_frontier"`
	CompactSets       []CurrentGoverningSetCompact      `json:"compact_sets,omitempty"`
	OmittedSets       int                               `json:"omitted_sets,omitempty"`
	FullAuditCommand  string                            `json:"full_audit_command,omitempty"`
	Sets              []CurrentGoverningSet             `json:"sets,omitempty"`
}

type CurrentGoverningSetSnapshot struct {
	GeneratedAt           string   `json:"generated_at"`
	SnapshotDigest        string   `json:"snapshot_digest"`
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

type CurrentGoverningAuthorityFrontier struct {
	AuthorityBoundary     string   `json:"authority_boundary"`
	CurrentStatusPolicy   []string `json:"current_status_policy"`
	TerminalStatusPolicy  []string `json:"terminal_status_policy"`
	CurrentDecisionRefs   []string `json:"current_decision_refs"`
	TerminalHistoryRefs   []string `json:"terminal_history_refs,omitempty"`
	TerminalHistoryPolicy string   `json:"terminal_history_policy"`
}

type CurrentGoverningSet struct {
	SetID                    string                          `json:"set_id"`
	SubjectRef               string                          `json:"subject_ref"`
	SubjectResolution        string                          `json:"subject_resolution"`
	BoundedContext           string                          `json:"bounded_context"`
	TargetRef                string                          `json:"target_ref"`
	AnswerPaths              []CurrentGoverningSetAnswerPath `json:"answer_paths,omitempty"`
	TargetResolution         string                          `json:"target_resolution"`
	WholeFileFallbackTargets []string                        `json:"whole_file_fallback_targets,omitempty"`
	Posture                  string                          `json:"posture"`
	CurrentDecisionRefs      []string                        `json:"current_decision_refs"`
	CurrentDecisions         []DecisionReconciliationItem    `json:"current_decisions"`
	TerminalHistoryRefs      []string                        `json:"terminal_history_refs,omitempty"`
	OperatorRequired         bool                            `json:"operator_required"`
	AuthorityBoundary        string                          `json:"authority_boundary"`
	Basis                    []string                        `json:"basis"`
	ScopeRepairHints         []string                        `json:"scope_repair_hints,omitempty"`
}

type CurrentGoverningSetCompact struct {
	SetID                    string                          `json:"set_id"`
	SubjectRef               string                          `json:"subject_ref"`
	SubjectResolution        string                          `json:"subject_resolution"`
	BoundedContext           string                          `json:"bounded_context,omitempty"`
	TargetRef                string                          `json:"target_ref"`
	TargetResolution         string                          `json:"target_resolution"`
	Posture                  string                          `json:"posture"`
	CurrentDecisionRefs      []string                        `json:"current_decision_refs"`
	TerminalHistoryRefs      []string                        `json:"terminal_history_refs,omitempty"`
	CurrentDecisionCount     int                             `json:"current_decision_count"`
	AnswerPaths              []CurrentGoverningSetAnswerPath `json:"answer_paths,omitempty"`
	OperatorRequired         bool                            `json:"operator_required"`
	ScopeRepairHints         []string                        `json:"scope_repair_hints,omitempty"`
	WholeFileFallbackTargets []string                        `json:"whole_file_fallback_targets,omitempty"`
}

type CurrentGoverningSetAnswerPath struct {
	TargetKind        string `json:"target_kind"`
	TargetRef         string `json:"target_ref"`
	CLI               string `json:"cli"`
	MCPCall           string `json:"mcp_call"`
	ExactRecordNeeded string `json:"exact_record_needed,omitempty"`
	AuthorityBoundary string `json:"authority_boundary"`
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
		SchemaVersion:     CurrentGoverningSetSchemaVersion,
		Authority:         CurrentGoverningSetAuthority,
		Snapshot:          newCurrentGoverningSetSnapshot(false),
		Summary:           summary,
		AuthorityFrontier: currentGoverningAuthorityFrontier(sets),
		Sets:              sets,
	}
	report = withCurrentGoverningSetSnapshotDigest(report)
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
	report.AuthorityFrontier = currentGoverningAuthorityFrontier(sets)
	return withCurrentGoverningSetSnapshotDigest(report)
}

func CompactCurrentGoverningSetReport(
	report CurrentGoverningSetReport,
	setLimit int,
) CurrentGoverningSetReport {
	compact := report
	compact.View = "compact"
	compact.FullAuditCommand = `haft_query(action="governing_set", full=true)`

	sets := report.Sets
	if setLimit > 0 && len(sets) > setLimit {
		compact.OmittedSets = len(sets) - setLimit
		sets = sets[:setLimit]
	}
	compact.CompactSets = currentGoverningSetCompactSets(sets)
	compact.Sets = nil

	return compact
}

func currentGoverningSetCompactSets(
	sets []CurrentGoverningSet,
) []CurrentGoverningSetCompact {
	out := make([]CurrentGoverningSetCompact, 0, len(sets))
	for _, set := range sets {
		out = append(out, currentGoverningSetCompact(set))
	}
	return out
}

func currentGoverningSetCompact(
	set CurrentGoverningSet,
) CurrentGoverningSetCompact {
	return CurrentGoverningSetCompact{
		SetID:                    set.SetID,
		SubjectRef:               set.SubjectRef,
		SubjectResolution:        set.SubjectResolution,
		BoundedContext:           set.BoundedContext,
		TargetRef:                set.TargetRef,
		TargetResolution:         set.TargetResolution,
		Posture:                  set.Posture,
		CurrentDecisionRefs:      append([]string(nil), set.CurrentDecisionRefs...),
		TerminalHistoryRefs:      append([]string(nil), set.TerminalHistoryRefs...),
		CurrentDecisionCount:     len(set.CurrentDecisionRefs),
		AnswerPaths:              append([]CurrentGoverningSetAnswerPath(nil), set.AnswerPaths...),
		OperatorRequired:         set.OperatorRequired,
		ScopeRepairHints:         append([]string(nil), set.ScopeRepairHints...),
		WholeFileFallbackTargets: append([]string(nil), set.WholeFileFallbackTargets...),
	}
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

func withCurrentGoverningSetSnapshotDigest(
	report CurrentGoverningSetReport,
) CurrentGoverningSetReport {
	report.Snapshot.SnapshotDigest = currentGoverningSetSnapshotDigest(report)
	return report
}

func currentGoverningSetSnapshotDigest(report CurrentGoverningSetReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("schema=%d\n", report.SchemaVersion))
	sb.WriteString("authority=" + report.Authority + "\n")
	sb.WriteString("source=" + report.Snapshot.Source + "\n")
	sb.WriteString("projection=" + report.Snapshot.Projection + "\n")
	sb.WriteString("authority_boundary=" + report.Snapshot.AuthorityBoundary + "\n")
	sb.WriteString("current_status_policy=" + strings.Join(report.Snapshot.CurrentStatusPolicy, ",") + "\n")
	sb.WriteString("terminal_status_policy=" + strings.Join(report.Snapshot.TerminalStatusPolicy, ",") + "\n")
	sb.WriteString("terminal_history_policy=" + report.Snapshot.TerminalHistoryPolicy + "\n")
	sb.WriteString(fmt.Sprintf("filter_applied=%t\n", report.Snapshot.FilterApplied))
	sb.WriteString(currentGoverningSetSummaryDigestLine(report.Summary))
	sb.WriteString(currentGoverningAuthorityFrontierDigestLine(report.AuthorityFrontier))
	for _, set := range report.Sets {
		sb.WriteString(currentGoverningSetDigestLine(set))
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func currentGoverningSetSummaryDigestLine(summary CurrentGoverningSetSummary) string {
	return fmt.Sprintf(
		"summary=%d,%d,%d,%d,%d,%d,%d,%d\n",
		summary.CurrentDecisions,
		summary.GoverningSets,
		summary.ConflictSets,
		summary.OverlapReviewSets,
		summary.MissingExplicitSubject,
		summary.FallbackTargetSets,
		summary.ScopeEnrichmentSets,
		summary.TerminalHistoryRefs,
	)
}

func currentGoverningAuthorityFrontierDigestLine(
	frontier CurrentGoverningAuthorityFrontier,
) string {
	return fmt.Sprintf(
		"frontier=%s|%s|%s|%s|%s\n",
		frontier.AuthorityBoundary,
		strings.Join(frontier.CurrentStatusPolicy, ","),
		strings.Join(frontier.TerminalStatusPolicy, ","),
		strings.Join(frontier.CurrentDecisionRefs, ","),
		strings.Join(frontier.TerminalHistoryRefs, ","),
	)
}

func currentGoverningSetDigestLine(set CurrentGoverningSet) string {
	return fmt.Sprintf(
		"set=%s|%s|%s|%s|%s|%s|%s|%t|%s\n",
		set.SetID,
		set.SubjectRef,
		set.SubjectResolution,
		set.BoundedContext,
		set.TargetRef,
		set.TargetResolution,
		set.Posture,
		set.OperatorRequired,
		strings.Join(set.CurrentDecisionRefs, ",")+"|"+strings.Join(set.TerminalHistoryRefs, ","),
	)
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
		AnswerPaths:              currentGoverningSetAnswerPaths(bucket.targetRef),
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

func currentGoverningSetAnswerPaths(targetRef string) []CurrentGoverningSetAnswerPath {
	trimmed := strings.TrimSpace(targetRef)
	if trimmed == "" {
		return nil
	}
	return []CurrentGoverningSetAnswerPath{{
		TargetKind:        currentGoverningTargetKind(trimmed),
		TargetRef:         trimmed,
		CLI:               fmt.Sprintf("haft decision governing-set --target-ref %q --json", trimmed),
		MCPCall:           fmt.Sprintf("haft_query(action=\"governing_set\", source_refs=[%q])", trimmed),
		ExactRecordNeeded: currentGoverningExactRecordHint(trimmed),
		AuthorityBoundary: "answer_path_is_read_only_not_evidence_or_gate_decision",
	}}
}

func currentGoverningTargetKind(targetRef string) string {
	switch {
	case strings.HasPrefix(targetRef, "claim:"):
		return "claim"
	case strings.Contains(targetRef, "#claim"):
		return "claim"
	case strings.HasPrefix(targetRef, "spec-section:"):
		return "spec_section"
	case strings.HasPrefix(targetRef, "spec_section:"):
		return "spec_section"
	case strings.HasPrefix(targetRef, "api_contract:"):
		return "api_contract"
	case strings.HasPrefix(targetRef, "api-contract:"):
		return "api_contract"
	case strings.HasPrefix(targetRef, "invariant:"):
		return "invariant"
	case strings.HasPrefix(targetRef, "symbol:"):
		return "symbol"
	case strings.HasPrefix(targetRef, "whole_file_fallback:"):
		return "whole_file_fallback"
	case strings.HasPrefix(targetRef, "whole-file-fallback:"):
		return "whole_file_fallback"
	case strings.HasPrefix(targetRef, "file:"):
		return "file_fallback"
	case strings.HasPrefix(targetRef, "unscoped:"):
		return "unscoped_decision"
	default:
		return "governance_target"
	}
}

func currentGoverningExactRecordHint(targetRef string) string {
	switch currentGoverningTargetKind(targetRef) {
	case "claim":
		return "claim lifecycle/detail view"
	case "spec_section":
		return "haft spec section lifecycle/detail"
	case "api_contract":
		return "interface contract or exact API-contract carrier"
	case "invariant":
		return "decision invariant or evidence path detail"
	case "symbol":
		return "haft_query code_context/node for symbol plus governing-set filtered JSON"
	case "whole_file_fallback":
		return "scope enrichment selection before stronger use"
	case "file_fallback":
		return "scope enrichment selection before stronger use"
	case "unscoped_decision":
		return "decision scope enrichment before stronger use"
	default:
		return "governing-set filtered JSON"
	}
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

func currentGoverningAuthorityFrontier(sets []CurrentGoverningSet) CurrentGoverningAuthorityFrontier {
	currentRefs := []string{}
	terminalRefs := []string{}
	for _, set := range sets {
		currentRefs = append(currentRefs, set.CurrentDecisionRefs...)
		terminalRefs = append(terminalRefs, set.TerminalHistoryRefs...)
	}
	return CurrentGoverningAuthorityFrontier{
		AuthorityBoundary:     "current_decision_refs_are_governing_authority_terminal_history_refs_are_not",
		CurrentStatusPolicy:   []string{string(StatusActive), string(StatusRefreshDue)},
		TerminalStatusPolicy:  []string{string(StatusSuperseded), string(StatusDeprecated)},
		CurrentDecisionRefs:   compactSortedStrings(currentRefs),
		TerminalHistoryRefs:   compactSortedStrings(terminalRefs),
		TerminalHistoryPolicy: "terminal decisions stay searchable history and are excluded from current authority",
	}
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
