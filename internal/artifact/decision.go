package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/codebase"

	"github.com/m0n0x41d/haft/internal/reff"
	"github.com/m0n0x41d/haft/logger"
)

// DecideInput is the input for creating a DecisionRecord.
type DecideInput struct {
	ProblemRef              string                  `json:"problem_ref,omitempty"`  // single problem (backward compat)
	ProblemRefs             []string                `json:"problem_refs,omitempty"` // multiple problems
	PortfolioRef            string                  `json:"portfolio_ref,omitempty"`
	ChoiceResult            *ChoiceResult           `json:"choice_result,omitempty"`
	TransformationRecord    *TransformationRecord   `json:"transformation_record,omitempty"`
	SelectedTitle           string                  `json:"selected_title"`
	WhySelected             string                  `json:"why_selected"`
	SelectionPolicy         string                  `json:"selection_policy"`
	CounterArgument         string                  `json:"counterargument"`
	WhyNotOthers            []RejectionReason       `json:"why_not_others,omitempty"`
	Invariants              []string                `json:"invariants,omitempty"`
	PreConditions           []string                `json:"pre_conditions,omitempty"`
	PostConditions          []string                `json:"post_conditions,omitempty"`
	Admissibility           []string                `json:"admissibility,omitempty"`
	EvidenceReqs            []string                `json:"evidence_requirements,omitempty"`
	Rollback                *RollbackSpec           `json:"rollback,omitempty"`
	RefreshTriggers         []string                `json:"refresh_triggers,omitempty"`
	WeakestLink             string                  `json:"weakest_link,omitempty"`
	ValidUntil              string                  `json:"valid_until,omitempty"`
	Context                 string                  `json:"context,omitempty"`
	TaskContext             string                  `json:"task_context,omitempty"`
	Mode                    string                  `json:"mode,omitempty"`
	SectionRefs             []string                `json:"section_refs,omitempty"`
	AffectedFiles           []string                `json:"affected_files,omitempty"`
	DecisionSubjectRef      string                  `json:"decision_subject_ref,omitempty"`
	ImplementationFootprint ImplementationFootprint `json:"implementation_footprint,omitempty"`
	GovernanceTargets       []GovernanceTarget      `json:"governance_targets,omitempty"`
	DriftWatchTargets       []DriftWatchTarget      `json:"drift_watch_targets,omitempty"`
	BindingTargets          []BindingTarget         `json:"binding_targets,omitempty"`
	BindingHints            []string                `json:"binding_hints,omitempty"`
	BindingScope            string                  `json:"binding_scope,omitempty"`
	BindingFallbackReason   string                  `json:"binding_fallback_reason,omitempty"`
	Predictions             []PredictionInput       `json:"predictions,omitempty"`
	Claims                  []DecisionClaim         `json:"claims,omitempty"`
	SearchKeywords          string                  `json:"search_keywords,omitempty"`
	FirstModuleCoverage     bool                    `json:"first_module_coverage,omitempty"`
	// GovernanceMode is "module" | "exact" | "" (default "module"). Controls
	// whether affected_files widen to module scope at baseline time.
	GovernanceMode string `json:"governance_mode,omitempty"`

	// Skips lists the DRR required fields the operator explicitly chose
	// to bypass for this decision. Only valid in tactical mode — using
	// Skips in standard/deep mode is rejected at validate time.
	//
	// Valid field names match the DRR required-field vocabulary:
	// "selection_policy", "counterargument", "weakest_link",
	// "why_not_others", "rollback", "predictions", "invariants",
	// "evidence_requirements", "refresh_triggers", "affected_files".
	//
	// Persisted in DecisionRecord StructuredData so the skip is
	// audit-visible after the fact (the operator acknowledged the gap
	// rather than the gap being silently absent).
	Skips      []string `json:"_skips,omitempty"`
	SkipReason string   `json:"_skip_reason,omitempty"`
}

// validRequiredFieldSkips is the canonical set of fields a tactical
// decision may explicitly skip. Names outside this set are rejected to
// prevent typos from silently disabling validation.
var validRequiredFieldSkips = map[string]bool{
	"selection_policy":      true,
	"counterargument":       true,
	"weakest_link":          true,
	"why_not_others":        true,
	"rollback":              true,
	"predictions":           true,
	"invariants":            true,
	"evidence_requirements": true,
	"refresh_triggers":      true,
	"affected_files":        true,
	"why_selected":          true,
}

// PredictionInput is a testable claim that measure should verify.
type PredictionInput struct {
	Claim         string `json:"claim"`
	Observable    string `json:"observable"`
	Threshold     string `json:"threshold"`
	VerifyAfter   string `json:"verify_after,omitempty"`  // RFC3339 or YYYY-MM-DD — when async evidence should be gathered
	Realizability string `json:"realizability,omitempty"` // C.28 verdict: realizable|nonrealizable|unknown
	// Probability is the optional elicited p(this claim holds) in [0,1] — a noisy
	// forecast sampled at /h-decide time, fed into decomposed-Brier calibration
	// once verified (dec-20260603-c3c7fa88). nil means the operator declined.
	Probability *float64 `json:"probability,omitempty"`
	// Command is the optional allowlist-class command form of Observable,
	// executable by the maintenance loop (test/build/vet/grep classes only).
	Command string `json:"command,omitempty"`
}

// RejectionReason explains why a variant was not selected.
type RejectionReason struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

// RollbackSpec defines when and how to reverse a decision.
type RollbackSpec struct {
	Triggers    []string `json:"triggers,omitempty"`
	Steps       []string `json:"steps,omitempty"`
	BlastRadius string   `json:"blast_radius,omitempty"`
}

// ApplyInput is the input for generating an implementation brief.
type ApplyInput struct {
	DecisionRef string `json:"decision_ref"`
}

// DecideContext holds pre-fetched data needed for pure decision construction.
type DecideContext struct {
	ID                string
	Now               time.Time
	Mode              Mode   // computed from chain (max of declared and inferred)
	Context           string // inherited from linked artifacts if not in input
	ProblemBody       string // pre-fetched problem markdown (fallback for older artifacts)
	ProblemStructured string // pre-fetched structured_data JSON (preferred, no re-parsing)
	Links             []Link
	ProblemRefs       []string // merged refs
}

// extractSection extracts a markdown section by heading from a body string. Pure.
func extractSection(body, heading string) string {
	marker := "## " + heading
	idx := strings.Index(body, marker)
	if idx == -1 {
		return ""
	}
	start := idx + len(marker)
	end := strings.Index(body[start:], "\n## ")
	if end > 0 {
		return strings.TrimSpace(body[start : start+end])
	}
	return strings.TrimSpace(body[start:])
}

func escapeMarkdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", "\\|")

	return value
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		compacted = append(compacted, trimmed)
	}

	return compacted
}

func normalizeRejectionReasons(values []RejectionReason) []RejectionReason {
	normalized := make([]RejectionReason, 0, len(values))

	for _, value := range values {
		variant := strings.TrimSpace(value.Variant)
		reason := strings.TrimSpace(value.Reason)
		if variant == "" && reason == "" {
			continue
		}

		normalized = append(normalized, RejectionReason{Variant: variant, Reason: reason})
	}

	return normalized
}

func normalizeRollbackSpec(spec *RollbackSpec) *RollbackSpec {
	if spec == nil {
		return nil
	}

	normalized := &RollbackSpec{
		Triggers:    compactStrings(spec.Triggers),
		Steps:       compactStrings(spec.Steps),
		BlastRadius: strings.TrimSpace(spec.BlastRadius),
	}

	if len(normalized.Triggers) == 0 && len(normalized.Steps) == 0 && normalized.BlastRadius == "" {
		return nil
	}

	return normalized
}

func normalizePredictionInputs(values []PredictionInput) []PredictionInput {
	normalized := make([]PredictionInput, 0, len(values))

	for _, value := range values {
		prediction := PredictionInput{
			Claim:         strings.TrimSpace(value.Claim),
			Observable:    strings.TrimSpace(value.Observable),
			Threshold:     strings.TrimSpace(value.Threshold),
			VerifyAfter:   strings.TrimSpace(value.VerifyAfter),
			Realizability: strings.TrimSpace(value.Realizability),
			Probability:   value.Probability,
			Command:       strings.TrimSpace(value.Command),
		}

		normalized = append(normalized, prediction)
	}

	return normalized
}

func normalizeDecisionInput(input DecideInput) DecideInput {
	input.ProblemRef = strings.TrimSpace(input.ProblemRef)
	input.ProblemRefs = compactStrings(input.ProblemRefs)
	input.PortfolioRef = strings.TrimSpace(input.PortfolioRef)
	input.SelectedTitle = strings.TrimSpace(input.SelectedTitle)
	input.WhySelected = strings.TrimSpace(input.WhySelected)
	input.SelectionPolicy = strings.TrimSpace(input.SelectionPolicy)
	input.CounterArgument = strings.TrimSpace(input.CounterArgument)
	input.WeakestLink = strings.TrimSpace(input.WeakestLink)
	input.ValidUntil = strings.TrimSpace(input.ValidUntil)
	input.Context = strings.TrimSpace(input.Context)
	input.TaskContext = strings.TrimSpace(input.TaskContext)
	input.Mode = strings.TrimSpace(input.Mode)
	input.SearchKeywords = strings.TrimSpace(input.SearchKeywords)
	input.ChoiceResult = NormalizeChoiceResult(input.ChoiceResult)
	input.TransformationRecord = NormalizeTransformationRecord(input.TransformationRecord)

	input.WhyNotOthers = normalizeRejectionReasons(input.WhyNotOthers)
	input.Invariants = compactStrings(input.Invariants)
	input.PreConditions = compactStrings(input.PreConditions)
	input.PostConditions = compactStrings(input.PostConditions)
	input.Admissibility = compactStrings(input.Admissibility)
	input.EvidenceReqs = compactStrings(input.EvidenceReqs)
	input.RefreshTriggers = compactStrings(input.RefreshTriggers)
	input.SectionRefs = compactStrings(input.SectionRefs)
	input.AffectedFiles = compactStrings(input.AffectedFiles)
	input.BindingTargets = normalizeBindingTargets(input.BindingTargets)
	input.BindingHints = compactStrings(input.BindingHints)
	input.BindingScope = strings.TrimSpace(input.BindingScope)
	input.BindingFallbackReason = strings.TrimSpace(input.BindingFallbackReason)
	input.Predictions = normalizePredictionInputs(input.Predictions)
	input.Claims = normalizeDecisionClaims(input.Claims)
	input.Rollback = normalizeRollbackSpec(input.Rollback)

	return input
}

