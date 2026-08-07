package cli

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

func TestInterfaceContractAuditUnderstandsClosedMemoryActionUnion(
	t *testing.T,
) {
	t.Parallel()

	toolSchemas := fullMemoryInterfaceAuditToolSchemas(t)
	capabilities := fullMemoryInterfaceAuditCapabilities()

	report := buildInterfaceContractAuditReportWithToolSchemas(
		capabilities,
		toolSchemas,
	)
	if report.Summary.SchemaCoveredSurfaces != len(capabilities) {
		t.Fatalf(
			"closed memory union covered=%d, want %d: %#v",
			report.Summary.SchemaCoveredSurfaces,
			len(capabilities),
			report,
		)
	}
	if report.Summary.SchemaMissingSurfaces != 0 {
		t.Fatalf("closed memory union has missing surfaces: %#v", report)
	}

	for _, capability := range capabilities {
		surface, ok := findContractAuditSurface(report, capability.ID)
		if !ok {
			t.Fatalf("%s missing from audit", capability.ID)
		}
		if surface.SchemaCoverage.Status != "covered" {
			t.Fatalf(
				"%s schema coverage = %#v",
				capability.ID,
				surface.SchemaCoverage,
			)
		}
		if surface.SchemaCoverage.AdditionalPropertiesPosture !=
			"closed_action_branch" {
			t.Fatalf(
				"%s closure posture = %#v",
				capability.ID,
				surface.SchemaCoverage,
			)
		}
		if !strings.HasPrefix(
			surface.SchemaCoverage.ActionSchemaPosture,
			"exact_",
		) {
			t.Fatalf(
				"%s discriminator posture = %#v",
				capability.ID,
				surface.SchemaCoverage,
			)
		}
		for _, required := range capability.InputContract.RequiredFields {
			if !slices.Contains(
				surface.SchemaCoverage.MCPRequiredFields,
				required,
			) {
				t.Fatalf(
					"%s branch required fields %v omit %q",
					capability.ID,
					surface.SchemaCoverage.MCPRequiredFields,
					required,
				)
			}
		}
	}

	resolve, _ := findContractAuditSurface(report, "memory.resolve")
	if slices.Contains(resolve.SchemaCoverage.MCPRequiredFields, "entity_ref") {
		t.Fatalf(
			"resolve inherited another action's required field: %#v",
			resolve.SchemaCoverage,
		)
	}
	resolveCapability := capabilities[2]
	resolveSchema := interfaceContractGeneratedSchemaFor(
		resolve,
		topLevelInterfaceContractFields(resolveCapability.InputContract),
		interfaceContractAuditExpectedMCPRequiredFieldsForSchema(
			resolveCapability,
			toolSchemas["haft_memory"],
		),
		toolSchemas["haft_memory"],
	)
	if resolveSchema["additionalProperties"] != false {
		t.Fatalf("generated resolve branch reopened the union: %#v", resolveSchema)
	}
	resolveProperties, ok :=
		resolveSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("generated resolve properties missing: %#v", resolveSchema)
	}
	if _, leaked := resolveProperties["entity_ref"]; leaked {
		t.Fatalf(
			"generated resolve branch flattened neighborhood fields: %#v",
			resolveProperties,
		)
	}
}

