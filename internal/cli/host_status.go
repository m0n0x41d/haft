package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/spf13/cobra"
)

const (
	hostStatusSchema          = "haft.host-status/v1"
	hostStatusMaxCarrierBytes = int64(4 << 20)
)

var hostStatusJSON bool

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Inspect versioned host-adapter installations",
}

var hostStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Read host manifests, carrier currentness, and duplicate skill roots",
	Long: `Inspect the current project ledger, every canonical host-installation
manifest location, and known skill discovery roots.

This command is read-only. Filesystem presence is reported as discovery
evidence, not Haft ownership. Only a valid installation manifest can establish
an owned adapter binding. The command never initializes, migrates, installs,
reconciles, overwrites, removes, or rebinds anything.`,
	Args: cobra.NoArgs,
	RunE: runHostStatus,
}

func init() {
	hostStatusCmd.Flags().BoolVar(
		&hostStatusJSON,
		"json",
		false,
		"Emit the complete read-only status as JSON",
	)
	hostCmd.AddCommand(hostStatusCmd)
	rootCmd.AddCommand(hostCmd)
}

type hostStatusReport struct {
	Schema                    string                     `json:"schema"`
	Project                   hostStatusProject          `json:"project"`
	ManifestLocationsChecked  int                        `json:"manifest_locations_checked"`
	Manifests                 []hostManifestStatus       `json:"manifests"`
	SkillRootObservations     []hostSkillRootObservation `json:"skill_root_observations"`
	ActiveSkillRoots          []hostActiveSkillRoot      `json:"active_skill_roots"`
	DuplicateSkillRoots       []hostDuplicateSkillRoot   `json:"duplicate_skill_roots"`
	SkillRootInspectionIssues []hostSkillRootIssue       `json:"skill_root_inspection_issues"`
	FilesystemPresenceMeaning string                     `json:"filesystem_presence_meaning"`
	MutationsPerformed        bool                       `json:"mutations_performed"`
}

type hostStatusProject struct {
	Root          string `json:"root"`
	ProjectID     string `json:"project_id"`
	DatabasePath  string `json:"database_path"`
	SchemaVersion int    `json:"schema_version"`
	CorePosture   string `json:"core_posture"`
}

type hostManifestStatus struct {
	Path           string                    `json:"path"`
	ReadPosture    string                    `json:"read_posture"`
	Host           initplanning.HostID       `json:"host,omitempty"`
	Scope          initplanning.InstallScope `json:"scope,omitempty"`
	ManifestRef    string                    `json:"manifest_ref,omitempty"`
	ManifestDigest string                    `json:"manifest_digest,omitempty"`
	BindingPosture string                    `json:"binding_posture"`
	Reasons        []string                  `json:"reasons,omitempty"`
	Currentness    *hostInstallationStatus   `json:"currentness,omitempty"`
}

type hostInstallationStatus struct {
	Posture                 initplanning.HostInstallationPosture      `json:"posture"`
	Reasons                 []string                                  `json:"reasons"`
	ManifestPresence        initplanning.InstallationManifestPresence `json:"manifest_presence"`
	ProjectRoot             string                                    `json:"project_root"`
	ProjectID               string                                    `json:"project_id"`
	Host                    initplanning.HostID                       `json:"host"`
	Scope                   initplanning.InstallScope                 `json:"scope"`
	ManifestRef             string                                    `json:"manifest_ref"`
	ManifestDigest          string                                    `json:"manifest_digest"`
	InstalledAdapterEdition string                                    `json:"installed_adapter_edition"`
	DesiredAdapterEdition   string                                    `json:"desired_adapter_edition"`
	InstalledComponents     []initplanning.Component                  `json:"installed_components"`
	DesiredComponents       []initplanning.Component                  `json:"desired_components"`
	InstalledTargetRoots    []string                                  `json:"installed_target_roots"`
	DesiredTargetRoots      []string                                  `json:"desired_target_roots"`
	InstalledPublication    hostPublicationStatus                     `json:"installed_publication"`
	DesiredPublication      hostPublicationStatus                     `json:"desired_publication"`
	Paths                   []hostPathCurrentness                     `json:"paths"`
	VacantTargets           []hostVacantTargetStatus                  `json:"vacant_targets"`
	ManagedFragments        []hostManagedFragmentCurrentness          `json:"managed_fragments,omitempty"`
	VacantManagedFragments  []hostVacantManagedFragmentStatus         `json:"vacant_managed_fragments,omitempty"`
	ReconcileSurface        string                                    `json:"reconcile_surface"`
}

type hostPublicationStatus struct {
	HaftVersion         string `json:"haft_version"`
	ExecutablePath      string `json:"executable_path"`
	ExecutableDigest    string `json:"executable_digest"`
	SkillBundleDigest   string `json:"skill_bundle_digest"`
	KernelCatalogDigest string `json:"kernel_catalog_digest"`
}

