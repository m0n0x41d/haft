package agent

import (
	"strings"
	"testing"
)

func TestComparerAgent_PromptRequiresParetoFrontExplanation(t *testing.T) {
	t.Parallel()

	prompt := ComparerAgent().SystemPrompt
	required := []string{
		`Per-variant score summary with evidence`,
		`Dominated-variant elimination`,
		`Trade-off explanation for each Pareto-front variant`,
		`Do NOT ask for the human's choice until after you show the elimination reasoning, Pareto front, and trade-offs.`,
		`selected_ref, treat it as an advisory recommendation only`,
	}

	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("comparer prompt missing %q", want)
		}
	}
}
