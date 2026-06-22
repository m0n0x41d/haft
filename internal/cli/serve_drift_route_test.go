package cli

import (
	"context"
	"encoding/json"
	"testing"

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
	if report.Summary.UniqueEvents == 0 {
		t.Fatalf("expected drift events, got %#v", report.Summary)
	}
	if report.Summary.ImpactedDecisions == 0 {
		t.Fatalf("expected impacted decisions, got %#v", report.Summary)
	}
	if !serveDriftEventsMentionDecision(report, seed.driftID) {
		t.Fatalf("drift events do not mention seeded drift decision %s: %#v", seed.driftID, report.Events)
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
