package codebase

import (
	gocontext "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// SymbolSnapshot captures a symbol's identity and content hash at a point in time.
type SymbolSnapshot struct {
	FilePath   string `json:"file_path"`
	SymbolName string `json:"symbol_name"`
	SymbolKind string `json:"symbol_kind"` // func, type, class, interface, method
	Line       int    `json:"line"`        // 1-based start line
	EndLine    int    `json:"end_line"`    // 1-based end line
	Hash       string `json:"hash"`        // SHA256 of the symbol's source text
}

// SymbolDrift describes how a single symbol changed between baseline and current.
type SymbolDrift struct {
	FilePath   string `json:"file_path"`
	SymbolName string `json:"symbol_name"`
	SymbolKind string `json:"symbol_kind"`
	Status     string `json:"status"` // "unchanged", "modified", "added", "removed"
	OldLine    int    `json:"old_line,omitempty"`
	NewLine    int    `json:"new_line,omitempty"`
}

// ExtractSymbolSnapshots extracts symbol-level hashes from a file using tree-sitter.
// Returns one snapshot per symbol, each with a content hash of the symbol's source text.
func ExtractSymbolSnapshots(projectRoot, relPath string) ([]SymbolSnapshot, error) {
	ext := filepath.Ext(relPath)
	langInfo, ok := languages[ext]
	if !ok {
		return nil, nil // unsupported language — skip silently
	}

	absPath := filepath.Join(projectRoot, relPath)
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relPath, err)
	}

	if len(content) > 500_000 {
		return nil, nil // skip very large files
	}

	parser := sitter.NewParser()
	parser.SetLanguage(langInfo.lang)
	tree, err := parser.ParseCtx(gocontext.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", relPath, err)
	}
	defer tree.Close()

	var snapshots []SymbolSnapshot

	// Use broader queries that capture the full node (not just the name)
	// so we can hash the complete symbol body
	bodyQueries := symbolBodyQueries(langInfo.name, langInfo.lang)

	for _, bq := range bodyQueries {
		q, err := sitter.NewQuery([]byte(bq.pattern), langInfo.lang)
		if err != nil {
			continue
		}

		qc := sitter.NewQueryCursor()
		qc.Exec(q, tree.RootNode())

		for {
			match, ok := qc.NextMatch()
			if !ok {
				break
			}

			var name string
			var bodyStart, bodyEnd uint32

			for _, capture := range match.Captures {
				captName := q.CaptureNameForId(capture.Index)
				switch captName {
				case "name":
					name = capture.Node.Content(content)
				case "body":
					bodyStart = capture.Node.StartByte()
					bodyEnd = capture.Node.EndByte()
				}
			}

			if name == "" || bodyEnd <= bodyStart {
				continue
			}

			bodyText := content[bodyStart:bodyEnd]
			h := sha256.Sum256(bodyText)

			startLine := int(match.Captures[0].Node.StartPoint().Row) + 1
			endLine := int(match.Captures[len(match.Captures)-1].Node.EndPoint().Row) + 1

			snapshots = append(snapshots, SymbolSnapshot{
				FilePath:   relPath,
				SymbolName: name,
				SymbolKind: bq.kind,
				Line:       startLine,
				EndLine:    endLine,
				Hash:       hex.EncodeToString(h[:]),
			})
		}
		q.Close()
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Line < snapshots[j].Line
	})

	return snapshots, nil
}

