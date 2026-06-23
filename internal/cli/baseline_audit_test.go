package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildBaselineTermAuditReportClassifiesAndSkipsNoise(t *testing.T) {
	root := t.TempDir()
	writeBaselineAuditFixture(t, root, "internal/project/specflow/baseline.go", strings.Join([]string{
		"const profile = \"spec_section_approval_baseline\"",
		"baseline.ProjectID = projectID",
		"Object: \"UnknownLegacyBaseline\",",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_spec_section.go", strings.Join([]string{
		"ctx.store.PutSpecSectionApproval(baseline)",
		"type baselineMutation struct{}",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/decision.go", strings.Join([]string{
		"const baselineProfile = \"verified_state_snapshot\"",
		"type BaselineInput struct{}",
		"logger.Debug().Msg(\"baseline.complete\")",
		"func noBaselineMateriality() {}",
		"baselineSymbolsByFile := groupSymbolsByFile(nil)",
		"baselinedFiles := make(map[string]struct{})",
		"`Run haft_decision(action=\"baseline\") first for CL3 scoring.`",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/verification.go", strings.Join([]string{
		"// VerificationPassResult returns the baseline snapshot and linked evidence item.",
		"Baseline []AffectedFile `json:\"baseline\"`",
		"Baseline: baseline,",
		`fmt.Sprintf("Baselined files (%d): %s", len(paths), strings.Join(paths, ", "))`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/run.go", strings.Join([]string{
		"Short: \"Implement a decision - plan, execute, verify, baseline\",",
		"ev.Phase(\"Baseline\")",
		"ev.OK(fmt.Sprintf(\"%d file(s) snapshotted\", len(baselined)))",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "docs/compare.md", "The benchmark baseline must stay stable.\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/tree_drilldown_test.go", "baselineResults, err := SearchSpecWithOptions(db, query, opts)\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/solution.go", "BaselineSet: []string{\"Redis\", \"NATS\"},\n")
	writeBaselineAuditFixture(t, root, "docs/fixture.md", "The baseline fixture is local test data.\n")
	writeBaselineAuditFixture(t, root, "docs/ambiguous.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, ".haft/decisions/dec.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, ".haft/methods/swe-core/refactor.yaml", "baseline and post-change checks make the claim concrete.\n")
	writeBaselineAuditFixture(t, root, "data/FPF/FPF-Spec.md", "Baseline is source-spec terminology.\n")
	writeBaselineAuditFixture(t, root, ".haft/specs/target-system.md", "Method: require explicit frames, decisions, scopes, baselines, and evidence.\n")
	writeBaselineAuditFixture(t, root, "spec/target-system/EXECUTION_CONTRACT.md", "Post-verify pass records a baseline snapshot.\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/baseline_model.go", "type BaselineKind string\n")
	writeBaselineAuditFixture(t, root, "docs/lifecycle.md", "Operators approve/rebaseline active baseline records.\n")
	writeBaselineAuditFixture(t, root, "CHANGELOG.md", "Baseline audit release note.\n")
	writeBaselineAuditFixture(t, root, "internal/cli/baseline_audit.go", "const baselineAuditKind = \"haft_baseline_term_audit\"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_spec_section_test.go", strings.Join([]string{
		"func newBaselineTestProject() {}",
		`"action": "rebaseline",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/check_test.go", strings.Join([]string{
		"t.Fatal(\"baseline_profile missing from drift finding\")",
		"mustBaselineDecision(t, fixture, driftDecision.Meta.ID)",
		"// Active sections need baselines so SpecSection drift detection works.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec.go", strings.Join([]string{
		"Use: \"rebaseline SECTION_ID\",",
		"approve sections, or rebaseline drift.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/maintenance_plan.go", strings.Join([]string{
		"type BaselineSnapshot struct{}",
		"AutoBaselineCandidates++",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/maintenance_exec_test.go", strings.Join([]string{
		"t.Fatal(\"auto mode should have re-baselined the additive drift\")",
		"t.Fatal(\"undo must restore the PRIOR baseline\")",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/use.go", strings.Join([]string{
		"BaselineCurrentness SpecificationUseCurrentness `json:\"baseline_currentness\"`",
		"status = SpecUseBaselineMissing",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/state.go", strings.Join([]string{
		"// SpecSectionBaseline freshness is enforced here.",
		"return DeriveStateWithBaselines(set, baselines, projectID)",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/drift.go", strings.Join([]string{
		"codeSpecSectionNeedsBaseline = \"spec_section_needs_baseline\"",
		"NextAction: fmt.Sprintf(\"haft_spec_section(action=\\\"approve\\\", section_id=%q) to record a baseline\", section.ID),",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec_baseline.go", strings.Join([]string{
		"// appendSpecHealthFindings runs SpecSection drift / missing-baseline checks.",
		"func appendSpecBaselineFindings(report project.SpecCheckReport, projectRoot string) project.SpecCheckReport {",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec_sync_test.go", strings.Join([]string{
		"func TestRunSpecSyncImportsTypedSectionsIntoSQLWithoutBaselines(t *testing.T) {}",
		"t.Fatalf(\"spec sync must not create baselines, got %d\", baselineRows)",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec_lifecycle.go", strings.Join([]string{
		"func runSpecRebaseline(cmd *cobra.Command, args []string) error {",
		`"action": "rebaseline",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/spec_section_schema.go", strings.Join([]string{
		"`rebaseline` overwrites a baseline after the operator confirms drift.",
		"Lifecycle and mutation JSON expose baseline_kind/profile.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/legacy_binding.go", strings.Join([]string{
		"LegacyBindingPostureMissingSymbolBaseline = \"missing_symbol_baseline\"",
		"LegacyBindingActionProposeRebaseline = \"propose_rebaseline_with_binding_targets\"",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/tools/haft.go", strings.Join([]string{
		"- baseline: Snapshot affected files after implementation and before measurement.",
		`"action": map[string]any{"type": "string", "enum": []string{"decide", "evidence", "baseline", "measure"}}`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_decision_test.go", strings.Join([]string{
		"func TestHandleQuintDecision_BaselineRequiresRef(t *testing.T) {}",
		`"action": "baseline",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/interface.go", strings.Join([]string{
		`Shape: ` + "`" + `{"source_edition":{...},"baseline_currentness":{...},"admission":{...}}` + "`" + `,`,
		`Note: "Read-only: it does not supersede, merge, retire, reopen, baseline, or create GateDecision records.",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/present/format.go", strings.Join([]string{
		"func BaselineResponse(decisionTitle string, decisionRef string, files []artifact.AffectedFile, navStrip string) string {",
		`sb.WriteString("No drift detected. All baselined decisions match current file state.\n")`,
		"noBaselineCount := 0",
		"return \" — additive only; safe to re-baseline without review\"",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-spec/SKILL.md", strings.Join([]string{
		"Primary spec lifecycle interface for a haft project.",
		"`rebaseline` and `reopen` are mutation commands.",
		"baseline state should change.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-verify/SKILL.md", strings.Join([]string{
		"baseline-vs-measure evidence loop with drift detection.",
		"## Step 3 — Baseline (if drift detection wanted)",
		"Do NOT skip baseline if the decision has affected_files.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/agent/guardrails.go", strings.Join([]string{
		"Guidance: \"This cycle already has a recorded decision. Run baseline/measure on that decision.\"",
		"// CanBaseline checks if haft_decision(baseline) is allowed.",
		`Tool: "haft_decision(baseline)",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, ".claude/worktrees/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "open-sleigh/.haft/decisions/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "node_modules/pkg/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "tui/node_modules/pkg/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "package-lock.json", `"baseline-browser-mapping": "dist/cli.cjs"`)

	report, err := buildBaselineTermAuditReport(root)
	if err != nil {
		t.Fatalf("buildBaselineTermAuditReport returned error: %v", err)
	}

	if report.Kind != baselineAuditKind {
		t.Fatalf("unexpected kind: %q", report.Kind)
	}
	if report.Authority != baselineAuditAuthority {
		t.Fatalf("unexpected authority: %q", report.Authority)
	}
	if report.Summary.SpecSectionApprovalBaseline != 11 {
		t.Fatalf("spec approval count = %d, want 11", report.Summary.SpecSectionApprovalBaseline)
	}
	if report.Summary.VerifiedStateSnapshot != 17 {
		t.Fatalf("verified state count = %d, want 17", report.Summary.VerifiedStateSnapshot)
	}
	if report.Summary.ComparisonOrBenchmark != 3 {
		t.Fatalf("comparison count = %d, want 3", report.Summary.ComparisonOrBenchmark)
	}
	if report.Summary.OrdinaryLanguageBaseline != 1 {
		t.Fatalf("ordinary count = %d, want 1", report.Summary.OrdinaryLanguageBaseline)
	}
	if report.Summary.HistoricalGovernanceCarrier != 1 {
		t.Fatalf("historical governance count = %d, want 1", report.Summary.HistoricalGovernanceCarrier)
	}
	if report.Summary.HistoricalGovernanceFiles != 1 {
		t.Fatalf("historical governance files = %d, want 1", report.Summary.HistoricalGovernanceFiles)
	}
	if report.Summary.SupportArchiveCarrier != 1 {
		t.Fatalf("support/archive count = %d, want 1", report.Summary.SupportArchiveCarrier)
	}
	if report.Summary.SupportArchiveFiles != 1 {
		t.Fatalf("support/archive files = %d, want 1", report.Summary.SupportArchiveFiles)
	}
	if report.Summary.SourceSpecReference != 1 {
		t.Fatalf("source spec count = %d, want 1", report.Summary.SourceSpecReference)
	}
	if report.Summary.SourceSpecFiles != 1 {
		t.Fatalf("source spec files = %d, want 1", report.Summary.SourceSpecFiles)
	}
	if report.Summary.ProjectSpecCarrier != 2 {
		t.Fatalf("project spec count = %d, want 2", report.Summary.ProjectSpecCarrier)
	}
	if report.Summary.ProjectSpecFiles != 2 {
		t.Fatalf("project spec files = %d, want 2", report.Summary.ProjectSpecFiles)
	}
	if report.Summary.TypedBaselineModel != 1 {
		t.Fatalf("typed baseline model count = %d, want 1", report.Summary.TypedBaselineModel)
	}
	if report.Summary.TypedBaselineModelFiles != 1 {
		t.Fatalf("typed baseline model files = %d, want 1", report.Summary.TypedBaselineModelFiles)
	}
	if report.Summary.LifecycleAuthority != 14 {
		t.Fatalf("lifecycle authority count = %d, want 14", report.Summary.LifecycleAuthority)
	}
	if report.Summary.LifecycleAuthorityFiles != 8 {
		t.Fatalf("lifecycle authority files = %d, want 8", report.Summary.LifecycleAuthorityFiles)
	}
	if report.Summary.ReleaseNotesCarrier != 1 {
		t.Fatalf("release notes count = %d, want 1", report.Summary.ReleaseNotesCarrier)
	}
	if report.Summary.ReleaseNotesFiles != 1 {
		t.Fatalf("release notes files = %d, want 1", report.Summary.ReleaseNotesFiles)
	}
	if report.Summary.AuditToolSurface != 1 {
		t.Fatalf("audit tool surface count = %d, want 1", report.Summary.AuditToolSurface)
	}
	if report.Summary.AuditToolSurfaceFiles != 1 {
		t.Fatalf("audit tool surface files = %d, want 1", report.Summary.AuditToolSurfaceFiles)
	}
	if report.Summary.TestFixtureSurface != 4 {
		t.Fatalf("test fixture surface count = %d, want 4", report.Summary.TestFixtureSurface)
	}
	if report.Summary.TestFixtureSurfaceFiles != 2 {
		t.Fatalf("test fixture surface files = %d, want 2", report.Summary.TestFixtureSurfaceFiles)
	}
	if report.Summary.AutonomousMaintenance != 4 {
		t.Fatalf("autonomous maintenance count = %d, want 4", report.Summary.AutonomousMaintenance)
	}
	if report.Summary.AutonomousMaintenanceFiles != 2 {
		t.Fatalf("autonomous maintenance files = %d, want 2", report.Summary.AutonomousMaintenanceFiles)
	}
	if report.Summary.SpecUseCurrentness != 2 {
		t.Fatalf("spec use currentness count = %d, want 2", report.Summary.SpecUseCurrentness)
	}
	if report.Summary.SpecUseCurrentnessFiles != 1 {
		t.Fatalf("spec use currentness files = %d, want 1", report.Summary.SpecUseCurrentnessFiles)
	}
	if report.Summary.LegacyBindingScope != 2 {
		t.Fatalf("legacy binding scope count = %d, want 2", report.Summary.LegacyBindingScope)
	}
	if report.Summary.LegacyBindingScopeFiles != 1 {
		t.Fatalf("legacy binding scope files = %d, want 1", report.Summary.LegacyBindingScopeFiles)
	}
	if report.Summary.DecisionBaselineAPI != 4 {
		t.Fatalf("decision baseline api count = %d, want 4", report.Summary.DecisionBaselineAPI)
	}
	if report.Summary.DecisionBaselineAPIFiles != 2 {
		t.Fatalf("decision baseline api files = %d, want 2", report.Summary.DecisionBaselineAPIFiles)
	}
	if report.Summary.BaselinePresentation != 4 {
		t.Fatalf("baseline presentation count = %d, want 4", report.Summary.BaselinePresentation)
	}
	if report.Summary.BaselinePresentationFiles != 1 {
		t.Fatalf("baseline presentation files = %d, want 1", report.Summary.BaselinePresentationFiles)
	}
	if report.Summary.InterfaceContractBaseline != 2 {
		t.Fatalf("interface contract count = %d, want 2", report.Summary.InterfaceContractBaseline)
	}
	if report.Summary.InterfaceContractFiles != 1 {
		t.Fatalf("interface contract files = %d, want 1", report.Summary.InterfaceContractFiles)
	}
	if report.Summary.LegacyAmbiguousBaseline != 2 {
		t.Fatalf("legacy ambiguous count = %d, want 2", report.Summary.LegacyAmbiguousBaseline)
	}
	if report.Summary.LegacyAmbiguousFiles != 2 {
		t.Fatalf("legacy ambiguous files = %d, want 2", report.Summary.LegacyAmbiguousFiles)
	}
	if len(report.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want one legacy ambiguous diagnostic", report.Diagnostics)
	}
	diagnostic := report.Diagnostics[0]
	if diagnostic.Code != "legacy_ambiguous_baseline_terms" {
		t.Fatalf("diagnostic code = %q", diagnostic.Code)
	}
	if diagnostic.Count != 2 || diagnostic.Files != 2 {
		t.Fatalf("diagnostic count/files = %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.NextAction, "typed baseline concept") {
		t.Fatalf("diagnostic next_action = %q", diagnostic.NextAction)
	}
	if len(diagnostic.Examples) != 2 || !strings.Contains(diagnostic.Examples[0], "docs/ambiguous.md") {
		t.Fatalf("diagnostic examples = %#v", diagnostic.Examples)
	}

	for _, finding := range report.Findings {
		if strings.Contains(finding.Path, "open-sleigh") {
			t.Fatalf("open-sleigh path was not skipped: %+v", finding)
		}
		if strings.Contains(finding.Path, "node_modules") {
			t.Fatalf("node_modules path was not skipped: %+v", finding)
		}
		if strings.Contains(finding.Path, ".claude") {
			t.Fatalf(".claude path was not skipped: %+v", finding)
		}
		if strings.Contains(finding.Path, "package-lock.json") {
			t.Fatalf("package-lock path was not skipped: %+v", finding)
		}
	}
}

func TestWriteBaselineAuditText(t *testing.T) {
	var output bytes.Buffer
	report := baselineTermAuditReport{
		SchemaVersion: 1,
		Authority:     baselineAuditAuthority,
		Summary: baselineTermAuditSummary{
			FilesScanned:                2,
			MatchedLines:                2,
			SpecSectionApprovalBaseline: 7,
			VerifiedStateSnapshot:       14,
			ComparisonOrBenchmark:       3,
			HistoricalGovernanceCarrier: 1,
			SupportArchiveCarrier:       1,
			SourceSpecReference:         1,
			ProjectSpecCarrier:          2,
			TypedBaselineModel:          1,
			LifecycleAuthority:          5,
			ReleaseNotesCarrier:         1,
			AuditToolSurface:            1,
			TestFixtureSurface:          4,
			AutonomousMaintenance:       4,
			SpecUseCurrentness:          2,
			LegacyBindingScope:          2,
			DecisionBaselineAPI:         4,
			BaselinePresentation:        2,
			InterfaceContractBaseline:   2,
			LegacyAmbiguousBaseline:     2,
			LegacyAmbiguousFiles:        2,
		},
		Diagnostics: []baselineTermAuditDiagnostic{{
			Level:      "warn",
			Code:       "legacy_ambiguous_baseline_terms",
			Category:   baselineAuditLegacyAmbiguous,
			Count:      2,
			Files:      2,
			NextAction: "rename the usage to a typed baseline concept",
		}},
		Findings: []baselineTermAuditFinding{{
			Path:     ".haft/decisions/dec.md",
			Line:     12,
			Category: baselineAuditLegacyAmbiguous,
			Snippet:  "Run baseline before release.",
		}},
	}

	if err := writeBaselineAuditText(&output, report); err != nil {
		t.Fatalf("writeBaselineAuditText returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Haft baseline term audit v1",
		"authority: read_only_term_audit_not_baseline_mutation",
		"spec_approval=7",
		"verified_state=14",
		"comparison=3",
		"historical_governance=1",
		"support_archive=1",
		"source_spec=1",
		"project_spec=2",
		"typed_model=1",
		"lifecycle_authority=5",
		"release_notes=1",
		"audit_tool=1",
		"test_fixture=4",
		"autonomous_maintenance=4",
		"spec_use_currentness=2",
		"legacy_binding_scope=2",
		"decision_baseline_api=4",
		"baseline_presentation=2",
		"interface_contract=2",
		"legacy_ambiguous=2",
		"diagnostic: [warn/legacy_ambiguous_baseline_terms] 2 legacy ambiguous baseline line(s) across 2 file(s)",
		"next_action: rename the usage to a typed baseline concept",
		".haft/decisions/dec.md:12 Run baseline before release.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("audit text missing %q:\n%s", want, text)
		}
	}
}

func writeBaselineAuditFixture(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
}
