package project

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SpecCoverageState string

const (
	SpecCoverageUncovered    SpecCoverageState = "uncovered"
	SpecCoverageReasoned     SpecCoverageState = "reasoned"
	SpecCoverageCommissioned SpecCoverageState = "commissioned"
	SpecCoverageImplemented  SpecCoverageState = "implemented"
	SpecCoverageVerified     SpecCoverageState = "verified"
	SpecCoverageStale        SpecCoverageState = "stale"
)

type SpecCoverageInput struct {
	Sections    []SpecSection
	Problems    []SpecCoverageProblem
	Decisions   []SpecCoverageDecision
	Commissions []SpecCoverageCommission
	RuntimeRuns []SpecCoverageRuntimeRun
	Evidence    []SpecCoverageEvidence
	Now         time.Time
}

type SpecCoverageProblem struct {
	ID          string
	Title       string
	Status      string
	ValidUntil  string
	SectionRefs []string
}

type SpecCoverageDecision struct {
	ID            string
	Title         string
	Status        string
	ValidUntil    string
	ProblemRefs   []string
	SectionRefs   []string
	AffectedFiles []string
	Drifted       bool
}

type SpecCoverageCommission struct {
	ID          string
	DecisionRef string
	State       string
	Status      string
	ValidUntil  string
	SectionRefs []string
}

type SpecCoverageRuntimeRun struct {
	ID                string
	CommissionRef     string
	Event             string
	Verdict           string
	Phase             string
	Reason            string
	RecordedAt        string
	ValidUntil        string
	SectionRefs       []string
	UnsupportedReason string
}

type SpecCoverageEvidence struct {
	ID          string
	ArtifactRef string
	Type        string
	Verdict     string
	CarrierRef  string
	ValidUntil  string
	SectionRefs []string
	CodeRefs    []string
	TestRefs    []string
}

type SpecCoverageReport struct {
	Sections []SpecCoverageSection `json:"sections"`
	Gaps     []SpecCoverageGap     `json:"gaps"`
	Summary  SpecCoverageSummary   `json:"summary"`
}

type SpecCoverageSection struct {
	SectionID    string             `json:"section_id"`
	Title        string             `json:"title,omitempty"`
	DocumentKind string             `json:"document_kind"`
	SpecKind     string             `json:"spec_kind"`
	Path         string             `json:"path"`
	DependsOn    []string           `json:"depends_on,omitempty"`
	TargetRefs   []string           `json:"target_refs,omitempty"`
	State        SpecCoverageState  `json:"state"`
	Why          []string           `json:"why"`
	NextAction   string             `json:"next_action"`
	Edges        []SpecCoverageEdge `json:"edges"`
	Gaps         []SpecCoverageGap  `json:"gaps"`
}

type SpecCoverageEdge struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

type SpecCoverageGap struct {
	SectionID  string `json:"section_id,omitempty"`
	Kind       string `json:"kind"`
	Detail     string `json:"detail"`
	NextAction string `json:"next_action,omitempty"`
}

type SpecCoverageSummary struct {
	TotalSections int            `json:"total_sections"`
	StateCounts   map[string]int `json:"state_counts"`
}

type specCoverageSignals struct {
	Section     SpecSection
	Problems    []SpecCoverageProblem
	Decisions   []SpecCoverageDecision
	Commissions []SpecCoverageCommission
	RuntimeRuns []SpecCoverageRuntimeRun
	Evidence    []SpecCoverageEvidence
	CodeRefs    []string
	TestRefs    []string
	StaleFacts  []string
}

func DeriveSpecCoverage(input SpecCoverageInput) SpecCoverageReport {
	input = normalizeSpecCoverageInput(input)
	sections := make([]SpecCoverageSection, 0, len(input.Sections))

	for _, section := range input.Sections {
		if section.Status != "active" {
			continue
		}

		signals := buildSpecCoverageSignals(input, section)
		state := deriveSpecCoverageState(signals)
		sections = append(sections, specCoverageSection(signals, state))
	}

	report := SpecCoverageReport{
		Sections: sections,
		Gaps:     []SpecCoverageGap{},
	}
	report.Summary = summarizeSpecCoverage(sections)

	return normalizeSpecCoverageReport(report)
}

