package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type valueStoreFixtureV1 struct {
	root       projectprofile.ProjectRootV1
	values     ProfileOnboardingValueSetV1
	payload    projectprofile.ProfileDeclarationPayload
	recordedAt time.Time
}

type valueStoreDatabaseV1 struct {
	kernel *db.Store
	raw    *sql.DB
	path   string
}

func TestStoreAndResolveProfileOnboardingValueSetV1AcrossRestart(t *testing.T) {
	directory := t.TempDir()
	databasePath := filepath.Join(directory, "haft.db")
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "restart")
	database := openValueStoreDatabaseV1(t, databasePath)
	ctx := context.Background()
	transaction := beginImmediateV1(t, database.raw)
	snapshot, err := StoreAndReloadProfileOnboardingValueSetV1(
		ctx,
		transaction,
		fixture.root,
		fixture.values,
		fixture.recordedAt,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("StoreAndReloadProfileOnboardingValueSetV1: %v", err)
	}
	assertValueSnapshotV1(t, snapshot, fixture.values)
	commitTransactionV1(t, transaction)
	assertValueTableCountsV1(t, database.raw, 1)
	assertNoAdmissionMutationV1(t, database.raw)
	err = database.kernel.Close()
	if err != nil {
		t.Fatalf("close first database: %v", err)
	}

	restarted := openValueStoreDatabaseV1(t, databasePath)
	t.Cleanup(func() { _ = restarted.kernel.Close() })
	restartedTransaction := beginReadV1(t, restarted.raw)
	identity := fixtureValueIdentityV1(t, fixture)
	reloaded, err := ResolveProfileOnboardingValueSetV1(
		ctx,
		restartedTransaction,
		identity,
	)
	if err != nil {
		_ = restartedTransaction.Rollback(ctx)
		t.Fatalf("ResolveProfileOnboardingValueSetV1 after restart: %v", err)
	}
	assertValueSnapshotV1(t, reloaded, fixture.values)
	rollbackTransactionV1(t, restartedTransaction)
}

func TestStoreProfileOnboardingValueSetV1ExactRetryIsIdempotent(t *testing.T) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "retry")
	ctx := context.Background()
	first := beginImmediateV1(t, database.raw)
	_, err := StoreAndReloadProfileOnboardingValueSetV1(
		ctx,
		first,
		fixture.root,
		fixture.values,
		fixture.recordedAt,
	)
	if err != nil {
		_ = first.Rollback(ctx)
		t.Fatalf("first store: %v", err)
	}
	commitTransactionV1(t, first)

	retry := beginImmediateV1(t, database.raw)
	snapshot, err := StoreAndReloadProfileOnboardingValueSetV1(
		ctx,
		retry,
		fixture.root,
		fixture.values,
		fixture.recordedAt.Add(time.Hour),
	)
	if err != nil {
		_ = retry.Rollback(ctx)
		t.Fatalf("exact retry: %v", err)
	}
	assertValueSnapshotV1(t, snapshot, fixture.values)
	commitTransactionV1(t, retry)
	assertValueTableCountsV1(t, database.raw, 1)
}

func TestStoreProfileOnboardingValueSetV1DoesNotHideUnrelatedInsertFailure(t *testing.T) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "injected")
	storeCommittedFixtureV1(t, database.raw, fixture)
	execDatabaseV1(t, database.raw, `CREATE TRIGGER injected_observed_basis_failure
		BEFORE INSERT ON observed_project_bases BEGIN
			SELECT RAISE(ABORT, 'unrelated injected observed-basis failure');
		END`)
	transaction := beginImmediateV1(t, database.raw)
	_, err := StoreAndReloadProfileOnboardingValueSetV1(
		context.Background(),
		transaction,
		fixture.root,
		fixture.values,
		fixture.recordedAt.Add(time.Hour),
	)
	if err == nil || !strings.Contains(err.Error(), "unrelated injected observed-basis failure") {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("unrelated insert failure = %v", err)
	}
	rollbackTransactionV1(t, transaction)
	assertValueTableCountsV1(t, database.raw, 1)
}

func TestStoreProfileOnboardingValueSetV1CallerRollbackLeavesNoMutation(t *testing.T) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "rollback")
	transaction := beginImmediateV1(t, database.raw)
	_, err := StoreAndReloadProfileOnboardingValueSetV1(
		context.Background(),
		transaction,
		fixture.root,
		fixture.values,
		fixture.recordedAt,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("store before caller rollback: %v", err)
	}
	rollbackTransactionV1(t, transaction)
	assertValueTableCountsV1(t, database.raw, 0)
	assertNoAdmissionMutationV1(t, database.raw)
}

