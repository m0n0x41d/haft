package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSemanticSpeechActUtteranceMigrationPreservesLegacyPolicies(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeMigration40(t)
	defer database.Close()

	legacy := semanticUtterancePolicyFixture(
		"context-policy:test:legacy",
		"review_digest",
		"",
	)
	insertLegacySemanticUtterancePolicy(t, database, legacy)

	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("migrate through v40: %v", err)
	}
	var literal string
	err := database.QueryRow(
		"SELECT utterance_literal FROM speech_act_context_policies WHERE context_policy_ref = ?",
		legacy.ref,
	).Scan(&literal)
	if err != nil {
		t.Fatalf("read preserved legacy policy: %v", err)
	}
	if literal != "" {
		t.Fatalf("legacy utterance literal = %q, want empty", literal)
	}
	assertMigrationVersionPresent(t, database, 40)
}

func TestSemanticSpeechActUtterancePolicyRequiresExactLiteralInCanonicalJSON(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "literal-policy.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	valid := semanticUtterancePolicyFixture(
		"context-policy:test:literal",
		"literal",
		"HAFT AS SOFTWARE PROJECT",
	)
	insertSemanticUtterancePolicy(t, store.conn, valid)

	tampered := semanticUtterancePolicyFixture(
		"context-policy:test:tampered",
		"literal",
		"ANOTHER ACTION",
	)
	tampered.literal = "HAFT AS SOFTWARE PROJECT"
	_, err = store.conn.Exec(semanticUtterancePolicyInsertSQL(), tampered.arguments()...)
	if err == nil || !strings.Contains(err.Error(), "utterance binding and literal disagree") {
		t.Fatalf("tampered literal policy error = %v", err)
	}
}

type semanticUtterancePolicyRow struct {
	ref       string
	binding   string
	literal   string
	canonical string
}

func semanticUtterancePolicyFixture(
	ref string,
	binding string,
	literal string,
) semanticUtterancePolicyRow {
	value := map[string]string{
		"schema":                        "haft.authority.speech-act-context-policy/v1",
		"ref":                           ref,
		"bounded_context_ref":           "bounded-context:test",
		"recognized_act_type_ref":       "speech-act-type:authorize",
		"authorizer_role_ref":           "role:terminal-authorizer",
		"admitted_holder_kind":          "U.System",
		"assignment_source_rule":        "observed-local-controlling-terminal-session/v1",
		"institutional_effect_rule_ref": "institution-rule:test",
		"instituted_object_kind":        "U.Commitment",
		"institutional_modality":        "MAY",
		"scoped_action":                 "profile.declare.from_onboarding_candidate",
		"utterance_description_ref":     "utterance:test",
		"utterance_verb":                "AUTHORIZE",
		"utterance_binding":             binding,
	}
	if literal != "" {
		value["utterance_literal"] = literal
	}
	canonical, _ := json.Marshal(value)
	return semanticUtterancePolicyRow{
		ref:       ref,
		binding:   binding,
		literal:   literal,
		canonical: string(canonical),
	}
}

func (row semanticUtterancePolicyRow) arguments() []any {
	return []any{
		row.ref,
		"sha256:" + strings.Repeat("a", 64),
		"bounded-context:test",
		"speech-act-type:authorize",
		"role:terminal-authorizer",
		"U.System",
		"observed-local-controlling-terminal-session/v1",
		"institution-rule:test",
		"U.Commitment",
		"MAY",
		"profile.declare.from_onboarding_candidate",
		"utterance:test",
		"AUTHORIZE",
		row.binding,
		row.literal,
		row.canonical,
		"2026-07-15T00:00:00Z",
	}
}

func semanticUtterancePolicyInsertSQL() string {
	return `INSERT INTO speech_act_context_policies (
		context_policy_ref, context_policy_digest, bounded_context_ref,
		recognized_act_type_ref, authorizer_role_ref, admitted_holder_kind,
		assignment_source_rule, institutional_effect_rule_ref,
		instituted_object_kind, institutional_modality, scoped_action,
		utterance_description_ref, utterance_verb, utterance_binding,
		utterance_literal, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
}

func insertSemanticUtterancePolicy(
	t testing.TB,
	database *sql.DB,
	row semanticUtterancePolicyRow,
) {
	t.Helper()
	_, err := database.Exec(semanticUtterancePolicyInsertSQL(), row.arguments()...)
	if err != nil {
		t.Fatalf("insert SpeechAct context policy: %v", err)
	}
}

func insertLegacySemanticUtterancePolicy(
	t testing.TB,
	database *sql.DB,
	row semanticUtterancePolicyRow,
) {
	t.Helper()
	statement := `INSERT INTO speech_act_context_policies (
		context_policy_ref, context_policy_digest, bounded_context_ref,
		recognized_act_type_ref, authorizer_role_ref, admitted_holder_kind,
		assignment_source_rule, institutional_effect_rule_ref,
		instituted_object_kind, institutional_modality, scoped_action,
		utterance_description_ref, utterance_verb, utterance_binding,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	arguments := row.arguments()
	legacyArguments := append(arguments[:14], arguments[15:]...)
	_, err := database.Exec(statement, legacyArguments...)
	if err != nil {
		t.Fatalf("insert legacy SpeechAct context policy: %v", err)
	}
}

func openDatabaseBeforeMigration40(t testing.TB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre-v40.db")
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build pre-v40 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v40 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 40, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through v39: %v", err)
	}
	return database
}

func assertMigrationVersionPresent(t testing.TB, database *sql.DB, version int) {
	t.Helper()
	var count int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = ?",
		version,
	).Scan(&count); err != nil {
		t.Fatalf("inspect schema version %d: %v", version, err)
	}
	if count != 1 {
		t.Fatalf("schema version %d count = %d", version, count)
	}
}
