package initplanning

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestClassifyInstallationCurrentnessProducesSevenExactOwnershipStates(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	report, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		fixture.observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	paths := report.Paths()
	if len(paths) != 7 {
		t.Fatalf("classified path count = %d, want 7", len(paths))
	}
	if !sort.SliceIsSorted(paths, func(left int, right int) bool {
		return paths[left].Path() < paths[right].Path()
	}) {
		t.Fatalf("classified paths are not canonical: %+v", paths)
	}
	want := map[string]PathCurrentnessKind{
		fixture.paths.current:  PathCurrentOwned,
		fixture.paths.outdated: PathOutdatedOwned,
		fixture.paths.modified: PathLocallyModifiedOwned,
		fixture.paths.legacy:   PathKnownLegacyExact,
		fixture.paths.foreign:  PathForeign,
		fixture.paths.orphaned: PathOrphanedOwned,
		fixture.paths.missing:  PathMissingOwned,
	}
	byPath := make(map[string]PathCurrentness, len(paths))
	for _, path := range paths {
		byPath[path.Path()] = path
		if path.Kind() != want[path.Path()] {
			t.Fatalf("path %s state = %s, want %s", path.Path(), path.Kind(), want[path.Path()])
		}
	}
	manifestBasis := fixture.manifest.OwnershipBasis()
	for _, path := range []string{
		fixture.paths.current,
		fixture.paths.outdated,
		fixture.paths.modified,
		fixture.paths.orphaned,
		fixture.paths.missing,
	} {
		basis := byPath[path].OwnershipBasis()
		if basis.Kind() != OwnershipManifestReceipt ||
			basis.Ref() != manifestBasis.Ref() ||
			basis.Digest() != manifestBasis.Digest() {
			t.Fatalf("path %s manifest basis = %+v", path, basis)
		}
	}
	legacyBasis := fixture.registry.OwnershipBasis()
	legacy := byPath[fixture.paths.legacy].OwnershipBasis()
	if legacy.Kind() != OwnershipLegacyRegistry ||
		legacy.Ref() != legacyBasis.Ref() ||
		legacy.Digest() != legacyBasis.Digest() {
		t.Fatalf("legacy basis = %+v", legacy)
	}
	if byPath[fixture.paths.foreign].OwnershipBasis().valid() {
		t.Fatal("foreign path claims an ownership basis")
	}
	modified := byPath[fixture.paths.modified]
	if modified.ObservedDigest() != modified.DesiredDigest() {
		t.Fatal("fixture no longer proves desired-byte equality is not ownership")
	}
	if modified.ObservedDigest() == modified.ManifestDigest() {
		t.Fatal("fixture no longer proves manifest divergence")
	}
	missing := byPath[fixture.paths.missing]
	if missing.ObservedDigest() != "" || missing.ManifestDigest() == "" {
		t.Fatalf("missing-owned evidence = %+v", missing)
	}
	vacant := report.VacantTargets()
	if len(vacant) != 1 || vacant[0].Path() != fixture.paths.vacant {
		t.Fatalf("vacant targets = %+v", vacant)
	}
	if vacant[0].DesiredDigest() == "" || vacant[0].Component() != ComponentSkills {
		t.Fatalf("vacant target lost desired projection: %+v", vacant[0])
	}
}

func TestCurrentnessDoesNotAdoptDesiredOrRegisteredBytesWithoutExactWitness(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	wrongDigest := digestBytes([]byte("unregistered legacy edit"))
	for index, observation := range fixture.observations {
		if observation.path != fixture.paths.legacy {
			continue
		}
		fixture.observations[index] = mustPresentObservation(
			t,
			observation.path,
			observation.Component(),
			wrongDigest,
		)
	}
	report, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		fixture.observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	for _, path := range report.Paths() {
		if path.Path() == fixture.paths.legacy && path.Kind() != PathForeign {
			t.Fatalf("wrong legacy digest state = %s", path.Kind())
		}
		if path.Path() == fixture.paths.foreign && path.Kind() != PathForeign {
			t.Fatalf("desired-byte collision state = %s", path.Kind())
		}
	}
}

