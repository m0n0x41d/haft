package fpf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type toolsListTestPage struct {
	responseBytes []byte
	tools         []map[string]interface{}
	nextCursor    string
}

func mustCatalogServer() *Server {
	server := NewServer("test")
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	server.SetMemoryHandler(func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", nil
	})
	return server
}

func mustToolsListResponsePages(t *testing.T) []toolsListTestPage {
	t.Helper()

	return mustToolsListResponsePagesForServer(t, mustCatalogServer())
}

func mustToolsListResponsePagesForServer(
	t *testing.T,
	server *Server,
) []toolsListTestPage {
	t.Helper()

	return []toolsListTestPage{
		mustToolsListResponsePage(t, server, nil, 0),
	}
}

func mustToolsListResponseBytes(t *testing.T) []byte {
	t.Helper()

	pages := mustToolsListResponsePages(t)
	responseBytes := make([]byte, 0)
	for _, page := range pages {
		responseBytes = append(responseBytes, page.responseBytes...)
	}
	return responseBytes
}

func mustToolsListResponsePage(
	t *testing.T,
	server *Server,
	cursor *string,
	pageIndex int,
) toolsListTestPage {
	t.Helper()

	var params json.RawMessage
	if cursor != nil {
		encoded, err := json.Marshal(map[string]string{"cursor": *cursor})
		if err != nil {
			t.Fatal(err)
		}
		params = encoded
	}
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      fmt.Sprintf("req-schema-%d", pageIndex),
		Params:  params,
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = stdout
	}()

	os.Stdout = writer
	server.handleToolsList(request)

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	response := map[string]interface{}{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal tools/list response: %v\n%s", err, string(responseBytes))
	}
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing or wrong type: %#v", response["result"])
	}
	rawTools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("tools missing or wrong type: %#v", result["tools"])
	}
	tools := make([]map[string]interface{}, 0, len(rawTools))
	for _, rawTool := range rawTools {
		tool, ok := rawTool.(map[string]interface{})
		if !ok {
			t.Fatalf("tool entry has wrong type: %#v", rawTool)
		}
		tools = append(tools, tool)
	}
	nextCursor, _ := result["nextCursor"].(string)
	return toolsListTestPage{
		responseBytes: responseBytes,
		tools:         tools,
		nextCursor:    nextCursor,
	}
}

func mustListToolProperties(t *testing.T, toolName string) map[string]interface{} {
	t.Helper()

	inputSchema := mustListToolInputSchema(t, toolName)
	properties, ok := inputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties missing or wrong type: %#v", toolName, inputSchema["properties"])
	}

	return properties
}

func mustSchemaProperties(t *testing.T, raw interface{}, label string) map[string]interface{} {
	t.Helper()

	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema missing or wrong type: %#v", label, raw)
	}
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s properties missing or wrong type: %#v", label, schema["properties"])
	}
	return properties
}

func mustArrayItemProperties(t *testing.T, raw interface{}, label string) map[string]interface{} {
	t.Helper()

	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema missing or wrong type: %#v", label, raw)
	}
	items, ok := schema["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s items missing or wrong type: %#v", label, schema["items"])
	}
	properties, ok := items["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s item properties missing or wrong type: %#v", label, items["properties"])
	}
	return properties
}

func mustListToolInputSchema(t *testing.T, toolName string) map[string]interface{} {
	t.Helper()

	for _, page := range mustToolsListResponsePages(t) {
		for _, tool := range page.tools {
			if tool["name"] != toolName {
				continue
			}

			inputSchema, ok := tool["inputSchema"].(map[string]interface{})
			if !ok {
				t.Fatalf("%s inputSchema missing or wrong type: %#v", toolName, tool["inputSchema"])
			}

			return inputSchema
		}
	}

	t.Fatalf("%s tool schema not found", toolName)
	return nil
}

func TestHandleToolsList_ReturnsCompleteCatalogWithoutPagination(t *testing.T) {
	server := mustCatalogServer()
	pages := mustToolsListResponsePages(t)
	if len(pages) != 1 {
		t.Fatalf("tools/list returned %d pages, want one atomic catalog", len(pages))
	}
	page := pages[0]
	if page.nextCursor != "" {
		t.Fatalf("tools/list returned nextCursor %q, want no pagination", page.nextCursor)
	}
	if len(page.tools) != len(server.ToolCatalog()) {
		t.Fatalf(
			"tools/list returned %d tools, want complete catalog of %d",
			len(page.tools),
			len(server.ToolCatalog()),
		)
	}
}

func TestHandleToolsList_DoesNotInlineContractGenerationManifest(t *testing.T) {
	body := string(mustToolsListResponseBytes(t))
	for _, forbidden := range []string{
		"haft_interface_contract_generation_manifest",
		"generator_target_surfaces",
		"generator_target_fields",
		"generated_schema_fragments",
		"runtime_schema_audit",
		"runtime_schema_drift",
		"surface_policy",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("tools/list inlined contract generation manifest fragment %q", forbidden)
		}
	}
	// source_digest is intentionally not a forbidden bare fragment: the
	// strict typed-memory ContextSlice wire uses that exact field for an
	// environment selector. The manifest-specific surrounding keys above
	// remain forbidden.
}

func TestHandleToolsList_DoesNotInlineContractAuditRequiredCoverage(t *testing.T) {
	body := string(mustToolsListResponseBytes(t))
	for _, forbidden := range []string{
		"haft_interface_contract_audit",
		"schema_required_covered_surfaces",
		"schema_required_missing_surfaces",
		"schema_missing_required_fields",
		"required_coverage=",
		"mcp_required_fields",
		"missing_required_fields",
		"action_required_fields",
		"required_posture",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("tools/list inlined contract audit required-coverage fragment %q", forbidden)
		}
	}
}

func TestHandleToolsList_DoesNotInlineInterfaceOutputShapeFragments(t *testing.T) {
	body := string(mustToolsListResponseBytes(t))
	for _, forbidden := range []string{
		"bounded_reliance|advisory_only|blocked",
		"legacy_formality_projection_lossy|unversioned_formality_source_scale_missing|current_f0_f9_formality",
		"not_claim_truth",
		"not_publication",
		"planned_edition",
		"markdown_sync_back",
		"semantic_field_update",
		"relationship_update",
		"sql_edition_update_not_approval_rebaseline_evidence_gate_claim_truth_global_truth_or_prose_authority",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("tools/list inlined interface output-shape fragment %q", forbidden)
		}
	}
}

