// Package localpractice parses versioned Haft Local-Practice signature
// carriers into a symbolic, source-preserving AST. The AST deliberately has
// no TypeEnv-bearing references: lowering is a later operation that may run
// only after a composite TypeEnv identity has been derived. Parse validates
// carrier grammar and source safety only. SemVer policy, carrier-to-manifest
// identity coherence, import/provide closure, and runtime mechanism/codec
// registry resolution are deliberately linker-owned.
package localpractice

import "fmt"

const SchemaVersion = "haft.local-practice/v1"

type SourceLineRange struct {
	start uint64
	end   uint64
}

func newSourceLineRange(start, end uint64) (SourceLineRange, error) {
	if start == 0 || end == 0 {
		return SourceLineRange{}, fmt.Errorf("source line range must be positive")
	}
	if end < start {
		return SourceLineRange{}, fmt.Errorf("source line range end %d precedes start %d", end, start)
	}
	return SourceLineRange{start: start, end: end}, nil
}

func (lineRange SourceLineRange) Start() uint64 { return lineRange.start }

func (lineRange SourceLineRange) End() uint64 { return lineRange.end }

type SourceDigest struct {
	value string
}

func (digest SourceDigest) String() string { return digest.value }

type SourceText struct {
	value string
	span  SourceLineRange
}

func (text SourceText) Value() string { return text.value }

func (text SourceText) Span() SourceLineRange { return text.span }

type OptionalSourceText struct {
	present bool
	value   SourceText
}

func (text OptionalSourceText) Value() (SourceText, bool) {
	return text.value, text.present
}

type ParsedCarrier struct {
	carrier Carrier
	digest  SourceDigest
}

func (parsed ParsedCarrier) Carrier() Carrier { return parsed.carrier.clone() }

func (parsed ParsedCarrier) Digest() SourceDigest { return parsed.digest }

type Carrier struct {
	schemaVersion     SourceText
	identity          CarrierIdentity
	baseTypeEnvRef    SourceText
	boundedContextRef SourceText
	compilerVersion   SourceText
	manifest          SignatureManifest
	signature         SignatureBlock
	span              SourceLineRange
}

func (carrier Carrier) SchemaVersion() SourceText { return carrier.schemaVersion }

func (carrier Carrier) Identity() CarrierIdentity { return carrier.identity }

func (carrier Carrier) BaseTypeEnvRef() SourceText { return carrier.baseTypeEnvRef }

func (carrier Carrier) BoundedContextRef() SourceText { return carrier.boundedContextRef }

func (carrier Carrier) CompilerVersion() SourceText { return carrier.compilerVersion }

func (carrier Carrier) Manifest() SignatureManifest { return carrier.manifest.clone() }

func (carrier Carrier) Signature() SignatureBlock { return carrier.signature.clone() }

func (carrier Carrier) Span() SourceLineRange { return carrier.span }

func (carrier Carrier) clone() Carrier {
	carrier.manifest = carrier.manifest.clone()
	carrier.signature = carrier.signature.clone()
	return carrier
}

type CarrierIdentity struct {
	id      SourceText
	edition SourceText
	span    SourceLineRange
}

func (identity CarrierIdentity) ID() SourceText { return identity.id }

func (identity CarrierIdentity) Edition() SourceText { return identity.edition }

func (identity CarrierIdentity) Span() SourceLineRange { return identity.span }

type PublicationState string

const (
	PublicationDraft      PublicationState = "draft"
	PublicationCandidate  PublicationState = "candidate"
	PublicationStable     PublicationState = "stable"
	PublicationDeprecated PublicationState = "deprecated"
)

type ManifestImport struct {
	signatureID SourceText
}

func (item ManifestImport) SignatureID() SourceText { return item.signatureID }

type ManifestProvide struct {
	symbol SourceText
}

func (item ManifestProvide) Symbol() SourceText { return item.symbol }

type SignatureManifest struct {
	id                  SourceText
	version             SourceText
	hasPublicationState bool
	publicationState    PublicationState
	imports             []ManifestImport
	provides            []ManifestProvide
	span                SourceLineRange
}

func (manifest SignatureManifest) ID() SourceText { return manifest.id }

func (manifest SignatureManifest) Version() SourceText { return manifest.version }

func (manifest SignatureManifest) PublicationState() (PublicationState, bool) {
	return manifest.publicationState, manifest.hasPublicationState
}

func (manifest SignatureManifest) Imports() []ManifestImport {
	return append([]ManifestImport(nil), manifest.imports...)
}

func (manifest SignatureManifest) Provides() []ManifestProvide {
	return append([]ManifestProvide(nil), manifest.provides...)
}

func (manifest SignatureManifest) Span() SourceLineRange { return manifest.span }

func (manifest SignatureManifest) clone() SignatureManifest {
	manifest.imports = manifest.Imports()
	manifest.provides = manifest.Provides()
	return manifest
}

type SignatureBlock struct {
	subjectBlock  SubjectBlock
	vocabulary    Vocabulary
	laws          Laws
	applicability Applicability
	span          SourceLineRange
}

