package projecttypeenv

import (
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestLowerCompositeValueShapesCoversClosedAlgebraAndIsPermutationInvariant(t *testing.T) {
	target := lowerCompositeTestTypeEnvRef(t, '1')
	inherited := lowerCompositeTestInheritedShapes(t)
	sources := lowerCompositeTestValueRepresentationSources()

	firstShapes, firstRefs, err := lowerCompositeValueShapes(
		sources,
		inherited,
		lowerCompositeTestProvenance(t, target),
	)
	if err != nil {
		t.Fatalf("lowerCompositeValueShapes() error = %v", err)
	}
	reversed := append([]compositeSourceDeclaration(nil), sources...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	secondShapes, secondRefs, err := lowerCompositeValueShapes(
		reversed,
		inherited,
		lowerCompositeTestProvenance(t, target),
	)
	if err != nil {
		t.Fatalf("permuted lowerCompositeValueShapes() error = %v", err)
	}

	if len(firstShapes) != 6 {
		t.Fatalf("shape count = %d, want 6", len(firstShapes))
	}
	if !reflect.DeepEqual(lowerCompositeTestShapeRefStrings(firstShapes), lowerCompositeTestShapeRefStrings(secondShapes)) {
		t.Fatalf("shape refs differ across permutation:\nfirst  = %v\nsecond = %v", lowerCompositeTestShapeRefStrings(firstShapes), lowerCompositeTestShapeRefStrings(secondShapes))
	}
	if !reflect.DeepEqual(lowerCompositeTestShapeMapStrings(firstRefs), lowerCompositeTestShapeMapStrings(secondRefs)) {
		t.Fatalf("resolved shape map differs across permutation")
	}
	if firstRefs["Base.Shape.External"] != inherited["Base.Shape.External"] {
		t.Fatal("inherited B shape reference was not preserved exactly")
	}

	kinds := make(map[typedmemory.ValueShapeKind]bool)
	for _, declaration := range firstShapes {
		kinds[declaration.Shape().Kind()] = true
	}
	wantKinds := []typedmemory.ValueShapeKind{
		typedmemory.ValueShapeScalar,
		typedmemory.ValueShapeRecord,
		typedmemory.ValueShapeSum,
		typedmemory.ValueShapeOrderedSequence,
		typedmemory.ValueShapeUnorderedSet,
		typedmemory.ValueShapeClaimGraph,
	}
	for _, kind := range wantKinds {
		if !kinds[kind] {
			t.Fatalf("lowered shapes do not contain %q", kind)
		}
	}
}

func TestLowerCompositeValueBindingsDerivesCodecFromExactOrderedContract(t *testing.T) {
	target := lowerCompositeTestTypeEnvRef(t, '2')
	sources := lowerCompositeTestValueRepresentationSources()
	shapes, refs, err := lowerCompositeValueShapes(
		sources,
		lowerCompositeTestInheritedShapes(t),
		lowerCompositeTestProvenance(t, target),
	)
	if err != nil {
		t.Fatalf("lowerCompositeValueShapes() error = %v", err)
	}
	if len(shapes) == 0 {
		t.Fatal("shape fixture did not lower")
	}
	bindings, specifications, err := lowerCompositeValueBindingsWithSpecifications(
		target,
		sources,
		refs,
		lowerCompositeTestProvenance(t, target),
	)
	if err != nil {
		t.Fatalf("lowerCompositeValueBindingsWithSpecifications() error = %v", err)
	}
	if len(bindings) != 1 || len(specifications) != 1 {
		t.Fatalf("binding/specification counts = %d/%d, want 1/1", len(bindings), len(specifications))
	}
	wantContract := []string{"trim no bytes", "encode UTF-8 exactly"}
	if !reflect.DeepEqual(specifications[0].Contract(), wantContract) {
		t.Fatalf("contract = %v, want exact authored order %v", specifications[0].Contract(), wantContract)
	}
	codecID, err := typedmemory.NewCodecID("App.Codec.TextV1")
	if err != nil {
		t.Fatalf("NewCodecID(): %v", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion("v1")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion(): %v", err)
	}
	wantSpecification, err := DeriveCodecSpecificationV1(
		codecID,
		version,
		refs["App.Shape.Text"],
		wantContract,
	)
	if err != nil {
		t.Fatalf("DeriveCodecSpecificationV1(): %v", err)
	}
	if bindings[0].Codec() != wantSpecification.Ref() {
		t.Fatalf("binding codec = %q, want %q", bindings[0].Codec(), wantSpecification.Ref())
	}
	reversedSpecification, err := DeriveCodecSpecificationV1(
		codecID,
		version,
		refs["App.Shape.Text"],
		[]string{"encode UTF-8 exactly", "trim no bytes"},
	)
	if err != nil {
		t.Fatalf("reverse DeriveCodecSpecificationV1(): %v", err)
	}
	if reversedSpecification.Ref() == bindings[0].Codec() {
		t.Fatal("codec identity ignored authored contract order")
	}
}

func TestLowerCompositeValueShapesRejectsMissingChildrenDeterministically(t *testing.T) {
	target := lowerCompositeTestTypeEnvRef(t, '3')
	left := lowerCompositeTestShapeDeclaration(
		"App.Shape.Left",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeOrderedSequence)),
			lowerCompositeTestFact("shape.element", "Missing.Shape.Z"),
		},
	)
	right := lowerCompositeTestShapeDeclaration(
		"App.Shape.Right",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeUnorderedSet)),
			lowerCompositeTestFact("shape.element", "Missing.Shape.A"),
		},
	)
	first := []compositeSourceDeclaration{{value: left}, {value: right}}
	second := []compositeSourceDeclaration{{value: right}, {value: left}}

	_, _, firstErr := lowerCompositeValueShapes(first, nil, lowerCompositeTestProvenance(t, target))
	_, _, secondErr := lowerCompositeValueShapes(second, nil, lowerCompositeTestProvenance(t, target))
	if firstErr == nil || secondErr == nil {
		t.Fatal("missing child reference was accepted")
	}
	if firstErr.Error() != secondErr.Error() {
		t.Fatalf("missing-child diagnostics differ:\nfirst  = %q\nsecond = %q", firstErr, secondErr)
	}
	want := "App.Shape.Left -> Missing.Shape.Z, App.Shape.Right -> Missing.Shape.A"
	if !strings.Contains(firstErr.Error(), want) {
		t.Fatalf("missing-child error = %q, want ordered witnesses %q", firstErr, want)
	}
}

