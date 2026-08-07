package typedmemory

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const typedValueDigestDomain = "haft.typedmemory.verified-typed-value.v1"

// CodecIssue is a known contradiction found by an available codec. Missing
// codec mechanism is represented separately as UnderdeterminedTypedValue.
type CodecIssue struct {
	code    DiagnosticCode
	message string
	path    DiagnosticPath
	witness ExpectedActualWitness
	repairs []RepairCandidate
}

func NewCodecIssue(code DiagnosticCode, message string, path DiagnosticPath) (CodecIssue, error) {
	witness := unknownExpectedActualWitness("codec implementation did not retain exact values")
	repairs := []RepairCandidate{defaultInvalidRepair(code, path, witness.Expected())}
	return NewCodecIssueWithDetails(code, message, path, witness, repairs)
}

func NewCodecIssueWithDetails(
	code DiagnosticCode,
	message string,
	path DiagnosticPath,
	witness ExpectedActualWitness,
	repairs []RepairCandidate,
) (CodecIssue, error) {
	if !code.valid() {
		return CodecIssue{}, fmt.Errorf("codec issue code is required")
	}
	if strings.TrimSpace(message) == "" {
		return CodecIssue{}, fmt.Errorf("codec issue message is required")
	}
	if !path.valid() {
		return CodecIssue{}, fmt.Errorf("codec issue path is required")
	}
	if !validDiagnosticWitness(witness) {
		return CodecIssue{}, fmt.Errorf("codec issue witness is required")
	}
	ownedRepairs, err := validateRepairCandidates(repairs)
	if err != nil {
		return CodecIssue{}, err
	}
	return CodecIssue{
		code:    code,
		message: strings.TrimSpace(message),
		path:    path,
		witness: witness,
		repairs: ownedRepairs,
	}, nil
}

func (issue CodecIssue) Code() DiagnosticCode { return issue.code }

func (issue CodecIssue) Message() string { return issue.message }

func (issue CodecIssue) Path() DiagnosticPath { return issue.path }

func (issue CodecIssue) Witness() ExpectedActualWitness { return issue.witness }

func (issue CodecIssue) RepairCandidates() []RepairCandidate {
	return append([]RepairCandidate(nil), issue.repairs...)
}

func (issue CodecIssue) valid() bool {
	if !issue.code.valid() ||
		issue.message == "" ||
		!issue.path.valid() ||
		!validDiagnosticWitness(issue.witness) {
		return false
	}
	_, err := validateRepairCandidates(issue.repairs)
	return err == nil
}

type CodecCanonicalization interface {
	codecCanonicalizationVariant()
}

type CanonicalizedCodecValue struct {
	value          TypedValue
	canonicalBytes []byte
}

func NewCanonicalizedCodecValue(value TypedValue, canonicalBytes []byte) (CanonicalizedCodecValue, error) {
	if !validTypedValue(value) {
		return CanonicalizedCodecValue{}, fmt.Errorf("canonicalized codec value must belong to the closed TypedValue algebra")
	}
	if len(canonicalBytes) == 0 {
		return CanonicalizedCodecValue{}, fmt.Errorf("canonicalized codec bytes are required")
	}
	return CanonicalizedCodecValue{
		value:          value,
		canonicalBytes: append([]byte(nil), canonicalBytes...),
	}, nil
}

func (value CanonicalizedCodecValue) Value() TypedValue { return value.value }

func (value CanonicalizedCodecValue) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value CanonicalizedCodecValue) valid() bool {
	return validTypedValue(value.value) && len(value.canonicalBytes) > 0
}

func (CanonicalizedCodecValue) codecCanonicalizationVariant() {}

type RejectedCodecValue struct {
	issues []CodecIssue
}

