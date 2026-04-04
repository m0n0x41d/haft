package present_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
)

func TestApplyFPFAnswerHygiene_RewritesInternalArtifactKinds(t *testing.T) {
	input := "ProblemCard ready. Reopen only works on DecisionRecords. Related SolutionPortfolio found."

	output := present.ApplyFPFAnswerHygiene(input)

	checks := []string{
		"problem ready.",
		"decisions.",
		"solution portfolio found.",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected %q in %q", check, output)
		}
	}

	if issues := present.LintFPFAnswer(output); len(issues) != 0 {
		t.Fatalf("expected rewritten output to be lint-clean, got %+v", issues)
	}
}

func TestLintFPFAnswer_FlagsInternalCompareFields(t *testing.T) {
	issues := present.LintFPFAnswer("selected_ref disagrees with non_dominated_set")

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %+v", issues)
	}
	if issues[0].Term != "selected_ref" {
		t.Fatalf("unexpected first issue: %+v", issues[0])
	}
	if issues[1].Term != "non_dominated_set" {
		t.Fatalf("unexpected second issue: %+v", issues[1])
	}
}

func TestMissingProblemResponse_StaysLintClean(t *testing.T) {
	output := present.MissingProblemResponse("\n-- nav --\n")

	if strings.Contains(output, "ProblemCard") {
		t.Fatalf("missing-problem response leaked raw kind: %s", output)
	}
	if issues := present.LintFPFAnswer(output); len(issues) != 0 {
		t.Fatalf("expected lint-clean missing-problem response, got %+v\n%s", issues, output)
	}
}

func TestSearchResponse_UsesPlainArtifactKindLabels(t *testing.T) {
	results := []*artifact.Artifact{{
		Meta: artifact.Meta{
			ID:    "dec-001",
			Kind:  artifact.KindDecisionRecord,
			Title: "Choose gRPC",
		},
		Body: "# Decision\n\nUse gRPC.",
	}}

	output := present.SearchResponse(results, "grpc")

	if !strings.Contains(output, "[decision]") {
		t.Fatalf("expected plain artifact label, got:\n%s", output)
	}
	if issues := present.LintFPFAnswer(output); len(issues) != 0 {
		t.Fatalf("expected lint-clean search response, got %+v\n%s", issues, output)
	}
}

func TestListResponse_UsesPlainHeadings(t *testing.T) {
	data := artifact.ListData{
		Kind: "DecisionRecord",
		Artifacts: []*artifact.Artifact{{
			Meta: artifact.Meta{
				ID:    "dec-001",
				Kind:  artifact.KindDecisionRecord,
				Title: "Choose gRPC",
			},
		}},
	}

	output := present.ListResponse(data)

	if !strings.Contains(output, "## Decision (1)") {
		t.Fatalf("expected plain heading, got:\n%s", output)
	}
	if issues := present.LintFPFAnswer(output); len(issues) != 0 {
		t.Fatalf("expected lint-clean list response, got %+v\n%s", issues, output)
	}
}
