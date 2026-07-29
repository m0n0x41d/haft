package typedmemoryevaluation_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

type evaluatorInput struct {
	value int
}

type evaluatorOutput struct {
	value int
}

func TestRegistryPermutationDeterminism(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x11)
	forward := []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{
		testRegistration(t, "rule:z", identity, evaluator),
		testRegistration(t, "rule:a", identity, evaluator),
		testRegistration(t, "rule:m", identity, evaluator),
	}
	reverse := append(
		[]typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput](nil),
		forward...,
	)
	slices.Reverse(reverse)
	left := testRegistry(t, forward)
	right := testRegistry(t, reverse)
	leftRules := registrationRules(left.Registrations())
	rightRules := registrationRules(right.Registrations())
	want := []string{"rule:a", "rule:m", "rule:z"}
	if !slices.Equal(leftRules, want) || !slices.Equal(rightRules, want) {
		t.Fatalf("canonical rules = %v / %v; want %v", leftRules, rightRules, want)
	}
	for _, rule := range want {
		leftResult := testLookup(t, left, testRule(t, rule), identity)
		rightResult := testLookup(t, right, testRule(t, rule), identity)
		if leftResult.Kind() != typedmemoryevaluation.FoundResult ||
			rightResult.Kind() != typedmemoryevaluation.FoundResult {
			t.Fatalf("lookup %q = %s / %s; want found", rule, leftResult.Kind(), rightResult.Kind())
		}
	}
}

func TestRegistryRejectsDuplicateRuleRefDeterministically(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x22)
	registration := testRegistration(t, "rule:duplicate", identity, evaluator)
	registrations := []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{
		registration,
		registration,
	}
	assertConstructionConflict(
		t,
		registrations,
		typedmemoryevaluation.DuplicateRuleRefRegistration,
		"rule:duplicate",
	)
}

func TestRegistryRejectsConflictingMechanismIdentityDeterministically(t *testing.T) {
	evaluator := testEvaluator(t)
	leftIdentity := testIdentity(t, "artifact:a", "1.0.0", 0x33)
	rightIdentity := testIdentity(t, "artifact:b", "2.0.0", 0x44)
	left := testRegistration(t, "rule:conflict", leftIdentity, evaluator)
	right := testRegistration(t, "rule:conflict", rightIdentity, evaluator)
	forward := []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{left, right}
	reverse := []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{right, left}
	forwardError := registryConstructionError(t, forward)
	reverseError := registryConstructionError(t, reverse)
	if forwardError.Error() != reverseError.Error() {
		t.Fatalf("permuted conflicts differ: %q / %q", forwardError, reverseError)
	}
	assertConstructionConflict(
		t,
		forward,
		typedmemoryevaluation.ConflictingMechanismIdentity,
		"rule:conflict",
	)
}

func TestRegistryLookupReturnsMissing(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x55)
	registration := testRegistration(t, "rule:present", identity, evaluator)
	registry := testRegistry(t, []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{registration})
	rule := testRule(t, "rule:missing")
	result := testLookup(t, registry, rule, identity)
	missing, ok := result.(typedmemoryevaluation.Missing[evaluatorInput, evaluatorOutput])
	if !ok {
		t.Fatalf("lookup = %T; want Missing", result)
	}
	if missing.RuleRef() != rule || missing.ExpectedIdentity() != identity {
		t.Fatal("Missing did not preserve exact RuleRef and expected identity")
	}
}

func TestRegistryLookupReturnsExactHitAndCallableMechanism(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x66)
	rule := testRule(t, "rule:exact")
	registration := testRegistration(t, rule.String(), identity, evaluator)
	registry := testRegistry(t, []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{registration})
	result := testLookup(t, registry, rule, identity)
	found, ok := result.(typedmemoryevaluation.Found[evaluatorInput, evaluatorOutput])
	if !ok {
		t.Fatalf("lookup = %T; want Found", result)
	}
	output, err := found.Registration().Evaluator().Evaluate(evaluatorInput{value: 20})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if output.value != 42 {
		t.Fatalf("Evaluate output = %d; want 42", output.value)
	}
}

