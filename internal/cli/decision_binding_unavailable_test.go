package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNonManualDecisionRecordWritesFailClosed(t *testing.T) {
	t.Run("internal helper", func(t *testing.T) {
		_, err := createArtifactFromInput(
			context.Background(),
			nil,
			"/unused/.haft",
			"decision.decide",
			[]byte(`{}`),
		)
		assertDecisionBindingUnavailable(t, err)
	})

	t.Run("MCP handler", func(t *testing.T) {
		_, _, err := handleQuintDecision(
			context.Background(),
			nil,
			"/unused/.haft",
			map[string]any{"action": "decide"},
		)
		assertDecisionBindingUnavailable(t, err)
	})
}

func assertDecisionBindingUnavailable(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, errDecisionBindingUnavailable) {
		t.Fatalf("error = %v, want %v", err, errDecisionBindingUnavailable)
	}
	for _, want := range []string{
		"operator_confirmation_required",
		"explicit_h_decide",
		"trusts that external invocation by project policy",
		"kernel neither observes it nor records a durable authorization receipt",
		"strict_cli_speech_act",
		"durable controlling-terminal SpeechAct",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want %q", err, want)
		}
	}
}
