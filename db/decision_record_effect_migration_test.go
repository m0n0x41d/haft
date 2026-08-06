package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	artifactpkg "github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/decisionbinding"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	_ "modernc.org/sqlite"
)

func TestDecisionRecordEffectMigrationInstallsAdditiveClosure(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "decision-effect.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	for _, table := range decisionRecordEffectMigration41Tables {
		assertSQLiteObjectExists(t, store.conn, "table", table)
	}
	for _, trigger := range []string{
		"decision_binding_contents_exact_collection_shape",
		"decision_binding_contents_no_retrobind",
		"decision_record_instituted_effects_exact_source",
		"decision_record_instituted_effects_exact_artifact",
		"decision_binding_contents_no_replace",
		"decision_binding_contents_no_update",
		"decision_binding_contents_no_delete",
		"decision_record_instituted_effects_no_replace",
		"decision_record_instituted_effects_no_update",
		"decision_record_instituted_effects_no_delete",
		"decision_binding_contents_project_ledger_root",
		"decision_record_instituted_effects_project_ledger_root",
	} {
		assertSQLiteObjectExists(t, store.conn, "trigger", trigger)
	}
	for _, index := range []string{
		"idx_decision_binding_contents_project_recorded",
		"idx_decision_record_effects_project_recorded",
	} {
		assertSQLiteObjectExists(t, store.conn, "index", index)
	}
	assertMigrationVersionPresent(t, store.conn, 41)
	assertDecisionEffectForeignKeysClean(t, store.conn)
}

func TestDecisionRecordEffectMigrationRejectsUnknownPartialFootprint(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeMigration41(t)
	defer database.Close()

	_, err := database.Exec(`CREATE TABLE decision_binding_contents (unknown TEXT)`)
	if err != nil {
		t.Fatalf("seed unknown partial v41 footprint: %v", err)
	}
	err = Migrate(database, "schema_version", kernelMigrations)
	if err == nil || !strings.Contains(err.Error(), "unknown partial schema") {
		t.Fatalf("partial v41 footprint error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 41)
	assertSQLiteObjectAbsent(t, database, "table", "decision_record_instituted_effects")
}

func TestDecisionRecordEffectMigrationPreservesLegacyDecisionWithoutBackfill(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeMigration41(t)
	defer database.Close()

	insertDecisionEffectArtifact(
		t,
		database,
		"dec-20260715-legacy-a1b2c3d4",
		"Legacy decision remains readable",
		"legacy body",
		"{}",
		"2026-07-14T10:00:00Z",
	)
	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("migrate legacy DecisionRecord through v41: %v", err)
	}

	var kind string
	var title string
	err := database.QueryRow(
		"SELECT kind, title FROM artifacts WHERE id = ?",
		"dec-20260715-legacy-a1b2c3d4",
	).Scan(&kind, &title)
	if err != nil {
		t.Fatalf("read preserved legacy DecisionRecord: %v", err)
	}
	if kind != "DecisionRecord" || title != "Legacy decision remains readable" {
		t.Fatalf("preserved legacy DecisionRecord = %s %q", kind, title)
	}
	for _, table := range decisionRecordEffectMigration41Tables {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("migration backfilled %d row(s) into %s", count, table)
		}
	}
	assertMigrationVersionPresent(t, database, 41)
}

func TestDecisionBindingContentRejectsForeignRootRetrobindAndInvalidCollections(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "decision-content-guards.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	insertProjectLedgerBindingForReviewV2(t, store.conn, "/project/bound")
	foreign := decisionEffectPreparedFixture("/project/foreign", "dec-20260715-foreign-a1b2c3d4")
	if err := insertDecisionBindingContent(store.conn, foreign); err == nil ||
		!strings.Contains(err.Error(), "bound project ledger root") {
		t.Fatalf("foreign DecisionBindingContent error = %v", err)
	}

	legacyRef := "dec-20260715-retro-a1b2c3d4"
	insertDecisionEffectArtifact(t, store.conn, legacyRef, "Existing", "body", "{}", "2026-07-15T09:00:00Z")
	retro := decisionEffectPreparedFixture("/project/bound", legacyRef)
	if err := insertDecisionBindingContent(store.conn, retro); err == nil ||
		!strings.Contains(err.Error(), "cannot authorize a legacy artifact retroactively") {
		t.Fatalf("retrobind content error = %v", err)
	}

	duplicate := decisionEffectPreparedFixture("/project/bound", "dec-20260715-duplicate-a1b2c3d4")
	duplicate.links = append(duplicate.links, duplicate.links[0])
	duplicate.refreshCanonical(t)
	if err := insertDecisionBindingContent(store.conn, duplicate); err == nil ||
		!strings.Contains(err.Error(), "canonical non-duplicated sets") {
		t.Fatalf("duplicate collection error = %v", err)
	}
}

