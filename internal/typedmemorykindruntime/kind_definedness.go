package typedmemorykindruntime

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// KindDefinednessObservation is the closed source posture presented to the
// selected definedness rule. Missing source content cannot be interpreted as
// a defined KindSignature.
type KindDefinednessObservation interface {
	kindDefinednessObservationVariant()
}

type ExactKindDefinednessObservation struct {
	inputs []typedmemory.MemberOfObservableInput
}

func NewExactKindDefinednessObservation(
	inputs []typedmemory.MemberOfObservableInput,
) (ExactKindDefinednessObservation, error) {
	normalized, err := normalizeObservableInputs(inputs)
	if err != nil {
		return ExactKindDefinednessObservation{}, err
	}
	if len(normalized) == 0 {
		return ExactKindDefinednessObservation{}, fmt.Errorf(
			"exact Kind definedness observation requires at least one observable input",
		)
	}
	return ExactKindDefinednessObservation{inputs: normalized}, nil
}

func (observation ExactKindDefinednessObservation) ObservableInputs() []typedmemory.MemberOfObservableInput {
	return append([]typedmemory.MemberOfObservableInput(nil), observation.inputs...)
}

func (ExactKindDefinednessObservation) kindDefinednessObservationVariant() {}

func (observation ExactKindDefinednessObservation) valid() bool {
	inputs, err := normalizeObservableInputs(observation.inputs)
	return err == nil &&
		len(inputs) > 0 &&
		exactObservableInputs(inputs, observation.inputs)
}

type MissingKindDefinednessObservation struct {
	inputs []typedmemory.ObservableInputRef
	repair typedmemory.RepairPointer
}

type MissingKindDefinednessObservationInput struct {
	ObservableInputs []typedmemory.ObservableInputRef
	Repair           typedmemory.RepairPointer
}

func NewMissingKindDefinednessObservation(
	input MissingKindDefinednessObservationInput,
) (MissingKindDefinednessObservation, error) {
	inputs, err := normalizeObservableInputRefs(input.ObservableInputs)
	if err != nil {
		return MissingKindDefinednessObservation{}, err
	}
	if len(inputs) == 0 {
		return MissingKindDefinednessObservation{}, fmt.Errorf(
			"missing Kind definedness observation requires at least one input",
		)
	}
	if !validRepairPointer(input.Repair) {
		return MissingKindDefinednessObservation{}, fmt.Errorf(
			"missing Kind definedness observation repair pointer is required",
		)
	}
	return MissingKindDefinednessObservation{
		inputs: inputs,
		repair: input.Repair,
	}, nil
}

func (observation MissingKindDefinednessObservation) ObservableInputs() []typedmemory.ObservableInputRef {
	return append([]typedmemory.ObservableInputRef(nil), observation.inputs...)
}

func (observation MissingKindDefinednessObservation) Repair() typedmemory.RepairPointer {
	return observation.repair
}

func (MissingKindDefinednessObservation) kindDefinednessObservationVariant() {}

func (observation MissingKindDefinednessObservation) valid() bool {
	inputs, err := normalizeObservableInputRefs(observation.inputs)
	return err == nil &&
		len(inputs) > 0 &&
		exactObservableInputRefs(inputs, observation.inputs) &&
		validRepairPointer(observation.repair)
}

// KindDefinednessRequest joins one entity-specific MemberOf request to the
// exact KindSignature, the already evaluated EntitySet, and source posture.
// The request cannot be built across TypeEnv, context, slice, or view seams.
type KindDefinednessRequest struct {
	memberOf    typedmemory.MemberOfEvaluationRequest
	signature   typedmemory.KindSignatureDefinition
	enumeration EntitySetEnumerationResult
	observation KindDefinednessObservation
	canonical   []byte
	digest      typedmemory.SHA256Digest
}

type KindDefinednessRequestInput struct {
	MemberOfRequest typedmemory.MemberOfEvaluationRequest
	Signature       typedmemory.KindSignatureDefinition
	Enumeration     EntitySetEnumerationResult
	Observation     KindDefinednessObservation
}