func TestProfileOnboardingStoreAndResolveRejectInvalidTransactionCapabilities(t *testing.T) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "capability")
	readTransaction := beginReadV1(t, database.raw)
	_, err := StoreAndReloadProfileOnboardingValueSetV1(
		context.Background(),
		readTransaction,
		fixture.root,
		fixture.values,
		fixture.recordedAt,
	)
	if !errors.Is(err, sqlitetransaction.ErrImmediateRequired) {
		_ = readTransaction.Rollback(context.Background())
		t.Fatalf("store with read transaction = %v, want immediate-required", err)
	}
	rollbackTransactionV1(t, readTransaction)
	_, err = ResolveProfileOnboardingValueSetV1(
		context.Background(),
		&sqlitetransaction.Transaction{},
		ProfileOnboardingValueIdentityV1{},
	)
	if !errors.Is(err, sqlitetransaction.ErrTransactionInvalid) {
		t.Fatalf("resolve with invalid transaction = %v, want transaction-invalid", err)
	}
	_, err = ResolveProfileOnboardingValueSetV1(
		context.Background(),
		readTransaction,
		ProfileOnboardingValueIdentityV1{},
	)
	if !errors.Is(err, sqlitetransaction.ErrTransactionFinished) {
		t.Fatalf("resolve with finished transaction = %v, want transaction-finished", err)
	}
}

func TestResolveProfileAdmissionValueSnapshotV1UsesExactDurableSupports(t *testing.T) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), "candidate")
	storeCommittedFixtureV1(t, database.raw, fixture)
	candidate := newAdmissionCandidateV1(t, fixture, "candidate")
	transaction := beginReadV1(t, database.raw)
	snapshot, err := ResolveProfileAdmissionValueSnapshotV1(
		context.Background(),
		transaction,
		candidate,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("ResolveProfileAdmissionValueSnapshotV1: %v", err)
	}
	values, ok := snapshot.Values()
	if !ok {
		_ = transaction.Rollback(context.Background())
		t.Fatal("candidate snapshot did not expose values")
	}
	if values.WorkRecord().RecordRef() != fixture.values.WorkRecord().RecordRef() {
		_ = transaction.Rollback(context.Background())
		t.Fatal("candidate snapshot resolved another Work record")
	}
	rollbackTransactionV1(t, transaction)
	assertValueTableCountsV1(t, database.raw, 1)
	assertNoAdmissionMutationV1(t, database.raw)
}