func TestDecisionBindingContentAcceptsCanonicalPreparedDecisionBytes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := NewStore(filepath.Join(root, "prepared-parity.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	reservation, err := artifactpkg.NewDecisionReservation(
		"dec-20260715-prepared-parity-a1b2c3d4",
	)
	if err != nil {
		t.Fatalf("reserve decision identity: %v", err)
	}
	proposal := artifactpkg.DecideInput{
		ProblemStatement: "A manual decision must bind the exact prepared semantic snapshot.",
		SelectedTitle:    "Use PreparedDecision as the reviewed decision content",
		WhySelected:      "It closes over enrichment and source reads before the SpeechAct.",
		SelectionPolicy:  "Prefer the representation that prevents post-review semantic drift.",
		CounterArgument:  "The additional snapshot can increase storage and implementation cost.",
		WhyNotOthers: []artifactpkg.RejectionReason{{
			Variant: "Bind raw DecideInput",
			Reason:  "It leaves enrichment and source resolution after the human review boundary.",
		}},
		WeakestLink: "Source revalidation must remain exact at institution time.",
		Rollback: &artifactpkg.RollbackSpec{
			Triggers: []string{"PreparedDecision cannot be revalidated without semantic ambiguity"},
		},
		AffectedFiles: []string{"internal/decisionbinding/effect.go"},
	}
	prepared, err := artifactpkg.PrepareDecision(
		context.Background(),
		artifactpkg.NewStore(store.conn),
		filepath.Join(root, ".haft"),
		reservation,
		proposal,
	)
	if err != nil {
		t.Fatalf("PrepareDecision: %v", err)
	}
	preparedBytes, preparedOK := prepared.CanonicalBytes()
	preparedDigest, digestOK := prepared.Digest()
	decisionRef, refOK := prepared.DecisionRef()
	projectRoot, rootOK := prepared.ProjectRoot()
	if !preparedOK || !digestOK || !refOK || !rootOK {
		t.Fatal("canonical PreparedDecision accessors are unavailable")
	}
	content, err := decisionbinding.NewDecisionBindingContent(prepared)
	if err != nil {
		t.Fatalf("NewDecisionBindingContent: %v", err)
	}
	contentBytes, contentBytesOK := content.CanonicalBytes()
	contentDigest, contentDigestOK := content.Digest()
	if !contentBytesOK || !contentDigestOK {
		t.Fatal("canonical DecisionBindingContent accessors are unavailable")
	}
	fixture := decisionEffectPrepared{
		root:              projectRoot,
		decisionRef:       decisionRef,
		contentDigest:     contentDigest.String(),
		preparedDigest:    preparedDigest.String(),
		preparedCanonical: string(preparedBytes),
		canonical:         string(contentBytes),
	}
	fixture.contentRef = "review-subject:decision-binding:" + strings.TrimPrefix(
		fixture.contentDigest,
		"sha256:",
	)
	if err := insertDecisionBindingContent(store.conn, fixture); err != nil {
		t.Fatalf("v41 rejected canonical PreparedDecision bytes: %v", err)
	}
}

func TestDecisionBindingContentRejectsMalformedSourcePinVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*decisionEffectPrepared)
	}{
		{
			name: "get found without digest",
			mutate: func(fixture *decisionEffectPrepared) {
				delete(fixture.sourcePins[0], "digest")
			},
		},
		{
			name: "get unavailable with found material",
			mutate: func(fixture *decisionEffectPrepared) {
				fixture.sourcePins[1]["version"] = 1
			},
		},
		{
			name: "list observation without digest",
			mutate: func(fixture *decisionEffectPrepared) {
				delete(fixture.sourcePins[2], "digest")
			},
		},
		{
			name: "list observation with malformed member",
			mutate: func(fixture *decisionEffectPrepared) {
				members := fixture.sourcePins[2]["members"].([]map[string]any)
				members[0]["version"] = 0
			},
		},
		{
			name: "list observation with duplicate member",
			mutate: func(fixture *decisionEffectPrepared) {
				members := fixture.sourcePins[2]["members"].([]map[string]any)
				fixture.sourcePins[2]["members"] = append(members, members[0])
			},
		},
		{
			name: "unknown source operation",
			mutate: func(fixture *decisionEffectPrepared) {
				fixture.sourcePins[0]["operation"] = "lookup"
			},
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			store, err := NewStore(filepath.Join(root, "source-pin.db"))
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}
			defer store.Close()

			decisionRef := fmt.Sprintf(
				"dec-20260715-pin-%d-a1b2c3d4",
				index,
			)
			fixture := decisionEffectPreparedFixture(root, decisionRef)
			test.mutate(&fixture)
			fixture.refreshCanonical(t)
			err = insertDecisionBindingContent(store.conn, fixture)
			if err == nil || !strings.Contains(err.Error(), "source pins") {
				t.Fatalf("malformed source pin error = %v", err)
			}
		})
	}
}

