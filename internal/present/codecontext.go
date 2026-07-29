package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graph"
)

type CodeContextLane string

const (
	CodeContextLaneIndex      CodeContextLane = "index"
	CodeContextLaneSymbols    CodeContextLane = "symbols"
	CodeContextLaneDecisions  CodeContextLane = "decisions"
	CodeContextLaneInvariants CodeContextLane = "invariants"
	CodeContextLaneNotes      CodeContextLane = "notes"
	CodeContextLaneProblems   CodeContextLane = "problems"
	CodeContextLanePortfolios CodeContextLane = "portfolios"
	CodeContextLaneAll        CodeContextLane = "all"
)

const (
	codeContextFileLevelInvariantLimit      = 8
	codeContextHighFanoutInvariantThreshold = 40
	codeContextInvariantSourceGroupLimit    = 12
	codeContextSpecSectionLimit             = 8
	codeContextSpecClaimLimit               = 3
)

func ValidCodeContextLaneNames() []string {
	return []string{
		string(CodeContextLaneIndex),
		string(CodeContextLaneSymbols),
		string(CodeContextLaneDecisions),
		string(CodeContextLaneInvariants),
		string(CodeContextLaneNotes),
		string(CodeContextLaneProblems),
		string(CodeContextLanePortfolios),
		string(CodeContextLaneAll),
	}
}

func ParseCodeContextLane(raw string) (CodeContextLane, bool) {
	switch CodeContextLane(strings.TrimSpace(strings.ToLower(raw))) {
	case CodeContextLaneIndex:
		return CodeContextLaneIndex, true
	case CodeContextLaneSymbols:
		return CodeContextLaneSymbols, true
	case CodeContextLaneDecisions:
		return CodeContextLaneDecisions, true
	case CodeContextLaneInvariants:
		return CodeContextLaneInvariants, true
	case CodeContextLaneNotes:
		return CodeContextLaneNotes, true
	case CodeContextLaneProblems:
		return CodeContextLaneProblems, true
	case CodeContextLanePortfolios:
		return CodeContextLanePortfolios, true
	case CodeContextLaneAll:
		return CodeContextLaneAll, true
	default:
		return "", false
	}
}

// CodeContextResponse renders the default progressive-disclosure index for an
// agent exploring a file or symbol. Typed lanes and full audit mode are separate
// calls; the default response names what exists and how to fetch it.
func CodeContextResponse(cc contextgraph.CodeContext) string {
	return CodeContextResponseWithOptions(cc, CodeContextRenderOptions{
		Lane:                  CodeContextLaneIndex,
		InvariantLimit:        12,
		ContextInvariantLimit: 8,
		ArtifactLimit:         20,
	})
}

// CodeContextResponseAll renders the old compact all-lane view. It remains as
// lane="all" for compatibility and recovery, but is no longer the default.
func CodeContextResponseAll(cc contextgraph.CodeContext) string {
	return CodeContextResponseWithOptions(cc, CodeContextRenderOptions{
		Lane:                  CodeContextLaneAll,
		InvariantLimit:        12,
		ContextInvariantLimit: 8,
		ArtifactLimit:         20,
	})
}

// CodeContextResponseFull renders the complete context projection for explicit
// full-mode calls. Full mode is the audit/backward-compatible dump, not the
// normal escalation path.
func CodeContextResponseFull(cc contextgraph.CodeContext) string {
	return CodeContextResponseWithOptions(cc, CodeContextRenderOptions{
		Lane:                  CodeContextLaneAll,
		Full:                  true,
		InvariantLimit:        0,
		ContextInvariantLimit: 0,
		ArtifactLimit:         0,
	})
}

type CodeContextSymbolItem struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
}

