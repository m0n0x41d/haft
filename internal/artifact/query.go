package artifact

import (
	"context"
	"fmt"
)

// QueryInput is the input for query operations.
type QueryInput struct {
	Action  string `json:"action"` // search, status, related, projection
	Query   string `json:"query,omitempty"`
	File    string `json:"file,omitempty"`
	Context string `json:"context,omitempty"`
	View    string `json:"view,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// FetchSearchResults performs FTS5 search and returns raw results.
func FetchSearchResults(ctx context.Context, store ArtifactStore, query string, limit int) ([]*Artifact, error) {
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 20
	}
	return store.Search(ctx, query, limit)
}

// StatusData holds all data needed to render the status dashboard.
type StatusData struct {
	ShippedDecisions   []*Artifact
	PendingDecisions   []*Artifact
	StaleItems         []StaleItem
	InProgressProblems []*Artifact
	InProgressBy       map[string]string // problem ID -> portfolio ID
	BacklogProblems    []*Artifact
	AddressedProblems  []*Artifact
	AddressedBy        map[string]string // problem ID -> decision ID
	RecentNotes        []*Artifact
}

// FetchStatusData gathers all dashboard data without formatting.
func FetchStatusData(ctx context.Context, store ArtifactStore, contextFilter string) (StatusData, error) {
	var data StatusData
	data.InProgressBy = make(map[string]string)
	data.AddressedBy = make(map[string]string)

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
	for _, d := range activeDecisions {
		if hasMeasurement(ctx, store, d.Meta.ID) {
			data.ShippedDecisions = append(data.ShippedDecisions, d)
		} else {
			data.PendingDecisions = append(data.PendingDecisions, d)
		}
	}

	// Stale artifacts (filtered by context if set)
	allStaleItems, _ := ScanStale(ctx, store)
	if contextFilter != "" {
		for _, s := range allStaleItems {
			if a, err := store.Get(ctx, s.ID); err == nil && a.Meta.Context == contextFilter {
				data.StaleItems = append(data.StaleItems, s)
			}
		}
	} else {
		data.StaleItems = allStaleItems
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
	for _, p := range activeProblems {
		backlinks, _ := store.GetBacklinks(ctx, p.Meta.ID)
		hasDecision := false
		hasPortfolio := false
		for _, bl := range backlinks {
			if bl.Type == "based_on" {
				if linked, err := store.Get(ctx, bl.Ref); err == nil {
					if linked.Meta.Kind == KindDecisionRecord {
						hasDecision = true
						data.AddressedBy[p.Meta.ID] = linked.Meta.ID
					} else if linked.Meta.Kind == KindSolutionPortfolio {
						hasPortfolio = true
						data.InProgressBy[p.Meta.ID] = linked.Meta.ID
					}
				}
			}
		}
		if hasDecision {
			data.AddressedProblems = append(data.AddressedProblems, p)
		} else if hasPortfolio {
			data.InProgressProblems = append(data.InProgressProblems, p)
		} else {
			data.BacklogProblems = append(data.BacklogProblems, p)
		}
	}

	// Recent notes (active only, context-filtered if set)
	if contextFilter != "" {
		all, _ := store.ListByContext(ctx, contextFilter)
		for _, a := range all {
			if a.Meta.Kind == KindNote && a.Meta.Status == StatusActive {
				data.RecentNotes = append(data.RecentNotes, a)
				if len(data.RecentNotes) >= 5 {
					break
				}
			}
		}
	} else {
		allNotes, _ := store.ListByKind(ctx, KindNote, 20)
		for _, n := range allNotes {
			if n.Meta.Status == StatusActive {
				data.RecentNotes = append(data.RecentNotes, n)
				if len(data.RecentNotes) >= 5 {
					break
				}
			}
		}
	}

	return data, nil
}

// ListData holds data for artifact listing by kind.
type ListData struct {
	Kind      Kind
	Artifacts []*Artifact
}

// FetchListData returns artifacts of a given kind.
func FetchListData(ctx context.Context, store ArtifactStore, kindStr string, limit int) (ListData, error) {
	if kindStr == "" {
		return ListData{}, fmt.Errorf("kind is required — use: Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport")
	}
	if limit <= 0 {
		limit = 50
	}

	kind, err := ParseKind(kindStr)
	if err != nil {
		return ListData{}, fmt.Errorf("%w (valid: Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport)", err)
	}
	artifacts, err := store.ListByKind(ctx, kind, limit)
	if err != nil {
		return ListData{}, err
	}

	return ListData{Kind: kind, Artifacts: artifacts}, nil
}

// FetchRelatedArtifacts finds artifacts linked to a specific file path.
func FetchRelatedArtifacts(ctx context.Context, store ArtifactStore, filePath string) ([]*Artifact, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}
	return store.SearchByAffectedFile(ctx, filePath)
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
