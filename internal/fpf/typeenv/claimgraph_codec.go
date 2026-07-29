package typeenv

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	claimGraphCanonicalEnvelopeDomain = "haft.typedmemory.canonical-envelope.v1"
	claimGraphCanonicalValueDomain    = "haft.typedmemory.claim-graph-codec.v1"
	claimGraphCanonicalNodeDomain     = "haft.typedmemory.claim-node.v1"
	claimGraphCanonicalEdgeDomain     = "haft.typedmemory.claim-edge.v1"
	claimGraphCanonicalTypedDomain    = "haft.typedmemory.closed-value.v1"

	claimGraphShapeID            = "haft.ClaimGraphShapeV1"
	claimGraphCodecID            = "haft.ClaimGraphCodecV1"
	claimGraphCodecVersion       = "1"
	claimGraphRepresentationRule = "haft.fpf.claim-graph-representation.v2"

	claimGraphMaxCanonicalBytes = uint64(8 * 1024 * 1024)
	claimGraphMaxNodes          = uint64(10_000)
	claimGraphMaxEdges          = uint64(20_000)
	claimGraphMaxValueDepth     = uint64(64)
	claimGraphMaxValueItems     = uint64(100_000)
)

// ClaimGraphDecodeBudget is the immutable resource contract carried by the
// P6 codec specification. Changing any limit changes the CodecRef digest.
type ClaimGraphDecodeBudget struct {
	maxCanonicalBytes uint64
	maxNodes          uint64
	maxEdges          uint64
	maxValueDepth     uint64
	maxValueItems     uint64
}

func P6ClaimGraphDecodeBudget() ClaimGraphDecodeBudget {
	return ClaimGraphDecodeBudget{
		maxCanonicalBytes: claimGraphMaxCanonicalBytes,
		maxNodes:          claimGraphMaxNodes,
		maxEdges:          claimGraphMaxEdges,
		maxValueDepth:     claimGraphMaxValueDepth,
		maxValueItems:     claimGraphMaxValueItems,
	}
}

func (budget ClaimGraphDecodeBudget) MaxCanonicalBytes() uint64 {
	return budget.maxCanonicalBytes
}

func (budget ClaimGraphDecodeBudget) MaxNodes() uint64 { return budget.maxNodes }

func (budget ClaimGraphDecodeBudget) MaxEdges() uint64 { return budget.maxEdges }

func (budget ClaimGraphDecodeBudget) MaxValueDepth() uint64 {
	return budget.maxValueDepth
}

func (budget ClaimGraphDecodeBudget) MaxValueItems() uint64 {
	return budget.maxValueItems
}

func (budget ClaimGraphDecodeBudget) valid() bool {
	return budget.maxCanonicalBytes > 0 &&
		budget.maxNodes > 0 &&
		budget.maxEdges > 0 &&
		budget.maxValueDepth > 0 &&
		budget.maxValueItems > 0
}

// P6ClaimGraphCodecV1 is a resource-bounded adapter over the closed canonical
// codec owned by typedmemory. It does not introduce a second encoding. The
// preflight only proves that decoding fits the immutable v1 budget before the
// typedmemory codec performs semantic canonicalization.
type P6ClaimGraphCodecV1 struct {
	shape  typedmemory.ValueShapeRef
	budget ClaimGraphDecodeBudget
	inner  typedmemory.ClaimGraphCodecV1
}

func newP6ClaimGraphCodecV1(
	shape typedmemory.ValueShapeRef,
	budget ClaimGraphDecodeBudget,
) (P6ClaimGraphCodecV1, error) {
	if !budget.valid() {
		return P6ClaimGraphCodecV1{}, fmt.Errorf("ClaimGraph decode budget is required")
	}
	inner, err := typedmemory.NewClaimGraphCodecV1(shape)
	if err != nil {
		return P6ClaimGraphCodecV1{}, err
	}
	return P6ClaimGraphCodecV1{
		shape:  shape,
		budget: budget,
		inner:  inner,
	}, nil
}

func (codec P6ClaimGraphCodecV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec P6ClaimGraphCodecV1) Budget() ClaimGraphDecodeBudget {
	return codec.budget
}

func (codec P6ClaimGraphCodecV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return codec.inner.Canonicalize(expectedShape, inputBytes)
	}
	if err := preflightClaimGraphBytes(inputBytes, codec.budget); err != nil {
		return rejectClaimGraphPreflight(err)
	}
	result := codec.inner.Canonicalize(expectedShape, inputBytes)
	canonical, ok := result.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return result
	}
	if err := preflightClaimGraphBytes(canonical.CanonicalBytes(), codec.budget); err != nil {
		return rejectClaimGraphPreflight(err)
	}
	return canonical
}

