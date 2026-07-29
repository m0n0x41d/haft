package codebase

import (
	gocontext "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type tsSymbolCandidate struct {
	name     string
	kind     string
	receiver string
	body     *sitter.Node
	exported bool
}

// SymbolLanguage names the concrete grammar used by a JS/TS file. JSTSLang's
// Language method intentionally remains "jsts" for module detection; persisted
// code symbols use the more precise language name.
func (j *JSTSLang) SymbolLanguage(path string) string {
	ext := normalizedExtension(path)
	switch ext {
	case ".ts", ".tsx", ".mts", ".cts":
		return "typescript"
	default:
		return "javascript"
	}
}

// ExtractSymbolSnapshots implements the rich JS/TS SymbolAdapter. It produces
// one canonical node for each declaration: an arrow assigned to a const is a
// func, never both a constant wrapper and a duplicate anonymous function.
func (j *JSTSLang) ExtractSymbolSnapshots(
	source AdmittedSource,
) ([]SymbolSnapshot, error) {
	relPath := source.Path().String()
	lang := tsGrammarForExt(normalizedExtension(relPath))
	if lang == nil {
		return nil, nil
	}
	content := source.bytes()
	return extractTSSymbolSnapshotsFromContent(relPath, content, lang)
}

func extractTSSymbolSnapshotsFromContent(relPath string, content []byte, lang *sitter.Language) ([]SymbolSnapshot, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(gocontext.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", relPath, err)
	}
	defer tree.Close()

	candidates := extractTSSymbolCandidates(tree.RootNode(), content)
	snapshots := make([]SymbolSnapshot, 0, len(candidates))
	for _, candidate := range candidates {
		snapshot, ok := tsSnapshot(relPath, content, candidate)
		if ok {
			snapshots = append(snapshots, snapshot)
		}
	}
	snapshots = dedupeTSSnapshots(snapshots)
	sort.Slice(snapshots, func(i, k int) bool {
		if snapshots[i].StartByte != snapshots[k].StartByte {
			return snapshots[i].StartByte < snapshots[k].StartByte
		}
		return snapshots[i].SymbolName < snapshots[k].SymbolName
	})
	return snapshots, nil
}

func extractTSSymbolCandidates(root *sitter.Node, content []byte) []tsSymbolCandidate {
	var out []tsSymbolCandidate
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		candidate, ok := tsDeclarationCandidate(node, content)
		if ok {
			out = append(out, candidate)
		}
		out = append(out, tsContainerMembers(node, content)...)
		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(root)
	return out
}

func tsDeclarationCandidate(node *sitter.Node, content []byte) (tsSymbolCandidate, bool) {
	typ := node.Type()
	switch typ {
	case "function_declaration", "function_signature":
		return tsNamedCandidate(node, content, "func", "")
	case "class_declaration", "abstract_class_declaration":
		return tsNamedCandidate(node, content, "class", "")
	case "interface_declaration":
		return tsNamedCandidate(node, content, "interface", "")
	case "type_alias_declaration":
		return tsNamedCandidate(node, content, "type_alias", "")
	case "enum_declaration":
		return tsNamedCandidate(node, content, "enum", "")
	case "method_definition":
		receiver := tsEnclosingClassName(node, content)
		if receiver == "" {
			return tsSymbolCandidate{}, false
		}
		return tsNamedCandidate(node, content, "method", receiver)
	case "public_field_definition", "field_definition":
		receiver := tsEnclosingClassName(node, content)
		if receiver == "" {
			return tsSymbolCandidate{}, false
		}
		kind := "property"
		value := node.ChildByFieldName("value")
		if tsCallableValue(value, 0) {
			kind = "method"
		}
		return tsNamedCandidate(node, content, kind, receiver)
	case "variable_declarator":
		return tsVariableCandidate(node, content)
	}
	return tsSymbolCandidate{}, false
}

func tsNamedCandidate(node *sitter.Node, content []byte, kind, receiver string) (tsSymbolCandidate, bool) {
	name := tsNodeName(node, content)
	if name == "" {
		return tsSymbolCandidate{}, false
	}
	exported := tsExported(node)
	if receiver != "" {
		exported = tsExportedContainer(node) && !tsPrivate(node, content)
	}
	return tsSymbolCandidate{
		name:     name,
		kind:     kind,
		receiver: receiver,
		body:     node,
		exported: exported,
	}, true
}

