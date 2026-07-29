package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

const (
	kindClassificationSignatureDomain  = "kind-classification-signature-definition.v1"
	kindClassificationRequestDomain    = "kind-classification-request.v1"
	kindClassificationFeatureDomain    = "governed-candidate-feature.v1"
	kindClassificationFeatureSetDomain = "governed-candidate-feature-set.v1"
	kindClassificationBasisDomain      = "kind-classification-evaluation-basis.v1"
	kindClassificationTrueDomain       = "kind-classification-judgement.true.v1"
	kindClassificationFalseDomain      = "kind-classification-judgement.false.v1"
	kindClassificationUnknownDomain    = "kind-classification-judgement.unknown.v1"
)

// LocalKindRef identifies one project-local U.Kind under one exact TypeEnv and
// bounded context. It is the KindSignature EntityOfConcern, not the candidate
// ValueKind and not a set of candidate entities.
type LocalKindRef struct {
	valueKind ValueKindRef
	context   BoundedContextRef
}

func NewLocalKindRef(
	valueKind ValueKindRef,
	context BoundedContextRef,
) (LocalKindRef, error) {
	if !valueKind.valid() {
		return LocalKindRef{}, fmt.Errorf("local kind ValueKindRef is required")
	}
	if !context.valid() {
		return LocalKindRef{}, fmt.Errorf("local kind bounded context is required")
	}
	return LocalKindRef{valueKind: valueKind, context: context}, nil
}

func (ref LocalKindRef) ValueKind() ValueKindRef { return ref.valueKind }

func (ref LocalKindRef) Context() BoundedContextRef { return ref.context }

func (ref LocalKindRef) TypeEnv() TypeEnvRef { return ref.valueKind.TypeEnv() }

func (ref LocalKindRef) String() string {
	return ref.valueKind.String() + "/local-context/" + ref.context.String()
}

func (ref LocalKindRef) valid() bool {
	return ref.valueKind.valid() && ref.context.valid()
}

// KindClassificationCandidate is the closed candidate algebra. Entity
// identity and already-verified typed values are disjoint variants; neither a
// carrier nor an evidence record can masquerade as the candidate.
type KindClassificationCandidate interface {
	Kind() KindClassificationCandidateKind
	ValueKind() ValueKindRef
	CanonicalBytes() []byte
	Digest() SHA256Digest
	kindClassificationCandidateVariant()
}

type KindClassificationCandidateKind uint8

const (
	KindEntityCandidate KindClassificationCandidateKind = iota + 1
	KindTypedValueCandidate
)

func (kind KindClassificationCandidateKind) String() string {
	switch kind {
	case KindEntityCandidate:
		return "entity"
	case KindTypedValueCandidate:
		return "typed_value"
	default:
		return ""
	}
}

