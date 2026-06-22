package specflow

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestReviewSpecificationSet_FindsMissingBearerAndSupport(t *testing.T) {
	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			{
				ID:            "ES.agent-policy.001",
				Spec:          "enabling-system",
				SystemFrame:   project.SystemReferenceFrame{ID: "enabling_system", Kind: "enabling_system", Source: "test"},
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
	reading := packet.Sections[0].StateReading
	if reading.Profile != ReviewProfileSemanticV2 {
		t.Fatalf("state_reading.profile = %q", reading.Profile)
	}
	if reading.Bearer == "" || reading.Frame == "" || reading.Use == "" || reading.ReopenCondition == "" {
		t.Fatalf("state_reading must name bearer/frame/use/reopen condition: %+v", reading)
	}
	if reading.Reading == "ready" || reading.Reading == "pass" || reading.Reading == "current" {
		t.Fatalf("state_reading uses unqualified reading: %+v", reading)
	}
}

func TestReviewSpecificationSet_ReturnsV2ProfileModelBoundaries(t *testing.T) {
	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{})

	if packet.Profile.ID != ReviewProfileSemanticV2 {
		t.Fatalf("profile.id = %q, want %q", packet.Profile.ID, ReviewProfileSemanticV2)
	}
	if packet.Profile.Authority != ReviewAuthority {
		t.Fatalf("profile.authority = %q, want %q", packet.Profile.Authority, ReviewAuthority)
	}

	assertReviewProfileInput(t, packet.Profile, "claim_register_v1", ReviewModelDispositionUsed)
	assertReviewProfileInput(t, packet.Profile, "system_reference_frame_v1", ReviewModelDispositionUsed)
	assertReviewProfileInput(t, packet.Profile, "state_readings_v1", ReviewModelDispositionUsed)
	assertReviewProfileInput(t, packet.Profile, "publication_unit_v1", ReviewModelDispositionBoundaryPreserved)
	assertReviewProfileInput(t, packet.Profile, "transformation_record_v1", ReviewModelDispositionBoundaryPreserved)
	assertReviewProfileInput(t, packet.Profile, "value_slice", ReviewModelDispositionAbstain)
}

func TestReviewSpecificationSet_BlocksFrameMismatch(t *testing.T) {
	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			{
				ID:            "TS.boundary.001",
				Spec:          "enabling-system",
				SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "test"},
				DocumentKind:  "enabling-system",
				Kind:          "boundary",
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

func TestReviewSpecificationSet_BlocksLicensingPlatformWithoutExplicitClaims(t *testing.T) {
	section := reviewSectionFixture(
		"ES.licensing-platform.001",
		"enabling-system",
		"enabling.licensing_platform",
		"Licensing platform policy",
		"definition",
		"object",
		nil,
	)
	section.Claims = nil
	section.EvidenceRequired = []project.SpecEvidenceRequirement{
		{Kind: "review", Description: "Human confirms the section still holds."},
	}

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	})

	assertReviewFinding(t, packet, "unknown_high_risk_without_explicit_claims", ReviewSeverityBlockedForStrongerUse)
	if packet.Sections[0].StrongerUse != ReviewUseBlockedForStrongerUse {
		t.Fatalf("stronger_use = %q, want blocked", packet.Sections[0].StrongerUse)
	}
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

func TestReviewSpecificationSet_BlocksDescriptionTreatedAsAuthority(t *testing.T) {
	section := reviewSectionFixture(
		"ES.explanation-authority.001",
		"enabling-system",
		"enabling.agent_policy",
		"Explanation attached to work authority",
		"explanation",
		"work",
		nil,
	)
	section.EvidenceRequired = nil

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	})

	assertReviewFinding(t, packet, "authority_like_without_evidence_requirement", ReviewSeverityBlockedForStrongerUse)
	assertReviewFinding(t, packet, "description_use_confusion", ReviewSeverityWarn)
}

func TestReviewSpecificationSet_UsesDeclaredFrameInsteadOfKindPrefix(t *testing.T) {
	section := reviewSectionFixture(
		"TS.boundary.001",
		"enabling-system",
		"target.boundary",
		"Compatibility naming in enabling frame",
		"definition",
		"object",
		nil,
	)
	section.SystemFrame = project.SystemReferenceFrame{ID: "enabling_system", Kind: "enabling_system", Source: "test"}

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{section},
	})

	assertNoReviewFinding(t, packet, "system_frame_mismatch")
	if packet.Sections[0].Frame != "enabling_system" {
		t.Fatalf("frame = %q, want declared enabling_system", packet.Sections[0].Frame)
	}
}

