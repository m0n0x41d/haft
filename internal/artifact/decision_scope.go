package artifact

import "strings"

func (df DecisionFields) EffectiveDriftBindingTargets() []BindingTarget {
	if targets := bindingTargetsFromDriftWatchTargets(df.DriftWatchTargets); len(targets) > 0 {
		return targets
	}
	if targets := bindingTargetsFromGovernanceTargets(df.GovernanceTargets); len(targets) > 0 {
		return targets
	}
	return normalizeBindingTargets(df.BindingTargets)
}

func (df DecisionFields) HasExplicitDriftAuthorityTargets() bool {
	return len(df.EffectiveDriftBindingTargets()) > 0
}

func (df DecisionFields) IsImplementationFootprintOnly() bool {
	return len(compactStrings(df.ImplementationFootprint.Files)) > 0 &&
		len(df.GovernanceTargets) == 0 &&
		len(df.DriftWatchTargets) == 0 &&
		len(df.BindingTargets) == 0
}

func bindingTargetsFromDriftWatchTargets(targets []DriftWatchTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	for _, target := range targets {
		if target.BindingTarget == nil {
			continue
		}
		out = append(out, *target.BindingTarget)
	}
	return normalizeBindingTargets(out)
}

func bindingTargetsFromGovernanceTargets(targets []GovernanceTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	for _, target := range targets {
		if target.BindingTarget == nil {
			continue
		}
		out = append(out, *target.BindingTarget)
	}
	return normalizeBindingTargets(out)
}

func normalizeImplementationFootprint(footprint ImplementationFootprint) ImplementationFootprint {
	return ImplementationFootprint{
		Files:    compactStrings(footprint.Files),
		Commits:  compactStrings(footprint.Commits),
		WorkRefs: compactStrings(footprint.WorkRefs),
	}
}

func normalizeGovernanceTargets(targets []GovernanceTarget) []GovernanceTarget {
	out := make([]GovernanceTarget, 0, len(targets))
	for _, target := range targets {
		kind := strings.TrimSpace(target.Kind)
		ref := strings.TrimSpace(target.Ref)
		if kind == "" && ref == "" && target.BindingTarget == nil {
			continue
		}
		normalized := GovernanceTarget{
			Kind: strings.TrimSpace(target.Kind),
			Ref:  strings.TrimSpace(target.Ref),
		}
		if target.BindingTarget != nil {
			binding := normalizeBindingTargets([]BindingTarget{*target.BindingTarget})
			if len(binding) > 0 {
				normalized.BindingTarget = &binding[0]
			}
		}
		out = append(out, normalized)
	}
	return out
}

func normalizeDriftWatchTargets(targets []DriftWatchTarget) []DriftWatchTarget {
	out := make([]DriftWatchTarget, 0, len(targets))
	for _, target := range targets {
		targetRef := strings.TrimSpace(target.TargetRef)
		trigger := strings.TrimSpace(target.Trigger)
		if targetRef == "" && trigger == "" && target.BindingTarget == nil {
			continue
		}
		normalized := DriftWatchTarget{
			TargetRef: targetRef,
			Trigger:   trigger,
		}
		if target.BindingTarget != nil {
			binding := normalizeBindingTargets([]BindingTarget{*target.BindingTarget})
			if len(binding) > 0 {
				normalized.BindingTarget = &binding[0]
			}
		}
		out = append(out, normalized)
	}
	return out
}