func NewKindDefinednessRequest(
	input KindDefinednessRequestInput,
) (KindDefinednessRequest, error) {
	if !validMemberOfEvaluationRequest(input.MemberOfRequest) ||
		!validKindSignatureDefinition(input.Signature) ||
		!validEntitySetEnumerationResult(input.Enumeration) ||
		!validKindDefinednessObservation(input.Observation) {
		return KindDefinednessRequest{}, fmt.Errorf(
			"Kind definedness requires an exact MemberOf request, signature, EntitySet result, and observation",
		)
	}
	memberQuery := input.MemberOfRequest.Query()
	enumerationDefinition := input.Enumeration.DefinitionRef()
	if input.Signature.ValueKind() != memberQuery.ValueKind() ||
		input.Signature.Ref().Context() != memberQuery.ContextSlice().Context() ||
		input.Signature.EntitySet() != enumerationDefinition ||
		input.Enumeration.ContextSliceRef() != memberQuery.ContextSlice().Ref() ||
		input.Enumeration.EvaluationViewDigest() != input.MemberOfRequest.View().Digest() {
		return KindDefinednessRequest{}, fmt.Errorf(
			"Kind definedness coordinates do not match the exact KindSignature, EntitySet, ContextSlice, and evaluation view",
		)
	}
	writer := canonicalKindDefinednessRequest(input)
	return KindDefinednessRequest{
		memberOf:    input.MemberOfRequest,
		signature:   input.Signature,
		enumeration: input.Enumeration,
		observation: cloneKindDefinednessObservation(input.Observation),
		canonical:   writer.bytes(),
		digest:      writer.digest(),
	}, nil
}

func (request KindDefinednessRequest) MemberOfRequest() typedmemory.MemberOfEvaluationRequest {
	return request.memberOf
}

func (request KindDefinednessRequest) Signature() typedmemory.KindSignatureDefinition {
	return request.signature
}

func (request KindDefinednessRequest) Enumeration() EntitySetEnumerationResult {
	return request.enumeration
}

func (request KindDefinednessRequest) Observation() KindDefinednessObservation {
	return cloneKindDefinednessObservation(request.observation)
}

func (request KindDefinednessRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonical...)
}

func (request KindDefinednessRequest) Digest() typedmemory.SHA256Digest {
	return request.digest
}

func (request KindDefinednessRequest) valid() bool {
	rebuilt, err := NewKindDefinednessRequest(KindDefinednessRequestInput{
		MemberOfRequest: request.memberOf,
		Signature:       request.signature,
		Enumeration:     request.enumeration,
		Observation:     request.observation,
	})
	return err == nil &&
		rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonical, request.canonical)
}

type KindDefinednessResultKind uint8

const (
	KindDefinedResult KindDefinednessResultKind = iota + 1
	KindDefinednessUndefinedResult
)

func (kind KindDefinednessResultKind) String() string {
	switch kind {
	case KindDefinedResult:
		return "defined"
	case KindDefinednessUndefinedResult:
		return "undefined"
	default:
		return ""
	}
}

type KindDefinednessResult interface {
	Kind() KindDefinednessResultKind
	MemberOfRequestDigest() typedmemory.SHA256Digest
	SignatureRef() typedmemory.KindSignatureRef
	CanonicalBytes() []byte
	Digest() typedmemory.SHA256Digest
	kindDefinednessResultVariant()
}

type KindDefined interface {
	KindDefinednessResult
	Basis() KindDefinednessBasis
	kindDefinedResult()
}

type kindDefined struct {
	basis     KindDefinednessBasis
	canonical []byte
	digest    typedmemory.SHA256Digest
}

func (kindDefined) Kind() KindDefinednessResultKind { return KindDefinedResult }

func (result kindDefined) MemberOfRequestDigest() typedmemory.SHA256Digest {
	return result.basis.memberOfRequestDigest
}

func (result kindDefined) SignatureRef() typedmemory.KindSignatureRef {
	return result.basis.signature
}

func (result kindDefined) Basis() KindDefinednessBasis { return result.basis.clone() }

func (result kindDefined) CanonicalBytes() []byte {
	return append([]byte(nil), result.canonical...)
}

func (result kindDefined) Digest() typedmemory.SHA256Digest { return result.digest }

func (kindDefined) kindDefinednessResultVariant() {}

func (kindDefined) kindDefinedResult() {}

type KindDefinednessUndefined interface {
	KindDefinednessResult
	Failure() KindDefinednessFailure
	kindDefinednessUndefinedResult()
}

