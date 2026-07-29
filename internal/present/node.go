package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/contextgraph"
)

// NodeResponse renders haft_node: every overload of a symbol name, each with its
// freshness-revalidated body, the reasoning fused onto exactly it, an immediate
// caller/callee trail, and (for container types) a member outline. The body is
// only shown when verified byte-exact against disk — a failed verification says
// so rather than printing possibly-stale source.
func NodeResponse(view codeintel.NodeView, lang string) string {
	var b strings.Builder
	renderIndexState(&b, view.Index)

	// No exact/single definition, but fuzzy matches exist → list them to pick
	// from, never auto-expand a dozen bodies.
	if !view.Found && len(view.Candidates) > 0 {
		fmt.Fprintf(&b, "## Node — no exact match for `%s`; did you mean:\n\n", view.Name)
		for _, c := range view.Candidates {
			recv := ""
			if c.Receiver != "" {
				recv = fmt.Sprintf(" (%s)", c.Receiver)
			}
			fmt.Fprintf(&b, "- `%s`%s `%s:%d` _(%s)_\n", c.Name, recv, c.FilePath, c.StartLine, c.Kind)
		}
		return b.String()
	}

	if !view.Found {
		if seedResolutionKind(view.NameResolution) == "seed_unavailable" {
			fmt.Fprintf(&b, "## Node — `%s` unavailable\n\n", view.Name)
			fmt.Fprintf(
				&b,
				"Symbol resolution is unavailable (%s); this is not evidence that the symbol is absent. Retry after the index capability is restored.\n",
				seedResolutionDetail(view.NameResolution),
			)
			return b.String()
		}
		fmt.Fprintf(&b, "## Node — `%s` not found\n\n", view.Name)
		b.WriteString("No symbol whose name matches that is in the code index (exact or substring). Check spelling, or pass `file` to scope it.\n")
		return b.String()
	}

	plural := ""
	if len(view.Overloads) != 1 {
		plural = "s"
	}
	title := fmt.Sprintf("## Node `%s` — %d definition%s", view.Name, len(view.Overloads), plural)
	if view.Fuzzy {
		title = fmt.Sprintf("## Node — no exact match for `%s`; showing the single fuzzy match `%s`", view.Name, view.Overloads[0].Symbol.Name)
	}
	b.WriteString(title + "\n\n")

	for i := range view.Overloads {
		renderOverload(&b, view.Overloads[i], lang)
	}
	if view.ColdBuilt {
		b.WriteString("_(code index built on first query; subsequent queries are warm.)_\n")
	}
	return b.String()
}

func renderOverload(b *strings.Builder, ol codeintel.NodeOverload, lang string) {
	sym := ol.Symbol
	recv := ""
	if sym.Receiver != "" {
		recv = fmt.Sprintf(" (%s)", sym.Receiver)
	}
	fmt.Fprintf(b, "### %s%s — `%s:%d-%d` _(%s)_\n", sym.Name, recv, sym.FilePath, sym.StartLine, sym.EndLine, sym.Kind)
	fmt.Fprintf(b, "Resolution: %d resolved • %d ambiguous • %d unresolved\n",
		ol.Resolution.Resolved, ol.Resolution.Ambiguous, ol.Resolution.Unresolved)
	if ol.Reindexed {
		b.WriteString("_file was edited — re-indexed for fresh byte offsets before slicing_\n")
	}

	renderHopGovernance(b, ol.Context)
	renderTrail(b, "Called by", ol.Callers)
	renderTrail(b, "Calls", ol.Callees)

	if len(ol.Members) > 0 {
		b.WriteString("\nMembers:\n")
		for _, m := range ol.Members {
			fmt.Fprintf(b, "- %s `%s:%d`\n", m.Name, m.FilePath, m.StartLine)
		}
	}

	b.WriteString("\n")
	if !ol.BodyOK {
		b.WriteString("⚠ source could not be verified byte-exact against disk (file changing, or symbol removed) — not shown to avoid stale source. Re-run.\n\n")
		return
	}
	fmt.Fprintf(b, "```%s\n%s\n```\n\n", lang, ol.Body)
}

// renderHopGovernance prints the fusion for one overload, same discipline as the
// flow view: symbol-level decisions, else module-level, else explicitly none.
func renderHopGovernance(b *strings.Builder, cc contextgraph.CodeContext) {
	switch {
	case len(cc.Decisions) > 0:
		b.WriteString("`exact_binding` decisions:\n")
		for _, d := range cc.Decisions {
			fmt.Fprintf(b, "- **%s** `%s`\n", d.Meta.Title, d.Meta.ID)
		}
		if cc.SymbolGranularity != "" && cc.SymbolGranularity != "line-precise" {
			fmt.Fprintf(b, "_granularity: %s_\n", cc.SymbolGranularity)
		}
	case len(cc.ModuleDecisions) > 0:
		fmt.Fprintf(b, "`module_context` — Module governed by %s _(no exact binding)_\n", moduleDecisionList(cc.ModuleDecisions))
	case len(cc.AffectedPathContextDecisions) > 0:
		b.WriteString(
			"`affected_path_context` exists, but no exact binding or module authority was found.\n",
		)
	default:
		b.WriteString("No recorded reasoning touches this definition.\n")
	}
}

func renderTrail(b *strings.Builder, label string, hops []codeintel.FusedHop) {
	if len(hops) == 0 {
		return
	}
	parts := make([]string, 0, len(hops))
	for _, h := range hops {
		via := ""
		if h.ViaKind == "interface_dispatch" {
			via = " via dispatch"
		}
		parts = append(parts, fmt.Sprintf("`%s` (%s:%d%s)", h.Symbol.Name, h.Symbol.FilePath, h.Symbol.StartLine, via))
	}
	fmt.Fprintf(b, "%s: %s\n", label, strings.Join(parts, ", "))
}
