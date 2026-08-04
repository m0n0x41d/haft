package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/contextgraph"
)

// ExploreResponse renders the capstone single-call view: the fused call-chain
// spine, the blast radius (who breaks + covering decisions), and verbatim seed
// source. The goal is sufficiency — the agent answers "how does this flow and
// what was decided about it" without further reads. Honest: a chain that stops
// at a dispatch boundary says so rather than implying completeness.
func ExploreResponse(res codeintel.ExploreResult, seedName, lang string) string {
	var b strings.Builder
	renderIndexState(&b, res.Index)
	renderIndexCoordination(&b, res.IndexRefresh)

	switch seedResolutionKind(res.SeedResolution) {
	case "candidate_set":
		if seedResolutionDetail(res.SeedResolution) == "fuzzy_candidates" {
			fmt.Fprintf(&b, "## Explore — no exact match for `%s`; did you mean:\n\n", seedName)
		} else {
			fmt.Fprintf(&b, "## Explore — `%s` is ambiguous\n\n", seedName)
			fmt.Fprintf(&b, "%d symbols share this name. Re-query with `file` (and `line`):\n\n", len(res.Candidates))
		}
		for _, c := range res.Candidates {
			fmt.Fprintf(&b, "- `%s` `%s:%d`%s\n", c.Name, c.FilePath, c.StartLine, receiverSuffix(c))
		}
		return b.String()
	case "seed_not_found":
		fmt.Fprintf(&b, "## Explore — `%s` not found\n\n", seedName)
		b.WriteString("No symbol whose name matches that is in the code index (exact or substring). Check spelling, or pass `file` to scope it.\n")
		return b.String()
	case "seed_unavailable":
		fmt.Fprintf(&b, "## Explore — `%s` unavailable\n\n", seedName)
		fmt.Fprintf(
			&b,
			"Seed resolution is unavailable (%s); this is not evidence that the symbol is absent. Retry after the index capability is restored.\n",
			seedResolutionDetail(res.SeedResolution),
		)
		return b.String()
	}

	fmt.Fprintf(&b, "## Explore `%s` — %s:%d\n\n", res.Seed.Name, res.Seed.FilePath, res.Seed.StartLine)
	fmt.Fprintf(&b, "Resolution: %d resolved • %d ambiguous • %d unresolved\n\n",
		res.Resolution.Resolved, res.Resolution.Ambiguous, res.Resolution.Unresolved)

	renderChain(&b, res)
	renderBlastRadius(&b, res.BlastRadius)
	renderSeedSource(&b, res, lang)

	if res.ColdBuilt {
		b.WriteString("_(code index built on first query; subsequent queries are warm.)_\n")
	}
	return b.String()
}

func renderChain(b *strings.Builder, res codeintel.ExploreResult) {
	fmt.Fprintf(b, "### Flow — spine of %d symbol(s)", len(res.Chain))
	bridges := res.ChainOutcome.Stats().BridgeCount()
	if bridges > 0 {
		fmt.Fprintf(b, ", crosses %d interface boundary", bridges)
	}
	b.WriteString("\n")

	for _, step := range res.Chain {
		arrow := "•"
		if step.Distance > 0 {
			arrow = "→"
		}
		via := ""
		if step.Distance > 0 {
			via = fmt.Sprintf(" _(%s", step.ViaKind)
			if step.Bridge() {
				via += " ⚠ heuristic boundary"
			}
			via += ")_"
		}
		recv := ""
		if step.Symbol.Receiver != "" {
			recv = fmt.Sprintf("(%s).", step.Symbol.Receiver)
		}
		fmt.Fprintf(b, "%s **%s%s** `%s:%d`%s\n", arrow, recv, step.Symbol.Name, step.Symbol.FilePath, step.Symbol.StartLine, via)
		renderChainGovernance(b, step.Context)
	}
	switch res.ChainOutcome.Termination().String() {
	case "unresolved_dispatch_boundary":
		b.WriteString("⚠ chain ends at an unresolved dispatch boundary — the flow continues through a dynamic call the static graph cannot resolve. Not shown rather than guessed.\n")
	case "max_hops_reached":
		b.WriteString("⚠ chain reached the configured hop bound — this is a truncated path, not a complete-flow claim.\n")
	case "visit_budget_reached":
		b.WriteString("⚠ chain search reached the visit budget — this partial spine is not a longest-path or no-more-callers claim.\n")
	}
	b.WriteString("\n")
	renderInvariantsInPlay(b, res.Chain)
}

// renderChainGovernance is the per-symbol fusion on the spine — the moat: code
// flow interleaved with the DECISIONS governing each step. Invariants are NOT
// shown per node — they are module-scoped, so repeating them on every hop buries
// the signal; they are collected once in renderInvariantsInPlay below.
func renderChainGovernance(b *strings.Builder, cc contextgraph.CodeContext) {
	switch {
	case len(cc.Decisions) > 0:
		titles := make([]string, 0, len(cc.Decisions))
		for _, d := range cc.Decisions {
			titles = append(titles, fmt.Sprintf("%s `%s`", d.Meta.Title, d.Meta.ID))
		}
		fmt.Fprintf(b, "    ⮡ `exact_binding`: %s\n", strings.Join(titles, "; "))
	case len(cc.ModuleDecisions) > 0:
		fmt.Fprintf(b, "    ⮡ `module_context`: %s\n", moduleDecisionList(cc.ModuleDecisions))
	case len(cc.AffectedPathContextDecisions) > 0:
		b.WriteString(
			"    ⮡ `affected_path_context` only (not binding authority)\n",
		)
	}
}

