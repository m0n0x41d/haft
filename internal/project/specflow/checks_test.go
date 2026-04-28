package specflow

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestRequireFieldFlagsMissingFieldOnly(t *testing.T) {
	check := RequireField{Field: "owner"}
	section := project.SpecSection{ID: "tgt-1"}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeFieldMissing {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeFieldMissing)
	}
	if findings[0].FieldPath != "owner" {
		t.Fatalf("field_path = %q, want %q", findings[0].FieldPath, "owner")
	}
}

func TestRequireFieldPassesWhenSet(t *testing.T) {
	check := RequireField{Field: "owner"}
	section := project.SpecSection{ID: "tgt-1", Owner: "human"}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(findings))
	}
}

func TestRequireStatementTypeRejectsUnknownValue(t *testing.T) {
	check := RequireStatementType{}
	section := project.SpecSection{ID: "tgt-1", StatementType: "totally-made-up"}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeStatementTypeInvalid {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeStatementTypeInvalid)
	}
}

func TestRequireStatementTypeAcceptsCanonicalValues(t *testing.T) {
	check := RequireStatementType{}
	for _, value := range ValidStatementTypes {
		section := project.SpecSection{ID: "tgt-1", StatementType: value}
		findings := check.RunOn(section, project.ProjectSpecificationSet{})
		if len(findings) != 0 {
			t.Fatalf("statement_type %q should pass, got %d findings", value, len(findings))
		}
	}
}

func TestRequireClaimLayerRejectsUnknownValue(t *testing.T) {
	check := RequireClaimLayer{}
	section := project.SpecSection{ID: "tgt-1", ClaimLayer: "L7"}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeClaimLayerInvalid {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeClaimLayerInvalid)
	}
}

func TestRequireClaimLayerAcceptsCanonicalValues(t *testing.T) {
	check := RequireClaimLayer{}
	for _, value := range ValidClaimLayers {
		section := project.SpecSection{ID: "tgt-1", ClaimLayer: value}
		findings := check.RunOn(section, project.ProjectSpecificationSet{})
		if len(findings) != 0 {
			t.Fatalf("claim_layer %q should pass, got %d findings", value, len(findings))
		}
	}
}

func TestRequireValidUntilRejectsGarbageDate(t *testing.T) {
	check := RequireValidUntil{}
	section := project.SpecSection{ID: "tgt-1", ValidUntil: "soon"}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeValidUntilUnparseable {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeValidUntilUnparseable)
	}
}

func TestRequireValidUntilAcceptsRFC3339AndShortDate(t *testing.T) {
	check := RequireValidUntil{}
	for _, value := range []string{"2026-10-28", "2026-10-28T12:00:00Z"} {
		section := project.SpecSection{ID: "tgt-1", ValidUntil: value}
		findings := check.RunOn(section, project.ProjectSpecificationSet{})
		if len(findings) != 0 {
			t.Fatalf("valid_until %q should pass, got %d findings", value, len(findings))
		}
	}
}

func TestRequireTermDefinedFlagsUndefinedTerm(t *testing.T) {
	check := RequireTermDefined{}
	section := project.SpecSection{ID: "tgt-1", Terms: []string{"Harnessability", "Mystery"}}
	set := project.ProjectSpecificationSet{
		TermMapEntries: []project.TermMapEntry{
			{Term: "Harnessability"},
		},
	}

	findings := check.RunOn(section, set)

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeTermNotDefined {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeTermNotDefined)
	}
	if !strings.Contains(findings[0].Message, "Mystery") {
		t.Fatalf("message %q should mention undefined term", findings[0].Message)
	}
}

func TestRequireTermDefinedIsCaseInsensitive(t *testing.T) {
	check := RequireTermDefined{}
	section := project.SpecSection{ID: "tgt-1", Terms: []string{"harnessability"}}
	set := project.ProjectSpecificationSet{
		TermMapEntries: []project.TermMapEntry{{Term: "Harnessability"}},
	}

	findings := check.RunOn(section, set)

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(findings))
	}
}

func TestRequireGuardLocationRejectsEmptyEvidence(t *testing.T) {
	check := RequireGuardLocation{}
	section := project.SpecSection{ID: "tgt-1"}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeGuardLocationMissing {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeGuardLocationMissing)
	}
}

func TestRequireGuardLocationRejectsUnknownKind(t *testing.T) {
	check := RequireGuardLocation{}
	section := project.SpecSection{
		ID: "tgt-1",
		EvidenceRequired: []project.SpecEvidenceRequirement{
			{Kind: "vibes"},
		},
	}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeGuardLocationMissing {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeGuardLocationMissing)
	}
}

func TestRequireGuardLocationAcceptsCanonicalKinds(t *testing.T) {
	check := RequireGuardLocation{}
	for _, kind := range ValidGuardLocations {
		section := project.SpecSection{
			ID: "tgt-1",
			EvidenceRequired: []project.SpecEvidenceRequirement{{Kind: kind}},
		}
		findings := check.RunOn(section, project.ProjectSpecificationSet{})
		if len(findings) != 0 {
			t.Fatalf("guard_location %q should pass, got %d findings", kind, len(findings))
		}
	}
}

func TestRequireBoundaryPerspectivesEnforcesMinimum(t *testing.T) {
	check := RequireBoundaryPerspectives{Min: 4}
	section := project.SpecSection{ID: "tgt-1", TargetRefs: []string{"law", "deontics"}}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Code != codeBoundaryPerspectivesMissing {
		t.Fatalf("code = %q, want %q", findings[0].Code, codeBoundaryPerspectivesMissing)
	}
}

func TestRequireBoundaryPerspectivesPassesAtMinimum(t *testing.T) {
	check := RequireBoundaryPerspectives{Min: 4}
	section := project.SpecSection{
		ID:         "tgt-1",
		TargetRefs: []string{"law", "admissibility", "deontics", "evidence"},
	}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(findings))
	}
}

func TestRequireBoundaryPerspectivesDefaultMinIsFour(t *testing.T) {
	check := RequireBoundaryPerspectives{} // Min == 0 -> defaults to 4
	section := project.SpecSection{
		ID:         "tgt-1",
		TargetRefs: []string{"law", "admissibility", "deontics"},
	}

	findings := check.RunOn(section, project.ProjectSpecificationSet{})

	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if !strings.Contains(findings[0].Message, "at least 4") {
		t.Fatalf("default min should be 4, message = %q", findings[0].Message)
	}
}
