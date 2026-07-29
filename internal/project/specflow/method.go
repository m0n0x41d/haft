// Package specflow models the Haft v7 spec onboarding method as a typed,
// pure Core artifact: a sequence of declarative Phases composed from a
// reusable vocabulary of Checks. Surfaces (MCP plugin, Desktop wizard, CLI)
// consume the same WorkflowIntent shape returned by NextStep; the method
// itself contains no I/O and no LLM calls.
//
// Phases are product knowledge shipped by Haft. Each Phase declares the
// fields a SpecSection must carry by composing Checks. A Phase that
// semantically requires a SoTA field (statement_type, claim_layer,
// valid_until, boundary_perspectives, guard_location) MUST include the
// corresponding Require* Check in its pipeline.
//
// FPF citations live in agent reasoning and in Phase prompt/context
// strings. They never appear inside the .haft/specs/* YAML carriers.
package specflow

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/project"
)

// PhaseID is the canonical identifier for an onboarding phase, e.g.
// "target.role.draft" or "target.section.approve".
type PhaseID string

// Audience marks who the phase output is addressed to: the host agent
// drafting content, or the human principal approving / steering.
type Audience string

const (
	AudienceAgent Audience = "agent"
	AudienceHuman Audience = "human"
)

// Phase is a declarative onboarding step. Phases are static Go values
// registered in phases.go; new phases are added by appending to the
// registry, never by branching on PhaseID at call sites.
type Phase struct {
	ID              PhaseID
	Required        bool
	DependsOn       []PhaseID
	DocumentKind    project.SpecDocumentKind
	SectionKind     string
	PromptForUser   string
	ContextForAgent string
	ExpectedFields  []string
	Checks          []Check
}

// Check is the unit of validation reused between SpecOnboardingMethod
// (forward, prescriptive) and speccheck (backward, diagnostic). Each
// Check returns zero or more SpecCheckFindings; an empty return means
// the section satisfies that check.
//
// Cross-section checks accept the full ProjectSpecificationSet to
// resolve term references, dependency edges, etc. Per-section checks
// ignore the set and read only the section.
type Check interface {
	Name() string
	RunOn(section project.SpecSection, set project.ProjectSpecificationSet) []project.SpecCheckFinding
}

// WorkflowIntent is what NextStep returns to the surface. Surfaces render
// this typed payload to the human and to the host agent. The shape is
// identical across MCP, CLI, and Desktop — that is the contract.
type WorkflowIntent struct {
	Phase            PhaseID                    `json:"phase"`
	Audience         Audience                   `json:"audience"`
	DocumentKind     project.SpecDocumentKind   `json:"document_kind"`
	SectionKind      string                     `json:"section_kind"`
	PromptForUser    string                     `json:"prompt_for_user"`
	ContextForAgent  string                     `json:"context_for_agent"`
	ExpectedFields   []string                   `json:"expected_fields,omitempty"`
	Checks           []string                   `json:"checks,omitempty"`
	BlockingFindings []project.SpecCheckFinding `json:"blocking_findings,omitempty"`
	Terminal         bool                       `json:"terminal"`
	Reason           string                     `json:"reason,omitempty"`
	ApplicabilityCue *PhaseApplicabilityCue     `json:"applicability_cue,omitempty"`
}

// terminalIntent is the intent NextStep returns when no further phase
// applies — every phase has been satisfied or the project is in a state
// the method does not advance from autonomously.
func terminalIntent(reason string) WorkflowIntent {
	return WorkflowIntent{
		Terminal: true,
		Reason:   reason,
	}
}

// NextStep is the deterministic, pure entry point for surfaces. Given a
// derived SpecState, it walks the canonical PhaseRegistry in order and
// returns the first phase that:
//
//   - has all DependsOn phases satisfied; AND
//   - is not yet satisfied (no active section, or active section has
//     blocking findings).
//
// Same input produces the same intent. No I/O, no LLM, no global state.
func NextStep(state SpecState) WorkflowIntent {
	return nextStepFromPhases(state, PhaseRegistry())
}

// NextStepForPhaseSet applies the same pure method to a scope-local phase set.
// It never turns a NotApplicable member into a phase, and it preserves an
// underdetermined member as one neutral non-terminal cue after all currently
// runnable required phases are satisfied.
func NextStepForPhaseSet(
	state SpecState,
	phaseSet ApplicablePhaseSet,
) (WorkflowIntent, error) {
	if !phaseSet.Valid() {
		return WorkflowIntent{}, fmt.Errorf("applicable phase set is invalid")
	}
	intent := nextStepFromApplicablePhaseSet(state, phaseSet)
	if !intent.Terminal {
		return intent, nil
	}
	cue, found := phaseSet.ApplicabilityCue()
	if !found {
		return intent, nil
	}
	return WorkflowIntent{
		Audience:         AudienceAgent,
		Reason:           "specification applicability is underdetermined; recover the named basis before treating the scope as ready",
		ApplicabilityCue: &cue,
	}, nil
}

func nextStepFromPhases(
	state SpecState,
	phases []Phase,
) WorkflowIntent {
	return nextStepFromPhaseCandidates(
		state,
		phases,
		func(PhaseID) bool {
			return false
		},
	)
}

func nextStepFromApplicablePhaseSet(
	state SpecState,
	phaseSet ApplicablePhaseSet,
) WorkflowIntent {
	return nextStepFromPhaseCandidates(
		state,
		phaseSet.Phases(),
		phaseSet.phaseBlockedByApplicability,
	)
}

func nextStepFromPhaseCandidates(
	state SpecState,
	phases []Phase,
	blocked func(PhaseID) bool,
) WorkflowIntent {
	for _, phase := range phases {
		if !phase.Required {
			continue
		}
		if blocked(phase.ID) {
			continue
		}
		if !state.DependenciesSatisfied(phase) {
			continue
		}
		if state.PhaseSatisfied(phase) {
			continue
		}

		section, findings, hasFailing := state.FirstFailingSection(phase)

		intent := WorkflowIntent{
			Phase:           phase.ID,
			Audience:        AudienceAgent,
			DocumentKind:    phase.DocumentKind,
			SectionKind:     phase.SectionKind,
			PromptForUser:   phase.PromptForUser,
			ContextForAgent: phase.ContextForAgent,
			ExpectedFields:  phase.ExpectedFields,
			Checks:          phaseCheckNames(phase),
		}

		if hasFailing {
			intent.Audience = AudienceHuman
			intent.BlockingFindings = findings
			intent.Reason = "section " + section.ID + " has blocking findings; resolve and re-run"
		}

		return intent
	}

	return terminalIntent("all required phases satisfied")
}
