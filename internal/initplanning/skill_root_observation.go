package initplanning

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const skillRootObservationSchema = "haft.skill-root-observation/v1"

type SkillRootObservationPlan struct {
	root       string
	host       HostID
	scope      InstallScope
	exposures  []SkillExposure
	filesystem InstallationObservationPlan
}

func BuildSkillRootObservationPlan(
	projection SkillComponentProjection,
	scope InstallScope,
) (SkillRootObservationPlan, error) {
	root, exposures, err := skillRootExposuresFromProjection(projection)
	if err != nil {
		return SkillRootObservationPlan{}, err
	}
	if _, err := validateSkillRootBinding(
		root,
		projection.host,
		scope,
		exposures,
	); err != nil {
		return SkillRootObservationPlan{}, err
	}
	targets := make([]ObservationTarget, len(exposures))
	for index, exposure := range exposures {
		targets[index] = ObservationTarget{
			path:        exposure.path,
			components:  ComponentSet{values: []Component{ComponentSkills}},
			requirement: ObservationIfPresent,
		}
	}
	filesystem := InstallationObservationPlan{
		managedRoots: []string{root},
		targets:      targets,
	}
	return SkillRootObservationPlan{
		root:       root,
		host:       projection.host,
		scope:      scope,
		exposures:  slices.Clone(exposures),
		filesystem: filesystem,
	}, nil
}

func skillRootExposuresFromProjection(
	projection SkillComponentProjection,
) (string, []SkillExposure, error) {
	if _, known := knownHosts[projection.host]; !known {
		return "", nil, fmt.Errorf("skill-root projection host is invalid")
	}
	if !adapterEditionPattern.MatchString(projection.edition) ||
		len(projection.records) == 0 {
		return "", nil, fmt.Errorf("skill-root projection is invalid")
	}
	exposures := make([]SkillExposure, 0, len(projection.records))
	root := ""
	for _, record := range projection.records {
		exposure, candidateRoot, err := skillExposureFromRecord(record)
		if err != nil {
			return "", nil, err
		}
		if root == "" {
			root = candidateRoot
		}
		if root != candidateRoot {
			return "", nil, fmt.Errorf("skill projection spans multiple discovery roots")
		}
		exposures = append(exposures, exposure)
	}
	sort.Slice(exposures, func(left int, right int) bool {
		return skillExposureKey(exposures[left]) < skillExposureKey(exposures[right])
	})
	if !pathWithinAnyRoot(root, []string{projection.root}) {
		return "", nil, fmt.Errorf("skill discovery root is outside its projection root")
	}
	return root, exposures, nil
}

func skillExposureFromRecord(
	record RenderedSkillRecord,
) (SkillExposure, string, error) {
	if !skillNamePattern.MatchString(record.Name) ||
		!sha256DigestPattern.MatchString(record.RenderedSkillDigest) {
		return SkillExposure{}, "", fmt.Errorf("rendered skill record is invalid")
	}
	if filepath.Base(record.RenderedSkillPath) != "SKILL.md" {
		return SkillExposure{}, "", fmt.Errorf(
			"rendered skill %s has no canonical skill carrier",
			record.Name,
		)
	}
	skillDirectory := filepath.Dir(record.RenderedSkillPath)
	if filepath.Base(skillDirectory) != record.Name {
		return SkillExposure{}, "", fmt.Errorf(
			"rendered skill %s path does not preserve its name",
			record.Name,
		)
	}
	exposure, err := NewSkillExposure(record.Name, record.RenderedSkillPath)
	if err != nil {
		return SkillExposure{}, "", err
	}
	return exposure, filepath.Dir(skillDirectory), nil
}

func skillExposureKey(exposure SkillExposure) string {
	return exposure.name + "\x00" + exposure.path
}

func (plan SkillRootObservationPlan) Root() string {
	return plan.root
}

func (plan SkillRootObservationPlan) Host() HostID {
	return plan.host
}

func (plan SkillRootObservationPlan) Scope() InstallScope {
	return plan.scope
}

func (plan SkillRootObservationPlan) Exposures() []SkillExposure {
	return slices.Clone(plan.exposures)
}

func (plan SkillRootObservationPlan) FilesystemPlan() InstallationObservationPlan {
	return InstallationObservationPlan{
		managedRoots: slices.Clone(plan.filesystem.managedRoots),
		targets:      slices.Clone(plan.filesystem.targets),
	}
}

type skillRootExposureObservationWire struct {
	Name   string              `json:"name"`
	Path   string              `json:"path"`
	State  PathObservationKind `json:"state"`
	Digest string              `json:"digest,omitempty"`
	Mode   uint32              `json:"mode,omitempty"`
}

type skillRootObservationWire struct {
	Schema    string                             `json:"schema"`
	Root      string                             `json:"root"`
	Host      HostID                             `json:"host"`
	Scope     InstallScope                       `json:"scope"`
	Exposures []skillRootExposureObservationWire `json:"exposures"`
}

