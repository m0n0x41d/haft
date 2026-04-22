package cli

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed commands/*.md
var embeddedCommands embed.FS

//go:embed skill/h-reason/SKILL.md
var embeddedHReasonSkill []byte

func installCommands(projectRoot string, platform string, local bool) (string, int, error) {
	entries, err := embeddedCommands.ReadDir("commands")
	if err != nil {
		return "", 0, fmt.Errorf("failed to read embedded commands: %w", err)
	}

	homeDir, _ := os.UserHomeDir()

	var destDir string
	var transformer func(string, string) (string, string)
	var ext string

	switch platform {
	case "claude":
		if local {
			destDir = filepath.Join(projectRoot, ".claude", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".claude", "commands")
		}
		transformer = transformClaude
		ext = ".md"
	case "cursor":
		if local {
			destDir = filepath.Join(projectRoot, ".cursor", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".cursor", "commands")
		}
		transformer = transformCursor
		ext = ".md"
	case "gemini":
		if local {
			destDir = filepath.Join(projectRoot, ".gemini", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".gemini", "commands")
		}
		transformer = transformGemini
		ext = ".toml"
	case "codex":
		// Codex only supports global prompts in ~/.codex/prompts/
		destDir = filepath.Join(homeDir, ".codex", "prompts")
		transformer = transformCodex
		ext = ".md"
	default:
		return "", 0, fmt.Errorf("unknown platform: %s", platform)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", 0, err
	}

	cleanupOldCommands(destDir, ext)

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := embeddedCommands.ReadFile("commands/" + entry.Name())
		if err != nil {
			continue
		}

		newName, newContent := transformer(entry.Name(), string(content))
		destPath := filepath.Join(destDir, newName)

		if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
			continue
		}
		count++
	}

	// Make path relative for display
	displayPath := destDir
	if strings.HasPrefix(destDir, homeDir) {
		displayPath = "~" + strings.TrimPrefix(destDir, homeDir)
	}

	return displayPath, count, nil
}

func installCodexSkills(projectRoot string, local bool) (string, int, error) {
	homeDir, _ := os.UserHomeDir()
	skillsRoot := codexSkillsRoot(homeDir, projectRoot, local)

	if err := os.MkdirAll(skillsRoot, 0755); err != nil {
		return "", 0, err
	}

	cleanupOldCodexSkills(skillsRoot)

	reasonSkill := transformCodexSkillReferences(string(embeddedHReasonSkill))
	if err := writeCodexSkill(skillsRoot, "h-reason", reasonSkill, true); err != nil {
		return "", 0, err
	}

	entries, err := embeddedCommands.ReadDir("commands")
	if err != nil {
		return "", 0, fmt.Errorf("failed to read embedded commands: %w", err)
	}

	count := 1
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := embeddedCommands.ReadFile("commands/" + entry.Name())
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		skill := transformCodexCommandSkill(name, string(content))
		if err := writeCodexSkill(skillsRoot, name, skill, false); err != nil {
			continue
		}
		count++
	}

	return displayHomePath(skillsRoot, homeDir), count, nil
}

func codexSkillsRoot(homeDir, projectRoot string, local bool) string {
	if local {
		return filepath.Join(projectRoot, ".agents", "skills")
	}
	return filepath.Join(homeDir, ".agents", "skills")
}

func cleanupOldCodexSkills(skillsRoot string) {
	for _, cmd := range deprecatedCommands {
		_ = os.RemoveAll(filepath.Join(skillsRoot, cmd))
	}
}

func writeCodexSkill(skillsRoot, name, content string, allowImplicit bool) error {
	skillDir := filepath.Join(skillsRoot, name)
	if err := os.MkdirAll(filepath.Join(skillDir, "agents"), 0755); err != nil {
		return err
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return err
	}

	return writeCodexSkillPolicy(skillDir, allowImplicit)
}

func transformCodexCommandSkill(name, content string) string {
	description := extractFrontmatterDescription(content)
	if description == "" {
		description = "Haft command: " + name
	}

	body := stripMarkdownFrontmatter(content)
	body = transformCodexSkillReferences(body)
	body = strings.ReplaceAll(body, "$ARGUMENTS", "Use the user's explicit skill invocation text as the request context.")
	body = strings.TrimSpace(body)

	return fmt.Sprintf(`---
name: %s
description: %s
---

## Codex Invocation

This skill is explicit-only. Use it only when the user invokes $%s; treat the text after the skill name as the request context.

%s
`, name, yamlDoubleQuote(description), name, body)
}

func transformCodexSkillReferences(content string) string {
	replacer := strings.NewReplacer(
		"/h-", "$h-",
		"Slash commands", "Explicit skill invocations",
		"slash commands", "explicit skill invocations",
		"Slash command", "Explicit skill",
		"slash command", "explicit skill",
		"Quint", "Haft",
		"quint", "haft",
	)
	return replacer.Replace(content)
}

func stripMarkdownFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}

	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return content
	}

	start := 4 + end + len("\n---")
	return strings.TrimLeft(content[start:], "\r\n")
}

func extractFrontmatterDescription(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return ""
	}

	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return ""
	}

	frontmatter := content[4 : 4+end]
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "description:") {
			continue
		}

		value := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		return strings.Trim(value, `"`)
	}

	return ""
}

func yamlDoubleQuote(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	)
	return `"` + replacer.Replace(value) + `"`
}

var deprecatedCommands = []string{
	// v4 commands
	"q0-init", "q-decay", "q-actualize", "q1-add", "q-implement",
	"q-internalize", "q-query", "q-reset", "q-resolve",
	"q1-hypothesize", "q2-verify", "q3-validate", "q4-audit", "q5-decide",
	// v5 q-prefix (renamed to h-prefix)
	"q-apply", "q-char", "q-compare", "q-decide", "q-explore",
	"q-frame", "q-note", "q-onboard", "q-problems", "q-refresh",
	"q-reason", "q-search", "q-status",
	// v6 h-refresh replaced by h-verify
	"h-refresh",
}

func cleanupOldCommands(destDir string, ext string) {
	for _, cmd := range deprecatedCommands {
		path := filepath.Join(destDir, cmd+ext)
		_ = os.Remove(path) // ignore error - file may not exist
	}
}

func cleanupCodexPromptCommands() (string, int, error) {
	homeDir, _ := os.UserHomeDir()
	destDir := filepath.Join(homeDir, ".codex", "prompts")

	entries, err := embeddedCommands.ReadDir("commands")
	if err != nil {
		return "", 0, fmt.Errorf("failed to read embedded commands: %w", err)
	}

	names := append([]string{}, deprecatedCommands...)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), ".md"))
	}

	removed := 0
	for _, name := range names {
		path := filepath.Join(destDir, name+".md")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			return "", removed, err
		}
		removed++
	}

	return displayHomePath(destDir, homeDir), removed, nil
}

func transformClaude(filename, content string) (string, string) {
	return filename, content
}

func transformCursor(filename, content string) (string, string) {
	return filename, content
}

func transformCodex(filename, content string) (string, string) {
	// Codex uses same format as Claude (markdown with frontmatter)
	// but calls them "prompts" and invokes via /prompts:<name>
	return filename, content
}

func transformGemini(filename, content string) (string, string) {
	name := strings.TrimSuffix(filename, ".md")
	newFilename := name + ".toml"

	description := extractFirstHeading(content)
	if description == "" {
		description = "FPF command: " + name
	}
	description = strings.ReplaceAll(description, `"`, `\"`)

	transformed := content
	transformed = strings.ReplaceAll(transformed, "$ARGUMENTS", "{{args}}")
	transformed = strings.ReplaceAll(transformed, "$1", "{{1}}")
	transformed = strings.ReplaceAll(transformed, "$2", "{{2}}")
	transformed = strings.ReplaceAll(transformed, "$3", "{{3}}")
	transformed = escapeTomlMultiline(transformed)

	tomlContent := fmt.Sprintf(`description = "%s"

prompt = """
%s
"""
`, description, transformed)

	return newFilename, tomlContent
}

func extractFirstHeading(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "# "))
		}
	}
	return ""
}

func escapeTomlMultiline(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"""`, `\"""`)
	return s
}

func installSkill(platform string, local bool, projectRoot string) (string, error) {
	homeDir, _ := os.UserHomeDir()

	var skillDir string
	switch platform {
	case "claude":
		if local {
			skillDir = filepath.Join(projectRoot, ".claude", "skills", "h-reason")
		} else {
			skillDir = filepath.Join(homeDir, ".claude", "skills", "h-reason")
		}
	case "cursor":
		if local {
			skillDir = filepath.Join(projectRoot, ".cursor", "skills", "h-reason")
		} else {
			skillDir = filepath.Join(homeDir, ".cursor", "skills", "h-reason")
		}
	case "air":
		skillDir = filepath.Join(projectRoot, "skills", "h-reason")
	case "codex":
		if local {
			skillDir = filepath.Join(projectRoot, ".agents", "skills", "h-reason")
		} else {
			skillDir = filepath.Join(homeDir, ".agents", "skills", "h-reason")
		}
	default:
		return "", nil
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create skill directory: %w", err)
	}

	// Remove old q-reason skill if it exists (migration)
	oldSkillDir := strings.Replace(skillDir, "h-reason", "q-reason", 1)
	if oldSkillDir != skillDir {
		_ = os.RemoveAll(oldSkillDir)
	}

	destPath := filepath.Join(skillDir, "SKILL.md")
	content := embeddedHReasonSkill
	if platform == "codex" {
		content = []byte(transformCodexSkillReferences(string(embeddedHReasonSkill)))
	}
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write skill: %w", err)
	}

	if platform == "codex" {
		if err := writeCodexSkillPolicy(skillDir, true); err != nil {
			return "", fmt.Errorf("failed to write skill policy: %w", err)
		}
	}

	return displayHomePath(skillDir, homeDir), nil
}

func writeCodexSkillPolicy(skillDir string, allowImplicit bool) error {
	agentsDir := filepath.Join(skillDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return err
	}

	policy := fmt.Sprintf("policy:\n  allow_implicit_invocation: %t\n", allowImplicit)
	return os.WriteFile(filepath.Join(agentsDir, "openai.yaml"), []byte(policy), 0644)
}

func displayHomePath(path, homeDir string) string {
	displayPath := path
	if homeDir == "" {
		return displayPath
	}

	homePrefix := homeDir + string(os.PathSeparator)
	if path == homeDir || strings.HasPrefix(path, homePrefix) {
		displayPath = "~" + strings.TrimPrefix(path, homeDir)
	}
	return displayPath
}
