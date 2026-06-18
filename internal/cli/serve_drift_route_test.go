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

func serveDriftRouteHasAction(route artifact.SemanticDriftRoute, action string) bool {
	for _, candidate := range route.CandidateRepairActions {
		if candidate == action {
			return true
		}
	}

	return false
}
