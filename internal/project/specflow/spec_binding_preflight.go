package specflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	SpecBindingPreflightSchemaVersion = 1
	SpecBindingPreflightRecordKind    = "spec_binding_preflight"
	SpecBindingPreflightAuthority     = "read_only_spec_binding_preflight"
	SpecBindingPreflightBoundary      = "preflight_is_advisory_validation_not_approval_baseline_evidence_gate_decision_claim_truth_global_truth_or_publication"

	SpecBindingProjectStateReady            = "ready"
	SpecBindingProjectStatePartial          = "partial"
	SpecBindingProjectStateNoSpecs          = "no_specs"
	SpecBindingProjectStateNoActiveSections = "no_active_sections"

	SpecBindingDecisionModeTactical = "tactical"
	SpecBindingDecisionModeStandard = "standard"
	SpecBindingDecisionModeDeep     = "deep"

	SpecBindingLoadBearingLow         = "low"
	SpecBindingLoadBearingNormal      = "normal"
	SpecBindingLoadBearingLoadBearing = "load_bearing"

	SpecBindingStateNoSpecs           = "no_specs"
	SpecBindingStateNoActiveSections  = "no_active_sections"
	SpecBindingStateProvidedRefsValid = "provided_refs_valid"
	SpecBindingStateInvalidRefs       = "invalid_refs"
	SpecBindingStateBoundExisting     = "bound_existing"
	SpecBindingStateAmbiguous         = "ambiguous"
	SpecBindingStateDraftNeeded       = "draft_section_needed"
	SpecBindingStateOutOfSpec         = "out_of_spec"
	SpecBindingStateConflict          = "conflict"

	SpecBindingOperatorActionNone            = "none"
	SpecBindingOperatorActionChooseSection   = "choose_section"
	SpecBindingOperatorActionDraftSection    = "draft_section"
	SpecBindingOperatorActionRecordRationale = "record_rationale"
	SpecBindingOperatorActionReopenProblem   = "reopen_problem"

	SpecBindingRelationGoverns     = "governs"
	SpecBindingRelationContextOnly = "context_only"
	SpecBindingRelationConflict    = "conflict"
	SpecBindingRelationGapFrom     = "gap_from"

	SpecBindingDebtNone = "none"
	SpecBindingDebtLow  = "low"
	SpecBindingDebtHigh = "high"
)

type SpecBindingPreflightInput struct {
	DecisionDraft SpecBindingDecisionDraft `json:"decision_draft"`
	Now           time.Time                `json:"-"`
}

