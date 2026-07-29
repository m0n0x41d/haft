package fpf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPiToolActionEnumsMirrorMCPToolCatalog(t *testing.T) {
	source := readPiToolSource(t)
	cases := map[string]string{
		"haft_query":   "haftQueryParameters",
		"haft_refresh": "haftRefreshParameters",
	}

	for toolName, constName := range cases {
		t.Run(toolName, func(t *testing.T) {
			mcpActions := mcpToolActionEnum(t, toolName)
			piActions := piToolActionEnum(t, source, constName)

			if got, want := sortedSetValues(piActions), sortedSetValues(mcpActions); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf(
					"%s Pi action enum drifted from MCP ToolCatalog\nmissing_in_pi=%v\nextra_in_pi=%v",
					toolName,
					sortedSetValues(setDifference(mcpActions, piActions)),
					sortedSetValues(setDifference(piActions, mcpActions)),
				)
			}
		})
	}
}

func TestPiMemoryActionVariantsMirrorMCPToolCatalog(t *testing.T) {
	source := readPiToolSource(t)
	mcpActions := mcpMemoryRequestActionEnum(t)
	piActions := map[string]struct{}{}
	for _, constName := range []string{
		"memoryValidationRequestSchema",
		"memoryAdmissionRequestSchema",
	} {
		for action := range piToolActionEnum(t, source, constName) {
			piActions[action] = struct{}{}
		}
	}

	if got, want := sortedSetValues(piActions), sortedSetValues(mcpActions); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf(
			"haft_memory Pi nested action variants drifted from MCP ToolCatalog\nmissing_in_pi=%v\nextra_in_pi=%v",
			sortedSetValues(setDifference(mcpActions, piActions)),
			sortedSetValues(setDifference(piActions, mcpActions)),
		)
	}
}

func TestPiMemorySchemaMirrorsV2AssertionBoundary(t *testing.T) {
	t.Parallel()

	source := readPiToolSource(t)
	properties := fullMemoryCatalogToolProperties(t, "haft_memory")
	kernelSchema, err := json.Marshal(properties)
	if err != nil {
		t.Fatal(err)
	}

	for _, fragment := range []string{
		`"haft.memory.v2"`,
		`"assert_relation"`,
		`"affirms_obtaining"`,
		`"denies_obtaining"`,
		`"obtaining_unknown"`,
	} {
		if !strings.Contains(string(kernelSchema), fragment) {
			t.Fatalf("kernel haft_memory schema omits %s", fragment)
		}
	}
	if strings.Contains(string(kernelSchema), `"instantiate_relation"`) {
		t.Fatal("kernel haft_memory schema reopened frozen v1 relation creation")
	}

	for _, fragment := range []string{
		`contract_version: enumOf("haft.memory.v2")`,
		`request: Type.Union([`,
		`basis: memoryValidationBasisSchema`,
		`basis: memoryAdmissionBasisSchema`,
		`change_set: memoryChangeSetSchema`,
		`kind: Type.Literal("assert_relation")`,
		`kind: Type.Literal("affirms_obtaining")`,
		`kind: Type.Literal("denies_obtaining")`,
		`kind: Type.Literal("obtaining_unknown")`,
	} {
		if !strings.Contains(source, fragment) {
			t.Fatalf("Pi haft_memory schema omits %q", fragment)
		}
	}
	if strings.Contains(source, `kind: Type.Literal("instantiate_relation")`) {
		t.Fatal("Pi haft_memory schema reopened frozen v1 relation creation")
	}
}

