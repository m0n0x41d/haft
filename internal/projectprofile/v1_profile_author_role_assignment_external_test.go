package projectprofile_test

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type profileAuthorRoleAssignmentFixtureV1 struct {
	assignment            projectprofile.ProfileAuthorRoleAssignmentV1
	workRecord            projectprofile.ProfileOnboardingWorkRecord
	systemAdmissionRef    projectprofile.SystemAdmissionRef
	systemAdmissionDigest projectprofile.ContentDigest
	roleAdmissionRef      projectprofile.RoleAdmissionRef
	roleAdmissionDigest   projectprofile.ContentDigest
	justificationRef      projectprofile.RoleAssignmentJustificationRef
	justificationDigest   projectprofile.ContentDigest
	provenanceRef         projectprofile.RoleAssignmentProvenanceRef
	provenanceDigest      projectprofile.ContentDigest
}

func TestProfileAuthorRoleAssignmentV1IsConcreteRelationForExactWorkPerformer(t *testing.T) {
	fixture := mustProfileAuthorRoleAssignmentFixtureV1(t)
	assignment := fixture.assignment
	record := fixture.workRecord
	err := projectprofile.ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1(
		record,
		assignment,
	)
	if err != nil {
		t.Fatalf("ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1: %v", err)
	}

	if assignment.RoleAssignmentRef() != record.PerformedBy() {
		t.Fatal("assignment identity differs from Work.performedBy")
	}
	if assignment.HolderSystemRef() != record.ExecutedWithin() {
		t.Fatal("assignment holder differs from Work.executedWithin")
	}
	if assignment.AdmittedRoleRef() != projectprofile.ProfileAuthorRoleRefV1() {
		t.Fatal("assignment did not preserve the admitted ProfileAuthor role")
	}
	if assignment.BoundedContextRef() != record.BoundedContextRef() {
		t.Fatal("assignment context differs from the Work context")
	}
	window := assignment.ValidityWindow()
	workInterval := record.WorkInterval()
	if window.From().After(workInterval.From()) || window.Until().Before(workInterval.Until()) {
		t.Fatal("assignment validity window does not cover Work")
	}

	assertProfileAuthorSupportBindingsV1(t, fixture)
}

func TestProfileAuthorRoleAssignmentV1RejectsMismatchedWorkRelation(t *testing.T) {
	fixture := mustProfileAuthorRoleAssignmentFixtureV1(t)
	record := fixture.workRecord
	wrongAssignmentRef := mustRoleAssignmentRefV1(t, "role-assignment:foreign")
	wrongHolderRef := mustSystemRefV1(t, "system:foreign")
	wrongContextRef := mustBoundedContextRefV1(t, "context:foreign")
	lateFrom := record.WorkInterval().From().Add(time.Minute)
	lateUntil := record.WorkInterval().Until().Add(time.Hour)
	lateWindow, err := projectprofile.NewRoleAssignmentWindowV1(lateFrom, lateUntil)
	if err != nil {
		t.Fatalf("NewRoleAssignmentWindowV1: %v", err)
	}
	testCases := []struct {
		name    string
		builder projectprofile.ProfileAuthorRoleAssignmentV1Builder
	}{
		{
			name: "assignment identity",
			builder: newProfileAuthorRoleAssignmentBuilderFromValuesV1(
				t,
				wrongAssignmentRef,
				record.ExecutedWithin(),
				record.BoundedContextRef(),
				fixture.assignment.ValidityWindow(),
			),
		},
		{
			name: "holder",
			builder: newProfileAuthorRoleAssignmentBuilderFromValuesV1(
				t,
				record.PerformedBy(),
				wrongHolderRef,
				record.BoundedContextRef(),
				fixture.assignment.ValidityWindow(),
			),
		},
		{
			name: "context",
			builder: newProfileAuthorRoleAssignmentBuilderFromValuesV1(
				t,
				record.PerformedBy(),
				record.ExecutedWithin(),
				wrongContextRef,
				fixture.assignment.ValidityWindow(),
			),
		},
		{
			name: "window",
			builder: newProfileAuthorRoleAssignmentBuilderFromValuesV1(
				t,
				record.PerformedBy(),
				record.ExecutedWithin(),
				record.BoundedContextRef(),
				lateWindow,
			),
		},
	}
	acceptedIndex := slices.IndexFunc(testCases, func(testCase struct {
		name    string
		builder projectprofile.ProfileAuthorRoleAssignmentV1Builder
	}) bool {
		assignment, buildErr := testCase.builder.Build()
		if buildErr != nil {
			t.Fatalf("Build(%s): %v", testCase.name, buildErr)
		}
		validationErr := projectprofile.ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1(
			record,
			assignment,
		)
		return validationErr == nil
	})
	if acceptedIndex >= 0 {
		t.Fatalf("Work accepted mismatched RoleAssignment %s", testCases[acceptedIndex].name)
	}
}