func TestHandleToolsList_MethodSchemaExposesPullAndClose(t *testing.T) {
	methodSchema := mustListToolProperties(t, "haft_method")

	action, ok := methodSchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_method action schema missing: %#v", methodSchema["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("haft_method action enum missing: %#v", action["enum"])
	}
	for _, want := range []string{"pull", "close", "show", "detail", "status", "catalog"} {
		if !schemaEnumContains(enum, want) {
			t.Fatalf("haft_method action enum = %#v, missing %q", enum, want)
		}
	}
	for _, key := range []string{"task", "declared_task_kind", "change_intent", "intended_files", "risk_signals", "pull_id", "gate_results", "verification", "waivers", "carry_through", "method_status"} {
		if _, ok := methodSchema[key]; !ok {
			t.Fatalf("haft_method schema missing %q", key)
		}
	}

	gateResults, ok := methodSchema["gate_results"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_results schema missing or wrong type: %#v", methodSchema["gate_results"])
	}
	gateItems, ok := gateResults["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_results.items missing or wrong type: %#v", gateResults["items"])
	}
	gateProperties, ok := gateItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("gate_results.items.properties missing or wrong type: %#v", gateItems["properties"])
	}
	for _, key := range []string{"gate_id", "status", "evidence_refs", "waiver_reason"} {
		if _, ok := gateProperties[key]; !ok {
			t.Fatalf("gate_results item schema missing %q", key)
		}
	}

	waivers, ok := methodSchema["waivers"].(map[string]interface{})
	if !ok {
		t.Fatalf("waivers schema missing or wrong type: %#v", methodSchema["waivers"])
	}
	waiverItems, ok := waivers["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("waivers.items missing or wrong type: %#v", waivers["items"])
	}
	waiverProperties, ok := waiverItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("waivers.items.properties missing or wrong type: %#v", waiverItems["properties"])
	}
	for _, key := range []string{"gate_id", "reason"} {
		if _, ok := waiverProperties[key]; !ok {
			t.Fatalf("waiver item schema missing %q", key)
		}
	}

	carryThrough, ok := methodSchema["carry_through"].(map[string]interface{})
	if !ok {
		t.Fatalf("carry_through schema missing or wrong type: %#v", methodSchema["carry_through"])
	}
	if carryThrough["type"] != "array" {
		t.Fatalf("carry_through type = %#v, want array", carryThrough["type"])
	}
	carryItems, ok := carryThrough["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("carry_through.items missing or wrong type: %#v", carryThrough["items"])
	}
	carryProperties, ok := carryItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("carry_through.items.properties missing or wrong type: %#v", carryItems["properties"])
	}
	for _, key := range []string{"source_ref", "source_item_ref", "acceptance_ref", "acceptance_ref_kind", "acceptance_ref_status", "disposition", "target_refs", "evidence_refs", "reason"} {
		if _, ok := carryProperties[key]; !ok {
			t.Fatalf("carry_through item schema missing %q", key)
		}
	}
	required, ok := carryItems["required"].([]interface{})
	if !ok {
		t.Fatalf("carry_through required fields missing or wrong type: %#v", carryItems["required"])
	}
	for _, key := range []string{"source_ref", "source_item_ref", "acceptance_ref"} {
		if !schemaEnumContains(required, key) {
			t.Fatalf("carry_through item required fields = %#v, missing %q", required, key)
		}
	}
	if _, ok := carryProperties["disposition"].(map[string]interface{}); !ok {
		t.Fatalf("carry_through disposition schema missing or wrong type: %#v", carryProperties["disposition"])
	}
	if _, ok := carryProperties["acceptance_ref_kind"].(map[string]interface{}); !ok {
		t.Fatalf("carry_through acceptance_ref_kind schema missing or wrong type: %#v", carryProperties["acceptance_ref_kind"])
	}
	if _, ok := carryProperties["acceptance_ref_status"].(map[string]interface{}); !ok {
		t.Fatalf("carry_through acceptance_ref_status schema missing or wrong type: %#v", carryProperties["acceptance_ref_status"])
	}
}

func TestHandleToolsList_RefreshSchemaIncludesReview(t *testing.T) {
	refreshSchema := mustListToolProperties(t, "haft_refresh")

	action, ok := refreshSchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_refresh action schema missing: %#v", refreshSchema["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("haft_refresh action enum missing: %#v", action["enum"])
	}
	for _, want := range []string{"scan", "plan", "review", "drain", "waive", "reopen", "supersede", "deprecate", "reconcile"} {
		if !schemaEnumContains(enum, want) {
			t.Fatalf("haft_refresh action enum = %#v, missing %q", enum, want)
		}
	}
	if _, ok := refreshSchema["dry_run"].(map[string]interface{}); !ok {
		t.Fatalf("haft_refresh schema missing dry_run field: %#v", refreshSchema)
	}
}

func TestHandleToolsList_QuerySchemaIncludesOperationalGate(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")
	properties := mustSchemaProperties(t, querySchema["operational_gate"], "operational_gate")
	for _, key := range []string{"schema_version", "gate_ref", "bearer_ref", "use_context", "rule", "evidence_refs", "expires_at", "reopen_condition"} {
		if _, ok := properties[key].(map[string]interface{}); !ok {
			t.Fatalf("operational_gate.%s schema missing or wrong type: %#v", key, properties[key])
		}
	}
}

func TestHandleToolsList_QuerySchemaIncludesContractAudit(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	action, ok := querySchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_query action schema missing: %#v", querySchema["action"])
	}
	enum, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("haft_query action enum missing: %#v", action["enum"])
	}
	if !schemaEnumContains(enum, "contract_audit") {
		t.Fatalf("haft_query action enum missing contract_audit: %#v", enum)
	}
	if !schemaEnumContains(enum, "contract_generation") {
		t.Fatalf("haft_query action enum missing contract_generation: %#v", enum)
	}
}