func mcpMemoryRequestActionEnum(t *testing.T) map[string]struct{} {
	t.Helper()
	properties := fullMemoryCatalogToolProperties(t, "haft_memory")
	request, ok := properties["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_memory request schema missing: %#v", properties["request"])
	}
	variants, ok := request["oneOf"].([]interface{})
	if !ok {
		t.Fatalf("haft_memory request variants missing: %#v", request["oneOf"])
	}
	actions := map[string]struct{}{}
	for index, rawVariant := range variants {
		variant, variantOK := rawVariant.(map[string]interface{})
		if !variantOK {
			t.Fatalf("haft_memory request variant %d = %T", index, rawVariant)
		}
		variantProperties, propertiesOK :=
			variant["properties"].(map[string]interface{})
		if !propertiesOK {
			t.Fatalf(
				"haft_memory request variant %d properties missing: %#v",
				index,
				variant,
			)
		}
		action, actionOK :=
			variantProperties["action"].(map[string]interface{})
		if !actionOK {
			t.Fatalf(
				"haft_memory request variant %d action missing: %#v",
				index,
				variantProperties["action"],
			)
		}
		values, valuesOK := action["enum"].([]interface{})
		if !valuesOK {
			t.Fatalf(
				"haft_memory request variant %d action enum missing: %#v",
				index,
				action["enum"],
			)
		}
		for _, rawValue := range values {
			value, valueOK := rawValue.(string)
			if !valueOK {
				t.Fatalf(
					"haft_memory request action contains non-string value: %#v",
					rawValue,
				)
			}
			actions[value] = struct{}{}
		}
	}
	return actions
}

func readPiToolSource(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "packages", "haft-pi", "extensions", "haft", "tools.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Pi tool schema mirror %s: %v", path, err)
	}
	return string(data)
}

func mcpToolActionEnum(t *testing.T, toolName string) map[string]struct{} {
	t.Helper()
	schema := fullMemoryCatalogToolSchema(t, toolName)
	if toolName == "haft_memory" {
		variants := memoryRequestVariantsByAction(
			t,
			schema,
			"haft_memory Pi parity source",
		)
		out := make(map[string]struct{}, len(variants))
		for action := range variants {
			out[action] = struct{}{}
		}
		return out
	}
	properties := mustSchemaProperties(t, schema, toolName)
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s action schema missing: %#v", toolName, properties["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("%s action enum missing or wrong type: %#v", toolName, action["enum"])
	}

	out := map[string]struct{}{}
	for _, raw := range enum {
		value, ok := raw.(string)
		if !ok {
			t.Fatalf("%s action enum contains non-string value: %#v", toolName, raw)
		}
		out[value] = struct{}{}
	}
	return out
}

func fullMemoryCatalogToolProperties(
	t *testing.T,
	toolName string,
) map[string]interface{} {
	t.Helper()
	schema := fullMemoryCatalogToolSchema(t, toolName)
	return mustSchemaProperties(t, schema, toolName)
}

func fullMemoryCatalogToolSchema(
	t *testing.T,
	toolName string,
) map[string]interface{} {
	t.Helper()
	server := NewServer("test")
	server.SetV5Handler(
		func(context.Context, string, json.RawMessage) (string, error) {
			return "", nil
		},
	)
	server.SetMemoryFullHandler(
		func(context.Context, json.RawMessage) (string, error) {
			return "", nil
		},
	)
	server.SetMemoryReadHandler(
		func(context.Context, json.RawMessage) (string, error) {
			return "", nil
		},
	)
	tool := catalogTool(t, server.ToolCatalog(), toolName)
	schema, ok := tool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("%s input schema = %T", toolName, tool.InputSchema)
	}
	return schema
}

func piToolActionEnum(t *testing.T, source string, constName string) map[string]struct{} {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(constName) + `\s*=\s*Type\.Object\(\{.*?action:\s*enumOf\((.*?)\)`)
	matches := pattern.FindStringSubmatch(source)
	if len(matches) != 2 {
		t.Fatalf("Pi schema mirror %s action enum not found", constName)
	}

	valuePattern := regexp.MustCompile(`"([^"]+)"`)
	out := map[string]struct{}{}
	for _, match := range valuePattern.FindAllStringSubmatch(matches[1], -1) {
		out[match[1]] = struct{}{}
	}
	if len(out) == 0 {
		t.Fatalf("Pi schema mirror %s action enum is empty", constName)
	}
	return out
}

func setDifference(left map[string]struct{}, right map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for value := range left {
		if _, ok := right[value]; !ok {
			out[value] = struct{}{}
		}
	}
	return out
}

func sortedSetValues(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
