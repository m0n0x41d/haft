package typeenv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestPinnedCompilerMaterializesClaimGraphShapeBindingAndCodec(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	result, err := CompileBaseTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv: %v", err)
	}
	if result.Rejected() {
		t.Fatalf("CompileBaseTypeEnv rejected: %v", result.Diagnostics())
	}
	artifact, ok := result.Artifact()
	if !ok {
		t.Fatal("accepted compilation has no artifact")
	}
	environment, ok := result.Environment()
	if !ok {
		t.Fatal("accepted compilation has no TypeEnv")
	}
	registry, ok := result.CodecRegistry()
	if !ok {
		t.Fatal("accepted compilation has no CodecRegistry")
	}

	if len(environment.ValueShapes()) != 1 {
		t.Fatalf("ValueShape count = %d, want 1", len(environment.ValueShapes()))
	}
	if len(environment.ValueBindings()) != 1 {
		t.Fatalf("ValueBinding count = %d, want 1", len(environment.ValueBindings()))
	}
	shape := environment.ValueShapes()[0]
	binding := environment.ValueBindings()[0]
	if shape.Ref().ID().String() != claimGraphShapeID {
		t.Fatalf("shape ID = %q", shape.Ref().ID().String())
	}
	if shape.Shape().Kind() != typedmemory.ValueShapeClaimGraph {
		t.Fatalf("shape kind = %q", shape.Shape().Kind())
	}
	if binding.ValueKind().ID().String() != claimGraphValueKindID {
		t.Fatalf("binding ValueKind = %q", binding.ValueKind().ID().String())
	}
	if binding.ValueShape() != shape.Ref() {
		t.Fatal("binding does not use the materialized ClaimGraph shape")
	}
	if binding.Codec().ID().String() != claimGraphCodecID {
		t.Fatalf("binding CodecID = %q", binding.Codec().ID().String())
	}
	if !registry.Contains(binding.Codec()) {
		t.Fatal("accepted compilation registry lacks its mandatory codec")
	}
	if len(environment.ContextKindAvailabilities()) != 0 {
		t.Fatal("ClaimGraph representation invented context-kind availability")
	}
	if len(environment.RelationSignatures()) != 0 {
		t.Fatal("ClaimGraph representation invented a relation signature")
	}

	claimGraphSource := resolveGrammarSourceID(t, snapshot, "C.2.1:4.2.1")
	assertCompilerDerivedClaimGraphProvenance(t, shape.Provenance(), claimGraphSource)
	assertCompilerDerivedClaimGraphProvenance(t, binding.Provenance(), claimGraphSource)
	assertLinkedClaimGraphDeclarations(t, artifact)
	assertClaimGraphCodecRoundTrip(t, environment, registry)

	ref, _ := artifact.TypeEnvRef()
	if bytes.Contains(artifact.CanonicalBytes(), []byte(ref.String())) {
		t.Fatal("self-reference-free artifact payload contains its derived TypeEnvRef")
	}
}

func TestRejectedCompilationDoesNotExposeCodecRegistry(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	mutated := mutatePinnedStructuralSource(
		t,
		snapshot,
		"A.6.5:4.2",
		"SlotSpec := <SlotKind, ValueKind, refMode>",
		"SlotSpec := <SlotKind, ValueKind>",
	)
	result, err := CompileBaseTypeEnv(mutated)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv: %v", err)
	}
	if !result.Rejected() {
		t.Fatal("mutated publication was not rejected")
	}
	registry, ok := result.CodecRegistry()
	if ok {
		t.Fatal("rejected compilation exposed a CodecRegistry")
	}
	if registry.Len() != 0 {
		t.Fatalf("rejected registry length = %d, want 0", registry.Len())
	}
}

