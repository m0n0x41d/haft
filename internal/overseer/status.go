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
	}

	summary.Signals = normalizeStatusSignals(summary.Signals)
	summary.HasSignals = len(summary.Signals) > 0 || len(summary.ExecutedActions) > 0
	return summary
}

func FormatStatusSignals(summary StatusSummary) string {
	if !summary.HasSignals {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Overseer Signals\n\n")

	sb.WriteString(formatExecutedDisclosure(summary))

	for i, signal := range summary.Signals {
		if i >= statusSignalLimit {
			sb.WriteString(fmt.Sprintf("- **INFO** %d more overseer signal(s) hidden from compact status.\n", len(summary.Signals)-statusSignalLimit))
			break
		}
		sb.WriteString(formatStatusSignal(signal))
	}

	if summary.SuppressedCount > 0 {
		sb.WriteString(fmt.Sprintf(
			"- **INFO** %d low-signal maintenance item(s) suppressed with audit trail (triage, not improvement); inspect `haft overseer maintain --json` for the latest classifier output.\n",
			summary.SuppressedCount,
		))
	}

	sb.WriteString("\n")
	return sb.String()
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
	fmt.Fprintf(&sb, "- **AUTONOMOUS MAINTENANCE** %d action(s) in run `%s` (undo: `haft overseer undo %s <action-id>`):\n",
		len(summary.ExecutedActions), summary.LatestMaintenanceID, summary.LatestMaintenanceID)
	for i, action := range summary.ExecutedActions {
		if i >= executedDisclosureLimit {
			fmt.Fprintf(&sb, "  - … and %d more (inspect `haft overseer maintain --json`)\n", len(summary.ExecutedActions)-executedDisclosureLimit)
			break
		}
		title := action.Title
		if title == "" {
			title = action.DecisionRef
		}
		fmt.Fprintf(&sb, "  - [%s] %s — %s (%s) `%s`\n", action.ID, action.Kind, title, action.Outcome, action.DecisionRef)
	}
	return sb.String()
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
	seen := make(map[string]bool)
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
		key := strings.Join([]string{
			signal.Severity,
			signal.Source,
			signal.Title,
			signal.Detail,
			signal.ReviewRunID,
			signal.MaintenanceRunID,
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
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
