// Package typeenv owns the source-derived, content-addressed FPF TypeEnv
// artifact. It deliberately keeps publication parsing and durable storage out
// of the pure sealing boundary.
package typeenv

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	artifactPayloadDomain = "base-typeenv-artifact-payload.v1"
	artifactWireDomain    = "base-typeenv-artifact-wire.v1"

	maximumArtifactPayloadBytes       = 16 << 20
	maximumAggregateScalarBytes       = 8 << 20
	maximumScalarStringBytes          = 256 << 10
	maximumScalarByteValueBytes       = 2 << 20
	maximumDeclarations               = 8 << 10
	maximumCoverageEntries            = 16 << 10
	maximumFieldsPerRecord            = 256
	maximumValuesPerCollection        = 2 << 10
	maximumDeclarationDepth           = 16
	maximumDeclarationValueNodes      = 128 << 10
	maximumDeclarationFields          = 64 << 10
	maximumDependenciesPerDeclaration = 2 << 10
	maximumSourcesPerBasis            = 256
)

type artifactBudget struct {
	scalarBytes int
	valueNodes  int
	fields      int
}

func (budget *artifactBudget) consumeScalar(label string, size int, maximum int) error {
	if size > maximum {
		return fmt.Errorf("%s exceeds %d bytes", label, maximum)
	}
	next := budget.scalarBytes + size
	if next > maximumAggregateScalarBytes {
		return fmt.Errorf("artifact scalar content exceeds %d bytes", maximumAggregateScalarBytes)
	}
	budget.scalarBytes = next
	return nil
}

func (budget *artifactBudget) consumeValue(depth int) error {
	if depth > maximumDeclarationDepth {
		return fmt.Errorf("declaration value depth exceeds %d", maximumDeclarationDepth)
	}
	budget.valueNodes++
	if budget.valueNodes > maximumDeclarationValueNodes {
		return fmt.Errorf("artifact exceeds %d declaration value nodes", maximumDeclarationValueNodes)
	}
	return nil
}

func (budget *artifactBudget) consumeFields(count int) error {
	if count > maximumFieldsPerRecord {
		return fmt.Errorf("declaration record exceeds %d fields", maximumFieldsPerRecord)
	}
	budget.fields += count
	if budget.fields > maximumDeclarationFields {
		return fmt.Errorf("artifact exceeds %d declaration fields", maximumDeclarationFields)
	}
	return nil
}

// ArtifactPosture makes absence of an executable environment explicit. A
// coverage-only result can preserve honest gaps without minting a TypeEnvRef.
type ArtifactPosture uint8

const (
	CompiledEnvironment ArtifactPosture = iota + 1
	CoverageOnly
)

func (posture ArtifactPosture) String() string {
	switch posture {
	case CompiledEnvironment:
		return "compiled_environment"
	case CoverageOnly:
		return "coverage_only"
	default:
		return ""
	}
}

func (posture ArtifactPosture) valid() bool {
	return posture.String() != ""
}

// DeclarationValueKind is the closed, semantics-neutral algebra used by the
// compiler to represent a declaration before TypeEnvRef-bearing lowering.
type DeclarationValueKind uint8

const (
	DeclarationText DeclarationValueKind = iota + 1
	DeclarationUnsigned
	DeclarationBoolean
	DeclarationBytes
	DeclarationSymbol
	DeclarationSequence
	DeclarationSet
	DeclarationRecord
)

// DeclarationValue cannot contain a TypeEnvRef. Cross-declaration links use
// SchemaSymbolRef, so the linked IR remains self-reference-free before sealing.
type DeclarationValue interface {
	Kind() DeclarationValueKind
	declarationValueVariant()
}

type TextValue struct {
	value string
}

func NewTextValue(value string) TextValue {
	return TextValue{value: value}
}

func (value TextValue) Kind() DeclarationValueKind { return DeclarationText }

func (value TextValue) Value() string { return value.value }

func (TextValue) declarationValueVariant() {}

type UnsignedValue struct {
	value uint64
}

func NewUnsignedValue(value uint64) UnsignedValue {
	return UnsignedValue{value: value}
}

func (value UnsignedValue) Kind() DeclarationValueKind { return DeclarationUnsigned }

func (value UnsignedValue) Value() uint64 { return value.value }

func (UnsignedValue) declarationValueVariant() {}

type BooleanValue struct {
	value bool
}

func NewBooleanValue(value bool) BooleanValue {
	return BooleanValue{value: value}
}

func (value BooleanValue) Kind() DeclarationValueKind { return DeclarationBoolean }

func (value BooleanValue) Value() bool { return value.value }

func (BooleanValue) declarationValueVariant() {}

type BytesValue struct {
	value []byte
}

func NewBytesValue(value []byte) BytesValue {
	owned := append([]byte(nil), value...)
	return BytesValue{value: owned}
}

func (value BytesValue) Kind() DeclarationValueKind { return DeclarationBytes }

func (value BytesValue) Value() []byte { return append([]byte(nil), value.value...) }

func (BytesValue) declarationValueVariant() {}

type SymbolValue struct {
	symbol typedmemory.SchemaSymbolRef
}

func NewSymbolValue(symbol typedmemory.SchemaSymbolRef) (SymbolValue, error) {
	if !validSchemaSymbol(symbol) {
		return SymbolValue{}, fmt.Errorf("linked declaration symbol is required")
	}
	return SymbolValue{symbol: symbol}, nil
}

func (value SymbolValue) Kind() DeclarationValueKind { return DeclarationSymbol }

func (value SymbolValue) Symbol() typedmemory.SchemaSymbolRef { return value.symbol }

func (SymbolValue) declarationValueVariant() {}

type SequenceValue struct {
	values []DeclarationValue
}

func NewSequenceValue(values []DeclarationValue) (SequenceValue, error) {
	if len(values) > maximumValuesPerCollection {
		return SequenceValue{}, fmt.Errorf("declaration sequence exceeds %d values", maximumValuesPerCollection)
	}
	budget := &artifactBudget{}
	if err := budget.consumeValue(1); err != nil {
		return SequenceValue{}, err
	}
	for _, value := range values {
		if err := validateDeclarationValueBudget(value, 2, budget); err != nil {
			return SequenceValue{}, err
		}
	}
	owned, err := cloneDeclarationValues(values)
	if err != nil {
		return SequenceValue{}, err
	}
	return SequenceValue{values: owned}, nil
}

func (value SequenceValue) Kind() DeclarationValueKind { return DeclarationSequence }

func (value SequenceValue) Values() []DeclarationValue {
	owned, _ := cloneDeclarationValues(value.values)
	return owned
}

func (SequenceValue) declarationValueVariant() {}

type SetValue struct {
	values []DeclarationValue
}

