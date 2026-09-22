package cli

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

// Exact Codex rendering of internal/cli/skill/h-reason/SKILL.md at
// de0f1407, using codex.skill-syntax.v1. This is the generated project copy
// observed after the September 2026 upgrade. Markers and MCP names alone
// cannot distinguish a generated carrier from a human-edited derivative.
const publicLegacyCodexReasonDigest = "sha256:6fefdc99a049a9197e742864477f5a9b62ef332022ef79d812217e3d906a4e2c"

func reconcilePublicCodexProjectSkills(
	request publicInitRequest,
	binding publicHostBinding,
	store initfs.ManifestStore,
	projection initplanning.HostAdapterProjection,
	candidates []currentStandardSkillCandidate,
	maxCarrierBytes int64,
) (initplanning.HostAdapterProjection, publicHostBinding, map[string]string, error) {
	exactDigests := make(map[string]string)
	if binding.host != initplanning.HostCodex ||
		binding.scope != initplanning.ScopeProject ||
		request.hostMode != publicHostFullIntegration || request.local {
		return projection, binding, exactDigests, nil
	}
	candidate, found := findCurrentStandardSkillCandidate(candidates, binding.host, binding.scope)
	if !found {
		return projection, binding, exactDigests, fmt.Errorf("Codex project skill projection is unavailable")
	}
	skillProjection, err := buildCurrentStandardSkillHostProjection(
		request.projectRoot, request.projectID, candidate, projection.Publication(),
	)
	if err != nil {
		return projection, binding, exactDigests, err
	}
	observations, err := observePublicProjectionPaths(skillProjection, maxCarrierBytes)
	if err != nil {
		return projection, binding, exactDigests, err
	}
	read, err := store.Read()
	if err != nil {
		return projection, binding, exactDigests, err
	}
	outputs, exactDigests, err := selectPublicDiscoveredCodexSkills(
		skillProjection.Outputs(), observations, read.Manifest().RenderedPaths(),
	)
	if err != nil || len(outputs) == 0 {
		return projection, binding, exactDigests, err
	}
	components := binding.components.Values()
	components = append(components, initplanning.ComponentSkills)
	binding.components, err = parseCurrentCoherentComponents(components)
	if err != nil {
		return projection, binding, exactDigests, err
	}
	projection, err = extendPublicProjectSkillProjection(projection, binding, candidate.targetRoot, outputs)
	return projection, binding, exactDigests, err
}

func observePublicProjectionPaths(
	projection initplanning.HostAdapterProjection,
	maxCarrierBytes int64,
) ([]initplanning.PathObservation, error) {
	outputs := projection.Outputs()
	if len(outputs) == 0 {
		return nil, nil
	}
	legacy := initplanning.WithoutKnownLegacyRegistry()
	plan, err := initplanning.BuildFirstInstallationObservationPlan(projection, legacy)
	if err != nil {
		return nil, err
	}
	observer, err := initfs.NewFileObserver(maxCarrierBytes)
	if err != nil {
		return nil, err
	}
	return observer.Observe(plan)
}

// Only already-discoverable carriers and previously owned paths enter the
// project publication. An absent unowned skill is never installed here.
// Digests remain pinned through legacy reconciliation and publication CAS.
func selectPublicDiscoveredCodexSkills(
	outputs []initplanning.RenderedOutput,
	observations []initplanning.PathObservation,
	manifestPaths []initplanning.ManifestPath,
) ([]initplanning.RenderedOutput, map[string]string, error) {
	observed := make(map[string]initplanning.PathObservation, len(observations))
	for _, observation := range observations {
		observed[observation.Path()] = observation
	}
	owned := make(map[string]initplanning.ManifestPath, len(manifestPaths))
	for _, path := range manifestPaths {
		owned[path.Path] = path
	}
	discoverable := make(map[string]bool)
	for _, output := range outputs {
		if filepath.Base(output.Path()) != "SKILL.md" {
			continue
		}
		observation := observed[output.Path()]
		_, previouslyOwned := owned[output.Path()]
		discoverable[filepath.Dir(output.Path())] = previouslyOwned ||
			observation.Kind() == initplanning.PathObservedPresent
	}
	selected := make([]initplanning.RenderedOutput, 0, len(outputs))
	exactDigests := make(map[string]string)
	for _, output := range outputs {
		observation := observed[output.Path()]
		manifest, previouslyOwned := owned[output.Path()]
		skillRoot := publicCodexOutputSkillRoot(output.Path())
		present := observation.Kind() == initplanning.PathObservedPresent
		if !previouslyOwned && (!present || !discoverable[skillRoot]) {
			continue
		}
		if present && !isExactPublicDiscoveredCodexSkill(output, observation, manifest, previouslyOwned) {
			return nil, nil, fmt.Errorf(
				"discoverable Codex project skill %s is ambiguous: its bytes or mode do not match an unchanged Haft manifest or an exact known generated carrier; preserved without changes; compare this project copy with the current global skill and explicitly keep, move, or replace it before rerunning haft init --codex",
				output.Path(),
			)
		}
		digest := output.Digest()
		if present {
			digest = observation.Digest()
		}
		selected = append(selected, output)
		exactDigests[output.Path()] = digest
	}
	return selected, exactDigests, nil
}

