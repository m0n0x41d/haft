package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectProfileAdmissionSupportIdentitySQL = `WITH selected AS (
	SELECT 'v1' AS storage_generation,
		a.admission_id, COALESCE(a.admission_digest, '') AS admission_digest,
		COALESCE(a.receipt_digest, '') AS receipt_digest,
		COALESCE(a.authority_resolution_ref, '') AS authority_resolution_ref,
		COALESCE(a.single_use_key, '') AS single_use_key,
		a.project_root, COALESCE(a.ledger_revision, 0) AS ledger_revision,
		a.profile_author_role_assignment_ref,
		a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref,
		a.observed_project_basis_digest,
		a.work_record_ref,
		a.work_record_digest,
		a.outcome_assessment_ref,
		a.outcome_assessment_digest
	FROM project_profile_admissions a
	WHERE a.admission_id = ?
	UNION ALL
	SELECT 'v2' AS storage_generation,
		a.admission_id, a.admission_digest,
		a.receipt_digest, a.authority_resolution_ref, a.single_use_key,
		a.project_root, a.ledger_revision,
		a.profile_author_role_assignment_ref,
		a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref,
		a.observed_project_basis_digest,
		a.work_record_ref,
		a.work_record_digest,
		a.outcome_assessment_ref,
		a.outcome_assessment_digest
	FROM project_profile_admissions_v2 a
	WHERE a.admission_id = ?
), all_admissions AS (
	SELECT 'v1' AS storage_generation,
		admission_id, COALESCE(admission_digest, '') AS admission_digest,
		COALESCE(receipt_digest, '') AS receipt_digest,
		COALESCE(authority_resolution_ref, '') AS authority_resolution_ref,
		COALESCE(single_use_key, '') AS single_use_key,
		project_root, COALESCE(ledger_revision, 0) AS ledger_revision
	FROM project_profile_admissions
	UNION ALL
	SELECT 'v2' AS storage_generation,
		admission_id, admission_digest, receipt_digest,
		authority_resolution_ref, single_use_key, project_root, ledger_revision
	FROM project_profile_admissions_v2
)
SELECT selected.storage_generation,
	selected.project_root,
	selected.profile_author_role_assignment_ref,
	selected.profile_author_role_assignment_digest,
	selected.observed_project_basis_ref,
	selected.observed_project_basis_digest,
	selected.work_record_ref,
	selected.work_record_digest,
	selected.outcome_assessment_ref,
	selected.outcome_assessment_digest,
	(SELECT COUNT(*) FROM selected),
	(
		SELECT COUNT(*) FROM all_admissions other
		WHERE other.storage_generation != selected.storage_generation
		AND (
			other.admission_id = selected.admission_id
			OR (selected.admission_digest != '' AND other.admission_digest = selected.admission_digest)
			OR (selected.receipt_digest != '' AND other.receipt_digest = selected.receipt_digest)
			OR (selected.authority_resolution_ref != '' AND other.authority_resolution_ref = selected.authority_resolution_ref)
			OR (selected.single_use_key != '' AND other.single_use_key = selected.single_use_key)
			OR (
				other.project_root = selected.project_root
				AND other.ledger_revision = selected.ledger_revision
				AND selected.ledger_revision > 0
			)
		)
	)
FROM selected
ORDER BY selected.storage_generation
LIMIT 1`

type profileAdmissionStorageGeneration string

const (
	profileAdmissionStorageV1 profileAdmissionStorageGeneration = "v1"
	profileAdmissionStorageV2 profileAdmissionStorageGeneration = "v2"
)

type profileAdmissionSupportIdentityRow struct {
	storageGeneration         profileAdmissionStorageGeneration
	projectRoot               string
	assignmentRef             string
	assignmentDigest          string
	basisRef                  string
	basisDigest               string
	workRef                   string
	workDigest                string
	assessmentRef             string
	assessmentDigest          string
	matchCount                int
	crossGenerationCollisions int
}

func (row *profileAdmissionSupportIdentityRow) scanTargets() []any {
	return []any{
		&row.storageGeneration,
		&row.projectRoot,
		&row.assignmentRef,
		&row.assignmentDigest,
		&row.basisRef,
		&row.basisDigest,
		&row.workRef,
		&row.workDigest,
		&row.assessmentRef,
		&row.assessmentDigest,
		&row.matchCount,
		&row.crossGenerationCollisions,
	}
}

// ResolveProfileAdmissionValueSnapshotByAdmissionRefV1 follows only the exact
// support refs and digests stored on one admission row. It reconstructs the
// structural Work DAG but does not prove authority use, admission canonical
// JSON, revision-head integrity, or COMMIT history.
func ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef projectprofile.ProfileDeclarationAdmissionRecordRef,
) (ProfileAdmissionValueSnapshotV1, error) {
	if ctx == nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf(
			"profile-admission support context is required",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf(
			"profile-admission support transaction is invalid: %w",
			err,
		)
	}
	canonicalAdmissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		admissionRef.String(),
	)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf(
			"profile-admission support ref is invalid: %w",
			err,
		)
	}
	row, err := loadProfileAdmissionSupportIdentity(
		ctx,
		transaction,
		canonicalAdmissionRef,
	)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, err
	}
	identity, err := parseProfileAdmissionSupportIdentity(row)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, err
	}
	durable, err := ResolveProfileOnboardingValueSetV1(
		ctx,
		transaction,
		identity,
	)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, err
	}
	values, ok := durable.Values()
	if !ok {
		return ProfileAdmissionValueSnapshotV1{}, fmt.Errorf(
			"durable profile-admission support is unusable",
		)
	}
	err = validateProfileAdmissionSupportSelection(row, values)
	if err != nil {
		return ProfileAdmissionValueSnapshotV1{}, err
	}
	return sealProfileAdmissionValueSnapshotV1(values)
}

