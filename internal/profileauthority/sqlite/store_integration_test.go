package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestV43PreparationSourceAndClosureRoundTrip(t *testing.T) {
	ctx := context.Background()
	storeDB, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "profile-authority-v43.sqlite"),
	)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = storeDB.Close() })
	database := storeDB.GetRawDB()
	database.SetMaxOpenConns(1)
	prepared := testPrepared(t, t.TempDir(), "roundtrip")
	seedProfileAuthoritySupport(t, database, prepared)
	clock := time.Date(2026, 7, 15, 12, 10, 0, 0, time.UTC)
	store, err := openWithClock(database, func() time.Time { return clock })
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	manual, ok := prepared.ManualSpeechAct()
	if !ok {
		t.Fatal("prepared authorization omitted manual SpeechAct")
	}
	content, _ := prepared.Content()
	validity, _ := content.AuthorizationValidity()
	verified, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		manual,
		validity.From().Add(time.Minute),
		validity.From().Add(2*time.Minute),
		validity.From().Add(3*time.Minute),
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	transaction := beginImmediateForTest(t, database)
	write, err := store.RecordPreparationAndSourceInTransaction(
		ctx,
		transaction,
		prepared,
		verified,
	)
	if err != nil {
		t.Fatalf("RecordPreparationAndSourceInTransaction: %v", err)
	}
	if write.Kind() != WriteStaged {
		t.Fatalf("source write kind = %q, want staged", write.Kind())
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit preparation/source: %v", finish.Err())
	}

	root, _ := content.ProjectRoot()
	action, _ := profileauthority.ActionKind()
	attempt, err := store.ResolveProjectAttempt(ctx, root, action)
	if err != nil {
		t.Fatalf("ResolveProjectAttempt pending: %v", err)
	}
	if attempt.Kind() != AttemptPendingClosure || attempt.CandidateCount() != 1 {
		t.Fatalf("pending attempt = %#v", attempt)
	}

	preparedDigest, _ := prepared.Digest()
	closureWrite, err := store.InstituteClosure(ctx, preparedDigest)
	if err != nil {
		t.Fatalf("InstituteClosure: %v", err)
	}
	if closureWrite.Kind() != WriteStaged {
		t.Fatalf("closure write kind = %q, want staged", closureWrite.Kind())
	}
	closure, ok := closureWrite.Closure()
	if !ok {
		t.Fatal("closure write omitted durable closure")
	}
	assertClosureLoads(t, database, closure)

	replay, err := store.InstituteClosure(ctx, preparedDigest)
	if err != nil {
		t.Fatalf("InstituteClosure replay: %v", err)
	}
	if replay.Kind() != WriteExactReplay {
		t.Fatalf("closure replay kind = %q, want exact_replay", replay.Kind())
	}
	attempt, err = store.ResolveProjectAttempt(ctx, root, action)
	if err != nil {
		t.Fatalf("ResolveProjectAttempt closed: %v", err)
	}
	if attempt.Kind() != AttemptClosed {
		t.Fatalf("closed attempt kind = %q", attempt.Kind())
	}

	replayTransaction := beginImmediateForTest(t, database)
	sourceReplay, err := store.RecordPreparationAndSourceInTransaction(
		ctx,
		replayTransaction,
		prepared,
		verified,
	)
	if err != nil {
		t.Fatalf("source exact replay: %v", err)
	}
	if sourceReplay.Kind() != WriteExactReplay {
		t.Fatalf("source replay kind = %q, want exact_replay", sourceReplay.Kind())
	}
	if finish := replayTransaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit source replay: %v", finish.Err())
	}
}

func TestV43SourceRejectionRollsBackPreparationSavepoint(t *testing.T) {
	ctx := context.Background()
	storeDB, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "profile-authority-reject.sqlite"),
	)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = storeDB.Close() })
	database := storeDB.GetRawDB()
	database.SetMaxOpenConns(1)
	prepared := testPrepared(t, t.TempDir(), "rejected")
	seedProfileAuthoritySupport(t, database, prepared)
	store, err := Open(database)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	transaction := beginImmediateForTest(t, database)
	result, err := store.RecordPreparationAndSourceInTransaction(
		ctx,
		transaction,
		prepared,
		authority.VerifiedSpeechActSource{},
	)
	if err != nil {
		t.Fatalf("typed source rejection: %v", err)
	}
	if result.Kind() != WriteRejected {
		t.Fatalf("write kind = %q, want rejected", result.Kind())
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit outer transaction after rejection: %v", finish.Err())
	}
	for _, table := range []string{contentTable, preparationTable, "speech_acts"} {
		if count := countRows(t, database, table); count != 0 {
			t.Fatalf("%s rows after savepoint rejection = %d, want 0", table, count)
		}
	}
}

