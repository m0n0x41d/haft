package memberofruntime

import (
	"cmp"
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

// Registration is one immutable store-facing MemberOf dispatch entry. The
// engine remains an opaque process capability; Identity is an exact coordinate
// supplied by the composition root, not an attestation derived from Engine.
type Registration struct {
	rule     typedmemory.RuleRef
	identity typedmemoryevaluation.MechanismIdentity
	engine   memberofevaluation.MemberOfEvaluationEngine
}

func NewRegistration(
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
	engine memberofevaluation.MemberOfEvaluationEngine,
) (Registration, error) {
	if err := validateRuleRef(rule); err != nil {
		return Registration{}, err
	}
	if err := validateIdentity(identity); err != nil {
		return Registration{}, err
	}
	if nilInterface(engine) {
		return Registration{}, fmt.Errorf("MemberOf evaluation engine is required")
	}
	return Registration{
		rule:     rule,
		identity: identity,
		engine:   engine,
	}, nil
}

func (registration Registration) RuleRef() typedmemory.RuleRef {
	return registration.rule
}

func (registration Registration) Identity() typedmemoryevaluation.MechanismIdentity {
	return registration.identity
}

func (registration Registration) Engine() memberofevaluation.MemberOfEvaluationEngine {
	return registration.engine
}

func (registration Registration) valid() bool {
	rebuilt, err := NewRegistration(
		registration.rule,
		registration.identity,
		registration.engine,
	)
	return err == nil && sameRegistrationCoordinates(rebuilt, registration)
}

type ConstructionConflictKind uint8

const (
	DuplicateRuleRefRegistration ConstructionConflictKind = iota + 1
	ConflictingMechanismIdentity
)

func (kind ConstructionConflictKind) String() string {
	switch kind {
	case DuplicateRuleRefRegistration:
		return "duplicate_rule_ref_registration"
	case ConflictingMechanismIdentity:
		return "conflicting_mechanism_identity"
	default:
		return ""
	}
}

// ConstructionConflict reports the first conflict in canonical RuleRef order.
type ConstructionConflict struct {
	kind       ConstructionConflictKind
	rule       typedmemory.RuleRef
	identities []typedmemoryevaluation.MechanismIdentity
}

func (conflict ConstructionConflict) Error() string {
	return fmt.Sprintf(
		"MemberOf runtime registry %s for RuleRef %q",
		conflict.kind.String(),
		conflict.rule.String(),
	)
}

func (conflict ConstructionConflict) Kind() ConstructionConflictKind {
	return conflict.kind
}

func (conflict ConstructionConflict) RuleRef() typedmemory.RuleRef {
	return conflict.rule
}

func (conflict ConstructionConflict) Identities() []typedmemoryevaluation.MechanismIdentity {
	return append([]typedmemoryevaluation.MechanismIdentity(nil), conflict.identities...)
}

// Registry is an immutable, canonically ordered MemberOf dispatcher.
type Registry struct {
	registrations []Registration
}

func NewRegistry(registrations []Registration) (Registry, error) {
	if len(registrations) > typedmemoryevaluation.MaxRegistryRegistrations {
		return Registry{}, fmt.Errorf(
			"MemberOf runtime registry has %d registrations; maximum is %d",
			len(registrations),
			typedmemoryevaluation.MaxRegistryRegistrations,
		)
	}
	normalized := append([]Registration(nil), registrations...)
	for index, registration := range normalized {
		if !registration.valid() {
			return Registry{}, fmt.Errorf(
				"MemberOf runtime registration %d is invalid",
				index,
			)
		}
	}
	slices.SortFunc(normalized, compareRegistrations)
	if conflict := firstConstructionConflict(normalized); conflict != nil {
		return Registry{}, *conflict
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

type LookupResultKind uint8

const (
	FoundResult LookupResultKind = iota + 1
	MissingResult
	MismatchResult
)

func (kind LookupResultKind) String() string {
	switch kind {
	case FoundResult:
		return "found"
	case MissingResult:
		return "missing"
	case MismatchResult:
		return "mismatch"
	default:
		return ""
	}
}

type LookupResult interface {
	Kind() LookupResultKind
	RuleRef() typedmemory.RuleRef
	lookupResultVariant()
}

type Found interface {
	LookupResult
	Registration() Registration
	foundLookupResult()
}

type found struct{ registration Registration }

func (found) Kind() LookupResultKind { return FoundResult }

func (result found) RuleRef() typedmemory.RuleRef {
	return result.registration.rule
}

func (result found) Registration() Registration { return result.registration }

func (found) lookupResultVariant() {}

func (found) foundLookupResult() {}

type Missing interface {
	LookupResult
	ExpectedIdentity() typedmemoryevaluation.MechanismIdentity
	missingLookupResult()
}

type missing struct {
	rule     typedmemory.RuleRef
	expected typedmemoryevaluation.MechanismIdentity
}

func (missing) Kind() LookupResultKind { return MissingResult }

func (result missing) RuleRef() typedmemory.RuleRef { return result.rule }

func (result missing) ExpectedIdentity() typedmemoryevaluation.MechanismIdentity {
	return result.expected
}

func (missing) lookupResultVariant() {}

func (missing) missingLookupResult() {}

type Mismatch interface {
	LookupResult
	RegisteredIdentity() typedmemoryevaluation.MechanismIdentity
	ExpectedIdentity() typedmemoryevaluation.MechanismIdentity
	mismatchLookupResult()
}

type mismatch struct {
	registration Registration
	expected     typedmemoryevaluation.MechanismIdentity
}

func (mismatch) Kind() LookupResultKind { return MismatchResult }

func (result mismatch) RuleRef() typedmemory.RuleRef {
	return result.registration.rule
}

func (result mismatch) RegisteredIdentity() typedmemoryevaluation.MechanismIdentity {
	return result.registration.identity
}

func (result mismatch) ExpectedIdentity() typedmemoryevaluation.MechanismIdentity {
	return result.expected
}

func (mismatch) lookupResultVariant() {}

func (mismatch) mismatchLookupResult() {}

func (registry Registry) Lookup(
	rule typedmemory.RuleRef,
	expected typedmemoryevaluation.MechanismIdentity,
) (LookupResult, error) {
	if err := validateRuleRef(rule); err != nil {
		return nil, err
	}
	if err := validateIdentity(expected); err != nil {
		return nil, err
	}
	registration, exists := registry.registrationForRule(rule)
	if !exists {
		return missing{rule: rule, expected: expected}, nil
	}
	if registration.identity != expected {
		return mismatch{registration: registration, expected: expected}, nil
	}
	return found{registration: registration}, nil
}

// EvaluateMemberOf dispatches only through the exact evaluator RuleRef carried
// by the query's selected TypeEnv KindSignature. Registry lookup does not make
// a kind available and cannot substitute a similarly named rule.
func (registry Registry) EvaluateMemberOf(
	ctx context.Context,
	input memberofevaluation.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	registration, err := registry.registrationForInput(input)
	if err != nil {
		return nil, err
	}
	judgement, err := registration.engine.EvaluateMemberOf(ctx, input)
	if err != nil {
		return nil, err
	}
	if !typedmemory.MemberOfJudgementMatchesRequest(input.Request(), judgement) {
		return nil, fmt.Errorf(
			"MemberOf evaluator %q returned a judgement outside the exact request",
			registration.rule.String(),
		)
	}
	return judgement, nil
}

// SelectSnapshotObservableInputs delegates only when the exact selected engine
// implements the optional selector contract. Every unresolved posture remains
// explicitly unavailable.
func (registry Registry) SelectSnapshotObservableInputs(
	input memberofevaluation.MemberOfEvaluationInput,
) memberofevaluation.SnapshotObservableInputSelection {
	registration, err := registry.registrationForInput(input)
	if err != nil {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	selector, selectable := registration.engine.(memberofevaluation.SnapshotObservableInputSelector)
	if !selectable || nilInterface(selector) {
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
	return normalizeSnapshotObservableInputSelection(
		selector.SelectSnapshotObservableInputs(input),
	)
}

func normalizeSnapshotObservableInputSelection(
	selection memberofevaluation.SnapshotObservableInputSelection,
) memberofevaluation.SnapshotObservableInputSelection {
	switch selection.(type) {
	case memberofevaluation.SnapshotObservableInputsSelected:
		return selection
	case memberofevaluation.SnapshotObservableInputsNotApplicable:
		return selection
	case memberofevaluation.SnapshotObservableInputsUnavailable:
		return selection
	default:
		return memberofevaluation.NewSnapshotObservableInputsUnavailable()
	}
}

func (registry Registry) registrationForInput(
	input memberofevaluation.MemberOfEvaluationInput,
) (Registration, error) {
	query := input.Request().Query()
	signature, found := input.Environment().KindSignatureDefinition(
		query.ValueKind(),
		query.ContextSlice().Context(),
	)
	if !found {
		return Registration{}, fmt.Errorf(
			"MemberOf KindSignature is unavailable in the exact TypeEnv",
		)
	}
	registration, found := registry.registrationForRule(signature.Evaluator())
	if !found {
		return Registration{}, fmt.Errorf(
			"MemberOf evaluator %q is not installed",
			signature.Evaluator().String(),
		)
	}
	return registration, nil
}

func (registry Registry) registrationForRule(
	rule typedmemory.RuleRef,
) (Registration, bool) {
	index, found := slices.BinarySearchFunc(
		registry.registrations,
		rule,
		compareRegistrationRule,
	)
	if !found {
		return Registration{}, false
	}
	return registry.registrations[index], true
}

func compareRegistrations(left, right Registration) int {
	if order := cmp.Compare(left.rule.String(), right.rule.String()); order != 0 {
		return order
	}
	return compareMechanismIdentities(left.identity, right.identity)
}

func compareRegistrationRule(
	registration Registration,
	rule typedmemory.RuleRef,
) int {
	return cmp.Compare(registration.rule.String(), rule.String())
}

func compareMechanismIdentities(
	left typedmemoryevaluation.MechanismIdentity,
	right typedmemoryevaluation.MechanismIdentity,
) int {
	if order := cmp.Compare(left.ArtifactRef().String(), right.ArtifactRef().String()); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Edition().String(), right.Edition().String()); order != 0 {
		return order
	}
	if order := cmp.Compare(left.Digest().String(), right.Digest().String()); order != 0 {
		return order
	}
	return cmp.Compare(left.Role().String(), right.Role().String())
}

func sameRegistrationCoordinates(left, right Registration) bool {
	return left.rule == right.rule &&
		left.identity == right.identity &&
		reflect.ValueOf(left.engine) == reflect.ValueOf(right.engine)
}

func firstConstructionConflict(registrations []Registration) *ConstructionConflict {
	for start := 0; start < len(registrations); {
		end := registrationGroupEnd(registrations, start)
		if end-start > 1 {
			return buildConstructionConflict(registrations[start:end])
		}
		start = end
	}
	return nil
}

func registrationGroupEnd(registrations []Registration, start int) int {
	rule := registrations[start].rule
	end := start + 1
	for end < len(registrations) && registrations[end].rule == rule {
		end++
	}
	return end
}

func buildConstructionConflict(group []Registration) *ConstructionConflict {
	identities := make([]typedmemoryevaluation.MechanismIdentity, 0, len(group))
	for _, registration := range group {
		if len(identities) == 0 || identities[len(identities)-1] != registration.identity {
			identities = append(identities, registration.identity)
		}
	}
	kind := DuplicateRuleRefRegistration
	if len(identities) > 1 {
		kind = ConflictingMechanismIdentity
	}
	return &ConstructionConflict{
		kind:       kind,
		rule:       group[0].rule,
		identities: identities,
	}
}

func validateRuleRef(rule typedmemory.RuleRef) error {
	rebuilt, err := typedmemory.NewRuleRef(rule.String())
	if err != nil || rebuilt != rule {
		return fmt.Errorf("MemberOf evaluator RuleRef is invalid")
	}
	return nil
}

func validateIdentity(identity typedmemoryevaluation.MechanismIdentity) error {
	rebuilt, err := typedmemoryevaluation.NewMechanismIdentity(
		identity.ArtifactRef(),
		identity.Edition(),
		identity.Digest(),
		identity.Role(),
	)
	if err != nil || rebuilt != identity {
		return fmt.Errorf("MemberOf evaluator mechanism identity is invalid")
	}
	if identity.Role() != typedmemoryevaluation.EvaluatorRole {
		return fmt.Errorf("MemberOf evaluator mechanism role is unsupported")
	}
	return nil
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

var _ memberofevaluation.MemberOfEvaluationEngine = Registry{}
var _ memberofevaluation.SnapshotObservableInputSelector = Registry{}
