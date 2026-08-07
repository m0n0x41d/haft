package codebase

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

type tsRelationSite struct {
	name string
	kind EdgeKind
	line int
}

func extractTSRelationSites(root *sitter.Node, content []byte) []tsRelationSite {
	sites := make([]tsRelationSite, 0)
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		switch node.Type() {
		case "new_expression":
			if constructor := tsNewExpressionConstructor(node); constructor != nil {
				sites = append(sites, tsRelationSite{
					name: constructor.Content(content),
					kind: EdgeInstantiates,
					line: int(constructor.StartPoint().Row) + 1,
				})
			}
		case "type_identifier":
			sites = append(sites, tsRelationSite{
				name: node.Content(content),
				kind: EdgeTypeReference,
				line: int(node.StartPoint().Row) + 1,
			})
		case "identifier":
			if tsValueReferenceIdentifier(node) {
				sites = append(sites, tsRelationSite{
					name: node.Content(content),
					kind: EdgeValueReference,
					line: int(node.StartPoint().Row) + 1,
				})
			}
		}
		for index := 0; index < int(node.NamedChildCount()); index++ {
			walk(node.NamedChild(index))
		}
	}
	walk(root)
	return sites
}

func tsNewExpressionConstructor(node *sitter.Node) *sitter.Node {
	if constructor := node.ChildByFieldName("constructor"); constructor != nil {
		return tsBareIdentifier(constructor)
	}
	if node.NamedChildCount() == 0 {
		return nil
	}
	return tsBareIdentifier(node.NamedChild(0))
}

func tsBareIdentifier(node *sitter.Node) *sitter.Node {
	if node == nil || node.Type() != "identifier" {
		return nil
	}
	return node
}

func tsValueReferenceIdentifier(node *sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	if tsNodeIsField(parent, "name", node) || tsNodeIsField(parent, "property", node) || tsNodeIsField(parent, "label", node) {
		return false
	}
	if tsNodeIsField(parent, "function", node) || tsNodeIsField(parent, "constructor", node) {
		return false
	}
	switch parent.Type() {
	case "import_specifier", "namespace_import", "import_clause", "export_specifier",
		"required_parameter", "optional_parameter", "rest_pattern", "variable_declarator",
		"function_declaration", "function_signature", "class_declaration", "interface_declaration",
		"type_alias_declaration", "enum_declaration", "method_definition", "new_expression":
		return false
	}
	return !tsInsideTypeSyntax(node)
}

func tsNodeIsField(parent *sitter.Node, field string, child *sitter.Node) bool {
	fieldNode := parent.ChildByFieldName(field)
	if fieldNode == nil {
		return false
	}
	return fieldNode.StartByte() == child.StartByte() && fieldNode.EndByte() == child.EndByte()
}

func tsInsideTypeSyntax(node *sitter.Node) bool {
	current := node.Parent()
	for current != nil {
		typeName := current.Type()
		if strings.Contains(typeName, "type") || strings.Contains(typeName, "annotation") {
			return true
		}
		switch typeName {
		case "statement_block", "program", "call_expression", "subscript_expression", "member_expression":
			return false
		}
		current = current.Parent()
	}
	return false
}

