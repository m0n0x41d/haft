package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func applyDecisionSpecBindingPreflight(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	input artifact.DecideInput,
) (artifact.DecideInput, error) {
	if !decisionNeedsSpecBindingPreflight(input) {
		return input, nil
	}
	if input.SpecBindingPreflight != nil {
		input.SpecBindingRequired = true
		return input, nil
	}

	projectRoot := filepath.Dir(haftDir)
	specSet, err := loadProjectSpecificationSetSQLFirst(projectRoot)
	if err != nil {
		return artifact.DecideInput{}, fmt.Errorf("load ProjectSpecificationSet for decision spec binding preflight: %w", err)
	}

	draft := specBindingDecisionDraftFromDecideInput(input)
	draft = enrichSpecBindingDecisionDraft(ctx, store, draft)
	result := specflow.BuildSpecBindingPreflight(specSet, specflow.SpecBindingPreflightInput{
		DecisionDraft: draft,
	})

	input.SpecBindingPreflight = specBindingPreflightReceiptFromSpecflow(result)
	input.SpecBindingRequired = true
	return input, nil
}

func decisionNeedsSpecBindingPreflight(input artifact.DecideInput) bool {
	if input.SpecBindingRequired || input.SpecBindingPreflight != nil || len(input.SectionRefs) > 0 {
		return true
	}

	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = string(artifact.ModeStandard)
	}
	return mode != string(artifact.ModeTactical) && mode != string(artifact.ModeNote)
}

func specBindingDecisionDraftFromDecideInput(input artifact.DecideInput) specflow.SpecBindingDecisionDraft {
	return specflow.SpecBindingDecisionDraft{
		SelectedTitle:         input.SelectedTitle,
		WhySelected:           input.WhySelected,
		CounterArgument:       input.CounterArgument,
		WeakestLink:           input.WeakestLink,
		Mode:                  input.Mode,
		DecisionSubjectRef:    input.DecisionSubjectRef,
		ProblemRefs:           artifact.MergeProblemRefs(input.ProblemRef, input.ProblemRefs),
		PortfolioRef:          input.PortfolioRef,
		SearchKeywords:        input.SearchKeywords,
		SectionRefs:           input.SectionRefs,
		AffectedFiles:         input.AffectedFiles,
		BindingHints:          input.BindingHints,
		BindingTargetRefs:     bindingTargetRefsFromDecisionInput(input),
		GovernanceTargetRefs:  governanceTargetRefsFromDecisionInput(input),
		BindingScope:          input.BindingScope,
		BindingFallbackReason: input.BindingFallbackReason,
	}
}

func enrichSpecBindingDecisionDraft(
	ctx context.Context,
	store *artifact.Store,
	draft specflow.SpecBindingDecisionDraft,
) specflow.SpecBindingDecisionDraft {
	if store == nil {
		return draft
	}
	draft = withProblemPortfolioLinkedSectionRefs(ctx, store, draft)
	draft = withActiveDecisionLineageSectionRefs(ctx, store, draft)
	return draft
}

func withProblemPortfolioLinkedSectionRefs(
	ctx context.Context,
	store *artifact.Store,
	draft specflow.SpecBindingDecisionDraft,
) specflow.SpecBindingDecisionDraft {
	refs := append([]string(nil), draft.ProblemRefs...)
	if draft.PortfolioRef != "" {
		refs = append(refs, draft.PortfolioRef)
		if portfolio, err := store.Get(ctx, draft.PortfolioRef); err == nil {
			refs = append(refs, artifact.ResolvePortfolioProblemRefs(portfolio)...)
		}
	}
	for _, ref := range compactSortedStrings(refs) {
		backlinks, err := store.GetBacklinks(ctx, ref)
		if err != nil {
			continue
		}
		for _, backlink := range backlinks {
			item, err := store.Get(ctx, backlink.Ref)
			if err != nil || item.Meta.Kind != artifact.KindDecisionRecord {
				continue
			}
			if !specBindingDecisionStatusInScope(item.Meta.Status) {
				continue
			}
			fields := item.UnmarshalDecisionFields()
			draft.ActiveDecisionRefs = append(draft.ActiveDecisionRefs, item.Meta.ID)
			draft.LinkedSectionRefs = append(draft.LinkedSectionRefs, fields.SectionRefs...)
		}
	}
	return draft
}

