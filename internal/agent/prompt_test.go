package agent

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_ResearchOnlyAndDelegatedReasoningMatrix(t *testing.T) {
	t.Parallel()

	prompt := BuildSystemPrompt(PromptConfig{
		ProjectRoot: "/repo",
		Cwd:         "/repo",
		Lemniscate:  true,
	})

	required := []string{
		`Research / prepare-and-wait requests`,
		`Delegated reasoning requests`,
		`frame → explore → compare`,
		`Do NOT stop after frame or explore.`,
		`Do NOT require manual "/explore" or "/compare" after frame.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	if strings.Contains(prompt, "After frame and after explore: STOP and present your work. Wait for user.") {
		t.Fatal("prompt still contains blanket stop-after-frame instruction")
	}
}

func TestBuildSystemPrompt_StopsDelegatedReasoningAtCompare(t *testing.T) {
	t.Parallel()

	prompt := BuildSystemPrompt(PromptConfig{
		ProjectRoot: "/repo",
		Cwd:         "/repo",
		Lemniscate:  true,
	})

	required := []string{
		`If the user asked only for preparation, present the framing candidate and STOP.`,
		`In symbiotic delegated mode, ASK which variant and wait.`,
		`Transformer Mandate applies at the compare -> decide boundary.`,
		`Autonomous mode skips the remaining pause after compare and carries through implementation.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
