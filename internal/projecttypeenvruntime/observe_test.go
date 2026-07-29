package projecttypeenvruntime_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorykindruntime"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type runtimeFixture struct {
	basis      projecttypeenv.RuntimeEvaluationBasisArtifact
	catalog    runtimemechanism.RuntimeMechanismArtifactV1
	policy     recordmembershipregistration.RegistrationArtifactV1
	codec      typedmemory.CodecRef
	rule       typedmemory.RuleRef
	codecs     typedmemory.CodecRegistry
	evaluators projecttypeenvruntime.MemberOfEvaluatorRegistry
}

type multipleMemberOfRuntimeFixture struct {
	basis    projecttypeenv.RuntimeEvaluationBasisArtifact
	catalog  runtimemechanism.RuntimeMechanismArtifactV1
	policies []recordmembershipregistration.RegistrationArtifactV1
	codecs   typedmemory.CodecRegistry
	rules    []typedmemory.RuleRef
	identity typedmemoryevaluation.MechanismIdentity
}

type inertCodec struct{}

func (inertCodec) Canonicalize(
	typedmemory.ValueShapeRef,
	[]byte,
) typedmemory.CodecCanonicalization {
	return typedmemory.RejectedCodecValue{}
}

// runtimeObservationMemberOfEngine is deliberately never invoked by these
// exact-coordinate tests. It is a concrete callable capability rather than a
// fabricated positive judgement: an accidental invocation fails closed.
type runtimeObservationMemberOfEngine struct{}

func (runtimeObservationMemberOfEngine) EvaluateMemberOf(
	context.Context,
	typedmemorystore.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, fmt.Errorf("runtime-observation MemberOf evaluator is not executable in this fixture")
}

func runtimeMemberOfRegistry(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) projecttypeenvruntime.MemberOfEvaluatorRegistry {
	t.Helper()
	registration := mustValue(memberofruntime.NewRegistration(
		rule,
		identity,
		runtimeObservationMemberOfEngine{},
	))
	return mustValue(memberofruntime.NewRegistry(
		[]memberofruntime.Registration{registration},
	))
}

func TestObserveCurrentTargetRuntimeMatchesExactInstalledCoordinates(t *testing.T) {
	t.Parallel()
	fixture := buildRuntimeFixture(t)
	result := observeFixture(fixture)
	if result.Kind() != projecttypeenvruntime.ResolutionMatched {
		t.Fatalf("result.Kind() = %s, want matched; issues = %#v", result.Kind(), issuesFromResult(result))
	}
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("result = %T, want Matched", result)
	}
	registry, ok := matched.Registry()
	if !ok || !registry.Valid() {
		t.Fatal("matched result did not expose a valid exact target runtime registry")
	}
	runtimeBasis, ok := registry.RuntimeBasisRef()
	if !ok || runtimeBasis != fixture.basis.Ref() {
		t.Fatalf("RuntimeBasisRef() = %v, %v; want %s", runtimeBasis, ok, fixture.basis.Ref())
	}
	digest, ok := registry.CoordinateDigest()
	if !ok || digest.String() == "" {
		t.Fatal("matched runtime registry has no coordinate digest")
	}
	codecs, ok := registry.CodecRegistry()
	if !ok || !codecs.Contains(fixture.codec) {
		t.Fatal("matched runtime registry did not retain the exact codec implementation")
	}
	evaluators, ok := registry.MemberOfRegistry()
	if !ok || evaluators.Len() != 1 {
		t.Fatalf("record-membership registry length = %d, %v; want 1, true", evaluators.Len(), ok)
	}
	policy, ok := registry.RegistrationPolicy()
	if !ok {
		t.Fatal("matched runtime registry did not expose registration-policy posture")
	}
	exact, ok := policy.(projecttypeenvruntime.ExactTargetRegistrationPolicy)
	if !ok {
		t.Fatalf("RegistrationPolicy() = %T, want ExactTargetRegistrationPolicy", policy)
	}
	artifact, ok := exact.Artifact()
	if !ok || artifact.Ref() != fixture.policy.Ref() {
		t.Fatalf("registration policy = %v, %v; want %s", artifact.Ref(), ok, fixture.policy.Ref())
	}
	if _, err := json.Marshal(registry); !errors.Is(
		err,
		projecttypeenvruntime.ErrExactTargetRuntimeRegistryNotSerializable,
	) {
		t.Fatalf("json.Marshal() error = %v, want non-serializable sentinel", err)
	}
}

