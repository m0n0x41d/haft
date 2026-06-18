package specflow

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	ReviewClaimClassLaw           = "L"
	ReviewClaimClassAdmissibility = "A"
	ReviewClaimClassDeontic       = "D"
	ReviewClaimClassWorkEffect    = "E"

	ReviewClaimPostureDeclared        = "declared"
	ReviewClaimPostureMixedUnresolved = "mixed_unresolved"
	ReviewClaimPostureUnclassified    = "unclassified"
)

type ClaimRegisterSummary struct {
	ExplicitClaims        int `json:"explicit_claims"`
	DeclaredClaims        int `json:"declared_claims"`
	MixedUnresolvedClaims int `json:"mixed_unresolved_claims"`
	UnclassifiedClaims    int `json:"unclassified_claims"`
	MissingSupportClaims  int `json:"missing_support_claims"`
}

type ReviewClaim struct {
	ID                   string     `json:"id"`
	Class                string     `json:"class,omitempty"`
	ClassName            string     `json:"class_name,omitempty"`
	RawClass             string     `json:"raw_class,omitempty"`
	Statement            string     `json:"statement,omitempty"`
	Posture              string     `json:"posture"`
	StrongerUse          string     `json:"stronger_use"`
	Scope                []string   `json:"scope,omitempty"`
	SupportRefs          []string   `json:"support_refs,omitempty"`
	EvidenceRefs         []string   `json:"evidence_refs,omitempty"`
	ValidUntil           string     `json:"valid_until,omitempty"`
	GoverningPatternRefs []string   `json:"governing_pattern_refs,omitempty"`
	Source               SourceSpan `json:"source"`
	FindingCodes         []string   `json:"finding_codes,omitempty"`
}

func reviewClaims(subject reviewSubject) []ReviewClaim {
	claims := make([]ReviewClaim, 0, len(subject.section.Claims))

	for index, claim := range subject.section.Claims {
		claims = append(claims, reviewClaim(subject.section, claim, index))
	}

	return claims
}

func reviewClaim(section project.SpecSection, claim project.SpecClaim, index int) ReviewClaim {
	class, className, posture := classifyReviewClaim(claim.Class)

	return ReviewClaim{
		ID:                   strings.TrimSpace(claim.ID),
		Class:                class,
		ClassName:            className,
		RawClass:             strings.TrimSpace(claim.Class),
		Statement:            strings.TrimSpace(claim.Statement),
		Posture:              posture,
		StrongerUse:          claimStrongerUse(class, posture),
		Scope:                append([]string(nil), claim.Scope...),
		SupportRefs:          append([]string(nil), claim.SupportRefs...),
		EvidenceRefs:         append([]string(nil), claim.EvidenceRefs...),
		ValidUntil:           strings.TrimSpace(claim.ValidUntil),
		GoverningPatternRefs: append([]string(nil), claim.GoverningPatternRefs...),
		Source:               sectionSource(section, fmt.Sprintf("claims[%d]", index)),
	}
}

func classifyReviewClaim(raw string) (string, string, string) {
	classes := reviewClaimClasses(raw)
	if len(classes) == 1 {
		class := classes[0]

		return class, reviewClaimClassName(class), ReviewClaimPostureDeclared
	}
	if len(classes) > 1 {
		return strings.Join(classes, "+"), "mixed L/A/D/E claim", ReviewClaimPostureMixedUnresolved
	}

	return "", "", ReviewClaimPostureUnclassified
}

func reviewClaimClasses(raw string) []string {
	tokens := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '/' || r == '+' || r == '&' || unicode.IsSpace(r)
	})

	classes := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		class := reviewClaimClassToken(token)
		if class == "" {
			continue
		}
		if _, ok := seen[class]; ok {
			continue
		}

		seen[class] = struct{}{}
		classes = append(classes, class)
	}

	return classes
}

func reviewClaimClassToken(token string) string {
	normalized := strings.ToUpper(strings.TrimSpace(token))
	normalized = strings.ReplaceAll(normalized, "_", "-")

	switch normalized {
	case "L", "LAW", "LAWS", "DEFINITION", "DEFINITIONS":
		return ReviewClaimClassLaw
	case "A", "ADMISSIBILITY", "ADMISSIBLE", "GATE", "GATES":
		return ReviewClaimClassAdmissibility
	case "D", "DEONTIC", "DEONTICS", "DUTY", "DUTIES", "COMMITMENT", "COMMITMENTS":
		return ReviewClaimClassDeontic
	case "E", "EFFECT", "EFFECTS", "WORK-EFFECT", "WORK-EFFECTS":
		return ReviewClaimClassWorkEffect
	default:
		return ""
	}
}

func reviewClaimClassName(class string) string {
	switch class {
	case ReviewClaimClassLaw:
		return "Laws / Definitions"
	case ReviewClaimClassAdmissibility:
		return "Admissibility / Gate"
	case ReviewClaimClassDeontic:
		return "Deontics"
	case ReviewClaimClassWorkEffect:
		return "Work-Effects"
	default:
		return ""
	}
}