func TestResolveProfileOnboardingValueSetV1RejectsDurableCorruption(t *testing.T) {
	tests := []struct {
		name      string
		trigger   string
		mutation  string
		wantError string
	}{
		{
			name:      "basis canonical record",
			trigger:   "observed_project_bases_no_update",
			mutation:  `UPDATE observed_project_bases SET observed_project_basis_json = '{}';`,
			wantError: "strictly decode durable ObservedProjectBasis",
		},
		{
			name:      "effect digest projection",
			trigger:   "profile_onboarding_effects_no_update",
			mutation:  `UPDATE profile_onboarding_effects SET effect_digest = 'sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff';`,
			wantError: "effect columns do not match",
		},
		{
			name:      "Work recording time",
			trigger:   "profile_onboarding_work_records_no_update",
			mutation:  `UPDATE profile_onboarding_work_records SET recorded_at = '2000-01-01T00:00:00Z';`,
			wantError: "validate durable Work recorded_at",
		},
		{
			name:      "assignment support recording time",
			trigger:   "profile_author_assignment_support_carriers_no_update",
			mutation:  `UPDATE profile_author_assignment_support_carriers SET recorded_at = '2000-01-01T00:00:00Z';`,
			wantError: "must not precede assignment provenance",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
			t.Cleanup(func() { _ = database.kernel.Close() })
			fixture := newValueStoreFixtureV1(t, filepath.Join(directory, "project"), strings.ReplaceAll(test.name, " ", "-"))
			storeCommittedFixtureV1(t, database.raw, fixture)
			execDatabaseV1(t, database.raw, "DROP TRIGGER "+test.trigger)
			execDatabaseV1(t, database.raw, test.mutation)
			transaction := beginReadV1(t, database.raw)
			identity := fixtureValueIdentityV1(t, fixture)
			_, err := ResolveProfileOnboardingValueSetV1(
				context.Background(),
				transaction,
				identity,
			)
			rollbackTransactionV1(t, transaction)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("corruption error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestStoreProfileOnboardingValueSetV1ConcurrentWorkIdentityCollision(t *testing.T) {
	directory := t.TempDir()
	database := openValueStoreDatabaseV1(t, filepath.Join(directory, "haft.db"))
	t.Cleanup(func() { _ = database.kernel.Close() })
	projectPath := filepath.Join(directory, "project")
	left := newValueStoreFixtureV1(t, projectPath, "collision-left")
	right := newValueStoreFixtureV1(t, projectPath, "collision-right")
	sharedWorkRef := left.values.WorkRecord().WorkRef()
	right.values = rebuildValueSetWithWorkRefV1(t, right.values, sharedWorkRef)
	fixtures := []valueStoreFixtureV1{left, right}
	start := make(chan struct{})
	results := make(chan error, len(fixtures))
	var workers sync.WaitGroup
	workers.Add(len(fixtures))
	for _, fixture := range fixtures {
		current := fixture
		go func() {
			defer workers.Done()
			transaction, err := sqlitetransaction.BeginImmediate(
				context.Background(),
				database.raw,
			)
			if err != nil {
				results <- err
				return
			}
			<-start
			_, storeErr := StoreAndReloadProfileOnboardingValueSetV1(
				context.Background(),
				transaction,
				current.root,
				current.values,
				current.recordedAt,
			)
			if storeErr != nil {
				_ = transaction.Rollback(context.Background())
				results <- storeErr
				return
			}
			commit := transaction.Commit(context.Background())
			results <- commit.Err()
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		failures++
		if !strings.Contains(err.Error(), "not durably readable") &&
			!strings.Contains(err.Error(), "append-only") {
			t.Fatalf("unexpected concurrent collision error: %v", err)
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent collision successes=%d failures=%d, want 1/1", successes, failures)
	}
	assertValueTableCountsV1(t, database.raw, 1)
	assertNoAdmissionMutationV1(t, database.raw)
}

func newValueStoreFixtureV1(
	t *testing.T,
	projectPath string,
	suffix string,
) valueStoreFixtureV1 {
	t.Helper()
	err := os.MkdirAll(projectPath, 0o755)
	if err != nil {
		t.Fatalf("create physical project root: %v", err)
	}
	physicalProjectPath, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatalf("resolve physical project root: %v", err)
	}
	root := mustParsedV1(t, physicalProjectPath, projectprofile.NewProjectRootV1)
	description := projectprofile.ProfileOnboardingMethodDescriptionV1Value()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodDescriptionV1: %v", err)
	}
	contract, err := projectprofile.ProfileOnboardingMethodContractV1Value()
	if err != nil {
		t.Fatalf("ProfileOnboardingMethodContractV1Value: %v", err)
	}
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV1(contract)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingMethodContractV1: %v", err)
	}
	session := mustParsedV1(t, "session:sqlite:"+suffix, projectprofile.NewSessionRef)
	system := mustParsedV1(t, "system:sqlite:"+suffix, projectprofile.NewSystemRef)
	kernel, err := projectprofile.NewProfileOnboardingKernelIdentityV1("haft-kernel", "v9-test")
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelIdentityV1: %v", err)
	}
	runtime, err := projectprofile.NewProfileOnboardingRuntimeIdentityV1("codex-runtime", "v9-test")
	if err != nil {
		t.Fatalf("NewProfileOnboardingRuntimeIdentityV1: %v", err)
	}
	identityBasis, err := projectprofile.NewProfileOnboardingKernelExecutorIdentityBasisV1(system, kernel)
	if err != nil {
		t.Fatalf("NewProfileOnboardingKernelExecutorIdentityBasisV1: %v", err)
	}
	systemWindow, err := projectprofile.NewProfileOnboardingExecutorAdmissionWindowV1(
		time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 18, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingExecutorAdmissionWindowV1: %v", err)
	}
	systemAdmissionBuilder := projectprofile.NewProfileOnboardingExecutorSystemAdmissionV1Builder(
		mustParsedV1(t, "system-admission:sqlite:"+suffix, projectprofile.NewSystemAdmissionRef),
		system,
	)
	systemAdmissionBuilder = systemAdmissionBuilder.IdentifiedBy(identityBasis)
	systemAdmissionBuilder = systemAdmissionBuilder.AdmittedToActBy(
		mustParsedV1(
			t,
			"acting-eligibility:sqlite:"+suffix,
			projectprofile.NewProfileOnboardingSystemActingEligibilityBasisRefV1,
		),
		testDigestV1(t, "acting-eligibility:"+suffix),
	)
	systemAdmissionBuilder = systemAdmissionBuilder.InSession(session)
	systemAdmissionBuilder = systemAdmissionBuilder.ValidDuring(systemWindow)
	systemAdmission, err := systemAdmissionBuilder.Build()
	if err != nil {
		t.Fatalf("executor-system admission Build: %v", err)
	}
	roleAdmission, err := projectprofile.NewProfileAuthorRoleAdmissionV1(
		mustParsedV1(t, "role-admission:sqlite:"+suffix, projectprofile.NewRoleAdmissionRef),
	)
	if err != nil {
		t.Fatalf("NewProfileAuthorRoleAdmissionV1: %v", err)
	}
	assignmentWindow, err := projectprofile.NewRoleAssignmentWindowV1(
		time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 17, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewRoleAssignmentWindowV1: %v", err)
	}
	justificationBuilder := projectprofile.NewProfileAuthorAssignmentJustificationV1Builder(
		mustParsedV1(t, "assignment-justification:sqlite:"+suffix, projectprofile.NewRoleAssignmentJustificationRef),
	)
	justificationBuilder = justificationBuilder.ApplyingAdmissions(systemAdmission, roleAdmission)
	justificationBuilder = justificationBuilder.ValidDuring(assignmentWindow)
	justification, err := justificationBuilder.Build()
	if err != nil {
		t.Fatalf("assignment justification Build: %v", err)
	}
	provenanceBuilder := projectprofile.NewProfileAuthorAssignmentProvenanceV1Builder(
		mustParsedV1(t, "assignment-provenance:sqlite:"+suffix, projectprofile.NewRoleAssignmentProvenanceRef),
		justification,
	)
	provenanceBuilder = provenanceBuilder.InSession(session)
	provenanceBuilder = provenanceBuilder.ProducedBy(kernel, runtime)
	provenanceBuilder = provenanceBuilder.RecordedAt(time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC))
	provenance, err := provenanceBuilder.Build()
	if err != nil {
		t.Fatalf("assignment provenance Build: %v", err)
	}
	support, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		t.Fatalf("CarryProfileAuthorAssignmentSupportV1: %v", err)
	}
	assignmentRef := mustParsedV1(t, "role-assignment:sqlite:"+suffix, projectprofile.NewRoleAssignmentRef)
	assignmentBuilder := projectprofile.NewProfileAuthorRoleAssignmentV1Builder(assignmentRef)
	assignmentBuilder = assignmentBuilder.HeldBy(system)
	assignmentBuilder = assignmentBuilder.Assigning(projectprofile.ProfileAuthorRoleRefV1())
	assignmentBuilder = assignmentBuilder.InContext(projectprofile.ProfileOnboardingBoundedContextRefV1())
	assignmentBuilder = assignmentBuilder.ValidDuring(assignmentWindow)
	assignmentBuilder = assignmentBuilder.WithSystemAdmission(systemAdmission.Ref(), support.SystemAdmissionDigest())
	assignmentBuilder = assignmentBuilder.WithRoleAdmission(roleAdmission.Ref(), support.RoleAdmissionDigest())
	assignmentBuilder = assignmentBuilder.JustifiedBy(justification.Ref(), support.JustificationDigest())
	assignmentBuilder = assignmentBuilder.WithProvenance(provenance.Ref(), support.ProvenanceDigest())
	assignment, err := assignmentBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileAuthorRoleAssignment Build: %v", err)
	}
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	basisWindow, err := projectprofile.NewBasisObservationWindowV1(
		time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewBasisObservationWindowV1: %v", err)
	}
	signal, err := projectprofile.NewObservedProjectSignalV1(
		mustParsedV1(t, "repository-shape", projectprofile.NewObservedProjectSignalKindV1),
		mustParsedV1(t, "software", projectprofile.NewObservedProjectSignalValueV1),
		mustParsedV1(t, "carrier:tree:"+suffix, projectprofile.NewSourceCarrierRefV1),
		[]projectprofile.EvidenceProvenancePathRefV1{
			mustParsedV1(t, "evidence:path:tree:"+suffix, projectprofile.NewEvidenceProvenancePathRefV1),
		},
	)
	if err != nil {
		t.Fatalf("NewObservedProjectSignalV1: %v", err)
	}
	classifier := mustParsedV1(t, "classifier-v9", projectprofile.NewClassifierVersion)
	basis, err := projectprofile.NewObservedProjectBasisV1(
		mustParsedV1(t, "observed-basis:sqlite:"+suffix, projectprofile.NewObservedProjectBasisRefV1),
		root,
		basisWindow,
		[]projectprofile.ObservedProjectSignalV1{signal},
		mustParsedV1(t, "detector-v9", projectprofile.NewObservedProjectDetectorVersionV1),
		classifier,
	)
	if err != nil {
		t.Fatalf("NewObservedProjectBasisV1: %v", err)
	}
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	workInterval, err := projectprofile.NewWorkIntervalV1(
		time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewWorkIntervalV1: %v", err)
	}
	pre := mustParsedV1(t, "state:before:"+suffix, projectprofile.NewStateRef)
	post := mustParsedV1(t, "state:after:"+suffix, projectprofile.NewStateRef)
	transition, err := projectprofile.NewPrePostStateTransitionV1(pre, post)
	if err != nil {
		t.Fatalf("NewPrePostStateTransitionV1: %v", err)
	}
	payload := newProfilePayloadV1(t, suffix)
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(payload)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	outcome, err := projectprofile.NewCandidatePayloadProduced(payloadDigest, basisDigest)
	if err != nil {
		t.Fatalf("NewCandidatePayloadProduced: %v", err)
	}
	bindings := newMethodBindingsV1(t, root, session, classifier)
	workBuilder := projectprofile.NewProfileOnboardingWorkRecordBuilder(
		mustParsedV1(t, "work-record:sqlite:"+suffix, projectprofile.NewProfileOnboardingWorkRecordRef),
		mustParsedV1(t, "work:sqlite:"+suffix, projectprofile.NewWorkRef),
	)
	workBuilder = workBuilder.Enacts(description.DescribedMethodRef(), description.Ref(), bindings)
	workBuilder = workBuilder.WithMethodDescriptionDigest(descriptionDigest)
	workBuilder = workBuilder.GovernedByMethodContract(contract.Ref(), contractDigest)
	workBuilder = workBuilder.PerformedBy(assignmentRef)
	workBuilder = workBuilder.WithProfileAuthorRoleAssignment(assignmentRef, assignmentDigest)
	workBuilder = workBuilder.ExecutedWithin(system)
	workBuilder = workBuilder.InContext(description.BoundedContextRef())
	workBuilder = workBuilder.During(workInterval, basisWindow)
	workBuilder = workBuilder.WithObservedProjectBasis(basis.Ref(), basisDigest)
	workBuilder = workBuilder.WithInputs([]projectprofile.WorkInputRef{
		mustParsedV1(t, basis.Ref().String(), projectprofile.NewWorkInputRef),
	})
	outputRef := mustParsedV1(t, "output:profile-candidate:"+suffix, projectprofile.NewWorkOutputRef)
	workBuilder = workBuilder.WithOutputs([]projectprofile.WorkOutputRef{outputRef})
	workBuilder = workBuilder.WithResources([]projectprofile.WorkResourceRef{
		mustParsedV1(t, "resource:sqlite:"+suffix, projectprofile.NewWorkResourceRef),
	})
	workBuilder = workBuilder.AffectingKind(description.AffectedRefKind())
	affectedRaw := "eoc:profile-classification:" + suffix
	workBuilder = workBuilder.Affecting([]projectprofile.AffectedReferentRef{
		mustParsedV1(t, affectedRaw, projectprofile.NewAffectedReferentRef),
	})
	workBuilder = workBuilder.OnStatePlane(description.StatePlaneRef(), transition)
	workBuilder = workBuilder.WithOutcome(outcome)
	work, err := workBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingWorkRecord Build: %v", err)
	}
	workDigest := mustWorkDigestV1(t, work)
	result, err := projectprofile.NewProfileOnboardingCandidateResultV1(
		outputRef,
		payloadDigest,
		basis.Ref(),
		basisDigest,
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingCandidateResultV1: %v", err)
	}
	affectedEntity := mustParsedV1(t, affectedRaw, projectprofile.NewEntityRef)
	effect, err := projectprofile.NewProfileOnboardingEffectV1(
		mustParsedV1(t, "effect:sqlite:"+suffix, projectprofile.NewProfileOnboardingEffectRefV1),
		work.RecordRef(),
		work.WorkRef(),
		workDigest,
		result,
		[]projectprofile.EntityRef{affectedEntity},
		description.StatePlaneRef(),
		transition,
		[]projectprofile.EvidenceProvenancePathRefV1{
			mustParsedV1(t, "evidence:path:effect:"+suffix, projectprofile.NewEvidenceProvenancePathRefV1),
		},
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingEffectV1: %v", err)
	}
	standardEdition := mustParsedV1(
		t,
		contract.AcceptanceStandardEdition(),
		projectprofile.NewProfileOnboardingAcceptanceStandardEditionV1,
	)
	assessment, err := projectprofile.NewProfileOnboardingOutcomeAssessmentV1(
		mustParsedV1(t, "assessment:sqlite:"+suffix, projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1),
		effect,
		contract.AcceptanceStandardRef(),
		standardEdition,
		mustParsedV1(t, "comparator:sqlite:"+suffix, projectprofile.NewProfileOnboardingComparatorRefV1),
		mustParsedV1(t, "v9-test", projectprofile.NewProfileOnboardingComparatorEditionV1),
		projectprofile.ProfileOnboardingAcceptancePassedV1Value(),
		[]projectprofile.EvidenceProvenancePathRefV1{
			mustParsedV1(t, "evidence:path:assessment:"+suffix, projectprofile.NewEvidenceProvenancePathRefV1),
		},
	)
	if err != nil {
		t.Fatalf("NewProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	valueBuilder := NewProfileOnboardingValueSetV1Builder(work)
	valueBuilder = valueBuilder.WithMethodDescription(description)
	valueBuilder = valueBuilder.WithMethodContract(contract)
	valueBuilder = valueBuilder.WithSystemAdmission(systemAdmission)
	valueBuilder = valueBuilder.WithRoleAdmission(roleAdmission)
	valueBuilder = valueBuilder.WithAssignmentJustification(justification)
	valueBuilder = valueBuilder.WithAssignmentProvenance(provenance)
	valueBuilder = valueBuilder.WithRoleAssignment(assignment)
	valueBuilder = valueBuilder.WithObservedBasis(basis)
	valueBuilder = valueBuilder.WithEffect(effect)
	valueBuilder = valueBuilder.WithAssessment(assessment)
	values, err := valueBuilder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingValueSetV1 Build: %v", err)
	}
	return valueStoreFixtureV1{
		root:       root,
		values:     values,
		payload:    payload,
		recordedAt: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
	}
}

