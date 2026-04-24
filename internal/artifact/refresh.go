package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
	"github.com/m0n0x41d/haft/logger"
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

const DefaultEpistemicDebtBudget = 30.0

type StaleCategory string

const (
	StaleCategoryEvidenceExpired       StaleCategory = "evidence_expired"
	StaleCategoryREffDegraded          StaleCategory = "reff_degraded"
	StaleCategoryDecisionStale         StaleCategory = "decision_stale"
	StaleCategoryPendingVerification   StaleCategory = "pending_verification"
	StaleCategoryEpistemicDebtExceeded StaleCategory = "epistemic_debt_exceeded"
	StaleCategoryScanFailed            StaleCategory = "scan_failed"
)

type DecisionDebtBreakdown struct {
	DecisionID      string  `json:"decision_id"`
	DecisionTitle   string  `json:"decision_title"`
	TotalED         float64 `json:"total_ed"`
	ExpiredEvidence int     `json:"expired_evidence"`
	MostOverdueDays int     `json:"most_overdue_days"`
}

// StaleItem describes one stale artifact with details.
type StaleItem struct {
	ID           string
	Title        string
	Kind         string
	Category     StaleCategory
	Reason       string
	ValidUntil   string
	DaysStale    int
	REff         float64
	DriftItems   []DriftItem
	TotalED      float64
	DebtBudget   float64
	DebtExcess   float64
	DecisionDebt []DecisionDebtBreakdown
}

type decisionDebtAccumulator struct {
	DecisionID      string
	DecisionTitle   string
	RawTotalED      float64
	ExpiredEvidence int
	MostOverdueDays int
}

type GovernanceAttention struct {
	BacklogCount             int
	InProgressCount          int
	AddressedWithoutDecision []AddressedProblemGap
	InvariantViolations      []InvariantViolationFinding
}

type AddressedProblemGap struct {
	ProblemID string
	Title     string
}

type InvariantViolationFinding struct {
	DecisionID    string
	DecisionTitle string
	Invariant     string
	Reason        string
}

