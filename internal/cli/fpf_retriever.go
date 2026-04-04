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
		})
	}

	return formattedResults
}