// EncodeInput accepts only the sealed typedmemory ClaimGraph algebra. It has
// no JSON or map-shaped entry point.
func (codec P6ClaimGraphCodecV1) EncodeInput(
	value typedmemory.ClaimGraphValue,
) typedmemory.CodecCanonicalization {
	result := codec.inner.EncodeInput(value)
	canonical, ok := result.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return result
	}
	if err := preflightClaimGraphBytes(canonical.CanonicalBytes(), codec.budget); err != nil {
		return rejectClaimGraphPreflight(err)
	}
	return canonical
}

// P6ClaimGraphRepresentation contains the compiler-owned mechanism and the
// declarations a later TypeEnv lowerer can add. Its provenance is explicitly
// compiler-derived from exact FPF source locations; FPF is never named as the
// author of the binary shape or codec.
type P6ClaimGraphRepresentation struct {
	shapeRef         typedmemory.ValueShapeRef
	codecRef         typedmemory.CodecRef
	shapeDeclaration typedmemory.ValueShapeDeclaration
	binding          typedmemory.ValueBinding
	codec            P6ClaimGraphCodecV1
	registry         typedmemory.CodecRegistry
	provenance       typedmemory.CompilerDerivedProvenance
}

func NewP6ClaimGraphRepresentation(
	valueKind typedmemory.ValueKindRef,
	exactSourceInputs []typedmemory.SourceLocation,
) (P6ClaimGraphRepresentation, error) {
	budget := P6ClaimGraphDecodeBudget()
	shapeRef, err := newClaimGraphShapeRef()
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	codecRef, err := newClaimGraphCodecRef(shapeRef, budget)
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	provenance, err := newClaimGraphRepresentationProvenance(
		valueKind,
		shapeRef,
		codecRef,
		exactSourceInputs,
	)
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	shape := typedmemory.NewClaimGraphShape()
	shapeDeclaration, err := typedmemory.NewValueShapeDeclaration(
		shapeRef,
		shape,
		provenance,
	)
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	binding, err := typedmemory.NewValueBinding(
		valueKind,
		shapeRef,
		codecRef,
		provenance,
	)
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	codec, err := newP6ClaimGraphCodecV1(shapeRef, budget)
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	registry := typedmemory.NewCodecRegistry()
	registry, err = registry.Register(codecRef, codec)
	if err != nil {
		return P6ClaimGraphRepresentation{}, err
	}
	return P6ClaimGraphRepresentation{
		shapeRef:         shapeRef,
		codecRef:         codecRef,
		shapeDeclaration: shapeDeclaration,
		binding:          binding,
		codec:            codec,
		registry:         registry,
		provenance:       provenance,
	}, nil
}

func (representation P6ClaimGraphRepresentation) ShapeRef() typedmemory.ValueShapeRef {
	return representation.shapeRef
}

func (representation P6ClaimGraphRepresentation) CodecRef() typedmemory.CodecRef {
	return representation.codecRef
}

func (representation P6ClaimGraphRepresentation) ShapeDeclaration() typedmemory.ValueShapeDeclaration {
	return representation.shapeDeclaration
}

func (representation P6ClaimGraphRepresentation) ValueBinding() typedmemory.ValueBinding {
	return representation.binding
}

func (representation P6ClaimGraphRepresentation) Codec() P6ClaimGraphCodecV1 {
	return representation.codec
}

func (representation P6ClaimGraphRepresentation) Registry() typedmemory.CodecRegistry {
	return representation.registry
}

func (representation P6ClaimGraphRepresentation) Provenance() typedmemory.CompilerDerivedProvenance {
	return representation.provenance
}

// Seal constructs a verified value from the closed ClaimGraph algebra. The
// digest is computed by typedmemory over the exact kind/shape/codec envelope.
func (representation P6ClaimGraphRepresentation) Seal(
	value typedmemory.ClaimGraphValue,
) (typedmemory.TypedValueVerification, error) {
	encoded := representation.codec.EncodeInput(value)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, claimGraphCanonicalizationError(encoded)
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		representation.binding.ValueKind(),
		representation.shapeRef,
		representation.codecRef,
		canonical.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		return nil, err
	}
	result := typedmemory.VerifyTypedValue(
		representation.registry,
		representation.binding,
		candidate,
	)
	return result, nil
}

