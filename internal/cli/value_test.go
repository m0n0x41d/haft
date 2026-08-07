package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestEngineeringValueSpaceHelpNamesAuthorityBoundaries(t *testing.T) {
	t.Parallel()

	normalized := strings.ToLower(strings.Join(strings.Fields(valueSpaceCmd.Long), " "))
	for _, want := range []string{"no total", "not evidence", "approval", "gate", "claim truth", "global truth", "publication", "product-value proof"} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("value space help missing %q:\n%s", want, valueSpaceCmd.Long)
		}
	}
}

func TestWriteEngineeringValueSpaceSummaryShowsMissingEvidence(t *testing.T) {
	t.Parallel()

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
	for _, want := range []string{
		"review_triggers:",
		"scope_violation_not_blocked_or_surfaced action=stop_or_retire_capability",
		"ceremony_exceeds_value_movement action=simplify_or_remove_capability",
		"missing_equal_budget_comparison action=review_before_continuing_investment",
		"single_proxy_value_claim action=stop_or_retire_capability",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing review trigger %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{
		"automatic_gate_passed",
		"approval_granted",
		"gate_passed",
		"global_truth=true",
		"global_truth=global_truth",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("summary must keep review triggers non-authoritative, found %q:\n%s", forbidden, got)
		}
	}
}

func TestWriteEngineeringValueSpaceSummaryShowsEvidenceContext(t *testing.T) {
	t.Parallel()

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
	if !strings.Contains(got, "authority_boundary: score=not_score evidence=not_evidence approval=not_approval gate_decision=not_gate_decision claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary should name complete authority boundary:\n%s", got)
	}
}
