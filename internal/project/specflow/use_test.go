package specflow

import (
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestBuildSpecificationUseRecordSeparatesAdmissionFromBaselineCurrentness(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		SpecificationUseBaselineInput{Status: SpecUseBaselineMissing},
		SpecificationUseInput{
			SectionID:  section.ID,
			UseContext: "agent planning read",
			Policy:     SpecUsePolicyDocumentaryOnly,
			Now:        specUseNow(),
		},
	)

	if record.Admission.Disposition != SpecUseDispositionAdmitted {
		t.Fatalf("admission = %+v, want admitted", record.Admission)
	}
	if record.BaselineCurrentness.Status != SpecUseBaselineMissing {
		t.Fatalf("baseline_currentness = %+v, want missing", record.BaselineCurrentness)
	}
	if record.GateDecision.Status != SpecUseGateDecisionNotApplicable {
		t.Fatalf("gate_decision = %+v, want no OperationalGate", record.GateDecision)
	}
}

func TestBuildSpecificationUseRecordBlocksStrongerUseWhenSourceNotCurrent(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		SpecificationUseBaselineInput{Status: SpecUseBaselineMissing},
		SpecificationUseInput{
			SectionID:  section.ID,
			UseContext: "commission preflight",
			Policy:     SpecUsePolicyStrongerUseRequiresCurrentSource,
			Now:        specUseNow(),
		},
	)

	if record.Admission.Disposition != SpecUseDispositionBlocked {
		t.Fatalf("admission = %+v, want blocked", record.Admission)
	}
	if record.Admission.Reason != "source_edition_not_current" {
		t.Fatalf("reason = %q, want source_edition_not_current", record.Admission.Reason)
	}
	if record.Admission.StrongerUse != ReviewUseBlockedForStrongerUse {
		t.Fatalf("stronger_use = %q, want blocked", record.Admission.StrongerUse)
	}
}

func TestBuildSpecificationUseRecordTemporaryWaiverDoesNotCreateGateDecision(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		SpecificationUseBaselineInput{Status: SpecUseBaselineDrifted},
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "bounded manual waiver",
			Policy:          SpecUsePolicyTemporaryWaiver,
			WaiverExpiresAt: "2099-01-01T00:00:00Z",
			Now:             specUseNow(),
		},
	)

	if record.Admission.Disposition != SpecUseDispositionWaived {
		t.Fatalf("admission = %+v, want waived", record.Admission)
	}
	if record.Admission.StrongerUse != SpecUseReadingTemporaryWaiver {
		t.Fatalf("stronger_use = %q, want temporary waiver reading", record.Admission.StrongerUse)
	}
	if record.GateDecision.Status != SpecUseGateDecisionNotApplicable {
		t.Fatalf("gate_decision = %+v, want no OperationalGate", record.GateDecision)
	}
}

func TestBuildSpecificationUseRecordOperationalGatePassesCurrentAdmittedUse(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		specUseCurrentBaseline(section),
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          SpecUsePolicyStrongerUseRequiresCurrentSource,
			OperationalGate: specUseGate(section.ID, "commission preflight"),
			Now:             specUseNow(),
		},
	)

	if record.GateDecision.Status != SpecUseGateDecisionPassed {
		t.Fatalf("gate_decision = %+v, want passed", record.GateDecision)
	}
	if record.GateDecision.Reason != "operational_gate_passed_for_declared_use" {
		t.Fatalf("reason = %q", record.GateDecision.Reason)
	}
	if record.GateDecision.AuthorityBoundary.Approval != "not_spec_approval" {
		t.Fatalf("authority boundary = %+v", record.GateDecision.AuthorityBoundary)
	}
	if record.GateDecision.OperationalGate == nil {
		t.Fatal("operational gate profile missing from gate decision")
	}
}

func TestBuildSpecificationUseRecordOperationalGateBlocksTemporaryWaiver(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		specUseCurrentBaseline(section),
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          SpecUsePolicyTemporaryWaiver,
			WaiverExpiresAt: "2099-01-01T00:00:00Z",
			OperationalGate: specUseGate(section.ID, "commission preflight"),
			Now:             specUseNow(),
		},
	)

	if record.Admission.Disposition != SpecUseDispositionWaived {
		t.Fatalf("admission = %+v, want waived", record.Admission)
	}
	if record.GateDecision.Status != SpecUseGateDecisionBlocked {
		t.Fatalf("gate_decision = %+v, want blocked", record.GateDecision)
	}
	if record.GateDecision.Reason != "admission_not_granted" {
		t.Fatalf("reason = %q, want admission_not_granted", record.GateDecision.Reason)
	}
}

func TestBuildSpecificationUseRecordOperationalGateBlocksDocumentaryOnly(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		specUseCurrentBaseline(section),
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          SpecUsePolicyDocumentaryOnly,
			OperationalGate: specUseGate(section.ID, "commission preflight"),
			Now:             specUseNow(),
		},
	)

	if record.Admission.Disposition != SpecUseDispositionAdmitted {
		t.Fatalf("admission = %+v, want admitted documentary reading", record.Admission)
	}
	if record.Admission.StrongerUse != ReviewUseDocumentaryReading {
		t.Fatalf("stronger_use = %q, want documentary reading", record.Admission.StrongerUse)
	}
	if record.GateDecision.Status != SpecUseGateDecisionBlocked {
		t.Fatalf("gate_decision = %+v, want blocked", record.GateDecision)
	}
	if record.GateDecision.Reason != "admission_not_granted" {
		t.Fatalf("reason = %q, want admission_not_granted", record.GateDecision.Reason)
	}
}