func TestLowerCompositeValueShapesRejectsCyclesDeterministically(t *testing.T) {
	target := lowerCompositeTestTypeEnvRef(t, '4')
	left := lowerCompositeTestShapeDeclaration(
		"App.Shape.Left",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeOrderedSequence)),
			lowerCompositeTestFact("shape.element", "App.Shape.Right"),
		},
	)
	right := lowerCompositeTestShapeDeclaration(
		"App.Shape.Right",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeUnorderedSet)),
			lowerCompositeTestFact("shape.element", "App.Shape.Left"),
		},
	)
	blocked := lowerCompositeTestShapeDeclaration(
		"App.Shape.Blocked",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeOrderedSequence)),
			lowerCompositeTestFact("shape.element", "App.Shape.Left"),
		},
	)
	first := []compositeSourceDeclaration{{value: left}, {value: blocked}, {value: right}}
	second := []compositeSourceDeclaration{{value: right}, {value: left}, {value: blocked}}

	_, _, firstErr := lowerCompositeValueShapes(first, nil, lowerCompositeTestProvenance(t, target))
	_, _, secondErr := lowerCompositeValueShapes(second, nil, lowerCompositeTestProvenance(t, target))
	if firstErr == nil || secondErr == nil {
		t.Fatal("shape dependency cycle was accepted")
	}
	if firstErr.Error() != secondErr.Error() {
		t.Fatalf("cycle diagnostics differ:\nfirst  = %q\nsecond = %q", firstErr, secondErr)
	}
	if !strings.Contains(firstErr.Error(), "App.Shape.Left, App.Shape.Right") {
		t.Fatalf("cycle error = %q, want canonical symbol order", firstErr)
	}
	if strings.Contains(firstErr.Error(), "cycle among: App.Shape.Blocked") {
		t.Fatalf("cycle error misclassified acyclic dependent: %q", firstErr)
	}
	if !strings.Contains(firstErr.Error(), "blocked dependents: App.Shape.Blocked") {
		t.Fatalf("cycle error omitted blocked dependent: %q", firstErr)
	}
}

