package typeenv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestP6ClaimGraphRepresentationIsDeterministicAndCompilerDerived(t *testing.T) {
	valueKind := claimGraphTestValueKind(t, "U.ClaimGraph")
	firstSource := claimGraphTestSource(t, "spec:c-2-1:claim-graph", "C.2.1", 100, 120, 'a')
	secondSource := claimGraphTestSource(t, "spec:a-6-5:slot-spec", "A.6.5", 200, 220, 'b')

	first, err := NewP6ClaimGraphRepresentation(
		valueKind,
		[]typedmemory.SourceLocation{firstSource, secondSource},
	)
	if err != nil {
		t.Fatalf("NewP6ClaimGraphRepresentation(first): %v", err)
	}
	second, err := NewP6ClaimGraphRepresentation(
		valueKind,
		[]typedmemory.SourceLocation{secondSource, firstSource},
	)
	if err != nil {
		t.Fatalf("NewP6ClaimGraphRepresentation(second): %v", err)
	}

	if first.ShapeRef() != second.ShapeRef() {
		t.Fatal("ClaimGraph shape identity changed under source permutation")
	}
	if first.CodecRef() != second.CodecRef() {
		t.Fatal("ClaimGraph codec identity changed under source permutation")
	}
	firstBasis := first.Provenance().CanonicalBytes()
	secondBasis := second.Provenance().CanonicalBytes()
	if !bytes.Equal(firstBasis, secondBasis) {
		t.Fatal("compiler-derived provenance changed under source permutation")
	}
	if first.ShapeRef().ID().String() != claimGraphShapeID {
		t.Fatalf("shape ID = %q", first.ShapeRef().ID().String())
	}
	if first.CodecRef().ID().String() != claimGraphCodecID {
		t.Fatalf("codec ID = %q", first.CodecRef().ID().String())
	}
	if first.Registry().Len() != 1 {
		t.Fatalf("codec registry length = %d, want 1", first.Registry().Len())
	}
	if first.ShapeDeclaration().Provenance().Reference() != first.Provenance().Reference() {
		t.Fatal("shape declaration lost compiler-derived provenance")
	}
	if first.ValueBinding().Provenance().Reference() != first.Provenance().Reference() {
		t.Fatal("value binding lost compiler-derived provenance")
	}

	_, err = NewP6ClaimGraphRepresentation(valueKind, nil)
	if err == nil {
		t.Fatal("representation without exact source inputs was accepted")
	}
}

func TestP6ClaimGraphCodecRoundTripsAndRequiresExactDigestOnRead(t *testing.T) {
	representation := claimGraphTestRepresentation(t)
	first := claimGraphTestGraph(t, "alpha", false, false)
	permuted := claimGraphTestGraph(t, "alpha", true, false)

	firstBytes := claimGraphTestCanonicalBytes(t, representation.Codec(), first)
	permutedBytes := claimGraphTestCanonicalBytes(t, representation.Codec(), permuted)
	if !bytes.Equal(firstBytes, permutedBytes) {
		t.Fatal("canonical bytes changed under node permutation")
	}

	sealed, err := representation.Seal(first)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	valid, ok := sealed.(typedmemory.ValidTypedValue)
	if !ok {
		t.Fatalf("Seal = %T, want ValidTypedValue", sealed)
	}
	digest := valid.Value().Digest()

	verified, err := representation.VerifyCanonical(firstBytes, digest)
	if err != nil {
		t.Fatalf("VerifyCanonical: %v", err)
	}
	verifiedValue, ok := verified.(typedmemory.ValidTypedValue)
	if !ok {
		t.Fatalf("VerifyCanonical = %T, want ValidTypedValue", verified)
	}
	if verifiedValue.Value().Digest() != digest {
		t.Fatal("round-trip changed the exact typed-value digest")
	}
	if !bytes.Equal(verifiedValue.Value().CanonicalBytes(), firstBytes) {
		t.Fatal("round-trip changed canonical bytes")
	}
}

func TestP6ClaimGraphCodecRejectsDigestTamperAndJSON(t *testing.T) {
	representation := claimGraphTestRepresentation(t)
	first := claimGraphTestGraph(t, "alpha", false, false)
	changed := claimGraphTestGraph(t, "omega", false, false)

	sealed, err := representation.Seal(first)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	valid := sealed.(typedmemory.ValidTypedValue)
	digest := valid.Value().Digest()
	changedBytes := claimGraphTestCanonicalBytes(t, representation.Codec(), changed)

	verification, err := representation.VerifyCanonical(changedBytes, digest)
	if err != nil {
		t.Fatalf("VerifyCanonical(changed): %v", err)
	}
	assertInvalidDiagnostic(
		t,
		verification,
		typedmemory.DiagnosticTypedValueDigestMismatch,
	)

	verification, err = representation.VerifyCanonical([]byte(`{"nodes":[]}`), digest)
	if err != nil {
		t.Fatalf("VerifyCanonical(JSON): %v", err)
	}
	assertInvalidDiagnostic(t, verification, typedmemory.DiagnosticMalformedValue)
}

