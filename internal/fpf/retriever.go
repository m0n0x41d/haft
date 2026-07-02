package fpf

import (
	"database/sql"
	"strings"
)

const SpecRetrievalModeSemantic = "semantic"

// SpecRetrievalModeFTS forces the deterministic FTS+tier path, bypassing the
// hybrid — a reproducible escape hatch.
const SpecRetrievalModeFTS = "fts"

// SpecRetrievalRequest captures deterministic spec retrieval controls for
// higher-level agent, CLI, and MCP surfaces.
type SpecRetrievalRequest struct {
	Query string
	Limit int
	Tier  string
	Full  bool
	Mode  string
	// HybridSearch, when set by the composition root, makes spec search hybrid
	// (FTS+graph fused with baked section vectors) BY DEFAULT. fpf stays
	// provider-free: the implementation lives in the shell (internal/cli) and is
	// injected as this func. nil => the deterministic SearchSpecWithOptions path.
	HybridSearch func(db *sql.DB, query string, limit int) ([]SpecSearchResult, error)
}

// SpecRetrievalResult is the structured retrieval response returned to shell
// layers before any surface-specific formatting is applied.
type SpecRetrievalResult struct {
	Query   string                 `json:"query"`
	Results []SpecRetrievedSection `json:"results"`
}

// SpecRetrievedSection is a presentation-ready FPF section hit with either
// snippet-sized content or the full section body.
type SpecRetrievedSection struct {
	SectionID  int                     `json:"section_id,omitempty"`
	PatternID  string                  `json:"pattern_id"`
	Heading    string                  `json:"heading"`
	Tier       string                  `json:"tier"`
	Reason     string                  `json:"reason"`
	Summary    string                  `json:"summary"`
	Content    string                  `json:"content"`
	Provenance SpecRetrievalProvenance `json:"provenance"`
}

type SpecRetrievalProvenance struct {
	ProfileID          string `json:"profile_id"`
	SourceKind         string `json:"source_kind"`
	SourceEdition      string `json:"source_edition"`
	SourceRef          string `json:"source_ref"`
	SourceHash         string `json:"source_hash"`
	ProfileValidity    string `json:"profile_validity"`
	Normativity        string `json:"normativity"`
	IndexSchemaVersion string `json:"index_schema_version"`
	RetrievalMode      string `json:"retrieval_mode"`
}

const (
	specRetrievalSourceKindSection       = "fpf_spec_section"
	specRetrievalSourceKindRouteCarrier  = "fpf_route_carrier_over_spec_section"
	specRetrievalSourceKindRelatedGraph  = "fpf_relation_graph_over_spec_section"
	specRetrievalNormativitySource       = "normative_fpf_source"
	specRetrievalNormativityRouteCarrier = "navigation_carrier_non_normative"
	specRetrievalNormativityGraphCarrier = "relation_carrier_non_normative"
)

type specRetrievalSourceProfile struct {
	SourceKind  string
	Normativity string
}

var specRetrievalSourceProfiles = map[string]specRetrievalSourceProfile{
	SpecSearchTierPattern: {
		SourceKind:  specRetrievalSourceKindSection,
		Normativity: specRetrievalNormativitySource,
	},
	SpecSearchTierDrillDown: {
		SourceKind:  specRetrievalSourceKindSection,
		Normativity: specRetrievalNormativitySource,
	},
	SpecSearchTierFTS: {
		SourceKind:  specRetrievalSourceKindSection,
		Normativity: specRetrievalNormativitySource,
	},
	SpecSearchTierRoute: {
		SourceKind:  specRetrievalSourceKindRouteCarrier,
		Normativity: specRetrievalNormativityRouteCarrier,
	},
	SpecSearchTierRelated: {
		SourceKind:  specRetrievalSourceKindRelatedGraph,
		Normativity: specRetrievalNormativityGraphCarrier,
	},
}

// RetrieveSpec resolves deterministic FPF search hits and hydrates content for
// downstream CLI, MCP, and agent surfaces.
func RetrieveSpec(db *sql.DB, request SpecRetrievalRequest) (SpecRetrievalResult, error) {
	query := strings.TrimSpace(request.Query)
	searchResults, err := retrieveSpecSearchResults(db, query, request)
	if err != nil {
		return SpecRetrievalResult{}, err
	}

	indexInfo, _ := GetSpecIndexInfo(db)
	results := make([]SpecRetrievedSection, 0, len(searchResults))
	for _, searchResult := range searchResults {
		provenance := buildSpecRetrievalProvenance(indexInfo, request, searchResult)
		results = append(results, hydrateRetrievedSection(db, searchResult, request.Full, provenance))
	}

	return SpecRetrievalResult{
		Query:   query,
		Results: results,
	}, nil
}

