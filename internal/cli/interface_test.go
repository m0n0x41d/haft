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

	for _, want := range []string{"decision.decide", "note.record", "query.status", "refresh.scan"} {
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

	invariants := strings.Join(capability.Invariants, " ")
	if !strings.Contains(invariants, "Human binding remains mandatory") {
		t.Fatalf("decision interface must preserve manual binding invariant:\n%s", invariants)
	}
}

func TestInterfaceCodeContextNamesCompactAndFullModes(t *testing.T) {
	capability, ok := findInterfaceCapability(haftInterfaceCatalog(), "query.code_context")
	if !ok {
		t.Fatal("query.code_context capability missing")
	}

	outputVolume := strings.Join(capability.OutputVolume, " ")
	if !strings.Contains(outputVolume, "default: compact") {
		t.Fatalf("code_context interface should name compact default:\n%s", outputVolume)
	}
	if !strings.Contains(outputVolume, "full=true") {
		t.Fatalf("code_context interface should name full=true recovery:\n%s", outputVolume)
	}
}
