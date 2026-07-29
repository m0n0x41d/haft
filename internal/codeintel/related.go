package codeintel

import (
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/graphrank"
	"github.com/m0n0x41d/haft/internal/projectpath"
	"github.com/m0n0x41d/haft/internal/textsearch"
)

// Phase 2 of dec-20260604-3aaad199: FTS5-seeded Personalized PageRank over the
// FUSED graph (code symbols + call/dispatch edges + reasoning artifacts + their
// links + the artifact<->file<->symbol bridges). Deterministic, no embeddings,
// no second runtime. Surfaces graph-proximity recall — "what's related to this"
// ranked by distance in the fused graph — folded into the existing related action.

// RelatedKind tags a ranked node so the surface can split symbols from artifacts.
type RelatedKind int

const (
	RelatedSymbol RelatedKind = iota
	RelatedArtifact
	relatedFile // connector node; filtered out of results
	relatedModule
)

// RelatedNode is one graph-proximity-ranked node (a symbol or an artifact).
type RelatedNode struct {
	ID    string
	Kind  RelatedKind
	Score float64
}

const fileNodePrefix = "file::"
const moduleNodePrefix = "module::"

func fileNode(path string) string { return fileNodePrefix + path }
func moduleNode(id string) string { return moduleNodePrefix + id }

type symbolModuleAssignment struct {
	SymbolID string
	ModuleID string
}

type moduleFusionInputs struct {
	Contexts    []graph.DecisionModuleContext
	Assignments []symbolModuleAssignment
}

func resolveModuleFusionInputs(
	modules []codebase.Module,
	contexts []graph.DecisionModuleContext,
	symbols []codebase.SymbolRef,
) (moduleFusionInputs, error) {
	graphContexts := moduleFusionGraphContexts(contexts)
	activeModules := make(map[string]bool)
	for _, context := range graphContexts {
		activeModules[context.ModuleID] = true
	}
	refs := make([]projectpath.ModuleRef, 0, len(modules))
	for _, module := range modules {
		ref, err := projectpath.NewModuleRef(module.ID, module.Path)
		if err != nil {
			return moduleFusionInputs{}, err
		}
		refs = append(refs, ref)
	}
	assignments := make([]symbolModuleAssignment, 0, len(symbols))
	for _, symbol := range symbols {
		filePath, err := projectpath.Parse(symbol.FilePath)
		if err != nil {
			return moduleFusionInputs{}, err
		}
		module, ok, err := projectpath.ResolveMostSpecificModule(
			refs,
			filePath,
		)
		if err != nil {
			return moduleFusionInputs{}, err
		}
		if !ok {
			continue
		}
		if !activeModules[module.ID()] {
			continue
		}
		assignments = append(assignments, symbolModuleAssignment{
			SymbolID: symbol.ID,
			ModuleID: module.ID(),
		})
	}
	return moduleFusionInputs{
		Contexts:    graphContexts,
		Assignments: assignments,
	}, nil
}

func moduleFusionGraphContexts(
	contexts []graph.DecisionModuleContext,
) []graph.DecisionModuleContext {
	result := make([]graph.DecisionModuleContext, 0, len(contexts))
	for _, context := range contexts {
		if context.Source != "explicit_module_binding" {
			continue
		}
		result = append(result, context)
	}
	return result
}

func (s *Service) loadModuleFusionInputs(
	ctx context.Context,
	symbols []codebase.SymbolRef,
) (moduleFusionInputs, error) {
	modules, err := s.scanner.GetModules(ctx)
	if err != nil {
		return moduleFusionInputs{}, err
	}
	contexts, err := s.graph.AllDecisionModuleContexts(ctx)
	if err != nil {
		return moduleFusionInputs{}, err
	}
	return resolveModuleFusionInputs(modules, contexts, symbols)
}

// Fused-graph edge weights. Uniform in v1 — per dec-20260604-3aaad199, weight
// tuning is an OBSERVATION (watched, not optimized), so they are named consts to
// tune later rather than magic literals.
const (
	wCode          = 1.0  // symbol -> symbol call/dispatch edge
	wLink          = 1.0  // artifact -> artifact reasoning link
	wBridge        = 1.0  // artifact <-> file, symbol <-> file connector
	wBindingBridge = 10.0 // exact artifact <-> symbol binding
	wModuleBridge  = 1.0  // typed module binding <-> owned symbol connector
)

