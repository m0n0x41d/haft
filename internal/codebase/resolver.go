package codebase

import (
	"context"
	"path/filepath"
	"strings"
)

// SymbolView is the read port over the symbol-node store that an EdgeResolver
// needs to resolve call targets to nodes (scoped per file / package / name).
// *SymbolStore satisfies it — resolvers depend on this abstraction, not the
// concrete store (hexagonal).
type SymbolView interface {
	GetByFile(ctx context.Context, filePath string) ([]CodeSymbol, error)
	GetByDir(ctx context.Context, dir string) ([]CodeSymbol, error)
	GetByName(ctx context.Context, name string) ([]CodeSymbol, error)
}

// EdgeResolver extracts the code→code edges originating in ONE file. It is the
// per-language PORT: the code graph is NOT Go-specific. Go is the first adapter;
// Python / TypeScript / Rust / … each add an EdgeResolver implementation plus a
// registry entry — exactly like the existing ModuleDetector / ImportParser
// adapters. Orchestration (scan, traversal, tools) codes against this interface,
// never against a language. An unresolved call/dispatch yields no edge.
type EdgeResolver interface {
	Language() string
	Extensions() []string
	ResolveFileEdges(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]CodeEdge, error)
}

// admittedEdgeResolver is the HG3 parser boundary used by index refreshes.
// Compatibility methods remain for point callers, but production batch
// resolution passes the exact AdmittedSource whose digest belongs to the
// candidate epoch.
type admittedEdgeResolver interface {
	ResolveAdmittedFileEdges(
		ctx context.Context,
		projectRoot string,
		source AdmittedSource,
		symbols SymbolView,
	) ([]CodeEdge, error)
}

type admittedEdgeOutcomeResolver interface {
	ResolveAdmittedFileEdgeOutcomes(
		ctx context.Context,
		projectRoot string,
		source AdmittedSource,
		symbols SymbolView,
	) ([]EdgeResolution, error)
}

type admittedProjectSnapshotEdgeOutcomeResolver interface {
	ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
		ctx context.Context,
		projectRoot string,
		source AdmittedSource,
		symbols SymbolView,
		snapshot *projectIndexSnapshot,
	) ([]EdgeResolution, error)
}

// EdgeOutcomeResolver is the truthful resolver port. New adapters should
// implement this in addition to EdgeResolver so unresolved and ambiguous
// relations remain inspectable without entering traversal. EdgeResolver stays
// as a compatibility projection for existing adapters during migration.
type EdgeOutcomeResolver interface {
	ResolveFileEdgeOutcomes(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]EdgeResolution, error)
}

// projectSnapshotEdgeOutcomeResolver is the prepared project-level resolver
// port used by whole-index refreshes. The snapshot is immutable and shared by
// every file resolver in one epoch; point queries keep using EdgeOutcomeResolver
// as a compatibility path.
type projectSnapshotEdgeOutcomeResolver interface {
	ResolveFileEdgeOutcomesWithProjectSnapshot(
		ctx context.Context,
		projectRoot string,
		relPath string,
		symbols SymbolView,
		snapshot *projectIndexSnapshot,
	) ([]EdgeResolution, error)
}

// ResolveFileEdges is the Go adapter's implementation of EdgeResolver — it
// composes the Go-specific extraction + resolution (call-site, intra-package,
// cross-file qualified, interface dispatch) behind the port. The Go-specific
// internals live in callsites.go / dispatch.go; nothing outside this method
// knows the language is Go.
func (g *GoLang) ResolveFileEdges(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]CodeEdge, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return g.ResolveAdmittedFileEdges(ctx, projectRoot, source, symbols)
}

func (g *GoLang) ResolveAdmittedFileEdges(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
) ([]CodeEdge, error) {
	return g.resolveAdmittedFileEdges(
		ctx,
		projectRoot,
		source,
		symbols,
		nil,
	)
}

func (g *GoLang) ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
	snapshot *projectIndexSnapshot,
) ([]EdgeResolution, error) {
	edges, err := g.resolveAdmittedFileEdges(
		ctx,
		projectRoot,
		source,
		symbols,
		snapshot.sources,
	)
	if err != nil {
		return nil, err
	}
	outcomes := make([]EdgeResolution, 0, len(edges))
	for _, edge := range edges {
		outcomes = append(outcomes, ResolvedEdge{Edge: edge})
	}
	return outcomes, nil
}

