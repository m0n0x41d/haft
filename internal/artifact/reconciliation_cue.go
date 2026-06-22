package artifact

const (
	ReconciliationCueSchemaVersion = 1
	ReconciliationCueAuthority     = "read_only_reconciliation_attention"

	ReconciliationCueHighFanout              = "high_fanout_drift_event"
	ReconciliationCueReconciliationCandidate = "decision_reconciliation_candidate"
	ReconciliationCueGoverningConflict       = "current_governing_conflict"
)

type ReconciliationCueReport struct {
	SchemaVersion int                      `json:"schema_version"`
	Authority     string                   `json:"authority"`
	Summary       ReconciliationCueSummary `json:"summary"`
	Cues          []ReconciliationCue      `json:"cues,omitempty"`
	Commands      []string                 `json:"commands,omitempty"`
}

type ReconciliationCueSummary struct {
	HighFanoutEvents       int `json:"high_fanout_events"`
	MaxFanout              int `json:"max_fanout"`
	ReconciliationGroups   int `json:"reconciliation_groups"`
	OperatorRequiredGroups int `json:"operator_required_groups"`
	GoverningConflictSets  int `json:"governing_conflict_sets"`
	GoverningOverlapSets   int `json:"governing_overlap_sets"`
}

type ReconciliationCue struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Command  string `json:"command"`
}

func BuildReconciliationCueReport(
	driftEvents DriftEventReport,
	reconciliation DecisionReconciliationPlan,
	governing CurrentGoverningSetReport,
) ReconciliationCueReport {
	report := ReconciliationCueReport{
		SchemaVersion: ReconciliationCueSchemaVersion,
		Authority:     ReconciliationCueAuthority,
	}

	report = appendHighFanoutCue(report, driftEvents)
	report = appendReconciliationCandidateCue(report, reconciliation)
	report = appendGoverningConflictCue(report, governing)
	report.Commands = reconciliationCueCommands(report.Cues)
	return report
}

func appendHighFanoutCue(
	report ReconciliationCueReport,
	driftEvents DriftEventReport,
) ReconciliationCueReport {
	report.Summary.MaxFanout = driftEvents.Summary.MaxFanout
	for _, event := range driftEvents.Events {
		if event.Fanout <= 1 {
			continue
		}
		report.Summary.HighFanoutEvents++
	}
	if report.Summary.HighFanoutEvents == 0 {
		return report
	}

	report.Cues = append(report.Cues, ReconciliationCue{
		Kind:     ReconciliationCueHighFanout,
		Severity: "medium",
		Title:    "High-fanout drift events need reconciliation review",
		Detail:   "one changed target impacts multiple current decisions; treat fanout as one event with many impacts, not independent debt",
		Command:  `haft_query(action="drift_events")`,
	})
	return report
}

func appendReconciliationCandidateCue(
	report ReconciliationCueReport,
	plan DecisionReconciliationPlan,
) ReconciliationCueReport {
	for _, group := range plan.Groups {
		if group.Category == DecisionReconciliationKeep {
			continue
		}
		report.Summary.ReconciliationGroups++
		if group.OperatorRequired {
			report.Summary.OperatorRequiredGroups++
		}
	}
	if report.Summary.ReconciliationGroups == 0 {
		return report
	}

	report.Cues = append(report.Cues, ReconciliationCue{
		Kind:     ReconciliationCueReconciliationCandidate,
		Severity: "medium",
		Title:    "Decision reconciliation candidates available",
		Detail:   "review-only grouping found reopen/merge/supersede/retire/conflict candidates; apply still requires explicit operator-approved selection",
		Command:  `haft_query(action="decision_reconcile")`,
	})
	return report
}

func appendGoverningConflictCue(
	report ReconciliationCueReport,
	governing CurrentGoverningSetReport,
) ReconciliationCueReport {
	report.Summary.GoverningConflictSets = governing.Summary.ConflictSets
	report.Summary.GoverningOverlapSets = governing.Summary.OverlapReviewSets
	if governing.Summary.ConflictSets == 0 && governing.Summary.OverlapReviewSets == 0 {
		return report
	}

	severity := "medium"
	if governing.Summary.ConflictSets > 0 {
		severity = "high"
	}
	report.Cues = append(report.Cues, ReconciliationCue{
		Kind:     ReconciliationCueGoverningConflict,
		Severity: severity,
		Title:    "Current governing authority needs operator review",
		Detail:   "active decisions overlap or explicitly conflict for the same subject/context/target; this is a blocker cue, not a GateDecision",
		Command:  `haft_query(action="governing_set")`,
	})
	return report
}

func reconciliationCueCommands(cues []ReconciliationCue) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, cue := range cues {
		if cue.Command == "" {
			continue
		}
		if _, ok := seen[cue.Command]; ok {
			continue
		}
		seen[cue.Command] = struct{}{}
		out = append(out, cue.Command)
	}
	return out
}
