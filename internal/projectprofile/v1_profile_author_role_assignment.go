package projectprofile

import "fmt"

const profileAuthorRoleAssignmentDigestDomainV1 = "haft.project-profile.profile-author-role-assignment/v1"

// The support references below keep the relation's admission, justification,
// and provenance claims distinct. A reference names a neighboring relation or
// record; its paired digest binds the exact relied-upon content.
type SystemAdmissionRef struct{ v1Reference }
type RoleAdmissionRef struct{ v1Reference }
type RoleAssignmentJustificationRef struct{ v1Reference }
type RoleAssignmentProvenanceRef struct{ v1Reference }

func NewSystemAdmissionRef(raw string) (SystemAdmissionRef, error) {
	ref, err := newV1Reference("system-admission ref", raw)
	return SystemAdmissionRef{v1Reference: ref}, err
}

func NewRoleAdmissionRef(raw string) (RoleAdmissionRef, error) {
	ref, err := newV1Reference("role-admission ref", raw)
	return RoleAdmissionRef{v1Reference: ref}, err
}

func NewRoleAssignmentJustificationRef(raw string) (RoleAssignmentJustificationRef, error) {
	ref, err := newV1Reference("RoleAssignment justification ref", raw)
	return RoleAssignmentJustificationRef{v1Reference: ref}, err
}

func NewRoleAssignmentProvenanceRef(raw string) (RoleAssignmentProvenanceRef, error) {
	ref, err := newV1Reference("RoleAssignment provenance ref", raw)
	return RoleAssignmentProvenanceRef{v1Reference: ref}, err
}

// ProfileAuthorRoleAssignmentV1 is the concrete U.RoleAssignment relation
// relied upon by final-v1 profile-onboarding Work. It is not a role, a system
// admission, permission, capability, method, or performed Work. The exact
// neighboring admission, justification, and provenance material stays
// separately addressable through strong ref+digest pairs. A.2.1 remains
// open-world for ordinary recognition; this local final-v1 carrier requires
// those pairs because durable profile admission relies on the assignment.
type ProfileAuthorRoleAssignmentV1 struct {
	roleAssignmentRef     RoleAssignmentRef
	holderSystemRef       SystemRef
	admittedRoleRef       RoleRef
	boundedContextRef     BoundedContextRef
	validityWindow        RoleAssignmentWindowV1
	systemAdmissionRef    SystemAdmissionRef
	systemAdmissionDigest ContentDigest
	roleAdmissionRef      RoleAdmissionRef
	roleAdmissionDigest   ContentDigest
	justificationRef      RoleAssignmentJustificationRef
	justificationDigest   ContentDigest
	provenanceRef         RoleAssignmentProvenanceRef
	provenanceDigest      ContentDigest
}

type ProfileAuthorRoleAssignmentV1Builder struct {
	value ProfileAuthorRoleAssignmentV1
}

func NewProfileAuthorRoleAssignmentV1Builder(
	ref RoleAssignmentRef,
) ProfileAuthorRoleAssignmentV1Builder {
	return ProfileAuthorRoleAssignmentV1Builder{
		value: ProfileAuthorRoleAssignmentV1{roleAssignmentRef: ref},
	}
}

func (builder ProfileAuthorRoleAssignmentV1Builder) HeldBy(
	holder SystemRef,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.holderSystemRef = holder
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) Assigning(
	role RoleRef,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.admittedRoleRef = role
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) InContext(
	context BoundedContextRef,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.boundedContextRef = context
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) ValidDuring(
	window RoleAssignmentWindowV1,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.validityWindow = window
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) WithSystemAdmission(
	ref SystemAdmissionRef,
	digest ContentDigest,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.systemAdmissionRef = ref
	builder.value.systemAdmissionDigest = digest
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) WithRoleAdmission(
	ref RoleAdmissionRef,
	digest ContentDigest,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.roleAdmissionRef = ref
	builder.value.roleAdmissionDigest = digest
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) JustifiedBy(
	ref RoleAssignmentJustificationRef,
	digest ContentDigest,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.justificationRef = ref
	builder.value.justificationDigest = digest
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) WithProvenance(
	ref RoleAssignmentProvenanceRef,
	digest ContentDigest,
) ProfileAuthorRoleAssignmentV1Builder {
	builder.value.provenanceRef = ref
	builder.value.provenanceDigest = digest
	return builder
}

func (builder ProfileAuthorRoleAssignmentV1Builder) Build() (ProfileAuthorRoleAssignmentV1, error) {
	return canonicalProfileAuthorRoleAssignmentV1(builder.value)
}

func (assignment ProfileAuthorRoleAssignmentV1) RoleAssignmentRef() RoleAssignmentRef {
	return assignment.roleAssignmentRef
}

func (assignment ProfileAuthorRoleAssignmentV1) HolderSystemRef() SystemRef {
	return assignment.holderSystemRef
}

func (assignment ProfileAuthorRoleAssignmentV1) AdmittedRoleRef() RoleRef {
	return assignment.admittedRoleRef
}

func (assignment ProfileAuthorRoleAssignmentV1) BoundedContextRef() BoundedContextRef {
	return assignment.boundedContextRef
}

