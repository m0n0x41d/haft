package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunSpecExportJSONRendersCurrentSQLEditionMarkdown(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()
	runSpecSyncForExportTest(t)
	restoreFlags := stubSpecExportFlags(t, true, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecExport(cmd, []string{"TS.sync.001"}); err != nil {
		t.Fatalf("runSpecExport: %v\n%s", err, output.String())
	}

	var result specExportResult
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, output.String())
	}
	if result.SourceOfTruth != "sql_project_graph" {
		t.Fatalf("source_of_truth = %q", result.SourceOfTruth)
	}
	if result.AuthorityBoundary != "publication_projection_only_not_approval_rebaseline_evidence_or_gate" {
		t.Fatalf("authority boundary = %q", result.AuthorityBoundary)
	}
	if result.Edition.SectionID != "TS.sync.001" {
		t.Fatalf("section id = %q", result.Edition.SectionID)
	}
	if result.Edition.SemanticHash == "" || result.Publication.SourceEditionHash != result.Edition.SemanticHash {
		t.Fatalf("edition/publication hashes = %#v / %#v", result.Edition, result.Publication)
	}
	if result.Publication.PublicationHash == "" {
		t.Fatalf("publication hash missing: %#v", result.Publication)
	}
	if !strings.Contains(result.Publication.Markdown, "```yaml spec-section") {
		t.Fatalf("markdown projection missing spec-section fence:\n%s", result.Publication.Markdown)
	}
	if result.Audit.SourceEpisteme != "sql_spec_section_edition" {
		t.Fatalf("audit source episteme = %#v", result.Audit)
	}
}

func TestRunSpecExportMarkdownPrintsCarrierProjectionOnly(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()
	runSpecSyncForExportTest(t)
	restoreFlags := stubSpecExportFlags(t, false, true)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	if err := runSpecExport(cmd, []string{"TS.sync.001"}); err != nil {
		t.Fatalf("runSpecExport: %v\n%s", err, output.String())
	}

	text := output.String()
	for _, want := range []string{"## TS.sync.001", "```yaml spec-section", "id: TS.sync.001"} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown output missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"authority_boundary:", "source_of_truth:", "publication_hash:"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("markdown-only output leaked audit field %q:\n%s", forbidden, text)
		}
	}
}

func TestRunSpecExportBlocksWhenSQLEditionMissing(t *testing.T) {
	root := setupSpecSyncProject(t)
	restoreCwd := chdirForTest(t, root)
	defer restoreCwd()
	restoreFlags := stubSpecExportFlags(t, true, false)
	defer restoreFlags()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	err := runSpecExport(cmd, []string{"TS.sync.001"})
	if err == nil {
		t.Fatal("expected spec export to block without SQL edition")
	}
	if !strings.Contains(err.Error(), "run `haft spec sync` first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSpecExportRejectsConflictingOutputModes(t *testing.T) {
	restoreFlags := stubSpecExportFlags(t, true, true)
	defer restoreFlags()

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	err := runSpecExport(cmd, []string{"TS.sync.001"})
	if err == nil {
		t.Fatal("expected conflicting output modes to fail")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func runSpecSyncForExportTest(t *testing.T) {
	t.Helper()
	restoreFlags := stubSpecSyncFlags(t, true)
	defer restoreFlags()

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	if err := runSpecSync(cmd, nil); err != nil {
		t.Fatalf("runSpecSync: %v", err)
	}
}

func stubSpecExportFlags(t *testing.T, jsonFlag bool, markdownFlag bool) func() {
	t.Helper()
	previousJSON := specExportJSON
	previousMarkdown := specExportMarkdown
	specExportJSON = jsonFlag
	specExportMarkdown = markdownFlag
	return func() {
		specExportJSON = previousJSON
		specExportMarkdown = previousMarkdown
	}
}