// CodeContextRenderOptions controls only presentation lane and volume. The
// underlying contextgraph fetch remains unchanged, so every compact lane is
// reversible by requesting another lane or Full=true.
type CodeContextRenderOptions struct {
	Full                  bool
	Lane                  CodeContextLane
	InvariantLimit        int
	ContextInvariantLimit int
	ArtifactLimit         int
	SymbolCount           int
	SymbolCountKnown      bool
	SymbolUnavailable     string
}

// CodeContextResponseWithOptions renders the fused code context using explicit
// lane and volume controls. Passing Full=true disables limits.
func CodeContextResponseWithOptions(cc contextgraph.CodeContext, options CodeContextRenderOptions) string {
	options = normalizeCodeContextOptions(options)
	switch options.Lane {
	case CodeContextLaneIndex:
		return renderCodeContextIndex(cc, options)
	case CodeContextLaneDecisions:
		return renderCodeContextDecisionsLane(cc, options)
	case CodeContextLaneInvariants:
		return renderCodeContextInvariantsLane(cc, options)
	case CodeContextLaneNotes:
		return renderCodeContextArtifactsLane(cc, CodeContextLaneNotes, "Notes", cc.Notes, options)
	case CodeContextLaneProblems:
		return renderCodeContextArtifactsLane(cc, CodeContextLaneProblems, "Problems framed around it", cc.Problems, options)
	case CodeContextLanePortfolios:
		return renderCodeContextArtifactsLane(cc, CodeContextLanePortfolios, "Solution variants explored", cc.Portfolios, options)
	default:
		return renderCodeContextAll(cc, options)
	}
}

func normalizeCodeContextOptions(options CodeContextRenderOptions) CodeContextRenderOptions {
	if options.Lane == "" {
		options.Lane = CodeContextLaneIndex
	}
	if options.Full {
		return options
	}
	if options.InvariantLimit <= 0 {
		options.InvariantLimit = 12
	}
	if options.ContextInvariantLimit <= 0 {
		options.ContextInvariantLimit = 8
	}
	if options.ArtifactLimit <= 0 {
		options.ArtifactLimit = 20
	}
	return options
}

func renderCodeContextAll(cc contextgraph.CodeContext, options CodeContextRenderOptions) string {
	var b strings.Builder

	renderCodeContextHeader(&b, "Code context", cc, true)

	if cc.Empty() {
		b.WriteString("No recorded reasoning touches this code yet — nothing decided, framed, or noted here.\n")
		b.WriteString("About to make a non-trivial change? Consider /h-frame or /h-note so the next reader sees why.\n")
		return b.String()
	}

	renderCodeContextDecisionLanes(&b, cc, options)
	renderSpecSections(&b, cc.Specs, options.Full)
	renderContextArtifacts(&b, "Problems framed around it", cc.Problems, options.ArtifactLimit, options.Full)
	renderContextArtifacts(&b, "Solution variants explored", cc.Portfolios, options.ArtifactLimit, options.Full)
	renderContextArtifacts(&b, "Notes", cc.Notes, options.ArtifactLimit, options.Full)

	renderTargetInvariantSection(&b, cc, options)
	renderInvariantSection(&b, "### Module context — invariants of module-governing decisions (may not bind this symbol)", cc.ContextInvariants, options.ContextInvariantLimit, options.Full)

	return b.String()
}

func renderCodeContextIndex(cc contextgraph.CodeContext, options CodeContextRenderOptions) string {
	var b strings.Builder

	renderCodeContextHeader(&b, "Code context index", cc, false)
	renderCodeContextLaneCounts(&b, cc, options)
	renderCodeContextRiskHints(&b, cc)

	b.WriteString("### Next calls\n")
	for _, lane := range []CodeContextLane{
		CodeContextLaneSymbols,
		CodeContextLaneDecisions,
		CodeContextLaneInvariants,
		CodeContextLaneNotes,
		CodeContextLaneProblems,
		CodeContextLanePortfolios,
		CodeContextLaneAll,
	} {
		fmt.Fprintf(&b, "- %s: `%s`\n", lane, codeContextQuery(cc.Target, lane, false))
	}
	fmt.Fprintf(&b, "- audit dump: `%s`\n", codeContextQuery(cc.Target, "", true))

	return b.String()
}

