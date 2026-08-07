package typedmemory

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
)

const (
	claimGraphCodecDomain = "haft.typedmemory.claim-graph-codec.v1"
	claimNodeCodecDomain  = "haft.typedmemory.claim-node.v1"
	claimEdgeCodecDomain  = "haft.typedmemory.claim-edge.v1"
	typedValueCodecDomain = "haft.typedmemory.closed-value.v1"
	maxCollectionItems    = uint64(1_000_000)
)

type ClaimNodeID struct {
	value string
}

func NewClaimNodeID(raw string) (ClaimNodeID, error) {
	value, err := parseOpaqueIdentifier("claim-node ID", raw)
	if err != nil {
		return ClaimNodeID{}, err
	}
	return ClaimNodeID{value: value}, nil
}

func (id ClaimNodeID) String() string { return id.value }

func (id ClaimNodeID) valid() bool { return id.value != "" }

type ClaimEdgeID struct {
	value string
}

func NewClaimEdgeID(raw string) (ClaimEdgeID, error) {
	value, err := parseOpaqueIdentifier("claim-edge ID", raw)
	if err != nil {
		return ClaimEdgeID{}, err
	}
	return ClaimEdgeID{value: value}, nil
}

func (id ClaimEdgeID) String() string { return id.value }

func (id ClaimEdgeID) valid() bool { return id.value != "" }

// ClaimNode binds stable node identity to an exact ValueKindRef and a member of
// the closed TypedValue algebra. The kind is a type assertion, not evidence
// that the claim is true.
type ClaimNode struct {
	id        ClaimNodeID
	valueKind ValueKindRef
	value     TypedValue
}

func NewClaimNode(id ClaimNodeID, valueKind ValueKindRef, value TypedValue) (ClaimNode, error) {
	if !id.valid() {
		return ClaimNode{}, fmt.Errorf("claim-node ID is required")
	}
	if !valueKind.valid() {
		return ClaimNode{}, fmt.Errorf("claim-node ValueKindRef is required")
	}
	if !validTypedValue(value) {
		return ClaimNode{}, fmt.Errorf("claim-node value must belong to the closed TypedValue algebra")
	}
	return ClaimNode{id: id, valueKind: valueKind, value: value}, nil
}

func (node ClaimNode) ID() ClaimNodeID { return node.id }

func (node ClaimNode) ValueKind() ValueKindRef { return node.valueKind }

func (node ClaimNode) Value() TypedValue { return node.value }

func (node ClaimNode) valid() bool {
	return node.id.valid() && node.valueKind.valid() && validTypedValue(node.value)
}

// ClaimEdge keeps a stable edge identity and exact relation-signature type.
// Source and target order is meaningful inside the edge; the edge set itself
// is unordered.
type ClaimEdge struct {
	id        ClaimEdgeID
	signature RelationSignatureRef
	source    ClaimNodeID
	target    ClaimNodeID
}

func NewClaimEdge(
	id ClaimEdgeID,
	signature RelationSignatureRef,
	source ClaimNodeID,
	target ClaimNodeID,
) (ClaimEdge, error) {
	if !id.valid() {
		return ClaimEdge{}, fmt.Errorf("claim-edge ID is required")
	}
	if !signature.valid() {
		return ClaimEdge{}, fmt.Errorf("claim-edge RelationSignatureRef is required")
	}
	if !source.valid() || !target.valid() {
		return ClaimEdge{}, fmt.Errorf("claim-edge source and target are required")
	}
	return ClaimEdge{id: id, signature: signature, source: source, target: target}, nil
}

func (edge ClaimEdge) ID() ClaimEdgeID { return edge.id }

func (edge ClaimEdge) Signature() RelationSignatureRef { return edge.signature }

func (edge ClaimEdge) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return edge.signature
}

func (ClaimEdge) RelationDeclarationPosture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (edge ClaimEdge) Source() ClaimNodeID { return edge.source }

func (edge ClaimEdge) Target() ClaimNodeID { return edge.target }

func (edge ClaimEdge) valid() bool {
	return edge.id.valid() && edge.signature.valid() && edge.source.valid() && edge.target.valid()
}

type ClaimGraphValue interface {
	TypedValue
	Nodes() []ClaimNode
	Edges() []ClaimEdge
	claimGraphValueVariant()
}

type claimGraphValue struct {
	nodes []ClaimNode
	edges []ClaimEdge
}

