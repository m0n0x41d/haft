package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

// SourceLineRange identifies an exact inclusive line interval in one source
// carrier. Zero and reversed ranges are deliberately unrepresentable.
type SourceLineRange struct {
	start uint64
	end   uint64
}

func NewSourceLineRange(start, end uint64) (SourceLineRange, error) {
	if start == 0 {
		return SourceLineRange{}, fmt.Errorf("source line range must start at a positive line")
	}
	if end < start {
		return SourceLineRange{}, fmt.Errorf("source line range end must not precede its start")
	}
	return SourceLineRange{start: start, end: end}, nil
}

func (lineRange SourceLineRange) Start() uint64 { return lineRange.start }

func (lineRange SourceLineRange) End() uint64 { return lineRange.end }

func (lineRange SourceLineRange) valid() bool {
	return lineRange.start > 0 && lineRange.end >= lineRange.start
}

// SourceLocation identifies exact FPF publication bytes. PatternID is absent
// for source units which are not owned by one pattern; its presence is exposed
// explicitly rather than represented by a sentinel string.
type SourceLocation struct {
	unitID      SourceUnitID
	revision    SourceRevision
	contentHash SHA256Digest
	lineRange   SourceLineRange
	patternID   PatternID
}

func NewUnpatternedSourceLocation(
	unitID SourceUnitID,
	revision SourceRevision,
	contentHash SHA256Digest,
	lineRange SourceLineRange,
) (SourceLocation, error) {
	return newSourceLocation(unitID, revision, contentHash, lineRange, PatternID{})
}

func NewPatternedSourceLocation(
	unitID SourceUnitID,
	revision SourceRevision,
	contentHash SHA256Digest,
	lineRange SourceLineRange,
	patternID PatternID,
) (SourceLocation, error) {
	if !patternID.valid() {
		return SourceLocation{}, fmt.Errorf("source PatternID is required")
	}
	return newSourceLocation(unitID, revision, contentHash, lineRange, patternID)
}

func newSourceLocation(
	unitID SourceUnitID,
	revision SourceRevision,
	contentHash SHA256Digest,
	lineRange SourceLineRange,
	patternID PatternID,
) (SourceLocation, error) {
	if !unitID.valid() {
		return SourceLocation{}, fmt.Errorf("source unit ID is required")
	}
	if !revision.valid() {
		return SourceLocation{}, fmt.Errorf("source revision is required")
	}
	if !contentHash.valid() {
		return SourceLocation{}, fmt.Errorf("source content hash is required")
	}
	if !lineRange.valid() {
		return SourceLocation{}, fmt.Errorf("source line range is required")
	}
	return SourceLocation{
		unitID:      unitID,
		revision:    revision,
		contentHash: contentHash,
		lineRange:   lineRange,
		patternID:   patternID,
	}, nil
}

func (location SourceLocation) UnitID() SourceUnitID { return location.unitID }

func (location SourceLocation) Revision() SourceRevision { return location.revision }

func (location SourceLocation) ContentHash() SHA256Digest { return location.contentHash }

func (location SourceLocation) LineRange() SourceLineRange { return location.lineRange }

func (location SourceLocation) PatternID() (PatternID, bool) {
	return location.patternID, location.patternID.valid()
}

func (location SourceLocation) valid() bool {
	return location.unitID.valid() &&
		location.revision.valid() &&
		location.contentHash.valid() &&
		location.lineRange.valid()
}

func (location SourceLocation) canonicalBytes() []byte {
	writer := newCanonicalWriter("source-location.v1")
	writer.addString(location.unitID.String())
	writer.addString(location.revision.String())
	writer.addString(location.contentHash.String())
	writer.addUint64(location.lineRange.Start())
	writer.addUint64(location.lineRange.End())
	writer.addString(location.patternID.String())
	return writer.bytes()
}

// DeclarationProvenance is a closed provenance union. It distinguishes a
// declaration copied from FPF, a representation lowering derived by the
// compiler from exact FPF inputs, and a declaration compiled from a project
// Local-Practice/DPF carrier.
type DeclarationProvenance interface {
	Reference() ProvenanceRef
	CanonicalBytes() []byte
	declarationProvenanceVariant()
}

type FPFSourceProvenance struct {
	reference ProvenanceRef
	location  SourceLocation
	ruleID    CompilerRuleID
}

