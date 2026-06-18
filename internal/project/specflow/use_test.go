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

func specUseNow() time.Time {
	return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
}
