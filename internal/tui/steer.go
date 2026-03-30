package tui

import (
	"embed"
	"strings"
)

// Embedded command prompts — rich FPF steering instructions.
// These are the same files installed as CC slash commands, but used
// inline when the user types /frame, /refresh, etc. in the haft TUI.
//
//go:embed commands/*.md
var embeddedCommandPrompts embed.FS

// commandPromptMap caches command name → prompt content.
var commandPromptMap map[string]string

func init() {
	commandPromptMap = make(map[string]string)
	entries, err := embeddedCommandPrompts.ReadDir("commands")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		name = strings.TrimPrefix(name, "h-") // h-frame → frame
		content, err := embeddedCommandPrompts.ReadFile("commands/" + entry.Name())
		if err != nil {
			continue
		}
		// Strip YAML frontmatter
		body := stripFrontmatter(string(content))
		commandPromptMap[name] = body
	}
}

// GetCommandPrompt returns the FPF steering prompt for a slash command.
// Returns "" if no prompt exists (command is handled internally).
func GetCommandPrompt(command string) string {
	return commandPromptMap[command]
}

// stripFrontmatter removes YAML frontmatter (--- ... ---) from markdown.
func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return s
	}
	end := strings.Index(s[3:], "---")
	if end < 0 {
		return s
	}
	return strings.TrimSpace(s[end+6:])
}
