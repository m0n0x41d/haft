package kindclassificationruntime

import (
	"cmp"
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/m0n0x41d/haft/internal/kindclassificationevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

type Registration struct {
	rule     typedmemory.RuleRef
	identity typedmemoryevaluation.MechanismIdentity
	engine   kindclassificationevaluation.Engine
}

func NewRegistration(
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
	engine kindclassificationevaluation.Engine,
) (Registration, error) {
	parsedRule, err := typedmemory.NewRuleRef(rule.String())
	if err != nil || parsedRule != rule {
		return Registration{}, fmt.Errorf("classification evaluator RuleRef is invalid")
	}
	parsedIdentity, err := typedmemoryevaluation.NewMechanismIdentity(
		identity.ArtifactRef(),
		identity.Edition(),
		identity.Digest(),
		identity.Role(),
	)
	if err != nil || parsedIdentity != identity ||
		identity.Role() != typedmemoryevaluation.EvaluatorRole {
		return Registration{}, fmt.Errorf("classification evaluator identity is invalid")
	}
	if nilInterface(engine) {
		return Registration{}, fmt.Errorf("classification evaluator engine is required")
	}
	return Registration{rule: rule, identity: identity, engine: engine}, nil
}

func (registration Registration) RuleRef() typedmemory.RuleRef { return registration.rule }

func (registration Registration) Identity() typedmemoryevaluation.MechanismIdentity {
	return registration.identity
}

func (registration Registration) Engine() kindclassificationevaluation.Engine {
	return registration.engine
}

func (registration Registration) valid() bool {
	rebuilt, err := NewRegistration(
		registration.rule,
		registration.identity,
		registration.engine,
	)
	return err == nil && rebuilt.rule == registration.rule &&
		rebuilt.identity == registration.identity &&
		reflect.ValueOf(rebuilt.engine) == reflect.ValueOf(registration.engine)
}

type Registry struct {
	registrations []Registration
}

func NewRegistry(registrations []Registration) (Registry, error) {
	if len(registrations) > typedmemoryevaluation.MaxRegistryRegistrations {
		return Registry{}, fmt.Errorf("classification registry exceeds its bounded capacity")
	}
	normalized := append([]Registration(nil), registrations...)
	for index, registration := range normalized {
		if !registration.valid() {
			return Registry{}, fmt.Errorf("classification registration %d is invalid", index)
		}
	}
	slices.SortFunc(normalized, compareRegistrations)
	for index := 1; index < len(normalized); index++ {
		if normalized[index-1].rule == normalized[index].rule {
			return Registry{}, fmt.Errorf(
				"classification registry has duplicate RuleRef %q",
				normalized[index].rule.String(),
			)
		}
	}
	return Registry{registrations: normalized}, nil
}

func (registry Registry) Len() int { return len(registry.registrations) }

func (registry Registry) Registrations() []Registration {
	return append([]Registration(nil), registry.registrations...)
}

func (registry Registry) Clone() Registry {
	return Registry{registrations: registry.Registrations()}
}

func (registry Registry) Registration(
	rule typedmemory.RuleRef,
) (Registration, bool) {
	index, found := slices.BinarySearchFunc(
		registry.registrations,
		rule,
		func(registration Registration, target typedmemory.RuleRef) int {
			return cmp.Compare(registration.rule.String(), target.String())
		},
	)
	if !found {
		return Registration{}, false
	}
	return registry.registrations[index], true
}

func (registry Registry) EvaluateKindClassification(
	ctx context.Context,
	input kindclassificationevaluation.EvaluationInput,
) (typedmemory.KindClassificationJudgement, error) {
	if !input.Valid() {
		return nil, fmt.Errorf("classification registry input is invalid")
	}
	rule := input.Signature().Criterion()
	registration, found := registry.Registration(rule)
	if !found {
		return unknownJudgement(
			input.Request(),
			typedmemory.KindUnknownCriterionUnavailable,
			"repair:kind-classification/install/"+rule.String(),
		)
	}
	judgement, err := registration.engine.EvaluateKindClassification(ctx, input)
	if err != nil {
		return nil, err
	}
	if !typedmemory.KindClassificationJudgementMatchesRequest(
		input.Request(),
		judgement,
	) {
		return nil, fmt.Errorf(
			"classification evaluator %q returned an uncorrelated judgement",
			rule.String(),
		)
	}
	return judgement, nil
}

func compareRegistrations(left Registration, right Registration) int {
	if order := cmp.Compare(left.rule.String(), right.rule.String()); order != 0 {
		return order
	}
	if order := cmp.Compare(
		left.identity.ArtifactRef().String(),
		right.identity.ArtifactRef().String(),
	); order != 0 {
		return order
	}
	if order := cmp.Compare(
		left.identity.Edition().String(),
		right.identity.Edition().String(),
	); order != 0 {
		return order
	}
	return cmp.Compare(
		left.identity.Digest().String(),
		right.identity.Digest().String(),
	)
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ kindclassificationevaluation.Engine = Registry{}
