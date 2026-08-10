package initplanning

import (
	"fmt"
	"slices"
	"sort"
)

// CoherentInstallationCurrentness keeps whole-file ownership and exact
// shared-carrier fragment ownership distinct while binding both observations
// to one host projection and one optional predecessor manifest.
type CoherentInstallationCurrentness struct {
	whole    InstallationCurrentness
	carriers []ManagedCarrierCurrentness
}

func ClassifyCoherentInstallationCurrentness(
	manifest InstallationManifest,
	projection HostAdapterProjection,
	pathObservations []PathObservation,
	carrierInputs []ManagedCarrierInput,
	legacySelection LegacyRegistrySelection,
	managedLegacy ManagedFragmentLegacyRegistry,
) (CoherentInstallationCurrentness, error) {
	whole, err := classifyCoherentWholeCurrentness(
		InstallationBaselineManifest,
		manifest,
		projection,
		pathObservations,
		legacySelection,
	)
	if err != nil {
		return CoherentInstallationCurrentness{}, err
	}
	carriers, err := classifyCoherentManagedCarriers(
		manifest,
		projection,
		carrierInputs,
		managedLegacy,
	)
	if err != nil {
		return CoherentInstallationCurrentness{}, err
	}
	effectiveProjection, err := projectionWithClassifiedManagedFragments(
		projection,
		carriers,
	)
	if err != nil {
		return CoherentInstallationCurrentness{}, err
	}
	whole.projection = effectiveProjection
	return CoherentInstallationCurrentness{
		whole:    whole,
		carriers: cloneManagedCarrierCurrentnessValues(carriers),
	}, nil
}

func ClassifyFirstCoherentInstallationCurrentness(
	projection HostAdapterProjection,
	pathObservations []PathObservation,
	carrierInputs []ManagedCarrierInput,
	legacySelection LegacyRegistrySelection,
	managedLegacy ManagedFragmentLegacyRegistry,
) (CoherentInstallationCurrentness, error) {
	whole, err := classifyCoherentWholeCurrentness(
		InstallationBaselineNoPriorManifest,
		InstallationManifest{},
		projection,
		pathObservations,
		legacySelection,
	)
	if err != nil {
		return CoherentInstallationCurrentness{}, err
	}
	carriers, err := classifyFirstCoherentManagedCarriers(
		projection,
		carrierInputs,
		managedLegacy,
	)
	if err != nil {
		return CoherentInstallationCurrentness{}, err
	}
	effectiveProjection, err := projectionWithClassifiedManagedFragments(
		projection,
		carriers,
	)
	if err != nil {
		return CoherentInstallationCurrentness{}, err
	}
	whole.projection = effectiveProjection
	return CoherentInstallationCurrentness{
		whole:    whole,
		carriers: cloneManagedCarrierCurrentnessValues(carriers),
	}, nil
}

func projectionWithClassifiedManagedFragments(
	projection HostAdapterProjection,
	carriers []ManagedCarrierCurrentness,
) (HostAdapterProjection, error) {
	fragments := make([]ManagedFragment, 0, len(projection.fragments))
	for _, carrier := range carriers {
		fragments = append(
			fragments,
			cloneManagedFragments(carrier.plan.desired)...,
		)
	}
	validated, err := validateProjectionManagedFragments(
		fragments,
		projection.components,
		projection.targetRoots,
		projection.outputs,
	)
	if err != nil {
		return HostAdapterProjection{}, fmt.Errorf(
			"materialize retained managed fragments: %w",
			err,
		)
	}
	next := cloneHostAdapterProjection(projection)
	next.fragments = validated
	next.retainedManagedFragmentComponents = nil
	return next, nil
}

func (currentness CoherentInstallationCurrentness) WholePaths() InstallationCurrentness {
	return cloneInstallationCurrentness(currentness.whole)
}

func (currentness CoherentInstallationCurrentness) ManagedCarriers() []ManagedCarrierCurrentness {
	return cloneManagedCarrierCurrentnessValues(currentness.carriers)
}

func CompileCoherentHostAdapterReconciliation(
	currentness CoherentInstallationCurrentness,
) (HostAdapterInstallPlan, error) {
	builder, err := hostAdapterReconciliationBuilder(currentness.whole)
	if err != nil {
		return HostAdapterInstallPlan{}, err
	}
	builder = builder.WithManagedFragments(
		currentness.whole.projection.fragments,
	)
	for _, carrierCurrentness := range currentness.carriers {
		reconciliation, err := CompileManagedCarrierReconciliation(
			carrierCurrentness,
		)
		if err != nil {
			return HostAdapterInstallPlan{}, err
		}
		carrierPlan, err := compileManagedCarrierInstallPlan(
			reconciliation,
		)
		if err != nil {
			return HostAdapterInstallPlan{}, err
		}
		builder = builder.AddManagedCarrierPlan(carrierPlan)
	}
	return builder.Build()
}

