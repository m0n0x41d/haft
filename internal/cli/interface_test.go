package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

func TestInterfaceCatalogTextStaysCompact(t *testing.T) {
	var output bytes.Buffer

	if err := writeInterfaceCatalogText(&output, haftInterfaceCatalog()); err != nil {
		t.Fatalf("writeInterfaceCatalogText returned error: %v", err)
	}

	result := output.String()
	for _, want := range []string{
		"Haft interface capabilities:",
		"query.contract_generation",
		"Use `haft interface <capability> --json`",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("compact interface catalog missing %q:\n%s", want, result)
		}
	}
	assertNoContractGenerationManifestInline(t, "compact interface catalog", result)
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
	if report.Summary.UnvalidatedFragments != 0 {
		t.Fatalf("unexpected unvalidated contract fragments in summary: %#v", report.Summary)
	}
	if report.Summary.ValidatedFragments == 0 {
		t.Fatalf("expected validated contract fragments in summary: %#v", report.Summary)
	}
	if report.Summary.LegacyFragments == 0 {
		t.Fatalf("expected legacy/manual contract fragments in summary: %#v", report.Summary)
	}
	if got := report.Summary.GeneratedTargetFragments + report.Summary.ValidatedFragments + report.Summary.LegacyFragments + report.Summary.UnvalidatedFragments; got != report.Summary.Capabilities {
		t.Fatalf("fragment posture counts = %d, capabilities = %d: %#v", got, report.Summary.Capabilities, report.Summary)
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
	if decide.ContractFragmentPosture != "validated_fragment" {
		t.Fatalf("decision.decide contract fragment posture = %q", decide.ContractFragmentPosture)
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
	if !contractAuditTestContains(decide.SchemaCoverage.MCPRequiredFields, "action") {
		t.Fatalf("decision.decide required fields missing action: %#v", decide.SchemaCoverage)
	}
	if len(decide.SchemaCoverage.MissingRequiredFields) != 0 {
		t.Fatalf("decision.decide missing required schema fields = %#v", decide.SchemaCoverage)
	}
	if decide.SchemaCoverage.RequiredPosture != "transport_action_required_action_specific_fields_validated_by_handler" {
		t.Fatalf("decision.decide required posture = %#v", decide.SchemaCoverage)
	}
	if !contractAuditTestContains(decide.SchemaCoverage.ActionRequiredFields, "selected_title") {
		t.Fatalf("decision.decide action required fields missing selected_title: %#v", decide.SchemaCoverage)
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
	if audit.ContractFragmentPosture != "validated_fragment" {
		t.Fatalf("query.contract_audit contract fragment posture = %q", audit.ContractFragmentPosture)
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
	if report.Summary.SchemaRequiredMissing != 0 || report.Summary.SchemaMissingRequiredFields != 0 {
		t.Fatalf("contract audit should have no missing required MCP schema fields: %#v", report.Summary)
	}
	if report.Summary.SchemaRequiredCovered == 0 {
		t.Fatalf("contract audit should count required-field coverage: %#v", report.Summary)
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
	if !strings.Contains(notes, "Contract fragment posture classifies every fragment") {
		t.Fatalf("audit notes missing contract fragment posture boundary:\n%s", notes)
	}
	if !strings.Contains(notes, "MCP required-field coverage checks transport-level required fields") {
		t.Fatalf("audit notes missing required-field boundary:\n%s", notes)
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
		if surface.ContractFragmentPosture == "" || surface.ContractFragmentPosture == "unvalidated_fragment" {
			t.Fatalf("%s has unvalidated contract fragment posture: %#v", surface.CapabilityID, surface)
		}
		if surface.MCPTool != "" && surface.MCPAction != "" && !strings.HasPrefix(surface.HostSchemaPosture, "validated_mcp_mirror") {
			t.Fatalf("%s MCP-backed surface should be a validated mirror, got %q", surface.CapabilityID, surface.HostSchemaPosture)
		}
		if surface.MCPTool != "" && surface.MCPAction != "" &&
			surface.ContractFragmentPosture != "validated_fragment" &&
			surface.ContractFragmentPosture != "generated_target_fragment" {
			t.Fatalf("%s MCP-backed fragment posture = %q", surface.CapabilityID, surface.ContractFragmentPosture)
		}
		if surface.MCPTool == "" && surface.HostSchemaPosture != "manual_cli_contract_not_generated" {
			t.Fatalf("%s CLI/manual surface posture = %q", surface.CapabilityID, surface.HostSchemaPosture)
		}
		if surface.MCPTool == "" && surface.ContractFragmentPosture != "legacy_fragment" {
			t.Fatalf("%s CLI/manual fragment posture = %q", surface.CapabilityID, surface.ContractFragmentPosture)
		}
	}
}

func TestInterfaceContractGenerationManifestListsGeneratorTargets(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())

	if report.Kind != "haft_interface_contract_generation_manifest" {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.Authority != "read_only_generation_manifest_not_host_materialization" {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.Source != "kernel_interface_catalog" {
		t.Fatalf("source = %q", report.Source)
	}
	if !strings.HasPrefix(report.SourceDigest, "sha256:") {
		t.Fatalf("source digest = %q", report.SourceDigest)
	}
	if len(report.ValidationRefs) == 0 {
		t.Fatalf("validation_refs missing from generation manifest")
	}
	if !stringSliceContains(report.ValidationRefs, "internal/cli/interface_test.go") {
		t.Fatalf("validation_refs missing interface test: %#v", report.ValidationRefs)
	}
	if report.Summary.GeneratorTargetSurfaces != 0 || report.Summary.GeneratorTargetFields != 0 {
		t.Fatalf("expected empty generator target queue after nested schema coverage: %#v", report.Summary)
	}
	if report.Summary.GeneratedPreviewFragments != report.Summary.Capabilities {
		t.Fatalf("generated preview fragments = %d, capabilities = %d", report.Summary.GeneratedPreviewFragments, report.Summary.Capabilities)
	}
	if report.Summary.GeneratedSchemaFragments == 0 {
		t.Fatalf("expected generated MCP schema fragments in generation manifest: %#v", report.Summary)
	}
	if report.Summary.BindingPreviewFragments == 0 {
		t.Fatalf("expected binding preview fragments in generation manifest: %#v", report.Summary)
	}
	if report.Summary.MaterializedCarriers == 0 {
		t.Fatalf("expected materialized carrier list in generation manifest: %#v", report.Summary)
	}
	if report.Summary.DigestGuardedCarriers != report.Summary.MaterializedCarriers {
		t.Fatalf("expected all materialized carriers source-digest guarded: %#v", report.Summary)
	}
	if report.Summary.AuthorityGuardedCarriers != report.Summary.MaterializedCarriers {
		t.Fatalf("expected all materialized carriers authority-guarded: %#v", report.Summary)
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
	if len(report.Carriers) != report.Summary.MaterializedCarriers {
		t.Fatalf("materialized carriers = %d, summary = %#v", len(report.Carriers), report.Summary)
	}
	if len(report.Fragments) != report.Summary.GeneratedPreviewFragments {
		t.Fatalf("generated fragments = %d, summary = %#v", len(report.Fragments), report.Summary)
	}
	if len(report.SchemaFragments) != report.Summary.GeneratedSchemaFragments {
		t.Fatalf("generated schema fragments = %d, summary = %#v", len(report.SchemaFragments), report.Summary)
	}
	notes := strings.Join(report.Notes, "\n")
	for _, want := range []string{"--write-schema-fragments", "--write-description-fragments", "--check-schema-fragments", "--check-description-fragments"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("contract_generation report notes missing %q:\n%s", want, notes)
		}
	}
	decide, ok := findContractGeneratedFragment(report, "decision.decide")
	if !ok {
		t.Fatal("decision.decide generated fragment missing")
	}
	for _, want := range []string{
		"kernel_interface_catalog",
		"binding actions require explicit operator/manual authorization",
		"not approval receipts",
	} {
		if !strings.Contains(decide.GeneratedText, want) && !strings.Contains(decide.AuthorityBoundary, want) {
			t.Fatalf("decision.decide generated fragment missing %q:\n%#v", want, decide)
		}
	}
	if !stringSliceContains(decide.InputFields, "choice_result") {
		t.Fatalf("decision.decide generated fragment missing choice_result input field: %#v", decide.InputFields)
	}
	decideSchema, ok := findContractGeneratedSchemaFragment(report, "decision.decide")
	if !ok {
		t.Fatal("decision.decide generated schema fragment missing")
	}
	if decideSchema.FragmentKind != "mcp_action_schema_fragment" {
		t.Fatalf("decision.decide schema fragment kind = %#v", decideSchema)
	}
	if decideSchema.MCPTool != "haft_decision" || decideSchema.MCPAction != "decide" {
		t.Fatalf("decision.decide schema fragment MCP target = %#v", decideSchema)
	}
	if !stringSliceContains(decideSchema.RequiredFields, "action") {
		t.Fatalf("decision.decide schema fragment required fields = %#v", decideSchema.RequiredFields)
	}
	if !stringSliceContains(decideSchema.ActionRequiredFields, "selected_title") {
		t.Fatalf("decision.decide schema fragment action required fields = %#v", decideSchema.ActionRequiredFields)
	}
	if !stringSliceContains(decideSchema.HandlerValidatedFields, "selected_title") {
		t.Fatalf("decision.decide schema fragment handler fields = %#v", decideSchema.HandlerValidatedFields)
	}
	if !strings.HasPrefix(decideSchema.SchemaDigest, "sha256:") {
		t.Fatalf("decision.decide schema digest = %q", decideSchema.SchemaDigest)
	}
	if !strings.Contains(decideSchema.AuthorityBoundary, "not operator authorization") {
		t.Fatalf("decision.decide schema fragment authority boundary = %q", decideSchema.AuthorityBoundary)
	}

	authorityNotes := strings.Join(report.Notes, " ")
	if !strings.Contains(authorityNotes, "not host materialization") ||
		!strings.Contains(authorityNotes, "not operator authorization") ||
		!strings.Contains(authorityNotes, "generated_schema_fragments") {
		t.Fatalf("generation manifest notes missing authority boundary:\n%s", authorityNotes)
	}
}

func TestInterfaceContractGeneratedSchemaFragmentsMatchToolsList(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	toolActionEnums := fpfToolActionEnums(t)
	toolProperties := fpfToolProperties(t)
	toolRequired := fpfToolRequiredFields(t)
	exclusions := interfaceContractAuditSchemaFieldExclusions()

	if len(report.SchemaFragments) == 0 {
		t.Fatal("expected generated schema fragments")
	}
	for _, fragment := range report.SchemaFragments {
		if !contractAuditTestContains(toolActionEnums[fragment.MCPTool], fragment.MCPAction) {
			t.Fatalf("%s generated schema action %q missing from %s enum %v", fragment.CapabilityID, fragment.MCPAction, fragment.MCPTool, toolActionEnums[fragment.MCPTool])
		}
		for _, field := range fragment.RequiredFields {
			if !toolRequired[fragment.MCPTool][field] {
				t.Fatalf("%s generated schema required field %q missing from %s required %v", fragment.CapabilityID, field, fragment.MCPTool, sortedStringSetKeys(toolRequired[fragment.MCPTool]))
			}
		}
		for _, field := range fragment.AllowedTopLevelFields {
			if exclusions[fragment.CapabilityID][field] {
				continue
			}
			if _, ok := toolProperties[fragment.MCPTool][field]; !ok {
				t.Fatalf("%s generated schema field %q missing from %s properties %s", fragment.CapabilityID, field, fragment.MCPTool, sortedMapKeys(toolProperties[fragment.MCPTool]))
			}
		}
		schemaProperties, ok := fragment.Schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s generated schema properties missing: %#v", fragment.CapabilityID, fragment.Schema)
		}
		actionProperty, ok := schemaProperties["action"].(map[string]any)
		if !ok {
			t.Fatalf("%s generated schema action property missing: %#v", fragment.CapabilityID, schemaProperties)
		}
		if actionProperty["const"] != fragment.MCPAction {
			t.Fatalf("%s generated schema action const = %#v, want %q", fragment.CapabilityID, actionProperty["const"], fragment.MCPAction)
		}
		if fragment.CapabilityID == "decision.decide" {
			selectedTitle, ok := schemaProperties["selected_title"].(map[string]any)
			if !ok {
				t.Fatalf("decision.decide generated schema selected_title property missing: %#v", schemaProperties)
			}
			if selectedTitle["type"] != "string" {
				t.Fatalf("decision.decide selected_title generated schema = %#v, want tools/list string schema", selectedTitle)
			}
		}
	}
}

func TestInterfaceContractGenerationRuntimeSchemaAuditValidatesLiveToolCatalog(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())

	if report.RuntimeAudit.Authority != "read_only_runtime_schema_validation_not_generation_authority" {
		t.Fatalf("runtime audit authority = %q", report.RuntimeAudit.Authority)
	}
	if report.RuntimeAudit.Status != "clean" {
		t.Fatalf("runtime audit status = %q, audit = %#v", report.RuntimeAudit.Status, report.RuntimeAudit)
	}
	if report.RuntimeAudit.RuntimeSchemaMirrors != report.Summary.GeneratedSchemaFragments {
		t.Fatalf("runtime mirrors = %d, summary = %#v", report.RuntimeAudit.RuntimeSchemaMirrors, report.Summary)
	}
	if report.Summary.RuntimeSchemaMirrors != report.RuntimeAudit.RuntimeSchemaMirrors {
		t.Fatalf("summary runtime mirrors = %d, audit = %d", report.Summary.RuntimeSchemaMirrors, report.RuntimeAudit.RuntimeSchemaMirrors)
	}
	if report.Summary.RuntimeSchemaDrift != 0 {
		t.Fatalf("summary runtime drift = %d, audit = %#v", report.Summary.RuntimeSchemaDrift, report.RuntimeAudit)
	}
	if len(report.RuntimeAudit.ValidationRefs) == 0 {
		t.Fatalf("runtime audit validation refs missing")
	}
}

func TestInterfaceContractGenerationRuntimeSchemaAuditDetectsFragmentDrift(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	if len(report.SchemaFragments) == 0 {
		t.Fatal("expected generated schema fragments")
	}
	report.SchemaFragments[0].SchemaDigest = "sha256:stale"

	audit := interfaceContractRuntimeSchemaAuditFor(report)

	if audit.Status != "drift" {
		t.Fatalf("runtime audit status = %q, want drift", audit.Status)
	}
	if audit.RuntimeSchemaDrift == 0 {
		t.Fatalf("runtime audit did not count drift: %#v", audit)
	}
	if !stringSliceContains(audit.SchemaDigestMismatches, report.SchemaFragments[0].CapabilityID) {
		t.Fatalf("runtime audit mismatches = %#v, want %q", audit.SchemaDigestMismatches, report.SchemaFragments[0].CapabilityID)
	}
}

func TestInterfaceContractGenerationManifestListsMaterializedCarriers(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())

	toolsCarrier, ok := findContractMaterializedCarrier(report, "packages/haft-pi/extensions/haft/tools.ts")
	if !ok {
		t.Fatalf("Pi tool carrier missing from materialized_carriers: %#v", report.Carriers)
	}
	if toolsCarrier.ExpectedSourceDigest != report.SourceDigest {
		t.Fatalf("Pi tool carrier source digest = %q, report digest = %q", toolsCarrier.ExpectedSourceDigest, report.SourceDigest)
	}
	if toolsCarrier.SyncPosture != "digest_guarded_by_repo_regression" {
		t.Fatalf("Pi tool carrier sync posture = %q", toolsCarrier.SyncPosture)
	}
	if !stringSliceContains(toolsCarrier.GeneratedFragmentRefs, "query.contract_generation") {
		t.Fatalf("Pi tool carrier missing contract_generation fragment ref: %#v", toolsCarrier.GeneratedFragmentRefs)
	}

	for _, carrier := range report.Carriers {
		source := readRepoFile(t, strings.Split(carrier.CarrierPath, "/")...)
		for _, marker := range carrier.RequiredMarkers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing required generated-contract marker %q", carrier.CarrierPath, marker)
			}
		}
		if carrier.SourceContract != "kernel_interface_catalog" {
			t.Fatalf("%s source contract = %q", carrier.CarrierPath, carrier.SourceContract)
		}
		if carrier.AuthorityBoundary == "" {
			t.Fatalf("%s missing authority boundary", carrier.CarrierPath)
		}
		if len(carrier.ValidationRefs) == 0 {
			t.Fatalf("%s missing validation refs", carrier.CarrierPath)
		}
	}
}