func TestObserveCurrentTargetRuntimeKeepsMultipleMemberOfFamiliesExact(
	t *testing.T,
) {
	t.Parallel()
	fixture := buildMultipleMemberOfRuntimeFixture(t)
	firstOnly := runtimeMemberOfRegistry(
		t,
		fixture.rules[0],
		fixture.identity,
	)
	missing := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: fixture.basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: firstOnly,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					fixture.catalog,
				},
				RegistrationPolicies: append(
					[]recordmembershipregistration.RegistrationArtifactV1(nil),
					fixture.policies...,
				),
			},
		},
	)
	if missing.Kind() != projecttypeenvruntime.ResolutionUnavailable ||
		!containsIssueCode(missing, projecttypeenvruntime.IssueEvaluatorRegistrationMissing) {
		t.Fatalf(
			"single-family runtime = %s, issues=%v; want unavailable second family",
			missing.Kind(),
			issueCodes(missing),
		)
	}

	registrations := make([]memberofruntime.Registration, 0, len(fixture.rules))
	for _, rule := range fixture.rules {
		registration := mustValue(memberofruntime.NewRegistration(
			rule,
			fixture.identity,
			runtimeObservationMemberOfEngine{},
		))
		registrations = append(registrations, registration)
	}
	installed := mustValue(memberofruntime.NewRegistry(registrations))
	missingPolicy := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: fixture.basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: installed,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					fixture.catalog,
				},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					fixture.policies[0],
				},
			},
		},
	)
	if missingPolicy.Kind() != projecttypeenvruntime.ResolutionUnavailable ||
		!containsIssueCode(missingPolicy, projecttypeenvruntime.IssueRegistrationPolicyMissing) {
		t.Fatalf(
			"single-policy runtime = %s, issues=%v; want unavailable second family policy",
			missingPolicy.Kind(),
			issueCodes(missingPolicy),
		)
	}
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: fixture.basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: installed,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					fixture.catalog,
				},
				RegistrationPolicies: append(
					[]recordmembershipregistration.RegistrationArtifactV1(nil),
					fixture.policies...,
				),
			},
		},
	)
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("two-family runtime = %s, issues=%v; want matched", result.Kind(), issueCodes(result))
	}
	exact, ok := matched.Registry()
	if !ok {
		t.Fatal("matched two-family runtime exposed no exact registry")
	}
	evaluators, ok := exact.MemberOfRegistry()
	if !ok || evaluators.Len() != 2 {
		t.Fatalf("exact MemberOf registry = %d, %v; want 2, true", evaluators.Len(), ok)
	}
	policySet, ok := exact.RegistrationPolicies()
	if !ok {
		t.Fatal("matched two-family runtime exposed no exact policy posture")
	}
	policies, ok := policySet.(projecttypeenvruntime.ExactTargetRegistrationPolicyRegistry)
	if !ok || policies.Len() != 2 {
		t.Fatalf("exact policy registry = %T len=%d; want two policies", policySet, policySet.Len())
	}
	for _, rule := range fixture.rules {
		policy, found := policies.Lookup(rule)
		if !found {
			t.Fatalf("policy registry has no exact policy for %s", rule.String())
		}
		artifact, valid := policy.Artifact()
		if !valid || artifact.Evaluator().Rule() != rule {
			t.Fatalf("policy lookup for %s returned invalid artifact", rule.String())
		}
	}
	if _, ok := exact.RegistrationPolicy(); ok {
		t.Fatal("singular compatibility policy getter accepted heterogeneous registry")
	}
}

