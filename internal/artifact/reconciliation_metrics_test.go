package artifact

import "testing"

func TestBuildReconciliationMetricsPacketSummarizesDogfoodBeforeAfterSignals(t *testing.T) {
	plan := DecisionReconciliationPlan{
		Summary: DecisionReconciliationSummary{
			ReviewedDecisions:         10,
			Groups:                    6,
			WholeFileFallbackOnly:     3,
			MissingExplicitSubject:    4,
			ScopeEnrichmentCandidates: 5,
			ConflictRequiresOperator:  1,
		},
	}
	governing := CurrentGoverningSetReport{
		Summary: CurrentGoverningSetSummary{
			CurrentDecisions:       9,
			GoverningSets:          7,
			FallbackTargetSets:     2,
			ScopeEnrichmentSets:    3,
			ConflictSets:           1,
			OverlapReviewSets:      2,
			MissingExplicitSubject: 4,
			TerminalHistoryRefs:    8,
		},
	}
	driftEvents := DriftEventReport{
		Summary: DriftEventSummary{
			UniqueEvents:                 11,
			ImpactedDecisions:            13,
			MaterialEvents:               6,
			AuditOnlyEvents:              4,
			NeedsBindingResolutionEvents: 5,
			SemanticTargetEvents:         7,
			FileFallbackEvents:           2,
			UnknownHighRiskEvents:        1,
			MaxFanout:                    9,
		},
	}

	packet := BuildReconciliationMetricsPacket(plan, governing, driftEvents)

	if packet.Authority != ReconciliationMetricsAuthority {
		t.Fatalf("authority = %q", packet.Authority)
	}
	if packet.Reconciliation.ScopeEnrichmentCandidates != 5 {
		t.Fatalf("scope_enrichment_candidates = %d", packet.Reconciliation.ScopeEnrichmentCandidates)
	}
	if packet.GoverningSet.FallbackTargetSets != 2 {
		t.Fatalf("fallback_target_sets = %d", packet.GoverningSet.FallbackTargetSets)
	}
	if packet.GoverningSet.ConflictSets != 1 {
		t.Fatalf("conflict_sets = %d", packet.GoverningSet.ConflictSets)
	}
	if packet.DriftEvents.UniqueEvents != 11 || packet.DriftEvents.MaxFanout != 9 {
		t.Fatalf("drift event metrics = %#v", packet.DriftEvents)
	}
	if packet.BeforeAfterUse.RequiredAuthority != "operator_approved_reconciliation_selection" {
		t.Fatalf("required authority = %q", packet.BeforeAfterUse.RequiredAuthority)
	}
	if len(packet.BeforeAfterUse.MutationBoundary) == 0 {
		t.Fatal("mutation boundary missing")
	}
}
