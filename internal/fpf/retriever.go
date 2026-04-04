package fpf

import (
	"database/sql"
	"strings"
)

// SpecRetrievalRequest captures deterministic spec retrieval controls for
// higher-level agent, CLI, and MCP surfaces.
type SpecRetrievalRequest struct {
	Query string
	Limit int
	Tier  string
	Full  bool
}

// SpecRetrievalResult is the structured retrieval response returned to shell
// layers before any surface-specific formatting is applied.
type SpecRetrievalResult struct {
	Query   string
	Results []SpecRetrievedSection
}

// SpecRetrievedSection is a presentation-ready FPF section hit with either
// snippet-sized content or the full section body.
type SpecRetrievedSection struct {
	PatternID string
	Heading   string
	Tier      string
	Reason    string
	Summary   string
	Content   string
}

// RetrieveSpec resolves deterministic FPF search hits and hydrates content for
// downstream CLI, MCP, and agent surfaces.
func RetrieveSpec(db *sql.DB, request SpecRetrievalRequest) (SpecRetrievalResult, error) {
	query := strings.TrimSpace(request.Query)
	searchResults, err := SearchSpecWithOptions(db, query, SpecSearchOptions{
		Limit: request.Limit,
		Tier:  request.Tier,
	})
	if err != nil {
		return SpecRetrievalResult{}, err
	}

	results := make([]SpecRetrievedSection, 0, len(searchResults))
	for _, searchResult := range searchResults {
		results = append(results, hydrateRetrievedSection(db, searchResult, request.Full))
	}

	return SpecRetrievalResult{
		Query:   query,
		Results: results,
	}, nil
}

func hydrateRetrievedSection(db *sql.DB, searchResult SpecSearchResult, full bool) SpecRetrievedSection {
	content := searchResult.Snippet
	if full {
		body, err := GetSpecSection(db, firstNonEmpty(searchResult.PatternID, searchResult.Heading))
		if err == nil {
			content = body
		}
	}

	return SpecRetrievedSection{
		PatternID: searchResult.PatternID,
		Heading:   searchResult.Heading,
		Tier:      searchResult.Tier,
		Reason:    searchResult.Reason,
		Summary:   searchResult.Summary,
		Content:   content,
	}
}
