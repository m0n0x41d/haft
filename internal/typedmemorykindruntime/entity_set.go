package typedmemorykindruntime

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	maximumEnumeratedEntities = 4 << 10
	maximumObservableInputs   = 4 << 10
)

// EntitySetObservation is the closed source posture presented to an
// enumeration evaluator. Missing source material is data, not an error and
// cannot be coerced to an empty U.EntitySet.
type EntitySetObservation interface {
	entitySetObservationVariant()
}

type ExactEntitySetObservation struct {
	entities []typedmemory.EntityID
	inputs   []typedmemory.MemberOfObservableInput
}

type ExactEntitySetObservationInput struct {
	Entities         []typedmemory.EntityID
	ObservableInputs []typedmemory.MemberOfObservableInput
}

func NewExactEntitySetObservation(
	input ExactEntitySetObservationInput,
) (ExactEntitySetObservation, error) {
	entities, err := normalizeEntityIDs(input.Entities)
	if err != nil {
		return ExactEntitySetObservation{}, err
	}
	inputs, err := normalizeObservableInputs(input.ObservableInputs)
	if err != nil {
		return ExactEntitySetObservation{}, err
	}
	if len(inputs) == 0 {
		return ExactEntitySetObservation{}, fmt.Errorf(
			"exact EntitySet observation requires at least one observable input",
		)
	}
	return ExactEntitySetObservation{
		entities: entities,
		inputs:   inputs,
	}, nil
}

func (observation ExactEntitySetObservation) Entities() []typedmemory.EntityID {
	return append([]typedmemory.EntityID(nil), observation.entities...)
}

func (observation ExactEntitySetObservation) ObservableInputs() []typedmemory.MemberOfObservableInput {
	return append([]typedmemory.MemberOfObservableInput(nil), observation.inputs...)
}

func (ExactEntitySetObservation) entitySetObservationVariant() {}

func (observation ExactEntitySetObservation) valid() bool {
	entities, entityErr := normalizeEntityIDs(observation.entities)
	inputs, inputErr := normalizeObservableInputs(observation.inputs)
	return entityErr == nil &&
		inputErr == nil &&
		len(inputs) > 0 &&
		exactEntityIDs(entities, observation.entities) &&
		exactObservableInputs(inputs, observation.inputs)
}

type MissingEntitySetObservation struct {
	inputs []typedmemory.ObservableInputRef
	repair typedmemory.RepairPointer
}

type MissingEntitySetObservationInput struct {
	ObservableInputs []typedmemory.ObservableInputRef
	Repair           typedmemory.RepairPointer
}

func NewMissingEntitySetObservation(
	input MissingEntitySetObservationInput,
) (MissingEntitySetObservation, error) {
	inputs, err := normalizeObservableInputRefs(input.ObservableInputs)
	if err != nil {
		return MissingEntitySetObservation{}, err
	}
	if len(inputs) == 0 {
		return MissingEntitySetObservation{}, fmt.Errorf(
			"missing EntitySet observation requires at least one missing input",
		)
	}
	if !validRepairPointer(input.Repair) {
		return MissingEntitySetObservation{}, fmt.Errorf(
			"missing EntitySet observation repair pointer is required",
		)
	}
	return MissingEntitySetObservation{
		inputs: inputs,
		repair: input.Repair,
	}, nil
}

func (observation MissingEntitySetObservation) ObservableInputs() []typedmemory.ObservableInputRef {
	return append([]typedmemory.ObservableInputRef(nil), observation.inputs...)
}

func (observation MissingEntitySetObservation) Repair() typedmemory.RepairPointer {
	return observation.repair
}

func (MissingEntitySetObservation) entitySetObservationVariant() {}

func (observation MissingEntitySetObservation) valid() bool {
	inputs, err := normalizeObservableInputRefs(observation.inputs)
	return err == nil &&
		len(inputs) > 0 &&
		exactObservableInputRefs(inputs, observation.inputs) &&
		validRepairPointer(observation.repair)
}

type EntitySetEnumerationRequest struct {
	contextSlice typedmemory.ContextSlice
	view         typedmemory.MemberOfEvaluationView
	definition   typedmemory.EntitySetDefinition
	candidates   EntitySetCandidateBasis
	observation  EntitySetObservation
	canonical    []byte
	digest       typedmemory.SHA256Digest
}