// buildFusedGraph (pure) fuses the code graph, the reasoning graph, and the
// artifact<->file<->symbol bridges into one bidirectional graphrank.Graph, plus
// a node->kind map so ranked results can be split into symbols vs artifacts.
// Edges go BOTH directions: "related" is symmetric — a caller is as related as a
// callee, a governing decision as related as the symbol it governs.
func buildFusedGraph(
	edges []codebase.CodeEdge,
	links []artifact.LinkEdge,
	affected []artifact.AffectedFileRef,
	bindings []artifact.SymbolBindingRef,
	syms []codebase.SymbolRef,
	modules moduleFusionInputs,
) (*graphrank.Graph, map[string]RelatedKind) {
	g := graphrank.NewGraph()
	kind := make(map[string]RelatedKind)
	symbolByAnchor := make(map[string]string, len(syms))

	for _, e := range edges {
		g.AddEdge(e.SrcID, e.DstID, wCode)
		g.AddEdge(e.DstID, e.SrcID, wCode)
		kind[e.SrcID] = RelatedSymbol
		kind[e.DstID] = RelatedSymbol
	}
	for _, l := range links {
		g.AddEdge(l.Source, l.Target, wLink)
		g.AddEdge(l.Target, l.Source, wLink)
		kind[l.Source] = RelatedArtifact
		kind[l.Target] = RelatedArtifact
	}
	for _, af := range affected {
		fn := fileNode(af.FilePath)
		g.AddEdge(af.ArtifactID, fn, wBridge)
		g.AddEdge(fn, af.ArtifactID, wBridge)
		kind[af.ArtifactID] = RelatedArtifact
		kind[fn] = relatedFile
	}
	for _, sr := range syms {
		if sr.AnchorID != "" {
			symbolByAnchor[sr.AnchorID] = sr.ID
		}
		fn := fileNode(sr.FilePath)
		g.AddEdge(sr.ID, fn, wBridge)
		g.AddEdge(fn, sr.ID, wBridge)
		kind[sr.ID] = RelatedSymbol
		kind[fn] = relatedFile
	}
	for _, binding := range bindings {
		symbolID, current := symbolByAnchor[binding.AnchorID]
		if !current {
			continue
		}
		g.AddEdge(binding.ArtifactID, symbolID, wBindingBridge)
		g.AddEdge(symbolID, binding.ArtifactID, wBindingBridge)
		kind[binding.ArtifactID] = RelatedArtifact
		kind[symbolID] = RelatedSymbol
	}
	for _, context := range modules.Contexts {
		moduleID := moduleNode(context.ModuleID)
		g.AddEdge(context.DecisionID, moduleID, wModuleBridge)
		g.AddEdge(moduleID, context.DecisionID, wModuleBridge)
		kind[context.DecisionID] = RelatedArtifact
		kind[moduleID] = relatedModule
	}
	for _, assignment := range modules.Assignments {
		moduleID := moduleNode(assignment.ModuleID)
		g.AddEdge(assignment.SymbolID, moduleID, wModuleBridge)
		g.AddEdge(moduleID, assignment.SymbolID, wModuleBridge)
		kind[assignment.SymbolID] = RelatedSymbol
		kind[moduleID] = relatedModule
	}
	return g, kind
}

// rankRelated runs PPR from the seeds and returns the top non-seed, non-file
// nodes by score, deterministically (score desc, then id asc). nil seeds or a
// seed that lands on no graph node yields an empty slice.
func rankRelated(g *graphrank.Graph, kind map[string]RelatedKind, seeds map[string]float64, limit int) []RelatedNode {
	scores := g.Rank(seeds, graphrank.DefaultParams())
	out := make([]RelatedNode, 0, len(scores))
	for id, sc := range scores {
		if sc <= 0 {
			continue
		}
		if _, isSeed := seeds[id]; isSeed {
			continue
		}
		k, ok := kind[id]
		if !ok || k == relatedFile || k == relatedModule {
			continue // drop file connector nodes — they are plumbing, not results
		}
		out = append(out, RelatedNode{ID: id, Kind: k, Score: sc})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// nodeFile / nodeName decompose a symbol node id (file#name#line per
// codebase.NodeID). They return "" for non-symbol ids (artifacts, file nodes).
func nodeFile(id string) string {
	if i := indexByteUntilHash(id); i > 0 {
		return id[:i]
	}
	return ""
}

func nodeName(id string) string {
	first := indexByteUntilHash(id)
	last := lastIndexByteUntilHash(id)
	if first < 0 || last <= first {
		return ""
	}
	return id[first+1 : last]
}

func indexByteUntilHash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' {
			return i
		}
	}
	return -1
}

