package fpf

import (
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
	properties := mustListToolProperties(t, toolName)
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