func TestObserveCurrentTargetRuntimeFailsClosedForMissingAndDriftedRuntime(t *testing.T) {
	t.Parallel()
	fixture := buildRuntimeFixture(t)
	alternateCatalog := runtimeCatalog(
		t,
		fixture.codec,
		fixture.rule,
		"artifact:test-runtime",
		"1.0.0",
		true,
	)
	alternateIdentity := mechanismIdentity(t, alternateCatalog)
	alternateEvaluators := runtimeMemberOfRegistry(
		t,
		fixture.rule,
		alternateIdentity,
	)
	alternatePolicy := registrationPolicy(
		t,
		alternateCatalog,
		fixture.rule,
	)
	emptyCodecs := typedmemory.NewCodecRegistry()
	emptyEvaluators, err := memberofruntime.NewRegistry(nil)
	if err != nil {
		t.Fatalf("NewRegistry(empty): %v", err)
	}
	testCases := []struct {
		name      string
		installed projecttypeenvruntime.InstalledRuntimeRegistryInput
		wantKind  projecttypeenvruntime.ResolutionKind
		wantCode  projecttypeenvruntime.IssueCode
	}{
		{
			name: "codec implementation missing",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               emptyCodecs,
				MemberOfEvaluators:   fixture.evaluators,
				MechanismCatalogs:    []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
			wantKind: projecttypeenvruntime.ResolutionUnavailable,
			wantCode: projecttypeenvruntime.IssueCodecImplementationMissing,
		},
		{
			name: "mechanism catalog missing",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   fixture.evaluators,
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
			wantKind: projecttypeenvruntime.ResolutionUnavailable,
			wantCode: projecttypeenvruntime.IssueMechanismCatalogMissing,
		},
		{
			name: "mechanism catalog digest drift",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   fixture.evaluators,
				MechanismCatalogs:    []runtimemechanism.RuntimeMechanismArtifactV1{alternateCatalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
			wantKind: projecttypeenvruntime.ResolutionDrifted,
			wantCode: projecttypeenvruntime.IssueMechanismCatalogDrift,
		},
		{
			name: "evaluator identity drift",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   alternateEvaluators,
				MechanismCatalogs:    []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
			wantKind: projecttypeenvruntime.ResolutionDrifted,
			wantCode: projecttypeenvruntime.IssueEvaluatorIdentityDrift,
		},
		{
			name: "evaluator registration missing",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   emptyEvaluators,
				MechanismCatalogs:    []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
			wantKind: projecttypeenvruntime.ResolutionUnavailable,
			wantCode: projecttypeenvruntime.IssueEvaluatorRegistrationMissing,
		},
		{
			name: "registration policy missing",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: fixture.evaluators,
				MechanismCatalogs:  []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
			},
			wantKind: projecttypeenvruntime.ResolutionUnavailable,
			wantCode: projecttypeenvruntime.IssueRegistrationPolicyMissing,
		},
		{
			name: "registration policy identity drift",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   fixture.evaluators,
				MechanismCatalogs:    []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{alternatePolicy},
			},
			wantKind: projecttypeenvruntime.ResolutionDrifted,
			wantCode: projecttypeenvruntime.IssueRegistrationPolicyDrift,
		},
		{
			name: "duplicate exact mechanism catalog",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: fixture.evaluators,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					fixture.catalog,
					fixture.catalog,
				},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
			wantKind: projecttypeenvruntime.ResolutionUnavailable,
			wantCode: projecttypeenvruntime.IssueMechanismCatalogDuplicate,
		},
		{
			name: "duplicate exact registration policy",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: fixture.evaluators,
				MechanismCatalogs:  []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					fixture.policy,
					fixture.policy,
				},
			},
			wantKind: projecttypeenvruntime.ResolutionUnavailable,
			wantCode: projecttypeenvruntime.IssueRegistrationPolicyDuplicate,
		},
		{
			name: "unexpected registration policy",
			installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: fixture.evaluators,
				MechanismCatalogs:  []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					fixture.policy,
					alternatePolicy,
				},
			},
			wantKind: projecttypeenvruntime.ResolutionDrifted,
			wantCode: projecttypeenvruntime.IssueUnexpectedRegistrationPolicy,
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
				projecttypeenvruntime.ObservationInput{
					RuntimeBasis: fixture.basis,
					Installed:    testCase.installed,
				},
			)
			if result.Kind() != testCase.wantKind {
				t.Fatalf(
					"result.Kind() = %s, want %s; issues = %v",
					result.Kind(),
					testCase.wantKind,
					issueCodes(result),
				)
			}
			if !containsIssueCode(result, testCase.wantCode) {
				t.Fatalf("issues = %v, want %s", issueCodes(result), testCase.wantCode)
			}
		})
	}
}

func TestObserveCurrentTargetRuntimeRejectsPolicyThatDoesNotMatchXPins(t *testing.T) {
	t.Parallel()
	fixture := buildRuntimeFixture(t)
	alternateCatalog := runtimeCatalog(
		t,
		fixture.codec,
		fixture.rule,
		"artifact:policy-coordinate-drift",
		"1.0.0",
		false,
	)
	mismatchedPolicy := registrationPolicy(
		t,
		alternateCatalog,
		fixture.rule,
	)
	basis := runtimeBasis(
		t,
		fixture.catalog,
		mismatchedPolicy,
		fixture.codec,
		fixture.rule,
	)
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             fixture.codecs,
				MemberOfEvaluators: fixture.evaluators,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					fixture.catalog,
					alternateCatalog,
				},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					mismatchedPolicy,
				},
			},
		},
	)
	if result.Kind() != projecttypeenvruntime.ResolutionDrifted {
		t.Fatalf("result.Kind() = %s, want drifted; issues = %v", result.Kind(), issueCodes(result))
	}
	for _, code := range []projecttypeenvruntime.IssueCode{
		projecttypeenvruntime.IssuePolicyEvaluatorDrift,
		projecttypeenvruntime.IssuePolicySourceDeliveryDrift,
	} {
		if !containsIssueCode(result, code) {
			t.Fatalf("issues = %v, want %s", issueCodes(result), code)
		}
	}
}

