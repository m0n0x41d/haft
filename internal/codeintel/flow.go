package codeintel

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graph"
)

// FusedHop is a reached symbol with its traversal metadata AND the reasoning
// graph fused onto it — the core promise of haft's code graph: never a bare
// code node, always "what calls this AND what was decided about it".
type FusedHop struct {
	Symbol     codebase.CodeSymbol
	Distance   int
	ViaKind    codebase.EdgeKind
	Provenance codebase.Provenance
	Context    contextgraph.CodeContext
}

// Governed reports whether any reasoning touches this hop — symbol-level or, as
// the honest fallback, the enclosing module. A blank here means "nothing
// decided", never "lookup skipped".
func (h FusedHop) Governed() bool {
	return len(h.Context.Decisions) > 0 || len(h.Context.ModuleDecisions) > 0
}

// FlowResult is the outcome of a callers/callees/impact query. When the seed
// name resolves to multiple symbols and nothing disambiguates, Ambiguous lists
// the candidates and Hops is empty — an honest "which one?" instead of a
// wrong-seed answer (the keystone discipline: overloads are never conflated).
type FlowResult struct {
	Seed           codebase.CodeSymbol
	SeedResolution SeedResolution
	Candidates     []codebase.CodeSymbol
	Direction      Direction
	Profile        TraversalProfile
	Depth          int
	Hops           []FusedHop
	ColdBuilt      bool // a one-time index build ran to answer this query
	IndexRefresh   IndexCoordinationResult
	Resolution     codebase.ResolutionCounts
	Index          codebase.IndexState
}

// Service is the imperative shell of the code-graph query surface: it owns the
// stores and composes the pure traverser with the fusion layer. Stores are
// stateless over the shared DB, so one per request is fine.
type Service struct {
	scanner *codebase.Scanner
	symbols *codebase.SymbolStore
	edges   *codebase.EdgeStore
	art     *artifact.Store
	graph   *graph.Store
	seeds   ConcernExactSeedSource

	beforeBasisConfirm func(context.Context) error
	coordinatorMu      sync.Mutex
	coordinator        *ProjectIndexCoordinator
}

// NewService wires a code-graph service over the artifact store's DB.
func NewService(store *artifact.Store) *Service {
	db := store.DB()
	return &Service{
		scanner: codebase.NewScanner(db),
		symbols: codebase.NewSymbolStore(db),
		edges:   codebase.NewEdgeStore(db),
		art:     store,
		graph:   graph.NewStore(db),
		seeds:   NoConcernExactSeedSource{},
	}
}

// NewServiceWithIndexCoordinator wires the checked project-scoped coordinator
// used by every production startup and request path that can refresh the code
// index. Tests and narrow in-process consumers may continue to use NewService;
// its lazy fallback is process-scoped and never fabricates a ledger binding.
func NewServiceWithIndexCoordinator(
	store *artifact.Store,
	coordinator *ProjectIndexCoordinator,
) *Service {
	service := NewService(store)
	service.coordinator = coordinator
	return service
}

// NewServiceWithConcernExactSeeds adds an optional exact-seed adapter without
// making the draft typed-memory runtime a dependency of core concern
// discovery. The ordinary NewService path always uses the explicit null
// adapter.
func NewServiceWithConcernExactSeeds(
	store *artifact.Store,
	seeds ConcernExactSeedSource,
) *Service {
	service := NewService(store)
	service.seeds = seeds
	return service
}

// Flow runs a callers/callees traversal from the named seed, fusing the
// reasoning graph onto every reached symbol. Impact is the Callers direction
// (who breaks if this changes); Callees is the forward dependency set.
func (s *Service) Flow(ctx context.Context, projectRoot, name, file string, line, depth int, dir Direction) (FlowResult, error) {
	return s.FlowWithProfile(ctx, projectRoot, name, file, line, depth, dir, TraversalCallFlow)
}

