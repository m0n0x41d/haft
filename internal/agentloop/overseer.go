package agentloop

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/session"
	"github.com/m0n0x41d/haft/internal/tui"
	"github.com/m0n0x41d/haft/logger"
)

// Overseer is a background goroutine that monitors project health.
// It checks for drift, evidence decay, symbol invariants, and cycle health periodically.
// Sends findings via Bus as OverseerAlertMsg — shown in status bar.
// Does NOT call LLM, does NOT block agent, does NOT modify artifacts.
type Overseer struct {
	ArtifactStore   artifact.ArtifactStore
	Cycles          session.CycleStore
	Bus             *tui.Bus
	CoordinatorChan chan []string // alerts injected into agent context
	SessionID       string
	ProjectRoot     string
	Interval        time.Duration // check interval (default 2 minutes)
}

// Run starts the overseer loop. Blocks until ctx is cancelled.
func (o *Overseer) Run(ctx context.Context) {
	if o.Interval <= 0 {
		o.Interval = 2 * time.Minute
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

	// 1. File-level drift check
	if o.ArtifactStore != nil && o.ProjectRoot != "" {
		drifted := o.checkDrift(ctx)
		if drifted > 0 {
			alerts = append(alerts, fmt.Sprintf("⚑ %d drifted", drifted))
			logger.Info().Str("component", "overseer").Int("drifted", drifted).Msg("overseer.drift_detected")
		}
	}

	// 2. Symbol-level invariant check (tree-sitter)
	if o.ArtifactStore != nil && o.ProjectRoot != "" {
		symbolAlerts := o.checkSymbolDrift(ctx)
		alerts = append(alerts, symbolAlerts...)
	}

	// 3. Evidence decay (expiring decisions)
	if o.ArtifactStore != nil {
		expiring := o.checkExpiring(ctx)
		if expiring > 0 {
			alerts = append(alerts, fmt.Sprintf("⏳ %d stale", expiring))
			logger.Info().Str("component", "overseer").Int("expiring", expiring).Msg("overseer.evidence_decay")
		}
	}

	// 4. Cycle health
	if o.Cycles != nil {
		if health := o.checkCycleHealth(ctx); health != "" {
			alerts = append(alerts, health)
			logger.Info().Str("component", "overseer").Str("health", health).Msg("overseer.cycle_health")
		}
	}

	// Always send to TUI — even empty alerts clear the status bar
	o.Bus.Send(tui.OverseerAlertMsg{Alerts: alerts})

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

func (o *Overseer) checkDrift(ctx context.Context) int {
	reports, err := artifact.CheckDrift(ctx, o.ArtifactStore, o.ProjectRoot)
	if err != nil {
		logger.Debug().Str("component", "overseer").Err(err).Msg("overseer.drift_check_error")
		return 0
	}
	count := 0
	for _, r := range reports {
		if len(r.Files) > 0 {
			count++
			logger.Debug().Str("component", "overseer").
				Str("decision", r.DecisionID).
				Int("files", len(r.Files)).
				Msg("overseer.decision_drifted")
		}
	}
	return count
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
		for filePath, baseSnapshots := range fileSymbols {
			currentSnapshots, err := codebase.ExtractSymbolSnapshots(o.ProjectRoot, filePath)
			if err != nil {
				// File might not exist anymore — file-level drift catches this
				continue
			}

			drifts := codebase.CompareSymbolSnapshots(baseSnapshots, currentSnapshots)
			for _, drift := range drifts {
				if drift.Status == "removed" {
					alert := fmt.Sprintf("⚠ %s: %s %s deleted (%s)",
						d.Meta.ID, drift.SymbolKind, drift.SymbolName, drift.FilePath)
					alerts = append(alerts, alert)
					logger.Warn().Str("component", "overseer").
						Str("decision", d.Meta.ID).
						Str("symbol", drift.SymbolName).
						Str("kind", drift.SymbolKind).
						Str("file", drift.FilePath).
						Msg("overseer.symbol_removed")
				}
				if drift.Status == "modified" {
					logger.Info().Str("component", "overseer").
						Str("decision", d.Meta.ID).
						Str("symbol", drift.SymbolName).
						Str("kind", drift.SymbolKind).
						Str("file", drift.FilePath).
						Msg("overseer.symbol_modified")
					// Modified symbols are logged but not alerted — only removed symbols
					// are likely invariant violations. Modified could be legitimate evolution.
				}
			}
		}
	}

	return alerts
}

func (o *Overseer) checkExpiring(ctx context.Context) int {
	items, err := artifact.ScanStale(ctx, o.ArtifactStore, o.ProjectRoot)
	if err != nil {
		return 0
	}
	return len(items)
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
