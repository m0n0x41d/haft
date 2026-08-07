package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type CompilerSchemaVersion struct {
	value string
}

func NewCompilerSchemaVersion(raw string) (CompilerSchemaVersion, error) {
	value, err := parseOpaqueIdentifier("compiler schema version", raw)
	if err != nil {
		return CompilerSchemaVersion{}, err
	}
	return CompilerSchemaVersion{value: value}, nil
}

func (version CompilerSchemaVersion) String() string { return version.value }

func (version CompilerSchemaVersion) valid() bool { return version.value != "" }

type ContextBridgeID struct {
	value string
}

func NewContextBridgeID(raw string) (ContextBridgeID, error) {
	value, err := parseQualifiedIdentifier("context bridge ID", raw)
	if err != nil {
		return ContextBridgeID{}, err
	}
	return ContextBridgeID{value: value}, nil
}

func (id ContextBridgeID) String() string { return id.value }

func (id ContextBridgeID) valid() bool { return id.value != "" }

type ConstraintID struct {
	value string
}

func NewConstraintID(raw string) (ConstraintID, error) {
	value, err := parseQualifiedIdentifier("constraint ID", raw)
	if err != nil {
		return ConstraintID{}, err
	}
	return ConstraintID{value: value}, nil
}

func (id ConstraintID) String() string { return id.value }

func (id ConstraintID) valid() bool { return id.value != "" }

type SchemaSymbolKind uint8

const (
	ContextSymbol SchemaSymbolKind = iota + 1
	KindSymbol
	SlotKindSymbol
	RefKindSymbol
	BridgeSymbol
	SignatureSymbol
	ShapeSymbol
	CodecSymbol
	ConstraintSymbol
	EntitySetSymbol
	KindSignatureSymbol
)

func (kind SchemaSymbolKind) String() string {
	switch kind {
	case ContextSymbol:
		return "context"
	case KindSymbol:
		return "kind"
	case SlotKindSymbol:
		return "slot_kind"
	case RefKindSymbol:
		return "ref_kind"
	case BridgeSymbol:
		return "bridge"
	case SignatureSymbol:
		return "signature"
	case ShapeSymbol:
		return "shape"
	case CodecSymbol:
		return "codec"
	case ConstraintSymbol:
		return "constraint"
	case EntitySetSymbol:
		return "entity_set"
	case KindSignatureSymbol:
		return "kind_signature"
	default:
		return ""
	}
}

func (kind SchemaSymbolKind) valid() bool { return kind.String() != "" }

type SchemaSymbolRef struct {
	kind SchemaSymbolKind
	key  string
}

func BoundedContextSymbolRef(ref BoundedContextRef) (SchemaSymbolRef, error) {
	if !ref.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("bounded-context symbol reference is required")
	}
	return SchemaSymbolRef{kind: ContextSymbol, key: ref.String()}, nil
}

func KindSymbolRef(id KindID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("kind symbol ID is required")
	}
	return SchemaSymbolRef{kind: KindSymbol, key: id.String()}, nil
}

func SlotKindSymbolRef(
	signature SignatureID,
	slotKind SlotKindID,
) (SchemaSymbolRef, error) {
	if !signature.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("SlotKind governing signature is required")
	}
	if !slotKind.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("SlotKind symbol ID is required")
	}
	key := signature.String() + "/slot/" + slotKind.String()
	return SchemaSymbolRef{kind: SlotKindSymbol, key: key}, nil
}

func RefKindSymbolRef(id RefKindID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("RefKind symbol ID is required")
	}
	return SchemaSymbolRef{kind: RefKindSymbol, key: id.String()}, nil
}

func ContextBridgeSymbolRef(id ContextBridgeID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("context bridge symbol ID is required")
	}
	return SchemaSymbolRef{kind: BridgeSymbol, key: id.String()}, nil
}

func RelationSymbolRef(id SignatureID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("relation-signature symbol ID is required")
	}
	return SchemaSymbolRef{kind: SignatureSymbol, key: id.String()}, nil
}

func ValueShapeSymbolRef(id ShapeID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("value-shape symbol ID is required")
	}
	return SchemaSymbolRef{kind: ShapeSymbol, key: id.String()}, nil
}

func CodecSymbolRef(id CodecID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("codec symbol ID is required")
	}
	return SchemaSymbolRef{kind: CodecSymbol, key: id.String()}, nil
}

func ConstraintSymbolRef(id ConstraintID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("constraint symbol ID is required")
	}
	return SchemaSymbolRef{kind: ConstraintSymbol, key: id.String()}, nil
}

func (ref SchemaSymbolRef) Kind() SchemaSymbolKind { return ref.kind }

func (ref SchemaSymbolRef) Key() string { return ref.key }

func (ref SchemaSymbolRef) String() string { return ref.kind.String() + ":" + ref.key }

func (ref SchemaSymbolRef) valid() bool { return ref.kind.valid() && ref.key != "" }

type CoveragePosture uint8

const (
	CoverageCompiled CoveragePosture = iota + 1
	CoverageSourceOnly
	CoverageUnsupported
)

func (posture CoveragePosture) String() string {
	switch posture {
	case CoverageCompiled:
		return "compiled"
	case CoverageSourceOnly:
		return "source_only"
	case CoverageUnsupported:
		return "unsupported"
	default:
		return ""
	}
}

func (posture CoveragePosture) valid() bool { return posture.String() != "" }

type CoverageSubjectKind uint8

const (
	SourceUnitCoverageSubject CoverageSubjectKind = iota + 1
	SchemaSymbolCoverageSubject
)

type CoverageSubject struct {
	kind   CoverageSubjectKind
	unitID SourceUnitID
	symbol SchemaSymbolRef
}

func SourceUnitCoverage(unitID SourceUnitID) (CoverageSubject, error) {
	if !unitID.valid() {
		return CoverageSubject{}, fmt.Errorf("coverage source unit is required")
	}
	return CoverageSubject{kind: SourceUnitCoverageSubject, unitID: unitID}, nil
}

func SchemaSymbolCoverage(symbol SchemaSymbolRef) (CoverageSubject, error) {
	if !symbol.valid() {
		return CoverageSubject{}, fmt.Errorf("coverage schema symbol is required")
	}
	return CoverageSubject{kind: SchemaSymbolCoverageSubject, symbol: symbol}, nil
}

func (subject CoverageSubject) Kind() CoverageSubjectKind { return subject.kind }

func (subject CoverageSubject) SourceUnitID() (SourceUnitID, bool) {
	return subject.unitID, subject.kind == SourceUnitCoverageSubject
}

func (subject CoverageSubject) SchemaSymbol() (SchemaSymbolRef, bool) {
	return subject.symbol, subject.kind == SchemaSymbolCoverageSubject
}

func (subject CoverageSubject) String() string {
	if subject.kind == SourceUnitCoverageSubject {
		return "source-unit:" + subject.unitID.String()
	}
	if subject.kind == SchemaSymbolCoverageSubject {
		return "schema-symbol:" + subject.symbol.String()
	}
	return ""
}

func (subject CoverageSubject) valid() bool {
	if subject.kind == SourceUnitCoverageSubject {
		return subject.unitID.valid()
	}
	if subject.kind == SchemaSymbolCoverageSubject {
		return subject.symbol.valid()
	}
	return false
}

type CoverageEntry struct {
	subject   CoverageSubject
	posture   CoveragePosture
	source    SourceLocation
	rationale string
}

func NewCompiledCoverageEntry(
	subject CoverageSubject,
	source SourceLocation,
) (CoverageEntry, error) {
	return newCoverageEntry(subject, CoverageCompiled, source, "")
}

func NewSourceOnlyCoverageEntry(
	subject CoverageSubject,
	source SourceLocation,
	rationale string,
) (CoverageEntry, error) {
	return newCoverageEntry(subject, CoverageSourceOnly, source, rationale)
}

func NewUnsupportedCoverageEntry(
	subject CoverageSubject,
	source SourceLocation,
	rationale string,
) (CoverageEntry, error) {
	return newCoverageEntry(subject, CoverageUnsupported, source, rationale)
}

func newCoverageEntry(
	subject CoverageSubject,
	posture CoveragePosture,
	source SourceLocation,
	rationale string,
) (CoverageEntry, error) {
	if !subject.valid() {
		return CoverageEntry{}, fmt.Errorf("coverage subject is required")
	}
	if !posture.valid() {
		return CoverageEntry{}, fmt.Errorf("coverage posture is required")
	}
	if !source.valid() {
		return CoverageEntry{}, fmt.Errorf("coverage source location is required")
	}
	parsedRationale := rationale
	if posture != CoverageCompiled {
		value, err := parseOpaqueIdentifier("coverage rationale", rationale)
		if err != nil {
			return CoverageEntry{}, err
		}
		parsedRationale = value
	}
	if posture == CoverageCompiled && rationale != "" {
		return CoverageEntry{}, fmt.Errorf("compiled coverage must not carry a gap rationale")
	}
	return CoverageEntry{
		subject:   subject,
		posture:   posture,
		source:    source,
		rationale: parsedRationale,
	}, nil
}

func (entry CoverageEntry) Subject() CoverageSubject { return entry.subject }

func (entry CoverageEntry) Posture() CoveragePosture { return entry.posture }

func (entry CoverageEntry) Source() SourceLocation { return entry.source }

func (entry CoverageEntry) Rationale() string { return entry.rationale }

func (entry CoverageEntry) valid() bool {
	if !entry.subject.valid() || !entry.posture.valid() || !entry.source.valid() {
		return false
	}
	if entry.posture == CoverageCompiled {
		return entry.rationale == ""
	}
	return entry.rationale != ""
}

type CoverageManifest struct {
	entries []CoverageEntry
}

func NewCoverageManifest(entries []CoverageEntry) (CoverageManifest, error) {
	if len(entries) == 0 {
		return CoverageManifest{}, fmt.Errorf("coverage manifest requires at least one entry")
	}
	copyEntries := append([]CoverageEntry(nil), entries...)
	sort.Slice(copyEntries, func(left, right int) bool {
		return copyEntries[left].subject.String() < copyEntries[right].subject.String()
	})
	for index, entry := range copyEntries {
		if !entry.valid() {
			return CoverageManifest{}, fmt.Errorf("coverage entry %d is invalid", index)
		}
		if index > 0 && entry.subject.String() == copyEntries[index-1].subject.String() {
			return CoverageManifest{}, fmt.Errorf("duplicate coverage subject %q", entry.subject.String())
		}
	}
	return CoverageManifest{entries: copyEntries}, nil
}

func (manifest CoverageManifest) Entries() []CoverageEntry {
	return append([]CoverageEntry(nil), manifest.entries...)
}

func (manifest CoverageManifest) Entry(subject CoverageSubject) (CoverageEntry, bool) {
	key := subject.String()
	index := sort.Search(len(manifest.entries), func(index int) bool {
		return manifest.entries[index].subject.String() >= key
	})
	if index >= len(manifest.entries) || manifest.entries[index].subject.String() != key {
		return CoverageEntry{}, false
	}
	return manifest.entries[index], true
}

func (manifest CoverageManifest) valid() bool { return len(manifest.entries) > 0 }

type KindDefinition struct {
	id         KindID
	provenance DeclarationProvenance
}

func NewKindDefinition(
	id KindID,
	provenance DeclarationProvenance,
) (KindDefinition, error) {
	if !id.valid() {
		return KindDefinition{}, fmt.Errorf("kind definition ID is required")
	}
	if !validDeclarationProvenance(provenance) {
		return KindDefinition{}, fmt.Errorf("kind definition provenance is required")
	}
	return KindDefinition{id: id, provenance: provenance}, nil
}

func (definition KindDefinition) ID() KindID { return definition.id }

func (definition KindDefinition) Provenance() DeclarationProvenance {
	return definition.provenance
}

func (definition KindDefinition) valid() bool {
	return definition.id.valid() &&
		validDeclarationProvenance(definition.provenance)
}

// SignatureFormality is the F0..F9 rigor declared by a C.3.2
// KindSignature. It describes the signature content; it is neither claim
// formality nor evidence strength.
type SignatureFormality uint8

const (
	SignatureF0 SignatureFormality = iota + 1
	SignatureF1
	SignatureF2
	SignatureF3
	SignatureF4
	SignatureF5
	SignatureF6
	SignatureF7
	SignatureF8
	SignatureF9
)

func NewSignatureFormality(level uint8) (SignatureFormality, error) {
	if level > 9 {
		return SignatureFormality(0), fmt.Errorf("signature formality must be in F0..F9")
	}
	return SignatureFormality(level + 1), nil
}

func (formality SignatureFormality) String() string {
	if formality < SignatureF0 || formality > SignatureF9 {
		return ""
	}
	return fmt.Sprintf("F%d", uint8(formality-SignatureF0))
}

