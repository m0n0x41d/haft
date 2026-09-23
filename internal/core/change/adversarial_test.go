package change

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func TestScenarioExtensionsPreservedByIDAndRemovedExplicitly(t *testing.T) {
	d, _, _, c := setup(t)
	d.Record.Claims[0].Examples[0].Extra = carrier.Extra{
		"future_scenario_requirement": "Review refund consequences",
		"fixture_basis":               map[string]any{"region": "EU"},
	}
	raw, err := carrier.Encode(d.Record, d.Body)
	if err != nil {
		t.Fatal(err)
	}
	_, snapshot, hash, err := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	ref := d.Record.ID + "@" + hash
	bases := map[string]Basis{ref: {Snapshot: snapshot, CurrentRef: ref}}
	c.Patches[0].Base = ref
	cl := changeClaim(d)
	ex := cl.Examples[0]
	ex.Then = "The agreed total remains unchanged."
	ex.Extra = carrier.Extra{"fixture_basis": "explicit replacement basis"}
	cl.Examples = []carrier.Example{ex}
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Clarify scenario outcome"}}
	r := Preview(c, bases)
	ready(t, r)
	got := r.Outputs[0].Successor.Claims[0].Examples[0]
	if got.Extra["future_scenario_requirement"] != "Review refund consequences" || got.Extra["fixture_basis"] != "explicit replacement basis" || got.Then != ex.Then {
		t.Fatalf("scenario extension preservation failed: %+v", got)
	}
	if _, exists := cl.Examples[0].Extra["future_scenario_requirement"]; exists {
		t.Fatal("preview mutated authored replacement")
	}
	if !bytes.Equal(bases[ref].Snapshot, snapshot) {
		t.Fatal("preview changed captured base")
	}
	c.Patches[0].Operations[0].RemoveExamples = []string{ex.ID}
	r = Preview(c, bases)
	ready(t, r)
	if len(r.Outputs[0].Successor.Claims[0].Examples) != 0 || len(r.Losses) != 1 {
		t.Fatal("explicit scenario removal not represented")
	}
	lost, ok := r.Losses[0].Before.(carrier.Example)
	if !ok || !reflect.DeepEqual(lost.Extra, d.Record.Claims[0].Examples[0].Extra) {
		t.Fatal("scenario loss omitted original extensions")
	}
}

