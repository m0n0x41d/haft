package project

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const SpecPlanAuthorityNotice = "SpecPlan proposals are review drafts only; listing them does not create DecisionRecords, WorkCommissions, or semantic authority."

type SpecPlanActionKind string

const (
	SpecPlanActionAccept  SpecPlanActionKind = "accept"
	SpecPlanActionMerge   SpecPlanActionKind = "merge"
	SpecPlanActionSplit   SpecPlanActionKind = "split"
	SpecPlanActionDiscard SpecPlanActionKind = "discard"
)

type SpecPlanReport struct {
	Authority     string                 `json:"authority"`
	ReviewActions []SpecPlanReviewAction `json:"review_actions"`
	Proposals     []SpecPlanProposal     `json:"proposals"`
	Summary       SpecPlanSummary        `json:"summary"`
}

type SpecPlanReviewAction struct {
	Kind       SpecPlanActionKind `json:"kind"`
	Effect     string             `json:"effect"`
	Executable bool               `json:"executable"`
	CommandGap string             `json:"command_gap,omitempty"`
}

type SpecPlanProposal struct {
	ID                  string                `json:"id"`
	Title               string                `json:"title"`
	DocumentKind        string                `json:"document_kind"`
	SpecKind            string                `json:"spec_kind"`
	AffectedArea        string                `json:"affected_area"`
	DependencyRefs      []string              `json:"dependency_refs"`
	SectionRefs         []string              `json:"section_refs"`
	States              []SpecCoverageState   `json:"states"`
	Reasons             []string              `json:"reasons"`
	DecisionRecordDraft SpecPlanDecisionDraft `json:"decision_record_draft"`
}

type SpecPlanDecisionDraft struct {
	Kind                 string                        `json:"kind"`
	SelectedTitle        string                        `json:"selected_title"`
	WhySelected          string                        `json:"why_selected"`
	SelectionPolicy      string                        `json:"selection_policy"`
	CounterArgument      string                        `json:"counterargument"`
	WhyNotOthers         []SpecPlanRejectedAlternative `json:"why_not_others"`
	WeakestLink          string                        `json:"weakest_link"`
	RollbackTriggers     []string                      `json:"rollback_triggers"`
	EvidenceRequirements []string                      `json:"evidence_requirements"`
	RefreshTriggers      []string                      `json:"refresh_triggers"`
	SectionRefs          []string                      `json:"section_refs"`
}

type SpecPlanRejectedAlternative struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

type SpecPlanSummary struct {
	TotalCandidates int            `json:"total_candidates"`
	TotalProposals  int            `json:"total_proposals"`
	StateCounts     map[string]int `json:"state_counts"`
}

type specPlanGroupKey struct {
	DocumentKind   string
	SpecKind       string
	AffectedArea   string
	DependencyKey  string
	DependencyRefs []string
}

type specPlanGroup struct {
	Key      specPlanGroupKey
	Sections []SpecCoverageSection
}

func BuildSpecPlan(report SpecCoverageReport) SpecPlanReport {
	report = normalizeSpecCoverageReport(report)
	candidates := specPlanCandidateSections(report.Sections)
	groups := groupSpecPlanCandidates(candidates)
	proposals := specPlanProposals(groups)

	return normalizeSpecPlanReport(SpecPlanReport{
		Authority:     SpecPlanAuthorityNotice,
		ReviewActions: specPlanReviewActions(),
		Proposals:     proposals,
		Summary:       summarizeSpecPlan(candidates, proposals),
	})
}

func ParseSpecPlanActionKind(value string) (SpecPlanActionKind, error) {
	kind := SpecPlanActionKind(strings.TrimSpace(value))
	if kind.IsValid() {
		return kind, nil
	}

	return "", fmt.Errorf("invalid spec plan action %q", value)
}

func (kind SpecPlanActionKind) IsValid() bool {
	switch kind {
	case SpecPlanActionAccept, SpecPlanActionMerge, SpecPlanActionSplit, SpecPlanActionDiscard:
		return true
	default:
		return false
	}
}

