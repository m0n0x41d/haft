// Package source reads exact captured FPF and Engineering DPF publications.
// Retrieval is advisory: no result establishes applicability or authority.
package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

const ExtractorVersion = "haft.source-extractor/1"

// Limits bound capture and retrieval. Zero fields select bounded defaults.
type Limits struct {
	MaxFiles      int   `json:"max_files"`
	MaxBytes      int64 `json:"max_bytes"`
	MaxFileBytes  int64 `json:"max_file_bytes"`
	MaxResults    int   `json:"max_results"`
	MaxQueryBytes int   `json:"max_query_bytes"`
}

func (l Limits) normalized() (Limits, error) {
	if l.MaxFiles == 0 {
		l.MaxFiles = 256
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 64 << 20
	}
	if l.MaxFileBytes == 0 {
		l.MaxFileBytes = 32 << 20
	}
	if l.MaxResults == 0 {
		l.MaxResults = 50
	}
	if l.MaxQueryBytes == 0 {
		l.MaxQueryBytes = 4096
	}
	if l.MaxFiles < 1 || l.MaxFiles > 4096 || l.MaxBytes < 1 || l.MaxBytes > 512<<20 || l.MaxFileBytes < 1 || l.MaxFileBytes > 128<<20 || l.MaxResults < 1 || l.MaxResults > 500 || l.MaxQueryBytes < 1 || l.MaxQueryBytes > 16384 {
		return l, fmt.Errorf("invalid_source_limits")
	}
	return l, nil
}

type File struct {
	Path string
	Raw  []byte
}
type ManifestEntry struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Digest string `json:"digest"`
}
type Status struct {
	Kind                string                 `json:"kind"` // available, degraded, unavailable
	Revision            carrier.SourceRevision `json:"source_revision"`
	ExtractorVersion    string                 `json:"extractor_version"`
	Manifest            []ManifestEntry        `json:"manifest"`
	Publications        int                    `json:"publications"`
	Patterns            int                    `json:"patterns"`
	Offline             bool                   `json:"offline"`
	UpstreamCurrentness string                 `json:"upstream_currentness"`
	Diagnostics         []carrier.Diagnostic   `json:"diagnostics,omitempty"`
}
type Unit struct {
	Kind     string         `json:"kind"` // pattern, document, or captured_body during portable replay
	Title    string         `json:"title,omitempty"`
	Raw      []byte         `json:"bytes_base64"`
	Source   carrier.Source `json:"source"`
	Snapshot []byte         `json:"-"`
}
type Inspection struct {
	Kind        string               `json:"kind"` // found, unavailable, ambiguous, invalid
	Unit        *Unit                `json:"unit,omitempty"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics,omitempty"`
}
type Candidate struct {
	Ref    string         `json:"ref"`
	Kind   string         `json:"kind"`
	Title  string         `json:"title"`
	Score  int            `json:"score"` // lexical match count; never applicability or precedence
	Source carrier.Source `json:"source"`
}
type SearchResult struct {
	Kind         string               `json:"kind"` // results, insufficient_basis, unavailable, invalid
	Query        string               `json:"query"`
	Advisory     bool                 `json:"advisory"`
	Candidates   []Candidate          `json:"candidates"`
	TotalMatches int                  `json:"total_matches"`
	Truncated    bool                 `json:"truncated"`
	Diagnostics  []carrier.Diagnostic `json:"diagnostics,omitempty"`
}

type indexedUnit struct {
	ref, kind, title, publication string
	raw                           []byte
	start, end                    int
	search                        string
}

// Reader owns its captured bytes. All public results are copies, so concurrent
// reads and later changes to input files cannot change this reader's basis.
type Reader struct {
	limits  Limits
	status  Status
	units   map[string][]indexedUnit
	bad     map[string][]carrier.Diagnostic
	aliases map[string][]string
	search  []indexedUnit
}

