package artifact

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Create tables directly (not using migrations.go to keep tests self-contained)
	stmts := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active', context TEXT, mode TEXT,
			title TEXT NOT NULL, content TEXT NOT NULL, file_path TEXT,
			valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			search_keywords TEXT DEFAULT '', structured_data TEXT DEFAULT '')`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL, target_id TEXT NOT NULL, link_type TEXT NOT NULL,
			created_at TEXT NOT NULL, PRIMARY KEY (source_id, target_id, link_type))`,
		`CREATE TABLE evidence_items (
			id TEXT PRIMARY KEY, artifact_ref TEXT NOT NULL, type TEXT NOT NULL,
			content TEXT NOT NULL, verdict TEXT, carrier_ref TEXT,
			congruence_level INTEGER DEFAULT 3, formality_level INTEGER DEFAULT 5,
			claim_scope TEXT DEFAULT '[]',
			valid_until TEXT, created_at TEXT NOT NULL)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
		`CREATE TABLE fpf_state (
			context_id TEXT PRIMARY KEY,
			active_role TEXT,
			epistemic_debt_budget REAL DEFAULT 30.0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE audit_log (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL,
			operation TEXT NOT NULL)`,
		`CREATE VIRTUAL TABLE artifacts_fts USING fts5(id, title, content, kind, search_keywords, tokenize='porter unicode61')`,
		`CREATE TRIGGER artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
		END`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup: %v\nSQL: %s", err, s)
		}
	}

	return NewStore(db)
}

func TestCreateAndGet(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a := &Artifact{
		Meta: Meta{
			ID:      "note-20260316-001",
			Kind:    KindNote,
			Title:   "Use RWMutex for cache",
			Context: "auth",
			Mode:    ModeNote,
		},
		Body: "Using RWMutex instead of channels. Contention <0.1%.",
	}

	if err := store.Create(ctx, a); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, "note-20260316-001")
	if err != nil {
		t.Fatal(err)
	}

	if got.Meta.Title != "Use RWMutex for cache" {
		t.Errorf("title = %q, want %q", got.Meta.Title, "Use RWMutex for cache")
	}
	if got.Meta.Kind != KindNote {
		t.Errorf("kind = %q, want %q", got.Meta.Kind, KindNote)
	}
	if got.Body != "Using RWMutex instead of channels. Contention <0.1%." {
		t.Errorf("body = %q", got.Body)
	}
	if got.Meta.Status != StatusActive {
		t.Errorf("status = %q, want %q", got.Meta.Status, StatusActive)
	}
	if got.Meta.Version != 1 {
		t.Errorf("version = %d, want 1", got.Meta.Version)
	}
}

func TestUpdate(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a := &Artifact{
		Meta: Meta{ID: "prob-20260316-001", Kind: KindProblemCard, Title: "Webhook reliability"},
		Body: "Initial framing",
	}
	store.Create(ctx, a)

	a.Body = "Updated framing with constraints"
	a.Meta.Status = StatusRefreshDue
	if err := store.Update(ctx, a); err != nil {
		t.Fatal(err)
	}

	got, _ := store.Get(ctx, "prob-20260316-001")
	if got.Body != "Updated framing with constraints" {
		t.Errorf("body not updated")
	}
	if got.Meta.Version != 2 {
		t.Errorf("version = %d, want 2", got.Meta.Version)
	}
}

func TestListByKind(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{Meta: Meta{ID: "note-001", Kind: KindNote, Title: "A"}, Body: "a"})
	store.Create(ctx, &Artifact{Meta: Meta{ID: "note-002", Kind: KindNote, Title: "B"}, Body: "b"})
	store.Create(ctx, &Artifact{Meta: Meta{ID: "prob-001", Kind: KindProblemCard, Title: "P"}, Body: "p"})

	notes, err := store.ListByKind(ctx, KindNote, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestSearch(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "NATS JetStream for events"},
		Body: "Selected NATS over Kafka for domain event infrastructure",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "note-001", Kind: KindNote, Title: "Redis config"},
		Body: "Using Redis for session cache with 15min TTL",
	})

	results, err := store.Search(ctx, "NATS Kafka events", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for NATS")
	}
	if results[0].Meta.ID != "dec-001" {
		t.Errorf("expected dec-001 as top result, got %s", results[0].Meta.ID)
	}
}

func TestSearch_KeywordsEnrichment(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// Create an artifact about Redis session cache, with search keywords
	// that include "caching" and "in-memory" — terms NOT in the title or body
	store.Create(ctx, &Artifact{
		Meta:           Meta{ID: "dec-002", Kind: KindDecisionRecord, Title: "Redis for session store"},
		Body:           "Selected Redis for session persistence with 15min TTL",
		SearchKeywords: "cache caching in-memory key-value nosql session store redis",
	})

	// Search for "caching strategy" — no match in title/body, but matches keywords
	results, err := store.Search(ctx, "caching strategy", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results via keyword enrichment — 'caching' should match search_keywords")
	}
	if results[0].Meta.ID != "dec-002" {
		t.Errorf("expected dec-002, got %s", results[0].Meta.ID)
	}

	// Search for "nosql" — only in keywords, not in title or body
	results2, err := store.Search(ctx, "nosql", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results2) == 0 {
		t.Fatal("expected search results for 'nosql' via keywords")
	}
}

func TestLinks(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{Meta: Meta{ID: "prob-001", Kind: KindProblemCard, Title: "P"}, Body: "p"})
	store.Create(ctx, &Artifact{Meta: Meta{ID: "sol-001", Kind: KindSolutionPortfolio, Title: "S"}, Body: "s"})

	store.AddLink(ctx, "sol-001", "prob-001", "based_on")

	links, _ := store.GetLinks(ctx, "sol-001")
	if len(links) != 1 || links[0].Ref != "prob-001" {
		t.Errorf("expected link to prob-001, got %+v", links)
	}

	backlinks, _ := store.GetBacklinks(ctx, "prob-001")
	if len(backlinks) != 1 || backlinks[0].Ref != "sol-001" {
		t.Errorf("expected backlink from sol-001, got %+v", backlinks)
	}
}

func TestAffectedFiles(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "D"}, Body: "d"})

	files := []AffectedFile{
		{Path: "internal/events/producer.go", Hash: "abc123"},
		{Path: "internal/events/consumer.go", Hash: "def456"},
	}
	store.SetAffectedFiles(ctx, "dec-001", files)

	got, _ := store.GetAffectedFiles(ctx, "dec-001")
	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got))
	}

	// Search by affected file
	results, _ := store.SearchByAffectedFile(ctx, "internal/events/producer.go")
	if len(results) != 1 || results[0].Meta.ID != "dec-001" {
		t.Errorf("expected dec-001 for file search")
	}
}

func TestFindStaleDecisions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	future := time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "Stale", ValidUntil: past},
		Body: "old decision",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-002", Kind: KindDecisionRecord, Title: "Fresh", ValidUntil: future},
		Body: "recent decision",
	})

	stale, err := store.FindStaleDecisions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale, got %d", len(stale))
	}
	if stale[0].Meta.ID != "dec-001" {
		t.Errorf("expected dec-001, got %s", stale[0].Meta.ID)
	}
}

func TestFindStaleDecisions_DateOnlyCurrentDayNotStale(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-today", Kind: KindDecisionRecord, Title: "Today", ValidUntil: today},
		Body: "still valid through end of day",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "dec-yesterday", Kind: KindDecisionRecord, Title: "Yesterday", ValidUntil: yesterday},
		Body: "expired at prior end of day",
	})

	stale, err := store.FindStaleDecisions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale decision, got %d", len(stale))
	}
	if stale[0].Meta.ID != "dec-yesterday" {
		t.Fatalf("expected dec-yesterday, got %s", stale[0].Meta.ID)
	}
}

func TestFindStaleArtifacts_DateOnlyCurrentDayNotStale(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")

	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "prob-today", Kind: KindProblemCard, Title: "Today", ValidUntil: today},
		Body: "still valid through end of day",
	})
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "prob-yesterday", Kind: KindProblemCard, Title: "Yesterday", ValidUntil: yesterday},
		Body: "expired at prior end of day",
	})

	stale, err := store.FindStaleArtifacts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale artifact, got %d", len(stale))
	}
	if stale[0].Meta.ID != "prob-yesterday" {
		t.Fatalf("expected prob-yesterday, got %s", stale[0].Meta.ID)
	}
}

func TestEvidenceItems(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, &Artifact{Meta: Meta{ID: "dec-001", Kind: KindDecisionRecord, Title: "D"}, Body: "d"})

	item := &EvidenceItem{
		ID:              "evid-001",
		Type:            "benchmark",
		Content:         "Load test: 100k events/sec, p99 < 50ms",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  7,
		ClaimScope:      []string{"throughput", "latency", "throughput"},
	}
	if err := store.AddEvidenceItem(ctx, item, "dec-001"); err != nil {
		t.Fatal(err)
	}

	items, err := store.GetEvidenceItems(ctx, "dec-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Content != "Load test: 100k events/sec, p99 < 50ms" {
		t.Errorf("content mismatch")
	}
	if items[0].FormalityLevel != 2 {
		t.Errorf("formality mismatch: got %d want 2", items[0].FormalityLevel)
	}
	if got := strings.Join(items[0].ClaimScope, ","); got != "latency,throughput" {
		t.Errorf("claim scope mismatch: got %q", got)
	}
}

func TestNextSequence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	seq, _ := store.NextSequence(ctx, KindNote)
	if seq != 1 {
		t.Errorf("first sequence = %d, want 1", seq)
	}

	today := time.Now().Format("20060102")
	store.Create(ctx, &Artifact{
		Meta: Meta{ID: "note-" + today + "-001", Kind: KindNote, Title: "A"}, Body: "a"})

	seq, _ = store.NextSequence(ctx, KindNote)
	if seq != 2 {
		t.Errorf("second sequence = %d, want 2", seq)
	}
}
