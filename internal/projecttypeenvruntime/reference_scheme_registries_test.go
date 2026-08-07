package projecttypeenvruntime_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

type referenceSchemeRuntimeRules struct {
	designation    typedmemory.RuleRef
	interpretation typedmemory.RuleRef
	measurement    typedmemory.RuleRef
	evaluation     typedmemory.RuleRef
	constitution   typedmemory.RuleRef
}

type referenceSchemeRuntimeFixture struct {
	basis     projecttypeenv.RuntimeEvaluationBasisArtifact
	identity  typedmemoryevaluation.MechanismIdentity
	rules     referenceSchemeRuntimeRules
	installed projecttypeenvruntime.InstalledRuntimeRegistryInput
}

func TestObserveCurrentTargetRuntimeMatchesAllReferenceSchemeCallables(
	t *testing.T,
) {
	t.Parallel()
	fixture := buildReferenceSchemeRuntimeFixture(t)
	observation := projecttypeenvruntime.ObservationInput{
		RuntimeBasis: fixture.basis,
		Installed:    fixture.installed,
	}
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(observation)
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("result = %s, issues = %v; want matched", result.Kind(), issueCodes(result))
	}
	registry, ok := matched.Registry()
	if !ok || !registry.Valid() {
		t.Fatal("matched result did not expose a valid exact target runtime registry")
	}
	assertReferenceSchemeRegistriesAreExactAndConservative(
		t,
		registry,
		fixture,
	)
	if _, err := json.Marshal(registry); !errors.Is(
		err,
		projecttypeenvruntime.ErrExactTargetRuntimeRegistryNotSerializable,
	) {
		t.Fatalf("json.Marshal() error = %v, want non-serializable sentinel", err)
	}
	var decoded projecttypeenvruntime.ExactTargetRuntimeRegistry
	if err := json.Unmarshal([]byte(`{}`), &decoded); !errors.Is(
		err,
		projecttypeenvruntime.ErrExactTargetRuntimeRegistryNotSerializable,
	) {
		t.Fatalf("json.Unmarshal() error = %v, want non-serializable sentinel", err)
	}
}

