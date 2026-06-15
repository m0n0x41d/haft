// Package recall composes haft's keyword/graph artifact store with the optional
// embedding port into one hybrid retrieval layer. Embeddings AUGMENT FTS5+PPR;
// they never replace it. With no embedder, or on any embedder fault, Search
// degrades to the store's own FTS ranking — recall never hard-fails on the
// optional semantic layer (decision dec-20260605-fe77b358).
package recall

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/embedding"
	"github.com/m0n0x41d/haft/logger"
)

const (
	rrfK                  = 60.0 // standard Reciprocal Rank Fusion constant
	candidatePoolFactor   = 4    // pull this many × limit candidates before fusing
	minCandidatePool      = 24
	minSemanticSimilarity = 0.15 // cosine floor — below this a doc is a non-match, not a weak hit
	corpusEmbeddingBatch  = 16   // bound sidecar activation memory during cold corpus warm
)

// corpusKinds scopes the semantic index to decisions + notes — the prose where
// keyword recall misses paraphrase (operator scope: "start small").
var corpusKinds = []artifact.Kind{artifact.KindDecisionRecord, artifact.KindNote}

// ArtifactSource is the slice of the artifact store the hybrid layer needs.
type ArtifactSource interface {
	Search(ctx context.Context, query string, limit int) ([]*artifact.Artifact, error)
	ListByKind(ctx context.Context, kind artifact.Kind, limit int) ([]*artifact.Artifact, error)
}

// Searcher is the narrow retrieval surface the server's search handlers depend
// on. Both *artifact.Store (FTS only) and *Hybrid (FTS+semantic) satisfy it,
// so the call site stays agnostic to whether embeddings are active.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]*artifact.Artifact, error)
}

// Hybrid fuses FTS ranking from the store with cosine ranking from an in-memory
// embedding index. The corpus index is built in the BACKGROUND: search never
// blocks on a cold (or re-warming) index — it returns FTS5+PPR until the index
// is ready, then fuses. The embedder is resolved lazily (so startup never
// blocks on a first-run model download) and vectors are cached to
// artifact_embeddings so re-warms re-embed only changed artifacts.
type Hybrid struct {
	source      ArtifactSource
	newEmbedder func() (embedding.Embedder, error)
	cache       vectorCache

	mu                  sync.Mutex
	embedder            embedding.Embedder
	embedderUnavailable bool
	index               *embedding.Index
	byID                map[string]*artifact.Artifact
	built               bool // a usable index has been installed at least once
	building            bool // a background build is in flight
	dirty               bool // corpus changed since the last build → rebuild needed
}

// NewHybrid builds the hybrid layer. newEmbedder is invoked once, in the
// background, to spawn/connect the embedder — a nil factory (or one that errors,
// e.g. the sidecar is absent) makes Search transparently delegate to the
// store's FTS ranking for the rest of the session.
func NewHybrid(source ArtifactSource, newEmbedder func() (embedding.Embedder, error), db *sql.DB) *Hybrid {
	return &Hybrid{source: source, newEmbedder: newEmbedder, cache: vectorCache{db: db}}
}

// Prewarm kicks the background corpus build so the index is ready before the
// first search rather than warming on it. Non-blocking.
func (h *Hybrid) Prewarm() {
	if h.newEmbedder != nil {
		h.ensureWarming()
	}
}

// Invalidate marks the corpus stale so the next search triggers a background
// rebuild — cheap, since cached vectors are reused and only changed artifacts
// re-embed. The existing index keeps serving until the rebuild swaps in, so
// there is no semantic-blind window after the first warm. Call it when a
// decision/note is created or updated.
func (h *Hybrid) Invalidate() {
	h.mu.Lock()
	h.dirty = true
	h.mu.Unlock()
	h.ensureWarming()
}

// Warm builds the index synchronously, blocking until ready. Production search
// uses the non-blocking background path; this is for callers that must have the
// semantic index available before searching (tests, explicit pre-warm).
func (h *Hybrid) Warm(ctx context.Context) error {
	index, byID, err := h.buildIndex(ctx)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if index != nil {
		h.index = index
		h.byID = byID
	}
	h.built = true
	return nil
}