func renderCodeContextLaneCounts(b *strings.Builder, cc contextgraph.CodeContext, options CodeContextRenderOptions) {
	b.WriteString("### Lane counts\n")
	if options.SymbolCountKnown {
		fmt.Fprintf(b, "- symbols: %d\n", options.SymbolCount)
	} else if options.SymbolUnavailable != "" {
		fmt.Fprintf(b, "- symbols: unavailable (%s)\n", options.SymbolUnavailable)
	} else {
		b.WriteString("- symbols: request lane=\"symbols\" for a capped file symbol list\n")
	}
	fmt.Fprintf(
		b,
		"- decisions: %d exact_binding, %d affected_path_context (not authority), %d module_context\n",
		len(cc.Decisions),
		len(cc.AffectedPathContextDecisions),
		len(cc.ModuleDecisions),
	)
	renderSpecSectionCounts(b, cc.Specs)
	if codeContextFileLevelInvariantCandidates(cc) {
		fmt.Fprintf(b, "- invariants: %d file-level candidate(s), %d module-context; narrow by symbol before treating as actionable\n", len(cc.Invariants), len(cc.ContextInvariants))
	} else {
		fmt.Fprintf(b, "- invariants: %d binding, %d module-context\n", len(cc.Invariants), len(cc.ContextInvariants))
	}
	fmt.Fprintf(b, "- notes: %d\n", len(cc.Notes))
	fmt.Fprintf(b, "- problems: %d\n", len(cc.Problems))
	fmt.Fprintf(b, "- portfolios: %d\n\n", len(cc.Portfolios))
}

func renderCodeContextRiskHints(b *strings.Builder, cc contextgraph.CodeContext) {
	cues := codeContextRiskHints(cc)
	if len(cues) == 0 {
		return
	}
	b.WriteString("### Risk cues\n")
	for _, cue := range cues {
		fmt.Fprintf(b, "- %s\n", cue)
	}
	b.WriteString("\n")
}

func codeContextRiskHints(cc contextgraph.CodeContext) []string {
	risks := make([]string, 0, 4)
	if cc.Module != "" && !cc.Governed && len(cc.ModuleDecisions) == 0 {
		risks = append(risks, "module is blind: no module-governing decision is recorded")
	}
	if issues := specSectionIssueCount(cc.Specs); issues > 0 {
		risks = append(risks, fmt.Sprintf("%d referenced SpecSection(s) are non-current or unresolved; inspect lane=\"decisions\"", issues))
	}

	inactive := 0
	unverified := 0
	for _, decision := range cc.Decisions {
		if decision.Meta.Status != artifact.StatusActive {
			inactive++
		}
		unverified += decisionUnverifiedPredictionCount(decision)
	}
	if inactive > 0 {
		risks = append(risks, fmt.Sprintf("%d governing decision(s) are non-active; inspect lane=\"decisions\"", inactive))
	}
	if unverified > 0 {
		risks = append(risks, fmt.Sprintf("%d prediction(s) remain unverified across governing decisions; inspect lane=\"decisions\"", unverified))
	}
	if codeContextFileLevelInvariantCandidates(cc) {
		risks = append(risks, "file-level invariant candidates exist; narrow code_context by symbol before treating constraints as actionable")
	} else if len(cc.Invariants) > 0 {
		risks = append(risks, "symbol-binding invariants exist; inspect lane=\"invariants\" before changing behavior")
	} else if len(cc.ContextInvariants) > 0 {
		risks = append(risks, "module-context invariants exist; inspect lane=\"invariants\" when changing module policy")
	}

	if len(risks) > 3 {
		return risks[:3]
	}
	return risks
}

