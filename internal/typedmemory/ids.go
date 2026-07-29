package typedmemory

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const sha256Prefix = "sha256:"

type SHA256Digest struct {
	value string
}

func NewSHA256Digest(raw string) (SHA256Digest, error) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, sha256Prefix) {
		return SHA256Digest{}, fmt.Errorf("digest must start with %q", sha256Prefix)
	}

	hexValue := strings.TrimPrefix(value, sha256Prefix)
	decoded, err := hex.DecodeString(hexValue)
	if err != nil {
		return SHA256Digest{}, fmt.Errorf("digest must contain lowercase SHA-256 hex: %w", err)
	}
	if len(decoded) != 32 || hexValue != strings.ToLower(hexValue) {
		return SHA256Digest{}, fmt.Errorf("digest must contain exactly 64 lowercase SHA-256 hex characters")
	}

	return SHA256Digest{value: value}, nil
}

func (digest SHA256Digest) String() string { return digest.value }

func (digest SHA256Digest) valid() bool { return digest.value != "" }

type EntityID struct {
	value string
}

func NewEntityID(raw string) (EntityID, error) {
	value, err := parseOpaqueIdentifier("entity ID", raw)
	if err != nil {
		return EntityID{}, err
	}
	return EntityID{value: value}, nil
}

func (id EntityID) String() string { return id.value }

func (id EntityID) valid() bool { return id.value != "" }

type AssertionID struct {
	value string
}

func NewAssertionID(raw string) (AssertionID, error) {
	value, err := parseOpaqueIdentifier("assertion ID", raw)
	if err != nil {
		return AssertionID{}, err
	}
	return AssertionID{value: value}, nil
}

func (id AssertionID) String() string { return id.value }

func (id AssertionID) valid() bool { return id.value != "" }

type BatchLocalRef struct {
	value string
}

func NewBatchLocalRef(raw string) (BatchLocalRef, error) {
	value, err := parseOpaqueIdentifier("batch-local reference", raw)
	if err != nil {
		return BatchLocalRef{}, err
	}
	return BatchLocalRef{value: value}, nil
}

func (ref BatchLocalRef) String() string { return ref.value }

func (ref BatchLocalRef) valid() bool { return ref.value != "" }

type BoundedContextRef struct {
	value string
}

func NewBoundedContextRef(raw string) (BoundedContextRef, error) {
	value, err := parseOpaqueIdentifier("bounded-context reference", raw)
	if err != nil {
		return BoundedContextRef{}, err
	}
	return BoundedContextRef{value: value}, nil
}

func (ref BoundedContextRef) String() string { return ref.value }

func (ref BoundedContextRef) valid() bool { return ref.value != "" }

type TypeEnvRef struct {
	digest SHA256Digest
}

func NewTypeEnvRef(digest SHA256Digest) (TypeEnvRef, error) {
	if !digest.valid() {
		return TypeEnvRef{}, fmt.Errorf("TypeEnv digest is required")
	}
	return TypeEnvRef{digest: digest}, nil
}

func (ref TypeEnvRef) Digest() SHA256Digest { return ref.digest }

func (ref TypeEnvRef) String() string { return "typeenv:" + ref.digest.String() }

func (ref TypeEnvRef) valid() bool { return ref.digest.valid() }

// ParseTypeEnvRef parses the canonical external representation into the
// strong reference used by typed-memory validation and storage boundaries.
func ParseTypeEnvRef(raw string) (TypeEnvRef, error) {
	digestRaw, found := strings.CutPrefix(raw, "typeenv:")
	if !found {
		return TypeEnvRef{}, fmt.Errorf("TypeEnv reference is malformed")
	}
	digest, err := NewSHA256Digest(digestRaw)
	if err != nil {
		return TypeEnvRef{}, fmt.Errorf("TypeEnv reference: %w", err)
	}
	ref, err := NewTypeEnvRef(digest)
	if err != nil {
		return TypeEnvRef{}, fmt.Errorf("TypeEnv reference: %w", err)
	}
	if ref.String() != raw {
		return TypeEnvRef{}, fmt.Errorf("TypeEnv reference is not canonical")
	}
	return ref, nil
}

type KindID struct {
	value string
}