type SpecBindingDecisionDraft struct {
	SelectedTitle         string   `json:"selected_title,omitempty"`
	WhySelected           string   `json:"why_selected,omitempty"`
	CounterArgument       string   `json:"counterargument,omitempty"`
	WeakestLink           string   `json:"weakest_link,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
	LoadBearingLevel      string   `json:"load_bearing_level,omitempty"`
	DecisionSubjectRef    string   `json:"decision_subject_ref,omitempty"`
	ProblemRefs           []string `json:"problem_refs,omitempty"`
	PortfolioRef          string   `json:"portfolio_ref,omitempty"`
	ActiveDecisionRefs    []string `json:"active_decision_refs,omitempty"`
	SearchKeywords        string   `json:"search_keywords,omitempty"`
	BindingScope          string   `json:"binding_scope,omitempty"`
	BindingFallbackReason string   `json:"binding_fallback_reason,omitempty"`
	DeclaredRelation      string   `json:"declared_relation,omitempty"`
	SectionRefs           []string `json:"section_refs,omitempty"`
	LinkedSectionRefs     []string `json:"linked_section_refs,omitempty"`
	LineageSectionRefs    []string `json:"active_decision_lineage_section_refs,omitempty"`
	AffectedFiles         []string `json:"affected_files,omitempty"`
	BindingHints          []string `json:"binding_hints,omitempty"`
	BindingTargetRefs     []string `json:"binding_target_refs,omitempty"`
	GovernanceTargetRefs  []string `json:"governance_target_refs,omitempty"`
	ConflictRefs          []string `json:"conflict_refs,omitempty"`
}

type SpecBindingPreflightResult struct {
	SchemaVersion          int                           `json:"schema_version"`
	RecordKind             string                        `json:"record_kind"`
	Authority              string                        `json:"authority"`
	AuthorityBoundary      string                        `json:"authority_boundary"`
	DecisionDraftDigest    string                        `json:"decision_draft_digest"`
	ProjectSpecState       string                        `json:"project_spec_state"`
	DecisionMode           string                        `json:"decision_mode"`
	LoadBearingLevel       string                        `json:"load_bearing_level"`
	State                  string                        `json:"state"`
	SelectedSectionRefs    []string                      `json:"selected_section_refs"`
	CandidateSectionRefs   []SpecBindingSectionCandidate `json:"candidate_section_refs"`
	InvalidRefs            []SpecBindingInvalidRef       `json:"invalid_refs,omitempty"`
	ConflictRefs           []string                      `json:"conflict_refs"`
	MissingSectionProposal *SpecBindingSectionProposal   `json:"missing_section_proposal"`
	OperatorActionRequired string                        `json:"operator_action_required"`
	AllowedNextActions     []string                      `json:"allowed_next_actions"`
	BlockedNextActions     []string                      `json:"blocked_next_actions"`
	StatusDebt             SpecBindingStatusDebt         `json:"status_debt"`
}

type SpecBindingSectionCandidate struct {
	SectionRef string   `json:"section_ref"`
	Relation   string   `json:"relation"`
	Confidence string   `json:"confidence"`
	Basis      []string `json:"basis"`
}

type SpecBindingInvalidRef struct {
	SectionRef string `json:"section_ref"`
	Reason     string `json:"reason"`
}

type SpecBindingSectionProposal struct {
	Reason string `json:"reason"`
}

type SpecBindingStatusDebt struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func BuildSpecBindingPreflight(
	specSet project.ProjectSpecificationSet,
	input SpecBindingPreflightInput,
) SpecBindingPreflightResult {
	normalized := normalizeSpecBindingPreflightInput(input)
	result := newSpecBindingPreflightResult(specSet, normalized)
	activeSections := activeSpecBindingSections(specSet.Sections)
	sectionIndex := specBindingSectionIndex(specSet.Sections)

	result.InvalidRefs = invalidSpecBindingRefs(normalized.DecisionDraft.SectionRefs, sectionIndex)
	if len(result.InvalidRefs) > 0 {
		result.State = SpecBindingStateInvalidRefs
		result.OperatorActionRequired = SpecBindingOperatorActionChooseSection
		result.BlockedNextActions = []string{"create_spec_bound_decision"}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtHigh,
			Message:  "provided section_refs are invalid for the current ProjectSpecificationSet",
		}
		return result
	}

	if len(normalized.DecisionDraft.SectionRefs) > 0 {
		result.State = SpecBindingStateProvidedRefsValid
		result.SelectedSectionRefs = canonicalSpecBindingSectionIDs(normalized.DecisionDraft.SectionRefs)
		result.AllowedNextActions = []string{"create_decision"}
		return result
	}

	if result.ProjectSpecState == SpecBindingProjectStateNoSpecs {
		result.State = SpecBindingStateNoSpecs
		result.AllowedNextActions = []string{"create_decision"}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtLow,
			Message:  "project has no ProjectSpecificationSet; decision remains unbound to specs",
		}
		return result
	}

	if result.ProjectSpecState == SpecBindingProjectStateNoActiveSections {
		result.State = SpecBindingStateNoActiveSections
		result.AllowedNextActions = []string{"create_decision"}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtLow,
			Message:  "project has specs but no active SpecSections; decision remains unbound to active specs",
		}
		return result
	}

	if specBindingDeclaredConflict(normalized.DecisionDraft) {
		result.State = SpecBindingStateConflict
		result.ConflictRefs = canonicalSpecBindingSectionIDs(normalized.DecisionDraft.ConflictRefs)
		result.OperatorActionRequired = SpecBindingOperatorActionReopenProblem
		result.BlockedNextActions = []string{"create_standard_decision"}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtHigh,
			Message:  "decision draft declares a conflict with active spec; use a spec-changing path",
		}
		return result
	}

	if specBindingDeclaredOutOfSpec(normalized.DecisionDraft) {
		result.State = SpecBindingStateOutOfSpec
		result.OperatorActionRequired = SpecBindingOperatorActionRecordRationale
		result.AllowedNextActions = []string{"create_tactical_out_of_spec_decision"}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtHigh,
			Message:  "decision is explicitly out-of-spec; status/overseer should retain debt until resolved",
		}
		return result
	}

	result.CandidateSectionRefs = candidateSpecBindingSections(activeSections, normalized.DecisionDraft)
	highConfidence := highConfidenceSpecBindingCandidates(result.CandidateSectionRefs)
	switch len(highConfidence) {
	case 0:
		result.State = SpecBindingStateDraftNeeded
		result.OperatorActionRequired = SpecBindingOperatorActionDraftSection
		result.BlockedNextActions = []string{"create_spec_bound_decision"}
		result.MissingSectionProposal = &SpecBindingSectionProposal{
			Reason: "no active SpecSection matched the decision draft with high confidence",
		}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtHigh,
			Message:  "new draft SpecSection or spec delta is needed before claiming spec-bound coverage",
		}
	case 1:
		result.State = SpecBindingStateBoundExisting
		result.SelectedSectionRefs = []string{highConfidence[0].SectionRef}
		result.AllowedNextActions = []string{"create_decision"}
	default:
		result.State = SpecBindingStateAmbiguous
		result.OperatorActionRequired = SpecBindingOperatorActionChooseSection
		result.BlockedNextActions = []string{"create_standard_decision"}
		result.StatusDebt = SpecBindingStatusDebt{
			Severity: SpecBindingDebtHigh,
			Message:  "multiple plausible active SpecSections matched; operator choice is required",
		}
	}

	return result
}

func normalizeSpecBindingPreflightInput(input SpecBindingPreflightInput) SpecBindingPreflightInput {
	draft := input.DecisionDraft
	draft.SelectedTitle = strings.TrimSpace(draft.SelectedTitle)
	draft.WhySelected = strings.TrimSpace(draft.WhySelected)
	draft.CounterArgument = strings.TrimSpace(draft.CounterArgument)
	draft.WeakestLink = strings.TrimSpace(draft.WeakestLink)
	draft.Mode = strings.TrimSpace(draft.Mode)
	draft.LoadBearingLevel = strings.TrimSpace(draft.LoadBearingLevel)
	draft.DecisionSubjectRef = strings.TrimSpace(draft.DecisionSubjectRef)
	draft.ProblemRefs = trimUniqueSpecBindingStrings(draft.ProblemRefs)
	draft.PortfolioRef = strings.TrimSpace(draft.PortfolioRef)
	draft.ActiveDecisionRefs = trimUniqueSpecBindingStrings(draft.ActiveDecisionRefs)
	draft.SearchKeywords = strings.TrimSpace(draft.SearchKeywords)
	draft.BindingScope = strings.TrimSpace(draft.BindingScope)
	draft.BindingFallbackReason = strings.TrimSpace(draft.BindingFallbackReason)
	draft.DeclaredRelation = strings.TrimSpace(draft.DeclaredRelation)
	draft.SectionRefs = trimUniqueSpecBindingStrings(draft.SectionRefs)
	draft.LinkedSectionRefs = trimUniqueSpecBindingStrings(draft.LinkedSectionRefs)
	draft.LineageSectionRefs = trimUniqueSpecBindingStrings(draft.LineageSectionRefs)
	draft.AffectedFiles = trimUniqueSpecBindingStrings(draft.AffectedFiles)
	draft.BindingHints = trimUniqueSpecBindingStrings(draft.BindingHints)
	draft.BindingTargetRefs = trimUniqueSpecBindingStrings(draft.BindingTargetRefs)
	draft.GovernanceTargetRefs = trimUniqueSpecBindingStrings(draft.GovernanceTargetRefs)
	draft.ConflictRefs = trimUniqueSpecBindingStrings(draft.ConflictRefs)
	input.DecisionDraft = draft
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	return input
}

func newSpecBindingPreflightResult(
	specSet project.ProjectSpecificationSet,
	input SpecBindingPreflightInput,
) SpecBindingPreflightResult {
	mode := strings.TrimSpace(input.DecisionDraft.Mode)
	if mode == "" {
		mode = SpecBindingDecisionModeStandard
	}
	loadBearing := strings.TrimSpace(input.DecisionDraft.LoadBearingLevel)
	if loadBearing == "" {
		loadBearing = defaultSpecBindingLoadBearing(input.DecisionDraft)
	}
	return SpecBindingPreflightResult{
		SchemaVersion:          SpecBindingPreflightSchemaVersion,
		RecordKind:             SpecBindingPreflightRecordKind,
		Authority:              SpecBindingPreflightAuthority,
		AuthorityBoundary:      SpecBindingPreflightBoundary,
		DecisionDraftDigest:    specBindingDecisionDraftDigest(input.DecisionDraft),
		ProjectSpecState:       specBindingProjectState(specSet),
		DecisionMode:           mode,
		LoadBearingLevel:       loadBearing,
		State:                  SpecBindingStateDraftNeeded,
		SelectedSectionRefs:    []string{},
		CandidateSectionRefs:   []SpecBindingSectionCandidate{},
		ConflictRefs:           []string{},
		MissingSectionProposal: nil,
		OperatorActionRequired: SpecBindingOperatorActionNone,
		AllowedNextActions:     []string{},
		BlockedNextActions:     []string{},
		StatusDebt:             SpecBindingStatusDebt{Severity: SpecBindingDebtNone},
	}
}

func specBindingProjectState(specSet project.ProjectSpecificationSet) string {
	if len(specSet.Documents) == 0 && len(specSet.Sections) == 0 && len(specSet.TermMapEntries) == 0 {
		return SpecBindingProjectStateNoSpecs
	}
	if len(activeSpecBindingSections(specSet.Sections)) == 0 {
		return SpecBindingProjectStateNoActiveSections
	}
	if len(specSet.Findings) > 0 {
		return SpecBindingProjectStatePartial
	}
	return SpecBindingProjectStateReady
}

func activeSpecBindingSections(sections []project.SpecSection) []project.SpecSection {
	out := []project.SpecSection{}
	for _, section := range sections {
		if strings.TrimSpace(section.Status) != string(project.SpecSectionStateActive) {
			continue
		}
		out = append(out, section)
	}
	return out
}

func specBindingSectionIndex(sections []project.SpecSection) map[string]project.SpecSection {
	index := map[string]project.SpecSection{}
	for _, section := range sections {
		id := canonicalSpecBindingSectionID(section.ID)
		if id == "" {
			continue
		}
		index[id] = section
	}
	return index
}

func invalidSpecBindingRefs(
	refs []string,
	sections map[string]project.SpecSection,
) []SpecBindingInvalidRef {
	invalid := []SpecBindingInvalidRef{}
	for _, ref := range refs {
		id := canonicalSpecBindingSectionID(ref)
		section, ok := sections[id]
		switch {
		case !ok:
			invalid = append(invalid, SpecBindingInvalidRef{SectionRef: ref, Reason: "unknown_section_ref"})
		case strings.TrimSpace(section.Status) == string(project.SpecSectionStateDraft):
			invalid = append(invalid, SpecBindingInvalidRef{SectionRef: ref, Reason: "draft_section_ref_used_as_active_binding"})
		case strings.TrimSpace(section.Status) != string(project.SpecSectionStateActive):
			invalid = append(invalid, SpecBindingInvalidRef{SectionRef: ref, Reason: "inactive_section_ref"})
		}
	}
	return invalid
}

func candidateSpecBindingSections(
	sections []project.SpecSection,
	draft SpecBindingDecisionDraft,
) []SpecBindingSectionCandidate {
	candidates := []SpecBindingSectionCandidate{}
	haystack := specBindingDraftSearchHaystack(draft)
	targetRefs := specBindingDraftTargetRefs(draft)
	linkedRefs := specBindingSectionRefSet(draft.LinkedSectionRefs)
	lineageRefs := specBindingSectionRefSet(draft.LineageSectionRefs)
	for _, section := range sections {
		score, basis := scoreSpecBindingSectionCandidate(section, haystack, targetRefs, linkedRefs, lineageRefs)
		if score == 0 {
			continue
		}
		candidates = append(candidates, SpecBindingSectionCandidate{
			SectionRef: strings.TrimSpace(section.ID),
			Relation:   specBindingCandidateRelation(score),
			Confidence: specBindingCandidateConfidence(score),
			Basis:      basis,
		})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.Confidence != right.Confidence {
			return specBindingConfidenceRank(left.Confidence) > specBindingConfidenceRank(right.Confidence)
		}
		return left.SectionRef < right.SectionRef
	})
	return candidates
}

func scoreSpecBindingSectionCandidate(
	section project.SpecSection,
	haystack string,
	targetRefs map[string]bool,
	linkedRefs map[string]bool,
	lineageRefs map[string]bool,
) (int, []string) {
	score := 0
	basis := []string{}
	id := strings.ToLower(strings.TrimSpace(section.ID))
	canonicalID := canonicalSpecBindingSectionID(section.ID)
	if canonicalID != "" && linkedRefs[canonicalID] {
		score += 95
		basis = append(basis, "matched problem/portfolio linked decision section_refs")
	}
	if canonicalID != "" && lineageRefs[canonicalID] {
		score += 95
		basis = append(basis, "matched active decision lineage section_refs")
	}
	if id != "" && strings.Contains(haystack, id) {
		score += 100
		basis = append(basis, "matched section id")
	}
	if targetRefMatches(section.TargetRefs, targetRefs) {
		score += 90
		basis = append(basis, "matched target_refs")
	}
	matchedTerms := matchingSpecBindingTerms(section.Terms, haystack)
	if len(matchedTerms) > 0 {
		score += 35 * len(matchedTerms)
		basis = append(basis, "matched terms: "+strings.Join(matchedTerms, ", "))
	}
	titleTokens := specBindingNeedleTokens(section.Title)
	if len(titleTokens) > 0 && allSpecBindingTokensPresent(titleTokens, haystack) {
		score += 45
		basis = append(basis, "matched section title")
	}
	kindTokens := specBindingNeedleTokens(section.Kind)
	if len(kindTokens) > 0 && allSpecBindingTokensPresent(kindTokens, haystack) {
		score += 30
		basis = append(basis, "matched section kind")
	}
	return score, basis
}

func targetRefMatches(sectionRefs []string, draftRefs map[string]bool) bool {
	for _, ref := range sectionRefs {
		trimmed := strings.ToLower(strings.TrimSpace(ref))
		if trimmed == "" {
			continue
		}
		if draftRefs[trimmed] {
			return true
		}
	}
	return false
}

func matchingSpecBindingTerms(terms []string, haystack string) []string {
	matched := []string{}
	for _, term := range terms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" || len(normalized) < 4 {
			continue
		}
		if strings.Contains(haystack, normalized) {
			matched = append(matched, strings.TrimSpace(term))
		}
	}
	sort.Strings(matched)
	return matched
}

func specBindingDraftSearchHaystack(draft SpecBindingDecisionDraft) string {
	values := []string{
		draft.SelectedTitle,
		draft.WhySelected,
		draft.CounterArgument,
		draft.WeakestLink,
		draft.DecisionSubjectRef,
		strings.Join(draft.ProblemRefs, "\n"),
		draft.PortfolioRef,
		strings.Join(draft.ActiveDecisionRefs, "\n"),
		draft.SearchKeywords,
		draft.BindingScope,
		draft.BindingFallbackReason,
		draft.DeclaredRelation,
	}
	values = append(values, draft.AffectedFiles...)
	values = append(values, draft.BindingHints...)
	values = append(values, draft.LinkedSectionRefs...)
	values = append(values, draft.LineageSectionRefs...)
	values = append(values, draft.BindingTargetRefs...)
	values = append(values, draft.GovernanceTargetRefs...)
	values = append(values, draft.ConflictRefs...)
	return strings.ToLower(strings.Join(values, "\n"))
}

func specBindingDraftTargetRefs(draft SpecBindingDecisionDraft) map[string]bool {
	refs := map[string]bool{}
	values := []string{draft.DecisionSubjectRef, draft.PortfolioRef}
	values = append(values, draft.ProblemRefs...)
	values = append(values, draft.ActiveDecisionRefs...)
	values = append(values, draft.AffectedFiles...)
	values = append(values, draft.BindingTargetRefs...)
	values = append(values, draft.GovernanceTargetRefs...)
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		refs[trimmed] = true
	}
	return refs
}

func specBindingSectionRefSet(refs []string) map[string]bool {
	out := map[string]bool{}
	for _, ref := range refs {
		id := canonicalSpecBindingSectionID(ref)
		if id == "" {
			continue
		}
		out[id] = true
	}
	return out
}

func specBindingNeedleTokens(value string) []string {
	tokens := []string{}
	for _, token := range strings.FieldsFunc(strings.ToLower(value), specBindingTokenSeparator) {
		trimmed := strings.TrimSpace(token)
		if len(trimmed) < 4 {
			continue
		}
		tokens = append(tokens, trimmed)
	}
	return trimUniqueSpecBindingStrings(tokens)
}

func specBindingTokenSeparator(r rune) bool {
	return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
}

func allSpecBindingTokensPresent(tokens []string, haystack string) bool {
	for _, token := range tokens {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return len(tokens) > 0
}

func specBindingCandidateRelation(score int) string {
	if score >= 90 {
		return SpecBindingRelationGoverns
	}
	return SpecBindingRelationContextOnly
}

func specBindingCandidateConfidence(score int) string {
	switch {
	case score >= 90:
		return "high"
	case score >= 60:
		return "medium"
	default:
		return "low"
	}
}

func specBindingConfidenceRank(confidence string) int {
	switch confidence {
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

func highConfidenceSpecBindingCandidates(
	candidates []SpecBindingSectionCandidate,
) []SpecBindingSectionCandidate {
	out := []SpecBindingSectionCandidate{}
	for _, candidate := range candidates {
		if candidate.Confidence == "high" {
			out = append(out, candidate)
		}
	}
	return out
}

func defaultSpecBindingLoadBearing(draft SpecBindingDecisionDraft) string {
	mode := strings.TrimSpace(draft.Mode)
	switch mode {
	case SpecBindingDecisionModeTactical:
		return SpecBindingLoadBearingLow
	case SpecBindingDecisionModeDeep:
		return SpecBindingLoadBearingLoadBearing
	default:
		return SpecBindingLoadBearingNormal
	}
}

func specBindingDeclaredConflict(draft SpecBindingDecisionDraft) bool {
	return strings.TrimSpace(draft.DeclaredRelation) == SpecBindingStateConflict ||
		strings.TrimSpace(draft.DeclaredRelation) == SpecBindingRelationConflict ||
		len(draft.ConflictRefs) > 0
}

func specBindingDeclaredOutOfSpec(draft SpecBindingDecisionDraft) bool {
	relation := strings.TrimSpace(draft.DeclaredRelation)
	if relation == SpecBindingStateOutOfSpec {
		return true
	}
	return strings.Contains(strings.ToLower(draft.BindingFallbackReason), "out-of-spec") ||
		strings.Contains(strings.ToLower(draft.BindingFallbackReason), "out of spec")
}

func specBindingDecisionDraftDigest(draft SpecBindingDecisionDraft) string {
	data, err := json.Marshal(draft)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func canonicalSpecBindingSectionIDs(refs []string) []string {
	out := []string{}
	for _, ref := range refs {
		id := canonicalSpecBindingSectionID(ref)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return trimUniqueSpecBindingStrings(out)
}

func canonicalSpecBindingSectionID(ref string) string {
	trimmed := strings.TrimSpace(ref)
	trimmed = strings.TrimPrefix(trimmed, "spec_section:")
	trimmed = strings.TrimPrefix(trimmed, "spec-section:")
	return strings.TrimSpace(trimmed)
}

func trimUniqueSpecBindingStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
