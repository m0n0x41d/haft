package cli

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/embedding"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/logger"
)

var errPatternUseSemanticUnavailable = errors.New("pattern-use semantic route index unavailable (no sidecar or no matching baked vectors)")

var (
	patternUseHybrid          *PatternUseHybrid
	patternUseHybridMu        sync.Mutex
	buildPatternUseHybridFunc = buildPatternUseHybrid
)

type PatternUseHybrid struct {
	newEmbedder func() (embedding.Embedder, error)

	mu                  sync.Mutex
	embedder            embedding.Embedder
	embedderUnavailable bool
	positiveIndex       *embedding.Index
	negativeIndex       *embedding.Index
	documents           map[string]fpf.PatternUseRouteEmbeddingRow
	intentPositiveIndex *embedding.Index
	intentNegativeIndex *embedding.Index
	intentDocuments     map[string]fpf.PatternUseIntentEmbeddingRow
	built               bool
	building            bool
}

type patternUseRouteAggregate struct {
	routeID     string
	score       float64
	documentIDs []string
}

type patternUseIntentAggregate struct {
	laneID      fpf.PatternUseIntentLane
	score       float64
	documentIDs []string
}

func ensurePatternUseHybrid() *PatternUseHybrid {
	patternUseHybridMu.Lock()
	defer patternUseHybridMu.Unlock()

	if patternUseHybrid == nil {
		patternUseHybrid = buildPatternUseHybridFunc()
	}
	return patternUseHybrid
}

func buildPatternUseHybrid() *PatternUseHybrid {
	embCfg, ok := fpfEmbeddingConfigFrom(embeddingConfigFromFile())
	if !ok {
		return nil
	}
	newEmbedder := func() (embedding.Embedder, error) {
		return embedding.New(embCfg)
	}
	return NewPatternUseHybrid(newEmbedder)
}

func NewPatternUseHybrid(newEmbedder func() (embedding.Embedder, error)) *PatternUseHybrid {
	return &PatternUseHybrid{newEmbedder: newEmbedder}
}

func (h *PatternUseHybrid) Match(query string) (fpf.PatternUseRouteMatch, bool) {
	query = strings.TrimSpace(query)
	if query == "" || h == nil || h.newEmbedder == nil {
		return fpf.PatternUseRouteMatch{}, false
	}

	h.ensureBuiltForSearch()
	embedder, positiveIndex, negativeIndex, documents, ready := h.snapshot()
	if !ready {
		return fpf.PatternUseRouteMatch{}, false
	}

	queryVectors, err := embedder.Embed(context.Background(), embedding.RoleQuery, []string{query})
	if err != nil || len(queryVectors) == 0 {
		if err != nil {
			logger.Warn().Err(err).Msg("pattern-use query embed failed — semantic route matching disabled for this query")
		}
		return fpf.PatternUseRouteMatch{}, false
	}

	positiveScores := positiveIndex.Search(queryVectors[0], 0)
	aggregates := aggregatePatternUseRouteScores(positiveScores, documents)
	if len(aggregates) == 0 {
		return fpf.PatternUseRouteMatch{}, false
	}

	top := aggregates[0]
	secondScore := 0.0
	if len(aggregates) > 1 {
		secondScore = aggregates[1].score
	}
	margin := top.score - secondScore
	if top.score < fpf.PatternUseSemanticMinSimilarity {
		return fpf.PatternUseRouteMatch{}, false
	}
	if margin < fpf.PatternUseSemanticMinMargin {
		return fpf.PatternUseRouteMatch{}, false
	}

	negativeScore := maxPatternUseNegativeScore(negativeIndex, queryVectors[0], documents, top.routeID)
	if negativeScore+fpf.PatternUseSemanticNegativeSlack >= top.score {
		return fpf.PatternUseRouteMatch{}, false
	}

	descriptor := embedder.Descriptor()
	contract := strings.Join([]string{descriptor.Provider, descriptor.Model, intString(descriptor.Dimensions)}, "/")
	return fpf.PatternUseRouteMatch{
		RouteID:            top.routeID,
		Strategy:           fpf.PatternUseRouteMatchStrategySemanticCompiledRoute,
		Score:              top.score,
		Margin:             margin,
		Contract:           contract,
		MatchedDocumentIDs: top.documentIDs,
	}, true
}

