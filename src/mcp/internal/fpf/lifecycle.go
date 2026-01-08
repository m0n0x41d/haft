package fpf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/assurance"
	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/logger"
)

func (t *Tools) InitProject() error {
	dirs := []string{
		"evidence",
		"decisions",
		"sessions",
		"agents",
	}

	for _, d := range dirs {
		path := filepath.Join(t.GetFPFDir(), d)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(path, ".gitkeep"), []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to write .gitkeep file: %v", err)
		}
	}

	if t.DB == nil {
		dbPath := filepath.Join(t.GetFPFDir(), "quint.db")
		database, err := db.NewStore(dbPath)
		if err != nil {
			fmt.Printf("Warning: Failed to init DB: %v\n", err)
		} else {
			t.DB = database
		}
	}

	return nil
}

func (t *Tools) RecordContext(vocabulary, invariants string) (string, error) {
	vocabFormatted := formatVocabulary(vocabulary)
	invFormatted := formatInvariants(invariants)

	content := fmt.Sprintf("# Bounded Context\n\n## Vocabulary\n\n%s\n\n## Invariants\n\n%s\n", vocabFormatted, invFormatted)
	path := filepath.Join(t.GetFPFDir(), "context.md")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func formatVocabulary(vocab string) string {
	termPattern := regexp.MustCompile(`([A-Z][a-zA-Z0-9_\[\],<>]+):\s*`)
	matches := termPattern.FindAllStringSubmatchIndex(vocab, -1)

	if len(matches) == 0 {
		return vocab
	}

	var lines []string
	for i, match := range matches {
		termStart := match[2]
		termEnd := match[3]
		defStart := match[1]

		var defEnd int
		if i+1 < len(matches) {
			defEnd = matches[i+1][0]
		} else {
			defEnd = len(vocab)
		}

		term := vocab[termStart:termEnd]
		def := strings.TrimSpace(vocab[defStart:defEnd])

		lines = append(lines, fmt.Sprintf("- **%s**: %s", term, def))
	}

	return strings.Join(lines, "\n")
}

func formatInvariants(inv string) string {
	numPattern := regexp.MustCompile(`(\d+)\.\s+`)
	matches := numPattern.FindAllStringSubmatchIndex(inv, -1)

	if len(matches) == 0 {
		return inv
	}

	var lines []string
	for i, match := range matches {
		numStart := match[2]
		numEnd := match[3]
		contentStart := match[1]

		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(inv)
		}

		num := inv[numStart:numEnd]
		content := strings.TrimSpace(inv[contentStart:contentEnd])

		lines = append(lines, fmt.Sprintf("%s. %s", num, content))
	}

	return strings.Join(lines, "\n")
}

func (t *Tools) RunDecay() error {
	defer t.RecordWork("RunDecay", time.Now())
	if t.DB == nil {
		return fmt.Errorf("DB not initialized")
	}

	ctx := context.Background()
	ids, err := t.DB.ListAllHolonIDs(ctx)
	if err != nil {
		return err
	}

	calc := assurance.New(t.DB.GetRawDB())
	updatedCount := 0

	for _, id := range ids {
		_, err := calc.CalculateReliability(ctx, id)
		if err != nil {
			fmt.Printf("Error calculating R for %s: %v\n", id, err)
			continue
		}
		updatedCount++
	}

	fmt.Printf("Decay update complete. Processed %d holons.\n", updatedCount)
	return nil
}

