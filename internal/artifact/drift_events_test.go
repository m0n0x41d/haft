package artifact

import (
	"strings"
	"testing"
	"time"
)

func TestBuildDriftEventReportGroupsSharedFileFanout(t *testing.T) {
	reports := []DriftReport{
		{
			DecisionID:    "dec-1",
			DecisionTitle: "First decision",
			Files: []DriftItem{{
				Path:        "shared.go",
				Status:      DriftModified,
				TriggerKind: DriftTriggerFileHash,
				Materiality: DriftMaterialityMaterialSymbol,
			}},
		},
		{
			DecisionID:    "dec-2",
			DecisionTitle: "Second decision",
			Files: []DriftItem{{
				Path:        "shared.go",
				Status:      DriftModified,
				TriggerKind: DriftTriggerFileHash,
				Materiality: DriftMaterialityMaterialSymbol,
			}},
		},
	}

	report := BuildDriftEventReport(reports)

	if report.SchemaVersion != 2 {
		t.Fatalf("schema_version = %d, want 2", report.SchemaVersion)
	}
	if report.Summary.UniqueEvents != 1 {
		t.Fatalf("unique_events = %d, want 1", report.Summary.UniqueEvents)
	}
	if report.Summary.ImpactedDecisions != 2 {
		t.Fatalf("impacted_decisions = %d, want 2", report.Summary.ImpactedDecisions)
	}
	if len(report.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(report.Events))
	}
	event := report.Events[0]
	if event.Fanout != 2 {
		t.Fatalf("fanout = %d, want 2", event.Fanout)
	}
	if event.ChangedTargetRef != "file:shared.go" {
		t.Fatalf("changed_target_ref = %q", event.ChangedTargetRef)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsOperatorJudgment {
		t.Fatalf("resolution_status = %q, want %s", event.ResolutionStatus, DriftEventResolutionNeedsOperatorJudgment)
	}
	if event.SuggestedNextCommand != `haft_refresh(action="review")` {
		t.Fatalf("suggested_next_command = %q, want review", event.SuggestedNextCommand)
	}
	if len(event.ImpactedDecisions) != 2 {
		t.Fatalf("impacted decisions = %#v", event.ImpactedDecisions)
	}
}

func TestCompactDriftEventReportKeepsSummaryAndOmissions(t *testing.T) {
	report := DriftEventReport{
		SchemaVersion: 2,
		Summary: DriftEventSummary{
			UniqueEvents:      3,
			ImpactedDecisions: 2,
		},
		Events: []DriftEvent{
			{
				EventID:          "drift-event-1",
				ChangedTargetRef: "file:one.go",
				SourceItems: []DriftEventSourceItem{
					{DecisionID: "dec-1", Path: "one.go"},
					{DecisionID: "dec-2", Path: "one.go"},
				},
			},
			{EventID: "drift-event-2", ChangedTargetRef: "file:two.go"},
			{EventID: "drift-event-3", ChangedTargetRef: "file:three.go"},
		},
		Compatibility: []DriftReport{
			{DecisionID: "dec-1"},
			{DecisionID: "dec-2"},
		},
	}

	compact := CompactDriftEventReport(report, 2)

	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if compact.Summary.UniqueEvents != 3 {
		t.Fatalf("summary unique_events = %d, want 3", compact.Summary.UniqueEvents)
	}
	if len(compact.Events) != 2 {
		t.Fatalf("events = %d, want capped 2", len(compact.Events))
	}
	if compact.OmittedEvents != 1 {
		t.Fatalf("omitted_events = %d, want 1", compact.OmittedEvents)
	}
	if len(compact.Compatibility) != 0 {
		t.Fatalf("compact report should omit compatibility reports: %#v", compact.Compatibility)
	}
	if compact.OmittedCompatibilityReports != 2 {
		t.Fatalf("omitted compatibility reports = %d, want 2", compact.OmittedCompatibilityReports)
	}
	if len(compact.Events[0].SourceItems) != 0 {
		t.Fatalf("compact event should omit source_items: %#v", compact.Events[0].SourceItems)
	}
	if compact.Events[0].OmittedSourceItems != 2 {
		t.Fatalf("omitted_source_items = %d, want 2", compact.Events[0].OmittedSourceItems)
	}
	if !strings.Contains(compact.FullAuditCommand, `full=true`) {
		t.Fatalf("full audit command should name full=true: %q", compact.FullAuditCommand)
	}
	if len(report.Events[0].SourceItems) != 2 {
		t.Fatalf("compact projection mutated source report: %#v", report.Events[0].SourceItems)
	}
}

