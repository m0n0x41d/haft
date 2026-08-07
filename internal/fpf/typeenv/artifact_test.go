package typeenv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSealBaseTypeEnvIsDeterministicAndRoundTrips(t *testing.T) {
	t.Parallel()

	fixture := newArtifactFixture(t)
	entity := fixture.declaration(t, "U.Entity", fixture.location(t, "body-a6", 10, 20), nil)
	entityReference := fixture.refKindDeclaration(
		t,
		"U.EntityRef",
		entity,
		fixture.location(t, "body-a14", 30, 42),
	)

	forward := fixture.compiledIR(t, []LinkedDeclaration{entity, entityReference})
	reverse := fixture.compiledIR(t, []LinkedDeclaration{entityReference, entity})
	first, err := SealBaseTypeEnv(forward)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(forward) error = %v", err)
	}
	second, err := SealBaseTypeEnv(reverse)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(reverse) error = %v", err)
	}
	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("permuting declarations changed canonical artifact bytes")
	}
	firstRef, firstHasRef := first.TypeEnvRef()
	secondRef, secondHasRef := second.TypeEnvRef()
	if !firstHasRef || !secondHasRef || firstRef.String() != secondRef.String() {
		t.Fatalf("derived refs differ: (%v, %q) vs (%v, %q)", firstHasRef, firstRef.String(), secondHasRef, secondRef.String())
	}
	if firstRef.Digest().String() != first.Digest().String() {
		t.Fatalf("TypeEnvRef digest = %q, artifact digest = %q", firstRef.Digest().String(), first.Digest().String())
	}

	wire, err := first.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}
	decoded, err := UnmarshalBaseTypeEnvArtifact(wire)
	if err != nil {
		t.Fatalf("UnmarshalBaseTypeEnvArtifact() error = %v", err)
	}
	if err := decoded.Verify(); err != nil {
		t.Fatalf("decoded.Verify() error = %v", err)
	}
	if !bytes.Equal(decoded.CanonicalBytes(), first.CanonicalBytes()) {
		t.Fatal("round trip changed canonical artifact bytes")
	}

	projections := decoded.DeclarationProjections()
	if len(projections) != 3 {
		t.Fatalf("projection count = %d, want context + kind + RefKind", len(projections))
	}
	referenceProjection, exists := decoded.SymbolManifest().Entry(entityReference.Symbol())
	if !exists {
		t.Fatal("entity RefKind projection missing")
	}
	dependencies := referenceProjection.Dependencies()
	if len(dependencies) != 1 || dependencies[0].String() != entity.Symbol().String() {
		t.Fatalf("RefKind dependencies = %v, want %q", dependencies, entity.Symbol().String())
	}
	if len(referenceProjection.SourceInputs()) != 1 {
		t.Fatalf("RefKind source input count = %d, want 1", len(referenceProjection.SourceInputs()))
	}
}

func TestCoverageOnlyArtifactDoesNotMintTypeEnvRef(t *testing.T) {
	t.Parallel()

	fixture := newArtifactFixture(t)
	location := fixture.location(t, "ambiguous-c21", 100, 130)
	unitSubject, err := typedmemory.SourceUnitCoverage(location.UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage() error = %v", err)
	}
	gap, err := typedmemory.NewSourceOnlyCoverageEntry(
		unitSubject,
		location,
		"reference_scheme_slot_cardinality_ambiguous",
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry() error = %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{gap})
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	ir, err := NewCoverageOnlyLinkedTypeEnvIR(
		fixture.revision,
		fixture.compiler,
		coverage,
		"no complete supported declaration",
	)
	if err != nil {
		t.Fatalf("NewCoverageOnlyLinkedTypeEnvIR() error = %v", err)
	}
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	if _, exists := artifact.TypeEnvRef(); exists {
		t.Fatal("coverage-only artifact minted a TypeEnvRef")
	}
	if artifact.Digest().String() == "" {
		t.Fatal("coverage-only artifact lacks its artifact digest")
	}
	if len(artifact.Declarations()) != 0 || len(artifact.DeclarationProjections()) != 0 {
		t.Fatal("coverage-only artifact contains an executable declaration projection")
	}
	_, exists, err := artifact.RecomputeIdentityRef()
	if err != nil {
		t.Fatalf("RecomputeIdentityRef() error = %v", err)
	}
	if exists {
		t.Fatal("recomputed coverage-only identity produced a TypeEnvRef")
	}

	wire, err := artifact.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}
	decoded, err := UnmarshalBaseTypeEnvArtifact(wire)
	if err != nil {
		t.Fatalf("UnmarshalBaseTypeEnvArtifact() error = %v", err)
	}
	if decoded.Posture() != CoverageOnly {
		t.Fatalf("decoded posture = %s, want coverage_only", decoded.Posture())
	}
}