type ExactKindEntityCandidate struct {
	entity         EntityID
	valueKind      ValueKindRef
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewExactKindEntityCandidate(
	entity EntityID,
	valueKind ValueKindRef,
) (ExactKindEntityCandidate, error) {
	if !entity.valid() {
		return ExactKindEntityCandidate{}, fmt.Errorf("kind-classification entity candidate requires an EntityID")
	}
	if !valueKind.valid() {
		return ExactKindEntityCandidate{}, fmt.Errorf("kind-classification entity candidate requires a ValueKindRef")
	}
	writer := newCanonicalWriter("kind-classification-candidate.entity.v1")
	writer.addString(entity.String())
	writer.addString(valueKind.String())
	return ExactKindEntityCandidate{
		entity:         entity,
		valueKind:      valueKind,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (ExactKindEntityCandidate) Kind() KindClassificationCandidateKind {
	return KindEntityCandidate
}

func (candidate ExactKindEntityCandidate) EntityID() EntityID { return candidate.entity }

func (candidate ExactKindEntityCandidate) ValueKind() ValueKindRef {
	return candidate.valueKind
}

func (candidate ExactKindEntityCandidate) CanonicalBytes() []byte {
	return append([]byte(nil), candidate.canonicalBytes...)
}

func (candidate ExactKindEntityCandidate) Digest() SHA256Digest { return candidate.digest }

func (ExactKindEntityCandidate) kindClassificationCandidateVariant() {}

type ExactKindTypedValueCandidate struct {
	value          VerifiedTypedValue
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewExactKindTypedValueCandidate(
	value VerifiedTypedValue,
) (ExactKindTypedValueCandidate, error) {
	if !validVerifiedTypedValue(value) {
		return ExactKindTypedValueCandidate{}, fmt.Errorf("kind-classification value candidate must be a verified typed value")
	}
	writer := newCanonicalWriter("kind-classification-candidate.typed-value.v1")
	writer.addString(value.ValueKind().String())
	writer.addString(value.ValueShape().String())
	writer.addString(value.Codec().String())
	writer.addString(value.Digest().String())
	writer.addBytes(value.CanonicalBytes())
	return ExactKindTypedValueCandidate{
		value:          value,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (ExactKindTypedValueCandidate) Kind() KindClassificationCandidateKind {
	return KindTypedValueCandidate
}

func (candidate ExactKindTypedValueCandidate) Value() VerifiedTypedValue {
	return candidate.value
}

func (candidate ExactKindTypedValueCandidate) ValueKind() ValueKindRef {
	return candidate.value.ValueKind()
}

func (candidate ExactKindTypedValueCandidate) CanonicalBytes() []byte {
	return append([]byte(nil), candidate.canonicalBytes...)
}

func (candidate ExactKindTypedValueCandidate) Digest() SHA256Digest {
	return candidate.digest
}

func (ExactKindTypedValueCandidate) kindClassificationCandidateVariant() {}

func validKindClassificationCandidate(candidate KindClassificationCandidate) bool {
	switch value := candidate.(type) {
	case ExactKindEntityCandidate:
		rebuilt, err := NewExactKindEntityCandidate(value.entity, value.valueKind)
		return err == nil &&
			rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonicalBytes, value.canonicalBytes)
	case ExactKindTypedValueCandidate:
		rebuilt, err := NewExactKindTypedValueCandidate(value.value)
		return err == nil &&
			rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonicalBytes, value.canonicalBytes)
	default:
		return false
	}
}

type KindReferenceSchemePin struct {
	versioned VersionedPin
}

func NewKindReferenceSchemePin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (KindReferenceSchemePin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return KindReferenceSchemePin{}, fmt.Errorf("kind reference scheme: %w", err)
	}
	return KindReferenceSchemePin{versioned: pin}, nil
}

func (pin KindReferenceSchemePin) Reference() CarrierRef { return pin.versioned.Reference() }

func (pin KindReferenceSchemePin) Edition() CarrierEdition { return pin.versioned.Edition() }

func (pin KindReferenceSchemePin) Digest() SHA256Digest { return pin.versioned.Digest() }

func (pin KindReferenceSchemePin) CanonicalBytes() []byte {
	return pin.versioned.canonicalBytes("kind-reference-scheme-pin.v1")
}

func (pin KindReferenceSchemePin) valid() bool { return pin.versioned.valid() }

type KindSignatureDependencyKind uint8

const (
	KindDependencyAssumption KindSignatureDependencyKind = iota + 1
	KindDependencyExternal
	KindDependencyStandard
	KindDependencyVersion
	KindDependencyUnit
	KindDependencyTemporalPolicy
)

func (kind KindSignatureDependencyKind) String() string {
	switch kind {
	case KindDependencyAssumption:
		return "assumption"
	case KindDependencyExternal:
		return "dependency"
	case KindDependencyStandard:
		return "standard"
	case KindDependencyVersion:
		return "version"
	case KindDependencyUnit:
		return "unit"
	case KindDependencyTemporalPolicy:
		return "temporal_policy"
	default:
		return ""
	}
}

func (kind KindSignatureDependencyKind) valid() bool { return kind.String() != "" }

type KindSignatureDependencyPin struct {
	kind      KindSignatureDependencyKind
	versioned VersionedPin
}

func NewKindSignatureDependencyPin(
	kind KindSignatureDependencyKind,
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (KindSignatureDependencyPin, error) {
	if !kind.valid() {
		return KindSignatureDependencyPin{}, fmt.Errorf("KindSignature dependency kind is required")
	}
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return KindSignatureDependencyPin{}, fmt.Errorf("KindSignature dependency: %w", err)
	}
	return KindSignatureDependencyPin{kind: kind, versioned: pin}, nil
}

func (pin KindSignatureDependencyPin) Kind() KindSignatureDependencyKind {
	return pin.kind
}

func (pin KindSignatureDependencyPin) Reference() CarrierRef {
	return pin.versioned.Reference()
}

func (pin KindSignatureDependencyPin) Edition() CarrierEdition {
	return pin.versioned.Edition()
}

func (pin KindSignatureDependencyPin) Digest() SHA256Digest {
	return pin.versioned.Digest()
}

func (pin KindSignatureDependencyPin) CanonicalBytes() []byte {
	writer := newCanonicalWriter("kind-signature-dependency-pin.v1")
	writer.addString(pin.kind.String())
	writer.addBytes(pin.versioned.canonicalBytes("kind-signature-dependency-coordinate.v1"))
	return writer.bytes()
}

func (pin KindSignatureDependencyPin) Valid() bool { return pin.valid() }

func (pin KindSignatureDependencyPin) valid() bool {
	return pin.kind.valid() && pin.versioned.valid()
}

type KindExtentRuleOption interface {
	CanonicalBytes() []byte
	kindExtentRuleOptionVariant()
}

type NoKindExtentRule struct{}

func (NoKindExtentRule) CanonicalBytes() []byte {
	writer := newCanonicalWriter("kind-extent-rule.none.v1")
	return writer.bytes()
}

func (NoKindExtentRule) kindExtentRuleOptionVariant() {}

type DeclaredKindExtentRule struct {
	rule RuleRef
}

func NewDeclaredKindExtentRule(rule RuleRef) (DeclaredKindExtentRule, error) {
	if !rule.valid() {
		return DeclaredKindExtentRule{}, fmt.Errorf("declared Kind ExtentRule is required")
	}
	return DeclaredKindExtentRule{rule: rule}, nil
}

func (rule DeclaredKindExtentRule) RuleRef() RuleRef { return rule.rule }

func (rule DeclaredKindExtentRule) CanonicalBytes() []byte {
	writer := newCanonicalWriter("kind-extent-rule.declared.v1")
	writer.addString(rule.rule.String())
	return writer.bytes()
}

func (DeclaredKindExtentRule) kindExtentRuleOptionVariant() {}

func validKindExtentRuleOption(option KindExtentRuleOption) bool {
	switch value := option.(type) {
	case NoKindExtentRule:
		return len(value.CanonicalBytes()) > 0
	case DeclaredKindExtentRule:
		return value.rule.valid()
	default:
		return false
	}
}

type KindClassificationSignatureRef struct {
	localKind LocalKindRef
	digest    SHA256Digest
}

func NewKindClassificationSignatureRef(
	localKind LocalKindRef,
	digest SHA256Digest,
) (KindClassificationSignatureRef, error) {
	if !localKind.valid() {
		return KindClassificationSignatureRef{}, fmt.Errorf("KindSignature local kind is required")
	}
	if !digest.valid() {
		return KindClassificationSignatureRef{}, fmt.Errorf("KindSignature edition digest is required")
	}
	return KindClassificationSignatureRef{localKind: localKind, digest: digest}, nil
}

func (ref KindClassificationSignatureRef) LocalKind() LocalKindRef { return ref.localKind }

func (ref KindClassificationSignatureRef) TypeEnv() TypeEnvRef {
	return ref.localKind.TypeEnv()
}

func (ref KindClassificationSignatureRef) Digest() SHA256Digest { return ref.digest }

func (ref KindClassificationSignatureRef) String() string {
	return ref.localKind.String() + "/kind-signature/" + ref.digest.String()
}

func (ref KindClassificationSignatureRef) valid() bool {
	return ref.localKind.valid() && ref.digest.valid()
}

type KindClassificationSignatureDefinitionInput struct {
	LocalKind          LocalKindRef
	CandidateValueKind ValueKindRef
	Criterion          RuleRef
	SliceConditions    RuleRef
	ReferenceScheme    KindReferenceSchemePin
	Dependencies       []KindSignatureDependencyPin
	Formality          SignatureFormality
	ExtentRule         KindExtentRuleOption
	Provenance         DeclarationProvenance
}

// KindClassificationSignatureDefinition is the current C.3.2 KindSignature
// declaration used for one-candidate classification. It deliberately has no
// EntitySet coordinate and does not materialize an extension.
type KindClassificationSignatureDefinition struct {
	reference          KindClassificationSignatureRef
	candidateValueKind ValueKindRef
	criterion          RuleRef
	sliceConditions    RuleRef
	referenceScheme    KindReferenceSchemePin
	dependencies       []KindSignatureDependencyPin
	formality          SignatureFormality
	extentRule         KindExtentRuleOption
	provenance         DeclarationProvenance
	canonicalBytes     []byte
}

func NewKindClassificationSignatureDefinition(
	input KindClassificationSignatureDefinitionInput,
) (KindClassificationSignatureDefinition, error) {
	if !input.LocalKind.valid() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature exact local kind is required")
	}
	if !input.CandidateValueKind.valid() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature candidate ValueKind is required")
	}
	if input.LocalKind.TypeEnv() != input.CandidateValueKind.TypeEnv() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature local kind and candidate ValueKind must use one TypeEnv")
	}
	if !input.Criterion.valid() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature direct-feature criterion is required")
	}
	if !input.SliceConditions.valid() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature ContextSlice conditions are required")
	}
	if !input.ReferenceScheme.valid() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature effective ReferenceScheme is required")
	}
	if !input.Formality.valid() {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature formality must be in F0..F9")
	}
	if !validKindExtentRuleOption(input.ExtentRule) {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature requires an explicit ExtentRule posture")
	}
	if !validDeclarationProvenance(input.Provenance) {
		return KindClassificationSignatureDefinition{}, fmt.Errorf("KindSignature provenance is required")
	}
	dependencies, err := normalizeKindSignatureDependencies(input.Dependencies)
	if err != nil {
		return KindClassificationSignatureDefinition{}, err
	}
	writer := canonicalKindClassificationSignatureDefinition(input, dependencies)
	reference, err := NewKindClassificationSignatureRef(input.LocalKind, writer.digest())
	if err != nil {
		return KindClassificationSignatureDefinition{}, err
	}
	return KindClassificationSignatureDefinition{
		reference:          reference,
		candidateValueKind: input.CandidateValueKind,
		criterion:          input.Criterion,
		sliceConditions:    input.SliceConditions,
		referenceScheme:    input.ReferenceScheme,
		dependencies:       dependencies,
		formality:          input.Formality,
		extentRule:         input.ExtentRule,
		provenance:         input.Provenance,
		canonicalBytes:     writer.bytes(),
	}, nil
}

