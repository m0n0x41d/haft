package cmd

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

func installCommands(projectRoot string, platform string, local bool) (string, int, error) {
	entries, err := embeddedCommands.ReadDir("commands")
	if err != nil {
		return "", 0, fmt.Errorf("failed to read embedded commands: %w", err)
	}

	homeDir, _ := os.UserHomeDir()

	var destDir string
	var transformer func(string, string) (string, string)

	switch platform {
	case "claude":
		if local {
			destDir = filepath.Join(projectRoot, ".claude", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".claude", "commands")
		}
		transformer = transformClaude
	case "cursor":
		if local {
			destDir = filepath.Join(projectRoot, ".cursor", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".cursor", "commands")
		}
		transformer = transformCursor
	case "gemini":
		if local {
			destDir = filepath.Join(projectRoot, ".gemini", "commands")
		} else {
			destDir = filepath.Join(homeDir, ".gemini", "commands")
		}
		transformer = transformGemini
	case "codex":
		// Codex only supports global prompts in ~/.codex/prompts/
		destDir = filepath.Join(homeDir, ".codex", "prompts")
		transformer = transformCodex
	default:
		return "", 0, fmt.Errorf("unknown platform: %s", platform)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", 0, err
	}

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
