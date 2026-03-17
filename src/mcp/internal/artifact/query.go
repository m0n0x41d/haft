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
func QuerySearch(ctx context.Context, store *Store, query string, limit int) (string, error) {
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
func QueryStatus(ctx context.Context, store *Store, contextFilter string) (string, error) {
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
		sb.WriteString(fmt.Sprintf("### Active Decisions (%d)\n\n", len(activeDecisions)))
		cap := 5
		for i, d := range activeDecisions {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(activeDecisions)-cap))
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
		sb.WriteString("\n")
	}

	// Stale decisions
	staleItems, _ := ScanStale(ctx, store)
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

	// Active problems — split into open (no decision) and addressed (has linked decision)
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
	var openProblems, addressedProblems []*Artifact
	addressedBy := make(map[string]string) // problem ID -> decision title
	for _, p := range activeProblems {
		backlinks, _ := store.GetBacklinks(ctx, p.Meta.ID)
		decisionFound := false
		for _, bl := range backlinks {
			if bl.Type == "based_on" {
				// Verify it's a decision
				if linked, err := store.Get(ctx, bl.Ref); err == nil && linked.Meta.Kind == KindDecisionRecord {
					decisionFound = true
					addressedBy[p.Meta.ID] = linked.Meta.ID
					break
				}
			}
		}
		if decisionFound {
			addressedProblems = append(addressedProblems, p)
		} else {
			openProblems = append(openProblems, p)
		}
	}
	if len(openProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### Open Problems (%d)\n\n", len(openProblems)))
		cap := 5
		for i, p := range openProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(openProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", p.Meta.Title, p.Meta.ID))
		}
		sb.WriteString("\n")
	}
	if len(addressedProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### Addressed Problems (%d)\n\n", len(addressedProblems)))
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

	// Recent notes
	notes, _ := store.ListByKind(ctx, KindNote, 5)
	if len(notes) > 0 {
		sb.WriteString(fmt.Sprintf("### Recent Notes (%d)\n\n", len(notes)))
		for _, n := range notes {
			sb.WriteString(fmt.Sprintf("- %s `%s` (%s)\n", n.Meta.Title, n.Meta.ID, n.Meta.CreatedAt.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}

	if len(activeDecisions) == 0 && len(staleItems) == 0 && len(openProblems) == 0 && len(addressedProblems) == 0 && len(notes) == 0 {
		sb.WriteString("No artifacts found. Use /q-note or /q-frame to get started.\n")
	}

	return sb.String(), nil
}

// QueryRelated finds artifacts linked to a specific file path.
func QueryRelated(ctx context.Context, store *Store, filePath string) (string, error) {
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

func filterActive(artifacts []*Artifact) []*Artifact {
	var result []*Artifact
	for _, a := range artifacts {
		if a.Meta.Status == StatusActive {
			result = append(result, a)
		}
	}
	return result
}
