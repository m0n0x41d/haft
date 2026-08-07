package codebase

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// TypeFacts is the package-scoped index needed to infer the types of LOCAL
// variables (`x := f()`, `x := recv.field.Method()`), so interface-dispatch
// resolution reaches `:=`-inferred receivers, not only declared ones.
//
// Everything here is PACKAGE-SCOPED on purpose: within one Go package, func
// names, (receiver, method) pairs, and (struct, field) pairs are all unique, so
// aggregating across the package's files is collision-free. Cross-package facts
// are deliberately absent — an unresolvable RHS yields NO inferred type, never a
// guessed one (the precision-cliff discipline that keeps dispatch edges honest).
type TypeFacts struct {
	FuncReturns   map[string][]string          // funcName -> ordered bare result types
	MethodReturns map[string][]string          // recvType \x00 methodName -> ordered bare result types
	StructFields  map[string]map[string]string // structType -> fieldName -> bare type
}

// NewTypeFacts returns an empty, ready-to-fill TypeFacts.
func NewTypeFacts() TypeFacts {
	return TypeFacts{
		FuncReturns:   map[string][]string{},
		MethodReturns: map[string][]string{},
		StructFields:  map[string]map[string]string{},
	}
}

// merge folds another file's facts in. Package-scoped uniqueness means there are
// no real conflicts; last-writer-wins is safe.
func (f TypeFacts) merge(other TypeFacts) {
	for k, v := range other.FuncReturns {
		f.FuncReturns[k] = v
	}
	for k, v := range other.MethodReturns {
		f.MethodReturns[k] = v
	}
	for k, v := range other.StructFields {
		f.StructFields[k] = v
	}
}

// ExtractGoTypeFacts reads one Go file and records its func/method return types
// and struct field types. Shell (file read + parse); the recorded facts are pure
// data. Unparseable files yield empty facts, never an error that aborts a scan.
func ExtractGoTypeFacts(projectRoot, relPath string) (TypeFacts, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return NewTypeFacts(), err
	}
	return ExtractGoTypeFactsFromSource(source)
}

// ExtractGoTypeFactsFromSource derives package type facts from admitted bytes.
func ExtractGoTypeFactsFromSource(
	source AdmittedSource,
) (TypeFacts, error) {
	facts := NewTypeFacts()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(
		fset,
		source.Path().String(),
		source.bytes(),
		0,
	)
	if err != nil {
		return facts, nil
	}
	for _, decl := range af.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			results := resultTypes(d.Type)
			if len(results) == 0 {
				continue
			}
			if d.Recv != nil {
				recv := receiverTypeName(d.Recv)
				if recv != "" {
					facts.MethodReturns[recv+"\x00"+d.Name.Name] = results
				}
				continue
			}
			facts.FuncReturns[d.Name.Name] = results
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				fields := map[string]string{}
				for _, field := range st.Fields.List {
					ft := goTypeName(field.Type)
					if ft == "" {
						continue
					}
					for _, n := range field.Names {
						fields[n.Name] = ft
					}
				}
				if len(fields) > 0 {
					facts.StructFields[ts.Name.Name] = fields
				}
			}
		}
	}
	return facts, nil
}

// resultTypes returns the ordered bare result types of a function signature
// ("" entries dropped), so a single-return func maps a `:=` LHS directly and a
// multi-return func maps positionally when LHS count matches.
func resultTypes(ft *ast.FuncType) []string {
	if ft.Results == nil {
		return nil
	}
	var out []string
	for _, field := range ft.Results.List {
		out = append(out, goTypeName(field.Type))
	}
	return out
}

// receiverTypeName extracts the bare receiver type of a method declaration.
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	return goTypeName(recv.List[0].Type)
}

// InferLocalVarTypes returns the variable→type map for a function body: the base
// (receiver + params) extended with locals whose RHS type resolves through the
// package's TypeFacts. Pure given (fn, base, facts).
//
// Soundness over precision: the result is a FLAT per-function table, but Go has
// lexical scope, so a name can denote different types at different lines (inner-
// block shadowing, or a base param shadowed by a local). A flat table cannot say
// which type holds at a given call site — so the moment a name is bound to two
// DIFFERENT types, it is dropped entirely (marked ambiguous). Absent is safe; a
// wrong type would become a wrong dispatch edge. Only `:=` and `var` bind here;
// `=` cannot change a Go variable's type, so it is ignored.
func InferLocalVarTypes(fn *ast.FuncDecl, base map[string]string, facts TypeFacts) map[string]string {
	vars := make(map[string]string, len(base))
	for k, v := range base {
		vars[k] = v
	}
	if fn.Body == nil {
		return vars
	}
	ambiguous := map[string]bool{}
	bind := func(name, t string) {
		if t == "" || name == "" || name == "_" || ambiguous[name] {
			return
		}
		if prev, ok := vars[name]; ok && prev != t {
			ambiguous[name] = true
			delete(vars, name)
			return
		}
		vars[name] = t
	}
	// Pre-order traversal visits statements in source order, so a `:=` is seen
	// before the statements that use the variable it binds.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			inferAssign(s, vars, facts, bind)
		case *ast.DeclStmt:
			inferDecl(s, vars, facts, bind)
		}
		return true
	})
	return vars
}