func renderCodeContextDecisionsLane(cc contextgraph.CodeContext, options CodeContextRenderOptions) string {
	var b strings.Builder

	renderCodeContextHeader(&b, "Code context decisions", cc, false)
	if len(cc.Decisions)+
		len(cc.AffectedPathContextDecisions)+
		len(cc.ModuleDecisions) == 0 {
		b.WriteString("No exact binding, affected-path context, or module context recorded for this target.\n")
		return b.String()
	}
	renderCodeContextDecisionLanes(&b, cc, options)
	renderSpecSections(&b, cc.Specs, options.Full)
	return b.String()
}

func renderCodeContextInvariantsLane(cc contextgraph.CodeContext, options CodeContextRenderOptions) string {
	var b strings.Builder

	renderCodeContextHeader(&b, "Code context invariants", cc, false)
	if len(cc.Invariants)+len(cc.ContextInvariants) == 0 {
		b.WriteString("No invariants recorded for this target or its module context.\n")
		return b.String()
	}
	renderTargetInvariantSection(&b, cc, options)
	renderInvariantSection(&b, "### Module context — invariants of module-governing decisions (may not bind this symbol)", cc.ContextInvariants, options.ContextInvariantLimit, options.Full)
	return b.String()
}

func renderCodeContextArtifactsLane(bcc contextgraph.CodeContext, lane CodeContextLane, heading string, items []*artifact.Artifact, options CodeContextRenderOptions) string {
	var b strings.Builder

	renderCodeContextHeader(&b, "Code context "+string(lane), bcc, false)
	if len(items) == 0 {
		fmt.Fprintf(&b, "No %s recorded for this target.\n", strings.ToLower(heading))
		return b.String()
	}
	renderContextArtifacts(&b, heading, items, options.ArtifactLimit, options.Full)
	return b.String()
}

func renderCodeContextHeader(b *strings.Builder, title string, cc contextgraph.CodeContext, listModuleDecisions bool) {
	fmt.Fprintf(b, "## %s — %s\n\n", title, codeContextTargetLabel(cc.Target))

	if cc.Module != "" {
		if len(cc.ModuleDecisions) > 0 {
			if listModuleDecisions {
				fmt.Fprintf(b, "Module `%s`: governed by %s\n\n", cc.Module, moduleDecisionList(cc.ModuleDecisions))
			} else {
				fmt.Fprintf(b, "Module `%s`: governed by %d decision(s)\n\n", cc.Module, len(cc.ModuleDecisions))
			}
		} else {
			fmt.Fprintf(b, "Module `%s`: blind — no decisions govern this module\n\n", cc.Module)
		}
	}

	if cc.Target.Symbol != "" && cc.SymbolGranularity != "" {
		fmt.Fprintf(b, "Symbol match granularity: %s\n\n", cc.SymbolGranularity)
	}
}

func codeContextTargetLabel(target contextgraph.Target) string {
	if target.Symbol != "" {
		return fmt.Sprintf("%s :: %s", target.File, target.Symbol)
	}
	return target.File
}

func codeContextQuery(target contextgraph.Target, lane CodeContextLane, full bool) string {
	parts := []string{
		`action="code_context"`,
		fmt.Sprintf("file=%q", target.File),
	}
	if target.Symbol != "" {
		parts = append(parts, fmt.Sprintf("symbol=%q", target.Symbol))
	}
	if target.Line > 0 {
		parts = append(parts, fmt.Sprintf("line=%d", target.Line))
	}
	if lane != "" {
		parts = append(parts, fmt.Sprintf("lane=%q", lane))
	}
	if full {
		parts = append(parts, "full=true")
	}
	return "haft_query(" + strings.Join(parts, ", ") + ")"
}