func TestInterfaceContractGenerationMaterializesSchemaFragmentsCarrier(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	path := filepath.Join(t.TempDir(), "generated", "mcp-schema-fragments.json")

	result, err := materializeInterfaceContractSchemaFragments(report, path)
	if err != nil {
		t.Fatalf("materialize schema fragments: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized schema fragments: %v", err)
	}
	if result.Path != path {
		t.Fatalf("materialized path = %q, want %q", result.Path, path)
	}
	if result.SourceDigest != report.SourceDigest {
		t.Fatalf("materialized source digest = %q, report digest = %q", result.SourceDigest, report.SourceDigest)
	}
	if result.CarrierDigest != interfaceContractGenerationDigestBytes(data) {
		t.Fatalf("carrier digest = %q, bytes digest = %q", result.CarrierDigest, interfaceContractGenerationDigestBytes(data))
	}
	if result.SchemaFragments != report.Summary.GeneratedSchemaFragments {
		t.Fatalf("schema fragment count = %d, summary = %#v", result.SchemaFragments, report.Summary)
	}
	if strings.Contains(string(data), "generated_at") {
		t.Fatalf("materialized schema carrier should be deterministic and omit generated_at:\n%s", string(data))
	}

	var carrier interfaceContractSchemaFragmentsCarrier
	if err := json.Unmarshal(data, &carrier); err != nil {
		t.Fatalf("decode materialized schema fragments: %v\n%s", err, string(data))
	}
	if carrier.Kind != "haft_interface_generated_mcp_schema_fragments" {
		t.Fatalf("carrier kind = %q", carrier.Kind)
	}
	if carrier.Authority != "generated_validation_carrier_not_runtime_schema_authority" {
		t.Fatalf("carrier authority = %q", carrier.Authority)
	}
	if carrier.SourceDigest != report.SourceDigest {
		t.Fatalf("carrier source digest = %q, report digest = %q", carrier.SourceDigest, report.SourceDigest)
	}
	if len(carrier.SchemaFragments) != len(report.SchemaFragments) {
		t.Fatalf("carrier schema fragments = %d, report = %d", len(carrier.SchemaFragments), len(report.SchemaFragments))
	}
	contractGeneration, ok := findCarrierGeneratedSchemaFragment(carrier, "query.contract_generation")
	if !ok {
		t.Fatalf("query.contract_generation schema fragment missing from materialized carrier")
	}
	if contractGeneration.SchemaDigest == "" || !strings.HasPrefix(contractGeneration.SchemaDigest, "sha256:") {
		t.Fatalf("contract_generation schema digest = %q", contractGeneration.SchemaDigest)
	}

	checkResult, err := checkInterfaceContractSchemaFragments(report, path)
	if err != nil {
		t.Fatalf("check materialized schema fragments: %v", err)
	}
	if !checkResult.Match {
		t.Fatalf("schema carrier check did not match: %#v", checkResult)
	}
	if err := os.WriteFile(path, append(data, []byte("\n{\"drift\":true}\n")...), 0o644); err != nil {
		t.Fatalf("corrupt materialized schema fragments: %v", err)
	}
	if _, err := checkInterfaceContractSchemaFragments(report, path); err == nil {
		t.Fatalf("expected schema carrier drift check to fail after file corruption")
	}
}

