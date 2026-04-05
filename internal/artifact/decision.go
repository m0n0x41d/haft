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
	ProblemRef          string            `json:"problem_ref,omitempty"`  // single problem (backward compat)
	ProblemRefs         []string          `json:"problem_refs,omitempty"` // multiple problems
	PortfolioRef        string            `json:"portfolio_ref,omitempty"`
	SelectedTitle       string            `json:"selected_title"`
	WhySelected         string            `json:"why_selected"`
	SelectionPolicy     string            `json:"selection_policy"`
	CounterArgument     string            `json:"counterargument"`
	WhyNotOthers        []RejectionReason `json:"why_not_others,omitempty"`
	Invariants          []string          `json:"invariants,omitempty"`
	PreConditions       []string          `json:"pre_conditions,omitempty"`
	PostConditions      []string          `json:"post_conditions,omitempty"`
	Admissibility       []string          `json:"admissibility,omitempty"`
	EvidenceReqs        []string          `json:"evidence_requirements,omitempty"`
	Rollback            *RollbackSpec     `json:"rollback,omitempty"`
	RefreshTriggers     []string          `json:"refresh_triggers,omitempty"`
	WeakestLink         string            `json:"weakest_link,omitempty"`
	ValidUntil          string            `json:"valid_until,omitempty"`
	Context             string            `json:"context,omitempty"`
	Mode                string            `json:"mode,omitempty"`
	AffectedFiles       []string          `json:"affected_files,omitempty"`
	Predictions         []PredictionInput `json:"predictions,omitempty"`
	SearchKeywords      string            `json:"search_keywords,omitempty"`
	FirstModuleCoverage bool              `json:"first_module_coverage,omitempty"`
}

// PredictionInput is a testable claim that measure should verify.
type PredictionInput struct {
	Claim      string `json:"claim"`
	Observable string `json:"observable"`
	Threshold  string `json:"threshold"`
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
			Claim:      strings.TrimSpace(value.Claim),
			Observable: strings.TrimSpace(value.Observable),
			Threshold:  strings.TrimSpace(value.Threshold),
		}
		if prediction.Claim == "" && prediction.Observable == "" && prediction.Threshold == "" {
			continue
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
	input.Mode = strings.TrimSpace(input.Mode)
	input.SearchKeywords = strings.TrimSpace(input.SearchKeywords)

	input.WhyNotOthers = normalizeRejectionReasons(input.WhyNotOthers)
	input.Invariants = compactStrings(input.Invariants)
	input.PreConditions = compactStrings(input.PreConditions)
	input.PostConditions = compactStrings(input.PostConditions)
	input.Admissibility = compactStrings(input.Admissibility)
	input.EvidenceReqs = compactStrings(input.EvidenceReqs)
	input.RefreshTriggers = compactStrings(input.RefreshTriggers)
	input.AffectedFiles = compactStrings(input.AffectedFiles)
	input.Predictions = normalizePredictionInputs(input.Predictions)
	input.Rollback = normalizeRollbackSpec(input.Rollback)

	return input
}

func validateDecisionInput(input DecideInput) error {
	var problems []string

	if input.SelectedTitle == "" {
		problems = append(problems, "selected_title is required — what variant was chosen?")
	}
	if input.WhySelected == "" {
		problems = append(problems, "why_selected is required — rationale for the choice")
	}
	if input.SelectionPolicy == "" {
		problems = append(problems, "selection_policy is required — state the explicit policy used to choose this option")
	}
	if input.CounterArgument == "" {
		problems = append(problems, "counterargument is required — record the strongest argument against this decision")
	}
	if input.WeakestLink == "" {
		problems = append(problems, "weakest_link is required — state the selected variant's weakest link")
	}
	if len(input.WhyNotOthers) == 0 {
		problems = append(problems, "why_not_others is required — record at least one rejected alternative and why it lost")
	}
	if input.Rollback == nil || len(input.Rollback.Triggers) == 0 {
		problems = append(problems, "rollback.triggers is required — record at least one trigger that would force reversal")
	}

	for i, rejection := range input.WhyNotOthers {
		switch {
		case rejection.Variant == "":
			problems = append(problems, fmt.Sprintf("why_not_others[%d].variant is required — name the rejected alternative", i))
		case rejection.Reason == "":
			problems = append(problems, fmt.Sprintf("why_not_others[%d].reason is required — explain why %q lost", i, rejection.Variant))
		case strings.EqualFold(rejection.Variant, input.SelectedTitle):
			problems = append(problems, fmt.Sprintf("why_not_others[%d].variant must not repeat selected_title %q", i, input.SelectedTitle))
		}
	}

	for i, prediction := range input.Predictions {
		problems = append(problems, predictionValidationProblems(i, prediction)...)
	}

	if len(problems) == 0 {
		return nil
	}

	return fmt.Errorf("decision record is incomplete:\n- %s", strings.Join(problems, "\n- "))
}

