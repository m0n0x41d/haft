package initplanning

import (
	"fmt"
	"slices"
	"sort"
)

type HostInstallationPosture string

const (
	HostInstallationCurrent           HostInstallationPosture = "current"
	HostInstallationReconcileRequired HostInstallationPosture = "reconcile_required"
	HostInstallationBlocked           HostInstallationPosture = "blocked_preserving_files"
)

type InstallationManifestPresence string

const (
	InstallationManifestMissing InstallationManifestPresence = "missing"
	InstallationManifestPresent InstallationManifestPresence = "present"
)

type PublicationStatus struct {
	HaftVersion         string
	ExecutablePath      string
	ExecutableDigest    string
	SkillBundleDigest   string
	KernelCatalogDigest string
}

type PathCurrentnessStatus struct {
	Path            string
	Component       Component
	State           PathCurrentnessKind
	ObservedDigest  string
	ObservedMode    uint32
	ManifestDigest  string
	ManifestMode    uint32
	DesiredDigest   string
	DesiredMode     uint32
	OwnershipKind   OwnershipBasisKind
	OwnershipRef    string
	OwnershipDigest string
}

type VacantTargetStatus struct {
	Path          string
	Component     Component
	DesiredDigest string
	DesiredMode   uint32
}

type ManagedFragmentCurrentnessStatus struct {
	CarrierPath     string
	Component       Component
	Kind            ManagedFragmentKind
	Selector        string
	MemberID        string
	MergeEdition    string
	State           ManagedFragmentCurrentnessKind
	ObservedDigest  string
	ManifestDigest  string
	DesiredDigest   string
	OwnershipKind   OwnershipBasisKind
	OwnershipRef    string
	OwnershipDigest string
}

type VacantManagedFragmentStatus struct {
	CarrierPath   string
	Component     Component
	Kind          ManagedFragmentKind
	Selector      string
	MemberID      string
	MergeEdition  string
	DesiredDigest string
}

type HostInstallationStatus struct {
	Posture                 HostInstallationPosture
	Reasons                 []string
	ManifestPresence        InstallationManifestPresence
	ProjectRoot             string
	ProjectID               string
	Host                    HostID
	Scope                   InstallScope
	ManifestRef             string
	ManifestDigest          string
	InstalledAdapterEdition string
	DesiredAdapterEdition   string
	InstalledComponents     []Component
	DesiredComponents       []Component
	InstalledTargetRoots    []string
	DesiredTargetRoots      []string
	InstalledPublication    PublicationStatus
	DesiredPublication      PublicationStatus
	Paths                   []PathCurrentnessStatus
	VacantTargets           []VacantTargetStatus
	ManagedFragments        []ManagedFragmentCurrentnessStatus
	VacantManagedFragments  []VacantManagedFragmentStatus
	ReconcileArgv           []string
}

var managedFragmentPostureByKind = map[ManagedFragmentCurrentnessKind]HostInstallationPosture{
	ManagedFragmentCurrentOwned:         HostInstallationCurrent,
	ManagedFragmentOutdatedOwned:        HostInstallationReconcileRequired,
	ManagedFragmentLocallyModifiedOwned: HostInstallationBlocked,
	ManagedFragmentKnownLegacyExact:     HostInstallationReconcileRequired,
	ManagedFragmentForeign:              HostInstallationReconcileRequired,
	ManagedFragmentOrphanedOwned:        HostInstallationReconcileRequired,
	ManagedFragmentMissingOwned:         HostInstallationReconcileRequired,
}

var hostInstallationPostureRank = map[HostInstallationPosture]int{
	HostInstallationCurrent:           0,
	HostInstallationReconcileRequired: 1,
	HostInstallationBlocked:           2,
}

func ProjectCoherentHostInstallationStatus(
	currentness CoherentInstallationCurrentness,
) (HostInstallationStatus, error) {
	status, err := ProjectHostInstallationStatus(currentness.whole)
	if err != nil {
		return HostInstallationStatus{}, err
	}
	managed, vacant, posture, reasons, err :=
		projectManagedFragmentStatuses(currentness.carriers)
	if err != nil {
		return HostInstallationStatus{}, err
	}
	status.ManagedFragments = managed
	status.VacantManagedFragments = vacant
	status.Posture = mergeHostInstallationPosture(
		status.Posture,
		posture,
	)
	status.Reasons = canonicalReasons(
		append(status.Reasons, reasons...),
	)
	return status, nil
}