func retrieveSpecSearchResults(db *sql.DB, query string, request SpecRetrievalRequest) ([]SpecSearchResult, error) {
	deterministic := func() ([]SpecSearchResult, error) {
		// Only the tree sub-mode is a SearchSpecWithOptions mode; retrieval-layer
		// modes (fts, the retired semantic) map to the deterministic default.
		specMode := ""
		if strings.EqualFold(strings.TrimSpace(request.Mode), SpecSearchModeTree) {
			specMode = SpecSearchModeTree
		}
		return SearchSpecWithOptions(db, query, SpecSearchOptions{
			Limit: request.Limit,
			Tier:  request.Tier,
			Mode:  specMode,
		})
	}
	// Explicit controls force the reproducible deterministic path. HybridSearch
	// only accepts query+limit, so it cannot honor tier filters or tree mode.
	if hasExplicitSpecRetrievalControls(request) {
		return deterministic()
	}
	// Hybrid by default when the composition root wired it (degrade-safe — the
	// hybrid itself falls back to the deterministic path when the sidecar or
	// baked vectors are unavailable).
	if request.HybridSearch != nil {
		return request.HybridSearch(db, query, request.Limit)
	}
	return deterministic()
}

func hasExplicitSpecRetrievalControls(request SpecRetrievalRequest) bool {
	if strings.TrimSpace(request.Tier) != "" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(request.Mode), SpecSearchModeTree) {
		return true
	}
	return normalizeSpecRetrievalMode(request.Mode) == SpecRetrievalModeFTS
}

func normalizeSpecRetrievalMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case SpecRetrievalModeFTS:
		return SpecRetrievalModeFTS
	}
	return ""
}

func hydrateRetrievedSection(db *sql.DB, searchResult SpecSearchResult, full bool, provenance SpecRetrievalProvenance) SpecRetrievedSection {
	content := searchResult.Snippet
	if full {
		body, err := GetSpecSection(db, firstNonEmpty(searchResult.PatternID, searchResult.Heading))
		if err == nil {
			content = body
		}
	}

	return SpecRetrievedSection{
		SectionID:  searchResult.SectionID,
		PatternID:  searchResult.PatternID,
		Heading:    searchResult.Heading,
		Tier:       searchResult.Tier,
		Reason:     searchResult.Reason,
		Summary:    searchResult.Summary,
		Content:    content,
		Provenance: provenance,
	}
}

func buildSpecRetrievalProvenance(info SpecIndexInfo, request SpecRetrievalRequest, result SpecSearchResult) SpecRetrievalProvenance {
	profile := specRetrievalSourceProfileForTier(result.Tier)

	return SpecRetrievalProvenance{
		ProfileID:          "fpf-spec-index-v" + firstNonEmpty(info.SchemaVersion, SpecIndexSchemaVersion),
		SourceKind:         profile.SourceKind,
		SourceEdition:      specRetrievalSourceEdition(info),
		SourceRef:          info.SpecPath,
		SourceHash:         firstNonEmpty(info.Commit, "unknown"),
		ProfileValidity:    specRetrievalProfileValidity(info),
		Normativity:        profile.Normativity,
		IndexSchemaVersion: firstNonEmpty(info.SchemaVersion, SpecIndexSchemaVersion),
		RetrievalMode:      effectiveSpecRetrievalMode(request),
	}
}

func specRetrievalSourceProfileForTier(tier string) specRetrievalSourceProfile {
	profile, ok := specRetrievalSourceProfiles[strings.TrimSpace(tier)]
	if ok {
		return profile
	}

	return specRetrievalSourceProfile{
		SourceKind:  specRetrievalSourceKindSection,
		Normativity: specRetrievalNormativitySource,
	}
}

func specRetrievalSourceEdition(info SpecIndexInfo) string {
	if edition := strings.TrimSpace(info.SourceEdition); edition != "" {
		return edition
	}
	if commit := strings.TrimSpace(info.Commit); commit != "" {
		return "fpf@" + commit
	}
	return "embedded-index-schema-" + firstNonEmpty(info.SchemaVersion, SpecIndexSchemaVersion)
}

func specRetrievalProfileValidity(info SpecIndexInfo) string {
	if validUntil := strings.TrimSpace(info.ProfileValidUntil); validUntil != "" {
		return "valid_until=" + validUntil
	}
	return "not_declared"
}

func effectiveSpecRetrievalMode(request SpecRetrievalRequest) string {
	mode := strings.TrimSpace(request.Mode)
	if strings.EqualFold(mode, SpecSearchModeTree) {
		return SpecSearchModeTree
	}
	if normalizeSpecRetrievalMode(mode) == SpecRetrievalModeFTS {
		return SpecRetrievalModeFTS
	}
	if strings.TrimSpace(request.Tier) != "" {
		return "tier-filter"
	}
	if request.HybridSearch != nil {
		return "hybrid"
	}
	return SpecRetrievalModeFTS
}