func TestHandleToolsList_QuerySchemaIncludesExactArtifactAliases(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")
	for _, field := range []string{"artifact_ref", "ref", "artifact_id"} {
		if _, ok := querySchema[field].(map[string]interface{}); !ok {
			t.Fatalf("haft_query schema missing %s: %#v", field, querySchema[field])
		}
	}

	piPath := filepath.Join("..", "..", "packages", "haft-pi", "extensions", "haft", "tools.ts")
	piBytes, err := os.ReadFile(piPath)
	if err != nil {
		t.Fatal(err)
	}
	piSchema := string(piBytes)
	for _, field := range []string{"artifact_ref:", "ref: OptStr()", "artifact_id: OptStr()"} {
		if !strings.Contains(piSchema, field) {
			t.Fatalf("Pi haft_query schema missing %q", field)
		}
	}
}

func TestHandleToolsList_QueryExactFieldsDeclareIdentifierNamespaces(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")
	for _, tc := range []struct {
		field    string
		required []string
	}{
		{
			field:    "identifier",
			required: []string{"FPF source identifier", "action=fpf", "wrong_identifier_namespace", "artifact_ref", "symbol"},
		},
		{
			field:    "artifact_ref",
			required: []string{"Canonical Haft artifact ID", "action=related", "wrong_identifier_namespace", "recovery_call"},
		},
		{
			field:    "symbol",
			required: []string{"action=node", "code symbol only", "wrong_identifier_namespace", "action=related", "artifact_ref=<id>"},
		},
	} {
		fieldSchema, ok := querySchema[tc.field].(map[string]interface{})
		if !ok {
			t.Fatalf("haft_query %s schema missing: %#v", tc.field, querySchema[tc.field])
		}
		description, _ := fieldSchema["description"].(string)
		for _, required := range tc.required {
			if !strings.Contains(description, required) {
				t.Fatalf("haft_query %s description missing %q: %q", tc.field, required, description)
			}
		}
	}
}

func TestPiHaftQueryActionsMirrorMCPEnum(t *testing.T) {
	querySchema := fullMemoryCatalogToolProperties(t, "haft_query")
	mcpActions := mustStringEnum(t, querySchema["action"], "haft_query.action")
	piActions := mustPiHaftQueryActions(t)

	for action := range mcpActions {
		if !piActions[action] {
			t.Fatalf("Pi haft_query action enum missing MCP action %q", action)
		}
	}
	for action := range piActions {
		if !mcpActions[action] {
			t.Fatalf("Pi haft_query action enum has non-MCP action %q", action)
		}
	}
}

func TestHandleToolsList_AdvertisesNativePiTools(t *testing.T) {
	for _, name := range []string{"haft_method", "haft_commission", "haft_spec_section"} {
		t.Run(name, func(t *testing.T) {
			if schema := mustListToolInputSchema(t, name); schema["type"] != "object" {
				t.Fatalf("%s input schema type = %#v, want object", name, schema["type"])
			}
		})
	}
}

func TestToolCatalog_AgentQualityDescriptionsExposeEffectsAndSourceFollowup(
	t *testing.T,
) {
	server := NewServer("test")
	server.SetV5Handler(
		func(
			_ context.Context,
			_ string,
			_ json.RawMessage,
		) (string, error) {
			return "", nil
		},
	)
	tools := make(map[string]Tool)
	for _, tool := range server.ToolCatalog() {
		tools[tool.Name] = tool
	}

	for _, toolName := range []string{"haft_note", "haft_problem", "haft_solution"} {
		full := tools[toolName].Description
		compact := compactToolDescription(toolName, full)
		for _, want := range []string{"Durably", "explicit save", "agent-inferred receiving use"} {
			if !strings.Contains(full, want) {
				t.Fatalf("%s full description missing %q: %q", toolName, want, full)
			}
			if !strings.Contains(compact, want) {
				t.Fatalf("%s compact description missing %q: %q", toolName, want, compact)
			}
		}
	}

	queryFull := tools["haft_query"].Description
	queryCompact := compactToolDescription("haft_query", queryFull)
	for _, text := range []string{queryFull, queryCompact} {
		for _, want := range []string{"concern", "exact", "inspect"} {
			if !strings.Contains(text, want) {
				t.Fatalf("haft_query description missing %q: %q", want, text)
			}
		}
	}

	refreshFull := tools["haft_refresh"].Description
	refreshCompact := compactToolDescription("haft_refresh", refreshFull)
	for _, text := range []string{refreshFull, refreshCompact} {
		if !strings.Contains(text, "lifecycle") ||
			!strings.Contains(text, "write") {
			t.Fatalf("haft_refresh description hides mutation: %q", text)
		}
	}

	methodCompact := compactToolDescription(
		"haft_method",
		tools["haft_method"].Description,
	)
	for _, want := range []string{"before non-mechanical edits", "close", "completion"} {
		if !strings.Contains(methodCompact, want) {
			t.Fatalf("haft_method compact description missing %q: %q", want, methodCompact)
		}
	}
}

func TestHandleToolsList_ExposesTaskContextAndNoteValidity(t *testing.T) {
	note := mustListToolProperties(t, "haft_note")
	for _, field := range []string{"task_context", "valid_until"} {
		if _, ok := note[field]; !ok {
			t.Fatalf("haft_note schema missing %q: %#v", field, note)
		}
	}
	for _, toolName := range []string{
		"haft_problem",
		"haft_solution",
		"haft_decision",
	} {
		properties := mustListToolProperties(t, toolName)
		if _, ok := properties["task_context"]; !ok {
			t.Fatalf("%s schema missing task_context: %#v", toolName, properties)
		}
	}
}

func TestToolCatalog_BindingDescriptionsNameHostReceiptVerifierBoundary(t *testing.T) {
	body := strings.Join([]string{
		toolCatalogActionDescription(t, "haft_commission"),
		toolCatalogActionDescription(t, "haft_spec_section"),
	}, "\n")
	for _, want := range []string{
		"host receipts require a registered kernel verifier",
		"operator_confirmation_required in default MCP cli-only mode",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("tools/list binding descriptions missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "not kernel-verifiable manual_cli authorization receipts") {
		t.Fatalf("tools/list still says only manual_cli receipts are kernel-verifiable:\n%s", body)
	}
}

func TestToolCatalog_SpecLifecycleDoesNotClaimFPFSourceCompatibility(t *testing.T) {
	tool := haftSpecSectionTool()
	body := strings.Join([]string{
		tool.Description,
		toolCatalogActionDescription(t, "haft_spec_section"),
		compactToolDescription(tool.Name, tool.Description),
	}, "\n")
	for _, want := range []string{
		"does not establish compatibility with a newer FPF source",
		"do not compare section meaning with a newer FPF source",
		"FPF source fit is separate",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("spec lifecycle descriptions missing %q:\n%s", want, body)
		}
	}
}