func (kind SpecPlanActionKind) Executable() bool {
	return kind == SpecPlanActionAccept
}

func FindSpecPlanProposal(report SpecPlanReport, proposalID string) (SpecPlanProposal, bool) {
	id := strings.TrimSpace(proposalID)
	if id == "" {
		return SpecPlanProposal{}, false
	}

	report = normalizeSpecPlanReport(report)
	for _, proposal := range report.Proposals {
		if proposal.ID != id {
			continue
		}

		return proposal, true
	}

	return SpecPlanProposal{}, false
}

func specPlanCandidateSections(sections []SpecCoverageSection) []SpecCoverageSection {
	candidates := make([]SpecCoverageSection, 0)

	for _, section := range sections {
		if !sectionNeedsSpecPlan(section) {
			continue
		}

		candidates = append(candidates, section)
	}

	return candidates
}

func sectionNeedsSpecPlan(section SpecCoverageSection) bool {
	switch section.State {
	case SpecCoverageUncovered, SpecCoverageStale:
		return true
	default:
		return false
	}
}

func groupSpecPlanCandidates(sections []SpecCoverageSection) []specPlanGroup {
	groupsByKey := map[string]specPlanGroup{}

	for _, section := range sections {
		key := specPlanKey(section)
		groupID := specPlanGroupID(key)
		group := groupsByKey[groupID]
		group.Key = key
		group.Sections = append(group.Sections, section)
		groupsByKey[groupID] = group
	}

	groups := make([]specPlanGroup, 0, len(groupsByKey))
	for _, group := range groupsByKey {
		groups = append(groups, group)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		left := specPlanGroupID(groups[i].Key)
		right := specPlanGroupID(groups[j].Key)
		return left < right
	})

	return groups
}

func specPlanKey(section SpecCoverageSection) specPlanGroupKey {
	dependencyRefs := sortedUniqueStrings(section.DependsOn)

	return specPlanGroupKey{
		DocumentKind:   strings.TrimSpace(section.DocumentKind),
		SpecKind:       strings.TrimSpace(section.SpecKind),
		AffectedArea:   specPlanAffectedArea(section),
		DependencyKey:  strings.Join(dependencyRefs, "\x00"),
		DependencyRefs: dependencyRefs,
	}
}

func specPlanGroupID(key specPlanGroupKey) string {
	parts := []string{
		key.DocumentKind,
		key.SpecKind,
		key.AffectedArea,
		key.DependencyKey,
	}

	return strings.Join(parts, "\x00")
}

func specPlanAffectedArea(section SpecCoverageSection) string {
	targetAreas := specPlanTargetAreas(section.TargetRefs)
	if len(targetAreas) > 0 {
		return strings.Join(targetAreas, "+")
	}

	sectionArea := specPlanAreaFromSpecSectionID(section.SectionID)
	if sectionArea != "" {
		return sectionArea
	}

	pathArea := strings.TrimSuffix(filepath.Base(section.Path), filepath.Ext(section.Path))
	if pathArea != "" && pathArea != "." {
		return pathArea
	}

	return "general"
}

func specPlanTargetAreas(refs []string) []string {
	areas := make([]string, 0, len(refs))

	for _, ref := range refs {
		area := specPlanAreaFromSpecSectionID(ref)
		if area == "" {
			continue
		}

		areas = append(areas, area)
	}

	return sortedUniqueStrings(areas)
}

func specPlanAreaFromSpecSectionID(id string) string {
	parts := strings.Split(strings.TrimSpace(id), ".")
	if len(parts) < 3 {
		return ""
	}
	if specPlanNumericToken(parts[1]) {
		return ""
	}

	return parts[1]
}

