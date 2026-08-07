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
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestRunSpecCheckCommandEmitsOneNeutralCueWithoutCanonicalProfile(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCheckJSON(t, false)
	defer restoreJSON()

	exitCode := stubSpecCheckExit(t)

	err := runSpecCheck(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCheck returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}
	if strings.Count(output.String(), "Profile cue:") != 1 {
		t.Fatalf("output = %q, want one profile cue", output.String())
	}
	if !strings.Contains(output.String(), "profile_underdetermined") {
		t.Fatalf("output = %q, want profile-underdetermined result", output.String())
	}
	for _, want := range []string{
		"Recovery surface: haft_onboard",
		"Next: " + projectSpecificationProfileRecoveryNextAction,
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output = %q, want %q", output.String(), want)
		}
	}
	if strings.Contains(output.String(), "software-system") {
		t.Fatalf("output = %q, want no speculative software pressure", output.String())
	}
}

func TestRunSpecCheckJSONRetainsNeutralProfileUnderdeterminedCue(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCheckJSON(t, true)
	defer restoreJSON()

	exitCode := stubSpecCheckExit(t)

	err := runSpecCheck(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCheck returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}

	var result publicSpecCheckResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if result.SpecCheckReport != nil {
		t.Fatalf("underdetermined result fabricated a spec-check report: %#v", result)
	}
	applicability := result.ProfileApplicability
	if applicability.Kind != string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("applicability kind = %q", applicability.Kind)
	}
	if applicability.Cue == nil ||
		applicability.Cue.Code != string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("profile cue = %#v", applicability.Cue)
	}
	if applicability.Cue.RecoverySurface !=
		projectSpecificationProfileRecoverySurface ||
		applicability.Cue.NextAction !=
			projectSpecificationProfileRecoveryNextAction {
		t.Fatalf("profile recovery cue = %#v", applicability.Cue)
	}
}

func TestRunSpecCoverageJSONFailsClosedOnlyItsOperationWithoutCanonicalProfile(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()
	restoreScope := stubSpecCoverageScopeID(t, "")
	defer restoreScope()
	exitCode := stubSpecCoverageExit(t)

	err := runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}

	var result publicSpecCoverageResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if result.SpecCoverageReport != nil {
		t.Fatalf("underdetermined profile fabricated coverage: %#v", result)
	}
	if result.ProfileApplicability.Kind != string(projectSpecificationProfileUnderdetermined) {
		t.Fatalf("profile applicability = %#v", result.ProfileApplicability)
	}
}

func TestRunSpecCoverageOmitsNotApplicableSoftwareSections(t *testing.T) {
	fixture := newNonSoftwareProfiledSpecTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()
	restoreScope := stubSpecCoverageScopeID(t, "documents-spec-cli")
	defer restoreScope()

	err := runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	var result publicSpecCoverageResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if result.SpecCoverageReport == nil {
		t.Fatalf("resolved non-software profile omitted coverage: %#v", result)
	}
	if result.ProfileApplicability.ScopeID != "documents-spec-cli" {
		t.Fatalf("scope_id = %q", result.ProfileApplicability.ScopeID)
	}
	if !sameSpecPlanCLIStrings(
		result.ProfileApplicability.ExcludedDocumentKinds,
		[]string{"software-system"},
	) {
		t.Fatalf(
			"excluded document kinds = %#v",
			result.ProfileApplicability.ExcludedDocumentKinds,
		)
	}
	for _, section := range result.Sections {
		if strings.HasPrefix(section.SectionID, "SS.") {
			t.Fatalf("not-applicable software section leaked into coverage: %#v", section)
		}
	}
}