func TestBuildSpecificationUseRecordOperationalGateBlocksNonCurrentSource(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		SpecificationUseBaselineInput{Status: SpecUseBaselineDrifted},
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          SpecUsePolicyStrongerUseRequiresCurrentSource,
			OperationalGate: specUseGate(section.ID, "commission preflight"),
			Now:             specUseNow(),
		},
	)

	if record.GateDecision.Status != SpecUseGateDecisionBlocked {
		t.Fatalf("gate_decision = %+v, want blocked", record.GateDecision)
	}
	if record.GateDecision.Reason != "source_edition_not_current" {
		t.Fatalf("reason = %q, want source_edition_not_current", record.GateDecision.Reason)
	}
}

func TestBuildSpecificationUseRecordOperationalGateBlocksAdmissionMismatch(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		specUseCurrentBaseline(section),
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          "",
			OperationalGate: specUseGate(section.ID, "commission preflight"),
			Now:             specUseNow(),
		},
	)

	if record.GateDecision.Status != SpecUseGateDecisionBlocked {
		t.Fatalf("gate_decision = %+v, want blocked", record.GateDecision)
	}
	if record.GateDecision.Reason != "admission_not_granted" {
		t.Fatalf("reason = %q, want admission_not_granted", record.GateDecision.Reason)
	}
}

func TestBuildSpecificationUseRecordOperationalGateBlocksBearerAndUseMismatch(t *testing.T) {
	section := specUseActiveSection()
	record := BuildSpecificationUseRecord(
		section,
		specUseCurrentBaseline(section),
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          SpecUsePolicyStrongerUseRequiresCurrentSource,
			OperationalGate: specUseGate("TS.other.001", "commission preflight"),
			Now:             specUseNow(),
		},
	)
	if record.GateDecision.Reason != "operational_gate_bearer_mismatch" {
		t.Fatalf("reason = %q, want bearer mismatch", record.GateDecision.Reason)
	}

	record = BuildSpecificationUseRecord(
		section,
		specUseCurrentBaseline(section),
		SpecificationUseInput{
			SectionID:       section.ID,
			UseContext:      "commission preflight",
			Policy:          SpecUsePolicyStrongerUseRequiresCurrentSource,
			OperationalGate: specUseGate(section.ID, "release approval"),
			Now:             specUseNow(),
		},
	)
	if record.GateDecision.Reason != "operational_gate_use_context_mismatch" {
		t.Fatalf("reason = %q, want use context mismatch", record.GateDecision.Reason)
	}
}

func TestBuildSpecificationUseRecordOperationalGateFailsClosed(t *testing.T) {
	section := specUseActiveSection()
	for name, gate := range map[string]*OperationalGateProfile{
		"unknown schema": {
			SchemaVersion: 999,
			GateRef:       "gate-1",
			BearerRef:     section.ID,
			UseContext:    "commission preflight",
			Rule:          OperationalGateRuleCurrentAdmittedUse,
		},
		"unknown rule": {
			SchemaVersion: OperationalGateSchemaVersion,
			GateRef:       "gate-1",
			BearerRef:     section.ID,
			UseContext:    "commission preflight",
			Rule:          "approve_because_green",
		},
		"expired": {
			SchemaVersion: OperationalGateSchemaVersion,
			GateRef:       "gate-1",
			BearerRef:     section.ID,
			UseContext:    "commission preflight",
			Rule:          OperationalGateRuleCurrentAdmittedUse,
			ExpiresAt:     "2026-06-01T00:00:00Z",
		},
	} {
		record := BuildSpecificationUseRecord(
			section,
			specUseCurrentBaseline(section),
			SpecificationUseInput{
				SectionID:       section.ID,
				UseContext:      "commission preflight",
				Policy:          SpecUsePolicyStrongerUseRequiresCurrentSource,
				OperationalGate: gate,
				Now:             specUseNow(),
			},
		)
		if record.GateDecision.Status != SpecUseGateDecisionBlocked {
			t.Fatalf("%s: gate_decision = %+v, want blocked", name, record.GateDecision)
		}
	}
}

func specUseActiveSection() project.SpecSection {
	return project.SpecSection{
		ID:            "TS.use.001",
		Spec:          "target-system",
		DocumentKind:  "target-system",
		Kind:          "target.environment",
		Title:         "Use target",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "human",
		Status:        string(project.SpecSectionStateActive),
		ValidUntil:    "2099-01-01",
		Path:          ".haft/specs/target-system.md",
		Line:          4,
	}
}

func specUseCurrentBaseline(section project.SpecSection) SpecificationUseBaselineInput {
	return SpecificationUseBaselineInput{
		ProjectID: "proj-1",
		Status:    SpecUseBaselineCurrent,
		Baseline: SectionBaseline{
			ProjectID:  "proj-1",
			SectionID:  section.ID,
			Hash:       HashSection(section),
			ApprovedBy: "human",
			CapturedAt: specUseNow(),
		},
	}
}

func specUseGate(sectionID string, useContext string) *OperationalGateProfile {
	return &OperationalGateProfile{
		SchemaVersion:   OperationalGateSchemaVersion,
		GateRef:         "gate-spec-use-1",
		BearerRef:       sectionID,
		UseContext:      useContext,
		Rule:            OperationalGateRuleCurrentAdmittedUse,
		EvidenceRefs:    []string{"evid-1"},
		ExpiresAt:       "2099-01-01T00:00:00Z",
		ReopenCondition: "section baseline drifts or admission policy changes",
	}
}

func specUseNow() time.Time {
	return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
}
