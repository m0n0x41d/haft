package typedmemory

import (
	"bytes"
	"fmt"
)

type RoleMaskRef struct {
	baseSignature KindClassificationSignatureRef
	digest        SHA256Digest
}

func NewRoleMaskRef(
	baseSignature KindClassificationSignatureRef,
	digest SHA256Digest,
) (RoleMaskRef, error) {
	if !baseSignature.valid() {
		return RoleMaskRef{}, fmt.Errorf("RoleMask base KindSignature edition is required")
	}
	if !digest.valid() {
		return RoleMaskRef{}, fmt.Errorf("RoleMask edition digest is required")
	}
	return RoleMaskRef{baseSignature: baseSignature, digest: digest}, nil
}

func (ref RoleMaskRef) BaseSignature() KindClassificationSignatureRef {
	return ref.baseSignature
}

func (ref RoleMaskRef) Digest() SHA256Digest { return ref.digest }

func (ref RoleMaskRef) String() string {
	return ref.baseSignature.String() + "/role-mask/" + ref.digest.String()
}

func (ref RoleMaskRef) valid() bool {
	return ref.baseSignature.valid() && ref.digest.valid()
}

type RoleMaskDefinitionInput struct {
	BaseSignature    KindClassificationSignatureRef
	FeatureCriterion RuleRef
	ScopeExpectation RuleRef
	Provenance       DeclarationProvenance
}

// RoleMaskDefinition is a named, versioned declaration episteme over one base
// KindSignature. Its direct feature criterion and separate scope expectation
// cannot be represented as one predicate.
type RoleMaskDefinition struct {
	reference        RoleMaskRef
	featureCriterion RuleRef
	scopeExpectation RuleRef
	provenance       DeclarationProvenance
	canonicalBytes   []byte
}

func NewRoleMaskDefinition(
	input RoleMaskDefinitionInput,
) (RoleMaskDefinition, error) {
	if !input.BaseSignature.valid() {
		return RoleMaskDefinition{}, fmt.Errorf("RoleMask base KindSignature edition is required")
	}
	if !input.FeatureCriterion.valid() {
		return RoleMaskDefinition{}, fmt.Errorf("RoleMask direct candidate-feature criterion is required")
	}
	if !input.ScopeExpectation.valid() {
		return RoleMaskDefinition{}, fmt.Errorf("RoleMask separate scope expectation is required")
	}
	if !validDeclarationProvenance(input.Provenance) {
		return RoleMaskDefinition{}, fmt.Errorf("RoleMask declaration provenance is required")
	}
	writer := newCanonicalWriter("role-mask-definition.v1")
	writer.addString(input.BaseSignature.String())
	writer.addString(input.FeatureCriterion.String())
	writer.addString(input.ScopeExpectation.String())
	writer.addBytes(input.Provenance.CanonicalBytes())
	reference, err := NewRoleMaskRef(input.BaseSignature, writer.digest())
	if err != nil {
		return RoleMaskDefinition{}, err
	}
	return RoleMaskDefinition{
		reference:        reference,
		featureCriterion: input.FeatureCriterion,
		scopeExpectation: input.ScopeExpectation,
		provenance:       input.Provenance,
		canonicalBytes:   writer.bytes(),
	}, nil
}

func (definition RoleMaskDefinition) Ref() RoleMaskRef { return definition.reference }

func (definition RoleMaskDefinition) BaseSignature() KindClassificationSignatureRef {
	return definition.reference.BaseSignature()
}

func (definition RoleMaskDefinition) FeatureCriterion() RuleRef {
	return definition.featureCriterion
}

func (definition RoleMaskDefinition) ScopeExpectation() RuleRef {
	return definition.scopeExpectation
}

func (definition RoleMaskDefinition) Provenance() DeclarationProvenance {
	return definition.provenance
}

func (definition RoleMaskDefinition) CanonicalBytes() []byte {
	return append([]byte(nil), definition.canonicalBytes...)
}

func (definition RoleMaskDefinition) valid() bool {
	rebuilt, err := NewRoleMaskDefinition(RoleMaskDefinitionInput{
		BaseSignature:    definition.BaseSignature(),
		FeatureCriterion: definition.featureCriterion,
		ScopeExpectation: definition.scopeExpectation,
		Provenance:       definition.provenance,
	})
	return err == nil && rebuilt.reference == definition.reference &&
		bytes.Equal(rebuilt.canonicalBytes, definition.canonicalBytes)
}