func TestObserveCurrentTargetRuntimeFailsClosedForReferenceSchemeRegistration(
	t *testing.T,
) {
	t.Parallel()
	fixture := buildReferenceSchemeRuntimeFixture(t)
	wrongArtifact, err := typedmemory.NewCarrierRef("artifact:wrong-reference-scheme-runtime")
	if err != nil {
		t.Fatal(err)
	}
	wrongEdition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	wrongDigest := digest(t, 0xd1)
	wrongIdentity, err := typedmemoryevaluation.NewMechanismIdentity(
		wrongArtifact,
		wrongEdition,
		wrongDigest,
		typedmemoryevaluation.EvaluatorRole,
	)
	if err != nil {
		t.Fatal(err)
	}
	contracts := []struct {
		name    string
		remove  func(*projecttypeenvruntime.InstalledRuntimeRegistryInput)
		replace func(*projecttypeenvruntime.InstalledRuntimeRegistryInput) error
	}{
		{
			name: "reference designation resolution",
			remove: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) {
				input.ReferenceDesignationResolutionEvaluators =
					projecttypeenvruntime.ReferenceDesignationResolutionEvaluatorRegistry{}
			},
			replace: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) error {
				registry, registryErr := typedmemoryevaluation.NewReferenceDesignationResolutionRegistry(
					fixture.rules.designation,
					wrongIdentity,
				)
				if registryErr != nil {
					return registryErr
				}
				input.ReferenceDesignationResolutionEvaluators = registry
				return nil
			},
		},
		{
			name: "claim interpretation",
			remove: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) {
				input.ClaimInterpretationEvaluators =
					projecttypeenvruntime.ClaimInterpretationEvaluatorRegistry{}
			},
			replace: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) error {
				registry, registryErr := typedmemoryevaluation.NewClaimInterpretationRegistry(
					fixture.rules.interpretation,
					wrongIdentity,
				)
				if registryErr != nil {
					return registryErr
				}
				input.ClaimInterpretationEvaluators = registry
				return nil
			},
		},
		{
			name: "claim measurement",
			remove: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) {
				input.ClaimMeasurementEvaluators =
					projecttypeenvruntime.ClaimMeasurementEvaluatorRegistry{}
			},
			replace: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) error {
				registry, registryErr := typedmemoryevaluation.NewClaimMeasurementRegistry(
					fixture.rules.measurement,
					wrongIdentity,
				)
				if registryErr != nil {
					return registryErr
				}
				input.ClaimMeasurementEvaluators = registry
				return nil
			},
		},
		{
			name: "claim evaluation",
			remove: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) {
				input.ClaimEvaluationEvaluators =
					projecttypeenvruntime.ClaimEvaluationEvaluatorRegistry{}
			},
			replace: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) error {
				registry, registryErr := typedmemoryevaluation.NewClaimEvaluationRegistry(
					fixture.rules.evaluation,
					wrongIdentity,
				)
				if registryErr != nil {
					return registryErr
				}
				input.ClaimEvaluationEvaluators = registry
				return nil
			},
		},
		{
			name: "episteme constitution evaluation",
			remove: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) {
				input.EpistemeConstitutionEvaluators =
					projecttypeenvruntime.EpistemeConstitutionEvaluatorRegistry{}
			},
			replace: func(input *projecttypeenvruntime.InstalledRuntimeRegistryInput) error {
				registry, registryErr := typedmemoryevaluation.NewEpistemeConstitutionEvaluationRegistry(
					fixture.rules.constitution,
					wrongIdentity,
				)
				if registryErr != nil {
					return registryErr
				}
				input.EpistemeConstitutionEvaluators = registry
				return nil
			},
		},
	}
	for _, contract := range contracts {
		contract := contract
		t.Run(contract.name+" missing", func(t *testing.T) {
			t.Parallel()
			installed := fixture.installed
			contract.remove(&installed)
			assertReferenceSchemeRuntimeFailure(
				t,
				fixture.basis,
				installed,
				projecttypeenvruntime.ResolutionUnavailable,
				projecttypeenvruntime.IssueEvaluatorRegistrationMissing,
			)
		})
		t.Run(contract.name+" mismatched", func(t *testing.T) {
			t.Parallel()
			installed := fixture.installed
			if err := contract.replace(&installed); err != nil {
				t.Fatal(err)
			}
			assertReferenceSchemeRuntimeFailure(
				t,
				fixture.basis,
				installed,
				projecttypeenvruntime.ResolutionDrifted,
				projecttypeenvruntime.IssueEvaluatorIdentityDrift,
			)
		})
	}
}

func TestReferenceSchemeRegistriesRejectDuplicateRegistration(t *testing.T) {
	t.Parallel()
	fixture := buildReferenceSchemeRuntimeFixture(t)
	assertDuplicateRegistryRegistrationRejected(
		t,
		fixture.installed.ReferenceDesignationResolutionEvaluators,
	)
	assertDuplicateRegistryRegistrationRejected(
		t,
		fixture.installed.ClaimInterpretationEvaluators,
	)
	assertDuplicateRegistryRegistrationRejected(
		t,
		fixture.installed.ClaimMeasurementEvaluators,
	)
	assertDuplicateRegistryRegistrationRejected(
		t,
		fixture.installed.ClaimEvaluationEvaluators,
	)
	assertDuplicateRegistryRegistrationRejected(
		t,
		fixture.installed.EpistemeConstitutionEvaluators,
	)
}