func (t *Tools) Actualize() (string, error) {
	var report strings.Builder
	fpfDir := filepath.Join(t.RootDir, ".fpf")
	quintDir := t.GetFPFDir()

	if _, err := os.Stat(fpfDir); err == nil {
		report.WriteString("MIGRATION: Found legacy .fpf directory.\n")

		if _, err := os.Stat(quintDir); err == nil {
			return report.String(), fmt.Errorf("migration conflict: both .fpf and .quint exist. Please resolve manually")
		}

		report.WriteString("MIGRATION: Renaming .fpf -> .quint\n")
		if err := os.Rename(fpfDir, quintDir); err != nil {
			return report.String(), fmt.Errorf("failed to rename .fpf: %w", err)
		}
		report.WriteString("MIGRATION: Success.\n")
	}

	legacyDB := filepath.Join(quintDir, "fpf.db")
	newDB := filepath.Join(quintDir, "quint.db")

	if _, err := os.Stat(legacyDB); err == nil {
		report.WriteString("MIGRATION: Found legacy fpf.db.\n")
		if err := os.Rename(legacyDB, newDB); err != nil {
			return report.String(), fmt.Errorf("failed to rename fpf.db: %w", err)
		}
		report.WriteString("MIGRATION: Renamed to quint.db.\n")
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = t.RootDir
	output, err := cmd.Output()
	if err == nil {
		currentCommit := strings.TrimSpace(string(output))
		lastCommit := t.FSM.State.LastCommit

		if lastCommit == "" {
			report.WriteString(fmt.Sprintf("RECONCILIATION: Initializing baseline commit to %s\n", currentCommit))
			t.FSM.State.LastCommit = currentCommit
			if err := t.FSM.SaveState("default"); err != nil {
				report.WriteString(fmt.Sprintf("Warning: Failed to save state: %v\n", err))
			}
		} else if currentCommit != lastCommit {
			report.WriteString(fmt.Sprintf("RECONCILIATION: Detected changes since %s\n", lastCommit))
			diffCmd := exec.Command("git", "diff", "--name-status", lastCommit, "HEAD")
			diffCmd.Dir = t.RootDir
			diffOutput, err := diffCmd.Output()
			if err == nil {
				report.WriteString("Changed files:\n")
				report.WriteString(string(diffOutput))
			} else {
				report.WriteString(fmt.Sprintf("Warning: Failed to get diff: %v\n", err))
			}

			t.FSM.State.LastCommit = currentCommit
			if err := t.FSM.SaveState("default"); err != nil {
				report.WriteString(fmt.Sprintf("Warning: Failed to save state: %v\n", err))
			}
		} else {
			report.WriteString("RECONCILIATION: No changes detected (Clean).\n")
		}
	} else {
		report.WriteString("RECONCILIATION: Not a git repository or git error.\n")
	}

	return report.String(), nil
}

func (t *Tools) ResetCycle(reason, contextID string, abandonAll bool) (string, error) {
	defer t.RecordWork("ResetCycle", time.Now())

	logger.Info().
		Str("reason", reason).
		Str("context_id", contextID).
		Bool("abandon_all", abandonAll).
		Msg("ResetCycle called")

	if reason == "" {
		reason = "user requested reset"
	}

	ctx := context.Background()
	var sb strings.Builder

	if contextID != "" {
		if t.DB == nil {
			return "", fmt.Errorf("database not initialized")
		}
		if err := t.DB.AbandonContext(ctx, contextID); err != nil {
			return "", fmt.Errorf("failed to abandon context %s: %w", contextID, err)
		}
		t.AuditLog("quint_reset", "abandon_context", "agent", contextID, "SUCCESS",
			map[string]string{"reason": reason}, "")
		sb.WriteString(fmt.Sprintf("Abandoned context: %s\n", contextID))
		sb.WriteString(fmt.Sprintf("Reason: %s\n", reason))
		return sb.String(), nil
	}

	if abandonAll {
		if t.DB == nil {
			return "", fmt.Errorf("database not initialized")
		}
		contexts, err := t.GetActiveDecisionContexts(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get active contexts: %w", err)
		}
		if len(contexts) == 0 {
			return "No active contexts to abandon.\n", nil
		}
		var abandoned []string
		for _, c := range contexts {
			if err := t.DB.AbandonContext(ctx, c.ID); err != nil {
				logger.Warn().Err(err).Str("context_id", c.ID).Msg("failed to abandon context")
				continue
			}
			abandoned = append(abandoned, c.ID)
		}
		t.AuditLog("quint_reset", "abandon_all_contexts", "agent", "", "SUCCESS",
			map[string]string{"reason": reason, "count": fmt.Sprintf("%d", len(abandoned))}, "")
		sb.WriteString(fmt.Sprintf("Abandoned %d contexts:\n", len(abandoned)))
		for _, id := range abandoned {
			sb.WriteString(fmt.Sprintf("  - %s\n", id))
		}
		sb.WriteString(fmt.Sprintf("Reason: %s\n", reason))
		return sb.String(), nil
	}

	currentStage := StageEmpty
	if activeContexts, err := t.GetActiveDecisionContexts(ctx); err == nil && len(activeContexts) > 0 {
		currentStage = t.getMostAdvancedStage(activeContexts)
	}

	sb.WriteString(fmt.Sprintf("Stage at reset: %s\n", currentStage))
	sb.WriteString(fmt.Sprintf("L0: %d, L1: %d, L2: %d, DRR: %d\n",
		t.countHolons("L0"), t.countHolons("L1"), t.countHolons("L2"), t.countDRRs()))

	if t.DB != nil {
		openDecisions, err := t.GetOpenDecisions(ctx)
		if err == nil && len(openDecisions) > 0 {
			sb.WriteString(fmt.Sprintf("Open decisions: %d\n", len(openDecisions)))
			for _, d := range openDecisions {
				sb.WriteString(fmt.Sprintf("  - %s\n", d.ID))
			}
		}
	}

	t.AuditLog("quint_reset", "cycle_reset", "agent", "", "SUCCESS",
		map[string]string{"reason": reason, "from_stage": string(currentStage)},
		sb.String())

	return fmt.Sprintf("Cycle reset. Session ended.\nPrevious stage: %s\nReason: %s\n\n%s",
		currentStage, reason, sb.String()), nil
}

func (t *Tools) Compact(mode string, retentionDays int64) (string, error) {
	defer t.RecordWork("Compact", time.Now())

	if t.DB == nil {
		return "", fmt.Errorf("database not available")
	}

	if mode == "" {
		mode = "preview"
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}

	ctx := context.Background()
	result := CompactResult{
		Mode:          mode,
		RetentionDays: retentionDays,
	}

	count, err := t.DB.CountCompactableHolons(ctx, retentionDays)
	if err != nil {
		return "", fmt.Errorf("failed to count compactable holons: %w", err)
	}
	result.EligibleCount = count

	if count == 0 {
		return fmt.Sprintf("No holons eligible for compaction (retention: %d days).\n\nAll archived holons are either:\n- Less than %d days old, or\n- Already compacted", retentionDays, retentionDays), nil
	}

	holons, err := t.DB.GetArchivedHolonsForCompaction(ctx, retentionDays)
	if err != nil {
		return "", fmt.Errorf("failed to get compactable holons: %w", err)
	}

	var sb strings.Builder

	if mode == "preview" {
		sb.WriteString(fmt.Sprintf("## Compaction Preview (retention: %d days)\n\n", retentionDays))
		sb.WriteString(fmt.Sprintf("**%d holons eligible for compaction:**\n\n", count))
		sb.WriteString("| ID | Title | Layer | Decision | Outcome | Resolved |\n")
		sb.WriteString("|-----|-------|-------|----------|---------|----------|\n")

		for _, h := range holons {
			resolvedAt := "unknown"
			if h.ResolvedAt != nil {
				if t, ok := h.ResolvedAt.(time.Time); ok {
					resolvedAt = t.Format("2006-01-02")
				} else if s, ok := h.ResolvedAt.(string); ok && len(s) >= 10 {
					resolvedAt = s[:10]
				}
			}
			outcome := h.DecisionOutcome
			if outcome == "selects" {
				outcome = "SELECTED"
			} else if outcome == "rejects" {
				outcome = "REJECTED"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				truncateString(h.ID, 12),
				truncateString(h.Title, 30),
				h.Layer,
				truncateString(h.DecisionTitle, 20),
				outcome,
				resolvedAt))
		}

		sb.WriteString("\n**Compaction removes:**\n")
		sb.WriteString("- Evidence records\n")
		sb.WriteString("- Characteristics\n")
		sb.WriteString("- Waivers\n")
		sb.WriteString("- Detailed content (replaced with '[COMPACTED]')\n")
		sb.WriteString("\n**Preserved:**\n")
		sb.WriteString("- Holon ID, title, type, kind, layer\n")
		sb.WriteString("- Relations (decision links)\n")
		sb.WriteString("- Audit log entries\n")
		sb.WriteString("\nTo execute: `quint_compact(mode=\"execute\", retention_days=")
		sb.WriteString(fmt.Sprintf("%d)`\n", retentionDays))

	} else if mode == "execute" {
		sb.WriteString(fmt.Sprintf("## Compaction Executed (retention: %d days)\n\n", retentionDays))

		for _, h := range holons {
			err := t.DB.CompactHolon(ctx, h.ID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", h.ID, err))
				continue
			}
			result.CompactedHolons = append(result.CompactedHolons, h.ID)
			result.CompactedCount++
		}

		sb.WriteString(fmt.Sprintf("**Compacted %d holons**\n\n", result.CompactedCount))

		if len(result.Errors) > 0 {
			sb.WriteString("**Errors:**\n")
			for _, e := range result.Errors {
				sb.WriteString(fmt.Sprintf("- %s\n", e))
			}
			sb.WriteString("\n")
		}

		t.AuditLog("quint_compact", "compaction",
			string(RoleMaintainer), "",
			"SUCCESS",
			map[string]any{
				"retention_days": retentionDays,
				"compacted":      result.CompactedCount,
				"errors":         len(result.Errors),
			},
			fmt.Sprintf("Compacted %d holons", result.CompactedCount))

		sb.WriteString("Compaction complete. Holon metadata and decision links preserved.\n")

	} else {
		return "", fmt.Errorf("invalid mode: %s (use 'preview' or 'execute')", mode)
	}

	return sb.String(), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
