package typedmemorycandidatecodec

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestNewSuiteRequiresExactCandidateShapeGraph(t *testing.T) {
	declarations := testCandidateShapeDeclarations(t)
	suite, err := NewSuite(declarations)
	if err != nil {
		t.Fatalf("NewSuite(): %v", err)
	}
	wantIDs := map[string]string{
		"text":        textShapeID,
		"polarity":    evidencePolarityShapeID,
		"instant":     canonicalInstantShapeID,
		"evidence":    evidenceUseQualifierShapeID,
		"interval":    performedIntervalShapeID,
		"code_anchor": codeAnchorLocatorShapeID,
	}
	gotIDs := map[string]string{
		"text":        suite.Text().Shape().ID().String(),
		"polarity":    suite.EvidencePolarity().Shape().ID().String(),
		"instant":     suite.CanonicalInstant().Shape().ID().String(),
		"evidence":    suite.EvidenceUseQualifier().Shape().ID().String(),
		"interval":    suite.PerformedInterval().Shape().ID().String(),
		"code_anchor": suite.CodeAnchorLocator().Shape().ID().String(),
	}
	for label, want := range wantIDs {
		if got := gotIDs[label]; got != want {
			t.Fatalf("%s shape ID = %q, want %q", label, got, want)
		}
	}

	missing := append([]typedmemory.ValueShapeDeclaration(nil), declarations[1:]...)
	if _, err := NewSuite(missing); err == nil {
		t.Fatal("NewSuite() accepted a missing TextV1 declaration")
	}

	duplicate := append([]typedmemory.ValueShapeDeclaration(nil), declarations...)
	duplicate = append(duplicate, declarations[0])
	if _, err := NewSuite(duplicate); err == nil {
		t.Fatal("NewSuite() accepted a duplicate candidate declaration")
	}

	wrong := append([]typedmemory.ValueShapeDeclaration(nil), declarations...)
	wrong[0] = testShapeDeclaration(
		t,
		textShapeID,
		mustScalarShape(typedmemory.ScalarBytes),
	)
	if _, err := NewSuite(wrong); err == nil {
		t.Fatal("NewSuite() accepted TextV1 with the wrong scalar structure")
	}
}

func testSuite(t *testing.T) Suite {
	t.Helper()
	suite, err := NewSuite(testCandidateShapeDeclarations(t))
	if err != nil {
		t.Fatalf("NewSuite(): %v", err)
	}
	return suite
}

func testCandidateShapeDeclarations(
	t *testing.T,
) []typedmemory.ValueShapeDeclaration {
	t.Helper()
	declarations := make([]typedmemory.ValueShapeDeclaration, 0, len(candidateShapeIDs))
	refs := make(map[string]typedmemory.ValueShapeRef, len(candidateShapeIDs))
	add := func(id string, shape typedmemory.ValueShape) {
		declaration := testShapeDeclaration(t, id, shape)
		declarations = append(declarations, declaration)
		refs[id] = declaration.Ref()
	}
	add(textShapeID, mustScalarShape(typedmemory.ScalarText))
	add(evidencePolarityShapeID, mustScalarShape(typedmemory.ScalarText))
	add(canonicalInstantShapeID, mustScalarShape(typedmemory.ScalarText))
	add(evidenceUseQualifierShapeID, mustRecordShape([]shapeField{
		{name: "polarity", ref: refs[evidencePolarityShapeID]},
	}))
	add(completedPerformedIntervalShapeID, mustRecordShape([]shapeField{
		{name: "start", ref: refs[canonicalInstantShapeID]},
		{name: "end", ref: refs[canonicalInstantShapeID]},
	}))
	add(inFlightPerformedIntervalShapeID, mustRecordShape([]shapeField{
		{name: "start", ref: refs[canonicalInstantShapeID]},
	}))
	add(performedIntervalStateShapeID, mustSumShape([]shapeField{
		{name: completedPerformedIntervalVariant, ref: refs[completedPerformedIntervalShapeID]},
		{name: inFlightPerformedIntervalVariant, ref: refs[inFlightPerformedIntervalShapeID]},
	}))
	add(performedIntervalShapeID, mustRecordShape([]shapeField{
		{name: "state", ref: refs[performedIntervalStateShapeID]},
	}))
	add(fileCodeAnchorTargetShapeID, mustRecordShape([]shapeField{
		{name: "path", ref: refs[textShapeID]},
	}))
	add(symbolCodeAnchorTargetShapeID, mustRecordShape([]shapeField{
		{name: "path", ref: refs[textShapeID]},
		{name: "symbol", ref: refs[textShapeID]},
	}))
	add(codeAnchorTargetShapeID, mustSumShape([]shapeField{
		{name: fileCodeAnchorTargetVariant, ref: refs[fileCodeAnchorTargetShapeID]},
		{name: symbolCodeAnchorTargetVariant, ref: refs[symbolCodeAnchorTargetShapeID]},
	}))
	add(codeAnchorLocatorShapeID, mustRecordShape([]shapeField{
		{name: "repository", ref: refs[textShapeID]},
		{name: "revision", ref: refs[textShapeID]},
		{name: "target", ref: refs[codeAnchorTargetShapeID]},
	}))
	return declarations
}

