package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

func TestInterfaceCatalogJSONListsCapabilities(t *testing.T) {
	var output bytes.Buffer

	if err := writeInterfaceCatalogJSON(&output, haftInterfaceCatalog()); err != nil {
		t.Fatalf("writeInterfaceCatalogJSON returned error: %v", err)
	}

	response := interfaceCatalogResponse{}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal catalog JSON: %v\n%s", err, output.String())
	}

	if response.Kind != "haft_interface_catalog" {
		t.Fatalf("kind = %q, want haft_interface_catalog", response.Kind)
	}

	ids := make(map[string]bool)
	for _, capability := range response.Capabilities {
		ids[capability.ID] = true
	}

	for _, want := range []string{"problem.characterize", "decision.decide", "decision.reconcile_apply", "note.record", "method.pull", "method.close", "method.status", "query.status", "query.related", "query.carrier_manifest", "query.carrier_check", "query.contract_audit", "query.contract_generation", "query.spec_review", "query.drift_events", "query.decision_reconcile", "query.governing_set", "refresh.scan", "refresh.review", "refresh.drain"} {
		if !ids[want] {
			t.Fatalf("catalog missing capability %q in %#v", want, response.Capabilities)
		}
	}
}

func TestInterfaceContractAuditReportsSourcesAndAuthorityPosture(t *testing.T) {
	report := buildInterfaceContractAuditReport(haftInterfaceCatalog())

	if report.Kind != "haft_interface_contract_audit" {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Authority != "read_only_contract_inventory_not_schema_generation" {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Summary.KernelOwnedContracts != report.Summary.Capabilities {
		t.Fatalf("kernel_owned = %d, capabilities = %d", report.Summary.KernelOwnedContracts, report.Summary.Capabilities)
	}
	if report.Summary.BindingAuthoritySurfaces == 0 {
		t.Fatalf("expected binding-sensitive surfaces in summary: %#v", report.Summary)
	}
	if report.Summary.ReadOnlySurfaces == 0 {
		t.Fatalf("expected read-only surfaces in summary: %#v", report.Summary)
	}
	if report.Summary.ValidatedMCPMirrors == 0 {
		t.Fatalf("expected validated MCP mirrors in summary: %#v", report.Summary)
	}
	if report.Summary.ManualCLIContracts == 0 {
		t.Fatalf("expected manual CLI contracts in summary: %#v", report.Summary)
	}
	if report.Summary.UnvalidatedHostFragments != 0 {
		t.Fatalf("unexpected unvalidated host fragments in summary: %#v", report.Summary)
	}

	decide, ok := findContractAuditSurface(report, "decision.decide")
	if !ok {
		t.Fatal("decision.decide missing from contract audit")
	}
	if decide.ContractSourcePosture != "kernel_interface_catalog_source" {
		t.Fatalf("decision.decide contract source posture = %q", decide.ContractSourcePosture)
	}
	if decide.HostSchemaPosture != "validated_mcp_mirror" {
		t.Fatalf("decision.decide host schema posture = %q", decide.HostSchemaPosture)
	}
	if decide.AuthorityPosture != "binding_denied_by_default_mcp" {
		t.Fatalf("decision.decide authority posture = %q", decide.AuthorityPosture)
	}
	if decide.SchemaCoverage.Status != "covered" {
		t.Fatalf("decision.decide schema coverage = %#v", decide.SchemaCoverage)
	}
	if len(decide.SchemaCoverage.MissingFields) != 0 {
		t.Fatalf("decision.decide missing schema fields = %#v", decide.SchemaCoverage.MissingFields)
	}
	if !contractAuditTestContains(decide.SchemaCoverage.ExcludedFields, "task_context") {
		t.Fatalf("decision.decide schema exclusions missing task_context: %#v", decide.SchemaCoverage)
	}
	if decide.ShapeCoverage.Status != "covered" {
		t.Fatalf("decision.decide shape coverage = %#v", decide.ShapeCoverage)
	}
	if len(decide.ShapeCoverage.MissingShapeFields) != 0 {
		t.Fatalf("decision.decide should not report missing shape fields: %#v", decide.ShapeCoverage)
	}
	if len(decide.ShapeCoverage.GeneratorTargetFields) != 0 {
		t.Fatalf("decision.decide should not expose generator targets after nested schema coverage: %#v", decide.ShapeCoverage)
	}
	for _, want := range []string{"kernel_interface_catalog"} {
		if !contractAuditTestContains(decide.ContractSources, want) {
			t.Fatalf("decision.decide contract sources missing %q: %#v", want, decide.ContractSources)
		}
	}
	for _, want := range []string{"internal/cli/interface_test.go", "internal/fpf/server_test.go"} {
		if !contractAuditTestContains(decide.ValidationRefs, want) {
			t.Fatalf("decision.decide validation refs missing %q: %#v", want, decide.ValidationRefs)
		}
	}

	audit, ok := findContractAuditSurface(report, "query.contract_audit")
	if !ok {
		t.Fatal("query.contract_audit missing from contract audit")
	}
	if audit.SchemaPosture != "mcp_schema_mirrored" {
		t.Fatalf("query.contract_audit schema posture = %q", audit.SchemaPosture)
	}
	if audit.HostSchemaPosture != "validated_mcp_mirror" {
		t.Fatalf("query.contract_audit host schema posture = %q", audit.HostSchemaPosture)
	}
	if audit.AuthorityPosture != "read_only_drill_down" {
		t.Fatalf("query.contract_audit authority posture = %q", audit.AuthorityPosture)
	}
	if audit.SchemaCoverage.Status != "covered" {
		t.Fatalf("query.contract_audit schema coverage = %#v", audit.SchemaCoverage)
	}
	if audit.ShapeCoverage.Status != "no_input_shapes" {
		t.Fatalf("query.contract_audit shape coverage = %#v", audit.ShapeCoverage)
	}
	if !audit.LegacyException {
		t.Fatalf("query.contract_audit should document standalone transport exception: %#v", audit)
	}

	compare, ok := findContractAuditSurface(report, "solution.compare")
	if !ok {
		t.Fatal("solution.compare missing from contract audit")
	}
	if compare.ShapeCoverage.Status != "covered" {
		t.Fatalf("solution.compare shape coverage = %#v", compare.ShapeCoverage)
	}
	if !contractAuditTestContains(compare.ShapeCoverage.SkippedFields, "scores") {
		t.Fatalf("solution.compare should explicitly skip dynamic score keys: %#v", compare.ShapeCoverage)
	}

	specUse, ok := findContractAuditSurface(report, "query.spec_use")
	if !ok {
		t.Fatal("query.spec_use missing from contract audit")
	}
	if specUse.ShapeCoverage.Status != "covered" {
		t.Fatalf("query.spec_use shape coverage = %#v", specUse.ShapeCoverage)
	}
	if len(specUse.ShapeCoverage.GeneratorTargetFields) != 0 {
		t.Fatalf("query.spec_use should not expose operational_gate generator targets after nested schema coverage: %#v", specUse.ShapeCoverage)
	}
	if report.Summary.ShapeMissingSurfaces != 0 {
		t.Fatalf("contract audit should not count generator targets as missing surfaces: %#v", report.Summary)
	}
	if report.Summary.ShapeGeneratorTargets != 0 || report.Summary.ShapeGeneratorTargetFields != 0 {
		t.Fatalf("contract audit should have no remaining generator targets: %#v", report.Summary)
	}
	specReview, ok := findContractAuditSurface(report, "query.spec_review")
	if !ok {
		t.Fatal("query.spec_review missing from contract audit")
	}
	if specReview.HostSchemaPosture != "validated_mcp_mirror" {
		t.Fatalf("query.spec_review host schema posture = %q", specReview.HostSchemaPosture)
	}
	if specReview.AuthorityPosture != "read_only_drill_down" {
		t.Fatalf("query.spec_review authority posture = %q", specReview.AuthorityPosture)
	}

	notes := strings.Join(report.Notes, " ")
	if !strings.Contains(notes, "Schema visibility is not operator authorization") {
		t.Fatalf("audit notes missing authority boundary:\n%s", notes)
	}
	if !strings.Contains(notes, "Host schema posture classifies each fragment") {
		t.Fatalf("audit notes missing host schema posture boundary:\n%s", notes)
	}
}

func TestInterfaceContractAuditClassifiesEveryHostFragment(t *testing.T) {
	report := buildInterfaceContractAuditReport(haftInterfaceCatalog())

	for _, surface := range report.Surfaces {
		if surface.ContractSourcePosture == "" || surface.ContractSourcePosture == "unclassified_contract_source" {
			t.Fatalf("%s has unclassified contract source posture: %#v", surface.CapabilityID, surface)
		}
		if surface.HostSchemaPosture == "" || surface.HostSchemaPosture == "unvalidated_host_schema_fragment" {
			t.Fatalf("%s has unvalidated host schema posture: %#v", surface.CapabilityID, surface)
		}
		if surface.MCPTool != "" && surface.MCPAction != "" && !strings.HasPrefix(surface.HostSchemaPosture, "validated_mcp_mirror") {
			t.Fatalf("%s MCP-backed surface should be a validated mirror, got %q", surface.CapabilityID, surface.HostSchemaPosture)
		}
		if surface.MCPTool == "" && surface.HostSchemaPosture != "manual_cli_contract_not_generated" {
			t.Fatalf("%s CLI/manual surface posture = %q", surface.CapabilityID, surface.HostSchemaPosture)
		}
	}
}

func TestInterfaceContractGenerationManifestListsGeneratorTargets(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())

	if report.Kind != "haft_interface_contract_generation_manifest" {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Authority != "read_only_generation_manifest_not_generated_schema" {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Source != "kernel_interface_catalog_field_shapes" {
		t.Fatalf("source = %q", report.Source)
	}
	if !strings.HasPrefix(report.SourceDigest, "sha256:") {
		t.Fatalf("source digest = %q", report.SourceDigest)
	}
	if report.Summary.GeneratorTargetSurfaces != 0 || report.Summary.GeneratorTargetFields != 0 {
		t.Fatalf("expected empty generator target queue after nested schema coverage: %#v", report.Summary)
	}
	if report.SurfacePolicy.DefaultStatus != "cue_or_count_only_never_inline_generation_manifest" {
		t.Fatalf("default status policy = %q", report.SurfacePolicy.DefaultStatus)
	}
	if report.SurfacePolicy.DefaultCodeContext != "lane_index_only_never_inline_generated_descriptions" {
		t.Fatalf("code_context policy = %q", report.SurfacePolicy.DefaultCodeContext)
	}
	if report.SurfacePolicy.ToolsList != "action_enum_and_compact_description_only_no_generated_schema_fragments" {
		t.Fatalf("tools/list policy = %q", report.SurfacePolicy.ToolsList)
	}
	if !stringSliceContains(report.SurfacePolicy.RequiredGuards, "carrier_semio_authority_boundary") {
		t.Fatalf("surface policy missing semio guard: %#v", report.SurfacePolicy.RequiredGuards)
	}

	if len(report.Targets) != 0 {
		t.Fatalf("expected no current generator targets: %#v", report.Targets)
	}

	notes := strings.Join(report.Notes, " ")
	if !strings.Contains(notes, "not a generated schema") || !strings.Contains(notes, "not operator authorization") {
		t.Fatalf("generation manifest notes missing authority boundary:\n%s", notes)
	}
}

func TestInterfaceContractGenerationTextIsCompact(t *testing.T) {
	var output bytes.Buffer
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())

	if err := writeInterfaceContractGenerationText(&output, report); err != nil {
		t.Fatalf("writeInterfaceContractGenerationText returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Haft interface contract generation manifest v1",
		"read_only_generation_manifest_not_generated_schema",
		"source: kernel_interface_catalog_field_shapes sha256:",
		"generator_target_surfaces=",
		"generator_target_fields=",
		"no current generator targets",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("contract generation text missing %q:\n%s", want, text)
		}
	}
}

func TestHandleQuintQueryContractGenerationReturnsReadOnlyManifest(t *testing.T) {
	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "contract_generation",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery contract_generation returned error: %v", err)
	}

	var report interfaceContractGenerationReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode contract generation manifest: %v\n%s", err, result)
	}
	if report.Authority != "read_only_generation_manifest_not_generated_schema" {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.SurfacePolicy.GeneratedDescriptions != "drill_down_only_validate_with_carrier_semio_before_host_materialization" {
		t.Fatalf("generated description policy = %q", report.SurfacePolicy.GeneratedDescriptions)
	}
	if len(report.Targets) != 0 {
		t.Fatalf("expected empty generator target manifest from MCP query: %#v", report.Targets)
	}
}

func TestDefaultStatusDoesNotInlineContractGenerationManifest(t *testing.T) {
	fixture := newCheckTestProject(t)

	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery status returned error: %v", err)
	}

	for _, forbidden := range []string{
		"haft_interface_contract_generation_manifest",
		"read_only_generation_manifest_not_generated_schema",
		"generator_target_surfaces",
		"generator_target_fields",
		"surface_policy",
	} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("default status inlined contract generation manifest fragment %q:\n%s", forbidden, result)
		}
	}
}

