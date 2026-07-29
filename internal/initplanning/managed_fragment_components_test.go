package initplanning

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestSharedCarrierPreservesExactPerFragmentComponents(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	carrierPath := filepath.Join(root, ".host", "config.json")
	mcpFragment := mustJSONObjectEntryFragmentForComponent(
		t,
		carrierPath,
		ComponentMCP,
		[]string{"mcpServers", "haft"},
		`{"command":"/usr/local/bin/haft","args":["serve"]}`,
	)
	skillsFragment := mustJSONObjectEntryFragmentForComponent(
		t,
		carrierPath,
		ComponentSkills,
		[]string{"skills", "externalRoot"},
		`"/project/.agents/skills"`,
	)
	components := mustComponents(
		t,
		ComponentSkills,
		ComponentMCP,
	)
	builder := NewHostAdapterProjectionBuilder(HostHermes)
	builder = builder.AtEdition("hermes.shared-components.v1")
	builder = builder.PublishedFrom(mustPublicationIdentity(t, root))
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(ScopeUser, components)
	builder = builder.AddTargetRoot(root)
	builder = builder.AddManagedFragment(mcpFragment)
	builder = builder.AddManagedFragment(skillsFragment)
	builder = builder.RecoverWith(mustRecovery(t, HostHermes))
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("build multi-component projection: %v", err)
	}
	carrier, err := NewPresentManagedCarrier(
		carrierPath,
		[]byte(`{"theme":"dark"}`),
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
		t.Fatalf("classify multi-component carrier: %v", err)
	}
	carriers := currentness.ManagedCarriers()
	if len(carriers) != 1 ||
		!slices.Equal(
			carriers[0].Components().Values(),
			[]Component{ComponentMCP, ComponentSkills},
		) {
		t.Fatalf("carrier components = %+v", carriers)
	}

	plan, err := CompileCoherentHostAdapterReconciliation(currentness)
	if err != nil {
		t.Fatalf("compile multi-component reconciliation: %v", err)
	}
	carrierPlans := plan.ManagedCarrierPlans()
	if len(carrierPlans) != 1 ||
		!slices.Equal(
			carrierPlans[0].Components().Values(),
			[]Component{ComponentMCP, ComponentSkills},
		) {
		t.Fatalf("carrier plan components = %+v", carrierPlans)
	}
	effectComponents := make(map[string]Component)
	for _, effect := range carrierPlans[0].Effects() {
		effectComponents[effect.Coordinate().Selector()] = effect.Component()
	}
	if effectComponents["/mcpServers/haft"] != ComponentMCP ||
		effectComponents["/skills/externalRoot"] != ComponentSkills {
		t.Fatalf("fragment effect components = %+v", effectComponents)
	}

	preview := previewHostPlan(plan)
	previewComponents := make(map[string]Component)
	for _, fragment := range preview.ManagedFragments {
		previewComponents[fragment.Selector] = fragment.Component
	}
	if previewComponents["/mcpServers/haft"] != ComponentMCP ||
		previewComponents["/skills/externalRoot"] != ComponentSkills {
		t.Fatalf(
			"fragment preview components = %+v",
			previewComponents,
		)
	}
	var carrierPreview FileEffectPreview
	for _, effect := range preview.Effects {
		if effect.Path == carrierPath {
			carrierPreview = effect
		}
	}
	if !slices.Equal(
		carrierPreview.Components,
		[]Component{ComponentMCP, ComponentSkills},
	) {
		t.Fatalf(
			"shared carrier preview components = %+v",
			carrierPreview.Components,
		)
	}

	batch, err := BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build multi-component publication batch: %v", err)
	}
	if !slices.Equal(
		batch.Manifest().Components(),
		[]Component{ComponentMCP, ComponentSkills},
	) {
		t.Fatalf(
			"manifest components = %+v",
			batch.Manifest().Components(),
		)
	}
	manifestComponents := make(map[string]Component)
	for _, fragment := range batch.Manifest().ManagedFragments() {
		manifestComponents[fragment.Selector] = fragment.Component
	}
	if manifestComponents["/mcpServers/haft"] != ComponentMCP ||
		manifestComponents["/skills/externalRoot"] != ComponentSkills {
		t.Fatalf(
			"manifest fragment components = %+v",
			manifestComponents,
		)
	}
	steps := batch.Steps()
	if len(steps) != 1 ||
		!slices.Equal(
			steps[0].Components().Values(),
			[]Component{ComponentMCP, ComponentSkills},
		) {
		t.Fatalf("publication steps = %+v", steps)
	}
	observationPlan, err := BuildHostPublicationObservationPlan(batch)
	if err != nil {
		t.Fatalf("build publication observation plan: %v", err)
	}
	targets := observationPlan.Targets()
	if len(targets) != 1 ||
		!slices.Equal(
			targets[0].Components().Values(),
			[]Component{ComponentMCP, ComponentSkills},
		) {
		t.Fatalf("publication observation targets = %+v", targets)
	}
	matching, err := ObservePresentPathForComponents(
		carrierPath,
		components,
		carrier.Digest(),
		carrier.Mode(),
	)
	if err != nil {
		t.Fatalf("build exact component-set observation: %v", err)
	}
	admission, err := ValidateHostPublicationPreconditions(
		batch,
		[]PathObservation{matching},
	)
	if err != nil ||
		admission.Kind() != HostPublicationPreconditionsMatched {
		t.Fatalf(
			"matching component-set admission = %+v, err=%v",
			admission,
			err,
		)
	}
	wrongComponents := mustPresentObservation(
		t,
		carrierPath,
		ComponentMCP,
		carrier.Digest(),
	)
	if _, err := ValidateHostPublicationPreconditions(
		batch,
		[]PathObservation{wrongComponents},
	); err == nil {
		t.Fatal("publication admitted a collapsed shared-carrier component")
	}
}

func mustJSONObjectEntryFragmentForComponent(
	t *testing.T,
	carrierPath string,
	component Component,
	selector []string,
	value string,
) ManagedFragment {
	t.Helper()
	fragment, err := NewJSONObjectEntryFragment(
		carrierPath,
		component,
		selector,
		[]byte(value),
		0o600,
		"json-merge-v1",
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	return fragment
}