// VerifyCanonical is the storage/transport read boundary. It always requires
// an exact asserted typed-value digest; a caller cannot silently downgrade to
// the no-digest posture.
func (representation P6ClaimGraphRepresentation) VerifyCanonical(
	canonicalBytes []byte,
	assertedDigest typedmemory.SHA256Digest,
) (typedmemory.TypedValueVerification, error) {
	exactDigest, err := typedmemory.NewExactAssertedDigest(assertedDigest)
	if err != nil {
		return nil, err
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		representation.binding.ValueKind(),
		representation.shapeRef,
		representation.codecRef,
		canonicalBytes,
		exactDigest,
	)
	if err != nil {
		return nil, err
	}
	result := typedmemory.VerifyTypedValue(
		representation.registry,
		representation.binding,
		candidate,
	)
	return result, nil
}

func newClaimGraphShapeRef() (typedmemory.ValueShapeRef, error) {
	id, err := typedmemory.NewShapeID(claimGraphShapeID)
	if err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	shape := typedmemory.NewClaimGraphShape()
	ref, err := typedmemory.DeriveValueShapeRef(id, shape)
	if err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	return ref, nil
}

func newClaimGraphCodecRef(
	shapeRef typedmemory.ValueShapeRef,
	budget ClaimGraphDecodeBudget,
) (typedmemory.CodecRef, error) {
	id, err := typedmemory.NewCodecID(claimGraphCodecID)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	version, err := typedmemory.NewCanonicalizationVersion(claimGraphCodecVersion)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	fields := []string{
		claimGraphCodecID,
		claimGraphCodecVersion,
		shapeRef.String(),
		claimGraphCanonicalValueDomain,
		claimGraphCanonicalNodeDomain,
		claimGraphCanonicalEdgeDomain,
		claimGraphCanonicalTypedDomain,
		strconv.FormatUint(budget.maxCanonicalBytes, 10),
		strconv.FormatUint(budget.maxNodes, 10),
		strconv.FormatUint(budget.maxEdges, 10),
		strconv.FormatUint(budget.maxValueDepth, 10),
		strconv.FormatUint(budget.maxValueItems, 10),
		"unordered-node-set",
		"unordered-edge-set",
		"duplicate-node-id-rejected",
		"duplicate-edge-id-rejected",
		"dangling-endpoint-rejected",
	}
	digest, err := digestClaimGraphSpecification(
		"haft.fpf.typeenv.claim-graph-codec-spec.v1",
		fields,
	)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	return typedmemory.NewCodecRef(id, version, digest)
}

func newClaimGraphRepresentationProvenance(
	valueKind typedmemory.ValueKindRef,
	shapeRef typedmemory.ValueShapeRef,
	codecRef typedmemory.CodecRef,
	inputs []typedmemory.SourceLocation,
) (typedmemory.CompilerDerivedProvenance, error) {
	if err := validateClaimGraphSourceInputs(inputs); err != nil {
		return typedmemory.CompilerDerivedProvenance{}, err
	}
	ruleID, err := typedmemory.NewCompilerRuleID(claimGraphRepresentationRule)
	if err != nil {
		return typedmemory.CompilerDerivedProvenance{}, err
	}
	digest, err := digestClaimGraphSourceInputs(valueKind, shapeRef, codecRef, inputs)
	if err != nil {
		return typedmemory.CompilerDerivedProvenance{}, err
	}
	referenceRaw := "compiler-derived:claim-graph-representation:" + digest.String()
	reference, err := typedmemory.NewProvenanceRef(referenceRaw)
	if err != nil {
		return typedmemory.CompilerDerivedProvenance{}, err
	}
	provenance, err := typedmemory.NewCompilerDerivedProvenance(
		reference,
		inputs,
		ruleID,
	)
	if err != nil {
		return typedmemory.CompilerDerivedProvenance{}, fmt.Errorf(
			"ClaimGraph representation requires exact FPF source inputs: %w",
			err,
		)
	}
	return provenance, nil
}

func validateClaimGraphSourceInputs(inputs []typedmemory.SourceLocation) error {
	if len(inputs) == 0 {
		return fmt.Errorf("ClaimGraph representation requires exact FPF source inputs")
	}
	revision := inputs[0].Revision().String()
	for index, input := range inputs {
		if input.Revision().String() != revision {
			return fmt.Errorf(
				"ClaimGraph source input %d belongs to another publication revision",
				index,
			)
		}
	}
	return nil
}

