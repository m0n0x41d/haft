package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/internal/reff"
	"github.com/m0n0x41d/quint-code/logger"
)

// DecideInput is the input for creating a DecisionRecord.
type DecideInput struct {
	ProblemRef      string            `json:"problem_ref,omitempty"`  // single problem (backward compat)
	ProblemRefs     []string          `json:"problem_refs,omitempty"` // multiple problems
	PortfolioRef    string            `json:"portfolio_ref,omitempty"`
	SelectedTitle   string            `json:"selected_title"`
	WhySelected     string            `json:"why_selected"`
	WhyNotOthers    []RejectionReason `json:"why_not_others,omitempty"`
	Invariants      []string          `json:"invariants,omitempty"`
	PreConditions   []string          `json:"pre_conditions,omitempty"`
	PostConditions  []string          `json:"post_conditions,omitempty"`
	Admissibility   []string          `json:"admissibility,omitempty"`
	EvidenceReqs    []string          `json:"evidence_requirements,omitempty"`
	Rollback        *RollbackSpec     `json:"rollback,omitempty"`
	RefreshTriggers []string          `json:"refresh_triggers,omitempty"`
	WeakestLink     string            `json:"weakest_link,omitempty"`
	ValidUntil      string            `json:"valid_until,omitempty"`
	Context         string            `json:"context,omitempty"`
	Mode            string            `json:"mode,omitempty"`
	AffectedFiles   []string          `json:"affected_files,omitempty"`
	SearchKeywords  string            `json:"search_keywords,omitempty"`
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
	ID          string
	Now         time.Time
	Mode        Mode   // computed from chain (max of declared and inferred)
	Context     string // inherited from linked artifacts if not in input
	ProblemBody string // pre-fetched problem markdown for signal/constraints extraction
	Links       []Link
	ProblemRefs []string // merged refs
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

// BuildDecisionArtifact constructs a DecisionRecord from input and pre-fetched context. Pure — no side effects.
func BuildDecisionArtifact(dctx DecideContext, input DecideInput) (*Artifact, error) {
	if input.SelectedTitle == "" {
		return nil, fmt.Errorf("selected_title is required — what variant was chosen?")
	}
	if input.WhySelected == "" {
		return nil, fmt.Errorf("why_selected is required — rationale for the choice")
	}

	title := input.SelectedTitle

	// Build the DRR markdown — FPF E.9 four-component structure
	var body strings.Builder
	body.WriteString(fmt.Sprintf("# %s\n", title))

	// === Component 1: Problem Frame ===
	body.WriteString("\n## 1. Problem Frame\n\n")
	if dctx.ProblemBody != "" {
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

	sd, _ := json.Marshal(DecisionFields{
		SelectedTitle: input.SelectedTitle,
		WhySelected:   input.WhySelected,
		WeakestLink:   input.WeakestLink,
		Invariants:    input.Invariants,
		PostConds:     input.PostConditions,
		Admissibility: input.Admissibility,
	})
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
func Decide(ctx context.Context, store ArtifactStore, quintDir string, input DecideInput) (*Artifact, string, error) {
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
	var problemBody string
	if primaryRef != "" {
		if prob, err := store.Get(ctx, primaryRef); err == nil {
			problemBody = prob.Body
		}
	}

	// Pure construction
	a, err := BuildDecisionArtifact(DecideContext{
		ID:          id,
		Now:         now,
		Mode:        mode,
		Context:     resolvedContext,
		ProblemBody: problemBody,
		Links:       links,
		ProblemRefs: problemRefs,
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

	filePath, err := WriteFile(quintDir, a)
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

	// Store updated hashes
	if err := store.SetAffectedFiles(ctx, input.DecisionRef, files); err != nil {
		return nil, fmt.Errorf("store baseline hashes: %w", err)
	}

	logger.ArtifactOp("baseline", input.DecisionRef, string(a.Meta.Kind))
	logger.Debug().Str("decision_ref", input.DecisionRef).Int("files", len(files)).Msg("baseline.complete")

	return files, nil
}

// CheckDrift compares current file state against stored baseline hashes for all active decisions.
func CheckDrift(ctx context.Context, store ArtifactStore, projectRoot string) ([]DriftReport, error) {
	decisions, err := store.ListByKind(ctx, KindDecisionRecord, 500)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}

	// Notes are observations, not implementations — skip baseline/drift checks for them

	var reports []DriftReport

	for _, d := range decisions {
		if d.Meta.Status != StatusActive {
			continue
		}

		files, err := store.GetAffectedFiles(ctx, d.Meta.ID)
		if err != nil || len(files) == 0 {
			continue
		}

		report := DriftReport{
			DecisionID:    d.Meta.ID,
			DecisionTitle: d.Meta.Title,
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
					Path:   f.Path,
					Status: DriftMissing,
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
				})
				hasDrift = true
			}
		}

		// Only include reports with drift or missing baselines
		if hasDrift || !hasAnyHash {
			reports = append(reports, report)
		}
	}

	logger.Debug().Int("drift_reports", len(reports)).Msg("drift.check.complete")

	return reports, nil
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
func Measure(ctx context.Context, store ArtifactStore, quintDir string, input MeasureInput) (*Artifact, error) {
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
					"Run `quint_decision(action=\"baseline\")` first for CL3 scoring.")
		}
	} else {
		// No affected_files — can't verify via baseline, treat as unverified
		hasBaseline = false
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
		FormalityLevel:  5,
	}, input.DecisionRef); err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}

	writeFileQuiet(quintDir, a)

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
func ComputeWLNKSummary(ctx context.Context, store ArtifactStore, artifactID string) WLNKSummary {
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

// scoreEvidence delegates to reff.ScoreEvidence (single source of truth).
func scoreEvidence(e EvidenceItem, now time.Time) float64 {
	return reff.ScoreEvidence(e.Verdict, e.CongruenceLevel, e.ValidUntil, now)
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