type EntitySetEnumerationRequestInput struct {
	ContextSlice typedmemory.ContextSlice
	View         typedmemory.MemberOfEvaluationView
	Definition   typedmemory.EntitySetDefinition
	Candidates   EntitySetCandidateBasis
	Observation  EntitySetObservation
}

func NewEntitySetEnumerationRequest(
	input EntitySetEnumerationRequestInput,
) (EntitySetEnumerationRequest, error) {
	if !validContextSlice(input.ContextSlice) {
		return EntitySetEnumerationRequest{}, fmt.Errorf(
			"EntitySet enumeration requires an exact ContextSlice",
		)
	}
	if !validEvaluationView(input.View) {
		return EntitySetEnumerationRequest{}, fmt.Errorf(
			"EntitySet enumeration requires an exact evaluation view",
		)
	}
	if !validEntitySetDefinition(input.Definition) {
		return EntitySetEnumerationRequest{}, fmt.Errorf(
			"EntitySet enumeration definition is invalid",
		)
	}
	if input.Definition.Ref().TypeEnv() != input.View.TypeEnv() ||
		input.Definition.Ref().Context() != input.ContextSlice.Context() {
		return EntitySetEnumerationRequest{}, fmt.Errorf(
			"EntitySet enumeration definition does not match the evaluation TypeEnv and ContextSlice",
		)
	}
	if !candidateBasisMatches(
		input.Candidates,
		input.Definition,
		input.View,
	) {
		return EntitySetEnumerationRequest{}, fmt.Errorf(
			"EntitySet candidate basis does not match the exact evaluation view and policy",
		)
	}
	if !validEntitySetObservation(input.Observation) {
		return EntitySetEnumerationRequest{}, fmt.Errorf(
			"EntitySet enumeration observation is invalid",
		)
	}
	writer := canonicalEntitySetEnumerationRequest(
		input.ContextSlice,
		input.View,
		input.Definition,
		input.Candidates,
		input.Observation,
	)
	return EntitySetEnumerationRequest{
		contextSlice: input.ContextSlice,
		view:         input.View,
		definition:   input.Definition,
		candidates:   cloneEntitySetCandidateBasis(input.Candidates),
		observation:  cloneEntitySetObservation(input.Observation),
		canonical:    writer.bytes(),
		digest:       writer.digest(),
	}, nil
}

func (request EntitySetEnumerationRequest) ContextSlice() typedmemory.ContextSlice {
	return request.contextSlice
}

func (request EntitySetEnumerationRequest) View() typedmemory.MemberOfEvaluationView {
	return request.view
}

func (request EntitySetEnumerationRequest) Definition() typedmemory.EntitySetDefinition {
	return request.definition
}

func (request EntitySetEnumerationRequest) Candidates() EntitySetCandidateBasis {
	return cloneEntitySetCandidateBasis(request.candidates)
}

func (request EntitySetEnumerationRequest) Observation() EntitySetObservation {
	return cloneEntitySetObservation(request.observation)
}

func (request EntitySetEnumerationRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonical...)
}

func (request EntitySetEnumerationRequest) Digest() typedmemory.SHA256Digest {
	return request.digest
}

func (request EntitySetEnumerationRequest) valid() bool {
	rebuilt, err := NewEntitySetEnumerationRequest(EntitySetEnumerationRequestInput{
		ContextSlice: request.contextSlice,
		View:         request.view,
		Definition:   request.definition,
		Candidates:   request.candidates,
		Observation:  request.observation,
	})
	return err == nil &&
		rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonical, request.canonical)
}

type EntitySetEnumerationResultKind uint8

const (
	EntitySetEnumeratedResult EntitySetEnumerationResultKind = iota + 1
	EntitySetEnumerationUndefinedResult
)

func (kind EntitySetEnumerationResultKind) String() string {
	switch kind {
	case EntitySetEnumeratedResult:
		return "enumerated"
	case EntitySetEnumerationUndefinedResult:
		return "undefined"
	default:
		return ""
	}
}

