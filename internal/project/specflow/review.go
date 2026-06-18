package specflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	ReviewKindSpecSemantic  = "spec_semantic_review"
	ReviewProfileSemanticV2 = "spec_semantic_review_v2"
	ReviewAuthority         = "advisory_only"

	ReviewSeverityInfo                  = "info"
	ReviewSeverityWarn                  = "warn"
	ReviewSeverityAbstain               = "abstain"
	ReviewSeverityBlockedForStrongerUse = "blocked_for_stronger_use"

	ReviewUseDocumentaryReading       = "documentary_reading"
	ReviewUseAbstainUntilClarified    = "abstain_until_clarified"
	ReviewUseBlockedForAuthorityUse   = "blocked_for_authority_use"
	ReviewUseBlockedForStrongerUse    = "blocked_for_stronger_use"
	ReviewUseAdvisoryFindingInputOnly = "advisory_finding_input_only"

	ReviewModelDispositionUsed              = "used"
	ReviewModelDispositionBoundaryPreserved = "boundary_preserved"
	ReviewModelDispositionAbstain           = "abstain"
)

type ReviewPacket struct {
	ReviewKind string          `json:"review_kind"`
	Authority  string          `json:"authority"`
	Profile    ReviewProfile   `json:"profile"`
	Summary    ReviewSummary   `json:"summary"`
	Sections   []ReviewSection `json:"sections"`
	Findings   []ReviewFinding `json:"findings"`
}

type ReviewProfile struct {
	SchemaVersion int                `json:"schema_version"`
	ID            string             `json:"id"`
	Authority     string             `json:"authority"`
	ModelInputs   []ReviewModelInput `json:"model_inputs"`
}

type ReviewModelInput struct {
	Name        string `json:"name"`
	Disposition string `json:"disposition"`
	Reading     string `json:"reading"`
}

type ReviewSummary struct {
	TotalSections              int `json:"total_sections"`
	ActiveSections             int `json:"active_sections"`
	CheckedSections            int `json:"checked_sections"`
	TotalFindings              int `json:"total_findings"`
	InfoFindings               int `json:"info_findings"`
	WarnFindings               int `json:"warn_findings"`
	AbstainFindings            int `json:"abstain_findings"`
	BlockedForStrongerUse      int `json:"blocked_for_stronger_use_findings"`
	StructuralFindingsObserved int `json:"structural_findings_observed"`
	ExplicitClaims             int `json:"explicit_claims"`
	DeclaredClaims             int `json:"declared_claims"`
	MixedUnresolvedClaims      int `json:"mixed_unresolved_claims"`
	UnclassifiedClaims         int `json:"unclassified_claims"`
	MissingSupportClaims       int `json:"missing_support_claims"`
}

type ReviewSection struct {
	SectionID     string                       `json:"section_id"`
	Title         string                       `json:"title,omitempty"`
	DocumentKind  string                       `json:"document_kind"`
	Kind          string                       `json:"kind"`
	StatementType string                       `json:"statement_type"`
	ClaimLayer    string                       `json:"claim_layer"`
	Bearer        string                       `json:"bearer"`
	Frame         string                       `json:"frame"`
	SystemFrame   project.SystemReferenceFrame `json:"system_frame"`
	StrongerUse   string                       `json:"stronger_use"`
	StateReading  StateReading                 `json:"state_reading"`
	ClaimRegister ClaimRegisterSummary         `json:"claim_register"`
	Claims        []ReviewClaim                `json:"claims,omitempty"`
	Source        SourceSpan                   `json:"source"`
	FindingCodes  []string                     `json:"finding_codes,omitempty"`
}

type StateReading struct {
	SchemaVersion   int    `json:"schema_version"`
	Profile         string `json:"profile"`
	Bearer          string `json:"bearer"`
	Frame           string `json:"frame"`
	Use             string `json:"use"`
	Reading         string `json:"reading"`
	ReopenCondition string `json:"reopen_condition"`
}

