package artifact

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/logger"
)

// RefreshAction is what the user wants to do with a stale decision.
type RefreshAction string

const (
	RefreshScan      RefreshAction = "scan"
	RefreshWaive     RefreshAction = "waive"
	RefreshReopen    RefreshAction = "reopen"
	RefreshSupersede RefreshAction = "supersede"
	RefreshDeprecate RefreshAction = "deprecate"
	RefreshReconcile RefreshAction = "reconcile"
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
// If projectRoot is non-empty, also checks for file drift on baselined decisions.
func ScanStale(ctx context.Context, store ArtifactStore, projectRoot ...string) ([]StaleItem, error) {
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
			reason := fmt.Sprintf("evidence degraded (R_eff: %.2f)", wlnk.REff)
			if wlnk.REff < 0.3 {
				reason = fmt.Sprintf("AT RISK — evidence degraded (R_eff: %.2f) — consider reopen or supersede", wlnk.REff)
			}
			items = append(items, StaleItem{
				ID:     d.Meta.ID,
				Title:  d.Meta.Title,
				Kind:   string(d.Meta.Kind),
				Reason: reason,
			})
		}
	}

	// Check for file drift if projectRoot is provided
	if len(projectRoot) > 0 && projectRoot[0] != "" {
		driftReports, driftErr := CheckDrift(ctx, store, projectRoot[0])
		if driftErr == nil {
			for _, r := range driftReports {
				if seen[r.DecisionID] {
					continue
				}
				if !r.HasBaseline {
					reason := fmt.Sprintf("no baseline — %d file(s) unmonitored", len(r.Files))
					if r.LikelyImplemented {
						reason += " — git activity detected after decision date"
					}
					items = append(items, StaleItem{
						ID:     r.DecisionID,
						Title:  r.DecisionTitle,
						Kind:   string(KindDecisionRecord),
						Reason: reason,
					})
				} else {
					// Count drifted files
					drifted := 0
					missing := 0
					for _, f := range r.Files {
						switch f.Status {
						case DriftModified:
							drifted++
						case DriftMissing:
							missing++
						}
					}
					reason := fmt.Sprintf("code drift — %d file(s) modified", drifted)
					if missing > 0 {
						reason += fmt.Sprintf(", %d file(s) missing", missing)
					}
					items = append(items, StaleItem{
						ID:     r.DecisionID,
						Title:  r.DecisionTitle,
						Kind:   string(KindDecisionRecord),
						Reason: reason,
					})
				}
			}
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
// BuildWaiverSection builds the waiver markdown to append. Pure.
func BuildWaiverSection(now time.Time, newValidUntil, reason, evidence string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Waiver (%s)\n\n", now.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Extended until:** %s\n", newValidUntil))
	sb.WriteString(fmt.Sprintf("**Reason:** %s\n", reason))
	if evidence != "" {
		sb.WriteString(fmt.Sprintf("**Evidence:** %s\n", evidence))
	}
	return sb.String()
}

// WaiveArtifact extends an artifact's validity. Orchestrates effects.
func WaiveArtifact(ctx context.Context, store ArtifactStore, quintDir string, artifactRef, reason, newValidUntil, evidence string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
	}

	now := time.Now().UTC()
	if newValidUntil == "" {
		newValidUntil = now.Add(90 * 24 * time.Hour).Format(time.RFC3339)
	}

	a.Meta.ValidUntil = newValidUntil
	a.Meta.Status = StatusActive
	a.Body += BuildWaiverSection(now, newValidUntil, reason, evidence)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update artifact: %w", err)
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// LineageSource holds pre-fetched data for building lineage notes. Pure input.
type LineageSource struct {
	DecisionRef    string
	Reason         string
	LinkedProblems []LinkedProblem // problems linked via "based_on"
	EvidenceItems  []EvidenceItem
}

// LinkedProblem holds a pre-fetched problem's body for characterization extraction.
type LinkedProblem struct {
	ID   string
	Body string
}

