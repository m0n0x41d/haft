package change

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

type Revision struct {
	Action     string
	ID         string
	CreatedAt  string
	Reason     string
	CurrentRef string
	Patches    []SectionPatch
	Intent     *string
	Tasks      *[]Task
	// Synced is supplied by the application from publication history, not a task
	// checkbox. It only affects an archive warning, never spec acceptance.
	Synced  bool
	Receipt *carrier.WriteReceipt
}
type RevisionResult struct {
	Kind        string               `json:"kind"`
	Raw         []byte               `json:"raw,omitempty"`
	Change      Change               `json:"change"`
	Preview     *PreviewResult       `json:"preview,omitempty"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics,omitempty"`
}

// Revise creates a new change carrier. Even archiving never edits its old bytes.
// Rebase supplies all new exact bases/operations and validates their new preview.
func Revise(baseRef string, snapshot []byte, edit Revision, bases map[string]Basis) RevisionResult {
	bad := func(code, msg string) RevisionResult {
		return RevisionResult{Kind: "conflict", Diagnostics: []carrier.Diagnostic{diag(code, "", msg)}}
	}
	id, hash, err := ParseRef(baseRef)
	if err != nil {
		return bad("invalid_change_ref", err.Error())
	}
	if edit.CurrentRef != baseRef {
		return bad("concurrent_change_revision", "Selected change head moved or is unknown")
	}
	s, err := carrier.ReadSnapshot(snapshot, hash)
	if err != nil {
		return bad("invalid_change_snapshot", err.Error())
	}
	d := Parse(s.Raw)
	if carrier.HasErrors(d.Diagnostics) || d.Change.ID != id {
		return bad("invalid_change_snapshot", "Snapshot is not the addressed valid change")
	}
	if edit.ID == id || !ValidID(edit.ID) {
		return bad("invalid_successor_id", "A change revision must have a new ID")
	}
	if strings.TrimSpace(edit.Reason) == "" {
		return bad("required", "Revision needs a reason")
	}
	c := d.Change
	c.ID = edit.ID
	c.CreatedAt = edit.CreatedAt
	c.Supersedes = []string{baseRef}
	c.SupersedeReason = edit.Reason
	c.WriteReceipt = edit.Receipt
	result := RevisionResult{Kind: "ready"}
	switch edit.Action {
	case "archive":
		c.State = "archived"
		if !edit.Synced {
			result.Diagnostics = append(result.Diagnostics, carrier.Diagnostic{Code: "unsynced_change", Message: "Archiving does not apply its spec delta", Severity: "warning"})
		}
		for _, t := range c.Tasks {
			if !t.Done {
				result.Diagnostics = append(result.Diagnostics, carrier.Diagnostic{Code: "unfinished_task", Path: t.ID, Message: "Task remains unfinished in archived history", Severity: "warning"})
			}
		}
	case "reopen":
		c.State = "open"
	case "rebase":
		if d.Change.State != "open" {
			return bad("archived_change", "Reopen before rebasing")
		}
		if edit.Patches == nil {
			return bad("explicit_rebase_required", "Supply explicitly resolved patches with their new exact bases")
		}
		c.Patches = edit.Patches
	case "update":
		if d.Change.State != "open" {
			return bad("archived_change", "Reopen before updating")
		}
		if edit.Patches != nil {
			if !sameBases(c.Patches, edit.Patches) {
				return bad("explicit_rebase_required", "Changing the authored base set requires rebase and a successful preview on those exact bases")
			}
			c.Patches = edit.Patches
		}
	default:
		return bad("unsupported_action", "Unknown change revision action")
	}
	if edit.Action == "update" || edit.Action == "rebase" {
		if edit.Intent != nil {
			c.Intent = *edit.Intent
		}
		if edit.Tasks != nil {
			c.Tasks = *edit.Tasks
		}
	}
	result.Diagnostics = append(result.Diagnostics, Validate(c)...)
	if carrier.HasErrors(result.Diagnostics) {
		result.Kind = "invalid"
		return result
	}
	if edit.Action == "rebase" {
		preview := Preview(c, bases)
		result.Preview = &preview
		if preview.Kind != "ready" {
			result.Kind = "conflict"
			result.Diagnostics = append(result.Diagnostics, preview.Diagnostics...)
			return result
		}
	}
	raw, err := Encode(c, bytes.Clone(d.Body))
	if err != nil {
		return bad("encode", err.Error())
	}
	result.Raw = raw
	result.Change = Parse(raw).Change
	return result
}

func sameBases(a, b []SectionPatch) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int, len(a))
	for _, p := range a {
		set[p.Base]++
	}
	for _, p := range b {
		set[p.Base]--
	}
	for _, count := range set {
		if count != 0 {
			return false
		}
	}
	return true
}

// Heads derives competing current change revisions. It never chooses by date.
// A malformed/missing exact predecessor is diagnosed, not guessed by key alone.
type HeadResult struct {
	Kind        string               `json:"kind"`
	Heads       []Document           `json:"heads,omitempty"`
	Diagnostics []carrier.Diagnostic `json:"diagnostics,omitempty"`
}