func (formality SignatureFormality) valid() bool {
	return formality >= SignatureF0 && formality <= SignatureF9
}

// KindAssumptionPin names one exact external assumption consumed by a
// KindSignature. Exact edition and bytes prevent a hidden "current" or
// "latest" dependency from entering MemberOf.
type KindAssumptionPin struct {
	versioned VersionedPin
}

func NewKindAssumptionPin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (KindAssumptionPin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return KindAssumptionPin{}, fmt.Errorf("kind-signature assumption: %w", err)
	}
	return KindAssumptionPin{versioned: pin}, nil
}

func (pin KindAssumptionPin) Reference() CarrierRef { return pin.versioned.reference }

func (pin KindAssumptionPin) Edition() CarrierEdition { return pin.versioned.edition }

func (pin KindAssumptionPin) Digest() SHA256Digest { return pin.versioned.digest }

func (pin KindAssumptionPin) CanonicalBytes() []byte {
	return pin.versioned.canonicalBytes("kind-signature-assumption.v1")
}

func (pin KindAssumptionPin) valid() bool { return pin.versioned.valid() }

type EntitySetDefinitionRef struct {
	typeEnv TypeEnvRef
	context BoundedContextRef
	digest  SHA256Digest
}

func NewEntitySetDefinitionRef(
	typeEnv TypeEnvRef,
	context BoundedContextRef,
	digest SHA256Digest,
) (EntitySetDefinitionRef, error) {
	if !typeEnv.valid() {
		return EntitySetDefinitionRef{}, fmt.Errorf("EntitySet definition TypeEnv is required")
	}
	if !context.valid() {
		return EntitySetDefinitionRef{}, fmt.Errorf("EntitySet definition context is required")
	}
	if !digest.valid() {
		return EntitySetDefinitionRef{}, fmt.Errorf("EntitySet definition digest is required")
	}
	return EntitySetDefinitionRef{
		typeEnv: typeEnv,
		context: context,
		digest:  digest,
	}, nil
}

func (ref EntitySetDefinitionRef) TypeEnv() TypeEnvRef { return ref.typeEnv }

func (ref EntitySetDefinitionRef) Context() BoundedContextRef { return ref.context }

func (ref EntitySetDefinitionRef) Digest() SHA256Digest { return ref.digest }

func (ref EntitySetDefinitionRef) String() string {
	return ref.typeEnv.String() +
		"/entity-set/" + ref.context.String() +
		"/" + ref.digest.String()
}

func (ref EntitySetDefinitionRef) valid() bool {
	return ref.typeEnv.valid() && ref.context.valid() && ref.digest.valid()
}

type EntitySetDefinitionInput struct {
	TypeEnv         TypeEnvRef
	Context         BoundedContextRef
	EnumerationRule RuleRef
	CandidatePolicy EntitySetCandidatePolicy
	Provenance      DeclarationProvenance
}

// EntitySetCandidatePolicy is the closed policy for whether an evaluator may
// include request-local declarations in U.EntitySet(slice). Persisted-only is
// deliberately distinct from a prospective view: a same-batch declaration
// never expands an EntitySet implicitly.
type EntitySetCandidatePolicy interface {
	AllowsPriorBatchDeclarations() bool
	CanonicalBytes() []byte
	entitySetCandidatePolicyVariant()
}

// PersistedEntitiesOnly admits only entities addressable in the pre-state
// snapshot. It is the conservative policy for source-derived TypeEnvs.
type PersistedEntitiesOnly struct{}

func (PersistedEntitiesOnly) AllowsPriorBatchDeclarations() bool { return false }

func (PersistedEntitiesOnly) CanonicalBytes() []byte {
	writer := newCanonicalWriter("entity-set-candidate-policy.persisted-only.v1")
	return writer.bytes()
}

func (PersistedEntitiesOnly) entitySetCandidatePolicyVariant() {}

// PriorBatchDeclarationsVisible explicitly permits an EntitySet evaluator to
// consume exact DeclareEntity candidate bytes from a prospective prefix. The
// separate rule is the executable policy for that candidate projection; the
// ordinary EnumerationRule continues to govern the persisted universe.
type PriorBatchDeclarationsVisible struct {
	evaluationRule RuleRef
}

func NewPriorBatchDeclarationsVisible(
	evaluationRule RuleRef,
) (PriorBatchDeclarationsVisible, error) {
	if !evaluationRule.valid() {
		return PriorBatchDeclarationsVisible{}, fmt.Errorf("prospective EntitySet evaluation rule is required")
	}
	return PriorBatchDeclarationsVisible{evaluationRule: evaluationRule}, nil
}

func (PriorBatchDeclarationsVisible) AllowsPriorBatchDeclarations() bool { return true }

func (policy PriorBatchDeclarationsVisible) EvaluationRule() RuleRef {
	return policy.evaluationRule
}

func (policy PriorBatchDeclarationsVisible) CanonicalBytes() []byte {
	writer := newCanonicalWriter("entity-set-candidate-policy.prior-declarations.v1")
	writer.addString(policy.evaluationRule.String())
	return writer.bytes()
}

func (PriorBatchDeclarationsVisible) entitySetCandidatePolicyVariant() {}

func validEntitySetCandidatePolicy(policy EntitySetCandidatePolicy) bool {
	switch value := policy.(type) {
	case PersistedEntitiesOnly:
		return len(value.CanonicalBytes()) > 0
	case PriorBatchDeclarationsVisible:
		return value.evaluationRule.valid() && len(value.CanonicalBytes()) > 0
	default:
		return false
	}
}

// EntitySetDefinition makes the universe U.EntitySet(slice) addressable for
// one bounded context. The concrete slice and its observable inputs remain
// evaluation-time values; this declaration only fixes the rule that resolves
// that universe.
type EntitySetDefinition struct {
	reference       EntitySetDefinitionRef
	enumerationRule RuleRef
	candidatePolicy EntitySetCandidatePolicy
	provenance      DeclarationProvenance
	canonicalBytes  []byte
}

func NewEntitySetDefinition(
	input EntitySetDefinitionInput,
) (EntitySetDefinition, error) {
	if !input.TypeEnv.valid() {
		return EntitySetDefinition{}, fmt.Errorf("EntitySet definition TypeEnv is required")
	}
	if !input.Context.valid() {
		return EntitySetDefinition{}, fmt.Errorf("EntitySet definition context is required")
	}
	if !input.EnumerationRule.valid() {
		return EntitySetDefinition{}, fmt.Errorf("EntitySet enumeration rule is required")
	}
	if !validEntitySetCandidatePolicy(input.CandidatePolicy) {
		return EntitySetDefinition{}, fmt.Errorf("EntitySet candidate visibility policy is required")
	}
	if !validDeclarationProvenance(input.Provenance) {
		return EntitySetDefinition{}, fmt.Errorf("EntitySet definition provenance is required")
	}
	writer := newCanonicalWriter("entity-set-definition.v2")
	writer.addString(input.TypeEnv.String())
	writer.addString(input.Context.String())
	writer.addString(input.EnumerationRule.String())
	writer.addBytes(input.CandidatePolicy.CanonicalBytes())
	writer.addBytes(input.Provenance.CanonicalBytes())
	reference, err := NewEntitySetDefinitionRef(
		input.TypeEnv,
		input.Context,
		writer.digest(),
	)
	if err != nil {
		return EntitySetDefinition{}, err
	}
	return EntitySetDefinition{
		reference:       reference,
		enumerationRule: input.EnumerationRule,
		candidatePolicy: input.CandidatePolicy,
		provenance:      input.Provenance,
		canonicalBytes:  writer.bytes(),
	}, nil
}

func (definition EntitySetDefinition) Ref() EntitySetDefinitionRef {
	return definition.reference
}

func (definition EntitySetDefinition) EnumerationRule() RuleRef {
	return definition.enumerationRule
}

func (definition EntitySetDefinition) CandidatePolicy() EntitySetCandidatePolicy {
	return definition.candidatePolicy
}

func (definition EntitySetDefinition) Provenance() DeclarationProvenance {
	return definition.provenance
}

func (definition EntitySetDefinition) CanonicalBytes() []byte {
	return append([]byte(nil), definition.canonicalBytes...)
}

func (definition EntitySetDefinition) valid() bool {
	if !definition.reference.valid() ||
		!definition.enumerationRule.valid() ||
		!validEntitySetCandidatePolicy(definition.candidatePolicy) ||
		!validDeclarationProvenance(definition.provenance) ||
		len(definition.canonicalBytes) == 0 {
		return false
	}
	writer := newCanonicalWriter("entity-set-definition.v2")
	writer.addString(definition.reference.TypeEnv().String())
	writer.addString(definition.reference.Context().String())
	writer.addString(definition.enumerationRule.String())
	writer.addBytes(definition.candidatePolicy.CanonicalBytes())
	writer.addBytes(definition.provenance.CanonicalBytes())
	return writer.digest() == definition.reference.Digest() &&
		bytes.Equal(writer.bytes(), definition.canonicalBytes)
}

type KindSignatureRef struct {
	kind    ValueKindRef
	context BoundedContextRef
	digest  SHA256Digest
}

func NewKindSignatureRef(
	kind ValueKindRef,
	context BoundedContextRef,
	digest SHA256Digest,
) (KindSignatureRef, error) {
	if !kind.valid() {
		return KindSignatureRef{}, fmt.Errorf("KindSignature ValueKind is required")
	}
	if !context.valid() {
		return KindSignatureRef{}, fmt.Errorf("KindSignature bounded context is required")
	}
	if !digest.valid() {
		return KindSignatureRef{}, fmt.Errorf("KindSignature digest is required")
	}
	return KindSignatureRef{kind: kind, context: context, digest: digest}, nil
}

func (ref KindSignatureRef) ValueKind() ValueKindRef { return ref.kind }

func (ref KindSignatureRef) TypeEnv() TypeEnvRef { return ref.kind.TypeEnv() }

func (ref KindSignatureRef) Context() BoundedContextRef { return ref.context }

func (ref KindSignatureRef) Digest() SHA256Digest { return ref.digest }

func (ref KindSignatureRef) String() string {
	return ref.kind.String() +
		"/context/" + ref.context.String() +
		"/signature/" + ref.digest.String()
}

func (ref KindSignatureRef) valid() bool {
	return ref.kind.valid() && ref.context.valid() && ref.digest.valid()
}

func (ref KindSignatureRef) key() string {
	return exactTupleKey(
		"kind-signature",
		ref.kind.String(),
		ref.context.String(),
	)
}

type KindSignatureDefinitionInput struct {
	ValueKind       ValueKindRef
	Formality       SignatureFormality
	Assumptions     []KindAssumptionPin
	DefinednessRule RuleRef
	Evaluator       RuleRef
	EntitySet       EntitySetDefinitionRef
	Provenance      DeclarationProvenance
}

// KindSignatureDefinition is the executable C.3.2 intension of one U.Kind in
// one bounded context. Its EntitySet, definedness rule, and evaluator are
// explicit and are deliberately separate from ContextKindAvailability, which
// only says that the kind vocabulary is available in that context.
type KindSignatureDefinition struct {
	reference       KindSignatureRef
	formality       SignatureFormality
	assumptions     []KindAssumptionPin
	definednessRule RuleRef
	evaluator       RuleRef
	entitySet       EntitySetDefinitionRef
	provenance      DeclarationProvenance
	canonicalBytes  []byte
}

func NewKindSignatureDefinition(
	input KindSignatureDefinitionInput,
) (KindSignatureDefinition, error) {
	if !input.ValueKind.valid() {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature ValueKind is required")
	}
	if !input.Formality.valid() {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature formality must be in F0..F9")
	}
	if !input.Evaluator.valid() {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature evaluator rule is required")
	}
	if !input.DefinednessRule.valid() {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature definedness rule is required")
	}
	if !input.EntitySet.valid() {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature EntitySet definition is required")
	}
	if input.EntitySet.TypeEnv() != input.ValueKind.TypeEnv() {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature and EntitySet must belong to one TypeEnv")
	}
	if !validDeclarationProvenance(input.Provenance) {
		return KindSignatureDefinition{}, fmt.Errorf("KindSignature provenance is required")
	}
	assumptions, err := normalizeKindAssumptions(input.Assumptions)
	if err != nil {
		return KindSignatureDefinition{}, err
	}
	writer := canonicalKindSignatureDefinition(
		input.ValueKind,
		input.Formality,
		assumptions,
		input.DefinednessRule,
		input.Evaluator,
		input.EntitySet,
		input.Provenance,
	)
	reference, err := NewKindSignatureRef(
		input.ValueKind,
		input.EntitySet.Context(),
		writer.digest(),
	)
	if err != nil {
		return KindSignatureDefinition{}, err
	}
	return KindSignatureDefinition{
		reference:       reference,
		formality:       input.Formality,
		assumptions:     assumptions,
		definednessRule: input.DefinednessRule,
		evaluator:       input.Evaluator,
		entitySet:       input.EntitySet,
		provenance:      input.Provenance,
		canonicalBytes:  writer.bytes(),
	}, nil
}

