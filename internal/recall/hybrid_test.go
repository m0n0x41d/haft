package recall

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/embedding"
)

// fakeEmbedder maps text topics to fixed orthogonal unit vectors so the test
// controls the cosine ranking exactly. It counts Embed calls to prove caching.
// Embed is mutex-guarded like the real sidecar adapter, so background warms race
// cleanly under -race.
type fakeEmbedder struct {
	mu         sync.Mutex
	calls      int
	roles      []embedding.Role
	batchSizes []int
}

func (f *fakeEmbedder) Descriptor() embedding.Descriptor {
	return embedding.Descriptor{Provider: "fake", Model: "topic-v1", Dimensions: 3}
}

func (f *fakeEmbedder) Embed(_ context.Context, role embedding.Role, texts []string) ([][]float32, error) {
	f.mu.Lock()
	f.calls++
	f.roles = append(f.roles, role)
	f.batchSizes = append(f.batchSizes, len(texts))
	f.mu.Unlock()
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = topicVector(text)
	}
	return out, nil
}

func (f *fakeEmbedder) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeEmbedder) documentBatchSizes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []int{}
	for index, role := range f.roles {
		if role == embedding.RoleDocument {
			out = append(out, f.batchSizes[index])
		}
	}
	return out
}

func (f *fakeEmbedder) Close() error { return nil }