func TestInterfaceContractGenerationMaterializesDescriptionFragmentsCarrier(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	path := filepath.Join(t.TempDir(), "generated", "description-fragments.json")

	result, err := materializeInterfaceContractDescriptionFragments(report, path)
	if err != nil {
		t.Fatalf("materialize description fragments: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized description fragments: %v", err)
	}
	if result.Path != path {
		t.Fatalf("materialized path = %q, want %q", result.Path, path)
	}
	if result.SourceDigest != report.SourceDigest {
		t.Fatalf("materialized source digest = %q, report digest = %q", result.SourceDigest, report.SourceDigest)
	}
	if result.CarrierDigest != interfaceContractGenerationDigestBytes(data) {
		t.Fatalf("carrier digest = %q, bytes digest = %q", result.CarrierDigest, interfaceContractGenerationDigestBytes(data))
	}
	if result.DescriptionFragments != report.Summary.GeneratedPreviewFragments {
		t.Fatalf("description fragment count = %d, summary = %#v", result.DescriptionFragments, report.Summary)
	}
	if strings.Contains(string(data), "generated_at") {
		t.Fatalf("materialized description carrier should be deterministic and omit generated_at:\n%s", string(data))
	}

	var carrier interfaceContractDescriptionFragmentsCarrier
	if err := json.Unmarshal(data, &carrier); err != nil {
		t.Fatalf("decode materialized description fragments: %v\n%s", err, string(data))
	}
	if carrier.Kind != "haft_interface_generated_description_fragments" {
		t.Fatalf("carrier kind = %q", carrier.Kind)
	}
	if carrier.Authority != "generated_text_carrier_not_operator_authorization" {
		t.Fatalf("carrier authority = %q", carrier.Authority)
	}
	if carrier.SourceDigest != report.SourceDigest {
		t.Fatalf("carrier source digest = %q, report digest = %q", carrier.SourceDigest, report.SourceDigest)
	}
	if len(carrier.DescriptionFragments) != len(report.Fragments) {
		t.Fatalf("carrier description fragments = %d, report = %d", len(carrier.DescriptionFragments), len(report.Fragments))
	}
	contractGeneration, ok := findCarrierGeneratedDescriptionFragment(carrier, "query.contract_generation")
	if !ok {
		t.Fatalf("query.contract_generation description fragment missing from materialized carrier")
	}
	if !strings.Contains(contractGeneration.AuthorityBoundary, "discovery only") {
		t.Fatalf("contract_generation description authority boundary = %q", contractGeneration.AuthorityBoundary)
	}

	checkResult, err := checkInterfaceContractDescriptionFragments(report, path)
	if err != nil {
		t.Fatalf("check materialized description fragments: %v", err)
	}
	if !checkResult.Match {
		t.Fatalf("description carrier check did not match: %#v", checkResult)
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
		"read_only_generation_manifest_not_host_materialization",
		"source: kernel_interface_catalog sha256:",
		"generator_target_surfaces=",
		"generator_target_fields=",
		"generated_preview_fragments=",
		"generated_schema_fragments=",
		"binding_preview_fragments=",
		"materialized_carriers=",
		"digest_guarded_carriers=",
		"authority_boundary_guarded_carriers=",
		"validation_refs=",
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
	if report.Authority != "read_only_generation_manifest_not_host_materialization" {
		t.Fatalf("authority = %q", report.Authority)
	}
	if report.SurfacePolicy.GeneratedDescriptions != "drill_down_only_validate_with_carrier_semio_before_host_materialization" {
		t.Fatalf("generated description policy = %q", report.SurfacePolicy.GeneratedDescriptions)
	}
	if len(report.ValidationRefs) == 0 {
		t.Fatalf("validation_refs missing from MCP manifest")
	}
	if len(report.Targets) != 0 {
		t.Fatalf("expected empty generator target manifest from MCP query: %#v", report.Targets)
	}
	if len(report.Fragments) == 0 {
		t.Fatalf("expected generated fragments from MCP manifest")
	}
	if len(report.SchemaFragments) == 0 {
		t.Fatalf("expected generated schema fragments from MCP manifest")
	}
}

func TestPiToolMetadataCarriesGeneratedContractAuthorityBoundaries(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	source := readRepoFile(t, "packages", "haft-pi", "extensions", "haft", "tools.ts")

	decide, ok := findContractGeneratedFragment(report, "decision.decide")
	if !ok {
		t.Fatal("decision.decide generated fragment missing")
	}
	contractGeneration, ok := findContractGeneratedFragment(report, "query.contract_generation")
	if !ok {
		t.Fatal("query.contract_generation generated fragment missing")
	}

	for _, want := range []string{
		decide.AuthorityBoundary,
		contractGeneration.AuthorityBoundary,
		report.SourceDigest,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("Pi tool metadata missing generated-contract marker %q", want)
		}
	}
	if !strings.Contains(source, "contract_generation") {
		t.Fatalf("Pi tool metadata missing contract_generation action")
	}
}