type hostPathCurrentness struct {
	Path            string                           `json:"path"`
	Component       initplanning.Component           `json:"component"`
	State           initplanning.PathCurrentnessKind `json:"state"`
	ObservedDigest  string                           `json:"observed_digest,omitempty"`
	ObservedMode    uint32                           `json:"observed_mode,omitempty"`
	ManifestDigest  string                           `json:"manifest_digest,omitempty"`
	ManifestMode    uint32                           `json:"manifest_mode,omitempty"`
	DesiredDigest   string                           `json:"desired_digest,omitempty"`
	DesiredMode     uint32                           `json:"desired_mode,omitempty"`
	OwnershipKind   initplanning.OwnershipBasisKind  `json:"ownership_kind,omitempty"`
	OwnershipRef    string                           `json:"ownership_ref,omitempty"`
	OwnershipDigest string                           `json:"ownership_digest,omitempty"`
}

type hostVacantTargetStatus struct {
	Path          string                 `json:"path"`
	Component     initplanning.Component `json:"component"`
	DesiredDigest string                 `json:"desired_digest"`
	DesiredMode   uint32                 `json:"desired_mode"`
}

type hostManagedFragmentCurrentness struct {
	CarrierPath     string                                      `json:"carrier_path"`
	Component       initplanning.Component                      `json:"component"`
	Kind            initplanning.ManagedFragmentKind            `json:"kind"`
	Selector        string                                      `json:"selector"`
	MemberID        string                                      `json:"member_id,omitempty"`
	MergeEdition    string                                      `json:"merge_edition"`
	State           initplanning.ManagedFragmentCurrentnessKind `json:"state"`
	ObservedDigest  string                                      `json:"observed_digest,omitempty"`
	ManifestDigest  string                                      `json:"manifest_digest,omitempty"`
	DesiredDigest   string                                      `json:"desired_digest,omitempty"`
	OwnershipKind   initplanning.OwnershipBasisKind             `json:"ownership_kind,omitempty"`
	OwnershipRef    string                                      `json:"ownership_ref,omitempty"`
	OwnershipDigest string                                      `json:"ownership_digest,omitempty"`
}

type hostVacantManagedFragmentStatus struct {
	CarrierPath   string                           `json:"carrier_path"`
	Component     initplanning.Component           `json:"component"`
	Kind          initplanning.ManagedFragmentKind `json:"kind"`
	Selector      string                           `json:"selector"`
	MemberID      string                           `json:"member_id,omitempty"`
	MergeEdition  string                           `json:"merge_edition"`
	DesiredDigest string                           `json:"desired_digest"`
}

type hostSkillRootObservation struct {
	Root           string                    `json:"root"`
	Host           initplanning.HostID       `json:"host"`
	Scope          initplanning.InstallScope `json:"scope"`
	EvidenceRef    string                    `json:"evidence_ref"`
	EvidenceDigest string                    `json:"evidence_digest"`
	ExpectedCount  int                       `json:"expected_count"`
	ObservedCount  int                       `json:"observed_count"`
}

type hostActiveSkillRoot struct {
	Root           string                       `json:"root"`
	Host           initplanning.HostID          `json:"host"`
	Scope          initplanning.InstallScope    `json:"scope"`
	Origin         initplanning.SkillRootOrigin `json:"origin"`
	EvidenceRef    string                       `json:"evidence_ref"`
	EvidenceDigest string                       `json:"evidence_digest"`
	SkillNames     []string                     `json:"skill_names"`
}

type hostDuplicateSkillRoot struct {
	SkillName           string                       `json:"skill_name"`
	LeftRoot            string                       `json:"left_root"`
	LeftHost            initplanning.HostID          `json:"left_host"`
	LeftScope           initplanning.InstallScope    `json:"left_scope"`
	LeftOrigin          initplanning.SkillRootOrigin `json:"left_origin"`
	LeftEvidenceRef     string                       `json:"left_evidence_ref"`
	LeftEvidenceDigest  string                       `json:"left_evidence_digest"`
	RightRoot           string                       `json:"right_root"`
	RightHost           initplanning.HostID          `json:"right_host"`
	RightScope          initplanning.InstallScope    `json:"right_scope"`
	RightOrigin         initplanning.SkillRootOrigin `json:"right_origin"`
	RightEvidenceRef    string                       `json:"right_evidence_ref"`
	RightEvidenceDigest string                       `json:"right_evidence_digest"`
}

type hostSkillRootIssue struct {
	Root   string                    `json:"root"`
	Host   initplanning.HostID       `json:"host"`
	Scope  initplanning.InstallScope `json:"scope"`
	Reason string                    `json:"reason"`
}

