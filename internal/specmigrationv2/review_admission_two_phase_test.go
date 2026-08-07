package specmigrationv2

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestReviewAdmissionServicePhaseTwoFailureKeepsDurableSourceAndRetryCompletes(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	prepared, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission: %v", err)
	}
	startedAt := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	observedAt := startedAt.Add(time.Nanosecond)
	endedAt := observedAt.Add(time.Nanosecond)
	source, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		prepared.state.manualSource,
		startedAt,
		observedAt,
		endedAt,
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	fixture.service.now = func() time.Time { return endedAt.Add(time.Nanosecond) }
	_, err = fixture.database.Exec(`CREATE TRIGGER inject_migration_review_phase_two_failure
		BEFORE INSERT ON migration_review_acceptance_contents
		BEGIN
			SELECT RAISE(ABORT, 'injected migration-review phase-two failure');
		END`)
	if err != nil {
		t.Fatalf("create phase-two failure trigger: %v", err)
	}

	_, err = fixture.service.Admit(context.Background(), prepared, source)
	if err == nil || !strings.Contains(err.Error(), "injected migration-review phase-two failure") {
		t.Fatalf("injected phase-two failure error = %v", err)
	}
	assertV39GenericSourceTableCounts(t, fixture.database, 1)
	assertMigrationReviewTableCounts(t, fixture, 0, 0, 0, 0, 0)

	_, err = fixture.database.Exec("DROP TRIGGER inject_migration_review_phase_two_failure")
	if err != nil {
		t.Fatalf("drop phase-two failure trigger: %v", err)
	}
	database := reopenV39ReviewDatabaseThroughStore(t, fixture)
	service, err := NewReviewAdmissionService(database)
	if err != nil {
		t.Fatalf("NewReviewAdmissionService after phase-two failure: %v", err)
	}
	service.now = func() time.Time { return endedAt.Add(2 * time.Nanosecond) }
	fixture.database = database
	fixture.service = service

	admitted, err := service.Resume(context.Background(), prepared)
	if err != nil {
		t.Fatalf("retry migration-review admission: %v", err)
	}
	if admitted.SpeechActRef().String() != prepared.SpeechActRef() {
		t.Fatal("retry admitted another SpeechAct")
	}
	assertV39GenericSourceTableCounts(t, database, 1)
	assertMigrationReviewTableCounts(t, fixture, 0, 0, 1, 1, 1)
}