// RoleMaskClassificationRequest adds exactly one RoleMask edition to the
// unchanged four-input base classification request. Scope coverage is not a
// hidden fifth classification input.
type RoleMaskClassificationRequest struct {
	baseRequest    KindClassificationRequest
	roleMask       RoleMaskRef
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewRoleMaskClassificationRequest(
	baseRequest KindClassificationRequest,
	roleMask RoleMaskRef,
) (RoleMaskClassificationRequest, error) {
	if !baseRequest.valid() {
		return RoleMaskClassificationRequest{}, fmt.Errorf("RoleMask judgement requires an exact base classification request")
	}
	if !roleMask.valid() {
		return RoleMaskClassificationRequest{}, fmt.Errorf("RoleMask judgement requires an exact RoleMask edition")
	}
	if roleMask.BaseSignature() != baseRequest.SignatureEdition() {
		return RoleMaskClassificationRequest{}, fmt.Errorf("RoleMask edition belongs to another base KindSignature")
	}
	writer := newCanonicalWriter("role-mask-classification-request.v1")
	writer.addBytes(baseRequest.CanonicalBytes())
	writer.addString(roleMask.String())
	return RoleMaskClassificationRequest{
		baseRequest:    baseRequest,
		roleMask:       roleMask,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (request RoleMaskClassificationRequest) BaseRequest() KindClassificationRequest {
	return request.baseRequest
}

func (request RoleMaskClassificationRequest) RoleMask() RoleMaskRef {
	return request.roleMask
}

func (request RoleMaskClassificationRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonicalBytes...)
}

func (request RoleMaskClassificationRequest) Digest() SHA256Digest { return request.digest }

func (request RoleMaskClassificationRequest) valid() bool {
	rebuilt, err := NewRoleMaskClassificationRequest(request.baseRequest, request.roleMask)
	return err == nil && rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonicalBytes, request.canonicalBytes)
}

type RoleMaskEvaluationBasis struct {
	requestDigest  SHA256Digest
	roleMask       RoleMaskRef
	criterion      RuleRef
	featureSet     GovernedCandidateFeatureSet
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewRoleMaskEvaluationBasis(
	request RoleMaskClassificationRequest,
	definition RoleMaskDefinition,
	features GovernedCandidateFeatureSet,
) (RoleMaskEvaluationBasis, error) {
	if !request.valid() || !definition.valid() {
		return RoleMaskEvaluationBasis{}, fmt.Errorf("RoleMask evaluation basis requires exact request and declaration")
	}
	if request.RoleMask() != definition.Ref() {
		return RoleMaskEvaluationBasis{}, fmt.Errorf("RoleMask evaluation declaration does not match the request edition")
	}
	if !features.validFor(request.BaseRequest()) {
		return RoleMaskEvaluationBasis{}, fmt.Errorf("RoleMask feature set does not match the exact base request")
	}
	writer := canonicalRoleMaskEvaluationBasis(
		request.Digest(),
		definition.Ref(),
		definition.FeatureCriterion(),
		features,
	)
	return RoleMaskEvaluationBasis{
		requestDigest:  request.Digest(),
		roleMask:       definition.Ref(),
		criterion:      definition.FeatureCriterion(),
		featureSet:     features,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (basis RoleMaskEvaluationBasis) RequestDigest() SHA256Digest {
	return basis.requestDigest
}

func (basis RoleMaskEvaluationBasis) RoleMask() RoleMaskRef { return basis.roleMask }

func (basis RoleMaskEvaluationBasis) Criterion() RuleRef { return basis.criterion }

func (basis RoleMaskEvaluationBasis) FeatureSet() GovernedCandidateFeatureSet {
	return basis.featureSet
}

func (basis RoleMaskEvaluationBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonicalBytes...)
}

func (basis RoleMaskEvaluationBasis) Digest() SHA256Digest { return basis.digest }

func (basis RoleMaskEvaluationBasis) validFor(
	request RoleMaskClassificationRequest,
) bool {
	if !request.valid() || basis.requestDigest != request.Digest() {
		return false
	}
	if basis.roleMask != request.RoleMask() ||
		!basis.criterion.valid() ||
		!basis.featureSet.validFor(request.BaseRequest()) {
		return false
	}
	writer := canonicalRoleMaskEvaluationBasis(
		basis.requestDigest,
		basis.roleMask,
		basis.criterion,
		basis.featureSet,
	)
	return writer.digest() == basis.digest &&
		bytes.Equal(writer.bytes(), basis.canonicalBytes)
}

func canonicalRoleMaskEvaluationBasis(
	requestDigest SHA256Digest,
	roleMask RoleMaskRef,
	criterion RuleRef,
	features GovernedCandidateFeatureSet,
) canonicalWriter {
	writer := newCanonicalWriter("role-mask-evaluation-basis.v1")
	writer.addString(requestDigest.String())
	writer.addString(roleMask.String())
	writer.addString(criterion.String())
	writer.addBytes(features.CanonicalBytes())
	return writer
}

type roleMaskJudgementBasis interface {
	CanonicalBytes() []byte
	roleMaskJudgementBasisVariant()
}

type settledRoleMaskJudgementBasis struct {
	basis RoleMaskEvaluationBasis
}

func (basis settledRoleMaskJudgementBasis) CanonicalBytes() []byte {
	return basis.basis.CanonicalBytes()
}

func (settledRoleMaskJudgementBasis) roleMaskJudgementBasisVariant() {}

type unknownRoleMaskJudgementBasis struct {
	reasons []KindClassificationUnknownReason
}

func (basis unknownRoleMaskJudgementBasis) CanonicalBytes() []byte {
	writer := newCanonicalWriter("role-mask-unknown-basis.v1")
	writer.addUint64(uint64(len(basis.reasons)))
	for _, reason := range basis.reasons {
		writer.addBytes(reason.CanonicalBytes())
	}
	return writer.bytes()
}

func (unknownRoleMaskJudgementBasis) roleMaskJudgementBasisVariant() {}

type RoleMaskJudgement struct {
	kind           KindClassificationJudgementKind
	request        RoleMaskClassificationRequest
	basis          roleMaskJudgementBasis
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewTrueRoleMaskJudgement(
	request RoleMaskClassificationRequest,
	basis RoleMaskEvaluationBasis,
) (RoleMaskJudgement, error) {
	return newSettledRoleMaskJudgement(KindClassificationTrue, request, basis)
}

func NewFalseRoleMaskJudgement(
	request RoleMaskClassificationRequest,
	basis RoleMaskEvaluationBasis,
) (RoleMaskJudgement, error) {
	return newSettledRoleMaskJudgement(KindClassificationFalse, request, basis)
}

func newSettledRoleMaskJudgement(
	kind KindClassificationJudgementKind,
	request RoleMaskClassificationRequest,
	basis RoleMaskEvaluationBasis,
) (RoleMaskJudgement, error) {
	if kind != KindClassificationTrue && kind != KindClassificationFalse {
		return RoleMaskJudgement{}, fmt.Errorf("settled RoleMask judgement must be true or false")
	}
	if !basis.validFor(request) {
		return RoleMaskJudgement{}, fmt.Errorf("settled RoleMask judgement requires exact direct-feature basis")
	}
	return sealRoleMaskJudgement(kind, request, settledRoleMaskJudgementBasis{basis: basis})
}

func NewUnknownRoleMaskJudgement(
	request RoleMaskClassificationRequest,
	reasons []KindClassificationUnknownReason,
) (RoleMaskJudgement, error) {
	if !request.valid() {
		return RoleMaskJudgement{}, fmt.Errorf("unknown RoleMask judgement requires an exact request")
	}
	normalized, err := normalizeKindClassificationUnknownReasons(reasons)
	if err != nil {
		return RoleMaskJudgement{}, err
	}
	return sealRoleMaskJudgement(
		KindClassificationUnknown,
		request,
		unknownRoleMaskJudgementBasis{reasons: normalized},
	)
}

func sealRoleMaskJudgement(
	kind KindClassificationJudgementKind,
	request RoleMaskClassificationRequest,
	basis roleMaskJudgementBasis,
) (RoleMaskJudgement, error) {
	if !request.valid() || basis == nil {
		return RoleMaskJudgement{}, fmt.Errorf("RoleMask judgement requires exact request and basis")
	}
	writer := newCanonicalWriter("role-mask-judgement." + kind.String() + ".v1")
	writer.addBytes(request.CanonicalBytes())
	writer.addBytes(basis.CanonicalBytes())
	return RoleMaskJudgement{
		kind:           kind,
		request:        request,
		basis:          basis,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (judgement RoleMaskJudgement) Kind() KindClassificationJudgementKind {
	return judgement.kind
}

func (judgement RoleMaskJudgement) Request() RoleMaskClassificationRequest {
	return judgement.request
}

func (judgement RoleMaskJudgement) EvaluationBasis() (RoleMaskEvaluationBasis, bool) {
	basis, exists := judgement.basis.(settledRoleMaskJudgementBasis)
	return basis.basis, exists
}

func (judgement RoleMaskJudgement) UnknownReasons() []KindClassificationUnknownReason {
	basis, exists := judgement.basis.(unknownRoleMaskJudgementBasis)
	if !exists {
		return nil
	}
	return append([]KindClassificationUnknownReason(nil), basis.reasons...)
}

func (judgement RoleMaskJudgement) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement RoleMaskJudgement) Digest() SHA256Digest { return judgement.digest }
