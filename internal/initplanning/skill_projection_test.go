package initplanning

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSkillComponentRendererDerivesHostCarriersFromOneBundle(t *testing.T) {
	bundle := mustSkillProjectionBundle(t)
	exactRewrite := mustSkillRewriteSet(t, "claude.exact.v1", nil)
	codexRewrite := mustSkillRewriteSet(
		t,
		"codex.syntax.v1",
		[]SkillRewriteInput{{From: "/h-", To: "$h-"}},
	)
	claudeRenderer := mustSkillRenderer(
		t,
		HostClaude,
		"claude.skills.v1",
		nil,
		SkillPolicyInSourceFrontmatter,
		exactRewrite,
	)
	codexRenderer := mustSkillRenderer(
		t,
		HostCodex,
		"codex.skills.v1",
		nil,
		SkillPolicyCodexOpenAIYAML,
		codexRewrite,
	)
	root := t.TempDir()
	claude, err := claudeRenderer.Render(bundle, filepath.Join(root, "claude"))
	if err != nil {
		t.Fatalf("render Claude projection: %v", err)
	}
	codex, err := codexRenderer.Render(bundle, filepath.Join(root, "codex"))
	if err != nil {
		t.Fatalf("render Codex projection: %v", err)
	}

	report, err := VerifySkillDerivationParity(
		bundle,
		[]SkillComponentProjection{codex, claude},
	)
	if err != nil {
		t.Fatalf("verify derivation parity: %v", err)
	}
	if report.BundleRef != bundle.Ref() ||
		report.BundleDigest != bundle.Digest() ||
		report.KernelCatalogDigest != bundle.KernelCatalogDigest() {
		t.Fatalf("parity report lost source identity: %#v", report)
	}
	if !slices.Equal(report.Hosts, []HostID{HostClaude, HostCodex}) {
		t.Fatalf("parity hosts = %v", report.Hosts)
	}
	if len(report.Projections) != 2 || report.SkillCount != 2 {
		t.Fatalf("parity projection summary = %#v", report)
	}
	if len(claude.Outputs()) != 2 || len(codex.Outputs()) != 4 {
		t.Fatalf("unexpected output counts: Claude=%d Codex=%d", len(claude.Outputs()), len(codex.Outputs()))
	}
	assertProjectedSkillContent(t, claude, "h-alpha", "/h-alpha")
	assertProjectedSkillContent(t, codex, "h-alpha", "$h-alpha")
	assertCodexPolicyContent(t, codex, "h-alpha", true)
	assertCodexPolicyContent(t, codex, "h-bind", false)

	records := codex.Records()
	records[0].Name = "mutated"
	if codex.Records()[0].Name == "mutated" {
		t.Fatal("projection records leaked mutable state")
	}
	outputs := codex.Outputs()
	content := outputs[0].Content()
	content[0] = 'X'
	if bytes.Equal(content, codex.Outputs()[0].Content()) {
		t.Fatal("projection outputs leaked mutable content")
	}
}

