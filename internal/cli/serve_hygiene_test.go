package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/present"
)

func TestHandleQuintProblem_CharacterizeMissingProblemUsesPlainLanguage(t *testing.T) {
	store := setupCLIArtifactStore(t)

	result, err := handleQuintProblem(context.Background(), store, t.TempDir(), map[string]any{
		"action": "characterize",
		"dimensions": []any{
			map[string]any{"name": "latency"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, "ProblemCard") {
		t.Fatalf("expected plain-language missing-problem response, got %q", result)
	}
	if !strings.Contains(result, "No active problem found.") {
		t.Fatalf("expected plain-language missing-problem response, got %q", result)
	}
	if issues := present.LintGeneratedText(result); len(issues) != 0 {
		t.Fatalf("expected lint-clean generated message, got %+v\n%s", issues, result)
	}
}

func TestHandleQuintDecision_ApplyMissingDecisionUsesPlainLanguage(t *testing.T) {
	store := setupCLIArtifactStore(t)

	result, err := handleQuintDecision(context.Background(), store, t.TempDir(), map[string]any{
		"action": "apply",
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, "DecisionRecord") {
		t.Fatalf("expected plain-language missing-decision response, got %q", result)
	}
	if !strings.Contains(result, "No decision found.") {
		t.Fatalf("expected plain-language missing-decision response, got %q", result)
	}
	if issues := present.LintGeneratedText(result); len(issues) != 0 {
		t.Fatalf("expected lint-clean generated message, got %+v\n%s", issues, result)
	}
}
