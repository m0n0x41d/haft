package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const workflowFile = "workflow.md"

type Workflow struct {
	Intent       string
	Defaults     WorkflowDefaults
	PathPolicies []WorkflowPathPolicy
	Exceptions   string
}

type WorkflowDefaults struct {
	Mode            string `yaml:"mode"`
	RequireDecision bool   `yaml:"require_decision"`
	RequireVerify   bool   `yaml:"require_verify"`
	AllowAutonomy   bool   `yaml:"allow_autonomy"`
}

type WorkflowPathPolicy struct {
	Path            string `yaml:"path"`
	Mode            string `yaml:"mode,omitempty"`
	RequireDecision *bool  `yaml:"require_decision,omitempty"`
	RequireVerify   *bool  `yaml:"require_verify,omitempty"`
	AllowAutonomy   *bool  `yaml:"allow_autonomy,omitempty"`
}

func WorkflowPath(haftDir string) string {
	return filepath.Join(haftDir, workflowFile)
}

func LoadWorkflow(projectRoot string) (*Workflow, error) {
	path := WorkflowPath(filepath.Join(projectRoot, ".haft"))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflow: %w", err)
	}

	return ParseWorkflow(string(data))
}

func ParseWorkflow(content string) (*Workflow, error) {
	sections := splitWorkflowSections(content)
	workflow := &Workflow{
		Intent:     strings.TrimSpace(sections["Intent"]),
		Exceptions: strings.TrimSpace(sections["Exceptions"]),
	}

	defaultsBlock := firstFencedBlock(sections["Defaults"])
	if defaultsBlock == "" {
		return nil, fmt.Errorf("workflow defaults block is required")
	}
	if err := yaml.Unmarshal([]byte(defaultsBlock), &workflow.Defaults); err != nil {
		return nil, fmt.Errorf("parse workflow defaults: %w", err)
	}
	if err := validateWorkflowDefaults(workflow.Defaults); err != nil {
		return nil, err
	}

	pathPoliciesBlock := firstFencedBlock(sections["Path Policies"])
	if pathPoliciesBlock != "" {
		if err := yaml.Unmarshal([]byte(pathPoliciesBlock), &workflow.PathPolicies); err != nil {
			return nil, fmt.Errorf("parse workflow path policies: %w", err)
		}
	}
	if err := validateWorkflowPathPolicies(workflow.PathPolicies); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (w *Workflow) PromptPrefix() string {
	if w == nil {
		return ""
	}

	lines := []string{
		"## Project Workflow",
	}

	if w.Intent != "" {
		lines = append(lines, "")
		lines = append(lines, "Intent:")
		lines = append(lines, w.Intent)
	}

	lines = append(lines, "")
	lines = append(lines, "Defaults:")
	lines = append(lines, fmt.Sprintf("- mode: %s", strings.TrimSpace(w.Defaults.Mode)))
	lines = append(lines, fmt.Sprintf("- require_decision: %t", w.Defaults.RequireDecision))
	lines = append(lines, fmt.Sprintf("- require_verify: %t", w.Defaults.RequireVerify))
	lines = append(lines, fmt.Sprintf("- allow_autonomy: %t", w.Defaults.AllowAutonomy))

	if len(w.PathPolicies) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Path policies: %d rule(s) declared in .haft/workflow.md. Apply them when touched files match.", len(w.PathPolicies)))
	}

	return strings.Join(lines, "\n")
}

func ExampleWorkflowMarkdown() string {
	return strings.Join([]string{
		"# Workflow",
		"",
		"## Intent",
		"",
		"Haft should bias toward small reversible changes, require explicit decisions for core/domain edits,",
		"and always verify behavior with tests or concrete runtime evidence before calling work complete.",
		"",
		"## Defaults",
		"",
		"```yaml",
		"mode: standard",
		"require_decision: true",
		"require_verify: true",
		"allow_autonomy: false",
		"```",
		"",
		"## Path Policies",
		"",
		"```yaml",
		"- path: \"internal/artifact/**\"",
		"  mode: standard",
		"  require_decision: true",
		"  require_verify: true",
		"- path: \"desktop/**\"",
		"  mode: tactical",
		"  require_decision: false",
		"  require_verify: true",
		"```",
		"",
		"## Exceptions",
		"",
		"Use tactical mode for narrow test-only fixes or low-risk docs updates. When a change touches a",
		"policy-heavy path, keep the decision explicit even if the code delta is small.",
		"",
	}, "\n")
}

func splitWorkflowSections(content string) map[string]string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	sections := make(map[string]*strings.Builder)
	current := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			sections[current] = &strings.Builder{}
			continue
		}
		if current == "" {
			continue
		}
		sections[current].WriteString(line)
		sections[current].WriteString("\n")
	}

	result := make(map[string]string, len(sections))
	for key, value := range sections {
		result[key] = value.String()
	}
	return result
}

func firstFencedBlock(section string) string {
	if section == "" {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n")
	inBlock := false
	var block strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inBlock {
				inBlock = true
				continue
			}
			break
		}
		if !inBlock {
			continue
		}
		block.WriteString(line)
		block.WriteString("\n")
	}

	return strings.TrimSpace(block.String())
}

func validateWorkflowDefaults(defaults WorkflowDefaults) error {
	if err := validateWorkflowMode(defaults.Mode, "defaults.mode"); err != nil {
		return err
	}
	return nil
}

func validateWorkflowPathPolicies(policies []WorkflowPathPolicy) error {
	for index, policy := range policies {
		if strings.TrimSpace(policy.Path) == "" {
			return fmt.Errorf("path_policies[%d].path is required", index)
		}
		if err := validateWorkflowMode(policy.Mode, fmt.Sprintf("path_policies[%d].mode", index)); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkflowMode(mode string, field string) error {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "", "tactical", "standard", "deep":
		return nil
	default:
		return fmt.Errorf("%s must be tactical, standard, or deep", field)
	}
}