func lastIndexByteUntilHash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '#' {
			return i
		}
	}
	return -1
}

// isCallableKind reports whether a symbol kind can carry call edges (and so can
// be "exercised by" a test). Types/consts/fields have no callers.
func isCallableKind(kind string) bool {
	switch kind {
	case "func", "function", "method":
		return true
	default:
		return false
	}
}

// SymbolCoverage is one callable symbol of a file plus the test functions that
// exercise it via call edges (dec-20260604-ef966a11). It is STRUCTURAL coverage
// — "exercised / tested by", NOT behavioral "verified": a test may call the
// symbol only as setup. Empty TestedBy means no test exercises it.
type SymbolCoverage struct {
	Symbol   string
	Exported bool
	TestedBy []string
}

// coverageFor (pure) builds the per-callable-symbol coverage map: for each
// callable symbol, the distinct test functions whose call edges reach it.
// Deterministic and order-stable. `callers` maps a symbol id to its inbound edges.
func coverageFor(syms []codebase.CodeSymbol, callers map[string][]codebase.CodeEdge) []SymbolCoverage {
	out := make([]SymbolCoverage, 0, len(syms))
	for _, sym := range syms {
		if !isCallableKind(sym.Kind) {
			continue
		}
		seen := map[string]bool{}
		var tests []string
		for _, e := range callers[sym.ID] {
			f := nodeFile(e.SrcID)
			if f == "" || !textsearch.IsTestPath(f) {
				continue
			}
			name := nodeName(e.SrcID)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			tests = append(tests, name)
		}
		sort.Strings(tests)
		out = append(out, SymbolCoverage{Symbol: sym.Name, Exported: sym.Exported, TestedBy: tests})
	}
	return out
}

// RelatedResult is a ranked node resolved to a display title — what the surface
// renders. Title is the artifact title, or "Name  (file:line)" for a symbol.
type RelatedResult struct {
	RelatedNode
	Title string
}

// RelatedView pairs proximity results with the exact index basis used to build
// the fused code/reasoning graph.
type RelatedView struct {
	Results []RelatedResult
	Index   codebase.IndexState
}

// RelatedToFile ranks the fused graph by proximity to a file: it seeds the file
// connector node (whose neighbors are the artifacts affecting that file and the
// symbols defined in it) and returns the nearest artifacts + symbols, each
// resolved to a display title. Thin shell — enumerate the stores, delegate to
// the pure builder + ranker, then resolve titles. A node whose title no longer
// resolves (stale id) is dropped rather than shown bare.
func (s *Service) RelatedToFile(ctx context.Context, projectRoot, filePath string, limit int) ([]RelatedResult, error) {
	view, err := s.RelatedView(ctx, projectRoot, filePath, limit)
	return view.Results, err
}

// RelatedView returns the public, basis-carrying proximity result.
func (s *Service) RelatedView(
	ctx context.Context,
	projectRoot string,
	filePath string,
	limit int,
) (RelatedView, error) {
	return retryIndexQuery(func() (RelatedView, error) {
		return s.relatedViewOnce(ctx, projectRoot, filePath, limit)
	})
}

func (s *Service) relatedViewOnce(
	ctx context.Context,
	projectRoot string,
	filePath string,
	limit int,
) (result RelatedView, resultErr error) {
	if _, err := s.EnsureIndex(ctx, projectRoot); err != nil {
		return RelatedView{}, err
	}
	indexMu.RLock()
	defer indexMu.RUnlock()
	indexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return RelatedView{}, err
	}
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, indexState); err != nil {
			result = RelatedView{}
			resultErr = err
		}
	}()
	edges, err := s.edges.AllEdges(ctx)
	if err != nil {
		return RelatedView{}, err
	}
	links, err := s.art.AllLinks(ctx)
	if err != nil {
		return RelatedView{}, err
	}
	affected, err := s.art.AllAffectedFiles(ctx)
	if err != nil {
		return RelatedView{}, err
	}
	bindings, err := s.art.AllActiveSymbolBindingRefs(ctx)
	if err != nil {
		return RelatedView{}, err
	}
	syms, err := s.symbols.AllSymbolRefs(ctx)
	if err != nil {
		return RelatedView{}, err
	}
	moduleFusion, err := s.loadModuleFusionInputs(ctx, syms)
	if err != nil {
		return RelatedView{}, err
	}
	g, kind := buildFusedGraph(
		edges,
		links,
		affected,
		bindings,
		syms,
		moduleFusion,
	)
	// Rank ALL nodes (limit 0); resolveRelated then drops co-file AND test-file
	// symbols (proximity is production-only — tests live in the separate Tested-by
	// lane, dec-20260604-ef966a11), THEN trim — filtering must precede the trim so
	// production neighbors are not crowded out before being chosen.
	ranked := rankRelated(g, kind, map[string]float64{fileNode(filePath): 1}, 0)
	if limit <= 0 {
		limit = 8
	}

	out := make([]RelatedResult, 0, limit)
	for _, n := range ranked {
		title, keep := s.resolveRelated(ctx, n, filePath)
		if !keep {
			continue
		}
		out = append(out, RelatedResult{RelatedNode: n, Title: title})
		if len(out) >= limit {
			break
		}
	}
	return RelatedView{
		Results: out,
		Index:   indexState,
	}, nil
}

