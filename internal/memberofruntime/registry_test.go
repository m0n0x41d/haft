package memberofruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestRegistryKeepsDistinctRulesInCanonicalOrder(t *testing.T) {
	ruleB := mustRule(t, "rule:memberof:b")
	ruleA := mustRule(t, "rule:memberof:a")
	identityB := mustIdentity(t, "carrier:memberof:b", "b")
	identityA := mustIdentity(t, "carrier:memberof:a", "a")
	registrationB := mustRegistration(t, ruleB, identityB, inertEngine{})
	registrationA := mustRegistration(t, ruleA, identityA, inertEngine{})

	registry, err := NewRegistry([]Registration{registrationB, registrationA})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if registry.Len() != 2 {
		t.Fatalf("Len = %d; want 2", registry.Len())
	}
	registrations := registry.Registrations()
	if registrations[0].RuleRef() != ruleA || registrations[1].RuleRef() != ruleB {
		t.Fatalf("Registrations are not canonically ordered")
	}
	foundResult, err := registry.Lookup(ruleA, identityA)
	if err != nil {
		t.Fatalf("Lookup exact registration: %v", err)
	}
	found, ok := foundResult.(Found)
	if !ok || found.Registration().Engine() == nil {
		t.Fatalf("Lookup result = %T; want Found with engine", foundResult)
	}

	registrations[0] = Registration{}
	cloned := registry.Clone().Registrations()
	if cloned[0].RuleRef() != ruleA {
		t.Fatalf("Registrations leaked mutable backing storage")
	}
}

func TestRegistryLookupKeepsIdentityMismatchDistinct(t *testing.T) {
	rule := mustRule(t, "rule:memberof:exact")
	installed := mustIdentity(t, "carrier:memberof:installed", "a")
	expected := mustIdentity(t, "carrier:memberof:expected", "b")
	registration := mustRegistration(t, rule, installed, inertEngine{})
	registry := mustRegistry(t, []Registration{registration})

	result, err := registry.Lookup(rule, expected)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	mismatch, ok := result.(Mismatch)
	if !ok {
		t.Fatalf("Lookup result = %T; want Mismatch", result)
	}
	if mismatch.RegisteredIdentity() != installed || mismatch.ExpectedIdentity() != expected {
		t.Fatalf("Mismatch lost exact identities")
	}
}

func TestRegistryMissingDispatchFailsClosed(t *testing.T) {
	registeredRule := mustRule(t, "rule:memberof:registered")
	missingRule := mustRule(t, "rule:memberof:missing")
	identity := mustIdentity(t, "carrier:memberof:registered", "a")
	registration := mustRegistration(t, registeredRule, identity, inertEngine{})
	registry := mustRegistry(t, []Registration{registration})

	_, found := registry.registrationForRule(missingRule)
	if found {
		t.Fatalf("missing RuleRef unexpectedly dispatched")
	}
	lookup, err := registry.Lookup(missingRule, identity)
	if err != nil {
		t.Fatalf("Lookup missing registration: %v", err)
	}
	if _, ok := lookup.(Missing); !ok {
		t.Fatalf("Lookup result = %T; want Missing", lookup)
	}
	_, err = registry.EvaluateMemberOf(
		context.Background(),
		memberofevaluation.MemberOfEvaluationInput{},
	)
	if err == nil {
		t.Fatalf("EvaluateMemberOf accepted an unresolved exact input")
	}
	selection := registry.SelectSnapshotObservableInputs(
		memberofevaluation.MemberOfEvaluationInput{},
	)
	if _, ok := selection.(memberofevaluation.SnapshotObservableInputsUnavailable); !ok {
		t.Fatalf("snapshot selection = %T; want unavailable", selection)
	}
}

func TestRegistryPreservesNotApplicableSnapshotSelection(t *testing.T) {
	selection := normalizeSnapshotObservableInputSelection(
		memberofevaluation.NewSnapshotObservableInputsNotApplicable(),
	)
	if _, ok := selection.(memberofevaluation.SnapshotObservableInputsNotApplicable); !ok {
		t.Fatalf("normalized selection = %T, want NotApplicable", selection)
	}
	unavailable := normalizeSnapshotObservableInputSelection(nil)
	if _, ok := unavailable.(memberofevaluation.SnapshotObservableInputsUnavailable); !ok {
		t.Fatalf("unknown selection = %T, want Unavailable", unavailable)
	}
}

func TestRegistryRejectsDuplicateAndConflictingRuleRegistrations(t *testing.T) {
	rule := mustRule(t, "rule:memberof:duplicate")
	identityA := mustIdentity(t, "carrier:memberof:a", "a")
	identityB := mustIdentity(t, "carrier:memberof:b", "b")
	registrationA := mustRegistration(t, rule, identityA, inertEngine{})
	registrationB := mustRegistration(t, rule, identityB, inertEngine{})

	_, err := NewRegistry([]Registration{registrationA, registrationA})
	assertConflict(t, err, DuplicateRuleRefRegistration)
	_, err = NewRegistry([]Registration{registrationA, registrationB})
	assertConflict(t, err, ConflictingMechanismIdentity)
}

func TestNewRegistrationRejectsTypedNilEngine(t *testing.T) {
	rule := mustRule(t, "rule:memberof:nil")
	identity := mustIdentity(t, "carrier:memberof:nil", "a")
	var engine *pointerEngine

	_, err := NewRegistration(rule, identity, engine)
	if err == nil {
		t.Fatalf("NewRegistration accepted a typed nil engine")
	}
}

type inertEngine struct{}

func (inertEngine) EvaluateMemberOf(
	context.Context,
	memberofevaluation.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, errors.New("inert test engine")
}

type pointerEngine struct{}

func (*pointerEngine) EvaluateMemberOf(
	context.Context,
	memberofevaluation.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, errors.New("unreachable")
}

func mustRule(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	value, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef: %v", err)
	}
	return value
}

func mustIdentity(
	t *testing.T,
	artifactRaw string,
	digestSeed string,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("NewCarrierRef: %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("NewCarrierEdition: %v", err)
	}
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + repeatHex(digestSeed),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	identity, err := typedmemoryevaluation.NewMechanismIdentity(
		artifact,
		edition,
		digest,
		typedmemoryevaluation.EvaluatorRole,
	)
	if err != nil {
		t.Fatalf("NewMechanismIdentity: %v", err)
	}
	return identity
}

func repeatHex(seed string) string {
	result := ""
	for len(result) < 64 {
		result += seed
	}
	return result[:64]
}

func mustRegistration(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
	engine memberofevaluation.MemberOfEvaluationEngine,
) Registration {
	t.Helper()
	value, err := NewRegistration(rule, identity, engine)
	if err != nil {
		t.Fatalf("NewRegistration: %v", err)
	}
	return value
}

func mustRegistry(t *testing.T, registrations []Registration) Registry {
	t.Helper()
	value, err := NewRegistry(registrations)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return value
}

func assertConflict(
	t *testing.T,
	err error,
	want ConstructionConflictKind,
) {
	t.Helper()
	var conflict ConstructionConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("NewRegistry error = %T %v; want ConstructionConflict", err, err)
	}
	if conflict.Kind() != want {
		t.Fatalf("conflict kind = %s; want %s", conflict.Kind(), want)
	}
}