type ReviewFinding struct {
	SectionID    string     `json:"section_id,omitempty"`
	RuleID       string     `json:"rule_id"`
	Severity     string     `json:"severity"`
	Finding      string     `json:"finding"`
	WhyItMatters string     `json:"why_it_matters"`
	FPFHint      FPFHint    `json:"fpf_hint"`
	Source       SourceSpan `json:"source"`
}

type FPFHint struct {
	Principle   string `json:"principle"`
	AgentAction string `json:"agent_action"`
	StrongerUse string `json:"stronger_use"`
}

type SourceSpan struct {
	Path      string `json:"path,omitempty"`
	Line      int    `json:"line,omitempty"`
	FieldPath string `json:"field_path,omitempty"`
}

type reviewSubject struct {
	section project.SpecSection
	bearer  string
	frame   string
	source  SourceSpan
}

// ReviewSpecificationSet runs the deterministic, read-only semantic floor over
// active SpecSections. It creates advisory findings only; lifecycle mutation and
// authority-bearing state are intentionally inexpressible here.
func ReviewSpecificationSet(set project.ProjectSpecificationSet) ReviewPacket {
	subjects := reviewSubjects(set)

	packet := ReviewPacket{
		ReviewKind: ReviewKindSpecSemantic,
		Authority:  ReviewAuthority,
		Profile:    semanticReviewProfile(),
		Summary: ReviewSummary{
			TotalSections:   len(set.Sections),
			ActiveSections:  len(subjects),
			CheckedSections: len(subjects),
		},
		Sections: make([]ReviewSection, 0, len(subjects)),
		Findings: structuralReviewFindings(set.Findings),
	}

	for _, subject := range subjects {
		findings := reviewSubjectFindings(subject)
		packet.Findings = append(packet.Findings, findings...)
		packet.Sections = append(packet.Sections, reviewSection(subject, findings))
	}

	packet.Summary = summarizeReview(packet)

	return packet
}

func semanticReviewProfile() ReviewProfile {
	return ReviewProfile{
		SchemaVersion: 1,
		ID:            ReviewProfileSemanticV2,
		Authority:     ReviewAuthority,
		ModelInputs: []ReviewModelInput{
			{
				Name:        "claim_register_v1",
				Disposition: ReviewModelDispositionUsed,
				Reading:     "explicit SpecSection claims are classified as L/A/D/E and checked for claim-scoped support",
			},
			{
				Name:        "system_reference_frame_v1",
				Disposition: ReviewModelDispositionUsed,
				Reading:     "declared target/enabling system_frame drives frame diagnostics",
			},
			{
				Name:        "state_readings_v1",
				Disposition: ReviewModelDispositionUsed,
				Reading:     "per-section readings name bearer, frame, use, and reopen condition",
			},
			{
				Name:        "publication_unit_v1",
				Disposition: ReviewModelDispositionBoundaryPreserved,
				Reading:     "review preserves source/publication/carrier boundaries and does not treat carrier bytes as semantic authority",
			},
			{
				Name:        "transformation_record_v1",
				Disposition: ReviewModelDispositionBoundaryPreserved,
				Reading:     "review requires transformation authority to come from explicit referenced records, not descriptive spec prose",
			},
			{
				Name:        "value_slice",
				Disposition: ReviewModelDispositionAbstain,
				Reading:     "no first-class ValueSlice model exists in this slice; high-risk value/platform sections block stronger use until explicit claims and support exist",
			},
		},
	}
}

func reviewSubjects(set project.ProjectSpecificationSet) []reviewSubject {
	subjects := make([]reviewSubject, 0, len(set.Sections))

	for _, section := range set.Sections {
		if !sectionIsActive(section) {
			continue
		}

		subject := reviewSubject{
			section: section,
			bearer:  sectionBearer(section),
			frame:   sectionFrame(section),
			source:  sectionSource(section, ""),
		}
		subjects = append(subjects, subject)
	}

	sort.SliceStable(subjects, func(i, j int) bool {
		left := subjects[i].section
		right := subjects[j].section

		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}

		return left.ID < right.ID
	})

	return subjects
}