func TestBuildDriftEventReportUsesSymbolTargetWhenMaterialSymbolKnown(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityMaterialSymbol,
			Symbols: []SymbolDriftItem{
				{SymbolName: "Run", SymbolKind: "func", Status: "modified"},
				{SymbolName: "Stop", SymbolKind: "func", Status: "removed"},
			},
		}},
	}})

	if report.Summary.UniqueEvents != 2 {
		t.Fatalf("unique_events = %d, want 2", report.Summary.UniqueEvents)
	}
	if report.Summary.SemanticTargetEvents != 2 {
		t.Fatalf("semantic_target_events = %d, want 2", report.Summary.SemanticTargetEvents)
	}
	event := report.Events[0]
	if event.TargetKind != "symbol" {
		t.Fatalf("target_kind = %q, want symbol", event.TargetKind)
	}
	if event.RootCause != DriftEventRootCauseSemanticTargetChanged && event.RootCause != DriftEventRootCauseTargetDeleted {
		t.Fatalf("root_cause = %q, want semantic_target_changed or target_deleted", event.RootCause)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsOperatorJudgment {
		t.Fatalf("resolution_status = %q, want %s", event.ResolutionStatus, DriftEventResolutionNeedsOperatorJudgment)
	}
	if len(event.SourceItems) != 1 || len(event.SourceItems[0].Symbols) != 2 {
		t.Fatalf("source items should preserve symbol details: %#v", event.SourceItems)
	}
}

func TestBuildDriftEventReportClassifiesAuditOnlyEvents(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:             "README.md",
			Status:           DriftModified,
			TriggerKind:      DriftTriggerFileHash,
			Materiality:      DriftMaterialityCarrierOnly,
			AuditOnly:        true,
			SuppressedReason: "carrier path changed; no code-object symbol drift",
		}},
	}})

	if report.Summary.AuditOnlyEvents != 1 {
		t.Fatalf("audit_only_events = %d, want 1", report.Summary.AuditOnlyEvents)
	}
	if report.Summary.MaterialEvents != 0 {
		t.Fatalf("material_events = %d, want 0", report.Summary.MaterialEvents)
	}
	if got := report.Events[0].ResolutionStatus; got != DriftEventResolutionResolved {
		t.Fatalf("resolution_status = %q, want %s", got, DriftEventResolutionResolved)
	}
	if got := report.Events[0].RootCause; got != DriftEventRootCauseCarrierOnlyChanged {
		t.Fatalf("root_cause = %q, want %s", got, DriftEventRootCauseCarrierOnlyChanged)
	}
	if !report.Events[0].AuditOnly {
		t.Fatal("event should be audit_only")
	}
}

func TestBuildDriftEventReportClassifiesScopeManifestAdjacentChurnAsImplementationFootprint(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "internal/new_route.go",
			Status:      DriftAdded,
			TriggerKind: DriftTriggerScopeManifest,
			Materiality: DriftMaterialityAdjacentFileChurn,
			AuditOnly:   true,
		}},
	}})

	if report.Summary.AuditOnlyEvents != 1 {
		t.Fatalf("audit_only_events = %d, want 1", report.Summary.AuditOnlyEvents)
	}
	if report.Summary.MaterialEvents != 0 {
		t.Fatalf("material_events = %d, want 0", report.Summary.MaterialEvents)
	}
	event := report.Events[0]
	if event.RootCause != DriftEventRootCauseImplementationFootprint {
		t.Fatalf("root_cause = %q, want %s", event.RootCause, DriftEventRootCauseImplementationFootprint)
	}
	if event.ResolutionStatus != DriftEventResolutionResolved {
		t.Fatalf("resolution_status = %q, want %s", event.ResolutionStatus, DriftEventResolutionResolved)
	}
	if event.SuggestedNextCommand != "" {
		t.Fatalf("suggested_next_command = %q, want empty for resolved implementation footprint cue", event.SuggestedNextCommand)
	}
	if event.SourceItems[0].TriggerKind != DriftTriggerScopeManifest {
		t.Fatalf("source trigger = %q, want scope_manifest", event.SourceItems[0].TriggerKind)
	}
}