func NewSetValue(values []DeclarationValue) (SetValue, error) {
	if len(values) > maximumValuesPerCollection {
		return SetValue{}, fmt.Errorf("declaration set exceeds %d values", maximumValuesPerCollection)
	}
	budget := &artifactBudget{}
	if err := budget.consumeValue(1); err != nil {
		return SetValue{}, err
	}
	for _, value := range values {
		if err := validateDeclarationValueBudget(value, 2, budget); err != nil {
			return SetValue{}, err
		}
	}
	owned, err := cloneDeclarationValues(values)
	if err != nil {
		return SetValue{}, err
	}
	sort.Slice(owned, func(left, right int) bool {
		leftBytes := canonicalDeclarationValue(owned[left])
		rightBytes := canonicalDeclarationValue(owned[right])
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	for index := 1; index < len(owned); index++ {
		previous := canonicalDeclarationValue(owned[index-1])
		current := canonicalDeclarationValue(owned[index])
		if bytes.Equal(previous, current) {
			return SetValue{}, fmt.Errorf("declaration set contains a duplicate value")
		}
	}
	return SetValue{values: owned}, nil
}

func (value SetValue) Kind() DeclarationValueKind { return DeclarationSet }

func (value SetValue) Values() []DeclarationValue {
	owned, _ := cloneDeclarationValues(value.values)
	return owned
}

func (SetValue) declarationValueVariant() {}

type DeclarationField struct {
	name  string
	value DeclarationValue
}

func NewDeclarationField(name string, value DeclarationValue) (DeclarationField, error) {
	parsedName, err := parseFieldName(name)
	if err != nil {
		return DeclarationField{}, err
	}
	budget := &artifactBudget{}
	if err := validateDeclarationValueBudget(value, 1, budget); err != nil {
		return DeclarationField{}, err
	}
	owned, err := cloneDeclarationValue(value)
	if err != nil {
		return DeclarationField{}, err
	}
	return DeclarationField{name: parsedName, value: owned}, nil
}

func (field DeclarationField) Name() string { return field.name }

func (field DeclarationField) Value() DeclarationValue {
	owned, _ := cloneDeclarationValue(field.value)
	return owned
}

type RecordValue struct {
	fields []DeclarationField
}

func NewRecordValue(fields []DeclarationField) (RecordValue, error) {
	budget := &artifactBudget{}
	if err := budget.consumeValue(1); err != nil {
		return RecordValue{}, err
	}
	if err := validateFieldsBudget(fields, 1, budget); err != nil {
		return RecordValue{}, err
	}
	owned, err := normalizeFields(fields, false)
	if err != nil {
		return RecordValue{}, err
	}
	return RecordValue{fields: owned}, nil
}

func (value RecordValue) Kind() DeclarationValueKind { return DeclarationRecord }

func (value RecordValue) Fields() []DeclarationField {
	owned, _ := normalizeFields(value.fields, false)
	return owned
}

func (RecordValue) declarationValueVariant() {}

type DeclarationBody struct {
	fields []DeclarationField
}

func NewDeclarationBody(fields []DeclarationField) (DeclarationBody, error) {
	budget := &artifactBudget{}
	if err := validateFieldsBudget(fields, 0, budget); err != nil {
		return DeclarationBody{}, err
	}
	owned, err := normalizeFields(fields, true)
	if err != nil {
		return DeclarationBody{}, err
	}
	return DeclarationBody{fields: owned}, nil
}

func (body DeclarationBody) Fields() []DeclarationField {
	owned, _ := normalizeFields(body.fields, true)
	return owned
}

func (body DeclarationBody) valid() bool {
	_, err := normalizeFields(body.fields, true)
	return err == nil
}

type DeclarationBasisKind uint8

const (
	SourceAuthoredBasis DeclarationBasisKind = iota + 1
	CompilerDerivedBasis
)

// DeclarationBasis distinguishes source-authored semantics from compiler-owned
// representation lowering. Both variants retain exact source inputs.
type DeclarationBasis interface {
	Kind() DeclarationBasisKind
	RuleID() typedmemory.CompilerRuleID
	SourceLocations() []typedmemory.SourceLocation
	declarationBasisVariant()
}

type SourceBasis struct {
	provenance typedmemory.FPFSourceProvenance
}

func NewSourceDeclarationBasis(
	provenance typedmemory.FPFSourceProvenance,
) (SourceBasis, error) {
	if !validFPFSourceProvenance(provenance) {
		return SourceBasis{}, fmt.Errorf("exact FPF source provenance is required")
	}
	return SourceBasis{provenance: provenance}, nil
}

func (SourceBasis) Kind() DeclarationBasisKind { return SourceAuthoredBasis }

func (basis SourceBasis) RuleID() typedmemory.CompilerRuleID {
	return basis.provenance.CompilerRuleID()
}

func (basis SourceBasis) SourceLocations() []typedmemory.SourceLocation {
	return []typedmemory.SourceLocation{basis.provenance.Location()}
}

func (basis SourceBasis) Provenance() typedmemory.FPFSourceProvenance {
	return basis.provenance
}

func (SourceBasis) declarationBasisVariant() {}

type DerivedBasis struct {
	ruleID typedmemory.CompilerRuleID
	inputs []typedmemory.SourceLocation
}

func NewCompilerDerivedDeclarationBasis(
	ruleID typedmemory.CompilerRuleID,
	inputs []typedmemory.SourceLocation,
) (DerivedBasis, error) {
	if ruleID.String() == "" {
		return DerivedBasis{}, fmt.Errorf("compiler-derived declaration rule is required")
	}
	if len(inputs) > maximumSourcesPerBasis {
		return DerivedBasis{}, fmt.Errorf("compiler-derived declaration exceeds %d source inputs", maximumSourcesPerBasis)
	}
	normalized, err := normalizeSourceLocations(inputs)
	if err != nil {
		return DerivedBasis{}, err
	}
	if len(normalized) == 0 {
		return DerivedBasis{}, fmt.Errorf("compiler-derived declaration requires exact source inputs")
	}
	return DerivedBasis{ruleID: ruleID, inputs: normalized}, nil
}

func (DerivedBasis) Kind() DeclarationBasisKind { return CompilerDerivedBasis }

func (basis DerivedBasis) RuleID() typedmemory.CompilerRuleID { return basis.ruleID }

func (basis DerivedBasis) SourceLocations() []typedmemory.SourceLocation {
	return append([]typedmemory.SourceLocation(nil), basis.inputs...)
}

func (DerivedBasis) declarationBasisVariant() {}

type LinkedDeclaration struct {
	symbol typedmemory.SchemaSymbolRef
	ruleID typedmemory.CompilerRuleID
	body   DeclarationBody
	basis  DeclarationBasis
}

func NewLinkedDeclaration(
	symbol typedmemory.SchemaSymbolRef,
	ruleID typedmemory.CompilerRuleID,
	body DeclarationBody,
	basis DeclarationBasis,
) (LinkedDeclaration, error) {
	if !validSchemaSymbol(symbol) {
		return LinkedDeclaration{}, fmt.Errorf("linked declaration symbol is required")
	}
	if ruleID.String() == "" {
		return LinkedDeclaration{}, fmt.Errorf("linked declaration compiler rule is required")
	}
	if !body.valid() {
		return LinkedDeclaration{}, fmt.Errorf("linked declaration body is required")
	}
	if !validDeclarationBasis(basis) {
		return LinkedDeclaration{}, fmt.Errorf("linked declaration basis is required")
	}
	if basis.RuleID().String() != ruleID.String() {
		return LinkedDeclaration{}, fmt.Errorf("linked declaration rule does not match its basis")
	}
	ownedBasis, err := cloneDeclarationBasis(basis)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	ownedBody, err := NewDeclarationBody(body.fields)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return LinkedDeclaration{
		symbol: symbol,
		ruleID: ruleID,
		body:   ownedBody,
		basis:  ownedBasis,
	}, nil
}

func (declaration LinkedDeclaration) Symbol() typedmemory.SchemaSymbolRef {
	return declaration.symbol
}

func (declaration LinkedDeclaration) RuleID() typedmemory.CompilerRuleID {
	return declaration.ruleID
}

func (declaration LinkedDeclaration) Body() DeclarationBody {
	body, _ := NewDeclarationBody(declaration.body.fields)
	return body
}

func (declaration LinkedDeclaration) Basis() DeclarationBasis {
	basis, _ := cloneDeclarationBasis(declaration.basis)
	return basis
}

func (declaration LinkedDeclaration) Dependencies() []typedmemory.SchemaSymbolRef {
	return declarationDependencies(declaration.body)
}

func (declaration LinkedDeclaration) Digest() typedmemory.SHA256Digest {
	canonical := canonicalLinkedDeclaration(declaration)
	digest, _ := typedmemory.NewSHA256Digest(digestCanonicalBytes(canonical))
	return digest
}

func (declaration LinkedDeclaration) valid() bool {
	if !validSchemaSymbol(declaration.symbol) {
		return false
	}
	if declaration.ruleID.String() == "" || !declaration.body.valid() {
		return false
	}
	if !validDeclarationBasis(declaration.basis) {
		return false
	}
	return declaration.ruleID.String() == declaration.basis.RuleID().String()
}

// LinkedTypeEnvIR is complete enough to seal but contains no TypeEnvRef. A
// compiled-environment artifact may preserve source-only declaration IR; its
// per-symbol coverage posture, not mere declaration presence, controls runtime
// lowering. A coverage-only artifact contains no declarations by construction.
type LinkedTypeEnvIR struct {
	posture      ArtifactPosture
	revision     typedmemory.SourceRevision
	compiler     typedmemory.CompilerSchemaVersion
	coverage     typedmemory.CoverageManifest
	declarations []LinkedDeclaration
	reason       string
}

func NewCompiledLinkedTypeEnvIR(
	revision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
	coverage typedmemory.CoverageManifest,
	declarations []LinkedDeclaration,
) (LinkedTypeEnvIR, error) {
	ir := LinkedTypeEnvIR{
		posture:      CompiledEnvironment,
		revision:     revision,
		compiler:     compiler,
		coverage:     coverage,
		declarations: declarations,
	}
	return normalizeLinkedTypeEnvIR(ir)
}

func NewCoverageOnlyLinkedTypeEnvIR(
	revision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
	coverage typedmemory.CoverageManifest,
	reason string,
) (LinkedTypeEnvIR, error) {
	parsedReason, err := parseReason(reason)
	if err != nil {
		return LinkedTypeEnvIR{}, err
	}
	ir := LinkedTypeEnvIR{
		posture:  CoverageOnly,
		revision: revision,
		compiler: compiler,
		coverage: coverage,
		reason:   parsedReason,
	}
	return normalizeLinkedTypeEnvIR(ir)
}

func (ir LinkedTypeEnvIR) Posture() ArtifactPosture { return ir.posture }

func (ir LinkedTypeEnvIR) SourceRevision() typedmemory.SourceRevision { return ir.revision }

func (ir LinkedTypeEnvIR) CompilerSchemaVersion() typedmemory.CompilerSchemaVersion {
	return ir.compiler
}

func (ir LinkedTypeEnvIR) CoverageManifest() typedmemory.CoverageManifest {
	manifest, _ := typedmemory.NewCoverageManifest(ir.coverage.Entries())
	return manifest
}

func (ir LinkedTypeEnvIR) Declarations() []LinkedDeclaration {
	return cloneLinkedDeclarations(ir.declarations)
}

func (ir LinkedTypeEnvIR) CoverageOnlyReason() (string, bool) {
	return ir.reason, ir.posture == CoverageOnly
}

type DeclarationProjection struct {
	symbol       typedmemory.SchemaSymbolRef
	digest       typedmemory.SHA256Digest
	ruleID       typedmemory.CompilerRuleID
	basisKind    DeclarationBasisKind
	dependencies []typedmemory.SchemaSymbolRef
	sources      []typedmemory.SourceLocation
}

func (projection DeclarationProjection) Symbol() typedmemory.SchemaSymbolRef {
	return projection.symbol
}

func (projection DeclarationProjection) Digest() typedmemory.SHA256Digest {
	return projection.digest
}

func (projection DeclarationProjection) RuleID() typedmemory.CompilerRuleID {
	return projection.ruleID
}

func (projection DeclarationProjection) BasisKind() DeclarationBasisKind {
	return projection.basisKind
}

func (projection DeclarationProjection) Dependencies() []typedmemory.SchemaSymbolRef {
	return append([]typedmemory.SchemaSymbolRef(nil), projection.dependencies...)
}

func (projection DeclarationProjection) SourceInputs() []typedmemory.SourceLocation {
	return append([]typedmemory.SourceLocation(nil), projection.sources...)
}

type SymbolManifest struct {
	entries []DeclarationProjection
}

func (manifest SymbolManifest) Entries() []DeclarationProjection {
	return cloneDeclarationProjections(manifest.entries)
}

func (manifest SymbolManifest) Entry(
	symbol typedmemory.SchemaSymbolRef,
) (DeclarationProjection, bool) {
	key := symbol.String()
	index := sort.Search(len(manifest.entries), func(index int) bool {
		return manifest.entries[index].symbol.String() >= key
	})
	if index >= len(manifest.entries) {
		return DeclarationProjection{}, false
	}
	entry := manifest.entries[index]
	if entry.symbol.String() != key {
		return DeclarationProjection{}, false
	}
	return cloneDeclarationProjection(entry), true
}

// BaseTypeEnvArtifact is authoritative only through its canonical payload.
// SQL projections are convenience indexes and must be re-derived from this
// value rather than reconstructed as authority.
type BaseTypeEnvArtifact struct {
	ir        LinkedTypeEnvIR
	manifest  SymbolManifest
	canonical []byte
	digest    typedmemory.SHA256Digest
	ref       typedmemory.TypeEnvRef
	hasRef    bool
}

func SealBaseTypeEnv(ir LinkedTypeEnvIR) (BaseTypeEnvArtifact, error) {
	normalized, err := normalizeLinkedTypeEnvIR(ir)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	manifest := deriveSymbolManifest(normalized.declarations)
	canonical := canonicalArtifactPayload(normalized, manifest)
	if len(canonical) > maximumArtifactPayloadBytes {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact payload exceeds %d bytes", maximumArtifactPayloadBytes)
	}
	digestText := digestCanonicalBytes(canonical)
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return BaseTypeEnvArtifact{}, fmt.Errorf("derive artifact digest: %w", err)
	}
	artifact := BaseTypeEnvArtifact{
		ir:        normalized,
		manifest:  manifest,
		canonical: canonical,
		digest:    digest,
	}
	if normalized.posture == CompiledEnvironment {
		ref, refErr := typedmemory.NewTypeEnvRef(digest)
		if refErr != nil {
			return BaseTypeEnvArtifact{}, fmt.Errorf("derive TypeEnvRef: %w", refErr)
		}
		artifact.ref = ref
		artifact.hasRef = true
	}
	if err := artifact.Verify(); err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	if normalized.posture == CompiledEnvironment {
		if _, err := LowerBaseTypeEnvArtifact(artifact); err != nil {
			return BaseTypeEnvArtifact{}, fmt.Errorf(
				"compiled artifact cannot materialize its derived TypeEnvRef: %w",
				err,
			)
		}
	}
	return artifact, nil
}

func (artifact BaseTypeEnvArtifact) Posture() ArtifactPosture { return artifact.ir.posture }

func (artifact BaseTypeEnvArtifact) Digest() typedmemory.SHA256Digest { return artifact.digest }

func (artifact BaseTypeEnvArtifact) TypeEnvRef() (typedmemory.TypeEnvRef, bool) {
	return artifact.ref, artifact.hasRef
}

func (artifact BaseTypeEnvArtifact) SourceRevision() typedmemory.SourceRevision {
	return artifact.ir.revision
}

func (artifact BaseTypeEnvArtifact) CompilerSchemaVersion() typedmemory.CompilerSchemaVersion {
	return artifact.ir.compiler
}

func (artifact BaseTypeEnvArtifact) CoverageManifest() typedmemory.CoverageManifest {
	return artifact.ir.CoverageManifest()
}

func (artifact BaseTypeEnvArtifact) Declarations() []LinkedDeclaration {
	return artifact.ir.Declarations()
}

func (artifact BaseTypeEnvArtifact) SymbolManifest() SymbolManifest {
	return SymbolManifest{entries: cloneDeclarationProjections(artifact.manifest.entries)}
}

func (artifact BaseTypeEnvArtifact) DeclarationProjections() []DeclarationProjection {
	return artifact.manifest.Entries()
}

func (artifact BaseTypeEnvArtifact) CoverageOnlyReason() (string, bool) {
	return artifact.ir.CoverageOnlyReason()
}

func (artifact BaseTypeEnvArtifact) CanonicalBytes() []byte {
	return append([]byte(nil), artifact.canonical...)
}

func (artifact BaseTypeEnvArtifact) RecomputeIdentityRef() (
	typedmemory.TypeEnvRef,
	bool,
	error,
) {
	recomputed, err := DecodeBaseTypeEnvArtifact(artifact.canonical)
	if err != nil {
		return typedmemory.TypeEnvRef{}, false, err
	}
	ref, exists := recomputed.TypeEnvRef()
	return ref, exists, nil
}

func (artifact BaseTypeEnvArtifact) Verify() error {
	normalized, err := normalizeLinkedTypeEnvIR(artifact.ir)
	if err != nil {
		return fmt.Errorf("artifact IR: %w", err)
	}
	expectedManifest := deriveSymbolManifest(normalized.declarations)
	if !equalSymbolManifests(expectedManifest, artifact.manifest) {
		return fmt.Errorf("artifact symbol manifest does not match declarations")
	}
	expectedCanonical := canonicalArtifactPayload(normalized, expectedManifest)
	if len(expectedCanonical) > maximumArtifactPayloadBytes {
		return fmt.Errorf("artifact payload exceeds %d bytes", maximumArtifactPayloadBytes)
	}
	if !equalCanonical(expectedCanonical, artifact.canonical) {
		return fmt.Errorf("artifact canonical bytes do not match its IR")
	}
	expectedDigest := digestCanonicalBytes(expectedCanonical)
	if artifact.digest.String() != expectedDigest {
		return fmt.Errorf("artifact digest does not match canonical bytes")
	}
	if normalized.posture == CoverageOnly {
		if artifact.hasRef {
			return fmt.Errorf("coverage-only artifact must not carry a TypeEnvRef")
		}
		return nil
	}
	if !artifact.hasRef {
		return fmt.Errorf("compiled artifact requires its derived TypeEnvRef")
	}
	if artifact.ref.Digest().String() != expectedDigest {
		return fmt.Errorf("artifact TypeEnvRef does not match canonical bytes")
	}
	return nil
}

func (artifact BaseTypeEnvArtifact) MarshalBinary() ([]byte, error) {
	if err := artifact.Verify(); err != nil {
		return nil, err
	}
	writer := newCanonicalWriter(artifactWireDomain)
	writer.addBytes(artifact.canonical)
	writer.addString(artifact.digest.String())
	writer.addBool(artifact.hasRef)
	if artifact.hasRef {
		writer.addString(artifact.ref.Digest().String())
	}
	encoded := writer.bytes()
	if len(encoded) > maximumCanonicalBytes {
		return nil, fmt.Errorf("artifact wire encoding exceeds %d bytes", maximumCanonicalBytes)
	}
	return encoded, nil
}

// DecodeBaseTypeEnvArtifact decodes authoritative self-reference-free payload
// bytes and derives identity. It never accepts a caller-supplied TypeEnvRef.
func DecodeBaseTypeEnvArtifact(canonical []byte) (BaseTypeEnvArtifact, error) {
	if len(canonical) > maximumArtifactPayloadBytes {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact payload exceeds %d bytes", maximumArtifactPayloadBytes)
	}
	owned := append([]byte(nil), canonical...)
	ir, encodedManifest, err := decodeArtifactPayload(owned)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	if !equalSymbolManifests(encodedManifest, artifact.manifest) {
		return BaseTypeEnvArtifact{}, fmt.Errorf("encoded symbol manifest does not match declarations")
	}
	if !equalCanonical(owned, artifact.canonical) {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact payload is not in canonical form")
	}
	return artifact, nil
}

func UnmarshalBaseTypeEnvArtifact(encoded []byte) (BaseTypeEnvArtifact, error) {
	if len(encoded) > maximumCanonicalBytes {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact wire encoding exceeds %d bytes", maximumCanonicalBytes)
	}
	owned := append([]byte(nil), encoded...)
	reader, err := newCanonicalReader(owned, artifactWireDomain)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	canonical, err := reader.readBytes()
	if err != nil {
		return BaseTypeEnvArtifact{}, fmt.Errorf("decode artifact payload: %w", err)
	}
	claimedDigest, err := reader.readString()
	if err != nil {
		return BaseTypeEnvArtifact{}, fmt.Errorf("decode artifact digest: %w", err)
	}
	hasRef, err := reader.readBool()
	if err != nil {
		return BaseTypeEnvArtifact{}, fmt.Errorf("decode artifact ref posture: %w", err)
	}
	claimedRefDigest := ""
	if hasRef {
		claimedRefDigest, err = reader.readString()
		if err != nil {
			return BaseTypeEnvArtifact{}, fmt.Errorf("decode TypeEnvRef digest: %w", err)
		}
	}
	if err := reader.requireDone(); err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	artifact, err := DecodeBaseTypeEnvArtifact(canonical)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	if artifact.digest.String() != claimedDigest {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact wire digest does not match canonical payload")
	}
	if artifact.hasRef != hasRef {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact wire TypeEnvRef posture is inconsistent")
	}
	if hasRef && artifact.ref.Digest().String() != claimedRefDigest {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact wire TypeEnvRef was tampered")
	}
	reencoded, err := artifact.MarshalBinary()
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	if !equalCanonical(owned, reencoded) {
		return BaseTypeEnvArtifact{}, fmt.Errorf("artifact wire encoding is not canonical")
	}
	return artifact, nil
}

type CompatibilityAssessmentKind uint8

const (
	InitialCompilation CompatibilityAssessmentKind = iota + 1
	ComparedCompilation
)

type CompatibilityAssessment interface {
	Kind() CompatibilityAssessmentKind
	compatibilityAssessmentVariant()
}

type InitialCompatibilityAssessment struct{}

func NewInitialCompatibilityAssessment() InitialCompatibilityAssessment {
	return InitialCompatibilityAssessment{}
}

func (InitialCompatibilityAssessment) Kind() CompatibilityAssessmentKind {
	return InitialCompilation
}

func (InitialCompatibilityAssessment) compatibilityAssessmentVariant() {}

type ComparedCompatibilityAssessment struct {
	diff typedmemory.TypeEnvCompatibilityDiff
}

func NewComparedCompatibilityAssessment(
	diff typedmemory.TypeEnvCompatibilityDiff,
) (ComparedCompatibilityAssessment, error) {
	if diff.Base().String() == "" {
		return ComparedCompatibilityAssessment{}, fmt.Errorf("compatibility base TypeEnv is required")
	}
	return ComparedCompatibilityAssessment{diff: diff}, nil
}

func (ComparedCompatibilityAssessment) Kind() CompatibilityAssessmentKind {
	return ComparedCompilation
}

func (assessment ComparedCompatibilityAssessment) Diff() typedmemory.TypeEnvCompatibilityDiff {
	return assessment.diff
}

func (ComparedCompatibilityAssessment) compatibilityAssessmentVariant() {}

// CompilationEnvelope holds run-relative compatibility outside authoritative
// artifact bytes. Changing the comparison base or rationale cannot change the
// content-addressed TypeEnv identity.
type CompilationEnvelope struct {
	artifact      BaseTypeEnvArtifact
	compatibility CompatibilityAssessment
}

func NewCompilationEnvelope(
	artifact BaseTypeEnvArtifact,
	compatibility CompatibilityAssessment,
) (CompilationEnvelope, error) {
	if err := artifact.Verify(); err != nil {
		return CompilationEnvelope{}, err
	}
	if !validCompatibilityAssessment(compatibility) {
		return CompilationEnvelope{}, fmt.Errorf("explicit compatibility assessment is required")
	}
	return CompilationEnvelope{
		artifact:      artifact,
		compatibility: compatibility,
	}, nil
}

func (envelope CompilationEnvelope) Artifact() BaseTypeEnvArtifact {
	return envelope.artifact
}

func (envelope CompilationEnvelope) Compatibility() CompatibilityAssessment {
	return envelope.compatibility
}

func validCompatibilityAssessment(assessment CompatibilityAssessment) bool {
	switch value := assessment.(type) {
	case InitialCompatibilityAssessment:
		return true
	case ComparedCompatibilityAssessment:
		return value.diff.Base().String() != ""
	default:
		return false
	}
}

func parseFieldName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("declaration field name is required")
	}
	if strings.ContainsAny(value, "\r\n\t/\\") {
		return "", fmt.Errorf("declaration field name must be one line without slash or backslash")
	}
	return value, nil
}