// renderInvariantsInPlay lists the UNIQUE invariants across all on-chain symbols
// once (deduplicated by text), so the constraints the flow must respect are
// visible without being repeated on every hop. First-seen order is preserved.
func renderInvariantsInPlay(b *strings.Builder, chain []codeintel.ChainStep) {
	seen := map[string]bool{}
	type inv struct{ text, from string }
	var uniq []inv
	for _, step := range chain {
		for _, iv := range step.Context.Invariants {
			if seen[iv.Text] {
				continue
			}
			seen[iv.Text] = true
			uniq = append(uniq, inv{iv.Text, iv.DecisionTitle})
		}
	}
	if len(uniq) == 0 {
		return
	}
	fmt.Fprintf(b, "### Invariants in play (%d)\n", len(uniq))
	for _, iv := range uniq {
		fmt.Fprintf(b, "- %s _(from %s)_\n", iv.text, iv.from)
	}
	b.WriteString("\n")
}

func renderBlastRadius(b *strings.Builder, callers []codeintel.FusedHop) {
	if len(callers) == 0 {
		b.WriteString("### Blast radius\nNo resolved callers — but some call forms (methods on concrete-typed fields across packages, reflection, callbacks) are not resolved yet, so callers may exist and be unshown. Do NOT read this as a safe-to-change leaf without double-checking.\n\n")
		return
	}
	governed := 0
	for _, h := range callers {
		if h.Governed() {
			governed++
		}
	}
	fmt.Fprintf(b, "### Blast radius — %d direct caller(s), %d governed\n", len(callers), governed)
	for _, h := range callers {
		fmt.Fprintf(b, "- `%s` (%s:%d)", h.Symbol.Name, h.Symbol.FilePath, h.Symbol.StartLine)
		switch {
		case len(h.Context.Decisions) > 0:
			b.WriteString(" — `exact_binding`: ")
			ids := make([]string, 0, len(h.Context.Decisions))
			for _, d := range h.Context.Decisions {
				ids = append(ids, d.Meta.ID)
			}
			b.WriteString(strings.Join(ids, ", "))
		case len(h.Context.ModuleDecisions) > 0:
			b.WriteString(" — `module_context`")
		case len(h.Context.AffectedPathContextDecisions) > 0:
			b.WriteString(" — `affected_path_context` only")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func renderSeedSource(b *strings.Builder, res codeintel.ExploreResult, lang string) {
	b.WriteString("### Seed source\n")
	if !res.SeedBodyOK {
		b.WriteString("⚠ source could not be verified byte-exact against disk — not shown to avoid stale source.\n\n")
		return
	}
	fmt.Fprintf(b, "```%s\n%s\n```\n\n", lang, res.SeedBody)
}

// ExploreBagResponse renders a multi-seed explore: how a bag of named symbols
// connects. Each adjacent pair is a leg — the fused path between them, or an
// honest "no static path". Seeds that did not resolve to one symbol are listed
// for the caller to disambiguate.
func ExploreBagResponse(res codeintel.ExploreBagResult) string {
	var b strings.Builder
	renderIndexState(&b, res.Index)
	b.WriteString("## Explore — connecting flow among seeds\n\n")
	fmt.Fprintf(&b, "Resolution across resolved seeds: %d resolved • %d ambiguous • %d unresolved\n\n",
		res.Resolution.Resolved, res.Resolution.Ambiguous, res.Resolution.Unresolved)

	if len(res.Unresolved) > 0 {
		fmt.Fprintf(&b, "Could not place these seeds (not found, or ambiguous — re-query each precisely): %s\n\n", strings.Join(res.Unresolved, ", "))
	}
	if len(res.Seeds) < 2 {
		b.WriteString("Need at least 2 resolvable seeds to connect a flow.\n")
		return b.String()
	}

	for _, leg := range res.Legs {
		if leg.Direction.String() == "none" {
			fmt.Fprintf(
				&b,
				"### %s ⇸ %s — no selected static path\nForward: %s (%s); reverse: %s (%s). No path was invented across a missing or bounded result.\n\n",
				leg.From.Name,
				leg.To.Name,
				pathOutcomeKind(leg.Forward),
				pathOutcomeDetail(leg.Forward),
				pathOutcomeKind(leg.Reverse),
				pathOutcomeDetail(leg.Reverse),
			)
			continue
		}
		heading := fmt.Sprintf("%s → %s", leg.From.Name, leg.To.Name)
		if leg.Direction.String() == "reverse" {
			heading = fmt.Sprintf("%s → %s (the call path runs this direction)", leg.To.Name, leg.From.Name)
		}
		fmt.Fprintf(&b, "### %s — %d hop(s)\n", heading, maxInt(len(leg.Steps)-1, 0))
		for _, step := range leg.Steps {
			recv := ""
			if step.Symbol.Receiver != "" {
				recv = fmt.Sprintf("(%s).", step.Symbol.Receiver)
			}
			via := ""
			if step.Distance > 0 {
				via = fmt.Sprintf(" _(%s", step.ViaKind)
				if step.Bridge() {
					via += " ⚠ heuristic"
				}
				via += ")_"
			}
			fmt.Fprintf(&b, "- **%s%s** `%s:%d`%s\n", recv, step.Symbol.Name, step.Symbol.FilePath, step.Symbol.StartLine, via)
			renderChainGovernance(&b, step.Context)
		}
		b.WriteString("\n")
	}
	if res.ColdBuilt {
		b.WriteString("_(code index built on first query; subsequent queries are warm.)_\n")
	}
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
