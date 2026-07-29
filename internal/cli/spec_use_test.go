package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecUseJSONReturnsSpecificationUseRecord(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()

	restoreFlags := stubSpecUseFlags(t, true, "agent planning read", specflow.SpecUsePolicyDocumentaryOnly, "", "")
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
	if record.GateDecision.AuthorityBoundary.Profile != "no_operational_gate_profile_not_gate_decision" {
		t.Fatalf("gate authority boundary = %+v", record.GateDecision.AuthorityBoundary)
	}
	if record.CurrentAuthority.AuthorityBoundary != specflow.CurrentAuthorityBoundaryReadOnly {
		t.Fatalf("current authority boundary = %q", record.CurrentAuthority.AuthorityBoundary)
	}
	if record.GateDecision.AuthorityBoundary.Publication != "not_publication" {
		t.Fatalf("gate publication boundary = %+v", record.GateDecision.AuthorityBoundary)
	}
	if strings.Contains(output.String(), `"status":"ready"`) || strings.Contains(output.String(), `"verdict":"pass"`) {
		t.Fatalf("spec use JSON must not expose ready/pass authority: %s", output.String())
	}
}

func TestBuildSpecUseRecordReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.use.001",
		Spec:          "target-system",
		Kind:          "target.environment",
		Title:         "SQL use section",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		ValidUntil:    "2026-12-31",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	record, err := buildSpecUseRecord(
		context.Background(),
		root,
		"TS.sql.use.001",
		"agent planning read",
		specflow.SpecUsePolicyDocumentaryOnly,
		"",
		nil,
		time.Now().UTC(),
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("buildSpecUseRecord: %v", err)
	}
	if record.SourceEdition.SectionID != "TS.sql.use.001" {
		t.Fatalf("source edition read carrier instead of SQL edition: %+v", record.SourceEdition)
	}
	if record.BaselineCurrentness.Status != specflow.SpecUseBaselineMissing {
		t.Fatalf("baseline currentness = %+v, want missing baseline for SQL edition", record.BaselineCurrentness)
	}
	if record.Admission.Disposition != specflow.SpecUseDispositionAdmitted {
		t.Fatalf("admission = %+v, want documentary admission", record.Admission)
	}
}

func TestRunSpecUseSummaryNamesSeparatedFields(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()

	restoreFlags := stubSpecUseFlags(t, false, "commission preflight", specflow.SpecUsePolicyStrongerUseRequiresCurrentSource, "", "")
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecUse(cmd, []string{"TS.environment.001"})
	if err != nil {
		t.Fatalf("runSpecUse returned error: %v", err)
	}

	result := output.String()
	for _, want := range []string{
		"admission:",
		"source_edition:",
		"baseline_currentness:",
		"current_authority:",
		"read_only_current_authority_frontier_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication",
		"gate_decision: not_applicable_no_operational_gate",
		"not_spec_approval/not_evidence_creation/not_work_commission/not_claim_truth/not_global_truth/not_publication",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("summary missing %q:\n%s", want, result)
		}
	}
}

func TestRunSpecUseJSONWithGateFileReturnsGateDecision(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()

	gateFile := writeSpecUseGateFile(t, specflow.OperationalGateProfile{
		SchemaVersion:   specflow.OperationalGateSchemaVersion,
		GateRef:         "gate-cli-1",
		BearerRef:       "TS.environment.001",
		UseContext:      "agent planning read",
		Rule:            specflow.OperationalGateRuleCurrentAdmittedUse,
		ExpiresAt:       "2099-01-01T00:00:00Z",
		ReopenCondition: "section baseline drifts",
	})
	restoreFlags := stubSpecUseFlags(t, true, "agent planning read", specflow.SpecUsePolicyDocumentaryOnly, "", gateFile)
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
	if record.GateDecision.Status != specflow.SpecUseGateDecisionBlocked {
		t.Fatalf("gate_decision = %+v, want blocked without baseline DB", record.GateDecision)
	}
	if record.GateDecision.GateRef != "gate-cli-1" {
		t.Fatalf("gate_ref = %q, want gate-cli-1", record.GateDecision.GateRef)
	}
	if record.GateDecision.OperationalGate == nil {
		t.Fatal("operational_gate missing")
	}
}

