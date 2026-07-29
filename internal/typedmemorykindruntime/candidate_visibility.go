package typedmemorykindruntime

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// EntitySetCandidateBasis is the closed observation-mode basis consumed by
// enumeration. Persisted evaluation and prospective prefix visibility cannot
// be represented by one optional field or boolean.
type EntitySetCandidateBasis interface {
	CanonicalBytes() []byte
	entitySetCandidateBasisVariant()
}

type PersistedEntitySetCandidateBasis struct {
	evaluationViewDigest typedmemory.SHA256Digest
}

func NewPersistedEntitySetCandidateBasis(
	view typedmemory.PersistedSnapshotView,
) (PersistedEntitySetCandidateBasis, error) {
	if !validEvaluationView(view) {
		return PersistedEntitySetCandidateBasis{}, fmt.Errorf(
			"persisted EntitySet candidate basis requires an exact view",
		)
	}
	return PersistedEntitySetCandidateBasis{
		evaluationViewDigest: view.Digest(),
	}, nil
}

func (basis PersistedEntitySetCandidateBasis) EvaluationViewDigest() typedmemory.SHA256Digest {
	return basis.evaluationViewDigest
}

func (basis PersistedEntitySetCandidateBasis) CanonicalBytes() []byte {
	writer := newCanonicalWriter("entity-set-candidate-basis.persisted.v1")
	writer.addString(basis.evaluationViewDigest.String())
	return writer.bytes()
}

func (PersistedEntitySetCandidateBasis) entitySetCandidateBasisVariant() {}

type ProspectiveEntitySetCandidateBasis struct {
	visible CandidateVisible
}

func NewProspectiveEntitySetCandidateBasis(
	visible CandidateVisible,
) (ProspectiveEntitySetCandidateBasis, error) {
	if !validCandidateVisibilityResult(visible) {
		return ProspectiveEntitySetCandidateBasis{}, fmt.Errorf(
			"prospective EntitySet candidate basis requires exact CandidateVisible",
		)
	}
	return ProspectiveEntitySetCandidateBasis{visible: visible}, nil
}

func (basis ProspectiveEntitySetCandidateBasis) Visible() CandidateVisible {
	return basis.visible
}

func (basis ProspectiveEntitySetCandidateBasis) CanonicalBytes() []byte {
	writer := newCanonicalWriter("entity-set-candidate-basis.prospective.v1")
	writer.addBytes(basis.visible.CanonicalBytes())
	return writer.bytes()
}

func (ProspectiveEntitySetCandidateBasis) entitySetCandidateBasisVariant() {}

func candidateBasisMatches(
	basis EntitySetCandidateBasis,
	definition typedmemory.EntitySetDefinition,
	view typedmemory.MemberOfEvaluationView,
) bool {
	switch exactView := view.(type) {
	case typedmemory.PersistedSnapshotView:
		persisted, ok := basis.(PersistedEntitySetCandidateBasis)
		return ok &&
			persisted.evaluationViewDigest == exactView.Digest()
	case typedmemory.ProspectiveBatchView:
		prospective, ok := basis.(ProspectiveEntitySetCandidateBasis)
		if !ok || !validCandidateVisibilityResult(prospective.visible) {
			return false
		}
		policy, ok := definition.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
		if !ok {
			return false
		}
		visibleBasis := prospective.visible.Basis()
		return prospective.visible.DefinitionRef() == definition.Ref() &&
			prospective.visible.EvaluationViewDigest() == exactView.Digest() &&
			visibleBasis.Rule() == policy.EvaluationRule()
	default:
		return false
	}
}

func cloneEntitySetCandidateBasis(
	basis EntitySetCandidateBasis,
) EntitySetCandidateBasis {
	switch value := basis.(type) {
	case PersistedEntitySetCandidateBasis:
		return value
	case ProspectiveEntitySetCandidateBasis:
		return value
	default:
		return nil
	}
}