func validateDecisionInput(input DecideInput) error {
	// Build skip set + validate skip names + reject skips outside tactical mode.
	skipSet, skipErr := buildSkipSet(input)
	if skipErr != nil {
		return skipErr
	}

	var missing []missingField

	addMissing := func(field, hint string) {
		if skipSet[field] {
			return
		}
		missing = append(missing, missingField{Field: field, Hint: hint})
	}

	if input.SelectedTitle == "" {
		// selected_title is not skippable — without it the decision has no
		// identity. Other required fields can be acknowledged-skipped in
		// tactical mode; this one cannot.
		missing = append(missing, missingField{
			Field: "selected_title",
			Hint:  "what variant was chosen? Required regardless of mode — a decision without a selection has no identity.",
		})
	}
	if input.WhySelected == "" {
		addMissing("why_selected", "rationale for the choice")
	}
	if input.SelectionPolicy == "" {
		addMissing("selection_policy", "the explicit policy used to choose this option (FPF CMP-02: declared BEFORE scoring)")
	}
	if input.CounterArgument == "" {
		addMissing("counterargument", "the strongest argument against this decision (FPF DEC-08: self-deception check)")
	}
	if input.WeakestLink == "" {
		addMissing("weakest_link", "what most plausibly breaks this choice (FPF X-WLNK)")
	}
	if len(input.WhyNotOthers) == 0 {
		addMissing("why_not_others", "at least one rejected alternative and why it lost (FPF CMP-04)")
	}
	if input.Rollback == nil || len(input.Rollback.Triggers) == 0 {
		addMissing("rollback", "at least one trigger that would force reversal (FPF DEC-05)")
	}
	if err := ValidateChoiceResult(input.ChoiceResult); err != nil {
		missing = append(missing, missingField{
			Field: "choice_result",
			Hint:  err.Error(),
		})
	}
	if err := ValidateTransformationRecord(input.TransformationRecord); err != nil {
		missing = append(missing, missingField{
			Field: "transformation_record",
			Hint:  err.Error(),
		})
	}

	// Structural checks on present fields — not subject to skip.
	for i, rejection := range input.WhyNotOthers {
		switch {
		case rejection.Variant == "":
			missing = append(missing, missingField{
				Field: fmt.Sprintf("why_not_others[%d].variant", i),
				Hint:  "name the rejected alternative",
			})
		case rejection.Reason == "":
			missing = append(missing, missingField{
				Field: fmt.Sprintf("why_not_others[%d].reason", i),
				Hint:  fmt.Sprintf("explain why %q lost", rejection.Variant),
			})
		case strings.EqualFold(rejection.Variant, input.SelectedTitle):
			missing = append(missing, missingField{
				Field: fmt.Sprintf("why_not_others[%d].variant", i),
				Hint:  fmt.Sprintf("must not repeat selected_title %q", input.SelectedTitle),
			})
		}
	}

	for i, prediction := range input.Predictions {
		for _, hint := range predictionValidationProblems(i, prediction) {
			missing = append(missing, missingField{
				Field: fmt.Sprintf("predictions[%d]", i),
				Hint:  hint,
			})
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return formatStructuredValidationError(missing, input)
}

// missingField pairs a missing-field name with a one-line hint about
// what the operator should supply.
type missingField struct {
	Field string
	Hint  string
}

// buildSkipSet validates the operator-supplied skip list and returns
// the set form for fast lookup. Returns a structured error when skips
// are used in standard/deep mode, when a skip names an unknown field,
// or when tactical-mode skips lack a reason.
func buildSkipSet(input DecideInput) (map[string]bool, error) {
	skipSet := map[string]bool{}
	if len(input.Skips) == 0 {
		return skipSet, nil
	}

	mode := strings.TrimSpace(input.Mode)
	if mode != string(ModeTactical) && mode != string(ModeNote) {
		return nil, fmt.Errorf(
			"_skips is only valid in tactical or note mode (got mode=%q); "+
				"standard and deep decisions cannot bypass required DRR fields; "+
				"to skip fields, switch to tactical mode with mode=\"tactical\"",
			mode,
		)
	}
	if strings.TrimSpace(input.SkipReason) == "" {
		return nil, fmt.Errorf(
			"_skip_reason is required when _skips is non-empty — explain why " +
				"the operator chose to bypass required fields; the skip + reason " +
				"is persisted in the DecisionRecord audit trail")
	}

	var unknown []string
	for _, field := range input.Skips {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if !validRequiredFieldSkips[field] {
			unknown = append(unknown, field)
			continue
		}
		skipSet[field] = true
	}
	if len(unknown) > 0 {
		valid := make([]string, 0, len(validRequiredFieldSkips))
		for f := range validRequiredFieldSkips {
			valid = append(valid, f)
		}
		sort.Strings(valid)
		return nil, fmt.Errorf(
			"_skips contains unknown field(s): %s. Valid skippable fields: %s",
			strings.Join(unknown, ", "),
			strings.Join(valid, ", "),
		)
	}

	return skipSet, nil
}

// formatStructuredValidationError builds the human+LLM-readable error
// returned to MCP callers when validation fails. The shape carries:
//   - the failed gate identity (decision DRR completeness)
//   - the missing fields with per-field hints
//   - how-to-proceed options (provide fields OR switch to tactical with skip)
//   - FPF spec references the operator can look up
//
// Plain text on purpose — JSON-in-text hurts LLM readability and skill
// bodies parse the human form just as well.
func formatStructuredValidationError(missing []missingField, input DecideInput) error {
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = string(ModeStandard)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "FPF discipline violation: decision in %s mode is incomplete.\n\n", mode)

	b.WriteString("Missing required fields:\n")
	for _, m := range missing {
		fmt.Fprintf(&b, "- %s — %s\n", m.Field, m.Hint)
	}

	b.WriteString("\nHow to proceed:\n")
	b.WriteString("- Option 1: Provide the missing fields and retry the call.\n")
	if mode == string(ModeStandard) || mode == string(ModeDeep) {
		b.WriteString("- Option 2: If this is a tactical change (<2-week reversible blast radius),\n")
		b.WriteString("  switch to tactical mode and explicitly acknowledge the skip:\n")
		b.WriteString("    \"mode\": \"tactical\",\n")
		b.WriteString("    \"_skips\": [\"<field1>\", \"<field2>\"],\n")
		b.WriteString("    \"_skip_reason\": \"<why the operator accepts the gap>\"\n")
		b.WriteString("  The skip + reason are persisted in the DecisionRecord audit trail.\n")
	} else {
		b.WriteString("- Option 2: Add the field name to _skips with a _skip_reason if the\n")
		b.WriteString("  operator explicitly chooses to bypass it for this tactical change.\n")
	}

	b.WriteString("\nReferences:\n")
	b.WriteString("- FPF E.9 — Design Rationale Record minimum kernel\n")
	b.WriteString("- FPF C.11 — Decision Theory requirements\n")
	b.WriteString("- haft_query(action=\"fpf\", query=\"E.9\") — full pattern text\n")
	b.WriteString("- haft_query(action=\"fpf\", query=\"DEC-01\") — DRR structure micro-pattern\n")

	return fmt.Errorf("%s", b.String())
}

func predictionValidationProblems(index int, prediction PredictionInput) []string {
	problems := []string{}
	missingCount := 0

	if prediction.Claim == "" {
		missingCount++
		problems = append(problems, fmt.Sprintf("predictions[%d].claim is required — predictions must include claim, observable, and threshold together", index))
	}
	if prediction.Observable == "" {
		missingCount++
		problems = append(problems, fmt.Sprintf("predictions[%d].observable is required — predictions must include claim, observable, and threshold together", index))
	}
	if prediction.Threshold == "" {
		missingCount++
		problems = append(problems, fmt.Sprintf("predictions[%d].threshold is required — predictions must include claim, observable, and threshold together", index))
	}
	if missingCount > 0 {
		problems = append([]string{
			fmt.Sprintf("predictions[%d] must include claim, observable, and threshold together", index),
		}, problems...)
	}
	if p := prediction.Probability; p != nil && (*p < 0 || *p > 1) {
		problems = append(problems, fmt.Sprintf("predictions[%d].probability must be in [0,1], got %v", index, *p))
	}

	return problems
}

func renderTransformationRecord(body *strings.Builder, record *TransformationRecord) {
	normalized := NormalizeTransformationRecord(record)
	if normalized == nil {
		return
	}

	body.WriteString("**Transformation record:**\n")
	body.WriteString("- Kind: target-state transformation only; not method, work authorization, evidence, or publication.\n")
	body.WriteString(fmt.Sprintf("- Transformed entity: %s\n", normalized.TransformedEntity))
	body.WriteString(fmt.Sprintf("- Initial state: %s\n", normalized.InitialState))
	body.WriteString(fmt.Sprintf("- Post state: %s\n", normalized.PostState))
	body.WriteString(fmt.Sprintf("- Relation: %s\n", normalized.Relation))
	body.WriteString(fmt.Sprintf("- Context: %s\n", normalized.Context))
	if normalized.Window != "" {
		body.WriteString(fmt.Sprintf("- Window: %s\n", normalized.Window))
	}
	renderTransformationRefs(body, "Method refs", normalized.MethodRefs)
	renderTransformationRefs(body, "Work refs", normalized.WorkRefs)
	renderTransformationRefs(body, "Evidence refs", normalized.EvidenceRefs)
	renderTransformationRefs(body, "Publication refs", normalized.PublicationRefs)
	body.WriteString("\n")
}

func renderTransformationRefs(body *strings.Builder, label string, refs []string) {
	if len(refs) == 0 {
		return
	}

	body.WriteString(fmt.Sprintf("- %s: %s\n", label, strings.Join(refs, ", ")))
}

// BuildDecisionArtifact constructs a DecisionRecord from input and pre-fetched context. Pure — no side effects.
func BuildDecisionArtifact(dctx DecideContext, input DecideInput) (*Artifact, error) {
	input = normalizeDecisionInput(input)

	if err := validateDecisionInput(input); err != nil {
		return nil, err
	}

	title := input.SelectedTitle

	// Build the DRR markdown — FPF E.9 four-component structure
	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n", title))

	// === Component 1: Problem Frame ===
	body.WriteString("\n## 1. Problem Frame\n\n")
	if dctx.ProblemStructured != "" {
		// Prefer structured data — canonical, no re-parsing
		var pf ProblemFields
		if err := json.Unmarshal([]byte(dctx.ProblemStructured), &pf); err == nil {
			if pf.Signal != "" {
				body.WriteString(fmt.Sprintf("**Signal:** %s\n\n", pf.Signal))
			}
			if len(pf.Constraints) > 0 {
				body.WriteString("**Constraints:**\n")
				for _, c := range pf.Constraints {
					body.WriteString(fmt.Sprintf("- %s\n", c))
				}
				body.WriteString("\n")
			}
			if pf.Acceptance != "" {
				body.WriteString(fmt.Sprintf("**Acceptance:** %s\n\n", pf.Acceptance))
			}
		}
	} else if dctx.ProblemBody != "" {
		// Fallback: parse markdown for older artifacts without structured_data
		if signal := extractSection(dctx.ProblemBody, "Signal"); signal != "" {
			body.WriteString(fmt.Sprintf("**Signal:** %s\n\n", signal))
		}
		if constraints := extractSection(dctx.ProblemBody, "Constraints"); constraints != "" {
			body.WriteString(fmt.Sprintf("**Constraints:**\n%s\n\n", constraints))
		}
		if acceptance := extractSection(dctx.ProblemBody, "Acceptance"); acceptance != "" {
			body.WriteString(fmt.Sprintf("**Acceptance:** %s\n\n", acceptance))
		}
	}

	// === Component 2: Decision (the contract) ===
	body.WriteString("## 2. Decision\n\n")
	body.WriteString(fmt.Sprintf("**Selected:** %s\n\n", input.SelectedTitle))
	body.WriteString(fmt.Sprintf("**Selection policy:** %s\n\n", input.SelectionPolicy))
	body.WriteString(fmt.Sprintf("**Why selected:** %s\n\n", input.WhySelected))
	if input.TransformationRecord != nil {
		renderTransformationRecord(&body, input.TransformationRecord)
	}

	rollbackTriggers := []string(nil)
	rollbackSteps := []string(nil)
	rollbackBlastRadius := ""
	if input.Rollback != nil {
		rollbackTriggers = input.Rollback.Triggers
		rollbackSteps = input.Rollback.Steps
		rollbackBlastRadius = input.Rollback.BlastRadius
	}

	choiceResult := input.ChoiceResult
	if choiceResult == nil {
		choiceResult = NewDecisionChoiceResult(DecisionChoiceResultInput{
			ProblemRefs:     dctx.ProblemRefs,
			PortfolioRef:    input.PortfolioRef,
			SelectedTitle:   input.SelectedTitle,
			WhySelected:     input.WhySelected,
			WhyNotOthers:    input.WhyNotOthers,
			SelectionPolicy: input.SelectionPolicy,
			ReopenCondition: choiceReopenCondition(rollbackTriggers),
		})
	}

	decisionFields := DecisionFields{
		ProblemRefs:             dctx.ProblemRefs,
		DecisionSubjectRef:      strings.TrimSpace(input.DecisionSubjectRef),
		ChoiceResult:            NormalizeChoiceResult(choiceResult),
		TransformationRecord:    NormalizeTransformationRecord(input.TransformationRecord),
		SelectedTitle:           input.SelectedTitle,
		WhySelected:             input.WhySelected,
		SelectionPolicy:         input.SelectionPolicy,
		CounterArgument:         input.CounterArgument,
		WeakestLink:             input.WeakestLink,
		TaskContext:             sanitizeIDSlug(input.TaskContext),
		SectionRefs:             input.SectionRefs,
		WhyNotOthers:            input.WhyNotOthers,
		Claims:                  decisionInputClaims(input),
		PreConditions:           input.PreConditions,
		RollbackTriggers:        rollbackTriggers,
		RollbackSteps:           rollbackSteps,
		RollbackBlastRadius:     rollbackBlastRadius,
		Invariants:              input.Invariants,
		PostConds:               input.PostConditions,
		Admissibility:           input.Admissibility,
		EvidenceRequirements:    input.EvidenceReqs,
		RefreshTriggers:         input.RefreshTriggers,
		Skips:                   cloneStringSlice(input.Skips),
		SkipReason:              input.SkipReason,
		FirstModuleCoverage:     input.FirstModuleCoverage,
		ImplementationFootprint: normalizeImplementationFootprint(input.ImplementationFootprint),
		GovernanceTargets:       normalizeGovernanceTargets(input.GovernanceTargets),
		DriftWatchTargets:       normalizeDriftWatchTargets(input.DriftWatchTargets),
		GovernanceMode:          GovernanceMode(strings.TrimSpace(input.GovernanceMode)),
		BindingTargets:          normalizeBindingTargets(input.BindingTargets),
	}
	decisionFields.Predictions = decisionPredictionsFromClaims(decisionFields.Claims)

	if len(input.Invariants) > 0 {
		body.WriteString("\n**Invariants:**\n")
		for _, inv := range input.Invariants {
			body.WriteString(fmt.Sprintf("- %s\n", inv))
		}
	}

	if len(input.SectionRefs) > 0 {
		body.WriteString("\n**Spec sections:**\n")
		for _, ref := range input.SectionRefs {
			body.WriteString(fmt.Sprintf("- %s\n", ref))
		}
	}

	if len(decisionFields.PreConditions) > 0 {
		body.WriteString("\n**Pre-conditions:**\n")
		for _, pc := range decisionFields.PreConditions {
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
	body.WriteString(fmt.Sprintf("**Counterargument:** %s\n\n", input.CounterArgument))
	body.WriteString(fmt.Sprintf("**Selected variant weakest link:** %s\n\n", input.WeakestLink))
	if len(input.WhyNotOthers) > 0 {
		body.WriteString("**Rejected alternatives:**\n")
		body.WriteString("| Variant | Verdict | Reason |\n")
		body.WriteString("|---------|---------|--------|\n")
		body.WriteString(fmt.Sprintf(
			"| %s | **Selected** | %s |\n",
			escapeMarkdownTableCell(input.SelectedTitle),
			escapeMarkdownTableCell(truncate(input.WhySelected, 60)),
		))
		for _, r := range input.WhyNotOthers {
			body.WriteString(fmt.Sprintf(
				"| %s | Rejected | %s |\n",
				escapeMarkdownTableCell(r.Variant),
				escapeMarkdownTableCell(r.Reason),
			))
		}
		body.WriteString("\n")
	}

	if len(decisionFields.EvidenceRequirements) > 0 {
		body.WriteString("**Evidence requirements:**\n")
		for _, e := range decisionFields.EvidenceRequirements {
			body.WriteString(fmt.Sprintf("- %s\n", e))
		}
		body.WriteString("\n")
	}

	if len(decisionFields.Claims) > 0 {
		body.WriteString("**Predictions:**\n")
		body.WriteString("| Claim | Observable | Threshold |\n")
		body.WriteString("|-------|------------|-----------|\n")
		for _, claim := range decisionFields.Claims {
			body.WriteString(fmt.Sprintf(
				"| %s | %s | %s |\n",
				escapeMarkdownTableCell(claim.Claim),
				escapeMarkdownTableCell(claim.Observable),
				escapeMarkdownTableCell(claim.Threshold),
			))
		}
		body.WriteString("\n")
	}

	// === Component 4: Consequences ===
	body.WriteString("## 4. Consequences\n\n")

	if input.Rollback != nil {
		body.WriteString("**Rollback plan:**\n")
		body.WriteString("Triggers:\n")
		for _, t := range input.Rollback.Triggers {
			body.WriteString(fmt.Sprintf("- %s\n", t))
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

	if len(decisionFields.RefreshTriggers) > 0 {
		body.WriteString("**Refresh triggers:**\n")
		for _, rt := range decisionFields.RefreshTriggers {
			body.WriteString(fmt.Sprintf("- %s\n", rt))
		}
		body.WriteString("\n")
	}

	if len(input.AffectedFiles) > 0 {
		body.WriteString("**Affected files:** ")
		body.WriteString(strings.Join(input.AffectedFiles, ", "))
		body.WriteString("\n")
	}
	if len(input.BindingTargets) > 0 {
		decisionFields.BindingTargets = normalizeBindingTargets(input.BindingTargets)
	}

	a := &Artifact{
		Meta: Meta{
			ID:         dctx.ID,
			Kind:       KindDecisionRecord,
			Version:    1,
			Status:     StatusActive,
			Context:    dctx.Context,
			Mode:       dctx.Mode,
			Title:      title,
			ValidUntil: input.ValidUntil,
			CreatedAt:  dctx.Now,
			UpdatedAt:  dctx.Now,
			Links:      dctx.Links,
		},
		Body:           body.String(),
		SearchKeywords: input.SearchKeywords,
	}

	sd, _ := json.Marshal(decisionFields)
	a.StructuredData = string(sd)

	return a, nil
}

func choiceReopenCondition(rollbackTriggers []string) string {
	triggers := compactStrings(rollbackTriggers)
	if len(triggers) == 0 {
		return ""
	}

	return "reopen choice if rollback triggers occur: " + strings.Join(triggers, "; ")
}

// MergeProblemRefs merges single ProblemRef with ProblemRefs array, deduplicating. Pure.
func MergeProblemRefs(single string, multiple []string) []string {
	refs := make([]string, len(multiple))
	copy(refs, multiple)
	if single != "" {
		found := false
		for _, r := range refs {
			if r == single {
				found = true
				break
			}
		}
		if !found {
			refs = append(refs, single)
		}
	}
	return refs
}

func decisionBlocksReplacement(status Status) bool {
	return status == StatusActive || status == StatusRefreshDue
}

// ResolvePortfolioProblemRefs returns problem refs linked to a portfolio.
func ResolvePortfolioProblemRefs(portfolio *Artifact) []string {
	if portfolio == nil {
		return nil
	}

	resolvedRefs := []string{}
	fields := portfolio.UnmarshalPortfolioFields()

	if fields.ProblemRef != "" {
		resolvedRefs = appendUniqueString(resolvedRefs, fields.ProblemRef)
	}

	for _, link := range portfolio.Meta.Links {
		if link.Type != "based_on" {
			continue
		}
		if !strings.HasPrefix(link.Ref, KindProblemCard.IDPrefix()+"-") {
			continue
		}

		resolvedRefs = appendUniqueString(resolvedRefs, link.Ref)
	}

	sort.Strings(resolvedRefs)
	return resolvedRefs
}

func resolveDecisionProblemRefs(ctx context.Context, store ArtifactStore, decision *Artifact) []string {
	if decision == nil {
		return nil
	}

	resolvedRefs := cloneStringSlice(decision.UnmarshalDecisionFields().ProblemRefs)

	for _, link := range decision.Meta.Links {
		if link.Type != "based_on" {
			continue
		}

		if strings.HasPrefix(link.Ref, KindProblemCard.IDPrefix()+"-") {
			resolvedRefs = appendUniqueString(resolvedRefs, link.Ref)
			continue
		}

		linkedArtifact, err := store.Get(ctx, link.Ref)
		if err != nil {
			continue
		}
		if linkedArtifact.Meta.Kind != KindSolutionPortfolio {
			continue
		}

		for _, problemRef := range ResolvePortfolioProblemRefs(linkedArtifact) {
			resolvedRefs = appendUniqueString(resolvedRefs, problemRef)
		}
	}

	sort.Strings(resolvedRefs)
	return resolvedRefs
}

func resolveIncomingDecisionProblemRefs(
	ctx context.Context,
	store ArtifactStore,
	problemRefs []string,
	portfolioRef string,
) []string {
	resolvedRefs := cloneStringSlice(problemRefs)

	if portfolioRef == "" {
		sort.Strings(resolvedRefs)
		return resolvedRefs
	}

	portfolio, err := store.Get(ctx, portfolioRef)
	if err != nil || portfolio.Meta.Kind != KindSolutionPortfolio {
		sort.Strings(resolvedRefs)
		return resolvedRefs
	}

	for _, problemRef := range ResolvePortfolioProblemRefs(portfolio) {
		resolvedRefs = appendUniqueString(resolvedRefs, problemRef)
	}

	sort.Strings(resolvedRefs)
	return resolvedRefs
}

func validateNoActiveDecisionConflict(
	ctx context.Context,
	store ArtifactStore,
	problemRefs []string,
	portfolioRef string,
) error {
	resolvedIncomingRefs := resolveIncomingDecisionProblemRefs(ctx, store, problemRefs, portfolioRef)

	if len(resolvedIncomingRefs) == 0 {
		return nil
	}

	decisions, err := store.ListByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		return fmt.Errorf("list decision records: %w", err)
	}

	for _, decision := range decisions {
		if !decisionBlocksReplacement(decision.Meta.Status) {
			continue
		}

		fullDecision, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			continue
		}

		for _, existingProblemRef := range resolveDecisionProblemRefs(ctx, store, fullDecision) {
			if !slicesContains(resolvedIncomingRefs, existingProblemRef) {
				continue
			}

			return fmt.Errorf(
				"problem_ref %s already has live DecisionRecord %s (%s) — supersede the previous decision or close the problem first",
				existingProblemRef,
				fullDecision.Meta.ID,
				fullDecision.Meta.Title,
			)
		}
	}

	return nil
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value != target {
			continue
		}

		return true
	}

	return false
}

// BuildLinks constructs artifact links from problem refs and portfolio ref. Pure.
func BuildLinks(problemRefs []string, portfolioRef string) []Link {
	var links []Link
	for _, ref := range problemRefs {
		links = append(links, Link{Ref: ref, Type: "based_on"})
	}
	if portfolioRef != "" {
		links = append(links, Link{Ref: portfolioRef, Type: "based_on"})
	}
	return links
}

func BuildLinksWithSectionRefs(problemRefs []string, portfolioRef string, sectionRefs []string) []Link {
	links := BuildLinks(problemRefs, portfolioRef)
	sectionLinks := BuildSpecSectionLinks(sectionRefs)
	links = append(links, sectionLinks...)

	return links
}

func BuildSpecSectionLinks(sectionRefs []string) []Link {
	links := make([]Link, 0, len(sectionRefs))
	seen := map[string]struct{}{}

	for _, ref := range compactStrings(sectionRefs) {
		if _, ok := seen[ref]; ok {
			continue
		}

		seen[ref] = struct{}{}
		links = append(links, Link{Ref: ref, Type: "governs"})
	}

	return links
}

// Decide creates a DecisionRecord artifact. Orchestrates effects around BuildDecisionArtifact.
func Decide(ctx context.Context, store ArtifactStore, haftDir string, input DecideInput) (*Artifact, string, error) {
	input = normalizeDecisionInput(input)
	input = enrichDecisionInputBindingTargets(projectRootFromHaftDir(haftDir), input)

	if _, err := ParseGovernanceMode(input.GovernanceMode); err != nil {
		return nil, "", err
	}

	problemRefs := MergeProblemRefs(input.ProblemRef, input.ProblemRefs)
	if err := validateNoActiveDecisionConflict(ctx, store, problemRefs, input.PortfolioRef); err != nil {
		return nil, "", err
	}

	// GenerateID uses a crypto/rand suffix since #63; no need for sequence
	// lookup. The seq parameter is preserved on GenerateID for call-site
	// backward compat — pass 0.
	id := GenerateIDWithTaskContext(KindDecisionRecord, 0, input.TaskContext)
	now := time.Now().UTC()

	// Pure: merge refs
	links := BuildLinksWithSectionRefs(problemRefs, input.PortfolioRef, input.SectionRefs)

	// Effects: compute mode from chain
	var declaredMode Mode
	if input.Mode == "" {
		declaredMode = ModeStandard
	} else {
		var err error
		declaredMode, err = ParseMode(input.Mode)
		if err != nil {
			return nil, "", fmt.Errorf("%w (valid: note, tactical, standard, deep)", err)
		}
	}
	chainMode := inferModeFromChain(ctx, store, problemRefs, input.PortfolioRef)
	mode := maxMode(declaredMode, chainMode)

	// Effects: inherit context from linked artifacts
	resolvedContext := input.Context
	if resolvedContext == "" {
		if input.PortfolioRef != "" {
			if p, err := store.Get(ctx, input.PortfolioRef); err == nil {
				resolvedContext = p.Meta.Context
			}
		} else if len(problemRefs) > 0 {
			if p, err := store.Get(ctx, problemRefs[0]); err == nil {
				resolvedContext = p.Meta.Context
			}
		}
	}

	// Effects: pre-fetch problem body
	primaryRef := input.ProblemRef
	if primaryRef == "" && len(problemRefs) > 0 {
		primaryRef = problemRefs[0]
	}
	var problemBody, problemStructured string
	if primaryRef != "" {
		if prob, err := store.Get(ctx, primaryRef); err == nil {
			problemBody = prob.Body
			problemStructured = prob.StructuredData
		}
	}

	// Pure construction
	a, err := BuildDecisionArtifact(DecideContext{
		ID:                id,
		Now:               now,
		Mode:              mode,
		Context:           resolvedContext,
		ProblemBody:       problemBody,
		ProblemStructured: problemStructured,
		Links:             links,
		ProblemRefs:       problemRefs,
	}, input)
	if err != nil {
		return nil, "", err
	}

	// Effects: persist
	if err := store.Create(ctx, a); err != nil {
		return nil, "", fmt.Errorf("store decision: %w", err)
	}

	logger.ArtifactOp("create", id, string(KindDecisionRecord))

	var warnings []string

	if len(input.AffectedFiles) > 0 {
		warnings = append(warnings, WarnSharedFiles(input.AffectedFiles)...)
		var files []AffectedFile
		for _, f := range input.AffectedFiles {
			files = append(files, AffectedFile{Path: f})
		}
		if err := store.SetAffectedFiles(ctx, id, files); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to track affected files: %v", err))
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

func projectRootFromHaftDir(haftDir string) string {
	clean := filepath.Clean(strings.TrimSpace(haftDir))
	if filepath.Base(clean) != ".haft" {
		return ""
	}
	return filepath.Dir(clean)
}

func enrichDecisionInputBindingTargets(projectRoot string, input DecideInput) DecideInput {
	if projectRoot == "" || len(input.AffectedFiles) == 0 || len(input.BindingTargets) > 0 {
		return input
	}

	files := make([]AffectedFile, 0, len(input.AffectedFiles))
	for _, path := range input.AffectedFiles {
		files = append(files, AffectedFile{Path: path})
	}
	resolution, err := ResolveBindingTargets(projectRoot, files, BindingResolutionOptions{
		Hints:          input.BindingHints,
		Scope:          input.BindingScope,
		FallbackReason: input.BindingFallbackReason,
		DecisionText:   decisionBindingResolutionText(input),
	})
	if err != nil || len(resolution.Targets) == 0 {
		return input
	}

	input.BindingTargets = resolution.Targets
	return input
}

func decisionBindingResolutionText(input DecideInput) string {
	fragments := []string{
		input.SelectedTitle,
		input.WhySelected,
		input.Context,
		input.TaskContext,
	}
	fragments = append(fragments, input.Invariants...)
	fragments = append(fragments, input.PreConditions...)
	fragments = append(fragments, input.PostConditions...)
	return strings.Join(compactStrings(fragments), "\n")
}

// BaselineInput is the input for snapshotting file hashes after implementation.
type BaselineInput struct {
	DecisionRef           string          `json:"decision_ref"`
	AffectedFiles         []string        `json:"affected_files,omitempty"` // optional: replace file list before hashing
	BindingTargets        []BindingTarget `json:"binding_targets,omitempty"`
	BindingHints          []string        `json:"binding_hints,omitempty"`
	BindingScope          string          `json:"binding_scope,omitempty"`
	BindingFallbackReason string          `json:"binding_fallback_reason,omitempty"`
}

func baselineBindingResolutionTargets(projectRoot string, input BaselineInput, fields DecisionFields) []BindingTarget {
	if len(input.BindingTargets) > 0 {
		return input.BindingTargets
	}
	if baselineInputRequestsBindingResolution(input) {
		return nil
	}
	return hydrateBaselineBindingTargets(projectRoot, fields.EffectiveDriftBindingTargets())
}

func baselineInputRequestsBindingResolution(input BaselineInput) bool {
	return len(input.BindingHints) > 0 ||
		strings.TrimSpace(input.BindingScope) != "" ||
		strings.TrimSpace(input.BindingFallbackReason) != ""
}

func hydrateBaselineBindingTargets(projectRoot string, targets []BindingTarget) []BindingTarget {
	out := make([]BindingTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, hydrateBaselineBindingTarget(projectRoot, target))
	}
	return normalizeBindingTargets(out)
}

func hydrateBaselineBindingTarget(projectRoot string, target BindingTarget) BindingTarget {
	if target.Kind != BindingTargetSymbol {
		return target
	}
	if strings.TrimSpace(target.FilePath) == "" || strings.TrimSpace(target.SymbolName) == "" {
		return target
	}

	snapshots, err := codebase.ExtractSymbolSnapshots(projectRoot, target.FilePath)
	if err != nil {
		return target
	}
	for _, snapshot := range snapshots {
		if !symbolSnapshotSameIdentity(snapshot, target) {
			continue
		}
		source := strings.TrimSpace(target.ResolutionSource)
		if source == "" {
			source = BindingResolutionSourceExplicitTargets
		}
		language := strings.TrimSpace(target.Language)
		if language == "" {
			if detected, ok := codebase.LanguageForPath(target.FilePath); ok {
				language = detected
			}
		}
		return symbolBindingTarget(snapshot, language, source)
	}
	return target
}

// Baseline snapshots the current state of affected files as the baseline for drift detection.
// If AffectedFiles is provided, it replaces the existing file list before hashing.
func Baseline(ctx context.Context, store ArtifactStore, projectRoot string, input BaselineInput) ([]AffectedFile, error) {
	if input.DecisionRef == "" {
		return nil, fmt.Errorf("decision_ref is required")
	}

	a, err := store.Get(ctx, input.DecisionRef)
	if err != nil {
		return nil, fmt.Errorf("decision %s not found: %w", input.DecisionRef, err)
	}
	if a.Meta.Kind != KindDecisionRecord && a.Meta.Kind != KindNote {
		return nil, fmt.Errorf("%s is %s — baseline only works on decisions and notes", input.DecisionRef, a.Meta.Kind)
	}

	files, err := store.GetAffectedFiles(ctx, input.DecisionRef)
	if err != nil {
		return nil, fmt.Errorf("get affected files: %w", err)
	}
	if len(input.AffectedFiles) > 0 {
		files = nil
		for _, f := range input.AffectedFiles {
			files = append(files, AffectedFile{Path: f})
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("decision %s has no affected_files — nothing to baseline", input.DecisionRef)
	}

	// Compute SHA-256 for each file (skip directories)
	var hashableFiles []AffectedFile
	for i := range files {
		absPath := filepath.Join(projectRoot, files[i].Path)
		hash, err := hashFile(absPath)
		if err != nil {
			// Skip directories and missing files gracefully
			logger.Debug().Str("path", files[i].Path).Err(err).Msg("baseline.skip_file")
			continue
		}
		files[i].Hash = hash
		hashableFiles = append(hashableFiles, files[i])
	}
	files = hashableFiles

	decisionFields := a.UnmarshalDecisionFields()
	decisionText := strings.Join([]string{
		a.Meta.Title,
		decisionFields.SelectedTitle,
		decisionFields.WhySelected,
		decisionFields.SelectionPolicy,
		decisionFields.WeakestLink,
		strings.Join(decisionFields.Invariants, "\n"),
		strings.Join(decisionFields.PostConds, "\n"),
	}, "\n")
	bindingTargets := baselineBindingResolutionTargets(projectRoot, input, decisionFields)
	resolution, err := ResolveBindingTargets(projectRoot, files, BindingResolutionOptions{
		ExplicitTargets: bindingTargets,
		Hints:           input.BindingHints,
		Scope:           input.BindingScope,
		FallbackReason:  input.BindingFallbackReason,
		DecisionText:    decisionText,
	})
	decisionFields.BindingDiagnostics = resolution.Diagnostics
	if err != nil {
		if persistErr := persistDecisionFields(ctx, store, a, decisionFields); persistErr != nil {
			logger.Warn().Str("decision_ref", input.DecisionRef).Err(persistErr).Msg("baseline.binding_diagnostics_failed")
		}
		return nil, err
	}
	decisionFields.BindingTargets = resolution.Targets

	if err := store.SetAffectedFiles(ctx, input.DecisionRef, files); err != nil {
		return nil, fmt.Errorf("store baseline hashes: %w", err)
	}

	if len(resolution.Symbols) > 0 {
		if err := store.SetAffectedSymbols(ctx, input.DecisionRef, resolution.Symbols); err != nil {
			logger.Warn().Str("decision_ref", input.DecisionRef).Err(err).Msg("baseline.symbols_failed")
		}
	}

	// Drift manifests are only built for module-mode governance. Exact-mode
	// decisions track only the listed files; siblings are not auto-captured
	// as drift. This honors X-SCOPE: explicit files mean explicit files.
	mode := a.UnmarshalDecisionFields().EffectiveGovernanceMode()
	var driftManifests []DriftScopeManifest
	if mode == GovernanceModeModule {
		driftManifests, err = buildDriftScopeManifests(projectRoot, files)
		if err != nil {
			return nil, fmt.Errorf("build drift manifests: %w", err)
		}
	}

	decisionFields.DriftManifests = driftManifests
	if err := persistDecisionFields(ctx, store, a, decisionFields); err != nil {
		return nil, fmt.Errorf("persist decision fields: %w", err)
	}

	logger.ArtifactOp("baseline", input.DecisionRef, string(a.Meta.Kind))
	logger.Debug().Str("decision_ref", input.DecisionRef).
		Int("files", len(files)).
		Int("symbols", len(resolution.Symbols)).
		Int("binding_targets", len(resolution.Targets)).
		Msg("baseline.complete")

	return files, nil
}

// CheckDrift compares current file state against stored baseline hashes for all active decisions.
func CheckDrift(ctx context.Context, store ArtifactStore, projectRoot string) ([]DriftReport, error) {
	decisions, err := store.ListActiveByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}

	// Notes are observations, not implementations — skip baseline/drift checks for them

	var reports []DriftReport
	// Memoize the per-scope tree walk across decisions: many decisions share the
	// whole-repo "." scope, so without this /h-status re-walks the entire tree
	// once per such decision — the dominant cost of the session-mandatory status
	// check. One walk per distinct scope per drift pass.
	scopeCache := map[string][]string{}

	for _, d := range decisions {
		decisionArtifact, err := store.Get(ctx, d.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("get decision %s: %w", d.Meta.ID, err)
		}
		decisionFields := decisionArtifact.UnmarshalDecisionFields()
		if decisionFields.IsImplementationFootprintOnly() {
			continue
		}
		evidenceItems, err := store.GetEvidenceItems(ctx, d.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("get evidence for decision %s: %w", d.Meta.ID, err)
		}

		files, err := store.GetAffectedFiles(ctx, d.Meta.ID)
		if err != nil || len(files) == 0 {
			continue
		}

		// Load the stored symbol-level baseline once per decision and group it
		// by file. Activates the symbol-drift path so CheckDrift can partition a
		// modified file into added-only (benign) vs governed-body-modified.
		baselineSymbolsByFile := groupSymbolsByFile(store.GetAffectedSymbols(ctx, d.Meta.ID))
		bindingTargetsByFile := groupBindingTargetsByFile(decisionFields.EffectiveDriftBindingTargets())

		report := DriftReport{
			DecisionID:    d.Meta.ID,
			DecisionTitle: decisionArtifact.Meta.Title,
		}

		// Check if any file has a baseline hash
		hasAnyHash := false
		for _, f := range files {
			if f.Hash != "" {
				hasAnyHash = true
				break
			}
		}
		report.HasBaseline = hasAnyHash
		if hasAnyHash {
			profile := VerifiedStateBaselineProfile()
			report.BaselineKind = profile.Kind
			report.BaselineProfile = &profile
		}

		if !hasAnyHash {
			// No baseline set — check git to distinguish "forgot to close loop" from "not started"
			anyChanged := false
			for _, f := range files {
				report.Files = append(report.Files, DriftItem{
					Path:        f.Path,
					Status:      DriftNoBaseline,
					Materiality: noBaselineMateriality(bindingTargetsByFile[f.Path]),
					TriggerKind: DriftTriggerMissingBaseline,
				})
				if projectRoot != "" && gitFileModifiedSince(projectRoot, f.Path, d.Meta.CreatedAt) {
					anyChanged = true
				}
			}
			report.LikelyImplemented = anyChanged
			reports = append(reports, report)
			continue
		}

		// Compare current state to baseline
		hasDrift := false
		for _, f := range files {
			if f.Hash == "" {
				// File was added to affected_files after baseline — treat as no_baseline
				report.Files = append(report.Files, DriftItem{
					Path:        f.Path,
					Status:      DriftNoBaseline,
					Materiality: noBaselineMateriality(bindingTargetsByFile[f.Path]),
					TriggerKind: DriftTriggerMissingBaseline,
				})
				continue
			}

			absPath := filepath.Join(projectRoot, f.Path)
			currentHash, err := hashFile(absPath)
			if err != nil {
				// File doesn't exist or can't be read
				assessment := assessMissingFileDrift(projectRoot, f.Path, baselineSymbolsByFile[f.Path], bindingTargetsByFile[f.Path])
				item := DriftItem{
					Path:             f.Path,
					Status:           DriftMissing,
					Invariants:       copyDriftInvariants(decisionFields.Invariants),
					Materiality:      assessment.Materiality,
					TriggerKind:      DriftTriggerMissingFile,
					ChangedTargetRef: assessment.ChangedTargetRef,
					TargetKind:       assessment.TargetKind,
					TargetStatus:     assessment.TargetStatus,
					FallbackKind:     assessment.FallbackKind,
					FallbackReason:   assessment.FallbackReason,
					AuditOnly:        assessment.AuditOnly,
					SuppressedReason: assessment.SuppressedReason,
				}
				report.Files = append(report.Files, attachDriftClaimEvidenceRefs(item, decisionFields.Claims, evidenceItems))
				hasDrift = true
				continue
			}

			if currentHash != f.Hash {
				lines := gitDiffStat(projectRoot, f.Path)
				assessment := assessModifiedFileDrift(projectRoot, f.Path, baselineSymbolsByFile[f.Path], bindingTargetsByFile[f.Path])
				item := DriftItem{
					Path:             f.Path,
					Status:           DriftModified,
					LinesChanged:     lines,
					Invariants:       copyDriftInvariants(decisionFields.Invariants),
					Symbols:          assessment.Symbols,
					Materiality:      assessment.Materiality,
					TriggerKind:      DriftTriggerFileHash,
					ChangedTargetRef: assessment.ChangedTargetRef,
					TargetKind:       assessment.TargetKind,
					TargetStatus:     assessment.TargetStatus,
					FallbackKind:     assessment.FallbackKind,
					FallbackReason:   assessment.FallbackReason,
					AuditOnly:        assessment.AuditOnly,
					SuppressedReason: assessment.SuppressedReason,
				}
				report.Files = append(report.Files, attachDriftClaimEvidenceRefs(item, decisionFields.Claims, evidenceItems))
				hasDrift = true
			}
		}

		addedFiles, err := detectAddedFiles(projectRoot, files, decisionFields.DriftManifests, scopeCache)
		if err != nil {
			return nil, fmt.Errorf("detect added files for %s: %w", d.Meta.ID, err)
		}
		for _, path := range addedFiles {
			item := DriftItem{
				Path:        path,
				Status:      DriftAdded,
				Invariants:  copyDriftInvariants(decisionFields.Invariants),
				Materiality: DriftMaterialityAdjacentFileChurn,
				TriggerKind: DriftTriggerScopeManifest,
				AuditOnly:   true,
			}
			report.Files = append(report.Files, attachDriftClaimEvidenceRefs(item, decisionFields.Claims, evidenceItems))
			hasDrift = true
		}

		// Only include reports with drift or missing baselines
		if hasDrift || !hasAnyHash {
			reports = append(reports, report)
		}
	}

	logger.Debug().Int("drift_reports", len(reports)).Msg("drift.check.complete")

	return reports, nil
}

func attachDriftClaimEvidenceRefs(
	item DriftItem,
	claims []DecisionClaim,
	evidenceItems []EvidenceItem,
) DriftItem {
	item.ClaimRefs = driftClaimRefsForTarget(claims, item.ChangedTargetRef)
	item.EvidenceRefs = driftEvidenceRefsForClaims(evidenceItems, item.ClaimRefs)
	for index, symbol := range item.Symbols {
		symbolTarget := driftEventSymbolTarget(item.Path, symbol)
		item.Symbols[index].ClaimRefs = driftClaimRefsForTarget(claims, symbolTarget)
		item.Symbols[index].EvidenceRefs = driftEvidenceRefsForClaims(evidenceItems, item.Symbols[index].ClaimRefs)
	}
	return item
}

func driftClaimRefsForTarget(claims []DecisionClaim, targetRef string) []string {
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" {
		return nil
	}
	var refs []string
	for _, claim := range claims {
		lifecycle := EffectiveClaimLifecycleStatus(claim)
		if lifecycle == ClaimLifecycleSuperseded || lifecycle == ClaimLifecycleDeprecated {
			continue
		}
		if !driftClaimHasTargetRef(claim, targetRef) {
			continue
		}
		refs = append(refs, claim.ID)
	}
	sort.Strings(refs)
	return normalizeClaimRefs(refs)
}

func driftClaimHasTargetRef(claim DecisionClaim, targetRef string) bool {
	for _, ref := range claim.GovernanceTargetRefs {
		if strings.TrimSpace(ref) == targetRef {
			return true
		}
	}
	return false
}

func driftEvidenceRefsForClaims(evidenceItems []EvidenceItem, claimRefs []string) []string {
	claimSet := map[string]struct{}{}
	for _, ref := range normalizeClaimRefs(claimRefs) {
		claimSet[ref] = struct{}{}
	}
	if len(claimSet) == 0 {
		return nil
	}
	refs := make([]string, 0, len(evidenceItems))
	for _, item := range evidenceItems {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		if !driftEvidenceItemBindsAnyClaim(item, claimSet) {
			continue
		}
		refs = append(refs, item.ID)
	}
	sort.Strings(refs)
	return compactStrings(refs)
}

func driftEvidenceItemBindsAnyClaim(item EvidenceItem, claimSet map[string]struct{}) bool {
	for _, ref := range normalizeClaimRefs(item.ClaimRefs) {
		if _, ok := claimSet[ref]; ok {
			return true
		}
	}
	return false
}

// groupSymbolsByFile buckets a decision's stored symbol baseline by file path.
// It tolerates a load error (returns nil) so drift detection degrades to
// file-level rather than failing the session-mandatory status check.
func groupSymbolsByFile(symbols []AffectedSymbol, err error) map[string][]AffectedSymbol {
	if err != nil || len(symbols) == 0 {
		return nil
	}
	byFile := make(map[string][]AffectedSymbol)
	for _, s := range symbols {
		byFile[s.FilePath] = append(byFile[s.FilePath], s)
	}
	return byFile
}

func groupBindingTargetsByFile(targets []BindingTarget) map[string][]BindingTarget {
	if len(targets) == 0 {
		return nil
	}
	byFile := make(map[string][]BindingTarget)
	for _, target := range targets {
		if target.FilePath == "" {
			continue
		}
		byFile[target.FilePath] = append(byFile[target.FilePath], target)
	}
	return byFile
}

type modifiedFileAssessment struct {
	Symbols          []SymbolDriftItem
	Materiality      DriftMateriality
	ChangedTargetRef string
	TargetKind       string
	TargetStatus     string
	FallbackKind     string
	FallbackReason   string
	AuditOnly        bool
	SuppressedReason string
}

// assessModifiedFileDrift partitions a modified file's change at symbol
// granularity against its stored baseline. File-hash drift is only material
// when a baselined symbol was modified or removed; unchanged governed symbols
// turn the file change into audit-only adjacent churn.
func assessModifiedFileDrift(projectRoot, relPath string, baseline []AffectedSymbol, targetGroups ...[]BindingTarget) modifiedFileAssessment {
	targets := flattenBindingTargetGroups(targetGroups)
	if generatedOrIgnoredPath(relPath) {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityGeneratedOrIgnored,
			AuditOnly:        true,
			SuppressedReason: "generated or local runtime path changed; no code-object symbol drift",
		}
	}
	if carrierOnlyPath(relPath) {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityCarrierOnly,
			AuditOnly:        true,
			SuppressedReason: "carrier path changed; no code-object symbol drift",
		}
	}
	if targetAssessment, ok := assessBindingTargetDrift(projectRoot, relPath, targets); ok {
		return targetAssessment
	}
	if len(baseline) == 0 {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityUnknownLegacyFileScope,
			SuppressedReason: "no symbol baseline for changed file",
		}
	}
	current, err := codebase.ExtractSymbolSnapshots(projectRoot, relPath)
	if err != nil || len(current) == 0 {
		// Unsupported language, oversized file, or no parseable symbols: cannot
		// classify — fail-safe rather than mislabel every baseline symbol removed.
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityUnknownLegacyFileScope,
			SuppressedReason: "symbol extraction unavailable for changed file",
		}
	}
	baseSnaps := make([]codebase.SymbolSnapshot, 0, len(baseline))
	for _, s := range baseline {
		baseSnaps = append(baseSnaps, codebase.SymbolSnapshot{
			FilePath:   s.FilePath,
			SymbolName: s.SymbolName,
			SymbolKind: s.SymbolKind,
			Line:       s.Line,
			Hash:       s.Hash,
		})
	}
	drifts := codebase.CompareSymbolSnapshots(baseSnaps, current)
	if len(drifts) == 0 {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityAdjacentFileChurn,
			AuditOnly:        true,
			SuppressedReason: "baselined symbols unchanged; file hash drift is outside governed symbol bodies",
		}
	}
	items := make([]SymbolDriftItem, 0, len(drifts))
	material := false
	for _, d := range drifts {
		items = append(items, SymbolDriftItem{
			SymbolName: d.SymbolName,
			SymbolKind: d.SymbolKind,
			Status:     d.Status,
		})
		if d.Status != "added" {
			material = true
		}
	}
	if material {
		return modifiedFileAssessment{
			Symbols:     items,
			Materiality: DriftMaterialityMaterialSymbol,
		}
	}
	return modifiedFileAssessment{
		Symbols:          items,
		Materiality:      DriftMaterialityAdjacentFileChurn,
		AuditOnly:        true,
		SuppressedReason: "only new symbols were detected; existing governed symbols are unchanged",
	}
}

