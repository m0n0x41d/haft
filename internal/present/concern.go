package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

// ConcernDiscoveryResponse renders separable lexical, graph, bridge,
// governance, and call evidence. Ranked candidates are never called selected;
// exact identity is rendered only for the closed stable-anchor/qualified-name
// outcome.
func ConcernDiscoveryResponse(
	result codeintel.ConcernDiscoveryResult,
) string {
	var builder strings.Builder
	renderIndexState(&builder, result.Index)
	fmt.Fprintf(
		&builder,
		"## Concern discovery — %q\n\n",
		result.Query.Raw(),
	)
	fmt.Fprintf(
		&builder,
		"Normalized terms: `%s`\n\n",
		strings.Join(result.Query.Terms(), "`, `"),
	)
	switch result.Outcome.String() {
	case codeintel.ConcernResolvedExactIdentity:
		builder.WriteString(
			"Exact identity was recovered from an explicit stable anchor or " +
				"unique qualified name; graph rank did not grant identity.\n\n",
		)
		renderConcernCandidates(
			&builder,
			result.Candidates(),
			result.Fused.Budget().MaxCandidates(),
		)
	case codeintel.ConcernCandidates:
		renderConcernCandidates(
			&builder,
			result.Candidates(),
			result.Fused.Budget().MaxCandidates(),
		)
	case codeintel.ConcernNoCandidates:
		builder.WriteString(
			"No lexical or reasoning-graph symbol candidates were found " +
				"within this complete published basis. This does not select " +
				"or invent a symbol.\n",
		)
	case codeintel.ConcernIncompleteBasis:
		fmt.Fprintf(
			&builder,
			"No candidate is visible under the incomplete index basis (%s). "+
				"This is not evidence that a matching symbol is absent.\n",
			result.Outcome.DetailCode(),
		)
	case codeintel.ConcernIndexUnavailable:
		builder.WriteString(
			"Concern discovery is unavailable because there is no published " +
				"code-index epoch. This is not evidence that a matching " +
				"symbol is absent.\n",
		)
	}
	if result.ColdBuilt {
		builder.WriteString(
			"\n_(code index built on first query; subsequent queries are warm.)_\n",
		)
	}
	if result.Basis.ReplayRef != "" {
		fmt.Fprintf(
			&builder,
			"\nFusion replay: `%s` · graph `%s` · %d code edge(s), "+
				"%d reasoning link(s), %d file bridge(s), %d exact symbol bridge(s)",
			result.Basis.ReplayRef,
			result.Basis.GraphDigest,
			result.Basis.CodeEdges,
			result.Basis.ReasoningLinks,
			result.Basis.AffectedFileBridges,
			result.Basis.ExactSymbolBridges,
		)
		fmt.Fprintf(
			&builder,
			" · induced %d/%d graph node(s), hops≤%d, cap=%d",
			result.Basis.GraphNodes,
			result.Basis.FullGraphNodes,
			result.Basis.GraphInductionMaxHops,
			result.Basis.GraphInductionMaxNodes,
		)
		if result.Basis.GraphNodeCapReached {
			builder.WriteString(
				" (node cap reached)",
			)
		}
		if result.Basis.GraphNodes < result.Basis.FullGraphNodes {
			builder.WriteString(
				" (bounded projection; missing graph support is not absence evidence)",
			)
		}
		if result.Basis.StaleSymbolBindings > 0 {
			fmt.Fprintf(
				&builder,
				" · %d stale symbol binding(s) dropped",
				result.Basis.StaleSymbolBindings,
			)
		}
		builder.WriteString(".\n")
	}
	return builder.String()
}

