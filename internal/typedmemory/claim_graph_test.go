package typedmemory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"testing"
)

func TestClaimGraphCodecCanonicalizesNodeAndEdgeSets(t *testing.T) {
	nodeA, nodeB, nodeC, edgeAB, edgeBC := valueTestGraphParts(t)
	first := valueTestClaimGraph(t, []ClaimNode{nodeA, nodeB, nodeC}, []ClaimEdge{edgeAB, edgeBC})
	second := valueTestClaimGraph(t, []ClaimNode{nodeC, nodeA, nodeB}, []ClaimEdge{edgeBC, edgeAB})
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	firstBytes := valueTestEncodedGraph(t, codec, first)
	secondBytes := valueTestEncodedGraph(t, codec, second)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("ClaimGraph canonical bytes changed under node/edge permutation")
	}

	roundTrip := codec.Canonicalize(shape, firstBytes)
	canonical, ok := roundTrip.(CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("round-trip = %T, want CanonicalizedCodecValue", roundTrip)
	}
	if !bytes.Equal(firstBytes, canonical.CanonicalBytes()) {
		t.Fatal("ClaimGraph canonical round-trip changed bytes")
	}
	encoded := codec.EncodeInput(second)
	encodedValue, ok := encoded.(CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("EncodeInput = %T; want CanonicalizedCodecValue", encoded)
	}
	canonicalGraph, ok := encodedValue.Value().(claimGraphValue)
	if !ok {
		t.Fatalf("canonical value = %T; want claimGraphValue", encodedValue.Value())
	}
	if canonicalGraph.Nodes()[0].ID().String() != "node-a" {
		t.Fatal("canonical codec value retained caller insertion order")
	}
}

func TestClaimGraphCodecV1GoldenCanonicalBytes(t *testing.T) {
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	canonical := valueTestEncodedGraph(t, codec, valueTestGraph(t))
	sum := sha256.Sum256(canonical)
	got := hex.EncodeToString(sum[:])
	const want = "de0ce814c2c5e4313459c9aae03f70b742dfbce3f6d1385505b1d6fc2ac7de60"
	if got != want {
		t.Fatalf("ClaimGraphCodecV1 golden byte digest = %s; want %s", got, want)
	}
}

func TestClosedValueEncodingPreservesSequenceOrderButNotSetOrder(t *testing.T) {
	a := NewTextValue("a")
	b := NewTextValue("b")
	orderedAB, err := NewOrderedSequenceValue([]TypedValue{a, b})
	if err != nil {
		t.Fatalf("NewOrderedSequenceValue(ab): %v", err)
	}
	orderedBA, err := NewOrderedSequenceValue([]TypedValue{b, a})
	if err != nil {
		t.Fatalf("NewOrderedSequenceValue(ba): %v", err)
	}
	orderedABBytes, issues := encodeTypedValue(orderedAB)
	if len(issues) > 0 {
		t.Fatalf("encode ordered ab: %s", issues[0].Message())
	}
	orderedBABytes, issues := encodeTypedValue(orderedBA)
	if len(issues) > 0 {
		t.Fatalf("encode ordered ba: %s", issues[0].Message())
	}
	if bytes.Equal(orderedABBytes, orderedBABytes) {
		t.Fatal("ordered sequence encoding discarded meaningful order")
	}

	setAB, err := NewUnorderedSetValue([]TypedValue{a, b})
	if err != nil {
		t.Fatalf("NewUnorderedSetValue(ab): %v", err)
	}
	setBA, err := NewUnorderedSetValue([]TypedValue{b, a})
	if err != nil {
		t.Fatalf("NewUnorderedSetValue(ba): %v", err)
	}
	setABBytes, issues := encodeTypedValue(setAB)
	if len(issues) > 0 {
		t.Fatalf("encode set ab: %s", issues[0].Message())
	}
	setBABytes, issues := encodeTypedValue(setBA)
	if len(issues) > 0 {
		t.Fatalf("encode set ba: %s", issues[0].Message())
	}
	if !bytes.Equal(setABBytes, setBABytes) {
		t.Fatal("unordered-set encoding changed under permutation")
	}
}

func TestSignedIntegerCanonicalRoundTripPreservesFullBitRange(t *testing.T) {
	values := []int64{math.MinInt64, -1, 0, 1, math.MaxInt64}
	for _, value := range values {
		encoded, issues := encodeTypedValue(NewSignedIntegerValue(value))
		if len(issues) > 0 {
			t.Fatalf("encode signed integer %d: %s", value, issues[0].Message())
		}
		decoded, err := decodeTypedValue(encoded)
		if err != nil {
			t.Fatalf("decode signed integer %d: %v", value, err)
		}
		scalar, ok := decoded.(ScalarTypedValue)
		if !ok {
			t.Fatalf("decoded signed integer %d = %T, want ScalarTypedValue", value, decoded)
		}
		got, ok := scalar.SignedInteger()
		if !ok {
			t.Fatalf("decoded signed integer %d lost its signed scalar variant", value)
		}
		if got != value {
			t.Fatalf("signed integer round-trip = %d, want %d", got, value)
		}
	}
}