func projectManagedFragmentStatuses(
	carriers []ManagedCarrierCurrentness,
) (
	[]ManagedFragmentCurrentnessStatus,
	[]VacantManagedFragmentStatus,
	HostInstallationPosture,
	[]string,
	error,
) {
	managed := make([]ManagedFragmentCurrentnessStatus, 0)
	vacant := make([]VacantManagedFragmentStatus, 0)
	reasons := make([]string, 0)
	posture := HostInstallationCurrent
	for _, carrier := range carriers {
		for _, state := range carrier.States() {
			projected, statePosture, err :=
				projectManagedFragmentCurrentness(state)
			if err != nil {
				return nil, nil, "", nil, err
			}
			managed = append(managed, projected)
			posture = mergeHostInstallationPosture(
				posture,
				statePosture,
			)
			if state.Kind() != ManagedFragmentCurrentOwned {
				reasons = append(
					reasons,
					"fragment_"+string(state.Kind()),
				)
			}
		}
		for _, target := range carrier.VacantTargets() {
			desired := target.Desired()
			coordinate := target.Coordinate()
			vacant = append(vacant, VacantManagedFragmentStatus{
				CarrierPath:   coordinate.CarrierPath(),
				Component:     desired.Component(),
				Kind:          coordinate.Kind(),
				Selector:      coordinate.Selector(),
				MemberID:      coordinate.MemberID(),
				MergeEdition:  coordinate.MergeEdition(),
				DesiredDigest: desired.Digest(),
			})
			posture = mergeHostInstallationPosture(
				posture,
				HostInstallationReconcileRequired,
			)
			reasons = append(reasons, "vacant_desired_fragment")
		}
	}
	return managed, vacant, posture, reasons, nil
}

func projectManagedFragmentCurrentness(
	currentness ManagedFragmentCurrentness,
) (
	ManagedFragmentCurrentnessStatus,
	HostInstallationPosture,
	error,
) {
	posture, known := managedFragmentPostureByKind[currentness.Kind()]
	if !known {
		return ManagedFragmentCurrentnessStatus{}, "", fmt.Errorf(
			"managed fragment currentness kind is invalid",
		)
	}
	if currentness.Kind() == ManagedFragmentForeign &&
		currentness.DesiredDigest() != "" {
		posture = HostInstallationBlocked
	}
	coordinate := currentness.Coordinate()
	basis := currentness.OwnershipBasis()
	return ManagedFragmentCurrentnessStatus{
		CarrierPath:     coordinate.CarrierPath(),
		Component:       currentness.Component(),
		Kind:            coordinate.Kind(),
		Selector:        coordinate.Selector(),
		MemberID:        coordinate.MemberID(),
		MergeEdition:    coordinate.MergeEdition(),
		State:           currentness.Kind(),
		ObservedDigest:  currentness.ObservedDigest(),
		ManifestDigest:  currentness.ManifestDigest(),
		DesiredDigest:   currentness.DesiredDigest(),
		OwnershipKind:   basis.Kind(),
		OwnershipRef:    basis.Ref(),
		OwnershipDigest: basis.Digest(),
	}, posture, nil
}

func mergeHostInstallationPosture(
	left HostInstallationPosture,
	right HostInstallationPosture,
) HostInstallationPosture {
	if hostInstallationPostureRank[right] > hostInstallationPostureRank[left] {
		return right
	}
	return left
}

