package codeintel

import (
	"context"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/contextgraph"
)

// TrailLimit caps the caller/callee trail shown per overload — a node view is a
// focused detail page, not a full traversal (that is callers/callees/impact).
const TrailLimit = 5

// NodeOverload is one same-name definition with everything needed to understand
// it without a Read: its byte-exact (freshness-revalidated) body, the reasoning
// fused onto exactly it (per-overload, line-precise), its immediate call trail,
// and — for container types — its member outline.
type NodeOverload struct {
	Symbol     codebase.CodeSymbol
	Body       string
	BodyOK     bool // slice verified byte-exact against the on-disk hash
	Reindexed  bool // the file was stale and was re-indexed to get fresh offsets
	Context    contextgraph.CodeContext
	Callers    []FusedHop
	Callees    []FusedHop
	Members    []codebase.CodeSymbol // container kinds only (methods on this type)
	Resolution codebase.ResolutionCounts
}

// NodeView is the full haft_node response: every overload of a name, each
// self-contained. Unlike a Flow seed, a node is never "ambiguous" — it shows
// ALL overloads rather than asking the caller to pick.
type NodeView struct {
	Name           string
	Found          bool
	NameResolution SeedResolution
	Overloads      []NodeOverload
	Candidates     []codebase.CodeSymbol // fuzzy matches when no exact name resolved and >1 hit
	Fuzzy          bool                  // the shown definitions / candidates came from the fuzzy fallback
	ColdBuilt      bool
	Index          codebase.IndexState
}

// Node assembles the detail view for a symbol name: all overloads (or the one
// covering file+line), each with a freshness-revalidated body, per-overload
// fusion, and an immediate caller/callee trail.
func (s *Service) Node(ctx context.Context, projectRoot, name, file string, line int) (NodeView, error) {
	return retryIndexQuery(func() (NodeView, error) {
		return s.nodeOnce(ctx, projectRoot, name, file, line)
	})
}

func (s *Service) nodeOnce(
	ctx context.Context,
	projectRoot string,
	name string,
	file string,
	line int,
) (result NodeView, resultErr error) {
	cold, err := s.EnsureIndex(ctx, projectRoot)
	if err != nil {
		return NodeView{}, err
	}
	indexMu.RLock()
	defer indexMu.RUnlock()
	indexState, err := s.scanner.CurrentIndexState(ctx)
	if err != nil {
		return NodeView{}, err
	}
	defer func() {
		if resultErr != nil {
			return
		}
		if err := s.ConfirmIndexState(ctx, indexState); err != nil {
			result = NodeView{}
			resultErr = err
		}
	}()
	if indexState.Epoch == 0 {
		resolution, err := unavailableSeedForIndex(name, indexState)
		if err != nil {
			return NodeView{}, err
		}
		return NodeView{
			Name:           name,
			NameResolution: resolution,
			ColdBuilt:      cold,
			Index:          indexState,
		}, nil
	}
	scope, err := traversalScopeForIndex(indexState)
	if err != nil {
		return NodeView{}, err
	}
	overloads, candidates, fuzzy, err := s.resolveOverloads(ctx, name, file, line)
	if err != nil {
		return NodeView{}, err
	}
	view := NodeView{Name: name, ColdBuilt: cold, Fuzzy: fuzzy, Candidates: candidates, Index: indexState}
	for _, sym := range overloads {
		ol, err := s.buildOverload(ctx, projectRoot, sym)
		if err != nil {
			return NodeView{}, err
		}
		view.Overloads = append(view.Overloads, ol)
	}
	view.Found = len(view.Overloads) > 0
	if view.Found {
		return view, nil
	}
	seedResolution, displayCandidates, err := classifySeedResolution(
		name,
		file,
		line,
		codebase.CodeSymbol{},
		candidates,
		fuzzy,
		scope,
	)
	if err != nil {
		return NodeView{}, err
	}
	view.NameResolution = seedResolution
	view.Candidates = displayCandidates
	return view, nil
}

