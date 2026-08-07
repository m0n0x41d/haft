package codeintel

import (
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/contextgraph"
)

// Explore budget. A single-call answer must stay focused: the spine is a deep
// but bounded path; at most one heuristic dispatch "bridge" may be crossed, so
// the chain never strings together multiple weak hops into a misleadingly-
// complete flow. ChainVisitBudget bounds the longest-path search (it degrades
// to a deep partial path on huge graphs rather than blowing up — the latency
// objective wins over provable-longest).
const (
	MaxChainHops     = 7
	MaxChainBridges  = 1
	ChainVisitBudget = 800
)

// ChainStep is one symbol on the explore spine, fused with the reasoning
// governing exactly it. Distance 0 is the seed; ViaKind/Provenance describe the
// edge that reached it (zero-valued for the seed).
type ChainStep struct {
	Symbol     codebase.CodeSymbol
	Distance   int
	ViaKind    codebase.EdgeKind
	Provenance codebase.Provenance
	Context    contextgraph.CodeContext
}

// Bridge reports whether this step crossed a heuristic interface-dispatch
// boundary — the one place the chain's honesty matters most.
func (c ChainStep) Bridge() bool {
	return c.ViaKind == codebase.EdgeInterfaceDispatch || c.Provenance == codebase.ProvenanceHeuristic
}

// ExploreResult is the single-call capstone: the connected spine (each on-chain
// symbol fused), the blast radius (who breaks if the seed changes, with covering
// decisions), and the seed's verbatim, freshness-revalidated source. Enough to
// answer "how does this flow work and what was decided about it" at 0–1 Read.
type ExploreResult struct {
	Seed           codebase.CodeSymbol
	SeedResolution SeedResolution
	Candidates     []codebase.CodeSymbol
	Chain          []ChainStep
	ChainOutcome   ChainOutcome
	BlastRadius    []FusedHop
	SeedBody       string
	SeedBodyOK     bool
	ColdBuilt      bool
	IndexRefresh   IndexCoordinationResult
	Resolution     codebase.ResolutionCounts
	Index          codebase.IndexState
}

// Explore assembles the capstone view for a seed: the deepest connected call
// chain (fused), the immediate blast radius, and verbatim seed source.
func (s *Service) Explore(ctx context.Context, projectRoot, name, file string, line int) (ExploreResult, error) {
	return retryIndexQuery(func() (ExploreResult, error) {
		return s.exploreOnce(ctx, projectRoot, name, file, line)
	})
}

func (s *Service) exploreOnce(
	ctx context.Context,
	projectRoot string,
	name string,
	file string,
	line int,
) (result ExploreResult, resultErr error) {
	indexRefresh, err := s.EnsureIndex(ctx, projectRoot)
	if err != nil {
		return ExploreResult{}, err
	}
	releaseIndexRead, err := s.acquireIndexRead(projectRoot)
	if err != nil {
		return ExploreResult{}, err
	}
	defer releaseIndexRead()
	publishedIndexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return ExploreResult{}, err
	}
	indexState := indexRefresh.EffectiveIndexState(publishedIndexState)
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, publishedIndexState); err != nil {
			result = ExploreResult{}
			resultErr = err
		}
	}()
	if indexState.Epoch == 0 {
		resolution, err := unavailableSeedForIndex(name, indexState)
		if err != nil {
			return ExploreResult{}, err
		}
		return ExploreResult{
			SeedResolution: resolution,
			ColdBuilt:      indexRefresh.Rebuilt(),
			IndexRefresh:   indexRefresh,
			Index:          indexState,
		}, nil
	}
	scope, err := traversalScopeForIndex(indexState)
	if err != nil {
		return ExploreResult{}, err
	}
	seed, candidates, fuzzy, err := s.resolveSeed(ctx, name, file, line)
	if err != nil {
		return ExploreResult{}, err
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
		return ExploreResult{}, err
	}
	res := ExploreResult{
		SeedResolution: seedResolution,
		Candidates:     displayCandidates,
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
		return ExploreResult{}, err
	}

	budget, err := NewTraversalBudget(MaxChainHops, ChainVisitBudget)
	if err != nil {
		return ExploreResult{}, err
	}
	chainOutcome, err := longestChain(
		ctx,
		s.edges,
		seed.ID,
		scope,
		budget,
		MaxChainBridges,
	)
	if err != nil {
		return ExploreResult{}, err
	}
	res.ChainOutcome = chainOutcome
	res.Chain, err = s.fuseChain(ctx, chainHopsFromPrefix(chainOutcome.path))
	if err != nil {
		return ExploreResult{}, err
	}

	blast, err := s.trail(ctx, seed.ID, Callers)
	if err != nil {
		return ExploreResult{}, err
	}
	res.BlastRadius = blast

	body, ok, _, freshSym, err := s.freshBody(ctx, projectRoot, seed)
	if err != nil {
		return ExploreResult{}, err
	}
	res.Seed = freshSym
	res.SeedBody = string(body)
	res.SeedBodyOK = ok
	return res, nil
}

