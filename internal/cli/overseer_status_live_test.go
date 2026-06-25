package cli

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
)

func TestLiveDriftStatusSignalUsesActionablePartitions(t *testing.T) {
	report := artifact.BuildDriftEventReport([]artifact.DriftReport{
		{
			DecisionID: "dec-material",
			Files: []artifact.DriftItem{{
				Path:        "internal/service.go",
				Status:      artifact.DriftModified,
				Materiality: artifact.DriftMaterialityMaterialSymbol,
				Symbols: []artifact.SymbolDriftItem{{
					SymbolName: "Run",
					SymbolKind: "func",
					Status:     "modified",
				}},
			}},
		},
		{
			DecisionID: "dec-binding",
			Files: []artifact.DriftItem{{
				Path:           "internal/legacy.txt",
				Status:         artifact.DriftModified,
				Materiality:    artifact.DriftMaterialityNeedsBindingResolution,
				FallbackKind:   artifact.BindingTargetWholeFileFallback,
				FallbackReason: "unsupported language",
			}},
		},
		{
			DecisionID: "dec-audit",
			Files: []artifact.DriftItem{{
				Path:        "CHANGELOG.md",
				Status:      artifact.DriftModified,
				Materiality: artifact.DriftMaterialityCarrierOnly,
				AuditOnly:   true,
			}},
		},
	})

	signal, ok := liveDriftStatusSignal(report)
	if !ok {
		t.Fatal("expected live drift signal")
	}
	for _, want := range []string{
		"Current drift needs operator review: 1 material event(s), 1 binding-resolution event(s)",
		"derived from current DriftEventReport",
		"1 audit-only/resolved event(s)",
	} {
		got := signal.Title + " " + signal.Detail
		if !strings.Contains(got, want) {
			t.Fatalf("live drift signal missing %q: %#v", want, signal)
		}
	}
	if strings.Contains(signal.Title, "3") {
		t.Fatalf("live drift signal should not headline total unique events: %#v", signal)
	}
}

func TestNonDriftStatusSignalsDropsStoredDriftSignals(t *testing.T) {
	signals := nonDriftStatusSignals([]overseer.StatusSignal{
		{
			Source: "drift",
			Title:  "Drift requires confirmation: old run",
		},
		{
			Source: "scoped_drift",
			Title:  "Scoped drift: old run",
		},
		{
			Source: "staleness",
			Title:  "Stale governance artifact",
		},
	})

	if len(signals) != 1 {
		t.Fatalf("signals = %#v", signals)
	}
	if signals[0].Source != "staleness" {
		t.Fatalf("remaining signal = %#v", signals[0])
	}
}
