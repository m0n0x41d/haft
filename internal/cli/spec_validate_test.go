package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func TestRunSpecValidateReviewsTenDraftSectionsWithoutApplicabilityAdmission(
	t *testing.T,
) {
	root := newDraftSpecValidationProject(t)
	restoreRoot := enterTestProjectRoot(t, root)
	defer restoreRoot()
	restoreJSON := stubSpecValidateJSON(t, true)
	defer restoreJSON()
	restoreExit := stubSpecValidateExit(t)
	defer restoreExit()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecValidate(cmd, nil); err != nil {
		t.Fatalf("runSpecValidate returned error: %v", err)
	}

	var report specflow.CarrierValidationReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode validation report: %v\n%s", err, output.String())
	}
	if report.Summary.TotalSections != 10 || report.Summary.DraftSections != 10 || report.Summary.CheckedSections != 10 {
		t.Fatalf("validation did not inspect all draft sections: %+v", report.Summary)
	}
	if report.Summary.ActiveSections != 0 {
		t.Fatalf("validation changed lifecycle reading: %+v", report.Summary)
	}
	if report.Summary.LifecycleObservations != 2 || report.Summary.StructuralFindings != 0 {
		t.Fatalf("validation did not separate lifecycle observations: %+v", report.Summary)
	}
	targetSections := 0
	for _, section := range report.Semantic.Sections {
		if section.DocumentKind == "target-system" {
			targetSections++
		}
		if section.Status != "draft" {
			t.Fatalf("validation changed draft status: %+v", section)
		}
	}
	if targetSections != 3 {
		t.Fatalf("target-system sections checked = %d, want 3", targetSections)
	}
	if report.AuthorityBoundary.Applicability != "not_applicability_determination_or_admission" ||
		report.AuthorityBoundary.Approval != "not_approval_or_baseline" {
		t.Fatalf("validation authority boundary = %+v", report.AuthorityBoundary)
	}
}

func TestWriteSpecValidationSummaryNamesIndependentSourceAndAuthority(
	t *testing.T,
) {
	t.Parallel()

	report, err := buildSpecValidationReport(newDraftSpecValidationProject(t))
	if err != nil {
		t.Fatalf("buildSpecValidationReport: %v", err)
	}

	var output bytes.Buffer
	if err := writeSpecValidationSummary(&output, report); err != nil {
		t.Fatalf("writeSpecValidationSummary: %v", err)
	}
	for _, expected := range []string{
		"source_basis: authored_carriers_without_profile_applicability_filter",
		"sections: total=10 draft=10 active=0 checked=10",
		"applicability=not_applicability_determination_or_admission",
		"approval=not_approval_or_baseline",
		"Lifecycle observations:",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("validation summary missing %q:\n%s", expected, output.String())
		}
	}
}

func TestHandleQuintQuerySpecValidateReturnsCarrierFirstReport(t *testing.T) {
	fixture := newBoundSpecQueryTestProject(t)
	result, err := handleQuintQuery(
		context.Background(),
		fixture.store,
		nil,
		fixture.haftDir,
		map[string]any{"action": "spec_validate"},
	)
	if err != nil {
		t.Fatalf("handleQuintQuery spec_validate returned error: %v", err)
	}

	var report specflow.CarrierValidationReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode MCP validation report: %v\n%s", err, result)
	}
	if report.ValidationKind != specflow.CarrierValidationKind || report.SourceBasis != specflow.CarrierValidationSource {
		t.Fatalf("MCP returned wrong validation contract: %+v", report)
	}
}

func newDraftSpecValidationProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("create spec dir: %v", err)
	}

	target := strings.Builder{}
	for index := 1; index <= 3; index++ {
		id := "TS.draft.00" + string(rune('0'+index))
		section := reviewCLISpecSection(id, "target-system", "target.boundary", "Draft target section", "definition", "object")
		target.WriteString(strings.Replace(section, "status: active", "status: draft", 1))
	}
	software := strings.Builder{}
	for index := 1; index <= 7; index++ {
		id := "SS.draft.00" + string(rune('0'+index))
		section := reviewCLISpecSection(id, "software-system", "software.interfaces", "Draft software section", "definition", "object")
		software.WriteString(strings.Replace(section, "status: active", "status: draft", 1))
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), target.String())
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "software-system.md"), software.String())
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())
	return root
}

func stubSpecValidateJSON(t *testing.T, value bool) func() {
	t.Helper()
	previous := specValidateJSON
	specValidateJSON = value
	return func() { specValidateJSON = previous }
}

func stubSpecValidateExit(t *testing.T) func() {
	t.Helper()
	previous := specValidateExit
	specValidateExit = func(code int) {
		t.Fatalf("spec validate unexpectedly exited with code %d", code)
	}
	return func() { specValidateExit = previous }
}