func newProfilePayloadV1(t *testing.T, suffix string) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID := mustParsedV1(t, "software-"+suffix, projectprofile.NewScopeID)
	scope, err := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet([]projectprofile.RealizationScope{scope})
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func newAdmissionCandidateV1(
	t *testing.T,
	fixture valueStoreFixtureV1,
	suffix string,
) projectprofile.ProfileDeclarationCandidateV1 {
	t.Helper()
	values := fixture.values
	work := values.WorkRecord()
	workDigest := mustWorkDigestV1(t, work)
	assignment := values.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		t.Fatalf("DigestProfileAuthorRoleAssignmentV1: %v", err)
	}
	basis := values.ObservedBasis()
	basisDigest, err := projectprofile.DigestObservedProjectBasisV1(basis)
	if err != nil {
		t.Fatalf("DigestObservedProjectBasisV1: %v", err)
	}
	assessment := values.Assessment()
	assessmentDigest := mustAssessmentDigestV1(t, assessment)
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(fixture.payload)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	authorityBasis := mustParsedV1(
		t,
		"authority-basis:sqlite:"+suffix,
		projectprofile.NewProfileDeclarationAuthorityBasisRef,
	)
	policy := mustParsedV1(t, "policy-v9", projectprofile.NewPolicyVersion)
	builder := projectprofile.NewCandidateProvenanceV1Builder(
		authorityBasis,
		work.RecordRef(),
		workDigest,
	)
	builder = builder.ForProject(fixture.root)
	builder = builder.ForProfileAuthorRoleAssignment(assignment.RoleAssignmentRef(), assignmentDigest)
	builder = builder.ClassifiedBy(basis.ClassifierVersion(), policy)
	builder = builder.InSession(values.AssignmentProvenance().SessionRef())
	builder = builder.ForPayload(payloadDigest)
	builder = builder.ForObservedProjectBasis(basis.Ref(), basisDigest)
	builder = builder.ForOutcomeAssessment(assessment.Ref(), assessmentDigest)
	provenance, err := builder.Build()
	if err != nil {
		t.Fatalf("CandidateProvenanceV1 Build: %v", err)
	}
	candidate, err := projectprofile.NewProfileDeclarationCandidateV1(fixture.payload, provenance)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCandidateV1: %v", err)
	}
	return candidate
}

