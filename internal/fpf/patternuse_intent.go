package fpf

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	patternUseLetterPatternRefRE = regexp.MustCompile(`(?i)\b([A-Z])\s*\.?\s*(\d+(?:\s*\.\s*(?:\d+|[A-Z]+))*)\b`)
	patternUseNamedPatternRefRE  = regexp.MustCompile(`(?i)\b((?:CHR|FRAME|EXP)-\d+|X-[A-Z0-9-]+)\b`)
)

type PatternUseIntentEmbeddingDocument struct {
	LaneID       PatternUseIntentLane
	DocumentID   string
	DocumentKind string
	Text         string
	ContentHash  string
}

func ExtractPatternUseRefs(query string) PatternUseReferenceSet {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return PatternUseReferenceSet{}.withValidatedShape()
	}

	refs := PatternUseReferenceSet{
		PatternRefs: extractPatternUsePatternRefs(trimmed),
		SurfaceRefs: extractPatternUseSurfaceRefs(trimmed),
		RouteRefs:   extractPatternUseRouteRefs(trimmed),
		CatalogRefs: extractPatternUseCatalogRefs(trimmed),
	}
	return refs.withValidatedShape()
}

func DefaultPatternUseIntentLaneCards() []PatternUseIntentLaneCard {
	return clonePatternUseIntentLaneCards([]PatternUseIntentLaneCard{
		{
			ID:              PatternUseIntentApplyPattern,
			SemanticTrigger: "Apply a reasoning pattern or task pattern to the operator concern and return the pattern-shaped output.",
			PositiveExamples: []string{
				"Choose a good name for this project/system/process.",
				"Choose a name for this project/system/process.",
				"Choose a better name for haft if any possible",
				"именуй нормально",
				"给这个系统起个好名字",
				"Propose an architecture for this system.",
				"Предложить архитектуру механизма, который выбирает подходящий паттерн рассуждения перед тем, как агент начинает отвечать; нужна структура и границы, без маркетинга.",
				"Debug this unclear failure.",
				"What should I do next?",
				"What's the next move?",
				"I am stuck; pick the next useful move.",
				"Survey SoTA for solving this problem.",
				"Find current practice before we design this.",
				"Compare these two implementation options.",
				"Which of these two designs is better?",
				"Clarify what is the object, description, carrier, and evidence here.",
				"Is this dashboard the product state or just a view?",
				"The spec says it, so can we rely on it?",
				"В документе написано, что механизм работает. Значит ли это, что мы доказали его работоспособность?",
				"Plan a public API change.",
				"Should we commit to this product direction?",
				"This review is positive; should we approve the direction?",
				"Is this plan actual work or just a plan?",
				"Did you do the work or only describe the plan?",
				"Ты сделал работу или только описал план?",
				"Plan the AI agent tool-call sequence for this risky change.",
				"What tools may the agent call before editing?",
				"Write this into specs.",
				"Approve or rebaseline this SpecSection.",
				"Should PatternUse become MethodPack?",
				"Should MethodPack become SWE-DPF?",
				"Do we need all FPF cards as route cards?",
				"[$h-reason] Разбери границу между FPF source cards, DPF source pack, PatternUseGateway и MethodPack. Не делай коммитов, нужен reasoning carrier.",
				"Разбери границу между FPF source cards, DPF source pack, PatternUseGateway и MethodPack.",
				"применить F.18 для имени системы",
				"apply F.18 to this project name",
			},
			NegativeExamples: []string{
				"what is F.18?",
				"explain what nameCard means",
				"what time is it",
				"how many route cards are currently in the index?",
			},
		},
		{
			ID:              PatternUseIntentExplainPattern,
			SemanticTrigger: "Explain, define, or look up a named pattern, card, surface, or term without applying it to a task.",
			PositiveExamples: []string{
				"what is F.18?",
				"explain what nameCard means",
				"объясни F.18 без применения",
				"解释一下 F.18 是什么",
			},
			NegativeExamples: []string{
				"apply F.18 to this name",
				"именуй нормально",
				"choose a name for this project",
			},
		},
		{
			ID:              PatternUseIntentComparePatterns,
			SemanticTrigger: "Compare patterns, route cards, or pattern candidates as catalog objects instead of applying a task route.",
			PositiveExamples: []string{
				"compare F.18 and A.7 as pattern cards",
				"which PatternUse route card should exist for this concern?",
				"сравни эти FPF карточки между собой",
				"比较这些模式卡",
			},
			NegativeExamples: []string{
				"Compare these two implementation options.",
				"Which of these two designs is better?",
				"[$h-reason] Разбери границу между FPF source cards, DPF source pack, PatternUseGateway и MethodPack. Не делай коммитов, нужен reasoning carrier.",
			},
		},
		{
			ID:              PatternUseIntentAuditRouter,
			SemanticTrigger: "Audit, debug, or improve the PatternUse router, PatternPull, route-card index, classifier, or gateway behavior.",
			PositiveExamples: []string{
				"why did F.18 route this wrong?",
				"audit the PatternUse router false positive",
				"роутер ошибочно выбрал nameCard, проверь почему",
				"调试 PatternUse 路由器",
			},
			NegativeExamples: []string{
				"choose a better name for this system",
				"debug this production failure",
				"This review is positive; should we approve the direction?",
				"Should we commit to this product direction?",
			},
		},
		{
			ID:              PatternUseIntentCatalogMeta,
			SemanticTrigger: "Ask about the FPF catalog, route-card coverage, pattern-card count, or whether all cards should be compiled.",
			PositiveExamples: []string{
				"сколько FPF карточек нужно поддерживать роутеру",
				"how many route cards are currently in the index?",
				"show the compiled PatternUse route-card count",
			},
			NegativeExamples: []string{
				"choose a nameCard output for this service",
				"apply this one pattern card now",
				"Do we need all FPF cards as route cards?",
				"Should PatternUse become MethodPack?",
			},
		},
		{
			ID:              PatternUseIntentMechanicalLookup,
			SemanticTrigger: "Answer a mechanical lookup, time/date request, local file listing, or exact term lookup where pattern choice is not material.",
			PositiveExamples: []string{
				"what time is it",
				"what is the term in this equation",
				"show the files in this directory",
				"сколько сейчас времени",
				"这个等式里的 term 是什么",
			},
			NegativeExamples: []string{
				"what should I do next?",
				"choose a better term for this subsystem",
				"Clarify what is the object, description, carrier, and evidence here.",
				"Is this dashboard the product state or just a view?",
			},
		},
		{
			ID:              PatternUseIntentStatusLookup,
			SemanticTrigger: "Read current status, dashboard, logs, or exact existing state without choosing a pattern-shaped method.",
			PositiveExamples: []string{
				"show haft status",
				"what is currently in the queue?",
				"покажи текущий статус",
				"查看当前状态",
			},
			NegativeExamples: []string{
				"what should we do next?",
				"frame the status problem",
			},
		},
		{
			ID:              PatternUseIntentUnknown,
			SemanticTrigger: "The query lacks enough signal to choose apply/explain/catalog/audit/mechanical intent.",
			PositiveExamples: []string{
				"help",
				"think about it",
				"не знаю",
				"看看这个",
			},
			NegativeExamples: []string{
				"apply F.18 to this project name",
				"do we need route cards for every FPF card?",
			},
		},
	})
}

