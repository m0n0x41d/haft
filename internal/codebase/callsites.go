package codebase

import (
	gocontext "context"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// CallSite is one extracted call expression: the called name, an optional
// qualifier (the selector operand — a package alias or receiver), and the line.
// The enclosing caller symbol is resolved later by line containment, not here.
type CallSite struct {
	Callee    string
	Qualifier string // "" = unqualified (intra-package candidate); non-"" = qualified (P1b)
	Line      int    // 1-based
	Shadowed  bool   // lexical binding hides an imported/global name at this call site
}

// ExtractCallSites walks a Go file's call expressions. Go only for P1a — other
// languages return nil (node extraction still works for them; edges don't yet).
// Pure relative to the file content; no DB.
func ExtractCallSites(projectRoot, relPath string) ([]CallSite, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return ExtractCallSitesFromSource(source)
}

// ExtractCallSitesFromSource is the tree-sitter core. It cannot receive raw
// filesystem bytes or silently apply its own resource policy.
func ExtractCallSitesFromSource(
	source AdmittedSource,
) ([]CallSite, error) {
	relPath := source.Path().String()
	ext := filepath.Ext(relPath)
	langInfo, ok := languages[ext]
	if !ok || langInfo.name != "go" {
		return nil, nil
	}
	content := source.bytes()

	parser := sitter.NewParser()
	parser.SetLanguage(langInfo.lang)
	tree, err := parser.ParseCtx(gocontext.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", relPath, err)
	}
	defer tree.Close()

	q, err := sitter.NewQuery([]byte(`(call_expression) @call`), langInfo.lang)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	var sites []CallSite
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, capture := range m.Captures {
			call := capture.Node
			fn := call.ChildByFieldName("function")
			if fn == nil {
				continue
			}
			var callee, qualifier string
			switch fn.Type() {
			case "identifier":
				callee = fn.Content(content)
			case "selector_expression":
				field := fn.ChildByFieldName("field")
				if field == nil {
					continue
				}
				callee = field.Content(content)
				if operand := fn.ChildByFieldName("operand"); operand != nil {
					qualifier = operand.Content(content)
				}
			default:
				continue
			}
			if callee == "" {
				continue
			}
			sites = append(sites, CallSite{
				Callee:    callee,
				Qualifier: qualifier,
				Line:      int(call.StartPoint().Row) + 1,
			})
		}
	}
	return sites, nil
}

// ResolveIntraPackageCallEdges resolves UNQUALIFIED call sites in a file to
// package-level definitions, against the package's symbol set. The over-
// resolution guard is exactly-1-or-drop: a callee with zero or multiple package
// candidates yields NO edge (an unresolved call is honest; a fanned-out wrong
// edge is corrosive). Qualified calls (receiver/package selectors) are left for
// P1b. Pure — symbols + call sites in, edges out.
func ResolveIntraPackageCallEdges(filePath string, fileSymbols, pkgSymbols []CodeSymbol, callSites []CallSite) []CodeEdge {
	byName := map[string][]CodeSymbol{}
	for _, s := range pkgSymbols {
		if s.Kind == "func" || s.Kind == "method" {
			byName[s.Name] = append(byName[s.Name], s)
		}
	}

	seen := map[string]bool{}
	var edges []CodeEdge
	for _, cs := range callSites {
		if cs.Qualifier != "" {
			continue // qualified → P1b (selector resolution)
		}
		caller := enclosingSymbol(fileSymbols, cs.Line)
		if caller == nil {
			continue
		}
		cands := byName[cs.Callee]
		if len(cands) != 1 {
			continue // 0 = undefined/builtin; >1 = ambiguous — DROP, never fan out
		}
		dst := cands[0]
		if dst.ID == caller.ID {
			continue // skip trivial self-recursion edges
		}
		key := caller.ID + "->" + dst.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, CodeEdge{
			SrcID:      caller.ID,
			DstID:      dst.ID,
			Kind:       EdgeCall,
			FilePath:   filePath,
			Line:       cs.Line,
			Provenance: ProvenanceStatic,
		})
	}
	return edges
}

// ResolveConcreteMethodCallEdges resolves QUALIFIED calls whose receiver is a
// CONCRETE-typed variable or field — `store.Get(...)`, `s.scanner.ScanEdges(...)`
// — to the method's definition node, including cross-package (the method lives
// where its type is declared). This is the static counterpart to interface
// dispatch: the receiver type is resolved (via vars + struct-field facts), then
// the method node is found by (receiver, name) with the same exactly-1-or-drop
// guard — a bare-receiver-name collision across packages (two `Store` types)
// drops rather than emitting a wrong edge. Interface receivers naturally yield
// no match here (interfaces have no method-body nodes) and are left to dispatch.
func ResolveConcreteMethodCallEdges(filePath string, fileSymbols []CodeSymbol, callSites []CallSite, signatures map[int]map[string]string, facts TypeFacts, lookup func(name string) []CodeSymbol) []CodeEdge {
	seen := map[string]bool{}
	var edges []CodeEdge
	for _, cs := range callSites {
		if cs.Qualifier == "" {
			continue // unqualified → P1a
		}
		caller := enclosingSymbol(fileSymbols, cs.Line)
		if caller == nil {
			continue
		}
		vars := signatures[caller.StartLine]
		if vars == nil {
			continue
		}
		recvType := resolveQualifierType(cs.Qualifier, vars, facts)
		if recvType == "" {
			continue // qualifier type unknown (package alias / unresolved field) — drop
		}
		var cands []CodeSymbol
		for _, s := range lookup(cs.Callee) {
			if s.Kind == "method" && s.Receiver == recvType {
				cands = append(cands, s)
			}
		}
		if len(cands) != 1 {
			continue // 0 = not found; >1 = receiver-name collision — DROP, never fan out
		}
		dst := cands[0]
		if dst.ID == caller.ID {
			continue
		}
		key := caller.ID + "->" + dst.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, CodeEdge{
			SrcID:      caller.ID,
			DstID:      dst.ID,
			Kind:       EdgeCall,
			FilePath:   filePath,
			Line:       cs.Line,
			Provenance: ProvenanceStatic,
		})
	}
	return edges
}