func NewKindID(raw string) (KindID, error) {
	value, err := parseQualifiedIdentifier("kind ID", raw)
	if err != nil {
		return KindID{}, err
	}
	return KindID{value: value}, nil
}

func (id KindID) String() string { return id.value }

func (id KindID) valid() bool { return id.value != "" }

type SignatureID struct {
	value string
}

func NewSignatureID(raw string) (SignatureID, error) {
	value, err := parseQualifiedIdentifier("relation-signature ID", raw)
	if err != nil {
		return SignatureID{}, err
	}
	return SignatureID{value: value}, nil
}

func (id SignatureID) String() string { return id.value }

func (id SignatureID) valid() bool { return id.value != "" }

type ShapeID struct {
	value string
}

func NewShapeID(raw string) (ShapeID, error) {
	value, err := parseQualifiedIdentifier("value-shape ID", raw)
	if err != nil {
		return ShapeID{}, err
	}
	return ShapeID{value: value}, nil
}

func (id ShapeID) String() string { return id.value }

func (id ShapeID) valid() bool { return id.value != "" }

type CodecID struct {
	value string
}

func NewCodecID(raw string) (CodecID, error) {
	value, err := parseQualifiedIdentifier("codec ID", raw)
	if err != nil {
		return CodecID{}, err
	}
	return CodecID{value: value}, nil
}

func (id CodecID) String() string { return id.value }

func (id CodecID) valid() bool { return id.value != "" }

type CanonicalizationVersion struct {
	value string
}

func NewCanonicalizationVersion(raw string) (CanonicalizationVersion, error) {
	value, err := parseOpaqueIdentifier("canonicalization version", raw)
	if err != nil {
		return CanonicalizationVersion{}, err
	}
	return CanonicalizationVersion{value: value}, nil
}

func (version CanonicalizationVersion) String() string { return version.value }

func (version CanonicalizationVersion) valid() bool { return version.value != "" }

type SlotKindID struct {
	value string
}

func NewSlotKindID(raw string) (SlotKindID, error) {
	value, err := parseQualifiedIdentifier("SlotKind ID", raw)
	if err != nil {
		return SlotKindID{}, err
	}
	return SlotKindID{value: value}, nil
}

func (id SlotKindID) String() string { return id.value }

func (id SlotKindID) valid() bool { return id.value != "" }

type RefKindID struct {
	value string
}

func NewRefKindID(raw string) (RefKindID, error) {
	value, err := parseQualifiedIdentifier("RefKind ID", raw)
	if err != nil {
		return RefKindID{}, err
	}
	return RefKindID{value: value}, nil
}

func (id RefKindID) String() string { return id.value }

func (id RefKindID) valid() bool { return id.value != "" }

type ReferenceID struct {
	value string
}

func NewReferenceID(raw string) (ReferenceID, error) {
	value, err := parseOpaqueIdentifier("reference ID", raw)
	if err != nil {
		return ReferenceID{}, err
	}
	return ReferenceID{value: value}, nil
}

func (id ReferenceID) String() string { return id.value }

func (id ReferenceID) valid() bool { return id.value != "" }

type RelationSignatureRef struct {
	typeEnv TypeEnvRef
	id      SignatureID
}

// TypedRelationDeclarationFragmentRef is the current semantic name for the
// edition-bound /signature/ coordinate. Historical carriers and wire formats
// retain RelationSignatureRef as their compatibility spelling; the coordinate
// alone does not establish a complete FPF RelationSignature episteme.
type TypedRelationDeclarationFragmentRef = RelationSignatureRef

func NewTypedRelationDeclarationFragmentRef(
	typeEnv TypeEnvRef,
	id SignatureID,
) (TypedRelationDeclarationFragmentRef, error) {
	if !typeEnv.valid() {
		return TypedRelationDeclarationFragmentRef{}, fmt.Errorf(
			"typed relation declaration fragment TypeEnv reference is required",
		)
	}
	if !id.valid() {
		return TypedRelationDeclarationFragmentRef{}, fmt.Errorf(
			"typed relation declaration fragment ID is required",
		)
	}
	return TypedRelationDeclarationFragmentRef{typeEnv: typeEnv, id: id}, nil
}