// ScanStale finds all stale decisions and returns actionable info.
// If projectRoot is non-empty, also checks for file drift on baselined decisions.
func ScanStale(ctx context.Context, store ArtifactStore, projectRoot ...string) ([]StaleItem, error) {
	now := time.Now().UTC()
	var items []StaleItem
	seenItems := make(map[string]struct{})

	addItem := func(item StaleItem) {
		key := fmt.Sprintf("%s|%s", item.ID, item.Category)
		if item.ID == "" {
			key = fmt.Sprintf("%s|%s", item.Title, item.Category)
		}
		if _, ok := seenItems[key]; ok {
			return
		}
		seenItems[key] = struct{}{}
		items = append(items, item)
	}

	staleArtifacts := collectStaleArtifacts(ctx, store, addItem)
	for _, a := range staleArtifacts {
		if !staleArtifactShouldSurface(ctx, store, a) {
			continue
		}
		addItem(buildExpiredStaleItem(a, now))
	}

	decisionCandidates, err := store.ListByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		addItem(buildScanFailureItem("active decision scan", err))
	}
	decisions := make([]*Artifact, 0, len(decisionCandidates))
	for _, decision := range decisionCandidates {
		if decision.Meta.Status != StatusActive && decision.Meta.Status != StatusRefreshDue {
			continue
		}
		decisions = append(decisions, decision)
	}
	for _, d := range decisions {
		evidenceItems, evidenceErr := store.GetEvidenceItems(ctx, d.Meta.ID)
		if evidenceErr != nil {
			addItem(buildScanFailureItem("R_eff scan for "+d.Meta.ID, evidenceErr))
			continue
		}
		if len(evidenceItems) == 0 {
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
			addItem(StaleItem{
				ID:       d.Meta.ID,
				Title:    d.Meta.Title,
				Kind:     string(d.Meta.Kind),
				Category: StaleCategoryREffDegraded,
				Reason:   reason,
				REff:     roundTenths(wlnk.REff),
			})
		}
	}

	// Check for claims with verify_after dates that have passed but remain unverified.
	// ListActiveByKind returns lightweight rows without StructuredData, so fetch full artifact per decision.
	for _, d := range decisions {
		full, fetchErr := store.Get(ctx, d.Meta.ID)
		if fetchErr != nil || full == nil {
			continue
		}
		fields := full.UnmarshalDecisionFields()
		for _, claim := range fields.Claims {
			if claim.VerifyAfter == "" || claim.Status != ClaimStatusUnverified {
				continue
			}
			verifyTime, ok := reff.ParseValidUntil(claim.VerifyAfter)
			if !ok {
				continue
			}
			if now.Before(verifyTime) {
				continue
			}
			reason := fmt.Sprintf("claim %s ready for verification (verify_after: %s). Observable: %s. Threshold: %s",
				claim.ID, claim.VerifyAfter, claim.Observable, claim.Threshold)
			addItem(StaleItem{
				ID:       d.Meta.ID,
				Title:    d.Meta.Title,
				Kind:     string(d.Meta.Kind),
				Category: StaleCategoryPendingVerification,
				Reason:   reason,
			})
		}
	}

	// Check for file drift if projectRoot is provided
	if len(projectRoot) > 0 && projectRoot[0] != "" {
		driftReports, driftErr := CheckDrift(ctx, store, projectRoot[0])
		if driftErr != nil {
			addItem(buildScanFailureItem("drift scan", driftErr))
		}
		for _, r := range driftReports {
			addItem(buildDecisionStaleItem(r))
		}
	}

	budget, budgetErr := store.EpistemicDebtBudget(ctx)
	if budgetErr != nil {
		addItem(buildScanFailureItem("epistemic debt budget", budgetErr))
	}

	breakdown, totalED, debtFailures := computeDecisionDebtBreakdown(ctx, store, decisions, now)
	for _, failure := range debtFailures {
		addItem(failure)
	}

	if budgetErr == nil {
		alert := reff.CheckEDBudget(totalED, budget)
		if alert != nil {
			addItem(StaleItem{
				ID:           "system/epistemic-debt",
				Title:        "Epistemic debt budget",
				Kind:         "System",
				Category:     StaleCategoryEpistemicDebtExceeded,
				Reason:       formatEDBudgetReason(*alert, breakdown),
				TotalED:      alert.TotalED,
				DebtBudget:   alert.Budget,
				DebtExcess:   alert.Excess,
				DecisionDebt: breakdown,
			})
		}
	}

	// Sort the highest-risk findings first while keeping artifact-level urgency visible.
	sort.Slice(items, func(i, j int) bool {
		left := staleCategoryPriority(items[i].Category)
		right := staleCategoryPriority(items[j].Category)
		if left != right {
			return left < right
		}
		if items[i].DaysStale != items[j].DaysStale {
			return items[i].DaysStale > items[j].DaysStale
		}
		if items[i].TotalED != items[j].TotalED {
			return items[i].TotalED > items[j].TotalED
		}
		if items[i].REff != items[j].REff {
			return items[i].REff < items[j].REff
		}
		return items[i].Title < items[j].Title
	})

	return items, nil
}

func collectStaleArtifacts(ctx context.Context, store ArtifactStore, addItem func(StaleItem)) []*Artifact {
	staleDecisions, decisionErr := store.FindStaleDecisions(ctx)
	if decisionErr != nil {
		addItem(buildScanFailureItem("stale decision scan", decisionErr))
	}

	staleOther, otherErr := store.FindStaleArtifacts(ctx)
	if otherErr != nil {
		addItem(buildScanFailureItem("stale artifact scan", otherErr))
	}

	seenArtifacts := make(map[string]bool)
	stale := make([]*Artifact, 0, len(staleDecisions)+len(staleOther))

	appendUnique := func(artifacts []*Artifact) {
		for _, artifact := range artifacts {
			if seenArtifacts[artifact.Meta.ID] {
				continue
			}
			seenArtifacts[artifact.Meta.ID] = true
			stale = append(stale, artifact)
		}
	}

	appendUnique(staleDecisions)
	appendUnique(staleOther)

	return stale
}

func staleArtifactShouldSurface(ctx context.Context, store ArtifactStore, item *Artifact) bool {
	if item.Meta.Kind != KindWorkCommission {
		return true
	}

	full, err := store.Get(ctx, item.Meta.ID)
	if err != nil {
		return true
	}

	payload := map[string]any{}
	if err := json.Unmarshal([]byte(full.StructuredData), &payload); err != nil {
		return true
	}

	return !workCommissionStateTerminal(textField(payload, "state"))
}

func buildExpiredStaleItem(a *Artifact, now time.Time) StaleItem {
	item := StaleItem{
		ID:       a.Meta.ID,
		Title:    a.Meta.Title,
		Kind:     string(a.Meta.Kind),
		Category: StaleCategoryEvidenceExpired,
	}

	if a.Meta.Status == StatusRefreshDue {
		item.Reason = "manually marked refresh_due"
	}

	if a.Meta.ValidUntil != "" {
		item.ValidUntil = a.Meta.ValidUntil
		validUntil, ok := reff.ParseValidUntil(a.Meta.ValidUntil)
		if ok && validUntil.Before(now) {
			days := int(now.Sub(validUntil).Hours() / 24)
			item.DaysStale = days
			item.Reason = fmt.Sprintf(
				"expired %d day(s) ago, debt: %.1f (%s)",
				days,
				roundTenths(reff.ComputeED(validUntil, now, 1.0)),
				validUntil.Format("2006-01-02"),
			)
		}
	}

	if item.Reason == "" {
		item.Reason = "refresh_due"
	}

	return item
}

