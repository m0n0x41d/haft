package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graph"
)

// CodeContextResponse renders the fused "what haft knows about this code" view
// for an agent exploring a file or symbol — decisions, problems, solution
// variants, notes, and invariants that touch it, plus module coverage. Pure
// formatting; no IO.
func CodeContextResponse(cc contextgraph.CodeContext) string {
	return CodeContextResponseWithOptions(cc, CodeContextRenderOptions{
		Full:                  false,
		InvariantLimit:        12,
		ContextInvariantLimit: 8,
	})
}

// CodeContextResponseFull renders the complete context projection for explicit
// full-mode calls. Default code_context stays compact because file-level
// invariants can grow into hundreds of bullets on governed modules.
func CodeContextResponseFull(cc contextgraph.CodeContext) string {
	return CodeContextResponseWithOptions(cc, CodeContextRenderOptions{
		Full:                  true,
		InvariantLimit:        0,
		ContextInvariantLimit: 0,
	})
}

// CodeContextRenderOptions controls only presentation volume. The underlying
// contextgraph fetch remains unchanged, so compact mode is reversible by
// re-rendering the same value with Full=true.
type CodeContextRenderOptions struct {
	Full                  bool
	InvariantLimit        int
	ContextInvariantLimit int
}

// CodeContextResponseWithOptions renders the fused code context using explicit
// volume controls. Passing Full=true disables limits.
func CodeContextResponseWithOptions(cc contextgraph.CodeContext, options CodeContextRenderOptions) string {
	var b strings.Builder

	target := cc.Target.File
	if cc.Target.Symbol != "" {
		target = fmt.Sprintf("%s :: %s", cc.Target.File, cc.Target.Symbol)
	}
	fmt.Fprintf(&b, "## Code context — %s\n\n", target)

	if cc.Module != "" {
		if len(cc.ModuleDecisions) > 0 {
			fmt.Fprintf(&b, "Module `%s`: governed by %s\n\n", cc.Module, moduleDecisionList(cc.ModuleDecisions))
		} else {
			fmt.Fprintf(&b, "Module `%s`: blind — no decisions govern this module\n\n", cc.Module)
		}
	}

	if cc.Target.Symbol != "" && cc.SymbolGranularity != "" {
		fmt.Fprintf(&b, "Symbol governance granularity: %s\n\n", cc.SymbolGranularity)
	}

	if cc.Empty() {
		b.WriteString("No recorded reasoning touches this code yet — nothing decided, framed, or noted here.\n")
		b.WriteString("About to make a non-trivial change? Consider /h-frame or /h-note so the next reader sees why.\n")
		return b.String()
	}

	renderGoverningDecisions(&b, cc.Decisions)
	renderContextArtifacts(&b, "Problems framed around it", cc.Problems)
	renderContextArtifacts(&b, "Solution variants explored", cc.Portfolios)
	renderContextArtifacts(&b, "Notes", cc.Notes)

	renderInvariantSection(&b, "### Invariants that must hold here", cc.Invariants, options.InvariantLimit, options.Full)
	renderInvariantSection(&b, "### Module context — invariants of module-governing decisions (may not bind this symbol)", cc.ContextInvariants, options.ContextInvariantLimit, options.Full)

	return b.String()
}

func renderInvariantSection(b *strings.Builder, heading string, invariants []graph.Invariant, limit int, full bool) {
	if len(invariants) == 0 {
		return
	}

	b.WriteString(heading)
	b.WriteString("\n")

	visible := invariants
	if !full && limit > 0 && len(invariants) > limit {
		visible = invariants[:limit]
	}

	for _, inv := range visible {
		fmt.Fprintf(b, "- %s _(from %s)_\n", inv.Text, inv.DecisionTitle)
	}

	if omitted := len(invariants) - len(visible); omitted > 0 {
		fmt.Fprintf(b, "- ... %d more omitted; re-run `haft_query(action=\"code_context\", ..., full=true)` for the complete invariant list.\n", omitted)
	}

	b.WriteString("\n")
}

// moduleDecisionList renders the module's governing decisions inline, each ID
// paired with its title (FPF A.7 re-grounding) — so a governed module is never
// a bare "governed" with no handle to the why.
func moduleDecisionList(decisions []graph.Node) string {
	parts := make([]string, 0, len(decisions))
	for _, d := range decisions {
		parts = append(parts, fmt.Sprintf("`%s` (%s)", d.ID, d.Name))
	}
	return strings.Join(parts, ", ")
}

// renderGoverningDecisions lists the decisions governing this code, each tagged
// with how much it can still be trusted: its non-active status (refresh-due /
// superseded) and how many of its predictions remain unverified. So a decision
// whose claims were never checked does not read as fully authoritative — the
// trust-decay signal surfaces exactly where the agent reads the rationale.
func renderGoverningDecisions(b *strings.Builder, items []*artifact.Artifact) {
	if len(items) == 0 {
		return
	}
	b.WriteString("### Decisions governing this code\n")
	for _, a := range items {
		suffix := ""
		if a.Meta.Status != artifact.StatusActive {
			suffix = fmt.Sprintf(" [%s]", a.Meta.Status)
		}
		fmt.Fprintf(b, "- **%s** `%s`%s%s\n", a.Meta.Title, a.Meta.ID, suffix, decisionVerificationTag(a))
	}
	b.WriteString("\n")
}

// decisionVerificationTag reports how many of a decision's predictions remain
// unverified, so a governing decision whose claims were never checked is not read
// as authoritative. Pure over the artifact's stored claims; empty when there are
// no claims or all are verified.
func decisionVerificationTag(a *artifact.Artifact) string {
	df := a.UnmarshalDecisionFields()
	if len(df.Claims) == 0 {
		return ""
	}
	unverified := 0
	for _, c := range df.Claims {
		if c.Status == "" || c.Status == artifact.ClaimStatusUnverified {
			unverified++
		}
	}
	if unverified == 0 {
		return ""
	}
	return fmt.Sprintf(" · %d/%d predictions unverified", unverified, len(df.Claims))
}

// renderContextArtifacts lists a kind-group, pairing every artifact ID with
// its human-readable title (FPF A.7 re-grounding) and flagging non-active
// status.
func renderContextArtifacts(b *strings.Builder, heading string, items []*artifact.Artifact) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s\n", heading)
	for _, a := range items {
		suffix := ""
		if a.Meta.Status != artifact.StatusActive {
			suffix = fmt.Sprintf(" [%s]", a.Meta.Status)
		}
		fmt.Fprintf(b, "- **%s** `%s`%s\n", a.Meta.Title, a.Meta.ID, suffix)
	}
	b.WriteString("\n")
}
