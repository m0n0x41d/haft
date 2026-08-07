package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestWriteDriftRouteSummaryNamesCompleteAuthorityBoundary(t *testing.T) {
	t.Parallel()

	route := artifact.BuildSemanticDriftRoute(artifact.DriftRouteInput{
		DriftKind:  "evidence_binding_drift",
		BearerRef:  "evid-1",
		UseContext: "release reliance",
	})

	var out bytes.Buffer
	if err := writeDriftRouteSummary(&out, route); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "authority_boundary: mutation=not_mutation evidence=not_evidence approval=not_approval gate_decision=not_gate_decision claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary did not name complete authority boundary:\n%s", got)
	}
}