func (definition KindSignatureDefinition) Ref() KindSignatureRef {
	return definition.reference
}

func (definition KindSignatureDefinition) ValueKind() ValueKindRef {
	return definition.reference.ValueKind()
}

func (definition KindSignatureDefinition) Formality() SignatureFormality {
	return definition.formality
}

func (definition KindSignatureDefinition) Assumptions() []KindAssumptionPin {
	return append([]KindAssumptionPin(nil), definition.assumptions...)
}

func (definition KindSignatureDefinition) DefinednessRule() RuleRef {
	return definition.definednessRule
}

func (definition KindSignatureDefinition) Evaluator() RuleRef {
	return definition.evaluator
}

func (definition KindSignatureDefinition) EntitySet() EntitySetDefinitionRef {
	return definition.entitySet
}

func (definition KindSignatureDefinition) Provenance() DeclarationProvenance {
	return definition.provenance
}

func (definition KindSignatureDefinition) CanonicalBytes() []byte {
	return append([]byte(nil), definition.canonicalBytes...)
}

func (definition KindSignatureDefinition) valid() bool {
	if !definition.reference.valid() ||
		!definition.formality.valid() ||
		!definition.definednessRule.valid() ||
		!definition.evaluator.valid() ||
		!definition.entitySet.valid() ||
		definition.reference.TypeEnv() != definition.entitySet.TypeEnv() ||
		definition.reference.Context() != definition.entitySet.Context() ||
		!validDeclarationProvenance(definition.provenance) ||
		len(definition.canonicalBytes) == 0 {
		return false
	}
	assumptions, err := normalizeKindAssumptions(definition.assumptions)
	if err != nil || len(assumptions) != len(definition.assumptions) {
		return false
	}
	writer := canonicalKindSignatureDefinition(
		definition.reference.ValueKind(),
		definition.formality,
		assumptions,
		definition.definednessRule,
		definition.evaluator,
		definition.entitySet,
		definition.provenance,
	)
	return writer.digest() == definition.reference.Digest() &&
		bytes.Equal(writer.bytes(), definition.canonicalBytes)
}

func canonicalKindSignatureDefinition(
	kind ValueKindRef,
	formality SignatureFormality,
	assumptions []KindAssumptionPin,
	definednessRule RuleRef,
	evaluator RuleRef,
	entitySet EntitySetDefinitionRef,
	provenance DeclarationProvenance,
) canonicalWriter {
	writer := newCanonicalWriter("kind-signature-definition.v1")
	writer.addString(kind.String())
	writer.addString(formality.String())
	writer.addUint64(uint64(len(assumptions)))
	for _, assumption := range assumptions {
		writer.addBytes(assumption.CanonicalBytes())
	}
	writer.addString(definednessRule.String())
	writer.addString(evaluator.String())
	writer.addString(entitySet.String())
	writer.addBytes(provenance.CanonicalBytes())
	return writer
}

func normalizeKindAssumptions(
	values []KindAssumptionPin,
) ([]KindAssumptionPin, error) {
	result := append([]KindAssumptionPin(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		leftRef := result[left].Reference().String()
		rightRef := result[right].Reference().String()
		if leftRef != rightRef {
			return leftRef < rightRef
		}
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]KindAssumptionPin, 0, len(result))
	for _, assumption := range result {
		if !assumption.valid() {
			return nil, fmt.Errorf("KindSignature assumption is invalid")
		}
		if len(normalized) == 0 {
			normalized = append(normalized, assumption)
			continue
		}
		previous := normalized[len(normalized)-1]
		if previous.Reference() != assumption.Reference() {
			normalized = append(normalized, assumption)
			continue
		}
		if bytes.Equal(previous.CanonicalBytes(), assumption.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"KindSignature assumption %q has conflicting exact pins",
			assumption.Reference().String(),
		)
	}
	return normalized, nil
}

// RefKindDefinition keeps reference identity distinct from the U.Kind of its
// referent. A RefKind is never itself accepted as a ValueKind.
type RefKindDefinition struct {
	ref        RefKindRef
	valueKind  ValueKindRef
	provenance DeclarationProvenance
}

func NewRefKindDefinition(
	ref RefKindRef,
	valueKind ValueKindRef,
	provenance DeclarationProvenance,
) (RefKindDefinition, error) {
	if !ref.valid() {
		return RefKindDefinition{}, fmt.Errorf("RefKind definition reference is required")
	}
	if !valueKind.valid() {
		return RefKindDefinition{}, fmt.Errorf("RefKind referent ValueKind is required")
	}
	if ref.TypeEnv() != valueKind.TypeEnv() {
		return RefKindDefinition{}, fmt.Errorf("RefKind and referent ValueKind must belong to one TypeEnv")
	}
	if !validDeclarationProvenance(provenance) {
		return RefKindDefinition{}, fmt.Errorf("RefKind definition provenance is required")
	}
	return RefKindDefinition{ref: ref, valueKind: valueKind, provenance: provenance}, nil
}

func (definition RefKindDefinition) Ref() RefKindRef { return definition.ref }

func (definition RefKindDefinition) ValueKind() ValueKindRef { return definition.valueKind }

func (definition RefKindDefinition) Provenance() DeclarationProvenance {
	return definition.provenance
}

func (definition RefKindDefinition) valid() bool {
	return definition.ref.valid() &&
		definition.valueKind.valid() &&
		definition.ref.TypeEnv() == definition.valueKind.TypeEnv() &&
		validDeclarationProvenance(definition.provenance)
}

type BoundedContext struct {
	ref        BoundedContextRef
	provenance DeclarationProvenance
}

func NewBoundedContext(
	ref BoundedContextRef,
	provenance DeclarationProvenance,
) (BoundedContext, error) {
	if !ref.valid() {
		return BoundedContext{}, fmt.Errorf("bounded-context reference is required")
	}
	if !validDeclarationProvenance(provenance) {
		return BoundedContext{}, fmt.Errorf("bounded-context provenance is required")
	}
	return BoundedContext{ref: ref, provenance: provenance}, nil
}

func (context BoundedContext) Ref() BoundedContextRef { return context.ref }

func (context BoundedContext) Provenance() DeclarationProvenance {
	return context.provenance
}

func (context BoundedContext) valid() bool {
	return context.ref.valid() && validDeclarationProvenance(context.provenance)
}

type SubkindRelation struct {
	subkind    KindID
	superkind  KindID
	provenance DeclarationProvenance
}

func NewSubkindRelation(
	subkind KindID,
	superkind KindID,
	provenance DeclarationProvenance,
) (SubkindRelation, error) {
	if !subkind.valid() || !superkind.valid() {
		return SubkindRelation{}, fmt.Errorf("subkind and superkind IDs are required")
	}
	if subkind == superkind {
		return SubkindRelation{}, fmt.Errorf("a kind cannot be its own direct superkind")
	}
	if !validDeclarationProvenance(provenance) {
		return SubkindRelation{}, fmt.Errorf("subkind-relation provenance is required")
	}
	return SubkindRelation{
		subkind:    subkind,
		superkind:  superkind,
		provenance: provenance,
	}, nil
}

func (relation SubkindRelation) Subkind() KindID { return relation.subkind }

func (relation SubkindRelation) Superkind() KindID { return relation.superkind }

func (relation SubkindRelation) Provenance() DeclarationProvenance {
	return relation.provenance
}

func (relation SubkindRelation) key() string {
	return exactTupleKey(
		"subkind-relation",
		relation.subkind.String(),
		relation.superkind.String(),
	)
}

func (relation SubkindRelation) valid() bool {
	return relation.subkind.valid() &&
		relation.superkind.valid() &&
		relation.subkind != relation.superkind &&
		validDeclarationProvenance(relation.provenance)
}

type CardinalityLimit interface {
	BoundedValue() (uint64, bool)
	cardinalityLimitVariant()
}

type FiniteCardinalityLimit struct {
	value uint64
}

func NewFiniteCardinalityLimit(value uint64) FiniteCardinalityLimit {
	return FiniteCardinalityLimit{value: value}
}

func (limit FiniteCardinalityLimit) BoundedValue() (uint64, bool) {
	return limit.value, true
}

func (FiniteCardinalityLimit) cardinalityLimitVariant() {}

type UnboundedCardinalityLimit struct{}

func (UnboundedCardinalityLimit) BoundedValue() (uint64, bool) { return 0, false }

func (UnboundedCardinalityLimit) cardinalityLimitVariant() {}

type Cardinality struct {
	minimum uint64
	maximum CardinalityLimit
}

func NewBoundedCardinality(minimum, maximum uint64) (Cardinality, error) {
	if maximum < minimum {
		return Cardinality{}, fmt.Errorf("cardinality maximum must not be less than minimum")
	}
	return Cardinality{
		minimum: minimum,
		maximum: NewFiniteCardinalityLimit(maximum),
	}, nil
}

func NewUnboundedCardinality(minimum uint64) Cardinality {
	return Cardinality{minimum: minimum, maximum: UnboundedCardinalityLimit{}}
}

func ExactlyOneCardinality() Cardinality {
	cardinality, _ := NewBoundedCardinality(1, 1)
	return cardinality
}

func (cardinality Cardinality) Minimum() uint64 { return cardinality.minimum }

func (cardinality Cardinality) Maximum() CardinalityLimit { return cardinality.maximum }

func (cardinality Cardinality) Allows(count uint64) bool {
	if count < cardinality.minimum {
		return false
	}
	maximum, bounded := cardinality.maximum.BoundedValue()
	if !bounded {
		return true
	}
	return count <= maximum
}

func (cardinality Cardinality) valid() bool {
	switch maximum := cardinality.maximum.(type) {
	case FiniteCardinalityLimit:
		return maximum.value >= cardinality.minimum
	case UnboundedCardinalityLimit:
		return true
	default:
		return false
	}
}

type SlotRefMode uint8

const (
	SlotByValue SlotRefMode = iota + 1
	SlotByReference
)

func (mode SlotRefMode) String() string {
	switch mode {
	case SlotByValue:
		return "by_value"
	case SlotByReference:
		return "by_reference"
	default:
		return ""
	}
}

type SlotTarget interface {
	RefMode() SlotRefMode
	CanonicalKey() string
	slotTargetVariant()
}

type ValueSlotTarget struct {
	kind ValueKindRef
}

func NewValueSlotTarget(kind ValueKindRef) (ValueSlotTarget, error) {
	if !kind.valid() {
		return ValueSlotTarget{}, fmt.Errorf("value slot target kind is required")
	}
	return ValueSlotTarget{kind: kind}, nil
}

func (target ValueSlotTarget) ValueKind() ValueKindRef { return target.kind }

func (ValueSlotTarget) RefMode() SlotRefMode { return SlotByValue }

func (target ValueSlotTarget) CanonicalKey() string { return target.kind.String() }

func (ValueSlotTarget) slotTargetVariant() {}

type ReferenceSlotTarget struct {
	valueKind     ValueKindRef
	referenceKind RefKindRef
}

func NewReferenceSlotTarget(
	valueKind ValueKindRef,
	referenceKind RefKindRef,
) (ReferenceSlotTarget, error) {
	if !valueKind.valid() {
		return ReferenceSlotTarget{}, fmt.Errorf("reference slot ValueKind is required")
	}
	if !referenceKind.valid() {
		return ReferenceSlotTarget{}, fmt.Errorf("reference slot target kind is required")
	}
	if valueKind.TypeEnv() != referenceKind.TypeEnv() {
		return ReferenceSlotTarget{}, fmt.Errorf("reference slot ValueKind and RefKind must belong to one TypeEnv")
	}
	return ReferenceSlotTarget{
		valueKind:     valueKind,
		referenceKind: referenceKind,
	}, nil
}

func (target ReferenceSlotTarget) ValueKind() ValueKindRef { return target.valueKind }

func (target ReferenceSlotTarget) ReferenceKind() RefKindRef { return target.referenceKind }

func (ReferenceSlotTarget) RefMode() SlotRefMode { return SlotByReference }

func (target ReferenceSlotTarget) CanonicalKey() string {
	return target.valueKind.String() + "/via/" + target.referenceKind.String()
}

func (ReferenceSlotTarget) slotTargetVariant() {}

func validSlotTarget(target SlotTarget) bool {
	switch value := target.(type) {
	case ValueSlotTarget:
		return value.kind.valid()
	case ReferenceSlotTarget:
		return value.valueKind.valid() &&
			value.referenceKind.valid() &&
			value.valueKind.TypeEnv() == value.referenceKind.TypeEnv()
	default:
		return false
	}
}

