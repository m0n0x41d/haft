package artifact

import "testing"

func TestBuildReconciliationCueReportSummarizesReadOnlyReviewLanes(t *testing.T) {
	report := BuildReconciliationCueReport(
		DriftEventReport{
			Summary: DriftEventSummary{MaxFanout: 3},
			Events: []DriftEvent{
				{Fanout: 3},
				{Fanout: 1},
			},
		},
		DecisionReconciliationPlan{
			Groups: []DecisionReconciliationGroup{
				{Category: DecisionReconciliationKeep},
				{Category: DecisionReconciliationMergeCandidate, OperatorRequired: true},
				{Category: DecisionReconciliationReopenCandidate, OperatorRequired: true},
			},
		},
		CurrentGoverningSetReport{
			Summary: CurrentGoverningSetSummary{
				ConflictSets:      1,
				OverlapReviewSets: 2,
			},
		},
	)

	if report.Authority != ReconciliationCueAuthority {
		t.Fatalf("authority = %q, want %q", report.Authority, ReconciliationCueAuthority)
	}
	if report.Summary.HighFanoutEvents != 1 || report.Summary.MaxFanout != 3 {
		t.Fatalf("fanout summary = %#v", report.Summary)
	}
	if report.Summary.ReconciliationGroups != 2 || report.Summary.OperatorRequiredGroups != 2 {
		t.Fatalf("reconciliation summary = %#v", report.Summary)
	}
	if report.Summary.GoverningConflictSets != 1 || report.Summary.GoverningOverlapSets != 2 {
		t.Fatalf("governing summary = %#v", report.Summary)
	}
	for _, want := range []string{
		`haft_query(action="drift_events")`,
		`haft_query(action="decision_reconcile")`,
		`haft_query(action="governing_set")`,
	} {
		if !containsString(report.Commands, want) {
			t.Fatalf("commands missing %q in %#v", want, report.Commands)
		}
	}
	if len(report.Cues) != 3 {
		t.Fatalf("cues = %#v, want three review cues", report.Cues)
	}
}

func TestBuildReconciliationCueReportStaysEmptyWithoutReviewSignals(t *testing.T) {
	report := BuildReconciliationCueReport(
		DriftEventReport{Events: []DriftEvent{{Fanout: 1}}},
		DecisionReconciliationPlan{Groups: []DecisionReconciliationGroup{{Category: DecisionReconciliationKeep}}},
		CurrentGoverningSetReport{},
	)

	if len(report.Cues) != 0 || len(report.Commands) != 0 {
		t.Fatalf("empty report should have no cues or commands: %#v", report)
	}
}