// NewReader builds a pure immutable local-tree reader from exact publication
// bytes. It never infers a Git commit from a parent repository or caller label.
func NewReader(repository string, files []File, limits Limits) (*Reader, error) {
	l, err := limits.normalized()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(repository) == "" {
		return nil, fmt.Errorf("source_repository_required")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("source_unavailable: no publications")
	}
	if len(files) > l.MaxFiles {
		return nil, fmt.Errorf("source_file_limit")
	}
	r := &Reader{limits: l, units: map[string][]indexedUnit{}, bad: map[string][]carrier.Diagnostic{}, aliases: map[string][]string{}}
	r.status = Status{Kind: "available", ExtractorVersion: ExtractorVersion, Offline: true, UpstreamCurrentness: "unknown"}
	owned := make([]File, len(files))
	total := int64(0)
	seen := map[string]bool{}
	for i, f := range files {
		if !publicationPath(f.Path) || seen[f.Path] {
			return nil, fmt.Errorf("invalid_or_duplicate_publication: %s", f.Path)
		}
		seen[f.Path] = true
		if !utf8.Valid(f.Raw) {
			return nil, fmt.Errorf("source_invalid_utf8: %s", f.Path)
		}
		total += int64(len(f.Raw))
		if int64(len(f.Raw)) > l.MaxFileBytes || total > l.MaxBytes {
			return nil, fmt.Errorf("source_byte_limit: %s", f.Path)
		}
		owned[i] = File{Path: f.Path, Raw: bytes.Clone(f.Raw)}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].Path < owned[j].Path })
	for _, f := range owned {
		r.status.Manifest = append(r.status.Manifest, ManifestEntry{f.Path, int64(len(f.Raw)), carrier.Digest(f.Raw)})
	}
	manifest, _ := json.Marshal(struct {
		Format string          `json:"format"`
		Files  []ManifestEntry `json:"files"`
	}{"haft.source-manifest/1", r.status.Manifest})
	manifest = append(manifest, '\n')
	r.status.Revision = carrier.SourceRevision{Kind: "local_tree", Repository: repository, TreeDigest: carrier.Digest(manifest)}
	r.status.Publications = len(owned)
	for _, f := range owned {
		doc := indexedUnit{ref: f.Path, kind: "document", title: documentTitle(f.Raw, f.Path), publication: f.Path, raw: f.Raw, start: 1, end: lineCount(f.Raw)}
		r.units[f.Path] = append(r.units[f.Path], doc)
		for _, alias := range documentAliases(f.Path) {
			r.aliases[alias] = append(r.aliases[alias], f.Path)
		}
		patterns, bad := extract(f)
		for ref, ds := range bad {
			r.bad[ref] = append(r.bad[ref], ds...)
		}
		for _, u := range patterns {
			r.units[u.ref] = append(r.units[u.ref], u)
		}
		if len(patterns) == 0 && len(bad) == 0 {
			r.search = append(r.search, doc)
		}
	}
	for alias, paths := range r.aliases {
		if len(paths) > 1 {
			r.bad[alias] = append(r.bad[alias], diag("ambiguous_document", alias, "Document alias has multiple captured publications"))
		}
	}
	refs := make([]string, 0, len(r.units))
	for ref := range r.units {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		us := r.units[ref]
		if len(us) > 1 {
			for _, u := range us {
				r.bad[ref] = append(r.bad[ref], diag("ambiguous_pattern", u.publication, "Multiple bodies claim exact identity "+ref))
			}
		}
		if len(us) == 1 && us[0].kind == "pattern" && len(r.bad[ref]) == 0 {
			r.search = append(r.search, us[0])
			r.status.Patterns++
		}
	}
	badRefs := make([]string, 0, len(r.bad))
	for ref := range r.bad {
		badRefs = append(badRefs, ref)
	}
	sort.Strings(badRefs)
	for _, ref := range badRefs {
		r.status.Diagnostics = append(r.status.Diagnostics, r.bad[ref]...)
	}
	if len(r.status.Diagnostics) > 0 {
		r.status.Kind = "degraded"
	}
	for i := range r.search {
		r.search[i].search = strings.ToLower(r.search[i].ref + "\n" + r.search[i].title + "\n" + string(r.search[i].raw))
	}
	return r, nil
}