type SlotSpec struct {
	slotKind    SlotKindID
	target      SlotTarget
	cardinality Cardinality
	provenance  DeclarationProvenance
}

func NewSlotSpec(
	slotKind SlotKindID,
	target SlotTarget,
	cardinality Cardinality,
	provenance DeclarationProvenance,
) (SlotSpec, error) {
	if !slotKind.valid() {
		return SlotSpec{}, fmt.Errorf("SlotKind is required")
	}
	if !validSlotTarget(target) {
		return SlotSpec{}, fmt.Errorf("slot target is required")
	}
	if !cardinality.valid() {
		return SlotSpec{}, fmt.Errorf("slot cardinality is invalid")
	}
	if !validDeclarationProvenance(provenance) {
		return SlotSpec{}, fmt.Errorf("slot provenance is required")
	}
	return SlotSpec{
		slotKind:    slotKind,
		target:      target,
		cardinality: cardinality,
		provenance:  provenance,
	}, nil
}

func (slot SlotSpec) SlotKind() SlotKindID { return slot.slotKind }

func (slot SlotSpec) Target() SlotTarget { return slot.target }

func (slot SlotSpec) RefMode() SlotRefMode { return slot.target.RefMode() }

func (slot SlotSpec) Cardinality() Cardinality { return slot.cardinality }

func (slot SlotSpec) Provenance() DeclarationProvenance { return slot.provenance }

func (slot SlotSpec) valid() bool {
	return slot.slotKind.valid() &&
		validSlotTarget(slot.target) &&
		slot.cardinality.valid() &&
		validDeclarationProvenance(slot.provenance)
}

type RelationDeclarationPosture string

const (
	// RelationDeclarationTypedFragment means that the declaration can support
	// only the exact local structural checks represented by its Contexts,
	// SlotSpecs, cardinalities, constraints, and provenance. It is not a full
	// FPF RelationSignature and carries no direct predicate, laws,
	// applicability, occurrence-identity rule, or declaration-episteme basis.
	RelationDeclarationTypedFragment RelationDeclarationPosture = "typed_relation_declaration_fragment"
)

func (posture RelationDeclarationPosture) String() string {
	return string(posture)
}

func (posture RelationDeclarationPosture) valid() bool {
	return posture == RelationDeclarationTypedFragment
}

// TypedRelationDeclarationFragment is the canonical Haft-local relation
// declaration shape. Its closed field set intentionally makes a complete FPF
// RelationSignature inexpressible: direct predicate/laws, applicability,
// occurrence identity, and the exact U.Signature/C.2.1 episteme basis do not
// exist in this type. Validation may therefore use it only for the declared
// local structural assertion checks.
type TypedRelationDeclarationFragment struct {
	ref        TypedRelationDeclarationFragmentRef
	contexts   []BoundedContextRef
	slots      []SlotSpec
	provenance DeclarationProvenance
}

func NewTypedRelationDeclarationFragment(
	ref TypedRelationDeclarationFragmentRef,
	contexts []BoundedContextRef,
	slots []SlotSpec,
	provenance DeclarationProvenance,
) (TypedRelationDeclarationFragment, error) {
	if !ref.valid() {
		return TypedRelationDeclarationFragment{}, fmt.Errorf(
			"typed relation declaration fragment reference is required",
		)
	}
	if len(contexts) == 0 {
		return TypedRelationDeclarationFragment{}, fmt.Errorf(
			"typed relation declaration fragment requires a bounded context",
		)
	}
	if len(slots) == 0 {
		return TypedRelationDeclarationFragment{}, fmt.Errorf(
			"typed relation declaration fragment requires at least one named slot",
		)
	}
	if !validDeclarationProvenance(provenance) {
		return TypedRelationDeclarationFragment{}, fmt.Errorf(
			"typed relation declaration fragment provenance is required",
		)
	}
	copyContexts := append([]BoundedContextRef(nil), contexts...)
	sort.Slice(copyContexts, func(left, right int) bool {
		return copyContexts[left].String() < copyContexts[right].String()
	})
	for index, context := range copyContexts {
		if !context.valid() {
			return TypedRelationDeclarationFragment{}, fmt.Errorf(
				"typed relation declaration fragment context %d is invalid",
				index,
			)
		}
		if index > 0 && context == copyContexts[index-1] {
			return TypedRelationDeclarationFragment{}, fmt.Errorf(
				"duplicate typed relation declaration fragment context %q",
				context.String(),
			)
		}
	}
	copySlots := append([]SlotSpec(nil), slots...)
	sort.Slice(copySlots, func(left, right int) bool {
		return copySlots[left].slotKind.String() < copySlots[right].slotKind.String()
	})
	for index, slot := range copySlots {
		if !slot.valid() {
			return TypedRelationDeclarationFragment{}, fmt.Errorf(
				"typed relation declaration fragment slot %d is invalid",
				index,
			)
		}
		if index > 0 && slot.slotKind == copySlots[index-1].slotKind {
			return TypedRelationDeclarationFragment{}, fmt.Errorf(
				"duplicate typed relation declaration fragment slot %q",
				slot.slotKind.String(),
			)
		}
	}
	return TypedRelationDeclarationFragment{
		ref:        ref,
		contexts:   copyContexts,
		slots:      copySlots,
		provenance: provenance,
	}, nil
}

func (fragment TypedRelationDeclarationFragment) Ref() TypedRelationDeclarationFragmentRef {
	return fragment.ref
}

func (fragment TypedRelationDeclarationFragment) Contexts() []BoundedContextRef {
	return append([]BoundedContextRef(nil), fragment.contexts...)
}

func (fragment TypedRelationDeclarationFragment) Slots() []SlotSpec {
	return append([]SlotSpec(nil), fragment.slots...)
}

func (fragment TypedRelationDeclarationFragment) Provenance() DeclarationProvenance {
	return fragment.provenance
}

func (fragment TypedRelationDeclarationFragment) Posture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (fragment TypedRelationDeclarationFragment) Slot(
	slotKind SlotKindID,
) (SlotSpec, bool) {
	index := sort.Search(len(fragment.slots), func(index int) bool {
		return fragment.slots[index].slotKind.String() >= slotKind.String()
	})
	if index >= len(fragment.slots) || fragment.slots[index].slotKind != slotKind {
		return SlotSpec{}, false
	}
	return fragment.slots[index], true
}

func (fragment TypedRelationDeclarationFragment) valid() bool {
	return fragment.ref.valid() &&
		len(fragment.contexts) > 0 &&
		len(fragment.slots) > 0 &&
		fragment.Posture().valid() &&
		validDeclarationProvenance(fragment.provenance)
}

// RelationSignature is the historical Go spelling retained for exact
// edition-bound replay. It aliases the structurally limited current type and
// must not be used to claim a complete FPF RelationSignature episteme.
type RelationSignature = TypedRelationDeclarationFragment

// NewRelationSignature preserves the historical constructor spelling for
// edition/wire compatibility. New code should call
// NewTypedRelationDeclarationFragment.
func NewRelationSignature(
	ref RelationSignatureRef,
	contexts []BoundedContextRef,
	slots []SlotSpec,
	provenance DeclarationProvenance,
) (RelationSignature, error) {
	return NewTypedRelationDeclarationFragment(ref, contexts, slots, provenance)
}

type ValueBinding struct {
	valueKind  ValueKindRef
	valueShape ValueShapeRef
	codec      CodecRef
	provenance DeclarationProvenance
}

// ValueShapeDeclaration keeps the closed shape definition in the immutable
// environment. The digest-bearing ValueShapeRef remains its identity.
type ValueShapeDeclaration struct {
	ref        ValueShapeRef
	shape      ValueShape
	provenance DeclarationProvenance
}

func NewValueShapeDeclaration(
	ref ValueShapeRef,
	shape ValueShape,
	provenance DeclarationProvenance,
) (ValueShapeDeclaration, error) {
	if !ref.valid() {
		return ValueShapeDeclaration{}, fmt.Errorf("value-shape declaration reference is required")
	}
	if !validValueShapeDeclaration(shape) {
		return ValueShapeDeclaration{}, fmt.Errorf("value-shape declaration is outside the closed algebra")
	}
	if !validDeclarationProvenance(provenance) {
		return ValueShapeDeclaration{}, fmt.Errorf("value-shape declaration provenance is required")
	}
	if err := VerifyValueShapeRef(ref, shape); err != nil {
		return ValueShapeDeclaration{}, fmt.Errorf(
			"value-shape declaration identity: %w",
			err,
		)
	}
	return ValueShapeDeclaration{ref: ref, shape: shape, provenance: provenance}, nil
}

func (declaration ValueShapeDeclaration) Ref() ValueShapeRef { return declaration.ref }

func (declaration ValueShapeDeclaration) Shape() ValueShape { return declaration.shape }

func (declaration ValueShapeDeclaration) Provenance() DeclarationProvenance {
	return declaration.provenance
}

func (declaration ValueShapeDeclaration) valid() bool {
	if !declaration.ref.valid() ||
		!validValueShapeDeclaration(declaration.shape) ||
		!validDeclarationProvenance(declaration.provenance) {
		return false
	}
	return VerifyValueShapeRef(declaration.ref, declaration.shape) == nil
}

func validValueShapeDeclaration(shape ValueShape) bool {
	switch value := shape.(type) {
	case scalarValueShape:
		return value.scalarKind.valid()
	case recordValueShape:
		return len(value.fields) > 0
	case sumValueShape:
		return len(value.variants) > 0
	case orderedSequenceValueShape:
		return value.element.valid()
	case unorderedSetValueShape:
		return value.element.valid()
	case claimGraphValueShape:
		return true
	default:
		return false
	}
}

func NewValueBinding(
	valueKind ValueKindRef,
	valueShape ValueShapeRef,
	codec CodecRef,
	provenance DeclarationProvenance,
) (ValueBinding, error) {
	if !valueKind.valid() {
		return ValueBinding{}, fmt.Errorf("value binding kind is required")
	}
	if !valueShape.valid() {
		return ValueBinding{}, fmt.Errorf("value binding shape is required")
	}
	if !codec.valid() {
		return ValueBinding{}, fmt.Errorf("value binding codec is required")
	}
	if !validDeclarationProvenance(provenance) {
		return ValueBinding{}, fmt.Errorf("value binding provenance is required")
	}
	return ValueBinding{
		valueKind:  valueKind,
		valueShape: valueShape,
		codec:      codec,
		provenance: provenance,
	}, nil
}

func (binding ValueBinding) ValueKind() ValueKindRef { return binding.valueKind }

func (binding ValueBinding) ValueShape() ValueShapeRef { return binding.valueShape }

func (binding ValueBinding) Codec() CodecRef { return binding.codec }

func (binding ValueBinding) Provenance() DeclarationProvenance { return binding.provenance }

func (binding ValueBinding) valid() bool {
	return binding.valueKind.valid() &&
		binding.valueShape.valid() &&
		binding.codec.valid() &&
		validDeclarationProvenance(binding.provenance)
}

type ConstraintRule interface {
	ID() ConstraintID
	Provenance() DeclarationProvenance
	CanonicalBytes() []byte
	constraintRuleVariant()
}

type KindDisjointConstraint struct {
	id         ConstraintID
	kinds      []KindID
	provenance DeclarationProvenance
}

func NewKindDisjointConstraint(
	id ConstraintID,
	kinds []KindID,
	provenance DeclarationProvenance,
) (KindDisjointConstraint, error) {
	if !id.valid() {
		return KindDisjointConstraint{}, fmt.Errorf("kind-disjoint constraint ID is required")
	}
	if len(kinds) < 2 {
		return KindDisjointConstraint{}, fmt.Errorf("kind-disjoint constraint requires at least two kinds")
	}
	if !validDeclarationProvenance(provenance) {
		return KindDisjointConstraint{}, fmt.Errorf("kind-disjoint constraint provenance is required")
	}
	copyKinds := append([]KindID(nil), kinds...)
	sort.Slice(copyKinds, func(left, right int) bool {
		return copyKinds[left].String() < copyKinds[right].String()
	})
	for index, kindID := range copyKinds {
		if !kindID.valid() {
			return KindDisjointConstraint{}, fmt.Errorf("kind-disjoint operand %d is invalid", index)
		}
		if index > 0 && kindID == copyKinds[index-1] {
			return KindDisjointConstraint{}, fmt.Errorf("duplicate kind-disjoint operand %q", kindID.String())
		}
	}
	return KindDisjointConstraint{id: id, kinds: copyKinds, provenance: provenance}, nil
}

func (constraint KindDisjointConstraint) ID() ConstraintID { return constraint.id }

func (constraint KindDisjointConstraint) Kinds() []KindID {
	return append([]KindID(nil), constraint.kinds...)
}

func (constraint KindDisjointConstraint) Provenance() DeclarationProvenance {
	return constraint.provenance
}