func TestObserveCurrentTargetRuntimeReportsMissingEntitySetEnumerationEvaluator(t *testing.T) {
	t.Parallel()
	rule := mustValue(typedmemory.NewRuleRef("test.entity-set-enumeration/v1"))
	entry := mustValue(runtimemechanism.NewEntitySetEnumerationEntry(rule))
	artifactRef := mustValue(typedmemory.NewCarrierRef("artifact:missing-entity-set-evaluator"))
	edition := mustValue(typedmemory.NewCarrierEdition("1.0.0"))
	catalog := mustValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	),
	)
	mechanism := mustValue(projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	pin := mustValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	),
	)
	basis := mustValue(projecttypeenv.SealRuntimeEvaluationBasis(
		[]projecttypeenv.RuntimeEvaluationMechanismPin{pin},
		catalog,
	),
	)
	emptyEvaluators := mustValue(memberofruntime.NewRegistry(nil))
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             typedmemory.NewCodecRegistry(),
				MemberOfEvaluators: emptyEvaluators,
				MechanismCatalogs:  []runtimemechanism.RuntimeMechanismArtifactV1{catalog},
			},
		},
	)
	if result.Kind() != projecttypeenvruntime.ResolutionUnavailable ||
		!containsIssueCode(result, projecttypeenvruntime.IssueEvaluatorRegistrationMissing) {
		t.Fatalf("result = %s, issues = %v; want missing evaluator unavailable", result.Kind(), issueCodes(result))
	}
}

func TestObserveCurrentTargetRuntimeMatchesEntitySetEnumerationRegistry(t *testing.T) {
	t.Parallel()
	rule := mustValue(typedmemory.NewRuleRef("test.entity-set-enumeration.exact/v1"))
	testExactTypedEvaluatorContract(
		t,
		rule,
		projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
		mustValue(runtimemechanism.NewEntitySetEnumerationEntry(rule)),
		"artifact:entity-set-enumeration-exact",
		func(identity typedmemoryevaluation.MechanismIdentity) projecttypeenvruntime.InstalledRuntimeRegistryInput {
			registry := mustValue(typedmemoryevaluation.NewEntitySetEnumerationRegistry(rule, identity))
			return projecttypeenvruntime.InstalledRuntimeRegistryInput{
				EntitySetEnumerationEvaluators: registry,
			}
		},
		func(registry projecttypeenvruntime.ExactTargetRuntimeRegistry) bool {
			installed, ok := registry.EntitySetEnumerationRegistry()
			return ok && installed.Len() == 1
		},
	)
}

func TestObserveCurrentTargetRuntimeMatchesCandidateVisibilityRegistry(t *testing.T) {
	t.Parallel()
	rule := mustValue(typedmemory.NewRuleRef("test.candidate-visibility.exact/v1"))
	testExactTypedEvaluatorContract(
		t,
		rule,
		projecttypeenv.RuntimeMechanismContractCandidateVisibility,
		mustValue(runtimemechanism.NewCandidateVisibilityEntry(rule)),
		"artifact:candidate-visibility-exact",
		func(identity typedmemoryevaluation.MechanismIdentity) projecttypeenvruntime.InstalledRuntimeRegistryInput {
			registry := mustValue(typedmemoryevaluation.NewCandidateVisibilityRegistry(rule, identity))
			return projecttypeenvruntime.InstalledRuntimeRegistryInput{
				CandidateVisibilityEvaluators: registry,
			}
		},
		func(registry projecttypeenvruntime.ExactTargetRuntimeRegistry) bool {
			installed, ok := registry.CandidateVisibilityRegistry()
			return ok && installed.Len() == 1
		},
	)
}

func TestObserveCurrentTargetRuntimeMatchesKindDefinednessRegistry(t *testing.T) {
	t.Parallel()
	rule := mustValue(typedmemory.NewRuleRef("test.kind-definedness.exact/v1"))
	testExactTypedEvaluatorContract(
		t,
		rule,
		projecttypeenv.RuntimeMechanismContractKindDefinedness,
		mustValue(runtimemechanism.NewKindDefinednessEntry(rule)),
		"artifact:kind-definedness-exact",
		func(identity typedmemoryevaluation.MechanismIdentity) projecttypeenvruntime.InstalledRuntimeRegistryInput {
			registry := mustValue(typedmemoryevaluation.NewKindDefinednessRegistry(rule, identity))
			return projecttypeenvruntime.InstalledRuntimeRegistryInput{
				KindDefinednessEvaluators: registry,
			}
		},
		func(registry projecttypeenvruntime.ExactTargetRuntimeRegistry) bool {
			installed, ok := registry.KindDefinednessRegistry()
			return ok && installed.Len() == 1
		},
	)
}