func normalizeSpecCoverageInput(input SpecCoverageInput) SpecCoverageInput {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}

	input.Problems = normalizeCoverageProblems(input.Problems)
	input.Decisions = normalizeCoverageDecisions(input.Decisions)
	input.Commissions = normalizeCoverageCommissions(input.Commissions)
	input.RuntimeRuns = normalizeCoverageRuntimeRuns(input.RuntimeRuns)
	input.Evidence = normalizeCoverageEvidence(input.Evidence)

	sort.SliceStable(input.Sections, func(i, j int) bool {
		left := input.Sections[i]
		right := input.Sections[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})

	return input
}

func normalizeCoverageProblems(values []SpecCoverageProblem) []SpecCoverageProblem {
	normalized := make([]SpecCoverageProblem, 0, len(values))

	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.Title = strings.TrimSpace(value.Title)
		value.Status = strings.TrimSpace(value.Status)
		value.ValidUntil = strings.TrimSpace(value.ValidUntil)
		value.SectionRefs = sortedUniqueStrings(value.SectionRefs)
		if value.ID == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].ID < normalized[j].ID
	})

	return normalized
}

func normalizeCoverageDecisions(values []SpecCoverageDecision) []SpecCoverageDecision {
	normalized := make([]SpecCoverageDecision, 0, len(values))

	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.Title = strings.TrimSpace(value.Title)
		value.Status = strings.TrimSpace(value.Status)
		value.ValidUntil = strings.TrimSpace(value.ValidUntil)
		value.ProblemRefs = sortedUniqueStrings(value.ProblemRefs)
		value.SectionRefs = sortedUniqueStrings(value.SectionRefs)
		value.AffectedFiles = sortedUniqueStrings(value.AffectedFiles)
		if value.ID == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].ID < normalized[j].ID
	})

	return normalized
}

func normalizeCoverageCommissions(values []SpecCoverageCommission) []SpecCoverageCommission {
	normalized := make([]SpecCoverageCommission, 0, len(values))

	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.DecisionRef = strings.TrimSpace(value.DecisionRef)
		value.State = strings.TrimSpace(value.State)
		value.Status = strings.TrimSpace(value.Status)
		value.ValidUntil = strings.TrimSpace(value.ValidUntil)
		value.SectionRefs = sortedUniqueStrings(value.SectionRefs)
		if value.ID == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].ID < normalized[j].ID
	})

	return normalized
}

func normalizeCoverageRuntimeRuns(values []SpecCoverageRuntimeRun) []SpecCoverageRuntimeRun {
	normalized := make([]SpecCoverageRuntimeRun, 0, len(values))

	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.CommissionRef = strings.TrimSpace(value.CommissionRef)
		value.Event = strings.TrimSpace(value.Event)
		value.Verdict = strings.TrimSpace(value.Verdict)
		value.Phase = strings.TrimSpace(value.Phase)
		value.Reason = strings.TrimSpace(value.Reason)
		value.RecordedAt = strings.TrimSpace(value.RecordedAt)
		value.ValidUntil = strings.TrimSpace(value.ValidUntil)
		value.UnsupportedReason = strings.TrimSpace(value.UnsupportedReason)
		value.SectionRefs = sortedUniqueStrings(value.SectionRefs)
		if value.ID == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].ID < normalized[j].ID
	})

	return normalized
}

func normalizeCoverageEvidence(values []SpecCoverageEvidence) []SpecCoverageEvidence {
	normalized := make([]SpecCoverageEvidence, 0, len(values))

	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.ArtifactRef = strings.TrimSpace(value.ArtifactRef)
		value.Type = strings.TrimSpace(value.Type)
		value.Verdict = strings.TrimSpace(value.Verdict)
		value.CarrierRef = strings.TrimSpace(value.CarrierRef)
		value.ValidUntil = strings.TrimSpace(value.ValidUntil)
		value.SectionRefs = sortedUniqueStrings(value.SectionRefs)
		value.CodeRefs = sortedUniqueStrings(value.CodeRefs)
		value.TestRefs = sortedUniqueStrings(value.TestRefs)
		if value.ID == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].ID < normalized[j].ID
	})

	return normalized
}

