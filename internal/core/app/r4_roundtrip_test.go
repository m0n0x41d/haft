package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/core/app"
	"github.com/m0n0x41d/haft/internal/core/transport"
)

// Exercise the installed skill's actual authoring route: serialize recall,
// copy its complete claim into JSON frontmatter, preview/apply, then recall.
// No carrier encoder or internal projection is used to build the change.
func TestR4RecallJSONChangeCarrierRoundTrip(t *testing.T) {
	for _, route := range []string{"carrier", "typed-revision"} {
		for _, edit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/edit=%t", route, edit), func(t *testing.T) {
				id := 0
				s := app.Service{Root: t.TempDir(), Now: func() time.Time {
					return time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
				}, NewID: func(kind string) (string, error) {
					id++
					if kind == "change" {
						return fmt.Sprintf("chg-20260924-%08x", id+1), nil
					}
					return fmt.Sprintf("spec-20260924-%08x", id), nil
				}}
				r4Call(t, s, app.Request{Operation: "remember", RequestID: "seed-r4", Carrier: r4Spec}, "written")
				before := r4Call(t, s, app.Request{Operation: "recall", Ref: "spec:r4"}, "found")
				claim := r4Claim(before)
				want := r4ExpectedClaim(t)
				if edit {
					// On the old API these fields live in extra. Copying that exact
					// returned object reproduces the reported nested-extra failure.
					r4Extension(claim, "x-bound")["x-bound"] = float64(7)
					r4Extension(r4ObjectAt(claim, "checks"), "x-check")["x-check"] = "revised check"
					r4Extension(r4ObjectAt(claim, "implemented_by"), "x-binding")["x-binding"] = "revised binding"
					r4Extension(r4ObjectAt(claim, "examples"), "x-example")["x-example"] = float64(9)
					want["x-bound"] = float64(7)
					r4ObjectAt(want, "checks")["x-check"] = "revised check"
					r4ObjectAt(want, "implemented_by")["x-binding"] = "revised binding"
					r4ObjectAt(want, "examples")["x-example"] = float64(9)
				}
				change := map[string]any{
					"format": "haft.change/1", "id": "chg-20260924-00000001",
					"title": "Round-trip extensions", "intent": "Preserve or explicitly edit extensions",
					"patches": []any{map[string]any{
						"base":       before["data"].(map[string]any)["exact_ref"],
						"operations": []any{map[string]any{"op": "MODIFIED", "claim_id": "rule", "claim": claim, "reason": "Review extension fields"}},
					}},
				}
				front, err := json.Marshal(change)
				if err != nil {
					t.Fatal(err)
				}
				r4Call(t, s, app.Request{Operation: "change", Action: "create", RequestID: "create-r4", Carrier: "---\n" + string(front) + "\n---\nKeep authored change prose.\n"}, "written")
				changeRef := "chg-20260924-00000001"
				if route == "typed-revision" {
					wire, err := json.Marshal(map[string]any{
						"format": app.Format, "operation": "change", "action": "update", "ref": changeRef,
						"request_id": "revise-r4", "revision": map[string]any{"reason": "Preserve the copied claim through typed JSON", "patches": change["patches"]},
					})
					if err != nil {
						t.Fatal(err)
					}
					var revision app.Request
					if err := json.Unmarshal(wire, &revision); err != nil {
						t.Fatal(err)
					}
					r4Call(t, s, revision, "written")
					changeRef = "chg-20260924-00000002"
				}
				preview := r4Call(t, s, app.Request{Operation: "change", Action: "preview", Ref: changeRef}, "ready")
				basis := preview["basis"].(map[string]any)
				r4Call(t, s, app.Request{Operation: "change", Action: "apply", Ref: basis["change_ref"].(string), RequestID: "apply-r4", ExpectedGeneration: basis["memory_generation"].(string), PreviewDigest: basis["preview_digest"].(string)}, "written")
				after := r4Call(t, s, app.Request{Operation: "recall", Ref: "spec:r4"}, "found")
				if got := r4Claim(after); !reflect.DeepEqual(got, want) {
					t.Errorf("JSON -> change -> readback changed extension structure or retained stale values:\ngot  %#v\nwant %#v", got, want)
				}
				// The exact predecessor remains retrievable with the original data.
				old := r4Call(t, s, app.Request{Operation: "recall", Ref: before["data"].(map[string]any)["exact_ref"].(string)}, "found")
				if r4Document(old)["raw"] != r4Document(before)["raw"] {
					t.Fatal("historical carrier bytes changed")
				}
				if got := r4Claim(old); !reflect.DeepEqual(got, r4ExpectedClaim(t)) {
					t.Errorf("historical claim fields changed: %#v", got)
				}
				record := r4Document(after)["record"].(map[string]any)
				if record["status"] != "proposed" || record["operator_confirmed"] == true || r4Document(after)["body"] != r4Document(before)["body"] {
					t.Fatal("change lost prose or inferred acceptance")
				}
			})
		}
	}
}

func r4Call(t *testing.T, s app.Service, q app.Request, kind string) map[string]any {
	t.Helper()
	q.Format = app.Format
	// Both directions pass through the public JSON wire, as CLI/MCP clients do.
	request, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := transport.DecodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	r := s.Execute(context.Background(), decoded)
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != kind {
		t.Fatalf("%s/%s: want %s, got %s", q.Operation, q.Action, kind, raw)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func r4Claim(result map[string]any) map[string]any {
	return r4ObjectAt(r4Document(result)["record"].(map[string]any), "claims")
}

func r4Document(result map[string]any) map[string]any {
	data := result["data"].(map[string]any)
	return data["resolution"].(map[string]any)["document"].(map[string]any)
}

func r4ObjectAt(parent map[string]any, field string) map[string]any {
	return parent[field].([]any)[0].(map[string]any)
}

func r4Extension(obj map[string]any, field string) map[string]any {
	if _, ok := obj[field]; ok {
		return obj
	}
	return obj["extra"].(map[string]any)
}

func r4ExpectedClaim(t *testing.T) map[string]any {
	t.Helper()
	var c map[string]any
	if err := json.Unmarshal([]byte(`{"id":"rule","kind":"law","text":"Preserve extension meaning.","x-bound":5,"x-tree":{"values":[1,{"two":true}]},"extra":{"literal":"keep this authored field"},"checks":[{"ref":"manual:review","covers":"Fixture inspection only","x-check":"original check"}],"implemented_by":[{"ref":"file:order.go","covers":"Fixture implementation only","x-binding":"original binding"}],"examples":[{"id":"example","given":"Original input","when":"Read and edit","then":"Preserved data","x-example":3}]}`), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

const r4Spec = `---
format: haft/1
id: spec-20260924-aabbccdd
kind: spec
title: R4 extension fixture
about: domain:Review.Extension
slug: r4
receiving_use: Inspect exact JSON authoring round trips
claims:
  - id: rule
    kind: law
    text: Preserve extension meaning.
    x-bound: 5
    x-tree: {values: [1, {two: true}]}
    extra: {literal: keep this authored field}
    checks:
      - ref: manual:review
        covers: Fixture inspection only
        x-check: original check
    implemented_by:
      - ref: file:order.go
        covers: Fixture implementation only
        x-binding: original binding
    examples:
      - id: example
        given: Original input
        when: Read and edit
        then: Preserved data
        x-example: 3
---
Keep the original specification prose.
`
