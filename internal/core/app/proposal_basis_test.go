package app

import (
	"os"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/change"
)

func activeProposalBasis(t *testing.T, s Service) (carrier.Document, string) {
	t.Helper()
	terms, err := os.ReadFile("../testdata/order/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "remember", Action: "terms", Carrier: string(terms)}, "written")
	raw, err := os.ReadFile("../testdata/order/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	d := carrier.Parse(raw)
	d.Record.Status = carrier.Active
	d.Record.Origin = "operator_request"
	d.Record.OperatorConfirmed = true
	run(t, s, Request{Operation: "remember", Carrier: encode(t, d.Record, d.Body)}, "written")
	d = readView(t, s).Documents[0]
	return d, d.Record.ID + "@" + d.Edition
}
func createProposalDelta(t *testing.T, s Service, id string, d carrier.Document, base string) string {
	t.Helper()
	cl := d.Record.Claims[0]
	cl.Text += " Clarified for " + id + "."
	c := change.Change{Format: change.Format, ID: id, ChangeKey: id, Title: "Refine proposed law", Intent: "Keep accepted content and refine its proposal", State: "open", CreatedAt: s.now(), Patches: []change.SectionPatch{{Base: base, Operations: []change.Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Refine the authored proposal"}}}}}
	raw, err := change.Encode(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "change", Action: "create", Carrier: string(raw)}, "written")
	return id
}
func applyProposalDelta(t *testing.T, s Service, id, base, nextID string) carrier.Document {
	t.Helper()
	preview := run(t, s, Request{Operation: "change", Action: "preview", Ref: id}, "ready")
	run(t, s, Request{Operation: "change", Action: "apply", Ref: preview.Basis["change_ref"], RequestID: "apply-" + id, ExpectedGeneration: preview.Basis["memory_generation"], PreviewDigest: preview.Basis["preview_digest"], Metadata: map[string]change.Metadata{base: {ID: nextID}}}, "written")
	for _, d := range readView(t, s).Documents {
		if d.Record.ID == nextID {
			return d
		}
	}
	t.Fatal("new proposal not published")
	return carrier.Document{}
}
func requirePreviewCode(t *testing.T, r Result, code string) {
	t.Helper()
	p, ok := r.Data.(change.PreviewResult)
	if !ok {
		t.Fatalf("no preview: %+v", r)
	}
	for _, d := range p.Diagnostics {
		if d.Code == code {
			return
		}
	}
	t.Fatalf("missing %s: %+v", code, p)
}

func TestDeltaRefinesProposedLeafWithoutChangingAcceptedHead(t *testing.T) {
	s := service(t)
	a, ar := activeProposalBasis(t, s)
	first := createProposalDelta(t, s, "chg-20260923-00000011", a, ar)
	b := applyProposalDelta(t, s, first, ar, "spec-20260923-00000002")
	br := b.Record.ID + "@" + b.Edition
	second := createProposalDelta(t, s, "chg-20260923-00000012", b, br)
	c := applyProposalDelta(t, s, second, br, "spec-20260923-00000003")
	v := readView(t, s)
	states := map[string]string{}
	for _, e := range v.Projection.Entries {
		states[e.Document.Record.ID] = e.State
	}
	if states[a.Record.ID] != carrier.Active || states[b.Record.ID] != carrier.Superseded || states[c.Record.ID] != carrier.Proposed || c.Record.OperatorConfirmed || c.Record.Origin != "agent_proposal" {
		t.Fatal("proposal refinement altered governing authority", states)
	}
	if got := v.Projection.Resolve("spec:order-cancel"); got.Kind != "found" || got.Document.Record.ID != a.Record.ID {
		t.Fatal("proposal refinement stole accepted alias")
	}
	stale := run(t, s, Request{Operation: "change", Action: "preview", Ref: second}, "conflict")
	requirePreviewCode(t, stale, "stale_authored_base")
}

func TestCompetingProposalLeavesBlockAuthoredProposalPreview(t *testing.T) {
	s := service(t)
	a, ar := activeProposalBasis(t, s)
	first := createProposalDelta(t, s, "chg-20260923-00000021", a, ar)
	b := applyProposalDelta(t, s, first, ar, "spec-20260923-00000002")
	br := b.Record.ID + "@" + b.Edition
	other := createProposalDelta(t, s, "chg-20260923-00000022", a, ar)
	applyProposalDelta(t, s, other, ar, "spec-20260923-00000003")
	refine := createProposalDelta(t, s, "chg-20260923-00000023", b, br)
	got := run(t, s, Request{Operation: "change", Action: "preview", Ref: refine}, "conflict")
	requirePreviewCode(t, got, "contested_authored_base")
	if got := readView(t, s).Projection.Heads(a.Record.ID); len(got) != 1 || got[0] != ar {
		t.Fatal("proposal conflict became active authority")
	}
}

func TestAcceptedSuccessorMakesFormerProposalBaseStale(t *testing.T) {
	s := service(t)
	a, ar := activeProposalBasis(t, s)
	first := createProposalDelta(t, s, "chg-20260923-00000031", a, ar)
	b := applyProposalDelta(t, s, first, ar, "spec-20260923-00000002")
	br := b.Record.ID + "@" + b.Edition
	refine := createProposalDelta(t, s, "chg-20260923-00000032", b, br)
	accepted := b.Record
	accepted.ID = "spec-20260923-00000003"
	accepted.Status = carrier.Active
	accepted.Origin = "operator_request"
	accepted.OperatorConfirmed = true
	accepted.Supersedes = []string{ar, br}
	accepted.SupersedeReason = "Explicitly accept the proposal"
	run(t, s, Request{Operation: "remember", Carrier: encode(t, accepted, b.Body), ExpectedHeads: []string{ar}}, "written")
	got := run(t, s, Request{Operation: "change", Action: "preview", Ref: refine}, "conflict")
	requirePreviewCode(t, got, "stale_authored_base")
	if len(readView(t, s).Projection.ProposalHeads(b.Record.ID)) != 0 {
		t.Fatal("accepted proposal remained a live proposal leaf")
	}
}
