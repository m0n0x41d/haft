package codeintel

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const maxTraversalCounter = int64(^uint64(0) >> 1)

var validEdgeKinds = map[codebase.EdgeKind]struct{}{
	codebase.EdgeCall:              {},
	codebase.EdgeInterfaceDispatch: {},
	codebase.EdgeImplements:        {},
	codebase.EdgeExtends:           {},
	codebase.EdgeEmbeds:            {},
	codebase.EdgeInstantiates:      {},
	codebase.EdgeValueReference:    {},
	codebase.EdgeTypeReference:     {},
	codebase.EdgeTemplateUse:       {},
	codebase.EdgeCallback:          {},
}

var validProvenance = map[codebase.Provenance]struct{}{
	codebase.ProvenanceStatic:    {},
	codebase.ProvenanceHeuristic: {},
}

// StableSymbol is a validated code-graph symbol identity. Its fields are
// private so weak strings enter traversal only through NewStableSymbol.
type StableSymbol struct {
	id string
}

// NewStableSymbol parses a weak symbol ID at the codeintel boundary.
func NewStableSymbol(id string) (StableSymbol, error) {
	if id == "" || strings.TrimSpace(id) != id {
		return StableSymbol{}, fmt.Errorf("stable symbol ID must be non-empty canonical text")
	}
	return StableSymbol{id: id}, nil
}

// ID returns the canonical symbol identity.
func (s StableSymbol) ID() string {
	return s.id
}

func (s StableSymbol) valid() bool {
	return s.id != "" && strings.TrimSpace(s.id) == s.id
}

// MarshalJSON keeps symbol identity stable without exposing mutable fields.
func (s StableSymbol) MarshalJSON() ([]byte, error) {
	if !s.valid() {
		return nil, fmt.Errorf("marshal invalid stable symbol")
	}
	return json.Marshal(s.id)
}

// TraversalScope is the exact published index/coverage basis of a semantic
// graph result. Known absence requires this stronger type.
type TraversalScope struct {
	epoch                 int64
	coverageRef           string
	knownAbsenceSupported bool
}

// NewTraversalScope constructs a scope only for a published positive epoch.
func NewTraversalScope(epoch int64, coverageRef string) (TraversalScope, error) {
	return NewTraversalScopeWithCoverage(
		epoch,
		coverageRef,
		true,
	)
}

// NewTraversalScopeWithCoverage constructs an exact published scope while
// keeping the stronger known-absence capability explicit.
func NewTraversalScopeWithCoverage(
	epoch int64,
	coverageRef string,
	knownAbsenceSupported bool,
) (TraversalScope, error) {
	if epoch < 1 {
		return TraversalScope{}, fmt.Errorf("traversal scope epoch must be positive")
	}
	if coverageRef == "" || strings.TrimSpace(coverageRef) != coverageRef {
		return TraversalScope{}, fmt.Errorf("coverage ref must be non-empty canonical text")
	}
	return TraversalScope{
		epoch:                 epoch,
		coverageRef:           coverageRef,
		knownAbsenceSupported: knownAbsenceSupported,
	}, nil
}

// Epoch returns the published graph epoch.
func (s TraversalScope) Epoch() int64 {
	return s.epoch
}

// CoverageRef returns the exact coverage carrier for the epoch.
func (s TraversalScope) CoverageRef() string {
	return s.coverageRef
}

func (s TraversalScope) SupportsKnownAbsence() bool {
	return s.valid() && s.knownAbsenceSupported
}

func (s TraversalScope) valid() bool {
	return s.epoch > 0 &&
		s.coverageRef != "" &&
		strings.TrimSpace(s.coverageRef) == s.coverageRef
}

func (s TraversalScope) MarshalJSON() ([]byte, error) {
	if !s.valid() {
		return nil, fmt.Errorf("marshal invalid traversal scope")
	}
	payload := struct {
		Epoch                 int64  `json:"epoch"`
		CoverageRef           string `json:"coverage_ref"`
		KnownAbsenceSupported bool   `json:"known_absence_supported"`
	}{
		Epoch:                 s.epoch,
		CoverageRef:           s.coverageRef,
		KnownAbsenceSupported: s.knownAbsenceSupported,
	}
	return json.Marshal(payload)
}

// IndexObservation names the best available index basis when a published
// complete traversal scope cannot be constructed. Epoch zero means no current
// epoch; the explicit basis ref says why that fact is known.
type IndexObservation struct {
	currentEpoch int64
	basisRef     string
}

// NewIndexObservation constructs an availability basis without fabricating
// complete coverage.
func NewIndexObservation(
	currentEpoch int64,
	basisRef string,
) (IndexObservation, error) {
	if currentEpoch < 0 {
		return IndexObservation{}, fmt.Errorf("current epoch cannot be negative")
	}
	if basisRef == "" || strings.TrimSpace(basisRef) != basisRef {
		return IndexObservation{}, fmt.Errorf("index observation ref must be canonical text")
	}
	return IndexObservation{
		currentEpoch: currentEpoch,
		basisRef:     basisRef,
	}, nil
}

