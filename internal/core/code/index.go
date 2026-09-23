package code

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

var toolchainPattern = regexp.MustCompile(`^go1\.(\d+)(?:\.\d+)?$`)

func diagnostic(code, p, message string) Diagnostic {
	return Diagnostic{Code: code, Path: p, Message: message, Severity: "warning"}
}
func sortedUnique(values []string) []string {
	values = append([]string{}, values...)
	sort.Strings(values)
	result := []string{}
	for _, v := range values {
		if len(result) == 0 || result[len(result)-1] != v {
			result = append(result, v)
		}
	}
	return result
}
func normalizedPath(p string) bool {
	return p != "" && p != "." && path.Clean(p) == p && !strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\\x00\r\n:")
}

// IndexFromFiles performs no IO. Configuration and every byte read by the build
// selector/parser come from the arguments. Input buffers are never retained.
func IndexFromFiles(input map[string][]byte, config Config) (Index, error) {
	return indexFromFiles(input, config, nil, &RefreshStats{})
}

type RefreshStats struct {
	ParsedFiles int `json:"parsed_files"`
	ReusedFiles int `json:"reused_files"`
}

// RefreshFromFiles reuses only unchanged parsed file bytes under the same parser
// version and build configuration. Package/import/reverse closure is rebuilt
// from the new admitted file set, including deletions and ignore changes.
func RefreshFromFiles(previous Index, input map[string][]byte, config Config) (Index, RefreshStats, error) {
	stats := RefreshStats{}
	index, err := indexFromFiles(input, config, &previous, &stats)
	return index, stats, err
}
func indexFromFiles(input map[string][]byte, config Config, previous *Index, stats *RefreshStats) (Index, error) {
	index := Index{Version: Version, Complete: true}
	match := toolchainPattern.FindStringSubmatch(config.Toolchain)
	if config.GOOS == "" || config.GOARCH == "" || match == nil {
		return index, fmt.Errorf("explicit GOOS, GOARCH and release toolchain (go1.N.P) required")
	}
	config.BuildTags = sortedUnique(config.BuildTags)
	config.ToolTags = sortedUnique(config.ToolTags)
	config.IgnorePatterns = append([]string{}, config.IgnorePatterns...)
	index.Config = config
	cachedFiles := map[string]File{}
	if previous != nil && previous.Version == Version {
		before, _ := json.Marshal(previous.Config)
		after, _ := json.Marshal(config)
		if bytes.Equal(before, after) {
			for _, file := range previous.Files {
				cachedFiles[file.Path] = file
			}
		}
	}
	files := map[string][]byte{}
	names := []string{}
	for p, raw := range input {
		if !normalizedPath(p) {
			return index, fmt.Errorf("invalid captured path %q", p)
		}
		if uint64(len(raw)) > uint64(^uint32(0)) {
			return index, fmt.Errorf("source exceeds tree-sitter byte coordinate range: %s", p)
		}
		files[p] = bytes.Clone(raw)
		names = append(names, p)
	}
	sort.Strings(names)
	minor, _ := strconv.Atoi(match[1])
	releaseTags := []string{}
	for n := 1; n <= minor; n++ {
		releaseTags = append(releaseTags, fmt.Sprintf("go1.%d", n))
	}
	selection := build.Context{GOOS: config.GOOS, GOARCH: config.GOARCH, Compiler: "gc", CgoEnabled: config.CGOEnabled, BuildTags: config.BuildTags, ToolTags: config.ToolTags, ReleaseTags: releaseTags}
	selection.OpenFile = func(p string) (io.ReadCloser, error) {
		raw, ok := files[path.Clean(p)]
		if !ok {
			return nil, fmt.Errorf("source not captured: %s", p)
		}
		return io.NopCloser(bytes.NewReader(raw)), nil
	}
	ignore := newIgnore(files, config.IgnorePatterns)
	module := modulePath(files["go.mod"])
	if module == "" {
		index.Diagnostics = append(index.Diagnostics, diagnostic("module_basis_absent", "go.mod", "Local import resolution lacks a captured module declaration"))
	}
	if _, present := files["go.work"]; present {
		index.Diagnostics = append(index.Diagnostics, diagnostic("workspace_closure_partial", "go.work", "Multi-module workspace resolution requires separately indexed roots"))
	}
	packageMap := map[string]*Package{}
	for _, p := range names {
		raw := files[p]
		f := File{Path: p, Digest: carrier.Digest(raw), Raw: raw, Status: "captured"}
		if ignore.ignored(p, false) {
			f.Status = "excluded"
			f.Reason = "ignore_rule"
			index.Files = append(index.Files, f)
			continue
		}
		if path.Ext(p) != ".go" {
			index.Files = append(index.Files, f)
			continue
		}
		if nestedModule(p, files) {
			f.Status = "unsupported"
			f.Reason = "nested_module"
			index.Complete = false
			index.Diagnostics = append(index.Diagnostics, diagnostic("nested_module", p, "Index this module as a separate root"))
			index.Files = append(index.Files, f)
			continue
		}
		if !config.IncludeTests && strings.HasSuffix(p, "_test.go") {
			f.Status = "excluded"
			f.Reason = "tests_not_selected"
			index.Files = append(index.Files, f)
			continue
		}
		selected, err := selection.MatchFile(path.Dir(p), path.Base(p))
		if err != nil {
			f.Status = "unresolved"
			f.Reason = "build_selection_error"
			index.Complete = false
			index.Diagnostics = append(index.Diagnostics, diagnostic(f.Reason, p, err.Error()))
			index.Files = append(index.Files, f)
			continue
		}
		if !selected {
			f.Status = "excluded"
			f.Reason = "build_constraint"
			index.Files = append(index.Files, f)
			continue
		}
		cached, reuse := cachedFiles[p]
		if reuse && cached.Digest == f.Digest && cached.Status == "indexed" {
			f = cached
			f.Raw = raw
			f.Imports = append([]string{}, cached.Imports...)
			for _, symbol := range previous.Symbols {
				if symbol.Path == p {
					index.Symbols = append(index.Symbols, symbol)
				}
			}
			stats.ReusedFiles++
		} else {
			stats.ParsedFiles++
			fset := token.NewFileSet()
			syntax, err := parser.ParseFile(fset, p, raw, parser.ParseComments|parser.AllErrors)
			if err != nil {
				f.Status = "unresolved"
				f.Reason = "parse_error"
				index.Complete = false
				index.Diagnostics = append(index.Diagnostics, diagnostic(f.Reason, p, err.Error()))
				index.Files = append(index.Files, f)
				continue
			}
			cgo := false
			for _, imp := range syntax.Imports {
				v, e := strconv.Unquote(imp.Path.Value)
				if e != nil {
					continue
				}
				if v == "C" {
					cgo = true
				}
				f.Imports = append(f.Imports, v)
			}
			if cgo && !config.CGOEnabled {
				f.Status = "excluded"
				f.Reason = "cgo_disabled"
				index.Files = append(index.Files, f)
				continue
			}
			f.SyntaxDigest = syntaxDigest(raw, cgo)
			f.Package = syntax.Name.Name
			f.Imports = sortedUnique(f.Imports)
			f.Status = "indexed"
			syms, err := extractSymbols(p, raw, syntax, fset)
			if err != nil {
				f.Status = "unresolved"
				f.Reason = "tree_sitter_error"
				index.Complete = false
				index.Diagnostics = append(index.Diagnostics, diagnostic(f.Reason, p, err.Error()))
			} else {
				index.Symbols = append(index.Symbols, syms...)
			}
		}
		index.Files = append(index.Files, f)
		if f.Status != "indexed" {
			continue
		}
		dir := path.Dir(p)
		id := dir + "::" + f.Package
		pkg := packageMap[id]
		if pkg == nil {
			ip := module
			if dir != "." {
				ip = strings.TrimSuffix(module, "/") + "/" + dir
			}
			if module == "" {
				ip = ""
			}
			pkg = &Package{ID: id, Directory: dir, Name: f.Package, ImportPath: ip}
			packageMap[id] = pkg
		}
		pkg.Files = append(pkg.Files, p)
		pkg.Imports = append(pkg.Imports, f.Imports...)
		for _, imported := range f.Imports {
			if imported == "C" {
				index.Diagnostics = append(index.Diagnostics, diagnostic("cgo_closure_partial", p, "C preamble bytes are pinned; transitive C compiler inputs are not resolved"))
			}
		}
		if strings.Contains(string(raw), "//go:embed") {
			index.Diagnostics = append(index.Diagnostics, diagnostic("embed_closure_partial", p, "Embed directives are pinned; wildcard expansion requires explicit captured assets"))
		}
	}
	byImport := map[string][]*Package{}
	for _, pkg := range packageMap {
		externalTest := strings.HasSuffix(pkg.Name, "_test")
		for _, source := range pkg.Files {
			if !strings.HasSuffix(source, "_test.go") {
				externalTest = false
			}
		}
		if pkg.ImportPath != "" && !externalTest {
			byImport[pkg.ImportPath] = append(byImport[pkg.ImportPath], pkg)
		}
	}
	for importPath, candidates := range byImport {
		if len(candidates) > 1 {
			index.Complete = false
			index.Diagnostics = append(index.Diagnostics, diagnostic("conflicting_package_names", importPath, "One directory contains multiple non-test package names"))
		}
	}
	for _, pkg := range packageMap {
		pkg.Imports = sortedUnique(pkg.Imports)
		for _, imp := range pkg.Imports {
			matches := byImport[imp]
			if len(matches) == 1 {
				pkg.LocalDependencies = append(pkg.LocalDependencies, matches[0].ID)
				matches[0].ReverseImports = append(matches[0].ReverseImports, pkg.ID)
			} else {
				pkg.ExternalDependencies = append(pkg.ExternalDependencies, imp)
				if len(matches) == 0 && module != "" && (imp == module || strings.HasPrefix(imp, module+"/")) {
					index.Diagnostics = append(index.Diagnostics, diagnostic("unresolved_local_import", pkg.Directory, "No selected captured package resolves "+imp))
				}
				if len(matches) > 1 {
					index.Complete = false
					index.Diagnostics = append(index.Diagnostics, diagnostic("ambiguous_package", imp, "Multiple captured package declarations share this import path"))
				}
			}
		}
	}
	for _, pkg := range packageMap {
		pkg.Files = sortedUnique(pkg.Files)
		pkg.LocalDependencies = sortedUnique(pkg.LocalDependencies)
		pkg.ExternalDependencies = sortedUnique(pkg.ExternalDependencies)
		pkg.ReverseImports = sortedUnique(pkg.ReverseImports)
		index.Packages = append(index.Packages, *pkg)
	}
	sort.Slice(index.Packages, func(a, b int) bool { return index.Packages[a].ID < index.Packages[b].ID })
	sort.Slice(index.Symbols, func(a, b int) bool {
		x, y := index.Symbols[a], index.Symbols[b]
		if x.Path != y.Path {
			return x.Path < y.Path
		}
		if x.StartByte != y.StartByte {
			return x.StartByte < y.StartByte
		}
		return x.QualifiedName < y.QualifiedName
	})
	sort.Slice(index.Diagnostics, func(a, b int) bool {
		x, y := index.Diagnostics[a], index.Diagnostics[b]
		return x.Path+"\x00"+x.Code < y.Path+"\x00"+y.Code
	})
	basis := struct {
		Version string
		Config  Config
		Files   []File
	}{Version, config, index.Files}
	b, _ := json.Marshal(basis)
	index.Basis = carrier.Digest(b)
	return index, nil
}

