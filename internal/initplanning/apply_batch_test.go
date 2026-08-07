package initplanning

import (
	"path/filepath"
	"testing"
)

func TestBuildHostPublicationBatchPreservesTypedEffectsAndManifestBaseline(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	currentness, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		fixture.projection,
		fixture.observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("classify currentness: %v", err)
	}
	plan, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile reconciliation: %v", err)
	}
	if _, err := BuildHostPublicationBatch(plan); err == nil {
		t.Fatal("blocked reconciliation became a publication batch")
	}

	cleaned := publicationBatchFixtureWithoutConflicts(t, fixture)
	batch, err := BuildHostPublicationBatch(cleaned.plan)
	if err != nil {
		t.Fatalf("build publication batch: %v", err)
	}
	if batch.ManifestPredecessor().Kind() != ManifestPredecessorExact ||
		batch.ManifestPredecessor().Digest() != fixture.manifest.Digest() {
		t.Fatalf("manifest predecessor = %#v", batch.ManifestPredecessor())
	}
	byPath := publicationStepsByPath(batch.Steps())
	want := map[string]HostPublicationStepKind{
		fixture.paths.current:  PublicationPreserve,
		fixture.paths.outdated: PublicationReplace,
		fixture.paths.legacy:   PublicationReplace,
		fixture.paths.orphaned: PublicationRemove,
		fixture.paths.missing:  PublicationCreate,
		fixture.paths.vacant:   PublicationCreate,
	}
	for path, kind := range want {
		step, exists := byPath[path]
		if !exists || step.Kind() != kind {
			t.Fatalf("step %s = %#v, want %s", path, step, kind)
		}
		if step.Component() != ComponentSkills {
			t.Fatalf("step %s component = %s", path, step.Component())
		}
	}
	if batch.Manifest().Digest() == fixture.manifest.Digest() {
		t.Fatal("new publication manifest unexpectedly equals its predecessor")
	}
	repeated, err := BuildHostPublicationBatch(cleaned.plan)
	if err != nil {
		t.Fatalf("repeat publication batch: %v", err)
	}
	if batch.Digest() == "" || repeated.Digest() != batch.Digest() {
		t.Fatalf("batch digests = %q / %q", batch.Digest(), repeated.Digest())
	}
}

func TestBuildHostPublicationBatchModelsFreshCreateAndExactLegacyAdoption(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "skills", "h-reason", "SKILL.md")
	content := []byte("known bytes")
	output, err := NewRenderedOutput(path, ComponentSkills, content, 0o644)
	if err != nil {
		t.Fatalf("build rendered output: %v", err)
	}
	projection := buildCurrentProjection(t, root, []RenderedOutput{output})

	t.Run("fresh create", func(t *testing.T) {
		missing := mustMissingObservation(t, path, ComponentSkills)
		currentness, err := ClassifyFirstInstallationCurrentness(
			projection,
			[]PathObservation{missing},
			WithoutKnownLegacyRegistry(),
		)
		if err != nil {
			t.Fatalf("classify fresh currentness: %v", err)
		}
		plan, err := CompileHostAdapterReconciliation(currentness)
		if err != nil {
			t.Fatalf("compile fresh plan: %v", err)
		}
		batch, err := BuildHostPublicationBatch(plan)
		if err != nil {
			t.Fatalf("build fresh batch: %v", err)
		}
		if batch.ManifestPredecessor().Kind() != ManifestPredecessorMissing {
			t.Fatalf("fresh manifest predecessor = %#v", batch.ManifestPredecessor())
		}
		steps := batch.Steps()
		if len(steps) != 1 || steps[0].Kind() != PublicationCreate {
			t.Fatalf("fresh steps = %#v", steps)
		}
	})

	t.Run("exact legacy adoption", func(t *testing.T) {
		registry, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
			Edition:     "legacy.v1",
			ProjectRoot: root,
			ProjectID:   projection.ProjectID().String(),
			Host:        projection.Host(),
			Scope:       projection.Scope(),
			TargetRoots: projection.TargetRoots(),
			Paths: []KnownLegacyPath{{
				Path:      path,
				Component: ComponentSkills,
				Digest:    output.Digest(),
			}},
		})
		if err != nil {
			t.Fatalf("build legacy registry: %v", err)
		}
		selection, err := WithKnownLegacyRegistry(registry)
		if err != nil {
			t.Fatalf("select legacy registry: %v", err)
		}
		present := mustPresentObservation(t, path, ComponentSkills, output.Digest())
		currentness, err := ClassifyFirstInstallationCurrentness(
			projection,
			[]PathObservation{present},
			selection,
		)
		if err != nil {
			t.Fatalf("classify legacy currentness: %v", err)
		}
		plan, err := CompileHostAdapterReconciliation(currentness)
		if err != nil {
			t.Fatalf("compile legacy plan: %v", err)
		}
		batch, err := BuildHostPublicationBatch(plan)
		if err != nil {
			t.Fatalf("build legacy batch: %v", err)
		}
		steps := batch.Steps()
		if len(steps) != 1 || steps[0].Kind() != PublicationAdoptLegacy {
			t.Fatalf("legacy steps = %#v", steps)
		}
		if batch.ManifestPredecessor().Kind() != ManifestPredecessorMissing {
			t.Fatalf("legacy manifest predecessor = %#v", batch.ManifestPredecessor())
		}
	})
}