func NewRejectedCodecValue(issues []CodecIssue) (RejectedCodecValue, error) {
	if len(issues) == 0 {
		return RejectedCodecValue{}, fmt.Errorf("rejected codec value requires at least one issue")
	}
	owned := append([]CodecIssue(nil), issues...)
	for index, issue := range owned {
		if !issue.valid() {
			return RejectedCodecValue{}, fmt.Errorf("codec issue at index %d is incomplete", index)
		}
	}
	return RejectedCodecValue{issues: owned}, nil
}

func (value RejectedCodecValue) Issues() []CodecIssue {
	return append([]CodecIssue(nil), value.issues...)
}

func (value RejectedCodecValue) valid() bool {
	if len(value.issues) == 0 {
		return false
	}
	for _, issue := range value.issues {
		if !issue.valid() {
			return false
		}
	}
	return true
}

func (RejectedCodecValue) codecCanonicalizationVariant() {}

// CodecImplementation is pure executable mechanism. Its presence in a
// registry does not admit a ValueKind or ValueShape into a project TypeEnv.
// Implementations must perform decode, shape-check, normalize, and canonical
// encode as one deterministic operation.
type CodecImplementation interface {
	Canonicalize(expectedShape ValueShapeRef, inputBytes []byte) CodecCanonicalization
}

type registeredCodec struct {
	ref            CodecRef
	implementation CodecImplementation
}

// CodecRegistry is immutable-by-operation: Register returns a new registry and
// leaves the receiver unchanged. An existing CodecRef can never be replaced in
// place because that would silently mutate historical canonical semantics.
type CodecRegistry struct {
	entries map[string]registeredCodec
}

func NewCodecRegistry() CodecRegistry {
	return CodecRegistry{entries: map[string]registeredCodec{}}
}

func (registry CodecRegistry) Register(
	ref CodecRef,
	implementation CodecImplementation,
) (CodecRegistry, error) {
	if !ref.valid() {
		return CodecRegistry{}, fmt.Errorf("registered CodecRef is required")
	}
	if !codecImplementationPresent(implementation) {
		return CodecRegistry{}, fmt.Errorf("codec implementation is required")
	}
	if _, exists := registry.entries[ref.String()]; exists {
		return CodecRegistry{}, fmt.Errorf("CodecRef %q is already registered and cannot be replaced", ref.String())
	}

	entries := make(map[string]registeredCodec, len(registry.entries)+1)
	for key, value := range registry.entries {
		entries[key] = value
	}
	entries[ref.String()] = registeredCodec{ref: ref, implementation: implementation}
	return CodecRegistry{entries: entries}, nil
}

func (registry CodecRegistry) Resolve(ref CodecRef) (CodecImplementation, bool) {
	entry, exists := registry.entries[ref.String()]
	if !exists {
		return nil, false
	}
	return entry.implementation, true
}

func (registry CodecRegistry) Contains(ref CodecRef) bool {
	_, exists := registry.entries[ref.String()]
	return exists
}

func (registry CodecRegistry) Len() int { return len(registry.entries) }