func TestRunSpecCoverageJSONReportsDerivedSectionStates(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)

	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Cover checkout spec",
		WhySelected:     "The checkout section needs one governing decision for coverage derivation.",
		SelectionPolicy: "Prefer direct section linkage over manual status fields.",
		CounterArgument: "A fixture could overfit to one section id.",
		WeakestLink:     "Coverage depends on explicit section refs.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual coverage status",
			Reason:  "Manual status would violate the derived coverage contract.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Coverage no longer derives from section refs."},
		},
		AffectedFiles: []string{"internal/checkout/flow.go"},
		SectionRefs:   []string{"TS.covered.001"},
	})
	_, err := artifact.AttachEvidence(context.Background(), fixture.store, artifact.EvidenceInput{
		ArtifactRef:     decision.Meta.ID,
		Content:         "Checkout spec measurement passed.",
		Type:            "measurement",
		Verdict:         "supports",
		CongruenceLevel: 3,
		FormalityLevel:  2,
	})
	if err != nil {
		t.Fatalf("attach evidence: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()

	err = runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	var report project.SpecCoverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	states := map[string]project.SpecCoverageState{}
	for _, section := range report.Sections {
		states[section.SectionID] = section.State
		if len(section.Why) == 0 {
			t.Fatalf("section %s has empty why", section.SectionID)
		}
		if section.NextAction == "" {
			t.Fatalf("section %s has empty next_action", section.SectionID)
		}
	}

	if got := states["TS.covered.001"]; got != project.SpecCoverageVerified {
		t.Fatalf("covered state = %q, want verified", got)
	}
	if got := states["TS.uncovered.001"]; got != project.SpecCoverageUncovered {
		t.Fatalf("uncovered state = %q, want uncovered", got)
	}
	if strings.Contains(output.String(), "percent") {
		t.Fatalf("JSON output contains percentage scalar: %s", output.String())
	}
}

func TestBuildPublicSpecCoverageReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	profileHarness := profileadmissionfixture.OpenExisting(t, root)
	profileHarness.AdmitSoftwareRevisionWithTargetEntity(
		t,
		"spec-coverage-sql-first",
		"entity:spec-coverage-sql-first-target",
	)
	if err := profileHarness.Close(); err != nil {
		t.Fatalf("close profile admission fixture: %v", err)
	}
	database := openSpecSyncDB(t, root)
	defer database.Close()

	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.coverage.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "target.environment",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	result, err := buildPublicSpecCoverage(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatalf("buildPublicSpecCoverage: %v", err)
	}
	if result.SpecCoverageReport == nil {
		t.Fatalf("resolved software profile omitted coverage: %#v", result.ProfileApplicability)
	}
	report := *result.SpecCoverageReport

	if len(report.Sections) != 1 {
		t.Fatalf("sections = %#v, want exactly the current SQL edition section", report.Sections)
	}
	if report.Sections[0].SectionID != "TS.sql.coverage.001" {
		t.Fatalf("section id = %q, want SQL edition section", report.Sections[0].SectionID)
	}
	if report.Sections[0].SectionID == "TS.sync.001" {
		t.Fatalf("coverage used carrier section instead of SQL edition: %#v", report.Sections)
	}
}

func TestRunSpecCoverageJSONReportsRuntimeRunDerivedEdges(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)

	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Cover runtime spec",
		WhySelected:     "The runtime section needs a WorkCommission attempt and external evidence.",
		SelectionPolicy: "Prefer runtime/evidence carriers over manual coverage status.",
		CounterArgument: "The fixture could pass if runtime events are ignored.",
		WeakestLink:     "Coverage depends on WorkCommission event storage being decoded.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Manual coverage status",
			Reason:  "Manual status would violate the derived coverage contract.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"RuntimeRun events no longer appear in SpecCoverage edges."},
		},
		AffectedFiles: []string{"internal/runtime/edge.go"},
		SectionRefs:   []string{"TS.covered.001"},
	})

	commissionID := "wc-runtime-coverage"
	runtimeRunID := commissionID + "#runtime-run-001"
	createSpecCoverageRuntimeCommission(t, fixture, decision.Meta.ID, commissionID)

	evidence, err := artifact.AttachEvidence(context.Background(), fixture.store, artifact.EvidenceInput{
		ArtifactRef:     commissionID,
		Content:         "Runtime evidence passed for the covered section.",
		Type:            "measurement",
		Verdict:         "supports",
		CarrierRef:      runtimeRunID,
		CongruenceLevel: 3,
		FormalityLevel:  2,
	})
	if err != nil {
		t.Fatalf("attach runtime evidence: %v", err)
	}

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()

	err = runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	var report project.SpecCoverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if len(report.Gaps) != 0 {
		t.Fatalf("global gaps = %#v, want no synthetic RuntimeRun gap", report.Gaps)
	}

	section := specCoverageCLISection(report, "TS.covered.001")
	if section.State != project.SpecCoverageVerified {
		t.Fatalf("covered state = %q, want verified; section = %#v", section.State, section)
	}
	if !specCoverageCLIEdgeTarget(section.Edges, runtimeRunID) {
		t.Fatalf("edges = %#v, want RuntimeRun edge", section.Edges)
	}
	runtimeRunEdge := specCoverageCLIEdge(section.Edges, project.SpecCoverageEdgeRuntimeRun, runtimeRunID)
	if runtimeRunEdge.CommissionRef != commissionID {
		t.Fatalf("runtime commission_ref = %q, want %q", runtimeRunEdge.CommissionRef, commissionID)
	}
	if runtimeRunEdge.StartedAt != "2026-04-25T12:00:00Z" {
		t.Fatalf("runtime started_at = %q", runtimeRunEdge.StartedAt)
	}
	if runtimeRunEdge.CompletedAt != "2026-04-25T12:04:00Z" {
		t.Fatalf("runtime completed_at = %q", runtimeRunEdge.CompletedAt)
	}
	if runtimeRunEdge.EvidenceStatus != project.RuntimeEvidenceSupports {
		t.Fatalf("runtime evidence_status = %q, want supports", runtimeRunEdge.EvidenceStatus)
	}
	if !sameSpecPlanCLIStrings(runtimeRunEdge.EvidenceRefs, []string{evidence.ID}) {
		t.Fatalf("runtime evidence_refs = %#v, want %s", runtimeRunEdge.EvidenceRefs, evidence.ID)
	}
	if len(runtimeRunEdge.PhaseOutcomes) != 5 {
		t.Fatalf("runtime phase_outcomes = %#v, want five persisted lifecycle outcomes", runtimeRunEdge.PhaseOutcomes)
	}
	if !specCoverageCLIEdgeTarget(section.Edges, evidence.ID) {
		t.Fatalf("edges = %#v, want evidence edge", section.Edges)
	}
}

