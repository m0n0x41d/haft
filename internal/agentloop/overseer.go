package agentloop

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/protocol"
	"github.com/m0n0x41d/haft/internal/session"
	"github.com/m0n0x41d/haft/logger"
)

// Overseer is a background goroutine that monitors project health.
// It checks for drift, evidence decay, symbol invariants, and cycle health periodically.
// Sends findings via Bus as OverseerAlertMsg — shown in status bar.
// Does NOT call LLM, does NOT block agent, does NOT modify artifacts.
type Overseer struct {
	ArtifactStore   artifact.ArtifactStore
	Cycles          session.CycleStore
	Bus             *protocol.Bus
	CoordinatorChan chan []string // alerts injected into agent context
	SessionID       string
	ProjectRoot     string
	Interval        time.Duration // check interval (default 5 minutes)
}

// Run starts the overseer loop. Blocks until ctx is cancelled.
func (o *Overseer) Run(ctx context.Context) {
	if o.Interval <= 0 {
		o.Interval = 5 * time.Minute
	}

	// Initial check after 10 seconds (let agent start first)
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			o.check(ctx)
			timer.Reset(o.Interval)
		}
	}
}

func (o *Overseer) check(ctx context.Context) {
	logger.Debug().Str("component", "overseer").Msg("overseer.check_start")

	var alerts []string
	var findings []protocol.OverseerFinding

	// 1. Refresh scan: expiry, drift, R_eff degradation, ED budget.
	if o.ArtifactStore != nil {
		staleAlerts, staleFindings := o.checkStale(ctx)
		alerts = append(alerts, staleAlerts...)
		findings = append(findings, staleFindings...)
	}

	// 2. Symbol-level invariant check (tree-sitter)
	if o.ArtifactStore != nil && o.ProjectRoot != "" {
		symbolAlerts := o.checkSymbolDrift(ctx)
		alerts = append(alerts, symbolAlerts...)
	}

	// 3. Cycle health
	if o.Cycles != nil {
		if health := o.checkCycleHealth(ctx); health != "" {
			alerts = append(alerts, health)
			logger.Info().Str("component", "overseer").Str("health", health).Msg("overseer.cycle_health")
		}
	}

	// Always send to TUI — even empty alerts clear the status bar
	if o.Bus != nil {
		_ = o.Bus.SendOverseerAlert(protocol.OverseerAlert{
			Alerts:   alerts,
			Findings: findings,
		})
	}

	if len(alerts) > 0 {
		// Send to coordinator (system message injection)
		if o.CoordinatorChan != nil {
			select {
			case o.CoordinatorChan <- alerts:
			default:
			}
		}

		logger.Debug().Str("component", "overseer").
			Int("alerts", len(alerts)).
			Msg("overseer.check_complete")
	}
}

func (o *Overseer) checkStale(ctx context.Context) ([]string, []protocol.OverseerFinding) {
	items, err := artifact.ScanStale(ctx, o.ArtifactStore, o.ProjectRoot)
	if err != nil {
		logger.Debug().Str("component", "overseer").Err(err).Msg("overseer.scan_stale_error")
		return nil, nil
	}

	alertCounts := make(map[string]int)
	findings := make([]protocol.OverseerFinding, 0, len(items))
	for _, item := range items {
		findingType := overseerFindingType(item.Category)
		alertCounts[findingType]++
		findings = append(findings, buildOverseerFinding(item, findingType))
	}

	alerts := make([]string, 0, len(alertCounts))
	if alertCounts["decision_stale"] > 0 {
		alerts = append(alerts, fmt.Sprintf("⚑ %d drifted", alertCounts["decision_stale"]))
		logger.Info().Str("component", "overseer").Int("drifted", alertCounts["decision_stale"]).Msg("overseer.drift_detected")
	}
	if alertCounts["evidence_expired"] > 0 {
		alerts = append(alerts, fmt.Sprintf("⏳ %d stale", alertCounts["evidence_expired"]))
		logger.Info().Str("component", "overseer").Int("stale", alertCounts["evidence_expired"]).Msg("overseer.evidence_decay")
	}
	if alertCounts["reff_degraded"] > 0 {
		alerts = append(alerts, fmt.Sprintf("⚠ %d weak evidence", alertCounts["reff_degraded"]))
	}
	if alertCounts["ed_budget_exceeded"] > 0 {
		for _, finding := range findings {
			if finding.Type == "ed_budget_exceeded" {
				alerts = append(alerts, fmt.Sprintf("⚠ ED %.1f/%.1f", finding.TotalED, finding.Budget))
				break
			}
		}
	}

	return alerts, findings
}

