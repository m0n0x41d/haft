package tools

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHaftDecisionToolModelDecideFailsClosedBeforeParsingOrMutation(t *testing.T) {
	fixture := setupDecisionToolFixture(t)
	testCases := map[string]map[string]any{
		"minimal payload": {
			"action": "decide",
		},
		"otherwise valid legacy payload": completeDecisionArgs(map[string]any{
			"action":         "decide",
			"problem_ref":    fixture.problem.Meta.ID,
			"portfolio_ref":  fixture.comparedPortfolio.Meta.ID,
			"selected_title": "gRPC",
			"why_selected":   "The model payload is still proposal content.",
		}),
		"malformed decision fields": {
			"action":         "decide",
			"selected_title": "gRPC",
			"predictions":    []any{map[string]any{"observable": 42}},
		},
	}

	before, err := fixture.store.ListByKind(fixture.ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		t.Fatal(err)
	}

	for name, args := range testCases {
		t.Run(name, func(t *testing.T) {
			_, executeErr := fixture.tool.Execute(fixture.ctx, mustJSON(t, args))
			if executeErr == nil {
				t.Fatal("model decision tool instituted a DecisionRecord without explicit operator authorization")
			}
			if !strings.Contains(executeErr.Error(), "operator_confirmation_required") ||
				!strings.Contains(executeErr.Error(), "haft artifact create decision.decide") {
				t.Fatalf("decision denial did not name the manual path: %v", executeErr)
			}
		})
	}

	after, err := fixture.store.ListByKind(fixture.ctx, artifact.KindDecisionRecord, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("model decide mutated DecisionRecords: before=%d after=%d", len(before), len(after))
	}
}
