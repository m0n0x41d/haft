package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecUseJSONReturnsSpecificationUseRecord(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()

	restoreFlags := stubSpecUseFlags(t, true, "agent planning read", specflow.SpecUsePolicyDocumentaryOnly, "")
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecUse(cmd, []string{"TS.environment.001"})
	if err != nil {
		t.Fatalf("runSpecUse returned error: %v", err)
	}

	var record specflow.SpecificationUseRecord
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode spec use JSON: %v\n%s", err, output.String())
	}
	if record.RecordKind != specflow.SpecificationUseRecordKind {
		t.Fatalf("record_kind = %q, want %q", record.RecordKind, specflow.SpecificationUseRecordKind)
	}
	if record.Admission.Disposition != specflow.SpecUseDispositionAdmitted {
		t.Fatalf("admission = %+v, want admitted", record.Admission)
	}
	if record.BaselineCurrentness.Status != specflow.SpecUseBaselineUnknown {
		t.Fatalf("baseline_currentness = %+v, want unknown without DB", record.BaselineCurrentness)
	}
	if record.GateDecision.Status != specflow.SpecUseGateDecisionNotApplicable {
		t.Fatalf("gate_decision = %+v, want no OperationalGate", record.GateDecision)
	}
	if strings.Contains(output.String(), `"status":"ready"`) || strings.Contains(output.String(), `"verdict":"pass"`) {
		t.Fatalf("spec use JSON must not expose ready/pass authority: %s", output.String())
	}
}

func TestRunSpecUseSummaryNamesSeparatedFields(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()

	restoreFlags := stubSpecUseFlags(t, false, "commission preflight", specflow.SpecUsePolicyStrongerUseRequiresCurrentSource, "")
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecUse(cmd, []string{"TS.environment.001"})
	if err != nil {
		t.Fatalf("runSpecUse returned error: %v", err)
	}

	result := output.String()
	for _, want := range []string{"admission:", "source_edition:", "baseline_currentness:", "gate_decision: not_applicable_no_operational_gate"} {
		if !strings.Contains(result, want) {
			t.Fatalf("summary missing %q:\n%s", want, result)
		}
	}
}

func TestHandleQuintQuerySpecUseReturnsCurrentnessRecord(t *testing.T) {
	fixture := newCheckTestProject(t)
	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action":      "spec_use",
		"section_id":  "TS.environment.001",
		"use_context": "commission preflight",
		"policy":      specflow.SpecUsePolicyStrongerUseRequiresCurrentSource,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_use returned error: %v", err)
	}

	var record specflow.SpecificationUseRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode spec_use packet: %v\n%s", err, result)
	}
	if record.BaselineCurrentness.Status != specflow.SpecUseBaselineCurrent {
		t.Fatalf("baseline_currentness = %+v, want current", record.BaselineCurrentness)
	}
	if record.Admission.Disposition != specflow.SpecUseDispositionAdmitted {
		t.Fatalf("admission = %+v, want admitted with current source", record.Admission)
	}
	if record.Admission.StrongerUse != specflow.SpecUseReadingStrongerUseAdmitted {
		t.Fatalf("stronger_use = %q, want admitted stronger-use reading", record.Admission.StrongerUse)
	}
	if record.GateDecision.Status != specflow.SpecUseGateDecisionNotApplicable {
		t.Fatalf("gate_decision = %+v, want no OperationalGate", record.GateDecision)
	}
}

func stubSpecUseFlags(
	t *testing.T,
	jsonOutput bool,
	useContext string,
	policy string,
	waiverExpiresAt string,
) func() {
	t.Helper()

	previousJSON := specUseJSON
	previousContext := specUseContext
	previousPolicy := specUsePolicy
	previousWaiver := specUseWaiverExpiresAt

	specUseJSON = jsonOutput
	specUseContext = useContext
	specUsePolicy = policy
	specUseWaiverExpiresAt = waiverExpiresAt

	return func() {
		specUseJSON = previousJSON
		specUseContext = previousContext
		specUsePolicy = previousPolicy
		specUseWaiverExpiresAt = previousWaiver
	}
}
