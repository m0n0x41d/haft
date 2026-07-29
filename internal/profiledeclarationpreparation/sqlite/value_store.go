package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func storeAuthoritySupport(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	plan profiledeclarationpreparation.Plan,
) error {
	support := plan.Support()
	builder := projectprofilesqlite.NewProfileOnboardingAuthoritySupportV1Builder(
		support.RoleAssignment(),
	)
	builder = builder.WithMethodDescriptionV2(support.MethodDescription())
	builder = builder.WithMethodContractV2(support.MethodContract())
	builder = builder.WithSystemAdmission(support.SystemAdmission())
	builder = builder.WithRoleAdmission(support.RoleAdmission())
	builder = builder.WithAssignmentJustification(support.AssignmentJustification())
	builder = builder.WithAssignmentProvenance(support.AssignmentProvenance())
	value, err := builder.Build()
	if err != nil {
		return err
	}
	snapshot, err := projectprofilesqlite.StoreAndReloadProfileOnboardingAuthoritySupportV1(
		ctx,
		transaction,
		value,
		plan.PreparedAt(),
	)
	if err != nil {
		return err
	}
	if _, ok := snapshot.Value(); !ok {
		return fmt.Errorf("durable profile authority support is unusable")
	}
	return nil
}

func storeValueSet(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	plan profiledeclarationpreparation.Plan,
	values profiledeclarationpreparation.ValueSet,
	recordedAt time.Time,
) error {
	value, err := storageValueSet(values)
	if err != nil {
		return err
	}
	snapshot, err := projectprofilesqlite.StoreAndReloadProfileOnboardingValueSetV1(
		ctx,
		transaction,
		plan.Root(),
		value,
		recordedAt,
	)
	if err != nil {
		return err
	}
	durable, ok := snapshot.Values()
	if !ok {
		return fmt.Errorf("durable profile-onboarding value set is unusable")
	}
	return validateDurableValuePins(values, durable)
}

func storageValueSet(
	values profiledeclarationpreparation.ValueSet,
) (projectprofilesqlite.ProfileOnboardingValueSetV1, error) {
	builder := projectprofilesqlite.NewProfileOnboardingValueSetV1Builder(
		values.WorkRecord(),
	)
	builder = builder.WithMethodDescriptionV2(values.MethodDescription())
	builder = builder.WithMethodContractV2(values.MethodContract())
	builder = builder.WithSystemAdmission(values.SystemAdmission())
	builder = builder.WithRoleAdmission(values.RoleAdmission())
	builder = builder.WithAssignmentJustification(values.AssignmentJustification())
	builder = builder.WithAssignmentProvenance(values.AssignmentProvenance())
	builder = builder.WithRoleAssignment(values.RoleAssignment())
	builder = builder.WithObservedBasis(values.ObservedBasis())
	builder = builder.WithEffect(values.Effect())
	builder = builder.WithAssessment(values.Assessment())
	return builder.Build()
}

func validateDurableValuePins(
	expected profiledeclarationpreparation.ValueSet,
	actual projectprofilesqlite.ProfileOnboardingValueSetV1,
) error {
	expectedWorkDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(
		expected.WorkRecord(),
	)
	if err != nil {
		return err
	}
	actualWorkDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(
		actual.WorkRecord(),
	)
	if err != nil {
		return err
	}
	expectedAssessmentDigest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(
		expected.Assessment(),
	)
	if err != nil {
		return err
	}
	actualAssessmentDigest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(
		actual.Assessment(),
	)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: expectedWorkDigest == actualWorkDigest, name: "Work digest"},
		{matches: expectedAssessmentDigest == actualAssessmentDigest, name: "assessment digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("durable profile-onboarding value set differs at %s", check.name)
		}
	}
	return nil
}