func buildSpecCoverageSignals(input SpecCoverageInput, section SpecSection) specCoverageSignals {
	problems := coverageProblemsForSection(input.Problems, section.ID)
	decisions := coverageDecisionsForSection(input.Decisions, problems, section.ID)
	commissions := coverageCommissionsForSection(input.Commissions, decisions, section.ID)
	commissionRefs := coverageCommissionRefsForSection(input.Commissions, decisions, section.ID)
	runtimeRuns := coverageRuntimeRunsForSection(input.RuntimeRuns, commissionRefs, section.ID)
	evidence := coverageEvidenceForSection(input.Evidence, problems, decisions, commissionRefs, runtimeRuns, section.ID)

	signals := specCoverageSignals{
		Section:     section,
		Problems:    problems,
		Decisions:   decisions,
		Commissions: commissions,
		RuntimeRuns: runtimeRuns,
		Evidence:    evidence,
		CodeRefs:    coverageCodeRefs(decisions, evidence),
		TestRefs:    coverageTestRefs(evidence),
	}
	signals.StaleFacts = coverageStaleFacts(input.Now, signals)

	return signals
}

func coverageProblemsForSection(problems []SpecCoverageProblem, sectionID string) []SpecCoverageProblem {
	result := make([]SpecCoverageProblem, 0)

	for _, problem := range problems {
		if !artifactStatusIsActive(problem.Status) {
			continue
		}
		if !containsString(problem.SectionRefs, sectionID) {
			continue
		}

		result = append(result, problem)
	}

	return result
}

func coverageDecisionsForSection(
	decisions []SpecCoverageDecision,
	problems []SpecCoverageProblem,
	sectionID string,
) []SpecCoverageDecision {
	result := make([]SpecCoverageDecision, 0)
	problemRefs := coverageProblemRefs(problems)

	for _, decision := range decisions {
		if !artifactStatusIsActive(decision.Status) {
			continue
		}
		if decisionCoversSection(decision, problemRefs, sectionID) {
			result = append(result, decision)
		}
	}

	return result
}

func coverageProblemRefs(problems []SpecCoverageProblem) []string {
	refs := make([]string, 0, len(problems))

	for _, problem := range problems {
		refs = append(refs, problem.ID)
	}

	return sortedUniqueStrings(refs)
}

func decisionCoversSection(
	decision SpecCoverageDecision,
	problemRefs []string,
	sectionID string,
) bool {
	if containsString(decision.SectionRefs, sectionID) {
		return true
	}

	for _, problemRef := range decision.ProblemRefs {
		if containsString(problemRefs, problemRef) {
			return true
		}
	}

	return false
}

func coverageCommissionsForSection(
	commissions []SpecCoverageCommission,
	decisions []SpecCoverageDecision,
	sectionID string,
) []SpecCoverageCommission {
	result := make([]SpecCoverageCommission, 0)
	decisionRefs := coverageDecisionRefs(decisions)

	for _, commission := range commissions {
		if !artifactStatusIsActive(commission.Status) {
			continue
		}
		if !commissionCoversSection(commission, decisionRefs, sectionID) {
			continue
		}
		if workCommissionStateIsTerminal(commission.State) {
			continue
		}

		result = append(result, commission)
	}

	return result
}

func coverageCommissionRefsForSection(
	commissions []SpecCoverageCommission,
	decisions []SpecCoverageDecision,
	sectionID string,
) []string {
	result := make([]string, 0)
	decisionRefs := coverageDecisionRefs(decisions)

	for _, commission := range commissions {
		if !artifactStatusIsActive(commission.Status) {
			continue
		}
		if !commissionCoversSection(commission, decisionRefs, sectionID) {
			continue
		}

		result = append(result, commission.ID)
	}

	return sortedUniqueStrings(result)
}

func coverageDecisionRefs(decisions []SpecCoverageDecision) []string {
	refs := make([]string, 0, len(decisions))

	for _, decision := range decisions {
		refs = append(refs, decision.ID)
	}

	return sortedUniqueStrings(refs)
}

func commissionCoversSection(
	commission SpecCoverageCommission,
	decisionRefs []string,
	sectionID string,
) bool {
	if containsString(commission.SectionRefs, sectionID) {
		return true
	}
	if containsString(decisionRefs, commission.DecisionRef) {
		return true
	}

	return false
}

