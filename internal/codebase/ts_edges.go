package codebase

import (
	"context"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// heritageRel is one class/interface and one base it inherits, with whether the
// relation is `extends` (class extends class, interface extends interface) or
// `implements` (class implements interface).
type heritageRel struct {
	subType  string
	baseName string
	extends  bool
}

// tsGrammarForExt picks the exact tree-sitter grammar for each JS/TS extension.
// TSX must not use the plain TypeScript grammar: JSX syntax changes the tree and
// silently loses declarations/calls when parsed with the wrong grammar.
func tsGrammarForExt(ext string) *sitter.Language {
	switch strings.ToLower(ext) {
	case ".ts", ".mts", ".cts":
		return typescript.GetLanguage()
	case ".tsx":
		return tsx.GetLanguage()
	case ".js", ".jsx", ".mjs", ".cjs":
		return javascript.GetLanguage()
	}
	return nil
}

// extractTSHeritage parses a JS/TS file and returns each class/interface heritage
// relation (extends / implements), reading the explicit clauses from the AST.
// Pure relative to file content. JS has only class `extends`; TS adds class
// `implements` and interface `extends`.
func extractTSHeritage(root *sitter.Node, content []byte) []heritageRel {
	var out []heritageRel
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		switch n.Type() {
		case "class_declaration":
			if name := declName(n, content); name != "" {
				for _, heritage := range childrenOfType(n, "class_heritage") {
					for _, base := range identChildren(firstChildOfType(heritage, "extends_clause"), content) {
						out = append(out, heritageRel{name, base, true})
					}
					for _, base := range identChildren(firstChildOfType(heritage, "implements_clause"), content) {
						out = append(out, heritageRel{name, base, false})
					}
				}
			}
		case "interface_declaration":
			if name := declName(n, content); name != "" {
				for _, base := range identChildren(firstChildOfType(n, "extends_type_clause"), content) {
					out = append(out, heritageRel{name, base, true})
				}
			}
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			walk(n.NamedChild(i))
		}
	}
	walk(root)
	return out
}

// declName returns the first direct identifier / type_identifier child — the
// class or interface name (heritage clauses come after, so they are not hit).
func declName(n *sitter.Node, content []byte) string {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if c.Type() == "type_identifier" || c.Type() == "identifier" {
			return c.Content(content)
		}
	}
	return ""
}

func firstChildOfType(n *sitter.Node, typ string) *sitter.Node {
	if n == nil {
		return nil
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if c := n.NamedChild(i); c.Type() == typ {
			return c
		}
	}
	return nil
}

func childrenOfType(n *sitter.Node, typ string) []*sitter.Node {
	var out []*sitter.Node
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if c := n.NamedChild(i); c.Type() == typ {
			out = append(out, c)
		}
	}
	return out
}

// identChildren returns the base names directly under a heritage clause:
// identifier / type_identifier (Base), and the head of a generic_type (Base<T>).
func identChildren(clause *sitter.Node, content []byte) []string {
	if clause == nil {
		return nil
	}
	var out []string
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		c := clause.NamedChild(i)
		switch c.Type() {
		case "identifier", "type_identifier":
			out = append(out, c.Content(content))
		case "generic_type":
			if name := declName(c, content); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// ResolveFileEdges makes JSTSLang an EdgeResolver. It emits two edge families:
//   - `extends` / `implements` edges from the explicit JS/TS heritage clauses,
//     resolved directory-locally (heuristic provenance — no import analysis);
//   - `call` edges, resolved through the project module/export model (file-local
//     defs, default/named/namespace imports, barrels, aliases, and workspaces)
//     with the same
//     exactly-1-or-drop discipline (static provenance).
//
// A base or call that does not resolve to exactly one symbol is retained as an
// ambiguous/unresolved diagnostic, never guessed. Instance-method calls remain
// unresolved until receiver facts prove their target.
func (j *JSTSLang) ResolveFileEdges(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]CodeEdge, error) {
	outcomes, err := j.ResolveFileEdgeOutcomes(ctx, projectRoot, relPath, symbols)
	if err != nil {
		return nil, err
	}
	edges, _ := PartitionEdgeResolutions(outcomes)
	return edges, nil
}

const tsResolverVersion = "typescript-v2"

// ResolveFileEdgeOutcomes is the authority-bearing TypeScript resolver path.
// Resolved relations become edges; ambiguous and unresolved call sites remain
// diagnostics. The legacy ResolveFileEdges method is only its edge projection.
func (j *JSTSLang) ResolveFileEdgeOutcomes(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]EdgeResolution, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return j.ResolveAdmittedFileEdgeOutcomes(
		ctx,
		projectRoot,
		source,
		symbols,
	)
}

