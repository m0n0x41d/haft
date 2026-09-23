package carrier

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const (
	Active     = "active"
	Proposed   = "proposed"
	Superseded = "superseded"
	Contested  = "contested"
	Historical = "historical"
	Invalid    = "invalid"
)

type Entry struct {
	Document    Document     `json:"document"`
	Ref         string       `json:"ref"`
	State       string       `json:"state"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type Projection struct {
	Entries     []Entry          `json:"entries"`
	ByID        map[string][]int `json:"by_id"`
	Aliases     map[string][]int `json:"aliases"`
	snapshots   map[string][]byte
	lineage     []int
	lineageByID map[string]int
}
type Resolution struct {
	Kind        string       `json:"kind"` // found, absent, unresolved, invalid, conflict
	Entries     []Entry      `json:"entries,omitempty"`
	Document    *Document    `json:"document,omitempty"`
	Claim       *Claim       `json:"claim,omitempty"`
	Use         *EvidenceUse `json:"use,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Project consumes a captured collection, not a mutable store. File order and
// timestamps never choose a winner. Unknown and invalid bytes remain in Entries.
func Project(documents []Document, snapshots map[string][]byte) Projection {
	return ProjectCaptured(documents, snapshots, snapshots)
}

// ProjectCaptured separates live edition verification from exact pinned
// dependency resolution. A current snapshot proves the captured live bytes but
// cannot repair absent pinned history. Callers preparing a publication may pass
// the exact persisted-plus-staged snapshot set as pinnedSnapshots; ordinary
// readers pass only persisted snapshots. Project retains its pure single-set API.
func ProjectCaptured(documents []Document, currentSnapshots, pinnedSnapshots map[string][]byte) Projection {
	captured := make(map[string][]byte, len(currentSnapshots)+len(pinnedSnapshots))
	for digest, raw := range currentSnapshots {
		captured[digest] = raw
	}
	for digest, raw := range pinnedSnapshots {
		captured[digest] = raw
	}
	invalid := map[int][]Diagnostic{}
	// Reject resolved forbidden dependencies before relying on their successors.
	// Rebuilding after each newly invalid record restores any predecessor that
	// the invalid successor would otherwise have suppressed.
	for pass := 0; pass <= len(documents); pass++ {
		p := projectOnce(documents, captured, pinnedSnapshots, invalid)
		changed := false
		for i, e := range p.Entries {
			if e.State == Invalid {
				continue
			}
			ds := p.ValidateReferences(e.Document.Record)
			p.Entries[i].Diagnostics = append(p.Entries[i].Diagnostics, ds...)
			for _, d := range ds {
				if d.Code == "quadrant_dependency" {
					invalid[i] = append(invalid[i], d)
					changed = true
				}
			}
		}
		if !changed {
			return p
		}
	}
	return projectOnce(documents, captured, pinnedSnapshots, invalid)
}

func projectOnce(documents []Document, captured, snapshots map[string][]byte, invalid map[int][]Diagnostic) Projection {
	p := Projection{ByID: map[string][]int{}, Aliases: map[string][]int{}, snapshots: map[string][]byte{}, lineageByID: map[string]int{}}
	for k, v := range snapshots {
		p.snapshots[k] = bytes.Clone(v)
	}
	for i, doc := range documents {
		// Reparse bytes so a caller-supplied Record cannot disagree with Raw.
		d := Parse(doc.Raw)
		d.Edition = doc.Edition
		if d.Edition == "" {
			_, _, d.Edition, _ = NewSnapshot(d.Raw, InterpretationBasis{})
		}
		e := Entry{Document: d, Ref: d.Record.ID + "@" + d.Edition, State: d.Record.Status, Diagnostics: append([]Diagnostic{}, d.Diagnostics...)}
		if !d.Valid() {
			e.State = Invalid
		} else if d.Record.Origin == "migrated_9x" {
			e.State = Historical
		}
		if ds := invalid[i]; len(ds) > 0 {
			e.State = Invalid
			e.Diagnostics = append(e.Diagnostics, ds...)
		}
		if !ValidDigest(d.Edition) {
			e.State = Invalid
			e.Diagnostics = append(e.Diagnostics, diagnostic("invalid_edition", "", "Captured edition is not a full digest"))
		}
		if doc.Edition != "" {
			material, exists := captured[doc.Edition]
			if !exists {
				e.State = Invalid
				e.Diagnostics = append(e.Diagnostics, diagnostic("unresolved_captured_edition", "edition", "An explicit captured edition requires its exact snapshot bytes"))
			} else if captured, err := ReadSnapshot(material, doc.Edition); err != nil {
				e.State = Invalid
				e.Diagnostics = append(e.Diagnostics, diagnostic("invalid_captured_edition", "edition", err.Error()))
			} else if !bytes.Equal(captured.Raw, d.Raw) {
				e.State = Invalid
				e.Diagnostics = append(e.Diagnostics, diagnostic("captured_edition_mismatch", "edition", "Captured snapshot bytes differ from the supplied live carrier; support cannot transfer"))
			}
		}
		p.Entries = append(p.Entries, e)
		p.ByID[d.Record.ID] = append(p.ByID[d.Record.ID], i)
		p.lineage = append(p.lineage, i)
	}
	mark := func(i int, code, path, msg string) {
		p.Entries[i].Diagnostics = append(p.Entries[i].Diagnostics, diagnostic(code, path, msg))
		p.Entries[i].State = Invalid
	}
	for id, is := range p.ByID {
		if len(is) > 1 {
			for _, i := range is {
				mark(i, "duplicate_id", "id", "More than one carrier has ID "+id)
			}
		}
	}
	parents := make([][]int, len(p.Entries))
	for i, e := range p.Entries {
		if e.State == Invalid {
			continue
		}
		for j, s := range e.Document.Record.Supersedes {
			ref, _ := ParseRef(s)
			old, err := resolvePinned(ref, p.snapshots)
			field := fmt.Sprintf("supersedes[%d]", j)
			if err != nil {
				mark(i, "unresolved_predecessor", field, err.Error())
				continue
			}
			if old.Record.Kind != e.Document.Record.Kind || old.Record.About != e.Document.Record.About || old.Record.Kind == "decision" && old.Record.Question != e.Document.Record.Question {
				mark(i, "lineage_mismatch", field, "Ordinary supersession preserves kind, subject and decision question")
				continue
			}
			for _, idx := range p.ByID[ref.RecordID] {
				parents[i] = append(parents[i], idx)
			}
		}
	}
	// Detect cycles before any active head is suppressed.
	colors := make([]int, len(p.Entries))
	stack := []int{}
	var visit func(int)
	visit = func(i int) {
		if colors[i] == 2 {
			return
		}
		if colors[i] == 1 {
			start := 0
			for k, n := range stack {
				if n == i {
					start = k
					break
				}
			}
			for _, n := range stack[start:] {
				mark(n, "succession_cycle", "supersedes", "Succession cycle has no current head")
			}
			return
		}
		colors[i] = 1
		stack = append(stack, i)
		for _, j := range parents[i] {
			visit(j)
		}
		stack = stack[:len(stack)-1]
		colors[i] = 2
	}
	for i := range p.Entries {
		visit(i)
	}
	// A proposal's active basis must be named explicitly when accepting it.
	for i, e := range p.Entries {
		if e.State != Active {
			continue
		}
		direct := map[string]bool{}
		for _, s := range e.Document.Record.Supersedes {
			direct[s] = true
		}
		seen := map[string]bool{}
		var proposalBasis func(string)
		proposalBasis = func(s string) {
			if seen[s] {
				return
			}
			seen[s] = true
			r, err := ParseRef(s)
			if err != nil {
				return
			}
			d, err := resolvePinned(r, p.snapshots)
			if err != nil {
				return
			}
			if d.Record.Status == "active" {
				if !direct[s] {
					mark(i, "missing_predecessor", "supersedes", "Accepted proposal omits its explicit active predecessor: "+s)
				}
				return
			}
			for _, ancestor := range d.Record.Supersedes {
				proposalBasis(ancestor)
			}
		}
		for _, s := range e.Document.Record.Supersedes {
			r, _ := ParseRef(s)
			d, err := resolvePinned(r, p.snapshots)
			if err == nil && d.Record.Status == "proposed" {
				proposalBasis(s)
			}
		}
	}
	// Union lineage independently of head suppression, including missing live
	// ancestors shared by two branches via their exact predecessor IDs.
	var find func(int) int
	find = func(i int) int {
		if p.lineage[i] != i {
			p.lineage[i] = find(p.lineage[i])
		}
		return p.lineage[i]
	}
	union := func(i, j int) {
		a, b := find(i), find(j)
		if a != b {
			p.lineage[b] = a
		}
	}
	ancestorOwner := map[string]int{}
	for i, e := range p.Entries {
		if e.State == Invalid {
			continue
		}
		var walk func(string, map[string]bool)
		walk = func(address string, seen map[string]bool) {
			if seen[address] {
				return
			}
			seen[address] = true
			ref, err := ParseRef(address)
			if err != nil {
				return
			}
			id := ref.RecordID
			if j, ok := ancestorOwner[id]; ok {
				union(i, j)
			} else {
				ancestorOwner[id] = i
			}
			if ref.Pinned() {
				if d, err := resolvePinned(ref, p.snapshots); err == nil {
					for _, ancestor := range d.Record.Supersedes {
						walk(ancestor, seen)
					}
				}
			}
		}
		walk(e.Document.Record.ID, map[string]bool{})
		for _, s := range e.Document.Record.Supersedes {
			walk(s, map[string]bool{})
		}
		for _, j := range parents[i] {
			if p.Entries[j].State != Invalid {
				union(i, j)
			}
		}
	}
	// Only directly named, unchanged predecessor editions are suppressed.
	for i, e := range p.Entries {
		if e.State == Invalid || e.State == Historical {
			continue
		}
		for _, s := range e.Document.Record.Supersedes {
			r, _ := ParseRef(s)
			for _, j := range p.ByID[r.RecordID] {
				old := &p.Entries[j]
				if old.State == Invalid || old.State == Historical {
					continue
				}
				if old.Ref != s {
					p.Entries[i].Diagnostics = append(p.Entries[i].Diagnostics, Diagnostic{Code: "predecessor_edition_changed", Path: "supersedes", Message: "Live predecessor differs from its named edition; current content is not retired", Severity: "warning"})
					continue
				}
				if e.Document.Record.Status == Active || old.Document.Record.Status == "proposed" {
					old.State = Superseded
				}
			}
		}
	}
	groups := map[int][]int{}
	for i, e := range p.Entries {
		p.lineage[i] = find(i)
		if e.State == Active {
			groups[find(i)] = append(groups[find(i)], i)
		}
	}
	for id, owner := range ancestorOwner {
		p.lineageByID[id] = find(owner)
	}
	for _, is := range groups {
		if len(is) > 1 {
			for _, i := range is {
				p.Entries[i].State = Contested
				p.Entries[i].Diagnostics = append(p.Entries[i].Diagnostics, Diagnostic{Code: "competing_heads", Message: "Multiple active heads require explicit resolution; no winner is inferred", Severity: "warning"})
			}
		}
	}
	for i, e := range p.Entries {
		if e.State == Invalid || e.Document.Record.Kind != "spec" {
			continue
		}
		for _, a := range append([]string{e.Document.Record.Slug}, e.Document.Record.Aliases...) {
			if a != "" {
				p.Aliases[a] = append(p.Aliases[a], i)
			}
		}
	}
	for alias, is := range p.Aliases {
		lines := map[int]bool{}
		for _, i := range is {
			lines[p.lineage[i]] = true
		}
		if len(lines) > 1 {
			for _, i := range is {
				p.Entries[i].Diagnostics = append(p.Entries[i].Diagnostics, Diagnostic{Code: "alias_conflict", Path: "aliases", Message: "Alias belongs to independent lineages: " + alias, Severity: "warning"})
			}
		}
	}
	return p
}

func resolvePinned(ref Ref, snapshots map[string][]byte) (Document, error) {
	var d Document
	raw, ok := snapshots[ref.Digest]
	if !ok {
		return d, fmt.Errorf("missing_snapshot: %s", ref.String())
	}
	s, err := ReadSnapshot(raw, ref.Digest)
	if err != nil {
		return d, err
	}
	d = s.Document()
	d.Edition = ref.Digest
	if d.Record.ID != ref.RecordID {
		return d, fmt.Errorf("snapshot_record_mismatch: %s", ref.String())
	}
	if !d.Valid() {
		return d, fmt.Errorf("invalid_snapshot_carrier: %s", ref.String())
	}
	return d, nil
}

// Heads returns every current active/contested head, or proposed heads only when
// no active head exists. References include the exact captured interpretation.
func (p Projection) Heads(id string) []string {
	is := p.ByID[id]
	line, known := p.lineageByID[id]
	if !known {
		if len(is) == 0 {
			return nil
		}
		line = p.lineage[is[0]]
	}
	var active, proposed []string
	for i, e := range p.Entries {
		if p.lineage[i] != line {
			continue
		}
		if e.State == Active || e.State == Contested {
			active = append(active, e.Ref)
		} else if e.State == Proposed {
			proposed = append(proposed, e.Ref)
		}
	}
	if len(active) > 0 {
		sort.Strings(active)
		return active
	}
	sort.Strings(proposed)
	return proposed
}
func (p Projection) Resolve(s string) Resolution {
	r, err := ParseRef(s)
	if err != nil {
		return Resolution{Kind: "invalid", Diagnostics: []Diagnostic{diagnostic("invalid_ref", "", err.Error())}}
	}
	if r.Pinned() {
		d, err := resolvePinned(r, p.snapshots)
		if err != nil {
			return Resolution{Kind: "unresolved", Diagnostics: []Diagnostic{diagnostic("unresolved_snapshot", "", err.Error())}}
		}
		return selectClaim(d, r)
	}
	is := p.ByID[r.RecordID]
	if r.Alias != "" {
		is = p.Aliases[r.Alias]
	}
	if len(is) == 0 {
		if line, ok := p.lineageByID[r.RecordID]; ok {
			for i := range p.Entries {
				if p.lineage[i] == line {
					is = append(is, i)
					break
				}
			}
		}
		if len(is) == 0 {
			return Resolution{Kind: "absent"}
		}
	}
	lines := map[int]bool{}
	for _, i := range is {
		lines[p.lineage[i]] = true
	}
	if len(lines) > 1 {
		return Resolution{Kind: "conflict", Diagnostics: []Diagnostic{diagnostic("alias_conflict", "", "Alias names independent record lineages")}}
	}
	var entries []Entry
	heads := p.Heads(p.Entries[is[0]].Document.Record.ID)
	for _, h := range heads {
		for _, e := range p.Entries {
			if e.Ref == h {
				entries = append(entries, e)
			}
		}
	}
	if len(entries) > 1 {
		return Resolution{Kind: "conflict", Entries: entries}
	}
	if len(entries) == 0 {
		e := p.Entries[is[0]]
		if e.State == Invalid {
			return Resolution{Kind: "invalid", Entries: []Entry{e}, Diagnostics: e.Diagnostics}
		}
		return selectClaim(e.Document, r)
	}
	res := selectClaim(entries[0].Document, r)
	res.Entries = entries
	return res
}
func selectClaim(d Document, r Ref) Resolution {
	res := Resolution{Kind: "found", Document: &d}
	if r.ClaimID == "" {
		return res
	}
	if d.Record.Kind == "evidence" {
		for _, u := range d.Record.Uses {
			if u.ID == r.ClaimID {
				res.Use = &u
				return res
			}
		}
	} else {
		for _, c := range d.Record.Claims {
			if c.ID == r.ClaimID {
				res.Claim = &c
				return res
			}
		}
	}
	res.Kind = "unresolved"
	res.Diagnostics = []Diagnostic{diagnostic("missing_claim", "", "The exact edition has no claim/use "+r.ClaimID)}
	return res
}

// ValidateSuccessor is an admission check over a transaction-current capture.
// expectedHeads is the complete exact live head set, not merely record IDs.
func ValidateSuccessor(r Record, p Projection, snapshots map[string][]byte, expectedHeads []string) []Diagnostic {
	ds := Validate(r, false)
	if HasErrors(ds) {
		return ds
	}
	if len(p.ByID[r.ID]) > 0 {
		ds = append(ds, diagnostic("duplicate_id", "id", "Remember creates new IDs; an existing carrier is never overwritten"))
	}
	actual := map[string]bool{}
	direct := map[string]bool{}
	for i, s := range r.Supersedes {
		direct[s] = true
		ref, _ := ParseRef(s)
		d, err := resolvePinned(ref, snapshots)
		field := fmt.Sprintf("supersedes[%d]", i)
		if err != nil {
			ds = append(ds, diagnostic("unresolved_predecessor", field, err.Error()))
			continue
		}
		if d.Record.Kind != r.Kind || d.Record.About != r.About || r.Kind == "decision" && d.Record.Question != r.Question {
			ds = append(ds, diagnostic("lineage_mismatch", field, "Supersession must preserve kind, subject and decision question"))
		}
		for _, head := range p.Heads(ref.RecordID) {
			actual[head] = true
		}
		if is := p.ByID[ref.RecordID]; len(is) > 0 {
			matches := false
			for _, j := range is {
				if p.Entries[j].Ref == s {
					matches = true
				}
			}
			if !matches {
				ds = append(ds, diagnostic("stale_predecessor", field, "Predecessor bytes or interpretation have changed"))
			}
		}
	}
	expected := map[string]bool{}
	for _, s := range expectedHeads {
		ref, err := ParseRef(s)
		if err != nil || !ref.Pinned() || ref.ClaimID != "" {
			ds = append(ds, diagnostic("invalid_expected_head", "expected_heads", "Expected heads require exact whole-record refs"))
		}
		if expected[s] {
			ds = append(ds, diagnostic("duplicate_expected_head", "expected_heads", "Expected heads must be a set"))
		}
		expected[s] = true
	}
	if !sameSet(actual, expected) {
		ds = append(ds, diagnostic("head_conflict", "expected_heads", "Transaction-current head set differs from the explicitly expected set"))
	}
	if r.Status == "active" {
		for s := range actual {
			if !direct[s] {
				ds = append(ds, diagnostic("missing_predecessor", "supersedes", "Accepted successor must explicitly replace current head "+s))
			}
		}
	}
	if r.Kind == "spec" {
		ancestorLines := map[int]bool{}
		for _, s := range r.Supersedes {
			ref, _ := ParseRef(s)
			if line, ok := p.lineageByID[ref.RecordID]; ok {
				ancestorLines[line] = true
			}
			for _, i := range p.ByID[ref.RecordID] {
				ancestorLines[p.lineage[i]] = true
			}
		}
		for _, alias := range append([]string{r.Slug}, r.Aliases...) {
			for _, i := range p.Aliases[alias] {
				if !ancestorLines[p.lineage[i]] {
					ds = append(ds, diagnostic("alias_conflict", "aliases", "Alias belongs to an independent lineage: "+alias))
					break
				}
			}
		}
		for _, s := range r.Supersedes {
			ref, _ := ParseRef(s)
			d, err := resolvePinned(ref, snapshots)
			if err != nil {
				continue
			}
			if d.Record.Retirement != nil && len(r.Claims) > 0 {
				ds = append(ds, diagnostic("retired_lineage", "claims", "A retired section requires a new lineage"))
			}
			for _, id := range d.Record.RetiredClaimIDs {
				found := false
				for _, x := range r.RetiredClaimIDs {
					if x == id {
						found = true
					}
				}
				if !found {
					ds = append(ds, diagnostic("retired_id_lost", "retired_claim_ids", "Successor must preserve tombstone "+id))
				}
			}
			for _, old := range d.Record.Claims {
				present := false
				for _, c := range r.Claims {
					if c.ID == old.ID {
						present = true
					}
				}
				for _, id := range r.RetiredClaimIDs {
					if id == old.ID {
						present = true
					}
				}
				if !present {
					ds = append(ds, diagnostic("removed_claim_not_retired", "retired_claim_ids", "Removed claim requires a stable tombstone: "+old.ID))
				}
			}
		}
	}
	ds = append(ds, p.ValidateReferences(r)...)
	return ds
}
func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for s := range a {
		if !b[s] {
			return false
		}
	}
	return true
}