func TestClaimGraphCodecRejectsDuplicateNodeIdentity(t *testing.T) {
	nodeA, _, _, _, _ := valueTestGraphParts(t)
	invalid := claimGraphValue{nodes: []ClaimNode{nodeA, nodeA}}
	raw, issues := encodeClaimGraph(invalid)
	if len(issues) > 0 {
		t.Fatalf("encode invalid fixture: %s", issues[0].Message())
	}
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	result := codec.Canonicalize(shape, raw)
	rejected, ok := result.(RejectedCodecValue)
	if !ok {
		t.Fatalf("duplicate result = %T, want RejectedCodecValue", result)
	}
	if code := rejected.Issues()[0].Code(); code != DiagnosticClaimGraphDuplicateNode {
		t.Fatalf("duplicate code = %q", code)
	}
}

func TestClaimGraphCodecRejectsDanglingEndpoint(t *testing.T) {
	nodeA, _, _, edgeAB, _ := valueTestGraphParts(t)
	invalid := claimGraphValue{nodes: []ClaimNode{nodeA}, edges: []ClaimEdge{edgeAB}}
	raw, issues := encodeClaimGraph(invalid)
	if len(issues) > 0 {
		t.Fatalf("encode invalid fixture: %s", issues[0].Message())
	}
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	result := codec.Canonicalize(shape, raw)
	rejected, ok := result.(RejectedCodecValue)
	if !ok {
		t.Fatalf("dangling result = %T, want RejectedCodecValue", result)
	}
	if code := rejected.Issues()[0].Code(); code != DiagnosticClaimGraphDanglingEdge {
		t.Fatalf("dangling code = %q", code)
	}
}

func TestClaimGraphCodecRejectsMalformedAndWrongShapeBytes(t *testing.T) {
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	malformed := codec.Canonicalize(shape, []byte("not a claim graph"))
	rejected, ok := malformed.(RejectedCodecValue)
	if !ok {
		t.Fatalf("malformed result = %T, want RejectedCodecValue", malformed)
	}
	if code := rejected.Issues()[0].Code(); code != DiagnosticMalformedValue {
		t.Fatalf("malformed code = %q", code)
	}

	wrongShape := valueTestShapeRef(t, "U.OtherShape", 'o')
	result := codec.Canonicalize(wrongShape, []byte("anything"))
	rejected, ok = result.(RejectedCodecValue)
	if !ok {
		t.Fatalf("wrong shape result = %T, want RejectedCodecValue", result)
	}
	if code := rejected.Issues()[0].Code(); code != DiagnosticValueShapeMismatch {
		t.Fatalf("wrong shape code = %q", code)
	}
}

func valueTestGraph(t *testing.T) ClaimGraphValue {
	t.Helper()
	nodeA, nodeB, nodeC, edgeAB, edgeBC := valueTestGraphParts(t)
	return valueTestClaimGraph(t, []ClaimNode{nodeA, nodeB, nodeC}, []ClaimEdge{edgeAB, edgeBC})
}

func valueTestGraphParts(t *testing.T) (ClaimNode, ClaimNode, ClaimNode, ClaimEdge, ClaimEdge) {
	t.Helper()
	claimKind := valueTestValueKindRef(t, "U.Proposition", 'k')
	statementSignature := valueTestRelationSignatureRef(t, "U.Supports", 'r')
	nodeA := valueTestClaimNode(t, "node-a", claimKind, "A")
	nodeB := valueTestClaimNode(t, "node-b", claimKind, "B")
	nodeC := valueTestClaimNode(t, "node-c", claimKind, "C")
	edgeAB := valueTestClaimEdge(t, "edge-ab", statementSignature, nodeA.ID(), nodeB.ID())
	edgeBC := valueTestClaimEdge(t, "edge-bc", statementSignature, nodeB.ID(), nodeC.ID())
	return nodeA, nodeB, nodeC, edgeAB, edgeBC
}

func valueTestClaimNode(t *testing.T, raw string, kind ValueKindRef, text string) ClaimNode {
	t.Helper()
	id, err := NewClaimNodeID(raw)
	if err != nil {
		t.Fatalf("NewClaimNodeID: %v", err)
	}
	node, err := NewClaimNode(id, kind, NewTextValue(text))
	if err != nil {
		t.Fatalf("NewClaimNode: %v", err)
	}
	return node
}

func valueTestClaimEdge(
	t *testing.T,
	raw string,
	signature RelationSignatureRef,
	source ClaimNodeID,
	target ClaimNodeID,
) ClaimEdge {
	t.Helper()
	id, err := NewClaimEdgeID(raw)
	if err != nil {
		t.Fatalf("NewClaimEdgeID: %v", err)
	}
	edge, err := NewClaimEdge(id, signature, source, target)
	if err != nil {
		t.Fatalf("NewClaimEdge: %v", err)
	}
	return edge
}

func valueTestClaimGraph(t *testing.T, nodes []ClaimNode, edges []ClaimEdge) ClaimGraphValue {
	t.Helper()
	graph, err := NewClaimGraphValue(nodes, edges)
	if err != nil {
		t.Fatalf("NewClaimGraphValue: %v", err)
	}
	return graph
}

func valueTestRelationSignatureRef(t *testing.T, raw string, fill byte) RelationSignatureRef {
	t.Helper()
	id, err := NewSignatureID(raw)
	if err != nil {
		t.Fatalf("NewSignatureID: %v", err)
	}
	ref, err := NewRelationSignatureRef(valueTestTypeEnvRef(t, fill), id)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef: %v", err)
	}
	return ref
}