func (o IndexObservation) valid() bool {
	return o.currentEpoch >= 0 &&
		o.basisRef != "" &&
		strings.TrimSpace(o.basisRef) == o.basisRef
}

func (o IndexObservation) MarshalJSON() ([]byte, error) {
	if !o.valid() {
		return nil, fmt.Errorf("marshal invalid index observation")
	}
	payload := struct {
		CurrentEpoch int64  `json:"current_epoch"`
		BasisRef     string `json:"basis_ref"`
	}{
		CurrentEpoch: o.currentEpoch,
		BasisRef:     o.basisRef,
	}
	return json.Marshal(payload)
}

// TraversalBudget is a validated pair of independent graph bounds.
type TraversalBudget struct {
	maxHops     int64
	visitBudget int64
}

// NewTraversalBudget constructs positive, addition-safe traversal bounds.
func NewTraversalBudget(
	maxHops int64,
	visitBudget int64,
) (TraversalBudget, error) {
	if maxHops < 1 || visitBudget < 1 {
		return TraversalBudget{}, fmt.Errorf("traversal budgets must be positive")
	}
	if maxHops >= maxTraversalCounter || visitBudget >= maxTraversalCounter {
		return TraversalBudget{}, fmt.Errorf("traversal budget exceeds safe counter range")
	}
	return TraversalBudget{
		maxHops:     maxHops,
		visitBudget: visitBudget,
	}, nil
}

// ParseTraversalBudget rejects numeric overflow at the weak text boundary.
func ParseTraversalBudget(
	maxHopsRaw string,
	visitBudgetRaw string,
) (TraversalBudget, error) {
	maxHops, err := strconv.ParseInt(maxHopsRaw, 10, 64)
	if err != nil {
		return TraversalBudget{}, fmt.Errorf("parse max hops: %w", err)
	}
	visitBudget, err := strconv.ParseInt(visitBudgetRaw, 10, 64)
	if err != nil {
		return TraversalBudget{}, fmt.Errorf("parse visit budget: %w", err)
	}
	return NewTraversalBudget(maxHops, visitBudget)
}

// MaxHops returns the path-depth bound.
func (b TraversalBudget) MaxHops() int64 {
	return b.maxHops
}

// VisitBudget returns the distinct-node visit bound.
func (b TraversalBudget) VisitBudget() int64 {
	return b.visitBudget
}

func (b TraversalBudget) valid() bool {
	return b.maxHops > 0 &&
		b.visitBudget > 0 &&
		b.maxHops < maxTraversalCounter &&
		b.visitBudget < maxTraversalCounter
}

func (b TraversalBudget) MarshalJSON() ([]byte, error) {
	if !b.valid() {
		return nil, fmt.Errorf("marshal invalid traversal budget")
	}
	payload := struct {
		MaxHops     int64 `json:"max_hops"`
		VisitBudget int64 `json:"visit_budget"`
	}{
		MaxHops:     b.maxHops,
		VisitBudget: b.visitBudget,
	}
	return json.Marshal(payload)
}

// TraversalStats records deterministic graph work. Wall-clock time does not
// enter this value.
type TraversalStats struct {
	scope          TraversalScope
	budget         TraversalBudget
	visitedNodes   int64
	inspectedEdges int64
	hopDepth       int64
	bridgeCount    int64
}

// NewTraversalStats validates counters against the declared scope and budget.
func NewTraversalStats(
	scope TraversalScope,
	budget TraversalBudget,
	visitedNodes int64,
	inspectedEdges int64,
	hopDepth int64,
	bridgeCount int64,
) (TraversalStats, error) {
	if !scope.valid() {
		return TraversalStats{}, fmt.Errorf("traversal stats require a valid scope")
	}
	if !budget.valid() {
		return TraversalStats{}, fmt.Errorf("traversal stats require a valid budget")
	}
	if visitedNodes < 1 ||
		inspectedEdges < 0 ||
		hopDepth < 0 ||
		bridgeCount < 0 {
		return TraversalStats{}, fmt.Errorf("traversal counters are invalid")
	}
	if visitedNodes > budget.visitBudget {
		return TraversalStats{}, fmt.Errorf("visited nodes exceed visit budget")
	}
	if hopDepth > budget.maxHops {
		return TraversalStats{}, fmt.Errorf("hop depth exceeds max hops")
	}
	if bridgeCount > hopDepth {
		return TraversalStats{}, fmt.Errorf("bridge count exceeds hop depth")
	}
	return TraversalStats{
		scope:          scope,
		budget:         budget,
		visitedNodes:   visitedNodes,
		inspectedEdges: inspectedEdges,
		hopDepth:       hopDepth,
		bridgeCount:    bridgeCount,
	}, nil
}

// Scope returns the exact graph basis.
func (s TraversalStats) Scope() TraversalScope {
	return s.scope
}

// Budget returns the configured traversal bounds.
func (s TraversalStats) Budget() TraversalBudget {
	return s.budget
}

