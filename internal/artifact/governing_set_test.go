package artifact

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBuildCurrentGoverningSetExcludesTerminalHistory(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	target := governingSetTestTarget("internal/store.go", "Save")
	createDecisionForReconciliation(t, store, "dec-old", StatusSuperseded, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:store-write",
		DriftWatchTargets: []DriftWatchTarget{{
			TargetRef:     "symbol:internal/store.go::Save",
			Trigger:       "symbol_body_changed",
			BindingTarget: &target,
		}},
	}, now)
	createDecisionForReconciliation(t, store, "dec-current", StatusActive, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:store-write",
		DriftWatchTargets: []DriftWatchTarget{{
			TargetRef:     "symbol:internal/store.go::Save",
			Trigger:       "symbol_body_changed",
			BindingTarget: &target,
		}},
	}, now)
	if err := store.AddLink(ctx, "dec-current", "dec-old", "supersedes"); err != nil {
		t.Fatalf("AddLink: %v", err)
	}

	report, err := BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		t.Fatalf("BuildCurrentGoverningSetReport: %v", err)
	}

	if report.Summary.CurrentDecisions != 1 {
		t.Fatalf("current_decisions = %d, want 1", report.Summary.CurrentDecisions)
	}
	if len(report.Sets) != 1 {
		t.Fatalf("sets = %#v", report.Sets)
	}
	set := report.Sets[0]
	if len(set.CurrentDecisionRefs) != 1 || set.CurrentDecisionRefs[0] != "dec-current" {
		t.Fatalf("current refs = %#v", set.CurrentDecisionRefs)
	}
	if len(set.TerminalHistoryRefs) != 1 || set.TerminalHistoryRefs[0] != "dec-old" {
		t.Fatalf("terminal history refs = %#v", set.TerminalHistoryRefs)
	}
	if set.Posture != GoverningSetPostureSingle {
		t.Fatalf("posture = %q", set.Posture)
	}
}

func TestBuildCurrentGoverningSetFlagsExplicitConflict(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	target := governingSetTestTarget("internal/server.go", "Serve")
	for _, id := range []string{"dec-a", "dec-b"} {
		createDecisionForReconciliation(t, store, id, StatusActive, "mcp", DecisionFields{
			DecisionSubjectRef: "subject:mcp-binding-mode",
			GovernanceTargets: []GovernanceTarget{{
				Kind:          "symbol",
				Ref:           "symbol:internal/server.go::Serve",
				BindingTarget: &target,
			}},
		}, now)
	}
	if err := store.AddLink(ctx, "dec-a", "dec-b", "contradicts"); err != nil {
		t.Fatalf("AddLink: %v", err)
	}

	report, err := BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		t.Fatalf("BuildCurrentGoverningSetReport: %v", err)
	}

	if report.Summary.ConflictSets != 1 {
		t.Fatalf("conflict_sets = %d, want 1", report.Summary.ConflictSets)
	}
	if len(report.Sets) != 1 {
		t.Fatalf("sets = %#v", report.Sets)
	}
	set := report.Sets[0]
	if set.Posture != GoverningSetPostureConflict {
		t.Fatalf("posture = %q", set.Posture)
	}
	if !set.OperatorRequired {
		t.Fatal("conflict set should require operator")
	}
}

func TestBuildCurrentGoverningSetKeepsMissingSubjectAndTargetUnique(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-a", StatusActive, "runtime", DecisionFields{}, now)
	createDecisionForReconciliation(t, store, "dec-b", StatusActive, "runtime", DecisionFields{}, now)

	report, err := BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		t.Fatalf("BuildCurrentGoverningSetReport: %v", err)
	}

	if report.Summary.GoverningSets != 2 {
		t.Fatalf("governing_sets = %d, want 2", report.Summary.GoverningSets)
	}
	if report.Summary.OverlapReviewSets != 0 || report.Summary.ConflictSets != 0 {
		t.Fatalf("summary = %#v, want no overlap/conflict for unscoped decisions", report.Summary)
	}
	for _, set := range report.Sets {
		if len(set.CurrentDecisionRefs) != 1 {
			t.Fatalf("unscoped decisions should stay unique: %#v", set)
		}
		if set.TargetResolution != "missing_explicit_target_unique_decision_scope" {
			t.Fatalf("target_resolution = %q", set.TargetResolution)
		}
	}
}