type EntitySetEnumerationResult interface {
	Kind() EntitySetEnumerationResultKind
	DefinitionRef() typedmemory.EntitySetDefinitionRef
	ContextSliceRef() typedmemory.ContextSliceRef
	EvaluationViewDigest() typedmemory.SHA256Digest
	CandidateBasis() EntitySetCandidateBasis
	CanonicalBytes() []byte
	Digest() typedmemory.SHA256Digest
	entitySetEnumerationResultVariant()
}

type EntitySetEnumerated interface {
	EntitySetEnumerationResult
	Entities() []typedmemory.EntityID
	Contains(typedmemory.EntityID) bool
	Basis() EntitySetEnumerationBasis
	entitySetEnumeratedResult()
}

type entitySetEnumerated struct {
	basis     EntitySetEnumerationBasis
	canonical []byte
	digest    typedmemory.SHA256Digest
}

func (entitySetEnumerated) Kind() EntitySetEnumerationResultKind {
	return EntitySetEnumeratedResult
}

func (result entitySetEnumerated) DefinitionRef() typedmemory.EntitySetDefinitionRef {
	return result.basis.definition
}

func (result entitySetEnumerated) ContextSliceRef() typedmemory.ContextSliceRef {
	return result.basis.contextSlice
}

func (result entitySetEnumerated) EvaluationViewDigest() typedmemory.SHA256Digest {
	return result.basis.evaluationViewDigest
}

func (result entitySetEnumerated) CandidateBasis() EntitySetCandidateBasis {
	return cloneEntitySetCandidateBasis(result.basis.candidates)
}

func (result entitySetEnumerated) Entities() []typedmemory.EntityID {
	return result.basis.Entities()
}

func (result entitySetEnumerated) Contains(entity typedmemory.EntityID) bool {
	key := entity.String()
	index := sort.Search(len(result.basis.entities), func(index int) bool {
		return result.basis.entities[index].String() >= key
	})
	return index < len(result.basis.entities) &&
		result.basis.entities[index] == entity
}

func (result entitySetEnumerated) Basis() EntitySetEnumerationBasis {
	return result.basis.clone()
}

func (result entitySetEnumerated) CanonicalBytes() []byte {
	return append([]byte(nil), result.canonical...)
}

func (result entitySetEnumerated) Digest() typedmemory.SHA256Digest {
	return result.digest
}

func (entitySetEnumerated) entitySetEnumerationResultVariant() {}

func (entitySetEnumerated) entitySetEnumeratedResult() {}

type EntitySetEnumerationUndefined interface {
	EntitySetEnumerationResult
	MissingObservableInputs() []typedmemory.ObservableInputRef
	Repair() typedmemory.RepairPointer
	entitySetEnumerationUndefinedResult()
}

type entitySetEnumerationUndefined struct {
	definition           typedmemory.EntitySetDefinitionRef
	contextSlice         typedmemory.ContextSliceRef
	evaluationViewDigest typedmemory.SHA256Digest
	candidates           EntitySetCandidateBasis
	missing              []typedmemory.ObservableInputRef
	repair               typedmemory.RepairPointer
	canonical            []byte
	digest               typedmemory.SHA256Digest
}

func (entitySetEnumerationUndefined) Kind() EntitySetEnumerationResultKind {
	return EntitySetEnumerationUndefinedResult
}

func (result entitySetEnumerationUndefined) DefinitionRef() typedmemory.EntitySetDefinitionRef {
	return result.definition
}

func (result entitySetEnumerationUndefined) ContextSliceRef() typedmemory.ContextSliceRef {
	return result.contextSlice
}

func (result entitySetEnumerationUndefined) EvaluationViewDigest() typedmemory.SHA256Digest {
	return result.evaluationViewDigest
}

func (result entitySetEnumerationUndefined) CandidateBasis() EntitySetCandidateBasis {
	return cloneEntitySetCandidateBasis(result.candidates)
}

func (result entitySetEnumerationUndefined) MissingObservableInputs() []typedmemory.ObservableInputRef {
	return append([]typedmemory.ObservableInputRef(nil), result.missing...)
}

func (result entitySetEnumerationUndefined) Repair() typedmemory.RepairPointer {
	return result.repair
}

func (result entitySetEnumerationUndefined) CanonicalBytes() []byte {
	return append([]byte(nil), result.canonical...)
}