// VisitedNodes returns the deterministic distinct-node count.
func (s TraversalStats) VisitedNodes() int64 {
	return s.visitedNodes
}

// InspectedEdges returns the deterministic edge-read count.
func (s TraversalStats) InspectedEdges() int64 {
	return s.inspectedEdges
}

// HopDepth returns the deepest reached hop.
func (s TraversalStats) HopDepth() int64 {
	return s.hopDepth
}

// BridgeCount returns the number of heuristic dispatch bridges.
func (s TraversalStats) BridgeCount() int64 {
	return s.bridgeCount
}

func (s TraversalStats) valid() bool {
	_, err := NewTraversalStats(
		s.scope,
		s.budget,
		s.visitedNodes,
		s.inspectedEdges,
		s.hopDepth,
		s.bridgeCount,
	)
	return err == nil
}

func (s TraversalStats) MarshalJSON() ([]byte, error) {
	if !s.valid() {
		return nil, fmt.Errorf("marshal invalid traversal stats")
	}
	payload := struct {
		Scope          TraversalScope  `json:"scope"`
		Budget         TraversalBudget `json:"budget"`
		VisitedNodes   int64           `json:"visited_nodes"`
		InspectedEdges int64           `json:"inspected_edges"`
		HopDepth       int64           `json:"hop_depth"`
		BridgeCount    int64           `json:"bridge_count"`
	}{
		Scope:          s.scope,
		Budget:         s.budget,
		VisitedNodes:   s.visitedNodes,
		InspectedEdges: s.inspectedEdges,
		HopDepth:       s.hopDepth,
		BridgeCount:    s.bridgeCount,
	}
	return json.Marshal(payload)
}

// CompletedTraversal is the proof carrier an algorithm may construct only
// after its frontier is exhausted under the declared scope and budget. It
// prevents a generic in-progress or truncated stats value from constructing
// known absence by accident.
type CompletedTraversal struct {
	stats TraversalStats
}

// NewCompletedTraversal seals deterministic stats as a completed traversal.
// The algorithm remains responsible for calling this only after frontier
// exhaustion; budget equality alone neither proves nor disproves completion.
func NewCompletedTraversal(
	stats TraversalStats,
) (CompletedTraversal, error) {
	if !stats.valid() || !stats.scope.SupportsKnownAbsence() {
		return CompletedTraversal{}, fmt.Errorf("completed traversal stats are invalid")
	}
	return CompletedTraversal{stats: stats}, nil
}

func (c CompletedTraversal) valid() bool {
	return c.stats.valid()
}

// Stats returns the deterministic completed traversal account.
func (c CompletedTraversal) Stats() TraversalStats {
	return c.stats
}

func (c CompletedTraversal) MarshalJSON() ([]byte, error) {
	if !c.valid() {
		return nil, fmt.Errorf("marshal invalid completed traversal")
	}
	return json.Marshal(c.stats)
}

// ResolvedSeedBasis is a closed identity tier that may select one seed.
type ResolvedSeedBasis struct {
	code string
}

var resolvedSeedBases = map[string]ResolvedSeedBasis{
	"exact_stable_id":      {code: "exact_stable_id"},
	"exact_anchor":         {code: "exact_anchor"},
	"exact_qualified_name": {code: "exact_qualified_name"},
	"unique_exact_name":    {code: "unique_exact_name"},
}

// ParseResolvedSeedBasis rejects weak or ranking-only selection bases.
func ParseResolvedSeedBasis(raw string) (ResolvedSeedBasis, error) {
	basis, found := resolvedSeedBases[raw]
	if !found {
		return ResolvedSeedBasis{}, fmt.Errorf("unknown resolved-seed basis %q", raw)
	}
	return basis, nil
}

func (b ResolvedSeedBasis) valid() bool {
	_, found := resolvedSeedBases[b.code]
	return found
}

func (b ResolvedSeedBasis) String() string {
	return b.code
}

func (b ResolvedSeedBasis) MarshalJSON() ([]byte, error) {
	if !b.valid() {
		return nil, fmt.Errorf("marshal invalid resolved-seed basis")
	}
	return json.Marshal(b.code)
}

// CandidateSetBasis is a closed non-selecting retrieval basis.
type CandidateSetBasis struct {
	code string
}

var candidateSetBases = map[string]CandidateSetBasis{
	"ambiguous_exact_name": {code: "ambiguous_exact_name"},
	"lexical_candidates":   {code: "lexical_candidates"},
	"fuzzy_candidates":     {code: "fuzzy_candidates"},
	"reasoning_candidates": {code: "reasoning_candidates"},
}

// ParseCandidateSetBasis rejects an undeclared candidate policy.
func ParseCandidateSetBasis(raw string) (CandidateSetBasis, error) {
	basis, found := candidateSetBases[raw]
	if !found {
		return CandidateSetBasis{}, fmt.Errorf("unknown candidate-set basis %q", raw)
	}
	return basis, nil
}

func (b CandidateSetBasis) valid() bool {
	_, found := candidateSetBases[b.code]
	return found
}

