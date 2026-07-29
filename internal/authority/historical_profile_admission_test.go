package authority

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func TestResolveHistoricalProfileAdmissionAuthorityV1ExactAndReadOnly(t *testing.T) {
	fixture := newHistoricalAuthorityFixture(t, authorityRowOverrides{})
	admissionID := "admission.historical.exact"
	insertAuthorityUse(t, fixture, "use-record.historical.exact", admissionID)
	before := authorityMutationCounts(t, fixture.database)
	transaction := beginAuthorityReadTransaction(t, fixture.database)
	admissionRef := mustParse(
		t,
		NewProfileDeclarationAdmissionRecordRef,
		admissionID,
	)
	proof, err := ResolveHistoricalProfileAdmissionAuthorityV1(
		context.Background(),
		transaction,
		admissionRef,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("load historical authority use: %v", err)
	}
	assertHistoricalAuthorityProof(t, proof, fixture, admissionRef)
	rollbackAuthorityTransaction(t, transaction)
	after := authorityMutationCounts(t, fixture.database)
	if before != after {
		t.Fatalf("historical authority load mutated ledger: before=%v after=%v", before, after)
	}
}

func TestResolveHistoricalProfileAdmissionAuthorityV1ResolvesExpiredHistory(t *testing.T) {
	historicalNow := time.Now().UTC().Add(-72 * time.Hour).Round(0)
	fixture := newHistoricalAuthorityFixtureAt(t, authorityRowOverrides{}, historicalNow)
	if !time.Now().UTC().After(fixture.envelope.authorizationValidityWindow.until) {
		t.Fatal("test permission history is not expired")
	}
	admissionID := "admission.historical.expired"
	insertAuthorityUse(t, fixture, "use-record.historical.expired", admissionID)
	transaction := beginAuthorityReadTransaction(t, fixture.database)
	admissionRef := mustParse(
		t,
		NewProfileDeclarationAdmissionRecordRef,
		admissionID,
	)
	proof, err := ResolveHistoricalProfileAdmissionAuthorityV1(
		context.Background(),
		transaction,
		admissionRef,
	)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("load expired historical authority use: %v", err)
	}
	assertHistoricalAuthorityProof(t, proof, fixture, admissionRef)
	rollbackAuthorityTransaction(t, transaction)
}

func TestResolveHistoricalProfileAdmissionAuthorityV1RejectsMissingAndDuplicate(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		fixture := newHistoricalAuthorityFixture(t, authorityRowOverrides{})
		transaction := beginAuthorityReadTransaction(t, fixture.database)
		admissionRef := mustParse(
			t,
			NewProfileDeclarationAdmissionRecordRef,
			"admission.historical.missing",
		)
		_, err := ResolveHistoricalProfileAdmissionAuthorityV1(
			context.Background(),
			transaction,
			admissionRef,
		)
		rollbackAuthorityTransaction(t, transaction)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("missing historical authority use = %v, want sql.ErrNoRows", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		fixture := newHistoricalAuthorityFixture(t, authorityRowOverrides{})
		admissionID := "admission.historical.duplicate"
		insertAuthorityUse(t, fixture, "use-record.historical.duplicate", admissionID)
		replaceAuthorityUseTableWithDuplicateRows(t, fixture.database)
		transaction := beginAuthorityReadTransaction(t, fixture.database)
		admissionRef := mustParse(
			t,
			NewProfileDeclarationAdmissionRecordRef,
			admissionID,
		)
		_, err := ResolveHistoricalProfileAdmissionAuthorityV1(
			context.Background(),
			transaction,
			admissionRef,
		)
		rollbackAuthorityTransaction(t, transaction)
		if err == nil || !strings.Contains(err.Error(), "matches 2 authority-use rows") {
			t.Fatalf("duplicate historical authority use = %v", err)
		}
	})
}

