package codebase

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// TypeEmbed is a Go type and one type it embeds — an anonymous struct field or
// an embedded interface (Go's composition / "extends"). Both names are bare
// (package-stripped).
type TypeEmbed struct {
	Type     string
	Embedded string
}

// ExtractGoEmbeds reads one Go file and records each struct/interface embedding
// (an anonymous field, or an embedded interface) as a (Type, Embedded) pair.
// Shell (read + parse); pure data out. Unparseable files yield nothing.
func ExtractGoEmbeds(projectRoot, relPath string) []TypeEmbed {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil
	}
	return ExtractGoEmbedsFromSource(source)
}

// ExtractGoEmbedsFromSource derives Go embedding relations from admitted bytes.
func ExtractGoEmbedsFromSource(source AdmittedSource) []TypeEmbed {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(
		fset,
		source.Path().String(),
		source.bytes(),
		0,
	)
	if err != nil {
		return nil
	}
	var out []TypeEmbed
	for _, decl := range af.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			switch t := ts.Type.(type) {
			case *ast.StructType:
				if t.Fields == nil {
					continue
				}
				for _, f := range t.Fields.List {
					if len(f.Names) != 0 {
						continue // a named field, not an embedding
					}
					if name := embeddedTypeName(f.Type); name != "" {
						out = append(out, TypeEmbed{Type: ts.Name.Name, Embedded: name})
					}
				}
			case *ast.InterfaceType:
				if t.Methods == nil {
					continue
				}
				for _, m := range t.Methods.List {
					if len(m.Names) != 0 {
						continue // a method spec, not an embedded interface
					}
					if name := embeddedTypeName(m.Type); name != "" {
						out = append(out, TypeEmbed{Type: ts.Name.Name, Embedded: name})
					}
				}
			}
		}
	}
	return out
}

// embeddedTypeName returns the bare embedded type name from an embedding field
// type: T, *T, or pkg.T -> T. Anything else yields "".
func embeddedTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr: // *T
		return embeddedTypeName(e.X)
	case *ast.SelectorExpr: // pkg.T -> T (external; resolution usually drops it)
		return e.Sel.Name
	}
	return ""
}

// ResolveEmbedsEdges emits `embeds` edges (type -> embedded type) for each
// embedding declared in relPath, resolving the embedded name to exactly one
// package-local type symbol. External or ambiguous embeds are dropped, never
// guessed (the no-wrong-edge invariant). Heuristic provenance: the embedding is
// explicit in source, but name resolution here is package-scoped without import
// analysis. Pure.
func ResolveEmbedsEdges(relPath string, fileSyms, pkgSyms []CodeSymbol, embeds []TypeEmbed) []CodeEdge {
	pkgTypes := map[string][]CodeSymbol{}
	for _, s := range pkgSyms {
		if s.Kind == "type" || s.Kind == "interface" {
			pkgTypes[s.Name] = append(pkgTypes[s.Name], s)
		}
	}
	fileType := map[string]CodeSymbol{}
	for _, s := range fileSyms {
		if s.Kind == "type" || s.Kind == "interface" {
			fileType[s.Name] = s
		}
	}

	var edges []CodeEdge
	for _, em := range embeds {
		if em.Embedded == em.Type {
			continue
		}
		src, ok := fileType[em.Type]
		if !ok {
			continue
		}
		cands := pkgTypes[em.Embedded]
		if len(cands) != 1 {
			continue // external or ambiguous — drop, don't guess
		}
		edges = append(edges, CodeEdge{
			SrcID:      src.ID,
			DstID:      cands[0].ID,
			Kind:       EdgeEmbeds,
			FilePath:   relPath,
			Provenance: ProvenanceHeuristic,
		})
	}
	return edges
}