// resolveRelated renders a ranked node's display title and reports whether to
// keep it. It drops nodes whose id no longer resolves AND symbols defined in the
// seed file itself — co-file symbols are trivially "related" (the caller already
// has the file), so excluding them lets the section surface the non-obvious
// cross-file symbols and graph-reachable artifacts instead.
func (s *Service) resolveRelated(ctx context.Context, n RelatedNode, seedFile string) (string, bool) {
	switch n.Kind {
	case RelatedArtifact:
		a, err := s.art.Get(ctx, n.ID)
		if err != nil || a == nil {
			return "", false
		}
		return a.Meta.Title, true
	case RelatedSymbol:
		sym, ok, err := s.symbols.GetByID(ctx, n.ID)
		if err != nil || !ok {
			return "", false
		}
		if sym.FilePath == seedFile {
			return "", false // co-file: trivially related — drop
		}
		if textsearch.IsTestPath(sym.FilePath) {
			return "", false // tests belong in the Tested-by lane, not proximity
		}
		return fmt.Sprintf("%s  (%s:%d)", sym.Name, sym.FilePath, sym.StartLine), true
	default:
		return "", false
	}
}

// TestedBy returns the structural test-coverage map for a file: each callable
// symbol defined in it and the test functions whose call edges reach it
// (dec-20260604-ef966a11). Thin shell — gather each symbol's inbound edges, then
// delegate to the pure coverageFor. "Exercised by", not "verified".
func (s *Service) TestedBy(ctx context.Context, projectRoot, filePath string) ([]SymbolCoverage, error) {
	view, err := s.TestCoverageView(ctx, projectRoot, filePath)
	return view.Symbols, err
}

// TestCoverageView pairs the structural exercise map with its exact index
// basis. It does not upgrade call evidence into verification evidence.
type TestCoverageView struct {
	Symbols []SymbolCoverage
	Index   codebase.IndexState
}

// TestCoverageView returns the public, basis-carrying structural test map.
func (s *Service) TestCoverageView(
	ctx context.Context,
	projectRoot string,
	filePath string,
) (TestCoverageView, error) {
	return retryIndexQuery(func() (TestCoverageView, error) {
		return s.testCoverageViewOnce(ctx, projectRoot, filePath)
	})
}

func (s *Service) testCoverageViewOnce(
	ctx context.Context,
	projectRoot string,
	filePath string,
) (result TestCoverageView, resultErr error) {
	if _, err := s.EnsureIndex(ctx, projectRoot); err != nil {
		return TestCoverageView{}, err
	}
	indexMu.RLock()
	defer indexMu.RUnlock()
	indexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return TestCoverageView{}, err
	}
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, indexState); err != nil {
			result = TestCoverageView{}
			resultErr = err
		}
	}()
	syms, err := s.symbols.GetByFile(ctx, filePath)
	if err != nil {
		return TestCoverageView{}, err
	}
	callers := make(map[string][]codebase.CodeEdge, len(syms))
	for _, sym := range syms {
		if !isCallableKind(sym.Kind) {
			continue
		}
		in, err := s.edges.InEdges(ctx, sym.ID)
		if err != nil {
			return TestCoverageView{}, err
		}
		callers[sym.ID] = in
	}
	return TestCoverageView{
		Symbols: coverageFor(syms, callers),
		Index:   indexState,
	}, nil
}
