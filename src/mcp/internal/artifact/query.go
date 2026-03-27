package artifact

import (
	"context"
	"fmt"
	"strings"
)

// QueryInput is the input for query operations.
type QueryInput struct {
	Action  string `json:"action"` // search, status, related
	Query   string `json:"query,omitempty"`
	File    string `json:"file,omitempty"`
	Context string `json:"context,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// QuerySearch performs FTS5 search across all artifacts.
func QuerySearch(ctx context.Context, store ArtifactStore, query string, limit int) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 20
	}

	results, err := store.Search(ctx, query, limit)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s\n", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search: %s (%d results)\n\n", query, len(results)))

	for i, a := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s] `%s`\n", i+1, a.Meta.Title, a.Meta.Kind, a.Meta.ID))
		if a.Meta.Context != "" {
			sb.WriteString(fmt.Sprintf("   Context: %s", a.Meta.Context))
		}
		if a.Meta.Status != StatusActive {
			sb.WriteString(fmt.Sprintf(" | Status: %s", a.Meta.Status))
		}
		sb.WriteString(fmt.Sprintf(" | %s\n", a.Meta.CreatedAt.Format("2006-01-02")))

		// Show first 120 chars of body as preview
		preview := strings.TrimSpace(a.Body)
		// Skip the title line
		if idx := strings.Index(preview, "\n"); idx > 0 {
			preview = strings.TrimSpace(preview[idx:])
		}
		if len(preview) > 120 {
			preview = preview[:117] + "..."
		}
		if preview != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", preview))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// QueryStatus returns a dashboard of active decisions, stale items, and recent notes.
func QueryStatus(ctx context.Context, store ArtifactStore, contextFilter string) (string, error) {
	var sb strings.Builder
	sb.WriteString("## Quint Status\n\n")

	// Active decisions
	var decisions []*Artifact
	if contextFilter != "" {
		all, _ := store.ListByContext(ctx, contextFilter)
		for _, a := range all {
			if a.Meta.Kind == KindDecisionRecord {
				decisions = append(decisions, a)
			}
		}
	} else {
		decisions, _ = store.ListByKind(ctx, KindDecisionRecord, 10)
	}
	activeDecisions := filterActive(decisions)
	if len(activeDecisions) > 0 {
		// Split by measurement status: shipped (has measurement) vs pending (no measurement)
		var shipped, pending []*Artifact
		for _, d := range activeDecisions {
			if hasMeasurement(ctx, store, d.Meta.ID) {
				shipped = append(shipped, d)
			} else {
				pending = append(pending, d)
			}
		}

		formatDecisionList := func(items []*Artifact, cap int) {
			for i, d := range items {
				if i >= cap {
					sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(items)-cap))
					break
				}
				line := fmt.Sprintf("- **%s** `%s`", d.Meta.Title, d.Meta.ID)
				if d.Meta.ValidUntil != "" {
					vu := d.Meta.ValidUntil
					if len(vu) > 10 {
						vu = vu[:10]
					}
					line += fmt.Sprintf(" (valid until %s)", vu)
				}
				sb.WriteString(line + "\n")
			}
		}

		if len(pending) > 0 {
			sb.WriteString(fmt.Sprintf("### Pending Implementation (%d)\n\n", len(pending)))
			formatDecisionList(pending, 5)
			sb.WriteString("\n")
		}

		if len(shipped) > 0 {
			sb.WriteString(fmt.Sprintf("### Shipped (%d)\n\n", len(shipped)))
			formatDecisionList(shipped, 5)
			sb.WriteString("\n")
		}
	}

	// Stale artifacts (filtered by context if set)
	allStaleItems, _ := ScanStale(ctx, store)
	var staleItems []StaleItem
	if contextFilter != "" {
		for _, s := range allStaleItems {
			// Check if artifact belongs to context
			if a, err := store.Get(ctx, s.ID); err == nil && a.Meta.Context == contextFilter {
				staleItems = append(staleItems, s)
			}
		}
	} else {
		staleItems = allStaleItems
	}
	if len(staleItems) > 0 {
		sb.WriteString(fmt.Sprintf("### Refresh Due (%d)\n\n", len(staleItems)))
		cap := 5
		for i, s := range staleItems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more (use /q-refresh to see all)\n", len(staleItems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", s.Title, s.ID, s.Reason))
		}
		sb.WriteString("\n")
	}

	// Active problems — three-way split: Backlog / In Progress / Addressed
	var problems []*Artifact
	if contextFilter != "" {
		all, _ := store.ListByContext(ctx, contextFilter)
		for _, a := range all {
			if a.Meta.Kind == KindProblemCard {
				problems = append(problems, a)
			}
		}
	} else {
		problems, _ = store.ListByKind(ctx, KindProblemCard, 20)
	}
	activeProblems := filterActive(problems)
	var backlogProblems, inProgressProblems, addressedProblems []*Artifact
	addressedBy := make(map[string]string)  // problem ID -> decision ID
	inProgressBy := make(map[string]string) // problem ID -> portfolio ID
	for _, p := range activeProblems {
		backlinks, _ := store.GetBacklinks(ctx, p.Meta.ID)
		hasDecision := false
		hasPortfolio := false
		for _, bl := range backlinks {
			if bl.Type == "based_on" {
				if linked, err := store.Get(ctx, bl.Ref); err == nil {
					if linked.Meta.Kind == KindDecisionRecord {
						hasDecision = true
						addressedBy[p.Meta.ID] = linked.Meta.ID
					} else if linked.Meta.Kind == KindSolutionPortfolio {
						hasPortfolio = true
						inProgressBy[p.Meta.ID] = linked.Meta.ID
					}
				}
			}
		}
		if hasDecision {
			addressedProblems = append(addressedProblems, p)
		} else if hasPortfolio {
			inProgressProblems = append(inProgressProblems, p)
		} else {
			backlogProblems = append(backlogProblems, p)
		}
	}
	if len(inProgressProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### In Progress (%d)\n\n", len(inProgressProblems)))
		cap := 5
		for i, p := range inProgressProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(inProgressProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s` → %s\n", p.Meta.Title, p.Meta.ID, inProgressBy[p.Meta.ID]))
		}
		sb.WriteString("\n")
	}
	if len(backlogProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### Backlog (%d)\n\n", len(backlogProblems)))
		cap := 5
		for i, p := range backlogProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(backlogProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", p.Meta.Title, p.Meta.ID))
		}
		sb.WriteString("\n")
	}
	if len(addressedProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### Addressed (%d)\n\n", len(addressedProblems)))
		cap := 3
		for i, p := range addressedProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(addressedProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s` → %s\n", p.Meta.Title, p.Meta.ID, addressedBy[p.Meta.ID]))
		}
		sb.WriteString("\n")
	}

	// Recent notes (active only, context-filtered if set)
	var notes []*Artifact
	if contextFilter != "" {
		all, _ := store.ListByContext(ctx, contextFilter)
		for _, a := range all {
			if a.Meta.Kind == KindNote && a.Meta.Status == StatusActive {
				notes = append(notes, a)
				if len(notes) >= 5 {
					break
				}
			}
		}
	} else {
		allNotes, _ := store.ListByKind(ctx, KindNote, 20)
		for _, n := range allNotes {
			if n.Meta.Status == StatusActive {
				notes = append(notes, n)
				if len(notes) >= 5 {
					break
				}
			}
		}
	}
	if len(notes) > 0 {
		sb.WriteString(fmt.Sprintf("### Recent Notes (%d)\n\n", len(notes)))
		for _, n := range notes {
			sb.WriteString(fmt.Sprintf("- %s `%s` (%s)\n", n.Meta.Title, n.Meta.ID, n.Meta.CreatedAt.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}

	if len(activeDecisions) == 0 && len(staleItems) == 0 && len(backlogProblems) == 0 && len(inProgressProblems) == 0 && len(addressedProblems) == 0 && len(notes) == 0 {
		sb.WriteString("No artifacts found. Use /q-note or /q-frame to get started.\n")
	}

	return sb.String(), nil
}

// QueryList returns all artifacts of a given kind with full details.
func QueryList(ctx context.Context, store ArtifactStore, kindStr string, limit int) (string, error) {
	if kindStr == "" {
		return "", fmt.Errorf("kind is required — use: Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport")
	}
	if limit <= 0 {
		limit = 50
	}

	kind, err := ParseKind(kindStr)
	if err != nil {
		return "", fmt.Errorf("%w (valid: Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport)", err)
	}
	artifacts, err := store.ListByKind(ctx, kind, limit)
	if err != nil {
		return "", err
	}

	if len(artifacts) == 0 {
		return fmt.Sprintf("No %s artifacts found.\n", kind), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d)\n\n", kind, len(artifacts)))

	for i, a := range artifacts {
		line := fmt.Sprintf("%d. **%s** `%s`", i+1, a.Meta.Title, a.Meta.ID)
		if a.Meta.Status != StatusActive {
			line += fmt.Sprintf(" [%s]", a.Meta.Status)
		}
		if a.Meta.ValidUntil != "" {
			vu := a.Meta.ValidUntil
			if len(vu) > 10 {
				vu = vu[:10]
			}
			line += fmt.Sprintf(" (valid until %s)", vu)
		}
		if a.Meta.Context != "" {
			line += fmt.Sprintf(" ctx:%s", a.Meta.Context)
		}
		sb.WriteString(line + "\n")
	}

	return sb.String(), nil
}

// QueryRelated finds artifacts linked to a specific file path.
func QueryRelated(ctx context.Context, store ArtifactStore, filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}

	results, err := store.SearchByAffectedFile(ctx, filePath)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return fmt.Sprintf("No decisions found affecting: %s\n", filePath), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Decisions affecting: %s\n\n", filePath))

	for _, a := range results {
		sb.WriteString(fmt.Sprintf("- **%s** [%s] `%s`", a.Meta.Title, a.Meta.Kind, a.Meta.ID))
		if a.Meta.Status == StatusRefreshDue {
			sb.WriteString(" ⚠ REFRESH DUE")
		} else if a.Meta.Status == StatusSuperseded {
			sb.WriteString(" (superseded)")
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// hasMeasurement checks if a decision has any measurement evidence (type=measurement, verdict not superseded).
func hasMeasurement(ctx context.Context, store ArtifactStore, decisionID string) bool {
	items, err := store.GetEvidenceItems(ctx, decisionID)
	if err != nil {
		return false
	}
	for _, e := range items {
		if e.Type == "measurement" && e.Verdict != "superseded" {
			return true
		}
	}
	return false
}

func filterActive(artifacts []*Artifact) []*Artifact {
	var result []*Artifact
	for _, a := range artifacts {
		if a.Meta.Status == StatusActive {
			result = append(result, a)
		}
	}
	return result
}
