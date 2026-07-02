package fpf

import (
	"database/sql"
	"strconv"
	"strings"
)

const (
	PatternUseRetrievedFallbackRouteID = "fpf_retrieval_fallback"
	PatternUseRetrievedCandidateLimit  = 5
	PatternUseSourceCardIndexedSection = "indexed_pattern_section"
)

type PatternUseRetrievedCandidate struct {
	SectionID     int                   `json:"section_id,omitempty"`
	PatternRef    string                `json:"pattern_ref"`
	Title         string                `json:"title"`
	Summary       string                `json:"summary,omitempty"`
	Snippet       string                `json:"snippet,omitempty"`
	SourceTier    string                `json:"source_tier,omitempty"`
	SourceReason  string                `json:"source_reason,omitempty"`
	SourceRef     string                `json:"source_ref,omitempty"`
	SourceKind    string                `json:"source_kind,omitempty"`
	Normativity   string                `json:"normativity,omitempty"`
	RetrievalMode string                `json:"retrieval_mode,omitempty"`
	SourceCard    *PatternUseSourceCard `json:"source_card,omitempty"`
}

func CountPatternCardSections(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sections WHERE id >= ?`, PatternChunkIDBase).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func PatternUseRetrievedCandidatesFromSpecResults(
	results []SpecRetrievedSection,
	limit int,
) []PatternUseRetrievedCandidate {
	if limit <= 0 {
		return []PatternUseRetrievedCandidate{}
	}

	candidates := make([]PatternUseRetrievedCandidate, 0, limit)
	seen := map[string]struct{}{}
	for _, result := range results {
		if result.SectionID < PatternChunkIDBase {
			continue
		}
		patternRef := strings.TrimSpace(result.PatternID)
		if patternRef == "" {
			continue
		}
		if _, ok := seen[patternRef]; ok {
			continue
		}
		seen[patternRef] = struct{}{}
		candidates = append(candidates, PatternUseRetrievedCandidate{
			SectionID:     result.SectionID,
			PatternRef:    patternRef,
			Title:         firstNonEmptyPatternUseString(result.Heading, patternRef),
			Summary:       strings.TrimSpace(result.Summary),
			Snippet:       firstNonEmptyPatternUseString(result.Content, result.Summary),
			SourceTier:    strings.TrimSpace(result.Tier),
			SourceReason:  strings.TrimSpace(result.Reason),
			SourceRef:     strings.TrimSpace(result.Provenance.SourceRef),
			SourceKind:    strings.TrimSpace(result.Provenance.SourceKind),
			Normativity:   strings.TrimSpace(result.Provenance.Normativity),
			RetrievalMode: strings.TrimSpace(result.Provenance.RetrievalMode),
		})
		if len(candidates) >= limit {
			break
		}
	}
	return candidates
}

func RecommendPatternUseWithRetrievedCandidates(
	request PatternUseRequest,
	retrieved []PatternUseRetrievedCandidate,
) PatternUseRecommendation {
	seed := RecommendPatternUseWithContext(request)
	if !PatternUseRecommendationNeedsRetrieval(seed) {
		return seed
	}
	if strings.TrimSpace(request.Query) == "" {
		return seed
	}

	candidates := normalizePatternUseRetrievedCandidates(retrieved, PatternUseRetrievedCandidateLimit)
	if len(candidates) == 0 {
		return seed
	}

	record := recommendationFromRetrievedPatternUseCandidates(request, candidates)
	return record.withValidatedShape()
}

func RecommendPatternUseCompactWithRetrievedCandidates(
	request PatternUseRequest,
	retrieved []PatternUseRetrievedCandidate,
) PatternUseCompactRecommendation {
	record := RecommendPatternUseWithRetrievedCandidates(request, retrieved)
	return compactRecommendationFromPatternUse(record)
}

func PatternUseRecommendationNeedsRetrieval(record PatternUseRecommendation) bool {
	if record.SupportLevel == PatternUseSupportMissing ||
		record.SupportLevel == PatternUseSupportAbstain {
		return true
	}
	return record.MatchedRouteID == "e11_patternuse_fallback"
}

func ShouldAttemptPatternUseRetrieval(query string) bool {
	normalized := normalizePatternUseQuery(query)
	if normalized == "" {
		return false
	}
	if isPatternUseMechanicalLookup(normalized) {
		return false
	}
	if len(ExtractPatternUseRefs(query).PatternRefs) > 0 {
		return true
	}
	for _, cue := range patternUseRetrievalGateCues() {
		if strings.Contains(normalized, cue) {
			return true
		}
	}
	return false
}

func PatternUseRetrievedCandidatesHaveLexicalRecallSignal(
	query string,
	retrieved []PatternUseRetrievedCandidate,
) bool {
	normalized := normalizePatternUseQuery(query)
	if normalized == "" || isPatternUseMechanicalLookup(normalized) {
		return false
	}

	queryTokens := patternUseLexicalTokenSet(normalized)
	candidates := normalizePatternUseRetrievedCandidates(retrieved, PatternUseRetrievedCandidateLimit)
	for _, candidate := range candidates {
		if !patternUseCandidateHasLexicalRetrievalTier(candidate) {
			continue
		}
		if patternUseCandidateRefAppearsInQuery(normalized, candidate) {
			return true
		}
		if patternUseCandidateTitleMatchesQuery(queryTokens, candidate.Title) {
			return true
		}
	}
	return false
}

func ShouldAttemptPatternUseSemanticRouting(query string) bool {
	normalized := normalizePatternUseQuery(query)
	if normalized == "" {
		return false
	}
	return !isPatternUseMechanicalLookup(normalized)
}

func normalizePatternUseRetrievedCandidates(
	retrieved []PatternUseRetrievedCandidate,
	limit int,
) []PatternUseRetrievedCandidate {
	if limit <= 0 {
		return []PatternUseRetrievedCandidate{}
	}

	out := make([]PatternUseRetrievedCandidate, 0, limit)
	seen := map[string]struct{}{}
	for _, candidate := range retrieved {
		patternRef := strings.TrimSpace(candidate.PatternRef)
		if patternRef == "" {
			continue
		}
		if _, ok := seen[patternRef]; ok {
			continue
		}
		seen[patternRef] = struct{}{}
		out = append(out, PatternUseRetrievedCandidate{
			SectionID:     candidate.SectionID,
			PatternRef:    patternRef,
			Title:         firstNonEmptyPatternUseString(candidate.Title, patternRef),
			Summary:       strings.TrimSpace(candidate.Summary),
			Snippet:       strings.TrimSpace(candidate.Snippet),
			SourceTier:    strings.TrimSpace(candidate.SourceTier),
			SourceReason:  strings.TrimSpace(candidate.SourceReason),
			SourceRef:     strings.TrimSpace(candidate.SourceRef),
			SourceKind:    strings.TrimSpace(candidate.SourceKind),
			Normativity:   strings.TrimSpace(candidate.Normativity),
			RetrievalMode: strings.TrimSpace(candidate.RetrievalMode),
			SourceCard:    clonePatternUseSourceCard(candidate.SourceCard),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func patternUseCandidateHasLexicalRetrievalTier(candidate PatternUseRetrievedCandidate) bool {
	tier := strings.ToLower(strings.TrimSpace(candidate.SourceTier))
	reason := strings.ToLower(strings.TrimSpace(candidate.SourceReason))
	mode := strings.ToLower(strings.TrimSpace(candidate.RetrievalMode))
	return tier == SpecSearchTierFTS ||
		tier == SpecSearchTierPattern ||
		mode == SpecRetrievalModeFTS ||
		strings.Contains(reason, "keyword")
}

func patternUseCandidateRefAppearsInQuery(
	normalizedQuery string,
	candidate PatternUseRetrievedCandidate,
) bool {
	refs := []string{candidate.PatternRef}
	refs = append(refs, extractPatternIDs(candidate.Title)...)
	refs = append(refs, extractPatternIDs(candidate.Summary)...)
	for _, ref := range dedupePatternUseStrings(refs) {
		normalizedRef := normalizePatternUseQuery(ref)
		if normalizedRef != "" && strings.Contains(normalizedQuery, normalizedRef) {
			return true
		}
	}
	return false
}

func patternUseCandidateTitleMatchesQuery(
	queryTokens map[string]struct{},
	title string,
) bool {
	titleTokens := patternUseLexicalTokens(title)
	overlap := 0
	for _, token := range titleTokens {
		if _, ok := queryTokens[token]; !ok {
			continue
		}
		overlap++
		if overlap >= 2 {
			return true
		}
		if len(token) >= 8 {
			return true
		}
	}
	return false
}

func patternUseLexicalTokenSet(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range patternUseLexicalTokens(text) {
		out[token] = struct{}{}
	}
	return out
}

func patternUseLexicalTokens(text string) []string {
	normalized := normalizePatternUseQuery(text)
	fields := strings.Fields(normalized)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) <= 2 {
			continue
		}
		out = append(out, field)
	}
	return dedupePatternUseStrings(out)
}

func HydratePatternUseRetrievedCandidatesWithAtlas(
	db *sql.DB,
	retrieved []PatternUseRetrievedCandidate,
) []PatternUseRetrievedCandidate {
	if db == nil {
		return normalizePatternUseRetrievedCandidates(retrieved, PatternUseRetrievedCandidateLimit)
	}

	candidates := normalizePatternUseRetrievedCandidates(retrieved, PatternUseRetrievedCandidateLimit)
	for index, candidate := range candidates {
		if candidate.SourceCard != nil {
			continue
		}
		card, ok := firstPatternUseAtlasCard(db, candidate)
		if !ok {
			sourceCard, ok := patternUseSourceCardFromIndexedPatternSection(db, candidate)
			if !ok {
				continue
			}
			candidates[index].SourceCard = sourceCard
			continue
		}
		candidates[index].Title = firstNonEmptyPatternUseString(candidate.Title, card.Title)
		candidates[index].SourceCard = patternUseSourceCardFromAtlas(card)
	}
	return candidates
}

func firstPatternUseAtlasCard(
	db *sql.DB,
	candidate PatternUseRetrievedCandidate,
) (PatternAtlasCardContent, bool) {
	for _, patternRef := range patternUseAtlasCandidateRefs(db, candidate) {
		card, err := GetPatternCard(db, patternRef)
		if err != nil {
			continue
		}
		return card, true
	}
	return PatternAtlasCardContent{}, false
}

func patternUseAtlasCandidateRefs(
	db *sql.DB,
	candidate PatternUseRetrievedCandidate,
) []string {
	refs := []string{candidate.PatternRef}
	text := strings.Join(
		[]string{
			candidate.Title,
			candidate.Summary,
			candidate.Snippet,
			candidate.SourceReason,
			candidate.SourceRef,
		},
		" ",
	)
	refs = append(refs, extractPatternIDs(text)...)
	if body, ok := patternUseIndexedPatternBody(db, candidate.PatternRef); ok {
		refs = append(refs, extractPatternIDs(body)...)
	}
	return dedupePatternUseStrings(refs)
}

func patternUseIndexedPatternBody(db *sql.DB, patternRef string) (string, bool) {
	if db == nil || strings.TrimSpace(patternRef) == "" {
		return "", false
	}

	var body string
	err := db.QueryRow(`
		SELECT body
		FROM sections
		WHERE pattern_id = ?
		ORDER BY id
		LIMIT 1`, strings.TrimSpace(patternRef)).Scan(&body)
	if err != nil {
		return "", false
	}
	return body, strings.TrimSpace(body) != ""
}

func patternUseSourceCardFromIndexedPatternSection(
	db *sql.DB,
	candidate PatternUseRetrievedCandidate,
) (*PatternUseSourceCard, bool) {
	if db == nil || candidate.SectionID < PatternChunkIDBase {
		return nil, false
	}
	if !isPatternUseGeneratedPatternSectionRef(candidate.PatternRef) {
		return nil, false
	}

	var patternRef string
	var body string
	err := db.QueryRow(`
		SELECT pattern_id, body
		FROM sections
		WHERE id = ?
		LIMIT 1`, candidate.SectionID).Scan(&patternRef, &body)
	if err != nil {
		return nil, false
	}
	if strings.TrimSpace(patternRef) == "" || strings.TrimSpace(body) == "" {
		return nil, false
	}

	info, _ := GetSpecIndexInfo(db)
	return &PatternUseSourceCard{
		BodyKind:    PatternUseSourceCardIndexedSection,
		SourceRef:   firstNonEmptyPatternUseString(info.SpecPath, candidate.SourceRef),
		FPFCommit:   strings.TrimSpace(info.Commit),
		RootNodeID:  "section:" + strconv.Itoa(candidate.SectionID),
		ContentHash: patternAtlasHash(body),
		NodeCount:   1,
		Body:        strings.TrimSpace(body),
	}, true
}

func isPatternUseGeneratedPatternSectionRef(patternRef string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(patternRef))
	for _, prefix := range []string{"CHR-", "FRAME-", "EXP-", "DEC-", "VER-", "X-"} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func recommendationFromRetrievedPatternUseCandidates(
	request PatternUseRequest,
	candidates []PatternUseRetrievedCandidate,
) PatternUseRecommendation {
	concern, useContext := patternUseConcernAndContext(request, patternUseRetrievedSourceRefs(candidates))
	candidateUses := patternUseCandidateUsesFromRetrieved(candidates)
	applicableUses := patternUseApplicableUsesFromRetrieved(candidates)
	recommendedUse := recommended(candidates[0].PatternRef, candidates[0].Title)
	refs := patternUseRetrievedPatternRefs(candidates)

	return PatternUseRecommendation{
		SchemaVersion:           PatternUseSchemaVersion,
		RecordKind:              PatternUseRecordKind,
		Authority:               PatternUseAuthority,
		ProjectConcernRef:       concern,
		UseContext:              useContext,
		CandidatePatternUseSet:  candidateUses,
		ApplicablePatternUseSet: applicableUses,
		RecommendedPatternUse:   recommendedUse,
		ReasonForRecommendation: "No compiled seed PatternUse route matched the concern; FPF-wide retrieval found candidate pattern cards. Treat these as read-only recall candidates and inspect the full card before applying a pattern.",
		WrongPatternBoundary: []WrongPatternBoundary{
			{
				TemptingPatternOrMove: "treat top-k FPF retrieval as a compiled route card",
				WhyWrongNow:           "Retrieval ranks candidate source text; it does not prove applicability or supply a pattern-specific output shape.",
			},
		},
		RequiredOutputShape: RequiredOutputShape{
			CarrierKind: "retrieved_pattern_applicability_card",
			RequiredSections: []string{
				"current_concern",
				"retrieved_pattern_candidates",
				"selected_pattern_ref",
				"source_snippet_or_section",
				"applicability_check",
				"wrong_pattern_boundary",
				"blocked_stronger_use",
				"next_action",
			},
		},
		RequiredEvidenceOrSoTA: []RequiredEvidenceOrSoTA{
			{
				Requirement:           "Read the full FPF card for the selected candidate and check that the current concern matches its trigger, boundary, anti-pattern, and closeout expectations.",
				FreshnessOrSourceRule: "Use embedded FPF retrieval provenance; if the card source is stale or snippet-only, fetch the full card before stronger use.",
			},
		},
		BlockedStrongerUse: []BlockedStrongerUse{
			{
				BlockedUse:       "Top-k FPF retrieval is not a compiled PatternUse route card.",
				UnblockCondition: "Read the full source card, validate applicability against the current concern, and only then produce the card-specific output shape.",
			},
		},
		CloseoutOrVerificationExpectation: []CloseoutOrVerificationExpectation{
			{Expectation: "Close by naming the selected pattern, the rejected near-miss candidate, and the one source sentence or section that justified applicability."},
		},
		SupportLevel:             PatternUseSupportRetrievedUncompiled,
		SuggestedHaftSurface:     "inline",
		NextGoverningPatternRefs: refs,
		MatchedRouteID:           PatternUseRetrievedFallbackRouteID,
		MatchedRecognitionCues:   refs,
		RouteMatchStrategy:       PatternUseRouteMatchStrategyRetrievedUncompiled,
		AuthorityBoundary:        append([]string(nil), patternUseAuthorityBoundary...),
	}
}

func patternUseCandidateUsesFromRetrieved(candidates []PatternUseRetrievedCandidate) []CandidatePatternUse {
	out := make([]CandidatePatternUse, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, CandidatePatternUse{
			PatternRef:          candidate.PatternRef,
			Title:               candidate.Title,
			ApplicabilityReason: patternUseRetrievedApplicabilityReason(candidate),
			SourceTier:          candidate.SourceTier,
			SourceReason:        candidate.SourceReason,
			Summary:             candidate.Summary,
			Snippet:             candidate.Snippet,
			SourceCard:          clonePatternUseSourceCard(candidate.SourceCard),
		})
	}
	return out
}

func patternUseApplicableUsesFromRetrieved(candidates []PatternUseRetrievedCandidate) []ApplicablePatternUse {
	out := make([]ApplicablePatternUse, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, applicable(candidate.PatternRef, candidate.Title))
	}
	return out
}

func patternUseRetrievedApplicabilityReason(candidate PatternUseRetrievedCandidate) string {
	parts := []string{
		"Retrieved from FPF pattern-card index as a candidate for the current concern.",
	}
	if candidate.SourceTier != "" {
		parts = append(parts, "tier="+candidate.SourceTier)
	}
	if candidate.SourceReason != "" {
		parts = append(parts, "reason="+candidate.SourceReason)
	}
	return strings.Join(parts, " ")
}

func patternUseRetrievedPatternRefs(candidates []PatternUseRetrievedCandidate) []string {
	refs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		refs = append(refs, candidate.PatternRef)
	}
	return dedupePatternUseStrings(refs)
}

func patternUseRetrievedSourceRefs(candidates []PatternUseRetrievedCandidate) []string {
	refs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		refs = append(refs, candidate.SourceRef)
		if candidate.SourceCard != nil {
			refs = append(refs, patternUseAtlasSourceRef(candidate.SourceCard))
		}
	}
	return dedupePatternUseStrings(refs)
}

func patternUseSourceCardFromAtlas(card PatternAtlasCardContent) *PatternUseSourceCard {
	return &PatternUseSourceCard{
		BodyKind:    strings.TrimSpace(card.BodyKind),
		SourceRef:   strings.TrimSpace(card.SourceRef),
		FPFCommit:   strings.TrimSpace(card.FPFCommit),
		StartLine:   card.StartLine,
		EndLine:     card.EndLine,
		RootNodeID:  strings.TrimSpace(card.RootNodeID),
		ContentHash: strings.TrimSpace(card.ContentHash),
		NodeCount:   card.NodeCount,
		Body:        strings.TrimSpace(card.Body),
	}
}

func patternUseAtlasSourceRef(card *PatternUseSourceCard) string {
	if card == nil || strings.TrimSpace(card.SourceRef) == "" {
		return ""
	}
	if card.StartLine <= 0 || card.EndLine <= 0 {
		return strings.TrimSpace(card.SourceRef)
	}
	return strings.Join(
		[]string{
			strings.TrimSpace(card.SourceRef),
			"L" + strconv.Itoa(card.StartLine) + "-L" + strconv.Itoa(card.EndLine),
		},
		"#",
	)
}

func patternUseRetrievalGateCues() []string {
	return []string{
		"fpf",
		"pattern",
		"card",
		"boundary",
		"admissibility",
		"norm",
		"claim",
		"proof",
		"evidence",
		"source",
		"carrier",
		"authority",
		"relation",
		"requirement",
		"spec",
		"policy",
		"decision",
		"commitment",
		"architecture",
		"structure",
		"work",
		"method",
		"plan",
		"frame",
		"explore",
		"compare",
		"diagnose",
		"verify",
		"route",
		"index",
		"substrate",
		"concern",
		"applicability",
		"validation",
		"verification",
		"sota",
		"api",
	}
}

func isPatternUseMechanicalLookup(normalizedQuery string) bool {
	for _, cue := range patternUseMechanicalLookupCues() {
		if normalizedQuery == cue {
			return true
		}
	}
	for _, prefix := range patternUseMechanicalLookupPrefixes() {
		if strings.HasPrefix(normalizedQuery, prefix) {
			return true
		}
	}
	return false
}

func patternUseMechanicalLookupCues() []string {
	return []string{
		"what time is it",
		"current time",
		"what is the time",
		"today s date",
		"what date is it",
		"show git status",
	}
}

func patternUseMechanicalLookupPrefixes() []string {
	return []string{
		"what is the term in",
		"what does the term",
		"define the term",
	}
}
