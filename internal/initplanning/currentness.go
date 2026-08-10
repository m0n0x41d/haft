package initplanning

import (
	"fmt"
	"io/fs"
	"slices"
	"sort"
)

type PathObservationKind string

const (
	PathObservedMissing PathObservationKind = "missing"
	PathObservedPresent PathObservationKind = "present"
)

type PathObservation struct {
	path       string
	components ComponentSet
	kind       PathObservationKind
	digest     string
	mode       fs.FileMode
}

func ObserveMissingPath(
	path string,
	component Component,
) (PathObservation, error) {
	components, err := singletonComponentSet(component)
	if err != nil {
		return PathObservation{}, fmt.Errorf(
			"missing-path observation component is invalid",
		)
	}
	return ObserveMissingPathForComponents(path, components)
}

func ObserveMissingPathForComponents(
	path string,
	components ComponentSet,
) (PathObservation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathObservation{}, err
	}
	if err := validateComponentSet(components); err != nil {
		return PathObservation{}, fmt.Errorf(
			"missing-path observation components are invalid: %w",
			err,
		)
	}
	return PathObservation{
		path:       canonical,
		components: ComponentSet{values: components.Values()},
		kind:       PathObservedMissing,
	}, nil
}

func ObservePresentPath(
	path string,
	component Component,
	digest string,
	mode fs.FileMode,
) (PathObservation, error) {
	components, err := singletonComponentSet(component)
	if err != nil {
		return PathObservation{}, fmt.Errorf(
			"present-path observation component is invalid",
		)
	}
	return ObservePresentPathForComponents(
		path,
		components,
		digest,
		mode,
	)
}

func ObservePresentPathForComponents(
	path string,
	components ComponentSet,
	digest string,
	mode fs.FileMode,
) (PathObservation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathObservation{}, err
	}
	if err := validateComponentSet(components); err != nil {
		return PathObservation{}, fmt.Errorf(
			"present-path observation components are invalid: %w",
			err,
		)
	}
	if !sha256DigestPattern.MatchString(digest) {
		return PathObservation{}, fmt.Errorf("present-path observation digest is invalid")
	}
	if mode == 0 || mode&^fs.FileMode(0o777) != 0 {
		return PathObservation{}, fmt.Errorf("present-path observation mode is invalid")
	}
	return PathObservation{
		path:       canonical,
		components: ComponentSet{values: components.Values()},
		kind:       PathObservedPresent,
		digest:     digest,
		mode:       mode,
	}, nil
}

func (observation PathObservation) Path() string {
	return observation.path
}

func (observation PathObservation) Component() Component {
	component, _ := observation.components.single()
	return component
}

func (observation PathObservation) Components() ComponentSet {
	return ComponentSet{values: observation.components.Values()}
}

func (observation PathObservation) Kind() PathObservationKind {
	return observation.kind
}

func (observation PathObservation) Digest() string {
	return observation.digest
}

func (observation PathObservation) Mode() fs.FileMode {
	return observation.mode
}

type PathCurrentnessKind string

const (
	PathCurrentOwned         PathCurrentnessKind = "current_owned"
	PathOutdatedOwned        PathCurrentnessKind = "outdated_owned"
	PathLocallyModifiedOwned PathCurrentnessKind = "locally_modified_owned"
	PathKnownLegacyExact     PathCurrentnessKind = "known_legacy_exact"
	PathForeign              PathCurrentnessKind = "foreign"
	PathOrphanedOwned        PathCurrentnessKind = "orphaned_owned"
	PathMissingOwned         PathCurrentnessKind = "missing_owned"
)

type PathCurrentness struct {
	path           string
	component      Component
	kind           PathCurrentnessKind
	observedDigest string
	manifestDigest string
	desiredDigest  string
	observedMode   fs.FileMode
	manifestMode   fs.FileMode
	desiredMode    fs.FileMode
	basis          OwnershipBasis
}

func (currentness PathCurrentness) Path() string {
	return currentness.path
}

func (currentness PathCurrentness) Component() Component {
	return currentness.component
}

func (currentness PathCurrentness) Kind() PathCurrentnessKind {
	return currentness.kind
}

func (currentness PathCurrentness) ObservedDigest() string {
	return currentness.observedDigest
}