func NewFPFSourceProvenance(
	reference ProvenanceRef,
	location SourceLocation,
	ruleID CompilerRuleID,
) (FPFSourceProvenance, error) {
	if !reference.valid() {
		return FPFSourceProvenance{}, fmt.Errorf("FPF provenance reference is required")
	}
	if !location.valid() {
		return FPFSourceProvenance{}, fmt.Errorf("FPF source location is required")
	}
	if !ruleID.valid() {
		return FPFSourceProvenance{}, fmt.Errorf("FPF compiler rule ID is required")
	}
	return FPFSourceProvenance{
		reference: reference,
		location:  location,
		ruleID:    ruleID,
	}, nil
}

func (provenance FPFSourceProvenance) Reference() ProvenanceRef {
	return provenance.reference
}

func (provenance FPFSourceProvenance) Location() SourceLocation {
	return provenance.location
}

func (provenance FPFSourceProvenance) CompilerRuleID() CompilerRuleID {
	return provenance.ruleID
}

func (provenance FPFSourceProvenance) CanonicalBytes() []byte {
	writer := newCanonicalWriter("fpf-source-provenance.v1")
	writer.addString(provenance.reference.String())
	writer.addBytes(provenance.location.canonicalBytes())
	writer.addString(provenance.ruleID.String())
	return writer.bytes()
}

func (FPFSourceProvenance) declarationProvenanceVariant() {}

// CompilerDerivedProvenance identifies representation supplied by Haft's
// compiler rather than authored by FPF. Every derivation retains the exact FPF
// source locations it consumed and a versioned compiler rule. This keeps a
// ClaimGraph codec/shape lowering, publication context, or other mechanism
// honest without turning it into a fabricated FPF declaration.
type CompilerDerivedProvenance struct {
	reference ProvenanceRef
	inputs    []SourceLocation
	ruleID    CompilerRuleID
}

func NewCompilerDerivedProvenance(
	reference ProvenanceRef,
	inputs []SourceLocation,
	ruleID CompilerRuleID,
) (CompilerDerivedProvenance, error) {
	if !reference.valid() {
		return CompilerDerivedProvenance{}, fmt.Errorf("compiler-derived provenance reference is required")
	}
	if len(inputs) == 0 {
		return CompilerDerivedProvenance{}, fmt.Errorf("compiler-derived provenance requires at least one source input")
	}
	if !ruleID.valid() {
		return CompilerDerivedProvenance{}, fmt.Errorf("compiler-derived provenance rule ID is required")
	}

	owned := append([]SourceLocation(nil), inputs...)
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].canonicalBytes(), owned[right].canonicalBytes()) < 0
	})
	for index, input := range owned {
		if !input.valid() {
			return CompilerDerivedProvenance{}, fmt.Errorf("compiler-derived source input %d is invalid", index)
		}
		if index == 0 {
			continue
		}
		if bytes.Equal(input.canonicalBytes(), owned[index-1].canonicalBytes()) {
			return CompilerDerivedProvenance{}, fmt.Errorf("duplicate compiler-derived source input %q", input.UnitID().String())
		}
	}

	return CompilerDerivedProvenance{
		reference: reference,
		inputs:    owned,
		ruleID:    ruleID,
	}, nil
}

func (provenance CompilerDerivedProvenance) Reference() ProvenanceRef {
	return provenance.reference
}

func (provenance CompilerDerivedProvenance) Inputs() []SourceLocation {
	return append([]SourceLocation(nil), provenance.inputs...)
}

func (provenance CompilerDerivedProvenance) CompilerRuleID() CompilerRuleID {
	return provenance.ruleID
}

func (provenance CompilerDerivedProvenance) CanonicalBytes() []byte {
	writer := newCanonicalWriter("compiler-derived-provenance.v1")
	writer.addString(provenance.reference.String())
	writer.addString(provenance.ruleID.String())
	for _, input := range provenance.inputs {
		writer.addBytes(input.canonicalBytes())
	}
	return writer.bytes()
}

func (provenance CompilerDerivedProvenance) valid() bool {
	rebuilt, err := NewCompilerDerivedProvenance(
		provenance.reference,
		provenance.inputs,
		provenance.ruleID,
	)
	return err == nil && bytes.Equal(
		rebuilt.CanonicalBytes(),
		provenance.CanonicalBytes(),
	)
}

func (CompilerDerivedProvenance) declarationProvenanceVariant() {}

type SignatureBlockRow uint8

const (
	SubjectBlockRow SignatureBlockRow = iota + 1
	VocabularyRow
	LawsRow
	ApplicabilityRow
)

func (row SignatureBlockRow) String() string {
	switch row {
	case SubjectBlockRow:
		return "subject_block"
	case VocabularyRow:
		return "vocabulary"
	case LawsRow:
		return "laws"
	case ApplicabilityRow:
		return "applicability"
	default:
		return ""
	}
}