func structuralReviewFindings(findings []project.SpecCheckFinding) []ReviewFinding {
	reviewFindings := make([]ReviewFinding, 0, len(findings))

	for _, finding := range findings {
		reviewFindings = append(reviewFindings, ReviewFinding{
			SectionID: finding.SectionID,
			RuleID:    "structural_findings_present",
			Severity:  ReviewSeverityBlockedForStrongerUse,
			Finding: fmt.Sprintf(
				"structural spec finding %q must be resolved before stronger semantic use",
				finding.Code,
			),
			WhyItMatters: "Semantic review cannot safely infer bearer, frame, or support when the carrier has structural findings.",
			FPFHint: FPFHint{
				Principle:   "Description != authority",
				AgentAction: "Run `haft spec check` and resolve structural findings before treating this section as stronger-use input.",
				StrongerUse: ReviewUseBlockedForStrongerUse,
			},
			Source: SourceSpan{
				Path:      finding.Path,
				Line:      finding.Line,
				FieldPath: finding.FieldPath,
			},
		})
	}

	return reviewFindings
}

func reviewSubjectFindings(subject reviewSubject) []ReviewFinding {
	findings := make([]ReviewFinding, 0)

	findings = append(findings, missingBearerFindings(subject)...)
	findings = append(findings, missingFrameFindings(subject)...)
	findings = append(findings, frameMismatchFindings(subject)...)
	findings = append(findings, activeCarrierFindings(subject)...)
	findings = append(findings, claimRegisterFindings(subject)...)
	findings = append(findings, strongClaimSupportFindings(subject)...)
	findings = append(findings, authoritySupportFindings(subject)...)
	findings = append(findings, highRiskUnknownFindings(subject)...)
	findings = append(findings, mixedStatementLayerFindings(subject)...)

	return findings
}

func missingBearerFindings(subject reviewSubject) []ReviewFinding {
	if sectionHasExplicitBearer(subject.section) {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"missing_bearer",
			ReviewSeverityWarn,
			"active SpecSection does not declare a complete primary object reading",
			"Agents need a named bearer so they do not confuse the markdown carrier, the described object, and the governance state.",
			"Object != Description != Carrier",
			"Add or clarify `title` and `kind` so the section names the object it describes.",
			ReviewUseAbstainUntilClarified,
			"title",
		),
	}
}

func missingFrameFindings(subject reviewSubject) []ReviewFinding {
	if subject.frame != "unknown" {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"missing_system_frame",
			ReviewSeverityAbstain,
			"active SpecSection does not declare a target-system or enabling-system frame",
			"Semantic review must know whether the section describes the target system or the enabling system before it can advise stronger use.",
			"Target system != enabling system",
			"Set `system_frame` to target_system or enabling_system, or move the carrier under the matching canonical spec file.",
			ReviewUseAbstainUntilClarified,
			"system_frame",
		),
	}
}

func frameMismatchFindings(subject reviewSubject) []ReviewFinding {
	mismatch := frameMismatch(subject.section)
	if mismatch == "" {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"system_frame_mismatch",
			ReviewSeverityBlockedForStrongerUse,
			mismatch,
			"Cross-frame carriers can make target-system claims look like enabling-system policy or the reverse.",
			"Target system != enabling system",
			"Align `system_frame` with its carrier frame, or split the views into separate target and enabling sections.",
			ReviewUseBlockedForStrongerUse,
			"system_frame",
		),
	}
}

func activeCarrierFindings(subject reviewSubject) []ReviewFinding {
	if !strings.EqualFold(strings.TrimSpace(subject.section.ClaimLayer), "carrier") {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"active_carrier_layer",
			ReviewSeverityAbstain,
			"active SpecSection has claim_layer=carrier",
			"Carrier-layer text can document where a description lives, but it is not enough to support stronger object/work/evidence use.",
			"Object != Description != Carrier",
			"Change `claim_layer` to the actual claim layer after human review, or keep the section draft/deprecated if it is only a placeholder.",
			ReviewUseAbstainUntilClarified,
			"claim_layer",
		),
	}
}