type kindDefinednessUndefined struct {
	memberOfRequestDigest typedmemory.SHA256Digest
	signature             typedmemory.KindSignatureRef
	failure               KindDefinednessFailure
	canonical             []byte
	digest                typedmemory.SHA256Digest
}

func (kindDefinednessUndefined) Kind() KindDefinednessResultKind {
	return KindDefinednessUndefinedResult
}

func (result kindDefinednessUndefined) MemberOfRequestDigest() typedmemory.SHA256Digest {
	return result.memberOfRequestDigest
}

func (result kindDefinednessUndefined) SignatureRef() typedmemory.KindSignatureRef {
	return result.signature
}

func (result kindDefinednessUndefined) Failure() KindDefinednessFailure {
	return cloneKindDefinednessFailure(result.failure)
}

func (result kindDefinednessUndefined) CanonicalBytes() []byte {
	return append([]byte(nil), result.canonical...)
}

func (result kindDefinednessUndefined) Digest() typedmemory.SHA256Digest {
	return result.digest
}

func (kindDefinednessUndefined) kindDefinednessResultVariant() {}

func (kindDefinednessUndefined) kindDefinednessUndefinedResult() {}

type KindDefinednessBasis struct {
	memberOfRequestDigest typedmemory.SHA256Digest
	signature             typedmemory.KindSignatureRef
	entitySet             typedmemory.EntitySetDefinitionRef
	contextSlice          typedmemory.ContextSliceRef
	evaluationViewDigest  typedmemory.SHA256Digest
	enumerationDigest     typedmemory.SHA256Digest
	rule                  typedmemory.RuleRef
	inputs                []typedmemory.MemberOfObservableInput
	assumptions           []typedmemory.KindAssumptionPin
	mechanism             EvaluationMechanism
	canonical             []byte
	digest                typedmemory.SHA256Digest
}

func (basis KindDefinednessBasis) MemberOfRequestDigest() typedmemory.SHA256Digest {
	return basis.memberOfRequestDigest
}

func (basis KindDefinednessBasis) SignatureRef() typedmemory.KindSignatureRef {
	return basis.signature
}

func (basis KindDefinednessBasis) EntitySetRef() typedmemory.EntitySetDefinitionRef {
	return basis.entitySet
}

func (basis KindDefinednessBasis) ContextSliceRef() typedmemory.ContextSliceRef {
	return basis.contextSlice
}

func (basis KindDefinednessBasis) EvaluationViewDigest() typedmemory.SHA256Digest {
	return basis.evaluationViewDigest
}

func (basis KindDefinednessBasis) EnumerationDigest() typedmemory.SHA256Digest {
	return basis.enumerationDigest
}

func (basis KindDefinednessBasis) Rule() typedmemory.RuleRef { return basis.rule }

func (basis KindDefinednessBasis) ObservableInputs() []typedmemory.MemberOfObservableInput {
	return append([]typedmemory.MemberOfObservableInput(nil), basis.inputs...)
}

func (basis KindDefinednessBasis) MatchedAssumptions() []typedmemory.KindAssumptionPin {
	return append([]typedmemory.KindAssumptionPin(nil), basis.assumptions...)
}

func (basis KindDefinednessBasis) Mechanism() EvaluationMechanism {
	return basis.mechanism
}

