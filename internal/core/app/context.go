package app

import (
	"reflect"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/code"
	"github.com/m0n0x41d/haft/internal/core/store"
)

// ContextPage reports output limits independently of captured-scope coverage.
// Offset applies to each band, in its documented stable order.
type ContextPage[T any] struct {
	Items      []T  `json:"items"`
	Total      int  `json:"total"`
	Offset     int  `json:"offset"`
	Limit      int  `json:"limit"`
	Truncated  bool `json:"truncated"`
	NextOffset *int `json:"next_offset,omitempty"`
}

func contextPage[T any](items []T, q Request) ContextPage[T] {
	if q.forDelivery {
		return ContextPage[T]{Items: append([]T{}, items...), Total: len(items), Limit: len(items)}
	}
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	start := min(q.Offset, len(items))
	end := min(start+limit, len(items))
	p := ContextPage[T]{Items: append([]T{}, items[start:end]...), Total: len(items), Offset: q.Offset, Limit: limit, Truncated: end-start < len(items)}
	if end < len(items) {
		p.NextOffset = &end
	}
	return p
}

type ContextRecord struct {
	Ref        string   `json:"ref"`
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Kind       string   `json:"kind"`
	State      string   `json:"state"`
	Origin     string   `json:"origin"`
	About      string   `json:"about"`
	Path       string   `json:"path"`
	MatchKinds []string `json:"match_kinds"`
	Selectors  []string `json:"selectors,omitempty"`
}

type ContextBinding struct {
	code.Binding
	OwnerState string `json:"owner_state"`
	Path       string `json:"path"`
}

type ContextEdge struct {
	store.Edge
	Direction string          `json:"direction"`
	Source    *carrier.Source `json:"source,omitempty"`
}

type ContextEvidence struct {
	OwnerRef          string                 `json:"owner_ref"`
	Title             string                 `json:"title"`
	State             string                 `json:"state"`
	Path              string                 `json:"path"`
	Use               carrier.EvidenceUse    `json:"use"`
	TargetStatus      string                 `json:"target_status"`
	TargetCurrentness string                 `json:"target_currentness"`
	CodeCurrentness   string                 `json:"code_currentness"`
	CheckCurrentness  string                 `json:"check_currentness"`
	Basis             *carrier.EvidenceBasis `json:"basis,omitempty"`
}

type ContextEvidencePage struct {
	ContextPage[ContextEvidence]
	PolarityCounts       map[string]int `json:"polarity_counts"`
	MixedPolarityTargets []string       `json:"mixed_polarity_targets"`
}

type ContextMention struct {
	Path     string `json:"path"`
	RecordID string `json:"record_id,omitempty"`
	Title    string `json:"title,omitempty"`
	State    string `json:"state,omitempty"`
	Excerpt  string `json:"excerpt"`
}

func contextCodeSelector(s string) bool {
	return strings.HasPrefix(s, "file:") || strings.HasPrefix(s, "dir:") || strings.HasPrefix(s, "sym:")
}

func selectorPath(s string) (kind, p string) {
	kind, p, _ = strings.Cut(s, ":")
	p, _, _ = strings.Cut(p, "::")
	return kind, p
}

func insidePath(file, directory string) bool {
	return directory == "." || file == directory || strings.HasPrefix(file, directory+"/")
}

// declaredContextMatch is a syntactic scope relation, not semantic applicability.
// Package proximity never creates a constraint match.
func declaredContextMatch(scope, query string, index code.Index) string {
	if scope == query {
		return "declared_exact_selector"
	}
	sk, sp := selectorPath(scope)
	qk, qp := selectorPath(query)
	if sk == "dir" && insidePath(qp, sp) {
		return "declared_scope_contains_query"
	}
	if qk == "dir" && insidePath(sp, qp) {
		return "declared_scope_within_query"
	}
	if sp != qp {
		return ""
	}
	if sk == "file" {
		return "declared_file_scope"
	}
	if qk == "file" {
		return "declared_scope_within_query"
	}
	a, b := index.Resolve(scope), index.Resolve(query)
	if a.Kind == "exact" && b.Kind == "exact" && len(a.Candidates) == 1 && len(b.Candidates) == 1 && a.Candidates[0].Anchor == b.Candidates[0].Anchor {
		return "declared_exact_symbol"
	}
	return ""
}

func contextOwner(ref string) string {
	r, err := carrier.ParseRef(ref)
	if err != nil {
		return ""
	}
	return r.RecordID
}

