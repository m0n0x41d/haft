package contextgraph

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
	"github.com/m0n0x41d/haft/internal/projectpath"
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
	canonical, err := projectpath.Parse(target.File)
	if err != nil {
		return CodeContext{}, fmt.Errorf("code-context target: %w", err)
	}
	target.File = canonical.String()

	rawFileLinked, err := store.SearchByAffectedFile(ctx, target.File)
	if err != nil {
		return CodeContext{}, fmt.Errorf("fetch file-linked artifacts: %w", err)
	}
	fileLinked := filterCurrentDecisions(rawFileLinked)
	affectedPathDecisions := decisionArtifacts(fileLinked)
	linked := fileLinked
	granularity := ""
	var symbolLinked symbolLinkResult

	if target.Symbol != "" {
		sl, err := fetchSymbolLinked(ctx, store, target)
		if err != nil {
			return CodeContext{}, err
		}
		symbolLinked = sl
		symbolLinked.linked = filterCurrentDecisions(symbolLinked.linked)
		symbolLinked.exact = decisionArtifacts(
			filterCurrentDecisions(symbolLinked.exact),
		)
		granularity = symbolLinked.granularity
		linked = dedupeArtifacts(
			append(fileLinked, symbolLinked.linked...),
		)
		affectedPathDecisions = decisionArtifacts(
			dedupeArtifacts(
				append(fileLinked, symbolLinked.linked...),
			),
		)
	}

	explicitBindings, err := currentExactBindingDecisions(
		ctx,
		store,
		target,
	)
	if err != nil {
		return CodeContext{}, err
	}
	exactBindings := dedupeArtifacts(
		append(symbolLinked.exact, explicitBindings...),
	)
	if len(explicitBindings) > 0 && granularity == "" {
		granularity = "binding-target-precise"
	}
	affectedPathDecisions = artifactsWithoutIDs(
		affectedPathDecisions,
		artifactIDs(exactBindings),
	)

	exactBindingNodes := decisionNodes(exactBindings)
	invariants, err := g.FindInvariantsForDecisions(ctx, exactBindingNodes)
	if err != nil {
		return CodeContext{}, fmt.Errorf("fetch invariants: %w", err)
	}

	module, moduleDecisions := moduleStatus(ctx, g, target.File)
	contextInv, err := g.FindInvariantsForDecisions(ctx, moduleDecisions)
	if err != nil {
		return CodeContext{}, fmt.Errorf("fetch module invariants: %w", err)
	}

	cc := BuildCodeContext(target, linked, invariants, module, false)
	cc.Decisions = exactBindings
	cc.SymbolGranularity = granularity
	cc.ExactBindingDecisions = exactBindings
	cc.AffectedPathContextDecisions = affectedPathDecisions
	cc.ModuleDecisions = moduleDecisions
	cc.ContextInvariants = contextInv
	cc.Governed = len(exactBindings) > 0 || len(moduleDecisions) > 0
	moduleArtifacts := loadDecisionArtifacts(ctx, store, moduleDecisions)
	specDecisions := dedupeArtifacts(
		append(exactBindings, moduleArtifacts...),
	)
	cc.Specs = fetchGoverningSpecSections(
		ctx,
		store.DB(),
		specDecisions,
		time.Now().UTC(),
	)
	return cc, nil
}

func filterCurrentDecisions(
	items []*artifact.Artifact,
) []*artifact.Artifact {
	result := make([]*artifact.Artifact, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.Meta.Kind != artifact.KindDecisionRecord {
			result = append(result, item)
			continue
		}
		if item.Meta.Status != artifact.StatusActive &&
			item.Meta.Status != artifact.StatusRefreshDue {
			continue
		}
		result = append(result, item)
	}
	return result
}

func decisionArtifacts(
	items []*artifact.Artifact,
) []*artifact.Artifact {
	result := make([]*artifact.Artifact, 0, len(items))
	for _, item := range items {
		if item == nil || item.Meta.Kind != artifact.KindDecisionRecord {
			continue
		}
		result = append(result, item)
	}
	return result
}

func decisionNodes(items []*artifact.Artifact) []graph.Node {
	result := make([]graph.Node, 0, len(items))
	for _, item := range decisionArtifacts(items) {
		result = append(result, graph.Node{
			ID:   item.Meta.ID,
			Kind: graph.KindDecision,
			Name: item.Meta.Title,
		})
	}
	return result
}