func flattenBindingTargetGroups(groups [][]BindingTarget) []BindingTarget {
	for _, group := range groups {
		if len(group) > 0 {
			return group
		}
	}
	return nil
}

func assessMissingFileDrift(projectRoot, relPath string, baseline []AffectedSymbol, targets []BindingTarget) modifiedFileAssessment {
	if moved, ok := findMovedSymbolTarget(projectRoot, relPath, targets); ok {
		return modifiedFileAssessment{
			Materiality: DriftMaterialityMaterialSymbol,
			ChangedTargetRef: driftEventSymbolTarget(moved.FilePath, SymbolDriftItem{
				SymbolName: moved.SymbolName,
				SymbolKind: moved.SymbolKind,
				Status:     "renamed",
			}),
			TargetKind:       BindingTargetSymbol,
			TargetStatus:     "renamed",
			SuppressedReason: fmt.Sprintf("governed symbol %s moved from %s to %s with identical body hash", moved.SymbolName, relPath, moved.FilePath),
		}
	}
	if candidate, ok := findEditedMovedSymbolTarget(projectRoot, relPath, targets); ok {
		return modifiedFileAssessment{
			Materiality: DriftMaterialityNeedsBindingResolution,
			ChangedTargetRef: driftEventSymbolTarget(candidate.FilePath, SymbolDriftItem{
				SymbolName: candidate.SymbolName,
				SymbolKind: candidate.SymbolKind,
				Status:     "retarget_candidate",
			}),
			TargetKind:       BindingTargetSymbol,
			TargetStatus:     "retarget_candidate",
			FallbackKind:     "edited_symbol_move_candidate",
			FallbackReason:   fmt.Sprintf("governed symbol %s disappeared from %s and a same-name symbol exists in %s with changed body hash; retarget requires operator review", candidate.SymbolName, relPath, candidate.FilePath),
			SuppressedReason: fmt.Sprintf("governed symbol %s may have moved from %s to %s with edits; retarget requires operator review", candidate.SymbolName, relPath, candidate.FilePath),
		}
	}
	if candidate, ok := findFuzzyEditedMovedSymbolTarget(projectRoot, relPath, targets); ok {
		return modifiedFileAssessment{
			Materiality: DriftMaterialityNeedsBindingResolution,
			ChangedTargetRef: driftEventSymbolTarget(candidate.FilePath, SymbolDriftItem{
				SymbolName: candidate.SymbolName,
				SymbolKind: candidate.SymbolKind,
				Status:     "retarget_candidate",
			}),
			TargetKind:       BindingTargetSymbol,
			TargetStatus:     "retarget_candidate",
			FallbackKind:     "fuzzy_edited_symbol_move_candidate",
			FallbackReason:   fmt.Sprintf("governed symbol %s disappeared from %s and exactly one same-name candidate exists in %s, but kind/receiver identity changed; retarget requires operator review", candidate.SymbolName, relPath, candidate.FilePath),
			SuppressedReason: fmt.Sprintf("governed symbol %s may have moved from %s to %s with changed kind/receiver identity; retarget requires operator review", candidate.SymbolName, relPath, candidate.FilePath),
		}
	}
	return modifiedFileAssessment{
		Materiality: missingFileMateriality(baseline, targets),
	}
}

