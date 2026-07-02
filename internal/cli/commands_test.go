package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillAirUsesProjectSkillsDir(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	// Multi-skill installer returns the skills ROOT (parent of each skill
	// folder), not a per-skill subdir. Each skill in allSkills lands as
	// `<root>/<skill-name>/SKILL.md`.
	displayPath, count, err := installSkill("air", false, projectRoot)
	if err != nil {
		t.Fatalf("installSkill returned error: %v", err)
	}
	if count != len(allSkills) {
		t.Errorf("installSkill installed %d skills, expected %d", count, len(allSkills))
	}

	wantRoot := filepath.Join(projectRoot, "skills")
	if displayPath != wantRoot {
		t.Fatalf("display path = %q, want %q", displayPath, wantRoot)
	}

	// Each governance-substrate skill should land at <root>/<name>/SKILL.md.
	for _, sk := range allSkills {
		skillPath := filepath.Join(wantRoot, sk.Name, "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read installed skill %q: %v", sk.Name, err)
		}
		if string(content) != string(sk.Content) {
			t.Fatalf("installed skill %q content mismatch", sk.Name)
		}
	}

	// Deprecated h-fpf directory MUST NOT exist after install — h-fpf
	// was the v8 narrow-fallback name; it's been superseded by the
	// h-reason umbrella. Re-running haft init always lands the clean
	// post-rename state.
	if _, err := os.Stat(filepath.Join(wantRoot, "h-fpf")); !os.IsNotExist(err) {
		t.Fatalf("h-fpf should be removed by deprecation cleanup; got err=%v", err)
	}
}