func CodeContextSymbolsResponse(target contextgraph.Target, symbols []CodeContextSymbolItem, limit int, refreshed bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## Code context symbols — %s\n\n", codeContextTargetLabel(target))
	if refreshed {
		b.WriteString("Symbol index refreshed before rendering this lane.\n\n")
	}
	if len(symbols) == 0 {
		b.WriteString("No symbols indexed for this file.\n")
		return b.String()
	}

	visible := symbols
	if limit > 0 && len(symbols) > limit {
		visible = symbols[:limit]
	}

	b.WriteString("### Symbols\n")
	for _, sym := range visible {
		fmt.Fprintf(&b, "- %s `%s` lines %d-%d\n", sym.Kind, sym.Name, sym.StartLine, sym.EndLine)
	}
	if omitted := len(symbols) - len(visible); omitted > 0 {
		fmt.Fprintf(&b, "- ... %d more omitted; re-run with a higher `limit` for more symbols.\n", omitted)
	}
	return b.String()
}

func CodeContextSymbolsUnavailableResponse(target contextgraph.Target, err error) string {
	return fmt.Sprintf("## Code context symbols — %s\n\nSymbol lane unavailable: %v\n", codeContextTargetLabel(target), err)
}

func renderTargetInvariantSection(b *strings.Builder, cc contextgraph.CodeContext, options CodeContextRenderOptions) {
	if len(cc.Invariants) == 0 {
		return
	}
	if codeContextFileLevelInvariantCandidates(cc) {
		renderFileLevelInvariantNotice(b, cc)
		renderInvariantSection(b, "### File-level invariant candidates", cc.Invariants, fileLevelInvariantLimit(options.InvariantLimit), options.Full)
		return
	}
	renderInvariantSection(b, "### Invariants that must hold here", cc.Invariants, options.InvariantLimit, options.Full)
}

func codeContextFileLevelInvariantCandidates(cc contextgraph.CodeContext) bool {
	return cc.Target.Symbol == "" && len(cc.Invariants) > 0
}

func fileLevelInvariantLimit(limit int) int {
	if limit <= 0 || limit > codeContextFileLevelInvariantLimit {
		return codeContextFileLevelInvariantLimit
	}
	return limit
}

func renderFileLevelInvariantNotice(b *strings.Builder, cc contextgraph.CodeContext) {
	b.WriteString("### Invariant relevance\n")
	fmt.Fprintf(b, "- File-level view found %d invariant candidate(s). Treat them as relevance context, not proof that every invariant binds every symbol in the file.\n", len(cc.Invariants))
	fmt.Fprintf(b, "- For actionable constraints, inspect symbols first with `%s`, then rerun code_context with `symbol`.\n", codeContextQuery(cc.Target, CodeContextLaneSymbols, false))
	fmt.Fprintf(b, "- Full audit remains available with `%s`.\n\n", codeContextQuery(cc.Target, "", true))
}