func findMovedSymbolTarget(projectRoot, relPath string, targets []BindingTarget) (codebase.SymbolSnapshot, bool) {
	for _, target := range targets {
		if target.Kind != BindingTargetSymbol {
			continue
		}
		if strings.TrimSpace(target.BodyHash) == "" {
			continue
		}
		moved, ok := findMovedSymbolSnapshot(projectRoot, relPath, target)
		if ok {
			return moved, true
		}
	}
	return codebase.SymbolSnapshot{}, false
}

func findEditedMovedSymbolTarget(projectRoot, relPath string, targets []BindingTarget) (codebase.SymbolSnapshot, bool) {
	for _, target := range targets {
		if target.Kind != BindingTargetSymbol {
			continue
		}
		if strings.TrimSpace(target.SymbolName) == "" {
			continue
		}
		moved, ok := findEditedMovedSymbolSnapshot(projectRoot, relPath, target)
		if ok {
			return moved, true
		}
	}
	return codebase.SymbolSnapshot{}, false
}

func findFuzzyEditedMovedSymbolTarget(projectRoot, relPath string, targets []BindingTarget) (codebase.SymbolSnapshot, bool) {
	for _, target := range targets {
		if target.Kind != BindingTargetSymbol {
			continue
		}
		if strings.TrimSpace(target.SymbolName) == "" {
			continue
		}
		moved, ok := findFuzzyEditedMovedSymbolSnapshot(projectRoot, relPath, target)
		if ok {
			return moved, true
		}
	}
	return codebase.SymbolSnapshot{}, false
}