// BagSeedResolution keeps each bag query paired with its typed identity result.
// Candidates are descriptive payload; Outcome remains the canonical state.
type BagSeedResolution struct {
	Query      string
	Outcome    SeedResolution
	Candidates []codebase.CodeSymbol
}

// BagLeg is the typed connection between two adjacent resolved seeds. Forward
// and Reverse are both evaluated so absence, truncation, and unavailability
// stay visible rather than collapsing into one Connected flag.
type BagLeg struct {
	From      codebase.CodeSymbol
	To        codebase.CodeSymbol
	Forward   PathOutcome
	Reverse   PathOutcome
	Direction BagLegDirection
	Steps     []ChainStep
}

// ExploreBagResult is the multi-seed explore: how a bag of named symbols
// connects. Each adjacent pair is a leg with its connecting path (or an honest
// no-path). Seeds that did not resolve to a single symbol are listed so the
// caller disambiguates them rather than the bag silently dropping them.
type ExploreBagResult struct {
	Seeds           []codebase.CodeSymbol
	SeedResolutions []BagSeedResolution
	Unresolved      []string
	Legs            []BagLeg
	ColdBuilt       bool
	IndexRefresh    IndexCoordinationResult
	Resolution      codebase.ResolutionCounts
	Index           codebase.IndexState
}

// ExploreBag connects a bag of >=2 seed names: resolve each (exact, else
// unambiguous fuzzy) and find the shortest connecting path between each adjacent
// pair over the call/dispatch edges, trying both directions. A pair with no
// static path is reported as not connected — never bridged with a guess.
func (s *Service) ExploreBag(ctx context.Context, projectRoot string, names []string) (ExploreBagResult, error) {
	return retryIndexQuery(func() (ExploreBagResult, error) {
		return s.exploreBagOnce(ctx, projectRoot, names)
	})
}