func (basis KindDefinednessBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (basis KindDefinednessBasis) Digest() typedmemory.SHA256Digest { return basis.digest }

func (basis KindDefinednessBasis) clone() KindDefinednessBasis {
	basis.inputs = basis.ObservableInputs()
	basis.assumptions = basis.MatchedAssumptions()
	basis.canonical = basis.CanonicalBytes()
	return basis
}

func (basis KindDefinednessBasis) valid() bool {
	inputs, inputErr := normalizeObservableInputs(basis.inputs)
	assumptions, assumptionErr := normalizeKindAssumptionPins(basis.assumptions)
	if inputErr != nil || assumptionErr != nil || len(inputs) == 0 {
		return false
	}
	writer := canonicalKindDefinednessBasis(
		basis.memberOfRequestDigest,
		basis.signature,
		basis.entitySet,
		basis.contextSlice,
		basis.evaluationViewDigest,
		basis.enumerationDigest,
		basis.rule,
		inputs,
		assumptions,
		basis.mechanism,
	)
	return validDigest(basis.memberOfRequestDigest) &&
		validKindSignatureRef(basis.signature) &&
		validEntitySetDefinitionRef(basis.entitySet) &&
		validContextSliceRef(basis.contextSlice) &&
		validDigest(basis.evaluationViewDigest) &&
		validDigest(basis.enumerationDigest) &&
		validRuleRef(basis.rule) &&
		validEvaluationMechanism(basis.mechanism) &&
		exactObservableInputs(inputs, basis.inputs) &&
		exactKindAssumptionPins(assumptions, basis.assumptions) &&
		writer.digest() == basis.digest &&
		bytes.Equal(writer.bytes(), basis.canonical)
}

type KindDefinednessFailureKind uint8

const (
	KindDefinednessEntitySetUnavailable KindDefinednessFailureKind = iota + 1
	KindDefinednessEntityOutsideSet
	KindDefinednessAssumptionsUnavailable
	KindDefinednessObservationUnavailable
)

func (kind KindDefinednessFailureKind) String() string {
	switch kind {
	case KindDefinednessEntitySetUnavailable:
		return "entity_set_unavailable"
	case KindDefinednessEntityOutsideSet:
		return "entity_outside_entity_set"
	case KindDefinednessAssumptionsUnavailable:
		return "assumptions_unavailable"
	case KindDefinednessObservationUnavailable:
		return "observation_unavailable"
	default:
		return ""
	}
}

type KindDefinednessFailure interface {
	Kind() KindDefinednessFailureKind
	CanonicalBytes() []byte
	Digest() typedmemory.SHA256Digest
	kindDefinednessFailureVariant()
}

type EntitySetUnavailableForDefinedness struct {
	enumeration EntitySetEnumerationUndefined
	canonical   []byte
	digest      typedmemory.SHA256Digest
}

func newEntitySetUnavailableForDefinedness(
	enumeration EntitySetEnumerationUndefined,
) EntitySetUnavailableForDefinedness {
	writer := newCanonicalWriter("kind-definedness-failure.entity-set-unavailable.v1")
	writer.addBytes(enumeration.CanonicalBytes())
	return EntitySetUnavailableForDefinedness{
		enumeration: enumeration,
		canonical:   writer.bytes(),
		digest:      writer.digest(),
	}
}

func (EntitySetUnavailableForDefinedness) Kind() KindDefinednessFailureKind {
	return KindDefinednessEntitySetUnavailable
}

func (failure EntitySetUnavailableForDefinedness) Enumeration() EntitySetEnumerationUndefined {
	return failure.enumeration
}

func (failure EntitySetUnavailableForDefinedness) CanonicalBytes() []byte {
	return append([]byte(nil), failure.canonical...)
}

func (failure EntitySetUnavailableForDefinedness) Digest() typedmemory.SHA256Digest {
	return failure.digest
}

func (EntitySetUnavailableForDefinedness) kindDefinednessFailureVariant() {}

type EntityOutsideSetForDefinedness struct {
	entity            typedmemory.EntityID
	enumerationDigest typedmemory.SHA256Digest
	canonical         []byte
	digest            typedmemory.SHA256Digest
}

func newEntityOutsideSetForDefinedness(
	entity typedmemory.EntityID,
	enumeration EntitySetEnumerated,
) EntityOutsideSetForDefinedness {
	writer := newCanonicalWriter("kind-definedness-failure.entity-outside-set.v1")
	writer.addString(entity.String())
	writer.addString(enumeration.Digest().String())
	return EntityOutsideSetForDefinedness{
		entity:            entity,
		enumerationDigest: enumeration.Digest(),
		canonical:         writer.bytes(),
		digest:            writer.digest(),
	}
}

func (EntityOutsideSetForDefinedness) Kind() KindDefinednessFailureKind {
	return KindDefinednessEntityOutsideSet
}

func (failure EntityOutsideSetForDefinedness) EntityID() typedmemory.EntityID {
	return failure.entity
}

func (failure EntityOutsideSetForDefinedness) EnumerationDigest() typedmemory.SHA256Digest {
	return failure.enumerationDigest
}

func (failure EntityOutsideSetForDefinedness) CanonicalBytes() []byte {
	return append([]byte(nil), failure.canonical...)
}

func (failure EntityOutsideSetForDefinedness) Digest() typedmemory.SHA256Digest {
	return failure.digest
}

func (EntityOutsideSetForDefinedness) kindDefinednessFailureVariant() {}

type AssumptionsUnavailableForDefinedness struct {
	assumptions []typedmemory.KindAssumptionPin
	canonical   []byte
	digest      typedmemory.SHA256Digest
}

func newAssumptionsUnavailableForDefinedness(
	assumptions []typedmemory.KindAssumptionPin,
) AssumptionsUnavailableForDefinedness {
	writer := newCanonicalWriter("kind-definedness-failure.assumptions-unavailable.v1")
	writer.addUint64(uint64(len(assumptions)))
	for _, assumption := range assumptions {
		writer.addBytes(assumption.CanonicalBytes())
	}
	return AssumptionsUnavailableForDefinedness{
		assumptions: append([]typedmemory.KindAssumptionPin(nil), assumptions...),
		canonical:   writer.bytes(),
		digest:      writer.digest(),
	}
}

func (AssumptionsUnavailableForDefinedness) Kind() KindDefinednessFailureKind {
	return KindDefinednessAssumptionsUnavailable
}

func (failure AssumptionsUnavailableForDefinedness) Assumptions() []typedmemory.KindAssumptionPin {
	return append([]typedmemory.KindAssumptionPin(nil), failure.assumptions...)
}

func (failure AssumptionsUnavailableForDefinedness) CanonicalBytes() []byte {
	return append([]byte(nil), failure.canonical...)
}

func (failure AssumptionsUnavailableForDefinedness) Digest() typedmemory.SHA256Digest {
	return failure.digest
}

func (AssumptionsUnavailableForDefinedness) kindDefinednessFailureVariant() {}

type ObservationUnavailableForDefinedness struct {
	inputs    []typedmemory.ObservableInputRef
	repair    typedmemory.RepairPointer
	canonical []byte
	digest    typedmemory.SHA256Digest
}

func newObservationUnavailableForDefinedness(
	observation MissingKindDefinednessObservation,
) ObservationUnavailableForDefinedness {
	writer := newCanonicalWriter("kind-definedness-failure.observation-unavailable.v1")
	writer.addUint64(uint64(len(observation.inputs)))
	for _, input := range observation.inputs {
		writer.addString(input.String())
	}
	writer.addString(observation.repair.String())
	return ObservationUnavailableForDefinedness{
		inputs:    observation.ObservableInputs(),
		repair:    observation.repair,
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}
}

func (ObservationUnavailableForDefinedness) Kind() KindDefinednessFailureKind {
	return KindDefinednessObservationUnavailable
}

func (failure ObservationUnavailableForDefinedness) ObservableInputs() []typedmemory.ObservableInputRef {
	return append([]typedmemory.ObservableInputRef(nil), failure.inputs...)
}

func (failure ObservationUnavailableForDefinedness) Repair() typedmemory.RepairPointer {
	return failure.repair
}

func (failure ObservationUnavailableForDefinedness) CanonicalBytes() []byte {
	return append([]byte(nil), failure.canonical...)
}

func (failure ObservationUnavailableForDefinedness) Digest() typedmemory.SHA256Digest {
	return failure.digest
}

func (ObservationUnavailableForDefinedness) kindDefinednessFailureVariant() {}

type KindDefinednessEvaluator struct {
	rule      typedmemory.RuleRef
	mechanism EvaluationMechanism
}

func NewKindDefinednessEvaluator(
	rule typedmemory.RuleRef,
	mechanism EvaluationMechanism,
) (KindDefinednessEvaluator, error) {
	if !validRuleRef(rule) {
		return KindDefinednessEvaluator{}, fmt.Errorf(
			"Kind definedness evaluator rule is invalid",
		)
	}
	if !validEvaluationMechanism(mechanism) {
		return KindDefinednessEvaluator{}, fmt.Errorf(
			"Kind definedness evaluator mechanism is invalid",
		)
	}
	return KindDefinednessEvaluator{rule: rule, mechanism: mechanism}, nil
}

func (evaluator KindDefinednessEvaluator) RuleRef() typedmemory.RuleRef {
	return evaluator.rule
}

func (evaluator KindDefinednessEvaluator) Mechanism() EvaluationMechanism {
	return evaluator.mechanism
}

func (evaluator KindDefinednessEvaluator) Evaluate(
	request KindDefinednessRequest,
) (KindDefinednessResult, error) {
	if !evaluator.valid() || !request.valid() {
		return nil, fmt.Errorf("Kind definedness evaluator or request is invalid")
	}
	if request.signature.DefinednessRule() != evaluator.rule {
		return nil, fmt.Errorf(
			"Kind definedness rule does not match the selected evaluator",
		)
	}
	switch enumeration := request.enumeration.(type) {
	case entitySetEnumerationUndefined:
		failure := newEntitySetUnavailableForDefinedness(enumeration)
		return newKindDefinednessUndefined(request, failure)
	case entitySetEnumerated:
		entity := request.memberOf.Query().EntityID()
		if !enumeration.Contains(entity) {
			failure := newEntityOutsideSetForDefinedness(entity, enumeration)
			return newKindDefinednessUndefined(request, failure)
		}
	default:
		return nil, fmt.Errorf("Kind definedness EntitySet result is unsupported")
	}
	missingAssumptions := missingKindAssumptions(request)
	if len(missingAssumptions) > 0 {
		failure := newAssumptionsUnavailableForDefinedness(missingAssumptions)
		return newKindDefinednessUndefined(request, failure)
	}
	switch observation := request.observation.(type) {
	case MissingKindDefinednessObservation:
		failure := newObservationUnavailableForDefinedness(observation)
		return newKindDefinednessUndefined(request, failure)
	case ExactKindDefinednessObservation:
		basis := newKindDefinednessBasis(request, observation, evaluator)
		if !basis.valid() {
			return nil, fmt.Errorf("Kind definedness basis is invalid")
		}
		writer := newCanonicalWriter("kind-definedness-result.defined.v1")
		writer.addBytes(basis.CanonicalBytes())
		return kindDefined{
			basis:     basis,
			canonical: writer.bytes(),
			digest:    writer.digest(),
		}, nil
	default:
		return nil, fmt.Errorf("Kind definedness observation is unsupported")
	}
}

func (evaluator KindDefinednessEvaluator) valid() bool {
	rebuilt, err := NewKindDefinednessEvaluator(evaluator.rule, evaluator.mechanism)
	return err == nil && rebuilt == evaluator
}

func newKindDefinednessBasis(
	request KindDefinednessRequest,
	observation ExactKindDefinednessObservation,
	evaluator KindDefinednessEvaluator,
) KindDefinednessBasis {
	query := request.memberOf.Query()
	assumptions := request.signature.Assumptions()
	writer := canonicalKindDefinednessBasis(
		request.memberOf.Digest(),
		request.signature.Ref(),
		request.signature.EntitySet(),
		query.ContextSlice().Ref(),
		request.memberOf.View().Digest(),
		request.enumeration.Digest(),
		evaluator.rule,
		observation.inputs,
		assumptions,
		evaluator.mechanism,
	)
	return KindDefinednessBasis{
		memberOfRequestDigest: request.memberOf.Digest(),
		signature:             request.signature.Ref(),
		entitySet:             request.signature.EntitySet(),
		contextSlice:          query.ContextSlice().Ref(),
		evaluationViewDigest:  request.memberOf.View().Digest(),
		enumerationDigest:     request.enumeration.Digest(),
		rule:                  evaluator.rule,
		inputs:                observation.ObservableInputs(),
		assumptions:           assumptions,
		mechanism:             evaluator.mechanism,
		canonical:             writer.bytes(),
		digest:                writer.digest(),
	}
}

func newKindDefinednessUndefined(
	request KindDefinednessRequest,
	failure KindDefinednessFailure,
) (KindDefinednessResult, error) {
	if !validKindDefinednessFailure(failure) {
		return nil, fmt.Errorf("Kind definedness failure basis is invalid")
	}
	writer := newCanonicalWriter("kind-definedness-result.undefined.v1")
	writer.addString(request.memberOf.Digest().String())
	writer.addString(request.signature.Ref().String())
	writer.addBytes(failure.CanonicalBytes())
	return kindDefinednessUndefined{
		memberOfRequestDigest: request.memberOf.Digest(),
		signature:             request.signature.Ref(),
		failure:               cloneKindDefinednessFailure(failure),
		canonical:             writer.bytes(),
		digest:                writer.digest(),
	}, nil
}

func canonicalKindDefinednessRequest(
	input KindDefinednessRequestInput,
) canonicalWriter {
	writer := newCanonicalWriter("kind-definedness-request.v1")
	writer.addBytes(input.MemberOfRequest.CanonicalBytes())
	writer.addBytes(input.Signature.CanonicalBytes())
	writer.addBytes(input.Enumeration.CanonicalBytes())
	switch observation := input.Observation.(type) {
	case ExactKindDefinednessObservation:
		writer.addString("exact")
		writer.addUint64(uint64(len(observation.inputs)))
		for _, observable := range observation.inputs {
			writer.addBytes(observable.CanonicalBytes())
		}
	case MissingKindDefinednessObservation:
		writer.addString("missing")
		writer.addUint64(uint64(len(observation.inputs)))
		for _, observable := range observation.inputs {
			writer.addString(observable.String())
		}
		writer.addString(observation.repair.String())
	}
	return writer
}

func canonicalKindDefinednessBasis(
	memberOfRequestDigest typedmemory.SHA256Digest,
	signature typedmemory.KindSignatureRef,
	entitySet typedmemory.EntitySetDefinitionRef,
	contextSlice typedmemory.ContextSliceRef,
	evaluationViewDigest typedmemory.SHA256Digest,
	enumerationDigest typedmemory.SHA256Digest,
	rule typedmemory.RuleRef,
	inputs []typedmemory.MemberOfObservableInput,
	assumptions []typedmemory.KindAssumptionPin,
	mechanism EvaluationMechanism,
) canonicalWriter {
	writer := newCanonicalWriter("kind-definedness-basis.v1")
	writer.addString(memberOfRequestDigest.String())
	writer.addString(signature.String())
	writer.addString(entitySet.String())
	writer.addString(contextSlice.String())
	writer.addString(evaluationViewDigest.String())
	writer.addString(enumerationDigest.String())
	writer.addString(rule.String())
	writer.addUint64(uint64(len(inputs)))
	for _, input := range inputs {
		writer.addBytes(input.CanonicalBytes())
	}
	writer.addUint64(uint64(len(assumptions)))
	for _, assumption := range assumptions {
		writer.addBytes(assumption.CanonicalBytes())
	}
	writer.addBytes(mechanism.CanonicalBytes())
	return writer
}

func validKindDefinednessObservation(
	observation KindDefinednessObservation,
) bool {
	switch value := observation.(type) {
	case ExactKindDefinednessObservation:
		return value.valid()
	case MissingKindDefinednessObservation:
		return value.valid()
	default:
		return false
	}
}

func cloneKindDefinednessObservation(
	observation KindDefinednessObservation,
) KindDefinednessObservation {
	switch value := observation.(type) {
	case ExactKindDefinednessObservation:
		value.inputs = value.ObservableInputs()
		return value
	case MissingKindDefinednessObservation:
		value.inputs = value.ObservableInputs()
		return value
	default:
		return nil
	}
}

func validKindDefinednessFailure(failure KindDefinednessFailure) bool {
	switch value := failure.(type) {
	case EntitySetUnavailableForDefinedness:
		rebuilt := newEntitySetUnavailableForDefinedness(value.enumeration)
		return validEntitySetEnumerationResult(value.enumeration) &&
			rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonical, value.canonical)
	case EntityOutsideSetForDefinedness:
		entity, err := typedmemory.NewEntityID(value.entity.String())
		if err != nil || entity != value.entity || !validDigest(value.enumerationDigest) {
			return false
		}
		writer := newCanonicalWriter("kind-definedness-failure.entity-outside-set.v1")
		writer.addString(value.entity.String())
		writer.addString(value.enumerationDigest.String())
		return writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonical)
	case AssumptionsUnavailableForDefinedness:
		assumptions, err := normalizeKindAssumptionPins(value.assumptions)
		if err != nil || len(assumptions) == 0 {
			return false
		}
		rebuilt := newAssumptionsUnavailableForDefinedness(assumptions)
		return exactKindAssumptionPins(assumptions, value.assumptions) &&
			rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonical, value.canonical)
	case ObservationUnavailableForDefinedness:
		observation, err := NewMissingKindDefinednessObservation(
			MissingKindDefinednessObservationInput{
				ObservableInputs: value.inputs,
				Repair:           value.repair,
			},
		)
		if err != nil {
			return false
		}
		rebuilt := newObservationUnavailableForDefinedness(observation)
		return rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonical, value.canonical)
	default:
		return false
	}
}

