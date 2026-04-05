package present_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
)

func assertLintCleanGeneratedText(t *testing.T, fragments ...string) {
	t.Helper()

	issues := present.LintGeneratedText(fragments...)
	if len(issues) == 0 {
		return
	}

	t.Fatalf("expected generated text to be lint-clean, got %+v\n%s", issues, strings.Join(fragments, "\n"))
}

func TestUserFacingArtifactKindLabel_UsesPlainLanguage(t *testing.T) {
	outputs := []string{
		present.UserFacingArtifactKindLabel("ProblemCard"),
		present.UserFacingArtifactKindLabel("DecisionRecord"),
		present.UserFacingArtifactKindLabel("SolutionPortfolio"),
		present.UserFacingArtifactKindHeading("ProblemCard", 2),
		present.UserFacingArtifactKindHeading("DecisionRecord", 1),
	}

	assertLintCleanGeneratedText(t, outputs...)
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

	if !strings.Contains(output, "No active problem found.") {
		t.Fatalf("expected plain-language missing-problem message, got %q", output)
	}

	assertLintCleanGeneratedText(t, output)
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

	assertLintCleanGeneratedText(t,
		present.UserFacingArtifactKindHeading("DecisionRecord", 1),
		present.UserFacingArtifactKindHeading("DecisionRecord", 2),
	)
}
