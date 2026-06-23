package artifact

import "testing"

func TestBuildSemanticDriftRouteKeepsEvidenceDriftOutOfCodeRepair(t *testing.T) {
	route := BuildSemanticDriftRoute(DriftRouteInput{
		DriftKind:  "evidence_binding_drift",
		BearerRef:  "evid-1",
		UseContext: "release reliance",
	})

	if !route.Recognized {
		t.Fatalf("route should be recognized")
	}
	if route.DriftLayer != "evidence" {
		t.Fatalf("layer = %q, want evidence", route.DriftLayer)
	}
	if driftRouteHasAction(route, "repair_code") {
		t.Fatalf("evidence drift must not route directly to repair_code: %#v", route.CandidateRepairActions)
	}
	if !driftRouteHasAction(route, "refresh_evidence") {
		t.Fatalf("route actions = %#v, want refresh_evidence", route.CandidateRepairActions)
	}
	if route.AuthorityBoundary.Mutation != DriftRouteBoundaryNotMutation {
		t.Fatalf("authority boundary = %+v", route.AuthorityBoundary)
	}
}

func TestBuildSemanticDriftRouteAllowsCodeRepairForRealizationDriftOnlyAsCandidate(t *testing.T) {
	route := BuildSemanticDriftRoute(DriftRouteInput{
		DriftKind: "transformation_realization_drift",
	})

	if route.DriftLayer != "realization" {
		t.Fatalf("layer = %q, want realization", route.DriftLayer)
	}
	if !driftRouteHasAction(route, "repair_code") {
		t.Fatalf("route actions = %#v, want repair_code candidate", route.CandidateRepairActions)
	}
	if route.AuthorityBoundary.Mutation != DriftRouteBoundaryNotMutation {
		t.Fatalf("route must stay read-only: %+v", route.AuthorityBoundary)
	}
}

func TestBuildSemanticDriftRouteRecognizesCarrierOnlyWithoutSemanticRepair(t *testing.T) {
	route := BuildSemanticDriftRoute(DriftRouteInput{
		DriftKind:  "carrier_only",
		BearerRef:  "publication-unit:spec-section:target",
		UseContext: "stronger spec reliance",
	})

	if !route.Recognized {
		t.Fatalf("carrier-only route should be recognized")
	}
	if route.DriftLayer != "carrier" {
		t.Fatalf("layer = %q, want carrier", route.DriftLayer)
	}
	if route.EntityOfConcernChangeMode != "preserve" {
		t.Fatalf("change mode = %q, want preserve", route.EntityOfConcernChangeMode)
	}
	if !driftRouteHasAction(route, "no_change") {
		t.Fatalf("route actions = %#v, want no_change", route.CandidateRepairActions)
	}
	if driftRouteHasAction(route, "repair_episteme_claim") {
		t.Fatalf("carrier-only drift must not route to semantic claim repair: %#v", route.CandidateRepairActions)
	}
	if driftRouteHasBlockedUse(route, "stronger_use_until_drift_kind_is_classified") {
		t.Fatalf("recognized carrier-only drift should not use unknown-kind blocker: %#v", route.BlockedUses)
	}
	if !driftRouteHasBlockedUse(route, "stronger spec reliance") {
		t.Fatalf("use context should stay visible in blocked/review uses: %#v", route.BlockedUses)
	}
}

func TestBuildSemanticDriftRouteUnknownKindFailsClosed(t *testing.T) {
	route := BuildSemanticDriftRoute(DriftRouteInput{
		DriftKind:  "mystery_drift",
		UseContext: "commission preflight",
	})

	if route.Recognized {
		t.Fatalf("route should not be recognized")
	}
	if route.DriftLayer != DriftRouteUnknownKind {
		t.Fatalf("layer = %q", route.DriftLayer)
	}
	if !driftRouteHasBlockedUse(route, "stronger_use_until_drift_kind_is_classified") {
		t.Fatalf("blocked uses = %#v", route.BlockedUses)
	}
	if !driftRouteHasBlockedUse(route, "commission preflight") {
		t.Fatalf("blocked uses = %#v", route.BlockedUses)
	}
}

func driftRouteHasAction(route SemanticDriftRoute, action string) bool {
	for _, candidate := range route.CandidateRepairActions {
		if candidate == action {
			return true
		}
	}

	return false
}

func driftRouteHasBlockedUse(route SemanticDriftRoute, blockedUse string) bool {
	for _, candidate := range route.BlockedUses {
		if candidate == blockedUse {
			return true
		}
	}

	return false
}
