package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestWriteEngineeringValueSpaceSummaryShowsMissingEvidence(t *testing.T) {
	space := artifact.BuildEngineeringValueSpace(artifact.EngineeringValueSpaceInput{
		BearerRef: "release-1",
	})

	var out bytes.Buffer
	if err := writeEngineeringValueSpaceSummary(&out, space); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "measurement_context: window=declared_window_required_before_value_claim method_ref=measurement_method_required_before_value_claim evidence_refs=0") {
		t.Fatalf("summary missing measurement context:\n%s", got)
	}
	if !strings.Contains(got, "evidence_missing_characteristics=11") {
		t.Fatalf("summary missing evidence-missing count:\n%s", got)
	}
	if strings.Contains(got, "total_score") {
		t.Fatalf("summary must not expose a total score:\n%s", got)
	}
}

func TestWriteEngineeringValueSpaceSummaryShowsEvidenceContext(t *testing.T) {
	space := artifact.BuildEngineeringValueSpace(artifact.EngineeringValueSpaceInput{
		BearerRef:    "release-1",
		Window:       "2026-Q3",
		MethodRef:    "method-1",
		EvidenceRefs: []string{"evid-1", "evid-2", "evid-1"},
	})

	var out bytes.Buffer
	if err := writeEngineeringValueSpaceSummary(&out, space); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "measurement_context: window=2026-Q3 method_ref=method-1 evidence_refs=2") {
		t.Fatalf("summary missing evidence context:\n%s", got)
	}
	if !strings.Contains(got, "evidence_missing_characteristics=0") {
		t.Fatalf("summary should show zero missing-evidence characteristics:\n%s", got)
	}
}