func findMovedSymbolSnapshot(projectRoot, oldRelPath string, target BindingTarget) (codebase.SymbolSnapshot, bool) {
	files, err := listScopeFiles(projectRoot, ".")
	if err != nil {
		return codebase.SymbolSnapshot{}, false
	}
	oldRelPath = normalizeProjectPath(oldRelPath)
	for _, path := range files {
		normalizedPath := normalizeProjectPath(path)
		if normalizedPath == oldRelPath {
			continue
		}
		if generatedOrIgnoredPath(normalizedPath) || carrierOnlyPath(normalizedPath) {
			continue
		}
		snapshots, err := codebase.ExtractSymbolSnapshots(projectRoot, normalizedPath)
		if err != nil {
			continue
		}
		for _, snapshot := range snapshots {
			if !symbolSnapshotMatchesBindingTarget(snapshot, target) {
				continue
			}
			return snapshot, true
		}
	}
	return codebase.SymbolSnapshot{}, false
}

func findEditedMovedSymbolSnapshot(projectRoot, oldRelPath string, target BindingTarget) (codebase.SymbolSnapshot, bool) {
	return findEditedMovedSymbolSnapshotBy(projectRoot, oldRelPath, target, symbolSnapshotSameIdentity)
}

func findFuzzyEditedMovedSymbolSnapshot(projectRoot, oldRelPath string, target BindingTarget) (codebase.SymbolSnapshot, bool) {
	return findEditedMovedSymbolSnapshotBy(projectRoot, oldRelPath, target, symbolSnapshotSameNameOnly)
}

func findEditedMovedSymbolSnapshotBy(
	projectRoot string,
	oldRelPath string,
	target BindingTarget,
	match func(codebase.SymbolSnapshot, BindingTarget) bool,
) (codebase.SymbolSnapshot, bool) {
	files, err := listScopeFiles(projectRoot, ".")
	if err != nil {
		return codebase.SymbolSnapshot{}, false
	}
	oldRelPath = normalizeProjectPath(oldRelPath)
	var candidates []codebase.SymbolSnapshot
	for _, path := range files {
		normalizedPath := normalizeProjectPath(path)
		if normalizedPath == oldRelPath {
			continue
		}
		if generatedOrIgnoredPath(normalizedPath) || carrierOnlyPath(normalizedPath) {
			continue
		}
		snapshots, err := codebase.ExtractSymbolSnapshots(projectRoot, normalizedPath)
		if err != nil {
			continue
		}
		for _, snapshot := range snapshots {
			if !match(snapshot, target) {
				continue
			}
			if strings.TrimSpace(snapshot.Hash) == strings.TrimSpace(target.BodyHash) {
				continue
			}
			candidates = append(candidates, snapshot)
		}
	}
	if len(candidates) != 1 {
		return codebase.SymbolSnapshot{}, false
	}
	return candidates[0], true
}

func symbolSnapshotMatchesBindingTarget(snapshot codebase.SymbolSnapshot, target BindingTarget) bool {
	if !symbolSnapshotSameIdentity(snapshot, target) {
		return false
	}
	return strings.TrimSpace(snapshot.Hash) == strings.TrimSpace(target.BodyHash)
}

func symbolSnapshotSameIdentity(snapshot codebase.SymbolSnapshot, target BindingTarget) bool {
	if snapshot.SymbolName != target.SymbolName {
		return false
	}
	if target.SymbolKind != "" && snapshot.SymbolKind != target.SymbolKind {
		return false
	}
	if strings.TrimSpace(snapshot.Receiver) != strings.TrimSpace(target.Receiver) {
		return false
	}
	return true
}

func symbolSnapshotSameNameOnly(snapshot codebase.SymbolSnapshot, target BindingTarget) bool {
	return strings.TrimSpace(snapshot.SymbolName) == strings.TrimSpace(target.SymbolName)
}

func assessBindingTargetDrift(projectRoot, relPath string, targets []BindingTarget) (modifiedFileAssessment, bool) {
	if len(targets) == 0 {
		return modifiedFileAssessment{}, false
	}
	hasPreciseTarget := false
	hasInspectablePreciseTarget := false
	for _, target := range targets {
		switch target.Kind {
		case BindingTargetWholeFileFallback:
			reason := bindingFallbackReason(target)
			return modifiedFileAssessment{
				Materiality:      DriftMaterialityNeedsBindingResolution,
				FallbackKind:     BindingTargetWholeFileFallback,
				FallbackReason:   reason,
				SuppressedReason: "whole-file fallback binding changed; resolve a symbol/range/module target before treating this as material drift",
			}, true
		case BindingTargetGenerated:
			return modifiedFileAssessment{
				Materiality:      DriftMaterialityGeneratedOrIgnored,
				AuditOnly:        true,
				SuppressedReason: "generated binding target changed; no code-object symbol drift",
			}, true
		case BindingTargetCarrier:
			return modifiedFileAssessment{
				Materiality:      DriftMaterialityCarrierOnly,
				AuditOnly:        true,
				SuppressedReason: "carrier binding target changed; no code-object symbol drift",
			}, true
		case BindingTargetRange:
			hasPreciseTarget = true
			hasInspectablePreciseTarget = true
			if !rangeTargetUnchanged(projectRoot, relPath, target) {
				return modifiedFileAssessment{
					Materiality: DriftMaterialityMaterialSymbol,
				}, true
			}
		case BindingTargetSymbol:
			hasPreciseTarget = true
			hasInspectablePreciseTarget = true
			assessment, ok := assessSymbolBindingTargetDrift(projectRoot, relPath, target)
			if ok {
				return assessment, true
			}
		case BindingTargetModule:
			hasPreciseTarget = true
		case BindingTargetSpecSection, BindingTargetAPIContract, BindingTargetInvariant:
			hasPreciseTarget = true
			hasInspectablePreciseTarget = true
			return assessSemanticBindingTargetDrift(projectRoot, relPath, target), true
		}
	}
	if hasInspectablePreciseTarget {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityAdjacentFileChurn,
			AuditOnly:        true,
			SuppressedReason: "binding target symbols/ranges unchanged; file hash drift is outside governed code objects",
			Symbols:          addedSymbolsOutsideBindingTargets(projectRoot, relPath, targets),
		}, true
	}
	if hasPreciseTarget {
		return modifiedFileAssessment{}, false
	}
	return modifiedFileAssessment{
		Materiality:      DriftMaterialityNeedsBindingResolution,
		FallbackKind:     "imprecise_binding_target",
		FallbackReason:   "binding target posture is not precise enough to classify drift",
		SuppressedReason: "binding target posture is not precise enough to classify drift",
	}, true
}

func assessSemanticBindingTargetDrift(projectRoot, relPath string, target BindingTarget) modifiedFileAssessment {
	targetRef := strings.TrimSpace(target.TargetRef)
	if targetRef == "" {
		targetRef = strings.TrimSpace(target.Kind)
	}
	if strings.TrimSpace(target.TextHash) == "" {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityNeedsBindingResolution,
			FallbackKind:     "semantic_target_missing_evaluator",
			FallbackReason:   "semantic target " + targetRef + " has no text_hash/carrier evaluator",
			SuppressedReason: "semantic target has no text_hash/carrier evaluator; add a concrete binding target before classifying drift",
		}
	}
	if rangeTargetUnchanged(projectRoot, relPath, target) {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityAdjacentFileChurn,
			AuditOnly:        true,
			SuppressedReason: "semantic target " + targetRef + " is unchanged; file hash drift is outside the governed target",
		}
	}
	changedRef := semanticBindingTargetRef(target)
	if semanticBindingTargetDeleted(projectRoot, relPath, target) {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityMaterialSemanticTarget,
			ChangedTargetRef: changedRef,
			TargetKind:       strings.TrimSpace(target.Kind),
			TargetStatus:     "removed",
			SuppressedReason: "semantic target " + targetRef + " was removed",
		}
	}
	return modifiedFileAssessment{
		Materiality:      DriftMaterialityMaterialSemanticTarget,
		ChangedTargetRef: changedRef,
		TargetKind:       strings.TrimSpace(target.Kind),
		TargetStatus:     "modified",
		SuppressedReason: "semantic target " + targetRef + " changed",
	}
}

func semanticBindingTargetDeleted(projectRoot, relPath string, target BindingTarget) bool {
	if target.ResolutionSource != BindingResolutionSourceMarkdownSection {
		return false
	}
	_, ok := extractMarkdownTargetRange(projectRoot, relPath, target.TargetRef)
	return !ok
}

func semanticBindingTargetRef(target BindingTarget) string {
	targetRef := strings.TrimSpace(target.TargetRef)
	if targetRef == "" {
		return strings.TrimSpace(target.Kind)
	}
	if strings.Contains(targetRef, ":") {
		return targetRef
	}
	kind := strings.TrimSpace(target.Kind)
	if kind == "" {
		return targetRef
	}
	return kind + ":" + targetRef
}

