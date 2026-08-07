package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func assertMemoryQueryModes(t *testing.T, tool fpf.Tool) {
	t.Helper()
	schema, ok := tool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("haft_query schema = %T", tool.InputSchema)
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_query properties = %#v", schema["properties"])
	}
	actions := schemaEnumStrings(t, properties["action"], "haft_query.action")
	if !actions["memory"] {
		t.Fatalf("haft_query actions omit memory: %#v", actions)
	}
	request, ok := properties["memory_request"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"haft_query memory_request = %#v",
			properties["memory_request"],
		)
	}
	rawVariants, ok := request["oneOf"].([]interface{})
	if !ok {
		t.Fatalf(
			"haft_query memory_request.oneOf = %#v",
			request["oneOf"],
		)
	}
	modes := map[string]bool{}
	for _, rawVariant := range rawVariants {
		variant, variantOK := rawVariant.(map[string]interface{})
		if !variantOK {
			t.Fatalf("haft_query memory variant = %#v", rawVariant)
		}
		variantProperties, propertiesOK :=
			variant["properties"].(map[string]interface{})
		if !propertiesOK {
			t.Fatalf("haft_query memory variant properties = %#v", variant)
		}
		modeSchema, modeSchemaOK :=
			variantProperties["mode"].(map[string]interface{})
		if !modeSchemaOK {
			t.Fatalf(
				"haft_query memory mode schema = %#v",
				variantProperties["mode"],
			)
		}
		mode, modeOK := modeSchema["const"].(string)
		if !modeOK || mode == "" {
			t.Fatalf(
				"haft_query memory mode const = %#v",
				modeSchema["const"],
			)
		}
		modes[mode] = true
	}
	for _, mode := range []string{
		typedmemorywire.ActionNeighborhood,
		typedmemorywire.ActionRecall,
		typedmemorywire.ActionResolve,
	} {
		if !modes[mode] {
			t.Fatalf("haft_query modes omit %q: %#v", mode, modes)
		}
	}
}

func schemaEnumStrings(
	t *testing.T,
	raw interface{},
	label string,
) map[string]bool {
	t.Helper()
	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema = %#v", label, raw)
	}
	rawValues, ok := schema["enum"].([]interface{})
	if !ok {
		t.Fatalf("%s enum = %#v", label, schema["enum"])
	}
	values := make(map[string]bool, len(rawValues))
	for _, rawValue := range rawValues {
		value, valueOK := rawValue.(string)
		if !valueOK {
			t.Fatalf("%s value = %#v", label, rawValue)
		}
		values[value] = true
	}
	return values
}

func assertMemoryValidationSchemaRetained(
	t *testing.T,
	server *fpf.Server,
) {
	t.Helper()
	for _, tool := range server.ToolCatalog() {
		if tool.Name != "haft_memory" {
			continue
		}
		schema, ok := tool.InputSchema.(map[string]interface{})
		if !ok || schema["type"] != "object" {
			t.Fatalf("haft_memory validation schema = %#v", tool.InputSchema)
		}
		if _, widened := schema["oneOf"]; widened {
			t.Fatal("unready runtime widened haft_memory schema")
		}
		return
	}
	t.Fatal("haft_memory validation schema is absent")
}

func TestProjectMemoryReadHandlerRejectsCrossActionSupersetShape(
	t *testing.T,
) {
	t.Parallel()

	payload := json.RawMessage(`{
	  "contract_version":"haft.memory.v1",
	  "action":"resolve",
	  "basis":{"kind":"project_current"},
	  "query":"authorization",
	  "max_candidates":8,
	  "change_set":{"changes":[]}
	}`)

	result, err := (projectMemoryReadRuntime{}).ReadOnlyMCPHandler()(
		context.Background(),
		payload,
	)
	if err == nil {
		t.Fatal("strict read decoder accepted a validate-only field")
	}
	if result != "" {
		t.Fatalf("strict read decoder returned result %q", result)
	}
}
