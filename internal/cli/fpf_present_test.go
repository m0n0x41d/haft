package cli

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/present"
)

func TestCLIAndMCPFPFSearchStayAligned(t *testing.T) {
	results := []present.FPFSearchResult{
		{
			PatternID: "A.6",
			Heading:   "Signature Stack & Boundary Discipline",
			Tier:      "pattern",
			Reason:    "exact pattern id",
			Content:   "Boundary routing body",
		},
		{
			PatternID: "A.6.B",
			Heading:   "Boundary Norm Square",
			Tier:      "route",
			Reason:    "Boundary discipline and routing",
			Content:   "Norm square body",
		},
	}

	cliOutput := formatCLIFPFSearch(results)
	mcpOutput := formatMCPFPFSearch(results)

	if cliOutput != mcpOutput {
		t.Fatalf("expected CLI and MCP search output to match\nCLI:\n%s\nMCP:\n%s", cliOutput, mcpOutput)
	}
	if strings.Contains(cliOutput, "tier:") {
		t.Fatalf("expected shared output to suppress metadata by default, got:\n%s", cliOutput)
	}
	if strings.Contains(cliOutput, "## FPF Spec:") {
		t.Fatalf("expected shared output to avoid extra headers, got:\n%s", cliOutput)
	}
	checks := []string{
		"### 1. A.6 — Signature Stack & Boundary Discipline",
		"Boundary routing body",
		"### 2. A.6.B — Boundary Norm Square",
		"Norm square body",
	}
	for _, check := range checks {
		if !strings.Contains(cliOutput, check) {
			t.Fatalf("expected shared output to contain %q, got:\n%s", check, cliOutput)
		}
	}
}

func TestSharedFPFSearchEmptyMessageMatches(t *testing.T) {
	cliOutput := formatCLIFPFSearch(nil)
	mcpOutput := formatMCPFPFSearch(nil)

	if cliOutput != "No results found.\n" {
		t.Fatalf("unexpected CLI empty output %q", cliOutput)
	}
	if cliOutput != mcpOutput {
		t.Fatalf("expected empty-state parity, got CLI=%q MCP=%q", cliOutput, mcpOutput)
	}
}

func TestAgentFPFSearchKeepsCompactDefaultShape(t *testing.T) {
	results := []present.FPFSearchResult{
		{
			PatternID: "A.6",
			Heading:   "Signature Stack & Boundary Discipline",
			Tier:      "pattern",
			Reason:    "exact pattern id",
			Content:   "Boundary routing body",
		},
	}

	output := formatAgentFPFSearch("A.6", results)

	if strings.Contains(output, "tier:") {
		t.Fatalf("expected agent output to hide metadata, got:\n%s", output)
	}
	if strings.Contains(output, "### 1.") {
		t.Fatalf("expected agent output to stay unnumbered, got:\n%s", output)
	}
	if !strings.Contains(output, "### A.6 — Signature Stack & Boundary Discipline") {
		t.Fatalf("expected agent heading output, got:\n%s", output)
	}
}
