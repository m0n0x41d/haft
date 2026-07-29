package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestResolveProfileAdmissionValueSnapshotByAdmissionRefV1ExactAndReadOnly(
	t *testing.T,
) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "admission-ref")
	storeCommittedFixtureV1(t, database.raw, fixture)
	replaceAdmissionTableForSupportSelectionV1(t, database.raw)
	admissionRef := insertAdmissionSupportSelectorRowV1(
		t,
		database.raw,
		fixture,
		"profile-admission.selector.exact",
	)
	before := admissionSupportSelectionCountsV1(t, database.raw)
	transaction := beginReadV1(t, database.raw)
	snapshot, err := ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
		context.Background(),
		transaction,
		admissionRef,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("resolve profile-admission support: %v", err)
	}
	values, ok := snapshot.Values()
	if !ok {
		_ = transaction.Rollback(context.Background())
		t.Fatal("resolved support snapshot did not expose values")
	}
	if values.WorkRecord().RecordRef() != fixture.values.WorkRecord().RecordRef() {
		_ = transaction.Rollback(context.Background())
		t.Fatal("resolved support snapshot selected another Work record")
	}
	rollbackTransactionV1(t, transaction)
	after := admissionSupportSelectionCountsV1(t, database.raw)
	if before != after {
		t.Fatalf("support selection mutated storage: before=%v after=%v", before, after)
	}
}

func TestResolveProfileAdmissionValueSnapshotByAdmissionRefV1SelectsExactV2Source(
	t *testing.T,
) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "admission-v2")
	storeCommittedFixtureV1(t, database.raw, fixture)
	replaceAdmissionV2TableForSupportSelectionV1(t, database.raw)
	admissionRef := insertAdmissionSupportSelectorRowV2(
		t,
		database.raw,
		fixture,
		"profile-admission.selector.v2",
	)

	transaction := beginReadV1(t, database.raw)
	snapshot, err := ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
		context.Background(),
		transaction,
		admissionRef,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("resolve v2 profile-admission support: %v", err)
	}
	values, ok := snapshot.Values()
	if !ok || values.WorkRecord().RecordRef() != fixture.values.WorkRecord().RecordRef() {
		_ = transaction.Rollback(context.Background())
		t.Fatal("v2 selector did not recover the exact common Work support")
	}
	rollbackTransactionV1(t, transaction)
}

func TestResolveProfileAdmissionValueSnapshotByAdmissionRefV1RejectsCrossGenerationIdentityCollision(
	t *testing.T,
) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "cross-generation")
	storeCommittedFixtureV1(t, database.raw, fixture)
	replaceAdmissionTableForSupportSelectionV1(t, database.raw)
	replaceAdmissionV2TableForSupportSelectionV1(t, database.raw)
	insertAdmissionSupportSelectorRowV1(
		t,
		database.raw,
		fixture,
		"profile-admission.selector.legacy",
	)
	v2Ref := insertAdmissionSupportSelectorRowV2(
		t,
		database.raw,
		fixture,
		"profile-admission.selector.current",
	)

	transaction := beginReadV1(t, database.raw)
	_, err := ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
		context.Background(),
		transaction,
		v2Ref,
	)
	rollbackTransactionV1(t, transaction)
	if err == nil || !strings.Contains(err.Error(), "cross-generation identity collision") {
		t.Fatalf("cross-generation profile-admission collision = %v", err)
	}
}