func (r *Reader) Status() Status {
	if r == nil {
		return Status{Kind: "unavailable", Offline: true, UpstreamCurrentness: "unknown", Diagnostics: []carrier.Diagnostic{diag("source_unavailable", "", "No captured source is available")}}
	}
	s := r.status
	s.Manifest = append([]ManifestEntry{}, s.Manifest...)
	s.Diagnostics = append([]carrier.Diagnostic{}, s.Diagnostics...)
	return s
}

func (r *Reader) Inspect(ref string) Inspection {
	if r == nil {
		return Inspection{Kind: "unavailable", Diagnostics: r.Status().Diagnostics}
	}
	if paths, ok := r.aliases[ref]; ok {
		if len(paths) != 1 {
			return Inspection{Kind: "ambiguous", Diagnostics: []carrier.Diagnostic{diag("ambiguous_document", ref, "Document alias has multiple captured publications")}}
		}
		ref = paths[0]
	}
	us := r.units[ref]
	if len(us) > 1 {
		return Inspection{Kind: "ambiguous", Diagnostics: append([]carrier.Diagnostic{}, r.bad[ref]...)}
	}
	if ds := r.bad[ref]; len(ds) > 0 {
		return Inspection{Kind: "invalid", Diagnostics: append([]carrier.Diagnostic{}, ds...)}
	}
	if len(us) == 0 {
		return Inspection{Kind: "unavailable", Diagnostics: []carrier.Diagnostic{diag("source_not_found", ref, "Exact source identity is absent from this captured tree; no broader fallback")}}
	}
	u, err := r.materialize(us[0])
	if err != nil {
		return Inspection{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("source_snapshot_error", ref, err.Error())}}
	}
	return Inspection{Kind: "found", Unit: &u}
}

func (r *Reader) materialize(u indexedUnit) (Unit, error) {
	s := carrier.Source{Ref: u.ref, SourceRevision: r.status.Revision, PublicationPath: u.publication, BodyDigest: carrier.Digest(u.raw), Lines: []int{u.start, u.end}}
	_, blob, digest, err := carrier.NewSourceSnapshot(u.raw, s)
	if err != nil {
		return Unit{}, err
	}
	s.SnapshotRef = digest
	return Unit{Kind: u.kind, Title: u.title, Raw: bytes.Clone(u.raw), Source: s, Snapshot: blob}, nil
}