func (h *PatternUseHybrid) MatchIntent(query string) (fpf.PatternUseIntentLaneMatch, bool) {
	query = strings.TrimSpace(query)
	if query == "" || h == nil || h.newEmbedder == nil {
		return fpf.PatternUseIntentLaneMatch{}, false
	}

	h.ensureBuiltForSearch()
	embedder, positiveIndex, negativeIndex, documents, ready := h.intentSnapshot()
	if !ready {
		return fpf.PatternUseIntentLaneMatch{}, false
	}

	queryVectors, err := embedder.Embed(context.Background(), embedding.RoleQuery, []string{query})
	if err != nil || len(queryVectors) == 0 {
		if err != nil {
			logger.Warn().Err(err).Msg("pattern-use query embed failed — semantic intent matching disabled for this query")
		}
		return fpf.PatternUseIntentLaneMatch{}, false
	}

	positiveScores := positiveIndex.Search(queryVectors[0], 0)
	aggregates := aggregatePatternUseIntentScores(positiveScores, documents)
	if len(aggregates) == 0 {
		return fpf.PatternUseIntentLaneMatch{}, false
	}

	top := aggregates[0]
	secondScore := 0.0
	if len(aggregates) > 1 {
		secondScore = aggregates[1].score
	}
	margin := top.score - secondScore
	if top.score < fpf.PatternUseSemanticMinSimilarity {
		return fpf.PatternUseIntentLaneMatch{}, false
	}
	if margin < fpf.PatternUseSemanticMinMargin {
		return fpf.PatternUseIntentLaneMatch{}, false
	}

	negativeScore := maxPatternUseIntentNegativeScore(negativeIndex, queryVectors[0], documents, top.laneID)
	if negativeScore+fpf.PatternUseSemanticNegativeSlack >= top.score {
		return fpf.PatternUseIntentLaneMatch{}, false
	}

	descriptor := embedder.Descriptor()
	contract := strings.Join([]string{descriptor.Provider, descriptor.Model, intString(descriptor.Dimensions)}, "/")
	return fpf.PatternUseIntentLaneMatch{
		Lane:               top.laneID,
		Strategy:           fpf.PatternUseIntentMatchStrategySemanticLane,
		Score:              top.score,
		Margin:             margin,
		Contract:           contract,
		MatchedDocumentIDs: top.documentIDs,
	}, true
}

func (h *PatternUseHybrid) ensureBuiltForSearch() {
	h.mu.Lock()
	if h.embedderUnavailable || h.newEmbedder == nil || h.built || h.building {
		h.mu.Unlock()
		return
	}
	h.building = true
	h.mu.Unlock()

	positiveIndex, negativeIndex, documents, intentPositiveIndex, intentNegativeIndex, intentDocuments := h.buildIndex()
	h.mu.Lock()
	h.building = false
	h.positiveIndex = positiveIndex
	h.negativeIndex = negativeIndex
	h.documents = documents
	h.intentPositiveIndex = intentPositiveIndex
	h.intentNegativeIndex = intentNegativeIndex
	h.intentDocuments = intentDocuments
	h.built = true
	h.mu.Unlock()
}

func (h *PatternUseHybrid) snapshot() (
	embedding.Embedder,
	*embedding.Index,
	*embedding.Index,
	map[string]fpf.PatternUseRouteEmbeddingRow,
	bool,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if indexLen(h.positiveIndex) == 0 {
		return nil, nil, nil, nil, false
	}
	if h.documents == nil {
		return nil, nil, nil, nil, false
	}
	return h.embedder, h.positiveIndex, h.negativeIndex, h.documents, true
}

func (h *PatternUseHybrid) intentSnapshot() (
	embedding.Embedder,
	*embedding.Index,
	*embedding.Index,
	map[string]fpf.PatternUseIntentEmbeddingRow,
	bool,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if indexLen(h.intentPositiveIndex) == 0 {
		return nil, nil, nil, nil, false
	}
	if h.intentDocuments == nil {
		return nil, nil, nil, nil, false
	}
	return h.embedder, h.intentPositiveIndex, h.intentNegativeIndex, h.intentDocuments, true
}