func TestUpdateCannotRetargetBaseSetButExplicitRebaseValidatesIt(t *testing.T) {
	d, base, bases, c := setup(t)
	cl := changeClaim(d)
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Clarify"}}
	raw, err := Encode(c, []byte("Keep authored prose.\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, snapshot, hash, err := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	ref := c.ID + "@" + hash
	edit := Revision{Action: "update", ID: "chg-20260923-00000002", CreatedAt: "2026-09-23T11:00:00Z", Reason: "Retarget authoring", CurrentRef: ref}
	unknown := "spec-20260923-00000009@" + carrier.Digest([]byte("unavailable"))
	edit.Patches = []SectionPatch{{Base: unknown, Operations: c.Patches[0].Operations}}
	got := Revise(ref, snapshot, edit, nil)
	if got.Kind != "conflict" || !code(got.Diagnostics, "explicit_rebase_required") || len(got.Raw) != 0 {
		t.Fatalf("update silently rebased: %+v", got)
	}
	edit.Action = "rebase"
	got = Revise(ref, snapshot, edit, nil)
	if got.Kind != "conflict" || !code(got.Diagnostics, "missing_authored_base") || len(got.Raw) != 0 {
		t.Fatal("rebase did not check unavailable base")
	}
	// A same-base authoring update remains allowed, preserving exact history.
	edit.Action = "update"
	edit.Patches = []SectionPatch{{Base: base, Operations: c.Patches[0].Operations}}
	got = Revise(ref, snapshot, edit, bases)
	if got.Kind != "ready" || got.Change.Patches[0].Base != base || !reflect.DeepEqual(got.Change.Supersedes, []string{ref}) {
		t.Fatalf("same-base update rejected: %+v", got)
	}
	// Rebase can move to a real, explicitly supplied current edition only after
	// producing a successful preview from that edition's own bytes.
	d.Record.ID = "spec-20260923-00000009"
	newRaw, err := carrier.Encode(d.Record, d.Body)
	if err != nil {
		t.Fatal(err)
	}
	_, newSnapshot, newHash, err := carrier.NewSnapshot(newRaw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	newRef := d.Record.ID + "@" + newHash
	edit.Action = "rebase"
	edit.Patches = []SectionPatch{{Base: newRef, Operations: c.Patches[0].Operations}}
	got = Revise(ref, snapshot, edit, map[string]Basis{newRef: {Snapshot: newSnapshot, CurrentRef: newRef}})
	if got.Kind != "ready" || got.Preview == nil || got.Preview.Kind != "ready" || got.Preview.Outputs[0].Base != newRef {
		t.Fatalf("explicit valid rebase failed: %+v", got)
	}
	if string(Parse(got.Raw).Body) != "Keep authored prose.\n" {
		t.Fatal("rebase altered explanatory prose")
	}
}

func historyChange(t *testing.T, id, key string, predecessors []string) (Document, []byte, string) {
	t.Helper()
	c := Change{Format: Format, ID: id, ChangeKey: key, Title: "Change history fixture", Intent: "Preserve exact change history", State: "open", CreatedAt: "2026-09-23T10:00:00Z", NoSpecChangeReason: "Historical docs-only fixture", Supersedes: predecessors}
	if len(predecessors) > 0 {
		c.SupersedeReason = "Explicit historical revision"
	}
	raw, err := Encode(c, []byte("History prose.\n"))
	if err != nil {
		t.Fatal(err)
	}
	d := Parse(raw)
	if carrier.HasErrors(d.Diagnostics) {
		t.Fatal(d.Diagnostics)
	}
	_, snapshot, hash, err := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	return d, snapshot, id + "@" + hash
}
func storeChangeSnapshot(store map[string][]byte, ref string, snapshot []byte) {
	_, hash, _ := ParseRef(ref)
	store[hash] = snapshot
}

func TestHeadsTraverseSnapshotOnlyAncestors(t *testing.T) {
	key := "chg-20260923-00000001"
	a, as, ar := historyChange(t, key, key, nil)
	_, bs, br := historyChange(t, "chg-20260923-00000002", key, []string{ar})
	c, _, _ := historyChange(t, "chg-20260923-00000003", key, []string{br})
	snaps := map[string][]byte{}
	storeChangeSnapshot(snaps, br, bs)
	got := Heads(key, []Document{c}, snaps)
	if got.Kind != "conflict" || !code(got.Diagnostics, "unresolved_predecessor") || len(got.Heads) != 0 {
		t.Fatalf("missing deep predecessor became a valid head: %+v", got)
	}
	storeChangeSnapshot(snaps, ar, as)
	for _, docs := range [][]Document{{c}, {a, c}, {c, a}} {
		got = Heads(key, docs, snaps)
		if got.Kind != "found" || len(got.Heads) != 1 || got.Heads[0].Change.ID != c.Change.ID {
			t.Fatalf("snapshot-only history did not suppress live ancestor: %+v", got)
		}
	}
	// Another live leaf remains a competing head, even with a snapshot-only
	// intermediate ancestor on the first branch.
	d, _, _ := historyChange(t, "chg-20260923-00000004", key, []string{ar})
	got = Heads(key, []Document{a, c, d}, snaps)
	if got.Kind != "conflict" || !code(got.Diagnostics, "competing_change_heads") || len(got.Heads) != 2 {
		t.Fatalf("competing branch hidden: %+v", got)
	}
}

func TestHeadsRejectMalformedWrongIdentityAndWrongKeyAncestor(t *testing.T) {
	key := "chg-20260923-00000001"
	_, original, ar := historyChange(t, key, key, nil)
	for _, mode := range []string{"malformed", "identity", "key"} {
		t.Run(mode, func(t *testing.T) {
			snapshot := original
			target := ar
			switch mode {
			case "malformed":
				_, snapshotHashBytes, hash, err := carrier.NewSnapshot([]byte("not a change carrier"), carrier.InterpretationBasis{})
				if err != nil {
					t.Fatal(err)
				}
				snapshot = snapshotHashBytes
				target = key + "@" + hash
			case "identity":
				other, blob, otherRef := historyChange(t, "chg-20260923-00000008", "chg-20260923-00000008", nil)
				_ = other
				_, hash, _ := ParseRef(otherRef)
				snapshot = blob
				target = key + "@" + hash
			case "key":
				_, snapshot, target = historyChange(t, "chg-20260923-00000008", "chg-20260923-00000008", nil)
			}
			_, bs, br := historyChange(t, "chg-20260923-00000002", key, []string{target})
			leaf, _, _ := historyChange(t, "chg-20260923-00000003", key, []string{br})
			snaps := map[string][]byte{}
			storeChangeSnapshot(snaps, target, snapshot)
			storeChangeSnapshot(snaps, br, bs)
			got := Heads(key, []Document{leaf}, snaps)
			if got.Kind != "conflict" || !code(got.Diagnostics, "change_lineage_mismatch") || len(got.Heads) != 0 {
				t.Fatalf("invalid exact ancestor became a head: %+v", got)
			}
		})
	}
}

func TestHeadsDetectIDCycleThroughDifferentSnapshotEditions(t *testing.T) {
	key := "chg-20260923-00000001"
	_, as, ar := historyChange(t, key, key, nil)
	_, bs, br := historyChange(t, "chg-20260923-00000002", key, []string{ar})
	// Historical bytes are finite and hash-valid, but reusing A's ID after B
	// creates A -> B -> A. A fresh leaf must not conceal that invalid history.
	_, a2s, a2r := historyChange(t, key, key, []string{br})
	leaf, _, _ := historyChange(t, "chg-20260923-00000003", key, []string{a2r})
	snaps := map[string][]byte{}
	storeChangeSnapshot(snaps, ar, as)
	storeChangeSnapshot(snaps, br, bs)
	storeChangeSnapshot(snaps, a2r, a2s)
	got := Heads(key, []Document{leaf}, snaps)
	if got.Kind != "conflict" || !code(got.Diagnostics, "change_succession_cycle") || len(got.Heads) != 0 {
		t.Fatalf("snapshot-only cycle became a valid head: %+v", got)
	}
}