func rebuildValueSetWithWorkRefV1(
	t *testing.T,
	values ProfileOnboardingValueSetV1,
	workRef projectprofile.WorkRef,
) ProfileOnboardingValueSetV1 {
	t.Helper()
	original := values.WorkRecord()
	builder := projectprofile.NewProfileOnboardingWorkRecordBuilder(
		original.RecordRef(),
		workRef,
	)
	builder = builder.Enacts(
		original.EnactsMethodRef(),
		original.MethodDescriptionRef(),
		original.ParameterBindings(),
	)
	builder = builder.WithMethodDescriptionDigest(original.MethodDescriptionDigest())
	builder = builder.GovernedByMethodContract(
		original.MethodContractRef(),
		original.MethodContractDigest(),
	)
	builder = builder.PerformedBy(original.PerformedBy())
	builder = builder.WithProfileAuthorRoleAssignment(
		original.ProfileAuthorRoleAssignmentRef(),
		original.ProfileAuthorRoleAssignmentDigest(),
	)
	builder = builder.ExecutedWithin(original.ExecutedWithin())
	builder = builder.InContext(original.BoundedContextRef())
	builder = builder.During(original.WorkInterval(), original.BasisObservationWindow())
	builder = builder.WithObservedProjectBasis(
		original.ObservedProjectBasisRef(),
		original.ObservedProjectBasisDigest(),
	)
	builder = builder.WithInputs(original.InputRefs())
	builder = builder.WithOutputs(original.OutputRefs())
	builder = builder.WithResources(original.ResourceRefs())
	builder = builder.AffectingKind(original.AffectedRefKind())
	builder = builder.Affecting(original.AffectedRefs())
	builder = builder.OnStatePlane(original.StatePlaneRef(), original.StateTransition())
	builder = builder.WithOutcome(original.Outcome())
	work, err := builder.Build()
	if err != nil {
		t.Fatalf("rebuild colliding Work: %v", err)
	}
	workDigest := mustWorkDigestV1(t, work)
	originalEffect := values.Effect()
	effect, err := projectprofile.NewProfileOnboardingEffectV1(
		originalEffect.Ref(),
		work.RecordRef(),
		work.WorkRef(),
		workDigest,
		originalEffect.Result(),
		originalEffect.AffectedEntityRefs(),
		originalEffect.StatePlaneRef(),
		originalEffect.StateWitness(),
		originalEffect.EvidencePathRefs(),
	)
	if err != nil {
		t.Fatalf("rebuild colliding effect: %v", err)
	}
	originalAssessment := values.Assessment()
	assessment, err := projectprofile.NewProfileOnboardingOutcomeAssessmentV1(
		originalAssessment.Ref(),
		effect,
		originalAssessment.AcceptanceStandardRef(),
		originalAssessment.AcceptanceStandardEdition(),
		originalAssessment.ComparatorRef(),
		originalAssessment.ComparatorEdition(),
		originalAssessment.Verdict(),
		originalAssessment.EvidencePathRefs(),
	)
	if err != nil {
		t.Fatalf("rebuild colliding assessment: %v", err)
	}
	valueBuilder := NewProfileOnboardingValueSetV1Builder(work)
	valueBuilder = valueBuilder.WithMethodDescription(values.MethodDescription())
	valueBuilder = valueBuilder.WithMethodContract(values.MethodContract())
	valueBuilder = valueBuilder.WithSystemAdmission(values.SystemAdmission())
	valueBuilder = valueBuilder.WithRoleAdmission(values.RoleAdmission())
	valueBuilder = valueBuilder.WithAssignmentJustification(values.AssignmentJustification())
	valueBuilder = valueBuilder.WithAssignmentProvenance(values.AssignmentProvenance())
	valueBuilder = valueBuilder.WithRoleAssignment(values.RoleAssignment())
	valueBuilder = valueBuilder.WithObservedBasis(values.ObservedBasis())
	valueBuilder = valueBuilder.WithEffect(effect)
	valueBuilder = valueBuilder.WithAssessment(assessment)
	result, err := valueBuilder.Build()
	if err != nil {
		t.Fatalf("rebuild colliding value set: %v", err)
	}
	return result
}

