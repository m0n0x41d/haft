package cli

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/present"
)

func retrieveEmbeddedFPF(request fpf.SpecRetrievalRequest) (fpf.SpecRetrievalResult, error) {
	db, cleanup, err := openFPFDBFunc()
	if err != nil {
		return fpf.SpecRetrievalResult{}, fmt.Errorf("open fpf db: %w", err)
	}
	defer cleanup()

	// Hybrid-by-default: the process-lived FPF searcher fuses FTS+graph with baked
	// section vectors, degrading to deterministic FTS when unavailable.
	if hybrid := ensureFPFHybrid(); hybrid != nil {
		request.HybridSearch = hybrid.Search
	}

	return fpf.RetrieveSpec(db, request)
}

func presentFPFRetrieval(results []fpf.SpecRetrievedSection) []present.FPFSearchResult {
	formattedResults := make([]present.FPFSearchResult, 0, len(results))
	for _, result := range results {
		formattedResults = append(formattedResults, present.FPFSearchResult{
			PatternID: result.PatternID,
			Heading:   result.Heading,
			Tier:      result.Tier,
			Reason:    result.Reason,
			Summary:   result.Summary,
			Content:   result.Content,
			Provenance: present.FPFSearchProvenance{
				ProfileID:          result.Provenance.ProfileID,
				SourceKind:         result.Provenance.SourceKind,
				SourceEdition:      result.Provenance.SourceEdition,
				SourceRef:          result.Provenance.SourceRef,
				SourceHash:         result.Provenance.SourceHash,
				ProfileValidity:    result.Provenance.ProfileValidity,
				Normativity:        result.Provenance.Normativity,
				IndexSchemaVersion: result.Provenance.IndexSchemaVersion,
				RetrievalMode:      result.Provenance.RetrievalMode,
			},
		})
	}

	return formattedResults
}