func TestResolveHistoricalProfileAdmissionAuthorityV1RejectsCorruption(t *testing.T) {
	tests := []struct {
		name      string
		trigger   string
		mutation  string
		wantError string
	}{
		{
			name:      "use crosslink",
			trigger:   "authority_uses_no_update",
			mutation:  `UPDATE authority_uses SET verifier_version = 'v-corrupt'`,
			wantError: "authority chain is invalid",
		},
		{
			name:      "presentation canonical digest",
			trigger:   "authority_presentations_no_update",
			mutation:  `UPDATE authority_presentations SET context_policy_digest = 'sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff'`,
			wantError: "presentation digest does not match canonical fields",
		},
		{
			name:      "resolution canonical digest",
			trigger:   "authority_resolution_records_no_update",
			mutation:  `UPDATE authority_resolution_records SET verifier_version = 'v-corrupt'`,
			wantError: "authority-resolution digest does not match canonical fields",
		},
	}
	runHistoricalAuthorityCorruptionCases(t, tests, 0)
}

func TestResolveHistoricalProfileAdmissionAuthorityV1TransactionLifecycle(t *testing.T) {
	fixture := newHistoricalAuthorityFixture(t, authorityRowOverrides{})
	admissionID := "admission.historical.lifecycle"
	insertAuthorityUse(t, fixture, "use-record.historical.lifecycle", admissionID)
	admissionRef := mustParse(
		t,
		NewProfileDeclarationAdmissionRecordRef,
		admissionID,
	)
	zero := &sqlitetransaction.Transaction{}
	_, err := ResolveHistoricalProfileAdmissionAuthorityV1(
		context.Background(),
		zero,
		admissionRef,
	)
	if !errors.Is(err, sqlitetransaction.ErrTransactionInvalid) {
		t.Fatalf("zero transaction = %v, want transaction-invalid", err)
	}
	finished := beginAuthorityReadTransaction(t, fixture.database)
	rollbackAuthorityTransaction(t, finished)
	_, err = ResolveHistoricalProfileAdmissionAuthorityV1(
		context.Background(),
		finished,
		admissionRef,
	)
	if !errors.Is(err, sqlitetransaction.ErrTransactionFinished) {
		t.Fatalf("finished transaction = %v, want transaction-finished", err)
	}
	var zeroProof HistoricalProfileAdmissionAuthorityProofV1
	if _, ok := zeroProof.RecordedUse(); ok {
		t.Fatal("zero historical proof exposed a recorded use")
	}
}

func runHistoricalAuthorityCorruptionCases(
	t *testing.T,
	tests []struct {
		name      string
		trigger   string
		mutation  string
		wantError string
	},
	index int,
) {
	t.Helper()
	if index == len(tests) {
		return
	}
	test := tests[index]
	t.Run(test.name, func(t *testing.T) {
		fixture := newHistoricalAuthorityFixture(t, authorityRowOverrides{})
		admissionID := "admission.historical.corrupt." + strings.ReplaceAll(test.name, " ", "-")
		insertAuthorityUse(t, fixture, "use-record.historical.corrupt."+strings.ReplaceAll(test.name, " ", "-"), admissionID)
		_, err := fixture.database.Exec("DROP TRIGGER " + test.trigger)
		if err != nil {
			t.Fatalf("drop corruption guard: %v", err)
		}
		_, err = fixture.database.Exec(test.mutation)
		if err != nil {
			t.Fatalf("seed historical corruption: %v", err)
		}
		transaction := beginAuthorityReadTransaction(t, fixture.database)
		admissionRef := mustParse(
			t,
			NewProfileDeclarationAdmissionRecordRef,
			admissionID,
		)
		_, err = ResolveHistoricalProfileAdmissionAuthorityV1(
			context.Background(),
			transaction,
			admissionRef,
		)
		rollbackAuthorityTransaction(t, transaction)
		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Fatalf("historical corruption error = %v, want containing %q", err, test.wantError)
		}
	})
	runHistoricalAuthorityCorruptionCases(t, tests, index+1)
}

