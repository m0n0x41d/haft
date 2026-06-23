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
	writeBaselineAuditFixture(t, root, "internal/cli/serve_spec_section.go", "ctx.store.PutSpecSectionApproval(baseline)\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/decision.go", strings.Join([]string{
		"const baselineProfile = \"verified_state_snapshot\"",
		"type BaselineInput struct{}",
	}, "\n")+"\n")
	writeBaselineAuditFixture(t, root, "docs/compare.md", "The benchmark baseline must stay stable.\n")
	writeBaselineAuditFixture(t, root, "docs/fixture.md", "The baseline fixture is local test data.\n")
	writeBaselineAuditFixture(t, root, "docs/ambiguous.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, ".haft/decisions/dec.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, ".haft/methods/swe-core/refactor.yaml", "baseline and post-change checks make the claim concrete.\n")
	writeBaselineAuditFixture(t, root, "data/FPF/FPF-Spec.md", "Baseline is source-spec terminology.\n")
	writeBaselineAuditFixture(t, root, "internal/project/specflow/baseline_model.go", "type BaselineKind string\n")
	writeBaselineAuditFixture(t, root, "docs/lifecycle.md", "Operators approve/rebaseline active baseline records.\n")
	writeBaselineAuditFixture(t, root, "CHANGELOG.md", "Baseline audit release note.\n")
	writeBaselineAuditFixture(t, root, "internal/cli/baseline_audit.go", "const baselineAuditKind = \"haft_baseline_term_audit\"\n")
	writeBaselineAuditFixture(t, root, "internal/cli/serve_spec_section_test.go", "func newBaselineTestProject() {}\n")
	writeBaselineAuditFixture(t, root, ".claude/worktrees/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "open-sleigh/.haft/decisions/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "node_modules/pkg/ignored.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, "tui/node_modules/pkg/ignored.md", "Run baseline before release.\n")

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
	if report.Summary.SpecSectionApprovalBaseline != 3 {
		t.Fatalf("spec approval count = %d, want 3", report.Summary.SpecSectionApprovalBaseline)
	}
	if report.Summary.VerifiedStateSnapshot != 2 {
		t.Fatalf("verified state count = %d, want 2", report.Summary.VerifiedStateSnapshot)
	}
	if report.Summary.ComparisonOrBenchmark != 1 {
		t.Fatalf("comparison count = %d, want 1", report.Summary.ComparisonOrBenchmark)
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
	if report.Summary.TypedBaselineModel != 1 {
		t.Fatalf("typed baseline model count = %d, want 1", report.Summary.TypedBaselineModel)
	}
	if report.Summary.TypedBaselineModelFiles != 1 {
		t.Fatalf("typed baseline model files = %d, want 1", report.Summary.TypedBaselineModelFiles)
	}
	if report.Summary.LifecycleAuthority != 1 {
		t.Fatalf("lifecycle authority count = %d, want 1", report.Summary.LifecycleAuthority)
	}
	if report.Summary.LifecycleAuthorityFiles != 1 {
		t.Fatalf("lifecycle authority files = %d, want 1", report.Summary.LifecycleAuthorityFiles)
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
	if report.Summary.TestFixtureSurface != 1 {
		t.Fatalf("test fixture surface count = %d, want 1", report.Summary.TestFixtureSurface)
	}
	if report.Summary.TestFixtureSurfaceFiles != 1 {
		t.Fatalf("test fixture surface files = %d, want 1", report.Summary.TestFixtureSurfaceFiles)
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
			SpecSectionApprovalBaseline: 3,
			VerifiedStateSnapshot:       2,
			HistoricalGovernanceCarrier: 1,
			SupportArchiveCarrier:       1,
			SourceSpecReference:         1,
			TypedBaselineModel:          1,
			LifecycleAuthority:          1,
			ReleaseNotesCarrier:         1,
			AuditToolSurface:            1,
			TestFixtureSurface:          1,
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
		"spec_approval=3",
		"verified_state=2",
		"historical_governance=1",
		"support_archive=1",
		"source_spec=1",
		"typed_model=1",
		"lifecycle_authority=1",
		"release_notes=1",
		"audit_tool=1",
		"test_fixture=1",
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