func currentExactBindingDecisions(
	ctx context.Context,
	store *artifact.Store,
	target Target,
) ([]*artifact.Artifact, error) {
	heads, err := store.ListByKind(
		ctx,
		artifact.KindDecisionRecord,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("list binding decisions: %w", err)
	}
	result := make([]*artifact.Artifact, 0)
	for _, head := range heads {
		if head == nil ||
			head.Meta.Status != artifact.StatusActive &&
				head.Meta.Status != artifact.StatusRefreshDue {
			continue
		}
		item, err := store.Get(ctx, head.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf(
				"load binding decision %s: %w",
				head.Meta.ID,
				err,
			)
		}
		if item == nil {
			continue
		}
		fields := item.UnmarshalDecisionFields()
		if decisionHasExactBinding(fields.EffectiveDriftBindingTargets(), target) {
			result = append(result, item)
		}
	}
	return result, nil
}

func decisionHasExactBinding(
	targets []artifact.BindingTarget,
	target Target,
) bool {
	for _, binding := range targets {
		bindingPath, err := projectpath.Parse(binding.FilePath)
		if err != nil || bindingPath.String() != target.File {
			continue
		}
		switch binding.Kind {
		case artifact.BindingTargetWholeFileFallback:
			return true
		case artifact.BindingTargetRange:
			if target.Line > 0 &&
				target.Line >= binding.Line &&
				target.Line <= binding.EndLine {
				return true
			}
		case artifact.BindingTargetSymbol:
			if target.Symbol == "" {
				continue
			}
			if target.AnchorID != "" &&
				binding.AnchorID == target.AnchorID {
				return true
			}
			if binding.SymbolName != target.Symbol {
				continue
			}
			if target.Line == 0 ||
				binding.Line == 0 ||
				binding.EndLine == 0 ||
				target.Line >= binding.Line &&
					target.Line <= binding.EndLine {
				return true
			}
		}
	}
	return false
}

func artifactIDs(items []*artifact.Artifact) map[string]bool {
	result := make(map[string]bool, len(items))
	for _, item := range items {
		if item != nil {
			result[item.Meta.ID] = true
		}
	}
	return result
}

func artifactsWithoutIDs(
	items []*artifact.Artifact,
	excluded map[string]bool,
) []*artifact.Artifact {
	result := make([]*artifact.Artifact, 0, len(items))
	for _, item := range items {
		if item == nil || excluded[item.Meta.ID] {
			continue
		}
		result = append(result, item)
	}
	return result
}

func loadDecisionArtifacts(
	ctx context.Context,
	store *artifact.Store,
	decisions []graph.Node,
) []*artifact.Artifact {
	result := make([]*artifact.Artifact, 0, len(decisions))
	for _, decision := range decisions {
		item, err := store.Get(ctx, decision.ID)
		if err != nil || item == nil {
			continue
		}
		result = append(result, item)
	}
	return result
}

// fetchSymbolLinked resolves the symbol-scoped artifacts for a target, preferring
// the LINE-AWARE join (which disambiguates overloads) and falling back to the
// line-blind join only when no line is given or no line-range row matches — in
// which case it reports "file+name" granularity so the caller never presents
// false per-symbol precision.
type symbolLinkResult struct {
	linked      []*artifact.Artifact
	exact       []*artifact.Artifact
	granularity string
}

func fetchSymbolLinked(
	ctx context.Context,
	store *artifact.Store,
	target Target,
) (symbolLinkResult, error) {
	if target.AnchorID != "" {
		precise, err := store.SearchBySymbolAnchor(ctx, target.AnchorID)
		if err != nil {
			return symbolLinkResult{}, fmt.Errorf(
				"fetch anchor-linked artifacts: %w",
				err,
			)
		}
		if len(precise) > 0 {
			return symbolLinkResult{
				linked:      precise,
				exact:       precise,
				granularity: "anchor-precise",
			}, nil
		}
		// No authority-bearing binding exists for this anchor. Continue through
		// the explicitly labeled compatibility projection for legacy artifacts.
	}
	if target.Line > 0 {
		precise, err := store.SearchByAffectedSymbolAt(ctx, target.Symbol, target.File, target.Line)
		if err != nil {
			return symbolLinkResult{}, fmt.Errorf(
				"fetch line-aware symbol artifacts: %w",
				err,
			)
		}
		if len(precise) > 0 {
			return symbolLinkResult{
				linked:      precise,
				granularity: "legacy-line-context (not an exact binding)",
			}, nil
		}
		// No line-range row covered this line (symbol not symbol-baselined, or
		// legacy rows without an end line): fall back, labeled.
	}
	blind, err := store.SearchByAffectedSymbol(ctx, target.Symbol, target.File)
	if err != nil {
		return symbolLinkResult{}, fmt.Errorf(
			"fetch symbol-linked artifacts: %w",
			err,
		)
	}
	if len(blind) == 0 {
		return symbolLinkResult{}, nil
	}
	return symbolLinkResult{
		linked:      blind,
		granularity: "legacy file+name context (not an exact binding)",
	}, nil
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
