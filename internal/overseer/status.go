package overseer

import (
	"fmt"
	"sort"
	"strings"
)

const statusSignalLimit = 7

func BuildStatusSummary(
	stored StoredRun,
	hasStored bool,
	maintenance MaintenanceRun,
	hasMaintenance bool,
) StatusSummary {
	summary := StatusSummary{
		Signals: []StatusSignal{},
	}

	if hasStored {
		summary.LatestReviewRunID = stored.Run.ReviewRunID
		summary.LatestPacketID = stored.Packet.PacketID
		summary.Signals = append(summary.Signals, reviewRunSignals(stored)...)
		summary.SuppressedCount += packetSuppressedCount(stored.Packet)
	}

	if hasMaintenance {
		summary.LatestMaintenanceID = maintenance.MaintenanceID
		for _, signal := range maintenance.Signals {
			signal.MaintenanceRunID = maintenance.MaintenanceID
			summary.Signals = append(summary.Signals, signal)
		}
		summary.SuppressedCount += len(maintenance.Suppressed)
		summary.ExecutedActions = maintenance.Executed
		if len(maintenance.Executed) > 0 {
			summary.LatestExecutedMaintenanceID = maintenance.MaintenanceID
		}
	}

	summary.Signals = normalizeStatusSignals(summary.Signals)
	summary.HasSignals = len(summary.Signals) > 0 || len(summary.ExecutedActions) > 0
	return summary
}

func CompactStatusSummaryForDefault(summary StatusSummary) StatusSummary {
	compactSignals := compactStatusSignalsForDefault(summary.Signals)
	omitted := len(summary.Signals) - len(compactSignals)
	if omitted < 0 {
		omitted = 0
	}

	projected := summary
	projected.Signals = compactSignals
	projected.SignalProjection = &StatusSignalProjection{
		Mode:               "compact_default",
		ExactSignalCount:   len(summary.Signals),
		EmittedSignalCount: len(compactSignals),
		OmittedSignalCount: omitted,
		ExactCommand:       "haft overseer status --json --full",
	}
	projected.HasSignals = len(projected.Signals) > 0 || len(projected.ExecutedActions) > 0
	return projected
}