func digestClaimGraphSourceInputs(
	valueKind typedmemory.ValueKindRef,
	shapeRef typedmemory.ValueShapeRef,
	codecRef typedmemory.CodecRef,
	inputs []typedmemory.SourceLocation,
) (typedmemory.SHA256Digest, error) {
	locations := make([]string, 0, len(inputs))
	for _, input := range inputs {
		lineRange := input.LineRange()
		patternID, hasPattern := input.PatternID()
		pattern := ""
		if hasPattern {
			pattern = patternID.String()
		}
		fields := []string{
			input.UnitID().String(),
			input.Revision().String(),
			input.ContentHash().String(),
			strconv.FormatUint(lineRange.Start(), 10),
			strconv.FormatUint(lineRange.End(), 10),
			pattern,
		}
		encoded := encodeClaimGraphDigestFields("source-location.v1", fields)
		locations = append(locations, string(encoded))
	}
	sort.Strings(locations)
	fields := []string{valueKind.String(), shapeRef.String(), codecRef.String()}
	fields = append(fields, locations...)
	return digestClaimGraphSpecification(
		"haft.fpf.typeenv.claim-graph-representation-inputs.v1",
		fields,
	)
}

func digestClaimGraphSpecification(
	domain string,
	fields []string,
) (typedmemory.SHA256Digest, error) {
	encoded := encodeClaimGraphDigestFields(domain, fields)
	sum := sha256.Sum256(encoded)
	hexValue := hex.EncodeToString(sum[:])
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hexValue)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("encode ClaimGraph SHA-256 digest: %w", err)
	}
	return digest, nil
}

func encodeClaimGraphDigestFields(domain string, fields []string) []byte {
	encoded := make([]byte, 0)
	encoded = appendClaimGraphDigestField(encoded, domain)
	for _, field := range fields {
		encoded = appendClaimGraphDigestField(encoded, field)
	}
	return encoded
}

func appendClaimGraphDigestField(target []byte, value string) []byte {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	target = append(target, length[:]...)
	target = append(target, value...)
	return target
}

func claimGraphCanonicalizationError(
	result typedmemory.CodecCanonicalization,
) error {
	rejected, ok := result.(typedmemory.RejectedCodecValue)
	if !ok {
		return fmt.Errorf("ClaimGraph codec returned incomplete result %T", result)
	}
	issues := rejected.Issues()
	if len(issues) == 0 {
		return fmt.Errorf("ClaimGraph codec rejected value without diagnostics")
	}
	return fmt.Errorf("ClaimGraph codec rejected value: %s", issues[0].Message())
}

func rejectClaimGraphPreflight(err error) typedmemory.CodecCanonicalization {
	path, _ := typedmemory.NewDiagnosticPath("claim_graph.canonical_bytes")
	issue, _ := typedmemory.NewCodecIssue(
		typedmemory.DiagnosticMalformedValue,
		"ClaimGraphCodecV1 decode budget rejected input: "+err.Error(),
		path,
	)
	rejected, _ := typedmemory.NewRejectedCodecValue([]typedmemory.CodecIssue{issue})
	return rejected
}

type claimGraphBudgetState struct {
	budget     ClaimGraphDecodeBudget
	totalNodes uint64
	totalEdges uint64
	totalItems uint64
}

func preflightClaimGraphBytes(
	input []byte,
	budget ClaimGraphDecodeBudget,
) error {
	if !budget.valid() {
		return fmt.Errorf("decode budget is invalid")
	}
	if uint64(len(input)) > budget.maxCanonicalBytes {
		return fmt.Errorf(
			"canonical bytes %d exceed limit %d",
			len(input),
			budget.maxCanonicalBytes,
		)
	}
	state := claimGraphBudgetState{budget: budget}
	return state.scanClaimGraph(input, 1)
}