func TestPiToolSchemasMirrorGeneratedSchemaFragments(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	source := readRepoFile(t, "packages", "haft-pi", "extensions", "haft", "tools.ts")
	piSchemas := make(map[string]piTypeObjectSchemaMirror)

	for _, fragment := range report.SchemaFragments {
		piSchema, ok := piSchemas[fragment.MCPTool]
		if !ok {
			piSchema = parsePiToolSchemaMirror(t, source, fragment.MCPTool)
			piSchemas[fragment.MCPTool] = piSchema
		}
		if !piSchema.Actions[fragment.MCPAction] {
			t.Fatalf("%s generated action %q missing from Pi %s action enum", fragment.CapabilityID, fragment.MCPAction, fragment.MCPTool)
		}
		for _, field := range fragment.AllowedTopLevelFields {
			if !piSchema.Fields[field] {
				t.Fatalf("%s generated field %q missing from Pi %s parameters", fragment.CapabilityID, field, fragment.MCPTool)
			}
		}
		for _, field := range fragment.RequiredFields {
			if !piSchema.RequiredFields[field] {
				t.Fatalf("%s generated required field %q is optional or missing in Pi %s parameters", fragment.CapabilityID, field, fragment.MCPTool)
			}
		}
	}
}

func parsePiToolSchemaMirror(t *testing.T, source string, tool string) piTypeObjectSchemaMirror {
	t.Helper()

	constByTool := map[string]string{
		"haft_query":        "haftQueryParameters",
		"haft_problem":      "haftProblemParameters",
		"haft_solution":     "haftSolutionParameters",
		"haft_decision":     "haftDecisionParameters",
		"haft_note":         "haftNoteParameters",
		"haft_refresh":      "haftRefreshParameters",
		"haft_method":       "haftMethodParameters",
		"haft_commission":   "haftCommissionParameters",
		"haft_spec_section": "haftSpecSectionParameters",
	}
	constName, ok := constByTool[tool]
	if !ok {
		t.Fatalf("no Pi schema const mapping for %s", tool)
	}
	return parsePiTypeObjectSchema(t, source, constName)
}