func (definition KindClassificationSignatureDefinition) Ref() KindClassificationSignatureRef {
	return definition.reference
}

func (definition KindClassificationSignatureDefinition) LocalKind() LocalKindRef {
	return definition.reference.LocalKind()
}

func (definition KindClassificationSignatureDefinition) CandidateValueKind() ValueKindRef {
	return definition.candidateValueKind
}

func (definition KindClassificationSignatureDefinition) Criterion() RuleRef {
	return definition.criterion
}

func (definition KindClassificationSignatureDefinition) SliceConditions() RuleRef {
	return definition.sliceConditions
}

func (definition KindClassificationSignatureDefinition) ReferenceScheme() KindReferenceSchemePin {
	return definition.referenceScheme
}

func (definition KindClassificationSignatureDefinition) Dependencies() []KindSignatureDependencyPin {
	return append([]KindSignatureDependencyPin(nil), definition.dependencies...)
}

func (definition KindClassificationSignatureDefinition) Formality() SignatureFormality {
	return definition.formality
}

func (definition KindClassificationSignatureDefinition) ExtentRule() KindExtentRuleOption {
	return definition.extentRule
}

func (definition KindClassificationSignatureDefinition) Provenance() DeclarationProvenance {
	return definition.provenance
}

func (definition KindClassificationSignatureDefinition) CanonicalBytes() []byte {
	return append([]byte(nil), definition.canonicalBytes...)
}