func TestToolCatalog_SpecLifecycleSeparatesProjectAndExactSectionReads(
	t *testing.T,
) {
	tool := haftSpecSectionTool()
	body := strings.Join([]string{
		tool.Description,
		toolCatalogActionDescription(t, "haft_spec_section"),
	}, "\n")
	for _, want := range []string{
		"project/scope-level",
		"reject section_id",
		"spec_trace",
		"spec_use",
		"not exact SpecSection lifecycle",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("spec lifecycle descriptions missing %q:\n%s", want, body)
		}
	}
}

func TestToolCatalog_SpecDraftActionsAreProfileIndependentAndNonBinding(
	t *testing.T,
) {
	tool := haftSpecSectionTool()
	body := strings.Join([]string{
		tool.Description,
		toolCatalogActionDescription(t, "haft_spec_section"),
	}, "\n")
	for _, want := range []string{
		"draft_contract",
		"profile-independent",
		"spec_validate",
		"does not establish applicability",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("spec draft descriptions missing %q:\n%s", want, body)
		}
	}
}

func toolCatalogActionDescription(t *testing.T, toolName string) string {
	t.Helper()

	var tool Tool
	switch toolName {
	case "haft_commission":
		tool = haftCommissionTool()
	case "haft_spec_section":
		tool = haftSpecSectionTool()
	default:
		t.Fatalf("unsupported un-compacted tool source %q", toolName)
	}

	schema, ok := tool.InputSchema.(map[string]interface{})
	if !ok {
		t.Fatalf("%s input schema has wrong type: %#v", toolName, tool.InputSchema)
	}
	properties := mustSchemaProperties(t, schema, toolName)
	action, ok := properties["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s action schema has wrong type: %#v", toolName, properties["action"])
	}
	description, ok := action["description"].(string)
	if !ok {
		t.Fatalf("%s action description missing or wrong type: %#v", toolName, action["description"])
	}
	return description
}

func TestHandleToolsList_CompareSchemaIncludesNarrativeFields(t *testing.T) {
	compareSchema := mustListToolProperties(t, "haft_solution")

	for _, key := range []string{"dominated_variants", "pareto_tradeoffs", "recommendation_rationale"} {
		if _, ok := compareSchema[key]; !ok {
			t.Fatalf("expected compare schema to expose %q", key)
		}
	}

	dominatedVariants, ok := compareSchema["dominated_variants"].(map[string]interface{})
	if !ok {
		t.Fatalf("dominated_variants schema missing or wrong type: %#v", compareSchema["dominated_variants"])
	}

	dominatedItems, ok := dominatedVariants["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("dominated_variants items missing or wrong type: %#v", dominatedVariants["items"])
	}

	dominatedProperties, ok := dominatedItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("dominated_variants properties missing or wrong type: %#v", dominatedItems["properties"])
	}

	for _, key := range []string{"variant", "dominated_by", "summary"} {
		if _, ok := dominatedProperties[key]; !ok {
			t.Fatalf("expected dominated_variants item schema to expose %q", key)
		}
	}

	paretoTradeoffs, ok := compareSchema["pareto_tradeoffs"].(map[string]interface{})
	if !ok {
		t.Fatalf("pareto_tradeoffs schema missing or wrong type: %#v", compareSchema["pareto_tradeoffs"])
	}

	paretoItems, ok := paretoTradeoffs["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("pareto_tradeoffs items missing or wrong type: %#v", paretoTradeoffs["items"])
	}

	paretoProperties, ok := paretoItems["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("pareto_tradeoffs properties missing or wrong type: %#v", paretoItems["properties"])
	}

	for _, key := range []string{"variant", "summary"} {
		if _, ok := paretoProperties[key]; !ok {
			t.Fatalf("expected pareto_tradeoffs item schema to expose %q", key)
		}
	}

	selectedRef, ok := compareSchema["selected_ref"].(map[string]interface{})
	if !ok {
		t.Fatalf("selected_ref schema missing or wrong type: %#v", compareSchema["selected_ref"])
	}
	if selectedRef["type"] != "string" {
		t.Fatalf("selected_ref type = %#v, want string", selectedRef["type"])
	}

	legacyRef, ok := compareSchema["legacy_recommendation_ref"].(map[string]interface{})
	if !ok {
		t.Fatalf("legacy_recommendation_ref schema missing or wrong type: %#v", compareSchema["legacy_recommendation_ref"])
	}

	legacyDescription, _ := legacyRef["description"].(string)
	if !strings.Contains(
		legacyDescription,
		"excluded from typed PortfolioComparison",
	) {
		t.Fatalf("unexpected legacy_recommendation_ref description: %q", legacyDescription)
	}

	selectedDescription, _ := selectedRef["description"].(string)
	if !strings.Contains(
		selectedDescription,
		"Excluded from typed PortfolioComparison",
	) {
		t.Fatalf("unexpected selected_ref description: %q", selectedDescription)
	}

	boundedContext, ok :=
		compareSchema["bounded_context_ref"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"bounded_context_ref schema missing or wrong type: %#v",
			compareSchema["bounded_context_ref"],
		)
	}
	boundedDescription, _ := boundedContext["description"].(string)
	if !strings.Contains(
		boundedDescription,
		"(explore/compare)",
	) {
		t.Fatalf(
			"unexpected bounded_context_ref description: %q",
			boundedDescription,
		)
	}
}

func TestHandleToolsList_CompareScoresSchemaExposesNestedMapShape(t *testing.T) {
	compareSchema := mustListToolProperties(t, "haft_solution")

	scores, ok := compareSchema["scores"].(map[string]interface{})
	if !ok {
		t.Fatalf("scores schema missing or wrong type: %#v", compareSchema["scores"])
	}

	variantScores, ok := scores["additionalProperties"].(map[string]interface{})
	if !ok {
		t.Fatalf("scores.additionalProperties missing or wrong type: %#v", scores["additionalProperties"])
	}
	if variantScores["type"] != "object" {
		t.Fatalf("scores additionalProperties type = %#v, want object", variantScores["type"])
	}

	dimensionScore, ok := variantScores["additionalProperties"].(map[string]interface{})
	if !ok {
		t.Fatalf("variant score additionalProperties missing or wrong type: %#v", variantScores["additionalProperties"])
	}
	if dimensionScore["type"] != "string" {
		t.Fatalf("dimension score type = %#v, want string", dimensionScore["type"])
	}
}

