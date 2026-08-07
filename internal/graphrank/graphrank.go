// Package graphrank computes Personalized PageRank over an in-memory directed
// weighted graph — the deterministic, embedding-free recall ranker for phase 2
// of dec-20260604-3aaad199 (staged retrieval roadmap).
//
// It is pure: no I/O and no dependency on any store. An adapter fuses haft's
// code graph (symbol -> symbol call/dispatch edges) and reasoning graph
// (artifact -> artifact links, artifact <-> file) into one Graph keyed by
// string node IDs, seeds it from lexical (FTS5 / substring) hits, and reads the
// ranked stationary distribution back. Determinism: same (graph, seeds, params)
// always yields the same scores — power iteration over a stably-ordered node
// set, no maps iterated for accumulation, no randomness.
package graphrank

import (
	"fmt"
	"sort"
)

// Edge is a weighted directed out-edge.
type Edge struct {
	To     string
	Weight float64
}

// Graph is a directed weighted graph keyed by string node IDs. Symbol IDs and
// artifact IDs share one namespace once fused. Built incrementally via AddEdge,
// then read by Rank. Insertion order is preserved so iteration is deterministic.
type Graph struct {
	out   map[string][]Edge
	index map[string]struct{}
	order []string
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{out: make(map[string][]Edge), index: make(map[string]struct{})}
}

func (g *Graph) register(id string) {
	if _, ok := g.index[id]; ok {
		return
	}
	g.index[id] = struct{}{}
	g.order = append(g.order, id)
}

// AddEdge adds a directed edge from->to with weight w. Both endpoints are
// registered as nodes. Non-positive weights and self-loops are ignored — they
// carry no proximity signal and would only perturb normalization.
func (g *Graph) AddEdge(from, to string, w float64) {
	if w <= 0 || from == to {
		return
	}
	g.register(from)
	g.register(to)
	g.out[from] = append(g.out[from], Edge{To: to, Weight: w})
}

// Has reports whether id is a node in the graph.
func (g *Graph) Has(id string) bool {
	_, ok := g.index[id]
	return ok
}

// Nodes returns all node IDs in stable insertion order.
func (g *Graph) Nodes() []string { return g.order }

// Len returns the node count.
func (g *Graph) Len() int { return len(g.order) }

// InductionBudget bounds the graph projection walked by one concern query.
// The canonical graph remains unchanged; the result is a deterministic read
// projection around exact existing seed nodes.
type InductionBudget struct {
	maxHops  int
	maxNodes int
}

func NewInductionBudget(
	maxHops int,
	maxNodes int,
) (InductionBudget, error) {
	if maxHops < 0 {
		return InductionBudget{}, fmt.Errorf(
			"graph induction max hops must be non-negative",
		)
	}
	if maxNodes < 1 {
		return InductionBudget{}, fmt.Errorf(
			"graph induction max nodes must be positive",
		)
	}
	return InductionBudget{
		maxHops:  maxHops,
		maxNodes: maxNodes,
	}, nil
}

func (b InductionBudget) MaxHops() int {
	return b.maxHops
}

func (b InductionBudget) MaxNodes() int {
	return b.maxNodes
}

type InducedProjection struct {
	Graph          *Graph
	NodeCapReached bool
}

// Induce returns the deterministic bounded neighborhood of the supplied seed
// nodes. Off-graph seeds are ignored. The fused adapter publishes related
// edges bidirectionally, so directed traversal preserves that current graph.
func (g *Graph) Induce(
	seedIDs []string,
	budget InductionBudget,
) InducedProjection {
	projected := NewGraph()
	seeds := append([]string{}, seedIDs...)
	sort.Strings(seeds)
	type queuedNode struct {
		id       string
		distance int
	}
	queue := make([]queuedNode, 0, len(seeds))
	seen := make(map[string]bool)
	nodeCapReached := false
	for _, seedID := range seeds {
		if !g.Has(seedID) || seen[seedID] {
			continue
		}
		if len(seen) >= budget.maxNodes {
			nodeCapReached = true
			break
		}
		seen[seedID] = true
		projected.register(seedID)
		queue = append(queue, queuedNode{id: seedID})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.distance >= budget.maxHops {
			continue
		}
		for _, edge := range g.out[current.id] {
			if !seen[edge.To] {
				if len(seen) >= budget.maxNodes {
					nodeCapReached = true
					continue
				}
				seen[edge.To] = true
				queue = append(queue, queuedNode{
					id:       edge.To,
					distance: current.distance + 1,
				})
			}
		}
	}
	for _, nodeID := range g.order {
		if !seen[nodeID] {
			continue
		}
		for _, edge := range g.out[nodeID] {
			if seen[edge.To] {
				projected.AddEdge(nodeID, edge.To, edge.Weight)
			}
		}
	}
	return InducedProjection{
		Graph:          projected,
		NodeCapReached: nodeCapReached,
	}
}

