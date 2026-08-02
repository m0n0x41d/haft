package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func TestV44ResolutionWriterIsSealedAfterV51AndAdmissionGateStaysClosed(
	t *testing.T,
) {
	ctx := context.Background()
	storeDB, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "profile-authority-v44.sqlite"),
	)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = storeDB.Close() })
	database := storeDB.GetRawDB()
	database.SetMaxOpenConns(1)
	clock := time.Date(2026, 7, 15, 12, 10, 0, 0, time.UTC)
	store, err := openWithClock(database, func() time.Time { return clock })
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	closure := stageClosureForResolutionTest(t, store, database, "v44-roundtrip")
	basis, _ := closure.Basis()
	basisRef, _ := basis.Ref()
	basisDigest, _ := basis.Digest()
	snapshot, err := store.PrepareClosureSnapshot(ctx, basisRef, basisDigest)
	if err != nil {
		t.Fatalf("PrepareClosureSnapshot: %v", err)
	}

	beforeResolution := beginImmediateForTest(t, database)
	_, _, err = store.ResolveForAdmission(ctx, beforeResolution, snapshot)
	if !errors.Is(err, ErrAuthorityResolutionRequired) {
		t.Fatalf("gate before pre-Work resolution error = %v", err)
	}
	if finish := beforeResolution.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback missing-resolution gate: %v", finish.Err())
	}

	write, err := store.InstituteResolution(ctx, snapshot)
	if err != nil {
		t.Fatalf("InstituteResolution: %v", err)
	}
	if write.Kind() != WriteRejected {
		t.Fatalf("resolution write kind = %q, want rejected", write.Kind())
	}
	detail, detailOK := write.RejectionDetail()
	if !detailOK || !strings.Contains(
		detail,
		"v2 profile admission writes are sealed after schema v51",
	) {
		t.Fatalf("resolution rejection detail = %q, %t", detail, detailOK)
	}
	resolutionCount := 0
	err = database.QueryRow(
		`SELECT COUNT(*)
		 FROM profile_declaration_authority_resolutions_v2
		 WHERE authority_basis_ref = ?`,
		basisRef.String(),
	).Scan(&resolutionCount)
	if err != nil {
		t.Fatalf("count v2 authority resolutions: %v", err)
	}
	if resolutionCount != 0 {
		t.Fatalf("sealed v2 resolution writer persisted %d row(s)", resolutionCount)
	}

	afterRejectedWrite := beginImmediateForTest(t, database)
	_, _, err = store.ResolveForAdmission(ctx, afterRejectedWrite, snapshot)
	if !errors.Is(err, ErrAuthorityResolutionRequired) {
		t.Fatalf("gate after rejected legacy write error = %v", err)
	}
	if finish := afterRejectedWrite.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("rollback post-rejection gate: %v", finish.Err())
	}
}

func stageClosureForResolutionTest(
	t *testing.T,
	store *Store,
	database *sql.DB,
	identity string,
) profileauthority.Closure {
	t.Helper()
	ctx := context.Background()
	prepared := testPrepared(t, t.TempDir(), identity)
	content, _ := prepared.Content()
	root, _ := content.ProjectRoot()
	boundAt := formatTime(time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC))
	bindingJSON := `{"schema":"haft.project-ledger-binding/v1","project_id":"qnt_a7f3b2c1","project_root":"` + root.String() + `","bound_at":"` + boundAt + `"}`
	mustExec(t, database, `INSERT INTO project_ledger_binding (
		binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
	) VALUES (1, 'qnt_a7f3b2c1', ?, ?, ?, ?)`,
		root.String(), testDigest(t, "ledger-binding:"+identity).String(), bindingJSON, boundAt)
	seedProfileAuthoritySupport(t, database, prepared)
	manual, ok := prepared.ManualSpeechAct()
	if !ok {
		t.Fatal("prepared authorization omitted manual SpeechAct")
	}
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
	result, err := store.RecordPreparationAndSourceInTransaction(
		ctx,
		transaction,
		prepared,
		verified,
	)
	if err != nil || result.Kind() != WriteStaged {
		t.Fatalf("RecordPreparationAndSourceInTransaction = %q, %v", result.Kind(), err)
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit preparation/source: %v", finish.Err())
	}
	preparedDigest, _ := prepared.Digest()
	closureWrite, err := store.InstituteClosure(ctx, preparedDigest)
	if err != nil {
		t.Fatalf("InstituteClosure: %v", err)
	}
	closure, ok := closureWrite.Closure()
	if !ok {
		t.Fatalf("InstituteClosure kind = %q omitted closure", closureWrite.Kind())
	}
	return closure
}