type blockingEmbedder struct {
	fakeEmbedder
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingEmbedder() *blockingEmbedder {
	return &blockingEmbedder{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (b *blockingEmbedder) Embed(ctx context.Context, role embedding.Role, texts []string) ([][]float32, error) {
	if role == embedding.RoleDocument {
		b.once.Do(func() {
			close(b.started)
			<-b.release
		})
	}
	return b.fakeEmbedder.Embed(ctx, role, texts)
}

func staticFactory(embedder embedding.Embedder) func() (embedding.Embedder, error) {
	return func() (embedding.Embedder, error) { return embedder, nil }
}

func blockingFactory(embedder embedding.Embedder) (func() (embedding.Embedder, error), <-chan struct{}, func()) {
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

func topicVector(text string) []float32 {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "embedding"), strings.Contains(lower, "vector"), strings.Contains(lower, "fastembed"):
		return []float32{1, 0, 0}
	case strings.Contains(lower, "migration"), strings.Contains(lower, "database"):
		return []float32{0, 1, 0}
	case strings.Contains(lower, "auth"), strings.Contains(lower, "oauth"), strings.Contains(lower, "token"):
		return []float32{0, 0, 1}
	default:
		return []float32{0, 0, 0}
	}
}

// fakeSource returns a fixed FTS ordering and a fixed corpus.
type fakeSource struct {
	corpus   []*artifact.Artifact
	ftsOrder []*artifact.Artifact
}

func (s fakeSource) Search(_ context.Context, _ string, limit int) ([]*artifact.Artifact, error) {
	return truncate(s.ftsOrder, limit), nil
}

func (s fakeSource) ListByKind(_ context.Context, kind artifact.Kind, _ int) ([]*artifact.Artifact, error) {
	var out []*artifact.Artifact
	for _, item := range s.corpus {
		if item.Meta.Kind == kind {
			out = append(out, item)
		}
	}
	return out, nil
}

type mutableFakeSource struct {
	mu       sync.Mutex
	corpus   []*artifact.Artifact
	ftsOrder []*artifact.Artifact
}

func (s *mutableFakeSource) Search(_ context.Context, _ string, limit int) ([]*artifact.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return truncate(s.ftsOrder, limit), nil
}

func (s *mutableFakeSource) ListByKind(_ context.Context, kind artifact.Kind, _ int) ([]*artifact.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*artifact.Artifact
	for _, item := range s.corpus {
		if item.Meta.Kind == kind {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *mutableFakeSource) replace(corpus, ftsOrder []*artifact.Artifact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.corpus = corpus
	s.ftsOrder = ftsOrder
}

func decisionArtifact(id, title, body string) *artifact.Artifact {
	item := &artifact.Artifact{Body: body}
	item.Meta.ID = id
	item.Meta.Title = title
	item.Meta.Kind = artifact.KindDecisionRecord
	return item
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	store, err := db.NewStore(filepath.Join(t.TempDir(), "recall.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store.GetRawDB()
}

func waitForHybridIdle(t *testing.T, hybrid *Hybrid) {
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
			t.Fatal("hybrid warm did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestHybridPromotesSemanticHit is the killer case: the document that FTS ranks
// LAST is the one semantically closest to the query, and fusion lifts it to the
// top — exactly the gap embeddings exist to close.
func TestHybridPromotesSemanticHit(t *testing.T) {
	d1 := decisionArtifact("dec-1", "Migration safety gate", "block a risky database migration from tactical mode")
	d2 := decisionArtifact("dec-2", "Rust embedding sidecar", "fastembed gemma produces local vectors")
	d3 := decisionArtifact("dec-3", "Auth token rotation", "rotate oauth secrets on a schedule")

	source := fakeSource{
		corpus:   []*artifact.Artifact{d1, d2, d3},
		ftsOrder: []*artifact.Artifact{d1, d3, d2}, // d2 ranked last by keyword
	}
	embedder := &fakeEmbedder{}
	hybrid := NewHybrid(source, staticFactory(embedder), testDB(t))
	if err := hybrid.Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}

	results, err := hybrid.Search(context.Background(), "how do we run vectors locally", 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 || results[0].Meta.ID != "dec-2" {
		t.Fatalf("expected dec-2 (semantic match) promoted to top, got %s", orderIDs(results))
	}
}

// TestHybridCachesCorpusEmbeddings proves the second warm reuses the cache: a
// fresh Hybrid over the same DB embeds only the query, not the corpus again.
func TestHybridCachesCorpusEmbeddings(t *testing.T) {
	conn := testDB(t)
	corpus := []*artifact.Artifact{
		decisionArtifact("dec-1", "Rust embedding sidecar", "fastembed gemma local vectors"),
		decisionArtifact("dec-2", "Auth token rotation", "rotate oauth secrets"),
	}
	source := fakeSource{corpus: corpus, ftsOrder: corpus}

	first := &fakeEmbedder{}
	firstHybrid := NewHybrid(source, staticFactory(first), conn)
	if err := firstHybrid.Warm(context.Background()); err != nil {
		t.Fatalf("first warm: %v", err)
	}
	if _, err := firstHybrid.Search(context.Background(), "vectors", 5); err != nil {
		t.Fatalf("first search: %v", err)
	}
	// First run: one corpus document batch (warm) + one query (search) = 2 calls.
	if first.callCount() != 2 {
		t.Fatalf("first run embed calls = %d, want 2 (corpus + query)", first.callCount())
	}

	second := &fakeEmbedder{}
	secondHybrid := NewHybrid(source, staticFactory(second), conn)
	if err := secondHybrid.Warm(context.Background()); err != nil {
		t.Fatalf("second warm: %v", err)
	}
	if _, err := secondHybrid.Search(context.Background(), "vectors", 5); err != nil {
		t.Fatalf("second search: %v", err)
	}
	// Second run: corpus hits the cache, so only the query is embedded.
	if second.callCount() != 1 {
		t.Fatalf("second run embed calls = %d, want 1 (query only — corpus cached)", second.callCount())
	}
}

func TestHybridBatchesCorpusMissEmbeddings(t *testing.T) {
	count := corpusEmbeddingBatch + 3
	corpus := make([]*artifact.Artifact, 0, count)
	for index := 0; index < count; index++ {
		id := "dec-" + string(rune('a'+index))
		item := decisionArtifact(id, "Rust embedding sidecar", "fastembed gemma local vectors")
		corpus = append(corpus, item)
	}
	source := fakeSource{corpus: corpus, ftsOrder: corpus}
	embedder := &fakeEmbedder{}
	hybrid := NewHybrid(source, staticFactory(embedder), testDB(t))

	if err := hybrid.Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}

	got := embedder.documentBatchSizes()
	want := []int{corpusEmbeddingBatch, 3}
	if !sameInts(got, want) {
		t.Fatalf("document batch sizes = %v, want %v", got, want)
	}
}

// TestHybridDegradesWithoutEmbedder proves a nil embedder falls straight through
// to the store's FTS ordering.
func TestHybridDegradesWithoutEmbedder(t *testing.T) {
	corpus := []*artifact.Artifact{
		decisionArtifact("dec-1", "A", "alpha"),
		decisionArtifact("dec-2", "B", "beta"),
	}
	source := fakeSource{corpus: corpus, ftsOrder: corpus}
	hybrid := NewHybrid(source, nil, testDB(t))

	results, err := hybrid.Search(context.Background(), "anything", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if orderIDs(results) != "dec-1,dec-2" {
		t.Fatalf("nil embedder should pass through FTS order, got %s", orderIDs(results))
	}
}

// TestHybridSearchNonBlockingBeforeWarm proves a search issued before the index
// is ready returns promptly with results (no block, no error) instead of waiting
// on the corpus embed.
func TestHybridSearchNonBlockingBeforeWarm(t *testing.T) {
	corpus := []*artifact.Artifact{
		decisionArtifact("dec-1", "A", "alpha"),
		decisionArtifact("dec-2", "B", "beta"),
	}
	src := fakeSource{corpus: corpus, ftsOrder: corpus}
	hybrid := NewHybrid(src, staticFactory(&fakeEmbedder{}), testDB(t))

	// No Warm: the first search must return immediately with FTS results.
	results, err := hybrid.Search(context.Background(), "alpha", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("first search before warm should still return FTS results, got none")
	}
}

func TestHybridSearchNonBlockingDuringColdEmbedderStart(t *testing.T) {
	corpus := []*artifact.Artifact{
		decisionArtifact("dec-1", "A", "alpha"),
		decisionArtifact("dec-2", "B", "beta"),
	}
	src := fakeSource{corpus: corpus, ftsOrder: corpus}
	factory, started, releaseFactory := blockingFactory(&fakeEmbedder{})
	defer releaseFactory()

	hybrid := NewHybrid(src, factory, testDB(t))
	hybrid.Prewarm()
	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("background embedder start did not begin")
	}

	type searchResult struct {
		results []*artifact.Artifact
		err     error
	}
	done := make(chan searchResult, 1)
	go func() {
		results, err := hybrid.Search(context.Background(), "alpha", 5)
		done <- searchResult{results: results, err: err}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("search: %v", got.err)
		}
		if orderIDs(got.results) != "dec-1,dec-2" {
			t.Fatalf("cold embedder should pass through FTS order, got %s", orderIDs(got.results))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Search blocked behind cold embedder startup")
	}

	releaseFactory()
	waitForHybridIdle(t, hybrid)
}

// TestHybridInvalidatePicksUpNewArtifact proves Invalidate triggers a background
// rebuild that brings a decision recorded mid-session into the semantic index —
// so it becomes semantically searchable without a restart.
func TestHybridInvalidatePicksUpNewArtifact(t *testing.T) {
	original := decisionArtifact("dec-a", "Rust embedding sidecar", "fastembed gemma vectors")
	src := &fakeSource{corpus: []*artifact.Artifact{original}, ftsOrder: []*artifact.Artifact{original}}
	hybrid := NewHybrid(src, staticFactory(&fakeEmbedder{}), testDB(t))
	if err := hybrid.Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}

	// A new auth decision is recorded mid-session; keyword still ranks dec-a first.
	authDoc := decisionArtifact("dec-b", "Auth token rotation", "rotate oauth secrets on a schedule")
	src.corpus = append(src.corpus, authDoc)
	src.ftsOrder = []*artifact.Artifact{original, authDoc}
	hybrid.Invalidate()

	// Invalidate rebuilds in the background; poll until dec-b is semantically
	// promoted above the keyword-first dec-a.
	deadline := time.Now().Add(2 * time.Second)
	for {
		results, err := hybrid.Search(context.Background(), "rotate authentication credentials", 5)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) > 0 && results[0].Meta.ID == "dec-b" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Invalidate did not surface dec-b semantically within 2s; got %s", orderIDs(results))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHybridInvalidateDuringWarmRewarms(t *testing.T) {
	original := decisionArtifact("dec-a", "Rust embedding sidecar", "fastembed gemma vectors")
	authDoc := decisionArtifact("dec-b", "Auth token rotation", "rotate oauth secrets on a schedule")
	src := &mutableFakeSource{
		corpus:   []*artifact.Artifact{original},
		ftsOrder: []*artifact.Artifact{original, authDoc},
	}
	embedder := newBlockingEmbedder()
	hybrid := NewHybrid(src, staticFactory(embedder), testDB(t))

	hybrid.Prewarm()
	select {
	case <-embedder.started:
	case <-time.After(1 * time.Second):
		t.Fatal("background warm did not start")
	}

	src.replace([]*artifact.Artifact{original, authDoc}, []*artifact.Artifact{original, authDoc})
	hybrid.Invalidate()
	close(embedder.release)

	deadline := time.Now().Add(2 * time.Second)
	for {
		if embedder.callCount() >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Invalidate during warm did not schedule a second build; embed calls = %d", embedder.callCount())
		}
		time.Sleep(5 * time.Millisecond)
	}
	waitForHybridIdle(t, hybrid)

	results, err := hybrid.Search(context.Background(), "rotate authentication credentials", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 || results[0].Meta.ID != "dec-b" {
		t.Fatalf("second build did not surface dec-b semantically; got %s", orderIDs(results))
	}
}

func TestFuseRRF(t *testing.T) {
	// id "x" is rank0 in list A and rank0 in list B → must win.
	fused := fuseRRF([][]string{{"x", "y"}, {"x", "z"}}, rrfK)
	if fused[0] != "x" {
		t.Fatalf("RRF should rank the doubly-top-ranked id first, got %v", fused)
	}
}

func orderIDs(items []*artifact.Artifact) string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.Meta.ID
	}
	return strings.Join(ids, ",")
}

func sameInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