func TestArtifactCodecRejectsPayloadAndRefTampering(t *testing.T) {
	t.Parallel()

	fixture := newArtifactFixture(t)
	declaration := fixture.declaration(t, "U.Entity", fixture.location(t, "body-a6", 10, 20), nil)
	ir := fixture.compiledIR(t, []LinkedDeclaration{declaration})
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	wire, err := artifact.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}

	payloadTampered := append([]byte(nil), wire...)
	needle := []byte("U.Entity")
	index := bytes.Index(payloadTampered, needle)
	if index < 0 {
		t.Fatal("test artifact does not contain expected declaration bytes")
	}
	payloadTampered[index] = 'V'
	if _, err := UnmarshalBaseTypeEnvArtifact(payloadTampered); err == nil {
		t.Fatal("payload tampering was accepted")
	}

	refTampered := append([]byte(nil), wire...)
	ref, _ := artifact.TypeEnvRef()
	refDigest := []byte(ref.Digest().String())
	index = bytes.LastIndex(refTampered, refDigest)
	if index < 0 {
		t.Fatal("test wire does not contain expected TypeEnvRef digest")
	}
	last := index + len(refDigest) - 1
	if refTampered[last] == '0' {
		refTampered[last] = '1'
	} else {
		refTampered[last] = '0'
	}
	if _, err := UnmarshalBaseTypeEnvArtifact(refTampered); err == nil || !strings.Contains(err.Error(), "tampered") {
		t.Fatalf("ref tampering error = %v, want explicit tamper rejection", err)
	}

	trailing := append(append([]byte(nil), wire...), 0)
	if _, err := UnmarshalBaseTypeEnvArtifact(trailing); err == nil {
		t.Fatal("wire trailing bytes were accepted")
	}
}

func TestCompiledIRRejectsPartialOrUnlinkedDeclarations(t *testing.T) {
	t.Parallel()

	fixture := newArtifactFixture(t)
	location := fixture.location(t, "body-a6", 10, 20)
	gapSubject, err := typedmemory.SourceUnitCoverage(location.UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage() error = %v", err)
	}
	gap, err := typedmemory.NewSourceOnlyCoverageEntry(gapSubject, location, "incomplete_declaration")
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry() error = %v", err)
	}
	gapCoverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{gap})
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	if _, err := NewCompiledLinkedTypeEnvIR(fixture.revision, fixture.compiler, gapCoverage, nil); err == nil {
		t.Fatal("zero-declaration compiled environment was accepted")
	}

	missingID, err := typedmemory.NewKindID("U.Missing")
	if err != nil {
		t.Fatalf("NewKindID() error = %v", err)
	}
	missingSymbol, err := typedmemory.KindSymbolRef(missingID)
	if err != nil {
		t.Fatalf("KindSymbolRef() error = %v", err)
	}
	declaration := fixture.declaration(t, "U.Entity", location, []typedmemory.SchemaSymbolRef{missingSymbol})
	coverage := fixture.coverage(t, []LinkedDeclaration{declaration})
	if _, err := NewCompiledLinkedTypeEnvIR(fixture.revision, fixture.compiler, coverage, []LinkedDeclaration{declaration}); err == nil {
		t.Fatal("unresolved declaration dependency was accepted")
	}
}