func (g *GoLang) resolveAdmittedFileEdges(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
	projectSources map[string]AdmittedSource,
) ([]CodeEdge, error) {
	relPath := source.Path().String()
	fileSyms, err := symbols.GetByFile(ctx, relPath)
	if err != nil {
		return nil, err
	}
	pkgSyms, err := symbols.GetByDir(ctx, filepath.Dir(relPath))
	if err != nil {
		return nil, err
	}
	sites, err := ExtractCallSitesFromSource(source)
	if err != nil {
		return nil, err
	}

	var edges []CodeEdge

	// 1. intra-package unqualified calls
	edges = append(edges, ResolveIntraPackageCallEdges(relPath, fileSyms, pkgSyms, sites)...)

	// 2. cross-file qualified calls (pkg.Func)
	imports, err := ExtractGoImportsFromSource(projectRoot, source)
	if err != nil {
		return nil, err
	}
	lookup := func(name string) []CodeSymbol {
		s, _ := symbols.GetByName(ctx, name)
		return s
	}
	edges = append(edges, ResolveCrossFileCallEdges(relPath, fileSyms, sites, imports, lookup)...)

	// 3. interface dispatch (interfaces, impls, AND type-facts scoped to the
	// package). Type-facts let dispatch resolve `:=`-inferred receivers (e.g.
	// `resolver := registry.ResolverForFile(p)` → EdgeResolver), not only
	// declared ones. Package scope keeps the facts collision-free.
	interfaces := map[string]InterfaceDef{}
	facts := NewTypeFacts()
	for _, pf := range distinctFiles(pkgSyms) {
		packageSource, err := admittedProjectSource(
			projectRoot,
			pf,
			projectSources,
		)
		if err != nil {
			return nil, err
		}
		defs, err := ExtractGoInterfacesFromSource(packageSource)
		if err != nil {
			return nil, err
		}
		for _, d := range defs {
			interfaces[d.Name] = d
		}
		fileFacts, err := ExtractGoTypeFactsFromSource(packageSource)
		if err != nil {
			return nil, err
		}
		facts.merge(fileFacts)
	}
	sigs, err := ExtractGoSignaturesWithLocalsFromSource(source, facts)
	if err != nil {
		return nil, err
	}
	edges = append(edges, ResolveInterfaceDispatchEdges(relPath, fileSyms, sites, sigs, interfaces, nonTestSymbols(pkgSyms))...)

	// 4. concrete method calls on typed vars/fields (store.Get, s.scanner.ScanEdges),
	// incl. cross-package — the static counterpart to dispatch, closing the
	// "method on a concrete-typed field" gap that left real callers unshown.
	edges = append(edges, ResolveConcreteMethodCallEdges(relPath, fileSyms, sites, sigs, facts, lookup)...)

	// 5. static implements edges: a type DECLARED here -> each package interface
	// whose method set it structurally covers (answers "what implements I").
	edges = append(edges, ResolveImplementsEdges(relPath, fileSyms, pkgSyms, interfaces)...)

	// 6. static embeds edges: a type DECLARED here -> each package type it embeds
	// (anonymous struct field / embedded interface = Go composition).
	embeds := ExtractGoEmbedsFromSource(source)
	edges = append(
		edges,
		ResolveEmbedsEdges(relPath, fileSyms, pkgSyms, embeds)...,
	)

	return dedupeEdges(edges), nil
}

func admittedProjectSource(
	projectRoot string,
	relPath string,
	sources map[string]AdmittedSource,
) (AdmittedSource, error) {
	source, found := sources[relPath]
	if found {
		return source, nil
	}
	return NewRegistry().ReadAdmittedSource(projectRoot, relPath)
}

// nonTestSymbols drops _test.go symbols from the dispatch impl-candidate set: a
// test double is never a production dispatch target (Go test files are not
// importable), so resolving to one would be a misleading edge — structurally
// valid but pointing at code that only runs under test.
func nonTestSymbols(syms []CodeSymbol) []CodeSymbol {
	out := make([]CodeSymbol, 0, len(syms))
	for _, s := range syms {
		if strings.HasSuffix(s.FilePath, "_test.go") {
			continue
		}
		out = append(out, s)
	}
	return out
}

func distinctFiles(syms []CodeSymbol) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range syms {
		if !seen[s.FilePath] {
			seen[s.FilePath] = true
			out = append(out, s.FilePath)
		}
	}
	return out
}

func dedupeEdges(edges []CodeEdge) []CodeEdge {
	seen := make(map[string]bool, len(edges))
	out := make([]CodeEdge, 0, len(edges))
	for _, e := range edges {
		k := e.SrcID + "\x00" + e.DstID + "\x00" + string(e.Kind)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}