func (row SignatureBlockRow) valid() bool { return row.String() != "" }

type ManifestDirection uint8

const (
	ManifestImport ManifestDirection = iota + 1
	ManifestProvide
)

func (direction ManifestDirection) String() string {
	switch direction {
	case ManifestImport:
		return "import"
	case ManifestProvide:
		return "provide"
	default:
		return ""
	}
}

func (direction ManifestDirection) valid() bool { return direction.String() != "" }

type SignatureManifestRef struct {
	id      string
	version string
}

func NewSignatureManifestRef(id, version string) (SignatureManifestRef, error) {
	parsedID, err := parseQualifiedIdentifier("signature manifest ID", id)
	if err != nil {
		return SignatureManifestRef{}, err
	}
	parsedVersion, err := parseOpaqueIdentifier("signature manifest version", version)
	if err != nil {
		return SignatureManifestRef{}, err
	}
	return SignatureManifestRef{id: parsedID, version: parsedVersion}, nil
}

func (ref SignatureManifestRef) ID() string { return ref.id }

func (ref SignatureManifestRef) Version() string { return ref.version }

func (ref SignatureManifestRef) String() string {
	return fmt.Sprintf(
		"signature-manifest:%d:%s:%d:%s",
		len(ref.id),
		ref.id,
		len(ref.version),
		ref.version,
	)
}

func (ref SignatureManifestRef) valid() bool { return ref.id != "" && ref.version != "" }

type ManifestSymbolBasis struct {
	manifest  SignatureManifestRef
	direction ManifestDirection
	symbol    SchemaSymbolRef
}

func NewManifestSymbolBasis(
	manifest SignatureManifestRef,
	direction ManifestDirection,
	symbol SchemaSymbolRef,
) (ManifestSymbolBasis, error) {
	if !manifest.valid() {
		return ManifestSymbolBasis{}, fmt.Errorf("signature manifest reference is required")
	}
	if !direction.valid() {
		return ManifestSymbolBasis{}, fmt.Errorf("manifest direction is required")
	}
	if !symbol.valid() {
		return ManifestSymbolBasis{}, fmt.Errorf("manifest symbol is required")
	}
	return ManifestSymbolBasis{
		manifest:  manifest,
		direction: direction,
		symbol:    symbol,
	}, nil
}

func (basis ManifestSymbolBasis) Manifest() SignatureManifestRef { return basis.manifest }

func (basis ManifestSymbolBasis) Direction() ManifestDirection { return basis.direction }

func (basis ManifestSymbolBasis) Symbol() SchemaSymbolRef { return basis.symbol }

func (basis ManifestSymbolBasis) valid() bool {
	return basis.manifest.valid() && basis.direction.valid() && basis.symbol.valid()
}

type ProjectSourceProvenance struct {
	reference     ProvenanceRef
	carrier       CarrierRef
	edition       CarrierEdition
	contentHash   SHA256Digest
	lineRange     SourceLineRange
	ruleID        CompilerRuleID
	context       BoundedContextRef
	baseTypeEnv   TypeEnvRef
	signatureRow  SignatureBlockRow
	manifestBasis ManifestSymbolBasis
}

type ProjectSourceProvenanceBuilder struct {
	value ProjectSourceProvenance
	err   error
}

func NewProjectSourceProvenanceBuilder(
	reference ProvenanceRef,
	carrier CarrierRef,
	edition CarrierEdition,
	contentHash SHA256Digest,
) *ProjectSourceProvenanceBuilder {
	return &ProjectSourceProvenanceBuilder{
		value: ProjectSourceProvenance{
			reference:   reference,
			carrier:     carrier,
			edition:     edition,
			contentHash: contentHash,
		},
	}
}

func (builder *ProjectSourceProvenanceBuilder) SetDeclarationRange(
	lineRange SourceLineRange,
) *ProjectSourceProvenanceBuilder {
	builder.value.lineRange = lineRange
	return builder
}

func (builder *ProjectSourceProvenanceBuilder) SetCompilerRule(
	ruleID CompilerRuleID,
) *ProjectSourceProvenanceBuilder {
	builder.value.ruleID = ruleID
	return builder
}

func (builder *ProjectSourceProvenanceBuilder) SetBoundedContext(
	context BoundedContextRef,
) *ProjectSourceProvenanceBuilder {
	builder.value.context = context
	return builder
}

func (builder *ProjectSourceProvenanceBuilder) SetBaseTypeEnv(
	base TypeEnvRef,
) *ProjectSourceProvenanceBuilder {
	builder.value.baseTypeEnv = base
	return builder
}

