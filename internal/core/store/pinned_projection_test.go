package store

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func TestReadNeverRepairsPinnedDependenciesFromComputedCurrentBytes(t *testing.T) {
	s := Store{Root: t.TempDir()}
	raw, err := os.ReadFile("../carrier/testdata/valid/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := os.ReadFile("../carrier/testdata/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	direct(t, s.Root, "specs/spec-20260923-00000001.md", raw)
	direct(t, s.Root, "specs/terms.md", terms)
	initial := read(t, s)
	if initial.Coverage != "complete" || len(initial.Snapshots) != 0 {
		t.Fatal("fresh valid live carrier requires premature persistence", initial.Diagnostics)
	}
	a := initial.Documents[0]
	ref := a.Record.ID + "@" + a.Edition
	ev, err := os.ReadFile("../carrier/testdata/valid/evidence.md")
	if err != nil {
		t.Fatal(err)
	}
	ev = bytes.ReplaceAll(ev, []byte("sha256:"+strings.Repeat("0", 64)), []byte(a.Edition))
	direct(t, s.Root, "evidence/ev-20260923-00000001.md", ev)
	v := read(t, s)
	if v.Coverage != "degraded" || !hasDiagnostic(v.Diagnostics, "unresolved_evidence_target") || v.Resolve(ref+"#L1").Kind != "unresolved" || v.Projection.Resolve(ref+"#L1").Kind != "unresolved" || len(v.Projection.UsesFor(ref+"#L1")) != 0 {
		t.Fatal("read silently mixed live availability with durable evidence basis", v.Diagnostics)
	}
	b := a.Record
	b.ID = "spec-20260923-00000002"
	b.Claims = append([]carrier.Claim{}, b.Claims...)
	b.Claims[0].Text = "Cancellation resets total"
	b.Supersedes = []string{ref}
	b.SupersedeReason = "Accepted replacement with explicit predecessor"
	next, err := carrier.Encode(b, a.Body)
	if err != nil {
		t.Fatal(err)
	}
	direct(t, s.Root, "specs/"+b.ID+".md", next)
	v = read(t, s)
	states := map[string]string{}
	for _, e := range v.Projection.Entries {
		states[e.Document.Record.ID] = e.State
	}
	if v.Coverage != "degraded" || !hasDiagnostic(v.Diagnostics, "unresolved_predecessor") || states[a.Record.ID] != carrier.Active || states[b.ID] != carrier.Invalid {
		t.Fatal("unpersisted predecessor governed succession", states, v.Diagnostics)
	}
	// Saving the original exact bytes is a real repair. It does not rewrite the
	// observation target or attach the old result to the changed successor.
	p := "editions/sha256/" + strings.TrimPrefix(a.Edition, "sha256:") + ".json"
	publish(t, s, request(v, "save-exact-original", Output{p, initial.CurrentSnapshots[a.Edition]}))
	v = read(t, s)
	states = map[string]string{}
	for _, e := range v.Projection.Entries {
		states[e.Document.Record.ID] = e.State
	}
	if v.Coverage != "complete" || v.Resolve(ref+"#L1").Kind != "found" || states[a.Record.ID] != carrier.Superseded || states[b.ID] != carrier.Active || len(v.Projection.UsesFor(ref+"#L1")) != 1 {
		t.Fatal("persisting exact original did not repair declared history", states, v.Diagnostics)
	}
	newTarget := b.ID + "@" + v.Editions["specs/"+b.ID+".md"] + "#L1"
	if len(v.Projection.UsesFor(newTarget)) != 0 {
		t.Fatal("old observation transferred to changed successor")
	}
}
