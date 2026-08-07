package present

import (
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

// FlowResponse renders a callers / callees / impact traversal fused with the
// reasoning graph. action is "callers" | "callees" | "impact". Every reached
// symbol shows BOTH its code relationship (call vs interface_dispatch, distance,
// heuristic provenance) AND its governance — symbol-level decisions, or the
// honest module-level fallback so a governed node never renders blank.
func FlowResponse(res codeintel.FlowResult, action, seedName string) string {
	var b strings.Builder
	renderIndexState(&b, res.Index)
	renderIndexCoordination(&b, res.IndexRefresh)

	// Candidates: surface them, never silently pick (keystone discipline). The
	// wording distinguishes exact-name overloads from a fuzzy (no-exact) match.
	switch seedResolutionKind(res.SeedResolution) {
	case "candidate_set":
		if seedResolutionDetail(res.SeedResolution) == "fuzzy_candidates" {
			fmt.Fprintf(&b, "## %s — no exact match for `%s`; did you mean:\n\n", titleFor(action), seedName)
		} else {
			fmt.Fprintf(&b, "## %s — `%s` is ambiguous\n\n", titleFor(action), seedName)
			fmt.Fprintf(&b, "%d symbols share this name. Re-query with `file` (and `line`) to disambiguate:\n\n", len(res.Candidates))
		}
		for _, c := range res.Candidates {
			fmt.Fprintf(&b, "- `%s` `%s:%d`%s\n", c.Name, c.FilePath, c.StartLine, receiverSuffix(c))
		}
		return b.String()
	case "seed_not_found":
		fmt.Fprintf(&b, "## %s — `%s` not found\n\n", titleFor(action), seedName)
		b.WriteString("No symbol whose name matches that is in the code index (exact or substring). Check spelling, or pass `file` to scope it.\n")
		return b.String()
	case "seed_unavailable":
		fmt.Fprintf(&b, "## %s — `%s` unavailable\n\n", titleFor(action), seedName)
		fmt.Fprintf(
			&b,
			"Seed resolution is unavailable (%s); this is not evidence that the symbol is absent. Retry after the index capability is restored.\n",
			seedResolutionDetail(res.SeedResolution),
		)
		return b.String()
	}

	fmt.Fprintf(&b, "## %s of `%s` — %s:%d (depth %d, %s, profile %s)\n\n",
		titleFor(action), res.Seed.Name, res.Seed.FilePath, res.Seed.StartLine, res.Depth, directionWord(res.Direction), res.Profile)

	governed := 0
	for _, h := range res.Hops {
		if h.Governed() {
			governed++
		}
	}
	fmt.Fprintf(&b, "%d reached • %d carry recorded reasoning (exact_binding or module_context)\n\n", len(res.Hops), governed)
	fmt.Fprintf(&b, "Resolution: %d resolved • %d ambiguous • %d unresolved\n\n",
		res.Resolution.Resolved, res.Resolution.Ambiguous, res.Resolution.Unresolved)

	if len(res.Hops) == 0 {
		b.WriteString(emptyReachMessage(res.Direction))
		return b.String()
	}

	for _, h := range res.Hops {
		renderFusedHop(&b, h)
	}

	if res.ColdBuilt {
		b.WriteString("\n_(code index built on first query; subsequent queries are warm.)_\n")
	}
	return b.String()
}

// These helpers preserve the legacy zero-value fixture shape while runtime
// services populate the typed union. The fallback can disappear after all
// callers use the canonical envelope.
func seedResolutionKind(outcome codeintel.SeedResolution) string {
	if outcome == nil {
		return "seed_not_found"
	}
	return outcome.Kind().String()
}

func seedResolutionDetail(outcome codeintel.SeedResolution) string {
	if outcome == nil {
		return "legacy_untyped_fixture"
	}
	return outcome.DetailCode()
}

func pathOutcomeKind(outcome codeintel.PathOutcome) string {
	if outcome == nil {
		return "legacy_untyped_path"
	}
	return outcome.Kind().String()
}

func pathOutcomeDetail(outcome codeintel.PathOutcome) string {
	if outcome == nil {
		return "legacy_untyped_fixture"
	}
	return outcome.DetailCode()
}

func renderIndexState(b *strings.Builder, state codebase.IndexState) {
	fmt.Fprintf(b, "Index epoch: %d", state.Epoch)
	if state.Degraded {
		fmt.Fprintf(b, " • degraded: %s", state.DegradedReason)
	}
	if state.Basis.Coverage.Posture != "" {
		fmt.Fprintf(
			b,
			" • coverage: %s (%d discovered, %d admitted, %d skipped)",
			state.Basis.Coverage.Posture,
			state.Basis.Coverage.DiscoveredFiles,
			state.Basis.Coverage.AdmittedFiles,
			state.Basis.Coverage.SkippedFiles,
		)
	}
	if state.Basis.BasisDigest != "" {
		fmt.Fprintf(
			b,
			"\nIndex basis: `sha256:%s` • corpus: `sha256:%s`",
			state.Basis.BasisDigest,
			state.Basis.CorpusDigest,
		)
	}
	if len(state.Basis.Exclusions) > 0 {
		b.WriteString("\nIndex exclusions:")
		limit := min(len(state.Basis.Exclusions), 5)
		for _, exclusion := range state.Basis.Exclusions[:limit] {
			fmt.Fprintf(
				b,
				"\n- `%s` — %s (%d observed, %d limit)",
				exclusion.Path,
				exclusion.Reason,
				exclusion.ObservedBytes,
				exclusion.LimitBytes,
			)
		}
		if remaining := len(state.Basis.Exclusions) - limit; remaining > 0 {
			fmt.Fprintf(b, "\n- … and %d more in this exact basis", remaining)
		}
	}
	b.WriteString("\n\n")
}

// IndexStateResponse renders the shared exact epoch/coverage basis for public
// code-derived responses that do not otherwise use Flow/Node/Explore carriers.
func IndexStateResponse(state codebase.IndexState) string {
	var b strings.Builder
	renderIndexState(&b, state)
	return b.String()
}

// IndexCoordinationResponse renders the closed freshness outcome for public
// code-derived responses that carry it separately from their index basis.
func IndexCoordinationResponse(
	result codeintel.IndexCoordinationResult,
) string {
	var builder strings.Builder
	renderIndexCoordination(&builder, result)
	return builder.String()
}

func renderIndexCoordination(
	builder *strings.Builder,
	result codeintel.IndexCoordinationResult,
) {
	if result.Outcome == "" {
		return
	}
	fmt.Fprintf(
		builder,
		"Index coordination: %s • wait: %s • published epoch: %d\n\n",
		result.Outcome,
		result.WaitDuration.Round(time.Millisecond),
		result.PublishedEpoch,
	)
}

// renderFusedHop prints one reached symbol: its code relationship, then its
// governance (symbol-level decisions, else module-level, else explicitly none).
func renderFusedHop(b *strings.Builder, h codeintel.FusedHop) {
	via := string(h.ViaKind)
	if h.Provenance == codebase.ProvenanceHeuristic {
		via += " ⚠ heuristic"
	}
	fmt.Fprintf(b, "- **%s** `%s:%d` _(%s, d%d)_\n", h.Symbol.Name, h.Symbol.FilePath, h.Symbol.StartLine, via, h.Distance)

	cc := h.Context
	switch {
	case len(cc.Decisions) > 0:
		for _, d := range cc.Decisions {
			fmt.Fprintf(b, "    governs: **%s** `%s` [`exact_binding`]\n", d.Meta.Title, d.Meta.ID)
		}
		if cc.SymbolGranularity != "" && cc.SymbolGranularity != "line-precise" {
			fmt.Fprintf(b, "    _granularity: %s_\n", cc.SymbolGranularity)
		}
	case len(cc.ModuleDecisions) > 0:
		fmt.Fprintf(b, "    module governed by %s _(`module_context`; no exact binding)_\n", moduleDecisionList(cc.ModuleDecisions))
	case len(cc.AffectedPathContextDecisions) > 0:
		b.WriteString(
			"    `affected_path_context` only (not binding authority)\n",
		)
	default:
		b.WriteString("    — no recorded reasoning\n")
	}
}

func titleFor(action string) string {
	switch action {
	case "impact":
		return "Impact"
	case "callers":
		return "Callers"
	default:
		return "Callees"
	}
}

func directionWord(dir codeintel.Direction) string {
	if dir == codeintel.Callers {
		return "inbound"
	}
	return "outbound"
}

// emptyReachMessage is direction-aware: an empty CALLEES set genuinely means a
// leaf (the symbol calls nothing resolvable). But an empty CALLERS set must NOT
// read as "leaf / safe to change" — some call forms (methods on concrete-typed
// fields across packages, reflection, callbacks, function values) aren't
// resolved, so callers may exist but be unshown. Honest coverage: a blank
// inbound result names its own incompleteness rather than implying none exist.
func emptyReachMessage(dir codeintel.Direction) string {
	if dir == codeintel.Callers {
		return "No resolved callers — but some call forms (methods on concrete-typed fields across packages, reflection, callbacks) are not resolved yet, so callers may exist and be unshown. Do NOT read this as \"leaf / safe to change\" — double-check before relying on it.\n"
	}
	return "Nothing reached at this depth — a leaf in this direction (calls nothing resolvable).\n"
}

func receiverSuffix(c codebase.CodeSymbol) string {
	if c.Receiver != "" {
		return fmt.Sprintf(" — receiver `%s`", c.Receiver)
	}
	return ""
}
