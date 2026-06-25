package project

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/m0n0x41d/haft/internal/embedding"
)

// tempIndexSchema mirrors OpenIndex's inline schema for a throwaway test index.
var tempIndexSchema = []string{
	`CREATE TABLE global_decisions (project_id TEXT NOT NULL, project_name TEXT NOT NULL, decision_id TEXT NOT NULL,
		title TEXT NOT NULL, selected_title TEXT, why_selected TEXT, weakest_link TEXT, primary_lang TEXT,
		created_at TEXT NOT NULL, PRIMARY KEY (project_id, decision_id))`,
	`CREATE VIRTUAL TABLE global_fts USING fts5(title, selected_title, why_selected, weakest_link,
		content='global_decisions', content_rowid='rowid', tokenize='porter unicode61')`,
	`CREATE TRIGGER global_fts_insert AFTER INSERT ON global_decisions BEGIN
		INSERT INTO global_fts(rowid, title, selected_title, why_selected, weakest_link)
		VALUES (new.rowid, new.title, new.selected_title, new.why_selected, new.weakest_link); END`,
	`CREATE TABLE global_embeddings (project_id TEXT NOT NULL, decision_id TEXT NOT NULL, provider TEXT NOT NULL,
		model TEXT NOT NULL, dim INTEGER NOT NULL, content_hash TEXT NOT NULL, vector BLOB NOT NULL,
		updated_at TEXT NOT NULL, PRIMARY KEY (project_id, decision_id, provider, model, dim))`,
}

func newTempIndexStore(t *testing.T) *IndexStore {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatalf("open temp index: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, s := range tempIndexSchema {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("temp index schema: %v", err)
		}
	}
	return &IndexStore{db: db}
}

const crossRankMiss = 1 << 30

// rankIn returns the 0-based rank of goldID in the results, or crossRankMiss if
// absent. Shared by the committed tests and the local eval harness.
func rankIn(results []IndexRecall, goldID string) int {
	for rank, r := range results {
		if r.ProjectName+"|"+r.DecisionID == goldID {
			return rank
		}
	}
	return crossRankMiss
}

func seedDecision(t *testing.T, ctx context.Context, store *IndexStore, projectID, decisionID, title, selected, why string) {
	t.Helper()
	if err := store.WriteDecision(ctx, IndexEntry{
		ProjectID: projectID, ProjectName: projectID, DecisionID: decisionID,
		Title: title, SelectedTitle: selected, WhySelected: why, PrimaryLang: "go", CreatedAt: "2026-01-01",
	}); err != nil {
		t.Fatalf("seed %s: %v", decisionID, err)
	}
}

// TestCrossHybridDegradesToFTS proves a nil-embedder CrossHybrid returns a
// byte-identical result set to IndexStore.Search — recall never changes when the
// sidecar is absent.
func TestCrossHybridDegradesToFTS(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	seedDecision(t, ctx, store, "haft", "dec-1", "Hybrid recall over FTS5 and PPR", "R2 sidecar", "embeddings augment keyword recall")
	seedDecision(t, ctx, store, "haft", "dec-2", "Self-healing sidecar respawn on fault", "respawn once", "a crash costs one query")
	seedDecision(t, ctx, store, "haft", "dec-3", "Non-blocking background warm", "lazy index build", "first search returns FTS while warming")

	hybrid := NewCrossHybrid(store, nil) // nil factory => degrade to FTS

	for _, q := range []string{"recall", "sidecar respawn crash", "warm background", "nonexistent zzz"} {
		want, err := store.Search(ctx, q, "other", "go", 5)
		if err != nil {
			t.Fatalf("store search %q: %v", q, err)
		}
		got, err := hybrid.Search(ctx, q, "other", "go", 5)
		if err != nil {
			t.Fatalf("hybrid search %q: %v", q, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("degraded hybrid differs from FTS for %q:\n want %#v\n got  %#v", q, want, got)
		}
	}
}

// TestIndexStoreORFallback proves the OR-fallback surfaces a decision that strict
// AND would miss: a query whose second term appears in no document still returns
// the gold via the matching first term.
func TestIndexStoreORFallback(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	seedDecision(t, ctx, store, "haft", "dec-brier", "Calibration of decision predictions via Brier scoring", "decomposed Brier", "measure overconfidence in forecasts")
	seedDecision(t, ctx, store, "haft", "dec-other", "Ceremony auto-scaler safety floor", "hard floor", "block risky changes from tactical")

	// "brier" matches dec-brier; "zzznomatch" matches nothing -> strict AND is
	// empty, OR-fallback recovers dec-brier.
	results, err := store.Search(ctx, "brier zzznomatch", "other", "go", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if rankIn(results, "haft|dec-brier") >= crossRankMiss {
		t.Fatalf("OR-fallback should surface dec-brier on an AND-impossible query; got %d results", len(results))
	}
}

// fakeEmbedderProj maps text topics to fixed orthogonal unit vectors so the test
// controls cosine ranking exactly. Mutex-guarded like the real sidecar adapter.
type fakeEmbedderProj struct {
	mu         sync.Mutex
	batchSizes []int
}

func (f *fakeEmbedderProj) Descriptor() embedding.Descriptor {
	return embedding.Descriptor{Provider: "fake", Model: "topic-v1", Dimensions: 3}
}

func (f *fakeEmbedderProj) Embed(_ context.Context, _ embedding.Role, texts []string) ([][]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batchSizes = append(f.batchSizes, len(texts))
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = projTopicVector(text)
	}
	return out, nil
}

func (f *fakeEmbedderProj) Close() error { return nil }

func (f *fakeEmbedderProj) batches() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.batchSizes...)
}

