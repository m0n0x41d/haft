package artifact

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// DecideInput is the input for creating a DecisionRecord.
type DecideInput struct {
	ProblemRef    string              `json:"problem_ref,omitempty"`    // single problem (backward compat)
	ProblemRefs   []string            `json:"problem_refs,omitempty"`  // multiple problems
	PortfolioRef  string              `json:"portfolio_ref,omitempty"`
	SelectedTitle string              `json:"selected_title"`
	WhySelected   string              `json:"why_selected"`
	WhyNotOthers  []RejectionReason   `json:"why_not_others,omitempty"`
	Invariants    []string            `json:"invariants,omitempty"`
	PreConditions []string            `json:"pre_conditions,omitempty"`
	PostConditions []string           `json:"post_conditions,omitempty"`
	Admissibility []string            `json:"admissibility,omitempty"`
	EvidenceReqs  []string            `json:"evidence_requirements,omitempty"`
	Rollback      *RollbackSpec       `json:"rollback,omitempty"`
	RefreshTriggers []string          `json:"refresh_triggers,omitempty"`
	WeakestLink   string              `json:"weakest_link,omitempty"`
	ValidUntil    string              `json:"valid_until,omitempty"`
	Context       string              `json:"context,omitempty"`
	Mode          string              `json:"mode,omitempty"`
	AffectedFiles []string            `json:"affected_files,omitempty"`
}

// RejectionReason explains why a variant was not selected.
type RejectionReason struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

// RollbackSpec defines when and how to reverse a decision.
type RollbackSpec struct {
	Triggers   []string `json:"triggers,omitempty"`
	Steps      []string `json:"steps,omitempty"`
	BlastRadius string  `json:"blast_radius,omitempty"`
}

// ApplyInput is the input for generating an implementation brief.
type ApplyInput struct {
	DecisionRef string `json:"decision_ref"`
}