// resolveOverloads returns every definition the node view should show: the one
// covering file+line if both given, else all same-name defs in the file (if
// given), else all same-name defs across the project.
// resolveOverloads returns the definitions to SHOW (overloads), a fuzzy
// CANDIDATE list when no exact name matched and more than one fuzzy hit exists,
// and whether the result came from the fuzzy fallback. Exact name → show all its
// overloads. Single fuzzy hit → show it (fuzzy=true). Multiple fuzzy hits → no
// overloads, candidates listed (never auto-expand a dozen fuzzy matches).
func (s *Service) resolveOverloads(ctx context.Context, name, file string, line int) (overloads, candidates []codebase.CodeSymbol, fuzzy bool, err error) {
	if file != "" && line > 0 {
		syms, err := s.symbols.GetByFile(ctx, file)
		if err != nil {
			return nil, nil, false, err
		}
		if sym, ok := symbolCoveringLine(syms, line); ok {
			return []codebase.CodeSymbol{sym}, nil, false, nil
		}
	}
	bareName, receiver := splitQualifiedSymbolName(name)
	exact, err := s.symbols.GetByName(ctx, bareName)
	if err != nil {
		return nil, nil, false, err
	}
	exact = filterByReceiver(exact, receiver)
	if file != "" {
		exact = filterByFile(exact, file)
	}
	if len(exact) > 0 {
		return exact, nil, false, nil
	}
	fuzzyHits, err := s.symbols.SearchSymbols(ctx, bareName, 12)
	if err != nil {
		return nil, nil, false, err
	}
	fuzzyHits = filterByReceiver(fuzzyHits, receiver)
	if file != "" {
		fuzzyHits = filterByFile(fuzzyHits, file)
	}
	switch len(fuzzyHits) {
	case 0:
		return nil, nil, false, nil
	case 1:
		return fuzzyHits, nil, true, nil
	default:
		return nil, fuzzyHits, true, nil
	}
}

// buildOverload assembles one overload: freshness-revalidated body + fusion +
// trail + (for containers) member outline.
func (s *Service) buildOverload(ctx context.Context, projectRoot string, sym codebase.CodeSymbol) (NodeOverload, error) {
	body, fresh, reindexed, freshSym, err := s.freshBody(ctx, projectRoot, sym)
	if err != nil {
		return NodeOverload{}, err
	}
	ol := NodeOverload{
		Symbol:    freshSym,
		Body:      string(body),
		BodyOK:    fresh,
		Reindexed: reindexed,
	}

	cc, err := contextgraph.FetchCodeContext(ctx, s.art, s.graph, contextgraph.Target{
		File:     freshSym.FilePath,
		Symbol:   freshSym.Name,
		AnchorID: freshSym.AnchorID,
		Line:     freshSym.StartLine,
	})
	if err != nil {
		return NodeOverload{}, err
	}
	ol.Context = cc
	ol.Resolution, err = s.edges.ResolutionCountsForSource(ctx, freshSym.ID)
	if err != nil {
		return NodeOverload{}, err
	}

	callers, err := s.trail(ctx, freshSym.ID, Callers)
	if err != nil {
		return NodeOverload{}, err
	}
	callees, err := s.trail(ctx, freshSym.ID, Callees)
	if err != nil {
		return NodeOverload{}, err
	}
	ol.Callers = callers
	ol.Callees = callees

	if isContainerKind(freshSym.Kind) {
		members, err := s.symbols.GetByDir(ctx, filepath.Dir(freshSym.FilePath))
		if err != nil {
			return NodeOverload{}, err
		}
		ol.Members = methodsOf(members, freshSym.Name)
	}
	return ol, nil
}

// trail walks one immediate hop and fuses each reached node (≤TrailLimit).
func (s *Service) trail(ctx context.Context, seedID string, dir Direction) ([]FusedHop, error) {
	hops, err := Traverse(ctx, s.edges, seedID, dir, 1, TrailLimit)
	if err != nil {
		return nil, err
	}
	out := make([]FusedHop, 0, len(hops))
	for _, h := range hops {
		fused, ok, err := s.fuse(ctx, h)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, fused)
		}
	}
	return out, nil
}

// freshBody is the P3 keystone: it returns a byte-exact body that has been
// re-read from disk AND re-hashed, never a slice from possibly-stale stored
// offsets. If the on-disk content no longer matches the stored hash, the file
// is re-indexed and the symbol re-resolved so the offsets are fresh before
// slicing — preventing confidently-wrong source on an actively-edited file.
func (s *Service) freshBody(ctx context.Context, projectRoot string, sym codebase.CodeSymbol) (body []byte, fresh, reindexed bool, freshSym codebase.CodeSymbol, err error) {
	abs := filepath.Join(projectRoot, sym.FilePath)
	content, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, false, sym, nil // unreadable — surface as un-verified, not fatal
	}
	if b, ok := codebase.VerifyBody(content, sym); ok {
		return b, true, false, sym, nil
	}

	// The file changed after the atomic refresh but before source slicing. Do not
	// perform a private per-file write that would bypass the epoch transaction.
	// Return an honest unverified body; the next query refreshes the closure.
	return nil, false, false, sym, nil
}

// methodsOf returns the symbols whose receiver is the given type name — the
// member outline for a container kind. Pure.
func methodsOf(syms []codebase.CodeSymbol, typeName string) []codebase.CodeSymbol {
	out := make([]codebase.CodeSymbol, 0)
	for _, sym := range syms {
		if sym.Receiver == typeName {
			out = append(out, sym)
		}
	}
	return out
}

// isContainerKind reports whether a symbol kind is a type that has members
// (struct / interface / class) rather than a callable.
func isContainerKind(kind string) bool {
	switch kind {
	case "type", "type_alias", "interface", "class", "struct", "enum":
		return true
	default:
		return false
	}
}
