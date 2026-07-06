package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
)

func TestRunPatternRecallCompactOmitsSourceCardBody(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternRecallFlags(true, fpf.PatternRecallCompactMode, fpf.PatternRecallDefaultLimit)
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternRecall(cmd, []string{"Use", "boundary", "norm", "square", "admissibility"})
	if err != nil {
		t.Fatalf("pattern recall returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternRecallResult
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.SupportLevel != fpf.PatternRecallSupportSourceCardRetrieved {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if len(record.CandidateSourceCards) == 0 {
		t.Fatalf("candidate set missing: %#v", record)
	}
	if record.CandidateSourceCards[0].PatternID != "A.6.B" {
		t.Fatalf("pattern = %q", record.CandidateSourceCards[0].PatternID)
	}
	if record.CandidateSourceCards[0].SourceCard != nil {
		t.Fatalf("compact recall should not carry source_card: %#v", record.CandidateSourceCards[0].SourceCard)
	}
}

func TestRunPatternRecallFullIncludesAtlasSourceCard(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternRecallFlags(true, fpf.PatternRecallFullMode, fpf.PatternRecallDefaultLimit)
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternRecall(cmd, []string{"Use", "boundary", "norm", "square", "admissibility"})
	if err != nil {
		t.Fatalf("pattern recall returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternRecallResult
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	card := record.CandidateSourceCards[0].SourceCard
	if card == nil {
		t.Fatalf("source card missing: %#v", record.CandidateSourceCards[0])
	}
	if !strings.Contains(card.Body, "Full boundary norm square card body") {
		t.Fatalf("source card body missing fixture text: %#v", card)
	}
	if card.LineStart == 0 || card.LineEnd == 0 {
		t.Fatalf("source card range missing: %#v", card)
	}
	if record.HasAuthorityViolation() {
		t.Fatalf("authority violation: %#v", record)
	}
}

func TestRunPatternRecallExactPatternRefWinsOverRetrievalRanking(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternRecallFlags(true, fpf.PatternRecallCompactMode, fpf.PatternRecallDefaultLimit)
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternRecall(cmd, []string{"A.6.B", "Boundary", "Norm", "Square"})
	if err != nil {
		t.Fatalf("pattern recall returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternRecallResult
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if len(record.CandidateSourceCards) == 0 {
		t.Fatalf("candidate set missing: %#v", record)
	}
	if record.CandidateSourceCards[0].PatternID != "A.6.B" {
		t.Fatalf("first pattern = %q, candidates=%#v", record.CandidateSourceCards[0].PatternID, record.CandidateSourceCards)
	}
	if record.CandidateSourceCards[0].SourceReason != "explicit pattern_ref in query" {
		t.Fatalf("source reason = %q", record.CandidateSourceCards[0].SourceReason)
	}
}

func TestRunPatternRecallMechanicalPromptReturnsMissing(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()
	restoreFlags := stubPatternRecallFlags(true, fpf.PatternRecallCompactMode, fpf.PatternRecallDefaultLimit)
	defer restoreFlags()

	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)

	err := runPatternRecall(cmd, []string{"what", "time", "is", "it"})
	if err != nil {
		t.Fatalf("pattern recall returned error: %v\n%s", err, output.String())
	}

	var record fpf.PatternRecallResult
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal output: %v\n%s", err, output.String())
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if record.SupportLevel != fpf.PatternRecallSupportMissing {
		t.Fatalf("support = %q", record.SupportLevel)
	}
	if len(record.CandidateSourceCards) != 0 {
		t.Fatalf("candidates = %#v", record.CandidateSourceCards)
	}
}

func TestHandleQuintQueryPatternRecallCompactAndFull(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	root := t.TempDir()
	store := setupCLIArtifactStore(t)
	haftDir := filepath.Join(root, ".haft")

	compact, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_recall",
		"mode":   "compact",
		"query":  "Use boundary norm square admissibility for this concern.",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(pattern_recall compact) returned error: %v", err)
	}
	var compactRecord fpf.PatternRecallResult
	if err := json.Unmarshal([]byte(compact), &compactRecord); err != nil {
		t.Fatalf("json.Unmarshal compact: %v\n%s", err, compact)
	}
	if err := compactRecord.Validate(); err != nil {
		t.Fatalf("compact Validate returned error: %v", err)
	}
	if compactRecord.CandidateSourceCards[0].SourceCard != nil {
		t.Fatalf("compact carried source_card: %#v", compactRecord.CandidateSourceCards[0].SourceCard)
	}

	full, err := handleQuintQuery(context.Background(), store, nil, haftDir, map[string]any{
		"action": "pattern_recall",
		"mode":   "full",
		"query":  "Use boundary norm square admissibility for this concern.",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery(pattern_recall full) returned error: %v", err)
	}
	var fullRecord fpf.PatternRecallResult
	if err := json.Unmarshal([]byte(full), &fullRecord); err != nil {
		t.Fatalf("json.Unmarshal full: %v\n%s", err, full)
	}
	if err := fullRecord.Validate(); err != nil {
		t.Fatalf("full Validate returned error: %v", err)
	}
	if fullRecord.CandidateSourceCards[0].SourceCard == nil {
		t.Fatalf("full source_card missing: %#v", fullRecord.CandidateSourceCards[0])
	}
}

func TestBuildPatternRecallAuditReport(t *testing.T) {
	dbPath := buildPatternUseFallbackTestDB(t)
	restoreOpen := stubOpenFPFDB(t, dbPath)
	defer restoreOpen()

	path := writePatternRecallAuditFixture(t, `
schema_version: 1
purpose: pattern recall fixture
status: fixture
canonical_prompts:
  - id: boundary_compact
    prompt: "Use boundary norm square admissibility for this concern."
    mode: compact
    expected_pattern_id: "A.6.B"
    expected_support_level: "source_card_retrieved"
held_out_prompts:
  - id: boundary_full
    prompt: "Use boundary norm square admissibility for this concern."
    mode: full
    expected_pattern_id: "A.6.B"
    expected_support_level: "source_card_retrieved"
    expect_source_body: true
adversarial_prompts:
  - id: mechanical_time
    prompt: "what time is it"
    mode: compact
    expected_support_level: "missing"
    expect_no_candidates: true
`)

	report, err := buildPatternRecallAuditReport(path, timeNow())
	if err != nil {
		t.Fatalf("buildPatternRecallAuditReport returned error: %v", err)
	}
	if !report.Summary.Pass {
		t.Fatalf("audit did not pass: %#v", report)
	}
	if report.Summary.Prompts != 3 {
		t.Fatalf("prompts = %d", report.Summary.Prompts)
	}
}

func stubPatternRecallFlags(jsonOutput bool, mode string, limit int) func() {
	originalJSON := patternRecallJSON
	originalMode := patternRecallMode
	originalLimit := patternRecallLimit
	patternRecallJSON = jsonOutput
	patternRecallMode = mode
	patternRecallLimit = limit

	return func() {
		patternRecallJSON = originalJSON
		patternRecallMode = originalMode
		patternRecallLimit = originalLimit
	}
}

func writePatternRecallAuditFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "patternrecall-prompts.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}