func schemaEnumContains(values []interface{}, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustStringEnum(t *testing.T, raw interface{}, label string) map[string]bool {
	t.Helper()

	schema, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s schema missing or wrong type: %#v", label, raw)
	}
	enum, ok := schema["enum"].([]interface{})
	if !ok {
		t.Fatalf("%s enum missing or wrong type: %#v", label, schema["enum"])
	}

	out := make(map[string]bool, len(enum))
	for _, rawValue := range enum {
		value, ok := rawValue.(string)
		if !ok {
			t.Fatalf("%s enum contains non-string value %#v", label, rawValue)
		}
		out[value] = true
	}
	return out
}

func mustPiHaftQueryActions(t *testing.T) map[string]bool {
	t.Helper()

	path := filepath.Join("..", "..", "packages", "haft-pi", "extensions", "haft", "tools.ts")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Pi tool schema mirror: %v", err)
	}
	content := string(raw)
	parametersIndex := strings.Index(content, "const haftQueryParameters = Type.Object({")
	if parametersIndex < 0 {
		t.Fatalf("Pi tool schema mirror missing haftQueryParameters in %s", path)
	}
	actionIndex := strings.Index(content[parametersIndex:], "action: enumOf(")
	if actionIndex < 0 {
		t.Fatalf("Pi haftQueryParameters missing action enum in %s", path)
	}
	actionBody := content[parametersIndex+actionIndex:]
	endIndex := strings.Index(actionBody, "\n  ),\n  artifact_ref:")
	if endIndex < 0 {
		t.Fatalf("Pi haft_query action enum has unexpected shape in %s", path)
	}

	matches := regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(actionBody[:endIndex], -1)
	if len(matches) == 0 {
		t.Fatalf("Pi haft_query action enum has no literals in %s", path)
	}
	out := make(map[string]bool, len(matches))
	for _, match := range matches {
		out[match[1]] = true
	}
	return out
}

func TestHandleToolsList_ProblemSchemaIncludesProfileFields(t *testing.T) {
	problemSchema := mustListToolProperties(t, "haft_problem")

	for _, key := range []string{
		"problem_type",
		"problem_profile",
		"source_kind",
		"why_now",
		"scope",
		"acceptance_probe",
		"freshness_disposition",
	} {
		if _, ok := problemSchema[key].(map[string]interface{}); !ok {
			t.Fatalf("%s schema missing or wrong type: %#v", key, problemSchema[key])
		}
	}

	problemType := problemSchema["problem_type"].(map[string]interface{})
	description, _ := problemType["description"].(string)
	if !strings.Contains(description, "optimization") {
		t.Fatalf("unexpected problem_type description: %q", description)
	}

	problemProfile := problemSchema["problem_profile"].(map[string]interface{})
	profileEnum, ok := problemProfile["enum"].([]interface{})
	if !ok {
		t.Fatalf("problem_profile enum missing or wrong type: %#v", problemProfile["enum"])
	}
	for _, want := range []string{"cue", "thin", "deep"} {
		if !schemaEnumContains(profileEnum, want) {
			t.Fatalf("problem_profile enum = %#v, missing %q", profileEnum, want)
		}
	}

	sourceKind := problemSchema["source_kind"].(map[string]interface{})
	sourceEnum, ok := sourceKind["enum"].([]interface{})
	if !ok {
		t.Fatalf("source_kind enum missing or wrong type: %#v", sourceKind["enum"])
	}
	for _, want := range []string{"observed_problem", "wish", "ticket", "chosen_method"} {
		if !schemaEnumContains(sourceEnum, want) {
			t.Fatalf("source_kind enum = %#v, missing %q", sourceEnum, want)
		}
	}
}

func TestHandleToolsList_DecisionSchemaMarksValidUntilForEvidence(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	validUntil, ok := decisionSchema["valid_until"].(map[string]interface{})
	if !ok {
		t.Fatalf("valid_until schema missing or wrong type: %#v", decisionSchema["valid_until"])
	}

	description, _ := validUntil["description"].(string)
	if description != "Expiry date" {
		t.Fatalf("unexpected valid_until description: %q", description)
	}

	for _, key := range []string{"predictions", "claim_refs", "claim_scope"} {
		if _, ok := decisionSchema[key]; !ok {
			t.Fatalf("expected decision schema to expose %q", key)
		}
	}
}

func TestHandleToolsList_DecisionSchemaExposesChoiceResult(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	choiceResult, ok := decisionSchema["choice_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("choice_result schema missing or wrong type: %#v", decisionSchema["choice_result"])
	}

	properties, ok := choiceResult["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("choice_result properties missing or wrong type: %#v", choiceResult["properties"])
	}

	nextMove, ok := properties["next_move"].(map[string]interface{})
	if !ok {
		t.Fatalf("choice_result.next_move missing or wrong type: %#v", properties["next_move"])
	}
	for _, key := range []string{"subject_ref", "option_set", "comparison_basis", "choice_rule", "variant_ref", "problem_refs", "portfolio_ref", "reason", "reversibility", "reopen_condition"} {
		if _, ok := properties[key].(map[string]interface{}); !ok {
			t.Fatalf("choice_result.%s missing or wrong type: %#v", key, properties[key])
		}
	}

	encoded, err := json.Marshal(nextMove["enum"])
	if err != nil {
		t.Fatalf("marshal next_move enum: %v", err)
	}
	for _, want := range []string{"choose_now", "reject_current_set", "probe_again", "reroute"} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("choice_result.next_move enum missing %q: %s", want, encoded)
		}
	}
}

func TestHandleToolsList_DecisionSchemaExposesTransformationRecord(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	transformationRecord, ok := decisionSchema["transformation_record"].(map[string]interface{})
	if !ok {
		t.Fatalf("transformation_record schema missing or wrong type: %#v", decisionSchema["transformation_record"])
	}

	properties, ok := transformationRecord["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("transformation_record properties missing or wrong type: %#v", transformationRecord["properties"])
	}

	for _, key := range []string{"transformed_entity", "initial_state", "post_state", "relation", "context", "window", "method_refs", "work_refs", "evidence_refs", "publication_refs"} {
		if _, ok := properties[key].(map[string]interface{}); !ok {
			t.Fatalf("transformation_record.%s schema missing or wrong type: %#v", key, properties[key])
		}
	}

}

