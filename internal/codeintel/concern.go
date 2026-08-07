package codeintel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const (
	ConcernCandidates             = "candidate_set"
	ConcernResolvedExactIdentity  = "resolved_exact_identity"
	ConcernNoCandidates           = "no_candidates"
	ConcernNoLexicalCandidates    = ConcernNoCandidates
	ConcernIndexUnavailable       = "index_unavailable"
	ConcernIncompleteBasis        = "incomplete_index_basis"
	DefaultConcernCandidateBudget = 12
)

type ConcernDiscoveryOutcome struct {
	code   string
	detail string
}

func (o ConcernDiscoveryOutcome) String() string {
	return o.code
}

func (o ConcernDiscoveryOutcome) DetailCode() string {
	return o.detail
}

// ConcernDiscoveryResult is a read-only candidate projection. Candidate order
// is advisory lexical precedence; no field can represent a selected identity.
type ConcernDiscoveryResult struct {
	Query        codebase.ConcernQuery
	Batch        codebase.SymbolDiscoveryBatch
	Fused        ConcernCandidateBatch
	Basis        ConcernFusionBasis
	Outcome      ConcernDiscoveryOutcome
	ColdBuilt    bool
	IndexRefresh IndexCoordinationResult
	Index        codebase.IndexState
}

func (r ConcernDiscoveryResult) Candidates() []ConcernCandidate {
	return r.Fused.Candidates()
}

func (r ConcernDiscoveryResult) LexicalCandidates() []codebase.SymbolDiscoveryCandidate {
	return r.Batch.Candidates()
}

func (r ConcernDiscoveryResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Query        codebase.ConcernQuery   `json:"query"`
		Outcome      string                  `json:"outcome"`
		Detail       string                  `json:"detail,omitempty"`
		Candidates   []ConcernCandidate      `json:"candidates"`
		Basis        ConcernFusionBasis      `json:"basis"`
		ColdBuilt    bool                    `json:"cold_built"`
		IndexRefresh IndexCoordinationResult `json:"index_refresh"`
		Index        codebase.IndexState     `json:"index"`
	}{
		Query:        r.Query,
		Outcome:      r.Outcome.String(),
		Detail:       r.Outcome.DetailCode(),
		Candidates:   r.Candidates(),
		Basis:        r.Basis,
		ColdBuilt:    r.ColdBuilt,
		IndexRefresh: r.IndexRefresh,
		Index:        r.Index,
	})
}

// DiscoverConcern parses the weak public string once and retrieves a bounded
// candidate set under one exact, freshness-confirmed published index basis.
func (s *Service) DiscoverConcern(
	ctx context.Context,
	projectRoot string,
	rawQuery string,
	maxCandidates int,
) (ConcernDiscoveryResult, error) {
	query, err := codebase.NewConcernQuery(rawQuery)
	if err != nil {
		return ConcernDiscoveryResult{}, err
	}
	budget, err := codebase.NewDiscoveryBudget(maxCandidates)
	if err != nil {
		return ConcernDiscoveryResult{}, err
	}
	return retryIndexQuery(func() (ConcernDiscoveryResult, error) {
		return s.discoverConcernOnce(
			ctx,
			projectRoot,
			query,
			budget,
		)
	})
}

func (s *Service) discoverConcernOnce(
	ctx context.Context,
	projectRoot string,
	query codebase.ConcernQuery,
	budget codebase.DiscoveryBudget,
) (result ConcernDiscoveryResult, resultErr error) {
	indexRefresh, err := s.EnsureIndex(ctx, projectRoot)
	if err != nil {
		return ConcernDiscoveryResult{}, err
	}
	releaseIndexRead, err := s.acquireIndexRead(projectRoot)
	if err != nil {
		return ConcernDiscoveryResult{}, err
	}
	defer releaseIndexRead()
	publishedIndexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return ConcernDiscoveryResult{}, err
	}
	indexState := indexRefresh.EffectiveIndexState(publishedIndexState)
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, publishedIndexState); err != nil {
			result = ConcernDiscoveryResult{}
			resultErr = err
			return
		}
		if result.Basis.GraphDigest == "" {
			return
		}
		if err := s.ConfirmConcernFusionBasis(
			ctx,
			query,
			indexState,
			result.Basis,
		); err != nil {
			result = ConcernDiscoveryResult{}
			resultErr = err
		}
	}()
	result = ConcernDiscoveryResult{
		Query:        query,
		ColdBuilt:    indexRefresh.Rebuilt(),
		IndexRefresh: indexRefresh,
		Index:        indexState,
	}
	if indexState.Epoch == 0 {
		result.Outcome = ConcernDiscoveryOutcome{
			code:   ConcernIndexUnavailable,
			detail: "no_published_epoch",
		}
		return result, nil
	}
	batch, err := s.symbols.DiscoverSymbols(
		ctx,
		query,
		budget,
		indexState.Epoch,
	)
	if err != nil {
		return ConcernDiscoveryResult{}, fmt.Errorf(
			"discover concern symbols: %w",
			err,
		)
	}
	result.Batch = batch
	fused, basis, err := s.fuseConcernCandidates(
		ctx,
		query,
		batch,
		budget,
		indexState,
	)
	if err != nil {
		return ConcernDiscoveryResult{}, fmt.Errorf(
			"fuse concern reasoning: %w",
			err,
		)
	}
	result.Fused = fused
	result.Basis = basis
	switch {
	case concernHasExactIdentity(fused.Candidates()):
		result.Outcome = ConcernDiscoveryOutcome{
			code: ConcernResolvedExactIdentity,
		}
	case len(fused.Candidates()) > 0:
		result.Outcome = ConcernDiscoveryOutcome{
			code: ConcernCandidates,
		}
	case indexState.SupportsKnownAbsence():
		result.Outcome = ConcernDiscoveryOutcome{
			code: ConcernNoLexicalCandidates,
		}
	default:
		result.Outcome = ConcernDiscoveryOutcome{
			code:   ConcernIncompleteBasis,
			detail: indexState.Basis.Coverage.Posture,
		}
	}
	return result, nil
}