func TestReferenceSchemeRegistriesDoNotChangeAnOlderExactX(t *testing.T) {
	t.Parallel()
	fixture := buildRuntimeFixture(t)
	baseline := observeFixture(fixture)
	baselineRegistry := exactRegistryFromResult(t, baseline)
	baselineDigest, ok := baselineRegistry.CoordinateDigest()
	if !ok {
		t.Fatal("baseline old-X registry has no coordinate digest")
	}
	identity := mechanismIdentity(t, fixture.catalog)
	designation := referenceDesignationRegistry(t, fixture.rule, identity)
	interpretation := claimInterpretationRegistry(t, fixture.rule, identity)
	measurement := claimMeasurementRegistry(t, fixture.rule, identity)
	evaluation := claimEvaluationRegistry(t, fixture.rule, identity)
	constitution := epistemeConstitutionRegistry(t, fixture.rule, identity)
	installed := projecttypeenvruntime.InstalledRuntimeRegistryInput{
		Codecs:                                   fixture.codecs,
		MemberOfEvaluators:                       fixture.evaluators,
		ReferenceDesignationResolutionEvaluators: designation,
		ClaimInterpretationEvaluators:            interpretation,
		ClaimMeasurementEvaluators:               measurement,
		ClaimEvaluationEvaluators:                evaluation,
		EpistemeConstitutionEvaluators:           constitution,
		MechanismCatalogs:                        []runtimemechanism.RuntimeMechanismArtifactV1{fixture.catalog},
		RegistrationPolicies:                     []recordmembershipregistration.RegistrationArtifactV1{fixture.policy},
	}
	observation := projecttypeenvruntime.ObservationInput{
		RuntimeBasis: fixture.basis,
		Installed:    installed,
	}
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(observation)
	registry := exactRegistryFromResult(t, result)
	coordinateDigest, ok := registry.CoordinateDigest()
	if !ok || coordinateDigest != baselineDigest {
		t.Fatalf(
			"old-X coordinate digest = %s, %v; want unchanged %s",
			coordinateDigest.String(),
			ok,
			baselineDigest.String(),
		)
	}
	assertReferenceSchemeRegistriesEmpty(t, registry)
}