func renderInvariantSection(b *strings.Builder, heading string, invariants []graph.Invariant, limit int, full bool) {
	if len(invariants) == 0 {
		return
	}

	if !full && len(invariants) > codeContextHighFanoutInvariantThreshold {
		renderInvariantFanoutSummary(b, heading, invariants)
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

type invariantSourceGroup struct {
	DecisionID    string
	DecisionTitle string
	Count         int
}

func renderInvariantFanoutSummary(b *strings.Builder, heading string, invariants []graph.Invariant) {
	groups := invariantSourceGroups(invariants)

	b.WriteString(heading)
	b.WriteString("\n")
	fmt.Fprintf(b, "- High fanout: %d invariant(s) from %d source group(s). Default lane shows source groups, not every invariant sentence.\n", len(invariants), len(groups))
	b.WriteString("- Full audit: `haft_query(action=\"code_context\", ..., full=true)`.\n")
	b.WriteString("\n")
	b.WriteString("#### Source groups\n")

	visible := groups
	if len(groups) > codeContextInvariantSourceGroupLimit {
		visible = groups[:codeContextInvariantSourceGroupLimit]
	}
	for _, group := range visible {
		fmt.Fprintf(b, "- %s: %d invariant(s)\n", invariantSourceGroupLabel(group), group.Count)
	}
	if omitted := len(groups) - len(visible); omitted > 0 {
		fmt.Fprintf(b, "- ... %d more source group(s) omitted; re-run `haft_query(action=\"code_context\", ..., full=true)` for every invariant sentence.\n", omitted)
	}

	b.WriteString("\n")
}

func invariantSourceGroups(invariants []graph.Invariant) []invariantSourceGroup {
	groups := make([]invariantSourceGroup, 0)
	index := make(map[string]int)
	for _, invariant := range invariants {
		key := invariantSourceGroupKey(invariant)
		if position, ok := index[key]; ok {
			groups[position].Count++
			continue
		}
		index[key] = len(groups)
		groups = append(groups, invariantSourceGroup{
			DecisionID:    strings.TrimSpace(invariant.DecisionID),
			DecisionTitle: strings.TrimSpace(invariant.DecisionTitle),
			Count:         1,
		})
	}
	return groups
}

func invariantSourceGroupKey(invariant graph.Invariant) string {
	return strings.TrimSpace(invariant.DecisionID) + "\x00" + strings.TrimSpace(invariant.DecisionTitle)
}

func invariantSourceGroupLabel(group invariantSourceGroup) string {
	title := strings.TrimSpace(group.DecisionTitle)
	id := strings.TrimSpace(group.DecisionID)
	if title != "" && id != "" {
		return fmt.Sprintf("**%s** `%s`", title, id)
	}
	if title != "" {
		return fmt.Sprintf("**%s**", title)
	}
	if id != "" {
		return fmt.Sprintf("`%s`", id)
	}
	return "unknown decision source"
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

func renderCodeContextDecisionLanes(
	b *strings.Builder,
	cc contextgraph.CodeContext,
	options CodeContextRenderOptions,
) {
	renderDecisionArtifacts(
		b,
		"### `exact_binding` — authority-bearing target/anchor bindings",
		cc.Decisions,
		options.ArtifactLimit,
		options.Full,
	)
	renderDecisionArtifacts(
		b,
		"### `affected_path_context` — exact backlinks only, not binding authority",
		cc.AffectedPathContextDecisions,
		options.ArtifactLimit,
		options.Full,
	)
	if len(cc.ModuleDecisions) == 0 {
		return
	}
	b.WriteString("### `module_context` — exact most-specific module\n")
	for _, decision := range cc.ModuleDecisions {
		fmt.Fprintf(
			b,
			"- **%s** `%s`\n",
			decision.Name,
			decision.ID,
		)
	}
	b.WriteString("\n")
}

// renderDecisionArtifacts lists one explicitly classified decision lane, each
// tagged with status and verification cues.
func renderDecisionArtifacts(
	b *strings.Builder,
	heading string,
	items []*artifact.Artifact,
	limit int,
	full bool,
) {
	if len(items) == 0 {
		return
	}
	b.WriteString(heading)
	b.WriteString("\n")
	visible := limitArtifacts(items, limit, full)
	for _, a := range visible {
		suffix := ""
		if a.Meta.Status != artifact.StatusActive {
			suffix = fmt.Sprintf(" [%s]", a.Meta.Status)
		}
		fmt.Fprintf(b, "- **%s** `%s`%s%s%s\n", a.Meta.Title, a.Meta.ID, suffix, decisionSpecSectionRefsTag(a), decisionVerificationTag(a))
	}
	renderArtifactOmission(b, len(items), len(visible))
	b.WriteString("\n")
}

func decisionSpecSectionRefsTag(a *artifact.Artifact) string {
	df := a.UnmarshalDecisionFields()
	if len(df.SectionRefs) == 0 {
		return ""
	}
	refs := make([]string, 0, len(df.SectionRefs))
	for _, ref := range df.SectionRefs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		refs = append(refs, fmt.Sprintf("`%s`", trimmed))
	}
	if len(refs) == 0 {
		return ""
	}
	return fmt.Sprintf(" · SpecSections: %s", strings.Join(refs, ", "))
}

type specSectionCounts struct {
	resolved            int
	unresolved          int
	baselineCurrent     int
	baselineDrifted     int
	baselineMissing     int
	baselineUnavailable int
}

func renderSpecSectionCounts(b *strings.Builder, sections []contextgraph.SpecSectionContext) {
	if len(sections) == 0 {
		b.WriteString("- specs: 0 referenced\n")
		return
	}
	counts := countSpecSections(sections)
	fmt.Fprintf(
		b,
		"- specs: %d referenced; %d resolved, %d unresolved; baselines current=%d drifted=%d missing=%d unavailable=%d\n",
		len(sections),
		counts.resolved,
		counts.unresolved,
		counts.baselineCurrent,
		counts.baselineDrifted,
		counts.baselineMissing,
		counts.baselineUnavailable,
	)
}

func countSpecSections(sections []contextgraph.SpecSectionContext) specSectionCounts {
	counts := specSectionCounts{}
	for _, section := range sections {
		if section.Resolution == contextgraph.SpecResolutionResolved {
			counts.resolved++
		} else {
			counts.unresolved++
		}
		switch section.BaselineState {
		case contextgraph.SpecBaselineCurrent:
			counts.baselineCurrent++
		case contextgraph.SpecBaselineDrifted:
			counts.baselineDrifted++
		case contextgraph.SpecBaselineMissing:
			counts.baselineMissing++
		case contextgraph.SpecBaselineUnavailable:
			counts.baselineUnavailable++
		}
	}
	return counts
}

func specSectionIssueCount(sections []contextgraph.SpecSectionContext) int {
	issues := 0
	for _, section := range sections {
		resolved := section.Resolution == contextgraph.SpecResolutionResolved
		current := section.BaselineState == contextgraph.SpecBaselineCurrent
		active := string(section.LifecycleState) == "active"
		if !resolved || !current || !active {
			issues++
		}
	}
	return issues
}

func renderSpecSections(b *strings.Builder, sections []contextgraph.SpecSectionContext, full bool) {
	if len(sections) == 0 {
		return
	}
	b.WriteString("### Referenced SpecSections\n")
	visible := sections
	if !full && len(visible) > codeContextSpecSectionLimit {
		visible = visible[:codeContextSpecSectionLimit]
	}
	for _, section := range visible {
		renderSpecSection(b, section, full)
	}
	if omitted := len(sections) - len(visible); omitted > 0 {
		fmt.Fprintf(b, "- ... %d more SpecSection(s) omitted; re-run `haft_query(action=\"code_context\", ..., full=true)` for all sections and claims.\n", omitted)
	}
	b.WriteString("\n")
}

func renderSpecSection(b *strings.Builder, section contextgraph.SpecSectionContext, full bool) {
	title := strings.TrimSpace(section.Title)
	if title == "" {
		title = "Unresolved SpecSection"
	}
	parts := []string{
		fmt.Sprintf("resolution=%s", nonEmptyString(string(section.Resolution), "unknown")),
		fmt.Sprintf("baseline=%s", nonEmptyString(string(section.BaselineState), "unknown")),
	}
	if section.LifecycleState != "" {
		parts = append(parts, fmt.Sprintf("lifecycle=%s", section.LifecycleState))
	}
	if section.ValidUntil != "" {
		parts = append(parts, "valid_until="+section.ValidUntil)
	}
	if section.SourceKind != "" {
		parts = append(parts, "source="+section.SourceKind)
	}
	carrier := strings.TrimSpace(section.CarrierPath)
	if carrier == "" {
		carrier = strings.TrimSpace(section.Path)
	}
	if carrier != "" {
		parts = append(parts, "carrier="+carrier)
	}
	if len(section.DecisionRefs) > 0 {
		parts = append(parts, "decisions="+strings.Join(section.DecisionRefs, ","))
	}
	if section.ResolutionDetail != "" {
		parts = append(parts, "resolution_detail="+compactContextDetail(section.ResolutionDetail))
	}
	if section.BaselineDetail != "" {
		parts = append(parts, "baseline_detail="+compactContextDetail(section.BaselineDetail))
	}
	fmt.Fprintf(b, "- **%s** `%s` · %s\n", title, section.ID, strings.Join(parts, " · "))
	renderSpecClaims(b, section.Claims, full)
}

func renderSpecClaims(b *strings.Builder, claims []contextgraph.SpecClaimContext, full bool) {
	if len(claims) == 0 {
		return
	}
	visible := claims
	if !full && len(visible) > codeContextSpecClaimLimit {
		visible = visible[:codeContextSpecClaimLimit]
	}
	for _, claim := range visible {
		label := strings.TrimSpace(claim.ID)
		if label == "" {
			label = "unnamed-claim"
		}
		class := strings.TrimSpace(claim.Class)
		if class == "" {
			class = "unclassified"
		}
		statement := compactContextDetail(claim.Statement)
		fmt.Fprintf(b, "  - claim `%s` [%s]: %s", label, class, statement)
		if claim.ValidUntil != "" {
			fmt.Fprintf(b, " (valid_until=%s)", claim.ValidUntil)
		}
		b.WriteString("\n")
	}
	if omitted := len(claims) - len(visible); omitted > 0 {
		fmt.Fprintf(b, "  - ... %d more claim(s) omitted; re-run with `full=true`.\n", omitted)
	}
}

func compactContextDetail(value string) string {
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

func nonEmptyString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

// decisionVerificationTag reports how many of a decision's predictions remain
// unverified, so a governing decision whose claims were never checked is not read
// as authoritative. Pure over the artifact's stored claims; empty when there are
// no claims or all are verified.
func decisionVerificationTag(a *artifact.Artifact) string {
	unverified := decisionUnverifiedPredictionCount(a)
	if unverified == 0 {
		return ""
	}
	df := a.UnmarshalDecisionFields()
	return fmt.Sprintf(" · %d/%d predictions unverified", unverified, len(df.Claims))
}

func decisionUnverifiedPredictionCount(a *artifact.Artifact) int {
	df := a.UnmarshalDecisionFields()
	if len(df.Claims) == 0 {
		return 0
	}
	unverified := 0
	for _, c := range df.Claims {
		if c.Status == "" || c.Status == artifact.ClaimStatusUnverified {
			unverified++
		}
	}
	return unverified
}

// renderContextArtifacts lists a kind-group, pairing every artifact ID with
// its human-readable title (FPF A.7 re-grounding) and flagging non-active
// status.
func renderContextArtifacts(b *strings.Builder, heading string, items []*artifact.Artifact, limit int, full bool) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s\n", heading)
	visible := limitArtifacts(items, limit, full)
	for _, a := range visible {
		suffix := ""
		if a.Meta.Status != artifact.StatusActive {
			suffix = fmt.Sprintf(" [%s]", a.Meta.Status)
		}
		fmt.Fprintf(b, "- **%s** `%s`%s\n", a.Meta.Title, a.Meta.ID, suffix)
	}
	renderArtifactOmission(b, len(items), len(visible))
	b.WriteString("\n")
}

func limitArtifacts(items []*artifact.Artifact, limit int, full bool) []*artifact.Artifact {
	if full || limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func renderArtifactOmission(b *strings.Builder, total int, visible int) {
	if omitted := total - visible; omitted > 0 {
		fmt.Fprintf(b, "- ... %d more omitted; re-run with a higher `limit` or `full=true` for audit detail.\n", omitted)
	}
}
