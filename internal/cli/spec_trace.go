package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

const specTraceAuthorityBoundary = "read_only_spec_trace_diagnostic_not_authority_evidence_approval_gate_decision_claim_truth_global_truth_or_publication"

type specTraceRecord struct {
	SchemaVersion       int                          `json:"schema_version"`
	RecordKind          string                       `json:"record_kind"`
	Authority           string                       `json:"authority"`
	AuthorityBoundary   string                       `json:"authority_boundary"`
	SectionID           string                       `json:"section_id"`
	Section             *project.SpecSection         `json:"section,omitempty"`
	BaselineCurrentness specTraceBaselineCurrentness `json:"baseline_currentness"`
	CurrentAuthority    specTraceAuthority           `json:"current_authority"`
	TerminalHistoryRefs []string                     `json:"terminal_history_refs,omitempty"`
	CodeBindings        []specTraceCodeBinding       `json:"code_bindings,omitempty"`
	Drilldowns          []specTraceDrilldown         `json:"drilldowns,omitempty"`
	MissingLinks        []specTraceMissingLink       `json:"missing_links,omitempty"`
}

type specTraceBaselineCurrentness struct {
	Status       string `json:"status"`
	ProjectID    string `json:"project_id,omitempty"`
	CurrentHash  string `json:"current_hash,omitempty"`
	BaselineHash string `json:"baseline_hash,omitempty"`
	CapturedAt   string `json:"captured_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

type specTraceAuthority struct {
	Status               string   `json:"status"`
	ExplicitDecisionRefs []string `json:"explicit_decision_refs,omitempty"`
	DerivedSectionRefs   []string `json:"derived_from_section_refs,omitempty"`
	ConflictSetRefs      []string `json:"conflict_set_refs,omitempty"`
	OverlapReviewSetRefs []string `json:"overlap_review_set_refs,omitempty"`
	AuthorityBoundary    string   `json:"authority_boundary"`
}

type specTraceCodeBinding struct {
	DecisionRef          string   `json:"decision_ref"`
	DecisionTitle        string   `json:"decision_title,omitempty"`
	TargetResolution     string   `json:"target_resolution,omitempty"`
	AffectedFiles        []string `json:"affected_files,omitempty"`
	BindingTargetRefs    []string `json:"binding_target_refs,omitempty"`
	CodeContextDrilldown []string `json:"code_context_drilldown,omitempty"`
}

type specTraceDrilldown struct {
	Label string `json:"label"`
	CLI   string `json:"cli,omitempty"`
	MCP   string `json:"mcp,omitempty"`
}

type specTraceMissingLink struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func buildSpecTraceRecord(
	ctx context.Context,
	projectRoot string,
	store *artifact.Store,
	sectionID string,
) (specTraceRecord, error) {
	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return specTraceRecord{}, err
	}
	section, ok := specUseSectionByID(specSet, sectionID)
	if !ok {
		return specTraceRecord{}, fmt.Errorf("spec section %q not found in ProjectSpecificationSet", strings.TrimSpace(sectionID))
	}
	record := specTraceRecord{
		SchemaVersion:       1,
		RecordKind:          "spec_trace",
		Authority:           "read_only_spec_trace_diagnostic",
		AuthorityBoundary:   specTraceAuthorityBoundary,
		SectionID:           section.ID,
		Section:             &section,
		BaselineCurrentness: specTraceBaseline(projectRoot, section),
		Drilldowns:          specTraceDrilldowns(section.ID),
	}
	if store == nil {
		record.MissingLinks = append(record.MissingLinks, specTraceMissingLink{
			Code:    "artifact_store_unavailable",
			Message: "artifact store was unavailable; current decisions and code bindings were not inspected",
		})
		return record, nil
	}
	report, err := artifact.BuildCurrentGoverningSetReportFiltered(ctx, store, artifact.CurrentGoverningSetFilter{
		TargetRef: "spec_section:" + section.ID,
	})
	if err != nil {
		record.MissingLinks = append(record.MissingLinks, specTraceMissingLink{
			Code:    "governing_set_unavailable",
			Message: err.Error(),
		})
		return record, nil
	}
	record.CurrentAuthority = specTraceAuthorityFromSets(report.Sets)
	record.TerminalHistoryRefs = specTraceTerminalHistoryRefs(report.Sets)
	record.CodeBindings = specTraceCodeBindings(ctx, store, report.Sets)
	record.MissingLinks = append(record.MissingLinks, specTraceMissingLinks(record)...)
	return record, nil
}

func specTraceBaseline(
	projectRoot string,
	section project.SpecSection,
) specTraceBaselineCurrentness {
	baseline := specUseBaselineInput(projectRoot, section)
	out := specTraceBaselineCurrentness{
		Status:      baseline.Status,
		ProjectID:   baseline.ProjectID,
		CurrentHash: specflow.HashSection(section),
		Error:       baseline.Error,
	}
	if baseline.Baseline.Hash != "" {
		out.BaselineHash = baseline.Baseline.Hash
		out.CapturedAt = baseline.Baseline.CapturedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return out
}

func specTraceAuthorityFromSets(
	sets []artifact.CurrentGoverningSet,
) specTraceAuthority {
	out := specTraceAuthority{
		Status:            "clear",
		AuthorityBoundary: artifact.CurrentGoverningAuthorityBoundary,
	}
	for _, set := range sets {
		switch set.TargetResolution {
		case artifact.CurrentGoverningTargetResolutionExplicit:
			out.ExplicitDecisionRefs = append(out.ExplicitDecisionRefs, set.CurrentDecisionRefs...)
		case artifact.CurrentGoverningTargetResolutionDerivedSectionRefs:
			out.DerivedSectionRefs = append(out.DerivedSectionRefs, set.CurrentDecisionRefs...)
		}
		switch set.Posture {
		case artifact.GoverningSetPostureConflict:
			out.Status = "conflict_requires_operator"
			out.ConflictSetRefs = append(out.ConflictSetRefs, set.SetID)
		case artifact.GoverningSetPostureOverlap:
			if out.Status != "conflict_requires_operator" {
				out.Status = "overlap_needs_review"
			}
			out.OverlapReviewSetRefs = append(out.OverlapReviewSetRefs, set.SetID)
		}
	}
	out.ExplicitDecisionRefs = compactSortedStrings(out.ExplicitDecisionRefs)
	out.DerivedSectionRefs = compactSortedStrings(out.DerivedSectionRefs)
	out.ConflictSetRefs = compactSortedStrings(out.ConflictSetRefs)
	out.OverlapReviewSetRefs = compactSortedStrings(out.OverlapReviewSetRefs)
	return out
}

func specTraceTerminalHistoryRefs(
	sets []artifact.CurrentGoverningSet,
) []string {
	refs := []string{}
	for _, set := range sets {
		refs = append(refs, set.TerminalHistoryRefs...)
	}
	return compactSortedStrings(refs)
}

func specTraceCodeBindings(
	ctx context.Context,
	store *artifact.Store,
	sets []artifact.CurrentGoverningSet,
) []specTraceCodeBinding {
	bindings := []specTraceCodeBinding{}
	for _, set := range sets {
		for _, decisionRef := range set.CurrentDecisionRefs {
			decision, err := store.Get(ctx, decisionRef)
			if err != nil || decision.Meta.Kind != artifact.KindDecisionRecord {
				continue
			}
			fields := decision.UnmarshalDecisionFields()
			files := compactSortedStrings(fields.ImplementationFootprint.Files)
			legacyFiles, _ := store.GetAffectedFiles(ctx, decisionRef)
			for _, file := range legacyFiles {
				files = append(files, file.Path)
			}
			files = compactSortedStrings(files)
			bindingRefs := specTraceBindingTargetRefs(fields.BindingTargets)
			bindings = append(bindings, specTraceCodeBinding{
				DecisionRef:          decisionRef,
				DecisionTitle:        decision.Meta.Title,
				TargetResolution:     set.TargetResolution,
				AffectedFiles:        files,
				BindingTargetRefs:    bindingRefs,
				CodeContextDrilldown: specTraceCodeContextDrilldowns(files, bindingRefs),
			})
		}
	}
	return bindings
}

func specTraceBindingTargetRefs(
	targets []artifact.BindingTarget,
) []string {
	refs := []string{}
	for _, target := range targets {
		if target.TargetRef != "" {
			refs = append(refs, target.TargetRef)
			continue
		}
		if target.FilePath != "" {
			refs = append(refs, target.FilePath)
		}
	}
	return compactSortedStrings(refs)
}

func specTraceCodeContextDrilldowns(
	files []string,
	bindingRefs []string,
) []string {
	out := []string{}
	for _, file := range files {
		out = append(out, fmt.Sprintf(`haft_query(action="code_context", file=%q, lane="decisions")`, file))
	}
	for _, ref := range bindingRefs {
		if strings.HasPrefix(ref, "symbol:") {
			out = append(out, fmt.Sprintf(`haft_query(action="code_context", symbol=%q, lane="decisions")`, ref))
		}
	}
	return compactSortedStrings(out)
}

func specTraceDrilldowns(sectionID string) []specTraceDrilldown {
	return []specTraceDrilldown{
		{
			Label: "spec use",
			CLI:   fmt.Sprintf("haft spec use %s --json", sectionID),
			MCP:   fmt.Sprintf(`haft_query(action="spec_use", section_id=%q)`, sectionID),
		},
		{
			Label: "governing set",
			CLI:   fmt.Sprintf("haft decision governing-set --target-ref spec_section:%s --json", sectionID),
			MCP:   fmt.Sprintf(`haft_query(action="governing_set", source_refs=["spec_section:%s"])`, sectionID),
		},
	}
}

func specTraceMissingLinks(record specTraceRecord) []specTraceMissingLink {
	missing := []specTraceMissingLink{}
	if len(record.CurrentAuthority.ExplicitDecisionRefs) == 0 && len(record.CurrentAuthority.DerivedSectionRefs) == 0 {
		missing = append(missing, specTraceMissingLink{
			Code:    "no_current_decision_for_section",
			Message: "no current DecisionRecord was found for this SpecSection target",
		})
	}
	if len(record.CodeBindings) == 0 {
		missing = append(missing, specTraceMissingLink{
			Code:    "no_code_binding_for_section_decisions",
			Message: "current section decisions did not expose affected files or binding targets",
		})
	}
	if record.BaselineCurrentness.Status == specflow.SpecUseBaselineDrifted {
		missing = append(missing, specTraceMissingLink{
			Code:    "source_edition_not_current",
			Message: "SpecSection baseline hash differs from the current SQL section hash",
		})
	}
	return missing
}