func TestInterfaceContractGenerationDiscoveryShapeNamesSurfacePolicy(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.contract_generation")
	if !ok {
		t.Fatal("query.contract_generation capability missing")
	}

	shape := capability.InputContract.FieldShapes[0].Shape
	if !strings.Contains(shape, "surface_policy") {
		t.Fatalf("contract generation discovery fields missing surface_policy: %s", shape)
	}
}

func TestInterfaceValueSpaceNamesSimplifyKillCriteriaBoundary(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.value_space")
	if !ok {
		t.Fatal("query.value_space capability missing")
	}

	shapes, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{
		"simplify_kill_criteria",
		"read_only_review_trigger_not_automatic_gate",
		"not automatic gates",
	} {
		if !strings.Contains(shapes, want) {
			t.Fatalf("query.value_space shapes missing %q:\n%s", want, shapes)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{
		"not evidence",
		"GateDecision",
		"product-value proof",
	} {
		if !strings.Contains(notes, want) {
			t.Fatalf("query.value_space notes missing %q:\n%s", want, notes)
		}
	}
}

func TestInterfaceSpecReviewNamesAdvisoryBoundary(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.spec_review")
	if !ok {
		t.Fatal("query.spec_review capability missing")
	}
	if capability.CurrentExecution.MCPCall != `haft_query(action="spec_review")` {
		t.Fatalf("spec_review MCP call = %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft spec review --json") {
		t.Fatalf("spec_review CLI command missing: %#v", capability.CurrentExecution)
	}
	contract, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{"advisory_only", "spec_semantic_review_v2", "claim_register", "state_reading", "blocked_for_stronger_use_findings", "category", "claim_posture|publication_boundary|frame|unknown_abstain"} {
		if !strings.Contains(contract, want) {
			t.Fatalf("spec_review contract missing %q:\n%s", want, contract)
		}
	}
	invariants := strings.Join(capability.Invariants, "\n")
	for _, want := range []string{"not evidence", "Default status", "abstains/blocks stronger use"} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("spec_review invariant missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceCatalogMCPActionsExistInToolsListSchemas(t *testing.T) {
	toolActionEnums := fpfToolActionEnums(t)

	for _, capability := range haftInterfaceCatalog() {
		toolName := capability.CurrentExecution.MCPTool
		action := capability.CurrentExecution.MCPAction
		if toolName == "" || action == "" {
			continue
		}

		enum, ok := toolActionEnums[toolName]
		if !ok {
			t.Fatalf("%s declares unknown MCP tool %q", capability.ID, toolName)
		}
		if !contractAuditTestContains(enum, action) {
			t.Fatalf("%s declares %s(action=%q), but tools/list enum is %#v", capability.ID, toolName, action, enum)
		}
	}
}

func TestInterfaceCatalogMCPFieldsExistInToolsListSchemas(t *testing.T) {
	toolProperties := fpfToolProperties(t)
	exclusions := interfaceContractAuditSchemaFieldExclusions()

	for _, capability := range haftInterfaceCatalog() {
		toolName := capability.CurrentExecution.MCPTool
		if toolName == "" || capability.CurrentExecution.MCPAction == "" {
			continue
		}

		properties, ok := toolProperties[toolName]
		if !ok {
			t.Fatalf("%s declares unknown MCP tool %q", capability.ID, toolName)
		}

		for _, field := range topLevelInterfaceContractFields(capability.InputContract) {
			if exclusions[capability.ID][field] {
				continue
			}
			if _, ok := properties[field]; !ok {
				t.Fatalf("%s declares field %q, but %s tools/list schema properties are %s", capability.ID, field, toolName, sortedMapKeys(properties))
			}
		}
	}
}

func TestInterfaceContractAuditTextIsCompact(t *testing.T) {
	var output bytes.Buffer
	report := buildInterfaceContractAuditReport(haftInterfaceCatalog())

	if err := writeInterfaceContractAuditText(&output, report); err != nil {
		t.Fatalf("writeInterfaceContractAuditText returned error: %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"Haft interface contract audit v1",
		"read_only_contract_inventory_not_schema_generation",
		"binding_sensitive=",
		"schema_coverage=",
		"shape_coverage=",
		"generator_targets=",
		"host_fragments=",
		"host_schema=",
		"query.contract_audit",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("contract audit text missing %q:\n%s", want, text)
		}
	}
}

func TestHandleQuintQueryContractAuditReturnsReadOnlyReport(t *testing.T) {
	store := setupCLIArtifactStore(t)

	result, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action": "contract_audit",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery contract_audit returned error: %v", err)
	}

	var report interfaceContractAuditReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode contract audit: %v\n%s", err, result)
	}
	if report.Authority != "read_only_contract_inventory_not_schema_generation" {
		t.Fatalf("authority = %q", report.Authority)
	}
	if _, ok := findContractAuditSurface(report, "decision.decide"); !ok {
		t.Fatalf("decision.decide missing from MCP contract audit: %#v", report.Surfaces)
	}
}

func findContractAuditSurface(report interfaceContractAuditReport, id string) (interfaceContractAuditSurface, bool) {
	for _, surface := range report.Surfaces {
		if surface.CapabilityID == id {
			return surface, true
		}
	}
	return interfaceContractAuditSurface{}, false
}

func findContractGenerationTarget(report interfaceContractGenerationReport, id string) (interfaceContractGenerationTarget, bool) {
	for _, target := range report.Targets {
		if target.CapabilityID == id {
			return target, true
		}
	}
	return interfaceContractGenerationTarget{}, false
}

func contractAuditTestContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func fpfToolActionEnums(t *testing.T) map[string][]string {
	t.Helper()

	toolProperties := fpfToolProperties(t)
	toolActionEnums := make(map[string][]string)

	for toolName, properties := range toolProperties {
		action, ok := properties["action"].(map[string]interface{})
		if !ok {
			continue
		}
		rawEnum, ok := action["enum"].([]interface{})
		if !ok {
			continue
		}

		values := make([]string, 0, len(rawEnum))
		for _, rawValue := range rawEnum {
			value, ok := rawValue.(string)
			if !ok {
				t.Fatalf("%s action enum contains non-string value %#v", toolName, rawValue)
			}
			values = append(values, value)
		}
		toolActionEnums[toolName] = values
	}

	return toolActionEnums
}

func fpfToolProperties(t *testing.T) map[string]map[string]interface{} {
	t.Helper()

	server := fpf.NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})

	toolProperties := make(map[string]map[string]interface{})
	for _, tool := range server.ToolCatalog() {
		inputSchema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			t.Fatalf("%s input schema has wrong type: %#v", tool.Name, tool.InputSchema)
		}
		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		toolProperties[tool.Name] = properties
	}

	return toolProperties
}