func NewClaimGraphValue(nodes []ClaimNode, edges []ClaimEdge) (ClaimGraphValue, error) {
	value := claimGraphValue{
		nodes: append([]ClaimNode(nil), nodes...),
		edges: append([]ClaimEdge(nil), edges...),
	}
	issues := validateClaimGraph(value)
	if len(issues) > 0 {
		return nil, fmt.Errorf("invalid ClaimGraph: %s", issues[0].Message())
	}
	return value, nil
}

func (claimGraphValue) Kind() TypedValueKind { return TypedValueClaimGraph }

func (value claimGraphValue) Nodes() []ClaimNode {
	return append([]ClaimNode(nil), value.nodes...)
}

func (value claimGraphValue) Edges() []ClaimEdge {
	return append([]ClaimEdge(nil), value.edges...)
}

func (claimGraphValue) typedValueVariant() {}

func (claimGraphValue) claimGraphValueVariant() {}

// ClaimGraphCodecV1 is tied to one exact ClaimGraph ValueShapeRef. The
// immutable CodecRef remains the separate key under which this mechanism is
// registered.
type ClaimGraphCodecV1 struct {
	shape ValueShapeRef
}

func NewClaimGraphCodecV1(shape ValueShapeRef) (ClaimGraphCodecV1, error) {
	if !shape.valid() {
		return ClaimGraphCodecV1{}, fmt.Errorf("ClaimGraphCodecV1 requires an exact ClaimGraph ValueShapeRef")
	}
	return ClaimGraphCodecV1{shape: shape}, nil
}

func (codec ClaimGraphCodecV1) Shape() ValueShapeRef { return codec.shape }

func (codec ClaimGraphCodecV1) Canonicalize(
	expectedShape ValueShapeRef,
	inputBytes []byte,
) CodecCanonicalization {
	if !codec.shape.valid() || expectedShape != codec.shape {
		issue := newCodecIssueWithWitness(
			DiagnosticValueShapeMismatch,
			"ClaimGraphCodecV1 was invoked for a different ValueShapeRef",
			"typed_value.value_shape_ref",
			diagnosticReference(codec.shape.String()),
			diagnosticReference(expectedShape.String()),
		)
		return RejectedCodecValue{issues: []CodecIssue{issue}}
	}

	value, decodeIssue := decodeClaimGraphBytes(inputBytes)
	if decodeIssue != nil {
		return RejectedCodecValue{issues: []CodecIssue{*decodeIssue}}
	}
	issues := validateClaimGraph(value)
	if len(issues) > 0 {
		return RejectedCodecValue{issues: issues}
	}
	canonicalBytes, encodeIssues := encodeClaimGraph(value)
	if len(encodeIssues) > 0 {
		return RejectedCodecValue{issues: encodeIssues}
	}
	canonicalValue := normalizeClaimGraph(value)
	return CanonicalizedCodecValue{value: canonicalValue, canonicalBytes: canonicalBytes}
}

// EncodeInput converts a constructed ClaimGraph into transport bytes accepted
// by Canonicalize. The result remains only a codec value; VerifyTypedValue must
// still prove the exact active binding before a VerifiedTypedValue exists.
func (codec ClaimGraphCodecV1) EncodeInput(value ClaimGraphValue) CodecCanonicalization {
	graph, exactVariant := value.(claimGraphValue)
	if !exactVariant {
		issue := newCodecIssueWithWitness(
			DiagnosticMalformedValue,
			"ClaimGraph value must be the exact closed variant",
			"claim_graph",
			diagnosticState("claimGraphValue"),
			diagnosticText(fmt.Sprintf("%T", value)),
		)
		return RejectedCodecValue{issues: []CodecIssue{issue}}
	}
	issues := validateClaimGraph(graph)
	if len(issues) > 0 {
		return RejectedCodecValue{issues: issues}
	}
	canonicalBytes, encodeIssues := encodeClaimGraph(graph)
	if len(encodeIssues) > 0 {
		return RejectedCodecValue{issues: encodeIssues}
	}
	canonicalValue := normalizeClaimGraph(graph)
	return CanonicalizedCodecValue{value: canonicalValue, canonicalBytes: canonicalBytes}
}