func TestRunSpecCoverageJSONReportsMalformedRuntimeCarrierGap(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)

	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Cover malformed runtime spec",
		WhySelected:     "Malformed runtime carriers must be represented as coverage gaps.",
		SelectionPolicy: "Prefer explicit gaps over silent runtime carrier loss.",
		CounterArgument: "The CLI could drop malformed events and make the graph look cleaner than reality.",
		WeakestLink:     "Coverage depends on WorkCommission event storage being decoded.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Ignore malformed runtime carrier",
			Reason:  "Silent carrier loss would violate derived coverage evidence semantics.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Malformed RuntimeRun events disappear from SpecCoverage gaps."},
		},
		AffectedFiles: []string{"internal/runtime/malformed.go"},
		SectionRefs:   []string{"TS.covered.001"},
	})

	createSpecCoverageMalformedRuntimeCommission(t, fixture, decision.Meta.ID, "wc-runtime-malformed", "run-runtime-malformed")

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()

	err := runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	var report project.SpecCoverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	section := specCoverageCLISection(report, "TS.covered.001")
	if section.State != project.SpecCoverageCommissioned {
		t.Fatalf("covered state = %q, want commissioned; section = %#v", section.State, section)
	}
	if !specCoverageCLIEdgeTarget(section.Edges, "run-runtime-malformed") {
		t.Fatalf("edges = %#v, want malformed RuntimeRun edge", section.Edges)
	}
	if !specCoverageCLIGapKind(section.Gaps, "runtime_run_unsupported") {
		t.Fatalf("gaps = %#v, want runtime_run_unsupported", section.Gaps)
	}
}

func TestRunSpecCoverageHumanSummaryIncludesWhyAndNextAction(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecCoverageCLICarriers(t, fixture.root)

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, false)
	defer restoreJSON()

	err := runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "haft spec coverage: 3 active section(s)") {
		t.Fatalf("output = %q, want section count summary", result)
	}
	if !strings.Contains(result, "why:") {
		t.Fatalf("output = %q, want per-section why", result)
	}
	if !strings.Contains(result, "next_action:") {
		t.Fatalf("output = %q, want per-section next_action", result)
	}
	if strings.Contains(result, "%") {
		t.Fatalf("output contains percentage scalar: %s", result)
	}
}

func TestRunSpecCoverageBlocksWhenSpecCheckHasFindings(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecCheckCLIFile(
		t,
		filepath.Join(fixture.haftDir, "specs", "term-map.md"),
		invalidCLITermMapCarrier(),
	)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecCoverage(cmd, nil)
	if err == nil {
		t.Fatal("runSpecCoverage returned nil, want spec-check block")
	}
	if !strings.Contains(err.Error(), "spec coverage blocked") {
		t.Fatalf("error = %q, want spec coverage block", err.Error())
	}
	if !strings.Contains(err.Error(), "haft spec check") {
		t.Fatalf("error = %q, want spec check next action", err.Error())
	}
}

func TestRunSpecCoverageJSONReportsSpecCheckBlock(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecCheckCLIFile(
		t,
		filepath.Join(fixture.haftDir, "specs", "term-map.md"),
		invalidCLITermMapCarrier(),
	)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecCoverageJSON(t, true)
	defer restoreJSON()

	exitCode := stubSpecCoverageExit(t)

	err := runSpecCoverage(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecCoverage returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}

	var report specCoverageBlockedJSONReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if report.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", report.Status)
	}
	if report.SpecCheck.Summary.TotalFindings == 0 {
		t.Fatalf("spec_check findings = 0, want blocker findings")
	}
	if len(report.Coverage.Gaps) != 1 {
		t.Fatalf("coverage gaps = %#v, want spec-check block gap", report.Coverage.Gaps)
	}
	if report.Coverage.Gaps[0].Kind != "spec_check_blocked" {
		t.Fatalf("gap kind = %q, want spec_check_blocked", report.Coverage.Gaps[0].Kind)
	}
	if report.SpecCheck.Findings[0].NextAction == "" {
		t.Fatalf("spec_check finding missing next_action: %+v", report.SpecCheck.Findings[0])
	}
}

