package graph

import (
	"context"
)

// ComputeImpactSet returns all decisions that might be affected when a module changes.
// This includes decisions directly governing the module AND decisions governing
// modules that transitively depend on the changed module.
func (s *Store) ComputeImpactSet(ctx context.Context, moduleID string) ([]ImpactItem, error) {
	// Direct: decisions on the changed module itself
	directDecisions, err := s.FindDecisionsForModule(ctx, moduleID)
	if err != nil {
		return nil, err
	}

	var result []ImpactItem
	seen := make(map[string]bool)

	for _, dec := range directDecisions {
		key := dec.ID + ":" + moduleID
		if !seen[key] {
			seen[key] = true
			result = append(result, ImpactItem{
				ModuleID:      moduleID,
				ModulePath:    "", // caller can fill from module lookup
				DecisionID:    dec.ID,
				DecisionTitle: dec.Name,
				Distance:      0,
				IsDirect:      true,
			})
		}
	}

	// Transitive: modules that depend on the changed module
	dependents, err := s.TransitiveDependents(ctx, moduleID)
	if err != nil {
		return result, nil // best-effort: return direct results if transitive fails
	}

	for _, dep := range dependents {
		depDecisions, err := s.FindDecisionsForModule(ctx, dep.ID)
		if err != nil {
			continue
		}
		for _, dec := range depDecisions {
			key := dec.ID + ":" + dep.ID
			if !seen[key] {
				seen[key] = true
				result = append(result, ImpactItem{
					ModuleID:      dep.ID,
					ModulePath:    dep.Path,
					DecisionID:    dec.ID,
					DecisionTitle: dec.Name,
					Distance:      1, // simplified: not computing exact hop count
					IsDirect:      false,
				})
			}
		}
	}

	return result, nil
}

// ComputeImpactForFile finds the module for a file, then computes the full impact set.
func (s *Store) ComputeImpactForFile(ctx context.Context, filePath string) ([]ImpactItem, error) {
	module, err := s.FindModuleForFile(ctx, filePath)
	if err != nil || module == nil {
		// File not in any module — fall back to direct decision lookup
		decisions, err := s.FindDecisionsForFile(ctx, filePath)
		if err != nil {
			return nil, err
		}
		result := make([]ImpactItem, 0, len(decisions))
		for _, dec := range decisions {
			result = append(result, ImpactItem{
				DecisionID:    dec.ID,
				DecisionTitle: dec.Name,
				Distance:      0,
				IsDirect:      true,
			})
		}
		return result, nil
	}

	items, err := s.ComputeImpactSet(ctx, module.ID)
	if err != nil {
		return nil, err
	}

	// Fill module path for the direct items
	for i := range items {
		if items[i].ModuleID == module.ID && items[i].ModulePath == "" {
			items[i].ModulePath = module.Path
		}
	}

	return items, nil
}