func modulePath(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			s := fields[1]
			if strings.HasPrefix(s, "\"") {
				s, _ = strconv.Unquote(s)
			}
			return s
		}
	}
	return ""
}
func nestedModule(p string, files map[string][]byte) bool {
	for d := path.Dir(p); d != "."; d = path.Dir(d) {
		if _, ok := files[d+"/go.mod"]; ok {
			return true
		}
	}
	return false
}

// Token spellings, inserted semicolons and directives remain significant. cgo
// files conservatively retain all comments because their preamble is executable.
func syntaxDigest(raw []byte, keepComments bool) string {
	fs := token.NewFileSet()
	f := fs.AddFile("", fs.Base(), len(raw))
	var s scanner.Scanner
	s.Init(f, raw, nil, scanner.ScanComments)
	tokens := []string{}
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT && !keepComments && !strings.HasPrefix(lit, "//go:") && !strings.HasPrefix(lit, "// +build") && !strings.HasPrefix(lit, "//line ") && !strings.HasPrefix(lit, "/*line ") {
			continue
		}
		if tok == token.SEMICOLON {
			lit = ";"
		}
		tokens = append(tokens, tok.String(), lit)
	}
	b, _ := json.Marshal(tokens)
	return carrier.Digest(b)
}
func receiverName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return receiverName(x.X)
	case *ast.IndexExpr:
		return receiverName(x.X)
	case *ast.IndexListExpr:
		return receiverName(x.X)
	case *ast.ParenExpr:
		return receiverName(x.X)
	}
	return ""
}