func TestP6ClaimGraphCodecRetainsCoreDuplicateAndDanglingRejection(t *testing.T) {
	representation := claimGraphTestRepresentation(t)
	graph := claimGraphTestGraph(t, "alpha", false, true)
	canonical := claimGraphTestCanonicalBytes(t, representation.Codec(), graph)

	duplicateNode := bytes.ReplaceAll(canonical, []byte("node-b"), []byte("node-a"))
	assertRejectedCodecCode(
		t,
		representation.Codec().Canonicalize(representation.ShapeRef(), duplicateNode),
		typedmemory.DiagnosticClaimGraphDuplicateNode,
	)

	duplicateEdge := bytes.ReplaceAll(canonical, []byte("edge-b"), []byte("edge-a"))
	assertRejectedCodecCode(
		t,
		representation.Codec().Canonicalize(representation.ShapeRef(), duplicateEdge),
		typedmemory.DiagnosticMalformedValue,
	)

	dangling := replaceLastSameLength(t, canonical, "node-b", "node-z")
	assertRejectedCodecCode(
		t,
		representation.Codec().Canonicalize(representation.ShapeRef(), dangling),
		typedmemory.DiagnosticClaimGraphDanglingEdge,
	)
}

func TestP6ClaimGraphCodecEnforcesDecodeBudgetsBeforeCoreDecode(t *testing.T) {
	representation := claimGraphTestRepresentation(t)
	graph := claimGraphTestGraph(t, "alpha", false, true)
	core, err := typedmemory.NewClaimGraphCodecV1(representation.ShapeRef())
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	canonical := claimGraphTestCanonicalBytes(t, core, graph)

	tests := []struct {
		name   string
		budget ClaimGraphDecodeBudget
		want   string
	}{
		{
			name: "bytes",
			budget: ClaimGraphDecodeBudget{
				maxCanonicalBytes: uint64(len(canonical) - 1),
				maxNodes:          10,
				maxEdges:          10,
				maxValueDepth:     10,
				maxValueItems:     100,
			},
			want: "canonical bytes",
		},
		{
			name: "nodes",
			budget: ClaimGraphDecodeBudget{
				maxCanonicalBytes: uint64(len(canonical) + 1),
				maxNodes:          1,
				maxEdges:          10,
				maxValueDepth:     10,
				maxValueItems:     100,
			},
			want: "nodes exceed limit",
		},
		{
			name: "edges",
			budget: ClaimGraphDecodeBudget{
				maxCanonicalBytes: uint64(len(canonical) + 1),
				maxNodes:          10,
				maxEdges:          1,
				maxValueDepth:     10,
				maxValueItems:     100,
			},
			want: "edges exceed limit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			codec, err := newP6ClaimGraphCodecV1(representation.ShapeRef(), test.budget)
			if err != nil {
				t.Fatalf("newP6ClaimGraphCodecV1: %v", err)
			}
			result := codec.Canonicalize(representation.ShapeRef(), canonical)
			rejected, ok := result.(typedmemory.RejectedCodecValue)
			if !ok {
				t.Fatalf("Canonicalize = %T, want RejectedCodecValue", result)
			}
			message := rejected.Issues()[0].Message()
			if !strings.Contains(message, test.want) {
				t.Fatalf("rejection %q does not contain %q", message, test.want)
			}
		})
	}
}

func TestP6ClaimGraphCodecEnforcesClosedValueDepthAndItemBudgets(t *testing.T) {
	representation := claimGraphTestRepresentation(t)
	nested := claimGraphTestNestedGraph(t)
	core, err := typedmemory.NewClaimGraphCodecV1(representation.ShapeRef())
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	canonical := claimGraphTestCanonicalBytes(t, core, nested)

	depthBudget := ClaimGraphDecodeBudget{
		maxCanonicalBytes: uint64(len(canonical) + 1),
		maxNodes:          10,
		maxEdges:          10,
		maxValueDepth:     2,
		maxValueItems:     100,
	}
	depthCodec, err := newP6ClaimGraphCodecV1(representation.ShapeRef(), depthBudget)
	if err != nil {
		t.Fatalf("newP6ClaimGraphCodecV1(depth): %v", err)
	}
	depthResult := depthCodec.Canonicalize(representation.ShapeRef(), canonical)
	assertRejectedMessage(t, depthResult, "value depth")

	itemBudget := ClaimGraphDecodeBudget{
		maxCanonicalBytes: uint64(len(canonical) + 1),
		maxNodes:          10,
		maxEdges:          10,
		maxValueDepth:     10,
		maxValueItems:     3,
	}
	itemCodec, err := newP6ClaimGraphCodecV1(representation.ShapeRef(), itemBudget)
	if err != nil {
		t.Fatalf("newP6ClaimGraphCodecV1(items): %v", err)
	}
	itemResult := itemCodec.Canonicalize(representation.ShapeRef(), canonical)
	assertRejectedMessage(t, itemResult, "closed value items")
}