func coverageRuntimeRunsForSection(
	runtimeRuns []SpecCoverageRuntimeRun,
	commissionRefs []string,
	sectionID string,
) []SpecCoverageRuntimeRun {
	result := make([]SpecCoverageRuntimeRun, 0)

	for _, runtimeRun := range runtimeRuns {
		if !runtimeRunCoversSection(runtimeRun, commissionRefs, sectionID) {
			continue
		}

		result = append(result, runtimeRun)
	}

	return result
}

func runtimeRunCoversSection(
	runtimeRun SpecCoverageRuntimeRun,
	commissionRefs []string,
	sectionID string,
) bool {
	if containsString(runtimeRun.SectionRefs, sectionID) {
		return true
	}
	if containsString(commissionRefs, runtimeRun.CommissionRef) {
		return true
	}

	return false
}

func coverageEvidenceForSection(
	evidence []SpecCoverageEvidence,
	problems []SpecCoverageProblem,
	decisions []SpecCoverageDecision,
	commissionRefs []string,
	runtimeRuns []SpecCoverageRuntimeRun,
	sectionID string,
) []SpecCoverageEvidence {
	result := make([]SpecCoverageEvidence, 0)
	artifactRefs := coverageEvidenceArtifactRefs(problems, decisions, commissionRefs, runtimeRuns)
	runtimeRunRefs := coverageRuntimeRunRefs(runtimeRuns)

	for _, item := range evidence {
		if item.Verdict == "superseded" {
			continue
		}
		if evidenceCoversSection(item, artifactRefs, runtimeRunRefs, sectionID) {
			result = append(result, item)
		}
	}

	return result
}

func coverageEvidenceArtifactRefs(
	problems []SpecCoverageProblem,
	decisions []SpecCoverageDecision,
	commissionRefs []string,
	runtimeRuns []SpecCoverageRuntimeRun,
) []string {
	refs := make([]string, 0, len(problems)+len(decisions)+len(commissionRefs)+len(runtimeRuns))

	for _, problem := range problems {
		refs = append(refs, problem.ID)
	}
	for _, decision := range decisions {
		refs = append(refs, decision.ID)
	}
	refs = append(refs, commissionRefs...)
	refs = append(refs, coverageRuntimeRunRefs(runtimeRuns)...)

	return sortedUniqueStrings(refs)
}

func coverageRuntimeRunRefs(runtimeRuns []SpecCoverageRuntimeRun) []string {
	refs := make([]string, 0, len(runtimeRuns))

	for _, runtimeRun := range runtimeRuns {
		refs = append(refs, runtimeRun.ID)
	}

	return sortedUniqueStrings(refs)
}

func evidenceCoversSection(
	item SpecCoverageEvidence,
	artifactRefs []string,
	runtimeRunRefs []string,
	sectionID string,
) bool {
	if containsString(item.SectionRefs, sectionID) {
		return true
	}
	if containsString(artifactRefs, item.ArtifactRef) {
		return true
	}
	if containsString(runtimeRunRefs, item.CarrierRef) {
		return true
	}

	return false
}

func coverageCodeRefs(
	decisions []SpecCoverageDecision,
	evidence []SpecCoverageEvidence,
) []string {
	refs := make([]string, 0)

	for _, decision := range decisions {
		refs = append(refs, decision.AffectedFiles...)
	}
	for _, item := range evidence {
		refs = append(refs, item.CodeRefs...)
	}

	return sortedUniqueStrings(refs)
}

func coverageTestRefs(evidence []SpecCoverageEvidence) []string {
	refs := make([]string, 0)

	for _, item := range evidence {
		refs = append(refs, item.TestRefs...)
	}

	return sortedUniqueStrings(refs)
}

