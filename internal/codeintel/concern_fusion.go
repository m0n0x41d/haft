package codeintel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graphrank"
	"github.com/m0n0x41d/haft/internal/textsearch"
)

const (
	ConcernSeedCodeLexical       = "code_lexical"
	ConcernSeedReasoningArtifact = "reasoning_artifact"
	ConcernSeedTypedMemoryExact  = "typed_memory_exact"

	ConcernExternalSymbolAnchor = "symbol_anchor"
	ConcernExternalArtifactRef  = "artifact_ref"

	ConcernLexicalPresent = "present"
	ConcernLexicalAbsent  = "absent"
	ConcernGraphPresent   = "present"
	ConcernGraphAbsent    = "absent"

	ConcernBridgeExactSymbol  = "exact_binding"
	ConcernBridgeAffectedFile = "affected_path_context"

	ConcernLaneASTSymbol         = "ast_symbol"
	ConcernLaneCodeLexical       = "code_lexical"
	ConcernLaneGraphProximity    = "graph_proximity"
	ConcernLaneExactSymbolBridge = "exact_binding"
	ConcernLaneFileBridge        = "affected_path_context"
	ConcernLaneStaticCall        = "static_call"
	ConcernLaneHeuristicDispatch = "heuristic_dispatch"
	ConcernLaneTestExercise      = "test_exercise"

	concernArtifactSeedLimit = 12
	concernEdgeDisplayLimit  = 5
	concernGraphProducerMin  = 64
	concernGraphProducerCap  = 400
	concernGraphMaxHops      = 4
	concernGraphMaxNodes     = 5000
)

// ConcernSeedOrigin is a closed origin lane. It remains attached to the
// normalized restart mass until the final graphrank adapter.
type ConcernSeedOrigin struct {
	code string
}

func (o ConcernSeedOrigin) String() string {
	return o.code
}

// ConcernSeed is one typed restart input. SourceRef names the lexical or exact
// carrier that produced the graph node; Weight is normalized across active
// origin lanes and never doubles as authority or applicability.
type ConcernSeed struct {
	nodeID    string
	origin    ConcernSeedOrigin
	sourceRef string
	lane      string
	weight    float64
}

func (s ConcernSeed) NodeID() string {
	return s.nodeID
}

func (s ConcernSeed) Origin() ConcernSeedOrigin {
	return s.origin
}

func (s ConcernSeed) SourceRef() string {
	return s.sourceRef
}

func (s ConcernSeed) Lane() string {
	return s.lane
}

func (s ConcernSeed) Weight() float64 {
	return s.weight
}

func (s ConcernSeed) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		NodeID    string  `json:"node_id"`
		Origin    string  `json:"origin"`
		SourceRef string  `json:"source_ref"`
		Lane      string  `json:"lane"`
		Weight    float64 `json:"weight"`
	}{
		NodeID:    s.nodeID,
		Origin:    s.origin.String(),
		SourceRef: s.sourceRef,
		Lane:      s.lane,
		Weight:    s.weight,
	})
}

// ConcernSeedSet owns normalization and is the only value allowed to lower
// typed origins into graphrank's legacy map[string]float64 port.
type ConcernSeedSet struct {
	seeds []ConcernSeed
}

func (s ConcernSeedSet) Items() []ConcernSeed {
	return append([]ConcernSeed{}, s.seeds...)
}

func (s ConcernSeedSet) RestartDistribution() map[string]float64 {
	distribution := make(map[string]float64, len(s.seeds))
	for _, seed := range s.seeds {
		distribution[seed.nodeID] += seed.weight
	}
	return distribution
}

func (s ConcernSeedSet) NodeIDs() []string {
	ids := make([]string, 0, len(s.seeds))
	for _, seed := range s.seeds {
		ids = append(ids, seed.nodeID)
	}
	return stableUniqueStrings(ids)
}

func (s ConcernSeedSet) distributionForOrigin(
	origin ConcernSeedOrigin,
) (map[string]float64, float64) {
	distribution := make(map[string]float64)
	laneMass := 0.0
	for _, seed := range s.seeds {
		if seed.origin.code != origin.code {
			continue
		}
		distribution[seed.nodeID] += seed.weight
		laneMass += seed.weight
	}
	return distribution, laneMass
}

type unweightedConcernSeed struct {
	nodeID    string
	origin    ConcernSeedOrigin
	sourceRef string
	lane      string
}

func newConcernSeedSet(inputs []unweightedConcernSeed) ConcernSeedSet {
	unique := make([]unweightedConcernSeed, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		if strings.TrimSpace(input.nodeID) == "" ||
			strings.TrimSpace(input.origin.code) == "" {
			continue
		}
		key := input.origin.code + "\x00" + input.nodeID
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, input)
	}
	sort.SliceStable(unique, func(left, right int) bool {
		if unique[left].origin.code != unique[right].origin.code {
			return unique[left].origin.code < unique[right].origin.code
		}
		if unique[left].nodeID != unique[right].nodeID {
			return unique[left].nodeID < unique[right].nodeID
		}
		return unique[left].sourceRef < unique[right].sourceRef
	})
	counts := make(map[string]int)
	for _, input := range unique {
		counts[input.origin.code]++
	}
	originCount := len(counts)
	if originCount == 0 {
		return ConcernSeedSet{}
	}
	seeds := make([]ConcernSeed, 0, len(unique))
	for _, input := range unique {
		originMass := 1.0 / float64(originCount)
		weight := originMass / float64(counts[input.origin.code])
		seeds = append(seeds, ConcernSeed{
			nodeID:    input.nodeID,
			origin:    input.origin,
			sourceRef: input.sourceRef,
			lane:      input.lane,
			weight:    weight,
		})
	}
	return ConcernSeedSet{seeds: seeds}
}

// ConcernExternalSeed is the future-proof exact-seed seam. It accepts only a
// durable symbol anchor or canonical reasoning-artifact reference.
type ConcernExternalSeed struct {
	kind      string
	reference string
	sourceRef string
	lane      string
}

func NewConcernSymbolAnchorSeed(
	anchorID string,
	sourceRef string,
	lane string,
) (ConcernExternalSeed, error) {
	if strings.TrimSpace(anchorID) == "" {
		return ConcernExternalSeed{}, fmt.Errorf(
			"external concern symbol anchor is required",
		)
	}
	return ConcernExternalSeed{
		kind:      ConcernExternalSymbolAnchor,
		reference: anchorID,
		sourceRef: sourceRef,
		lane:      lane,
	}, nil
}