func claimGraphTestRepresentation(t *testing.T) P6ClaimGraphRepresentation {
	t.Helper()
	valueKind := claimGraphTestValueKind(t, "U.ClaimGraph")
	source := claimGraphTestSource(t, "spec:c-2-1:claim-graph", "C.2.1", 100, 120, 'a')
	representation, err := NewP6ClaimGraphRepresentation(
		valueKind,
		[]typedmemory.SourceLocation{source},
	)
	if err != nil {
		t.Fatalf("NewP6ClaimGraphRepresentation: %v", err)
	}
	return representation
}

func claimGraphTestSource(
	t *testing.T,
	unitRaw string,
	patternRaw string,
	start uint64,
	end uint64,
	fill byte,
) typedmemory.SourceLocation {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID(unitRaw)
	if err != nil {
		t.Fatalf("NewSourceUnitID: %v", err)
	}
	revision, err := typedmemory.NewSourceRevision("fpf-test-revision")
	if err != nil {
		t.Fatalf("NewSourceRevision: %v", err)
	}
	digest := claimGraphTestDigest(t, fill)
	lineRange, err := typedmemory.NewSourceLineRange(start, end)
	if err != nil {
		t.Fatalf("NewSourceLineRange: %v", err)
	}
	pattern, err := typedmemory.NewPatternID(patternRaw)
	if err != nil {
		t.Fatalf("NewPatternID: %v", err)
	}
	location, err := typedmemory.NewPatternedSourceLocation(
		unit,
		revision,
		digest,
		lineRange,
		pattern,
	)
	if err != nil {
		t.Fatalf("NewPatternedSourceLocation: %v", err)
	}
	return location
}

func claimGraphTestDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(string(fill), 64)
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	return digest
}

func claimGraphTestTypeEnv(t *testing.T) typedmemory.TypeEnvRef {
	t.Helper()
	digest := claimGraphTestDigest(t, 'c')
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return ref
}

func claimGraphTestValueKind(t *testing.T, raw string) typedmemory.ValueKindRef {
	t.Helper()
	id, err := typedmemory.NewKindID(raw)
	if err != nil {
		t.Fatalf("NewKindID: %v", err)
	}
	ref, err := typedmemory.NewValueKindRef(claimGraphTestTypeEnv(t), id)
	if err != nil {
		t.Fatalf("NewValueKindRef: %v", err)
	}
	return ref
}

func claimGraphTestGraph(
	t *testing.T,
	firstText string,
	reverseNodes bool,
	twoEdges bool,
) typedmemory.ClaimGraphValue {
	t.Helper()
	nodeKind := claimGraphTestValueKind(t, "U.Claim")
	nodeA := claimGraphTestNode(t, "node-a", nodeKind, typedmemory.NewTextValue(firstText))
	nodeB := claimGraphTestNode(t, "node-b", nodeKind, typedmemory.NewTextValue("beta"))
	nodes := []typedmemory.ClaimNode{nodeA, nodeB}
	if reverseNodes {
		nodes = []typedmemory.ClaimNode{nodeB, nodeA}
	}
	edgeA := claimGraphTestEdge(t, "edge-a", "node-a", "node-b")
	edges := []typedmemory.ClaimEdge{edgeA}
	if twoEdges {
		edgeB := claimGraphTestEdge(t, "edge-b", "node-b", "node-a")
		edges = append(edges, edgeB)
	}
	graph, err := typedmemory.NewClaimGraphValue(nodes, edges)
	if err != nil {
		t.Fatalf("NewClaimGraphValue: %v", err)
	}
	return graph
}

func claimGraphTestNestedGraph(t *testing.T) typedmemory.ClaimGraphValue {
	t.Helper()
	leaf := typedmemory.NewTextValue("leaf")
	inner, err := typedmemory.NewOrderedSequenceValue([]typedmemory.TypedValue{leaf})
	if err != nil {
		t.Fatalf("NewOrderedSequenceValue(inner): %v", err)
	}
	outer, err := typedmemory.NewOrderedSequenceValue([]typedmemory.TypedValue{inner})
	if err != nil {
		t.Fatalf("NewOrderedSequenceValue(outer): %v", err)
	}
	nodeKind := claimGraphTestValueKind(t, "U.Claim")
	node := claimGraphTestNode(t, "node-a", nodeKind, outer)
	graph, err := typedmemory.NewClaimGraphValue([]typedmemory.ClaimNode{node}, nil)
	if err != nil {
		t.Fatalf("NewClaimGraphValue: %v", err)
	}
	return graph
}