func newMethodBindingsV1(
	t *testing.T,
	root projectprofile.ProjectRootV1,
	session projectprofile.SessionRef,
	classifier projectprofile.ClassifierVersion,
) projectprofile.MethodParameterBindings {
	t.Helper()
	values := []struct {
		name  string
		value string
	}{
		{name: classifierVersionBinding, value: classifier.String()},
		{name: policyVersionBinding, value: "policy-v9"},
		{name: projectRootBinding, value: root.String()},
		{name: sessionRefBinding, value: session.String()},
	}
	bindings := make([]projectprofile.MethodParameterBinding, 0, len(values))
	for _, value := range values {
		binding, err := projectprofile.NewMethodParameterBinding(value.name, value.value)
		if err != nil {
			t.Fatalf("NewMethodParameterBinding(%s): %v", value.name, err)
		}
		bindings = append(bindings, binding)
	}
	result, err := projectprofile.NewMethodParameterBindings(bindings)
	if err != nil {
		t.Fatalf("NewMethodParameterBindings: %v", err)
	}
	return result
}

func openValueStoreDatabaseV1(t *testing.T, path string) valueStoreDatabaseV1 {
	t.Helper()
	kernel, err := db.NewStore(path)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	return valueStoreDatabaseV1{
		kernel: kernel,
		raw:    kernel.GetRawDB(),
		path:   path,
	}
}