func NewConcernArtifactSeed(
	artifactRef string,
	sourceRef string,
	lane string,
) (ConcernExternalSeed, error) {
	if !artifact.IsArtifactID(strings.TrimSpace(artifactRef)) {
		return ConcernExternalSeed{}, fmt.Errorf(
			"external concern artifact reference is not canonical",
		)
	}
	return ConcernExternalSeed{
		kind:      ConcernExternalArtifactRef,
		reference: artifactRef,
		sourceRef: sourceRef,
		lane:      lane,
	}, nil
}

type ConcernExactSeedBatch struct {
	seeds []ConcernExternalSeed
}

func NewConcernExactSeedBatch(
	seeds []ConcernExternalSeed,
) ConcernExactSeedBatch {
	return ConcernExactSeedBatch{
		seeds: append([]ConcernExternalSeed{}, seeds...),
	}
}

func (b ConcernExactSeedBatch) Items() []ConcernExternalSeed {
	return append([]ConcernExternalSeed{}, b.seeds...)
}

type ConcernSeedRequest struct {
	Query codebase.ConcernQuery
	Index codebase.IndexState
}

// ConcernExactSeedSource is optional. A proven typed-memory runtime may supply
// exact existing references here later; core discovery never depends on it.
type ConcernExactSeedSource interface {
	ExactConcernSeeds(
		context.Context,
		ConcernSeedRequest,
	) (ConcernExactSeedBatch, error)
}

// NoConcernExactSeedSource is the explicit null adapter used by NewService.
type NoConcernExactSeedSource struct{}

func (NoConcernExactSeedSource) ExactConcernSeeds(
	context.Context,
	ConcernSeedRequest,
) (ConcernExactSeedBatch, error) {
	return NewConcernExactSeedBatch(nil), nil
}

type ConcernLexicalSupport struct {
	kind      string
	candidate codebase.SymbolDiscoveryCandidate
}

func lexicalConcernSupport(
	candidate codebase.SymbolDiscoveryCandidate,
) ConcernLexicalSupport {
	return ConcernLexicalSupport{
		kind:      ConcernLexicalPresent,
		candidate: candidate,
	}
}

func noLexicalConcernSupport() ConcernLexicalSupport {
	return ConcernLexicalSupport{kind: ConcernLexicalAbsent}
}

func (s ConcernLexicalSupport) Kind() string {
	return s.kind
}

func (s ConcernLexicalSupport) Candidate() (
	codebase.SymbolDiscoveryCandidate,
	bool,
) {
	return s.candidate, s.kind == ConcernLexicalPresent
}

func (s ConcernLexicalSupport) MarshalJSON() ([]byte, error) {
	if s.kind == ConcernLexicalPresent {
		return json.Marshal(struct {
			Kind      string                            `json:"kind"`
			Candidate codebase.SymbolDiscoveryCandidate `json:"candidate"`
		}{
			Kind:      s.kind,
			Candidate: s.candidate,
		})
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
	}{
		Kind: ConcernLexicalAbsent,
	})
}

type ConcernGraphSupport struct {
	kind           string
	combined       float64
	codeLexical    float64
	reasoning      float64
	typedMemory    float64
	restartOrigins []string
}

func (s ConcernGraphSupport) Kind() string {
	return s.kind
}

func (s ConcernGraphSupport) Combined() float64 {
	return s.combined
}

func (s ConcernGraphSupport) CodeLexical() float64 {
	return s.codeLexical
}

func (s ConcernGraphSupport) Reasoning() float64 {
	return s.reasoning
}

func (s ConcernGraphSupport) TypedMemory() float64 {
	return s.typedMemory
}

func (s ConcernGraphSupport) RestartOrigins() []string {
	return append([]string{}, s.restartOrigins...)
}

func (s ConcernGraphSupport) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string   `json:"kind"`
		Combined       float64  `json:"combined,omitempty"`
		CodeLexical    float64  `json:"code_lexical,omitempty"`
		Reasoning      float64  `json:"reasoning,omitempty"`
		TypedMemory    float64  `json:"typed_memory,omitempty"`
		RestartOrigins []string `json:"restart_origins,omitempty"`
	}{
		Kind:           s.kind,
		Combined:       s.combined,
		CodeLexical:    s.codeLexical,
		Reasoning:      s.reasoning,
		TypedMemory:    s.typedMemory,
		RestartOrigins: s.RestartOrigins(),
	})
}