func extractSymbols(p string, raw []byte, parsed *ast.File, fset *token.FileSet) ([]Symbol, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	language := golang.GetLanguage()
	parser.SetLanguage(language)
	tree, err := parser.ParseCtx(context.Background(), nil, raw)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	if tree.RootNode().HasError() {
		return nil, fmt.Errorf("captured source contains tree-sitter error nodes")
	}
	receivers := map[int]string{}
	valueNames := map[int][]string{}
	for _, decl := range parsed.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Recv != nil && len(f.Recv.List) > 0 {
			receivers[fset.Position(f.Pos()).Offset] = receiverName(f.Recv.List[0].Type)
		}
		if declaration, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range declaration.Specs {
				if value, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range value.Names {
						valueNames[fset.Position(value.Pos()).Offset] = append(valueNames[fset.Position(value.Pos()).Offset], name.Name)
					}
				}
			}
		}
	}
	queries := []struct{ query, kind string }{
		{`(function_declaration name: (identifier) @name) @body`, "func"},
		{`(method_declaration name: (field_identifier) @name) @body`, "method"},
		{`(type_spec name: (type_identifier) @name) @body`, "type"},
		{`(type_alias name: (type_identifier) @name) @body`, "type"},
		{`(var_spec name: (identifier) @name) @body`, "var"},
		{`(const_spec name: (identifier) @name) @body`, "const"},
	}
	result := []Symbol{}
	seen := map[string]bool{}
	for _, query := range queries {
		q, e := sitter.NewQuery([]byte(query.query), language)
		if e != nil {
			return nil, e
		}
		cursor := sitter.NewQueryCursor()
		cursor.Exec(q, tree.RootNode())
		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}
			name := ""
			var node *sitter.Node
			for _, capture := range match.Captures {
				if q.CaptureNameForId(capture.Index) == "name" {
					name = capture.Node.Content(raw)
				}
				if q.CaptureNameForId(capture.Index) == "body" {
					node = capture.Node
				}
			}
			if node == nil || name == "" {
				continue
			}
			start, end := int(node.StartByte()), int(node.EndByte())
			if start < 0 || end > len(raw) || end <= start {
				continue
			}
			// Locals are not durable file-scope symbol selectors.
			local := false
			for parent := node.Parent(); parent != nil; parent = parent.Parent() {
				if parent.Type() == "block" {
					local = true
					break
				}
			}
			if local {
				continue
			}
			names := []string{name}
			if (query.kind == "var" || query.kind == "const") && len(valueNames[start]) > 0 {
				names = valueNames[start]
			}
			for _, name := range names {
				qualified := name
				if receiver := receivers[start]; receiver != "" {
					qualified = receiver + "." + name
				}
				key := fmt.Sprintf("%d/%s/%s", start, query.kind, qualified)
				if seen[key] {
					continue
				}
				seen[key] = true
				header := []byte(query.kind + "\x00" + qualified)
				if body := node.ChildByFieldName("body"); body != nil {
					header = raw[start:int(body.StartByte())]
				}
				sig := syntaxDigest(header, false)
				anchor := carrier.Digest([]byte(Version + "\x00" + p + "\x00" + query.kind + "\x00" + qualified + "\x00" + sig))
				result = append(result, Symbol{Anchor: "sym:" + anchor, Path: p, Name: name, QualifiedName: qualified, Kind: query.kind, SignatureDigest: sig, SyntaxDigest: syntaxDigest(raw[start:end], false), RawDigest: carrier.Digest(raw[start:end]), StartByte: start, EndByte: end, Line: int(node.StartPoint().Row) + 1, EndLine: int(node.EndPoint().Row) + 1})
			}
		}
		cursor.Close()
		q.Close()
	}
	return result, nil
}