func beginImmediateV1(
	t *testing.T,
	database *sql.DB,
) *sqlitetransaction.Transaction {
	t.Helper()
	transaction, err := sqlitetransaction.BeginImmediate(
		context.Background(),
		database,
	)
	if err != nil {
		t.Fatalf("begin immediate SQLite transaction: %v", err)
	}
	return transaction
}

func beginReadV1(
	t *testing.T,
	database *sql.DB,
) *sqlitetransaction.Transaction {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(
		context.Background(),
		database,
	)
	if err != nil {
		t.Fatalf("begin read SQLite transaction: %v", err)
	}
	return transaction
}

func commitTransactionV1(
	t *testing.T,
	transaction *sqlitetransaction.Transaction,
) {
	t.Helper()
	result := transaction.Commit(context.Background())
	if !result.Succeeded() {
		t.Fatalf("commit SQLite transaction: %v", result.Err())
	}
}

func rollbackTransactionV1(
	t *testing.T,
	transaction *sqlitetransaction.Transaction,
) {
	t.Helper()
	result := transaction.Rollback(context.Background())
	if !result.Succeeded() {
		t.Fatalf("roll back SQLite transaction: %v", result.Err())
	}
}

func execDatabaseV1(t *testing.T, database *sql.DB, statement string) {
	t.Helper()
	_, err := database.Exec(statement)
	if err != nil {
		t.Fatalf("exec database SQL %q: %v", statement, err)
	}
}

