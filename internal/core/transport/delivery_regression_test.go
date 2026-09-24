package transport

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/core/app"
	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/delivery"
)

// Measure the actual serialized MCP result, including both representations and
// escaping. Every call also checks text-only/structuredContent/error parity.
func deliveryCall(t *testing.T, c *client, q app.Request) delivery.Response {
	t.Helper()
	q.Format = delivery.Format
	fmt.Fprint(c.in, call(2, q))
	line, err := c.out.ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err := json.Unmarshal(line, &rpc); err != nil || rpc.Error != nil {
		t.Fatalf("RPC: %s (%v)", line, err)
	}
	if len(rpc.Result) > delivery.Budget {
		t.Fatalf("serialized MCP result: %d > %d", len(rpc.Result), delivery.Budget)
	}
	var wire struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Structured delivery.Response `json:"structuredContent"`
		IsError    bool              `json:"isError"`
	}
	if err := json.Unmarshal(rpc.Result, &wire); err != nil {
		t.Fatal(err)
	}
	var text delivery.Response
	if len(wire.Content) != 1 {
		t.Fatal("missing text parity")
	}
	if err := json.Unmarshal([]byte(wire.Content[0].Text), &text); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire.Structured, text) || wire.IsError != text.IsError {
		t.Fatal("MCP parity")
	}
	return wire.Structured
}
func readDelivery(t *testing.T, c *client, q delivery.Request) delivery.Response {
	t.Helper()
	raw, _ := json.Marshal(q)
	var request app.Request
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	return deliveryCall(t, c, request)
}
func deliveryParts(t *testing.T, c *client, r delivery.Response) map[string]delivery.Descriptor {
	t.Helper()
	parts := map[string]delivery.Descriptor{}
	for q := r.Delivery.Catalog; q != nil; {
		page := readDelivery(t, c, *q)
		if page.Delivery.Encoding != "part_directory" {
			t.Fatalf("directory: %+v", page)
		}
		var data struct {
			Parts []delivery.Descriptor `json:"parts"`
		}
		raw, _ := json.Marshal(page.Data)
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		for _, p := range data.Parts {
			parts[p.Name] = p
		}
		q = page.Delivery.Next
	}
	return parts
}
func deliveryBytes(t *testing.T, c *client, q delivery.Request, kind, encoding string, isError bool) []byte {
	t.Helper()
	var full []byte
	digest := q.ExpectedDigest
	for {
		page := readDelivery(t, c, q)
		state := page.Delivery
		if page.Kind != kind || page.IsError != isError || state.Encoding != encoding {
			t.Fatalf("chunk outcome: %+v", page)
		}
		if state.Offset != len(full) || state.Digest != digest {
			t.Fatal("chunk basis/offset changed")
		}
		data := page.Data.(map[string]any)
		var chunk []byte
		if encoding == "utf-8" {
			chunk = []byte(data["text"].(string))
			if !utf8.Valid(chunk) {
				t.Fatal("split UTF-8 code point")
			}
		} else {
			var err error
			chunk, err = base64.StdEncoding.DecodeString(data["bytes_base64"].(string))
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(chunk) != state.ReturnedBytes {
			t.Fatal("incorrect chunk length")
		}
		full = append(full, chunk...)
		if state.Next == nil {
			if !state.Complete || len(full) != state.TotalBytes || carrier.Digest(full) != digest {
				t.Fatal("incomplete or lossy member")
			}
			return full
		}
		if state.Complete || len(chunk) == 0 {
			t.Fatal("non-progressing continuation")
		}
		q = *state.Next
	}
}
func TestMCPDirectorySelectionAndImmutableReads(t *testing.T) {
	root := t.TempDir()
	service := app.Service{Root: root}
	c := newClient(t, service)
	created := deliveryCall(t, c, app.Request{Operation: "remember", RequestID: "r9-spec", Carrier: "---\nkind: spec\ntitle: Directory selection\nslug: r9\nabout: domain:Edge\nreceiving_use: Delivery regression\nclaims:\n- id: rule\n  kind: definition\n  text: Keep this exact claim\n  future_extension: {number: 9007199254740993}\n---\nExact body.\n"})
	if created.Kind != "written" {
		t.Fatal(created)
	}
	summary := deliveryCall(t, c, app.Request{Operation: "recall", Ref: "spec:r9#rule", Limit: 1})
	parts := deliveryParts(t, c, summary)
	first := readDelivery(t, c, *summary.Delivery.Catalog)
	if first.Delivery.Next == nil {
		t.Fatal("missing directory continuation")
	}
	oldNext := *first.Delivery.Next
	originals := map[string][]byte{}
	for _, name := range []string{"claim", "carrier", "snapshot"} {
		q := parts[name].Request
		if q.ExpectedGeneration != "" {
			t.Fatalf("immutable %s is generation-gated", name)
		}
		q.View = "bytes"
		originals[name] = deliveryBytes(t, c, q, "found", "base64", false)
	}
	header, _ := json.Marshal(map[string]any{"kind": "note", "title": "Actual backlink mutation", "about": "domain:Edge", "links": []any{map[string]any{"kind": "relies_on", "target": parts["claim"].Request.Ref, "reason": "Selected relation changed"}}})
	note := deliveryCall(t, c, app.Request{Operation: "remember", RequestID: "r9-note", Carrier: "---\n" + string(header) + "\n---\nUses the exact claim.\n"})
	if note.Kind != "written" {
		t.Fatal(note)
	}
	// An unchanged cursor must never splice a second generation into this list.
	for _, q := range []delivery.Request{oldNext, *summary.Delivery.Catalog, parts["backlinks"].Request} {
		stale := readDelivery(t, c, q)
		if stale.Kind != "stale" || !stale.IsError {
			t.Fatalf("old %s request: want stale, got %s", q.Part, stale.Kind)
		}
	}
	if oldNext.ExpectedGeneration != first.Basis["memory_generation"] {
		t.Fatal("directory continuation omitted its captured generation")
	}
	current := deliveryCall(t, c, app.Request{Operation: "recall", Ref: "spec:r9#rule", Limit: 1})
	now := deliveryParts(t, c, current)
	if now["backlinks"].Digest == parts["backlinks"].Digest {
		t.Fatal("test did not change selected backlinks")
	}
	unrelated := deliveryCall(t, c, app.Request{Operation: "remember", RequestID: "r9-unrelated", Carrier: "---\nkind: note\ntitle: Unrelated memory\nabout: domain:Other\n---\nUnrelated write.\n"})
	if unrelated.Kind != "written" {
		t.Fatal(unrelated)
	}
	if err := os.RemoveAll(filepath.Join(root, ".haft", ".cache", "disclosure")); err != nil {
		t.Fatal(err)
	}
	restarted := newClient(t, app.Service{Root: root})
	for name, want := range originals {
		q := parts[name].Request
		q.View = "bytes"
		if got := deliveryBytes(t, restarted, q, "found", "base64", false); !bytes.Equal(got, want) {
			t.Fatalf("immutable %s changed after generation/cache/restart", name)
		}
	}
}