func TestBuildCurrentGoverningSetSurfacesFallbackScopeRepair(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDecisionForReconciliation(t, store, "dec-fallback", StatusActive, "drift", DecisionFields{
		BindingTargets: []BindingTarget{{
			Kind:     BindingTargetWholeFileFallback,
			FilePath: "internal/shared.go",
			Reason:   "unsupported language",
		}},
	}, now)

	report, err := BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		t.Fatalf("BuildCurrentGoverningSetReport: %v", err)
	}

	if report.Summary.FallbackTargetSets != 1 {
		t.Fatalf("fallback_target_sets = %d, want 1", report.Summary.FallbackTargetSets)
	}
	if report.Summary.ScopeEnrichmentSets != 1 {
		t.Fatalf("scope_enrichment_sets = %d, want 1", report.Summary.ScopeEnrichmentSets)
	}
	if len(report.Sets) != 1 {
		t.Fatalf("sets = %#v", report.Sets)
	}
	set := report.Sets[0]
	if set.TargetResolution != "whole_file_fallback_requires_scope_enrichment" {
		t.Fatalf("target_resolution = %q", set.TargetResolution)
	}
	if len(set.WholeFileFallbackTargets) != 1 || set.WholeFileFallbackTargets[0] != "whole_file_fallback:internal/shared.go" {
		t.Fatalf("whole_file_fallback_targets = %#v", set.WholeFileFallbackTargets)
	}
	if len(set.ScopeRepairHints) != 1 || !strings.Contains(set.ScopeRepairHints[0], "whole-file fallback") {
		t.Fatalf("scope_repair_hints = %#v", set.ScopeRepairHints)
	}
}

func TestFilterCurrentGoverningSetReportBySubjectAndTarget(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	saveTarget := governingSetTestTarget("internal/store.go", "Save")
	loadTarget := governingSetTestTarget("internal/store.go", "Load")
	createDecisionForReconciliation(t, store, "dec-save", StatusActive, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:store-write",
		DriftWatchTargets: []DriftWatchTarget{{
			TargetRef:     "symbol:internal/store.go::Save",
			Trigger:       "symbol_body_changed",
			BindingTarget: &saveTarget,
		}},
	}, now)
	createDecisionForReconciliation(t, store, "dec-load", StatusActive, "artifact", DecisionFields{
		DecisionSubjectRef: "subject:store-read",
		DriftWatchTargets: []DriftWatchTarget{{
			TargetRef:     "symbol:internal/store.go::Load",
			Trigger:       "symbol_body_changed",
			BindingTarget: &loadTarget,
		}},
	}, now)

	report, err := BuildCurrentGoverningSetReportFiltered(ctx, store, CurrentGoverningSetFilter{
		SubjectRef: "subject:store-write",
		TargetRef:  "symbol:internal/store.go::Save",
	})
	if err != nil {
		t.Fatalf("BuildCurrentGoverningSetReportFiltered: %v", err)
	}

	if report.Filter == nil {
		t.Fatal("filter missing")
	}
	if report.Summary.GoverningSets != 1 || report.Summary.CurrentDecisions != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if len(report.Sets) != 1 || report.Sets[0].CurrentDecisionRefs[0] != "dec-save" {
		t.Fatalf("sets = %#v", report.Sets)
	}
}

func TestFilterCurrentGoverningSetReportByQuery(t *testing.T) {
	report := FilterCurrentGoverningSetReport(CurrentGoverningSetReport{
		SchemaVersion: CurrentGoverningSetSchemaVersion,
		Authority:     CurrentGoverningSetAuthority,
		Sets: []CurrentGoverningSet{
			{
				SetID:               "governing-set-save",
				SubjectRef:          "subject:store-write",
				TargetRef:           "symbol:internal/store.go::Save",
				Posture:             GoverningSetPostureSingle,
				CurrentDecisionRefs: []string{"dec-save"},
				CurrentDecisions:    []DecisionReconciliationItem{{DecisionID: "dec-save"}},
			},
			{
				SetID:               "governing-set-load",
				SubjectRef:          "subject:store-read",
				TargetRef:           "symbol:internal/store.go::Load",
				Posture:             GoverningSetPostureSingle,
				CurrentDecisionRefs: []string{"dec-load"},
				CurrentDecisions:    []DecisionReconciliationItem{{DecisionID: "dec-load"}},
			},
		},
	}, CurrentGoverningSetFilter{Query: "Load"})

	if report.Filter == nil || report.Filter.Query != "Load" {
		t.Fatalf("filter = %#v", report.Filter)
	}
	if report.Summary.GoverningSets != 1 || report.Summary.CurrentDecisions != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if len(report.Sets) != 1 || report.Sets[0].TargetRef != "symbol:internal/store.go::Load" {
		t.Fatalf("sets = %#v", report.Sets)
	}
}

func governingSetTestTarget(filePath string, symbolName string) BindingTarget {
	return BindingTarget{
		Kind:       BindingTargetSymbol,
		FilePath:   filePath,
		SymbolKind: "func",
		SymbolName: symbolName,
		BodyHash:   "hash-" + symbolName,
	}
}
