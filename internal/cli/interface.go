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
	RequiredFields []string        `json:"required_fields"`
	OptionalFields []string        `json:"optional_fields,omitempty"`
	FieldShapes    []fieldShape    `json:"field_shapes,omitempty"`
	InputTemplates []inputTemplate `json:"input_templates,omitempty"`
	Notes          []string        `json:"notes,omitempty"`
}

type fieldShape struct {
	Field string `json:"field"`
	Shape string `json:"shape"`
	Note  string `json:"note,omitempty"`
}

type inputTemplate struct {
	Name  string         `json:"name"`
	Value map[string]any `json:"value"`
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
				OptionalFields: []string{"problem_type", "problem_profile", "source_kind", "why_now", "scope", "acceptance_probe", "freshness_disposition", "constraints", "optimization_targets", "observation_indicators", "acceptance", "blast_radius", "reversibility", "context", "mode", "task_context"},
				Notes:          []string{"Frame before exploring when the problem is fuzzy or a solution was proposed before acceptance criteria.", "P2W readiness is computed: wish/ticket/chosen_method sources cannot become ready without explicit scope and acceptance probe."},
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "problem.characterize",
			Purpose: "Attach comparison dimensions and an optional parity plan to a ProblemCard.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_problem",
				MCPAction:        "characterize",
				MCPCall:          `haft_problem(action="characterize", problem_ref="...", dimensions=[...])`,
				CLIStatus:        "mcp_only",
				DiscoveryCommand: "haft interface problem.characterize --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"problem_ref", "dimensions[].name"},
				OptionalFields: []string{"dimensions[].role", "dimensions[].polarity", "dimensions[].scale_type", "dimensions[].unit", "dimensions[].proxy_for", "dimensions[].how_to_measure", "dimensions[].valid_until", "parity_plan", "context", "mode"},
				FieldShapes: []fieldShape{
					{
						Field: "dimensions[]",
						Shape: `{"name":"latency","role":"target","polarity":"lower_is_better","scale_type":"ratio","unit":"ms","valid_until":"2026-08-12"}`,
						Note:  "One object per comparison dimension; name is required.",
					},
					{
						Field: "parity_plan",
						Shape: `{"baseline_set":["V1","V2"],"window":"same replay window","budget":"same operator time","missing_data_policy":"explicit_abstain"}`,
						Note:  "Deep mode requires a structured parity plan; standard mode may warn on gaps.",
					},
				},
				InputTemplates: []inputTemplate{
					{
						Name: "characterize_problem",
						Value: map[string]any{
							"problem_ref": "prob-...",
							"dimensions": []any{
								map[string]any{
									"name":           "latency",
									"role":           "target",
									"polarity":       "lower_is_better",
									"scale_type":     "ratio",
									"unit":           "ms",
									"how_to_measure": "same replay window",
									"valid_until":    "2026-08-12",
								},
							},
							"parity_plan": map[string]any{
								"baseline_set":        []any{"V1", "V2"},
								"window":              "same replay window",
								"budget":              "same operator time",
								"missing_data_policy": "explicit_abstain",
							},
						},
					},
				},
				Notes: []string{"Declare comparison dimensions before scoring variants; observations are watch-only and should not become hidden targets."},
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
				FieldShapes: []fieldShape{
					{
						Field: "variants[]",
						Shape: `{"title":"...","weakest_link":"...","novelty_marker":"...","stepping_stone":true,"stepping_stone_basis":"..."}`,
						Note:  "At least two variants; title, weakest_link, and novelty_marker are required.",
					},
				},
				InputTemplates: []inputTemplate{
					{
						Name: "explore_variants",
						Value: map[string]any{
							"problem_ref": "prob-...",
							"variants": []any{
								map[string]any{
									"title":                "Adapter lane",
									"description":          "Add a narrow adapter while preserving the existing kernel contract.",
									"weakest_link":         "Adapter coverage can miss one important caller path.",
									"novelty_marker":       "Changes carrier shape without changing kernel authority.",
									"stepping_stone":       true,
									"stepping_stone_basis": "Opens later host-specific adapters.",
								},
								map[string]any{
									"title":          "Kernel lane",
									"weakest_link":   "Higher blast radius in shared validation code.",
									"novelty_marker": "Moves the invariant into the authoritative kernel path.",
								},
							},
						},
					},
				},
				Notes: []string{"At least one variant should be a stepping stone, or explain why no stepping-stone variant exists."},
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
				OptionalFields: []string{"non_dominated_set", "incomparable", "dominated_variants", "pareto_tradeoffs", "policy_applied", "recommendation_rationale", "legacy_recommendation_ref", "selected_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "scores",
						Shape: `{"V1":{"latency":"10ms","cost":"$5"},"V2":{"latency":"25ms","cost":"$1"}}`,
						Note:  "Canonical nesting: outer key variant_id, inner key dimension_name, value string score. Do not send dimension-first scores.",
					},
					{
						Field: "dominated_variants[]",
						Shape: `{"variant":"V2","dominated_by":["V1"],"summary":"Higher latency with no compensating benefit."}`,
						Note:  "Explain each compared variant outside the Pareto front exactly once.",
					},
					{
						Field: "pareto_tradeoffs[]",
						Shape: `{"variant":"V1","summary":"Lowest latency under the declared parity window."}`,
						Note:  "Explain each Pareto-front variant exactly once.",
					},
					{
						Field: "incomparable",
						Shape: `[["V1","V3"]]`,
						Note:  "Pairs that should remain intentionally incomparable.",
					},
				},
				InputTemplates: []inputTemplate{
					{
						Name: "flat_compare",
						Value: map[string]any{
							"portfolio_ref": "sol-...",
							"dimensions":    []any{"latency", "cost"},
							"scores": map[string]any{
								"V1": map[string]any{"latency": "10ms", "cost": "$5"},
								"V2": map[string]any{"latency": "25ms", "cost": "$1"},
							},
							"non_dominated_set": []any{"V1"},
							"dominated_variants": []any{
								map[string]any{
									"variant":      "V2",
									"dominated_by": []any{"V1"},
									"summary":      "Higher latency with no compensating benefit in this comparison.",
								},
							},
							"pareto_tradeoffs": []any{
								map[string]any{
									"variant": "V1",
									"summary": "Lowest latency under the declared parity window.",
								},
							},
							"incomparable":              []any{},
							"policy_applied":            "Prefer the lowest-risk Pareto-front variant.",
							"legacy_recommendation_ref": "V1",
							"recommendation_rationale":  "V1 best satisfies the declared target under parity.",
						},
					},
				},
				Notes: []string{
					"Declare parity before scoring; preserve incomparable variants instead of forcing a scalar winner.",
					"legacy_recommendation_ref is advisory and is the preferred alias for legacy selected_ref; it is not ChoiceResult.",
					"CLI input-file still accepts legacy results{...}, but agents should use the flat fields shown in flat_compare.",
				},
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
				OptionalFields: []string{"problem_ref", "problem_refs", "portfolio_ref", "choice_result", "transformation_record", "evidence_requirements", "refresh_triggers", "context", "mode", "task_context", "section_refs", "search_keywords", "_skips", "_skip_reason"},
				FieldShapes: []fieldShape{
					{
						Field: "choice_result",
						Shape: `{"subject_ref":"operator","next_move":"choose_now","variant_ref":"V1","reason":"explicit h-decide"}`,
						Note:  "Exact human choice outcome; compare never creates it.",
					},
					{
						Field: "transformation_record",
						Shape: `{"schema_version":1,"transformed_entity":"ProblemCard profile","initial_state":"implicit prose","post_state":"typed profile/readiness object","relation":"makes explicit","context":"semantic-spine slice"}`,
						Note:  "Describes the target transformation only; method/work/evidence/publication remain separate.",
					},
					{
						Field: "why_not_others[]",
						Shape: `{"variant":"V2","reason":"Worse fit under the declared selection policy."}`,
						Note:  "One object per serious rejected variant.",
					},
					{
						Field: "rollback",
						Shape: `{"triggers":["metric regresses"],"steps":["revert the adapter"],"blast_radius":"single surface"}`,
						Note:  "Rollback is an object, not free text.",
					},
					{
						Field: "predictions[]",
						Shape: `{"claim":"...","observable":"...","threshold":"...","verify_after":"2026-07-12","probability":0.7}`,
						Note:  "Claims must be observable later; probability is optional.",
					},
				},
				InputTemplates: []inputTemplate{
					{
						Name: "decide",
						Value: map[string]any{
							"problem_ref":   "prob-...",
							"portfolio_ref": "sol-...",
							"transformation_record": map[string]any{
								"schema_version":     1,
								"transformed_entity": "interface contract discovery",
								"initial_state":      "nested schema hidden in long tool descriptions",
								"post_state":         "explicit input template reachable by interface discovery",
								"relation":           "separates discovery carrier from execution authority",
								"context":            "agent-facing planning",
							},
							"selected_title":   "Typed contract templates in haft interface",
							"why_selected":     "Agents can see the nested input shape before their first tool call.",
							"selection_policy": "Prefer compact discoverability that does not bloat tools/list.",
							"weakest_link":     "Templates can drift from validators if not covered by tests.",
							"counterargument":  "MCP schemas alone might be enough for some hosts.",
							"why_not_others": []any{
								map[string]any{"variant": "MCP-only schema", "reason": "Bloats tools/list and is harder for humans to inspect."},
							},
							"rollback": map[string]any{
								"triggers":     []any{"tools/list exceeds budget", "templates fail round-trip tests"},
								"steps":        []any{"remove expanded templates", "restore compact field list"},
								"blast_radius": "agent-facing discovery only",
							},
							"predictions": []any{
								map[string]any{
									"claim":        "Malformed nested calls decrease after interface templates ship.",
									"observable":   "Fewer schema-retry comments in dogfood sessions.",
									"threshold":    "No repeated shape-fix retry for solution.compare in the next dogfood run.",
									"verify_after": "2026-07-12",
									"probability":  0.7,
								},
							},
							"invariants":     []any{"Kernel validation remains authoritative."},
							"affected_files": []any{"internal/cli/interface.go"},
							"valid_until":    "2026-08-12",
						},
					},
				},
				Notes: []string{"Manual-only per Transformer Mandate; tactical skips are accepted only in tactical mode and require _skip_reason.", "transformation_record is an explicit target-state description, not a MethodRun, WorkCommission, evidence item, or publication unit."},
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
			OutputVolume: []string{"default: max 3 method cards, max 3 hard gates per card, plus close_template JSON", "detail action: full definition for one method"},
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
				FieldShapes: []fieldShape{
					{
						Field: "gate_results[]",
						Shape: `{"gate_id":"<hard-gate-id>","status":"satisfied","evidence_refs":["go test ./..."]}`,
						Note:  "Use gate_id, not gate; use status=satisfied, not result=pass.",
					},
					{
						Field: "verification",
						Shape: `{"commands":["go test ./..."],"result":"pass","output_ref":"local terminal"}`,
						Note:  "Verification is an object with command evidence.",
					},
				},
				Notes: []string{
					`gate_results[] shape: {"gate_id":"<hard-gate-id>","status":"satisfied","evidence_refs":["<evidence-ref>"]}`,
					`waivers[] shape: {"gate_id":"<hard-gate-id>","reason":"<why waived>"}`,
					`verification shape: {"commands":["<command>"],"result":"<pass|partial|failed>","output_ref":"<optional>"}`,
					"Hard gates require either satisfied evidence_refs or an explicit waiver reason.",
					"After context compaction, call method.status then method.show to recover the pull_id and close_template.",
				},
			},
			OutputVolume: []string{"default: compact close acknowledgement", "show action: run status, selected methods, and close_template for open runs"},
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
			Purpose: "Show the compact operator cockpit; use full=true for detailed status and complete module coverage.",
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
				Notes: []string{
					"Default output is a compact operator cockpit; omitted detail is not evidence of absence.",
					"Pass full=true for detailed status, including shipped/pending/unassessed decision lists, addressed problems, recent notes, and full coverage when available.",
					"Use haft_query(action=\"coverage\") for module coverage, haft_refresh(action=\"scan\", verbose=true) for drift/stale detail, and haft_refresh(action=\"plan\") for the maintenance work order.",
				},
			},
			OutputVolume: []string{"default: compact cockpit plus one-line coverage cue", "full=true: detailed status plus complete coverage projection"},
			Invariants:   commonInterfaceInvariants(),
		},
		{
			ID:      "query.code_context",
			Purpose: "Show code-governance context before editing a file or symbol.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "code_context",
				MCPCall:          `haft_query(action="code_context", file="...", lane="index")`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.code_context --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"file"},
				OptionalFields: []string{"symbol", "line", "context", "lane", "limit", "full"},
				Notes: []string{
					"Default output is lane=index: target, governed/blind status, lane counts, hard risks, and exact next calls.",
					"Valid lanes: index, symbols, decisions, invariants, notes, problems, portfolios, all.",
					"Prefer one typed lane at a time; lane=all restores the compact all-lane view; full=true is audit/backcompat dump.",
				},
			},
			OutputVolume: []string{
				"default: lane=index under normal planning budget",
				"typed lanes: capped by limit, default 20",
				"lane=all: compact all-lane recovery view",
				"full=true: complete audit dump",
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "query.related",
			Purpose: "Recover one artifact carrier by ref, including explicit ProblemCard semantic views when available.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "related",
				MCPCall:          `haft_query(action="related", ref="prob-...")`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.related --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"ref"},
				OptionalFields: []string{"artifact_id"},
				FieldShapes: []fieldShape{
					{
						Field: "problem_card.semantic",
						Shape: `{"status":"exact|legacy|degraded","profile":{"id":"...","hash":"sha256:..."},"publication_unit":{"source_edition_pin":{...},"publication_hash":"sha256:...","carrier_hash":"sha256:..."}}`,
						Note:  "Semantic status is explicit; missing legacy envelopes are not promoted to exact.",
					},
					{
						Field: "problem_card.views",
						Shape: `{"working":{...},"exact":{"source_episteme":{...},"publication_projection":{...},"carrier_bytes":{...}},"audit":{...}}`,
						Note:  "working is compact; exact/audit split source episteme, publication projection, carrier bytes, and provenance.",
					},
				},
				Notes: []string{
					"For ProblemCard refs, the response preserves legacy keys and adds semantic + views.",
					"SQLite remains runtime source of truth; markdown is a carrier imported through explicit sync.",
					"Audit views must label legacy/degraded semantics instead of fabricating exact provenance.",
				},
			},
			OutputVolume: []string{
				"default: one JSON artifact payload",
				"ProblemCard: legacy payload plus semantic, working, exact, and audit view objects",
			},
			Invariants: append(commonInterfaceInvariants(),
				"No new top-level MCP action; related remains the single-artifact recovery path.",
				"Compatibility projections must not become authority or evidence by presentation.",
			),
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
