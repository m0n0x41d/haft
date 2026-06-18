package specflow

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestReviewSpecificationSet_FindsMissingBearerAndSupport(t *testing.T) {
	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			{
				ID:            "ES.agent-policy.001",
				Spec:          "enabling-system",
				DocumentKind:  "enabling-system",
				Kind:          "enabling.agent_policy",
				StatementType: "duty",
				ClaimLayer:    "work",
				Owner:         "human",
				Status:        string(project.SpecSectionStateActive),
				Path:          ".haft/specs/enabling-system.md",
				Line:          7,
			},
		},
	})

	assertReviewFinding(t, packet, "missing_bearer", ReviewSeverityWarn)
	assertReviewFinding(t, packet, "strong_claim_without_support", ReviewSeverityWarn)
	assertReviewFinding(t, packet, "authority_like_without_evidence_requirement", ReviewSeverityBlockedForStrongerUse)

	if packet.Findings[0].FPFHint.AgentAction == "" {
		t.Fatalf("first finding has empty agent action: %+v", packet.Findings[0])
	}
	if packet.Sections[0].StrongerUse != ReviewUseBlockedForStrongerUse {
		t.Fatalf("stronger_use = %q, want %q", packet.Sections[0].StrongerUse, ReviewUseBlockedForStrongerUse)
	}
}

func TestReviewSpecificationSet_BlocksFrameMismatch(t *testing.T) {
	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			{
				ID:            "TS.boundary.001",
				Spec:          "enabling-system",
				DocumentKind:  "enabling-system",
				Kind:          "target.boundary",
				Title:         "Boundary in wrong carrier",
				StatementType: "definition",
				ClaimLayer:    "object",
				Owner:         "human",
				Status:        string(project.SpecSectionStateActive),
				Path:          ".haft/specs/enabling-system.md",
				Line:          12,
				EvidenceRequired: []project.SpecEvidenceRequirement{
					{Kind: "review", Description: "Human confirms the frame."},
				},
			},
		},
	})

	assertReviewFinding(t, packet, "system_frame_mismatch", ReviewSeverityBlockedForStrongerUse)
}

func TestReviewSpecificationSet_DoesNotFalseBlockLegitimateMultiView(t *testing.T) {
	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			reviewSectionFixture(
				"TS.boundary.001",
				"target-system",
				"target.boundary",
				"Target boundary",
				"definition",
				"object",
				[]string{"shared.object"},
			),
			reviewSectionFixture(
				"ES.effect-boundaries.001",
				"enabling-system",
				"enabling.effect_boundaries",
				"Enabling effect boundary",
				"duty",
				"work",
				[]string{"shared.object"},
			),
		},
	})

	for _, finding := range packet.Findings {
		if finding.Severity == ReviewSeverityBlockedForStrongerUse {
			t.Fatalf("unexpected stronger-use block for multi-view fixture: %+v", finding)
		}
	}
	if packet.Summary.CheckedSections != 2 {
		t.Fatalf("checked_sections = %d, want 2", packet.Summary.CheckedSections)
	}
}

func TestReviewSpecificationSet_UsesExplicitClaimSupport(t *testing.T) {
	section := reviewSectionFixture(
		"ES.gate.001",
		"enabling-system",
		"enabling.gate",
		"Explicit claim support",
		"duty",
		"work",
		nil,
	)
	section.EvidenceRequired = nil
	section.Claims = []project.SpecClaim{
		{
			ID:          "ES.gate.001.A1",
			Class:       "A",
			Statement:   "Gate is admissible only after human review.",
			SupportRefs: []string{"dec-20260618-example"},
		},
	}

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	})

	assertNoReviewFinding(t, packet, "strong_claim_without_support")
	assertNoReviewFinding(t, packet, "authority_like_without_evidence_requirement")
	if packet.Summary.ExplicitClaims != 1 || packet.Summary.DeclaredClaims != 1 {
		t.Fatalf("claim summary = %+v, want one declared explicit claim", packet.Summary)
	}
	if packet.Summary.MissingSupportClaims != 0 {
		t.Fatalf("missing_support_claims = %d, want 0", packet.Summary.MissingSupportClaims)
	}
	if len(packet.Sections[0].Claims) != 1 {
		t.Fatalf("review claims = %#v, want one claim", packet.Sections[0].Claims)
	}
	if packet.Sections[0].Claims[0].Class != ReviewClaimClassAdmissibility {
		t.Fatalf("claim class = %q, want %q", packet.Sections[0].Claims[0].Class, ReviewClaimClassAdmissibility)
	}
}