func (constraint KindDisjointConstraint) CanonicalBytes() []byte {
	writer := newCanonicalWriter("kind-disjoint-constraint.v1")
	writer.addString(constraint.id.String())
	for _, kindID := range constraint.kinds {
		writer.addString(kindID.String())
	}
	writer.addBytes(constraint.provenance.CanonicalBytes())
	return writer.bytes()
}

func (KindDisjointConstraint) constraintRuleVariant() {}

type SlotGroupMode uint8

const (
	SlotGroupAllOrNone SlotGroupMode = iota + 1
	SlotGroupAtLeastOne
	SlotGroupExactlyOne
)

func (mode SlotGroupMode) String() string {
	switch mode {
	case SlotGroupAllOrNone:
		return "all_or_none"
	case SlotGroupAtLeastOne:
		return "at_least_one"
	case SlotGroupExactlyOne:
		return "exactly_one"
	default:
		return ""
	}
}

func (mode SlotGroupMode) valid() bool { return mode.String() != "" }

type SlotGroupConstraint struct {
	id         ConstraintID
	signature  RelationSignatureRef
	slots      []SlotKindID
	mode       SlotGroupMode
	provenance DeclarationProvenance
}

func NewSlotGroupConstraint(
	id ConstraintID,
	signature RelationSignatureRef,
	slots []SlotKindID,
	mode SlotGroupMode,
	provenance DeclarationProvenance,
) (SlotGroupConstraint, error) {
	if !id.valid() {
		return SlotGroupConstraint{}, fmt.Errorf("slot-group constraint ID is required")
	}
	if !signature.valid() {
		return SlotGroupConstraint{}, fmt.Errorf(
			"slot-group typed relation declaration fragment is required",
		)
	}
	if len(slots) < 2 {
		return SlotGroupConstraint{}, fmt.Errorf("slot-group constraint requires at least two slots")
	}
	if !mode.valid() {
		return SlotGroupConstraint{}, fmt.Errorf("slot-group constraint mode is required")
	}
	if !validDeclarationProvenance(provenance) {
		return SlotGroupConstraint{}, fmt.Errorf("slot-group constraint provenance is required")
	}
	copySlots := append([]SlotKindID(nil), slots...)
	sort.Slice(copySlots, func(left, right int) bool {
		return copySlots[left].String() < copySlots[right].String()
	})
	for index, slot := range copySlots {
		if !slot.valid() {
			return SlotGroupConstraint{}, fmt.Errorf("slot-group operand %d is invalid", index)
		}
		if index > 0 && slot == copySlots[index-1] {
			return SlotGroupConstraint{}, fmt.Errorf("duplicate slot-group operand %q", slot.String())
		}
	}
	return SlotGroupConstraint{
		id:         id,
		signature:  signature,
		slots:      copySlots,
		mode:       mode,
		provenance: provenance,
	}, nil
}

func (constraint SlotGroupConstraint) ID() ConstraintID { return constraint.id }

func (constraint SlotGroupConstraint) Signature() RelationSignatureRef {
	return constraint.signature
}

func (constraint SlotGroupConstraint) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return constraint.signature
}

func (constraint SlotGroupConstraint) Slots() []SlotKindID {
	return append([]SlotKindID(nil), constraint.slots...)
}

func (constraint SlotGroupConstraint) Mode() SlotGroupMode { return constraint.mode }

func (constraint SlotGroupConstraint) Provenance() DeclarationProvenance {
	return constraint.provenance
}

func (constraint SlotGroupConstraint) CanonicalBytes() []byte {
	writer := newCanonicalWriter("slot-group-constraint.v1")
	writer.addString(constraint.id.String())
	writer.addString(constraint.signature.String())
	writer.addString(constraint.mode.String())
	for _, slot := range constraint.slots {
		writer.addString(slot.String())
	}
	writer.addBytes(constraint.provenance.CanonicalBytes())
	return writer.bytes()
}

func (SlotGroupConstraint) constraintRuleVariant() {}

func validConstraintRule(rule ConstraintRule) bool {
	switch value := rule.(type) {
	case KindDisjointConstraint:
		return value.id.valid() &&
			len(value.kinds) >= 2 &&
			validDeclarationProvenance(value.provenance)
	case SlotGroupConstraint:
		return value.id.valid() &&
			value.signature.valid() &&
			len(value.slots) >= 2 &&
			value.mode.valid() &&
			validDeclarationProvenance(value.provenance)
	case SlotCardinalityConstraint:
		return value.valid()
	case ReferenceSlotSubsetConstraint:
		return value.valid()
	case ReferenceSlotPartitionConstraint:
		return value.valid()
	default:
		return false
	}
}

type TypeEnv struct {
	ref                      TypeEnvRef
	sourceRevision           SourceRevision
	compilerSchemaVersion    CompilerSchemaVersion
	coverage                 CoverageManifest
	contexts                 []BoundedContext
	kinds                    []KindDefinition
	entitySets               []EntitySetDefinition
	kindSignatures           []KindSignatureDefinition
	classificationSignatures []KindClassificationSignatureDefinition
	refKinds                 []RefKindDefinition
	kindAvailabilities       []ContextKindAvailability
	subkinds                 []SubkindRelation
	bridges                  []ContextBridge
	relationFragments        []TypedRelationDeclarationFragment
	shapes                   []ValueShapeDeclaration
	valueBindings            []ValueBinding
	constraints              []ConstraintRule
}

type TypeEnvBuilder struct {
	value TypeEnv
	err   error
}

func NewTypeEnvBuilder(ref TypeEnvRef) *TypeEnvBuilder {
	return &TypeEnvBuilder{value: TypeEnv{ref: ref}}
}

func (builder *TypeEnvBuilder) SetSourceRevision(
	revision SourceRevision,
) *TypeEnvBuilder {
	builder.value.sourceRevision = revision
	return builder
}

func (builder *TypeEnvBuilder) SetCompilerSchemaVersion(
	version CompilerSchemaVersion,
) *TypeEnvBuilder {
	builder.value.compilerSchemaVersion = version
	return builder
}

func (builder *TypeEnvBuilder) SetCoverageManifest(
	manifest CoverageManifest,
) *TypeEnvBuilder {
	builder.value.coverage = manifest
	return builder
}

func (builder *TypeEnvBuilder) AddBoundedContext(
	context BoundedContext,
) *TypeEnvBuilder {
	builder.value.contexts = append(builder.value.contexts, context)
	return builder
}

func (builder *TypeEnvBuilder) AddKindDefinition(
	definition KindDefinition,
) *TypeEnvBuilder {
	builder.value.kinds = append(builder.value.kinds, definition)
	return builder
}

func (builder *TypeEnvBuilder) AddEntitySetDefinition(
	definition EntitySetDefinition,
) *TypeEnvBuilder {
	builder.value.entitySets = append(builder.value.entitySets, definition)
	return builder
}

func (builder *TypeEnvBuilder) AddKindSignatureDefinition(
	definition KindSignatureDefinition,
) *TypeEnvBuilder {
	builder.value.kindSignatures = append(builder.value.kindSignatures, definition)
	return builder
}

// AddKindClassificationSignatureDefinition adds one current C.3.2
// KindSignature. The older AddKindSignatureDefinition spelling remains the
// edition-tagged MemberOf/EntitySet replay path and is deliberately not
// promoted into this collection.
func (builder *TypeEnvBuilder) AddKindClassificationSignatureDefinition(
	definition KindClassificationSignatureDefinition,
) *TypeEnvBuilder {
	builder.value.classificationSignatures = append(
		builder.value.classificationSignatures,
		definition,
	)
	return builder
}

func (builder *TypeEnvBuilder) AddRefKindDefinition(
	definition RefKindDefinition,
) *TypeEnvBuilder {
	builder.value.refKinds = append(builder.value.refKinds, definition)
	return builder
}

func (builder *TypeEnvBuilder) AddContextKindAvailability(
	availability ContextKindAvailability,
) *TypeEnvBuilder {
	builder.value.kindAvailabilities = append(
		builder.value.kindAvailabilities,
		cloneContextKindAvailability(availability),
	)
	return builder
}

func (builder *TypeEnvBuilder) AddSubkindRelation(
	relation SubkindRelation,
) *TypeEnvBuilder {
	builder.value.subkinds = append(builder.value.subkinds, relation)
	return builder
}

func (builder *TypeEnvBuilder) AddContextBridge(
	bridge ContextBridge,
) *TypeEnvBuilder {
	builder.value.bridges = append(builder.value.bridges, cloneContextBridge(bridge))
	return builder
}

func (builder *TypeEnvBuilder) AddTypedRelationDeclarationFragment(
	fragment TypedRelationDeclarationFragment,
) *TypeEnvBuilder {
	builder.value.relationFragments = append(
		builder.value.relationFragments,
		fragment,
	)
	return builder
}

// AddRelationSignature preserves the historical builder spelling for exact
// edition replay. It adds the same structurally limited fragment.
func (builder *TypeEnvBuilder) AddRelationSignature(
	signature RelationSignature,
) *TypeEnvBuilder {
	return builder.AddTypedRelationDeclarationFragment(signature)
}

func (builder *TypeEnvBuilder) AddValueShape(
	declaration ValueShapeDeclaration,
) *TypeEnvBuilder {
	builder.value.shapes = append(builder.value.shapes, declaration)
	return builder
}

func (builder *TypeEnvBuilder) AddValueBinding(
	binding ValueBinding,
) *TypeEnvBuilder {
	builder.value.valueBindings = append(builder.value.valueBindings, binding)
	return builder
}

func (builder *TypeEnvBuilder) AddConstraint(rule ConstraintRule) *TypeEnvBuilder {
	builder.value.constraints = append(builder.value.constraints, rule)
	return builder
}

func (builder *TypeEnvBuilder) Build() (TypeEnv, error) {
	if builder == nil {
		return TypeEnv{}, fmt.Errorf("TypeEnv builder is required")
	}
	if builder.err != nil {
		return TypeEnv{}, builder.err
	}
	value := cloneTypeEnv(builder.value)
	for index, rule := range value.constraints {
		if !validConstraintRule(rule) {
			return TypeEnv{}, fmt.Errorf("constraint %d is invalid", index)
		}
	}
	canonicalizeTypeEnv(&value)
	if err := validateTypeEnv(value); err != nil {
		return TypeEnv{}, err
	}
	return value, nil
}

func (environment TypeEnv) Ref() TypeEnvRef { return environment.ref }

func (environment TypeEnv) SourceRevision() SourceRevision {
	return environment.sourceRevision
}

func (environment TypeEnv) CompilerSchemaVersion() CompilerSchemaVersion {
	return environment.compilerSchemaVersion
}

func (environment TypeEnv) CoverageManifest() CoverageManifest {
	manifest, _ := NewCoverageManifest(environment.coverage.entries)
	return manifest
}

func (environment TypeEnv) BoundedContexts() []BoundedContext {
	return append([]BoundedContext(nil), environment.contexts...)
}

func (environment TypeEnv) KindDefinitions() []KindDefinition {
	return append([]KindDefinition(nil), environment.kinds...)
}

func (environment TypeEnv) EntitySetDefinitions() []EntitySetDefinition {
	result := make([]EntitySetDefinition, 0, len(environment.entitySets))
	for _, definition := range environment.entitySets {
		result = append(result, cloneEntitySetDefinition(definition))
	}
	return result
}

func (environment TypeEnv) KindSignatureDefinitions() []KindSignatureDefinition {
	result := make([]KindSignatureDefinition, 0, len(environment.kindSignatures))
	for _, definition := range environment.kindSignatures {
		result = append(result, cloneKindSignatureDefinition(definition))
	}
	return result
}

// KindClassificationSignatureDefinitions returns only current C.3.2
// signatures. Historical MemberOf-era signatures remain separately
// addressable through KindSignatureDefinitions for exact replay.
func (environment TypeEnv) KindClassificationSignatureDefinitions() []KindClassificationSignatureDefinition {
	result := make([]KindClassificationSignatureDefinition, 0, len(environment.classificationSignatures))
	for _, definition := range environment.classificationSignatures {
		result = append(result, cloneKindClassificationSignatureDefinition(definition))
	}
	return result
}

func (environment TypeEnv) RefKindDefinitions() []RefKindDefinition {
	return append([]RefKindDefinition(nil), environment.refKinds...)
}

func (environment TypeEnv) ContextKindAvailabilities() []ContextKindAvailability {
	return cloneContextKindAvailabilities(environment.kindAvailabilities)
}

func (environment TypeEnv) SubkindRelations() []SubkindRelation {
	return append([]SubkindRelation(nil), environment.subkinds...)
}

func (environment TypeEnv) ContextBridges() []ContextBridge {
	return cloneContextBridges(environment.bridges)
}

func (environment TypeEnv) TypedRelationDeclarationFragments() []TypedRelationDeclarationFragment {
	return append(
		[]TypedRelationDeclarationFragment(nil),
		environment.relationFragments...,
	)
}