func TestDecisionRecordEffectRequiresExactSourceAndArtifactClosure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := NewStore(filepath.Join(root, "decision-effect-exact.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	fixture := decisionEffectPreparedFixture(root, "dec-20260715-exact-a1b2c3d4")
	insertDecisionEffectArtifact(t, store.conn, "note-decision-source", "Decision source", "source", "{}", "2026-07-15T09:00:00Z")
	insertDecisionEffectArtifact(t, store.conn, "note-decision-other", "Other source", "other", "{}", "2026-07-15T09:00:00Z")
	if err := insertDecisionBindingContent(store.conn, fixture); err != nil {
		t.Fatalf("insert PreparedDecision content: %v", err)
	}
	source := recordDecisionEffectSpeechActSource(t, store.conn, fixture)
	insertPreparedDecisionArtifactRows(t, store.conn, fixture, source.occurredAt)
	if err := insertDecisionRecordEffect(store.conn, fixture, source); err != nil {
		t.Fatalf("insert exact DecisionRecord effect: %v", err)
	}

	var count int
	if err := store.conn.QueryRow("SELECT COUNT(*) FROM decision_record_instituted_effects").Scan(&count); err != nil {
		t.Fatalf("count DecisionRecord effects: %v", err)
	}
	if count != 1 {
		t.Fatalf("DecisionRecord effect count = %d, want 1", count)
	}

	if _, err := store.conn.Exec(
		"UPDATE decision_record_instituted_effects SET project_root = project_root WHERE decision_ref = ?",
		fixture.decisionRef,
	); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("effect update error = %v", err)
	}

	var contentBefore string
	var effectBefore string
	if err := store.conn.QueryRow(
		"SELECT canonical_json FROM decision_binding_contents WHERE decision_ref = ?",
		fixture.decisionRef,
	).Scan(&contentBefore); err != nil {
		t.Fatalf("read immutable decision content: %v", err)
	}
	if err := store.conn.QueryRow(
		"SELECT canonical_json FROM decision_record_instituted_effects WHERE decision_ref = ?",
		fixture.decisionRef,
	).Scan(&effectBefore); err != nil {
		t.Fatalf("read immutable decision effect: %v", err)
	}

	projectionMutations := []struct {
		statement string
		arguments []any
	}{
		{
			statement: "UPDATE artifacts SET title = 'Later lifecycle projection' WHERE id = ?",
			arguments: []any{fixture.decisionRef},
		},
		{
			statement: "INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES (?, ?, ?, ?)",
			arguments: []any{fixture.decisionRef, "note-decision-other", "related_to", source.occurredAt},
		},
		{
			statement: "UPDATE affected_files SET file_hash = ? WHERE artifact_id = ?",
			arguments: []any{decisionEffectDigest("later baseline"), fixture.decisionRef},
		},
	}
	for _, mutation := range projectionMutations {
		if _, err := store.conn.Exec(mutation.statement, mutation.arguments...); err != nil {
			t.Fatalf("evolve current DecisionRecord projection: %v", err)
		}
	}

	var contentAfter string
	var effectAfter string
	if err := store.conn.QueryRow(
		"SELECT canonical_json FROM decision_binding_contents WHERE decision_ref = ?",
		fixture.decisionRef,
	).Scan(&contentAfter); err != nil {
		t.Fatalf("reread immutable decision content: %v", err)
	}
	if err := store.conn.QueryRow(
		"SELECT canonical_json FROM decision_record_instituted_effects WHERE decision_ref = ?",
		fixture.decisionRef,
	).Scan(&effectAfter); err != nil {
		t.Fatalf("reread immutable decision effect: %v", err)
	}
	if contentAfter != contentBefore || effectAfter != effectBefore {
		t.Fatal("later project projection work rewrote the historical decision source or effect")
	}
	if _, err := store.conn.Exec(
		"DELETE FROM artifacts WHERE id = ?",
		fixture.decisionRef,
	); err == nil {
		t.Fatal("foreign-key closure allowed deletion of an instituted DecisionRecord")
	}
}