func normalizeClaimGraph(value claimGraphValue) claimGraphValue {
	nodes := append([]ClaimNode(nil), value.nodes...)
	edges := append([]ClaimEdge(nil), value.edges...)
	sort.Slice(nodes, func(left, right int) bool {
		return nodes[left].id.String() < nodes[right].id.String()
	})
	sort.Slice(edges, func(left, right int) bool {
		return edges[left].id.String() < edges[right].id.String()
	})
	return claimGraphValue{nodes: nodes, edges: edges}
}

func validateClaimGraph(value claimGraphValue) []CodecIssue {
	issues := make([]CodecIssue, 0)
	nodes := make(map[string]struct{}, len(value.nodes))
	for index, node := range value.nodes {
		if !node.valid() {
			issues = append(issues, newCodecIssueWithWitness(
				DiagnosticMalformedValue,
				fmt.Sprintf("claim node at index %d is incomplete", index),
				fmt.Sprintf("claim_graph.nodes[%d]", index),
				diagnosticState("complete ClaimNode"),
				diagnosticState("incomplete"),
			))
			continue
		}
		key := node.id.String()
		if _, exists := nodes[key]; exists {
			issues = append(issues, newCodecIssueWithWitness(
				DiagnosticClaimGraphDuplicateNode,
				fmt.Sprintf("duplicate claim-node ID %q", key),
				"claim_graph.nodes."+key,
				diagnosticState("unique ClaimNodeID"),
				diagnosticReference(key),
			))
			continue
		}
		nodes[key] = struct{}{}
	}

	edges := make(map[string]struct{}, len(value.edges))
	for index, edge := range value.edges {
		if !edge.valid() {
			issues = append(issues, newCodecIssueWithWitness(
				DiagnosticMalformedValue,
				fmt.Sprintf("claim edge at index %d is incomplete", index),
				fmt.Sprintf("claim_graph.edges[%d]", index),
				diagnosticState("complete ClaimEdge"),
				diagnosticState("incomplete"),
			))
			continue
		}
		key := edge.id.String()
		if _, exists := edges[key]; exists {
			issues = append(issues, newCodecIssueWithWitness(
				DiagnosticMalformedValue,
				fmt.Sprintf("duplicate claim-edge ID %q", key),
				"claim_graph.edges."+key,
				diagnosticState("unique ClaimEdgeID"),
				diagnosticReference(key),
			))
			continue
		}
		edges[key] = struct{}{}
		_, sourceExists := nodes[edge.source.String()]
		_, targetExists := nodes[edge.target.String()]
		if !sourceExists || !targetExists {
			missingEndpoints := make([]string, 0, 2)
			if !sourceExists {
				missingEndpoints = append(missingEndpoints, edge.source.String())
			}
			if !targetExists {
				missingEndpoints = append(missingEndpoints, edge.target.String())
			}
			issues = append(issues, newCodecIssueWithWitness(
				DiagnosticClaimGraphDanglingEdge,
				fmt.Sprintf("claim edge %q has a dangling endpoint", key),
				"claim_graph.edges."+key,
				diagnosticState("both edge endpoints present"),
				diagnosticSet(missingEndpoints),
			))
		}
	}
	return issues
}

func encodeClaimGraph(value claimGraphValue) ([]byte, []CodecIssue) {
	nodes := append([]ClaimNode(nil), value.nodes...)
	edges := append([]ClaimEdge(nil), value.edges...)
	sort.Slice(nodes, func(left, right int) bool {
		return nodes[left].id.String() < nodes[right].id.String()
	})
	sort.Slice(edges, func(left, right int) bool {
		return edges[left].id.String() < edges[right].id.String()
	})

	writer := newCanonicalWriter(claimGraphCodecDomain)
	writer.addUint64(uint64(len(nodes)))
	for _, node := range nodes {
		encoded, issues := encodeClaimNode(node)
		if len(issues) > 0 {
			return nil, issues
		}
		writer.addBytes(encoded)
	}
	writer.addUint64(uint64(len(edges)))
	for _, edge := range edges {
		writer.addBytes(encodeClaimEdge(edge))
	}
	return writer.bytes(), nil
}

func encodeClaimNode(node ClaimNode) ([]byte, []CodecIssue) {
	valueBytes, issues := encodeTypedValue(node.value)
	if len(issues) > 0 {
		return nil, issues
	}
	writer := newCanonicalWriter(claimNodeCodecDomain)
	writer.addString(node.id.String())
	writer.addString(node.valueKind.TypeEnv().Digest().String())
	writer.addString(node.valueKind.ID().String())
	writer.addBytes(valueBytes)
	return writer.bytes(), nil
}

