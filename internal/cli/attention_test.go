package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestWriteBlockedUseAttentionSummaryNamesCompleteAuthorityBoundary(t *testing.T) {
	item := artifact.BuildBlockedUseAttentionItem(artifact.BlockedUseAttentionInput{
		BearerRef:         "dec-1",
		FindingKind:       "missing_exact_source",
		BlockedUse:        "release reliance",
		SourceRefs:        []string{"evid-1"},
		ExactRecordNeeded: "current EvidencePath",
	})

	var out bytes.Buffer
	if err := writeBlockedUseAttentionSummary(&out, item); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "authority_boundary: work_plan=not_work_plan evidence=not_evidence approval=not_approval gate_decision=not_gate_decision claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary did not name complete authority boundary:\n%s", got)
	}
}