func TestCompatibilityAssessmentIsOutsideArtifactDigest(t *testing.T) {
	t.Parallel()

	fixture := newArtifactFixture(t)
	declaration := fixture.declaration(t, "U.Entity", fixture.location(t, "body-a6", 10, 20), nil)
	ir := fixture.compiledIR(t, []LinkedDeclaration{declaration})
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	baseDigest := mustDigest(t, strings.Repeat("a", 64))
	base, err := typedmemory.NewTypeEnvRef(baseDigest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	firstChange, err := typedmemory.NewCompatibilityChange(
		declaration.Symbol(),
		typedmemory.CompatibilityAdded,
		"first_assessment",
	)
	if err != nil {
		t.Fatalf("NewCompatibilityChange(first) error = %v", err)
	}
	secondChange, err := typedmemory.NewCompatibilityChange(
		declaration.Symbol(),
		typedmemory.CompatibilityChanged,
		"second_assessment",
	)
	if err != nil {
		t.Fatalf("NewCompatibilityChange(second) error = %v", err)
	}
	firstDiff, err := typedmemory.NewTypeEnvCompatibilityDiff(base, []typedmemory.CompatibilityChange{firstChange})
	if err != nil {
		t.Fatalf("NewTypeEnvCompatibilityDiff(first) error = %v", err)
	}
	secondDiff, err := typedmemory.NewTypeEnvCompatibilityDiff(base, []typedmemory.CompatibilityChange{secondChange})
	if err != nil {
		t.Fatalf("NewTypeEnvCompatibilityDiff(second) error = %v", err)
	}
	firstAssessment, _ := NewComparedCompatibilityAssessment(firstDiff)
	secondAssessment, _ := NewComparedCompatibilityAssessment(secondDiff)
	firstEnvelope, err := NewCompilationEnvelope(artifact, firstAssessment)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(first) error = %v", err)
	}
	secondEnvelope, err := NewCompilationEnvelope(artifact, secondAssessment)
	if err != nil {
		t.Fatalf("NewCompilationEnvelope(second) error = %v", err)
	}
	if firstEnvelope.Artifact().Digest().String() != secondEnvelope.Artifact().Digest().String() {
		t.Fatal("compatibility assessment changed artifact digest")
	}
	if !bytes.Equal(firstEnvelope.Artifact().CanonicalBytes(), secondEnvelope.Artifact().CanonicalBytes()) {
		t.Fatal("compatibility assessment changed canonical artifact bytes")
	}
}

func TestArtifactConstructionAndSealEnforceResourceBudgets(t *testing.T) {
	t.Parallel()

	tooManyValues := make([]DeclarationValue, maximumValuesPerCollection+1)
	for index := range tooManyValues {
		tooManyValues[index] = NewUnsignedValue(uint64(index))
	}
	if _, err := NewSequenceValue(tooManyValues); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized sequence error = %v, want budget rejection", err)
	}

	oversizedText := NewTextValue(strings.Repeat("x", maximumScalarStringBytes+1))
	if _, err := NewDeclarationField("oversized", oversizedText); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized text error = %v, want budget rejection", err)
	}

	fixture := newArtifactFixture(t)
	valid := fixture.declaration(t, "U.Entity", fixture.location(t, "body-a6", 10, 20), nil)
	ir := fixture.compiledIR(t, []LinkedDeclaration{valid})
	deep := DeclarationValue(NewTextValue("leaf"))
	for depth := 0; depth <= maximumDeclarationDepth; depth++ {
		deep = SequenceValue{values: []DeclarationValue{deep}}
	}
	ir.declarations[0].body = DeclarationBody{
		fields: []DeclarationField{{name: "deep", value: deep}},
	}
	if _, err := SealBaseTypeEnv(ir); err == nil || !strings.Contains(err.Error(), "depth exceeds") {
		t.Fatalf("deep SealBaseTypeEnv() error = %v, want depth rejection", err)
	}

	ir = fixture.compiledIR(t, []LinkedDeclaration{valid})
	largeFields := make([]DeclarationField, 40)
	largeValue := strings.Repeat("y", 220<<10)
	for index := range largeFields {
		largeFields[index] = DeclarationField{
			name:  strings.Repeat("f", index+1),
			value: TextValue{value: largeValue},
		}
	}
	ir.declarations[0].body = DeclarationBody{fields: largeFields}
	if _, err := SealBaseTypeEnv(ir); err == nil || !strings.Contains(err.Error(), "scalar content exceeds") {
		t.Fatalf("aggregate SealBaseTypeEnv() error = %v, want scalar budget rejection", err)
	}
}