func TestHandleToolsList_DecisionSchemaExposesScopeNestedShapes(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	implementationFootprint := mustSchemaProperties(t, decisionSchema["implementation_footprint"], "implementation_footprint")
	for _, key := range []string{"files", "commits", "work_refs"} {
		if _, ok := implementationFootprint[key].(map[string]interface{}); !ok {
			t.Fatalf("implementation_footprint.%s schema missing or wrong type: %#v", key, implementationFootprint[key])
		}
	}

	governanceTarget := mustArrayItemProperties(t, decisionSchema["governance_targets"], "governance_targets")
	for _, key := range []string{"kind", "ref", "binding_target"} {
		if _, ok := governanceTarget[key].(map[string]interface{}); !ok {
			t.Fatalf("governance_targets.%s schema missing or wrong type: %#v", key, governanceTarget[key])
		}
	}

	driftWatchTarget := mustArrayItemProperties(t, decisionSchema["drift_watch_targets"], "drift_watch_targets")
	for _, key := range []string{"target_ref", "trigger", "binding_target"} {
		if _, ok := driftWatchTarget[key].(map[string]interface{}); !ok {
			t.Fatalf("drift_watch_targets.%s schema missing or wrong type: %#v", key, driftWatchTarget[key])
		}
	}

	claim := mustArrayItemProperties(t, decisionSchema["claims"], "claims")
	for _, key := range []string{"id", "claim", "observable", "threshold", "lifecycle_status", "successor_ref", "retired_reason", "governance_target_refs"} {
		if _, ok := claim[key].(map[string]interface{}); !ok {
			t.Fatalf("claims.%s schema missing or wrong type: %#v", key, claim[key])
		}
	}
}

func TestHaftDecisionSchemaExposesTaskContext(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	taskContext, ok := decisionSchema["task_context"].(map[string]interface{})
	if !ok {
		t.Fatalf("task_context schema missing or wrong type: %#v", decisionSchema["task_context"])
	}

	description, _ := taskContext["description"].(string)
	if !strings.Contains(description, "Stable task/fork context") {
		t.Fatalf("unexpected task_context description: %q", description)
	}
}

func TestHandleToolsList_DecisionSchemaExposesTacticalSkipFields(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	for _, key := range []string{"_skips", "_skip"} {
		field, ok := decisionSchema[key].(map[string]interface{})
		if !ok {
			t.Fatalf("decision schema missing %q skip field", key)
		}
		if field["type"] != "array" {
			t.Fatalf("%s type = %v, want array", key, field["type"])
		}
		items, ok := field["items"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s items = %T, want map[string]interface{}", key, field["items"])
		}
		if items["type"] != "string" {
			t.Fatalf("%s items.type = %v, want string", key, items["type"])
		}
		description, _ := field["description"].(string)
		if !strings.Contains(description, "_skip_reason") {
			t.Fatalf("%s description should mention _skip_reason: %q", key, description)
		}
	}

	reason, ok := decisionSchema["_skip_reason"].(map[string]interface{})
	if !ok {
		t.Fatalf("decision schema missing _skip_reason field")
	}
	if reason["type"] != "string" {
		t.Fatalf("_skip_reason type = %v, want string", reason["type"])
	}
}

func TestHandleToolsList_DecisionSchemaRequiresCompletePredictions(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	predictions, ok := decisionSchema["predictions"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions schema missing or wrong type: %#v", decisionSchema["predictions"])
	}

	items, ok := predictions["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("prediction items schema missing or wrong type: %#v", predictions["items"])
	}

	required, ok := items["required"].([]interface{})
	if !ok {
		t.Fatalf("prediction required fields missing or wrong type: %#v", items["required"])
	}

	got := make([]string, 0, len(required))
	for _, item := range required {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("prediction required item has wrong type: %#v", item)
		}
		got = append(got, value)
	}

	want := []string{"claim", "observable", "threshold"}
	if len(got) != len(want) {
		t.Fatalf("prediction required fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("prediction required fields = %v, want %v", got, want)
		}
	}
}

func TestHandleToolsList_CommissionSchemaExposesRunnableClaimActions(t *testing.T) {
	commissionInputSchema := mustListToolInputSchema(t, "haft_commission")
	commissionSchema, ok := commissionInputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("commission properties missing or wrong type: %#v", commissionInputSchema["properties"])
	}

	action, ok := commissionSchema["action"].(map[string]interface{})
	if !ok {
		t.Fatalf("action schema missing or wrong type: %#v", commissionSchema["action"])
	}

	values, ok := action["enum"].([]interface{})
	if !ok {
		t.Fatalf("action enum missing or wrong type: %#v", action["enum"])
	}

	got := map[string]bool{}
	for _, value := range values {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("action enum value has wrong type: %#v", value)
		}
		got[name] = true
	}

	for _, want := range []string{"create", "list", "list_runnable", "show", "claim_for_preflight", "requeue", "cancel"} {
		if !got[want] {
			t.Fatalf("expected haft_commission action %q in schema enum %#v", want, values)
		}
	}
	// Per-action conditional requirements (commission_id required for
	// show/requeue/cancel; reason required for requeue/cancel) are
	// enforced at the handler boundary in internal/cli/serve_commission.go,
	// not in the MCP-advertised schema. Anthropic API rejects top-level
	// allOf/oneOf/anyOf in input_schema (validation error
	// "tools.N.custom.input_schema does not support oneOf, allOf, or
	// anyOf at the top level"), so the schema only declares "action" as
	// universally required and the handler returns an error when a
	// conditional field is missing for the given action.
	assertCommissionSchemaHasNoTopLevelCompositors(t, commissionInputSchema)

	override, ok := commissionSchema["spec_readiness_override"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec_readiness_override schema missing or wrong type: %#v", commissionSchema["spec_readiness_override"])
	}
	if override["type"] != "object" {
		t.Fatalf("spec_readiness_override type = %#v, want object", override["type"])
	}
}