func PatternUseIntentEmbeddingDocuments(cards []PatternUseIntentLaneCard) []PatternUseIntentEmbeddingDocument {
	documents := []PatternUseIntentEmbeddingDocument{}
	for _, card := range clonePatternUseIntentLaneCards(cards) {
		synopsis := patternUseIntentSynopsisDocument(card)
		documents = append(documents, patternUseIntentEmbeddingDocument(
			card.ID,
			string(card.ID)+":synopsis",
			PatternUseRouteDocumentKindSynopsis,
			synopsis,
		))

		for index, example := range card.PositiveExamples {
			documentID := fmt.Sprintf("%s:positive:%02d", card.ID, index+1)
			documents = append(documents, patternUseIntentEmbeddingDocument(
				card.ID,
				documentID,
				PatternUseRouteDocumentKindPositiveExample,
				example,
			))
		}

		for index, example := range card.NegativeExamples {
			documentID := fmt.Sprintf("%s:negative:%02d", card.ID, index+1)
			documents = append(documents, patternUseIntentEmbeddingDocument(
				card.ID,
				documentID,
				PatternUseRouteDocumentKindNegativeExample,
				example,
			))
		}
	}
	return documents
}

func PatternUseIntentLaneAllowsCompiledRoute(lane PatternUseIntentLane) bool {
	return lane == PatternUseIntentApplyPattern
}

func PatternUseIntentLanePermitsRetrieval(lane PatternUseIntentLane) bool {
	switch lane {
	case PatternUseIntentApplyPattern,
		PatternUseIntentExplainPattern,
		PatternUseIntentComparePatterns,
		PatternUseIntentAuditRouter,
		PatternUseIntentCatalogMeta:
		return true
	default:
		return false
	}
}

func (refs PatternUseReferenceSet) withValidatedShape() PatternUseReferenceSet {
	refs.PatternRefs = dedupePatternUseStrings(refs.PatternRefs)
	refs.SurfaceRefs = dedupePatternUseStrings(refs.SurfaceRefs)
	refs.RouteRefs = dedupePatternUseStrings(refs.RouteRefs)
	refs.CatalogRefs = dedupePatternUseStrings(refs.CatalogRefs)
	return refs
}