func TestProfileAuthorRoleAssignmentV1CanonicalRoundTripAndDigest(t *testing.T) {
	fixture := mustProfileAuthorRoleAssignmentFixtureV1(t)
	data, err := projectprofile.EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(fixture.assignment)
	if err != nil {
		t.Fatalf("EncodeProfileAuthorRoleAssignmentV1CanonicalJSON: %v", err)
	}
	decoded, err := projectprofile.DecodeProfileAuthorRoleAssignmentV1CanonicalJSON(data)
	if err != nil {
		t.Fatalf("DecodeProfileAuthorRoleAssignmentV1CanonicalJSON: %v", err)
	}
	leftDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(fixture.assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	rightDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(decoded)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1(decoded): %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatal("RoleAssignment digest changed after canonical round trip")
	}
	trimmed := bytes.TrimSpace(data)
	if !bytes.Equal(data, trimmed) {
		t.Fatal("canonical RoleAssignment JSON contains surrounding whitespace")
	}

	nonCanonical := append([]byte{}, data...)
	nonCanonical = append(nonCanonical, '\n')
	if _, err := projectprofile.DecodeProfileAuthorRoleAssignmentV1CanonicalJSON(nonCanonical); err == nil {
		t.Fatal("decoder accepted a non-canonical whitespace variant")
	}
	dataText := string(data)
	unknownField := strings.Replace(dataText, "}", ",\"foreign\":true}", 1)
	unknownData := []byte(unknownField)
	if _, err := projectprofile.DecodeProfileAuthorRoleAssignmentV1CanonicalJSON(unknownData); err == nil {
		t.Fatal("decoder accepted a foreign JSON field")
	}
	foreignSchema := strings.Replace(
		dataText,
		"haft.project-profile.profile-author-role-assignment/v1",
		"foreign.role-assignment/v1",
		1,
	)
	foreignData := []byte(foreignSchema)
	if _, err := projectprofile.DecodeProfileAuthorRoleAssignmentV1CanonicalJSON(foreignData); err == nil {
		t.Fatal("decoder accepted a foreign schema")
	}
}

func TestProfileAuthorRoleAssignmentV1RejectsZeroMissingAndForeignRoleValues(t *testing.T) {
	zero := projectprofile.ProfileAuthorRoleAssignmentV1{}
	if _, err := projectprofile.EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(zero); err == nil {
		t.Fatal("encoder accepted zero ProfileAuthorRoleAssignmentV1")
	}
	if _, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(zero); err == nil {
		t.Fatal("digest accepted zero ProfileAuthorRoleAssignmentV1")
	}

	fixture := mustV1Fixture(t)
	wrongRole, err := projectprofile.NewRoleRef("haft:role:foreign/v1")
	if err != nil {
		t.Fatalf("NewRoleRef: %v", err)
	}
	assignmentWindow := fixture.assignment.ValidityWindow()
	builder := newProfileAuthorRoleAssignmentBuilderV1(t, fixture.record, assignmentWindow)
	builder = builder.Assigning(wrongRole)
	if _, err := builder.Build(); err == nil {
		t.Fatal("builder accepted a foreign role value")
	}

	base := newProfileAuthorRoleAssignmentBuilderV1(t, fixture.record, assignmentWindow)
	missingCases := []struct {
		name    string
		builder projectprofile.ProfileAuthorRoleAssignmentV1Builder
	}{
		{name: "system admission", builder: base.WithSystemAdmission(projectprofile.SystemAdmissionRef{}, projectprofile.ContentDigest{})},
		{name: "role admission", builder: base.WithRoleAdmission(projectprofile.RoleAdmissionRef{}, projectprofile.ContentDigest{})},
		{name: "justification", builder: base.JustifiedBy(projectprofile.RoleAssignmentJustificationRef{}, projectprofile.ContentDigest{})},
		{name: "provenance", builder: base.WithProvenance(projectprofile.RoleAssignmentProvenanceRef{}, projectprofile.ContentDigest{})},
	}
	acceptedIndex := slices.IndexFunc(missingCases, func(testCase struct {
		name    string
		builder projectprofile.ProfileAuthorRoleAssignmentV1Builder
	}) bool {
		_, err := testCase.builder.Build()
		return err == nil
	})
	if acceptedIndex >= 0 {
		t.Fatalf("builder accepted missing %s binding", missingCases[acceptedIndex].name)
	}
}

