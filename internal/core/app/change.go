package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/change"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func changeDocuments(v store.View) []change.Document {
	docs := []change.Document{}
	paths := []string{}
	for p := range v.Files {
		if strings.HasPrefix(p, "changes/") && strings.HasSuffix(p, ".md") {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		docs = append(docs, change.Parse(v.Files[p]))
	}
	return docs
}
func changeSnapshot(raw []byte, v store.View) ([]byte, string, error) {
	basis, err := interpretation(carrier.Record{}, v)
	if err != nil {
		return nil, "", err
	}
	_, blob, hash, err := carrier.NewSnapshot(raw, basis)
	return blob, hash, err
}
func resolveChange(ref string, v store.View) (change.Document, []byte, string, error) {
	if strings.Contains(ref, "@") {
		id, hash, err := change.ParseRef(ref)
		if err != nil {
			return change.Document{}, nil, "", err
		}
		blob, ok := v.Snapshots[hash]
		if !ok {
			return change.Document{}, nil, "", fmt.Errorf("exact change snapshot unavailable")
		}
		snap, err := carrier.ReadSnapshot(blob, hash)
		if err != nil {
			return change.Document{}, nil, "", err
		}
		d := change.Parse(snap.Raw)
		if carrier.HasErrors(d.Diagnostics) || d.Change.ID != id {
			return d, nil, "", fmt.Errorf("invalid change snapshot identity")
		}
		return d, blob, ref, nil
	}
	var matches []change.Document
	for _, d := range changeDocuments(v) {
		if d.Change.ID == ref {
			matches = append(matches, d)
		}
	}
	if len(matches) != 1 {
		return change.Document{}, nil, "", fmt.Errorf("change identity absent or ambiguous")
	}
	d := matches[0]
	if carrier.HasErrors(d.Diagnostics) {
		return d, nil, "", fmt.Errorf("invalid change carrier: %v", d.Diagnostics)
	}
	blob, hash, err := changeSnapshot(d.Raw, v)
	return d, blob, d.Change.ID + "@" + hash, err
}
func currentChange(d change.Document, v store.View) (string, change.HeadResult) {
	heads := change.Heads(d.Change.ChangeKey, changeDocuments(v), v.AllSnapshots())
	if heads.Kind != "found" || len(heads.Heads) != 1 {
		return "", heads
	}
	_, hash, err := changeSnapshot(heads.Heads[0].Raw, v)
	if err != nil {
		return "", heads
	}
	return heads.Heads[0].Change.ID + "@" + hash, heads
}
func changeBases(c change.Change, v store.View) map[string]change.Basis {
	result := map[string]change.Basis{}
	for _, patch := range c.Patches {
		ref, err := carrier.ParseRef(patch.Base)
		if err != nil {
			continue
		}
		basis := change.Basis{Snapshot: v.AllSnapshots()[ref.Digest]}
		heads := v.Projection.Heads(ref.RecordID)
		// An exact proposed authored base belongs to the proposal authoring
		// branch, even while its accepted predecessor still governs the alias.
		// Once proposals are accepted, normal heads make the old base stale.
		if snapshot, err := carrier.ReadSnapshot(basis.Snapshot, ref.Digest); err == nil {
			authored := snapshot.Document()
			if authored.Valid() && authored.Record.ID == ref.RecordID && authored.Record.Status == carrier.Proposed {
				if proposals := v.Projection.ProposalHeads(ref.RecordID); len(proposals) > 0 {
					heads = proposals
				}
			}
		}
		basis.Contested = heads
		if len(heads) == 1 {
			basis.CurrentRef = heads[0]
		}
		result[patch.Base] = basis
	}
	return result
}
func appliedChange(ref string, v store.View) bool {
	for p, b := range v.Files {
		if !strings.HasPrefix(p, "changes/applied-") || !strings.HasSuffix(p, ".json") {
			continue
		}
		var log struct {
			Format string `json:"format"`
			Change string `json:"change"`
		}
		if json.Unmarshal(b, &log) == nil && log.Format == "haft.change-application/1" && log.Change == ref {
			return true
		}
	}
	return false
}