func buildReferenceSchemeRuntimeFixture(
	t *testing.T,
) referenceSchemeRuntimeFixture {
	t.Helper()
	designationRule := referenceSchemeRule(
		t,
		"haft.reference-scheme.project-memory.test/designation-resolution-v1",
	)
	interpretationRule := referenceSchemeRule(
		t,
		"haft.reference-scheme.project-memory.test/claim-interpretation-v1",
	)
	measurementRule := referenceSchemeRule(
		t,
		"haft.reference-scheme.project-memory.test/claim-measurement-v1",
	)
	evaluationRule := referenceSchemeRule(
		t,
		"haft.reference-scheme.project-memory.test/claim-evaluation-v1",
	)
	constitutionRule := referenceSchemeRule(
		t,
		"haft.episteme-constitution.project-memory.test/evaluate-v1",
	)
	rules := referenceSchemeRuntimeRules{
		designation:    designationRule,
		interpretation: interpretationRule,
		measurement:    measurementRule,
		evaluation:     evaluationRule,
		constitution:   constitutionRule,
	}
	designationEntry := referenceDesignationEntry(t, rules.designation)
	interpretationEntry := claimInterpretationEntry(t, rules.interpretation)
	measurementEntry := claimMeasurementEntry(t, rules.measurement)
	evaluationEntry := claimEvaluationEntry(t, rules.evaluation)
	constitutionEntry := epistemeConstitutionEntry(t, rules.constitution)
	entries := []runtimemechanism.RuntimeMechanismEntryV1{
		designationEntry,
		interpretationEntry,
		measurementEntry,
		evaluationEntry,
		constitutionEntry,
	}
	artifact, err := typedmemory.NewCarrierRef("artifact:reference-scheme-runtime-test")
	if err != nil {
		t.Fatal(err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		entries,
	)
	if err != nil {
		t.Fatal(err)
	}
	mechanism, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog)
	if err != nil {
		t.Fatal(err)
	}
	designationPin := evaluatorRuntimePin(
		rules.designation,
		projecttypeenv.RuntimeMechanismContractReferenceDesignationResolution,
		mechanism,
		catalog,
	)
	interpretationPin := evaluatorRuntimePin(
		rules.interpretation,
		projecttypeenv.RuntimeMechanismContractClaimInterpretation,
		mechanism,
		catalog,
	)
	measurementPin := evaluatorRuntimePin(
		rules.measurement,
		projecttypeenv.RuntimeMechanismContractClaimMeasurement,
		mechanism,
		catalog,
	)
	evaluationPin := evaluatorRuntimePin(
		rules.evaluation,
		projecttypeenv.RuntimeMechanismContractClaimEvaluation,
		mechanism,
		catalog,
	)
	constitutionPin := evaluatorRuntimePin(
		rules.constitution,
		projecttypeenv.RuntimeMechanismContractEpistemeConstitutionEvaluation,
		mechanism,
		catalog,
	)
	pins := []projecttypeenv.RuntimeEvaluationMechanismPin{
		designationPin,
		interpretationPin,
		measurementPin,
		evaluationPin,
		constitutionPin,
	}
	basis, err := projecttypeenv.SealRuntimeEvaluationBasis(pins, catalog)
	if err != nil {
		t.Fatal(err)
	}
	identity := mechanismIdentity(t, catalog)
	designation := referenceDesignationRegistry(t, rules.designation, identity)
	interpretation := claimInterpretationRegistry(t, rules.interpretation, identity)
	measurement := claimMeasurementRegistry(t, rules.measurement, identity)
	evaluation := claimEvaluationRegistry(t, rules.evaluation, identity)
	constitution := epistemeConstitutionRegistry(t, rules.constitution, identity)
	installed := projecttypeenvruntime.InstalledRuntimeRegistryInput{
		ReferenceDesignationResolutionEvaluators: designation,
		ClaimInterpretationEvaluators:            interpretation,
		ClaimMeasurementEvaluators:               measurement,
		ClaimEvaluationEvaluators:                evaluation,
		EpistemeConstitutionEvaluators:           constitution,
		MechanismCatalogs:                        []runtimemechanism.RuntimeMechanismArtifactV1{catalog},
	}
	return referenceSchemeRuntimeFixture{
		basis:     basis,
		identity:  identity,
		rules:     rules,
		installed: installed,
	}
}