func TestHostPublicationObservationAndPreconditionAdmissionAreExact(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	cleaned := publicationBatchFixtureWithoutConflicts(t, fixture)
	batch, err := BuildHostPublicationBatch(cleaned.plan)
	if err != nil {
		t.Fatalf("build publication batch: %v", err)
	}
	observationPlan, err := BuildHostPublicationObservationPlan(batch)
	if err != nil {
		t.Fatalf("build publication observation plan: %v", err)
	}
	if len(observationPlan.Targets()) != len(batch.Steps()) {
		t.Fatalf(
			"observation targets = %d, want %d",
			len(observationPlan.Targets()),
			len(batch.Steps()),
		)
	}
	admission, err := ValidateHostPublicationPreconditions(
		batch,
		cleaned.observations,
	)
	if err != nil {
		t.Fatalf("validate matching preconditions: %v", err)
	}
	if admission.Kind() != HostPublicationPreconditionsMatched {
		t.Fatalf("matching admission = %#v", admission)
	}

	changed := cloneObservations(cleaned.observations)
	for index, observation := range changed {
		if observation.Path() != fixture.paths.outdated {
			continue
		}
		changed[index] = mustPresentObservation(
			t,
			observation.Path(),
			observation.Component(),
			digestBytes([]byte("raced")),
		)
	}
	admission, err = ValidateHostPublicationPreconditions(batch, changed)
	if err != nil {
		t.Fatalf("validate changed preconditions: %v", err)
	}
	if admission.Kind() != HostPublicationPreconditionsChanged ||
		len(admission.Changes()) != 1 ||
		admission.Changes()[0].Path() != fixture.paths.outdated {
		t.Fatalf("changed admission = %#v", admission)
	}
	if _, err := ValidateHostPublicationPreconditions(
		batch,
		changed[:len(changed)-1],
	); err == nil {
		t.Fatal("incomplete precondition evidence was accepted")
	}
	duplicate := append(cloneObservations(cleaned.observations), cleaned.observations[0])
	if _, err := ValidateHostPublicationPreconditions(batch, duplicate); err == nil {
		t.Fatal("duplicate precondition evidence was accepted")
	}
}

type publicationBatchFixture struct {
	plan         HostAdapterInstallPlan
	observations []PathObservation
}

func publicationBatchFixtureWithoutConflicts(
	t *testing.T,
	fixture currentnessFixture,
) publicationBatchFixture {
	t.Helper()
	outputs := make([]RenderedOutput, 0, len(fixture.projection.Outputs())-1)
	for _, output := range fixture.projection.Outputs() {
		if output.Path() == fixture.paths.foreign {
			continue
		}
		outputs = append(outputs, output)
	}
	projection := buildCurrentProjection(t, fixture.projection.ProjectRoot(), outputs)
	manifestPaths := make(map[string]ManifestPath, len(fixture.manifest.RenderedPaths()))
	for _, manifestPath := range fixture.manifest.RenderedPaths() {
		manifestPaths[manifestPath.Path] = manifestPath
	}
	observations := make([]PathObservation, 0, len(fixture.observations)-1)
	for _, observation := range fixture.observations {
		if observation.Path() == fixture.paths.foreign {
			continue
		}
		if observation.Path() == fixture.paths.modified {
			manifestPath := manifestPaths[observation.Path()]
			observation = mustPresentObservation(
				t,
				observation.Path(),
				observation.Component(),
				manifestPath.Digest,
			)
		}
		observations = append(observations, observation)
	}
	currentness, err := ClassifyInstallationCurrentness(
		fixture.manifest,
		projection,
		observations,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("classify cleaned currentness: %v", err)
	}
	plan, err := CompileHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile cleaned reconciliation: %v", err)
	}
	if len(plan.Conflicts()) != 0 {
		t.Fatalf("cleaned plan conflicts = %#v", plan.Conflicts())
	}
	return publicationBatchFixture{
		plan:         plan,
		observations: observations,
	}
}

func publicationStepsByPath(
	steps []HostPublicationStep,
) map[string]HostPublicationStep {
	result := make(map[string]HostPublicationStep, len(steps))
	for _, step := range steps {
		result[step.Path()] = step
	}
	return result
}