// Decide creates a DecisionRecord artifact — the crown jewel.
func Decide(ctx context.Context, store *Store, quintDir string, input DecideInput) (*Artifact, string, error) {
	if input.SelectedTitle == "" {
		return nil, "", fmt.Errorf("selected_title is required — what variant was chosen?")
	}
	if input.WhySelected == "" {
		return nil, "", fmt.Errorf("why_selected is required — rationale for the choice")
	}

	seq, err := store.NextSequence(ctx, KindDecisionRecord)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindDecisionRecord, seq)
	now := time.Now().UTC()

	mode := Mode(input.Mode)
	if mode == "" {
		mode = ModeStandard
	}

	// Merge ProblemRef (single, backward compat) + ProblemRefs (array) into one list
	problemRefs := input.ProblemRefs
	if input.ProblemRef != "" {
		found := false
		for _, r := range problemRefs {
			if r == input.ProblemRef {
				found = true
				break
			}
		}
		if !found {
			problemRefs = append(problemRefs, input.ProblemRef)
		}
	}

	var links []Link
	for _, ref := range problemRefs {
		links = append(links, Link{Ref: ref, Type: "based_on"})
	}
	if input.PortfolioRef != "" {
		links = append(links, Link{Ref: input.PortfolioRef, Type: "based_on"})
	}

	// Inherit context from linked artifacts
	if input.Context == "" {
		if input.PortfolioRef != "" {
			if p, err := store.Get(ctx, input.PortfolioRef); err == nil {
				input.Context = p.Meta.Context
			}
		} else if len(problemRefs) > 0 {
			if p, err := store.Get(ctx, problemRefs[0]); err == nil {
				input.Context = p.Meta.Context
			}
		}
	}

	// Use first problem ref for Problem Frame section
	if input.ProblemRef == "" && len(problemRefs) > 0 {
		input.ProblemRef = problemRefs[0]
	}

	title := input.SelectedTitle

	// Build the DRR markdown — FPF E.9 four-component structure
	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n", title))

	// === Component 1: Problem Frame ===
	body.WriteString("\n## 1. Problem Frame\n\n")
	if input.ProblemRef != "" {
		if prob, err := store.Get(ctx, input.ProblemRef); err == nil {
			// Extract signal from problem body
			if idx := strings.Index(prob.Body, "## Signal"); idx != -1 {
				sigStart := idx + len("## Signal")
				sigEnd := strings.Index(prob.Body[sigStart:], "\n## ")
				var signal string
				if sigEnd > 0 {
					signal = strings.TrimSpace(prob.Body[sigStart : sigStart+sigEnd])
				} else {
					signal = strings.TrimSpace(prob.Body[sigStart:])
				}
				if signal != "" {
					body.WriteString(fmt.Sprintf("**Signal:** %s\n\n", signal))
				}
			}
			// Extract constraints
			if idx := strings.Index(prob.Body, "## Constraints"); idx != -1 {
				cStart := idx + len("## Constraints")
				cEnd := strings.Index(prob.Body[cStart:], "\n## ")
				var constraints string
				if cEnd > 0 {
					constraints = strings.TrimSpace(prob.Body[cStart : cStart+cEnd])
				} else {
					constraints = strings.TrimSpace(prob.Body[cStart:])
				}
				if constraints != "" {
					body.WriteString(fmt.Sprintf("**Constraints:**\n%s\n\n", constraints))
				}
			}
			// Extract acceptance
			if idx := strings.Index(prob.Body, "## Acceptance"); idx != -1 {
				aStart := idx + len("## Acceptance")
				aEnd := strings.Index(prob.Body[aStart:], "\n## ")
				var acceptance string
				if aEnd > 0 {
					acceptance = strings.TrimSpace(prob.Body[aStart : aStart+aEnd])
				} else {
					acceptance = strings.TrimSpace(prob.Body[aStart:])
				}
				if acceptance != "" {
					body.WriteString(fmt.Sprintf("**Acceptance:** %s\n\n", acceptance))
				}
			}
		}
	}

	// === Component 2: Decision (the contract) ===
	body.WriteString("## 2. Decision\n\n")
	body.WriteString(fmt.Sprintf("**Selected:** %s\n\n", input.SelectedTitle))
	body.WriteString(fmt.Sprintf("%s\n", input.WhySelected))

	if len(input.Invariants) > 0 {
		body.WriteString("\n**Invariants:**\n")
		for _, inv := range input.Invariants {
			body.WriteString(fmt.Sprintf("- %s\n", inv))
		}
	}

	if len(input.PreConditions) > 0 {
		body.WriteString("\n**Pre-conditions:**\n")
		for _, pc := range input.PreConditions {
			body.WriteString(fmt.Sprintf("- [ ] %s\n", pc))
		}
	}

	if len(input.PostConditions) > 0 {
		body.WriteString("\n**Post-conditions:**\n")
		for _, pc := range input.PostConditions {
			body.WriteString(fmt.Sprintf("- [ ] %s\n", pc))
		}
	}

	if len(input.Admissibility) > 0 {
		body.WriteString("\n**Admissibility:**\n")
		for _, a := range input.Admissibility {
			body.WriteString(fmt.Sprintf("- NOT: %s\n", a))
		}
	}

	// === Component 3: Rationale ===
	body.WriteString("\n## 3. Rationale\n\n")
	if len(input.WhyNotOthers) > 0 {
		body.WriteString("| Variant | Verdict | Reason |\n")
		body.WriteString("|---------|---------|--------|\n")
		body.WriteString(fmt.Sprintf("| %s | **Selected** | %s |\n", input.SelectedTitle, truncate(input.WhySelected, 60)))
		for _, r := range input.WhyNotOthers {
			body.WriteString(fmt.Sprintf("| %s | Rejected | %s |\n", r.Variant, r.Reason))
		}
		body.WriteString("\n")
	}

	if input.WeakestLink != "" {
		body.WriteString(fmt.Sprintf("**Weakest link:** %s\n\n", input.WeakestLink))
	}

	if len(input.EvidenceReqs) > 0 {
		body.WriteString("**Evidence requirements:**\n")
		for _, e := range input.EvidenceReqs {
			body.WriteString(fmt.Sprintf("- %s\n", e))
		}
		body.WriteString("\n")
	}

	// === Component 4: Consequences ===
	body.WriteString("## 4. Consequences\n\n")

	if input.Rollback != nil {
		body.WriteString("**Rollback plan:**\n")
		if len(input.Rollback.Triggers) > 0 {
			body.WriteString("Triggers:\n")
			for _, t := range input.Rollback.Triggers {
				body.WriteString(fmt.Sprintf("- %s\n", t))
			}
		}
		if len(input.Rollback.Steps) > 0 {
			body.WriteString("Steps:\n")
			for i, s := range input.Rollback.Steps {
				body.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
			}
		}
		if input.Rollback.BlastRadius != "" {
			body.WriteString(fmt.Sprintf("Blast radius: %s\n", input.Rollback.BlastRadius))
		}
		body.WriteString("\n")
	}

	if len(input.RefreshTriggers) > 0 {
		body.WriteString("**Refresh triggers:**\n")
		for _, rt := range input.RefreshTriggers {
			body.WriteString(fmt.Sprintf("- %s\n", rt))
		}
		body.WriteString("\n")
	}

	if len(input.AffectedFiles) > 0 {
		body.WriteString("**Affected files:** ")
		body.WriteString(strings.Join(input.AffectedFiles, ", "))
		body.WriteString("\n")
	}

	a := &Artifact{
		Meta: Meta{
			ID:         id,
			Kind:       KindDecisionRecord,
			Version:    1,
			Status:     StatusActive,
			Context:    input.Context,
			Mode:       mode,
			Title:      title,
			ValidUntil: input.ValidUntil,
			CreatedAt:  now,
			UpdatedAt:  now,
			Links:      links,
		},
		Body: body.String(),
	}

	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store decision: %w", err)
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