func publicCodexOutputSkillRoot(path string) string {
	parent := filepath.Dir(path)
	if filepath.Base(path) == "openai.yaml" {
		return filepath.Dir(parent)
	}
	return parent
}

func isExactPublicDiscoveredCodexSkill(
	output initplanning.RenderedOutput,
	observation initplanning.PathObservation,
	manifest initplanning.ManifestPath,
	previouslyOwned bool,
) bool {
	if previouslyOwned {
		mode := uint32(observation.Mode())
		return observation.Digest() == manifest.Digest && mode == manifest.Mode
	}
	if observation.Mode() != output.Mode() {
		return false
	}
	if observation.Digest() == output.Digest() {
		return true
	}
	path := output.Path()
	skillRoot := filepath.Dir(path)
	return filepath.Base(path) == "SKILL.md" && filepath.Base(skillRoot) == "h-reason" &&
		observation.Digest() == publicLegacyCodexReasonDigest
}

func extendPublicProjectSkillProjection(
	projection initplanning.HostAdapterProjection,
	binding publicHostBinding,
	skillRoot string,
	outputs []initplanning.RenderedOutput,
) (initplanning.HostAdapterProjection, error) {
	builder := initplanning.NewHostAdapterProjectionBuilder(projection.Host()).
		AtEdition(projection.Edition()).
		PublishedFrom(projection.Publication()).
		ForProject(projection.ProjectRoot(), projection.ProjectID().String()).
		WithSelection(projection.Scope(), binding.components).
		RecoverWith(projection.Recovery())
	roots := projection.TargetRoots()
	if !slices.Contains(roots, skillRoot) {
		roots = append(roots, skillRoot)
	}
	for _, root := range roots {
		builder = builder.AddTargetRoot(root)
	}
	for _, output := range projection.Outputs() {
		builder = builder.AddOutput(output)
	}
	for _, output := range outputs {
		builder = builder.AddOutput(output)
	}
	for _, fragment := range projection.ManagedFragments() {
		builder = builder.AddManagedFragment(fragment)
	}
	return builder.Build()
}

// A normal init may adopt only part of a discoverable project root. Status
// compares owned and present paths; absent unowned duplicates are not
// installation debt. Present foreign collisions must remain visible.
func installedPublicCodexProjectSkillProjection(
	projection initplanning.HostAdapterProjection,
	manifest initplanning.InstallationManifest,
	observations []initplanning.PathObservation,
) (initplanning.HostAdapterProjection, error) {
	if projection.Host() != initplanning.HostCodex || projection.Scope() != initplanning.ScopeProject {
		return projection, nil
	}
	owned := make(map[string]bool)
	for _, path := range manifest.RenderedPaths() {
		owned[path.Path] = true
	}
	present := make(map[string]bool)
	for _, observation := range observations {
		present[observation.Path()] = observation.Kind() == initplanning.PathObservedPresent
	}
	components := projection.Components()
	builder := initplanning.NewHostAdapterProjectionBuilder(projection.Host()).
		AtEdition(projection.Edition()).
		PublishedFrom(projection.Publication()).
		ForProject(projection.ProjectRoot(), projection.ProjectID().String()).
		WithSelection(projection.Scope(), components).
		RecoverWith(projection.Recovery())
	for _, root := range projection.TargetRoots() {
		builder = builder.AddTargetRoot(root)
	}
	for _, output := range projection.Outputs() {
		if output.Component() == initplanning.ComponentSkills && !owned[output.Path()] && !present[output.Path()] {
			continue
		}
		builder = builder.AddOutput(output)
	}
	for _, fragment := range projection.ManagedFragments() {
		builder = builder.AddManagedFragment(fragment)
	}
	return builder.Build()
}