func parseReason(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("coverage-only reason is required")
	}
	if strings.ContainsAny(value, "\r\n\t") {
		return "", fmt.Errorf("coverage-only reason must be one line")
	}
	return value, nil
}

func validateDeclarationValueBudget(
	value DeclarationValue,
	depth int,
	budget *artifactBudget,
) error {
	if budget == nil {
		return fmt.Errorf("artifact budget is required")
	}
	if err := budget.consumeValue(depth); err != nil {
		return err
	}
	switch typed := value.(type) {
	case TextValue:
		return budget.consumeScalar("declaration text", len(typed.value), maximumScalarStringBytes)
	case UnsignedValue:
		return nil
	case BooleanValue:
		return nil
	case BytesValue:
		return budget.consumeScalar("declaration bytes", len(typed.value), maximumScalarByteValueBytes)
	case SymbolValue:
		if !validSchemaSymbol(typed.symbol) {
			return fmt.Errorf("linked declaration symbol is required")
		}
		return budget.consumeScalar("schema symbol", len(typed.symbol.String()), maximumScalarStringBytes)
	case SequenceValue:
		return validateCollectionBudget(typed.values, depth, budget, "declaration sequence")
	case SetValue:
		return validateCollectionBudget(typed.values, depth, budget, "declaration set")
	case RecordValue:
		return validateFieldsBudget(typed.fields, depth, budget)
	default:
		return fmt.Errorf("value is outside the closed declaration algebra")
	}
}

