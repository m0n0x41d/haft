package initplanning

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestProjectHostInstallationStatusReportsBlockedPathsAndExactIdentities(
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
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	status, err := ProjectHostInstallationStatus(currentness)
	if err != nil {
		t.Fatalf("ProjectHostInstallationStatus: %v", err)
	}
	if status.Posture != HostInstallationBlocked {
		t.Fatalf("status posture = %s, want %s", status.Posture, HostInstallationBlocked)
	}
	for _, reason := range []string{
		"adapter_edition_changed",
		"path_foreign",
		"path_locally_modified_owned",
		"vacant_desired_path",
	} {
		if !slices.Contains(status.Reasons, reason) {
			t.Fatalf("status reasons %v omit %s", status.Reasons, reason)
		}
	}
	if status.ManifestRef != fixture.manifest.Ref() ||
		status.ManifestDigest != fixture.manifest.Digest() {
		t.Fatalf("status manifest identity = %s %s", status.ManifestRef, status.ManifestDigest)
	}
	if status.InstalledPublication.SkillBundleDigest != fixture.manifest.SkillBundleDigest() ||
		status.DesiredPublication.SkillBundleDigest != fixture.projection.Publication().SkillBundleDigest() {
		t.Fatal("status lost installed or desired publication identity")
	}
	if len(status.Paths) != 7 || len(status.VacantTargets) != 1 {
		t.Fatalf("status paths/vacant = %d/%d", len(status.Paths), len(status.VacantTargets))
	}
	if !slices.Equal(status.ReconcileArgv, fixture.projection.Recovery().Argv()) {
		t.Fatalf("status reconcile argv = %v", status.ReconcileArgv)
	}
}

func TestProjectHostInstallationStatusIsCurrentOnlyOnExactMetadataAndPaths(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "skills", "h-reason.md")
	prior := map[string][]byte{path: []byte("current skill")}
	manifest := buildManifestFromPriorBytes(t, root, prior)
	output := mustOutput(t, path, ComponentSkills, prior[path])
	projection := buildProjectionWithPublication(
		t,
		root,
		"codex.v1",
		mustPublicationIdentity(t, root),
		[]RenderedOutput{output},
	)
	observation := mustPresentObservation(t, path, ComponentSkills, output.Digest())
	currentness, err := ClassifyInstallationCurrentness(
		manifest,
		projection,
		[]PathObservation{observation},
		WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	status, err := ProjectHostInstallationStatus(currentness)
	if err != nil {
		t.Fatalf("ProjectHostInstallationStatus: %v", err)
	}
	if status.Posture != HostInstallationCurrent || len(status.Reasons) != 0 {
		t.Fatalf("exact installation status = %s %v", status.Posture, status.Reasons)
	}
	if len(status.Paths) != 1 || status.Paths[0].State != PathCurrentOwned {
		t.Fatalf("exact path status = %+v", status.Paths)
	}
}

func TestProjectHostInstallationStatusDetectsMetadataDriftWithCurrentBytes(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	path := filepath.Join(root, "skills", "h-reason.md")
	prior := map[string][]byte{path: []byte("current skill")}
	manifest := buildManifestFromPriorBytes(t, root, prior)
	output := mustOutput(t, path, ComponentSkills, prior[path])
	driftedPublication, err := NewPublicationIdentity(PublicationIdentityInput{
		HaftVersion:         "v9.next",
		ExecutablePath:      filepath.Join(root, "bin", "haft-next"),
		ExecutableDigest:    digestBytes([]byte("next binary")),
		SkillBundleDigest:   digestBytes([]byte("next skill bundle")),
		KernelCatalogDigest: digestBytes([]byte("next kernel catalog")),
	})
	if err != nil {
		t.Fatalf("NewPublicationIdentity: %v", err)
	}
	projection := buildProjectionWithPublication(
		t,
		root,
		"codex.v2",
		driftedPublication,
		[]RenderedOutput{output},
	)
	observation := mustPresentObservation(t, path, ComponentSkills, output.Digest())
	currentness, err := ClassifyInstallationCurrentness(
		manifest,
		projection,
		[]PathObservation{observation},
		WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("ClassifyInstallationCurrentness: %v", err)
	}
	if stateForPath(t, currentness, path) != PathCurrentOwned {
		t.Fatal("metadata drift incorrectly changed byte currentness")
	}
	status, err := ProjectHostInstallationStatus(currentness)
	if err != nil {
		t.Fatalf("ProjectHostInstallationStatus: %v", err)
	}
	if status.Posture != HostInstallationReconcileRequired {
		t.Fatalf("metadata-drift posture = %s", status.Posture)
	}
	for _, reason := range []string{
		"adapter_edition_changed",
		"haft_version_changed",
		"executable_path_changed",
		"executable_digest_changed",
		"skill_bundle_changed",
		"kernel_catalog_changed",
	} {
		if !slices.Contains(status.Reasons, reason) {
			t.Fatalf("metadata reasons %v omit %s", status.Reasons, reason)
		}
	}
}