func blockingCrossFactory(embedder embedding.Embedder) (func() (embedding.Embedder, error), <-chan struct{}, func()) {
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	var releaseOnce sync.Once
	factory := func() (embedding.Embedder, error) {
		startedOnce.Do(func() {
			close(started)
		})
		<-release
		return embedder, nil
	}
	releaseFactory := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	return factory, started, releaseFactory
}

func waitForCrossHybridIdle(t *testing.T, hybrid *CrossHybrid) {
	t.Helper()

	deadline := time.Now().Add(1 * time.Second)
	for {
		hybrid.mu.Lock()
		building := hybrid.building
		hybrid.mu.Unlock()
		if !building {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("cross hybrid warm did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func projTopicVector(text string) []float32 {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "vector"), strings.Contains(lower, "dense"), strings.Contains(lower, "representation"), strings.Contains(lower, "fastembed"), strings.Contains(lower, "gemma"):
		return []float32{1, 0, 0}
	case strings.Contains(lower, "migration"), strings.Contains(lower, "database"):
		return []float32{0, 1, 0}
	case strings.Contains(lower, "oauth"), strings.Contains(lower, "token"):
		return []float32{0, 0, 1}
	default:
		return []float32{0, 0, 0}
	}
}

// TestCrossHybridFusesSemanticHit proves a decision that FTS misses entirely
// (no shared query words) but is semantically nearest is surfaced by the fusion.
func TestCrossHybridFusesSemanticHit(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	seedDecision(t, ctx, store, "haft", "dec-a", "Database migration safety gate", "block risky migration", "additive only")
	seedDecision(t, ctx, store, "haft", "dec-b", "Rust fastembed gemma sidecar", "local embeddings", "augment keyword search")
	seedDecision(t, ctx, store, "haft", "dec-c", "OAuth token rotation", "rotate secrets", "scheduled rollover")

	hybrid := NewCrossHybrid(store, func() (embedding.Embedder, error) { return &fakeEmbedderProj{}, nil })
	if err := hybrid.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}

	// Query shares NO words with any document; only the embedding connects it to
	// dec-b (vector topic). FTS returns nothing; fusion must surface dec-b.
	results, err := hybrid.Search(ctx, "compute dense vector representations", "other", "go", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 || results[0].ProjectName+"|"+results[0].DecisionID != "haft|dec-b" {
		t.Fatalf("expected dec-b promoted by semantic fusion, got %v", recallIDs(results))
	}
}

func TestCrossHybridWarmBatchesCorpusMisses(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	for i := range crossEmbeddingBatch*2 + 3 {
		seedDecision(
			t,
			ctx,
			store,
			"haft",
			fmt.Sprintf("dec-%02d", i),
			fmt.Sprintf("Rust fastembed gemma sidecar %02d", i),
			"local embeddings",
			"augment keyword search",
		)
	}

	embedder := &fakeEmbedderProj{}
	hybrid := NewCrossHybrid(store, func() (embedding.Embedder, error) { return embedder, nil })
	if err := hybrid.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}

	want := []int{crossEmbeddingBatch, crossEmbeddingBatch, 3}
	if got := embedder.batches(); !reflect.DeepEqual(want, got) {
		t.Fatalf("cross-project warm embed batch sizes = %v, want %v", got, want)
	}
}

// TestCrossHybridFiltersSelfSemanticHits proves the cross-project surface keeps
// its boundary even when the semantic index contains the current project's own
// decisions. The global index may warm over all projects, but Search must only
// return other-project rows.
func TestCrossHybridFiltersSelfSemanticHits(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	seedDecision(t, ctx, store, "current", "dec-self", "Rust fastembed gemma sidecar", "local embeddings", "augment keyword search")
	seedDecision(t, ctx, store, "other", "dec-other", "Rust fastembed gemma sidecar", "local embeddings", "augment keyword search")

	hybrid := NewCrossHybrid(store, func() (embedding.Embedder, error) { return &fakeEmbedderProj{}, nil })
	if err := hybrid.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}

	results, err := hybrid.Search(ctx, "compute dense vector representations", "current", "go", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if rankIn(results, "current|dec-self") < crossRankMiss {
		t.Fatalf("self-project semantic hit leaked into cross-project recall: %v", recallIDs(results))
	}
	if rankIn(results, "other|dec-other") >= crossRankMiss {
		t.Fatalf("other-project semantic hit should remain after filtering self hits: %v", recallIDs(results))
	}
}

func TestCrossHybridSearchNonBlockingDuringColdEmbedderStart(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	seedDecision(t, ctx, store, "haft", "dec-brier", "Calibration of decision predictions via Brier scoring", "decomposed Brier", "measure overconfidence in forecasts")
	seedDecision(t, ctx, store, "haft", "dec-other", "Ceremony auto-scaler safety floor", "hard floor", "block risky changes from tactical")

	factory, started, releaseFactory := blockingCrossFactory(&fakeEmbedderProj{})
	defer releaseFactory()

	hybrid := NewCrossHybrid(store, factory)
	hybrid.Prewarm()
	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("background cross-project embedder start did not begin")
	}

	want, err := store.Search(ctx, "brier zzznomatch", "other", "go", 5)
	if err != nil {
		t.Fatalf("store search: %v", err)
	}

	type searchResult struct {
		results []IndexRecall
		err     error
	}
	done := make(chan searchResult, 1)
	go func() {
		results, err := hybrid.Search(ctx, "brier zzznomatch", "other", "go", 5)
		done <- searchResult{results: results, err: err}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("hybrid search: %v", got.err)
		}
		if !reflect.DeepEqual(want, got.results) {
			t.Fatalf("cold embedder degrade not byte-identical to FTS:\n want %#v\n got  %#v", want, got.results)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Search blocked behind cold cross-project embedder startup")
	}

	releaseFactory()
	waitForCrossHybridIdle(t, hybrid)
}