func resolveTSReferenceOutcomes(
	relPath string,
	fileSymbols []CodeSymbol,
	sites []tsRelationSite,
	imports tsImports,
	lookup func(string) []CodeSymbol,
	sourceHash string,
) []EdgeResolution {
	outcomes := make([]EdgeResolution, 0)
	seen := make(map[string]bool)
	for _, site := range sites {
		caller := enclosingSymbol(fileSymbols, site.line)
		if caller == nil {
			continue
		}
		candidates, origin, candidate := resolveTSRelationCandidates(site, fileSymbols, imports, lookup)
		if !candidate {
			continue
		}
		if len(candidates) == 0 {
			outcomes = append(outcomes, UnresolvedEdge{
				SourceID:           caller.ID,
				Kind:               site.kind,
				FilePath:           relPath,
				Line:               site.line,
				Reason:             tsReferenceMissingReason(site.name, imports),
				Origin:             origin,
				ResolverVersion:    tsResolverVersion,
				SourceSnapshotHash: sourceHash,
			})
			continue
		}
		if len(candidates) > 1 {
			outcomes = append(outcomes, AmbiguousEdge{
				SourceID:           caller.ID,
				Kind:               site.kind,
				FilePath:           relPath,
				Line:               site.line,
				Reason:             ResolutionReasonMultipleCandidates,
				CandidateIDs:       codeSymbolIDs(candidates),
				Origin:             origin,
				ResolverVersion:    tsResolverVersion,
				SourceSnapshotHash: sourceHash,
			})
			continue
		}
		target := candidates[0]
		if target.ID == caller.ID {
			continue
		}
		key := caller.ID + "\x00" + target.ID + "\x00" + string(site.kind)
		if seen[key] {
			continue
		}
		seen[key] = true
		outcomes = append(outcomes, ResolvedEdge{Edge: CodeEdge{
			SrcID:              caller.ID,
			DstID:              target.ID,
			Kind:               site.kind,
			FilePath:           relPath,
			Line:               site.line,
			Provenance:         ProvenanceStatic,
			Origin:             origin,
			ResolutionMethod:   ResolutionMethodExactSymbol,
			Confidence:         ConfidenceExact,
			ResolverVersion:    tsResolverVersion,
			SourceSnapshotHash: sourceHash,
		}})
	}
	return outcomes
}

func resolveTSRelationCandidates(
	site tsRelationSite,
	fileSymbols []CodeSymbol,
	imports tsImports,
	lookup func(string) []CodeSymbol,
) ([]CodeSymbol, EdgeOrigin, bool) {
	predicate := tsValueSymbol
	origin := EdgeOriginASTValueReference
	if site.kind == EdgeTypeReference {
		predicate = tsTypeSymbol
		origin = EdgeOriginASTTypeReference
	}
	if site.kind == EdgeInstantiates {
		predicate = tsConstructableSymbol
		origin = EdgeOriginASTNew
	}
	local := make([]CodeSymbol, 0)
	for _, symbol := range fileSymbols {
		if symbol.Receiver == "" && symbol.Name == site.name && predicate(symbol) {
			local = append(local, symbol)
		}
	}
	binding, imported := imports.names[site.name]
	if len(local) > 0 && imported {
		importedCandidates := tsImportedGenericCandidates(binding, imports, lookup, predicate)
		return dedupeCodeSymbols(append(local, importedCandidates...)), EdgeOriginNamedImport, true
	}
	if len(local) > 0 {
		return local, origin, true
	}
	if imported {
		return tsImportedGenericCandidates(binding, imports, lookup, predicate), EdgeOriginNamedImport, true
	}
	return nil, origin, false
}

func tsImportedGenericCandidates(
	binding tsNameImport,
	imports tsImports,
	lookup func(string) []CodeSymbol,
	predicate func(CodeSymbol) bool,
) []CodeSymbol {
	targets := make([]tsExportTarget, 0)
	for _, base := range binding.bases {
		if imports.model == nil {
			targets = append(targets, tsExportTarget{fileBase: base, symbolName: binding.orig})
			continue
		}
		targets = append(targets, imports.model.ResolveExport(base, binding.orig)...)
	}
	candidates := make([]CodeSymbol, 0)
	for _, target := range targets {
		for _, symbol := range lookup(target.symbolName) {
			if predicate(symbol) && tsFileMatchesBase(symbol.FilePath, target.fileBase) {
				candidates = append(candidates, symbol)
			}
		}
	}
	return dedupeCodeSymbols(candidates)
}

func tsValueSymbol(symbol CodeSymbol) bool {
	switch symbol.Kind {
	case "func", "class", "constant", "variable", "enum":
		return true
	default:
		return false
	}
}

func tsTypeSymbol(symbol CodeSymbol) bool {
	switch symbol.Kind {
	case "class", "interface", "type_alias", "enum":
		return true
	default:
		return false
	}
}

func tsConstructableSymbol(symbol CodeSymbol) bool { return symbol.Kind == "class" }

func tsReferenceMissingReason(name string, imports tsImports) ResolutionReason {
	if binding, imported := imports.names[name]; imported && binding.external {
		return ResolutionReasonExternalDependency
	}
	return ResolutionReasonNoCandidate
}

func codeSymbolIDs(symbols []CodeSymbol) []string {
	ids := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		ids = append(ids, symbol.ID)
	}
	return ids
}
