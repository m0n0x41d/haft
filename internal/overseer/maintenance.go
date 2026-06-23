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
	run.ReconciliationProposals = normalizeReconciliationProposals(input.ReconciliationProposals)
	run.ReconciliationSummary = buildMaintenanceReconciliationSummary(run.ReconciliationProposals)
	run.Summary = maintenanceSummary(run, input)

	if len(run.Signals) == 0 && len(run.Suppressed) == 0 && len(run.Executed) == 0 && len(run.ReconciliationProposals) == 0 {
		run.Verdict = "clean"
	}

	runID, err := maintenanceRunID(run)
	if err != nil {
		return MaintenanceRun{}, err
	}
	run.MaintenanceID = runID
	run.AfterAction = buildMaintenanceAfterAction(run)
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
		action.EvidenceRefs = compactStrings(action.EvidenceRefs)
		if action.Kind == "" || action.DecisionRef == "" {
			continue
		}
		if !isAllowedExecutedMaintenanceActionKind(action.Kind) {
			continue
		}
		if action.ID == "" {
			action.ID = fmt.Sprintf("act-%03d", i+1)
		}
		out = append(out, action)
	}
	return out
}

func isAllowedExecutedMaintenanceActionKind(kind string) bool {
	switch kind {
	case "auto_rebaseline", "observable_run", "revalidate_stale":
		return true
	default:
		return false
	}
}

func normalizeReconciliationProposals(items []MaintenanceReconciliationProposal) []MaintenanceReconciliationProposal {
	out := make([]MaintenanceReconciliationProposal, 0, len(items))
	for index, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			item.ID = fmt.Sprintf("reconcile-proposal-%03d", index+1)
		}
		item.Kind = strings.TrimSpace(item.Kind)
		item.GroupID = strings.TrimSpace(item.GroupID)
		item.Category = strings.TrimSpace(item.Category)
		item.Reason = strings.TrimSpace(item.Reason)
		item.DecisionRefs = compactStrings(item.DecisionRefs)
		item.FallbackTargets = compactStrings(item.FallbackTargets)
		item.ScopeRepairHints = compactStrings(item.ScopeRepairHints)
		item.SuggestedCommand = strings.TrimSpace(item.SuggestedCommand)
		item.AuthorityBoundary = strings.TrimSpace(item.AuthorityBoundary)
		if item.AuthorityBoundary == "" {
			item.AuthorityBoundary = "read_only_reconciliation_proposal_not_binding_authority"
		}
		if item.Kind == "" || item.Reason == "" {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].ID < out[j].ID
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func buildMaintenanceAfterAction(run MaintenanceRun) MaintenanceAfterActionReport {
	report := MaintenanceAfterActionReport{
		AuthorityBoundary: "after_action_report_only_not_binding_authority",
	}
	for _, action := range run.Executed {
		item := MaintenanceAfterActionItem{
			Ref:          action.DecisionRef,
			Title:        action.Title,
			Action:       action.Kind,
			Outcome:      action.Outcome,
			Command:      action.Detail,
			EvidenceRefs: action.EvidenceRefs,
			Reason:       action.Detail,
		}
		switch action.Kind {
		case "auto_rebaseline", "revalidate_stale":
			if action.Outcome == "applied" {
				report.AutoClosedItems = append(report.AutoClosedItems, item)
			}
		case "observable_run":
			report.EvidenceChecked = append(report.EvidenceChecked, item)
		}
		if action.PriorState != "" && action.ID != "" && run.MaintenanceID != "" {
			report.UndoCommands = append(report.UndoCommands, "haft overseer undo "+run.MaintenanceID+" "+action.ID)
		}
	}
	for _, signal := range run.Signals {
		report.RemainingOperatorJudgment = append(report.RemainingOperatorJudgment, MaintenanceAfterActionItem{
			Ref:     signal.FindingID,
			Title:   signal.Title,
			Action:  signal.Source,
			Outcome: "needs_operator",
			Command: signal.Command,
			Reason:  signal.Detail,
		})
	}
	return report
}

func buildMaintenanceReconciliationSummary(
	proposals []MaintenanceReconciliationProposal,
) *MaintenanceReconciliationSummary {
	if len(proposals) == 0 {
		return nil
	}

	summary := MaintenanceReconciliationSummary{
		AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
		ProposalCount:     len(proposals),
		ByKind:            map[string]int{},
	}
	commands := make([]string, 0)
	for _, proposal := range proposals {
		summary.ByKind[proposal.Kind]++
		if proposal.Fanout > summary.MaxFanout {
			summary.MaxFanout = proposal.Fanout
		}
		if isFallbackMaintenanceReconciliationProposal(proposal) {
			summary.FallbackProposalCount++
		}
		if isHighFanoutMaintenanceReconciliationProposal(proposal) {
			summary.HighFanoutProposalCount++
		}
		commands = append(commands, proposal.SuggestedCommand)
	}
	summary.SuggestedCommands = compactStrings(commands)
	return &summary
}

func isFallbackMaintenanceReconciliationProposal(proposal MaintenanceReconciliationProposal) bool {
	if len(proposal.FallbackTargets) > 0 {
		return true
	}
	return strings.Contains(proposal.Kind, "fallback")
}

func isHighFanoutMaintenanceReconciliationProposal(proposal MaintenanceReconciliationProposal) bool {
	if proposal.Fanout >= 5 {
		return true
	}
	return strings.Contains(proposal.Kind, "high_fanout")
}

func maintenanceSummary(run MaintenanceRun, input MaintenanceInput) MaintenanceSummary {
	summary := MaintenanceSummary{
		SignalCount:                 len(run.Signals),
		SuppressedCount:             len(run.Suppressed),
		StaleCount:                  len(input.Stale),
		SpecHealthCount:             len(input.SpecHealth),
		CoverageGapCount:            len(input.CoverageGaps),
		ExecutedCount:               len(run.Executed),
		ReconciliationProposalCount: len(run.ReconciliationProposals),
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
	canonical.AfterAction.UndoCommands = nil
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

func compactStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
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