func withActiveDecisionLineageSectionRefs(
	ctx context.Context,
	store *artifact.Store,
	draft specflow.SpecBindingDecisionDraft,
) specflow.SpecBindingDecisionDraft {
	report, err := artifact.BuildCurrentGoverningSetReport(ctx, store)
	if err != nil {
		return draft
	}
	targetRefs := specBindingDraftTargetRefsForLineage(draft)
	for _, set := range report.Sets {
		if !targetRefs[set.TargetRef] && !specBindingSetIntersectsDecisions(set, draft.ActiveDecisionRefs) {
			continue
		}
		for _, decisionRef := range set.CurrentDecisionRefs {
			item, err := store.Get(ctx, decisionRef)
			if err != nil || item.Meta.Kind != artifact.KindDecisionRecord {
				continue
			}
			fields := item.UnmarshalDecisionFields()
			draft.ActiveDecisionRefs = append(draft.ActiveDecisionRefs, decisionRef)
			draft.LineageSectionRefs = append(draft.LineageSectionRefs, fields.SectionRefs...)
		}
	}
	return draft
}

func specBindingDraftTargetRefsForLineage(
	draft specflow.SpecBindingDecisionDraft,
) map[string]bool {
	out := map[string]bool{}
	values := []string{draft.DecisionSubjectRef, draft.PortfolioRef}
	values = append(values, draft.ProblemRefs...)
	values = append(values, draft.AffectedFiles...)
	values = append(values, draft.BindingTargetRefs...)
	values = append(values, draft.GovernanceTargetRefs...)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[trimmed] = true
	}
	return out
}

func specBindingSetIntersectsDecisions(
	set artifact.CurrentGoverningSet,
	decisionRefs []string,
) bool {
	wanted := map[string]bool{}
	for _, ref := range decisionRefs {
		trimmed := strings.TrimSpace(ref)
		if trimmed != "" {
			wanted[trimmed] = true
		}
	}
	for _, ref := range set.CurrentDecisionRefs {
		if wanted[ref] {
			return true
		}
	}
	return false
}

func specBindingDecisionStatusInScope(status artifact.Status) bool {
	return status == artifact.StatusActive || status == artifact.StatusRefreshDue
}

func bindingTargetRefsFromDecisionInput(input artifact.DecideInput) []string {
	refs := []string{}
	for _, target := range input.BindingTargets {
		if ref := strings.TrimSpace(target.TargetRef); ref != "" {
			refs = append(refs, ref)
			continue
		}
		if path := strings.TrimSpace(target.FilePath); path != "" {
			refs = append(refs, path)
		}
	}
	return refs
}

func governanceTargetRefsFromDecisionInput(input artifact.DecideInput) []string {
	refs := []string{}
	for _, target := range input.GovernanceTargets {
		if ref := strings.TrimSpace(target.Ref); ref != "" {
			refs = append(refs, ref)
			continue
		}
		if target.BindingTarget != nil {
			refs = append(refs, bindingTargetRefsFromDecisionInput(artifact.DecideInput{
				BindingTargets: []artifact.BindingTarget{*target.BindingTarget},
			})...)
		}
	}
	return refs
}

func specBindingPreflightReceiptFromSpecflow(
	result specflow.SpecBindingPreflightResult,
) *artifact.SpecBindingPreflight {
	return &artifact.SpecBindingPreflight{
		SchemaVersion:          result.SchemaVersion,
		RecordKind:             result.RecordKind,
		ProjectSpecState:       result.ProjectSpecState,
		DecisionMode:           result.DecisionMode,
		LoadBearingLevel:       result.LoadBearingLevel,
		State:                  result.State,
		SelectedSectionRefs:    result.SelectedSectionRefs,
		ConflictRefs:           result.ConflictRefs,
		OperatorActionRequired: result.OperatorActionRequired,
		StatusDebt: artifact.SpecBindingStatusDebt{
			Severity: result.StatusDebt.Severity,
			Message:  result.StatusDebt.Message,
		},
		AuthorityBoundary:   result.AuthorityBoundary,
		DecisionDraftDigest: result.DecisionDraftDigest,
	}
}