// NewRelationSignatureRef preserves the historical edition/wire spelling.
// New code should use NewTypedRelationDeclarationFragmentRef.
func NewRelationSignatureRef(typeEnv TypeEnvRef, id SignatureID) (RelationSignatureRef, error) {
	if !typeEnv.valid() {
		return RelationSignatureRef{}, fmt.Errorf("relation-signature TypeEnv reference is required")
	}
	if !id.valid() {
		return RelationSignatureRef{}, fmt.Errorf("relation-signature ID is required")
	}
	return RelationSignatureRef{typeEnv: typeEnv, id: id}, nil
}

func (ref RelationSignatureRef) TypeEnv() TypeEnvRef { return ref.typeEnv }

func (ref RelationSignatureRef) ID() SignatureID { return ref.id }

func (ref RelationSignatureRef) String() string {
	return ref.typeEnv.String() + "/signature/" + ref.id.String()
}

func (ref RelationSignatureRef) valid() bool { return ref.typeEnv.valid() && ref.id.valid() }

type ValueKindRef struct {
	typeEnv TypeEnvRef
	id      KindID
}

func NewValueKindRef(typeEnv TypeEnvRef, id KindID) (ValueKindRef, error) {
	if !typeEnv.valid() {
		return ValueKindRef{}, fmt.Errorf("value-kind TypeEnv reference is required")
	}
	if !id.valid() {
		return ValueKindRef{}, fmt.Errorf("value-kind ID is required")
	}
	return ValueKindRef{typeEnv: typeEnv, id: id}, nil
}

func (ref ValueKindRef) TypeEnv() TypeEnvRef { return ref.typeEnv }

func (ref ValueKindRef) ID() KindID { return ref.id }

func (ref ValueKindRef) String() string {
	return ref.typeEnv.String() + "/value-kind/" + ref.id.String()
}

func (ref ValueKindRef) valid() bool { return ref.typeEnv.valid() && ref.id.valid() }

type RefKindRef struct {
	typeEnv TypeEnvRef
	id      RefKindID
}

func NewRefKindRef(typeEnv TypeEnvRef, id RefKindID) (RefKindRef, error) {
	if !typeEnv.valid() {
		return RefKindRef{}, fmt.Errorf("reference-kind TypeEnv reference is required")
	}
	if !id.valid() {
		return RefKindRef{}, fmt.Errorf("reference-kind ID is required")
	}
	return RefKindRef{typeEnv: typeEnv, id: id}, nil
}

func (ref RefKindRef) TypeEnv() TypeEnvRef { return ref.typeEnv }

func (ref RefKindRef) ID() RefKindID { return ref.id }

func (ref RefKindRef) String() string {
	return ref.typeEnv.String() + "/ref-kind/" + ref.id.String()
}

func (ref RefKindRef) valid() bool { return ref.typeEnv.valid() && ref.id.valid() }

type ValueShapeRef struct {
	id     ShapeID
	digest SHA256Digest
}

func NewValueShapeRef(id ShapeID, digest SHA256Digest) (ValueShapeRef, error) {
	if !id.valid() {
		return ValueShapeRef{}, fmt.Errorf("value-shape ID is required")
	}
	if !digest.valid() {
		return ValueShapeRef{}, fmt.Errorf("value-shape digest is required")
	}
	return ValueShapeRef{id: id, digest: digest}, nil
}

func (ref ValueShapeRef) ID() ShapeID { return ref.id }

func (ref ValueShapeRef) Digest() SHA256Digest { return ref.digest }

func (ref ValueShapeRef) String() string {
	return "shape:" + ref.id.String() + "@" + ref.digest.String()
}

func (ref ValueShapeRef) valid() bool { return ref.id.valid() && ref.digest.valid() }

type CodecRef struct {
	id      CodecID
	version CanonicalizationVersion
	digest  SHA256Digest
}

func NewCodecRef(id CodecID, version CanonicalizationVersion, digest SHA256Digest) (CodecRef, error) {
	if !id.valid() {
		return CodecRef{}, fmt.Errorf("codec ID is required")
	}
	if !version.valid() {
		return CodecRef{}, fmt.Errorf("canonicalization version is required")
	}
	if !digest.valid() {
		return CodecRef{}, fmt.Errorf("codec specification digest is required")
	}
	return CodecRef{id: id, version: version, digest: digest}, nil
}

