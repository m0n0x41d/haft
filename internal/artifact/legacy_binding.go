package artifact

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
)

const (
	LegacyBindingSchemaVersion = "legacy_binding_report.v2"
	LegacyBindingAuthority     = "binding_target_review_proposal"

	LegacyBindingPostureCarrierOnly           = "carrier_or_generated_only"
	LegacyBindingPostureNoParseableSymbols    = "no_parseable_symbols"
	LegacyBindingPostureMissingSymbolBaseline = "missing_symbol_baseline"
	LegacyBindingPostureAmbiguousFileScope    = "ambiguous_file_scope"
	LegacyBindingPostureAlreadyPrecise        = "already_symbol_baselined"
	LegacyBindingPostureMissingBindingTargets = "missing_binding_targets"

	LegacyBindingActionNoAction            = "no_action"
	LegacyBindingActionProposeRebaseline   = "propose_rebaseline_with_binding_targets"
	LegacyBindingActionNeedsOperatorSelect = "needs_operator_symbol_selection"
	LegacyBindingActionKeepLegacyFileScope = "keep_legacy_file_scope"
)

const maxUsefulLegacySymbols = 8

type LegacyBindingReport struct {
	SchemaVersion string               `json:"schema_version"`
	Authority     string               `json:"authority"`
	Summary       LegacyBindingSummary `json:"summary"`
	Items         []LegacyBindingItem  `json:"items"`
	Applied       []LegacyBindingApply `json:"applied,omitempty"`
}