func lowerCompositeTestValueRepresentationSources() []compositeSourceDeclaration {
	text := lowerCompositeTestShapeDeclaration(
		"App.Shape.Text",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeScalar)),
			lowerCompositeTestFact("shape.scalar_kind", string(typedmemory.ScalarText)),
		},
	)
	record := lowerCompositeTestShapeDeclaration(
		"App.Shape.Record",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeRecord)),
			lowerCompositeTestFact(keyedPath("shape.fields", "external")+".name", "external"),
			lowerCompositeTestFact(keyedPath("shape.fields", "external")+".shape", "Base.Shape.External"),
			lowerCompositeTestFact(keyedPath("shape.fields", "text")+".name", "text"),
			lowerCompositeTestFact(keyedPath("shape.fields", "text")+".shape", "App.Shape.Text"),
		},
	)
	claimGraph := lowerCompositeTestShapeDeclaration(
		"App.Shape.Claims",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeClaimGraph)),
		},
	)
	sum := lowerCompositeTestShapeDeclaration(
		"App.Shape.Sum",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeSum)),
			lowerCompositeTestFact(keyedPath("shape.variants", "claims")+".name", "claims"),
			lowerCompositeTestFact(keyedPath("shape.variants", "claims")+".shape", "App.Shape.Claims"),
			lowerCompositeTestFact(keyedPath("shape.variants", "record")+".name", "record"),
			lowerCompositeTestFact(keyedPath("shape.variants", "record")+".shape", "App.Shape.Record"),
		},
	)
	ordered := lowerCompositeTestShapeDeclaration(
		"App.Shape.Sequence",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeOrderedSequence)),
			lowerCompositeTestFact("shape.element", "App.Shape.Record"),
		},
	)
	unordered := lowerCompositeTestShapeDeclaration(
		"App.Shape.Set",
		[]SourceFact{
			lowerCompositeTestFact("shape.kind", string(localpractice.ValueShapeUnorderedSet)),
			lowerCompositeTestFact("shape.element", "App.Shape.Sum"),
		},
	)
	codec := SymbolicDeclaration{
		kind:   localpractice.DeclarationCodecBinding,
		symbol: lowerCompositeTestScalar("App.Codec.TextV1"),
		span:   lowerCompositeTestSpan(),
		facts: []SourceFact{
			lowerCompositeTestFact("value_kind", "App.TextValue"),
			lowerCompositeTestFact("value_shape", "App.Shape.Text"),
			lowerCompositeTestFact("canonicalization_version", "v1"),
			lowerCompositeTestFact(indexedPath("contract", 0), "trim no bytes"),
			lowerCompositeTestFact(indexedPath("contract", 1), "encode UTF-8 exactly"),
		},
	}
	return []compositeSourceDeclaration{
		{value: codec},
		{value: unordered},
		{value: sum},
		{value: claimGraph},
		{value: ordered},
		{value: record},
		{value: text},
	}
}

func lowerCompositeTestShapeDeclaration(
	symbol string,
	facts []SourceFact,
) SymbolicDeclaration {
	return SymbolicDeclaration{
		kind:   localpractice.DeclarationValueShape,
		symbol: lowerCompositeTestScalar(symbol),
		span:   lowerCompositeTestSpan(),
		facts:  append([]SourceFact(nil), facts...),
	}
}