func claimStrongerUse(class string, posture string) string {
	if posture != ReviewClaimPostureDeclared {
		return ReviewUseAbstainUntilClarified
	}
	if class == ReviewClaimClassAdmissibility {
		return ReviewUseBlockedForAuthorityUse
	}

	return ReviewUseDocumentaryReading
}

func claimRegisterSummary(claims []ReviewClaim, findings []ReviewFinding) ClaimRegisterSummary {
	summary := ClaimRegisterSummary{
		ExplicitClaims: len(claims),
	}

	for _, claim := range claims {
		switch claim.Posture {
		case ReviewClaimPostureDeclared:
			summary.DeclaredClaims++
		case ReviewClaimPostureMixedUnresolved:
			summary.MixedUnresolvedClaims++
		case ReviewClaimPostureUnclassified:
			summary.UnclassifiedClaims++
		}
	}
	for _, finding := range findings {
		if finding.RuleID == "declared_claim_without_support" {
			summary.MissingSupportClaims++
		}
	}

	return summary
}

func claimRegisterFindings(subject reviewSubject) []ReviewFinding {
	claims := reviewClaims(subject)
	findings := make([]ReviewFinding, 0, len(claims))

	for _, claim := range claims {
		findings = append(findings, claimPostureFindings(subject, claim)...)
		findings = append(findings, claimSupportFindings(subject, claim)...)
	}

	return findings
}

func claimPostureFindings(subject reviewSubject, claim ReviewClaim) []ReviewFinding {
	switch claim.Posture {
	case ReviewClaimPostureMixedUnresolved:
		return []ReviewFinding{
			newClaimReviewFinding(
				subject,
				claim,
				"mixed_claim_unresolved",
				ReviewSeverityWarn,
				fmt.Sprintf("claim %q declares mixed L/A/D/E classes %q", claim.ID, claim.RawClass),
				"A load-bearing sentence cannot safely act as law, gate, duty, and work/evidence relation at the same time.",
				"Boundary Norm Square (L/A/D/E)",
				"Split this claim into one stable claim id per L/A/D/E class before stronger use.",
				ReviewUseAbstainUntilClarified,
				"class",
			),
		}
	case ReviewClaimPostureUnclassified:
		return []ReviewFinding{
			newClaimReviewFinding(
				subject,
				claim,
				"claim_class_unresolved",
				ReviewSeverityAbstain,
				fmt.Sprintf("claim %q does not declare a resolvable L/A/D/E class", claim.ID),
				"Spec review can only route support checks when the claim's role is explicit.",
				"Classify every load-bearing sentence",
				"Set `class` to one of L, A, D, or E, or keep the section as documentary-only.",
				ReviewUseAbstainUntilClarified,
				"class",
			),
		}
	default:
		return nil
	}
}

func claimSupportFindings(subject reviewSubject, claim ReviewClaim) []ReviewFinding {
	if claim.Posture != ReviewClaimPostureDeclared {
		return nil
	}
	if !claimRequiresSupport(claim.Class) {
		return nil
	}
	if claimHasSupport(claim) {
		return nil
	}

	severity := ReviewSeverityWarn
	strongerUse := ReviewUseAbstainUntilClarified
	if claim.Class == ReviewClaimClassAdmissibility {
		severity = ReviewSeverityBlockedForStrongerUse
		strongerUse = ReviewUseBlockedForAuthorityUse
	}

	return []ReviewFinding{
		newClaimReviewFinding(
			subject,
			claim,
			"declared_claim_without_support",
			severity,
			fmt.Sprintf("claim %q (%s) has no claim-level support refs", claim.ID, claim.Class),
			"Explicit claims need claim-scoped support so a section-level description does not impersonate evidence or authority.",
			"Evidence/support refs required for stronger use",
			"Add `support_refs`, `evidence_refs`, or `governing_pattern_refs` to this claim, or downgrade its class/use.",
			strongerUse,
			"support_refs",
		),
	}
}

func claimRequiresSupport(class string) bool {
	switch class {
	case ReviewClaimClassAdmissibility, ReviewClaimClassDeontic, ReviewClaimClassWorkEffect:
		return true
	default:
		return false
	}
}

func claimHasSupport(claim ReviewClaim) bool {
	if len(claim.SupportRefs) > 0 {
		return true
	}
	if len(claim.EvidenceRefs) > 0 {
		return true
	}
	if len(claim.GoverningPatternRefs) > 0 {
		return true
	}

	return false
}

func newClaimReviewFinding(
	subject reviewSubject,
	claim ReviewClaim,
	ruleID string,
	severity string,
	finding string,
	whyItMatters string,
	principle string,
	agentAction string,
	strongerUse string,
	field string,
) ReviewFinding {
	source := claim.Source
	if strings.TrimSpace(field) != "" {
		source.FieldPath = joinReviewFieldPath(source.FieldPath, field)
	}

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
		Source: source,
	}
}

func joinReviewFieldPath(path string, field string) string {
	trimmedPath := strings.TrimSpace(path)
	trimmedField := strings.TrimSpace(field)
	if trimmedPath == "" {
		return "$." + trimmedField
	}
	if strings.HasSuffix(trimmedPath, "]") {
		return trimmedPath + "." + trimmedField
	}

	return trimmedPath + "." + trimmedField
}
