package delivery

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

func exampleDocument(parts ...Part) Document {
	return Document{Ref: "result:sha256:" + strings.Repeat("1", 64), Lifetime: "transient_cache", Operation: "check", Kind: "unattributable", IsError: true, Basis: map[string]string{"memory_generation": "sha256:" + strings.Repeat("2", 64)}, Coverage: "degraded", Summary: json.RawMessage(`{"application_outcome":"unattributable","runner_outcome":"passed","current_basis":"unknown","declared_check_binding":false}`), Parts: parts}
}
func wireCheck(t *testing.T, r Response) {
	t.Helper()
	raw, err := json.Marshal(MCPResult(r))
	if err != nil || len(raw) > Budget {
		t.Fatalf("wire size %d: %v", len(raw), err)
	}
	var wire struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Structured json.RawMessage `json:"structuredContent"`
		IsError    bool            `json:"isError"`
	}
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(wire.Content[0].Text)) || !bytes.Equal([]byte(wire.Content[0].Text), wire.Structured) || wire.IsError != r.IsError {
		t.Fatal("text/structured/error divergence")
	}
}
func TestSerializedBudgetLosslessChunks(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"ascii", []byte(strings.Repeat("hello\\world\"\n", 1000))},
		{"cyrillic", []byte(strings.Repeat("Иван: «проверка» 🌱\n", 1000))},
		{"escaping", bytes.Repeat([]byte{0, 1, 2, 7, '\n', '\r', '\t', '"', '\\', '<', '>', '&'}, 1000)},
		{"multi-MiB", []byte(strings.Repeat("Крупный payload & < > \\\t\"\n", 90000))},
		{"binary", bytes.Repeat([]byte{255, 254, 0, 127}, 2000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := exampleDocument(Binary("carrier", tc.raw))
			q := readRequest(d, d.Parts[0], "bytes")
			var got []byte
			count := 0
			for {
				r := Present(d, q)
				wireCheck(t, r)
				if r.Kind != "unattributable" || !r.Failed() {
					t.Fatal("outcome changed", r.Kind)
				}
				var data struct {
					Bytes []byte `json:"bytes_base64"`
				}
				b, _ := json.Marshal(r.Data)
				if err := json.Unmarshal(b, &data); err != nil {
					t.Fatal(err)
				}
				if r.Delivery.Offset != len(got) || r.Delivery.ReturnedBytes != len(data.Bytes) || r.Delivery.TotalBytes != len(tc.raw) || r.Delivery.Digest != carrier.Digest(tc.raw) {
					t.Fatal("invalid chunk basis")
				}
				got = append(got, data.Bytes...)
				count++
				if r.Delivery.Next == nil {
					if !r.Delivery.Complete {
						t.Fatal("unfinished")
					}
					break
				}
				if len(data.Bytes) == 0 {
					t.Fatal("no progress")
				}
				q = *r.Delivery.Next
			}
			if !bytes.Equal(got, tc.raw) {
				t.Fatal("lost bytes")
			}
			t.Logf("bytes=%d pages=%d", len(got), count)
		})
	}
}
func TestUTF8DetailAndCursorBinding(t *testing.T) {
	raw := []byte(strings.Repeat("я 🌱 \" < >\n", 4000))
	d := exampleDocument(Text("body", raw))
	q := readRequest(d, d.Parts[0], "detail")
	var got []byte
	first := Present(d, q)
	if first.Delivery.Next == nil {
		t.Fatal("missing continuation")
	}
	for _, mutate := range []func(*Request){func(q *Request) { q.View = "bytes" }, func(q *Request) { q.Part = "summary" }, func(q *Request) { q.Ref += "x" }, func(q *Request) { q.ExpectedGeneration = "changed" }, func(q *Request) { q.ExpectedDigest = "sha256:" + strings.Repeat("9", 64) }, func(q *Request) { q.Cursor = q.Cursor[:len(q.Cursor)-1] + "!" }} {
		bad := *first.Delivery.Next
		mutate(&bad)
		r := Present(d, bad)
		wireCheck(t, r)
		if !r.IsError || r.Kind == "unattributable" {
			t.Fatal("cursor mutation accepted")
		}
	}
	for {
		r := Present(d, q)
		wireCheck(t, r)
		text := r.Data.(map[string]any)["text"].(string)
		if !utf8.ValidString(text) {
			t.Fatal("split UTF-8")
		}
		got = append(got, text...)
		if r.Delivery.Next == nil {
			break
		}
		q = *r.Delivery.Next
	}
	if !bytes.Equal(raw, got) {
		t.Fatal("text lost")
	}
}
func TestHugeMembersDiagnosticsAndDirectories(t *testing.T) {
	huge := strings.Repeat("Иван\"\n\\\x00<>", 50000)
	members := map[string]any{"id": "rule", "x-extension": map[string]any{"big": huge, "number": json.Number("900719925474099312345")}, strings.Repeat("key", 5000): "long key"}
	parts := []Part{JSON("claim", members, false)}
	for i := 0; i < 150; i++ {
		p := Text(fmt.Sprintf("part_%d", i), []byte(huge))
		p.Label = huge
		parts = append(parts, p)
	}
	d := exampleDocument(parts...)
	for i := 0; i < 2000; i++ {
		d.Diagnostics = append(d.Diagnostics, carrier.Diagnostic{Code: huge, Path: huge, Message: huge, Severity: "error"})
		d.Limits = append(d.Limits, huge)
	}
	d.Summary = rawJSONTest(members)
	r := Present(d, Request{})
	wireCheck(t, r)
	if r.Delivery.Catalog == nil || r.Delivery.Omissions.Diagnostics != 2000 {
		t.Fatal("hidden omissions")
	}
	q := *r.Delivery.Catalog
	total := 0
	for {
		r = Present(d, q)
		wireCheck(t, r)
		b, _ := json.Marshal(r.Data)
		var page struct {
			Parts []Descriptor `json:"parts"`
		}
		json.Unmarshal(b, &page)
		total += len(page.Parts)
		if r.Delivery.Next == nil {
			break
		}
		q = *r.Delivery.Next
	}
	if total != len(parts) {
		t.Fatal("lost directory parts", total)
	}
	r = Present(d, readRequest(d, parts[0], "detail"))
	wireCheck(t, r)
	if r.Delivery.Encoding != "json_members" || r.Delivery.Complete {
		t.Fatal("large object presented complete")
	}
	// The whole authoring structure, including exact large integers and unknown
	// nested extensions, is available independently of the human-readable view.
	q = readRequest(d, parts[0], "bytes")
	var full []byte
	for {
		r = Present(d, q)
		wireCheck(t, r)
		b, _ := json.Marshal(r.Data)
		var data map[string]string
		json.Unmarshal(b, &data)
		chunk, _ := base64.StdEncoding.DecodeString(data["bytes_base64"])
		full = append(full, chunk...)
		if r.Delivery.Next == nil {
			break
		}
		q = *r.Delivery.Next
	}
	if !bytes.Equal(full, parts[0].Raw) {
		t.Fatal("authoring structure lost")
	}
}
func rawJSONTest(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