func TestBuildDriftEventReportClassifiesMaterialScopeManifestAsSchemaChanged(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "internal/schema.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerScopeManifest,
			Materiality: DriftMaterialityMaterialSymbol,
		}},
	}})

	if report.Summary.AuditOnlyEvents != 0 {
		t.Fatalf("audit_only_events = %d, want 0", report.Summary.AuditOnlyEvents)
	}
	if report.Summary.MaterialEvents != 1 {
		t.Fatalf("material_events = %d, want 1", report.Summary.MaterialEvents)
	}
	event := report.Events[0]
	if event.RootCause != DriftEventRootCauseSchemaChanged {
		t.Fatalf("root_cause = %q, want %s", event.RootCause, DriftEventRootCauseSchemaChanged)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsOperatorJudgment {
		t.Fatalf("resolution_status = %q, want %s", event.ResolutionStatus, DriftEventResolutionNeedsOperatorJudgment)
	}
}

func TestBuildDriftEventReportExposesBindingFallbackMetadata(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:           "shared.go",
			Status:         DriftModified,
			TriggerKind:    DriftTriggerFileHash,
			Materiality:    DriftMaterialityNeedsBindingResolution,
			FallbackKind:   BindingTargetWholeFileFallback,
			FallbackReason: "unsupported language",
		}},
	}})

	if report.Summary.NeedsBindingResolutionEvents != 1 {
		t.Fatalf("needs_binding_resolution_events = %d, want 1", report.Summary.NeedsBindingResolutionEvents)
	}
	event := report.Events[0]
	if event.FallbackKind != BindingTargetWholeFileFallback {
		t.Fatalf("fallback_kind = %q, want %q", event.FallbackKind, BindingTargetWholeFileFallback)
	}
	if event.FallbackReason != "unsupported language" {
		t.Fatalf("fallback_reason = %q", event.FallbackReason)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsScopeEnrichment {
		t.Fatalf("resolution_status = %q, want %s", event.ResolutionStatus, DriftEventResolutionNeedsScopeEnrichment)
	}
	if event.SuggestedNextCommand != "haft decision reconcile --json" {
		t.Fatalf("suggested_next_command = %q, want reconcile drill-down", event.SuggestedNextCommand)
	}
	if event.RootCause != DriftEventRootCauseBindingTargetMissing {
		t.Fatalf("root_cause = %q, want %s", event.RootCause, DriftEventRootCauseBindingTargetMissing)
	}
	if report.Summary.FileFallbackEvents != 1 {
		t.Fatalf("file_fallback_events = %d, want 1", report.Summary.FileFallbackEvents)
	}
	if len(event.SourceItems) != 1 || event.SourceItems[0].FallbackKind != BindingTargetWholeFileFallback {
		t.Fatalf("source_items did not preserve fallback metadata: %#v", event.SourceItems)
	}
}