func (currentness PathCurrentness) ManifestDigest() string {
	return currentness.manifestDigest
}

func (currentness PathCurrentness) DesiredDigest() string {
	return currentness.desiredDigest
}

func (currentness PathCurrentness) Mode() fs.FileMode {
	return currentness.observedMode
}

func (currentness PathCurrentness) ManifestMode() fs.FileMode {
	return currentness.manifestMode
}

func (currentness PathCurrentness) DesiredMode() fs.FileMode {
	return currentness.desiredMode
}

func (currentness PathCurrentness) OwnershipBasis() OwnershipBasis {
	return currentness.basis
}

type VacantTarget struct {
	path      string
	component Component
	digest    string
	mode      fs.FileMode
}

func (target VacantTarget) Path() string {
	return target.path
}

func (target VacantTarget) Component() Component {
	return target.component
}

func (target VacantTarget) DesiredDigest() string {
	return target.digest
}

func (target VacantTarget) Mode() fs.FileMode {
	return target.mode
}

type InstallationCurrentness struct {
	baseline      InstallationBaselineKind
	manifest      InstallationManifest
	manifestBasis OwnershipBasis
	projection    HostAdapterProjection
	managedRoots  []string
	paths         []PathCurrentness
	vacantTargets []VacantTarget
}

type InstallationBaselineKind string

const (
	InstallationBaselineNoPriorManifest InstallationBaselineKind = "no_prior_manifest"
	InstallationBaselineManifest        InstallationBaselineKind = "installation_manifest"
)