func encodeClaimEdge(edge ClaimEdge) []byte {
	writer := newCanonicalWriter(claimEdgeCodecDomain)
	writer.addString(edge.id.String())
	writer.addString(edge.signature.TypeEnv().Digest().String())
	writer.addString(edge.signature.ID().String())
	writer.addString(edge.source.String())
	writer.addString(edge.target.String())
	return writer.bytes()
}

func decodeClaimGraphBytes(input []byte) (claimGraphValue, *CodecIssue) {
	reader, err := newDomainReader(input, claimGraphCodecDomain)
	if err != nil {
		issue := malformedClaimGraphIssue(err)
		return claimGraphValue{}, &issue
	}
	nodeCount, err := reader.readCount()
	if err != nil {
		issue := malformedClaimGraphIssue(err)
		return claimGraphValue{}, &issue
	}
	nodes := make([]ClaimNode, 0, nodeCount)
	for index := uint64(0); index < nodeCount; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			issue := malformedClaimGraphIssue(readErr)
			return claimGraphValue{}, &issue
		}
		node, decodeErr := decodeClaimNode(encoded)
		if decodeErr != nil {
			issue := malformedClaimGraphIssue(decodeErr)
			return claimGraphValue{}, &issue
		}
		nodes = append(nodes, node)
	}
	edgeCount, err := reader.readCount()
	if err != nil {
		issue := malformedClaimGraphIssue(err)
		return claimGraphValue{}, &issue
	}
	edges := make([]ClaimEdge, 0, edgeCount)
	for index := uint64(0); index < edgeCount; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			issue := malformedClaimGraphIssue(readErr)
			return claimGraphValue{}, &issue
		}
		edge, decodeErr := decodeClaimEdge(encoded)
		if decodeErr != nil {
			issue := malformedClaimGraphIssue(decodeErr)
			return claimGraphValue{}, &issue
		}
		edges = append(edges, edge)
	}
	if err := reader.requireEnd(); err != nil {
		issue := malformedClaimGraphIssue(err)
		return claimGraphValue{}, &issue
	}
	return claimGraphValue{nodes: nodes, edges: edges}, nil
}