func (s Service) change(ctx context.Context, q Request, r Result, v store.View, payload []byte) Result {
	if q.Action == "list" {
		keys := map[string]bool{}
		docs := changeDocuments(v)
		for _, d := range docs {
			keys[d.Change.ChangeKey] = true
		}
		result := map[string]change.HeadResult{}
		for key := range keys {
			result[key] = change.Heads(key, docs, v.AllSnapshots())
		}
		r.Kind = "results"
		r.Data = map[string]any{"lineages": result, "documents": docs}
		return r
	}
	if q.Action == "create" {
		front, body, err := carrier.SplitFrontmatter([]byte(q.Carrier))
		if err != nil {
			return failure(r, "invalid_change", err.Error())
		}
		node, ds := carrier.ParseYAML(front)
		if carrier.HasErrors(ds) {
			r.Diagnostics = append(r.Diagnostics, ds...)
			return r
		}
		var c change.Change
		if err := node.Decode(&c); err != nil {
			return failure(r, "invalid_change", err.Error())
		}
		if c.Format == "" {
			c.Format = change.Format
		}
		if c.ID == "" {
			c.ID, err = s.newID("change")
			if err != nil {
				return unavailable(r, err)
			}
		}
		if c.CreatedAt == "" {
			c.CreatedAt = s.now()
		}
		if c.State == "" {
			c.State = "open"
		}
		if c.ChangeKey == "" {
			c.ChangeKey = c.ID
		}
		if len(c.Supersedes) > 0 || c.ChangeKey != c.ID {
			return failure(r, "revision_requires_exact_base", "Use a revision action to change an existing change")
		}
		c.WriteReceipt = receipt(q, payload)
		ds = change.Validate(c)
		r.Diagnostics = append(r.Diagnostics, ds...)
		if carrier.HasErrors(ds) {
			return r
		}
		for _, old := range changeDocuments(v) {
			if old.Change.ID == c.ID {
				return conflict(r, "duplicate_change_id", c.ID)
			}
		}
		raw, err := change.Encode(c, body)
		if err != nil {
			return failure(r, "encode", err.Error())
		}
		outputs := []store.Output{}
		snaps, err := snapshotInputs(v, q.Snapshots, &outputs)
		if err != nil {
			return failure(r, "invalid_snapshot_input", err.Error())
		}
		for _, p := range c.Patches {
			ref, _ := carrier.ParseRef(p.Base)
			blob := snaps[ref.Digest]
			snap, err := carrier.ReadSnapshot(blob, ref.Digest)
			if err != nil || snap.Document().Record.ID != ref.RecordID {
				return conflict(r, "authored_base_unavailable", p.Base)
			}
			outputs = appendSnapshot(outputs, ref.Digest, blob)
		}
		return s.publishChange(ctx, q, r, v, payload, raw, outputs)
	}
	d, blob, ref, err := resolveChange(q.Ref, v)
	if err != nil {
		return conflict(r, "change_unresolved", err.Error())
	}
	r.Basis["change_ref"] = ref
	if q.Action == "show" {
		r.Kind = "found"
		r.Data = map[string]any{"document": d, "exact_ref": ref, "applied": appliedChange(ref, v)}
		return r
	}
	current, heads := currentChange(d, v)
	if current != ref {
		r.Diagnostics = append(r.Diagnostics, heads.Diagnostics...)
		return conflict(r, "concurrent_change_revision", "Requested change is not its sole current revision")
	}
	if q.Action == "preview" || q.Action == "apply" || q.Action == "sync" {
		preview := change.Preview(d.Change, changeBases(d.Change, v))
		r.Data = preview
		r.Kind = preview.Kind
		r.Basis["preview_digest"] = preview.Digest
		if q.Action == "preview" || preview.Kind != "ready" {
			return r
		}
		if q.RequestID == "" || q.ExpectedGeneration == "" || q.PreviewDigest == "" {
			return failure(r, "apply_preconditions_required", "Apply requires request_id, expected_generation and preview_digest from preview")
		}
		if q.PreviewDigest != preview.Digest {
			return conflict(r, "preview_changed", "Re-read and assess the current preview")
		}
		if appliedChange(ref, v) {
			r.Kind = "already_applied"
			return r
		}
		outputs := appendSnapshot(nil, strings.Split(ref, "@")[1], blob)
		newRefs := []string{}
		snaps := v.AllSnapshots()
		for _, out := range preview.Outputs {
			m := q.Metadata[out.Base]
			if m.ID == "" {
				m.ID, err = s.newID("spec")
				if err != nil {
					return unavailable(r, err)
				}
			}
			if m.CreatedAt == "" {
				m.CreatedAt = s.now()
			}
			m.Receipt = receipt(q, payload)
			raw, ds := change.Materialize(out, m)
			r.Diagnostics = append(r.Diagnostics, ds...)
			if carrier.HasErrors(ds) {
				r.Kind = "invalid"
				return r
			}
			doc := carrier.Parse(raw)
			base, _ := carrier.ParseRef(out.Base)
			ds = carrier.ValidateSuccessor(doc.Record, v.Projection, snaps, v.Projection.Heads(base.RecordID))
			r.Diagnostics = append(r.Diagnostics, ds...)
			if carrier.HasErrors(ds) {
				r.Kind = "conflict"
				return r
			}
			for _, previous := range doc.Record.Supersedes {
				p, _ := carrier.ParseRef(previous)
				outputs = appendSnapshot(outputs, p.Digest, snaps[p.Digest])
			}
			var newRef string
			outputs, newRef, err = addRecord(outputs, raw, doc.Record, v)
			if err != nil {
				return failure(r, "interpretation_basis", err.Error())
			}
			newRefs = append(newRefs, newRef)
		}
		log, err := json.Marshal(struct {
			Format  string                `json:"format"`
			Change  string                `json:"change"`
			Preview string                `json:"preview_digest"`
			Outputs []string              `json:"outputs"`
			Receipt *carrier.WriteReceipt `json:"write_receipt"`
		}{"haft.change-application/1", ref, preview.Digest, newRefs, receipt(q, payload)})
		if err != nil {
			return failure(r, "encode", err.Error())
		}
		log = append(log, '\n')
		outputs = append(outputs, store.Output{Path: "changes/applied-" + strings.TrimPrefix(carrier.Digest(log), "sha256:") + ".json", Bytes: log})
		return s.publish(ctx, q, r, v, payload, outputs, []string{ref})
	}
	if q.Action != "archive" && q.Action != "reopen" && q.Action != "rebase" && q.Action != "update" {
		return failure(r, "unsupported_action", "Unsupported change action")
	}
	if q.Revision == nil {
		return failure(r, "revision_required", "Change revision requires reason and explicit edits")
	}
	edit := *q.Revision
	edit.Action = q.Action
	edit.CurrentRef = current
	edit.Receipt = receipt(q, payload)
	edit.Synced = appliedChange(ref, v)
	if edit.ID == "" {
		edit.ID, err = s.newID("change")
		if err != nil {
			return unavailable(r, err)
		}
	}
	if edit.CreatedAt == "" {
		edit.CreatedAt = s.now()
	}
	basisChange := d.Change
	if edit.Patches != nil {
		basisChange.Patches = edit.Patches
	}
	revision := change.Revise(ref, blob, edit, changeBases(basisChange, v))
	r.Kind = revision.Kind
	r.Data = revision
	r.Diagnostics = append(r.Diagnostics, revision.Diagnostics...)
	if revision.Kind != "ready" {
		return r
	}
	outputs := appendSnapshot(nil, strings.Split(ref, "@")[1], blob)
	for _, patch := range revision.Change.Patches {
		p, _ := carrier.ParseRef(patch.Base)
		b := v.AllSnapshots()[p.Digest]
		if len(b) == 0 {
			return conflict(r, "authored_base_unavailable", patch.Base)
		}
		outputs = appendSnapshot(outputs, p.Digest, b)
	}
	return s.publishChange(ctx, q, r, v, payload, revision.Raw, outputs)
}
func (s Service) publishChange(ctx context.Context, q Request, r Result, v store.View, payload, raw []byte, outputs []store.Output) Result {
	d := change.Parse(raw)
	if carrier.HasErrors(d.Diagnostics) {
		r.Diagnostics = append(r.Diagnostics, d.Diagnostics...)
		r.Kind = "invalid"
		return r
	}
	blob, hash, err := changeSnapshot(raw, v)
	if err != nil {
		return failure(r, "change_snapshot", err.Error())
	}
	outputs = appendSnapshot(outputs, hash, blob)
	outputs = append(outputs, store.Output{Path: "changes/" + d.Change.ID + ".md", Bytes: raw})
	return s.publish(ctx, q, r, v, payload, outputs, d.Change.Supersedes)
}