func TestProfileAuthorRoleAssignmentV1HasNoOpenInterfaceOrMutableSurface(t *testing.T) {
	fixture := mustProfileAuthorRoleAssignmentFixtureV1(t)
	assignmentType := reflect.TypeOf(fixture.assignment)
	fields := reflect.VisibleFields(assignmentType)
	exposedIndex := slices.IndexFunc(fields, func(field reflect.StructField) bool {
		return field.PkgPath == ""
	})
	if exposedIndex >= 0 {
		t.Fatalf("RoleAssignment exposes mutable field %q", fields[exposedIndex].Name)
	}
	interfaceIndex := slices.IndexFunc(fields, func(field reflect.StructField) bool {
		return field.Type.Kind() == reflect.Interface
	})
	if interfaceIndex >= 0 {
		t.Fatalf("RoleAssignment admits foreign interface embedding through %q", fields[interfaceIndex].Name)
	}

	changedBuilder := newProfileAuthorRoleAssignmentBuilderV1(
		t,
		fixture.workRecord,
		fixture.assignment.ValidityWindow(),
	)
	changedDigest := digestOfTB(t, "profile-author-role-assignment:changed-provenance")
	changedBuilder = changedBuilder.WithProvenance(fixture.provenanceRef, changedDigest)
	changed, err := changedBuilder.Build()
	if err != nil {
		t.Fatalf("Build(changed): %v", err)
	}
	left, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(fixture.assignment)
	if err != nil {
		t.Fatalf("Digest(original): %v", err)
	}
	right, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(changed)
	if err != nil {
		t.Fatalf("Digest(changed): %v", err)
	}
	if left == right {
		t.Fatal("RoleAssignment digest ignored changed provenance binding")
	}
}

func TestProfileAuthorRoleAssignmentV1CanonicalizesValidityWindowToUTC(t *testing.T) {
	fixture := mustV1Fixture(t)
	location := time.FixedZone("UTC+03", 3*60*60)
	from := time.Date(2026, 7, 14, 12, 0, 0, 0, location)
	until := from.Add(3 * time.Hour)
	window, err := projectprofile.NewRoleAssignmentWindowV1(from, until)
	if err != nil {
		t.Fatalf("NewRoleAssignmentWindowV1: %v", err)
	}
	builder := newProfileAuthorRoleAssignmentBuilderV1(t, fixture.record, window)
	assignment, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	data, err := projectprofile.EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(assignment)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dataText := string(data)
	if strings.Contains(dataText, "+03:00") {
		t.Fatal("canonical validity window retained a non-UTC offset")
	}
	if !strings.Contains(dataText, "2026-07-14T09:00:00Z") {
		t.Fatalf("canonical validity window lacks expected UTC start: %s", data)
	}
}

func mustProfileAuthorRoleAssignmentFixtureV1(
	t testing.TB,
) profileAuthorRoleAssignmentFixtureV1 {
	t.Helper()
	fixture := mustV1Fixture(t)
	assignmentWindow := fixture.assignment.ValidityWindow()
	builder := newProfileAuthorRoleAssignmentBuilderV1(t, fixture.record, assignmentWindow)
	assignment, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorRoleAssignmentV1Builder.Build: %v", err)
	}
	return profileAuthorRoleAssignmentFixtureV1{
		assignment:            assignment,
		workRecord:            fixture.record,
		systemAdmissionRef:    mustSystemAdmissionRefV1(t, "system-admission:profile-agent/v1"),
		systemAdmissionDigest: digestOfTB(t, "profile-author-system-admission"),
		roleAdmissionRef:      mustRoleAdmissionRefV1(t, "role-admission:profile-author/v1"),
		roleAdmissionDigest:   digestOfTB(t, "profile-author-role-admission"),
		justificationRef:      mustRoleAssignmentJustificationRefV1(t, "role-assignment-justification:profile-onboarding/v1"),
		justificationDigest:   digestOfTB(t, "profile-author-role-assignment-justification"),
		provenanceRef:         mustRoleAssignmentProvenanceRefV1(t, "role-assignment-provenance:profile-onboarding/v1"),
		provenanceDigest:      digestOfTB(t, "profile-author-role-assignment-provenance"),
	}
}

