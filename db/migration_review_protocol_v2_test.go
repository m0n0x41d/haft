package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationReviewProtocolV2InstallsCanonicalTablesAndTriggers(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "review-v2.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	for _, table := range migrationReviewProtocolV2Tables {
		assertSQLiteObjectExists(t, store.conn, "table", table)
	}
	for _, trigger := range []string{
		"migration_review_admissions_v2_exact_sources",
		"migration_review_instituted_effects_exact_sources",
		"migration_review_acceptance_contents_no_replace",
		"migration_review_acceptance_contents_no_update",
		"migration_review_acceptance_contents_no_delete",
		"migration_review_admissions_v2_no_replace",
		"migration_review_admissions_v2_no_update",
		"migration_review_admissions_v2_no_delete",
		"migration_review_instituted_effects_no_replace",
		"migration_review_instituted_effects_no_update",
		"migration_review_instituted_effects_no_delete",
		"migration_review_acceptance_contents_project_ledger_root",
		"migration_review_admissions_v2_project_ledger_root",
		"migration_review_instituted_effects_project_ledger_root",
	} {
		assertSQLiteObjectExists(t, store.conn, "trigger", trigger)
	}
	for _, legacy := range []string{"migration_review_speech_acts", "migration_review_admissions"} {
		var count int
		if err := store.conn.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(legacy)).Scan(&count); err != nil {
			t.Fatalf("inspect historical table %s: %v", legacy, err)
		}
		if count != 0 {
			t.Fatalf("historical table %s contains %d new row(s)", legacy, count)
		}
	}
}

func TestMigrationReviewProtocolV2RejectsUnknownPartialFootprint(t *testing.T) {
	database := openDatabaseBeforeMigration39(t)
	defer database.Close()

	_, err := database.Exec(`CREATE TABLE migration_review_acceptance_contents (unknown TEXT)`)
	if err != nil {
		t.Fatalf("seed unknown partial v39 footprint: %v", err)
	}
	err = Migrate(database, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "unknown partial schema") {
		t.Fatalf("partial v39 footprint error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 39)
}

func TestMigrationReviewProtocolV2RootGuardAndAppendOnlyContent(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "review-v2-root.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	insertProjectLedgerBindingForReviewV2(t, store.conn, "/project/one")
	foreign := reviewV2ContentFixture("/project/two", "foreign")
	if err := insertReviewV2Content(store.conn, foreign); err == nil || !strings.Contains(err.Error(), "bound project ledger root") {
		t.Fatalf("foreign-root review content error = %v", err)
	}

	bound := reviewV2ContentFixture("/project/one", "bound")
	if err := insertReviewV2Content(store.conn, bound); err != nil {
		t.Fatalf("insert exact-root review content: %v", err)
	}
	if _, err := store.conn.Exec(
		"UPDATE migration_review_acceptance_contents SET source_carrier = 'changed' WHERE review_content_ref = ?",
		bound.ref,
	); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("review content update error = %v", err)
	}
	if _, err := store.conn.Exec(
		"DELETE FROM migration_review_acceptance_contents WHERE review_content_ref = ?",
		bound.ref,
	); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("review content delete error = %v", err)
	}
}

