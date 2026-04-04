package cli

import "github.com/m0n0x41d/haft/internal/present"

func formatCLIFPFSearch(results []present.FPFSearchResult) string {
	return formatCLIFPFSearchWithExplain(results, false)
}

func formatCLIFPFSearchWithExplain(results []present.FPFSearchResult, explain bool) string {
	options := sharedFPFSearchOptions()
	options.ShowMetadata = explain
	return present.FormatFPFSearch(results, options)
}

func formatMCPFPFSearch(results []present.FPFSearchResult) string {
	options := sharedFPFSearchOptions()
	return present.FormatFPFSearch(results, options)
}

func formatAgentFPFSearch(query string, results []present.FPFSearchResult) string {
	options := present.FPFSearchOptions{
		EmptyMessage: "No FPF spec matches for: " + query,
	}

	return present.FormatFPFSearch(results, options)
}

func sharedFPFSearchOptions() present.FPFSearchOptions {
	return present.FPFSearchOptions{
		Enumerate:    true,
		EmptyMessage: "No results found.",
	}
}