func storeCommittedFixtureV1(
	t *testing.T,
	database *sql.DB,
	fixture valueStoreFixtureV1,
) {
	t.Helper()
	transaction := beginImmediateV1(t, database)
	_, err := StoreAndReloadProfileOnboardingValueSetV1(
		context.Background(),
		transaction,
		fixture.root,
		fixture.values,
		fixture.recordedAt,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("store committed fixture: %v", err)
	}
	commitTransactionV1(t, transaction)
}

func assertValueSnapshotV1(
	t *testing.T,
	snapshot DurableProfileOnboardingSnapshotV1,
	want ProfileOnboardingValueSetV1,
) {
	t.Helper()
	got, ok := snapshot.Values()
	if !ok {
		t.Fatal("snapshot did not expose valid values")
	}
	root := want.ObservedBasis().ProjectRoot()
	recordedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	gotRows, err := prepareProfileOnboardingRowsV1(root, got, recordedAt)
	if err != nil {
		t.Fatalf("prepare reloaded rows: %v", err)
	}
	wantRows, err := prepareProfileOnboardingRowsV1(root, want, recordedAt)
	if err != nil {
		t.Fatalf("prepare expected rows: %v", err)
	}
	if !sameProfileOnboardingRowSemantics(gotRows, wantRows) {
		t.Fatal("snapshot values differ from expected values")
	}
}

func assertValueTableCountsV1(t *testing.T, database *sql.DB, want int) {
	t.Helper()
	tables := []string{
		"profile_onboarding_method_descriptions",
		"profile_onboarding_method_contracts",
		"profile_onboarding_executor_system_admissions",
		"profile_author_role_admissions",
		"profile_author_assignment_support_carriers",
		"profile_author_role_assignments",
		"observed_project_bases",
		"profile_onboarding_work_records",
		"profile_onboarding_effects",
		"profile_onboarding_outcome_assessments",
	}
	for _, table := range tables {
		if countRowsV1(t, database, table) != want {
			t.Fatalf("%s row count != %d", table, want)
		}
	}
}

func assertNoAdmissionMutationV1(t *testing.T, database *sql.DB) {
	t.Helper()
	tables := []string{
		"project_profile_admissions",
		"authority_uses",
		"project_profile_revisions",
	}
	for _, table := range tables {
		if countRowsV1(t, database, table) != 0 {
			t.Fatalf("storage mutated %s", table)
		}
	}
}

func countRowsV1(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	var count int
	row := database.QueryRow("SELECT COUNT(*) FROM " + table)
	err := row.Scan(&count)
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func fixtureValueIdentityV1(
	t *testing.T,
	fixture valueStoreFixtureV1,
) ProfileOnboardingValueIdentityV1 {
	t.Helper()
	work := fixture.values.WorkRecord()
	workRef := work.RecordRef()
	workDigest := mustWorkDigestV1(t, work)
	assessment := fixture.values.Assessment()
	assessmentRef := assessment.Ref()
	assessmentDigest := mustAssessmentDigestV1(t, assessment)
	builder := NewProfileOnboardingValueIdentityV1Builder(fixture.root)
	builder = builder.WithWork(workRef, workDigest)
	builder = builder.WithAssessment(assessmentRef, assessmentDigest)
	identity, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileOnboardingValueIdentityV1 Build: %v", err)
	}
	return identity
}

func mustWorkDigestV1(
	t *testing.T,
	value projectprofile.ProfileOnboardingWorkRecord,
) projectprofile.ContentDigest {
	t.Helper()
	digest, err := projectprofile.DigestProfileOnboardingWorkRecord(value)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingWorkRecord: %v", err)
	}
	return digest
}

func mustAssessmentDigestV1(
	t *testing.T,
	value projectprofile.ProfileOnboardingOutcomeAssessmentV1,
) projectprofile.ContentDigest {
	t.Helper()
	digest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(value)
	if err != nil {
		t.Fatalf("DigestProfileOnboardingOutcomeAssessmentV1: %v", err)
	}
	return digest
}

func testDigestV1(t *testing.T, seed string) projectprofile.ContentDigest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return mustParsedV1(t, raw, projectprofile.NewContentDigest)
}

func mustParsedV1[T any](
	t *testing.T,
	raw string,
	parse func(string) (T, error),
) T {
	t.Helper()
	value, err := parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return value
}