func testShapeDeclaration(
	t *testing.T,
	idRaw string,
	shape typedmemory.ValueShape,
) typedmemory.ValueShapeDeclaration {
	t.Helper()
	id, err := typedmemory.NewShapeID(idRaw)
	if err != nil {
		t.Fatalf("NewShapeID(%q): %v", idRaw, err)
	}
	ref, err := typedmemory.DeriveValueShapeRef(id, shape)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef(%q): %v", idRaw, err)
	}
	declaration, err := typedmemory.NewValueShapeDeclaration(
		ref,
		shape,
		testCompilerProvenance(t),
	)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration(%q): %v", idRaw, err)
	}
	return declaration
}

func testCompilerProvenance(t *testing.T) typedmemory.CompilerDerivedProvenance {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID("candidate-codec-test-source")
	if err != nil {
		t.Fatal(err)
	}
	revision, err := typedmemory.NewSourceRevision("candidate-codec-test-revision")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("ab", sha256.Size),
	)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := typedmemory.NewSourceLineRange(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unit,
		revision,
		digest,
		lines,
	)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := typedmemory.NewProvenanceRef("candidate-codec-test-provenance")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := typedmemory.NewCompilerRuleID("haft.candidate-codec-test.v1")
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewCompilerDerivedProvenance(
		reference,
		[]typedmemory.SourceLocation{location},
		rule,
	)
	if err != nil {
		t.Fatal(err)
	}
	return provenance
}

func requireCanonical(
	t *testing.T,
	result typedmemory.CodecCanonicalization,
) typedmemory.CanonicalizedCodecValue {
	t.Helper()
	canonical, ok := result.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("result = %T, want CanonicalizedCodecValue: %v", result, rejectionError(result))
	}
	return canonical
}

func requireRejected(
	t *testing.T,
	result typedmemory.CodecCanonicalization,
) typedmemory.RejectedCodecValue {
	t.Helper()
	rejected, ok := result.(typedmemory.RejectedCodecValue)
	if !ok {
		t.Fatalf("result = %T, want RejectedCodecValue", result)
	}
	if len(rejected.Issues()) == 0 {
		t.Fatal("rejection has no issues")
	}
	return rejected
}

func assertIdempotent(
	t *testing.T,
	codec typedmemory.CodecImplementation,
	shape typedmemory.ValueShapeRef,
	first typedmemory.CanonicalizedCodecValue,
) {
	t.Helper()
	second := requireCanonical(t, codec.Canonicalize(shape, first.CanonicalBytes()))
	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("decode/encode changed canonical bytes")
	}
	mutated := first.CanonicalBytes()
	mutated[0] ^= 0xff
	if bytes.Equal(mutated, first.CanonicalBytes()) {
		t.Fatal("CanonicalBytes() aliases internal storage")
	}
}

func canonicalDigest(value typedmemory.CanonicalizedCodecValue) string {
	digest := sha256.Sum256(value.CanonicalBytes())
	return fmt.Sprintf("%x", digest[:])
}
