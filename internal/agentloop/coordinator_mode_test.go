package agentloop

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestInteractionModePrompt_AutonomousChainsFullCycle(t *testing.T) {
	t.Parallel()

	prompt := interactionModePrompt(agent.InteractionAutonomous)

	required := []string{
		`## [MODE: AUTONOMOUS — ACTIVE NOW]`,
		`Only after the request is classified as autonomous execution should you chain frame → explore → compare → decide → implement → measure without pauses.`,
		`Once the request is already classified as autonomous execution, SKIP the remaining "STOP and present" checkpoints.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestInteractionModePrompt_AutonomousDoesNotOverrideRequestClassification(t *testing.T) {
	t.Parallel()

	prompt := interactionModePrompt(agent.InteractionAutonomous)

	required := []string{
		`It does NOT by itself reclassify direct-response, research-only, delegated-reasoning, or compare-only requests into implementation work.`,
		`If the request is direct response / direct action, answer directly.`,
		`If the request is research / prepare-and-wait, investigate and STOP.`,
		`If the request is delegated reasoning without implementation delegation, continue through compare and then wait for the human choice.`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	forbidden := []string{
		`This OVERRIDES the collaborative workflow rules above.`,
		`When the user says "do it" or "давай" — that means START WORKING NOW, not "explain your plan."`,
	}

	for _, banned := range forbidden {
		if strings.Contains(prompt, banned) {
			t.Fatalf("prompt still contains contradictory wording %q", banned)
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
		"pick variant V2":                  "V2",
		"go with gRPC":                     "V2",
		"go with gRPC then":                "V2",
		"gRPC now":                         "V2",
		"use gRPC":                         "V2",
		"pick V2 please":                   "V2",
		"V2 please":                        "V2",
		"let's pick V2":                    "V2",
		"actually choose REST":             "V1",
		"okay V2 then":                     "V2",
		"choose REST now":                  "V1",
		"choose gRPC because latency wins": "V2",
		"V2":                               "V2",
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
		"gRPC because of tooling overhead is risky",
		"REST because latency is worse should be ruled out",
		"do not choose gRPC",
		"use gRPC benchmarks from the previous run",
		"proceed with gRPC benchmarks from the previous run",
		"ship gRPC benchmarks from the previous run",
	}

	for _, input := range inputs {
		if got, ok := detectExplicitDecisionSelection(input, candidates); ok {
			t.Fatalf("detectExplicitDecisionSelection(%q) = %q, want no match", input, got)
		}
	}
}

func TestAttemptsExplicitDecisionSelection_IgnoresAnalyticalReasonClauses(t *testing.T) {
	t.Parallel()

	candidates := []decisionSelectionCandidate{
		{VariantRef: "V1", Aliases: normalizeDecisionSelectionAliases([]string{"V1", "REST", "variant V1"})},
		{VariantRef: "V2", Aliases: normalizeDecisionSelectionAliases([]string{"V2", "gRPC", "variant V2"})},
	}

	inputs := []string{
		"gRPC because of tooling overhead is risky",
		"REST because latency is worse should be ruled out",
	}

	for _, input := range inputs {
		if attemptsExplicitDecisionSelection(input, candidates) {
			t.Fatalf("attemptsExplicitDecisionSelection(%q) = true, want false", input)
		}
	}
}

func TestSelectionCandidatesForPortfolio_UsesRecoverableLegacyVariants(t *testing.T) {
	t.Parallel()

	portfolio := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:   "sol-legacy",
			Kind: artifact.KindSolutionPortfolio,
		},
		Body: `# Legacy portfolio

## Variants (2)

### V1. REST

### V2. gRPC

## Comparison

Legacy comparison body.
`,
		StructuredData: `{}`,
	}

	candidates, err := selectionCandidatesForPortfolio(portfolio)
	if err != nil {
		t.Fatalf("selectionCandidatesForPortfolio legacy: %v", err)
	}

	selectedRef, ok := detectExplicitDecisionSelection("pick gRPC", candidates)
	if !ok {
		t.Fatal("expected legacy body variants to remain selectable")
	}
	if selectedRef != "V2" {
		t.Fatalf("detectExplicitDecisionSelection legacy = %q, want %q", selectedRef, "V2")
	}
}
