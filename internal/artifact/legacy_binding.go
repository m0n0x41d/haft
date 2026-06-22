package artifact

import (
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const (
	LegacyBindingSchemaVersion = "legacy_symbol_binding_report.v1"
	LegacyBindingAuthority     = "read_only_symbol_binding_proposal"

	LegacyBindingPostureCarrierOnly           = "carrier_or_generated_only"
	LegacyBindingPostureNoParseableSymbols    = "no_parseable_symbols"
	LegacyBindingPostureMissingSymbolBaseline = "missing_symbol_baseline"
	LegacyBindingPostureAmbiguousFileScope    = "ambiguous_file_scope"
	LegacyBindingPostureAlreadyPrecise        = "already_symbol_baselined"

	LegacyBindingActionNoAction            = "no_action"
	LegacyBindingActionProposeRebaseline   = "propose_rebaseline_with_symbols"
	LegacyBindingActionNeedsOperatorSelect = "needs_operator_symbol_selection"
	LegacyBindingActionKeepLegacyFileScope = "keep_legacy_file_scope"
)

const maxUsefulLegacySymbols = 8

type LegacyBindingReport struct {
	SchemaVersion string               `json:"schema_version"`
	Authority     string               `json:"authority"`
	Summary       LegacyBindingSummary `json:"summary"`
	Items         []LegacyBindingItem  `json:"items"`
}

type LegacyBindingSummary struct {
	TotalDecisions          int `json:"total_decisions"`
	AlreadyPrecise          int `json:"already_precise"`
	MissingSymbolBaseline   int `json:"missing_symbol_baseline"`
	AmbiguousFileScope      int `json:"ambiguous_file_scope"`
	HighConfidenceProposals int `json:"high_confidence_proposals"`
	NeedsOperatorSelection  int `json:"needs_operator_selection"`
	CarrierOrGeneratedOnly  int `json:"carrier_or_generated_only"`
	NoParseableSymbols      int `json:"no_parseable_symbols"`
}

type LegacyBindingItem struct {
	DecisionID           string                   `json:"decision_id"`
	DecisionTitle        string                   `json:"decision_title"`
	AffectedFiles        []string                 `json:"affected_files"`
	StoredSymbolCount    int                      `json:"stored_symbol_count"`
	CandidateSymbolCount int                      `json:"candidate_symbol_count"`
	Posture              string                   `json:"posture"`
	RecommendedAction    string                   `json:"recommended_action"`
	HighConfidence       bool                     `json:"high_confidence,omitempty"`
	Reason               string                   `json:"reason"`
	CandidateSymbols     []LegacyBindingCandidate `json:"candidate_symbols,omitempty"`
}

type LegacyBindingCandidate struct {
	FilePath   string `json:"file_path"`
	SymbolName string `json:"symbol_name"`
	SymbolKind string `json:"symbol_kind"`
	Line       int    `json:"line"`
	EndLine    int    `json:"end_line"`
}

type LegacyBindingOptions struct {
	CandidateLimit int
	IncludeClean   bool
}

func BuildLegacyBindingReport(
	ctx context.Context,
	store ArtifactStore,
	projectRoot string,
	options LegacyBindingOptions,
) (LegacyBindingReport, error) {
	limit := options.CandidateLimit
	if limit <= 0 {
		limit = 20
	}

	decisions, err := store.ListActiveByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		return LegacyBindingReport{}, fmt.Errorf("list decisions: %w", err)
	}

	report := LegacyBindingReport{
		SchemaVersion: LegacyBindingSchemaVersion,
		Authority:     LegacyBindingAuthority,
		Items:         []LegacyBindingItem{},
	}

	for _, decision := range decisions {
		files, err := store.GetAffectedFiles(ctx, decision.Meta.ID)
		if err != nil || len(files) == 0 {
			continue
		}
		symbols, err := store.GetAffectedSymbols(ctx, decision.Meta.ID)
		if err != nil {
			return LegacyBindingReport{}, fmt.Errorf("get affected symbols for %s: %w", decision.Meta.ID, err)
		}

		item := buildLegacyBindingItem(projectRoot, decision, files, symbols, limit)
		report.Summary.TotalDecisions++
		countLegacyBindingSummary(&report.Summary, item)
		if item.RecommendedAction == LegacyBindingActionNoAction && !options.IncludeClean {
			continue
		}
		report.Items = append(report.Items, item)
	}

	sort.Slice(report.Items, func(i, j int) bool {
		left := legacyBindingActionRank(report.Items[i].RecommendedAction)
		right := legacyBindingActionRank(report.Items[j].RecommendedAction)
		if left != right {
			return left < right
		}
		return report.Items[i].DecisionID < report.Items[j].DecisionID
	})

	return report, nil
}

