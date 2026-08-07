package typedmemory

import (
	"bytes"
	"testing"
)

func TestFPFSourceProvenancePreservesExactSourceCoordinates(t *testing.T) {
	provenance := typeEnvTestFPFProvenance(t, "prov:fpf:slot", 0x11)

	patternID, hasPattern := provenance.Location().PatternID()
	if !hasPattern || patternID.String() != "A.6.5" {
		t.Fatalf("PatternID = %q, %v; want A.6.5, true", patternID.String(), hasPattern)
	}
	if provenance.Location().LineRange().Start() != 16540 {
		t.Fatalf("line start = %d; want 16540", provenance.Location().LineRange().Start())
	}
	if provenance.Location().LineRange().End() != 16610 {
		t.Fatalf("line end = %d; want 16610", provenance.Location().LineRange().End())
	}
	if len(provenance.CanonicalBytes()) == 0 {
		t.Fatal("canonical provenance bytes are empty")
	}
	if !bytes.Equal(provenance.CanonicalBytes(), provenance.CanonicalBytes()) {
		t.Fatal("canonical provenance bytes are not deterministic")
	}
}

func TestSourceLocationMakesPatternAbsenceExplicit(t *testing.T) {
	unitID := typeEnvTestSourceUnitID(t, "spec:preface")
	revision := typeEnvTestSourceRevision(t, "fpf-revision")
	digest := typeEnvTestDigest(t, 0x12)
	lineRange := typeEnvTestLineRange(t, 10, 20)

	location, err := NewUnpatternedSourceLocation(unitID, revision, digest, lineRange)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation() error = %v", err)
	}
	if _, hasPattern := location.PatternID(); hasPattern {
		t.Fatal("unpatterned source location fabricated a PatternID")
	}
	if _, err := NewSourceLineRange(5, 4); err == nil {
		t.Fatal("NewSourceLineRange() accepted a reversed range")
	}
}

func TestCompilerDerivedProvenanceIsCanonicalAcrossInputPermutation(t *testing.T) {
	first := typeEnvTestFPFProvenance(t, "prov:fpf:first", 0x21).Location()
	second := typeEnvTestFPFProvenance(t, "prov:fpf:second", 0x22).Location()
	reference := typeEnvTestProvenanceRef(t, "prov:compiler:claim-graph-lowering")
	rule := typeEnvTestCompilerRuleID(t, "compiler.claim-graph-codec.v1")

	forward, err := NewCompilerDerivedProvenance(
		reference,
		[]SourceLocation{first, second},
		rule,
	)
	if err != nil {
		t.Fatalf("NewCompilerDerivedProvenance(forward): %v", err)
	}
	reverse, err := NewCompilerDerivedProvenance(
		reference,
		[]SourceLocation{second, first},
		rule,
	)
	if err != nil {
		t.Fatalf("NewCompilerDerivedProvenance(reverse): %v", err)
	}

	if !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("source-input permutation changed compiler-derived provenance")
	}
	if len(forward.Inputs()) != 2 {
		t.Fatalf("Inputs() length = %d; want 2", len(forward.Inputs()))
	}
	if forward.CompilerRuleID() != rule {
		t.Fatal("compiler-derived provenance lost its compiler rule")
	}

	kindID := typeEnvTestKindID(t, "U.ClaimGraph")
	if _, err := NewKindDefinition(kindID, forward); err != nil {
		t.Fatalf("compiler-derived provenance rejected by sealed declaration union: %v", err)
	}
}

func TestCompilerDerivedProvenanceRejectsMissingOrDuplicateSourceBasis(t *testing.T) {
	input := typeEnvTestFPFProvenance(t, "prov:fpf:input", 0x23).Location()
	reference := typeEnvTestProvenanceRef(t, "prov:compiler:test")
	rule := typeEnvTestCompilerRuleID(t, "compiler.test.v1")

	if _, err := NewCompilerDerivedProvenance(reference, nil, rule); err == nil {
		t.Fatal("compiler-derived provenance accepted no exact source inputs")
	}
	if _, err := NewCompilerDerivedProvenance(
		reference,
		[]SourceLocation{input, input},
		rule,
	); err == nil {
		t.Fatal("compiler-derived provenance accepted duplicate source inputs")
	}
}

func TestProjectSourceProvenanceRequiresCompleteExtensionBasis(t *testing.T) {
	base := typeEnvTestTypeEnvRef(t, 0x13)
	context := typeEnvTestContextRef(t, "ctx:haft")
	carrier := typeEnvTestCarrierRef(t, "carrier:.haft/local-practice.md")
	edition := typeEnvTestCarrierEdition(t, "2026-07-16")
	digest := typeEnvTestDigest(t, 0x14)
	reference := typeEnvTestProvenanceRef(t, "prov:project:kind")

	_, err := NewProjectSourceProvenanceBuilder(reference, carrier, edition, digest).Build()
	if err == nil {
		t.Fatal("incomplete project provenance unexpectedly built")
	}

	symbol := typeEnvTestKindSymbol(t, "Haft.ProjectMemory")
	manifest := typeEnvTestManifestRef(t)
	basis, err := NewManifestSymbolBasis(manifest, ManifestProvide, symbol)
	if err != nil {
		t.Fatalf("NewManifestSymbolBasis() error = %v", err)
	}
	provenance, err := NewProjectSourceProvenanceBuilder(reference, carrier, edition, digest).
		SetDeclarationRange(typeEnvTestLineRange(t, 30, 42)).
		SetCompilerRule(typeEnvTestCompilerRuleID(t, "local-practice.kind.v1")).
		SetBoundedContext(context).
		SetBaseTypeEnv(base).
		SetSignatureBlockRow(VocabularyRow).
		SetManifestBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if provenance.BaseTypeEnv() != base || provenance.BoundedContext() != context {
		t.Fatal("project provenance lost its exact base or bounded context")
	}
}

func TestSignatureManifestRefStringIsInjectiveAcrossDelimiterBoundaries(t *testing.T) {
	left, err := NewSignatureManifestRef("manifest@v1", "edition")
	if err != nil {
		t.Fatalf("NewSignatureManifestRef(left): %v", err)
	}
	right, err := NewSignatureManifestRef("manifest", "v1@edition")
	if err != nil {
		t.Fatalf("NewSignatureManifestRef(right): %v", err)
	}
	if left == right {
		t.Fatal("distinct signature-manifest references became equal")
	}
	if left.String() == right.String() {
		t.Fatalf("ambiguous signature-manifest strings: %q", left.String())
	}
}
