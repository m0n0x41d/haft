package sqlite

import (
	"context"
	"database/sql"
	"testing"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestServiceMintsCanonicalTokenForFreshAndReplay(t *testing.T) {
	fixture := newTransactionFixture(t, "service-fresh-replay", "service-fresh-replay.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	freshResult := service.Admit(context.Background(), fixture.request)
	fresh := requireCanonicalAdmission(t, freshResult, CanonicalAdmissionFresh)
	if !fresh.Valid() {
		t.Fatal("fresh canonical admission is invalid")
	}
	if fresh.ProjectRoot() != fixture.root {
		t.Fatalf("fresh project root = %q, want %q", fresh.ProjectRoot().String(), fixture.root.String())
	}
	assertCanonicalTokenBindings(t, fresh)
	replayResult := service.Admit(context.Background(), fixture.request)
	replay := requireCanonicalAdmission(t, replayResult, CanonicalAdmissionReplayed)
	if replay.AdmissionRecordRef() != fresh.AdmissionRecordRef() {
		t.Fatal("replay returned another admission ref")
	}
	if replay.AdmissionRecordDigest() != fresh.AdmissionRecordDigest() {
		t.Fatal("replay returned another admission digest")
	}
	if replay.LedgerRevision() != fresh.LedgerRevision() {
		t.Fatal("replay returned another ledger revision")
	}
	assertCanonicalMutationCounts(t, fixture.database, 1)
}

func TestServiceMintsRecoveredTokenOnlyAfterRequestFreeReread(t *testing.T) {
	fixture := newTransactionFixture(t, "service-recovered", "service-recovered.nonce")
	finisher := &ambiguousCommitFinisher{}
	fixture.adapter.finisher = finisher
	service := Service{adapter: fixture.adapter}
	result := service.Admit(context.Background(), fixture.request)
	recovered := requireCanonicalAdmission(t, result, CanonicalAdmissionRecovered)
	if !recovered.Valid() {
		t.Fatal("recovered canonical admission is invalid")
	}
	assertCanonicalMutationCounts(t, fixture.database, 1)
}

func TestServiceResolveCurrentSurvivesRestartWithoutAuthorityRequest(t *testing.T) {
	fixture := newTransactionFixture(t, "service-restart", "service-restart.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	admitted := requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	databasePath := databasePath(t, fixture.database)
	err = fixture.store.Close()
	if err != nil {
		t.Fatalf("close first store: %v", err)
	}
	restarted, err := kerneldb.NewStore(databasePath)
	if err != nil {
		t.Fatalf("restart NewStore: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	assertRestartedV2ProfileWorkSupport(
		t,
		restarted.GetRawDB(),
		fixture.request.Candidate(),
	)
	restartedService, err := NewService(restarted.GetRawDB())
	if err != nil {
		t.Fatalf("restart NewService: %v", err)
	}
	resolved := requireCanonicalAdmission(
		t,
		restartedService.ResolveCurrent(context.Background(), fixture.root),
		CanonicalAdmissionResolvedAfterRestart,
	)
	if resolved.AdmissionRecordDigest() != admitted.AdmissionRecordDigest() {
		t.Fatal("restart resolver returned another admission digest")
	}
	if resolved.WorkRecordRef() != admitted.WorkRecordRef() {
		t.Fatal("restart resolver returned another Work-record ref")
	}
	if resolved.AuthorityBasisRef() != admitted.AuthorityBasisRef() {
		t.Fatal("restart resolver returned another authority-basis ref")
	}
}

func assertRestartedV2ProfileWorkSupport(
	t testing.TB,
	database *sql.DB,
	candidate projectprofile.ProfileDeclarationCandidateV1,
) {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(context.Background(), database)
	if err != nil {
		t.Fatalf("begin restarted value-snapshot read: %v", err)
	}
	snapshot, err := projectprofilesqlite.ResolveProfileAdmissionValueSnapshotV1(
		context.Background(),
		transaction,
		candidate,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("resolve restarted profile values: %v", err)
	}
	values, ok := snapshot.Values()
	if !ok {
		_ = transaction.Rollback(context.Background())
		t.Fatal("restarted profile values are unusable")
	}
	_, descriptionV2 := values.MethodDescriptionV2()
	_, contractV2 := values.MethodContractV2()
	_, workInputV2 := values.WorkRecord().ProfileOnboardingWorkInputRefV2()
	finish := transaction.Rollback(context.Background())
	if !finish.Succeeded() {
		t.Fatalf("close restarted value-snapshot read: %v", finish.Err())
	}
	if !descriptionV2 || !contractV2 || !workInputV2 {
		t.Fatalf(
			"restarted Work support editions: description_v2=%t contract_v2=%t work_input_v2=%t",
			descriptionV2,
			contractV2,
			workInputV2,
		)
	}
}

func TestServiceResolveCurrentFailsClosedOnDurableSupportCorruption(t *testing.T) {
	fixture := newTransactionFixture(t, "service-corrupt", "service-corrupt.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	_, err = fixture.database.Exec("DROP TRIGGER profile_onboarding_work_records_no_update")
	if err != nil {
		t.Fatalf("drop Work update guard: %v", err)
	}
	_, err = fixture.database.Exec(
		"UPDATE profile_onboarding_work_records SET work_record_json = '{}'",
	)
	if err != nil {
		t.Fatalf("corrupt durable Work: %v", err)
	}
	result := service.ResolveCurrent(context.Background(), fixture.root)
	if result.Kind() != AdmissionResultWriteFailed {
		t.Fatalf("ResolveCurrent kind = %q, want write_failed", result.Kind())
	}
	if _, ok := result.Admission(); ok {
		t.Fatal("corrupted durable support minted a canonical admission")
	}
}

func TestServiceResolveCurrentFailsClosedOnCurrentAuthorityCorruption(t *testing.T) {
	fixture := newTransactionFixture(t, "service-authority-corrupt", "service-authority-corrupt.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	requireCanonicalAdmission(
		t,
		service.Admit(context.Background(), fixture.request),
		CanonicalAdmissionFresh,
	)
	_, err = fixture.database.Exec(
		"DROP TRIGGER profile_declaration_authority_resolutions_v3_no_update",
	)
	if err != nil {
		t.Fatalf("drop authority-resolution update guard: %v", err)
	}
	corruption, err := fixture.database.Exec(
		`UPDATE profile_declaration_authority_resolutions_v3
		 SET verifier_version = 'v-corrupt',
		     canonical_json = json_set(canonical_json, '$.verifier_version', 'v-corrupt')`,
	)
	if err != nil {
		t.Fatalf("corrupt durable authority resolution: %v", err)
	}
	affected, err := corruption.RowsAffected()
	if err != nil {
		t.Fatalf("count corrupted authority resolutions: %v", err)
	}
	if affected != 1 {
		t.Fatalf("corrupted authority resolutions = %d, want one", affected)
	}
	result := service.ResolveCurrent(context.Background(), fixture.root)
	if result.Kind() != AdmissionResultWriteFailed {
		t.Fatalf("ResolveCurrent kind = %q, want write_failed", result.Kind())
	}
	if _, ok := result.Admission(); ok {
		t.Fatal("corrupted current authority minted a canonical admission")
	}
}

func TestCanonicalAdmissionAndResultZeroValuesAreInvalid(t *testing.T) {
	zeroAdmission := CanonicalProfileAdmission{}
	if zeroAdmission.Valid() {
		t.Fatal("zero canonical admission is valid")
	}
	if zeroAdmission.AdmissionRecordCanonicalJSON() != nil {
		t.Fatal("zero canonical admission exposed record bytes")
	}
	zeroResult := AdmissionResult{}
	if _, ok := zeroResult.Admission(); ok {
		t.Fatal("zero result exposed an admission")
	}
	if _, ok := zeroResult.Denials(); ok {
		t.Fatal("zero result exposed denials")
	}
	if _, ok := zeroResult.Failure(); ok {
		t.Fatal("zero result exposed a failure")
	}
}

func TestServiceResolveCurrentReportsAbsentProfileWithoutMintingToken(t *testing.T) {
	fixture := newTransactionFixture(t, "service-absent", "service-absent.nonce")
	service, err := NewService(fixture.database)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	result := service.ResolveCurrent(context.Background(), fixture.root)
	if result.Kind() != AdmissionResultNotAdmitted {
		t.Fatalf("ResolveCurrent kind = %q, want not_admitted", result.Kind())
	}
	if _, ok := result.Admission(); ok {
		t.Fatal("absent profile minted a canonical admission")
	}
	denials, ok := result.Denials()
	if !ok || len(denials) != 1 || denials[0].Code() != "profile_not_declared" {
		t.Fatalf("ResolveCurrent denials = %#v, want profile_not_declared", denials)
	}
}

func requireCanonicalAdmission(
	t *testing.T,
	result AdmissionResult,
	wantDelivery CanonicalAdmissionDelivery,
) CanonicalProfileAdmission {
	t.Helper()
	if result.Kind() != AdmissionResultAdmitted {
		denials, _ := result.Denials()
		failure, _ := result.Failure()
		t.Fatalf("result kind = %q, denials = %#v, failure = %#v", result.Kind(), denials, failure)
	}
	admission, ok := result.Admission()
	if !ok {
		t.Fatal("admitted result omitted canonical admission")
	}
	if admission.Delivery() != wantDelivery {
		t.Fatalf("delivery = %q, want %q", admission.Delivery(), wantDelivery)
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(admission.Payload())
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	if payloadDigest.String() == "" {
		t.Fatal("canonical admission exposed an empty payload digest")
	}
	return admission
}

func assertCanonicalTokenBindings(
	t *testing.T,
	admission CanonicalProfileAdmission,
) {
	t.Helper()
	provenance := admission.CandidateProvenance()
	if provenance.ProjectRoot() != admission.ProjectRoot() {
		t.Fatal("token provenance project root differs from token root")
	}
	if provenance.WorkRecordRef() != admission.WorkRecordRef() {
		t.Fatal("token provenance Work ref differs from token Work ref")
	}
	if provenance.WorkRecordDigest() != admission.WorkRecordDigest() {
		t.Fatal("token provenance Work digest differs from token Work digest")
	}
	if provenance.AuthorityBasisRef() != admission.AuthorityBasisRef() {
		t.Fatal("token provenance authority basis differs from token authority basis")
	}
	if provenance.ProfileAuthorRoleAssignmentRef() != admission.ProfileAuthorRoleAssignmentRef() {
		t.Fatal("token provenance RoleAssignment ref differs from token RoleAssignment ref")
	}
	if provenance.ProfileAuthorRoleAssignmentDigest() != admission.ProfileAuthorRoleAssignmentDigest() {
		t.Fatal("token provenance RoleAssignment digest differs from token RoleAssignment digest")
	}
	if provenance.ObservedProjectBasisRef() != admission.ObservedProjectBasisRef() {
		t.Fatal("token provenance observed-basis ref differs from token observed-basis ref")
	}
	if provenance.ObservedProjectBasisDigest() != admission.ObservedProjectBasisDigest() {
		t.Fatal("token provenance observed-basis digest differs from token observed-basis digest")
	}
	if provenance.OutcomeAssessmentRef() != admission.OutcomeAssessmentRef() {
		t.Fatal("token provenance assessment ref differs from token assessment ref")
	}
	if provenance.OutcomeAssessmentDigest() != admission.OutcomeAssessmentDigest() {
		t.Fatal("token provenance assessment digest differs from token assessment digest")
	}
	next, err := admission.ExpectedLedgerRevision().Next()
	if err != nil {
		t.Fatalf("advance expected ledger revision: %v", err)
	}
	if next != admission.LedgerRevision() {
		t.Fatal("token expected revision does not advance to committed revision")
	}
	if admission.RecordedAt().IsZero() {
		t.Fatal("token recording time is absent")
	}
	if admission.AuthorityBasisDigest().String() == "" {
		t.Fatal("token authority-basis digest is absent")
	}
	if admission.AuthorityResolutionRef().String() == "" || admission.AuthorityResolutionDigest().String() == "" {
		t.Fatal("token authority-resolution binding is absent")
	}
	if len(admission.ReceiptCanonicalJSON()) == 0 || admission.ReceiptDigest().String() == "" {
		t.Fatal("token exact receipt is absent")
	}
	recordBytes := admission.AdmissionRecordCanonicalJSON()
	recordBytes[0] = 'x'
	if admission.AdmissionRecordCanonicalJSON()[0] == 'x' {
		t.Fatal("token admission-record bytes are mutable through getter")
	}
}