func TestBuildDriftEventReportRoutesLegacyFileFallbackToBindingResolution(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityUnknownLegacyFileScope,
		}},
	}})

	if report.Summary.FileFallbackEvents != 1 {
		t.Fatalf("file_fallback_events = %d, want 1", report.Summary.FileFallbackEvents)
	}
	if report.Summary.NeedsBindingResolutionEvents != 1 {
		t.Fatalf("needs_binding_resolution_events = %d, want 1", report.Summary.NeedsBindingResolutionEvents)
	}
	if report.Summary.UnknownHighRiskEvents != 0 {
		t.Fatalf("unknown_high_risk_events = %d, want 0", report.Summary.UnknownHighRiskEvents)
	}
	if report.Summary.MaterialEvents != 1 {
		t.Fatalf("material_events = %d, want 1", report.Summary.MaterialEvents)
	}
	event := report.Events[0]
	if event.TargetKind != "file" {
		t.Fatalf("target_kind = %q, want file", event.TargetKind)
	}
	if event.RootCause != DriftEventRootCauseBindingTargetMissing {
		t.Fatalf("root_cause = %q, want %s", event.RootCause, DriftEventRootCauseBindingTargetMissing)
	}
	if event.ResolutionStatus != DriftEventResolutionNeedsScopeEnrichment {
		t.Fatalf("resolution_status = %q, want %s", event.ResolutionStatus, DriftEventResolutionNeedsScopeEnrichment)
	}
	if event.FallbackKind != BindingTargetWholeFileFallback {
		t.Fatalf("fallback_kind = %q, want %q", event.FallbackKind, BindingTargetWholeFileFallback)
	}
	if !strings.Contains(event.FallbackReason, "legacy file-scope") {
		t.Fatalf("fallback_reason = %q, want legacy file-scope explanation", event.FallbackReason)
	}
	if event.SuggestedNextCommand != "haft decision reconcile --json" {
		t.Fatalf("suggested_next_command = %q, want reconcile drill-down", event.SuggestedNextCommand)
	}
	if len(event.SourceItems) != 1 || event.SourceItems[0].FallbackKind != BindingTargetWholeFileFallback {
		t.Fatalf("source_items did not preserve fallback metadata: %#v", event.SourceItems)
	}
}

func TestApplyDriftEventResolutionLedgerOverlaysResolvedStatus(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityMaterialSymbol,
		}},
	}})
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	ledger := NewDriftEventResolutionLedger([]DriftEventResolution{{
		EventID: report.Events[0].EventID,
		Status:  DriftEventResolutionResolved,
		Reason:  "verified additive-only in focused review",
	}})

	overlaid := ApplyDriftEventResolutionLedger(report, ledger, now)

	if overlaid.Events[0].ResolutionStatus != DriftEventResolutionResolved {
		t.Fatalf("resolution_status = %q, want resolved", overlaid.Events[0].ResolutionStatus)
	}
	if overlaid.Events[0].SuggestedNextCommand != "" {
		t.Fatalf("suggested_next_command = %q, want empty after ledger resolution", overlaid.Events[0].SuggestedNextCommand)
	}
	if overlaid.Events[0].ResolutionRecord == nil {
		t.Fatal("resolution_record should be attached")
	}
	if overlaid.Events[0].ResolutionRecordPosture != DriftEventResolutionRecordPostureApplied {
		t.Fatalf("resolution_record_posture = %q, want applied", overlaid.Events[0].ResolutionRecordPosture)
	}
	if overlaid.Summary.ResolvedByLedgerEvents != 1 {
		t.Fatalf("resolved_by_ledger_events = %d, want 1", overlaid.Summary.ResolvedByLedgerEvents)
	}
	if overlaid.Summary.ImpactedDecisions != 1 {
		t.Fatalf("impacted_decisions = %d, want 1", overlaid.Summary.ImpactedDecisions)
	}
}

func TestApplyDriftEventResolutionLedgerInvalidatesTargetChangedRecord(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityMaterialSymbol,
		}},
	}})
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	record := BindDriftEventResolutionToEvent(DriftEventResolution{
		EventID: report.Events[0].EventID,
		Status:  DriftEventResolutionResolved,
		Reason:  "verified original target",
	}, report.Events[0])

	report.Events[0].ChangedTargetRef = "symbol:shared.go::func:Different"
	report.Events[0].TargetKind = BindingTargetSymbol
	report.Events[0].TargetStatus = "retarget_candidate"
	report.Events[0].RootCause = DriftEventRootCauseRetargetCandidate
	report.Events[0].ResolutionStatus = DriftEventResolutionNeedsReconcile
	report.Events[0].SuggestedNextCommand = "haft drift route --json"

	overlaid := ApplyDriftEventResolutionLedger(
		report,
		NewDriftEventResolutionLedger([]DriftEventResolution{record}),
		now,
	)

	if overlaid.Events[0].ResolutionStatus == DriftEventResolutionResolved {
		t.Fatalf("target-changed record should not resolve event: %#v", overlaid.Events[0])
	}
	if overlaid.Events[0].SuggestedNextCommand == "" {
		t.Fatal("target-changed record should not clear suggested next command")
	}
	if overlaid.Events[0].ResolutionRecord == nil {
		t.Fatal("target-changed resolution record should remain visible for audit")
	}
	if overlaid.Events[0].ResolutionRecordPosture != DriftEventResolutionRecordPostureStaleEventBinding {
		t.Fatalf("resolution_record_posture = %q, want stale_event_binding", overlaid.Events[0].ResolutionRecordPosture)
	}
	if overlaid.Summary.ResolvedByLedgerEvents != 0 {
		t.Fatalf("resolved_by_ledger_events = %d, want 0", overlaid.Summary.ResolvedByLedgerEvents)
	}
}