func decodeClaimNode(encoded []byte) (ClaimNode, error) {
	reader, err := newDomainReader(encoded, claimNodeCodecDomain)
	if err != nil {
		return ClaimNode{}, err
	}
	idRaw, err := reader.readString()
	if err != nil {
		return ClaimNode{}, err
	}
	id, err := NewClaimNodeID(idRaw)
	if err != nil {
		return ClaimNode{}, err
	}
	valueKind, err := reader.readValueKindRef()
	if err != nil {
		return ClaimNode{}, err
	}
	valueBytes, err := reader.readBytes()
	if err != nil {
		return ClaimNode{}, err
	}
	value, err := decodeTypedValue(valueBytes)
	if err != nil {
		return ClaimNode{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return ClaimNode{}, err
	}
	return ClaimNode{id: id, valueKind: valueKind, value: value}, nil
}

func decodeClaimEdge(encoded []byte) (ClaimEdge, error) {
	reader, err := newDomainReader(encoded, claimEdgeCodecDomain)
	if err != nil {
		return ClaimEdge{}, err
	}
	idRaw, err := reader.readString()
	if err != nil {
		return ClaimEdge{}, err
	}
	id, err := NewClaimEdgeID(idRaw)
	if err != nil {
		return ClaimEdge{}, err
	}
	signature, err := reader.readRelationSignatureRef()
	if err != nil {
		return ClaimEdge{}, err
	}
	sourceRaw, err := reader.readString()
	if err != nil {
		return ClaimEdge{}, err
	}
	targetRaw, err := reader.readString()
	if err != nil {
		return ClaimEdge{}, err
	}
	source, err := NewClaimNodeID(sourceRaw)
	if err != nil {
		return ClaimEdge{}, err
	}
	target, err := NewClaimNodeID(targetRaw)
	if err != nil {
		return ClaimEdge{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return ClaimEdge{}, err
	}
	return ClaimEdge{id: id, signature: signature, source: source, target: target}, nil
}

func encodeTypedValue(value TypedValue) ([]byte, []CodecIssue) {
	writer := newCanonicalWriter(typedValueCodecDomain)
	writer.addString(string(value.Kind()))

	switch typed := value.(type) {
	case scalarTypedValue:
		if err := encodeScalarValue(&writer, typed); err != nil {
			issue := newCodecIssueWithWitness(
				DiagnosticMalformedValue,
				"scalar canonical encoding failed",
				"typed_value.scalar",
				diagnosticState("canonical scalar bytes"),
				diagnosticText(err.Error()),
			)
			return nil, []CodecIssue{issue}
		}
	case recordTypedValue:
		writer.addUint64(uint64(len(typed.fields)))
		for _, field := range typed.fields {
			encoded, issues := encodeTypedValue(field.value)
			if len(issues) > 0 {
				return nil, issues
			}
			writer.addString(field.name.String())
			writer.addBytes(encoded)
		}
	case sumTypedValue:
		encoded, issues := encodeTypedValue(typed.value)
		if len(issues) > 0 {
			return nil, issues
		}
		writer.addString(typed.variant.String())
		writer.addBytes(encoded)
	case orderedSequenceTypedValue:
		writer.addUint64(uint64(len(typed.items)))
		for _, item := range typed.items {
			encoded, issues := encodeTypedValue(item)
			if len(issues) > 0 {
				return nil, issues
			}
			writer.addBytes(encoded)
		}
	case unorderedSetTypedValue:
		encodedItems, issues := encodeUnorderedItems(typed.items)
		if len(issues) > 0 {
			return nil, issues
		}
		writer.addUint64(uint64(len(encodedItems)))
		for _, encoded := range encodedItems {
			writer.addBytes(encoded)
		}
	case claimGraphValue:
		issues := validateClaimGraph(typed)
		if len(issues) > 0 {
			return nil, issues
		}
		encoded, issues := encodeClaimGraph(typed)
		if len(issues) > 0 {
			return nil, issues
		}
		writer.addBytes(encoded)
	default:
		issue := newCodecIssueWithWitness(
			DiagnosticMalformedValue,
			"value does not belong to the closed TypedValue algebra",
			"typed_value",
			diagnosticState("closed TypedValue variant"),
			diagnosticText(fmt.Sprintf("%T", value)),
		)
		return nil, []CodecIssue{issue}
	}
	return writer.bytes(), nil
}

func encodeScalarValue(writer *canonicalWriter, value scalarTypedValue) error {
	writer.addString(string(value.kind))
	switch value.kind {
	case ScalarText:
		writer.addString(value.text)
	case ScalarBoolean:
		if value.boolean {
			writer.addString("true")
			return nil
		}
		writer.addString("false")
	case ScalarSignedInteger:
		var encoded [8]byte
		written, err := binary.Encode(encoded[:], binary.BigEndian, value.signedInteger)
		if err != nil {
			return fmt.Errorf("encode signed integer: %w", err)
		}
		if written != len(encoded) {
			return fmt.Errorf("encode signed integer: wrote %d bytes, want %d", written, len(encoded))
		}
		writer.addBytes(encoded[:])
	case ScalarUnsignedInteger:
		writer.addUint64(value.unsignedInt)
	case ScalarBytes:
		writer.addBytes(value.bytes)
	}
	return nil
}

func encodeUnorderedItems(items []TypedValue) ([][]byte, []CodecIssue) {
	encodedItems := make([][]byte, 0, len(items))
	for _, item := range items {
		encoded, issues := encodeTypedValue(item)
		if len(issues) > 0 {
			return nil, issues
		}
		encodedItems = append(encodedItems, encoded)
	}
	encodedItems = sortedCanonicalBytes(encodedItems)
	for index := 1; index < len(encodedItems); index++ {
		if bytes.Equal(encodedItems[index-1], encodedItems[index]) {
			issue := newCodecIssueWithWitness(
				DiagnosticMalformedValue,
				"unordered set contains a duplicate canonical value",
				"typed_value.unordered_set",
				diagnosticState("unique canonical values"),
				diagnosticByteDigest(encodedItems[index]),
			)
			return nil, []CodecIssue{issue}
		}
	}
	return encodedItems, nil
}

func decodeTypedValue(encoded []byte) (TypedValue, error) {
	reader, err := newDomainReader(encoded, typedValueCodecDomain)
	if err != nil {
		return nil, err
	}
	kind, err := reader.readString()
	if err != nil {
		return nil, err
	}
	value, err := decodeTypedValueBody(TypedValueKind(kind), reader)
	if err != nil {
		return nil, err
	}
	if err := reader.requireEnd(); err != nil {
		return nil, err
	}
	return value, nil
}

func decodeTypedValueBody(kind TypedValueKind, reader *canonicalReader) (TypedValue, error) {
	switch kind {
	case TypedValueScalar:
		return decodeScalarValue(reader)
	case TypedValueRecord:
		return decodeRecordValue(reader)
	case TypedValueSum:
		return decodeSumValue(reader)
	case TypedValueOrderedSequence:
		return decodeSequenceValue(reader, true)
	case TypedValueUnorderedSet:
		return decodeSequenceValue(reader, false)
	case TypedValueClaimGraph:
		encoded, err := reader.readBytes()
		if err != nil {
			return nil, err
		}
		return decodeClaimGraphBytesAsValue(encoded)
	default:
		return nil, fmt.Errorf("unknown closed TypedValue kind %q", kind)
	}
}

func decodeScalarValue(reader *canonicalReader) (TypedValue, error) {
	kindRaw, err := reader.readString()
	if err != nil {
		return nil, err
	}
	kind := ScalarKind(kindRaw)
	switch kind {
	case ScalarText:
		value, readErr := reader.readString()
		return NewTextValue(value), readErr
	case ScalarBoolean:
		value, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		if value == "true" {
			return NewBooleanValue(true), nil
		}
		if value == "false" {
			return NewBooleanValue(false), nil
		}
		return nil, fmt.Errorf("invalid canonical boolean %q", value)
	case ScalarSignedInteger:
		value, readErr := reader.readFixedInt64()
		if readErr != nil {
			return nil, readErr
		}
		return NewSignedIntegerValue(value), nil
	case ScalarUnsignedInteger:
		value, readErr := reader.readUint64()
		if readErr != nil {
			return nil, readErr
		}
		return NewUnsignedIntegerValue(value), nil
	case ScalarBytes:
		value, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		return NewBytesValue(value), nil
	default:
		return nil, fmt.Errorf("unknown scalar kind %q", kind)
	}
}

func decodeRecordValue(reader *canonicalReader) (TypedValue, error) {
	count, err := reader.readCount()
	if err != nil {
		return nil, err
	}
	fields := make([]RecordFieldValue, 0, count)
	for index := uint64(0); index < count; index++ {
		nameRaw, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		name, parseErr := NewValueMemberName(nameRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		value, decodeErr := decodeTypedValue(encoded)
		if decodeErr != nil {
			return nil, decodeErr
		}
		field, fieldErr := NewRecordFieldValue(name, value)
		if fieldErr != nil {
			return nil, fieldErr
		}
		fields = append(fields, field)
	}
	return NewRecordValue(fields)
}

func decodeSumValue(reader *canonicalReader) (TypedValue, error) {
	nameRaw, err := reader.readString()
	if err != nil {
		return nil, err
	}
	name, err := NewValueMemberName(nameRaw)
	if err != nil {
		return nil, err
	}
	encoded, err := reader.readBytes()
	if err != nil {
		return nil, err
	}
	value, err := decodeTypedValue(encoded)
	if err != nil {
		return nil, err
	}
	return NewSumValue(name, value)
}

func decodeSequenceValue(reader *canonicalReader, ordered bool) (TypedValue, error) {
	count, err := reader.readCount()
	if err != nil {
		return nil, err
	}
	items := make([]TypedValue, 0, count)
	for index := uint64(0); index < count; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		item, decodeErr := decodeTypedValue(encoded)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, item)
	}
	if ordered {
		return NewOrderedSequenceValue(items)
	}
	return NewUnorderedSetValue(items)
}

func decodeClaimGraphBytesAsValue(encoded []byte) (TypedValue, error) {
	value, issue := decodeClaimGraphBytes(encoded)
	if issue != nil {
		return nil, fmt.Errorf("%s", issue.Message())
	}
	issues := validateClaimGraph(value)
	if len(issues) > 0 {
		return nil, fmt.Errorf("%s", issues[0].Message())
	}
	return value, nil
}

func malformedClaimGraphIssue(err error) CodecIssue {
	return newCodecIssueWithWitness(
		DiagnosticMalformedValue,
		"malformed ClaimGraph bytes: "+err.Error(),
		"claim_graph",
		diagnosticState("canonical ClaimGraph bytes"),
		diagnosticText(err.Error()),
	)
}

type canonicalReader struct {
	input  []byte
	offset int
}

func newDomainReader(input []byte, domain string) (*canonicalReader, error) {
	reader := &canonicalReader{input: append([]byte(nil), input...)}
	envelope, err := reader.readString()
	if err != nil {
		return nil, err
	}
	if envelope != canonicalEnvelopeDomain {
		return nil, fmt.Errorf("unexpected canonical envelope %q", envelope)
	}
	actualDomain, err := reader.readString()
	if err != nil {
		return nil, err
	}
	if actualDomain != domain {
		return nil, fmt.Errorf("unexpected canonical domain %q", actualDomain)
	}
	return reader, nil
}

func (reader *canonicalReader) readBytes() ([]byte, error) {
	if len(reader.input)-reader.offset < 8 {
		return nil, fmt.Errorf("truncated canonical length prefix")
	}
	length := binary.BigEndian.Uint64(reader.input[reader.offset : reader.offset+8])
	reader.offset += 8
	remaining := len(reader.input) - reader.offset
	remainingValue, err := strconv.ParseUint(strconv.Itoa(remaining), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("canonical remaining byte count is invalid: %w", err)
	}
	if length > remainingValue {
		return nil, fmt.Errorf("canonical field length %d exceeds remaining bytes %d", length, remaining)
	}
	lengthValue, err := strconv.Atoi(strconv.FormatUint(length, 10))
	if err != nil {
		return nil, fmt.Errorf("canonical field length %d does not fit this runtime: %w", length, err)
	}
	end := reader.offset + lengthValue
	value := append([]byte(nil), reader.input[reader.offset:end]...)
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (reader *canonicalReader) readUint64() (uint64, error) {
	return reader.readFixedUint64()
}

func (reader *canonicalReader) readFixedUint64() (uint64, error) {
	value, err := reader.readBytes()
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("canonical uint64 requires exactly 8 bytes")
	}
	return binary.BigEndian.Uint64(value), nil
}

func (reader *canonicalReader) readFixedInt64() (int64, error) {
	value, err := reader.readBytes()
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("canonical int64 requires exactly 8 bytes")
	}
	decoded := int64(0)
	read, err := binary.Decode(value, binary.BigEndian, &decoded)
	if err != nil {
		return 0, fmt.Errorf("decode canonical int64: %w", err)
	}
	if read != len(value) {
		return 0, fmt.Errorf("decode canonical int64: read %d bytes, want %d", read, len(value))
	}
	return decoded, nil
}

