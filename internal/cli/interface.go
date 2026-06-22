package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
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
	if isInterfaceContractAuditID(capabilityID) {
		report := buildInterfaceContractAuditReport(catalog)
		if interfaceJSON {
			return writeJSON(output, report)
		}
		return writeInterfaceContractAuditText(output, report)
	}

	if isInterfaceContractGenerationID(capabilityID) {
		report := buildInterfaceContractGenerationReport(catalog)
		if interfaceJSON {
			return writeJSON(output, report)
		}
		return writeInterfaceContractGenerationText(output, report)
	}

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
			Purpose: "Create a binding DecisionRecord after explicit operator invocation. MCP defaults to cli-only binding mode and returns operator_confirmation_required for decide.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_decision",
				MCPAction:        "decide",
				MCPCall:          `haft_decision(action="decide", ...) -> operator_confirmation_required in default MCP mode`,
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
				OptionalFields: []string{"problem_ref", "problem_refs", "portfolio_ref", "choice_result", "transformation_record", "decision_subject_ref", "implementation_footprint", "governance_targets", "drift_watch_targets", "claims", "evidence_requirements", "refresh_triggers", "context", "mode", "task_context", "section_refs", "search_keywords", "binding_hints", "binding_scope", "binding_fallback_reason", "binding_targets", "_skips", "_skip_reason"},
				FieldShapes: []fieldShape{
					{
						Field: "choice_result",
						Shape: `{"subject_ref":"operator","option_set":["V1","V2"],"comparison_basis":["selected V1: ...","rejected V2: ..."],"choice_rule":"declared selection policy","next_move":"choose_now","variant_ref":"V1","reason":"explicit h-decide","reversibility":"two-week rollback","reopen_condition":"reopen if rollback triggers occur"}`,
						Note:  "Exact human choice outcome; compare never creates it and DecisionRecord remains a compatibility projection.",
					},
					{
						Field: "transformation_record",
						Shape: `{"schema_version":1,"transformed_entity":"ProblemCard profile","initial_state":"implicit prose","post_state":"typed profile/readiness object","relation":"makes explicit","context":"semantic-spine slice","window":"2026-Q3","method_refs":["mpull-..."],"work_refs":["wc-..."],"evidence_refs":["evid-..."],"publication_refs":["pub-..."]}`,
						Note:  "Describes the target transformation only; method/work/evidence/publication refs remain separate records and are not proof of occurrence, approval, or publication.",
					},
					{
						Field: "binding_targets",
						Shape: `[{"kind":"symbol","file_path":"internal/artifact/decision.go","symbol_name":"Baseline","symbol_kind":"func","line":1079,"end_line":1178},{"kind":"api_contract|invariant|spec_section","target_ref":"api_contract:haft/status","file_path":"docs/contracts.md","text_hash":"...","anchor_hash":"..."}]`,
						Note:  "Legacy compatibility projection. Explicit semantic target kinds use target_ref; an exact haft-target: <target_ref> marker, unambiguous markdown heading, or fenced yaml spec-section id can attach file_path + text_hash automatically, otherwise drift evaluation needs concrete carrier/range fields or fails closed as needs binding resolution. New decisions with affected_files auto-enrich binding_targets when a precise target is safely resolvable; ambiguous files remain unenriched. New decisions should prefer governance_targets plus drift_watch_targets; use binding_scope=whole_file only with binding_fallback_reason.",
					},
					{
						Field: "implementation_footprint",
						Shape: `{"files":["internal/artifact/decision.go"],"commits":["abc123"],"work_refs":["wc-..."]}`,
						Note:  "Historical provenance of touched files/work. Footprint-only files are not governance drift authority.",
					},
					{
						Field: "governance_targets[]",
						Shape: `{"kind":"symbol|api_contract|invariant|spec_section","ref":"symbol:internal/artifact/decision.go::Baseline","binding_target":{"kind":"symbol|api_contract|invariant|spec_section","target_ref":"api_contract:haft/status","file_path":"docs/contracts.md","symbol_name":"Baseline","symbol_kind":"func","body_hash":"...","text_hash":"..."}}`,
						Note:  "What the decision actually regulates: symbol, range, module, API contract, invariant, spec section, or carrier.",
					},
					{
						Field: "drift_watch_targets[]",
						Shape: `{"target_ref":"symbol:internal/artifact/decision.go::Baseline","trigger":"symbol_body_changed|schema_or_behavior_changed|invariant_changed","binding_target":{"kind":"symbol|api_contract|invariant|spec_section","target_ref":"api_contract:haft/status","file_path":"docs/contracts.md","symbol_name":"Baseline","symbol_kind":"func","body_hash":"...","text_hash":"..."}}`,
						Note:  "Concrete drift triggers. Drift scan uses these before governance_targets, then legacy binding_targets, then file fallback.",
					},
					{
						Field: "claims[]",
						Shape: `{"id":"claim-001","claim":"...","observable":"...","threshold":"...","lifecycle_status":"active|refresh_due|superseded|deprecated","successor_ref":"dec-new#claim-002","retired_reason":"...","governance_target_refs":["api_contract:haft/status"]}`,
						Note:  "Explicit canonical claims. Empty legacy lifecycle reads as active; predictions remain the compatibility projection.",
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
							"choice_result": map[string]any{
								"subject_ref":      "operator",
								"option_set":       []any{"Typed contract templates in haft interface", "MCP-only schema"},
								"comparison_basis": []any{"selected Typed contract templates in haft interface: Agents can see the nested input shape before their first tool call.", "rejected MCP-only schema: Bloats tools/list and is harder for humans to inspect."},
								"choice_rule":      "Prefer compact discoverability that does not bloat tools/list.",
								"next_move":        "choose_now",
								"variant_ref":      "Typed contract templates in haft interface",
								"portfolio_ref":    "sol-...",
								"reason":           "Agents can see the nested input shape before their first tool call.",
								"reopen_condition": "reopen if compact discoverability no longer protects context budget",
							},
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
				Notes: []string{
					"MCP schema discovery may show decide fields, but default MCP execution is fail-closed because model-supplied arguments are not kernel-verifiable operator authorization.",
					"Use the input-file CLI/manual path for the binding act; v1 accepts only manual_cli authorization receipts and marks MCP receipt-backed binding unsupported.",
					"Manual-only per Transformer Mandate; tactical skips are accepted only in tactical mode and require _skip_reason.",
					"choice_result carries C.11 subject, option_set, comparison_basis, choice_rule, next_move, reversibility, and reopen_condition; DecisionRecord remains the compatibility projection.",
					"transformation_record is an explicit target-state description, not a MethodRun, WorkCommission, evidence item, or publication unit; refs point outward and do not prove occurrence, approval, evidence truth, or publication.",
				},
			},
			Invariants: append(commonInterfaceInvariants(), "Human binding remains mandatory for DecisionRecord creation."),
		},
		{
			ID:      "decision.reconcile_apply",
			Purpose: "Apply an operator-approved DecisionReconciliationPlan selection to lineage/status or explicit scope enrichment.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "decision_reconcile",
				MCPCall:          `haft_query(action="decision_reconcile") for report-only planning; MCP has no apply action in this slice`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft decision reconcile apply selection.json --json",
				DiscoveryCommand: "haft interface decision.reconcile_apply --json",
				InputFileFlow: []string{
					"haft_query(action=\"decision_reconcile\") or `haft decision reconcile --json`",
					"operator approves a reviewed selection document",
					"haft decision reconcile apply selection.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"schema_version", "authority", "operator_approval_ref", "items"},
				FieldShapes: []fieldShape{
					{
						Field: "selection_document",
						Shape: `{"schema_version":1,"authority":"operator_approved_reconciliation_selection","operator_approval_ref":"chat:...","items":[{"operation":"merge_through_successor|supersede|retire_without_successor|reopen|enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-old"],"successor_ref":"dec-new","decision_subject_ref":"subject:ref","governance_targets":[{"kind":"api_contract","ref":"api_contract:..."}],"drift_watch_targets":[{"target_ref":"api_contract:...","trigger":"schema_or_behavior_changed"}],"claim_governance_target_refs":{"claim-id":["api_contract:..."]},"reason":"..."}]}`,
						Note:  "successor_ref is required for supersede/merge_through_successor, forbidden for retire_without_successor/enrich_scope, and omitted for reopen; enrich_scope requires exactly one decision plus decision_subject_ref and governance_targets or drift_watch_targets; this apply path does not create binding decisions.",
					},
				},
				Notes: []string{
					"This is an explicit lifecycle mutation path; run the report-only plan first and apply only reviewed selections.",
					"merge_through_successor requires an already-created successor DecisionRecord; this command does not create binding decisions.",
					"enrich_scope changes only DecisionRecord scope fields; it does not change status, lineage, evidence, baselines, or gates.",
					"Validation covers the whole batch before mutation so a later invalid item cannot leave earlier items partially applied.",
				},
			},
			OutputVolume: []string{"default: compact applied operation list", "--json: DecisionReconciliationApplyResult"},
			Invariants: append(commonInterfaceInvariants(),
				"MCP has no reconciliation apply action in this slice.",
				"Operator approval ref is required in the selection document.",
				"Old decision IDs remain searchable; terminal status changes preserve lineage.",
				"Scope enrichment is additive: existing subjects cannot be retargeted by enrich_scope.",
			),
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
			ID:      "query.contract_audit",
			Purpose: "Audit kernel-owned interface contracts, mirrored host schemas, validation refs, and legacy transport exceptions.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "contract_audit",
				MCPCall:          `haft_query(action="contract_audit")`,
				CLIStatus:        "available",
				CLICommand:       "haft interface contract-audit --json",
				DiscoveryCommand: "haft interface query.contract_audit --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"kind":"haft_interface_contract_audit","schema_version":1,"authority":"read_only_contract_inventory_not_schema_generation","summary":{"capabilities":32,"kernel_owned_contracts":32,"mcp_mirrored_actions":20,"cli_available_surfaces":12,"binding_authority_surfaces":4,"legacy_transport_exceptions":18,"schema_covered_surfaces":20,"schema_missing_surfaces":0,"schema_excluded_fields":8,"shape_covered_surfaces":20,"shape_missing_surfaces":0,"shape_generator_targets":0},"surfaces":[{"capability_id":"decision.decide","contract_sources":["kernel_interface_catalog"],"schema_posture":"mcp_schema_mirrored","authority_posture":"binding_denied_by_default_mcp","validation_refs":["internal/cli/interface_test.go","internal/fpf/server_test.go"],"legacy_exception":false,"schema_coverage":{"checked":true,"status":"covered","excluded_fields":["task_context"]},"shape_coverage":{"checked":true,"status":"covered"}}]}`,
						Note:  "The audit identifies contract fragments and validation posture; it does not generate schemas, approve binding actions, or change tool descriptions.",
					},
				},
				Notes: []string{
					"Use this before Phase F1 schema generation so host/schema drift is visible without inlining tool schemas into status.",
					"Read-only: schema visibility is not operator authorization and not binding authority.",
				},
			},
			OutputVolume: []string{"default: compact contract-source audit; --json: full surface list; never in default status"},
			Invariants: append(commonInterfaceInvariants(),
				"Contract audit is read-only.",
				"Generated schema work remains a later phase.",
				"Binding-denied MCP surfaces remain denied even when their schema is visible.",
			),
		},
		{
			ID:      "query.contract_generation",
			Purpose: "List remaining kernel-owned contract generator targets for MCP/host schema generation without mutating schemas.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "contract_generation",
				MCPCall:          `haft_query(action="contract_generation")`,
				CLIStatus:        "available",
				CLICommand:       "haft interface contract-generation --json",
				DiscoveryCommand: "haft interface query.contract_generation --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"kind":"haft_interface_contract_generation_manifest","schema_version":1,"authority":"read_only_generation_manifest_not_generated_schema","source":"kernel_interface_catalog_field_shapes","source_digest":"sha256:...","summary":{"capabilities":33,"generator_target_surfaces":0,"generator_target_fields":0},"surface_policy":{"default_status":"cue_or_count_only_never_inline_generation_manifest","default_code_context":"lane_index_only_never_inline_generated_descriptions","tools_list":"action_enum_and_compact_description_only_no_generated_schema_fragments","compact_cli":"summary_counts_only_field_targets_require_json","generated_descriptions":"drill_down_only_validate_with_carrier_semio_before_host_materialization","required_guards":["carrier_semio_authority_boundary","tools_list_context_budget","compact_status_no_manifest_inline","code_context_lane_index_default"]},"targets":[]}`,
						Note:  "The manifest is the kernel-owned queue for future generation; it does not generate schemas or authorize binding actions.",
					},
				},
				Notes: []string{
					"Use this after contract-audit: it lists any remaining generator targets classified by the audit.",
					"Read-only: schema generation targets are not generated schemas, operator authorization, evidence, or gate passage.",
				},
			},
			OutputVolume: []string{"default: compact generator target manifest; --json: field-level targets; never in default status"},
			Invariants: append(commonInterfaceInvariants(),
				"Contract generation manifest is read-only.",
				"Fields come from kernel interface catalog field_shapes and current MCP schema gaps.",
				"Schema visibility remains separate from binding authorization.",
			),
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
					"Use haft_query(action=\"coverage\") for module coverage, haft_refresh(action=\"scan\", verbose=true) for drift/stale detail, haft_refresh(action=\"plan\") for the maintenance work order, haft_refresh(action=\"review\") for a read-only needs-judgment packet, and haft_refresh(action=\"drain\", dry_run=true) to preview safe closures.",
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
			ID:      "query.carrier_manifest",
			Purpose: "Show the read-only carrier authority manifest for current/support/compat/provenance/archive scope classification.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "carrier_manifest",
				MCPCall:          `haft_query(action="carrier_manifest")`,
				CLIStatus:        "available",
				CLICommand:       "haft carrier manifest --json",
				DiscoveryCommand: "haft interface query.carrier_manifest --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"generated_by":"haft carrier manifest","entries":[{"id":"...","path_pattern":"...","authority_class":"current_authority|support_material|compatibility_carrier|provenance|archive|external_sidekick_out_of_scope","current":true|false,"normativity":"..."}]}`,
						Note:  "Carrier classifies where wording lives; it is not itself a binding decision or evidence item.",
					},
				},
				Notes: []string{
					"Use this when carrier/currentness wording is ambiguous; do not add it to default status.",
					"Open-Sleigh remains external_sidekick_out_of_scope unless explicitly reopened.",
				},
			},
			OutputVolume: []string{"default: JSON manifest drill-down only; never in compact status"},
			Invariants: append(commonInterfaceInvariants(),
				"Carrier manifest is read-only.",
				"Carrier classes are review/discovery metadata, not binding authority by themselves.",
				"Default status must not inline carrier manifest entries.",
			),
		},
		{
			ID:      "query.carrier_check",
			Purpose: "Run the read-only fixed-point semio check over current/support/compat carrier wording.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "carrier_check",
				MCPCall:          `haft_query(action="carrier_check")`,
				CLIStatus:        "available",
				CLICommand:       "haft carrier check --json",
				DiscoveryCommand: "haft interface query.carrier_check --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"checked_files":["README.md",...],"checked_generated_surfaces":["generated/interface/query.status",...],"findings":[{"path":"...","line":1,"term":"desktop|tui|standalone|haft agent|operator_authorization_boundary","snippet":"...","diagnostic":"..."}]}`,
						Note:  "Findings mean a dead runtime-surface mention lacks dropped/archive/provenance/support/not-current context, or generated/host wording implies schema/prompt/model authorization.",
					},
				},
				Notes: []string{
					"The check is deterministic and read-only; it does not mutate carriers or decisions.",
					"It avoids substring matches in decision slugs, package names, and ordinary words.",
					"It also checks generated interface descriptions that are not materialized as carrier files.",
					"Use it as a focused drill-down before changing current carrier wording.",
				},
			},
			OutputVolume: []string{"default: checked file list plus findings array; never in compact status"},
			Invariants: append(commonInterfaceInvariants(),
				"Carrier semio check is read-only.",
				"Carrier semio findings are review inputs, not evidence, approval, or GateDecision.",
				"Default status must not inline carrier semio findings.",
			),
		},
		{
			ID:      "query.spec_review",
			Purpose: "Build a read-only advisory semantic review packet over typed SpecSections.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "spec_review",
				MCPCall:          `haft_query(action="spec_review")`,
				CLIStatus:        "available",
				CLICommand:       "haft spec review --json",
				DiscoveryCommand: "haft interface query.spec_review --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"authority":"advisory_only","review_kind":"spec_semantic","profile":{"id":"spec_semantic_review_v2","model_disposition":{...}},"summary":{"checked_sections":10,"explicit_claims":0,"blocked_for_stronger_use_findings":0},"sections":[{"system_frame":{...},"claim_register":{...},"state_reading":{...},"findings":[{"rule_id":"...","category":"claim_posture|publication_boundary|frame|unknown_abstain"}]}]}`,
						Note:  "Findings are review inputs and never evidence, approval, rebaseline, GateDecision, or SpecUseAdmission.",
					},
				},
				Notes: []string{
					"Spec semantic review is advisory and read-only.",
					"It can abstain/block stronger use when the semantic profile lacks enough structure.",
					"Claim counts are profile/read findings until first-class ClaimRegister exists.",
				},
			},
			OutputVolume: []string{"default: one JSON spec semantic review packet; compact text via `haft spec review`"},
			Invariants: append(commonInterfaceInvariants(),
				"Spec review findings are not evidence, approval, rebaseline, GateDecision, or SpecUseAdmission.",
				"Default status must not inline the review packet.",
				"Unknown or high-risk semantic posture abstains/blocks stronger use instead of passing.",
			),
		},
		{
			ID:      "query.spec_use",
			Purpose: "Build a read-only SpecificationUseRecord for one SpecSection and one declared use context.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "spec_use",
				MCPCall:          `haft_query(action="spec_use", section_id="TS.role.001", use_context="commission preflight", policy="stronger_use_requires_current_source")`,
				CLIStatus:        "available",
				DiscoveryCommand: "haft interface query.spec_use --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"section_id", "use_context", "policy"},
				OptionalFields: []string{"waiver_expires_at", "operational_gate"},
				FieldShapes: []fieldShape{
					{
						Field: "policy",
						Shape: `"documentary_only" | "stronger_use_requires_current_source" | "temporary_waiver"`,
						Note:  "Admission policy is explicit and never inferred from baseline currentness alone.",
					},
					{
						Field: "operational_gate",
						Shape: `{"schema_version":1,"gate_ref":"gate-...","bearer_ref":"TS.role.001","use_context":"commission preflight","rule":"require_current_source_and_admitted_use","evidence_refs":["evid-..."],"expires_at":"2099-01-01T00:00:00Z","reopen_condition":"section baseline drifts"}`,
						Note:  "Optional v1 gate profile; evaluation is local/read-only and does not create approval, evidence, or work authority.",
					},
					{
						Field: "response",
						Shape: `{"source_edition":{...},"baseline_currentness":{...},"admission":{...},"gate_decision":{"status":"not_applicable_no_operational_gate|passed|blocked","authority_boundary":{...}}}`,
						Note:  "Currentness, admission, waiver expiry, and gate status are separate fields.",
					},
				},
				Notes: []string{
					"SpecificationUseRecord is read-only; it does not approve/rebaseline specs, create evidence, create WorkCommissions, or mutate an OperationalGate.",
					"Use policy=temporary_waiver only with waiver_expires_at; the waiver is represented in the response and is not global truth.",
				},
			},
			OutputVolume: []string{"default: one JSON SpecificationUseRecord"},
			Invariants: append(commonInterfaceInvariants(),
				"Baseline currentness is not admission; admission policy is a distinct field.",
				"GateDecision passed/blocked is emitted only from an explicit OperationalGate profile.",
				"GateDecision remains a derived reading, not spec approval or execution authority.",
			),
		},
		{
			ID:      "query.evidence_path",
			Purpose: "Build a read-only EvidencePath/RelianceDisposition record for one evidence item and declared attempted use.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "evidence_path",
				MCPCall:          `haft_query(action="evidence_path", artifact_ref="dec-...", evidence_ref="evid-...", attempted_use="verification reliance", method_ref="mpull-...")`,
				CLIStatus:        "available",
				CLICommand:       "haft evidence path ARTIFACT_REF EVIDENCE_REF --attempted-use ... --method-ref ... --json",
				DiscoveryCommand: "haft interface query.evidence_path --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"artifact_ref", "evidence_ref", "attempted_use"},
				OptionalFields: []string{"claim_ref", "producer_ref", "method_ref", "work_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"claim_binding":{...},"trace_binding":{...},"currentness_window":{...},"reliance_disposition":{"disposition":"bounded_reliance|advisory_only|blocked"},"authority_boundary":{"approval":"not_approval","gate_decision":"not_gate_decision","global_truth":"not_global_truth"}}`,
						Note:  "Reliance is bounded to the declared use and never creates approval, gate passage, or global truth.",
					},
				},
				Notes: []string{
					"EvidencePathRecord is read-only and derived from an existing EvidenceItem.",
					"Missing attempted use, missing trace refs, expired evidence, refuting evidence, or an unbound requested claim cannot produce bounded reliance.",
				},
			},
			OutputVolume: []string{"default: one JSON EvidencePathRecord"},
			Invariants: append(commonInterfaceInvariants(),
				"Evidence presence is not approval, gate passage, or global truth.",
				"Claim, trace, currentness, and attempted-use boundaries stay explicit in the response.",
			),
		},
		{
			ID:      "query.change_case",
			Purpose: "Build a read-only EngineeringChangeCase projection over one DecisionRecord's problem, transformation, choice, and evidence refs.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "change_case",
				MCPCall:          `haft_query(action="change_case", artifact_ref="dec-...", attempted_use="implementation review", method_ref="mpull-...")`,
				CLIStatus:        "available",
				CLICommand:       "haft change case DECISION_REF --attempted-use ... --method-ref ... --json",
				DiscoveryCommand: "haft interface query.change_case --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"artifact_ref"},
				OptionalFields: []string{"attempted_use", "producer_ref", "method_ref", "work_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"case_ref":"change-case:dec-...","problem_card_refs":[...],"transformation_refs":[...],"choice_result_ref":"...","evidence_item_refs":[...],"evidence_path_refs":[...],"authority_boundary":{"proof":"not_proof","gate_decision":"not_gate_decision","work_occurrence":"not_work_occurrence"}}`,
						Note:  "The case is derived from existing artifacts; it is not a new root kind, proof, approval, GateDecision, or performed work.",
					},
				},
				Notes: []string{
					"EvidencePath records are included only when attempted_use is declared.",
					"Missing referenced ProblemCards remain visible as refs instead of being fabricated.",
				},
			},
			OutputVolume: []string{"default: one JSON EngineeringChangeCase projection"},
			Invariants: append(commonInterfaceInvariants(),
				"EngineeringChangeCase is a derived projection, not a mutation or new FPF root kind.",
				"Default status, related, and code_context payloads do not inline change cases.",
			),
		},
		{
			ID:      "query.correspondence_graph",
			Purpose: "Build a read-only expected-vs-observed correspondence graph for one DecisionRecord.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "correspondence_graph",
				MCPCall:          `haft_query(action="correspondence_graph", artifact_ref="dec-...")`,
				CLIStatus:        "available",
				CLICommand:       "haft correspondence graph DECISION_REF --json",
				DiscoveryCommand: "haft interface query.correspondence_graph --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"artifact_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"path_status":"graph_path_not_proof","expected_realization":[...],"observed_realization":[...],"edges":[{"relation_kind":"...","origin":"declared","path_status":"graph_path_not_proof"}],"gaps":[...],"authority_boundary":{"proof":"not_proof","evidence":"not_evidence"}}`,
						Note:  "Edges are candidate correspondence paths; they are not evidence, proof, approval, GateDecision, or global truth.",
					},
				},
				Notes: []string{
					"Expected nodes come from decision intent, claims, and TransformationRecord when present.",
					"Observed nodes come from affected_files and evidence items; missing bindings stay as gaps.",
				},
			},
			OutputVolume: []string{"default: one JSON QualifiedCorrespondenceGraph"},
			Invariants: append(commonInterfaceInvariants(),
				"Graph path is not proof.",
				"Correspondence edges are qualified by origin and source refs.",
			),
		},
		{
			ID:      "query.drift_route",
			Purpose: "Build a read-only semantic drift taxonomy and repair-route projection.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "drift_route",
				MCPCall:          `haft_query(action="drift_route", drift_kind="evidence_binding_drift", bearer_ref="evid-...", use_context="release reliance")`,
				CLIStatus:        "available",
				CLICommand:       "haft drift route DRIFT_KIND --bearer-ref ... --use-context ... --json",
				DiscoveryCommand: "haft interface query.drift_route --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"drift_kind"},
				OptionalFields: []string{"bearer_ref", "use_context"},
				FieldShapes: []fieldShape{
					{
						Field: "drift_kind",
						Shape: `"carrier_drift" | "publication_faithfulness_drift" | "episteme_claim_drift" | "transformation_realization_drift" | "implementation_correspondence_drift" | "evidence_binding_drift" | ...`,
						Note:  "Unknown drift kinds fail closed with no_change/view and stronger-use block.",
					},
					{
						Field: "response",
						Shape: `{"drift_layer":"evidence|publication|episteme|...","candidate_repair_actions":[...],"language_state_move_kinds":[...],"authority_boundary":{"mutation":"not_mutation","gate_decision":"not_gate_decision"}}`,
						Note:  "Routing is advisory and does not execute repair or create evidence/approval.",
					},
				},
				Notes: []string{
					"Haft does not route description/evidence/publication drift directly to code repair.",
					"Use the route as review input; mutation still needs the governing decision/workflow.",
				},
			},
			OutputVolume: []string{"default: one JSON SemanticDriftRoute"},
			Invariants: append(commonInterfaceInvariants(),
				"Repair routing is read-only.",
				"Candidate repair actions are not performed work.",
			),
		},
		{
			ID:      "query.drift_events",
			Purpose: "Group per-decision drift findings into read-only DriftEvents with fanout and impacted current decisions.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "drift_events",
				MCPCall:          `haft_query(action="drift_events")`,
				CLIStatus:        "available",
				CLICommand:       "haft drift events --json; haft drift events resolve EVENT_ID --status resolved|waived_until --reason ...",
				DiscoveryCommand: "haft interface query.drift_events --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":2,"summary":{"unique_events":1,"impacted_decisions":7,"material_events":1,"audit_only_events":0,"needs_binding_resolution_events":1,"semantic_target_events":1,"file_fallback_events":0,"unknown_high_risk_events":0,"resolved_by_ledger_events":1,"waived_by_ledger_events":0,"max_fanout":7},"events":[{"event_id":"drift-event-...","changed_target_ref":"symbol:...","target_kind":"symbol|spec_section|api_contract|invariant|file|...","target_status":"modified|removed|renamed|retarget_candidate","trigger_kind":"file_hash","materiality":"material_symbol","root_cause":"semantic_target_changed|binding_target_missing|carrier_only_changed|generated_artifact_changed|target_deleted|target_renamed|retarget_candidate|schema_changed|unknown_high_risk","root_cause_detail":"...","fallback_kind":"whole_file_fallback|edited_symbol_move_candidate","fallback_reason":"unsupported language","fanout":7,"impacted_decisions":[...],"source_items":[{"changed_target_ref":"symbol:...","target_kind":"symbol","target_status":"renamed","symbols":[...],"fallback_kind":"whole_file_fallback"}],"resolution_status":"open|needs_scope_enrichment|needs_rebaseline|needs_reconcile|needs_operator_judgment|resolved|waived_until","resolution_record":{"event_id":"drift-event-...","status":"resolved|waived_until","reason":"...","evidence_refs":["..."],"waiver_expires_at":"2026-07-01"},"suggested_next_command":"haft decision reconcile --json|haft_refresh(action=\"review\")|..."}],"compatibility_reports":[...]}`,
						Note:  "Compatibility reports preserve the old per-decision shape; DriftEvents prefer symbol/semantic changed targets when available, fall back to file targets only when unresolved, keep source symbol details, expose fallback metadata, and can overlay non-binding resolution metadata from a local ledger.",
					},
				},
				Notes: []string{
					"Use this when a shared changed target creates several per-decision drift findings.",
					"One DriftEvent can carry many impacted decisions; file overlap alone is not a merge/supersede decision.",
					"needs_binding_resolution_events means fallback/imprecise binding must be resolved before treating the event as proven material authority drift.",
					"root_cause, resolution_status, and suggested_next_command are computed review posture, not evidence, approval, or gate passage.",
					"`haft drift events resolve` writes only DriftEvent resolution metadata with authority=drift_event_resolution_metadata_not_decision_authority.",
					"Resolution ledgers do not change baseline, evidence, lineage, DecisionRecord status, gate decisions, or carrier authority.",
				},
			},
			OutputVolume: []string{"default: JSON DriftEventReport drill-down; compact status may cue event counts but must not inline compatibility reports"},
			Invariants: append(commonInterfaceInvariants(),
				"DriftEvent aggregation is read-only.",
				"Fanout is not independent debt count.",
				"Compatibility per-decision drift reports remain available.",
			),
		},
		{
			ID:      "query.decision_reconcile",
			Purpose: "Build a read-only DecisionReconciliationPlan over current decisions.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "decision_reconcile",
				MCPCall:          `haft_query(action="decision_reconcile")`,
				CLIStatus:        "available",
				CLICommand:       "haft decision reconcile --json",
				DiscoveryCommand: "haft interface query.decision_reconcile --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"authority":"report_only_not_binding_authority","summary":{"reviewed_decisions":12,"merge_candidates":1,"conflict_requires_operator":0,"scope_enrichment_candidates":3},"groups":[{"category":"merge_candidate|reopen_candidate|keep|...","subject_ref":"...","bounded_context":"...","governance_targets":[...],"whole_file_fallback_targets":[...],"scope_repair_hints":["use enrich_scope ..."],"decision_refs":[...],"basis":[...],"operator_required":true,"preview":{"authority":"report_only_preview_not_binding_authority","read_only":true,"operation":"merge_through_successor|supersede|retire_without_successor|reopen|enrich_scope|claim_lifecycle_update|keep|operator_judgment_required","apply_operation":"merge_through_successor|...","current":{"decision_refs":[...],"statuses":[{"decision_ref":"dec-old","status":"active"}]},"proposed":{"statuses":[{"decision_ref":"dec-old","status":"superseded"}],"lineage_relations":[{"relation":"mergedFrom|supersedes|retiredWithSuccessor|retiredWithoutSuccessor","source_ref":"$successor_ref|dec-old","target_ref":"dec-old|dec-new","requires_successor_ref":true}],"effects":[...],"required_successor_ref":true},"required_selection_fields":["operator_approval_ref","items[].reviewed_group_id","items[].successor_ref|items[].claim_lifecycle_updates[]"],"validation_notes":["apply-ready only after an existing successor_ref is selected","preview is advisory and cannot authorize apply"],"downstream_impact":{"internal_edges":1,"external_edges":2,"incoming_edges":1,"outgoing_edges":2,"dependent_refs":["evid-1"],"review_cue":"external dependent refs require review before apply; this report does not relink downstream artifacts"},"downstream_migration_report":{"required_before_apply":true,"auto_relink":false,"policy":"review_dependents_before_apply_no_auto_relink","dependent_refs":["evid-1"],"review_steps":[...],"selection_impact":[...]},"consolidated_successor_workflow":{"required":true,"authority":"review_contract_not_binding_authority","binding_path":"create_or_select_successor_decision_then_apply_operator_approved_selection","existing_successor_ref_required":true,"required_packet_fields":["retained_claims","withdrawn_claims","changed_assumptions","remaining_evidence","governance_scope","drift_watch_targets","valid_until"],"mutation_boundary":["this preview does not create the successor"]},"mutation_boundary":["preview generation is read-only"]}}]}`,
						Note:  "Grouping requires explicit decision subject and governance-target overlap; affected_files are footprint hints and never merge evidence by themselves. Scope repair hints and previews are read-only prompts for operator-approved selection documents.",
					},
				},
				Notes: []string{
					"Use this after DriftEvents show high fanout or old decisions need authority-frontier cleanup.",
					"Report-only: it does not supersede, merge, retire, reopen, baseline, or mutate evidence.",
					"scope_enrichment_candidates are repair prompts, not automatic mutations.",
					"lineage_relations are preview labels, not authority mutations; apply still requires an operator-approved selection document.",
					"preview shows current/proposed state and required approval fields; it is not an apply operation.",
					"downstream_impact shows links/backlinks that must be reviewed before apply; it does not relink dependencies.",
					"downstream_migration_report names dependent refs and review policy before apply; it never relinks automatically.",
					"consolidated_successor_workflow names the successor packet review contract; it never creates or approves a successor.",
					"claim_lifecycle_update is an operator-approved apply operation for explicit claims; it keeps the parent DecisionRecord current.",
					"Apply/lineage mutation belongs to a separate explicit operator-approved slice.",
				},
			},
			OutputVolume: []string{"default: JSON DecisionReconciliationPlan drill-down; compact status may cue availability but must not inline groups"},
			Invariants: append(commonInterfaceInvariants(),
				"DecisionReconciliationPlan is read-only.",
				"File overlap alone is not a merge/supersede signal.",
				"Operator approval is required for every non-keep authority mutation candidate.",
				"Preview generation is read-only and cannot create selection documents or apply mutations.",
			),
		},
		{
			ID:      "query.governing_set",
			Purpose: "Build a read-only current governing set projection over active decision authority.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "governing_set",
				MCPCall:          `haft_query(action="governing_set", query="symbol:...")`,
				CLIStatus:        "available",
				CLICommand:       "haft decision governing-set --query symbol:... --json",
				DiscoveryCommand: "haft interface query.governing_set --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"query", "bearer_ref", "source_refs"},
				FieldShapes: []fieldShape{
					{
						Field: "filters",
						Shape: `{"query":"substring across set id, subject, target, decision refs, history refs, fallback targets, repair hints","bearer_ref":"exact subject_ref","source_refs":["exact target_ref"]}`,
						Note:  "CLI equivalents are --query, --subject-ref, and --target-ref. Filters are read-only drill-down constraints; they do not change the governing-set model.",
					},
					{
						Field: "response",
						Shape: `{"schema_version":1,"authority":"read_only_current_authority_frontier","snapshot":{"generated_at":"RFC3339","source":"artifact_store_decision_records","projection":"refreshable_current_governing_frontier","authority_boundary":"derived_read_only_not_gate_decision","current_status_policy":["active","refresh_due"],"terminal_status_policy":["superseded","deprecated"],"terminal_history_policy":"terminal decisions stay searchable history and are excluded from current authority","filter_applied":true},"filter":{"query":"symbol:...","subject_ref":"subject:...","target_ref":"symbol:..."},"summary":{"current_decisions":7,"governing_sets":5,"conflict_sets":1,"overlap_review_sets":1,"fallback_target_sets":1,"scope_enrichment_sets":2,"terminal_history_refs":3},"sets":[{"set_id":"governing-set-...","subject_ref":"...","bounded_context":"...","target_ref":"...","answer_paths":[{"target_kind":"claim|spec_section|api_contract|invariant|symbol|whole_file_fallback","target_ref":"...","cli":"haft decision governing-set --target-ref ... --json","mcp_call":"haft_query(action=\"governing_set\", source_refs=[...])","exact_record_needed":"...","authority_boundary":"answer_path_is_read_only_not_evidence_or_gate_decision"}],"target_resolution":"explicit_governance_or_watch_target|whole_file_fallback_requires_scope_enrichment","whole_file_fallback_targets":[...],"scope_repair_hints":["replace whole-file fallback ..."],"posture":"single_current_authority|overlap_needs_review|conflict_requires_operator","current_decision_refs":[...],"terminal_history_refs":[...],"operator_required":true,"authority_boundary":"derived_read_only_not_gate_decision"}]}`,
						Note:  "Terminal decisions are history refs, not current authority; conflicts, overlaps, and fallback scope repair hints are review cues, not automatic lineage mutations.",
					},
				},
				Notes: []string{
					"Use this after reconciliation planning to ask what currently governs a subject/context/target.",
					"Use query/source_refs/bearer_ref for focused drill-down instead of expanding default status.",
					"The projection is derived from active/refresh_due decisions and effective governance/drift targets.",
					"snapshot is provenance for the refreshable projection, not a persisted authority artifact.",
					"answer_paths give exact CLI/MCP drill-downs for claim/spec-section/API-contract/invariant/symbol targets; they are read-only affordances, not evidence.",
					"fallback_target_sets and scope_enrichment_sets point to old decisions that need operator-approved scope enrichment before stronger use.",
					"Read-only: it does not supersede, merge, retire, reopen, baseline, or create GateDecision records.",
				},
			},
			OutputVolume: []string{"default: compact CurrentGoverningSet summary", "--json: CurrentGoverningSetReport drill-down; compact status should only cue conflicts/overlap"},
			Invariants: append(commonInterfaceInvariants(),
				"CurrentGoverningSet is read-only.",
				"Terminal records remain searchable history, not live authority.",
				"Conflicting current authority requires operator review or explicit waiver before stronger use.",
			),
		},
		{
			ID:      "query.blocked_use_attention",
			Purpose: "Build a read-only object-first attention item for one blocked use with exact source return.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "blocked_use",
				MCPCall:          `haft_query(action="blocked_use", bearer_ref="dec-...", blocked_use="release reliance", source_refs=["dec-..."], exact_record_needed="current EvidencePath")`,
				CLIStatus:        "available",
				CLICommand:       "haft attention blocked BEARER_REF --blocked-use ... --source-ref ... --exact-record-needed ... --json",
				DiscoveryCommand: "haft interface query.blocked_use_attention --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"bearer_ref", "blocked_use"},
				OptionalFields: []string{"label", "finding_kind", "source_refs", "exact_record_needed", "next_actions", "role_ref", "valid_until"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"object":{"bearer_ref":"...","entity_or_subject_label":"..."},"blocked_use":"...","source_return":{"status":"source_return_declared|exact_record_needed|missing_source_return","source_refs":[...]},"next_admissible_actions":[...],"authority_boundary":{"work_plan":"not_work_plan","gate_decision":"not_gate_decision"}}`,
						Note:  "The item points back to exact source records; action invitations are not WorkPlans.",
					},
				},
				Notes: []string{
					"Use this when a compact cockpit cue is insufficient and the agent needs the exact object/source return before stronger use.",
					"Missing source refs fail closed as missing_source_return and suggest recover_exact_source_record.",
				},
			},
			OutputVolume: []string{"default: one JSON BlockedUseAttentionItem"},
			Invariants: append(commonInterfaceInvariants(),
				"Attention items are read-only review inputs, not WorkPlans or evidence.",
				"Default status, related, and code_context payloads do not inline attention items.",
			),
		},
		{
			ID:      "query.value_space",
			Purpose: "Build a read-only Haft engineering-value characteristic space with no single score.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "value_space",
				MCPCall:          `haft_query(action="value_space", bearer_ref="release-...", context="2026-Q3", method_ref="method-...", source_refs=["evid-..."])`,
				CLIStatus:        "available",
				CLICommand:       "haft value space BEARER_REF --window ... --method-ref ... --evidence-ref ... --json",
				DiscoveryCommand: "haft interface query.value_space --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"bearer_ref"},
				OptionalFields: []string{"context", "method_ref", "source_refs"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"score_policy":{"single_score":"no_single_haft_or_fpf_score","aggregation":"characteristic_space_only"},"characteristics":[{"bearer_ref":"...","method":"...","window":"...","denominator":"...","evidence_refs":[...],"reopen_condition":"..."}],"interpretation_rules":{"healthy_reopening":"healthy_reopening_not_counted_as_simple_failure"}}`,
						Note:  "Every characteristic names bearer, method, window, denominator, and evidence refs; missing evidence blocks a value claim.",
					},
				},
				Notes: []string{
					"Use source_refs for evidence refs in the compact MCP schema; CLI names them --evidence-ref.",
					"Healthy reopening is interpreted separately from avoidable rework or simple failure.",
				},
			},
			OutputVolume: []string{"default: one JSON HaftEngineeringValueECS projection"},
			Invariants: append(commonInterfaceInvariants(),
				"No single Haft or FPF value score exists.",
				"Value characteristics are review inputs, not evidence, approval, GateDecision, or global truth.",
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
		{
			ID:      "refresh.review",
			Purpose: "Build a read-only judgment packet for rung-3 maintenance tasks; never mutates, approves, or creates evidence.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_refresh",
				MCPAction:        "review",
				MCPCall:          `haft_refresh(action="review")`,
				CLIStatus:        "available",
				CLICommand:       "haft overseer judgment --json",
				DiscoveryCommand: "haft interface refresh.review --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"context"},
				Notes: []string{
					"Output groups only rung-3 needs-judgment tasks by recommendation, confidence, source, and category.",
					"Suggested commands are candidates for explicit operator approval; the packet is not evidence, approval, or mutation.",
				},
			},
			OutputVolume: []string{"default: compact grouped judgment packet", "CLI --json: full task list with source return and suggested commands"},
			Invariants: append(commonInterfaceInvariants(),
				"Rung-3 judgment remains outside automated execution.",
				"Confidence labels are review metadata, not authority.",
			),
		},
		{
			ID:      "refresh.drain",
			Purpose: "Explicitly execute machine-safe maintenance actions and return a closed/failed/needs_operator report.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_refresh",
				MCPAction:        "drain",
				MCPCall:          `haft_refresh(action="drain", dry_run=true)`,
				CLIStatus:        "available",
				CLICommand:       "haft overseer drain --dry-run --json",
				DiscoveryCommand: "haft interface refresh.drain --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"dry_run", "context"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":"maintenance_drain.v1","dry_run":true,"summary":{"executed_actions":1,"needs_operator_tasks":3,"reconciliation_proposal_count":2},"executed":[{"kind":"observable_run","evidence_refs":["evid-..."]}],"reconciliation_proposals":[{"kind":"high_fanout_reconciliation_review|fallback_scope_repair_review|fallback_governing_scope_review","suggested_command":"haft decision reconcile --json","authority_boundary":"read_only_reconciliation_proposal_not_binding_authority"}],"after_action":{"auto_closed_items":[...],"evidence_checked":[{"command":"go test ...","evidence_refs":["evid-..."]}],"remaining_operator_judgment":[...],"undo_commands":["haft overseer undo <run-id> <action-id>"],"authority_boundary":"after_action_report_only_not_binding_authority"},"needs_operator":[...]}`,
						Note:  "Reconciliation proposals and after_action are report-only; they do not supersede, retire, merge, approve, create decisions, or apply reconciliation selections.",
					},
				},
				Notes: []string{
					"dry_run=true proposes machine-safe actions without mutating; dry_run=false executes only rung-1/rung-2 safe actions.",
					"Material drift, semantic uncertainty, reopen/supersede choices, and weak waivers are returned as needs_operator.",
					"reconciliation_proposals are read-only review batches for high-fanout/fallback groups; suggested commands are inspect-only.",
					"after_action lists auto-closed items, evidence refs, remaining operator judgment, and undo commands for autonomous mutations.",
				},
			},
			OutputVolume: []string{"default: compact drain report", "CLI --json: executed actions, reconciliation_proposals, after_action, and needs_operator groups"},
			Invariants: append(commonInterfaceInvariants(),
				"Drain is opt-in; default status and refresh.review remain read-only.",
				"Drain does not create semantic approval, GateDecision, or global truth.",
				"Drain never applies DecisionReconciliationSelection documents.",
			),
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

type interfaceContractAuditReport struct {
	Kind          string                          `json:"kind"`
	SchemaVersion int                             `json:"schema_version"`
	Authority     string                          `json:"authority"`
	Summary       interfaceContractAuditSummary   `json:"summary"`
	Surfaces      []interfaceContractAuditSurface `json:"surfaces"`
	Notes         []string                        `json:"notes"`
}

type interfaceContractAuditSummary struct {
	Capabilities               int `json:"capabilities"`
	KernelOwnedContracts       int `json:"kernel_owned_contracts"`
	MCPMirroredActions         int `json:"mcp_mirrored_actions"`
	CLIAvailableSurfaces       int `json:"cli_available_surfaces"`
	BindingAuthoritySurfaces   int `json:"binding_authority_surfaces"`
	ReadOnlySurfaces           int `json:"read_only_surfaces"`
	LegacyTransportExceptions  int `json:"legacy_transport_exceptions"`
	SchemaCoveredSurfaces      int `json:"schema_covered_surfaces"`
	SchemaMissingSurfaces      int `json:"schema_missing_surfaces"`
	SchemaExcludedFields       int `json:"schema_excluded_fields"`
	ShapeCoveredSurfaces       int `json:"shape_covered_surfaces"`
	ShapeMissingSurfaces       int `json:"shape_missing_surfaces"`
	ShapeSkippedFields         int `json:"shape_skipped_fields"`
	ShapeGeneratorTargets      int `json:"shape_generator_targets"`
	ShapeGeneratorTargetFields int `json:"shape_generator_target_fields"`
	ValidatedMCPMirrors        int `json:"validated_mcp_mirrors"`
	ManualCLIContracts         int `json:"manual_cli_contracts"`
	UnvalidatedHostFragments   int `json:"unvalidated_host_fragments"`
}

type interfaceContractAuditSurface struct {
	CapabilityID          string                               `json:"capability_id"`
	MCPTool               string                               `json:"mcp_tool"`
	MCPAction             string                               `json:"mcp_action,omitempty"`
	CLIStatus             string                               `json:"cli_status"`
	CLICommand            string                               `json:"cli_command,omitempty"`
	ContractSources       []string                             `json:"contract_sources"`
	ContractSourcePosture string                               `json:"contract_source_posture"`
	HostSchemaPosture     string                               `json:"host_schema_posture"`
	SchemaPosture         string                               `json:"schema_posture"`
	AuthorityPosture      string                               `json:"authority_posture"`
	ValidationRefs        []string                             `json:"validation_refs"`
	LegacyException       bool                                 `json:"legacy_exception"`
	Notes                 []string                             `json:"notes,omitempty"`
	SchemaCoverage        interfaceContractAuditSchemaCoverage `json:"schema_coverage"`
	ShapeCoverage         interfaceContractAuditShapeCoverage  `json:"shape_coverage"`
}

type interfaceContractAuditSchemaCoverage struct {
	Checked        bool     `json:"checked"`
	Status         string   `json:"status"`
	MissingFields  []string `json:"missing_fields,omitempty"`
	ExcludedFields []string `json:"excluded_fields,omitempty"`
}

type interfaceContractAuditShapeCoverage struct {
	Checked               bool     `json:"checked"`
	Status                string   `json:"status"`
	MissingShapeFields    []string `json:"missing_shape_fields,omitempty"`
	GeneratorTargetFields []string `json:"generator_target_fields,omitempty"`
	SkippedFields         []string `json:"skipped_fields,omitempty"`
}

type interfaceContractGenerationReport struct {
	Kind          string                              `json:"kind"`
	SchemaVersion int                                 `json:"schema_version"`
	Authority     string                              `json:"authority"`
	Source        string                              `json:"source"`
	SourceDigest  string                              `json:"source_digest"`
	Summary       interfaceContractGenerationSummary  `json:"summary"`
	SurfacePolicy interfaceContractGenerationPolicy   `json:"surface_policy"`
	Targets       []interfaceContractGenerationTarget `json:"targets"`
	Notes         []string                            `json:"notes"`
}

type interfaceContractGenerationSummary struct {
	Capabilities            int `json:"capabilities"`
	GeneratorTargetSurfaces int `json:"generator_target_surfaces"`
	GeneratorTargetFields   int `json:"generator_target_fields"`
}

type interfaceContractGenerationPolicy struct {
	DefaultStatus         string   `json:"default_status"`
	DefaultCodeContext    string   `json:"default_code_context"`
	ToolsList             string   `json:"tools_list"`
	CompactCLI            string   `json:"compact_cli"`
	GeneratedDescriptions string   `json:"generated_descriptions"`
	RequiredGuards        []string `json:"required_guards"`
}

type interfaceContractGenerationTarget struct {
	CapabilityID       string   `json:"capability_id"`
	MCPTool            string   `json:"mcp_tool"`
	MCPAction          string   `json:"mcp_action"`
	GenerationMode     string   `json:"generation_mode"`
	SourceContract     string   `json:"source_contract"`
	HostSchemaPosture  string   `json:"host_schema_posture"`
	AuthorityPosture   string   `json:"authority_posture"`
	AuthorityBoundary  string   `json:"authority_boundary"`
	Fields             []string `json:"fields"`
	ValidationRefs     []string `json:"validation_refs"`
	NextValidationStep string   `json:"next_validation_step"`
}

func isInterfaceContractAuditID(id string) bool {
	switch id {
	case "contract-audit", "contract_audit", "query.contract_audit":
		return true
	default:
		return false
	}
}

func isInterfaceContractGenerationID(id string) bool {
	switch id {
	case "contract-generation", "contract_generation", "query.contract_generation":
		return true
	default:
		return false
	}
}

func buildInterfaceContractGenerationReport(catalog []interfaceCapability) interfaceContractGenerationReport {
	audit := buildInterfaceContractAuditReport(catalog)
	targets := make([]interfaceContractGenerationTarget, 0)

	for _, surface := range audit.Surfaces {
		if len(surface.ShapeCoverage.GeneratorTargetFields) == 0 {
			continue
		}
		target := interfaceContractGenerationTarget{
			CapabilityID:       surface.CapabilityID,
			MCPTool:            surface.MCPTool,
			MCPAction:          surface.MCPAction,
			GenerationMode:     "nested_mcp_schema_property_generation",
			SourceContract:     "kernel_interface_catalog_field_shapes",
			HostSchemaPosture:  surface.HostSchemaPosture,
			AuthorityPosture:   surface.AuthorityPosture,
			AuthorityBoundary:  "schema_visibility_not_operator_authorization",
			Fields:             uniqueSortedStrings(surface.ShapeCoverage.GeneratorTargetFields),
			ValidationRefs:     uniqueInterfaceContractAuditStrings(surface.ValidationRefs),
			NextValidationStep: "generate_or_validate_nested_schema_properties_from_kernel_interface_catalog",
		}
		targets = append(targets, target)
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].CapabilityID < targets[j].CapabilityID
	})

	summary := interfaceContractGenerationSummary{
		Capabilities:            audit.Summary.Capabilities,
		GeneratorTargetSurfaces: len(targets),
		GeneratorTargetFields:   countInterfaceContractGenerationFields(targets),
	}

	return interfaceContractGenerationReport{
		Kind:          "haft_interface_contract_generation_manifest",
		SchemaVersion: 1,
		Authority:     "read_only_generation_manifest_not_generated_schema",
		Source:        "kernel_interface_catalog_field_shapes",
		SourceDigest:  interfaceContractGenerationDigest(targets),
		Summary:       summary,
		SurfacePolicy: interfaceContractGenerationSurfacePolicy(),
		Targets:       targets,
		Notes: []string{
			"This manifest is derived from contract-audit generator targets; it is not a generated schema.",
			"Schema visibility is not operator authorization, binding authority, evidence, or gate passage.",
			"Default status must not inline this report; use haft interface contract-generation --json or haft_query(action=\"contract_generation\").",
		},
	}
}

func interfaceContractGenerationSurfacePolicy() interfaceContractGenerationPolicy {
	return interfaceContractGenerationPolicy{
		DefaultStatus:         "cue_or_count_only_never_inline_generation_manifest",
		DefaultCodeContext:    "lane_index_only_never_inline_generated_descriptions",
		ToolsList:             "action_enum_and_compact_description_only_no_generated_schema_fragments",
		CompactCLI:            "summary_counts_only_field_targets_require_json",
		GeneratedDescriptions: "drill_down_only_validate_with_carrier_semio_before_host_materialization",
		RequiredGuards: []string{
			"carrier_semio_authority_boundary",
			"tools_list_context_budget",
			"compact_status_no_manifest_inline",
			"code_context_lane_index_default",
		},
	}
}

func countInterfaceContractGenerationFields(targets []interfaceContractGenerationTarget) int {
	total := 0
	for _, target := range targets {
		total += len(target.Fields)
	}
	return total
}

func interfaceContractGenerationDigest(targets []interfaceContractGenerationTarget) string {
	payload, err := json.Marshal(targets)
	if err != nil {
		return "sha256:unavailable"
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", sum)
}

func buildInterfaceContractAuditReport(catalog []interfaceCapability) interfaceContractAuditReport {
	surfaces := make([]interfaceContractAuditSurface, 0, len(catalog))
	summary := interfaceContractAuditSummary{Capabilities: len(catalog)}
	toolProperties := interfaceContractAuditToolProperties()
	exclusions := interfaceContractAuditSchemaFieldExclusions()

	for _, capability := range catalog {
		surface := buildInterfaceContractAuditSurface(capability, toolProperties, exclusions)
		surfaces = append(surfaces, surface)

		if interfaceContractAuditContainsString(surface.ContractSources, "kernel_interface_catalog") {
			summary.KernelOwnedContracts++
		}
		if surface.SchemaPosture == "mcp_schema_mirrored" {
			summary.MCPMirroredActions++
		}
		if surface.CLIStatus == "available" || surface.CLIStatus == "input_file_execution_shipped" {
			summary.CLIAvailableSurfaces++
		}
		if interfaceContractAuditBindingSensitive(surface.AuthorityPosture) {
			summary.BindingAuthoritySurfaces++
		}
		if strings.Contains(surface.AuthorityPosture, "read_only") {
			summary.ReadOnlySurfaces++
		}
		if surface.LegacyException {
			summary.LegacyTransportExceptions++
		}
		if surface.SchemaCoverage.Status == "covered" {
			summary.SchemaCoveredSurfaces++
		}
		if surface.SchemaCoverage.Status == "missing_fields" || surface.SchemaCoverage.Status == "missing_mcp_tool_schema" {
			summary.SchemaMissingSurfaces++
		}
		summary.SchemaExcludedFields += len(surface.SchemaCoverage.ExcludedFields)
		if surface.ShapeCoverage.Status == "covered" || surface.ShapeCoverage.Status == "no_input_shapes" {
			summary.ShapeCoveredSurfaces++
		}
		if surface.ShapeCoverage.Status == "missing_shape_fields" {
			summary.ShapeMissingSurfaces++
		}
		if surface.ShapeCoverage.Status == "generator_targets" {
			summary.ShapeGeneratorTargets++
		}
		summary.ShapeSkippedFields += len(surface.ShapeCoverage.SkippedFields)
		summary.ShapeGeneratorTargetFields += len(surface.ShapeCoverage.GeneratorTargetFields)
		switch surface.HostSchemaPosture {
		case "validated_mcp_mirror", "validated_mcp_mirror_with_generator_targets":
			summary.ValidatedMCPMirrors++
		case "manual_cli_contract_not_generated":
			summary.ManualCLIContracts++
		default:
			summary.UnvalidatedHostFragments++
		}
	}

	return interfaceContractAuditReport{
		Kind:          "haft_interface_contract_audit",
		SchemaVersion: 1,
		Authority:     "read_only_contract_inventory_not_schema_generation",
		Summary:       summary,
		Surfaces:      surfaces,
		Notes: []string{
			"Kernel interface catalog is the audited contract source for this report.",
			"Host schema posture classifies each fragment as a validated mirror, manual CLI contract, or unvalidated host fragment.",
			"Schema visibility is not operator authorization, binding authority, evidence, or gate passage.",
			"Generated MCP/host schema work remains a later phase and must validate against this inventory.",
			"Default status must not inline this report; use haft interface contract-audit --json or haft_query(action=\"contract_audit\").",
		},
	}
}

func buildInterfaceContractAuditSurface(
	capability interfaceCapability,
	toolProperties map[string]map[string]interface{},
	exclusions map[string]map[string]bool,
) interfaceContractAuditSurface {
	execution := capability.CurrentExecution
	surface := interfaceContractAuditSurface{
		CapabilityID:     capability.ID,
		MCPTool:          execution.MCPTool,
		MCPAction:        execution.MCPAction,
		CLIStatus:        execution.CLIStatus,
		CLICommand:       execution.CLICommand,
		ContractSources:  []string{"kernel_interface_catalog"},
		SchemaPosture:    interfaceContractAuditSchemaPosture(execution),
		AuthorityPosture: interfaceContractAuditAuthorityPosture(capability),
		ValidationRefs:   interfaceContractAuditValidationRefs(capability),
		LegacyException:  interfaceContractAuditLegacyException(execution),
		SchemaCoverage:   interfaceContractAuditSchemaCoverageFor(capability, toolProperties, exclusions),
		ShapeCoverage:    interfaceContractAuditShapeCoverageFor(capability, toolProperties, exclusions),
	}
	surface.ContractSourcePosture = interfaceContractAuditContractSourcePosture(surface)
	surface.HostSchemaPosture = interfaceContractAuditHostSchemaPosture(surface)

	if surface.LegacyException {
		surface.Notes = append(surface.Notes, "old standalone transport differs by documented exception; do not treat it as current host schema truth")
	}
	if interfaceContractAuditBindingSensitive(surface.AuthorityPosture) {
		surface.Notes = append(surface.Notes, "binding-sensitive surface remains governed by explicit operator/manual authorization")
	}
	if surface.SchemaPosture == "not_in_mcp_schema" {
		surface.Notes = append(surface.Notes, "schema is discoverable through CLI/interface only in this slice")
	}

	return surface
}

func interfaceContractAuditContractSourcePosture(surface interfaceContractAuditSurface) string {
	if interfaceContractAuditContainsString(surface.ContractSources, "kernel_interface_catalog") {
		return "kernel_interface_catalog_source"
	}
	return "unclassified_contract_source"
}

func interfaceContractAuditHostSchemaPosture(surface interfaceContractAuditSurface) string {
	if surface.MCPTool == "" || surface.MCPAction == "" {
		return "manual_cli_contract_not_generated"
	}
	if surface.SchemaCoverage.Status != "covered" {
		return "unvalidated_host_schema_fragment"
	}
	if surface.ShapeCoverage.Status == "generator_targets" {
		return "validated_mcp_mirror_with_generator_targets"
	}
	return "validated_mcp_mirror"
}

func interfaceContractAuditShapeCoverageFor(
	capability interfaceCapability,
	toolProperties map[string]map[string]interface{},
	exclusions map[string]map[string]bool,
) interfaceContractAuditShapeCoverage {
	execution := capability.CurrentExecution
	if execution.MCPTool == "" || execution.MCPAction == "" {
		return interfaceContractAuditShapeCoverage{
			Checked: false,
			Status:  "not_mcp_backed",
		}
	}

	properties, ok := toolProperties[execution.MCPTool]
	if !ok {
		return interfaceContractAuditShapeCoverage{
			Checked:            true,
			Status:             "missing_mcp_tool_schema",
			MissingShapeFields: []string{"action"},
		}
	}

	inputFields := topLevelInterfaceContractFieldSet(capability.InputContract)
	missing := make([]string, 0)
	skipped := make([]string, 0)

	for _, shape := range capability.InputContract.FieldShapes {
		topLevel := topLevelInterfaceContractField(shape.Field)
		if topLevel == "" {
			continue
		}
		if interfaceContractAuditShapeFieldExcluded(capability.ID, shape.Field) {
			skipped = append(skipped, shape.Field)
			continue
		}
		if !inputFields[topLevel] {
			skipped = append(skipped, shape.Field)
			continue
		}
		if exclusions[capability.ID][topLevel] {
			skipped = append(skipped, shape.Field)
			continue
		}

		propertySchema, ok := properties[topLevel]
		if !ok {
			missing = append(missing, topLevel)
			continue
		}

		for _, missingField := range missingNestedShapeFields(topLevel, shape.Shape, propertySchema) {
			missing = append(missing, missingField)
		}
	}

	missing = uniqueSortedStrings(missing)
	skipped = uniqueSortedStrings(skipped)
	generatorTargets := make([]string, 0)
	realMissing := make([]string, 0, len(missing))
	for _, field := range missing {
		if interfaceContractAuditShapeGeneratorTarget(capability.ID, field) {
			generatorTargets = append(generatorTargets, field)
			continue
		}
		realMissing = append(realMissing, field)
	}
	generatorTargets = uniqueSortedStrings(generatorTargets)
	realMissing = uniqueSortedStrings(realMissing)

	status := "covered"
	if len(realMissing) > 0 {
		status = "missing_shape_fields"
	} else if len(generatorTargets) > 0 {
		status = "generator_targets"
	} else if len(capability.InputContract.FieldShapes) == len(skipped) {
		status = "no_input_shapes"
	}

	return interfaceContractAuditShapeCoverage{
		Checked:               true,
		Status:                status,
		MissingShapeFields:    realMissing,
		GeneratorTargetFields: generatorTargets,
		SkippedFields:         skipped,
	}
}

func missingNestedShapeFields(root string, shape string, propertySchema interface{}) []string {
	expected, ok := objectKeysFromShape(shape)
	if !ok || len(expected) == 0 {
		return nil
	}

	properties := nestedSchemaProperties(propertySchema)
	if len(properties) == 0 {
		return prefixShapeFields(root, expected)
	}

	missing := make([]string, 0)
	for _, field := range expected {
		if _, ok := properties[field]; !ok {
			missing = append(missing, root+"."+field)
		}
	}
	return missing
}

func objectKeysFromShape(shape string) ([]string, bool) {
	shape = strings.TrimSpace(shape)
	if !strings.HasPrefix(shape, "{") {
		return nil, false
	}

	decoded := map[string]interface{}{}
	if err := json.Unmarshal([]byte(shape), &decoded); err != nil {
		return nil, false
	}

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, true
}

func nestedSchemaProperties(schema interface{}) map[string]interface{} {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil
	}
	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		return properties
	}
	items, ok := schemaMap["items"].(map[string]interface{})
	if !ok {
		return nil
	}
	properties, _ := items["properties"].(map[string]interface{})
	return properties
}

func prefixShapeFields(root string, fields []string) []string {
	prefixed := make([]string, 0, len(fields))
	for _, field := range fields {
		prefixed = append(prefixed, root+"."+field)
	}
	return prefixed
}

func interfaceContractAuditSchemaCoverageFor(
	capability interfaceCapability,
	toolProperties map[string]map[string]interface{},
	exclusions map[string]map[string]bool,
) interfaceContractAuditSchemaCoverage {
	execution := capability.CurrentExecution
	if execution.MCPTool == "" || execution.MCPAction == "" {
		return interfaceContractAuditSchemaCoverage{
			Checked: false,
			Status:  "not_mcp_backed",
		}
	}

	properties, ok := toolProperties[execution.MCPTool]
	if !ok {
		return interfaceContractAuditSchemaCoverage{
			Checked:       true,
			Status:        "missing_mcp_tool_schema",
			MissingFields: []string{"action"},
		}
	}

	missing := make([]string, 0)
	excluded := make([]string, 0)
	for _, field := range topLevelInterfaceContractFields(capability.InputContract) {
		if exclusions[capability.ID][field] {
			excluded = append(excluded, field)
			continue
		}
		if _, ok := properties[field]; !ok {
			missing = append(missing, field)
		}
	}
	sort.Strings(missing)
	sort.Strings(excluded)

	status := "covered"
	if len(missing) > 0 {
		status = "missing_fields"
	}

	return interfaceContractAuditSchemaCoverage{
		Checked:        true,
		Status:         status,
		MissingFields:  missing,
		ExcludedFields: excluded,
	}
}

func interfaceContractAuditToolProperties() map[string]map[string]interface{} {
	server := fpf.NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})

	toolProperties := make(map[string]map[string]interface{})
	for _, tool := range server.ToolCatalog() {
		inputSchema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			continue
		}
		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		toolProperties[tool.Name] = properties
	}
	return toolProperties
}

func topLevelInterfaceContractFields(contract interfaceContract) []string {
	seen := topLevelInterfaceContractFieldSet(contract)
	fields := make([]string, 0, len(contract.RequiredFields)+len(contract.OptionalFields))
	for field := range seen {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func topLevelInterfaceContractFieldSet(contract interfaceContract) map[string]bool {
	fields := make(map[string]bool)
	for _, field := range append(contract.RequiredFields, contract.OptionalFields...) {
		topLevel := topLevelInterfaceContractField(field)
		if topLevel == "" {
			continue
		}
		fields[topLevel] = true
	}
	return fields
}

func topLevelInterfaceContractField(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	if idx := strings.Index(field, "."); idx >= 0 {
		field = field[:idx]
	}
	if idx := strings.Index(field, "[]"); idx >= 0 {
		field = field[:idx]
	}
	return strings.TrimSpace(field)
}

func interfaceContractAuditSchemaFieldExclusions() map[string]map[string]bool {
	exclude := map[string][]string{
		"decision.reconcile_apply":    {"schema_version", "authority", "operator_approval_ref", "items"},
		"note.record":                 {"task_context"},
		"problem.frame":               {"task_context"},
		"solution.explore":            {"task_context"},
		"decision.decide":             {"task_context"},
		"query.related":               {"ref", "artifact_id"},
		"query.governing_set":         {"source_refs"},
		"query.blocked_use_attention": {"label", "finding_kind", "next_actions", "role_ref", "valid_until"},
	}

	result := make(map[string]map[string]bool, len(exclude))
	for capabilityID, fields := range exclude {
		result[capabilityID] = make(map[string]bool, len(fields))
		for _, field := range fields {
			result[capabilityID][field] = true
		}
	}
	return result
}

func interfaceContractAuditShapeFieldExcluded(capabilityID string, field string) bool {
	exclude := map[string]map[string]bool{
		"solution.compare": {
			"scores": true,
		},
	}
	return exclude[capabilityID][field]
}

func interfaceContractAuditShapeGeneratorTarget(capabilityID string, field string) bool {
	targets := map[string][]string{
		"decision.decide": {
			"claims.claim",
			"claims.governance_target_refs",
			"claims.id",
			"claims.lifecycle_status",
			"claims.observable",
			"claims.retired_reason",
			"claims.successor_ref",
			"claims.threshold",
			"drift_watch_targets.binding_target",
			"drift_watch_targets.target_ref",
			"drift_watch_targets.trigger",
			"governance_targets.binding_target",
			"governance_targets.kind",
			"governance_targets.ref",
			"implementation_footprint.commits",
			"implementation_footprint.files",
			"implementation_footprint.work_refs",
		},
		"query.spec_use": {
			"operational_gate.bearer_ref",
			"operational_gate.evidence_refs",
			"operational_gate.expires_at",
			"operational_gate.gate_ref",
			"operational_gate.reopen_condition",
			"operational_gate.rule",
			"operational_gate.schema_version",
			"operational_gate.use_context",
		},
	}

	for _, target := range targets[capabilityID] {
		if field == target {
			return true
		}
	}
	return false
}

func interfaceContractAuditSchemaPosture(execution interfaceExecution) string {
	if execution.MCPTool == "haft_query" && execution.MCPAction != "" {
		return "mcp_schema_mirrored"
	}
	if execution.MCPTool != "" && execution.MCPAction != "" {
		return "mcp_tool_schema_mirrored"
	}
	if execution.CLICommand != "" {
		return "cli_contract_only"
	}
	return "not_in_mcp_schema"
}

func interfaceContractAuditAuthorityPosture(capability interfaceCapability) string {
	execution := capability.CurrentExecution
	if mcpActionRequiresOperatorConfirmation(execution.MCPTool, execution.MCPAction) {
		return "binding_denied_by_default_mcp"
	}
	if capability.ID == "decision.reconcile_apply" {
		return "cli_operator_approved_binding_apply"
	}
	if strings.HasPrefix(capability.ID, "query.") {
		return "read_only_drill_down"
	}
	if strings.HasPrefix(capability.ID, "method.") {
		return "method_run_workflow_state"
	}
	if strings.HasPrefix(capability.ID, "refresh.") && execution.MCPAction == "drain" {
		return "machine_safe_maintenance_with_explicit_policy"
	}
	return "non_binding_or_draft_surface"
}

func interfaceContractAuditBindingSensitive(posture string) bool {
	switch posture {
	case "binding_denied_by_default_mcp", "cli_operator_approved_binding_apply":
		return true
	default:
		return false
	}
}

func interfaceContractAuditValidationRefs(capability interfaceCapability) []string {
	refs := []string{"internal/cli/interface_test.go"}

	if capability.CurrentExecution.MCPTool != "" {
		refs = append(refs, "internal/fpf/server_test.go")
	}
	if capability.CurrentExecution.MCPTool == "haft_query" {
		refs = append(refs, "internal/cli/serve_parity_test.go")
	}
	if strings.Contains(capability.CurrentExecution.CLIStatus, "input_file") {
		refs = append(refs, "internal/cli/interface_test.go")
	}
	if strings.Contains(capability.ID, "carrier") {
		refs = append(refs, "internal/cli/carrier_manifest_test.go")
	}
	if strings.Contains(capability.ID, "decision_reconcile") || strings.Contains(capability.ID, "governing_set") || strings.Contains(capability.ID, "reconcile_apply") {
		refs = append(refs, "internal/cli/decision_reconcile_test.go")
	}

	return uniqueInterfaceContractAuditStrings(refs)
}

func interfaceContractAuditLegacyException(execution interfaceExecution) bool {
	if execution.MCPTool != "haft_query" {
		return false
	}

	switch execution.MCPAction {
	case "search", "status", "related", "projection", "fpf":
		return false
	default:
		return true
	}
}

func interfaceContractAuditContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uniqueInterfaceContractAuditStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func uniqueSortedStrings(values []string) []string {
	unique := uniqueInterfaceContractAuditStrings(values)
	sort.Strings(unique)
	return unique
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

func writeInterfaceContractAuditText(output io.Writer, report interfaceContractAuditReport) error {
	if _, err := fmt.Fprintf(output, "Haft interface contract audit v%d\n", report.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", report.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: capabilities=%d kernel_owned=%d mcp_mirrored=%d cli_available=%d binding_sensitive=%d read_only=%d legacy_exceptions=%d schema_coverage=%d covered/%d missing excluded_fields=%d shape_coverage=%d covered/%d missing generator_targets=%d fields=%d skipped_fields=%d host_fragments=%d validated_mcp/%d manual_cli/%d unvalidated\n",
		report.Summary.Capabilities,
		report.Summary.KernelOwnedContracts,
		report.Summary.MCPMirroredActions,
		report.Summary.CLIAvailableSurfaces,
		report.Summary.BindingAuthoritySurfaces,
		report.Summary.ReadOnlySurfaces,
		report.Summary.LegacyTransportExceptions,
		report.Summary.SchemaCoveredSurfaces,
		report.Summary.SchemaMissingSurfaces,
		report.Summary.SchemaExcludedFields,
		report.Summary.ShapeCoveredSurfaces,
		report.Summary.ShapeMissingSurfaces,
		report.Summary.ShapeGeneratorTargets,
		report.Summary.ShapeGeneratorTargetFields,
		report.Summary.ShapeSkippedFields,
		report.Summary.ValidatedMCPMirrors,
		report.Summary.ManualCLIContracts,
		report.Summary.UnvalidatedHostFragments,
	); err != nil {
		return err
	}

	for _, surface := range report.Surfaces {
		if _, err := fmt.Fprintf(output, "- %s source=%s host_schema=%s schema=%s schema_coverage=%s shape_coverage=%s authority=%s cli=%s\n", surface.CapabilityID, surface.ContractSourcePosture, surface.HostSchemaPosture, surface.SchemaPosture, surface.SchemaCoverage.Status, surface.ShapeCoverage.Status, surface.AuthorityPosture, surface.CLIStatus); err != nil {
			return err
		}
	}

	return nil
}

func writeInterfaceContractGenerationText(output io.Writer, report interfaceContractGenerationReport) error {
	if _, err := fmt.Fprintf(output, "Haft interface contract generation manifest v%d\n", report.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", report.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", report.Source, report.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: capabilities=%d generator_target_surfaces=%d generator_target_fields=%d\n",
		report.Summary.Capabilities,
		report.Summary.GeneratorTargetSurfaces,
		report.Summary.GeneratorTargetFields,
	); err != nil {
		return err
	}

	for _, target := range report.Targets {
		if _, err := fmt.Fprintf(output, "- %s mcp=%s/%s mode=%s fields=%d authority=%s\n", target.CapabilityID, target.MCPTool, target.MCPAction, target.GenerationMode, len(target.Fields), target.AuthorityBoundary); err != nil {
			return err
		}
	}
	if len(report.Targets) == 0 {
		if _, err := fmt.Fprintln(output, "- no current generator targets"); err != nil {
			return err
		}
	}

	return nil
}