func ProjectHostInstallationStatus(
	currentness InstallationCurrentness,
) (HostInstallationStatus, error) {
	if currentness.baseline == InstallationBaselineNoPriorManifest {
		return projectFirstInstallationStatus(currentness)
	}
	if currentness.baseline != InstallationBaselineManifest {
		return HostInstallationStatus{}, fmt.Errorf("installation currentness baseline is invalid")
	}
	manifest := currentness.manifest
	projection := currentness.projection
	if !currentness.manifestBasis.valid() || !projection.publication.valid() {
		return HostInstallationStatus{}, fmt.Errorf("installation currentness is invalid")
	}
	reasons := metadataDriftReasons(manifest, projection)
	paths := make([]PathCurrentnessStatus, len(currentness.paths))
	posture := HostInstallationCurrent
	if len(reasons) > 0 || len(currentness.vacantTargets) > 0 {
		posture = HostInstallationReconcileRequired
	}
	for index, path := range currentness.paths {
		basis := path.basis
		paths[index] = PathCurrentnessStatus{
			Path:            path.path,
			Component:       path.component,
			State:           path.kind,
			ObservedDigest:  path.observedDigest,
			ObservedMode:    uint32(path.observedMode.Perm()),
			ManifestDigest:  path.manifestDigest,
			ManifestMode:    uint32(path.manifestMode.Perm()),
			DesiredDigest:   path.desiredDigest,
			DesiredMode:     uint32(path.desiredMode.Perm()),
			OwnershipKind:   basis.kind,
			OwnershipRef:    basis.ref,
			OwnershipDigest: basis.digest,
		}
		if path.kind != PathCurrentOwned {
			if posture == HostInstallationCurrent {
				posture = HostInstallationReconcileRequired
			}
			reasons = append(reasons, "path_"+string(path.kind))
		}
		foreignCollision := path.kind == PathForeign && path.desiredDigest != ""
		if path.kind == PathLocallyModifiedOwned || foreignCollision {
			posture = HostInstallationBlocked
		}
	}
	vacant := make([]VacantTargetStatus, len(currentness.vacantTargets))
	for index, target := range currentness.vacantTargets {
		vacant[index] = VacantTargetStatus{
			Path:          target.path,
			Component:     target.component,
			DesiredDigest: target.digest,
			DesiredMode:   uint32(target.mode.Perm()),
		}
	}
	if len(vacant) > 0 {
		reasons = append(reasons, "vacant_desired_path")
	}
	reasons = canonicalReasons(reasons)
	return HostInstallationStatus{
		Posture:                 posture,
		Reasons:                 reasons,
		ManifestPresence:        InstallationManifestPresent,
		ProjectRoot:             manifest.wire.ProjectRoot,
		ProjectID:               manifest.wire.ProjectID,
		Host:                    manifest.wire.Host,
		Scope:                   manifest.wire.InstallScope,
		ManifestRef:             manifest.Ref(),
		ManifestDigest:          manifest.Digest(),
		InstalledAdapterEdition: manifest.wire.AdapterEdition,
		DesiredAdapterEdition:   projection.edition,
		InstalledComponents:     slices.Clone(manifest.wire.Components),
		DesiredComponents:       projection.components.Values(),
		InstalledTargetRoots:    slices.Clone(manifest.wire.TargetRoots),
		DesiredTargetRoots:      slices.Clone(projection.targetRoots),
		InstalledPublication:    manifestPublicationStatus(manifest),
		DesiredPublication:      publicationStatus(projection.publication),
		Paths:                   paths,
		VacantTargets:           vacant,
		ReconcileArgv:           projection.recovery.Argv(),
	}, nil
}