func TestObserveCurrentTargetRuntimeRequiresEveryPinnedC32Callable(t *testing.T) {
	t.Parallel()
	enumerationRule := mustValue(typedmemory.NewRuleRef("test.c32.enumeration/v1"))
	visibilityRule := mustValue(typedmemory.NewRuleRef("test.c32.visibility/v1"))
	definednessRule := mustValue(typedmemory.NewRuleRef("test.c32.definedness/v1"))
	artifact := mustValue(typedmemory.NewCarrierRef("artifact:c32-callable-chain"))
	edition := mustValue(typedmemory.NewCarrierEdition("1.0.0"))
	catalog := mustValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		[]runtimemechanism.RuntimeMechanismEntryV1{
			mustValue(runtimemechanism.NewEntitySetEnumerationEntry(enumerationRule)),
			mustValue(runtimemechanism.NewCandidateVisibilityEntry(visibilityRule)),
			mustValue(runtimemechanism.NewKindDefinednessEntry(definednessRule)),
		},
	))
	mechanism := mustValue(projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	pins := []projecttypeenv.RuntimeEvaluationMechanismPin{
		evaluatorRuntimePin(
			enumerationRule,
			projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
			mechanism,
			catalog,
		),
		evaluatorRuntimePin(
			visibilityRule,
			projecttypeenv.RuntimeMechanismContractCandidateVisibility,
			mechanism,
			catalog,
		),
		evaluatorRuntimePin(
			definednessRule,
			projecttypeenv.RuntimeMechanismContractKindDefinedness,
			mechanism,
			catalog,
		),
	}
	basis := mustValue(projecttypeenv.SealRuntimeEvaluationBasis(pins, catalog))
	identity := mechanismIdentity(t, catalog)
	installed := projecttypeenvruntime.InstalledRuntimeRegistryInput{
		EntitySetEnumerationEvaluators: mustValue(
			typedmemoryevaluation.NewEntitySetEnumerationRegistry(enumerationRule, identity),
		),
		CandidateVisibilityEvaluators: mustValue(
			typedmemoryevaluation.NewCandidateVisibilityRegistry(visibilityRule, identity),
		),
		KindDefinednessEvaluators: mustValue(
			typedmemoryevaluation.NewKindDefinednessRegistry(definednessRule, identity),
		),
		MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{catalog},
	}
	extraRule := mustValue(typedmemory.NewRuleRef("test.c32.unpinned-definedness/v1"))
	extraRegistry := mustValue(
		typedmemoryevaluation.NewKindDefinednessRegistry(extraRule, identity),
	)
	definednessRegistrations := installed.KindDefinednessEvaluators.Registrations()
	extraRegistrations := extraRegistry.Registrations()
	definednessRegistrations = append(definednessRegistrations, extraRegistrations...)
	installed.KindDefinednessEvaluators = mustValue(
		typedmemoryevaluation.NewRegistry(definednessRegistrations),
	)
	matched := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed:    installed,
		},
	)
	if matched.Kind() != projecttypeenvruntime.ResolutionMatched {
		t.Fatalf("complete C.3.2 callable set = %s, issues = %v; want matched", matched.Kind(), issueCodes(matched))
	}
	matchedResult := matched.(projecttypeenvruntime.Matched)
	exactRegistry, ok := matchedResult.Registry()
	if !ok {
		t.Fatal("matched C.3.2 callable set did not expose exact registry")
	}
	exactDefinedness, ok := exactRegistry.KindDefinednessRegistry()
	if !ok || exactDefinedness.Len() != 1 {
		t.Fatalf("exact KindDefinedness registry = %d, %v; want only one X-pinned registration", exactDefinedness.Len(), ok)
	}
	extraLookup := mustValue(exactDefinedness.Lookup(extraRule, identity))
	if _, missing := extraLookup.(typedmemoryevaluation.Missing[
		typedmemorykindruntime.KindDefinednessRequest,
		typedmemorykindruntime.KindDefinednessResult,
	]); !missing {
		t.Fatalf("unpinned extra lookup = %T, want Missing", extraLookup)
	}
	installed.KindDefinednessEvaluators = projecttypeenvruntime.KindDefinednessEvaluatorRegistry{}
	missing := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed:    installed,
		},
	)
	if missing.Kind() != projecttypeenvruntime.ResolutionUnavailable ||
		!containsIssueCode(missing, projecttypeenvruntime.IssueEvaluatorRegistrationMissing) {
		t.Fatalf("incomplete C.3.2 callable set = %s, issues = %v; want unavailable", missing.Kind(), issueCodes(missing))
	}
}

