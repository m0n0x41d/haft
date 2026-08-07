package typeenv

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSourceLocationPreservesExactSourceUnitCoordinates(t *testing.T) {
	unit := sourceBasisFixture()
	location, err := sourceLocation(unit)
	if err != nil {
		t.Fatalf("sourceLocation(): %v", err)
	}
	if location.UnitID().String() != unit.UnitID {
		t.Fatalf("UnitID = %q, want %q", location.UnitID().String(), unit.UnitID)
	}
	if location.Revision().String() != unit.Provenance.SourceRevision {
		t.Fatalf("revision = %q, want %q", location.Revision().String(), unit.Provenance.SourceRevision)
	}
	if location.ContentHash().String() != "sha256:"+unit.Provenance.ContentHash {
		t.Fatalf("content hash = %q", location.ContentHash().String())
	}
	patternID, present := location.PatternID()
	if !present || patternID.String() != "A.6.5" {
		t.Fatalf("PatternID = %q, %v; want A.6.5, true", patternID.String(), present)
	}
}

func TestSourceLocationRejectsInvalidCoordinates(t *testing.T) {
	unit := sourceBasisFixture()
	unit.Provenance.StartLine = 0
	if _, err := sourceLocation(unit); err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("invalid line range error = %v", err)
	}

	unit = sourceBasisFixture()
	unit.Provenance.ContentHash = "not-a-digest"
	if _, err := sourceLocation(unit); err == nil {
		t.Fatal("sourceLocation accepted an invalid content hash")
	}
}

func TestSourceFPFProvenanceRetainsCompilerRule(t *testing.T) {
	ruleID, err := typedmemory.NewCompilerRuleID("fpf.slot-production.v1")
	if err != nil {
		t.Fatalf("NewCompilerRuleID(): %v", err)
	}
	provenance, err := sourceFPFProvenance(sourceBasisFixture(), ruleID)
	if err != nil {
		t.Fatalf("sourceFPFProvenance(): %v", err)
	}
	if provenance.CompilerRuleID() != ruleID {
		t.Fatal("source provenance lost compiler rule identity")
	}
}

func sourceBasisFixture() fpf.SourceUnit {
	return fpf.SourceUnit{
		UnitID:    "spec:pattern_section:a-6-5:1159",
		Role:      fpf.SourceUnitRolePatternSection,
		PatternID: "A.6.5",
		Provenance: fpf.SourceProvenance{
			SourcePath:     "data/FPF/FPF-Spec.md",
			StartLine:      16603,
			EndLine:        16613,
			ContentHash:    strings.Repeat("a", 64),
			SourceRevision: strings.Repeat("b", 40),
		},
	}
}