func TestDecisionRecordEffectRejectsArtifactDriftAndLeavesSpeechActSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store, err := NewStore(filepath.Join(root, "decision-effect-drift.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	fixture := decisionEffectPreparedFixture(root, "dec-20260715-drift-a1b2c3d4")
	insertDecisionEffectArtifact(t, store.conn, "note-decision-source", "Decision source", "source", "{}", "2026-07-15T09:00:00Z")
	if err := insertDecisionBindingContent(store.conn, fixture); err != nil {
		t.Fatalf("insert PreparedDecision content: %v", err)
	}
	source := recordDecisionEffectSpeechActSource(t, store.conn, fixture)
	insertPreparedDecisionArtifactRows(t, store.conn, fixture, source.occurredAt)
	if _, err := store.conn.Exec(
		"UPDATE artifacts SET title = 'Unreviewed title' WHERE id = ?",
		fixture.decisionRef,
	); err != nil {
		t.Fatalf("tamper staged DecisionRecord: %v", err)
	}
	err = insertDecisionRecordEffect(store.conn, fixture, source)
	if err == nil || !strings.Contains(err.Error(), "exact staged artifact") {
		t.Fatalf("artifact drift effect error = %v", err)
	}

	var sourceCount int
	if err := store.conn.QueryRow(
		"SELECT COUNT(*) FROM speech_acts WHERE speech_act_ref = ?",
		source.speechActRef,
	).Scan(&sourceCount); err != nil {
		t.Fatalf("count preserved SpeechAct source: %v", err)
	}
	if sourceCount != 1 {
		t.Fatalf("failed effect left SpeechAct source count = %d, want 1", sourceCount)
	}
}

func TestDecisionRecordEffectRejectsWrongFrameAndCrossDomainSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		material func(testing.TB, decisionEffectPrepared) decisionEffectSpeechActMaterial
	}{
		{
			name: "wrong decision content parameter",
			material: func(t testing.TB, fixture decisionEffectPrepared) decisionEffectSpeechActMaterial {
				return decisionEffectSpeechActMaterial{
					policy: buildDecisionEffectPolicy(t),
					frame: buildDecisionEffectFrameWithDigest(
						t,
						fixture,
						decisionEffectDigest("different-decision-content"),
					),
				}
			},
		},
		{
			name: "self-consistent foreign institutional domain",
			material: func(t testing.TB, fixture decisionEffectPrepared) decisionEffectSpeechActMaterial {
				return decisionEffectSpeechActMaterial{
					policy: buildForeignDecisionEffectPolicy(t),
					frame:  buildForeignDecisionEffectFrame(t, fixture),
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			store, err := NewStore(filepath.Join(root, "decision-source-reject.db"))
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}
			defer store.Close()

			fixture := decisionEffectPreparedFixture(
				root,
				"dec-20260715-source-reject-a1b2c3d4",
			)
			insertDecisionEffectArtifact(
				t,
				store.conn,
				"note-decision-source",
				"Decision source",
				"source",
				"{}",
				"2026-07-15T09:00:00Z",
			)
			if err := insertDecisionBindingContent(store.conn, fixture); err != nil {
				t.Fatalf("insert PreparedDecision content: %v", err)
			}
			source := recordDecisionEffectSpeechActSourceWithMaterial(
				t,
				store.conn,
				fixture,
				test.material(t, fixture),
			)
			insertPreparedDecisionArtifactRows(t, store.conn, fixture, source.occurredAt)
			err = insertDecisionRecordEffect(store.conn, fixture, source)
			if err == nil || !strings.Contains(err.Error(), "exact decision content") {
				t.Fatalf("foreign or mismatched source effect error = %v", err)
			}
		})
	}
}

type decisionEffectPrepared struct {
	root              string
	decisionRef       string
	contentRef        string
	contentDigest     string
	preparedDigest    string
	proposalInput     map[string]any
	resolvedInput     map[string]any
	artifact          map[string]any
	links             []map[string]any
	affectedFiles     []map[string]any
	sourcePins        []map[string]any
	preparedCanonical string
	canonical         string
}

func decisionEffectPreparedFixture(root string, decisionRef string) decisionEffectPrepared {
	structured := map[string]any{"selected_title": "Use exact DecisionRecord effects"}
	fixture := decisionEffectPrepared{
		root:          root,
		decisionRef:   decisionRef,
		proposalInput: map[string]any{"selected_title": "Use exact DecisionRecord effects"},
		resolvedInput: map[string]any{"selected_title": "Use exact DecisionRecord effects"},
		artifact: map[string]any{
			"id":              decisionRef,
			"kind":            "DecisionRecord",
			"version":         1,
			"status":          "active",
			"context":         "",
			"mode":            "deep",
			"title":           "Use exact DecisionRecord effects",
			"valid_until":     "2026-08-15T00:00:00Z",
			"body":            "# Use exact DecisionRecord effects\n",
			"search_keywords": "decision speech act",
			"structured_data": structured,
		},
		links: []map[string]any{{"ref": "note-decision-source", "type": "based_on"}},
		affectedFiles: []map[string]any{{
			"path": "internal/decisionbinding/effect.go",
		}},
		sourcePins: []map[string]any{
			{
				"operation": "get",
				"ref":       "note-decision-source",
				"outcome":   "found",
				"version":   1,
				"digest":    decisionEffectDigest("note-decision-source"),
			},
			{
				"operation": "get",
				"ref":       "prob-missing-source",
				"outcome":   "unavailable",
			},
			{
				"operation": "list_by_kind",
				"ref":       "kind:DecisionRecord;limit:0",
				"outcome":   "observed",
				"digest":    decisionEffectDigest("decision-record-set"),
				"members": []map[string]any{{
					"ref":     "note-decision-source",
					"version": 1,
					"digest":  decisionEffectDigest("note-decision-source"),
				}},
			},
		},
	}
	fixture.refreshCanonical(nil)
	return fixture
}

