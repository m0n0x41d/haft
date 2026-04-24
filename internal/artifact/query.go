package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const defaultCommissionAttentionAfter = 24 * time.Hour

const defaultCommissionLeaseAttentionAfter = 2 * time.Hour

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

// ProblemAdoptionRefs captures the linked artifacts needed to resume work on a problem.
type ProblemAdoptionRefs struct {
	PortfolioRef         string
	ComparedPortfolioRef string
	DecisionRef          string
}

// ResolveProblemAdoptionRefs discovers linked artifacts for problem adoption by
// traversing artifact relationships rather than searching carriers with FTS.
func ResolveProblemAdoptionRefs(ctx context.Context, store ArtifactStore, problemRef string) ProblemAdoptionRefs {
	targetRef := strings.TrimSpace(problemRef)
	if store == nil || targetRef == "" {
		return ProblemAdoptionRefs{}
	}

	relatedArtifacts := relatedArtifactsByTarget(ctx, store, targetRef)
	portfolioCandidates := filterArtifactsByKind(relatedArtifacts, KindSolutionPortfolio)
	visiblePortfolios := filterArtifactsByStatus(portfolioCandidates, adoptionIncludesStatus)

	comparedPortfolio := selectLatestArtifact(visiblePortfolios, func(item *Artifact) bool {
		return ResolveComparedPortfolioRef(ctx, store, item.Meta.ID) != ""
	})

	portfolio := comparedPortfolio
	if portfolio == nil {
		portfolio = selectLatestArtifact(visiblePortfolios, func(*Artifact) bool { return true })
	}

	decisionCandidates := decisionCandidatesForAdoption(ctx, store, targetRef, portfolio)

	decision := selectLatestArtifact(decisionCandidates, func(*Artifact) bool { return true })

	refs := ProblemAdoptionRefs{}
	if portfolio != nil {
		refs.PortfolioRef = portfolio.Meta.ID
	}
	if comparedPortfolio != nil {
		refs.ComparedPortfolioRef = comparedPortfolio.Meta.ID
	}
	if decision != nil {
		refs.DecisionRef = decision.Meta.ID
	}

	return refs
}

func decisionCandidatesForAdoption(
	ctx context.Context,
	store ArtifactStore,
	problemRef string,
	portfolio *Artifact,
) []*Artifact {
	if portfolio != nil {
		return decisionsLinkedToTarget(ctx, store, portfolio.Meta.ID)
	}

	return decisionsLinkedToTarget(ctx, store, problemRef)
}

// StatusData holds all data needed to render the status dashboard.
type StatusData struct {
	HealthyDecisions    []*Artifact
	PendingDecisions    []*Artifact
	UnassessedDecisions []*Artifact
	DecisionHealth      map[string]DecisionHealth // decision ID -> derived maturity/freshness
	StaleItems          []StaleItem
	OpenCommissions     []WorkCommissionStatus
	CommissionAttention []WorkCommissionStatus
	InProgressProblems  []*Artifact
	InProgressBy        map[string]string // problem ID -> portfolio ID
	BacklogProblems     []*Artifact
	AddressedProblems   []*Artifact
	AddressedBy         map[string]string // problem ID -> decision ID
	RecentNotes         []*Artifact
}