func runHostStatus(cmd *cobra.Command, _ []string) error {
	binding, err := resolveProjectBinding()
	if err != nil {
		return projectBindingError(binding, err)
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		return err
	}
	report, err := buildHostStatusReport(
		cmd.Context(),
		binding,
		runtime,
	)
	if err != nil {
		return err
	}
	if hostStatusJSON {
		return writeHostStatusJSON(cmd.OutOrStdout(), report)
	}
	return writeHostStatusText(cmd.OutOrStdout(), report)
}

func buildHostStatusReport(
	ctx context.Context,
	binding ProjectBinding,
	runtime currentHostPublicationRuntime,
) (hostStatusReport, error) {
	projectRoot, err := filepath.EvalSymlinks(binding.ProjectRoot)
	if err != nil {
		return hostStatusReport{}, fmt.Errorf(
			"resolve physical project root: %w",
			err,
		)
	}
	projectRoot = filepath.Clean(projectRoot)
	ledger, err := openCurrentProjectLedger(
		ctx,
		projectRoot,
		projectledger.ReadOnly,
		"read-only host status",
	)
	if err != nil {
		return hostStatusReport{}, err
	}
	schemaVersion, schemaErr := db.CurrentSchemaVersion()
	closeErr := ledger.Close()
	if err := errors.Join(schemaErr, closeErr); err != nil {
		return hostStatusReport{}, err
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		return hostStatusReport{}, err
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		return hostStatusReport{}, err
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		return hostStatusReport{}, err
	}
	inspector, err := initfs.NewHostStatusInspector(
		hostStatusMaxCarrierBytes,
	)
	if err != nil {
		return hostStatusReport{}, err
	}
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projectRoot,
			ProjectID:    binding.ProjectID,
			UserHomeRoot: runtime.userHomeRoot,
		},
	)
	if err != nil {
		return hostStatusReport{}, err
	}
	manifests, manifestRoots, checked, err := inspectHostManifests(
		layout,
		projectRoot,
		binding.ProjectID,
		candidates,
		bundle,
		publication,
		runtime,
		inspector,
	)
	if err != nil {
		return hostStatusReport{}, err
	}
	observations, discoveredRoots, issues := inspectCurrentSkillRoots(
		candidates,
		inspector,
	)
	activeRoots := mergeManifestAndDiscoveredSkillRoots(
		manifestRoots,
		discoveredRoots,
	)
	sortActiveHostSkillRoots(activeRoots)
	duplicates, err := initplanning.FindDuplicateSkillRoots(activeRoots)
	if err != nil {
		return hostStatusReport{}, err
	}
	return hostStatusReport{
		Schema: hostStatusSchema,
		Project: hostStatusProject{
			Root:          projectRoot,
			ProjectID:     binding.ProjectID,
			DatabasePath:  binding.DBPath,
			SchemaVersion: schemaVersion,
			CorePosture:   "current_read_only",
		},
		ManifestLocationsChecked:  checked,
		Manifests:                 manifests,
		SkillRootObservations:     projectSkillRootObservations(observations),
		ActiveSkillRoots:          projectActiveSkillRoots(activeRoots),
		DuplicateSkillRoots:       projectDuplicateSkillRoots(duplicates),
		SkillRootInspectionIssues: issues,
		FilesystemPresenceMeaning: "discovery_evidence_only_without_valid_manifest",
		MutationsPerformed:        false,
	}, nil
}

func mergeManifestAndDiscoveredSkillRoots(
	manifestRoots []initplanning.ActiveSkillRoot,
	discoveredRoots []initplanning.ActiveSkillRoot,
) []initplanning.ActiveSkillRoot {
	result := slices.Clone(manifestRoots)
	owned := make(map[string]struct{}, len(manifestRoots))
	for _, root := range manifestRoots {
		owned[skillRootBindingKey(root)] = struct{}{}
	}
	for _, root := range discoveredRoots {
		if _, manifested := owned[skillRootBindingKey(root)]; manifested {
			continue
		}
		result = append(result, root)
	}
	return result
}

func skillRootBindingKey(root initplanning.ActiveSkillRoot) string {
	return root.Root() + "\x00" +
		string(root.Host()) + "\x00" +
		string(root.Scope())
}

