package initfs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestHostStatusInspectorReadsExactCurrentBindingWithoutMutation(
	t *testing.T,
) {
	root := t.TempDir()
	content := []byte("current skill")
	carrierPath := filepath.Join(root, "skills", "h-reason", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(carrierPath), 0o755); err != nil {
		t.Fatalf("create carrier directory: %v", err)
	}
	if err := os.WriteFile(carrierPath, content, 0o644); err != nil {
		t.Fatalf("write carrier: %v", err)
	}
	projection := mustObservationProjection(
		t,
		root,
		[]initplanning.RenderedOutput{
			mustObservationOutput(t, carrierPath, content),
		},
	)
	manifest := mustManifestForContent(t, root, content)
	manifestPath := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	store := mustManifestStore(t, root, manifestPath)
	if _, err := store.Persist(manifest, ExpectManifestMissing()); err != nil {
		t.Fatalf("persist manifest fixture: %v", err)
	}
	before := snapshotTreeMetadata(t, root)
	inspector := mustHostStatusInspector(t)
	inspection, err := inspector.InspectBinding(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("InspectBinding: %v", err)
	}
	status := inspection.Status()
	if inspection.ManifestPath() != manifestPath ||
		inspection.ManifestReadKind() != ManifestReadPresent ||
		status.Posture != initplanning.HostInstallationCurrent ||
		status.ManifestDigest != manifest.Digest() ||
		len(status.Paths) != 1 ||
		status.Paths[0].State != initplanning.PathCurrentOwned {
		t.Fatalf("current binding inspection = %#v", inspection)
	}
	after := snapshotTreeMetadata(t, root)
	if !slices.Equal(before, after) {
		t.Fatalf("read-only status changed filesystem metadata:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestHostStatusInspectorReportsMissingManifestWithoutCreatingItsParent(
	t *testing.T,
) {
	root := t.TempDir()
	carrierPath := filepath.Join(root, "skills", "h-reason", "SKILL.md")
	projection := mustObservationProjection(
		t,
		root,
		[]initplanning.RenderedOutput{
			mustObservationOutput(t, carrierPath, []byte("desired")),
		},
	)
	manifestPath := filepath.Join(root, ".haft", "host-installations", "codex.project.json")
	store := mustManifestStore(t, root, manifestPath)
	inspection, err := mustHostStatusInspector(t).InspectBinding(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("InspectBinding missing manifest: %v", err)
	}
	status := inspection.Status()
	if inspection.ManifestReadKind() != ManifestReadMissing ||
		status.ManifestPresence != initplanning.InstallationManifestMissing ||
		status.Posture != initplanning.HostInstallationReconcileRequired ||
		!slices.Contains(status.Reasons, "installation_manifest_missing") ||
		len(status.VacantTargets) != 1 {
		t.Fatalf("missing binding status = %#v", status)
	}
	if _, err := os.Lstat(filepath.Dir(manifestPath)); !os.IsNotExist(err) {
		t.Fatalf("read-only missing-manifest status created a parent: %v", err)
	}
}

func TestHostStatusInspectorReadsFragmentOnlyCoherentBindingWithoutMutation(
	t *testing.T,
) {
	root := t.TempDir()
	carrierPath := filepath.Join(root, ".codex", "config.json")
	if err := os.MkdirAll(filepath.Dir(carrierPath), 0o755); err != nil {
		t.Fatalf("create shared-carrier parent: %v", err)
	}
	if err := os.WriteFile(
		carrierPath,
		[]byte(`{"theme":"dark"}`),
		0o640,
	); err != nil {
		t.Fatalf("write shared carrier: %v", err)
	}
	projection := mustFragmentOnlyStatusProjection(
		t,
		root,
		carrierPath,
	)
	manifestPath := filepath.Join(
		root,
		".haft",
		"host-installations",
		"codex.project.json",
	)
	store := mustManifestStore(t, root, manifestPath)
	before := snapshotTreeMetadata(t, root)
	inspection, err := mustHostStatusInspector(t).InspectCoherentBinding(
		store,
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
		initplanning.NoManagedFragmentLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("InspectCoherentBinding: %v", err)
	}
	status := inspection.Status()
	if inspection.ManifestReadKind() != ManifestReadMissing ||
		status.ManifestPresence != initplanning.InstallationManifestMissing ||
		status.Posture != initplanning.HostInstallationReconcileRequired ||
		len(status.Paths) != 0 ||
		len(status.ManagedFragments) != 0 ||
		len(status.VacantManagedFragments) != 1 ||
		status.VacantManagedFragments[0].CarrierPath != carrierPath {
		t.Fatalf("fragment-only coherent status = %#v", status)
	}
	after := snapshotTreeMetadata(t, root)
	if !slices.Equal(before, after) {
		t.Fatalf(
			"fragment-only read-only status changed filesystem metadata:\nbefore=%v\nafter=%v",
			before,
			after,
		)
	}
	if _, err := os.Lstat(filepath.Dir(manifestPath)); !os.IsNotExist(err) {
		t.Fatalf(
			"fragment-only read-only status created a manifest parent: %v",
			err,
		)
	}
}

func TestHostStatusInspectorDiscoversDuplicateSkillRootsWithExactEvidence(
	t *testing.T,
) {
	parent := t.TempDir()
	leftRoot := filepath.Join(parent, "user-skills")
	rightRoot := filepath.Join(parent, "project-skills")
	leftProjection := mustStatusSkillProjection(t, leftRoot)
	rightProjection := mustStatusSkillProjection(t, rightRoot)
	leftPlan, err := initplanning.BuildSkillRootObservationPlan(
		leftProjection,
		initplanning.ScopeUser,
	)
	if err != nil {
		t.Fatalf("build left skill-root plan: %v", err)
	}
	rightPlan, err := initplanning.BuildSkillRootObservationPlan(
		rightProjection,
		initplanning.ScopeProject,
	)
	if err != nil {
		t.Fatalf("build right skill-root plan: %v", err)
	}
	writeProjectedSkills(t, leftProjection)
	writeProjectedSkills(t, rightProjection)
	status, err := mustHostStatusInspector(t).InspectSkillRoots(
		nil,
		[]initplanning.SkillRootObservationPlan{rightPlan, leftPlan},
	)
	if err != nil {
		t.Fatalf("InspectSkillRoots: %v", err)
	}
	if len(status.Observations()) != 2 ||
		len(status.ActiveRoots()) != 2 ||
		len(status.Duplicates()) != 1 {
		t.Fatalf(
			"skill-root status counts = observations:%d active:%d duplicates:%d",
			len(status.Observations()),
			len(status.ActiveRoots()),
			len(status.Duplicates()),
		)
	}
	duplicate := status.Duplicates()[0]
	if duplicate.SkillName != "h-reason" ||
		duplicate.LeftOrigin != initplanning.SkillRootDiscovered ||
		duplicate.RightOrigin != initplanning.SkillRootDiscovered ||
		duplicate.LeftEvidenceDigest == "" ||
		duplicate.RightEvidenceDigest == "" ||
		duplicate.LeftRoot == duplicate.RightRoot {
		t.Fatalf("duplicate skill-root evidence = %#v", duplicate)
	}
}

func TestHostStatusInspectorKeepsEmptyAndSymlinkedDiscoveryDistinct(
	t *testing.T,
) {
	parent := t.TempDir()
	emptyRoot := filepath.Join(parent, "empty")
	projection := mustStatusSkillProjection(t, emptyRoot)
	plan, err := initplanning.BuildSkillRootObservationPlan(
		projection,
		initplanning.ScopeProject,
	)
	if err != nil {
		t.Fatalf("build empty skill-root plan: %v", err)
	}
	status, err := mustHostStatusInspector(t).InspectSkillRoots(
		nil,
		[]initplanning.SkillRootObservationPlan{plan},
	)
	if err != nil {
		t.Fatalf("inspect empty root: %v", err)
	}
	if len(status.Observations()) != 1 ||
		status.Observations()[0].ObservedCount() != 0 ||
		len(status.ActiveRoots()) != 0 ||
		len(status.Duplicates()) != 0 {
		t.Fatalf("empty skill-root status = %#v", status)
	}

	realRoot := filepath.Join(parent, "real")
	linkedRoot := filepath.Join(parent, "linked")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatalf("create real root: %v", err)
	}
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatalf("create linked root: %v", err)
	}
	linkedProjection := mustStatusSkillProjection(t, linkedRoot)
	linkedPlan, err := initplanning.BuildSkillRootObservationPlan(
		linkedProjection,
		initplanning.ScopeProject,
	)
	if err != nil {
		t.Fatalf("build linked skill-root plan: %v", err)
	}
	if _, err := mustHostStatusInspector(t).InspectSkillRoots(
		nil,
		[]initplanning.SkillRootObservationPlan{linkedPlan},
	); err == nil {
		t.Fatal("symlinked discovery root was treated as an empty safe root")
	}
}