// FetchStatusData gathers all dashboard data without formatting.
func FetchStatusData(ctx context.Context, store ArtifactStore, contextFilter string) (StatusData, error) {
	var data StatusData
	data.InProgressBy = make(map[string]string)
	data.AddressedBy = make(map[string]string)
	data.DecisionHealth = make(map[string]DecisionHealth)

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
		health := DeriveDecisionHealth(ctx, store, d.Meta.ID)
		data.DecisionHealth[d.Meta.ID] = health

		if health.Maturity == DecisionMaturityUnassessed {
			data.UnassessedDecisions = append(data.UnassessedDecisions, d)
			continue
		}

		if health.Maturity == DecisionMaturityPending {
			data.PendingDecisions = append(data.PendingDecisions, d)
			continue
		}

		if health.Freshness == DecisionFreshnessHealthy {
			data.HealthyDecisions = append(data.HealthyDecisions, d)
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

	if commissions, err := FetchWorkCommissionStatuses(ctx, store); err == nil {
		for _, commission := range commissions {
			if commission.Terminal {
				continue
			}
			data.OpenCommissions = append(data.OpenCommissions, commission)
			if commission.AttentionReason != "" {
				data.CommissionAttention = append(data.CommissionAttention, commission)
			}
		}
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

type WorkCommissionStatus struct {
	ID               string
	State            string
	DecisionRef      string
	PlanRef          string
	ValidUntil       string
	FetchedAt        string
	Terminal         bool
	Expired          bool
	AttentionReason  string
	SuggestedActions []string
}

func FetchWorkCommissionStatuses(ctx context.Context, store ArtifactStore) ([]WorkCommissionStatus, error) {
	items, err := store.ListByKind(ctx, KindWorkCommission, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	statuses := make([]WorkCommissionStatus, 0, len(items))
	for _, item := range items {
		full, err := store.Get(ctx, item.Meta.ID)
		if err != nil {
			return nil, err
		}

		payload := map[string]any{}
		if err := json.Unmarshal([]byte(full.StructuredData), &payload); err != nil {
			return nil, fmt.Errorf("decode WorkCommission %s: %w", item.Meta.ID, err)
		}
		if textField(payload, "id") == "" {
			payload["id"] = item.Meta.ID
		}

		statuses = append(statuses, workCommissionStatusFromPayload(payload, now))
	}

	return statuses, nil
}

func workCommissionStatusFromPayload(payload map[string]any, now time.Time) WorkCommissionStatus {
	state := textField(payload, "state")
	terminal := workCommissionStateTerminal(state)
	expired := workCommissionStateExpired(payload, now)
	attentionReason := workCommissionStatusAttentionReason(payload, now, terminal, expired)

	return WorkCommissionStatus{
		ID:               textField(payload, "id"),
		State:            state,
		DecisionRef:      textField(payload, "decision_ref"),
		PlanRef:          textField(payload, "implementation_plan_ref"),
		ValidUntil:       textField(payload, "valid_until"),
		FetchedAt:        textField(payload, "fetched_at"),
		Terminal:         terminal,
		Expired:          expired,
		AttentionReason:  attentionReason,
		SuggestedActions: workCommissionStatusSuggestedActions(state, attentionReason),
	}
}

func workCommissionStatusAttentionReason(
	payload map[string]any,
	now time.Time,
	terminal bool,
	expired bool,
) string {
	if terminal {
		return ""
	}
	if expired {
		return "expired before terminal state"
	}

	state := textField(payload, "state")
	switch state {
	case "blocked_stale", "blocked_policy", "blocked_conflict", "needs_human_review", "failed":
		return "requires operator decision: " + state
	case "preflighting", "running":
		return workCommissionLeaseAttentionReason(payload, now)
	default:
		return workCommissionOpenAttentionReason(payload, now)
	}
}

func workCommissionLeaseAttentionReason(payload map[string]any, now time.Time) string {
	lease, ok := objectField(payload, "lease")
	if !ok {
		return "active state has no lease"
	}

	claimedAt, ok := timeField(lease, "claimed_at")
	if !ok {
		return "active lease has no claimed_at"
	}
	if now.Sub(claimedAt) >= defaultCommissionLeaseAttentionAfter {
		return "active lease older than " + defaultCommissionLeaseAttentionAfter.String()
	}
	return ""
}

func workCommissionOpenAttentionReason(payload map[string]any, now time.Time) string {
	fetchedAt, ok := timeField(payload, "fetched_at")
	if !ok {
		return "open commission has no fetched_at"
	}
	if now.Sub(fetchedAt) >= defaultCommissionAttentionAfter {
		return "open longer than " + defaultCommissionAttentionAfter.String()
	}
	return ""
}

func workCommissionStatusSuggestedActions(state string, reason string) []string {
	if reason == "" {
		return nil
	}

	switch state {
	case "preflighting", "running":
		return []string{"inspect", "requeue", "cancel"}
	case "blocked_stale":
		return []string{"refresh_decision", "requeue", "cancel"}
	case "blocked_policy", "blocked_conflict", "needs_human_review", "failed":
		return []string{"inspect", "requeue", "cancel"}
	default:
		return []string{"inspect", "cancel"}
	}
}

func workCommissionStateTerminal(state string) bool {
	switch state {
	case "completed", "completed_with_projection_debt", "cancelled", "expired":
		return true
	default:
		return false
	}
}

func workCommissionStateExpired(payload map[string]any, now time.Time) bool {
	validUntil, ok := timeField(payload, "valid_until")
	return ok && !validUntil.After(now)
}

func textField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func objectField(payload map[string]any, key string) (map[string]any, bool) {
	value, ok := payload[key]
	if !ok {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}

func timeField(payload map[string]any, key string) (time.Time, bool) {
	value := textField(payload, key)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
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

func relatedArtifactsByTarget(ctx context.Context, store ArtifactStore, targetRef string) []*Artifact {
	backlinks, err := store.GetBacklinks(ctx, targetRef)
	if err != nil {
		return nil
	}

	artifacts := make([]*Artifact, 0, len(backlinks))

	for _, backlink := range backlinks {
		if backlink.Type != "based_on" {
			continue
		}

		artifactItem, err := store.Get(ctx, backlink.Ref)
		if err != nil {
			continue
		}

		artifacts = appendUniqueArtifacts(artifacts, artifactItem)
	}

	sortArtifactsNewestFirst(artifacts)
	return artifacts
}

func decisionsLinkedToTarget(ctx context.Context, store ArtifactStore, targetRef string) []*Artifact {
	relatedArtifacts := relatedArtifactsByTarget(ctx, store, targetRef)
	decisionArtifacts := filterArtifactsByKind(relatedArtifacts, KindDecisionRecord)
	decisionArtifacts = filterArtifactsByStatus(decisionArtifacts, adoptionIncludesStatus)

	return decisionArtifacts
}

func filterArtifactsByKind(artifacts []*Artifact, kind Kind) []*Artifact {
	result := make([]*Artifact, 0, len(artifacts))

	for _, artifactItem := range artifacts {
		if artifactItem.Meta.Kind != kind {
			continue
		}

		result = append(result, artifactItem)
	}

	return result
}

func filterArtifactsByStatus(artifacts []*Artifact, include func(Status) bool) []*Artifact {
	result := make([]*Artifact, 0, len(artifacts))

	for _, artifactItem := range artifacts {
		if !include(artifactItem.Meta.Status) {
			continue
		}

		result = append(result, artifactItem)
	}

	return result
}

func appendUniqueArtifacts(existing []*Artifact, candidates ...*Artifact) []*Artifact {
	seen := make(map[string]struct{}, len(existing))

	for _, item := range existing {
		seen[item.Meta.ID] = struct{}{}
	}

	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if _, ok := seen[candidate.Meta.ID]; ok {
			continue
		}

		seen[candidate.Meta.ID] = struct{}{}
		existing = append(existing, candidate)
	}

	return existing
}

func sortArtifactsNewestFirst(artifacts []*Artifact) {
	sort.SliceStable(artifacts, func(left int, right int) bool {
		leftCreated := artifacts[left].Meta.CreatedAt
		rightCreated := artifacts[right].Meta.CreatedAt

		if leftCreated.Equal(rightCreated) {
			return artifacts[left].Meta.ID < artifacts[right].Meta.ID
		}

		return leftCreated.After(rightCreated)
	})
}

func selectLatestArtifact(artifacts []*Artifact, include func(*Artifact) bool) *Artifact {
	for _, artifactItem := range artifacts {
		if !include(artifactItem) {
			continue
		}

		return artifactItem
	}

	return nil
}

func adoptionIncludesStatus(status Status) bool {
	return status == StatusActive || status == StatusRefreshDue
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