func strongClaimSupportFindings(subject reviewSubject) []ReviewFinding {
	if sectionHasExplicitClaims(subject.section) {
		return nil
	}
	if !sectionMakesStrongClaim(subject.section) {
		return nil
	}
	if sectionHasSupportRefs(subject.section) {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"strong_claim_without_support",
			ReviewSeverityWarn,
			"active SpecSection makes a stronger claim without support refs",
			"Stronger claims need explicit support carriers so agents do not treat prose as evidence.",
			"Evidence/support refs required for stronger use",
			"Add `depends_on`, `target_refs`, or `evidence_required` entries that identify what supports this section.",
			ReviewUseAbstainUntilClarified,
			"evidence_required",
		),
	}
}

func authoritySupportFindings(subject reviewSubject) []ReviewFinding {
	if sectionHasExplicitClaims(subject.section) {
		return nil
	}
	if !sectionUsesAuthorityVocabulary(subject.section) {
		return nil
	}
	if len(subject.section.EvidenceRequired) > 0 {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"authority_like_without_evidence_requirement",
			ReviewSeverityBlockedForStrongerUse,
			"authority-like SpecSection has no evidence_required guard",
			"A duty/admissibility/evidence section can be misread as a gate unless the required guard evidence is explicit.",
			"Description != authority",
			"Add `evidence_required` guards or weaken the statement so it is not authority-like.",
			ReviewUseBlockedForAuthorityUse,
			"evidence_required",
		),
	}
}

func highRiskUnknownFindings(subject reviewSubject) []ReviewFinding {
	fieldPath := sectionHighRiskSignalFieldPath(subject.section)
	if fieldPath == "" {
		return nil
	}
	if sectionHasExplicitClaims(subject.section) {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"unknown_high_risk_without_explicit_claims",
			ReviewSeverityBlockedForStrongerUse,
			"high-risk SpecSection has no explicit claim register entries",
			"Licensing, legal, compliance, privacy, and security platform sections can affect external obligations; prose alone must not become authority.",
			"Unknown high-risk use must abstain",
			"Declare explicit L/A/D/E `claims` with claim-scoped support refs before using this section as authority, admission, or execution input.",
			ReviewUseBlockedForStrongerUse,
			fieldPath,
		),
	}
}

func mixedStatementLayerFindings(subject reviewSubject) []ReviewFinding {
	statementType := strings.ToLower(strings.TrimSpace(subject.section.StatementType))
	claimLayer := strings.ToLower(strings.TrimSpace(subject.section.ClaimLayer))
	if statementType != "explanation" || claimLayer != "work" {
		return nil
	}

	return []ReviewFinding{
		newReviewFinding(
			subject,
			"description_use_confusion",
			ReviewSeverityWarn,
			"explanation statement is attached to work claim_layer",
			"An explanation describes; a work-layer section constrains production or execution. Mixing them invites agents to treat description as permission.",
			"Description != work",
			"Split the explanatory text from the work rule, or change the statement_type/claim_layer pair to the intended reading.",
			ReviewUseAbstainUntilClarified,
			"statement_type",
		),
	}
}

func newReviewFinding(
	subject reviewSubject,
	ruleID string,
	severity string,
	finding string,
	whyItMatters string,
	principle string,
	agentAction string,
	strongerUse string,
	fieldPath string,
) ReviewFinding {
	return ReviewFinding{
		SectionID:    subject.section.ID,
		RuleID:       ruleID,
		Severity:     severity,
		Finding:      finding,
		WhyItMatters: whyItMatters,
		FPFHint: FPFHint{
			Principle:   principle,
			AgentAction: agentAction,
			StrongerUse: strongerUse,
		},
		Source: sectionSource(subject.section, fieldPath),
	}
}