// checkSymbolDrift uses tree-sitter to check if symbols from decision baselines
// still exist and haven't been modified. Reports removed/modified symbols as alerts.
// This catches invariant violations at function/type granularity — not just file hash.
func (o *Overseer) checkSymbolDrift(ctx context.Context) []string {
	decisions, err := o.ArtifactStore.ListByKind(ctx, artifact.KindDecisionRecord, 100)
	if err != nil {
		return nil
	}

	var alerts []string

	for _, d := range decisions {
		if d.Meta.Status != artifact.StatusActive {
			continue
		}

		baseline, err := o.ArtifactStore.GetAffectedSymbols(ctx, d.Meta.ID)
		if err != nil || len(baseline) == 0 {
			continue
		}

		// Group baseline symbols by file
		fileSymbols := make(map[string][]codebase.SymbolSnapshot)
		for _, sym := range baseline {
			fileSymbols[sym.FilePath] = append(fileSymbols[sym.FilePath], codebase.SymbolSnapshot{
				FilePath:   sym.FilePath,
				SymbolName: sym.SymbolName,
				SymbolKind: sym.SymbolKind,
				Line:       sym.Line,
				EndLine:    sym.EndLine,
				Hash:       sym.Hash,
			})
		}

		// Extract current symbols and compare per file
		removedCount := 0
		for filePath, baseSnapshots := range fileSymbols {
			currentSnapshots, err := codebase.ExtractSymbolSnapshots(o.ProjectRoot, filePath)
			if err != nil {
				continue
			}

			drifts := codebase.CompareSymbolSnapshots(baseSnapshots, currentSnapshots)
			for _, drift := range drifts {
				if drift.Status == "removed" {
					removedCount++
					logger.Warn().Str("component", "overseer").
						Str("decision", d.Meta.ID).
						Str("symbol", drift.SymbolName).
						Str("file", drift.FilePath).
						Msg("overseer.symbol_removed")
				}
			}
		}
		if removedCount > 0 {
			alerts = append(alerts, fmt.Sprintf("⚠ %d symbols removed", removedCount))
		}
	}

	return alerts
}

func (o *Overseer) checkCycleHealth(ctx context.Context) string {
	cycle, err := o.Cycles.GetActiveCycle(ctx, o.SessionID)
	if err != nil || cycle == nil {
		return ""
	}

	// Check if cycle has been open too long without progress
	age := time.Since(cycle.CreatedAt)
	if age > 30*time.Minute && cycle.DecisionRef == "" {
		return "⚠ cycle open 30m+"
	}

	return ""
}

func buildOverseerFinding(item artifact.StaleItem, findingType string) protocol.OverseerFinding {
	finding := protocol.OverseerFinding{
		Type:       findingType,
		Category:   string(item.Category),
		ArtifactID: item.ID,
		Title:      item.Title,
		Kind:       item.Kind,
		Summary:    item.Reason,
		Reason:     item.Reason,
		DaysStale:  item.DaysStale,
		REff:       item.REff,
		TotalED:    item.TotalED,
		Budget:     item.DebtBudget,
		Excess:     item.DebtExcess,
	}

	if len(item.DriftItems) > 0 {
		finding.DriftItems = make([]protocol.OverseerDriftItem, 0, len(item.DriftItems))
		for _, driftItem := range item.DriftItems {
			finding.DriftItems = append(finding.DriftItems, protocol.OverseerDriftItem{
				Path:         driftItem.Path,
				Status:       string(driftItem.Status),
				LinesChanged: driftItem.LinesChanged,
			})
		}
	}

	if len(item.DecisionDebt) > 0 {
		finding.DebtBreakdown = make([]protocol.OverseerDebtBreakdown, 0, len(item.DecisionDebt))
		for _, debt := range item.DecisionDebt {
			finding.DebtBreakdown = append(finding.DebtBreakdown, protocol.OverseerDebtBreakdown{
				DecisionID:      debt.DecisionID,
				DecisionTitle:   debt.DecisionTitle,
				TotalED:         debt.TotalED,
				ExpiredEvidence: debt.ExpiredEvidence,
				MostOverdueDays: debt.MostOverdueDays,
			})
		}
	}

	return finding
}

func overseerFindingType(category artifact.StaleCategory) string {
	switch category {
	case artifact.StaleCategoryEvidenceExpired:
		return "evidence_expired"
	case artifact.StaleCategoryREffDegraded:
		return "reff_degraded"
	case artifact.StaleCategoryEpistemicDebtExceeded:
		return "ed_budget_exceeded"
	case artifact.StaleCategoryDecisionStale:
		return "decision_stale"
	default:
		return "decision_stale"
	}
}
