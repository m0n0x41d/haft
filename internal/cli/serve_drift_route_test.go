package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_DriftRouteDoesNotRepairCodeForEvidenceDrift(t *testing.T) {
	store := setupCLIArtifactStore(t)
	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":      "drift_route",
		"drift_kind":  "evidence_binding_drift",
		"bearer_ref":  "evid-1",
		"use_context": "release reliance",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery drift_route returned error: %v", err)
	}

	var route artifact.SemanticDriftRoute
	if err := json.Unmarshal([]byte(result), &route); err != nil {
		t.Fatalf("decode drift route: %v\n%s", err, result)
	}

	if route.DriftLayer != "evidence" {
		t.Fatalf("layer = %q", route.DriftLayer)
	}
	if serveDriftRouteHasAction(route, "repair_code") {
		t.Fatalf("evidence drift must not route to repair_code: %#v", route.CandidateRepairActions)
	}
	if route.AuthorityBoundary.Mutation != artifact.DriftRouteBoundaryNotMutation {
		t.Fatalf("authority boundary = %+v", route.AuthorityBoundary)
	}
}

func TestHandleQuintQuery_DriftRouteUnknownKindFailsClosed(t *testing.T) {
	store := setupCLIArtifactStore(t)
	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":     "drift_route",
		"drift_kind": "mystery_drift",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery drift_route returned error: %v", err)
	}

	var route artifact.SemanticDriftRoute
	if err := json.Unmarshal([]byte(result), &route); err != nil {
		t.Fatalf("decode drift route: %v\n%s", err, result)
	}

	if route.Recognized {
		t.Fatalf("unknown route should not be recognized")
	}
	if route.DriftLayer != artifact.DriftRouteUnknownKind {
		t.Fatalf("layer = %q", route.DriftLayer)
	}
}

func TestHandleQuintQuery_DriftEventsReturnsFanoutProjection(t *testing.T) {
	fixture := newCheckTestProject(t)
	seed := seedGovernanceDebt(t, fixture)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "drift_events",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery drift_events returned error: %v", err)
	}

	var report artifact.DriftEventReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode drift event report: %v\n%s", err, result)
	}
	if report.SchemaVersion != 2 {
		t.Fatalf("schema_version = %d, want 2", report.SchemaVersion)
	}
	if report.View != "compact" {
		t.Fatalf("default drift_events view = %q, want compact", report.View)
	}
	if report.Summary.UniqueEvents == 0 {
		t.Fatalf("expected drift events, got %#v", report.Summary)
	}
	if len(report.Compatibility) != 0 {
		t.Fatalf("default drift_events should omit compatibility audit reports: %#v", report.Compatibility)
	}
	if report.FullAuditCommand == "" {
		t.Fatal("default drift_events should name full audit command")
	}
	if report.Summary.ImpactedDecisions == 0 {
		t.Fatalf("expected impacted decisions, got %#v", report.Summary)
	}
	if !serveDriftEventsMentionDecision(report, seed.driftID) {
		t.Fatalf("drift events do not mention seeded drift decision %s: %#v", seed.driftID, report.Events)
	}

	fullResult, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "drift_events",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery drift_events full returned error: %v", err)
	}

	var fullReport artifact.DriftEventReport
	if err := json.Unmarshal([]byte(fullResult), &fullReport); err != nil {
		t.Fatalf("decode full drift event report: %v\n%s", err, fullResult)
	}
	if fullReport.View != "" {
		t.Fatalf("full drift_events view = %q, want empty audit view", fullReport.View)
	}
	if len(fullReport.Compatibility) == 0 {
		t.Fatalf("full drift_events should preserve compatibility audit reports")
	}
	if !serveDriftEventsHasSourceItems(fullReport) {
		t.Fatalf("full drift_events should preserve source_items audit detail: %#v", fullReport.Events)
	}
}