func claimGraphTestNode(
	t *testing.T,
	idRaw string,
	kind typedmemory.ValueKindRef,
	value typedmemory.TypedValue,
) typedmemory.ClaimNode {
	t.Helper()
	id, err := typedmemory.NewClaimNodeID(idRaw)
	if err != nil {
		t.Fatalf("NewClaimNodeID: %v", err)
	}
	node, err := typedmemory.NewClaimNode(id, kind, value)
	if err != nil {
		t.Fatalf("NewClaimNode: %v", err)
	}
	return node
}

func claimGraphTestEdge(
	t *testing.T,
	idRaw string,
	sourceRaw string,
	targetRaw string,
) typedmemory.ClaimEdge {
	t.Helper()
	id, err := typedmemory.NewClaimEdgeID(idRaw)
	if err != nil {
		t.Fatalf("NewClaimEdgeID: %v", err)
	}
	signatureID, err := typedmemory.NewSignatureID("U.Relates")
	if err != nil {
		t.Fatalf("NewSignatureID: %v", err)
	}
	signature, err := typedmemory.NewRelationSignatureRef(
		claimGraphTestTypeEnv(t),
		signatureID,
	)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef: %v", err)
	}
	source, err := typedmemory.NewClaimNodeID(sourceRaw)
	if err != nil {
		t.Fatalf("NewClaimNodeID(source): %v", err)
	}
	target, err := typedmemory.NewClaimNodeID(targetRaw)
	if err != nil {
		t.Fatalf("NewClaimNodeID(target): %v", err)
	}
	edge, err := typedmemory.NewClaimEdge(id, signature, source, target)
	if err != nil {
		t.Fatalf("NewClaimEdge: %v", err)
	}
	return edge
}

type claimGraphEncoder interface {
	EncodeInput(typedmemory.ClaimGraphValue) typedmemory.CodecCanonicalization
}

func claimGraphTestCanonicalBytes(
	t *testing.T,
	codec claimGraphEncoder,
	graph typedmemory.ClaimGraphValue,
) []byte {
	t.Helper()
	result := codec.EncodeInput(graph)
	canonical, ok := result.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("EncodeInput = %T, want CanonicalizedCodecValue", result)
	}
	return canonical.CanonicalBytes()
}

func assertInvalidDiagnostic(
	t *testing.T,
	verification typedmemory.TypedValueVerification,
	want typedmemory.DiagnosticCode,
) {
	t.Helper()
	invalid, ok := verification.(typedmemory.InvalidTypedValue)
	if !ok {
		t.Fatalf("verification = %T, want InvalidTypedValue", verification)
	}
	diagnostics := invalid.Diagnostics()
	if len(diagnostics) == 0 {
		t.Fatal("invalid result has no diagnostics")
	}
	if diagnostics[0].Code() != want {
		t.Fatalf("diagnostic code = %q, want %q", diagnostics[0].Code(), want)
	}
}

func assertRejectedCodecCode(
	t *testing.T,
	result typedmemory.CodecCanonicalization,
	want typedmemory.DiagnosticCode,
) {
	t.Helper()
	rejected, ok := result.(typedmemory.RejectedCodecValue)
	if !ok {
		t.Fatalf("canonicalization = %T, want RejectedCodecValue", result)
	}
	issues := rejected.Issues()
	if len(issues) == 0 {
		t.Fatal("rejected canonicalization has no issues")
	}
	if issues[0].Code() != want {
		t.Fatalf("codec issue = %q, want %q", issues[0].Code(), want)
	}
}

func assertRejectedMessage(
	t *testing.T,
	result typedmemory.CodecCanonicalization,
	want string,
) {
	t.Helper()
	rejected, ok := result.(typedmemory.RejectedCodecValue)
	if !ok {
		t.Fatalf("canonicalization = %T, want RejectedCodecValue", result)
	}
	message := rejected.Issues()[0].Message()
	if !strings.Contains(message, want) {
		t.Fatalf("rejection %q does not contain %q", message, want)
	}
}

func replaceLastSameLength(
	t *testing.T,
	input []byte,
	old string,
	replacement string,
) []byte {
	t.Helper()
	if len(old) != len(replacement) {
		t.Fatal("replacement fixture must preserve canonical field length")
	}
	index := bytes.LastIndex(input, []byte(old))
	if index < 0 {
		t.Fatalf("fixture does not contain %q", old)
	}
	result := append([]byte(nil), input...)
	copy(result[index:index+len(old)], replacement)
	return result
}
