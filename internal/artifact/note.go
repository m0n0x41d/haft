package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NoteInput is the input for creating a note. Per dec-20260604-26be1e4b a note is
// a single-purpose FACT carrier: Observations are atomic facts, Rationale is
// OPTIONAL (a fact needs only a title + an observation or a source), and Anchors
// are typed edges into the reasoning graph.
type NoteInput struct {
	Title           string           `json:"title"`
	TaskContext     string           `json:"task_context,omitempty"`
	Rationale       string           `json:"rationale,omitempty"`
	Observations    []string         `json:"observations,omitempty"`
	AffectedFiles   []string         `json:"affected_files,omitempty"`
	AffectedSymbols []AffectedSymbol `json:"affected_symbols,omitempty"` // pre-resolved symbol anchors (validated at the shell)
	Anchors         []NoteAnchor     `json:"anchors,omitempty"`
	Evidence        string           `json:"evidence,omitempty"`
	Context         string           `json:"context,omitempty"`
	ValidUntil      string           `json:"valid_until,omitempty"`
	SearchKeywords  string           `json:"search_keywords,omitempty"`
}

// NoteAnchor is a typed edge from a fact-note to a target artifact (decision,
// problem, note, solution) — persisted as a real artifact link and surfaced in
// related/backlinks. The target MUST exist; a dead anchor is rejected, never
// silently kept (dec-20260604-26be1e4b, no-wrong-edge).
type NoteAnchor struct {
	Type string `json:"type"` // governs | about | relates_to | implements | supersedes
	Ref  string `json:"ref"`  // target artifact ID
}

// NoteValidation holds the result of pre-recording checks.
type NoteValidation struct {
	OK        bool
	Warnings  []string
	Conflicts []ConflictInfo
	Suggest   string // suggested alternative action (e.g., "/h-frame")
}

// ConflictInfo describes a conflict with an existing decision.
type ConflictInfo struct {
	DecisionID    string
	DecisionTitle string
	Reason        string
}

// ValidateNote runs the three checks before recording.
func ValidateNote(ctx context.Context, store ArtifactStore, input NoteInput) NoteValidation {
	v := NoteValidation{OK: true}

	// Check 1: not content-free. A note is a fact carrier (dec-20260604-26be1e4b):
	// rationale is OPTIONAL, but a fact needs a title plus at least one of an
	// observation, a source (evidence), or a rationale.
	if strings.TrimSpace(input.Rationale) == "" && len(input.Observations) == 0 && strings.TrimSpace(input.Evidence) == "" {
		v.OK = false
		v.Warnings = append(v.Warnings, "Content-free note. Provide at least one observation, a source, or a rationale.")
		return v
	}
	if words := len(strings.Fields(input.Rationale)); words > 0 && words < 5 && len(input.AffectedFiles) > 0 {
		v.Warnings = append(v.Warnings, fmt.Sprintf("Rationale is very short (%d words) for a change that affects files. Consider expanding.", words))
	}

	// Check 2: Conflict with existing decisions
	if store != nil {
		conflicts := checkConflicts(ctx, store, input)
		if len(conflicts) > 0 {
			v.Conflicts = conflicts
			for _, c := range conflicts {
				v.Warnings = append(v.Warnings, fmt.Sprintf(
					"Potential conflict with active decision %s (%s): %s",
					c.DecisionID, c.DecisionTitle, c.Reason,
				))
			}
		}
	}

	// Check 2b: Overlap with existing decisions (containment-based dedup)
	if store != nil {
		overlap := checkDecisionOverlap(ctx, store, input.Title)
		if overlap != nil {
			if overlap.Similarity > 0.7 {
				v.OK = false
				v.Warnings = append(v.Warnings, fmt.Sprintf(
					"Decision %s (%s) already covers this topic (%.0f%% of note words found in decision). Notes are for observations and discoveries, not architectural choices. Use /h-decide for decisions.",
					overlap.DecisionID, overlap.DecisionTitle, overlap.Similarity*100,
				))
				return v
			}
			v.Warnings = append(v.Warnings, fmt.Sprintf(
				"Similar to decision %s (%s) — %.0f%% word containment. Make sure this note adds information not already in the decision.",
				overlap.DecisionID, overlap.DecisionTitle, overlap.Similarity*100,
			))
		}
	}

	// Check 2c: Overlap with existing notes (same containment check)
	if store != nil {
		overlap := checkNoteOverlap(ctx, store, input.Title)
		if overlap != nil {
			if overlap.Similarity > 0.7 {
				v.OK = false
				v.Warnings = append(v.Warnings, fmt.Sprintf(
					"Note %s (%s) already records this (%.0f%% word containment). Duplicate note rejected.",
					overlap.DecisionID, overlap.DecisionTitle, overlap.Similarity*100,
				))
				return v
			}
			v.Warnings = append(v.Warnings, fmt.Sprintf(
				"Similar to existing note %s (%s) — %.0f%% word containment.",
				overlap.DecisionID, overlap.DecisionTitle, overlap.Similarity*100,
			))
		}
	}

	// Check 3: Shared/generated files — will cause false drift
	v.Warnings = append(v.Warnings, WarnSharedFiles(input.AffectedFiles)...)

	// Check 4: Scope — is this too big for a note?
	if len(input.AffectedFiles) > 3 {
		v.Suggest = "/h-frame"
		v.Warnings = append(v.Warnings, fmt.Sprintf(
			"This note affects %d files. Consider using /h-frame for a proper problem exploration.",
			len(input.AffectedFiles),
		))
	}
	architecturalKeywords := []string{"migrate", "replace", "rewrite", "architecture", "redesign", "overhaul", "rearchitect"}
	titleLower := strings.ToLower(input.Title)
	rationaleLower := strings.ToLower(input.Rationale)
	for _, kw := range architecturalKeywords {
		if strings.Contains(titleLower, kw) || strings.Contains(rationaleLower, kw) {
			v.Suggest = "/h-frame"
			v.Warnings = append(v.Warnings, fmt.Sprintf(
				"This sounds like an architectural change (%q detected). Consider /h-frame instead of a note.",
				kw,
			))
			break
		}
	}

	return v
}

