package initplanning

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"
)

func TestSkillRootObservationProjectsBoundedActiveExposureEvidence(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	projection := mustSkillRootProjection(t, root, []string{"haft"})
	plan, err := BuildSkillRootObservationPlan(
		projection,
		ScopeProject,
	)
	if err != nil {
		t.Fatalf("BuildSkillRootObservationPlan: %v", err)
	}
	if plan.Root() != filepath.Join(root, "haft") ||
		plan.Host() != HostCodex ||
		plan.Scope() != ScopeProject {
		t.Fatalf("skill-root plan identity = %s %s %s", plan.Root(), plan.Host(), plan.Scope())
	}
	if len(plan.Exposures()) != 2 ||
		len(plan.FilesystemPlan().Targets()) != 2 {
		t.Fatalf("skill-root plan exposure counts = %d/%d", len(plan.Exposures()), len(plan.FilesystemPlan().Targets()))
	}
	present := plan.Exposures()[1]
	observation, err := ObservePresentPath(
		present.Path(),
		ComponentSkills,
		digestBytes([]byte("observed skill")),
		0o640,
	)
	if err != nil {
		t.Fatalf("ObservePresentPath: %v", err)
	}
	projected, err := ProjectSkillRootObservation(
		plan,
		[]PathObservation{observation},
	)
	if err != nil {
		t.Fatalf("ProjectSkillRootObservation: %v", err)
	}
	if projected.ExpectedCount() != 2 ||
		projected.ObservedCount() != 1 ||
		projected.EvidenceRef() == "" ||
		projected.EvidenceDigest() == "" {
		t.Fatalf("skill-root observation identity = %#v", projected)
	}
	active, exists := projected.ActiveRoot()
	if !exists {
		t.Fatal("present skill carrier did not activate the discovery root")
	}
	if active.Origin() != SkillRootDiscovered ||
		active.EvidenceRef() != projected.EvidenceRef() ||
		active.EvidenceDigest() != projected.EvidenceDigest() ||
		!slices.Equal(active.Exposures(), []SkillExposure{present}) {
		t.Fatalf("active discovered root = %#v", active)
	}
}

func TestSkillRootObservationIsDeterministicAndKeepsEmptyRootInactive(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	projection := mustSkillRootProjection(t, root, nil)
	plan, err := BuildSkillRootObservationPlan(
		projection,
		ScopeUser,
	)
	if err != nil {
		t.Fatalf("BuildSkillRootObservationPlan: %v", err)
	}
	exposures := plan.Exposures()
	left := []PathObservation{
		mustSkillRootPresentObservation(t, exposures[1], "second"),
		mustSkillRootPresentObservation(t, exposures[0], "first"),
	}
	right := slices.Clone(left)
	slices.Reverse(right)
	leftResult, err := ProjectSkillRootObservation(plan, left)
	if err != nil {
		t.Fatalf("ProjectSkillRootObservation left: %v", err)
	}
	rightResult, err := ProjectSkillRootObservation(plan, right)
	if err != nil {
		t.Fatalf("ProjectSkillRootObservation right: %v", err)
	}
	if leftResult.EvidenceDigest() != rightResult.EvidenceDigest() ||
		!bytes.Equal(leftResult.CanonicalBytes(), rightResult.CanonicalBytes()) {
		t.Fatal("filesystem observation order changed skill-root evidence")
	}
	empty, err := ProjectSkillRootObservation(plan, nil)
	if err != nil {
		t.Fatalf("ProjectSkillRootObservation empty: %v", err)
	}
	if empty.ObservedCount() != 0 {
		t.Fatalf("empty root observed count = %d", empty.ObservedCount())
	}
	if _, exists := empty.ActiveRoot(); exists {
		t.Fatal("empty discovery root became active")
	}
	if empty.EvidenceDigest() == leftResult.EvidenceDigest() {
		t.Fatal("empty and active root observations share an evidence identity")
	}
}

func TestSkillRootObservationRejectsUnplannedAndMissingObservations(
	t *testing.T,
) {
	root := canonicalTempRoot(t)
	projection := mustSkillRootProjection(t, root, nil)
	plan, err := BuildSkillRootObservationPlan(
		projection,
		ScopeProject,
	)
	if err != nil {
		t.Fatalf("BuildSkillRootObservationPlan: %v", err)
	}
	unplanned, err := ObservePresentPath(
		filepath.Join(root, "h-other", "SKILL.md"),
		ComponentSkills,
		digestBytes([]byte("other")),
		0o644,
	)
	if err != nil {
		t.Fatalf("ObservePresentPath unplanned: %v", err)
	}
	if _, err := ProjectSkillRootObservation(
		plan,
		[]PathObservation{unplanned},
	); err == nil {
		t.Fatal("unplanned skill carrier entered root discovery evidence")
	}
	missing, err := ObserveMissingPath(
		plan.Exposures()[0].Path(),
		ComponentSkills,
	)
	if err != nil {
		t.Fatalf("ObserveMissingPath: %v", err)
	}
	if _, err := ProjectSkillRootObservation(
		plan,
		[]PathObservation{missing},
	); err == nil {
		t.Fatal("explicit missing observation entered if-present root evidence")
	}
}

func mustSkillRootProjection(
	t *testing.T,
	root string,
	prefix []string,
) SkillComponentProjection {
	t.Helper()
	bundle, err := BuildSkillSourceBundle(
		"skills.v1",
		digestBytes([]byte("kernel catalog")),
		[]SkillSourceInput{
			implicitSkillInput("h-reason", "Reason"),
			manualSkillInput("h-decide", "Decide"),
		},
	)
	if err != nil {
		t.Fatalf("BuildSkillSourceBundle: %v", err)
	}
	rewrite := mustSkillRewriteSet(t, "codex.exact.v1", nil)
	renderer := mustSkillRenderer(
		t,
		HostCodex,
		"codex.skills.v1",
		prefix,
		SkillPolicyInSourceFrontmatter,
		rewrite,
	)
	projection, err := renderer.Render(bundle, root)
	if err != nil {
		t.Fatalf("SkillComponentRenderer.Render: %v", err)
	}
	return projection
}

func mustSkillRootPresentObservation(
	t *testing.T,
	exposure SkillExposure,
	content string,
) PathObservation {
	t.Helper()
	observation, err := ObservePresentPath(
		exposure.Path(),
		ComponentSkills,
		digestBytes([]byte(content)),
		0o644,
	)
	if err != nil {
		t.Fatalf("ObservePresentPath: %v", err)
	}
	return observation
}