func TestArtifactDecoderRejectsHostileDepthCountsAndBytesBeforeAllocation(t *testing.T) {
	t.Parallel()

	encodedValue := canonicalDeclarationValue(NewTextValue("leaf"))
	for depth := 0; depth <= maximumDeclarationDepth; depth++ {
		writer := newCanonicalWriter("declaration-value.v1")
		writer.addByte(byte(DeclarationSequence))
		writer.addUint64(1)
		writer.addBytes(encodedValue)
		encodedValue = writer.bytes()
	}
	budget := &artifactBudget{}
	if _, err := decodeDeclarationValue(encodedValue, budget, 1); err == nil || !strings.Contains(err.Error(), "depth exceeds") {
		t.Fatalf("deep decoder error = %v, want depth rejection", err)
	}

	collectionWriter := newCanonicalWriter("declaration-value.v1")
	collectionWriter.addByte(byte(DeclarationSequence))
	collectionWriter.addUint64(maximumValuesPerCollection + 1)
	budget = &artifactBudget{}
	if _, err := decodeDeclarationValue(collectionWriter.bytes(), budget, 1); err == nil || !strings.Contains(err.Error(), "declaration collection exceeds") {
		t.Fatalf("oversized collection decoder error = %v, want count rejection", err)
	}

	coverageWriter := newCanonicalWriter("coverage-manifest.v1")
	coverageWriter.addUint64(maximumCoverageEntries + 1)
	if _, err := decodeCoverageManifest(coverageWriter.bytes()); err == nil || !strings.Contains(err.Error(), "coverage manifest exceeds") {
		t.Fatalf("oversized coverage decoder error = %v, want count rejection", err)
	}

	oversizedPayload := make([]byte, maximumArtifactPayloadBytes+1)
	if _, err := DecodeBaseTypeEnvArtifact(oversizedPayload); err == nil || !strings.Contains(err.Error(), "payload exceeds") {
		t.Fatalf("oversized payload error = %v, want byte-budget rejection", err)
	}
	oversizedWire := make([]byte, maximumCanonicalBytes+1)
	if _, err := UnmarshalBaseTypeEnvArtifact(oversizedWire); err == nil || !strings.Contains(err.Error(), "wire encoding exceeds") {
		t.Fatalf("oversized wire error = %v, want byte-budget rejection", err)
	}
}

func TestZeroValueArtifactCannotCrossIntegrityBoundary(t *testing.T) {
	t.Parallel()

	artifact := BaseTypeEnvArtifact{}
	if err := artifact.Verify(); err == nil {
		t.Fatal("zero-value artifact passed Verify")
	}
	if _, err := artifact.MarshalBinary(); err == nil {
		t.Fatal("zero-value artifact passed MarshalBinary")
	}
	if _, exists := artifact.TypeEnvRef(); exists {
		t.Fatal("zero-value artifact exposed a TypeEnvRef")
	}
}

type artifactFixture struct {
	revision typedmemory.SourceRevision
	compiler typedmemory.CompilerSchemaVersion
	ruleID   typedmemory.CompilerRuleID
}

func newArtifactFixture(t *testing.T) artifactFixture {
	t.Helper()
	revision, err := typedmemory.NewSourceRevision("44dd88188a07646ef23aca32627a3f670525853f")
	if err != nil {
		t.Fatalf("NewSourceRevision() error = %v", err)
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion("fpf-typeenv.v1")
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion() error = %v", err)
	}
	ruleID, err := typedmemory.NewCompilerRuleID("fpf.explicit-declaration.v1")
	if err != nil {
		t.Fatalf("NewCompilerRuleID() error = %v", err)
	}
	return artifactFixture{revision: revision, compiler: compiler, ruleID: ruleID}
}