// OverlapInfo describes overlap between a note and an existing decision.
type OverlapInfo struct {
	DecisionID    string
	DecisionTitle string
	Similarity    float64
}

// checkDecisionOverlap finds the most overlapping active decision using containment.
// Containment = "what fraction of the note's words appear in the decision title?"
// Returns the highest overlap if above the warning threshold (0.5), nil otherwise.
func checkDecisionOverlap(ctx context.Context, store ArtifactStore, noteTitle string) *OverlapInfo {
	decisions, err := store.ListByKind(ctx, KindDecisionRecord, 100)
	if err != nil {
		return nil
	}

	var best *OverlapInfo
	bestSim := 0.5 // minimum threshold to report (containment is higher than Jaccard)

	for _, d := range decisions {
		if d.Meta.Status != StatusActive {
			continue
		}
		sim := containment(noteTitle, d.Meta.Title)
		if sim > bestSim {
			bestSim = sim
			best = &OverlapInfo{
				DecisionID:    d.Meta.ID,
				DecisionTitle: d.Meta.Title,
				Similarity:    sim,
			}
		}
	}

	return best
}

// checkNoteOverlap finds the most overlapping active note using containment.
func checkNoteOverlap(ctx context.Context, store ArtifactStore, noteTitle string) *OverlapInfo {
	notes, err := store.ListByKind(ctx, KindNote, 500)
	if err != nil {
		return nil
	}

	var best *OverlapInfo
	bestSim := 0.5

	for _, n := range notes {
		if n.Meta.Status != StatusActive {
			continue
		}
		sim := containment(noteTitle, n.Meta.Title)
		if sim > bestSim {
			bestSim = sim
			best = &OverlapInfo{
				DecisionID:    n.Meta.ID,
				DecisionTitle: n.Meta.Title,
				Similarity:    sim,
			}
		}
	}

	return best
}

func checkConflicts(ctx context.Context, store ArtifactStore, input NoteInput) []ConflictInfo {
	var conflicts []ConflictInfo

	// Search by title keywords
	if input.Title != "" {
		results, err := store.Search(ctx, input.Title, 5)
		if err == nil {
			for _, r := range results {
				if r.Meta.Kind == KindDecisionRecord && r.Meta.Status == StatusActive {
					conflicts = append(conflicts, ConflictInfo{
						DecisionID:    r.Meta.ID,
						DecisionTitle: r.Meta.Title,
						Reason:        "related active decision found by title match",
					})
				}
			}
		}
	}

	// Search by affected files
	for _, file := range input.AffectedFiles {
		results, err := store.SearchByAffectedFile(ctx, file)
		if err == nil {
			for _, r := range results {
				if r.Meta.Kind == KindDecisionRecord && r.Meta.Status == StatusActive {
					// Avoid duplicates
					found := false
					for _, existing := range conflicts {
						if existing.DecisionID == r.Meta.ID {
							found = true
							break
						}
					}
					if !found {
						conflicts = append(conflicts, ConflictInfo{
							DecisionID:    r.Meta.ID,
							DecisionTitle: r.Meta.Title,
							Reason:        fmt.Sprintf("decision affects same file: %s", file),
						})
					}
				}
			}
		}
	}

	return conflicts
}