func (definition KindClassificationSignatureDefinition) Valid() bool {
	return definition.valid()
}

func (definition KindClassificationSignatureDefinition) valid() bool {
	input := KindClassificationSignatureDefinitionInput{
		LocalKind:          definition.reference.LocalKind(),
		CandidateValueKind: definition.candidateValueKind,
		Criterion:          definition.criterion,
		SliceConditions:    definition.sliceConditions,
		ReferenceScheme:    definition.referenceScheme,
		Dependencies:       definition.dependencies,
		Formality:          definition.formality,
		ExtentRule:         definition.extentRule,
		Provenance:         definition.provenance,
	}
	rebuilt, err := NewKindClassificationSignatureDefinition(input)
	return err == nil &&
		rebuilt.reference == definition.reference &&
		bytes.Equal(rebuilt.canonicalBytes, definition.canonicalBytes)
}

func canonicalKindClassificationSignatureDefinition(
	input KindClassificationSignatureDefinitionInput,
	dependencies []KindSignatureDependencyPin,
) canonicalWriter {
	writer := newCanonicalWriter(kindClassificationSignatureDomain)
	writer.addString(input.LocalKind.String())
	writer.addString(input.CandidateValueKind.String())
	writer.addString(input.Criterion.String())
	writer.addString(input.SliceConditions.String())
	writer.addBytes(input.ReferenceScheme.CanonicalBytes())
	writer.addUint64(uint64(len(dependencies)))
	for _, dependency := range dependencies {
		writer.addBytes(dependency.CanonicalBytes())
	}
	writer.addString(input.Formality.String())
	writer.addBytes(input.ExtentRule.CanonicalBytes())
	writer.addBytes(input.Provenance.CanonicalBytes())
	return writer
}

func normalizeKindSignatureDependencies(
	values []KindSignatureDependencyPin,
) ([]KindSignatureDependencyPin, error) {
	result := append([]KindSignatureDependencyPin(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]KindSignatureDependencyPin, 0, len(result))
	for _, dependency := range result {
		if !dependency.valid() {
			return nil, fmt.Errorf("KindSignature dependency is invalid")
		}
		if len(normalized) == 0 {
			normalized = append(normalized, dependency)
			continue
		}
		previous := normalized[len(normalized)-1]
		if previous.Kind() != dependency.Kind() || previous.Reference() != dependency.Reference() {
			normalized = append(normalized, dependency)
			continue
		}
		if bytes.Equal(previous.CanonicalBytes(), dependency.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"KindSignature dependency %s:%s has conflicting exact pins",
			dependency.Kind().String(),
			dependency.Reference().String(),
		)
	}
	return normalized, nil
}

type KindClassificationRequestInput struct {
	Candidate        KindClassificationCandidate
	LocalKind        LocalKindRef
	SignatureEdition KindClassificationSignatureRef
	ContextSlice     ContextSlice
}

// KindClassificationRequest is exactly the current C.3.2 four-input
// judgement coordinate: candidate, local kind, KindSignature edition, and
// ContextSlice. No evidence or receiving-guard state is part of this request.
type KindClassificationRequest struct {
	candidate        KindClassificationCandidate
	localKind        LocalKindRef
	signatureEdition KindClassificationSignatureRef
	contextSlice     ContextSlice
	canonicalBytes   []byte
	digest           SHA256Digest
}

func NewKindClassificationRequest(
	input KindClassificationRequestInput,
) (KindClassificationRequest, error) {
	if !validKindClassificationCandidate(input.Candidate) {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification candidate is invalid")
	}
	if !input.LocalKind.valid() {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification local kind is required")
	}
	if !input.SignatureEdition.valid() {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification KindSignature edition is required")
	}
	if !validCompleteContextSlice(input.ContextSlice) {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification ContextSlice must be exact and complete")
	}
	if input.SignatureEdition.LocalKind() != input.LocalKind {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification signature edition belongs to another local kind")
	}
	if input.ContextSlice.Context() != input.LocalKind.Context() {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification ContextSlice belongs to another bounded context")
	}
	if input.Candidate.ValueKind().TypeEnv() != input.LocalKind.TypeEnv() {
		return KindClassificationRequest{}, fmt.Errorf("kind-classification candidate and local kind belong to different TypeEnvs")
	}
	writer := canonicalKindClassificationRequest(input)
	return KindClassificationRequest{
		candidate:        input.Candidate,
		localKind:        input.LocalKind,
		signatureEdition: input.SignatureEdition,
		contextSlice:     input.ContextSlice,
		canonicalBytes:   writer.bytes(),
		digest:           writer.digest(),
	}, nil
}

func (request KindClassificationRequest) Candidate() KindClassificationCandidate {
	return request.candidate
}

func (request KindClassificationRequest) LocalKind() LocalKindRef { return request.localKind }

func (request KindClassificationRequest) SignatureEdition() KindClassificationSignatureRef {
	return request.signatureEdition
}

