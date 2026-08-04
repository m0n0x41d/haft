package specflow

import (
	"fmt"
	"maps"
	"slices"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	PhaseApplicabilityUnderdeterminedCode = "profile_capability_applicability_underdetermined"
	PhaseApplicabilityRecoveryCode        = "project_profile_recovery_required"
)

// PhaseApplicabilityIssue preserves one capability-local missing basis. It is
// orientation, not a lifecycle gate, waiver, or instruction to create a
// carrier.
type PhaseApplicabilityIssue struct {
	DocumentKind project.SpecDocumentKind  `json:"document_kind"`
	Capability   projectprofile.Capability `json:"capability"`
	MissingBasis string                    `json:"missing_basis"`
}

// PhaseApplicabilityDependencyIssue preserves one semantic prerequisite whose
// phase is unavailable because its specification member is underdetermined.
// It blocks only BlockedPhase; it neither makes the prerequisite applicable nor
// turns the cue into a human gate.
type PhaseApplicabilityDependencyIssue struct {
	BlockedPhase         PhaseID                   `json:"blocked_phase"`
	RequiredPhase        PhaseID                   `json:"required_phase"`
	RequiredDocumentKind project.SpecDocumentKind  `json:"required_document_kind"`
	RequiredCapability   projectprofile.Capability `json:"required_capability"`
	RequiredMissingBasis string                    `json:"required_missing_basis"`
}

// PhaseApplicabilityCue is the single neutral result emitted after all
// currently runnable required phases are satisfied while another specification
// member or one of its semantic prerequisites remains underdetermined.
type PhaseApplicabilityCue struct {
	Code                 string                              `json:"code"`
	ScopeID              string                              `json:"scope_id"`
	ProfilePayloadDigest string                              `json:"profile_payload_digest"`
	Issues               []PhaseApplicabilityIssue           `json:"issues"`
	BlockedDependencies  []PhaseApplicabilityDependencyIssue `json:"blocked_dependencies,omitempty"`
	Recovery             PhaseApplicabilityRecovery          `json:"recovery"`
}

// PhaseApplicabilityRecovery makes the public contract boundary explicit.
// It does not fabricate a mutation call before the public surface resolves
// the canonical profile origin. SafeContinuationCalls expose the work that
// remains valid without that effect through public Haft surfaces.
type PhaseApplicabilityRecovery struct {
	ResultKind            string          `json:"result_kind"`
	Code                  string          `json:"code"`
	RequiredEffect        string          `json:"required_effect"`
	SubjectRef            string          `json:"subject_ref"`
	ScopeID               string          `json:"scope_id"`
	ProfileOrigin         string          `json:"profile_origin,omitempty"`
	RecoverySurface       string          `json:"recovery_surface"`
	MutationAvailability  string          `json:"mutation_availability"`
	HumanGateRequired     bool            `json:"human_gate_required"`
	Why                   string          `json:"why"`
	SafeContinuationCalls []DraftToolCall `json:"safe_continuation_calls"`
	ReturnCondition       string          `json:"return_condition"`
}

// ApplicablePhaseSet is the immutable phase projection for one exact
// scope-local capability matrix. It is not a project workflow: it only limits
// the local specflow MethodDescription to specification members that are
// Required for the selected scope.
type ApplicablePhaseSet struct {
	phases []Phase
	cue    PhaseApplicabilityCue
}

func (set ApplicablePhaseSet) Valid() bool {
	return validateApplicablePhaseSet(set) == nil
}

func (set ApplicablePhaseSet) Phases() []Phase {
	if !set.Valid() {
		return nil
	}
	return clonePhases(set.phases)
}

func (set ApplicablePhaseSet) ApplicabilityCue() (
	PhaseApplicabilityCue,
	bool,
) {
	if !set.Valid() || len(set.cue.Issues) == 0 {
		return PhaseApplicabilityCue{}, false
	}
	return clonePhaseApplicabilityCue(set.cue), true
}

func (set ApplicablePhaseSet) phaseBlockedByApplicability(id PhaseID) bool {
	if !set.Valid() {
		return false
	}
	return phaseBlockedByApplicability(
		id,
		set.phases,
		set.cue.BlockedDependencies,
		make(map[PhaseID]bool),
	)
}