func extractPatternUsePatternRefs(query string) []string {
	refs := []string{}
	for _, match := range patternUseLetterPatternRefRE.FindAllStringSubmatch(query, -1) {
		if len(match) < 3 {
			continue
		}
		letter := strings.ToUpper(strings.TrimSpace(match[1]))
		tail := normalizePatternUseRefTail(match[2])
		if letter == "" || tail == "" {
			continue
		}
		refs = append(refs, letter+"."+tail)
	}
	for _, match := range patternUseNamedPatternRefRE.FindAllStringSubmatch(query, -1) {
		if len(match) < 2 {
			continue
		}
		refs = append(refs, strings.ToUpper(strings.TrimSpace(match[1])))
	}
	return dedupePatternUseStrings(refs)
}

func normalizePatternUseRefTail(raw string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "")
	tail := strings.ToUpper(replacer.Replace(strings.TrimSpace(raw)))
	tail = strings.Trim(tail, ".")
	if tail == "" {
		return ""
	}
	return tail
}

func extractPatternUseSurfaceRefs(query string) []string {
	normalized := normalizePatternUseQuery(query)
	refs := []string{}
	surfaceNeedles := map[string][]string{
		"nameCard":   {"namecard", "name card"},
		"ADR":        {"adr"},
		"h-diagnose": {"h diagnose", "h-diagnose"},
		"h-frame":    {"h frame", "h-frame"},
		"h-explore":  {"h explore", "h-explore"},
		"h-compare":  {"h compare", "h-compare"},
		"h-reason":   {"h reason", "h-reason"},
		"h-verify":   {"h verify", "h-verify"},
	}
	for ref, needles := range surfaceNeedles {
		if patternUseNormalizedContainsAny(normalized, needles...) {
			refs = append(refs, ref)
		}
	}
	return dedupePatternUseStrings(refs)
}

func extractPatternUseRouteRefs(query string) []string {
	normalized := normalizePatternUseQuery(query)
	refs := []string{}
	for _, route := range DefaultPatternUseRouteCards() {
		routeID := normalizePatternUseQuery(route.ID)
		if routeID == "" {
			continue
		}
		if !patternUseNormalizedContainsAny(normalized, routeID) {
			continue
		}
		refs = append(refs, route.ID)
	}
	return dedupePatternUseStrings(refs)
}

func extractPatternUseCatalogRefs(query string) []string {
	normalized := normalizePatternUseQuery(query)
	refs := []string{}
	if patternUseNormalizedContainsAny(
		normalized,
		"fpf card",
		"fpf cards",
		"pattern card",
		"pattern cards",
		"route card",
		"route cards",
		"250 fpf",
		"all fpf",
		"compile every fpf",
		"компилировать все 250",
		"fpf карточ",
		"route cards",
		"模式卡",
		"路由卡",
	) {
		refs = append(refs, "fpf_pattern_card_catalog")
	}
	return dedupePatternUseStrings(refs)
}

func patternUseNormalizedContainsAny(normalized string, needles ...string) bool {
	for _, needle := range needles {
		candidate := normalizePatternUseQuery(needle)
		if candidate == "" {
			continue
		}
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

func patternUseIntentEmbeddingDocument(
	laneID PatternUseIntentLane,
	documentID string,
	documentKind string,
	text string,
) PatternUseIntentEmbeddingDocument {
	canonicalText := strings.TrimSpace(text)
	return PatternUseIntentEmbeddingDocument{
		LaneID:       laneID,
		DocumentID:   strings.TrimSpace(documentID),
		DocumentKind: strings.TrimSpace(documentKind),
		Text:         canonicalText,
		ContentHash:  specContentHash(canonicalText),
	}
}

func patternUseIntentSynopsisDocument(card PatternUseIntentLaneCard) string {
	parts := []string{
		"intent_lane: " + string(card.ID),
		"trigger: " + card.SemanticTrigger,
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func clonePatternUseIntentLaneCards(values []PatternUseIntentLaneCard) []PatternUseIntentLaneCard {
	out := make([]PatternUseIntentLaneCard, 0, len(values))
	for _, value := range values {
		out = append(out, PatternUseIntentLaneCard{
			ID:               value.ID,
			SemanticTrigger:  strings.TrimSpace(value.SemanticTrigger),
			PositiveExamples: dedupePatternUseStrings(value.PositiveExamples),
			NegativeExamples: dedupePatternUseStrings(value.NegativeExamples),
		})
	}
	return out
}