// UsesFor returns only exact target matches. A live address or another edition,
// including one with the same claim ID, never inherits an earlier pass.
func (p Projection) UsesFor(target string) []EvidenceUse {
	r, err := ParseRef(target)
	if err != nil || !r.Pinned() {
		return nil
	}
	if p.Resolve(target).Kind != "found" {
		return nil
	}
	var out []EvidenceUse
	for _, e := range p.Entries {
		if (e.State != Active && e.State != Contested) || e.Document.Record.Kind != "evidence" {
			continue
		}
		for _, u := range e.Document.Record.Uses {
			if u.Target == target {
				out = append(out, u)
			}
		}
	}
	return out
}

type EvidenceUseMatch struct {
	OwnerRef     string      `json:"owner_ref"`
	State        string      `json:"state"`
	TargetStatus string      `json:"target_status"`
	Use          EvidenceUse `json:"use"`
}

// EvidenceUsesFor includes historical and superseded evidence explicitly with
// its owning edition and projected state. It never reassigns an exact target.
func (p Projection) EvidenceUsesFor(target string) []EvidenceUseMatch {
	r, err := ParseRef(target)
	if err != nil || !r.Pinned() {
		return nil
	}
	var out []EvidenceUseMatch
	targetStatus := p.Resolve(target).Kind
	for _, e := range p.Entries {
		if e.State == Invalid || e.Document.Record.Kind != "evidence" {
			continue
		}
		for _, u := range e.Document.Record.Uses {
			if u.Target == target {
				out = append(out, EvidenceUseMatch{OwnerRef: e.Ref, State: e.State, TargetStatus: targetStatus, Use: u})
			}
		}
	}
	return out
}