func TestCurrentnessTreatsOwnedModeDriftAsModificationOrOutdatedProjection(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	modeModified := cloneObservations(fixture.observations)
	for index, observation := range modeModified {
		if observation.path != fixture.paths.current {
			continue
		}
		replacement, err := ObservePresentPath(
			observation.path,
			observation.Component(),
			observation.digest,
			0o600,
		)
		if err != nil {
			t.Fatalf("ObservePresentPath: %v", err)
		}
		modeModified[index] = replacement
	}
	report, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		modeModified,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness mode-modified: %v", err)
	}
	if stateForPath(t, report, fixture.paths.current) != PathLocallyModifiedOwned {
		t.Fatal("manifest-owned mode drift was not classified as a local modification")
	}

	outputs := fixture.projection.Outputs()
	for index, output := range outputs {
		if output.path != fixture.paths.current {
			continue
		}
		replacement, err := NewRenderedOutput(
			output.path,
			output.Component(),
			output.content,
			0o600,
		)
		if err != nil {
			t.Fatalf("NewRenderedOutput: %v", err)
		}
		outputs[index] = replacement
	}
	projection := buildCurrentProjection(t, root, outputs)
	report, err = ClassifyInstallationCurrentness(
		fixture.manifest,
		projection,
		fixture.observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness desired-mode: %v", err)
	}
	if stateForPath(t, report, fixture.paths.current) != PathOutdatedOwned {
		t.Fatal("desired mode change was not classified as an outdated owned carrier")
	}
}

func TestClassifyInstallationCurrentnessRejectsIncompleteOrAmbiguousEvidence(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	missingRequired := removeObservation(fixture.observations, fixture.paths.current)
	duplicate := append(
		cloneObservations(fixture.observations),
		fixture.observations[0],
	)
	extraMissing := append(
		cloneObservations(fixture.observations),
		mustMissingObservation(t, filepath.Join(root, "extra-missing"), ComponentSkills),
	)
	componentMismatch := cloneObservations(fixture.observations)
	for index, observation := range componentMismatch {
		if observation.path == fixture.paths.current {
			componentMismatch[index].components = mustComponents(
				t,
				ComponentMCP,
			)
		}
	}
	duplicateProjection := cloneHostAdapterProjection(fixture.projection)
	duplicateProjection.outputs = append(
		duplicateProjection.outputs,
		duplicateProjection.outputs[0],
	)
	for name, test := range map[string]struct {
		projection   HostAdapterProjection
		observations []PathObservation
		legacy       LegacyRegistrySelection
	}{
		"missing required": {
			projection: fixture.projection, observations: missingRequired, legacy: fixture.legacySelection,
		},
		"duplicate observation": {
			projection: fixture.projection, observations: duplicate, legacy: fixture.legacySelection,
		},
		"unrelated missing": {
			projection: fixture.projection, observations: extraMissing, legacy: fixture.legacySelection,
		},
		"component mismatch": {
			projection: fixture.projection, observations: componentMismatch, legacy: fixture.legacySelection,
		},
		"duplicate desired output": {
			projection: duplicateProjection, observations: fixture.observations, legacy: fixture.legacySelection,
		},
		"invalid legacy selection": {
			projection: fixture.projection, observations: fixture.observations, legacy: LegacyRegistrySelection{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ClassifyInstallationCurrentness(
				fixture.manifest,
				test.projection,
				test.observations,
				test.legacy,
			)
			if err == nil {
				t.Fatal("classification accepted incomplete or ambiguous evidence")
			}
		})
	}
}

func TestCurrentnessRejectsLegacyRegistryFromAnotherInstallationBinding(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	other, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
		Edition:     "legacy.v1",
		ProjectRoot: root,
		ProjectID:   "qnt_b68cc95b",
		Host:        HostCodex,
		Scope:       ScopeProject,
		TargetRoots: []string{root},
		Paths:       fixture.registry.Paths(),
	})
	if err != nil {
		t.Fatalf("BuildKnownLegacyDigestRegistry: %v", err)
	}
	selection, err := WithKnownLegacyRegistry(other)
	if err != nil {
		t.Fatalf("WithKnownLegacyRegistry: %v", err)
	}
	_, err = ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		fixture.observations,
		selection,
	)
	if err == nil {
		t.Fatal("classification accepted another project's legacy registry")
	}
}