func TestPiToolMetadataCarriesSelectedGeneratedQueryFragments(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	source := readRepoFile(t, "packages", "haft-pi", "extensions", "haft", "tools.ts")

	for _, tc := range []struct {
		capabilityID string
		action       string
		carrierHint  string
	}{
		{
			capabilityID: "query.contract_generation",
			action:       "contract_generation",
			carrierHint:  "generated fragments are read-only previews",
		},
		{
			capabilityID: "query.drift_events",
			action:       "drift_events",
			carrierHint:  "drift fanout",
		},
		{
			capabilityID: "query.decision_reconcile",
			action:       "decision_reconcile",
			carrierHint:  "reconciliation",
		},
		{
			capabilityID: "query.governing_set",
			action:       "governing_set",
			carrierHint:  "current-authority drill-downs",
		},
	} {
		fragment, ok := findContractGeneratedFragment(report, tc.capabilityID)
		if !ok {
			t.Fatalf("%s generated fragment missing", tc.capabilityID)
		}
		for _, want := range []string{
			fragment.AuthorityBoundary,
			tc.action,
			tc.carrierHint,
		} {
			if !strings.Contains(source, want) {
				t.Fatalf("Pi tool metadata missing generated-fragment carrier text %q for %s", want, tc.capabilityID)
			}
		}
	}
}

func TestPiPromptCarriersCarryGeneratedContractAuthorityBoundaries(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	decide, ok := findContractGeneratedFragment(report, "decision.decide")
	if !ok {
		t.Fatal("decision.decide generated fragment missing")
	}

	source := strings.Join([]string{
		readRepoFile(t, "packages", "haft-pi", "prompts", "h-decide.md"),
		readRepoFile(t, "packages", "haft-pi", "prompts", "h-commission.md"),
		readRepoFile(t, "packages", "haft-pi", "prompts", "h-reason.md"),
	}, "\n")

	for _, want := range []string{
		decide.AuthorityBoundary,
		"operator_confirmation_required",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("Pi prompt carriers missing generated-contract authority boundary %q", want)
		}
	}
}

func TestPiPackageDocsCarryGeneratedContractAuthorityBoundary(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	contractGeneration, ok := findContractGeneratedFragment(report, "query.contract_generation")
	if !ok {
		t.Fatal("query.contract_generation generated fragment missing")
	}

	readme := readRepoFile(t, "packages", "haft-pi", "README.md")
	metadata := readRepoFile(t, "packages", "haft-pi", "package.json")

	for _, want := range []string{
		"haft interface contract-generation --json",
		"haft_query(action=\"contract_generation\")",
		contractGeneration.AuthorityBoundary,
		"provider tool-calling ergonomics, not as a second authority",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("Pi README missing generated-contract boundary %q", want)
		}
	}
	if !strings.Contains(metadata, "kernel-validated prompt guidance") {
		t.Fatalf("Pi package metadata missing kernel-validated contract hint")
	}
}