func evaluatorRuntimePin(
	rule typedmemory.RuleRef,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
) projecttypeenv.RuntimeEvaluationMechanismPin {
	return mustValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         contract,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
}

func testExactTypedEvaluatorContract(
	t *testing.T,
	rule typedmemory.RuleRef,
	contract projecttypeenv.RuntimeMechanismInvocationContract,
	entry runtimemechanism.RuntimeMechanismEntryV1,
	artifactRaw string,
	install func(typedmemoryevaluation.MechanismIdentity) projecttypeenvruntime.InstalledRuntimeRegistryInput,
	assertRegistry func(projecttypeenvruntime.ExactTargetRuntimeRegistry) bool,
) {
	t.Helper()
	artifactRef := mustValue(typedmemory.NewCarrierRef(artifactRaw))
	edition := mustValue(typedmemory.NewCarrierEdition("1.0.0"))
	catalog := mustValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	))
	mechanism := mustValue(projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	pin := mustValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         contract,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
	basis := mustValue(projecttypeenv.SealRuntimeEvaluationBasis(
		[]projecttypeenv.RuntimeEvaluationMechanismPin{pin},
		catalog,
	))
	installed := install(mechanismIdentity(t, catalog))
	installed.MechanismCatalogs = []runtimemechanism.RuntimeMechanismArtifactV1{catalog}
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed:    installed,
		},
	)
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("result = %s, issues = %v; want matched", result.Kind(), issueCodes(result))
	}
	registry, ok := matched.Registry()
	if !ok || !assertRegistry(registry) {
		t.Fatal("matched result did not retain the exact typed evaluator registry")
	}
}

func TestObserveCurrentTargetRuntimeMatchesCodecOnlyXWithoutRegistrationPolicy(
	t *testing.T,
) {
	t.Parallel()
	codec := codecRef(t)
	entry := mustValue(runtimemechanism.NewCodecCanonicalizationEntry(codec))
	artifact := mustValue(typedmemory.NewCarrierRef("artifact:codec-only-runtime"))
	edition := mustValue(typedmemory.NewCarrierEdition("1.0.0"))
	catalog := mustValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	))
	mechanism := mustValue(projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	pin := mustValue(projecttypeenv.NewCodecRuntimeMechanismPin(
		projecttypeenv.CodecRuntimeMechanismPinInput{
			Codec:            codec,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
	basis := mustValue(projecttypeenv.SealRuntimeEvaluationBasis(
		[]projecttypeenv.RuntimeEvaluationMechanismPin{pin},
		catalog,
	))
	codecs := mustValue(typedmemory.NewCodecRegistry().Register(codec, inertCodec{}))
	evaluators := mustValue(memberofruntime.NewRegistry(nil))
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             codecs,
				MemberOfEvaluators: evaluators,
				MechanismCatalogs:  []runtimemechanism.RuntimeMechanismArtifactV1{catalog},
			},
		},
	)
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("result = %s, issues = %v; want matched", result.Kind(), issueCodes(result))
	}
	registry, ok := matched.Registry()
	if !ok {
		t.Fatal("matched codec-only X did not expose its exact registry")
	}
	policy, ok := registry.RegistrationPolicy()
	if !ok {
		t.Fatal("matched codec-only X did not expose policy absence")
	}
	if _, ok := policy.(projecttypeenvruntime.NoTargetRegistrationPolicy); !ok {
		t.Fatalf("RegistrationPolicy() = %T, want NoTargetRegistrationPolicy", policy)
	}
	policySet, ok := registry.RegistrationPolicies()
	if !ok {
		t.Fatal("matched codec-only X did not expose canonical policy absence")
	}
	if _, ok := policySet.(projecttypeenvruntime.NoTargetRegistrationPolicies); !ok {
		t.Fatalf("RegistrationPolicies() = %T, want NoTargetRegistrationPolicies", policySet)
	}
}

