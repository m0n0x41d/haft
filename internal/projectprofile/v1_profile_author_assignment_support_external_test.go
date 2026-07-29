package projectprofile_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type profileAuthorAssignmentSupportFixtureV1 struct {
	systemAdmission  projectprofile.ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission    projectprofile.ProfileAuthorRoleAdmissionV1
	justification    projectprofile.ProfileAuthorAssignmentJustificationV1
	provenance       projectprofile.ProfileAuthorAssignmentProvenanceV1
	carrier          projectprofile.ProfileAuthorAssignmentSupportCarrierV1
	assignment       projectprofile.ProfileAuthorRoleAssignmentV1
	systemRef        projectprofile.SystemRef
	sessionRef       projectprofile.SessionRef
	kernel           projectprofile.ProfileOnboardingKernelIdentityV1
	systemWindow     projectprofile.ProfileOnboardingExecutorAdmissionWindowV1
	assignmentWindow projectprofile.RoleAssignmentWindowV1
}

func TestProfileAuthorAssignmentSupportV1BindsExactClosedChain(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	err := projectprofile.ValidateProfileAuthorRoleAssignmentV1Support(
		fixture.assignment,
		fixture.carrier,
	)
	if err != nil {
		t.Fatalf("ValidateProfileAuthorRoleAssignmentV1Support: %v", err)
	}
	if fixture.systemAdmission.GoverningPatternRef().String() != "A.1" {
		t.Fatal("executor-system admission lost direct A.1 authority")
	}
	if fixture.roleAdmission.GoverningPatternRef().String() != "A.2.1" {
		t.Fatal("ProfileAuthor assignment admission did not use direct A.2.1 authority")
	}
	if fixture.systemAdmission.SessionRef() != fixture.provenance.SessionRef() {
		t.Fatal("system admission and provenance lost the exact onboarding session")
	}
	if !fixture.systemAdmission.ValidityWindow().CoversWork(mustSupportWorkIntervalV1(t)) {
		t.Fatal("executor-system admission window did not cover the bounded Work interval")
	}
	if fixture.systemAdmission.IdentityBasis().Kind().String() != "kernel_owned" {
		t.Fatal("executor-system admission lost the kernel-owned identity discriminant")
	}
	if fixture.carrier.SystemAdmission().Ref() != fixture.systemAdmission.Ref() ||
		fixture.carrier.RoleAdmission().Ref() != fixture.roleAdmission.Ref() ||
		fixture.carrier.Justification().Ref() != fixture.justification.Ref() ||
		fixture.carrier.Provenance().Ref() != fixture.provenance.Ref() {
		t.Fatal("support carrier did not preserve four separately addressed objects")
	}
}

