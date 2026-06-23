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
	if report.Snapshot.Source != "artifact_store_decision_records" {
		t.Fatalf("snapshot source = %q", report.Snapshot.Source)
	}
	if !strings.HasPrefix(report.Snapshot.SnapshotDigest, "sha256:") {
		t.Fatalf("snapshot digest = %q", report.Snapshot.SnapshotDigest)
	}
	if report.Snapshot.Projection != "refreshable_current_governing_frontier" {
		t.Fatalf("snapshot projection = %q", report.Snapshot.Projection)
	}
	if report.Snapshot.FilterApplied {
		t.Fatal("snapshot filter_applied = true, want false for unfiltered report")
	}
	if !containsString(report.Snapshot.TerminalStatusPolicy, string(StatusSuperseded)) {
		t.Fatalf("terminal status policy = %#v", report.Snapshot.TerminalStatusPolicy)
	}
	if report.AuthorityFrontier.AuthorityBoundary != "current_decision_refs_are_governing_authority_terminal_history_refs_are_not" {
		t.Fatalf("authority_frontier boundary = %q", report.AuthorityFrontier.AuthorityBoundary)
	}
	if len(report.AuthorityFrontier.CurrentDecisionRefs) != 1 || report.AuthorityFrontier.CurrentDecisionRefs[0] != "dec-current" {
		t.Fatalf("authority_frontier current refs = %#v", report.AuthorityFrontier.CurrentDecisionRefs)
	}
	if len(report.AuthorityFrontier.TerminalHistoryRefs) != 1 || report.AuthorityFrontier.TerminalHistoryRefs[0] != "dec-old" {
		t.Fatalf("authority_frontier terminal refs = %#v", report.AuthorityFrontier.TerminalHistoryRefs)
	}
	if !containsString(report.AuthorityFrontier.CurrentStatusPolicy, string(StatusActive)) {
		t.Fatalf("authority_frontier current policy = %#v", report.AuthorityFrontier.CurrentStatusPolicy)
	}
	if !strings.Contains(report.AuthorityFrontier.TerminalHistoryPolicy, "excluded from current authority") {
		t.Fatalf("authority_frontier terminal policy = %q", report.AuthorityFrontier.TerminalHistoryPolicy)
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
	if len(set.AnswerPaths) != 1 {
		t.Fatalf("answer_paths = %#v", set.AnswerPaths)
	}
	if set.AnswerPaths[0].TargetKind != "symbol" {
		t.Fatalf("answer path target_kind = %q", set.AnswerPaths[0].TargetKind)
	}
	if !strings.Contains(set.AnswerPaths[0].CLI, "--target-ref") {
		t.Fatalf("answer path cli = %q", set.AnswerPaths[0].CLI)
	}

	mutated := report
	mutated.Snapshot.GeneratedAt = "2099-01-01T00:00:00Z"
	if currentGoverningSetSnapshotDigest(mutated) != report.Snapshot.SnapshotDigest {
		t.Fatal("snapshot digest should ignore generated_at and reflect governing content")
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

func TestCompactCurrentGoverningSetReportPreservesSummaryAndOmitsAuditSets(t *testing.T) {
	report := CurrentGoverningSetReport{
		SchemaVersion: CurrentGoverningSetSchemaVersion,
		Authority:     CurrentGoverningSetAuthority,
		Summary: CurrentGoverningSetSummary{
			CurrentDecisions: 3,
			GoverningSets:    3,
		},
		AuthorityFrontier: CurrentGoverningAuthorityFrontier{
			AuthorityBoundary:   "current_decision_refs_are_governing_authority_terminal_history_refs_are_not",
			CurrentDecisionRefs: []string{"dec-1", "dec-2", "dec-3"},
			TerminalHistoryRefs: []string{"dec-old"},
		},
		Sets: []CurrentGoverningSet{
			{
				SetID:                    "governing-set-1",
				SubjectRef:               "subject:store",
				SubjectResolution:        "explicit_subject",
				BoundedContext:           "artifact",
				TargetRef:                "symbol:internal/store.go::Save",
				TargetResolution:         "explicit_governance_or_watch_target",
				Posture:                  GoverningSetPostureOverlap,
				CurrentDecisionRefs:      []string{"dec-1", "dec-2"},
				CurrentDecisions:         []DecisionReconciliationItem{{DecisionID: "dec-1"}, {DecisionID: "dec-2"}},
				TerminalHistoryRefs:      []string{"dec-old"},
				AnswerPaths:              currentGoverningSetAnswerPaths("symbol:internal/store.go::Save"),
				OperatorRequired:         true,
				ScopeRepairHints:         []string{"use enrich_scope"},
				WholeFileFallbackTargets: []string{"whole_file_fallback:internal/store.go"},
			},
			{
				SetID:               "governing-set-2",
				SubjectRef:          "subject:load",
				TargetRef:           "symbol:internal/store.go::Load",
				Posture:             GoverningSetPostureSingle,
				CurrentDecisionRefs: []string{"dec-3"},
				CurrentDecisions:    []DecisionReconciliationItem{{DecisionID: "dec-3"}},
			},
			{
				SetID:               "governing-set-3",
				SubjectRef:          "subject:list",
				TargetRef:           "symbol:internal/store.go::List",
				Posture:             GoverningSetPostureSingle,
				CurrentDecisionRefs: []string{"dec-4"},
				CurrentDecisions:    []DecisionReconciliationItem{{DecisionID: "dec-4"}},
			},
		},
	}

	compact := CompactCurrentGoverningSetReport(report, 2)

	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if compact.Summary.GoverningSets != 3 || compact.Summary.CurrentDecisions != 3 {
		t.Fatalf("summary = %#v, want preserved source summary", compact.Summary)
	}
	if len(compact.Sets) != 0 {
		t.Fatalf("compact report should omit full audit sets: %#v", compact.Sets)
	}
	if len(compact.CompactSets) != 2 || compact.OmittedSets != 1 {
		t.Fatalf("compact set count = %d omitted = %d", len(compact.CompactSets), compact.OmittedSets)
	}
	if len(compact.AuthorityFrontier.CurrentDecisionRefs) != 3 {
		t.Fatalf("compact authority frontier current refs = %#v", compact.AuthorityFrontier.CurrentDecisionRefs)
	}
	if len(compact.AuthorityFrontier.TerminalHistoryRefs) != 1 || compact.AuthorityFrontier.TerminalHistoryRefs[0] != "dec-old" {
		t.Fatalf("compact authority frontier terminal refs = %#v", compact.AuthorityFrontier.TerminalHistoryRefs)
	}
	first := compact.CompactSets[0]
	if first.SetID != "governing-set-1" || first.CurrentDecisionCount != 2 || !first.OperatorRequired {
		t.Fatalf("compact first set = %#v", first)
	}
	if len(first.AnswerPaths) != 1 || first.AnswerPaths[0].TargetKind != "symbol" {
		t.Fatalf("answer_paths = %#v", first.AnswerPaths)
	}
	if len(first.TerminalHistoryRefs) != 1 || first.TerminalHistoryRefs[0] != "dec-old" {
		t.Fatalf("terminal_history_refs = %#v", first.TerminalHistoryRefs)
	}
	if !strings.Contains(compact.FullAuditCommand, "full=true") {
		t.Fatalf("full audit command = %q", compact.FullAuditCommand)
	}
	if len(report.Sets) != 3 {
		t.Fatalf("source report mutated: %#v", report.Sets)
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
	if !report.Snapshot.FilterApplied {
		t.Fatal("snapshot filter_applied = false, want true for filtered report")
	}
	if !strings.HasPrefix(report.Snapshot.SnapshotDigest, "sha256:") {
		t.Fatalf("filtered snapshot digest = %q", report.Snapshot.SnapshotDigest)
	}
	if report.Summary.GoverningSets != 1 || report.Summary.CurrentDecisions != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if len(report.Sets) != 1 || report.Sets[0].CurrentDecisionRefs[0] != "dec-save" {
		t.Fatalf("sets = %#v", report.Sets)
	}

	full, err := BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		t.Fatalf("BuildCurrentGoverningSetReport: %v", err)
	}
	if full.Snapshot.SnapshotDigest == report.Snapshot.SnapshotDigest {
		t.Fatal("filtered snapshot digest should differ from the unfiltered frontier digest")
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

func TestCurrentGoverningTargetKindClassifiesExactAnswerTargets(t *testing.T) {
	cases := map[string]string{
		"dec-1#claim-a":                     "claim",
		"claim:dec-1#claim-a":               "claim",
		"spec-section:system-boundary":      "spec_section",
		"spec_section:system-boundary":      "spec_section",
		"api_contract:haft_query/status":    "api_contract",
		"api-contract:haft_query/status":    "api_contract",
		"invariant:decision-terminal-state": "invariant",
		"symbol:internal/store.go::Save":    "symbol",
		"whole_file_fallback:internal/x.go": "whole_file_fallback",
		"whole-file-fallback:internal/x.go": "whole_file_fallback",
		"file:internal/x.go":                "file_fallback",
		"unscoped:dec-1":                    "unscoped_decision",
	}
	for targetRef, want := range cases {
		if got := currentGoverningTargetKind(targetRef); got != want {
			t.Fatalf("target kind for %q = %q, want %q", targetRef, got, want)
		}
	}
}

func TestCurrentGoverningSetAnswerPathsNameExactDrilldowns(t *testing.T) {
	cases := map[string]string{
		"claim:dec-1#claim-a":               "claim lifecycle/detail view",
		"spec_section:system-boundary":      "haft spec section lifecycle/detail",
		"api_contract:haft_query/status":    "interface contract or exact API-contract carrier",
		"invariant:decision-terminal-state": "decision invariant or evidence path detail",
		"symbol:internal/store.go::Save":    "haft_query code_context/node for symbol plus governing-set filtered JSON",
		"whole_file_fallback:internal/x.go": "scope enrichment selection before stronger use",
		"file:internal/x.go":                "scope enrichment selection before stronger use",
		"unscoped:dec-1":                    "decision scope enrichment before stronger use",
	}
	for targetRef, want := range cases {
		paths := currentGoverningSetAnswerPaths(targetRef)
		if len(paths) != 1 {
			t.Fatalf("answer paths for %q = %#v", targetRef, paths)
		}
		path := paths[0]
		if path.TargetRef != targetRef {
			t.Fatalf("answer path target_ref = %q, want %q", path.TargetRef, targetRef)
		}
		if !strings.Contains(path.CLI, "--target-ref") {
			t.Fatalf("answer path cli = %q", path.CLI)
		}
		if !strings.Contains(path.MCPCall, "source_refs") {
			t.Fatalf("answer path mcp_call = %q", path.MCPCall)
		}
		if path.ExactRecordNeeded != want {
			t.Fatalf("answer path exact_record_needed for %q = %q, want %q", targetRef, path.ExactRecordNeeded, want)
		}
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
