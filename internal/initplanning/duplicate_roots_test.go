package initplanning

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestFindDuplicateSkillRootsRelatesGlobalAndProjectExposureWithoutOwnershipInference(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	manifest, err := BuildInstallationManifest(
		mustManifestAdapterPlan(t, root, false),
	)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	projectSkillPath := filepath.Join(
		root,
		".agents",
		"skills",
		"h-reason",
		"SKILL.md",
	)
	projectExposure := mustSkillExposure(t, "h-reason", projectSkillPath)
	projectRoot, err := NewManifestSkillRoot(
		filepath.Join(root, ".agents", "skills"),
		HostCodex,
		ScopeProject,
		manifest,
		[]SkillExposure{projectExposure},
	)
	if err != nil {
		t.Fatalf("NewManifestSkillRoot: %v", err)
	}
	globalRootPath := filepath.Join(root, "user", ".agents", "skills")
	globalRoot, err := NewDiscoveredSkillRoot(
		globalRootPath,
		HostCodex,
		ScopeUser,
		"skill-root-observation:fixture",
		digestBytes([]byte("global root scan")),
		[]SkillExposure{
			mustSkillExposure(
				t,
				"h-reason",
				filepath.Join(globalRootPath, "h-reason", "SKILL.md"),
			),
			mustSkillExposure(
				t,
				"h-spec",
				filepath.Join(globalRootPath, "h-spec", "SKILL.md"),
			),
		},
	)
	if err != nil {
		t.Fatalf("NewDiscoveredSkillRoot: %v", err)
	}
	duplicates, err := FindDuplicateSkillRoots([]ActiveSkillRoot{
		globalRoot,
		projectRoot,
	})
	if err != nil {
		t.Fatalf("FindDuplicateSkillRoots: %v", err)
	}
	if len(duplicates) != 1 {
		t.Fatalf("duplicate count = %d, want 1: %+v", len(duplicates), duplicates)
	}
	duplicate := duplicates[0]
	if duplicate.SkillName != "h-reason" {
		t.Fatalf("duplicate skill = %s", duplicate.SkillName)
	}
	origins := map[SkillRootOrigin]bool{
		duplicate.LeftOrigin:  true,
		duplicate.RightOrigin: true,
	}
	if !origins[SkillRootManifestOwned] || !origins[SkillRootDiscovered] {
		t.Fatalf("duplicate origins = %v", origins)
	}
	if duplicate.LeftEvidenceRef == "" || duplicate.RightEvidenceRef == "" {
		t.Fatal("duplicate relation lost evidence identity")
	}
	if globalRoot.Origin() != SkillRootDiscovered {
		t.Fatal("filesystem discovery was promoted to manifest ownership")
	}
}

func TestDuplicateSkillRootRelationIsDeterministicAndPairwise(t *testing.T) {
	root := canonicalTempRoot(t)
	roots := []ActiveSkillRoot{
		mustDiscoveredSkillRoot(t, filepath.Join(root, "c"), ScopeUser, "h-reason"),
		mustDiscoveredSkillRoot(t, filepath.Join(root, "a"), ScopeProject, "h-reason"),
		mustDiscoveredSkillRoot(t, filepath.Join(root, "b"), ScopeProject, "h-reason"),
	}
	left, err := FindDuplicateSkillRoots(roots)
	if err != nil {
		t.Fatalf("FindDuplicateSkillRoots: %v", err)
	}
	right, err := FindDuplicateSkillRoots([]ActiveSkillRoot{
		roots[2],
		roots[0],
		roots[1],
	})
	if err != nil {
		t.Fatalf("FindDuplicateSkillRoots reordered: %v", err)
	}
	if len(left) != 3 {
		t.Fatalf("three roots produced %d pairs, want 3", len(left))
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("root input order changed relations\nleft: %+v\nright: %+v", left, right)
	}
}

func TestSkillRootConstructorsRejectUnprovenOrAmbiguousExposure(t *testing.T) {
	root := canonicalTempRoot(t)
	manifest, err := BuildInstallationManifest(
		mustManifestAdapterPlan(t, root, false),
	)
	if err != nil {
		t.Fatalf("BuildInstallationManifest: %v", err)
	}
	skillRoot := filepath.Join(root, ".agents", "skills")
	foreignExposure := mustSkillExposure(
		t,
		"h-foreign",
		filepath.Join(skillRoot, "h-foreign", "SKILL.md"),
	)
	if _, err := NewManifestSkillRoot(
		skillRoot,
		HostCodex,
		ScopeProject,
		manifest,
		[]SkillExposure{foreignExposure},
	); err == nil {
		t.Fatal("manifest root accepted an exposure absent from the manifest")
	}
	duplicateName := []SkillExposure{
		mustSkillExposure(t, "h-reason", filepath.Join(skillRoot, "one", "SKILL.md")),
		mustSkillExposure(t, "h-reason", filepath.Join(skillRoot, "two", "SKILL.md")),
	}
	if _, err := NewDiscoveredSkillRoot(
		skillRoot,
		HostCodex,
		ScopeProject,
		"skill-root-observation:fixture",
		digestBytes([]byte("scan")),
		duplicateName,
	); err == nil {
		t.Fatal("discovered root accepted duplicate skill names")
	}
	if _, err := NewSkillExposure("bad/name", filepath.Join(skillRoot, "bad")); err == nil {
		t.Fatal("skill exposure accepted an invalid name")
	}
	if _, err := NewDiscoveredSkillRoot(
		skillRoot,
		HostCodex,
		ScopeProject,
		"skill-root-observation:fixture",
		"bad-digest",
		[]SkillExposure{foreignExposure},
	); err == nil {
		t.Fatal("discovered root accepted an invalid observation digest")
	}
}

func TestFindDuplicateSkillRootsRejectsRepeatedRootObservation(t *testing.T) {
	root := canonicalTempRoot(t)
	observed := mustDiscoveredSkillRoot(t, root, ScopeProject, "h-reason")
	if _, err := FindDuplicateSkillRoots([]ActiveSkillRoot{observed, observed}); err == nil {
		t.Fatal("duplicate-root projection accepted the same observation twice")
	}
	before := observed.Exposures()
	changed := observed.Exposures()
	changed[0].name = "changed"
	if !reflect.DeepEqual(observed.Exposures(), before) {
		t.Fatal("skill-root exposure getter exposed carrier storage")
	}
}

func mustSkillExposure(
	t *testing.T,
	name string,
	path string,
) SkillExposure {
	t.Helper()
	exposure, err := NewSkillExposure(name, path)
	if err != nil {
		t.Fatalf("NewSkillExposure: %v", err)
	}
	return exposure
}

func mustDiscoveredSkillRoot(
	t *testing.T,
	root string,
	scope InstallScope,
	skill string,
) ActiveSkillRoot {
	t.Helper()
	exposure := mustSkillExposure(
		t,
		skill,
		filepath.Join(root, skill, "SKILL.md"),
	)
	observed, err := NewDiscoveredSkillRoot(
		root,
		HostCodex,
		scope,
		"skill-root-observation:"+skill,
		digestBytes([]byte(root+skill)),
		[]SkillExposure{exposure},
	)
	if err != nil {
		t.Fatalf("NewDiscoveredSkillRoot: %v", err)
	}
	return observed
}
