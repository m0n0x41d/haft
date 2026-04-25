package project

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const SpecPlanAuthorityNotice = "SpecPlan proposals are review drafts only; they do not create DecisionRecords, WorkCommissions, or semantic authority."

type SpecPlanReport struct {
	Authority     string                 `json:"authority"`
	ReviewActions []SpecPlanReviewAction `json:"review_actions"`
	Proposals     []SpecPlanProposal     `json:"proposals"`
	Summary       SpecPlanSummary        `json:"summary"`
}

type SpecPlanReviewAction struct {
	Kind   string `json:"kind"`
	Effect string `json:"effect"`
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
	Kind            string   `json:"kind"`
	SelectedTitle   string   `json:"selected_title"`
	WhySelected     string   `json:"why_selected"`
	SelectionPolicy string   `json:"selection_policy"`
	WeakestLink     string   `json:"weakest_link"`
	SectionRefs     []string `json:"section_refs"`
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
		Kind:            "DecisionRecord",
		SelectedTitle:   specPlanProposalTitle(group),
		WhySelected:     specPlanDraftWhySelected(group),
		SelectionPolicy: "Group by document kind, spec kind, dependency signature, and affected area; human review may merge, split, or discard before deciding.",
		WeakestLink:     "The draft is only useful if the grouped sections represent one load-bearing decision boundary after human review.",
		SectionRefs:     sectionRefs,
	}
}

func specPlanDraftWhySelected(group specPlanGroup) string {
	count := len(group.Sections)
	return fmt.Sprintf("%d uncovered or stale SpecSection(s) need a human-approved DecisionRecord before execution authority exists.", count)
}

func specPlanReviewActions() []SpecPlanReviewAction {
	return []SpecPlanReviewAction{
		{
			Kind:   "merge",
			Effect: "combine proposal groups before creating a DecisionRecord",
		},
		{
			Kind:   "split",
			Effect: "split section refs into smaller DecisionRecord drafts",
		},
		{
			Kind:   "discard",
			Effect: "drop a draft if the group is not one load-bearing decision",
		},
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
	if proposal.DecisionRecordDraft.SectionRefs == nil {
		proposal.DecisionRecordDraft.SectionRefs = []string{}
	}

	return proposal
}