func TestCurrentnessReportGettersDoNotExposeMutableSlices(t *testing.T) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	report, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		fixture.observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	pathsBefore := report.Paths()
	pathsChanged := report.Paths()
	pathsChanged[0].path = filepath.Join(root, "changed")
	if !reflect.DeepEqual(report.Paths(), pathsBefore) {
		t.Fatal("currentness path getter exposed carrier storage")
	}
	vacantBefore := report.VacantTargets()
	vacantChanged := report.VacantTargets()
	vacantChanged[0].path = filepath.Join(root, "changed-vacant")
	if !reflect.DeepEqual(report.VacantTargets(), vacantBefore) {
		t.Fatal("vacant-target getter exposed carrier storage")
	}
}

type currentnessFixturePaths struct {
	current  string
	outdated string
	modified string
	orphaned string
	missing  string
	legacy   string
	foreign  string
	vacant   string
}

type currentnessFixture struct {
	manifest        InstallationManifest
	registry        KnownLegacyDigestRegistry
	legacySelection LegacyRegistrySelection
	projection      HostAdapterProjection
	observations    []PathObservation
	paths           currentnessFixturePaths
}

func newCurrentnessFixture(t *testing.T, root string) currentnessFixture {
	t.Helper()
	paths := currentnessFixturePaths{
		current:  filepath.Join(root, "skills", "current.md"),
		outdated: filepath.Join(root, "skills", "outdated.md"),
		modified: filepath.Join(root, "skills", "modified.md"),
		orphaned: filepath.Join(root, "skills", "orphaned.md"),
		missing:  filepath.Join(root, "skills", "missing.md"),
		legacy:   filepath.Join(root, "skills", "legacy.md"),
		foreign:  filepath.Join(root, "skills", "foreign.md"),
		vacant:   filepath.Join(root, "skills", "vacant.md"),
	}
	prior := map[string][]byte{
		paths.current:  []byte("current"),
		paths.outdated: []byte("outdated-old"),
		paths.modified: []byte("modified-old"),
		paths.orphaned: []byte("orphaned"),
		paths.missing:  []byte("missing-owned"),
	}
	manifest := buildManifestFromPriorBytes(t, root, prior)
	legacyBytes := []byte("known legacy")
	registry, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
		Edition:     "legacy.v1",
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Host:        HostCodex,
		Scope:       ScopeProject,
		TargetRoots: []string{root},
		Paths: []KnownLegacyPath{{
			Path:      paths.legacy,
			Component: ComponentSkills,
			Digest:    digestBytes(legacyBytes),
		}},
	})
	if err != nil {
		t.Fatalf("BuildKnownLegacyDigestRegistry: %v", err)
	}
	legacySelection, err := WithKnownLegacyRegistry(registry)
	if err != nil {
		t.Fatalf("WithKnownLegacyRegistry: %v", err)
	}
	modifiedDesired := []byte("modified-by-user-to-next-version")
	foreignDesired := []byte("same desired bytes without receipt")
	currentOutputs := []RenderedOutput{
		mustOutput(t, paths.vacant, ComponentSkills, []byte("new vacant target")),
		mustOutput(t, paths.foreign, ComponentSkills, foreignDesired),
		mustOutput(t, paths.legacy, ComponentSkills, []byte("legacy successor")),
		mustOutput(t, paths.missing, ComponentSkills, prior[paths.missing]),
		mustOutput(t, paths.modified, ComponentSkills, modifiedDesired),
		mustOutput(t, paths.outdated, ComponentSkills, []byte("outdated-new")),
		mustOutput(t, paths.current, ComponentSkills, prior[paths.current]),
	}
	projection := buildCurrentProjection(t, root, currentOutputs)
	observations := []PathObservation{
		mustMissingObservation(t, paths.vacant, ComponentSkills),
		mustPresentObservation(t, paths.foreign, ComponentSkills, digestBytes(foreignDesired)),
		mustPresentObservation(t, paths.legacy, ComponentSkills, digestBytes(legacyBytes)),
		mustMissingObservation(t, paths.missing, ComponentSkills),
		mustPresentObservation(t, paths.orphaned, ComponentSkills, digestBytes(prior[paths.orphaned])),
		mustPresentObservation(t, paths.modified, ComponentSkills, digestBytes(modifiedDesired)),
		mustPresentObservation(t, paths.outdated, ComponentSkills, digestBytes(prior[paths.outdated])),
		mustPresentObservation(t, paths.current, ComponentSkills, digestBytes(prior[paths.current])),
	}
	return currentnessFixture{
		manifest:        manifest,
		registry:        registry,
		legacySelection: legacySelection,
		projection:      projection,
		observations:    observations,
		paths:           paths,
	}
}

