package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWorkflow_Success(t *testing.T) {
	workflow, err := ParseWorkflow(ExampleWorkflowMarkdown())
	if err != nil {
		t.Fatalf("ParseWorkflow returned error: %v", err)
	}

	if workflow.Intent == "" {
		t.Fatal("expected intent section")
	}
	if workflow.Defaults.Mode != "standard" {
		t.Fatalf("defaults.mode = %q", workflow.Defaults.Mode)
	}
	if !workflow.Defaults.RequireDecision {
		t.Fatal("expected require_decision=true")
	}
	if len(workflow.PathPolicies) != 2 {
		t.Fatalf("path policies = %d, want 2", len(workflow.PathPolicies))
	}
	if workflow.PathPolicies[0].Path != "internal/artifact/**" {
		t.Fatalf("first policy path = %q", workflow.PathPolicies[0].Path)
	}
}

func TestParseWorkflow_RejectsInvalidMode(t *testing.T) {
	_, err := ParseWorkflow(strings.Replace(
		ExampleWorkflowMarkdown(),
		"mode: standard",
		"mode: autopilot",
		1,
	))
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
}

func TestLoadWorkflow_ReadsProjectFile(t *testing.T) {
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("mkdir .haft: %v", err)
	}

	path := WorkflowPath(haftDir)
	if err := os.WriteFile(path, []byte(ExampleWorkflowMarkdown()), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	workflow, err := LoadWorkflow(projectRoot)
	if err != nil {
		t.Fatalf("LoadWorkflow returned error: %v", err)
	}
	if workflow == nil {
		t.Fatal("expected workflow")
	}
}

func TestWorkflowPromptPrefix_UsesIntentAndDefaults(t *testing.T) {
	workflow, err := ParseWorkflow(ExampleWorkflowMarkdown())
	if err != nil {
		t.Fatalf("ParseWorkflow returned error: %v", err)
	}

	prefix := workflow.PromptPrefix()
	wantFragments := []string{
		"## Project Workflow",
		"Intent:",
		"Defaults:",
		"- mode: standard",
		"- require_decision: true",
		"- require_verify: true",
		"- allow_autonomy: false",
	}

	for _, fragment := range wantFragments {
		if !strings.Contains(prefix, fragment) {
			t.Fatalf("prompt prefix missing %q\n%s", fragment, prefix)
		}
	}
}