func buildDecisionStaleItem(report DriftReport) StaleItem {
	reason := formatDecisionStaleReason(report)

	return StaleItem{
		ID:         report.DecisionID,
		Title:      report.DecisionTitle,
		Kind:       string(KindDecisionRecord),
		Category:   StaleCategoryDecisionStale,
		Reason:     reason,
		DriftItems: report.Files,
	}
}

func formatDecisionStaleReason(report DriftReport) string {
	if !report.HasBaseline {
		reason := fmt.Sprintf("no baseline — %d file(s) unmonitored", len(report.Files))
		if report.LikelyImplemented {
			reason += " — git activity detected after decision date"
		}
		return reason
	}

	modified := 0
	added := 0
	missing := 0
	noBaseline := 0
	for _, file := range report.Files {
		switch file.Status {
		case DriftModified:
			modified++
		case DriftAdded:
			added++
		case DriftMissing:
			missing++
		case DriftNoBaseline:
			noBaseline++
		}
	}

	summary := fmt.Sprintf("code drift — %d modified", modified)
	if added > 0 {
		summary += fmt.Sprintf(", %d added", added)
	}
	if missing > 0 {
		summary += fmt.Sprintf(", %d missing", missing)
	}
	if noBaseline > 0 {
		summary += fmt.Sprintf(", %d unbaselined", noBaseline)
	}

	details := summarizeDriftItems(report.Files, 3)
	if details == "" {
		return summary
	}

	return summary + " — " + details
}

func summarizeDriftItems(items []DriftItem, limit int) string {
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		description := fmt.Sprintf("%s (%s)", item.Path, item.Status)
		if item.LinesChanged != "" {
			description += " " + item.LinesChanged
		}
		parts = append(parts, description)
	}

	if len(parts) <= limit {
		return strings.Join(parts, ", ")
	}

	visible := strings.Join(parts[:limit], ", ")
	hidden := len(parts) - limit
	return fmt.Sprintf("%s, +%d more", visible, hidden)
}

func computeDecisionDebtBreakdown(ctx context.Context, store ArtifactStore, decisions []*Artifact, now time.Time) ([]DecisionDebtBreakdown, float64, []StaleItem) {
	accumulators := make([]decisionDebtAccumulator, 0)
	failures := make([]StaleItem, 0)
	for _, decision := range decisions {
		evidenceItems, err := store.GetEvidenceItems(ctx, decision.Meta.ID)
		if err != nil {
			failures = append(failures, buildScanFailureItem("ED scan for "+decision.Meta.ID, err))
			continue
		}

		rawTotalED := 0.0
		expiredCount := 0
		mostOverdueDays := 0

		for _, item := range evidenceItems {
			if item.Verdict == "superseded" || item.ValidUntil == "" {
				continue
			}

			validUntil, ok := reff.ParseValidUntil(item.ValidUntil)
			if !ok {
				continue
			}

			ed := reff.ComputeED(validUntil, now, 1.0)
			if ed <= 0 {
				continue
			}

			expiredCount++
			rawTotalED += ed

			overdueDays := int(now.Sub(validUntil).Hours() / 24)
			if overdueDays > mostOverdueDays {
				mostOverdueDays = overdueDays
			}
		}

		if rawTotalED <= 0 {
			continue
		}

		accumulators = append(accumulators, decisionDebtAccumulator{
			DecisionID:      decision.Meta.ID,
			DecisionTitle:   decision.Meta.Title,
			RawTotalED:      rawTotalED,
			ExpiredEvidence: expiredCount,
			MostOverdueDays: mostOverdueDays,
		})
	}

	sort.Slice(accumulators, func(i, j int) bool {
		if accumulators[i].RawTotalED != accumulators[j].RawTotalED {
			return accumulators[i].RawTotalED > accumulators[j].RawTotalED
		}
		return accumulators[i].DecisionID < accumulators[j].DecisionID
	})

	totalED := 0.0
	breakdown := make([]DecisionDebtBreakdown, 0, len(accumulators))
	for _, item := range accumulators {
		totalED += item.RawTotalED
		breakdown = append(breakdown, DecisionDebtBreakdown{
			DecisionID:      item.DecisionID,
			DecisionTitle:   item.DecisionTitle,
			TotalED:         item.RawTotalED,
			ExpiredEvidence: item.ExpiredEvidence,
			MostOverdueDays: item.MostOverdueDays,
		})
	}

	return breakdown, totalED, failures
}

