package cli

import (
	"context"
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

// applyProfileAwareReadinessReminder appends one scope-local profile or
// readiness cue to human-readable reasoning-tool results. It never blocks the
// underlying call and never alters machine-readable JSON.
func applyProfileAwareReadinessReminder(
	ctx context.Context,
	result string,
	toolName string,
	haftDir string,
	args map[string]any,
) string {
	if _, ok := readinessReminderTools[toolName]; !ok {
		return result
	}
	if machineJSONResponse(result) {
		return result
	}

	projectRoot := filepath.Dir(haftDir)
	request, err := projectSpecificationScopeRequestFromFlag(
		stringArg(args, "scope_id"),
	)
	if err != nil {
		return result
	}
	readiness, err := inspectCanonicalProjectReadiness(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return result
	}
	if cue := readiness.profileCue(); cue != "" {
		return result + "\n\n" +
			"── Project profile ─────────────\n" +
			cue + "\n" +
			"This does not block the completed operation and requires no action now.\n" +
			"Use /h-onboard only when onboarding or profile-dependent capability review is the current question.\n" +
			"────────────────────────────────"
	}
	applicability, resolved := readiness.resolvedApplicability()
	if !resolved ||
		readiness.facts.Status != project.ReadinessNeedsOnboard ||
		len(requiredCommissionAuthorityDocumentKinds(applicability)) == 0 {
		return result
	}
	return appendReadinessReminder(result)
}

func appendReadinessReminder(result string) string {
	return result + "\n\n" +
		"── ⚠ Project readiness ─────────\n" +
		"This project is `needs_onboard` — `.haft/` exists but the\n" +
		"ProjectSpecificationSet has no active SpecSections yet. Decisions\n" +
		"made now cannot link to spec refs and downstream\n" +
		"WorkCommission execution starts will block until specs are in\n" +
		"place. Run /h-spec to follow the typed spec lifecycle\n" +
		"(/h-onboard remains a bootstrap alias), or proceed and record the work as tactical\n" +
		"so coverage will not later confuse it with spec-driven work.\n" +
		"────────────────────────────────"
}