func validateCollectionBudget(
	values []DeclarationValue,
	depth int,
	budget *artifactBudget,
	label string,
) error {
	if len(values) > maximumValuesPerCollection {
		return fmt.Errorf("%s exceeds %d values", label, maximumValuesPerCollection)
	}
	for _, value := range values {
		if err := validateDeclarationValueBudget(value, depth+1, budget); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldsBudget(
	fields []DeclarationField,
	containerDepth int,
	budget *artifactBudget,
) error {
	if err := budget.consumeFields(len(fields)); err != nil {
		return err
	}
	for _, field := range fields {
		if err := budget.consumeScalar("declaration field name", len(field.name), maximumScalarStringBytes); err != nil {
			return err
		}
		if err := validateDeclarationValueBudget(field.value, containerDepth+1, budget); err != nil {
			return err
		}
	}
	return nil
}

func validateIRResourceBudget(ir LinkedTypeEnvIR) error {
	if len(ir.declarations) > maximumDeclarations {
		return fmt.Errorf("artifact exceeds %d declarations", maximumDeclarations)
	}
	budget := &artifactBudget{}
	metadata := []struct {
		label string
		value string
	}{
		{label: "source revision", value: ir.revision.String()},
		{label: "compiler schema version", value: ir.compiler.String()},
		{label: "coverage-only reason", value: ir.reason},
	}
	for _, item := range metadata {
		if err := budget.consumeScalar(item.label, len(item.value), maximumScalarStringBytes); err != nil {
			return err
		}
	}
	coverageEntries := ir.coverage.Entries()
	if len(coverageEntries) > maximumCoverageEntries {
		return fmt.Errorf("coverage manifest exceeds %d entries", maximumCoverageEntries)
	}
	for _, entry := range coverageEntries {
		if err := consumeCoverageBudget(entry, budget); err != nil {
			return err
		}
	}
	for _, declaration := range ir.declarations {
		if err := consumeDeclarationBudget(declaration, budget); err != nil {
			return err
		}
	}
	return nil
}

func consumeCoverageBudget(
	entry typedmemory.CoverageEntry,
	budget *artifactBudget,
) error {
	values := []struct {
		label string
		value string
	}{
		{label: "coverage subject", value: entry.Subject().String()},
		{label: "coverage posture", value: entry.Posture().String()},
		{label: "coverage rationale", value: entry.Rationale()},
	}
	for _, item := range values {
		if err := budget.consumeScalar(item.label, len(item.value), maximumScalarStringBytes); err != nil {
			return err
		}
	}
	return consumeSourceLocationBudget(entry.Source(), budget)
}

func consumeDeclarationBudget(
	declaration LinkedDeclaration,
	budget *artifactBudget,
) error {
	values := []struct {
		label string
		value string
	}{
		{label: "declaration symbol", value: declaration.symbol.String()},
		{label: "declaration compiler rule", value: declaration.ruleID.String()},
	}
	for _, item := range values {
		if err := budget.consumeScalar(item.label, len(item.value), maximumScalarStringBytes); err != nil {
			return err
		}
	}
	if err := validateFieldsBudget(declaration.body.fields, 0, budget); err != nil {
		return err
	}
	dependencies := declarationDependencies(declaration.body)
	if len(dependencies) > maximumDependenciesPerDeclaration {
		return fmt.Errorf("declaration %q exceeds %d dependencies", declaration.symbol.String(), maximumDependenciesPerDeclaration)
	}
	if !validDeclarationBasis(declaration.basis) {
		return fmt.Errorf("declaration %q basis is invalid", declaration.symbol.String())
	}
	if source, ok := declaration.basis.(SourceBasis); ok {
		value := source.provenance.Reference().String()
		if err := budget.consumeScalar("provenance reference", len(value), maximumScalarStringBytes); err != nil {
			return err
		}
	}
	locations := declaration.basis.SourceLocations()
	if len(locations) > maximumSourcesPerBasis {
		return fmt.Errorf("declaration %q exceeds %d source inputs", declaration.symbol.String(), maximumSourcesPerBasis)
	}
	for _, location := range locations {
		if err := consumeSourceLocationBudget(location, budget); err != nil {
			return err
		}
	}
	return nil
}

func consumeSourceLocationBudget(
	location typedmemory.SourceLocation,
	budget *artifactBudget,
) error {
	values := []struct {
		label string
		value string
	}{
		{label: "source unit ID", value: location.UnitID().String()},
		{label: "source revision", value: location.Revision().String()},
		{label: "source content hash", value: location.ContentHash().String()},
	}
	patternID, hasPattern := location.PatternID()
	if hasPattern {
		values = append(values, struct {
			label string
			value string
		}{label: "source PatternID", value: patternID.String()})
	}
	for _, item := range values {
		if err := budget.consumeScalar(item.label, len(item.value), maximumScalarStringBytes); err != nil {
			return err
		}
	}
	return nil
}

func validSchemaSymbol(symbol typedmemory.SchemaSymbolRef) bool {
	return symbol.Kind().String() != "" && symbol.Key() != "" && symbol.String() != ""
}

func cloneDeclarationValues(values []DeclarationValue) ([]DeclarationValue, error) {
	owned := make([]DeclarationValue, 0, len(values))
	for index, value := range values {
		cloned, err := cloneDeclarationValue(value)
		if err != nil {
			return nil, fmt.Errorf("declaration value %d: %w", index, err)
		}
		owned = append(owned, cloned)
	}
	return owned, nil
}

func cloneDeclarationValue(value DeclarationValue) (DeclarationValue, error) {
	switch typed := value.(type) {
	case TextValue:
		return NewTextValue(typed.value), nil
	case UnsignedValue:
		return NewUnsignedValue(typed.value), nil
	case BooleanValue:
		return NewBooleanValue(typed.value), nil
	case BytesValue:
		return NewBytesValue(typed.value), nil
	case SymbolValue:
		return NewSymbolValue(typed.symbol)
	case SequenceValue:
		return NewSequenceValue(typed.values)
	case SetValue:
		return NewSetValue(typed.values)
	case RecordValue:
		return NewRecordValue(typed.fields)
	default:
		return nil, fmt.Errorf("value is outside the closed declaration algebra")
	}
}

func normalizeFields(fields []DeclarationField, requireNonEmpty bool) ([]DeclarationField, error) {
	if requireNonEmpty && len(fields) == 0 {
		return nil, fmt.Errorf("declaration body requires at least one field")
	}
	if len(fields) > maximumFieldsPerRecord {
		return nil, fmt.Errorf("declaration record exceeds %d fields", maximumFieldsPerRecord)
	}
	owned := make([]DeclarationField, 0, len(fields))
	for index, field := range fields {
		name, err := parseFieldName(field.name)
		if err != nil {
			return nil, fmt.Errorf("declaration field %d: %w", index, err)
		}
		value, err := cloneDeclarationValue(field.value)
		if err != nil {
			return nil, fmt.Errorf("declaration field %q: %w", name, err)
		}
		owned = append(owned, DeclarationField{name: name, value: value})
	}
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].name < owned[right].name
	})
	for index := 1; index < len(owned); index++ {
		if owned[index-1].name == owned[index].name {
			return nil, fmt.Errorf("duplicate declaration field %q", owned[index].name)
		}
	}
	return owned, nil
}

func canonicalDeclarationValue(value DeclarationValue) []byte {
	writer := newCanonicalWriter("declaration-value.v1")
	switch typed := value.(type) {
	case TextValue:
		writer.addByte(byte(DeclarationText))
		writer.addString(typed.value)
	case UnsignedValue:
		writer.addByte(byte(DeclarationUnsigned))
		writer.addUint64(typed.value)
	case BooleanValue:
		writer.addByte(byte(DeclarationBoolean))
		writer.addBool(typed.value)
	case BytesValue:
		writer.addByte(byte(DeclarationBytes))
		writer.addBytes(typed.value)
	case SymbolValue:
		writer.addByte(byte(DeclarationSymbol))
		writer.addBytes(canonicalSchemaSymbol(typed.symbol))
	case SequenceValue:
		writer.addByte(byte(DeclarationSequence))
		writer.addUint64(uint64(len(typed.values)))
		for _, member := range typed.values {
			writer.addBytes(canonicalDeclarationValue(member))
		}
	case SetValue:
		writer.addByte(byte(DeclarationSet))
		writer.addUint64(uint64(len(typed.values)))
		for _, member := range typed.values {
			writer.addBytes(canonicalDeclarationValue(member))
		}
	case RecordValue:
		writer.addByte(byte(DeclarationRecord))
		writer.addBytes(canonicalFields(typed.fields, "declaration-record-fields.v1"))
	}
	return writer.bytes()
}

