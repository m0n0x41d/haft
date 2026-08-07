package projectmemory

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestSelectedRecordMembershipRuntimeReturnsExplicitNotRequiredForEmptyX(
	t *testing.T,
) {
	t.Parallel()

	runtimeBasis, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis: %v", err)
	}
	selected, err := selectedMemberOfRuntime(
		runtimeBasis,
		projecttypeenvruntime.ExactTargetRuntimeRegistry{},
	)
	if err != nil {
		t.Fatalf("selectedMemberOfRuntime: %v", err)
	}
	if _, ok := selected.(typedmemorystore.MemberOfNotRequired); !ok {
		t.Fatalf(
			"selected membership runtime = %T, want MemberOfNotRequired",
			selected,
		)
	}
}

func TestSelectedMemberOfRuntimePreservesTwoRealFamilies(t *testing.T) {
	t.Parallel()

	base := newRecordMembershipAdmissionEngineFixture(t)
	recordRule := recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef()
	projectEntityRule := mustRecordMembershipEngineValue(
		typedmemory.NewRuleRef(projectEntityMemberOfRule))
	catalog := selectedTwoFamilyMemberOfCatalog(
		t,
		recordRule,
		projectEntityRule,
		base.enumeration,
		base.visibility,
		base.definedness,
	)
	recordPolicy := recordMembershipEnginePolicy(
		t,
		catalog,
		recordRule,
		base.manifest,
		base.adapter,
	)
	entityManifest := mustRecordMembershipEngineValue(
		CurrentProjectEntityUniverseMappingManifestV1())
	entityPolicy := recordMembershipEnginePolicy(
		t,
		catalog,
		projectEntityRule,
		entityManifest.Ref(),
		entityManifest.AdapterVersion(),
	)
	basis := selectedTwoFamilyRuntimeBasis(
		t,
		catalog,
		[]recordmembershipregistration.RegistrationArtifactV1{
			recordPolicy,
			entityPolicy,
		},
		recordRule,
		projectEntityRule,
		base.enumeration,
		base.visibility,
		base.definedness,
	)
	identity := recordMembershipEngineMechanismIdentity(t, catalog)
	recordCarrier := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewRecordMembershipRegistry(identity))
	enumeration := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewEntitySetEnumerationRegistry(
			base.enumeration,
			identity,
		))
	visibility := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewCandidateVisibilityRegistry(
			base.visibility,
			identity,
		))
	definedness := mustRecordMembershipEngineValue(
		typedmemoryevaluation.NewKindDefinednessRegistry(
			base.definedness,
			identity,
		))
	recordEngine := mustRecordMembershipEngineValue(
		NewRecordMembershipAdmissionEngineBuilder().
			SetEntitySetEnumeration(enumeration).
			SetCandidateVisibility(visibility).
			SetKindDefinedness(definedness).
			SetRecordCarrierMembership(recordCarrier).
			SetRegistrationPolicy(recordPolicy).
			Build())
	projectEntityEngine := mustRecordMembershipEngineValue(
		NewProjectEntityMembershipAdmissionEngineBuilder().
			SetEntitySetEnumeration(enumeration).
			SetCandidateVisibility(visibility).
			SetKindDefinedness(definedness).
			SetMechanismIdentity(identity).
			Build())
	recordRegistry := mustRecordMembershipEngineValue(
		NewRecordMembershipEvaluatorRegistry(recordEngine))
	projectEntityRegistry := mustRecordMembershipEngineValue(
		NewProjectEntityMembershipEvaluatorRegistry(projectEntityEngine))
	registrations := append(
		recordRegistry.Registrations(),
		projectEntityRegistry.Registrations()...,
	)
	installed := mustRecordMembershipEngineValue(
		memberofruntime.NewRegistry(registrations))

	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:                         typedmemory.NewCodecRegistry(),
				EntitySetEnumerationEvaluators: enumeration,
				CandidateVisibilityEvaluators:  visibility,
				KindDefinednessEvaluators:      definedness,
				MemberOfEvaluators:             installed,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					catalog,
				},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					recordPolicy,
					entityPolicy,
				},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		withIssues, _ := resolution.(interface {
			Issues() []projecttypeenvruntime.Issue
		})
		issues := []projecttypeenvruntime.Issue(nil)
		if withIssues != nil {
			issues = withIssues.Issues()
		}
		t.Fatalf(
			"ObserveCurrentTargetRuntime() = %s, want matched; issues=%#v",
			resolution.Kind().String(),
			issues,
		)
	}
	exactRuntime, ok := matched.Registry()
	if !ok {
		t.Fatal("matched two-family runtime exposed no exact registry")
	}
	selectedRegistry, err := selectedMemberOfEvaluatorRegistry(exactRuntime)
	if err != nil {
		t.Fatalf("selectedMemberOfEvaluatorRegistry() error = %v", err)
	}
	if selectedRegistry.Len() != 2 {
		t.Fatalf("selected MemberOf registry length = %d, want 2", selectedRegistry.Len())
	}
	assertSelectedMemberOfFamily[RecordMembershipAdmissionEngine](t, selectedRegistry, recordRule, identity)
	assertSelectedMemberOfFamily[ProjectEntityMembershipAdmissionEngine](t, selectedRegistry, projectEntityRule, identity)

	selected, err := selectedMemberOfRuntime(basis, exactRuntime)
	if err != nil {
		t.Fatalf("selectedMemberOfRuntime() error = %v", err)
	}
	if _, ok := selected.(typedmemorystore.ExactMemberOfRuntime); !ok {
		t.Fatalf(
			"selected membership posture = %T, want ExactMemberOfRuntime",
			selected,
		)
	}
}