func Heads(key string, docs []Document, snapshots map[string][]byte) HeadResult {
	result := HeadResult{Kind: "absent"}
	byID := map[string][]Document{}
	for _, input := range docs {
		d := Parse(input.Raw)
		if d.Change.ChangeKey == key {
			byID[d.Change.ID] = append(byID[d.Change.ID], d)
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// Snapshot nodes retain exact edition identity; the separate ID graph detects
	// reuse/cycles even when two historical editions have different byte hashes.
	type node struct {
		doc     Document
		valid   bool
		parents []string
	}
	nodes := map[string]*node{}
	edges := map[string][]string{}
	var load func(string) *node
	load = func(ref string) *node {
		if n, ok := nodes[ref]; ok {
			return n
		}
		n := &node{}
		nodes[ref] = n
		old, hash, err := ParseRef(ref)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, diag("invalid_predecessor", ref, err.Error()))
			return n
		}
		snap, err := carrier.ReadSnapshot(snapshots[hash], hash)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, diag("unresolved_predecessor", ref, err.Error()))
			return n
		}
		n.doc = Parse(snap.Raw)
		if carrier.HasErrors(n.doc.Diagnostics) || n.doc.Change.ID != old || n.doc.Change.ChangeKey != key {
			result.Diagnostics = append(result.Diagnostics, diag("change_lineage_mismatch", ref, "Snapshot predecessor is not the addressed valid change in this lineage"))
			return n
		}
		if live := byID[old]; len(live) > 1 {
			result.Diagnostics = append(result.Diagnostics, diag("duplicate_change_id", old, "Snapshot predecessor has competing live carrier bytes"))
			return n
		} else if len(live) == 1 && !bytes.Equal(live[0].Raw, snap.Raw) {
			result.Diagnostics = append(result.Diagnostics, diag("change_predecessor_changed", old, "Live predecessor differs from its pinned bytes"))
			return n
		}
		n.valid = true
		n.parents = append([]string{}, n.doc.Change.Supersedes...)
		for _, parent := range n.parents {
			p := load(parent)
			if p.doc.Change.ID != "" {
				edges[old] = append(edges[old], p.doc.Change.ID)
			}
		}
		return n
	}
	roots := map[string]*node{}
	for _, id := range ids {
		group := byID[id]
		if len(group) != 1 {
			result.Diagnostics = append(result.Diagnostics, diag("duplicate_change_id", id, "Multiple carriers claim the same change ID"))
			continue
		}
		d := group[0]
		if carrier.HasErrors(d.Diagnostics) {
			result.Diagnostics = append(result.Diagnostics, d.Diagnostics...)
			continue
		}
		n := &node{doc: d, valid: true, parents: append([]string{}, d.Change.Supersedes...)}
		roots[id] = n
		for _, parent := range n.parents {
			p := load(parent)
			if p.doc.Change.ID != "" {
				edges[id] = append(edges[id], p.doc.Change.ID)
			}
		}
	}
	colors := map[string]int{}
	cyclic := map[string]bool{}
	stack := []string{}
	var visit func(string)
	visit = func(id string) {
		if colors[id] == 1 {
			for i := len(stack) - 1; i >= 0; i-- {
				cyclic[stack[i]] = true
				if stack[i] == id {
					break
				}
			}
			result.Diagnostics = append(result.Diagnostics, diag("change_succession_cycle", id, "Change revision IDs form a cycle through exact captured history"))
			return
		}
		if colors[id] == 2 {
			return
		}
		colors[id] = 1
		stack = append(stack, id)
		for _, parent := range edges[id] {
			visit(parent)
		}
		stack = stack[:len(stack)-1]
		colors[id] = 2
	}
	for _, id := range ids {
		visit(id)
	}
	// Propagate invalid history without trusting a valid-looking descendant. The
	// finite node set bounds this pass even when the captured graph has a cycle.
	all := make([]*node, 0, len(nodes)+len(roots))
	for _, n := range nodes {
		all = append(all, n)
	}
	for _, n := range roots {
		all = append(all, n)
	}
	for _, n := range all {
		if cyclic[n.doc.Change.ID] {
			n.valid = false
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range all {
			if !n.valid {
				continue
			}
			for _, parent := range n.parents {
				if p := nodes[parent]; p == nil || !p.valid {
					n.valid = false
					changed = true
					break
				}
			}
		}
	}
	suppressed := map[string]bool{}
	visited := map[string]bool{}
	var suppress func(string)
	suppress = func(ref string) {
		if visited[ref] {
			return
		}
		visited[ref] = true
		n := nodes[ref]
		if n == nil || !n.valid {
			return
		}
		suppressed[n.doc.Change.ID] = true
		for _, parent := range n.parents {
			suppress(parent)
		}
	}
	for _, id := range ids {
		if n := roots[id]; n != nil && n.valid {
			for _, parent := range n.parents {
				suppress(parent)
			}
		}
	}
	for _, id := range ids {
		if n := roots[id]; n != nil && n.valid && !suppressed[id] {
			result.Heads = append(result.Heads, n.doc)
		}
	}
	sortDocuments(result.Heads)
	if carrier.HasErrors(result.Diagnostics) {
		result.Kind = "conflict"
	} else if len(result.Heads) > 1 {
		result.Kind = "conflict"
		result.Diagnostics = append(result.Diagnostics, diag("competing_change_heads", key, fmt.Sprintf("%d concurrent revisions require explicit selection", len(result.Heads))))
	} else if len(result.Heads) == 1 {
		result.Kind = "found"
	}
	return result
}

func sortDocuments(d []Document) {
	sort.Slice(d, func(i, j int) bool { return d[i].Change.ID < d[j].Change.ID })
}
