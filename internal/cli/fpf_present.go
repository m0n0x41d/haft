package cli

import "github.com/m0n0x41d/haft/internal/present"

//nolint:unused // exercised by package tests
func formatCLIFPFSearch(results []present.FPFSearchResult) string {
	return formatCLIFPFSearchWithExplain(results, false)
}

func formatCLIFPFSearchWithExplain(results []present.FPFSearchResult, explain bool) string {
	options := sharedFPFSearchOptions()
	options.ShowMetadata = explain
	return present.FormatFPFSearch(results, options)
}

//nolint:unused // exercised by package tests
func formatMCPFPFSearch(results []present.FPFSearchResult) string {
	return formatMCPFPFSearchWithExplain(results, false)
}

func formatMCPFPFSearchWithExplain(results []present.FPFSearchResult, explain bool) string {
	options := sharedFPFSearchOptions()
	options.ShowMetadata = explain
	return present.FormatFPFSearch(results, options)
}

//nolint:unused // exercised by package tests
func formatAgentFPFSearch(query string, results []present.FPFSearchResult) string {
	return formatAgentFPFSearchWithExplain(query, results, false)
}

func formatAgentFPFSearchWithExplain(query string, results []present.FPFSearchResult, explain bool) string {
	options := present.FPFSearchOptions{
		EmptyMessage: "No FPF spec matches for: " + query,
		ShowMetadata: explain,
	}

	return present.FormatFPFSearch(results, options)
}

func sharedFPFSearchOptions() present.FPFSearchOptions {
	return present.FPFSearchOptions{
		Enumerate:    true,
		EmptyMessage: "No results found.",
	}
}
