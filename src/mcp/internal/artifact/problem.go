package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ProblemFrameInput is the input for framing a problem.
type ProblemFrameInput struct {
	Title                 string   `json:"title"`
	Signal                string   `json:"signal"`
	Constraints           []string `json:"constraints,omitempty"`
	OptimizationTargets   []string `json:"optimization_targets,omitempty"`
	ObservationIndicators []string `json:"observation_indicators,omitempty"`
	Acceptance            string   `json:"acceptance,omitempty"`
	BlastRadius           string   `json:"blast_radius,omitempty"`
	Reversibility         string   `json:"reversibility,omitempty"`
	Context               string   `json:"context,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
}

// CharacterizeInput is the input for adding comparison dimensions.
type CharacterizeInput struct {
	ProblemRef  string                `json:"problem_ref"`
	Dimensions  []ComparisonDimension `json:"dimensions"`
	ParityRules string                `json:"parity_rules,omitempty"`
}

// ComparisonDimension defines a single axis for comparing variants.
type ComparisonDimension struct {
	Name         string `json:"name"`
	ScaleType    string `json:"scale_type,omitempty"` // ordinal, ratio, nominal
	Unit         string `json:"unit,omitempty"`
	Polarity     string `json:"polarity,omitempty"` // higher_better, lower_better
	Role         string `json:"role,omitempty"`     // constraint, target, observation (default: target)
	HowToMeasure string `json:"how_to_measure,omitempty"`
	ValidUntil   string `json:"valid_until,omitempty"` // when this measurement definition expires (RFC3339)
}

// BuildProblemArtifact constructs a ProblemCard from input. Pure — no side effects.
// The recall parameter is pre-fetched related history (may be empty).
func BuildProblemArtifact(id string, now time.Time, input ProblemFrameInput, recall string) (*Artifact, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if input.Signal == "" {
		return nil, fmt.Errorf("signal is required — what's anomalous or broken?")
	}

	var mode Mode
	if input.Mode == "" {
		mode = ModeStandard
	} else {
		var err error
		mode, err = ParseMode(input.Mode)
		if err != nil {
			return nil, fmt.Errorf("%w (valid: note, tactical, standard, deep)", err)
		}
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n\n", input.Title))
	body.WriteString(fmt.Sprintf("## Signal\n\n%s\n", input.Signal))

	if len(input.Constraints) > 0 {
		body.WriteString("\n## Constraints\n\n")
		for _, c := range input.Constraints {
			body.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}

	if len(input.OptimizationTargets) > 0 {
		body.WriteString("\n## Optimization Targets\n\n")
		for _, t := range input.OptimizationTargets {
			body.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}

	if len(input.ObservationIndicators) > 0 {
		body.WriteString("\n## Observation Indicators\n\n")
		for _, i := range input.ObservationIndicators {
			body.WriteString(fmt.Sprintf("- %s\n", i))
		}
	}

	if input.Acceptance != "" {
		body.WriteString(fmt.Sprintf("\n## Acceptance\n\n%s\n", input.Acceptance))
	}

	if input.BlastRadius != "" {
		body.WriteString(fmt.Sprintf("\n## Blast Radius\n\n%s\n", input.BlastRadius))
	}

	if input.Reversibility != "" {
		body.WriteString(fmt.Sprintf("\n## Reversibility\n\n%s\n", input.Reversibility))
	}

	a := &Artifact{
		Meta: Meta{
			ID:        id,
			Kind:      KindProblemCard,
			Version:   1,
			Status:    StatusActive,
			Context:   input.Context,
			Mode:      mode,
			Title:     input.Title,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: body.String(),
	}

	if recall != "" {
		a.Body += recall
	}

	// Populate structured data — canonical fields alongside markdown body
	sd, _ := json.Marshal(ProblemFields{
		Signal:                input.Signal,
		Constraints:           input.Constraints,
		OptimizationTargets:   input.OptimizationTargets,
		ObservationIndicators: input.ObservationIndicators,
		Acceptance:            input.Acceptance,
		BlastRadius:           input.BlastRadius,
		Reversibility:         input.Reversibility,
	})
	a.StructuredData = string(sd)

	return a, nil
}

// FrameProblem creates a ProblemCard artifact. Orchestrates effects around BuildProblemArtifact.
func FrameProblem(ctx context.Context, store ArtifactStore, quintDir string, input ProblemFrameInput) (*Artifact, string, error) {
	seq, err := store.NextSequence(ctx, KindProblemCard)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindProblemCard, seq)

	// Pre-fetch recall (side effect)
	recallQuery := input.Title
	if input.Signal != "" {
		recallQuery += " " + input.Signal
	}
	recall := recallRelated(ctx, store, recallQuery)

	// Pure construction
	a, err := BuildProblemArtifact(id, time.Now().UTC(), input, recall)
	if err != nil {
		return nil, "", err
	}

	// Persist (side effects)
	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store problem: %w", err)
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// CharacterizeProblem adds comparison dimensions to an existing ProblemCard.
func CharacterizeProblem(ctx context.Context, store ArtifactStore, quintDir string, input CharacterizeInput) (*Artifact, string, error) {
	if input.ProblemRef == "" {
		return nil, "", fmt.Errorf("problem_ref is required")
	}

	a, err := store.Get(ctx, input.ProblemRef)
	if err != nil {
		return nil, "", fmt.Errorf("problem %s not found: %w", input.ProblemRef, err)
	}
	if a.Meta.Kind != KindProblemCard {
		return nil, "", fmt.Errorf("%s is %s, not ProblemCard", input.ProblemRef, a.Meta.Kind)
	}

	if len(input.Dimensions) == 0 {
		return nil, "", fmt.Errorf("at least one comparison dimension is required")
	}

	// Count existing characterization versions
	charVersion := 1
	for i := 1; ; i++ {
		marker := fmt.Sprintf("## Characterization v%d", i)
		if !strings.Contains(a.Body, marker) {
			charVersion = i
			break
		}
	}

	// Append new characterization version (never overwrite — keep history)
	var section strings.Builder
	section.WriteString(fmt.Sprintf("\n## Characterization v%d (%s)\n\n",
		charVersion, time.Now().UTC().Format("2006-01-02")))
	// Check if any dimension has valid_until — only show column if used
	hasValidUntil := false
	for _, d := range input.Dimensions {
		if d.ValidUntil != "" {
			hasValidUntil = true
			break
		}
	}

	if hasValidUntil {
		section.WriteString("| Dimension | Role | Scale | Unit | Polarity | Measurement | Valid Until |\n")
		section.WriteString("|-----------|------|-------|------|----------|-------------|-------------|\n")
	} else {
		section.WriteString("| Dimension | Role | Scale | Unit | Polarity | Measurement |\n")
		section.WriteString("|-----------|------|-------|------|----------|-------------|\n")
	}
	for _, d := range input.Dimensions {
		role := d.Role
		if role == "" {
			role = "target"
		}
		scale := d.ScaleType
		if scale == "" {
			scale = "-"
		}
		unit := d.Unit
		if unit == "" {
			unit = "-"
		}
		polarity := d.Polarity
		if polarity == "" {
			polarity = "-"
		}
		measure := d.HowToMeasure
		if measure == "" {
			measure = "-"
		}
		if hasValidUntil {
			vu := d.ValidUntil
			if vu == "" {
				vu = "-"
			} else if len(vu) > 10 {
				vu = vu[:10]
			}
			section.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n", d.Name, role, scale, unit, polarity, measure, vu))
		} else {
			section.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", d.Name, role, scale, unit, polarity, measure))
		}
	}

	if input.ParityRules != "" {
		section.WriteString(fmt.Sprintf("\n**Parity rules:** %s\n", input.ParityRules))
	}

	a.Body += section.String()

	if err := store.Update(ctx, a); err != nil {
		return nil, "", fmt.Errorf("update problem: %w", err)
	}

	filePath, err := WriteFile(quintDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// SelectProblems lists active ProblemCards, optionally filtered by context.
func SelectProblems(ctx context.Context, store ArtifactStore, contextFilter string, limit int) ([]*Artifact, error) {
	if limit <= 0 {
		limit = 20
	}

	if contextFilter != "" {
		all, err := store.ListByContext(ctx, contextFilter)
		if err != nil {
			return nil, err
		}
		var problems []*Artifact
		for _, a := range all {
			if a.Meta.Kind == KindProblemCard && a.Meta.Status == StatusActive {
				problems = append(problems, a)
			}
		}
		return problems, nil
	}

	return store.ListActiveByKind(ctx, KindProblemCard, limit)
}

// FindActiveProblem returns the most recent active ProblemCard for a context (or globally).
func FindActiveProblem(ctx context.Context, store ArtifactStore, contextName string) (*Artifact, error) {
	var problems []*Artifact

	if contextName != "" {
		all, e := store.ListByContext(ctx, contextName)
		if e != nil {
			return nil, e
		}
		for _, a := range all {
			if a.Meta.Kind == KindProblemCard && a.Meta.Status == StatusActive {
				problems = append(problems, a)
			}
		}
	} else {
		active, e := store.ListActiveByKind(ctx, KindProblemCard, 1)
		if e != nil {
			return nil, e
		}
		problems = active
	}

	if len(problems) == 0 {
		return nil, nil
	}
	return problems[0], nil
}

// ProblemListItem holds pre-fetched enrichment data for a problem in the list view.
type ProblemListItem struct {
	Problem        *Artifact
	Signals        string // Goldilocks signals (pure, from body)
	CharCount      int
	EvidenceTotal  int
	EvidenceSupp   int
	EvidenceWeak   int
	EvidenceRefute int
	ForwardLinks   int
	BackLinks      int
}

// EnrichProblemsForList pre-fetches store data for each problem. Effect boundary.
func EnrichProblemsForList(ctx context.Context, store ArtifactStore, problems []*Artifact) []ProblemListItem {
	items := make([]ProblemListItem, len(problems))
	for i, p := range problems {
		item := ProblemListItem{
			Problem:   p,
			Signals:   extractGoldilocksSignals(p),
			CharCount: countCharacterizations(p),
		}

		evidItems, _ := store.GetEvidenceItems(ctx, p.Meta.ID)
		item.EvidenceTotal = len(evidItems)
		for _, e := range evidItems {
			switch e.Verdict {
			case "supports", "accepted":
				item.EvidenceSupp++
			case "weakens", "partial":
				item.EvidenceWeak++
			case "refutes", "failed":
				item.EvidenceRefute++
			}
		}

		links, _ := store.GetLinks(ctx, p.Meta.ID)
		backlinks, _ := store.GetBacklinks(ctx, p.Meta.ID)
		item.ForwardLinks = len(links)
		item.BackLinks = len(backlinks)

		items[i] = item
	}
	return items
}

func extractGoldilocksSignals(p *Artifact) string {
	var signals strings.Builder
	body := p.Body

	// Blast radius and reversibility (existing)
	if strings.Contains(body, "## Blast Radius") {
		if idx := strings.Index(body, "## Blast Radius"); idx != -1 {
			rest := body[idx+len("## Blast Radius"):]
			rest = strings.TrimLeft(rest, "\n\r ")
			if end := strings.Index(rest, "\n#"); end > 0 {
				rest = rest[:end]
			}
			line := strings.TrimSpace(strings.Split(rest, "\n")[0])
			if line != "" {
				signals.WriteString(fmt.Sprintf("Blast radius: %s\n", line))
			}
		}
	}
	if strings.Contains(body, "## Reversibility") {
		if idx := strings.Index(body, "## Reversibility"); idx != -1 {
			rest := body[idx+len("## Reversibility"):]
			rest = strings.TrimLeft(rest, "\n\r ")
			line := strings.TrimSpace(strings.Split(rest, "\n")[0])
			if line != "" {
				signals.WriteString(fmt.Sprintf("Reversibility: %s\n", line))
			}
		}
	}

	// Readiness score: count how well-framed the problem is
	readiness := 0
	readinessMax := 6
	if strings.Contains(body, "## Signal") {
		readiness++
	}
	if strings.Contains(body, "## Constraints") {
		readiness++
	}
	if strings.Contains(body, "## Acceptance") {
		readiness++
	}
	if strings.Contains(body, "## Optimization Targets") {
		readiness++
	}
	if strings.Contains(body, "## Blast Radius") {
		readiness++
	}
	if countCharacterizations(p) > 0 {
		readiness++
	}
	signals.WriteString(fmt.Sprintf("Readiness: %d/%d", readiness, readinessMax))

	// Complexity signals: constraint count + target count
	constraintCount := countBullets(body, "## Constraints")
	targetCount := countBullets(body, "## Optimization Targets")
	dimCount := countCharacterizationDimensions(p)

	var complexity []string
	if constraintCount > 0 {
		complexity = append(complexity, fmt.Sprintf("%d constraints", constraintCount))
	}
	if targetCount > 0 {
		complexity = append(complexity, fmt.Sprintf("%d targets", targetCount))
	}
	if dimCount > 0 {
		complexity = append(complexity, fmt.Sprintf("%d dimensions", dimCount))
	}
	if len(complexity) > 0 {
		signals.WriteString(fmt.Sprintf(" | Complexity: %s", strings.Join(complexity, ", ")))
	}
	signals.WriteString("\n")

	return signals.String()
}

// countBullets counts "- " lines in a section of the body.
func countBullets(body, section string) int {
	idx := strings.Index(body, section)
	if idx == -1 {
		return 0
	}
	rest := body[idx+len(section):]
	if end := strings.Index(rest, "\n## "); end > 0 {
		rest = rest[:end]
	}
	count := 0
	for _, line := range strings.Split(rest, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			count++
		}
	}
	return count
}

// countCharacterizationDimensions counts dimension rows in the latest characterization table.
// Uses extractCharacterizedDimensions from solution.go (same package).
func countCharacterizationDimensions(p *Artifact) int {
	return len(extractCharacterizedDimensions(p.Body))
}

func countCharacterizations(p *Artifact) int {
	count := 0
	for i := 1; i <= 100; i++ {
		if strings.Contains(p.Body, fmt.Sprintf("## Characterization v%d", i)) {
			count = i
		} else {
			break
		}
	}
	return count
}
