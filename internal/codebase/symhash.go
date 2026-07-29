package codebase

import (
	gocontext "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
)

// SymbolSnapshot captures a symbol's identity and content hash at a point in time.
type SymbolSnapshot struct {
	FilePath      string `json:"file_path"`
	SymbolName    string `json:"symbol_name"`
	SymbolKind    string `json:"symbol_kind"`        // func, method, class, interface, type_alias, enum, constant, variable, property
	QualifiedName string `json:"qualified_name"`     // Receiver.member for methods, bare name for file-scope declarations
	SignatureHash string `json:"signature_hash"`     // declaration signature, excluding body and source coordinates
	Line          int    `json:"line"`               // 1-based start line
	EndLine       int    `json:"end_line"`           // 1-based end line
	Hash          string `json:"hash"`               // SHA256 of the symbol's source text
	StartByte     int    `json:"start_byte"`         // body start byte offset — for byte-exact source slicing
	EndByte       int    `json:"end_byte"`           // body end byte offset
	Receiver      string `json:"receiver,omitempty"` // method receiver type (Go), "" otherwise
	Exported      bool   `json:"exported"`           // first rune uppercase (Go export proxy)
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
	registry := NewRegistry()
	return registry.ExtractSymbolSnapshots(projectRoot, relPath)
}