func TestProfileAuthorAssignmentSupportV1CanonicalRoundTripsAndDigests(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	testCases := []struct {
		name       string
		encode     func() ([]byte, error)
		roundTrip  func([]byte) error
		wantDigest projectprofile.ContentDigest
	}{
		{
			name: "executor system admission",
			encode: func() ([]byte, error) {
				return projectprofile.EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(fixture.systemAdmission)
			},
			roundTrip: func(data []byte) error {
				value, err := projectprofile.DecodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(data)
				if err != nil {
					return err
				}
				digest, err := projectprofile.DigestProfileOnboardingExecutorSystemAdmissionV1(value)
				if err != nil {
					return err
				}
				if digest != fixture.carrier.SystemAdmissionDigest() {
					return supportTestErrorV1("executor-system admission digest changed after round trip")
				}
				return nil
			},
			wantDigest: fixture.carrier.SystemAdmissionDigest(),
		},
		{
			name: "role admission",
			encode: func() ([]byte, error) {
				return projectprofile.EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(fixture.roleAdmission)
			},
			roundTrip: func(data []byte) error {
				value, err := projectprofile.DecodeProfileAuthorRoleAdmissionV1CanonicalJSON(data)
				if err != nil {
					return err
				}
				digest, err := projectprofile.DigestProfileAuthorRoleAdmissionV1(value)
				if err != nil {
					return err
				}
				if digest != fixture.carrier.RoleAdmissionDigest() {
					return supportTestErrorV1("role-admission digest changed after round trip")
				}
				return nil
			},
			wantDigest: fixture.carrier.RoleAdmissionDigest(),
		},
		{
			name: "assignment justification",
			encode: func() ([]byte, error) {
				return projectprofile.EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(fixture.justification)
			},
			roundTrip: func(data []byte) error {
				value, err := projectprofile.DecodeProfileAuthorAssignmentJustificationV1CanonicalJSON(data)
				if err != nil {
					return err
				}
				digest, err := projectprofile.DigestProfileAuthorAssignmentJustificationV1(value)
				if err != nil {
					return err
				}
				if digest != fixture.carrier.JustificationDigest() {
					return supportTestErrorV1("justification digest changed after round trip")
				}
				return nil
			},
			wantDigest: fixture.carrier.JustificationDigest(),
		},
		{
			name: "assignment provenance",
			encode: func() ([]byte, error) {
				return projectprofile.EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(fixture.provenance)
			},
			roundTrip: func(data []byte) error {
				value, err := projectprofile.DecodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(data)
				if err != nil {
					return err
				}
				digest, err := projectprofile.DigestProfileAuthorAssignmentProvenanceV1(value)
				if err != nil {
					return err
				}
				if digest != fixture.carrier.ProvenanceDigest() {
					return supportTestErrorV1("provenance digest changed after round trip")
				}
				return nil
			},
			wantDigest: fixture.carrier.ProvenanceDigest(),
		},
	}
	failingIndex := slices.IndexFunc(testCases, func(testCase struct {
		name       string
		encode     func() ([]byte, error)
		roundTrip  func([]byte) error
		wantDigest projectprofile.ContentDigest
	}) bool {
		data, err := testCase.encode()
		if err != nil {
			t.Fatalf("Encode(%s): %v", testCase.name, err)
		}
		if !bytes.Equal(data, bytes.TrimSpace(data)) {
			t.Fatalf("%s canonical JSON contains surrounding whitespace", testCase.name)
		}
		return testCase.roundTrip(data) != nil
	})
	if failingIndex >= 0 {
		data, _ := testCases[failingIndex].encode()
		t.Fatalf("%s canonical round trip failed: %v", testCases[failingIndex].name, testCases[failingIndex].roundTrip(data))
	}
}

func TestProfileAuthorAssignmentSupportV1RejectsUnderdeterminedExecutorIdentity(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	zeroBasis := projectprofile.ProfileOnboardingExecutorIdentityBasisV1{}
	if _, _, ok := zeroBasis.OperatorDesignation(); ok {
		t.Fatal("zero executor identity basis exposed an operator designation")
	}
	builder := projectprofile.NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		mustSupportSystemAdmissionRefV1(t, "system-admission:underdetermined/v1"),
		fixture.systemRef,
	)
	builder = builder.IdentifiedBy(zeroBasis)
	builder = builder.AdmittedToActBy(
		mustSupportActingBasisRefV1(t, "acting-basis:underdetermined/v1"),
		digestSupportV1(t, "acting-basis-underdetermined"),
	)
	builder = builder.InSession(fixture.sessionRef)
	builder = builder.ValidDuring(fixture.systemWindow)
	if _, err := builder.Build(); err == nil || !strings.Contains(err.Error(), "underdetermined") {
		t.Fatalf("Build(zero identity basis) error = %v, want underdetermined", err)
	}

	foreignSystem := mustSupportSystemRefV1(t, "system:foreign-executor/v1")
	foreignBasis, err := projectprofile.NewProfileOnboardingKernelExecutorIdentityBasisV1(
		foreignSystem,
		fixture.kernel,
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelExecutorIdentityBasisV1: %v", err)
	}
	builder = builder.IdentifiedBy(foreignBasis)
	if _, err := builder.Build(); err == nil {
		t.Fatal("system admission accepted identity basis bound to another system")
	}
}

