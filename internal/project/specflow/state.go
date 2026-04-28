package specflow

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

// SpecState is the derived view of where a project sits in the
// onboarding method. State is a function of the parsed
// ProjectSpecificationSet — it is recomputed on each call, never stored.
//
// When BaselineStore + ProjectID are populated, NextStep also enforces
// SpecSectionBaseline freshness: an active section without a baseline,
// or with a baseline that has drifted, is treated as not satisfied so
// the phase remains the next step until the operator triages.
type SpecState struct {
	Set            project.ProjectSpecificationSet
	SectionsByKind map[string][]project.SpecSection
	Baselines      BaselineStore
	ProjectID      string
}

// DeriveState builds a SpecState from a parsed ProjectSpecificationSet
// without baseline awareness. The caller is responsible for parsing
// carriers (typically via project.CheckSpecificationSet); DeriveState
// performs no I/O.
func DeriveState(set project.ProjectSpecificationSet) SpecState {
	return DeriveStateWithBaselines(set, nil, "")
}

// DeriveStateWithBaselines builds a SpecState that also enforces
// SpecSectionBaseline freshness. Pass the project's BaselineStore and
// canonical project_id; pass nil/"" to skip baseline enforcement (same
// behavior as DeriveState).
func DeriveStateWithBaselines(set project.ProjectSpecificationSet, baselines BaselineStore, projectID string) SpecState {
	sections := make(map[string][]project.SpecSection)
	for _, section := range set.Sections {
		key := strings.TrimSpace(section.Kind)
		if key == "" {
			continue
		}
		sections[key] = append(sections[key], section)
	}

	return SpecState{
		Set:            set,
		SectionsByKind: sections,
		Baselines:      baselines,
		ProjectID:      strings.TrimSpace(projectID),
	}
}

// SectionsForPhase returns sections matching the phase's SectionKind.
// Empty slice if none exist.
func (s SpecState) SectionsForPhase(phase Phase) []project.SpecSection {
	return s.SectionsByKind[phase.SectionKind]
}

// PhaseSatisfied returns true when at least one active section exists
// for the phase's SectionKind, no Check on that section produces an
// error-level finding, and (when baseline awareness is configured) the
// section has a current baseline. A draft section with passing checks
// is considered "in progress", not satisfied.
func (s SpecState) PhaseSatisfied(phase Phase) bool {
	sections := s.SectionsForPhase(phase)
	if len(sections) == 0 {
		return false
	}

	for _, section := range sections {
		if !sectionIsActive(section) {
			continue
		}
		findings := runPhaseChecks(phase, section, s.Set)
		if hasErrorFinding(findings) {
			continue
		}
		if s.sectionDriftBlocked(section) {
			continue
		}
		return true
	}

	return false
}

// sectionDriftBlocked is true when baseline enforcement is configured
// and the active section either lacks a baseline or has drifted.
func (s SpecState) sectionDriftBlocked(section project.SpecSection) bool {
	if s.Baselines == nil || s.ProjectID == "" {
		return false
	}

	baseline, err := s.Baselines.Get(s.ProjectID, section.ID)
	if err != nil {
		return true // BaselineNotFound or storage error — block until triaged
	}

	return baseline.Hash != HashSection(section)
}

// PhaseInProgress returns true when at least one section exists for the
// phase's SectionKind in any non-active state (draft, etc.) — meaning
// the agent has started but the human has not approved.
func (s SpecState) PhaseInProgress(phase Phase) bool {
	for _, section := range s.SectionsForPhase(phase) {
		if !sectionIsActive(section) {
			return true
		}
	}
	return false
}

// FirstFailingSection returns the first section for the phase with at
// least one error-level finding, plus those findings. Drift / missing-
// baseline findings (when baseline awareness is configured) are
// included alongside structural Check findings so the surface can show
// the operator a single triage list.
func (s SpecState) FirstFailingSection(phase Phase) (project.SpecSection, []project.SpecCheckFinding, bool) {
	for _, section := range s.SectionsForPhase(phase) {
		findings := runPhaseChecks(phase, section, s.Set)
		if sectionIsActive(section) {
			findings = append(findings, s.sectionDriftFindings(section)...)
		}
		if hasErrorFinding(findings) {
			return section, findings, true
		}
	}
	return project.SpecSection{}, nil, false
}

func (s SpecState) sectionDriftFindings(section project.SpecSection) []project.SpecCheckFinding {
	if s.Baselines == nil || s.ProjectID == "" {
		return nil
	}
	scoped := project.ProjectSpecificationSet{Sections: []project.SpecSection{section}}
	return SectionBaselineFindings(scoped, s.Baselines, s.ProjectID)
}

// DependenciesSatisfied returns true when every PhaseID in phase.DependsOn
// is satisfied per PhaseSatisfied.
func (s SpecState) DependenciesSatisfied(phase Phase) bool {
	for _, dep := range phase.DependsOn {
		depPhase, ok := FindPhase(dep)
		if !ok {
			return false
		}
		if !s.PhaseSatisfied(depPhase) {
			return false
		}
	}
	return true
}

func sectionIsActive(section project.SpecSection) bool {
	return strings.EqualFold(strings.TrimSpace(section.Status), string(project.SpecSectionStateActive))
}

func runPhaseChecks(phase Phase, section project.SpecSection, set project.ProjectSpecificationSet) []project.SpecCheckFinding {
	var findings []project.SpecCheckFinding
	for _, check := range phase.Checks {
		findings = append(findings, check.RunOn(section, set)...)
	}
	return findings
}

func hasErrorFinding(findings []project.SpecCheckFinding) bool {
	for _, finding := range findings {
		if strings.EqualFold(finding.Level, FindingLevelError) {
			return true
		}
	}
	return false
}

func phaseCheckNames(phase Phase) []string {
	names := make([]string, 0, len(phase.Checks))
	for _, check := range phase.Checks {
		names = append(names, check.Name())
	}
	return names
}
