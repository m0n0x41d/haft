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
		"cannot verify conversational provenance",
		"direct, unambiguous operator request",
		"host_routed_operator_request",
		"haft artifact create decision.decide --input-file",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want %q", err, want)
		}
	}
}