func classifyCoherentWholeCurrentness(
	baseline InstallationBaselineKind,
	manifest InstallationManifest,
	projection HostAdapterProjection,
	observations []PathObservation,
	legacySelection LegacyRegistrySelection,
) (InstallationCurrentness, error) {
	var manifestByPath map[string]ManifestPath
	var legacyByPath map[string]KnownLegacyPath
	var manifestBasis OwnershipBasis
	var legacyBasis OwnershipBasis
	var managedRoots []string
	var currentByPath map[string]RenderedOutput
	var err error

	if baseline == InstallationBaselineManifest {
		manifestBasis = manifest.OwnershipBasis()
		if !manifestBasis.valid() {
			return InstallationCurrentness{}, fmt.Errorf(
				"installation manifest is invalid",
			)
		}
		if err := validateProjectionBinding(manifest, projection); err != nil {
			return InstallationCurrentness{}, err
		}
		managedRoots, err = mergeManagedRoots(
			manifest.wire.TargetRoots,
			projection.targetRoots,
		)
		if err != nil {
			return InstallationCurrentness{}, err
		}
		legacyByPath, legacyBasis, err = prepareLegacyRegistry(
			manifest,
			legacySelection,
			managedRoots,
		)
		if err != nil {
			return InstallationCurrentness{}, err
		}
		manifestByPath = manifestPathsByPath(manifest.RenderedPaths())
		currentByPath, err = renderedOutputsByPath(projection.outputs)
		if err != nil {
			return InstallationCurrentness{}, err
		}
	}
	if baseline == InstallationBaselineNoPriorManifest {
		managedRoots, currentByPath, err =
			validateFirstInstallationProjection(projection)
		if err != nil {
			return InstallationCurrentness{}, err
		}
		legacyByPath, legacyBasis, err =
			prepareFirstInstallationLegacyRegistry(
				projection,
				legacySelection,
				managedRoots,
			)
		if err != nil {
			return InstallationCurrentness{}, err
		}
		manifestByPath = map[string]ManifestPath{}
	}
	if baseline != InstallationBaselineManifest &&
		baseline != InstallationBaselineNoPriorManifest {
		return InstallationCurrentness{}, fmt.Errorf(
			"coherent installation baseline is invalid",
		)
	}

	observedByPath, err := observationsByPath(observations)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	if err := validateObservedRoots(
		observedByPath,
		managedRoots,
	); err != nil {
		return InstallationCurrentness{}, err
	}
	if err := requireObservationCoverage(
		manifestByPath,
		currentByPath,
		observedByPath,
	); err != nil {
		return InstallationCurrentness{}, err
	}

	paths := make([]PathCurrentness, 0, len(observations))
	vacant := make([]VacantTarget, 0, len(projection.outputs))
	for _, path := range sortedObservationPaths(observedByPath) {
		observation := observedByPath[path]
		manifestPath, manifestOwned := manifestByPath[path]
		currentOutput, desired := currentByPath[path]
		legacyPath, knownLegacy := legacyByPath[path]
		if err := validateObservationComponent(
			observation,
			manifestPath,
			manifestOwned,
			currentOutput,
			desired,
			legacyPath,
			knownLegacy,
		); err != nil {
			return InstallationCurrentness{}, err
		}
		classified, target, emitted, err := classifyObservedPath(
			observation,
			manifestPath,
			manifestOwned,
			currentOutput,
			desired,
			legacyPath,
			knownLegacy,
			manifestBasis,
			legacyBasis,
		)
		if err != nil {
			return InstallationCurrentness{}, err
		}
		if emitted == classificationEmitted {
			paths = append(paths, classified)
		}
		if emitted == vacancyEmitted {
			vacant = append(vacant, target)
		}
		if emitted != classificationEmitted &&
			emitted != vacancyEmitted {
			return InstallationCurrentness{}, fmt.Errorf(
				"path %s produced no coherent currentness result",
				path,
			)
		}
	}
	return InstallationCurrentness{
		baseline:      baseline,
		manifest:      cloneInstallationManifest(manifest),
		manifestBasis: manifestBasis,
		projection:    cloneHostAdapterProjection(projection),
		managedRoots:  slices.Clone(managedRoots),
		paths:         paths,
		vacantTargets: vacant,
	}, nil
}

type coherentManagedFragmentGroup struct {
	desired  []ManagedFragment
	manifest []ManagedFragmentRecord
	legacy   []ManagedFragmentRecord
}

