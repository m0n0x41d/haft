package cli

import (
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/project"
)

// readinessReminderTools enumerates the reasoning-loop tool names that
// should soft-warn the operator when the project is `needs_onboard`.
// The reminder is informational — it never blocks the call. Tools that
// already enforce readiness at the handler boundary (haft_commission,
// haft_spec_section, haft_refresh, haft_query) are intentionally
// excluded so the warning lands where it would change behavior, not
// where it would be redundant.
var readinessReminderTools = map[string]struct{}{
	"haft_problem":  {},
	"haft_solution": {},
	"haft_decision": {},
	"haft_note":     {},
}

// applyReadinessReminder appends a soft nudge to a tool result when the
// project is initialized but has no active SpecSections — i.e. the
// project is `needs_onboard`. Decisions made in this state cannot link
// to spec refs and downstream commission/harness paths will block. The
// reminder tells the operator to run /h-onboard or to mark the work
// tactical with explicit reason.
//
// Skipped when:
//   - the tool is not in readinessReminderTools (avoids spamming reads,
//     scans, and surface tools that already enforce readiness);
//   - the result is a machine-readable JSON payload (do not corrupt
//     deserialization on the consumer side);
//   - the project has no .haft (`needs_init` or `missing` — different
//     conversation, not this nudge);
//   - the project is `ready` (no nudge needed).
func applyReadinessReminder(result, toolName, haftDir string) string {
	if _, ok := readinessReminderTools[toolName]; !ok {
		return result
	}
	if machineJSONResponse(result) {
		return result
	}

	projectRoot := filepath.Dir(haftDir)
	facts, err := project.InspectReadiness(projectRoot)
	if err != nil {
		return result
	}
	if facts.Status != project.ReadinessNeedsOnboard {
		return result
	}

	return result + "\n\n" +
		"── ⚠ Project readiness ─────────\n" +
		"This project is `needs_onboard` — `.haft/` exists but the\n" +
		"ProjectSpecificationSet has no active SpecSections yet. Decisions\n" +
		"made now cannot link to spec refs and downstream\n" +
		"WorkCommissions / harness runs will block until specs are in\n" +
		"place. Run /h-onboard to draft TargetSystemSpec and\n" +
		"EnablingSystemSpec, or proceed and record the work as tactical\n" +
		"so coverage will not later confuse it with spec-driven work.\n" +
		"────────────────────────────────"
}