func (b CandidateSetBasis) String() string {
	return b.code
}

func (b CandidateSetBasis) MarshalJSON() ([]byte, error) {
	if !b.valid() {
		return nil, fmt.Errorf("marshal invalid candidate-set basis")
	}
	return json.Marshal(b.code)
}

// SeedUnavailableReason is a closed reason for lacking a seed-resolution
// capability. It is not a runtime error and not known absence.
type SeedUnavailableReason struct {
	code string
}

var seedUnavailableReasons = map[string]SeedUnavailableReason{
	"index_unavailable":      {code: "index_unavailable"},
	"index_incomplete":       {code: "index_incomplete"},
	"retry_required":         {code: "retry_required"},
	"capability_unavailable": {code: "capability_unavailable"},
}

// ParseSeedUnavailableReason validates a weak machine reason code.
func ParseSeedUnavailableReason(raw string) (SeedUnavailableReason, error) {
	reason, found := seedUnavailableReasons[raw]
	if !found {
		return SeedUnavailableReason{}, fmt.Errorf(
			"unknown seed-unavailable reason %q",
			raw,
		)
	}
	return reason, nil
}

func (r SeedUnavailableReason) valid() bool {
	_, found := seedUnavailableReasons[r.code]
	return found
}

func (r SeedUnavailableReason) String() string {
	return r.code
}

func (r SeedUnavailableReason) MarshalJSON() ([]byte, error) {
	if !r.valid() {
		return nil, fmt.Errorf("marshal invalid seed-unavailable reason")
	}
	return json.Marshal(r.code)
}

// SeedResolutionKind is the closed public discriminator for SeedResolution.
type SeedResolutionKind struct {
	code string
}

var seedResolutionKinds = map[string]SeedResolutionKind{
	"resolved_seed":    {code: "resolved_seed"},
	"candidate_set":    {code: "candidate_set"},
	"seed_not_found":   {code: "seed_not_found"},
	"seed_unavailable": {code: "seed_unavailable"},
}

func (k SeedResolutionKind) String() string {
	return k.code
}

func (k SeedResolutionKind) valid() bool {
	_, found := seedResolutionKinds[k.code]
	return found
}

func (k SeedResolutionKind) MarshalJSON() ([]byte, error) {
	if !k.valid() {
		return nil, fmt.Errorf("marshal invalid seed-resolution kind")
	}
	return json.Marshal(k.code)
}

// SeedResolution is a sealed semantic union. Concrete variants stay private,
// so external packages cannot combine them with boolean flags.
type SeedResolution interface {
	json.Marshaler
	Kind() SeedResolutionKind
	DetailCode() string
	seedResolution()
}

type resolvedSeed struct {
	symbol StableSymbol
	basis  ResolvedSeedBasis
	scope  TraversalScope
}

// NewResolvedSeed constructs the only seed variant that permits traversal.
func NewResolvedSeed(
	symbol StableSymbol,
	basis ResolvedSeedBasis,
	scope TraversalScope,
) (SeedResolution, error) {
	if !symbol.valid() || !basis.valid() || !scope.valid() {
		return nil, fmt.Errorf("resolved seed inputs are invalid")
	}
	return resolvedSeed{
		symbol: symbol,
		basis:  basis,
		scope:  scope,
	}, nil
}

func (resolvedSeed) seedResolution() {}

func (resolvedSeed) Kind() SeedResolutionKind {
	return seedResolutionKinds["resolved_seed"]
}

func (r resolvedSeed) DetailCode() string {
	return r.basis.String()
}

func (r resolvedSeed) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind   SeedResolutionKind `json:"kind"`
		Symbol StableSymbol       `json:"symbol"`
		Basis  ResolvedSeedBasis  `json:"basis"`
		Scope  TraversalScope     `json:"scope"`
	}{
		Kind:   r.Kind(),
		Symbol: r.symbol,
		Basis:  r.basis,
		Scope:  r.scope,
	}
	return json.Marshal(payload)
}

type candidateSet struct {
	candidates []StableSymbol
	basis      CandidateSetBasis
	scope      TraversalScope
}

// NewCandidateSet constructs a non-empty, unique, stable-ordered candidate
// set. Candidate order never implies automatic identity selection.
func NewCandidateSet(
	candidates []StableSymbol,
	basis CandidateSetBasis,
	scope TraversalScope,
) (SeedResolution, error) {
	if len(candidates) == 0 || !basis.valid() || !scope.valid() {
		return nil, fmt.Errorf("candidate-set inputs are invalid")
	}
	ordered := slices.Clone(candidates)
	for _, candidate := range ordered {
		if !candidate.valid() {
			return nil, fmt.Errorf("candidate set contains an invalid symbol")
		}
	}
	slices.SortFunc(ordered, func(left StableSymbol, right StableSymbol) int {
		return strings.Compare(left.id, right.id)
	})
	for index := 1; index < len(ordered); index++ {
		if ordered[index-1].id == ordered[index].id {
			return nil, fmt.Errorf("candidate set repeats symbol %q", ordered[index].id)
		}
	}
	return candidateSet{
		candidates: ordered,
		basis:      basis,
		scope:      scope,
	}, nil
}