func (fixture *decisionEffectPrepared) refreshCanonical(t testing.TB) {
	preparedValue := struct {
		Schema        string           `json:"schema"`
		ProjectRoot   string           `json:"project_root"`
		DecisionRef   string           `json:"decision_ref"`
		ProposalInput map[string]any   `json:"proposal_input"`
		ResolvedInput map[string]any   `json:"resolved_input"`
		Artifact      map[string]any   `json:"artifact"`
		Links         []map[string]any `json:"links"`
		AffectedFiles []map[string]any `json:"affected_files"`
		SourcePins    []map[string]any `json:"source_pins"`
	}{
		Schema:        decisionBindingPreparedSchemaV1,
		ProjectRoot:   fixture.root,
		DecisionRef:   fixture.decisionRef,
		ProposalInput: fixture.proposalInput,
		ResolvedInput: fixture.resolvedInput,
		Artifact:      fixture.artifact,
		Links:         fixture.links,
		AffectedFiles: fixture.affectedFiles,
		SourcePins:    fixture.sourcePins,
	}
	prepared, err := json.Marshal(preparedValue)
	if err != nil {
		if t != nil {
			t.Fatalf("encode PreparedDecision fixture: %v", err)
		}
		panic(err)
	}
	fixture.preparedCanonical = string(prepared)
	fixture.preparedDigest = decisionEffectDigest(fixture.preparedCanonical)
	fixture.refreshContentCanonical(t)
}

func (fixture *decisionEffectPrepared) refreshContentCanonical(t testing.TB) {
	contentValue := struct {
		Schema                 string          `json:"schema"`
		PreparedDecisionDigest string          `json:"prepared_decision_digest"`
		PreparedDecision       json.RawMessage `json:"prepared_decision"`
	}{
		Schema:                 decisionBindingContentSchemaV1,
		PreparedDecisionDigest: fixture.preparedDigest,
		PreparedDecision:       json.RawMessage(fixture.preparedCanonical),
	}
	encoded, err := json.Marshal(contentValue)
	if err != nil {
		if t != nil {
			t.Fatalf("encode DecisionBindingContent fixture: %v", err)
		}
		panic(err)
	}
	fixture.canonical = string(encoded)
	fixture.contentDigest = decisionEffectDigest(fixture.canonical)
	fixture.contentRef = "review-subject:decision-binding:" + strings.TrimPrefix(
		fixture.contentDigest,
		"sha256:",
	)
}