func codecImplementationPresent(implementation CodecImplementation) bool {
	if implementation == nil {
		return false
	}
	value := reflect.ValueOf(implementation)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

type TypedValueVerification interface {
	typedValueVerificationVariant()
}

type ValidTypedValue struct {
	value VerifiedTypedValue
}

func (result ValidTypedValue) Value() VerifiedTypedValue { return result.value }

func (ValidTypedValue) typedValueVerificationVariant() {}

type InvalidTypedValue struct {
	diagnostics []Diagnostic
}

func (result InvalidTypedValue) Diagnostics() []Diagnostic {
	return copyDiagnostics(result.diagnostics)
}

func (InvalidTypedValue) typedValueVerificationVariant() {}

type UnderdeterminedTypedValue struct {
	diagnostics []Diagnostic
}

func (result UnderdeterminedTypedValue) Diagnostics() []Diagnostic {
	return copyDiagnostics(result.diagnostics)
}

func (UnderdeterminedTypedValue) typedValueVerificationVariant() {}

// VerifyTypedValue is the only constructor path for VerifiedTypedValue. The
// caller must first resolve the exact active ValueBinding from the TypeEnv;
// registry presence alone never supplies that admission.
func VerifyTypedValue(
	registry CodecRegistry,
	binding ValueBinding,
	candidate TypedValueCandidate,
) TypedValueVerification {
	if !binding.valid() {
		diagnostic := newValueUnderdeterminedDiagnosticWithRequired(
			DiagnosticValueBindingNotActive,
			"the active TypeEnv did not supply an exact value binding",
			"typed_value.binding",
			"activate-or-compile-the-exact-value-binding",
			diagnosticState("active exact ValueBinding"),
		)
		return UnderdeterminedTypedValue{diagnostics: []Diagnostic{diagnostic}}
	}
	if !candidate.valid() {
		diagnostic := newValueInvalidDiagnosticWithWitness(
			DiagnosticMalformedValue,
			"typed-value candidate envelope is incomplete",
			"typed_value.candidate",
			diagnosticState("complete typed-value candidate envelope"),
			diagnosticState("incomplete"),
		)
		return InvalidTypedValue{diagnostics: []Diagnostic{diagnostic}}
	}

	mismatches := compareCandidateWithBinding(binding, candidate)
	if len(mismatches) > 0 {
		return InvalidTypedValue{diagnostics: mismatches}
	}

	implementation, available := registry.Resolve(binding.Codec())
	if !available {
		diagnostic := newValueUnderdeterminedDiagnosticWithRequired(
			DiagnosticCodecUnavailable,
			fmt.Sprintf("codec implementation %s is not registered", binding.Codec().String()),
			"typed_value.codec",
			"register-the-exact-codec-ref",
			diagnosticReference(binding.Codec().String()),
		)
		return UnderdeterminedTypedValue{diagnostics: []Diagnostic{diagnostic}}
	}

	firstPass := implementation.Canonicalize(binding.ValueShape(), candidate.InputBytes())
	canonicalized, rejected := codecCanonicalizedValue(firstPass)
	if rejected != nil {
		return InvalidTypedValue{diagnostics: diagnosticsFromCodecIssues(rejected.Issues())}
	}

	secondPass := implementation.Canonicalize(binding.ValueShape(), canonicalized.CanonicalBytes())
	roundTrip, roundTripRejected := codecCanonicalizedValue(secondPass)
	if roundTripRejected != nil {
		return InvalidTypedValue{diagnostics: diagnosticsFromCodecIssues(roundTripRejected.Issues())}
	}
	if !bytes.Equal(canonicalized.CanonicalBytes(), roundTrip.CanonicalBytes()) {
		diagnostic := newValueInvalidDiagnosticWithWitness(
			DiagnosticMalformedValue,
			"codec canonical round-trip changed canonical bytes",
			"typed_value.canonical_bytes",
			diagnosticByteDigest(canonicalized.CanonicalBytes()),
			diagnosticByteDigest(roundTrip.CanonicalBytes()),
		)
		return InvalidTypedValue{diagnostics: []Diagnostic{diagnostic}}
	}

	digest := digestTypedValue(
		binding.ValueKind(),
		binding.ValueShape(),
		binding.Codec(),
		canonicalized.CanonicalBytes(),
	)
	asserted, hasAsserted := candidate.AssertedDigest().Digest()
	if hasAsserted && asserted != digest {
		diagnostic := newValueInvalidDiagnosticWithWitness(
			DiagnosticTypedValueDigestMismatch,
			fmt.Sprintf("asserted digest %s does not match canonical digest %s", asserted.String(), digest.String()),
			"typed_value.digest",
			diagnosticReference(digest.String()),
			diagnosticReference(asserted.String()),
		)
		return InvalidTypedValue{diagnostics: []Diagnostic{diagnostic}}
	}

	verified := verifiedTypedValue{
		valueKind:      binding.ValueKind(),
		valueShape:     binding.ValueShape(),
		codec:          binding.Codec(),
		canonicalBytes: canonicalized.CanonicalBytes(),
		digest:         digest,
	}
	return ValidTypedValue{value: verified}
}

func compareCandidateWithBinding(binding ValueBinding, candidate TypedValueCandidate) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, 3)
	if candidate.ValueKind() != binding.ValueKind() {
		diagnostics = append(diagnostics, newValueInvalidDiagnosticWithWitness(
			DiagnosticValueKindMismatch,
			"candidate ValueKindRef does not match the active binding",
			"typed_value.value_kind_ref",
			diagnosticReference(binding.ValueKind().String()),
			diagnosticReference(candidate.ValueKind().String()),
		))
	}
	if candidate.ValueShape() != binding.ValueShape() {
		diagnostics = append(diagnostics, newValueInvalidDiagnosticWithWitness(
			DiagnosticValueShapeMismatch,
			"candidate ValueShapeRef does not match the active binding",
			"typed_value.value_shape_ref",
			diagnosticReference(binding.ValueShape().String()),
			diagnosticReference(candidate.ValueShape().String()),
		))
	}
	if candidate.Codec() != binding.Codec() {
		diagnostics = append(diagnostics, newValueInvalidDiagnosticWithWitness(
			DiagnosticCodecRefMismatch,
			"candidate CodecRef does not match the active binding",
			"typed_value.codec_ref",
			diagnosticReference(binding.Codec().String()),
			diagnosticReference(candidate.Codec().String()),
		))
	}
	return diagnostics
}