func TestApplyDriftEventResolutionLedgerInvalidatesMaterialityChangedRecord(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityAdjacentFileChurn,
			AuditOnly:   true,
		}},
	}})
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	record := BindDriftEventResolutionToEvent(DriftEventResolution{
		EventID: report.Events[0].EventID,
		Status:  DriftEventResolutionResolved,
		Reason:  "verified original audit-only posture",
	}, report.Events[0])

	report.Events[0].AuditOnly = false
	report.Events[0].Materiality = DriftMaterialityMaterialSymbol
	report.Events[0].ResolutionStatus = DriftEventResolutionNeedsOperatorJudgment
	report.Events[0].SuggestedNextCommand = `haft_refresh(action="review")`

	overlaid := ApplyDriftEventResolutionLedger(
		report,
		NewDriftEventResolutionLedger([]DriftEventResolution{record}),
		now,
	)

	if overlaid.Events[0].ResolutionStatus == DriftEventResolutionResolved {
		t.Fatalf("materiality-changed record should not resolve event: %#v", overlaid.Events[0])
	}
	if overlaid.Events[0].SuggestedNextCommand == "" {
		t.Fatal("materiality-changed record should not clear suggested next command")
	}
	if overlaid.Events[0].ResolutionRecord == nil {
		t.Fatal("materiality-changed resolution record should remain visible for audit")
	}
	if overlaid.Events[0].ResolutionRecordPosture != DriftEventResolutionRecordPostureStaleEventBinding {
		t.Fatalf("resolution_record_posture = %q, want stale_event_binding", overlaid.Events[0].ResolutionRecordPosture)
	}
	if overlaid.Summary.ResolvedByLedgerEvents != 0 {
		t.Fatalf("resolved_by_ledger_events = %d, want 0", overlaid.Summary.ResolvedByLedgerEvents)
	}
}

func TestApplyDriftEventResolutionLedgerRespectsWaiverExpiry(t *testing.T) {
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityMaterialSymbol,
		}},
	}})
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	ledger := NewDriftEventResolutionLedger([]DriftEventResolution{{
		EventID:         report.Events[0].EventID,
		Status:          DriftEventResolutionWaivedUntil,
		Reason:          "temporary migration waiver",
		WaiverExpiresAt: "2026-06-23",
	}})

	active := ApplyDriftEventResolutionLedger(report, ledger, now)
	if active.Events[0].ResolutionStatus != DriftEventResolutionWaivedUntil {
		t.Fatalf("active waiver status = %q, want waived_until", active.Events[0].ResolutionStatus)
	}
	if active.Summary.WaivedByLedgerEvents != 1 {
		t.Fatalf("waived_by_ledger_events = %d, want 1", active.Summary.WaivedByLedgerEvents)
	}

	expired := ApplyDriftEventResolutionLedger(report, ledger, now.Add(48*time.Hour))
	if expired.Events[0].ResolutionStatus == DriftEventResolutionWaivedUntil {
		t.Fatalf("expired waiver should not override resolution: %#v", expired.Events[0])
	}
	if expired.Events[0].ResolutionRecord == nil {
		t.Fatal("expired resolution record should remain visible for audit")
	}
	if expired.Events[0].ResolutionRecordPosture != DriftEventResolutionRecordPostureInactiveWaiver {
		t.Fatalf("resolution_record_posture = %q, want inactive_waiver", expired.Events[0].ResolutionRecordPosture)
	}
}