// assertCommissionSchemaHasNoTopLevelCompositors guards the regression
// from 2026-04-28: a top-level allOf with conditional if/then blocks
// passed Go-side schema construction and Haft tests, but Anthropic API
// rejected it as input_schema validation error, taking the entire MCP
// server offline (`/mcp` reported `1 MCP server failed`). The handler
// at internal/cli/serve_commission.go enforces per-action requirements;
// the schema must remain free of top-level allOf / oneOf / anyOf.
func assertCommissionSchemaHasNoTopLevelCompositors(t *testing.T, schema map[string]interface{}) {
	t.Helper()
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if _, present := schema[key]; present {
			t.Fatalf("commission schema must not declare top-level %q (Anthropic API rejects it)", key)
		}
	}
}

// TestHandleToolsList_NoToolDeclaresTopLevelCompositors is the broader
// version of the same guard: iterates every advertised tool and asserts
// none of them declares top-level allOf / oneOf / anyOf in
// inputSchema. Anthropic API rejects all three at the top level
// regardless of which tool ships them, so ALL tools must comply.
func TestHandleToolsList_NoToolDeclaresTopLevelCompositors(t *testing.T) {
	pages := mustToolsListResponsePages(t)
	toolCount := 0
	for _, page := range pages {
		toolCount += len(page.tools)
	}
	if toolCount == 0 {
		t.Fatalf("no tools advertised")
	}

	banned := []string{"allOf", "oneOf", "anyOf"}
	for _, page := range pages {
		for _, tool := range page.tools {
			name, _ := tool["name"].(string)
			schema, ok := tool["inputSchema"].(map[string]any)
			if !ok {
				t.Fatalf("tool %q inputSchema missing", name)
			}
			for _, key := range banned {
				if _, present := schema[key]; present {
					t.Fatalf("tool %q declares top-level %q in inputSchema; Anthropic API rejects this and takes the whole MCP server offline", name, key)
				}
			}
		}
	}
}

func TestHandleToolsList_FPFQuerySchemaIncludesMode(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	mode, ok := querySchema["mode"].(map[string]interface{})
	if !ok {
		t.Fatalf("mode schema missing or wrong type: %#v", querySchema["mode"])
	}

	wantModes := []interface{}{
		"concern",
		"lookup",
		"inspect",
		"tactical",
		"standard",
		"deep",
	}
	if got := mode["type"]; got != "string" {
		t.Fatalf("FPF Query mode type = %#v, want string", got)
	}
	if got := mode["enum"]; !reflect.DeepEqual(got, wantModes) {
		t.Fatalf("shared haft_query modes = %#v, want non-memory modes %#v", got, wantModes)
	}

	action := querySchema["action"].(map[string]interface{})
	actions := action["enum"].([]interface{})
	for _, retired := range []interface{}{"pattern_use", "pattern_recall"} {
		if slices.Contains(actions, retired) {
			t.Fatalf("retired query action %q remains public: %#v", retired, actions)
		}
	}
	if !slices.Contains(actions, interface{}("fpf")) {
		t.Fatalf("fpf action missing: %#v", actions)
	}

	for _, field := range []string{
		"query",
		"identifier",
		"entity_of_concern",
		"known_context",
		"intended_use",
		"roles",
		"max_candidates_per_role",
		"max_total_candidates",
		"max_excerpt_characters",
		"max_relations_per_candidate",
		"max_candidates",
		"view",
		"trace_ref",
	} {
		if _, ok := querySchema[field]; !ok {
			t.Fatalf("FPF Query schema field %q missing", field)
		}
	}

	for _, field := range []string{"query", "identifier", "entity_of_concern", "intended_use"} {
		schema := querySchema[field].(map[string]interface{})
		if schema["type"] != "string" {
			t.Fatalf("FPF Query %s schema = %#v, want string", field, schema)
		}
	}
	traceRefSchema := querySchema["trace_ref"].(map[string]interface{})
	if traceRefSchema["type"] != "string" {
		t.Fatalf(
			"FPF Query trace_ref schema = %#v, want string",
			traceRefSchema,
		)
	}
	if _, exists := traceRefSchema["enum"]; exists {
		t.Fatalf(
			"shared haft_query trace_ref must remain action-specific, got global enum: %#v",
			traceRefSchema,
		)
	}
	viewSchema := querySchema["view"].(map[string]interface{})
	if viewSchema["type"] != "string" {
		t.Fatalf(
			"shared non-memory haft_query view lost its string type: %#v",
			viewSchema,
		)
	}
	view := querySchema["view"].(map[string]interface{})
	viewDescription, _ := view["description"].(string)
	for _, fragment := range []string{"action=fpf", "action=explore", "working (default)", "trace", "diagnostic"} {
		if !strings.Contains(viewDescription, fragment) {
			t.Fatalf("FPF Query view description missing %q: %q", fragment, viewDescription)
		}
	}
	memoryRequest, ok := querySchema["memory_request"].(map[string]interface{})
	if !ok {
		t.Fatalf(
			"haft_query memory_request schema missing: %#v",
			querySchema["memory_request"],
		)
	}
	variants, ok := memoryRequest["oneOf"].([]interface{})
	if !ok || len(variants) != 3 {
		t.Fatalf(
			"haft_query memory_request variants = %#v, want three",
			memoryRequest["oneOf"],
		)
	}
	traceRef := querySchema["trace_ref"].(map[string]interface{})
	traceDescription, _ := traceRef["description"].(string)
	for _, fragment := range []string{"action=fpf", "action=explore", "view=trace", "view=diagnostic", "opaque replay reference", "replay_mismatch before retrieval", "Explore index/request/result drift", "working view rejects"} {
		if !strings.Contains(traceDescription, fragment) {
			t.Fatalf("FPF Query trace_ref description missing %q: %q", fragment, traceDescription)
		}
	}
	for _, field := range []string{"known_context", "roles"} {
		schema := querySchema[field].(map[string]interface{})
		if schema["type"] != "array" {
			t.Fatalf("FPF Query %s schema = %#v, want string array", field, schema)
		}
		items := schema["items"].(map[string]interface{})
		if items["type"] != "string" {
			t.Fatalf("FPF Query %s items = %#v, want string", field, items)
		}
	}
	for _, field := range []string{"max_candidates_per_role", "max_total_candidates", "max_excerpt_characters", "max_relations_per_candidate"} {
		schema := querySchema[field].(map[string]interface{})
		if schema["type"] != "integer" || schema["minimum"] != float64(0) {
			t.Fatalf("FPF Query %s schema = %#v, want non-negative integer", field, schema)
		}
	}
	maxCandidates := querySchema["max_candidates"].(map[string]interface{})
	if maxCandidates["type"] != "integer" ||
		maxCandidates["minimum"] != float64(1) ||
		maxCandidates["maximum"] != float64(50) {
		t.Fatalf(
			"Explore max_candidates schema = %#v, want integer 1..50",
			maxCandidates,
		)
	}
}