func TestResolveProfileAdmissionValueSnapshotByAdmissionRefV1RejectsMissingAndDuplicate(
	t *testing.T,
) {
	t.Run("missing", func(t *testing.T) {
		directory := t.TempDir()
		database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
		t.Cleanup(func() { _ = database.kernel.Close() })
		replaceAdmissionTableForSupportSelectionV1(t, database.raw)
		admissionRef := mustParsedV1(
			t,
			"profile-admission.selector.missing",
			projectprofile.NewProfileDeclarationAdmissionRecordRef,
		)
		transaction := beginReadV1(t, database.raw)
		_, err := ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
			context.Background(),
			transaction,
			admissionRef,
		)
		rollbackTransactionV1(t, transaction)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("missing support selector = %v, want sql.ErrNoRows", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		directory := t.TempDir()
		database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
		t.Cleanup(func() { _ = database.kernel.Close() })
		fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "duplicate")
		storeCommittedFixtureV1(t, database.raw, fixture)
		replaceAdmissionTableForSupportSelectionV1(t, database.raw)
		admissionRef := insertAdmissionSupportSelectorRowV1(
			t,
			database.raw,
			fixture,
			"profile-admission.selector.duplicate",
		)
		insertAdmissionSupportSelectorRowV1(
			t,
			database.raw,
			fixture,
			admissionRef.String(),
		)
		transaction := beginReadV1(t, database.raw)
		_, err := ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
			context.Background(),
			transaction,
			admissionRef,
		)
		rollbackTransactionV1(t, transaction)
		if err == nil || !strings.Contains(err.Error(), "matches 2 support selectors") {
			t.Fatalf("duplicate support selector = %v", err)
		}
	})
}

func TestResolveProfileAdmissionValueSnapshotByAdmissionRefV1RejectsCorruption(
	t *testing.T,
) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "corrupt")
	storeCommittedFixtureV1(t, database.raw, fixture)
	replaceAdmissionTableForSupportSelectionV1(t, database.raw)
	admissionRef := insertAdmissionSupportSelectorRowV1(
		t,
		database.raw,
		fixture,
		"profile-admission.selector.corrupt",
	)
	foreignDigest := testDigestV1(t, "foreign assignment")
	arguments := []any{foreignDigest.String(), admissionRef.String()}
	_, err := database.raw.Exec(
		`UPDATE project_profile_admissions
		 SET profile_author_role_assignment_digest = ?
		 WHERE admission_id = ?`,
		arguments...,
	)
	if err != nil {
		t.Fatalf("seed corrupt support selector: %v", err)
	}
	transaction := beginReadV1(t, database.raw)
	_, err = ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
		context.Background(),
		transaction,
		admissionRef,
	)
	rollbackTransactionV1(t, transaction)
	if err == nil || !strings.Contains(err.Error(), "mismatched ProfileAuthor RoleAssignment digest") {
		t.Fatalf("corrupt support selector = %v", err)
	}
}

func TestResolveProfileAdmissionValueSnapshotByAdmissionRefV1TransactionLifecycle(
	t *testing.T,
) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	admissionRef := mustParsedV1(
		t,
		"profile-admission.selector.lifecycle",
		projectprofile.NewProfileDeclarationAdmissionRecordRef,
	)
	_, err := ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
		context.Background(),
		&sqlitetransaction.Transaction{},
		admissionRef,
	)
	if !errors.Is(err, sqlitetransaction.ErrTransactionInvalid) {
		t.Fatalf("zero transaction = %v, want transaction-invalid", err)
	}
	finished := beginReadV1(t, database.raw)
	rollbackTransactionV1(t, finished)
	_, err = ResolveProfileAdmissionValueSnapshotByAdmissionRefV1(
		context.Background(),
		finished,
		admissionRef,
	)
	if !errors.Is(err, sqlitetransaction.ErrTransactionFinished) {
		t.Fatalf("finished transaction = %v, want transaction-finished", err)
	}
	var zero ProfileAdmissionValueSnapshotV1
	if _, ok := zero.Values(); ok {
		t.Fatal("zero support snapshot exposed values")
	}
}

func replaceAdmissionTableForSupportSelectionV1(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	statements := []string{
		"ALTER TABLE project_profile_admissions RENAME TO project_profile_admissions_guarded",
		"CREATE TABLE project_profile_admissions AS SELECT * FROM project_profile_admissions_guarded WHERE 0",
	}
	execAdmissionSupportStatementsV1(t, database, statements, 0)
}

func replaceAdmissionV2TableForSupportSelectionV1(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	statements := []string{
		"ALTER TABLE project_profile_admissions_v2 RENAME TO project_profile_admissions_v2_guarded",
		"CREATE TABLE project_profile_admissions_v2 AS SELECT * FROM project_profile_admissions_v2_guarded WHERE 0",
	}
	execAdmissionSupportStatementsV1(t, database, statements, 0)
}