func assertClosureLoads(
	t *testing.T,
	database *sql.DB,
	closure profileauthority.Closure,
) {
	t.Helper()
	permission, _ := closure.Permission()
	permissionRef, _ := permission.Ref()
	permissionDigest, _ := permission.Digest()
	if _, err := LoadPermission(context.Background(), database, permissionRef, permissionDigest); err != nil {
		t.Fatalf("LoadPermission: %v", err)
	}
	effect, _ := closure.Effect()
	effectDigest, _ := effect.Digest()
	if _, err := LoadInstitutedEffect(context.Background(), database, effectDigest); err != nil {
		t.Fatalf("LoadInstitutedEffect: %v", err)
	}
	basis, _ := closure.Basis()
	basisRef, _ := basis.Ref()
	basisDigest, _ := basis.Digest()
	if _, err := LoadFourRefBasis(context.Background(), database, basisRef, basisDigest); err != nil {
		t.Fatalf("LoadFourRefBasis: %v", err)
	}
	if _, err := LoadClosure(context.Background(), database, basisRef, basisDigest); err != nil {
		t.Fatalf("LoadClosure: %v", err)
	}
}

func testPrepared(
	t *testing.T,
	rootRaw string,
	identity string,
) profileauthority.PreparedAuthorization {
	t.Helper()
	root := mustParse(t, rootRaw, authority.NewProjectRoot)
	contentRef := mustParse(
		t,
		"profile-authorization-content:"+identity,
		authority.NewAuthorizationContentRef,
	)
	authorRef := mustParse(
		t,
		"role-assignment:profile-author:"+identity,
		authority.NewRoleAssignmentRef,
	)
	authorDigest := testDigest(t, "author:"+identity)
	methodRef := mustParse(
		t,
		"method-description:profile-onboarding:"+identity,
		authority.NewMethodDescriptionRef,
	)
	methodDigest := testDigest(t, "method:"+identity)
	contractRef := mustParse(
		t,
		"method-contract:profile-onboarding:"+identity,
		authority.NewMethodContractRef,
	)
	contractDigest := testDigest(t, "contract:"+identity)
	classifier := mustParse(t, "classifier:v9:"+identity, authority.NewClassifierVersion)
	policyVersion := mustParse(t, "profile-policy:v9:"+identity, authority.NewPolicyVersion)
	futureSession := mustParse(
		t,
		"session:profile-onboarding:"+identity,
		authority.NewSessionRef,
	)
	validFrom := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	validity := mustWindow(t, validFrom, validFrom.Add(4*time.Hour))
	workWindow := mustWindow(t, validFrom.Add(time.Hour), validFrom.Add(3*time.Hour))
	basisWindow := mustWindow(t, validFrom.Add(30*time.Minute), validFrom.Add(3*time.Hour))
	singleUse := mustParse(t, "profile.single-use."+identity, authority.NewSingleUseKey)
	content, err := profileauthority.NewAuthorizationContentBuilder(contentRef, root).
		ForProfileAuthor(authorRef, authorDigest).
		ForMethod(methodRef, methodDigest, contractRef, contractDigest).
		WithVersions(classifier, policyVersion).
		InSession(futureSession).
		AllowWorkWithin(workWindow).
		AllowBasisObservationWithin(basisWindow).
		ValidWithin(validity).
		SingleUse(singleUse).
		Build()
	if err != nil {
		t.Fatalf("Build AuthorizationContent: %v", err)
	}
	prepared, err := profileauthority.NewPreparedAuthorizationBuilder(
		content,
		mustParse(t, "permission:profile-declaration:"+identity, authority.NewPermissionRef),
		mustParse(t, "speech-act:profile-declaration:"+identity, authority.NewSpeechActRef),
		mustParse(t, "carrier:terminal-capture:profile-authority:"+identity, authority.NewCarrierRef),
	).
		InSpeechActSession(mustParse(t, "session:profile-authority:"+identity, authority.NewSessionRef)).
		WithinClaimScope(mustParse(t, "claim-scope:profile-declaration:"+identity, authority.NewClaimScopeRef)).
		UnderEnactabilityPredicate(mustParse(t, "A-profile-admission:"+identity, profileauthority.NewEnactabilityPredicateRef)).
		WithAdjudication(
			mustParse(t, "E-profile-authorization:"+identity, profileauthority.NewEvidenceClaimRef),
			mustParse(t, "carrier-class:controlling-terminal:"+identity, profileauthority.NewCarrierClassRef),
		).
		VerifiedBy(
			mustParse(t, "verifier:profile-authority:"+identity, authority.NewVerifierIdentity),
			mustParse(t, "verifier-version:v1:"+identity, authority.NewVerifierVersion),
			mustParse(t, "verification-policy:profile-authority:"+identity, authority.NewVerificationPolicyRef),
			testDigest(t, "verification-policy:"+identity),
		).
		AsBasis(mustParse(t, "profile-authority-basis:"+identity, profileauthority.NewBasisRef)).
		Build()
	if err != nil {
		t.Fatalf("Build PreparedAuthorization: %v", err)
	}
	return prepared
}