func tsVariableCandidate(node *sitter.Node, content []byte) (tsSymbolCandidate, bool) {
	if !tsFileScopeVariable(node) {
		return tsSymbolCandidate{}, false
	}
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil || !tsSimpleNameNode(nameNode) {
		return tsSymbolCandidate{}, false
	}
	name := strings.Trim(nameNode.Content(content), "'\"`")
	if name == "" {
		return tsSymbolCandidate{}, false
	}
	value := node.ChildByFieldName("value")
	kind := tsVariableKind(node)
	if tsCallableValue(value, 0) {
		kind = "func"
	}
	return tsSymbolCandidate{
		name:     name,
		kind:     kind,
		body:     node,
		exported: tsExported(node),
	}, true
}

func tsContainerMembers(node *sitter.Node, content []byte) []tsSymbolCandidate {
	switch node.Type() {
	case "variable_declarator":
		return tsObjectMembers(node, content)
	case "enum_declaration":
		return tsEnumMembers(node, content)
	case "type_alias_declaration":
		return tsTypeAliasMembers(node, content)
	default:
		return nil
	}
}

func tsObjectMembers(node *sitter.Node, content []byte) []tsSymbolCandidate {
	if !tsFileScopeVariable(node) {
		return nil
	}
	container := tsNodeName(node, content)
	value := node.ChildByFieldName("value")
	if container == "" || value == nil || !tsObjectNode(value) {
		return nil
	}
	exported := tsExported(node)
	var out []tsSymbolCandidate
	for i := 0; i < int(value.NamedChildCount()); i++ {
		member := value.NamedChild(i)
		if member == nil {
			continue
		}
		candidate, ok := tsObjectMemberCandidate(member, content, container, exported)
		if ok {
			out = append(out, candidate)
		}
	}
	return out
}

func tsObjectMemberCandidate(node *sitter.Node, content []byte, receiver string, exported bool) (tsSymbolCandidate, bool) {
	if node.Type() == "method_definition" {
		name := tsNodeName(node, content)
		return tsMemberCandidate(node, name, "method", receiver, exported)
	}
	if node.Type() != "pair" {
		return tsSymbolCandidate{}, false
	}
	key := node.ChildByFieldName("key")
	value := node.ChildByFieldName("value")
	if key == nil || !tsCallableValue(value, 0) {
		return tsSymbolCandidate{}, false
	}
	name := strings.Trim(key.Content(content), "'\"`")
	return tsMemberCandidate(node, name, "method", receiver, exported)
}

func tsMemberCandidate(node *sitter.Node, name, kind, receiver string, exported bool) (tsSymbolCandidate, bool) {
	if name == "" {
		return tsSymbolCandidate{}, false
	}
	return tsSymbolCandidate{
		name:     name,
		kind:     kind,
		receiver: receiver,
		body:     node,
		exported: exported,
	}, true
}

func tsEnumMembers(node *sitter.Node, content []byte) []tsSymbolCandidate {
	receiver := tsNodeName(node, content)
	body := firstChildOfType(node, "enum_body")
	if receiver == "" || body == nil {
		return nil
	}
	exported := tsExported(node)
	var out []tsSymbolCandidate
	for i := 0; i < int(body.NamedChildCount()); i++ {
		member := body.NamedChild(i)
		if member == nil {
			continue
		}
		name := tsNodeName(member, content)
		if name == "" && tsSimpleNameNode(member) {
			name = strings.Trim(member.Content(content), "'\"`")
		}
		candidate, ok := tsMemberCandidate(member, name, "enum_member", receiver, exported)
		if ok {
			out = append(out, candidate)
		}
	}
	return out
}

func tsTypeAliasMembers(node *sitter.Node, content []byte) []tsSymbolCandidate {
	receiver := tsNodeName(node, content)
	objectType := firstChildOfType(node, "object_type")
	if receiver == "" || objectType == nil {
		return nil
	}
	exported := tsExported(node)
	var out []tsSymbolCandidate
	for i := 0; i < int(objectType.NamedChildCount()); i++ {
		member := objectType.NamedChild(i)
		if member == nil {
			continue
		}
		kind := "property"
		if member.Type() == "method_signature" || member.Type() == "call_signature" {
			kind = "method"
		}
		name := tsNodeName(member, content)
		candidate, ok := tsMemberCandidate(member, name, kind, receiver, exported)
		if ok {
			out = append(out, candidate)
		}
	}
	return out
}

