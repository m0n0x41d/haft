package initplanning

import "fmt"

func CompileHostAdapterReconciliation(
	currentness InstallationCurrentness,
) (HostAdapterInstallPlan, error) {
	builder, err := hostAdapterReconciliationBuilder(currentness)
	if err != nil {
		return HostAdapterInstallPlan{}, err
	}
	return builder.Build()
}

func hostAdapterReconciliationBuilder(
	currentness InstallationCurrentness,
) (HostAdapterInstallPlanBuilder, error) {
	if err := validateReconciliationBaseline(currentness); err != nil {
		return HostAdapterInstallPlanBuilder{}, err
	}
	projection := currentness.projection
	if projection.projectRoot == "" || projection.projectID.String() == "" {
		return HostAdapterInstallPlanBuilder{}, fmt.Errorf("installation currentness lacks a host projection")
	}
	outputs := make(map[string]RenderedOutput, len(projection.outputs))
	for _, output := range projection.outputs {
		outputs[output.path] = output
	}
	included := make(map[string]struct{}, len(outputs))
	builder := NewHostAdapterInstallPlanBuilder(projection.host)
	builder = builder.AtEdition(projection.edition)
	builder = builder.PublishedFrom(projection.publication)
	builder = builder.ForProject(
		projection.projectRoot,
		projection.projectID.String(),
	)
	builder = builder.WithSelection(
		projection.scope,
		projection.components,
	)
	builder = builder.WithManifestBasis(currentness.manifestBasis)
	for _, root := range currentness.managedRoots {
		builder = builder.AddTargetRoot(root)
	}
	for _, path := range currentness.paths {
		next, emitted, err := addCurrentnessEffect(
			builder,
			path,
			outputs,
		)
		if err != nil {
			return HostAdapterInstallPlanBuilder{}, err
		}
		builder = next
		if emitted != "" {
			included[emitted] = struct{}{}
		}
	}
	for _, target := range currentness.vacantTargets {
		output, exists := outputs[target.path]
		if !exists || output.digest != target.digest {
			return HostAdapterInstallPlanBuilder{}, fmt.Errorf("vacant target %s differs from the host projection", target.path)
		}
		expectation, err := ExpectMissing(target.path)
		if err != nil {
			return HostAdapterInstallPlanBuilder{}, err
		}
		builder = builder.AddOutput(expectation, output)
		included[target.path] = struct{}{}
	}
	for path := range outputs {
		if _, exists := included[path]; !exists {
			return HostAdapterInstallPlanBuilder{}, fmt.Errorf("host projection path %s lacks a currentness disposition", path)
		}
	}
	builder = builder.RecoverWith(projection.recovery)
	return builder, nil
}

func validateReconciliationBaseline(
	currentness InstallationCurrentness,
) error {
	switch currentness.baseline {
	case InstallationBaselineManifest:
		if !currentness.manifestBasis.valid() {
			return fmt.Errorf("installed currentness lacks a manifest basis")
		}
		return nil
	case InstallationBaselineNoPriorManifest:
		if currentness.manifestBasis.valid() || len(currentness.manifest.canonical) != 0 {
			return fmt.Errorf("first-install currentness fabricates a manifest basis")
		}
		return nil
	default:
		return fmt.Errorf("installation currentness baseline is invalid")
	}
}