func (candidateSet) seedResolution() {}

func (candidateSet) Kind() SeedResolutionKind {
	return seedResolutionKinds["candidate_set"]
}

func (c candidateSet) DetailCode() string {
	return c.basis.String()
}

func (c candidateSet) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind       SeedResolutionKind `json:"kind"`
		Candidates []StableSymbol     `json:"candidates"`
		Basis      CandidateSetBasis  `json:"basis"`
		Scope      TraversalScope     `json:"scope"`
	}{
		Kind:       c.Kind(),
		Candidates: slices.Clone(c.candidates),
		Basis:      c.basis,
		Scope:      c.scope,
	}
	return json.Marshal(payload)
}

type seedNotFound struct {
	query string
	scope TraversalScope
}

// NewSeedNotFound constructs known absence only under an exact traversal scope.
func NewSeedNotFound(
	query string,
	scope TraversalScope,
) (SeedResolution, error) {
	if strings.TrimSpace(query) == "" ||
		!scope.SupportsKnownAbsence() {
		return nil, fmt.Errorf("seed-not-found inputs are invalid")
	}
	return seedNotFound{
		query: query,
		scope: scope,
	}, nil
}

func (seedNotFound) seedResolution() {}

func (seedNotFound) Kind() SeedResolutionKind {
	return seedResolutionKinds["seed_not_found"]
}

func (seedNotFound) DetailCode() string {
	return "complete_indexed_scope"
}

func (s seedNotFound) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind  SeedResolutionKind `json:"kind"`
		Query string             `json:"query"`
		Scope TraversalScope     `json:"scope"`
	}{
		Kind:  s.Kind(),
		Query: s.query,
		Scope: s.scope,
	}
	return json.Marshal(payload)
}

type seedUnavailable struct {
	query       string
	reason      SeedUnavailableReason
	observation IndexObservation
}

// NewSeedUnavailable constructs semantic unavailability without claiming a
// complete searched scope.
func NewSeedUnavailable(
	query string,
	reason SeedUnavailableReason,
	observation IndexObservation,
) (SeedResolution, error) {
	if strings.TrimSpace(query) == "" ||
		!reason.valid() ||
		!observation.valid() {
		return nil, fmt.Errorf("seed-unavailable inputs are invalid")
	}
	return seedUnavailable{
		query:       query,
		reason:      reason,
		observation: observation,
	}, nil
}

func (seedUnavailable) seedResolution() {}

func (seedUnavailable) Kind() SeedResolutionKind {
	return seedResolutionKinds["seed_unavailable"]
}

func (s seedUnavailable) DetailCode() string {
	return s.reason.String()
}

func (s seedUnavailable) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind        SeedResolutionKind    `json:"kind"`
		Query       string                `json:"query"`
		Reason      SeedUnavailableReason `json:"reason"`
		Observation IndexObservation      `json:"observation"`
	}{
		Kind:        s.Kind(),
		Query:       s.query,
		Reason:      s.reason,
		Observation: s.observation,
	}
	return json.Marshal(payload)
}

// GraphHop is one validated directed static graph relation.
type GraphHop struct {
	from       StableSymbol
	to         StableSymbol
	kind       codebase.EdgeKind
	provenance codebase.Provenance
}

// NewGraphHop constructs a hop only from supported edge and provenance codes.
func NewGraphHop(
	from StableSymbol,
	to StableSymbol,
	kind codebase.EdgeKind,
	provenance codebase.Provenance,
) (GraphHop, error) {
	if !from.valid() || !to.valid() || from.id == to.id {
		return GraphHop{}, fmt.Errorf("graph hop endpoints are invalid")
	}
	if _, found := validEdgeKinds[kind]; !found {
		return GraphHop{}, fmt.Errorf("unknown graph edge kind %q", kind)
	}
	if _, found := validProvenance[provenance]; !found {
		return GraphHop{}, fmt.Errorf("unknown graph provenance %q", provenance)
	}
	return GraphHop{
		from:       from,
		to:         to,
		kind:       kind,
		provenance: provenance,
	}, nil
}

func (h GraphHop) valid() bool {
	_, err := NewGraphHop(h.from, h.to, h.kind, h.provenance)
	return err == nil
}

func (h GraphHop) MarshalJSON() ([]byte, error) {
	if !h.valid() {
		return nil, fmt.Errorf("marshal invalid graph hop")
	}
	payload := struct {
		From       StableSymbol        `json:"from"`
		To         StableSymbol        `json:"to"`
		Kind       codebase.EdgeKind   `json:"kind"`
		Provenance codebase.Provenance `json:"provenance"`
	}{
		From:       h.from,
		To:         h.to,
		Kind:       h.kind,
		Provenance: h.provenance,
	}
	return json.Marshal(payload)
}

type pathPrefix struct {
	seed StableSymbol
	hops []GraphHop
}

