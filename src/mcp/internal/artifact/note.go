package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NoteInput is the input for creating a note.
type NoteInput struct {
	Title          string   `json:"title"`
	Rationale      string   `json:"rationale"`
	AffectedFiles  []string `json:"affected_files,omitempty"`
	Evidence       string   `json:"evidence,omitempty"`
	Context        string   `json:"context,omitempty"`
	ValidUntil     string   `json:"valid_until,omitempty"`
	SearchKeywords string   `json:"search_keywords,omitempty"`
}

// NoteValidation holds the result of pre-recording checks.
type NoteValidation struct {
	OK        bool
	Warnings  []string
	Conflicts []ConflictInfo
	Suggest   string // suggested alternative action (e.g., "/q-frame")
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

	// Check 1: Rationale
	if strings.TrimSpace(input.Rationale) == "" {
		v.OK = false
		v.Warnings = append(v.Warnings, "Missing rationale. Why this choice? What alternatives were considered?")
		return v
	}
	words := len(strings.Fields(input.Rationale))
	if words < 5 && len(input.AffectedFiles) > 0 {
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
					"Decision %s (%s) already covers this topic (%.0f%% of note words found in decision). Notes are for observations and discoveries, not architectural choices. Use /q-decide for decisions.",
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
		v.Suggest = "/q-frame"
		v.Warnings = append(v.Warnings, fmt.Sprintf(
			"This note affects %d files. Consider using /q-frame for a proper problem exploration.",
			len(input.AffectedFiles),
		))
	}
	architecturalKeywords := []string{"migrate", "replace", "rewrite", "architecture", "redesign", "overhaul", "rearchitect"}
	titleLower := strings.ToLower(input.Title)
	rationaleLower := strings.ToLower(input.Rationale)
	for _, kw := range architecturalKeywords {
		if strings.Contains(titleLower, kw) || strings.Contains(rationaleLower, kw) {
			v.Suggest = "/q-frame"
			v.Warnings = append(v.Warnings, fmt.Sprintf(
				"This sounds like an architectural change (%q detected). Consider /q-frame instead of a note.",
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
	body.WriteString(fmt.Sprintf("## Rationale\n\n%s\n", input.Rationale))
	if input.Evidence != "" {
		body.WriteString(fmt.Sprintf("\n## Evidence\n\n%s\n", input.Evidence))
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
func CreateNote(ctx context.Context, store ArtifactStore, quintDir string, input NoteInput) (*Artifact, string, error) {
	seq, err := store.NextSequence(ctx, KindNote)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindNote, seq)
	a := BuildNoteArtifact(id, time.Now().UTC(), input)

	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store note: %w", err)
	}

	var warnings []string

	if len(input.AffectedFiles) > 0 {
		var files []AffectedFile
		for _, f := range input.AffectedFiles {
			files = append(files, AffectedFile{Path: f})
		}
		if err := store.SetAffectedFiles(ctx, id, files); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to track affected files: %v", err))
		}
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("file write failed (DB saved OK): %v", err))
	}

	if len(warnings) > 0 {
		return a, filePath, &WriteWarning{Warnings: warnings}
	}

	return a, filePath, nil
}

// FormatNoteResponse builds the MCP tool response for a note.
func FormatNoteResponse(a *Artifact, filePath string, validation NoteValidation, navStrip string) string {
	var sb strings.Builder

	if len(validation.Warnings) > 0 && validation.OK {
		for _, w := range validation.Warnings {
			sb.WriteString(fmt.Sprintf("⚠ %s\n", w))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Recorded: %s\n", a.Meta.Title))
	sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
	if filePath != "" {
		sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
	}

	sb.WriteString(navStrip)

	return sb.String()
}

// FormatNoteRejection builds the response when a note is rejected.
func FormatNoteRejection(validation NoteValidation, navStrip string) string {
	var sb strings.Builder

	for _, w := range validation.Warnings {
		sb.WriteString(fmt.Sprintf("⚠ %s\n", w))
	}

	if len(validation.Conflicts) > 0 {
		sb.WriteString("\nConflicting decisions:\n")
		for _, c := range validation.Conflicts {
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", c.DecisionID, c.DecisionTitle, c.Reason))
		}
	}

	sb.WriteString("\nOptions:\n")
	if validation.Suggest != "" {
		sb.WriteString(fmt.Sprintf("  1. Use %s to start a proper exploration\n", validation.Suggest))
		sb.WriteString("  2. Add rationale and retry\n")
	} else {
		sb.WriteString("  1. Add rationale explaining why this choice\n")
		sb.WriteString("  2. Provide evidence supporting the decision\n")
	}

	sb.WriteString(navStrip)

	return sb.String()
}