// Search returns artifacts for the query, fusing keyword and semantic ranking.
// It always falls back to FTS-only on any semantic-path issue — a cold index, a
// rebuild in flight, a missing embedder, or an embed fault — never erroring out
// recall for the optional semantic layer.
func (h *Hybrid) Search(ctx context.Context, query string, limit int) ([]*artifact.Artifact, error) {
	if limit <= 0 {
		limit = 20
	}
	if h.newEmbedder == nil {
		return h.source.Search(ctx, query, limit)
	}

	h.ensureWarming()
	embedder, index, byID, ready := h.snapshot()
	if !ready {
		return h.source.Search(ctx, query, limit)
	}

	pool := max(limit*candidatePoolFactor, minCandidatePool)

	ftsResults, err := h.source.Search(ctx, query, pool)
	if err != nil {
		return nil, err
	}

	queryVectors, err := embedder.Embed(ctx, embedding.RoleQuery, []string{query})
	if err != nil || len(queryVectors) == 0 {
		if err != nil {
			logger.Warn().Err(err).Msg("hybrid query embed failed — returning FTS5+PPR results")
		}
		return truncate(ftsResults, limit), nil
	}
	semantic := index.Search(queryVectors[0], pool)

	fused := fuseRRF([][]string{idsOf(ftsResults), semanticIDs(semantic, minSemanticSimilarity)}, rrfK)
	return resolveOrder(byID, fused, ftsResults, limit), nil
}

// snapshot returns the current ready index, embedder and lookup — even while a
// rebuild is in flight, the existing index keeps serving until the new one
// swaps in (no semantic-blind window mid-rewarm).
func (h *Hybrid) snapshot() (embedding.Embedder, *embedding.Index, map[string]*artifact.Artifact, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.index == nil || h.index.Len() == 0 {
		return nil, nil, nil, false
	}
	return h.embedder, h.index, h.byID, true
}

// ensureWarming starts a background corpus build when one is needed (cold or
// dirty) and not already running. Non-blocking.
func (h *Hybrid) ensureWarming() {
	h.mu.Lock()
	if h.embedderUnavailable || h.building || (h.built && !h.dirty) {
		h.mu.Unlock()
		return
	}
	h.building = true
	h.dirty = false
	h.mu.Unlock()
	go h.warmAsync()
}

func (h *Hybrid) warmAsync() {
	index, byID, err := h.buildIndex(context.Background())
	h.mu.Lock()
	h.building = false
	if err != nil {
		h.mu.Unlock()
		logger.Warn().Err(err).Msg("hybrid recall corpus warm failed — search uses FTS5+PPR until the next attempt")
		return
	}
	if index != nil {
		h.index = index
		h.byID = byID
	}
	h.built = true
	needRewarm := h.dirty
	h.mu.Unlock()
	if needRewarm {
		h.ensureWarming()
	}
}

// resolveEmbedder spawns the embedder once and reuses it across rebuilds. A nil
// factory or factory error (sidecar absent / disabled) marks it permanently
// unavailable, so the build does not respawn on every search — that is a
// degrade to FTS5+PPR, not a failure.
func (h *Hybrid) resolveEmbedder() embedding.Embedder {
	h.mu.Lock()
	if h.embedderUnavailable || h.newEmbedder == nil {
		h.embedderUnavailable = true
		h.mu.Unlock()
		return nil
	}
	if h.embedder != nil {
		embedder := h.embedder
		h.mu.Unlock()
		return embedder
	}
	newEmbedder := h.newEmbedder
	h.mu.Unlock()

	embedder, err := newEmbedder()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.embedder != nil {
		return h.embedder
	}
	if err != nil {
		h.embedderUnavailable = true
		logger.Info().Err(err).Msg("embedding sidecar unavailable — hybrid recall degrades to FTS5+PPR")
		return nil
	}
	if embedder == nil {
		h.embedderUnavailable = true
		return nil
	}
	h.embedder = embedder
	return embedder
}

// buildIndex embeds the decision/note corpus (reusing cached vectors) into a
// fresh in-memory cosine index. Runs OFF the lock so the slow embed never
// blocks concurrent searches. Returns a nil index (no error) when the embedder
// is unavailable.
func (h *Hybrid) buildIndex(ctx context.Context) (*embedding.Index, map[string]*artifact.Artifact, error) {
	embedder := h.resolveEmbedder()
	if embedder == nil {
		return nil, nil, nil
	}

	descriptor := embedder.Descriptor()
	cached, err := h.cache.load(ctx, descriptor)
	if err != nil {
		return nil, nil, err
	}

	corpus, err := h.loadCorpus(ctx)
	if err != nil {
		return nil, nil, err
	}

	byID := make(map[string]*artifact.Artifact, len(corpus))
	vectors := make(map[string][]float32, len(corpus))
	var missIDs, missHashes, missTexts []string

	for _, item := range corpus {
		byID[item.Meta.ID] = item
		text := corpusText(item)
		if text == "" {
			continue
		}
		hash := contentHash(text)
		if hit, ok := cached[item.Meta.ID]; ok && hit.ContentHash == hash {
			vectors[item.Meta.ID] = hit.Vector
			continue
		}
		missIDs = append(missIDs, item.Meta.ID)
		missHashes = append(missHashes, hash)
		missTexts = append(missTexts, text)
	}

	if err := h.embedMisses(ctx, embedder, descriptor, missIDs, missHashes, missTexts, vectors); err != nil {
		return nil, nil, err
	}

	index := embedding.NewIndex(len(vectors))
	for id, vector := range vectors {
		index.Add(id, vector)
	}
	return index, byID, nil
}

