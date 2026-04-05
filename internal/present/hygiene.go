package present

import "strings"

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
}

var fpfAnswerHygieneRules = []fpfAnswerHygieneRule{
	{term: "ProblemCards", replacement: "problems", reason: "internal artifact kind"},
	{term: "ProblemCard", replacement: "problem", reason: "internal artifact kind"},
	{term: "SolutionPortfolios", replacement: "solution portfolios", reason: "internal artifact kind"},
	{term: "SolutionPortfolio", replacement: "solution portfolio", reason: "internal artifact kind"},
	{term: "DecisionRecords", replacement: "decisions", reason: "internal artifact kind"},
	{term: "DecisionRecord", replacement: "decision", reason: "internal artifact kind"},
	{term: "EvidencePacks", replacement: "evidence packs", reason: "internal artifact kind"},
	{term: "EvidencePack", replacement: "evidence pack", reason: "internal artifact kind"},
	{term: "RefreshReports", replacement: "refresh reports", reason: "internal artifact kind"},
	{term: "RefreshReport", replacement: "refresh report", reason: "internal artifact kind"},
	{term: "selected_ref", replacement: "recommended variant", reason: "internal compare field"},
	{term: "non_dominated_set", replacement: "Pareto front", reason: "internal compare field"},
}

// ApplyFPFAnswerHygiene rewrites known internal terms in user-facing text.
func ApplyFPFAnswerHygiene(text string) string {
	rewritten := text

	for _, rule := range fpfAnswerHygieneRules {
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
	switch strings.TrimSpace(kind) {
	case "ProblemCard":
		return "problem"
	case "SolutionPortfolio":
		return "solution portfolio"
	case "DecisionRecord":
		return "decision"
	case "EvidencePack":
		return "evidence pack"
	case "RefreshReport":
		return "refresh report"
	default:
		return strings.TrimSpace(kind)
	}
}

// UserFacingArtifactKindHeading renders artifact kinds as a list heading.
func UserFacingArtifactKindHeading(kind string, count int) string {
	switch strings.TrimSpace(kind) {
	case "ProblemCard":
		if count == 1 {
			return "Problem"
		}
		return "Problems"
	case "SolutionPortfolio":
		if count == 1 {
			return "Solution Portfolio"
		}
		return "Solution Portfolios"
	case "DecisionRecord":
		if count == 1 {
			return "Decision"
		}
		return "Decisions"
	case "EvidencePack":
		if count == 1 {
			return "Evidence Pack"
		}
		return "Evidence Packs"
	case "RefreshReport":
		if count == 1 {
			return "Refresh Report"
		}
		return "Refresh Reports"
	default:
		return strings.TrimSpace(kind)
	}
}