// BuildLineageNotes constructs lineage markdown from pre-fetched sources. Pure.
func BuildLineageNotes(src LineageSource) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n## Lineage from %s\n\n", src.DecisionRef))
	sb.WriteString(fmt.Sprintf("**Reopen reason:** %s\n\n", src.Reason))

	for _, prob := range src.LinkedProblems {
		// Extract latest versioned characterization
		for i := 100; i >= 1; i-- {
			marker := fmt.Sprintf("## Characterization v%d", i)
			if section := extractSection(prob.Body, fmt.Sprintf("Characterization v%d", i)); section != "" {
				_ = marker // used for documentation only
				sb.WriteString(fmt.Sprintf("**Prior characterization (from %s):**\n%s\n", prob.ID, section))
				break
			}
		}
		// Old-style characterization
		if section := extractSection(prob.Body, "Comparison Dimensions"); section != "" {
			sb.WriteString(fmt.Sprintf("**Prior characterization (from %s):**\n%s\n", prob.ID, section))
		}
	}

	if len(src.EvidenceItems) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Prior evidence (%d items):**\n", len(src.EvidenceItems)))
		for _, e := range src.EvidenceItems {
			sb.WriteString(fmt.Sprintf("- [%s] %s", e.Type, truncate(e.Content, 80)))
			if e.CarrierRef != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", e.CarrierRef))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ReopenDecision marks a decision as refresh_due and creates a new ProblemCard linked to it.
// Orchestrates effects around BuildLineageNotes.
func ReopenDecision(ctx context.Context, store ArtifactStore, quintDir string, decisionRef, reason string) (*Artifact, *Artifact, error) {
	dec, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, nil, fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if dec.Meta.Kind != KindDecisionRecord {
		return nil, nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, dec.Meta.Kind)
	}

	// Effect: mark as refresh_due
	dec.Meta.Status = StatusRefreshDue
	if err := store.Update(ctx, dec); err != nil {
		return nil, nil, fmt.Errorf("update decision: %w", err)
	}
	writeFileQuiet(quintDir, dec)

	// Effect: pre-fetch linked problems for lineage
	var linkedProblems []LinkedProblem
	for _, link := range dec.Meta.Links {
		if link.Type != "based_on" {
			continue
		}
		origArt, err := store.Get(ctx, link.Ref)
		if err != nil || origArt.Meta.Kind != KindProblemCard {
			continue
		}
		linkedProblems = append(linkedProblems, LinkedProblem{ID: origArt.Meta.ID, Body: origArt.Body})
	}

	// Effect: pre-fetch evidence
	evidenceItems, _ := store.GetEvidenceItems(ctx, decisionRef)

	// Pure: build lineage
	lineage := BuildLineageNotes(LineageSource{
		DecisionRef:    decisionRef,
		Reason:         reason,
		LinkedProblems: linkedProblems,
		EvidenceItems:  evidenceItems,
	})

	// Effect: create new ProblemCard with lineage
	signal := fmt.Sprintf("Decision %s needs re-evaluation: %s", decisionRef, reason)
	newProb, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title:   fmt.Sprintf("Revisit: %s", strings.TrimPrefix(dec.Meta.Title, "Decision: ")),
		Signal:  signal,
		Context: dec.Meta.Context,
	})
	if err != nil {
		return dec, nil, fmt.Errorf("create new problem: %w", err)
	}

	newProb.Body += lineage
	if err := store.Update(ctx, newProb); err != nil {
		logger.Warn().Err(err).Str("problem", newProb.Meta.ID).Msg("failed to append lineage to new problem")
	}
	writeFileQuiet(quintDir, newProb)

	if err := store.AddLink(ctx, newProb.Meta.ID, decisionRef, "revisits"); err != nil {
		logger.Warn().Err(err).Str("problem", newProb.Meta.ID).Str("decision", decisionRef).Msg("failed to link problem to decision")
	}

	return dec, newProb, nil
}

// SupersedeArtifact marks an artifact as superseded by another.
// Works on any artifact kind (notes, problems, decisions, etc.).
// BuildSupersedeSection builds the supersede markdown. Pure.
func BuildSupersedeSection(now time.Time, newArtifactRef, reason string) string {
	return fmt.Sprintf("\n## Superseded (%s)\n\n**Replaced by:** %s\n**Reason:** %s\n",
		now.Format("2006-01-02"), newArtifactRef, reason)
}

