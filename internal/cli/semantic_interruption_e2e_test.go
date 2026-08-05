package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestSemanticInterruptionBoundaryScopesAttentionAndBindingConflict(t *testing.T) {
	fixture := newSemanticInterruptionProject(t)
	ctx := context.Background()

	seedHistoricalImplementationDescription(t, fixture, time.Now().UTC())
	commissionID := seedAlreadyAuthorizedUnrelatedWork(t, fixture, ctx)

	status, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatalf("read status with historical prose: %v", err)
	}
	for _, marker := range []string{
		"Historical upload implementation description",
		"Interruption boundary",
		"Continue unrelated already-authorized Work",
	} {
		if !strings.Contains(status, marker) {
			t.Fatalf("status missing semantic-interruption marker %q:\n%s", marker, status)
		}
	}

	firstEvent := recordUnrelatedWorkEvidence(t, fixture, ctx, commissionID, "implementation_probe")
	assertNoAcknowledgementGate(t, firstEvent)
	assertRecordedWorkEvidence(t, firstEvent, "implementation_probe", 1)

	seedSpecUseGoverningDecision(t, fixture.store, "dec-semantic-gate-a", "spec_section:TS.sync.001")
	seedSpecUseGoverningDecision(t, fixture.store, "dec-semantic-gate-b", "spec_section:TS.sync.001")
	if err := fixture.store.AddLink(ctx, "dec-semantic-gate-a", "dec-semantic-gate-b", "contradicts"); err != nil {
		t.Fatalf("seed contradictory binding: %v", err)
	}

	result, err := handleQuintQuery(ctx, fixture.store, nil, fixture.haftDir, map[string]any{
		"action":      "spec_use",
		"section_id":  "TS.sync.001",
		"use_context": "commission preflight",
		"policy":      specflow.SpecUsePolicyStrongerUseRequiresCurrentSource,
		"operational_gate": map[string]any{
			"schema_version":   float64(specflow.OperationalGateSchemaVersion),
			"gate_ref":         "gate-semantic-interruption",
			"bearer_ref":       "TS.sync.001",
			"use_context":      "commission preflight",
			"rule":             specflow.OperationalGateRuleCurrentAdmittedUse,
			"evidence_refs":    []any{"evid-current-implementation-probe"},
			"expires_at":       "2099-01-01T00:00:00Z",
			"reopen_condition": "the section authority changes",
		},
	})
	if err != nil {
		t.Fatalf("evaluate affected spec use: %v", err)
	}

	var record specflow.SpecificationUseRecord
	if err := json.Unmarshal([]byte(result), &record); err != nil {
		t.Fatalf("decode affected spec-use result: %v\n%s", err, result)
	}
	if record.CurrentAuthority.Status != specflow.SpecUseCurrentAuthorityConflict {
		t.Fatalf("current authority = %+v, want exact conflict", record.CurrentAuthority)
	}
	if record.GateDecision.Status != specflow.SpecUseGateDecisionBlocked {
		t.Fatalf("gate decision = %+v, want affected operation blocked", record.GateDecision)
	}
	if record.GateDecision.Reason != "current_authority_conflict_requires_operator" {
		t.Fatalf("gate reason = %q, want exact semantic choice", record.GateDecision.Reason)
	}
	for _, decisionRef := range []string{"dec-semantic-gate-a", "dec-semantic-gate-b"} {
		if !containsString(record.CurrentAuthority.DecisionRefs, decisionRef) {
			t.Fatalf("current authority decision refs = %#v, missing %q", record.CurrentAuthority.DecisionRefs, decisionRef)
		}
	}
	if !containsString(record.CurrentAuthority.TargetRefs, "spec_section:TS.sync.001") {
		t.Fatalf("current authority target refs = %#v, want affected semantic target", record.CurrentAuthority.TargetRefs)
	}

	secondEvent := recordUnrelatedWorkEvidence(t, fixture, ctx, commissionID, "unrelated_verification")
	assertNoAcknowledgementGate(t, secondEvent)
	assertRecordedWorkEvidence(t, secondEvent, "unrelated_verification", 2)
}

func newSemanticInterruptionProject(t *testing.T) checkTestProject {
	t.Helper()

	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	t.Cleanup(func() { _ = database.Close() })

	rawDatabase := database.GetRawDB()
	fixture := checkTestProject{
		root:    root,
		haftDir: haftDirFor(root),
		store:   artifact.NewStore(rawDatabase),
		db:      rawDatabase,
	}
	specSet, err := project.LoadProjectSpecificationSet(root)
	if err != nil {
		t.Fatalf("load semantic-interruption spec set: %v", err)
	}
	editionStore := specflow.NewSQLiteSpecSectionEditionStore(rawDatabase)
	scope, err := newSpecSyncScope("")
	if err != nil {
		t.Fatalf("construct full-project sync scope: %v", err)
	}
	if _, err := syncProjectSpecificationSetToSQLWithScope(
		"qnt_5eec5eec",
		specSet,
		editionStore,
		scope,
	); err != nil {
		t.Fatalf("sync semantic-interruption spec editions: %v", err)
	}

	baselineStore := specflow.NewSQLiteBaselineStore(rawDatabase)
	for _, section := range specSet.Sections {
		if section.Status != string(project.SpecSectionStateActive) {
			continue
		}
		baseline := specflow.SectionBaseline{
			ProjectID:  "qnt_5eec5eec",
			SectionID:  section.ID,
			Hash:       specflow.HashSection(section),
			ApprovedBy: "semantic-interruption-fixture",
		}
		if err := baselineStore.Put(baseline); err != nil {
			t.Fatalf("baseline semantic-interruption section %s: %v", section.ID, err)
		}
	}

	return fixture
}

