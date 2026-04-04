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

func TestDecisionBoundarySatisfied_RequiresExplicitSelection(t *testing.T) {
	t.Parallel()

	cycle := &agent.Cycle{
		PortfolioRef:         "port-1",
		ComparedPortfolioRef: "port-1",
	}

	if decisionBoundarySatisfied(agent.InteractionSymbiotic, cycle) {
		t.Fatal("expected compare -> decide boundary to remain blocked without an explicit selection")
	}
}

func TestDecisionBoundarySatisfied_RequiresSelectionOnActiveComparedPortfolio(t *testing.T) {
	t.Parallel()

	cycle := &agent.Cycle{
		PortfolioRef:         "port-1",
		ComparedPortfolioRef: "port-1",
		SelectedPortfolioRef: "port-old",
		SelectedVariantRef:   "V2",
	}

	if decisionBoundarySatisfied(agent.InteractionSymbiotic, cycle) {
		t.Fatal("expected stale selection to remain blocked")
	}
}

func TestDecisionBoundarySatisfied_AllowsExplicitSelection(t *testing.T) {
	t.Parallel()

	cycle := &agent.Cycle{
		PortfolioRef:         "port-1",
		ComparedPortfolioRef: "port-1",
		SelectedPortfolioRef: "port-1",
		SelectedVariantRef:   "V2",
	}

	if !decisionBoundarySatisfied(agent.InteractionSymbiotic, cycle) {
		t.Fatal("expected explicit selection to satisfy the decision boundary")
	}
}

func TestDecisionBoundarySatisfied_AutonomousSkipsPause(t *testing.T) {
	t.Parallel()

	if !decisionBoundarySatisfied(agent.InteractionAutonomous, nil) {
		t.Fatal("expected autonomous mode to skip the compare -> decide pause")
	}
}

func TestDetectExplicitDecisionSelection(t *testing.T) {
	t.Parallel()

	candidates := []decisionSelectionCandidate{
		{VariantRef: "V1", Aliases: normalizeDecisionSelectionAliases([]string{"V1", "REST", "variant V1"})},
		{VariantRef: "V2", Aliases: normalizeDecisionSelectionAliases([]string{"V2", "gRPC", "variant V2"})},
	}

	cases := map[string]string{
		"pick variant V2":      "V2",
		"go with gRPC":         "V2",
		"actually choose REST": "V1",
		"V2":                   "V2",
	}

	for input, want := range cases {
		got, ok := detectExplicitDecisionSelection(input, candidates)
		if !ok {
			t.Fatalf("detectExplicitDecisionSelection(%q) = no match, want %q", input, want)
		}
		if got != want {
			t.Fatalf("detectExplicitDecisionSelection(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDetectExplicitDecisionSelection_IgnoresFollowUps(t *testing.T) {
	t.Parallel()

	candidates := []decisionSelectionCandidate{
		{VariantRef: "V1", Aliases: normalizeDecisionSelectionAliases([]string{"V1", "REST", "variant V1"})},
		{VariantRef: "V2", Aliases: normalizeDecisionSelectionAliases([]string{"V2", "gRPC", "variant V2"})},
	}

	inputs := []string{
		"show the table again",
		"explain option V2",
		"can we choose V2?",
		"variant V2 is bad because of tooling",
		"do not choose gRPC",
	}

	for _, input := range inputs {
		if got, ok := detectExplicitDecisionSelection(input, candidates); ok {
			t.Fatalf("detectExplicitDecisionSelection(%q) = %q, want no match", input, got)
		}
	}
}
