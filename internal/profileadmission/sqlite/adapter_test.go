package sqlite

import (
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/authority"
)

func TestNewTransactionAdapterRequiresExactCanonicalSchema(t *testing.T) {
	if _, err := newAdapter(nil); err == nil {
		t.Fatal("nil database was accepted")
	}
	store, err := kerneldb.NewStore(filepath.Join(t.TempDir(), "haft.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	if _, err = newAdapter(database); err != nil {
		t.Fatalf("newAdapter: %v", err)
	}
	_, err = database.Exec("DROP TRIGGER project_profile_admissions_v3_revision_cas")
	if err != nil {
		t.Fatalf("drop required trigger: %v", err)
	}
	if _, err = newAdapter(database); err == nil {
		t.Fatal("adapter accepted schema without revision CAS trigger")
	}
}

func TestFinishEvidenceClassificationPreservesCleanupProof(t *testing.T) {
	statementFailure := errors.New("commit failed")
	cleanupFailure := errors.New("cleanup failed")
	rolledBack := transactionFinishEvidence{statementErr: statementFailure}
	if failedCommitPosture(rolledBack) != AdmissionDefinitelyNotCommitted {
		t.Fatal("successful cleanup after failed COMMIT was not classified definitely-not-committed")
	}
	ambiguous := transactionFinishEvidence{
		statementErr: statementFailure,
		cleanupErr:   cleanupFailure,
	}
	if failedCommitPosture(ambiguous) != AdmissionCommitOutcomeUnknown {
		t.Fatal("failed COMMIT plus failed cleanup was not classified unknown")
	}
	if !rollbackProvesNotCommitted(transactionFinishEvidence{}) {
		t.Fatal("successful ROLLBACK did not prove definitely-not-committed")
	}
	if rollbackProvesNotCommitted(ambiguous) {
		t.Fatal("failed ROLLBACK cleanup incorrectly proved definitely-not-committed")
	}
}

func TestLedgerHeadEvidenceRejectsMaxInt64History(t *testing.T) {
	evidence := ledgerHeadEvidence{
		revisionCount:  math.MaxInt64,
		minimum:        1,
		maximum:        math.MaxInt64,
		admissionCount: math.MaxInt64,
		exactUseCount:  math.MaxInt64,
	}
	empty, err := validateLedgerHeadEvidence(evidence)
	if err == nil {
		t.Fatal("MaxInt64 ledger head was accepted")
	}
	if empty {
		t.Fatal("MaxInt64 ledger head was classified as empty")
	}
}

func TestLedgerHeadEvidenceEnforcesExactHistoryValidationBoundary(t *testing.T) {
	atLimit := ledgerHeadEvidence{
		revisionCount:  exactHistoryValidationLimit,
		minimum:        1,
		maximum:        exactHistoryValidationLimit,
		admissionCount: exactHistoryValidationLimit,
		exactUseCount:  exactHistoryValidationLimit,
	}
	empty, err := validateLedgerHeadEvidence(atLimit)
	if err != nil || empty {
		t.Fatalf("exact validation boundary was rejected: empty=%t error=%v", empty, err)
	}
	aboveLimit := atLimit
	aboveLimit.revisionCount++
	aboveLimit.maximum++
	aboveLimit.admissionCount++
	aboveLimit.exactUseCount++
	empty, err = validateLedgerHeadEvidence(aboveLimit)
	if err == nil {
		t.Fatal("history above exact-validation boundary was accepted")
	}
	if empty {
		t.Fatal("history above exact-validation boundary was classified empty")
	}
}

func mustAuthorityWindow(t *testing.T, from time.Time, until time.Time) authority.TimeWindow {
	t.Helper()
	value, err := authority.NewTimeWindow(from, until)
	if err != nil {
		t.Fatalf("NewTimeWindow: %v", err)
	}
	return value
}

func mustValue[T any](
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