func TestInterfaceContractAuditReadsNestedMemoryMCPEnvelopeAsExactBranches(
	t *testing.T,
) {
	t.Parallel()

	// haft_memory is the validate/admit boundary. Project-memory reads are
	// deliberately exposed through the closed
	// haft_query(action="memory", memory_request={...}) branches.
	// and are covered by the read-surface schema tests.
	capabilities := fullMemoryInterfaceAuditCapabilities()[:2]
	report := buildInterfaceContractAuditReportWithToolSchemas(
		capabilities,
		installedFullMemoryInterfaceAuditToolSchemas(t),
	)
	if report.Summary.SchemaMissingSurfaces != 0 ||
		report.Summary.ValidatedMCPMirrors != len(capabilities) {
		t.Fatalf("nested memory host schema audit = %#v", report)
	}
	for _, capability := range capabilities {
		surface, ok := findContractAuditSurface(report, capability.ID)
		if !ok || surface.SchemaCoverage.Status != "covered" {
			t.Fatalf("%s nested schema coverage = %#v", capability.ID, surface)
		}
		if surface.SchemaCoverage.ActionSchemaPosture !=
			"exact_singleton_enum_discriminator" ||
			surface.SchemaCoverage.AdditionalPropertiesPosture !=
				"closed_action_branch" {
			t.Fatalf("%s nested schema posture = %#v", capability.ID, surface)
		}
		for _, required := range capability.InputContract.RequiredFields {
			if !slices.Contains(
				surface.SchemaCoverage.MCPRequiredFields,
				required,
			) {
				t.Fatalf(
					"%s nested branch omits %q: %#v",
					capability.ID,
					required,
					surface.SchemaCoverage,
				)
			}
		}
	}
}

func TestInterfaceContractAuditRejectsCatalogActionAbsentFromHandlerSurface(
	t *testing.T,
) {
	t.Parallel()

	toolSchemas := fullMemoryInterfaceAuditToolSchemas(t)
	overstated := memoryInterfaceAuditCapability(
		"memory.delete",
		"delete",
		[]string{"contract_version", "action", "entity_ref"},
		nil,
	)
	capabilities := append(
		fullMemoryInterfaceAuditCapabilities(),
		overstated,
	)
	report := buildInterfaceContractAuditReportWithToolSchemas(
		capabilities,
		toolSchemas,
	)
	surface, ok := findContractAuditSurface(report, overstated.ID)
	if !ok {
		t.Fatal("overstated memory capability missing from audit")
	}
	if surface.SchemaCoverage.Status != "missing_mcp_action" {
		t.Fatalf(
			"overstated action coverage = %#v",
			surface.SchemaCoverage,
		)
	}
	if surface.SchemaCoverage.ActionSchemaPosture !=
		"catalog_action_not_advertised" {
		t.Fatalf(
			"overstated action posture = %#v",
			surface.SchemaCoverage,
		)
	}
	if surface.HostSchemaPosture != "unvalidated_host_schema_fragment" {
		t.Fatalf("overstated host posture = %#v", surface)
	}
	if report.Summary.SchemaMissingSurfaces != 1 ||
		report.Summary.ValidatedMCPMirrors != len(capabilities)-1 {
		t.Fatalf("overstated action summary = %#v", report.Summary)
	}
}

func TestInterfaceContractAuditRejectsHandlerActionsAbsentFromCatalog(
	t *testing.T,
) {
	t.Parallel()

	toolSchemas := installedFullMemoryInterfaceAuditToolSchemas(t)
	validateOnlyCatalog := []interfaceCapability{
		fullMemoryInterfaceAuditCapabilities()[0],
	}
	report := buildInterfaceContractAuditReportWithToolSchemas(
		validateOnlyCatalog,
		toolSchemas,
	)
	validate, ok := findContractAuditSurface(report, "memory.validate")
	if !ok {
		t.Fatal("memory.validate missing from overexposed-handler audit")
	}
	if validate.SchemaCoverage.Status !=
		"handler_action_surface_overexposed" {
		t.Fatalf(
			"overexposed handler coverage = %#v",
			validate.SchemaCoverage,
		)
	}
	for _, action := range []string{"admit"} {
		if !slices.Contains(
			validate.SchemaCoverage.ActionUnionDiagnostics,
			"handler_action_not_cataloged:"+action,
		) {
			t.Fatalf(
				"overexposed handler diagnostics omit %q: %#v",
				action,
				validate.SchemaCoverage,
			)
		}
	}
	if report.Summary.SchemaMissingSurfaces != 1 ||
		report.Summary.ValidatedMCPMirrors != 0 {
		t.Fatalf("overexposed handler summary = %#v", report.Summary)
	}
}