// CompareSymbolSnapshots compares baseline snapshots against current state.
func CompareSymbolSnapshots(baseline []SymbolSnapshot, current []SymbolSnapshot) []SymbolDrift {
	baseMap := make(map[string]SymbolSnapshot) // key: file:name
	for _, s := range baseline {
		baseMap[s.FilePath+":"+s.SymbolName] = s
	}

	currMap := make(map[string]SymbolSnapshot)
	for _, s := range current {
		currMap[s.FilePath+":"+s.SymbolName] = s
	}

	var drifts []SymbolDrift

	// Check baseline symbols against current
	for key, base := range baseMap {
		curr, exists := currMap[key]
		if !exists {
			drifts = append(drifts, SymbolDrift{
				FilePath:   base.FilePath,
				SymbolName: base.SymbolName,
				SymbolKind: base.SymbolKind,
				Status:     "removed",
				OldLine:    base.Line,
			})
			continue
		}
		if curr.Hash != base.Hash {
			drifts = append(drifts, SymbolDrift{
				FilePath:   base.FilePath,
				SymbolName: base.SymbolName,
				SymbolKind: base.SymbolKind,
				Status:     "modified",
				OldLine:    base.Line,
				NewLine:    curr.Line,
			})
		}
		// unchanged — don't report
	}

	// Check for new symbols not in baseline
	for key, curr := range currMap {
		if _, exists := baseMap[key]; !exists {
			drifts = append(drifts, SymbolDrift{
				FilePath:   curr.FilePath,
				SymbolName: curr.SymbolName,
				SymbolKind: curr.SymbolKind,
				Status:     "added",
				NewLine:    curr.Line,
			})
		}
	}

	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].FilePath != drifts[j].FilePath {
			return drifts[i].FilePath < drifts[j].FilePath
		}
		return drifts[i].SymbolName < drifts[j].SymbolName
	})

	return drifts
}

// FormatSymbolDrift renders drift report for display.
func FormatSymbolDrift(drifts []SymbolDrift) string {
	if len(drifts) == 0 {
		return "No symbol-level drift detected."
	}

	var b strings.Builder
	currentFile := ""
	for _, d := range drifts {
		if d.FilePath != currentFile {
			if currentFile != "" {
				b.WriteString("\n")
			}
			b.WriteString(d.FilePath + ":\n")
			currentFile = d.FilePath
		}

		switch d.Status {
		case "modified":
			b.WriteString(fmt.Sprintf("  ~ %s %s (line %d→%d)\n", d.SymbolKind, d.SymbolName, d.OldLine, d.NewLine))
		case "removed":
			b.WriteString(fmt.Sprintf("  - %s %s (was line %d)\n", d.SymbolKind, d.SymbolName, d.OldLine))
		case "added":
			b.WriteString(fmt.Sprintf("  + %s %s (line %d)\n", d.SymbolKind, d.SymbolName, d.NewLine))
		}
	}

	return b.String()
}

// symbolBodyQuery captures both the symbol name and its full body for hashing.
type symbolBodyQuery struct {
	pattern string
	kind    string
}

// symbolBodyQueries returns tree-sitter queries that capture both @name and @body
// for each symbol type in the given language.
func symbolBodyQueries(langName string, lang *sitter.Language) []symbolBodyQuery {
	switch langName {
	case "go":
		return []symbolBodyQuery{
			{"(function_declaration name: (identifier) @name) @body", "func"},
			{"(method_declaration name: (field_identifier) @name) @body", "method"},
			{"(type_declaration (type_spec name: (type_identifier) @name)) @body", "type"},
		}
	case "python":
		return []symbolBodyQuery{
			{"(function_definition name: (identifier) @name) @body", "func"},
			{"(class_definition name: (identifier) @name) @body", "class"},
		}
	case "javascript":
		return []symbolBodyQuery{
			{"(function_declaration name: (identifier) @name) @body", "func"},
			{"(class_declaration name: (identifier) @name) @body", "class"},
			{"(method_definition name: (property_identifier) @name) @body", "method"},
		}
	case "typescript":
		return []symbolBodyQuery{
			{"(function_declaration name: (identifier) @name) @body", "func"},
			{"(class_declaration name: (type_identifier) @name) @body", "class"},
			{"(interface_declaration name: (type_identifier) @name) @body", "interface"},
			{"(method_definition name: (property_identifier) @name) @body", "method"},
		}
	case "rust":
		return []symbolBodyQuery{
			{"(function_item name: (identifier) @name) @body", "func"},
			{"(struct_item name: (type_identifier) @name) @body", "type"},
			{"(enum_item name: (type_identifier) @name) @body", "type"},
			{"(trait_item name: (type_identifier) @name) @body", "interface"},
			{"(impl_item type: (type_identifier) @name) @body", "type"},
		}
	case "c", "cpp":
		return []symbolBodyQuery{
			{"(function_definition declarator: (function_declarator declarator: (identifier) @name)) @body", "func"},
			{"(struct_specifier name: (type_identifier) @name) @body", "type"},
		}
	}
	return nil
}