func (s *Service) exploreBagOnce(
	ctx context.Context,
	projectRoot string,
	names []string,
) (result ExploreBagResult, resultErr error) {
	indexRefresh, err := s.EnsureIndex(ctx, projectRoot)
	if err != nil {
		return ExploreBagResult{}, err
	}
	releaseIndexRead, err := s.acquireIndexRead(projectRoot)
	if err != nil {
		return ExploreBagResult{}, err
	}
	defer releaseIndexRead()
	publishedIndexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return ExploreBagResult{}, err
	}
	indexState := indexRefresh.EffectiveIndexState(publishedIndexState)
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, publishedIndexState); err != nil {
			result = ExploreBagResult{}
			resultErr = err
		}
	}()
	if indexState.Epoch == 0 {
		result := ExploreBagResult{
			ColdBuilt:    indexRefresh.Rebuilt(),
			IndexRefresh: indexRefresh,
			Index:        indexState,
		}
		for _, name := range names {
			resolution, err := unavailableSeedForIndex(name, indexState)
			if err != nil {
				return ExploreBagResult{}, err
			}
			result.SeedResolutions = append(
				result.SeedResolutions,
				BagSeedResolution{
					Query:   name,
					Outcome: resolution,
				},
			)
			result.Unresolved = append(result.Unresolved, name)
		}
		return result, nil
	}
	scope, err := traversalScopeForIndex(indexState)
	if err != nil {
		return ExploreBagResult{}, err
	}
	budget, err := NewTraversalBudget(MaxChainHops, ChainVisitBudget)
	if err != nil {
		return ExploreBagResult{}, err
	}
	res := ExploreBagResult{
		ColdBuilt:    indexRefresh.Rebuilt(),
		IndexRefresh: indexRefresh,
		Index:        indexState,
	}
	var seeds []codebase.CodeSymbol
	for _, n := range names {
		seed, candidates, fuzzy, err := s.resolveSeed(ctx, n, "", 0)
		if err != nil {
			return ExploreBagResult{}, err
		}
		outcome, displayCandidates, err := classifySeedResolution(
			n,
			"",
			0,
			seed,
			candidates,
			fuzzy,
			scope,
		)
		if err != nil {
			return ExploreBagResult{}, err
		}
		res.SeedResolutions = append(res.SeedResolutions, BagSeedResolution{
			Query:      n,
			Outcome:    outcome,
			Candidates: displayCandidates,
		})
		if outcome.Kind().String() != "resolved_seed" {
			res.Unresolved = append(res.Unresolved, n) // not found or ambiguous — can't place in the bag
			continue
		}
		counts, err := s.edges.ResolutionCountsForSource(ctx, seed.ID)
		if err != nil {
			return ExploreBagResult{}, err
		}
		res.Resolution.Resolved += counts.Resolved
		res.Resolution.Ambiguous += counts.Ambiguous
		res.Resolution.Unresolved += counts.Unresolved
		seeds = append(seeds, seed)
	}
	res.Seeds = seeds
	if len(seeds) < 2 {
		return res, nil
	}
	for i := 0; i+1 < len(seeds); i++ {
		from, to := seeds[i], seeds[i+1]
		forward, err := shortestPath(
			ctx,
			s.edges,
			from.ID,
			to.ID,
			scope,
			budget,
		)
		if err != nil {
			return ExploreBagResult{}, err
		}
		reverse, err := shortestPath(
			ctx,
			s.edges,
			to.ID,
			from.ID,
			scope,
			budget,
		)
		if err != nil {
			return ExploreBagResult{}, err
		}
		direction := selectBagLegDirection(forward, reverse)
		selected := PathOutcome(nil)
		if direction.String() == "forward" {
			selected = forward
		}
		if direction.String() == "reverse" {
			selected = reverse
		}
		leg := BagLeg{
			From:      from,
			To:        to,
			Forward:   forward,
			Reverse:   reverse,
			Direction: direction,
		}
		if selected != nil {
			leg.Steps, err = s.fusePathOutcome(ctx, selected)
			if err != nil {
				return ExploreBagResult{}, err
			}
		}
		res.Legs = append(res.Legs, leg)
	}
	return res, nil
}

func selectBagLegDirection(
	forward PathOutcome,
	reverse PathOutcome,
) BagLegDirection {
	if forward.Kind().String() == "path_found" {
		return bagLegDirections["forward"]
	}
	if reverse.Kind().String() == "path_found" {
		return bagLegDirections["reverse"]
	}
	return bagLegDirections["none"]
}