func classifyCoherentManagedCarriers(
	manifest InstallationManifest,
	projection HostAdapterProjection,
	inputs []ManagedCarrierInput,
	legacy ManagedFragmentLegacyRegistry,
) ([]ManagedCarrierCurrentness, error) {
	plans, err := BuildInstalledCoherentManagedCarrierObservationPlans(
		manifest,
		projection,
		legacy,
	)
	if err != nil {
		return nil, err
	}
	return classifyCoherentManagedCarrierInputs(
		projection.targetRoots,
		plans,
		inputs,
	)
}

func classifyFirstCoherentManagedCarriers(
	projection HostAdapterProjection,
	inputs []ManagedCarrierInput,
	legacy ManagedFragmentLegacyRegistry,
) ([]ManagedCarrierCurrentness, error) {
	plans, err := BuildFirstCoherentManagedCarrierObservationPlans(
		projection,
		legacy,
	)
	if err != nil {
		return nil, err
	}
	return classifyCoherentManagedCarrierInputs(
		projection.targetRoots,
		plans,
		inputs,
	)
}

func BuildInstalledCoherentManagedCarrierObservationPlans(
	manifest InstallationManifest,
	projection HostAdapterProjection,
	legacy ManagedFragmentLegacyRegistry,
) ([]ManagedFragmentObservationPlan, error) {
	manifestBasis := manifest.OwnershipBasis()
	if !manifestBasis.valid() {
		return nil, fmt.Errorf("installation manifest is invalid")
	}
	if err := validateProjectionBinding(manifest, projection); err != nil {
		return nil, err
	}
	records, err := manifest.ManagedFragmentRecords()
	if err != nil {
		return nil, err
	}
	return buildCoherentManagedCarrierObservationPlans(
		projection,
		records,
		manifestBasis,
		legacy,
	)
}

func BuildFirstCoherentManagedCarrierObservationPlans(
	projection HostAdapterProjection,
	legacy ManagedFragmentLegacyRegistry,
) ([]ManagedFragmentObservationPlan, error) {
	if len(projection.retainedManagedFragmentComponents) != 0 {
		return nil, fmt.Errorf(
			"installed managed-fragment retention requires a prior manifest",
		)
	}
	if _, _, err := validateFirstInstallationProjection(projection); err != nil {
		return nil, err
	}
	return buildCoherentManagedCarrierObservationPlans(
		projection,
		nil,
		OwnershipBasis{},
		legacy,
	)
}

func buildCoherentManagedCarrierObservationPlans(
	projection HostAdapterProjection,
	manifestRecords []ManagedFragmentRecord,
	manifestBasis OwnershipBasis,
	legacy ManagedFragmentLegacyRegistry,
) ([]ManagedFragmentObservationPlan, error) {
	if err := validateManagedFragmentLegacyRegistry(legacy); err != nil {
		return nil, err
	}
	desired, err := canonicalDesiredManagedFragments(
		projection.fragments,
	)
	if err != nil {
		return nil, err
	}
	manifestRecords, err = canonicalManagedFragmentRecords(
		manifestRecords,
	)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]coherentManagedFragmentGroup)
	for _, fragment := range desired {
		path := fragment.coordinate.carrierPath
		group := groups[path]
		group.desired = append(
			group.desired,
			cloneManagedFragment(fragment),
		)
		groups[path] = group
	}
	for _, record := range manifestRecords {
		path := record.coordinate.carrierPath
		group := groups[path]
		group.manifest = append(
			group.manifest,
			cloneManagedFragmentRecord(record),
		)
		groups[path] = group
	}
	for _, record := range legacy.records {
		path := record.coordinate.carrierPath
		group := groups[path]
		group.legacy = append(
			group.legacy,
			cloneManagedFragmentRecord(record),
		)
		groups[path] = group
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf(
			"coherent installation has no managed fragment groups",
		)
	}
	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	plans := make(
		[]ManagedFragmentObservationPlan,
		0,
		len(paths),
	)
	for _, path := range paths {
		group := groups[path]
		if !pathWithinAnyRoot(path, projection.targetRoots) {
			return nil, fmt.Errorf(
				"managed carrier %s is outside target roots",
				path,
			)
		}
		baseline := NoPriorManagedFragmentBaseline()
		if len(group.manifest) != 0 {
			if !manifestBasis.valid() {
				return nil, fmt.Errorf(
					"managed carrier %s has ownership records without a manifest basis",
					path,
				)
			}
			baseline, err = NewManagedFragmentManifestBaseline(
				group.manifest,
				manifestBasis,
			)
			if err != nil {
				return nil, err
			}
		}
		groupLegacy := NoManagedFragmentLegacyRegistry()
		if len(group.legacy) != 0 {
			groupLegacy, err = NewManagedFragmentLegacyRegistry(
				group.legacy,
				legacy.basis,
			)
			if err != nil {
				return nil, err
			}
		}
		observationPlan, err := BuildManagedFragmentObservationPlan(
			group.desired,
			baseline,
			groupLegacy,
		)
		if err != nil {
			return nil, err
		}
		observationPlan.retainedManagedFragmentComponents = slices.Clone(
			projection.retainedManagedFragmentComponents,
		)
		plans = append(plans, observationPlan)
	}
	return plans, nil
}