// newPathPrefix validates a contiguous path beginning at one seed. A
// source-equals-target path is represented by its seed and zero edges.
func newPathPrefix(
	seed StableSymbol,
	hops []GraphHop,
) (pathPrefix, error) {
	if !seed.valid() {
		return pathPrefix{}, fmt.Errorf("path prefix requires a valid seed")
	}
	ordered := slices.Clone(hops)
	currentID := seed.id
	for _, hop := range ordered {
		if !hop.valid() || hop.from.id != currentID {
			return pathPrefix{}, fmt.Errorf("path prefix is not a contiguous directed path")
		}
		currentID = hop.to.id
	}
	return pathPrefix{
		seed: seed,
		hops: ordered,
	}, nil
}

func (p pathPrefix) valid() bool {
	_, err := newPathPrefix(p.seed, p.hops)
	return err == nil
}

func (p pathPrefix) MarshalJSON() ([]byte, error) {
	if !p.valid() {
		return nil, fmt.Errorf("marshal invalid path prefix")
	}
	payload := struct {
		Seed StableSymbol `json:"seed"`
		Hops []GraphHop   `json:"hops"`
	}{
		Seed: p.seed,
		Hops: slices.Clone(p.hops),
	}
	return json.Marshal(payload)
}

// PathTruncationReason is a closed budget stop, never known absence.
type PathTruncationReason struct {
	code string
}

var pathTruncationReasons = map[string]PathTruncationReason{
	"max_hops":     {code: "max_hops"},
	"visit_budget": {code: "visit_budget"},
}

// ParsePathTruncationReason validates a weak truncation reason.
func ParsePathTruncationReason(raw string) (PathTruncationReason, error) {
	reason, found := pathTruncationReasons[raw]
	if !found {
		return PathTruncationReason{}, fmt.Errorf(
			"unknown path-truncation reason %q",
			raw,
		)
	}
	return reason, nil
}

func (r PathTruncationReason) valid() bool {
	_, found := pathTruncationReasons[r.code]
	return found
}

func (r PathTruncationReason) String() string {
	return r.code
}

func (r PathTruncationReason) MarshalJSON() ([]byte, error) {
	if !r.valid() {
		return nil, fmt.Errorf("marshal invalid path-truncation reason")
	}
	return json.Marshal(r.code)
}

// PathUnavailableReason is a closed missing semantic capability. Runtime port
// errors are deliberately absent from this set.
type PathUnavailableReason struct {
	code string
}

var pathUnavailableReasons = map[string]PathUnavailableReason{
	"resolver_capability": {code: "resolver_capability"},
	"language_capability": {code: "language_capability"},
	"index_capability":    {code: "index_capability"},
}

// ParsePathUnavailableReason validates a weak capability reason.
func ParsePathUnavailableReason(raw string) (PathUnavailableReason, error) {
	reason, found := pathUnavailableReasons[raw]
	if !found {
		return PathUnavailableReason{}, fmt.Errorf(
			"unknown path-unavailable reason %q",
			raw,
		)
	}
	return reason, nil
}

func (r PathUnavailableReason) valid() bool {
	_, found := pathUnavailableReasons[r.code]
	return found
}

func (r PathUnavailableReason) String() string {
	return r.code
}

func (r PathUnavailableReason) MarshalJSON() ([]byte, error) {
	if !r.valid() {
		return nil, fmt.Errorf("marshal invalid path-unavailable reason")
	}
	return json.Marshal(r.code)
}

// PathOutcomeKind is the closed discriminator for PathOutcome.
type PathOutcomeKind struct {
	code string
}

var pathOutcomeKinds = map[string]PathOutcomeKind{
	"path_found":                       {code: "path_found"},
	"path_absent_within_indexed_graph": {code: "path_absent_within_indexed_graph"},
	"path_truncated":                   {code: "path_truncated"},
	"path_unavailable":                 {code: "path_unavailable"},
}

func (k PathOutcomeKind) String() string {
	return k.code
}

func (k PathOutcomeKind) valid() bool {
	_, found := pathOutcomeKinds[k.code]
	return found
}

func (k PathOutcomeKind) MarshalJSON() ([]byte, error) {
	if !k.valid() {
		return nil, fmt.Errorf("marshal invalid path-outcome kind")
	}
	return json.Marshal(k.code)
}

// PathOutcome is a sealed semantic union. Store and cancellation errors remain
// on the ordinary error channel of the traversal algorithm.
type PathOutcome interface {
	json.Marshaler
	Kind() PathOutcomeKind
	DetailCode() string
	pathOutcome()
}

type pathFound struct {
	path  pathPrefix
	stats TraversalStats
}

