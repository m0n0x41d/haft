package artifact

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

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
	files := compactStrings(footprint.Files)
	for index, rawPath := range files {
		files[index] = normalizeDecisionProjectPath(rawPath)
	}
	return ImplementationFootprint{
		Files:    files,
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

func normalizeDecisionProjectPath(rawPath string) string {
	canonical, err := projectpath.Parse(rawPath)
	if err != nil {
		return strings.TrimSpace(rawPath)
	}
	return canonical.String()
}

func normalizeDecisionModulePath(rawPath string) string {
	canonical, err := projectpath.ParseModule(rawPath)
	if err != nil {
		return strings.TrimSpace(rawPath)
	}
	return canonical.String()
}

func validateDecisionProjectPaths(input DecideInput) error {
	for index, rawPath := range input.AffectedFiles {
		if err := validateDecisionProjectPath(
			fmt.Sprintf("affected_files[%d]", index),
			rawPath,
		); err != nil {
			return err
		}
	}
	for index, rawPath := range input.ImplementationFootprint.Files {
		if err := validateDecisionProjectPath(
			fmt.Sprintf("implementation_footprint.files[%d]", index),
			rawPath,
		); err != nil {
			return err
		}
	}
	for index, target := range input.BindingTargets {
		if err := validateDecisionBindingTargetPaths(
			fmt.Sprintf("binding_targets[%d]", index),
			target,
		); err != nil {
			return err
		}
	}
	for index, target := range input.GovernanceTargets {
		if target.BindingTarget == nil {
			continue
		}
		if err := validateDecisionBindingTargetPaths(
			fmt.Sprintf(
				"governance_targets[%d].binding_target",
				index,
			),
			*target.BindingTarget,
		); err != nil {
			return err
		}
	}
	for index, target := range input.DriftWatchTargets {
		if target.BindingTarget == nil {
			continue
		}
		if err := validateDecisionBindingTargetPaths(
			fmt.Sprintf(
				"drift_watch_targets[%d].binding_target",
				index,
			),
			*target.BindingTarget,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateDecisionBindingTargetPaths(
	field string,
	target BindingTarget,
) error {
	kind := strings.TrimSpace(target.Kind)
	filePath := strings.TrimSpace(target.FilePath)
	modulePath := strings.TrimSpace(target.ModulePath)
	if filePath != "" {
		if err := validateDecisionProjectPath(
			field+".file_path",
			filePath,
		); err != nil {
			return err
		}
	}
	if kind == BindingTargetModule && modulePath == "" {
		return fmt.Errorf("%s.module_path is required", field)
	}
	if modulePath == "" {
		return nil
	}
	if _, err := projectpath.ParseModule(modulePath); err != nil {
		return fmt.Errorf("%s.module_path: %w", field, err)
	}
	return nil
}

func validateDecisionProjectPath(field string, rawPath string) error {
	if _, err := projectpath.Parse(rawPath); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	return nil
}