func bindingFallbackReason(target BindingTarget) string {
	for _, candidate := range []string{
		target.Reason,
		target.WhySymbolFailed,
		target.WhyRangeFailed,
		target.LanguageSupport,
		target.ResolutionSource,
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return "whole-file fallback binding"
}

func addedSymbolsOutsideBindingTargets(projectRoot, relPath string, targets []BindingTarget) []SymbolDriftItem {
	current, err := codebase.ExtractSymbolSnapshots(projectRoot, relPath)
	if err != nil || len(current) == 0 {
		return nil
	}

	targetKeys := make(map[string]struct{})
	for _, target := range targets {
		if target.Kind != BindingTargetSymbol {
			continue
		}
		targetKeys[symbolBindingTargetKey(target.SymbolKind, target.SymbolName, target.Receiver)] = struct{}{}
	}
	if len(targetKeys) == 0 {
		return nil
	}

	items := make([]SymbolDriftItem, 0)
	for _, snapshot := range current {
		key := symbolBindingTargetKey(snapshot.SymbolKind, snapshot.SymbolName, snapshot.Receiver)
		if _, ok := targetKeys[key]; ok {
			continue
		}
		items = append(items, SymbolDriftItem{
			SymbolName: snapshot.SymbolName,
			SymbolKind: snapshot.SymbolKind,
			Status:     "added",
		})
	}
	return items
}

func symbolBindingTargetKey(kind, name, receiver string) string {
	return strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(name) + "\x00" + strings.TrimSpace(receiver)
}

func assessSymbolBindingTargetDrift(projectRoot, relPath string, target BindingTarget) (modifiedFileAssessment, bool) {
	if strings.TrimSpace(target.BodyHash) == "" {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityNeedsBindingResolution,
			SuppressedReason: "symbol binding target has no body_hash; rebaseline the binding target before classifying drift",
		}, true
	}

	current, err := codebase.ExtractSymbolSnapshots(projectRoot, relPath)
	if err != nil || len(current) == 0 {
		return modifiedFileAssessment{
			Materiality:      DriftMaterialityUnknownLegacyFileScope,
			SuppressedReason: "symbol extraction unavailable for binding target",
		}, true
	}

	for _, snapshot := range current {
		if snapshot.SymbolName != target.SymbolName {
			continue
		}
		if target.SymbolKind != "" && snapshot.SymbolKind != target.SymbolKind {
			continue
		}
		if strings.TrimSpace(target.Receiver) != strings.TrimSpace(snapshot.Receiver) {
			continue
		}
		if snapshot.Hash == target.BodyHash {
			return modifiedFileAssessment{}, false
		}
		return modifiedFileAssessment{
			Materiality: DriftMaterialityMaterialSymbol,
			Symbols: []SymbolDriftItem{
				{
					SymbolName: target.SymbolName,
					SymbolKind: target.SymbolKind,
					Status:     "modified",
				},
			},
		}, true
	}

	return modifiedFileAssessment{
		Materiality: DriftMaterialityMaterialSymbol,
		Symbols: []SymbolDriftItem{
			{
				SymbolName: target.SymbolName,
				SymbolKind: target.SymbolKind,
				Status:     "removed",
			},
		},
	}, true
}

func rangeTargetUnchanged(projectRoot, relPath string, target BindingTarget) bool {
	if target.Line > 0 && target.EndLine >= target.Line {
		current, ok := bindingTargetRangeTextHash(projectRoot, relPath, target.Line, target.EndLine)
		return ok && current == target.TextHash
	}
	current, err := codebase.ExtractStableFileRange(projectRoot, relPath)
	if err != nil {
		return false
	}
	return current.TextHash == target.TextHash
}

func bindingTargetRangeTextHash(projectRoot, relPath string, startLine, endLine int) (string, bool) {
	content, err := os.ReadFile(filepath.Join(projectRoot, relPath))
	if err != nil {
		return "", false
	}
	lines := splitBindingTextLines(string(content))
	if startLine <= 0 || endLine < startLine || startLine > len(lines) {
		return "", false
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	normalized := normalizeBindingRangeText(strings.Join(lines[startLine-1:endLine], "\n"))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:]), true
}

func noBaselineMateriality(targets []BindingTarget) DriftMateriality {
	if bindingTargetsNeedResolution(targets) {
		return DriftMaterialityNeedsBindingResolution
	}
	return DriftMaterialityUnknownLegacyFileScope
}

func missingFileMateriality(baseline []AffectedSymbol, targets []BindingTarget) DriftMateriality {
	if bindingTargetsNeedResolution(targets) {
		return DriftMaterialityNeedsBindingResolution
	}
	if len(baseline) == 0 {
		return DriftMaterialityUnknownLegacyFileScope
	}
	return DriftMaterialityMaterialSymbol
}

func bindingTargetsNeedResolution(targets []BindingTarget) bool {
	for _, target := range targets {
		if target.Kind == BindingTargetWholeFileFallback {
			return true
		}
	}
	return false
}

func carrierOnlyPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	if clean == "CHANGELOG.md" {
		return true
	}
	if strings.HasPrefix(clean, ".context/") {
		return true
	}
	if strings.HasPrefix(clean, ".haft/specs/") || strings.HasPrefix(clean, ".haft/decisions/") {
		return true
	}
	if strings.HasPrefix(clean, "open-sleigh/.haft/") {
		return true
	}
	if strings.HasPrefix(clean, "docs/") {
		return true
	}
	if strings.HasSuffix(clean, "/SKILL.md") || clean == "AGENTS.md" || clean == "CLAUDE.md" {
		return true
	}
	return false
}

func generatedOrIgnoredPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	if clean == "data/FPF" || strings.HasSuffix(clean, ".db") {
		return true
	}
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		switch part {
		case "node_modules", "target", "_build", "deps", "vendor":
			return true
		}
	}
	return false
}

func persistDriftManifests(
	ctx context.Context,
	store ArtifactStore,
	artifact *Artifact,
	manifests []DriftScopeManifest,
) error {
	if artifact.Meta.Kind != KindDecisionRecord {
		return nil
	}

	fields := artifact.UnmarshalDecisionFields()
	fields.DriftManifests = manifests

	return persistDecisionFields(ctx, store, artifact, fields)
}

func persistDecisionFields(
	ctx context.Context,
	store ArtifactStore,
	artifact *Artifact,
	fields DecisionFields,
) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal decision fields: %w", err)
	}

	updated := *artifact
	updated.StructuredData = string(data)

	return store.Update(ctx, &updated)
}

func buildDriftScopeManifests(projectRoot string, files []AffectedFile) ([]DriftScopeManifest, error) {
	scopeSet := make(map[string]struct{})
	scopes := make([]string, 0, len(files))

	for _, file := range files {
		scope := normalizeDriftScope(filepath.Dir(file.Path))
		if _, ok := scopeSet[scope]; ok {
			continue
		}
		scopeSet[scope] = struct{}{}
		scopes = append(scopes, scope)
	}

	sort.Strings(scopes)

	manifests := make([]DriftScopeManifest, 0, len(scopes))
	for _, scope := range scopes {
		scopeFiles, err := listScopeFiles(projectRoot, scope)
		if err != nil {
			return nil, fmt.Errorf("list scope %s: %w", scope, err)
		}

		manifests = append(manifests, DriftScopeManifest{
			Scope: scope,
			Files: scopeFiles,
		})
	}

	return manifests, nil
}

func detectAddedFiles(
	projectRoot string,
	files []AffectedFile,
	manifests []DriftScopeManifest,
	scopeCache map[string][]string,
) ([]string, error) {
	if len(manifests) == 0 {
		return nil, nil
	}

	baselinedFiles := make(map[string]struct{})
	governedFiles := make(map[string]struct{})
	addedFiles := make([]string, 0)

	for _, file := range files {
		governedFiles[normalizeProjectPath(file.Path)] = struct{}{}
	}
	for _, manifest := range manifests {
		for _, path := range manifest.Files {
			baselinedFiles[normalizeProjectPath(path)] = struct{}{}
		}

		scopeFiles, err := listScopeFilesCached(projectRoot, manifest.Scope, scopeCache)
		if err != nil {
			return nil, fmt.Errorf("list scope %s: %w", manifest.Scope, err)
		}

		for _, path := range scopeFiles {
			normalizedPath := normalizeProjectPath(path)
			if _, ok := baselinedFiles[normalizedPath]; ok {
				continue
			}
			if _, ok := governedFiles[normalizedPath]; ok {
				continue
			}
			governedFiles[normalizedPath] = struct{}{}
			addedFiles = append(addedFiles, normalizedPath)
		}
	}

	sort.Strings(addedFiles)

	return addedFiles, nil
}

// listScopeFilesCached memoizes listScopeFiles by normalized scope within one
// drift pass, so a scope walked for many decisions (especially the whole-repo
// ".") is walked only once. nil cache falls through to a direct walk.
func listScopeFilesCached(projectRoot, scope string, cache map[string][]string) ([]string, error) {
	scope = normalizeDriftScope(scope)
	if cache != nil {
		if v, ok := cache[scope]; ok {
			return v, nil
		}
	}
	files, err := listScopeFiles(projectRoot, scope)
	if err != nil {
		return nil, err
	}
	if cache != nil {
		cache[scope] = files
	}
	return files, nil
}

func listScopeFiles(projectRoot string, scope string) ([]string, error) {
	scope = normalizeDriftScope(scope)
	if files, ok := listGitScopeFiles(projectRoot, scope); ok {
		return files, nil
	}

	scopePath := filepath.Join(projectRoot, scope)
	entries := make([]string, 0)
	ignoreChecker := codebase.NewIgnoreChecker(projectRoot)

	err := filepath.WalkDir(scopePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}

		relPath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		normalizedPath := normalizeProjectPath(relPath)

		if driftPathNeverScanned(normalizedPath) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if codebase.IsExcludedDir(entry.Name()) {
				return filepath.SkipDir
			}
			if ignoreChecker.IsIgnored(normalizedPath) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoreChecker.IsIgnored(normalizedPath) {
			return nil
		}

		entries = append(entries, normalizedPath)
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	sort.Strings(entries)

	return entries, nil
}

func listGitScopeFiles(projectRoot string, scope string) ([]string, bool) {
	if strings.TrimSpace(projectRoot) == "" {
		return nil, false
	}

	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "--", scope)
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, false
	}

	seen := map[string]struct{}{}
	files := make([]string, 0)
	for _, line := range strings.Split(string(output), "\n") {
		path := normalizeProjectPath(line)
		if path == "." || path == "" {
			continue
		}
		if driftPathNeverScanned(path) {
			continue
		}
		if !driftPathExistsAsFile(projectRoot, path) {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		files = append(files, path)
	}

	sort.Strings(files)
	return files, true
}