func codecCanonicalizedValue(
	result CodecCanonicalization,
) (CanonicalizedCodecValue, *RejectedCodecValue) {
	switch value := result.(type) {
	case CanonicalizedCodecValue:
		if !value.valid() {
			break
		}
		return value, nil
	case RejectedCodecValue:
		if !value.valid() {
			break
		}
		return CanonicalizedCodecValue{}, &value
	}
	issue := newCodecIssueWithWitness(
		DiagnosticMalformedValue,
		"codec returned an incomplete or unknown canonicalization result",
		"typed_value.codec_result",
		diagnosticSet([]string{"CanonicalizedCodecValue", "RejectedCodecValue"}),
		diagnosticText(fmt.Sprintf("%T", result)),
	)
	rejected := RejectedCodecValue{issues: []CodecIssue{issue}}
	return CanonicalizedCodecValue{}, &rejected
}

func diagnosticsFromCodecIssues(issues []CodecIssue) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, len(issues))
	for _, issue := range issues {
		basis, _ := NewCoreValidatorBasis(
			RuleRef{value: "typedmemory.codec.canonicalization.v1"},
		)
		diagnostic, _ := NewInvalidDiagnosticWithDetails(
			issue.code,
			issue.message,
			issue.path,
			issue.witness,
			basis,
			issue.repairs,
		)
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

func digestTypedValue(
	valueKind ValueKindRef,
	valueShape ValueShapeRef,
	codec CodecRef,
	canonicalBytes []byte,
) SHA256Digest {
	writer := newCanonicalWriter(typedValueDigestDomain)
	writer.addString(valueKind.String())
	writer.addString(valueShape.String())
	writer.addString(codec.String())
	writer.addBytes(canonicalBytes)
	return writer.digest()
}

// ComputeTypedValueDigest recomputes the exact domain-separated digest stored
// for canonical typed-value bytes. It does not construct or certify a
// VerifiedTypedValue; callers must still recover the applicable TypeEnv and
// codec evidence separately.
func ComputeTypedValueDigest(
	valueKind ValueKindRef,
	valueShape ValueShapeRef,
	codec CodecRef,
	canonicalBytes []byte,
) (SHA256Digest, error) {
	if !valueKind.valid() || !valueShape.valid() || !codec.valid() {
		return SHA256Digest{}, fmt.Errorf("typed-value digest requires exact kind, shape, and codec refs")
	}
	if len(canonicalBytes) == 0 {
		return SHA256Digest{}, fmt.Errorf("typed-value digest requires non-empty canonical bytes")
	}
	return digestTypedValue(valueKind, valueShape, codec, canonicalBytes), nil
}

// VerifyStoredTypedValueDigest verifies a durable value row from its exact
// canonical reference strings without making the storage adapter duplicate
// those reference grammars. It verifies identity only; it does not recreate
// codec execution evidence or construct a VerifiedTypedValue.
func VerifyStoredTypedValueDigest(
	valueKindRaw string,
	valueShapeRaw string,
	codecRaw string,
	canonicalBytes []byte,
	asserted SHA256Digest,
) error {
	if !asserted.valid() {
		return fmt.Errorf("stored typed-value digest is required")
	}
	valueKind, err := parseCanonicalValueKindRef(valueKindRaw)
	if err != nil {
		return fmt.Errorf("stored typed-value kind: %w", err)
	}
	valueShape, err := parseCanonicalValueShapeRef(valueShapeRaw)
	if err != nil {
		return fmt.Errorf("stored typed-value shape: %w", err)
	}
	codec, err := parseCanonicalCodecRef(codecRaw)
	if err != nil {
		return fmt.Errorf("stored typed-value codec: %w", err)
	}
	computed, err := ComputeTypedValueDigest(valueKind, valueShape, codec, canonicalBytes)
	if err != nil {
		return err
	}
	if computed != asserted {
		return fmt.Errorf(
			"stored typed-value digest %s does not match canonical digest %s",
			asserted.String(),
			computed.String(),
		)
	}
	return nil
}

func parseCanonicalValueKindRef(raw string) (ValueKindRef, error) {
	typeEnvRaw, kindRaw, found := strings.Cut(raw, "/value-kind/")
	if !found || !strings.HasPrefix(typeEnvRaw, "typeenv:") {
		return ValueKindRef{}, fmt.Errorf("canonical ValueKindRef is malformed")
	}
	typeEnvDigest, err := NewSHA256Digest(strings.TrimPrefix(typeEnvRaw, "typeenv:"))
	if err != nil {
		return ValueKindRef{}, err
	}
	typeEnv, err := NewTypeEnvRef(typeEnvDigest)
	if err != nil {
		return ValueKindRef{}, err
	}
	kind, err := NewKindID(kindRaw)
	if err != nil {
		return ValueKindRef{}, err
	}
	ref, err := NewValueKindRef(typeEnv, kind)
	if err != nil {
		return ValueKindRef{}, err
	}
	if ref.String() != raw {
		return ValueKindRef{}, fmt.Errorf("ValueKindRef is not in canonical form")
	}
	return ref, nil
}

func parseCanonicalValueShapeRef(raw string) (ValueShapeRef, error) {
	if !strings.HasPrefix(raw, "shape:") {
		return ValueShapeRef{}, fmt.Errorf("canonical ValueShapeRef is malformed")
	}
	body := strings.TrimPrefix(raw, "shape:")
	digestMarker := strings.LastIndex(body, "@sha256:")
	if digestMarker <= 0 {
		return ValueShapeRef{}, fmt.Errorf("canonical ValueShapeRef digest is missing")
	}
	shape, err := NewShapeID(body[:digestMarker])
	if err != nil {
		return ValueShapeRef{}, err
	}
	digest, err := NewSHA256Digest(body[digestMarker+1:])
	if err != nil {
		return ValueShapeRef{}, err
	}
	ref, err := NewValueShapeRef(shape, digest)
	if err != nil {
		return ValueShapeRef{}, err
	}
	if ref.String() != raw {
		return ValueShapeRef{}, fmt.Errorf("ValueShapeRef is not in canonical form")
	}
	return ref, nil
}

func parseCanonicalCodecRef(raw string) (CodecRef, error) {
	if !strings.HasPrefix(raw, "codec:") {
		return CodecRef{}, fmt.Errorf("canonical CodecRef is malformed")
	}
	idRaw, remaining, err := parseCanonicalLengthSegment(strings.TrimPrefix(raw, "codec:"))
	if err != nil {
		return CodecRef{}, err
	}
	versionRaw, digestRaw, err := parseCanonicalLengthSegment(remaining)
	if err != nil {
		return CodecRef{}, err
	}
	id, err := NewCodecID(idRaw)
	if err != nil {
		return CodecRef{}, err
	}
	version, err := NewCanonicalizationVersion(versionRaw)
	if err != nil {
		return CodecRef{}, err
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return CodecRef{}, err
	}
	ref, err := NewCodecRef(id, version, digest)
	if err != nil {
		return CodecRef{}, err
	}
	if ref.String() != raw {
		return CodecRef{}, fmt.Errorf("CodecRef is not in canonical form")
	}
	return ref, nil
}

func parseCanonicalLengthSegment(raw string) (string, string, error) {
	separator := strings.IndexByte(raw, ':')
	if separator <= 0 {
		return "", "", fmt.Errorf("canonical length-prefixed segment is malformed")
	}
	lengthRaw := raw[:separator]
	length, err := strconv.Atoi(lengthRaw)
	if err != nil || length < 0 || strconv.Itoa(length) != lengthRaw {
		return "", "", fmt.Errorf("canonical length-prefixed segment has an invalid length")
	}
	payloadAndTail := raw[separator+1:]
	if len(payloadAndTail) <= length || payloadAndTail[length] != ':' {
		return "", "", fmt.Errorf("canonical length-prefixed segment has inconsistent bytes")
	}
	return payloadAndTail[:length], payloadAndTail[length+1:], nil
}

func newCodecIssueWithWitness(
	code DiagnosticCode,
	message string,
	path string,
	expected DiagnosticDatum,
	actual DiagnosticDatum,
) CodecIssue {
	diagnosticPath := DiagnosticPath{value: path}
	witness, _ := NewExpectedActualWitness(expected, actual)
	repairs := []RepairCandidate{defaultInvalidRepair(code, diagnosticPath, expected)}
	issue, _ := NewCodecIssueWithDetails(code, message, diagnosticPath, witness, repairs)
	return issue
}

func newValueInvalidDiagnosticWithWitness(
	code DiagnosticCode,
	message string,
	path string,
	expected DiagnosticDatum,
	actual DiagnosticDatum,
) Diagnostic {
	diagnosticPath := DiagnosticPath{value: path}
	witness, _ := NewExpectedActualWitness(expected, actual)
	basis, _ := NewCoreValidatorBasis(
		RuleRef{value: "typedmemory.value-verification.v1"},
	)
	repairs := []RepairCandidate{defaultInvalidRepair(code, diagnosticPath, expected)}
	diagnostic, _ := NewInvalidDiagnosticWithDetails(
		code,
		strings.TrimSpace(message),
		diagnosticPath,
		witness,
		basis,
		repairs,
	)
	return diagnostic
}

func newValueUnderdeterminedDiagnosticWithRequired(
	code DiagnosticCode,
	message string,
	path string,
	repair string,
	required DiagnosticDatum,
) Diagnostic {
	diagnosticPath := DiagnosticPath{value: path}
	witness, _ := NewMissingBasisWitness(
		required,
		"typed-value check could not observe an actual value without the missing basis",
	)
	basis, _ := NewMissingRuntimeBasis(missingRuntimeKind(code), required)
	repairPointer := RepairPointer{value: repair}
	repairs := []RepairCandidate{
		defaultMissingBasisRepair(code, basis, repairPointer, required),
	}
	diagnostic, _ := NewUnderdeterminedDiagnosticWithDetails(
		code,
		strings.TrimSpace(message),
		diagnosticPath,
		witness,
		basis,
		repairPointer,
		repairs,
	)
	return diagnostic
}

func diagnosticByteDigest(value []byte) DiagnosticDatum {
	writer := newCanonicalWriter("typedmemory.diagnostic-byte-witness.v1")
	writer.addBytes(value)
	return diagnosticReference(writer.digest().String())
}