func TestRunSpecPlanJSONGroupsDraftsAndDoesNotMutateDB(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecPlanCLICarriers(t, fixture.root)

	before := countSpecPlanArtifacts(t, fixture)

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecPlanJSON(t, true)
	defer restoreJSON()

	err := runSpecPlan(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecPlan returned error: %v", err)
	}

	after := countSpecPlanArtifacts(t, fixture)
	if after != before {
		t.Fatalf("artifact count changed from %d to %d; spec plan listing must be read-only", before, after)
	}
	assertSpecPlanNoStoredKind(t, fixture, artifact.KindDecisionRecord)
	assertSpecPlanNoStoredKind(t, fixture, artifact.KindWorkCommission)

	var report project.SpecPlanReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if report.Summary.TotalCandidates != 4 {
		t.Fatalf("total_candidates = %d, want 4", report.Summary.TotalCandidates)
	}
	if len(report.Proposals) != 3 {
		t.Fatalf("proposals = %#v, want grouped proposals", report.Proposals)
	}

	proposal := specPlanCLIProposal(report, "acceptance", "checkout", []string{"TS.role.001"})
	if got := proposal.SectionRefs; !sameSpecPlanCLIStrings(got, []string{"TS.checkout.001", "TS.checkout.002"}) {
		t.Fatalf("checkout proposal section_refs = %#v, want grouped uncovered/stale sections", got)
	}
	if got := proposal.DecisionRecordDraft.SectionRefs; !sameSpecPlanCLIStrings(got, proposal.SectionRefs) {
		t.Fatalf("draft section_refs = %#v, want proposal section refs %#v", got, proposal.SectionRefs)
	}
}

func TestRunSpecPlanJSONRequiresExactScopeForMixedProfile(t *testing.T) {
	fixture := newMixedProfiledSpecTestProject(t)
	writeSpecPlanCLICarriers(t, fixture.root)
	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	before := countSpecPlanArtifacts(t, fixture)
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecPlanJSON(t, true)
	defer restoreJSON()
	restoreScope := stubSpecPlanScopeID(t, "")
	defer restoreScope()
	restoreAccept := stubSpecPlanAccept(t, "must-not-bind-without-scope")
	defer restoreAccept()
	exitCode := stubSpecPlanExit(t)

	err := runSpecPlan(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecPlan returned error: %v", err)
	}
	if *exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", *exitCode)
	}
	if after := countSpecPlanArtifacts(t, fixture); after != before {
		t.Fatalf("artifact count changed from %d to %d", before, after)
	}

	var result publicSpecPlanResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if result.SpecPlanReport != nil {
		t.Fatalf("scope-choice result fabricated a plan: %#v", result)
	}
	if result.ProfileApplicability.Kind != string(projectSpecificationScopeChoiceRequired) {
		t.Fatalf("profile applicability = %#v", result.ProfileApplicability)
	}
}

func TestRunSpecPlanAcceptBindsFromHostRoutedOperatorRequest(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecPlanCLICarriers(t, fixture.root)

	request := automaticProjectSpecificationScopeRequest()
	result, err := buildPublicSpecPlan(
		context.Background(),
		fixture.root,
		request,
	)
	if err != nil {
		t.Fatalf("buildPublicSpecPlan: %v", err)
	}
	if result.SpecPlanReport == nil || len(result.Proposals) == 0 {
		t.Fatalf("missing proposal in %#v", result)
	}
	proposal := result.Proposals[0]

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecPlanJSON(t, true)
	defer restoreJSON()
	restoreAccept := stubSpecPlanAccept(t, proposal.ID)
	defer restoreAccept()
	restoreScope := stubSpecPlanScopeID(t, "")
	defer restoreScope()

	before := countSpecPlanArtifacts(t, fixture)
	err = runSpecPlan(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecPlan accept returned error: %v", err)
	}
	if after := countSpecPlanArtifacts(t, fixture); after != before+1 {
		t.Fatalf("artifact count changed from %d to %d", before, after)
	}

	var accepted specPlanAcceptResult
	if err := json.Unmarshal(output.Bytes(), &accepted); err != nil {
		t.Fatalf("decode accept JSON output: %v", err)
	}
	if accepted.DecisionRef == "" || accepted.DecisionTitle == "" {
		t.Fatalf("accepted decision identity is incomplete: %#v", accepted)
	}
	if accepted.ProfileApplicability.ScopeID != "software-spec-cli" {
		t.Fatalf("accept lost exact profile scope: %#v", accepted.ProfileApplicability)
	}
	decision, err := fixture.store.Get(context.Background(), accepted.DecisionRef)
	if err != nil {
		t.Fatalf("load accepted DecisionRecord: %v", err)
	}
	fields := decision.UnmarshalDecisionFields()
	if fields.AuthorityProvenance != "host_routed_operator_request" {
		t.Fatalf("DecisionRecord authority provenance = %q", fields.AuthorityProvenance)
	}
	if !sameSpecPlanCLIStrings(fields.SectionRefs, accepted.SectionRefs) {
		t.Fatalf(
			"DecisionRecord section_refs = %#v, want %#v",
			fields.SectionRefs,
			accepted.SectionRefs,
		)
	}
	assertSpecPlanNoStoredKind(t, fixture, artifact.KindWorkCommission)
}