func insertDecisionBindingContent(database *sql.DB, fixture decisionEffectPrepared) error {
	_, err := database.Exec(`INSERT INTO decision_binding_contents (
		decision_content_ref, decision_content_digest, prepared_decision_digest, project_root,
		decision_ref, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.preparedDigest,
		fixture.root,
		fixture.decisionRef,
		fixture.canonical,
		"2026-07-15T09:59:59Z",
	)
	return err
}

type decisionEffectSpeechActSource struct {
	speechActRef        string
	speechActDigest     string
	contextPolicyRef    string
	contextPolicyDigest string
	occurredAt          string
}

type decisionEffectSpeechActMaterial struct {
	policy authority.SpeechActContextPolicy
	frame  authority.SpeechActExecutionFrame
}

func recordDecisionEffectSpeechActSource(
	t testing.TB,
	database *sql.DB,
	fixture decisionEffectPrepared,
) decisionEffectSpeechActSource {
	t.Helper()
	material := decisionEffectSpeechActMaterial{
		policy: buildDecisionEffectPolicy(t),
		frame:  buildDecisionEffectFrame(t, fixture),
	}
	return recordDecisionEffectSpeechActSourceWithMaterial(
		t,
		database,
		fixture,
		material,
	)
}

func recordDecisionEffectSpeechActSourceWithMaterial(
	t testing.TB,
	database *sql.DB,
	fixture decisionEffectPrepared,
	material decisionEffectSpeechActMaterial,
) decisionEffectSpeechActSource {
	t.Helper()
	root := mustAuthorityValue(t, authority.NewProjectRoot, fixture.root)
	speechRef := mustAuthorityValue(
		t,
		authority.NewSpeechActRef,
		"speech-act:decision-binding:"+strings.TrimPrefix(fixture.contentDigest, "sha256:"),
	)
	captureRef := mustAuthorityValue(
		t,
		authority.NewCarrierRef,
		"carrier:terminal-capture:decision-binding:"+strings.TrimPrefix(fixture.contentDigest, "sha256:"),
	)
	sessionRef := mustAuthorityValue(
		t,
		authority.NewSessionRef,
		"session:decision-binding:"+strings.TrimPrefix(fixture.contentDigest, "sha256:"),
	)
	reviewSubject := mustAuthorityValue(t, authority.NewSpeechActReviewSubjectRef, fixture.contentRef)
	reviewDigest := mustAuthorityValue(t, authority.NewDigest, fixture.contentDigest)
	instituted := mustAuthorityValue(t, authority.NewInstitutedObjectRef, fixture.decisionRef)
	intent, err := authority.NewPreparedSpeechActIntentBuilder(speechRef, captureRef).
		ForProject(root).
		InSession(sessionRef).
		Reviewing(reviewSubject, reviewDigest).
		Institutes(instituted).
		UnderContextPolicy(material.policy).
		WithExecutionFrame(material.frame).
		Build()
	if err != nil {
		t.Fatalf("build decision SpeechAct intent: %v", err)
	}
	prepared, err := authority.PrepareManualSpeechAct(intent, "Bind this exact reviewed DecisionRecord.")
	if err != nil {
		t.Fatalf("prepare manual decision SpeechAct: %v", err)
	}
	started := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	occurred := started.Add(time.Second)
	ended := occurred.Add(time.Second)
	verified, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		prepared,
		started,
		occurred,
		ended,
	)
	if err != nil {
		t.Fatalf("capture decision SpeechAct fixture: %v", err)
	}
	writer, err := authority.OpenSpeechActSourceWriter(database)
	if err != nil {
		t.Fatalf("open SpeechAct source writer: %v", err)
	}
	transaction, err := sqlitetransaction.BeginImmediate(context.Background(), database)
	if err != nil {
		t.Fatalf("begin SpeechAct source transaction: %v", err)
	}
	result, err := writer.RecordInTransaction(context.Background(), transaction, verified)
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("record decision SpeechAct source: %v", err)
	}
	if result.Kind() != authority.SpeechActSourceWriteStaged {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("SpeechAct source write kind = %s", result.Kind())
	}
	if finish := transaction.Commit(context.Background()); !finish.Succeeded() {
		t.Fatalf("commit SpeechAct source: %v", finish.Err())
	}

	var source decisionEffectSpeechActSource
	err = database.QueryRow(`SELECT
		speech_act_ref, speech_act_digest, context_policy_ref, context_policy_digest
		FROM speech_acts WHERE review_subject_ref = ?`,
		fixture.contentRef,
	).Scan(
		&source.speechActRef,
		&source.speechActDigest,
		&source.contextPolicyRef,
		&source.contextPolicyDigest,
	)
	if err != nil {
		t.Fatalf("read recorded decision SpeechAct source: %v", err)
	}
	source.occurredAt = occurred.Format(time.RFC3339Nano)
	return source
}

func buildDecisionEffectPolicy(t testing.TB) authority.SpeechActContextPolicy {
	t.Helper()
	rule, err := authority.NewInstitutionalEffectRule(
		mustAuthorityValue(t, authority.NewInstitutionalEffectRuleRef, decisionBindingEffectRuleRefV1),
		mustAuthorityValue(t, authority.NewInstitutedObjectKind, decisionBindingObjectKindV1),
		mustAuthorityValue(t, authority.NewInstitutionalModality, decisionBindingModalityV1),
		mustAuthorityValue(t, authority.NewActionKind, decisionBindingActionV1),
		mustLiteralUtteranceRule(t),
		mustAuthorityValue(t, authority.NewUtteranceRef, decisionBindingUtteranceRefV1),
	)
	if err != nil {
		t.Fatalf("build decision effect rule: %v", err)
	}
	policy, err := authority.NewSpeechActContextPolicy(
		mustAuthorityValue(t, authority.NewContextPolicyRef, decisionBindingPolicyRefV1),
		mustAuthorityValue(t, authority.NewBoundedContextRef, decisionBindingContextRefV1),
		mustAuthorityValue(t, authority.NewSpeechActTypeRef, decisionBindingActTypeRefV1),
		rule,
	)
	if err != nil {
		t.Fatalf("build decision context policy: %v", err)
	}
	return policy
}

func mustLiteralUtteranceRule(t testing.TB) authority.SpeechActUtteranceRule {
	t.Helper()
	rule, err := authority.NewLiteralSpeechActUtteranceRule(
		decisionBindingUtteranceVerbV1,
		decisionBindingUtteranceTextV1,
	)
	if err != nil {
		t.Fatalf("build decision utterance rule: %v", err)
	}
	return rule
}

func buildDecisionEffectFrame(
	t testing.TB,
	fixture decisionEffectPrepared,
) authority.SpeechActExecutionFrame {
	return buildDecisionEffectFrameWithDigest(t, fixture, fixture.contentDigest)
}

func buildDecisionEffectFrameWithDigest(
	t testing.TB,
	fixture decisionEffectPrepared,
	contentDigest string,
) authority.SpeechActExecutionFrame {
	t.Helper()
	method, err := authority.NewManualControllingTTYMethodDescription(
		mustAuthorityValue(t, authority.NewMethodRef, decisionBindingMethodRefV1),
		mustAuthorityValue(t, authority.NewMethodDescriptionRef, decisionBindingMethodDescRefV1),
		mustAuthorityValue(t, authority.NewMethodProcedureRef, decisionBindingProcedureRefV1),
		mustAuthorityValue(t, authority.NewBoundedContextRef, decisionBindingContextRefV1),
	)
	if err != nil {
		t.Fatalf("build decision MethodDescription: %v", err)
	}
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:decision-binding-content-digest",
		contentDigest,
	)
	if err != nil {
		t.Fatalf("build decision Work parameter: %v", err)
	}
	frame, err := authority.NewSpeechActExecutionFrameBuilder(method).
		ExecutedWithin(mustAuthorityValue(t, authority.NewSystemRef, decisionBindingSystemRefV1)).
		OnStatePlane(
			mustAuthorityValue(t, authority.NewStatePlaneRef, decisionBindingStatePlaneRefV1),
			mustAuthorityValue(t, authority.NewDeltaPredicateRef, decisionBindingDeltaRefV1),
		).
		WithOutcome(mustAuthorityValue(t, authority.NewWorkOutcomeRef, decisionBindingOutcomeRefV1)).
		WithUtteranceDescription(mustAuthorityValue(t, authority.NewUtteranceRef, decisionBindingUtteranceRefV1)).
		BindParameter(parameter).
		UseResource(mustAuthorityValue(t, authority.NewWorkResourceRef, "resource:controlling-terminal")).
		Affect(mustAuthorityValue(
			t,
			authority.NewAffectedRef,
			"affected:decision-record:"+fixture.decisionRef,
		)).
		Affect(mustAuthorityValue(
			t,
			authority.NewAffectedRef,
			"affected:decision-binding-content:"+fixture.contentDigest,
		)).
		Build()
	if err != nil {
		t.Fatalf("build decision execution frame: %v", err)
	}
	return frame
}

func buildForeignDecisionEffectPolicy(
	t testing.TB,
) authority.SpeechActContextPolicy {
	t.Helper()
	utterance, err := authority.NewLiteralSpeechActUtteranceRule(
		"GRANT",
		"THIS REVIEWED SCOPE",
	)
	if err != nil {
		t.Fatalf("build foreign utterance rule: %v", err)
	}
	rule, err := authority.NewInstitutionalEffectRule(
		mustAuthorityValue(
			t,
			authority.NewInstitutionalEffectRuleRef,
			"institution-rule:foreign-grant:v1",
		),
		mustAuthorityValue(t, authority.NewInstitutedObjectKind, "haft.WorkCommission"),
		mustAuthorityValue(t, authority.NewInstitutionalModality, "MAY"),
		mustAuthorityValue(t, authority.NewActionKind, "commission.grant"),
		utterance,
		mustAuthorityValue(t, authority.NewUtteranceRef, "utterance:grant-reviewed-scope:v1"),
	)
	if err != nil {
		t.Fatalf("build foreign institutional rule: %v", err)
	}
	policy, err := authority.NewSpeechActContextPolicy(
		mustAuthorityValue(t, authority.NewContextPolicyRef, "context-policy:foreign-grant:v1"),
		mustAuthorityValue(t, authority.NewBoundedContextRef, "bounded-context:foreign-grant"),
		mustAuthorityValue(t, authority.NewSpeechActTypeRef, "speech-act-type:grant"),
		rule,
	)
	if err != nil {
		t.Fatalf("build foreign context policy: %v", err)
	}
	return policy
}

func buildForeignDecisionEffectFrame(
	t testing.TB,
	fixture decisionEffectPrepared,
) authority.SpeechActExecutionFrame {
	t.Helper()
	boundedContext := mustAuthorityValue(
		t,
		authority.NewBoundedContextRef,
		"bounded-context:foreign-grant",
	)
	method, err := authority.NewManualControllingTTYMethodDescription(
		mustAuthorityValue(t, authority.NewMethodRef, "method:foreign-grant"),
		mustAuthorityValue(
			t,
			authority.NewMethodDescriptionRef,
			"method-description:foreign-grant:v1",
		),
		mustAuthorityValue(
			t,
			authority.NewMethodProcedureRef,
			"procedure:foreign-grant:v1",
		),
		boundedContext,
	)
	if err != nil {
		t.Fatalf("build foreign MethodDescription: %v", err)
	}
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:foreign-content-digest",
		fixture.contentDigest,
	)
	if err != nil {
		t.Fatalf("build foreign Work parameter: %v", err)
	}
	frame, err := authority.NewSpeechActExecutionFrameBuilder(method).
		ExecutedWithin(mustAuthorityValue(t, authority.NewSystemRef, "system:foreign-grant")).
		OnStatePlane(
			mustAuthorityValue(t, authority.NewStatePlaneRef, "state-plane:foreign-grant"),
			mustAuthorityValue(t, authority.NewDeltaPredicateRef, "delta-predicate:foreign-grant"),
		).
		WithOutcome(mustAuthorityValue(t, authority.NewWorkOutcomeRef, "work-outcome:foreign-grant")).
		WithUtteranceDescription(mustAuthorityValue(
			t,
			authority.NewUtteranceRef,
			"utterance:grant-reviewed-scope:v1",
		)).
		BindParameter(parameter).
		UseResource(mustAuthorityValue(t, authority.NewWorkResourceRef, "resource:controlling-terminal")).
		Affect(mustAuthorityValue(
			t,
			authority.NewAffectedRef,
			"affected:foreign-grant:"+fixture.decisionRef,
		)).
		Build()
	if err != nil {
		t.Fatalf("build foreign execution frame: %v", err)
	}
	return frame
}

func insertPreparedDecisionArtifactRows(
	t testing.TB,
	database *sql.DB,
	fixture decisionEffectPrepared,
	occurredAt string,
) {
	t.Helper()
	structured, err := json.Marshal(fixture.artifact["structured_data"])
	if err != nil {
		t.Fatalf("encode DecisionRecord structured data: %v", err)
	}
	_, err = database.Exec(`INSERT INTO artifacts (
		id, kind, version, status, context, mode, title, content,
		valid_until, created_at, updated_at, search_keywords, structured_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.decisionRef,
		fixture.artifact["kind"],
		fixture.artifact["version"],
		fixture.artifact["status"],
		fixture.artifact["context"],
		fixture.artifact["mode"],
		fixture.artifact["title"],
		fixture.artifact["body"],
		fixture.artifact["valid_until"],
		occurredAt,
		occurredAt,
		fixture.artifact["search_keywords"],
		string(structured),
	)
	if err != nil {
		t.Fatalf("insert prepared DecisionRecord: %v", err)
	}
	for _, link := range fixture.links {
		_, err := database.Exec(`INSERT INTO artifact_links (
			source_id, target_id, link_type, created_at
		) VALUES (?, ?, ?, ?)`,
			fixture.decisionRef,
			link["ref"],
			link["type"],
			occurredAt,
		)
		if err != nil {
			t.Fatalf("insert prepared DecisionRecord link: %v", err)
		}
	}
	for _, file := range fixture.affectedFiles {
		_, err := database.Exec(`INSERT INTO affected_files (
			artifact_id, file_path, file_hash
		) VALUES (?, ?, ?)`,
			fixture.decisionRef,
			file["path"],
			"",
		)
		if err != nil {
			t.Fatalf("insert prepared DecisionRecord affected file: %v", err)
		}
	}
}

func insertDecisionRecordEffect(
	database *sql.DB,
	fixture decisionEffectPrepared,
	source decisionEffectSpeechActSource,
) error {
	effectDigest := decisionEffectDigest(
		fixture.contentDigest + ":" + source.speechActDigest + ":" + fixture.decisionRef,
	)
	canonical, err := json.Marshal(map[string]any{
		"schema":                        "haft.decision-record-instituted-effect/v1",
		"effect_digest":                 effectDigest,
		"project_root":                  fixture.root,
		"decision_ref":                  fixture.decisionRef,
		"decision_content_ref":          fixture.contentRef,
		"decision_content_digest":       fixture.contentDigest,
		"speech_act_ref":                source.speechActRef,
		"speech_act_digest":             source.speechActDigest,
		"context_policy_ref":            source.contextPolicyRef,
		"context_policy_digest":         source.contextPolicyDigest,
		"institutional_effect_rule_ref": decisionBindingEffectRuleRefV1,
	})
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO decision_record_instituted_effects (
		effect_digest, project_root, decision_ref,
		decision_content_ref, decision_content_digest,
		speech_act_ref, speech_act_digest,
		context_policy_ref, context_policy_digest,
		institutional_effect_rule_ref, canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		effectDigest,
		fixture.root,
		fixture.decisionRef,
		fixture.contentRef,
		fixture.contentDigest,
		source.speechActRef,
		source.speechActDigest,
		source.contextPolicyRef,
		source.contextPolicyDigest,
		decisionBindingEffectRuleRefV1,
		string(canonical),
		"2026-07-15T10:00:02Z",
	)
	return err
}

func insertDecisionEffectArtifact(
	t testing.TB,
	database *sql.DB,
	id string,
	title string,
	body string,
	structured string,
	createdAt string,
) {
	t.Helper()
	_, err := database.Exec(`INSERT INTO artifacts (
		id, kind, version, status, context, mode, title, content,
		valid_until, created_at, updated_at, search_keywords, structured_data
	) VALUES (?, 'DecisionRecord', 1, 'active', '', 'deep', ?, ?, '', ?, ?, '', ?)`,
		id,
		title,
		body,
		createdAt,
		createdAt,
		structured,
	)
	if err != nil {
		t.Fatalf("insert DecisionRecord artifact %s: %v", id, err)
	}
}

func openDatabaseBeforeMigration41(t testing.TB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pre-v41.db")
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build pre-v41 DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v41 database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 41, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate through v40: %v", err)
	}
	return database
}

func assertDecisionEffectForeignKeysClean(t testing.TB, database *sql.DB) {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("DecisionRecord effect migration left a foreign-key violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("foreign_key_check rows: %v", err)
	}
}

func assertSQLiteObjectAbsent(
	t testing.TB,
	database *sql.DB,
	kind string,
	name string,
) {
	t.Helper()
	var count int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect absent SQLite %s %s: %v", kind, name, err)
	}
	if count != 0 {
		t.Fatalf("SQLite %s %s unexpectedly exists", kind, name)
	}
}

func decisionEffectDigest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func mustAuthorityValue[T any](
	t testing.TB,
	constructor func(string) (T, error),
	raw string,
) T {
	t.Helper()
	value, err := constructor(raw)
	if err != nil {
		t.Fatalf("construct authority value %q: %v", raw, err)
	}
	return value
}
