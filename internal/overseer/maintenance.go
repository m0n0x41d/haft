package overseer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	maintenanceSourceStale      = "staleness"
	maintenanceSourceDrift      = "drift"
	maintenanceSourceSpecHealth = "spec_health"
	maintenanceSourceCoverage   = "spec_coverage"
)

func BuildMaintenanceRun(input MaintenanceInput) (MaintenanceRun, error) {
	run := MaintenanceRun{
		SchemaVersion: MaintenanceRunSchemaVersion,
		CreatedAt:     strings.TrimSpace(input.CreatedAt),
		Verdict:       "signals_recorded",
		Authority:     DefaultReviewAuthority(),
		Signals:       []StatusSignal{},
		Suppressed:    []MaintenanceSuppression{},
	}

	run = appendDriftMaintenance(run, input.Drift)
	run = appendSummaryMaintenance(run, maintenanceSourceStale, input.Stale)
	run = appendSummaryMaintenance(run, maintenanceSourceSpecHealth, input.SpecHealth)
	run = appendSummaryMaintenance(run, maintenanceSourceCoverage, input.CoverageGaps)
	run.Signals = normalizeStatusSignals(run.Signals)
	run.Suppressed = normalizeSuppressions(run.Suppressed)
	run.Executed = normalizeExecutedActions(input.Executed)
	run.Summary = maintenanceSummary(run, input)

	if len(run.Signals) == 0 && len(run.Suppressed) == 0 && len(run.Executed) == 0 {
		run.Verdict = "clean"
	}

	runID, err := maintenanceRunID(run)
	if err != nil {
		return MaintenanceRun{}, err
	}
	run.MaintenanceID = runID
	return run, nil
}

func appendDriftMaintenance(run MaintenanceRun, findings []MaintenanceDriftFinding) MaintenanceRun {
	for _, finding := range findings {
		action := strings.TrimSpace(finding.Action)
		switch action {
		case "auto_resolve_silent":
			run.Suppressed = append(run.Suppressed, MaintenanceSuppression{
				ID:     strings.TrimSpace(finding.ID),
				Title:  strings.TrimSpace(finding.Title),
				Source: maintenanceSourceDrift,
				Action: "suppressed_auto_baseline_candidate",
				Reason: strings.TrimSpace(finding.Reason),
				Detail: strings.TrimSpace(finding.Summary),
			})
		case "stage_for_confirm":
			run.Signals = append(run.Signals, StatusSignal{
				Severity: "high",
				Source:   maintenanceSourceDrift,
				Title:    "Drift requires confirmation: " + signalTitle(finding.ID, finding.Title),
				Detail:   signalDetail(finding.Summary, finding.Reason),
			})
		default:
			run.Signals = append(run.Signals, StatusSignal{
				Severity: "medium",
				Source:   maintenanceSourceDrift,
				Title:    "Drift needs review: " + signalTitle(finding.ID, finding.Title),
				Detail:   signalDetail(finding.Summary, finding.Reason),
			})
		}
	}
	return run
}

func appendSummaryMaintenance(run MaintenanceRun, source string, findings []FindingSummary) MaintenanceRun {
	for _, finding := range normalizeFindingSummaries(findings) {
		run.Signals = append(run.Signals, StatusSignal{
			Severity: maintenanceSummarySeverity(source, finding),
			Source:   source,
			Title:    maintenanceSummaryTitle(source, finding),
			Detail:   strings.TrimSpace(finding.Reason),
		})
	}
	return run
}

func maintenanceSummarySeverity(source string, finding FindingSummary) string {
	level := strings.ToLower(strings.TrimSpace(finding.Category))
	if level == "error" || strings.Contains(strings.ToLower(finding.Reason), "at risk") {
		return "high"
	}
	if source == maintenanceSourceDrift {
		return "high"
	}
	return "medium"
}

func maintenanceSummaryTitle(source string, finding FindingSummary) string {
	title := signalTitle(finding.ID, finding.Title)
	switch source {
	case maintenanceSourceStale:
		return "Stale governance artifact: " + title
	case maintenanceSourceSpecHealth:
		return "Spec health finding: " + title
	case maintenanceSourceCoverage:
		return "Spec coverage gap: " + title
	default:
		return title
	}
}

func normalizeExecutedActions(actions []MaintenanceAction) []MaintenanceAction {
	out := make([]MaintenanceAction, 0, len(actions))
	for i, action := range actions {
		action.Kind = strings.TrimSpace(action.Kind)
		action.DecisionRef = strings.TrimSpace(action.DecisionRef)
		action.Outcome = strings.TrimSpace(action.Outcome)
		if action.Kind == "" || action.DecisionRef == "" {
			continue
		}
		if action.ID == "" {
			action.ID = fmt.Sprintf("act-%03d", i+1)
		}
		out = append(out, action)
	}
	return out
}

func maintenanceSummary(run MaintenanceRun, input MaintenanceInput) MaintenanceSummary {
	summary := MaintenanceSummary{
		SignalCount:      len(run.Signals),
		SuppressedCount:  len(run.Suppressed),
		StaleCount:       len(input.Stale),
		SpecHealthCount:  len(input.SpecHealth),
		CoverageGapCount: len(input.CoverageGaps),
		ExecutedCount:    len(run.Executed),
	}
	for _, drift := range input.Drift {
		switch strings.TrimSpace(drift.Action) {
		case "auto_resolve_silent":
			summary.AutoResolvableDrift++
		case "stage_for_confirm":
			summary.ConfirmRequiredDrift++
		default:
			summary.ReviewRequiredDrift++
		}
	}
	return summary
}

func maintenanceRunID(run MaintenanceRun) (string, error) {
	canonical := run
	canonical.MaintenanceID = ""
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal maintenance run: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return "omnt-" + hex.EncodeToString(sum[:])[:16], nil
}

func normalizeSuppressions(items []MaintenanceSuppression) []MaintenanceSuppression {
	out := make([]MaintenanceSuppression, 0, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Title = strings.TrimSpace(item.Title)
		item.Source = strings.TrimSpace(item.Source)
		item.Action = strings.TrimSpace(item.Action)
		item.Reason = strings.TrimSpace(item.Reason)
		item.Detail = strings.TrimSpace(item.Detail)
		if item.ID == "" && item.Title == "" {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source == out[j].Source {
			return out[i].ID < out[j].ID
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func signalDetail(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "; ")
}

func signalTitle(id string, title string) string {
	title = strings.TrimSpace(title)
	id = strings.TrimSpace(id)
	if title != "" {
		if id != "" {
			return fmt.Sprintf("%s `%s`", title, id)
		}
		return title
	}
	if id != "" {
		return "untitled artifact `" + id + "`"
	}
	return "unnamed signal"
}