func (request KindClassificationRequest) ContextSlice() ContextSlice {
	return request.contextSlice
}

func (request KindClassificationRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonicalBytes...)
}

func (request KindClassificationRequest) Digest() SHA256Digest { return request.digest }

func (request KindClassificationRequest) Valid() bool { return request.valid() }

func (request KindClassificationRequest) valid() bool {
	input := KindClassificationRequestInput{
		Candidate:        request.candidate,
		LocalKind:        request.localKind,
		SignatureEdition: request.signatureEdition,
		ContextSlice:     request.contextSlice,
	}
	rebuilt, err := NewKindClassificationRequest(input)
	return err == nil &&
		rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonicalBytes, request.canonicalBytes)
}

func canonicalKindClassificationRequest(input KindClassificationRequestInput) canonicalWriter {
	writer := newCanonicalWriter(kindClassificationRequestDomain)
	writer.addBytes(input.Candidate.CanonicalBytes())
	writer.addString(input.LocalKind.String())
	writer.addString(input.SignatureEdition.String())
	writer.addBytes(input.ContextSlice.CanonicalBytes())
	return writer
}

type KindFeatureKey struct {
	value string
}

func NewKindFeatureKey(raw string) (KindFeatureKey, error) {
	value, err := parseQualifiedIdentifier("governed candidate feature key", raw)
	if err != nil {
		return KindFeatureKey{}, err
	}
	return KindFeatureKey{value: value}, nil
}

func (key KindFeatureKey) String() string { return key.value }

func (key KindFeatureKey) valid() bool { return key.value != "" }

type GovernedCandidateFeatureInput struct {
	Key          KindFeatureKey
	Value        VerifiedTypedValue
	Governor     RuleRef
	Source       CarrierRef
	SourceDigest SHA256Digest
}