func (block SignatureBlock) SubjectBlock() SubjectBlock { return block.subjectBlock }

func (block SignatureBlock) Vocabulary() Vocabulary { return block.vocabulary.clone() }

func (block SignatureBlock) Laws() Laws { return block.laws.clone() }

func (block SignatureBlock) Applicability() Applicability {
	return block.applicability.clone()
}

func (block SignatureBlock) Span() SourceLineRange { return block.span }

func (block SignatureBlock) clone() SignatureBlock {
	block.vocabulary = block.vocabulary.clone()
	block.laws = block.laws.clone()
	block.applicability = block.applicability.clone()
	return block
}

type SubjectBlock struct {
	subjectKind     SourceText
	rangedValueKind SourceText
	sliceSet        SourceText
	extentRule      SourceText
	resultKind      OptionalSourceText
	span            SourceLineRange
}

func (block SubjectBlock) SubjectKind() SourceText { return block.subjectKind }

func (block SubjectBlock) RangedValueKind() SourceText { return block.rangedValueKind }

func (block SubjectBlock) SliceSet() SourceText { return block.sliceSet }

func (block SubjectBlock) ExtentRule() SourceText { return block.extentRule }

func (block SubjectBlock) ResultKind() OptionalSourceText { return block.resultKind }

func (block SubjectBlock) Span() SourceLineRange { return block.span }

type DeclarationKind string

const (
	DeclarationBoundedContext DeclarationKind = "bounded_context"
	DeclarationValueKind      DeclarationKind = "value_kind"
	DeclarationSubkind        DeclarationKind = "subkind"
	DeclarationRefKind        DeclarationKind = "ref_kind"
	DeclarationEntitySet      DeclarationKind = "entity_set_definition"
	DeclarationKindSignature  DeclarationKind = "kind_signature_definition"
	// DeclarationKindClassificationSignature is the current C.3.2 carrier.
	// The older kind_signature_definition token remains sealed to the
	// MemberOf/EntitySet editions 1.0.0-1.2.0.
	DeclarationKindClassificationSignature DeclarationKind = "kind_classification_signature_definition"
	// DeclarationRelationSignature is the exact historical source token used
	// by the sealed 1.0.0, 1.1.0, and 1.2.0 carriers. The compiler interprets this
	// edition-tagged spelling as a TypedRelationDeclarationFragment; it does not
	// claim a complete FPF RelationSignature.
	DeclarationRelationSignature           DeclarationKind = "relation_signature"
	DeclarationRuntimeEvaluatorInput       DeclarationKind = "runtime_evaluator_input"
	DeclarationValueShape                  DeclarationKind = "value_shape"
	DeclarationCodecBinding                DeclarationKind = "codec_binding"
	DeclarationRuntimeEvaluatorRequirement DeclarationKind = "runtime_evaluator_requirement"
	DeclarationConstraint                  DeclarationKind = "constraint"
	DeclarationKindBridge                  DeclarationKind = "kind_bridge"
)

type Declaration interface {
	Kind() DeclarationKind
	Symbol() SourceText
	Span() SourceLineRange
	declarationVariant()
}

type Vocabulary struct {
	declarations []Declaration
	span         SourceLineRange
}

func (vocabulary Vocabulary) Declarations() []Declaration {
	return append([]Declaration(nil), vocabulary.declarations...)
}

func (vocabulary Vocabulary) Span() SourceLineRange { return vocabulary.span }

func (vocabulary Vocabulary) clone() Vocabulary {
	vocabulary.declarations = vocabulary.Declarations()
	return vocabulary
}

type ValueKindDeclaration struct {
	symbol SourceText
	span   SourceLineRange
}

func (ValueKindDeclaration) Kind() DeclarationKind { return DeclarationValueKind }

func (declaration ValueKindDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration ValueKindDeclaration) Span() SourceLineRange { return declaration.span }

func (ValueKindDeclaration) declarationVariant() {}

// BoundedContextDeclaration explicitly names the bounded context owned by the
// carrier. Unlike declaration symbols for TypeEnv kinds, its symbol is an
// opaque BoundedContextRef and therefore need not be a qualified source name.
// Coherence with the carrier root and Signature Applicability is compiler-
// verified because this package deliberately has no TypeEnv-bearing types.
type BoundedContextDeclaration struct {
	symbol SourceText
	span   SourceLineRange
}

func (BoundedContextDeclaration) Kind() DeclarationKind { return DeclarationBoundedContext }

func (declaration BoundedContextDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration BoundedContextDeclaration) Span() SourceLineRange { return declaration.span }

func (BoundedContextDeclaration) declarationVariant() {}

// SubkindDeclaration gives a source declaration its own addressable identity
// while keeping the structural child/super relation explicit. Its declaration
// symbol is provenance only; the relation itself exports no TypeEnv schema
// symbol.
type SubkindDeclaration struct {
	symbol    SourceText
	childKind SourceText
	superKind SourceText
	span      SourceLineRange
}

func (SubkindDeclaration) Kind() DeclarationKind { return DeclarationSubkind }

func (declaration SubkindDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration SubkindDeclaration) ChildKind() SourceText { return declaration.childKind }

