package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/check"
	"github.com/m0n0x41d/haft/internal/core/delivery"
)

func public(t *testing.T, s Service, q Request) delivery.Response {
	t.Helper()
	q.Format = delivery.Format
	r := s.Call(context.Background(), q)
	if n := delivery.Size(r); n > delivery.Budget {
		t.Fatalf("oversized public result %d", n)
	}
	return r
}
func nextApp(q delivery.Request) Request {
	raw, _ := json.Marshal(q)
	var next Request
	json.Unmarshal(raw, &next)
	return next
}
func deliveredPart(t *testing.T, s Service, r delivery.Response, name string) delivery.Request {
	t.Helper()
	for _, p := range r.Delivery.Available {
		if p.Name == name {
			return p.Request
		}
	}
	if r.Delivery.Catalog == nil {
		t.Fatalf("no part directory: %+v", r)
	}
	q := *r.Delivery.Catalog
	for {
		page := public(t, s, nextApp(q))
		if page.IsError && page.Kind != "unattributable" && page.Kind != "assertion_failure" {
			t.Fatalf("part directory failed: %+v", page)
		}
		raw, _ := json.Marshal(page.Data)
		var data struct {
			Parts []delivery.Descriptor `json:"parts"`
		}
		json.Unmarshal(raw, &data)
		for _, p := range data.Parts {
			if p.Name == name {
				return p.Request
			}
		}
		if page.Delivery.Next == nil {
			t.Fatalf("missing part %s", name)
		}
		q = *page.Delivery.Next
	}
}
func collectPart(t *testing.T, s Service, q delivery.Request) []byte {
	t.Helper()
	q.View = "bytes"
	var all []byte
	for n := 0; ; n++ {
		if n > 10000 {
			t.Fatal("unbounded continuation")
		}
		r := public(t, s, nextApp(q))
		if r.Delivery.Encoding != "base64" {
			t.Fatalf("chunk failed: %+v", r)
		}
		var data struct {
			Raw []byte `json:"bytes_base64"`
		}
		raw, _ := json.Marshal(r.Data)
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		if r.Delivery.Offset != len(all) {
			t.Fatal("offset skipped")
		}
		all = append(all, data.Raw...)
		if r.Delivery.Next == nil {
			if !r.Delivery.Complete || len(all) != r.Delivery.TotalBytes || carrier.Digest(all) != r.Delivery.Digest {
				t.Fatal("incomplete/digest mismatch")
			}
			break
		}
		q = *r.Delivery.Next
	}
	return all
}
func TestPublicExactPartsRestartGenerationAndCorruption(t *testing.T) {
	s := service(t)
	body := []byte("Author scope.\n\nDiagnostic report:\n```json\n{\"result_kind\":\"unattributable\",\"data\":{\"runner_outcome\":\"passed\",\"current_basis\":\"unknown\"},\"diagnostics\":[{\"code\":\"declared_check_mismatch\"}]}\n```\n" + strings.Repeat("Кириллица < & \" \\ 🌱\n", 400))
	record := carrier.Record{Kind: "spec", Title: "Progressive claim", About: "domain:Delivery", Slug: "delivery", ReceivingUse: "Extension authoring and evidence", Claims: []carrier.Claim{{ID: "rule", Kind: "definition", Text: "Some complete text", Extra: carrier.Extra{"x-nested": map[string]any{"bytes": strings.Repeat("X", 12000)}}}}}
	r := public(t, s, Request{Operation: "remember", RequestID: "delivery-record", Carrier: encode(t, record, body)})
	if r.Kind != "written" {
		t.Fatal(r)
	}
	summary := public(t, s, Request{Operation: "recall", Ref: "spec:delivery#rule"})
	if summary.Kind != "found" {
		t.Fatal(summary)
	}
	carrierQ := deliveredPart(t, s, summary, "carrier")
	snapshotQ := deliveredPart(t, s, summary, "snapshot")
	claimQ := deliveredPart(t, s, summary, "claim")
	bodyQ := deliveredPart(t, s, summary, "body")
	linksQ := deliveredPart(t, s, summary, "links")
	original := collectPart(t, s, carrierQ)
	if !bytes.Equal(collectPart(t, s, bodyQ), body) {
		t.Fatal("body changed")
	}
	snapshot := collectPart(t, s, snapshotQ)
	ref, _ := carrier.ParseRef(carrierQ.Ref)
	if carrier.Digest(snapshot) != ref.Digest {
		t.Fatal("snapshot digest not edition")
	}
	claim := collectPart(t, s, claimQ)
	if !bytes.Contains(claim, []byte("x-nested")) {
		t.Fatal("claim extensions missing")
	}
	reports := deliveredPart(t, s, summary, "reports")
	if !bytes.Contains(collectPart(t, s, reports), []byte("Diagnostic report")) {
		t.Fatal("report lost")
	}
	// A new generation invalidates selection pages, not the exact persisted bytes.
	public(t, s, Request{Operation: "remember", RequestID: "another", Carrier: "---\nkind: note\ntitle: Another generation\nabout: domain:Delivery\n---\nNote.\n"})
	if got := public(t, s, nextApp(linksQ)); got.Kind != "stale" {
		t.Fatal("relations accepted changed generation", got)
	}
	if err := os.RemoveAll(filepath.Join(s.Root, ".haft/.cache")); err != nil {
		t.Fatal(err)
	}
	restarted := Service{Root: s.Root}
	if !bytes.Equal(collectPart(t, restarted, carrierQ), original) {
		t.Fatal("pinned read depended on cache or process")
	}
	// Corruption and absence are different outcomes and neither substitutes live bytes.
	path := filepath.Join(s.Root, ".haft/editions/sha256", strings.TrimPrefix(ref.Digest, "sha256:")+".json")
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := public(t, restarted, nextApp(snapshotQ)); got.Kind != "corrupt" {
		t.Fatalf("corrupt became %s: %+v", got.Kind, got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := public(t, restarted, nextApp(snapshotQ)); got.Kind != "missing" {
		t.Fatal("missing not explicit", got)
	}
}
func TestTransientPrepareContextObserveRetention(t *testing.T) {
	s, q := runnerRegressionFixture(t, map[string]string{"answer.go": "package answer\nfunc Answer()int{return 1}\n", "answer_test.go": runnerAnswerTest})
	prepared := public(t, s, q)
	if prepared.Kind != "prepared" {
		t.Fatal(prepared)
	}
	expectedQ := deliveredPart(t, s, prepared, "expected")
	var expected check.Contract
	if err := json.Unmarshal(collectPart(t, s, expectedQ), &expected); err != nil {
		t.Fatal(err)
	}
	if expected.Basis.Code == "" {
		t.Fatal("expected lost")
	}
	original := s.Execute(context.Background(), Request{Format: Format, Operation: "context", CaptureCode: true})
	contextResult := public(t, s, Request{Operation: "context", CaptureCode: true})
	captureQ := deliveredPart(t, s, contextResult, "code_capture")
	if !sameJSON(delivery.Value(collectPart(t, s, captureQ)), delivery.Value(rawJSON(original.Data.(map[string]any)["code_capture"]))) {
		t.Fatal("context capture changed")
	}
	before := runnerRegressionPrepare(t, s, q)
	input := runnerRegressionRun(t, s, before, 0)
	// Transform this real local run into synthetic foreign metadata to exercise
	// presentation. TestObserveCannotAttributeOtherPackageWithSameTestName runs
	// the actual foreign package separately.
	input.Expected.Selector.Package += "/other"
	input.Observed.Selector = input.Expected.Selector
	input.Observed.Command[len(input.Observed.Command)-1] = input.Expected.Selector.Package
	input.Observed.Stdout = bytes.ReplaceAll(input.Observed.Stdout, []byte("regression.example/answer"), []byte("regression.example/answer/other"))
	observed := public(t, s, Request{Operation: "check", Action: "observe", Observation: &input})
	if observed.Kind != "unattributable" || !observed.IsError {
		t.Fatal("false attribution")
	}
	fields := object(delivery.Value(rawJSON(observed.Data)))
	if fields["runner_outcome"] != "passed" || fields["current_basis"] != "unknown" || fields["declared_check_binding"] != false {
		t.Fatal("summary hid refusal", fields)
	}
	diagnostics := collectPart(t, s, deliveredPart(t, s, observed, "diagnostics"))
	if !bytes.Contains(diagnostics, []byte("declared_check_mismatch")) {
		t.Fatal("diagnostic inaccessible")
	}
	observedQ := deliveredPart(t, s, observed, "result")
	exact := collectPart(t, s, observedQ)
	runnerRegressionWrite(t, s.Root, "answer.go", "package answer\nfunc Answer()int{return 2}\n")
	if stale := public(t, s, nextApp(expectedQ)); stale.Kind != "stale" {
		t.Fatal("changed prepare basis not stale")
	}
	if stale := public(t, s, nextApp(captureQ)); stale.Kind != "stale" {
		t.Fatal("changed context basis not stale")
	}
	// Observe is a captured historical event; code drift does not destroy it.
	if !bytes.Equal(collectPart(t, s, observedQ), exact) {
		t.Fatal("observation rewritten")
	}
	remember := Request{Operation: "remember", RequestID: "retain", Carrier: "---\nkind: note\ntitle: Retained exact observation\nabout: domain:Delivery\n---\nThe application refused attribution.\n", Retain: []Retention{{Ref: observedQ.Ref, Part: "result"}}}
	if saved := public(t, s, remember); saved.Kind != "written" {
		t.Fatal(saved)
	}
	saved := public(t, s, Request{Operation: "recall", Query: "Retained exact observation"})
	if saved.Kind != "results" {
		t.Fatal(saved)
	}
	v := readView(t, s)
	found := false
	savedID := ""
	for _, doc := range v.Documents {
		if doc.Record.Title == "Retained exact observation" {
			savedID = doc.Record.ID
			d := makeDelivery(Request{Operation: "recall", Ref: doc.Record.ID}, recall(Request{Ref: doc.Record.ID}, Result{Basis: map[string]string{}, Diagnostics: []carrier.Diagnostic{}}, v))
			p, err := d.Member("report_1")
			if err != nil {
				t.Fatal(err)
			}
			var retained struct {
				Raw []byte `json:"bytes_base64"`
			}
			if err = json.Unmarshal(p.Raw, &retained); err != nil {
				t.Fatal(err)
			}
			found = bytes.Equal(retained.Raw, exact)
		}
	}
	if !found {
		t.Fatal("retention lost complete original observation")
	}
	os.RemoveAll(filepath.Join(s.Root, ".haft/.cache"))
	if expired := public(t, s, nextApp(observedQ)); expired.Kind != "expired" {
		t.Fatal("missing transient became history", expired.Kind)
	}
	// A saved JSON attachment remains field-readable after all transient data
	// disappears. The client follows returned member requests, not base64 blobs.
	restarted := Service{Root: s.Root}
	retained := public(t, restarted, Request{Operation: "recall", Ref: savedID})
	contentQ := deliveredPart(t, restarted, retained, "report_1_content")
	if !bytes.Equal(collectPart(t, restarted, contentQ), exact) {
		t.Fatal("decoded retained bytes changed")
	}
	qMember := contentQ
	selected := false
	for n := 0; n < 30; n++ {
		page := public(t, restarted, nextApp(qMember))
		var members struct {
			Members []struct {
				Key  string              `json:"key"`
				Read delivery.Descriptor `json:"read"`
			} `json:"members"`
		}
		if err := json.Unmarshal(rawJSON(page.Data), &members); err != nil {
			t.Fatal(err)
		}
		for _, member := range members.Members {
			if member.Key == "result_kind" {
				field := public(t, restarted, nextApp(member.Read.Request))
				if object(field.Data)["text"] != "unattributable" || !field.Delivery.Complete {
					t.Fatal("retained field unavailable", field)
				}
				selected = true
			}
		}
		if selected || page.Delivery.Next == nil {
			break
		}
		qMember = *page.Delivery.Next
	}
	if !selected {
		t.Fatal("no selective retained JSON read")
	}
	if replay := public(t, s, remember); replay.Kind != "replayed" {
		t.Fatal("committed retention replay needed cache", replay)
	}
}

func TestRetainedAttachmentViewsRejectCorruption(t *testing.T) {
	for _, tc := range []struct {
		name, media, digest, wantError string
		raw                            []byte
	}{
		{"json", "json", "", "", []byte(`{"unknown-extension":{"number":900719925474099312345}}`)},
		{"text", "text", "", "", []byte("Кириллица < & \n")},
		{"binary", "binary", "", "", []byte{0, 255, 1}},
		{"corrupt", "json", "sha256:incorrect", "retained_digest_mismatch", []byte(`{}`)},
		{"invalid-json", "json", "", "retained_encoding_invalid", []byte("broken")},
		{"invalid-utf8", "text", "", "retained_encoding_invalid", []byte{255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			digest := tc.digest
			if digest == "" {
				digest = carrier.Digest(tc.raw)
			}
			raw := rawJSON(map[string]any{"format": "haft.retained-part/1", "media": tc.media, "digest": digest, "bytes_base64": tc.raw})
			var d delivery.Document
			addReports(&d, []byte("Retained data:\n```json\n"+string(raw)+"\n```\n"))
			catalog, _ := d.Member("reports")
			var entries []map[string]any
			json.Unmarshal(catalog.Raw, &entries)
			p, err := d.Member("report_1_content")
			if tc.wantError != "" {
				if err == nil || entries[0]["content_error"] != tc.wantError {
					t.Fatal("invalid bytes advertised as verified", entries)
				}
			} else if err != nil || !bytes.Equal(p.Raw, tc.raw) || p.Media != tc.media {
				t.Fatal("retained projection lost bytes", err)
			}
			envelope, _ := d.Member("report_1")
			if !bytes.Equal(bytes.TrimSpace(envelope.Raw), raw) {
				t.Fatal("envelope rewritten")
			}
		})
	}
}
func TestTransientSourceStaleAndCorrupt(t *testing.T) {
	root := t.TempDir()
	sources := t.TempDir()
	raw := "# A.1\n\n## A.1 - Exact source\n\nComplete governing body.\n\n### A.1:End\n"
	path := filepath.Join(sources, "FPF-Spec.md")
	os.WriteFile(path, []byte(raw), 0600)
	s := Service{Root: root, SourceRoot: sources, SourceRepository: "fixture-source"}
	response := public(t, s, Request{Operation: "fpf", Action: "inspect", Ref: "A.1"})
	if response.Kind != "found" {
		t.Fatal(response)
	}
	q := deliveredPart(t, s, response, "source_body")
	if !bytes.Contains(collectPart(t, s, q), []byte("Complete governing body")) {
		t.Fatal("body omitted")
	}
	os.WriteFile(path, []byte(strings.ReplaceAll(raw, "Complete", "Changed")), 0600)
	if got := public(t, s, nextApp(q)); got.Kind != "stale" {
		t.Fatal("source drift accepted", got)
	}
	os.WriteFile(path, []byte(raw), 0600)
	cache := filepath.Join(root, ".haft/.cache/disclosure", strings.TrimPrefix(q.Ref, "result:sha256:")+".json")
	os.WriteFile(cache, []byte("corrupt"), 0600)
	if got := public(t, s, nextApp(q)); got.Kind != "corrupt" {
		t.Fatal("corrupt transient accepted", got)
	}
}

func TestYAMLExtensionsRetainCanonicalBytes(t *testing.T) {
	s := service(t)
	raw := "---\nkind: spec\ntitle: YAML extension\nabout: domain:Delivery\nslug: yaml-extension\nreceiving_use: Preserve non-JSON extensions\nclaims:\n  - id: rule\n    kind: definition\n    text: Typed definition\n    x-float: .nan\n---\nExact authored body.\n"
	saved := public(t, s, Request{Operation: "remember", RequestID: "yaml-extension", Carrier: raw})
	if saved.Kind != "written" {
		t.Fatal(saved)
	}
	read := public(t, s, Request{Operation: "recall", Ref: "spec:yaml-extension#rule"})
	if read.Kind != "found" {
		t.Fatal(read)
	}
	carrierRaw := collectPart(t, s, deliveredPart(t, s, read, "carrier"))
	if !bytes.Contains(carrierRaw, []byte("x-float: .nan")) {
		t.Fatal("non-JSON extension lost")
	}
	claimRaw := collectPart(t, s, deliveredPart(t, s, read, "claim"))
	if !bytes.Contains(claimRaw, []byte("x-float: .nan")) {
		t.Fatal("YAML authoring projection lost extension")
	}
}

func TestLargeTransientResultsUseSameBoundedPath(t *testing.T) {
	s := service(t)
	huge := strings.Repeat("Журнал \\\"<&>\n", 140000)
	for _, op := range []struct {
		operation, action, kind string
		data                    map[string]any
	}{
		{"check", "prepare", "prepared", map[string]any{"expected": map[string]any{"scope": "Bounded scope", "basis": map[string]any{"claim": "exact"}}, "basis_capture": map[string]any{"implementation_preimage_base64": huge}}},
		{"check", "observe", "unattributable", map[string]any{"current_basis": "unknown", "declared_check_binding": false, "observation": map[string]any{"status": "passed", "reason_code": "selected_test_passed", "input": map[string]any{"observed": map[string]any{"stdout_base64": huge}}}}},
		{"context", "", "results", map[string]any{"code_capture": map[string]any{"files": map[string]any{"big.go": huge}}}},
		{"change", "preview", "conflict", map[string]any{"diagnostics": []any{map[string]any{"code": "conflict", "message": huge}}, "outputs": []any{map[string]any{"body": huge}}}},
	} {
		t.Run(op.operation+op.action, func(t *testing.T) {
			q := Request{Format: Format, Operation: op.operation, Action: op.action}
			r := Result{Format: Format, Operation: op.operation, Kind: op.kind, Data: op.data, Basis: map[string]string{}, Diagnostics: []carrier.Diagnostic{{Code: "large_diagnostic", Message: huge, Severity: "warning"}}, Limits: []string{huge}, Coverage: "degraded"}
			delivered := s.deliver(context.Background(), q, r, "", "", "")
			if delivery.Size(delivered) > delivery.Budget || delivered.Kind != r.Kind || delivered.IsError != r.Failed() {
				t.Fatal("budget changed outcome")
			}
			if delivered.Delivery.Catalog == nil {
				t.Fatal("large transient lost continuation")
			}
			document, err := s.loadTransient(context.Background(), delivered.Delivery.Catalog.Ref, true)
			if err != nil {
				t.Fatal(err)
			}
			exact, err := document.Member("result")
			if err != nil || !sameJSON(delivery.Value(exact.Raw), delivery.Value(rawJSON(r))) {
				t.Fatal("cached input/output changed")
			}
			seen := map[string]bool{}
			for _, p := range document.Parts {
				if seen[p.Name] {
					t.Fatal("duplicate part locator", p.Name)
				}
				seen[p.Name] = true
			}
		})
	}
}