// enclosingSymbol returns the innermost func/method whose body covers the line.
func enclosingSymbol(syms []CodeSymbol, line int) *CodeSymbol {
	var best *CodeSymbol
	for i := range syms {
		s := &syms[i]
		if s.Kind != "func" && s.Kind != "method" {
			continue
		}
		if line >= s.StartLine && line <= s.EndLine {
			if best == nil || (s.EndLine-s.StartLine) < (best.EndLine-best.StartLine) {
				best = s
			}
		}
	}
	return best
}

// GoImport is one resolved import of a Go file: the alias the source uses to
// qualify calls, the import path, and the LOCAL directory it maps to (relative
// to projectRoot) — empty for external (non-module) dependencies, which never
// resolve to a node.
type GoImport struct {
	Alias      string
	ImportPath string
	LocalDir   string
}

// ExtractGoImports returns the import aliases of a Go file mapped to local
// directories, using the module path from go.mod. The alias is the explicit
// rename or, by convention, the import path's last segment (haft's packages
// follow dir==package, so this resolves correctly; a mismatch simply fails to
// resolve — an honest miss, not a wrong edge). External imports get LocalDir "".
func ExtractGoImports(projectRoot, relPath string) ([]GoImport, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return ExtractGoImportsFromSource(projectRoot, source)
}

// ExtractGoImportsFromSource parses imports from the exact admitted bytes while
// retaining project-root metadata only for local-module path resolution.
func ExtractGoImportsFromSource(
	projectRoot string,
	source AdmittedSource,
) ([]GoImport, error) {
	relPath := source.Path().String()
	absPath := filepath.Join(projectRoot, filepath.FromSlash(relPath))
	fset := token.NewFileSet()
	f, err := parser.ParseFile(
		fset,
		relPath,
		source.bytes(),
		parser.ImportsOnly,
	)
	if err != nil {
		return nil, nil // skip unparseable
	}

	goModDir := findGoModDir(filepath.Dir(absPath), projectRoot)
	modulePath, goModPrefix := "", ""
	if goModDir != "" {
		modulePath = readGoModulePathFromDir(goModDir)
		goModPrefix, _ = filepath.Rel(projectRoot, goModDir)
		if goModPrefix == "." {
			goModPrefix = ""
		}
	}

	var imports []GoImport
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			segs := strings.Split(importPath, "/")
			alias = segs[len(segs)-1]
		}
		if alias == "_" || alias == "." {
			continue // blank/dot imports never qualify a call
		}
		localDir := ""
		if modulePath != "" && (importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")) {
			lp := strings.TrimPrefix(strings.TrimPrefix(importPath, modulePath), "/")
			if goModPrefix != "" {
				lp = filepath.Join(goModPrefix, lp)
			}
			localDir = lp
		}
		imports = append(imports, GoImport{Alias: alias, ImportPath: importPath, LocalDir: localDir})
	}
	return imports, nil
}

// ResolveCrossFileCallEdges resolves QUALIFIED call sites (pkg.Func) to exported
// package-level functions in the imported local package. Chain: qualifier →
// import alias → local dir → exported func node. Same exactly-1-or-drop guard:
// an external import, an unknown qualifier (a receiver variable, not a package),
// or an unresolved name yields NO edge. lookup is the by-name node accessor
// (injected so the resolver stays pure relative to the store).
func ResolveCrossFileCallEdges(filePath string, fileSymbols []CodeSymbol, callSites []CallSite, imports []GoImport, lookup func(name string) []CodeSymbol) []CodeEdge {
	aliasDir := map[string]string{}
	for _, imp := range imports {
		if imp.LocalDir != "" {
			aliasDir[imp.Alias] = imp.LocalDir
		}
	}

	seen := map[string]bool{}
	var edges []CodeEdge
	for _, cs := range callSites {
		if cs.Qualifier == "" {
			continue // unqualified → P1a
		}
		dir, ok := aliasDir[cs.Qualifier]
		if !ok {
			continue // not a local package alias (external dep, or a receiver var) — drop
		}
		caller := enclosingSymbol(fileSymbols, cs.Line)
		if caller == nil {
			continue
		}
		var cands []CodeSymbol
		for _, s := range lookup(cs.Callee) {
			if s.Exported && s.Kind == "func" && filepath.Dir(s.FilePath) == dir {
				cands = append(cands, s)
			}
		}
		if len(cands) != 1 {
			continue // exactly-1-or-drop
		}
		dst := cands[0]
		key := caller.ID + "->" + dst.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		edges = append(edges, CodeEdge{
			SrcID:      caller.ID,
			DstID:      dst.ID,
			Kind:       EdgeCall,
			FilePath:   filePath,
			Line:       cs.Line,
			Provenance: ProvenanceStatic,
		})
	}
	return edges
}