func sortedMapKeys(values map[string]interface{}) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func TestInterfaceDecisionContractExposesCLIAndManualInvariant(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "decision.decide")
	if !ok {
		t.Fatal("decision.decide capability missing")
	}

	if capability.CurrentExecution.CLIStatus != "input_file_execution_shipped" {
		t.Fatalf("CLI status = %q, want shipped input-file execution", capability.CurrentExecution.CLIStatus)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft artifact create decision.decide") {
		t.Fatalf("decision capability should name CLI input-file execution:\n%#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.MCPCall, `haft_decision(action="decide"`) {
		t.Fatalf("decision capability should name current MCP execution:\n%#v", capability.CurrentExecution)
	}

	required := strings.Join(capability.InputContract.RequiredFields, " ")
	for _, want := range []string{"selected_title", "rollback", "valid_until"} {
		if !strings.Contains(required, want) {
			t.Fatalf("decision required fields missing %q in %q", want, required)
		}
	}

	optionals := strings.Join(capability.InputContract.OptionalFields, " ")
	if !strings.Contains(optionals, "choice_result") {
		t.Fatalf("decision optional fields missing choice_result in %q", optionals)
	}
	if !strings.Contains(optionals, "transformation_record") {
		t.Fatalf("decision optional fields missing transformation_record in %q", optionals)
	}
	for _, want := range []string{"decision_subject_ref", "implementation_footprint", "governance_targets", "drift_watch_targets"} {
		if !strings.Contains(optionals, want) {
			t.Fatalf("decision optional fields missing %q in %q", want, optionals)
		}
	}
	if !strings.Contains(optionals, "claims") {
		t.Fatalf("decision optional fields missing claims in %q", optionals)
	}

	fieldShapes := ""
	for _, shape := range capability.InputContract.FieldShapes {
		fieldShapes += shape.Field + " " + shape.Shape + " " + shape.Note + " "
	}
	for _, want := range []string{"choice_result", "option_set", "comparison_basis", "choice_rule", "next_move", "reversibility", "reopen_condition", "transformation_record", "transformed_entity", "initial_state", "post_state", "relation", "context", "window", "method_refs", "work_refs", "evidence_refs", "publication_refs", "implementation_footprint", "Footprint-only files are not governance drift authority", "governance_targets", "drift_watch_targets", "binding_targets", "target_ref", "api_contract|invariant|spec_section", "haft-target: <target_ref>", "fenced yaml spec-section id", "text_hash", "needs binding resolution", "affected_files auto-enrich binding_targets", "ambiguous files remain unenriched", "schema_or_behavior_changed", "claims[]", "lifecycle_status", "successor_ref", "governance_target_refs", "predictions remain the compatibility projection"} {
		if !strings.Contains(fieldShapes, want) {
			t.Fatalf("decision field shapes missing %q:\n%s", want, fieldShapes)
		}
	}

	invariants := strings.Join(capability.Invariants, " ")
	if !strings.Contains(invariants, "Human binding remains mandatory") {
		t.Fatalf("decision interface must preserve manual binding invariant:\n%s", invariants)
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	if !strings.Contains(notes, "C.11 subject, option_set, comparison_basis, choice_rule, next_move, reversibility, and reopen_condition") {
		t.Fatalf("decision interface notes missing C.11 choice_result warning:\n%s", notes)
	}
	if !strings.Contains(notes, "not a MethodRun, WorkCommission, evidence item, or publication unit") {
		t.Fatalf("decision interface notes missing transformation separation warning:\n%s", notes)
	}
	if !strings.Contains(notes, "refs point outward and do not prove occurrence") {
		t.Fatalf("decision interface notes missing transformation refs boundary:\n%s", notes)
	}
}

func TestInterfaceProblemFrameExposesProblemProfileAdmissionFields(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "problem.frame")
	if !ok {
		t.Fatal("problem.frame capability missing")
	}

	optionals := strings.Join(capability.InputContract.OptionalFields, " ")
	for _, want := range []string{
		"problem_profile",
		"source_kind",
		"why_now",
		"scope",
		"acceptance_probe",
		"freshness_disposition",
	} {
		if !strings.Contains(optionals, want) {
			t.Fatalf("problem.frame optional fields missing %q in %q", want, optionals)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"P2W readiness is computed", "wish/ticket/chosen_method"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("problem.frame notes missing %q:\n%s", want, notes)
		}
	}
}

