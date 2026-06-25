package project

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/embedding"
	"github.com/m0n0x41d/haft/logger"
)

// CrossHybrid adds semantic recall to the cross-project index by fusing the
// FTS5 ranking (IndexStore.Search, now AND+OR) with cosine ranking from an
// in-memory embedding index. It mirrors recall.Hybrid's shape (lazy embedder,
// background warm, Invalidate, degrade-to-FTS) over IndexRecall rows WITHOUT
// importing or touching the shipped per-project recall.Hybrid. Embeddings
// AUGMENT; with no sidecar Search delegates byte-for-byte to IndexStore.Search.
const (
	crossRRFK             = 60.0
	crossCandidateFactor  = 4
	crossMinCandidatePool = 24
	crossMinSimilarity    = 0.15 // cosine floor — below this a doc is a non-match
	crossEmbeddingBatch   = 16   // bound sidecar activation memory during cross-project warm
)

// crossVectorCache persists cross-project decision embeddings in
// global_embeddings, keyed by the decision identity (project_id, decision_id)
// plus the model contract. Mirrors recall's vectorCache over the index DB.
type crossVectorCache struct {
	db *sql.DB
}

type crossCachedVector struct {
	Vector      []float32
	ContentHash string
}

func (c crossVectorCache) load(ctx context.Context, d embedding.Descriptor) (map[string]crossCachedVector, error) {
	rows, err := c.db.QueryContext(ctx,
		`SELECT project_id, decision_id, content_hash, vector FROM global_embeddings WHERE provider=? AND model=? AND dim=?`,
		d.Provider, d.Model, d.Dimensions)
	if err != nil {
		return nil, fmt.Errorf("load cross embeddings: %w", err)
	}
	defer rows.Close()

	out := make(map[string]crossCachedVector)
	for rows.Next() {
		var projectID, decisionID, hash string
		var blob []byte
		if err := rows.Scan(&projectID, &decisionID, &hash, &blob); err != nil {
			return nil, fmt.Errorf("scan cross embedding: %w", err)
		}
		vec := embedding.DecodeVector(blob)
		if vec == nil {
			continue
		}
		out[projectID+"|"+decisionID] = crossCachedVector{Vector: vec, ContentHash: hash}
	}
	return out, rows.Err()
}

func (c crossVectorCache) store(ctx context.Context, d embedding.Descriptor, projectID, decisionID, hash string, vec []float32) error {
	_, err := c.db.ExecContext(ctx,
		`INSERT INTO global_embeddings (project_id, decision_id, provider, model, dim, content_hash, vector, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(project_id, decision_id, provider, model, dim)
		 DO UPDATE SET content_hash=excluded.content_hash, vector=excluded.vector, updated_at=excluded.updated_at`,
		projectID, decisionID, d.Provider, d.Model, d.Dimensions, hash, embedding.EncodeVector(vec))
	if err != nil {
		return fmt.Errorf("store cross embedding %s/%s: %w", projectID, decisionID, err)
	}
	return nil
}

// crossCorpusRow is an indexed decision plus its language (CL is assigned at
// query time against the caller's currentLang, like IndexStore.Search).
type crossCorpusRow struct {
	recall    IndexRecall
	projectID string
	lang      string
}

// CrossHybrid fuses FTS and semantic ranking over the cross-project index.
type CrossHybrid struct {
	store       *IndexStore
	newEmbedder func() (embedding.Embedder, error)
	cache       crossVectorCache

	mu                     sync.Mutex
	embedder               embedding.Embedder
	embedderUnavailable    bool
	index                  *embedding.Index
	byID                   map[string]crossCorpusRow
	built, building, dirty bool
}

// NewCrossHybrid builds the cross-project hybrid. newEmbedder is resolved lazily
// in the background; a nil factory (or an absent sidecar) makes Search delegate
// to IndexStore.Search for the rest of the session.
func NewCrossHybrid(store *IndexStore, newEmbedder func() (embedding.Embedder, error)) *CrossHybrid {
	return &CrossHybrid{store: store, newEmbedder: newEmbedder, cache: crossVectorCache{db: store.db}}
}

// Prewarm kicks the background corpus build so the first search is fast.
func (h *CrossHybrid) Prewarm() {
	if h.newEmbedder != nil {
		h.ensureWarming()
	}
}

// Invalidate marks the corpus stale so the next search rebuilds in the
// background (cached vectors reused, only the changed/added decision re-embeds).
func (h *CrossHybrid) Invalidate() {
	h.mu.Lock()
	h.dirty = true
	h.mu.Unlock()
	h.ensureWarming()
}