// FlowWithProfile runs a fused traversal over one declared semantic relation
// family. The compatibility Flow entrypoint remains call_flow.
func (s *Service) FlowWithProfile(
	ctx context.Context,
	projectRoot string,
	name string,
	file string,
	line int,
	depth int,
	dir Direction,
	profile TraversalProfile,
) (FlowResult, error) {
	return retryIndexQuery(func() (FlowResult, error) {
		return s.flowWithProfileOnce(
			ctx,
			projectRoot,
			name,
			file,
			line,
			depth,
			dir,
			profile,
		)
	})
}

func (s *Service) flowWithProfileOnce(
	ctx context.Context,
	projectRoot string,
	name string,
	file string,
	line int,
	depth int,
	dir Direction,
	profile TraversalProfile,
) (result FlowResult, resultErr error) {
	indexRefresh, err := s.EnsureIndex(ctx, projectRoot)
	if err != nil {
		return FlowResult{}, err
	}
	releaseIndexRead, err := s.acquireIndexRead(projectRoot)
	if err != nil {
		return FlowResult{}, err
	}
	defer releaseIndexRead()
	publishedIndexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return FlowResult{}, err
	}
	indexState := indexRefresh.EffectiveIndexState(publishedIndexState)
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, publishedIndexState); err != nil {
			result = FlowResult{}
			resultErr = err
		}
	}()
	if indexState.Epoch == 0 {
		resolution, err := unavailableSeedForIndex(name, indexState)
		if err != nil {
			return FlowResult{}, err
		}
		return FlowResult{
			SeedResolution: resolution,
			Direction:      dir,
			Profile:        profile,
			Depth:          depth,
			ColdBuilt:      indexRefresh.Rebuilt(),
			IndexRefresh:   indexRefresh,
			Index:          indexState,
		}, nil
	}
	scope, err := traversalScopeForIndex(indexState)
	if err != nil {
		return FlowResult{}, err
	}
	seed, candidates, fuzzy, err := s.resolveSeed(ctx, name, file, line)
	if err != nil {
		return FlowResult{}, err
	}
	seedResolution, displayCandidates, err := classifySeedResolution(
		name,
		file,
		line,
		seed,
		candidates,
		fuzzy,
		scope,
	)
	if err != nil {
		return FlowResult{}, err
	}
	res := FlowResult{
		SeedResolution: seedResolution,
		Candidates:     displayCandidates,
		Direction:      dir,
		Profile:        profile,
		Depth:          depth,
		ColdBuilt:      indexRefresh.Rebuilt(),
		IndexRefresh:   indexRefresh,
		Index:          indexState,
	}
	if seedResolution.Kind().String() != "resolved_seed" {
		return res, nil
	}
	res.Seed = seed
	res.Resolution, err = s.edges.ResolutionCountsForSource(ctx, seed.ID)
	if err != nil {
		return FlowResult{}, err
	}

	hops, err := TraverseWithProfile(ctx, s.edges, seed.ID, dir, depth, MaxResults, profile)
	if err != nil {
		return FlowResult{}, err
	}
	for _, h := range hops {
		fused, ok, err := s.fuse(ctx, h)
		if err != nil {
			return FlowResult{}, err
		}
		if !ok {
			continue // edge to a node no longer in the symbol table — drop, don't fabricate
		}
		res.Hops = append(res.Hops, fused)
	}
	sort.SliceStable(res.Hops, func(i, j int) bool { return res.Hops[i].Distance < res.Hops[j].Distance })
	return res, nil
}

func unavailableSeedForIndex(
	query string,
	indexState codebase.IndexState,
) (SeedResolution, error) {
	observation, err := NewIndexObservation(
		indexState.Epoch,
		indexState.Basis.CoverageRef(),
	)
	if err != nil {
		return nil, err
	}
	reason, err := ParseSeedUnavailableReason("index_unavailable")
	if err != nil {
		return nil, err
	}
	return NewSeedUnavailable(query, reason, observation)
}