func (result entitySetEnumerationUndefined) Digest() typedmemory.SHA256Digest {
	return result.digest
}

func (entitySetEnumerationUndefined) entitySetEnumerationResultVariant() {}

func (entitySetEnumerationUndefined) entitySetEnumerationUndefinedResult() {}

type EntitySetEnumerationBasis struct {
	definition           typedmemory.EntitySetDefinitionRef
	rule                 typedmemory.RuleRef
	contextSlice         typedmemory.ContextSliceRef
	evaluationViewDigest typedmemory.SHA256Digest
	candidates           EntitySetCandidateBasis
	entities             []typedmemory.EntityID
	inputs               []typedmemory.MemberOfObservableInput
	mechanism            EvaluationMechanism
	canonical            []byte
	digest               typedmemory.SHA256Digest
}

func (basis EntitySetEnumerationBasis) DefinitionRef() typedmemory.EntitySetDefinitionRef {
	return basis.definition
}

func (basis EntitySetEnumerationBasis) Rule() typedmemory.RuleRef { return basis.rule }

func (basis EntitySetEnumerationBasis) ContextSliceRef() typedmemory.ContextSliceRef {
	return basis.contextSlice
}

func (basis EntitySetEnumerationBasis) EvaluationViewDigest() typedmemory.SHA256Digest {
	return basis.evaluationViewDigest
}

func (basis EntitySetEnumerationBasis) CandidateBasis() EntitySetCandidateBasis {
	return cloneEntitySetCandidateBasis(basis.candidates)
}

func (basis EntitySetEnumerationBasis) Entities() []typedmemory.EntityID {
	return append([]typedmemory.EntityID(nil), basis.entities...)
}

func (basis EntitySetEnumerationBasis) ObservableInputs() []typedmemory.MemberOfObservableInput {
	return append([]typedmemory.MemberOfObservableInput(nil), basis.inputs...)
}

func (basis EntitySetEnumerationBasis) Mechanism() EvaluationMechanism {
	return basis.mechanism
}