func TestObserveCurrentTargetRuntimeInvalidAndMutationIsolation(t *testing.T) {
	t.Parallel()
	invalid := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{},
	)
	if invalid.Kind() != projecttypeenvruntime.ResolutionInvalid ||
		!containsIssueCode(invalid, projecttypeenvruntime.IssueRuntimeBasisInvalid) {
		t.Fatalf("zero input = %s, issues = %v; want invalid X", invalid.Kind(), issueCodes(invalid))
	}

	fixture := buildRuntimeFixture(t)
	catalogs := []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog}
	policies := []recordmembershipregistration.RegistrationArtifactV1{fixture.policy}
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: fixture.basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   fixture.evaluators,
				MechanismCatalogs:    catalogs,
				RegistrationPolicies: policies,
			},
		},
	)
	catalogs[0] = runtimemechanism.RuntimeMechanismArtifactV1{}
	policies[0] = recordmembershipregistration.RegistrationArtifactV1{}
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("result = %s, issues = %v; want matched", result.Kind(), issueCodes(result))
	}
	registry, ok := matched.Registry()
	if !ok || !registry.Valid() {
		t.Fatal("mutating caller slices changed the matched runtime observation")
	}
	firstDigest, _ := registry.CoordinateDigest()
	secondDigest, _ := registry.CoordinateDigest()
	if firstDigest != secondDigest {
		t.Fatal("coordinate digest changed across immutable reads")
	}
}

func observeFixture(fixture runtimeFixture) projecttypeenvruntime.Resolution {
	return projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: fixture.basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:               fixture.codecs,
				MemberOfEvaluators:   fixture.evaluators,
				MechanismCatalogs:    []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
			},
		},
	)
}

func buildRuntimeFixture(t *testing.T) runtimeFixture {
	t.Helper()
	codec := codecRef(t)
	rule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	catalog := runtimeCatalog(
		t,
		codec,
		rule,
		"artifact:test-runtime",
		"1.0.0",
		false,
	)
	policy := registrationPolicy(t, catalog, rule)
	basis := runtimeBasis(t, catalog, policy, codec, rule)
	codecs := mustValue(typedmemory.NewCodecRegistry().Register(codec, inertCodec{}))
	evaluators := runtimeMemberOfRegistry(
		t,
		rule,
		mechanismIdentity(t, catalog),
	)
	return runtimeFixture{
		basis:      basis,
		catalog:    catalog,
		policy:     policy,
		codec:      codec,
		rule:       rule,
		codecs:     codecs,
		evaluators: evaluators,
	}
}

func buildMultipleMemberOfRuntimeFixture(
	t *testing.T,
) multipleMemberOfRuntimeFixture {
	t.Helper()
	codec := codecRef(t)
	rules := []typedmemory.RuleRef{
		recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
		mustValue(typedmemory.NewRuleRef("test.memberof.other-family/v1")),
	}
	entries := []runtimemechanism.RuntimeMechanismEntryV1{
		mustValue(runtimemechanism.NewCodecCanonicalizationEntry(codec)),
		mustValue(runtimemechanism.NewMemberOfEntry(rules[0])),
		mustValue(runtimemechanism.NewCarrierMembershipDeliveryEntry(rules[0])),
		mustValue(runtimemechanism.NewMemberOfEntry(rules[1])),
		mustValue(runtimemechanism.NewCarrierMembershipDeliveryEntry(rules[1])),
	}
	artifact := mustValue(typedmemory.NewCarrierRef("artifact:test-multiple-memberof-runtime"))
	edition := mustValue(typedmemory.NewCarrierEdition("1.0.0"))
	catalog := mustValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		entries,
	))
	policies := []recordmembershipregistration.RegistrationArtifactV1{
		registrationPolicy(t, catalog, rules[0]),
		registrationPolicy(t, catalog, rules[1]),
	}
	mechanism := mustValue(projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	pins := make([]projecttypeenv.RuntimeEvaluationBasisPin, 0, 7)
	pins = append(pins, mustValue(projecttypeenv.NewCodecRuntimeMechanismPin(
		projecttypeenv.CodecRuntimeMechanismPinInput{
			Codec:            codec,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	)))
	for _, rule := range rules {
		pins = append(pins, mustValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
			projecttypeenv.EvaluatorRuntimeMechanismPinInput{
				Rule:             rule,
				Contract:         projecttypeenv.RuntimeMechanismContractMemberOf,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		)))
		pins = append(pins, mustValue(projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		)))
	}
	for _, policy := range policies {
		pins = append(pins, mustValue(projecttypeenv.NewRegistrationPolicyPin(policy)))
	}
	basis := mustValue(projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		pins,
		[]runtimemechanism.RuntimeMechanismArtifactV1{catalog},
		nil,
	))
	codecs := mustValue(typedmemory.NewCodecRegistry().Register(codec, inertCodec{}))
	return multipleMemberOfRuntimeFixture{
		basis:    basis,
		catalog:  catalog,
		policies: append([]recordmembershipregistration.RegistrationArtifactV1(nil), policies...),
		codecs:   codecs,
		rules:    append([]typedmemory.RuleRef(nil), rules...),
		identity: mechanismIdentity(t, catalog),
	}
}