func (declaration SubkindDeclaration) SuperKind() SourceText { return declaration.superKind }

func (declaration SubkindDeclaration) Span() SourceLineRange { return declaration.span }

func (SubkindDeclaration) declarationVariant() {}

type RefKindDeclaration struct {
	symbol    SourceText
	valueKind SourceText
	span      SourceLineRange
}

func (RefKindDeclaration) Kind() DeclarationKind { return DeclarationRefKind }

func (declaration RefKindDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration RefKindDeclaration) ValueKind() SourceText { return declaration.valueKind }

func (declaration RefKindDeclaration) Span() SourceLineRange { return declaration.span }

func (RefKindDeclaration) declarationVariant() {}

type EntitySetPolicyKind string

const (
	EntitySetPersistedOnly EntitySetPolicyKind = "persisted_entities_only"
	EntitySetPriorBatch    EntitySetPolicyKind = "prior_batch_declarations_visible"
)

type EntitySetCandidatePolicy interface {
	Kind() EntitySetPolicyKind
	Span() SourceLineRange
	entitySetCandidatePolicyVariant()
}

type PersistedEntitiesOnlyPolicy struct {
	span SourceLineRange
}

func (PersistedEntitiesOnlyPolicy) Kind() EntitySetPolicyKind {
	return EntitySetPersistedOnly
}

func (policy PersistedEntitiesOnlyPolicy) Span() SourceLineRange { return policy.span }

func (PersistedEntitiesOnlyPolicy) entitySetCandidatePolicyVariant() {}

type PriorBatchDeclarationsVisiblePolicy struct {
	evaluationRule SourceText
	span           SourceLineRange
}

func (PriorBatchDeclarationsVisiblePolicy) Kind() EntitySetPolicyKind {
	return EntitySetPriorBatch
}

func (policy PriorBatchDeclarationsVisiblePolicy) EvaluationRule() SourceText {
	return policy.evaluationRule
}

func (policy PriorBatchDeclarationsVisiblePolicy) Span() SourceLineRange { return policy.span }

func (PriorBatchDeclarationsVisiblePolicy) entitySetCandidatePolicyVariant() {}

type EntitySetDefinitionDeclaration struct {
	symbol          SourceText
	enumerationRule SourceText
	candidatePolicy EntitySetCandidatePolicy
	span            SourceLineRange
}

func (EntitySetDefinitionDeclaration) Kind() DeclarationKind { return DeclarationEntitySet }

func (declaration EntitySetDefinitionDeclaration) Symbol() SourceText {
	return declaration.symbol
}

func (declaration EntitySetDefinitionDeclaration) EnumerationRule() SourceText {
	return declaration.enumerationRule
}

func (declaration EntitySetDefinitionDeclaration) CandidatePolicy() EntitySetCandidatePolicy {
	return declaration.candidatePolicy
}