func seedProfileAuthoritySupport(
	t *testing.T,
	database *sql.DB,
	prepared profileauthority.PreparedAuthorization,
) {
	t.Helper()
	content, _ := prepared.Content()
	authorRef, authorDigest, _ := content.ProfileAuthor()
	methodRef, methodDigest, _ := content.MethodDescription()
	contractRef, contractDigest, _ := content.MethodContract()
	sessionRef, _ := content.FutureWorkSession()
	validity, _ := content.AuthorizationValidity()
	validFrom := formatTime(validity.From().Add(-time.Hour))
	validUntil := formatTime(validity.Until().Add(time.Hour))
	recordedAt := formatTime(validity.From().Add(-2 * time.Hour))
	contextRef := "context:profile-onboarding"
	roleRef := "role:profile-author"
	systemRef := "system:haft-kernel"
	mustExec(t, database, `INSERT INTO profile_onboarding_method_descriptions (
		method_description_ref, described_method_ref, bounded_context_ref,
		source_revision, edition, required_role_ref, required_system_kind,
		state_plane_ref, affected_ref_kind, effect_witness_rule_ref,
		method_description_json, method_description_digest, recorded_at
	) VALUES (?, 'method:profile-onboarding', ?, 'fpf:revision', 'v1', ?,
		'U.System', 'state:profile', 'ProfileClassificationEpistemeV1',
		'rule:effect-witness', '{}', ?, ?)`,
		methodRef.String(), contextRef, roleRef, methodDigest.String(), recordedAt)
	mustExec(t, database, `INSERT INTO profile_onboarding_method_contracts (
		method_contract_ref, edition, method_description_ref,
		method_description_digest, bounded_context_ref,
		role_admission_policy_ref, system_admission_policy_ref,
		parameter_spec_set_digest, accepted_result_kinds_json,
		required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
		effect_state_witness_rule_ref, acceptance_standard_ref,
		acceptance_standard_edition, holder_equals_executed_within_rule_ref,
		method_contract_json, method_contract_digest, recorded_at
	) VALUES (?, 'v1', ?, ?, ?, 'policy:role-admission',
		'policy:system-admission', ?, '["CandidatePayloadProduced"]',
		'["work_interval"]', '["rule:coverage"]', 'rule:effect-witness',
		'acceptance:profile-onboarding', 'v1', 'rule:holder-is-executor',
		'{}', ?, ?)`,
		contractRef.String(), methodRef.String(), methodDigest.String(), contextRef,
		testDigest(t, "parameters").String(), contractDigest.String(), recordedAt)
	mustExec(t, database, `INSERT INTO profile_onboarding_executor_system_admissions (
		system_admission_ref, system_ref, admitted_system_kind,
		bounded_context_ref, governing_pattern_ref,
		identity_basis_kind, identity_basis_system_ref,
		identity_basis_kernel_identity, identity_basis_kernel_version,
		identity_basis_designation_ref, identity_basis_designation_digest,
		acting_eligibility_basis_ref, acting_eligibility_basis_digest,
		session_ref, valid_from, valid_until, method_description_ref,
		method_description_digest, method_contract_ref, method_contract_digest,
		system_admission_policy_ref, system_admission_json,
		system_admission_digest, recorded_at
	) VALUES ('system-admission:profile-authority', ?, 'U.System', ?, 'A.1',
		'kernel_owned', ?, 'haft-kernel', 'v9', '', '',
		'eligibility:profile-authority', ?, ?, ?, ?, ?, ?, ?, ?,
		'policy:system-admission', '{}', ?, ?)`,
		systemRef, contextRef, systemRef, testDigest(t, "eligibility").String(),
		sessionRef.String(), validFrom, validUntil, methodRef.String(), methodDigest.String(),
		contractRef.String(), contractDigest.String(), testDigest(t, "system-admission").String(), recordedAt)
	mustExec(t, database, `INSERT INTO profile_author_role_admissions (
		role_admission_ref, role_ref, bounded_context_ref, governing_pattern_ref,
		method_description_ref, method_description_digest, method_contract_ref,
		method_contract_digest, role_admission_policy_ref, role_admission_json,
		role_admission_digest, recorded_at
	) VALUES ('role-admission:profile-authority', ?, ?, 'A.2.1', ?, ?, ?, ?,
		'policy:role-admission', '{}', ?, ?)`,
		roleRef, contextRef, methodRef.String(), methodDigest.String(), contractRef.String(),
		contractDigest.String(), testDigest(t, "role-admission").String(), recordedAt)
	mustExec(t, database, `INSERT INTO profile_author_assignment_support_carriers (
		assignment_justification_ref, assignment_rule_ref,
		assignment_rule_statement, bounded_context_ref,
		system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest, assignment_from,
		assignment_until, method_contract_ref, method_contract_digest,
		assignment_justification_json, assignment_justification_digest,
		assignment_provenance_ref, provenance_justification_ref,
		provenance_justification_digest, session_ref, kernel_identity,
		kernel_version, runtime_identity, runtime_version,
		provenance_recorded_at, assignment_provenance_json,
		assignment_provenance_digest, recorded_at
	) VALUES ('assignment-justification:profile-authority', 'rule:assignment',
		'exact pre-existing assignment support', ?,
		'system-admission:profile-authority', ?,
		'role-admission:profile-authority', ?, ?, ?, ?, ?, '{}', ?,
		'assignment-provenance:profile-authority',
		'assignment-justification:profile-authority', ?, ?,
		'haft-kernel', 'v9', 'codex', 'v1', ?, '{}', ?, ?)`,
		contextRef, testDigest(t, "system-admission").String(),
		testDigest(t, "role-admission").String(), validFrom, validUntil,
		contractRef.String(), contractDigest.String(), testDigest(t, "assignment").String(),
		testDigest(t, "assignment").String(), sessionRef.String(), recordedAt,
		testDigest(t, "provenance").String(), recordedAt)
	mustExec(t, database, `INSERT INTO profile_author_role_assignments (
		role_assignment_ref, holder_system_ref, admitted_role_ref,
		bounded_context_ref, valid_from, valid_until,
		system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest,
		assignment_justification_ref, assignment_justification_digest,
		assignment_provenance_ref, assignment_provenance_digest,
		role_assignment_json, role_assignment_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, 'system-admission:profile-authority', ?,
		'role-admission:profile-authority', ?,
		'assignment-justification:profile-authority', ?,
		'assignment-provenance:profile-authority', ?, '{}', ?, ?)`,
		authorRef.String(), systemRef, roleRef, contextRef, validFrom, validUntil,
		testDigest(t, "system-admission").String(), testDigest(t, "role-admission").String(),
		testDigest(t, "assignment").String(), testDigest(t, "provenance").String(),
		authorDigest.String(), recordedAt)
}

func beginImmediateForTest(
	t *testing.T,
	database *sql.DB,
) *sqlitetransaction.Transaction {
	t.Helper()
	transaction, err := sqlitetransaction.BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	return transaction
}

func countRows(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func mustExec(t *testing.T, database *sql.DB, query string, arguments ...any) {
	t.Helper()
	if _, err := database.Exec(query, arguments...); err != nil {
		t.Fatalf("seed profile authority support: %v", err)
	}
}

func testDigest(t *testing.T, seed string) authority.Digest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	digest, err := authority.NewDigest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("digest fixture: %v", err)
	}
	return digest
}

func mustParse[T any](t *testing.T, raw string, parse func(string) (T, error)) T {
	t.Helper()
	value, err := parse(raw)
	if err != nil {
		t.Fatalf("parse fixture %q: %v", raw, err)
	}
	return value
}

func mustWindow(
	t *testing.T,
	from time.Time,
	until time.Time,
) authority.TimeWindow {
	t.Helper()
	window, err := authority.NewTimeWindow(from, until)
	if err != nil {
		t.Fatalf("time window fixture: %v", err)
	}
	return window
}