type SkillRootObservation struct {
	root           string
	host           HostID
	scope          InstallScope
	evidenceRef    string
	evidenceDigest string
	canonical      []byte
	expectedCount  int
	observedCount  int
	activeRoot     ActiveSkillRoot
	active         bool
}

func ProjectSkillRootObservation(
	plan SkillRootObservationPlan,
	observations []PathObservation,
) (SkillRootObservation, error) {
	if err := validateSkillRootObservationPlan(plan); err != nil {
		return SkillRootObservation{}, err
	}
	observedByPath, err := observationsByPath(observations)
	if err != nil {
		return SkillRootObservation{}, err
	}
	exposureByPath := make(map[string]SkillExposure, len(plan.exposures))
	for _, exposure := range plan.exposures {
		exposureByPath[exposure.path] = exposure
	}
	if err := validateSkillRootObservations(observedByPath, exposureByPath); err != nil {
		return SkillRootObservation{}, err
	}
	wire, activeExposures := buildSkillRootObservationWire(
		plan,
		observedByPath,
	)
	canonical, err := json.Marshal(wire)
	if err != nil {
		return SkillRootObservation{}, fmt.Errorf("encode skill-root observation: %w", err)
	}
	digest := digestBytesForManifest(canonical)
	evidenceRef := "skill-root-observation:" + strings.TrimPrefix(digest, "sha256:")
	result := SkillRootObservation{
		root:           plan.root,
		host:           plan.host,
		scope:          plan.scope,
		evidenceRef:    evidenceRef,
		evidenceDigest: digest,
		canonical:      canonical,
		expectedCount:  len(plan.exposures),
		observedCount:  len(activeExposures),
	}
	if len(activeExposures) == 0 {
		return result, nil
	}
	activeRoot, err := NewDiscoveredSkillRoot(
		plan.root,
		plan.host,
		plan.scope,
		evidenceRef,
		digest,
		activeExposures,
	)
	if err != nil {
		return SkillRootObservation{}, err
	}
	result.activeRoot = activeRoot
	result.active = true
	return result, nil
}

func validateSkillRootObservationPlan(
	plan SkillRootObservationPlan,
) error {
	if _, err := validateSkillRootBinding(
		plan.root,
		plan.host,
		plan.scope,
		plan.exposures,
	); err != nil {
		return err
	}
	if len(plan.filesystem.managedRoots) != 1 ||
		plan.filesystem.managedRoots[0] != plan.root ||
		len(plan.filesystem.targets) != len(plan.exposures) {
		return fmt.Errorf("skill-root filesystem observation plan is invalid")
	}
	return nil
}

func validateSkillRootObservations(
	observedByPath map[string]PathObservation,
	exposureByPath map[string]SkillExposure,
) error {
	for path, observation := range observedByPath {
		if _, expected := exposureByPath[path]; !expected {
			return fmt.Errorf("skill-root observation contains an unplanned path %s", path)
		}
		if observation.kind != PathObservedPresent ||
			observation.Component() != ComponentSkills {
			return fmt.Errorf("skill-root observation for %s is not a present skill carrier", path)
		}
	}
	return nil
}

func buildSkillRootObservationWire(
	plan SkillRootObservationPlan,
	observedByPath map[string]PathObservation,
) (skillRootObservationWire, []SkillExposure) {
	exposures := make([]skillRootExposureObservationWire, 0, len(plan.exposures))
	active := make([]SkillExposure, 0, len(observedByPath))
	for _, exposure := range plan.exposures {
		observation, present := observedByPath[exposure.path]
		wire := skillRootExposureObservationWire{
			Name:  exposure.name,
			Path:  exposure.path,
			State: PathObservedMissing,
		}
		if present {
			wire.State = PathObservedPresent
			wire.Digest = observation.digest
			wire.Mode = uint32(observation.mode.Perm())
			active = append(active, exposure)
		}
		exposures = append(exposures, wire)
	}
	return skillRootObservationWire{
		Schema:    skillRootObservationSchema,
		Root:      plan.root,
		Host:      plan.host,
		Scope:     plan.scope,
		Exposures: exposures,
	}, active
}

func (observation SkillRootObservation) Root() string {
	return observation.root
}

func (observation SkillRootObservation) Host() HostID {
	return observation.host
}

func (observation SkillRootObservation) Scope() InstallScope {
	return observation.scope
}

func (observation SkillRootObservation) EvidenceRef() string {
	return observation.evidenceRef
}

func (observation SkillRootObservation) EvidenceDigest() string {
	return observation.evidenceDigest
}

func (observation SkillRootObservation) CanonicalBytes() []byte {
	return slices.Clone(observation.canonical)
}

func (observation SkillRootObservation) ExpectedCount() int {
	return observation.expectedCount
}

func (observation SkillRootObservation) ObservedCount() int {
	return observation.observedCount
}

func (observation SkillRootObservation) ActiveRoot() (ActiveSkillRoot, bool) {
	if !observation.active {
		return ActiveSkillRoot{}, false
	}
	root := observation.activeRoot
	root.exposures = slices.Clone(observation.activeRoot.exposures)
	return root, true
}