func TestReviewSpecificationSet_UsesCarrierAndSidekickFrames(t *testing.T) {
	carrierSection := reviewSectionFixture(
		"TS.carrier-frame.001",
		"target-system",
		"target.publication_carrier",
		"Publication carrier view",
		"explanation",
		"description",
		nil,
	)
	carrierSection.SystemFrame = project.SystemReferenceFrame{ID: "carrier", Kind: "carrier", Source: "test"}

	sidekickSection := reviewSectionFixture(
		"ES.sidekick-frame.001",
		"enabling-system",
		"enabling.open_sleigh_sidekick",
		"Open-Sleigh sidekick view",
		"explanation",
		"description",
		nil,
	)
	sidekickSection.SystemFrame = project.SystemReferenceFrame{ID: "sidekick", Kind: "sidekick", Source: "test"}

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{carrierSection, sidekickSection},
	})

	assertNoReviewFinding(t, packet, "system_frame_mismatch")
	sections := reviewSectionsByID(packet.Sections)
	if sections["TS.carrier-frame.001"].Frame != "carrier" {
		t.Fatalf("carrier frame = %q, want carrier", sections["TS.carrier-frame.001"].Frame)
	}
	if sections["ES.sidekick-frame.001"].Frame != "sidekick" {
		t.Fatalf("sidekick frame = %q, want sidekick", sections["ES.sidekick-frame.001"].Frame)
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

func TestReviewSpecificationSet_CategorizesH9Findings(t *testing.T) {
	missingObject := reviewSectionFixture(
		"TS.missing-object.001",
		"target-system",
		"",
		"",
		"definition",
		"object",
		nil,
	)
	frameMismatch := reviewSectionFixture(
		"TS.frame.001",
		"enabling-system",
		"target.boundary",
		"Frame mismatch",
		"definition",
		"object",
		nil,
	)
	frameMismatch.SystemFrame = project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "test"}
	carrierLayer := reviewSectionFixture(
		"TS.carrier.001",
		"target-system",
		"target.publication_carrier",
		"Carrier layer",
		"explanation",
		"carrier",
		nil,
	)
	descriptionAuthority := reviewSectionFixture(
		"ES.description-authority.001",
		"enabling-system",
		"enabling.agent_policy",
		"Description authority",
		"explanation",
		"work",
		nil,
	)
	highRiskUnknown := reviewSectionFixture(
		"ES.licensing.001",
		"enabling-system",
		"enabling.licensing_platform",
		"Licensing platform",
		"definition",
		"object",
		nil,
	)
	claimPosture := reviewSectionFixture(
		"TS.claim.001",
		"target-system",
		"target.claim",
		"Claim posture",
		"definition",
		"object",
		nil,
	)
	claimPosture.Claims = []project.SpecClaim{{
		ID:        "TS.claim.001.mixed",
		Class:     "A/D",
		Statement: "One sentence acts as gate and duty.",
	}}

	packet := ReviewSpecificationSet(project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			missingObject,
			frameMismatch,
			carrierLayer,
			descriptionAuthority,
			highRiskUnknown,
			claimPosture,
		},
	})

	assertReviewFindingCategory(t, packet, "missing_bearer", ReviewFindingCategoryPrimaryObject)
	assertReviewFindingCategory(t, packet, "system_frame_mismatch", ReviewFindingCategoryFrame)
	assertReviewFindingCategory(t, packet, "active_carrier_layer", ReviewFindingCategoryPublicationBoundary)
	assertReviewFindingCategory(t, packet, "description_use_confusion", ReviewFindingCategoryDescriptionBoundary)
	assertReviewFindingCategory(t, packet, "unknown_high_risk_without_explicit_claims", ReviewFindingCategoryUnknownAbstain)
	assertReviewFindingCategory(t, packet, "mixed_claim_unresolved", ReviewFindingCategoryClaimPosture)
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
		ID:   id,
		Spec: documentKind,
		SystemFrame: project.SystemReferenceFrame{
			ID:     strings.ReplaceAll(documentKind, "-", "_"),
			Kind:   strings.ReplaceAll(documentKind, "-", "_"),
			Source: "test",
		},
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

func assertReviewFindingCategory(
	t *testing.T,
	packet ReviewPacket,
	ruleID string,
	category string,
) {
	t.Helper()

	for _, finding := range packet.Findings {
		if finding.RuleID != ruleID {
			continue
		}
		if finding.Category != category {
			t.Fatalf("%s category = %q, want %q", ruleID, finding.Category, category)
		}
		return
	}

	t.Fatalf("finding %q not found in %#v", ruleID, packet.Findings)
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

func reviewSectionsByID(sections []ReviewSection) map[string]ReviewSection {
	out := make(map[string]ReviewSection, len(sections))
	for _, section := range sections {
		out[section.SectionID] = section
	}

	return out
}

func assertReviewProfileInput(
	t *testing.T,
	profile ReviewProfile,
	name string,
	disposition string,
) {
	t.Helper()

	for _, input := range profile.ModelInputs {
		if input.Name != name {
			continue
		}
		if input.Disposition != disposition {
			t.Fatalf("%s disposition = %q, want %q", name, input.Disposition, disposition)
		}
		if input.Reading == "" {
			t.Fatalf("%s reading is empty in %+v", name, profile)
		}
		return
	}

	t.Fatalf("profile input %q not found in %+v", name, profile)
}
