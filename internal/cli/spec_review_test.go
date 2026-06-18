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

func TestRunSpecReviewJSONReturnsAdvisoryPacket(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecReviewJSON(t, true)
	defer restoreJSON()

	err := runSpecReview(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecReview returned error: %v", err)
	}

	var packet specflow.ReviewPacket
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatalf("decode review JSON: %v\n%s", err, output.String())
	}
	if packet.ReviewKind != specflow.ReviewKindSpecSemantic {
		t.Fatalf("review_kind = %q, want %q", packet.ReviewKind, specflow.ReviewKindSpecSemantic)
	}
	if packet.Authority != specflow.ReviewAuthority {
		t.Fatalf("authority = %q, want %q", packet.Authority, specflow.ReviewAuthority)
	}
	if packet.Summary.CheckedSections != 2 {
		t.Fatalf("checked_sections = %d, want 2", packet.Summary.CheckedSections)
	}
	if strings.Contains(output.String(), `"status":"ready"`) || strings.Contains(output.String(), `"verdict":"pass"`) {
		t.Fatalf("review JSON must not expose ready/pass authority: %s", output.String())
	}
}

func TestRunSpecReviewSummaryNamesAdvisoryBoundary(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecReviewJSON(t, false)
	defer restoreJSON()

	err := runSpecReview(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecReview returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "advisory_only") {
		t.Fatalf("summary missing advisory boundary:\n%s", result)
	}
	if !strings.Contains(result, "not evidence, approval, rebaseline, GateDecision, or SpecUseAdmission") {
		t.Fatalf("summary missing authority disclaimer:\n%s", result)
	}
}

func TestHandleQuintQuerySpecReviewReturnsPacket(t *testing.T) {
	fixture := newCheckTestProject(t)
	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "spec_review",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_review returned error: %v", err)
	}

	var packet specflow.ReviewPacket
	if err := json.Unmarshal([]byte(result), &packet); err != nil {
		t.Fatalf("decode MCP spec_review packet: %v\n%s", err, result)
	}
	if packet.Authority != specflow.ReviewAuthority {
		t.Fatalf("authority = %q, want %q", packet.Authority, specflow.ReviewAuthority)
	}
	if packet.Summary.CheckedSections == 0 {
		t.Fatalf("checked_sections = 0, want active sections")
	}
}

func newSpecReviewCLIProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), reviewCLISpecSection(
		"TS.environment.001",
		"target-system",
		"target.environment",
		"Test target environment",
		"definition",
		"object",
	))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), reviewCLISpecSection(
		"ES.agent-policy.001",
		"enabling-system",
		"enabling.agent_policy",
		"Test agent policy",
		"duty",
		"work",
	))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())

	return root
}

func reviewCLISpecSection(
	id string,
	spec string,
	kind string,
	title string,
	statementType string,
	claimLayer string,
) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"spec: " + spec + "\n" +
		"kind: " + kind + "\n" +
		"title: " + title + "\n" +
		"statement_type: " + statementType + "\n" +
		"claim_layer: " + claimLayer + "\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-01-01\n" +
		"evidence_required:\n" +
		"  - kind: review\n" +
		"    description: Human confirms this section still holds.\n" +
		"```\n"
}

func stubSpecReviewJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specReviewJSON
	specReviewJSON = value

	return func() {
		specReviewJSON = previous
	}
}