func runtimeCatalog(
	t *testing.T,
	codec typedmemory.CodecRef,
	rule typedmemory.RuleRef,
	artifactRaw string,
	editionRaw string,
	extra bool,
) runtimemechanism.RuntimeMechanismArtifactV1 {
	t.Helper()
	entries := []runtimemechanism.RuntimeMechanismEntryV1{
		mustValue(runtimemechanism.NewCodecCanonicalizationEntry(codec)),
		mustValue(runtimemechanism.NewMemberOfEntry(rule)),
		mustValue(runtimemechanism.NewCarrierMembershipDeliveryEntry(rule)),
	}
	if extra {
		extraRule := mustValue(typedmemory.NewRuleRef("test.extra-evaluator/v1"))
		entries = append(entries, mustValue(runtimemechanism.NewKindDefinednessEntry(extraRule)))
	}
	artifact := mustValue(typedmemory.NewCarrierRef(artifactRaw))
	edition := mustValue(typedmemory.NewCarrierEdition(editionRaw))
	return mustValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		entries,
	),
	)
}

func registrationPolicy(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	rule typedmemory.RuleRef,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	identity := catalog.Identity()
	evaluator := mustValue(recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	),
	)
	delivery := mustValue(recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	),
	)
	manifest := mustValue(recordmapping.NewMappingManifestRef(
		"test.record-mapping",
		"1.0.0",
		digest(t, 0x31),
	),
	)
	adapter := mustValue(recordmapping.NewAdapterVersion("test-adapter/1.0.0"))
	mapping := mustValue(recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		},
	),
	)
	return mustValue(recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
		},
	),
	)
}

func runtimeBasis(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	policy recordmembershipregistration.RegistrationArtifactV1,
	codec typedmemory.CodecRef,
	rule typedmemory.RuleRef,
) projecttypeenv.RuntimeEvaluationBasisArtifact {
	t.Helper()
	mechanism := mustValue(projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	codecPin := mustValue(projecttypeenv.NewCodecRuntimeMechanismPin(
		projecttypeenv.CodecRuntimeMechanismPinInput{
			Codec:            codec,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	),
	)
	evaluatorPin := mustValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         projecttypeenv.RuntimeMechanismContractMemberOf,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	),
	)
	deliveryPin := mustValue(projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
		projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
			Rule:             rule,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	),
	)
	policyPin := mustValue(projecttypeenv.NewRegistrationPolicyPin(policy))
	pins := []projecttypeenv.RuntimeEvaluationBasisPin{
		codecPin,
		evaluatorPin,
		deliveryPin,
		policyPin,
	}
	return mustValue(projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		pins,
		[]runtimemechanism.RuntimeMechanismArtifactV1{catalog},
		nil,
	),
	)
}

func codecRef(t *testing.T) typedmemory.CodecRef {
	t.Helper()
	id := mustValue(typedmemory.NewCodecID("Haft.Codec.TestV1"))
	version := mustValue(typedmemory.NewCanonicalizationVersion("v1"))
	return mustValue(typedmemory.NewCodecRef(id, version, digest(t, 0x21)))
}

func mechanismIdentity(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	identity := catalog.Identity()
	return mustValue(typedmemoryevaluation.NewMechanismIdentity(
		identity.Artifact(),
		identity.Edition(),
		identity.Digest(),
		typedmemoryevaluation.EvaluatorRole,
	),
	)
}

func digest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := make([]byte, len("sha256:")+64)
	copy(raw, "sha256:")
	const alphabet = "0123456789abcdef"
	for index := len("sha256:"); index < len(raw); index++ {
		raw[index] = alphabet[int(fill+byte(index))%len(alphabet)]
	}
	return mustValue(typedmemory.NewSHA256Digest(string(raw)))
}

func issueCodes(result projecttypeenvruntime.Resolution) []projecttypeenvruntime.IssueCode {
	issues := issuesFromResult(result)
	codes := make([]projecttypeenvruntime.IssueCode, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code())
	}
	return codes
}

func containsIssueCode(
	result projecttypeenvruntime.Resolution,
	code projecttypeenvruntime.IssueCode,
) bool {
	for _, issue := range issuesFromResult(result) {
		if issue.Code() == code {
			return true
		}
	}
	return false
}

func issuesFromResult(result projecttypeenvruntime.Resolution) []projecttypeenvruntime.Issue {
	switch value := result.(type) {
	case projecttypeenvruntime.Invalid:
		return value.Issues()
	case projecttypeenvruntime.Unavailable:
		return value.Issues()
	case projecttypeenvruntime.Drifted:
		return value.Issues()
	default:
		return nil
	}
}

func mustValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