func TestRegistryLookupReturnsIdentityMismatch(t *testing.T) {
	evaluator := testEvaluator(t)
	registered := testIdentity(t, "artifact:evaluator", "1.0.0", 0x77)
	expected := testIdentity(t, "artifact:evaluator", "2.0.0", 0x77)
	rule := testRule(t, "rule:mismatch")
	registration := testRegistration(t, rule.String(), registered, evaluator)
	registry := testRegistry(t, []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{registration})
	result := testLookup(t, registry, rule, expected)
	mismatch, ok := result.(typedmemoryevaluation.Mismatch[evaluatorInput, evaluatorOutput])
	if !ok {
		t.Fatalf("lookup = %T; want Mismatch", result)
	}
	if mismatch.RegisteredIdentity() != registered || mismatch.ExpectedIdentity() != expected {
		t.Fatal("Mismatch did not preserve registered and expected identities")
	}
}

func TestRegistryConstructionAndSnapshotsAreMutationIsolated(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x88)
	first := testRegistration(t, "rule:first", identity, evaluator)
	second := testRegistration(t, "rule:second", identity, evaluator)
	input := []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{first}
	registry := testRegistry(t, input)
	input[0] = second
	snapshot := registry.Registrations()
	snapshot[0] = second
	clone := registry.Clone()
	for _, candidate := range []typedmemoryevaluation.Registry[evaluatorInput, evaluatorOutput]{registry, clone} {
		result := testLookup(t, candidate, first.RuleRef(), identity)
		if result.Kind() != typedmemoryevaluation.FoundResult {
			t.Fatalf("isolated lookup = %s; want found", result.Kind())
		}
		missing := testLookup(t, candidate, second.RuleRef(), identity)
		if missing.Kind() != typedmemoryevaluation.MissingResult {
			t.Fatalf("mutated registration leaked into registry: %s", missing.Kind())
		}
	}
}

func TestRegistryConcurrentLookupIsReadOnly(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x89)
	rule := testRule(t, "rule:concurrent")
	registration := testRegistration(t, rule.String(), identity, evaluator)
	registry := testRegistry(t, []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{registration})
	errorsFound := make(chan error, 32)
	var workers sync.WaitGroup
	for worker := 0; worker < cap(errorsFound); worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for iteration := 0; iteration < 64; iteration++ {
				result, err := registry.Lookup(rule, identity)
				if err != nil {
					errorsFound <- err
					return
				}
				if result.Kind() != typedmemoryevaluation.FoundResult {
					errorsFound <- fmt.Errorf("lookup kind = %s", result.Kind())
					return
				}
			}
		}()
	}
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
}

func TestRegistryEnforcesResourceBounds(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0x99)
	registrations := make(
		[]typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput],
		0,
		typedmemoryevaluation.MaxRegistryRegistrations+1,
	)
	for index := 0; index <= typedmemoryevaluation.MaxRegistryRegistrations; index++ {
		rule := fmt.Sprintf("rule:bounded:%04d", index)
		registration := testRegistration(t, rule, identity, evaluator)
		registrations = append(registrations, registration)
	}
	_, err := typedmemoryevaluation.NewRegistry(registrations)
	if err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("NewRegistry oversized error = %v; want resource-bound rejection", err)
	}
}

func TestRegistryPresenceDoesNotCreateContextKindAvailability(t *testing.T) {
	evaluator := testEvaluator(t)
	identity := testIdentity(t, "artifact:evaluator", "1.0.0", 0xaa)
	rule := testRule(t, "rule:presence-only")
	registration := testRegistration(t, rule.String(), identity, evaluator)
	registry := testRegistry(t, []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput]{registration})
	var environment typedmemory.TypeEnv
	before := environment.ContextKindAvailabilities()
	result := testLookup(t, registry, rule, identity)
	after := environment.ContextKindAvailabilities()
	if result.Kind() != typedmemoryevaluation.FoundResult {
		t.Fatalf("lookup = %s; want found", result.Kind())
	}
	if len(before) != 0 || len(after) != 0 {
		t.Fatalf("registry lookup changed ContextKindAvailabilities: before=%d after=%d", len(before), len(after))
	}
}