func (state *claimGraphBudgetState) scanClaimGraph(
	input []byte,
	valueDepth uint64,
) error {
	if valueDepth > state.budget.maxValueDepth {
		return fmt.Errorf(
			"value depth %d exceeds limit %d",
			valueDepth,
			state.budget.maxValueDepth,
		)
	}
	reader, err := newClaimGraphBudgetReader(input, claimGraphCanonicalValueDomain)
	if err != nil {
		return err
	}
	nodeCount, err := reader.readCount()
	if err != nil {
		return err
	}
	if err := state.consumeNodes(nodeCount); err != nil {
		return err
	}
	for index := uint64(0); index < nodeCount; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			return readErr
		}
		if scanErr := state.scanClaimNode(encoded, valueDepth); scanErr != nil {
			return fmt.Errorf("claim node %d: %w", index, scanErr)
		}
	}
	edgeCount, err := reader.readCount()
	if err != nil {
		return err
	}
	if err := state.consumeEdges(edgeCount); err != nil {
		return err
	}
	for index := uint64(0); index < edgeCount; index++ {
		encoded, readErr := reader.readBytes()
		if readErr != nil {
			return readErr
		}
		if scanErr := scanClaimEdgeEnvelope(encoded); scanErr != nil {
			return fmt.Errorf("claim edge %d: %w", index, scanErr)
		}
	}
	return reader.requireEnd()
}

func (state *claimGraphBudgetState) scanClaimNode(
	input []byte,
	valueDepth uint64,
) error {
	reader, err := newClaimGraphBudgetReader(input, claimGraphCanonicalNodeDomain)
	if err != nil {
		return err
	}
	if _, err := reader.readString(); err != nil {
		return err
	}
	if _, err := reader.readString(); err != nil {
		return err
	}
	if _, err := reader.readString(); err != nil {
		return err
	}
	valueBytes, err := reader.readBytes()
	if err != nil {
		return err
	}
	if err := state.scanTypedValue(valueBytes, valueDepth); err != nil {
		return err
	}
	return reader.requireEnd()
}

func scanClaimEdgeEnvelope(input []byte) error {
	reader, err := newClaimGraphBudgetReader(input, claimGraphCanonicalEdgeDomain)
	if err != nil {
		return err
	}
	for field := 0; field < 5; field++ {
		if _, err := reader.readString(); err != nil {
			return err
		}
	}
	return reader.requireEnd()
}

func (state *claimGraphBudgetState) scanTypedValue(
	input []byte,
	depth uint64,
) error {
	if depth > state.budget.maxValueDepth {
		return fmt.Errorf(
			"value depth %d exceeds limit %d",
			depth,
			state.budget.maxValueDepth,
		)
	}
	if err := state.consumeItems(1); err != nil {
		return err
	}
	reader, err := newClaimGraphBudgetReader(input, claimGraphCanonicalTypedDomain)
	if err != nil {
		return err
	}
	kind, err := reader.readString()
	if err != nil {
		return err
	}
	switch typedmemory.TypedValueKind(kind) {
	case typedmemory.TypedValueScalar:
		err = scanScalarTypedValue(reader)
	case typedmemory.TypedValueRecord:
		err = state.scanRecordTypedValue(reader, depth)
	case typedmemory.TypedValueSum:
		err = state.scanSumTypedValue(reader, depth)
	case typedmemory.TypedValueOrderedSequence,
		typedmemory.TypedValueUnorderedSet:
		err = state.scanSequenceTypedValue(reader, depth)
	case typedmemory.TypedValueClaimGraph:
		err = state.scanNestedClaimGraph(reader, depth)
	default:
		err = fmt.Errorf("unknown closed TypedValue kind %q", kind)
	}
	if err != nil {
		return err
	}
	return reader.requireEnd()
}

func scanScalarTypedValue(reader *claimGraphBudgetReader) error {
	kind, err := reader.readString()
	if err != nil {
		return err
	}
	switch typedmemory.ScalarKind(kind) {
	case typedmemory.ScalarText,
		typedmemory.ScalarBoolean,
		typedmemory.ScalarBytes:
		_, err = reader.readBytes()
		return err
	case typedmemory.ScalarSignedInteger,
		typedmemory.ScalarUnsignedInteger:
		_, err = reader.readUint64()
		return err
	default:
		return fmt.Errorf("unknown scalar kind %q", kind)
	}
}

func (state *claimGraphBudgetState) scanRecordTypedValue(
	reader *claimGraphBudgetReader,
	depth uint64,
) error {
	count, err := reader.readCount()
	if err != nil {
		return err
	}
	if err := state.consumeItems(count); err != nil {
		return err
	}
	for index := uint64(0); index < count; index++ {
		if _, err := reader.readString(); err != nil {
			return err
		}
		encoded, err := reader.readBytes()
		if err != nil {
			return err
		}
		if err := state.scanTypedValue(encoded, depth+1); err != nil {
			return fmt.Errorf("record field %d: %w", index, err)
		}
	}
	return nil
}