func TestInstallCodexSkillsWritesExplicitCommandSkills(t *testing.T) {
	projectRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	displayPath, count, err := installCodexSkills(projectRoot, false)
	if err != nil {
		t.Fatalf("installCodexSkills returned error: %v", err)
	}

	// Codex installer writes exactly allSkills — no embedded commands
	// path. Skills are the primary surface; slash commands are not
	// shipped with haft.
	if count != len(allSkills) {
		t.Fatalf("installed skill count = %d, want %d (len(allSkills))",
			count, len(allSkills))
	}
	if displayPath != "~/.agents/skills" {
		t.Fatalf("display path = %q, want %q", displayPath, "~/.agents/skills")
	}

	skillsRoot := filepath.Join(homeDir, ".agents", "skills")

	// h-frame is an auto-triggering workflow skill — its SKILL.md is the
	// raw skill body (frontmatter has the description for routing), NOT
	// the command-wrapper variant that prefixes "This skill is explicit-
	// only". Slash-command-style $h-X references must still be rewritten
	// from /h-X.
	frameSkillPath := filepath.Join(skillsRoot, "h-frame", "SKILL.md")
	frameSkill, err := os.ReadFile(frameSkillPath)
	if err != nil {
		t.Fatalf("failed to read h-frame skill: %v", err)
	}
	frameContent := string(frameSkill)
	for _, want := range []string{
		"name: h-frame",
		"$h-explore",
	} {
		if !strings.Contains(frameContent, want) {
			t.Fatalf("h-frame skill missing %q:\n%s", want, frameContent)
		}
	}
	for _, banned := range []string{"/h-", "/q-", "Quint"} {
		if strings.Contains(frameContent, banned) {
			t.Fatalf("h-frame skill contains stale token %q:\n%s", banned, frameContent)
		}
	}

	skillFiles, err := filepath.Glob(filepath.Join(skillsRoot, "h-*", "SKILL.md"))
	if err != nil {
		t.Fatalf("glob installed skills: %v", err)
	}
	for _, skillFile := range skillFiles {
		content, err := os.ReadFile(skillFile)
		if err != nil {
			t.Fatalf("read installed skill %s: %v", skillFile, err)
		}
		for _, banned := range []string{"/h-", "/q-", "$ARGUMENTS", "Quint"} {
			if strings.Contains(string(content), banned) {
				t.Fatalf("%s contains stale token %q", skillFile, banned)
			}
		}
	}

	// h-frame is an auto-triggering workflow skill — policy must reflect
	// that. Manual-only skills (h-decide, h-commission) get asserted
	// below.
	framePolicyPath := filepath.Join(skillsRoot, "h-frame", "agents", "openai.yaml")
	framePolicy, err := os.ReadFile(framePolicyPath)
	if err != nil {
		t.Fatalf("failed to read h-frame policy: %v", err)
	}
	if !strings.Contains(string(framePolicy), "allow_implicit_invocation: true") {
		t.Fatalf("h-frame should allow implicit invocation, got:\n%s", string(framePolicy))
	}

	// h-reason is the umbrella entry — broad auto-trigger description
	// plus manual /h-reason invocation. Carries the full FPF reasoning
	// palette (frame, explore, compare, verify, note, slideument
	// patterns). Verify policy allows implicit invocation.
	reasonPolicyPath := filepath.Join(skillsRoot, "h-reason", "agents", "openai.yaml")
	reasonPolicy, err := os.ReadFile(reasonPolicyPath)
	if err != nil {
		t.Fatalf("failed to read h-reason policy: %v", err)
	}
	if !strings.Contains(string(reasonPolicy), "allow_implicit_invocation: true") {
		t.Fatalf("h-reason should allow implicit invocation, got:\n%s", string(reasonPolicy))
	}

	specSkillPath := filepath.Join(skillsRoot, "h-spec", "SKILL.md")
	specSkill, err := os.ReadFile(specSkillPath)
	if err != nil {
		t.Fatalf("failed to read h-spec skill: %v", err)
	}
	specContent := string(specSkill)
	for _, want := range []string{
		"name: h-spec",
		`haft_spec_section(action="lifecycle")`,
		"SpecSectionBaseline",
	} {
		if !strings.Contains(specContent, want) {
			t.Fatalf("h-spec skill missing %q:\n%s", want, specContent)
		}
	}
	specPolicyPath := filepath.Join(skillsRoot, "h-spec", "agents", "openai.yaml")
	specPolicy, err := os.ReadFile(specPolicyPath)
	if err != nil {
		t.Fatalf("failed to read h-spec policy: %v", err)
	}
	if !strings.Contains(string(specPolicy), "allow_implicit_invocation: true") {
		t.Fatalf("h-spec should allow implicit invocation, got:\n%s", string(specPolicy))
	}

	// h-decide is manual-only (Transformer Mandate via codex policy +
	// disable-model-invocation in claude frontmatter).
	decidePolicyPath := filepath.Join(skillsRoot, "h-decide", "agents", "openai.yaml")
	decidePolicy, err := os.ReadFile(decidePolicyPath)
	if err != nil {
		t.Fatalf("failed to read h-decide policy: %v", err)
	}
	if !strings.Contains(string(decidePolicy), "allow_implicit_invocation: false") {
		t.Fatalf("h-decide must be explicit-only per Transformer Mandate, got:\n%s", string(decidePolicy))
	}
	decideSkillPath := filepath.Join(skillsRoot, "h-decide", "SKILL.md")
	decideSkill, err := os.ReadFile(decideSkillPath)
	if err != nil {
		t.Fatalf("failed to read h-decide skill: %v", err)
	}
	for _, want := range []string{"haft interface decision.decide --json", "haft artifact create decision.decide"} {
		if !strings.Contains(string(decideSkill), want) {
			t.Fatalf("h-decide skill should point at compact CLI path %q:\n%s", want, string(decideSkill))
		}
	}

	noteSkillPath := filepath.Join(skillsRoot, "h-note", "SKILL.md")
	noteSkill, err := os.ReadFile(noteSkillPath)
	if err != nil {
		t.Fatalf("failed to read h-note skill: %v", err)
	}
	noteContent := string(noteSkill)
	for _, want := range []string{"mcp__haft__haft_note", "haft interface note.record --json", "haft artifact create note.record"} {
		if !strings.Contains(noteContent, want) {
			t.Fatalf("h-note skill missing %q:\n%s", want, noteContent)
		}
	}
	if strings.Contains(noteContent, `haft_problem(
  action="frame"`) {
		t.Fatalf("h-note skill must not emulate notes with ProblemCards:\n%s", noteContent)
	}

	// Deprecated h-fpf directory must be removed (migration step —
	// h-fpf was the v8 narrow-fallback name; it's been replaced by the
	// h-reason umbrella).
	if _, err := os.Stat(filepath.Join(skillsRoot, "h-fpf")); !os.IsNotExist(err) {
		t.Fatalf("h-fpf must be removed by deprecation cleanup; got err=%v", err)
	}
}

func TestInstallCodexSkillsLocalUsesProjectAgentsDir(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	displayPath, _, err := installCodexSkills(projectRoot, true)
	if err != nil {
		t.Fatalf("installCodexSkills returned error: %v", err)
	}

	wantPath := filepath.Join(projectRoot, ".agents", "skills")
	if displayPath != wantPath {
		t.Fatalf("display path = %q, want %q", displayPath, wantPath)
	}
}