func (fixture artifactFixture) location(
	t *testing.T,
	unit string,
	start uint64,
	end uint64,
) typedmemory.SourceLocation {
	t.Helper()
	unitID, err := typedmemory.NewSourceUnitID(unit)
	if err != nil {
		t.Fatalf("NewSourceUnitID() error = %v", err)
	}
	digest := mustDigest(t, strings.Repeat("b", 64))
	lineRange, err := typedmemory.NewSourceLineRange(start, end)
	if err != nil {
		t.Fatalf("NewSourceLineRange() error = %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unitID,
		fixture.revision,
		digest,
		lineRange,
	)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation() error = %v", err)
	}
	return location
}

func (fixture artifactFixture) declaration(
	t *testing.T,
	kindName string,
	location typedmemory.SourceLocation,
	dependencies []typedmemory.SchemaSymbolRef,
) LinkedDeclaration {
	t.Helper()
	kindID, err := typedmemory.NewKindID(kindName)
	if err != nil {
		t.Fatalf("NewKindID() error = %v", err)
	}
	symbol, err := typedmemory.KindSymbolRef(kindID)
	if err != nil {
		t.Fatalf("KindSymbolRef() error = %v", err)
	}
	kindField, err := NewDeclarationField("kind_id", NewTextValue(kindName))
	if err != nil {
		t.Fatalf("NewDeclarationField(kind_id) error = %v", err)
	}
	roleField, err := NewDeclarationField("semantic_role", NewTextValue("value_kind"))
	if err != nil {
		t.Fatalf("NewDeclarationField(semantic_role) error = %v", err)
	}
	fields := []DeclarationField{kindField, roleField}
	if len(dependencies) > 0 {
		values := make([]DeclarationValue, 0, len(dependencies))
		for _, dependency := range dependencies {
			value, valueErr := NewSymbolValue(dependency)
			if valueErr != nil {
				t.Fatalf("NewSymbolValue() error = %v", valueErr)
			}
			values = append(values, value)
		}
		set, setErr := NewSetValue(values)
		if setErr != nil {
			t.Fatalf("NewSetValue() error = %v", setErr)
		}
		dependencyField, fieldErr := NewDeclarationField("dependencies", set)
		if fieldErr != nil {
			t.Fatalf("NewDeclarationField(dependencies) error = %v", fieldErr)
		}
		fields = append(fields, dependencyField)
	}
	body, err := NewDeclarationBody(fields)
	if err != nil {
		t.Fatalf("NewDeclarationBody() error = %v", err)
	}
	ruleID, err := typedmemory.NewCompilerRuleID(valueKindCompilerRule)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(ValueKind) error = %v", err)
	}
	provenanceRef, err := typedmemory.NewProvenanceRef("source-" + kindName)
	if err != nil {
		t.Fatalf("NewProvenanceRef() error = %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		provenanceRef,
		location,
		ruleID,
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance() error = %v", err)
	}
	basis, err := NewSourceDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewSourceDeclarationBasis() error = %v", err)
	}
	declaration, err := NewLinkedDeclaration(symbol, ruleID, body, basis)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration() error = %v", err)
	}
	return declaration
}