func coverageStaleFacts(now time.Time, signals specCoverageSignals) []string {
	facts := make([]string, 0)

	if validUntilExpired(signals.Section.ValidUntil, now) {
		facts = append(facts, "spec section valid_until has expired")
	}

	for _, problem := range signals.Problems {
		if validUntilExpired(problem.ValidUntil, now) {
			facts = append(facts, fmt.Sprintf("problem %s valid_until has expired", problem.ID))
		}
	}
	for _, decision := range signals.Decisions {
		if decision.Status == "refresh_due" {
			facts = append(facts, fmt.Sprintf("decision %s is refresh_due", decision.ID))
		}
		if decision.Drifted {
			facts = append(facts, fmt.Sprintf("decision %s has drift findings", decision.ID))
		}
		if validUntilExpired(decision.ValidUntil, now) {
			facts = append(facts, fmt.Sprintf("decision %s valid_until has expired", decision.ID))
		}
	}
	for _, commission := range signals.Commissions {
		if validUntilExpired(commission.ValidUntil, now) {
			facts = append(facts, fmt.Sprintf("commission %s valid_until has expired", commission.ID))
		}
	}
	for _, runtimeRun := range signals.RuntimeRuns {
		if validUntilExpired(runtimeRun.ValidUntil, now) {
			facts = append(facts, fmt.Sprintf("RuntimeRun %s valid_until has expired", runtimeRun.ID))
		}
	}
	for _, item := range signals.Evidence {
		if validUntilExpired(item.ValidUntil, now) {
			facts = append(facts, fmt.Sprintf("evidence %s valid_until has expired", item.ID))
		}
	}

	return sortedUniqueStrings(facts)
}

func deriveSpecCoverageState(signals specCoverageSignals) SpecCoverageState {
	switch {
	case len(signals.StaleFacts) > 0:
		return SpecCoverageStale
	case hasVerifiedSpecCoverageEvidence(signals.Evidence):
		return SpecCoverageVerified
	case runtimeRunsSupportImplementation(signals.RuntimeRuns) && len(signals.CodeRefs) > 0:
		return SpecCoverageImplemented
	case len(signals.Evidence) > 0 && len(signals.CodeRefs) > 0:
		return SpecCoverageImplemented
	case len(signals.Commissions) > 0:
		return SpecCoverageCommissioned
	case len(signals.Decisions) > 0:
		return SpecCoverageReasoned
	default:
		return SpecCoverageUncovered
	}
}

func hasVerifiedSpecCoverageEvidence(evidence []SpecCoverageEvidence) bool {
	for _, item := range evidence {
		if evidenceVerdictSupports(item.Verdict) {
			return true
		}
	}

	return false
}

func runtimeRunsSupportImplementation(runtimeRuns []SpecCoverageRuntimeRun) bool {
	for _, runtimeRun := range runtimeRuns {
		if !runtimeRunSupportsImplementation(runtimeRun) {
			continue
		}

		return true
	}

	return false
}

func runtimeRunSupportsImplementation(runtimeRun SpecCoverageRuntimeRun) bool {
	if runtimeRunIsUnsupported(runtimeRun) {
		return false
	}

	return runtimeRunVerdictSupportsImplementation(runtimeRun.Verdict)
}

func runtimeRunIsUnsupported(runtimeRun SpecCoverageRuntimeRun) bool {
	if runtimeRun.UnsupportedReason != "" {
		return true
	}
	if runtimeRun.Event == "" {
		return true
	}
	if runtimeRun.Verdict == "" {
		return true
	}

	return false
}

func runtimeRunVerdictSupportsImplementation(verdict string) bool {
	switch strings.TrimSpace(verdict) {
	case "pass", "completed", "accepted", "supports":
		return true
	default:
		return false
	}
}

func specCoverageSection(
	signals specCoverageSignals,
	state SpecCoverageState,
) SpecCoverageSection {
	return SpecCoverageSection{
		SectionID:    signals.Section.ID,
		Title:        signals.Section.Title,
		DocumentKind: signals.Section.DocumentKind,
		SpecKind:     signals.Section.Kind,
		Path:         signals.Section.Path,
		DependsOn:    signals.Section.DependsOn,
		TargetRefs:   signals.Section.TargetRefs,
		State:        state,
		Why:          specCoverageWhy(signals, state),
		NextAction:   specCoverageNextAction(signals, state),
		Edges:        specCoverageEdges(signals),
		Gaps:         sectionCoverageGaps(signals, state),
	}
}