func TestHandleQuintQuerySpecUseReturnsCurrentnessRecord(t *testing.T) {
	fixture := newBoundSpecQueryTestProject(t)
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

func TestHandleQuintQuerySpecUseReadsCurrentAuthorityFromSectionRefs(t *testing.T) {
	fixture := newBoundSpecQueryTestProject(t)
	seedSpecUseSectionRefDecision(t, fixture.store, "dec-spec-section-ref", "TS.environment.001")

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
	if record.CurrentAuthority.Status != specflow.SpecUseCurrentAuthorityClear {
		t.Fatalf("current_authority = %+v, want clear", record.CurrentAuthority)
	}
	if !containsString(record.CurrentAuthority.DecisionRefs, "dec-spec-section-ref") {
		t.Fatalf("current_authority decision_refs = %#v", record.CurrentAuthority.DecisionRefs)
	}
	if !containsString(record.CurrentAuthority.TargetRefs, "spec_section:TS.environment.001") {
		t.Fatalf("current_authority target_refs = %#v", record.CurrentAuthority.TargetRefs)
	}
	if record.CurrentAuthority.AuthorityBoundary != specflow.CurrentAuthorityBoundaryReadOnly {
		t.Fatalf("current_authority boundary = %q", record.CurrentAuthority.AuthorityBoundary)
	}
}

func TestHandleQuintQuerySpecUseAcceptsOperationalGateObject(t *testing.T) {
	fixture := newBoundSpecQueryTestProject(t)
	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action":      "spec_use",
		"section_id":  "TS.environment.001",
		"use_context": "commission preflight",
		"policy":      specflow.SpecUsePolicyStrongerUseRequiresCurrentSource,
		"operational_gate": map[string]any{
			"schema_version":   float64(specflow.OperationalGateSchemaVersion),
			"gate_ref":         "gate-mcp-1",
			"bearer_ref":       "TS.environment.001",
			"use_context":      "commission preflight",
			"rule":             specflow.OperationalGateRuleCurrentAdmittedUse,
			"evidence_refs":    []any{"evid-1"},
			"expires_at":       "2099-01-01T00:00:00Z",
			"reopen_condition": "section baseline drifts",
		},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_use returned error: %v", err)
	}

	var record specflow.SpecificationUseRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode spec_use packet: %v\n%s", err, result)
	}
	if record.GateDecision.Status != specflow.SpecUseGateDecisionPassed {
		t.Fatalf("gate_decision = %+v, want passed", record.GateDecision)
	}
	if record.GateDecision.AuthorityBoundary.WorkCommission != "not_work_commission" {
		t.Fatalf("authority boundary = %+v", record.GateDecision.AuthorityBoundary)
	}
	if record.GateDecision.AuthorityBoundary.ClaimTruth != "not_claim_truth" {
		t.Fatalf("claim truth boundary = %+v", record.GateDecision.AuthorityBoundary)
	}
}