func TestProjectCoherentHostInstallationStatusIncludesExactManagedFragmentState(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	carrierPath := filepath.Join(root, ".codex", "config.json")
	fragment := mustProjectionJSONObjectEntryFragment(
		t,
		carrierPath,
		[]string{"mcpServers", "haft"},
		`{"command":"haft","args":["serve"]}`,
		"json-merge-v1",
	)
	projection := buildCoherentProjection(
		t,
		root,
		RenderedOutput{},
		fragment,
	)
	unmanagedCarrier, err := NewPresentManagedCarrier(
		carrierPath,
		[]byte(`{"theme":"dark"}`),
		0o640,
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier(unmanaged): %v", err)
	}
	first, err := ClassifyFirstCoherentInstallationCurrentness(
		projection,
		nil,
		[]ManagedCarrierInput{unmanagedCarrier},
		WithoutKnownLegacyRegistry(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify first coherent installation: %v", err)
	}
	plan, err := CompileCoherentHostAdapterReconciliation(first)
	if err != nil {
		t.Fatalf("compile first coherent reconciliation: %v", err)
	}
	batch, err := BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build first coherent batch: %v", err)
	}
	step := publicationStepsByPath(batch.Steps())[carrierPath]
	output, available := step.Output()
	if !available {
		t.Fatal("coherent batch omitted shared-carrier output")
	}
	installedCarrier, err := NewPresentManagedCarrier(
		carrierPath,
		output.Content(),
		output.Mode(),
	)
	if err != nil {
		t.Fatalf("NewPresentManagedCarrier(installed): %v", err)
	}
	currentness, err := ClassifyCoherentInstallationCurrentness(
		batch.Manifest(),
		projection,
		nil,
		[]ManagedCarrierInput{installedCarrier},
		WithoutKnownLegacyRegistry(),
		NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("classify installed coherent installation: %v", err)
	}
	status, err := ProjectCoherentHostInstallationStatus(currentness)
	if err != nil {
		t.Fatalf("ProjectCoherentHostInstallationStatus: %v", err)
	}
	if status.Posture != HostInstallationCurrent ||
		len(status.Reasons) != 0 ||
		len(status.Paths) != 0 ||
		len(status.ManagedFragments) != 1 ||
		len(status.VacantManagedFragments) != 0 {
		t.Fatalf("coherent status envelope = %#v", status)
	}
	managed := status.ManagedFragments[0]
	if managed.CarrierPath != carrierPath ||
		managed.Component != ComponentMCP ||
		managed.Kind != ManagedJSONObjectEntry ||
		managed.Selector != "/mcpServers/haft" ||
		managed.State != ManagedFragmentCurrentOwned ||
		managed.ObservedDigest != fragment.Digest() ||
		managed.ManifestDigest != fragment.Digest() ||
		managed.DesiredDigest != fragment.Digest() ||
		managed.OwnershipRef != batch.Manifest().Ref() {
		t.Fatalf("managed fragment status = %#v", managed)
	}
}

func buildProjectionWithPublication(
	t *testing.T,
	root string,
	edition string,
	publication PublicationIdentity,
	outputs []RenderedOutput,
) HostAdapterProjection {
	t.Helper()
	builder := NewHostAdapterProjectionBuilder(HostCodex)
	builder = builder.AtEdition(edition)
	builder = builder.PublishedFrom(publication)
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(
		ScopeProject,
		mustComponents(t, ComponentSkills),
	)
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
