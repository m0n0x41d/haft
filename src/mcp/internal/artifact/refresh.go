package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RefreshAction is what the user wants to do with a stale decision.
type RefreshAction string

const (
	RefreshScan      RefreshAction = "scan"
	RefreshWaive     RefreshAction = "waive"
	RefreshReopen    RefreshAction = "reopen"
	RefreshSupersede RefreshAction = "supersede"
	RefreshDeprecate RefreshAction = "deprecate"
)

// RefreshInput is the input for refresh operations.
type RefreshInput struct {
	Action      RefreshAction `json:"action"`
	DecisionRef string        `json:"decision_ref,omitempty"`
	Reason      string        `json:"reason,omitempty"`
	NewValidUntil string      `json:"new_valid_until,omitempty"`
	Evidence    string        `json:"evidence,omitempty"`
	Context     string        `json:"context,omitempty"`
}

// StaleItem describes one stale decision with details.
type StaleItem struct {
	ID         string
	Title      string
	Reason     string
	ValidUntil string
	DaysStale  int
}

// ScanStale finds all stale decisions and returns actionable info.
func ScanStale(ctx context.Context, store *Store) ([]StaleItem, error) {
	stale, err := store.FindStaleDecisions(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var items []StaleItem

	for _, a := range stale {
		item := StaleItem{
			ID:    a.Meta.ID,
			Title: a.Meta.Title,
		}

		if a.Meta.Status == StatusRefreshDue {
			item.Reason = "manually marked refresh_due"
		}

		if a.Meta.ValidUntil != "" {
			item.ValidUntil = a.Meta.ValidUntil
			if t, err := time.Parse(time.RFC3339, a.Meta.ValidUntil); err == nil {
				if t.Before(now) {
					days := int(now.Sub(t).Hours() / 24)
					item.DaysStale = days
					item.Reason = fmt.Sprintf("expired %d day(s) ago (%s)", days, t.Format("2006-01-02"))
				}
			}
		}

		if item.Reason == "" {
			item.Reason = "refresh_due"
		}

		items = append(items, item)
	}

	return items, nil
}

// WaiveDecision extends a decision's validity with justification.
func WaiveDecision(ctx context.Context, store *Store, quintDir string, decisionRef, reason, newValidUntil, evidence string) (*Artifact, error) {
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord {
		return nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, a.Meta.Kind)
	}

	if newValidUntil == "" {
		// Default: extend 90 days from now
		newValidUntil = time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339)
	}

	a.Meta.ValidUntil = newValidUntil
	a.Meta.Status = StatusActive

	// Append waiver to body
	waiver := fmt.Sprintf("\n## Waiver (%s)\n\n", time.Now().UTC().Format("2006-01-02"))
	waiver += fmt.Sprintf("**Extended until:** %s\n", newValidUntil)
	waiver += fmt.Sprintf("**Reason:** %s\n", reason)
	if evidence != "" {
		waiver += fmt.Sprintf("**Evidence:** %s\n", evidence)
	}
	a.Body += waiver

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update decision: %w", err)
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// ReopenDecision marks a decision as refresh_due and creates a new ProblemCard linked to it.
func ReopenDecision(ctx context.Context, store *Store, quintDir string, decisionRef, reason string) (*Artifact, *Artifact, error) {
	dec, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, nil, fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if dec.Meta.Kind != KindDecisionRecord {
		return nil, nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, dec.Meta.Kind)
	}

	// Mark as refresh_due
	dec.Meta.Status = StatusRefreshDue
	if err := store.Update(ctx, dec); err != nil {
		return nil, nil, fmt.Errorf("update decision: %w", err)
	}
	writeFileQuiet(quintDir, dec)

	// Create a new ProblemCard linked to the old decision
	newProb, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title:   fmt.Sprintf("Revisit: %s", strings.TrimPrefix(dec.Meta.Title, "Decision: ")),
		Signal:  fmt.Sprintf("Decision %s needs re-evaluation: %s", decisionRef, reason),
		Context: dec.Meta.Context,
	})
	if err != nil {
		return dec, nil, fmt.Errorf("create new problem: %w", err)
	}

	// Link new problem to old decision
	store.AddLink(ctx, newProb.Meta.ID, decisionRef, "revisits")

	return dec, newProb, nil
}