func canonicalFields(fields []DeclarationField, domain string) []byte {
	writer := newCanonicalWriter(domain)
	writer.addUint64(uint64(len(fields)))
	for _, field := range fields {
		writer.addString(field.name)
		writer.addBytes(canonicalDeclarationValue(field.value))
	}
	return writer.bytes()
}

func declarationDependencies(body DeclarationBody) []typedmemory.SchemaSymbolRef {
	byKey := map[string]typedmemory.SchemaSymbolRef{}
	for _, field := range body.fields {
		collectValueDependencies(field.value, byKey)
	}
	dependencies := make([]typedmemory.SchemaSymbolRef, 0, len(byKey))
	for _, dependency := range byKey {
		dependencies = append(dependencies, dependency)
	}
	sort.Slice(dependencies, func(left, right int) bool {
		return dependencies[left].String() < dependencies[right].String()
	})
	return dependencies
}

func collectValueDependencies(
	value DeclarationValue,
	destination map[string]typedmemory.SchemaSymbolRef,
) {
	switch typed := value.(type) {
	case SymbolValue:
		destination[typed.symbol.String()] = typed.symbol
	case SequenceValue:
		for _, member := range typed.values {
			collectValueDependencies(member, destination)
		}
	case SetValue:
		for _, member := range typed.values {
			collectValueDependencies(member, destination)
		}
	case RecordValue:
		for _, field := range typed.fields {
			collectValueDependencies(field.value, destination)
		}
	}
}

func validFPFSourceProvenance(provenance typedmemory.FPFSourceProvenance) bool {
	return provenance.Reference().String() != "" &&
		validSourceLocation(provenance.Location()) &&
		provenance.CompilerRuleID().String() != ""
}

func validSourceLocation(location typedmemory.SourceLocation) bool {
	lineRange := location.LineRange()
	return location.UnitID().String() != "" &&
		location.Revision().String() != "" &&
		location.ContentHash().String() != "" &&
		lineRange.Start() > 0 &&
		lineRange.End() >= lineRange.Start()
}

func normalizeSourceLocations(
	locations []typedmemory.SourceLocation,
) ([]typedmemory.SourceLocation, error) {
	if len(locations) > maximumSourcesPerBasis {
		return nil, fmt.Errorf("declaration basis exceeds %d source inputs", maximumSourcesPerBasis)
	}
	owned := append([]typedmemory.SourceLocation(nil), locations...)
	sort.Slice(owned, func(left, right int) bool {
		leftBytes := canonicalSourceLocation(owned[left])
		rightBytes := canonicalSourceLocation(owned[right])
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	for index, location := range owned {
		if !validSourceLocation(location) {
			return nil, fmt.Errorf("source location %d is invalid", index)
		}
		if index == 0 {
			continue
		}
		previous := canonicalSourceLocation(owned[index-1])
		current := canonicalSourceLocation(location)
		if bytes.Equal(previous, current) {
			return nil, fmt.Errorf("duplicate source location")
		}
	}
	return owned, nil
}

func validDeclarationBasis(basis DeclarationBasis) bool {
	switch typed := basis.(type) {
	case SourceBasis:
		return validFPFSourceProvenance(typed.provenance)
	case DerivedBasis:
		if typed.ruleID.String() == "" || len(typed.inputs) == 0 || len(typed.inputs) > maximumSourcesPerBasis {
			return false
		}
		_, err := normalizeSourceLocations(typed.inputs)
		return err == nil
	default:
		return false
	}
}

func cloneDeclarationBasis(basis DeclarationBasis) (DeclarationBasis, error) {
	switch typed := basis.(type) {
	case SourceBasis:
		return NewSourceDeclarationBasis(typed.provenance)
	case DerivedBasis:
		return NewCompilerDerivedDeclarationBasis(typed.ruleID, typed.inputs)
	default:
		return nil, fmt.Errorf("basis is outside the closed declaration-basis algebra")
	}
}

func normalizeLinkedTypeEnvIR(ir LinkedTypeEnvIR) (LinkedTypeEnvIR, error) {
	if !ir.posture.valid() {
		return LinkedTypeEnvIR{}, fmt.Errorf("artifact posture is required")
	}
	if ir.revision.String() == "" {
		return LinkedTypeEnvIR{}, fmt.Errorf("source revision is required")
	}
	if ir.compiler.String() == "" {
		return LinkedTypeEnvIR{}, fmt.Errorf("compiler schema version is required")
	}
	if err := validateIRResourceBudget(ir); err != nil {
		return LinkedTypeEnvIR{}, fmt.Errorf("artifact resource budget: %w", err)
	}
	coverage, err := typedmemory.NewCoverageManifest(ir.coverage.Entries())
	if err != nil {
		return LinkedTypeEnvIR{}, fmt.Errorf("coverage manifest: %w", err)
	}
	if err := validateCoverageRevision(coverage, ir.revision); err != nil {
		return LinkedTypeEnvIR{}, err
	}
	if ir.posture == CoverageOnly {
		return normalizeCoverageOnlyIR(ir, coverage)
	}
	return normalizeCompiledIR(ir, coverage)
}

func normalizeCoverageOnlyIR(
	ir LinkedTypeEnvIR,
	coverage typedmemory.CoverageManifest,
) (LinkedTypeEnvIR, error) {
	if len(ir.declarations) != 0 {
		return LinkedTypeEnvIR{}, fmt.Errorf("coverage-only artifact must not contain declarations")
	}
	reason, err := parseReason(ir.reason)
	if err != nil {
		return LinkedTypeEnvIR{}, err
	}
	hasGap := false
	for _, entry := range coverage.Entries() {
		if entry.Posture() != typedmemory.CoverageCompiled {
			hasGap = true
		}
		if entry.Posture() == typedmemory.CoverageCompiled {
			_, isSymbol := entry.Subject().SchemaSymbol()
			if isSymbol {
				return LinkedTypeEnvIR{}, fmt.Errorf("coverage-only artifact cannot claim a compiled schema symbol")
			}
		}
	}
	if !hasGap {
		return LinkedTypeEnvIR{}, fmt.Errorf("coverage-only artifact requires at least one explicit gap")
	}
	return LinkedTypeEnvIR{
		posture:  CoverageOnly,
		revision: ir.revision,
		compiler: ir.compiler,
		coverage: coverage,
		reason:   reason,
	}, nil
}

func normalizeCompiledIR(
	ir LinkedTypeEnvIR,
	coverage typedmemory.CoverageManifest,
) (LinkedTypeEnvIR, error) {
	if ir.reason != "" {
		return LinkedTypeEnvIR{}, fmt.Errorf("compiled artifact must not carry a coverage-only reason")
	}
	if len(ir.declarations) == 0 {
		return LinkedTypeEnvIR{}, fmt.Errorf("compiled artifact requires at least one complete declaration")
	}
	for index, declaration := range ir.declarations {
		if !declaration.valid() {
			return LinkedTypeEnvIR{}, fmt.Errorf("linked declaration %d is invalid", index)
		}
	}
	declarations := cloneLinkedDeclarations(ir.declarations)
	if len(declarations) != len(ir.declarations) {
		return LinkedTypeEnvIR{}, fmt.Errorf("linked declaration cloning failed")
	}
	sort.Slice(declarations, func(left, right int) bool {
		return declarations[left].symbol.String() < declarations[right].symbol.String()
	})
	for index, declaration := range declarations {
		if !declaration.valid() {
			return LinkedTypeEnvIR{}, fmt.Errorf("linked declaration %d is invalid", index)
		}
		if index > 0 && declaration.symbol.String() == declarations[index-1].symbol.String() {
			return LinkedTypeEnvIR{}, fmt.Errorf("duplicate linked declaration %q", declaration.symbol.String())
		}
		for _, source := range declaration.basis.SourceLocations() {
			if source.Revision().String() != ir.revision.String() {
				return LinkedTypeEnvIR{}, fmt.Errorf("declaration %q references another source revision", declaration.symbol.String())
			}
		}
	}
	if err := validateLinkedDependencies(declarations); err != nil {
		return LinkedTypeEnvIR{}, err
	}
	if err := validateCompiledCoverage(declarations, coverage); err != nil {
		return LinkedTypeEnvIR{}, err
	}
	return LinkedTypeEnvIR{
		posture:      CompiledEnvironment,
		revision:     ir.revision,
		compiler:     ir.compiler,
		coverage:     coverage,
		declarations: declarations,
	}, nil
}

func validateCoverageRevision(
	coverage typedmemory.CoverageManifest,
	revision typedmemory.SourceRevision,
) error {
	for _, entry := range coverage.Entries() {
		if entry.Source().Revision().String() != revision.String() {
			return fmt.Errorf("coverage subject %q references another source revision", entry.Subject().String())
		}
	}
	return nil
}

func validateLinkedDependencies(declarations []LinkedDeclaration) error {
	known := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		known[declaration.symbol.String()] = struct{}{}
	}
	for _, declaration := range declarations {
		for _, dependency := range declaration.Dependencies() {
			if _, exists := known[dependency.String()]; !exists {
				return fmt.Errorf("declaration %q has unresolved symbol %q", declaration.symbol.String(), dependency.String())
			}
		}
	}
	return nil
}

func validateCompiledCoverage(
	declarations []LinkedDeclaration,
	coverage typedmemory.CoverageManifest,
) error {
	known := make(map[string]struct{}, len(declarations))
	postures := make(map[string]typedmemory.CoveragePosture, len(declarations))
	for _, declaration := range declarations {
		known[declaration.symbol.String()] = struct{}{}
		subject, err := typedmemory.SchemaSymbolCoverage(declaration.symbol)
		if err != nil {
			return err
		}
		entry, exists := coverage.Entry(subject)
		if !exists {
			return fmt.Errorf("declaration %q requires explicit coverage", declaration.symbol.String())
		}
		if entry.Posture() != typedmemory.CoverageCompiled &&
			entry.Posture() != typedmemory.CoverageSourceOnly {
			return fmt.Errorf(
				"declaration %q coverage posture %q cannot preserve compiler IR",
				declaration.symbol.String(),
				entry.Posture().String(),
			)
		}
		if !sourceLocationIn(entry.Source(), declaration.basis.SourceLocations()) {
			return fmt.Errorf("declaration %q coverage does not match its exact source basis", declaration.symbol.String())
		}
		postures[declaration.symbol.String()] = entry.Posture()
	}
	for _, declaration := range declarations {
		if postures[declaration.symbol.String()] != typedmemory.CoverageCompiled {
			continue
		}
		for _, dependency := range declaration.Dependencies() {
			if postures[dependency.String()] != typedmemory.CoverageCompiled {
				return fmt.Errorf(
					"compiled declaration %q depends on non-executable declaration %q",
					declaration.symbol.String(),
					dependency.String(),
				)
			}
		}
	}
	for _, entry := range coverage.Entries() {
		if entry.Posture() != typedmemory.CoverageCompiled {
			continue
		}
		symbol, isSymbol := entry.Subject().SchemaSymbol()
		if !isSymbol {
			continue
		}
		if _, exists := known[symbol.String()]; !exists {
			return fmt.Errorf("compiled coverage for %q has no declaration", symbol.String())
		}
	}
	return nil
}

func sourceLocationIn(
	candidate typedmemory.SourceLocation,
	locations []typedmemory.SourceLocation,
) bool {
	candidateBytes := canonicalSourceLocation(candidate)
	for _, location := range locations {
		locationBytes := canonicalSourceLocation(location)
		if bytes.Equal(candidateBytes, locationBytes) {
			return true
		}
	}
	return false
}

func linkedDeclarationIsCompiled(
	artifact BaseTypeEnvArtifact,
	declaration LinkedDeclaration,
) bool {
	subject, err := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
	if err != nil {
		return false
	}
	entry, exists := artifact.CoverageManifest().Entry(subject)
	return exists && entry.Posture() == typedmemory.CoverageCompiled
}

func cloneLinkedDeclarations(source []LinkedDeclaration) []LinkedDeclaration {
	owned := make([]LinkedDeclaration, 0, len(source))
	for _, declaration := range source {
		cloned, err := NewLinkedDeclaration(
			declaration.symbol,
			declaration.ruleID,
			declaration.body,
			declaration.basis,
		)
		if err == nil {
			owned = append(owned, cloned)
		}
	}
	return owned
}

func deriveSymbolManifest(declarations []LinkedDeclaration) SymbolManifest {
	entries := make([]DeclarationProjection, 0, len(declarations))
	for _, declaration := range declarations {
		entry := DeclarationProjection{
			symbol:       declaration.symbol,
			digest:       declaration.Digest(),
			ruleID:       declaration.ruleID,
			basisKind:    declaration.basis.Kind(),
			dependencies: declaration.Dependencies(),
			sources:      declaration.basis.SourceLocations(),
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].symbol.String() < entries[right].symbol.String()
	})
	return SymbolManifest{entries: entries}
}

