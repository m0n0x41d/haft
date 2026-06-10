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
) (string, error) {
	data, err := artifact.FetchStatusData(ctx, store, contextName, projectRoot)
	if err != nil {
		return "", err
	}

	return present.StatusGovernor(present.GovernorData{
		OverseerLine:    governorOverseerLine(projectRoot),
		PendingCount:    len(data.PendingDecisions),
		UnassessedCount: len(data.UnassessedDecisions),
		StaleCount:      len(data.StaleItems),
		DriftCount:      len(data.Drift),
		TopAttention:    governorAttention(data),
		ActiveProblems:  governorProblems(data),
		OpenMethodRuns:  governorMethodRuns(ctx, store),
	}), nil
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
	for _, r := range data.Drift {
		items = append(items, fmt.Sprintf("drift: %s `%s` — %d file(s)", r.DecisionTitle, r.DecisionID, len(r.Files)))
	}
	return items
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