// SupersedeDecision marks a decision as superseded by another.
func SupersedeDecision(ctx context.Context, store *Store, quintDir string, decisionRef, newDecisionRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord {
		return nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, a.Meta.Kind)
	}

	a.Meta.Status = StatusSuperseded
	a.Body += fmt.Sprintf("\n## Superseded (%s)\n\n**Replaced by:** %s\n**Reason:** %s\n",
		time.Now().UTC().Format("2006-01-02"), newDecisionRef, reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update decision: %w", err)
	}

	if newDecisionRef != "" {
		store.AddLink(ctx, newDecisionRef, decisionRef, "supersedes")
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// DeprecateDecision marks a decision as deprecated (no longer relevant).
func DeprecateDecision(ctx context.Context, store *Store, quintDir string, decisionRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord {
		return nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, a.Meta.Kind)
	}

	a.Meta.Status = StatusDeprecated
	a.Body += fmt.Sprintf("\n## Deprecated (%s)\n\n**Reason:** %s\n",
		time.Now().UTC().Format("2006-01-02"), reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update decision: %w", err)
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// CreateRefreshReport creates a RefreshReport artifact summarizing what was done.
func CreateRefreshReport(ctx context.Context, store *Store, quintDir string, decisionRef, action, reason, outcome string) (*Artifact, error) {
	seq, err := store.NextSequence(ctx, KindRefreshReport)
	if err != nil {
		return nil, err
	}

	id := GenerateID(KindRefreshReport, seq)
	now := time.Now().UTC()

	var body strings.Builder
	body.WriteString(fmt.Sprintf("# Refresh Report\n\n"))
	body.WriteString(fmt.Sprintf("## Decision\n\n%s\n\n", decisionRef))
	body.WriteString(fmt.Sprintf("## Action\n\n%s\n\n", action))
	body.WriteString(fmt.Sprintf("## Reason\n\n%s\n\n", reason))
	body.WriteString(fmt.Sprintf("## Outcome\n\n%s\n", outcome))

	a := &Artifact{
		Meta: Meta{
			ID:        id,
			Kind:      KindRefreshReport,
			Version:   1,
			Status:    StatusActive,
			Title:     fmt.Sprintf("Refresh: %s", decisionRef),
			CreatedAt: now,
			UpdatedAt: now,
			Links:     []Link{{Ref: decisionRef, Type: "refreshes"}},
		},
		Body: body.String(),
	}

	if err := store.Create(ctx, a); err != nil {
		return nil, err
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// FormatScanResponse formats the stale scan results.
func FormatScanResponse(items []StaleItem, navStrip string) string {
	var sb strings.Builder

	if len(items) == 0 {
		sb.WriteString("No stale decisions found. All decisions are current.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Refresh Due (%d decision(s))\n\n", len(items)))
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s]\n", i+1, item.Title, item.ID))
		sb.WriteString(fmt.Sprintf("   Reason: %s\n\n", item.Reason))
	}

	sb.WriteString("**Actions:**\n")
	sb.WriteString("- `waive` — extend validity with justification\n")
	sb.WriteString("- `reopen` — start new problem cycle linked to old decision\n")
	sb.WriteString("- `supersede` — replace with a new decision\n")
	sb.WriteString("- `deprecate` — mark as no longer relevant\n")

	sb.WriteString(navStrip)
	return sb.String()
}

// FormatRefreshActionResponse formats the result of a refresh action.
func FormatRefreshActionResponse(action RefreshAction, dec *Artifact, newProb *Artifact, navStrip string) string {
	var sb strings.Builder

	switch action {
	case RefreshWaive:
		sb.WriteString(fmt.Sprintf("Waived: %s\n", dec.Meta.Title))
		sb.WriteString(fmt.Sprintf("New valid_until: %s\n", dec.Meta.ValidUntil))
	case RefreshReopen:
		sb.WriteString(fmt.Sprintf("Reopened: %s → status: refresh_due\n", dec.Meta.Title))
		if newProb != nil {
			sb.WriteString(fmt.Sprintf("New ProblemCard: %s (%s)\n", newProb.Meta.Title, newProb.Meta.ID))
			sb.WriteString("Use /q-explore to find new solutions.\n")
		}
	case RefreshSupersede:
		sb.WriteString(fmt.Sprintf("Superseded: %s\n", dec.Meta.Title))
	case RefreshDeprecate:
		sb.WriteString(fmt.Sprintf("Deprecated: %s\n", dec.Meta.Title))
	}

	sb.WriteString(navStrip)
	return sb.String()
}