func TestPiSkillCarriersCarrySelectedGeneratedQueryFragments(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	source := readRepoFile(t, "packages", "haft-pi", "skills", "h-status", "SKILL.md")

	for _, tc := range []struct {
		capabilityID string
		action       string
		carrierHint  string
	}{
		{
			capabilityID: "query.contract_generation",
			action:       "contract_generation",
			carrierHint:  "generated-fragment",
		},
		{
			capabilityID: "query.drift_events",
			action:       "drift_events",
			carrierHint:  "drift fanout",
		},
		{
			capabilityID: "query.decision_reconcile",
			action:       "decision_reconcile",
			carrierHint:  "reconciliation",
		},
		{
			capabilityID: "query.governing_set",
			action:       "governing_set",
			carrierHint:  "current-authority drill-downs",
		},
	} {
		fragment, ok := findContractGeneratedFragment(report, tc.capabilityID)
		if !ok {
			t.Fatalf("%s generated fragment missing", tc.capabilityID)
		}
		for _, want := range []string{
			fragment.AuthorityBoundary,
			tc.action,
			tc.carrierHint,
		} {
			if !strings.Contains(source, want) {
				t.Fatalf("Pi skill carrier missing generated-fragment carrier text %q for %s", want, tc.capabilityID)
			}
		}
	}
}

func TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	decide, ok := findContractGeneratedFragment(report, "decision.decide")
	if !ok {
		t.Fatal("decision.decide generated fragment missing")
	}

	source := strings.Join([]string{
		readRepoFile(t, "internal", "cli", "skill", "h-decide", "SKILL.md"),
		readRepoFile(t, "internal", "cli", "skill", "h-commission", "SKILL.md"),
		readRepoFile(t, "internal", "cli", "skill", "h-reason", "SKILL.md"),
		readRepoFile(t, "internal", "cli", "claude_md_template.md"),
		readRepoFile(t, "CLAUDE.md"),
		readRepoFile(t, "AGENTS.md"),
	}, "\n")

	for _, want := range []string{
		decide.AuthorityBoundary,
		"operator_confirmation_required",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("bundled skill/template carriers missing generated-contract authority boundary %q", want)
		}
	}
}

func TestBundledSkillCarriersCarrySelectedGeneratedQueryFragments(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	source := strings.Join([]string{
		readRepoFile(t, "internal", "cli", "skill", "h-status", "SKILL.md"),
		readRepoFile(t, "internal", "cli", "skill", "h-verify", "SKILL.md"),
		readRepoFile(t, "internal", "cli", "skill", "h-reason", "SKILL.md"),
	}, "\n")

	for _, tc := range []struct {
		capabilityID string
		action       string
		carrierHint  string
	}{
		{
			capabilityID: "query.contract_generation",
			action:       "contract_generation",
			carrierHint:  "generated-fragment",
		},
		{
			capabilityID: "query.drift_events",
			action:       "drift_events",
			carrierHint:  "drift fanout",
		},
		{
			capabilityID: "query.decision_reconcile",
			action:       "decision_reconcile",
			carrierHint:  "reconciliation",
		},
		{
			capabilityID: "query.governing_set",
			action:       "governing_set",
			carrierHint:  "current-authority drill-downs",
		},
	} {
		fragment, ok := findContractGeneratedFragment(report, tc.capabilityID)
		if !ok {
			t.Fatalf("%s generated fragment missing", tc.capabilityID)
		}
		for _, want := range []string{
			fragment.AuthorityBoundary,
			tc.action,
			tc.carrierHint,
		} {
			if !strings.Contains(source, want) {
				t.Fatalf("bundled skill carriers missing generated-fragment carrier text %q for %s", want, tc.capabilityID)
			}
		}
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
		"read_only_generation_manifest_not_host_materialization",
		"generator_target_surfaces",
		"generator_target_fields",
		"generated_preview_fragments",
		"generated_schema_fragments",
		"runtime_schema_audit",
		"runtime_schema_drift",
		"generated_fragments",
		"materialized_carriers",
		"digest_guarded_carriers",
		"authority_boundary_guarded_carriers",
		"surface_policy",
	} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("default status inlined contract generation manifest fragment %q:\n%s", forbidden, result)
		}
	}
	assertNoContractAuditInline(t, "default status", result)
	assertNoInterfaceOutputShapeInline(t, "default status", result)
}