func (basis EntitySetEnumerationBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (basis EntitySetEnumerationBasis) Digest() typedmemory.SHA256Digest {
	return basis.digest
}

func (basis EntitySetEnumerationBasis) clone() EntitySetEnumerationBasis {
	basis.candidates = cloneEntitySetCandidateBasis(basis.candidates)
	basis.entities = basis.Entities()
	basis.inputs = basis.ObservableInputs()
	basis.canonical = basis.CanonicalBytes()
	return basis
}

func (basis EntitySetEnumerationBasis) valid() bool {
	entities, entityErr := normalizeEntityIDs(basis.entities)
	inputs, inputErr := normalizeObservableInputs(basis.inputs)
	if entityErr != nil || inputErr != nil || len(inputs) == 0 {
		return false
	}
	writer := canonicalEntitySetEnumerationBasis(
		basis.definition,
		basis.rule,
		basis.contextSlice,
		basis.evaluationViewDigest,
		basis.candidates,
		entities,
		inputs,
		basis.mechanism,
	)
	return validEntitySetDefinitionRef(basis.definition) &&
		validRuleRef(basis.rule) &&
		validContextSliceRef(basis.contextSlice) &&
		validDigest(basis.evaluationViewDigest) &&
		validEntitySetCandidateBasis(basis.candidates) &&
		validEvaluationMechanism(basis.mechanism) &&
		exactEntityIDs(entities, basis.entities) &&
		exactObservableInputs(inputs, basis.inputs) &&
		writer.digest() == basis.digest &&
		bytes.Equal(writer.bytes(), basis.canonical)
}

type EntitySetEnumerationEvaluator struct {
	rule      typedmemory.RuleRef
	mechanism EvaluationMechanism
}

func NewEntitySetEnumerationEvaluator(
	rule typedmemory.RuleRef,
	mechanism EvaluationMechanism,
) (EntitySetEnumerationEvaluator, error) {
	if !validRuleRef(rule) {
		return EntitySetEnumerationEvaluator{}, fmt.Errorf(
			"EntitySet enumeration evaluator rule is invalid",
		)
	}
	if !validEvaluationMechanism(mechanism) {
		return EntitySetEnumerationEvaluator{}, fmt.Errorf(
			"EntitySet enumeration evaluator mechanism is invalid",
		)
	}
	return EntitySetEnumerationEvaluator{
		rule:      rule,
		mechanism: mechanism,
	}, nil
}

func (evaluator EntitySetEnumerationEvaluator) RuleRef() typedmemory.RuleRef {
	return evaluator.rule
}

func (evaluator EntitySetEnumerationEvaluator) Mechanism() EvaluationMechanism {
	return evaluator.mechanism
}

func (evaluator EntitySetEnumerationEvaluator) Evaluate(
	request EntitySetEnumerationRequest,
) (EntitySetEnumerationResult, error) {
	if !evaluator.valid() {
		return nil, fmt.Errorf("EntitySet enumeration evaluator is invalid")
	}
	if !request.valid() {
		return nil, fmt.Errorf("EntitySet enumeration request is invalid")
	}
	if request.definition.EnumerationRule() != evaluator.rule {
		return nil, fmt.Errorf(
			"EntitySet enumeration rule does not match the selected evaluator",
		)
	}
	contextSlice := request.contextSlice
	view := request.view
	switch observation := request.observation.(type) {
	case ExactEntitySetObservation:
		basis := newEntitySetEnumerationBasis(
			request.definition,
			contextSlice.Ref(),
			view.Digest(),
			request.candidates,
			observation,
			evaluator,
		)
		if !basis.valid() {
			return nil, fmt.Errorf("EntitySet enumeration basis is invalid")
		}
		writer := newCanonicalWriter("entity-set-enumeration-result.enumerated.v1")
		writer.addBytes(basis.CanonicalBytes())
		return entitySetEnumerated{
			basis:     basis,
			canonical: writer.bytes(),
			digest:    writer.digest(),
		}, nil
	case MissingEntitySetObservation:
		writer := canonicalUndefinedEntitySetEnumeration(
			request.definition.Ref(),
			contextSlice.Ref(),
			view.Digest(),
			request.candidates,
			observation.inputs,
			observation.repair,
		)
		return entitySetEnumerationUndefined{
			definition:           request.definition.Ref(),
			contextSlice:         contextSlice.Ref(),
			evaluationViewDigest: view.Digest(),
			candidates:           cloneEntitySetCandidateBasis(request.candidates),
			missing:              observation.ObservableInputs(),
			repair:               observation.repair,
			canonical:            writer.bytes(),
			digest:               writer.digest(),
		}, nil
	default:
		return nil, fmt.Errorf("EntitySet enumeration observation variant is unsupported")
	}
}

func (evaluator EntitySetEnumerationEvaluator) valid() bool {
	rebuilt, err := NewEntitySetEnumerationEvaluator(
		evaluator.rule,
		evaluator.mechanism,
	)
	return err == nil && rebuilt == evaluator
}

func newEntitySetEnumerationBasis(
	definition typedmemory.EntitySetDefinition,
	contextSlice typedmemory.ContextSliceRef,
	evaluationViewDigest typedmemory.SHA256Digest,
	candidates EntitySetCandidateBasis,
	observation ExactEntitySetObservation,
	evaluator EntitySetEnumerationEvaluator,
) EntitySetEnumerationBasis {
	writer := canonicalEntitySetEnumerationBasis(
		definition.Ref(),
		evaluator.rule,
		contextSlice,
		evaluationViewDigest,
		candidates,
		observation.entities,
		observation.inputs,
		evaluator.mechanism,
	)
	return EntitySetEnumerationBasis{
		definition:           definition.Ref(),
		rule:                 evaluator.rule,
		contextSlice:         contextSlice,
		evaluationViewDigest: evaluationViewDigest,
		candidates:           cloneEntitySetCandidateBasis(candidates),
		entities:             observation.Entities(),
		inputs:               observation.ObservableInputs(),
		mechanism:            evaluator.mechanism,
		canonical:            writer.bytes(),
		digest:               writer.digest(),
	}
}

func canonicalEntitySetEnumerationRequest(
	contextSlice typedmemory.ContextSlice,
	view typedmemory.MemberOfEvaluationView,
	definition typedmemory.EntitySetDefinition,
	candidates EntitySetCandidateBasis,
	observation EntitySetObservation,
) canonicalWriter {
	writer := newCanonicalWriter("entity-set-enumeration-request.v1")
	writer.addBytes(contextSlice.CanonicalBytes())
	writer.addBytes(view.CanonicalBytes())
	writer.addBytes(definition.CanonicalBytes())
	writer.addBytes(candidates.CanonicalBytes())
	switch value := observation.(type) {
	case ExactEntitySetObservation:
		writer.addString("exact")
		writer.addUint64(uint64(len(value.entities)))
		for _, entity := range value.entities {
			writer.addString(entity.String())
		}
		writer.addUint64(uint64(len(value.inputs)))
		for _, input := range value.inputs {
			writer.addBytes(input.CanonicalBytes())
		}
	case MissingEntitySetObservation:
		writer.addString("missing")
		writer.addUint64(uint64(len(value.inputs)))
		for _, input := range value.inputs {
			writer.addString(input.String())
		}
		writer.addString(value.repair.String())
	}
	return writer
}

func canonicalEntitySetEnumerationBasis(
	definition typedmemory.EntitySetDefinitionRef,
	rule typedmemory.RuleRef,
	contextSlice typedmemory.ContextSliceRef,
	evaluationViewDigest typedmemory.SHA256Digest,
	candidates EntitySetCandidateBasis,
	entities []typedmemory.EntityID,
	inputs []typedmemory.MemberOfObservableInput,
	mechanism EvaluationMechanism,
) canonicalWriter {
	writer := newCanonicalWriter("entity-set-enumeration-basis.v1")
	writer.addString(definition.String())
	writer.addString(rule.String())
	writer.addString(contextSlice.String())
	writer.addString(evaluationViewDigest.String())
	writer.addBytes(candidates.CanonicalBytes())
	writer.addUint64(uint64(len(entities)))
	for _, entity := range entities {
		writer.addString(entity.String())
	}
	writer.addUint64(uint64(len(inputs)))
	for _, input := range inputs {
		writer.addBytes(input.CanonicalBytes())
	}
	writer.addBytes(mechanism.CanonicalBytes())
	return writer
}

func canonicalUndefinedEntitySetEnumeration(
	definition typedmemory.EntitySetDefinitionRef,
	contextSlice typedmemory.ContextSliceRef,
	evaluationViewDigest typedmemory.SHA256Digest,
	candidates EntitySetCandidateBasis,
	missing []typedmemory.ObservableInputRef,
	repair typedmemory.RepairPointer,
) canonicalWriter {
	writer := newCanonicalWriter("entity-set-enumeration-result.undefined.v1")
	writer.addString(definition.String())
	writer.addString(contextSlice.String())
	writer.addString(evaluationViewDigest.String())
	writer.addBytes(candidates.CanonicalBytes())
	writer.addUint64(uint64(len(missing)))
	for _, input := range missing {
		writer.addString(input.String())
	}
	writer.addString(repair.String())
	return writer
}

func validEntitySetObservation(observation EntitySetObservation) bool {
	switch value := observation.(type) {
	case ExactEntitySetObservation:
		return value.valid()
	case MissingEntitySetObservation:
		return value.valid()
	default:
		return false
	}
}

func cloneEntitySetObservation(observation EntitySetObservation) EntitySetObservation {
	switch value := observation.(type) {
	case ExactEntitySetObservation:
		value.entities = value.Entities()
		value.inputs = value.ObservableInputs()
		return value
	case MissingEntitySetObservation:
		value.inputs = value.ObservableInputs()
		return value
	default:
		return nil
	}
}

func validEntitySetEnumerationResult(result EntitySetEnumerationResult) bool {
	switch value := result.(type) {
	case entitySetEnumerated:
		if !value.basis.valid() {
			return false
		}
		writer := newCanonicalWriter("entity-set-enumeration-result.enumerated.v1")
		writer.addBytes(value.basis.CanonicalBytes())
		return writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonical)
	case entitySetEnumerationUndefined:
		missing, err := normalizeObservableInputRefs(value.missing)
		if err != nil || len(missing) == 0 || !validRepairPointer(value.repair) {
			return false
		}
		writer := canonicalUndefinedEntitySetEnumeration(
			value.definition,
			value.contextSlice,
			value.evaluationViewDigest,
			value.candidates,
			missing,
			value.repair,
		)
		return validEntitySetDefinitionRef(value.definition) &&
			validContextSliceRef(value.contextSlice) &&
			validDigest(value.evaluationViewDigest) &&
			validEntitySetCandidateBasis(value.candidates) &&
			exactObservableInputRefs(missing, value.missing) &&
			writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonical)
	default:
		return false
	}
}