func TestProfileAuthorAssignmentSupportV1RejectsWindowSessionAndKernelDrift(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	wrongSession := mustSupportSessionRefV1(t, "session:profile-onboarding:foreign/v1")
	wrongSessionProvenance := mustSupportProvenanceV1(
		t,
		fixture.justification,
		wrongSession,
		fixture.kernel,
		"provenance:wrong-session/v1",
	)
	if _, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		fixture.systemAdmission,
		fixture.roleAdmission,
		fixture.justification,
		wrongSessionProvenance,
	); err == nil {
		t.Fatal("support carrier accepted provenance from another onboarding session")
	}

	otherKernel, err := projectprofile.NewProfileOnboardingKernelIdentityV1("haft-kernel", "build-foreign")
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelIdentityV1: %v", err)
	}
	wrongKernelProvenance := mustSupportProvenanceV1(
		t,
		fixture.justification,
		fixture.sessionRef,
		otherKernel,
		"provenance:wrong-kernel/v1",
	)
	if _, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		fixture.systemAdmission,
		fixture.roleAdmission,
		fixture.justification,
		wrongKernelProvenance,
	); err == nil {
		t.Fatal("kernel-owned executor admission accepted different provenance kernel build")
	}

	outOfWindowBuilder := projectprofile.NewProfileAuthorAssignmentProvenanceV1Builder(
		mustSupportProvenanceRefV1(t, "provenance:outside-system-admission/v1"),
		fixture.justification,
	)
	outOfWindowBuilder = outOfWindowBuilder.InSession(fixture.sessionRef)
	outOfWindowBuilder = outOfWindowBuilder.ProducedBy(
		fixture.kernel,
		fixture.provenance.Runtime(),
	)
	outOfWindowBuilder = outOfWindowBuilder.RecordedAt(
		fixture.systemWindow.Until().Add(time.Second),
	)
	outOfWindowProvenance, err := outOfWindowBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentProvenanceV1Builder.Build: %v", err)
	}
	if _, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		fixture.systemAdmission,
		fixture.roleAdmission,
		fixture.justification,
		outOfWindowProvenance,
	); err == nil || !strings.Contains(err.Error(), "inside the executor-system admission window") {
		t.Fatalf("out-of-window origin metadata error = %v, want system-admission-window rejection", err)
	}

	narrowSystemWindow, err := projectprofile.NewProfileOnboardingExecutorAdmissionWindowV1(
		fixture.assignmentWindow.From().Add(time.Minute),
		fixture.assignmentWindow.Until().Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingExecutorAdmissionWindowV1: %v", err)
	}
	narrowSystem := mustSupportSystemAdmissionV1(
		t,
		fixture.systemRef,
		fixture.sessionRef,
		fixture.kernel,
		narrowSystemWindow,
		"system-admission:narrow/v1",
	)
	justificationBuilder := projectprofile.NewProfileAuthorAssignmentJustificationV1Builder(
		mustSupportJustificationRefV1(t, "justification:narrow/v1"),
	)
	justificationBuilder = justificationBuilder.ApplyingAdmissions(narrowSystem, fixture.roleAdmission)
	justificationBuilder = justificationBuilder.ValidDuring(fixture.assignmentWindow)
	if _, err := justificationBuilder.Build(); err == nil {
		t.Fatal("justification accepted assignment window outside system-admission window")
	}
}