func TestAttachDriftClaimEvidenceRefsUsesExactGovernanceTargets(t *testing.T) {
	item := DriftItem{
		Path:             "shared.go",
		Status:           DriftModified,
		ChangedTargetRef: driftEventFileTarget("shared.go"),
		Symbols: []SymbolDriftItem{{
			SymbolName: "Run",
			SymbolKind: "func",
			Status:     "modified",
		}},
	}
	claims := []DecisionClaim{
		{
			ID:                   "claim-file",
			GovernanceTargetRefs: []string{driftEventFileTarget("shared.go")},
		},
		{
			ID:                   "claim-symbol",
			GovernanceTargetRefs: []string{driftEventSymbolTarget("shared.go", item.Symbols[0])},
		},
		{
			ID:                   "claim-old",
			LifecycleStatus:      ClaimLifecycleSuperseded,
			GovernanceTargetRefs: []string{driftEventSymbolTarget("shared.go", item.Symbols[0])},
		},
	}
	evidenceItems := []EvidenceItem{
		{ID: "evid-file", ClaimRefs: []string{"claim-file"}},
		{ID: "evid-symbol", ClaimRefs: []string{"claim-symbol"}},
		{ID: "evid-old", ClaimRefs: []string{"claim-old"}},
	}

	enriched := attachDriftClaimEvidenceRefs(item, claims, evidenceItems)

	if strings.Join(enriched.ClaimRefs, ",") != "claim-file" {
		t.Fatalf("file claim_refs = %#v", enriched.ClaimRefs)
	}
	if strings.Join(enriched.EvidenceRefs, ",") != "evid-file" {
		t.Fatalf("file evidence_refs = %#v", enriched.EvidenceRefs)
	}
	if strings.Join(enriched.Symbols[0].ClaimRefs, ",") != "claim-symbol" {
		t.Fatalf("symbol claim_refs = %#v", enriched.Symbols[0].ClaimRefs)
	}
	if strings.Join(enriched.Symbols[0].EvidenceRefs, ",") != "evid-symbol" {
		t.Fatalf("symbol evidence_refs = %#v", enriched.Symbols[0].EvidenceRefs)
	}
}

func TestBuildDriftEventReportCarriesSourceClaimEvidenceRefs(t *testing.T) {
	symbol := SymbolDriftItem{
		SymbolName:   "Run",
		SymbolKind:   "func",
		Status:       "modified",
		ClaimRefs:    []string{"claim-symbol"},
		EvidenceRefs: []string{"evid-symbol"},
	}
	report := BuildDriftEventReport([]DriftReport{{
		DecisionID: "dec-1",
		Files: []DriftItem{{
			Path:        "shared.go",
			Status:      DriftModified,
			TriggerKind: DriftTriggerFileHash,
			Materiality: DriftMaterialityMaterialSymbol,
			Symbols:     []SymbolDriftItem{symbol},
		}},
	}})

	if len(report.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(report.Events))
	}
	source := report.Events[0].SourceItems[0]
	if strings.Join(source.ClaimRefs, ",") != "claim-symbol" {
		t.Fatalf("source claim_refs = %#v", source.ClaimRefs)
	}
	if strings.Join(source.EvidenceRefs, ",") != "evid-symbol" {
		t.Fatalf("source evidence_refs = %#v", source.EvidenceRefs)
	}
}

func TestUpsertDriftEventResolutionValidatesNonBindingMetadata(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	_, err := UpsertDriftEventResolution(NewDriftEventResolutionLedger(nil), DriftEventResolution{
		EventID: "drift-event-1",
		Status:  DriftEventResolutionWaivedUntil,
		Reason:  "missing expiry",
	}, now)
	if err == nil {
		t.Fatal("expected missing waiver expiry to fail")
	}

	ledger, err := UpsertDriftEventResolution(NewDriftEventResolutionLedger(nil), DriftEventResolution{
		EventID:      "drift-event-1",
		Status:       DriftEventResolutionResolved,
		Reason:       "verified additive-only",
		EvidenceRefs: []string{"go test ./..."},
	}, now)
	if err != nil {
		t.Fatalf("UpsertDriftEventResolution: %v", err)
	}
	if ledger.Authority != DriftEventResolutionLedgerAuthority {
		t.Fatalf("authority = %q", ledger.Authority)
	}
	if len(ledger.Records) != 1 {
		t.Fatalf("records = %#v", ledger.Records)
	}
}
