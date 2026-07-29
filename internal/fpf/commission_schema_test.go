package fpf

import (
	"strings"
	"testing"
)

func TestHaftCommissionSchemaExposesScopeButNotProjectRootAuthority(t *testing.T) {
	tool := haftCommissionTool()
	schema, ok := tool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("commission schema = %T", tool.InputSchema)
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("commission properties = %T", schema["properties"])
	}
	if _, exists := properties["project_root"]; exists {
		t.Fatal("model-facing commission schema exposes server-owned project_root")
	}
	scope, ok := properties["scope_id"].(map[string]string)
	if !ok {
		t.Fatalf("scope_id schema = %T", properties["scope_id"])
	}
	if scope["type"] != "string" ||
		!strings.Contains(scope["description"], "Exact admitted ScopeID") {
		t.Fatalf("scope_id schema = %#v", scope)
	}
}