func TestHandleQuintQuerySpecUseOperationalGateBlocksCurrentAuthorityConflict(t *testing.T) {
	fixture := newBoundSpecQueryTestProject(t)
	seedSpecUseGoverningDecision(t, fixture.store, "dec-spec-use-a", "spec_section:TS.environment.001")
	seedSpecUseGoverningDecision(t, fixture.store, "dec-spec-use-b", "spec_section:TS.environment.001")
	if err := fixture.store.AddLink(context.Background(), "dec-spec-use-a", "dec-spec-use-b", "contradicts"); err != nil {
		t.Fatalf("add contradicts link: %v", err)
	}

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action":      "spec_use",
		"section_id":  "TS.environment.001",
		"use_context": "commission preflight",
		"policy":      specflow.SpecUsePolicyStrongerUseRequiresCurrentSource,
		"operational_gate": map[string]any{
			"schema_version":   float64(specflow.OperationalGateSchemaVersion),
			"gate_ref":         "gate-mcp-1",
			"bearer_ref":       "TS.environment.001",
			"use_context":      "commission preflight",
			"rule":             specflow.OperationalGateRuleCurrentAdmittedUse,
			"evidence_refs":    []any{"evid-1"},
			"expires_at":       "2099-01-01T00:00:00Z",
			"reopen_condition": "section baseline drifts",
		},
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_use returned error: %v", err)
	}

	var record specflow.SpecificationUseRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode spec_use packet: %v\n%s", err, result)
	}
	if record.CurrentAuthority.Status != specflow.SpecUseCurrentAuthorityConflict {
		t.Fatalf("current_authority = %+v, want conflict", record.CurrentAuthority)
	}
	if record.GateDecision.Status != specflow.SpecUseGateDecisionBlocked {
		t.Fatalf("gate_decision = %+v, want blocked", record.GateDecision)
	}
	if record.GateDecision.Reason != "current_authority_conflict_requires_operator" {
		t.Fatalf("gate reason = %q", record.GateDecision.Reason)
	}
}

func stubSpecUseFlags(
	t *testing.T,
	jsonOutput bool,
	useContext string,
	policy string,
	waiverExpiresAt string,
	gateFile string,
) func() {
	t.Helper()

	previousJSON := specUseJSON
	previousContext := specUseContext
	previousPolicy := specUsePolicy
	previousWaiver := specUseWaiverExpiresAt
	previousGateFile := specUseGateFile

	specUseJSON = jsonOutput
	specUseContext = useContext
	specUsePolicy = policy
	specUseWaiverExpiresAt = waiverExpiresAt
	specUseGateFile = gateFile

	return func() {
		specUseJSON = previousJSON
		specUseContext = previousContext
		specUsePolicy = previousPolicy
		specUseWaiverExpiresAt = previousWaiver
		specUseGateFile = previousGateFile
	}
}

func writeSpecUseGateFile(t *testing.T, gate specflow.OperationalGateProfile) string {
	t.Helper()

	data, err := json.Marshal(gate)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "gate.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func seedSpecUseGoverningDecision(
	t *testing.T,
	store *artifact.Store,
	id string,
	targetRef string,
) {
	t.Helper()

	fields := artifact.DecisionFields{
		DecisionSubjectRef: "spec_section:TS.environment.001",
		GovernanceTargets: []artifact.GovernanceTarget{{
			Kind: "spec_section",
			Ref:  targetRef,
		}},
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal decision fields: %v", err)
	}
	now := time.Now().UTC()
	err = store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        id,
			Kind:      artifact.KindDecisionRecord,
			Version:   1,
			Status:    artifact.StatusActive,
			Context:   "spec",
			Title:     "Spec use governing decision " + id,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "decision body",
		StructuredData: string(payload),
	})
	if err != nil {
		t.Fatalf("create decision %s: %v", id, err)
	}
}

func seedSpecUseSectionRefDecision(
	t *testing.T,
	store *artifact.Store,
	id string,
	sectionRef string,
) {
	t.Helper()

	fields := artifact.DecisionFields{
		DecisionSubjectRef: "spec_section:" + sectionRef,
		SectionRefs:        []string{sectionRef},
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal decision fields: %v", err)
	}
	now := time.Now().UTC()
	err = store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        id,
			Kind:      artifact.KindDecisionRecord,
			Version:   1,
			Status:    artifact.StatusActive,
			Context:   "spec",
			Title:     "Spec use section ref decision " + id,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body:           "decision body",
		StructuredData: string(payload),
	})
	if err != nil {
		t.Fatalf("create decision %s: %v", id, err)
	}
}