func reviewSection(subject reviewSubject, findings []ReviewFinding) ReviewSection {
	codes := make([]string, 0, len(findings))
	for _, finding := range findings {
		codes = append(codes, finding.RuleID)
	}
	claims := reviewClaims(subject)
	for index := range claims {
		claims[index].FindingCodes = reviewClaimFindingCodes(claims[index], findings)
	}

	strongerUse := sectionStrongerUse(findings)

	return ReviewSection{
		SectionID:     subject.section.ID,
		Title:         subject.section.Title,
		DocumentKind:  subject.section.DocumentKind,
		Kind:          subject.section.Kind,
		StatementType: subject.section.StatementType,
		ClaimLayer:    subject.section.ClaimLayer,
		Bearer:        subject.bearer,
		Frame:         subject.frame,
		SystemFrame:   subject.section.SystemFrame,
		StrongerUse:   strongerUse,
		StateReading:  sectionStateReading(subject, strongerUse),
		ClaimRegister: claimRegisterSummary(claims, findings),
		Claims:        claims,
		Source:        subject.source,
		FindingCodes:  codes,
	}
}

func sectionStateReading(subject reviewSubject, strongerUse string) StateReading {
	return StateReading{
		SchemaVersion:   1,
		Profile:         ReviewProfileSemanticV2,
		Bearer:          subject.bearer,
		Frame:           subject.frame,
		Use:             strongerUse,
		Reading:         strongerUse,
		ReopenCondition: sectionReopenCondition(subject, strongerUse),
	}
}

func sectionReopenCondition(subject reviewSubject, strongerUse string) string {
	return fmt.Sprintf(
		"reopen this %s reading if bearer %q, frame %q, use %q, carrier bytes, support refs, or valid_until/currentness change",
		ReviewProfileSemanticV2,
		subject.bearer,
		subject.frame,
		strongerUse,
	)
}

func reviewClaimFindingCodes(claim ReviewClaim, findings []ReviewFinding) []string {
	codes := make([]string, 0)
	for _, finding := range findings {
		if !strings.HasPrefix(finding.Source.FieldPath, claim.Source.FieldPath) {
			continue
		}

		codes = append(codes, finding.RuleID)
	}

	return codes
}

func summarizeReview(packet ReviewPacket) ReviewSummary {
	summary := packet.Summary
	summary.TotalFindings = len(packet.Findings)

	for _, finding := range packet.Findings {
		switch finding.Severity {
		case ReviewSeverityInfo:
			summary.InfoFindings++
		case ReviewSeverityWarn:
			summary.WarnFindings++
		case ReviewSeverityAbstain:
			summary.AbstainFindings++
		case ReviewSeverityBlockedForStrongerUse:
			summary.BlockedForStrongerUse++
		}

		if finding.RuleID == "structural_findings_present" {
			summary.StructuralFindingsObserved++
		}
	}
	for _, section := range packet.Sections {
		summary.ExplicitClaims += section.ClaimRegister.ExplicitClaims
		summary.DeclaredClaims += section.ClaimRegister.DeclaredClaims
		summary.MixedUnresolvedClaims += section.ClaimRegister.MixedUnresolvedClaims
		summary.UnclassifiedClaims += section.ClaimRegister.UnclassifiedClaims
		summary.MissingSupportClaims += section.ClaimRegister.MissingSupportClaims
	}

	return summary
}

func sectionStrongerUse(findings []ReviewFinding) string {
	for _, finding := range findings {
		if finding.Severity == ReviewSeverityBlockedForStrongerUse {
			return ReviewUseBlockedForStrongerUse
		}
	}
	for _, finding := range findings {
		if finding.Severity == ReviewSeverityAbstain {
			return ReviewUseAbstainUntilClarified
		}
	}

	return ReviewUseDocumentaryReading
}