// fuseChain resolves a node-id path to symbols and fuses each. Distance is the
// position in the EMITTED chain (not the raw hop index), so it stays contiguous
// even if a stale/deleted node is skipped mid-path.
func (s *Service) fuseChain(ctx context.Context, hops []chainHop) ([]ChainStep, error) {
	steps := make([]ChainStep, 0, len(hops))
	for _, h := range hops {
		sym, ok, err := s.symbols.GetByID(ctx, h.NodeID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		cc, err := contextgraph.FetchCodeContext(ctx, s.art, s.graph, contextgraph.Target{File: sym.FilePath, Symbol: sym.Name, AnchorID: sym.AnchorID, Line: sym.StartLine})
		if err != nil {
			return nil, err
		}
		steps = append(steps, ChainStep{Symbol: sym, Distance: len(steps), ViaKind: h.ViaKind, Provenance: h.Provenance, Context: cc})
	}
	return steps, nil
}

func (s *Service) fusePathOutcome(
	ctx context.Context,
	outcome PathOutcome,
) ([]ChainStep, error) {
	found, ok := outcome.(pathFound)
	if !ok {
		return nil, fmt.Errorf(
			"cannot fuse non-found path outcome %q",
			outcome.Kind().String(),
		)
	}
	return s.fuseChain(ctx, chainHopsFromPrefix(found.path))
}

// shortestPath is a bounded BFS from fromID to toID over out-edges. It returns
// a typed semantic outcome plus an ordinary port error; bounded truncation can
// therefore never masquerade as graph absence. Pure relative to EdgeSource.
func shortestPath(
	ctx context.Context,
	src EdgeSource,
	fromID string,
	toID string,
	scope TraversalScope,
	budget TraversalBudget,
) (PathOutcome, error) {
	from, err := NewStableSymbol(fromID)
	if err != nil {
		return nil, err
	}
	if _, err := NewStableSymbol(toID); err != nil {
		return nil, err
	}
	if fromID == toID {
		stats, err := NewTraversalStats(scope, budget, 1, 0, 0, 0)
		if err != nil {
			return nil, err
		}
		return NewPathFound(from, []GraphHop{}, stats)
	}
	visited := map[string]bool{fromID: true}
	reachedBy := map[string]chainHop{}
	pred := map[string]string{}
	type frontier struct {
		id    string
		depth int64
	}
	queue := []frontier{{fromID, 0}}
	deepest := fromID
	maxDepth := int64(0)
	inspectedEdges := int64(0)
	hitMaxHops := false
	hitVisitBudget := false
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= budget.MaxHops() {
			hitMaxHops = true
			continue
		}
		edges, err := src.OutEdges(ctx, cur.id)
		if err != nil {
			return nil, err
		}
		sortChainEdges(edges)
		inspectedEdges += int64(len(edges))
		for _, e := range edges {
			if !edgeAllowed(TraversalCallFlow, e.Kind) {
				continue
			}
			if visited[e.DstID] {
				continue
			}
			if int64(len(visited)) >= budget.VisitBudget() {
				hitVisitBudget = true
				continue
			}
			visited[e.DstID] = true
			reachedBy[e.DstID] = chainHop{NodeID: e.DstID, ViaKind: e.Kind, Provenance: e.Provenance}
			pred[e.DstID] = cur.id
			nextDepth := cur.depth + 1
			if nextDepth > maxDepth {
				maxDepth = nextDepth
				deepest = e.DstID
			}
			if e.DstID == toID {
				path := reconstructPath(fromID, toID, reachedBy, pred)
				return pathFoundOutcome(
					from,
					path,
					scope,
					budget,
					int64(len(visited)),
					inspectedEdges,
					nextDepth,
				)
			}
			queue = append(queue, frontier{e.DstID, nextDepth})
		}
	}
	partial := reconstructPath(fromID, deepest, reachedBy, pred)
	stats, err := traversalStatsForChain(
		scope,
		budget,
		int64(len(visited)),
		inspectedEdges,
		maxDepth,
		partial,
	)
	if err != nil {
		return nil, err
	}
	graphHops, err := graphHopsFromChain(partial)
	if err != nil {
		return nil, err
	}
	if hitVisitBudget {
		reason := pathTruncationReasons["visit_budget"]
		return NewPathTruncated(reason, from, graphHops, stats)
	}
	if hitMaxHops {
		reason := pathTruncationReasons["max_hops"]
		return NewPathTruncated(reason, from, graphHops, stats)
	}
	if !scope.SupportsKnownAbsence() {
		reason := pathUnavailableReasons["index_capability"]
		return NewPathUnavailable(reason, stats)
	}
	completion, err := NewCompletedTraversal(stats)
	if err != nil {
		return nil, err
	}
	return NewPathAbsentWithinIndexedGraph(completion)
}

