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
	writeBaselineAuditFixture(t, root, "internal/artifact/query.go", strings.Join([]string{
		"// Drift reports for active decisions whose baselined affected_files",
		"// have moved since baseline (H1 of V2 — reality-aware decisions).",
		"// CheckDrift compares baselined affected_files against current state.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/refresh.go", strings.Join([]string{
		"// If projectRoot is non-empty, also checks for file drift on baselined decisions.",
		"noBaseline := 0",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/types.go", strings.Join([]string{
		"// DriftScopeManifest stores the baseline file set for one governed scope.",
		"// DriftStatus represents the state of a file relative to its baseline.",
		"BaselineKindUnknownLegacy BaselineKind = \"unknown_legacy_baseline\"",
		"// additive (new symbols only) — safe to re-baseline without operator review.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/baseline_test.go", strings.Join([]string{
		"func TestCheckDriftReportsNoBaseline(t *testing.T) {}",
		"func TestCheckDriftReportsNoBaseline_LikelyImplementedFalseWithoutGit(t *testing.T) {}",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/decision_measure_test.go", strings.Join([]string{
		"func TestMeasureDoesNotRewriteDecisionBaselineHashes(t *testing.T) {}",
		"SelectedTitle: \"Keep drift baseline stable\"",
		"t.Fatalf(\"drift reports = %+v, want one report from unchanged baseline snapshot\", reports)",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/verification_test.go", strings.Join([]string{
		"if len(result.Baseline) != 2 {",
		"for _, file := range result.Baseline {",
		"// Missing files are now skipped gracefully in Baseline.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/codebase/symhash.go", strings.Join([]string{
		"// SymbolDrift describes how a single symbol changed between baseline and current.",
		"// CompareSymbolSnapshots compares baseline snapshots against current state.",
		"func CompareSymbolSnapshots(baseline []SymbolSnapshot, current []SymbolSnapshot) []SymbolDrift {",
		"// Check for new symbols not in baseline",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/run.go", strings.Join([]string{
		"Short: \"Implement a decision - plan, execute, verify, baseline\",",
		"ev.Phase(\"Baseline\")",
		"ev.OK(fmt.Sprintf(\"%d file(s) snapshotted\", len(baselined)))",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "docs/compare.md", "The benchmark baseline must stay stable.\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/tree_drilldown_test.go", strings.Join([]string{
		"baselineResults, err := SearchSpecWithOptions(db, query, opts)",
		"func TestSearchSpec_TreeModeGoldenQueriesBeatBaselineOnFullCorpus(t *testing.T) {}",
		"baseline := goldenRetrievalMetrics{Name: \"deterministic\", Total: len(cases)}",
		"baseline.Successful++",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/parity_schema.go", "Variant IDs that share comparable baseline conditions (e.g., same cohort, same dataset version).\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/value_space.go", strings.Join([]string{
		"Trigger: \"value_claim_is_made_without_equal_budget_baseline_or_explicit_abstain\"",
		"EvidenceRule: \"compare against declared baseline under parity or label the value claim unavailable for the window\"",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/solution_test.go", "func TestCompare_ErrorsOnScoredVariantOutsideStructuredBaseline(t *testing.T) {}\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/umbrella_triggers.json", "\"recover_to\": \"better on WHICH dimension, versus what baseline\"\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/patterns/compare.md", "Parity keeps same budget, same assumptions, same scope, same evidence standards, and same baseline.\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/solution.go", "BaselineSet: []string{\"Redis\", \"NATS\"},\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_projection_test.go", strings.Join([]string{
		"CounterArgument: \"Tooling and local debugging remain weaker than the simpler HTTP baseline.\"",
		"CounterArgument: \"Tooling and local debugging remain weaker than the simpler HTTP baseline.\"",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "docs/fixture.md", "The baseline fixture is local test data.\n")
	writeBaselineAuditFixture(t, root, "docs/ambiguous.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "README.md", strings.Join([]string{
		"| `haft_decision` | Decision contracts: invariants, claims, evidence, baseline lifecycle |",
		"| **h-verify** | auto | Baseline \u2192 measure \u2192 evidence loop with drift detection |",
		"it takes a baseline snapshot on completion.",
		"reads decision predictions + valid_until + baseline file hashes",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "AGENTS.md", "Drift on touched files: re-baseline via `haft_decision(action=\"baseline\", ...)`.\n")
	writeBaselineAuditFixture(t, root, "CLAUDE.md", "Drift on touched files: re-baseline via `haft_decision(action=\"baseline\", ...)`.\n")
	writeBaselineAuditFixture(t, root, "MIGRATION-v8.md", "Decisions, problems, evidence, baselines, WorkCommissions all still load.\n")
	writeBaselineAuditFixture(t, root, "packages/haft-pi/prompts/h-verify.md", strings.Join([]string{
		"6. Drift on touched files: re-baseline via",
		"`haft_decision(action=\"baseline\")` or surface the drift inline.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "spec/integration/MCP_PROTOCOL.md", "| `haft_decision` | decide, apply, measure, evidence, baseline | Execute, Verify |\n")
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
		"WeakestLink: \"Baseline and drift detection must both agree on the governed file.\"",
		"if err := store.Put(baseline); err != nil {",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/golden_e2e_test.go", "if err := store.Put(baseline); err != nil {\n")
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
	writeBaselineAuditFixture(t, root, "internal/artifact/maintenance_plan.go", strings.Join([]string{
		"// baselines, and config gating live in the shell (internal/cli) — this file",
		"// re-baseline — the undo payload of the maintenance ledger.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/maintenance_plan_test.go", strings.Join([]string{
		"if !maintenanceReviewTestContains(materialDrift.SuggestedCommands, `haft_decision(action=\"baseline\"`) {",
		"t.Fatalf(\"material drift suggested commands missing baseline candidate: %#v\", materialDrift.SuggestedCommands)",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/maintenance_drain.go", strings.Join([]string{
		"cfg.MaintenanceRebaseline = overseer.MaintenanceModePropose",
		"cfg.MaintenanceRebaseline = overseer.MaintenanceModeAuto",
		"Review needs_operator groups before any baseline, waive, reopen, or supersede.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/overseer.go", strings.Join([]string{
		"decides, commissions, rebaselines, or contributes directly to R_eff.",
		"Restored prior baseline for %s.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/codeintel_doctrine.go", "MUTATED governance state on its own (re-baselined drift the conservative gate\n")
	writeBaselineAuditFixture(t, root, "internal/overseer/config.go", strings.Join([]string{
		"// MaintenanceRebaseline gates re-baselining decisions whose drift the",
		"MaintenanceRebaseline string `json:\"maintenance_rebaseline\" yaml:\"maintenance_rebaseline\"`",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/overseer/maintenance.go", "Action: \"suppressed_auto_baseline_candidate\"\n")
	writeBaselineAuditFixture(t, root, "internal/overseer/storage_test.go", "Title: \"Autonomous additive rebaseline\"\n")
	writeBaselineAuditFixture(t, root, "internal/overseer/risk.go", strings.Join([]string{
		"\"rebaseline\",",
		"\"rebaseline authority\",",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/overseer/runner.go", "\"Do not approve, merge, deploy, decide, commission, or rebaseline.\"\n")
	writeBaselineAuditFixture(t, root, "internal/method/builtin.go", strings.Join([]string{
		"Intent: \"A refactor should preserve public behavior; baseline and post-change checks make that claim concrete.\"",
		"ID: \"baseline_and_post_refactor_checks_recorded\"",
		"RequiredEvidence: []string{\"baseline_test_output\", \"post_change_test_output\"},",
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
	writeBaselineAuditFixture(t, root, "internal/project/specflow/baseline_test.go", strings.Join([]string{
		"if !errors.Is(err, ErrBaselineNotFound) {",
		"if err := store.Put(baseline); err != nil {",
		"kind: BaselineKindUnknownLegacy,",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/projection.go", strings.Join([]string{
		"// `haft spec status`, MCP, and agent skills. Mutations such as rebaseline and",
		"if findingsOnlyContainCode(intent.BlockingFindings, codeSpecSectionNeedsBaseline) {",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/projection_test.go", strings.Join([]string{
		"func TestProjectLifecycleActiveSectionWithoutBaselineRequiresApprove(t *testing.T) {}",
		"func TestProjectLifecycleMixedStructuralAndBaselineFindingsRequiresClarify(t *testing.T) {}",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/staleness.go", "NextAction: fmt.Sprintf(\"triage staleness on %q: rebaseline if the claim is still current\", section.ID)\n")
	writeBaselineAuditFixture(t, root, "internal/contextgraph/fetch.go", "No line-range row covered this line (symbol not symbol-baselined, or unknown).\n")
	writeBaselineAuditFixture(t, root, "internal/graphrank/graphrank.go", "Dangling mass teleports to s; baseline teleport is restart*s.\n")
	writeBaselineAuditFixture(t, root, "internal/reff/reff_test.go", strings.Join([]string{
		"baseline := ScoreTypedEvidence(\"explicit_measure\", \"supports\", 3, validUntil, now)",
		"t.Fatalf(\"baseline ScoreTypedEvidence = %v, want 0.8\", baseline)",
		"if baseline != 0.8 {",
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
	writeBaselineAuditFixture(t, root, "internal/cli/spec_onboard_test.go", `t.Fatalf("expected missing-baseline finding for SQL edition: %#v", intent)`+"\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/spec_section_schema.go", strings.Join([]string{
		"`rebaseline` overwrites a baseline after the operator confirms drift.",
		"Lifecycle and mutation JSON expose baseline_kind/profile.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/legacy_binding.go", strings.Join([]string{
		"LegacyBindingPostureMissingSymbolBaseline = \"missing_symbol_baseline\"",
		"LegacyBindingActionProposeRebaseline = \"propose_rebaseline_with_binding_targets\"",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/drift_bindings_test.go", strings.Join([]string{
		"Posture: artifact.LegacyBindingPostureMissingSymbolBaseline,",
		"RecommendedAction: artifact.LegacyBindingActionProposeRebaseline,",
		"\"action=propose_rebaseline_with_binding_targets\",",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/binding_surface_inventory.go", strings.Join([]string{
		"{Tool: \"haft_decision\", Action: \"baseline\", Class: bindingSurfaceEvidenceRecording}",
		"Action: \"rebaseline\",",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/binding_surface_inventory_test.go", "{tool: \"haft_spec_section\", action: \"rebaseline\", class: bindingSurfaceLifecycleAuthorityMutation}\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/drift_events.go", strings.Join([]string{
		"DriftEventResolutionNeedsRebaseline = \"needs_rebaseline\"",
		"return DriftEventResolutionNeedsRebaseline",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/maintenance_review.go", strings.Join([]string{
		"suggestedAction: \"if benign, approve rebaseline; if material, reopen or supersede the decision\"",
		"fmt.Sprintf(`haft_decision(action=\"baseline\", decision_ref=\"%s\") # only after operator approves drift as benign`, task.DecisionRef)",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/reconciliation.go", strings.Join([]string{
		"\"enrich_scope does not change decision status, lineage, evidence, baselines, or gates\"",
		"\"does not create evidence, baselines, gates, or admissions\"",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/reconciliation_metrics.go", "metrics do not approve, supersede, retire, enrich, waive, or rebaseline decisions\n")
	writeBaselineAuditFixture(t, root, "internal/cli/drift_route.go", strings.Join([]string{
		"does not mutate code, carriers, evidence, decisions, baselines, or gates.",
		"This is a read-only projection: it does not mutate decisions, baselines,",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/decision_reconcile.go", strings.Join([]string{
		"baselines, or carriers.",
		"This command does not mutate decisions, links, evidence, baselines, or carriers.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec_review.go", strings.Join([]string{
		"rebaseline, reopen, create evidence, create decisions, or act as",
		"authority: advisory_only; not evidence, approval, rebaseline, GateDecision, or SpecUseAdmission",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec_apply_change.go", "AuthorityBoundary: \"not_approval_not_rebaseline_not_evidence\"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/spec_sync.go", "AuthorityBoundary: \"not_approval_not_rebaseline_not_evidence\"\n")
	writeBaselineAuditFixture(t, root, "internal/tools/haft.go", strings.Join([]string{
		"- baseline: Snapshot affected files after implementation and before measurement.",
		`"action": map[string]any{"type": "string", "enum": []string{"decide", "evidence", "baseline", "measure"}}`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/server.go", strings.Join([]string{
		`"enum": []interface{}{"decide", "apply", "measure", "evidence", "baseline"},`,
		`"description": "decide=DRR creation, apply=brief, measure=impact, evidence=attach, baseline=snapshot files",`,
		`"description": "(decide/baseline) Affected files.",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/tools/haft_test.go", strings.Join([]string{
		`baselineResult, err := tool.Execute(fixture.ctx, mustJSON(t, map[string]any{`,
		`"action": "baseline",`,
		`t.Fatalf("baseline output should pair decision title with ref:\n%s", baselineResult.DisplayText)`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_decision_test.go", strings.Join([]string{
		"func TestHandleQuintDecision_BaselineRequiresRef(t *testing.T) {}",
		`"action": "baseline",`,
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_parity_test.go", strings.Join([]string{
		"mcpActions: []string{\"decide\", \"apply\", \"measure\", \"evidence\", \"baseline\"},",
		"standaloneActions: []string{\"decide\", \"evidence\", \"baseline\", \"measure\"},",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/fpf/fpf-routes.json", "\"baseline\",\n")
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
	writeBaselineAuditFixture(t, root, "internal/present/format_test.go", strings.Join([]string{
		`t.Fatalf("baseline response should pair decision title with ref:\n%s", response)`,
		`t.Fatalf("baseline response should not expose a bare decision ref:\n%s", response)`,
		"CounterArgument: \"Tooling and local debugging remain weaker than the simpler HTTP baseline.\"",
		"func TestDriftResponseSummary_EmptyAndNoBaseline(t *testing.T) {}",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-spec/SKILL.md", strings.Join([]string{
		"Primary spec lifecycle interface for a haft project.",
		"`rebaseline` and `reopen` are mutation commands.",
		"baseline state should change.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-onboard/SKILL.md", strings.Join([]string{
		"approve, rebaseline, or reopen requests should route to h-spec.",
		"the operator to choose rebaseline, reopen, rollback, deprecate, or supersede.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-commission/SKILL.md", "- Drift on affected_files since baseline (drifted -> flag)\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-decide/SKILL.md", "`mcp__haft__haft_decision(action=\"baseline\", decision_ref=\"dec-...\")` - snapshot affected files for drift detection\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-frame/SKILL.md", "Good signal: \"webhook retries hit 15% over baseline 2% since 2026-05-20\"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-note/SKILL.md", "Accept: \"Observation: tests run 30s slower on M1 baseline since dependency update\"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-reason/SKILL.md", strings.Join([]string{
		"re-baseline via `haft_decision(action=\"baseline\", ...)` or surface drift inline.",
		"Use when the operator asks to approve, rebaseline, or reopen specs.",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/skill/h-spec-cover/SKILL.md", strings.Join([]string{
		"approve / rebaseline / reopen for a spec section",
		"Recommend `/h-verify` to baseline+measure the existing decisions.",
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
	writeBaselineAuditFixture(t, root, "internal/agent/cycle.go", "// baseline/measure -> same active decision chain\n")
	writeBaselineAuditFixture(t, root, "internal/agent/prompt.go", "- Execute: decision record with claims, implementation, baseline\n")
	writeBaselineAuditFixture(t, root, "internal/cli/testdata/routing-prompts.yaml", "- prompt: \"the spec section is stale, should we rebaseline or reopen it\"\n")
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
	if report.Summary.SpecSectionApprovalBaseline != 13 {
		t.Fatalf("spec approval count = %d, want 13", report.Summary.SpecSectionApprovalBaseline)
	}
	if report.Summary.VerifiedStateSnapshot != 43 {
		t.Fatalf("verified state count = %d, want 43", report.Summary.VerifiedStateSnapshot)
	}
	if report.Summary.ComparisonOrBenchmark != 15 {
		t.Fatalf("comparison count = %d, want 15", report.Summary.ComparisonOrBenchmark)
	}
	if report.Summary.OrdinaryLanguageBaseline != 5 {
		t.Fatalf("ordinary count = %d, want 5", report.Summary.OrdinaryLanguageBaseline)
	}
	if report.Summary.HistoricalGovernanceCarrier != 1 {
		t.Fatalf("historical governance count = %d, want 1", report.Summary.HistoricalGovernanceCarrier)
	}
	if report.Summary.HistoricalGovernanceFiles != 1 {
		t.Fatalf("historical governance files = %d, want 1", report.Summary.HistoricalGovernanceFiles)
	}
	if report.Summary.SupportArchiveCarrier != 2 {
		t.Fatalf("support/archive count = %d, want 2", report.Summary.SupportArchiveCarrier)
	}
	if report.Summary.SupportArchiveFiles != 2 {
		t.Fatalf("support/archive files = %d, want 2", report.Summary.SupportArchiveFiles)
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
	if report.Summary.TypedBaselineModel != 3 {
		t.Fatalf("typed baseline model count = %d, want 3", report.Summary.TypedBaselineModel)
	}
	if report.Summary.TypedBaselineModelFiles != 3 {
		t.Fatalf("typed baseline model files = %d, want 3", report.Summary.TypedBaselineModelFiles)
	}
	if report.Summary.LifecycleAuthority != 48 {
		t.Fatalf("lifecycle authority count = %d, want 48", report.Summary.LifecycleAuthority)
	}
	if report.Summary.LifecycleAuthorityFiles != 30 {
		t.Fatalf("lifecycle authority files = %d, want 30", report.Summary.LifecycleAuthorityFiles)
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
	if report.Summary.TestFixtureSurface != 10 {
		t.Fatalf("test fixture surface count = %d, want 10", report.Summary.TestFixtureSurface)
	}
	if report.Summary.TestFixtureSurfaceFiles != 5 {
		t.Fatalf("test fixture surface files = %d, want 5", report.Summary.TestFixtureSurfaceFiles)
	}
	if report.Summary.AutonomousMaintenance != 20 {
		t.Fatalf("autonomous maintenance count = %d, want 20", report.Summary.AutonomousMaintenance)
	}
	if report.Summary.AutonomousMaintenanceFiles != 12 {
		t.Fatalf("autonomous maintenance files = %d, want 12", report.Summary.AutonomousMaintenanceFiles)
	}
	if report.Summary.MethodPackSurface != 3 {
		t.Fatalf("method pack count = %d, want 3", report.Summary.MethodPackSurface)
	}
	if report.Summary.MethodPackSurfaceFiles != 1 {
		t.Fatalf("method pack files = %d, want 1", report.Summary.MethodPackSurfaceFiles)
	}
	if report.Summary.SpecUseCurrentness != 2 {
		t.Fatalf("spec use currentness count = %d, want 2", report.Summary.SpecUseCurrentness)
	}
	if report.Summary.SpecUseCurrentnessFiles != 1 {
		t.Fatalf("spec use currentness files = %d, want 1", report.Summary.SpecUseCurrentnessFiles)
	}
	if report.Summary.LegacyBindingScope != 5 {
		t.Fatalf("legacy binding scope count = %d, want 5", report.Summary.LegacyBindingScope)
	}
	if report.Summary.LegacyBindingScopeFiles != 2 {
		t.Fatalf("legacy binding scope files = %d, want 2", report.Summary.LegacyBindingScopeFiles)
	}
	if report.Summary.DecisionBaselineAPI != 16 {
		t.Fatalf("decision baseline api count = %d, want 16", report.Summary.DecisionBaselineAPI)
	}
	if report.Summary.DecisionBaselineAPIFiles != 9 {
		t.Fatalf("decision baseline api files = %d, want 9", report.Summary.DecisionBaselineAPIFiles)
	}
	if report.Summary.BaselinePresentation != 8 {
		t.Fatalf("baseline presentation count = %d, want 8", report.Summary.BaselinePresentation)
	}
	if report.Summary.BaselinePresentationFiles != 2 {
		t.Fatalf("baseline presentation files = %d, want 2", report.Summary.BaselinePresentationFiles)
	}
	if report.Summary.InterfaceContractBaseline != 2 {
		t.Fatalf("interface contract count = %d, want 2", report.Summary.InterfaceContractBaseline)
	}
	if report.Summary.InterfaceContractFiles != 1 {
		t.Fatalf("interface contract files = %d, want 1", report.Summary.InterfaceContractFiles)
	}
	if report.Summary.LegacyAmbiguousBaseline != 1 {
		t.Fatalf("legacy ambiguous count = %d, want 1", report.Summary.LegacyAmbiguousBaseline)
	}
	if report.Summary.LegacyAmbiguousFiles != 1 {
		t.Fatalf("legacy ambiguous files = %d, want 1", report.Summary.LegacyAmbiguousFiles)
	}
	if len(report.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want one legacy ambiguous diagnostic", report.Diagnostics)
	}
	diagnostic := report.Diagnostics[0]
	if diagnostic.Code != "legacy_ambiguous_baseline_terms" {
		t.Fatalf("diagnostic code = %q", diagnostic.Code)
	}
	if diagnostic.Count != 1 || diagnostic.Files != 1 {
		t.Fatalf("diagnostic count/files = %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.NextAction, "typed baseline concept") {
		t.Fatalf("diagnostic next_action = %q", diagnostic.NextAction)
	}
	if len(diagnostic.Examples) != 1 || !strings.Contains(diagnostic.Examples[0], "docs/ambiguous.md") {
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
			MethodPackSurface:           3,
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
		"method_pack=3",
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

func TestClassifyBaselineTermTreatsNotRebaselineBoundaryAsLifecycleAuthority(t *testing.T) {
	category, rationale := classifyBaselineTerm(
		"internal/cli/spec_apply_change_test.go",
		`AuthorityBoundary: "not_approval_not_rebaseline_not_evidence"`,
	)

	if category != baselineAuditLifecycleAuth {
		t.Fatalf("category = %q, want %q (%s)", category, baselineAuditLifecycleAuth, rationale)
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
