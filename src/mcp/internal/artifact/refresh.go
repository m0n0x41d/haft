package artifact

import (
	"context"
	"fmt"
	"sort"
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
// ArtifactRef accepts any artifact kind (notes, problems, decisions, etc.).
type RefreshInput struct {
	Action        RefreshAction `json:"action"`
	ArtifactRef   string        `json:"artifact_ref,omitempty"`
	DecisionRef   string        `json:"decision_ref,omitempty"` // deprecated: use ArtifactRef
	Reason        string        `json:"reason,omitempty"`
	NewValidUntil string        `json:"new_valid_until,omitempty"`
	Evidence      string        `json:"evidence,omitempty"`
	Context       string        `json:"context,omitempty"`
}

// ResolveRef returns ArtifactRef if set, otherwise falls back to DecisionRef for backward compat.
func (r RefreshInput) ResolveRef() string {
	if r.ArtifactRef != "" {
		return r.ArtifactRef
	}
	return r.DecisionRef
}

// StaleItem describes one stale artifact with details.
type StaleItem struct {
	ID         string
	Title      string
	Kind       string
	Reason     string
	ValidUntil string
	DaysStale  int
}

// ScanStale finds all stale decisions and returns actionable info.
func ScanStale(ctx context.Context, store *Store) ([]StaleItem, error) {
	// Check both decisions and all other artifact types
	staleDecisions, err := store.FindStaleDecisions(ctx)
	if err != nil {
		return nil, err
	}
	staleOther, _ := store.FindStaleArtifacts(ctx)

	// Merge, dedup by ID
	seen := make(map[string]bool)
	var stale []*Artifact
	for _, a := range staleDecisions {
		if !seen[a.Meta.ID] {
			stale = append(stale, a)
			seen[a.Meta.ID] = true
		}
	}
	for _, a := range staleOther {
		if !seen[a.Meta.ID] {
			stale = append(stale, a)
			seen[a.Meta.ID] = true
		}
	}

	now := time.Now().UTC()
	var items []StaleItem

	for _, a := range stale {
		item := StaleItem{
			ID:    a.Meta.ID,
			Title: a.Meta.Title,
			Kind:  string(a.Meta.Kind),
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
					item.Reason = fmt.Sprintf("expired %d day(s) ago, debt: %.1f (%s)", days, float64(days), t.Format("2006-01-02"))
				}
			}
		}

		if item.Reason == "" {
			item.Reason = "refresh_due"
		}

		items = append(items, item)
	}

	// Check active decisions with evidence for R_eff degradation
	decisions, _ := store.ListByKind(ctx, KindDecisionRecord, 100)
	for _, d := range decisions {
		if d.Meta.Status != StatusActive || seen[d.Meta.ID] {
			continue
		}
		wlnk := ComputeWLNKSummary(ctx, store, d.Meta.ID)
		if !wlnk.HasEvidence {
			continue
		}
		if wlnk.REff < 0.5 {
			items = append(items, StaleItem{
				ID:    d.Meta.ID,
				Title: d.Meta.Title,
				Kind:  string(d.Meta.Kind),
				Reason: fmt.Sprintf("evidence degraded (R_eff: %.2f)", wlnk.REff),
			})
		}
	}

	// Sort by debt descending — most overdue first
	sort.Slice(items, func(i, j int) bool {
		return items[i].DaysStale > items[j].DaysStale
	})

	return items, nil
}

