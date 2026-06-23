// Package contextgraph fuses haft's reasoning graph (decisions, problems,
// solution variants, notes, invariants, coverage) onto a code location. It is
// the first brick of haft-as-context-graph: where a code-intelligence tool
// answers "how is this code wired", contextgraph answers "what has been
// reasoned and decided about this code" — the part no pure code-graph can do.
//
// Layering (functional core / imperative shell):
//   - this file is the PURE core: immutable value types + BuildCodeContext,
//     no IO, no DB, deterministic given its inputs.
//   - fetch.go is the thin SHELL: it runs the queries and hands the rows here.
package contextgraph

import (
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

// Target is the code location a context is assembled for. A bare File is the
// file-level view; adding Symbol narrows to a single symbol within that file;
// adding Line (1-based) disambiguates overloaded same-name symbols by which
// one's body covers that line — the keystone of honest per-symbol fusion.
type Target struct {
	File   string
	Symbol string
	Line   int
}

// CodeContext is the fused projection of the reasoning graph onto one code
// location — an immutable value, assembled by BuildCodeContext and rendered by
// the presentation layer. Artifacts are grouped by kind so the caller never
// re-derives the taxonomy.
type CodeContext struct {
	Target     Target
	Decisions  []*artifact.Artifact // DecisionRecords governing this code
	Problems   []*artifact.Artifact // ProblemCards framed around this code
	Portfolios []*artifact.Artifact // SolutionPortfolios — the variants explored
	Notes      []*artifact.Artifact // micro-decisions / rationale captured here
	Invariants []graph.Invariant    // invariants linked to this target; file-level views are relevance candidates
	Module     string               // module the file belongs to ("" if none)
	Governed   bool                 // module carries ≥1 decision (vs. blind)

	// ContextInvariants are invariants from decisions governing the file's MODULE
	// that do NOT bind the target symbol directly. Surfaced as context, never as
	// "must hold here", so module-level (e.g. roadmap) invariants are not asserted
	// as constraints on a symbol they do not govern. File-level views keep their
	// legacy invariant payload in Invariants for compatibility; presentation must
	// label that payload as relevance candidates, not symbol-local authority.
	ContextInvariants []graph.Invariant

	// ModuleDecisions are the decisions governing the file's MODULE — surfaced
	// so a symbol with no symbol-level link still shows "module governed by
	// dec-Y" rather than rendering blank (which would read as "safe to change").
	ModuleDecisions []graph.Node

	// SymbolGranularity records HOW symbol-scoped artifacts were matched, so the
	// presentation never overstates precision:
	//   ""                                  — file-level view (no symbol given)
	//   "line-precise"                      — overloads disambiguated by line range
	//   "file+name (overload not disambiguated)" — line-blind fallback; same-name
	//                                         overloads may share this governance
	SymbolGranularity string
}

// Empty reports that no recorded reasoning touches this code — a signal the
// agent can treat as "nothing decided here yet", not "lookup failed".
func (c CodeContext) Empty() bool {
	return len(c.Decisions)+len(c.Problems)+len(c.Portfolios)+len(c.Notes)+len(c.Invariants)+len(c.ContextInvariants) == 0
}

// BuildCodeContext groups a flat, already-deduplicated slice of linked
// artifacts into the kind-typed CodeContext. Pure: same inputs → same value,
// no side effects. Input order is preserved within each group (the shell
// supplies them newest-first), so the core imposes no hidden sort policy.
func BuildCodeContext(target Target, linked []*artifact.Artifact, invariants []graph.Invariant, module string, governed bool) CodeContext {
	cc := CodeContext{
		Target:     target,
		Invariants: invariants,
		Module:     module,
		Governed:   governed,
	}
	for _, a := range linked {
		switch a.Meta.Kind {
		case artifact.KindDecisionRecord:
			cc.Decisions = append(cc.Decisions, a)
		case artifact.KindProblemCard:
			cc.Problems = append(cc.Problems, a)
		case artifact.KindSolutionPortfolio:
			cc.Portfolios = append(cc.Portfolios, a)
		case artifact.KindNote:
			cc.Notes = append(cc.Notes, a)
		}
	}
	return cc
}
