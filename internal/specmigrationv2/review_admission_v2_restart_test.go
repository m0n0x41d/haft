package specmigrationv2

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/authority"
)

type v39PublicAdmissionClosure struct {
	audit    PacketPartitionAudit
	admitted AdmittedMigrationReview
}

func TestMigrationReviewAdmissionV2DurableRestartResolveCurrentForAudit(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	closure := admitV39PublicReviewClosure(t, fixture)
	database := reopenV39ReviewDatabaseThroughStore(t, fixture)
	service, err := NewReviewAdmissionService(database)
	if err != nil {
		t.Fatalf("NewReviewAdmissionService after restart: %v", err)
	}

	resolved, err := service.ResolveCurrentForAudit(
		context.Background(),
		fixture.carrier,
		closure.audit,
	)
	if err != nil {
		t.Fatalf("ResolveCurrentForAudit after restart: %v", err)
	}
	assertSameV39ReviewAdmission(t, closure.admitted, resolved)
}

func TestMigrationReviewAdmissionV2DurableRestartRejectsSelfConsistentReviewTextSubstitution(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	closure := admitV39PublicReviewClosure(t, fixture)
	tests := []struct {
		name      string
		statement string
		value     string
	}{
		{
			name: "arbitrary review text",
			statement: `UPDATE migration_review_admissions_v2
				SET review_text = ?, admission_json = json_set(admission_json, '$.review_text', ?)
				WHERE admission_ref = ?`,
			value: "self-consistent but arbitrary migration-review text",
		},
		{
			name: "alternate context policy",
			statement: `UPDATE migration_review_admissions_v2
				SET context_policy_ref = ?, admission_json = json_set(admission_json, '$.context_policy_ref', ?)
				WHERE admission_ref = ?`,
			value: "context-policy:generic-accept:v1",
		},
		{
			name: "alternate method",
			statement: `UPDATE migration_review_admissions_v2
				SET method_ref = ?, admission_json = json_set(admission_json, '$.method_ref', ?)
				WHERE admission_ref = ?`,
			value: "method:generic-accept",
		},
		{
			name: "alternate method description",
			statement: `UPDATE migration_review_admissions_v2
				SET method_description_ref = ?, admission_json = json_set(admission_json, '$.method_description_ref', ?)
				WHERE admission_ref = ?`,
			value: "method-description:generic-accept:v1",
		},
		{
			name: "alternate act type",
			statement: `UPDATE migration_review_admissions_v2
				SET act_type_ref = ?, admission_json = json_set(admission_json, '$.act_type_ref', ?)
				WHERE admission_ref = ?`,
			value: "speech-act-type:authorize",
		},
		{
			name: "alternate effect rule",
			statement: `UPDATE migration_review_admissions_v2
				SET institutional_effect_rule_ref = ?, admission_json = json_set(admission_json, '$.institutional_effect_rule_ref', ?)
				WHERE admission_ref = ?`,
			value: "institution-rule:accept-institutes-generic-review:v1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.database.Exec(
				test.statement,
				test.value,
				test.value,
				closure.admitted.ReviewRef().String(),
			)
			if err == nil || !strings.Contains(err.Error(), "append-only") {
				t.Fatalf("protocol substitution error = %v", err)
			}
		})
	}

	database := reopenV39ReviewDatabaseThroughStore(t, fixture)
	service, err := NewReviewAdmissionService(database)
	if err != nil {
		t.Fatalf("NewReviewAdmissionService after rejected substitutions: %v", err)
	}
	resolved, err := service.ResolveCurrentForAudit(
		context.Background(),
		fixture.carrier,
		closure.audit,
	)
	if err != nil {
		t.Fatalf("ResolveCurrentForAudit after rejected substitutions: %v", err)
	}
	assertSameV39ReviewAdmission(t, closure.admitted, resolved)
}

func TestMigrationReviewAdmissionV2RejectsArbitraryValidSourceReviewTextBeforeWrite(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	prepared, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission: %v", err)
	}
	arbitraryText := "self-consistent but arbitrary migration-review text"
	intent, ok := prepared.state.manualSource.Intent()
	if !ok {
		t.Fatal("prepared migration review omitted its SpeechAct intent")
	}
	arbitraryPrepared, err := authority.PrepareManualSpeechAct(intent, arbitraryText)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct arbitrary review: %v", err)
	}
	startedAt := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	observedAt := startedAt.Add(time.Nanosecond)
	endedAt := observedAt.Add(time.Nanosecond)
	source, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		arbitraryPrepared,
		startedAt,
		observedAt,
		endedAt,
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	fixture.service.now = func() time.Time { return endedAt.Add(time.Nanosecond) }

	_, err = fixture.service.Admit(context.Background(), prepared, source)
	if err == nil || (!strings.Contains(err.Error(), "review digest") &&
		!strings.Contains(err.Error(), "review text")) {
		t.Fatalf("arbitrary valid source review error = %v", err)
	}
	assertMigrationReviewTableCounts(t, fixture, 0, 0, 0, 0, 0)
	assertV39GenericSourceTableCounts(t, fixture.database, 0)
}

func admitV39PublicReviewClosure(
	t *testing.T,
	fixture reviewAdmissionFixture,
) v39PublicAdmissionClosure {
	t.Helper()
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
	fixture.service.now = func() time.Time { return endedAt.Add(time.Nanosecond) }
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
	admitted, err := fixture.service.Admit(
		context.Background(),
		prepared,
		source,
	)
	if err != nil {
		t.Fatalf("ReviewAdmissionService.Admit: %v", err)
	}
	return v39PublicAdmissionClosure{audit: audit, admitted: admitted}
}

func reopenV39ReviewDatabaseThroughStore(
	t *testing.T,
	fixture reviewAdmissionFixture,
) *sql.DB {
	t.Helper()
	if err := fixture.database.Close(); err != nil {
		t.Fatalf("close review database before restart: %v", err)
	}
	path := filepath.Join(fixture.root.String(), ".haft", "haft.db")
	store, err := kerneldb.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore after restart: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store.GetRawDB()
}

func assertSameV39ReviewAdmission(
	t *testing.T,
	want AdmittedMigrationReview,
	got AdmittedMigrationReview,
) {
	t.Helper()
	if got.ReviewRef().String() != want.ReviewRef().String() {
		t.Fatalf("resolved admission ref = %q, want %q", got.ReviewRef().String(), want.ReviewRef().String())
	}
	if !got.ReviewAdmissionDigest().Equal(want.ReviewAdmissionDigest()) {
		t.Fatal("restart resolved another admission digest")
	}
	if got.SpeechActRef().String() != want.SpeechActRef().String() ||
		!got.SpeechActDigest().Equal(want.SpeechActDigest()) {
		t.Fatal("restart resolved another instituting SpeechAct")
	}
}

func assertV39GenericSourceTableCounts(
	t *testing.T,
	database *sql.DB,
	want int,
) {
	t.Helper()
	tables := []string{
		"speech_act_method_descriptions",
		"speech_act_context_policies",
		"terminal_capture_records",
		"speech_act_role_assignments",
		"speech_acts",
	}
	for _, table := range tables {
		var got int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d", table, got, want)
		}
	}
}