// GovernedCandidateFeature is an exact candidate-side fact input. Source
// delivery makes the fact inspectable; it is not Evidence and does not make a
// criterion true by its mere presence.
type GovernedCandidateFeature struct {
	key            KindFeatureKey
	value          VerifiedTypedValue
	governor       RuleRef
	source         CarrierRef
	sourceDigest   SHA256Digest
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewGovernedCandidateFeature(
	input GovernedCandidateFeatureInput,
) (GovernedCandidateFeature, error) {
	if !input.Key.valid() {
		return GovernedCandidateFeature{}, fmt.Errorf("governed candidate feature key is required")
	}
	if !validVerifiedTypedValue(input.Value) {
		return GovernedCandidateFeature{}, fmt.Errorf("governed candidate feature requires a verified typed value")
	}
	if !input.Governor.valid() {
		return GovernedCandidateFeature{}, fmt.Errorf("governed candidate feature direct governor is required")
	}
	if !input.Source.valid() {
		return GovernedCandidateFeature{}, fmt.Errorf("governed candidate feature source is required")
	}
	if !input.SourceDigest.valid() {
		return GovernedCandidateFeature{}, fmt.Errorf("governed candidate feature source digest is required")
	}
	writer := newCanonicalWriter(kindClassificationFeatureDomain)
	writer.addString(input.Key.String())
	writer.addString(input.Value.ValueKind().String())
	writer.addString(input.Value.ValueShape().String())
	writer.addString(input.Value.Codec().String())
	writer.addString(input.Value.Digest().String())
	writer.addBytes(input.Value.CanonicalBytes())
	writer.addString(input.Governor.String())
	writer.addString(input.Source.String())
	writer.addString(input.SourceDigest.String())
	return GovernedCandidateFeature{
		key:            input.Key,
		value:          input.Value,
		governor:       input.Governor,
		source:         input.Source,
		sourceDigest:   input.SourceDigest,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (feature GovernedCandidateFeature) Key() KindFeatureKey { return feature.key }

func (feature GovernedCandidateFeature) Value() VerifiedTypedValue { return feature.value }

func (feature GovernedCandidateFeature) Governor() RuleRef { return feature.governor }

func (feature GovernedCandidateFeature) Source() CarrierRef { return feature.source }

func (feature GovernedCandidateFeature) SourceDigest() SHA256Digest {
	return feature.sourceDigest
}

func (feature GovernedCandidateFeature) CanonicalBytes() []byte {
	return append([]byte(nil), feature.canonicalBytes...)
}

func (feature GovernedCandidateFeature) Digest() SHA256Digest { return feature.digest }

func (feature GovernedCandidateFeature) valid() bool {
	rebuilt, err := NewGovernedCandidateFeature(GovernedCandidateFeatureInput{
		Key:          feature.key,
		Value:        feature.value,
		Governor:     feature.governor,
		Source:       feature.source,
		SourceDigest: feature.sourceDigest,
	})
	return err == nil &&
		rebuilt.digest == feature.digest &&
		bytes.Equal(rebuilt.canonicalBytes, feature.canonicalBytes)
}

type GovernedCandidateFeatureSet struct {
	requestDigest  SHA256Digest
	features       []GovernedCandidateFeature
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewGovernedCandidateFeatureSet(
	request KindClassificationRequest,
	features []GovernedCandidateFeature,
) (GovernedCandidateFeatureSet, error) {
	if !request.valid() {
		return GovernedCandidateFeatureSet{}, fmt.Errorf("governed feature set requires an exact classification request")
	}
	if len(features) == 0 {
		return GovernedCandidateFeatureSet{}, fmt.Errorf("governed feature set requires at least one direct candidate feature")
	}
	normalized, err := normalizeGovernedCandidateFeatures(request, features)
	if err != nil {
		return GovernedCandidateFeatureSet{}, err
	}
	writer := newCanonicalWriter(kindClassificationFeatureSetDomain)
	writer.addString(request.Digest().String())
	writer.addUint64(uint64(len(normalized)))
	for _, feature := range normalized {
		writer.addBytes(feature.CanonicalBytes())
	}
	return GovernedCandidateFeatureSet{
		requestDigest:  request.Digest(),
		features:       normalized,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (set GovernedCandidateFeatureSet) RequestDigest() SHA256Digest {
	return set.requestDigest
}

func (set GovernedCandidateFeatureSet) Features() []GovernedCandidateFeature {
	return append([]GovernedCandidateFeature(nil), set.features...)
}

func (set GovernedCandidateFeatureSet) Feature(
	key KindFeatureKey,
) (GovernedCandidateFeature, bool) {
	index := sort.Search(len(set.features), func(index int) bool {
		return set.features[index].Key().String() >= key.String()
	})
	if index >= len(set.features) || set.features[index].Key() != key {
		return GovernedCandidateFeature{}, false
	}
	return set.features[index], true
}

func (set GovernedCandidateFeatureSet) CanonicalBytes() []byte {
	return append([]byte(nil), set.canonicalBytes...)
}

func (set GovernedCandidateFeatureSet) Digest() SHA256Digest { return set.digest }

func (set GovernedCandidateFeatureSet) ValidFor(
	request KindClassificationRequest,
) bool {
	return set.validFor(request)
}

func (set GovernedCandidateFeatureSet) validFor(request KindClassificationRequest) bool {
	rebuilt, err := NewGovernedCandidateFeatureSet(request, set.features)
	return err == nil &&
		rebuilt.requestDigest == set.requestDigest &&
		rebuilt.digest == set.digest &&
		bytes.Equal(rebuilt.canonicalBytes, set.canonicalBytes)
}

func normalizeGovernedCandidateFeatures(
	request KindClassificationRequest,
	values []GovernedCandidateFeature,
) ([]GovernedCandidateFeature, error) {
	result := append([]GovernedCandidateFeature(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		if result[left].Key() != result[right].Key() {
			return result[left].Key().String() < result[right].Key().String()
		}
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]GovernedCandidateFeature, 0, len(result))
	for _, feature := range result {
		if !feature.valid() {
			return nil, fmt.Errorf("governed feature set contains an invalid feature")
		}
		if feature.Value().ValueKind().TypeEnv() != request.LocalKind().TypeEnv() {
			return nil, fmt.Errorf("governed feature %q belongs to another TypeEnv", feature.Key().String())
		}
		if len(normalized) == 0 {
			normalized = append(normalized, feature)
			continue
		}
		previous := normalized[len(normalized)-1]
		if previous.Key() != feature.Key() {
			normalized = append(normalized, feature)
			continue
		}
		if bytes.Equal(previous.CanonicalBytes(), feature.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf("governed feature %q has conflicting exact values", feature.Key().String())
	}
	return normalized, nil
}

// KindClassificationEvaluationBasis joins a current KindSignature edition to
// exact governed candidate features. It excludes Evidence and guard state.
type KindClassificationEvaluationBasis struct {
	requestDigest    SHA256Digest
	signatureEdition KindClassificationSignatureRef
	criterion        RuleRef
	featureSet       GovernedCandidateFeatureSet
	canonicalBytes   []byte
	digest           SHA256Digest
}

func NewKindClassificationEvaluationBasis(
	request KindClassificationRequest,
	signature KindClassificationSignatureDefinition,
	features GovernedCandidateFeatureSet,
) (KindClassificationEvaluationBasis, error) {
	if !request.valid() {
		return KindClassificationEvaluationBasis{}, fmt.Errorf("classification basis requires an exact request")
	}
	if !signature.valid() {
		return KindClassificationEvaluationBasis{}, fmt.Errorf("classification basis requires an exact KindSignature")
	}
	if request.SignatureEdition() != signature.Ref() {
		return KindClassificationEvaluationBasis{}, fmt.Errorf("classification basis KindSignature does not match the request edition")
	}
	if request.Candidate().ValueKind() != signature.CandidateValueKind() {
		return KindClassificationEvaluationBasis{}, fmt.Errorf("classification candidate ValueKind does not match the KindSignature")
	}
	if !features.validFor(request) {
		return KindClassificationEvaluationBasis{}, fmt.Errorf("classification feature set does not match the exact request")
	}
	writer := canonicalKindClassificationEvaluationBasis(
		request.Digest(),
		signature.Ref(),
		signature.Criterion(),
		features,
	)
	return KindClassificationEvaluationBasis{
		requestDigest:    request.Digest(),
		signatureEdition: signature.Ref(),
		criterion:        signature.Criterion(),
		featureSet:       features,
		canonicalBytes:   writer.bytes(),
		digest:           writer.digest(),
	}, nil
}

func (basis KindClassificationEvaluationBasis) RequestDigest() SHA256Digest {
	return basis.requestDigest
}

func (basis KindClassificationEvaluationBasis) SignatureEdition() KindClassificationSignatureRef {
	return basis.signatureEdition
}

func (basis KindClassificationEvaluationBasis) Criterion() RuleRef { return basis.criterion }

func (basis KindClassificationEvaluationBasis) FeatureSet() GovernedCandidateFeatureSet {
	return basis.featureSet
}

func (basis KindClassificationEvaluationBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonicalBytes...)
}

func (basis KindClassificationEvaluationBasis) Digest() SHA256Digest { return basis.digest }

func (basis KindClassificationEvaluationBasis) ValidFor(
	request KindClassificationRequest,
) bool {
	return basis.validFor(request)
}

func (basis KindClassificationEvaluationBasis) validFor(
	request KindClassificationRequest,
) bool {
	if !request.valid() || basis.requestDigest != request.Digest() {
		return false
	}
	if basis.signatureEdition != request.SignatureEdition() ||
		!basis.criterion.valid() ||
		!basis.featureSet.validFor(request) {
		return false
	}
	writer := canonicalKindClassificationEvaluationBasis(
		basis.requestDigest,
		basis.signatureEdition,
		basis.criterion,
		basis.featureSet,
	)
	return writer.digest() == basis.digest &&
		bytes.Equal(writer.bytes(), basis.canonicalBytes)
}

func canonicalKindClassificationEvaluationBasis(
	requestDigest SHA256Digest,
	signatureEdition KindClassificationSignatureRef,
	criterion RuleRef,
	features GovernedCandidateFeatureSet,
) canonicalWriter {
	writer := newCanonicalWriter(kindClassificationBasisDomain)
	writer.addString(requestDigest.String())
	writer.addString(signatureEdition.String())
	writer.addString(criterion.String())
	writer.addBytes(features.CanonicalBytes())
	return writer
}

type KindClassificationJudgementKind string

const (
	KindClassificationTrue    KindClassificationJudgementKind = "true"
	KindClassificationFalse   KindClassificationJudgementKind = "false"
	KindClassificationUnknown KindClassificationJudgementKind = "unknown"
)

func (kind KindClassificationJudgementKind) String() string { return string(kind) }

type KindClassificationJudgement interface {
	Kind() KindClassificationJudgementKind
	Request() KindClassificationRequest
	CanonicalBytes() []byte
	Digest() SHA256Digest
	kindClassificationJudgementVariant()
}

type TrueKindClassification struct {
	request        KindClassificationRequest
	basis          KindClassificationEvaluationBasis
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewTrueKindClassification(
	request KindClassificationRequest,
	basis KindClassificationEvaluationBasis,
) (TrueKindClassification, error) {
	if !request.valid() || !basis.validFor(request) {
		return TrueKindClassification{}, fmt.Errorf("true classification requires an exact request and direct-feature basis")
	}
	writer := canonicalSettledKindClassification(kindClassificationTrueDomain, request, basis)
	return TrueKindClassification{
		request:        request,
		basis:          basis,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (TrueKindClassification) Kind() KindClassificationJudgementKind {
	return KindClassificationTrue
}

func (judgement TrueKindClassification) Request() KindClassificationRequest {
	return judgement.request
}

func (judgement TrueKindClassification) Basis() KindClassificationEvaluationBasis {
	return judgement.basis
}

func (judgement TrueKindClassification) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement TrueKindClassification) Digest() SHA256Digest { return judgement.digest }

func (TrueKindClassification) kindClassificationJudgementVariant() {}

type FalseKindClassification struct {
	request        KindClassificationRequest
	basis          KindClassificationEvaluationBasis
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewFalseKindClassification(
	request KindClassificationRequest,
	basis KindClassificationEvaluationBasis,
) (FalseKindClassification, error) {
	if !request.valid() || !basis.validFor(request) {
		return FalseKindClassification{}, fmt.Errorf("false classification requires an exact request and direct-feature basis")
	}
	writer := canonicalSettledKindClassification(kindClassificationFalseDomain, request, basis)
	return FalseKindClassification{
		request:        request,
		basis:          basis,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (FalseKindClassification) Kind() KindClassificationJudgementKind {
	return KindClassificationFalse
}

func (judgement FalseKindClassification) Request() KindClassificationRequest {
	return judgement.request
}

func (judgement FalseKindClassification) Basis() KindClassificationEvaluationBasis {
	return judgement.basis
}

func (judgement FalseKindClassification) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement FalseKindClassification) Digest() SHA256Digest { return judgement.digest }

func (FalseKindClassification) kindClassificationJudgementVariant() {}

func canonicalSettledKindClassification(
	domain string,
	request KindClassificationRequest,
	basis KindClassificationEvaluationBasis,
) canonicalWriter {
	writer := newCanonicalWriter(domain)
	writer.addBytes(request.CanonicalBytes())
	writer.addBytes(basis.CanonicalBytes())
	return writer
}

type KindClassificationUnknownReasonKind uint8

const (
	KindUnknownMissingGovernedFeature KindClassificationUnknownReasonKind = iota + 1
	KindUnknownDependencyUnavailable
	KindUnknownCandidateOutsideDomain
	KindUnknownCriterionUnavailable
	KindUnknownFeatureSourceUnavailable
	KindUnknownFeatureSourceMalformed
	KindUnknownFeatureSourceUntrusted
	KindUnknownFeatureSourceMismatch
	KindUnknownSupportingEvidenceUnavailable
)

func (kind KindClassificationUnknownReasonKind) String() string {
	switch kind {
	case KindUnknownMissingGovernedFeature:
		return "missing_governed_feature"
	case KindUnknownDependencyUnavailable:
		return "dependency_unavailable"
	case KindUnknownCandidateOutsideDomain:
		return "candidate_outside_declared_domain"
	case KindUnknownCriterionUnavailable:
		return "criterion_unavailable"
	case KindUnknownFeatureSourceUnavailable:
		return "feature_source_unavailable"
	case KindUnknownFeatureSourceMalformed:
		return "feature_source_malformed"
	case KindUnknownFeatureSourceUntrusted:
		return "feature_source_untrusted"
	case KindUnknownFeatureSourceMismatch:
		return "feature_source_mismatch"
	case KindUnknownSupportingEvidenceUnavailable:
		return "supporting_evidence_unavailable"
	default:
		return ""
	}
}

func (kind KindClassificationUnknownReasonKind) valid() bool { return kind.String() != "" }

type KindClassificationUnknownReason struct {
	kind   KindClassificationUnknownReasonKind
	repair RepairPointer
}

func NewKindClassificationUnknownReason(
	kind KindClassificationUnknownReasonKind,
	repair RepairPointer,
) (KindClassificationUnknownReason, error) {
	if !kind.valid() {
		return KindClassificationUnknownReason{}, fmt.Errorf("classification unknown reason kind is required")
	}
	if !repair.valid() {
		return KindClassificationUnknownReason{}, fmt.Errorf("classification unknown reason repair pointer is required")
	}
	return KindClassificationUnknownReason{kind: kind, repair: repair}, nil
}

func (reason KindClassificationUnknownReason) Kind() KindClassificationUnknownReasonKind {
	return reason.kind
}

func (reason KindClassificationUnknownReason) RepairPointer() RepairPointer {
	return reason.repair
}

func (reason KindClassificationUnknownReason) CanonicalBytes() []byte {
	writer := newCanonicalWriter("kind-classification-unknown-reason.v1")
	writer.addString(reason.kind.String())
	writer.addString(reason.repair.String())
	return writer.bytes()
}

func (reason KindClassificationUnknownReason) valid() bool {
	return reason.kind.valid() && reason.repair.valid()
}

type UnknownKindClassification struct {
	request        KindClassificationRequest
	reasons        []KindClassificationUnknownReason
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewUnknownKindClassification(
	request KindClassificationRequest,
	reasons []KindClassificationUnknownReason,
) (UnknownKindClassification, error) {
	if !request.valid() {
		return UnknownKindClassification{}, fmt.Errorf("unknown classification requires an exact request")
	}
	normalized, err := normalizeKindClassificationUnknownReasons(reasons)
	if err != nil {
		return UnknownKindClassification{}, err
	}
	writer := newCanonicalWriter(kindClassificationUnknownDomain)
	writer.addBytes(request.CanonicalBytes())
	writer.addUint64(uint64(len(normalized)))
	for _, reason := range normalized {
		writer.addBytes(reason.CanonicalBytes())
	}
	return UnknownKindClassification{
		request:        request,
		reasons:        normalized,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (UnknownKindClassification) Kind() KindClassificationJudgementKind {
	return KindClassificationUnknown
}

func (judgement UnknownKindClassification) Request() KindClassificationRequest {
	return judgement.request
}

func (judgement UnknownKindClassification) Reasons() []KindClassificationUnknownReason {
	return append([]KindClassificationUnknownReason(nil), judgement.reasons...)
}

func (judgement UnknownKindClassification) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement UnknownKindClassification) Digest() SHA256Digest {
	return judgement.digest
}

func (UnknownKindClassification) kindClassificationJudgementVariant() {}

func normalizeKindClassificationUnknownReasons(
	values []KindClassificationUnknownReason,
) ([]KindClassificationUnknownReason, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("unknown classification requires at least one exact reason")
	}
	result := append([]KindClassificationUnknownReason(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]KindClassificationUnknownReason, 0, len(result))
	for _, reason := range result {
		if !reason.valid() {
			return nil, fmt.Errorf("unknown classification contains an invalid reason")
		}
		if len(normalized) == 0 || !bytes.Equal(
			normalized[len(normalized)-1].CanonicalBytes(),
			reason.CanonicalBytes(),
		) {
			normalized = append(normalized, reason)
		}
	}
	return normalized, nil
}

func validKindClassificationJudgement(judgement KindClassificationJudgement) bool {
	switch value := judgement.(type) {
	case TrueKindClassification:
		rebuilt, err := NewTrueKindClassification(value.request, value.basis)
		return err == nil && rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonicalBytes, value.canonicalBytes)
	case FalseKindClassification:
		rebuilt, err := NewFalseKindClassification(value.request, value.basis)
		return err == nil && rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonicalBytes, value.canonicalBytes)
	case UnknownKindClassification:
		rebuilt, err := NewUnknownKindClassification(value.request, value.reasons)
		return err == nil && rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonicalBytes, value.canonicalBytes)
	default:
		return false
	}
}

func KindClassificationJudgementMatchesRequest(
	request KindClassificationRequest,
	judgement KindClassificationJudgement,
) bool {
	return request.valid() &&
		validKindClassificationJudgement(judgement) &&
		judgement.Request().Digest() == request.Digest() &&
		bytes.Equal(judgement.Request().CanonicalBytes(), request.CanonicalBytes())
}

func KindClassificationJudgementValid(
	judgement KindClassificationJudgement,
) bool {
	return validKindClassificationJudgement(judgement)
}