func TestReviewSpecificationSet_FindsExplicitClaimPostureIssues(t *testing.T) {
	section := reviewSectionFixture(
		"TS.claims.001",
		"target-system",
		"target.claims",
		"Explicit claim posture",
		"definition",
		"object",
		nil,
	)
	section.Claims = []project.SpecClaim{
		{
			ID:        "TS.claims.001.mixed",
			Class:     "A/D",
			Statement: "One sentence acts as gate and duty.",
		},
		{
			ID:        "TS.claims.001.unknown",
			Class:     "promise",
			Statement: "Unknown local class should not become canonical.",
		},
		{
			ID:        "TS.claims.001.gate",
			Class:     "A",
			Statement: "Gate claim lacks support.",
		},
	}

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	})

	assertReviewFinding(t, packet, "mixed_claim_unresolved", ReviewSeverityWarn)
	assertReviewFinding(t, packet, "claim_class_unresolved", ReviewSeverityAbstain)
	assertReviewFinding(t, packet, "declared_claim_without_support", ReviewSeverityBlockedForStrongerUse)
	if packet.Summary.ExplicitClaims != 3 {
		t.Fatalf("explicit_claims = %d, want 3", packet.Summary.ExplicitClaims)
	}
	if packet.Summary.MixedUnresolvedClaims != 1 {
		t.Fatalf("mixed_unresolved_claims = %d, want 1", packet.Summary.MixedUnresolvedClaims)
	}
	if packet.Summary.UnclassifiedClaims != 1 {
		t.Fatalf("unclassified_claims = %d, want 1", packet.Summary.UnclassifiedClaims)
	}
	if packet.Summary.MissingSupportClaims != 1 {
		t.Fatalf("missing_support_claims = %d, want 1", packet.Summary.MissingSupportClaims)
	}
	if packet.Sections[0].StrongerUse != ReviewUseBlockedForStrongerUse {
		t.Fatalf("stronger_use = %q, want blocked", packet.Sections[0].StrongerUse)
	}
}

func reviewSectionFixture(
	id string,
	documentKind string,
	kind string,
	title string,
	statementType string,
	claimLayer string,
	targetRefs []string,
) project.SpecSection {
	return project.SpecSection{
		ID:            id,
		Spec:          documentKind,
		DocumentKind:  documentKind,
		Kind:          kind,
		Title:         title,
		StatementType: statementType,
		ClaimLayer:    claimLayer,
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		TargetRefs:    targetRefs,
		Path:          ".haft/specs/" + documentKind + ".md",
		Line:          4,
		EvidenceRequired: []project.SpecEvidenceRequirement{
			{Kind: "review", Description: "Human confirms the section still holds."},
		},
	}
}

func assertReviewFinding(
	t *testing.T,
	packet ReviewPacket,
	ruleID string,
	severity string,
) {
	t.Helper()

	for _, finding := range packet.Findings {
		if finding.RuleID != ruleID {
			continue
		}
		if finding.Severity != severity {
			t.Fatalf("%s severity = %q, want %q", ruleID, finding.Severity, severity)
		}
		return
	}

	t.Fatalf("finding %q not found in %#v", ruleID, packet.Findings)
}

func assertNoReviewFinding(
	t *testing.T,
	packet ReviewPacket,
	ruleID string,
) {
	t.Helper()

	for _, finding := range packet.Findings {
		if finding.RuleID == ruleID {
			t.Fatalf("unexpected finding %q in %#v", ruleID, packet.Findings)
		}
	}
}