func TestProfileAuthorAssignmentSupportV1OperatorDesignationIsExactAlternative(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	designationRef, err := projectprofile.NewProfileOnboardingSystemIdentityBasisRefV1(
		"operator-designation:executor/v1",
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingSystemIdentityBasisRefV1: %v", err)
	}
	basis, err := projectprofile.NewProfileOnboardingOperatorDesignatedExecutorIdentityBasisV1(
		fixture.systemRef,
		designationRef,
		digestSupportV1(t, "operator-designated-executor"),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOperatorDesignatedExecutorIdentityBasisV1: %v", err)
	}
	if basis.Kind().String() != "operator_designated" {
		t.Fatal("operator-designated basis lost its closed discriminant")
	}
	if _, ok := basis.KernelIdentity(); ok {
		t.Fatal("operator-designated basis exposed a kernel-owned identity")
	}
	systemAdmission := mustSupportSystemAdmissionWithBasisV1(
		t,
		fixture.systemRef,
		fixture.sessionRef,
		basis,
		fixture.systemWindow,
		"system-admission:operator-designated/v1",
	)
	justification := mustSupportJustificationV1(
		t,
		systemAdmission,
		fixture.roleAdmission,
		fixture.assignmentWindow,
		"justification:operator-designated/v1",
	)
	otherKernel, err := projectprofile.NewProfileOnboardingKernelIdentityV1("haft-kernel", "build-observer")
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelIdentityV1: %v", err)
	}
	provenance := mustSupportProvenanceV1(
		t,
		justification,
		fixture.sessionRef,
		otherKernel,
		"provenance:operator-designated/v1",
	)
	carrier, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		fixture.roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		t.Fatalf("CarryProfileAuthorAssignmentSupportV1: %v", err)
	}
	data := carrier.SystemAdmissionCanonicalJSON()
	decoded, err := projectprofile.DecodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(data)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON: %v", err)
	}
	if decoded.IdentityBasis().Kind().String() != "operator_designated" {
		t.Fatal("operator-designated identity basis changed during canonical round trip")
	}
}

func TestProfileAuthorAssignmentSupportV1StrictDecodersRejectForeignStructure(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	systemData := fixture.carrier.SystemAdmissionCanonicalJSON()
	roleData := fixture.carrier.RoleAdmissionCanonicalJSON()
	provenanceData := fixture.carrier.ProvenanceCanonicalJSON()
	mutations := []struct {
		name   string
		decode func([]byte) error
		data   []byte
	}{
		{
			name: "non-canonical system whitespace",
			decode: func(data []byte) error {
				_, err := projectprofile.DecodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(data)
				return err
			},
			data: append(append([]byte{}, systemData...), '\n'),
		},
		{
			name: "mixed executor identity variants",
			decode: func(data []byte) error {
				_, err := projectprofile.DecodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(data)
				return err
			},
			data: []byte(strings.Replace(
				string(systemData),
				"\"kernel_owned\":{",
				"\"operator_designated\":{\"ref\":\"designation:foreign\",\"digest\":\"sha256:"+strings.Repeat("a", 64)+"\"},\"kernel_owned\":{",
				1,
			)),
		},
		{
			name: "role family instead of direct pattern",
			decode: func(data []byte) error {
				_, err := projectprofile.DecodeProfileAuthorRoleAdmissionV1CanonicalJSON(data)
				return err
			},
			data: []byte(strings.Replace(string(roleData), "\"A.2.1\"", "\"A.2\"", 1)),
		},
		{
			name: "provenance foreign field",
			decode: func(data []byte) error {
				_, err := projectprofile.DecodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(data)
				return err
			},
			data: []byte(strings.Replace(string(provenanceData), "}", ",\"role_assignment_ref\":\"role-assignment:forbidden\"}", 1)),
		},
	}
	acceptedIndex := slices.IndexFunc(mutations, func(mutation struct {
		name   string
		decode func([]byte) error
		data   []byte
	}) bool {
		return mutation.decode(mutation.data) == nil
	})
	if acceptedIndex >= 0 {
		t.Fatalf("strict decoder accepted %s", mutations[acceptedIndex].name)
	}
}

func TestProfileAuthorAssignmentSupportV1HasNoAggregateOntologyOrAssignmentCycle(t *testing.T) {
	fixture := mustProfileAuthorAssignmentSupportFixtureV1(t)
	carrierType := reflect.TypeOf(fixture.carrier)
	forbiddenMethods := []string{"Ref", "Schema", "Digest", "ContentDigest"}
	methodIndex := slices.IndexFunc(forbiddenMethods, func(name string) bool {
		_, found := carrierType.MethodByName(name)
		return found
	})
	if methodIndex >= 0 {
		t.Fatalf("support carrier exposes aggregate ontology method %q", forbiddenMethods[methodIndex])
	}
	provenanceType := reflect.TypeOf(fixture.provenance)
	fieldIndex := slices.IndexFunc(reflect.VisibleFields(provenanceType), func(field reflect.StructField) bool {
		return strings.Contains(strings.ToLower(field.Name), "roleassignment")
	})
	if fieldIndex >= 0 {
		t.Fatalf("provenance embeds a RoleAssignment dependency in field %q", reflect.VisibleFields(provenanceType)[fieldIndex].Name)
	}
	if strings.Contains(string(fixture.carrier.ProvenanceCanonicalJSON()), "role_assignment") {
		t.Fatal("provenance JSON contains a RoleAssignment dependency")
	}

	data := fixture.carrier.SystemAdmissionCanonicalJSON()
	original := fixture.carrier.SystemAdmissionCanonicalJSON()
	data[0] = 'X'
	if !bytes.Equal(original, fixture.carrier.SystemAdmissionCanonicalJSON()) {
		t.Fatal("support carrier leaked mutable canonical JSON bytes")
	}
}