func (fixture artifactFixture) refKindDeclaration(
	t *testing.T,
	refKindName string,
	referent LinkedDeclaration,
	location typedmemory.SourceLocation,
) LinkedDeclaration {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID(refKindName)
	if err != nil {
		t.Fatalf("NewRefKindID() error = %v", err)
	}
	symbol, err := typedmemory.RefKindSymbolRef(refKindID)
	if err != nil {
		t.Fatalf("RefKindSymbolRef() error = %v", err)
	}
	refKindField, err := NewDeclarationField("ref_kind", NewTextValue(refKindName))
	if err != nil {
		t.Fatalf("NewDeclarationField(ref_kind) error = %v", err)
	}
	referentValue, err := NewSymbolValue(referent.Symbol())
	if err != nil {
		t.Fatalf("NewSymbolValue(referent) error = %v", err)
	}
	referentField, err := NewDeclarationField("referent_value_kind", referentValue)
	if err != nil {
		t.Fatalf("NewDeclarationField(referent_value_kind) error = %v", err)
	}
	body, err := NewDeclarationBody([]DeclarationField{refKindField, referentField})
	if err != nil {
		t.Fatalf("NewDeclarationBody(RefKind) error = %v", err)
	}
	ruleID, err := typedmemory.NewCompilerRuleID(refKindCompilerRule)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(RefKind) error = %v", err)
	}
	provenanceRef, err := typedmemory.NewProvenanceRef("source-" + refKindName)
	if err != nil {
		t.Fatalf("NewProvenanceRef(RefKind) error = %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		provenanceRef,
		location,
		ruleID,
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance(RefKind) error = %v", err)
	}
	basis, err := NewSourceDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewSourceDeclarationBasis(RefKind) error = %v", err)
	}
	declaration, err := NewLinkedDeclaration(symbol, ruleID, body, basis)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration(RefKind) error = %v", err)
	}
	return declaration
}

func (fixture artifactFixture) contextDeclaration(t *testing.T) LinkedDeclaration {
	t.Helper()
	contextRef, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	symbol, err := typedmemory.BoundedContextSymbolRef(contextRef)
	if err != nil {
		t.Fatalf("BoundedContextSymbolRef() error = %v", err)
	}
	contextField, err := NewDeclarationField("context_ref", NewTextValue(contextRef.String()))
	if err != nil {
		t.Fatalf("NewDeclarationField(context_ref) error = %v", err)
	}
	revisionField, err := NewDeclarationField(
		"source_revision",
		NewTextValue(fixture.revision.String()),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(source_revision) error = %v", err)
	}
	body, err := NewDeclarationBody([]DeclarationField{contextField, revisionField})
	if err != nil {
		t.Fatalf("NewDeclarationBody(context) error = %v", err)
	}
	ruleID, err := typedmemory.NewCompilerRuleID(publicationContextCompilerRule)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(context) error = %v", err)
	}
	location := fixture.location(t, "fixture:publication-context", 1, 1)
	provenanceRef, err := typedmemory.NewProvenanceRef("source-fpf-publication-context")
	if err != nil {
		t.Fatalf("NewProvenanceRef(context) error = %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		provenanceRef,
		location,
		ruleID,
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance(context) error = %v", err)
	}
	basis, err := NewSourceDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewSourceDeclarationBasis(context) error = %v", err)
	}
	declaration, err := NewLinkedDeclaration(symbol, ruleID, body, basis)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration(context) error = %v", err)
	}
	return declaration
}

func (fixture artifactFixture) coverage(
	t *testing.T,
	declarations []LinkedDeclaration,
) typedmemory.CoverageManifest {
	t.Helper()
	entries := make([]typedmemory.CoverageEntry, 0, len(declarations))
	for _, declaration := range declarations {
		subject, err := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
		if err != nil {
			t.Fatalf("SchemaSymbolCoverage() error = %v", err)
		}
		location := declaration.Basis().SourceLocations()[0]
		entry, err := typedmemory.NewCompiledCoverageEntry(subject, location)
		if err != nil {
			t.Fatalf("NewCompiledCoverageEntry() error = %v", err)
		}
		entries = append(entries, entry)
	}
	coverage, err := typedmemory.NewCoverageManifest(entries)
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	return coverage
}

func (fixture artifactFixture) compiledIR(
	t *testing.T,
	declarations []LinkedDeclaration,
) LinkedTypeEnvIR {
	t.Helper()
	declarations = append(
		[]LinkedDeclaration{fixture.contextDeclaration(t)},
		declarations...,
	)
	coverage := fixture.coverage(t, declarations)
	ir, err := NewCompiledLinkedTypeEnvIR(
		fixture.revision,
		fixture.compiler,
		coverage,
		declarations,
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR() error = %v", err)
	}
	return ir
}

func mustDigest(t *testing.T, hexValue string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hexValue)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return digest
}