func (reader *canonicalReader) readCount() (uint64, error) {
	value, err := reader.readUint64()
	if err != nil {
		return 0, err
	}
	if value > maxCollectionItems {
		return 0, fmt.Errorf("canonical collection count %d exceeds limit", value)
	}
	return value, nil
}

func (reader *canonicalReader) readValueKindRef() (ValueKindRef, error) {
	typeEnv, err := reader.readTypeEnvRef()
	if err != nil {
		return ValueKindRef{}, err
	}
	idRaw, err := reader.readString()
	if err != nil {
		return ValueKindRef{}, err
	}
	id, err := NewKindID(idRaw)
	if err != nil {
		return ValueKindRef{}, err
	}
	return NewValueKindRef(typeEnv, id)
}

func (reader *canonicalReader) readRelationSignatureRef() (RelationSignatureRef, error) {
	typeEnv, err := reader.readTypeEnvRef()
	if err != nil {
		return RelationSignatureRef{}, err
	}
	idRaw, err := reader.readString()
	if err != nil {
		return RelationSignatureRef{}, err
	}
	id, err := NewSignatureID(idRaw)
	if err != nil {
		return RelationSignatureRef{}, err
	}
	return NewRelationSignatureRef(typeEnv, id)
}

func (reader *canonicalReader) readTypeEnvRef() (TypeEnvRef, error) {
	digestRaw, err := reader.readString()
	if err != nil {
		return TypeEnvRef{}, err
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return TypeEnvRef{}, err
	}
	return NewTypeEnvRef(digest)
}

func (reader *canonicalReader) requireEnd() error {
	if reader.offset != len(reader.input) {
		return fmt.Errorf("canonical value has %d trailing bytes", len(reader.input)-reader.offset)
	}
	return nil
}