func buildCurrentProjection(
	t *testing.T,
	root string,
	outputs []RenderedOutput,
) HostAdapterProjection {
	t.Helper()
	components := mustComponents(t, ComponentSkills)
	builder := NewHostAdapterProjectionBuilder(HostCodex)
	builder = builder.AtEdition("codex.v2")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(ScopeProject, components)
	builder = builder.AddTargetRoot(root)
	for _, output := range outputs {
		builder = builder.AddOutput(output)
	}
	builder = builder.RecoverWith(mustRecovery(t, HostCodex))
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterProjectionBuilder.Build: %v", err)
	}
	return projection
}

func buildManifestFromPriorBytes(
	t *testing.T,
	root string,
	prior map[string][]byte,
) InstallationManifest {
	t.Helper()
	components := mustComponents(t, ComponentSkills)
	recovery := mustRecovery(t, HostCodex)
	paths := make([]string, 0, len(prior))
	for path := range prior {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	builder := NewHostAdapterInstallPlanBuilder(HostCodex)
	builder = builder.AtEdition("codex.v1")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(ScopeProject, components)
	builder = builder.AddTargetRoot(root)
	for _, path := range paths {
		output := mustOutput(t, path, ComponentSkills, prior[path])
		builder = builder.AddOutput(mustMissing(t, path), output)
	}
	builder = builder.RecoverWith(recovery)
	plan, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterInstallPlanBuilder.Build: %v", err)
	}
	manifest, err := BuildInstallationManifest(plan)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	return manifest
}

func mustMissingObservation(
	t *testing.T,
	path string,
	component Component,
) PathObservation {
	t.Helper()
	observation, err := ObserveMissingPath(path, component)
	if err != nil {
		t.Fatalf("ObserveMissingPath: %v", err)
	}
	return observation
}

func mustPresentObservation(
	t *testing.T,
	path string,
	component Component,
	digest string,
) PathObservation {
	t.Helper()
	observation, err := ObservePresentPath(path, component, digest, 0o644)
	if err != nil {
		t.Fatalf("ObservePresentPath: %v", err)
	}
	return observation
}

func removeObservation(
	observations []PathObservation,
	path string,
) []PathObservation {
	result := make([]PathObservation, 0, len(observations)-1)
	for _, observation := range observations {
		if observation.path != path {
			result = append(result, observation)
		}
	}
	return result
}

func cloneObservations(observations []PathObservation) []PathObservation {
	return append([]PathObservation(nil), observations...)
}

func stateForPath(
	t *testing.T,
	report InstallationCurrentness,
	path string,
) PathCurrentnessKind {
	t.Helper()
	for _, currentness := range report.Paths() {
		if currentness.Path() == path {
			return currentness.Kind()
		}
	}
	t.Fatalf("path %s is absent from currentness report", path)
	return ""
}