func validEntitySetCandidateBasis(basis EntitySetCandidateBasis) bool {
	switch value := basis.(type) {
	case PersistedEntitySetCandidateBasis:
		digest, err := typedmemory.NewSHA256Digest(
			value.evaluationViewDigest.String(),
		)
		return err == nil && digest == value.evaluationViewDigest
	case ProspectiveEntitySetCandidateBasis:
		return validCandidateVisibilityResult(value.visible)
	default:
		return false
	}
}

// CandidateVisibilityRequest is the exact prospective-prefix question for an
// EntitySet whose declaration selected PriorBatchDeclarationsVisible. The
// request does not decide membership in a kind.
type CandidateVisibilityRequest struct {
	definition typedmemory.EntitySetDefinition
	view       typedmemory.ProspectiveBatchView
	canonical  []byte
	digest     typedmemory.SHA256Digest
}

type CandidateVisibilityRequestInput struct {
	Definition typedmemory.EntitySetDefinition
	View       typedmemory.ProspectiveBatchView
}

func NewCandidateVisibilityRequest(
	input CandidateVisibilityRequestInput,
) (CandidateVisibilityRequest, error) {
	if !validEntitySetDefinition(input.Definition) {
		return CandidateVisibilityRequest{}, fmt.Errorf(
			"candidate visibility EntitySet definition is invalid",
		)
	}
	if !validEvaluationView(input.View) {
		return CandidateVisibilityRequest{}, fmt.Errorf(
			"candidate visibility requires an exact prospective view",
		)
	}
	policy, ok := input.Definition.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
	if !ok {
		return CandidateVisibilityRequest{}, fmt.Errorf(
			"candidate visibility requires PriorBatchDeclarationsVisible",
		)
	}
	if !validRuleRef(policy.EvaluationRule()) {
		return CandidateVisibilityRequest{}, fmt.Errorf(
			"candidate visibility rule is invalid",
		)
	}
	if input.Definition.Ref().TypeEnv() != input.View.TypeEnv() ||
		input.Definition.Ref().Context() != input.View.Declaration().Context() {
		return CandidateVisibilityRequest{}, fmt.Errorf(
			"candidate visibility definition does not match the prospective view",
		)
	}
	writer := canonicalCandidateVisibilityRequest(input.Definition, input.View)
	return CandidateVisibilityRequest{
		definition: input.Definition,
		view:       input.View,
		canonical:  writer.bytes(),
		digest:     writer.digest(),
	}, nil
}

func (request CandidateVisibilityRequest) Definition() typedmemory.EntitySetDefinition {
	return request.definition
}

func (request CandidateVisibilityRequest) View() typedmemory.ProspectiveBatchView {
	return request.view
}

func (request CandidateVisibilityRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonical...)
}

func (request CandidateVisibilityRequest) Digest() typedmemory.SHA256Digest {
	return request.digest
}

func (request CandidateVisibilityRequest) valid() bool {
	rebuilt, err := NewCandidateVisibilityRequest(CandidateVisibilityRequestInput{
		Definition: request.definition,
		View:       request.view,
	})
	return err == nil &&
		rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonical, request.canonical)
}

type CandidateVisibilityResult interface {
	DefinitionRef() typedmemory.EntitySetDefinitionRef
	EvaluationViewDigest() typedmemory.SHA256Digest
	CanonicalBytes() []byte
	Digest() typedmemory.SHA256Digest
	candidateVisibilityResultVariant()
}

// CandidateVisible is the unforgeable successful refinement consumed by a
// prospective EntitySetEnumeration basis.
type CandidateVisible interface {
	CandidateVisibilityResult
	Basis() CandidateVisibilityBasis
	candidateVisibleResult()
}

type candidateVisible struct {
	basis     CandidateVisibilityBasis
	canonical []byte
	digest    typedmemory.SHA256Digest
}

func (result candidateVisible) DefinitionRef() typedmemory.EntitySetDefinitionRef {
	return result.basis.definition
}

func (result candidateVisible) EvaluationViewDigest() typedmemory.SHA256Digest {
	return result.basis.evaluationViewDigest
}

func (result candidateVisible) Basis() CandidateVisibilityBasis {
	return result.basis.clone()
}

