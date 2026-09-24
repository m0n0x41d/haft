package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/change"
	"github.com/m0n0x41d/haft/internal/core/store"
)

// r3Batch uses actual application writes and captured editions throughout. The
// dependency intentionally points from the first output to a new second-output
// claim, so checking each output against the old view cannot admit this batch.
func r3Batch(t *testing.T, s Service, reverse, mutual bool) Request {
	t.Helper()
	for i, slug := range []string{"alpha", "beta"} {
		r := carrier.Record{ID: []string{"spec-20260924-00000301", "spec-20260924-00000302"}[i], Kind: "spec", Title: slug, About: "domain:Batch." + slug, Slug: slug, ReceivingUse: "Publish and recover this jointly revised fixture.", Claims: []carrier.Claim{{ID: "base", Kind: "definition", Text: "Existing " + slug + " definition."}}}
		run(t, s, Request{Operation: "remember", RequestID: "seed-" + slug, Carrier: encode(t, r, []byte("Preserve "+slug+" prose.\n"))}, "written")
	}
	v := readView(t, s)
	a := v.Projection.Resolve("spec:alpha").Document
	b := v.Projection.Resolve("spec:beta").Document
	if a == nil || b == nil {
		t.Fatal("fixture specs did not resolve")
	}
	alpha := a.Record.Claims[0]
	alpha.Refs = []string{"spec:beta#new-rule"}
	beta := carrier.Claim{ID: "new-rule", Kind: "definition", Text: "New beta definition supplied by this same change."}
	if mutual {
		beta.Refs = []string{"spec:alpha#base"}
	}
	ar, br := a.Record.ID+"@"+a.Edition, b.Record.ID+"@"+b.Edition
	patches := []change.SectionPatch{
		{Base: ar, Operations: []change.Operation{{Op: "MODIFIED", ClaimID: alpha.ID, Claim: &alpha, Reason: "Use the joint definition"}}},
		{Base: br, Operations: []change.Operation{{Op: "ADDED", Claim: &beta, Reason: "Supply the new definition"}}},
	}
	if reverse {
		patches[0], patches[1] = patches[1], patches[0]
	}
	c := change.Change{Format: change.Format, ID: "chg-20260924-00000301", ChangeKey: "chg-20260924-00000301", Title: "Joint definitions", Intent: "Publish both definitions together", State: "open", CreatedAt: s.now(), Patches: patches}
	raw, err := change.Encode(c, []byte("This is one authored batch.\n"))
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "change", Action: "create", RequestID: "create-batch", Carrier: string(raw)}, "written")
	p := run(t, s, Request{Operation: "change", Action: "preview", Ref: c.ID}, "ready")
	return Request{Format: Format, Operation: "change", Action: "apply", Ref: p.Basis["change_ref"], RequestID: "apply-batch", ExpectedGeneration: p.Basis["memory_generation"], PreviewDigest: p.Basis["preview_digest"], Metadata: map[string]change.Metadata{ar: {ID: "spec-20260924-00000303"}, br: {ID: "spec-20260924-00000304"}}}
}

func r3RequireBatch(t *testing.T, s Service) {
	t.Helper()
	v := readView(t, s)
	if len(v.Documents) != 4 || v.Coverage != "complete" {
		t.Fatalf("incomplete joint publication: documents=%d coverage=%s diagnostics=%+v", len(v.Documents), v.Coverage, v.Diagnostics)
	}
	for _, slug := range []string{"alpha", "beta"} {
		r := v.Projection.Resolve("spec:" + slug)
		if r.Kind != "found" || r.Document == nil || r.Document.Record.Status != carrier.Proposed || r.Document.Record.OperatorConfirmed || r.Document.Record.Origin != "agent_proposal" {
			t.Fatalf("joint output missing or authority inferred for %s: %+v", slug, r)
		}
		if ds := v.Projection.ValidateReferences(r.Document.Record); carrier.HasErrors(ds) {
			t.Fatalf("published dependency invalid: %+v", ds)
		}
		if string(r.Document.Body) != "Preserve "+slug+" prose.\n" {
			t.Fatal("authored body changed")
		}
	}
	a := v.Projection.Resolve("spec:alpha#base")
	b := v.Projection.Resolve("spec:beta#new-rule")
	if a.Claim == nil || b.Claim == nil || !reflect.DeepEqual(a.Claim.Refs, []string{"spec:beta#new-rule"}) {
		t.Fatalf("joint claim dependency not preserved: alpha=%+v beta=%+v", a, b)
	}
}

func r3Journals(t *testing.T, s Service) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(s.Root, ".haft", "transactions"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestR3BatchReferencesNewClaimInSameChange(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, mutual := range []bool{false, true} {
			name := "forward"
			if reverse {
				name = "reverse"
			}
			if mutual {
				name += "-mutual"
			}
			t.Run(name, func(t *testing.T) {
				s := service(t)
				q := r3Batch(t, s, reverse, mutual)
				before := readView(t, s)
				run(t, s, q, "written")
				r3RequireBatch(t, s)
				after := readView(t, s)
				for path, raw := range before.Files {
					if !bytes.Equal(after.Files[path], raw) {
						t.Fatalf("predecessor/history overwritten: %s", path)
					}
				}
				run(t, s, q, "replayed")
				if replay := readView(t, s); replay.Generation != after.Generation || !reflect.DeepEqual(replay.Files, after.Files) {
					t.Fatal("same-request replay changed the joint publication")
				}
			})
		}
	}
}