// DeriveApplicablePhaseSet projects the central capability matrix once. A
// NotApplicable document contributes no phase. An Underdetermined document
// contributes one neutral cue, never a fake drafting phase.
func DeriveApplicablePhaseSet(
	applicability project.ProjectSpecificationSetApplicability,
) (ApplicablePhaseSet, error) {
	if !applicability.Valid() {
		return ApplicablePhaseSet{}, fmt.Errorf(
			"project specification applicability is invalid",
		)
	}

	requiredKinds := applicability.ApplicableDocumentKinds()
	phases := slices.DeleteFunc(
		PhaseRegistry(),
		func(phase Phase) bool {
			return !slices.Contains(requiredKinds, phase.DocumentKind)
		},
	)

	cue, err := phaseApplicabilityCue(applicability)
	if err != nil {
		return ApplicablePhaseSet{}, err
	}
	cue, err = addUnavailableDependencyIssues(phases, cue)
	if err != nil {
		return ApplicablePhaseSet{}, err
	}
	set := ApplicablePhaseSet{
		phases: phases,
		cue:    cue,
	}
	if err := validateApplicablePhaseSet(set); err != nil {
		return ApplicablePhaseSet{}, err
	}
	return set, nil
}

func phaseApplicabilityCue(
	applicability project.ProjectSpecificationSetApplicability,
) (PhaseApplicabilityCue, error) {
	members := applicability.Members()
	issues := make([]PhaseApplicabilityIssue, 0)
	for _, member := range members {
		if member.Kind() != projectprofile.CapabilityUnderdetermined {
			continue
		}
		missingBasis, found := member.MissingBasis()
		if !found {
			return PhaseApplicabilityCue{}, fmt.Errorf(
				"underdetermined %q applicability has no missing basis",
				member.DocumentKind(),
			)
		}
		issues = append(issues, PhaseApplicabilityIssue{
			DocumentKind: member.DocumentKind(),
			Capability:   member.Capability(),
			MissingBasis: string(missingBasis),
		})
	}
	recovery := phaseApplicabilityRecovery(applicability)
	return PhaseApplicabilityCue{
		Code:                 PhaseApplicabilityUnderdeterminedCode,
		ScopeID:              applicability.ScopeID().String(),
		ProfilePayloadDigest: applicability.ProfilePayloadDigest().String(),
		Issues:               issues,
		Recovery:             recovery,
	}, nil
}

func phaseApplicabilityRecovery(
	applicability project.ProjectSpecificationSetApplicability,
) PhaseApplicabilityRecovery {
	scopeID := applicability.ScopeID().String()
	payloadDigest := applicability.ProfilePayloadDigest().String()
	contractCall := DraftToolCall{
		Tool: "haft_spec_section",
		Arguments: map[string]interface{}{
			"action": "draft_contract",
		},
	}
	validationCall := DraftToolCall{
		Tool: "haft_query",
		Arguments: map[string]interface{}{
			"action": "spec_validate",
		},
	}
	return PhaseApplicabilityRecovery{
		ResultKind:           "blocked",
		Code:                 PhaseApplicabilityRecoveryCode,
		RequiredEffect:       "resolve_missing_project_profile_relation",
		SubjectRef:           payloadDigest,
		ScopeID:              scopeID,
		RecoverySurface:      "haft_onboard",
		MutationAvailability: "depends_on_current_profile_origin",
		HumanGateRequired:    true,
		Why:                  "The lifecycle surface must resolve the current canonical profile origin before naming an available mutation path. Follow the origin-specific public recovery route returned by the surface.",
		SafeContinuationCalls: []DraftToolCall{
			contractCall,
			validationCall,
		},
		ReturnCondition: "The missing relation is admitted through the origin-specific public profile route; then retry the unchanged lifecycle request.",
	}
}