func loadProfileAdmissionSupportIdentity(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef projectprofile.ProfileDeclarationAdmissionRecordRef,
) (profileAdmissionSupportIdentityRow, error) {
	row := profileAdmissionSupportIdentityRow{}
	arguments := []any{
		admissionRef.String(),
		admissionRef.String(),
	}
	destinations := row.scanTargets()
	err := transaction.ScanOne(
		ctx,
		selectProfileAdmissionSupportIdentitySQL,
		arguments,
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return profileAdmissionSupportIdentityRow{}, fmt.Errorf(
			"profile-admission support for %q: %w",
			admissionRef.String(),
			sql.ErrNoRows,
		)
	}
	if err != nil {
		return profileAdmissionSupportIdentityRow{}, fmt.Errorf(
			"load profile-admission support identity: %w",
			err,
		)
	}
	if row.matchCount != 1 {
		return profileAdmissionSupportIdentityRow{}, fmt.Errorf(
			"profile-admission ref matches %d support selectors; expected exactly one",
			row.matchCount,
		)
	}
	if row.crossGenerationCollisions != 0 {
		return profileAdmissionSupportIdentityRow{}, fmt.Errorf(
			"profile-admission ref has %d cross-generation identity collision(s)",
			row.crossGenerationCollisions,
		)
	}
	if row.storageGeneration != profileAdmissionStorageV1 &&
		row.storageGeneration != profileAdmissionStorageV2 {
		return profileAdmissionSupportIdentityRow{}, fmt.Errorf(
			"profile-admission support has unknown storage generation %q",
			row.storageGeneration,
		)
	}
	return row, nil
}

func parseProfileAdmissionSupportIdentity(
	row profileAdmissionSupportIdentityRow,
) (ProfileOnboardingValueIdentityV1, error) {
	projectRoot, err := projectprofile.NewProjectRootV1(row.projectRoot)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf(
			"parse admission support project root: %w",
			err,
		)
	}
	workRef, err := projectprofile.NewProfileOnboardingWorkRecordRef(row.workRef)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf(
			"parse admission support Work ref: %w",
			err,
		)
	}
	workDigest, err := projectprofile.NewContentDigest(row.workDigest)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf(
			"parse admission support Work digest: %w",
			err,
		)
	}
	assessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(
		row.assessmentRef,
	)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf(
			"parse admission support assessment ref: %w",
			err,
		)
	}
	assessmentDigest, err := projectprofile.NewContentDigest(row.assessmentDigest)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf(
			"parse admission support assessment digest: %w",
			err,
		)
	}
	builder := NewProfileOnboardingValueIdentityV1Builder(projectRoot)
	builder = builder.WithWork(workRef, workDigest)
	builder = builder.WithAssessment(assessmentRef, assessmentDigest)
	identity, err := builder.Build()
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf(
			"build admission support identity: %w",
			err,
		)
	}
	return identity, nil
}

func validateProfileAdmissionSupportSelection(
	row profileAdmissionSupportIdentityRow,
	values ProfileOnboardingValueSetV1,
) error {
	work := values.WorkRecord()
	workDigest, err := projectprofile.DigestProfileOnboardingWorkRecord(work)
	if err != nil {
		return fmt.Errorf("redigest selected profile-onboarding Work: %w", err)
	}
	assignment := values.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return fmt.Errorf("redigest selected ProfileAuthor RoleAssignment: %w", err)
	}
	basis := values.ObservedBasis()
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(basis)
	if err != nil {
		return fmt.Errorf("redigest selected ObservedProjectBasis: %w", err)
	}
	assessment := values.Assessment()
	assessmentDigest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(assessment)
	if err != nil {
		return fmt.Errorf("redigest selected profile-onboarding outcome assessment: %w", err)
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: basis.ProjectRoot().String() == row.projectRoot, name: "project root"},
		{matches: assignment.RoleAssignmentRef().String() == row.assignmentRef, name: "ProfileAuthor RoleAssignment ref"},
		{matches: assignmentDigest.String() == row.assignmentDigest, name: "ProfileAuthor RoleAssignment digest"},
		{matches: basis.Ref().String() == row.basisRef, name: "ObservedProjectBasis ref"},
		{matches: basisDigest.String() == row.basisDigest, name: "ObservedProjectBasis digest"},
		{matches: work.RecordRef().String() == row.workRef, name: "Work-record ref"},
		{matches: workDigest.String() == row.workDigest, name: "Work-record digest"},
		{matches: assessment.Ref().String() == row.assessmentRef, name: "outcome-assessment ref"},
		{matches: assessmentDigest.String() == row.assessmentDigest, name: "outcome-assessment digest"},
	}
	return validateProfileAdmissionSupportChecks(checks, 0)
}

func validateProfileAdmissionSupportChecks(
	checks []struct {
		matches bool
		name    string
	},
	index int,
) error {
	if index == len(checks) {
		return nil
	}
	check := checks[index]
	if !check.matches {
		return fmt.Errorf("profile-admission row has a mismatched %s", check.name)
	}
	return validateProfileAdmissionSupportChecks(checks, index+1)
}