func TestInterfaceContractAuditRejectsMalformedOrOpenActionBranches(
	t *testing.T,
) {
	t.Parallel()

	closedBranch := func(actionSchema map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"action": actionSchema,
				"value":  map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"action", "value"},
		}
	}
	exactConst := func(action string) map[string]interface{} {
		return map[string]interface{}{
			"type":  "string",
			"const": action,
		}
	}

	testCases := []struct {
		name     string
		branches []interface{}
		status   string
	}{
		{
			name: "multi value discriminator",
			branches: []interface{}{
				closedBranch(map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"resolve", "recall"},
				}),
			},
			status: "malformed_action_union",
		},
		{
			name: "absent discriminator",
			branches: []interface{}{
				closedBranch(map[string]interface{}{"type": "string"}),
			},
			status: "malformed_action_union",
		},
		{
			name: "non string discriminator",
			branches: []interface{}{
				closedBranch(map[string]interface{}{
					"type":  "integer",
					"const": 1,
				}),
			},
			status: "malformed_action_union",
		},
		{
			name: "mixed type singleton looking enum",
			branches: []interface{}{
				closedBranch(map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"resolve", 1},
				}),
			},
			status: "malformed_action_union",
		},
		{
			name: "duplicate discriminator",
			branches: []interface{}{
				closedBranch(exactConst("resolve")),
				closedBranch(exactConst("resolve")),
			},
			status: "malformed_action_union",
		},
		{
			name: "non object branch",
			branches: []interface{}{
				map[string]interface{}{
					"type":                 "array",
					"additionalProperties": false,
					"properties": map[string]interface{}{
						"action": exactConst("resolve"),
						"value":  map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"action", "value"},
				},
			},
			status: "malformed_action_union",
		},
		{
			name: "open branch",
			branches: []interface{}{
				map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
					"properties": map[string]interface{}{
						"action": exactConst("resolve"),
						"value":  map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"action", "value"},
				},
			},
			status: "open_action_branch",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			schemas := interfaceContractAuditToolSchemasFromCatalog(
				[]fpf.Tool{
					{
						Name: "haft_memory",
						InputSchema: map[string]interface{}{
							"oneOf": testCase.branches,
						},
					},
				},
			)
			capability := memoryInterfaceAuditCapability(
				"memory.resolve",
				"resolve",
				[]string{"action", "value"},
				nil,
			)
			surface := buildInterfaceContractAuditSurface(
				capability,
				schemas,
				map[string]map[string]bool{},
			)
			if surface.SchemaCoverage.Status != testCase.status {
				t.Fatalf(
					"coverage status = %q, want %q: %#v",
					surface.SchemaCoverage.Status,
					testCase.status,
					surface.SchemaCoverage,
				)
			}
			if surface.HostSchemaPosture !=
				"unvalidated_host_schema_fragment" {
				t.Fatalf("host posture = %#v", surface)
			}
		})
	}
}

func TestInterfaceContractAuditAcceptsConstActionDiscriminator(
	t *testing.T,
) {
	t.Parallel()

	schemas := interfaceContractAuditToolSchemasFromCatalog(
		[]fpf.Tool{
			{
				Name: "haft_memory",
				InputSchema: map[string]interface{}{
					"oneOf": []interface{}{
						map[string]interface{}{
							"type":                 "object",
							"additionalProperties": false,
							"properties": map[string]interface{}{
								"action": map[string]interface{}{
									"type":  "string",
									"const": "resolve",
								},
								"value": map[string]interface{}{"type": "string"},
							},
							"required": []interface{}{"action", "value"},
						},
					},
				},
			},
		},
	)
	capability := memoryInterfaceAuditCapability(
		"memory.resolve",
		"resolve",
		[]string{"action", "value"},
		nil,
	)
	surface := buildInterfaceContractAuditSurface(
		capability,
		schemas,
		map[string]map[string]bool{},
	)
	if surface.SchemaCoverage.Status != "covered" ||
		surface.SchemaCoverage.ActionSchemaPosture !=
			"exact_const_discriminator" {
		t.Fatalf("const discriminator coverage = %#v", surface.SchemaCoverage)
	}
}

