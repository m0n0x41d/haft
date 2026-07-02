package specflow

import (
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	SpecFitProbeSchemaVersion = 1
	SpecFitProbeRecordKind    = "spec_fit_probe"
	SpecFitProbeAuthority     = "read_only_spec_fit_probe"
	SpecFitProbeBoundary      = "spec_fit_probe_is_advisory_not_approval_not_baseline_not_evidence_not_gate_decision_not_claim_truth_not_publication"

	SpecFitStateRelatesExisting    = "relates_existing"
	SpecFitStateSpecGap            = "spec_gap"
	SpecFitStateConflict           = "conflict"
	SpecFitStateOutsideCurrentSpec = "outside_current_spec"
	SpecFitStateNoSignal           = "no_signal"

	SpecFitNextOrdinaryExplore  = "ordinary_explore"
	SpecFitNextDraftSection     = "draft_section"
	SpecFitNextExploreSpecDelta = "explore_spec_delta"
)

type SpecFitProbeInput struct {
	ProblemSignal    string                `json:"problem_signal,omitempty"`
	Scope            string                `json:"scope,omitempty"`
	Mode             string                `json:"mode,omitempty"`
	SectionRefs      []string              `json:"section_refs,omitempty"`
	AffectedFiles    []string              `json:"affected_files,omitempty"`
	TargetRefs       []string              `json:"target_refs,omitempty"`
	ConflictRefs     []string              `json:"conflict_refs,omitempty"`
	DeclaredRelation string                `json:"declared_relation,omitempty"`
	Variants         []SpecFitVariantInput `json:"variants,omitempty"`
}

type SpecFitVariantInput struct {
	ID               string   `json:"id,omitempty"`
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	SectionRefs      []string `json:"section_refs,omitempty"`
	AffectedFiles    []string `json:"affected_files,omitempty"`
	TargetRefs       []string `json:"target_refs,omitempty"`
	ConflictRefs     []string `json:"conflict_refs,omitempty"`
	DeclaredRelation string   `json:"declared_relation,omitempty"`
}

type SpecFitProbeResult struct {
	SchemaVersion        int                    `json:"schema_version"`
	RecordKind           string                 `json:"record_kind"`
	Authority            string                 `json:"authority"`
	AuthorityBoundary    string                 `json:"authority_boundary"`
	State                string                 `json:"state"`
	CandidateSectionRefs []string               `json:"candidate_section_refs"`
	ConflictRefs         []string               `json:"conflict_refs"`
	NextExpectedAction   string                 `json:"next_expected_action"`
	VariantSpecFit       []SpecFitVariantResult `json:"variant_spec_fit,omitempty"`
}

type SpecFitVariantResult struct {
	VariantRef     string   `json:"variant_ref,omitempty"`
	State          string   `json:"state"`
	SectionRefs    []string `json:"section_refs,omitempty"`
	ConflictRefs   []string `json:"conflict_refs,omitempty"`
	ProposedDelta  string   `json:"proposed_delta,omitempty"`
	ExpectedAction string   `json:"expected_action"`
}

func BuildSpecFitProbe(specSet project.ProjectSpecificationSet, input SpecFitProbeInput) SpecFitProbeResult {
	input = normalizeSpecFitProbeInput(input)
	variantResults := specFitVariantResults(specSet, input)
	state := aggregateSpecFitState(variantResults)

	return SpecFitProbeResult{
		SchemaVersion:        SpecFitProbeSchemaVersion,
		RecordKind:           SpecFitProbeRecordKind,
		Authority:            SpecFitProbeAuthority,
		AuthorityBoundary:    SpecFitProbeBoundary,
		State:                state,
		CandidateSectionRefs: specFitCandidateSectionRefs(variantResults),
		ConflictRefs:         specFitConflictRefs(variantResults),
		NextExpectedAction:   specFitNextAction(state),
		VariantSpecFit:       variantResults,
	}
}

func normalizeSpecFitProbeInput(input SpecFitProbeInput) SpecFitProbeInput {
	input.ProblemSignal = strings.TrimSpace(input.ProblemSignal)
	input.Scope = strings.TrimSpace(input.Scope)
	input.Mode = strings.TrimSpace(input.Mode)
	input.SectionRefs = compactStrings(input.SectionRefs)
	input.AffectedFiles = compactStrings(input.AffectedFiles)
	input.TargetRefs = compactStrings(input.TargetRefs)
	input.ConflictRefs = compactStrings(input.ConflictRefs)
	input.DeclaredRelation = strings.TrimSpace(input.DeclaredRelation)
	for i, variant := range input.Variants {
		variant.ID = strings.TrimSpace(variant.ID)
		variant.Title = strings.TrimSpace(variant.Title)
		variant.Description = strings.TrimSpace(variant.Description)
		variant.SectionRefs = compactStrings(variant.SectionRefs)
		variant.AffectedFiles = compactStrings(variant.AffectedFiles)
		variant.TargetRefs = compactStrings(variant.TargetRefs)
		variant.ConflictRefs = compactStrings(variant.ConflictRefs)
		variant.DeclaredRelation = strings.TrimSpace(variant.DeclaredRelation)
		input.Variants[i] = variant
	}
	return input
}