func projectFirstInstallationStatus(
	currentness InstallationCurrentness,
) (HostInstallationStatus, error) {
	projection := currentness.projection
	if currentness.manifestBasis.valid() ||
		len(currentness.manifest.canonical) != 0 ||
		!projection.publication.valid() {
		return HostInstallationStatus{}, fmt.Errorf("first-install currentness is invalid")
	}
	paths := make([]PathCurrentnessStatus, len(currentness.paths))
	posture := HostInstallationReconcileRequired
	reasons := []string{"installation_manifest_missing"}
	for index, path := range currentness.paths {
		if path.kind != PathKnownLegacyExact && path.kind != PathForeign {
			return HostInstallationStatus{}, fmt.Errorf("first-install path %s has invalid state %s", path.path, path.kind)
		}
		basis := path.basis
		paths[index] = PathCurrentnessStatus{
			Path:            path.path,
			Component:       path.component,
			State:           path.kind,
			ObservedDigest:  path.observedDigest,
			ObservedMode:    uint32(path.observedMode.Perm()),
			DesiredDigest:   path.desiredDigest,
			DesiredMode:     uint32(path.desiredMode.Perm()),
			OwnershipKind:   basis.kind,
			OwnershipRef:    basis.ref,
			OwnershipDigest: basis.digest,
		}
		reasons = append(reasons, "path_"+string(path.kind))
		if path.kind == PathForeign && path.desiredDigest != "" {
			posture = HostInstallationBlocked
		}
	}
	vacant := make([]VacantTargetStatus, len(currentness.vacantTargets))
	for index, target := range currentness.vacantTargets {
		vacant[index] = VacantTargetStatus{
			Path:          target.path,
			Component:     target.component,
			DesiredDigest: target.digest,
			DesiredMode:   uint32(target.mode.Perm()),
		}
	}
	if len(vacant) > 0 {
		reasons = append(reasons, "vacant_desired_path")
	}
	return HostInstallationStatus{
		Posture:               posture,
		Reasons:               canonicalReasons(reasons),
		ManifestPresence:      InstallationManifestMissing,
		ProjectRoot:           projection.projectRoot,
		ProjectID:             projection.projectID.String(),
		Host:                  projection.host,
		Scope:                 projection.scope,
		DesiredAdapterEdition: projection.edition,
		DesiredComponents:     projection.components.Values(),
		DesiredTargetRoots:    slices.Clone(projection.targetRoots),
		DesiredPublication:    publicationStatus(projection.publication),
		Paths:                 paths,
		VacantTargets:         vacant,
		ReconcileArgv:         projection.recovery.Argv(),
	}, nil
}

func metadataDriftReasons(
	manifest InstallationManifest,
	projection HostAdapterProjection,
) []string {
	reasons := make([]string, 0, 9)
	if manifest.wire.AdapterEdition != projection.edition {
		reasons = append(reasons, "adapter_edition_changed")
	}
	publication := projection.publication
	if manifest.wire.HaftVersion != publication.haftVersion {
		reasons = append(reasons, "haft_version_changed")
	}
	if manifest.wire.ExecutablePath != publication.executablePath {
		reasons = append(reasons, "executable_path_changed")
	}
	if manifest.wire.ExecutableDigest != publication.executableDigest {
		reasons = append(reasons, "executable_digest_changed")
	}
	if manifest.wire.SkillBundleDigest != publication.skillBundleDigest {
		reasons = append(reasons, "skill_bundle_changed")
	}
	if manifest.wire.KernelCatalogDigest != publication.kernelCatalogDigest {
		reasons = append(reasons, "kernel_catalog_changed")
	}
	if !slices.Equal(manifest.wire.Components, projection.components.values) {
		reasons = append(reasons, "component_selection_changed")
	}
	if !slices.Equal(manifest.wire.TargetRoots, projection.targetRoots) {
		reasons = append(reasons, "target_roots_changed")
	}
	return reasons
}

func canonicalReasons(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	for _, reason := range raw {
		seen[reason] = struct{}{}
	}
	reasons := make([]string, 0, len(seen))
	for reason := range seen {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons
}

func manifestPublicationStatus(
	manifest InstallationManifest,
) PublicationStatus {
	return PublicationStatus{
		HaftVersion:         manifest.wire.HaftVersion,
		ExecutablePath:      manifest.wire.ExecutablePath,
		ExecutableDigest:    manifest.wire.ExecutableDigest,
		SkillBundleDigest:   manifest.wire.SkillBundleDigest,
		KernelCatalogDigest: manifest.wire.KernelCatalogDigest,
	}
}

func publicationStatus(identity PublicationIdentity) PublicationStatus {
	return PublicationStatus{
		HaftVersion:         identity.haftVersion,
		ExecutablePath:      identity.executablePath,
		ExecutableDigest:    identity.executableDigest,
		SkillBundleDigest:   identity.skillBundleDigest,
		KernelCatalogDigest: identity.kernelCatalogDigest,
	}
}