func assertHistoricalAuthorityProof(
	t *testing.T,
	proof HistoricalProfileAdmissionAuthorityProofV1,
	fixture authorityFixture,
	admissionRef ProfileDeclarationAdmissionRecordRef,
) {
	t.Helper()
	useRef, ok := proof.AuthorityUseRecordRef()
	if !ok || !useRef.valid() {
		t.Fatalf("authority-use ref = %+v, ok=%v", useRef, ok)
	}
	resolutionRef, ok := proof.AuthorityResolutionID()
	if !ok || resolutionRef != fixture.authorityResolution.id {
		t.Fatalf("resolution ref = %+v, ok=%v", resolutionRef, ok)
	}
	resolutionDigest, ok := proof.AuthorityResolutionDigest()
	if !ok || resolutionDigest != fixture.authorityResolution.digest {
		t.Fatalf("resolution digest = %+v, ok=%v", resolutionDigest, ok)
	}
	basisRef, ok := proof.AuthorityBasisRef()
	if !ok || basisRef != fixture.presentation.id {
		t.Fatalf("authority-basis ref = %+v, ok=%v", basisRef, ok)
	}
	basisDigest, ok := proof.AuthorityBasisDigest()
	if !ok || basisDigest != fixture.presentation.digest {
		t.Fatalf("authority-basis digest = %+v, ok=%v", basisDigest, ok)
	}
	envelope, ok := proof.Envelope()
	if !ok || envelope.singleUseKey != fixture.envelope.singleUseKey {
		t.Fatalf("authority envelope = %+v, ok=%v", envelope, ok)
	}
	policyRef, ok := proof.VerificationPolicyRef()
	if !ok || policyRef != fixture.authorityResolution.verificationPolicyRef {
		t.Fatalf("verification-policy ref = %+v, ok=%v", policyRef, ok)
	}
	policyDigest, ok := proof.VerificationPolicyDigest()
	if !ok || policyDigest != fixture.authorityResolution.verificationPolicyDigest {
		t.Fatalf("verification-policy digest = %+v, ok=%v", policyDigest, ok)
	}
	resolutionWindow, ok := proof.ResolutionWindow()
	if !ok || resolutionWindow.From() != fixture.authorityResolution.resolvedAt ||
		resolutionWindow.Until() != fixture.authorityResolution.validUntil {
		t.Fatalf("resolution window = %+v, ok=%v", resolutionWindow, ok)
	}
	verifierIdentity, ok := proof.VerifierIdentity()
	if !ok || verifierIdentity != fixture.authorityResolution.verifierIdentity {
		t.Fatalf("verifier identity = %+v, ok=%v", verifierIdentity, ok)
	}
	verifierVersion, ok := proof.VerifierVersion()
	if !ok || verifierVersion != fixture.authorityResolution.verifierVersion {
		t.Fatalf("verifier version = %+v, ok=%v", verifierVersion, ok)
	}
	committedRef, ok := proof.CommittedResultRef()
	if !ok || committedRef != admissionRef {
		t.Fatalf("committed admission ref = %+v, ok=%v", committedRef, ok)
	}
	actionKind, ok := proof.ActionKind()
	if !ok || actionKind != fixture.envelope.actionKind {
		t.Fatalf("action kind = %+v, ok=%v", actionKind, ok)
	}
	projectRoot, ok := proof.ProjectRoot()
	if !ok || projectRoot != fixture.envelope.projectRoot {
		t.Fatalf("project root = %+v, ok=%v", projectRoot, ok)
	}
}

func replaceAuthorityUseTableWithDuplicateRows(t *testing.T, database *sql.DB) {
	t.Helper()
	statements := []string{
		"PRAGMA foreign_keys = OFF",
		"ALTER TABLE authority_uses RENAME TO authority_uses_guarded",
		"CREATE TABLE authority_uses AS SELECT * FROM authority_uses_guarded WHERE 0",
		"INSERT INTO authority_uses SELECT * FROM authority_uses_guarded",
		"INSERT INTO authority_uses SELECT * FROM authority_uses_guarded",
	}
	execHistoricalAuthorityStatements(t, database, statements, 0)
}

func execHistoricalAuthorityStatements(
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
		t.Fatalf("execute historical corruption statement %q: %v", statements[index], err)
	}
	execHistoricalAuthorityStatements(t, database, statements, index+1)
}