func cloneDeclarationProjection(source DeclarationProjection) DeclarationProjection {
	return DeclarationProjection{
		symbol:       source.symbol,
		digest:       source.digest,
		ruleID:       source.ruleID,
		basisKind:    source.basisKind,
		dependencies: append([]typedmemory.SchemaSymbolRef(nil), source.dependencies...),
		sources:      append([]typedmemory.SourceLocation(nil), source.sources...),
	}
}

func cloneDeclarationProjections(source []DeclarationProjection) []DeclarationProjection {
	owned := make([]DeclarationProjection, 0, len(source))
	for _, entry := range source {
		owned = append(owned, cloneDeclarationProjection(entry))
	}
	return owned
}

func equalSymbolManifests(left, right SymbolManifest) bool {
	leftBytes := canonicalSymbolManifest(left)
	rightBytes := canonicalSymbolManifest(right)
	return bytes.Equal(leftBytes, rightBytes)
}

func canonicalLinkedDeclaration(declaration LinkedDeclaration) []byte {
	writer := newCanonicalWriter("linked-declaration.v1")
	writer.addBytes(canonicalSchemaSymbol(declaration.symbol))
	writer.addString(declaration.ruleID.String())
	writer.addBytes(canonicalFields(declaration.body.fields, "declaration-body-fields.v1"))
	writer.addBytes(canonicalDeclarationBasis(declaration.basis))
	return writer.bytes()
}

func canonicalDeclarationBasis(basis DeclarationBasis) []byte {
	writer := newCanonicalWriter("declaration-basis.v1")
	writer.addByte(byte(basis.Kind()))
	switch typed := basis.(type) {
	case SourceBasis:
		writer.addString(typed.provenance.Reference().String())
		writer.addBytes(canonicalSourceLocation(typed.provenance.Location()))
		writer.addString(typed.provenance.CompilerRuleID().String())
	case DerivedBasis:
		writer.addString(typed.ruleID.String())
		writer.addUint64(uint64(len(typed.inputs)))
		for _, input := range typed.inputs {
			writer.addBytes(canonicalSourceLocation(input))
		}
	}
	return writer.bytes()
}

func canonicalSchemaSymbol(symbol typedmemory.SchemaSymbolRef) []byte {
	writer := newCanonicalWriter("schema-symbol.v1")
	writer.addByte(byte(symbol.Kind()))
	writer.addString(symbol.Key())
	return writer.bytes()
}

func canonicalSourceLocation(location typedmemory.SourceLocation) []byte {
	writer := newCanonicalWriter("source-location.v1")
	writer.addString(location.UnitID().String())
	writer.addString(location.Revision().String())
	writer.addString(location.ContentHash().String())
	writer.addUint64(location.LineRange().Start())
	writer.addUint64(location.LineRange().End())
	patternID, hasPattern := location.PatternID()
	writer.addBool(hasPattern)
	if hasPattern {
		writer.addString(patternID.String())
	}
	return writer.bytes()
}

func canonicalCoverageManifest(manifest typedmemory.CoverageManifest) []byte {
	writer := newCanonicalWriter("coverage-manifest.v1")
	entries := manifest.Entries()
	writer.addUint64(uint64(len(entries)))
	for _, entry := range entries {
		entryWriter := newCanonicalWriter("coverage-entry.v1")
		entryWriter.addString(entry.Subject().String())
		entryWriter.addString(entry.Posture().String())
		entryWriter.addBytes(canonicalSourceLocation(entry.Source()))
		entryWriter.addString(entry.Rationale())
		writer.addBytes(entryWriter.bytes())
	}
	return writer.bytes()
}

func canonicalSymbolManifest(manifest SymbolManifest) []byte {
	writer := newCanonicalWriter("symbol-manifest.v1")
	writer.addUint64(uint64(len(manifest.entries)))
	for _, entry := range manifest.entries {
		entryWriter := newCanonicalWriter("symbol-manifest-entry.v1")
		entryWriter.addBytes(canonicalSchemaSymbol(entry.symbol))
		entryWriter.addString(entry.digest.String())
		entryWriter.addString(entry.ruleID.String())
		entryWriter.addByte(byte(entry.basisKind))
		entryWriter.addUint64(uint64(len(entry.dependencies)))
		for _, dependency := range entry.dependencies {
			entryWriter.addBytes(canonicalSchemaSymbol(dependency))
		}
		entryWriter.addUint64(uint64(len(entry.sources)))
		for _, source := range entry.sources {
			entryWriter.addBytes(canonicalSourceLocation(source))
		}
		writer.addBytes(entryWriter.bytes())
	}
	return writer.bytes()
}

func canonicalArtifactPayload(ir LinkedTypeEnvIR, manifest SymbolManifest) []byte {
	writer := newCanonicalWriter(artifactPayloadDomain)
	writer.addByte(byte(ir.posture))
	writer.addString(ir.revision.String())
	writer.addString(ir.compiler.String())
	writer.addString(ir.reason)
	writer.addBytes(canonicalCoverageManifest(ir.coverage))
	writer.addUint64(uint64(len(ir.declarations)))
	for _, declaration := range ir.declarations {
		writer.addBytes(canonicalLinkedDeclaration(declaration))
	}
	writer.addBytes(canonicalSymbolManifest(manifest))
	return writer.bytes()
}

func decodeArtifactPayload(
	canonical []byte,
) (LinkedTypeEnvIR, SymbolManifest, error) {
	reader, err := newCanonicalReader(canonical, artifactPayloadDomain)
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	postureTag, err := reader.readByte()
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	posture := ArtifactPosture(postureTag)
	if !posture.valid() {
		return LinkedTypeEnvIR{}, SymbolManifest{}, fmt.Errorf("unknown artifact posture %d", postureTag)
	}
	revisionText, err := reader.readString()
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	revision, err := typedmemory.NewSourceRevision(revisionText)
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	compilerText, err := reader.readString()
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion(compilerText)
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	reason, err := reader.readString()
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	coverageBytes, err := reader.readBytes()
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	coverage, err := decodeCoverageManifest(coverageBytes)
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	declarationCount, err := reader.readCountLimit(maximumDeclarations, "artifact declarations")
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	declarations := make([]LinkedDeclaration, 0, declarationCount)
	decodeBudget := &artifactBudget{}
	for index := 0; index < declarationCount; index++ {
		declarationBytes, readErr := reader.readBytes()
		if readErr != nil {
			return LinkedTypeEnvIR{}, SymbolManifest{}, readErr
		}
		declaration, decodeErr := decodeLinkedDeclaration(declarationBytes, decodeBudget)
		if decodeErr != nil {
			return LinkedTypeEnvIR{}, SymbolManifest{}, decodeErr
		}
		declarations = append(declarations, declaration)
	}
	manifestBytes, err := reader.readBytes()
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	manifest, err := decodeSymbolManifest(manifestBytes)
	if err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	if err := reader.requireDone(); err != nil {
		return LinkedTypeEnvIR{}, SymbolManifest{}, err
	}
	if posture == CoverageOnly {
		ir, buildErr := NewCoverageOnlyLinkedTypeEnvIR(revision, compiler, coverage, reason)
		return ir, manifest, buildErr
	}
	if reason != "" {
		return LinkedTypeEnvIR{}, SymbolManifest{}, fmt.Errorf("compiled artifact carries a coverage-only reason")
	}
	ir, err := NewCompiledLinkedTypeEnvIR(revision, compiler, coverage, declarations)
	return ir, manifest, err
}