func TestCleanupCodexPromptCommandsRemovesOnlyHaftPrompts(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	promptDir := filepath.Join(homeDir, ".codex", "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"h-frame.md":  "old h-frame prompt",
		"q-frame.md":  "old q-frame prompt",
		"q-reason.md": "old q-reason prompt",
		"custom.md":   "user prompt",
	}
	for name, content := range files {
		path := filepath.Join(promptDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	displayPath, removed, err := cleanupCodexPromptCommands()
	if err != nil {
		t.Fatalf("cleanupCodexPromptCommands returned error: %v", err)
	}
	if displayPath != "~/.codex/prompts" {
		t.Fatalf("display path = %q, want %q", displayPath, "~/.codex/prompts")
	}
	if removed != 3 {
		t.Fatalf("removed = %d, want 3", removed)
	}

	for _, removedName := range []string{"h-frame.md", "q-frame.md", "q-reason.md"} {
		if _, err := os.Stat(filepath.Join(promptDir, removedName)); !os.IsNotExist(err) {
			t.Fatalf("%s should have been removed", removedName)
		}
	}
	if _, err := os.Stat(filepath.Join(promptDir, "custom.md")); err != nil {
		t.Fatalf("custom prompt should remain: %v", err)
	}
}

// TestHDecideSkill_IsManualOnlyTransformerMandate verifies that the
// h-decide skill carries the structural Transformer Mandate enforcement
// (disable-model-invocation) so the agent cannot auto-fire a binding
// DecisionRecord write. Per v8 governance substrate pivot.
func TestHDecideSkill_IsManualOnlyTransformerMandate(t *testing.T) {
	content := string(embeddedHDecideSkill)

	required := []string{
		`disable-model-invocation: true`,
		`MANUAL ONLY`,
		`Transformer Mandate`,
	}

	for _, want := range required {
		if !strings.Contains(content, want) {
			t.Fatalf("h-decide skill missing %q — Transformer Mandate enforcement broken", want)
		}
	}
}

// TestHReasonSkill_IsFullUmbrella verifies that the h-reason umbrella
// carries the full FPF reasoning palette (frame, explore, compare,
// verify, note, slideument patterns) rather than acting as a narrow
// fallback. It must reference specialized skills as "heavy versions"
// to delegate to, and point at the spec-search MCP path for deep
// references.
func TestHReasonSkill_IsFullUmbrella(t *testing.T) {
	content := string(embeddedHReasonSkill)

	// Must reference specialized skills it can delegate heavy versions to.
	for _, sk := range []string{"h-frame", "h-diagnose", "h-explore", "h-compare", "h-decide", "h-verify", "h-spec"} {
		if !strings.Contains(content, sk) {
			t.Fatalf("h-reason must reference %q as a heavy-version delegate", sk)
		}
	}
	// Must point at the spec-search MCP path so the agent can retrieve
	// pattern text without h-reason having to inline the full spec.
	if !strings.Contains(content, `haft_query(action="fpf"`) {
		t.Fatal("h-reason must point at haft_query(action=\"fpf\", ...) for spec lookups")
	}
	if !strings.Contains(content, `action="pattern_use"`) {
		t.Fatal("h-reason must consult haft_query(action=\"pattern_use\", ...) before generic triage")
	}
	for _, want := range []string{
		"PatternUse Gateway",
		`mode="compact"`,
		`mode="full"`,
		"should_use_pattern",
		"suggested_haft_surface",
		"Do not inline the FPF catalog",
		"not a DecisionRecord",
		"not MethodPack",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("h-reason PatternUse routing instructions missing %q", want)
		}
	}
	// Must carry the "Description != Work" rule — the core anti-pattern
	// that this umbrella is designed to NOT fall into.
	if !strings.Contains(content, "Description ≠ Work") {
		t.Fatal("h-reason must carry the Description ≠ Work core rule")
	}
	if !strings.Contains(content, "unpersisted reasoning, not durable project governance, evidence, or authority") {
		t.Fatal("h-reason must distinguish useful chat reasoning from durable governance authority")
	}
	// Must cover the slideument patterns that don't have dedicated skills.
	for _, pat := range []string{"Goldilocks", "NQD", "stepping", "Anti-Goodhart"} {
		if !strings.Contains(content, pat) {
			t.Fatalf("h-reason must cover slideument pattern %q", pat)
		}
	}
}

func TestSubstantiveSkillsCarryCatalogFreePatternUseGateway(t *testing.T) {
	skills := map[string][]byte{
		"h-frame":    embeddedHFrameSkill,
		"h-diagnose": embeddedHDiagnoseSkill,
		"h-explore":  embeddedHExploreSkill,
		"h-compare":  embeddedHCompareSkill,
		"h-verify":   embeddedHVerifySkill,
	}

	for name, contentBytes := range skills {
		t.Run(name, func(t *testing.T) {
			content := string(contentBytes)
			for _, want := range []string{
				"PatternUse Gateway",
				`action="pattern_use"`,
				`mode="compact"`,
				`mode="full"`,
				"Do not inline the FPF catalog",
				"not MethodPack",
			} {
				if !strings.Contains(content, want) {
					t.Fatalf("%s missing PatternUse gateway fragment %q", name, want)
				}
			}
			for _, banned := range []string{
				"Naming/terminology requests should route",
				"Architecture requests should route",
				"SoTA/current-practice requests should route",
			} {
				if strings.Contains(content, banned) {
					t.Fatalf("%s inlines route category prose %q", name, banned)
				}
			}
		})
	}
}