func TestHandleQuintQuery_DriftEventsAppliesDefaultResolutionLedger(t *testing.T) {
	fixture := newCheckTestProject(t)
	seed := seedGovernanceDebt(t, fixture)

	initialResult, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "drift_events",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery drift_events initial returned error: %v", err)
	}

	var initialReport artifact.DriftEventReport
	if err := json.Unmarshal([]byte(initialResult), &initialReport); err != nil {
		t.Fatalf("decode initial drift event report: %v\n%s", err, initialResult)
	}
	event, ok := serveDriftEventsEventMentioningDecision(initialReport, seed.driftID)
	if !ok {
		t.Fatalf("initial drift events do not mention seeded drift decision %s: %#v", seed.driftID, initialReport.Events)
	}

	now := timeNow()
	record := artifact.BindDriftEventResolutionToEvent(artifact.DriftEventResolution{
		EventID:      event.EventID,
		Status:       artifact.DriftEventResolutionResolved,
		Reason:       "verified additive-only in focused regression fixture",
		EvidenceRefs: []string{"test:evidence"},
		RecordedAt:   now.Format(time.RFC3339),
		RecordedBy:   "serve_drift_route_test",
	}, event)
	ledger, err := artifact.UpsertDriftEventResolution(
		artifact.NewDriftEventResolutionLedger(nil),
		record,
		now,
	)
	if err != nil {
		t.Fatalf("upsert drift event resolution: %v", err)
	}
	if err := writeDriftEventResolutionLedger(driftEventResolutionLedgerPath(filepath.Dir(fixture.haftDir), ""), ledger); err != nil {
		t.Fatalf("write drift event resolution ledger: %v", err)
	}

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "drift_events",
		"full":   true,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery drift_events returned error: %v", err)
	}

	var report artifact.DriftEventReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode drift event report: %v\n%s", err, result)
	}
	if report.Summary.ResolvedByLedgerEvents != 1 {
		t.Fatalf("resolved_by_ledger_events = %d, want 1", report.Summary.ResolvedByLedgerEvents)
	}
	resolvedEvent, ok := serveDriftEventsEventMentioningDecision(report, seed.driftID)
	if !ok {
		t.Fatalf("resolved drift events do not mention seeded drift decision %s: %#v", seed.driftID, report.Events)
	}
	if resolvedEvent.ResolutionStatus != artifact.DriftEventResolutionResolved {
		t.Fatalf("resolution_status = %q, want resolved", resolvedEvent.ResolutionStatus)
	}
	if resolvedEvent.ResolutionRecord == nil {
		t.Fatal("resolution_record missing")
	}
}

func TestApplyDefaultDriftEventResolutionLedgerToStatusData(t *testing.T) {
	fixture := newCheckTestProject(t)
	seed := seedGovernanceDebt(t, fixture)
	projectRoot := filepath.Dir(fixture.haftDir)
	reports, err := artifact.CheckDrift(context.Background(), fixture.store, projectRoot)
	if err != nil {
		t.Fatalf("check drift: %v", err)
	}
	report := artifact.BuildDriftEventReport(reports)
	event, ok := serveDriftEventsEventMentioningDecision(report, seed.driftID)
	if !ok {
		t.Fatalf("drift events do not mention seeded drift decision %s: %#v", seed.driftID, report.Events)
	}

	now := timeNow()
	record := artifact.BindDriftEventResolutionToEvent(artifact.DriftEventResolution{
		EventID:    event.EventID,
		Status:     artifact.DriftEventResolutionResolved,
		Reason:     "verified in status helper regression fixture",
		RecordedAt: now.Format(time.RFC3339),
		RecordedBy: "serve_drift_route_test",
	}, event)
	ledger, err := artifact.UpsertDriftEventResolution(
		artifact.NewDriftEventResolutionLedger(nil),
		record,
		now,
	)
	if err != nil {
		t.Fatalf("upsert drift event resolution: %v", err)
	}
	if err := writeDriftEventResolutionLedger(driftEventResolutionLedgerPath(projectRoot, ""), ledger); err != nil {
		t.Fatalf("write drift event resolution ledger: %v", err)
	}

	data := applyDefaultDriftEventResolutionLedgerToStatusData(
		context.Background(),
		fixture.store,
		projectRoot,
		artifact.StatusData{DriftEvents: report},
	)
	resolvedEvent, ok := serveDriftEventsEventMentioningDecision(data.DriftEvents, seed.driftID)
	if !ok {
		t.Fatalf("status drift events do not mention seeded drift decision %s: %#v", seed.driftID, data.DriftEvents.Events)
	}
	if resolvedEvent.ResolutionStatus != artifact.DriftEventResolutionResolved {
		t.Fatalf("resolution_status = %q, want resolved", resolvedEvent.ResolutionStatus)
	}
	if data.DriftEvents.Summary.ResolvedByLedgerEvents != 1 {
		t.Fatalf("resolved_by_ledger_events = %d, want 1", data.DriftEvents.Summary.ResolvedByLedgerEvents)
	}
}

func serveDriftRouteHasAction(route artifact.SemanticDriftRoute, action string) bool {
	for _, candidate := range route.CandidateRepairActions {
		if candidate == action {
			return true
		}
	}

	return false
}

func serveDriftEventsMentionDecision(report artifact.DriftEventReport, decisionID string) bool {
	for _, event := range report.Events {
		for _, decision := range event.ImpactedDecisions {
			if decision.DecisionID == decisionID {
				return true
			}
		}
	}
	return false
}

func serveDriftEventsEventMentioningDecision(
	report artifact.DriftEventReport,
	decisionID string,
) (artifact.DriftEvent, bool) {
	for _, event := range report.Events {
		for _, decision := range event.ImpactedDecisions {
			if decision.DecisionID == decisionID {
				return event, true
			}
		}
	}
	return artifact.DriftEvent{}, false
}

func serveDriftEventsHasSourceItems(report artifact.DriftEventReport) bool {
	for _, event := range report.Events {
		if len(event.SourceItems) > 0 {
			return true
		}
	}
	return false
}