func (builder *ProjectSourceProvenanceBuilder) SetSignatureBlockRow(
	row SignatureBlockRow,
) *ProjectSourceProvenanceBuilder {
	builder.value.signatureRow = row
	return builder
}

func (builder *ProjectSourceProvenanceBuilder) SetManifestBasis(
	basis ManifestSymbolBasis,
) *ProjectSourceProvenanceBuilder {
	builder.value.manifestBasis = basis
	return builder
}

func (builder *ProjectSourceProvenanceBuilder) Build() (ProjectSourceProvenance, error) {
	if builder == nil {
		return ProjectSourceProvenance{}, fmt.Errorf("project provenance builder is required")
	}
	if builder.err != nil {
		return ProjectSourceProvenance{}, builder.err
	}
	if err := builder.value.validate(); err != nil {
		return ProjectSourceProvenance{}, err
	}
	return builder.value, nil
}

func (provenance ProjectSourceProvenance) validate() error {
	checks := []struct {
		valid   bool
		message string
	}{
		{provenance.reference.valid(), "project provenance reference is required"},
		{provenance.carrier.valid(), "project source carrier is required"},
		{provenance.edition.valid(), "project source carrier edition is required"},
		{provenance.contentHash.valid(), "project source content hash is required"},
		{provenance.lineRange.valid(), "project declaration line range is required"},
		{provenance.ruleID.valid(), "project compiler rule ID is required"},
		{provenance.context.valid(), "project bounded context is required"},
		{provenance.baseTypeEnv.valid(), "project base TypeEnv is required"},
		{provenance.signatureRow.valid(), "signature-block row is required"},
		{provenance.manifestBasis.valid(), "manifest symbol basis is required"},
	}
	for _, check := range checks {
		if !check.valid {
			return fmt.Errorf("%s", check.message)
		}
	}
	return nil
}

func (provenance ProjectSourceProvenance) Reference() ProvenanceRef {
	return provenance.reference
}

func (provenance ProjectSourceProvenance) Carrier() CarrierRef { return provenance.carrier }

func (provenance ProjectSourceProvenance) Edition() CarrierEdition { return provenance.edition }

func (provenance ProjectSourceProvenance) ContentHash() SHA256Digest {
	return provenance.contentHash
}

func (provenance ProjectSourceProvenance) LineRange() SourceLineRange {
	return provenance.lineRange
}

func (provenance ProjectSourceProvenance) CompilerRuleID() CompilerRuleID {
	return provenance.ruleID
}

func (provenance ProjectSourceProvenance) BoundedContext() BoundedContextRef {
	return provenance.context
}

func (provenance ProjectSourceProvenance) BaseTypeEnv() TypeEnvRef {
	return provenance.baseTypeEnv
}

func (provenance ProjectSourceProvenance) SignatureBlockRow() SignatureBlockRow {
	return provenance.signatureRow
}

func (provenance ProjectSourceProvenance) ManifestBasis() ManifestSymbolBasis {
	return provenance.manifestBasis
}

func (provenance ProjectSourceProvenance) CanonicalBytes() []byte {
	writer := newCanonicalWriter("project-source-provenance.v1")
	writer.addString(provenance.reference.String())
	writer.addString(provenance.carrier.String())
	writer.addString(provenance.edition.String())
	writer.addString(provenance.contentHash.String())
	writer.addUint64(provenance.lineRange.Start())
	writer.addUint64(provenance.lineRange.End())
	writer.addString(provenance.ruleID.String())
	writer.addString(provenance.context.String())
	writer.addString(provenance.baseTypeEnv.String())
	writer.addString(provenance.signatureRow.String())
	writer.addString(provenance.manifestBasis.Manifest().String())
	writer.addString(provenance.manifestBasis.Direction().String())
	writer.addString(provenance.manifestBasis.Symbol().String())
	return writer.bytes()
}

func (ProjectSourceProvenance) declarationProvenanceVariant() {}

func validDeclarationProvenance(provenance DeclarationProvenance) bool {
	switch value := provenance.(type) {
	case FPFSourceProvenance:
		return value.reference.valid() && value.location.valid() && value.ruleID.valid()
	case CompilerDerivedProvenance:
		return value.valid()
	case ProjectSourceProvenance:
		return value.validate() == nil
	default:
		return false
	}
}

func cloneDeclarationProvenance(
	provenance DeclarationProvenance,
) DeclarationProvenance {
	switch value := provenance.(type) {
	case FPFSourceProvenance:
		return value
	case CompilerDerivedProvenance:
		value.inputs = append([]SourceLocation(nil), value.inputs...)
		return value
	case ProjectSourceProvenance:
		return value
	default:
		return nil
	}
}