func specCoverageWhy(signals specCoverageSignals, state SpecCoverageState) []string {
	switch state {
	case SpecCoverageStale:
		return signals.StaleFacts
	case SpecCoverageVerified:
		return []string{fmt.Sprintf("%d active supporting evidence item(s) cover this section", countSupportingEvidence(signals.Evidence))}
	case SpecCoverageImplemented:
		if len(signals.Evidence) == 0 && runtimeRunsSupportImplementation(signals.RuntimeRuns) {
			return []string{"active RuntimeRun exists for a decision with code scope, but no supporting verification evidence is present"}
		}
		return []string{"active evidence exists for a decision with code scope, but no supporting verification evidence is present"}
	case SpecCoverageCommissioned:
		return []string{fmt.Sprintf("%d active WorkCommission(s) cover this section", len(signals.Commissions))}
	case SpecCoverageReasoned:
		return []string{fmt.Sprintf("%d active DecisionRecord(s) cover this section", len(signals.Decisions))}
	default:
		if len(signals.Problems) > 0 {
			return []string{"problem framing exists, but no active DecisionRecord or evidence covers this section"}
		}
		return []string{"no active DecisionRecord or evidence covers this section"}
	}
}

func countSupportingEvidence(evidence []SpecCoverageEvidence) int {
	count := 0

	for _, item := range evidence {
		if !evidenceVerdictSupports(item.Verdict) {
			continue
		}

		count++
	}

	return count
}

func specCoverageNextAction(
	signals specCoverageSignals,
	state SpecCoverageState,
) string {
	switch state {
	case SpecCoverageStale:
		return "refresh, waive, reopen, or supersede the stale linked carrier before relying on this section"
	case SpecCoverageVerified:
		return "monitor freshness and refresh before linked evidence or decisions expire"
	case SpecCoverageImplemented:
		return "attach supporting measurement or test evidence for the section's required checks"
	case SpecCoverageCommissioned:
		return "run or complete the WorkCommission and attach evidence"
	case SpecCoverageReasoned:
		return "create a WorkCommission from the linked DecisionRecord"
	default:
		if len(signals.Problems) > 0 {
			return "create a DecisionRecord linked to this section"
		}
		return "frame the gap or create a DecisionRecord linked to this section"
	}
}

func specCoverageEdges(signals specCoverageSignals) []SpecCoverageEdge {
	edges := make([]SpecCoverageEdge, 0)

	for _, problem := range signals.Problems {
		edges = append(edges, SpecCoverageEdge{Type: "spec_section->ProblemCard", Target: problem.ID})
	}
	for _, decision := range signals.Decisions {
		edges = append(edges, SpecCoverageEdge{Type: "spec_section->DecisionRecord", Target: decision.ID})
	}
	for _, commission := range signals.Commissions {
		edges = append(edges, SpecCoverageEdge{Type: "DecisionRecord->WorkCommission", Target: commission.ID})
	}
	for _, runtimeRun := range signals.RuntimeRuns {
		edges = append(edges, SpecCoverageEdge{Type: "WorkCommission->RuntimeRun", Target: runtimeRun.ID})
	}
	for _, item := range signals.Evidence {
		edges = append(edges, SpecCoverageEdge{Type: "evidence_item", Target: item.ID})
		if containsString(coverageRuntimeRunRefs(signals.RuntimeRuns), item.CarrierRef) {
			edges = append(edges, SpecCoverageEdge{Type: "RuntimeRun->evidence_item", Target: item.ID})
		}
	}
	for _, ref := range signals.CodeRefs {
		edges = append(edges, SpecCoverageEdge{Type: "spec_section->file", Target: filepath.ToSlash(ref)})
	}
	for _, ref := range signals.TestRefs {
		edges = append(edges, SpecCoverageEdge{Type: "spec_section->test", Target: filepath.ToSlash(ref)})
	}

	sort.SliceStable(edges, func(i, j int) bool {
		left := edges[i].Type + "\x00" + edges[i].Target
		right := edges[j].Type + "\x00" + edges[j].Target
		return left < right
	})

	return edges
}