func lowerCompositeTestFact(path string, value string) SourceFact {
	return SourceFact{path: path, value: lowerCompositeTestScalar(value)}
}

func lowerCompositeTestScalar(value string) SourceScalar {
	return SourceScalar{value: value, span: lowerCompositeTestSpan()}
}

func lowerCompositeTestSpan() SourceSpan {
	return SourceSpan{start: 1, end: 1}
}

func lowerCompositeTestInheritedShapes(
	t *testing.T,
) map[string]typedmemory.ValueShapeRef {
	t.Helper()
	id, err := typedmemory.NewShapeID("Base.Shape.External")
	if err != nil {
		t.Fatalf("NewShapeID(): %v", err)
	}
	ref, err := typedmemory.DeriveValueShapeRef(id, typedmemory.NewClaimGraphShape())
	if err != nil {
		t.Fatalf("DeriveValueShapeRef(): %v", err)
	}
	return map[string]typedmemory.ValueShapeRef{"Base.Shape.External": ref}
}

func lowerCompositeTestTypeEnvRef(t *testing.T, fill byte) typedmemory.TypeEnvRef {
	t.Helper()
	digest := lowerCompositeTestDigest(t, fill)
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	return ref
}

func lowerCompositeTestDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(string(fill), 64)
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}

func lowerCompositeTestProvenance(
	t *testing.T,
	base typedmemory.TypeEnvRef,
) func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error) {
	t.Helper()
	return func(
		source compositeSourceDeclaration,
		compilerRule string,
	) (typedmemory.ProjectSourceProvenance, error) {
		declaration := source.value
		reference, err := typedmemory.NewProvenanceRef("test-" + strings.ReplaceAll(declaration.Symbol().Value(), ".", "-"))
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		carrier, err := typedmemory.NewCarrierRef("test-carrier")
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		edition, err := typedmemory.NewCarrierEdition("v1")
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		lineRange, err := typedmemory.NewSourceLineRange(1, 1)
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		rule, err := typedmemory.NewCompilerRuleID(compilerRule)
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		context, err := typedmemory.NewBoundedContextRef("test-context")
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		manifest, err := typedmemory.NewSignatureManifestRef("Test.Manifest", "v1")
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		symbol, err := lowerCompositeTestSchemaSymbol(declaration)
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		basis, err := typedmemory.NewManifestSymbolBasis(
			manifest,
			typedmemory.ManifestProvide,
			symbol,
		)
		if err != nil {
			return typedmemory.ProjectSourceProvenance{}, err
		}
		builder := typedmemory.NewProjectSourceProvenanceBuilder(
			reference,
			carrier,
			edition,
			lowerCompositeTestDigest(t, 'a'),
		)
		builder = builder.SetDeclarationRange(lineRange)
		builder = builder.SetCompilerRule(rule)
		builder = builder.SetBoundedContext(context)
		builder = builder.SetBaseTypeEnv(base)
		builder = builder.SetSignatureBlockRow(typedmemory.VocabularyRow)
		builder = builder.SetManifestBasis(basis)
		return builder.Build()
	}
}

func lowerCompositeTestSchemaSymbol(
	declaration SymbolicDeclaration,
) (typedmemory.SchemaSymbolRef, error) {
	switch declaration.Kind() {
	case localpractice.DeclarationValueShape:
		id, err := typedmemory.NewShapeID(declaration.Symbol().Value())
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case localpractice.DeclarationCodecBinding:
		id, err := typedmemory.NewCodecID(declaration.Symbol().Value())
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.CodecSymbolRef(id)
	default:
		return typedmemory.SchemaSymbolRef{}, nil
	}
}

func lowerCompositeTestShapeRefStrings(
	values []typedmemory.ValueShapeDeclaration,
) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Ref().String())
	}
	return result
}

func lowerCompositeTestShapeMapStrings(
	values map[string]typedmemory.ValueShapeRef,
) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value.String()
	}
	return result
}