func TestPublicInterfaceCatalogSeparatesMemoryMutationAndReadSurfaces(
	t *testing.T,
) {
	t.Parallel()

	memoryCapabilities := make([]string, 0)
	for _, capability := range haftInterfaceCatalog() {
		if strings.HasPrefix(capability.ID, "memory.") {
			memoryCapabilities = append(memoryCapabilities, capability.ID)
		}
	}
	wantCapabilities := []string{
		"memory.validate",
		"memory.admit",
		"memory.backfill",
		"memory.resolve",
		"memory.neighborhood",
		"memory.recall",
	}
	if !slices.Equal(memoryCapabilities, wantCapabilities) {
		t.Fatalf(
			"public memory capabilities = %#v, want %#v",
			memoryCapabilities,
			wantCapabilities,
		)
	}

	report := buildInterfaceContractAuditReport(haftInterfaceCatalog())
	for _, capabilityID := range wantCapabilities {
		surface, ok := findContractAuditSurface(report, capabilityID)
		if !ok {
			t.Fatalf("%s catalog drifted: %#v", capabilityID, surface)
		}
		if capabilityID == "memory.backfill" {
			if surface.SchemaCoverage.Status != "not_mcp_backed" {
				t.Fatalf("%s catalog drifted: %#v", capabilityID, surface)
			}
		} else if surface.SchemaCoverage.Status != "covered" {
			t.Fatalf("%s catalog drifted: %#v", capabilityID, surface)
		}
		switch capabilityID {
		case "memory.validate", "memory.admit":
			if surface.MCPTool != "haft_memory" {
				t.Fatalf(
					"%s MCP tool = %q, want haft_memory",
					capabilityID,
					surface.MCPTool,
				)
			}
		case "memory.backfill":
			if surface.MCPTool != "" ||
				surface.CLICommand !=
					"haft memory backfill --input-file request.json" {
				t.Fatalf(
					"%s execution = %#v, want CLI-only backfill",
					capabilityID,
					surface,
				)
			}
		default:
			if surface.MCPTool != "haft_query" ||
				surface.MCPAction != "memory" {
				t.Fatalf(
					"%s execution = %s/%s, want haft_query/memory",
					capabilityID,
					surface.MCPTool,
					surface.MCPAction,
				)
			}
		}
	}
}

func TestMemoryInterfaceUsesNestedMCPExamplesAndFlatCLIContracts(
	t *testing.T,
) {
	t.Parallel()

	wantCLI := map[string]string{
		"memory.validate": "haft memory validate --input-file request.json",
		"memory.admit":    "haft memory admit --input-file request.json",
	}
	for capabilityID, cliCommand := range wantCLI {
		capability, ok := findInterfaceCapability(
			haftInterfaceCatalog(),
			capabilityID,
		)
		if !ok {
			t.Fatalf("%s capability missing", capabilityID)
		}
		if !strings.HasPrefix(
			capability.CurrentExecution.MCPCall,
			"haft_memory(request={",
		) {
			t.Fatalf(
				"%s MCP example is not nested: %q",
				capabilityID,
				capability.CurrentExecution.MCPCall,
			)
		}
		if capability.CurrentExecution.CLICommand != cliCommand {
			t.Fatalf(
				"%s CLI wire = %q, want flat input-file command %q",
				capabilityID,
				capability.CurrentExecution.CLICommand,
				cliCommand,
			)
		}
		if slices.Contains(capability.InputContract.RequiredFields, "request") {
			t.Fatalf(
				"%s CLI/interface input contract gained MCP envelope: %#v",
				capabilityID,
				capability.InputContract.RequiredFields,
			)
		}
		encoded, err := json.Marshal(capability)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"merge_entities", "split_entity"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf(
					"%s interface output exposes unsupported %s",
					capabilityID,
					forbidden,
				)
			}
		}
	}

	server := fpf.NewServer("test")
	var memorySchema interface{}
	for _, tool := range server.ToolCatalog() {
		if tool.Name == "haft_memory" {
			memorySchema = tool.InputSchema
			break
		}
	}
	encodedSchema, err := json.Marshal(memorySchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"merge_entities", "split_entity"} {
		if strings.Contains(string(encodedSchema), forbidden) {
			t.Fatalf("haft_memory MCP schema exposes unsupported %s", forbidden)
		}
	}
}

