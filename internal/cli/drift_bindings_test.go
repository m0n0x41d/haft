package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestWriteDriftBindingsSummaryNamesActions(t *testing.T) {
	report := artifact.LegacyBindingReport{
		Authority: artifact.LegacyBindingAuthority,
		Summary: artifact.LegacyBindingSummary{
			TotalDecisions:          2,
			HighConfidenceProposals: 1,
			NeedsOperatorSelection:  1,
			AmbiguousFileScope:      1,
			AlreadyPrecise:          0,
		},
		Items: []artifact.LegacyBindingItem{
			{
				DecisionID:           "dec-one",
				DecisionTitle:        "One symbol",
				Posture:              artifact.LegacyBindingPostureMissingSymbolBaseline,
				RecommendedAction:    artifact.LegacyBindingActionProposeRebaseline,
				CandidateSymbolCount: 1,
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDriftBindingsSummary(&buf, report); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	for _, want := range []string{
		"haft drift bindings",
		"read_only_symbol_binding_proposal",
		"high_confidence=1",
		"One symbol `dec-one`",
		"action=propose_rebaseline_with_symbols",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("summary missing %q:\n%s", want, output)
		}
	}
}