func (ref CodecRef) ID() CodecID { return ref.id }

func (ref CodecRef) Version() CanonicalizationVersion { return ref.version }

func (ref CodecRef) SpecificationDigest() SHA256Digest { return ref.digest }

func (ref CodecRef) String() string {
	return fmt.Sprintf(
		"codec:%d:%s:%d:%s:%s",
		len(ref.id.String()),
		ref.id.String(),
		len(ref.version.String()),
		ref.version.String(),
		ref.digest.String(),
	)
}

func (ref CodecRef) valid() bool {
	return ref.id.valid() && ref.version.valid() && ref.digest.valid()
}

type ScopeID struct {
	value string
}

func NewScopeID(raw string) (ScopeID, error) {
	value, err := parseQualifiedIdentifier("scope ID", raw)
	if err != nil {
		return ScopeID{}, err
	}
	return ScopeID{value: value}, nil
}

func (id ScopeID) String() string { return id.value }

type SourceUnitID struct {
	value string
}

func NewSourceUnitID(raw string) (SourceUnitID, error) {
	value, err := parseOpaqueIdentifier("source unit ID", raw)
	if err != nil {
		return SourceUnitID{}, err
	}
	return SourceUnitID{value: value}, nil
}

func (id SourceUnitID) String() string { return id.value }

func (id SourceUnitID) valid() bool { return id.value != "" }

type PatternID struct {
	value string
}

func NewPatternID(raw string) (PatternID, error) {
	value, err := parseOpaqueIdentifier("PatternID", raw)
	if err != nil {
		return PatternID{}, err
	}
	return PatternID{value: value}, nil
}

func (id PatternID) String() string { return id.value }

func (id PatternID) valid() bool { return id.value != "" }

type SourceRevision struct {
	value string
}

func NewSourceRevision(raw string) (SourceRevision, error) {
	value, err := parseOpaqueIdentifier("source revision", raw)
	if err != nil {
		return SourceRevision{}, err
	}
	return SourceRevision{value: value}, nil
}

func (revision SourceRevision) String() string { return revision.value }

func (revision SourceRevision) valid() bool { return revision.value != "" }

type CompilerRuleID struct {
	value string
}

func NewCompilerRuleID(raw string) (CompilerRuleID, error) {
	value, err := parseQualifiedIdentifier("compiler rule ID", raw)
	if err != nil {
		return CompilerRuleID{}, err
	}
	return CompilerRuleID{value: value}, nil
}

func (id CompilerRuleID) String() string { return id.value }

func (id CompilerRuleID) valid() bool { return id.value != "" }

type CarrierRef struct {
	value string
}

func NewCarrierRef(raw string) (CarrierRef, error) {
	value, err := parseOpaqueIdentifier("carrier reference", raw)
	if err != nil {
		return CarrierRef{}, err
	}
	return CarrierRef{value: value}, nil
}

func (ref CarrierRef) String() string { return ref.value }

func (ref CarrierRef) valid() bool { return ref.value != "" }

type CarrierEdition struct {
	value string
}

func NewCarrierEdition(raw string) (CarrierEdition, error) {
	value, err := parseOpaqueIdentifier("carrier edition", raw)
	if err != nil {
		return CarrierEdition{}, err
	}
	return CarrierEdition{value: value}, nil
}

func (edition CarrierEdition) String() string { return edition.value }

func (edition CarrierEdition) valid() bool { return edition.value != "" }

type ProvenanceRef struct {
	value string
}

func NewProvenanceRef(raw string) (ProvenanceRef, error) {
	value, err := parseOpaqueIdentifier("provenance reference", raw)
	if err != nil {
		return ProvenanceRef{}, err
	}
	return ProvenanceRef{value: value}, nil
}

func (ref ProvenanceRef) String() string { return ref.value }

func (ref ProvenanceRef) valid() bool { return ref.value != "" }

type EntityAlias struct {
	value string
}

func NewEntityAlias(raw string) (EntityAlias, error) {
	value, err := parseOpaqueIdentifier("entity alias", raw)
	if err != nil {
		return EntityAlias{}, err
	}
	return EntityAlias{value: value}, nil
}