func inspectHostManifests(
	layout initplanning.PublicationLayout,
	projectRoot string,
	projectID string,
	candidates []currentStandardSkillCandidate,
	bundle initplanning.SkillSourceBundle,
	publication initplanning.PublicationIdentity,
	runtime currentHostPublicationRuntime,
	inspector initfs.HostStatusInspector,
) ([]hostManifestStatus, []initplanning.ActiveSkillRoot, int, error) {
	hosts := canonicalHostStatusHosts()
	scopes := []initplanning.InstallScope{
		initplanning.ScopeProject,
		initplanning.ScopeUser,
	}
	statuses := make([]hostManifestStatus, 0)
	roots := make([]initplanning.ActiveSkillRoot, 0)
	checked := 0
	for _, host := range hosts {
		for _, scope := range scopes {
			location, err := layout.ManifestLocation(host, scope)
			if err != nil {
				return nil, nil, checked, err
			}
			checked++
			store, err := initfs.NewManifestStore(
				location.Root(),
				location.Path(),
				hostStatusMaxCarrierBytes,
			)
			if err != nil {
				return nil, nil, checked, err
			}
			read, err := store.Read()
			if err != nil {
				statuses = append(statuses, hostManifestStatus{
					Path:           location.Path(),
					ReadPosture:    "invalid_or_unreadable",
					BindingPosture: "unavailable",
					Reasons:        []string{err.Error()},
				})
				continue
			}
			if read.Kind() == initfs.ManifestReadMissing {
				continue
			}
			manifest := read.Manifest()
			if manifest.Host() != host || manifest.Scope() != scope {
				statuses = append(statuses, hostManifestStatus{
					Path:           store.Path(),
					ReadPosture:    "valid_manifest",
					Host:           manifest.Host(),
					Scope:          manifest.Scope(),
					ManifestRef:    manifest.Ref(),
					ManifestDigest: manifest.Digest(),
					BindingPosture: "manifest_location_mismatch",
					Reasons: []string{
						fmt.Sprintf(
							"manifest declares host %s scope %s at the %s %s manifest location",
							manifest.Host(),
							manifest.Scope(),
							host,
							scope,
						),
					},
				})
				continue
			}
			status := inspectOneHostManifest(
				store,
				manifest,
				projectRoot,
				projectID,
				candidates,
				bundle,
				publication,
				runtime,
				inspector,
			)
			statuses = append(statuses, status)
			root, present, rootErr := manifestSkillRoot(manifest)
			if rootErr != nil {
				statuses[len(statuses)-1].Reasons = append(
					statuses[len(statuses)-1].Reasons,
					"manifest_skill_root: "+rootErr.Error(),
				)
				continue
			}
			if present {
				roots = append(roots, root)
			}
		}
	}
	sort.Slice(statuses, func(left int, right int) bool {
		return statuses[left].Path < statuses[right].Path
	})
	return statuses, roots, checked, nil
}

func inspectOneHostManifest(
	store initfs.ManifestStore,
	manifest initplanning.InstallationManifest,
	projectRoot string,
	projectID string,
	candidates []currentStandardSkillCandidate,
	bundle initplanning.SkillSourceBundle,
	publication initplanning.PublicationIdentity,
	runtime currentHostPublicationRuntime,
	inspector initfs.HostStatusInspector,
) hostManifestStatus {
	result := hostManifestStatus{
		Path:           store.Path(),
		ReadPosture:    "valid_manifest",
		Host:           manifest.Host(),
		Scope:          manifest.Scope(),
		ManifestRef:    manifest.Ref(),
		ManifestDigest: manifest.Digest(),
		BindingPosture: "not_evaluated",
	}
	sharedUserSkills := manifest.Scope() == initplanning.ScopeUser &&
		onlySkillsComponent(manifest.Components())
	sameProject := manifest.ProjectRoot() == projectRoot &&
		manifest.ProjectID() == projectID
	if !sameProject && !sharedUserSkills {
		result.BindingPosture = "other_project_binding"
		result.Reasons = []string{
			fmt.Sprintf(
				"manifest belongs to project %s at %s",
				manifest.ProjectID(),
				manifest.ProjectRoot(),
			),
		}
		return result
	}
	if sharedUserSkills {
		projectRoot = manifest.ProjectRoot()
		projectID = manifest.ProjectID()
	}
	if onlySkillsComponent(manifest.Components()) {
		return inspectStandardSkillManifest(
			result,
			store,
			manifest,
			projectRoot,
			projectID,
			candidates,
			publication,
			inspector,
		)
	}
	components, err := parseCurrentCoherentComponents(
		manifest.Components(),
	)
	if err != nil {
		result.BindingPosture = "desired_projection_unavailable"
		result.Reasons = []string{err.Error()}
		return result
	}
	projection, err := buildSelectedCurrentCoherentHostProjection(
		projectRoot,
		projectID,
		manifest.Host(),
		manifest.Scope(),
		components,
		candidates,
		bundle,
		publication,
		runtime,
	)
	if err != nil {
		result.BindingPosture = "desired_projection_unavailable"
		result.Reasons = []string{err.Error()}
		return result
	}
	inspection, err := inspector.InspectCoherentBinding(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		result.BindingPosture = "currentness_unavailable"
		result.Reasons = []string{err.Error()}
		return result
	}
	currentness := projectHostInstallationStatus(
		inspection.Status(),
	)
	result.BindingPosture = "evaluated"
	result.Currentness = &currentness
	return result
}