func addUnavailableDependencyIssues(
	phases []Phase,
	cue PhaseApplicabilityCue,
) (PhaseApplicabilityCue, error) {
	selected := make(map[PhaseID]struct{}, len(phases))
	for _, phase := range phases {
		selected[phase.ID] = struct{}{}
	}
	result := clonePhaseApplicabilityCue(cue)
	for _, phase := range phases {
		for _, dependency := range phase.DependsOn {
			if _, found := selected[dependency]; found {
				continue
			}
			dependencyPhase, found := FindPhase(dependency)
			if !found {
				return PhaseApplicabilityCue{}, fmt.Errorf(
					"applicable phase %q has unknown dependency %q",
					phase.ID,
					dependency,
				)
			}
			issue, found := applicabilityIssueForDocumentKind(
				cue.Issues,
				dependencyPhase.DocumentKind,
			)
			if !found {
				return PhaseApplicabilityCue{}, fmt.Errorf(
					"applicable phase %q requires unavailable phase %q without underdetermined applicability basis",
					phase.ID,
					dependency,
				)
			}
			result.BlockedDependencies = append(
				result.BlockedDependencies,
				PhaseApplicabilityDependencyIssue{
					BlockedPhase:         phase.ID,
					RequiredPhase:        dependency,
					RequiredDocumentKind: issue.DocumentKind,
					RequiredCapability:   issue.Capability,
					RequiredMissingBasis: issue.MissingBasis,
				},
			)
		}
	}
	return result, nil
}

func applicabilityIssueForDocumentKind(
	issues []PhaseApplicabilityIssue,
	documentKind project.SpecDocumentKind,
) (PhaseApplicabilityIssue, bool) {
	for _, issue := range issues {
		if issue.DocumentKind == documentKind {
			return issue, true
		}
	}
	return PhaseApplicabilityIssue{}, false
}

func clonePhases(phases []Phase) []Phase {
	result := append([]Phase{}, phases...)
	for index := range result {
		result[index].DependsOn = append(
			[]PhaseID{},
			result[index].DependsOn...,
		)
		result[index].ExpectedFields = append(
			[]string{},
			result[index].ExpectedFields...,
		)
		result[index].Checks = append(
			[]Check{},
			result[index].Checks...,
		)
	}
	return result
}

func clonePhaseApplicabilityCue(
	cue PhaseApplicabilityCue,
) PhaseApplicabilityCue {
	cue.Issues = append([]PhaseApplicabilityIssue{}, cue.Issues...)
	cue.BlockedDependencies = append(
		[]PhaseApplicabilityDependencyIssue{},
		cue.BlockedDependencies...,
	)
	cue.Recovery.SafeContinuationCalls = cloneDraftToolCalls(
		cue.Recovery.SafeContinuationCalls,
	)
	return cue
}

func cloneDraftToolCalls(values []DraftToolCall) []DraftToolCall {
	result := make([]DraftToolCall, len(values))
	fillClonedDraftToolCalls(values, result, 0)
	return result
}

func fillClonedDraftToolCalls(
	values []DraftToolCall,
	result []DraftToolCall,
	index int,
) {
	if index == len(values) {
		return
	}
	arguments := maps.Clone(values[index].Arguments)
	result[index] = DraftToolCall{
		Tool:      values[index].Tool,
		Arguments: arguments,
	}
	fillClonedDraftToolCalls(values, result, index+1)
}

func validateApplicablePhaseSet(set ApplicablePhaseSet) error {
	if set.cue.Code != PhaseApplicabilityUnderdeterminedCode {
		return fmt.Errorf("phase applicability cue code is invalid")
	}
	if set.cue.ScopeID == "" || set.cue.ProfilePayloadDigest == "" {
		return fmt.Errorf("phase applicability provenance is incomplete")
	}
	if err := validatePhaseApplicabilityRecovery(set.cue); err != nil {
		return err
	}
	seen := make(map[PhaseID]struct{}, len(set.phases))
	for _, phase := range set.phases {
		if phase.ID == "" {
			return fmt.Errorf("applicable phase has no identifier")
		}
		if _, found := seen[phase.ID]; found {
			return fmt.Errorf("applicable phase %q is duplicated", phase.ID)
		}
		seen[phase.ID] = struct{}{}
	}
	for _, issue := range set.cue.Issues {
		if issue.DocumentKind == "" ||
			issue.Capability == "" ||
			issue.MissingBasis == "" {
			return fmt.Errorf("phase applicability issue is incomplete")
		}
	}
	for _, dependencyIssue := range set.cue.BlockedDependencies {
		if err := validatePhaseApplicabilityDependencyIssue(
			dependencyIssue,
			set,
			seen,
		); err != nil {
			return err
		}
	}
	for _, phase := range set.phases {
		for _, dependency := range phase.DependsOn {
			if _, found := seen[dependency]; found {
				continue
			}
			if !hasBlockedDependencyIssue(
				set.cue.BlockedDependencies,
				phase.ID,
				dependency,
			) {
				return fmt.Errorf(
					"applicable phase %q retains unexplained unavailable dependency %q",
					phase.ID,
					dependency,
				)
			}
		}
	}
	return nil
}