func (alias EntityAlias) String() string { return alias.value }

func (alias EntityAlias) valid() bool { return alias.value != "" }

type ResolutionBasisRef struct {
	value string
}

func NewResolutionBasisRef(raw string) (ResolutionBasisRef, error) {
	value, err := parseOpaqueIdentifier("resolution basis reference", raw)
	if err != nil {
		return ResolutionBasisRef{}, err
	}
	return ResolutionBasisRef{value: value}, nil
}

func (ref ResolutionBasisRef) String() string { return ref.value }

func (ref ResolutionBasisRef) valid() bool { return ref.value != "" }

type ReconciliationBasisRef struct {
	value string
}

func NewReconciliationBasisRef(raw string) (ReconciliationBasisRef, error) {
	value, err := parseOpaqueIdentifier("reconciliation basis reference", raw)
	if err != nil {
		return ReconciliationBasisRef{}, err
	}
	return ReconciliationBasisRef{value: value}, nil
}

func (ref ReconciliationBasisRef) String() string { return ref.value }

func (ref ReconciliationBasisRef) valid() bool { return ref.value != "" }

type RuleRef struct {
	value string
}

func NewRuleRef(raw string) (RuleRef, error) {
	value, err := parseOpaqueIdentifier("rule reference", raw)
	if err != nil {
		return RuleRef{}, err
	}
	return RuleRef{value: value}, nil
}

func (ref RuleRef) String() string { return ref.value }

func (ref RuleRef) valid() bool { return ref.value != "" }

type RepairPointer struct {
	value string
}

func NewRepairPointer(raw string) (RepairPointer, error) {
	value, err := parseOpaqueIdentifier("repair pointer", raw)
	if err != nil {
		return RepairPointer{}, err
	}
	return RepairPointer{value: value}, nil
}

func (pointer RepairPointer) String() string { return pointer.value }

func (pointer RepairPointer) valid() bool { return pointer.value != "" }

type GraphRevision struct {
	value uint64
}

func NewGraphRevision(value uint64) GraphRevision {
	return GraphRevision{value: value}
}

func (revision GraphRevision) Value() uint64 { return revision.value }

type StrongRef interface {
	RefKind() RefKindRef
	ReferenceKey() string
	strongRefVariant()
}

type PersistedRef struct {
	kind RefKindRef
	id   ReferenceID
}

func NewPersistedRef(kind RefKindRef, id ReferenceID) (PersistedRef, error) {
	if !kind.valid() {
		return PersistedRef{}, fmt.Errorf("persisted reference kind is required")
	}
	if !id.valid() {
		return PersistedRef{}, fmt.Errorf("persisted reference ID is required")
	}
	return PersistedRef{kind: kind, id: id}, nil
}

func (ref PersistedRef) RefKind() RefKindRef { return ref.kind }

func (ref PersistedRef) ReferenceID() ReferenceID { return ref.id }

func (ref PersistedRef) ReferenceKey() string { return "persisted:" + ref.id.String() }

func (PersistedRef) strongRefVariant() {}

type LocalRef struct {
	kind RefKindRef
	ref  BatchLocalRef
}

func NewLocalRef(kind RefKindRef, ref BatchLocalRef) (LocalRef, error) {
	if !kind.valid() {
		return LocalRef{}, fmt.Errorf("batch-local reference kind is required")
	}
	if !ref.valid() {
		return LocalRef{}, fmt.Errorf("batch-local reference is required")
	}
	return LocalRef{kind: kind, ref: ref}, nil
}

func (ref LocalRef) RefKind() RefKindRef { return ref.kind }

func (ref LocalRef) BatchLocalRef() BatchLocalRef { return ref.ref }

func (ref LocalRef) ReferenceKey() string { return "local:" + ref.ref.String() }

func (LocalRef) strongRefVariant() {}

func parseOpaqueIdentifier(name, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, "\r\n\t") {
		return "", fmt.Errorf("%s must be one line", name)
	}
	return value, nil
}

func parseQualifiedIdentifier(name, raw string) (string, error) {
	value, err := parseOpaqueIdentifier(name, raw)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(value, " /\\") {
		return "", fmt.Errorf("%s must not contain whitespace, slash, or backslash", name)
	}
	return value, nil
}