func TestRunSpecPlanAcceptUsesHostRoutedDecisionBinder(t *testing.T) {
	fixture := newCheckTestProject(t)
	proposal := project.SpecPlanProposal{
		ID:           "spec-plan-test-accept",
		Title:        "Bind checkout acceptance sections",
		SectionRefs:  []string{"TS.checkout.001", "TS.checkout.002"},
		Reasons:      []string{"checkout acceptance basis is current"},
		AffectedArea: "checkout",
		DecisionRecordDraft: project.SpecPlanDecisionDraft{
			SelectedTitle:   "Bind checkout acceptance sections",
			WhySelected:     "The sections share one acceptance boundary.",
			SelectionPolicy: "Bind the smallest coherent specification group.",
			CounterArgument: "Separate decisions would isolate each section.",
			WhyNotOthers: []project.SpecPlanRejectedAlternative{{
				Variant: "Keep sections unbound",
				Reason:  "It leaves the current acceptance basis implicit.",
			}},
			WeakestLink:      "The sections may diverge later.",
			RollbackTriggers: []string{"The sections acquire different owners."},
			SectionRefs:      []string{"TS.checkout.001", "TS.checkout.002"},
		},
	}
	report := project.SpecPlanReport{
		Proposals: []project.SpecPlanProposal{proposal},
	}

	before := countSpecPlanArtifacts(t, fixture)
	stubHostRoutedDecisionBinder(t, &fakeHostRoutedDecisionBinder{
		result: decisionBindingOutcome{
			DecisionRef: "dec-20260715-spec-plan-a1b2c3d4",
			Title:       proposal.DecisionRecordDraft.SelectedTitle,
			FilePath:    filepath.Join(fixture.root, ".haft", "decisions", "spec-plan.md"),
		},
	})
	specificationSet := project.ProjectSpecificationSet{
		Sections: []project.SpecSection{
			{ID: "TS.checkout.001", Status: "active"},
			{ID: "TS.checkout.002", Status: "active"},
		},
	}
	result, err := acceptSpecPlanProposalForSpecificationSet(
		context.Background(),
		fixture.root,
		report,
		proposal.ID,
		specificationSet,
	)
	if err != nil {
		t.Fatalf("acceptSpecPlanProposalForSpecificationSet error = %v", err)
	}

	after := countSpecPlanArtifacts(t, fixture)
	if after != before {
		t.Fatalf("artifact count changed from %d to %d; the fake binding seam must not create a DecisionRecord", before, after)
	}
	assertSpecPlanNoStoredKind(t, fixture, artifact.KindDecisionRecord)
	assertSpecPlanNoStoredKind(t, fixture, artifact.KindWorkCommission)
	assertSpecPlanNoPlanFiles(t, fixture.root)

	if result.Action != string(project.SpecPlanActionAccept) {
		t.Fatalf("action = %q, want %q", result.Action, project.SpecPlanActionAccept)
	}
	if result.DecisionRef != "dec-20260715-spec-plan-a1b2c3d4" ||
		result.DecisionTitle != proposal.DecisionRecordDraft.SelectedTitle {
		t.Fatalf("readable decision identity = (%q, %q)", result.DecisionRef, result.DecisionTitle)
	}
	if got := result.SectionRefs; !sameSpecPlanCLIStrings(got, []string{"TS.checkout.001", "TS.checkout.002"}) {
		t.Fatalf("result section_refs = %#v, want accepted proposal section refs", got)
	}
}