// reconstructPath walks predecessors from toID back to fromID and reverses.
func reconstructPath(fromID, toID string, reachedBy map[string]chainHop, pred map[string]string) []chainHop {
	var rev []chainHop
	for n := toID; n != fromID; n = pred[n] {
		rev = append(rev, reachedBy[n])
	}
	rev = append(rev, chainHop{NodeID: fromID})
	out := make([]chainHop, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	return out
}

func pathFoundOutcome(
	seed StableSymbol,
	path []chainHop,
	scope TraversalScope,
	budget TraversalBudget,
	visitedNodes int64,
	inspectedEdges int64,
	hopDepth int64,
) (PathOutcome, error) {
	stats, err := traversalStatsForChain(
		scope,
		budget,
		visitedNodes,
		inspectedEdges,
		hopDepth,
		path,
	)
	if err != nil {
		return nil, err
	}
	graphHops, err := graphHopsFromChain(path)
	if err != nil {
		return nil, err
	}
	return NewPathFound(seed, graphHops, stats)
}

func traversalStatsForChain(
	scope TraversalScope,
	budget TraversalBudget,
	visitedNodes int64,
	inspectedEdges int64,
	hopDepth int64,
	path []chainHop,
) (TraversalStats, error) {
	bridges := int64(0)
	for _, hop := range path {
		if hop.ViaKind == codebase.EdgeInterfaceDispatch ||
			hop.Provenance == codebase.ProvenanceHeuristic {
			bridges++
		}
	}
	return NewTraversalStats(
		scope,
		budget,
		visitedNodes,
		inspectedEdges,
		hopDepth,
		bridges,
	)
}

func graphHopsFromChain(path []chainHop) ([]GraphHop, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("graph path requires a seed")
	}
	hops := make([]GraphHop, 0, len(path)-1)
	previous, err := NewStableSymbol(path[0].NodeID)
	if err != nil {
		return nil, err
	}
	for _, step := range path[1:] {
		current, err := NewStableSymbol(step.NodeID)
		if err != nil {
			return nil, err
		}
		hop, err := NewGraphHop(
			previous,
			current,
			step.ViaKind,
			step.Provenance,
		)
		if err != nil {
			return nil, err
		}
		hops = append(hops, hop)
		previous = current
	}
	return hops, nil
}

func chainHopsFromPrefix(path pathPrefix) []chainHop {
	hops := make([]chainHop, 0, len(path.hops)+1)
	hops = append(hops, chainHop{NodeID: path.seed.ID()})
	for _, hop := range path.hops {
		hops = append(hops, chainHop{
			NodeID:     hop.to.ID(),
			ViaKind:    hop.kind,
			Provenance: hop.provenance,
		})
	}
	return hops
}

// chainHop is a pure node-id step on the spine — the shell resolves it to a
// symbol and fuses it. The seed itself is the zero-valued first hop.
type chainHop struct {
	NodeID     string
	ViaKind    codebase.EdgeKind
	Provenance codebase.Provenance
}

// chainSuffix is a candidate continuation from a node during the longest-path
// search (the path AFTER the current node), plus its typed termination.
type chainSuffix struct {
	path        []chainHop
	bridges     int
	termination ChainTermination
}