func TestR3InvalidBatchPublishesNothing(t *testing.T) {
	for _, invalid := range []string{"missing-reference", "quadrant", "duplicate-output-id", "self-succession", "unconfirmed-active"} {
		t.Run(invalid, func(t *testing.T) {
			s := service(t)
			q := r3Batch(t, s, false, false)
			if invalid == "duplicate-output-id" || invalid == "self-succession" || invalid == "unconfirmed-active" {
				for base, m := range q.Metadata {
					switch invalid {
					case "duplicate-output-id":
						m.ID = "spec-20260924-00000303"
					case "self-succession":
						m.Supersedes = []string{base, m.ID + "@" + strings.Split(base, "@")[1]}
					case "unconfirmed-active":
						m.Status = carrier.Active
					}
					q.Metadata[base] = m
				}
			} else {
				v := readView(t, s)
				d, _, _, err := resolveChange(q.Ref, v)
				if err != nil {
					t.Fatal(err)
				}
				patches := d.Change.Patches
				if invalid == "missing-reference" {
					patches[0].Operations[0].Claim.Refs = []string{"spec:beta#absent"}
				} else {
					patches[1].Operations[0].Claim.Kind = "prescription"
				}
				run(t, s, Request{Operation: "change", Action: "update", Ref: q.Ref, RequestID: "revise-invalid-batch", Revision: &change.Revision{ID: "chg-20260924-00000302", Reason: "Exercise invalid joint relation", Patches: patches}}, "written")
				p := run(t, s, Request{Operation: "change", Action: "preview", Ref: "chg-20260924-00000302"}, "ready")
				q.Ref, q.PreviewDigest, q.ExpectedGeneration = p.Basis["change_ref"], p.Basis["preview_digest"], p.Basis["memory_generation"]
			}
			before := readView(t, s)
			journals := r3Journals(t, s)
			r := s.Execute(context.Background(), q)
			if !r.Failed() {
				t.Fatalf("invalid batch admitted: %+v", r)
			}
			code := map[string]string{"missing-reference": "unresolved_claim", "quadrant": "quadrant_dependency", "duplicate-output-id": "duplicate_id", "self-succession": "succession_cycle", "unconfirmed-active": "operator_confirmation_required"}[invalid]
			found := false
			for _, d := range r.Diagnostics {
				found = found || d.Code == code
			}
			if !found {
				t.Fatalf("missing specific batch rejection %s: %+v", code, r)
			}
			after := readView(t, s)
			if after.Generation != before.Generation || !reflect.DeepEqual(after.Files, before.Files) || len(after.Documents) != 2 || !reflect.DeepEqual(r3Journals(t, s), journals) {
				t.Fatal("invalid batch published carriers, snapshots or journal")
			}
		})
	}
}

func TestR3BatchGenerationAndInterruptedPublication(t *testing.T) {
	t.Run("generation", func(t *testing.T) {
		s := service(t)
		q := r3Batch(t, s, false, false)
		run(t, s, Request{Operation: "remember", RequestID: "concurrent-note", Carrier: "---\nkind: note\ntitle: Concurrent write\nabout: domain:Batch\n---\nChanges publication generation.\n"}, "written")
		before := readView(t, s)
		r := run(t, s, q, "conflict")
		found := false
		for _, d := range r.Diagnostics {
			found = found || d.Code == "concurrent_write"
		}
		if !found || !reflect.DeepEqual(readView(t, s).Files, before.Files) {
			t.Fatalf("stale generation was not rejected without writes: %+v", r)
		}
	})
	t.Run("write-between-capture-and-publish", func(t *testing.T) {
		s := service(t)
		q := r3Batch(t, s, false, false)
		for base, m := range q.Metadata {
			if m.ID == "spec-20260924-00000303" {
				m.ID = ""
				q.Metadata[base] = m
			}
		}
		other := Service{Root: s.Root, Now: s.Now}
		var afterForeign store.View
		s.NewID = func(kind string) (string, error) {
			if kind != "spec" {
				t.Fatalf("unexpected allocation: %s", kind)
			}
			run(t, other, Request{Operation: "remember", RequestID: "between-read-and-publish", Carrier: "---\nkind: note\ntitle: Concurrent instance\nabout: domain:Batch\n---\nWritten after capture and before joint publication.\n"}, "written")
			afterForeign = readView(t, other)
			return "spec-20260924-00000303", nil
		}
		run(t, s, q, "concurrent_write")
		if afterForeign.Generation == "" || !reflect.DeepEqual(readView(t, other).Files, afterForeign.Files) {
			t.Fatal("joint publication overrode a writer that moved generation after capture")
		}
	})
	t.Run("interrupted", func(t *testing.T) {
		s := service(t)
		q := r3Batch(t, s, false, false)
		before := readView(t, s)
		s.Store = &store.Store{Root: s.Root, Fault: func(p store.Point) error {
			if p.Stage == "published" && strings.HasPrefix(p.Path, "specs/") {
				return errors.New("interrupt joint carrier publication")
			}
			return nil
		}}
		run(t, s, q, "interrupted")
		s.Store = nil
		pending := readView(t, s)
		if len(pending.Documents) != 2 || pending.Generation != before.Generation || pending.Projection.Resolve("spec:beta#new-rule").Kind == "found" {
			t.Fatal("cooperative read exposed a partial batch")
		}
		run(t, s, q, "replayed")
		r3RequireBatch(t, s)
	})
}
