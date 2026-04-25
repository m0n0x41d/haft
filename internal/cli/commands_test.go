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

	displayPath, err := installSkill("air", false, projectRoot)
	if err != nil {
		t.Fatalf("installSkill returned error: %v", err)
	}

	wantDir := filepath.Join(projectRoot, "skills", "h-reason")
	if displayPath != wantDir {
		t.Fatalf("display path = %q, want %q", displayPath, wantDir)
	}

	skillPath := filepath.Join(wantDir, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read installed skill: %v", err)
	}

	if string(content) != string(embeddedHReasonSkill) {
		t.Fatalf("installed skill content mismatch")
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

	commandCount := embeddedCommandCount(t)
	if count != commandCount+1 {
		t.Fatalf("installed skill count = %d, want %d", count, commandCount+1)
	}
	if displayPath != "~/.agents/skills" {
		t.Fatalf("display path = %q, want %q", displayPath, "~/.agents/skills")
	}

	skillsRoot := filepath.Join(homeDir, ".agents", "skills")
	frameSkillPath := filepath.Join(skillsRoot, "h-frame", "SKILL.md")
	frameSkill, err := os.ReadFile(frameSkillPath)
	if err != nil {
		t.Fatalf("failed to read h-frame skill: %v", err)
	}

	frameContent := string(frameSkill)
	for _, want := range []string{
		"name: h-frame",
		"This skill is explicit-only",
		"Use the user's explicit skill invocation text as the request context.",
		"$h-decide",
	} {
		if !strings.Contains(frameContent, want) {
			t.Fatalf("h-frame skill missing %q:\n%s", want, frameContent)
		}
	}
	for _, banned := range []string{"/h-", "/q-", "$ARGUMENTS", "Quint"} {
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

	explicitPolicyPath := filepath.Join(skillsRoot, "h-frame", "agents", "openai.yaml")
	explicitPolicy, err := os.ReadFile(explicitPolicyPath)
	if err != nil {
		t.Fatalf("failed to read h-frame policy: %v", err)
	}
	if !strings.Contains(string(explicitPolicy), "allow_implicit_invocation: false") {
		t.Fatalf("h-frame should be explicit-only, got:\n%s", string(explicitPolicy))
	}

	reasonPolicyPath := filepath.Join(skillsRoot, "h-reason", "agents", "openai.yaml")
	reasonPolicy, err := os.ReadFile(reasonPolicyPath)
	if err != nil {
		t.Fatalf("failed to read h-reason policy: %v", err)
	}
	if !strings.Contains(string(reasonPolicy), "allow_implicit_invocation: true") {
		t.Fatalf("h-reason should allow implicit invocation, got:\n%s", string(reasonPolicy))
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

func embeddedCommandCount(t *testing.T) int {
	t.Helper()

	entries, err := embeddedCommands.ReadDir("commands")
	if err != nil {
		t.Fatalf("read embedded commands: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		count++
	}
	return count
}

func TestEmbeddedHReasonSkill_Path5RequiresAutonomousMode(t *testing.T) {
	content := string(embeddedHReasonSkill)

	required := []string{
		`ONLY when autonomous mode is already enabled for the session`,
		`If autonomous mode is OFF, phrases like "figure out the best approach and do it" or "fix everything" are NOT enough`,
	}

	for _, want := range required {
		if !strings.Contains(content, want) {
			t.Fatalf("embedded skill missing %q", want)
		}
	}
}

func TestV7EmbeddedCommandPromptsDescribeSpecFirstSurfaceContracts(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		required []string
	}{
		{
			name: "h-onboard",
			path: "commands/h-onboard.md",
			required: []string{
				"TargetSystemSpec",
				"EnablingSystemSpec",
				"TermMap",
				"SpecCoverage",
				"haft spec check",
				"needs_onboard",
				"Claude Code and Codex",
			},
		},
		{
			name: "h-status",
			path: "commands/h-status.md",
			required: []string{
				"needs_onboard",
				"haft spec check",
				"WorkCommissions",
				"stale, blocked, or running-too-long WorkCommissions",
				`haft_commission(action="show"`,
				"do not start Open-Sleigh",
			},
		},
		{
			name: "h-commission",
			path: "commands/h-commission.md",
			required: []string{
				"authorization step only",
				"must not start Open-Sleigh",
				"does not own runtime lifecycle",
				"WorkCommission = bounded permission to execute",
				"Do not requeue a commission whose `valid_until` has expired",
				"Do not physically delete WorkCommissions",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			contentBytes, err := embeddedCommands.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read command %s: %v", tc.path, err)
			}

			content := string(contentBytes)
			for _, required := range tc.required {
				if !strings.Contains(content, required) {
					t.Fatalf("%s missing %q:\n%s", tc.path, required, content)
				}
			}
		})
	}
}