func TestProfileAuthorAssignmentSupportV1RejectsZeroObjects(t *testing.T) {
	zeroSystem := projectprofile.ProfileOnboardingExecutorSystemAdmissionV1{}
	zeroRole := projectprofile.ProfileAuthorRoleAdmissionV1{}
	zeroJustification := projectprofile.ProfileAuthorAssignmentJustificationV1{}
	zeroProvenance := projectprofile.ProfileAuthorAssignmentProvenanceV1{}
	testCases := []struct {
		name   string
		encode func() error
	}{
		{name: "system admission", encode: func() error {
			_, err := projectprofile.EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(zeroSystem)
			return err
		}},
		{name: "role admission", encode: func() error {
			_, err := projectprofile.EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(zeroRole)
			return err
		}},
		{name: "justification", encode: func() error {
			_, err := projectprofile.EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(zeroJustification)
			return err
		}},
		{name: "provenance", encode: func() error {
			_, err := projectprofile.EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(zeroProvenance)
			return err
		}},
	}
	acceptedIndex := slices.IndexFunc(testCases, func(testCase struct {
		name   string
		encode func() error
	}) bool {
		return testCase.encode() == nil
	})
	if acceptedIndex >= 0 {
		t.Fatalf("encoder accepted zero %s", testCases[acceptedIndex].name)
	}
}

type supportTestErrorV1 string

func (err supportTestErrorV1) Error() string { return string(err) }

func mustProfileAuthorAssignmentSupportFixtureV1(
	t testing.TB,
) profileAuthorAssignmentSupportFixtureV1 {
	t.Helper()
	systemRef := mustSupportSystemRefV1(t, "system:haft-profile-onboarding/v1")
	sessionRef := mustSupportSessionRefV1(t, "session:profile-onboarding:test/v1")
	kernel, err := projectprofile.NewProfileOnboardingKernelIdentityV1("haft-kernel", "build-test")
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelIdentityV1: %v", err)
	}
	systemWindow, err := projectprofile.NewProfileOnboardingExecutorAdmissionWindowV1(
		time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 18, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingExecutorAdmissionWindowV1: %v", err)
	}
	assignmentWindow, err := projectprofile.NewRoleAssignmentWindowV1(
		time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 17, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewRoleAssignmentWindowV1: %v", err)
	}
	systemAdmission := mustSupportSystemAdmissionV1(
		t,
		systemRef,
		sessionRef,
		kernel,
		systemWindow,
		"system-admission:profile-onboarding/test-v1",
	)
	roleAdmission, err := projectprofile.NewProfileAuthorRoleAdmissionV1(
		mustSupportRoleAdmissionRefV1(t, "role-admission:profile-author/test-v1"),
	)
	if err != nil {
		t.Fatalf("NewProfileAuthorRoleAdmissionV1: %v", err)
	}
	justification := mustSupportJustificationV1(
		t,
		systemAdmission,
		roleAdmission,
		assignmentWindow,
		"justification:profile-author/test-v1",
	)
	provenance := mustSupportProvenanceV1(
		t,
		justification,
		sessionRef,
		kernel,
		"provenance:profile-author/test-v1",
	)
	carrier, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		t.Fatalf("CarryProfileAuthorAssignmentSupportV1: %v", err)
	}
	assignmentBuilder := projectprofile.NewProfileAuthorRoleAssignmentV1Builder(
		mustSupportRoleAssignmentRefV1(t, "role-assignment:profile-author/test-v1"),
	)
	assignmentBuilder = assignmentBuilder.HeldBy(systemRef)
	assignmentBuilder = assignmentBuilder.Assigning(projectprofile.ProfileAuthorRoleRefV1())
	assignmentBuilder = assignmentBuilder.InContext(projectprofile.ProfileOnboardingBoundedContextRefV1())
	assignmentBuilder = assignmentBuilder.ValidDuring(assignmentWindow)
	assignmentBuilder = assignmentBuilder.WithSystemAdmission(systemAdmission.Ref(), carrier.SystemAdmissionDigest())
	assignmentBuilder = assignmentBuilder.WithRoleAdmission(roleAdmission.Ref(), carrier.RoleAdmissionDigest())
	assignmentBuilder = assignmentBuilder.JustifiedBy(justification.Ref(), carrier.JustificationDigest())
	assignmentBuilder = assignmentBuilder.WithProvenance(provenance.Ref(), carrier.ProvenanceDigest())
	assignment, err := assignmentBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorRoleAssignmentV1Builder.Build: %v", err)
	}
	return profileAuthorAssignmentSupportFixtureV1{
		systemAdmission:  systemAdmission,
		roleAdmission:    roleAdmission,
		justification:    justification,
		provenance:       provenance,
		carrier:          carrier,
		assignment:       assignment,
		systemRef:        systemRef,
		sessionRef:       sessionRef,
		kernel:           kernel,
		systemWindow:     systemWindow,
		assignmentWindow: assignmentWindow,
	}
}

