package contextgraph

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

// FetchCodeContext is the imperative shell: it gathers the linked artifacts,
// invariants, and module/coverage status for a Target, then defers all shaping
// to the pure BuildCodeContext. The two stores wrap the same DB connection;
// the caller owns their lifecycle.
//
// When a Symbol is set, the result is the UNION of file-level and
// symbol-level links (deduplicated): the agent sees both what governs the file
// and what targets the exact symbol, never losing the broader context.
func FetchCodeContext(ctx context.Context, store *artifact.Store, g *graph.Store, target Target) (CodeContext, error) {
	if target.File == "" {
		return CodeContext{}, fmt.Errorf("file is required")
	}

	fileLinked, err := store.SearchByAffectedFile(ctx, target.File)
	if err != nil {
		return CodeContext{}, fmt.Errorf("fetch file-linked artifacts: %w", err)
	}
	linked := fileLinked
	granularity := ""
	var symbolLinked []*artifact.Artifact

	if target.Symbol != "" {
		sl, gran, err := fetchSymbolLinked(ctx, store, target)
		if err != nil {
			return CodeContext{}, err
		}
		symbolLinked = sl
		granularity = gran
		linked = dedupeArtifacts(append(fileLinked, symbolLinked...))
	}

	invariants, err := g.FindInvariantsForFile(ctx, target.File)
	if err != nil {
		return CodeContext{}, fmt.Errorf("fetch invariants: %w", err)
	}

	module, moduleDecisions := moduleStatus(ctx, g, target.File)

	// A symbol view asserts "must hold here" ONLY for invariants whose decision
	// governs the symbol directly; the file's module-level invariants become
	// context, not constraints on this symbol.
	binding, contextInv := partitionInvariants(invariants, target.Symbol, symbolLinked)

	cc := BuildCodeContext(target, linked, binding, module, len(moduleDecisions) > 0)
	cc.SymbolGranularity = granularity
	cc.ModuleDecisions = moduleDecisions
	cc.ContextInvariants = contextInv
	return cc, nil
}

// partitionInvariants splits the file's invariants into those that BIND a
// symbol target and those that are merely module/file CONTEXT. For a file-level
// view (no symbol), the legacy payload stays in the primary invariant lane so
// existing callers keep seeing it, but presentation labels it as file-level
// relevance candidates. For a symbol view only invariants whose source decision
// governs the symbol directly (matched via affected_symbols) bind it; the rest
// are context, so a roadmap invariant is never asserted as a constraint on a
// symbol it does not govern. Pure.
func partitionInvariants(invariants []graph.Invariant, symbol string, symbolLinked []*artifact.Artifact) (binding, contextInv []graph.Invariant) {
	if symbol == "" {
		return invariants, nil
	}
	symbolDecisions := make(map[string]bool, len(symbolLinked))
	for _, a := range symbolLinked {
		if a != nil && a.Meta.Kind == artifact.KindDecisionRecord {
			symbolDecisions[a.Meta.ID] = true
		}
	}
	for _, inv := range invariants {
		if symbolDecisions[inv.DecisionID] {
			binding = append(binding, inv)
		} else {
			contextInv = append(contextInv, inv)
		}
	}
	return binding, contextInv
}

// fetchSymbolLinked resolves the symbol-scoped artifacts for a target, preferring
// the LINE-AWARE join (which disambiguates overloads) and falling back to the
// line-blind join only when no line is given or no line-range row matches — in
// which case it reports "file+name" granularity so the caller never presents
// false per-symbol precision.
func fetchSymbolLinked(ctx context.Context, store *artifact.Store, target Target) ([]*artifact.Artifact, string, error) {
	if target.Line > 0 {
		precise, err := store.SearchByAffectedSymbolAt(ctx, target.Symbol, target.File, target.Line)
		if err != nil {
			return nil, "", fmt.Errorf("fetch line-aware symbol artifacts: %w", err)
		}
		if len(precise) > 0 {
			return precise, "line-precise", nil
		}
		// No line-range row covered this line (symbol not symbol-baselined, or
		// legacy rows without an end line): fall back, labeled.
	}
	blind, err := store.SearchByAffectedSymbol(ctx, target.Symbol, target.File)
	if err != nil {
		return nil, "", fmt.Errorf("fetch symbol-linked artifacts: %w", err)
	}
	return blind, "file+name (overload not disambiguated)", nil
}

// moduleStatus resolves the file's module and the decisions governing it.
// Best-effort: a missing module is not an error — the file simply has no
// module-level coverage. Returns the decisions (not just a bool) so the
// presentation can name them instead of rendering a governed module blank.
func moduleStatus(ctx context.Context, g *graph.Store, file string) (module string, moduleDecisions []graph.Node) {
	node, err := g.FindModuleForFile(ctx, file)
	if err != nil || node == nil {
		return "", nil
	}
	decisions, err := g.FindDecisionsForModule(ctx, node.ID)
	if err != nil {
		return node.Name, nil
	}
	return node.Name, decisions
}

// dedupeArtifacts removes duplicate artifacts by ID, preserving first-seen
// order (file-linked first, then symbol-only additions).
func dedupeArtifacts(in []*artifact.Artifact) []*artifact.Artifact {
	seen := make(map[string]bool, len(in))
	out := make([]*artifact.Artifact, 0, len(in))
	for _, a := range in {
		if a == nil || seen[a.Meta.ID] {
			continue
		}
		seen[a.Meta.ID] = true
		out = append(out, a)
	}
	return out
}
