package transport

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestR4InlineClaimExtensionsCrossClosedTransport(t *testing.T) {
	request := `{"format":"haft.api/1","operation":"change","action":"update","revision":{"reason":"Edit extensions","patches":[{"base":"exact-base","operations":[{"op":"MODIFIED","claim_id":"rule","claim":{"id":"rule","kind":"guard","text":"Scope","x-bound":7,"extra":{"literal":"keep"},"checks":[{"ref":"manual:review","covers":"Review","x-check":true}],"implemented_by":[{"ref":"file:order.go","covers":"Body","x-binding":{"nested":[1,2]}}],"examples":[{"id":"case","text":"Scenario","x-example":9}],"evidence_inputs":[{"ref":"exact-use","applicability":"Scope","x-input":"keep"}]},"reason":"Explicit edit"}]}]}}`
	q, err := DecodeRequest([]byte(request))
	if err != nil {
		t.Fatal(err)
	}
	c := q.Revision.Patches[0].Operations[0].Claim
	if c.Extra["x-bound"] != 7 || c.Checks[0].Extra["x-check"] != true || c.ImplementedBy[0].Extra["x-binding"] == nil || c.Examples[0].Extra["x-example"] != 9 || c.EvidenceInputs[0].Extra["x-input"] != "keep" {
		t.Fatalf("transport lost inline claim data: %#v", c)
	}
	for _, mutation := range []struct{ old, new, want string }{
		{`"action":"update"`, `"action":"update","unknown_control":true`, "unknown_field"},
		{`"reason":"Edit extensions"`, `"reason":"Edit extensions","unknown_control":true`, "unknown_field"},
		{`"base":"exact-base"`, `"base":"exact-base","unknown_control":true`, "unknown_field"},
		{`"op":"MODIFIED"`, `"op":"MODIFIED","unknown_control":true`, "unknown_field"},
		{`"x-bound":7`, `"ID":"hidden"`, "unknown_field"},
		{`"x-check":true`, `"Ref":"hidden"`, "unknown_field"},
		{`"x-example":9`, `"Then":"hidden"`, "unknown_field"},
		{`"x-input":"keep"`, `"Applicability":"hidden"`, "unknown_field"},
		{`"x-bound":7`, `"x-bound":7,"x-bound":8`, "duplicate_field"},
		{`"nested":[1,2]`, `"nested":1,"nested":2`, "duplicate_field"},
		{`"text":"Scope"`, `"text":12`, "invalid_json"},
	} {
		raw := strings.Replace(request, mutation.old, mutation.new, 1)
		if _, err := DecodeRequest([]byte(raw)); err == nil || !strings.Contains(err.Error(), mutation.want) {
			t.Errorf("%s: wanted %s, got %v", mutation.new, mutation.want, err)
		}
	}
	// A re-encoded typed request is accepted by exactly the same public decoder.
	wire, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRequest(wire); err != nil {
		t.Fatal(err)
	}
}

func TestR4SchemaOpensOnlyClaimExtensionObjects(t *testing.T) {
	root := RequestSchema()
	property := func(v map[string]any, key string) map[string]any {
		return v["properties"].(map[string]any)[key].(map[string]any)
	}
	items := func(v map[string]any) map[string]any { return v["items"].(map[string]any) }
	revision := property(root, "revision")
	patch := items(property(revision, "patches"))
	operation := items(property(patch, "operations"))
	claim := property(operation, "claim")
	for _, object := range []map[string]any{root, revision, patch, operation} {
		if object["additionalProperties"] != false {
			t.Fatal("control object schema opened")
		}
	}
	for _, object := range []map[string]any{claim, items(property(claim, "checks")), items(property(claim, "implemented_by")), items(property(claim, "examples")), items(property(claim, "evidence_inputs"))} {
		if object["additionalProperties"] != true {
			t.Fatal("inline extension schema remains closed")
		}
	}
}
