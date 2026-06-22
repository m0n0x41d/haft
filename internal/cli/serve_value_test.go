package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintQuery_ValueSpaceReturnsCharacteristicSpace(t *testing.T) {
	store := setupCLIArtifactStore(t)
	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":      "value_space",
		"bearer_ref":  "release-1",
		"context":     "2026-Q3",
		"method_ref":  "method-1",
		"source_refs": []any{"evid-1"},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery value_space returned error: %v", err)
	}

	var space artifact.EngineeringValueSpace
	if err := json.Unmarshal([]byte(result), &space); err != nil {
		t.Fatalf("decode engineering value space: %v\n%s", err, result)
	}

	if space.ScorePolicy.SingleScore != artifact.EngineeringValueNoSingleScore {
		t.Fatalf("single score = %q", space.ScorePolicy.SingleScore)
	}
	if len(space.Characteristics) == 0 {
		t.Fatalf("expected characteristics")
	}
	if space.Characteristics[0].BearerRef != "release-1" {
		t.Fatalf("bearer = %q", space.Characteristics[0].BearerRef)
	}
	if len(space.Characteristics[0].EvidenceRefs) != 1 {
		t.Fatalf("evidence refs = %#v", space.Characteristics[0].EvidenceRefs)
	}
	if len(space.SimplifyKillCriteria) == 0 {
		t.Fatalf("expected simplify/kill criteria")
	}
	if space.SimplifyKillCriteria[0].AuthorityBoundary != artifact.EngineeringValueSimplifyKillAuthority {
		t.Fatalf("simplify/kill authority = %+v", space.SimplifyKillCriteria[0])
	}
	if space.AuthorityBoundary.Score != artifact.EngineeringValueBoundaryNotScore {
		t.Fatalf("authority boundary = %+v", space.AuthorityBoundary)
	}
}