func specFitVariantResults(
	specSet project.ProjectSpecificationSet,
	input SpecFitProbeInput,
) []SpecFitVariantResult {
	variants := input.Variants
	if len(variants) == 0 {
		variants = []SpecFitVariantInput{{
			ID:               "probe",
			Title:            input.ProblemSignal,
			Description:      input.Scope,
			SectionRefs:      input.SectionRefs,
			AffectedFiles:    input.AffectedFiles,
			TargetRefs:       input.TargetRefs,
			ConflictRefs:     input.ConflictRefs,
			DeclaredRelation: input.DeclaredRelation,
		}}
	}

	results := make([]SpecFitVariantResult, 0, len(variants))
	for _, variant := range variants {
		preflight := BuildSpecBindingPreflight(specSet, SpecBindingPreflightInput{
			DecisionDraft: specflowDraftFromSpecFitVariant(input, variant),
		})
		state := specFitStateFromPreflight(preflight.State)
		result := SpecFitVariantResult{
			VariantRef:     specFitVariantRef(variant),
			State:          state,
			SectionRefs:    preflight.SelectedSectionRefs,
			ConflictRefs:   preflight.ConflictRefs,
			ExpectedAction: specFitNextAction(state),
		}
		if preflight.MissingSectionProposal != nil {
			result.ProposedDelta = preflight.MissingSectionProposal.Reason
		}
		results = append(results, result)
	}
	return results
}

func specflowDraftFromSpecFitVariant(
	input SpecFitProbeInput,
	variant SpecFitVariantInput,
) SpecBindingDecisionDraft {
	return SpecBindingDecisionDraft{
		SelectedTitle:        firstNonEmpty(variant.Title, input.ProblemSignal),
		WhySelected:          firstNonEmpty(variant.Description, input.Scope),
		Mode:                 input.Mode,
		SectionRefs:          append(compactStrings(input.SectionRefs), variant.SectionRefs...),
		AffectedFiles:        append(compactStrings(input.AffectedFiles), variant.AffectedFiles...),
		GovernanceTargetRefs: append(compactStrings(input.TargetRefs), variant.TargetRefs...),
		ConflictRefs:         append(compactStrings(input.ConflictRefs), variant.ConflictRefs...),
		DeclaredRelation:     firstNonEmpty(variant.DeclaredRelation, input.DeclaredRelation),
	}
}

func specFitVariantRef(variant SpecFitVariantInput) string {
	if variant.ID != "" {
		return variant.ID
	}
	return variant.Title
}

func specFitStateFromPreflight(state string) string {
	switch state {
	case SpecBindingStateProvidedRefsValid, SpecBindingStateBoundExisting:
		return SpecFitStateRelatesExisting
	case SpecBindingStateDraftNeeded, SpecBindingStateAmbiguous:
		return SpecFitStateSpecGap
	case SpecBindingStateConflict, SpecBindingStateInvalidRefs:
		return SpecFitStateConflict
	case SpecBindingStateOutOfSpec:
		return SpecFitStateOutsideCurrentSpec
	default:
		return SpecFitStateNoSignal
	}
}

func aggregateSpecFitState(results []SpecFitVariantResult) string {
	state := SpecFitStateNoSignal
	for _, result := range results {
		switch result.State {
		case SpecFitStateConflict:
			return SpecFitStateConflict
		case SpecFitStateOutsideCurrentSpec:
			state = SpecFitStateOutsideCurrentSpec
		case SpecFitStateSpecGap:
			if state != SpecFitStateOutsideCurrentSpec {
				state = SpecFitStateSpecGap
			}
		case SpecFitStateRelatesExisting:
			if state == SpecFitStateNoSignal {
				state = SpecFitStateRelatesExisting
			}
		}
	}
	return state
}

func specFitNextAction(state string) string {
	switch state {
	case SpecFitStateSpecGap:
		return SpecFitNextDraftSection
	case SpecFitStateConflict, SpecFitStateOutsideCurrentSpec:
		return SpecFitNextExploreSpecDelta
	default:
		return SpecFitNextOrdinaryExplore
	}
}

func specFitCandidateSectionRefs(results []SpecFitVariantResult) []string {
	refs := []string{}
	for _, result := range results {
		refs = append(refs, result.SectionRefs...)
	}
	return sortedUniqueStrings(refs)
}

func specFitConflictRefs(results []SpecFitVariantResult) []string {
	refs := []string{}
	for _, result := range results {
		refs = append(refs, result.ConflictRefs...)
	}
	return sortedUniqueStrings(refs)
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range compactStrings(values) {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func compactStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