func sectionSource(section project.SpecSection, fieldPath string) SourceSpan {
	source := SourceSpan{
		Path: section.Path,
		Line: section.Line,
	}
	if strings.TrimSpace(fieldPath) != "" {
		source.FieldPath = "$." + strings.TrimSpace(fieldPath)
	}

	return source
}

func sectionBearer(section project.SpecSection) string {
	parts := []string{
		strings.TrimSpace(section.DocumentKind),
		strings.TrimSpace(section.Kind),
		strings.TrimSpace(section.Title),
	}

	return strings.Join(nonEmptyStrings(parts), " / ")
}

func sectionFrame(section project.SpecSection) string {
	frame := strings.TrimSpace(section.SystemFrame.Kind)
	if frame == "" {
		return "unknown"
	}

	return frame
}

func sectionHasExplicitBearer(section project.SpecSection) bool {
	if strings.TrimSpace(section.DocumentKind) == "" {
		return false
	}
	if strings.TrimSpace(section.Kind) == "" {
		return false
	}
	if strings.TrimSpace(section.Title) == "" {
		return false
	}

	return true
}

func frameMismatch(section project.SpecSection) string {
	documentKind := strings.TrimSpace(section.DocumentKind)
	frame := sectionFrame(section)

	if frame == "target_system" && documentKind != string(project.SpecDocumentKindTargetSystem) {
		return fmt.Sprintf("declared system_frame %q conflicts with carrier frame %q", frame, documentKind)
	}
	if frame == "enabling_system" && documentKind != string(project.SpecDocumentKindEnablingSystem) {
		return fmt.Sprintf("declared system_frame %q conflicts with carrier frame %q", frame, documentKind)
	}

	return ""
}

func sectionMakesStrongClaim(section project.SpecSection) bool {
	statementType := strings.ToLower(strings.TrimSpace(section.StatementType))
	claimLayer := strings.ToLower(strings.TrimSpace(section.ClaimLayer))
	if statementType == "explanation" {
		return false
	}
	if claimLayer == "carrier" {
		return false
	}

	return true
}

func sectionUsesAuthorityVocabulary(section project.SpecSection) bool {
	statementType := strings.ToLower(strings.TrimSpace(section.StatementType))
	claimLayer := strings.ToLower(strings.TrimSpace(section.ClaimLayer))
	if statementType == "admissibility" {
		return true
	}
	if statementType == "duty" {
		return true
	}
	if statementType == "evidence" {
		return true
	}
	if claimLayer == "work" {
		return true
	}
	if claimLayer == "evidence" {
		return true
	}

	return false
}

func sectionHasSupportRefs(section project.SpecSection) bool {
	if len(section.DependsOn) > 0 {
		return true
	}
	if len(section.TargetRefs) > 0 {
		return true
	}
	if len(section.EvidenceRequired) > 0 {
		return true
	}

	return false
}

type reviewStructuredTextField struct {
	fieldPath string
	value     string
}

func sectionHighRiskSignalFieldPath(section project.SpecSection) string {
	fields := []reviewStructuredTextField{
		{fieldPath: "kind", value: section.Kind},
		{fieldPath: "title", value: section.Title},
		{fieldPath: "spec", value: section.Spec},
		{fieldPath: "statement_type", value: section.StatementType},
		{fieldPath: "claim_layer", value: section.ClaimLayer},
	}
	for index, term := range section.Terms {
		fields = append(fields, reviewStructuredTextField{
			fieldPath: fmt.Sprintf("terms[%d]", index),
			value:     term,
		})
	}
	for _, field := range fields {
		if !textHasHighRiskSignal(field.value) {
			continue
		}

		return field.fieldPath
	}

	return ""
}

func textHasHighRiskSignal(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	signals := []string{
		"licens",
		"legal",
		"compliance",
		"privacy",
		"security",
	}
	for _, signal := range signals {
		if !strings.Contains(normalized, signal) {
			continue
		}

		return true
	}

	return false
}

func sectionHasExplicitClaims(section project.SpecSection) bool {
	return len(section.Claims) > 0
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}

	return out
}
