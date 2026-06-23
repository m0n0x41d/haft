package cli

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestGovernorAttentionUsesDriftEventsNotPerDecisionDrift(t *testing.T) {
	items := governorAttention(artifact.StatusData{
		Drift: []artifact.DriftReport{
			{
				DecisionID:    "dec-a",
				DecisionTitle: "First noisy decision",
				Files: []artifact.DriftItem{{
					Path:   "internal/shared.go",
					Status: artifact.DriftModified,
				}},
			},
			{
				DecisionID:    "dec-b",
				DecisionTitle: "Second noisy decision",
				Files: []artifact.DriftItem{{
					Path:   "internal/shared.go",
					Status: artifact.DriftModified,
				}},
			},
		},
	})

	joined := strings.Join(items, "\n")
	if !strings.Contains(joined, "drift-events: 1 unique, 2 impacted decision(s), max fanout 2") {
		t.Fatalf("drift event summary missing:\n%s", joined)
	}
	if !strings.Contains(joined, artifact.StatusCompactDriftEventsCommand) {
		t.Fatalf("drill-down command missing:\n%s", joined)
	}
	for _, unwanted := range []string{"First noisy decision", "Second noisy decision", "dec-a", "dec-b"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("governor attention leaked per-decision drift %q:\n%s", unwanted, joined)
		}
	}
}