// longestChain finds the deepest simple path of out-edges from seedID, up to
// maxHops, crossing at most maxBridges heuristic dispatch edges. Pure relative
// to the EdgeSource. Its result cannot hide budget or dispatch termination in
// booleans or tuple positions.
func longestChain(
	ctx context.Context,
	src EdgeSource,
	seedID string,
	scope TraversalScope,
	budget TraversalBudget,
	maxBridges int,
) (ChainOutcome, error) {
	seed, err := NewStableSymbol(seedID)
	if err != nil {
		return ChainOutcome{}, err
	}
	if maxBridges < 0 {
		return ChainOutcome{}, fmt.Errorf("max bridges cannot be negative")
	}
	pathVisited := map[string]bool{seedID: true}
	distinctVisited := map[string]bool{seedID: true}
	inspectedEdges := int64(0)
	maxDepthObserved := int64(0)
	hitVisitBudget := false
	var traversalErr error

	var dfs func(node string, depth int64, usedBridges int) chainSuffix
	dfs = func(node string, depth int64, usedBridges int) chainSuffix {
		if depth >= budget.MaxHops() {
			return chainSuffix{
				termination: chainTerminations["max_hops_reached"],
			}
		}
		edges, e := src.OutEdges(ctx, node)
		if e != nil {
			traversalErr = e
			return chainSuffix{termination: chainTerminations["leaf_reached"]}
		}
		inspectedEdges += int64(len(edges))
		sortChainEdges(edges)
		best := chainSuffix{termination: chainTerminations["leaf_reached"]}
		sawCappedBridge := false
		for _, ed := range edges {
			if traversalErr != nil {
				return best
			}
			if !edgeAllowed(TraversalCallFlow, ed.Kind) {
				continue
			}
			next := ed.DstID
			if pathVisited[next] {
				continue
			}
			isBridge := ed.Kind == codebase.EdgeInterfaceDispatch || ed.Provenance == codebase.ProvenanceHeuristic
			if isBridge && usedBridges >= maxBridges {
				sawCappedBridge = true
				continue
			}
			if !distinctVisited[next] &&
				int64(len(distinctVisited)) >= budget.VisitBudget() {
				hitVisitBudget = true
				continue
			}
			distinctVisited[next] = true
			pathVisited[next] = true
			nextDepth := depth + 1
			if nextDepth > maxDepthObserved {
				maxDepthObserved = nextDepth
			}
			sub := dfs(next, depth+1, usedBridges+boolInt(isBridge))
			pathVisited[next] = false

			cand := chainSuffix{
				path: append(
					[]chainHop{{
						NodeID:     next,
						ViaKind:    ed.Kind,
						Provenance: ed.Provenance,
					}},
					sub.path...,
				),
				bridges:     boolInt(isBridge) + sub.bridges,
				termination: sub.termination,
			}
			// Prefer the more INFORMATIVE spine: a flow that crosses an interface
			// boundary (the moat — reasoning across dynamic dispatch) is worth a
			// couple of static hops, but a much deeper static flow still wins.
			if chainScore(cand) > chainScore(best) {
				best = cand
			}
		}
		if len(best.path) == 0 && sawCappedBridge {
			best.termination = chainTerminations["unresolved_dispatch_boundary"]
		}
		return best
	}

	tail := dfs(seedID, 0, 0)
	if traversalErr != nil {
		return ChainOutcome{}, traversalErr
	}
	if hitVisitBudget {
		tail.termination = chainTerminations["visit_budget_reached"]
	}
	if tail.termination.String() == "leaf_reached" &&
		!scope.SupportsKnownAbsence() {
		tail.termination = chainTerminations["coverage_incomplete"]
	}
	chain := append([]chainHop{{NodeID: seedID}}, tail.path...)
	stats, err := NewTraversalStats(
		scope,
		budget,
		int64(len(distinctVisited)),
		inspectedEdges,
		maxDepthObserved,
		int64(tail.bridges),
	)
	if err != nil {
		return ChainOutcome{}, err
	}
	graphHops, err := graphHopsFromChain(chain)
	if err != nil {
		return ChainOutcome{}, err
	}
	return NewChainOutcome(seed, graphHops, tail.termination, stats)
}

// sortChainEdges orders a node's out-edges deterministically: resolved static
// calls before heuristic dispatch (so the spine prefers solid ground), then by
// destination id for stable output.
func sortChainEdges(edges []codebase.CodeEdge) {
	sort.SliceStable(edges, func(i, j int) bool {
		bi := edges[i].Provenance == codebase.ProvenanceHeuristic
		bj := edges[j].Provenance == codebase.ProvenanceHeuristic
		if bi != bj {
			return !bi // static first
		}
		return edges[i].DstID < edges[j].DstID
	})
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// chainScore values a candidate suffix: its length plus a bonus per interface
// boundary crossed, so a cross-interface flow beats a marginally-longer static
// one without letting a single bridge outweigh a genuinely deeper static flow.
func chainScore(s chainSuffix) int {
	return len(s.path) + 2*s.bridges
}
