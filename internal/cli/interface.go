package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var interfaceJSON bool

var interfaceCmd = &cobra.Command{
	Use:   "interface [capability]",
	Short: "Show compact Haft interface contracts",
	Long: `Show compact, machine-readable Haft interface contracts.

Use this instead of pasting long MCP schemas or CLI help into an agent
session. Artifact execution still goes through the MCP tool named in the
contract until the input-file artifact CLI is shipped.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInterface,
}

type interfaceCatalogResponse struct {
	Kind         string                       `json:"kind"`
	Version      int                          `json:"version"`
	Capabilities []interfaceCapabilitySummary `json:"capabilities"`
}

type interfaceCapabilitySummary struct {
	ID      string `json:"id"`
	Purpose string `json:"purpose"`
}

type interfaceCapability struct {
	ID               string             `json:"id"`
	Purpose          string             `json:"purpose"`
	CurrentExecution interfaceExecution `json:"current_execution"`
	InputContract    interfaceContract  `json:"input_contract"`
	OutputVolume     []string           `json:"output_volume,omitempty"`
	Invariants       []string           `json:"invariants"`
}

type interfaceExecution struct {
	MCPTool          string   `json:"mcp_tool"`
	MCPAction        string   `json:"mcp_action,omitempty"`
	MCPCall          string   `json:"mcp_call"`
	CLIStatus        string   `json:"cli_status"`
	CLICommand       string   `json:"cli_command,omitempty"`
	DiscoveryCommand string   `json:"discovery_command"`
	InputFileFlow    []string `json:"input_file_flow,omitempty"`
}

type interfaceContract struct {
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

func init() {
	interfaceCmd.Flags().BoolVar(&interfaceJSON, "json", false, "print structured JSON output")
	rootCmd.AddCommand(interfaceCmd)
}

func runInterface(cmd *cobra.Command, args []string) error {
	catalog := haftInterfaceCatalog()
	output := cmd.OutOrStdout()

	if len(args) == 0 {
		if interfaceJSON {
			return writeInterfaceCatalogJSON(output, catalog)
		}
		return writeInterfaceCatalogText(output, catalog)
	}

	capabilityID := strings.TrimSpace(args[0])
	capability, ok := findInterfaceCapability(catalog, capabilityID)
	if !ok {
		return fmt.Errorf("unknown capability %q — run `haft interface --json` to list available capabilities", capabilityID)
	}

	if interfaceJSON {
		return writeJSON(output, capability)
	}

	return writeInterfaceCapabilityText(output, capability)
}

func haftInterfaceCatalog() []interfaceCapability {
	return []interfaceCapability{
		{
			ID:      "problem.frame",
			Purpose: "Create a ProblemCard after framing the signal, constraints, and acceptance criteria.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_problem",
				MCPAction:        "frame",
				MCPCall:          `haft_problem(action="frame", ...)`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft artifact create problem.frame --input-file input.json --json",
				DiscoveryCommand: "haft interface problem.frame --json",
				InputFileFlow: []string{
					"haft interface problem.frame --json",
					"write input JSON with required fields",
					"haft artifact create problem.frame --input-file input.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"title", "signal"},
				OptionalFields: []string{"problem_type", "constraints", "optimization_targets", "observation_indicators", "acceptance", "blast_radius", "reversibility", "context", "mode", "task_context"},
				Notes:          []string{"Frame before exploring when the problem is fuzzy or a solution was proposed before acceptance criteria."},
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "solution.explore",
			Purpose: "Create a SolutionPortfolio with 2+ genuinely distinct variants and weakest links.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_solution",
				MCPAction:        "explore",
				MCPCall:          `haft_solution(action="explore", ...)`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft artifact create solution.explore --input-file input.json --json",
				DiscoveryCommand: "haft interface solution.explore --json",
				InputFileFlow: []string{
					"haft interface solution.explore --json",
					"write input JSON with variants[]",
					"haft artifact create solution.explore --input-file input.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"variants[].title", "variants[].weakest_link", "variants[].novelty_marker"},
				OptionalFields: []string{"problem_ref", "variants[].description", "variants[].strengths", "variants[].risks", "variants[].stepping_stone", "variants[].stepping_stone_basis", "no_stepping_stone_rationale", "context", "mode", "task_context"},
				Notes:          []string{"At least one variant should be a stepping stone, or explain why no stepping-stone variant exists."},
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "solution.compare",
			Purpose: "Attach parity comparison results to a SolutionPortfolio.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_solution",
				MCPAction:        "compare",
				MCPCall:          `haft_solution(action="compare", ...)`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft artifact create solution.compare --input-file input.json --json",
				DiscoveryCommand: "haft interface solution.compare --json",
				InputFileFlow: []string{
					"haft interface solution.compare --json",
					"write input JSON with scores and comparison notes",
					"haft artifact create solution.compare --input-file input.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"portfolio_ref", "dimensions", "scores"},
				OptionalFields: []string{"non_dominated_set", "incomparable", "dominated_variants", "pareto_tradeoffs", "policy_applied", "recommendation_rationale", "selected_ref"},
				Notes:          []string{"Declare parity before scoring; preserve incomparable variants instead of forcing a scalar winner."},
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "decision.decide",
			Purpose: "Create a binding DecisionRecord after explicit operator invocation.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_decision",
				MCPAction:        "decide",
				MCPCall:          `haft_decision(action="decide", ...)`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft artifact create decision.decide --input-file input.json --json",
				DiscoveryCommand: "haft interface decision.decide --json",
				InputFileFlow: []string{
					"haft interface decision.decide --json",
					"write input JSON with full DRR fields",
					"haft artifact create decision.decide --input-file input.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"selected_title", "why_selected", "selection_policy", "weakest_link", "counterargument", "why_not_others", "rollback", "predictions", "invariants", "affected_files", "valid_until"},
				OptionalFields: []string{"problem_ref", "problem_refs", "portfolio_ref", "evidence_requirements", "refresh_triggers", "context", "mode", "task_context", "section_refs", "search_keywords", "_skips", "_skip_reason"},
				Notes:          []string{"Manual-only per Transformer Mandate; tactical skips are accepted only in tactical mode and require _skip_reason."},
			},
			Invariants: append(commonInterfaceInvariants(), "Human binding remains mandatory for DecisionRecord creation."),
		},
		{
			ID:      "note.record",
			Purpose: "Record a fact/observation note into the reasoning graph.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_note",
				MCPAction:        "",
				MCPCall:          `haft_note(title="...", observations=[...])`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft artifact create note.record --input-file input.json --json",
				DiscoveryCommand: "haft interface note.record --json",
				InputFileFlow: []string{
					"haft interface note.record --json",
					"write input JSON with title plus observations/evidence/rationale",
					"haft artifact create note.record --input-file input.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"title"},
				OptionalFields: []string{"observations", "evidence", "rationale", "anchors", "affected_files", "context", "valid_until", "search_keywords", "task_context"},
				Notes:          []string{"A note is not a decision; use decision.decide for choices among alternatives."},
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "method.pull",
			Purpose: "Create an open MethodRun and return compact task-local SWE method cards before non-trivial code work.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_method",
				MCPAction:        "pull",
				MCPCall:          `haft_method(action="pull", task="...", declared_task_kind="...", change_intent="...", intended_files=[...], risk_signals=[...])`,
				CLIStatus:        "mcp_only",
				DiscoveryCommand: "haft interface method.pull --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"task"},
				OptionalFields: []string{"declared_task_kind", "change_intent", "intended_files", "risk_signals", "user_scope_constraints", "artifact_refs", "ceremony_request", "response_budget", "context"},
				Notes:          []string{"Use for feature, bugfix/debug, refactor, external integration, governed files, cross-module edits, behavior changes, or failing tests.", "Mechanical edits should request low/none ceremony."},
			},
			OutputVolume: []string{"default: max 3 method cards, max 3 hard gates per card", "detail action: full definition for one method"},
			Invariants: append(commonInterfaceInvariants(),
				"No internal LLM classification; the agent declares task shape and risk signals.",
				"Pull persists a MethodRun carrier under .haft/method-runs.",
			),
		},
		{
			ID:      "method.close",
			Purpose: "Close an existing MethodRun by pull_id with changed files, hard-gate results, verification evidence, and explicit waivers.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_method",
				MCPAction:        "close",
				MCPCall:          `haft_method(action="close", pull_id="mpull-...", changed_files=[...], gate_results=[...], verification={...}, waivers=[...])`,
				CLIStatus:        "mcp_only",
				DiscoveryCommand: "haft interface method.close --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"pull_id"},
				OptionalFields: []string{"changed_files", "gate_results", "verification", "waivers"},
				Notes:          []string{"Hard gates require either satisfied evidence_refs or an explicit waiver reason.", "After context compaction, call method.status or method.show to recover the pull_id and required gates."},
			},
			OutputVolume: []string{"default: compact close acknowledgement", "show action: run status and selected methods"},
			Invariants: append(commonInterfaceInvariants(),
				"Close updates the same MethodRun carrier; pull and close are not separate artifact kinds.",
				"Soft gates do not require waivers.",
			),
		},
		{
			ID:      "method.status",
			Purpose: "List open MethodRuns so agents can recover pull_id after context compaction.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_method",
				MCPAction:        "status",
				MCPCall:          `haft_method(action="status")`,
				CLIStatus:        "mcp_only",
				DiscoveryCommand: "haft interface method.status --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"limit"},
				Notes:          []string{"Use before final response when a prior pull_id may have been compacted out of context."},
			},
			OutputVolume: []string{"default: compact open MethodRun list"},
			Invariants:   commonInterfaceInvariants(),
		},
		{
			ID:      "query.status",
			Purpose: "Show the compact project dashboard; use full=true for complete module coverage.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "status",
				MCPCall:          `haft_query(action="status", full=false)`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.status --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"context", "full"},
				Notes:          []string{"Default output is compact; pass full=true for complete coverage detail."},
			},
			OutputVolume: []string{"default: compact dashboard plus coverage summary", "full=true: complete coverage projection"},
			Invariants:   commonInterfaceInvariants(),
		},
		{
			ID:      "query.code_context",
			Purpose: "Show code-governance context before editing a file or symbol.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "code_context",
				MCPCall:          `haft_query(action="code_context", file="...", full=false)`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.code_context --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"file"},
				OptionalFields: []string{"symbol", "line", "context", "full"},
				Notes:          []string{"Default output caps invariant lists; pass full=true for the complete invariant projection."},
			},
			OutputVolume: []string{"default: compact invariant sections", "full=true: complete invariant lists"},
			Invariants:   commonInterfaceInvariants(),
		},
		{
			ID:      "refresh.scan",
			Purpose: "Scan stale decisions and drift; use verbose=true for full per-file and impact dumps.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_refresh",
				MCPAction:        "scan",
				MCPCall:          `haft_refresh(action="scan", verbose=false)`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface refresh.scan --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"context", "verbose"},
				Notes:          []string{"Default output summarizes counts, top modified files, and capped impact propagation."},
			},
			OutputVolume: []string{"default: compact drift summary", "verbose=true: full drift detail"},
			Invariants:   commonInterfaceInvariants(),
		},
	}
}

func commonInterfaceInvariants() []string {
	return []string{
		"Kernel validation remains authoritative.",
		"Existing MCP tools remain backward-compatible in this migration slice.",
		"Skills must retrieve contracts on demand instead of inlining long schemas.",
	}
}

func findInterfaceCapability(catalog []interfaceCapability, id string) (interfaceCapability, bool) {
	for _, capability := range catalog {
		if capability.ID == id {
			return capability, true
		}
	}

	return interfaceCapability{}, false
}

func writeInterfaceCatalogJSON(output io.Writer, catalog []interfaceCapability) error {
	summaries := make([]interfaceCapabilitySummary, 0, len(catalog))
	for _, capability := range catalog {
		summaries = append(summaries, interfaceCapabilitySummary{
			ID:      capability.ID,
			Purpose: capability.Purpose,
		})
	}

	response := interfaceCatalogResponse{
		Kind:         "haft_interface_catalog",
		Version:      1,
		Capabilities: summaries,
	}

	return writeJSON(output, response)
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeInterfaceCatalogText(output io.Writer, catalog []interfaceCapability) error {
	if _, err := fmt.Fprintln(output, "Haft interface capabilities:"); err != nil {
		return err
	}

	for _, capability := range catalog {
		if _, err := fmt.Fprintf(output, "- %s — %s\n", capability.ID, capability.Purpose); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(output, "\nUse `haft interface <capability> --json` for the compact machine-readable contract.")
	return err
}

func writeInterfaceCapabilityText(output io.Writer, capability interfaceCapability) error {
	lines := []string{
		fmt.Sprintf("%s — %s", capability.ID, capability.Purpose),
		fmt.Sprintf("MCP: %s", capability.CurrentExecution.MCPCall),
		fmt.Sprintf("CLI status: %s", capability.CurrentExecution.CLIStatus),
		fmt.Sprintf("Discovery: %s", capability.CurrentExecution.DiscoveryCommand),
		fmt.Sprintf("Required: %s", strings.Join(capability.InputContract.RequiredFields, ", ")),
	}

	if capability.CurrentExecution.CLICommand != "" {
		lines = append(lines, fmt.Sprintf("CLI: %s", capability.CurrentExecution.CLICommand))
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}

	if len(capability.InputContract.OptionalFields) > 0 {
		if _, err := fmt.Fprintf(output, "Optional: %s\n", strings.Join(capability.InputContract.OptionalFields, ", ")); err != nil {
			return err
		}
	}

	return nil
}