func ClassifyInstallationCurrentness(
	manifest InstallationManifest,
	projection HostAdapterProjection,
	observations []PathObservation,
	legacySelection LegacyRegistrySelection,
) (InstallationCurrentness, error) {
	if len(manifest.wire.ManagedFragments) != 0 ||
		len(projection.fragments) != 0 {
		return InstallationCurrentness{}, fmt.Errorf(
			"whole-path installation currentness cannot classify managed fragment ownership",
		)
	}
	manifestBasis := manifest.OwnershipBasis()
	if !manifestBasis.valid() {
		return InstallationCurrentness{}, fmt.Errorf("installation manifest is invalid")
	}
	if err := validateProjectionBinding(manifest, projection); err != nil {
		return InstallationCurrentness{}, err
	}
	managedRoots, err := mergeManagedRoots(
		manifest.wire.TargetRoots,
		projection.targetRoots,
	)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	legacyByPath, legacyBasis, err := prepareLegacyRegistry(
		manifest,
		legacySelection,
		managedRoots,
	)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	manifestByPath := manifestPathsByPath(manifest.RenderedPaths())
	currentByPath, err := renderedOutputsByPath(projection.outputs)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	observedByPath, err := observationsByPath(observations)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	if err := validateObservedRoots(observedByPath, managedRoots); err != nil {
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
	ordered := sortedObservationPaths(observedByPath)
	for _, path := range ordered {
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
		switch emitted {
		case classificationEmitted:
			paths = append(paths, classified)
		case vacancyEmitted:
			vacant = append(vacant, target)
		default:
			return InstallationCurrentness{}, fmt.Errorf("path %s produced no currentness result", path)
		}
	}
	return InstallationCurrentness{
		baseline:      InstallationBaselineManifest,
		manifest:      cloneInstallationManifest(manifest),
		manifestBasis: manifestBasis,
		projection:    cloneHostAdapterProjection(projection),
		managedRoots:  slices.Clone(managedRoots),
		paths:         paths,
		vacantTargets: vacant,
	}, nil
}

func ClassifyFirstInstallationCurrentness(
	projection HostAdapterProjection,
	observations []PathObservation,
	legacySelection LegacyRegistrySelection,
) (InstallationCurrentness, error) {
	if len(projection.fragments) != 0 {
		return InstallationCurrentness{}, fmt.Errorf(
			"whole-path first-install currentness cannot classify managed fragment ownership",
		)
	}
	managedRoots, currentByPath, err := validateFirstInstallationProjection(projection)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	legacyByPath, legacyBasis, err := prepareFirstInstallationLegacyRegistry(
		projection,
		legacySelection,
		managedRoots,
	)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	observedByPath, err := observationsByPath(observations)
	if err != nil {
		return InstallationCurrentness{}, err
	}
	if err := validateObservedRoots(observedByPath, managedRoots); err != nil {
		return InstallationCurrentness{}, err
	}
	if err := requireObservationCoverage(
		map[string]ManifestPath{},
		currentByPath,
		observedByPath,
	); err != nil {
		return InstallationCurrentness{}, err
	}
	paths := make([]PathCurrentness, 0, len(observations))
	vacant := make([]VacantTarget, 0, len(projection.outputs))
	for _, path := range sortedObservationPaths(observedByPath) {
		observation := observedByPath[path]
		currentOutput, desired := currentByPath[path]
		legacyPath, knownLegacy := legacyByPath[path]
		if err := validateObservationComponent(
			observation,
			ManifestPath{},
			false,
			currentOutput,
			desired,
			legacyPath,
			knownLegacy,
		); err != nil {
			return InstallationCurrentness{}, err
		}
		classified, target, emitted, err := classifyObservedPath(
			observation,
			ManifestPath{},
			false,
			currentOutput,
			desired,
			legacyPath,
			knownLegacy,
			OwnershipBasis{},
			legacyBasis,
		)
		if err != nil {
			return InstallationCurrentness{}, err
		}
		switch emitted {
		case classificationEmitted:
			paths = append(paths, classified)
		case vacancyEmitted:
			vacant = append(vacant, target)
		default:
			return InstallationCurrentness{}, fmt.Errorf("path %s produced no first-install currentness result", path)
		}
	}
	return InstallationCurrentness{
		baseline:      InstallationBaselineNoPriorManifest,
		projection:    cloneHostAdapterProjection(projection),
		managedRoots:  slices.Clone(managedRoots),
		paths:         paths,
		vacantTargets: vacant,
	}, nil
}

func validateFirstInstallationProjection(
	projection HostAdapterProjection,
) ([]string, map[string]RenderedOutput, error) {
	if projection.projectRoot == "" ||
		projection.projectID.String() == "" ||
		!projection.publication.valid() ||
		len(projection.recovery.argv) == 0 {
		return nil, nil, fmt.Errorf("first-install host adapter projection is invalid")
	}
	retained, err := canonicalRetainedManagedFragmentComponents(
		projection.retainedManagedFragmentComponents,
		projection.components,
	)
	if err != nil || !slices.Equal(
		retained,
		projection.retainedManagedFragmentComponents,
	) {
		return nil, nil, fmt.Errorf(
			"first-install host adapter projection retained managed fragments are invalid",
		)
	}
	managedRoots, err := mergeManagedRoots(projection.targetRoots, nil)
	if err != nil {
		return nil, nil, err
	}
	outputs, err := validateProjectionOutputs(
		projection.outputs,
		projection.components,
		managedRoots,
	)
	if err != nil {
		return nil, nil, err
	}
	if _, err := validateProjectionManagedFragments(
		projection.fragments,
		projection.components,
		managedRoots,
		outputs,
	); err != nil {
		return nil, nil, err
	}
	currentByPath, err := renderedOutputsByPath(outputs)
	if err != nil {
		return nil, nil, err
	}
	return managedRoots, currentByPath, nil
}

func prepareFirstInstallationLegacyRegistry(
	projection HostAdapterProjection,
	selection LegacyRegistrySelection,
	managedRoots []string,
) (map[string]KnownLegacyPath, OwnershipBasis, error) {
	switch selection.kind {
	case LegacyRegistryNotSelected:
		return map[string]KnownLegacyPath{}, OwnershipBasis{}, nil
	case LegacyRegistrySelected:
		registry := selection.registry
		if registry.wire.ProjectRoot != projection.projectRoot ||
			registry.wire.ProjectID != projection.projectID.String() ||
			registry.wire.Host != projection.host ||
			registry.wire.Scope != projection.scope {
			return nil, OwnershipBasis{}, fmt.Errorf("known-legacy registry belongs to another first installation binding")
		}
		basis := registry.OwnershipBasis()
		if !basis.valid() {
			return nil, OwnershipBasis{}, fmt.Errorf("known-legacy registry basis is invalid")
		}
		entries := make(map[string]KnownLegacyPath, len(registry.wire.Paths))
		for _, path := range registry.wire.Paths {
			if !pathWithinAnyRoot(path.Path, managedRoots) {
				return nil, OwnershipBasis{}, fmt.Errorf("known-legacy path %s is outside the first installation roots", path.Path)
			}
			entries[path.Path] = path
		}
		return entries, basis, nil
	default:
		return nil, OwnershipBasis{}, fmt.Errorf("known-legacy registry selection is invalid")
	}
}

func validateProjectionBinding(
	manifest InstallationManifest,
	projection HostAdapterProjection,
) error {
	if projection.projectRoot == "" || projection.projectID.String() == "" {
		return fmt.Errorf("host adapter projection is invalid")
	}
	if projection.projectRoot != manifest.wire.ProjectRoot ||
		projection.projectID.String() != manifest.wire.ProjectID ||
		projection.host != manifest.wire.Host ||
		projection.scope != manifest.wire.InstallScope {
		return fmt.Errorf("host adapter projection belongs to another installation binding")
	}
	retained, err := canonicalRetainedManagedFragmentComponents(
		projection.retainedManagedFragmentComponents,
		projection.components,
	)
	if err != nil || !slices.Equal(
		retained,
		projection.retainedManagedFragmentComponents,
	) {
		return fmt.Errorf(
			"host adapter projection retained managed fragments are invalid",
		)
	}
	outputs, err := validateProjectionOutputs(
		projection.outputs,
		projection.components,
		projection.targetRoots,
	)
	if err != nil {
		return fmt.Errorf("host adapter projection: %w", err)
	}
	fragments, err := validateProjectionManagedFragments(
		projection.fragments,
		projection.components,
		projection.targetRoots,
		outputs,
	)
	if err != nil {
		return fmt.Errorf("host adapter projection: %w", err)
	}
	if len(projection.recovery.argv) == 0 ||
		len(outputs) != len(projection.outputs) ||
		len(fragments) != len(projection.fragments) {
		return fmt.Errorf("host adapter projection is invalid")
	}
	return nil
}

func prepareLegacyRegistry(
	manifest InstallationManifest,
	selection LegacyRegistrySelection,
	managedRoots []string,
) (map[string]KnownLegacyPath, OwnershipBasis, error) {
	switch selection.kind {
	case LegacyRegistryNotSelected:
		return map[string]KnownLegacyPath{}, OwnershipBasis{}, nil
	case LegacyRegistrySelected:
		registry := selection.registry
		if registry.wire.ProjectRoot != manifest.wire.ProjectRoot ||
			registry.wire.ProjectID != manifest.wire.ProjectID ||
			registry.wire.Host != manifest.wire.Host ||
			registry.wire.Scope != manifest.wire.InstallScope {
			return nil, OwnershipBasis{}, fmt.Errorf("known-legacy registry belongs to another installation binding")
		}
		basis := registry.OwnershipBasis()
		if !basis.valid() {
			return nil, OwnershipBasis{}, fmt.Errorf("known-legacy registry basis is invalid")
		}
		entries := make(map[string]KnownLegacyPath, len(registry.wire.Paths))
		for _, path := range registry.wire.Paths {
			if !pathWithinAnyRoot(path.Path, managedRoots) {
				return nil, OwnershipBasis{}, fmt.Errorf("known-legacy path %s is outside the installation roots", path.Path)
			}
			entries[path.Path] = path
		}
		return entries, basis, nil
	default:
		return nil, OwnershipBasis{}, fmt.Errorf("known-legacy registry selection is invalid")
	}
}

func mergeManagedRoots(left []string, right []string) ([]string, error) {
	seen := make(map[string]struct{}, len(left)+len(right))
	for _, candidate := range append(slices.Clone(left), right...) {
		root, err := parseCanonicalAbsolutePath(candidate)
		if err != nil || root != candidate {
			return nil, fmt.Errorf("installation managed root is invalid")
		}
		seen[root] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for root := range seen {
		result = append(result, root)
	}
	sort.Strings(result)
	if len(result) == 0 {
		return nil, fmt.Errorf("installation has no managed roots")
	}
	return result, nil
}

func validateObservedRoots(
	observations map[string]PathObservation,
	managedRoots []string,
) error {
	for path := range observations {
		if !pathWithinAnyRoot(path, managedRoots) {
			return fmt.Errorf("observed path %s is outside the installation roots", path)
		}
	}
	return nil
}

func manifestPathsByPath(paths []ManifestPath) map[string]ManifestPath {
	result := make(map[string]ManifestPath, len(paths))
	for _, path := range paths {
		result[path.Path] = path
	}
	return result
}

func renderedOutputsByPath(
	outputs []RenderedOutput,
) (map[string]RenderedOutput, error) {
	result := make(map[string]RenderedOutput, len(outputs))
	for _, output := range outputs {
		if output.path == "" || !sha256DigestPattern.MatchString(output.digest) {
			return nil, fmt.Errorf("current rendered output is invalid")
		}
		if _, duplicate := result[output.path]; duplicate {
			return nil, fmt.Errorf("current rendered outputs repeat path %s", output.path)
		}
		result[output.path] = output
	}
	return result, nil
}

func observationsByPath(
	observations []PathObservation,
) (map[string]PathObservation, error) {
	result := make(map[string]PathObservation, len(observations))
	for _, observation := range observations {
		if err := validatePathObservation(observation); err != nil {
			return nil, err
		}
		if _, duplicate := result[observation.path]; duplicate {
			return nil, fmt.Errorf("path observations repeat path %s", observation.path)
		}
		result[observation.path] = observation
	}
	return result, nil
}

func validatePathObservation(observation PathObservation) error {
	canonical, err := parseCanonicalAbsolutePath(observation.path)
	if err != nil || canonical != observation.path {
		return fmt.Errorf("path observation path is invalid")
	}
	if err := validateComponentSet(observation.components); err != nil {
		return fmt.Errorf("path observation components are invalid: %w", err)
	}
	switch observation.kind {
	case PathObservedMissing:
		if observation.digest != "" || observation.mode != 0 {
			return fmt.Errorf("missing-path observation carries present-file data")
		}
		return nil
	case PathObservedPresent:
		if !sha256DigestPattern.MatchString(observation.digest) {
			return fmt.Errorf("present-path observation digest is invalid")
		}
		if observation.mode == 0 || observation.mode&^fs.FileMode(0o777) != 0 {
			return fmt.Errorf("present-path observation mode is invalid")
		}
		return nil
	default:
		return fmt.Errorf("path observation kind is invalid")
	}
}

func requireObservationCoverage(
	manifest map[string]ManifestPath,
	current map[string]RenderedOutput,
	observed map[string]PathObservation,
) error {
	required := make(map[string]struct{}, len(manifest)+len(current))
	for path := range manifest {
		required[path] = struct{}{}
	}
	for path := range current {
		required[path] = struct{}{}
	}
	for path := range required {
		if _, exists := observed[path]; !exists {
			return fmt.Errorf("required path %s has no filesystem observation", path)
		}
	}
	for path, observation := range observed {
		_, requiredPath := required[path]
		if !requiredPath && observation.kind == PathObservedMissing {
			return fmt.Errorf("unrelated missing path %s cannot produce currentness", path)
		}
	}
	return nil
}

func sortedObservationPaths(
	observed map[string]PathObservation,
) []string {
	paths := make([]string, 0, len(observed))
	for path := range observed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func validateObservationComponent(
	observation PathObservation,
	manifest ManifestPath,
	manifestOwned bool,
	current RenderedOutput,
	desired bool,
	legacy KnownLegacyPath,
	knownLegacy bool,
) error {
	component, singleton := observation.components.single()
	if !singleton {
		return fmt.Errorf(
			"path %s whole-file observation does not name exactly one component",
			observation.path,
		)
	}
	if knownLegacy && desired && legacy.Component != current.Component() {
		return fmt.Errorf("path %s desired component differs from its legacy ownership witness", observation.path)
	}
	if manifestOwned && component != manifest.Component {
		return fmt.Errorf("path %s observation component differs from manifest", observation.path)
	}
	if !manifestOwned && desired && component != current.Component() {
		return fmt.Errorf("path %s observation component differs from current projection", observation.path)
	}
	if !manifestOwned && !desired && knownLegacy && component != legacy.Component {
		return fmt.Errorf("path %s observation component differs from legacy registry", observation.path)
	}
	return nil
}

type currentnessEmission string

const (
	classificationEmitted currentnessEmission = "classification"
	vacancyEmitted        currentnessEmission = "vacancy"
)

func classifyObservedPath(
	observation PathObservation,
	manifest ManifestPath,
	manifestOwned bool,
	current RenderedOutput,
	desired bool,
	legacy KnownLegacyPath,
	knownLegacy bool,
	manifestBasis OwnershipBasis,
	legacyBasis OwnershipBasis,
) (PathCurrentness, VacantTarget, currentnessEmission, error) {
	if manifestOwned {
		classified := classifyManifestOwnedPath(
			observation,
			manifest,
			current,
			desired,
			manifestBasis,
		)
		return classified, VacantTarget{}, classificationEmitted, nil
	}
	if observation.kind == PathObservedMissing {
		if !desired {
			return PathCurrentness{}, VacantTarget{}, "", fmt.Errorf("unowned missing path %s has no desired output", observation.path)
		}
		target := VacantTarget{
			path:      observation.path,
			component: current.Component(),
			digest:    current.digest,
			mode:      current.mode,
		}
		return PathCurrentness{}, target, vacancyEmitted, nil
	}
	legacyExact := knownLegacy && observation.digest == legacy.Digest
	if legacyExact {
		classified := PathCurrentness{
			path:           observation.path,
			component:      observation.Component(),
			kind:           PathKnownLegacyExact,
			observedDigest: observation.digest,
			desiredDigest:  desiredDigest(current, desired),
			observedMode:   observation.mode,
			desiredMode:    desiredMode(current, desired),
			basis:          legacyBasis,
		}
		return classified, VacantTarget{}, classificationEmitted, nil
	}
	classified := PathCurrentness{
		path:           observation.path,
		component:      observation.Component(),
		kind:           PathForeign,
		observedDigest: observation.digest,
		desiredDigest:  desiredDigest(current, desired),
		observedMode:   observation.mode,
		desiredMode:    desiredMode(current, desired),
	}
	return classified, VacantTarget{}, classificationEmitted, nil
}

func classifyManifestOwnedPath(
	observation PathObservation,
	manifest ManifestPath,
	current RenderedOutput,
	desired bool,
	manifestBasis OwnershipBasis,
) PathCurrentness {
	desiredValue := desiredDigest(current, desired)
	base := PathCurrentness{
		path:           observation.path,
		component:      manifest.Component,
		observedDigest: observation.digest,
		manifestDigest: manifest.Digest,
		desiredDigest:  desiredValue,
		observedMode:   observation.mode,
		manifestMode:   fs.FileMode(manifest.Mode),
		desiredMode:    desiredMode(current, desired),
		basis:          manifestBasis,
	}
	if observation.kind == PathObservedMissing {
		base.kind = PathMissingOwned
		return base
	}
	modeChanged := uint32(observation.mode.Perm()) != manifest.Mode
	if observation.digest != manifest.Digest || modeChanged {
		base.kind = PathLocallyModifiedOwned
		return base
	}
	if !desired {
		base.kind = PathOrphanedOwned
		return base
	}
	desiredModeChanged := uint32(current.mode.Perm()) != manifest.Mode
	if current.digest == manifest.Digest && !desiredModeChanged {
		base.kind = PathCurrentOwned
		return base
	}
	base.kind = PathOutdatedOwned
	return base
}

func desiredDigest(output RenderedOutput, present bool) string {
	if !present {
		return ""
	}
	return output.digest
}

func desiredMode(output RenderedOutput, present bool) fs.FileMode {
	if !present {
		return 0
	}
	return output.mode
}

func (currentness InstallationCurrentness) ManifestBasis() OwnershipBasis {
	return currentness.manifestBasis
}

func (currentness InstallationCurrentness) Baseline() InstallationBaselineKind {
	return currentness.baseline
}

func (currentness InstallationCurrentness) Manifest() InstallationManifest {
	return cloneInstallationManifest(currentness.manifest)
}

func (currentness InstallationCurrentness) Projection() HostAdapterProjection {
	return cloneHostAdapterProjection(currentness.projection)
}

func (currentness InstallationCurrentness) ManagedRoots() []string {
	return slices.Clone(currentness.managedRoots)
}

func (currentness InstallationCurrentness) Paths() []PathCurrentness {
	return slices.Clone(currentness.paths)
}

func (currentness InstallationCurrentness) VacantTargets() []VacantTarget {
	return slices.Clone(currentness.vacantTargets)
}