func driftPathExistsAsFile(projectRoot string, relPath string) bool {
	info, err := os.Stat(filepath.Join(projectRoot, relPath))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func driftPathNeverScanned(relPath string) bool {
	normalized := normalizeProjectPath(relPath)
	if normalized == "." || normalized == "" {
		return false
	}
	for _, part := range strings.Split(normalized, "/") {
		if codebase.IsExcludedDir(part) {
			return true
		}
	}
	return false
}

func normalizeDriftScope(scope string) string {
	scope = filepath.ToSlash(filepath.Clean(strings.TrimSpace(scope)))
	if scope == "" || scope == "/" {
		return "."
	}
	return scope
}

func normalizeProjectPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func copyDriftInvariants(invariants []string) []string {
	if len(invariants) == 0 {
		return nil
	}

	return append([]string(nil), invariants...)
}

// hashFile computes SHA-256 of a file's contents. Skips directories.
func hashFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("skip directory: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// gitFileModifiedSince checks if a file has git commits after the given time.
// Returns false if git is unavailable or fails.
func gitFileModifiedSince(projectRoot, filePath string, since time.Time) bool {
	sinceStr := since.Format("2006-01-02T15:04:05")
	cmd := exec.Command("git", "log", "--oneline", "--after="+sinceStr, "--", filePath)
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// gitDiffStat returns a short diff stat for a file (e.g., "+8 -2").
// Returns empty string if git is not available or fails.
func gitDiffStat(projectRoot, filePath string) string {
	cmd := exec.Command("git", "diff", "--numstat", "HEAD", "--", filePath)
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) >= 2 {
		return fmt.Sprintf("+%s -%s", parts[0], parts[1])
	}
	return ""
}

// Apply is deprecated — the decide response now includes the full DRR body.
// Kept for backward compatibility: returns the DRR body directly.
func Apply(ctx context.Context, store ArtifactStore, decisionRef string) (string, error) {
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
	DecisionRef    string   `json:"decision_ref"`
	Findings       string   `json:"findings"`
	CriteriaMet    []string `json:"criteria_met,omitempty"`
	CriteriaNotMet []string `json:"criteria_not_met,omitempty"`
	Measurements   []string `json:"measurements,omitempty"`
	Verdict        string   `json:"verdict"` // accepted, partial, failed
}

// EvidenceInput attaches evidence to any artifact.
// CongruenceLevel and FormalityLevel use -1 as "not provided" sentinel.
// JSON decodes missing fields as 0, which is a valid CL value (opposed context).
// Callers from MCP should set these to -1 when the user doesn't provide them.
type EvidenceInput struct {
	ArtifactRef        string   `json:"artifact_ref"`
	Content            string   `json:"content"`
	Type               string   `json:"type"`    // measurement, test, research, benchmark, audit
	Verdict            string   `json:"verdict"` // supports, weakens, refutes
	CarrierRef         string   `json:"carrier_ref,omitempty"`
	CongruenceLevel    int      `json:"congruence_level"` // 0-3; -1 = not provided (defaults to 3)
	FormalityLevel     int      `json:"formality_level"`  // F0-F9; -1 = not provided (defaults by evidence type)
	FormalityScaleID   string   `json:"formality_scale_id,omitempty"`
	ClaimRefs          []string `json:"claim_refs,omitempty"`
	ClaimScope         []string `json:"claim_scope,omitempty"`
	ValidUntil         string   `json:"valid_until,omitempty"`
	CausalSupportBasis string   `json:"causal_support_basis,omitempty"` // C.28; accepts canonical or alias form
	Provenance         string   `json:"provenance,omitempty"`           // "", "machine", "llm-review"
}

// Measure records post-implementation impact against the DRR's acceptance criteria.
func Measure(ctx context.Context, store ArtifactStore, haftDir string, input MeasureInput) (*Artifact, error) {
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

	// Inductive verification gate: check if baseline exists for decisions with affected_files
	var measureWarnings []string
	hasBaseline := false
	files, _ := store.GetAffectedFiles(ctx, input.DecisionRef)
	if len(files) > 0 {
		for _, f := range files {
			if f.Hash != "" {
				hasBaseline = true
				break
			}
		}
		if !hasBaseline {
			measureWarnings = append(measureWarnings,
				"⚠ No baseline found for this decision's affected files. "+
					"Implementation may not be verified. Measurement recorded at CL1 (self-evidence). "+
					"Run `haft_decision(action=\"baseline\")` first for CL3 scoring.")
		}
	} else {
		// No affected_files — can't verify via baseline, treat as unverified
		hasBaseline = false
	}
	measureCL := measurementCongruenceLevel(hasBaseline)

	scopeCandidates := measurementScopeCandidates(ctx, store, a)
	criteriaMetScope := measuredCriteriaScope(input.CriteriaMet, nil, scopeCandidates)
	criteriaNotMetScope := measuredCriteriaScope(nil, input.CriteriaNotMet, scopeCandidates)

	decisionFields := a.UnmarshalDecisionFields()
	claimRefs := measuredDecisionClaimRefs(
		decisionFields.Claims,
		input.CriteriaMet,
		criteriaMetScope,
		input.CriteriaNotMet,
		criteriaNotMetScope,
	)
	claimScope := decisionMeasurementCoverageScope(
		decisionFields.Claims,
		claimRefs,
		criteriaMetScope,
		criteriaNotMetScope,
	)
	evidenceItem := &EvidenceItem{
		ID:         fmt.Sprintf("evid-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000),
		Type:       "measurement",
		Content:    fmt.Sprintf("Impact measurement: %s\n%s", input.Verdict, input.Findings),
		Verdict:    input.Verdict,
		ClaimRefs:  claimRefs,
		ClaimScope: claimScope,
		ValidUntil: a.Meta.ValidUntil,
	}
	claimEvidence := measurementClaimEvidence(
		decisionFields.Claims,
		input.CriteriaMet,
		criteriaMetScope,
		input.CriteriaNotMet,
		criteriaNotMetScope,
	)
	measurementItems := decisionMeasurementEvidenceItems(
		decisionFields.Claims,
		*evidenceItem,
		claimEvidence,
	)
	applyMeasurementEvidencePosture(evidenceItem, measureCL)
	for index := range measurementItems {
		applyMeasurementEvidencePosture(&measurementItems[index], measureCL)
	}

	activeEvidence, err := decisionActiveClaimEvidenceAfterMeasurement(
		ctx,
		store,
		a.Meta.ID,
		decisionFields.Claims,
		measurementItems,
	)
	if err != nil {
		return nil, fmt.Errorf("load claim evidence: %w", err)
	}

	decisionFields.Claims = rebuildDecisionClaimsFromEvidence(
		decisionFields.Claims,
		activeEvidence,
	)
	decisionFields.Predictions = decisionPredictionsFromClaims(decisionFields.Claims)

	sd, err := json.Marshal(decisionFields)
	if err != nil {
		return nil, fmt.Errorf("marshal decision fields: %w", err)
	}
	a.StructuredData = string(sd)

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

	if err := store.CommitMeasurement(ctx, a, measurementItems); err != nil {
		return nil, fmt.Errorf("record measurement: %w", err)
	}

	writeFileQuiet(haftDir, a)

	if len(measureWarnings) > 0 {
		return a, &WriteWarning{Warnings: measureWarnings}
	}
	return a, nil
}

func measurementCongruenceLevel(hasBaseline bool) int {
	if hasBaseline {
		return 3
	}
	return 1
}

func applyMeasurementEvidencePosture(item *EvidenceItem, congruenceLevel int) {
	if item == nil {
		return
	}
	formalityScale := reff.CurrentFormalityScale(defaultEvidenceFormalityLevel("measurement"))
	item.CongruenceLevel = congruenceLevel
	item.FormalityLevel = formalityScale.Level
	item.FormalityScale = &formalityScale
	item.FormalityBridge = evidenceFormalityBridge(formalityScale)
}

func decisionActiveClaimEvidenceAfterMeasurement(
	ctx context.Context,
	store ArtifactStore,
	decisionID string,
	decisionClaims []DecisionClaim,
	incoming []EvidenceItem,
) ([]EvidenceItem, error) {
	items, err := store.GetEvidenceItems(ctx, decisionID)
	if err != nil {
		return nil, err
	}

	active := make([]EvidenceItem, 0, len(items)+len(incoming))
	incomingKeys := make([][]string, 0, len(incoming))

	for _, item := range incoming {
		incomingKeys = append(incomingKeys, measurementBindingKeys(decisionClaims, item))
	}

	for _, item := range items {
		if item.Verdict == "superseded" {
			continue
		}
		if item.Type != "measurement" {
			active = append(active, item)
			continue
		}

		existingKeys := measurementBindingKeys(decisionClaims, item)
		overlapsIncoming := false

		for _, keys := range incomingKeys {
			if !measurementKeysOverlap(existingKeys, keys) {
				continue
			}

			overlapsIncoming = true
			break
		}
		if overlapsIncoming {
			continue
		}

		active = append(active, item)
	}

	active = append(active, incoming...)

	return active, nil
}

func decisionMeasurementEvidenceItems(
	claims []DecisionClaim,
	base EvidenceItem,
	claimEvidence []EvidenceItem,
) []EvidenceItem {
	if len(claimEvidence) == 0 {
		return []EvidenceItem{base}
	}

	normalizedClaims := normalizeDecisionClaims(claims)
	aliasIndex := buildDecisionClaimAliasIndex(normalizedClaims)
	scopeByPrediction := make(map[int][]string, len(normalizedClaims))
	sharedScope := make([]string, 0, len(base.ClaimScope))
	claimIndex := make(map[string]int, len(normalizedClaims))
	items := make([]EvidenceItem, 0, len(claimEvidence))

	for index, claim := range normalizedClaims {
		claimIndex[claim.ID] = index
	}

	for _, scope := range normalizeClaimScope(base.ClaimScope) {
		predictionIndex, ok := resolvePredictionAlias(scope, aliasIndex)
		if ok {
			scopeByPrediction[predictionIndex] = append(scopeByPrediction[predictionIndex], scope)
			continue
		}

		sharedScope = append(sharedScope, scope)
	}

	for index, claimItem := range claimEvidence {
		item := base
		item.ID = fmt.Sprintf("%s-claim-%03d", base.ID, index+1)
		item.Verdict = claimItem.Verdict
		item.ClaimRefs = normalizeClaimRefs(claimItem.ClaimRefs)
		item.ClaimScope = decisionMeasurementClaimScope(
			normalizedClaims,
			item.ClaimRefs,
			claimIndex,
			scopeByPrediction,
			sharedScope,
		)
		items = append(items, item)
	}

	return items
}

func decisionMeasurementClaimScope(
	claims []DecisionClaim,
	claimRefs []string,
	claimIndex map[string]int,
	scopeByPrediction map[int][]string,
	sharedScope []string,
) []string {
	scope := make([]string, 0, len(sharedScope)+len(claimRefs))
	scope = append(scope, sharedScope...)

	for _, ref := range normalizeClaimRefs(claimRefs) {
		index, ok := claimIndex[ref]
		if !ok {
			continue
		}

		scope = append(scope, scopeByPrediction[index]...)
	}

	scope = normalizeClaimScope(scope)
	if len(scope) > 0 {
		return scope
	}

	return decisionClaimScopeFromRefs(claims, claimRefs)
}

// AttachEvidence adds an evidence item to any artifact.
func AttachEvidence(ctx context.Context, store ArtifactStore, input EvidenceInput) (*EvidenceItem, error) {
	if input.ArtifactRef == "" {
		return nil, fmt.Errorf("artifact_ref is required")
	}
	if input.Content == "" {
		return nil, fmt.Errorf("content is required — what's the evidence?")
	}

	// Verify artifact exists
	artifactItem, err := store.Get(ctx, input.ArtifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", input.ArtifactRef, err)
	}
	input.ClaimRefs = normalizeClaimRefs(input.ClaimRefs)
	input.ClaimScope = normalizeClaimScope(input.ClaimScope)

	if artifactItem.Meta.Kind == KindDecisionRecord {
		decisionClaims := artifactItem.UnmarshalDecisionFields().Claims
		claimRefs, claimScope, err := normalizeDecisionEvidenceBinding(
			decisionClaims,
			input.ClaimRefs,
			input.ClaimScope,
		)
		if err != nil {
			return nil, err
		}

		input.ClaimRefs = claimRefs
		input.ClaimScope = claimScope
	}
	if artifactItem.Meta.Kind != KindDecisionRecord && len(input.ClaimRefs) > 0 {
		return nil, fmt.Errorf("claim_refs require a decision artifact with structured claims")
	}

	if input.Type == "" {
		input.Type = "general"
	}
	if input.CongruenceLevel < 0 {
		input.CongruenceLevel = 3
	}
	if input.FormalityLevel < 0 {
		input.FormalityLevel = defaultEvidenceFormalityLevel(input.Type)
	}
	formalityScale := evidenceInputFormalityScale(input.FormalityScaleID, input.FormalityLevel)
	formalityLevel := formalityScale.Level
	formalityBridge := evidenceFormalityBridge(formalityScale)
	storedVerdict := canonicalStoredEvidenceVerdict(input.Type, input.Verdict)
	err = validateEvidenceCongruenceAtIngest(storedVerdict, input.CongruenceLevel)
	if err != nil {
		return nil, err
	}

	causalBasis, err := ParseCausalSupportBasis(input.CausalSupportBasis)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("evid-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)

	item := &EvidenceItem{
		ID:                 id,
		Type:               input.Type,
		Content:            input.Content,
		Verdict:            storedVerdict,
		CarrierRef:         input.CarrierRef,
		CongruenceLevel:    input.CongruenceLevel,
		FormalityLevel:     formalityLevel,
		FormalityScale:     &formalityScale,
		FormalityBridge:    formalityBridge,
		ClaimRefs:          input.ClaimRefs,
		ClaimScope:         input.ClaimScope,
		ValidUntil:         input.ValidUntil,
		CausalSupportBasis: causalBasis,
		Provenance:         normalizeEvidenceProvenance(input.Provenance),
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
	ArtifactID          string
	EvidenceCount       int
	Supporting          int
	Weakening           int
	Refuting            int
	HasEvidence         bool     // true if at least one evidence item exists
	FEff                int      // computed: min(formality_level_i) across evidence chain
	FormalityScaleID    string   // scale id for the evidence item that determines FEff
	FormalityBridgeLoss string   // bridge/loss label for the evidence item that determines FEff
	GEff                []string // computed: union(claim_scope_i) across evidence chain
	REff                float64  // computed: min(effective_score) across evidence chain
	MinFreshness        string   // earliest parsed valid_until across all evidence, preserving the original carrier string
	WeakestCL           int      // minimum congruence level
	WeakestF            int      // compatibility alias for FEff
	ExpectedScope       []string // explicit acceptance identifiers, when available
	CoverageGaps        []string // expected scope not covered by GEff
	CoverageKnown       bool     // false when the problem frame has no explicit identifiers
	Summary             string   // human-readable one-liner
}

// ComputeWLNKSummary returns a WLNK summary for an artifact based on its evidence items.
// R_eff is computed as min(effective_score_i) where:
//   - base_score: supports=1.0, weakens=0.5, refutes=0.0
//   - CL penalty: CL3=0.0, CL2=0.1, CL1=0.4, CL0=0.9
//   - decay: expired evidence scores 0.1 regardless of verdict
//   - effective_score = max(0, base_score - clPenalty)
func ComputeWLNKSummary(ctx context.Context, store ArtifactStore, artifactID string) WLNKSummary {
	result := WLNKSummary{
		ArtifactID: artifactID,
		WeakestCL:  3,
		FEff:       0,
		WeakestF:   0,
	}
	artifactItem, artifactErr := store.Get(ctx, artifactID)
	decisionClaims := []DecisionClaim(nil)

	if artifactErr == nil && artifactItem.Meta.Kind == KindDecisionRecord {
		decisionClaims = artifactItem.UnmarshalDecisionFields().Claims
	}

	result.ExpectedScope, result.CoverageKnown = explicitAcceptanceScope(ctx, store, artifactID)
	result.CoverageGaps = append(result.CoverageGaps, result.ExpectedScope...)

	items, err := store.GetEvidenceItems(ctx, artifactID)
	if err != nil || len(items) == 0 {
		result.Summary = "no evidence attached"
		return result
	}

	// Filter out superseded evidence (FPF F.10:6.1 — superseded within same Window)
	var activeItems []EvidenceItem
	for _, e := range items {
		if e.Verdict != "superseded" {
			activeItems = append(activeItems, e)
		}
	}

	if len(activeItems) == 0 {
		result.Summary = "no active evidence (all superseded)"
		return result
	}

	result.EvidenceCount = len(activeItems)
	result.HasEvidence = true
	now := time.Now().UTC()
	minREff := 1.0
	minFormality := 9
	minFreshnessAt := time.Time{}
	hasMinFreshness := false

	for _, e := range activeItems {
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
		if e.FormalityLevel < minFormality {
			minFormality = e.FormalityLevel
			formalityScale := evidenceItemFormalityScale(e)
			result.FormalityScaleID = formalityScale.ScaleID
			result.FormalityBridgeLoss = evidenceItemFormalityBridgeLoss(e, formalityScale)
		}

		expiry, ok := reff.ParseValidUntil(e.ValidUntil)
		if ok {
			if !hasMinFreshness || expiry.Before(minFreshnessAt) {
				minFreshnessAt = expiry
				result.MinFreshness = e.ValidUntil
				hasMinFreshness = true
			}
		}

		// Compute per-item effective score for R_eff
		score := scoreEvidence(e, now)
		if score < minREff {
			minREff = score
		}
	}

	result.FEff = minFormality
	result.WeakestF = minFormality
	result.GEff = computeClaimCoverage(activeItems, decisionClaims)
	result.REff = minREff
	if result.CoverageKnown {
		result.CoverageGaps = differenceScope(result.ExpectedScope, activeItems, decisionClaims)
	} else {
		result.CoverageGaps = nil
	}

	// Build summary
	var parts []string
	parts = append(parts, formatAssuranceSummary(result))
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
	if result.MinFreshness != "" {
		if expiry, ok := reff.ParseValidUntil(result.MinFreshness); ok {
			if expiry.Before(now) {
				parts = append(parts, "STALE evidence")
			} else {
				days := int(expiry.Sub(now).Hours() / 24)
				parts = append(parts, fmt.Sprintf("freshest expires in %dd", days))
			}
		}
	}
	if result.WeakestCL < 3 {
		clLabels := map[int]string{0: "opposed", 1: "different context", 2: "similar context"}
		parts = append(parts, fmt.Sprintf("weakest CL: %s", clLabels[result.WeakestCL]))
	}
	if len(result.CoverageGaps) > 0 {
		parts = append(parts, "coverage gaps: "+strings.Join(result.CoverageGaps, ", "))
	}

	result.Summary = strings.Join(parts, ", ")
	return result
}

// scoreEvidence delegates to reff (single source of truth) and applies the
// C.28 causal-basis cap so the WLNK summary honors CC-B3.9 in lockstep with
// the assurance engine. Realizability is not yet plumbed per-claim into this
// path (TODO post-7.1); the cap fires on CausalSupportBasis alone here.
func scoreEvidence(e EvidenceItem, now time.Time) float64 {
	return reff.ScoreEvidenceWithCausalBasis(
		e.Type,
		e.Verdict,
		e.CongruenceLevel,
		string(e.CausalSupportBasis),
		"",
		e.ValidUntil,
		now,
	)
}

func defaultEvidenceFormalityLevel(evidenceType string) int {
	switch strings.ToLower(strings.TrimSpace(evidenceType)) {
	case "measurement", "test", "benchmark", "audit":
		return 2
	case "research":
		return 1
	default:
		return 1
	}
}

func normalizeFormalityLevel(level int) int {
	return reff.NormalizeFormalityLevel(level)
}

func evidenceInputFormalityScale(scaleID string, level int) reff.FormalityScale {
	scale := reff.FormalityScale{
		ScaleID: scaleID,
		Level:   level,
	}
	return reff.NormalizeFormalityScale(scale)
}

func evidenceFormalityBridge(scale reff.FormalityScale) *reff.FormalityBridge {
	switch scale.ScaleID {
	case reff.FormalityScaleLegacy:
		bridge := reff.LegacyFormalityBridge(scale.Level)
		return &bridge
	case reff.FormalityScaleUnversioned:
		bridge := reff.UnversionedFormalityBridge(scale.Level)
		return &bridge
	default:
		return nil
	}
}

func evidenceItemFormalityScale(item EvidenceItem) reff.FormalityScale {
	if item.FormalityScale != nil {
		return reff.NormalizeFormalityScale(*item.FormalityScale)
	}

	return reff.UnversionedFormalityScale(item.FormalityLevel)
}

func evidenceItemFormalityBridgeLoss(item EvidenceItem, scale reff.FormalityScale) string {
	if item.FormalityBridge != nil {
		return item.FormalityBridge.Loss
	}
	if bridge := evidenceFormalityBridge(scale); bridge != nil {
		return bridge.Loss
	}

	return reff.FormalityBridgeNoLoss
}

func normalizeClaimScope(scope []string) []string {
	if len(scope) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(scope))
	normalized := make([]string, 0, len(scope))

	for _, item := range scope {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	sort.Strings(normalized)

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func computeClaimCoverage(items []EvidenceItem, claims []DecisionClaim) []string {
	scope := make([]string, 0, len(items))

	for _, item := range items {
		itemScope := evidenceCoverageScope(item, claims)
		scope = append(scope, itemScope...)
	}

	return normalizeClaimScope(scope)
}

func evidenceCoverageScope(item EvidenceItem, claims []DecisionClaim) []string {
	return mergeDecisionCoverageScope(claims, item.ClaimRefs, item.ClaimScope)
}

func measuredCriteriaScope(criteriaMet []string, criteriaNotMet []string, scopeCandidates []string) []string {
	scope := make([]string, 0, len(criteriaMet)+len(criteriaNotMet))
	scope = append(scope, criteriaMet...)
	scope = append(scope, criteriaNotMet...)
	scope = normalizeClaimScope(scope)

	aliasIndex := buildCriterionAliasIndex(scopeCandidates)
	resolved := make([]string, 0, len(scope))

	for _, item := range scope {
		resolvedItem := resolveMeasuredCriterion(item, aliasIndex)
		resolved = append(resolved, resolvedItem)
	}

	return normalizeClaimScope(resolved)
}

func differenceScope(expected []string, items []EvidenceItem, claims []DecisionClaim) []string {
	if len(expected) == 0 {
		return nil
	}

	coveredKeys := coverageIdentityKeySet(items, claims)
	gaps := make([]string, 0, len(expected))

	for _, item := range expected {
		identityKey := coverageIdentityKey(item, claims)
		if identityKey == "" {
			gaps = append(gaps, item)
			continue
		}
		if _, ok := coveredKeys[identityKey]; ok {
			continue
		}
		gaps = append(gaps, item)
	}

	return gaps
}

func coverageIdentityKeySet(items []EvidenceItem, claims []DecisionClaim) map[string]struct{} {
	identityKeys := make(map[string]struct{})

	for _, item := range items {
		coverageRefs := mergedDecisionCoverageRefs(
			claims,
			item.ClaimRefs,
			item.ClaimScope,
		)
		unresolvedScope := unresolvedDecisionCoverageScope(
			claims,
			coverageRefs,
			item.ClaimScope,
		)

		for _, ref := range coverageRefs {
			identityKeys[coverageClaimRefKey(ref)] = struct{}{}
		}
		for _, scopeItem := range unresolvedScope {
			identityKeys[coverageLiteralScopeKey(scopeItem)] = struct{}{}
		}
	}

	return identityKeys
}

func coverageIdentityKey(scope string, claims []DecisionClaim) string {
	normalizedScope := normalizeClaimScope([]string{scope})

	if len(normalizedScope) == 0 {
		return ""
	}

	coverageRefs := mergedDecisionCoverageRefs(
		claims,
		nil,
		normalizedScope,
	)
	if len(coverageRefs) > 0 {
		return coverageClaimRefKey(coverageRefs[0])
	}

	return coverageLiteralScopeKey(normalizedScope[0])
}

func coverageClaimRefKey(ref string) string {
	return "claim:" + strings.TrimSpace(ref)
}

func coverageLiteralScopeKey(scope string) string {
	return "scope:" + strings.TrimSpace(scope)
}

func explicitAcceptanceScope(ctx context.Context, store ArtifactStore, artifactID string) ([]string, bool) {
	artifactItem, err := store.Get(ctx, artifactID)
	if err != nil {
		return nil, false
	}

	if artifactItem.Meta.Kind == KindProblemCard {
		scope := explicitAcceptanceCriteria(artifactItem.UnmarshalProblemFields().Acceptance)
		return scope, len(scope) > 0
	}

	if artifactItem.Meta.Kind != KindDecisionRecord {
		return nil, false
	}

	scope := make([]string, 0)
	for _, link := range artifactItem.Meta.Links {
		if link.Type != "based_on" {
			continue
		}
		problem, err := store.Get(ctx, link.Ref)
		if err != nil || problem.Meta.Kind != KindProblemCard {
			continue
		}
		scope = append(scope, explicitAcceptanceCriteria(problem.UnmarshalProblemFields().Acceptance)...)
	}

	scope = normalizeClaimScope(scope)
	return scope, len(scope) > 0
}

func measurementScopeCandidates(ctx context.Context, store ArtifactStore, decision *Artifact) []string {
	scope := make([]string, 0)
	scope = append(scope, decision.UnmarshalDecisionFields().PostConds...)

	acceptanceScope, _ := explicitAcceptanceScope(ctx, store, decision.Meta.ID)
	scope = append(scope, acceptanceScope...)

	return normalizeClaimScope(scope)
}

func explicitAcceptanceCriteria(acceptance string) []string {
	lines := strings.Split(acceptance, "\n")
	criteria := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "- [ ] "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ] ")))
		case strings.HasPrefix(trimmed, "- [x] "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "- [x] ")))
		case strings.HasPrefix(trimmed, "- "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case strings.HasPrefix(trimmed, "* "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "* ")))
		default:
			return nil
		}
	}

	return normalizeClaimScope(criteria)
}

