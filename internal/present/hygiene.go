package present

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// FPFAnswerHygieneIssue reports an internal term that leaked into user-facing text.
type FPFAnswerHygieneIssue struct {
	Term        string
	Replacement string
	Reason      string
}

type fpfAnswerHygieneRule struct {
	term        string
	replacement string
	reason      string
	rewriteSafe bool
}

var fpfAnswerHygieneRules = []fpfAnswerHygieneRule{
	{term: "ProblemCards", replacement: "problems", reason: "internal artifact kind", rewriteSafe: true},
	{term: "ProblemCard", replacement: "problem", reason: "internal artifact kind", rewriteSafe: true},
	{term: "SolutionPortfolios", replacement: "solution portfolios", reason: "internal artifact kind", rewriteSafe: true},
	{term: "SolutionPortfolio", replacement: "solution portfolio", reason: "internal artifact kind", rewriteSafe: true},
	{term: "DecisionRecords", replacement: "decisions", reason: "internal artifact kind", rewriteSafe: true},
	{term: "DecisionRecord", replacement: "decision", reason: "internal artifact kind", rewriteSafe: true},
	{term: "EvidencePacks", replacement: "evidence packs", reason: "internal artifact kind", rewriteSafe: true},
	{term: "EvidencePack", replacement: "evidence pack", reason: "internal artifact kind", rewriteSafe: true},
	{term: "RefreshReports", replacement: "refresh reports", reason: "internal artifact kind", rewriteSafe: true},
	{term: "RefreshReport", replacement: "refresh report", reason: "internal artifact kind", rewriteSafe: true},
	{term: "selected_ref", replacement: "recommended variant", reason: "internal compare field"},
	{term: "non_dominated_set", replacement: "Pareto front", reason: "internal compare field"},
}

// ApplyFPFAnswerHygiene rewrites only the rules that are safe for natural-language text.
// Internal field names stay lint-only so literal user input and exact echoes remain intact.
func ApplyFPFAnswerHygiene(text string) string {
	rewritten := text

	for _, rule := range fpfAnswerHygieneRules {
		if !rule.rewriteSafe {
			continue
		}

		rewritten = strings.ReplaceAll(rewritten, rule.term, rule.replacement)
	}

	return rewritten
}

// LintFPFAnswer reports internal terms that should not appear in user-facing
// Haft output.
func LintFPFAnswer(text string) []FPFAnswerHygieneIssue {
	issues := make([]FPFAnswerHygieneIssue, 0)

	for _, rule := range fpfAnswerHygieneRules {
		if !strings.Contains(text, rule.term) {
			continue
		}

		issues = append(issues, FPFAnswerHygieneIssue{
			Term:        rule.term,
			Replacement: rule.replacement,
			Reason:      rule.reason,
		})
	}

	return issues
}

// LintGeneratedText joins generated fragments and reports internal term leaks.
func LintGeneratedText(fragments ...string) []FPFAnswerHygieneIssue {
	return LintFPFAnswer(strings.Join(fragments, "\n"))
}

// UserFacingArtifactKindLabel renders internal artifact kinds as plain language.
func UserFacingArtifactKindLabel(kind string) string {
	label := artifact.Kind(strings.TrimSpace(kind)).UserFacingLabel()
	return label
}

// UserFacingArtifactKindHeading renders artifact kinds as a list heading.
func UserFacingArtifactKindHeading(kind string, count int) string {
	heading := artifact.Kind(strings.TrimSpace(kind)).UserFacingHeading(count)
	return heading
}