func validatePhaseApplicabilityRecovery(cue PhaseApplicabilityCue) error {
	recovery := cue.Recovery
	if recovery.ResultKind != "blocked" ||
		recovery.Code != PhaseApplicabilityRecoveryCode ||
		recovery.RequiredEffect == "" ||
		recovery.SubjectRef != cue.ProfilePayloadDigest ||
		recovery.ScopeID != cue.ScopeID ||
		recovery.RecoverySurface == "" ||
		recovery.MutationAvailability == "" ||
		!recovery.HumanGateRequired ||
		recovery.Why == "" ||
		len(recovery.SafeContinuationCalls) == 0 ||
		recovery.ReturnCondition == "" {
		return fmt.Errorf("phase applicability recovery contract is incomplete")
	}
	return nil
}

func validatePhaseApplicabilityDependencyIssue(
	issue PhaseApplicabilityDependencyIssue,
	set ApplicablePhaseSet,
	selected map[PhaseID]struct{},
) error {
	if issue.BlockedPhase == "" ||
		issue.RequiredPhase == "" ||
		issue.RequiredDocumentKind == "" ||
		issue.RequiredCapability == "" ||
		issue.RequiredMissingBasis == "" {
		return fmt.Errorf("phase applicability dependency issue is incomplete")
	}
	if _, found := selected[issue.BlockedPhase]; !found {
		return fmt.Errorf(
			"phase applicability dependency issue references unselected blocked phase %q",
			issue.BlockedPhase,
		)
	}
	if _, found := selected[issue.RequiredPhase]; found {
		return fmt.Errorf(
			"phase applicability dependency issue marks selected phase %q unavailable",
			issue.RequiredPhase,
		)
	}
	blockedPhase, found := phaseByID(set.phases, issue.BlockedPhase)
	if !found || !slices.Contains(blockedPhase.DependsOn, issue.RequiredPhase) {
		return fmt.Errorf(
			"phase applicability dependency issue is not declared by blocked phase %q",
			issue.BlockedPhase,
		)
	}
	requiredPhase, found := FindPhase(issue.RequiredPhase)
	if !found || requiredPhase.DocumentKind != issue.RequiredDocumentKind {
		return fmt.Errorf(
			"phase applicability dependency issue has inconsistent required phase %q",
			issue.RequiredPhase,
		)
	}
	applicabilityIssue, found := applicabilityIssueForDocumentKind(
		set.cue.Issues,
		issue.RequiredDocumentKind,
	)
	if !found ||
		applicabilityIssue.Capability != issue.RequiredCapability ||
		applicabilityIssue.MissingBasis != issue.RequiredMissingBasis {
		return fmt.Errorf(
			"phase applicability dependency issue has no matching applicability issue",
		)
	}
	return nil
}

func phaseByID(phases []Phase, id PhaseID) (Phase, bool) {
	for _, phase := range phases {
		if phase.ID == id {
			return phase, true
		}
	}
	return Phase{}, false
}

func hasBlockedDependencyIssue(
	issues []PhaseApplicabilityDependencyIssue,
	blockedPhase PhaseID,
	requiredPhase PhaseID,
) bool {
	for _, issue := range issues {
		if issue.BlockedPhase == blockedPhase &&
			issue.RequiredPhase == requiredPhase {
			return true
		}
	}
	return false
}

func phaseBlockedByApplicability(
	id PhaseID,
	phases []Phase,
	issues []PhaseApplicabilityDependencyIssue,
	visiting map[PhaseID]bool,
) bool {
	for _, issue := range issues {
		if issue.BlockedPhase == id {
			return true
		}
	}
	if visiting[id] {
		return false
	}
	phase, found := phaseByID(phases, id)
	if !found {
		return false
	}
	visiting[id] = true
	defer delete(visiting, id)
	for _, dependency := range phase.DependsOn {
		if phaseBlockedByApplicability(
			dependency,
			phases,
			issues,
			visiting,
		) {
			return true
		}
	}
	return false
}
