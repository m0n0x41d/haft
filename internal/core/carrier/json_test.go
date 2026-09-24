package carrier

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestInlineClaimJSONPreservesNestedExtensions(t *testing.T) {
	raw := []byte(`{"id":"rule","kind":"guard","text":"Bounded input","x-number":9223372036854775807,"extra":{"literal":"keep"},"checks":[{"ref":"manual:review","covers":"Review","x-check":{"nested":[true,null,3]}}],"implemented_by":[{"ref":"file:order.go","covers":"Body","x-binding":"kept"}],"examples":[{"id":"case","text":"Scenario","x-example":7}],"evidence_inputs":[{"ref":"ev-20260924-aabbccdd#use","applicability":"Declared scope","x-input":"kept"}]}`)
	var claim Claim
	if err := json.Unmarshal(raw, &claim); err != nil {
		t.Fatal(err)
	}
	if claim.Checks[0].Extra["x-check"] == nil || claim.ImplementedBy[0].Extra["x-binding"] != "kept" || claim.Examples[0].Extra["x-example"] != 3+4 || claim.EvidenceInputs[0].Extra["x-input"] != "kept" {
		t.Fatalf("nested extensions lost: %#v", claim)
	}
	wire, err := json.Marshal(claim)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip Claim
	if err := json.Unmarshal(wire, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(claim, roundtrip) {
		t.Fatalf("roundtrip changed claim:\n%#v\n%#v", claim, roundtrip)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["x-number"]) != "9223372036854775807" || string(fields["extra"]) != `{"literal":"keep"}` {
		t.Fatalf("extension namespace changed: %s", wire)
	}
	// JSON frontmatter and typed JSON revision requests interpret the same fields.
	node, ds := ParseYAML(wire)
	if HasErrors(ds) {
		t.Fatal(ds)
	}
	var fromYAML Claim
	if err := node.Decode(&fromYAML); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(claim, fromYAML) {
		t.Fatal("JSON/YAML claim interpretations differ")
	}
}

func TestInlineClaimJSONRejectsAmbiguity(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"id":1}`, `{"id":"a","checks":[{"ref":3}]}`, `{"id":"a","id":"b"}`, `{"id":"a","x":{"same":1,"same":2}}`, `{"id":"a","checks":[{"ref":"manual:a","x":1,"x":2}]}`} {
		t.Run(raw, func(t *testing.T) {
			c := Claim{ID: "unchanged"}
			if json.Unmarshal([]byte(raw), &c) == nil {
				t.Fatal("ambiguous/non-object claim accepted")
			}
			if c.ID != "unchanged" {
				t.Fatal("failed decode partially mutated destination")
			}
		})
	}
	for _, value := range []any{Claim{Extra: Extra{"refs": []string{"other"}}}, Binding{Extra: Extra{"conditions": "hidden"}}, Example{Extra: Extra{"then": "hidden"}}, EvidenceInput{Extra: Extra{"applicability": "hidden"}}} {
		if _, err := json.Marshal(value); err == nil || !strings.Contains(err.Error(), "collides") {
			t.Fatalf("known/extension collision accepted: %#v, %v", value, err)
		}
	}
}

func TestInlineJSONDoesNotInterpretCaseFoldedExtension(t *testing.T) {
	var binding Binding
	if err := json.Unmarshal([]byte(`{"ref":"manual:review","covers":"Scope","Conditions":{"literal":true}}`), &binding); err != nil {
		t.Fatal(err)
	}
	if binding.Conditions != "" || !reflect.DeepEqual(binding.Extra["Conditions"], map[string]any{"literal": true}) {
		t.Fatalf("literal extension promoted to control field: %#v", binding)
	}
}

// These exact envelopes were emitted by the pre-fix implementation, captured
// before editing the codec. They exercise both raw-carrier and derived source
// envelopes; new JSON authoring must not reinterpret or redigest either one.
func TestR4HistoricalSnapshotCompatibility(t *testing.T) {
	recordRaw := []byte(r4HistoricalRecordSnapshot)
	snap, err := ReadSnapshot(recordRaw, "sha256:9e6f38dc606b6c2ed09a29c2b0d749d6264b671961b56ab93dc901c29ed1e4a6")
	if err != nil {
		t.Fatal(err)
	}
	d := snap.Document()
	if !d.Valid() {
		t.Fatal(d.Diagnostics)
	}
	if d.Record.Claims[0].Extra["x-bound"] != 5 || !reflect.DeepEqual(d.Record.Claims[0].Extra["extra"], map[string]any{"literal": "keep"}) {
		t.Fatalf("historical extension semantics changed: %#v", d.Record.Claims[0])
	}
	if !bytes.Equal(d.Bytes(), snap.Raw) {
		t.Fatal("historical carrier bytes changed")
	}
	_, encoded, digest, err := NewSnapshot(snap.Raw, snap.Interpretation)
	if err != nil || !bytes.Equal(encoded, recordRaw) || digest != "sha256:9e6f38dc606b6c2ed09a29c2b0d749d6264b671961b56ab93dc901c29ed1e4a6" {
		t.Fatalf("historical edition changed: %s %v", digest, err)
	}
	sourceRaw := []byte(r4HistoricalSourceSnapshot)
	src, err := ReadSourceSnapshot(sourceRaw, "sha256:4891f9f2a25a6a3b9bb589f25f34063bd10492654a32c536b7bd30baeb394ce7")
	if err != nil {
		t.Fatal(err)
	}
	if src.Provenance.Extra["slice"] != "excerpt" || src.Provenance.SourceRevision.Extra["revision_extension"] != "keep" || !reflect.DeepEqual(src.Provenance.Extra["extra"], map[string]any{"literal": "source field"}) {
		t.Fatalf("old source Extra envelopes reinterpreted: %#v", src.Provenance)
	}
	_, encoded, digest, err = NewSourceSnapshot(src.Raw, src.Provenance)
	if err != nil || !bytes.Equal(encoded, sourceRaw) || digest != "sha256:4891f9f2a25a6a3b9bb589f25f34063bd10492654a32c536b7bd30baeb394ce7" {
		t.Fatalf("historical source snapshot changed: %s %v", digest, err)
	}
}

const r4HistoricalRecordSnapshot = `{"format":"haft.snapshot/1","carrier_bytes_base64":"LS0tCmZvcm1hdDogaGFmdC8xCmlkOiBzcGVjLTIwMjYwOTI0LWFhYmJjY2RkCmtpbmQ6IHNwZWMKdGl0bGU6IEhpc3RvcmljYWwgZXh0ZW5zaW9uCnN0YXR1czogcHJvcG9zZWQKb3JpZ2luOiBhZ2VudF9wcm9wb3NhbAphYm91dDogZG9tYWluOlJldmlldy5FeHRlbnNpb24KY3JlYXRlZF9hdDogMjAyNi0wOS0yNFQxMDowMDowMFoKc2x1ZzogaGlzdG9yaWMtcjQKcmVjZWl2aW5nX3VzZTogRXh0ZW5zaW9uIHByZXNlcnZhdGlvbgpjbGFpbXM6CiAgLSBpZDogcnVsZQogICAga2luZDogZGVmaW5pdGlvbgogICAgdGV4dDogRXhhY3Qgb3JpZ2luYWwgbWVhbmluZy4KICAgIHgtYm91bmQ6IDUKICAgIGV4dHJhOiB7bGl0ZXJhbDoga2VlcH0KLS0tCk9yaWdpbmFsIHByb3NlLgo=","interpretation":{"version":"haft.interpretation/1","namespace":"","terms":null,"other":[]}}
`
const r4HistoricalSourceSnapshot = `{"format":"haft.source-snapshot/1","source_bytes_base64":"Q2FwdHVyZWQgaGlzdG9yaWNhbCBzb3VyY2Ugc2xpY2UuCg==","provenance":{"ref":"FPF:Example","source_revision":{"kind":"unknown","reason":"Fixture lacks revision","extra":{"revision_extension":"keep"}},"body_digest":"sha256:218ca193075848a5dfaa444ff68f1214155c0fd5988268b6a3fb20fc09fb02a1","extra":{"extra":{"literal":"source field"},"slice":"excerpt"}}}
`
