package cli

import (
	"context"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/graph"
)

func scanGovernanceAttention(ctx context.Context, store *artifact.Store) artifact.GovernanceAttention {
	if store == nil {
		return artifact.GovernanceAttention{}
	}

	statusData, _ := artifact.FetchStatusData(ctx, store, "")
	attention := artifact.GovernanceAttention{
		BacklogCount:    len(statusData.BacklogProblems),
		InProgressCount: len(statusData.InProgressProblems),
	}

	problems, _ := store.ListByKind(ctx, artifact.KindProblemCard, 0)
	for _, problem := range problems {
		if problem.Meta.Status != artifact.StatusAddressed {
			continue
		}

		backlinks, err := store.GetBacklinks(ctx, problem.Meta.ID)
		if err != nil {
			continue
		}

		hasDecision := false
		for _, backlink := range backlinks {
			if backlink.Type != "based_on" {
				continue
			}
			linked, err := store.Get(ctx, backlink.Ref)
			if err != nil {
				continue
			}
			if linked.Meta.Kind == artifact.KindDecisionRecord {
				hasDecision = true
				break
			}
		}
		if hasDecision {
			continue
		}

		attention.AddressedWithoutDecision = append(attention.AddressedWithoutDecision, artifact.AddressedProblemGap{
			ProblemID: problem.Meta.ID,
			Title:     problem.Meta.Title,
		})
	}

	graphStore := graph.NewStore(store.DB())
	decisions, _ := store.ListByKind(ctx, artifact.KindDecisionRecord, 0)
	for _, decision := range decisions {
		if decision.Meta.Status != artifact.StatusActive && decision.Meta.Status != artifact.StatusRefreshDue {
			continue
		}

		results, err := graph.VerifyInvariants(ctx, graphStore, store.DB(), decision.Meta.ID)
		if err != nil {
			continue
		}

		for _, result := range results {
			if result.Status != graph.InvariantViolated {
				continue
			}
			attention.InvariantViolations = append(attention.InvariantViolations, artifact.InvariantViolationFinding{
				DecisionID:    decision.Meta.ID,
				DecisionTitle: decision.Meta.Title,
				Invariant:     result.Invariant.Text,
				Reason:        result.Reason,
			})
		}
	}

	return attention
}