func buildCriterionAliasIndex(candidates []string) map[string]string {
	counts := make(map[string]int)
	aliases := make(map[string]string)

	for _, candidate := range normalizeClaimScope(candidates) {
		for _, key := range criterionAliasKeys(candidate) {
			counts[key]++
			if _, exists := aliases[key]; exists {
				continue
			}
			aliases[key] = candidate
		}
	}

	index := make(map[string]string)

	for key, candidate := range aliases {
		if counts[key] != 1 {
			continue
		}
		index[key] = candidate
	}

	return index
}

func resolveMeasuredCriterion(value string, aliasIndex map[string]string) string {
	for _, key := range criterionAliasKeys(value) {
		candidate, ok := aliasIndex[key]
		if ok {
			return candidate
		}
	}

	trimmed := stripTrailingCriterionAnnotations(value)
	trimmed = strings.TrimSpace(trimmed)
	if trimmed != "" {
		return trimmed
	}

	return strings.TrimSpace(value)
}

func criterionAliasKeys(value string) []string {
	keys := make([]string, 0, 2)

	exactKey := criterionMatchKey(value)
	if exactKey != "" {
		keys = append(keys, exactKey)
	}

	trimmedKey := criterionMatchKey(stripTrailingCriterionAnnotations(value))
	if trimmedKey != "" && trimmedKey != exactKey {
		keys = append(keys, trimmedKey)
	}

	return keys
}

func criterionMatchKey(value string) string {
	trimmed := trimCriterionLeadMarkers(value)
	trimmed = strings.ToLower(strings.TrimSpace(trimmed))
	fields := strings.Fields(trimmed)
	return strings.Join(fields, " ")
}

func stripTrailingCriterionAnnotations(value string) string {
	trimmed := trimCriterionLeadMarkers(value)
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimRight(trimmed, ".,;:")

	for {
		next, changed := trimTrailingCriterionGroup(trimmed, '(', ')')
		if changed {
			trimmed = strings.TrimSpace(next)
			trimmed = strings.TrimRight(trimmed, ".,;:")
			continue
		}

		next, changed = trimTrailingCriterionGroup(trimmed, '[', ']')
		if changed {
			trimmed = strings.TrimSpace(next)
			trimmed = strings.TrimRight(trimmed, ".,;:")
			continue
		}

		return strings.TrimSpace(trimmed)
	}
}

func trimCriterionLeadMarkers(value string) string {
	trimmed := strings.TrimSpace(value)

	switch {
	case strings.HasPrefix(trimmed, "- [ ] "):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ] "))
	case strings.HasPrefix(trimmed, "- [x] "):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "- [x] "))
	case strings.HasPrefix(trimmed, "- "):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	case strings.HasPrefix(trimmed, "* "):
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))
	default:
		return trimmed
	}
}

func trimTrailingCriterionGroup(value string, open byte, close byte) (string, bool) {
	if value == "" {
		return value, false
	}
	if value[len(value)-1] != close {
		return value, false
	}

	depth := 0

	for i := len(value) - 1; i >= 0; i-- {
		switch value[i] {
		case close:
			depth++
		case open:
			depth--
			if depth == 0 {
				return value[:i], true
			}
		}
	}

	return value, false
}

func formatAssuranceSummary(summary WLNKSummary) string {
	scaleID := summary.FormalityScaleID
	if scaleID == "" {
		scaleID = reff.FormalityScaleCurrent
	}
	bridgeLoss := summary.FormalityBridgeLoss
	if bridgeLoss == "" {
		bridgeLoss = reff.FormalityBridgeNoLoss
	}

	formality := fmt.Sprintf(
		"F%d (%s) scale=%s bridge_loss=%s",
		summary.FEff,
		formalityLabel(summary.FEff),
		scaleID,
		bridgeLoss,
	)
	coverage := "G: no claim scope"
	switch {
	case summary.CoverageKnown:
		coveredCriteria := len(summary.ExpectedScope) - len(summary.CoverageGaps)
		if coveredCriteria < 0 {
			coveredCriteria = 0
		}
		coverage = fmt.Sprintf("G: %d/%d criteria covered", coveredCriteria, len(summary.ExpectedScope))
	case len(summary.GEff) > 0:
		coverage = fmt.Sprintf("G: %d covered (acceptance ids unavailable)", len(summary.GEff))
	}
	return fmt.Sprintf(
		"Assurance: %s | %s | R: %.2f | boundary=diagnostic_not_approval_gate_claim_truth_global_truth_or_publication",
		formality,
		coverage,
		summary.REff,
	)
}

func formalityLabel(level int) string {
	switch level {
	case 0:
		return "informal-or-unsubstantiated"
	case 1:
		return "structured-narrative"
	case 2:
		return "structured-schema-or-test"
	case 3:
		return "formalizable"
	case 4:
		return "predicate-like"
	case 5:
		return "executable-semantics"
	case 6:
		return "checked-model"
	case 7:
		return "machine-checkable-obligations"
	case 8:
		return "machine-checked-proof"
	case 9:
		return "proof-grade"
	default:
		return "unknown"
	}
}

// modeRank maps Mode to a numeric rank for comparison.
func modeRank(m Mode) int {
	switch m {
	case ModeNote:
		return 0
	case ModeTactical:
		return 1
	case ModeStandard:
		return 2
	case ModeDeep:
		return 3
	default:
		return 1
	}
}

// maxMode returns the higher of two modes (deeper reasoning wins).
func maxMode(a, b Mode) Mode {
	if modeRank(a) >= modeRank(b) {
		return a
	}
	return b
}

// inferModeFromChain determines the minimum mode based on what artifacts
// actually exist in the reasoning chain. This reflects what happened,
// not what the agent declared.
func inferModeFromChain(ctx context.Context, store ArtifactStore, problemRefs []string, portfolioRef string) Mode {
	// No linked problem → note-level (agent just called decide directly)
	if len(problemRefs) == 0 && portfolioRef == "" {
		return ModeTactical
	}

	// Check if any linked problem has characterization
	hasCharacterization := false
	for _, ref := range problemRefs {
		prob, err := store.Get(ctx, ref)
		if err != nil {
			continue
		}
		if strings.Contains(prob.Body, "## Characterization") {
			hasCharacterization = true
			break
		}
	}

	// Check if portfolio has comparison
	hasComparison := false
	if portfolioRef != "" {
		portfolio, err := store.Get(ctx, portfolioRef)
		if err == nil {
			hasComparison = strings.Contains(portfolio.Body, "## Comparison")
		}
	}

	// Derive mode from chain evidence
	switch {
	case hasCharacterization && hasComparison:
		return ModeStandard
	case hasCharacterization || hasComparison:
		return ModeStandard
	case len(problemRefs) > 0:
		return ModeTactical // has problem but no char/compare = tactical with frame
	default:
		return ModeTactical
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
