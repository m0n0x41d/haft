package cli

import "github.com/m0n0x41d/haft/internal/present"

func formatCLIFPFSearch(results []present.FPFSearchResult) string {
	options := sharedFPFSearchOptions()
	return present.FormatFPFSearch(results, options)
}

func formatMCPFPFSearch(results []present.FPFSearchResult) string {
	options := sharedFPFSearchOptions()
	return present.FormatFPFSearch(results, options)
}

func formatAgentFPFSearch(query string, results []present.FPFSearchResult) string {
	_ = query
	return formatCLIFPFSearch(results)
}

func sharedFPFSearchOptions() present.FPFSearchOptions {
	return present.FPFSearchOptions{
		Enumerate:    true,
		EmptyMessage: "No results found.",
	}
}