func (h *PatternUseHybrid) resolveEmbedder() embedding.Embedder {
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
	if h.embedder != nil {
		cached := h.embedder
		h.mu.Unlock()
		closeDiscardedEmbedder(embedder)
		return cached
	}
	defer h.mu.Unlock()
	if err != nil {
		h.embedderUnavailable = true
		closeDiscardedEmbedder(embedder)
		logger.Info().Err(err).Msg("embedding sidecar unavailable — PatternUse semantic routing disabled")
		return nil
	}
	if embedder == nil {
		h.embedderUnavailable = true
		return nil
	}
	h.embedder = embedder
	return embedder
}

func (h *PatternUseHybrid) buildIndex() (
	*embedding.Index,
	*embedding.Index,
	map[string]fpf.PatternUseRouteEmbeddingRow,
	*embedding.Index,
	*embedding.Index,
	map[string]fpf.PatternUseIntentEmbeddingRow,
) {
	embedder := h.resolveEmbedder()
	if embedder == nil {
		return nil, nil, nil, nil, nil, nil
	}
	descriptor := embedder.Descriptor()

	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		logger.Warn().Err(err).Msg("fpf index open failed — PatternUse semantic routing disabled")
		return nil, nil, nil, nil, nil, nil
	}
	defer cleanup()

	if version, _ := fpf.GetSpecMeta(db, "schema_version"); version != fpf.SpecIndexSchemaVersion {
		return nil, nil, nil, nil, nil, nil
	}
	rows, err := fpf.LoadPatternUseRouteEmbeddings(db, descriptor.Provider, descriptor.Model, descriptor.Dimensions)
	if err != nil {
		logger.Warn().Err(err).Msg("pattern-use route embeddings load failed — semantic routing disabled")
		return nil, nil, nil, nil, nil, nil
	}
	if len(rows) == 0 {
		if provider, model, dim, count, _ := fpf.PatternUseRouteEmbeddingContract(db); count > 0 {
			logger.Warn().
				Str("baked", provider+"/"+model).Int("baked_dim", dim).
				Str("runtime", descriptor.Provider+"/"+descriptor.Model).Int("runtime_dim", descriptor.Dimensions).
				Msg("PatternUse baked route vectors are under a different model contract — semantic routing disabled")
		}
		return nil, nil, nil, nil, nil, nil
	}
	intentRows, err := fpf.LoadPatternUseIntentEmbeddings(db, descriptor.Provider, descriptor.Model, descriptor.Dimensions)
	if err != nil {
		logger.Warn().Err(err).Msg("pattern-use intent embeddings load failed — semantic routing disabled")
		return nil, nil, nil, nil, nil, nil
	}
	if len(intentRows) == 0 {
		if provider, model, dim, count, _ := fpf.PatternUseIntentEmbeddingContract(db); count > 0 {
			logger.Warn().
				Str("baked", provider+"/"+model).Int("baked_dim", dim).
				Str("runtime", descriptor.Provider+"/"+descriptor.Model).Int("runtime_dim", descriptor.Dimensions).
				Msg("PatternUse baked intent vectors are under a different model contract — semantic routing disabled")
		}
		return nil, nil, nil, nil, nil, nil
	}

	positiveIndex := embedding.NewIndex(len(rows))
	negativeIndex := embedding.NewIndex(len(rows))
	documents := make(map[string]fpf.PatternUseRouteEmbeddingRow, len(rows))
	for _, row := range rows {
		documents[row.DocumentID] = row
		if row.DocumentKind == fpf.PatternUseRouteDocumentKindNegativeExample {
			negativeIndex.Add(row.DocumentID, row.Vector)
			continue
		}
		positiveIndex.Add(row.DocumentID, row.Vector)
	}

	intentPositiveIndex := embedding.NewIndex(len(intentRows))
	intentNegativeIndex := embedding.NewIndex(len(intentRows))
	intentDocuments := make(map[string]fpf.PatternUseIntentEmbeddingRow, len(intentRows))
	for _, row := range intentRows {
		intentDocuments[row.DocumentID] = row
		if row.DocumentKind == fpf.PatternUseRouteDocumentKindNegativeExample {
			intentNegativeIndex.Add(row.DocumentID, row.Vector)
			continue
		}
		intentPositiveIndex.Add(row.DocumentID, row.Vector)
	}
	return positiveIndex, negativeIndex, documents, intentPositiveIndex, intentNegativeIndex, intentDocuments
}

