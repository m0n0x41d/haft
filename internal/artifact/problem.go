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
	TaskContext           string   `json:"task_context,omitempty"`
	ProblemType           string   `json:"problem_type,omitempty"`
	ProblemProfile        string   `json:"problem_profile,omitempty"`
	SourceKind            string   `json:"source_kind,omitempty"`
	Signal                string   `json:"signal"`
	WhyNow                string   `json:"why_now,omitempty"`
	Scope                 string   `json:"scope,omitempty"`
	AcceptanceProbe       string   `json:"acceptance_probe,omitempty"`
	FreshnessDisposition  string   `json:"freshness_disposition,omitempty"`
	Constraints           []string `json:"constraints,omitempty"`
	OptimizationTargets   []string `json:"optimization_targets,omitempty"`
	ObservationIndicators []string `json:"observation_indicators,omitempty"`
	Acceptance            string   `json:"acceptance,omitempty"`
	BlastRadius           string   `json:"blast_radius,omitempty"`
	Reversibility         string   `json:"reversibility,omitempty"`
	Context               string   `json:"context,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
}

const (
	ProblemProfileCue  = "cue"
	ProblemProfileThin = "thin"
	ProblemProfileDeep = "deep"

	ProblemReadinessCueOnly   = "cue_only"
	ProblemReadinessCandidate = "p2w_candidate"
	ProblemReadinessReady     = "p2w_ready"
	ProblemReadinessBlocked   = "p2w_blocked"
	ProblemBoundaryExplicit   = "explicit"
	ProblemBoundaryMissing    = "missing"
	ProblemBoundaryPartial    = "partial"
	ProblemSourceObserved     = "observed_problem"
	ProblemSourceWish         = "wish"
	ProblemSourceTicket       = "ticket"
	ProblemSourceChosenMethod = "chosen_method"
)

// CharacterizeInput is the input for adding comparison dimensions.
type CharacterizeInput struct {
	ProblemRef  string                `json:"problem_ref"`
	Dimensions  []ComparisonDimension `json:"dimensions"`
	ParityRules string                `json:"parity_rules,omitempty"`
	ParityPlan  *ParityPlan           `json:"parity_plan,omitempty"`
}

// ComparisonDimension defines a single axis for comparing variants.
type ComparisonDimension struct {
	Name         string `json:"name"`
	ScaleType    string `json:"scale_type,omitempty"` // ordinal, ratio, nominal
	Unit         string `json:"unit,omitempty"`
	Polarity     string `json:"polarity,omitempty"`  // higher_better, lower_better
	Role         string `json:"role,omitempty"`      // constraint, target, observation (default: target)
	ProxyFor     string `json:"proxy_for,omitempty"` // intended value this dimension proxies (FPF E.13 — value before proxy)
	HowToMeasure string `json:"how_to_measure,omitempty"`
	ValidUntil   string `json:"valid_until,omitempty"` // when this measurement definition expires (RFC3339 or YYYY-MM-DD)
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
	problemType, err := ParseProblemType(input.ProblemType)
	if err != nil {
		return nil, err
	}
	profile, err := BuildProblemCardProfile(input)
	if err != nil {
		return nil, err
	}

	var mode Mode
	if input.Mode == "" {
		mode = ModeStandard
	} else {
		mode, err = ParseMode(input.Mode)
		if err != nil {
			return nil, fmt.Errorf("%w (valid: note, tactical, standard, deep)", err)
		}
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n\n", input.Title))
	body.WriteString(fmt.Sprintf("## Signal\n\n%s\n", input.Signal))

	if problemType != "" {
		body.WriteString(fmt.Sprintf("\n## Problem Type\n\n%s\n", problemType))
	}
	renderProblemProfileSections(&body, profile)

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
	fields := ProblemFields{
		ProblemType:           problemType,
		Signal:                input.Signal,
		Profile:               profilePtr(profile),
		Constraints:           input.Constraints,
		OptimizationTargets:   input.OptimizationTargets,
		ObservationIndicators: input.ObservationIndicators,
		Acceptance:            input.Acceptance,
		BlastRadius:           input.BlastRadius,
		Reversibility:         input.Reversibility,
	}
	fields.Semantic = semanticPtr(NewProblemSemanticEnvelopeForProblem(id, now, fields, a.Body))
	sd, _ := json.Marshal(fields)
	a.StructuredData = string(sd)

	return a, nil
}

func semanticPtr(value SemanticEnvelope) *SemanticEnvelope {
	return &value
}

func profilePtr(value ProblemCardProfile) *ProblemCardProfile {
	return &value
}

func BuildProblemCardProfile(input ProblemFrameInput) (ProblemCardProfile, error) {
	level, err := normalizeProblemProfileLevel(input.ProblemProfile)
	if err != nil {
		return ProblemCardProfile{}, err
	}
	sourceKind, err := normalizeProblemSourceKind(input.SourceKind)
	if err != nil {
		return ProblemCardProfile{}, err
	}

	profile := ProblemCardProfile{
		Level:                level,
		SourceKind:           sourceKind,
		WhyNow:               strings.TrimSpace(input.WhyNow),
		Scope:                strings.TrimSpace(input.Scope),
		AcceptanceProbe:      strings.TrimSpace(input.AcceptanceProbe),
		FreshnessDisposition: strings.TrimSpace(input.FreshnessDisposition),
	}
	profile.BoundaryStatus = problemBoundaryStatus(profile)
	profile.Blockers = problemReadinessBlockers(profile)
	profile.Readiness = problemReadiness(profile)

	return profile, nil
}

func normalizeProblemProfileLevel(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ProblemProfileThin, nil
	}

	switch normalized {
	case ProblemProfileCue, ProblemProfileThin, ProblemProfileDeep:
		return normalized, nil
	default:
		return "", fmt.Errorf("problem_profile must be cue, thin, or deep")
	}
}

func normalizeProblemSourceKind(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if normalized == "" {
		return ProblemSourceObserved, nil
	}

	switch normalized {
	case ProblemSourceObserved, ProblemSourceWish, ProblemSourceTicket, ProblemSourceChosenMethod:
		return normalized, nil
	default:
		return "", fmt.Errorf("source_kind must be observed_problem, wish, ticket, or chosen_method")
	}
}

func problemBoundaryStatus(profile ProblemCardProfile) string {
	hasScope := strings.TrimSpace(profile.Scope) != ""
	hasProbe := strings.TrimSpace(profile.AcceptanceProbe) != ""
	if hasScope && hasProbe {
		return ProblemBoundaryExplicit
	}
	if hasScope || hasProbe {
		return ProblemBoundaryPartial
	}

	return ProblemBoundaryMissing
}

func problemReadinessBlockers(profile ProblemCardProfile) []string {
	blockers := make([]string, 0)
	if profile.Level != ProblemProfileDeep {
		blockers = append(blockers, "problem_profile is not deep")
	}
	if strings.TrimSpace(profile.WhyNow) == "" {
		blockers = append(blockers, "why_now missing")
	}
	if strings.TrimSpace(profile.Scope) == "" {
		blockers = append(blockers, "scope boundary missing")
	}
	if strings.TrimSpace(profile.AcceptanceProbe) == "" {
		blockers = append(blockers, "acceptance_probe missing")
	}
	if strings.TrimSpace(profile.FreshnessDisposition) == "" {
		blockers = append(blockers, "freshness_disposition missing")
	}
	if sourceKindNeedsBoundary(profile.SourceKind) && profile.BoundaryStatus != ProblemBoundaryExplicit {
		blockers = append(blockers, "wish/ticket/chosen_method source requires explicit boundary before P2W readiness")
	}

	return blockers
}

func sourceKindNeedsBoundary(sourceKind string) bool {
	switch sourceKind {
	case ProblemSourceWish, ProblemSourceTicket, ProblemSourceChosenMethod:
		return true
	default:
		return false
	}
}

func problemReadiness(profile ProblemCardProfile) string {
	if profile.Level == ProblemProfileCue {
		return ProblemReadinessCueOnly
	}
	if len(profile.Blockers) == 0 {
		return ProblemReadinessReady
	}
	if sourceKindNeedsBoundary(profile.SourceKind) && profile.BoundaryStatus != ProblemBoundaryExplicit {
		return ProblemReadinessBlocked
	}
	if profile.Level == ProblemProfileDeep {
		return ProblemReadinessBlocked
	}

	return ProblemReadinessCandidate
}

func renderProblemProfileSections(body *strings.Builder, profile ProblemCardProfile) {
	body.WriteString(fmt.Sprintf("\n## Problem Profile\n\n%s\n", profile.Level))
	body.WriteString(fmt.Sprintf("\n## P2W Readiness\n\n%s\n", profile.Readiness))
	if profile.WhyNow != "" {
		body.WriteString(fmt.Sprintf("\n## Why Now\n\n%s\n", profile.WhyNow))
	}
	if profile.Scope != "" {
		body.WriteString(fmt.Sprintf("\n## Scope\n\n%s\n", profile.Scope))
	}
	if profile.AcceptanceProbe != "" {
		body.WriteString(fmt.Sprintf("\n## Acceptance Probe\n\n%s\n", profile.AcceptanceProbe))
	}
	if profile.FreshnessDisposition != "" {
		body.WriteString(fmt.Sprintf("\n## Freshness Disposition\n\n%s\n", profile.FreshnessDisposition))
	}
}

// FrameProblem creates a ProblemCard artifact. Orchestrates effects around BuildProblemArtifact.
func FrameProblem(ctx context.Context, store ArtifactStore, haftDir string, input ProblemFrameInput) (*Artifact, string, error) {
	// GenerateID uses a crypto/rand suffix since #63; no sequence lookup
	// required. seq parameter preserved for backward compat — pass 0.
	id := GenerateIDWithTaskContext(KindProblemCard, 0, input.TaskContext)

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

	filePath, err := WriteFile(haftDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// CharacterizeProblem adds comparison dimensions to an existing ProblemCard.
func CharacterizeProblem(ctx context.Context, store ArtifactStore, haftDir string, input CharacterizeInput) (*Artifact, string, error) {
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

	parityPlan := mergeLegacyParityRules(input.ParityPlan, input.ParityRules)
	if input.ParityPlan != nil {
		if err := ValidateParityPlan(*parityPlan); err != nil {
			return nil, "", err
		}
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
	// Optional columns appear only when at least one dimension uses them.
	hasProxyFor := false
	hasValidUntil := false
	for _, d := range input.Dimensions {
		hasProxyFor = hasProxyFor || d.ProxyFor != ""
		hasValidUntil = hasValidUntil || d.ValidUntil != ""
	}

	orDash := func(v string) string {
		if v == "" {
			return "-"
		}
		return v
	}

	headers := []string{"Dimension", "Role", "Scale", "Unit", "Polarity", "Measurement"}
	if hasProxyFor {
		headers = append(headers, "Proxy For (value)")
	}
	if hasValidUntil {
		headers = append(headers, "Valid Until")
	}
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = strings.Repeat("-", len(headers[i]))
	}
	section.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	section.WriteString("|" + strings.Join(separators, "|") + "|\n")

	for _, d := range input.Dimensions {
		role := d.Role
		if role == "" {
			role = "target"
		}
		cells := []string{d.Name, role, orDash(d.ScaleType), orDash(d.Unit), orDash(d.Polarity), orDash(d.HowToMeasure)}
		if hasProxyFor {
			cells = append(cells, orDash(d.ProxyFor))
		}
		if hasValidUntil {
			vu := d.ValidUntil
			if len(vu) > 10 {
				vu = vu[:10]
			}
			cells = append(cells, orDash(vu))
		}
		section.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}

	if input.ParityRules != "" && input.ParityPlan == nil {
		section.WriteString(fmt.Sprintf("\n**Parity rules:** %s\n", input.ParityRules))
	}
	if input.ParityPlan != nil {
		section.WriteString(renderParityPlanSection(parityPlan))
	}

	a.Body += section.String()

	fields := a.UnmarshalProblemFields()
	fields.Characterizations = append(fields.Characterizations, CharacterizationSnapshot{
		Version:    charVersion,
		Dimensions: cloneDimensions(input.Dimensions),
		ParityPlan: cloneParityPlan(parityPlan),
	})
	fields.Semantic = semanticPtr(NewProblemSemanticEnvelopeForProblem(a.Meta.ID, time.Now().UTC(), fields, a.Body))
	sd, _ := json.Marshal(fields)
	a.StructuredData = string(sd)

	if err := store.Update(ctx, a); err != nil {
		return nil, "", fmt.Errorf("update problem: %w", err)
	}

	filePath, err := WriteFile(haftDir, a)
	if err != nil {
		return a, "", fmt.Errorf("file write (DB saved OK): %w", err)
	}

	return a, filePath, nil
}

// ValueBeforeProxyWarning surfaces target-role dimensions that do not name the
// value they proxy (FPF E.13 — value before proxy). Soft warning, not a gate:
// field honesty is curation's job; the kernel only refuses to let the omission
// pass silently.
func ValueBeforeProxyWarning(dims []ComparisonDimension) string {
	missing := make([]string, 0, len(dims))
	for _, d := range dims {
		role := d.Role
		if role == "" {
			role = "target"
		}
		if role == "target" && d.ProxyFor == "" {
			missing = append(missing, d.Name)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("⚠ Value before proxy (FPF E.13): target dimension(s) %s do not name the value they proxy. A target under optimization pressure is a proxy — set proxy_for (the intended value this number serves) or re-tag as observation.",
		strings.Join(missing, ", "))
}

func renderParityPlanSection(plan *ParityPlan) string {
	if plan == nil {
		return ""
	}

	var section strings.Builder
	section.WriteString("\n**Parity plan:**\n")
	section.WriteString(fmt.Sprintf("- Baseline set: %s\n", strings.Join(plan.BaselineSet, ", ")))
	section.WriteString(fmt.Sprintf("- Window: %s\n", plan.Window))
	section.WriteString(fmt.Sprintf("- Budget: %s\n", plan.Budget))
	section.WriteString(fmt.Sprintf("- Missing data policy: %s\n", plan.MissingDataPolicy))

	for _, rule := range plan.Normalization {
		section.WriteString(fmt.Sprintf("- Normalize %s with %s\n", rule.Dimension, rule.Method))
	}
	for _, condition := range plan.PinnedConditions {
		section.WriteString(fmt.Sprintf("- Pinned condition: %s\n", condition))
	}

	return section.String()
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
