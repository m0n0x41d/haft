package artifact

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NavState holds the computed navigation state for a context.
type NavState struct {
	Context       string
	Mode          Mode
	DerivedStatus DerivedStatus
	ProblemTitle  string
	ProblemStatus string
	PortfolioInfo string
	DecisionInfo  string
	StaleCount    int
	StaleItems    []string
	NextAction    string
}

// BuildNavStrip computes the current state from the artifact store and formats it.
func BuildNavStrip(ctx context.Context, store *Store, contextName string) string {
	state := ComputeNavState(ctx, store, contextName)
	return FormatNavStrip(state)
}

// ComputeNavState derives the current state from artifact completeness.
func ComputeNavState(ctx context.Context, store *Store, contextName string) NavState {
	state := NavState{Context: contextName}

	var artifacts []*Artifact
	var err error
	if contextName != "" {
		artifacts, err = store.ListByContext(ctx, contextName)
	} else {
		artifacts, err = store.ListActive(ctx, 100)
	}
	if err != nil || len(artifacts) == 0 {
		state.DerivedStatus = DerivedUnderframed
		state.NextAction = `quint_problem(action="frame", title="...")`
		return state
	}

	var problems, portfolios, decisions []*Artifact
	for _, a := range artifacts {
		switch a.Meta.Kind {
		case KindProblemCard:
			problems = append(problems, a)
		case KindSolutionPortfolio:
			portfolios = append(portfolios, a)
		case KindDecisionRecord:
			decisions = append(decisions, a)
		}
	}

	// Derive status from what exists
	switch {
	case len(decisions) > 0:
		state.DerivedStatus = DerivedDecided
		d := decisions[0]
		state.DecisionInfo = d.Meta.Title
		if len(decisions) > 1 {
			state.DecisionInfo += fmt.Sprintf(" (+%d more)", len(decisions)-1)
		}
		if d.Meta.Status == StatusRefreshDue {
			state.DerivedStatus = DerivedRefreshDue
		}
		state.Mode = d.Meta.Mode
	case len(portfolios) > 0:
		p := portfolios[0]
		if strings.Contains(p.Body, "## Comparison") || strings.Contains(p.Body, "## Non-Dominated Set") {
			state.DerivedStatus = DerivedCompared
			state.NextAction = `quint_decision(action="decide", ...)`
		} else {
			state.DerivedStatus = DerivedExploring
			state.NextAction = `quint_solution(action="compare", ...)`
		}
		state.PortfolioInfo = p.Meta.Title
		if len(portfolios) > 1 {
			state.PortfolioInfo += fmt.Sprintf(" (+%d more)", len(portfolios)-1)
		}
		state.Mode = p.Meta.Mode
	case len(problems) > 0:
		state.DerivedStatus = DerivedFramed
		state.ProblemTitle = problems[0].Meta.Title
		state.ProblemStatus = string(problems[0].Meta.Status)
		if len(problems) > 1 {
			state.ProblemStatus += fmt.Sprintf(", +%d more", len(problems)-1)
		}
		state.NextAction = `quint_solution(action="explore", ...)`
		state.Mode = problems[0].Meta.Mode
	default:
		state.DerivedStatus = DerivedUnderframed
		state.NextAction = `quint_problem(action="frame", title="...")`
	}

	// Check for stale decisions
	stale, err := store.FindStaleDecisions(ctx)
	if err == nil && len(stale) > 0 {
		state.StaleCount = len(stale)
		now := time.Now().UTC()
		for _, s := range stale {
			reason := "refresh_due"
			if s.Meta.ValidUntil != "" {
				if t, err := time.Parse(time.RFC3339, s.Meta.ValidUntil); err == nil && t.Before(now) {
					reason = fmt.Sprintf("expired %s", t.Format("2006-01-02"))
				}
			}
			state.StaleItems = append(state.StaleItems, fmt.Sprintf("%s: %s (%s)", s.Meta.ID, s.Meta.Title, reason))
		}
		if state.DerivedStatus == DerivedDecided {
			state.DerivedStatus = DerivedRefreshDue
			state.NextAction = "quint_refresh(...)"
		}
	}

	return state
}

// FormatNavStrip renders the nav state as a compact text block.
func FormatNavStrip(state NavState) string {
	var sb strings.Builder

	sb.WriteString("\n── Quint ──────────────────────────\n")

	if state.Context != "" {
		sb.WriteString(fmt.Sprintf("Context: %s\n", state.Context))
	}
	if state.Mode != "" {
		sb.WriteString(fmt.Sprintf("Mode: %s\n", state.Mode))
	}

	sb.WriteString(fmt.Sprintf("Status: %s\n", state.DerivedStatus))

	if state.ProblemTitle != "" {
		sb.WriteString(fmt.Sprintf("Problem: %s", state.ProblemTitle))
		if state.ProblemStatus != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", state.ProblemStatus))
		}
		sb.WriteString("\n")
	}
	if state.PortfolioInfo != "" {
		sb.WriteString(fmt.Sprintf("Portfolio: %s\n", state.PortfolioInfo))
	}
	if state.DecisionInfo != "" {
		sb.WriteString(fmt.Sprintf("Decision: %s\n", state.DecisionInfo))
	}

	if state.StaleCount > 0 {
		sb.WriteString(fmt.Sprintf("Stale: %d decision(s) need refresh\n", state.StaleCount))
	}

	if state.NextAction != "" {
		sb.WriteString(fmt.Sprintf("Next: %s\n", state.NextAction))
	}

	sb.WriteString("───────────────────────────────────\n")

	return sb.String()
}