// Apply is deprecated — the decide response now includes the full DRR body.
// Kept for backward compatibility: returns the DRR body directly.
func Apply(ctx context.Context, store *Store, decisionRef string) (string, error) {
	a, err := store.Get(ctx, decisionRef)
	if err != nil {
		return "", fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord {
		return "", fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, a.Meta.Kind)
	}
	return a.Body, nil
}

// MeasureInput records impact after implementation.
type MeasureInput struct {
	DecisionRef string   `json:"decision_ref"`
	Findings    string   `json:"findings"`
	CriteriaMet []string `json:"criteria_met,omitempty"`
	CriteriaNotMet []string `json:"criteria_not_met,omitempty"`
	Measurements []string `json:"measurements,omitempty"`
	Verdict     string   `json:"verdict"` // accepted, partial, failed
}

// EvidenceInput attaches evidence to any artifact.
// CongruenceLevel and FormalityLevel use -1 as "not provided" sentinel.
// JSON decodes missing fields as 0, which is a valid CL value (opposed context).
// Callers from MCP should set these to -1 when the user doesn't provide them.
type EvidenceInput struct {
	ArtifactRef     string `json:"artifact_ref"`
	Content         string `json:"content"`
	Type            string `json:"type"`    // measurement, test, research, benchmark, audit
	Verdict         string `json:"verdict"` // supports, weakens, refutes
	CarrierRef      string `json:"carrier_ref,omitempty"`
	CongruenceLevel int    `json:"congruence_level"` // 0-3; -1 = not provided (defaults to 3)
	FormalityLevel  int    `json:"formality_level"`  // 0-9; -1 = not provided (defaults to 5)
	ValidUntil      string `json:"valid_until,omitempty"`
}

