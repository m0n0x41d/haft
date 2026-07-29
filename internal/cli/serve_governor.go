package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
	"github.com/m0n0x41d/haft/internal/overseer"
	"github.com/m0n0x41d/haft/internal/present"
)

// governorStatusResponse serves haft_query(action="status", view="governor"):
// a compact, prompt-budgeted projection for host-side prompt governors. The
// kernel owns the budget so host carriers (e.g. @haft/pi before_agent_start)
// stop clipping the full dashboard client-side.
func governorStatusResponse(
	ctx context.Context,
	store *artifact.Store,
	contextName string,
	projectRoot string,
	readiness canonicalProjectReadiness,
) (string, error) {
	data, err := artifact.FetchStatusData(ctx, store, contextName, projectRoot)
	if err != nil {
		return "", err
	}
	data.SpecBindingDebt = specBindingDebtReportForCanonicalStatus(
		ctx,
		store,
		readiness,
	)
	data = applyDefaultDriftEventResolutionLedgerToStatusData(ctx, store, projectRoot, data)
	driftEvents := governorDriftEvents(data)

	body := present.StatusGovernor(present.GovernorData{
		OverseerLine:               governorOverseerLine(projectRoot),
		ReconciliationLine:         present.ReconciliationCueSummary(data.ReconciliationCues),
		PendingCount:               len(data.PendingDecisions),
		UnassessedCount:            len(data.UnassessedDecisions),
		StaleCount:                 len(data.StaleItems),
		DriftEventCount:            driftEvents.Summary.UniqueEvents,
		DriftImpactedDecisionCount: driftEvents.Summary.ImpactedDecisions,
		DriftMaxFanout:             driftEvents.Summary.MaxFanout,
		TopAttention:               governorAttention(data),
		ActiveProblems:             governorProblems(data),
		OpenMethodRuns:             governorMethodRuns(ctx, store),
	})
	return statusProfilePrefix(readiness, false) + body, nil
}

func governorOverseerLine(projectRoot string) string {
	summary, err := overseer.LoadStatusSummary(projectRoot)
	if err != nil || !summary.HasSignals {
		return ""
	}

	high := 0
	for _, signal := range summary.Signals {
		if strings.EqualFold(signal.Severity, "high") {
			high++
		}
	}

	line := fmt.Sprintf("%d signal(s), %d high", len(summary.Signals), high)
	if summary.SuppressedCount > 0 {
		line += fmt.Sprintf(", %d suppressed", summary.SuppressedCount)
	}
	return line
}

func governorAttention(data artifact.StatusData) []string {
	items := []string{}
	for _, s := range data.StaleItems {
		items = append(items, fmt.Sprintf("refresh-due: %s `%s` — %s", s.Title, s.ID, s.Reason))
	}
	driftEvents := governorDriftEvents(data)
	if driftEvents.Summary.UniqueEvents > 0 {
		items = append(items, fmt.Sprintf(
			"drift-events: %d unique, %d impacted decision(s), max fanout %d — drill down: %s",
			driftEvents.Summary.UniqueEvents,
			driftEvents.Summary.ImpactedDecisions,
			driftEvents.Summary.MaxFanout,
			artifact.StatusCompactDriftEventsCommand,
		))
	}
	if data.SpecBindingDebt.Summary.Total() > 0 {
		summary := data.SpecBindingDebt.Summary
		items = append(items, fmt.Sprintf(
			"spec-binding-debt: missing=%d invalid_refs=%d draft_section_needed=%d out_of_spec=%d — drill down: haft_query(action=\"status\", full=true)",
			summary.DecisionsMissingSpecBinding,
			summary.DecisionsWithInvalidSpecRefs,
			summary.DraftSectionNeededDebt,
			summary.OutOfSpecDecisionDebt,
		))
	}
	return items
}

func governorDriftEvents(data artifact.StatusData) artifact.DriftEventReport {
	if len(data.DriftEvents.Events) > 0 || data.DriftEvents.SchemaVersion != 0 {
		return data.DriftEvents
	}
	if len(data.Drift) == 0 {
		return artifact.DriftEventReport{}
	}
	return artifact.BuildDriftEventReport(data.Drift)
}

func governorProblems(data artifact.StatusData) []string {
	items := []string{}
	for _, p := range data.InProgressProblems {
		items = append(items, fmt.Sprintf("%s `%s`", p.Meta.Title, p.Meta.ID))
	}
	return items
}

func governorMethodRuns(ctx context.Context, store *artifact.Store) []string {
	runs, err := methodpkg.OpenRuns(ctx, store, governorMethodRunLimit)
	if err != nil {
		return nil
	}

	items := []string{}
	for _, run := range runs {
		items = append(items, fmt.Sprintf("`%s` — %s", run.ID, run.TaskSignature.Task))
	}
	return items
}

const governorMethodRunLimit = 3