// SupersedeArtifact marks an artifact as superseded by another. Orchestrates effects.
func SupersedeArtifact(ctx context.Context, store ArtifactStore, quintDir string, artifactRef, newArtifactRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
	}

	a.Meta.Status = StatusSuperseded
	a.Body += BuildSupersedeSection(time.Now().UTC(), newArtifactRef, reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update artifact: %w", err)
	}

	if newArtifactRef != "" {
		if err := store.AddLink(ctx, newArtifactRef, artifactRef, "supersedes"); err != nil {
			logger.Warn().Err(err).Str("new", newArtifactRef).Str("old", artifactRef).Msg("failed to create supersedes link")
		}
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// BuildDeprecateSection builds the deprecate markdown. Pure.
func BuildDeprecateSection(now time.Time, reason string) string {
	return fmt.Sprintf("\n## Deprecated (%s)\n\n**Reason:** %s\n",
		now.Format("2006-01-02"), reason)
}

// DeprecateArtifact marks an artifact as deprecated. Orchestrates effects.
func DeprecateArtifact(ctx context.Context, store ArtifactStore, quintDir string, artifactRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
	}

	a.Meta.Status = StatusDeprecated
	a.Body += BuildDeprecateSection(time.Now().UTC(), reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update artifact: %w", err)
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// BuildRefreshReportArtifact constructs a RefreshReport. Pure.
func BuildRefreshReportArtifact(id string, now time.Time, decisionRef, action, reason, outcome string) *Artifact {
	var body strings.Builder
	body.WriteString("# Refresh Report\n\n")
	body.WriteString(fmt.Sprintf("## Decision\n\n%s\n\n", decisionRef))
	body.WriteString(fmt.Sprintf("## Action\n\n%s\n\n", action))
	body.WriteString(fmt.Sprintf("## Reason\n\n%s\n\n", reason))
	body.WriteString(fmt.Sprintf("## Outcome\n\n%s\n", outcome))

	return &Artifact{
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
}

// CreateRefreshReport creates a RefreshReport artifact. Orchestrates effects.
func CreateRefreshReport(ctx context.Context, store ArtifactStore, quintDir string, decisionRef, action, reason, outcome string) (*Artifact, error) {
	seq, err := store.NextSequence(ctx, KindRefreshReport)
	if err != nil {
		return nil, err
	}

	a := BuildRefreshReportArtifact(GenerateID(KindRefreshReport, seq), time.Now().UTC(), decisionRef, action, reason, outcome)

	if err := store.Create(ctx, a); err != nil {
		return nil, err
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// ReconcileOverlap describes a note that overlaps with a decision.
type ReconcileOverlap struct {
	NoteID        string
	NoteTitle     string
	DecisionID    string
	DecisionTitle string
	Similarity    float64
}

// Reconcile scans all active notes against all active decisions for overlaps.
// Returns overlapping pairs sorted by similarity (highest first).
func Reconcile(ctx context.Context, store ArtifactStore) ([]ReconcileOverlap, error) {
	notes, err := store.ListByKind(ctx, KindNote, 500)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	decisions, err := store.ListByKind(ctx, KindDecisionRecord, 500)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}

	var overlaps []ReconcileOverlap

	for _, n := range notes {
		if n.Meta.Status != StatusActive {
			continue
		}

		for _, d := range decisions {
			if d.Meta.Status != StatusActive {
				continue
			}
			// Containment: what fraction of note title words appear in decision title?
			sim := containment(n.Meta.Title, d.Meta.Title)

			if sim > 0.5 {
				overlaps = append(overlaps, ReconcileOverlap{
					NoteID:        n.Meta.ID,
					NoteTitle:     n.Meta.Title,
					DecisionID:    d.Meta.ID,
					DecisionTitle: d.Meta.Title,
					Similarity:    sim,
				})
			}
		}
	}

	// Sort by similarity descending
	sort.Slice(overlaps, func(i, j int) bool {
		return overlaps[i].Similarity > overlaps[j].Similarity
	})

	return overlaps, nil
}