type ConcernArtifactSupport struct {
	ArtifactRef string `json:"artifact_ref"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Lane        string `json:"lane"`
	Relation    string `json:"relation"`
	Seeded      bool   `json:"seeded"`
}

type ConcernEdgeSupport struct {
	Direction  string              `json:"direction"`
	Peer       codebase.CodeSymbol `json:"peer"`
	Kind       codebase.EdgeKind   `json:"kind"`
	Provenance codebase.Provenance `json:"provenance"`
}

type ConcernCallEvidence struct {
	Incoming         []ConcernEdgeSupport      `json:"incoming"`
	Outgoing         []ConcernEdgeSupport      `json:"outgoing"`
	OutgoingCoverage codebase.ResolutionCounts `json:"outgoing_coverage"`
}

type ConcernArtifactRef struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

type ConcernInvariantRef struct {
	DecisionRef string `json:"decision_ref"`
	Text        string `json:"text"`
	ContextOnly bool   `json:"context_only"`
}

type ConcernSpecRef struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Resolution string `json:"resolution"`
}

type ConcernGovernance struct {
	Decisions                       []ConcernArtifactRef  `json:"decisions"`
	ExactBindingDecisionRefs        []string              `json:"exact_binding_decision_refs"`
	AffectedPathContextDecisionRefs []string              `json:"affected_path_context_decision_refs"`
	Problems                        []ConcernArtifactRef  `json:"problems"`
	Alternatives                    []ConcernArtifactRef  `json:"alternatives"`
	Notes                           []ConcernArtifactRef  `json:"notes"`
	Specs                           []ConcernSpecRef      `json:"specs"`
	Invariants                      []ConcernInvariantRef `json:"invariants"`
	ModuleDecisionRefs              []string              `json:"module_decision_refs"`
	SymbolGranularity               string                `json:"symbol_granularity"`
}

// ConcernCandidate keeps every evidence component separate. Candidate order is
// advisory; there is intentionally no selected/winner field.
type ConcernCandidate struct {
	symbol              codebase.CodeSymbol
	sourceLane          string
	directBridge        string
	directNameTermMatch bool
	lexical             ConcernLexicalSupport
	graph               ConcernGraphSupport
	originLanes         []string
	artifacts           []ConcernArtifactSupport
	governance          ConcernGovernance
	calls               ConcernCallEvidence
	epoch               int64
}

func (c ConcernCandidate) Symbol() codebase.CodeSymbol {
	return c.symbol
}

func (c ConcernCandidate) SourceLane() string {
	return c.sourceLane
}

func (c ConcernCandidate) DirectBridge() string {
	return c.directBridge
}

func (c ConcernCandidate) Lexical() ConcernLexicalSupport {
	return c.lexical
}

func (c ConcernCandidate) Graph() ConcernGraphSupport {
	return c.graph
}

func (c ConcernCandidate) OriginLanes() []string {
	return append([]string{}, c.originLanes...)
}

func (c ConcernCandidate) Artifacts() []ConcernArtifactSupport {
	return append([]ConcernArtifactSupport{}, c.artifacts...)
}

func (c ConcernCandidate) Governance() ConcernGovernance {
	return c.governance
}

func (c ConcernCandidate) Calls() ConcernCallEvidence {
	return c.calls
}

func (c ConcernCandidate) Epoch() int64 {
	return c.epoch
}

func (c ConcernCandidate) MarshalJSON() ([]byte, error) {
	if c.symbol.AnchorID == "" || c.epoch < 1 {
		return nil, fmt.Errorf("marshal invalid fused concern candidate")
	}
	return json.Marshal(struct {
		Symbol       codebase.CodeSymbol      `json:"symbol"`
		SourceLane   string                   `json:"source_lane"`
		DirectBridge string                   `json:"direct_bridge,omitempty"`
		Lexical      ConcernLexicalSupport    `json:"lexical"`
		Graph        ConcernGraphSupport      `json:"graph"`
		OriginLanes  []string                 `json:"origin_lanes"`
		Artifacts    []ConcernArtifactSupport `json:"reasoning_artifacts"`
		Governance   ConcernGovernance        `json:"governance"`
		Calls        ConcernCallEvidence      `json:"call_evidence"`
		Epoch        int64                    `json:"epoch"`
	}{
		Symbol:       c.symbol,
		SourceLane:   c.sourceLane,
		DirectBridge: c.directBridge,
		Lexical:      c.lexical,
		Graph:        c.graph,
		OriginLanes:  c.OriginLanes(),
		Artifacts:    c.Artifacts(),
		Governance:   c.governance,
		Calls:        c.calls,
		Epoch:        c.epoch,
	})
}

type ConcernCandidateBatch struct {
	candidates []ConcernCandidate
	budget     codebase.DiscoveryBudget
}

func (b ConcernCandidateBatch) Candidates() []ConcernCandidate {
	return append([]ConcernCandidate{}, b.candidates...)
}

func (b ConcernCandidateBatch) Budget() codebase.DiscoveryBudget {
	return b.budget
}

type ConcernFusionBasis struct {
	Schema                 string           `json:"schema"`
	CodeEpoch              int64            `json:"code_epoch"`
	GraphDigest            string           `json:"graph_digest"`
	QueryDigest            string           `json:"query_digest"`
	ReplayRef              string           `json:"replay_ref"`
	FullGraphNodes         int              `json:"full_graph_nodes"`
	GraphNodes             int              `json:"induced_graph_nodes"`
	GraphInductionMaxHops  int              `json:"graph_induction_max_hops"`
	GraphInductionMaxNodes int              `json:"graph_induction_max_nodes"`
	GraphNodeCapReached    bool             `json:"graph_node_cap_reached"`
	CodeEdges              int              `json:"code_edges"`
	ReasoningLinks         int              `json:"reasoning_links"`
	AffectedFileBridges    int              `json:"affected_file_bridges"`
	ExactSymbolBridges     int              `json:"exact_symbol_bridges"`
	ModuleContextBridges   int              `json:"module_context_bridges"`
	StaleSymbolBindings    int              `json:"stale_symbol_bindings"`
	ArtifactSeedLimit      int              `json:"artifact_seed_limit"`
	AppliedCandidateBudget int              `json:"applied_candidate_budget"`
	PPR                    graphrank.Params `json:"ppr"`
	Seeds                  []ConcernSeed    `json:"seeds"`
}

type concernFusionInputs struct {
	artifacts []*artifact.Artifact
	edges     []codebase.CodeEdge
	links     []artifact.LinkEdge
	affected  []artifact.AffectedFileRef
	bindings  []artifact.SymbolBindingRef
	symbols   []codebase.CodeSymbol
	refs      []codebase.SymbolRef
	modules   moduleFusionInputs
	external  []ConcernExternalSeed
}

type concernGraphScores struct {
	combined    map[string]float64
	codeLexical map[string]float64
	reasoning   map[string]float64
	typedMemory map[string]float64
	origins     []string
}

func scoreConcernGraph(
	graph *graphrank.Graph,
	seeds ConcernSeedSet,
) concernGraphScores {
	origins := []ConcernSeedOrigin{
		{code: ConcernSeedCodeLexical},
		{code: ConcernSeedReasoningArtifact},
		{code: ConcernSeedTypedMemoryExact},
	}
	result := concernGraphScores{
		combined:    make(map[string]float64),
		codeLexical: make(map[string]float64),
		reasoning:   make(map[string]float64),
		typedMemory: make(map[string]float64),
	}
	for _, origin := range origins {
		distribution, laneMass := seeds.distributionForOrigin(origin)
		if laneMass == 0 {
			continue
		}
		scores := graph.Rank(distribution, graphrank.DefaultParams())
		result.origins = append(result.origins, origin.String())
		for nodeID, score := range scores {
			contribution := score * laneMass
			result.combined[nodeID] += contribution
			switch origin.code {
			case ConcernSeedCodeLexical:
				result.codeLexical[nodeID] = contribution
			case ConcernSeedReasoningArtifact:
				result.reasoning[nodeID] = contribution
			case ConcernSeedTypedMemoryExact:
				result.typedMemory[nodeID] = contribution
			}
		}
	}
	return result
}

func concernGraphSupportFor(
	scores concernGraphScores,
	symbolID string,
) ConcernGraphSupport {
	combined := scores.combined[symbolID]
	if combined <= 0 {
		return ConcernGraphSupport{kind: ConcernGraphAbsent}
	}
	return ConcernGraphSupport{
		kind:           ConcernGraphPresent,
		combined:       combined,
		codeLexical:    scores.codeLexical[symbolID],
		reasoning:      scores.reasoning[symbolID],
		typedMemory:    scores.typedMemory[symbolID],
		restartOrigins: append([]string{}, scores.origins...),
	}
}

func (s *Service) fuseConcernCandidates(
	ctx context.Context,
	query codebase.ConcernQuery,
	lexical codebase.SymbolDiscoveryBatch,
	budget codebase.DiscoveryBudget,
	index codebase.IndexState,
) (ConcernCandidateBatch, ConcernFusionBasis, error) {
	inputs, err := s.loadConcernFusionInputs(ctx, query, index)
	if err != nil {
		return ConcernCandidateBatch{}, ConcernFusionBasis{}, err
	}
	seeds := concernSeeds(lexical, inputs)
	fullGraph, kinds := buildFusedGraph(
		inputs.edges,
		inputs.links,
		inputs.affected,
		inputs.bindings,
		inputs.refs,
		inputs.modules,
	)
	_ = kinds
	inductionBudget, err := graphrank.NewInductionBudget(
		concernGraphMaxHops,
		concernGraphMaxNodes,
	)
	if err != nil {
		return ConcernCandidateBatch{}, ConcernFusionBasis{}, err
	}
	projection := fullGraph.Induce(
		seeds.NodeIDs(),
		inductionBudget,
	)
	scores := scoreConcernGraph(projection.Graph, seeds)
	candidates := assembleConcernCandidates(
		query,
		lexical,
		inputs,
		scores,
		index.Epoch,
	)
	sortConcernCandidates(candidates)
	if len(candidates) > budget.MaxCandidates() {
		candidates = candidates[:budget.MaxCandidates()]
	}
	for index := range candidates {
		hydrated, err := s.hydrateConcernCandidate(
			ctx,
			candidates[index],
			inputs,
		)
		if err != nil {
			return ConcernCandidateBatch{}, ConcernFusionBasis{}, err
		}
		candidates[index] = hydrated
	}
	basis, err := concernFusionBasis(
		query,
		budget,
		index,
		inputs,
		seeds,
		fullGraph.Len(),
		projection.Graph.Len(),
		projection.NodeCapReached,
		candidates,
	)
	if err != nil {
		return ConcernCandidateBatch{}, ConcernFusionBasis{}, err
	}
	return ConcernCandidateBatch{
		candidates: candidates,
		budget:     budget,
	}, basis, nil
}

func (s *Service) loadConcernFusionInputs(
	ctx context.Context,
	query codebase.ConcernQuery,
	index codebase.IndexState,
) (concernFusionInputs, error) {
	artifacts, err := s.art.Search(
		ctx,
		query.Raw(),
		concernArtifactSeedLimit,
	)
	if err != nil {
		return concernFusionInputs{}, fmt.Errorf(
			"retrieve reasoning-artifact concern seeds: %w",
			err,
		)
	}
	edges, err := s.edges.AllEdges(ctx)
	if err != nil {
		return concernFusionInputs{}, err
	}
	links, err := s.art.AllLinks(ctx)
	if err != nil {
		return concernFusionInputs{}, err
	}
	affected, err := s.art.AllAffectedFiles(ctx)
	if err != nil {
		return concernFusionInputs{}, err
	}
	bindings, err := s.art.AllActiveSymbolBindingRefs(ctx)
	if err != nil {
		return concernFusionInputs{}, err
	}
	symbols, err := s.symbols.AllSymbols(ctx)
	if err != nil {
		return concernFusionInputs{}, err
	}
	refs, err := s.symbols.AllSymbolRefs(ctx)
	if err != nil {
		return concernFusionInputs{}, err
	}
	moduleFusion, err := s.loadModuleFusionInputs(ctx, refs)
	if err != nil {
		return concernFusionInputs{}, err
	}
	external, err := s.seeds.ExactConcernSeeds(
		ctx,
		ConcernSeedRequest{Query: query, Index: index},
	)
	if err != nil {
		return concernFusionInputs{}, fmt.Errorf(
			"retrieve optional exact concern seeds: %w",
			err,
		)
	}
	return concernFusionInputs{
		artifacts: artifacts,
		edges:     edges,
		links:     links,
		affected:  affected,
		bindings:  bindings,
		symbols:   symbols,
		refs:      refs,
		modules:   moduleFusion,
		external:  external.Items(),
	}, nil
}

func concernSeeds(
	lexical codebase.SymbolDiscoveryBatch,
	inputs concernFusionInputs,
) ConcernSeedSet {
	seeds := make([]unweightedConcernSeed, 0)
	for _, candidate := range lexical.Candidates() {
		symbol := candidate.Symbol()
		seeds = append(seeds, unweightedConcernSeed{
			nodeID:    symbol.ID,
			origin:    ConcernSeedOrigin{code: ConcernSeedCodeLexical},
			sourceRef: symbol.AnchorID,
			lane:      ConcernLaneCodeLexical,
		})
	}
	for _, item := range inputs.artifacts {
		seeds = append(seeds, unweightedConcernSeed{
			nodeID:    item.Meta.ID,
			origin:    ConcernSeedOrigin{code: ConcernSeedReasoningArtifact},
			sourceRef: item.Meta.ID,
			lane:      artifactConcernLane(item.Meta.Kind),
		})
	}
	symbolByAnchor := make(map[string]string, len(inputs.refs))
	for _, ref := range inputs.refs {
		if ref.AnchorID != "" {
			symbolByAnchor[ref.AnchorID] = ref.ID
		}
	}
	for _, external := range inputs.external {
		nodeID := ""
		switch external.kind {
		case ConcernExternalSymbolAnchor:
			nodeID = symbolByAnchor[external.reference]
		case ConcernExternalArtifactRef:
			nodeID = external.reference
		}
		seeds = append(seeds, unweightedConcernSeed{
			nodeID:    nodeID,
			origin:    ConcernSeedOrigin{code: ConcernSeedTypedMemoryExact},
			sourceRef: external.sourceRef,
			lane:      external.lane,
		})
	}
	return newConcernSeedSet(seeds)
}

func assembleConcernCandidates(
	query codebase.ConcernQuery,
	lexical codebase.SymbolDiscoveryBatch,
	inputs concernFusionInputs,
	scores concernGraphScores,
	epoch int64,
) []ConcernCandidate {
	lexicalByID := make(map[string]codebase.SymbolDiscoveryCandidate)
	for _, candidate := range lexical.Candidates() {
		lexicalByID[candidate.Symbol().ID] = candidate
	}
	symbolByID := make(map[string]codebase.CodeSymbol, len(inputs.symbols))
	for _, symbol := range inputs.symbols {
		symbolByID[symbol.ID] = symbol
	}
	ids := make([]string, 0, len(scores.combined)+len(lexicalByID))
	seen := make(map[string]bool)
	for symbolID := range lexicalByID {
		ids = append(ids, symbolID)
		seen[symbolID] = true
	}
	if lexicalBatchHasExactIdentity(lexical) {
		return concernCandidatesForIDs(
			query,
			ids,
			symbolByID,
			lexicalByID,
			scores,
			inputs,
			epoch,
		)
	}
	graphIDs := make([]string, 0, len(scores.combined))
	for symbolID, score := range scores.combined {
		symbol, exists := symbolByID[symbolID]
		hasReasoningOrigin := scores.reasoning[symbolID] > 0 ||
			scores.typedMemory[symbolID] > 0
		if !exists ||
			score <= 0 ||
			!hasReasoningOrigin ||
			!matchesConcernFilters(symbol, query) {
			continue
		}
		graphIDs = append(graphIDs, symbolID)
	}
	sort.SliceStable(graphIDs, func(left, right int) bool {
		leftScore := scores.combined[graphIDs[left]]
		rightScore := scores.combined[graphIDs[right]]
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return graphIDs[left] < graphIDs[right]
	})
	producerLimit := concernGraphProducerLimit(len(lexical.Candidates()))
	if len(graphIDs) > producerLimit {
		graphIDs = graphIDs[:producerLimit]
	}
	for _, symbolID := range graphIDs {
		if seen[symbolID] {
			continue
		}
		seen[symbolID] = true
		ids = append(ids, symbolID)
	}
	return concernCandidatesForIDs(
		query,
		ids,
		symbolByID,
		lexicalByID,
		scores,
		inputs,
		epoch,
	)
}

func concernCandidatesForIDs(
	query codebase.ConcernQuery,
	ids []string,
	symbolByID map[string]codebase.CodeSymbol,
	lexicalByID map[string]codebase.SymbolDiscoveryCandidate,
	scores concernGraphScores,
	inputs concernFusionInputs,
	epoch int64,
) []ConcernCandidate {
	candidates := make([]ConcernCandidate, 0, len(ids))
	for _, symbolID := range ids {
		symbol, exists := symbolByID[symbolID]
		if !exists || symbol.AnchorID == "" {
			continue
		}
		lexicalSupport := noLexicalConcernSupport()
		if evidence, matched := lexicalByID[symbolID]; matched {
			lexicalSupport = lexicalConcernSupport(evidence)
		}
		graphSupport := concernGraphSupportFor(scores, symbolID)
		lanes := []string{ConcernLaneASTSymbol}
		if lexicalSupport.kind == ConcernLexicalPresent {
			lanes = append(lanes, ConcernLaneCodeLexical)
		}
		if graphSupport.kind == ConcernGraphPresent {
			lanes = append(lanes, ConcernLaneGraphProximity)
		}
		directBridge := concernDirectSeedBridge(symbol, inputs)
		candidates = append(candidates, ConcernCandidate{
			symbol:       symbol,
			sourceLane:   concernSymbolSourceLane(symbol.FilePath),
			directBridge: directBridge,
			directNameTermMatch: directBridge == ConcernBridgeExactSymbol &&
				concernSymbolNameMatchesQuery(symbol.Name, query),
			lexical:     lexicalSupport,
			graph:       graphSupport,
			originLanes: stableUniqueStrings(lanes),
			epoch:       epoch,
		})
	}
	return candidates
}

func concernSymbolNameMatchesQuery(
	name string,
	query codebase.ConcernQuery,
) bool {
	nameTerms := textsearch.Terms(
		name,
		textsearch.Options{Stems: true},
	)
	queryTerms := query.Terms()
	for _, nameTerm := range nameTerms {
		for _, queryTerm := range queryTerms {
			if nameTerm == queryTerm {
				return true
			}
		}
	}
	return false
}

func concernDirectSeedBridge(
	symbol codebase.CodeSymbol,
	inputs concernFusionInputs,
) string {
	seeded := make(map[string]bool, len(inputs.artifacts))
	for _, item := range inputs.artifacts {
		seeded[item.Meta.ID] = true
	}
	for _, binding := range inputs.bindings {
		if binding.AnchorID == symbol.AnchorID &&
			seeded[binding.ArtifactID] {
			return ConcernBridgeExactSymbol
		}
	}
	for _, affected := range inputs.affected {
		if affected.FilePath == symbol.FilePath &&
			seeded[affected.ArtifactID] {
			return ConcernBridgeAffectedFile
		}
	}
	return ""
}

func concernSymbolSourceLane(path string) string {
	switch {
	case textsearch.IsGeneratedPath(path):
		return codebase.SymbolLaneGenerated
	case textsearch.IsTestPath(path):
		return codebase.SymbolLaneTest
	default:
		return codebase.SymbolLaneProduction
	}
}

func lexicalBatchHasExactIdentity(
	lexical codebase.SymbolDiscoveryBatch,
) bool {
	candidates := lexical.Candidates()
	if len(candidates) != 1 {
		return false
	}
	tier := candidates[0].Tier().String()
	return tier == codebase.LexicalTierExactStableID ||
		tier == codebase.LexicalTierExactQualifiedName
}

func concernHasExactIdentity(candidates []ConcernCandidate) bool {
	if len(candidates) != 1 {
		return false
	}
	candidate, present := candidates[0].lexical.Candidate()
	if !present {
		return false
	}
	tier := candidate.Tier().String()
	return tier == codebase.LexicalTierExactStableID ||
		tier == codebase.LexicalTierExactQualifiedName
}

func concernGraphProducerLimit(lexicalCount int) int {
	limit := lexicalCount * 8
	if limit < concernGraphProducerMin {
		limit = concernGraphProducerMin
	}
	if limit > concernGraphProducerCap {
		limit = concernGraphProducerCap
	}
	return limit
}

func matchesConcernFilters(
	symbol codebase.CodeSymbol,
	query codebase.ConcernQuery,
) bool {
	checks := []bool{
		matchesEqualFilters(symbol.Kind, query.KindFilters()),
		matchesEqualFilters(symbol.Lang, query.LanguageFilters()),
		matchesContainsFilters(symbol.FilePath, query.PathFilters()),
		matchesContainsFilters(symbol.Name, query.NameFilters()),
	}
	for _, matched := range checks {
		if !matched {
			return false
		}
	}
	return true
}

func matchesEqualFilters(value string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if strings.EqualFold(value, filter) {
			return true
		}
	}
	return false
}

func matchesContainsFilters(value string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	lowerValue := strings.ToLower(value)
	for _, filter := range filters {
		if strings.Contains(lowerValue, strings.ToLower(filter)) {
			return true
		}
	}
	return false
}

func sortConcernCandidates(candidates []ConcernCandidate) {
	sort.SliceStable(candidates, func(left, right int) bool {
		leftCandidate := candidates[left]
		rightCandidate := candidates[right]
		leftTier := concernLexicalTierRank(leftCandidate.lexical)
		rightTier := concernLexicalTierRank(rightCandidate.lexical)
		leftExactLexical := leftTier <= 2
		rightExactLexical := rightTier <= 2
		if leftExactLexical != rightExactLexical {
			return leftExactLexical
		}
		leftExactBridge := leftCandidate.directBridge ==
			ConcernBridgeExactSymbol
		rightExactBridge := rightCandidate.directBridge ==
			ConcernBridgeExactSymbol
		if leftExactBridge != rightExactBridge {
			return leftExactBridge
		}
		if leftExactBridge &&
			leftCandidate.directNameTermMatch !=
				rightCandidate.directNameTermMatch {
			return leftCandidate.directNameTermMatch
		}
		if leftTier != rightTier {
			return leftTier < rightTier
		}
		leftCoverage, leftTotal := concernLexicalCoverage(
			leftCandidate.lexical,
		)
		rightCoverage, rightTotal := concernLexicalCoverage(
			rightCandidate.lexical,
		)
		leftRatio := leftCoverage * rightTotal
		rightRatio := rightCoverage * leftTotal
		if leftRatio != rightRatio {
			return leftRatio > rightRatio
		}
		leftField, leftFieldTotal := concernFieldCoverage(
			leftCandidate.lexical,
		)
		rightField, rightFieldTotal := concernFieldCoverage(
			rightCandidate.lexical,
		)
		leftFieldRatio := leftField * rightFieldTotal
		rightFieldRatio := rightField * leftFieldTotal
		if leftFieldRatio != rightFieldRatio {
			return leftFieldRatio > rightFieldRatio
		}
		if leftCandidate.graph.combined != rightCandidate.graph.combined {
			return leftCandidate.graph.combined >
				rightCandidate.graph.combined
		}
		if len(leftCandidate.originLanes) !=
			len(rightCandidate.originLanes) {
			return len(leftCandidate.originLanes) >
				len(rightCandidate.originLanes)
		}
		if leftCandidate.symbol.AnchorID !=
			rightCandidate.symbol.AnchorID {
			return leftCandidate.symbol.AnchorID <
				rightCandidate.symbol.AnchorID
		}
		if leftCandidate.symbol.FilePath !=
			rightCandidate.symbol.FilePath {
			return leftCandidate.symbol.FilePath <
				rightCandidate.symbol.FilePath
		}
		return leftCandidate.symbol.QualifiedName <
			rightCandidate.symbol.QualifiedName
	})
}

func concernLexicalTierRank(support ConcernLexicalSupport) int {
	candidate, present := support.Candidate()
	if !present {
		return 6
	}
	switch candidate.Tier().String() {
	case codebase.LexicalTierExactStableID:
		return 0
	case codebase.LexicalTierExactQualifiedName:
		return 1
	case codebase.LexicalTierExactName:
		return 2
	case codebase.LexicalTierAllTerms:
		return 3
	case codebase.LexicalTierPartialTerms:
		return 4
	case codebase.LexicalTierEditDistance:
		return 5
	default:
		return 6
	}
}

func concernLexicalCoverage(
	support ConcernLexicalSupport,
) (int, int) {
	candidate, present := support.Candidate()
	if !present {
		return 0, 1
	}
	coverage := candidate.Coverage()
	total := coverage.Total()
	if total < 1 {
		total = 1
	}
	return coverage.Covered(), total
}

func concernFieldCoverage(
	support ConcernLexicalSupport,
) (int, int) {
	candidate, present := support.Candidate()
	if !present {
		return 0, 1
	}
	coverage := candidate.FieldCoverage()
	total := coverage.Total()
	if total < 1 {
		total = 1
	}
	return coverage.Covered(), total
}

func (s *Service) hydrateConcernCandidate(
	ctx context.Context,
	candidate ConcernCandidate,
	inputs concernFusionInputs,
) (ConcernCandidate, error) {
	target := contextgraph.Target{
		File:     candidate.symbol.FilePath,
		Symbol:   candidate.symbol.Name,
		AnchorID: candidate.symbol.AnchorID,
		Line:     candidate.symbol.StartLine,
	}
	codeContext, err := contextgraph.FetchCodeContext(
		ctx,
		s.art,
		s.graph,
		target,
	)
	if err != nil {
		return ConcernCandidate{}, fmt.Errorf(
			"fuse concern governance for %s: %w",
			candidate.symbol.AnchorID,
			err,
		)
	}
	artifactSupports, err := s.directConcernArtifacts(
		ctx,
		candidate.symbol,
		inputs.artifacts,
	)
	if err != nil {
		return ConcernCandidate{}, err
	}
	candidate.artifacts = artifactSupports
	candidate.governance = concernGovernance(codeContext)
	candidate.calls, err = s.concernCallEvidence(
		ctx,
		candidate.symbol,
		inputs,
	)
	if err != nil {
		return ConcernCandidate{}, err
	}
	lanes := append([]string{}, candidate.originLanes...)
	for _, support := range artifactSupports {
		lanes = append(lanes, support.Lane)
		switch support.Relation {
		case ConcernBridgeExactSymbol:
			lanes = append(lanes, ConcernLaneExactSymbolBridge)
		case ConcernBridgeAffectedFile:
			lanes = append(lanes, ConcernLaneFileBridge)
		}
	}
	for _, edge := range append(
		append([]ConcernEdgeSupport{}, candidate.calls.Incoming...),
		candidate.calls.Outgoing...,
	) {
		if textsearch.IsTestPath(edge.Peer.FilePath) {
			lanes = append(lanes, ConcernLaneTestExercise)
		}
		if edge.Provenance == codebase.ProvenanceHeuristic {
			lanes = append(lanes, ConcernLaneHeuristicDispatch)
			continue
		}
		lanes = append(lanes, ConcernLaneStaticCall)
	}
	if textsearch.IsTestPath(candidate.symbol.FilePath) {
		lanes = append(lanes, ConcernLaneTestExercise)
	}
	if len(candidate.governance.Specs) > 0 {
		lanes = append(lanes, "spec")
	}
	if len(candidate.governance.Invariants) > 0 {
		lanes = append(lanes, "invariant")
	}
	candidate.originLanes = stableUniqueStrings(lanes)
	return candidate, nil
}

func (s *Service) directConcernArtifacts(
	ctx context.Context,
	symbol codebase.CodeSymbol,
	seeds []*artifact.Artifact,
) ([]ConcernArtifactSupport, error) {
	byID := make(map[string]ConcernArtifactSupport)
	seeded := make(map[string]bool, len(seeds))
	for _, seed := range seeds {
		seeded[seed.Meta.ID] = true
	}
	fileLinked, err := s.art.SearchByAffectedFile(
		ctx,
		symbol.FilePath,
	)
	if err != nil {
		return nil, err
	}
	for _, item := range fileLinked {
		byID[item.Meta.ID] = concernArtifactSupport(
			item,
			ConcernBridgeAffectedFile,
			seeded[item.Meta.ID],
		)
	}
	anchorLinked, err := s.art.SearchBySymbolAnchor(
		ctx,
		symbol.AnchorID,
	)
	if err != nil {
		return nil, err
	}
	for _, item := range anchorLinked {
		byID[item.Meta.ID] = concernArtifactSupport(
			item,
			ConcernBridgeExactSymbol,
			seeded[item.Meta.ID],
		)
	}
	out := make([]ConcernArtifactSupport, 0, len(byID))
	for _, support := range byID {
		out = append(out, support)
	}
	sort.SliceStable(out, func(left, right int) bool {
		if out[left].Relation != out[right].Relation {
			return out[left].Relation < out[right].Relation
		}
		if out[left].Kind != out[right].Kind {
			return out[left].Kind < out[right].Kind
		}
		return out[left].ArtifactRef < out[right].ArtifactRef
	})
	return out, nil
}

func concernArtifactSupport(
	item *artifact.Artifact,
	relation string,
	seeded bool,
) ConcernArtifactSupport {
	return ConcernArtifactSupport{
		ArtifactRef: item.Meta.ID,
		Title:       item.Meta.Title,
		Kind:        string(item.Meta.Kind),
		Lane:        artifactConcernLane(item.Meta.Kind),
		Relation:    relation,
		Seeded:      seeded,
	}
}

func artifactConcernLane(kind artifact.Kind) string {
	switch kind {
	case artifact.KindDecisionRecord:
		return "decision"
	case artifact.KindProblemCard:
		return "problem"
	case artifact.KindSolutionPortfolio:
		return "alternative"
	case artifact.KindEvidencePack:
		return "evidence"
	case artifact.KindNote:
		return "note"
	case artifact.KindWorkCommission, artifact.KindMethodRun:
		return "work"
	case artifact.KindRefreshReport:
		return "freshness"
	default:
		return "reasoning_artifact"
	}
}

func concernGovernance(
	codeContext contextgraph.CodeContext,
) ConcernGovernance {
	invariants := make([]ConcernInvariantRef, 0)
	for _, invariant := range codeContext.Invariants {
		invariants = append(invariants, ConcernInvariantRef{
			DecisionRef: invariant.DecisionID,
			Text:        invariant.Text,
		})
	}
	for _, invariant := range codeContext.ContextInvariants {
		invariants = append(invariants, ConcernInvariantRef{
			DecisionRef: invariant.DecisionID,
			Text:        invariant.Text,
			ContextOnly: true,
		})
	}
	specs := make([]ConcernSpecRef, 0, len(codeContext.Specs))
	for _, spec := range codeContext.Specs {
		specs = append(specs, ConcernSpecRef{
			ID:         spec.ID,
			Title:      spec.Title,
			Resolution: string(spec.Resolution),
		})
	}
	moduleRefs := make([]string, 0, len(codeContext.ModuleDecisions))
	for _, decision := range codeContext.ModuleDecisions {
		moduleRefs = append(moduleRefs, decision.ID)
	}
	return ConcernGovernance{
		Decisions: concernArtifactRefs(codeContext.Decisions),
		ExactBindingDecisionRefs: concernArtifactIDs(
			codeContext.ExactBindingDecisions,
		),
		AffectedPathContextDecisionRefs: concernArtifactIDs(
			codeContext.AffectedPathContextDecisions,
		),
		Problems:           concernArtifactRefs(codeContext.Problems),
		Alternatives:       concernArtifactRefs(codeContext.Portfolios),
		Notes:              concernArtifactRefs(codeContext.Notes),
		Specs:              specs,
		Invariants:         invariants,
		ModuleDecisionRefs: stableUniqueStrings(moduleRefs),
		SymbolGranularity:  codeContext.SymbolGranularity,
	}
}

func concernArtifactIDs(items []*artifact.Artifact) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, item.Meta.ID)
	}
	return stableUniqueStrings(result)
}

func concernArtifactRefs(
	items []*artifact.Artifact,
) []ConcernArtifactRef {
	out := make([]ConcernArtifactRef, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, ConcernArtifactRef{
			ID:    item.Meta.ID,
			Kind:  string(item.Meta.Kind),
			Title: item.Meta.Title,
		})
	}
	return out
}

func (s *Service) concernCallEvidence(
	ctx context.Context,
	symbol codebase.CodeSymbol,
	inputs concernFusionInputs,
) (ConcernCallEvidence, error) {
	symbols := make(map[string]codebase.CodeSymbol, len(inputs.symbols))
	for _, item := range inputs.symbols {
		symbols[item.ID] = item
	}
	incoming := make([]ConcernEdgeSupport, 0)
	outgoing := make([]ConcernEdgeSupport, 0)
	for _, edge := range inputs.edges {
		switch {
		case edge.DstID == symbol.ID:
			peer, exists := symbols[edge.SrcID]
			if !exists {
				continue
			}
			incoming = append(incoming, concernEdgeSupport(
				"incoming",
				peer,
				edge,
			))
		case edge.SrcID == symbol.ID:
			peer, exists := symbols[edge.DstID]
			if !exists {
				continue
			}
			outgoing = append(outgoing, concernEdgeSupport(
				"outgoing",
				peer,
				edge,
			))
		}
	}
	if len(incoming) > concernEdgeDisplayLimit {
		incoming = incoming[:concernEdgeDisplayLimit]
	}
	if len(outgoing) > concernEdgeDisplayLimit {
		outgoing = outgoing[:concernEdgeDisplayLimit]
	}
	coverage, err := s.edges.ResolutionCountsForSource(
		ctx,
		symbol.ID,
	)
	if err != nil {
		return ConcernCallEvidence{}, err
	}
	return ConcernCallEvidence{
		Incoming:         incoming,
		Outgoing:         outgoing,
		OutgoingCoverage: coverage,
	}, nil
}

func concernEdgeSupport(
	direction string,
	peer codebase.CodeSymbol,
	edge codebase.CodeEdge,
) ConcernEdgeSupport {
	return ConcernEdgeSupport{
		Direction:  direction,
		Peer:       peer,
		Kind:       edge.Kind,
		Provenance: edge.Provenance,
	}
}

func concernFusionBasis(
	query codebase.ConcernQuery,
	budget codebase.DiscoveryBudget,
	index codebase.IndexState,
	inputs concernFusionInputs,
	seeds ConcernSeedSet,
	fullGraphNodes int,
	graphNodes int,
	graphNodeCapReached bool,
	candidates []ConcernCandidate,
) (ConcernFusionBasis, error) {
	currentAnchors := make(map[string]bool, len(inputs.refs))
	for _, ref := range inputs.refs {
		if ref.AnchorID != "" {
			currentAnchors[ref.AnchorID] = true
		}
	}
	currentBindings := 0
	staleBindings := 0
	for _, binding := range inputs.bindings {
		if currentAnchors[binding.AnchorID] {
			currentBindings++
			continue
		}
		staleBindings++
	}
	graphDigest, err := concernGraphMaterialDigest(index, inputs)
	if err != nil {
		return ConcernFusionBasis{}, err
	}
	queryMaterial := struct {
		Query  codebase.ConcernQuery
		Budget int
		Epoch  int64
	}{
		Query:  query,
		Budget: budget.MaxCandidates(),
		Epoch:  index.Epoch,
	}
	queryDigest, err := digestConcernValue(queryMaterial)
	if err != nil {
		return ConcernFusionBasis{}, err
	}
	candidateAnchors := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidateAnchors = append(
			candidateAnchors,
			candidate.symbol.AnchorID,
		)
	}
	ppr := graphrank.DefaultParams()
	replayDigest, err := digestConcernValue(struct {
		Schema                 string
		Graph                  string
		Query                  string
		GraphInductionMaxHops  int
		GraphInductionMaxNodes int
		GraphNodeCapReached    bool
		PPR                    graphrank.Params
		Seeds                  []ConcernSeed
		Candidates             []string
	}{
		Schema:                 "haft.concern-fusion-replay/v1",
		Graph:                  graphDigest,
		Query:                  queryDigest,
		GraphInductionMaxHops:  concernGraphMaxHops,
		GraphInductionMaxNodes: concernGraphMaxNodes,
		GraphNodeCapReached:    graphNodeCapReached,
		PPR:                    ppr,
		Seeds:                  seeds.Items(),
		Candidates:             candidateAnchors,
	})
	if err != nil {
		return ConcernFusionBasis{}, err
	}
	return ConcernFusionBasis{
		Schema:                 "haft.concern-fusion-basis/v1",
		CodeEpoch:              index.Epoch,
		GraphDigest:            graphDigest,
		QueryDigest:            queryDigest,
		ReplayRef:              "concern:v1:" + replayDigest,
		FullGraphNodes:         fullGraphNodes,
		GraphNodes:             graphNodes,
		GraphInductionMaxHops:  concernGraphMaxHops,
		GraphInductionMaxNodes: concernGraphMaxNodes,
		GraphNodeCapReached:    graphNodeCapReached,
		CodeEdges:              len(inputs.edges),
		ReasoningLinks:         len(inputs.links),
		AffectedFileBridges:    len(inputs.affected),
		ExactSymbolBridges:     currentBindings,
		ModuleContextBridges:   len(inputs.modules.Contexts),
		StaleSymbolBindings:    staleBindings,
		ArtifactSeedLimit:      concernArtifactSeedLimit,
		AppliedCandidateBudget: budget.MaxCandidates(),
		PPR:                    ppr,
		Seeds:                  seeds.Items(),
	}, nil
}

type concernArtifactDigestRef struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	UpdatedAt        string `json:"updated_at"`
	BodyDigest       string `json:"body_digest"`
	KeywordsDigest   string `json:"keywords_digest"`
	StructuredDigest string `json:"structured_digest"`
}

type concernExternalSeedDigestRef struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	SourceRef string `json:"source_ref"`
	Lane      string `json:"lane"`
}

func concernGraphMaterialDigest(
	index codebase.IndexState,
	inputs concernFusionInputs,
) (string, error) {
	graphMaterial := struct {
		CodeEpoch int64
		Edges     []codebase.CodeEdge
		Links     []artifact.LinkEdge
		Affected  []artifact.AffectedFileRef
		Bindings  []artifact.SymbolBindingRef
		Symbols   []codebase.SymbolRef
		Modules   moduleFusionInputs
		Artifacts []concernArtifactDigestRef
		External  []concernExternalSeedDigestRef
	}{
		CodeEpoch: index.Epoch,
		Edges:     inputs.edges,
		Links:     inputs.links,
		Affected:  inputs.affected,
		Bindings:  inputs.bindings,
		Symbols:   inputs.refs,
		Modules:   inputs.modules,
	}
	for _, item := range inputs.artifacts {
		graphMaterial.Artifacts = append(
			graphMaterial.Artifacts,
			concernArtifactDigest(item),
		)
	}
	for _, seed := range inputs.external {
		graphMaterial.External = append(
			graphMaterial.External,
			concernExternalSeedDigestRef{
				Kind:      seed.kind,
				Reference: seed.reference,
				SourceRef: seed.sourceRef,
				Lane:      seed.lane,
			},
		)
	}
	sort.SliceStable(
		graphMaterial.External,
		func(left, right int) bool {
			leftSeed := graphMaterial.External[left]
			rightSeed := graphMaterial.External[right]
			if leftSeed.Kind != rightSeed.Kind {
				return leftSeed.Kind < rightSeed.Kind
			}
			if leftSeed.Reference != rightSeed.Reference {
				return leftSeed.Reference < rightSeed.Reference
			}
			return leftSeed.SourceRef < rightSeed.SourceRef
		},
	)
	return digestConcernValue(graphMaterial)
}

func concernArtifactDigest(
	item *artifact.Artifact,
) concernArtifactDigestRef {
	bodyDigest, _ := digestConcernValue(item.Body)
	keywordsDigest, _ := digestConcernValue(item.SearchKeywords)
	structuredDigest, _ := digestConcernValue(item.StructuredData)
	return concernArtifactDigestRef{
		ID:               item.Meta.ID,
		Kind:             string(item.Meta.Kind),
		Title:            item.Meta.Title,
		Status:           string(item.Meta.Status),
		UpdatedAt:        item.Meta.UpdatedAt.UTC().Format(time.RFC3339Nano),
		BodyDigest:       bodyDigest,
		KeywordsDigest:   keywordsDigest,
		StructuredDigest: structuredDigest,
	}
}

func (s *Service) ConfirmConcernFusionBasis(
	ctx context.Context,
	query codebase.ConcernQuery,
	index codebase.IndexState,
	before ConcernFusionBasis,
) error {
	current, err := s.loadConcernFusionInputs(ctx, query, index)
	if err != nil {
		return err
	}
	after, err := concernGraphMaterialDigest(index, current)
	if err != nil {
		return err
	}
	if before.GraphDigest == after {
		return nil
	}
	return &ConcernGraphBasisChangedError{
		Before: before.GraphDigest,
		After:  after,
	}
}

func digestConcernValue(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func stableUniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