func FormatStatusSignals(summary StatusSummary) string {
	if !summary.HasSignals {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Overseer Signals\n\n")

	sb.WriteString(formatExecutedDisclosure(summary))

	signals := compactStatusSignalsForDefault(summary.Signals)
	for i, signal := range signals {
		if i >= statusSignalLimit {
			sb.WriteString(fmt.Sprintf("- **INFO** %d more overseer signal(s) hidden from compact status.\n", len(signals)-statusSignalLimit))
			break
		}
		sb.WriteString(formatStatusSignal(signal))
	}

	sb.WriteString("\n")
	return sb.String()
}

func compactStatusSignalsForDefault(signals []StatusSignal) []StatusSignal {
	driftSignals := make([]StatusSignal, 0)
	staleSignals := make([]StatusSignal, 0)
	otherSignals := make([]StatusSignal, 0, len(signals))
	for _, signal := range signals {
		if isDefaultStatusDriftSignal(signal) {
			driftSignals = append(driftSignals, signal)
			continue
		}
		if isDefaultStatusStaleSignal(signal) {
			staleSignals = append(staleSignals, signal)
			continue
		}
		otherSignals = append(otherSignals, signal)
	}
	if len(driftSignals) <= 1 && len(staleSignals) <= 1 {
		return signals
	}

	if len(driftSignals) > 1 {
		otherSignals = append(otherSignals, compactDriftStatusSignal(driftSignals))
	} else {
		otherSignals = append(otherSignals, driftSignals...)
	}
	if len(staleSignals) > 1 {
		otherSignals = append(otherSignals, compactStaleStatusSignal(staleSignals))
	} else {
		otherSignals = append(otherSignals, staleSignals...)
	}
	otherSignals = normalizeStatusSignals(otherSignals)
	return otherSignals
}

func isDefaultStatusDriftSignal(signal StatusSignal) bool {
	switch strings.TrimSpace(signal.Source) {
	case maintenanceSourceDrift, "scoped_drift":
		return true
	default:
		return false
	}
}

func isDefaultStatusStaleSignal(signal StatusSignal) bool {
	switch strings.TrimSpace(signal.Source) {
	case maintenanceSourceStale, "scoped_stale":
		return true
	default:
		return false
	}
}

func compactDriftStatusSignal(signals []StatusSignal) StatusSignal {
	severity := "medium"
	confirmRequired := 0
	reviewRequired := 0
	for _, signal := range signals {
		if severityRank(signal.Severity) < severityRank(severity) {
			severity = signal.Severity
		}
		title := strings.ToLower(signal.Title)
		if strings.Contains(title, "requires confirmation") {
			confirmRequired++
			continue
		}
		reviewRequired++
	}

	title := fmt.Sprintf("Drift grouped for review: %d item(s)", len(signals))
	if confirmRequired > 0 {
		title = fmt.Sprintf("Drift requires confirmation: %d item(s) grouped", confirmRequired)
		if reviewRequired > 0 {
			title += fmt.Sprintf(", %d more need review", reviewRequired)
		}
	}

	return StatusSignal{
		Severity: severity,
		Source:   maintenanceSourceDrift,
		Title:    title,
		Detail:   "compact status groups per-decision drift; inspect exact items with `haft overseer judgment --json --limit 20`, `haft overseer drain --dry-run --json`, or `haft_refresh(action=\"scan\", verbose=true)`",
	}
}

func compactStaleStatusSignal(signals []StatusSignal) StatusSignal {
	severity := "medium"
	atRisk := 0
	for _, signal := range signals {
		if severityRank(signal.Severity) < severityRank(severity) {
			severity = signal.Severity
		}
		if strings.Contains(strings.ToLower(signal.Detail), "at risk") {
			atRisk++
		}
	}

	title := fmt.Sprintf("Stale governance artifacts: %d item(s) grouped", len(signals))
	if atRisk > 0 {
		title += fmt.Sprintf(", %d at risk", atRisk)
	}

	return StatusSignal{
		Severity: severity,
		Source:   maintenanceSourceStale,
		Title:    title,
		Detail:   "compact status groups per-decision stale findings; inspect exact items with `haft_refresh(action=\"scan\", verbose=true)`, `haft_refresh(action=\"review\")`, or `haft overseer drain --dry-run --json`",
	}
}

const executedDisclosureLimit = 5

// formatExecutedDisclosure renders the autonomous-maintenance ledger of the
// latest run at the top of status — the loop acts only in the open: every
// entry names the decision, the action, and the undo path.
func formatExecutedDisclosure(summary StatusSummary) string {
	if len(summary.ExecutedActions) == 0 {
		return ""
	}

	var sb strings.Builder
	maintenanceID := statusExecutedMaintenanceID(summary)
	fmt.Fprintf(&sb, "- **AUTONOMOUS MAINTENANCE** %d action(s) in run `%s` (undo: `haft overseer undo %s <action-id>`):\n",
		len(summary.ExecutedActions), maintenanceID, maintenanceID)
	for i, action := range summary.ExecutedActions {
		if i >= executedDisclosureLimit {
			fmt.Fprintf(&sb, "  - … and %d more (inspect `haft overseer status --json`)\n", len(summary.ExecutedActions)-executedDisclosureLimit)
			break
		}
		title := action.Title
		if title == "" {
			title = "Untitled decision"
		}
		undo := statusActionUndoCommand(maintenanceID, action)
		if undo != "" {
			fmt.Fprintf(&sb, "  - [%s] %s — %s `%s` (%s; undo: `%s`)\n", action.ID, action.Kind, title, action.DecisionRef, action.Outcome, undo)
			continue
		}
		fmt.Fprintf(&sb, "  - [%s] %s — %s `%s` (%s)\n", action.ID, action.Kind, title, action.DecisionRef, action.Outcome)
	}
	return sb.String()
}

func statusExecutedMaintenanceID(summary StatusSummary) string {
	if summary.LatestExecutedMaintenanceID != "" {
		return summary.LatestExecutedMaintenanceID
	}
	return summary.LatestMaintenanceID
}

func statusActionUndoCommand(maintenanceID string, action MaintenanceAction) string {
	if maintenanceID == "" || action.ID == "" || action.PriorState == "" {
		return ""
	}
	if action.Undo != "" {
		return action.Undo
	}
	return "haft overseer undo " + maintenanceID + " " + action.ID
}

func reviewRunSignals(stored StoredRun) []StatusSignal {
	signals := make([]StatusSignal, 0)

	for _, finding := range UnresolvedFindings(stored.Run) {
		signals = append(signals, StatusSignal{
			Severity:    normalizeSeverity(finding.Severity),
			Source:      "review_finding",
			Title:       strings.TrimSpace(finding.Claim),
			Detail:      strings.TrimSpace(finding.ConcreteHarm),
			Command:     "haft overseer show " + stored.Run.ReviewRunID,
			ReviewRunID: stored.Run.ReviewRunID,
			PacketID:    stored.Packet.PacketID,
			FindingID:   finding.ID,
		})
	}

	if len(signals) == 0 &&
		stored.Run.Verdict == "packet_generated" &&
		riskNeedsReview(stored.Packet.Risk.Level) {
		signals = append(signals, StatusSignal{
			Severity:    normalizeSeverity(stored.Packet.Risk.Level),
			Source:      "deterministic_packet",
			Title:       strings.ToUpper(stored.Packet.Risk.Level) + " risk overseer packet pending review",
			Detail:      packetReviewDetail(stored.Packet),
			Command:     "haft overseer show " + stored.Run.ReviewRunID,
			ReviewRunID: stored.Run.ReviewRunID,
			PacketID:    stored.Packet.PacketID,
		})
	}

	signals = append(signals, packetFindingSignals(stored)...)
	return signals
}

func packetFindingSignals(stored StoredRun) []StatusSignal {
	findings := stored.Packet.DeterministicFindings
	signals := make([]StatusSignal, 0)
	signals = append(signals, findingSummarySignals(stored, "scoped_stale", "Scoped stale governance debt", "medium", findings.Stale)...)
	signals = append(signals, findingSummarySignals(stored, "scoped_drift", "Scoped drift detected", "high", findings.Drift)...)
	signals = append(signals, findingSummarySignals(stored, "scoped_spec_health", "Scoped spec health finding", "medium", findings.SpecHealth)...)
	signals = append(signals, findingSummarySignals(stored, "scoped_coverage_gap", "Scoped coverage gap", "medium", findings.CoverageGaps)...)
	return signals
}

func findingSummarySignals(
	stored StoredRun,
	source string,
	prefix string,
	severity string,
	findings []FindingSummary,
) []StatusSignal {
	out := make([]StatusSignal, 0, len(findings))
	for _, finding := range findings {
		out = append(out, StatusSignal{
			Severity:    severity,
			Source:      source,
			Title:       prefix + ": " + signalTitle(finding.ID, finding.Title),
			Detail:      strings.TrimSpace(finding.Reason),
			Command:     "haft overseer show " + stored.Run.ReviewRunID,
			ReviewRunID: stored.Run.ReviewRunID,
			PacketID:    stored.Packet.PacketID,
		})
	}
	return out
}

func normalizeStatusSignals(signals []StatusSignal) []StatusSignal {
	out := make([]StatusSignal, 0, len(signals))
	seen := make(map[string]int)
	for _, signal := range signals {
		signal.Severity = normalizeSeverity(signal.Severity)
		signal.Source = strings.TrimSpace(signal.Source)
		signal.Title = strings.TrimSpace(signal.Title)
		signal.Detail = strings.TrimSpace(signal.Detail)
		signal.Command = strings.TrimSpace(signal.Command)
		signal.ReviewRunID = strings.TrimSpace(signal.ReviewRunID)
		signal.PacketID = strings.TrimSpace(signal.PacketID)
		signal.FindingID = strings.TrimSpace(signal.FindingID)
		signal.MaintenanceRunID = strings.TrimSpace(signal.MaintenanceRunID)
		if signal.Title == "" {
			continue
		}
		key := statusSignalDedupeKey(signal)
		if existing, ok := seen[key]; ok {
			if severityRank(signal.Severity) < severityRank(out[existing].Severity) {
				out[existing] = signal
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, signal)
	}

	sort.Slice(out, func(i, j int) bool {
		left := severityRank(out[i].Severity)
		right := severityRank(out[j].Severity)
		if left != right {
			return left < right
		}
		if out[i].Source == out[j].Source {
			return out[i].Title < out[j].Title
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func statusSignalDedupeKey(signal StatusSignal) string {
	return strings.Join([]string{
		normalizeStatusSignalSubject(signal.Title),
		signal.Detail,
	}, "\x00")
}

func normalizeStatusSignalSubject(title string) string {
	subject := strings.TrimSpace(title)
	prefixes := []string{
		"Scoped stale governance debt: ",
		"Stale governance artifact: ",
		"Scoped drift detected: ",
		"Drift detected: ",
		"Scoped spec health finding: ",
		"Spec health finding: ",
		"Scoped coverage gap: ",
		"Coverage gap: ",
	}
	for _, prefix := range prefixes {
		subject = strings.TrimPrefix(subject, prefix)
	}
	return strings.Join(strings.Fields(subject), " ")
}

func formatStatusSignal(signal StatusSignal) string {
	line := fmt.Sprintf("- **%s** %s", strings.ToUpper(signal.Severity), signal.Title)
	if signal.Detail != "" {
		line += " - " + signal.Detail
	}
	if signal.Command != "" {
		line += " - `" + signal.Command + "`"
	}
	return line + "\n"
}

func packetReviewDetail(packet Packet) string {
	modes := strings.Join(packet.ReviewRequest.Modes, ", ")
	if modes == "" {
		modes = "no specialized mode"
	}
	return fmt.Sprintf("%d changed file(s); modes: %s", len(packet.ChangedFiles), modes)
}

func packetSuppressedCount(packet Packet) int {
	suppressed := packet.DeterministicFindings.Suppressed
	return suppressed.UnrelatedStale +
		suppressed.UnrelatedDrift +
		suppressed.UnrelatedSpecHealth +
		suppressed.UnrelatedCoverageGaps
}

func riskNeedsReview(level string) bool {
	level = normalizeSeverity(level)
	return level == "high" || level == "medium"
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "high":
		return "high"
	case "medium", "warning":
		return "medium"
	case "low", "info":
		return "low"
	default:
		return "medium"
	}
}

func severityRank(severity string) int {
	switch normalizeSeverity(severity) {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}