func tsNodeName(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	name := node.ChildByFieldName("name")
	if name != nil && tsSimpleNameNode(name) {
		return strings.Trim(name.Content(content), "'\"`")
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && tsSimpleNameNode(child) {
			return strings.Trim(child.Content(content), "'\"`")
		}
	}
	return ""
}

func tsSimpleNameNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type() {
	case "identifier", "type_identifier", "property_identifier", "private_property_identifier", "string":
		return true
	default:
		return false
	}
}

func tsCallableValue(node *sitter.Node, depth int) bool {
	if node == nil || depth > 4 {
		return false
	}
	switch node.Type() {
	case "arrow_function", "function_expression":
		return true
	case "parenthesized_expression", "as_expression", "satisfies_expression":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if tsCallableValue(node.NamedChild(i), depth+1) {
				return true
			}
		}
	case "call_expression":
		args := node.ChildByFieldName("arguments")
		if args == nil {
			return false
		}
		for i := 0; i < int(args.NamedChildCount()); i++ {
			if tsCallableValue(args.NamedChild(i), depth+1) {
				return true
			}
		}
	}
	return false
}

func tsFileScopeVariable(node *sitter.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Type() {
		case "lexical_declaration", "variable_declaration", "export_statement":
			parent = parent.Parent()
		case "program":
			return true
		default:
			return false
		}
	}
	return false
}

func tsVariableKind(node *sitter.Node) string {
	parent := node.Parent()
	if parent == nil {
		return "variable"
	}
	for i := 0; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if child != nil && child.Type() == "const" {
			return "constant"
		}
	}
	return "variable"
}

func tsExported(node *sitter.Node) bool {
	current := node.Parent()
	for current != nil {
		if current.Type() == "export_statement" {
			return true
		}
		if current.Type() == "program" {
			return false
		}
		current = current.Parent()
	}
	return false
}

func tsExportedContainer(node *sitter.Node) bool {
	current := node.Parent()
	for current != nil {
		switch current.Type() {
		case "class_declaration", "abstract_class_declaration", "interface_declaration":
			return tsExported(current)
		case "program":
			return false
		}
		current = current.Parent()
	}
	return false
}

func tsPrivate(node *sitter.Node, content []byte) bool {
	name := node.ChildByFieldName("name")
	if name != nil && name.Type() == "private_property_identifier" {
		return true
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "accessibility_modifier" && child.Content(content) == "private" {
			return true
		}
	}
	return false
}

func tsEnclosingClassName(node *sitter.Node, content []byte) string {
	current := node.Parent()
	for current != nil {
		switch current.Type() {
		case "class_declaration", "abstract_class_declaration":
			return tsNodeName(current, content)
		case "program":
			return ""
		}
		current = current.Parent()
	}
	return ""
}

func tsObjectNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	return node.Type() == "object" || node.Type() == "object_expression"
}

func tsSnapshot(relPath string, content []byte, candidate tsSymbolCandidate) (SymbolSnapshot, bool) {
	if candidate.name == "" || candidate.kind == "" || candidate.body == nil {
		return SymbolSnapshot{}, false
	}
	start := candidate.body.StartByte()
	end := candidate.body.EndByte()
	if end <= start || int(end) > len(content) {
		return SymbolSnapshot{}, false
	}
	body := content[start:end]
	hash := sha256.Sum256(body)
	qualifiedName := qualifiedSymbolName(candidate.receiver, candidate.name)
	return SymbolSnapshot{
		FilePath:      relPath,
		SymbolName:    candidate.name,
		SymbolKind:    candidate.kind,
		QualifiedName: qualifiedName,
		SignatureHash: signatureHashFromDeclaration(candidate.kind, qualifiedName, body),
		Line:          int(candidate.body.StartPoint().Row) + 1,
		EndLine:       int(candidate.body.EndPoint().Row) + 1,
		Hash:          hex.EncodeToString(hash[:]),
		StartByte:     int(start),
		EndByte:       int(end),
		Receiver:      candidate.receiver,
		Exported:      candidate.exported,
	}, true
}

func dedupeTSSnapshots(snapshots []SymbolSnapshot) []SymbolSnapshot {
	seen := make(map[string]bool, len(snapshots))
	out := make([]SymbolSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%d", snapshot.SymbolKind, snapshot.Receiver, snapshot.SymbolName, snapshot.StartByte)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, snapshot)
	}
	return out
}