func classifyCoherentManagedCarrierInputs(
	managedRoots []string,
	plans []ManagedFragmentObservationPlan,
	inputs []ManagedCarrierInput,
) ([]ManagedCarrierCurrentness, error) {
	inputByPath, err := managedCarrierInputsByPath(
		inputs,
		managedRoots,
	)
	if err != nil {
		return nil, err
	}
	if len(inputByPath) != len(plans) {
		return nil, fmt.Errorf(
			"managed carrier input coverage is incomplete",
		)
	}
	currentness := make(
		[]ManagedCarrierCurrentness,
		0,
		len(plans),
	)
	for _, plan := range plans {
		input, exists := inputByPath[plan.carrierPath]
		if !exists {
			return nil, fmt.Errorf(
				"managed carrier %s has no exact input",
				plan.carrierPath,
			)
		}
		plan, err = materializeRetainedManagedFragments(
			plan,
			input,
		)
		if err != nil {
			return nil, err
		}
		observation, err := ObserveManagedCarrier(
			plan,
			input,
		)
		if err != nil {
			return nil, err
		}
		classified, err := ClassifyManagedFragmentCurrentness(
			plan,
			observation,
		)
		if err != nil {
			return nil, err
		}
		currentness = append(currentness, classified)
	}
	return currentness, nil
}

func managedCarrierInputsByPath(
	inputs []ManagedCarrierInput,
	roots []string,
) (map[string]ManagedCarrierInput, error) {
	result := make(
		map[string]ManagedCarrierInput,
		len(inputs),
	)
	for _, input := range inputs {
		if err := validateManagedCarrierInput(input); err != nil {
			return nil, err
		}
		if !pathWithinAnyRoot(input.path, roots) {
			return nil, fmt.Errorf(
				"managed carrier input %s is outside target roots",
				input.path,
			)
		}
		if _, duplicate := result[input.path]; duplicate {
			return nil, fmt.Errorf(
				"managed carrier input repeats %s",
				input.path,
			)
		}
		result[input.path] = cloneManagedCarrierInput(input)
	}
	return result, nil
}

func validateManagedCarrierInput(
	input ManagedCarrierInput,
) error {
	canonical, err := parseCanonicalAbsolutePath(input.path)
	if err != nil || canonical != input.path {
		return fmt.Errorf("managed carrier input path is invalid")
	}
	if input.kind == ManagedCarrierMissing {
		if len(input.content) != 0 ||
			input.digest != "" ||
			input.mode != 0 {
			return fmt.Errorf(
				"missing managed carrier carries present-file evidence",
			)
		}
		return nil
	}
	if input.kind != ManagedCarrierPresent ||
		input.digest != managedFragmentDigest(input.content) ||
		!validPermissionMode(input.mode) {
		return fmt.Errorf("present managed carrier input is invalid")
	}
	return nil
}

func cloneManagedCarrierCurrentnessValues(
	source []ManagedCarrierCurrentness,
) []ManagedCarrierCurrentness {
	result := make([]ManagedCarrierCurrentness, len(source))
	for index, currentness := range source {
		result[index] = ManagedCarrierCurrentness{
			plan: cloneManagedFragmentObservationPlan(
				currentness.plan,
			),
			observation: cloneManagedCarrierObservation(
				currentness.observation,
			),
			states: cloneManagedFragmentCurrentness(
				currentness.states,
			),
			vacant: cloneManagedFragmentVacantTargets(
				currentness.vacant,
			),
		}
	}
	return result
}

func cloneManagedFragmentVacantTargets(
	source []ManagedFragmentVacantTarget,
) []ManagedFragmentVacantTarget {
	result := make(
		[]ManagedFragmentVacantTarget,
		len(source),
	)
	for index, target := range source {
		result[index] = ManagedFragmentVacantTarget{
			fragment: cloneManagedFragment(target.fragment),
		}
	}
	return result
}

func cloneInstallationCurrentness(
	currentness InstallationCurrentness,
) InstallationCurrentness {
	return InstallationCurrentness{
		baseline:      currentness.baseline,
		manifest:      cloneInstallationManifest(currentness.manifest),
		manifestBasis: currentness.manifestBasis,
		projection:    cloneHostAdapterProjection(currentness.projection),
		managedRoots:  slices.Clone(currentness.managedRoots),
		paths:         slices.Clone(currentness.paths),
		vacantTargets: slices.Clone(currentness.vacantTargets),
	}
}
