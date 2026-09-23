package change

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func setup(t *testing.T) (carrier.Document, string, map[string]Basis, Change) {
	t.Helper()
	raw, err := os.ReadFile("../testdata/order/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	d := carrier.Parse(raw)
	if !d.Valid() {
		t.Fatal(d.Diagnostics)
	}
	_, snapshot, hash, err := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	if err != nil {
		t.Fatal(err)
	}
	ref := d.Record.ID + "@" + hash
	c := Change{Format: Format, ID: "chg-20260923-00000001", ChangeKey: "chg-20260923-00000001", Title: "Clarify cancellation", Intent: "Preserve amount and reject disallowed states", State: "open", CreatedAt: "2026-09-23T10:00:00Z", Patches: []SectionPatch{{Base: ref}}}
	return d, ref, map[string]Basis{ref: {Snapshot: snapshot, CurrentRef: ref}}, c
}
func code(ds []carrier.Diagnostic, s string) bool {
	for _, d := range ds {
		if d.Code == s {
			return true
		}
	}
	return false
}
func changeClaim(d carrier.Document) carrier.Claim {
	c := d.Record.Claims[0]
	c.Text = "Cancellation preserves total for each accepted domain transition."
	return c
}
func ready(t *testing.T, r PreviewResult) {
	t.Helper()
	if r.Kind != "ready" {
		t.Fatalf("preview %s: %+v", r.Kind, r.Diagnostics)
	}
}

func TestAddedModifiedRemovedRenamedAndProse(t *testing.T) {
	d, ref, bases, c := setup(t)
	modified := changeClaim(d)
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: modified.ID, Claim: &modified, Reason: "Clarify the claim scope"}}
	before, _ := json.Marshal(bases)
	preview := Preview(c, bases)
	ready(t, preview)
	if len(preview.Outputs) != 1 || !bytes.Equal(preview.Outputs[0].Body, d.Body) {
		t.Fatal("body lost")
	}
	if preview.Outputs[0].Successor.Status != "proposed" || preview.Outputs[0].Successor.OperatorConfirmed || preview.Outputs[0].Successor.ID != "" {
		t.Fatal("preview gained authority or publishable identity")
	}
	after, _ := json.Marshal(bases)
	if !bytes.Equal(before, after) {
		t.Fatal("mutated base")
	}
	if Preview(c, bases).Digest != preview.Digest {
		t.Fatal("preview is nondeterministic")
	}
	raw, ds := Materialize(preview.Outputs[0], Metadata{ID: "spec-20260923-00000002", CreatedAt: "2026-09-23T11:00:00Z"})
	if carrier.HasErrors(ds) {
		t.Fatal(ds)
	}
	next := carrier.Parse(raw)
	if !next.Valid() || !reflect.DeepEqual(next.Record.Supersedes, []string{ref}) {
		t.Fatal(next.Diagnostics)
	}
	// RENAMED rewrites only local dependency addresses; no evidence use is moved.
	c.Patches[0].Operations = []Operation{{Op: "RENAMED", ClaimID: "cancelable", NewID: "eligible", Reason: "Repair domain term locator"}}
	preview = Preview(c, bases)
	ready(t, preview)
	out := preview.Outputs[0].Successor
	if out.Claims[1].ID != "eligible" || out.Claims[2].Refs[0] != "eligible" || !contains(out.RetiredClaimIDs, "cancelable") {
		t.Fatal("rename lost identity history")
	}
	// Remove referring guard first, then its definition. The unrelated law survives.
	c.Patches[0].Operations = []Operation{{Op: "REMOVED", ClaimID: "admission", Reason: "Move this rule to another section"}, {Op: "REMOVED", ClaimID: "cancelable", Reason: "Move definition with guard"}}
	preview = Preview(c, bases)
	ready(t, preview)
	if len(preview.Outputs[0].Successor.Claims) != 1 || len(preview.Losses) != 2 {
		t.Fatal("removal loss not explicit")
	}
	added := carrier.Claim{ID: "no-refund", Kind: "definition", Text: "Cancellation is not a refund."}
	c.Patches[0].Operations = []Operation{{Op: "ADDED", Claim: &added}}
	preview = Preview(c, bases)
	ready(t, preview)
	if len(preview.Outputs[0].Successor.Claims) != 4 {
		t.Fatal("claim not added")
	}
}
func TestAddedNeverOverwritesAndIdenticalIsNoop(t *testing.T) {
	d, _, bases, c := setup(t)
	original := d.Record.Claims[0]
	c.Patches[0].Operations = []Operation{{Op: "ADDED", Claim: &original}}
	r := Preview(c, bases)
	ready(t, r)
	if len(r.Outputs) != 0 {
		t.Fatal("identical ADDED created successor")
	}
	changed := changeClaim(d)
	c.Patches[0].Operations[0].Claim = &changed
	r = Preview(c, bases)
	if r.Kind != "conflict" || !code(r.Diagnostics, "claim_already_exists") {
		t.Fatal("ADDED overwrote changed content")
	}
}
func TestScenarioAndUnknownFieldsDoNotDisappear(t *testing.T) {
	d, ref, bases, c := setup(t)
	d.Record.Claims[0].Extra = carrier.Extra{"extension": map[string]any{"risk": "refund interpretation"}}
	raw, err := carrier.Encode(d.Record, d.Body)
	if err != nil {
		t.Fatal(err)
	}
	d = carrier.Parse(raw)
	_, snapshot, hash, _ := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	newRef := d.Record.ID + "@" + hash
	delete(bases, ref)
	bases[newRef] = Basis{Snapshot: snapshot, CurrentRef: newRef}
	c.Patches[0].Base = newRef
	modified := changeClaim(d)
	modified.Extra = nil
	modified.Examples = nil
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: modified.ID, Claim: &modified, Reason: "Clarify"}}
	r := Preview(c, bases)
	ready(t, r)
	got := r.Outputs[0].Successor.Claims[0]
	if len(got.Examples) != 1 || got.Extra["extension"] == nil {
		t.Fatal("omission silently dropped scenario/extension")
	}
	modified.Examples = []carrier.Example{}
	r = Preview(c, bases)
	if !code(r.Diagnostics, "scenario_loss") {
		t.Fatal("explicit replacement silently dropped scenario")
	}
	c.Patches[0].Operations[0].RemoveExamples = []string{"paid-order"}
	c.Patches[0].Operations[0].RemoveFields = []string{"extension"}
	r = Preview(c, bases)
	ready(t, r)
	if len(r.Losses) != 2 || len(r.Outputs[0].Successor.Claims[0].Examples) != 0 {
		t.Fatal("explicit removal not reported")
	}
}