func aggregatePatternUseRouteScores(
	scored []embedding.Scored,
	documents map[string]fpf.PatternUseRouteEmbeddingRow,
) []patternUseRouteAggregate {
	byRoute := map[string]patternUseRouteAggregate{}
	for _, item := range scored {
		if item.Score <= 0 {
			continue
		}
		document, ok := documents[item.ID]
		if !ok {
			continue
		}
		if document.DocumentKind == fpf.PatternUseRouteDocumentKindNegativeExample {
			continue
		}
		current := byRoute[document.RouteID]
		if item.Score > current.score {
			current.routeID = document.RouteID
			current.score = item.Score
			current.documentIDs = []string{document.DocumentID}
			byRoute[document.RouteID] = current
			continue
		}
		if item.Score == current.score {
			current.documentIDs = append(current.documentIDs, document.DocumentID)
			byRoute[document.RouteID] = current
		}
	}

	out := make([]patternUseRouteAggregate, 0, len(byRoute))
	for _, aggregate := range byRoute {
		aggregate.documentIDs = topNStrings(aggregate.documentIDs, fpf.PatternUseSemanticTopK)
		out = append(out, aggregate)
	}
	sort.SliceStable(out, func(left, right int) bool {
		if out[left].score == out[right].score {
			return out[left].routeID < out[right].routeID
		}
		return out[left].score > out[right].score
	})
	return out
}

func aggregatePatternUseIntentScores(
	scored []embedding.Scored,
	documents map[string]fpf.PatternUseIntentEmbeddingRow,
) []patternUseIntentAggregate {
	byLane := map[fpf.PatternUseIntentLane]patternUseIntentAggregate{}
	for _, item := range scored {
		if item.Score <= 0 {
			continue
		}
		document, ok := documents[item.ID]
		if !ok {
			continue
		}
		if document.DocumentKind == fpf.PatternUseRouteDocumentKindNegativeExample {
			continue
		}
		current := byLane[document.LaneID]
		if item.Score > current.score {
			current.laneID = document.LaneID
			current.score = item.Score
			current.documentIDs = []string{document.DocumentID}
			byLane[document.LaneID] = current
			continue
		}
		if item.Score == current.score {
			current.documentIDs = append(current.documentIDs, document.DocumentID)
			byLane[document.LaneID] = current
		}
	}

	out := make([]patternUseIntentAggregate, 0, len(byLane))
	for _, aggregate := range byLane {
		aggregate.documentIDs = topNStrings(aggregate.documentIDs, fpf.PatternUseSemanticTopK)
		out = append(out, aggregate)
	}
	sort.SliceStable(out, func(left, right int) bool {
		if out[left].score == out[right].score {
			return out[left].laneID < out[right].laneID
		}
		return out[left].score > out[right].score
	})
	return out
}

func maxPatternUseNegativeScore(
	index *embedding.Index,
	query []float32,
	documents map[string]fpf.PatternUseRouteEmbeddingRow,
	routeID string,
) float64 {
	if indexLen(index) == 0 {
		return 0
	}
	maxScore := 0.0
	for _, item := range index.Search(query, 0) {
		document, ok := documents[item.ID]
		if !ok {
			continue
		}
		if document.RouteID != routeID {
			continue
		}
		if item.Score > maxScore {
			maxScore = item.Score
		}
	}
	return maxScore
}

func maxPatternUseIntentNegativeScore(
	index *embedding.Index,
	query []float32,
	documents map[string]fpf.PatternUseIntentEmbeddingRow,
	laneID fpf.PatternUseIntentLane,
) float64 {
	if indexLen(index) == 0 {
		return 0
	}
	maxScore := 0.0
	for _, item := range index.Search(query, 0) {
		document, ok := documents[item.ID]
		if !ok {
			continue
		}
		if document.LaneID != laneID {
			continue
		}
		if item.Score > maxScore {
			maxScore = item.Score
		}
	}
	return maxScore
}

func intString(value int) string {
	return strings.TrimSpace(strconv.Itoa(value))
}