func TestFirstInstallationHasNoSyntheticManifestOwnership(t *testing.T) {
	root := canonicalTempRoot(t)
	vacantPath := filepath.Join(root, "skills", "vacant.md")
	legacyPath := filepath.Join(root, "skills", "legacy.md")
	foreignPath := filepath.Join(root, "skills", "foreign.md")
	legacyBytes := []byte("known legacy")
	foreignBytes := []byte("desired bytes without ownership")
	projection := buildCurrentProjection(t, root, []RenderedOutput{
		mustOutput(t, vacantPath, ComponentSkills, []byte("vacant target")),
		mustOutput(t, legacyPath, ComponentSkills, []byte("legacy successor")),
		mustOutput(t, foreignPath, ComponentSkills, foreignBytes),
	})
	registry, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
		Edition:     "legacy.v1",
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Host:        HostCodex,
		Scope:       ScopeProject,
		TargetRoots: []string{root},
		Paths: []KnownLegacyPath{{
			Path:      legacyPath,
			Component: ComponentSkills,
			Digest:    digestBytes(legacyBytes),
		}},
	})
	if err != nil {
		t.Fatalf("BuildKnownLegacyDigestRegistry: %v", err)
	}
	legacySelection, err := WithKnownLegacyRegistry(registry)
	if err != nil {
		t.Fatalf("WithKnownLegacyRegistry: %v", err)
	}
	currentness, err := ClassifyFirstInstallationCurrentness(
		projection,
		[]PathObservation{
			mustMissingObservation(t, vacantPath, ComponentSkills),
			mustPresentObservation(t, legacyPath, ComponentSkills, digestBytes(legacyBytes)),
			mustPresentObservation(t, foreignPath, ComponentSkills, digestBytes(foreignBytes)),
		},
		legacySelection,
	)
	if err != nil {
		t.Fatalf("ClassifyFirstInstallationCurrentness: %v", err)
	}
	if currentness.Baseline() != InstallationBaselineNoPriorManifest ||
		currentness.ManifestBasis().valid() ||
		len(currentness.Manifest().CanonicalBytes()) != 0 {
		t.Fatalf("first installation fabricated manifest ownership: %#v", currentness)
	}
	if stateForPath(t, currentness, legacyPath) != PathKnownLegacyExact {
		t.Fatal("exact registered legacy path was not classified for adoption")
	}
	if stateForPath(t, currentness, foreignPath) != PathForeign {
		t.Fatal("desired-byte equality was treated as ownership on first install")
	}
	if len(currentness.VacantTargets()) != 1 || currentness.VacantTargets()[0].Path() != vacantPath {
		t.Fatalf("first-install vacant targets = %#v", currentness.VacantTargets())
	}
	plan, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileHostAdapterReconciliation: %v", err)
	}
	if len(plan.Conflicts()) != 1 || plan.Conflicts()[0].Path() != foreignPath {
		t.Fatalf("first-install foreign collision was not preserved: %#v", plan.Conflicts())
	}
	if _, err := BuildInstallationManifest(plan); err == nil {
		t.Fatal("blocked first-install plan produced an ownership manifest")
	}
	status, err := ProjectHostInstallationStatus(currentness)
	if err != nil {
		t.Fatalf("ProjectHostInstallationStatus: %v", err)
	}
	if status.ManifestPresence != InstallationManifestMissing ||
		status.Posture != HostInstallationBlocked {
		t.Fatalf("first-install status = %#v", status)
	}
}

func TestFirstInstallationBuildsManifestOnlyFromCleanPlan(t *testing.T) {
	root := canonicalTempRoot(t)
	firstPath := filepath.Join(root, "skills", "first.md")
	secondPath := filepath.Join(root, "skills", "second.md")
	projection := buildCurrentProjection(t, root, []RenderedOutput{
		mustOutput(t, firstPath, ComponentSkills, []byte("first")),
		mustOutput(t, secondPath, ComponentSkills, []byte("second")),
	})
	currentness, err := ClassifyFirstInstallationCurrentness(
		projection,
		[]PathObservation{
			mustMissingObservation(t, firstPath, ComponentSkills),
			mustMissingObservation(t, secondPath, ComponentSkills),
		},
		WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("ClassifyFirstInstallationCurrentness: %v", err)
	}
	plan, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileHostAdapterReconciliation: %v", err)
	}
	if len(plan.Conflicts()) != 0 || len(plan.Outputs()) != 2 {
		t.Fatalf("clean first-install plan = conflicts %d outputs %d", len(plan.Conflicts()), len(plan.Outputs()))
	}
	manifest, err := BuildInstallationManifest(plan)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	if len(manifest.RenderedPaths()) != 2 || !manifest.OwnershipBasis().valid() {
		t.Fatalf("first successful manifest = %#v", manifest)
	}
	status, err := ProjectHostInstallationStatus(currentness)
	if err != nil {
		t.Fatalf("ProjectHostInstallationStatus: %v", err)
	}
	if status.ManifestPresence != InstallationManifestMissing ||
		status.Posture != HostInstallationReconcileRequired {
		t.Fatalf("clean first-install status = %#v", status)
	}
}