func renderConcernCandidates(
	builder *strings.Builder,
	candidates []codeintel.ConcernCandidate,
	maxCandidates int,
) {
	fmt.Fprintf(
		builder,
		"%d bounded fused candidate(s), applied max_candidates=%d. "+
			"Order is deterministic retrieval "+
			"precedence, not identity selection:\n\n",
		len(candidates),
		maxCandidates,
	)
	for _, candidate := range candidates {
		symbol := candidate.Symbol()
		lexical, lexicalPresent := candidate.Lexical().Candidate()
		lexicalTier := "none"
		termCoverage := "0/0"
		fieldCoverage := "0/0"
		if lexicalPresent {
			lexicalTier = lexical.Tier().String()
			termCoverage = fmt.Sprintf(
				"%d/%d",
				lexical.Coverage().Covered(),
				lexical.Coverage().Total(),
			)
			fieldCoverage = fmt.Sprintf(
				"%d/%d",
				lexical.FieldCoverage().Covered(),
				lexical.FieldCoverage().Total(),
			)
		}
		fmt.Fprintf(
			builder,
			"- `%s` `%s:%d` — lexical=%s; graph=%s/%.8f; "+
				"kind=%s; lane=%s; terms=%s; field_terms=%s; anchor=`%s`\n",
			symbol.QualifiedName,
			symbol.FilePath,
			symbol.StartLine,
			lexicalTier,
			candidate.Graph().Kind(),
			candidate.Graph().Combined(),
			symbol.Kind,
			candidate.SourceLane(),
			termCoverage,
			fieldCoverage,
			symbol.AnchorID,
		)
		fmt.Fprintf(
			builder,
			"  - evidence lanes: %s\n",
			strings.Join(candidate.OriginLanes(), ", "),
		)
		if candidate.DirectBridge() != "" {
			fmt.Fprintf(
				builder,
				"  - strongest direct reasoning bridge: %s "+
					"(ranking evidence only; does not grant identity)\n",
				candidate.DirectBridge(),
			)
		}
		if candidate.Graph().Kind() == codeintel.ConcernGraphPresent {
			fmt.Fprintf(
				builder,
				"  - graph components: code=%.8f reasoning=%.8f "+
					"typed_memory=%.8f; restart origins=%s\n",
				candidate.Graph().CodeLexical(),
				candidate.Graph().Reasoning(),
				candidate.Graph().TypedMemory(),
				strings.Join(candidate.Graph().RestartOrigins(), ", "),
			)
		}
		if lexicalPresent {
			renderConcernLexicalMatches(builder, lexical)
		}
		for _, support := range candidate.Artifacts() {
			fmt.Fprintf(
				builder,
				"  - %s `%s` — %s; relation=%s; seed=%t\n",
				support.Lane,
				support.ArtifactRef,
				support.Title,
				support.Relation,
				support.Seeded,
			)
		}
		governance := candidate.Governance()
		if len(governance.Invariants) > 0 ||
			len(governance.Specs) > 0 ||
			len(governance.ExactBindingDecisionRefs) > 0 ||
			len(governance.AffectedPathContextDecisionRefs) > 0 ||
			len(governance.ModuleDecisionRefs) > 0 {
			fmt.Fprintf(
				builder,
				"  - governance lanes: exact_binding=%d, "+
					"affected_path_context=%d (not authority), "+
					"module_context=%d; %d invariant(s), "+
					"%d spec section(s); symbol match=%s\n",
				len(governance.ExactBindingDecisionRefs),
				len(governance.AffectedPathContextDecisionRefs),
				len(governance.ModuleDecisionRefs),
				len(governance.Invariants),
				len(governance.Specs),
				governance.SymbolGranularity,
			)
		}
		calls := candidate.Calls()
		fmt.Fprintf(
			builder,
			"  - calls: %d bounded incoming, %d bounded outgoing; "+
				"outgoing resolution=%d resolved/%d ambiguous/%d unresolved\n",
			len(calls.Incoming),
			len(calls.Outgoing),
			calls.OutgoingCoverage.Resolved,
			calls.OutgoingCoverage.Ambiguous,
			calls.OutgoingCoverage.Unresolved,
		)
		if len(calls.Incoming) == 0 {
			builder.WriteString(
				"  - no resolved static caller is visible in this basis; " +
					"dynamic/reflection/callback forms may remain unresolved, " +
					"so this is not evidence that changing the symbol is safe\n",
			)
		}
	}
	builder.WriteString(
		"\nChoose among ranked candidates by the current engineering use and " +
			"inspect the exact symbol route; graph proximity is neither " +
			"applicability nor a decision.\n",
	)
}

func renderConcernLexicalMatches(
	builder *strings.Builder,
	lexical codebase.SymbolDiscoveryCandidate,
) {
	for _, match := range lexical.Matches() {
		fields := make([]string, 0, len(match.Fields()))
		for _, field := range match.Fields() {
			fields = append(fields, field.String())
		}
		fmt.Fprintf(
			builder,
			"  - `%s` matched %s\n",
			match.Term(),
			strings.Join(fields, ", "),
		)
	}
}
