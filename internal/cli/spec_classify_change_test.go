package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
)

func TestRunSpecClassifyChangeJSONReportsRelationshipUpdate(t *testing.T) {
	t.Parallel()

	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "target-system-updated.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", "depends_on:\n  - TS.boundary.001\n"))
	restore := stubSpecClassifyChangeFlags(t, before, after, "TS.sync.001", "target-system", true)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecClassifyChange(cmd, nil); err != nil {
		t.Fatalf("runSpecClassifyChange: %v", err)
	}

	var report project.SpecCarrierChangeReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, output.String())
	}
	if report.Kind != project.SpecCarrierChangeRelationshipUpdate {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.ImportPosture != project.SpecCarrierImportPostureRecognizedUpdate {
		t.Fatalf("import posture = %q", report.ImportPosture)
	}
	if !strings.Contains(strings.Join(report.RelationshipFields, ","), "depends_on") {
		t.Fatalf("relationship fields = %#v", report.RelationshipFields)
	}
}

func TestRunSpecClassifyChangeTextReportsHighRiskDocumentShift(t *testing.T) {
	t.Parallel()

	before := writeSpecClassifyChangeFile(t, "target-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	after := writeSpecClassifyChangeFile(t, "enabling-system.md", specClassifyChangeCarrier("TS.sync.001", "acceptance", ""))
	restore := stubSpecClassifyChangeFlags(t, before, after, "TS.sync.001", "", false)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecClassifyChange(cmd, nil); err != nil {
		t.Fatalf("runSpecClassifyChange: %v", err)
	}

	text := output.String()
	for _, want := range []string{"spec carrier change: unknown_high_risk", "import_posture: abstain_block", "high_risk_fields: document_kind"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func TestRunSpecClassifyChangeRequiresExplicitInputs(t *testing.T) {
	t.Parallel()

	restore := stubSpecClassifyChangeFlags(t, "", "", "", "", false)
	defer restore()

	err := runSpecClassifyChange(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("expected missing input error")
	}
	if !strings.Contains(err.Error(), "--before") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func specClassifyChangeCarrier(id string, kind string, extra string) string {
	return "## " + id + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: " + kind + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		extra +
		"```\n"
}

func writeSpecClassifyChangeFile(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func stubSpecClassifyChangeFlags(t *testing.T, before string, after string, section string, kind string, jsonFlag bool) func() {
	t.Helper()
	previousBefore := specClassifyBefore
	previousAfter := specClassifyAfter
	previousSection := specClassifySection
	previousKind := specClassifyKind
	previousJSON := specClassifyChangeJSON
	specClassifyBefore = before
	specClassifyAfter = after
	specClassifySection = section
	specClassifyKind = kind
	specClassifyChangeJSON = jsonFlag
	return func() {
		specClassifyBefore = previousBefore
		specClassifyAfter = previousAfter
		specClassifySection = previousSection
		specClassifyKind = previousKind
		specClassifyChangeJSON = previousJSON
	}
}
