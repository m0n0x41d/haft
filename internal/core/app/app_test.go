package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/change"
	"github.com/m0n0x41d/haft/internal/core/code"
	"github.com/m0n0x41d/haft/internal/core/source"
	"github.com/m0n0x41d/haft/internal/core/store"
)

func service(t *testing.T) Service {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"go.mod", "order.go", "policy.go", "order_test.go"} {
		b, err := os.ReadFile("../testdata/order/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return Service{Root: root, Now: func() time.Time { return time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC) }}
}
func run(t *testing.T, s Service, q Request, want string) Result {
	t.Helper()
	q.Format = Format
	r := s.Execute(context.Background(), q)
	if r.Kind != want {
		b, _ := json.MarshalIndent(r, "", "  ")
		t.Fatalf("%s/%s want %s got %s: %s", q.Operation, q.Action, want, r.Kind, b)
	}
	return r
}
func seed(t *testing.T, s Service) (carrier.Document, string) {
	t.Helper()
	terms, err := os.ReadFile("../testdata/order/terms.md")
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "remember", Action: "terms", Carrier: string(terms), RequestID: "terms"}, "written")
	raw, err := os.ReadFile("../testdata/order/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "remember", Carrier: string(raw), RequestID: "spec"}, "written")
	v, err := s.memory().Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d := v.Documents[0]
	return d, d.Record.ID + "@" + d.Edition
}
func encode(t *testing.T, r carrier.Record, body []byte) string {
	t.Helper()
	b, e := carrier.Encode(r, body)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func readView(t *testing.T, s Service) store.View {
	t.Helper()
	v, err := s.memory().Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRememberGeneratedMetadataAndLostReplyReplay(t *testing.T) {
	s := service(t)
	s.Store = &store.Store{Root: s.Root, Fault: func(p store.Point) error {
		if p.Stage == "committed" {
			return errors.New("reply lost")
		}
		return nil
	}}
	q := Request{Operation: "remember", RequestID: "note-once", Carrier: "---\nkind: note\ntitle: Current fixture observation\nabout: domain:Billing.Order\n---\nCancellation fixture prepared.\n"}
	first := run(t, s, q, "written")
	if len(first.Diagnostics) == 0 {
		t.Fatal("lost reply not reported")
	}
	run(t, s, q, "replayed")
	v := readView(t, s)
	if len(v.Documents) != 1 || v.Documents[0].Record.OperatorConfirmed || v.Documents[0].Record.Status != "proposed" {
		t.Fatal("replay duplicate or authority inferred")
	}
	q.Carrier += "different\n"
	run(t, s, q, "request_conflict")
	q.RequestID = "other"
	q.Carrier = string(v.Documents[0].Raw)
	run(t, s, q, "invalid")
}

func TestEvidencePinsExpectedLiveEditionAndOldTermBasis(t *testing.T) {
	s := service(t)
	d, old := seed(t, s)
	evidence := carrier.Record{Kind: "evidence", Status: "active", Title: "Property observation", About: d.Record.About, Claim: "Recorded property result", ObservedAt: s.now(), Method: "go test", Source: "captured-run.json", Basis: &carrier.EvidenceBasis{Kind: "code", Ref: "captured-run.json"}, Uses: []carrier.EvidenceUse{{ID: "amount", Target: d.Record.ID + "#total-preserved", Polarity: "supports", Scope: "Synthetic domain fixture only"}}}
	q := Request{Operation: "remember", RequestID: "evidence", Carrier: encode(t, evidence, nil)}
	run(t, s, q, "conflict")
	q.ExpectedTargets = map[string]string{evidence.Uses[0].Target: d.Edition}
	run(t, s, q, "written")
	v := readView(t, s)
	if len(v.Projection.UsesFor(old+"#total-preserved")) != 1 {
		t.Fatal("exact use missing")
	}
	termsPath := filepath.Join(s.Root, ".haft", "specs", "terms.md")
	raw, err := os.ReadFile(termsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(termsPath, append(raw, []byte("\nChanged interpretation note.\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	v = readView(t, s)
	live := v.Projection.Resolve(d.Record.ID)
	if live.Kind != "found" || live.Document.Edition == d.Edition {
		t.Fatal("term edit did not change captured edition")
	}
	if len(v.Projection.UsesFor(live.Document.Record.ID+"@"+live.Document.Edition+"#total-preserved")) != 0 {
		t.Fatal("old pass transferred")
	}
	oldRead := run(t, s, Request{Operation: "recall", Ref: old + "#total-preserved"}, "found")
	if oldRead.Data == nil {
		t.Fatal("lost old snapshot")
	}
	q.RequestID = "evidence-stale"
	run(t, s, q, "conflict")
}

func TestChangePreviewApplyReplayAndArchiveAreSeparate(t *testing.T) {
	s := service(t)
	d, base := seed(t, s)
	cl := d.Record.Claims[0]
	cl.Text += " The exact amount is unchanged."
	c := change.Change{Format: change.Format, ID: "chg-20260923-00000001", ChangeKey: "chg-20260923-00000001", Title: "Clarify amount", Intent: "Clarify total semantics", State: "open", CreatedAt: s.now(), Tasks: []change.Task{{ID: "code", Text: "Review implementation", Done: false}}, Patches: []change.SectionPatch{{Base: base, Operations: []change.Operation{{Op: "MODIFIED", ClaimID: cl.ID, Claim: &cl, Reason: "Clarify scope"}}}}}
	b, err := change.Encode(c, []byte("Keep this authored prose.\n"))
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, Request{Operation: "change", Action: "create", RequestID: "create-change", Carrier: string(b)}, "written")
	preview := run(t, s, Request{Operation: "change", Action: "preview", Ref: c.ID}, "ready")
	q := Request{Operation: "change", Action: "apply", Ref: preview.Basis["change_ref"], RequestID: "apply-once", ExpectedGeneration: preview.Basis["memory_generation"], PreviewDigest: preview.Basis["preview_digest"]}
	run(t, s, q, "written")
	run(t, s, q, "replayed")
	v := readView(t, s)
	if len(v.Documents) != 2 {
		t.Fatalf("wanted exactly one spec successor: %d", len(v.Documents))
	}
	for _, doc := range v.Documents {
		if doc.Record.OperatorConfirmed || doc.Record.Status != "proposed" {
			t.Fatal("application inferred acceptance")
		}
	}
	run(t, s, Request{Operation: "change", Action: "archive", Ref: c.ID, RequestID: "archive", Revision: &change.Revision{Reason: "Preserve completed review; code task remains recorded"}}, "written")
	v = readView(t, s)
	if len(v.Documents) != 2 {
		t.Fatal("archive applied spec again")
	}
	list := run(t, s, Request{Operation: "change", Action: "list"}, "results")
	raw, _ := json.Marshal(list)
	if !strings.Contains(string(raw), "archived") || !strings.Contains(string(raw), "Keep this authored prose") { // bytes are base64; inspect carriers below
		found := false
		for _, doc := range changeDocuments(v) {
			if doc.Change.State == "archived" {
				found = true
				if doc.Change.Tasks[0].Done || string(doc.Body) != "Keep this authored prose.\n" {
					t.Fatal("archive changed task/prose")
				}
			}
		}
		if !found {
			t.Fatal("archive missing")
		}
	}
}

func TestPrepareUsesDeclaredCheckAndBidirectionalBindings(t *testing.T) {
	s := service(t)
	d, _ := seed(t, s)
	run(t, s, Request{Operation: "check", Action: "structural", Strict: true}, "structurally_valid")
	q := Request{Operation: "check", Action: "prepare", Ref: d.Record.ID + "#total-preserved", CheckRef: "pbt:order_test.go::TestCancelPreservesTotal", Scope: "1000 generated new/paid cases, seed 23"}
	prepared := run(t, s, q, "prepared")
	raw, _ := json.Marshal(prepared)
	if !strings.Contains(string(raw), "^TestCancelPreservesTotal$") || !strings.Contains(string(raw), "example.test/orders") {
		t.Fatal(string(raw))
	}
	q.CheckRef = "test:order_test.go::TestCurrency"
	run(t, s, q, "conflict")
	r := run(t, s, Request{Operation: "context", Ref: "sym:order.go::Order.Cancel"}, "exact")
	raw, _ = json.Marshal(r)
	if !strings.Contains(string(raw), "total-preserved") {
		t.Fatal("reverse implementation binding absent")
	}
	r = run(t, s, Request{Operation: "impact", Ref: "sym:order_test.go::TestCancelPreservesTotal"}, "exact")
	raw, _ = json.Marshal(r)
	if !strings.Contains(string(raw), "total-preserved") {
		t.Fatal("reverse oracle binding absent")
	}
}

func TestSourceWorksWithoutProjectMemoryAndPortableReplay(t *testing.T) {
	s := Service{Root: filepath.Join(t.TempDir(), "absent-project"), SourceRoot: "../source/testdata/pin-a", SourceRepository: "fixture://source"}
	// Select an exact valid fixture publication without assuming a host DB.
	entries, err := os.ReadDir(s.SourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	_ = entries
	r := run(t, s, Request{Operation: "source", Action: "inspect", Ref: "FPF-Spec.md"}, "found")
	data := r.Data.(map[string]any)
	blob := data["snapshot_bytes_base64"].([]byte)
	hash := r.Basis["source_snapshot"]
	s.SourceRoot = "/unavailable"
	run(t, s, Request{Operation: "source", Action: "inspect", Ref: hash, Snapshots: map[string][]byte{hash: blob}}, "found")
	run(t, s, Request{Operation: "source", Action: "inspect", Ref: hash}, "unavailable")
}

func TestPortableCodeCaptureDistinguishesFormattingAndDependency(t *testing.T) {
	s := service(t)
	seed(t, s)
	q := Request{Operation: "context", Ref: "sym:order.go::Order.Cancel", CaptureCode: true}
	initial := run(t, s, q, "exact").Data.(map[string]any)["code_capture"].(CodeCapture)
	q.CaptureCode = false
	q.PriorCode = &initial
	p := filepath.Join(s.Root, "order.go")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append([]byte("\n\n"), raw...), 0600); err != nil {
		t.Fatal(err)
	}
	changed := run(t, s, q, "exact").Data.(map[string]any)["code_change"].(code.Change)
	if changed.Kind != "navigation_only" || !changed.EvidenceBasisChanged {
		t.Fatalf("formatting result: %+v", changed)
	}
	helper := filepath.Join(s.Root, "policy.go")
	raw, err = os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	edit := strings.Replace(string(raw), `status == "new" || status == "paid"`, `status == "new"`, 1)
	if err := os.WriteFile(helper, []byte(edit), 0600); err != nil {
		t.Fatal(err)
	}
	changed = run(t, s, q, "exact").Data.(map[string]any)["code_change"].(code.Change)
	if changed.Kind == "navigation_only" || changed.Kind == "unchanged" {
		t.Fatalf("dependency not detected: %+v", changed)
	}
	initial.Files["order.go"] = []byte("tampered")
	run(t, s, q, "invalid")
}

func TestAdmissionRejectsInvalidBodyAndAlteredSourceLocator(t *testing.T) {
	s := service(t)
	q := Request{Operation: "remember", RequestID: "invalid-body", Carrier: "---\nkind: note\ntitle: Body\nabout: domain:Fixture\n---\n" + string([]byte{0xff})}
	run(t, s, q, "invalid")
	if len(readView(t, s).Documents) != 0 {
		t.Fatal("invalid bytes were published")
	}
	s.SourceRoot = "../source/testdata/pin-a"
	s.SourceRepository = "fixture://source"
	read := run(t, s, Request{Operation: "source", Action: "inspect", Ref: "FPF-Spec.md"}, "found")
	data := read.Data.(map[string]any)
	inspection := data["inspection"].(source.Inspection)
	original := inspection.Unit.Source
	note := carrier.Record{Kind: "note", Title: "Source basis", About: "domain:Fixture", Sources: []carrier.Source{original}}
	note.Sources[0].PublicationPath = "wrong.md"
	note.Sources[0].Lines = []int{900, 901}
	q = Request{Operation: "remember", RequestID: "source-note", Carrier: encode(t, note, nil), Snapshots: map[string][]byte{original.SnapshotRef: inspection.Unit.Snapshot}}
	run(t, s, q, "conflict")
	note.Sources[0] = original
	q.Carrier = encode(t, note, nil)
	run(t, s, q, "written")
	if v := readView(t, s); v.Coverage != "complete" {
		t.Fatal(v.Diagnostics)
	}
}