func mustSupportSystemAdmissionV1(
	t testing.TB,
	systemRef projectprofile.SystemRef,
	sessionRef projectprofile.SessionRef,
	kernel projectprofile.ProfileOnboardingKernelIdentityV1,
	window projectprofile.ProfileOnboardingExecutorAdmissionWindowV1,
	refRaw string,
) projectprofile.ProfileOnboardingExecutorSystemAdmissionV1 {
	t.Helper()
	basis, err := projectprofile.NewProfileOnboardingKernelExecutorIdentityBasisV1(systemRef, kernel)
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelExecutorIdentityBasisV1: %v", err)
	}
	return mustSupportSystemAdmissionWithBasisV1(t, systemRef, sessionRef, basis, window, refRaw)
}

func mustSupportSystemAdmissionWithBasisV1(
	t testing.TB,
	systemRef projectprofile.SystemRef,
	sessionRef projectprofile.SessionRef,
	basis projectprofile.ProfileOnboardingExecutorIdentityBasisV1,
	window projectprofile.ProfileOnboardingExecutorAdmissionWindowV1,
	refRaw string,
) projectprofile.ProfileOnboardingExecutorSystemAdmissionV1 {
	t.Helper()
	builder := projectprofile.NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		mustSupportSystemAdmissionRefV1(t, refRaw),
		systemRef,
	)
	builder = builder.IdentifiedBy(basis)
	builder = builder.AdmittedToActBy(
		mustSupportActingBasisRefV1(t, "acting-eligibility:"+refRaw),
		digestSupportV1(t, "acting-eligibility-"+refRaw),
	)
	builder = builder.InSession(sessionRef)
	builder = builder.ValidDuring(window)
	value, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingExecutorSystemAdmissionV1Builder.Build: %v", err)
	}
	return value
}

func mustSupportJustificationV1(
	t testing.TB,
	systemAdmission projectprofile.ProfileOnboardingExecutorSystemAdmissionV1,
	roleAdmission projectprofile.ProfileAuthorRoleAdmissionV1,
	window projectprofile.RoleAssignmentWindowV1,
	refRaw string,
) projectprofile.ProfileAuthorAssignmentJustificationV1 {
	t.Helper()
	builder := projectprofile.NewProfileAuthorAssignmentJustificationV1Builder(
		mustSupportJustificationRefV1(t, refRaw),
	)
	builder = builder.ApplyingAdmissions(systemAdmission, roleAdmission)
	builder = builder.ValidDuring(window)
	value, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentJustificationV1Builder.Build: %v", err)
	}
	return value
}

