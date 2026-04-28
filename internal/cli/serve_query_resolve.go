package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

// ResolveTermResult is the structured response shape for
// haft_query(action="resolve_term"). It bundles every place inside the
// bounded project context where the term could be grounded so the host
// agent can frame work without bouncing back to the operator with vague
// "what do you mean?" questions. Investigation-first discipline.
type ResolveTermResult struct {
	Term            string                       `json:"term"`
	Resolution      string                       `json:"resolution"` // resolved | ambiguous | absent
	TermMapEntries  []project.TermMapEntry       `json:"term_map_entries"`
	SpecSectionRefs []ResolveTermSpecSectionRef  `json:"spec_section_refs"`
	ArtifactMentions []ResolveTermArtifactMention `json:"artifact_mentions"`
	NextAction      string                       `json:"next_action"`
}

// ResolveTermSpecSectionRef is a SpecSection whose `terms` field
// references the resolved term. The section itself often disambiguates
// what the term means inside this project.
type ResolveTermSpecSectionRef struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Title         string `json:"title,omitempty"`
	StatementType string `json:"statement_type,omitempty"`
	ClaimLayer    string `json:"claim_layer,omitempty"`
	DocumentKind  string `json:"document_kind,omitempty"`
	Path          string `json:"path,omitempty"`
}

// ResolveTermArtifactMention is a past Decision/Problem/Note/etc.
// whose title or content mentions the term — useful for past
// disambiguation context the operator may have already resolved.
type ResolveTermArtifactMention struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

// handleQuintQueryResolveTerm sweeps term-map entries, spec sections,
// and FTS-indexed artifacts for grounding context on a given term. It
// returns a typed result the host agent can read in one round-trip
// instead of issuing N independent Glob/Grep/Read calls.
func handleQuintQueryResolveTerm(ctx context.Context, store *artifact.Store, haftDir string, args map[string]any) (string, error) {
	term := strings.TrimSpace(stringArg(args, "term"))
	if term == "" {
		return "", fmt.Errorf("term is required for resolve_term action")
	}

	projectRoot := filepath.Dir(haftDir)
	limit := 20
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	result := ResolveTermResult{
		Term:             term,
		TermMapEntries:   []project.TermMapEntry{},
		SpecSectionRefs:  []ResolveTermSpecSectionRef{},
		ArtifactMentions: []ResolveTermArtifactMention{},
	}

	if specSet, err := project.LoadProjectSpecificationSet(projectRoot); err == nil {
		result.TermMapEntries = matchingTermMapEntries(specSet, term)
		result.SpecSectionRefs = matchingSpecSectionRefs(specSet, term)
	}

	if store != nil {
		hits, err := artifact.FetchSearchResults(ctx, store, term, limit)
		if err == nil {
			result.ArtifactMentions = mentionsFromArtifacts(hits)
		}
	}

	result.Resolution = classifyResolution(result)
	result.NextAction = nextActionForResolution(result)

	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal resolve_term: %w", err)
	}
	return string(payload), nil
}

func matchingTermMapEntries(set project.ProjectSpecificationSet, term string) []project.TermMapEntry {
	needle := strings.ToLower(term)
	out := []project.TermMapEntry{}
	for _, entry := range set.TermMapEntries {
		if strings.ToLower(strings.TrimSpace(entry.Term)) == needle {
			out = append(out, entry)
			continue
		}
		for _, alias := range entry.Aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == needle {
				out = append(out, entry)
				break
			}
		}
	}
	return out
}

func matchingSpecSectionRefs(set project.ProjectSpecificationSet, term string) []ResolveTermSpecSectionRef {
	needle := strings.ToLower(term)
	out := []ResolveTermSpecSectionRef{}
	for _, section := range set.Sections {
		mentions := false
		for _, sectionTerm := range section.Terms {
			if strings.ToLower(strings.TrimSpace(sectionTerm)) == needle {
				mentions = true
				break
			}
		}
		if !mentions {
			continue
		}
		out = append(out, ResolveTermSpecSectionRef{
			ID:            section.ID,
			Kind:          section.Kind,
			Title:         section.Title,
			StatementType: section.StatementType,
			ClaimLayer:    section.ClaimLayer,
			DocumentKind:  section.DocumentKind,
			Path:          section.Path,
		})
	}
	return out
}

func mentionsFromArtifacts(hits []*artifact.Artifact) []ResolveTermArtifactMention {
	out := []ResolveTermArtifactMention{}
	for _, hit := range hits {
		if hit == nil {
			continue
		}
		out = append(out, ResolveTermArtifactMention{
			ID:    hit.Meta.ID,
			Kind:  string(hit.Meta.Kind),
			Title: hit.Meta.Title,
		})
	}
	return out
}

func classifyResolution(r ResolveTermResult) string {
	switch {
	case len(r.TermMapEntries) == 1 && len(r.SpecSectionRefs) <= 1:
		return "resolved"
	case len(r.TermMapEntries) == 0 && len(r.SpecSectionRefs) == 0 && len(r.ArtifactMentions) == 0:
		return "absent"
	default:
		return "ambiguous"
	}
}

func nextActionForResolution(r ResolveTermResult) string {
	switch r.Resolution {
	case "resolved":
		return fmt.Sprintf("term %q has a single canonical entry; use it directly without asking the operator", r.Term)
	case "absent":
		return fmt.Sprintf("term %q is not in term-map and has no artifact mentions; if it is load-bearing, propose adding it to .haft/specs/term-map.md and/or ask the operator only the one missing question", r.Term)
	default:
		return fmt.Sprintf("term %q is ambiguous in this project; surface the candidates to the operator with a SINGLE concrete question naming the disambiguations found", r.Term)
	}
}
