package initfs

import (
	"fmt"
	"slices"
	"sort"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type HostStatusInspector struct {
	observer FileObserver
}

func NewHostStatusInspector(
	maxCarrierBytes int64,
) (HostStatusInspector, error) {
	observer, err := NewFileObserver(maxCarrierBytes)
	if err != nil {
		return HostStatusInspector{}, err
	}
	return HostStatusInspector{observer: observer}, nil
}

type HostBindingInspection struct {
	manifestPath     string
	manifestReadKind ManifestReadKind
	status           initplanning.HostInstallationStatus
}

func (inspector HostStatusInspector) InspectBinding(
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
) (HostBindingInspection, error) {
	if inspector.observer.maxFileBytes <= 0 {
		return HostBindingInspection{}, fmt.Errorf("host status inspector is invalid")
	}
	if err := store.valid(); err != nil {
		return HostBindingInspection{}, err
	}
	manifestRead, err := store.Read()
	if err != nil {
		return HostBindingInspection{}, err
	}
	currentness, err := inspector.observeBindingCurrentness(
		manifestRead,
		projection,
		legacySelection,
	)
	if err != nil {
		return HostBindingInspection{}, err
	}
	status, err := initplanning.ProjectHostInstallationStatus(currentness)
	if err != nil {
		return HostBindingInspection{}, err
	}
	return HostBindingInspection{
		manifestPath:     store.Path(),
		manifestReadKind: manifestRead.Kind(),
		status:           cloneHostInstallationStatus(status),
	}, nil
}

func (inspector HostStatusInspector) InspectCurrentness(
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
) (initplanning.InstallationCurrentness, error) {
	if inspector.observer.maxFileBytes <= 0 {
		return initplanning.InstallationCurrentness{},
			fmt.Errorf("host status inspector is invalid")
	}
	if err := store.valid(); err != nil {
		return initplanning.InstallationCurrentness{}, err
	}
	manifestRead, err := store.Read()
	if err != nil {
		return initplanning.InstallationCurrentness{}, err
	}
	return inspector.observeBindingCurrentness(
		manifestRead,
		projection,
		legacySelection,
	)
}

func (inspector HostStatusInspector) InspectCoherentBinding(
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (HostBindingInspection, error) {
	manifestRead, currentness, err :=
		inspector.inspectCoherentCurrentness(
			store,
			projection,
			legacySelection,
			managedLegacy,
		)
	if err != nil {
		return HostBindingInspection{}, err
	}
	status, err := initplanning.ProjectCoherentHostInstallationStatus(
		currentness,
	)
	if err != nil {
		return HostBindingInspection{}, err
	}
	return HostBindingInspection{
		manifestPath:     store.Path(),
		manifestReadKind: manifestRead.Kind(),
		status:           cloneHostInstallationStatus(status),
	}, nil
}

func (inspector HostStatusInspector) InspectCoherentCurrentness(
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (initplanning.CoherentInstallationCurrentness, error) {
	_, currentness, err := inspector.inspectCoherentCurrentness(
		store,
		projection,
		legacySelection,
		managedLegacy,
	)
	return currentness, err
}

func (inspector HostStatusInspector) inspectCoherentCurrentness(
	store ManifestStore,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (
	ManifestReadResult,
	initplanning.CoherentInstallationCurrentness,
	error,
) {
	if inspector.observer.maxFileBytes <= 0 {
		return ManifestReadResult{},
			initplanning.CoherentInstallationCurrentness{},
			fmt.Errorf(
				"host status inspector is invalid",
			)
	}
	if err := store.valid(); err != nil {
		return ManifestReadResult{},
			initplanning.CoherentInstallationCurrentness{},
			err
	}
	manifestRead, err := store.Read()
	if err != nil {
		return ManifestReadResult{},
			initplanning.CoherentInstallationCurrentness{},
			err
	}
	currentness, err := inspector.observeCoherentBindingCurrentness(
		manifestRead,
		projection,
		legacySelection,
		managedLegacy,
	)
	if err != nil {
		return ManifestReadResult{},
			initplanning.CoherentInstallationCurrentness{},
			err
	}
	return manifestRead, currentness, nil
}

func (inspector HostStatusInspector) observeCoherentBindingCurrentness(
	manifestRead ManifestReadResult,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (initplanning.CoherentInstallationCurrentness, error) {
	switch manifestRead.Kind() {
	case ManifestReadMissing:
		return inspector.observeFirstCoherentInstallation(
			projection,
			legacySelection,
			managedLegacy,
		)
	case ManifestReadPresent:
		return inspector.observeInstalledCoherentBinding(
			manifestRead.Manifest(),
			projection,
			legacySelection,
			managedLegacy,
		)
	default:
		return initplanning.CoherentInstallationCurrentness{}, fmt.Errorf(
			"manifest read result is invalid",
		)
	}
}

func (inspector HostStatusInspector) observeFirstCoherentInstallation(
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (initplanning.CoherentInstallationCurrentness, error) {
	wholePlan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		legacySelection,
	)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	wholeObservations, err := inspector.observeWholePlan(wholePlan)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	carrierPlans, err :=
		initplanning.BuildFirstCoherentManagedCarrierObservationPlans(
			projection,
			managedLegacy,
		)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	carrierInputs, err := inspector.observeManagedCarrierPlans(
		carrierPlans,
		projection.TargetRoots(),
	)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	return initplanning.ClassifyFirstCoherentInstallationCurrentness(
		projection,
		wholeObservations,
		carrierInputs,
		legacySelection,
		managedLegacy,
	)
}

func (inspector HostStatusInspector) observeInstalledCoherentBinding(
	manifest initplanning.InstallationManifest,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
	managedLegacy initplanning.ManagedFragmentLegacyRegistry,
) (initplanning.CoherentInstallationCurrentness, error) {
	wholePlan, err := initplanning.BuildInstalledObservationPlan(
		manifest,
		projection,
		legacySelection,
	)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	wholeObservations, err := inspector.observeWholePlan(wholePlan)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	carrierPlans, err :=
		initplanning.BuildInstalledCoherentManagedCarrierObservationPlans(
			manifest,
			projection,
			managedLegacy,
		)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	carrierInputs, err := inspector.observeManagedCarrierPlans(
		carrierPlans,
		wholePlan.ManagedRoots(),
	)
	if err != nil {
		return initplanning.CoherentInstallationCurrentness{}, err
	}
	return initplanning.ClassifyCoherentInstallationCurrentness(
		manifest,
		projection,
		wholeObservations,
		carrierInputs,
		legacySelection,
		managedLegacy,
	)
}

func (inspector HostStatusInspector) observeWholePlan(
	plan initplanning.InstallationObservationPlan,
) ([]initplanning.PathObservation, error) {
	if len(plan.Targets()) == 0 {
		return []initplanning.PathObservation{}, nil
	}
	return inspector.observer.Observe(plan)
}

func (inspector HostStatusInspector) observeManagedCarrierPlans(
	plans []initplanning.ManagedFragmentObservationPlan,
	managedRoots []string,
) ([]initplanning.ManagedCarrierInput, error) {
	inputs := make(
		[]initplanning.ManagedCarrierInput,
		0,
		len(plans),
	)
	for _, plan := range plans {
		input, err := inspector.observer.ObserveManagedCarrier(
			plan,
			managedRoots,
		)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func (inspector HostStatusInspector) observeBindingCurrentness(
	manifestRead ManifestReadResult,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
) (initplanning.InstallationCurrentness, error) {
	switch manifestRead.Kind() {
	case ManifestReadMissing:
		return inspector.observeFirstInstallation(
			projection,
			legacySelection,
		)
	case ManifestReadPresent:
		return inspector.observeInstalledBinding(
			manifestRead.Manifest(),
			projection,
			legacySelection,
		)
	default:
		return initplanning.InstallationCurrentness{}, fmt.Errorf(
			"manifest read result is invalid",
		)
	}
}

func (inspector HostStatusInspector) observeFirstInstallation(
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
) (initplanning.InstallationCurrentness, error) {
	plan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		legacySelection,
	)
	if err != nil {
		return initplanning.InstallationCurrentness{}, err
	}
	observations, err := inspector.observer.Observe(plan)
	if err != nil {
		return initplanning.InstallationCurrentness{}, err
	}
	return initplanning.ClassifyFirstInstallationCurrentness(
		projection,
		observations,
		legacySelection,
	)
}

func (inspector HostStatusInspector) observeInstalledBinding(
	manifest initplanning.InstallationManifest,
	projection initplanning.HostAdapterProjection,
	legacySelection initplanning.LegacyRegistrySelection,
) (initplanning.InstallationCurrentness, error) {
	plan, err := initplanning.BuildInstalledObservationPlan(
		manifest,
		projection,
		legacySelection,
	)
	if err != nil {
		return initplanning.InstallationCurrentness{}, err
	}
	observations, err := inspector.observer.Observe(plan)
	if err != nil {
		return initplanning.InstallationCurrentness{}, err
	}
	return initplanning.ClassifyInstallationCurrentness(
		manifest,
		projection,
		observations,
		legacySelection,
	)
}

func (inspection HostBindingInspection) ManifestPath() string {
	return inspection.manifestPath
}

func (inspection HostBindingInspection) ManifestReadKind() ManifestReadKind {
	return inspection.manifestReadKind
}

func (inspection HostBindingInspection) Status() initplanning.HostInstallationStatus {
	return cloneHostInstallationStatus(inspection.status)
}

func cloneHostInstallationStatus(
	status initplanning.HostInstallationStatus,
) initplanning.HostInstallationStatus {
	status.Reasons = slices.Clone(status.Reasons)
	status.InstalledComponents = slices.Clone(status.InstalledComponents)
	status.DesiredComponents = slices.Clone(status.DesiredComponents)
	status.InstalledTargetRoots = slices.Clone(status.InstalledTargetRoots)
	status.DesiredTargetRoots = slices.Clone(status.DesiredTargetRoots)
	status.Paths = slices.Clone(status.Paths)
	status.VacantTargets = slices.Clone(status.VacantTargets)
	status.ManagedFragments = slices.Clone(status.ManagedFragments)
	status.VacantManagedFragments = slices.Clone(
		status.VacantManagedFragments,
	)
	status.ReconcileArgv = slices.Clone(status.ReconcileArgv)
	return status
}

type SkillRootStatus struct {
	observations []initplanning.SkillRootObservation
	activeRoots  []initplanning.ActiveSkillRoot
	duplicates   []initplanning.DuplicateSkillRoot
}

func (inspector HostStatusInspector) InspectSkillRoots(
	manifestOwnedRoots []initplanning.ActiveSkillRoot,
	plans []initplanning.SkillRootObservationPlan,
) (SkillRootStatus, error) {
	if inspector.observer.maxFileBytes <= 0 {
		return SkillRootStatus{}, fmt.Errorf("host status inspector is invalid")
	}
	activeRoots := slices.Clone(manifestOwnedRoots)
	if err := requireManifestOwnedRoots(activeRoots); err != nil {
		return SkillRootStatus{}, err
	}
	observations := make([]initplanning.SkillRootObservation, 0, len(plans))
	seenPlans := make(map[string]struct{}, len(plans))
	for _, plan := range plans {
		key := skillRootPlanKey(plan)
		if _, duplicate := seenPlans[key]; duplicate {
			return SkillRootStatus{}, fmt.Errorf(
				"skill-root status repeats observation plan %s",
				key,
			)
		}
		seenPlans[key] = struct{}{}
		filesystemObservations, err := inspector.observer.Observe(
			plan.FilesystemPlan(),
		)
		if err != nil {
			return SkillRootStatus{}, err
		}
		observation, err := initplanning.ProjectSkillRootObservation(
			plan,
			filesystemObservations,
		)
		if err != nil {
			return SkillRootStatus{}, err
		}
		observations = append(observations, observation)
		activeRoot, active := observation.ActiveRoot()
		if active {
			activeRoots = append(activeRoots, activeRoot)
		}
	}
	sortSkillRootObservations(observations)
	sortActiveSkillRoots(activeRoots)
	duplicates, err := initplanning.FindDuplicateSkillRoots(activeRoots)
	if err != nil {
		return SkillRootStatus{}, err
	}
	return SkillRootStatus{
		observations: slices.Clone(observations),
		activeRoots:  slices.Clone(activeRoots),
		duplicates:   slices.Clone(duplicates),
	}, nil
}

func requireManifestOwnedRoots(
	roots []initplanning.ActiveSkillRoot,
) error {
	for _, root := range roots {
		if root.Origin() != initplanning.SkillRootManifestOwned {
			return fmt.Errorf(
				"preclassified skill root %s lacks manifest ownership",
				root.Root(),
			)
		}
	}
	return nil
}

func skillRootPlanKey(
	plan initplanning.SkillRootObservationPlan,
) string {
	return plan.Root() + "\x00" + string(plan.Host()) + "\x00" + string(plan.Scope())
}

func sortSkillRootObservations(
	observations []initplanning.SkillRootObservation,
) {
	sort.Slice(observations, func(left int, right int) bool {
		leftKey := observations[left].Root() + "\x00" +
			string(observations[left].Host()) + "\x00" +
			string(observations[left].Scope())
		rightKey := observations[right].Root() + "\x00" +
			string(observations[right].Host()) + "\x00" +
			string(observations[right].Scope())
		return leftKey < rightKey
	})
}

func sortActiveSkillRoots(
	roots []initplanning.ActiveSkillRoot,
) {
	sort.Slice(roots, func(left int, right int) bool {
		leftKey := roots[left].Root() + "\x00" +
			string(roots[left].Host()) + "\x00" +
			string(roots[left].Scope()) + "\x00" +
			string(roots[left].Origin())
		rightKey := roots[right].Root() + "\x00" +
			string(roots[right].Host()) + "\x00" +
			string(roots[right].Scope()) + "\x00" +
			string(roots[right].Origin())
		return leftKey < rightKey
	})
}

func (status SkillRootStatus) Observations() []initplanning.SkillRootObservation {
	return slices.Clone(status.observations)
}

func (status SkillRootStatus) ActiveRoots() []initplanning.ActiveSkillRoot {
	return slices.Clone(status.activeRoots)
}

func (status SkillRootStatus) Duplicates() []initplanning.DuplicateSkillRoot {
	return slices.Clone(status.duplicates)
}