// inferAssign binds LHS names from a `:=` definition when the RHS type(s)
// resolve. `=` is skipped — reassignment cannot change a variable's Go type.
func inferAssign(s *ast.AssignStmt, vars map[string]string, facts TypeFacts, bind func(name, t string)) {
	if s.Tok != token.DEFINE {
		return
	}
	// a, b := f()  — single call feeding multiple LHS, mapped positionally.
	if len(s.Rhs) == 1 {
		if call, ok := s.Rhs[0].(*ast.CallExpr); ok && len(s.Lhs) > 1 {
			results := inferCallResults(call, vars, facts)
			if len(results) == len(s.Lhs) {
				for i, lhs := range s.Lhs {
					bindIdent(lhs, results[i], bind)
				}
			}
			return
		}
	}
	// Parallel / single definition: each RHS resolved independently.
	if len(s.Lhs) == len(s.Rhs) {
		for i, lhs := range s.Lhs {
			bindIdent(lhs, inferExprType(s.Rhs[i], vars, facts), bind)
		}
	}
}

// inferDecl handles `var x T` / `var x = expr` / `var x T = expr`.
func inferDecl(s *ast.DeclStmt, vars map[string]string, facts TypeFacts, bind func(name, t string)) {
	gd, ok := s.Decl.(*ast.GenDecl)
	if !ok || gd.Tok != token.VAR {
		return
	}
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if vs.Type != nil {
			t := goTypeName(vs.Type)
			for _, name := range vs.Names {
				bind(name.Name, t)
			}
			continue
		}
		if len(vs.Names) == len(vs.Values) {
			for i, name := range vs.Names {
				bind(name.Name, inferExprType(vs.Values[i], vars, facts))
			}
		}
	}
}

func bindIdent(lhs ast.Expr, t string, bind func(name, t string)) {
	if id, ok := lhs.(*ast.Ident); ok {
		bind(id.Name, t)
	}
}

// inferExprType resolves the bare type of an expression using known var types +
// package facts. Returns "" for anything it cannot resolve uniquely (drop).
func inferExprType(expr ast.Expr, vars map[string]string, facts TypeFacts) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return vars[e.Name]
	case *ast.ParenExpr:
		return inferExprType(e.X, vars, facts)
	case *ast.StarExpr: // *p
		return inferExprType(e.X, vars, facts)
	case *ast.UnaryExpr: // &x
		if e.Op == token.AND {
			return inferExprType(e.X, vars, facts)
		}
		return ""
	case *ast.CompositeLit: // T{...}
		return goTypeName(e.Type)
	case *ast.SelectorExpr: // x.field (field access, not a call)
		tx := inferExprType(e.X, vars, facts)
		if tx == "" {
			return ""
		}
		if fields, ok := facts.StructFields[tx]; ok {
			return fields[e.Sel.Name]
		}
		return ""
	case *ast.CallExpr:
		results := inferCallResults(e, vars, facts)
		if len(results) == 1 {
			return results[0]
		}
		return "" // zero or multi result in single-value context — drop
	default:
		return ""
	}
}

// resolveQualifierType resolves the bare TYPE of a call qualifier — a simple
// receiver var ("store") or a dotted field path ("s.scanner") — using the
// function's var→type table and the package's struct-field facts. Returns "" for
// anything it cannot resolve uniquely (a package selector, an unknown field):
// conservative drop, so a concrete method edge is never bound to a guessed type.
func resolveQualifierType(qualifier string, vars map[string]string, facts TypeFacts) string {
	parts := strings.Split(qualifier, ".")
	if len(parts) == 0 {
		return ""
	}
	t, ok := vars[parts[0]]
	if !ok {
		return "" // base is not a known var (likely a package alias) — drop
	}
	for _, field := range parts[1:] {
		fields, ok := facts.StructFields[t]
		if !ok {
			return ""
		}
		t, ok = fields[field]
		if !ok || t == "" {
			return ""
		}
	}
	return t
}

// inferCallResults resolves a call's ordered result types via package facts.
// Returns nil when the callee is unknown (e.g. a cross-package function whose
// return type is not in this package's facts) — conservative drop.
func inferCallResults(call *ast.CallExpr, vars map[string]string, facts TypeFacts) []string {
	switch fn := call.Fun.(type) {
	case *ast.Ident: // f(...)
		return facts.FuncReturns[fn.Name]
	case *ast.SelectorExpr: // recv.Method(...) or pkg.Func(...)
		tx := inferExprType(fn.X, vars, facts)
		if tx == "" {
			return nil // receiver type unknown (or pkg selector) → drop
		}
		return facts.MethodReturns[tx+"\x00"+fn.Sel.Name]
	default:
		return nil
	}
}