func TestPatchWireRetainsOmittedVersusEmptyExamples(t *testing.T) {
	d, _, bases, c := setup(t)
	cl := changeClaim(d)
	cl.Examples = nil
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Clarify"}}
	for _, empty := range []bool{false, true} {
		if empty {
			cl.Examples = []carrier.Example{}
		}
		raw, err := Encode(c, nil)
		if err != nil {
			t.Fatal(err)
		}
		parsed := Parse(raw)
		if carrier.HasErrors(parsed.Diagnostics) {
			t.Fatal(parsed.Diagnostics)
		}
		wireJSON, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Change
		if err := json.Unmarshal(wireJSON, &decoded); err != nil {
			t.Fatal(err)
		}
		for _, roundtrip := range []Change{parsed.Change, decoded} {
			if (roundtrip.Patches[0].Operations[0].Claim.Examples != nil) != empty {
				t.Fatalf("lost examples presence, empty=%v: %s", empty, raw)
			}
			result := Preview(roundtrip, bases)
			if empty {
				if !code(result.Diagnostics, "scenario_loss") {
					t.Fatalf("empty scenario replacement accepted: %+v", result)
				}
			} else {
				ready(t, result)
			}
		}
	}
}
func TestAuthoredBaseAndSnapshotsAreChecked(t *testing.T) {
	d, ref, bases, c := setup(t)
	cl := changeClaim(d)
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Clarify"}}
	basis := bases[ref]
	basis.CurrentRef = "spec-20260923-00000002@" + carrier.Digest([]byte("new"))
	bases[ref] = basis
	r := Preview(c, bases)
	if !code(r.Diagnostics, "stale_authored_base") {
		t.Fatal("old authored intent followed new head")
	}
	basis.CurrentRef = ref
	basis.Snapshot = append(basis.Snapshot, ' ')
	bases[ref] = basis
	r = Preview(c, bases)
	if !code(r.Diagnostics, "invalid_authored_snapshot") {
		t.Fatal("digest mismatch accepted")
	}
	basis.Contested = []string{ref, "other"}
	bases[ref] = basis
	r = Preview(c, bases)
	if !code(r.Diagnostics, "contested_authored_base") {
		t.Fatal("chose a contested winner")
	}
}
func TestRetirementBodyAndDanglingReferences(t *testing.T) {
	_, _, bases, c := setup(t)
	c.Patches[0].Operations = []Operation{{Op: "REMOVED", ClaimID: "cancelable", Reason: "Remove definition"}}
	r := Preview(c, bases)
	if !code(r.Diagnostics, "unresolved_claim") {
		t.Fatal("dangling local dependency accepted")
	}
	c.Patches[0].Operations = []Operation{{Op: "REMOVED", ClaimID: "cancelable", Reason: "Retire"}, {Op: "REMOVED", ClaimID: "admission", Reason: "Retire"}, {Op: "REMOVED", ClaimID: "total-preserved", Reason: "Retire"}}
	r = Preview(c, bases)
	if !code(r.Diagnostics, "retirement_required") {
		t.Fatal("last claim removed without retirement")
	}
	c.Patches[0].RetireReason = "Retire this synthetic capability but preserve its history"
	r = Preview(c, bases)
	ready(t, r)
	if r.Outputs[0].Successor.Retirement == nil || len(r.Outputs[0].Body) == 0 {
		t.Fatal("retirement deleted prose")
	}
	body := "Explicit replacement prose\n"
	c.Patches[0].Body = &body
	r = Preview(c, bases)
	if !code(r.Diagnostics, "unacknowledged_body_change") {
		t.Fatal("silent body replacement")
	}
	d, _, _, _ := setup(t)
	c.Patches[0].ExpectedBodyDigest = carrier.Digest(d.Body)
	c.Patches[0].BodyChangeReason = "Preserve explanation in historical edition"
	r = Preview(c, bases)
	ready(t, r)
	if !bytes.Equal(r.Outputs[0].Body, []byte(body)) || len(r.Losses) != 4 {
		t.Fatal("body loss missing from preview")
	}
}
func TestTombstonesCannotBeReusedAndBatchConflictPublishesNothing(t *testing.T) {
	d, _, bases, c := setup(t)
	cl := d.Record.Claims[0]
	c.Patches[0].Operations = []Operation{{Op: "REMOVED", ClaimID: cl.ID, Reason: "Remove"}, {Op: "ADDED", Claim: &cl}}
	r := Preview(c, bases)
	if !code(r.Diagnostics, "retired_claim_id") || len(r.Outputs) != 0 {
		t.Fatal("retired identity reused")
	}
	d2 := d.Record
	d2.ID = "spec-20260923-00000009"
	d2.Slug = "other"
	raw, _ := carrier.Encode(d2, d.Body)
	_, snapshot, hash, _ := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	ref := d2.ID + "@" + hash
	bases[ref] = Basis{Snapshot: snapshot, CurrentRef: ref}
	newClaim := carrier.Claim{ID: "extra", Kind: "definition", Text: "Explicit extra definition"}
	c.Patches = append(c.Patches, SectionPatch{Base: ref, Operations: []Operation{{Op: "ADDED", Claim: &newClaim}}})
	r = Preview(c, bases)
	if r.Kind != "conflict" || len(r.Outputs) != 0 {
		t.Fatal("partial batch advertised publishable outputs")
	}
}
func TestChangeRoundTripArchiveAndExplicitRebase(t *testing.T) {
	d, ref, bases, c := setup(t)
	cl := changeClaim(d)
	c.Patches[0].Operations = []Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Clarify"}}
	c.Tasks = []Task{{ID: "run", Text: "Execute the selected property", Done: false}}
	c.Extra = carrier.Extra{"extension": "keep this context"}
	raw, err := Encode(c, []byte("Intent prose\n"))
	if err != nil {
		t.Fatal(err)
	}
	doc := Parse(raw)
	if carrier.HasErrors(doc.Diagnostics) {
		t.Fatal(doc.Diagnostics)
	}
	_, snapshot, hash, _ := carrier.NewSnapshot(raw, carrier.InterpretationBasis{})
	changeRef := c.ID + "@" + hash
	edit := Revision{Action: "archive", ID: "chg-20260923-00000002", CreatedAt: "2026-09-23T11:00:00Z", Reason: "Keep unfinished work for reference", CurrentRef: changeRef}
	archived := Revise(changeRef, snapshot, edit, bases)
	if archived.Kind != "ready" || archived.Change.State != "archived" || !code(archived.Diagnostics, "unsynced_change") || !code(archived.Diagnostics, "unfinished_task") {
		t.Fatal(archived)
	}
	if archived.Change.Tasks[0].Done || archived.Change.Extra["extension"] != "keep this context" || string(Parse(archived.Raw).Body) != "Intent prose\n" {
		t.Fatal("archive changed work or lost unknown prose")
	}
	if Preview(archived.Change, bases).Kind != "conflict" {
		t.Fatal("archived intent applied")
	}
	edit.Action = "rebase"
	edit.Reason = "Resolve explicit new authoring basis"
	if r := Revise(changeRef, snapshot, edit, bases); !code(r.Diagnostics, "explicit_rebase_required") {
		t.Fatal("implicit rebase accepted")
	}
	edit.Patches = []SectionPatch{{Base: ref, Operations: c.Patches[0].Operations}}
	rebased := Revise(changeRef, snapshot, edit, bases)
	if rebased.Kind != "ready" || rebased.Preview == nil {
		t.Fatal(rebased)
	}
	p := Heads(c.ChangeKey, []Document{doc, Parse(archived.Raw), Parse(rebased.Raw)}, map[string][]byte{hash: snapshot})
	// Both revisions intentionally use the same ID here: duplicate ID is a conflict.
	if p.Kind != "conflict" || !code(p.Diagnostics, "duplicate_change_id") {
		t.Fatal("duplicate revision ID winner chosen")
	}
}
func TestInvalidChangeHasNoImperativeFallback(t *testing.T) {
	_, _, _, c := setup(t)
	c.Patches = nil
	c.NoSpecChangeReason = "Docs only"
	raw, _ := Encode(c, []byte("Do not parse this body as a patch.\n"))
	bad := bytes.Replace(raw, []byte("state: open"), []byte("state: open\nstate: archived"), 1)
	if !carrier.HasErrors(Parse(bad).Diagnostics) {
		t.Fatal("duplicate control key accepted")
	}
	c.Patches = []SectionPatch{{Base: "spec:order-cancel", Operations: []Operation{{Op: "OVERWRITE"}}}}
	if !carrier.HasErrors(Validate(c)) {
		t.Fatal("unknown operation/live baseline accepted")
	}
	if !strings.HasPrefix(carrier.Digest(raw), "sha256:") {
		t.Fatal("bad fixture hash")
	}
}