func TestInterfaceContractGenerationCompactTextDoesNotInlineRuntimeSchemaAudit(t *testing.T) {
	report := buildInterfaceContractGenerationReport(haftInterfaceCatalog())
	var text strings.Builder

	if err := writeInterfaceContractGenerationText(&text, report); err != nil {
		t.Fatalf("write contract generation text: %v", err)
	}

	body := text.String()
	for _, want := range []string{"runtime_schema_mirrors=", "runtime_schema_drift=0"} {
		if !strings.Contains(body, want) {
			t.Fatalf("compact contract generation text missing count %q:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{
		"runtime_schema_audit",
		"missing_runtime_tools",
		"schema_digest_mismatches",
		"live_mcp_tools_list_tool_catalog",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("compact contract generation text inlined runtime audit detail %q:\n%s", forbidden, body)
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
	if !strings.Contains(shape, "validation_refs") {
		t.Fatalf("contract generation discovery fields missing validation_refs: %s", shape)
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

func TestInterfaceEvidencePathNamesFormalityDiagnostics(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.evidence_path")
	if !ok {
		t.Fatal("query.evidence_path capability missing")
	}

	shapes, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{
		"formality_diagnostics",
		"legacy_formality_projection_lossy",
		"unversioned_formality_source_scale_missing",
		"current_f0_f9_formality",
		"claim_truth",
		"not_claim_truth",
		"publication",
		"not_publication",
	} {
		if !strings.Contains(shapes, want) {
			t.Fatalf("query.evidence_path shapes missing %q:\n%s", want, shapes)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{
		"blocks bounded reliance",
		"legacy or undeclared/lossy formality",
		"publication",
	} {
		if !strings.Contains(notes, want) {
			t.Fatalf("query.evidence_path notes missing %q:\n%s", want, notes)
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

func TestInterfaceCatalogMCPRequiredFieldsExistInToolsListSchemas(t *testing.T) {
	toolRequired := fpfToolRequiredFields(t)

	for _, capability := range haftInterfaceCatalog() {
		toolName := capability.CurrentExecution.MCPTool
		if toolName == "" || capability.CurrentExecution.MCPAction == "" {
			continue
		}

		required, ok := toolRequired[toolName]
		if !ok {
			t.Fatalf("%s declares unknown MCP tool %q", capability.ID, toolName)
		}
		for _, field := range interfaceContractAuditExpectedMCPRequiredFields(capability) {
			if !required[field] {
				t.Fatalf("%s expects %s tools/list schema to require %q; required=%v", capability.ID, toolName, field, sortedStringSetKeys(required))
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
		"required_coverage=",
		"shape_coverage=",
		"generator_targets=",
		"host_fragments=",
		"fragment_posture=",
		"host_schema=",
		"fragment=",
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

func findContractGeneratedFragment(report interfaceContractGenerationReport, id string) (interfaceContractGeneratedFragment, bool) {
	for _, fragment := range report.Fragments {
		if fragment.CapabilityID == id {
			return fragment, true
		}
	}
	return interfaceContractGeneratedFragment{}, false
}

func findContractMaterializedCarrier(report interfaceContractGenerationReport, path string) (interfaceContractMaterializedCarrier, bool) {
	for _, carrier := range report.Carriers {
		if carrier.CarrierPath == path {
			return carrier, true
		}
	}
	return interfaceContractMaterializedCarrier{}, false
}

func findContractGeneratedSchemaFragment(report interfaceContractGenerationReport, id string) (interfaceContractGeneratedSchemaFragment, bool) {
	for _, fragment := range report.SchemaFragments {
		if fragment.CapabilityID == id {
			return fragment, true
		}
	}
	return interfaceContractGeneratedSchemaFragment{}, false
}

func findCarrierGeneratedSchemaFragment(carrier interfaceContractSchemaFragmentsCarrier, id string) (interfaceContractGeneratedSchemaFragment, bool) {
	for _, fragment := range carrier.SchemaFragments {
		if fragment.CapabilityID == id {
			return fragment, true
		}
	}
	return interfaceContractGeneratedSchemaFragment{}, false
}

func findCarrierGeneratedDescriptionFragment(carrier interfaceContractDescriptionFragmentsCarrier, id string) (interfaceContractGeneratedFragment, bool) {
	for _, fragment := range carrier.DescriptionFragments {
		if fragment.CapabilityID == id {
			return fragment, true
		}
	}
	return interfaceContractGeneratedFragment{}, false
}

func readRepoFile(t *testing.T, elem ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{"..", ".."}, elem...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repo file %s: %v", path, err)
	}
	return string(data)
}

type piTypeObjectSchemaMirror struct {
	Actions        map[string]bool
	Fields         map[string]bool
	RequiredFields map[string]bool
}

func parsePiTypeObjectSchema(t *testing.T, source string, constName string) piTypeObjectSchemaMirror {
	t.Helper()

	bodyPattern := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(constName) + `\s*=\s*Type\.Object\(\{(.*?)\n\}\);`)
	matches := bodyPattern.FindStringSubmatch(source)
	if len(matches) != 2 {
		t.Fatalf("Pi Type.Object schema %s not found", constName)
	}

	fieldPattern := regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*):\s*([^\n]+)`)
	fields := map[string]bool{}
	requiredFields := map[string]bool{}
	for _, match := range fieldPattern.FindAllStringSubmatch(matches[1], -1) {
		field := match[1]
		value := strings.TrimSpace(match[2])
		fields[field] = true
		if strings.HasPrefix(value, "Opt") || strings.HasPrefix(value, "Type.Optional") {
			continue
		}
		requiredFields[field] = true
	}
	if len(fields) == 0 {
		t.Fatalf("Pi Type.Object schema %s has no fields", constName)
	}

	actionPattern := regexp.MustCompile(`(?s)action:\s*enumOf\((.*?)\),`)
	actionMatches := actionPattern.FindStringSubmatch(matches[1])
	if len(actionMatches) != 2 {
		t.Fatalf("Pi Type.Object schema %s action enum not found", constName)
	}
	actionValuePattern := regexp.MustCompile(`"([^"]+)"`)
	actions := map[string]bool{}
	for _, match := range actionValuePattern.FindAllStringSubmatch(actionMatches[1], -1) {
		actions[match[1]] = true
	}
	if len(actions) == 0 {
		t.Fatalf("Pi Type.Object schema %s action enum is empty", constName)
	}

	return piTypeObjectSchemaMirror{
		Actions:        actions,
		Fields:         fields,
		RequiredFields: requiredFields,
	}
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

func fpfToolRequiredFields(t *testing.T) map[string]map[string]bool {
	t.Helper()

	server := fpf.NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})

	toolRequired := make(map[string]map[string]bool)
	for _, tool := range server.ToolCatalog() {
		inputSchema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			t.Fatalf("%s input schema has wrong type: %#v", tool.Name, tool.InputSchema)
		}
		toolRequired[tool.Name] = stringSetFromSchemaRequired(inputSchema["required"])
	}

	return toolRequired
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
	for _, want := range []string{"host authorization receipts require principal, session, action, payload hash, expiry, source, and a registered kernel verifier", "default MCP cli-only mode"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("decision interface notes missing host receipt boundary %q:\n%s", want, notes)
		}
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
	if !containsString(capability.InputContract.OptionalFields, "limit") || !containsString(capability.InputContract.OptionalFields, "full") {
		t.Fatalf("governing_set optional fields = %#v, want limit and full", capability.InputContract.OptionalFields)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision governing-set") ||
		!strings.Contains(capability.CurrentExecution.CLICommand, "--query") {
		t.Fatalf("governing_set CLI command missing governing-set drill-down:\n%#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "--write-snapshot") ||
		!strings.Contains(capability.CurrentExecution.CLICommand, "--check-snapshot") {
		t.Fatalf("governing_set CLI command missing snapshot write/check:\n%#v", capability.CurrentExecution)
	}

	fieldShapes := ""
	for _, shape := range capability.InputContract.FieldShapes {
		fieldShapes += shape.Field + " " + shape.Shape + " " + shape.Note + " "
	}
	for _, want := range []string{"read_only_current_authority_frontier", "authority_frontier", "current_decision_refs_are_governing_authority_terminal_history_refs_are_not", "single_current_authority", "conflict_requires_operator", "fallback_target_sets", "scope_enrichment_sets", "whole_file_fallback_requires_scope_enrichment", "scope_repair_hints", "derived_read_only_not_gate_decision", "Terminal decisions are history refs", "bearer_ref", "source_refs", "--subject-ref", "--target-ref", "limit caps compact sets", "limit=5", "read_only_current_governing_frontier_snapshot_check", "snapshot_digest", "Snapshot carriers are comparison aids"} {
		if !strings.Contains(fieldShapes, want) {
			t.Fatalf("governing_set field shapes missing %q:\n%s", want, fieldShapes)
		}
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	for _, want := range []string{"active/refresh_due", "fallback_target_sets", "scope_enrichment_sets", "Read-only", "does not supersede", "focused drill-down", "--write-snapshot", "--check-snapshot"} {
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
	for _, want := range []string{"operational_gate", "current_authority", "require_current_source_and_admitted_use", "current_authority_conflict_requires_operator", "passed|blocked"} {
		if !strings.Contains(shapes, want) {
			t.Fatalf("query.spec_use contract missing %q:\n%s", want, shapes)
		}
	}
	invariants := strings.Join(capability.Invariants, "\n")
	if !strings.Contains(invariants, "GateDecision remains a derived reading") {
		t.Fatalf("query.spec_use invariants missing derived-reading boundary:\n%s", invariants)
	}
	if !strings.Contains(invariants, "Current-authority conflict posture") {
		t.Fatalf("query.spec_use invariants missing current-authority boundary:\n%s", invariants)
	}
}

func TestInterfaceDriftEventsDocumentsFanoutBoundary(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.drift_events")
	if !ok {
		t.Fatal("query.drift_events capability missing")
	}

	if capability.CurrentExecution.MCPCall != `haft_query(action="drift_events", limit=5)` {
		t.Fatalf("drift_events MCP call = %#v", capability.CurrentExecution)
	}
	if !containsString(capability.InputContract.OptionalFields, "limit") || !containsString(capability.InputContract.OptionalFields, "full") {
		t.Fatalf("drift_events optional fields = %#v, want limit and full", capability.InputContract.OptionalFields)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft drift events --json") {
		t.Fatalf("drift_events CLI command missing: %#v", capability.CurrentExecution)
	}
	shapes, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{"schema_version", "unique_events", "impacted_decisions", "omitted_impacted_decisions", "needs_binding_resolution_events", "semantic_target_events", "file_fallback_events", "unknown_high_risk_events", "root_cause", "semantic_target_changed", "target_renamed", "retarget_candidate", "implementation_footprint_churn", "schema_changed", "target_status", "modified|removed|renamed|retarget_candidate", "edited_symbol_move_candidate", "needs_scope_enrichment", "suggested_next_command", "haft decision reconcile --json", "haft_refresh(action=", "fallback_kind", "fallback_reason", "max_fanout", "compatibility_reports", "resolution_record", "resolution_record_posture", "stale_event_binding", "inactive_waiver", "materiality", "audit_only", "limit caps compact events", "inlined impacted_decisions", "limit=5", "complete impacted_decisions"} {
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

	if capability.CurrentExecution.MCPCall != `haft_query(action="decision_reconcile", limit=5)` {
		t.Fatalf("decision_reconcile MCP call = %#v", capability.CurrentExecution)
	}
	if !containsString(capability.InputContract.OptionalFields, "limit") || !containsString(capability.InputContract.OptionalFields, "full") {
		t.Fatalf("decision_reconcile optional fields = %#v, want limit and full", capability.InputContract.OptionalFields)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile --json") {
		t.Fatalf("decision_reconcile CLI command missing: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile metrics --json") {
		t.Fatalf("decision_reconcile metrics CLI command missing: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile selection-draft --json") {
		t.Fatalf("decision_reconcile selection-draft CLI command missing: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile selection-draft --write-template selection.json --json") {
		t.Fatalf("decision_reconcile selection-draft write-template CLI command missing: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile selection-draft --json --full") {
		t.Fatalf("decision_reconcile full selection-draft CLI command missing: %#v", capability.CurrentExecution)
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
		"lineage_relations",
		"mergedFrom/supersedes/retiredWithSuccessor/retiredWithoutSuccessor",
		"preview is advisory and cannot authorize apply",
		"downstream_impact",
		"does not relink downstream artifacts",
		"read_only_reconciliation_metrics_not_binding_authority",
		"capture_before_and_after_operator_approved_reconciliation_apply",
		"fallback_target_sets",
		"max_fanout",
		"report_only_selection_draft_not_operator_approval",
		"emitted_candidates",
		"omitted_candidates",
		"full_audit_command",
		"selection-draft --json --full",
		"candidate_posture",
		"confidence",
		"suggested_review_action",
		"blocking_questions",
		"selection_document_template",
		"--write-template selection.json",
		"writes the bounded selection_document_template to a file",
		"operator_approval_ref",
		"TODO_operator_reviewed_scope_enrichment_reason",
		"not apply-ready",
		"Selection drafts are read-only review aids",
		"Default output is bounded",
		"does not create approval",
		"limit caps compact groups",
		"limit=5",
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
	if !strings.Contains(capability.CurrentExecution.CLICommand, "haft decision reconcile selection-review") {
		t.Fatalf("CLI command missing selection-review preflight: %#v", capability.CurrentExecution)
	}
	if !strings.Contains(capability.CurrentExecution.MCPCall, "MCP has no apply action") {
		t.Fatalf("MCP call must name no-apply boundary: %#v", capability.CurrentExecution)
	}

	contract, _ := marshalContractFragments(t, capability.InputContract)
	for _, want := range []string{
		"operator_approved_reconciliation_selection",
		"operator_approval_ref",
		"merge_through_successor",
		"claim_lifecycle_update",
		"claim_lifecycle_updates",
		"lifecycle_status",
		"keeps the parent DecisionRecord current",
		"read_only_selection_review_not_apply_authority",
		"review does not create operator approval",
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
		"Selection review is read-only",
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