func (j *JSTSLang) ResolveFileEdgeOutcomesWithProjectSnapshot(
	ctx context.Context,
	projectRoot string,
	relPath string,
	symbols SymbolView,
	snapshot *projectIndexSnapshot,
) ([]EdgeResolution, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return j.ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
		ctx,
		projectRoot,
		source,
		symbols,
		snapshot,
	)
}

func (j *JSTSLang) ResolveAdmittedFileEdgeOutcomes(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
) ([]EdgeResolution, error) {
	return j.resolveAdmittedFileEdgeOutcomes(
		ctx,
		projectRoot,
		source,
		symbols,
		nil,
	)
}

func (j *JSTSLang) ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
	snapshot *projectIndexSnapshot,
) ([]EdgeResolution, error) {
	return j.resolveAdmittedFileEdgeOutcomes(
		ctx,
		projectRoot,
		source,
		symbols,
		snapshot.typescript,
	)
}

func (j *JSTSLang) resolveAdmittedFileEdgeOutcomes(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
	snapshot *tsProjectSnapshot,
) ([]EdgeResolution, error) {
	relPath := source.Path().String()
	lang := tsGrammarForExt(filepath.Ext(relPath))
	if lang == nil {
		return nil, nil
	}
	content := source.bytes()
	sourceHash := "sha256:" + source.Digest()
	return resolveTSContentEdgeOutcomes(ctx, projectRoot, relPath, content, sourceHash, lang, symbols, snapshot)
}

func resolveTSContentEdgeOutcomes(
	ctx context.Context,
	projectRoot string,
	relPath string,
	content []byte,
	sourceHash string,
	lang *sitter.Language,
	symbols SymbolView,
	snapshot *tsProjectSnapshot,
) ([]EdgeResolution, error) {
	// Parse the file ONCE; every extractor reads the shared tree.
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, nil
	}
	defer tree.Close()
	root := tree.RootNode()

	fileSyms, err := symbols.GetByFile(ctx, relPath)
	if err != nil {
		return nil, err
	}
	pkgSyms, err := symbols.GetByDir(ctx, filepath.Dir(relPath))
	if err != nil {
		return nil, err
	}

	lookup := func(name string) []CodeSymbol {
		s, _ := symbols.GetByName(ctx, name)
		return s
	}
	projectResolution := loadTSProjectResolution(projectRoot)
	var projectModel *tsProjectModel
	if snapshot != nil {
		projectResolution = snapshot.resolution
		projectModel = snapshot.model
	}
	if projectModel == nil {
		projectModel, err = loadTSProjectModel(
			projectRoot,
			projectResolution,
		)
		if err != nil {
			return nil, err
		}
	}
	imports := extractTSImports(root, content, filepath.Dir(relPath), projectResolution)
	imports.model = projectModel

	var outcomes []EdgeResolution
	heritage := tsHeritageEdges(root, content, relPath, fileSyms, imports, lookup)
	outcomes = append(outcomes, resolvedTSOutcomes(heritage, sourceHash, EdgeOriginHeritage)...)

	callAnalysis := extractTSCallAnalysis(root, content, lang)
	callOutcomes := resolveTSCallOutcomes(
		relPath,
		fileSyms,
		pkgSyms,
		callAnalysis.calls,
		callAnalysis.callbacks,
		callAnalysis.receiverTypes,
		imports,
		lookup,
		sourceHash,
	)
	callEdges, _ := PartitionEdgeResolutions(callOutcomes)
	outcomes = append(outcomes, callOutcomes...)
	referenceSites := extractTSRelationSites(root, content)
	outcomes = append(outcomes, resolveTSReferenceOutcomes(relPath, fileSyms, referenceSites, imports, lookup, sourceHash)...)

	// Intra-file EventEmitter dispatch: pair .on("e", h) with .emit("e").
	emitter := synthesizeEmitterEdges(
		relPath,
		fileSyms,
		callAnalysis.emitterRegisters,
		callAnalysis.emitterDispatches,
		edgePairs(callEdges),
	)
	outcomes = append(outcomes, resolvedTSOutcomes(emitter, sourceHash, EdgeOriginEmitterPair)...)

	return dedupeEdgeOutcomes(outcomes), nil
}