func (result candidateVisible) CanonicalBytes() []byte {
	return append([]byte(nil), result.canonical...)
}

func (result candidateVisible) Digest() typedmemory.SHA256Digest {
	return result.digest
}

func (candidateVisible) candidateVisibilityResultVariant() {}

func (candidateVisible) candidateVisibleResult() {}

type CandidateVisibilityBasis struct {
	definition           typedmemory.EntitySetDefinitionRef
	rule                 typedmemory.RuleRef
	evaluationViewDigest typedmemory.SHA256Digest
	declarationDigest    typedmemory.SHA256Digest
	declarationOrdinal   uint64
	evaluationOrdinal    uint64
	prefixDigest         typedmemory.SHA256Digest
	mechanism            EvaluationMechanism
	canonical            []byte
	digest               typedmemory.SHA256Digest
}

func (basis CandidateVisibilityBasis) DefinitionRef() typedmemory.EntitySetDefinitionRef {
	return basis.definition
}

func (basis CandidateVisibilityBasis) Rule() typedmemory.RuleRef { return basis.rule }

func (basis CandidateVisibilityBasis) EvaluationViewDigest() typedmemory.SHA256Digest {
	return basis.evaluationViewDigest
}

func (basis CandidateVisibilityBasis) DeclarationDigest() typedmemory.SHA256Digest {
	return basis.declarationDigest
}

func (basis CandidateVisibilityBasis) DeclarationOrdinal() uint64 {
	return basis.declarationOrdinal
}

func (basis CandidateVisibilityBasis) EvaluationOrdinal() uint64 {
	return basis.evaluationOrdinal
}

func (basis CandidateVisibilityBasis) PrefixDigest() typedmemory.SHA256Digest {
	return basis.prefixDigest
}

func (basis CandidateVisibilityBasis) Mechanism() EvaluationMechanism {
	return basis.mechanism
}

func (basis CandidateVisibilityBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (basis CandidateVisibilityBasis) Digest() typedmemory.SHA256Digest {
	return basis.digest
}

func (basis CandidateVisibilityBasis) clone() CandidateVisibilityBasis {
	basis.canonical = basis.CanonicalBytes()
	return basis
}

func (basis CandidateVisibilityBasis) valid() bool {
	writer := canonicalCandidateVisibilityBasis(
		basis.definition,
		basis.rule,
		basis.evaluationViewDigest,
		basis.declarationDigest,
		basis.declarationOrdinal,
		basis.evaluationOrdinal,
		basis.prefixDigest,
		basis.mechanism,
	)
	return validEntitySetDefinitionRef(basis.definition) &&
		validRuleRef(basis.rule) &&
		validDigest(basis.evaluationViewDigest) &&
		validDigest(basis.declarationDigest) &&
		validDigest(basis.prefixDigest) &&
		basis.declarationOrdinal < basis.evaluationOrdinal &&
		validEvaluationMechanism(basis.mechanism) &&
		writer.digest() == basis.digest &&
		bytes.Equal(writer.bytes(), basis.canonical)
}

type CandidateVisibilityEvaluator struct {
	rule      typedmemory.RuleRef
	mechanism EvaluationMechanism
}

func NewCandidateVisibilityEvaluator(
	rule typedmemory.RuleRef,
	mechanism EvaluationMechanism,
) (CandidateVisibilityEvaluator, error) {
	if !validRuleRef(rule) {
		return CandidateVisibilityEvaluator{}, fmt.Errorf(
			"candidate visibility evaluator rule is invalid",
		)
	}
	if !validEvaluationMechanism(mechanism) {
		return CandidateVisibilityEvaluator{}, fmt.Errorf(
			"candidate visibility evaluator mechanism is invalid",
		)
	}
	return CandidateVisibilityEvaluator{
		rule:      rule,
		mechanism: mechanism,
	}, nil
}

func (evaluator CandidateVisibilityEvaluator) RuleRef() typedmemory.RuleRef {
	return evaluator.rule
}

func (evaluator CandidateVisibilityEvaluator) Evaluate(
	request CandidateVisibilityRequest,
) (CandidateVisibilityResult, error) {
	if !evaluator.valid() {
		return nil, fmt.Errorf("candidate visibility evaluator is invalid")
	}
	if !request.valid() {
		return nil, fmt.Errorf("candidate visibility request is invalid")
	}
	policy := request.definition.CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible)
	if policy.EvaluationRule() != evaluator.rule {
		return nil, fmt.Errorf(
			"candidate visibility rule does not match the selected evaluator",
		)
	}
	view := request.view
	basis := newCandidateVisibilityBasis(request.definition, view, evaluator)
	if !basis.valid() {
		return nil, fmt.Errorf("candidate visibility basis is invalid")
	}
	writer := newCanonicalWriter("candidate-visibility-result.visible.v1")
	writer.addBytes(basis.CanonicalBytes())
	return candidateVisible{
		basis:     basis,
		canonical: writer.bytes(),
		digest:    writer.digest(),
	}, nil
}