func TestHandleInitialize_IncludesWorkflowInstructionsWhenConfigured(t *testing.T) {
	server := NewServer("test")
	server.SetInstructions("## Project Workflow\nDefaults:\n- mode: standard")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      "req-init",
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Stdout = stdout
	}()

	os.Stdout = writer
	server.handleInitialize(request)

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	responseBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	response := map[string]interface{}{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal initialize response: %v\n%s", err, string(responseBytes))
	}

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing or wrong type: %#v", response["result"])
	}
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("serverInfo missing or wrong type: %#v", result["serverInfo"])
	}
	if serverInfo["version"] != "test" {
		t.Fatalf("serverInfo version must come from the server constructor: %#v", serverInfo["version"])
	}

	instructions, _ := result["instructions"].(string)
	if !strings.Contains(instructions, "Project Workflow") {
		t.Fatalf("expected workflow instructions, got %#v", result["instructions"])
	}
}

// TestHandleToolsList_SolutionExposesParityPlan is the regression test for
// GitHub issue #62 — deep-mode haft_solution(action="compare") requires a
// structured parity plan, but the MCP-side schema in handleToolsList did not
// expose any parameter that accepts it. Without parity_plan in the schema,
// MCP clients (Claude Code, Cursor, Gemini CLI, Codex) cannot reach deep mode.
//
// The schema must expose parity_plan as an object with at minimum the four
// fields the deep-mode validator requires per FPF G.9:4.2.
func TestHandleToolsList_SolutionExposesParityPlan(t *testing.T) {
	solutionSchema := mustListToolProperties(t, "haft_solution")

	parityPlan, ok := solutionSchema["parity_plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan missing from haft_solution schema (issue #62); got: %#v", solutionSchema["parity_plan"])
	}
	if pType, _ := parityPlan["type"].(string); pType != "object" {
		t.Fatalf("parity_plan should be an object schema, got type=%q", pType)
	}
	props, ok := parityPlan["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan.properties missing or wrong type: %#v", parityPlan["properties"])
	}
	for _, key := range []string{"baseline_set", "window", "budget", "missing_data_policy"} {
		if _, ok := props[key]; !ok {
			t.Errorf("parity_plan must expose %q (required for deep-mode comparison per FPF G.9:4.2)", key)
		}
	}
}

// TestHandleToolsList_ProblemExposesParityPlan ensures characterize can
// declare a structured parity plan early, not just prose parity_rules.
// Same MCP gap as the haft_solution case but on the characterize entry point.
func TestHandleToolsList_ProblemExposesParityPlan(t *testing.T) {
	problemSchema := mustListToolProperties(t, "haft_problem")

	parityPlan, ok := problemSchema["parity_plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan missing from haft_problem schema (issue #62); got: %#v", problemSchema["parity_plan"])
	}
	props, ok := parityPlan["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("parity_plan.properties missing or wrong type: %#v", parityPlan["properties"])
	}
	for _, key := range []string{"baseline_set", "window", "budget", "missing_data_policy"} {
		if _, ok := props[key]; !ok {
			t.Errorf("parity_plan on haft_problem must expose %q", key)
		}
	}
}

func TestHandleToolsList_QuerySchemaIncludesProjectionView(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	view, ok := querySchema["view"].(map[string]interface{})
	if !ok {
		t.Fatalf("view schema missing or wrong type: %#v", querySchema["view"])
	}

	_ = view
}

func TestHandleToolsList_QuerySchemaIncludesCodeContextLane(t *testing.T) {
	querySchema := mustListToolProperties(t, "haft_query")

	lane, ok := querySchema["lane"].(map[string]interface{})
	if !ok {
		t.Fatalf("lane schema missing or wrong type: %#v", querySchema["lane"])
	}

	enum, ok := lane["enum"].([]interface{})
	if !ok {
		t.Fatalf("lane enum missing or wrong type: %#v", lane["enum"])
	}
	got := make(map[string]bool)
	for _, value := range enum {
		text, _ := value.(string)
		got[text] = true
	}
	for _, want := range []string{"index", "symbols", "decisions", "invariants", "notes", "problems", "portfolios", "all"} {
		if !got[want] {
			t.Fatalf("lane enum missing %q: %#v", want, enum)
		}
	}
}

func TestHandleToolsList_DecisionSchemaExposesCausalSupportBasis(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")

	basis, ok := decisionSchema["causal_support_basis"].(map[string]interface{})
	if !ok {
		t.Fatalf("haft_decision schema must expose causal_support_basis: %#v", decisionSchema["causal_support_basis"])
	}
	if basis["type"] != "string" {
		t.Fatalf("causal_support_basis type = %v, want string", basis["type"])
	}
	desc, _ := basis["description"].(string)
	if !strings.Contains(desc, "C.28") || !strings.Contains(desc, "simulation_only") {
		t.Fatalf("causal_support_basis description must cite C.28 and simulation_only: %q", desc)
	}

	predictions, ok := decisionSchema["predictions"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions schema missing: %#v", decisionSchema["predictions"])
	}
	items, ok := predictions["items"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions.items missing: %#v", predictions["items"])
	}
	props, ok := items["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("predictions.items.properties missing: %#v", items["properties"])
	}
	if _, ok := props["realizability"]; !ok {
		t.Fatalf("predictions[].realizability missing from haft_decision schema")
	}
}

func TestHandleToolsList_DecisionSchemaExposesDirectProblemStatement(t *testing.T) {
	decisionSchema := mustListToolProperties(t, "haft_decision")
	if _, ok := decisionSchema["problem_statement"]; !ok {
		t.Fatalf("haft_decision schema must expose problem_statement: %#v", decisionSchema)
	}
}