// NewPathFound constructs a found path whose depth matches its exact hops.
func NewPathFound(
	seed StableSymbol,
	hops []GraphHop,
	stats TraversalStats,
) (PathOutcome, error) {
	path, err := newPathPrefix(seed, hops)
	if err != nil {
		return nil, err
	}
	if !path.valid() || !stats.valid() {
		return nil, fmt.Errorf("path-found inputs are invalid")
	}
	hopCount := int64(len(path.hops))
	if hopCount != stats.hopDepth {
		return nil, fmt.Errorf("path hop count differs from traversal depth")
	}
	minimumVisited := hopCount + 1
	if stats.visitedNodes < minimumVisited {
		return nil, fmt.Errorf("path has more nodes than traversal visited")
	}
	return pathFound{
		path:  path,
		stats: stats,
	}, nil
}

func (pathFound) pathOutcome() {}

func (pathFound) Kind() PathOutcomeKind {
	return pathOutcomeKinds["path_found"]
}

func (pathFound) DetailCode() string {
	return "connected"
}

func (p pathFound) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind  PathOutcomeKind `json:"kind"`
		Path  pathPrefix      `json:"path"`
		Stats TraversalStats  `json:"stats"`
	}{
		Kind:  p.Kind(),
		Path:  p.path,
		Stats: p.stats,
	}
	return json.Marshal(payload)
}

type pathAbsentWithinIndexedGraph struct {
	stats TraversalStats
}

// NewPathAbsentWithinIndexedGraph records true bounded absence only with the
// complete indexed scope already carried by stats.
func NewPathAbsentWithinIndexedGraph(
	completion CompletedTraversal,
) (PathOutcome, error) {
	if !completion.valid() {
		return nil, fmt.Errorf("path-absent completion is invalid")
	}
	return pathAbsentWithinIndexedGraph(completion), nil
}

func (pathAbsentWithinIndexedGraph) pathOutcome() {}

func (pathAbsentWithinIndexedGraph) Kind() PathOutcomeKind {
	return pathOutcomeKinds["path_absent_within_indexed_graph"]
}

func (pathAbsentWithinIndexedGraph) DetailCode() string {
	return "complete_traversal"
}

func (p pathAbsentWithinIndexedGraph) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind  PathOutcomeKind `json:"kind"`
		Stats TraversalStats  `json:"stats"`
	}{
		Kind:  p.Kind(),
		Stats: p.stats,
	}
	return json.Marshal(payload)
}

type pathTruncated struct {
	reason  PathTruncationReason
	partial pathPrefix
	stats   TraversalStats
}

// NewPathTruncated constructs only a budget stop that is witnessed by stats.
func NewPathTruncated(
	reason PathTruncationReason,
	seed StableSymbol,
	hops []GraphHop,
	stats TraversalStats,
) (PathOutcome, error) {
	partial, err := newPathPrefix(seed, hops)
	if err != nil {
		return nil, err
	}
	if !reason.valid() || !partial.valid() || !stats.valid() {
		return nil, fmt.Errorf("path-truncated inputs are invalid")
	}
	partialDepth := int64(len(partial.hops))
	if partialDepth > stats.hopDepth {
		return nil, fmt.Errorf("partial path exceeds traversal depth")
	}
	if reason.code == "max_hops" && stats.hopDepth != stats.budget.maxHops {
		return nil, fmt.Errorf("max-hops truncation lacks matching stats")
	}
	if reason.code == "visit_budget" &&
		stats.visitedNodes != stats.budget.visitBudget {
		return nil, fmt.Errorf("visit-budget truncation lacks matching stats")
	}
	return pathTruncated{
		reason:  reason,
		partial: partial,
		stats:   stats,
	}, nil
}

func (pathTruncated) pathOutcome() {}

func (pathTruncated) Kind() PathOutcomeKind {
	return pathOutcomeKinds["path_truncated"]
}

func (p pathTruncated) DetailCode() string {
	return p.reason.String()
}

func (p pathTruncated) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind    PathOutcomeKind      `json:"kind"`
		Reason  PathTruncationReason `json:"reason"`
		Partial pathPrefix           `json:"partial_path"`
		Stats   TraversalStats       `json:"stats"`
	}{
		Kind:    p.Kind(),
		Reason:  p.reason,
		Partial: p.partial,
		Stats:   p.stats,
	}
	return json.Marshal(payload)
}

type pathUnavailable struct {
	reason PathUnavailableReason
	stats  TraversalStats
}

// NewPathUnavailable constructs semantic capability absence. Runtime errors
// are not accepted as reason codes.
func NewPathUnavailable(
	reason PathUnavailableReason,
	stats TraversalStats,
) (PathOutcome, error) {
	if !reason.valid() || !stats.valid() {
		return nil, fmt.Errorf("path-unavailable inputs are invalid")
	}
	return pathUnavailable{
		reason: reason,
		stats:  stats,
	}, nil
}

func (pathUnavailable) pathOutcome() {}

func (pathUnavailable) Kind() PathOutcomeKind {
	return pathOutcomeKinds["path_unavailable"]
}

func (p pathUnavailable) DetailCode() string {
	return p.reason.String()
}

func (p pathUnavailable) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind   PathOutcomeKind       `json:"kind"`
		Reason PathUnavailableReason `json:"reason"`
		Stats  TraversalStats        `json:"stats"`
	}{
		Kind:   p.Kind(),
		Reason: p.reason,
		Stats:  p.stats,
	}
	return json.Marshal(payload)
}

