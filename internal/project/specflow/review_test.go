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
