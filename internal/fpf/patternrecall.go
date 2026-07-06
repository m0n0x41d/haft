package fpf

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	PatternRecallSchemaVersion = 1
	PatternRecallRecordKind    = "pattern_recall"
	PatternRecallAuthority     = "read_only_pattern_recall_source_card_retrieval_not_pattern_application"

	PatternRecallCompactMode = "compact"
	PatternRecallFullMode    = "full"

	PatternRecallSupportSourceCardRetrieved = "source_card_retrieved"
	PatternRecallSupportMissing             = "missing"

	PatternRecallSourceTierFPFCore                = "fpf_core"
	PatternRecallSourceTierSlideument             = "slideument"
	PatternRecallSourceTierDerivedHaftOperational = "derived_haft_operational"

	PatternRecallDefaultLimit = 5
)

var patternRecallAuthorityBoundary = []string{
	"read_only_source_card_recall",
	"not_pattern_application",
	"not_evidence",
	"not_decision_record",
	"not_work_commission",
	"not_methodpack_gate",
	"not_approval",
}

type PatternRecallRequest struct {
	Query      string   `json:"query"`
	Mode       string   `json:"mode,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

type PatternRecallResult struct {
	SchemaVersion        int                      `json:"schema_version"`
	RecordKind           string                   `json:"record_kind"`
	Authority            string                   `json:"authority"`
	Query                string                   `json:"query"`
	Mode                 string                   `json:"mode"`
	SupportLevel         string                   `json:"support_level"`
	CandidateSourceCards []PatternRecallCandidate `json:"candidate_source_cards,omitempty"`
	OneLineBoundary      string                   `json:"one_line_boundary"`
	FullRecallCommand    string                   `json:"full_recall_command,omitempty"`
	AuthorityBoundary    []string                 `json:"authority_boundary"`
}

type PatternRecallCandidate struct {
	PatternID      string                   `json:"pattern_id"`
	Title          string                   `json:"title"`
	Summary        string                   `json:"summary,omitempty"`
	Snippet        string                   `json:"snippet,omitempty"`
	WhyRetrieved   string                   `json:"why_retrieved,omitempty"`
	SourceTier     string                   `json:"source_tier,omitempty"`
	SourceReason   string                   `json:"source_reason,omitempty"`
	SourceRefShort string                   `json:"source_ref_short,omitempty"`
	SupportLevel   string                   `json:"support_level"`
	SourceCard     *PatternRecallSourceCard `json:"source_card,omitempty"`
}

type PatternRecallSourceCard struct {
	BodyKind     string `json:"body_kind"`
	SourcePath   string `json:"source_path"`
	SourceCommit string `json:"source_commit"`
	LineStart    int    `json:"line_start,omitempty"`
	LineEnd      int    `json:"line_end,omitempty"`
	RootNodeID   string `json:"root_node_id,omitempty"`
	BodyHash     string `json:"body_hash"`
	NodeCount    int    `json:"node_count,omitempty"`
	Body         string `json:"body"`
}

func NormalizePatternRecallMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return PatternRecallCompactMode, nil
	}
	switch mode {
	case PatternRecallCompactMode, PatternRecallFullMode:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported pattern recall mode %q: use compact or full", mode)
	}
}

func ShouldAttemptPatternRecall(query string) bool {
	normalized := normalizePatternUseQuery(query)
	if normalized == "" {
		return false
	}
	if isPatternUseMechanicalLookup(normalized) {
		return false
	}
	if patternRecallLooksLikeRouterMetaQuestion(normalized) {
		return false
	}
	return true
}

func PatternRecallFromRetrievedCandidates(
	request PatternRecallRequest,
	retrieved []PatternUseRetrievedCandidate,
) PatternRecallResult {
	mode, err := NormalizePatternRecallMode(request.Mode)
	if err != nil {
		mode = PatternRecallCompactMode
	}
	if !ShouldAttemptPatternRecall(request.Query) {
		return missingPatternRecallResult(request, mode)
	}

	limit := request.Limit
	if limit <= 0 {
		limit = PatternRecallDefaultLimit
	}

	candidates := normalizePatternUseRetrievedCandidates(retrieved, limit)
	if len(candidates) == 0 {
		return missingPatternRecallResult(request, mode)
	}

	out := make([]PatternRecallCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, patternRecallCandidateFromRetrieved(mode, candidate))
	}

	return PatternRecallResult{
		SchemaVersion:        PatternRecallSchemaVersion,
		RecordKind:           PatternRecallRecordKind,
		Authority:            PatternRecallAuthority,
		Query:                strings.TrimSpace(request.Query),
		Mode:                 mode,
		SupportLevel:         PatternRecallSupportSourceCardRetrieved,
		CandidateSourceCards: out,
		OneLineBoundary:      "PatternRecall returns read-only source-card candidates; applying a pattern still requires the agent's reasoning and evidence boundary.",
		FullRecallCommand:    patternRecallFullCommand(request.Query),
		AuthorityBoundary:    append([]string(nil), patternRecallAuthorityBoundary...),
	}
}

func PatternRecallExactCandidatesFromAtlas(
	db *sql.DB,
	query string,
	limit int,
) []PatternUseRetrievedCandidate {
	if db == nil || limit == 0 {
		return nil
	}
	refs := ExtractPatternUseRefs(query).PatternRefs
	if len(refs) == 0 {
		return nil
	}
	if limit < 0 {
		limit = len(refs)
	}

	out := make([]PatternUseRetrievedCandidate, 0, len(refs))
	for _, ref := range refs {
		card, err := GetPatternCard(db, ref)
		if err != nil {
			continue
		}
		out = append(out, PatternUseRetrievedCandidate{
			PatternRef:    card.PatternID,
			Title:         card.Title,
			Summary:       card.Title,
			Snippet:       patternRecallSnippet(card.Body),
			SourceTier:    SpecSearchTierPattern,
			SourceReason:  "explicit pattern_ref in query",
			SourceRef:     card.SourceRef,
			SourceKind:    "fpf_pattern_card",
			Normativity:   "normative_fpf_source",
			RetrievalMode: "exact_pattern_ref",
			SourceCard:    patternUseSourceCardFromAtlas(card),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (record PatternRecallResult) Validate() error {
	if record.RecordKind != PatternRecallRecordKind {
		return fmt.Errorf("pattern recall record_kind = %q", record.RecordKind)
	}
	if record.Authority != PatternRecallAuthority {
		return fmt.Errorf("pattern recall authority = %q", record.Authority)
	}
	if record.SupportLevel == string(PatternUseSupportImplementedSubstrate) {
		return fmt.Errorf("pattern recall must not claim implemented_substrate support")
	}
	if record.SupportLevel != PatternRecallSupportSourceCardRetrieved &&
		record.SupportLevel != PatternRecallSupportMissing {
		return fmt.Errorf("unsupported pattern recall support_level %q", record.SupportLevel)
	}
	mode, err := NormalizePatternRecallMode(record.Mode)
	if err != nil {
		return err
	}
	if record.SupportLevel == PatternRecallSupportMissing && len(record.CandidateSourceCards) > 0 {
		return fmt.Errorf("missing pattern recall must not carry candidates")
	}
	if record.SupportLevel == PatternRecallSupportSourceCardRetrieved && len(record.CandidateSourceCards) == 0 {
		return fmt.Errorf("source_card_retrieved pattern recall requires candidates")
	}
	for _, candidate := range record.CandidateSourceCards {
		if strings.TrimSpace(candidate.PatternID) == "" {
			return fmt.Errorf("pattern recall candidate missing pattern_id")
		}
		if !validPatternRecallSourceTier(candidate.SourceTier) {
			return fmt.Errorf("pattern recall candidate %s has invalid source_tier %q", candidate.PatternID, candidate.SourceTier)
		}
		if candidate.SupportLevel != PatternRecallSupportSourceCardRetrieved {
			return fmt.Errorf("pattern recall candidate support_level = %q", candidate.SupportLevel)
		}
		if mode == PatternRecallCompactMode &&
			candidate.SourceCard != nil &&
			strings.TrimSpace(candidate.SourceCard.Body) != "" {
			return fmt.Errorf("compact pattern recall must not carry source_card body")
		}
		if mode == PatternRecallFullMode {
			if candidate.SourceCard == nil {
				return fmt.Errorf("full pattern recall candidate %s missing source_card", candidate.PatternID)
			}
			if strings.TrimSpace(candidate.SourceCard.Body) == "" {
				return fmt.Errorf("full pattern recall candidate %s missing source_card body", candidate.PatternID)
			}
			if strings.TrimSpace(candidate.SourceCard.BodyHash) == "" {
				return fmt.Errorf("full pattern recall candidate %s missing source_card body_hash", candidate.PatternID)
			}
		}
	}
	if record.HasAuthorityViolation() {
		return fmt.Errorf("pattern recall contains authority overclaim")
	}
	return nil
}

func (record PatternRecallResult) HasAuthorityViolation() bool {
	if record.SupportLevel == string(PatternUseSupportImplementedSubstrate) {
		return true
	}
	text := strings.ToLower(strings.Join(record.AuthorityBoundary, " "))
	for _, forbidden := range []string{"decisionrecord", "workcommission", "gate_passed", "approval_granted"} {
		if strings.Contains(text, forbidden) {
			return true
		}
	}
	for _, candidate := range record.CandidateSourceCards {
		if candidate.SupportLevel == string(PatternUseSupportImplementedSubstrate) {
			return true
		}
	}
	return false
}

func missingPatternRecallResult(request PatternRecallRequest, mode string) PatternRecallResult {
	return PatternRecallResult{
		SchemaVersion:     PatternRecallSchemaVersion,
		RecordKind:        PatternRecallRecordKind,
		Authority:         PatternRecallAuthority,
		Query:             strings.TrimSpace(request.Query),
		Mode:              mode,
		SupportLevel:      PatternRecallSupportMissing,
		OneLineBoundary:   "No source-card recall candidate was selected.",
		FullRecallCommand: patternRecallFullCommand(request.Query),
		AuthorityBoundary: append([]string(nil), patternRecallAuthorityBoundary...),
	}
}

func patternRecallCandidateFromRetrieved(
	mode string,
	candidate PatternUseRetrievedCandidate,
) PatternRecallCandidate {
	out := PatternRecallCandidate{
		PatternID:      strings.TrimSpace(candidate.PatternRef),
		Title:          firstNonEmptyPatternUseString(candidate.Title, candidate.PatternRef),
		Summary:        strings.TrimSpace(candidate.Summary),
		Snippet:        patternRecallSnippet(candidate.Snippet),
		WhyRetrieved:   patternRecallWhyRetrieved(candidate),
		SourceTier:     patternRecallSourceTier(candidate),
		SourceReason:   strings.TrimSpace(candidate.SourceReason),
		SourceRefShort: strings.TrimSpace(candidate.SourceRef),
		SupportLevel:   PatternRecallSupportSourceCardRetrieved,
	}
	if mode == PatternRecallFullMode && candidate.SourceCard != nil {
		out.SourceCard = patternRecallSourceCardFromPatternUse(candidate.SourceCard)
	}
	return out
}

func patternRecallSourceCardFromPatternUse(card *PatternUseSourceCard) *PatternRecallSourceCard {
	if card == nil {
		return nil
	}
	return &PatternRecallSourceCard{
		BodyKind:     strings.TrimSpace(card.BodyKind),
		SourcePath:   strings.TrimSpace(card.SourceRef),
		SourceCommit: strings.TrimSpace(card.FPFCommit),
		LineStart:    card.StartLine,
		LineEnd:      card.EndLine,
		RootNodeID:   strings.TrimSpace(card.RootNodeID),
		BodyHash:     strings.TrimSpace(card.ContentHash),
		NodeCount:    card.NodeCount,
		Body:         strings.TrimSpace(card.Body),
	}
}

func patternRecallWhyRetrieved(candidate PatternUseRetrievedCandidate) string {
	parts := []string{"Retrieved from the embedded FPF pattern-card index as a source-card candidate."}
	if candidate.SourceTier != "" {
		parts = append(parts, "tier="+candidate.SourceTier)
	}
	if candidate.SourceReason != "" {
		parts = append(parts, "reason="+candidate.SourceReason)
	}
	if candidate.RetrievalMode != "" {
		parts = append(parts, "mode="+candidate.RetrievalMode)
	}
	return strings.Join(parts, " ")
}

func patternRecallSourceTier(candidate PatternUseRetrievedCandidate) string {
	if strings.EqualFold(strings.TrimSpace(candidate.Normativity), "normative_fpf_source") {
		return PatternRecallSourceTierFPFCore
	}
	sourceKind := strings.ToLower(strings.TrimSpace(candidate.SourceKind))
	if strings.Contains(sourceKind, "slide") {
		return PatternRecallSourceTierSlideument
	}
	if strings.Contains(sourceKind, "route") || strings.Contains(sourceKind, "haft") {
		return PatternRecallSourceTierDerivedHaftOperational
	}
	if sourceKind != "" {
		return PatternRecallSourceTierFPFCore
	}
	return PatternRecallSourceTierFPFCore
}

func validPatternRecallSourceTier(sourceTier string) bool {
	switch strings.TrimSpace(sourceTier) {
	case PatternRecallSourceTierFPFCore,
		PatternRecallSourceTierSlideument,
		PatternRecallSourceTierDerivedHaftOperational:
		return true
	default:
		return false
	}
}

func patternRecallSnippet(snippet string) string {
	snippet = strings.TrimSpace(snippet)
	if len([]rune(snippet)) <= 360 {
		return snippet
	}
	return string([]rune(snippet)[:360])
}

func patternRecallFullCommand(query string) string {
	query = strings.ReplaceAll(strings.TrimSpace(query), `"`, `\"`)
	if query == "" {
		return `haft_query(action="pattern_recall", mode="full", query="<query>")`
	}
	return fmt.Sprintf(`haft_query(action="pattern_recall", mode="full", query="%s")`, query)
}

func patternRecallLooksLikeRouterMetaQuestion(normalized string) bool {
	metaMarkers := []string{
		"all fpf cards",
		"all pattern cards",
		"all route cards",
		"250 fpf cards",
		"250 pattern cards",
		"compile all fpf",
		"compile every fpf",
		"route every fpf",
		"нужно ли компилировать все",
		"надо ли компилировать все",
		"все 250 fpf",
		"все карточки fpf",
		"所有 fpf 卡片",
		"全部 fpf 卡片",
	}
	for _, marker := range metaMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
