package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/project"
)

const currentScopeSpecHealthStatusSource = "current_scope_spec_health"

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
		"derived from current DriftEventReport; %d audit-only/resolved event(s) stay in drill-downs. These signals are attention, not a project-wide Work gate; inspect exact affected authority before interrupting current Work. Inspect exact items with `%s`, `haft overseer judgment --json --limit 20`, or `haft overseer drain --dry-run --json`",
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
			"Current drift needs scoped inspection: %d material event(s), %d binding-resolution event(s)",
			material.UniqueEvents,
			binding.UniqueEvents,
		)
	}
	if material.UniqueEvents > 0 {
		return fmt.Sprintf(
			"Current material drift needs scoped inspection: %d event(s), %d impacted decision(s), max fanout %d",
			material.UniqueEvents,
			material.ImpactedDecisions,
			material.MaxFanout,
		)
	}
	return fmt.Sprintf(
		"Current drift has unresolved binding targets: %d event(s), %d impacted decision(s)",
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

// overlayLiveProfileSpecHealthStatusSignals replaces persisted spec-health
// attention with a current exact-scope projection. The persisted overseer run
// remains available through its own drill-down; the public project status must
// not present an old or unscoped SoftwareSystemSpec finding as current.
func overlayLiveProfileSpecHealthStatusSignals(
	ctx context.Context,
	readiness canonicalProjectReadiness,
	summary overseer.StatusSummary,
) overseer.StatusSummary {
	retained := nonProfileSpecHealthStatusSignals(summary.Signals)
	findings, scopeID, resolved := currentScopeSpecHealthFindings(
		ctx,
		readiness,
	)
	if resolved {
		live := currentScopeSpecHealthStatusSignals(findings, scopeID)
		retained = append(live, retained...)
	}
	summary.Signals = retained
	summary.SignalProjection = nil
	summary.HasSignals = len(summary.Signals) > 0 ||
		len(summary.ExecutedActions) > 0
	return summary
}

func currentScopeSpecHealthFindings(
	ctx context.Context,
	readiness canonicalProjectReadiness,
) ([]project.SpecCheckFinding, string, bool) {
	applicability, resolved := readiness.resolvedApplicability()
	if !resolved {
		return nil, "", false
	}
	projectRoot := readiness.resolution.ProjectRoot().String()
	specificationSet, err := loadProjectSpecificationSetSQLFirstForScope(
		projectRoot,
		applicability,
	)
	if err != nil {
		return nil, "", false
	}
	report := project.SpecCheckReportFromSpecificationSet(specificationSet)
	report = appendSpecHealthFindingsFromSet(
		report,
		specificationSet,
		projectRoot,
	)
	return report.Findings, applicability.ScopeID().String(), true
}

func currentScopeSpecHealthStatusSignals(
	findings []project.SpecCheckFinding,
	scopeID string,
) []overseer.StatusSignal {
	result := make([]overseer.StatusSignal, len(findings))
	for index, finding := range findings {
		result[index] = overseer.StatusSignal{
			Severity: currentScopeSpecHealthSeverity(finding.Level),
			Source:   currentScopeSpecHealthStatusSource,
			Title: "Current scope spec health: " +
				currentScopeSpecHealthTitle(finding),
			Detail:  strings.TrimSpace(finding.Message),
			Command: "haft spec check --scope-id " + scopeID,
		}
	}
	return result
}

func currentScopeSpecHealthSeverity(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return "high"
	case "info":
		return "low"
	default:
		return "medium"
	}
}

func currentScopeSpecHealthTitle(finding project.SpecCheckFinding) string {
	code := strings.TrimSpace(finding.Code)
	if code != "" {
		return code
	}
	path := strings.TrimSpace(finding.Path)
	if path != "" {
		return path
	}
	return "unclassified finding"
}

func nonProfileSpecHealthStatusSignals(
	signals []overseer.StatusSignal,
) []overseer.StatusSignal {
	result := make([]overseer.StatusSignal, 0, len(signals))
	for _, signal := range signals {
		switch strings.TrimSpace(signal.Source) {
		case "spec_health", "scoped_spec_health", currentScopeSpecHealthStatusSource:
			continue
		default:
			result = append(result, signal)
		}
	}
	return result
}