func addCurrentnessEffect(
	builder HostAdapterInstallPlanBuilder,
	path PathCurrentness,
	outputs map[string]RenderedOutput,
) (HostAdapterInstallPlanBuilder, string, error) {
	output, desired := outputs[path.path]
	switch path.kind {
	case PathCurrentOwned:
		if !desired {
			return builder, "", fmt.Errorf("current-owned path %s has no desired output", path.path)
		}
		expectation, err := ExpectCurrentOwned(
			path.path,
			path.observedDigest,
			path.observedMode,
			path.basis,
		)
		if err != nil {
			return builder, "", err
		}
		return builder.AddOutput(expectation, output), path.path, nil
	case PathOutdatedOwned:
		if !desired {
			return builder, "", fmt.Errorf("outdated-owned path %s has no desired output", path.path)
		}
		expectation, err := ExpectOutdatedOwned(
			path.path,
			path.observedDigest,
			path.observedMode,
			path.basis,
		)
		if err != nil {
			return builder, "", err
		}
		return builder.AddOutput(expectation, output), path.path, nil
	case PathMissingOwned:
		if !desired {
			return builder, "", nil
		}
		expectation, err := ExpectMissingOwned(
			path.path,
			path.manifestDigest,
			path.manifestMode,
			path.basis,
		)
		if err != nil {
			return builder, "", err
		}
		return builder.AddOutput(expectation, output), path.path, nil
	case PathLocallyModifiedOwned:
		return addLocallyModifiedEffect(builder, path, output, desired)
	case PathKnownLegacyExact:
		return addKnownLegacyEffect(builder, path, output, desired)
	case PathForeign:
		return addForeignEffect(builder, path, output, desired)
	case PathOrphanedOwned:
		if desired {
			return builder, "", fmt.Errorf("orphaned-owned path %s still has a desired output", path.path)
		}
		expectation, err := ExpectOrphanedOwned(
			path.path,
			path.observedDigest,
			path.observedMode,
			path.basis,
		)
		if err != nil {
			return builder, "", err
		}
		removal, err := NewPlannedRemoval(expectation, path.component)
		if err != nil {
			return builder, "", err
		}
		return builder.AddRemoval(removal), "", nil
	default:
		return builder, "", fmt.Errorf("path %s has an unknown currentness state", path.path)
	}
}

func addLocallyModifiedEffect(
	builder HostAdapterInstallPlanBuilder,
	path PathCurrentness,
	output RenderedOutput,
	desired bool,
) (HostAdapterInstallPlanBuilder, string, error) {
	conflict, err := NewLocallyModifiedOwnedConflict(
		path.path,
		"manifest-owned path differs from its recorded digest; preserve it until an explicit keep, export, or replace operation",
		path.basis,
	)
	if err != nil {
		return builder, "", err
	}
	next := builder.AddConflict(conflict)
	if !desired {
		return next, "", nil
	}
	expectation, err := ExpectLocallyModifiedOwned(
		path.path,
		path.observedDigest,
		path.observedMode,
		path.manifestDigest,
		path.manifestMode,
		path.basis,
	)
	if err != nil {
		return builder, "", err
	}
	next = next.AddOutput(expectation, output)
	return next, path.path, nil
}

func addKnownLegacyEffect(
	builder HostAdapterInstallPlanBuilder,
	path PathCurrentness,
	output RenderedOutput,
	desired bool,
) (HostAdapterInstallPlanBuilder, string, error) {
	expectation, err := ExpectKnownLegacyExact(
		path.path,
		path.observedDigest,
		path.observedMode,
		path.basis,
	)
	if err != nil {
		return builder, "", err
	}
	if desired {
		return builder.AddOutput(expectation, output), path.path, nil
	}
	removal, err := NewPlannedRemoval(expectation, path.component)
	if err != nil {
		return builder, "", err
	}
	return builder.AddRemoval(removal), "", nil
}

func addForeignEffect(
	builder HostAdapterInstallPlanBuilder,
	path PathCurrentness,
	output RenderedOutput,
	desired bool,
) (HostAdapterInstallPlanBuilder, string, error) {
	if !desired {
		return builder, "", nil
	}
	expectation, err := ExpectForeign(
		path.path,
		path.observedDigest,
		path.observedMode,
	)
	if err != nil {
		return builder, "", err
	}
	conflict, err := NewForeignConflict(
		path.path,
		"unowned path collides with the desired host-adapter projection; preserve it",
	)
	if err != nil {
		return builder, "", err
	}
	next := builder.AddOutput(expectation, output)
	next = next.AddConflict(conflict)
	return next, path.path, nil
}