func execAdmissionSupportStatementsV1(
	t *testing.T,
	database *sql.DB,
	statements []string,
	index int,
) {
	t.Helper()
	if index == len(statements) {
		return
	}
	_, err := database.Exec(statements[index])
	if err != nil {
		t.Fatalf("prepare support-selector table %q: %v", statements[index], err)
	}
	execAdmissionSupportStatementsV1(t, database, statements, index+1)
}

func insertAdmissionSupportSelectorRowV1(
	t *testing.T,
	database *sql.DB,
	fixture valueStoreFixtureV1,
	admissionID string,
) projectprofile.ProfileDeclarationAdmissionRecordRef {
	return insertAdmissionSupportSelectorRowForTableV1(
		t,
		database,
		fixture,
		admissionID,
		"project_profile_admissions",
	)
}

func insertAdmissionSupportSelectorRowV2(
	t *testing.T,
	database *sql.DB,
	fixture valueStoreFixtureV1,
	admissionID string,
) projectprofile.ProfileDeclarationAdmissionRecordRef {
	return insertAdmissionSupportSelectorRowForTableV1(
		t,
		database,
		fixture,
		admissionID,
		"project_profile_admissions_v2",
	)
}

func insertAdmissionSupportSelectorRowForTableV1(
	t *testing.T,
	database *sql.DB,
	fixture valueStoreFixtureV1,
	admissionID string,
	table string,
) projectprofile.ProfileDeclarationAdmissionRecordRef {
	t.Helper()
	values := fixture.values
	assignment := values.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		t.Fatalf("digest selector RoleAssignment: %v", err)
	}
	basis := values.ObservedBasis()
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("digest selector ObservedProjectBasis: %v", err)
	}
	work := values.WorkRecord()
	workDigest := mustWorkDigestV1(t, work)
	assessment := values.Assessment()
	assessmentDigest := mustAssessmentDigestV1(t, assessment)
	arguments := []any{
		admissionID,
		testDigestV1(t, "admission:"+admissionID).String(),
		testDigestV1(t, "receipt:"+admissionID).String(),
		"authority-resolution:" + admissionID,
		"single-use:" + admissionID,
		fixture.root.String(),
		1,
		assignment.RoleAssignmentRef().String(),
		assignmentDigest.String(),
		basis.Ref().String(),
		basisDigest.String(),
		work.RecordRef().String(),
		workDigest.String(),
		assessment.Ref().String(),
		assessmentDigest.String(),
	}
	statement := `INSERT INTO project_profile_admissions (
			admission_id,
			admission_digest,
			receipt_digest,
			authority_resolution_ref,
			single_use_key,
			project_root,
			ledger_revision,
			profile_author_role_assignment_ref,
			profile_author_role_assignment_digest,
			observed_project_basis_ref,
			observed_project_basis_digest,
			work_record_ref,
			work_record_digest,
			outcome_assessment_ref,
			outcome_assessment_digest
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if table == "project_profile_admissions_v2" {
		statement = strings.Replace(statement, "project_profile_admissions (", "project_profile_admissions_v2 (", 1)
	}
	_, err = database.Exec(statement, arguments...)
	if err != nil {
		t.Fatalf("insert support-selector row: %v", err)
	}
	return mustParsedV1(
		t,
		admissionID,
		projectprofile.NewProfileDeclarationAdmissionRecordRef,
	)
}

type admissionSupportSelectionCounts struct {
	admissions  int
	assignments int
	bases       int
	work        int
	assessments int
}

func admissionSupportSelectionCountsV1(
	t *testing.T,
	database *sql.DB,
) admissionSupportSelectionCounts {
	t.Helper()
	return admissionSupportSelectionCounts{
		admissions:  countRowsV1(t, database, "project_profile_admissions"),
		assignments: countRowsV1(t, database, "profile_author_role_assignments"),
		bases:       countRowsV1(t, database, "observed_project_bases"),
		work:        countRowsV1(t, database, "profile_onboarding_work_records"),
		assessments: countRowsV1(t, database, "profile_onboarding_outcome_assessments"),
	}
}
