package fpf

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/assurance"
	"github.com/m0n0x41d/quint-code/logger"

	"github.com/google/uuid"
)

func (t *Tools) AuditEvidence(hypothesisID, risks string) (string, error) {
	defer t.RecordWork("AuditEvidence", time.Now())

	logger.Info().Str("hypothesis_id", hypothesisID).Msg("AuditEvidence called")

	_, err := t.ManageEvidence("audit", "add", hypothesisID, "audit_report", risks, "pass", "L2", "auditor", "")
	if err != nil {
		logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("AuditEvidence: failed to add evidence")
		return "", err
	}

	logger.Info().Str("hypothesis_id", hypothesisID).Msg("AuditEvidence: completed")
	return "Audit recorded for " + hypothesisID, nil
}

func (t *Tools) UnifiedAudit(holonID, risks string) (string, error) {
	defer t.RecordWork("UnifiedAudit", time.Now())

	logger.Info().
		Str("holon_id", holonID).
		Bool("has_risks", risks != "").
		Msg("UnifiedAudit called")

	if t.DB == nil {
		logger.Error().Msg("UnifiedAudit: database not initialized")
		return "", ErrDatabaseNotInitialized
	}

	if holonID == "" {
		return "", fmt.Errorf("holon_id is required")
	}

	var result strings.Builder

	calc := assurance.New(t.DB.GetRawDB())
	report, err := calc.CalculateReliability(context.Background(), holonID)
	if err != nil {
		return "", fmt.Errorf("failed to calculate reliability: %w", err)
	}

	result.WriteString(fmt.Sprintf("# Audit Report: %s\n\n", holonID))
	result.WriteString(fmt.Sprintf("**R_eff: %.2f**\n", report.FinalScore))
	result.WriteString(fmt.Sprintf("- Self Score: %.2f\n", report.SelfScore))
	if report.WeakestLink != "" {
		result.WriteString(fmt.Sprintf("- Weakest Link: %s\n", report.WeakestLink))
	}
	if report.DecayPenalty > 0 {
		result.WriteString(fmt.Sprintf("- Decay Penalty: %.2f\n", report.DecayPenalty))
	}
	if len(report.Factors) > 0 {
		result.WriteString("\n**Factors:**\n")
		for _, f := range report.Factors {
			result.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	result.WriteString("\n## Assurance Tree\n\n```\n")
	tree, err := t.buildAuditTree(holonID, 0, calc)
	if err != nil {
		result.WriteString(fmt.Sprintf("(Unable to build tree: %v)\n", err))
	} else {
		result.WriteString(tree)
	}
	result.WriteString("```\n")

	if risks != "" {
		_, err := t.ManageEvidence("audit", "add", holonID, "audit_report", risks, "pass", "L2", "auditor", "")
		if err != nil {
			result.WriteString(fmt.Sprintf("\n⚠️ Failed to record audit: %v\n", err))
		} else {
			result.WriteString("\n✓ Audit evidence recorded\n")
		}
	}

	return result.String(), nil
}

func (t *Tools) ManageEvidence(operation, action, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil string) (string, error) {
	defer t.RecordWork("ManageEvidence", time.Now())

	ctx := context.Background()

	if action == "check" {
		if t.DB == nil {
			return "", ErrDatabaseNotInitialized
		}
		if targetID == "all" {
			return "Global evidence audit not implemented yet. Please specify a target_id.", nil
		}
		ev, err := t.DB.GetEvidence(ctx, targetID)
		if err != nil {
			return "", err
		}
		var report string
		for _, e := range ev {
			report += fmt.Sprintf("- [%s] %s (L:%s, Ref:%s): %s\n", e.Verdict, e.Type, e.AssuranceLevel.String, e.CarrierRef.String, e.Content)
		}
		if report == "" {
			return "No evidence found for " + targetID, nil
		}
		return report, nil
	}

	shouldPromote := false

	normalizedVerdict := strings.ToLower(verdict)

	switch normalizedVerdict {
	case "pass":
		switch operation {
		case "verification":
			if assuranceLevel == "L1" || assuranceLevel == "L2" {
				shouldPromote = true
			}
		case "validation":
			if assuranceLevel == "L2" {
				shouldPromote = true
			}
		}
	}

	var moveErr error
	if (normalizedVerdict == "pass") && shouldPromote {
		switch operation {
		case "verification":
			moveErr = t.MoveHypothesis(targetID, "L0", "L1")
		case "validation":
			holon, err := t.DB.GetHolon(context.Background(), targetID)
			if err == nil && holon.Layer == "L0" {
				return "", fmt.Errorf("hypothesis %s is still in L0: run /q2-verify to promote it to L1 before testing", targetID)
			}
			if err == nil && holon.Layer == "L1" {
				moveErr = t.MoveHypothesis(targetID, "L1", "L2")
			}
		}
	} else if normalizedVerdict == "fail" || normalizedVerdict == "refine" {
		switch operation {
		case "verification":
			moveErr = t.MoveHypothesis(targetID, "L0", "invalid")
		case "validation":
			holon, err := t.DB.GetHolon(context.Background(), targetID)
			if err == nil && holon.Layer == "L1" {
				moveErr = t.MoveHypothesis(targetID, "L1", "invalid")
			}
		}
	}

	if moveErr != nil {
		return "", fmt.Errorf("failed to move hypothesis: %v", moveErr)
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s-%s.md", date, evidenceType, targetID)
	path := filepath.Join(t.GetFPFDir(), "evidence", filename)

	body := fmt.Sprintf("\n%s", content)
	fields := map[string]string{
		"id":              filename,
		"type":            evidenceType,
		"target":          targetID,
		"verdict":         normalizedVerdict,
		"assurance_level": assuranceLevel,
		"carrier_ref":     carrierRef,
		"valid_until":     validUntil,
		"date":            date,
	}

	if err := WriteWithHash(path, fields, body); err != nil {
		return "", err
	}

	if t.DB != nil {
		carrierHash := t.hashCarrierFiles(carrierRef)

		carrierCommit := ""
		if currentHead, err := t.getCurrentHead(); err == nil {
			carrierCommit = currentHead
		}

		if err := t.DB.AddEvidence(ctx, filename, targetID, evidenceType, content, normalizedVerdict, assuranceLevel, carrierRef, carrierHash, carrierCommit, validUntil); err != nil {
			logger.Warn().Err(err).Msg("failed to add evidence to DB")
		}
		if err := t.DB.Link(ctx, filename, targetID, "verifiedBy"); err != nil {
			logger.Warn().Err(err).Msg("failed to link evidence in DB")
		}

		if normalizedVerdict == "pass" {
			if err := t.DB.ClearHolonReverification(ctx, targetID); err != nil {
				logger.Warn().Err(err).Msg("failed to clear reverification flag")
			}
			t.recalculateHolonR(ctx, targetID)
		}
	}

	if !shouldPromote && verdict == "PASS" {
		return path + " (Evidence recorded, but Assurance Level insufficient for promotion)", nil
	}
	return path, nil
}

func (t *Tools) CheckDecay(deprecate, waiveID, waiveUntil, waiveRationale string) (string, error) {
	defer t.RecordWork("CheckDecay", time.Now())
	if t.DB == nil {
		return "", ErrDatabaseNotInitialized
	}

	switch {
	case deprecate != "":
		return t.deprecateHolon(deprecate)
	case waiveID != "":
		if waiveUntil == "" || waiveRationale == "" {
			return "", fmt.Errorf("waive requires both --until and --rationale parameters")
		}
		return t.createWaiver(waiveID, waiveUntil, waiveRationale)
	default:
		return t.generateFreshnessReport()
	}
}

func (t *Tools) deprecateHolon(holonID string) (string, error) {
	ctx := context.Background()
	holon, err := t.DB.GetHolon(ctx, holonID)
	if err != nil {
		return "", fmt.Errorf("holon not found: %s", holonID)
	}

	var newLayer string
	switch holon.Layer {
	case "L2":
		newLayer = "L1"
	case "L1":
		newLayer = "L0"
	default:
		return "", fmt.Errorf("cannot deprecate %s from %s (only L2 and L1 can be deprecated)", holonID, holon.Layer)
	}

	if err := t.MoveHypothesis(holonID, holon.Layer, newLayer); err != nil {
		return "", err
	}

	t.AuditLog("quint_check_decay", "deprecate", "user", holonID, "SUCCESS",
		map[string]string{"from": holon.Layer, "to": newLayer}, "Evidence expired, holon deprecated")

	return fmt.Sprintf("Deprecated: %s %s → %s\n\nThis decision now requires re-evaluation.\nNext step: Run /q1-hypothesize to explore alternatives.", holonID, holon.Layer, newLayer), nil
}

func (t *Tools) createWaiver(evidenceID, until, rationale string) (string, error) {
	ctx := context.Background()

	_, err := t.DB.GetEvidenceByID(ctx, evidenceID)
	if err != nil {
		return "", fmt.Errorf("evidence not found: %s", evidenceID)
	}

	untilTime, err := time.Parse("2006-01-02", until)
	if err != nil {
		untilTime, err = time.Parse(time.RFC3339, until)
		if err != nil {
			return "", fmt.Errorf("invalid date format: %s (use YYYY-MM-DD or RFC3339)", until)
		}
	}

	if untilTime.Before(time.Now()) {
		return "", fmt.Errorf("waive_until must be a future date")
	}

	id := uuid.New().String()
	if err := t.DB.CreateWaiver(ctx, id, evidenceID, "user", untilTime, rationale); err != nil {
		return "", fmt.Errorf("failed to create waiver: %v", err)
	}

	t.AuditLog("quint_check_decay", "waive", "user", evidenceID, "SUCCESS",
		map[string]string{"until": until, "rationale": rationale}, "")

	return fmt.Sprintf(`Waiver recorded:
- Evidence: %s
- Waived until: %s
- Rationale: %s

⚠️ This evidence returns to EXPIRED status after %s.
   Set a reminder to run /q3-validate before then.`, evidenceID, until, rationale, until), nil
}

func (t *Tools) generateFreshnessReport() (string, error) {
	ctx := context.Background()

	staleRows, err := t.DB.GetStaleEvidence(ctx)
	if err != nil {
		return "", err
	}

	type evidenceInfo struct {
		ID          string
		Type        string
		DaysOverdue int
	}

	staleHolons := make(map[string][]evidenceInfo)
	holonTitles := make(map[string]string)
	holonLayers := make(map[string]string)

	for _, row := range staleRows {
		holonTitles[row.HolonID] = row.Title
		holonLayers[row.HolonID] = row.Layer
		staleHolons[row.HolonID] = append(staleHolons[row.HolonID], evidenceInfo{
			ID:          row.EvidenceID,
			Type:        row.EvidenceType,
			DaysOverdue: row.DaysOverdue,
		})
	}

	waiverRows, err := t.DB.GetActiveWaivers(ctx)
	if err != nil {
		return "", err
	}

	type waiverInfo struct {
		EvidenceID      string
		HolonID         string
		HolonTitle      string
		WaivedUntil     string
		WaivedBy        string
		Rationale       string
		DaysUntilExpiry int
	}

	var activeWaivers []waiverInfo
	for _, row := range waiverRows {
		activeWaivers = append(activeWaivers, waiverInfo{
			EvidenceID:      row.EvidenceID,
			HolonID:         row.HolonID,
			HolonTitle:      row.HolonTitle,
			WaivedUntil:     row.WaivedUntil,
			WaivedBy:        row.WaivedBy,
			Rationale:       row.Rationale,
			DaysUntilExpiry: row.DaysUntilExpiry,
		})
	}

	var result strings.Builder
	result.WriteString("## Evidence Freshness Report\n\n")

	if len(staleHolons) == 0 {
		result.WriteString("### All holons FRESH ✓\n\nNo expired evidence found.\n")
	} else {
		result.WriteString(fmt.Sprintf("### STALE (%d holons require action)\n\n", len(staleHolons)))

		for holonID, evidenceItems := range staleHolons {
			result.WriteString(fmt.Sprintf("#### %s (%s)\n", holonTitles[holonID], holonLayers[holonID]))
			result.WriteString("| ID | Type | Status | Details |\n")
			result.WriteString("|-----|------|--------|--------|\n")
			for _, item := range evidenceItems {
				result.WriteString(fmt.Sprintf("| %s | %s | EXPIRED | %d days overdue |\n", item.ID, item.Type, item.DaysOverdue))
			}
			result.WriteString("\nActions:\n")
			result.WriteString(fmt.Sprintf("  → /q3-validate %s (refresh evidence)\n", holonID))
			result.WriteString("  → Deprecate: downgrade holon if decision needs rethinking\n")
			result.WriteString("  → Waive: accept risk temporarily with documented rationale\n\n")
		}
	}

	if len(activeWaivers) > 0 {
		result.WriteString("---\n\n### WAIVED (temporary risk acceptance)\n\n")
		result.WriteString("| Holon | Evidence | Waived Until | By | Rationale |\n")
		result.WriteString("|-------|----------|--------------|----|-----------|\n")
		for _, w := range activeWaivers {
			waivedUntilShort := w.WaivedUntil
			if len(waivedUntilShort) > 10 {
				waivedUntilShort = waivedUntilShort[:10]
			}
			result.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", w.HolonTitle, w.EvidenceID, waivedUntilShort, w.WaivedBy, w.Rationale))
		}
		for _, w := range activeWaivers {
			if w.DaysUntilExpiry <= 30 {
				result.WriteString(fmt.Sprintf("\n⚠️ Waiver for %s expires in %d days\n", w.EvidenceID, w.DaysUntilExpiry))
			}
		}
	}

	return result.String(), nil
}
