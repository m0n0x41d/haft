package authority

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestLoadProfileAdmissionAuthorityForTransactionIsReadOnlyAndAdmitsNewUse(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	before := authorityMutationCounts(t, fixture.database)
	transaction := beginAuthorityReadTransaction(t, fixture.database)

	check, err := loadProfileAdmissionAuthorityForTransactionAt(
		context.Background(),
		transaction,
		fixture.request,
		fixture.now,
	)
	if err != nil {
		t.Fatalf("load transaction authority: %v", err)
	}
	if check.Kind() != ProfileAdmissionAuthoritySnapshotLoaded {
		t.Fatalf("check kind = %q, want snapshot_loaded", check.Kind())
	}
	snapshot, ok := check.Snapshot()
	if !ok {
		t.Fatal("loaded check did not expose its snapshot")
	}
	decision := snapshot.AssessUse(testDigest(t, 'e'))
	if decision.Kind() != ProfileAdmissionNewUseAdmitted {
		t.Fatalf("use decision kind = %q, want new_use_admitted", decision.Kind())
	}
	admitted, ok := decision.AdmittedUse()
	if !ok {
		t.Fatal("new-use decision did not expose its admitted view")
	}
	resolutionRef, ok := admitted.AuthorityResolutionID()
	if !ok || resolutionRef != fixture.authorityResolution.id {
		t.Fatalf("resolution ref = %+v, ok=%v", resolutionRef, ok)
	}
	rollbackAuthorityTransaction(t, transaction)
	after := authorityMutationCounts(t, fixture.database)
	if before != after {
		t.Fatalf("read-only loader mutated ledger: before=%v after=%v", before, after)
	}
}

func TestProfileAdmissionAuthoritySnapshotReplaysExactRecordedRequest(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	requestDigest := testDigest(t, 'e')
	recorded := testProfileAdmissionRecordedUse(t, fixture, requestDigest)
	snapshot := testProfileAdmissionAuthoritySnapshot(
		t,
		fixture,
		fixture.now.Add(3*time.Hour),
		[]Denial{{
			code:   DenialResolutionExpired,
			detail: "canonical authority resolution is expired",
		}},
		&recorded,
	)
	linkedUse, ok := snapshot.RecordedUse()
	if !ok || linkedUse.AdmissionRequestDigest() != requestDigest {
		t.Fatalf("recorded use = %+v, ok=%v", linkedUse, ok)
	}

	decision := snapshot.AssessUse(requestDigest)
	if decision.Kind() != ProfileAdmissionOriginalUse {
		t.Fatalf("use decision kind = %q, want original_use", decision.Kind())
	}
	original, ok := decision.OriginalUse()
	if !ok {
		t.Fatal("original-use decision did not expose its durable use")
	}
	if original.AuthorityUseRecordRef() != recorded.AuthorityUseRecordRef() {
		t.Fatalf("authority-use ref = %q, want %q", original.AuthorityUseRecordRef(), recorded.AuthorityUseRecordRef())
	}
	if original.CommittedResultRef() != recorded.CommittedResultRef() {
		t.Fatalf("committed-result ref = %q, want %q", original.CommittedResultRef(), recorded.CommittedResultRef())
	}
}

func TestProfileAdmissionAuthoritySnapshotRejectsDifferentRequestReplay(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	recorded := testProfileAdmissionRecordedUse(t, fixture, testDigest(t, 'e'))
	snapshot := testProfileAdmissionAuthoritySnapshot(
		t,
		fixture,
		fixture.now,
		nil,
		&recorded,
	)

	decision := snapshot.AssessUse(testDigest(t, '9'))
	if decision.Kind() != ProfileAdmissionUseNotAdmitted {
		t.Fatalf("use decision kind = %q, want not_admitted", decision.Kind())
	}
	assertProfileAdmissionUseDeniedWithCode(
		t,
		decision,
		DenialSingleUseAlreadyConsumed,
	)
}

func TestProfileAdmissionAuthoritySnapshotAppliesCurrentDenialToUnusedAuthority(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	snapshot := testProfileAdmissionAuthoritySnapshot(
		t,
		fixture,
		fixture.now.Add(3*time.Hour),
		[]Denial{{
			code:   DenialResolutionExpired,
			detail: "canonical authority resolution is expired",
		}},
		nil,
	)

	decision := snapshot.AssessUse(testDigest(t, 'e'))
	if decision.Kind() != ProfileAdmissionUseNotAdmitted {
		t.Fatalf("use decision kind = %q, want not_admitted", decision.Kind())
	}
	assertProfileAdmissionUseDeniedWithCode(
		t,
		decision,
		DenialResolutionExpired,
	)
}

func TestLoadProfileAdmissionAuthorityForTransactionDefersCurrentDenialUntilUseAssessment(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	judgementTime := fixture.now.Add(3 * time.Hour)
	transaction := beginAuthorityReadTransaction(t, fixture.database)

	check, err := loadProfileAdmissionAuthorityForTransactionAt(
		context.Background(),
		transaction,
		fixture.request,
		judgementTime,
	)
	if err != nil {
		t.Fatalf("load expired transaction authority: %v", err)
	}
	if check.Kind() != ProfileAdmissionAuthoritySnapshotLoaded {
		t.Fatalf("check kind = %q, want snapshot_loaded", check.Kind())
	}
	snapshot, ok := check.Snapshot()
	if !ok {
		t.Fatal("loaded check did not expose its snapshot")
	}
	decision := snapshot.AssessUse(testDigest(t, 'e'))
	if decision.Kind() != ProfileAdmissionUseNotAdmitted {
		t.Fatalf("use decision kind = %q, want not_admitted", decision.Kind())
	}
	assertProfileAdmissionUseDeniedWithCode(
		t,
		decision,
		DenialResolutionExpired,
	)
	rollbackAuthorityTransaction(t, transaction)
}

