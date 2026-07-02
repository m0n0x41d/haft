package cli

import (
	"context"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

func specBindingDebtReportForStatus(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) artifact.SpecBindingDebtReport {
	report := artifact.SpecBindingDebtReport{
		SchemaVersion: 1,
		Authority:     "read_only_spec_binding_debt_attention",
	}

	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return report
	}

	activeSectionRefs := activeSpecSectionRefSet(specSet)
	if len(activeSectionRefs) == 0 {
		return report
	}

	decisions, err := store.ListByKind(ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		return report
	}

	for _, decision := range decisions {
		if decision == nil || !specBindingDebtDecisionStatusInScope(decision.Meta.Status) {
			continue
		}
		full, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			continue
		}
		appendSpecBindingDebtForDecision(&report, full, activeSectionRefs)
	}

	return report
}

func activeSpecSectionRefSet(specSet project.ProjectSpecificationSet) map[string]struct{} {
	refs := map[string]struct{}{}
	for _, section := range specSet.Sections {
		if strings.TrimSpace(section.Status) != string(project.SpecSectionStateActive) {
			continue
		}
		id := strings.TrimSpace(section.ID)
		if id == "" {
			continue
		}
		refs[id] = struct{}{}
		refs["spec_section:"+id] = struct{}{}
		refs["spec-section:"+id] = struct{}{}
	}
	return refs
}

func specBindingDebtDecisionStatusInScope(status artifact.Status) bool {
	return status == artifact.StatusActive || status == artifact.StatusRefreshDue
}

func appendSpecBindingDebtForDecision(
	report *artifact.SpecBindingDebtReport,
	decision *artifact.Artifact,
	activeSectionRefs map[string]struct{},
) {
	fields := decision.UnmarshalDecisionFields()
	mode := decision.Meta.Mode
	if mode == "" {
		mode = artifact.ModeStandard
	}

	if specBindingDebtDecisionNeedsBinding(mode, fields) {
		report.Summary.DecisionsMissingSpecBinding++
		report.Items = append(report.Items, artifact.SpecBindingDebtItem{
			DecisionRef: decision.Meta.ID,
			Title:       decision.Meta.Title,
			Kind:        "decisions_missing_spec_binding",
			Message:     "standard/deep spec-enabled decision has no section_refs or spec_binding_preflight receipt",
		})
	}

	invalidRefs := invalidSpecSectionRefs(fields.SectionRefs, activeSectionRefs)
	if len(invalidRefs) > 0 {
		report.Summary.DecisionsWithInvalidSpecRefs++
		report.Items = append(report.Items, artifact.SpecBindingDebtItem{
			DecisionRef: decision.Meta.ID,
			Title:       decision.Meta.Title,
			Kind:        "decisions_with_invalid_spec_refs",
			SectionRefs: invalidRefs,
			Message:     "section_refs do not resolve to active SpecSections",
		})
	}

	if fields.SpecBindingPreflight == nil {
		return
	}
	switch fields.SpecBindingPreflight.State {
	case artifact.SpecBindingStateDraftNeeded:
		report.Summary.DraftSectionNeededDebt++
		report.Items = append(report.Items, artifact.SpecBindingDebtItem{
			DecisionRef: decision.Meta.ID,
			Title:       decision.Meta.Title,
			Kind:        "draft_section_needed_debt",
			Message:     "preflight said a draft SpecSection or spec delta is needed",
		})
	case artifact.SpecBindingStateOutOfSpec:
		report.Summary.OutOfSpecDecisionDebt++
		report.Items = append(report.Items, artifact.SpecBindingDebtItem{
			DecisionRef: decision.Meta.ID,
			Title:       decision.Meta.Title,
			Kind:        "out_of_spec_decision_debt",
			Message:     specBindingDebtMessage(fields.SpecBindingPreflight),
		})
	}
}

func specBindingDebtDecisionNeedsBinding(mode artifact.Mode, fields artifact.DecisionFields) bool {
	if mode == artifact.ModeTactical || mode == artifact.ModeNote {
		return false
	}
	return len(fields.SectionRefs) == 0 && fields.SpecBindingPreflight == nil
}

func invalidSpecSectionRefs(sectionRefs []string, activeSectionRefs map[string]struct{}) []string {
	invalid := []string{}
	for _, ref := range sectionRefs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		if _, ok := activeSectionRefs[trimmed]; ok {
			continue
		}
		invalid = append(invalid, trimmed)
	}
	return invalid
}

func specBindingDebtMessage(preflight *artifact.SpecBindingPreflight) string {
	if preflight == nil || strings.TrimSpace(preflight.StatusDebt.Message) == "" {
		return "decision is explicitly outside current active specs"
	}
	return preflight.StatusDebt.Message
}
