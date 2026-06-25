package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
)

func overlayLiveDriftStatusSignal(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	summary overseer.StatusSummary,
) overseer.StatusSummary {
	report, ok := liveDriftEventReport(ctx, store, projectRoot)
	if !ok {
		return summary
	}
	signal, hasSignal := liveDriftStatusSignal(report)

	signals := nonDriftStatusSignals(summary.Signals)
	if hasSignal {
		signals = append([]overseer.StatusSignal{signal}, signals...)
	}
	summary.Signals = signals
	summary.HasSignals = len(summary.Signals) > 0 || len(summary.ExecutedActions) > 0
	return summary
}

func liveDriftEventReport(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) (artifact.DriftEventReport, bool) {
	if store == nil || strings.TrimSpace(projectRoot) == "" {
		return artifact.DriftEventReport{}, false
	}
	reports, err := artifact.CheckDrift(ctx, store, projectRoot)
	if err != nil {
		return artifact.DriftEventReport{}, false
	}
	ledger, err := readDriftEventResolutionLedger(driftEventResolutionLedgerPath(projectRoot, ""))
	if err != nil {
		return artifact.DriftEventReport{}, false
	}
	report := buildDriftEventReportWithResolutionLedger(reports, ledger, timeNow())
	return report, true
}

func liveDriftStatusSignal(report artifact.DriftEventReport) (overseer.StatusSignal, bool) {
	openEvents := artifact.OpenDriftEvents(report.Events)
	partitions := artifact.PartitionDriftEvents(openEvents)
	materialSummary := artifact.SummarizeDriftEventGroup(partitions.Material)
	bindingSummary := artifact.SummarizeDriftEventGroup(partitions.NeedsBindingResolution)

	if materialSummary.UniqueEvents == 0 && bindingSummary.UniqueEvents == 0 {
		return overseer.StatusSignal{}, false
	}

	title := liveDriftStatusSignalTitle(materialSummary, bindingSummary)
	detail := fmt.Sprintf(
		"derived from current DriftEventReport; %d audit-only/resolved event(s) stay in drill-downs. Inspect exact items with `%s`, `haft overseer judgment --json --limit 20`, or `haft overseer drain --dry-run --json`",
		len(partitions.AuditOnly),
		artifact.StatusCompactDriftEventsCommand,
	)
	return overseer.StatusSignal{
		Severity: "high",
		Source:   "drift",
		Title:    title,
		Detail:   detail,
	}, true
}

func liveDriftStatusSignalTitle(
	material artifact.DriftEventGroupSummary,
	binding artifact.DriftEventGroupSummary,
) string {
	if material.UniqueEvents > 0 && binding.UniqueEvents > 0 {
		return fmt.Sprintf(
			"Current drift needs operator review: %d material event(s), %d binding-resolution event(s)",
			material.UniqueEvents,
			binding.UniqueEvents,
		)
	}
	if material.UniqueEvents > 0 {
		return fmt.Sprintf(
			"Current material drift needs operator review: %d event(s), %d impacted decision(s), max fanout %d",
			material.UniqueEvents,
			material.ImpactedDecisions,
			material.MaxFanout,
		)
	}
	return fmt.Sprintf(
		"Current drift needs binding resolution: %d event(s), %d impacted decision(s)",
		binding.UniqueEvents,
		binding.ImpactedDecisions,
	)
}

func nonDriftStatusSignals(signals []overseer.StatusSignal) []overseer.StatusSignal {
	out := make([]overseer.StatusSignal, 0, len(signals))
	for _, signal := range signals {
		switch strings.TrimSpace(signal.Source) {
		case "drift", "scoped_drift":
			continue
		default:
			out = append(out, signal)
		}
	}
	return out
}