func TestClaimGraphMechanismBytesContributeToProjectedTypeEnvRef(t *testing.T) {
	artifact := claimGraphTestCompiledArtifact(t)
	codecSymbol := claimGraphTestCodecSymbol(t)
	codec, ok := artifactDeclaration(artifact, codecSymbol)
	if !ok {
		t.Fatal("compiled artifact has no ClaimGraph codec declaration")
	}

	shapeRef, err := newClaimGraphShapeRef()
	if err != nil {
		t.Fatalf("newClaimGraphShapeRef: %v", err)
	}
	changedBudget := P6ClaimGraphDecodeBudget()
	changedBudget.maxNodes++
	changedCodecRef, err := newClaimGraphCodecRef(shapeRef, changedBudget)
	if err != nil {
		t.Fatalf("newClaimGraphCodecRef(changed): %v", err)
	}
	valueSymbol, err := kindSymbol(claimGraphValueKindID)
	if err != nil {
		t.Fatalf("kindSymbol: %v", err)
	}
	bodies, err := claimGraphDeclarationBodies(
		valueSymbol,
		shapeRef,
		changedCodecRef,
		changedBudget,
		codec.Basis().SourceLocations(),
	)
	if err != nil {
		t.Fatalf("claimGraphDeclarationBodies(changed): %v", err)
	}
	changedCodec, err := NewLinkedDeclaration(
		codec.Symbol(),
		codec.RuleID(),
		bodies.codec,
		codec.Basis(),
	)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration(changed codec): %v", err)
	}
	changedDeclarations := replaceLinkedDeclaration(
		t,
		artifact.Declarations(),
		changedCodec,
	)
	changedRef := projectedTypeEnvRef(t, artifact, changedDeclarations)
	originalRef, _ := artifact.TypeEnvRef()
	if changedRef == originalRef {
		t.Fatal("changed codec budget/config did not change projected TypeEnvRef")
	}
	if changedCodec.Digest() == codec.Digest() {
		t.Fatal("changed codec configuration did not change declaration digest")
	}
}

func TestExecutableLowerRejectsIgnoredOrContradictoryDeclarationBytes(t *testing.T) {
	artifact := claimGraphTestCompiledArtifact(t)

	tests := []struct {
		name   string
		symbol typedmemory.SchemaSymbolRef
		mutate func(t *testing.T, body DeclarationBody) DeclarationBody
	}{
		{
			name:   "unknown shape field",
			symbol: claimGraphTestShapeSymbol(t),
			mutate: func(t *testing.T, body DeclarationBody) DeclarationBody {
				return addDeclarationField(t, body, "ignored_identity", NewTextValue("must-not-ignore"))
			},
		},
		{
			name:   "wrong codec field type",
			symbol: claimGraphTestCodecSymbol(t),
			mutate: func(t *testing.T, body DeclarationBody) DeclarationBody {
				return replaceDeclarationField(t, body, "codec_id", NewUnsignedValue(7))
			},
		},
		{
			name:   "redundant context identity mismatch",
			symbol: claimGraphTestContextSymbol(t),
			mutate: func(t *testing.T, body DeclarationBody) DeclarationBody {
				return replaceDeclarationField(
					t,
					body,
					"context_ref",
					NewTextValue("fpf:another-publication"),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			declaration, ok := artifactDeclaration(artifact, test.symbol)
			if !ok {
				t.Fatalf("artifact has no %s", test.symbol.String())
			}
			body := test.mutate(t, declaration.Body())
			mutated, err := NewLinkedDeclaration(
				declaration.Symbol(),
				declaration.RuleID(),
				body,
				declaration.Basis(),
			)
			if err != nil {
				t.Fatalf("NewLinkedDeclaration(mutated): %v", err)
			}
			declarations := replaceLinkedDeclaration(t, artifact.Declarations(), mutated)
			ir, err := NewCompiledLinkedTypeEnvIR(
				artifact.SourceRevision(),
				artifact.CompilerSchemaVersion(),
				artifact.CoverageManifest(),
				declarations,
			)
			if err != nil {
				t.Fatalf("NewCompiledLinkedTypeEnvIR: %v", err)
			}
			_, err = SealBaseTypeEnv(ir)
			if err == nil {
				t.Fatal("SealBaseTypeEnv accepted declaration bytes ignored by lowering")
			}
			if !strings.Contains(err.Error(), "not exactly lowerable") {
				t.Fatalf("SealBaseTypeEnv error = %v", err)
			}
		})
	}
}

func TestExecutableLowerRejectsSlotOwnerMismatch(t *testing.T) {
	revision, _ := typedmemory.NewSourceRevision("fixture-revision")
	compiler, _ := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	rootUnit := linkerSourceUnit("fixture:root", "Fixture.Root", "Fixture.Owner", 10)
	slotUnit := linkerSourceUnit("fixture:slot", "Fixture.Slot", "Fixture.Owner", 20)
	root := RelationRootDeclaration{
		source:      rootUnit,
		owner:       rootUnit.ParentPatternID,
		subjectKind: "Fixture.Subject",
		relation:    "Fixture.Relation",
	}
	slot := SlotDeclarationFragment{
		source:      slotUnit,
		owner:       slotUnit.ParentPatternID,
		slotKind:    "FixtureSlot",
		valueKind:   "Fixture.Value",
		reference:   ByValueEvidence{},
		cardinality: BoundedCardinalityEvidence{minimum: 1, maximum: 1},
	}
	artifact, err := linkStructuralDeclarations(
		revision,
		compiler,
		[]StructuralDeclaration{root, slot},
		nil,
	)
	if err != nil {
		t.Fatalf("linkStructuralDeclarations: %v", err)
	}
	signatureID, _ := typedmemory.NewSignatureID("Fixture.Relation")
	slotID, _ := typedmemory.NewSlotKindID("FixtureSlot")
	slotSymbol, _ := typedmemory.SlotKindSymbolRef(signatureID, slotID)
	declaration, ok := artifactDeclaration(artifact, slotSymbol)
	if !ok {
		t.Fatal("fixture artifact has no SlotKind declaration")
	}
	body := replaceDeclarationField(
		t,
		declaration.Body(),
		"governing_relation",
		NewTextValue("Fixture.OtherRelation"),
	)
	mutated, err := NewLinkedDeclaration(
		declaration.Symbol(),
		declaration.RuleID(),
		body,
		declaration.Basis(),
	)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration(mutated): %v", err)
	}
	declarations := replaceLinkedDeclaration(t, artifact.Declarations(), mutated)
	ir, err := NewCompiledLinkedTypeEnvIR(
		artifact.SourceRevision(),
		artifact.CompilerSchemaVersion(),
		artifact.CoverageManifest(),
		declarations,
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR: %v", err)
	}
	if _, err := SealBaseTypeEnv(ir); err == nil {
		t.Fatal("SealBaseTypeEnv accepted SlotKind owner mismatch")
	}
}