func TestRunSpecPlanHumanSummaryStatesProposalAuthorityAndReviewActions(t *testing.T) {
	fixture := newSoftwareProfiledSpecTestProject(t)
	writeSpecPlanCLICarriers(t, fixture.root)

	restore := enterTestProjectRoot(t, fixture.root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecPlanJSON(t, false)
	defer restoreJSON()

	err := runSpecPlan(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecPlan returned error: %v", err)
	}

	result := output.String()
	required := []string{
		"haft spec plan:",
		"does not create DecisionRecords",
		"WorkCommissions",
		"review_actions: accept, merge, split, discard",
		"decision_record_draft:",
	}
	for _, want := range required {
		if !strings.Contains(result, want) {
			t.Fatalf("output = %q, want %q", result, want)
		}
	}
}

func TestSpecPlanHelpStatesProposalsAreNotAuthority(t *testing.T) {
	t.Parallel()

	help := strings.Join(strings.Fields(specPlanCmd.Long), " ")
	required := []string{
		"not authority",
		"Use --accept <proposal-id>",
		"no WorkCommissions are",
		"Merge, split, and discard",
		"direct, unambiguous operator request",
		"does not require a skill token",
		"exact --scope-id",
	}

	for _, want := range required {
		if !strings.Contains(help, want) {
			t.Fatalf("spec plan help missing %q:\n%s", want, specPlanCmd.Long)
		}
	}
	if specPlanCmd.Flags().Lookup("scope-id") == nil {
		t.Fatal("spec plan omitted exact --scope-id selector")
	}
	if specCoverageCmd.Flags().Lookup("scope-id") == nil {
		t.Fatal("spec coverage omitted exact --scope-id selector")
	}
	if !strings.Contains(specCoverageCmd.Long, "Unresolved applicability blocks") {
		t.Fatalf("spec coverage help lost local fail-closed boundary:\n%s", specCoverageCmd.Long)
	}
}

func newSoftwareProfiledSpecTestProject(t *testing.T) checkTestProject {
	t.Helper()

	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevisionWithTargetEntity(
		t,
		"spec-cli",
		"entity:spec-cli-target",
	)
	return newProfiledSpecTestProject(t, harness)
}

func newNonSoftwareProfiledSpecTestProject(t *testing.T) checkTestProject {
	t.Helper()

	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	scope := newSpecTestNonSoftwareScope(
		t,
		"documents-spec-cli",
		"entity:spec-cli-documents",
	)
	admitSpecTestProfile(t, harness, "spec-cli-non-software", []projectprofile.RealizationScope{scope})
	return newProfiledSpecTestProject(t, harness)
}

func newMixedProfiledSpecTestProject(t *testing.T) checkTestProject {
	t.Helper()

	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	softwareScopeID, err := projectprofile.NewScopeID("software-spec-cli")
	if err != nil {
		t.Fatalf("software ScopeID: %v", err)
	}
	softwareEntityRef, err := projectprofile.NewEntityRef("entity:spec-cli-target")
	if err != nil {
		t.Fatalf("software EntityRef: %v", err)
	}
	softwareScope, err := projectprofile.NewSoftwareRealization(
		softwareScopeID,
		projectprofile.NewReferencedEntity(softwareEntityRef),
	)
	if err != nil {
		t.Fatalf("software realization: %v", err)
	}
	nonSoftwareScope := newSpecTestNonSoftwareScope(
		t,
		"documents-spec-cli",
		"entity:spec-cli-documents",
	)
	admitSpecTestProfile(
		t,
		harness,
		"spec-cli-mixed",
		[]projectprofile.RealizationScope{softwareScope, nonSoftwareScope},
	)
	return newProfiledSpecTestProject(t, harness)
}

func newSpecTestNonSoftwareScope(
	t *testing.T,
	rawScopeID string,
	rawEntityRef string,
) projectprofile.NonSoftwareRealization {
	t.Helper()

	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		t.Fatalf("non-software ScopeID: %v", err)
	}
	entityRef, err := projectprofile.NewEntityRef(rawEntityRef)
	if err != nil {
		t.Fatalf("non-software EntityRef: %v", err)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NewReferencedEntity(entityRef),
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("non-software realization: %v", err)
	}
	return scope
}

func admitSpecTestProfile(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
	suffix string,
	scopes []projectprofile.RealizationScope,
) {
	t.Helper()

	scopeSet, err := projectprofile.NewScopeSet(scopes)
	if err != nil {
		t.Fatalf("profile ScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopeSet)
	if err != nil {
		t.Fatalf("profile payload: %v", err)
	}
	request := harness.PrepareRevision(t, suffix, payload)
	service, err := profileadmissionsqlite.NewService(harness.Database())
	if err != nil {
		t.Fatalf("profile admission service: %v", err)
	}
	result := service.Admit(context.Background(), request)
	if result.Kind() != profileadmissionsqlite.AdmissionResultAdmitted {
		denials, _ := result.Denials()
		failure, _ := result.Failure()
		t.Fatalf(
			"profile admission = %q, denials = %#v, failure = %#v",
			result.Kind(),
			denials,
			failure,
		)
	}
}

func newProfiledSpecTestProject(
	t *testing.T,
	harness *profileadmissionfixture.Harness,
) checkTestProject {
	t.Helper()

	root := harness.Root().String()
	fixture := checkTestProject{
		root:    root,
		haftDir: filepath.Join(root, ".haft"),
		store:   artifact.NewStore(harness.Database()),
		db:      harness.Database(),
	}
	writeCheckTestSpecCarriers(t, fixture)
	return fixture
}

func newSpecCheckCLIProject(t *testing.T, termMap string) string {
	t.Helper()

	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), validCLISpecSectionCarrier("TS.use.001", "environment-change"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "software-system.md"), validCLISpecSectionCarrier("SS.role.001", "software.role"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), termMap)

	return root
}

func validCLISpecSectionCarrier(id string, kind string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"```\n"
}

func validCLITermMapCarrier() string {
	return "```yaml term-map\n" +
		"entries:\n" +
		"  - term: HarnessableProject\n" +
		"    category: enabling\n" +
		"    definition: A project with active specs.\n" +
		"```\n"
}

func invalidCLITermMapCarrier() string {
	return "```yaml term-map\n" +
		"entries:\n" +
		"  - category: enabling\n" +
		"    definition: A project with active specs.\n" +
		"```\n"
}

func writeSpecCoverageCLICarriers(t *testing.T, root string) {
	t.Helper()

	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), strings.Join([]string{
		coverageCLISpecSection("TS.covered.001", "Covered section"),
		coverageCLISpecSection("TS.uncovered.001", "Uncovered section"),
	}, "\n"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "software-system.md"), coverageCLISpecSection("SS.role.001", "Software role"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())
}