func TestPureEvaluatorZeroValueFailsWithoutPanic(t *testing.T) {
	var evaluator typedmemoryevaluation.PureEvaluator[evaluatorInput, evaluatorOutput]
	output, err := evaluator.Evaluate(evaluatorInput{value: 20})
	if err == nil {
		t.Fatal("zero PureEvaluator succeeded")
	}
	if output != (evaluatorOutput{}) {
		t.Fatalf("zero PureEvaluator output = %#v; want zero output", output)
	}
}

func TestMechanismIdentityUsesClosedExactEditionGrammar(t *testing.T) {
	accepted := []string{
		"1.0.0",
		"2.3.4-rc.1+build.7",
		"build-20260717.1",
		"build-20260717.2.arm64",
	}
	for _, edition := range accepted {
		if err := mechanismIdentityEditionError(t, edition); err != nil {
			t.Errorf("exact edition %q rejected: %v", edition, err)
		}
	}
	rejected := []string{
		"latest",
		"main",
		"stable",
		"*",
		"1.x",
		"^1.2.3",
		">=1.2.3",
		"v1.2.3",
		"1.2",
		"1.2.3-01",
		"build-20260717.01",
	}
	for _, edition := range rejected {
		err := mechanismIdentityEditionError(t, edition)
		if err == nil || !strings.Contains(err.Error(), "must be an exact") {
			t.Errorf("moving or malformed edition %q error = %v", edition, err)
		}
	}
}

func TestRegistryIdentityCoordinatesMatchRuntimeBasisBounds(t *testing.T) {
	const (
		maximumRuleRefBytes    = 1 << 10
		maximumCoordinateBytes = 4 << 10
	)

	boundaryArtifact := strings.Repeat("a", maximumCoordinateBytes)
	boundaryEditionPrefix := "1.0.0+"
	boundaryEdition := boundaryEditionPrefix + strings.Repeat(
		"a",
		maximumCoordinateBytes-len(boundaryEditionPrefix),
	)
	identity := testIdentity(t, boundaryArtifact, boundaryEdition, 0xcd)

	registry, err := typedmemoryevaluation.NewRegistry[
		evaluatorInput,
		evaluatorOutput,
	](nil)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	boundaryRule := testRule(t, strings.Repeat("r", maximumRuleRefBytes))
	if _, err := registry.Lookup(boundaryRule, identity); err != nil {
		t.Fatalf("boundary RuleRef lookup rejected: %v", err)
	}

	oversizedRule := testRule(t, strings.Repeat("r", maximumRuleRefBytes+1))
	if _, err := registry.Lookup(oversizedRule, identity); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized RuleRef error = %v; want bounded rejection", err)
	}

	_, err = typedmemoryevaluation.NewMechanismIdentity(
		testCarrierRef(t, strings.Repeat("a", maximumCoordinateBytes+1)),
		testCarrierEdition(t, "1.0.0"),
		testDigest(t, 0xce),
		typedmemoryevaluation.EvaluatorRole,
	)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized artifact error = %v; want bounded rejection", err)
	}

	oversizedEdition := boundaryEdition + "a"
	_, err = typedmemoryevaluation.NewMechanismIdentity(
		testCarrierRef(t, "artifact:evaluator"),
		testCarrierEdition(t, oversizedEdition),
		testDigest(t, 0xcf),
		typedmemoryevaluation.EvaluatorRole,
	)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized edition error = %v; want bounded rejection", err)
	}
}

func TestVettedRecordMembershipCatalogBindsFixedRule(t *testing.T) {
	identity := testIdentity(t, "artifact:record-membership", "1.0.0", 0xcc)
	registry, err := typedmemoryevaluation.NewRecordMembershipRegistry(identity)
	if err != nil {
		t.Fatalf("NewRecordMembershipRegistry: %v", err)
	}
	evaluator := recordcarrier.NewRecordMembershipEvaluatorV1()
	result, err := registry.Lookup(evaluator.RuleRef(), identity)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if result.Kind() != typedmemoryevaluation.FoundResult {
		t.Fatalf("vetted catalog lookup = %s; want found", result.Kind())
	}
	if registry.Len() != 1 {
		t.Fatalf("vetted catalog registrations = %d; want 1", registry.Len())
	}
}