func sectionCoverageGaps(
	signals specCoverageSignals,
	state SpecCoverageState,
) []SpecCoverageGap {
	gaps := unsupportedRuntimeRunGaps(signals)
	sectionID := signals.Section.ID

	switch state {
	case SpecCoverageStale:
		gaps = append(gaps, SpecCoverageGap{
			SectionID:  sectionID,
			Kind:       "stale_link",
			Detail:     strings.Join(signals.StaleFacts, "; "),
			NextAction: "refresh or supersede stale linked carriers",
		})
	case SpecCoverageVerified:
		return gaps
	case SpecCoverageImplemented:
		gaps = append(gaps, SpecCoverageGap{
			SectionID:  sectionID,
			Kind:       "verification_missing",
			Detail:     "implementation evidence exists without supporting verification evidence",
			NextAction: "attach supporting measurement or test evidence",
		})
	case SpecCoverageCommissioned:
		gaps = append(gaps, SpecCoverageGap{
			SectionID:  sectionID,
			Kind:       "evidence_missing",
			Detail:     "active WorkCommission exists without linked evidence",
			NextAction: "complete runtime work and attach evidence",
		})
	case SpecCoverageReasoned:
		gaps = append(gaps, SpecCoverageGap{
			SectionID:  sectionID,
			Kind:       "commission_missing",
			Detail:     "active DecisionRecord exists without an active WorkCommission",
			NextAction: "create a WorkCommission from the decision",
		})
	default:
		gaps = append(gaps, SpecCoverageGap{
			SectionID:  sectionID,
			Kind:       "decision_missing",
			Detail:     "active section has no governing DecisionRecord or evidence",
			NextAction: "create a DecisionRecord linked to this section",
		})
	}

	return gaps
}

func unsupportedRuntimeRunGaps(signals specCoverageSignals) []SpecCoverageGap {
	gaps := make([]SpecCoverageGap, 0)

	for _, runtimeRun := range signals.RuntimeRuns {
		if !runtimeRunIsUnsupported(runtimeRun) {
			continue
		}

		detail := runtimeRun.UnsupportedReason
		if detail == "" {
			detail = fmt.Sprintf("RuntimeRun %s has an unsupported storage shape", runtimeRun.ID)
		}

		gaps = append(gaps, SpecCoverageGap{
			SectionID:  signals.Section.ID,
			Kind:       "runtime_run_unsupported",
			Detail:     detail,
			NextAction: "record RuntimeRun event and verdict fields before deriving runtime coverage from this carrier",
		})
	}

	return gaps
}

func summarizeSpecCoverage(sections []SpecCoverageSection) SpecCoverageSummary {
	summary := SpecCoverageSummary{
		TotalSections: len(sections),
		StateCounts:   map[string]int{},
	}

	for _, section := range sections {
		summary.StateCounts[string(section.State)]++
	}

	return summary
}

func normalizeSpecCoverageReport(report SpecCoverageReport) SpecCoverageReport {
	if report.Sections == nil {
		report.Sections = []SpecCoverageSection{}
	}
	if report.Gaps == nil {
		report.Gaps = []SpecCoverageGap{}
	}
	if report.Summary.StateCounts == nil {
		report.Summary.StateCounts = map[string]int{}
	}

	for index := range report.Sections {
		if report.Sections[index].Why == nil {
			report.Sections[index].Why = []string{}
		}
		if report.Sections[index].Edges == nil {
			report.Sections[index].Edges = []SpecCoverageEdge{}
		}
		if report.Sections[index].Gaps == nil {
			report.Sections[index].Gaps = []SpecCoverageGap{}
		}
	}

	return report
}

func artifactStatusIsActive(status string) bool {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		return true
	}
	if normalized == "active" {
		return true
	}
	if normalized == "refresh_due" {
		return true
	}

	return false
}

func workCommissionStateIsTerminal(state string) bool {
	switch strings.TrimSpace(state) {
	case "completed", "completed_with_projection_debt", "cancelled", "failed", "expired":
		return true
	default:
		return false
	}
}

func evidenceVerdictSupports(verdict string) bool {
	switch strings.TrimSpace(verdict) {
	case "supports", "accepted":
		return true
	default:
		return false
	}
}

func validUntilExpired(validUntil string, now time.Time) bool {
	expiry, ok := parseCoverageValidUntil(validUntil)
	if !ok {
		return false
	}

	return expiry.Before(now)
}

func parseCoverageValidUntil(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, true
	}

	return time.Time{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}

		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	sort.Strings(result)
	return result
}
