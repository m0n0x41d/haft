package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ExploreInput is the input for creating a SolutionPortfolio with variants.
type ExploreInput struct {
	ProblemRef string    `json:"problem_ref,omitempty"`
	Variants   []Variant `json:"variants"`
	Context    string    `json:"context,omitempty"`
	Mode       string    `json:"mode,omitempty"`
}

// CompareInput is the input for running a parity comparison.
type CompareInput struct {
	PortfolioRef string           `json:"portfolio_ref,omitempty"`
	Results      ComparisonResult `json:"results"`
}

// ExploreSolutions creates a SolutionPortfolio artifact with variants.
// ExploreContext holds pre-fetched data for pure portfolio construction.
type ExploreContext struct {
	ID           string
	Now          time.Time
	ProblemTitle string // from linked problem
	Context      string // inherited
	Mode         Mode   // inherited or default
	Links        []Link
}

// ValidateExploreInput checks variant constraints. Pure.
func ValidateExploreInput(input ExploreInput) error {
	if len(input.Variants) == 0 {
		return fmt.Errorf("no variants received — check that 'variants' is a JSON array of objects with 'title' and 'weakest_link' fields")
	}
	if len(input.Variants) < 2 {
		return fmt.Errorf("at least 2 variants required (got %d) — genuinely distinct options, not variations of one idea", len(input.Variants))
	}
	for i, v := range input.Variants {
		if v.Title == "" {
			return fmt.Errorf("variant %d: title is required", i+1)
		}
		if v.WeakestLink == "" {
			return fmt.Errorf("variant %d (%s): weakest_link is required — what bounds this option's quality?", i+1, v.Title)
		}
	}
	return nil
}

// CheckVariantDiversity warns on near-identical variants (Jaccard > 0.5). Pure.
func CheckVariantDiversity(variants []Variant) []string {
	var warnings []string
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			textI := variants[i].Title + " " + variants[i].Description
			textJ := variants[j].Title + " " + variants[j].Description
			sim := jaccardSimilarity(textI, textJ)
			if sim > 0.5 {
				warnings = append(warnings,
					fmt.Sprintf("Variants '%s' and '%s' look similar (%.0f%% word overlap) — do they differ in kind, not degree?",
						variants[i].Title, variants[j].Title, sim*100))
			}
		}
	}
	return warnings
}