func normalizeEntityIDs(
	values []typedmemory.EntityID,
) ([]typedmemory.EntityID, error) {
	if len(values) > maximumEnumeratedEntities {
		return nil, fmt.Errorf(
			"EntitySet observation exceeds %d entities",
			maximumEnumeratedEntities,
		)
	}
	owned := append([]typedmemory.EntityID(nil), values...)
	for _, entity := range owned {
		rebuilt, err := typedmemory.NewEntityID(entity.String())
		if err != nil || rebuilt != entity {
			return nil, fmt.Errorf("EntitySet observation contains an invalid EntityID")
		}
	}
	sort.Slice(owned, func(left int, right int) bool {
		return owned[left].String() < owned[right].String()
	})
	result := make([]typedmemory.EntityID, 0, len(owned))
	for _, entity := range owned {
		if len(result) == 0 || result[len(result)-1] != entity {
			result = append(result, entity)
		}
	}
	return result, nil
}

func normalizeObservableInputs(
	values []typedmemory.MemberOfObservableInput,
) ([]typedmemory.MemberOfObservableInput, error) {
	if len(values) > maximumObservableInputs {
		return nil, fmt.Errorf(
			"EntitySet observation exceeds %d observable inputs",
			maximumObservableInputs,
		)
	}
	owned := append([]typedmemory.MemberOfObservableInput(nil), values...)
	for _, input := range owned {
		rebuilt, err := typedmemory.NewMemberOfObservableInput(
			input.Reference(),
			input.Digest(),
		)
		if err != nil || !bytes.Equal(rebuilt.CanonicalBytes(), input.CanonicalBytes()) {
			return nil, fmt.Errorf("EntitySet observation contains an invalid observable input")
		}
	}
	sort.Slice(owned, func(left int, right int) bool {
		leftRef := owned[left].Reference().String()
		rightRef := owned[right].Reference().String()
		if leftRef != rightRef {
			return leftRef < rightRef
		}
		return owned[left].Digest().String() < owned[right].Digest().String()
	})
	result := make([]typedmemory.MemberOfObservableInput, 0, len(owned))
	for _, input := range owned {
		if len(result) == 0 || result[len(result)-1].Reference() != input.Reference() {
			result = append(result, input)
			continue
		}
		previous := result[len(result)-1]
		if previous.Digest() != input.Digest() {
			return nil, fmt.Errorf(
				"EntitySet observable input %q has conflicting digests",
				input.Reference().String(),
			)
		}
	}
	return result, nil
}