// RelationSignatures preserves the historical accessor spelling for exact
// edition replay. Every returned value has typed-fragment posture.
func (environment TypeEnv) RelationSignatures() []RelationSignature {
	return append([]RelationSignature(nil), environment.relationFragments...)
}

func (environment TypeEnv) ValueShapes() []ValueShapeDeclaration {
	return append([]ValueShapeDeclaration(nil), environment.shapes...)
}

func (environment TypeEnv) ValueBindings() []ValueBinding {
	return append([]ValueBinding(nil), environment.valueBindings...)
}

func (environment TypeEnv) Constraints() []ConstraintRule {
	return append([]ConstraintRule(nil), environment.constraints...)
}

func (environment TypeEnv) BoundedContext(ref BoundedContextRef) (BoundedContext, bool) {
	index := sort.Search(len(environment.contexts), func(index int) bool {
		return environment.contexts[index].ref.String() >= ref.String()
	})
	if index >= len(environment.contexts) || environment.contexts[index].ref != ref {
		return BoundedContext{}, false
	}
	return environment.contexts[index], true
}

func (environment TypeEnv) KindDefinition(id KindID) (KindDefinition, bool) {
	index := sort.Search(len(environment.kinds), func(index int) bool {
		return environment.kinds[index].id.String() >= id.String()
	})
	if index >= len(environment.kinds) || environment.kinds[index].id != id {
		return KindDefinition{}, false
	}
	return environment.kinds[index], true
}

func (environment TypeEnv) EntitySetDefinition(
	ref EntitySetDefinitionRef,
) (EntitySetDefinition, bool) {
	index := sort.Search(len(environment.entitySets), func(index int) bool {
		return environment.entitySets[index].reference.String() >= ref.String()
	})
	if index >= len(environment.entitySets) || environment.entitySets[index].reference != ref {
		return EntitySetDefinition{}, false
	}
	return cloneEntitySetDefinition(environment.entitySets[index]), true
}

func (environment TypeEnv) EntitySetForContext(
	context BoundedContextRef,
) (EntitySetDefinition, bool) {
	for _, definition := range environment.entitySets {
		if definition.reference.Context() == context {
			return cloneEntitySetDefinition(definition), true
		}
	}
	return EntitySetDefinition{}, false
}

func (environment TypeEnv) KindSignatureDefinition(
	kind ValueKindRef,
	context BoundedContextRef,
) (KindSignatureDefinition, bool) {
	for _, definition := range environment.kindSignatures {
		if definition.ValueKind() == kind && definition.Ref().Context() == context {
			return cloneKindSignatureDefinition(definition), true
		}
	}
	return KindSignatureDefinition{}, false
}

func (environment TypeEnv) KindClassificationSignatureDefinition(
	localKind LocalKindRef,
) (KindClassificationSignatureDefinition, bool) {
	index := sort.Search(len(environment.classificationSignatures), func(index int) bool {
		return environment.classificationSignatures[index].LocalKind().String() >= localKind.String()
	})
	if index >= len(environment.classificationSignatures) ||
		environment.classificationSignatures[index].LocalKind() != localKind {
		return KindClassificationSignatureDefinition{}, false
	}
	return cloneKindClassificationSignatureDefinition(environment.classificationSignatures[index]), true
}

func (environment TypeEnv) RefKindDefinition(ref RefKindRef) (RefKindDefinition, bool) {
	index := sort.Search(len(environment.refKinds), func(index int) bool {
		return environment.refKinds[index].ref.String() >= ref.String()
	})
	if index >= len(environment.refKinds) || environment.refKinds[index].ref != ref {
		return RefKindDefinition{}, false
	}
	return environment.refKinds[index], true
}

func (environment TypeEnv) HasKindInContext(context BoundedContextRef, kindID KindID) bool {
	_, exists := environment.ContextKindAvailability(context, kindID)
	return exists
}

func (environment TypeEnv) ContextKindAvailability(
	context BoundedContextRef,
	kindID KindID,
) (ContextKindAvailability, bool) {
	key := context.String() + "/kind/" + kindID.String()
	index := sort.Search(len(environment.kindAvailabilities), func(index int) bool {
		return environment.kindAvailabilities[index].key() >= key
	})
	if index >= len(environment.kindAvailabilities) ||
		environment.kindAvailabilities[index].key() != key {
		return ContextKindAvailability{}, false
	}
	return cloneContextKindAvailability(environment.kindAvailabilities[index]), true
}

func (environment TypeEnv) IsSubkind(subkind, superkind KindID) bool {
	if subkind == superkind {
		return true
	}
	visited := map[KindID]struct{}{subkind: {}}
	frontier := []KindID{subkind}
	for len(frontier) > 0 {
		current := frontier[0]
		frontier = frontier[1:]
		for _, relation := range environment.subkinds {
			if relation.subkind != current {
				continue
			}
			if relation.superkind == superkind {
				return true
			}
			if _, exists := visited[relation.superkind]; exists {
				continue
			}
			visited[relation.superkind] = struct{}{}
			frontier = append(frontier, relation.superkind)
		}
	}
	return false
}

func (environment TypeEnv) HasContextBridge(
	source BoundedContextRef,
	target BoundedContextRef,
	sourceKind KindID,
	targetKind KindID,
) bool {
	for _, bridge := range environment.bridges {
		if bridge.AllowsMapping(source, sourceKind, target, targetKind) {
			return true
		}
	}
	return false
}

func (environment TypeEnv) TypedRelationDeclarationFragment(
	ref TypedRelationDeclarationFragmentRef,
) (TypedRelationDeclarationFragment, bool) {
	index := sort.Search(len(environment.relationFragments), func(index int) bool {
		return environment.relationFragments[index].ref.String() >= ref.String()
	})
	if index >= len(environment.relationFragments) ||
		environment.relationFragments[index].ref != ref {
		return TypedRelationDeclarationFragment{}, false
	}
	return environment.relationFragments[index], true
}

// RelationSignature preserves the historical accessor spelling for exact
// edition replay. It returns the same structurally limited fragment.
func (environment TypeEnv) RelationSignature(
	ref RelationSignatureRef,
) (RelationSignature, bool) {
	return environment.TypedRelationDeclarationFragment(ref)
}

func (environment TypeEnv) ValueBinding(kind ValueKindRef) (ValueBinding, bool) {
	index := sort.Search(len(environment.valueBindings), func(index int) bool {
		return environment.valueBindings[index].valueKind.String() >= kind.String()
	})
	if index >= len(environment.valueBindings) || environment.valueBindings[index].valueKind != kind {
		return ValueBinding{}, false
	}
	return environment.valueBindings[index], true
}

func (environment TypeEnv) ValueShape(ref ValueShapeRef) (ValueShapeDeclaration, bool) {
	index := sort.Search(len(environment.shapes), func(index int) bool {
		return environment.shapes[index].ref.String() >= ref.String()
	})
	if index >= len(environment.shapes) || environment.shapes[index].ref != ref {
		return ValueShapeDeclaration{}, false
	}
	return environment.shapes[index], true
}

func cloneTypeEnv(source TypeEnv) TypeEnv {
	entitySets := make([]EntitySetDefinition, 0, len(source.entitySets))
	for _, definition := range source.entitySets {
		entitySets = append(entitySets, cloneEntitySetDefinition(definition))
	}
	kindSignatures := make([]KindSignatureDefinition, 0, len(source.kindSignatures))
	for _, definition := range source.kindSignatures {
		kindSignatures = append(kindSignatures, cloneKindSignatureDefinition(definition))
	}
	classificationSignatures := make(
		[]KindClassificationSignatureDefinition,
		0,
		len(source.classificationSignatures),
	)
	for _, definition := range source.classificationSignatures {
		classificationSignatures = append(
			classificationSignatures,
			cloneKindClassificationSignatureDefinition(definition),
		)
	}
	return TypeEnv{
		ref:                      source.ref,
		sourceRevision:           source.sourceRevision,
		compilerSchemaVersion:    source.compilerSchemaVersion,
		coverage:                 CoverageManifest{entries: append([]CoverageEntry(nil), source.coverage.entries...)},
		contexts:                 append([]BoundedContext(nil), source.contexts...),
		kinds:                    append([]KindDefinition(nil), source.kinds...),
		entitySets:               entitySets,
		kindSignatures:           kindSignatures,
		classificationSignatures: classificationSignatures,
		refKinds:                 append([]RefKindDefinition(nil), source.refKinds...),
		kindAvailabilities:       cloneContextKindAvailabilities(source.kindAvailabilities),
		subkinds:                 append([]SubkindRelation(nil), source.subkinds...),
		bridges:                  cloneContextBridges(source.bridges),
		relationFragments: append(
			[]TypedRelationDeclarationFragment(nil),
			source.relationFragments...,
		),
		shapes:        append([]ValueShapeDeclaration(nil), source.shapes...),
		valueBindings: append([]ValueBinding(nil), source.valueBindings...),
		constraints:   append([]ConstraintRule(nil), source.constraints...),
	}
}

func canonicalizeTypeEnv(environment *TypeEnv) {
	sort.Slice(environment.contexts, func(left, right int) bool {
		return environment.contexts[left].ref.String() < environment.contexts[right].ref.String()
	})
	sort.Slice(environment.kinds, func(left, right int) bool {
		return environment.kinds[left].id.String() < environment.kinds[right].id.String()
	})
	sort.Slice(environment.entitySets, func(left, right int) bool {
		return environment.entitySets[left].reference.String() <
			environment.entitySets[right].reference.String()
	})
	sort.Slice(environment.kindSignatures, func(left, right int) bool {
		leftRef := environment.kindSignatures[left].Ref()
		rightRef := environment.kindSignatures[right].Ref()
		if leftRef.key() != rightRef.key() {
			return leftRef.key() < rightRef.key()
		}
		return leftRef.Digest().String() < rightRef.Digest().String()
	})
	sort.Slice(environment.classificationSignatures, func(left, right int) bool {
		leftRef := environment.classificationSignatures[left].LocalKind().String()
		rightRef := environment.classificationSignatures[right].LocalKind().String()
		if leftRef != rightRef {
			return leftRef < rightRef
		}
		return environment.classificationSignatures[left].Ref().Digest().String() <
			environment.classificationSignatures[right].Ref().Digest().String()
	})
	sort.Slice(environment.refKinds, func(left, right int) bool {
		return environment.refKinds[left].ref.String() < environment.refKinds[right].ref.String()
	})
	sort.Slice(environment.kindAvailabilities, func(left, right int) bool {
		return environment.kindAvailabilities[left].key() < environment.kindAvailabilities[right].key()
	})
	sort.Slice(environment.subkinds, func(left, right int) bool {
		return environment.subkinds[left].key() < environment.subkinds[right].key()
	})
	sort.Slice(environment.bridges, func(left, right int) bool {
		return environment.bridges[left].id.String() < environment.bridges[right].id.String()
	})
	sort.Slice(environment.relationFragments, func(left, right int) bool {
		return environment.relationFragments[left].ref.String() <
			environment.relationFragments[right].ref.String()
	})
	sort.Slice(environment.shapes, func(left, right int) bool {
		return environment.shapes[left].ref.String() < environment.shapes[right].ref.String()
	})
	sort.Slice(environment.valueBindings, func(left, right int) bool {
		return environment.valueBindings[left].valueKind.String() < environment.valueBindings[right].valueKind.String()
	})
	sort.Slice(environment.constraints, func(left, right int) bool {
		return environment.constraints[left].ID().String() < environment.constraints[right].ID().String()
	})
}

func validateTypeEnv(environment TypeEnv) error {
	checks := []struct {
		valid   bool
		message string
	}{
		{environment.ref.valid(), "TypeEnv reference is required"},
		{environment.sourceRevision.valid(), "TypeEnv source revision is required"},
		{environment.compilerSchemaVersion.valid(), "TypeEnv compiler schema version is required"},
		{environment.coverage.valid(), "TypeEnv coverage manifest is required"},
		{len(environment.contexts) > 0, "TypeEnv requires at least one bounded context"},
	}
	for _, check := range checks {
		if !check.valid {
			return fmt.Errorf("%s", check.message)
		}
	}
	if err := validateContextDeclarations(environment); err != nil {
		return err
	}
	if err := validateKindDeclarations(environment); err != nil {
		return err
	}
	if err := validateLegacyMembershipDeclarations(environment); err != nil {
		return err
	}
	if err := validateKindClassificationDeclarations(environment); err != nil {
		return err
	}
	if err := validateSubkindGraph(environment); err != nil {
		return err
	}
	if err := validateBridges(environment); err != nil {
		return err
	}
	if err := validateRelationDeclarationFragments(environment); err != nil {
		return err
	}
	if err := validateValueShapes(environment); err != nil {
		return err
	}
	if err := validateValueBindings(environment); err != nil {
		return err
	}
	return validateConstraints(environment)
}

