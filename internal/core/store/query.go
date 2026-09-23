package store

import (
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

type SearchHit struct {
	Path     string `json:"path"`
	RecordID string `json:"record_id,omitempty"`
	Title    string `json:"title,omitempty"`
	State    string `json:"state,omitempty"`
	Text     string `json:"text"`
}
type SearchResult struct {
	Hits       []SearchHit `json:"hits"`
	Truncated  bool        `json:"truncated"`
	Generation string      `json:"generation"`
	Coverage   string      `json:"coverage"`
}

// Search includes invalid and unknown carrier bytes. It is text retrieval,
// never applicability ranking or a governing-set selector.
func (v View) Search(query string, limit int) SearchResult {
	if limit <= 0 {
		limit = 100
	}
	out := SearchResult{Generation: v.Generation, Coverage: v.Coverage}
	metadata := map[string]carrier.Entry{}
	for i, p := range v.DocumentPaths {
		metadata[p] = v.Projection.Entries[i]
	}
	keys := make([]string, 0, len(v.Files))
	for p := range v.Files {
		if !strings.HasPrefix(p, "editions/") {
			keys = append(keys, p)
		}
	}
	sort.Strings(keys)
	for _, p := range keys {
		raw := v.Files[p]
		if !strings.Contains(strings.ToLower(string(raw)), strings.ToLower(query)) {
			continue
		}
		if len(out.Hits) == limit {
			out.Truncated = true
			break
		}
		e := metadata[p]
		out.Hits = append(out.Hits, SearchHit{Path: p, RecordID: e.Document.Record.ID, Title: e.Document.Record.Title, State: e.State, Text: string(raw)})
	}
	return out
}

type Edge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Scope      string `json:"scope,omitempty"`
	State      string `json:"state"`
	Path       string `json:"path"`
	Unresolved bool   `json:"unresolved,omitempty"`
}

// Edges are derived from each canonical typed relation exactly once. Navigation,
// implementation, constraint and evidence never collapse into one authority edge.
func (v View) Edges() []Edge {
	var out []Edge
	for i, e := range v.Projection.Entries {
		if e.State == carrier.Invalid {
			continue
		}
		r := e.Document.Record
		add := func(from, to, kind, scope string) {
			unresolved := false
			if ref, err := carrier.ParseRef(to); err == nil {
				res := v.Resolve(ref.String())
				unresolved = res.Kind != "found"
			}
			out = append(out, Edge{From: from, To: to, Kind: kind, Scope: scope, State: e.State, Path: v.DocumentPaths[i], Unresolved: unresolved})
		}
		add(e.Ref, r.About, "about", "")
		for _, target := range r.Constrains {
			add(e.Ref, target, "constraint", "")
		}
		for _, link := range r.Links {
			kind := "navigation"
			if link.Kind == "relies_on" {
				kind = "premise"
			}
			add(e.Ref, link.Target, kind, link.Reason)
		}
		for _, ref := range r.Supersedes {
			add(e.Ref, ref, "succession", r.SupersedeReason)
		}
		for _, term := range r.Terms {
			add(e.Ref, term, "term", "")
		}
		for _, source := range r.Sources {
			add(e.Ref, source.Ref, "source", source.BodyDigest)
		}
		for _, c := range r.Claims {
			from := e.Ref + "#" + c.ID
			for _, b := range c.ImplementedBy {
				add(from, b.Ref, "implementation", b.Covers)
			}
			for _, b := range c.Checks {
				add(from, b.Ref, "check", b.Covers)
			}
			for _, ref := range c.Refs {
				if !strings.ContainsAny(ref, "#:@") {
					ref = e.Ref + "#" + ref
				}
				add(from, ref, "claim_dependency", "")
			}
			for _, term := range c.Terms {
				add(from, term, "term", "")
			}
			for _, input := range c.EvidenceInputs {
				add(from, input.Ref, "evidence_input", input.Applicability)
			}
		}
		for _, u := range r.Uses {
			add(e.Ref+"#"+u.ID, u.Target, "evidence", u.Scope)
		}
	}
	return out
}
func (v View) Links(from string) []Edge {
	var out []Edge
	for _, e := range v.Edges() {
		if e.From == from {
			out = append(out, e)
		}
	}
	return out
}
func (v View) Backlinks(to string) []Edge {
	var out []Edge
	for _, e := range v.Edges() {
		if e.To == to {
			out = append(out, e)
		}
	}
	return out
}

// Resolve distinguishes durable pinned history from an unpersisted live
// computation. CurrentSnapshots support admission and live projection only;
// they can never repair a missing/corrupt historical snapshot by fallback.
func (v View) Resolve(address string) carrier.Resolution {
	ref, err := carrier.ParseRef(address)
	if err == nil && ref.Pinned() {
		if _, exists := v.Snapshots[ref.Digest]; !exists {
			return carrier.Resolution{Kind: "unresolved", Diagnostics: []carrier.Diagnostic{diag("snapshot_not_persisted", address, "Pinned history is unavailable; computed current bytes are not a durable snapshot")}}
		}
	}
	return v.Projection.Resolve(address)
}
