package cli

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func applyProblemSpecFit(
	_ context.Context,
	_ *artifact.Store,
	haftDir string,
	input artifact.ProblemFrameInput,
) artifact.ProblemFrameInput {
	if input.SpecFit != nil {
		return input
	}
	record, ok := buildSpecFitRecord(haftDir, specflow.SpecFitProbeInput{
		ProblemSignal: firstNonEmptyString(input.Signal, input.Title),
		Scope:         firstNonEmptyString(input.Scope, input.BlastRadius, input.Context),
		Mode:          input.Mode,
	})
	if !ok {
		return input
	}
	input.SpecFit = record
	return input
}

func applyExploreSpecFit(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	input artifact.ExploreInput,
) artifact.ExploreInput {
	if input.SpecFit != nil {
		return input
	}
	problemSignal, problemScope := specFitProblemContext(ctx, store, input.ProblemRef)
	record, ok := buildSpecFitRecord(haftDir, specflow.SpecFitProbeInput{
		ProblemSignal: firstNonEmptyString(problemSignal, input.Context),
		Scope:         problemScope,
		Mode:          input.Mode,
		Variants:      specFitVariantsFromExplore(input.Variants),
	})
	if !ok {
		return input
	}
	input.SpecFit = record
	return input
}

func applyCompareSpecFit(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	input artifact.CompareInput,
) artifact.CompareInput {
	if input.SpecFit != nil {
		input.Results.VariantSpecFit = cloneArtifactSpecFitVariants(input.SpecFit.VariantSpecFit)
		return input
	}
	problemSignal, problemScope := specFitPortfolioContext(ctx, store, input.PortfolioRef)
	record, ok := buildSpecFitRecord(haftDir, specflow.SpecFitProbeInput{
		ProblemSignal: problemSignal,
		Scope:         problemScope,
		Variants:      specFitVariantsFromPortfolio(ctx, store, input.PortfolioRef),
	})
	if !ok {
		return input
	}
	input.SpecFit = record
	input.Results.VariantSpecFit = cloneArtifactSpecFitVariants(record.VariantSpecFit)
	return input
}

func buildSpecFitRecord(
	haftDir string,
	input specflow.SpecFitProbeInput,
) (*artifact.SpecFitRecord, bool) {
	specSet, err := loadProjectSpecificationSetSQLFirst(filepath.Dir(haftDir))
	if err != nil || !specFitHasActiveSections(specSet.Sections) {
		return nil, false
	}
	result := specflow.BuildSpecFitProbe(specSet, input)
	return specFitRecordFromSpecflow(result), true
}

func specFitHasActiveSections(sections []project.SpecSection) bool {
	for _, section := range sections {
		if strings.TrimSpace(section.Status) == string(project.SpecSectionStateActive) {
			return true
		}
	}
	return false
}

func specFitProblemContext(
	ctx context.Context,
	store *artifact.Store,
	problemRef string,
) (string, string) {
	if store == nil || strings.TrimSpace(problemRef) == "" {
		return "", ""
	}
	problem, err := store.Get(ctx, problemRef)
	if err != nil || problem.Meta.Kind != artifact.KindProblemCard {
		return "", ""
	}
	fields := problem.UnmarshalProblemFields()
	scope := ""
	if fields.Profile != nil {
		scope = fields.Profile.Scope
	}
	return fields.Signal, firstNonEmptyString(scope, fields.BlastRadius, problem.Meta.Context)
}

func specFitPortfolioContext(
	ctx context.Context,
	store *artifact.Store,
	portfolioRef string,
) (string, string) {
	if store == nil || strings.TrimSpace(portfolioRef) == "" {
		return "", ""
	}
	portfolio, err := store.Get(ctx, portfolioRef)
	if err != nil || portfolio.Meta.Kind != artifact.KindSolutionPortfolio {
		return portfolioRef, ""
	}
	for _, problemRef := range artifact.ResolvePortfolioProblemRefs(portfolio) {
		signal, scope := specFitProblemContext(ctx, store, problemRef)
		if signal != "" || scope != "" {
			return signal, scope
		}
	}
	return portfolio.Meta.Title, portfolio.Meta.Context
}

func specFitVariantsFromExplore(
	variants []artifact.Variant,
) []specflow.SpecFitVariantInput {
	out := make([]specflow.SpecFitVariantInput, 0, len(variants))
	for _, variant := range variants {
		out = append(out, specflow.SpecFitVariantInput{
			ID:          variant.ID,
			Title:       variant.Title,
			Description: firstNonEmptyString(variant.Description, variant.AssumptionNotes),
		})
	}
	return out
}

func specFitVariantsFromPortfolio(
	ctx context.Context,
	store *artifact.Store,
	portfolioRef string,
) []specflow.SpecFitVariantInput {
	if store == nil || strings.TrimSpace(portfolioRef) == "" {
		return nil
	}
	portfolio, err := store.Get(ctx, portfolioRef)
	if err != nil || portfolio.Meta.Kind != artifact.KindSolutionPortfolio {
		return nil
	}
	fields := portfolio.UnmarshalPortfolioFields()
	return specFitVariantsFromExplore(fields.Variants)
}

func specFitRecordFromSpecflow(
	result specflow.SpecFitProbeResult,
) *artifact.SpecFitRecord {
	return &artifact.SpecFitRecord{
		SchemaVersion:        result.SchemaVersion,
		RecordKind:           result.RecordKind,
		Authority:            result.Authority,
		AuthorityBoundary:    result.AuthorityBoundary,
		State:                result.State,
		CandidateSectionRefs: append([]string(nil), result.CandidateSectionRefs...),
		ConflictRefs:         append([]string(nil), result.ConflictRefs...),
		NextExpectedAction:   result.NextExpectedAction,
		VariantSpecFit:       specFitVariantsFromSpecflow(result.VariantSpecFit),
	}
}

func specFitVariantsFromSpecflow(
	items []specflow.SpecFitVariantResult,
) []artifact.SpecFitVariantRecord {
	out := make([]artifact.SpecFitVariantRecord, 0, len(items))
	for _, item := range items {
		out = append(out, artifact.SpecFitVariantRecord{
			VariantRef:     item.VariantRef,
			State:          item.State,
			SectionRefs:    append([]string(nil), item.SectionRefs...),
			ConflictRefs:   append([]string(nil), item.ConflictRefs...),
			ProposedDelta:  item.ProposedDelta,
			ExpectedAction: item.ExpectedAction,
		})
	}
	return out
}

func cloneArtifactSpecFitVariants(
	items []artifact.SpecFitVariantRecord,
) []artifact.SpecFitVariantRecord {
	out := make([]artifact.SpecFitVariantRecord, 0, len(items))
	for _, item := range items {
		cloned := item
		cloned.SectionRefs = append([]string(nil), item.SectionRefs...)
		cloned.ConflictRefs = append([]string(nil), item.ConflictRefs...)
		out = append(out, cloned)
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
