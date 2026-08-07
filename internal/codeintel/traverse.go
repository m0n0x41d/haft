// Package codeintel is the query surface over the code graph (symbols + edges):
// callers / callees / impact traversal, and the fused tools. It depends on
// internal/codebase (extraction + stores) and is language-agnostic — it walks
// edges and node ids, never a specific language.
package codeintel

import (
	"context"

	"github.com/m0n0x41d/haft/internal/codebase"
)

// Direction selects which way to walk the call graph.
type Direction int

const (
	Callees Direction = iota // outbound: what the seed calls
	Callers                  // inbound: what calls the seed
)

// ParseTraversalProfile validates the public query value. Empty preserves the
// compatibility default; unknown values are rejected by the query boundary.
func ParseTraversalProfile(raw string) (TraversalProfile, bool) {
	if raw == "" {
		return TraversalCallFlow, true
	}
	profiles := map[string]TraversalProfile{
		string(TraversalCallFlow):        TraversalCallFlow,
		string(TraversalTypeImpact):      TraversalTypeImpact,
		string(TraversalReferenceImpact): TraversalReferenceImpact,
		string(TraversalAllCode):         TraversalAllCode,
	}
	profile, ok := profiles[raw]
	return profile, ok
}

// TraversalProfile names a semantic relation family. Traversal never mixes
// call flow with type hierarchy merely because both happen to be code edges.
type TraversalProfile string

const (
	TraversalCallFlow        TraversalProfile = "call_flow"
	TraversalTypeImpact      TraversalProfile = "type_impact"
	TraversalReferenceImpact TraversalProfile = "reference_impact"
	TraversalAllCode         TraversalProfile = "all_code"
)

// Traversal caps. The hard ceiling on depth is the safety net against runaway
// walks; the defaults keep responses small and fast (the latency objective).
const (
	MaxDepthCeiling = 10
	DefaultDepth    = 2
	MaxResults      = 20
)

// EdgeSource is the read port the traverser needs — *codebase.EdgeStore
// satisfies it. The traverser depends on this abstraction, not the store.
type EdgeSource interface {
	OutEdges(ctx context.Context, srcID string) ([]codebase.CodeEdge, error)
	InEdges(ctx context.Context, dstID string) ([]codebase.CodeEdge, error)
}

// Hop is a reached node and its BFS distance from the seed, plus how it was
// reached (call vs interface_dispatch) so the caller can mark dispatch hops.
type Hop struct {
	NodeID     string
	Distance   int
	ViaKind    codebase.EdgeKind
	Provenance codebase.Provenance
}

// Traverse performs a bounded BFS over code edges from seedID in one direction.
// Depth is clamped to [1, MaxDepthCeiling] (default DefaultDepth), results to
// MaxResults; a visited-set prevents cycles. Pure relative to the EdgeSource.
func Traverse(ctx context.Context, src EdgeSource, seedID string, dir Direction, maxDepth, limit int) ([]Hop, error) {
	return TraverseWithProfile(ctx, src, seedID, dir, maxDepth, limit, TraversalCallFlow)
}

// TraverseWithProfile performs a bounded BFS over exactly one semantic edge
// family. Unknown profiles admit no edges instead of silently widening scope.
func TraverseWithProfile(
	ctx context.Context,
	src EdgeSource,
	seedID string,
	dir Direction,
	maxDepth int,
	limit int,
	profile TraversalProfile,
) ([]Hop, error) {
	if maxDepth <= 0 {
		maxDepth = DefaultDepth
	}
	if maxDepth > MaxDepthCeiling {
		maxDepth = MaxDepthCeiling
	}
	if limit <= 0 || limit > MaxResults {
		limit = MaxResults
	}

	visited := map[string]bool{seedID: true}
	out := []Hop{}
	queue := []Hop{{NodeID: seedID, Distance: 0}}

	for len(queue) > 0 && len(out) < limit {
		cur := queue[0]
		queue = queue[1:]
		if cur.Distance >= maxDepth {
			continue
		}

		var edges []codebase.CodeEdge
		var err error
		if dir == Callees {
			edges, err = src.OutEdges(ctx, cur.NodeID)
		} else {
			edges, err = src.InEdges(ctx, cur.NodeID)
		}
		if err != nil {
			return nil, err
		}

		for _, e := range edges {
			if !edgeAllowed(profile, e.Kind) {
				continue
			}
			next := e.DstID
			if dir == Callers {
				next = e.SrcID
			}
			if visited[next] {
				continue
			}
			visited[next] = true
			hop := Hop{NodeID: next, Distance: cur.Distance + 1, ViaKind: e.Kind, Provenance: e.Provenance}
			out = append(out, hop)
			queue = append(queue, hop)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func edgeAllowed(profile TraversalProfile, kind codebase.EdgeKind) bool {
	allowed := map[TraversalProfile]map[codebase.EdgeKind]struct{}{
		TraversalCallFlow: {
			codebase.EdgeCall:              {},
			codebase.EdgeInterfaceDispatch: {},
			codebase.EdgeCallback:          {},
		},
		TraversalTypeImpact: {
			codebase.EdgeImplements: {},
			codebase.EdgeExtends:    {},
			codebase.EdgeEmbeds:     {},
		},
		TraversalReferenceImpact: {
			codebase.EdgeInstantiates:   {},
			codebase.EdgeValueReference: {},
			codebase.EdgeTypeReference:  {},
			codebase.EdgeTemplateUse:    {},
		},
	}
	if profile == TraversalAllCode {
		return true
	}
	_, ok := allowed[profile][kind]
	return ok
}