func resolvedTSOutcomes(edges []CodeEdge, sourceHash string, origin EdgeOrigin) []EdgeResolution {
	outcomes := make([]EdgeResolution, 0, len(edges))
	for _, edge := range edges {
		edge.Origin = origin
		edge.ResolverVersion = tsResolverVersion
		edge.SourceSnapshotHash = sourceHash
		if edge.Provenance == ProvenanceHeuristic {
			edge.ResolutionMethod = ResolutionMethodHeuristic
			edge.Confidence = ConfidenceLow
		} else {
			edge.ResolutionMethod = ResolutionMethodExactSymbol
			edge.Confidence = ConfidenceHigh
		}
		outcomes = append(outcomes, ResolvedEdge{Edge: edge})
	}
	return outcomes
}

func dedupeEdgeOutcomes(outcomes []EdgeResolution) []EdgeResolution {
	seenEdges := map[string]bool{}
	deduped := make([]EdgeResolution, 0, len(outcomes))
	for _, outcome := range outcomes {
		resolved, ok := outcome.(ResolvedEdge)
		if !ok {
			deduped = append(deduped, outcome)
			continue
		}
		key := resolved.Edge.SrcID + "\x00" + resolved.Edge.DstID + "\x00" + string(resolved.Edge.Kind)
		if seenEdges[key] {
			continue
		}
		seenEdges[key] = true
		deduped = append(deduped, outcome)
	}
	return deduped
}

// tsHeritageEdges resolves `extends` / `implements` edges from explicit heritage
// clauses with IMPORT AWARENESS: a base resolves to a same-file class/interface
// or to an explicitly imported one, never to an unimported same-named type in a
// sibling module. A base that resolves to neither (external / unresolved) or to
// both (a shadowed redefinition) is dropped — never guessed. Pure.
func tsHeritageEdges(root *sitter.Node, content []byte, relPath string, fileSyms []CodeSymbol, imports tsImports, lookup func(string) []CodeSymbol) []CodeEdge {
	rels := extractTSHeritage(root, content)
	if len(rels) == 0 {
		return nil
	}

	fileByName := map[string]CodeSymbol{}
	for _, s := range fileSyms {
		if s.Kind == "class" || s.Kind == "interface" {
			fileByName[s.Name] = s
		}
	}

	var edges []CodeEdge
	for _, r := range rels {
		sub, ok := fileByName[r.subType]
		if !ok || r.baseName == r.subType {
			continue
		}
		dst := resolveTSBaseType(r.baseName, fileByName, imports, lookup)
		if dst == nil {
			continue
		}
		kind := EdgeImplements
		if r.extends {
			kind = EdgeExtends
		}
		edges = append(edges, CodeEdge{
			SrcID:      sub.ID,
			DstID:      dst.ID,
			Kind:       kind,
			FilePath:   relPath,
			Provenance: ProvenanceHeuristic,
		})
	}
	return edges
}

// resolveTSBaseType resolves a base type name to exactly one class/interface with
// import awareness: an explicit named import is authoritative for the name in this
// file (resolved cross-module), else a same-file type. A name that is BOTH
// imported and locally defined, or NEITHER, is dropped. Without this an unimported
// same-named type in a sibling module would wrongly shadow the imported base.
func resolveTSBaseType(b string, fileByName map[string]CodeSymbol, imports tsImports, lookup func(string) []CodeSymbol) *CodeSymbol {
	bind, imported := imports.names[b]
	local, isLocal := fileByName[b]
	if imported && isLocal {
		return nil // shadowed redefinition — ambiguous, drop
	}
	if imported {
		var cands []CodeSymbol
		targets := make([]tsExportTarget, 0)
		for _, base := range bind.bases {
			if imports.model == nil {
				targets = append(targets, tsExportTarget{fileBase: base, symbolName: bind.orig})
				continue
			}
			targets = append(targets, imports.model.ResolveExport(base, bind.orig)...)
		}
		for _, target := range targets {
			for _, symbol := range lookup(target.symbolName) {
				if (symbol.Kind == "class" || symbol.Kind == "interface") && tsFileMatchesBase(symbol.FilePath, target.fileBase) {
					cands = append(cands, symbol)
				}
			}
		}
		cands = dedupeCodeSymbols(cands)
		if len(cands) != 1 {
			return nil
		}
		return &cands[0]
	}
	if isLocal {
		return &local
	}
	return nil
}