// Search is bounded lexical source retrieval. The caller must inspect a full
// body before deciding applicability; rank supplies no selection policy.
func (r *Reader) Search(query string, limit int) SearchResult {
	out := SearchResult{Query: query, Advisory: true, Candidates: []Candidate{}}
	if r == nil {
		out.Kind = "unavailable"
		out.Diagnostics = r.Status().Diagnostics
		return out
	}
	if len(query) > r.limits.MaxQueryBytes || !utf8.ValidString(query) || limit < 0 {
		out.Kind = "invalid"
		out.Diagnostics = []carrier.Diagnostic{diag("invalid_search", "query", "Query or result limit is outside the declared bounds")}
		return out
	}
	if limit == 0 || limit > r.limits.MaxResults {
		limit = r.limits.MaxResults
	}
	terms := strings.FieldsFunc(strings.ToLower(query), func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '.' })
	unique := map[string]bool{}
	normalized := []string{}
	for _, term := range terms {
		if !unique[term] {
			unique[term] = true
			normalized = append(normalized, term)
		}
	}
	if len(normalized) > 64 {
		out.Kind = "invalid"
		out.Diagnostics = []carrier.Diagnostic{diag("invalid_search", "query", "Lexical search accepts at most 64 distinct query terms")}
		return out
	}
	if len(normalized) == 0 {
		out.Kind = "insufficient_basis"
		return out
	}
	type scored struct {
		u     indexedUnit
		score int
	}
	matches := []scored{}
	for _, u := range r.search {
		score := 0
		for _, term := range normalized {
			if strings.Contains(u.search, term) {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, scored{u, score})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].u.ref != matches[j].u.ref {
			return matches[i].u.ref < matches[j].u.ref
		}
		return matches[i].u.publication < matches[j].u.publication
	})
	out.TotalMatches = len(matches)
	out.Truncated = len(matches) > limit
	if len(matches) > limit {
		matches = matches[:limit]
	}
	for _, m := range matches {
		u, err := r.materialize(m.u)
		if err != nil {
			out.Kind = "invalid"
			out.Diagnostics = append(out.Diagnostics, diag("source_snapshot_error", m.u.ref, err.Error()))
			return out
		}
		out.Candidates = append(out.Candidates, Candidate{m.u.ref, m.u.kind, m.u.title, m.score, u.Source})
	}
	out.Diagnostics = append([]carrier.Diagnostic{}, r.status.Diagnostics...)
	out.Kind = "results"
	if len(out.Candidates) == 0 {
		out.Kind = "insufficient_basis"
	}
	return out
}

// InspectSnapshot replays exact portable bytes without accessing any original
// tree. A snapshot can hold an excerpt, so replay never upgrades it to a pattern.
func InspectSnapshot(raw []byte, digest string) Inspection {
	if len(raw) == 0 {
		return Inspection{Kind: "unavailable", Diagnostics: []carrier.Diagnostic{diag("source_snapshot_unavailable", digest, "No portable source snapshot bytes are available")}}
	}
	s, err := carrier.ReadSourceSnapshot(raw, digest)
	if err != nil {
		return Inspection{Kind: "invalid", Diagnostics: []carrier.Diagnostic{diag("invalid_source_snapshot", digest, err.Error())}}
	}
	s.Provenance.SnapshotRef = digest
	return Inspection{Kind: "found", Unit: &Unit{Kind: "captured_body", Raw: bytes.Clone(s.Raw), Source: s.Provenance, Snapshot: bytes.Clone(raw)}}
}

func diag(code, p, msg string) carrier.Diagnostic {
	return carrier.Diagnostic{Code: code, Path: p, Message: msg, Severity: "error"}
}
func publicationPath(p string) bool {
	if p == "" || path.Clean(p) != p || strings.ContainsAny(p, "\\\x00\r\n") || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") {
		return false
	}
	if !strings.Contains(p, "/") {
		return strings.EqualFold(p, "readme.md") || p == "USING-FPF.md" || p == "FPF-Spec.md"
	}
	return strings.HasPrefix(p, "Engineering DPF Suite/") && strings.HasSuffix(strings.ToLower(p), ".md")
}
func documentAliases(p string) []string {
	switch {
	case !strings.Contains(p, "/") && strings.EqualFold(p, "readme.md"):
		return []string{"fpf-ecosystem"}
	case p == "USING-FPF.md":
		return []string{"fpf-usage-guide"}
	case p == "Engineering DPF Suite/README.md":
		return []string{"engineering-suite"}
	case p == "Engineering DPF Suite/ENGINEERING-DPF-SUITE-REFERENCE.md":
		return []string{"engineering-suite-reference"}
	}
	return nil
}
func lineCount(b []byte) int {
	n := bytes.Count(b, []byte{'\n'})
	if len(b) == 0 || b[len(b)-1] != '\n' {
		n++
	}
	return n
}
func documentTitle(b []byte, fallback string) string {
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("# ")) {
			return strings.TrimSpace(string(line[2:]))
		}
	}
	return fallback
}