func assertSelectedMemberOfFamily[T any](
	t *testing.T,
	registry memberofruntime.Registry,
	rule typedmemory.RuleRef,
	identity typedmemoryevaluation.MechanismIdentity,
) {
	t.Helper()
	lookup, err := registry.Lookup(rule, identity)
	if err != nil {
		t.Fatalf("lookup selected MemberOf family %q: %v", rule.String(), err)
	}
	found, ok := lookup.(memberofruntime.Found)
	if !ok {
		t.Fatalf("selected MemberOf family %q = %T, want found", rule.String(), lookup)
	}
	if _, ok := found.Registration().Engine().(T); !ok {
		t.Fatalf(
			"selected MemberOf family %q engine = %T, want %T",
			rule.String(),
			found.Registration().Engine(),
			*new(T),
		)
	}
}

func selectedTwoFamilyMemberOfCatalog(
	t *testing.T,
	recordRule typedmemory.RuleRef,
	projectEntityRule typedmemory.RuleRef,
	enumerationRule typedmemory.RuleRef,
	visibilityRule typedmemory.RuleRef,
	definednessRule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismArtifactV1 {
	t.Helper()
	entries := []runtimemechanism.RuntimeMechanismEntryV1{
		mustRecordMembershipEngineValue(
			runtimemechanism.NewEntitySetEnumerationEntry(enumerationRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewCandidateVisibilityEntry(visibilityRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewKindDefinednessEntry(definednessRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewMemberOfEntry(recordRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewCarrierMembershipDeliveryEntry(recordRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewMemberOfEntry(projectEntityRule)),
		mustRecordMembershipEngineValue(
			runtimemechanism.NewCarrierMembershipDeliveryEntry(projectEntityRule)),
	}
	artifact := mustRecordMembershipEngineValue(
		typedmemory.NewCarrierRef("artifact:selected-two-family-memberof-runtime/v1"))
	edition := mustRecordMembershipEngineValue(
		typedmemory.NewCarrierEdition("build-20260717.1"))
	return mustRecordMembershipEngineValue(
		runtimemechanism.SealRuntimeMechanismArtifactV1(
			artifact,
			edition,
			entries,
		))
}

func selectedTwoFamilyRuntimeBasis(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	policies []recordmembershipregistration.RegistrationArtifactV1,
	recordRule typedmemory.RuleRef,
	projectEntityRule typedmemory.RuleRef,
	enumerationRule typedmemory.RuleRef,
	visibilityRule typedmemory.RuleRef,
	definednessRule typedmemory.RuleRef,
) projecttypeenv.RuntimeEvaluationBasisArtifact {
	t.Helper()
	mechanism := mustRecordMembershipEngineValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog))
	pins := []projecttypeenv.RuntimeEvaluationBasisPin{
		recordMembershipEngineEvaluatorPin(
			t,
			enumerationRule,
			projecttypeenv.RuntimeMechanismContractEntitySetEnumeration,
			mechanism,
			catalog,
		),
		recordMembershipEngineEvaluatorPin(
			t,
			visibilityRule,
			projecttypeenv.RuntimeMechanismContractCandidateVisibility,
			mechanism,
			catalog,
		),
		recordMembershipEngineEvaluatorPin(
			t,
			definednessRule,
			projecttypeenv.RuntimeMechanismContractKindDefinedness,
			mechanism,
			catalog,
		),
		recordMembershipEngineEvaluatorPin(
			t,
			recordRule,
			projecttypeenv.RuntimeMechanismContractMemberOf,
			mechanism,
			catalog,
		),
		recordMembershipEngineEvaluatorPin(
			t,
			projectEntityRule,
			projecttypeenv.RuntimeMechanismContractMemberOf,
			mechanism,
			catalog,
		),
		mustRecordMembershipEngineValue(
			projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
				projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
					Rule:             recordRule,
					Mechanism:        mechanism,
					ResolvedArtifact: &catalog,
				},
			)),
		mustRecordMembershipEngineValue(
			projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
				projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
					Rule:             projectEntityRule,
					Mechanism:        mechanism,
					ResolvedArtifact: &catalog,
				},
			)),
	}
	for _, policy := range policies {
		pins = append(
			pins,
			mustRecordMembershipEngineValue(
				projecttypeenv.NewRegistrationPolicyPin(policy)),
		)
	}
	return mustRecordMembershipEngineValue(
		projecttypeenv.SealRuntimeEvaluationBasisWithPins(
			pins,
			[]runtimemechanism.RuntimeMechanismArtifactV1{catalog},
			nil,
		))
}