func inspectStandardSkillManifest(
	result hostManifestStatus,
	store initfs.ManifestStore,
	manifest initplanning.InstallationManifest,
	projectRoot string,
	projectID string,
	candidates []currentStandardSkillCandidate,
	publication initplanning.PublicationIdentity,
	inspector initfs.HostStatusInspector,
) hostManifestStatus {
	candidate, found := findCurrentStandardSkillCandidate(
		candidates,
		manifest.Host(),
		manifest.Scope(),
	)
	if !found {
		result.BindingPosture = "desired_projection_unavailable"
		result.Reasons = []string{
			"host has no lossless standard skill projection at this scope",
		}
		return result
	}
	projection, err := buildCurrentStandardSkillHostProjection(
		projectRoot,
		projectID,
		candidate,
		publication,
	)
	if err != nil {
		result.BindingPosture = "desired_projection_unavailable"
		result.Reasons = []string{err.Error()}
		return result
	}
	inspection, err := inspector.InspectBinding(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		result.BindingPosture = "currentness_unavailable"
		result.Reasons = []string{err.Error()}
		return result
	}
	currentness := projectHostInstallationStatus(
		inspection.Status(),
	)
	result.BindingPosture = "evaluated"
	result.Currentness = &currentness
	return result
}

func onlySkillsComponent(components []initplanning.Component) bool {
	return len(components) == 1 &&
		components[0] == initplanning.ComponentSkills
}

func projectHostInstallationStatus(
	status initplanning.HostInstallationStatus,
) hostInstallationStatus {
	paths := make([]hostPathCurrentness, len(status.Paths))
	for index, path := range status.Paths {
		paths[index] = hostPathCurrentness{
			Path:            path.Path,
			Component:       path.Component,
			State:           path.State,
			ObservedDigest:  path.ObservedDigest,
			ObservedMode:    path.ObservedMode,
			ManifestDigest:  path.ManifestDigest,
			ManifestMode:    path.ManifestMode,
			DesiredDigest:   path.DesiredDigest,
			DesiredMode:     path.DesiredMode,
			OwnershipKind:   path.OwnershipKind,
			OwnershipRef:    path.OwnershipRef,
			OwnershipDigest: path.OwnershipDigest,
		}
	}
	return hostInstallationStatus{
		Posture:                 status.Posture,
		Reasons:                 slices.Clone(status.Reasons),
		ManifestPresence:        status.ManifestPresence,
		ProjectRoot:             status.ProjectRoot,
		ProjectID:               status.ProjectID,
		Host:                    status.Host,
		Scope:                   status.Scope,
		ManifestRef:             status.ManifestRef,
		ManifestDigest:          status.ManifestDigest,
		InstalledAdapterEdition: status.InstalledAdapterEdition,
		DesiredAdapterEdition:   status.DesiredAdapterEdition,
		InstalledComponents:     slices.Clone(status.InstalledComponents),
		DesiredComponents:       slices.Clone(status.DesiredComponents),
		InstalledTargetRoots:    slices.Clone(status.InstalledTargetRoots),
		DesiredTargetRoots:      slices.Clone(status.DesiredTargetRoots),
		InstalledPublication: projectHostPublicationStatus(
			status.InstalledPublication,
		),
		DesiredPublication: projectHostPublicationStatus(
			status.DesiredPublication,
		),
		Paths: paths,
		VacantTargets: projectHostVacantTargetStatuses(
			status.VacantTargets,
		),
		ManagedFragments: projectHostManagedFragmentCurrentness(
			status.ManagedFragments,
		),
		VacantManagedFragments: projectHostVacantManagedFragmentStatuses(
			status.VacantManagedFragments,
		),
		ReconcileSurface: "pending_planner_driven_p12i_public_wiring",
	}
}

func projectHostPublicationStatus(
	status initplanning.PublicationStatus,
) hostPublicationStatus {
	return hostPublicationStatus{
		HaftVersion:         status.HaftVersion,
		ExecutablePath:      status.ExecutablePath,
		ExecutableDigest:    status.ExecutableDigest,
		SkillBundleDigest:   status.SkillBundleDigest,
		KernelCatalogDigest: status.KernelCatalogDigest,
	}
}

func projectHostVacantTargetStatuses(
	statuses []initplanning.VacantTargetStatus,
) []hostVacantTargetStatus {
	result := make([]hostVacantTargetStatus, len(statuses))
	for index, status := range statuses {
		result[index] = hostVacantTargetStatus{
			Path:          status.Path,
			Component:     status.Component,
			DesiredDigest: status.DesiredDigest,
			DesiredMode:   status.DesiredMode,
		}
	}
	return result
}