// extractLegacySymbolSnapshots is the transitional adapter for languages that
// have not moved to a dedicated SymbolAdapter yet. It preserves their existing
// query behavior while every consumer already reads the canonical snapshots.
func extractLegacySymbolSnapshots(
	source AdmittedSource,
	langInfo *languageInfo,
) ([]SymbolSnapshot, error) {
	relPath := source.Path().String()
	content := source.bytes()

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

			var name, receiverSrc string
			var bodyNode *sitter.Node

			for _, capture := range match.Captures {
				captName := q.CaptureNameForId(capture.Index)
				switch captName {
				case "name":
					name = capture.Node.Content(content)
				case "receiver":
					receiverSrc = capture.Node.Content(content)
				case "body":
					bodyNode = capture.Node
				}
			}

			if name == "" || bodyNode == nil {
				continue
			}
			bodyStart := bodyNode.StartByte()
			bodyEnd := bodyNode.EndByte()
			if bodyEnd <= bodyStart {
				continue
			}

			bodyText := content[bodyStart:bodyEnd]
			h := sha256.Sum256(bodyText)

			startLine := int(bodyNode.StartPoint().Row) + 1
			endLine := int(bodyNode.EndPoint().Row) + 1

			snapshots = append(snapshots, SymbolSnapshot{
				FilePath:   relPath,
				SymbolName: name,
				SymbolKind: bq.kind,
				Line:       startLine,
				EndLine:    endLine,
				Hash:       hex.EncodeToString(h[:]),
				StartByte:  int(bodyStart),
				EndByte:    int(bodyEnd),
				Receiver:   parseReceiverType(receiverSrc),
				Exported:   isExportedName(name),
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
	baseGroups := groupSymbolSnapshots(baseline)
	currentGroups := groupSymbolSnapshots(current)
	var drifts []SymbolDrift
	seenGroups := map[string]bool{}
	for key, baseGroup := range baseGroups {
		seenGroups[key] = true
		currentGroup := currentGroups[key]
		drifts = append(drifts, compareSymbolSnapshotGroup(baseGroup, currentGroup)...)
	}
	for key, currentGroup := range currentGroups {
		if seenGroups[key] {
			continue
		}
		for _, currentSnapshot := range currentGroup {
			drifts = append(drifts, addedSymbolDrift(currentSnapshot))
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

func groupSymbolSnapshots(snapshots []SymbolSnapshot) map[string][]SymbolSnapshot {
	groups := make(map[string][]SymbolSnapshot)
	for _, snapshot := range snapshots {
		normalized := normalizeSnapshotForComparison(snapshot)
		key := symbolSnapshotBaseKey(normalized)
		groups[key] = append(groups[key], normalized)
	}
	return groups
}

func normalizeSnapshotForComparison(snapshot SymbolSnapshot) SymbolSnapshot {
	snapshot.FilePath = normalizeAnchorPath(snapshot.FilePath)
	snapshot.SymbolName = strings.TrimSpace(snapshot.SymbolName)
	snapshot.SymbolKind = strings.TrimSpace(snapshot.SymbolKind)
	snapshot.Receiver = strings.TrimSpace(snapshot.Receiver)
	if strings.TrimSpace(snapshot.QualifiedName) == "" {
		snapshot.QualifiedName = qualifiedSymbolName(snapshot.Receiver, snapshot.SymbolName)
	}
	snapshot.SignatureHash = strings.TrimSpace(snapshot.SignatureHash)
	return snapshot
}

func symbolSnapshotBaseKey(snapshot SymbolSnapshot) string {
	parts := []string{
		snapshot.FilePath,
		snapshot.SymbolKind,
		snapshot.QualifiedName,
	}
	return strings.Join(parts, "\x00")
}

func compareSymbolSnapshotGroup(baseline, current []SymbolSnapshot) []SymbolDrift {
	used := make([]bool, len(current))
	drifts := make([]SymbolDrift, 0)
	for _, baselineSnapshot := range baseline {
		position := matchCurrentSnapshot(baselineSnapshot, current, used, len(baseline), len(current))
		if position < 0 {
			drifts = append(drifts, removedSymbolDrift(baselineSnapshot))
			continue
		}
		used[position] = true
		currentSnapshot := current[position]
		if currentSnapshot.Hash != baselineSnapshot.Hash {
			drifts = append(drifts, modifiedSymbolDrift(baselineSnapshot, currentSnapshot))
		}
	}
	for position, currentSnapshot := range current {
		if !used[position] {
			drifts = append(drifts, addedSymbolDrift(currentSnapshot))
		}
	}
	return drifts
}

func matchCurrentSnapshot(
	baseline SymbolSnapshot,
	current []SymbolSnapshot,
	used []bool,
	baselineCount int,
	currentCount int,
) int {
	if baselineCount == 1 && currentCount == 1 {
		return 0
	}
	if baseline.SignatureHash != "" {
		for position, candidate := range current {
			if !used[position] && candidate.SignatureHash == baseline.SignatureHash {
				return position
			}
		}
	}
	bestPosition := -1
	bestDistance := 0
	for position, candidate := range current {
		if used[position] {
			continue
		}
		distance := snapshotLineDistance(baseline.Line, candidate.Line)
		if bestPosition < 0 || distance < bestDistance {
			bestPosition = position
			bestDistance = distance
		}
	}
	return bestPosition
}

func snapshotLineDistance(first, second int) int {
	if first > second {
		return first - second
	}
	return second - first
}

func removedSymbolDrift(snapshot SymbolSnapshot) SymbolDrift {
	return SymbolDrift{
		FilePath:   snapshot.FilePath,
		SymbolName: snapshot.SymbolName,
		SymbolKind: snapshot.SymbolKind,
		Status:     "removed",
		OldLine:    snapshot.Line,
	}
}

func modifiedSymbolDrift(baseline, current SymbolSnapshot) SymbolDrift {
	return SymbolDrift{
		FilePath:   baseline.FilePath,
		SymbolName: baseline.SymbolName,
		SymbolKind: baseline.SymbolKind,
		Status:     "modified",
		OldLine:    baseline.Line,
		NewLine:    current.Line,
	}
}

func addedSymbolDrift(snapshot SymbolSnapshot) SymbolDrift {
	return SymbolDrift{
		FilePath:   snapshot.FilePath,
		SymbolName: snapshot.SymbolName,
		SymbolKind: snapshot.SymbolKind,
		Status:     "added",
		NewLine:    snapshot.Line,
	}
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

// parseReceiverType extracts the bare receiver type from a Go method receiver
// parameter list (e.g. "(s *Store)" → "Store", "(*Store)" → "Store",
// "(s Store[T])" → "Store"). Best-effort; returns "" for non-method symbols.
func parseReceiverType(receiverSrc string) string {
	t := strings.TrimSpace(receiverSrc)
	t = strings.TrimPrefix(t, "(")
	t = strings.TrimSuffix(t, ")")
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	fields := strings.Fields(t)
	last := fields[len(fields)-1] // the type token (or the only token)
	last = strings.TrimPrefix(last, "*")
	if i := strings.IndexByte(last, '['); i >= 0 {
		last = last[:i] // drop generic type parameters
	}
	return last
}

// isExportedName reports whether a symbol name begins with an uppercase rune —
// a Go export proxy, harmless (and roughly meaningful) for other languages.
func isExportedName(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper([]rune(name)[0])
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
			{"(method_declaration receiver: (parameter_list) @receiver name: (field_identifier) @name) @body", "method"},
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