func TestMigrationReviewProtocolV2AdmissionRequiresGenericExactSources(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "review-v2-exact.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	content := reviewV2ContentFixture("/project/review", "exact")
	if err := insertReviewV2Content(store.conn, content); err != nil {
		t.Fatalf("insert review content: %v", err)
	}
	admissionJSON, err := json.Marshal(map[string]any{
		"schema":                 "haft.spec-migration-v2.semantic-review-admission/v2",
		"admission_ref":          "review-admission:v2:exact",
		"project_root":           content.root,
		"packet_carrier_digest":  content.packetCarrierDigest,
		"review_content_ref":     content.ref,
		"review_content_digest":  content.digest,
		"review_digest":          reviewV2Digest("review"),
		"capture_carrier_ref":    "carrier:terminal-capture:migration-review:exact",
		"capture_carrier_digest": reviewV2Digest("capture"),
		"speech_act_ref":         "speech-act:migration-review:exact",
		"speech_act_digest":      reviewV2Digest("speech"),
		"admitted_at":            "2026-07-15T08:00:00Z",
	})
	if err != nil {
		t.Fatalf("encode admission fixture: %v", err)
	}
	_, err = store.conn.Exec(`INSERT INTO migration_review_admissions_v2 (
		admission_ref, admission_digest, project_root, packet_carrier_digest,
		review_content_ref, review_content_digest, review_digest,
		capture_carrier_ref, capture_carrier_digest,
		speech_act_ref, speech_act_digest,
		admission_json, admitted_at, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"review-admission:v2:exact",
		reviewV2Digest("admission"),
		content.root,
		content.packetCarrierDigest,
		content.ref,
		content.digest,
		reviewV2Digest("review"),
		"carrier:terminal-capture:migration-review:exact",
		reviewV2Digest("capture"),
		"speech-act:migration-review:exact",
		reviewV2Digest("speech"),
		string(admissionJSON),
		"2026-07-15T08:00:00Z",
		"2026-07-15T08:00:00Z",
	)
	if err == nil {
		t.Fatal("review admission was stored without generic capture and SpeechAct sources")
	}
	var count int
	if scanErr := store.conn.QueryRow("SELECT COUNT(*) FROM migration_review_admissions_v2").Scan(&count); scanErr != nil {
		t.Fatalf("count rejected admissions: %v", scanErr)
	}
	if count != 0 {
		t.Fatalf("rejected review admission left %d row(s)", count)
	}
}

func TestMigrationReviewProtocolV2TriggerPinsCanonicalReviewTextAndProtocol(t *testing.T) {
	database := openDatabaseBeforeMigration40(t)
	defer database.Close()

	var triggerSQL string
	err := database.QueryRow(`SELECT sql FROM sqlite_master
		WHERE type = 'trigger' AND name = 'migration_review_admissions_v2_exact_sources'`).Scan(&triggerSQL)
	if err != nil {
		t.Fatalf("read migration-review exact-source trigger: %v", err)
	}
	required := []string{
		"capture.review_text = NEW.review_text",
		"NEW.context_policy_ref = 'context-policy:migration-review-acceptance:v1'",
		"NEW.act_type_ref = 'speech-act-type:accept'",
		"NEW.method_ref = 'method:migration-review-acceptance'",
		"NEW.method_description_ref = 'method-description:migration-review-acceptance:v1'",
		"NEW.bounded_context_ref = 'bounded-context:haft-spec-migration-v2'",
		"NEW.institutional_effect_rule_ref = 'institution-rule:accept-institutes-migration-review-admission:v1'",
		"policy.context_policy_digest = NEW.context_policy_digest",
		"policy.recognized_act_type_ref = NEW.act_type_ref",
		"policy.institutional_effect_rule_ref = NEW.institutional_effect_rule_ref",
		"method.method_description_digest = NEW.method_description_digest",
		"method.procedure_ref = 'procedure:review-exact-intent-capture-controlling-terminal:v1'",
		"method.bounded_context_ref = 'bounded-context:haft-spec-migration-v2'",
	}
	for _, fragment := range required {
		if !strings.Contains(triggerSQL, fragment) {
			t.Fatalf("migration-review exact-source trigger omitted %q", fragment)
		}
	}
}

type reviewV2ContentRow struct {
	ref                 string
	digest              string
	root                string
	packetDigest        string
	packetCarrierDigest string
	auditDigest         string
	sourceDigest        string
	zeroPassDigest      string
	canonicalJSON       string
}

func reviewV2ContentFixture(root string, token string) reviewV2ContentRow {
	row := reviewV2ContentRow{
		ref:                 "review-content:migration-v2:" + token,
		digest:              reviewV2Digest("content-" + token),
		root:                root,
		packetDigest:        reviewV2Digest("packet-" + token),
		packetCarrierDigest: reviewV2Digest("carrier-" + token),
		auditDigest:         reviewV2Digest("audit-" + token),
		sourceDigest:        reviewV2Digest("source-" + token),
		zeroPassDigest:      reviewV2Digest("zero-" + token),
	}
	canonical, _ := json.Marshal(map[string]any{
		"schema":                     "haft.spec-migration-v2.review-acceptance-content/v2",
		"review_content_ref":         row.ref,
		"project_root":               row.root,
		"packet_digest":              row.packetDigest,
		"packet_carrier_digest":      row.packetCarrierDigest,
		"partition_audit_schema":     "haft.spec-migration-v2.packet-partition-audit/v1",
		"partition_audit_status":     "verified",
		"partition_audit_digest":     row.auditDigest,
		"source_carrier":             "spec/enabling-system.md",
		"source_digest":              row.sourceDigest,
		"target_carrier_digests":     []any{},
		"fpf_revision":               strings.Repeat("a", 40),
		"semantic_zero_pass_carrier": ".context/semantic-review.md",
		"semantic_zero_pass_digest":  row.zeroPassDigest,
		"lifecycle_intent":           []any{},
	})
	row.canonicalJSON = string(canonical)
	return row
}

func insertReviewV2Content(database *sql.DB, row reviewV2ContentRow) error {
	_, err := database.Exec(`INSERT INTO migration_review_acceptance_contents (
		review_content_ref, review_content_digest, project_root,
		packet_digest, packet_carrier_digest,
		partition_audit_schema, partition_audit_status, partition_audit_digest,
		source_carrier, source_digest, target_carrier_digests_json,
		fpf_revision, semantic_zero_pass_carrier, semantic_zero_pass_digest,
		lifecycle_intent_json, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ref,
		row.digest,
		row.root,
		row.packetDigest,
		row.packetCarrierDigest,
		"haft.spec-migration-v2.packet-partition-audit/v1",
		"verified",
		row.auditDigest,
		"spec/enabling-system.md",
		row.sourceDigest,
		"[]",
		strings.Repeat("a", 40),
		".context/semantic-review.md",
		row.zeroPassDigest,
		"[]",
		row.canonicalJSON,
		"2026-07-15T08:00:00Z",
	)
	return err
}

func insertProjectLedgerBindingForReviewV2(t *testing.T, database *sql.DB, root string) {
	t.Helper()
	boundAt := "2026-07-15T08:00:00Z"
	canonical, err := json.Marshal(map[string]any{
		"schema":       "haft.project-ledger-binding/v1",
		"project_id":   "qnt_1234abcd",
		"project_root": root,
		"bound_at":     boundAt,
	})
	if err != nil {
		t.Fatalf("encode project-ledger binding: %v", err)
	}
	_, err = database.Exec(`INSERT INTO project_ledger_binding (
		binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
	) VALUES (1, ?, ?, ?, ?, ?)`,
		"qnt_1234abcd",
		root,
		reviewV2Digest("ledger"),
		string(canonical),
		boundAt,
	)
	if err != nil {
		t.Fatalf("insert project-ledger binding: %v", err)
	}
}

func openDatabaseBeforeMigration39(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre-v39.db")
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build pre-v39 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v39 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 39, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through v38: %v", err)
	}
	return database
}

func assertSQLiteObjectExists(t *testing.T, database *sql.DB, kind string, name string) {
	t.Helper()
	var count int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("inspect %s %s: %v", kind, name, err)
	}
	if count != 1 {
		t.Fatalf("%s %s count = %d", kind, name, count)
	}
}

func assertMigrationVersionAbsent(t *testing.T, database *sql.DB, version int) {
	t.Helper()
	var count int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = ?",
		version,
	).Scan(&count)
	if err != nil {
		t.Fatalf("inspect migration version %d: %v", version, err)
	}
	if count != 0 {
		t.Fatalf("failed migration version %d was recorded", version)
	}
}

func reviewV2Digest(seed string) string {
	value := seed
	for len(value) < 64 {
		value += seed
	}
	return "sha256:" + value[:64]
}