func writeSpecPlanCLICarriers(t *testing.T, root string) {
	t.Helper()

	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), strings.Join([]string{
		specPlanCLISpecSection("TS.checkout.001", "Checkout active", "acceptance", "", []string{"TS.role.001"}),
		specPlanCLISpecSection("TS.checkout.002", "Checkout stale", "acceptance", "2020-01-01", []string{"TS.role.001"}),
		specPlanCLISpecSection("TS.checkout.003", "Checkout boundary", "acceptance", "", []string{"TS.boundary.001"}),
	}, "\n"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "software-system.md"), coverageCLISpecSection("SS.role.001", "Software role"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())
}

func specPlanCLISpecSection(
	id string,
	title string,
	kind string,
	validUntil string,
	dependsOn []string,
) string {
	lines := []string{
		"## " + id + " " + title,
		"",
		"```yaml spec-section",
		"id: " + id,
		"kind: " + kind,
		"title: " + title,
		"statement_type: evidence",
		"claim_layer: object",
		"owner: human",
		"status: active",
	}
	if validUntil != "" {
		lines = append(lines, "valid_until: "+validUntil)
	}
	if len(dependsOn) > 0 {
		lines = append(lines, "depends_on:")
		for _, ref := range dependsOn {
			lines = append(lines, "  - "+ref)
		}
	}
	lines = append(lines, "```", "")

	return strings.Join(lines, "\n")
}

func coverageCLISpecSection(id string, title string) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: environment-change\n" +
		"title: " + title + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"```\n"
}

func coverageCLIDraftSpecSection(id string, title string) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: creator-role\n" +
		"title: " + title + "\n" +
		"statement_type: explanation\n" +
		"claim_layer: carrier\n" +
		"owner: human\n" +
		"status: draft\n" +
		"```\n"
}

func createSpecCoverageRuntimeCommission(
	t *testing.T,
	fixture checkTestProject,
	decisionID string,
	commissionID string,
) {
	t.Helper()

	payload := map[string]any{
		"id":           commissionID,
		"decision_ref": decisionID,
		"state":        "completed",
		"valid_until":  "2099-01-01T00:00:00Z",
		"events": []any{
			map[string]any{
				"action":      "record_run_event",
				"event":       "phase_outcome",
				"verdict":     "pass",
				"recorded_at": "2026-04-25T12:00:00Z",
				"payload": map[string]any{
					"phase": "preflight",
					"next":  "advance:execute",
				},
			},
			map[string]any{
				"action":      "record_preflight",
				"event":       "preflight_checked",
				"verdict":     "pass",
				"recorded_at": "2026-04-25T12:01:00Z",
				"payload": map[string]any{
					"phase": "preflight",
				},
			},
			map[string]any{
				"action":      "start_after_preflight",
				"event":       "preflight_passed",
				"verdict":     "pass",
				"recorded_at": "2026-04-25T12:02:00Z",
				"payload": map[string]any{
					"phase": "preflight",
				},
			},
			map[string]any{
				"action":      "record_run_event",
				"event":       "phase_outcome",
				"verdict":     "pass",
				"recorded_at": "2026-04-25T12:03:00Z",
				"payload": map[string]any{
					"phase": "execute",
					"next":  "advance:measure",
				},
			},
			map[string]any{
				"action":      "complete_or_block",
				"event":       "workflow_terminal",
				"verdict":     "pass",
				"recorded_at": "2026-04-25T12:04:00Z",
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode runtime commission: %v", err)
	}

	err = fixture.store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         commissionID,
			Kind:       artifact.KindWorkCommission,
			Status:     artifact.StatusActive,
			Title:      "Runtime coverage commission",
			ValidUntil: "2099-01-01T00:00:00Z",
		},
		Body:           "Runtime coverage commission",
		StructuredData: string(encoded),
	})
	if err != nil {
		t.Fatalf("create runtime commission: %v", err)
	}
}

func createSpecCoverageMalformedRuntimeCommission(
	t *testing.T,
	fixture checkTestProject,
	decisionID string,
	commissionID string,
	runtimeRunID string,
) {
	t.Helper()

	payload := map[string]any{
		"id":           commissionID,
		"decision_ref": decisionID,
		"state":        "running",
		"valid_until":  "2099-01-01T00:00:00Z",
		"events": []any{
			map[string]any{
				"action":         "record_run_event",
				"event":          "phase_outcome",
				"runtime_run_id": runtimeRunID,
				"recorded_at":    "2026-04-25T12:00:00Z",
				"payload": map[string]any{
					"phase": "execute",
				},
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode malformed runtime commission: %v", err)
	}

	err = fixture.store.Create(context.Background(), &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         commissionID,
			Kind:       artifact.KindWorkCommission,
			Status:     artifact.StatusActive,
			Title:      "Malformed runtime coverage commission",
			ValidUntil: "2099-01-01T00:00:00Z",
		},
		Body:           "Malformed runtime coverage commission",
		StructuredData: string(encoded),
	})
	if err != nil {
		t.Fatalf("create malformed runtime commission: %v", err)
	}
}

