package present

import (
	"fmt"
	"strings"
)

// FPFSearchResult is the presentation model for an FPF search hit.
type FPFSearchResult struct {
	PatternID  string
	Heading    string
	Tier       string
	Reason     string
	Summary    string
	Content    string
	Provenance FPFSearchProvenance
}

type FPFSearchProvenance struct {
	ProfileID          string
	SourceKind         string
	SourceEdition      string
	SourceRef          string
	SourceHash         string
	ProfileValidity    string
	Normativity        string
	IndexSchemaVersion string
	RetrievalMode      string
}

// FPFSearchOptions controls how FPF search results are rendered.
type FPFSearchOptions struct {
	Header       string
	Enumerate    bool
	ShowMetadata bool
	EmptyMessage string
}

// FPFInfo contains inspectable FPF index metadata for presentation.
type FPFInfo struct {
	Version           string
	Commit            string
	IndexedSections   string
	BuildTime         string
	SpecPath          string
	SchemaVersion     string
	SourceEdition     string
	ProfileValidUntil string
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

		metadata := formatFPFResultMetadata(result, options.ShowMetadata)
		if metadata != "" {
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
	if sourceEdition := strings.TrimSpace(info.SourceEdition); sourceEdition != "" {
		lines = append(lines, fmt.Sprintf("FPF source edition: %s", sourceEdition))
	}
	if profileValidUntil := strings.TrimSpace(info.ProfileValidUntil); profileValidUntil != "" {
		lines = append(lines, fmt.Sprintf("FPF profile valid until: %s", profileValidUntil))
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
	if patternID == "" {
		return title
	}

	titleFolded := strings.ToUpper(title)
	patternFolded := strings.ToUpper(patternID)
	if strings.Contains(titleFolded, patternFolded) {
		return title
	}
	return patternID + " — " + title
}

func formatFPFResultMetadata(result FPFSearchResult, showMetadata bool) string {
	if !showMetadata {
		return ""
	}

	tier := strings.TrimSpace(result.Tier)
	reason := strings.TrimSpace(result.Reason)
	summary := strings.TrimSpace(result.Summary)
	lines := make([]string, 0, 2)

	switch {
	case tier != "" && reason != "":
		lines = append(lines, fmt.Sprintf("tier: %s · %s", tier, reason))
	case tier != "":
		lines = append(lines, "tier: "+tier)
	case reason != "":
		lines = append(lines, reason)
	}

	if summary != "" {
		lines = append(lines, "summary: "+summary)
	}
	if provenance := formatFPFProvenance(result.Provenance); provenance != "" {
		lines = append(lines, provenance)
	}

	return strings.Join(lines, "\n")
}

func formatFPFProvenance(provenance FPFSearchProvenance) string {
	profileID := strings.TrimSpace(provenance.ProfileID)
	if profileID == "" {
		return ""
	}

	parts := []string{"profile: " + profileID}
	if sourceKind := strings.TrimSpace(provenance.SourceKind); sourceKind != "" {
		parts = append(parts, "source_kind="+sourceKind)
	}
	if sourceEdition := strings.TrimSpace(provenance.SourceEdition); sourceEdition != "" {
		parts = append(parts, "source_edition="+sourceEdition)
	}
	if sourceRef := strings.TrimSpace(provenance.SourceRef); sourceRef != "" {
		parts = append(parts, "source_ref="+sourceRef)
	}
	if sourceHash := strings.TrimSpace(provenance.SourceHash); sourceHash != "" {
		parts = append(parts, "source_hash="+sourceHash)
	}
	if profileValidity := strings.TrimSpace(provenance.ProfileValidity); profileValidity != "" {
		parts = append(parts, "profile_validity="+profileValidity)
	}
	if normativity := strings.TrimSpace(provenance.Normativity); normativity != "" {
		parts = append(parts, "normativity="+normativity)
	}
	if schemaVersion := strings.TrimSpace(provenance.IndexSchemaVersion); schemaVersion != "" {
		parts = append(parts, "index_schema="+schemaVersion)
	}
	if retrievalMode := strings.TrimSpace(provenance.RetrievalMode); retrievalMode != "" {
		parts = append(parts, "retrieval="+retrievalMode)
	}

	return strings.Join(parts, " · ")
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