// Measure records post-implementation impact against the DRR's acceptance criteria.
func Measure(ctx context.Context, store *Store, quintDir string, input MeasureInput) (*Artifact, error) {
	if input.DecisionRef == "" {
		return nil, fmt.Errorf("decision_ref is required")
	}
	if input.Findings == "" {
		return nil, fmt.Errorf("findings is required — what actually happened?")
	}
	if input.Verdict == "" {
		return nil, fmt.Errorf("verdict is required — accepted, partial, or failed")
	}

	a, err := store.Get(ctx, input.DecisionRef)
	if err != nil {
		return nil, fmt.Errorf("decision %s not found: %w", input.DecisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord {
		return nil, fmt.Errorf("%s is %s, not DecisionRecord", input.DecisionRef, a.Meta.Kind)
	}

	// Append impact measurement section to DRR body
	var section strings.Builder
	section.WriteString(fmt.Sprintf("\n## Impact Measurement (%s)\n\n", time.Now().UTC().Format("2006-01-02")))
	section.WriteString(fmt.Sprintf("**Verdict:** %s\n\n", input.Verdict))
	section.WriteString(fmt.Sprintf("**Findings:**\n%s\n", input.Findings))

	if len(input.CriteriaMet) > 0 {
		section.WriteString("\n**Criteria met:**\n")
		for _, c := range input.CriteriaMet {
			section.WriteString(fmt.Sprintf("- [x] %s\n", c))
		}
	}
	if len(input.CriteriaNotMet) > 0 {
		section.WriteString("\n**Criteria NOT met:**\n")
		for _, c := range input.CriteriaNotMet {
			section.WriteString(fmt.Sprintf("- [ ] %s\n", c))
		}
	}
	if len(input.Measurements) > 0 {
		section.WriteString("\n**Measurements:**\n")
		for _, m := range input.Measurements {
			section.WriteString(fmt.Sprintf("- %s\n", m))
		}
	}

	a.Body += section.String()

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update decision: %w", err)
	}

	// Record as evidence item
	evidID := fmt.Sprintf("evid-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)
	if err := store.AddEvidenceItem(ctx, &EvidenceItem{
		ID:             evidID,
		Type:           "measurement",
		Content:        fmt.Sprintf("Impact measurement: %s\n%s", input.Verdict, input.Findings),
		Verdict:        input.Verdict,
		CongruenceLevel: 3,
		FormalityLevel:  5,
	}, input.DecisionRef); err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}

	writeFileQuiet(quintDir, a)
	return a, nil
}

// AttachEvidence adds an evidence item to any artifact.
func AttachEvidence(ctx context.Context, store *Store, input EvidenceInput) (*EvidenceItem, error) {
	if input.ArtifactRef == "" {
		return nil, fmt.Errorf("artifact_ref is required")
	}
	if input.Content == "" {
		return nil, fmt.Errorf("content is required — what's the evidence?")
	}

	// Verify artifact exists
	_, err := store.Get(ctx, input.ArtifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", input.ArtifactRef, err)
	}

	if input.Type == "" {
		input.Type = "general"
	}
	if input.CongruenceLevel < 0 {
		input.CongruenceLevel = 3
	}
	if input.FormalityLevel < 0 {
		input.FormalityLevel = 5
	}

	id := fmt.Sprintf("evid-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)

	item := &EvidenceItem{
		ID:              id,
		Type:            input.Type,
		Content:         input.Content,
		Verdict:         input.Verdict,
		CarrierRef:      input.CarrierRef,
		CongruenceLevel: input.CongruenceLevel,
		FormalityLevel:  input.FormalityLevel,
		ValidUntil:      input.ValidUntil,
	}

	if err := store.AddEvidenceItem(ctx, item, input.ArtifactRef); err != nil {
		return nil, fmt.Errorf("store evidence: %w", err)
	}

	return item, nil
}

// WLNKSummary holds WLNK analysis for an artifact based on its evidence items.
// R_eff is computed per FPF B.3: min(effective_score_i) across all evidence,
// where effective_score = max(0, base_score - clPenalty).
type WLNKSummary struct {
	ArtifactID    string
	EvidenceCount int
	Supporting    int
	Weakening     int
	Refuting      int
	HasEvidence   bool    // true if at least one evidence item exists
	REff          float64 // computed: min(effective_score) across evidence chain
	MinFreshness  string  // earliest valid_until across all evidence
	WeakestCL     int     // minimum congruence level
	WeakestF      int     // minimum formality level
	Summary       string  // human-readable one-liner
}

// ComputeWLNKSummary returns a WLNK summary for an artifact based on its evidence items.
// R_eff is computed as min(effective_score_i) where:
//   - base_score: supports=1.0, weakens=0.5, refutes=0.0
//   - CL penalty: CL3=0.0, CL2=0.1, CL1=0.4, CL0=0.9
//   - decay: expired evidence scores 0.1 regardless of verdict
//   - effective_score = max(0, base_score - clPenalty)
func ComputeWLNKSummary(ctx context.Context, store *Store, artifactID string) WLNKSummary {
	result := WLNKSummary{
		ArtifactID: artifactID,
		WeakestCL:  3,
		WeakestF:   9,
	}

	items, err := store.GetEvidenceItems(ctx, artifactID)
	if err != nil || len(items) == 0 {
		result.Summary = "no evidence attached"
		return result
	}

	result.EvidenceCount = len(items)
	result.HasEvidence = true
	now := time.Now().UTC()
	minREff := 1.0

	for _, e := range items {
		switch e.Verdict {
		case "supports", "accepted":
			result.Supporting++
		case "weakens", "partial":
			result.Weakening++
		case "refutes", "failed":
			result.Refuting++
		}

		if e.CongruenceLevel < result.WeakestCL {
			result.WeakestCL = e.CongruenceLevel
		}
		if e.FormalityLevel < result.WeakestF && e.FormalityLevel > 0 {
			result.WeakestF = e.FormalityLevel
		}

		if e.ValidUntil != "" {
			if result.MinFreshness == "" || e.ValidUntil < result.MinFreshness {
				result.MinFreshness = e.ValidUntil
			}
		}

		// Compute per-item effective score for R_eff
		score := scoreEvidence(e, now)
		if score < minREff {
			minREff = score
		}
	}

	result.REff = minREff

	// Build summary
	var parts []string
	parts = append(parts, fmt.Sprintf("%d evidence item(s)", result.EvidenceCount))
	if result.Supporting > 0 {
		parts = append(parts, fmt.Sprintf("%d supporting", result.Supporting))
	}
	if result.Weakening > 0 {
		parts = append(parts, fmt.Sprintf("%d weakening", result.Weakening))
	}
	if result.Refuting > 0 {
		parts = append(parts, fmt.Sprintf("%d REFUTING", result.Refuting))
	}
	parts = append(parts, fmt.Sprintf("R_eff: %.2f", result.REff))
	if result.MinFreshness != "" {
		if t, err := time.Parse(time.RFC3339, result.MinFreshness); err == nil {
			if t.Before(now) {
				parts = append(parts, "STALE evidence")
			} else {
				days := int(t.Sub(now).Hours() / 24)
				parts = append(parts, fmt.Sprintf("freshest expires in %dd", days))
			}
		}
	}
	if result.WeakestCL < 3 {
		clLabels := map[int]string{0: "opposed", 1: "different context", 2: "similar context"}
		parts = append(parts, fmt.Sprintf("weakest CL: %s", clLabels[result.WeakestCL]))
	}

	result.Summary = strings.Join(parts, ", ")
	return result
}

// scoreEvidence computes the effective reliability score for a single evidence item.
// FPF B.3: R_eff = max(0, base_score - Φ(CL)), with decay override for expired evidence.
func scoreEvidence(e EvidenceItem, now time.Time) float64 {
	// Expired evidence is weak regardless of verdict (FPF B.3.4)
	if e.ValidUntil != "" {
		if t, err := time.Parse(time.RFC3339, e.ValidUntil); err == nil && t.Before(now) {
			return 0.1
		}
	}

	base := verdictToScore(e.Verdict)
	penalty := clPenalty(e.CongruenceLevel)
	return math.Max(0, base-penalty)
}

// verdictToScore maps evidence verdict to base reliability score.
func verdictToScore(verdict string) float64 {
	switch verdict {
	case "supports", "accepted":
		return 1.0
	case "weakens", "partial":
		return 0.5
	case "refutes", "failed":
		return 0.0
	default:
		return 0.5 // unknown verdict treated as weakening
	}
}

// clPenalty returns the congruence level penalty per FPF B.3.
// CL3 (same context) = no penalty, CL0 (opposed) = near-total penalty.
func clPenalty(cl int) float64 {
	switch cl {
	case 3:
		return 0.0
	case 2:
		return 0.1
	case 1:
		return 0.4
	default: // CL=0 or invalid
		return 0.9
	}
}

// FormatDecisionResponse builds the MCP tool response.
func FormatDecisionResponse(action string, a *Artifact, filePath string, extra string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "decide":
		sb.WriteString(fmt.Sprintf("Decision recorded: %s\nID: %s\n", a.Meta.Title, a.Meta.ID))
		if a.Meta.ValidUntil != "" {
			sb.WriteString(fmt.Sprintf("Valid until: %s\n", a.Meta.ValidUntil))
		}
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
		sb.WriteString("\n---\n\n")
		sb.WriteString(a.Body)
	case "apply":
		sb.WriteString(extra)
	case "measure":
		sb.WriteString(fmt.Sprintf("Impact measured: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
		sb.WriteString(extra)
	case "evidence":
		sb.WriteString(extra)
	}

	sb.WriteString(navStrip)
	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