func seedHistoricalImplementationDescription(t *testing.T, fixture checkTestProject, now time.Time) {
	t.Helper()

	historical := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         "note-historical-upload-description",
			Kind:       artifact.KindNote,
			Version:    1,
			Status:     artifact.StatusActive,
			Context:    "implementation-history",
			Title:      "Historical upload implementation description",
			ValidUntil: now.Add(-24 * time.Hour).Format(time.RFC3339),
			CreatedAt:  now.Add(-30 * 24 * time.Hour),
			UpdatedAt:  now.Add(-30 * 24 * time.Hour),
		},
		Body: "Historical prose: upload the file before attempting the database record.",
	}
	if err := fixture.store.Create(context.Background(), historical); err != nil {
		t.Fatalf("seed historical implementation description: %v", err)
	}
}

func seedAlreadyAuthorizedUnrelatedWork(
	t *testing.T,
	fixture checkTestProject,
	ctx context.Context,
) string {
	t.Helper()

	decision := createCommissionDecisionFixture(
		t,
		ctx,
		fixture.store,
		fixture.haftDir,
		"Unrelated reversible work",
		"internal/cli/semantic_interruption_e2e_test.go",
	)
	result, err := handleHaftCommission(ctx, fixture.store, map[string]any{
		"action":        "create_from_decision",
		"decision_ref":  decision.Meta.ID,
		"repo_ref":      "local:haft",
		"base_sha":      "base-semantic-interruption",
		"target_branch": "feature/semantic-interruption",
		"valid_until":   "2099-01-01T00:00:00Z",
		"spec_readiness_override": map[string]any{
			"kind":              "tactical",
			"out_of_spec":       true,
			"project_readiness": "needs_onboard",
			"reason":            "acceptance fixture isolates an existing WorkCommission from unrelated attention",
		},
	})
	if err != nil {
		t.Fatalf("create already-authorized unrelated WorkCommission: %v", err)
	}

	var created map[string]map[string]any
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatalf("decode created WorkCommission: %v", err)
	}
	commissionID := stringField(created["commission"], "id")
	if commissionID == "" {
		t.Fatalf("created WorkCommission has no id: %#v", created)
	}

	_, err = handleHaftCommission(ctx, fixture.store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": commissionID,
		"runner_id":     "external:semantic-interruption",
	})
	if err != nil {
		t.Fatalf("claim already-authorized WorkCommission: %v", err)
	}

	return commissionID
}

func recordUnrelatedWorkEvidence(
	t *testing.T,
	fixture checkTestProject,
	ctx context.Context,
	commissionID string,
	phase string,
) string {
	t.Helper()

	result, err := handleHaftCommission(ctx, fixture.store, map[string]any{
		"action":        "record_run_event",
		"commission_id": commissionID,
		"runner_id":     "external:semantic-interruption",
		"event":         "phase_outcome",
		"verdict":       "pass",
		"payload": map[string]any{
			"phase": phase,
		},
	})
	if err != nil {
		t.Fatalf("record unrelated already-authorized Work evidence: %v", err)
	}

	return result
}

func assertNoAcknowledgementGate(t *testing.T, result string) {
	t.Helper()

	for _, forbidden := range []string{
		"operator_confirmation_required",
		"DECIDE THIS REVIEWED CHOICE",
		"reply yes",
		"ответь да",
	} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("unrelated authorized Work opened acknowledgement gate %q:\n%s", forbidden, result)
		}
	}
}

func assertRecordedWorkEvidence(t *testing.T, result string, phase string, eventCount int) {
	t.Helper()

	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("decode WorkCommission evidence response: %v", err)
	}
	events, ok := payload["commission"]["events"].([]any)
	if !ok || len(events) != eventCount {
		t.Fatalf("WorkCommission events = %#v, want %d", payload["commission"]["events"], eventCount)
	}
	last, ok := events[len(events)-1].(map[string]any)
	if !ok {
		t.Fatalf("last WorkCommission event = %#v, want object", events[len(events)-1])
	}
	eventPayload, ok := last["payload"].(map[string]any)
	if !ok || stringField(eventPayload, "phase") != phase {
		t.Fatalf("last WorkCommission event payload = %#v, want phase %q", last["payload"], phase)
	}
	if stringField(last, "runtime_run_id") == "" {
		t.Fatalf("last WorkCommission event has no RuntimeRun evidence ref: %#v", last)
	}
}