func specCoverageCLISection(report project.SpecCoverageReport, sectionID string) project.SpecCoverageSection {
	for _, section := range report.Sections {
		if section.SectionID == sectionID {
			return section
		}
	}

	return project.SpecCoverageSection{}
}

func specCoverageCLIEdgeTarget(edges []project.SpecCoverageEdge, target string) bool {
	for _, edge := range edges {
		if edge.Target == target {
			return true
		}
	}

	return false
}

func specCoverageCLIEdge(
	edges []project.SpecCoverageEdge,
	edgeType project.SpecCoverageEdgeType,
	target string,
) project.SpecCoverageEdge {
	for _, edge := range edges {
		if edge.Type != edgeType {
			continue
		}
		if edge.Target == target {
			return edge
		}
	}

	return project.SpecCoverageEdge{}
}

func specCoverageCLIGapKind(gaps []project.SpecCoverageGap, kind string) bool {
	for _, gap := range gaps {
		if gap.Kind == kind {
			return true
		}
	}

	return false
}

func countSpecPlanArtifacts(t *testing.T, fixture checkTestProject) int {
	t.Helper()

	items, err := fixture.store.ListByKind(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("count artifacts: %v", err)
	}

	return len(items)
}

func assertSpecPlanNoStoredKind(t *testing.T, fixture checkTestProject, kind artifact.Kind) {
	t.Helper()

	items, err := fixture.store.ListByKind(context.Background(), kind, 0)
	if err != nil {
		t.Fatalf("list %s artifacts: %v", kind, err)
	}
	if len(items) != 0 {
		t.Fatalf("%s artifacts = %#v, want none", kind, items)
	}
}

func assertSpecPlanNoPlanFiles(t *testing.T, root string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(root, ".haft", "plans"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read .haft/plans: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf(".haft/plans entries = %#v, want none", entries)
	}
}

func assertSpecPlanDecisionLinks(
	t *testing.T,
	fixture checkTestProject,
	decisionRef string,
	sectionRefs []string,
) {
	t.Helper()

	links, err := fixture.store.GetLinks(context.Background(), decisionRef)
	if err != nil {
		t.Fatalf("load decision links: %v", err)
	}

	seen := map[string]bool{}
	for _, link := range links {
		if link.Type != "governs" {
			continue
		}

		seen[link.Ref] = true
	}

	for _, ref := range sectionRefs {
		if seen[ref] {
			continue
		}

		t.Fatalf("decision links = %#v, want governs link to %s", links, ref)
	}
}

func specPlanCLIProposal(
	report project.SpecPlanReport,
	specKind string,
	affectedArea string,
	dependencyRefs []string,
) project.SpecPlanProposal {
	for _, proposal := range report.Proposals {
		if proposal.SpecKind != specKind {
			continue
		}
		if proposal.AffectedArea != affectedArea {
			continue
		}
		if !sameSpecPlanCLIStrings(proposal.DependencyRefs, dependencyRefs) {
			continue
		}

		return proposal
	}

	return project.SpecPlanProposal{}
}

func sameSpecPlanCLIStrings(left []string, right []string) bool {
	left = cleanStringSlice(left)
	right = cleanStringSlice(right)
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func writeSpecCheckCLIFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stubSpecCheckJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specCheckJSON
	specCheckJSON = value

	return func() {
		specCheckJSON = previous
	}
}

func stubSpecCheckExit(t *testing.T) *int {
	t.Helper()

	exitCode := new(int)
	previous := specCheckExit
	specCheckExit = func(code int) {
		*exitCode = code
	}
	t.Cleanup(func() {
		specCheckExit = previous
	})

	return exitCode
}

func stubSpecCoverageExit(t *testing.T) *int {
	t.Helper()

	exitCode := new(int)
	previous := specCoverageExit
	specCoverageExit = func(code int) {
		*exitCode = code
	}
	t.Cleanup(func() {
		specCoverageExit = previous
	})

	return exitCode
}

func stubSpecCoverageJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specCoverageJSON
	specCoverageJSON = value

	return func() {
		specCoverageJSON = previous
	}
}

func stubSpecCoverageScopeID(t *testing.T, value string) func() {
	t.Helper()

	previous := specCoverageScopeID
	specCoverageScopeID = value

	return func() {
		specCoverageScopeID = previous
	}
}

func stubSpecPlanJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specPlanJSON
	specPlanJSON = value

	return func() {
		specPlanJSON = previous
	}
}

func stubSpecPlanScopeID(t *testing.T, value string) func() {
	t.Helper()

	previous := specPlanScopeID
	specPlanScopeID = value

	return func() {
		specPlanScopeID = previous
	}
}

func stubSpecPlanExit(t *testing.T) *int {
	t.Helper()

	exitCode := new(int)
	previous := specPlanExit
	specPlanExit = func(code int) {
		*exitCode = code
	}
	t.Cleanup(func() {
		specPlanExit = previous
	})

	return exitCode
}

func stubSpecPlanAccept(t *testing.T, value string) func() {
	t.Helper()

	previous := specPlanAcceptID
	specPlanAcceptID = value

	return func() {
		specPlanAcceptID = previous
	}
}
