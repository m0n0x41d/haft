package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func TestRecallResolvesRelationsWithoutRetargetingHistory(t *testing.T) {
	s := service(t)
	d, old := seed(t, s)
	claim := "#total-preserved"
	alias := "spec:order-cancel" + claim
	note := carrier.Record{Kind: "note", Title: "Live navigation", About: d.Record.About, Links: []carrier.Link{{Kind: "relates_to", Target: alias}, {Kind: "relates_to", Target: old + claim}}}
	run(t, s, Request{Operation: "remember", RequestID: "linked-note", Carrier: encode(t, note, nil)}, "written")
	evidence := carrier.Record{Kind: "evidence", Title: "Old observation", About: d.Record.About, Claim: "Historical only", ObservedAt: s.now(), Method: "fixture", Source: "fixture.json", Basis: &carrier.EvidenceBasis{Kind: "code", Ref: "fixture.json"}, Uses: []carrier.EvidenceUse{{ID: "old", Target: old + claim, Polarity: "supports", Scope: "Previous exact edition only"}}}
	run(t, s, Request{Operation: "remember", RequestID: "old-evidence", Carrier: encode(t, evidence, nil)}, "written")
	read := func(ref string) map[string]any {
		return run(t, s, Request{Operation: "recall", Ref: ref}, "found").Data.(map[string]any)
	}
	aliasRead, exactRead := read(alias), read(old+claim)
	if len(aliasRead["links"].([]store.Edge)) == 0 || !reflect.DeepEqual(aliasRead["links"], exactRead["links"]) || !reflect.DeepEqual(aliasRead["backlinks"], exactRead["backlinks"]) {
		t.Errorf("alias and exact endpoint relations differ: alias links=%v backlinks=%v exact links=%v backlinks=%v", aliasRead["links"], aliasRead["backlinks"], exactRead["links"], exactRead["backlinks"])
	}
	if len(exactRead["backlinks"].([]store.Edge)) != 3 {
		t.Errorf("missing equivalent live/pinned backlinks: %v", exactRead["backlinks"])
	}
	v := readView(t, s)
	successor := d.Record
	successor.ID = ""
	successor.Claims[0].Text += " Updated wording; no evidence transfer."
	successor.Supersedes = []string{old}
	successor.SupersedeReason = "Clarify fixture claim"
	run(t, s, Request{Operation: "remember", RequestID: "successor", Carrier: encode(t, successor, d.Body), ExpectedGeneration: v.Generation, ExpectedHeads: []string{old}}, "written")
	next := read(alias)
	if next["exact_ref"] == old+claim {
		t.Fatal("alias did not move to successor")
	}
	if !reflect.DeepEqual(next["links"], read(next["exact_ref"].(string))["links"]) {
		t.Fatal("successor alias/exact links differ")
	}
	backlinks := next["backlinks"].([]store.Edge)
	if len(backlinks) != 1 || backlinks[0].To != alias || backlinks[0].Kind == "evidence" {
		t.Fatalf("pinned old relations moved to successor or authored live target changed: %v", backlinks)
	}
	oldRead := read(old + claim)
	for _, edge := range oldRead["backlinks"].([]store.Edge) {
		if edge.To == alias {
			t.Fatal("live target falsely assigned to historical edition")
		}
	}
	if len(oldRead["backlinks"].([]store.Edge)) != 2 {
		t.Fatal("historical pinned backlinks lost", oldRead)
	}
	// Removing an old canonical carrier leaves its committed edition readable;
	// outgoing history must derive from that exact edition, never the new head.
	v = readView(t, s)
	for i, doc := range v.Documents {
		if doc.Record.ID == d.Record.ID {
			if err := os.Remove(filepath.Join(s.Root, ".haft", v.DocumentPaths[i])); err != nil {
				t.Fatal(err)
			}
		}
	}
	oldRead = read(old + claim)
	if len(oldRead["links"].([]store.Edge)) == 0 || oldRead["links"].([]store.Edge)[0].From != old+claim {
		t.Fatal("historical outgoing endpoint changed", oldRead)
	}
	if len(oldRead["links"].([]store.Edge)) != len(exactRead["links"].([]store.Edge)) {
		t.Fatal("snapshot-only historical links lost", oldRead)
	}
	if _, err := s.memory().Read(context.Background()); err != nil {
		t.Fatal(err)
	}
}