func mechanismIdentityEditionError(t *testing.T, raw string) error {
	t.Helper()
	_, err := typedmemoryevaluation.NewMechanismIdentity(
		testCarrierRef(t, "artifact:evaluator"),
		testCarrierEdition(t, raw),
		testDigest(t, 0xbb),
		typedmemoryevaluation.EvaluatorRole,
	)
	return err
}

func assertConstructionConflict(
	t *testing.T,
	registrations []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput],
	wantKind typedmemoryevaluation.ConstructionConflictKind,
	wantRule string,
) {
	t.Helper()
	err := registryConstructionError(t, registrations)
	var conflict typedmemoryevaluation.ConstructionConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("NewRegistry error = %T %v; want ConstructionConflict", err, err)
	}
	if conflict.Kind() != wantKind || conflict.RuleRef().String() != wantRule {
		t.Fatalf(
			"conflict = %s %q; want %s %q",
			conflict.Kind(),
			conflict.RuleRef().String(),
			wantKind,
			wantRule,
		)
	}
}

func registryConstructionError(
	t *testing.T,
	registrations []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput],
) error {
	t.Helper()
	_, err := typedmemoryevaluation.NewRegistry(registrations)
	if err == nil {
		t.Fatal("NewRegistry succeeded; want conflict")
	}
	return err
}

func testRegistry(
	t *testing.T,
	registrations []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput],
) typedmemoryevaluation.Registry[evaluatorInput, evaluatorOutput] {
	t.Helper()
	registry, err := typedmemoryevaluation.NewRegistry(registrations)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return registry
}

func testLookup(
	t *testing.T,
	registry typedmemoryevaluation.Registry[evaluatorInput, evaluatorOutput],
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) typedmemoryevaluation.LookupResult[evaluatorInput, evaluatorOutput] {
	t.Helper()
	result, err := registry.Lookup(rule, identity)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if result == nil {
		t.Fatal("Lookup returned nil result")
	}
	return result
}

func testRegistration(
	t *testing.T,
	rule string,
	identity typedmemoryevaluation.MechanismIdentity,
	evaluator typedmemoryevaluation.PureEvaluator[evaluatorInput, evaluatorOutput],
) typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput] {
	t.Helper()
	registration, err := typedmemoryevaluation.NewRegistrationForTest(
		testRule(t, rule),
		identity,
		evaluator,
	)
	if err != nil {
		t.Fatalf("NewRegistration: %v", err)
	}
	return registration
}

func testEvaluator(
	t *testing.T,
) typedmemoryevaluation.PureEvaluator[evaluatorInput, evaluatorOutput] {
	t.Helper()
	evaluator, err := typedmemoryevaluation.NewPureEvaluatorForTest(
		func(input evaluatorInput) (evaluatorOutput, error) {
			return evaluatorOutput{value: input.value + 22}, nil
		},
	)
	if err != nil {
		t.Fatalf("NewPureEvaluator: %v", err)
	}
	return evaluator
}

func testIdentity(
	t *testing.T,
	artifact string,
	edition string,
	digestFill byte,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	identity, err := typedmemoryevaluation.NewMechanismIdentity(
		testCarrierRef(t, artifact),
		testCarrierEdition(t, edition),
		testDigest(t, digestFill),
		typedmemoryevaluation.EvaluatorRole,
	)
	if err != nil {
		t.Fatalf("NewMechanismIdentity: %v", err)
	}
	return identity
}

func testRule(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef: %v", err)
	}
	return rule
}

func testCarrierRef(t *testing.T, raw string) typedmemory.CarrierRef {
	t.Helper()
	ref, err := typedmemory.NewCarrierRef(raw)
	if err != nil {
		t.Fatalf("NewCarrierRef: %v", err)
	}
	return ref
}

func testCarrierEdition(t *testing.T, raw string) typedmemory.CarrierEdition {
	t.Helper()
	edition, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition: %v", err)
	}
	return edition
}

func testDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(fmt.Sprintf("%02x", fill), 32)
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	return digest
}

func registrationRules(
	registrations []typedmemoryevaluation.Registration[evaluatorInput, evaluatorOutput],
) []string {
	rules := make([]string, 0, len(registrations))
	for _, registration := range registrations {
		rules = append(rules, registration.RuleRef().String())
	}
	return rules
}