// BuildPortfolioArtifact constructs a SolutionPortfolio from input. Pure — no side effects.
func BuildPortfolioArtifact(ectx ExploreContext, input ExploreInput, diversityWarnings []string, recall string) *Artifact {
	title := "Solution Portfolio"
	if ectx.ProblemTitle != "" {
		title = fmt.Sprintf("Solutions for: %s", ectx.ProblemTitle)
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n\n", title))

	if input.ProblemRef != "" {
		body.WriteString(fmt.Sprintf("Problem: %s\n\n", input.ProblemRef))
	}

	body.WriteString(fmt.Sprintf("## Variants (%d)\n\n", len(input.Variants)))

	for i, v := range input.Variants {
		vid := v.ID
		if vid == "" {
			vid = fmt.Sprintf("V%d", i+1)
		}
		body.WriteString(fmt.Sprintf("### %s. %s\n\n", vid, v.Title))

		if v.Description != "" {
			body.WriteString(fmt.Sprintf("%s\n\n", v.Description))
		}

		if len(v.Strengths) > 0 {
			body.WriteString("**Strengths:**\n")
			for _, s := range v.Strengths {
				body.WriteString(fmt.Sprintf("- %s\n", s))
			}
			body.WriteString("\n")
		}

		body.WriteString(fmt.Sprintf("**Weakest link:** %s\n\n", v.WeakestLink))

		if len(v.Risks) > 0 {
			body.WriteString("**Risks:**\n")
			for _, r := range v.Risks {
				body.WriteString(fmt.Sprintf("- %s\n", r))
			}
			body.WriteString("\n")
		}

		if v.SteppingStone {
			body.WriteString("**Stepping stone:** yes — opens future possibilities\n\n")
		}

		if v.RollbackNotes != "" {
			body.WriteString(fmt.Sprintf("**Rollback:** %s\n\n", v.RollbackNotes))
		}
	}

	// Summary table
	body.WriteString("## Summary\n\n")
	body.WriteString("| Variant | Weakest Link | Stepping Stone |\n")
	body.WriteString("|---------|-------------|----------------|\n")
	for i, v := range input.Variants {
		vid := v.ID
		if vid == "" {
			vid = fmt.Sprintf("V%d", i+1)
		}
		ss := "no"
		if v.SteppingStone {
			ss = "yes"
		}
		body.WriteString(fmt.Sprintf("| %s. %s | %s | %s |\n", vid, v.Title, v.WeakestLink, ss))
	}
	body.WriteString("\n")

	a := &Artifact{
		Meta: Meta{
			ID:        ectx.ID,
			Kind:      KindSolutionPortfolio,
			Version:   1,
			Status:    StatusActive,
			Context:   ectx.Context,
			Mode:      ectx.Mode,
			Title:     title,
			CreatedAt: ectx.Now,
			UpdatedAt: ectx.Now,
			Links:     ectx.Links,
		},
		Body: body.String(),
	}

	if len(diversityWarnings) > 0 {
		a.Body += "\n## Diversity Warnings\n\n"
		for _, w := range diversityWarnings {
			a.Body += fmt.Sprintf("- ⚠ %s\n", w)
		}
	}

	if recall != "" {
		a.Body += recall
	}

	if ectx.Mode == ModeStandard || ectx.Mode == ModeDeep {
		a.Body += "\n## SoTA Survey Reminder\n\n"
		a.Body += "Before deciding, have you surveyed existing solutions?\n"
		a.Body += "- **Web search** — industry patterns, blog posts, case studies\n"
		a.Body += "- **Library docs** — check current API/usage patterns for relevant libraries\n"
		a.Body += "- **FPF spec search** — `quint_query(action=\"fpf\", query=\"<topic>\")` for methodology patterns\n"
		a.Body += "\n## Evidence Collection\n\n"
		a.Body += "Research each variant before comparing. Run tests, check benchmarks, validate claims.\n"
		a.Body += fmt.Sprintf("Attach findings: `quint_decision(action=\"evidence\", artifact_ref=\"%s\", evidence_content=\"...\", evidence_type=\"research\", evidence_verdict=\"supports\")`\n", a.Meta.ID)
	}

	return a
}

// ExploreSolutions creates a SolutionPortfolio. Orchestrates effects around BuildPortfolioArtifact.
func ExploreSolutions(ctx context.Context, store ArtifactStore, quintDir string, input ExploreInput) (*Artifact, string, error) {
	if err := ValidateExploreInput(input); err != nil {
		return nil, "", err
	}

	diversityWarnings := CheckVariantDiversity(input.Variants)

	// Effects: resolve problem reference
	var problemTitle string
	var links []Link
	resolvedContext := input.Context
	resolvedMode := input.Mode
	if input.ProblemRef != "" {
		prob, err := store.Get(ctx, input.ProblemRef)
		if err != nil {
			return nil, "", fmt.Errorf("problem %s not found: %w", input.ProblemRef, err)
		}
		if prob.Meta.Kind != KindProblemCard {
			return nil, "", fmt.Errorf("%s is %s, not ProblemCard", input.ProblemRef, prob.Meta.Kind)
		}
		problemTitle = prob.Meta.Title
		links = append(links, Link{Ref: input.ProblemRef, Type: "based_on"})
		if resolvedContext == "" {
			resolvedContext = prob.Meta.Context
		}
		if resolvedMode == "" {
			resolvedMode = string(prob.Meta.Mode)
		}
	}

	seq, err := store.NextSequence(ctx, KindSolutionPortfolio)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindSolutionPortfolio, seq)
	mode := Mode(resolvedMode)
	if mode == "" {
		mode = ModeStandard
	}

	recall := recallRelated(ctx, store, problemTitle)

	// Pure construction
	a := BuildPortfolioArtifact(ExploreContext{
		ID:           id,
		Now:          time.Now().UTC(),
		ProblemTitle: problemTitle,
		Context:      resolvedContext,
		Mode:         mode,
		Links:        links,
	}, input, diversityWarnings, recall)

	// Effects: persist
	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store portfolio: %w", err)
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// CompareSolutions adds comparison results to an existing SolutionPortfolio.
func CompareSolutions(ctx context.Context, store ArtifactStore, quintDir string, input CompareInput) (*Artifact, string, error) {
	if input.PortfolioRef == "" {
		return nil, "", fmt.Errorf("portfolio_ref is required")
	}

	a, err := store.Get(ctx, input.PortfolioRef)
	if err != nil {
		return nil, "", fmt.Errorf("portfolio %s not found: %w", input.PortfolioRef, err)
	}
	if a.Meta.Kind != KindSolutionPortfolio {
		return nil, "", fmt.Errorf("%s is %s, not SolutionPortfolio", input.PortfolioRef, a.Meta.Kind)
	}

	if len(input.Results.Dimensions) == 0 {
		return nil, "", fmt.Errorf("at least one comparison dimension is required")
	}
	if len(input.Results.NonDominatedSet) == 0 {
		return nil, "", fmt.Errorf("non_dominated_set is required — which variants are on the Pareto front?")
	}

	// Cross-check against characterization from linked ProblemCard
	var compareWarnings []string
	links, _ := store.GetLinks(ctx, input.PortfolioRef)
	for _, link := range links {
		if link.Type != "based_on" {
			continue
		}
		prob, err := store.Get(ctx, link.Ref)
		if err != nil || prob.Meta.Kind != KindProblemCard {
			continue
		}

		// Extract characterized dimensions with roles
		charDimsWithRoles := extractCharacterizedDimensionsWithRoles(prob.Body)
		if len(charDimsWithRoles) == 0 {
			break
		}

		// Check dimension coverage: which characterized dims are missing from compare?
		compareDimsLower := make(map[string]bool)
		for _, d := range input.Results.Dimensions {
			compareDimsLower[strings.ToLower(strings.TrimSpace(d))] = true
		}
		for _, cd := range charDimsWithRoles {
			if !compareDimsLower[strings.ToLower(cd.Name)] {
				severity := "missing"
				if cd.Role == "constraint" {
					severity = "CONSTRAINT missing"
				} else if cd.Role == "observation" {
					severity = "observation missing (informational)"
				}
				compareWarnings = append(compareWarnings,
					fmt.Sprintf("Characterized dimension '%s' (%s) not in comparison", cd.Name, severity))
			}
		}

		// Check dimension measurement freshness
		now := time.Now().UTC()
		for _, cd := range charDimsWithRoles {
			if cd.ValidUntil != "" {
				// Try both RFC3339 and date-only formats
				var parsed time.Time
				var parseErr error
				parsed, parseErr = time.Parse(time.RFC3339, cd.ValidUntil)
				if parseErr != nil {
					parsed, parseErr = time.Parse("2006-01-02", cd.ValidUntil)
				}
				if parseErr == nil && parsed.Before(now) {
					days := int(now.Sub(parsed).Hours() / 24)
					compareWarnings = append(compareWarnings,
						fmt.Sprintf("Dimension '%s' measurement expired %d day(s) ago — remeasure before comparing", cd.Name, days))
				}
			}
		}

		// Check score completeness: are all variants scored on all dimensions?
		for variantID, scores := range input.Results.Scores {
			var gaps []string
			for _, d := range input.Results.Dimensions {
				if scores[d] == "" || scores[d] == "-" {
					gaps = append(gaps, d)
				}
			}
			if len(gaps) > 0 {
				compareWarnings = append(compareWarnings,
					fmt.Sprintf("Variant %s missing scores for: %s", variantID, strings.Join(gaps, ", ")))
			}
		}

		// Remind about parity rules (free-text)
		if strings.Contains(prob.Body, "**Parity rules:**") {
			compareWarnings = append(compareWarnings,
				"ProblemCard has parity rules defined — verify comparison respects them")
		}

		// Auto-generate parity checklist with role-aware language
		if len(charDimsWithRoles) > 0 {
			compareWarnings = append(compareWarnings, "Parity checklist (per characterized dimension):")
			for _, d := range charDimsWithRoles {
				switch d.Role {
				case "constraint":
					compareWarnings = append(compareWarnings,
						fmt.Sprintf("  - CONSTRAINT '%s': is it satisfied by all variants?", d.Name))
				case "observation":
					compareWarnings = append(compareWarnings,
						fmt.Sprintf("  - OBSERVE '%s': monitored under same conditions? (don't optimize)", d.Name))
				default:
					compareWarnings = append(compareWarnings,
						fmt.Sprintf("  - TARGET '%s': measured under same conditions for all variants?", d.Name))
				}
			}
		}

		break // only check first linked problem
	}

	// Pure: build comparison section + apply to body
	a.Body = BuildComparisonBody(a.Body, input.Results, compareWarnings)

	// Effects: persist
	if err := store.Update(ctx, a); err != nil {
		return nil, "", fmt.Errorf("update portfolio: %w", err)
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// BuildComparisonBody appends comparison results to an existing portfolio body. Pure.
func BuildComparisonBody(existingBody string, results ComparisonResult, warnings []string) string {
	var section strings.Builder
	section.WriteString("\n## Comparison\n\n")

	header := "| Variant |"
	sep := "|---------|"
	for _, d := range results.Dimensions {
		header += fmt.Sprintf(" %s |", d)
		sep += "------|"
	}
	section.WriteString(header + "\n")
	section.WriteString(sep + "\n")

	for variantID, scores := range results.Scores {
		row := fmt.Sprintf("| %s |", variantID)
		for _, d := range results.Dimensions {
			val := scores[d]
			if val == "" {
				val = "-"
			}
			row += fmt.Sprintf(" %s |", val)
		}
		section.WriteString(row + "\n")
	}
	section.WriteString("\n")

	section.WriteString(fmt.Sprintf("## Non-Dominated Set\n\n**Pareto front:** %s\n\n",
		strings.Join(results.NonDominatedSet, ", ")))

	if len(results.Incomparable) > 0 {
		section.WriteString("**Incomparable pairs:**\n")
		for _, pair := range results.Incomparable {
			section.WriteString(fmt.Sprintf("- %s vs %s\n", pair[0], pair[1]))
		}
		section.WriteString("\n")
	}

	if results.PolicyApplied != "" {
		section.WriteString(fmt.Sprintf("**Selection policy:** %s\n\n", results.PolicyApplied))
	}

	if results.SelectedRef != "" {
		section.WriteString(fmt.Sprintf("**Recommended:** %s\n\n", results.SelectedRef))
	}

	// Strip existing comparison if present
	body := existingBody
	if idx := strings.Index(body, "\n## Comparison"); idx != -1 {
		body = body[:idx]
	}
	body += section.String()

	if len(warnings) > 0 {
		body += "\n## Comparison Warnings\n\n"
		for _, w := range warnings {
			body += fmt.Sprintf("- ⚠ %s\n", w)
		}
	}

	return body
}

// charDim holds a parsed dimension with its indicator role and freshness.
type charDim struct {
	Name       string
	Role       string // constraint, target, observation
	ValidUntil string // measurement freshness (RFC3339 or empty)
}

// extractCharacterizedDimensions parses dimension names and roles from the latest
// Characterization table in a ProblemCard body. Returns nil if no characterization found.
func extractCharacterizedDimensions(body string) []string {
	dims := extractCharacterizedDimensionsWithRoles(body)
	if dims == nil {
		return nil
	}
	var names []string
	for _, d := range dims {
		names = append(names, d.Name)
	}
	return names
}

// extractCharacterizedDimensionsWithRoles parses dimension names and roles.
// Table format: | Dimension | Role | Scale | Unit | Polarity | Measurement |
// Old format (no Role column): | Dimension | Scale | Unit | Polarity | Measurement |
func extractCharacterizedDimensionsWithRoles(body string) []charDim {
	lastIdx := -1
	for i := 100; i >= 1; i-- {
		marker := fmt.Sprintf("## Characterization v%d", i)
		if idx := strings.Index(body, marker); idx != -1 {
			lastIdx = idx
			break
		}
	}
	if lastIdx == -1 {
		return nil
	}

	section := body[lastIdx:]
	if endIdx := strings.Index(section[1:], "\n## "); endIdx != -1 {
		section = section[:endIdx+1]
	}

	var dims []charDim
	lines := strings.Split(section, "\n")
	inTable := false
	hasRoleColumn := false
	hasValidUntilColumn := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break
			}
			continue
		}
		if strings.Contains(line, "Dimension") {
			hasRoleColumn = strings.Contains(line, "Role")
			hasValidUntilColumn = strings.Contains(line, "Valid Until")
			inTable = true
			continue
		}
		if strings.Contains(line, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if name == "" || name == "-" {
			continue
		}
		role := "target"
		if hasRoleColumn && len(parts) >= 4 {
			r := strings.TrimSpace(parts[2])
			if r == "constraint" || r == "target" || r == "observation" {
				role = r
			}
		}
		validUntil := ""
		if hasValidUntilColumn {
			// Valid Until is the last data column
			lastCol := strings.TrimSpace(parts[len(parts)-2]) // -2 because last is empty after trailing |
			if lastCol != "" && lastCol != "-" {
				validUntil = lastCol
			}
		}
		dims = append(dims, charDim{Name: name, Role: role, ValidUntil: validUntil})
	}
	return dims
}

// jaccardSimilarity computes Jaccard index (intersection/union) of word sets from two texts.
func jaccardSimilarity(a, b string) float64 {
	wordsA := wordSet(a)
	wordsB := wordSet(b)
	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 0
	}

	intersection := 0
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
	}

	union := len(wordsA)
	for w := range wordsB {
		if !wordsA[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// containment computes what fraction of words in 'subset' appear in 'superset'.
// Answers: "is the topic of 'subset' already covered by 'superset'?"
func containment(subset, superset string) float64 {
	subWords := wordSet(subset)
	superWords := wordSet(superset)
	if len(subWords) == 0 {
		return 0
	}

	intersection := 0
	for w := range subWords {
		if superWords[w] {
			intersection++
		}
	}

	return float64(intersection) / float64(len(subWords))
}

// wordSet splits text into a set of lowercase words, stripping punctuation.
func wordSet(text string) map[string]bool {
	set := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		// Strip common punctuation
		w = strings.Trim(w, ".,;:!?\"'()-[]{}/*")
		if len(w) > 1 { // skip single chars
			set[w] = true
		}
	}
	return set
}

// recallRelated searches for existing active artifacts related to the given query.
// Pass title + signal for better recall. Returns a markdown section or empty string.
func recallRelated(ctx context.Context, store ArtifactStore, query string) string {
	if store == nil || query == "" {
		return ""
	}

	results, err := store.Search(ctx, query, 5)
	if err != nil || len(results) == 0 {
		return ""
	}

	// Filter to active only, skip self-matches by checking creation time (just created = skip)
	var related []*Artifact
	for _, r := range results {
		if r.Meta.Status != StatusActive && r.Meta.Status != StatusRefreshDue {
			continue
		}
		related = append(related, r)
		if len(related) >= 3 {
			break
		}
	}

	if len(related) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Related History\n\n")
	for _, r := range related {
		sb.WriteString(fmt.Sprintf("- [%s] **%s** `%s`\n", r.Meta.Kind, r.Meta.Title, r.Meta.ID))
	}
	return sb.String()
}

// FindActivePortfolio returns the most recent active SolutionPortfolio for a context.
func FindActivePortfolio(ctx context.Context, store ArtifactStore, contextName string) (*Artifact, error) {
	var portfolios []*Artifact
	var err error

	if contextName != "" {
		all, e := store.ListByContext(ctx, contextName)
		if e != nil {
			return nil, e
		}
		for _, a := range all {
			if a.Meta.Kind == KindSolutionPortfolio && a.Meta.Status == StatusActive {
				portfolios = append(portfolios, a)
			}
		}
	} else {
		portfolios, err = store.ListByKind(ctx, KindSolutionPortfolio, 1)
		if err != nil {
			return nil, err
		}
	}

	if len(portfolios) == 0 {
		return nil, nil
	}
	return portfolios[0], nil
}

// FormatSolutionResponse builds the MCP tool response.
func FormatSolutionResponse(action string, a *Artifact, filePath string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "explore":
		sb.WriteString(fmt.Sprintf("Portfolio created: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
	case "compare":
		sb.WriteString(fmt.Sprintf("Comparison added to: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))

		// Extract non-dominated set from body
		if idx := strings.Index(a.Body, "**Pareto front:**"); idx != -1 {
			end := strings.Index(a.Body[idx:], "\n")
			if end > 0 {
				sb.WriteString(a.Body[idx:idx+end] + "\n")
			}
		}
		if idx := strings.Index(a.Body, "**Recommended:**"); idx != -1 {
			end := strings.Index(a.Body[idx:], "\n")
			if end > 0 {
				sb.WriteString(a.Body[idx:idx+end] + "\n")
			}
		}
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// FormatMissingProblemResponse returns prescriptive guidance when problem is missing.
func FormatMissingProblemResponse(navStrip string) string {
	return "No active ProblemCard found.\n\n" +
		"Frame the problem first:\n" +
		"  /q-frame — define what's anomalous, constraints, acceptance criteria\n\n" +
		"Or explore directly in tactical mode:\n" +
		"  quint_solution(action=\"explore\", variants=[...])\n" +
		"  → will create a lightweight ProblemCard from context\n" +
		navStrip
}