// validateLegacyMembershipDeclarations keeps the sealed EntitySet/MemberOf
// declaration family readable under its original TypeEnv editions. Current
// C.3.2 compilation never writes into these collections.
func validateLegacyMembershipDeclarations(environment TypeEnv) error {
	seenContexts := map[BoundedContextRef]struct{}{}
	seenSignatures := map[string]struct{}{}
	for index, definition := range environment.entitySets {
		if !definition.valid() {
			return fmt.Errorf("EntitySet definition %d is invalid", index)
		}
		if definition.Ref().TypeEnv() != environment.ref {
			return fmt.Errorf(
				"EntitySet definition %q belongs to another TypeEnv",
				definition.Ref().String(),
			)
		}
		if _, exists := environment.BoundedContext(definition.Ref().Context()); !exists {
			return fmt.Errorf(
				"EntitySet definition %q references unknown context",
				definition.Ref().String(),
			)
		}
		if _, exists := seenContexts[definition.Ref().Context()]; exists {
			return fmt.Errorf(
				"duplicate EntitySet definition for context %q",
				definition.Ref().Context().String(),
			)
		}
		seenContexts[definition.Ref().Context()] = struct{}{}
	}
	for index, definition := range environment.kindSignatures {
		if !definition.valid() {
			return fmt.Errorf("KindSignature definition %d is invalid", index)
		}
		if definition.Ref().TypeEnv() != environment.ref {
			return fmt.Errorf(
				"KindSignature definition %q belongs to another TypeEnv",
				definition.Ref().String(),
			)
		}
		if _, exists := environment.KindDefinition(definition.ValueKind().ID()); !exists {
			return fmt.Errorf(
				"KindSignature definition %q references unknown ValueKind",
				definition.Ref().String(),
			)
		}
		if _, exists := environment.EntitySetDefinition(definition.EntitySet()); !exists {
			return fmt.Errorf(
				"KindSignature definition %q references unknown EntitySet definition",
				definition.Ref().String(),
			)
		}
		if definition.Ref().Context() != definition.EntitySet().Context() {
			return fmt.Errorf(
				"KindSignature definition %q and EntitySet use different contexts",
				definition.Ref().String(),
			)
		}
		if !environment.HasKindInContext(
			definition.Ref().Context(),
			definition.ValueKind().ID(),
		) {
			return fmt.Errorf(
				"KindSignature definition %q uses kind %q unavailable in context %q",
				definition.Ref().String(),
				definition.ValueKind().String(),
				definition.Ref().Context().String(),
			)
		}
		key := definition.Ref().key()
		if _, exists := seenSignatures[key]; exists {
			return fmt.Errorf(
				"duplicate KindSignature definition for %q in context %q",
				definition.ValueKind().String(),
				definition.Ref().Context().String(),
			)
		}
		seenSignatures[key] = struct{}{}
	}
	return nil
}

