package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ProblemFrameInput is the input for framing a problem.
type ProblemFrameInput struct {
	Title                string   `json:"title"`
	Signal               string   `json:"signal"`
	Constraints          []string `json:"constraints,omitempty"`
	OptimizationTargets  []string `json:"optimization_targets,omitempty"`
	ObservationIndicators []string `json:"observation_indicators,omitempty"`
	Acceptance           string   `json:"acceptance,omitempty"`
	BlastRadius          string   `json:"blast_radius,omitempty"`
	Reversibility        string   `json:"reversibility,omitempty"`
	Context              string   `json:"context,omitempty"`
	Mode                 string   `json:"mode,omitempty"`
}

// CharacterizeInput is the input for adding comparison dimensions.
type CharacterizeInput struct {
	ProblemRef    string              `json:"problem_ref"`
	Dimensions    []ComparisonDimension `json:"dimensions"`
	ParityRules   string              `json:"parity_rules,omitempty"`
}

// ComparisonDimension defines a single axis for comparing variants.
type ComparisonDimension struct {
	Name         string `json:"name"`
	ScaleType    string `json:"scale_type,omitempty"`    // ordinal, ratio, nominal
	Unit         string `json:"unit,omitempty"`
	Polarity     string `json:"polarity,omitempty"`      // higher_better, lower_better
	Role         string `json:"role,omitempty"`           // constraint, target, observation (default: target)
	HowToMeasure string `json:"how_to_measure,omitempty"`
	ValidUntil   string `json:"valid_until,omitempty"`   // when this measurement definition expires (RFC3339)
}

// FrameProblem creates a ProblemCard artifact.
func FrameProblem(ctx context.Context, store *Store, quintDir string, input ProblemFrameInput) (*Artifact, string, error) {
	if input.Title == "" {
		return nil, "", fmt.Errorf("title is required")
	}
	if input.Signal == "" {
		return nil, "", fmt.Errorf("signal is required — what's anomalous or broken?")
	}

	seq, err := store.NextSequence(ctx, KindProblemCard)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindProblemCard, seq)
	now := time.Now().UTC()

	mode := Mode(input.Mode)
	if mode == "" {
		mode = ModeStandard
	}

	// Build markdown body
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

	// Archive recall: search for related past artifacts
	if recall := recallRelated(ctx, store, input.Title); recall != "" {
		a.Body += recall
	}

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
func CharacterizeProblem(ctx context.Context, store *Store, quintDir string, input CharacterizeInput) (*Artifact, string, error) {
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
func SelectProblems(ctx context.Context, store *Store, contextFilter string, limit int) ([]*Artifact, error) {
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

	return store.ListByKind(ctx, KindProblemCard, limit)
}

// FindActiveProblem returns the most recent active ProblemCard for a context (or globally).
func FindActiveProblem(ctx context.Context, store *Store, contextName string) (*Artifact, error) {
	var problems []*Artifact
	var err error

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
		problems, err = store.ListByKind(ctx, KindProblemCard, 1)
		if err != nil {
			return nil, err
		}
	}

	if len(problems) == 0 {
		return nil, nil
	}
	return problems[0], nil
}

// FormatProblemResponse builds the MCP tool response for a framed problem.
func FormatProblemResponse(action string, a *Artifact, filePath string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "frame":
		sb.WriteString(fmt.Sprintf("Problem framed: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
		sb.WriteString(fmt.Sprintf("Mode: %s\n", a.Meta.Mode))
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
	case "characterize":
		sb.WriteString(fmt.Sprintf("Characterization added to: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// FormatProblemsListResponse builds the response for listing problems with Goldilocks signals.
func FormatProblemsListResponse(problems []*Artifact, store *Store, ctx context.Context, navStrip string) string {
	var sb strings.Builder

	if len(problems) == 0 {
		sb.WriteString("No active problems found.\n")
		sb.WriteString("Use /q-frame to frame a new problem.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Active Problems (%d)\n\n", len(problems)))
	sb.WriteString("Goldilocks guide: pick problems in the growth zone — not too trivial, not too impossible for your current capacity.\n\n")

	for i, p := range problems {
		sb.WriteString(fmt.Sprintf("### %d. %s [%s]\n", i+1, p.Meta.Title, p.Meta.ID))
		if p.Meta.Context != "" {
			sb.WriteString(fmt.Sprintf("Context: %s | ", p.Meta.Context))
		}
		sb.WriteString(fmt.Sprintf("Mode: %s | Created: %s\n", p.Meta.Mode, p.Meta.CreatedAt.Format("2006-01-02")))

		// Goldilocks signals from body
		signals := extractGoldilocksSignals(p)
		if signals != "" {
			sb.WriteString(signals)
		}

		// Characterization status
		charCount := countCharacterizations(p)
		if charCount > 0 {
			sb.WriteString(fmt.Sprintf("Characterization: %d version(s) defined\n", charCount))
		} else {
			sb.WriteString("Characterization: not yet defined\n")
		}

		// Linked artifacts
		if store != nil {
			links, _ := store.GetLinks(ctx, p.Meta.ID)
			backlinks, _ := store.GetBacklinks(ctx, p.Meta.ID)
			if len(links)+len(backlinks) > 0 {
				sb.WriteString(fmt.Sprintf("Links: %d forward, %d back\n", len(links), len(backlinks)))
			}
		}

		// Staleness
		if p.Meta.ValidUntil != "" {
			vu := p.Meta.ValidUntil
			if len(vu) > 10 {
				vu = vu[:10]
			}
			sb.WriteString(fmt.Sprintf("Valid until: %s\n", vu))
		}

		sb.WriteString("\n")
	}

	sb.WriteString(navStrip)
	return sb.String()
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
func countCharacterizationDimensions(p *Artifact) int {
	// Reuse existing parser — just count results
	dims := extractCharacterizedDimensionsFromBody(p.Body)
	return len(dims)
}

// extractCharacterizedDimensionsFromBody is a local helper to avoid import cycle.
// Counts table rows (lines starting with | that aren't header/separator).
func extractCharacterizedDimensionsFromBody(body string) []string {
	lastIdx := -1
	for i := 100; i >= 1; i-- {
		marker := fmt.Sprintf("## Characterization v%d", i)
		if idx := strings.Index(body, marker); idx != -1 {
			lastIdx = idx
			break
		}
	}
	if lastIdx == -1 {
		return nil
	}
	section := body[lastIdx:]
	if endIdx := strings.Index(section[1:], "\n## "); endIdx != -1 {
		section = section[:endIdx+1]
	}
	var dims []string
	inTable := false
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if inTable {
				break
			}
			continue
		}
		if strings.Contains(line, "Dimension") || strings.Contains(line, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) >= 3 {
			name := strings.TrimSpace(parts[1])
			if name != "" && name != "-" {
				dims = append(dims, name)
			}
		}
	}
	return dims
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