func newProfileAuthorRoleAssignmentBuilderV1(
	t testing.TB,
	record projectprofile.ProfileOnboardingWorkRecord,
	window projectprofile.RoleAssignmentWindowV1,
) projectprofile.ProfileAuthorRoleAssignmentV1Builder {
	t.Helper()
	return newProfileAuthorRoleAssignmentBuilderFromValuesV1(
		t,
		record.PerformedBy(),
		record.ExecutedWithin(),
		record.BoundedContextRef(),
		window,
	)
}

func newProfileAuthorRoleAssignmentBuilderFromValuesV1(
	t testing.TB,
	assignmentRef projectprofile.RoleAssignmentRef,
	holderRef projectprofile.SystemRef,
	contextRef projectprofile.BoundedContextRef,
	window projectprofile.RoleAssignmentWindowV1,
) projectprofile.ProfileAuthorRoleAssignmentV1Builder {
	t.Helper()
	systemAdmissionRef := mustSystemAdmissionRefV1(t, "system-admission:profile-agent/v1")
	systemAdmissionDigest := digestOfTB(t, "profile-author-system-admission")
	roleAdmissionRef := mustRoleAdmissionRefV1(t, "role-admission:profile-author/v1")
	roleAdmissionDigest := digestOfTB(t, "profile-author-role-admission")
	justificationRef := mustRoleAssignmentJustificationRefV1(t, "role-assignment-justification:profile-onboarding/v1")
	justificationDigest := digestOfTB(t, "profile-author-role-assignment-justification")
	provenanceRef := mustRoleAssignmentProvenanceRefV1(t, "role-assignment-provenance:profile-onboarding/v1")
	provenanceDigest := digestOfTB(t, "profile-author-role-assignment-provenance")
	builder := projectprofile.NewProfileAuthorRoleAssignmentV1Builder(assignmentRef)
	builder = builder.HeldBy(holderRef)
	builder = builder.Assigning(projectprofile.ProfileAuthorRoleRefV1())
	builder = builder.InContext(contextRef)
	builder = builder.ValidDuring(window)
	builder = builder.WithSystemAdmission(systemAdmissionRef, systemAdmissionDigest)
	builder = builder.WithRoleAdmission(roleAdmissionRef, roleAdmissionDigest)
	builder = builder.JustifiedBy(justificationRef, justificationDigest)
	builder = builder.WithProvenance(provenanceRef, provenanceDigest)
	return builder
}

func assertProfileAuthorSupportBindingsV1(
	t testing.TB,
	fixture profileAuthorRoleAssignmentFixtureV1,
) {
	t.Helper()
	assignment := fixture.assignment
	if assignment.SystemAdmissionRef() != fixture.systemAdmissionRef ||
		assignment.SystemAdmissionDigest() != fixture.systemAdmissionDigest {
		t.Fatal("assignment lost the system-admission binding")
	}
	if assignment.RoleAdmissionRef() != fixture.roleAdmissionRef ||
		assignment.RoleAdmissionDigest() != fixture.roleAdmissionDigest {
		t.Fatal("assignment lost the role-admission binding")
	}
	if assignment.JustificationRef() != fixture.justificationRef ||
		assignment.JustificationDigest() != fixture.justificationDigest {
		t.Fatal("assignment lost the justification binding")
	}
	if assignment.ProvenanceRef() != fixture.provenanceRef ||
		assignment.ProvenanceDigest() != fixture.provenanceDigest {
		t.Fatal("assignment lost the provenance binding")
	}
}

func mustSystemAdmissionRefV1(t testing.TB, raw string) projectprofile.SystemAdmissionRef {
	t.Helper()
	ref, err := projectprofile.NewSystemAdmissionRef(raw)
	if err != nil {
		t.Fatalf("NewSystemAdmissionRef: %v", err)
	}
	return ref
}

func mustRoleAdmissionRefV1(t testing.TB, raw string) projectprofile.RoleAdmissionRef {
	t.Helper()
	ref, err := projectprofile.NewRoleAdmissionRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAdmissionRef: %v", err)
	}
	return ref
}

func mustRoleAssignmentJustificationRefV1(
	t testing.TB,
	raw string,
) projectprofile.RoleAssignmentJustificationRef {
	t.Helper()
	ref, err := projectprofile.NewRoleAssignmentJustificationRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentJustificationRef: %v", err)
	}
	return ref
}

func mustRoleAssignmentProvenanceRefV1(
	t testing.TB,
	raw string,
) projectprofile.RoleAssignmentProvenanceRef {
	t.Helper()
	ref, err := projectprofile.NewRoleAssignmentProvenanceRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentProvenanceRef: %v", err)
	}
	return ref
}