func (assignment ProfileAuthorRoleAssignmentV1) ValidityWindow() RoleAssignmentWindowV1 {
	return assignment.validityWindow
}

func (assignment ProfileAuthorRoleAssignmentV1) SystemAdmissionRef() SystemAdmissionRef {
	return assignment.systemAdmissionRef
}

func (assignment ProfileAuthorRoleAssignmentV1) SystemAdmissionDigest() ContentDigest {
	return assignment.systemAdmissionDigest
}

func (assignment ProfileAuthorRoleAssignmentV1) RoleAdmissionRef() RoleAdmissionRef {
	return assignment.roleAdmissionRef
}

func (assignment ProfileAuthorRoleAssignmentV1) RoleAdmissionDigest() ContentDigest {
	return assignment.roleAdmissionDigest
}

func (assignment ProfileAuthorRoleAssignmentV1) JustificationRef() RoleAssignmentJustificationRef {
	return assignment.justificationRef
}

func (assignment ProfileAuthorRoleAssignmentV1) JustificationDigest() ContentDigest {
	return assignment.justificationDigest
}

func (assignment ProfileAuthorRoleAssignmentV1) ProvenanceRef() RoleAssignmentProvenanceRef {
	return assignment.provenanceRef
}

func (assignment ProfileAuthorRoleAssignmentV1) ProvenanceDigest() ContentDigest {
	return assignment.provenanceDigest
}

// ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1
// checks only the direct A.2.1/A.15.1 relation between an already admitted
// assignment and a Work occurrence. Resolution and semantic validation of the
// referenced system/role admissions, justification, and provenance remain an
// effect-boundary responsibility.
func ValidateProfileOnboardingWorkRecordAgainstProfileAuthorRoleAssignmentV1(
	record ProfileOnboardingWorkRecord,
	assignment ProfileAuthorRoleAssignmentV1,
) error {
	work, err := canonicalizeProfileOnboardingWorkRecord(record)
	if err != nil {
		return err
	}
	relation, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return err
	}
	checks := []profileAuthorRoleAssignmentCheckV1{
		{valid: relation.roleAssignmentRef == work.performedBy, reason: "assignment ref must equal Work.performedBy"},
		{valid: relation.holderSystemRef == work.executedWithin, reason: "assignment holder must equal Work.executedWithin"},
		{valid: relation.boundedContextRef == work.boundedContextRef, reason: "assignment and Work contexts must match"},
		{valid: relation.validityWindow.covers(work.workInterval), reason: "assignment window must cover the Work interval"},
	}
	return visitSliceV1(checks, validateProfileAuthorRoleAssignmentCheckV1)
}

func DigestProfileAuthorRoleAssignmentV1(
	assignment ProfileAuthorRoleAssignmentV1,
) (ContentDigest, error) {
	canonical, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return ContentDigest{}, err
	}
	data, err := EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(canonical)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(profileAuthorRoleAssignmentDigestDomainV1)
	writer.add(string(data))
	return writer.digest(), nil
}

func canonicalProfileAuthorRoleAssignmentV1(
	assignment ProfileAuthorRoleAssignmentV1,
) (ProfileAuthorRoleAssignmentV1, error) {
	checks := []profileAuthorRoleAssignmentCheckV1{
		{valid: assignment.roleAssignmentRef.valid(), reason: "RoleAssignment ref is required"},
		{valid: assignment.holderSystemRef.valid(), reason: "holder U.System ref is required"},
		{valid: assignment.admittedRoleRef == ProfileAuthorRoleRefV1(), reason: "assignment must assign ProfileAuthorRole v1"},
		{valid: assignment.boundedContextRef.valid(), reason: "bounded-context ref is required"},
		{valid: assignment.validityWindow.valid(), reason: "assignment validity window is required"},
		{valid: assignment.systemAdmissionRef.valid(), reason: "system-admission ref is required"},
		{valid: assignment.systemAdmissionDigest.valid(), reason: "system-admission digest is required"},
		{valid: assignment.roleAdmissionRef.valid(), reason: "role-admission ref is required"},
		{valid: assignment.roleAdmissionDigest.valid(), reason: "role-admission digest is required"},
		{valid: assignment.justificationRef.valid(), reason: "assignment-justification ref is required"},
		{valid: assignment.justificationDigest.valid(), reason: "assignment-justification digest is required"},
		{valid: assignment.provenanceRef.valid(), reason: "assignment-provenance ref is required"},
		{valid: assignment.provenanceDigest.valid(), reason: "assignment-provenance digest is required"},
	}
	err := visitSliceV1(checks, validateProfileAuthorRoleAssignmentCheckV1)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	return assignment, nil
}

type profileAuthorRoleAssignmentCheckV1 struct {
	valid  bool
	reason string
}

func validateProfileAuthorRoleAssignmentCheckV1(
	_ int,
	check profileAuthorRoleAssignmentCheckV1,
) error {
	if !check.valid {
		return fmt.Errorf("ProfileAuthorRoleAssignmentV1 is invalid: %s", check.reason)
	}
	return nil
}