func (evaluator CandidateVisibilityEvaluator) valid() bool {
	rebuilt, err := NewCandidateVisibilityEvaluator(
		evaluator.rule,
		evaluator.mechanism,
	)
	return err == nil && rebuilt == evaluator
}

func newCandidateVisibilityBasis(
	definition typedmemory.EntitySetDefinition,
	view typedmemory.ProspectiveBatchView,
	evaluator CandidateVisibilityEvaluator,
) CandidateVisibilityBasis {
	prefix := view.OrderedCandidatePrefix()
	writer := canonicalCandidateVisibilityBasis(
		definition.Ref(),
		evaluator.rule,
		view.Digest(),
		view.DeclarationDigest(),
		view.DeclarationChangeOrdinal(),
		view.EvaluationChangeOrdinal(),
		prefix.Digest(),
		evaluator.mechanism,
	)
	return CandidateVisibilityBasis{
		definition:           definition.Ref(),
		rule:                 evaluator.rule,
		evaluationViewDigest: view.Digest(),
		declarationDigest:    view.DeclarationDigest(),
		declarationOrdinal:   view.DeclarationChangeOrdinal(),
		evaluationOrdinal:    view.EvaluationChangeOrdinal(),
		prefixDigest:         prefix.Digest(),
		mechanism:            evaluator.mechanism,
		canonical:            writer.bytes(),
		digest:               writer.digest(),
	}
}

func canonicalCandidateVisibilityRequest(
	definition typedmemory.EntitySetDefinition,
	view typedmemory.ProspectiveBatchView,
) canonicalWriter {
	writer := newCanonicalWriter("candidate-visibility-request.v1")
	writer.addBytes(definition.CanonicalBytes())
	writer.addBytes(view.CanonicalBytes())
	return writer
}

func canonicalCandidateVisibilityBasis(
	definition typedmemory.EntitySetDefinitionRef,
	rule typedmemory.RuleRef,
	evaluationViewDigest typedmemory.SHA256Digest,
	declarationDigest typedmemory.SHA256Digest,
	declarationOrdinal uint64,
	evaluationOrdinal uint64,
	prefixDigest typedmemory.SHA256Digest,
	mechanism EvaluationMechanism,
) canonicalWriter {
	writer := newCanonicalWriter("candidate-visibility-basis.v1")
	writer.addString(definition.String())
	writer.addString(rule.String())
	writer.addString(evaluationViewDigest.String())
	writer.addString(declarationDigest.String())
	writer.addUint64(declarationOrdinal)
	writer.addUint64(evaluationOrdinal)
	writer.addString(prefixDigest.String())
	writer.addBytes(mechanism.CanonicalBytes())
	return writer
}

func validCandidateVisibilityResult(result CandidateVisibilityResult) bool {
	value, ok := result.(candidateVisible)
	if !ok || !value.basis.valid() {
		return false
	}
	writer := newCanonicalWriter("candidate-visibility-result.visible.v1")
	writer.addBytes(value.basis.CanonicalBytes())
	return writer.digest() == value.digest &&
		bytes.Equal(writer.bytes(), value.canonical)
}