func buildLegacyBindingItem(
	projectRoot string,
	decision *Artifact,
	files []AffectedFile,
	stored []AffectedSymbol,
	candidateLimit int,
) LegacyBindingItem {
	paths := affectedFilePaths(files)
	candidates := legacyBindingCandidates(projectRoot, files, candidateLimit)

	item := LegacyBindingItem{
		DecisionID:           decision.Meta.ID,
		DecisionTitle:        decision.Meta.Title,
		AffectedFiles:        paths,
		StoredSymbolCount:    len(stored),
		CandidateSymbolCount: len(candidates),
		CandidateSymbols:     candidates,
	}

	if allCarrierOrGenerated(paths) {
		item.Posture = LegacyBindingPostureCarrierOnly
		item.RecommendedAction = LegacyBindingActionNoAction
		item.Reason = "affected files are carrier/generated paths; no symbol binding needed"
		item.CandidateSymbols = nil
		return item
	}

	if len(stored) == 0 {
		return missingLegacySymbolBaseline(item)
	}

	if len(stored) > maxUsefulLegacySymbols || len(files) > 1 {
		item.Posture = LegacyBindingPostureAmbiguousFileScope
		item.RecommendedAction = LegacyBindingActionNeedsOperatorSelect
		item.Reason = "stored symbol baseline is broad; operator should select the governed symbols before rebaseline"
		return item
	}

	item.Posture = LegacyBindingPostureAlreadyPrecise
	item.RecommendedAction = LegacyBindingActionNoAction
	item.Reason = "decision already has a compact symbol baseline"
	item.CandidateSymbols = nil
	return item
}

func missingLegacySymbolBaseline(item LegacyBindingItem) LegacyBindingItem {
	switch {
	case item.CandidateSymbolCount == 0:
		item.Posture = LegacyBindingPostureNoParseableSymbols
		item.RecommendedAction = LegacyBindingActionKeepLegacyFileScope
		item.Reason = "affected files expose no parseable symbols; keep explicit legacy file scope"
	case item.CandidateSymbolCount == 1:
		item.Posture = LegacyBindingPostureMissingSymbolBaseline
		item.RecommendedAction = LegacyBindingActionProposeRebaseline
		item.HighConfidence = true
		item.Reason = "exactly one parseable symbol in affected files; safe to propose symbol-baseline rebaseline"
	default:
		item.Posture = LegacyBindingPostureMissingSymbolBaseline
		item.RecommendedAction = LegacyBindingActionNeedsOperatorSelect
		item.Reason = "multiple parseable symbols found; operator must choose governed symbol boundary"
	}
	return item
}

func legacyBindingCandidates(projectRoot string, files []AffectedFile, limit int) []LegacyBindingCandidate {
	out := []LegacyBindingCandidate{}
	for _, file := range files {
		if carrierOnlyPath(file.Path) || generatedOrIgnoredPath(file.Path) {
			continue
		}
		snaps, err := codebase.ExtractSymbolSnapshots(projectRoot, file.Path)
		if err != nil {
			continue
		}
		for _, snap := range snaps {
			out = append(out, LegacyBindingCandidate{
				FilePath:   snap.FilePath,
				SymbolName: snap.SymbolName,
				SymbolKind: snap.SymbolKind,
				Line:       snap.Line,
				EndLine:    snap.EndLine,
			})
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func affectedFilePaths(files []AffectedFile) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.Path)
	}
	sort.Strings(out)
	return out
}

func allCarrierOrGenerated(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if !carrierOnlyPath(path) && !generatedOrIgnoredPath(path) {
			return false
		}
	}
	return true
}

func countLegacyBindingSummary(summary *LegacyBindingSummary, item LegacyBindingItem) {
	switch item.Posture {
	case LegacyBindingPostureAlreadyPrecise:
		summary.AlreadyPrecise++
	case LegacyBindingPostureMissingSymbolBaseline:
		summary.MissingSymbolBaseline++
	case LegacyBindingPostureAmbiguousFileScope:
		summary.AmbiguousFileScope++
	case LegacyBindingPostureCarrierOnly:
		summary.CarrierOrGeneratedOnly++
	case LegacyBindingPostureNoParseableSymbols:
		summary.NoParseableSymbols++
	}
	if item.HighConfidence {
		summary.HighConfidenceProposals++
	}
	if item.RecommendedAction == LegacyBindingActionNeedsOperatorSelect {
		summary.NeedsOperatorSelection++
	}
}

func legacyBindingActionRank(action string) int {
	switch action {
	case LegacyBindingActionProposeRebaseline:
		return 0
	case LegacyBindingActionNeedsOperatorSelect:
		return 1
	case LegacyBindingActionKeepLegacyFileScope:
		return 2
	default:
		return 3
	}
}
