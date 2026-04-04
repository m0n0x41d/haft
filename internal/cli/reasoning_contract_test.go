package cli

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
)

func TestReasoningSourcesShareCanonicalInteractionMatrix(t *testing.T) {
	t.Parallel()

	prompt := agent.BuildSystemPrompt(agent.PromptConfig{
		ProjectRoot: "/repo",
		Cwd:         "/repo",
		Lemniscate:  true,
	})
	skill := string(embeddedHReasonSkill)
	commandBytes, err := embeddedCommands.ReadFile("commands/h-compare.md")
	if err != nil {
		t.Fatalf("read embedded compare command: %v", err)
	}
	command := string(commandBytes)

	sources := map[string]string{
		"prompt":    prompt,
		"skill":     skill,
		"h-compare": command,
	}

	required := []string{
		`Direct response / direct action`,
		`Research / prepare-and-wait`,
		`Delegated reasoning`,
		`Autonomous execution`,
	}

	for name, content := range sources {
		for _, want := range required {
			if !strings.Contains(content, want) {
				t.Fatalf("%s missing %q", name, want)
			}
		}
	}
}

func TestReasoningSourcesRejectKnownContradictoryPhrases(t *testing.T) {
	t.Parallel()

	prompt := agent.BuildSystemPrompt(agent.PromptConfig{
		ProjectRoot: "/repo",
		Cwd:         "/repo",
		Lemniscate:  true,
	})
	skill := string(embeddedHReasonSkill)
	commandBytes, err := embeddedCommands.ReadFile("commands/h-compare.md")
	if err != nil {
		t.Fatalf("read embedded compare command: %v", err)
	}
	command := string(commandBytes)

	sources := map[string]string{
		"prompt":    prompt,
		"skill":     skill,
		"h-compare": command,
	}

	forbidden := []string{
		`Path 3`,
		`Path 4`,
		`Path 5`,
		`"давай" / "do it" / "go ahead" = START WORKING`,
		`After frame and after explore: STOP and present your work. Wait for user.`,
		"`/h-frame` → `/h-decide`",
		"tactical skips exploration",
		"Tactical mode may skip some artifacts",
	}

	for name, content := range sources {
		for _, bad := range forbidden {
			if strings.Contains(content, bad) {
				t.Fatalf("%s still contains %q", name, bad)
			}
		}
	}
}