func mustHostStatusInspector(t *testing.T) HostStatusInspector {
	t.Helper()
	inspector, err := NewHostStatusInspector(1 << 20)
	if err != nil {
		t.Fatalf("NewHostStatusInspector: %v", err)
	}
	return inspector
}

func mustFragmentOnlyStatusProjection(
	t *testing.T,
	root string,
	carrierPath string,
) initplanning.HostAdapterProjection {
	t.Helper()
	fragment, err := initplanning.NewJSONObjectEntryFragment(
		carrierPath,
		initplanning.ComponentMCP,
		[]string{"mcpServers", "haft"},
		[]byte(`{"command":"haft","args":["serve"]}`),
		0o640,
		"json-merge-v1",
	)
	if err != nil {
		t.Fatalf("NewJSONObjectEntryFragment: %v", err)
	}
	publication, err := initplanning.NewPublicationIdentity(
		initplanning.PublicationIdentityInput{
			HaftVersion:         "v9-test",
			ExecutablePath:      filepath.Join(root, "bin", "haft"),
			ExecutableDigest:    "sha256:" + strings.Repeat("a", 64),
			SkillBundleDigest:   "sha256:" + strings.Repeat("b", 64),
			KernelCatalogDigest: "sha256:" + strings.Repeat("c", 64),
		},
	)
	if err != nil {
		t.Fatalf("NewPublicationIdentity: %v", err)
	}
	components, err := initplanning.ParseComponentSet(
		[]string{string(initplanning.ComponentMCP)},
	)
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	recovery, err := initplanning.NewRecoveryOperation(
		[]string{"haft", "init", "--codex", "--local"},
	)
	if err != nil {
		t.Fatalf("NewRecoveryOperation: %v", err)
	}
	builder := initplanning.NewHostAdapterProjectionBuilder(
		initplanning.HostCodex,
	)
	builder = builder.AtEdition("codex.fragment-only.v1")
	builder = builder.PublishedFrom(publication)
	builder = builder.ForProject(root, "qnt_e3149c17")
	builder = builder.WithSelection(
		initplanning.ScopeProject,
		components,
	)
	builder = builder.AddTargetRoot(root)
	builder = builder.AddManagedFragment(fragment)
	builder = builder.RecoverWith(recovery)
	projection, err := builder.Build()
	if err != nil {
		t.Fatalf("build fragment-only projection: %v", err)
	}
	return projection
}

