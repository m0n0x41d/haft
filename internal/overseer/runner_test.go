package overseer

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunConfiguredReviewerReadsCommandResultFile(t *testing.T) {
	root := t.TempDir()
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: root, Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z"),
	}
	config := DefaultConfig()
	config.LLMReview = "on"
	config.ReviewerAgent = "command"
	config.ReviewerCommand = `printf '%s' '{"findings":[{"id":"ofind-runner","severity":"high","confidence":"high","claim":"Runner found an invariant violation.","concrete_harm":"The agent would miss the review signal."}]}' > "$HAFT_OVERSEER_RESULT_FILE"`

	result, err := RunConfiguredReviewer(context.Background(), root, config, stored)
	if err != nil {
		t.Fatalf("RunConfiguredReviewer returned error: %v", err)
	}
	if result.PromptPath == "" || result.PacketPath == "" || result.ResultPath == "" {
		t.Fatalf("runner paths were not populated: %+v", result)
	}
	if got := result.Input.Reviewer.Agent; got != "command" {
		t.Fatalf("reviewer agent = %q, want command", got)
	}
	if len(result.Input.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(result.Input.Findings))
	}

	stored, err = IngestReviewResult(stored, result.Input, "2026-06-09T01:00:00Z")
	if err != nil {
		t.Fatalf("IngestReviewResult returned error: %v", err)
	}
	if stored.Run.Findings[0].CountsForREff {
		t.Fatalf("runner finding must stay advisory and not count for R_eff")
	}
	if !strings.Contains(FormatStatusSignals(BuildStatusSummary(stored, true, MaintenanceRun{}, false)), "Runner found an invariant violation") {
		t.Fatalf("runner finding did not surface in status")
	}
}

func TestRunConfiguredReviewerMapsHumanReadableFindingAliases(t *testing.T) {
	root := t.TempDir()
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: root, Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z"),
	}
	config := DefaultConfig()
	config.LLMReview = "on"
	config.ReviewerAgent = "command"
	config.ReviewerCommand = `printf '%s' '{"findings":[{"id":"ofind-alias","severity":"medium","title":"Smoke reviewer finding","description":"This proves the reviewer result was ingested from the post-commit hook.","recommendation":"Close this finding with a disposition after the smoke test."}]}' > "$HAFT_OVERSEER_RESULT_FILE"`

	result, err := RunConfiguredReviewer(context.Background(), root, config, stored)
	if err != nil {
		t.Fatalf("RunConfiguredReviewer returned error: %v", err)
	}

	stored, err = IngestReviewResult(stored, result.Input, "2026-06-09T01:00:00Z")
	if err != nil {
		t.Fatalf("IngestReviewResult returned error: %v", err)
	}
	if stored.Run.Verdict != "findings_recorded" {
		t.Fatalf("verdict = %q, want findings_recorded", stored.Run.Verdict)
	}
	if len(stored.Run.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(stored.Run.Findings))
	}
	finding := stored.Run.Findings[0]
	if finding.Claim != "Smoke reviewer finding" {
		t.Fatalf("claim = %q, want alias title", finding.Claim)
	}
	if finding.ConcreteHarm != "This proves the reviewer result was ingested from the post-commit hook." {
		t.Fatalf("concrete harm = %q, want alias description", finding.ConcreteHarm)
	}
	if finding.MinimalFix != "Close this finding with a disposition after the smoke test." {
		t.Fatalf("minimal fix = %q, want alias recommendation", finding.MinimalFix)
	}
}

func TestRunConfiguredReviewerTimeoutKillsNestedShell(t *testing.T) {
	root := t.TempDir()
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: root, Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z"),
	}
	config := DefaultConfig()
	config.LLMReview = "on"
	config.ReviewerAgent = "command"
	config.ReviewTimeoutSeconds = 1
	config.ReviewerCommand = `sh -c 'sleep 30'`

	started := time.Now()
	_, err = RunConfiguredReviewer(context.Background(), root, config, stored)
	if err == nil {
		t.Fatalf("RunConfiguredReviewer should time out")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("timeout took %s, want under 5s", elapsed)
	}
}

func TestReviewAbstentionCreatesNoFindings(t *testing.T) {
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z"),
	}
	input := ReviewAbstention(DefaultConfig(), stored, "codex command unavailable")
	if input.Verdict != "review_abstained" {
		t.Fatalf("verdict = %q, want review_abstained", input.Verdict)
	}
	if len(input.Findings) != 0 {
		t.Fatalf("abstention findings = %d, want 0", len(input.Findings))
	}
	if len(input.ScopeCoverage.Abstentions) != 1 {
		t.Fatalf("abstentions = %d, want 1", len(input.ScopeCoverage.Abstentions))
	}
}

func TestReviewResultJSONSchemaIsStrictForCodexStructuredOutput(t *testing.T) {
	schema := ReviewResultJSONSchema()
	if strings.Contains(schema, `"additionalProperties": true`) {
		t.Fatalf("schema contains loose nested object settings:\n%s", schema)
	}
	for _, want := range []string{
		`"reviewer": {`,
		`"additionalProperties": false`,
		`"required": ["reviewer", "verdict", "findings", "non_findings_under_scope"]`,
		`"required": ["agent"]`,
		`"non_findings_under_scope": {`,
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("schema missing %q:\n%s", want, schema)
		}
	}
}