func traversalScopeForIndex(
	indexState codebase.IndexState,
) (TraversalScope, error) {
	return NewTraversalScopeWithCoverage(
		indexState.Epoch,
		indexState.Basis.CoverageRef(),
		indexState.SupportsKnownAbsence(),
	)
}

// classifySeedResolution is the pure identity boundary shared by Explore,
// ExploreBag, and Flow. A fuzzy hit remains a candidate set even when it has
// one item; lexical rank is never automatic identity selection.
func classifySeedResolution(
	query string,
	file string,
	line int,
	seed codebase.CodeSymbol,
	candidates []codebase.CodeSymbol,
	fuzzy bool,
	scope TraversalScope,
) (SeedResolution, []codebase.CodeSymbol, error) {
	displayCandidates := append([]codebase.CodeSymbol(nil), candidates...)
	if fuzzy && seed.ID != "" {
		displayCandidates = append(displayCandidates, seed)
		seed = codebase.CodeSymbol{}
	}
	sort.SliceStable(
		displayCandidates,
		func(left int, right int) bool {
			return displayCandidates[left].ID < displayCandidates[right].ID
		},
	)
	if len(displayCandidates) > 0 {
		stableCandidates, err := stableSymbols(displayCandidates)
		if err != nil {
			return nil, nil, err
		}
		basisCode := "ambiguous_exact_name"
		if fuzzy {
			basisCode = "fuzzy_candidates"
		}
		basis, err := ParseCandidateSetBasis(basisCode)
		if err != nil {
			return nil, nil, err
		}
		outcome, err := NewCandidateSet(stableCandidates, basis, scope)
		return outcome, displayCandidates, err
	}
	if seed.ID == "" {
		if scope.SupportsKnownAbsence() {
			outcome, err := NewSeedNotFound(query, scope)
			return outcome, nil, err
		}
		observation, err := NewIndexObservation(
			scope.Epoch(),
			scope.CoverageRef(),
		)
		if err != nil {
			return nil, nil, err
		}
		reason, err := ParseSeedUnavailableReason("index_incomplete")
		if err != nil {
			return nil, nil, err
		}
		outcome, err := NewSeedUnavailable(query, reason, observation)
		return outcome, nil, err
	}
	stableSeed, err := NewStableSymbol(seed.ID)
	if err != nil {
		return nil, nil, err
	}
	basisCode := "unique_exact_name"
	if file != "" && line > 0 {
		basisCode = "exact_anchor"
	}
	if strings.Contains(query, ".") {
		basisCode = "exact_qualified_name"
	}
	basis, err := ParseResolvedSeedBasis(basisCode)
	if err != nil {
		return nil, nil, err
	}
	outcome, err := NewResolvedSeed(stableSeed, basis, scope)
	return outcome, nil, err
}

func stableSymbols(
	symbols []codebase.CodeSymbol,
) ([]StableSymbol, error) {
	out := make([]StableSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		stable, err := NewStableSymbol(symbol.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, stable)
	}
	return out, nil
}

// fuse resolves a hop's node id back to its symbol and attaches the reasoning
// graph. ok=false when the symbol is gone (the edge is stale) — the caller
// drops it rather than emitting a half-resolved row.
func (s *Service) fuse(ctx context.Context, h Hop) (FusedHop, bool, error) {
	sym, ok, err := s.symbols.GetByID(ctx, h.NodeID)
	if err != nil || !ok {
		return FusedHop{}, false, err
	}
	cc, err := contextgraph.FetchCodeContext(ctx, s.art, s.graph, contextgraph.Target{
		File:     sym.FilePath,
		Symbol:   sym.Name,
		AnchorID: sym.AnchorID,
		Line:     sym.StartLine,
	})
	if err != nil {
		return FusedHop{}, false, err
	}
	return FusedHop{
		Symbol:     sym,
		Distance:   h.Distance,
		ViaKind:    h.ViaKind,
		Provenance: h.Provenance,
		Context:    cc,
	}, true, nil
}