func (h *Hybrid) loadCorpus(ctx context.Context) ([]*artifact.Artifact, error) {
	var corpus []*artifact.Artifact
	for _, kind := range corpusKinds {
		items, err := h.source.ListByKind(ctx, kind, 0)
		if err != nil {
			return nil, err
		}
		corpus = append(corpus, items...)
	}
	return corpus, nil
}

// embedMisses embeds the not-cached artifacts as documents and writes them
// back to the cache, populating vectors in place.
func (h *Hybrid) embedMisses(
	ctx context.Context,
	embedder embedding.Embedder,
	descriptor embedding.Descriptor,
	ids, hashes, texts []string,
	vectors map[string][]float32,
) error {
	if len(ids) == 0 {
		return nil
	}
	for start := 0; start < len(ids); start += corpusEmbeddingBatch {
		end := min(start+corpusEmbeddingBatch, len(ids))
		err := h.embedMissBatch(
			ctx,
			embedder,
			descriptor,
			ids[start:end],
			hashes[start:end],
			texts[start:end],
			vectors,
		)
		if err != nil {
			return fmt.Errorf("embed corpus misses [%d:%d]: %w", start, end, err)
		}
	}
	return nil
}

func (h *Hybrid) embedMissBatch(
	ctx context.Context,
	embedder embedding.Embedder,
	descriptor embedding.Descriptor,
	ids, hashes, texts []string,
	vectors map[string][]float32,
) error {
	embedded, err := embedder.Embed(ctx, embedding.RoleDocument, texts)
	if err != nil {
		return err
	}
	if len(embedded) != len(ids) {
		return fmt.Errorf("embedded %d vectors for %d corpus misses", len(embedded), len(ids))
	}
	for index, id := range ids {
		vectors[id] = embedded[index]
		if err := h.cache.store(ctx, descriptor, id, hashes[index], embedded[index]); err != nil {
			return err
		}
	}
	return nil
}

// resolveOrder maps the fused id ordering back to artifacts, preferring the
// corpus lookup but falling back to FTS result objects for non-corpus kinds.
func resolveOrder(byID map[string]*artifact.Artifact, orderedIDs []string, ftsResults []*artifact.Artifact, limit int) []*artifact.Artifact {
	lookup := make(map[string]*artifact.Artifact, len(byID)+len(ftsResults))
	maps.Copy(lookup, byID)
	for _, item := range ftsResults {
		lookup[item.Meta.ID] = item
	}

	out := make([]*artifact.Artifact, 0, limit)
	for _, id := range orderedIDs {
		item, ok := lookup[id]
		if !ok {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// fuseRRF merges ranked id-lists by Reciprocal Rank Fusion: an id's score is
// the sum over lists of 1/(k + rank), so a strong hit in either list surfaces
// even if the other list misses it entirely. Robust to bm25/cosine scale gap.
func fuseRRF(ranked [][]string, k float64) []string {
	score := make(map[string]float64)
	for _, list := range ranked {
		for rank, id := range list {
			score[id] += 1.0 / (k + float64(rank) + 1.0)
		}
	}

	ids := make([]string, 0, len(score))
	for id := range score {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		if score[ids[i]] == score[ids[j]] {
			return ids[i] < ids[j]
		}
		return score[ids[i]] > score[ids[j]]
	})
	return ids
}

func idsOf(items []*artifact.Artifact) []string {
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.Meta.ID
	}
	return ids
}

// semanticIDs keeps only candidates at or above the cosine floor — a doc below
// it is a non-match and must not earn RRF rank credit that would let keyword-
// unrelated noise ride into the fused results.
func semanticIDs(scored []embedding.Scored, minScore float64) []string {
	ids := make([]string, 0, len(scored))
	for _, item := range scored {
		if item.Score < minScore {
			continue
		}
		ids = append(ids, item.ID)
	}
	return ids
}

func truncate(items []*artifact.Artifact, limit int) []*artifact.Artifact {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func corpusText(item *artifact.Artifact) string {
	return strings.TrimSpace(item.Meta.Title + "\n" + item.Body)
}

func contentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
