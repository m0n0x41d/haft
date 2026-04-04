package agentloop

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
)

func TestInteractionModePrompt_AutonomousChainsFullCycle(t *testing.T) {
	t.Parallel()

	prompt := interactionModePrompt(agent.InteractionAutonomous)

	required := []string{
		`## [MODE: AUTONOMOUS — ACTIVE NOW]`,
		`frame → explore → compare → decide → implement → measure`,
		`SKIP all "STOP and present" checkpoints.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestInteractionModePrompt_SymbioticIsEmpty(t *testing.T) {
	t.Parallel()

	if prompt := interactionModePrompt(agent.InteractionSymbiotic); prompt != "" {
		t.Fatalf("prompt = %q, want empty", prompt)
	}
}