func specPlanNumericToken(value string) bool {
	if value == "" {
		return false
	}

	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

func specPlanProposals(groups []specPlanGroup) []SpecPlanProposal {
	proposals := make([]SpecPlanProposal, 0, len(groups))

	for index, group := range groups {
		sectionRefs := specPlanGroupSectionRefs(group)
		proposals = append(proposals, SpecPlanProposal{
			ID:                  fmt.Sprintf("spec-plan-%03d", index+1),
			Title:               specPlanProposalTitle(group),
			DocumentKind:        group.Key.DocumentKind,
			SpecKind:            group.Key.SpecKind,
			AffectedArea:        group.Key.AffectedArea,
			DependencyRefs:      group.Key.DependencyRefs,
			SectionRefs:         sectionRefs,
			States:              specPlanGroupStates(group),
			Reasons:             specPlanProposalReasons(group),
			DecisionRecordDraft: specPlanDecisionDraft(group, sectionRefs),
		})
	}

	return proposals
}

func specPlanGroupSectionRefs(group specPlanGroup) []string {
	refs := make([]string, 0, len(group.Sections))

	for _, section := range group.Sections {
		refs = append(refs, section.SectionID)
	}

	return sortedUniqueStrings(refs)
}

func specPlanGroupStates(group specPlanGroup) []SpecCoverageState {
	states := make([]string, 0, len(group.Sections))

	for _, section := range group.Sections {
		states = append(states, string(section.State))
	}

	unique := sortedUniqueStrings(states)
	result := make([]SpecCoverageState, 0, len(unique))
	for _, state := range unique {
		result = append(result, SpecCoverageState(state))
	}

	return result
}

func specPlanProposalReasons(group specPlanGroup) []string {
	reasons := make([]string, 0, len(group.Sections))

	for _, section := range group.Sections {
		reason := specPlanSectionReason(section)
		reasons = append(reasons, reason)
	}

	return sortedUniqueStrings(reasons)
}

func specPlanSectionReason(section SpecCoverageSection) string {
	why := strings.Join(section.Why, "; ")
	if why == "" {
		why = section.NextAction
	}
	if why == "" {
		why = "coverage state requires human review"
	}

	return fmt.Sprintf("%s is %s: %s", section.SectionID, section.State, why)
}

func specPlanProposalTitle(group specPlanGroup) string {
	area := group.Key.AffectedArea
	specKind := group.Key.SpecKind
	documentKind := group.Key.DocumentKind

	return fmt.Sprintf("Review %s %s coverage for %s", documentKind, specKind, area)
}

func specPlanDecisionDraft(group specPlanGroup, sectionRefs []string) SpecPlanDecisionDraft {
	return SpecPlanDecisionDraft{
		Kind:                 "DecisionRecord",
		SelectedTitle:        specPlanProposalTitle(group),
		WhySelected:          specPlanDraftWhySelected(group),
		SelectionPolicy:      "Group by document kind, spec kind, dependency signature, and affected area; human review may accept this group or defer for merge, split, or discard.",
		CounterArgument:      "The grouped sections may not share one decision boundary; accepting without review can create a misleading DecisionRecord.",
		WhyNotOthers:         specPlanRejectedAlternatives(),
		WeakestLink:          "The draft is only useful if the grouped sections represent one load-bearing decision boundary after human review.",
		RollbackTriggers:     specPlanRollbackTriggers(),
		EvidenceRequirements: specPlanEvidenceRequirements(),
		RefreshTriggers:      specPlanRefreshTriggers(),
		SectionRefs:          sectionRefs,
	}
}

func specPlanDraftWhySelected(group specPlanGroup) string {
	count := len(group.Sections)
	return fmt.Sprintf("%d uncovered or stale SpecSection(s) need a human-approved DecisionRecord before execution authority exists.", count)
}

func specPlanReviewActions() []SpecPlanReviewAction {
	return []SpecPlanReviewAction{
		{
			Kind:       SpecPlanActionAccept,
			Effect:     "create one DecisionRecord from this reviewed proposal",
			Executable: true,
		},
		{
			Kind:       SpecPlanActionMerge,
			Effect:     "combine proposal groups before creating a DecisionRecord",
			Executable: false,
			CommandGap: "No merge command is implemented in this slice; rerun spec plan and accept only coherent proposals.",
		},
		{
			Kind:       SpecPlanActionSplit,
			Effect:     "split section refs into smaller DecisionRecord drafts",
			Executable: false,
			CommandGap: "No split command is implemented in this slice; keep the proposal unaccepted until a split command exists.",
		},
		{
			Kind:       SpecPlanActionDiscard,
			Effect:     "drop a draft if the group is not one load-bearing decision",
			Executable: false,
			CommandGap: "No discard command is implemented in this slice; leaving the proposal unaccepted has the same persistence effect.",
		},
	}
}

func specPlanRejectedAlternatives() []SpecPlanRejectedAlternative {
	return []SpecPlanRejectedAlternative{
		{
			Variant: "One DecisionRecord per SpecSection",
			Reason:  "It would turn SpecPlan into a one-decision-per-bullet factory instead of preserving a coherent decision boundary.",
		},
		{
			Variant: "Leave the sections uncovered",
			Reason:  "It would leave active uncovered or stale SpecSections without a governing DecisionRecord.",
		},
	}
}

func specPlanRollbackTriggers() []string {
	return []string{
		"Human review determines the accepted proposal combines separate decision boundaries.",
		"SpecCoverage still reports the accepted section refs as uncovered after DecisionRecord creation.",
	}
}

func specPlanEvidenceRequirements() []string {
	return []string{
		"Rerun `haft spec coverage` and confirm the accepted section refs move from uncovered or stale to reasoned or stronger.",
	}
}

func specPlanRefreshTriggers() []string {
	return []string{
		"Any accepted SpecSection expires, changes materially, or drifts from its governing DecisionRecord.",
	}
}

func summarizeSpecPlan(
	candidates []SpecCoverageSection,
	proposals []SpecPlanProposal,
) SpecPlanSummary {
	summary := SpecPlanSummary{
		TotalCandidates: len(candidates),
		TotalProposals:  len(proposals),
		StateCounts:     map[string]int{},
	}

	for _, section := range candidates {
		summary.StateCounts[string(section.State)]++
	}

	return summary
}

func normalizeSpecPlanReport(report SpecPlanReport) SpecPlanReport {
	if report.Authority == "" {
		report.Authority = SpecPlanAuthorityNotice
	}
	if report.ReviewActions == nil {
		report.ReviewActions = []SpecPlanReviewAction{}
	}
	if report.Proposals == nil {
		report.Proposals = []SpecPlanProposal{}
	}
	if report.Summary.StateCounts == nil {
		report.Summary.StateCounts = map[string]int{}
	}

	for index := range report.Proposals {
		report.Proposals[index] = normalizeSpecPlanProposal(report.Proposals[index])
	}

	return report
}

func normalizeSpecPlanProposal(proposal SpecPlanProposal) SpecPlanProposal {
	proposal.DependencyRefs = sortedUniqueStrings(proposal.DependencyRefs)
	proposal.SectionRefs = sortedUniqueStrings(proposal.SectionRefs)
	proposal.Reasons = sortedUniqueStrings(proposal.Reasons)
	if proposal.States == nil {
		proposal.States = []SpecCoverageState{}
	}
	proposal.DecisionRecordDraft.SectionRefs = sortedUniqueStrings(proposal.DecisionRecordDraft.SectionRefs)
	if proposal.DecisionRecordDraft.WhyNotOthers == nil {
		proposal.DecisionRecordDraft.WhyNotOthers = []SpecPlanRejectedAlternative{}
	}
	if proposal.DecisionRecordDraft.RollbackTriggers == nil {
		proposal.DecisionRecordDraft.RollbackTriggers = []string{}
	}
	if proposal.DecisionRecordDraft.EvidenceRequirements == nil {
		proposal.DecisionRecordDraft.EvidenceRequirements = []string{}
	}
	if proposal.DecisionRecordDraft.RefreshTriggers == nil {
		proposal.DecisionRecordDraft.RefreshTriggers = []string{}
	}

	return proposal
}