// Warm builds the index synchronously, blocking until ready. Production search
// uses the non-blocking background path; this is for callers that must have the
// semantic index available before searching (tests, explicit pre-warm).
func (h *CrossHybrid) Warm(ctx context.Context) error {
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

// Search returns cross-project decisions for the query, fusing keyword and
// semantic ranking. It delegates byte-for-byte to IndexStore.Search on any
// semantic-path issue (no embedder, cold/rewarming index, embed fault).
func (h *CrossHybrid) Search(ctx context.Context, query, currentProjectID, currentLang string, limit int) ([]IndexRecall, error) {
	if limit <= 0 {
		limit = 5
	}
	if h.newEmbedder == nil {
		return h.store.Search(ctx, query, currentProjectID, currentLang, limit)
	}

	h.ensureWarming()
	embedder, index, byID, ready := h.snapshot()
	if !ready {
		return h.store.Search(ctx, query, currentProjectID, currentLang, limit)
	}

	pool := max(limit*crossCandidateFactor, crossMinCandidatePool)
	// Fuse PRECISE AND-only FTS with semantic: the embedding arm already carries
	// paraphrase recall, so feeding the lower-precision OR-fallback matches into
	// the RRF would only dilute the top ranks with shared-one-term noise. The
	// degraded path above still returns the full AND+OR Search (recall floor).
	var ftsResults []IndexRecall
	if sanitized := sanitizeFTS(query); sanitized != "" {
		var err error
		ftsResults, err = h.store.runFTS(ctx, sanitized, currentProjectID, currentLang, pool)
		if err != nil {
			return nil, err
		}
	}

	queryVectors, err := embedder.Embed(ctx, embedding.RoleQuery, []string{query})
	if err != nil || len(queryVectors) == 0 {
		// Query-embed fault: degrade to the FULL AND+OR recall floor — byte-
		// identical to IndexStore.Search, never the AND-only pool (which would
		// silently drop OR-recoverable recall on every faulting search).
		if err != nil {
			logger.Warn().Err(err).Msg("cross-project query embed failed — delegating to IndexStore.Search")
		}
		return h.store.Search(ctx, query, currentProjectID, currentLang, limit)
	}
	semantic := index.Search(queryVectors[0], pool)

	fused := fuseRRF([][]string{recallIDs(ftsResults), semanticIDs(semantic, crossMinSimilarity)}, crossRRFK)
	result := resolveRecall(fused, byID, ftsResults, currentProjectID, currentLang, limit)

	// Recall floor: embeddings AUGMENT, never reduce recall. The clean AND+semantic
	// fusion gives the high-quality top ranks; if it under-fills, top up with the
	// AND+OR FTS rows appended at the TAIL — they never dilute the fused top, and
	// hybrid recall is guaranteed >= IndexStore.Search.
	if len(result) < limit {
		if floor, ferr := h.store.Search(ctx, query, currentProjectID, currentLang, limit); ferr == nil {
			result = appendUniqueRecall(result, floor, limit)
		}
	}
	return result, nil
}

func (h *CrossHybrid) snapshot() (embedding.Embedder, *embedding.Index, map[string]crossCorpusRow, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.index == nil || h.index.Len() == 0 {
		return nil, nil, nil, false
	}
	return h.embedder, h.index, h.byID, true
}

func (h *CrossHybrid) ensureWarming() {
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

func (h *CrossHybrid) warmAsync() {
	index, byID, err := h.buildIndex(context.Background())
	h.mu.Lock()
	h.building = false
	if err != nil {
		h.mu.Unlock()
		logger.Warn().Err(err).Msg("cross-project recall warm failed — search uses FTS until the next attempt")
		return
	}
	if index != nil {
		h.index = index
		h.byID = byID
	}
	h.built = true
	// An Invalidate that arrived during this build set dirty without scheduling a
	// rebuild (ensureWarming saw building=true). Re-check and self-reschedule so
	// the new decision is indexed promptly, not deferred to the next Search.
	needRewarm := h.dirty
	h.mu.Unlock()
	if needRewarm {
		h.ensureWarming()
	}
}

func (h *CrossHybrid) resolveEmbedder() embedding.Embedder {
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
		logger.Info().Err(err).Msg("embedding sidecar unavailable — cross-project recall degrades to FTS")
		return nil
	}
	if embedder == nil {
		h.embedderUnavailable = true
		return nil
	}
	h.embedder = embedder
	return embedder
}

func (h *CrossHybrid) buildIndex(ctx context.Context) (*embedding.Index, map[string]crossCorpusRow, error) {
	embedder := h.resolveEmbedder()
	if embedder == nil {
		return nil, nil, nil
	}

	descriptor := embedder.Descriptor()
	cached, err := h.cache.load(ctx, descriptor)
	if err != nil {
		return nil, nil, err
	}
	corpus, err := h.store.loadCorpus(ctx)
	if err != nil {
		return nil, nil, err
	}

	byID := make(map[string]crossCorpusRow, len(corpus))
	vectors := make(map[string][]float32, len(corpus))
	var misses []IndexEntry
	var missHashes []string

	for _, entry := range corpus {
		fuseKey := entry.ProjectName + "|" + entry.DecisionID
		byID[fuseKey] = crossCorpusRow{
			recall: IndexRecall{
				ProjectName: entry.ProjectName,
				DecisionID:  entry.DecisionID,
				Title:       entry.Title,
				WhySelected: entry.WhySelected,
				WeakestLink: entry.WeakestLink,
			},
			projectID: entry.ProjectID,
			lang:      entry.PrimaryLang,
		}
		text := corpusText(entry)
		if text == "" {
			continue
		}
		hash := contentHash(text)
		if hit, ok := cached[entry.ProjectID+"|"+entry.DecisionID]; ok && hit.ContentHash == hash {
			vectors[fuseKey] = hit.Vector
			continue
		}
		misses = append(misses, entry)
		missHashes = append(missHashes, hash)
	}

	if err := h.embedMisses(ctx, embedder, descriptor, misses, missHashes, vectors); err != nil {
		return nil, nil, err
	}

	index := embedding.NewIndex(len(vectors))
	for id, vec := range vectors {
		index.Add(id, vec)
	}
	return index, byID, nil
}

func (h *CrossHybrid) embedMisses(ctx context.Context, embedder embedding.Embedder, d embedding.Descriptor, misses []IndexEntry, hashes []string, vectors map[string][]float32) error {
	if len(misses) == 0 {
		return nil
	}
	for start := 0; start < len(misses); start += crossEmbeddingBatch {
		end := min(start+crossEmbeddingBatch, len(misses))
		err := h.embedMissBatch(
			ctx,
			embedder,
			d,
			misses[start:end],
			hashes[start:end],
			vectors,
		)
		if err != nil {
			return fmt.Errorf("embed cross-project misses [%d:%d]: %w", start, end, err)
		}
	}
	return nil
}

func (h *CrossHybrid) embedMissBatch(ctx context.Context, embedder embedding.Embedder, d embedding.Descriptor, misses []IndexEntry, hashes []string, vectors map[string][]float32) error {
	texts := make([]string, len(misses))
	for i, entry := range misses {
		texts[i] = corpusText(entry)
	}
	embedded, err := embedder.Embed(ctx, embedding.RoleDocument, texts)
	if err != nil {
		return err
	}
	if len(embedded) != len(misses) {
		return fmt.Errorf("embedded %d vectors for %d corpus misses", len(embedded), len(misses))
	}
	for i, entry := range misses {
		vectors[entry.ProjectName+"|"+entry.DecisionID] = embedded[i]
		if err := h.cache.store(ctx, d, entry.ProjectID, entry.DecisionID, hashes[i], embedded[i]); err != nil {
			return err
		}
	}
	return nil
}

// resolveRecall maps the fused id ordering back to IndexRecall rows. FTS rows
// carry their query-time CL/Similarity; semantic-only rows get CL assigned now.
func resolveRecall(orderedIDs []string, byID map[string]crossCorpusRow, ftsResults []IndexRecall, currentProjectID string, currentLang string, limit int) []IndexRecall {
	ftsByID := make(map[string]IndexRecall, len(ftsResults))
	for _, r := range ftsResults {
		ftsByID[r.ProjectName+"|"+r.DecisionID] = r
	}

	out := make([]IndexRecall, 0, limit)
	for _, id := range orderedIDs {
		if r, ok := ftsByID[id]; ok {
			out = append(out, r)
		} else if row, ok := byID[id]; ok {
			if currentProjectID != "" && row.projectID == currentProjectID {
				continue
			}
			r := row.recall
			r.CL = clFor(currentLang, row.lang)
			out = append(out, r)
		} else {
			continue
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

func clFor(currentLang, lang string) int {
	if currentLang != "" && lang != "" && currentLang == lang {
		return 2
	}
	return 1
}

// corpusText is the SINGLE source of the text used for BOTH the content_hash and
// the document embedding — it must mirror exactly what global_fts indexes
// (title + selected_title + why_selected + weakest_link), so an unchanged
// decision is a cache hit and a re-decided one re-embeds.
func corpusText(entry IndexEntry) string {
	return strings.TrimSpace(entry.Title + " " + entry.SelectedTitle + " " + entry.WhySelected + " " + entry.WeakestLink)
}

func contentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// fuseRRF merges ranked id-lists by Reciprocal Rank Fusion. Re-implemented
// locally (not imported from recall) to keep package project free of a recall
// edge — the same ~12 lines the cross-project eval already carries.
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

func recallIDs(items []IndexRecall) []string {
	ids := make([]string, len(items))
	for i, r := range items {
		ids[i] = r.ProjectName + "|" + r.DecisionID
	}
	return ids
}
