package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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

	for _, want := range []string{"problem.characterize", "decision.decide", "note.record", "method.pull", "method.close", "method.status", "query.status", "query.related", "refresh.scan", "refresh.review", "refresh.drain"} {
		if !ids[want] {
			t.Fatalf("catalog missing capability %q in %#v", want, response.Capabilities)
		}
	}
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

	fieldShapes := ""
	for _, shape := range capability.InputContract.FieldShapes {
		fieldShapes += shape.Field + " " + shape.Shape + " " + shape.Note + " "
	}
	for _, want := range []string{"choice_result", "option_set", "comparison_basis", "choice_rule", "next_move", "transformation_record", "transformed_entity", "initial_state", "post_state", "relation", "context"} {
		if !strings.Contains(fieldShapes, want) {
			t.Fatalf("decision field shapes missing %q:\n%s", want, fieldShapes)
		}
	}

	invariants := strings.Join(capability.Invariants, " ")
	if !strings.Contains(invariants, "Human binding remains mandatory") {
		t.Fatalf("decision interface must preserve manual binding invariant:\n%s", invariants)
	}

	notes := strings.Join(capability.InputContract.Notes, " ")
	if !strings.Contains(notes, "C.11 subject, option_set, comparison_basis, choice_rule, and next_move") {
		t.Fatalf("decision interface notes missing C.11 choice_result warning:\n%s", notes)
	}
	if !strings.Contains(notes, "not a MethodRun, WorkCommission, evidence item, or publication unit") {
		t.Fatalf("decision interface notes missing transformation separation warning:\n%s", notes)
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