func projectHostManagedFragmentCurrentness(
	statuses []initplanning.ManagedFragmentCurrentnessStatus,
) []hostManagedFragmentCurrentness {
	result := make(
		[]hostManagedFragmentCurrentness,
		len(statuses),
	)
	for index, status := range statuses {
		result[index] = hostManagedFragmentCurrentness{
			CarrierPath:     status.CarrierPath,
			Component:       status.Component,
			Kind:            status.Kind,
			Selector:        status.Selector,
			MemberID:        status.MemberID,
			MergeEdition:    status.MergeEdition,
			State:           status.State,
			ObservedDigest:  status.ObservedDigest,
			ManifestDigest:  status.ManifestDigest,
			DesiredDigest:   status.DesiredDigest,
			OwnershipKind:   status.OwnershipKind,
			OwnershipRef:    status.OwnershipRef,
			OwnershipDigest: status.OwnershipDigest,
		}
	}
	return result
}

func projectHostVacantManagedFragmentStatuses(
	statuses []initplanning.VacantManagedFragmentStatus,
) []hostVacantManagedFragmentStatus {
	result := make(
		[]hostVacantManagedFragmentStatus,
		len(statuses),
	)
	for index, status := range statuses {
		result[index] = hostVacantManagedFragmentStatus{
			CarrierPath:   status.CarrierPath,
			Component:     status.Component,
			Kind:          status.Kind,
			Selector:      status.Selector,
			MemberID:      status.MemberID,
			MergeEdition:  status.MergeEdition,
			DesiredDigest: status.DesiredDigest,
		}
	}
	return result
}

func manifestSkillRoot(
	manifest initplanning.InstallationManifest,
) (initplanning.ActiveSkillRoot, bool, error) {
	exposures := make([]initplanning.SkillExposure, 0)
	root := ""
	for _, path := range manifest.RenderedPaths() {
		if path.Component != initplanning.ComponentSkills ||
			filepath.Base(path.Path) != "SKILL.md" {
			continue
		}
		skillDirectory := filepath.Dir(path.Path)
		name := filepath.Base(skillDirectory)
		exposure, err := initplanning.NewSkillExposure(name, path.Path)
		if err != nil {
			return initplanning.ActiveSkillRoot{}, false, err
		}
		candidateRoot := filepath.Dir(skillDirectory)
		if root == "" {
			root = candidateRoot
		}
		if root != candidateRoot {
			return initplanning.ActiveSkillRoot{}, false, fmt.Errorf(
				"manifest skill paths span multiple discovery roots",
			)
		}
		exposures = append(exposures, exposure)
	}
	if len(exposures) == 0 {
		return initplanning.ActiveSkillRoot{}, false, nil
	}
	active, err := initplanning.NewManifestSkillRoot(
		root,
		manifest.Host(),
		manifest.Scope(),
		manifest,
		exposures,
	)
	if err != nil {
		return initplanning.ActiveSkillRoot{}, false, err
	}
	return active, true, nil
}

func inspectCurrentSkillRoots(
	candidates []currentStandardSkillCandidate,
	inspector initfs.HostStatusInspector,
) (
	[]initplanning.SkillRootObservation,
	[]initplanning.ActiveSkillRoot,
	[]hostSkillRootIssue,
) {
	observations := make([]initplanning.SkillRootObservation, 0, len(candidates))
	roots := make([]initplanning.ActiveSkillRoot, 0, len(candidates))
	issues := make([]hostSkillRootIssue, 0)
	for _, candidate := range candidates {
		plan, err := initplanning.BuildSkillRootObservationPlan(
			candidate.projection,
			candidate.scope,
		)
		if err != nil {
			issues = append(issues, hostSkillRootIssue{
				Root:   candidate.targetRoot,
				Host:   candidate.host,
				Scope:  candidate.scope,
				Reason: err.Error(),
			})
			continue
		}
		status, err := inspector.InspectSkillRoots(
			nil,
			[]initplanning.SkillRootObservationPlan{plan},
		)
		if err != nil {
			issues = append(issues, hostSkillRootIssue{
				Root:   candidate.targetRoot,
				Host:   candidate.host,
				Scope:  candidate.scope,
				Reason: err.Error(),
			})
			continue
		}
		observations = append(observations, status.Observations()...)
		roots = append(roots, status.ActiveRoots()...)
	}
	sort.Slice(issues, func(left int, right int) bool {
		leftKey := issues[left].Root + "\x00" +
			string(issues[left].Host) + "\x00" +
			string(issues[left].Scope)
		rightKey := issues[right].Root + "\x00" +
			string(issues[right].Host) + "\x00" +
			string(issues[right].Scope)
		return leftKey < rightKey
	})
	return observations, roots, issues
}

func canonicalHostStatusHosts() []initplanning.HostID {
	return []initplanning.HostID{
		initplanning.HostAir,
		initplanning.HostAntigravity,
		initplanning.HostClaude,
		initplanning.HostCodex,
		initplanning.HostCursor,
		initplanning.HostGemini,
		initplanning.HostGrok,
		initplanning.HostHermes,
		initplanning.HostOpenCode,
		initplanning.HostPi,
		initplanning.HostZed,
	}
}