func TestInterfaceStatusNamesCockpitAndDetailCalls(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.status")
	if !ok {
		t.Fatal("query.status capability missing")
	}

	if !strings.Contains(capability.Purpose, "operator cockpit") {
		t.Fatalf("status purpose should name cockpit default:\n%s", capability.Purpose)
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{
		"compact operator cockpit",
		"not evidence of absence",
		"full=true for detailed status",
		`haft_query(action="coverage")`,
		`haft_refresh(action="scan", verbose=true)`,
		`haft_refresh(action="plan")`,
		`haft_refresh(action="review")`,
		`haft_refresh(action="drain", dry_run=true)`,
	} {
		if !strings.Contains(notes, want) {
			t.Fatalf("status interface notes missing %q:\n%s", want, notes)
		}
	}

	outputVolume := strings.Join(capability.OutputVolume, " ")
	for _, want := range []string{
		"default: compact cockpit",
		"one-line coverage cue",
		"full=true: detailed status",
		"complete coverage projection",
	} {
		if !strings.Contains(outputVolume, want) {
			t.Fatalf("status output volume missing %q:\n%s", want, outputVolume)
		}
	}
}

func TestInterfaceCarrierSurfacesAreReadOnlyDrillDowns(t *testing.T) {
	for _, capabilityID := range []string{"query.carrier_manifest", "query.carrier_check"} {
		t.Run(capabilityID, func(t *testing.T) {
			capability, ok := findInterfaceCapability(haftInterfaceCatalog(), capabilityID)
			if !ok {
				t.Fatalf("%s capability missing", capabilityID)
			}
			if capability.CurrentExecution.MCPTool != "haft_query" {
				t.Fatalf("%s MCP tool = %q", capabilityID, capability.CurrentExecution.MCPTool)
			}
			if capability.CurrentExecution.CLIStatus != "available" {
				t.Fatalf("%s CLI status = %q", capabilityID, capability.CurrentExecution.CLIStatus)
			}
			if !strings.Contains(capability.CurrentExecution.CLICommand, "haft carrier") {
				t.Fatalf("%s CLI command missing carrier path: %#v", capabilityID, capability.CurrentExecution)
			}
			outputVolume := strings.Join(capability.OutputVolume, " ")
			if !strings.Contains(outputVolume, "never in compact status") {
				t.Fatalf("%s output volume must preserve compact status boundary: %s", capabilityID, outputVolume)
			}
			invariants := strings.Join(capability.Invariants, " ")
			for _, want := range []string{"read-only", "Default status"} {
				if !strings.Contains(invariants, want) {
					t.Fatalf("%s invariants missing %q:\n%s", capabilityID, want, invariants)
				}
			}
		})
	}
}

func TestInterfaceGoverningSetNamesAuthorityFrontier(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.governing_set")
	if !ok {
		t.Fatal("query.governing_set capability missing")
	}

	if !strings.Contains(capability.CurrentExecution.MCPCall, `haft_query(action="governing_set"`) {
		t.Fatalf("governing_set MCP call = %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision governing-set") ||
		!strings.Contains(capability.CurrentExecution.CLICommand, "--query") {
		t.Fatalf("governing_set CLI command missing governing-set drill-down:\n%#v", capability.CurrentExecution)
	}

	fieldShapes := ""
	for _, shape := range capability.InputContract.FieldShapes {
		fieldShapes += shape.Field + " " + shape.Shape + " " + shape.Note + " "
	}
	for _, want := range []string{"read_only_current_authority_frontier", "single_current_authority", "conflict_requires_operator", "fallback_target_sets", "scope_enrichment_sets", "whole_file_fallback_requires_scope_enrichment", "scope_repair_hints", "derived_read_only_not_gate_decision", "Terminal decisions are history refs", "bearer_ref", "source_refs", "--subject-ref", "--target-ref"} {
		if !strings.Contains(fieldShapes, want) {
			t.Fatalf("governing_set field shapes missing %q:\n%s", want, fieldShapes)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"active/refresh_due", "fallback_target_sets", "scope_enrichment_sets", "Read-only", "does not supersede", "focused drill-down"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("governing_set notes missing %q:\n%s", want, notes)
		}
	}

	invariants := strings.Join(capability.Invariants, " ")
	for _, want := range []string{"read-only", "not live authority", "operator review"} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("governing_set invariant missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceRefreshReviewNamesAuthorityBoundary(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "refresh.review")
	if !ok {
		t.Fatal("refresh.review capability missing")
	}

	if capability.CurrentExecution.MCPCall != `haft_refresh(action="review")` {
		t.Fatalf("refresh.review MCP call = %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft overseer judgment --json") {
		t.Fatalf("refresh.review CLI command missing judgment path:\n%#v", capability.CurrentExecution)
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"rung-3", "not evidence", "approval", "mutation"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("refresh.review notes missing %q:\n%s", want, notes)
		}
	}

	invariants := strings.Join(capability.Invariants, " ")
	for _, want := range []string{"outside automated execution", "review metadata, not authority"} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("refresh.review invariant missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceRefreshDrainNamesSafeClosureBoundary(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "refresh.drain")
	if !ok {
		t.Fatal("refresh.drain capability missing")
	}

	if capability.CurrentExecution.MCPCall != `haft_refresh(action="drain", dry_run=true)` {
		t.Fatalf("refresh.drain MCP call = %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft overseer drain --dry-run --json") {
		t.Fatalf("refresh.drain CLI command missing drain path:\n%#v", capability.CurrentExecution)
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"dry_run=true", "rung-1/rung-2", "needs_operator"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("refresh.drain notes missing %q:\n%s", want, notes)
		}
	}

	invariants := strings.Join(capability.Invariants, " ")
	for _, want := range []string{"opt-in", "not create semantic approval"} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("refresh.drain invariant missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceCodeContextNamesLaneEscalation(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.code_context")
	if !ok {
		t.Fatal("query.code_context capability missing")
	}

	if !strings.Contains(capability.CurrentExecution.MCPCall, `lane="index"`) {
		t.Fatalf("code_context interface should show lane=index default:\n%#v", capability.CurrentExecution)
	}

	optionals := strings.Join(capability.InputContract.OptionalFields, " ")
	for _, want := range []string{"lane", "limit", "full"} {
		if !strings.Contains(optionals, want) {
			t.Fatalf("code_context optional fields missing %q in %q", want, optionals)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"Default output is lane=index", "symbols, decisions, invariants, notes, problems, portfolios, all", "Prefer one typed lane"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("code_context notes missing %q:\n%s", want, notes)
		}
	}

	outputVolume := strings.Join(capability.OutputVolume, " ")
	if !strings.Contains(outputVolume, "default: lane=index") {
		t.Fatalf("code_context interface should name lane index default:\n%s", outputVolume)
	}
	if !strings.Contains(outputVolume, "typed lanes") || !strings.Contains(outputVolume, "full=true: complete audit dump") {
		t.Fatalf("code_context interface should name typed lanes and audit dump:\n%s", outputVolume)
	}
}

func TestInterfaceRelatedDocumentsSemanticViews(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.related")
	if !ok {
		t.Fatal("query.related capability missing")
	}

	if !strings.Contains(capability.CurrentExecution.MCPCall, `action="related"`) {
		t.Fatalf("related interface should name related action:\n%#v", capability.CurrentExecution)
	}

	fieldShapes := ""
	for _, shape := range capability.InputContract.FieldShapes {
		fieldShapes += shape.Field + " " + shape.Shape + " " + shape.Note + " "
	}
	for _, want := range []string{"problem_card.semantic", "publication_unit", "source_edition_pin", "problem_card.views", "source_episteme", "publication_projection", "carrier_bytes"} {
		if !strings.Contains(fieldShapes, want) {
			t.Fatalf("related field shapes missing %q:\n%s", want, fieldShapes)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	if !strings.Contains(notes, "SQLite remains runtime source of truth") {
		t.Fatalf("related notes should document source-of-truth policy:\n%s", notes)
	}
}

func TestInterfaceMethodCloseNamesEvidenceAndWaiverContract(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "method.close")
	if !ok {
		t.Fatal("method.close capability missing")
	}

	required := strings.Join(capability.InputContract.RequiredFields, " ")
	if !strings.Contains(required, "pull_id") {
		t.Fatalf("method.close required fields = %q, want pull_id", required)
	}

	optionals := strings.Join(capability.InputContract.OptionalFields, " ")
	for _, want := range []string{"gate_results", "verification", "waivers"} {
		if !strings.Contains(optionals, want) {
			t.Fatalf("method.close optional fields = %q, missing %q", optionals, want)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"gate_results[] shape", "evidence_refs", "waivers[] shape", "verification shape", "close_template"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("method.close notes missing %q:\n%s", want, notes)
		}
	}

	outputVolume := strings.Join(capability.OutputVolume, " ")
	if !strings.Contains(outputVolume, "close_template") {
		t.Fatalf("method.close should name show close_template recovery:\n%s", outputVolume)
	}
}

func TestInterfaceNestedContractsExposeShapesAndTemplates(t *testing.T) {
	cases := []struct {
		capability string
		field      string
		template   string
		fragment   string
	}{
		{
			capability: "problem.characterize",
			field:      "dimensions[]",
			template:   "characterize_problem",
			fragment:   "parity_plan",
		},
		{
			capability: "solution.explore",
			field:      "variants[]",
			template:   "explore_variants",
			fragment:   "stepping_stone_basis",
		},
		{
			capability: "solution.compare",
			field:      "scores",
			template:   "flat_compare",
			fragment:   "dimension_name",
		},
		{
			capability: "decision.decide",
			field:      "predictions[]",
			template:   "decide",
			fragment:   "rollback",
		},
		{
			capability: "method.close",
			field:      "gate_results[]",
			template:   "",
			fragment:   "Use gate_id, not gate",
		},
	}

	for _, tc := range cases {
		t.Run(tc.capability, func(t *testing.T) {
			capability, ok := findInterfaceCapability(haftInterfaceCatalog(), tc.capability)
			if !ok {
				t.Fatalf("%s capability missing", tc.capability)
			}

			shapes, templates := marshalContractFragments(t, capability.InputContract)
			if !strings.Contains(shapes, tc.field) {
				t.Fatalf("%s field_shapes missing %q:\n%s", tc.capability, tc.field, shapes)
			}
			if !strings.Contains(shapes, tc.fragment) && !strings.Contains(templates, tc.fragment) {
				t.Fatalf("%s contract missing fragment %q:\nshapes=%s\ntemplates=%s", tc.capability, tc.fragment, shapes, templates)
			}
			if tc.template != "" && !strings.Contains(templates, tc.template) {
				t.Fatalf("%s input_templates missing %q:\n%s", tc.capability, tc.template, templates)
			}
		})
	}
}

func TestInterfaceSpecUseDocumentsOperationalGate(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.spec_use")
	if !ok {
		t.Fatal("query.spec_use capability missing")
	}

	shapes, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{"operational_gate", "require_current_source_and_admitted_use", "passed|blocked"} {
		if !strings.Contains(shapes, want) {
			t.Fatalf("query.spec_use contract missing %q:\n%s", want, shapes)
		}
	}
	invariants := strings.Join(capability.Invariants, "\n")
	if !strings.Contains(invariants, "GateDecision remains a derived reading") {
		t.Fatalf("query.spec_use invariants missing derived-reading boundary:\n%s", invariants)
	}
}

func TestInterfaceDriftEventsDocumentsFanoutBoundary(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.drift_events")
	if !ok {
		t.Fatal("query.drift_events capability missing")
	}

	if capability.CurrentExecution.MCPCall != `haft_query(action="drift_events")` {
		t.Fatalf("drift_events MCP call = %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft drift events --json") {
		t.Fatalf("drift_events CLI command missing: %#v", capability.CurrentExecution)
	}
	shapes, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{"schema_version", "unique_events", "impacted_decisions", "needs_binding_resolution_events", "semantic_target_events", "file_fallback_events", "unknown_high_risk_events", "root_cause", "semantic_target_changed", "target_renamed", "retarget_candidate", "schema_changed", "target_status", "modified|removed|renamed|retarget_candidate", "edited_symbol_move_candidate", "needs_scope_enrichment", "suggested_next_command", "haft decision reconcile --json", "haft_refresh(action=", "fallback_kind", "fallback_reason", "max_fanout", "compatibility_reports"} {
		if !strings.Contains(shapes, want) {
			t.Fatalf("query.drift_events contract missing %q:\n%s", want, shapes)
		}
	}
	invariants := strings.Join(capability.Invariants, "\n")
	for _, want := range []string{"read-only", "Fanout is not independent debt count", "Compatibility per-decision drift reports remain available"} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("query.drift_events invariants missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceDecisionReconcileIsReadOnlyAndRejectsFileOverlapMerge(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.decision_reconcile")
	if !ok {
		t.Fatal("query.decision_reconcile capability missing")
	}

	if capability.CurrentExecution.MCPCall != `haft_query(action="decision_reconcile")` {
		t.Fatalf("decision_reconcile MCP call = %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile --json") {
		t.Fatalf("decision_reconcile CLI command missing: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile metrics --json") {
		t.Fatalf("decision_reconcile metrics CLI command missing: %#v", capability.CurrentExecution)
	}

	shapes, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{
		"report_only_not_binding_authority",
		"affected_files are footprint hints",
		"never merge evidence",
		"scope_enrichment_candidates",
		"scope_repair_hints",
		"enrich_scope",
		"preview",
		"report_only_preview_not_binding_authority",
		"required_selection_fields",
		"items[].successor_ref",
		"validation_notes",
		"preview is advisory and cannot authorize apply",
		"downstream_impact",
		"does not relink downstream artifacts",
		"read_only_reconciliation_metrics_not_binding_authority",
		"capture_before_and_after_operator_approved_reconciliation_apply",
		"fallback_target_sets",
		"max_fanout",
	} {
		if !strings.Contains(shapes, want) {
			t.Fatalf("query.decision_reconcile contract missing %q:\n%s", want, shapes)
		}
	}
	invariants := strings.Join(capability.Invariants, "\n")
	for _, want := range []string{
		"read-only",
		"File overlap alone is not a merge/supersede signal",
		"Operator approval is required",
		"Preview generation is read-only",
	} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("query.decision_reconcile invariants missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceDecisionReconcileApplyRequiresOperatorApprovedCLISelection(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "decision.reconcile_apply")
	if !ok {
		t.Fatal("decision.reconcile_apply capability missing")
	}

	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile apply") {
		t.Fatalf("CLI command missing apply path: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.MCPCall, "MCP has no apply action") {
		t.Fatalf("MCP call must name no-apply boundary: %#v", capability.CurrentExecution)
	}

	contract, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{
		"operator_approved_reconciliation_selection",
		"operator_approval_ref",
		"merge_through_successor",
		"does not create binding decisions",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("decision.reconcile_apply contract missing %q:\n%s", want, contract)
		}
	}
	invariants := strings.Join(capability.Invariants, "\n")
	for _, want := range []string{
		"MCP has no reconciliation apply action",
		"Operator approval ref is required",
		"Old decision IDs remain searchable",
	} {
		if !strings.Contains(invariants, want) {
			t.Fatalf("decision.reconcile_apply invariants missing %q:\n%s", want, invariants)
		}
	}
}

func TestInterfaceCompareTemplateUsesFlatCanonicalScores(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "solution.compare")
	if !ok {
		t.Fatal("solution.compare capability missing")
	}

	_, templates := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{`"scores"`, `"V1"`, `"latency"`, `"dominated_variants"`, `"pareto_tradeoffs"`, `"legacy_recommendation_ref"`} {
		if !strings.Contains(templates, want) {
			t.Fatalf("solution.compare template missing %q:\n%s", want, templates)
		}
	}
	if strings.Contains(templates, `"results"`) {
		t.Fatalf("solution.compare should advertise flat canonical fields, not legacy results carrier:\n%s", templates)
	}
}

func TestInterfaceCompareJSONStaysUnderPlanningBudget(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "solution.compare")
	if !ok {
		t.Fatal("solution.compare capability missing")
	}

	var output bytes.Buffer
	if err := writeJSON(&output, capability); err != nil {
		t.Fatalf("write solution.compare JSON: %v", err)
	}

	const maxInterfaceBytes = 5000
	if output.Len() > maxInterfaceBytes {
		t.Fatalf("solution.compare interface JSON = %d bytes, want <= %d", output.Len(), maxInterfaceBytes)
	}
}

func marshalContractFragments(t *testing.T, contract interfaceContract) (string, string) {
	t.Helper()

	shapes, err := json.Marshal(contract.FieldShapes)
	if err != nil {
		t.Fatalf("marshal field shapes: %v", err)
	}

	templates, err := json.Marshal(contract.InputTemplates)
	if err != nil {
		t.Fatalf("marshal input templates: %v", err)
	}

	return string(shapes), string(templates)
}