func referenceSchemeRule(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func referenceDesignationEntry(
	t *testing.T,
	rule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismEntryV1 {
	t.Helper()
	entry, err := runtimemechanism.NewReferenceDesignationResolutionEntry(rule)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func claimInterpretationEntry(
	t *testing.T,
	rule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismEntryV1 {
	t.Helper()
	entry, err := runtimemechanism.NewClaimInterpretationEntry(rule)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func claimMeasurementEntry(
	t *testing.T,
	rule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismEntryV1 {
	t.Helper()
	entry, err := runtimemechanism.NewClaimMeasurementEntry(rule)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func claimEvaluationEntry(
	t *testing.T,
	rule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismEntryV1 {
	t.Helper()
	entry, err := runtimemechanism.NewClaimEvaluationEntry(rule)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func epistemeConstitutionEntry(
	t *testing.T,
	rule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismEntryV1 {
	t.Helper()
	entry, err := runtimemechanism.NewEpistemeConstitutionEvaluationEntry(rule)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func referenceDesignationRegistry(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) projecttypeenvruntime.ReferenceDesignationResolutionEvaluatorRegistry {
	t.Helper()
	registry, err := typedmemoryevaluation.NewReferenceDesignationResolutionRegistry(
		rule,
		identity,
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func claimInterpretationRegistry(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) projecttypeenvruntime.ClaimInterpretationEvaluatorRegistry {
	t.Helper()
	registry, err := typedmemoryevaluation.NewClaimInterpretationRegistry(
		rule,
		identity,
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func claimMeasurementRegistry(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) projecttypeenvruntime.ClaimMeasurementEvaluatorRegistry {
	t.Helper()
	registry, err := typedmemoryevaluation.NewClaimMeasurementRegistry(
		rule,
		identity,
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func claimEvaluationRegistry(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) projecttypeenvruntime.ClaimEvaluationEvaluatorRegistry {
	t.Helper()
	registry, err := typedmemoryevaluation.NewClaimEvaluationRegistry(
		rule,
		identity,
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func epistemeConstitutionRegistry(
	t *testing.T,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) projecttypeenvruntime.EpistemeConstitutionEvaluatorRegistry {
	t.Helper()
	registry, err := typedmemoryevaluation.NewEpistemeConstitutionEvaluationRegistry(
		rule,
		identity,
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func assertReferenceSchemeRegistriesAreExactAndConservative(
	t *testing.T,
	registry projecttypeenvruntime.ExactTargetRuntimeRegistry,
	fixture referenceSchemeRuntimeFixture,
) {
	t.Helper()
	designation, ok := registry.ReferenceDesignationResolutionRegistry()
	if !ok || designation.Len() != 1 {
		t.Fatalf("designation registry = %d, %v; want 1, true", designation.Len(), ok)
	}
	designationRequest := typedmemoryevaluation.NewReferenceDesignationResolutionRequest()
	assertConservativeEvaluator(
		t,
		designation,
		fixture.rules.designation,
		fixture.identity,
		designationRequest,
	)
	interpretation, ok := registry.ClaimInterpretationRegistry()
	if !ok || interpretation.Len() != 1 {
		t.Fatalf("interpretation registry = %d, %v; want 1, true", interpretation.Len(), ok)
	}
	interpretationRequest := typedmemoryevaluation.NewClaimInterpretationRequest()
	assertConservativeEvaluator(
		t,
		interpretation,
		fixture.rules.interpretation,
		fixture.identity,
		interpretationRequest,
	)
	measurement, ok := registry.ClaimMeasurementRegistry()
	if !ok || measurement.Len() != 1 {
		t.Fatalf("measurement registry = %d, %v; want 1, true", measurement.Len(), ok)
	}
	measurementRequest := typedmemoryevaluation.NewClaimMeasurementRequest()
	assertConservativeEvaluator(
		t,
		measurement,
		fixture.rules.measurement,
		fixture.identity,
		measurementRequest,
	)
	evaluation, ok := registry.ClaimEvaluationRegistry()
	if !ok || evaluation.Len() != 1 {
		t.Fatalf("evaluation registry = %d, %v; want 1, true", evaluation.Len(), ok)
	}
	evaluationRequest := typedmemoryevaluation.NewClaimEvaluationRequest()
	assertConservativeEvaluator(
		t,
		evaluation,
		fixture.rules.evaluation,
		fixture.identity,
		evaluationRequest,
	)
	constitution, ok := registry.EpistemeConstitutionEvaluationRegistry()
	if !ok || constitution.Len() != 1 {
		t.Fatalf("constitution registry = %d, %v; want 1, true", constitution.Len(), ok)
	}
	constitutionLookup, err := constitution.Lookup(
		fixture.rules.constitution,
		fixture.identity,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := constitutionLookup.(typedmemoryevaluation.Found[
		typedmemoryevaluation.EpistemeConstitutionEvaluationRequest,
		typedmemoryevaluation.EpistemeConstitutionEvaluationResult,
	]); !ok {
		t.Fatalf("constitution Lookup() = %T, want exact Found", constitutionLookup)
	}
}

func assertConservativeEvaluator[
	Input any,
	Output interface {
		Kind() typedmemoryevaluation.ConservativeEvaluationResultKind
		Reason() typedmemoryevaluation.ConservativeEvaluationReason
	},
](
	t *testing.T,
	registry typedmemoryevaluation.Registry[Input, Output],
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
	request Input,
) {
	t.Helper()
	lookup, err := registry.Lookup(rule, identity)
	if err != nil {
		t.Fatal(err)
	}
	found, ok := lookup.(typedmemoryevaluation.Found[Input, Output])
	if !ok {
		t.Fatalf("Lookup() = %T, want exact Found", lookup)
	}
	registration := found.Registration()
	evaluator := registration.Evaluator()
	result, err := evaluator.Evaluate(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind() != typedmemoryevaluation.ConservativeEvaluationUnderdetermined ||
		result.Reason() != typedmemoryevaluation.ConservativeEvaluationSemanticContractUnavailable {
		t.Fatalf(
			"evaluation = %s/%s, want underdetermined/semantic_contract_unavailable",
			result.Kind(),
			result.Reason(),
		)
	}
}

func assertDuplicateRegistryRegistrationRejected[Input, Output any](
	t *testing.T,
	registry typedmemoryevaluation.Registry[Input, Output],
) {
	t.Helper()
	registrations := registry.Registrations()
	if len(registrations) != 1 {
		t.Fatalf("fixture registry registrations = %d, want 1", len(registrations))
	}
	duplicate := append(registrations, registrations[0])
	_, err := typedmemoryevaluation.NewRegistry(duplicate)
	var conflict typedmemoryevaluation.ConstructionConflict
	if !errors.As(err, &conflict) ||
		conflict.Kind() != typedmemoryevaluation.DuplicateRuleRefRegistration {
		t.Fatalf("duplicate registry error = %v, want duplicate RuleRef conflict", err)
	}
}

func assertReferenceSchemeRuntimeFailure(
	t *testing.T,
	basis projecttypeenv.RuntimeEvaluationBasisArtifact,
	installed projecttypeenvruntime.InstalledRuntimeRegistryInput,
	wantKind projecttypeenvruntime.ResolutionKind,
	wantCode projecttypeenvruntime.IssueCode,
) {
	t.Helper()
	observation := projecttypeenvruntime.ObservationInput{
		RuntimeBasis: basis,
		Installed:    installed,
	}
	result := projecttypeenvruntime.ObserveCurrentTargetRuntime(observation)
	if result.Kind() != wantKind || !containsIssueCode(result, wantCode) {
		t.Fatalf(
			"result = %s, issues = %v; want %s with %s",
			result.Kind(),
			issueCodes(result),
			wantKind,
			wantCode,
		)
	}
}

func exactRegistryFromResult(
	t *testing.T,
	result projecttypeenvruntime.Resolution,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	matched, ok := result.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("result = %s, issues = %v; want matched", result.Kind(), issueCodes(result))
	}
	registry, ok := matched.Registry()
	if !ok {
		t.Fatal("matched result did not expose its exact registry")
	}
	return registry
}

func assertReferenceSchemeRegistriesEmpty(
	t *testing.T,
	registry projecttypeenvruntime.ExactTargetRuntimeRegistry,
) {
	t.Helper()
	designation, designationOK := registry.ReferenceDesignationResolutionRegistry()
	interpretation, interpretationOK := registry.ClaimInterpretationRegistry()
	measurement, measurementOK := registry.ClaimMeasurementRegistry()
	evaluation, evaluationOK := registry.ClaimEvaluationRegistry()
	constitution, constitutionOK := registry.EpistemeConstitutionEvaluationRegistry()
	if !designationOK || designation.Len() != 0 ||
		!interpretationOK || interpretation.Len() != 0 ||
		!measurementOK || measurement.Len() != 0 ||
		!evaluationOK || evaluation.Len() != 0 ||
		!constitutionOK || constitution.Len() != 0 {
		t.Fatal("old X retained an unpinned ReferenceScheme callable")
	}
}
