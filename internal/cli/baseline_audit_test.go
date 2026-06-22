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
	writeBaselineAuditFixture(t, root, "internal/project/specflow/baseline.go", "const profile = \"spec_section_approval_baseline\"\n")
	writeBaselineAuditFixture(t, root, "internal/artifact/decision.go", "const baselineProfile = \"verified_state_snapshot\"\n")
	writeBaselineAuditFixture(t, root, "docs/compare.md", "The benchmark baseline must stay stable.\n")
	writeBaselineAuditFixture(t, root, "docs/fixture.md", "The baseline fixture is local test data.\n")
	writeBaselineAuditFixture(t, root, "docs/ambiguous.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, ".haft/decisions/dec.md", "Run baseline before release.\n")
	writeBaselineAuditFixture(t, root, ".haft/methods/swe-core/refactor.yaml", "baseline and post-change checks make the claim concrete.\n")
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
	if report.Summary.SpecSectionApprovalBaseline != 1 {
		t.Fatalf("spec approval count = %d, want 1", report.Summary.SpecSectionApprovalBaseline)
	}
	if report.Summary.VerifiedStateSnapshot != 1 {
		t.Fatalf("verified state count = %d, want 1", report.Summary.VerifiedStateSnapshot)
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
			VerifiedStateSnapshot:       1,
			HistoricalGovernanceCarrier: 1,
			SupportArchiveCarrier:       1,
			LegacyAmbiguousBaseline:     1,
			LegacyAmbiguousFiles:        1,
		},
		Diagnostics: []baselineTermAuditDiagnostic{{
			Level:      "warn",
			Code:       "legacy_ambiguous_baseline_terms",
			Category:   baselineAuditLegacyAmbiguous,
			Count:      1,
			Files:      1,
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
		"verified_state=1",
		"historical_governance=1",
		"support_archive=1",
		"legacy_ambiguous=1",
		"diagnostic: [warn/legacy_ambiguous_baseline_terms] 1 legacy ambiguous baseline line(s) across 1 file(s)",
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