func TestWholePathCurrentnessRejectsManagedFragmentProjection(t *testing.T) {
	root := canonicalTempRoot(t)
	carrierPath := filepath.Join(root, ".codex", "settings.json")
	fragment := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
		"semantic-merge-v1",
	)
	builder := baseManagedProjectionBuilder(t, root)
	builder = builder.AddManagedFragment(fragment)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("HostAdapterProjectionBuilder.Build: %v", err)
	}
	if _, err := ClassifyFirstInstallationCurrentness(
		projection,
		nil,
		WithoutKnownLegacyRegistry(),
	); err == nil || !strings.Contains(err.Error(), "managed fragment") {
		t.Fatalf("whole-path first-install currentness error = %v", err)
	}
	manifest, err := BuildProjectionInstallationManifest(projection)
	if err != nil {
		t.Fatalf("BuildProjectionInstallationManifest: %v", err)
	}
	if _, err := ClassifyInstallationCurrentness(
		manifest,
		projection,
		nil,
		WithoutKnownLegacyRegistry(),
	); err == nil || !strings.Contains(err.Error(), "managed fragment") {
		t.Fatalf("whole-path manifest currentness error = %v", err)
	}
}

func TestPathObservationConstructorsRejectIllegalStates(t *testing.T) {
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "skill.md")
	if _, err := ObserveMissingPath(path, Component("unknown")); err == nil {
		t.Fatal("missing observation accepted unknown component")
	}
	if _, err := ObservePresentPath(path, ComponentSkills, "bad", 0o644); err == nil {
		t.Fatal("present observation accepted invalid digest")
	}
	if _, err := ObservePresentPath(path, ComponentSkills, digestBytes([]byte("x")), 0); err == nil {
		t.Fatal("present observation accepted zero mode")
	}
}

func TestKnownLegacyRegistryIsCanonicalContentAddressedAndStrict(t *testing.T) {
	root := canonicalTempRoot(t)
	left := mustLegacyRegistryWithOrder(t, root, false)
	right := mustLegacyRegistryWithOrder(t, root, true)
	if left.Digest() != right.Digest() ||
		!bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("legacy registry insertion order changed its identity")
	}
	parsed, err := ParseKnownLegacyDigestRegistry(left.CanonicalBytes())
	if err != nil {
		t.Fatalf("ParseKnownLegacyDigestRegistry: %v", err)
	}
	if parsed.Ref() != left.Ref() || parsed.Digest() != left.Digest() {
		t.Fatal("parsed legacy registry changed identity")
	}
	var expanded map[string]any
	if err := json.Unmarshal(left.CanonicalBytes(), &expanded); err != nil {
		t.Fatalf("decode registry fixture: %v", err)
	}
	expanded["unknown"] = "field"
	unknown, err := json.Marshal(expanded)
	if err != nil {
		t.Fatalf("encode registry fixture: %v", err)
	}
	if _, err := ParseKnownLegacyDigestRegistry(unknown); err == nil {
		t.Fatal("legacy registry parser accepted unknown field")
	}
	pretty, err := json.MarshalIndent(expandedWithoutUnknown(expanded), "", "  ")
	if err != nil {
		t.Fatalf("encode pretty registry fixture: %v", err)
	}
	if _, err := ParseKnownLegacyDigestRegistry(pretty); err == nil {
		t.Fatal("legacy registry parser accepted non-canonical bytes")
	}
	before := left.Paths()
	changed := left.Paths()
	changed[0].Path = filepath.Join(root, "changed")
	if !reflect.DeepEqual(left.Paths(), before) {
		t.Fatal("legacy registry path getter exposed carrier storage")
	}
}

func mustLegacyRegistryWithOrder(
	t *testing.T,
	root string,
	reverse bool,
) KnownLegacyDigestRegistry {
	t.Helper()
	paths := []KnownLegacyPath{
		{
			Path:      filepath.Join(root, "a.md"),
			Component: ComponentSkills,
			Digest:    digestBytes([]byte("a")),
		},
		{
			Path:      filepath.Join(root, "b.md"),
			Component: ComponentSkills,
			Digest:    digestBytes([]byte("b")),
		},
	}
	if reverse {
		paths[0], paths[1] = paths[1], paths[0]
	}
	registry, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
		Edition:     "legacy.v1",
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Host:        HostCodex,
		Scope:       ScopeProject,
		TargetRoots: []string{root},
		Paths:       paths,
	})
	if err != nil {
		t.Fatalf("BuildKnownLegacyDigestRegistry: %v", err)
	}
	return registry
}