// ValidateReferences checks resolvability and quadrant separation on exact
// captured endpoints. It cannot decide whether a declared dependency is true.
func (p Projection) ValidateReferences(r Record) []Diagnostic {
	var ds []Diagnostic
	if r.OptionsRef != "" {
		resolved := p.Resolve(r.OptionsRef)
		if resolved.Kind != "found" || resolved.Document == nil || resolved.Document.Record.Kind != "options" {
			ds = append(ds, diagnostic("unresolved_options", "options_ref", "Exact selected option set is unavailable"))
		}
	}
	for i, link := range r.Links {
		ref, err := ParseRef(link.Target)
		if err == nil && ref.Pinned() && p.Resolve(link.Target).Kind != "found" {
			ds = append(ds, diagnostic("unresolved_pinned_link", fmt.Sprintf("links[%d].target", i), "Exact linked edition is unavailable; no live fallback"))
		}
	}
	for i, c := range r.Claims {
		for j, s := range c.Refs {
			if !strings.ContainsAny(s, "#:@") {
				continue
			}
			res := p.Resolve(s)
			field := fmt.Sprintf("claims[%d].refs[%d]", i, j)
			if res.Kind != "found" || res.Claim == nil {
				ds = append(ds, diagnostic("unresolved_claim", field, "Claim dependency does not resolve uniquely"))
			} else if !AllowedDependency(c.Kind, res.Claim.Kind) {
				ds = append(ds, diagnostic("quadrant_dependency", field, "External dependency violates quadrant separation"))
			}
		}
		for j, input := range c.EvidenceInputs {
			res := p.Resolve(input.Ref)
			if res.Kind != "found" || res.Use == nil {
				ds = append(ds, diagnostic("unresolved_evidence_input", fmt.Sprintf("claims[%d].evidence_inputs[%d]", i, j), "Exact evidence use is unavailable"))
			}
		}
	}
	for i, u := range r.Uses {
		res := p.Resolve(u.Target)
		if res.Kind != "found" {
			ds = append(ds, diagnostic("unresolved_evidence_target", fmt.Sprintf("uses[%d].target", i), "Exact evidence target is unavailable; no live fallback"))
		}
	}
	return ds
}
