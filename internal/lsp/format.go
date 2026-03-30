package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// L1: Pure functions — formatting, language detection, URI conversion
// ---------------------------------------------------------------------------

// DetectLanguage returns the LSP languageId for a file extension.
func DetectLanguage(path string) string {
	langs := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".jsx":   "javascriptreact",
		".ts":    "typescript",
		".tsx":   "typescriptreact",
		".rs":    "rust",
		".c":     "c",
		".cpp":   "cpp",
		".cc":    "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".java":  "java",
		".rb":    "ruby",
		".php":   "php",
		".swift": "swift",
		".kt":    "kotlin",
		".cs":    "csharp",
		".lua":   "lua",
		".zig":   "zig",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".toml":  "toml",
		".html":  "html",
		".css":   "css",
		".scss":  "scss",
		".md":    "markdown",
		".sh":    "shellscript",
		".bash":  "shellscript",
		".zsh":   "shellscript",
	}
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := langs[ext]; ok {
		return lang
	}
	return "plaintext"
}

// FileToURI converts an absolute file path to a file:// URI.
func FileToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file://" + abs
}

// URIToFile converts a file:// URI to a file path.
func URIToFile(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		return u.Path
	}
	return strings.TrimPrefix(uri, "file://")
}

// SeverityString returns a human-readable label for a diagnostic severity.
func SeverityString(s DiagnosticSeverity) string {
	switch s {
	case SeverityError:
		return "Error"
	case SeverityWarning:
		return "Warn"
	case SeverityInfo:
		return "Info"
	case SeverityHint:
		return "Hint"
	default:
		return "Unknown"
	}
}

// FormatDiagnostic renders a single diagnostic as a one-line string.
// Format: "Error: path:line:col [source]code message"
func FormatDiagnostic(d Diagnostic, projectRoot string) string {
	path := d.File
	if rel, err := filepath.Rel(projectRoot, d.File); err == nil {
		path = rel
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s:%d:%d", SeverityString(d.Severity), path, d.Line, d.Col)
	if d.Source != "" || d.Code != "" {
		b.WriteString(" [")
		if d.Source != "" {
			b.WriteString(d.Source)
		}
		if d.Code != "" {
			if d.Source != "" {
				b.WriteString("/")
			}
			b.WriteString(d.Code)
		}
		b.WriteString("]")
	}
	b.WriteString(" ")
	b.WriteString(d.Message)

	if len(d.Tags) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(d.Tags, ", "))
		b.WriteString(")")
	}

	return b.String()
}

// FormatDiagnostics renders a list of diagnostics grouped by file.
func FormatDiagnostics(diags []Diagnostic, projectRoot string) string {
	if len(diags) == 0 {
		return "No diagnostics."
	}

	// Sort: errors first, then by file, then by line
	sorted := make([]Diagnostic, len(diags))
	copy(sorted, diags)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Severity != sorted[j].Severity {
			return sorted[i].Severity < sorted[j].Severity
		}
		if sorted[i].File != sorted[j].File {
			return sorted[i].File < sorted[j].File
		}
		return sorted[i].Line < sorted[j].Line
	})

	var b strings.Builder
	for _, d := range sorted {
		b.WriteString(FormatDiagnostic(d, projectRoot))
		b.WriteString("\n")
	}
	return b.String()
}

// FormatLocation renders a location as "file:line:col".
func FormatLocation(loc Location, projectRoot string) string {
	path := loc.File
	if rel, err := filepath.Rel(projectRoot, loc.File); err == nil {
		path = rel
	}
	return fmt.Sprintf("%s:%d:%d", path, loc.StartLine, loc.StartCol)
}

// CountDiagnostics aggregates diagnostics by severity.
func CountDiagnostics(diags []Diagnostic) DiagnosticCounts {
	var c DiagnosticCounts
	for _, d := range diags {
		switch d.Severity {
		case SeverityError:
			c.Error++
		case SeverityWarning:
			c.Warning++
		case SeverityInfo:
			c.Info++
		case SeverityHint:
			c.Hint++
		}
	}
	return c
}

// ConvertRawDiag converts a raw protocol diagnostic to our type.
func ConvertRawDiag(uri string, raw rawDiag) Diagnostic {
	d := Diagnostic{
		File:     URIToFile(uri),
		Line:     raw.Range.Start.Line + 1,     // 0-based → 1-based
		Col:      raw.Range.Start.Character + 1, // 0-based → 1-based
		Severity: DiagnosticSeverity(raw.Severity),
		Source:   raw.Source,
		Message:  raw.Message,
	}

	// Code can be string or int
	switch v := raw.Code.(type) {
	case string:
		d.Code = v
	case float64:
		d.Code = fmt.Sprintf("%d", int(v))
	}

	// Tags: 1=unnecessary, 2=deprecated
	for _, tag := range raw.Tags {
		switch tag {
		case 1:
			d.Tags = append(d.Tags, "unnecessary")
		case 2:
			d.Tags = append(d.Tags, "deprecated")
		}
	}

	return d
}