func sortActiveHostSkillRoots(roots []initplanning.ActiveSkillRoot) {
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

func projectSkillRootObservations(
	observations []initplanning.SkillRootObservation,
) []hostSkillRootObservation {
	result := make([]hostSkillRootObservation, len(observations))
	for index, observation := range observations {
		result[index] = hostSkillRootObservation{
			Root:           observation.Root(),
			Host:           observation.Host(),
			Scope:          observation.Scope(),
			EvidenceRef:    observation.EvidenceRef(),
			EvidenceDigest: observation.EvidenceDigest(),
			ExpectedCount:  observation.ExpectedCount(),
			ObservedCount:  observation.ObservedCount(),
		}
	}
	sort.Slice(result, func(left int, right int) bool {
		leftKey := result[left].Root + "\x00" +
			string(result[left].Host) + "\x00" +
			string(result[left].Scope)
		rightKey := result[right].Root + "\x00" +
			string(result[right].Host) + "\x00" +
			string(result[right].Scope)
		return leftKey < rightKey
	})
	return result
}

func projectActiveSkillRoots(
	roots []initplanning.ActiveSkillRoot,
) []hostActiveSkillRoot {
	result := make([]hostActiveSkillRoot, len(roots))
	for index, root := range roots {
		exposures := root.Exposures()
		names := make([]string, len(exposures))
		for exposureIndex, exposure := range exposures {
			names[exposureIndex] = exposure.Name()
		}
		sort.Strings(names)
		result[index] = hostActiveSkillRoot{
			Root:           root.Root(),
			Host:           root.Host(),
			Scope:          root.Scope(),
			Origin:         root.Origin(),
			EvidenceRef:    root.EvidenceRef(),
			EvidenceDigest: root.EvidenceDigest(),
			SkillNames:     names,
		}
	}
	return result
}

func projectDuplicateSkillRoots(
	duplicates []initplanning.DuplicateSkillRoot,
) []hostDuplicateSkillRoot {
	result := make([]hostDuplicateSkillRoot, len(duplicates))
	for index, duplicate := range duplicates {
		result[index] = hostDuplicateSkillRoot{
			SkillName:           duplicate.SkillName,
			LeftRoot:            duplicate.LeftRoot,
			LeftHost:            duplicate.LeftHost,
			LeftScope:           duplicate.LeftScope,
			LeftOrigin:          duplicate.LeftOrigin,
			LeftEvidenceRef:     duplicate.LeftEvidenceRef,
			LeftEvidenceDigest:  duplicate.LeftEvidenceDigest,
			RightRoot:           duplicate.RightRoot,
			RightHost:           duplicate.RightHost,
			RightScope:          duplicate.RightScope,
			RightOrigin:         duplicate.RightOrigin,
			RightEvidenceRef:    duplicate.RightEvidenceRef,
			RightEvidenceDigest: duplicate.RightEvidenceDigest,
		}
	}
	return result
}

func writeHostStatusJSON(
	writer io.Writer,
	report hostStatusReport,
) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode host status: %w", err)
	}
	return nil
}

func writeHostStatusText(
	writer io.Writer,
	report hostStatusReport,
) error {
	builder := strings.Builder{}
	fmt.Fprintln(&builder, "Haft host status")
	fmt.Fprintf(
		&builder,
		"Core: %s project=%s schema=%d\n",
		report.Project.CorePosture,
		report.Project.ProjectID,
		report.Project.SchemaVersion,
	)
	fmt.Fprintf(
		&builder,
		"Manifests: %d present across %d canonical locations\n",
		len(report.Manifests),
		report.ManifestLocationsChecked,
	)
	for _, manifest := range report.Manifests {
		fmt.Fprintf(
			&builder,
			"  - %s: %s",
			manifest.Path,
			manifest.BindingPosture,
		)
		if manifest.Currentness != nil {
			fmt.Fprintf(
				&builder,
				" (%s)",
				manifest.Currentness.Posture,
			)
		}
		fmt.Fprintln(&builder)
		for _, reason := range manifest.Reasons {
			fmt.Fprintf(&builder, "      %s\n", reason)
		}
	}
	summaries := summarizeHostDuplicateRootPairs(
		report.DuplicateSkillRoots,
	)
	fmt.Fprintf(
		&builder,
		"Active skill roots: %d; duplicate exposures: %d across %d distinct root pairs; inspection issues: %d\n",
		len(report.ActiveSkillRoots),
		len(report.DuplicateSkillRoots),
		len(summaries),
		len(report.SkillRootInspectionIssues),
	)
	for _, root := range report.ActiveSkillRoots {
		fmt.Fprintf(
			&builder,
			"  - %s [%s/%s, %s]: skills=%d\n",
			root.Root,
			root.Host,
			root.Scope,
			root.Origin,
			len(root.SkillNames),
		)
	}
	if len(report.DuplicateSkillRoots) > 0 {
		fmt.Fprintln(
			&builder,
			"  Complete per-skill evidence: haft host status --json",
		)
	}
	fmt.Fprintln(
		&builder,
		"Filesystem presence: discovery evidence only without a valid installation manifest.",
	)
	fmt.Fprintln(&builder, "Effects: none")
	if _, err := io.WriteString(writer, builder.String()); err != nil {
		return fmt.Errorf("write host status: %w", err)
	}
	return nil
}