func (declaration EntitySetDefinitionDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (EntitySetDefinitionDeclaration) declarationVariant() {}

type SignatureFormality uint8

const (
	SignatureF0 SignatureFormality = iota
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

func (formality SignatureFormality) String() string {
	if formality > SignatureF9 {
		return ""
	}
	return fmt.Sprintf("F%d", uint8(formality))
}

type KindSignatureAssumption struct {
	carrierRef SourceText
	edition    SourceText
	digest     SourceText
	span       SourceLineRange
}

func (assumption KindSignatureAssumption) CarrierRef() SourceText {
	return assumption.carrierRef
}

func (assumption KindSignatureAssumption) Edition() SourceText { return assumption.edition }

func (assumption KindSignatureAssumption) Digest() SourceText { return assumption.digest }

func (assumption KindSignatureAssumption) Span() SourceLineRange { return assumption.span }

// KindSignatureMembershipBasisKind declares how the deterministic MemberOf
// evaluator receives the observable content required by C.3.2. This is a Haft
// Local-Practice implementation choice, not a new FPF kind.
type KindSignatureMembershipBasisKind string

const (
	KindSignatureDirectObservableInputs KindSignatureMembershipBasisKind = "direct_observable_inputs"
	KindSignatureCarrierFirst           KindSignatureMembershipBasisKind = "carrier_first"
)

// KindSignatureMembershipBasis is a closed sum. A KindSignature cannot omit
// the basis and leave later runtime-basis derivation to naming convention.
type KindSignatureMembershipBasis interface {
	Kind() KindSignatureMembershipBasisKind
	KindSource() SourceText
	Span() SourceLineRange
	kindSignatureMembershipBasisVariant()
}

// DirectObservableInputsMembershipBasis means the membership evaluator's
// declared inputs already are the observable slice content. It requires no
// separate carrier-membership adapter mechanism.
type DirectObservableInputsMembershipBasis struct {
	kindSource SourceText
	span       SourceLineRange
}

func (DirectObservableInputsMembershipBasis) Kind() KindSignatureMembershipBasisKind {
	return KindSignatureDirectObservableInputs
}

func (basis DirectObservableInputsMembershipBasis) KindSource() SourceText {
	return basis.kindSource
}

func (basis DirectObservableInputsMembershipBasis) Span() SourceLineRange {
	return basis.span
}

func (DirectObservableInputsMembershipBasis) kindSignatureMembershipBasisVariant() {}

// CarrierFirstMembershipBasis means observable membership inputs must first
// be recovered from a carrier by one exact adapter RuleRef. The declaration
// names that rule symbolically; E compilation does not claim it is executable.
type CarrierFirstMembershipBasis struct {
	kindSource  SourceText
	adapterRule SourceText
	span        SourceLineRange
}

func (CarrierFirstMembershipBasis) Kind() KindSignatureMembershipBasisKind {
	return KindSignatureCarrierFirst
}

func (basis CarrierFirstMembershipBasis) KindSource() SourceText {
	return basis.kindSource
}

func (basis CarrierFirstMembershipBasis) AdapterRule() SourceText {
	return basis.adapterRule
}

func (basis CarrierFirstMembershipBasis) Span() SourceLineRange { return basis.span }

func (CarrierFirstMembershipBasis) kindSignatureMembershipBasisVariant() {}

type KindSignatureDefinitionDeclaration struct {
	symbol          SourceText
	valueKind       SourceText
	formality       SignatureFormality
	assumptions     []KindSignatureAssumption
	definednessRule SourceText
	evaluatorRule   SourceText
	membershipBasis KindSignatureMembershipBasis
	entitySet       SourceText
	span            SourceLineRange
}

func (KindSignatureDefinitionDeclaration) Kind() DeclarationKind {
	return DeclarationKindSignature
}

func (declaration KindSignatureDefinitionDeclaration) Symbol() SourceText {
	return declaration.symbol
}

func (declaration KindSignatureDefinitionDeclaration) ValueKind() SourceText {
	return declaration.valueKind
}

func (declaration KindSignatureDefinitionDeclaration) Formality() SignatureFormality {
	return declaration.formality
}

func (declaration KindSignatureDefinitionDeclaration) Assumptions() []KindSignatureAssumption {
	return append([]KindSignatureAssumption(nil), declaration.assumptions...)
}

func (declaration KindSignatureDefinitionDeclaration) DefinednessRule() SourceText {
	return declaration.definednessRule
}

func (declaration KindSignatureDefinitionDeclaration) EvaluatorRule() SourceText {
	return declaration.evaluatorRule
}

func (declaration KindSignatureDefinitionDeclaration) MembershipBasis() KindSignatureMembershipBasis {
	return declaration.membershipBasis
}

func (declaration KindSignatureDefinitionDeclaration) EntitySet() SourceText {
	return declaration.entitySet
}

func (declaration KindSignatureDefinitionDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (KindSignatureDefinitionDeclaration) declarationVariant() {}

type ReferenceModeKind string

const (
	ReferenceByValue ReferenceModeKind = "by_value"
	ReferenceByKind  ReferenceModeKind = "reference"
)

type ReferenceMode interface {
	Kind() ReferenceModeKind
	Span() SourceLineRange
	referenceModeVariant()
}

type ByValueReferenceMode struct {
	span SourceLineRange
}

func (ByValueReferenceMode) Kind() ReferenceModeKind { return ReferenceByValue }

func (mode ByValueReferenceMode) Span() SourceLineRange { return mode.span }

func (ByValueReferenceMode) referenceModeVariant() {}

type RefKindReferenceMode struct {
	refKind SourceText
	span    SourceLineRange
}

func (RefKindReferenceMode) Kind() ReferenceModeKind { return ReferenceByKind }

func (mode RefKindReferenceMode) RefKind() SourceText { return mode.refKind }

func (mode RefKindReferenceMode) Span() SourceLineRange { return mode.span }

func (RefKindReferenceMode) referenceModeVariant() {}

type Cardinality struct {
	minimum uint64
	maximum OptionalMaximum
	span    SourceLineRange
}

func (cardinality Cardinality) Minimum() uint64 { return cardinality.minimum }

func (cardinality Cardinality) Maximum() OptionalMaximum { return cardinality.maximum }

func (cardinality Cardinality) Span() SourceLineRange { return cardinality.span }

type OptionalMaximum struct {
	unbounded bool
	value     uint64
}

func (maximum OptionalMaximum) Value() (uint64, bool) {
	return maximum.value, !maximum.unbounded
}

func (maximum OptionalMaximum) Unbounded() bool { return maximum.unbounded }

type SlotSpec struct {
	slotKind  SourceText
	valueKind SourceText
	reference ReferenceMode
	span      SourceLineRange
}

func (slot SlotSpec) SlotKind() SourceText { return slot.slotKind }

func (slot SlotSpec) ValueKind() SourceText { return slot.valueKind }

func (slot SlotSpec) ReferenceMode() ReferenceMode { return slot.reference }

func (slot SlotSpec) Span() SourceLineRange { return slot.span }

type RelationDeclarationPosture string

const (
	RelationDeclarationTypedFragment RelationDeclarationPosture = "typed_relation_declaration_fragment"
)

// TypedRelationDeclarationFragmentDeclaration is the current semantic type
// for the historical relation_signature source form. The closed carrier has
// only a symbol and A.6.5-like SlotSpecs; predicate/laws, applicability,
// occurrence identity, and U.Signature/C.2.1 identity basis are inexpressible.
type TypedRelationDeclarationFragmentDeclaration struct {
	symbol SourceText
	slots  []SlotSpec
	span   SourceLineRange
}

func (TypedRelationDeclarationFragmentDeclaration) Kind() DeclarationKind {
	return DeclarationRelationSignature
}

func (declaration TypedRelationDeclarationFragmentDeclaration) Symbol() SourceText {
	return declaration.symbol
}

func (declaration TypedRelationDeclarationFragmentDeclaration) Slots() []SlotSpec {
	return append([]SlotSpec(nil), declaration.slots...)
}

func (declaration TypedRelationDeclarationFragmentDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (TypedRelationDeclarationFragmentDeclaration) Posture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (TypedRelationDeclarationFragmentDeclaration) declarationVariant() {}

// RelationSignatureDeclaration is the historical Go spelling retained for
// exact carrier-edition compatibility.
type RelationSignatureDeclaration = TypedRelationDeclarationFragmentDeclaration

// RuntimeEvaluatorInputDeclaration is a local input-carrier schema for one
// separately declared runtime evaluator requirement. Its SlotSpecs describe
// designations supplied to that evaluator; they do not declare a relation or
// claim that any direct predicate obtains.
type RuntimeEvaluatorInputDeclaration struct {
	symbol               SourceText
	evaluatorRequirement SourceText
	slots                []SlotSpec
	span                 SourceLineRange
}

func (RuntimeEvaluatorInputDeclaration) Kind() DeclarationKind {
	return DeclarationRuntimeEvaluatorInput
}

func (declaration RuntimeEvaluatorInputDeclaration) Symbol() SourceText {
	return declaration.symbol
}

func (declaration RuntimeEvaluatorInputDeclaration) EvaluatorRequirement() SourceText {
	return declaration.evaluatorRequirement
}

func (declaration RuntimeEvaluatorInputDeclaration) Slots() []SlotSpec {
	return append([]SlotSpec(nil), declaration.slots...)
}

func (declaration RuntimeEvaluatorInputDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (RuntimeEvaluatorInputDeclaration) declarationVariant() {}

type ValueShapeKind string

const (
	ValueShapeScalar          ValueShapeKind = "scalar"
	ValueShapeRecord          ValueShapeKind = "record"
	ValueShapeSum             ValueShapeKind = "sum"
	ValueShapeOrderedSequence ValueShapeKind = "ordered_sequence"
	ValueShapeUnorderedSet    ValueShapeKind = "unordered_set"
	ValueShapeClaimGraph      ValueShapeKind = "claim_graph"
)

type ValueShape interface {
	Kind() ValueShapeKind
	Span() SourceLineRange
	valueShapeVariant()
}

type ScalarValueShape struct {
	scalarKind SourceText
	span       SourceLineRange
}

func (ScalarValueShape) Kind() ValueShapeKind { return ValueShapeScalar }

func (shape ScalarValueShape) ScalarKind() SourceText { return shape.scalarKind }

func (shape ScalarValueShape) Span() SourceLineRange { return shape.span }

func (ScalarValueShape) valueShapeVariant() {}

type ShapeMember struct {
	name  SourceText
	shape SourceText
	span  SourceLineRange
}

func (member ShapeMember) Name() SourceText { return member.name }

func (member ShapeMember) Shape() SourceText { return member.shape }

func (member ShapeMember) Span() SourceLineRange { return member.span }

type RecordValueShape struct {
	fields []ShapeMember
	span   SourceLineRange
}

func (RecordValueShape) Kind() ValueShapeKind { return ValueShapeRecord }

func (shape RecordValueShape) Fields() []ShapeMember {
	return append([]ShapeMember(nil), shape.fields...)
}

func (shape RecordValueShape) Span() SourceLineRange { return shape.span }

func (RecordValueShape) valueShapeVariant() {}

type SumValueShape struct {
	variants []ShapeMember
	span     SourceLineRange
}

func (SumValueShape) Kind() ValueShapeKind { return ValueShapeSum }

func (shape SumValueShape) Variants() []ShapeMember {
	return append([]ShapeMember(nil), shape.variants...)
}

func (shape SumValueShape) Span() SourceLineRange { return shape.span }

func (SumValueShape) valueShapeVariant() {}

type CollectionValueShape struct {
	kind    ValueShapeKind
	element SourceText
	span    SourceLineRange
}

func (shape CollectionValueShape) Kind() ValueShapeKind { return shape.kind }

func (shape CollectionValueShape) ElementShape() SourceText { return shape.element }

func (shape CollectionValueShape) Span() SourceLineRange { return shape.span }

func (CollectionValueShape) valueShapeVariant() {}

type ClaimGraphValueShape struct {
	span SourceLineRange
}

func (ClaimGraphValueShape) Kind() ValueShapeKind { return ValueShapeClaimGraph }

func (shape ClaimGraphValueShape) Span() SourceLineRange { return shape.span }

func (ClaimGraphValueShape) valueShapeVariant() {}

type ValueShapeDeclaration struct {
	symbol SourceText
	shape  ValueShape
	span   SourceLineRange
}

func (ValueShapeDeclaration) Kind() DeclarationKind { return DeclarationValueShape }

func (declaration ValueShapeDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration ValueShapeDeclaration) Shape() ValueShape { return declaration.shape }

func (declaration ValueShapeDeclaration) Span() SourceLineRange { return declaration.span }

func (ValueShapeDeclaration) declarationVariant() {}

type CodecBindingDeclaration struct {
	symbol                  SourceText
	valueKind               SourceText
	valueShape              SourceText
	canonicalizationVersion SourceText
	contract                []SourceText
	span                    SourceLineRange
}

func (CodecBindingDeclaration) Kind() DeclarationKind { return DeclarationCodecBinding }

func (declaration CodecBindingDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration CodecBindingDeclaration) ValueKind() SourceText {
	return declaration.valueKind
}

func (declaration CodecBindingDeclaration) ValueShape() SourceText {
	return declaration.valueShape
}

func (declaration CodecBindingDeclaration) CanonicalizationVersion() SourceText {
	return declaration.canonicalizationVersion
}

func (declaration CodecBindingDeclaration) Contract() []SourceText {
	return append([]SourceText(nil), declaration.contract...)
}

func (declaration CodecBindingDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (CodecBindingDeclaration) declarationVariant() {}

// RuntimeEvaluatorRequirementDeclaration is a source-owned requirement for
// one evaluator coordinate that a later X basis must resolve. It describes a
// semantic RuleRef and invocation contract; it neither selects a mechanism nor
// claims that a mechanism is registered, callable, invoked, or satisfied.
type RuntimeEvaluatorRequirementDeclaration struct {
	symbol             SourceText
	ruleRef            SourceText
	invocationContract SourceText
	span               SourceLineRange
}

func (RuntimeEvaluatorRequirementDeclaration) Kind() DeclarationKind {
	return DeclarationRuntimeEvaluatorRequirement
}

func (declaration RuntimeEvaluatorRequirementDeclaration) Symbol() SourceText {
	return declaration.symbol
}

func (declaration RuntimeEvaluatorRequirementDeclaration) RuleRef() SourceText {
	return declaration.ruleRef
}

func (declaration RuntimeEvaluatorRequirementDeclaration) InvocationContract() SourceText {
	return declaration.invocationContract
}

func (declaration RuntimeEvaluatorRequirementDeclaration) Span() SourceLineRange {
	return declaration.span
}

func (RuntimeEvaluatorRequirementDeclaration) declarationVariant() {}

type ConstraintKind string

const (
	ConstraintKindDisjoint           ConstraintKind = "kind_disjoint"
	ConstraintSlotGroup              ConstraintKind = "slot_group"
	ConstraintSlotCardinality        ConstraintKind = "slot_cardinality"
	ConstraintReferenceSlotSubset    ConstraintKind = "reference_slot_subset"
	ConstraintReferenceSlotPartition ConstraintKind = "reference_slot_partition"
)

type ConstraintRule interface {
	Kind() ConstraintKind
	Span() SourceLineRange
	constraintRuleVariant()
}

type KindDisjointConstraint struct {
	kinds []SourceText
	span  SourceLineRange
}

func (KindDisjointConstraint) Kind() ConstraintKind { return ConstraintKindDisjoint }

func (constraint KindDisjointConstraint) Kinds() []SourceText {
	return append([]SourceText(nil), constraint.kinds...)
}

func (constraint KindDisjointConstraint) Span() SourceLineRange { return constraint.span }

func (KindDisjointConstraint) constraintRuleVariant() {}

type SlotGroupMode string

const (
	SlotGroupAllOrNone  SlotGroupMode = "all_or_none"
	SlotGroupAtLeastOne SlotGroupMode = "at_least_one"
	SlotGroupExactlyOne SlotGroupMode = "exactly_one"
)

type SlotGroupConstraint struct {
	relation SourceText
	slots    []SourceText
	mode     SlotGroupMode
	span     SourceLineRange
}

func (SlotGroupConstraint) Kind() ConstraintKind { return ConstraintSlotGroup }

func (constraint SlotGroupConstraint) Relation() SourceText { return constraint.relation }

func (constraint SlotGroupConstraint) Slots() []SourceText {
	return append([]SourceText(nil), constraint.slots...)
}

func (constraint SlotGroupConstraint) Mode() SlotGroupMode { return constraint.mode }

func (constraint SlotGroupConstraint) Span() SourceLineRange { return constraint.span }

func (SlotGroupConstraint) constraintRuleVariant() {}

type SlotCardinalityConstraint struct {
	relation    SourceText
	slot        SourceText
	cardinality Cardinality
	span        SourceLineRange
}

func (SlotCardinalityConstraint) Kind() ConstraintKind {
	return ConstraintSlotCardinality
}

func (constraint SlotCardinalityConstraint) Relation() SourceText {
	return constraint.relation
}

func (constraint SlotCardinalityConstraint) Slot() SourceText { return constraint.slot }

func (constraint SlotCardinalityConstraint) Cardinality() Cardinality {
	return constraint.cardinality
}

func (constraint SlotCardinalityConstraint) Span() SourceLineRange { return constraint.span }

func (SlotCardinalityConstraint) constraintRuleVariant() {}

type ReferenceSlotSubsetConstraint struct {
	relation SourceText
	subset   SourceText
	superset SourceText
	span     SourceLineRange
}

func (ReferenceSlotSubsetConstraint) Kind() ConstraintKind {
	return ConstraintReferenceSlotSubset
}

func (constraint ReferenceSlotSubsetConstraint) Relation() SourceText {
	return constraint.relation
}

func (constraint ReferenceSlotSubsetConstraint) Subset() SourceText {
	return constraint.subset
}

func (constraint ReferenceSlotSubsetConstraint) Superset() SourceText {
	return constraint.superset
}

func (constraint ReferenceSlotSubsetConstraint) Span() SourceLineRange {
	return constraint.span
}

func (ReferenceSlotSubsetConstraint) constraintRuleVariant() {}

type ReferenceSlotPartitionConstraint struct {
	relation SourceText
	whole    SourceText
	parts    []SourceText
	span     SourceLineRange
}

func (ReferenceSlotPartitionConstraint) Kind() ConstraintKind {
	return ConstraintReferenceSlotPartition
}

func (constraint ReferenceSlotPartitionConstraint) Relation() SourceText {
	return constraint.relation
}

func (constraint ReferenceSlotPartitionConstraint) Whole() SourceText {
	return constraint.whole
}

func (constraint ReferenceSlotPartitionConstraint) Parts() []SourceText {
	return append([]SourceText(nil), constraint.parts...)
}

func (constraint ReferenceSlotPartitionConstraint) Span() SourceLineRange {
	return constraint.span
}

func (ReferenceSlotPartitionConstraint) constraintRuleVariant() {}

type ConstraintDeclaration struct {
	symbol SourceText
	rule   ConstraintRule
	span   SourceLineRange
}

func (ConstraintDeclaration) Kind() DeclarationKind { return DeclarationConstraint }

func (declaration ConstraintDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration ConstraintDeclaration) Rule() ConstraintRule { return declaration.rule }

func (declaration ConstraintDeclaration) Span() SourceLineRange { return declaration.span }

func (ConstraintDeclaration) declarationVariant() {}

// KindBridgeDirectionKind is the explicitly declared direction of one
// C.3.3 KindBridge. One-way never grants the inverse mapping. Two-way grants
// only the exact inverse of the same named mapping; it does not imply any
// additional kind correspondence.
type KindBridgeDirectionKind string

const (
	KindBridgeOneWay KindBridgeDirectionKind = "one_way"
	KindBridgeTwoWay KindBridgeDirectionKind = "two_way"
)

type KindBridgeDirection struct {
	kind KindBridgeDirectionKind
	span SourceLineRange
}

func (direction KindBridgeDirection) Kind() KindBridgeDirectionKind {
	return direction.kind
}

func (direction KindBridgeDirection) Span() SourceLineRange { return direction.span }

// KindBridgeEndpoint pins one bounded context and its exact vocabulary or
// Standard edition. The edition is source identity, not an implicit "latest"
// selector.
type KindBridgeEndpoint struct {
	boundedContextRef SourceText
	edition           SourceText
	span              SourceLineRange
}

func (endpoint KindBridgeEndpoint) BoundedContextRef() SourceText {
	return endpoint.boundedContextRef
}

func (endpoint KindBridgeEndpoint) Edition() SourceText { return endpoint.edition }

func (endpoint KindBridgeEndpoint) Span() SourceLineRange { return endpoint.span }

type KindBridgeMappingKind string

const KindBridgeNamedTarget KindBridgeMappingKind = "named_target"

type KindBridgeMapping interface {
	Kind() KindBridgeMappingKind
	SourceKind() SourceText
	TargetKind() SourceText
	Span() SourceLineRange
	kindBridgeMappingVariant()
}

// NamedTargetKindMapping is the closed v1 KindBridge mapping form. Signature
// translation is deliberately inexpressible until its source grammar and
// lowering rules are specified end to end.
type NamedTargetKindMapping struct {
	kindSource SourceText
	sourceKind SourceText
	targetKind SourceText
	span       SourceLineRange
}

func (NamedTargetKindMapping) Kind() KindBridgeMappingKind {
	return KindBridgeNamedTarget
}

func (mapping NamedTargetKindMapping) KindSource() SourceText { return mapping.kindSource }

func (mapping NamedTargetKindMapping) SourceKind() SourceText {
	return mapping.sourceKind
}

func (mapping NamedTargetKindMapping) TargetKind() SourceText {
	return mapping.targetKind
}

func (mapping NamedTargetKindMapping) Span() SourceLineRange { return mapping.span }

func (NamedTargetKindMapping) kindBridgeMappingVariant() {}

// KindBridgeOrderPreservationKind states which source SubkindOf fragment the
// bridge claims to cover. v1 admits only an explicit empty fragment. This is a
// real, narrow C.3.3 bridge; it must not silently claim preservation of links
// that the carrier cannot yet express and check.
type KindBridgeOrderPreservationKind string

const KindBridgeNoOrderLinksCovered KindBridgeOrderPreservationKind = "no_links_covered"

type KindBridgeOrderPreservation struct {
	kind KindBridgeOrderPreservationKind
	span SourceLineRange
}

func (preservation KindBridgeOrderPreservation) Kind() KindBridgeOrderPreservationKind {
	return preservation.kind
}

func (preservation KindBridgeOrderPreservation) Span() SourceLineRange {
	return preservation.span
}

// KindCongruenceLevel is C.3.3 CL^k on the closed ordinal 0..3 ladder. It is
// kept distinct from scope CL and does not alter F or G.
type KindCongruenceLevel struct {
	value uint8
	span  SourceLineRange
}

func (level KindCongruenceLevel) Value() uint8 { return level.value }

func (level KindCongruenceLevel) Span() SourceLineRange { return level.span }

// KindBridgeDeclaration is one source-level C.3.3 named-target bridge in the
// Vocabulary row. Its surrounding Signature retains Laws and Applicability;
// the declaration itself retains exact endpoint editions, direction, CL^k,
// loss notes, definedness, and source coordinates. It is not a derived
// ContextKindAvailability and grants no project-head or write authority.
type KindBridgeDeclaration struct {
	symbol            SourceText
	source            KindBridgeEndpoint
	target            KindBridgeEndpoint
	mapping           NamedTargetKindMapping
	direction         KindBridgeDirection
	orderPreservation KindBridgeOrderPreservation
	kindCongruence    KindCongruenceLevel
	lossNotes         []SourceText
	definednessArea   []SourceText
	span              SourceLineRange
}

func (KindBridgeDeclaration) Kind() DeclarationKind { return DeclarationKindBridge }

func (declaration KindBridgeDeclaration) Symbol() SourceText { return declaration.symbol }

func (declaration KindBridgeDeclaration) Source() KindBridgeEndpoint {
	return declaration.source
}

func (declaration KindBridgeDeclaration) Target() KindBridgeEndpoint {
	return declaration.target
}

func (declaration KindBridgeDeclaration) Mapping() KindBridgeMapping {
	return declaration.mapping
}

func (declaration KindBridgeDeclaration) Direction() KindBridgeDirection {
	return declaration.direction
}

func (declaration KindBridgeDeclaration) OrderPreservation() KindBridgeOrderPreservation {
	return declaration.orderPreservation
}

func (declaration KindBridgeDeclaration) KindCongruence() KindCongruenceLevel {
	return declaration.kindCongruence
}

func (declaration KindBridgeDeclaration) LossNotes() []SourceText {
	return append([]SourceText(nil), declaration.lossNotes...)
}

func (declaration KindBridgeDeclaration) DefinednessArea() []SourceText {
	return append([]SourceText(nil), declaration.definednessArea...)
}

func (declaration KindBridgeDeclaration) Span() SourceLineRange { return declaration.span }

// AllowsMapping is the exact source-level direction check used by parser and
// compiler tests. It intentionally returns false for every correspondence not
// literally declared, including the reverse of a one-way bridge.
func (declaration KindBridgeDeclaration) AllowsMapping(
	sourceContext string,
	sourceKind string,
	targetContext string,
	targetKind string,
) bool {
	forward := sourceContext == declaration.source.boundedContextRef.value &&
		sourceKind == declaration.mapping.sourceKind.value &&
		targetContext == declaration.target.boundedContextRef.value &&
		targetKind == declaration.mapping.targetKind.value
	if forward {
		return true
	}
	if declaration.direction.kind != KindBridgeTwoWay {
		return false
	}
	return sourceContext == declaration.target.boundedContextRef.value &&
		sourceKind == declaration.mapping.targetKind.value &&
		targetContext == declaration.source.boundedContextRef.value &&
		targetKind == declaration.mapping.sourceKind.value
}

func (KindBridgeDeclaration) declarationVariant() {}

type Laws struct {
	constraintRefs []SourceText
	invariants     []SourceText
	span           SourceLineRange
}

func (laws Laws) ConstraintRefs() []SourceText {
	return append([]SourceText(nil), laws.constraintRefs...)
}

func (laws Laws) Invariants() []SourceText {
	return append([]SourceText(nil), laws.invariants...)
}

func (laws Laws) Span() SourceLineRange { return laws.span }

func (laws Laws) clone() Laws {
	laws.constraintRefs = laws.ConstraintRefs()
	laws.invariants = laws.Invariants()
	return laws
}

type Applicability struct {
	boundedContextRef SourceText
	assumptions       []SourceText
	span              SourceLineRange
}

func (applicability Applicability) BoundedContextRef() SourceText {
	return applicability.boundedContextRef
}

func (applicability Applicability) Assumptions() []SourceText {
	return append([]SourceText(nil), applicability.assumptions...)
}

func (applicability Applicability) Span() SourceLineRange { return applicability.span }

func (applicability Applicability) clone() Applicability {
	applicability.assumptions = applicability.Assumptions()
	return applicability
}