func predictionValidationProblems(index int, prediction PredictionInput) []string {
	problems := []string{}

	if prediction.Claim == "" {
		problems = append(problems, fmt.Sprintf("predictions[%d].claim is required — predictions must include claim, observable, and threshold together", index))
	}
	if prediction.Observable == "" {
		problems = append(problems, fmt.Sprintf("predictions[%d].observable is required — predictions must include claim, observable, and threshold together", index))
	}
	if prediction.Threshold == "" {
		problems = append(problems, fmt.Sprintf("predictions[%d].threshold is required — predictions must include claim, observable, and threshold together", index))
	}

	return problems
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

	rollbackTriggers := []string(nil)
	rollbackSteps := []string(nil)
	rollbackBlastRadius := ""
	if input.Rollback != nil {
		rollbackTriggers = input.Rollback.Triggers
		rollbackSteps = input.Rollback.Steps
		rollbackBlastRadius = input.Rollback.BlastRadius
	}

	decisionFields := DecisionFields{
		ProblemRefs:          dctx.ProblemRefs,
		SelectedTitle:        input.SelectedTitle,
		WhySelected:          input.WhySelected,
		SelectionPolicy:      input.SelectionPolicy,
		CounterArgument:      input.CounterArgument,
		WeakestLink:          input.WeakestLink,
		WhyNotOthers:         input.WhyNotOthers,
		Claims:               newDecisionClaims(input.Predictions),
		PreConditions:        input.PreConditions,
		RollbackTriggers:     rollbackTriggers,
		RollbackSteps:        rollbackSteps,
		RollbackBlastRadius:  rollbackBlastRadius,
		Invariants:           input.Invariants,
		PostConds:            input.PostConditions,
		Admissibility:        input.Admissibility,
		EvidenceRequirements: input.EvidenceReqs,
		RefreshTriggers:      input.RefreshTriggers,
		FirstModuleCoverage:  input.FirstModuleCoverage,
	}
	decisionFields.Predictions = decisionPredictionsFromClaims(decisionFields.Claims)

	if len(input.Invariants) > 0 {
		body.WriteString("\n**Invariants:**\n")
		for _, inv := range input.Invariants {
			body.WriteString(fmt.Sprintf("- %s\n", inv))
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

// Decide creates a DecisionRecord artifact. Orchestrates effects around BuildDecisionArtifact.
func Decide(ctx context.Context, store ArtifactStore, haftDir string, input DecideInput) (*Artifact, string, error) {
	input = normalizeDecisionInput(input)

	seq, err := store.NextSequence(ctx, KindDecisionRecord)
	if err != nil {
		return nil, "", fmt.Errorf("generate ID: %w", err)
	}

	id := GenerateID(KindDecisionRecord, seq)
	now := time.Now().UTC()

	// Pure: merge refs
	problemRefs := MergeProblemRefs(input.ProblemRef, input.ProblemRefs)
	links := BuildLinks(problemRefs, input.PortfolioRef)

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

// BaselineInput is the input for snapshotting file hashes after implementation.
type BaselineInput struct {
	DecisionRef   string   `json:"decision_ref"`
	AffectedFiles []string `json:"affected_files,omitempty"` // optional: replace file list before hashing
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

	// If new files provided, replace the list
	if len(input.AffectedFiles) > 0 {
		var files []AffectedFile
		for _, f := range input.AffectedFiles {
			files = append(files, AffectedFile{Path: f})
		}
		if err := store.SetAffectedFiles(ctx, input.DecisionRef, files); err != nil {
			return nil, fmt.Errorf("replace affected files: %w", err)
		}
	}

	// Get current affected files
	files, err := store.GetAffectedFiles(ctx, input.DecisionRef)
	if err != nil {
		return nil, fmt.Errorf("get affected files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("decision %s has no affected_files — nothing to baseline", input.DecisionRef)
	}

	// Compute SHA-256 for each file
	for i := range files {
		absPath := filepath.Join(projectRoot, files[i].Path)
		hash, err := hashFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", files[i].Path, err)
		}
		files[i].Hash = hash
	}

	// Store updated file hashes
	if err := store.SetAffectedFiles(ctx, input.DecisionRef, files); err != nil {
		return nil, fmt.Errorf("store baseline hashes: %w", err)
	}

	// Extract and store symbol-level snapshots (tree-sitter powered)
	var symbols []AffectedSymbol
	for _, f := range files {
		snapshots, err := codebase.ExtractSymbolSnapshots(projectRoot, f.Path)
		if err != nil || snapshots == nil {
			continue
		}
		for _, s := range snapshots {
			symbols = append(symbols, AffectedSymbol{
				FilePath:   s.FilePath,
				SymbolName: s.SymbolName,
				SymbolKind: s.SymbolKind,
				Line:       s.Line,
				EndLine:    s.EndLine,
				Hash:       s.Hash,
			})
		}
	}
	if len(symbols) > 0 {
		if err := store.SetAffectedSymbols(ctx, input.DecisionRef, symbols); err != nil {
			logger.Warn().Str("decision_ref", input.DecisionRef).Err(err).Msg("baseline.symbols_failed")
		}
	}

	driftManifests, err := buildDriftScopeManifests(projectRoot, files)
	if err != nil {
		return nil, fmt.Errorf("build drift manifests: %w", err)
	}

	err = persistDriftManifests(ctx, store, a, driftManifests)
	if err != nil {
		return nil, fmt.Errorf("persist drift manifests: %w", err)
	}

	logger.ArtifactOp("baseline", input.DecisionRef, string(a.Meta.Kind))
	logger.Debug().Str("decision_ref", input.DecisionRef).
		Int("files", len(files)).
		Int("symbols", len(symbols)).
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

	for _, d := range decisions {
		decisionArtifact, err := store.Get(ctx, d.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("get decision %s: %w", d.Meta.ID, err)
		}

		files, err := store.GetAffectedFiles(ctx, d.Meta.ID)
		if err != nil || len(files) == 0 {
			continue
		}

		decisionFields := decisionArtifact.UnmarshalDecisionFields()

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

		if !hasAnyHash {
			// No baseline set — check git to distinguish "forgot to close loop" from "not started"
			anyChanged := false
			for _, f := range files {
				report.Files = append(report.Files, DriftItem{
					Path:   f.Path,
					Status: DriftNoBaseline,
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
					Path:   f.Path,
					Status: DriftNoBaseline,
				})
				continue
			}

			absPath := filepath.Join(projectRoot, f.Path)
			currentHash, err := hashFile(absPath)
			if err != nil {
				// File doesn't exist or can't be read
				report.Files = append(report.Files, DriftItem{
					Path:       f.Path,
					Status:     DriftMissing,
					Invariants: copyDriftInvariants(decisionFields.Invariants),
				})
				hasDrift = true
				continue
			}

			if currentHash != f.Hash {
				lines := gitDiffStat(projectRoot, f.Path)
				report.Files = append(report.Files, DriftItem{
					Path:         f.Path,
					Status:       DriftModified,
					LinesChanged: lines,
					Invariants:   copyDriftInvariants(decisionFields.Invariants),
				})
				hasDrift = true
			}
		}

		addedFiles, err := detectAddedFiles(projectRoot, files, decisionFields.DriftManifests)
		if err != nil {
			return nil, fmt.Errorf("detect added files for %s: %w", d.Meta.ID, err)
		}
		for _, path := range addedFiles {
			report.Files = append(report.Files, DriftItem{
				Path:       path,
				Status:     DriftAdded,
				Invariants: copyDriftInvariants(decisionFields.Invariants),
			})
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

		scopeFiles, err := listScopeFiles(projectRoot, manifest.Scope)
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

func listScopeFiles(projectRoot string, scope string) ([]string, error) {
	scope = normalizeDriftScope(scope)
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

// hashFile computes SHA-256 of a file's contents.
func hashFile(path string) (string, error) {
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
	ArtifactRef     string   `json:"artifact_ref"`
	Content         string   `json:"content"`
	Type            string   `json:"type"`    // measurement, test, research, benchmark, audit
	Verdict         string   `json:"verdict"` // supports, weakens, refutes
	CarrierRef      string   `json:"carrier_ref,omitempty"`
	CongruenceLevel int      `json:"congruence_level"` // 0-3; -1 = not provided (defaults to 3)
	FormalityLevel  int      `json:"formality_level"`  // F0-F3; legacy 0-9 inputs are normalized
	ClaimRefs       []string `json:"claim_refs,omitempty"`
	ClaimScope      []string `json:"claim_scope,omitempty"`
	ValidUntil      string   `json:"valid_until,omitempty"`
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

	scopeCandidates := measurementScopeCandidates(ctx, store, a)
	criteriaMetScope := measuredCriteriaScope(input.CriteriaMet, nil, scopeCandidates)
	criteriaNotMetScope := measuredCriteriaScope(nil, input.CriteriaNotMet, scopeCandidates)
	claimScope := make([]string, 0, len(criteriaMetScope)+len(criteriaNotMetScope))
	claimScope = append(claimScope, criteriaMetScope...)
	claimScope = append(claimScope, criteriaNotMetScope...)
	claimScope = normalizeClaimScope(claimScope)

	decisionFields := a.UnmarshalDecisionFields()
	claimRefs := measuredDecisionClaimRefs(
		decisionFields.Claims,
		input.CriteriaMet,
		criteriaMetScope,
		input.CriteriaNotMet,
		criteriaNotMetScope,
	)
	decisionFields.Claims = adjudicateDecisionClaims(
		decisionFields.Claims,
		true,
		input.CriteriaMet,
		criteriaMetScope,
		input.CriteriaNotMet,
		criteriaNotMetScope,
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

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update decision: %w", err)
	}

	// Supersede previous measurements on this artifact (FPF F.10:6.1 — newer evidence replaces older within the same Window)
	if err := store.SupersedeEvidenceByType(ctx, input.DecisionRef, "measurement"); err != nil {
		logger.Warn().Err(err).Str("decision_ref", input.DecisionRef).Msg("failed to supersede old measurements")
	}

	// Record as evidence item
	// CL based on verification quality: baseline exists = CL3, no baseline = CL1 (self-evidence, FPF A.12)
	measureCL := 1 // default: self-evidence (no independent verification)
	if hasBaseline {
		measureCL = 3 // baseline exists = independent file-level verification
	}

	evidID := fmt.Sprintf("evid-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)
	if err := store.AddEvidenceItem(ctx, &EvidenceItem{
		ID:              evidID,
		Type:            "measurement",
		Content:         fmt.Sprintf("Impact measurement: %s\n%s", input.Verdict, input.Findings),
		Verdict:         input.Verdict,
		CongruenceLevel: measureCL,
		FormalityLevel:  2,
		ClaimRefs:       claimRefs,
		ClaimScope:      claimScope,
		ValidUntil:      a.Meta.ValidUntil,
	}, input.DecisionRef); err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}

	writeFileQuiet(haftDir, a)

	if len(measureWarnings) > 0 {
		return a, &WriteWarning{Warnings: measureWarnings}
	}
	return a, nil
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
		claimRefs, err := resolveDecisionEvidenceClaimRefs(artifactItem.UnmarshalDecisionFields().Claims, input.ClaimRefs, input.ClaimScope)
		if err != nil {
			return nil, err
		}

		input.ClaimRefs = claimRefs
		if len(input.ClaimScope) == 0 {
			input.ClaimScope = decisionClaimScopeFromRefs(artifactItem.UnmarshalDecisionFields().Claims, input.ClaimRefs)
		}
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

	id := fmt.Sprintf("evid-%s-%09d", time.Now().Format("20060102"), time.Now().UnixNano()%1000000000)

	item := &EvidenceItem{
		ID:              id,
		Type:            input.Type,
		Content:         input.Content,
		Verdict:         input.Verdict,
		CarrierRef:      input.CarrierRef,
		CongruenceLevel: input.CongruenceLevel,
		FormalityLevel:  normalizeFormalityLevel(input.FormalityLevel),
		ClaimRefs:       input.ClaimRefs,
		ClaimScope:      input.ClaimScope,
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
	HasEvidence   bool     // true if at least one evidence item exists
	FEff          int      // computed: min(formality_level_i) across evidence chain
	GEff          []string // computed: union(claim_scope_i) across evidence chain
	REff          float64  // computed: min(effective_score) across evidence chain
	MinFreshness  string   // earliest parsed valid_until across all evidence, preserving the original carrier string
	WeakestCL     int      // minimum congruence level
	WeakestF      int      // compatibility alias for FEff
	ExpectedScope []string // explicit acceptance identifiers, when available
	CoverageGaps  []string // expected scope not covered by GEff
	CoverageKnown bool     // false when the problem frame has no explicit identifiers
	Summary       string   // human-readable one-liner
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
	minFormality := 3
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
	result.GEff = computeClaimCoverage(activeItems)
	result.REff = minREff
	if result.CoverageKnown {
		result.CoverageGaps = differenceScope(result.ExpectedScope, result.GEff)
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

// scoreEvidence delegates to reff.ScoreEvidence (single source of truth).
func scoreEvidence(e EvidenceItem, now time.Time) float64 {
	return reff.ScoreEvidence(e.Verdict, e.CongruenceLevel, e.ValidUntil, now)
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
	switch {
	case level < 0:
		return 0
	case level <= 3:
		return level
	case level <= 5:
		return 1
	case level <= 8:
		return 2
	default:
		return 3
	}
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

func computeClaimCoverage(items []EvidenceItem) []string {
	scope := make([]string, 0, len(items))
	for _, item := range items {
		scope = append(scope, item.ClaimScope...)
	}
	return normalizeClaimScope(scope)
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

func differenceScope(expected []string, covered []string) []string {
	if len(expected) == 0 {
		return nil
	}

	coveredSet := make(map[string]struct{}, len(covered))
	for _, item := range covered {
		coveredSet[item] = struct{}{}
	}

	gaps := make([]string, 0, len(expected))
	for _, item := range expected {
		if _, ok := coveredSet[item]; ok {
			continue
		}
		gaps = append(gaps, item)
	}

	return gaps
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
	formality := fmt.Sprintf("F%d (%s)", summary.FEff, formalityLabel(summary.FEff))
	coverage := "G: no claim scope"
	switch {
	case summary.CoverageKnown:
		coverage = fmt.Sprintf("G: %d/%d criteria covered", len(summary.GEff), len(summary.ExpectedScope))
	case len(summary.GEff) > 0:
		coverage = fmt.Sprintf("G: %d covered (acceptance ids unavailable)", len(summary.GEff))
	}
	return fmt.Sprintf("Assurance: %s | %s | R: %.2f", formality, coverage, summary.REff)
}

func formalityLabel(level int) string {
	switch level {
	case 0:
		return "unsubstantiated"
	case 1:
		return "structured-informal"
	case 2:
		return "structured-formal"
	case 3:
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
