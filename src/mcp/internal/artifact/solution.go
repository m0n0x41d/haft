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
func ExploreSolutions(ctx context.Context, store *Store, quintDir string, input ExploreInput) (*Artifact, string, error) {
	if len(input.Variants) < 2 {
		return nil, "", fmt.Errorf("at least 2 variants required — genuinely distinct options, not variations of one idea")
	}

	for i, v := range input.Variants {
		if v.Title == "" {
			return nil, "", fmt.Errorf("variant %d: title is required", i+1)
		}
		if v.WeakestLink == "" {
			return nil, "", fmt.Errorf("variant %d (%s): weakest_link is required — what bounds this option's quality?", i+1, v.Title)
		}
	}

	// Diversity check: warn on near-identical variants (Jaccard > 0.5)
	var diversityWarnings []string
	for i := 0; i < len(input.Variants); i++ {
		for j := i + 1; j < len(input.Variants); j++ {
			textI := input.Variants[i].Title + " " + input.Variants[i].Description
			textJ := input.Variants[j].Title + " " + input.Variants[j].Description
			sim := jaccardSimilarity(textI, textJ)
			if sim > 0.5 {
				diversityWarnings = append(diversityWarnings,
					fmt.Sprintf("Variants '%s' and '%s' look similar (%.0f%% word overlap) — do they differ in kind, not degree?",
						input.Variants[i].Title, input.Variants[j].Title, sim*100))
			}
		}
	}

	// Resolve problem reference
	var problemTitle string
	var links []Link
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
		if input.Context == "" {
			input.Context = prob.Meta.Context
		}
		if input.Mode == "" {
			input.Mode = string(prob.Meta.Mode)
		}
	}

	seq, err := store.NextSequence(ctx, KindSolutionPortfolio)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindSolutionPortfolio, seq)
	now := time.Now().UTC()

	mode := Mode(input.Mode)
	if mode == "" {
		mode = ModeStandard
	}

	title := "Solution Portfolio"
	if problemTitle != "" {
		title = fmt.Sprintf("Solutions for: %s", problemTitle)
	}

	// Build markdown body
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
			ID:        id,
			Kind:      KindSolutionPortfolio,
			Version:   1,
			Status:    StatusActive,
			Context:   input.Context,
			Mode:      mode,
			Title:     title,
			CreatedAt: now,
			UpdatedAt: now,
			Links:     links,
		},
		Body: body.String(),
	}

	// Append diversity warnings if any
	if len(diversityWarnings) > 0 {
		a.Body += "\n## Diversity Warnings\n\n"
		for _, w := range diversityWarnings {
			a.Body += fmt.Sprintf("- ⚠ %s\n", w)
		}
	}

	// Archive recall: search for related past artifacts
	if recall := recallRelated(ctx, store, title); recall != "" {
		a.Body += recall
	}

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
func CompareSolutions(ctx context.Context, store *Store, quintDir string, input CompareInput) (*Artifact, string, error) {
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

		// Extract characterized dimension names
		charDims := extractCharacterizedDimensions(prob.Body)
		if len(charDims) == 0 {
			break
		}

		// Check dimension coverage: which characterized dims are missing from compare?
		compareDimsLower := make(map[string]bool)
		for _, d := range input.Results.Dimensions {
			compareDimsLower[strings.ToLower(strings.TrimSpace(d))] = true
		}
		var missingDims []string
		for _, cd := range charDims {
			if !compareDimsLower[strings.ToLower(cd)] {
				missingDims = append(missingDims, cd)
			}
		}
		if len(missingDims) > 0 {
			compareWarnings = append(compareWarnings,
				fmt.Sprintf("Characterized dimensions not in comparison: %s", strings.Join(missingDims, ", ")))
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

		// Auto-generate parity checklist from characterized dimensions
		if len(charDims) > 0 {
			compareWarnings = append(compareWarnings, "Parity checklist (per characterized dimension):")
			for _, d := range charDims {
				compareWarnings = append(compareWarnings,
					fmt.Sprintf("  - Is '%s' measured under the same conditions for all variants?", d))
			}
		}

		break // only check first linked problem
	}

	// Build comparison section
	var section strings.Builder
	section.WriteString("\n## Comparison\n\n")

	// Build comparison table
	header := "| Variant |"
	sep := "|---------|"
	for _, d := range input.Results.Dimensions {
		header += fmt.Sprintf(" %s |", d)
		sep += "------|"
	}
	section.WriteString(header + "\n")
	section.WriteString(sep + "\n")

	for variantID, scores := range input.Results.Scores {
		row := fmt.Sprintf("| %s |", variantID)
		for _, d := range input.Results.Dimensions {
			val := scores[d]
			if val == "" {
				val = "-"
			}
			row += fmt.Sprintf(" %s |", val)
		}
		section.WriteString(row + "\n")
	}
	section.WriteString("\n")

	// Non-dominated set
	section.WriteString(fmt.Sprintf("## Non-Dominated Set\n\n**Pareto front:** %s\n\n",
		strings.Join(input.Results.NonDominatedSet, ", ")))

	if len(input.Results.Incomparable) > 0 {
		section.WriteString("**Incomparable pairs:**\n")
		for _, pair := range input.Results.Incomparable {
			section.WriteString(fmt.Sprintf("- %s vs %s\n", pair[0], pair[1]))
		}
		section.WriteString("\n")
	}

	if input.Results.PolicyApplied != "" {
		section.WriteString(fmt.Sprintf("**Selection policy:** %s\n\n", input.Results.PolicyApplied))
	}

	if input.Results.SelectedRef != "" {
		section.WriteString(fmt.Sprintf("**Recommended:** %s\n\n", input.Results.SelectedRef))
	}

	// Remove existing comparison if present, then append
	if idx := strings.Index(a.Body, "\n## Comparison"); idx != -1 {
		a.Body = a.Body[:idx]
	}
	a.Body += section.String()

	// Append warnings to body before saving
	if len(compareWarnings) > 0 {
		a.Body += "\n## Comparison Warnings\n\n"
		for _, w := range compareWarnings {
			a.Body += fmt.Sprintf("- ⚠ %s\n", w)
		}
	}

	if err := store.Update(ctx, a); err != nil {
		return nil, "", fmt.Errorf("update portfolio: %w", err)
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// extractCharacterizedDimensions parses dimension names from the latest
// Characterization table in a ProblemCard body. Returns nil if no characterization found.
func extractCharacterizedDimensions(body string) []string {
	// Find the latest characterization version
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

	// Extract the characterization section
	section := body[lastIdx:]
	if endIdx := strings.Index(section[1:], "\n## "); endIdx != -1 {
		section = section[:endIdx+1]
	}

	// Parse dimension names from markdown table rows
	// Format: | Name | Scale | Unit | Polarity | Measurement |
	var dims []string
	lines := strings.Split(section, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break // end of table
			}
			continue
		}
		// Skip header and separator rows
		if strings.Contains(line, "Dimension") || strings.Contains(line, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		// Extract first column (dimension name)
		parts := strings.SplitN(line, "|", 3)
		if len(parts) >= 3 {
			name := strings.TrimSpace(parts[1])
			if name != "" && name != "-" {
				dims = append(dims, name)
			}
		}
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

// recallRelated searches for existing active artifacts related to the given title.
// Returns a markdown section or empty string if no matches found.
func recallRelated(ctx context.Context, store *Store, title string) string {
	if store == nil || title == "" {
		return ""
	}

	results, err := store.Search(ctx, title, 5)
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
func FindActivePortfolio(ctx context.Context, store *Store, contextName string) (*Artifact, error) {
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
