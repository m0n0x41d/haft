package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/core/app"
)

func TestNoClobberPublicationHasFailureExitAndReadableResult(t *testing.T) {
	root := t.TempDir()
	q := app.Request{Format: app.Format, Operation: "remember", Action: "terms", RequestID: "first", Carrier: "---\nformat: haft.terms/1\nterms:\n  - id: Fixture.Amount\n    definition: A synthetic integer amount.\n---\n"}
	first := (app.Service{Root: root}).Execute(context.Background(), q)
	if first.Kind != "written" {
		t.Fatal(first)
	}
	q.RequestID = "different"
	raw, _ := json.Marshal(q)
	exit, result, _ := invoke(t, []string{"api", "--input", "-", "--root", root}, string(raw))
	if exit != 1 || result["result_kind"] != "path_conflict" {
		t.Fatal(exit, result)
	}
}
