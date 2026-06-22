package codebase

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	BindingSupportSymbols             = "symbols_supported"
	BindingSupportRangeOnly           = "range_only"
	BindingSupportUnsupportedLanguage = "unsupported_language"
	BindingSupportReadFailed          = "read_failed"
)

type FileBindingSupport struct {
	FilePath string
	Language string
	Posture  string
	Symbols  []SymbolSnapshot
	Ranges   []StableRangeSnapshot
	Reason   string
}

type BindingLanguageSupport struct {
	Language         string   `json:"language"`
	Extensions       []string `json:"extensions"`
	SymbolExtraction bool     `json:"symbol_extraction"`
	RangeFallback    bool     `json:"range_fallback"`
}

type StableRangeSnapshot struct {
	FilePath    string
	Language    string
	StartLine   int
	EndLine     int
	AnchorHash  string
	TextHash    string
	NearestName string
}

func LanguageForPath(relPath string) (string, bool) {
	info, ok := languages[filepath.Ext(relPath)]
	if !ok {
		return "", false
	}
	return info.name, true
}

func SupportedBindingLanguages() []BindingLanguageSupport {
	byLanguage := make(map[string][]string)
	for ext, info := range languages {
		byLanguage[info.name] = append(byLanguage[info.name], ext)
	}

	out := make([]BindingLanguageSupport, 0, len(byLanguage))
	for language, extensions := range byLanguage {
		sort.Strings(extensions)
		out = append(out, BindingLanguageSupport{
			Language:         language,
			Extensions:       extensions,
			SymbolExtraction: true,
			RangeFallback:    true,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Language < out[j].Language
	})
	return out
}

func InspectFileBindingSupport(projectRoot, relPath string) FileBindingSupport {
	language, ok := LanguageForPath(relPath)
	if !ok {
		return FileBindingSupport{
			FilePath: relPath,
			Posture:  BindingSupportUnsupportedLanguage,
			Reason:   "file extension is not supported by the symbol extractor",
		}
	}

	symbols, err := ExtractSymbolSnapshots(projectRoot, relPath)
	if err == nil && len(symbols) > 0 {
		return FileBindingSupport{
			FilePath: relPath,
			Language: language,
			Posture:  BindingSupportSymbols,
			Symbols:  symbols,
		}
	}

	stableRange, rangeErr := ExtractStableFileRange(projectRoot, relPath)
	if rangeErr != nil {
		return FileBindingSupport{
			FilePath: relPath,
			Language: language,
			Posture:  BindingSupportReadFailed,
			Reason:   rangeErr.Error(),
		}
	}

	return FileBindingSupport{
		FilePath: relPath,
		Language: language,
		Posture:  BindingSupportRangeOnly,
		Ranges:   []StableRangeSnapshot{stableRange},
		Reason:   "supported source file has no extracted named symbols",
	}
}

func ExtractStableFileRange(projectRoot, relPath string) (StableRangeSnapshot, error) {
	absPath := filepath.Join(projectRoot, relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return StableRangeSnapshot{}, err
	}

	language, _ := LanguageForPath(relPath)
	normalized := normalizeRangeText(string(content))
	anchor := firstNonEmptyLine(normalized)
	anchorHash := sha256.Sum256([]byte(anchor))
	textHash := sha256.Sum256([]byte(normalized))

	return StableRangeSnapshot{
		FilePath:   relPath,
		Language:   language,
		StartLine:  1,
		EndLine:    countLines(content),
		AnchorHash: hex.EncodeToString(anchorHash[:]),
		TextHash:   hex.EncodeToString(textHash[:]),
	}, nil
}

func normalizeRangeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		lines = append(lines, strings.TrimRight(scanner.Text(), " \t"))
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	return strings.Count(string(content), "\n") + 1
}
