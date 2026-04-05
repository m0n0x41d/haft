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
		`## Checkpointed workflow — you propose, human decides`,
		`### Canonical interaction matrix`,
		`Direct response / direct action requests`,
		`Research / prepare-and-wait requests`,
		`Delegated reasoning requests`,
		`Autonomous execution requests`,
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

	if strings.Contains(prompt, `"давай" / "do it" / "go ahead" = START WORKING`) {
		t.Fatal("prompt still treats ordinary go-ahead language as unconditional autonomous execution")
	}

	if strings.Contains(prompt, "## Collaborative workflow — you propose, human decides") {
		t.Fatal("prompt still exposes collaborative as the canonical workflow label")
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
		`In checkpointed delegated mode, ASK which variant and wait only AFTER that explanation.`,
		`Transformer Mandate applies at the compare -> decide boundary.`,
		`Autonomous mode skips the remaining pause after compare and carries through implementation.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestBuildSystemPrompt_RequiresParetoFrontDiscussionBeforeChoice(t *testing.T) {
	t.Parallel()

	prompt := BuildSystemPrompt(PromptConfig{
		ProjectRoot: "/repo",
		Cwd:         "/repo",
		Lemniscate:  true,
	})

	required := []string{
		`### Compare presentation contract`,
		`Do not jump from the score grid to "pick X".`,
		`Dominated-variant elimination`,
		`Pareto front members`,
		`Trade-off explanation`,
		`Recommendation is advisory. The human choice is separate.`,
		`ask the user to choose only AFTER the Pareto front and trade-offs are explained.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