func cloneKindDefinednessFailure(
	failure KindDefinednessFailure,
) KindDefinednessFailure {
	switch value := failure.(type) {
	case EntitySetUnavailableForDefinedness:
		value.canonical = value.CanonicalBytes()
		return value
	case EntityOutsideSetForDefinedness:
		value.canonical = value.CanonicalBytes()
		return value
	case AssumptionsUnavailableForDefinedness:
		value.assumptions = value.Assumptions()
		value.canonical = value.CanonicalBytes()
		return value
	case ObservationUnavailableForDefinedness:
		value.inputs = value.ObservableInputs()
		value.canonical = value.CanonicalBytes()
		return value
	default:
		return nil
	}
}

func missingKindAssumptions(
	request KindDefinednessRequest,
) []typedmemory.KindAssumptionPin {
	slice := request.memberOf.Query().ContextSlice()
	missing := make([]typedmemory.KindAssumptionPin, 0)
	for _, assumption := range request.signature.Assumptions() {
		if !contextSliceContainsAssumption(slice, assumption) {
			missing = append(missing, assumption)
		}
	}
	return missing
}

func contextSliceContainsAssumption(
	slice typedmemory.ContextSlice,
	assumption typedmemory.KindAssumptionPin,
) bool {
	for _, pin := range slice.StandardPins() {
		if versionedPinMatches(pin.VersionedPin(), assumption) {
			return true
		}
	}
	for _, pin := range slice.VocabularyPins() {
		if versionedPinMatches(pin.VersionedPin(), assumption) {
			return true
		}
	}
	for _, pin := range slice.RoleSetPins() {
		if versionedPinMatches(pin.VersionedPin(), assumption) {
			return true
		}
	}
	application, ok := slice.GammaTime().(typedmemory.GammaPolicyApplication)
	return ok &&
		application.PolicyRef() == assumption.Reference() &&
		application.PolicyEdition() == assumption.Edition() &&
		application.PolicyDigest() == assumption.Digest()
}