func (state *claimGraphBudgetState) scanSumTypedValue(
	reader *claimGraphBudgetReader,
	depth uint64,
) error {
	if err := state.consumeItems(1); err != nil {
		return err
	}
	if _, err := reader.readString(); err != nil {
		return err
	}
	encoded, err := reader.readBytes()
	if err != nil {
		return err
	}
	return state.scanTypedValue(encoded, depth+1)
}

func (state *claimGraphBudgetState) scanSequenceTypedValue(
	reader *claimGraphBudgetReader,
	depth uint64,
) error {
	count, err := reader.readCount()
	if err != nil {
		return err
	}
	if err := state.consumeItems(count); err != nil {
		return err
	}
	for index := uint64(0); index < count; index++ {
		encoded, err := reader.readBytes()
		if err != nil {
			return err
		}
		if err := state.scanTypedValue(encoded, depth+1); err != nil {
			return fmt.Errorf("sequence item %d: %w", index, err)
		}
	}
	return nil
}

func (state *claimGraphBudgetState) scanNestedClaimGraph(
	reader *claimGraphBudgetReader,
	depth uint64,
) error {
	encoded, err := reader.readBytes()
	if err != nil {
		return err
	}
	return state.scanClaimGraph(encoded, depth+1)
}

func (state *claimGraphBudgetState) consumeNodes(count uint64) error {
	if count > state.budget.maxNodes-state.totalNodes {
		return fmt.Errorf(
			"ClaimGraph nodes exceed limit %d",
			state.budget.maxNodes,
		)
	}
	state.totalNodes += count
	return nil
}

func (state *claimGraphBudgetState) consumeEdges(count uint64) error {
	if count > state.budget.maxEdges-state.totalEdges {
		return fmt.Errorf(
			"ClaimGraph edges exceed limit %d",
			state.budget.maxEdges,
		)
	}
	state.totalEdges += count
	return nil
}

func (state *claimGraphBudgetState) consumeItems(count uint64) error {
	if count > state.budget.maxValueItems-state.totalItems {
		return fmt.Errorf(
			"closed value items exceed limit %d",
			state.budget.maxValueItems,
		)
	}
	state.totalItems += count
	return nil
}

type claimGraphBudgetReader struct {
	input  []byte
	offset int
}

func newClaimGraphBudgetReader(
	input []byte,
	domain string,
) (*claimGraphBudgetReader, error) {
	reader := &claimGraphBudgetReader{input: input}
	envelope, err := reader.readString()
	if err != nil {
		return nil, err
	}
	if envelope != claimGraphCanonicalEnvelopeDomain {
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

func (reader *claimGraphBudgetReader) readBytes() ([]byte, error) {
	remaining := len(reader.input) - reader.offset
	if remaining < 8 {
		return nil, fmt.Errorf("truncated canonical length prefix")
	}
	start := reader.offset
	end := start + 8
	length := binary.BigEndian.Uint64(reader.input[start:end])
	reader.offset = end
	remaining = len(reader.input) - reader.offset
	//nolint:gosec // remaining is non-negative after the explicit prefix bounds check.
	if length > uint64(remaining) {
		return nil, fmt.Errorf(
			"canonical field length %d exceeds remaining bytes %d",
			length,
			remaining,
		)
	}
	if length > claimGraphMaxCanonicalBytes {
		return nil, fmt.Errorf(
			"canonical field length %d exceeds remaining bytes %d",
			length,
			remaining,
		)
	}
	boundedLength := int(length)
	valueEnd := reader.offset + boundedLength
	value := reader.input[reader.offset:valueEnd]
	reader.offset = valueEnd
	return value, nil
}

func (reader *claimGraphBudgetReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (reader *claimGraphBudgetReader) readCount() (uint64, error) {
	return reader.readUint64()
}

func (reader *claimGraphBudgetReader) readUint64() (uint64, error) {
	value, err := reader.readBytes()
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("canonical uint64 requires exactly 8 bytes")
	}
	return binary.BigEndian.Uint64(value), nil
}

func (reader *claimGraphBudgetReader) requireEnd() error {
	if reader.offset != len(reader.input) {
		return fmt.Errorf(
			"canonical value has %d trailing bytes",
			len(reader.input)-reader.offset,
		)
	}
	return nil
}

// Compile-time assertions keep this adapter on the executable typedmemory
// mechanism boundary and prevent an accidental parallel codec interface.
var _ typedmemory.CodecImplementation = P6ClaimGraphCodecV1{}