// queryFaultEmbedder embeds documents fine (so Warm builds a ready index) but
// faults on query embeds — simulating a sidecar that crashes/times out on the
// query call after warm.
type queryFaultEmbedder struct{ mu sync.Mutex }

func (f *queryFaultEmbedder) Descriptor() embedding.Descriptor {
	return embedding.Descriptor{Provider: "fake", Model: "topic-v1", Dimensions: 3}
}

func (f *queryFaultEmbedder) Embed(_ context.Context, role embedding.Role, texts []string) ([][]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if role == embedding.RoleQuery {
		return nil, fmt.Errorf("simulated query embed fault")
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = projTopicVector(text)
	}
	return out, nil
}

func (f *queryFaultEmbedder) Close() error { return nil }

// TestCrossHybridEmbedFaultDegradesToFTS proves that when the query embed faults
// AFTER the index is warm, Search still returns the full AND+OR IndexStore.Search
// result (byte-identical), not the AND-only pool — the degrade contract holds on
// route 3, not just on the nil-embedder route.
func TestCrossHybridEmbedFaultDegradesToFTS(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	seedDecision(t, ctx, store, "haft", "dec-brier", "Calibration of decision predictions via Brier scoring", "decomposed Brier", "measure overconfidence in forecasts")
	seedDecision(t, ctx, store, "haft", "dec-other", "Rust fastembed gemma sidecar", "local embeddings", "augment keyword recall")

	hybrid := NewCrossHybrid(store, func() (embedding.Embedder, error) { return &queryFaultEmbedder{}, nil })
	if err := hybrid.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}

	// "brier zzznomatch" AND-misses (zzznomatch absent) but OR-hits dec-brier.
	// The query embed faults, so the degrade floor must be the full AND+OR Search.
	want, err := store.Search(ctx, "brier zzznomatch", "other", "go", 5)
	if err != nil {
		t.Fatalf("store search: %v", err)
	}
	got, err := hybrid.Search(ctx, "brier zzznomatch", "other", "go", 5)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("embed-fault degrade not byte-identical to FTS:\n want %#v\n got  %#v", want, got)
	}
}

// TestCrossHybridPreservesRecallFloor proves the additive invariant: a healthy
// embedder must never make recall WORSE than IndexStore.Search. A decision that
// OR-fallback surfaces (shares one term) but whose embedding is far from the
// query (cosine < floor) must still appear via the recall-floor top-up.
func TestCrossHybridPreservesRecallFloor(t *testing.T) {
	ctx := context.Background()
	store := newTempIndexStore(t)
	// dec-brier shares the token "brier" with the query; its topic vector is the
	// zero vector (no topic word matches), so query-cosine is 0 < 0.15 floor.
	seedDecision(t, ctx, store, "haft", "dec-brier", "Calibration scoring with brier metric", "brier", "overconfidence")
	seedDecision(t, ctx, store, "haft", "dec-mig", "Database migration safety gate", "additive migration", "block risky changes")

	hybrid := NewCrossHybrid(store, func() (embedding.Embedder, error) { return &fakeEmbedderProj{}, nil })
	if err := hybrid.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}

	// AND-impossible ("zzznomatch" absent), low query-cosine to dec-brier; only
	// OR-fallback surfaces it. Hybrid recall must be >= FTS recall.
	results, err := hybrid.Search(ctx, "brier zzznomatch", "other", "go", 5)
	if err != nil {
		t.Fatalf("hybrid search: %v", err)
	}
	if rankIn(results, "haft|dec-brier") >= crossRankMiss {
		t.Fatalf("recall floor violated: a healthy hybrid dropped an OR-recoverable decision; got %v", recallIDs(results))
	}
}