// resolveSeed maps a (name, file, line) request to a single seed symbol, or to
// a candidate list when the name is ambiguous and nothing disambiguates it.
func (s *Service) resolveSeed(ctx context.Context, name, file string, line int) (seed codebase.CodeSymbol, candidates []codebase.CodeSymbol, fuzzy bool, err error) {
	// Most precise: a file + line covers exactly one symbol body.
	if file != "" && line > 0 {
		syms, err := s.symbols.GetByFile(ctx, file)
		if err != nil {
			return codebase.CodeSymbol{}, nil, false, err
		}
		if sym, ok := symbolCoveringLine(syms, line); ok {
			return sym, nil, false, nil
		}
	}
	// Exact name first (existing behavior): one → seed, many → overload candidates.
	bareName, receiver := splitQualifiedSymbolName(name)
	exact, err := s.symbols.GetByName(ctx, bareName)
	if err != nil {
		return codebase.CodeSymbol{}, nil, false, err
	}
	exact = filterByReceiver(exact, receiver)
	if file != "" {
		exact = filterByFile(exact, file)
	}
	if len(exact) == 1 {
		return exact[0], nil, false, nil
	}
	if len(exact) > 1 {
		return codebase.CodeSymbol{}, exact, false, nil
	}
	// No exact match → fuzzy substring fallback. Exactly one fuzzy match is used
	// (labeled fuzzy so the caller can say so); more than one is returned as
	// candidates — NEVER silently pick among ambiguous fuzzy matches.
	fuzzyHits, err := s.symbols.SearchSymbols(ctx, bareName, 12)
	if err != nil {
		return codebase.CodeSymbol{}, nil, false, err
	}
	fuzzyHits = filterByReceiver(fuzzyHits, receiver)
	if file != "" {
		fuzzyHits = filterByFile(fuzzyHits, file)
	}
	switch len(fuzzyHits) {
	case 0:
		return codebase.CodeSymbol{}, nil, false, nil // genuinely not found
	case 1:
		return fuzzyHits[0], nil, true, nil
	default:
		return codebase.CodeSymbol{}, fuzzyHits, true, nil
	}
}

// splitQualifiedSymbolName accepts the natural `Receiver.member` seed form
// used by Go methods and TypeScript object/class members. Storage keeps name and
// receiver separate so the code node remains canonical; query parsing never
// mints a duplicate dotted-name node.
func splitQualifiedSymbolName(name string) (bareName, receiver string) {
	trimmed := strings.TrimSpace(name)
	index := strings.LastIndex(trimmed, ".")
	if index <= 0 || index+1 >= len(trimmed) {
		return trimmed, ""
	}
	return trimmed[index+1:], trimmed[:index]
}

func filterByReceiver(syms []codebase.CodeSymbol, receiver string) []codebase.CodeSymbol {
	if receiver == "" {
		return syms
	}
	out := make([]codebase.CodeSymbol, 0, len(syms))
	for _, sym := range syms {
		if sym.Receiver == receiver {
			out = append(out, sym)
		}
	}
	return out
}

// symbolCoveringLine returns the innermost symbol whose [StartLine, EndLine]
// covers line (max StartLine wins for nested bodies). Pure.
func symbolCoveringLine(syms []codebase.CodeSymbol, line int) (codebase.CodeSymbol, bool) {
	best := codebase.CodeSymbol{}
	found := false
	for _, sym := range syms {
		if sym.StartLine <= line && line <= sym.EndLine {
			if !found || sym.StartLine > best.StartLine {
				best = sym
				found = true
			}
		}
	}
	return best, found
}

func filterByFile(syms []codebase.CodeSymbol, file string) []codebase.CodeSymbol {
	out := make([]codebase.CodeSymbol, 0, len(syms))
	for _, sym := range syms {
		if sym.FilePath == file {
			out = append(out, sym)
		}
	}
	return out
}
