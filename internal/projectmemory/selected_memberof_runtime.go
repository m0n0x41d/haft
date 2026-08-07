package projectmemory

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// exactMemberOfFamilyBinder is the package-owned bridge from an installed
// family engine to the callable refinement selected by one exact X. Every
// family must rebuild itself from the X-filtered prerequisite registries; a
// generic registration alone is not permission to reuse a broader installed
// engine.
type exactMemberOfFamilyBinder interface {
	bindExactMemberOfFamily(
		projecttypeenvruntime.ExactTargetRuntimeRegistry,
		memberofruntime.Registration,
	) (memberofruntime.Registration, error)
}

func selectedMemberOfEvaluatorRegistry(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (memberofruntime.Registry, error) {
	if !runtime.Valid() {
		return memberofruntime.Registry{}, ErrProjectTypeEnvRuntimeUnavailable
	}
	matched, found := runtime.MemberOfRegistry()
	if !found || matched.Len() == 0 {
		return memberofruntime.Registry{}, fmt.Errorf(
			"%w: selected X exposes no exact MemberOf evaluator registry",
			ErrProjectTypeEnvRuntimeUnavailable,
		)
	}
	bound := make([]memberofruntime.Registration, 0, matched.Len())
	for _, registration := range matched.Registrations() {
		binder, ok := registration.Engine().(exactMemberOfFamilyBinder)
		if !ok {
			return memberofruntime.Registry{}, fmt.Errorf(
				"%w: selected X MemberOf family %q has no exact-runtime binder",
				ErrProjectTypeEnvRuntimeUnavailable,
				registration.RuleRef().String(),
			)
		}
		rebound, err := binder.bindExactMemberOfFamily(runtime, registration)
		if err != nil {
			return memberofruntime.Registry{}, fmt.Errorf(
				"bind selected X MemberOf family %q: %w",
				registration.RuleRef().String(),
				err,
			)
		}
		if rebound.RuleRef() != registration.RuleRef() ||
			rebound.Identity() != registration.Identity() {
			return memberofruntime.Registry{}, fmt.Errorf(
				"%w: selected X MemberOf family %q changed exact coordinates while binding",
				ErrProjectTypeEnvRuntimeUncorrelated,
				registration.RuleRef().String(),
			)
		}
		bound = append(bound, rebound)
	}
	return memberofruntime.NewRegistry(bound)
}

func (RecordMembershipAdmissionEngine) bindExactMemberOfFamily(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
	registration memberofruntime.Registration,
) (memberofruntime.Registration, error) {
	engine, err := NewRecordMembershipAdmissionEngine(runtime)
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	registry, err := NewRecordMembershipEvaluatorRegistry(engine)
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	return exactBoundRegistration(registry, registration)
}

func (ProjectEntityMembershipAdmissionEngine) bindExactMemberOfFamily(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
	registration memberofruntime.Registration,
) (memberofruntime.Registration, error) {
	enumeration, found := runtime.EntitySetEnumerationRegistry()
	if !found || enumeration.Len() == 0 {
		return memberofruntime.Registration{}, ErrProjectEntityMembershipRuntimeMissing
	}
	definedness, found := runtime.KindDefinednessRegistry()
	if !found || definedness.Len() == 0 {
		return memberofruntime.Registration{}, ErrProjectEntityMembershipRuntimeMissing
	}
	visibility, found := runtime.CandidateVisibilityRegistry()
	if !found || visibility.Len() == 0 {
		return memberofruntime.Registration{}, ErrProjectEntityMembershipRuntimeMissing
	}
	engine, err := NewProjectEntityMembershipAdmissionEngineBuilder().
		SetEntitySetEnumeration(enumeration).
		SetCandidateVisibility(visibility).
		SetKindDefinedness(definedness).
		SetMechanismIdentity(registration.Identity()).
		Build()
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	registry, err := NewProjectEntityMembershipEvaluatorRegistry(engine)
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	return exactBoundRegistration(registry, registration)
}

func (CarrierFamilyMembershipAdmissionEngine) bindExactMemberOfFamily(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
	registration memberofruntime.Registration,
) (memberofruntime.Registration, error) {
	enumeration, found := runtime.EntitySetEnumerationRegistry()
	if !found || enumeration.Len() == 0 {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeMissing
	}
	definedness, found := runtime.KindDefinednessRegistry()
	if !found || definedness.Len() == 0 {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeMissing
	}
	visibility, found := runtime.CandidateVisibilityRegistry()
	if !found || visibility.Len() == 0 {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeMissing
	}
	policySet, found := runtime.RegistrationPolicies()
	if !found {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeMissing
	}
	policies, exact := policySet.(projecttypeenvruntime.ExactTargetRegistrationPolicyRegistry)
	if !exact {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeMissing
	}
	selectedPolicy, found := policies.Lookup(registration.RuleRef())
	if !found {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeMissing
	}
	policy, found := selectedPolicy.Artifact()
	if !found {
		return memberofruntime.Registration{}, ErrCarrierFamilyMembershipRuntimeInvalid
	}
	builder, err := exactCarrierFamilyBuilder(registration.RuleRef())
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	engine, err := builder.
		SetEntitySetEnumeration(enumeration).
		SetCandidateVisibility(visibility).
		SetKindDefinedness(definedness).
		SetMechanismIdentity(registration.Identity()).
		SetRegistrationPolicy(policy).
		Build()
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	registry, err := NewCarrierFamilyMembershipEvaluatorRegistry(
		[]CarrierFamilyMembershipAdmissionEngine{engine},
	)
	if err != nil {
		return memberofruntime.Registration{}, err
	}
	return exactBoundRegistration(registry, registration)
}

func exactCarrierFamilyBuilder(
	rule typedmemory.RuleRef,
) (CarrierFamilyMembershipAdmissionEngineBuilder, error) {
	builders := map[string]func() CarrierFamilyMembershipAdmissionEngineBuilder{
		"haft.member-of.carrier-edition-carrier/v1":           NewCarrierEditionMembershipAdmissionEngineBuilder,
		"haft.member-of.project-claim-carrier/v1":             NewProjectClaimMembershipAdmissionEngineBuilder,
		"haft.member-of.performed-work-occurrence-carrier/v1": NewPerformedWorkOccurrenceMembershipAdmissionEngineBuilder,
		"haft.member-of.code-anchor-carrier/v1":               NewCodeAnchorMembershipAdmissionEngineBuilder,
	}
	build, found := builders[rule.String()]
	if !found {
		return CarrierFamilyMembershipAdmissionEngineBuilder{}, fmt.Errorf(
			"%w: unsupported sealed carrier-family rule %q",
			ErrCarrierFamilyMembershipRuntimeInvalid,
			rule.String(),
		)
	}
	return build(), nil
}

func exactBoundRegistration(
	registry memberofruntime.Registry,
	expected memberofruntime.Registration,
) (memberofruntime.Registration, error) {
	registrations := registry.Registrations()
	if len(registrations) != 1 {
		return memberofruntime.Registration{}, fmt.Errorf(
			"exact MemberOf family binder returned %d registrations",
			len(registrations),
		)
	}
	actual := registrations[0]
	if actual.RuleRef() != expected.RuleRef() ||
		actual.Identity() != expected.Identity() {
		return memberofruntime.Registration{}, fmt.Errorf(
			"exact MemberOf family binder changed its registration coordinates",
		)
	}
	return actual, nil
}
