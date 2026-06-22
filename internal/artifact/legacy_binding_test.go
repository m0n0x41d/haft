package artifact

import (
	"context"
	"testing"
)

func TestBuildLegacyBindingReportProposesSingleSymbolRebaseline(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}
`)

	dec := createTestDecision(t, store, "dec-legacy-bind-001", "Legacy single symbol")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}

	report, err := BuildLegacyBindingReport(ctx, store, projectRoot, LegacyBindingOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if report.Authority != LegacyBindingAuthority {
		t.Fatalf("authority = %q, want %q", report.Authority, LegacyBindingAuthority)
	}
	if report.Summary.HighConfidenceProposals != 1 {
		t.Fatalf("high-confidence proposals = %d, want 1", report.Summary.HighConfidenceProposals)
	}
	if len(report.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(report.Items))
	}

	item := report.Items[0]
	if item.RecommendedAction != LegacyBindingActionProposeRebaseline {
		t.Fatalf("action = %q, want %q", item.RecommendedAction, LegacyBindingActionProposeRebaseline)
	}
	if !item.HighConfidence {
		t.Fatal("single-symbol proposal should be high-confidence")
	}
	if len(item.CandidateSymbols) != 1 || item.CandidateSymbols[0].SymbolName != "Run" {
		t.Fatalf("candidate symbols = %+v, want Run", item.CandidateSymbols)
	}
}

func TestBuildLegacyBindingReportSurfacesAmbiguousSymbolSelection(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}

func Stop() string {
	return "stop"
}
`)

	dec := createTestDecision(t, store, "dec-legacy-bind-002", "Legacy multi symbol")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}

	report, err := BuildLegacyBindingReport(ctx, store, projectRoot, LegacyBindingOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if report.Summary.NeedsOperatorSelection != 1 {
		t.Fatalf("needs operator selection = %d, want 1", report.Summary.NeedsOperatorSelection)
	}
	item := report.Items[0]
	if item.Posture != LegacyBindingPostureMissingSymbolBaseline {
		t.Fatalf("posture = %q, want %q", item.Posture, LegacyBindingPostureMissingSymbolBaseline)
	}
	if item.RecommendedAction != LegacyBindingActionNeedsOperatorSelect {
		t.Fatalf("action = %q, want %q", item.RecommendedAction, LegacyBindingActionNeedsOperatorSelect)
	}
}

func TestBuildLegacyBindingReportSkipsCarrierGeneratedOnlyByDefault(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "CHANGELOG.md", "# changes\n")
	writeTestFile(t, projectRoot, "internal/cli/fpf.db", "sqlite-ish\n")

	dec := createTestDecision(t, store, "dec-legacy-bind-003", "Carrier-only decision")
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{
		{Path: "CHANGELOG.md"},
		{Path: "internal/cli/fpf.db"},
	}); err != nil {
		t.Fatal(err)
	}

	report, err := BuildLegacyBindingReport(ctx, store, projectRoot, LegacyBindingOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.CarrierOrGeneratedOnly != 1 {
		t.Fatalf("carrier/generated count = %d, want 1", report.Summary.CarrierOrGeneratedOnly)
	}
	if len(report.Items) != 0 {
		t.Fatalf("carrier/generated no-action item should be omitted by default: %+v", report.Items)
	}

	report, err = BuildLegacyBindingReport(ctx, store, projectRoot, LegacyBindingOptions{IncludeClean: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("include-clean items = %d, want 1", len(report.Items))
	}
	if report.Items[0].Posture != LegacyBindingPostureCarrierOnly {
		t.Fatalf("posture = %q, want %q", report.Items[0].Posture, LegacyBindingPostureCarrierOnly)
	}
}