func fullMemoryInterfaceAuditToolSchemas(
	t *testing.T,
) map[string]interfaceContractAuditToolSchema {
	t.Helper()
	branches := make(
		[]interface{},
		0,
		len(fullMemoryInterfaceAuditCapabilities()),
	)
	for _, capability := range fullMemoryInterfaceAuditCapabilities() {
		properties := make(map[string]interface{})
		for _, field := range topLevelInterfaceContractFields(
			capability.InputContract,
		) {
			properties[field] = map[string]interface{}{"type": "string"}
		}
		properties["action"] = map[string]interface{}{
			"type":  "string",
			"const": capability.CurrentExecution.MCPAction,
		}
		branches = append(branches, map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties":           properties,
			"required": topLevelInterfaceContractRequiredFields(
				capability.InputContract,
			),
		})
	}
	return interfaceContractAuditToolSchemasFromCatalog(
		[]fpf.Tool{
			{
				Name: "haft_memory",
				InputSchema: map[string]interface{}{
					"oneOf": branches,
				},
			},
		},
	)
}

func installedFullMemoryInterfaceAuditToolSchemas(
	t *testing.T,
) map[string]interfaceContractAuditToolSchema {
	t.Helper()
	server := fpf.NewServer("test")
	server.SetMemoryFullHandler(
		func(
			_ context.Context,
			_ json.RawMessage,
		) (string, error) {
			return "", nil
		},
	)
	return interfaceContractAuditToolSchemasFromCatalog(server.ToolCatalog())
}

func fullMemoryInterfaceAuditCapabilities() []interfaceCapability {
	return []interfaceCapability{
		memoryInterfaceAuditCapability(
			"memory.validate",
			"validate",
			[]string{"contract_version", "action", "basis", "change_set"},
			nil,
		),
		memoryInterfaceAuditCapability(
			"memory.admit",
			"admit",
			[]string{
				"contract_version",
				"action",
				"basis",
				"authority_class",
				"idempotency_key",
				"request_provenance_ref",
				"change_set",
			},
			nil,
		),
		memoryInterfaceAuditCapability(
			"memory.resolve",
			"resolve",
			[]string{
				"contract_version",
				"action",
				"basis",
				"query",
				"max_candidates",
			},
			[]string{"bounded_context_ref"},
		),
		memoryInterfaceAuditCapability(
			"memory.neighborhood",
			"neighborhood",
			[]string{
				"contract_version",
				"action",
				"basis",
				"entity_ref",
				"bounded_context_ref",
				"view",
				"read_budget",
			},
			nil,
		),
		memoryInterfaceAuditCapability(
			"memory.recall",
			"recall",
			[]string{
				"contract_version",
				"action",
				"basis",
				"entity_ref",
				"bounded_context_ref",
				"view",
				"read_budget",
				"query",
				"candidate_budget",
			},
			nil,
		),
	}
}

func memoryInterfaceAuditCapability(
	id string,
	action string,
	required []string,
	optional []string,
) interfaceCapability {
	return interfaceCapability{
		ID:      id,
		Purpose: "test-only sealed typed-memory interface contract",
		CurrentExecution: interfaceExecution{
			MCPTool:   "haft_memory",
			MCPAction: action,
			CLIStatus: "sealed",
		},
		InputContract: interfaceContract{
			RequiredFields: required,
			OptionalFields: optional,
		},
		Invariants: commonInterfaceInvariants(),
	}
}
