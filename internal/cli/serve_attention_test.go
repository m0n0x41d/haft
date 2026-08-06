package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_BlockedUseAttentionReturnsExactSourceItem(t *testing.T) {
	t.Parallel()

	store := setupCLIArtifactStore(t)
	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":              "blocked_use",
		"bearer_ref":          "dec-1",
		"label":               "Spec semantic review decision",
		"finding_kind":        "evidence_freshness_drift",
		"blocked_use":         "release reliance",
		"source_refs":         []any{"dec-1", "evid-1"},
		"exact_record_needed": "current EvidencePath",
		"next_actions":        []any{"recover_exact_source_record", "refresh_evidence"},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery blocked_use returned error: %v", err)
	}

	var item artifact.BlockedUseAttentionItem
	if err := json.Unmarshal([]byte(result), &item); err != nil {
		t.Fatalf("decode blocked-use attention item: %v\n%s", err, result)
	}

	if item.Object.BearerRef != "dec-1" {
		t.Fatalf("bearer = %q", item.Object.BearerRef)
	}
	if item.SourceReturn.Status != artifact.BlockedUseExactRecordNeeded {
		t.Fatalf("source return = %+v", item.SourceReturn)
	}
	if item.AuthorityBoundary.WorkPlan != artifact.BlockedUseBoundaryNotWorkPlan {
		t.Fatalf("authority boundary = %+v", item.AuthorityBoundary)
	}
	if item.AuthorityBoundary.ClaimTruth != artifact.BlockedUseBoundaryNotClaimTruth {
		t.Fatalf("claim truth boundary = %+v", item.AuthorityBoundary)
	}
	if item.AuthorityBoundary.Publication != artifact.BlockedUseBoundaryNotPublication {
		t.Fatalf("publication boundary = %+v", item.AuthorityBoundary)
	}
}