func versionedPinMatches(
	pin typedmemory.VersionedPin,
	assumption typedmemory.KindAssumptionPin,
) bool {
	return pin.Reference() == assumption.Reference() &&
		pin.Edition() == assumption.Edition() &&
		pin.Digest() == assumption.Digest()
}

func normalizeKindAssumptionPins(
	values []typedmemory.KindAssumptionPin,
) ([]typedmemory.KindAssumptionPin, error) {
	result := append([]typedmemory.KindAssumptionPin(nil), values...)
	for _, assumption := range result {
		rebuilt, err := typedmemory.NewKindAssumptionPin(
			assumption.Reference(),
			assumption.Edition(),
			assumption.Digest(),
		)
		if err != nil || !bytes.Equal(
			rebuilt.CanonicalBytes(),
			assumption.CanonicalBytes(),
		) {
			return nil, fmt.Errorf("Kind definedness contains an invalid assumption")
		}
	}
	return result, nil
}

func exactKindAssumptionPins(
	left []typedmemory.KindAssumptionPin,
	right []typedmemory.KindAssumptionPin,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index].CanonicalBytes(), right[index].CanonicalBytes()) {
			return false
		}
	}
	return true
}

func validKindDefinednessResult(result KindDefinednessResult) bool {
	switch value := result.(type) {
	case kindDefined:
		if !value.basis.valid() {
			return false
		}
		writer := newCanonicalWriter("kind-definedness-result.defined.v1")
		writer.addBytes(value.basis.CanonicalBytes())
		return writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonical)
	case kindDefinednessUndefined:
		if !validDigest(value.memberOfRequestDigest) ||
			!validKindSignatureRef(value.signature) ||
			!validKindDefinednessFailure(value.failure) {
			return false
		}
		writer := newCanonicalWriter("kind-definedness-result.undefined.v1")
		writer.addString(value.memberOfRequestDigest.String())
		writer.addString(value.signature.String())
		writer.addBytes(value.failure.CanonicalBytes())
		return writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonical)
	default:
		return false
	}
}