func mustSupportProvenanceV1(
	t testing.TB,
	justification projectprofile.ProfileAuthorAssignmentJustificationV1,
	sessionRef projectprofile.SessionRef,
	kernel projectprofile.ProfileOnboardingKernelIdentityV1,
	refRaw string,
) projectprofile.ProfileAuthorAssignmentProvenanceV1 {
	t.Helper()
	runtime, err := projectprofile.NewProfileOnboardingRuntimeIdentityV1("codex-runtime", "runtime-test")
	if err != nil {
		t.Fatalf("NewProfileOnboardingRuntimeIdentityV1: %v", err)
	}
	builder := projectprofile.NewProfileAuthorAssignmentProvenanceV1Builder(
		mustSupportProvenanceRefV1(t, refRaw),
		justification,
	)
	builder = builder.InSession(sessionRef)
	builder = builder.ProducedBy(kernel, runtime)
	builder = builder.RecordedAt(time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC))
	value, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorAssignmentProvenanceV1Builder.Build: %v", err)
	}
	return value
}

func mustSupportSessionRefV1(t testing.TB, raw string) projectprofile.SessionRef {
	t.Helper()
	value, err := projectprofile.NewSessionRef(raw)
	if err != nil {
		t.Fatalf("NewSessionRef: %v", err)
	}
	return value
}

func mustSupportActingBasisRefV1(
	t testing.TB,
	raw string,
) projectprofile.ProfileOnboardingSystemActingEligibilityBasisRefV1 {
	t.Helper()
	value, err := projectprofile.NewProfileOnboardingSystemActingEligibilityBasisRefV1(raw)
	if err != nil {
		t.Fatalf("NewProfileOnboardingSystemActingEligibilityBasisRefV1: %v", err)
	}
	return value
}

func mustSupportWorkIntervalV1(t testing.TB) projectprofile.WorkIntervalV1 {
	t.Helper()
	value, err := projectprofile.NewWorkIntervalV1(
		time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 16, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	return value
}

func digestSupportV1(t testing.TB, seed string) projectprofile.ContentDigest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	value, err := projectprofile.NewContentDigest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return value
}

func mustSupportSystemRefV1(t testing.TB, raw string) projectprofile.SystemRef {
	t.Helper()
	value, err := projectprofile.NewSystemRef(raw)
	if err != nil {
		t.Fatalf("NewSystemRef: %v", err)
	}
	return value
}

func mustSupportSystemAdmissionRefV1(
	t testing.TB,
	raw string,
) projectprofile.SystemAdmissionRef {
	t.Helper()
	value, err := projectprofile.NewSystemAdmissionRef(raw)
	if err != nil {
		t.Fatalf("NewSystemAdmissionRef: %v", err)
	}
	return value
}

func mustSupportRoleAdmissionRefV1(
	t testing.TB,
	raw string,
) projectprofile.RoleAdmissionRef {
	t.Helper()
	value, err := projectprofile.NewRoleAdmissionRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAdmissionRef: %v", err)
	}
	return value
}

func mustSupportJustificationRefV1(
	t testing.TB,
	raw string,
) projectprofile.RoleAssignmentJustificationRef {
	t.Helper()
	value, err := projectprofile.NewRoleAssignmentJustificationRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentJustificationRef: %v", err)
	}
	return value
}

func mustSupportProvenanceRefV1(
	t testing.TB,
	raw string,
) projectprofile.RoleAssignmentProvenanceRef {
	t.Helper()
	value, err := projectprofile.NewRoleAssignmentProvenanceRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentProvenanceRef: %v", err)
	}
	return value
}

func mustSupportRoleAssignmentRefV1(
	t testing.TB,
	raw string,
) projectprofile.RoleAssignmentRef {
	t.Helper()
	value, err := projectprofile.NewRoleAssignmentRef(raw)
	if err != nil {
		t.Fatalf("NewRoleAssignmentRef: %v", err)
	}
	return value
}