func mustStatusSkillProjection(
	t *testing.T,
	root string,
) initplanning.SkillComponentProjection {
	t.Helper()
	content := []byte("---\nname: h-reason\ndescription: source\n---\nbody\n")
	bundle, err := initplanning.BuildSkillSourceBundle(
		"skills.v1",
		"sha256:"+strings.Repeat("a", 64),
		[]initplanning.SkillSourceInput{{
			Name:             "h-reason",
			Description:      "Reason",
			InvocationPolicy: initplanning.SkillInvocationImplicitAllowed,
			Content:          content,
		}},
	)
	if err != nil {
		t.Fatalf("BuildSkillSourceBundle: %v", err)
	}
	rewrite, err := initplanning.NewSkillRewriteSet(
		"codex.exact.v1",
		nil,
	)
	if err != nil {
		t.Fatalf("NewSkillRewriteSet: %v", err)
	}
	renderer, err := initplanning.NewSkillComponentRenderer(
		initplanning.HostCodex,
		"codex.skills.v1",
		nil,
		initplanning.SkillPolicyInSourceFrontmatter,
		rewrite,
	)
	if err != nil {
		t.Fatalf("NewSkillComponentRenderer: %v", err)
	}
	projection, err := renderer.Render(bundle, root)
	if err != nil {
		t.Fatalf("SkillComponentRenderer.Render: %v", err)
	}
	return projection
}

func writeProjectedSkills(
	t *testing.T,
	projection initplanning.SkillComponentProjection,
) {
	t.Helper()
	for _, output := range projection.Outputs() {
		if err := os.MkdirAll(filepath.Dir(output.Path()), 0o755); err != nil {
			t.Fatalf("create projected skill parent: %v", err)
		}
		if err := os.WriteFile(
			output.Path(),
			output.Content(),
			output.Mode().Perm(),
		); err != nil {
			t.Fatalf("write projected skill: %v", err)
		}
	}
}

func snapshotTreeMetadata(
	t *testing.T,
	root string,
) []string {
	t.Helper()
	snapshot := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		snapshot = append(
			snapshot,
			path+"\x00"+info.Mode().String()+"\x00"+info.ModTime().UTC().String(),
		)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree metadata: %v", err)
	}
	slices.Sort(snapshot)
	return snapshot
}
