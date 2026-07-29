package typedmemoryevaluation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorykindruntime"
)

// EntitySetEnumerationRegistry is the package-owned callable registry for the
// C.3.2 EntitySet enumeration invocation contract. Its factory binds only the
// reviewed typedmemorykindruntime evaluator; callers cannot register an
// arbitrary closure under this contract.
type EntitySetEnumerationRegistry = Registry[
	typedmemorykindruntime.EntitySetEnumerationRequest,
	typedmemorykindruntime.EntitySetEnumerationResult,
]

// CandidateVisibilityRegistry is the package-owned callable registry for the
// prospective-prefix visibility contract selected by
// PriorBatchDeclarationsVisible.
type CandidateVisibilityRegistry = Registry[
	typedmemorykindruntime.CandidateVisibilityRequest,
	typedmemorykindruntime.CandidateVisibilityResult,
]

// KindDefinednessRegistry is the package-owned callable registry for the
// C.3.2 KindSignature definedness invocation contract.
type KindDefinednessRegistry = Registry[
	typedmemorykindruntime.KindDefinednessRequest,
	typedmemorykindruntime.KindDefinednessResult,
]

func NewEntitySetEnumerationRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (EntitySetEnumerationRegistry, error) {
	mechanism, err := evaluationMechanismFromIdentity(identity)
	if err != nil {
		return EntitySetEnumerationRegistry{}, err
	}
	evaluator, err := typedmemorykindruntime.NewEntitySetEnumerationEvaluator(
		rule,
		mechanism,
	)
	if err != nil {
		return EntitySetEnumerationRegistry{}, err
	}
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluator.Evaluate,
	)
}

func NewCandidateVisibilityRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (CandidateVisibilityRegistry, error) {
	mechanism, err := evaluationMechanismFromIdentity(identity)
	if err != nil {
		return CandidateVisibilityRegistry{}, err
	}
	evaluator, err := typedmemorykindruntime.NewCandidateVisibilityEvaluator(
		rule,
		mechanism,
	)
	if err != nil {
		return CandidateVisibilityRegistry{}, err
	}
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluator.Evaluate,
	)
}

func NewKindDefinednessRegistry(
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
) (KindDefinednessRegistry, error) {
	mechanism, err := evaluationMechanismFromIdentity(identity)
	if err != nil {
		return KindDefinednessRegistry{}, err
	}
	evaluator, err := typedmemorykindruntime.NewKindDefinednessEvaluator(
		rule,
		mechanism,
	)
	if err != nil {
		return KindDefinednessRegistry{}, err
	}
	return newSingleEvaluatorRegistry(
		rule,
		identity,
		evaluator.Evaluate,
	)
}

func evaluationMechanismFromIdentity(
	identity MechanismIdentity,
) (typedmemorykindruntime.EvaluationMechanism, error) {
	if !identity.valid() || identity.Role() != EvaluatorRole {
		return typedmemorykindruntime.EvaluationMechanism{}, fmt.Errorf(
			"kind-runtime evaluator mechanism identity is invalid",
		)
	}
	return typedmemorykindruntime.NewEvaluationMechanism(
		typedmemorykindruntime.EvaluationMechanismInput{
			Artifact: identity.ArtifactRef(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	)
}

func newSingleEvaluatorRegistry[Input, Output any](
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
	evaluate func(Input) (Output, error),
) (Registry[Input, Output], error) {
	mechanism, err := newPureEvaluator(evaluate)
	if err != nil {
		return Registry[Input, Output]{}, err
	}
	registration, err := newRegistration(rule, identity, mechanism)
	if err != nil {
		return Registry[Input, Output]{}, err
	}
	return NewRegistry([]Registration[Input, Output]{registration})
}