func decodeLinkedDeclaration(
	data []byte,
	budget *artifactBudget,
) (LinkedDeclaration, error) {
	reader, err := newCanonicalReader(data, "linked-declaration.v1")
	if err != nil {
		return LinkedDeclaration{}, err
	}
	symbolBytes, err := reader.readBytes()
	if err != nil {
		return LinkedDeclaration{}, err
	}
	symbol, err := decodeSchemaSymbol(symbolBytes)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	ruleText, err := reader.readString()
	if err != nil {
		return LinkedDeclaration{}, err
	}
	ruleID, err := typedmemory.NewCompilerRuleID(ruleText)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	bodyBytes, err := reader.readBytes()
	if err != nil {
		return LinkedDeclaration{}, err
	}
	fields, err := decodeFields(bodyBytes, "declaration-body-fields.v1", budget, 0)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	body, err := NewDeclarationBody(fields)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	basisBytes, err := reader.readBytes()
	if err != nil {
		return LinkedDeclaration{}, err
	}
	basis, err := decodeDeclarationBasis(basisBytes)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	if err := reader.requireDone(); err != nil {
		return LinkedDeclaration{}, err
	}
	declaration, err := NewLinkedDeclaration(symbol, ruleID, body, basis)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	if !equalCanonical(data, canonicalLinkedDeclaration(declaration)) {
		return LinkedDeclaration{}, fmt.Errorf("linked declaration is not canonical")
	}
	return declaration, nil
}

func decodeDeclarationValue(
	data []byte,
	budget *artifactBudget,
	depth int,
) (DeclarationValue, error) {
	if err := budget.consumeValue(depth); err != nil {
		return nil, err
	}
	reader, err := newCanonicalReader(data, "declaration-value.v1")
	if err != nil {
		return nil, err
	}
	tag, err := reader.readByte()
	if err != nil {
		return nil, err
	}
	var value DeclarationValue
	switch DeclarationValueKind(tag) {
	case DeclarationText:
		decoded, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		if budgetErr := budget.consumeScalar("declaration text", len(decoded), maximumScalarStringBytes); budgetErr != nil {
			return nil, budgetErr
		}
		value = NewTextValue(decoded)
	case DeclarationUnsigned:
		decoded, readErr := reader.readUint64()
		if readErr != nil {
			return nil, readErr
		}
		value = NewUnsignedValue(decoded)
	case DeclarationBoolean:
		decoded, readErr := reader.readBool()
		if readErr != nil {
			return nil, readErr
		}
		value = NewBooleanValue(decoded)
	case DeclarationBytes:
		decoded, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		if budgetErr := budget.consumeScalar("declaration bytes", len(decoded), maximumScalarByteValueBytes); budgetErr != nil {
			return nil, budgetErr
		}
		value = NewBytesValue(decoded)
	case DeclarationSymbol:
		decoded, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		symbol, decodeErr := decodeSchemaSymbol(decoded)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if budgetErr := budget.consumeScalar("schema symbol", len(symbol.String()), maximumScalarStringBytes); budgetErr != nil {
			return nil, budgetErr
		}
		value, err = NewSymbolValue(symbol)
	case DeclarationSequence:
		value, err = decodeSequenceValue(reader, false, budget, depth)
	case DeclarationSet:
		value, err = decodeSequenceValue(reader, true, budget, depth)
	case DeclarationRecord:
		fieldBytes, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		fields, decodeErr := decodeFields(
			fieldBytes,
			"declaration-record-fields.v1",
			budget,
			depth,
		)
		if decodeErr != nil {
			return nil, decodeErr
		}
		value, err = NewRecordValue(fields)
	default:
		return nil, fmt.Errorf("unknown declaration value tag %d", tag)
	}
	if err != nil {
		return nil, err
	}
	if err := reader.requireDone(); err != nil {
		return nil, err
	}
	if !equalCanonical(data, canonicalDeclarationValue(value)) {
		return nil, fmt.Errorf("declaration value is not canonical")
	}
	return value, nil
}

func decodeSequenceValue(
	reader *canonicalReader,
	asSet bool,
	budget *artifactBudget,
	depth int,
) (DeclarationValue, error) {
	count, err := reader.readCountLimit(maximumValuesPerCollection, "declaration collection")
	if err != nil {
		return nil, err
	}
	values := make([]DeclarationValue, 0, count)
	for index := 0; index < count; index++ {
		valueBytes, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		value, decodeErr := decodeDeclarationValue(valueBytes, budget, depth+1)
		if decodeErr != nil {
			return nil, decodeErr
		}
		values = append(values, value)
	}
	if asSet {
		return NewSetValue(values)
	}
	return NewSequenceValue(values)
}

func decodeFields(
	data []byte,
	domain string,
	budget *artifactBudget,
	containerDepth int,
) ([]DeclarationField, error) {
	reader, err := newCanonicalReader(data, domain)
	if err != nil {
		return nil, err
	}
	count, err := reader.readCountLimit(maximumFieldsPerRecord, "declaration record")
	if err != nil {
		return nil, err
	}
	if err := budget.consumeFields(count); err != nil {
		return nil, err
	}
	fields := make([]DeclarationField, 0, count)
	for index := 0; index < count; index++ {
		name, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		if budgetErr := budget.consumeScalar("declaration field name", len(name), maximumScalarStringBytes); budgetErr != nil {
			return nil, budgetErr
		}
		valueBytes, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		value, decodeErr := decodeDeclarationValue(valueBytes, budget, containerDepth+1)
		if decodeErr != nil {
			return nil, decodeErr
		}
		field, buildErr := NewDeclarationField(name, value)
		if buildErr != nil {
			return nil, buildErr
		}
		fields = append(fields, field)
	}
	if err := reader.requireDone(); err != nil {
		return nil, err
	}
	canonical := canonicalFields(fields, domain)
	if !equalCanonical(data, canonical) {
		return nil, fmt.Errorf("declaration fields are not canonical")
	}
	return fields, nil
}

func decodeDeclarationBasis(data []byte) (DeclarationBasis, error) {
	reader, err := newCanonicalReader(data, "declaration-basis.v1")
	if err != nil {
		return nil, err
	}
	tag, err := reader.readByte()
	if err != nil {
		return nil, err
	}
	var basis DeclarationBasis
	switch DeclarationBasisKind(tag) {
	case SourceAuthoredBasis:
		referenceText, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		reference, buildErr := typedmemory.NewProvenanceRef(referenceText)
		if buildErr != nil {
			return nil, buildErr
		}
		locationBytes, readErr := reader.readBytes()
		if readErr != nil {
			return nil, readErr
		}
		location, decodeErr := decodeSourceLocation(locationBytes)
		if decodeErr != nil {
			return nil, decodeErr
		}
		ruleText, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		ruleID, buildErr := typedmemory.NewCompilerRuleID(ruleText)
		if buildErr != nil {
			return nil, buildErr
		}
		provenance, buildErr := typedmemory.NewFPFSourceProvenance(reference, location, ruleID)
		if buildErr != nil {
			return nil, buildErr
		}
		basis, err = NewSourceDeclarationBasis(provenance)
	case CompilerDerivedBasis:
		ruleText, readErr := reader.readString()
		if readErr != nil {
			return nil, readErr
		}
		ruleID, buildErr := typedmemory.NewCompilerRuleID(ruleText)
		if buildErr != nil {
			return nil, buildErr
		}
		count, readErr := reader.readCountLimit(maximumSourcesPerBasis, "compiler-derived source inputs")
		if readErr != nil {
			return nil, readErr
		}
		inputs := make([]typedmemory.SourceLocation, 0, count)
		for index := 0; index < count; index++ {
			locationBytes, locationErr := reader.readBytes()
			if locationErr != nil {
				return nil, locationErr
			}
			location, decodeErr := decodeSourceLocation(locationBytes)
			if decodeErr != nil {
				return nil, decodeErr
			}
			inputs = append(inputs, location)
		}
		basis, err = NewCompilerDerivedDeclarationBasis(ruleID, inputs)
	default:
		return nil, fmt.Errorf("unknown declaration basis tag %d", tag)
	}
	if err != nil {
		return nil, err
	}
	if err := reader.requireDone(); err != nil {
		return nil, err
	}
	if !equalCanonical(data, canonicalDeclarationBasis(basis)) {
		return nil, fmt.Errorf("declaration basis is not canonical")
	}
	return basis, nil
}

func decodeSchemaSymbol(data []byte) (typedmemory.SchemaSymbolRef, error) {
	reader, err := newCanonicalReader(data, "schema-symbol.v1")
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	kindTag, err := reader.readByte()
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	key, err := reader.readString()
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	if err := reader.requireDone(); err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	symbol, err := newSchemaSymbol(typedmemory.SchemaSymbolKind(kindTag), key)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	if !equalCanonical(data, canonicalSchemaSymbol(symbol)) {
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf("schema symbol is not canonical")
	}
	return symbol, nil
}