type LegacyBindingSummary struct {
	TotalDecisions          int `json:"total_decisions"`
	AlreadyPrecise          int `json:"already_precise"`
	MissingSymbolBaseline   int `json:"missing_symbol_baseline"`
	MissingBindingTargets   int `json:"missing_binding_targets"`
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
	BindingTargets       []BindingTarget          `json:"binding_targets,omitempty"`
	Diagnostics          []BindingDiagnostic      `json:"diagnostics,omitempty"`
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

type LegacyBindingApplyOptions struct {
	CandidateLimit int
}

type LegacyBindingSelectionDocument struct {
	SchemaVersion string                   `json:"schema_version,omitempty"`
	Authority     string                   `json:"authority,omitempty"`
	Items         []LegacyBindingSelection `json:"items"`
}

type LegacyBindingSelection struct {
	DecisionID      string          `json:"decision_id"`
	DecisionTitle   string          `json:"decision_title,omitempty"`
	BindingTargets  []BindingTarget `json:"binding_targets"`
	ReviewRationale string          `json:"review_rationale,omitempty"`
}

type LegacyBindingApply struct {
	DecisionID         string   `json:"decision_id"`
	DecisionTitle      string   `json:"decision_title"`
	BindingTargetKinds []string `json:"binding_target_kinds"`
	SymbolCount        int      `json:"symbol_count"`
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
		fullDecision, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			return LegacyBindingReport{}, fmt.Errorf("get decision %s: %w", decision.Meta.ID, err)
		}

		files, err := store.GetAffectedFiles(ctx, fullDecision.Meta.ID)
		if err != nil || len(files) == 0 {
			continue
		}
		symbols, err := store.GetAffectedSymbols(ctx, fullDecision.Meta.ID)
		if err != nil {
			return LegacyBindingReport{}, fmt.Errorf("get affected symbols for %s: %w", fullDecision.Meta.ID, err)
		}

		item := buildLegacyBindingItem(projectRoot, fullDecision, files, symbols, limit)
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

func ApplyHighConfidenceLegacyBindingRepairs(
	ctx context.Context,
	store ArtifactStore,
	projectRoot string,
	options LegacyBindingApplyOptions,
) (LegacyBindingReport, error) {
	report, err := BuildLegacyBindingReport(ctx, store, projectRoot, LegacyBindingOptions{
		CandidateLimit: options.CandidateLimit,
		IncludeClean:   false,
	})
	if err != nil {
		return LegacyBindingReport{}, err
	}

	for _, item := range report.Items {
		if !item.HighConfidence || item.RecommendedAction != LegacyBindingActionProposeRebaseline {
			continue
		}
		if len(item.Diagnostics) > 0 || len(item.BindingTargets) == 0 {
			continue
		}

		artifact, err := store.Get(ctx, item.DecisionID)
		if err != nil {
			return LegacyBindingReport{}, fmt.Errorf("get decision %s: %w", item.DecisionID, err)
		}

		fields := artifact.UnmarshalDecisionFields()
		fields.BindingTargets = normalizeBindingTargets(item.BindingTargets)
		fields.BindingDiagnostics = nil

		if err := persistDecisionFields(ctx, store, artifact, fields); err != nil {
			return LegacyBindingReport{}, fmt.Errorf("persist binding targets for %s: %w", item.DecisionID, err)
		}

		symbols := affectedSymbolsFromBindingTargets(item.BindingTargets)
		if err := store.SetAffectedSymbols(ctx, item.DecisionID, symbols); err != nil {
			return LegacyBindingReport{}, fmt.Errorf("persist affected symbols for %s: %w", item.DecisionID, err)
		}

		report.Applied = append(report.Applied, LegacyBindingApply{
			DecisionID:         item.DecisionID,
			DecisionTitle:      item.DecisionTitle,
			BindingTargetKinds: bindingTargetKinds(item.BindingTargets),
			SymbolCount:        len(symbols),
		})
	}

	return report, nil
}

func ApplyLegacyBindingSelections(
	ctx context.Context,
	store ArtifactStore,
	document LegacyBindingSelectionDocument,
) (LegacyBindingReport, error) {
	report := LegacyBindingReport{
		SchemaVersion: LegacyBindingSchemaVersion,
		Authority:     LegacyBindingAuthority,
	}

	for _, selection := range document.Items {
		if strings.TrimSpace(selection.DecisionID) == "" {
			return LegacyBindingReport{}, fmt.Errorf("binding selection is missing decision_id")
		}
		if len(selection.BindingTargets) == 0 {
			return LegacyBindingReport{}, fmt.Errorf("binding selection for %s is missing binding_targets", selection.DecisionID)
		}

		decisionArtifact, err := store.Get(ctx, selection.DecisionID)
		if err != nil {
			return LegacyBindingReport{}, fmt.Errorf("get decision %s: %w", selection.DecisionID, err)
		}
		if decisionArtifact.Meta.Kind != KindDecisionRecord {
			return LegacyBindingReport{}, fmt.Errorf("%s is %s, not a DecisionRecord", selection.DecisionID, decisionArtifact.Meta.Kind)
		}

		targets := normalizeBindingTargets(selection.BindingTargets)
		fields := decisionArtifact.UnmarshalDecisionFields()
		fields.BindingTargets = targets
		fields.BindingDiagnostics = nil

		if err := persistDecisionFields(ctx, store, decisionArtifact, fields); err != nil {
			return LegacyBindingReport{}, fmt.Errorf("persist binding targets for %s: %w", selection.DecisionID, err)
		}

		symbols := affectedSymbolsFromBindingTargets(targets)
		if err := store.SetAffectedSymbols(ctx, selection.DecisionID, symbols); err != nil {
			return LegacyBindingReport{}, fmt.Errorf("persist affected symbols for %s: %w", selection.DecisionID, err)
		}

		report.Applied = append(report.Applied, LegacyBindingApply{
			DecisionID:         selection.DecisionID,
			DecisionTitle:      decisionArtifact.Meta.Title,
			BindingTargetKinds: bindingTargetKinds(targets),
			SymbolCount:        len(symbols),
		})
	}

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
	resolution, resolutionErr := ResolveBindingTargets(projectRoot, files, BindingResolutionOptions{})

	item := LegacyBindingItem{
		DecisionID:           decision.Meta.ID,
		DecisionTitle:        decision.Meta.Title,
		AffectedFiles:        paths,
		StoredSymbolCount:    len(stored),
		CandidateSymbolCount: len(candidates),
		CandidateSymbols:     candidates,
	}
	if resolutionErr == nil {
		item.BindingTargets = resolution.Targets
	}
	item.Diagnostics = resolution.Diagnostics

	fields := decision.UnmarshalDecisionFields()
	if allCarrierOrGenerated(paths) {
		item.Posture = LegacyBindingPostureCarrierOnly
		item.RecommendedAction = LegacyBindingActionNoAction
		item.Reason = "affected files are carrier/generated paths; no symbol binding needed"
		item.CandidateSymbols = nil
		return item
	}

	if len(fields.BindingTargets) > 0 {
		item.BindingTargets = fields.BindingTargets
		item.CandidateSymbols = nil
		if bindingTargetsNeedResolution(fields.BindingTargets) {
			item.Posture = LegacyBindingPostureMissingBindingTargets
			item.RecommendedAction = LegacyBindingActionNeedsOperatorSelect
			item.Reason = "decision has binding_targets, but at least one target is still a whole-file fallback"
			return item
		}
		item.Posture = LegacyBindingPostureAlreadyPrecise
		item.RecommendedAction = LegacyBindingActionNoAction
		item.Reason = "decision already has explicit binding_targets"
		return item
	}

	if len(stored) == 0 {
		return missingLegacySymbolBaseline(item)
	}

	if len(stored) == 1 {
		item.Posture = LegacyBindingPostureMissingBindingTargets
		item.RecommendedAction = LegacyBindingActionProposeRebaseline
		item.HighConfidence = true
		item.Reason = "stored symbol baseline has exactly one symbol; safe to project it into binding_targets"
		item.BindingTargets = []BindingTarget{bindingTargetFromAffectedSymbol(stored[0])}
		item.CandidateSymbols = nil
		return item
	}

	if singleHighConfidenceTarget(resolution, resolutionErr) {
		item.Posture = LegacyBindingPostureMissingBindingTargets
		item.RecommendedAction = LegacyBindingActionProposeRebaseline
		item.HighConfidence = true
		item.Reason = "stored symbol baseline exists but binding_targets are missing; resolver produced one precise target"
		item.CandidateSymbols = nil
		return item
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

func bindingTargetFromAffectedSymbol(symbol AffectedSymbol) BindingTarget {
	return BindingTarget{
		Kind:             BindingTargetSymbol,
		FilePath:         symbol.FilePath,
		SymbolName:       symbol.SymbolName,
		SymbolKind:       symbol.SymbolKind,
		Line:             symbol.Line,
		EndLine:          symbol.EndLine,
		BodyHash:         symbol.Hash,
		Confidence:       "high",
		ResolutionSource: "stored_single_symbol_baseline",
	}
}

func missingLegacySymbolBaseline(item LegacyBindingItem) LegacyBindingItem {
	if singleHighConfidenceTarget(BindingResolution{Targets: item.BindingTargets, Diagnostics: item.Diagnostics}, nil) {
		item.Posture = LegacyBindingPostureMissingBindingTargets
		item.RecommendedAction = LegacyBindingActionProposeRebaseline
		item.HighConfidence = true
		item.Reason = "resolver produced one precise binding target; safe to apply binding_targets"
		return item
	}
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

func singleHighConfidenceTarget(resolution BindingResolution, err error) bool {
	if err != nil || len(resolution.Diagnostics) > 0 || len(resolution.Targets) != 1 {
		return false
	}
	switch resolution.Targets[0].Kind {
	case BindingTargetSymbol, BindingTargetRange, BindingTargetModule:
		return true
	default:
		return false
	}
}

func bindingTargetKinds(targets []BindingTarget) []string {
	kinds := make([]string, 0, len(targets))
	for _, target := range targets {
		kinds = append(kinds, target.Kind)
	}
	sort.Strings(kinds)
	return kinds
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
	case LegacyBindingPostureMissingBindingTargets:
		summary.MissingBindingTargets++
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