func assertCompilerDerivedClaimGraphProvenance(
	t *testing.T,
	provenance typedmemory.DeclarationProvenance,
	source fpf.SourceUnit,
) {
	t.Helper()
	derived, ok := provenance.(typedmemory.CompilerDerivedProvenance)
	if !ok {
		t.Fatalf("provenance = %T, want CompilerDerivedProvenance", provenance)
	}
	if derived.CompilerRuleID().String() != claimGraphRepresentationRule {
		t.Fatalf("compiler rule = %q", derived.CompilerRuleID().String())
	}
	inputs := derived.Inputs()
	if len(inputs) != 1 {
		t.Fatalf("source input count = %d, want 1", len(inputs))
	}
	input := inputs[0]
	want, err := sourceLocation(source)
	if err != nil {
		t.Fatalf("sourceLocation(%s): %v", source.SourceID, err)
	}
	if input.UnitID() != want.UnitID() ||
		input.Revision() != want.Revision() ||
		input.ContentHash() != want.ContentHash() ||
		input.LineRange() != want.LineRange() {
		t.Fatalf("ClaimGraph source input = %#v", input)
	}
}

func assertLinkedClaimGraphDeclarations(
	t *testing.T,
	artifact BaseTypeEnvArtifact,
) {
	t.Helper()
	for _, symbol := range []typedmemory.SchemaSymbolRef{
		claimGraphTestShapeSymbol(t),
		claimGraphTestCodecSymbol(t),
	} {
		declaration, ok := artifactDeclaration(artifact, symbol)
		if !ok {
			t.Fatalf("artifact has no %s", symbol.String())
		}
		if declaration.Basis().Kind() != CompilerDerivedBasis {
			t.Fatalf("%s basis = %v, want compiler-derived", symbol.String(), declaration.Basis().Kind())
		}
		locations := declaration.Basis().SourceLocations()
		if len(locations) != 1 || locations[0].UnitID().String() != "spec:pattern_section:c-2-1:3164" {
			t.Fatalf("%s exact sources = %#v", symbol.String(), locations)
		}
	}
}

func assertClaimGraphCodecRoundTrip(
	t *testing.T,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
) {
	t.Helper()
	binding := environment.ValueBindings()[0]
	implementation, ok := registry.Resolve(binding.Codec())
	if !ok {
		t.Fatal("registry cannot resolve ClaimGraph codec")
	}
	codec, ok := implementation.(P6ClaimGraphCodecV1)
	if !ok {
		t.Fatalf("codec implementation = %T", implementation)
	}
	graph := claimGraphTestGraph(t, "alpha", false, false)
	encoded := codec.EncodeInput(graph)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("EncodeInput = %T", encoded)
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		binding.ValueKind(),
		binding.ValueShape(),
		binding.Codec(),
		canonical.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}
	verification := typedmemory.VerifyTypedValue(registry, binding, candidate)
	if _, ok := verification.(typedmemory.ValidTypedValue); !ok {
		t.Fatalf("VerifyTypedValue = %T", verification)
	}
}