// The current lineage relates review candidates only. Evidence retains its
// original exact target; no use is moved to a successor by this key.
func contextLineage(p carrier.Projection, id string) string {
	refs := append(p.Heads(id), p.ProposalHeads(id)...)
	if len(refs) == 0 {
		return "record:" + id
	}
	sort.Strings(refs)
	return strings.Join(refs, "\x00")
}

func sortedContextSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func enrichContext(q Request, r *Result, data map[string]any, v store.View, index code.Index) {
	entries := v.Projection.Entries
	related := make([]map[string]bool, len(entries))
	direct := make([]map[string]bool, len(entries))
	selectors := make([]map[string]bool, len(entries))
	byID := map[string][]int{}
	byPath := map[string]int{}
	for n, e := range entries {
		related[n], direct[n], selectors[n] = map[string]bool{}, map[string]bool{}, map[string]bool{}
		byID[e.Document.Record.ID] = append(byID[e.Document.Record.ID], n)
		byPath[v.DocumentPaths[n]] = n
	}
	focusID := contextOwner(q.Ref)
	found := v.Resolve(q.Ref)
	if found.Document != nil {
		focusID = found.Document.Record.ID
	}
	focusLineage := ""
	if focusID != "" {
		focusLineage = contextLineage(v.Projection, focusID)
	}
	literal := strings.TrimPrefix(q.Ref, "source:")
	codeQuery := contextCodeSelector(q.Ref)
	sourceLocator := false
	affected := map[string]bool{}
	if codeQuery {
		for _, p := range index.Related(q.Ref).AffectedFiles {
			affected[p] = true
		}
	}
	for n, e := range entries {
		doc := e.Document.Record
		if focusLineage != "" && contextLineage(v.Projection, doc.ID) == focusLineage {
			related[n]["same_record_lineage"] = true
		}
		if q.Ref == "" && q.Query == "" {
			related[n]["captured_memory"] = true
		}
		if q.Ref == "" && q.Query != "" && strings.Contains(strings.ToLower(string(e.Document.Raw)), strings.ToLower(q.Query)) {
			related[n]["text_candidate"] = true
		}
		for _, source := range doc.Sources {
			if literal != "" && (source.Ref == literal || source.PublicationPath == literal || source.SnapshotRef == literal) {
				related[n]["source_locator"] = true
				sourceLocator = true
			}
		}
		if !codeQuery {
			continue
		}
		for _, selector := range doc.Constrains {
			if match := declaredContextMatch(selector, q.Ref, index); match != "" {
				direct[n][match] = true
				selectors[n][selector] = true
				continue
			}
			kind, p := selectorPath(selector)
			for file := range affected {
				if file == p || kind == "dir" && insidePath(file, p) {
					related[n]["package_constraint_candidate"] = true
					selectors[n][selector] = true
				}
			}
		}
	}
	edges := v.Edges()
	for _, edge := range edges {
		if q.Ref != "" && (edge.To == q.Ref || edge.From == q.Ref || edge.To == literal) {
			for _, n := range byID[contextOwner(edge.From)] {
				related[n]["exact_link_endpoint"] = true
			}
		}
	}
	bindings := index.Bindings(v.Documents)
	if codeQuery {
		bindings = index.BindingsFor(q.Ref, v.Documents)
	}
	var bindingRows []ContextBinding
	for _, binding := range bindings {
		for _, n := range byID[contextOwner(binding.Claim)] {
			if entries[n].State == carrier.Invalid || !codeQuery && len(related[n])+len(direct[n]) == 0 {
				continue
			}
			related[n]["claim_"+binding.Kind] = true
			bindingRows = append(bindingRows, ContextBinding{Binding: binding, OwnerState: entries[n].State, Path: v.DocumentPaths[n]})
		}
	}
	var constrainedRows, proposedRows, relatedRows []ContextRecord
	relevant := map[string]bool{}
	families := map[string]bool{}
	currentTargets := map[string]bool{}
	for n, e := range entries {
		doc := e.Document.Record
		if e.State == carrier.Active || e.State == carrier.Contested || e.State == carrier.Proposed {
			currentTargets[e.Ref] = true
			for _, claim := range doc.Claims {
				currentTargets[e.Ref+"#"+claim.ID] = true
			}
		}
		if len(related[n])+len(direct[n]) == 0 {
			continue
		}
		relevant[doc.ID] = true
		families[contextLineage(v.Projection, doc.ID)] = true
		row := ContextRecord{Ref: e.Ref, ID: doc.ID, Title: doc.Title, Kind: doc.Kind, State: e.State, Origin: doc.Origin, About: doc.About, Path: v.DocumentPaths[n], Selectors: sortedContextSet(selectors[n])}
		if len(direct[n]) > 0 {
			row.MatchKinds = sortedContextSet(direct[n])
			if e.State == carrier.Active || e.State == carrier.Contested {
				constrainedRows = append(constrainedRows, row)
			} else if e.State != carrier.Proposed {
				related[n]["non_governing_constraint"] = true
			}
		}
		if e.State == carrier.Proposed {
			matches := map[string]bool{}
			for k := range related[n] {
				matches[k] = true
			}
			for k := range direct[n] {
				matches[k] = true
			}
			row.MatchKinds = sortedContextSet(matches)
			proposedRows = append(proposedRows, row)
		}
		if len(related[n]) > 0 {
			row.MatchKinds = sortedContextSet(related[n])
			relatedRows = append(relatedRows, row)
		}
	}
	for _, rows := range [][]ContextRecord{constrainedRows, proposedRows, relatedRows} {
		sort.Slice(rows, func(a, b int) bool { return rows[a].Ref+"\x00"+rows[a].Path < rows[b].Ref+"\x00"+rows[b].Path })
	}
	var evidenceRows []ContextEvidence
	counts := map[string]int{"supports": 0, "weakens": 0, "inconclusive": 0}
	polarities := map[string]map[string]bool{}
	for n, e := range entries {
		if e.State == carrier.Invalid {
			continue
		}
		for _, use := range e.Document.Record.Uses {
			id := contextOwner(use.Target)
			if !families[contextLineage(v.Projection, id)] && use.Target != q.Ref && (focusID == "" || id != focusID) {
				continue
			}
			status := v.Resolve(use.Target).Kind
			currentness := "unknown"
			if status == "found" {
				currentness = "changed"
				if currentTargets[use.Target] {
					currentness = "same"
				}
			}
			evidenceRows = append(evidenceRows, ContextEvidence{OwnerRef: e.Ref, Title: e.Document.Record.Title, State: e.State, Path: v.DocumentPaths[n], Use: use, TargetStatus: status, TargetCurrentness: currentness, CodeCurrentness: "unknown", CheckCurrentness: "unknown", Basis: e.Document.Record.Basis})
			relevant[e.Document.Record.ID] = true
			counts[use.Polarity]++
			if polarities[use.Target] == nil {
				polarities[use.Target] = map[string]bool{}
			}
			polarities[use.Target][use.Polarity] = true
		}
	}
	sort.Slice(evidenceRows, func(a, b int) bool {
		x, y := evidenceRows[a], evidenceRows[b]
		return x.Use.Target+"\x00"+x.OwnerRef+"\x00"+x.Use.ID < y.Use.Target+"\x00"+y.OwnerRef+"\x00"+y.Use.ID
	})
	mixed := []string{}
	for target, kinds := range polarities {
		if kinds["supports"] && kinds["weakens"] {
			mixed = append(mixed, target)
		}
	}
	sort.Strings(mixed)
	var linkedRows, unresolvedRows []ContextEdge
	for _, edge := range edges {
		from, to := contextOwner(edge.From), contextOwner(edge.To)
		outgoing := relevant[from]
		incoming := relevant[to] || focusID != "" && to == focusID || q.Ref != "" && (edge.To == q.Ref || edge.To == literal)
		if !outgoing && !incoming {
			continue
		}
		direction := "incoming"
		if outgoing {
			direction = "outgoing"
		}
		if outgoing && incoming {
			direction = "internal"
		}
		// Prefer direction relative to the requested endpoint over membership
		// in the broader review neighborhood (which can include both owners).
		fromFocus := q.Ref != "" && (edge.From == q.Ref || focusID != "" && from == focusID)
		toFocus := q.Ref != "" && (edge.To == q.Ref || edge.To == literal || focusID != "" && to == focusID)
		if codeQuery && edge.Kind == "constraint" && declaredContextMatch(edge.To, q.Ref, index) != "" {
			toFocus = true
		}
		if fromFocus && !toFocus {
			direction = "outgoing"
		} else if toFocus && !fromFocus {
			direction = "incoming"
		}
		if contextCodeSelector(edge.To) {
			edge.Unresolved = index.Resolve(edge.To).Kind != "exact"
		}
		if strings.HasPrefix(edge.To, "test:") || strings.HasPrefix(edge.To, "pbt:") {
			edge.Unresolved = index.ResolveCheck(edge.To).Kind != "exact"
		}
		var sourceBasis *carrier.Source
		if edge.Kind == "source" {
			for _, n := range byID[from] {
				for _, source := range entries[n].Document.Record.Sources {
					if source.Ref == edge.To && source.BodyDigest == edge.Scope {
						original := source
						sourceBasis = &original
						if literal != "" && (source.PublicationPath == literal || source.SnapshotRef == literal) && !fromFocus {
							direction = "incoming"
						}
						captured, err := carrier.ReadSourceSnapshot(v.Snapshots[source.SnapshotRef], source.SnapshotRef)
						source.SnapshotRef = ""
						captured.Provenance.SnapshotRef = ""
						if err != nil || source.SourceRevision.Kind == "unknown" || !reflect.DeepEqual(source, captured.Provenance) {
							edge.Unresolved = true
						}
					}
				}
			}
		}
		row := ContextEdge{Edge: edge, Direction: direction, Source: sourceBasis}
		linkedRows = append(linkedRows, row)
		if edge.Unresolved {
			unresolvedRows = append(unresolvedRows, row)
		}
	}
	for _, rows := range [][]ContextEdge{linkedRows, unresolvedRows} {
		sort.Slice(rows, func(a, b int) bool {
			x, y := rows[a], rows[b]
			return x.From+"\x00"+x.Kind+"\x00"+x.To+"\x00"+x.Path < y.From+"\x00"+y.Kind+"\x00"+y.To+"\x00"+y.Path
		})
	}
	needle := q.Query
	if needle == "" {
		needle = q.Ref
	}
	if contextCodeSelector(needle) {
		_, needle = selectorPath(needle)
	}
	needle = strings.ToLower(needle)
	var mentions []ContextMention
	for path, raw := range v.Files {
		if strings.HasPrefix(path, "editions/") || !strings.Contains(strings.ToLower(string(raw)), needle) {
			continue
		}
		row := ContextMention{Path: path}
		if n, ok := byPath[path]; ok {
			row.RecordID, row.Title, row.State = entries[n].Document.Record.ID, entries[n].Document.Record.Title, entries[n].State
		}
		text := []rune(string(raw))
		row.Excerpt = string(text[:min(400, len(text))])
		mentions = append(mentions, row)
	}
	sort.Slice(mentions, func(a, b int) bool { return mentions[a].Path < mentions[b].Path })
	constrained, proposed, relatedPage := contextPage(constrainedRows, q), contextPage(proposedRows, q), contextPage(relatedRows, q)
	evidence, linked, unresolved := contextPage(evidenceRows, q), contextPage(linkedRows, q), contextPage(unresolvedRows, q)
	bindingsPage, mentioned := contextPage(bindingRows, q), contextPage(mentions, q)
	data["constrained"], data["proposed"], data["related"] = constrained, proposed, relatedPage
	data["evidence"] = ContextEvidencePage{ContextPage: evidence, PolarityCounts: counts, MixedPolarityTargets: mixed}
	data["bindings"], data["linked"], data["unresolved"], data["mentioned"] = bindingsPage, linked, unresolved, mentioned
	truncated := constrained.Truncated || proposed.Truncated || relatedPage.Truncated || evidence.Truncated || linked.Truncated || unresolved.Truncated || bindingsPage.Truncated || mentioned.Truncated
	if symbols, ok := data["symbols"].(ContextPage[code.Symbol]); ok {
		truncated = truncated || symbols.Truncated
	}
	if relation, ok := data["code_related"].(map[string]any); ok {
		for _, key := range []string{"dependency_files", "affected_files", "external_dependencies"} {
			truncated = truncated || relation[key].(ContextPage[string]).Truncated
		}
	}
	data["output_truncated"] = truncated
	data["search_scope"] = map[string]any{"selector": q.Ref, "query": q.Query, "scope": "captured local project", "memory_generation": v.Generation, "memory_coverage": v.Coverage, "index_generation": index.Basis, "code_basis": index.Basis, "code_complete": index.Complete}
	if sourceLocator && !codeQuery {
		r.Kind = "results"
		if found.Document == nil {
			delete(data, "memory")
		}
		filtered := r.Diagnostics[:0]
		for _, diagnostic := range r.Diagnostics {
			if diagnostic.Code != "invalid_ref" {
				filtered = append(filtered, diagnostic)
			}
		}
		r.Diagnostics = filtered
	}
	r.Limits = append(r.Limits,
		"Declared scope matches are review candidates; semantic applicability is not inferred. Package proximity and implementation links do not establish constraints",
		"Bands are ordered by exact reference and path; mentions by path. Each band uses offset/limit (default 50, maximum 500); output_truncated is separate from captured-scope coverage",
		"Evidence polarity counts cover all matching uses, including hidden pages. Mixed polarity does not imply a contradiction across different scopes",
		"Evidence retains its original exact target and owner posture. Old targets are not transferred; code and check currentness remain unknown without a verified comparison basis")
}