// WaiveArtifact extends an artifact's validity with justification.
// Works on any artifact kind (notes, problems, decisions, etc.).
func WaiveArtifact(ctx context.Context, store *Store, quintDir string, artifactRef, reason, newValidUntil, evidence string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
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
		return nil, fmt.Errorf("update artifact: %w", err)
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

	// Build lineage: carry forward context from previous cycle
	signal := fmt.Sprintf("Decision %s needs re-evaluation: %s", decisionRef, reason)

	// Extract what failed from the old decision body
	var lineageNotes strings.Builder
	lineageNotes.WriteString(fmt.Sprintf("\n## Lineage from %s\n\n", decisionRef))
	lineageNotes.WriteString(fmt.Sprintf("**Reopen reason:** %s\n\n", reason))

	// Carry forward characterization from the original problem if it exists
	if dec.Meta.Links != nil {
		for _, link := range dec.Meta.Links {
			if link.Type == "based_on" {
				origArt, err := store.Get(ctx, link.Ref)
				if err != nil {
					continue
				}
				if origArt.Meta.Kind == KindProblemCard {
					// Extract latest characterization from original problem
					for i := 100; i >= 1; i-- {
						marker := fmt.Sprintf("## Characterization v%d", i)
						if idx := strings.Index(origArt.Body, marker); idx != -1 {
							end := strings.Index(origArt.Body[idx+1:], "\n## ")
							var charSection string
							if end > 0 {
								charSection = origArt.Body[idx : idx+1+end]
							} else {
								charSection = origArt.Body[idx:]
							}
							lineageNotes.WriteString(fmt.Sprintf("**Prior characterization (from %s):**\n%s\n",
								origArt.Meta.ID, strings.TrimSpace(charSection)))
							break
						}
					}
					// Also check old-style characterization
					if strings.Contains(origArt.Body, "## Comparison Dimensions") {
						if idx := strings.Index(origArt.Body, "## Comparison Dimensions"); idx != -1 {
							end := strings.Index(origArt.Body[idx+1:], "\n## ")
							var charSection string
							if end > 0 {
								charSection = origArt.Body[idx : idx+1+end]
							} else {
								charSection = origArt.Body[idx:]
							}
							lineageNotes.WriteString(fmt.Sprintf("**Prior characterization (from %s):**\n%s\n",
								origArt.Meta.ID, strings.TrimSpace(charSection)))
						}
					}
				}
			}
		}
	}

	// Carry forward linked evidence references
	evidenceItems, _ := store.GetEvidenceItems(ctx, decisionRef)
	if len(evidenceItems) > 0 {
		lineageNotes.WriteString(fmt.Sprintf("\n**Prior evidence (%d items):**\n", len(evidenceItems)))
		for _, e := range evidenceItems {
			lineageNotes.WriteString(fmt.Sprintf("- [%s] %s", e.Type, truncate(e.Content, 80)))
			if e.CarrierRef != "" {
				lineageNotes.WriteString(fmt.Sprintf(" (%s)", e.CarrierRef))
			}
			lineageNotes.WriteString("\n")
		}
	}

	// Create a new ProblemCard with lineage
	newProb, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title:   fmt.Sprintf("Revisit: %s", strings.TrimPrefix(dec.Meta.Title, "Decision: ")),
		Signal:  signal,
		Context: dec.Meta.Context,
	})
	if err != nil {
		return dec, nil, fmt.Errorf("create new problem: %w", err)
	}

	// Append lineage to the new problem body
	newProb.Body += lineageNotes.String()
	store.Update(ctx, newProb)
	writeFileQuiet(quintDir, newProb)

	// Link new problem to old decision
	store.AddLink(ctx, newProb.Meta.ID, decisionRef, "revisits")

	return dec, newProb, nil
}

// SupersedeArtifact marks an artifact as superseded by another.
// Works on any artifact kind (notes, problems, decisions, etc.).
func SupersedeArtifact(ctx context.Context, store *Store, quintDir string, artifactRef, newArtifactRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
	}

	a.Meta.Status = StatusSuperseded
	a.Body += fmt.Sprintf("\n## Superseded (%s)\n\n**Replaced by:** %s\n**Reason:** %s\n",
		time.Now().UTC().Format("2006-01-02"), newArtifactRef, reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update artifact: %w", err)
	}

	if newArtifactRef != "" {
		store.AddLink(ctx, newArtifactRef, artifactRef, "supersedes")
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// DeprecateArtifact marks an artifact as deprecated (no longer relevant).
// Works on any artifact kind (notes, problems, decisions, etc.).
func DeprecateArtifact(ctx context.Context, store *Store, quintDir string, artifactRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
	}

	a.Meta.Status = StatusDeprecated
	a.Body += fmt.Sprintf("\n## Deprecated (%s)\n\n**Reason:** %s\n",
		time.Now().UTC().Format("2006-01-02"), reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update artifact: %w", err)
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

	sb.WriteString(fmt.Sprintf("## Refresh Due (%d artifact(s))\n\n", len(items)))
	for i, item := range items {
		kindLabel := item.Kind
		if kindLabel == "" {
			kindLabel = "DecisionRecord"
		}
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s] (%s)\n", i+1, item.Title, item.ID, kindLabel))
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
