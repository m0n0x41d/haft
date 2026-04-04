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

func TestDecisionBoundarySatisfied_RequiresPostCompareUserSelection(t *testing.T) {
	t.Parallel()

	messages := []agent.Message{
		{
			Role: agent.RoleTool,
			Parts: []agent.Part{
				agent.ToolResultPart{ToolName: "haft_solution"},
			},
		},
		{
			Role: agent.RoleUser,
			Parts: []agent.Part{
				agent.TextPart{Text: "keep going"},
			},
		},
		{
			Role: agent.RoleTool,
			Parts: []agent.Part{
				agent.ToolResultPart{ToolName: "haft_solution"},
			},
		},
	}

	if decisionBoundarySatisfied(agent.InteractionSymbiotic, messages) {
		t.Fatal("expected compare -> decide boundary to remain blocked without a post-compare user selection")
	}
}

func TestDecisionBoundarySatisfied_AllowsPostCompareUserSelection(t *testing.T) {
	t.Parallel()

	messages := []agent.Message{
		{
			Role: agent.RoleTool,
			Parts: []agent.Part{
				agent.ToolResultPart{ToolName: "haft_solution"},
			},
		},
		{
			Role: agent.RoleUser,
			Parts: []agent.Part{
				agent.TextPart{Text: "pick variant B"},
			},
		},
	}

	if !decisionBoundarySatisfied(agent.InteractionSymbiotic, messages) {
		t.Fatal("expected post-compare user selection to satisfy decision boundary")
	}
}

func TestDecisionBoundarySatisfied_AutonomousSkipsPause(t *testing.T) {
	t.Parallel()

	messages := []agent.Message{
		{
			Role: agent.RoleTool,
			Parts: []agent.Part{
				agent.ToolResultPart{ToolName: "haft_solution"},
			},
		},
	}

	if !decisionBoundarySatisfied(agent.InteractionAutonomous, messages) {
		t.Fatal("expected autonomous mode to skip the compare -> decide pause")
	}
}