func claimGraphTestCompiledArtifact(t *testing.T) BaseTypeEnvArtifact {
	t.Helper()
	result, err := CompileBaseTypeEnv(loadPinnedGrammarSnapshot(t))
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv: %v", err)
	}
	artifact, ok := result.Artifact()
	if !ok {
		t.Fatalf("CompileBaseTypeEnv rejected: %v", result.Diagnostics())
	}
	return artifact
}

func claimGraphTestShapeSymbol(t *testing.T) typedmemory.SchemaSymbolRef {
	t.Helper()
	id, err := typedmemory.NewShapeID(claimGraphShapeID)
	if err != nil {
		t.Fatalf("NewShapeID: %v", err)
	}
	symbol, err := typedmemory.ValueShapeSymbolRef(id)
	if err != nil {
		t.Fatalf("ValueShapeSymbolRef: %v", err)
	}
	return symbol
}

func claimGraphTestCodecSymbol(t *testing.T) typedmemory.SchemaSymbolRef {
	t.Helper()
	id, err := typedmemory.NewCodecID(claimGraphCodecID)
	if err != nil {
		t.Fatalf("NewCodecID: %v", err)
	}
	symbol, err := typedmemory.CodecSymbolRef(id)
	if err != nil {
		t.Fatalf("CodecSymbolRef: %v", err)
	}
	return symbol
}

func claimGraphTestContextSymbol(t *testing.T) typedmemory.SchemaSymbolRef {
	t.Helper()
	ref, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	symbol, err := typedmemory.BoundedContextSymbolRef(ref)
	if err != nil {
		t.Fatalf("BoundedContextSymbolRef: %v", err)
	}
	return symbol
}

func replaceLinkedDeclaration(
	t *testing.T,
	declarations []LinkedDeclaration,
	replacement LinkedDeclaration,
) []LinkedDeclaration {
	t.Helper()
	result := append([]LinkedDeclaration(nil), declarations...)
	found := false
	for index, declaration := range result {
		if declaration.Symbol() != replacement.Symbol() {
			continue
		}
		result[index] = replacement
		found = true
	}
	if !found {
		t.Fatalf("declaration %s was not found", replacement.Symbol().String())
	}
	return result
}

func addDeclarationField(
	t *testing.T,
	body DeclarationBody,
	name string,
	value DeclarationValue,
) DeclarationBody {
	t.Helper()
	field, err := NewDeclarationField(name, value)
	if err != nil {
		t.Fatalf("NewDeclarationField: %v", err)
	}
	fields := append(body.Fields(), field)
	result, err := NewDeclarationBody(fields)
	if err != nil {
		t.Fatalf("NewDeclarationBody: %v", err)
	}
	return result
}

func replaceDeclarationField(
	t *testing.T,
	body DeclarationBody,
	name string,
	value DeclarationValue,
) DeclarationBody {
	t.Helper()
	fields := body.Fields()
	found := false
	for index, field := range fields {
		if field.Name() != name {
			continue
		}
		replacement, err := NewDeclarationField(name, value)
		if err != nil {
			t.Fatalf("NewDeclarationField: %v", err)
		}
		fields[index] = replacement
		found = true
	}
	if !found {
		t.Fatalf("field %q was not found", name)
	}
	result, err := NewDeclarationBody(fields)
	if err != nil {
		t.Fatalf("NewDeclarationBody: %v", err)
	}
	return result
}

func projectedTypeEnvRef(
	t *testing.T,
	artifact BaseTypeEnvArtifact,
	declarations []LinkedDeclaration,
) typedmemory.TypeEnvRef {
	t.Helper()
	ir, err := NewCompiledLinkedTypeEnvIR(
		artifact.SourceRevision(),
		artifact.CompilerSchemaVersion(),
		artifact.CoverageManifest(),
		declarations,
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR: %v", err)
	}
	normalized, err := normalizeLinkedTypeEnvIR(ir)
	if err != nil {
		t.Fatalf("normalizeLinkedTypeEnvIR: %v", err)
	}
	manifest := deriveSymbolManifest(normalized.declarations)
	payload := canonicalArtifactPayload(normalized, manifest)
	digest, err := typedmemory.NewSHA256Digest(digestCanonicalBytes(payload))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return ref
}
