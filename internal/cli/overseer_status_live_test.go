package cli

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/project"
)

func TestLiveDriftStatusSignalUsesActionablePartitions(t *testing.T) {
	t.Parallel()

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
		"Current drift needs scoped inspection: 1 material event(s), 1 binding-resolution event(s)",
		"derived from current DriftEventReport",
		"1 audit-only/resolved event(s)",
		"attention, not a project-wide Work gate",
		"inspect exact affected authority before interrupting current Work",
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
	t.Parallel()

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

func TestNonProfileSpecHealthStatusSignalsDropsStoredAndPriorLiveSignals(
	t *testing.T,
) {
	t.Parallel()

	signals := nonProfileSpecHealthStatusSignals([]overseer.StatusSignal{
		{
			Source: "spec_health",
			Title:  "Spec health finding from an old maintenance run",
		},
		{
			Source: "scoped_spec_health",
			Title:  "Scoped spec health finding from an old review packet",
		},
		{
			Source: currentScopeSpecHealthStatusSource,
			Title:  "Prior current-scope projection",
		},
		{
			Source: "staleness",
			Title:  "Unrelated stale governance artifact",
		},
	})

	if len(signals) != 1 {
		t.Fatalf("signals = %#v", signals)
	}
	if signals[0].Source != "staleness" {
		t.Fatalf("remaining signal = %#v", signals[0])
	}
}

func TestCurrentScopeSpecHealthStatusSignalPreservesSoftwareFinding(
	t *testing.T,
) {
	t.Parallel()

	signals := currentScopeSpecHealthStatusSignals(
		[]project.SpecCheckFinding{
			{
				Level:   "error",
				Code:    "spec_carrier_missing_file",
				Path:    ".haft/specs/software-system.md",
				Message: "software-system spec carrier is missing",
			},
		},
		"software-main",
	)

	if len(signals) != 1 {
		t.Fatalf("signals = %#v", signals)
	}
	signal := signals[0]
	if signal.Severity != "high" ||
		signal.Source != currentScopeSpecHealthStatusSource ||
		!strings.Contains(signal.Detail, "software-system") ||
		signal.Command != "haft spec check --scope-id software-main" {
		t.Fatalf("signal = %#v", signal)
	}
}