// Params controls the random walk.
type Params struct {
	Restart float64 `json:"restart"`   // teleport-to-seed probability
	MaxIter int     `json:"max_iter"`  // power-iteration cap
	Tol     float64 `json:"tolerance"` // L1 convergence threshold
}

// DefaultParams are sane PPR defaults: 0.15 restart (the classic PageRank
// damping complement), 100-iteration cap, 1e-8 L1 tolerance.
func DefaultParams() Params {
	return Params{Restart: 0.15, MaxIter: 100, Tol: 1e-8}
}

func (p Params) normalized() Params {
	if p.Restart <= 0 || p.Restart >= 1 {
		p.Restart = 0.15
	}
	if p.MaxIter <= 0 {
		p.MaxIter = 100
	}
	if p.Tol <= 0 {
		p.Tol = 1e-8
	}
	return p
}

// Rank computes Personalized PageRank: the stationary distribution of a random
// walk that, each step, teleports back to the seed distribution with probability
// Restart and otherwise follows an out-edge chosen proportionally to its weight.
//
// seeds maps node -> non-negative mass; it is filtered to graph nodes and
// normalized internally. Seeds not present in the graph are ignored. With no
// valid seed (empty or all-off-graph) the result is nil — an unseeded walk has
// no personalization to report. Dangling nodes (no out-edges) teleport their
// mass to the seed vector, so probability is conserved.
//
// Deterministic and pure: depends only on (graph, seeds, params).
func (g *Graph) Rank(seeds map[string]float64, p Params) map[string]float64 {
	p = p.normalized()
	n := len(g.order)
	if n == 0 {
		return nil
	}

	// Personalization vector s, normalized over the seeds that land on a node.
	s := make([]float64, n)
	idx := make(map[string]int, n)
	for i, id := range g.order {
		idx[id] = i
	}
	var seedTotal float64
	for id, w := range seeds {
		if w <= 0 {
			continue
		}
		if i, ok := idx[id]; ok {
			s[i] += w
			seedTotal += w
		}
	}
	if seedTotal == 0 {
		return nil
	}
	for i := range s {
		s[i] /= seedTotal
	}

	// Pre-sum out-weights per node (a node with zero is dangling).
	outSum := make([]float64, n)
	for i, id := range g.order {
		for _, e := range g.out[id] {
			outSum[i] += e.Weight
		}
	}

	score := make([]float64, n)
	copy(score, s) // warm start from the personalization vector
	next := make([]float64, n)

	for iter := 0; iter < p.MaxIter; iter++ {
		// Dangling mass teleports to s; baseline teleport is restart*s.
		var dangling float64
		for i := range g.order {
			if outSum[i] == 0 {
				dangling += score[i]
			}
		}
		base := p.Restart + (1-p.Restart)*dangling
		for i := range next {
			next[i] = base * s[i]
		}
		// Push each node's score along its out-edges, weighted.
		for i, id := range g.order {
			if outSum[i] == 0 || score[i] == 0 {
				continue
			}
			share := (1 - p.Restart) * score[i] / outSum[i]
			for _, e := range g.out[id] {
				next[idx[e.To]] += share * e.Weight
			}
		}

		var delta float64
		for i := range score {
			diff := next[i] - score[i]
			if diff < 0 {
				diff = -diff
			}
			delta += diff
		}
		score, next = next, score
		if delta < p.Tol {
			break
		}
	}

	result := make(map[string]float64, n)
	for i, id := range g.order {
		result[id] = score[i]
	}
	return result
}
