package present

import (
	"fmt"
	"strings"
)

// FPFSearchResult is the presentation model for an FPF search hit.
type FPFSearchResult struct {
	PatternID string
	Heading   string
	Tier      string
	Reason    string
	Content   string
}

// FPFSearchOptions controls how FPF search results are rendered.
type FPFSearchOptions struct {
	Header       string
	Enumerate    bool
	EmptyMessage string
}

// FPFInfo contains inspectable FPF index metadata for presentation.
type FPFInfo struct {
	Version         string
	Commit          string
	Source          string
	IndexedSections string
	BuildTime       string
	SpecPath        string
	SchemaVersion   string
}

// FormatFPFSearch renders FPF search results as markdown.
func FormatFPFSearch(results []FPFSearchResult, options FPFSearchOptions) string {
	if len(results) == 0 {
		return ensureTrailingNewline(options.EmptyMessage)
	}

	var sb strings.Builder

	if header := strings.TrimSpace(options.Header); header != "" {
		sb.WriteString(header)
		sb.WriteString("\n\n")
	}

	for index, result := range results {
		sb.WriteString("### ")
		if options.Enumerate {
			sb.WriteString(fmt.Sprintf("%d. ", index+1))
		}
		sb.WriteString(formatFPFResultTitle(result))
		sb.WriteString("\n\n")

		if metadata := formatFPFResultMetadata(result); metadata != "" {
			sb.WriteString(metadata)
			sb.WriteString("\n\n")
		}

		content := strings.TrimRight(result.Content, "\n")
		if content != "" {
			sb.WriteString(content)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// FormatFPFSection renders a single FPF section body.
func FormatFPFSection(title string, body string) string {
	trimmedBody := strings.TrimRight(body, "\n")
	return fmt.Sprintf("## %s\n\n%s\n", strings.TrimSpace(title), trimmedBody)
}

// FormatFPFInfo renders FPF index metadata.
func FormatFPFInfo(info FPFInfo) string {
	lines := []string{
		fmt.Sprintf("haft fpf version: %s", strings.TrimSpace(info.Version)),
	}

	if schemaVersion := strings.TrimSpace(info.SchemaVersion); schemaVersion != "" {
		lines = append(lines, fmt.Sprintf("FPF index schema version: %s", schemaVersion))
	}
	if commit := strings.TrimSpace(info.Commit); commit != "" {
		lines = append(lines, fmt.Sprintf("FPF upstream commit: %s", commit))
	}
	if source := strings.TrimSpace(info.Source); source != "" {
		lines = append(lines, fmt.Sprintf("FPF source: %s", source))
	}
	if indexedSections := strings.TrimSpace(info.IndexedSections); indexedSections != "" {
		lines = append(lines, fmt.Sprintf("Indexed sections: %s", indexedSections))
	}
	if buildTime := strings.TrimSpace(info.BuildTime); buildTime != "" {
		lines = append(lines, fmt.Sprintf("Build time: %s", buildTime))
	}
	if specPath := strings.TrimSpace(info.SpecPath); specPath != "" {
		lines = append(lines, fmt.Sprintf("Spec path: %s", specPath))
	}

	return strings.Join(lines, "\n") + "\n"
}

func formatFPFResultTitle(result FPFSearchResult) string {
	title := strings.TrimSpace(result.Heading)
	patternID := strings.TrimSpace(result.PatternID)
	if patternID == "" || strings.Contains(title, patternID) {
		return title
	}
	return patternID + " — " + title
}

func formatFPFResultMetadata(result FPFSearchResult) string {
	tier := strings.TrimSpace(result.Tier)
	reason := strings.TrimSpace(result.Reason)

	switch {
	case tier != "" && reason != "":
		return fmt.Sprintf("tier: %s · %s", tier, reason)
	case tier != "":
		return "tier: " + tier
	default:
		return reason
	}
}

func ensureTrailingNewline(text string) string {
	if text == "" {
		return ""
	}
	if strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}
