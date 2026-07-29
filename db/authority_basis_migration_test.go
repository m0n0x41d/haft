package db

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuthorityBasisMigration38InstallsCanonicalClosure(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/v38.db")
	if err != nil {
		t.Fatalf("open v38 store: %v", err)
	}
	defer store.Close()

	assertSQLiteObjectsExist(t, store.conn, "table", []string{
		"speech_act_method_descriptions",
		"speech_act_context_policies",
		"profile_declaration_authorization_contents",
		"terminal_capture_records",
		"speech_act_role_assignments",
		"speech_acts",
		"profile_declaration_permissions",
		"speech_act_instituted_effects",
		"authority_basis_presentations",
		"authority_basis_resolutions",
	}, 0)
	assertSQLiteObjectsExist(t, store.conn, "trigger", []string{
		"profile_declaration_authorization_contents_exact_sources",
		"speech_act_role_assignments_exact_sources",
		"speech_acts_exact_sources",
		"profile_declaration_permissions_exact_instituting_sources",
		"speech_act_instituted_effects_exact_sources",
		"authority_basis_presentations_exact_graph",
		"authority_basis_resolutions_exact_presentation",
		"authority_presentations_require_v38_basis",
		"authority_resolution_records_require_v38_basis",
	}, 0)
	assertSQLiteObjectsExist(t, store.conn, "trigger", []string{
		"profile_declaration_authorization_contents_project_ledger_root",
		"terminal_capture_records_project_ledger_root",
		"speech_act_role_assignments_project_ledger_root",
		"speech_acts_project_ledger_root",
		"profile_declaration_permissions_project_ledger_root",
		"speech_act_instituted_effects_project_ledger_root",
		"authority_basis_presentations_project_ledger_root",
		"authority_basis_resolutions_project_ledger_root",
	}, 0)
	assertAuthorityBasisImmutabilityTriggers(t, store.conn, authorityBasisMigration38Tables, 0)

	forbiddenGenericTables := []string{
		"authorization_contents",
		"permissions",
		"instituted_effects",
		"basis_presentations",
		"basis_resolutions",
	}
	assertSQLiteObjectsAbsent(t, store.conn, "table", forbiddenGenericTables, 0)

	contentColumns := tableColumns(t, store.conn, "profile_declaration_authorization_contents")
	assertColumnsAbsent(t, contentColumns, []string{
		"speech_act_digest",
		"capture_carrier_digest",
		"work_record_ref",
		"payload_digest",
		"candidate_digest",
		"outcome_digest",
	}, 0)
	assertColumnsPresent(t, contentColumns, []string{
		"method_contract_ref",
		"method_contract_digest",
		"classifier_version",
		"policy_version",
		"allowed_work_from",
		"allowed_work_until",
		"basis_observation_from",
		"basis_observation_until",
		"authorization_valid_from",
		"authorization_valid_until",
		"single_use_key",
	}, 0)

	captureColumns := tableColumns(t, store.conn, "terminal_capture_records")
	assertColumnsPresent(t, captureColumns, []string{
		"prepared_speech_act_intent_digest",
		"review_text",
		"review_digest",
		"started_at",
		"exact_utterance_observed_at",
		"ended_at",
		"observed_session_material",
		"observation_nonce",
		"observation_digest",
		"observed_holder_system_ref",
		"observed_role_assignment_ref",
	}, 0)

	permissionColumns := tableColumns(t, store.conn, "profile_declaration_permissions")
	assertColumnsPresent(t, permissionColumns, []string{
		"claim_scope_ref",
		"referents_json",
		"adjudication_evidence_claim_refs_json",
		"adjudication_carrier_refs_json",
		"adjudication_evaluation_policy_ref",
		"adjudication_evaluation_policy_digest",
	}, 0)

	var versionCount int
	if err := store.conn.QueryRow("SELECT COUNT(*) FROM schema_version WHERE version = 38").Scan(&versionCount); err != nil {
		t.Fatalf("inspect v38 marker: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("v38 marker count = %d, want 1", versionCount)
	}
	assertNoForeignKeyViolationsV38(t, store.conn)
}

func TestAuthorityBasisMigration38AcceptsExactGenericSpeechActSources(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/source.db")
	if err != nil {
		t.Fatalf("open v38 source store: %v", err)
	}
	defer store.Close()

	contextRef := "bounded-context:haft-local-authority"
	err = insertGenericSpeechActSourceFixture(store.conn, "/tmp/project", contextRef, contextRef)
	if err != nil {
		t.Fatalf("insert exact generic SpeechAct source graph: %v", err)
	}
	assertTableRowCountV38(t, store.conn, "speech_act_method_descriptions", 1)
	assertTableRowCountV38(t, store.conn, "speech_act_context_policies", 1)
	assertTableRowCountV38(t, store.conn, "terminal_capture_records", 1)
	assertTableRowCountV38(t, store.conn, "speech_act_role_assignments", 1)
	assertTableRowCountV38(t, store.conn, "speech_acts", 1)

	if _, err := store.conn.Exec(`UPDATE speech_acts SET work_kind = 'Other'`); err == nil {
		t.Fatal("v38 SpeechAct accepted UPDATE")
	}
	if _, err := store.conn.Exec(`DELETE FROM terminal_capture_records`); err == nil {
		t.Fatal("v38 terminal capture accepted DELETE")
	}
}

func TestAuthorityBasisMigration38RejectsMethodContextMismatch(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/context-mismatch.db")
	if err != nil {
		t.Fatalf("open v38 mismatch store: %v", err)
	}
	defer store.Close()

	err = insertGenericSpeechActSourceFixture(
		store.conn,
		"/tmp/project",
		"bounded-context:another-authority",
		"bounded-context:haft-local-authority",
	)
	if err == nil || !strings.Contains(err.Error(), "MethodDescription") {
		t.Fatalf("SpeechAct accepted a MethodDescription from another bounded context: %v", err)
	}
	assertTableRowCountV38(t, store.conn, "speech_acts", 0)
}

func TestAuthorityBasisMigration38RejectsNonStrictTimeWindows(t *testing.T) {
	t.Run("authorization content allowed Work equal", func(t *testing.T) {
		store, err := NewStore(t.TempDir() + "/content-work-equal.db")
		if err != nil {
			t.Fatalf("open content-work-equal store: %v", err)
		}
		defer store.Close()
		insertTypedMemoryDAGFixture(t, store.conn)

		err = insertAuthorityContentFixture(
			store.conn,
			"2026-07-14T00:20:00Z",
			"2026-07-14T00:20:00Z",
			"2026-07-14T00:15:00Z",
			"2026-07-14T00:20:00Z",
		)
		if err == nil {
			t.Fatal("authorization content accepted an equal allowed Work window")
		}
	})

	t.Run("authorization content basis reversed", func(t *testing.T) {
		store, err := NewStore(t.TempDir() + "/content-basis-reversed.db")
		if err != nil {
			t.Fatalf("open content-basis-reversed store: %v", err)
		}
		defer store.Close()
		insertTypedMemoryDAGFixture(t, store.conn)

		err = insertAuthorityContentFixture(
			store.conn,
			"2026-07-14T00:20:00Z",
			"2026-07-14T00:30:00Z",
			"2026-07-14T00:20:00Z",
			"2026-07-14T00:15:00Z",
		)
		if err == nil {
			t.Fatal("authorization content accepted a reversed basis-observation window")
		}
	})

	t.Run("RoleAssignment reversed", func(t *testing.T) {
		store, err := NewStore(t.TempDir() + "/assignment-reversed.db")
		if err != nil {
			t.Fatalf("open assignment-reversed store: %v", err)
		}
		defer store.Close()

		err = insertGenericSpeechActSourceFixtureWithWindows(
			store.conn,
			"/tmp/project",
			"bounded-context:haft-local-authority",
			"bounded-context:haft-local-authority",
			"2026-07-15T10:00:01Z",
			"2026-07-15T10:00:00Z",
			"2026-07-15T10:00:00Z",
			"2026-07-15T10:00:01Z",
		)
		if err == nil {
			t.Fatal("RoleAssignment accepted a reversed validity window")
		}
		assertTableRowCountV38(t, store.conn, "speech_act_role_assignments", 0)
	})

	t.Run("SpeechAct equal", func(t *testing.T) {
		store, err := NewStore(t.TempDir() + "/speech-act-equal.db")
		if err != nil {
			t.Fatalf("open speech-act-equal store: %v", err)
		}
		defer store.Close()

		err = insertGenericSpeechActSourceFixtureWithWindows(
			store.conn,
			"/tmp/project",
			"bounded-context:haft-local-authority",
			"bounded-context:haft-local-authority",
			"2026-07-15T10:00:00Z",
			"2026-07-15T10:00:01Z",
			"2026-07-15T10:00:00Z",
			"2026-07-15T10:00:00Z",
		)
		if err == nil {
			t.Fatal("SpeechAct accepted an equal Work window")
		}
		assertTableRowCountV38(t, store.conn, "speech_act_role_assignments", 1)
		assertTableRowCountV38(t, store.conn, "speech_acts", 0)
	})
}

func TestAuthorityBasisMigration38PreservesNanosecondCaptureOrdering(t *testing.T) {
	accepted := []struct {
		name    string
		started string
		exact   string
		ended   string
	}{
		{
			name:    "one nanosecond boundaries",
			started: "2026-07-15T10:00:00.000000001Z",
			exact:   "2026-07-15T10:00:00.000000002Z",
			ended:   "2026-07-15T10:00:00.000000003Z",
		},
		{
			name:    "whole second to fractional",
			started: "2026-07-15T10:00:00Z",
			exact:   "2026-07-15T10:00:00.000000001Z",
			ended:   "2026-07-15T10:00:00.000000002Z",
		},
		{
			name:    "variable width fractions",
			started: "2026-07-15T10:00:00.1Z",
			exact:   "2026-07-15T10:00:00.11Z",
			ended:   "2026-07-15T10:00:00.111Z",
		},
	}
	for _, test := range accepted {
		t.Run("accepts "+test.name, func(t *testing.T) {
			store, err := NewStore(t.TempDir() + "/accepted.db")
			if err != nil {
				t.Fatalf("open accepted store: %v", err)
			}
			defer store.Close()
			err = insertTerminalCaptureFixtureAt(
				store.conn,
				"/tmp/project",
				test.started,
				test.exact,
				test.ended,
			)
			if err != nil {
				t.Fatalf("ordered nanosecond capture rejected: %v", err)
			}
		})
	}

	rejected := []struct {
		name    string
		started string
		exact   string
		ended   string
	}{
		{
			name:    "exact equals start",
			started: "2026-07-15T10:00:00.000000001Z",
			exact:   "2026-07-15T10:00:00.000000001Z",
			ended:   "2026-07-15T10:00:00.000000002Z",
		},
		{
			name:    "exact equals end",
			started: "2026-07-15T10:00:00.000000001Z",
			exact:   "2026-07-15T10:00:00.000000002Z",
			ended:   "2026-07-15T10:00:00.000000002Z",
		},
		{
			name:    "exact precedes start",
			started: "2026-07-15T10:00:00.000000002Z",
			exact:   "2026-07-15T10:00:00.000000001Z",
			ended:   "2026-07-15T10:00:00.000000003Z",
		},
		{
			name:    "noncanonical trailing fractional zero",
			started: "2026-07-15T10:00:00.100Z",
			exact:   "2026-07-15T10:00:00.2Z",
			ended:   "2026-07-15T10:00:00.3Z",
		},
		{
			name:    "noncanonical separator",
			started: "2026-07-15 10:00:00Z",
			exact:   "2026-07-15T10:00:00.1Z",
			ended:   "2026-07-15T10:00:00.2Z",
		},
		{
			name:    "too many fractional digits",
			started: "2026-07-15T10:00:00.0000000001Z",
			exact:   "2026-07-15T10:00:00.000000002Z",
			ended:   "2026-07-15T10:00:00.000000003Z",
		},
		{
			name:    "hour twenty four",
			started: "2026-07-15T24:00:00Z",
			exact:   "2026-07-16T00:00:00.1Z",
			ended:   "2026-07-16T00:00:00.2Z",
		},
		{
			name:    "minute sixty",
			started: "2026-07-15T10:60:00Z",
			exact:   "2026-07-15T11:00:00.1Z",
			ended:   "2026-07-15T11:00:00.2Z",
		},
		{
			name:    "second sixty",
			started: "2026-07-15T10:00:60Z",
			exact:   "2026-07-15T10:01:00.1Z",
			ended:   "2026-07-15T10:01:00.2Z",
		},
		{
			name:    "normalized invalid calendar date",
			started: "2026-02-30T10:00:00Z",
			exact:   "2026-03-02T10:00:00.1Z",
			ended:   "2026-03-02T10:00:00.2Z",
		},
	}
	for _, test := range rejected {
		t.Run("rejects "+test.name, func(t *testing.T) {
			store, err := NewStore(t.TempDir() + "/rejected.db")
			if err != nil {
				t.Fatalf("open rejected store: %v", err)
			}
			defer store.Close()
			err = insertTerminalCaptureFixtureAt(
				store.conn,
				"/tmp/project",
				test.started,
				test.exact,
				test.ended,
			)
			if err == nil {
				t.Fatal("nonordered or noncanonical capture was accepted")
			}
		})
	}
}

func TestAuthorityBasisMigration38TriggerBoundsPreserveNanoseconds(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/trigger-boundary.db")
	if err != nil {
		t.Fatalf("open trigger-boundary store: %v", err)
	}
	defer store.Close()
	_, err = store.conn.Exec(`CREATE TEMP TABLE nano_trigger_boundary (
		assignment_from TEXT NOT NULL,
		work_from TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create trigger boundary table: %v", err)
	}
	trigger := `CREATE TEMP TRIGGER nano_trigger_boundary_guard
		BEFORE INSERT ON nano_trigger_boundary
		WHEN NOT (` + sqliteUTCNanoLessOrEqual("NEW.assignment_from", "NEW.work_from") + `)
		BEGIN SELECT RAISE(ABORT, 'outside nanosecond bound'); END`
	if _, err = store.conn.Exec(trigger); err != nil {
		t.Fatalf("create nanosecond boundary trigger: %v", err)
	}
	_, err = store.conn.Exec(
		`INSERT INTO nano_trigger_boundary (assignment_from, work_from) VALUES (?, ?)`,
		"2026-07-14T00:10:00Z",
		"2026-07-14T00:10:00.000000001Z",
	)
	if err != nil {
		t.Fatalf("inside nanosecond trigger bound rejected: %v", err)
	}
	_, err = store.conn.Exec(
		`INSERT INTO nano_trigger_boundary (assignment_from, work_from) VALUES (?, ?)`,
		"2026-07-14T00:10:00Z",
		"2026-07-14T00:09:59.999999999Z",
	)
	if err == nil || !strings.Contains(err.Error(), "outside nanosecond bound") {
		t.Fatalf("outside nanosecond trigger bound error = %v", err)
	}
}

func TestAuthorityBasisMigration38RejectsForeignProjectRoot(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/root.db")
	if err != nil {
		t.Fatalf("open v38 root store: %v", err)
	}
	defer store.Close()

	boundAt := "2026-07-15T10:00:00Z"
	bindingJSON := mustAuthorityBasisJSON(t, map[string]any{
		"schema":       "haft.project-ledger-binding/v1",
		"project_id":   "qnt_a7f3b2c1",
		"project_root": "/tmp/bound",
		"bound_at":     boundAt,
	})
	_, err = store.conn.Exec(`INSERT INTO project_ledger_binding (
		binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
	) VALUES (1, ?, ?, ?, ?, ?)`,
		"qnt_a7f3b2c1",
		"/tmp/bound",
		authorityBasisTestDigest("1"),
		bindingJSON,
		boundAt,
	)
	if err != nil {
		t.Fatalf("insert project-ledger binding: %v", err)
	}
	err = insertTerminalCaptureFixture(store.conn, "/tmp/foreign")
	if err == nil || !strings.Contains(err.Error(), "bound project ledger root") {
		t.Fatalf("terminal capture accepted foreign project root: %v", err)
	}
}

func TestAuthorityBasisMigration38AllowsRepeatedObservedTerminalMaterial(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/repeated-observation.db")
	if err != nil {
		t.Fatalf("open repeated-observation store: %v", err)
	}
	defer store.Close()
	if err := insertTerminalCaptureFixture(store.conn, "/tmp/project"); err != nil {
		t.Fatalf("insert first capture: %v", err)
	}

	preparedDigest := authorityBasisTestDigest("b")
	reviewDigest := authorityBasisTestDigest("c")
	observationDigest := authorityBasisTestDigest("7")
	secondJSON := mustAuthorityBasisJSON(t, map[string]any{
		"schema":                            "haft.authority.terminal-capture/v1",
		"carrier_ref":                       "carrier:terminal-capture:second",
		"project_root":                      "/tmp/project",
		"prepared_speech_act_intent_digest": preparedDigest,
		"review_text":                       "second exact reviewed authority intent",
		"review_digest":                     reviewDigest,
		"canonical_utterance":               "AUTHORIZE " + reviewDigest,
		"started_at":                        "2026-07-15T10:01:00Z",
		"exact_utterance_observed_at":       "2026-07-15T10:01:00.5Z",
		"ended_at":                          "2026-07-15T10:01:01Z",
		"session_ref":                       "session:prepared-intent:second",
		"observed_session_material":         "path=/dev/tty;mode=Dcrw--w----;pid=1;ppid=0",
		"observation_nonce":                 strings.Repeat("b", 32),
		"observation_digest":                observationDigest,
		"observed_holder_system_ref":        "system:local-terminal-session:second",
		"observed_role_assignment_ref":      "role-assignment:terminal-authorizer:second",
	})
	_, err = store.conn.Exec(`INSERT INTO terminal_capture_records (
		capture_carrier_ref, capture_carrier_digest, project_root,
		prepared_speech_act_intent_digest, review_text, review_digest,
		canonical_utterance, started_at, exact_utterance_observed_at, ended_at,
		intent_session_ref,
		observed_session_material, observation_nonce, observation_digest,
		observed_holder_system_ref, observed_role_assignment_ref,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"carrier:terminal-capture:second",
		authorityBasisTestDigest("d"),
		"/tmp/project",
		preparedDigest,
		"second exact reviewed authority intent",
		reviewDigest,
		"AUTHORIZE "+reviewDigest,
		"2026-07-15T10:01:00Z",
		"2026-07-15T10:01:00.5Z",
		"2026-07-15T10:01:01Z",
		"session:prepared-intent:second",
		"path=/dev/tty;mode=Dcrw--w----;pid=1;ppid=0",
		strings.Repeat("b", 32),
		observationDigest,
		"system:local-terminal-session:second",
		"role-assignment:terminal-authorizer:second",
		secondJSON,
		"2026-07-15T10:01:01Z",
	)
	if err != nil {
		t.Fatalf("second capture with repeated terminal observation was rejected: %v", err)
	}
	assertTableRowCountV38(t, store.conn, "terminal_capture_records", 2)
}

func TestAuthorityBasisMigration38RejectsNonEmptyLegacyAuthorityAtomically(t *testing.T) {
	store := newStoreBeforeAuthoritySourceMigration(t)
	defer store.Close()
	insertTypedMemoryDAGFixture(t, store.conn)
	if err := insertTypedMemoryAuthorityFixture(store.conn, "session:test"); err != nil {
		t.Fatalf("insert pre-v38 authority fixture: %v", err)
	}

	err := Migrate(store.conn, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "authority_presentations contains 1 row") {
		t.Fatalf("v38 upgraded nonempty presentation-only authority: %v", err)
	}
	assertMigration38Uncommitted(t, store.conn)
}

func TestAuthorityBasisMigration38RejectsUnknownHybridAtomically(t *testing.T) {
	store := newStoreBeforeAuthoritySourceMigration(t)
	defer store.Close()
	if _, err := store.conn.Exec(`CREATE TABLE speech_acts (unknown_column TEXT)`); err != nil {
		t.Fatalf("install unknown hybrid table: %v", err)
	}

	err := Migrate(store.conn, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "unversioned v38 table speech_acts") {
		t.Fatalf("v38 accepted unknown hybrid schema: %v", err)
	}
	assertMigration38Uncommitted(t, store.conn)
}

func TestAuthorityBasisMigration38BlocksLegacyPresentationWithoutClosure(t *testing.T) {
	database := openDatabaseBeforeMigration39(t)
	defer database.Close()
	insertTypedMemoryDAGFixture(t, database)

	err := insertTypedMemoryAuthorityPresentationFixture(
		database,
		"session:test",
		"digest:assignment",
		"digest:method-description",
		"digest:contract",
	)
	if err == nil || !strings.Contains(err.Error(), "exact v38 authority basis closure") {
		t.Fatalf("legacy presentation bypassed v38 source closure: %v", err)
	}
}

func TestAuthorityBasisMigration38UsesPostActPermissionStartInLegacyProjection(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/permission-window.db")
	if err != nil {
		t.Fatalf("open permission-window store: %v", err)
	}
	defer store.Close()

	var triggerSQL string
	err = store.conn.QueryRow(`SELECT sql FROM sqlite_master
		WHERE type = 'trigger' AND name = 'authority_presentations_exact_assignment_method'`).Scan(&triggerSQL)
	if err != nil {
		t.Fatalf("read v38 compatibility trigger: %v", err)
	}
	forbidden := "NEW.permission_valid_from = NEW.valid_from"
	if strings.Contains(triggerSQL, forbidden) {
		t.Fatalf("v38 compatibility trigger retains pre-act equality %q", forbidden)
	}
	if strings.Contains(triggerSQL, "julianday") {
		t.Fatalf("v38 compatibility trigger uses precision-losing julianday ordering: %s", triggerSQL)
	}
	for _, required := range []string{
		"instr(NEW.valid_from, '.')",
		"instr(NEW.permission_valid_from, '.')",
		"instr(NEW.permission_valid_until, '.')",
		"NEW.permission_valid_until = NEW.valid_until",
	} {
		if !strings.Contains(triggerSQL, required) {
			t.Fatalf("v38 compatibility trigger is missing %q: %s", required, triggerSQL)
		}
	}
}

func insertAuthorityContentFixture(
	connection *sql.DB,
	allowedWorkFrom string,
	allowedWorkUntil string,
	basisObservationFrom string,
	basisObservationUntil string,
) error {
	contentJSON, err := authorityBasisJSON(map[string]any{
		"schema":                                "haft.authority.profile-declaration-authorization-content/v1",
		"ref":                                   "authorization-content:test",
		"project_root":                          "/tmp/project",
		"action_kind":                           "profile.declare.from_onboarding_candidate",
		"profile_author_role_assignment_ref":    "assignment:test",
		"profile_author_role_assignment_digest": "digest:assignment",
		"method_description_ref":                "method-description:onboard",
		"method_description_digest":             "digest:method-description",
		"method_contract_ref":                   "contract:onboard",
		"method_contract_digest":                "digest:contract",
		"classifier_version":                    "classifier:v1",
		"policy_version":                        "policy:v1",
		"session_ref":                           "session:test",
		"allowed_work_from":                     allowedWorkFrom,
		"allowed_work_until":                    allowedWorkUntil,
		"basis_observation_from":                basisObservationFrom,
		"basis_observation_until":               basisObservationUntil,
		"authorization_valid_from":              "2026-07-14T00:10:00Z",
		"authorization_valid_until":             "2026-07-14T00:50:00Z",
		"single_use_key":                        "single-use:test",
	})
	if err != nil {
		return err
	}
	_, err = connection.Exec(`INSERT INTO profile_declaration_authorization_contents (
		authorization_content_ref, authorization_content_digest, project_root,
		action_kind, profile_author_role_assignment_ref,
		profile_author_role_assignment_digest, method_description_ref,
		method_description_digest, method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref,
		allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until,
		authorization_valid_from, authorization_valid_until,
		single_use_key, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"authorization-content:test",
		authorityBasisTestDigest("e"),
		"/tmp/project",
		"profile.declare.from_onboarding_candidate",
		"assignment:test",
		"digest:assignment",
		"method-description:onboard",
		"digest:method-description",
		"contract:onboard",
		"digest:contract",
		"classifier:v1",
		"policy:v1",
		"session:test",
		allowedWorkFrom,
		allowedWorkUntil,
		basisObservationFrom,
		basisObservationUntil,
		"2026-07-14T00:10:00Z",
		"2026-07-14T00:50:00Z",
		"single-use:test",
		contentJSON,
		"2026-07-14T00:10:00Z",
	)
	return err
}

func insertGenericSpeechActSourceFixture(
	connection *sql.DB,
	projectRoot string,
	methodContext string,
	actContext string,
) error {
	return insertGenericSpeechActSourceFixtureWithWindows(
		connection,
		projectRoot,
		methodContext,
		actContext,
		"2026-07-15T10:00:00Z",
		"2026-07-15T10:00:01Z",
		"2026-07-15T10:00:00Z",
		"2026-07-15T10:00:01Z",
	)
}

func insertGenericSpeechActSourceFixtureWithWindows(
	connection *sql.DB,
	projectRoot string,
	methodContext string,
	actContext string,
	assignmentFrom string,
	assignmentUntil string,
	actFrom string,
	actUntil string,
) error {
	methodDigest := authorityBasisTestDigest("1")
	methodJSON, err := authorityBasisJSON(map[string]any{
		"schema":                 "haft.authority.speech-act-method-description/v1",
		"method_ref":             "method:manual-authority-issue",
		"method_description_ref": "method-description:manual-authority-issue:v1",
		"procedure_ref":          "procedure:manual-authority-issue:v1",
		"bounded_context_ref":    methodContext,
		"procedure_semantics":    "display review; capture exact terminal utterance; derive sources",
	})
	if err != nil {
		return err
	}
	_, err = connection.Exec(`INSERT INTO speech_act_method_descriptions (
		method_description_ref, method_description_digest, method_ref,
		procedure_ref, bounded_context_ref, procedure_semantics,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"method-description:manual-authority-issue:v1",
		methodDigest,
		"method:manual-authority-issue",
		"procedure:manual-authority-issue:v1",
		methodContext,
		"display review; capture exact terminal utterance; derive sources",
		methodJSON,
		"2026-07-15T10:00:01Z",
	)
	if err != nil {
		return err
	}

	policyDigest := authorityBasisTestDigest("2")
	policyJSON, err := authorityBasisJSON(map[string]any{
		"schema":                        "haft.authority.speech-act-context-policy/v1",
		"ref":                           "context-policy:profile-authority:v1",
		"bounded_context_ref":           actContext,
		"recognized_act_type_ref":       "speech-act-type:authorize",
		"authorizer_role_ref":           "role:project-principal-authorizer",
		"admitted_holder_kind":          "U.System",
		"assignment_source_rule":        "observed-local-controlling-terminal-session/v1",
		"institutional_effect_rule_ref": "institution-rule:authorize-profile:v1",
		"instituted_object_kind":        "U.Commitment",
		"institutional_modality":        "MAY",
		"scoped_action":                 "profile.declare.from_onboarding_candidate",
		"utterance_description_ref":     "utterance:authorize-reviewed-intent:v1",
		"utterance_verb":                "AUTHORIZE",
		"utterance_binding":             "review_digest",
	})
	if err != nil {
		return err
	}
	_, err = connection.Exec(`INSERT INTO speech_act_context_policies (
		context_policy_ref, context_policy_digest, bounded_context_ref,
		recognized_act_type_ref, authorizer_role_ref, admitted_holder_kind,
		assignment_source_rule, institutional_effect_rule_ref,
		instituted_object_kind, institutional_modality, scoped_action,
		utterance_description_ref, utterance_verb, utterance_binding,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"context-policy:profile-authority:v1",
		policyDigest,
		actContext,
		"speech-act-type:authorize",
		"role:project-principal-authorizer",
		"U.System",
		"observed-local-controlling-terminal-session/v1",
		"institution-rule:authorize-profile:v1",
		"U.Commitment",
		"MAY",
		"profile.declare.from_onboarding_candidate",
		"utterance:authorize-reviewed-intent:v1",
		"AUTHORIZE",
		"review_digest",
		policyJSON,
		"2026-07-15T10:00:01Z",
	)
	if err != nil {
		return err
	}
	if err := insertTerminalCaptureFixture(connection, projectRoot); err != nil {
		return err
	}

	captureDigest := authorityBasisTestDigest("4")
	assignmentDigest := authorityBasisTestDigest("8")
	assignmentJSON, err := authorityBasisJSON(map[string]any{
		"schema":                               "haft.authority.context-policy-assigned-terminal-session/v1",
		"role_assignment_ref":                  "role-assignment:terminal-authorizer:test",
		"project_root":                         projectRoot,
		"holder_system_ref":                    "system:local-terminal-session:test",
		"admitted_holder_kind":                 "U.System",
		"role_ref":                             "role:project-principal-authorizer",
		"bounded_context_ref":                  actContext,
		"valid_from":                           assignmentFrom,
		"valid_until":                          assignmentUntil,
		"justification_source_ref":             "context-policy:profile-authority:v1",
		"justification_source_digest":          policyDigest,
		"assignment_provenance_carrier_ref":    "carrier:terminal-capture:test",
		"assignment_provenance_carrier_digest": captureDigest,
		"identity_boundary":                    "anti-accident terminal-session identity only",
	})
	if err != nil {
		return err
	}
	_, err = connection.Exec(`INSERT INTO speech_act_role_assignments (
		role_assignment_ref, role_assignment_digest, project_root,
		holder_system_ref, admitted_holder_kind, role_ref, bounded_context_ref,
		valid_from, valid_until, context_policy_ref, context_policy_digest,
		provenance_carrier_ref, provenance_carrier_digest, identity_boundary,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"role-assignment:terminal-authorizer:test",
		assignmentDigest,
		projectRoot,
		"system:local-terminal-session:test",
		"U.System",
		"role:project-principal-authorizer",
		actContext,
		assignmentFrom,
		assignmentUntil,
		"context-policy:profile-authority:v1",
		policyDigest,
		"carrier:terminal-capture:test",
		captureDigest,
		"anti-accident terminal-session identity only",
		assignmentJSON,
		"2026-07-15T10:00:01Z",
	)
	if err != nil {
		return err
	}

	parametersJSON := `[]`
	inputRefsJSON := `["review-subject:profile-intent:test"]`
	outputRefsJSON := `["permission:profile-declaration:test"]`
	resourceRefsJSON := `[]`
	affectedRefsJSON := `["project-profile:/tmp/project"]`
	actJSON, err := authorityBasisJSON(map[string]any{
		"schema":                              "haft.authority.speech-act/v1",
		"speech_act_ref":                      "speech-act:profile-authority:test",
		"project_root":                        projectRoot,
		"work_kind":                           "Communicative",
		"act_type_ref":                        "speech-act-type:authorize",
		"performed_by_role_assignment_ref":    "role-assignment:terminal-authorizer:test",
		"performed_by_role_assignment_digest": assignmentDigest,
		"method_ref":                          "method:manual-authority-issue",
		"method_description_ref":              "method-description:manual-authority-issue:v1",
		"method_description_digest":           methodDigest,
		"executed_within_system_ref":          "system:local-terminal-session:test",
		"bounded_context_ref":                 actContext,
		"window_from":                         actFrom,
		"window_until":                        actUntil,
		"parameters":                          []any{},
		"input_refs":                          []string{"review-subject:profile-intent:test"},
		"output_refs":                         []string{"permission:profile-declaration:test"},
		"resource_refs":                       []any{},
		"affected_refs":                       []string{"project-profile:/tmp/project"},
		"state_plane_ref":                     "state-plane:project-governance",
		"delta_predicate_ref":                 "delta-predicate:permission-instituted",
		"outcome_ref":                         "work-outcome:permission-instituted",
		"utterance_ref":                       "utterance:authorize-reviewed-intent:v1",
		"capture_carrier_ref":                 "carrier:terminal-capture:test",
		"capture_carrier_digest":              captureDigest,
		"review_subject_ref":                  "review-subject:profile-intent:test",
		"review_subject_digest":               authorityBasisTestDigest("9"),
		"instituted_object_ref":               "permission:profile-declaration:test",
	})
	if err != nil {
		return err
	}
	_, err = connection.Exec(`INSERT INTO speech_acts (
		speech_act_ref, speech_act_digest, project_root, work_kind, act_type_ref,
		performed_by_ref, performed_by_digest, method_ref,
		method_description_ref, method_description_digest,
		executed_within_ref, bounded_context_ref, window_from, window_until,
		parameters_json, input_refs_json, output_refs_json, resource_refs_json,
		affected_refs_json, state_plane_ref, delta_predicate_ref, outcome_ref,
		utterance_ref, capture_carrier_ref, capture_carrier_digest,
		review_subject_ref, review_subject_digest, instituted_object_ref,
		context_policy_ref, context_policy_digest, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"speech-act:profile-authority:test",
		authorityBasisTestDigest("a"),
		projectRoot,
		"Communicative",
		"speech-act-type:authorize",
		"role-assignment:terminal-authorizer:test",
		assignmentDigest,
		"method:manual-authority-issue",
		"method-description:manual-authority-issue:v1",
		methodDigest,
		"system:local-terminal-session:test",
		actContext,
		actFrom,
		actUntil,
		parametersJSON,
		inputRefsJSON,
		outputRefsJSON,
		resourceRefsJSON,
		affectedRefsJSON,
		"state-plane:project-governance",
		"delta-predicate:permission-instituted",
		"work-outcome:permission-instituted",
		"utterance:authorize-reviewed-intent:v1",
		"carrier:terminal-capture:test",
		captureDigest,
		"review-subject:profile-intent:test",
		authorityBasisTestDigest("9"),
		"permission:profile-declaration:test",
		"context-policy:profile-authority:v1",
		policyDigest,
		actJSON,
		"2026-07-15T10:00:01Z",
	)
	return err
}

func insertTerminalCaptureFixture(connection *sql.DB, projectRoot string) error {
	return insertTerminalCaptureFixtureAt(
		connection,
		projectRoot,
		"2026-07-15T10:00:00Z",
		"2026-07-15T10:00:00.5Z",
		"2026-07-15T10:00:01Z",
	)
}

func insertTerminalCaptureFixtureAt(
	connection *sql.DB,
	projectRoot string,
	startedAt string,
	exactUtteranceObservedAt string,
	endedAt string,
) error {
	preparedDigest := authorityBasisTestDigest("5")
	reviewDigest := authorityBasisTestDigest("6")
	observationDigest := authorityBasisTestDigest("7")
	captureJSON, err := authorityBasisJSON(map[string]any{
		"schema":                            "haft.authority.terminal-capture/v1",
		"carrier_ref":                       "carrier:terminal-capture:test",
		"project_root":                      projectRoot,
		"prepared_speech_act_intent_digest": preparedDigest,
		"review_text":                       "exact reviewed authority intent",
		"review_digest":                     reviewDigest,
		"canonical_utterance":               "AUTHORIZE " + reviewDigest,
		"started_at":                        startedAt,
		"exact_utterance_observed_at":       exactUtteranceObservedAt,
		"ended_at":                          endedAt,
		"session_ref":                       "session:prepared-intent:test",
		"observed_session_material":         "path=/dev/tty;mode=Dcrw--w----;pid=1;ppid=0",
		"observation_nonce":                 strings.Repeat("a", 32),
		"observation_digest":                observationDigest,
		"observed_holder_system_ref":        "system:local-terminal-session:test",
		"observed_role_assignment_ref":      "role-assignment:terminal-authorizer:test",
	})
	if err != nil {
		return err
	}
	_, err = connection.Exec(`INSERT INTO terminal_capture_records (
		capture_carrier_ref, capture_carrier_digest, project_root,
		prepared_speech_act_intent_digest, review_text, review_digest,
		canonical_utterance, started_at, exact_utterance_observed_at, ended_at,
		intent_session_ref,
		observed_session_material, observation_nonce, observation_digest,
		observed_holder_system_ref, observed_role_assignment_ref,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"carrier:terminal-capture:test",
		authorityBasisTestDigest("4"),
		projectRoot,
		preparedDigest,
		"exact reviewed authority intent",
		reviewDigest,
		"AUTHORIZE "+reviewDigest,
		startedAt,
		exactUtteranceObservedAt,
		endedAt,
		"session:prepared-intent:test",
		"path=/dev/tty;mode=Dcrw--w----;pid=1;ppid=0",
		strings.Repeat("a", 32),
		observationDigest,
		"system:local-terminal-session:test",
		"role-assignment:terminal-authorizer:test",
		captureJSON,
		"2026-07-15T10:00:01Z",
	)
	return err
}

func authorityBasisJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func mustAuthorityBasisJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := authorityBasisJSON(value)
	if err != nil {
		t.Fatalf("encode authority-basis fixture JSON: %v", err)
	}
	return encoded
}

func authorityBasisTestDigest(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}

func assertSQLiteObjectsExist(
	t *testing.T,
	connection *sql.DB,
	objectType string,
	names []string,
	index int,
) {
	t.Helper()
	if index >= len(names) {
		return
	}
	name := names[index]
	var actual string
	if err := connection.QueryRow(
		"SELECT name FROM sqlite_master WHERE type = ? AND name = ?",
		objectType,
		name,
	).Scan(&actual); err != nil || actual != name {
		t.Fatalf("missing %s %s: actual=%q err=%v", objectType, name, actual, err)
	}
	assertSQLiteObjectsExist(t, connection, objectType, names, index+1)
}

func assertSQLiteObjectsAbsent(
	t *testing.T,
	connection *sql.DB,
	objectType string,
	names []string,
	index int,
) {
	t.Helper()
	if index >= len(names) {
		return
	}
	name := names[index]
	var count int
	if err := connection.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		objectType,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect forbidden %s %s: %v", objectType, name, err)
	}
	if count != 0 {
		t.Fatalf("unexpected %s %s exists", objectType, name)
	}
	assertSQLiteObjectsAbsent(t, connection, objectType, names, index+1)
}

func assertAuthorityBasisImmutabilityTriggers(
	t *testing.T,
	connection *sql.DB,
	tables []string,
	index int,
) {
	t.Helper()
	if index >= len(tables) {
		return
	}
	table := tables[index]
	assertSQLiteObjectsExist(t, connection, "trigger", []string{
		table + "_no_replace",
		table + "_no_update",
		table + "_no_delete",
	}, 0)
	assertAuthorityBasisImmutabilityTriggers(t, connection, tables, index+1)
}

func assertColumnsPresent(
	t *testing.T,
	columns map[string]bool,
	names []string,
	index int,
) {
	t.Helper()
	if index >= len(names) {
		return
	}
	if !columns[names[index]] {
		t.Fatalf("required column %s is absent", names[index])
	}
	assertColumnsPresent(t, columns, names, index+1)
}

func assertColumnsAbsent(
	t *testing.T,
	columns map[string]bool,
	names []string,
	index int,
) {
	t.Helper()
	if index >= len(names) {
		return
	}
	if columns[names[index]] {
		t.Fatalf("forbidden future-result column %s exists", names[index])
	}
	assertColumnsAbsent(t, columns, names, index+1)
}

func assertTableRowCountV38(
	t *testing.T,
	connection *sql.DB,
	table string,
	want int,
) {
	t.Helper()
	var count int
	if err := connection.QueryRow(
		"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table),
	).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("%s row count = %d, want %d", table, count, want)
	}
}

func assertMigration38Uncommitted(t *testing.T, connection *sql.DB) {
	t.Helper()
	var count int
	if err := connection.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 38",
	).Scan(&count); err != nil {
		t.Fatalf("inspect rolled-back v38 marker: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed v38 migration recorded %d marker(s)", count)
	}
	var tableCount int
	if err := connection.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'speech_act_method_descriptions'",
	).Scan(&tableCount); err != nil {
		t.Fatalf("inspect rolled-back v38 schema: %v", err)
	}
	if tableCount != 0 {
		t.Fatal("failed v38 migration left canonical tables behind")
	}
}

func assertNoForeignKeyViolationsV38(t *testing.T, connection *sql.DB) {
	t.Helper()
	rows, err := connection.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run v38 foreign-key check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("v38 schema has a foreign-key violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read v38 foreign-key check: %v", err)
	}
}
