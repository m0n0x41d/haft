package initplanning

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestFirstInstallationObservationPlanSeparatesRequiredAndOptionalLegacyPaths(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	desiredPath := filepath.Join(root, "skills", "desired.md")
	retiredLegacyPath := filepath.Join(root, "skills", "retired.md")
	desiredBytes := []byte("desired")
	projection := buildCurrentProjection(t, root, []RenderedOutput{
		mustOutput(t, desiredPath, ComponentSkills, desiredBytes),
	})
	registry, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
		Edition:     "legacy.v1",
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Host:        HostCodex,
		Scope:       ScopeProject,
		TargetRoots: []string{root},
		Paths: []KnownLegacyPath{
			{
				Path:      desiredPath,
				Component: ComponentSkills,
				Digest:    digestBytes(desiredBytes),
			},
			{
				Path:      retiredLegacyPath,
				Component: ComponentSkills,
				Digest:    digestBytes([]byte("retired")),
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildKnownLegacyDigestRegistry: %v", err)
	}
	selection, err := WithKnownLegacyRegistry(registry)
	if err != nil {
		t.Fatalf("WithKnownLegacyRegistry: %v", err)
	}
	plan, err := BuildFirstInstallationObservationPlan(projection, selection)
	if err != nil {
		t.Fatalf("BuildFirstInstallationObservationPlan: %v", err)
	}
	targets := plan.Targets()
	if len(targets) != 2 {
		t.Fatalf("observation targets = %#v", targets)
	}
	byPath := map[string]ObservationTarget{}
	for _, target := range targets {
		byPath[target.Path()] = target
	}
	if byPath[desiredPath].Requirement() != ObservationRequired {
		t.Fatal("desired first-install path is not required")
	}
	if byPath[retiredLegacyPath].Requirement() != ObservationIfPresent {
		t.Fatal("legacy-only path is not optional when absent")
	}
	before := plan.Targets()
	changed := plan.Targets()
	changed[0].path = filepath.Join(root, "changed")
	if !reflect.DeepEqual(plan.Targets(), before) {
		t.Fatal("observation plan exposed mutable targets")
	}
}

func TestInstalledObservationPlanRequiresManifestOrphansAndDesiredVacancies(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	fixture := newCurrentnessFixture(t, root)
	plan, err := BuildInstalledObservationPlan(
		fixture.manifest,
		fixture.projection,
		fixture.legacySelection,
	)
	if err != nil {
		t.Fatalf("BuildInstalledObservationPlan: %v", err)
	}
	byPath := map[string]ObservationTarget{}
	for _, target := range plan.Targets() {
		byPath[target.Path()] = target
	}
	for _, path := range []string{fixture.paths.orphaned, fixture.paths.vacant} {
		target, exists := byPath[path]
		if !exists || target.Requirement() != ObservationRequired {
			t.Fatalf("required installed target %s = %#v", path, target)
		}
	}
}

func TestFirstInstallationRejectsLegacyAdoptionAcrossComponents(t *testing.T) {
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "skills", "candidate.md")
	bytes := []byte("known bytes")
	projection := buildCurrentProjection(t, root, []RenderedOutput{
		mustOutput(t, path, ComponentSkills, bytes),
	})
	registry, err := BuildKnownLegacyDigestRegistry(KnownLegacyDigestRegistryInput{
		Edition:     "legacy.v1",
		ProjectRoot: root,
		ProjectID:   "qnt_e3149c17",
		Host:        HostCodex,
		Scope:       ScopeProject,
		TargetRoots: []string{root},
		Paths: []KnownLegacyPath{{
			Path:      path,
			Component: ComponentMCP,
			Digest:    digestBytes(bytes),
		}},
	})
	if err != nil {
		t.Fatalf("BuildKnownLegacyDigestRegistry: %v", err)
	}
	selection, err := WithKnownLegacyRegistry(registry)
	if err != nil {
		t.Fatalf("WithKnownLegacyRegistry: %v", err)
	}
	_, err = ClassifyFirstInstallationCurrentness(
		projection,
		[]PathObservation{
			mustPresentObservation(t, path, ComponentSkills, digestBytes(bytes)),
		},
		selection,
	)
	if err == nil {
		t.Fatal("first installation adopted a legacy witness from another component")
	}
}
