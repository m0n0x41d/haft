package initplanning

import (
	"fmt"
	"slices"
	"sort"
)

type ObservationRequirement string

const (
	ObservationRequired  ObservationRequirement = "required"
	ObservationIfPresent ObservationRequirement = "if_present"
)

type ObservationTarget struct {
	path        string
	components  ComponentSet
	requirement ObservationRequirement
}

func (target ObservationTarget) Path() string {
	return target.path
}

func (target ObservationTarget) Component() Component {
	component, _ := target.components.single()
	return component
}

func (target ObservationTarget) Components() ComponentSet {
	return ComponentSet{values: target.components.Values()}
}

func (target ObservationTarget) Requirement() ObservationRequirement {
	return target.requirement
}

type InstallationObservationPlan struct {
	managedRoots []string
	targets      []ObservationTarget
}

func (plan InstallationObservationPlan) ManagedRoots() []string {
	return slices.Clone(plan.managedRoots)
}

func (plan InstallationObservationPlan) Targets() []ObservationTarget {
	return slices.Clone(plan.targets)
}

func BuildInstalledObservationPlan(
	manifest InstallationManifest,
	projection HostAdapterProjection,
	legacySelection LegacyRegistrySelection,
) (InstallationObservationPlan, error) {
	manifestBasis := manifest.OwnershipBasis()
	if !manifestBasis.valid() {
		return InstallationObservationPlan{}, fmt.Errorf("installation manifest is invalid")
	}
	if err := validateProjectionBinding(manifest, projection); err != nil {
		return InstallationObservationPlan{}, err
	}
	managedRoots, err := mergeManagedRoots(
		manifest.wire.TargetRoots,
		projection.targetRoots,
	)
	if err != nil {
		return InstallationObservationPlan{}, err
	}
	legacyByPath, _, err := prepareLegacyRegistry(
		manifest,
		legacySelection,
		managedRoots,
	)
	if err != nil {
		return InstallationObservationPlan{}, err
	}
	currentByPath, err := renderedOutputsByPath(projection.outputs)
	if err != nil {
		return InstallationObservationPlan{}, err
	}
	return buildInstallationObservationPlan(
		managedRoots,
		manifestPathsByPath(manifest.RenderedPaths()),
		currentByPath,
		legacyByPath,
	)
}

func BuildFirstInstallationObservationPlan(
	projection HostAdapterProjection,
	legacySelection LegacyRegistrySelection,
) (InstallationObservationPlan, error) {
	managedRoots, currentByPath, err := validateFirstInstallationProjection(projection)
	if err != nil {
		return InstallationObservationPlan{}, err
	}
	legacyByPath, _, err := prepareFirstInstallationLegacyRegistry(
		projection,
		legacySelection,
		managedRoots,
	)
	if err != nil {
		return InstallationObservationPlan{}, err
	}
	return buildInstallationObservationPlan(
		managedRoots,
		map[string]ManifestPath{},
		currentByPath,
		legacyByPath,
	)
}

func buildInstallationObservationPlan(
	managedRoots []string,
	manifestByPath map[string]ManifestPath,
	currentByPath map[string]RenderedOutput,
	legacyByPath map[string]KnownLegacyPath,
) (InstallationObservationPlan, error) {
	targets := make(map[string]ObservationTarget, len(manifestByPath)+len(currentByPath)+len(legacyByPath))
	for path, manifest := range manifestByPath {
		targets[path] = ObservationTarget{
			path:        path,
			components:  ComponentSet{values: []Component{manifest.Component}},
			requirement: ObservationRequired,
		}
	}
	for path, output := range currentByPath {
		if _, exists := targets[path]; exists {
			continue
		}
		targets[path] = ObservationTarget{
			path:        path,
			components:  output.Components(),
			requirement: ObservationRequired,
		}
	}
	for path, legacy := range legacyByPath {
		if _, exists := targets[path]; exists {
			continue
		}
		targets[path] = ObservationTarget{
			path:        path,
			components:  ComponentSet{values: []Component{legacy.Component}},
			requirement: ObservationIfPresent,
		}
	}
	orderedPaths := make([]string, 0, len(targets))
	for path := range targets {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Strings(orderedPaths)
	orderedTargets := make([]ObservationTarget, len(orderedPaths))
	for index, path := range orderedPaths {
		target := targets[path]
		if err := validateObservationTarget(target, managedRoots); err != nil {
			return InstallationObservationPlan{}, err
		}
		orderedTargets[index] = target
	}
	return InstallationObservationPlan{
		managedRoots: slices.Clone(managedRoots),
		targets:      orderedTargets,
	}, nil
}

func validateObservationTarget(
	target ObservationTarget,
	managedRoots []string,
) error {
	canonical, err := parseCanonicalAbsolutePath(target.path)
	if err != nil || canonical != target.path || !pathWithinAnyRoot(target.path, managedRoots) {
		return fmt.Errorf("observation target path is invalid")
	}
	if err := validateComponentSet(target.components); err != nil {
		return fmt.Errorf("observation target components are invalid: %w", err)
	}
	if target.requirement != ObservationRequired && target.requirement != ObservationIfPresent {
		return fmt.Errorf("observation target requirement is invalid")
	}
	return nil
}
