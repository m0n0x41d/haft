package authority

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenSpeechActSourceWriterRejectsStaleMigration38CaptureSchema(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "stale-v38.db"))
	if err != nil {
		t.Fatalf("open stale v38 fixture: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`CREATE TABLE schema_version (version INTEGER PRIMARY KEY);
		INSERT INTO schema_version (version) VALUES (38), (40);
		CREATE TABLE speech_act_context_policies (
			context_policy_ref TEXT PRIMARY KEY,
			utterance_literal TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE terminal_capture_records (
			capture_carrier_ref TEXT PRIMARY KEY,
			observed_at TEXT NOT NULL
		)`)
	if err != nil {
		t.Fatalf("create stale v38 fixture: %v", err)
	}

	_, err = OpenSpeechActSourceWriter(database)
	if err == nil || !strings.Contains(err.Error(), "stale terminal-capture schema") {
		t.Fatalf("stale migration 38 schema error = %v", err)
	}
}

func TestOpenSpeechActSourceWriterRequiresSemanticUtteranceMigration40(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "missing-v40.db"))
	if err != nil {
		t.Fatalf("open missing v40 fixture: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`CREATE TABLE schema_version (version INTEGER PRIMARY KEY);
		INSERT INTO schema_version (version) VALUES (38)`)
	if err != nil {
		t.Fatalf("create missing v40 fixture: %v", err)
	}

	_, err = OpenSpeechActSourceWriter(database)
	if err == nil || !strings.Contains(err.Error(), "migration 40 is unavailable") {
		t.Fatalf("missing migration 40 error = %v", err)
	}
}