func newSchemaSymbol(
	kind typedmemory.SchemaSymbolKind,
	key string,
) (typedmemory.SchemaSymbolRef, error) {
	switch kind {
	case typedmemory.ContextSymbol:
		ref, err := typedmemory.NewBoundedContextRef(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.BoundedContextSymbolRef(ref)
	case typedmemory.KindSymbol:
		id, err := typedmemory.NewKindID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSymbolRef(id)
	case typedmemory.SlotKindSymbol:
		return newSlotKindSymbol(key)
	case typedmemory.RefKindSymbol:
		id, err := typedmemory.NewRefKindID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.RefKindSymbolRef(id)
	case typedmemory.BridgeSymbol:
		id, err := typedmemory.NewContextBridgeID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ContextBridgeSymbolRef(id)
	case typedmemory.SignatureSymbol:
		id, err := typedmemory.NewSignatureID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.RelationSymbolRef(id)
	case typedmemory.ShapeSymbol:
		id, err := typedmemory.NewShapeID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case typedmemory.CodecSymbol:
		id, err := typedmemory.NewCodecID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.CodecSymbolRef(id)
	case typedmemory.ConstraintSymbol:
		id, err := typedmemory.NewConstraintID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ConstraintSymbolRef(id)
	case typedmemory.EntitySetSymbol:
		id, err := typedmemory.NewEntitySetSymbolID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.EntitySetSymbolRef(id)
	case typedmemory.KindSignatureSymbol:
		id, err := typedmemory.NewKindSignatureSymbolID(key)
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSignatureSymbolRef(id)
	default:
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf("unknown schema symbol kind %d", kind)
	}
}

func newSlotKindSymbol(key string) (typedmemory.SchemaSymbolRef, error) {
	parts := strings.Split(key, "/slot/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf("invalid SlotKind symbol key %q", key)
	}
	signature, err := typedmemory.NewSignatureID(parts[0])
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	slotKind, err := typedmemory.NewSlotKindID(parts[1])
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	return typedmemory.SlotKindSymbolRef(signature, slotKind)
}

func decodeSourceLocation(data []byte) (typedmemory.SourceLocation, error) {
	reader, err := newCanonicalReader(data, "source-location.v1")
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	unitText, err := reader.readString()
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	unitID, err := typedmemory.NewSourceUnitID(unitText)
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	revisionText, err := reader.readString()
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	revision, err := typedmemory.NewSourceRevision(revisionText)
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	digestText, err := reader.readString()
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	start, err := reader.readUint64()
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	end, err := reader.readUint64()
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	lineRange, err := typedmemory.NewSourceLineRange(start, end)
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	hasPattern, err := reader.readBool()
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	var location typedmemory.SourceLocation
	if hasPattern {
		patternText, readErr := reader.readString()
		if readErr != nil {
			return typedmemory.SourceLocation{}, readErr
		}
		patternID, buildErr := typedmemory.NewPatternID(patternText)
		if buildErr != nil {
			return typedmemory.SourceLocation{}, buildErr
		}
		location, err = typedmemory.NewPatternedSourceLocation(
			unitID,
			revision,
			digest,
			lineRange,
			patternID,
		)
	} else {
		location, err = typedmemory.NewUnpatternedSourceLocation(
			unitID,
			revision,
			digest,
			lineRange,
		)
	}
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}
	if err := reader.requireDone(); err != nil {
		return typedmemory.SourceLocation{}, err
	}
	if !equalCanonical(data, canonicalSourceLocation(location)) {
		return typedmemory.SourceLocation{}, fmt.Errorf("source location is not canonical")
	}
	return location, nil
}

func decodeCoverageManifest(data []byte) (typedmemory.CoverageManifest, error) {
	reader, err := newCanonicalReader(data, "coverage-manifest.v1")
	if err != nil {
		return typedmemory.CoverageManifest{}, err
	}
	count, err := reader.readCountLimit(maximumCoverageEntries, "coverage manifest")
	if err != nil {
		return typedmemory.CoverageManifest{}, err
	}
	entries := make([]typedmemory.CoverageEntry, 0, count)
	for index := 0; index < count; index++ {
		entryBytes, readErr := reader.readBytes()
		if readErr != nil {
			return typedmemory.CoverageManifest{}, readErr
		}
		entry, decodeErr := decodeCoverageEntry(entryBytes)
		if decodeErr != nil {
			return typedmemory.CoverageManifest{}, decodeErr
		}
		entries = append(entries, entry)
	}
	if err := reader.requireDone(); err != nil {
		return typedmemory.CoverageManifest{}, err
	}
	manifest, err := typedmemory.NewCoverageManifest(entries)
	if err != nil {
		return typedmemory.CoverageManifest{}, err
	}
	if !equalCanonical(data, canonicalCoverageManifest(manifest)) {
		return typedmemory.CoverageManifest{}, fmt.Errorf("coverage manifest is not canonical")
	}
	return manifest, nil
}

func decodeCoverageEntry(data []byte) (typedmemory.CoverageEntry, error) {
	reader, err := newCanonicalReader(data, "coverage-entry.v1")
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	subjectText, err := reader.readString()
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	subject, err := decodeCoverageSubject(subjectText)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	postureText, err := reader.readString()
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	locationBytes, err := reader.readBytes()
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	location, err := decodeSourceLocation(locationBytes)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	rationale, err := reader.readString()
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	if err := reader.requireDone(); err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	var entry typedmemory.CoverageEntry
	switch postureText {
	case typedmemory.CoverageCompiled.String():
		entry, err = typedmemory.NewCompiledCoverageEntry(subject, location)
	case typedmemory.CoverageSourceOnly.String():
		entry, err = typedmemory.NewSourceOnlyCoverageEntry(subject, location, rationale)
	case typedmemory.CoverageUnsupported.String():
		entry, err = typedmemory.NewUnsupportedCoverageEntry(subject, location, rationale)
	default:
		return typedmemory.CoverageEntry{}, fmt.Errorf("unknown coverage posture %q", postureText)
	}
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	return entry, nil
}

func decodeCoverageSubject(raw string) (typedmemory.CoverageSubject, error) {
	const sourcePrefix = "source-unit:"
	const symbolPrefix = "schema-symbol:"
	if strings.HasPrefix(raw, sourcePrefix) {
		value := strings.TrimPrefix(raw, sourcePrefix)
		unitID, err := typedmemory.NewSourceUnitID(value)
		if err != nil {
			return typedmemory.CoverageSubject{}, err
		}
		return typedmemory.SourceUnitCoverage(unitID)
	}
	if strings.HasPrefix(raw, symbolPrefix) {
		value := strings.TrimPrefix(raw, symbolPrefix)
		kindEnd := strings.IndexByte(value, ':')
		if kindEnd < 1 {
			return typedmemory.CoverageSubject{}, fmt.Errorf("invalid schema-symbol coverage subject %q", raw)
		}
		kindText := value[:kindEnd]
		key := value[kindEnd+1:]
		kind, err := schemaSymbolKindFromString(kindText)
		if err != nil {
			return typedmemory.CoverageSubject{}, err
		}
		symbol, err := newSchemaSymbol(kind, key)
		if err != nil {
			return typedmemory.CoverageSubject{}, err
		}
		return typedmemory.SchemaSymbolCoverage(symbol)
	}
	return typedmemory.CoverageSubject{}, fmt.Errorf("unknown coverage subject %q", raw)
}

func schemaSymbolKindFromString(raw string) (typedmemory.SchemaSymbolKind, error) {
	kinds := []typedmemory.SchemaSymbolKind{
		typedmemory.ContextSymbol,
		typedmemory.KindSymbol,
		typedmemory.SlotKindSymbol,
		typedmemory.RefKindSymbol,
		typedmemory.BridgeSymbol,
		typedmemory.SignatureSymbol,
		typedmemory.ShapeSymbol,
		typedmemory.CodecSymbol,
		typedmemory.ConstraintSymbol,
		typedmemory.EntitySetSymbol,
		typedmemory.KindSignatureSymbol,
	}
	for _, kind := range kinds {
		if kind.String() == raw {
			return kind, nil
		}
	}
	return 0, fmt.Errorf("unknown schema symbol kind %q", raw)
}

func decodeSymbolManifest(data []byte) (SymbolManifest, error) {
	reader, err := newCanonicalReader(data, "symbol-manifest.v1")
	if err != nil {
		return SymbolManifest{}, err
	}
	count, err := reader.readCountLimit(maximumDeclarations, "symbol manifest")
	if err != nil {
		return SymbolManifest{}, err
	}
	entries := make([]DeclarationProjection, 0, count)
	for index := 0; index < count; index++ {
		entryBytes, readErr := reader.readBytes()
		if readErr != nil {
			return SymbolManifest{}, readErr
		}
		entry, decodeErr := decodeSymbolManifestEntry(entryBytes)
		if decodeErr != nil {
			return SymbolManifest{}, decodeErr
		}
		entries = append(entries, entry)
	}
	if err := reader.requireDone(); err != nil {
		return SymbolManifest{}, err
	}
	manifest := SymbolManifest{entries: entries}
	if !equalCanonical(data, canonicalSymbolManifest(manifest)) {
		return SymbolManifest{}, fmt.Errorf("symbol manifest is not canonical")
	}
	return manifest, nil
}

func decodeSymbolManifestEntry(data []byte) (DeclarationProjection, error) {
	reader, err := newCanonicalReader(data, "symbol-manifest-entry.v1")
	if err != nil {
		return DeclarationProjection{}, err
	}
	symbolBytes, err := reader.readBytes()
	if err != nil {
		return DeclarationProjection{}, err
	}
	symbol, err := decodeSchemaSymbol(symbolBytes)
	if err != nil {
		return DeclarationProjection{}, err
	}
	digestText, err := reader.readString()
	if err != nil {
		return DeclarationProjection{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return DeclarationProjection{}, err
	}
	ruleText, err := reader.readString()
	if err != nil {
		return DeclarationProjection{}, err
	}
	ruleID, err := typedmemory.NewCompilerRuleID(ruleText)
	if err != nil {
		return DeclarationProjection{}, err
	}
	basisTag, err := reader.readByte()
	if err != nil {
		return DeclarationProjection{}, err
	}
	basisKind := DeclarationBasisKind(basisTag)
	if basisKind != SourceAuthoredBasis && basisKind != CompilerDerivedBasis {
		return DeclarationProjection{}, fmt.Errorf("unknown declaration basis kind %d", basisTag)
	}
	dependencyCount, err := reader.readCountLimit(
		maximumDependenciesPerDeclaration,
		"symbol manifest dependencies",
	)
	if err != nil {
		return DeclarationProjection{}, err
	}
	dependencies := make([]typedmemory.SchemaSymbolRef, 0, dependencyCount)
	for index := 0; index < dependencyCount; index++ {
		dependencyBytes, readErr := reader.readBytes()
		if readErr != nil {
			return DeclarationProjection{}, readErr
		}
		dependency, decodeErr := decodeSchemaSymbol(dependencyBytes)
		if decodeErr != nil {
			return DeclarationProjection{}, decodeErr
		}
		dependencies = append(dependencies, dependency)
	}
	sourceCount, err := reader.readCountLimit(maximumSourcesPerBasis, "symbol manifest sources")
	if err != nil {
		return DeclarationProjection{}, err
	}
	sources := make([]typedmemory.SourceLocation, 0, sourceCount)
	for index := 0; index < sourceCount; index++ {
		sourceBytes, readErr := reader.readBytes()
		if readErr != nil {
			return DeclarationProjection{}, readErr
		}
		source, decodeErr := decodeSourceLocation(sourceBytes)
		if decodeErr != nil {
			return DeclarationProjection{}, decodeErr
		}
		sources = append(sources, source)
	}
	if err := reader.requireDone(); err != nil {
		return DeclarationProjection{}, err
	}
	return DeclarationProjection{
		symbol:       symbol,
		digest:       digest,
		ruleID:       ruleID,
		basisKind:    basisKind,
		dependencies: dependencies,
		sources:      sources,
	}, nil
}
