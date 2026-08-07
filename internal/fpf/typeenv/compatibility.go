package typeenv

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// CompareBaseTypeEnvArtifacts describes declaration-level change against the
// previous content-addressed environment. The report is deliberately outside
// the current artifact identity: comparison history cannot change TypeEnvRef.
func CompareBaseTypeEnvArtifacts(
	previous BaseTypeEnvArtifact,
	current BaseTypeEnvArtifact,
) (CompatibilityAssessment, error) {
	if err := current.Verify(); err != nil {
		return nil, fmt.Errorf("current base TypeEnv artifact: %w", err)
	}
	previousRef, hasPreviousRef := previous.TypeEnvRef()
	if !hasPreviousRef {
		return NewInitialCompatibilityAssessment(), nil
	}
	if err := previous.Verify(); err != nil {
		return nil, fmt.Errorf("previous base TypeEnv artifact: %w", err)
	}

	changes, err := compareDeclarationManifests(
		previous.DeclarationProjections(),
		current.DeclarationProjections(),
	)
	if err != nil {
		return nil, err
	}
	diff, err := typedmemory.NewTypeEnvCompatibilityDiff(previousRef, changes)
	if err != nil {
		return nil, fmt.Errorf("build base TypeEnv compatibility diff: %w", err)
	}
	return NewComparedCompatibilityAssessment(diff)
}

func compareDeclarationManifests(
	previous []DeclarationProjection,
	current []DeclarationProjection,
) ([]typedmemory.CompatibilityChange, error) {
	previousBySymbol := declarationProjectionMap(previous)
	currentBySymbol := declarationProjectionMap(current)
	symbols := make([]string, 0, len(previousBySymbol)+len(currentBySymbol))
	seen := make(map[string]struct{}, len(previousBySymbol)+len(currentBySymbol))
	for symbol := range previousBySymbol {
		symbols = append(symbols, symbol)
		seen[symbol] = struct{}{}
	}
	for symbol := range currentBySymbol {
		if _, exists := seen[symbol]; exists {
			continue
		}
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)

	changes := make([]typedmemory.CompatibilityChange, 0)
	for _, symbol := range symbols {
		before, existedBefore := previousBySymbol[symbol]
		after, existsNow := currentBySymbol[symbol]
		kind, rationale, changed := classifyDeclarationChange(
			before,
			existedBefore,
			after,
			existsNow,
		)
		if !changed {
			continue
		}
		symbolRef := after.Symbol()
		if !existsNow {
			symbolRef = before.Symbol()
		}
		change, err := typedmemory.NewCompatibilityChange(symbolRef, kind, rationale)
		if err != nil {
			return nil, fmt.Errorf("describe compatibility change for %q: %w", symbol, err)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func declarationProjectionMap(
	projections []DeclarationProjection,
) map[string]DeclarationProjection {
	indexed := make(map[string]DeclarationProjection, len(projections))
	for _, projection := range projections {
		indexed[projection.Symbol().String()] = projection
	}
	return indexed
}

func classifyDeclarationChange(
	previous DeclarationProjection,
	existedBefore bool,
	current DeclarationProjection,
	existsNow bool,
) (typedmemory.CompatibilityChangeKind, string, bool) {
	if !existedBefore && existsNow {
		return typedmemory.CompatibilityAdded, "source_declaration_added", true
	}
	if existedBefore && !existsNow {
		return typedmemory.CompatibilityRemoved, "source_declaration_removed", true
	}
	if previous.Digest().String() != current.Digest().String() {
		return typedmemory.CompatibilityChanged, "source_declaration_changed", true
	}
	return 0, "", false
}