func formatEDBudgetReason(alert reff.EDBudgetAlert, breakdown []DecisionDebtBreakdown) string {
	reason := fmt.Sprintf(
		"epistemic debt budget exceeded: %s / %s (excess %s)",
		formatEDValue(alert.TotalED),
		formatEDValue(alert.Budget),
		formatEDValue(alert.Excess),
	)
	if len(breakdown) == 0 {
		return reason
	}

	parts := make([]string, 0, len(breakdown))
	for _, item := range breakdown {
		part := fmt.Sprintf("%s %s", item.DecisionID, formatEDValue(item.TotalED))
		parts = append(parts, part)
	}

	return reason + " — " + summarizeStrings(parts, 3)
}

func summarizeStrings(parts []string, limit int) string {
	if len(parts) <= limit {
		return strings.Join(parts, ", ")
	}

	visible := strings.Join(parts[:limit], ", ")
	return fmt.Sprintf("%s, +%d more", visible, len(parts)-limit)
}

func staleCategoryPriority(category StaleCategory) int {
	switch category {
	case StaleCategoryScanFailed:
		return 0
	case StaleCategoryEpistemicDebtExceeded:
		return 1
	case StaleCategoryEvidenceExpired:
		return 2
	case StaleCategoryPendingVerification:
		return 3
	case StaleCategoryREffDegraded:
		return 4
	case StaleCategoryDecisionStale:
		return 5
	default:
		return 6
	}
}

func roundTenths(value float64) float64 {
	return math.Round(value*10) / 10
}

func formatEDValue(value float64) string {
	if value < 1.0 {
		return fmt.Sprintf("%.3f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func buildScanFailureItem(stage string, err error) StaleItem {
	return StaleItem{
		ID:       "system/scan-failure/" + strings.ReplaceAll(stage, " ", "-"),
		Title:    "Refresh scan failure",
		Kind:     "System",
		Category: StaleCategoryScanFailed,
		Reason:   fmt.Sprintf("%s failed: %v", stage, err),
	}
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
func WaiveArtifact(ctx context.Context, store ArtifactStore, haftDir string, artifactRef, reason, newValidUntil, evidence string) (*Artifact, error) {
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

	writeFileQuiet(haftDir, a)
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
func ReopenDecision(ctx context.Context, store ArtifactStore, haftDir string, decisionRef, reason string) (*Artifact, *Artifact, error) {
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
	writeFileQuiet(haftDir, dec)

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
	newProb, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
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
	writeFileQuiet(haftDir, newProb)

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
func SupersedeArtifact(ctx context.Context, store ArtifactStore, haftDir string, artifactRef, newArtifactRef, reason string) (*Artifact, error) {
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

	writeFileQuiet(haftDir, a)
	return a, nil
}

// BuildDeprecateSection builds the deprecate markdown. Pure.
func BuildDeprecateSection(now time.Time, reason string) string {
	return fmt.Sprintf("\n## Deprecated (%s)\n\n**Reason:** %s\n",
		now.Format("2006-01-02"), reason)
}

// DeprecateArtifact marks an artifact as deprecated. Orchestrates effects.
func DeprecateArtifact(ctx context.Context, store ArtifactStore, haftDir string, artifactRef, reason string) (*Artifact, error) {
	a, err := store.Get(ctx, artifactRef)
	if err != nil {
		return nil, fmt.Errorf("artifact %s not found: %w", artifactRef, err)
	}

	a.Meta.Status = StatusDeprecated
	a.Body += BuildDeprecateSection(time.Now().UTC(), reason)

	if err := store.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update artifact: %w", err)
	}

	writeFileQuiet(haftDir, a)
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
func CreateRefreshReport(ctx context.Context, store ArtifactStore, haftDir string, decisionRef, action, reason, outcome string) (*Artifact, error) {
	// GenerateID uses a crypto/rand suffix since #63; no sequence lookup
	// required. seq parameter preserved for backward compat — pass 0.
	a := BuildRefreshReportArtifact(GenerateID(KindRefreshReport, 0), time.Now().UTC(), decisionRef, action, reason, outcome)

	if err := store.Create(ctx, a); err != nil {
		return nil, err
	}

	writeFileQuiet(haftDir, a)
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