type hostDuplicateRootPairSummary struct {
	LeftRoot    string
	LeftHost    initplanning.HostID
	LeftScope   initplanning.InstallScope
	LeftOrigin  initplanning.SkillRootOrigin
	RightRoot   string
	RightHost   initplanning.HostID
	RightScope  initplanning.InstallScope
	RightOrigin initplanning.SkillRootOrigin
	SkillCount  int
}

type hostDuplicateRootPairAccumulator struct {
	summary    hostDuplicateRootPairSummary
	skillNames map[string]struct{}
}

func summarizeHostDuplicateRootPairs(
	duplicates []hostDuplicateSkillRoot,
) []hostDuplicateRootPairSummary {
	pairs := make(map[string]hostDuplicateRootPairAccumulator)
	for _, duplicate := range duplicates {
		summary := projectHostDuplicateRootPairSummary(duplicate)
		key := hostDuplicateRootPairSummaryKey(summary)
		accumulator, found := pairs[key]
		if !found {
			accumulator = hostDuplicateRootPairAccumulator{
				summary:    summary,
				skillNames: make(map[string]struct{}),
			}
		}
		accumulator.skillNames[duplicate.SkillName] = struct{}{}
		pairs[key] = accumulator
	}
	result := make(
		[]hostDuplicateRootPairSummary,
		0,
		len(pairs),
	)
	for _, accumulator := range pairs {
		summary := accumulator.summary
		summary.SkillCount = len(accumulator.skillNames)
		result = append(result, summary)
	}
	sort.Slice(result, func(left int, right int) bool {
		leftKey := hostDuplicateRootPairSummaryKey(result[left])
		rightKey := hostDuplicateRootPairSummaryKey(result[right])
		return leftKey < rightKey
	})
	return result
}

func projectHostDuplicateRootPairSummary(
	duplicate hostDuplicateSkillRoot,
) hostDuplicateRootPairSummary {
	leftKey := hostDuplicateRootEndpointKey(
		duplicate.LeftRoot,
		duplicate.LeftHost,
		duplicate.LeftScope,
		duplicate.LeftOrigin,
	)
	rightKey := hostDuplicateRootEndpointKey(
		duplicate.RightRoot,
		duplicate.RightHost,
		duplicate.RightScope,
		duplicate.RightOrigin,
	)
	if leftKey <= rightKey {
		return hostDuplicateRootPairSummary{
			LeftRoot:    duplicate.LeftRoot,
			LeftHost:    duplicate.LeftHost,
			LeftScope:   duplicate.LeftScope,
			LeftOrigin:  duplicate.LeftOrigin,
			RightRoot:   duplicate.RightRoot,
			RightHost:   duplicate.RightHost,
			RightScope:  duplicate.RightScope,
			RightOrigin: duplicate.RightOrigin,
		}
	}
	return hostDuplicateRootPairSummary{
		LeftRoot:    duplicate.RightRoot,
		LeftHost:    duplicate.RightHost,
		LeftScope:   duplicate.RightScope,
		LeftOrigin:  duplicate.RightOrigin,
		RightRoot:   duplicate.LeftRoot,
		RightHost:   duplicate.LeftHost,
		RightScope:  duplicate.LeftScope,
		RightOrigin: duplicate.LeftOrigin,
	}
}

func hostDuplicateRootPairSummaryKey(
	summary hostDuplicateRootPairSummary,
) string {
	left := hostDuplicateRootEndpointKey(
		summary.LeftRoot,
		summary.LeftHost,
		summary.LeftScope,
		summary.LeftOrigin,
	)
	right := hostDuplicateRootEndpointKey(
		summary.RightRoot,
		summary.RightHost,
		summary.RightScope,
		summary.RightOrigin,
	)
	return left + "\x00" + right
}

func hostDuplicateRootEndpointKey(
	root string,
	host initplanning.HostID,
	scope initplanning.InstallScope,
	origin initplanning.SkillRootOrigin,
) string {
	return root + "\x00" +
		string(host) + "\x00" +
		string(scope) + "\x00" +
		string(origin)
}