func normalizeObservableInputRefs(
	values []typedmemory.ObservableInputRef,
) ([]typedmemory.ObservableInputRef, error) {
	if len(values) > maximumObservableInputs {
		return nil, fmt.Errorf(
			"missing EntitySet observation exceeds %d inputs",
			maximumObservableInputs,
		)
	}
	owned := append([]typedmemory.ObservableInputRef(nil), values...)
	for _, input := range owned {
		rebuilt, err := typedmemory.NewObservableInputRef(input.String())
		if err != nil || rebuilt != input {
			return nil, fmt.Errorf("missing EntitySet observation contains an invalid input")
		}
	}
	sort.Slice(owned, func(left int, right int) bool {
		return owned[left].String() < owned[right].String()
	})
	result := make([]typedmemory.ObservableInputRef, 0, len(owned))
	for _, input := range owned {
		if len(result) == 0 || result[len(result)-1] != input {
			result = append(result, input)
		}
	}
	return result, nil
}

func exactEntityIDs(left, right []typedmemory.EntityID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func exactObservableInputs(
	left []typedmemory.MemberOfObservableInput,
	right []typedmemory.MemberOfObservableInput,
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

func exactObservableInputRefs(
	left []typedmemory.ObservableInputRef,
	right []typedmemory.ObservableInputRef,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