// ChainTermination is a closed reason why a longest-chain computation stopped.
type ChainTermination struct {
	code string
}

var chainTerminations = map[string]ChainTermination{
	"leaf_reached":                 {code: "leaf_reached"},
	"coverage_incomplete":          {code: "coverage_incomplete"},
	"unresolved_dispatch_boundary": {code: "unresolved_dispatch_boundary"},
	"max_hops_reached":             {code: "max_hops_reached"},
	"visit_budget_reached":         {code: "visit_budget_reached"},
}

// ParseChainTermination validates a weak chain-termination code.
func ParseChainTermination(raw string) (ChainTermination, error) {
	termination, found := chainTerminations[raw]
	if !found {
		return ChainTermination{}, fmt.Errorf(
			"unknown chain termination %q",
			raw,
		)
	}
	return termination, nil
}

func (t ChainTermination) valid() bool {
	_, found := chainTerminations[t.code]
	return found
}

func (t ChainTermination) String() string {
	return t.code
}

func (t ChainTermination) MarshalJSON() ([]byte, error) {
	if !t.valid() {
		return nil, fmt.Errorf("marshal invalid chain termination")
	}
	return json.Marshal(t.code)
}

// ChainOutcome is one deterministic longest-chain result. The path,
// termination, and work account are constructed together so callers cannot
// combine a complete-looking chain with a budget or dispatch stop hidden in a
// separate boolean.
type ChainOutcome struct {
	path        pathPrefix
	termination ChainTermination
	stats       TraversalStats
}

// NewChainOutcome validates one selected path and its explicit stop reason.
func NewChainOutcome(
	seed StableSymbol,
	hops []GraphHop,
	termination ChainTermination,
	stats TraversalStats,
) (ChainOutcome, error) {
	path, err := newPathPrefix(seed, hops)
	if err != nil {
		return ChainOutcome{}, err
	}
	if !termination.valid() || !stats.valid() {
		return ChainOutcome{}, fmt.Errorf("chain outcome inputs are invalid")
	}
	pathDepth := int64(len(path.hops))
	if pathDepth > stats.hopDepth {
		return ChainOutcome{}, fmt.Errorf("chain path exceeds observed traversal depth")
	}
	if stats.bridgeCount > pathDepth {
		return ChainOutcome{}, fmt.Errorf("chain bridge count exceeds selected path")
	}
	if termination.code == "max_hops_reached" &&
		pathDepth != stats.budget.maxHops {
		return ChainOutcome{}, fmt.Errorf("max-hops chain lacks matching path depth")
	}
	if termination.code == "visit_budget_reached" &&
		stats.visitedNodes != stats.budget.visitBudget {
		return ChainOutcome{}, fmt.Errorf("visit-budget chain lacks matching stats")
	}
	return ChainOutcome{
		path:        path,
		termination: termination,
		stats:       stats,
	}, nil
}

func (c ChainOutcome) valid() bool {
	_, err := NewChainOutcome(
		c.path.seed,
		c.path.hops,
		c.termination,
		c.stats,
	)
	return err == nil
}

// Termination returns the explicit reason the selected search stopped.
func (c ChainOutcome) Termination() ChainTermination {
	return c.termination
}

// Stats returns the deterministic traversal account.
func (c ChainOutcome) Stats() TraversalStats {
	return c.stats
}

func (c ChainOutcome) MarshalJSON() ([]byte, error) {
	if !c.valid() {
		return nil, fmt.Errorf("marshal invalid chain outcome")
	}
	payload := struct {
		Path        pathPrefix       `json:"path"`
		Termination ChainTermination `json:"termination"`
		Stats       TraversalStats   `json:"stats"`
	}{
		Path:        c.path,
		Termination: c.termination,
		Stats:       c.stats,
	}
	return json.Marshal(payload)
}

// BagLegDirection is the selected typed direction for one ExploreBag leg.
// "none" means neither evaluated direction produced a connected path; the two
// PathOutcome values retain the exact absence, truncation, or capability state.
type BagLegDirection struct {
	code string
}

var bagLegDirections = map[string]BagLegDirection{
	"none":    {code: "none"},
	"forward": {code: "forward"},
	"reverse": {code: "reverse"},
}

// ParseBagLegDirection validates a weak presentation or fixture value.
func ParseBagLegDirection(raw string) (BagLegDirection, error) {
	direction, found := bagLegDirections[raw]
	if !found {
		return BagLegDirection{}, fmt.Errorf("unknown bag-leg direction %q", raw)
	}
	return direction, nil
}

func (d BagLegDirection) valid() bool {
	_, found := bagLegDirections[d.code]
	return found
}

func (d BagLegDirection) String() string {
	return d.code
}

func (d BagLegDirection) MarshalJSON() ([]byte, error) {
	if !d.valid() {
		return nil, fmt.Errorf("marshal invalid bag-leg direction")
	}
	return json.Marshal(d.code)
}