func TestLoadProfileAdmissionAuthorityForTransactionRejectsBindingMismatchBeforeSnapshot(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	request := fixture.request
	request.envelope.sessionRef = mustParse(t, NewSessionRef, "session:other")
	transaction := beginAuthorityReadTransaction(t, fixture.database)

	check, err := loadProfileAdmissionAuthorityForTransactionAt(
		context.Background(),
		transaction,
		request,
		fixture.now,
	)
	if err != nil {
		t.Fatalf("load mismatched transaction authority: %v", err)
	}
	if check.Kind() != ProfileAdmissionAuthorityNotAdmitted {
		t.Fatalf("check kind = %q, want not_admitted", check.Kind())
	}
	denial, ok := check.NotAdmitted()
	if !ok {
		t.Fatal("not-admitted check did not expose reasons")
	}
	assertNotAdmittedContainsCode(t, denial, DenialCanonicalRecordInvalid)
	rollbackAuthorityTransaction(t, transaction)
}

func TestLoadProfileAdmissionAuthorityForTransactionRejectsInvalidCapability(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	_, err := loadProfileAdmissionAuthorityForTransactionAt(
		context.Background(),
		&sqlitetransaction.Transaction{},
		fixture.request,
		fixture.now,
	)
	if err == nil {
		t.Fatal("invalid transaction capability unexpectedly accepted")
	}
}

func testProfileAdmissionAuthoritySnapshot(
	t *testing.T,
	fixture authorityFixture,
	judgementTime time.Time,
	currentDenials []Denial,
	recordedUse *ProfileAdmissionRecordedUse,
) ProfileAdmissionAuthoritySnapshot {
	t.Helper()
	state := profileAdmissionAuthoritySnapshotState{
		presentation:   Presentation{value: fixture.presentation},
		resolution:     fixture.authorityResolution,
		judgementTime:  judgementTime,
		currentDenials: cloneDenials(currentDenials),
		recordedUse:    recordedUse,
	}
	snapshot := ProfileAdmissionAuthoritySnapshot{state: &state}
	if !snapshot.valid() {
		t.Fatal("test profile-admission authority snapshot is invalid")
	}
	return snapshot
}

func testProfileAdmissionRecordedUse(
	t *testing.T,
	fixture authorityFixture,
	requestDigest Digest,
) ProfileAdmissionRecordedUse {
	t.Helper()
	envelopeDigest, err := fixture.envelope.Digest()
	if err != nil {
		t.Fatalf("digest envelope: %v", err)
	}
	projectBindingDigest, err := ProjectBindingDigest(
		fixture.envelope.actionKind,
		fixture.envelope.projectRoot,
	)
	if err != nil {
		t.Fatalf("digest project binding: %v", err)
	}
	value := ProfileAdmissionRecordedUse{
		useRef: mustParse(
			t,
			NewAuthorityUseRecordRef,
			"authority-use:profile-declaration:one",
		),
		authorityResolutionRef:    fixture.authorityResolution.id,
		authorityResolutionDigest: fixture.authorityResolution.digest,
		singleUseKey:              fixture.envelope.singleUseKey,
		actionKind:                fixture.envelope.actionKind,
		projectRoot:               fixture.envelope.projectRoot,
		projectBindingDigest:      projectBindingDigest,
		envelopeDigest:            envelopeDigest,
		authorityRecordRef:        fixture.presentation.id,
		authorityRecordDigest:     fixture.presentation.digest,
		admissionRequestDigest:    requestDigest,
		verifierIdentity:          fixture.authorityResolution.verifierIdentity,
		verifierVersion:           fixture.authorityResolution.verifierVersion,
		committedResultRef: mustParse(
			t,
			NewProfileDeclarationAdmissionRecordRef,
			"profile-admission:one",
		),
		committedResultDigest: testDigest(t, 'f'),
		consumedAt:            fixture.now,
	}
	if err := validateProfileDeclarationRecordedUse(
		value,
		fixture.presentation,
		fixture.authorityResolution,
		fixture.now,
	); err != nil {
		t.Fatalf("test recorded authority use is invalid: %v", err)
	}
	return value
}

func assertProfileAdmissionUseDeniedWithCode(
	t *testing.T,
	decision ProfileAdmissionUseDecision,
	code DenialCode,
) {
	t.Helper()
	denial, ok := decision.NotAdmitted()
	if !ok {
		t.Fatalf("decision did not expose not-admitted reasons: %+v", decision)
	}
	assertNotAdmittedContainsCode(t, denial, code)
}

func assertNotAdmittedContainsCode(
	t *testing.T,
	denial NotAdmitted,
	code DenialCode,
) {
	t.Helper()
	for _, reason := range denial.Reasons() {
		if reason.Code() == code {
			return
		}
	}
	t.Fatalf("denial does not contain code %q: %+v", code, denial.Reasons())
}

func beginAuthorityReadTransaction(
	t *testing.T,
	database *sql.DB,
) *sqlitetransaction.Transaction {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(
		context.Background(),
		database,
	)
	if err != nil {
		t.Fatalf("begin authority read transaction: %v", err)
	}
	return transaction
}

func rollbackAuthorityTransaction(
	t *testing.T,
	transaction *sqlitetransaction.Transaction,
) {
	t.Helper()
	result := transaction.Rollback(context.Background())
	if !result.Succeeded() {
		t.Fatalf("roll back authority read transaction: %v", result.Err())
	}
}