func validateKindClassificationDeclarations(environment TypeEnv) error {
	seen := make(map[string]struct{}, len(environment.classificationSignatures))
	for index, definition := range environment.classificationSignatures {
		if !definition.Valid() {
			return fmt.Errorf("current KindSignature definition %d is invalid", index)
		}
		localKind := definition.LocalKind()
		if localKind.TypeEnv() != environment.ref ||
			definition.CandidateValueKind().TypeEnv() != environment.ref {
			return fmt.Errorf(
				"current KindSignature definition %q belongs to another TypeEnv",
				definition.Ref().String(),
			)
		}
		if _, exists := environment.BoundedContext(localKind.Context()); !exists {
			return fmt.Errorf(
				"current KindSignature definition %q references unknown context",
				definition.Ref().String(),
			)
		}
		if _, exists := environment.KindDefinition(localKind.ValueKind().ID()); !exists {
			return fmt.Errorf(
				"current KindSignature definition %q references unknown local kind",
				definition.Ref().String(),
			)
		}
		if _, exists := environment.KindDefinition(definition.CandidateValueKind().ID()); !exists {
			return fmt.Errorf(
				"current KindSignature definition %q references unknown candidate ValueKind",
				definition.Ref().String(),
			)
		}
		if !environment.HasKindInContext(localKind.Context(), localKind.ValueKind().ID()) {
			return fmt.Errorf(
				"current KindSignature definition %q uses local kind unavailable in context %q",
				definition.Ref().String(),
				localKind.Context().String(),
			)
		}
		key := localKind.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf(
				"duplicate current KindSignature definition for local kind %q",
				key,
			)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func cloneEntitySetDefinition(
	definition EntitySetDefinition,
) EntitySetDefinition {
	definition.canonicalBytes = append([]byte(nil), definition.canonicalBytes...)
	return definition
}

func cloneKindSignatureDefinition(
	definition KindSignatureDefinition,
) KindSignatureDefinition {
	definition.assumptions = append([]KindAssumptionPin(nil), definition.assumptions...)
	definition.canonicalBytes = append([]byte(nil), definition.canonicalBytes...)
	return definition
}

func cloneKindClassificationSignatureDefinition(
	definition KindClassificationSignatureDefinition,
) KindClassificationSignatureDefinition {
	definition.dependencies = append(
		[]KindSignatureDependencyPin(nil),
		definition.dependencies...,
	)
	definition.canonicalBytes = append([]byte(nil), definition.canonicalBytes...)
	return definition
}

func validateValueShapes(environment TypeEnv) error {
	for index, declaration := range environment.shapes {
		if !declaration.ref.valid() ||
			!validValueShapeDeclaration(declaration.shape) ||
			!validDeclarationProvenance(declaration.provenance) {
			return fmt.Errorf("value-shape declaration %d is invalid", index)
		}
		if index > 0 && declaration.ref == environment.shapes[index-1].ref {
			return fmt.Errorf("duplicate value-shape declaration %q", declaration.ref.String())
		}
	}
	dependencies, err := valueShapeDependencyGraph(environment.shapes)
	if err != nil {
		return err
	}
	if err := validateValueShapeDependencyClosure(environment.shapes, dependencies); err != nil {
		return err
	}
	cycle := firstValueShapeDependencyCycle(environment.shapes, dependencies)
	if len(cycle) > 0 {
		return fmt.Errorf(
			"value-shape dependency cycle: %s",
			formatValueShapeDependencyCycle(cycle),
		)
	}
	for index, declaration := range environment.shapes {
		if err := VerifyValueShapeRef(declaration.ref, declaration.shape); err != nil {
			return fmt.Errorf(
				"value-shape declaration %d has invalid content identity: %w",
				index,
				err,
			)
		}
	}
	return nil
}

func valueShapeDependencyGraph(
	declarations []ValueShapeDeclaration,
) (map[ValueShapeRef][]ValueShapeRef, error) {
	dependencies := make(map[ValueShapeRef][]ValueShapeRef, len(declarations))
	for _, declaration := range declarations {
		children, err := valueShapeDependencies(declaration.shape)
		if err != nil {
			return nil, fmt.Errorf(
				"value-shape declaration %q has invalid dependencies: %w",
				declaration.ref.String(),
				err,
			)
		}
		dependencies[declaration.ref] = children
	}
	return dependencies, nil
}

func validateValueShapeDependencyClosure(
	declarations []ValueShapeDeclaration,
	dependencies map[ValueShapeRef][]ValueShapeRef,
) error {
	declared := make(map[ValueShapeRef]struct{}, len(declarations))
	for _, declaration := range declarations {
		declared[declaration.ref] = struct{}{}
	}
	for _, declaration := range declarations {
		for _, child := range dependencies[declaration.ref] {
			if _, exists := declared[child]; !exists {
				return fmt.Errorf(
					"value-shape declaration %q references missing child shape %q",
					declaration.ref.String(),
					child.String(),
				)
			}
		}
	}
	return nil
}

type valueShapeDependencyVisitState uint8

const (
	valueShapeDependencyUnvisited valueShapeDependencyVisitState = iota
	valueShapeDependencyVisiting
	valueShapeDependencyVisited
)

type valueShapeDependencyFrame struct {
	ref       ValueShapeRef
	nextChild int
}

func firstValueShapeDependencyCycle(
	declarations []ValueShapeDeclaration,
	dependencies map[ValueShapeRef][]ValueShapeRef,
) []ValueShapeRef {
	states := make(map[ValueShapeRef]valueShapeDependencyVisitState, len(declarations))
	positions := make(map[ValueShapeRef]int, len(declarations))
	for _, declaration := range declarations {
		start := declaration.ref
		if states[start] != valueShapeDependencyUnvisited {
			continue
		}
		states[start] = valueShapeDependencyVisiting
		positions[start] = 0
		path := []ValueShapeRef{start}
		stack := []valueShapeDependencyFrame{{ref: start}}
		for len(stack) > 0 {
			frameIndex := len(stack) - 1
			frame := &stack[frameIndex]
			children := dependencies[frame.ref]
			if frame.nextChild >= len(children) {
				states[frame.ref] = valueShapeDependencyVisited
				delete(positions, frame.ref)
				stack = stack[:frameIndex]
				path = path[:len(path)-1]
				continue
			}
			child := children[frame.nextChild]
			frame.nextChild++
			switch states[child] {
			case valueShapeDependencyUnvisited:
				states[child] = valueShapeDependencyVisiting
				positions[child] = len(path)
				path = append(path, child)
				stack = append(stack, valueShapeDependencyFrame{ref: child})
			case valueShapeDependencyVisiting:
				cycleStart := positions[child]
				cycle := append([]ValueShapeRef(nil), path[cycleStart:]...)
				cycle = append(cycle, child)
				return canonicalizeValueShapeDependencyCycle(cycle)
			case valueShapeDependencyVisited:
			}
		}
	}
	return nil
}

func canonicalizeValueShapeDependencyCycle(
	cycle []ValueShapeRef,
) []ValueShapeRef {
	if len(cycle) < 2 {
		return append([]ValueShapeRef(nil), cycle...)
	}
	body := cycle[:len(cycle)-1]
	minimum := 0
	for index := 1; index < len(body); index++ {
		if body[index].String() < body[minimum].String() {
			minimum = index
		}
	}
	canonical := make([]ValueShapeRef, len(cycle))
	for offset := range body {
		index := (minimum + offset) % len(body)
		canonical[offset] = body[index]
	}
	canonical[len(canonical)-1] = canonical[0]
	return canonical
}

func formatValueShapeDependencyCycle(cycle []ValueShapeRef) string {
	parts := make([]string, 0, len(cycle))
	for _, ref := range cycle {
		parts = append(parts, ref.String())
	}
	return strings.Join(parts, " -> ")
}

func validateContextDeclarations(environment TypeEnv) error {
	for index, context := range environment.contexts {
		if !context.valid() {
			return fmt.Errorf("bounded context %d is invalid", index)
		}
		if index > 0 && context.ref == environment.contexts[index-1].ref {
			return fmt.Errorf("duplicate bounded context %q", context.ref.String())
		}
	}
	return nil
}

func validateKindDeclarations(environment TypeEnv) error {
	for index, definition := range environment.kinds {
		if !definition.valid() {
			return fmt.Errorf("kind definition %d is invalid", index)
		}
		if index > 0 && definition.id == environment.kinds[index-1].id {
			return fmt.Errorf("duplicate kind definition %q", definition.id.String())
		}
	}
	for index, definition := range environment.refKinds {
		if !definition.valid() {
			return fmt.Errorf("RefKind definition %d is invalid", index)
		}
		if definition.ref.TypeEnv() != environment.ref ||
			definition.valueKind.TypeEnv() != environment.ref {
			return fmt.Errorf("RefKind definition %q belongs to another TypeEnv", definition.ref.String())
		}
		if index > 0 && definition.ref == environment.refKinds[index-1].ref {
			return fmt.Errorf("duplicate RefKind definition %q", definition.ref.String())
		}
		if _, exists := environment.KindDefinition(definition.valueKind.ID()); !exists {
			return fmt.Errorf("RefKind definition %q references unknown ValueKind", definition.ref.String())
		}
	}
	for index, availability := range environment.kindAvailabilities {
		if !availability.valid() {
			return fmt.Errorf("context kind availability %d is invalid", index)
		}
		if index > 0 && availability.key() == environment.kindAvailabilities[index-1].key() {
			return fmt.Errorf("duplicate context kind availability %q", availability.key())
		}
		if _, exists := environment.BoundedContext(availability.context); !exists {
			return fmt.Errorf(
				"context kind availability references unknown context %q",
				availability.context.String(),
			)
		}
		if _, exists := environment.KindDefinition(availability.kindID); !exists {
			return fmt.Errorf(
				"context kind availability references unknown kind %q",
				availability.kindID.String(),
			)
		}
	}
	return nil
}

func validateSubkindGraph(environment TypeEnv) error {
	for index, relation := range environment.subkinds {
		if !relation.valid() {
			return fmt.Errorf("subkind relation %d is invalid", index)
		}
		if index > 0 && relation.key() == environment.subkinds[index-1].key() {
			return fmt.Errorf("duplicate subkind relation %q", relation.key())
		}
		subkind, subkindExists := environment.KindDefinition(relation.subkind)
		superkind, superkindExists := environment.KindDefinition(relation.superkind)
		if !subkindExists || !superkindExists {
			return fmt.Errorf("subkind relation %q references an unknown kind", relation.key())
		}
		_ = subkind
		_ = superkind
	}
	for _, definition := range environment.kinds {
		if subkindCycleFrom(environment.subkinds, definition.id) {
			return fmt.Errorf("subkind cycle reaches %q", definition.id.String())
		}
	}
	return nil
}

func subkindCycleFrom(relations []SubkindRelation, start KindID) bool {
	visited := map[KindID]struct{}{}
	active := map[KindID]struct{}{}
	var visit func(KindID) bool
	visit = func(current KindID) bool {
		if _, exists := active[current]; exists {
			return true
		}
		if _, exists := visited[current]; exists {
			return false
		}
		visited[current] = struct{}{}
		active[current] = struct{}{}
		for _, relation := range relations {
			if relation.subkind == current && visit(relation.superkind) {
				return true
			}
		}
		delete(active, current)
		return false
	}
	return visit(start)
}

func validateBridges(environment TypeEnv) error {
	for index, bridge := range environment.bridges {
		if !bridge.valid() {
			return fmt.Errorf("context bridge %d is invalid", index)
		}
		if index > 0 && bridge.id == environment.bridges[index-1].id {
			return fmt.Errorf("duplicate context bridge %q", bridge.id.String())
		}
		if _, exists := environment.BoundedContext(bridge.source.Context()); !exists {
			return fmt.Errorf("context bridge %q references unknown source context", bridge.id.String())
		}
		if _, exists := environment.BoundedContext(bridge.target.Context()); !exists {
			return fmt.Errorf("context bridge %q references unknown target context", bridge.id.String())
		}
		sourceKind, sourceExists := environment.KindDefinition(bridge.mapping.SourceKind())
		targetKind, targetExists := environment.KindDefinition(bridge.mapping.TargetKind())
		if !sourceExists || !targetExists {
			return fmt.Errorf("context bridge %q references an unknown kind", bridge.id.String())
		}
		sourceValueKind, err := NewValueKindRef(environment.ref, sourceKind.ID())
		if err != nil {
			return fmt.Errorf("context bridge %q source kind reference: %w", bridge.id.String(), err)
		}
		targetValueKind, err := NewValueKindRef(environment.ref, targetKind.ID())
		if err != nil {
			return fmt.Errorf("context bridge %q target kind reference: %w", bridge.id.String(), err)
		}
		if !environment.hasAnyKindSignature(sourceValueKind, bridge.source.Context()) {
			return fmt.Errorf(
				"context bridge %q requires source KindSignature for %q in context %q",
				bridge.id.String(),
				sourceKind.ID().String(),
				bridge.source.Context().String(),
			)
		}
		if !environment.hasAnyKindSignature(targetValueKind, bridge.target.Context()) {
			return fmt.Errorf(
				"context bridge %q requires target KindSignature for %q in context %q",
				bridge.id.String(),
				targetKind.ID().String(),
				bridge.target.Context().String(),
			)
		}
	}
	return nil
}

func (environment TypeEnv) hasAnyKindSignature(
	valueKind ValueKindRef,
	context BoundedContextRef,
) bool {
	localKind, err := NewLocalKindRef(valueKind, context)
	if err == nil {
		if _, exists := environment.KindClassificationSignatureDefinition(localKind); exists {
			return true
		}
	}
	_, exists := environment.KindSignatureDefinition(valueKind, context)
	return exists
}

func validateRelationDeclarationFragments(environment TypeEnv) error {
	for index, fragment := range environment.relationFragments {
		if !fragment.valid() {
			return fmt.Errorf(
				"typed relation declaration fragment %d is invalid",
				index,
			)
		}
		if fragment.ref.TypeEnv() != environment.ref {
			return fmt.Errorf(
				"typed relation declaration fragment %q belongs to another TypeEnv",
				fragment.ref.String(),
			)
		}
		if index > 0 &&
			fragment.ref == environment.relationFragments[index-1].ref {
			return fmt.Errorf(
				"duplicate typed relation declaration fragment %q",
				fragment.ref.String(),
			)
		}
		for _, context := range fragment.contexts {
			if _, exists := environment.BoundedContext(context); !exists {
				return fmt.Errorf(
					"typed relation declaration fragment %q references unknown context %q",
					fragment.ref.String(),
					context.String(),
				)
			}
		}
		for _, slot := range fragment.slots {
			if err := validateSlotSpec(environment, fragment.ref, slot); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSlotSpec(
	environment TypeEnv,
	fragment TypedRelationDeclarationFragmentRef,
	slot SlotSpec,
) error {
	switch target := slot.target.(type) {
	case ValueSlotTarget:
		if target.kind.TypeEnv() != environment.ref {
			return fmt.Errorf("typed relation declaration fragment %q slot %q value kind belongs to another TypeEnv", fragment.String(), slot.slotKind.String())
		}
		_, found := environment.KindDefinition(target.kind.ID())
		if !found {
			return fmt.Errorf("typed relation declaration fragment %q slot %q has no ValueKind definition", fragment.String(), slot.slotKind.String())
		}
	case ReferenceSlotTarget:
		if target.valueKind.TypeEnv() != environment.ref ||
			target.referenceKind.TypeEnv() != environment.ref {
			return fmt.Errorf("typed relation declaration fragment %q slot %q reference kind belongs to another TypeEnv", fragment.String(), slot.slotKind.String())
		}
		_, valueFound := environment.KindDefinition(target.valueKind.ID())
		if !valueFound {
			return fmt.Errorf("typed relation declaration fragment %q slot %q has no ValueKind definition", fragment.String(), slot.slotKind.String())
		}
		referenceDefinition, referenceFound := environment.RefKindDefinition(target.referenceKind)
		if !referenceFound {
			return fmt.Errorf("typed relation declaration fragment %q slot %q has no RefKind definition", fragment.String(), slot.slotKind.String())
		}
		if !environment.IsSubkind(target.valueKind.ID(), referenceDefinition.valueKind.ID()) {
			return fmt.Errorf("typed relation declaration fragment %q slot %q ValueKind is incompatible with its RefKind", fragment.String(), slot.slotKind.String())
		}
	default:
		return fmt.Errorf("typed relation declaration fragment %q slot %q target is unknown", fragment.String(), slot.slotKind.String())
	}
	return nil
}

func validateValueBindings(environment TypeEnv) error {
	for index, binding := range environment.valueBindings {
		if !binding.valid() {
			return fmt.Errorf("value binding %d is invalid", index)
		}
		if binding.valueKind.TypeEnv() != environment.ref {
			return fmt.Errorf("value binding %q belongs to another TypeEnv", binding.valueKind.String())
		}
		if index > 0 && binding.valueKind == environment.valueBindings[index-1].valueKind {
			return fmt.Errorf("duplicate value binding %q", binding.valueKind.String())
		}
		_, exists := environment.KindDefinition(binding.valueKind.ID())
		if !exists {
			return fmt.Errorf("value binding %q has no ValueKind definition", binding.valueKind.String())
		}
		if _, exists := environment.ValueShape(binding.valueShape); !exists {
			return fmt.Errorf("value binding %q references unknown shape %q", binding.valueKind.String(), binding.valueShape.String())
		}
	}
	return nil
}

func validateConstraints(environment TypeEnv) error {
	for index, rule := range environment.constraints {
		if !validConstraintRule(rule) {
			return fmt.Errorf("constraint %d is invalid", index)
		}
		if index > 0 && rule.ID() == environment.constraints[index-1].ID() {
			return fmt.Errorf("duplicate constraint %q", rule.ID().String())
		}
		switch value := rule.(type) {
		case KindDisjointConstraint:
			for _, kindID := range value.kinds {
				if _, exists := environment.KindDefinition(kindID); !exists {
					return fmt.Errorf("constraint %q references unknown kind %q", value.id.String(), kindID.String())
				}
			}
			for _, definition := range environment.kinds {
				matches := disjointConstraintMatches(environment, value, definition.id)
				if len(matches) > 1 {
					return fmt.Errorf(
						"kind %q is a subkind of multiple operands of disjoint constraint %q",
						definition.id.String(),
						value.id.String(),
					)
				}
			}
		case SlotGroupConstraint:
			fragment, exists := environment.TypedRelationDeclarationFragment(value.signature)
			if !exists {
				return fmt.Errorf(
					"constraint %q references an unknown typed relation declaration fragment",
					value.id.String(),
				)
			}
			for _, slot := range value.slots {
				if _, exists := fragment.Slot(slot); !exists {
					return fmt.Errorf("constraint %q references unknown slot %q", value.id.String(), slot.String())
				}
			}
		case SlotCardinalityConstraint:
			if err := validateSlotCardinalityConstraint(environment, value); err != nil {
				return err
			}
		case ReferenceSlotSubsetConstraint:
			if err := validateReferenceSlotSubsetConstraint(environment, value); err != nil {
				return err
			}
		case ReferenceSlotPartitionConstraint:
			if err := validateReferenceSlotPartitionConstraint(environment, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSlotCardinalityConstraint(
	environment TypeEnv,
	constraint SlotCardinalityConstraint,
) error {
	fragment, exists := environment.TypedRelationDeclarationFragment(constraint.signature)
	if !exists {
		return fmt.Errorf(
			"constraint %q references an unknown typed relation declaration fragment",
			constraint.id.String(),
		)
	}
	slot, exists := fragment.Slot(constraint.slot)
	if !exists {
		return fmt.Errorf(
			"constraint %q references unknown slot %q",
			constraint.id.String(),
			constraint.slot.String(),
		)
	}
	if !equalCardinality(constraint.cardinality, slot.cardinality) {
		return fmt.Errorf(
			"constraint %q cardinality does not exactly match slot %q",
			constraint.id.String(),
			constraint.slot.String(),
		)
	}
	return nil
}

func validateReferenceSlotSubsetConstraint(
	environment TypeEnv,
	constraint ReferenceSlotSubsetConstraint,
) error {
	fragment, exists := environment.TypedRelationDeclarationFragment(constraint.signature)
	if !exists {
		return fmt.Errorf(
			"constraint %q references an unknown typed relation declaration fragment",
			constraint.id.String(),
		)
	}
	subset, err := referenceSlotTarget(fragment, constraint.id, constraint.subset)
	if err != nil {
		return err
	}
	superset, err := referenceSlotTarget(fragment, constraint.id, constraint.superset)
	if err != nil {
		return err
	}
	if !equalReferenceSlotTarget(subset, superset) {
		return fmt.Errorf(
			"constraint %q reference slots do not have one exact target",
			constraint.id.String(),
		)
	}
	return nil
}

func validateReferenceSlotPartitionConstraint(
	environment TypeEnv,
	constraint ReferenceSlotPartitionConstraint,
) error {
	fragment, exists := environment.TypedRelationDeclarationFragment(constraint.signature)
	if !exists {
		return fmt.Errorf(
			"constraint %q references an unknown typed relation declaration fragment",
			constraint.id.String(),
		)
	}
	whole, err := referenceSlotTarget(fragment, constraint.id, constraint.whole)
	if err != nil {
		return err
	}
	for _, part := range constraint.parts {
		partTarget, partErr := referenceSlotTarget(fragment, constraint.id, part)
		if partErr != nil {
			return partErr
		}
		if !equalReferenceSlotTarget(whole, partTarget) {
			return fmt.Errorf(
				"constraint %q reference slots do not have one exact target",
				constraint.id.String(),
			)
		}
	}
	return nil
}

func referenceSlotTarget(
	fragment TypedRelationDeclarationFragment,
	constraintID ConstraintID,
	slotID SlotKindID,
) (ReferenceSlotTarget, error) {
	slot, exists := fragment.Slot(slotID)
	if !exists {
		return ReferenceSlotTarget{}, fmt.Errorf(
			"constraint %q references unknown slot %q",
			constraintID.String(),
			slotID.String(),
		)
	}
	target, reference := slot.target.(ReferenceSlotTarget)
	if !reference {
		return ReferenceSlotTarget{}, fmt.Errorf(
			"constraint %q slot %q must be ByReference",
			constraintID.String(),
			slotID.String(),
		)
	}
	return target, nil
}

func equalReferenceSlotTarget(left, right ReferenceSlotTarget) bool {
	return left.valueKind == right.valueKind && left.referenceKind == right.referenceKind
}
