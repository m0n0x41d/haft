package initplanning

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestFirstCoherentInstallationPublishesWholeOutputsAndExactSharedFragments(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	skillPath := filepath.Join(root, ".codex", "skills", "h-reason", "SKILL.md")
	carrierPath := filepath.Join(root, ".codex", "config.json")
	skill := mustOutput(
		t,
		skillPath,
		ComponentSkills,
		[]byte("canonical skill"),
	)
	fragment := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"/usr/local/bin/haft","args":["serve"]}`,
		"json-merge-v1",
	)
	projection := buildCoherentProjection(
		t,
		root,
		skill,
		fragment,
	)
	carrier, err := NewPresentManagedCarrier(
		carrierPath,
		[]byte(`{"theme":"dark"}`),
		0o640,
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier: %v", err)
	}
	currentness, err := ClassifyFirstCoherentInstallationCurrentness(
		projection,
		[]PathObservation{
			mustMissingObservation(t, skillPath, ComponentSkills),
		},
		[]ManagedCarrierInput{carrier},
		WithoutKnownLegacyRegistry(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("ClassifyFirstCoherentInstallationCurrentness: %v", err)
	}
	plan, err := CompileCoherentHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileCoherentHostAdapterReconciliation: %v", err)
	}
	if len(plan.ManagedCarrierPlans()) != 1 ||
		len(plan.ManagedFragments()) != 1 ||
		len(plan.ManagedFragmentConflicts()) != 0 {
		t.Fatalf(
			"coherent plan carriers=%d fragments=%d conflicts=%d",
			len(plan.ManagedCarrierPlans()),
			len(plan.ManagedFragments()),
			len(plan.ManagedFragmentConflicts()),
		)
	}
	preview := previewHostPlan(plan)
	if len(preview.ManagedFragments) != 1 ||
		preview.ManagedFragments[0].CarrierPath != carrierPath ||
		preview.ManagedFragments[0].Effect != ManagedFragmentCreate {
		t.Fatalf(
			"managed fragment preview = %+v",
			preview.ManagedFragments,
		)
	}
	previewEffects := make(map[string]FileEffectPreview)
	for _, effect := range preview.Effects {
		previewEffects[effect.Path] = effect
	}
	if !previewEffects[carrierPath].SharedCarrier ||
		previewEffects[carrierPath].Effect != FileUpdate {
		t.Fatalf(
			"shared carrier preview = %+v",
			previewEffects[carrierPath],
		)
	}
	batch, err := BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("BuildHostPublicationBatch: %v", err)
	}
	if batch.Manifest().Schema() != installationManifestSchemaV2 {
		t.Fatalf("manifest schema = %s", batch.Manifest().Schema())
	}
	rendered := batch.Manifest().RenderedPaths()
	if len(rendered) != 1 || rendered[0].Path != skillPath {
		t.Fatalf("manifest whole paths = %+v", rendered)
	}
	managed := batch.Manifest().ManagedFragments()
	if len(managed) != 1 || managed[0].CarrierPath != carrierPath {
		t.Fatalf("manifest managed fragments = %+v", managed)
	}
	if batch.ManifestPredecessor().Kind() != ManifestPredecessorMissing {
		t.Fatalf(
			"first coherent manifest predecessor = %+v",
			batch.ManifestPredecessor(),
		)
	}
	steps := publicationStepsByPath(batch.Steps())
	carrierStep, exists := steps[carrierPath]
	if !exists ||
		!carrierStep.IsManagedCarrier() ||
		carrierStep.Kind() != PublicationReplace ||
		carrierStep.Expectation().Kind() != PredecessorSharedCarrierExact {
		t.Fatalf("shared carrier step = %+v", carrierStep)
	}
	carrierOutput, available := carrierStep.Output()
	if !available || carrierOutput.Mode() != 0o640 {
		t.Fatalf("shared carrier output = %+v, available=%t", carrierOutput, available)
	}
	var merged map[string]any
	if err := json.Unmarshal(carrierOutput.Content(), &merged); err != nil {
		t.Fatalf("decode merged carrier: %v", err)
	}
	if merged["theme"] != "dark" {
		t.Fatalf("shared user field was not preserved: %s", carrierOutput.Content())
	}
	servers, ok := merged["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("merged mcpServers = %#v", merged["mcpServers"])
	}
	if _, ok := servers["haft"].(map[string]any); !ok {
		t.Fatalf("merged Haft fragment = %#v", servers["haft"])
	}
}

func TestCoherentInstallationUsesV2ManifestAsFragmentOwnershipAndCASBasis(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	skillPath := filepath.Join(root, ".codex", "skills", "h-reason", "SKILL.md")
	carrierPath := filepath.Join(root, ".codex", "config.json")
	skill := mustOutput(
		t,
		skillPath,
		ComponentSkills,
		[]byte("canonical skill"),
	)
	priorFragment := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"/old/haft","args":["serve"]}`,
		"json-merge-v1",
	)
	priorProjection := buildCoherentProjection(
		t,
		root,
		skill,
		priorFragment,
	)
	priorCarrier, err := NewPresentManagedCarrier(
		carrierPath,
		[]byte(`{"theme":"dark"}`),
		0o640,
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier(prior): %v", err)
	}
	firstCurrentness, err := ClassifyFirstCoherentInstallationCurrentness(
		priorProjection,
		[]PathObservation{
			mustMissingObservation(t, skillPath, ComponentSkills),
		},
		[]ManagedCarrierInput{priorCarrier},
		WithoutKnownLegacyRegistry(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify first coherent install: %v", err)
	}
	firstPlan, err := CompileCoherentHostAdapterReconciliation(firstCurrentness)
	if err != nil {
		t.Fatalf("compile first coherent install: %v", err)
	}
	firstBatch, err := BuildHostPublicationBatch(firstPlan)
	if err != nil {
		t.Fatalf("build first coherent batch: %v", err)
	}
	firstSteps := publicationStepsByPath(firstBatch.Steps())
	firstCarrierOutput, available := firstSteps[carrierPath].Output()
	if !available {
		t.Fatal("first coherent batch omitted merged carrier output")
	}

	currentFragment := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"/current/haft","args":["serve"]}`,
		"json-merge-v1",
	)
	currentProjection := buildCoherentProjection(
		t,
		root,
		skill,
		currentFragment,
	)
	observedCarrier, err := NewPresentManagedCarrier(
		carrierPath,
		firstCarrierOutput.Content(),
		firstCarrierOutput.Mode(),
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier(current): %v", err)
	}
	currentness, err := ClassifyCoherentInstallationCurrentness(
		firstBatch.Manifest(),
		currentProjection,
		[]PathObservation{
			mustPresentObservation(
				t,
				skillPath,
				ComponentSkills,
				skill.Digest(),
			),
		},
		[]ManagedCarrierInput{observedCarrier},
		WithoutKnownLegacyRegistry(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("ClassifyCoherentInstallationCurrentness: %v", err)
	}
	plan, err := CompileCoherentHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileCoherentHostAdapterReconciliation: %v", err)
	}
	batch, err := BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("BuildHostPublicationBatch(current): %v", err)
	}
	if batch.ManifestPredecessor().Kind() != ManifestPredecessorExact ||
		batch.ManifestPredecessor().Ref() != firstBatch.Manifest().Ref() ||
		batch.ManifestPredecessor().Digest() != firstBatch.Manifest().Digest() {
		t.Fatalf(
			"coherent manifest predecessor = %+v",
			batch.ManifestPredecessor(),
		)
	}
	carrierStep := publicationStepsByPath(batch.Steps())[carrierPath]
	if carrierStep.Kind() != PublicationReplace ||
		carrierStep.Expectation().Kind() != PredecessorSharedCarrierExact {
		t.Fatalf("current shared carrier step = %+v", carrierStep)
	}
	if batch.Manifest().ManagedFragments()[0].Digest != currentFragment.Digest() {
		t.Fatalf(
			"current managed manifest = %+v",
			batch.Manifest().ManagedFragments(),
		)
	}
}

func TestCoherentInstallationPreservesForeignFragmentCoordinate(t *testing.T) {
	root := canonicalTempRoot(t)
	carrierPath := filepath.Join(root, ".codex", "config.json")
	fragment := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"/current/haft","args":["serve"]}`,
		"json-merge-v1",
	)
	projection := buildCoherentProjection(
		t,
		root,
		RenderedOutput{},
		fragment,
	)
	carrier, err := NewPresentManagedCarrier(
		carrierPath,
		[]byte(`{"mcpServers":{"haft":{"command":"/operator/haft"}}}`),
		0o600,
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier: %v", err)
	}
	currentness, err := ClassifyFirstCoherentInstallationCurrentness(
		projection,
		nil,
		[]ManagedCarrierInput{carrier},
		WithoutKnownLegacyRegistry(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("ClassifyFirstCoherentInstallationCurrentness: %v", err)
	}
	plan, err := CompileCoherentHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("CompileCoherentHostAdapterReconciliation: %v", err)
	}
	if len(plan.ManagedFragmentConflicts()) != 1 {
		t.Fatalf(
			"managed fragment conflicts = %+v",
			plan.ManagedFragmentConflicts(),
		)
	}
	if _, err := BuildHostPublicationBatch(plan); err == nil {
		t.Fatal("foreign shared fragment became a publication batch")
	}
}

func buildCoherentProjection(
	t *testing.T,
	root string,
	output RenderedOutput,
	fragment ManagedFragment,
) HostAdapterProjection {
	t.Helper()
	builder := NewHostAdapterProjectionBuilder(HostCodex)
	builder = builder.AtEdition("codex.coherent.v1")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	components := []Component{ComponentMCP}
	if output.Path() != "" {
		components = append(components, ComponentSkills)
	}
	builder = builder.WithSelection(
		ScopeProject,
		mustComponents(t, components...),
	)
	builder = builder.AddTargetRoot(root)
	if output.Path() != "" {
		builder = builder.AddOutput(output)
	}
	builder = builder.AddManagedFragment(fragment)
	builder = builder.RecoverWith(mustRecovery(t, HostCodex))
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("build coherent projection: %v", err)
	}
	return projection
}