func TestVerifySkillDerivationParityRejectsMalformedProjection(t *testing.T) {
	bundle := mustSkillProjectionBundle(t)
	rewrite := mustSkillRewriteSet(t, "codex.syntax.v1", nil)
	renderer := mustSkillRenderer(
		t,
		HostCodex,
		"codex.skills.v1",
		nil,
		SkillPolicyCodexOpenAIYAML,
		rewrite,
	)
	projection, err := renderer.Render(bundle, t.TempDir())
	if err != nil {
		t.Fatalf("render projection: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(SkillComponentProjection) SkillComponentProjection
		message string
	}{
		{
			name: "duplicate record",
			mutate: func(candidate SkillComponentProjection) SkillComponentProjection {
				candidate.records[1] = candidate.records[0]
				return candidate
			},
			message: "repeats skill",
		},
		{
			name: "wrong source bundle",
			mutate: func(candidate SkillComponentProjection) SkillComponentProjection {
				candidate.bundleRef = "skill-source-bundle:wrong"
				return candidate
			},
			message: "another source bundle",
		},
		{
			name: "missing policy output",
			mutate: func(candidate SkillComponentProjection) SkillComponentProjection {
				candidate.outputs = candidate.outputs[:len(candidate.outputs)-1]
				return candidate
			},
			message: "output coverage is incomplete",
		},
		{
			name: "output escapes root",
			mutate: func(candidate SkillComponentProjection) SkillComponentProjection {
				candidate.outputs[0].path = filepath.Join(filepath.Dir(candidate.root), "escape", "SKILL.md")
				return candidate
			},
			message: "projection output",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneSkillProjectionForTest(projection)
			candidate = test.mutate(candidate)
			_, err := VerifySkillDerivationParity(bundle, []SkillComponentProjection{candidate})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestSkillRewriteSetIsDeterministicAndClosed(t *testing.T) {
	rules := []SkillRewriteInput{
		{From: "source-a", To: "target-a"},
		{From: "source-b", To: "target-b"},
	}
	first := mustSkillRewriteSet(t, "host.rules.v1", rules)
	second := mustSkillRewriteSet(t, "host.rules.v1", rules)
	if first.Digest() != second.Digest() {
		t.Fatalf("equal rewrite sets differ: %s != %s", first.Digest(), second.Digest())
	}
	rendered, err := first.Apply([]byte("source-a/source-b"))
	if err != nil {
		t.Fatalf("apply rewrite set: %v", err)
	}
	if string(rendered) != "target-a/target-b" {
		t.Fatalf("rendered = %q", rendered)
	}
	exact := mustSkillRewriteSet(t, "host.exact.v1", nil)
	input := []byte("unchanged")
	output, err := exact.Apply(input)
	if err != nil {
		t.Fatalf("apply exact rewrite: %v", err)
	}
	output[0] = 'X'
	if string(input) != "unchanged" {
		t.Fatal("exact rewrite leaked mutable source bytes")
	}

	invalid := [][]SkillRewriteInput{
		{{From: "", To: "target"}},
		{{From: "same", To: "same"}},
		{{From: "repeat", To: "one"}, {From: "repeat", To: "two"}},
	}
	for _, candidate := range invalid {
		if _, err := NewSkillRewriteSet("host.invalid.v1", candidate); err == nil {
			t.Fatalf("invalid rewrite set was accepted: %#v", candidate)
		}
	}
}

func mustSkillProjectionBundle(t *testing.T) SkillSourceBundle {
	t.Helper()
	kernelDigest := "sha256:" + strings.Repeat("a", 64)
	bundle, err := BuildSkillSourceBundle(
		"test-skills.v1",
		kernelDigest,
		[]SkillSourceInput{
			{
				Name:             "h-alpha",
				Description:      "Implicit test skill",
				InvocationPolicy: SkillInvocationImplicitAllowed,
				Content:          []byte("---\nname: h-alpha\ndescription: Implicit test skill\n---\nUse /h-alpha.\n"),
			},
			{
				Name:             "h-bind",
				Description:      "Manual test skill",
				InvocationPolicy: SkillInvocationManualOnly,
				Content:          []byte("---\nname: h-bind\ndescription: Manual test skill\ndisable-model-invocation: true\n---\nUse /h-bind.\n"),
			},
		},
	)
	if err != nil {
		t.Fatalf("build skill projection bundle: %v", err)
	}
	return bundle
}

func mustSkillRewriteSet(
	t *testing.T,
	id string,
	rules []SkillRewriteInput,
) SkillRewriteSet {
	t.Helper()
	rewrite, err := NewSkillRewriteSet(id, rules)
	if err != nil {
		t.Fatalf("build rewrite set: %v", err)
	}
	return rewrite
}

func mustSkillRenderer(
	t *testing.T,
	host HostID,
	edition string,
	prefix []string,
	policy SkillPolicyCarrierKind,
	rewrite SkillRewriteSet,
) SkillComponentRenderer {
	t.Helper()
	renderer, err := NewSkillComponentRenderer(
		host,
		edition,
		prefix,
		policy,
		rewrite,
	)
	if err != nil {
		t.Fatalf("build skill renderer: %v", err)
	}
	return renderer
}

func assertProjectedSkillContent(
	t *testing.T,
	projection SkillComponentProjection,
	skillName string,
	marker string,
) {
	t.Helper()
	for _, record := range projection.Records() {
		if record.Name != skillName {
			continue
		}
		for _, output := range projection.Outputs() {
			if output.Path() == record.RenderedSkillPath && strings.Contains(string(output.Content()), marker) {
				return
			}
		}
	}
	t.Fatalf("skill %s projection lacks marker %q", skillName, marker)
}

func assertCodexPolicyContent(
	t *testing.T,
	projection SkillComponentProjection,
	skillName string,
	allowImplicit bool,
) {
	t.Helper()
	expected := "policy:\n  allow_implicit_invocation: "
	expected += map[bool]string{true: "true\n", false: "false\n"}[allowImplicit]
	for _, record := range projection.Records() {
		if record.Name != skillName {
			continue
		}
		for _, output := range projection.Outputs() {
			if output.Path() == record.RenderedPolicyPath && string(output.Content()) == expected {
				return
			}
		}
	}
	t.Fatalf("skill %s Codex policy differs from %q", skillName, expected)
}

func cloneSkillProjectionForTest(
	projection SkillComponentProjection,
) SkillComponentProjection {
	projection.records = slices.Clone(projection.records)
	projection.outputs = cloneRenderedOutputs(projection.outputs)
	return projection
}