// CreateNote creates a Note artifact after validation passes.
// BuildNoteArtifact constructs a Note from input. Pure — no side effects.
func BuildNoteArtifact(id string, now time.Time, input NoteInput) *Artifact {
	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n\n", input.Title))
	if len(input.Observations) > 0 {
		body.WriteString("## Observations\n\n")
		for _, o := range input.Observations {
			body.WriteString(fmt.Sprintf("- %s\n", o))
		}
		body.WriteString("\n")
	}
	if strings.TrimSpace(input.Rationale) != "" {
		body.WriteString(fmt.Sprintf("## Rationale\n\n%s\n", input.Rationale))
	}
	if input.Evidence != "" {
		body.WriteString(fmt.Sprintf("\n## Source\n\n%s\n", input.Evidence))
	}
	if len(input.Anchors) > 0 {
		body.WriteString("\n## Anchors\n\n")
		for _, an := range input.Anchors {
			body.WriteString(fmt.Sprintf("- %s `%s`\n", an.Type, an.Ref))
		}
	}
	if len(input.AffectedFiles) > 0 {
		body.WriteString("\n## Affected Files\n\n")
		for _, f := range input.AffectedFiles {
			body.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	validUntil := input.ValidUntil
	if validUntil == "" {
		validUntil = now.Add(90 * 24 * time.Hour).Format(time.RFC3339)
	}

	return &Artifact{
		Meta: Meta{
			ID:         id,
			Kind:       KindNote,
			Version:    1,
			Status:     StatusActive,
			Context:    input.Context,
			Mode:       ModeNote,
			Title:      input.Title,
			ValidUntil: validUntil,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Body:           body.String(),
		SearchKeywords: input.SearchKeywords,
	}
}

// CreateNote creates a Note artifact. Orchestrates effects around BuildNoteArtifact.
func CreateNote(ctx context.Context, store ArtifactStore, haftDir string, input NoteInput) (*Artifact, string, error) {
	affectedFiles, err := canonicalAffectedFilePaths(input.AffectedFiles)
	if err != nil {
		return nil, "", err
	}
	input.AffectedFiles = make([]string, 0, len(affectedFiles))
	for _, file := range affectedFiles {
		input.AffectedFiles = append(input.AffectedFiles, file.Path)
	}

	// GenerateID uses a crypto/rand suffix since #63; no sequence lookup
	// required. seq parameter preserved for backward compat — pass 0.
	id := GenerateIDWithTaskContext(KindNote, 0, input.TaskContext)

	// Validate typed anchors BEFORE creating anything: a dead anchor (target does
	// not exist) rejects the whole note — no dead edges, no fabricated
	// relationships (dec-20260604-26be1e4b).
	for _, an := range input.Anchors {
		if strings.TrimSpace(an.Ref) == "" {
			return nil, "", fmt.Errorf("anchor has an empty ref")
		}
		if _, err := store.Get(ctx, an.Ref); err != nil {
			return nil, "", fmt.Errorf("anchor target %q does not exist — anchors must point to a real artifact", an.Ref)
		}
	}

	a := BuildNoteArtifact(id, time.Now().UTC(), input)
	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store note: %w", err)
	}

	var warnings []string

	if len(affectedFiles) > 0 {
		if err := store.SetAffectedFiles(ctx, id, affectedFiles); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to track affected files: %v", err))
		}
	}

	// Symbol anchors (pre-resolved at the shell) — surface the fact in
	// code_context / node at the exact code symbol (dec-20260604-26be1e4b).
	if len(input.AffectedSymbols) > 0 {
		if err := store.SetAffectedSymbols(ctx, id, input.AffectedSymbols); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to track affected symbols: %v", err))
		}
	}

	// Persist typed anchors as real artifact links — they surface in
	// related / backlinks at the anchored decision/problem (the fusion payoff).
	for _, an := range input.Anchors {
		linkType := strings.TrimSpace(an.Type)
		if linkType == "" {
			linkType = "anchors"
		}
		if err := store.AddLink(ctx, id, an.Ref, linkType); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to anchor %s -> %s: %v", id, an.Ref, err))
		}
	}

	filePath, err := WriteFile(haftDir, a)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("file write failed (DB saved OK): %v", err))
	}

	if len(warnings) > 0 {
		return a, filePath, &WriteWarning{Warnings: warnings}
	}

	return a, filePath, nil
}
