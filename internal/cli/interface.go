package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/spf13/cobra"
)

var interfaceJSON bool
var interfaceWriteSchemaFragments string
var interfaceWriteDescriptionFragments string
var interfaceCheckSchemaFragments string
var interfaceCheckDescriptionFragments string
var interfaceCheckMaterializedCarriers bool
var interfaceSyncMaterializedCarriers bool

var interfaceContractDigestMarkerRE = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

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
	interfaceCmd.Flags().StringVar(&interfaceWriteSchemaFragments, "write-schema-fragments", "", "write generated MCP action schema fragments to a deterministic JSON carrier file")
	interfaceCmd.Flags().StringVar(&interfaceWriteDescriptionFragments, "write-description-fragments", "", "write generated host/skill/plugin description fragments to a deterministic JSON carrier file")
	interfaceCmd.Flags().StringVar(&interfaceCheckSchemaFragments, "check-schema-fragments", "", "check a generated MCP action schema fragments carrier against current contracts without rewriting it")
	interfaceCmd.Flags().StringVar(&interfaceCheckDescriptionFragments, "check-description-fragments", "", "check a generated host/skill/plugin description fragments carrier against current contracts without rewriting it")
	interfaceCmd.Flags().BoolVar(&interfaceCheckMaterializedCarriers, "check-materialized-carriers", false, "check all listed host/skill/plugin/Pi materialized carriers for current generated-contract markers")
	interfaceCmd.Flags().BoolVar(&interfaceSyncMaterializedCarriers, "sync-materialized-carriers", false, "rewrite listed host/skill/plugin/Pi materialized carrier source-digest markers from current generated contracts")
	rootCmd.AddCommand(interfaceCmd)
}

func runInterface(cmd *cobra.Command, args []string) error {
	catalog := haftInterfaceCatalog()
	output := cmd.OutOrStdout()
	if interfaceContractMaterializationFlagCount() > 1 {
		return fmt.Errorf("contract-generation carrier write/check flags are mutually exclusive")
	}

	if len(args) == 0 {
		if interfaceContractMaterializationFlagCount() != 0 {
			return fmt.Errorf("contract-generation carrier write/check flags are only valid with `haft interface contract-generation`")
		}
		if interfaceJSON {
			return writeInterfaceCatalogJSON(output, catalog)
		}
		return writeInterfaceCatalogText(output, catalog)
	}

	capabilityID := strings.TrimSpace(args[0])
	if interfaceContractMaterializationFlagCount() != 0 && !isInterfaceContractGenerationID(capabilityID) {
		return fmt.Errorf("contract-generation carrier write/check flags are only valid with `haft interface contract-generation`")
	}
	if isInterfaceContractAuditID(capabilityID) {
		report := buildInterfaceContractAuditReport(catalog)
		if interfaceJSON {
			return writeJSON(output, report)
		}
		return writeInterfaceContractAuditText(output, report)
	}

	if isInterfaceContractGenerationID(capabilityID) {
		report := buildInterfaceContractGenerationReport(catalog)
		if interfaceWriteSchemaFragments != "" {
			result, err := materializeInterfaceContractSchemaFragments(report, interfaceWriteSchemaFragments)
			if err != nil {
				return err
			}
			if interfaceJSON {
				return writeJSON(output, result)
			}
			return writeInterfaceContractSchemaMaterializationText(output, result)
		}
		if interfaceWriteDescriptionFragments != "" {
			result, err := materializeInterfaceContractDescriptionFragments(report, interfaceWriteDescriptionFragments)
			if err != nil {
				return err
			}
			if interfaceJSON {
				return writeJSON(output, result)
			}
			return writeInterfaceContractDescriptionMaterializationText(output, result)
		}
		if interfaceCheckSchemaFragments != "" {
			result, err := checkInterfaceContractSchemaFragments(report, interfaceCheckSchemaFragments)
			if err != nil {
				return err
			}
			if interfaceJSON {
				return writeJSON(output, result)
			}
			return writeInterfaceContractCarrierCheckText(output, result)
		}
		if interfaceCheckDescriptionFragments != "" {
			result, err := checkInterfaceContractDescriptionFragments(report, interfaceCheckDescriptionFragments)
			if err != nil {
				return err
			}
			if interfaceJSON {
				return writeJSON(output, result)
			}
			return writeInterfaceContractCarrierCheckText(output, result)
		}
		if interfaceCheckMaterializedCarriers {
			result, err := checkInterfaceContractMaterializedCarriers(report)
			if err != nil {
				return err
			}
			if interfaceJSON {
				return writeJSON(output, result)
			}
			return writeInterfaceContractMaterializedCarrierCheckText(output, result)
		}
		if interfaceSyncMaterializedCarriers {
			result, err := syncInterfaceContractMaterializedCarriers(report)
			if err != nil {
				return err
			}
			if interfaceJSON {
				return writeJSON(output, result)
			}
			return writeInterfaceContractMaterializedCarrierSyncText(output, result)
		}
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

func interfaceContractMaterializationFlagCount() int {
	total := 0
	for _, value := range []string{
		interfaceWriteSchemaFragments,
		interfaceWriteDescriptionFragments,
		interfaceCheckSchemaFragments,
		interfaceCheckDescriptionFragments,
	} {
		if value != "" {
			total++
		}
	}
	if interfaceCheckMaterializedCarriers {
		total++
	}
	if interfaceSyncMaterializedCarriers {
		total++
	}
	return total
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
					"Use the input-file CLI/manual path for the binding act in default MCP cli-only mode; host authorization receipts require principal, session, action, payload hash, expiry, source, and a registered kernel verifier before they can become a binding path.",
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
				CLICommand:       "haft decision reconcile selection-review selection.json --json; haft decision reconcile apply selection.json --json",
				DiscoveryCommand: "haft interface decision.reconcile_apply --json",
				InputFileFlow: []string{
					"haft_query(action=\"decision_reconcile\") or `haft decision reconcile --json`",
					"operator approves a reviewed selection document",
					"haft decision reconcile selection-review selection.json --json",
					"haft decision reconcile apply selection.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"schema_version", "authority", "operator_approval_ref", "items"},
				FieldShapes: []fieldShape{
					{
						Field: "selection_document",
						Shape: `{"schema_version":1,"authority":"operator_approved_reconciliation_selection","operator_approval_ref":"chat:...","items":[{"operation":"merge_through_successor|supersede|retire_without_successor|reopen|enrich_scope|claim_lifecycle_update","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-old"],"successor_ref":"dec-new","decision_subject_ref":"subject:ref","governance_targets":[{"kind":"api_contract","ref":"api_contract:..."}],"drift_watch_targets":[{"target_ref":"api_contract:...","trigger":"schema_or_behavior_changed"}],"remove_whole_file_fallback_targets":["whole_file_fallback:.haft/solutions/sol-old.md"],"claim_governance_target_refs":{"claim-id":["api_contract:..."]},"claim_lifecycle_updates":[{"decision_ref":"dec-old","claim_id":"claim-1","lifecycle_status":"active|superseded|deprecated|refresh_due","successor_ref":"dec-new#claim-4","reason":"..."}],"reason":"..."}]}`,
						Note:  "successor_ref is required for supersede/merge_through_successor, forbidden for retire_without_successor/enrich_scope, and omitted for reopen; enrich_scope requires exactly one decision plus decision_subject_ref and governance_targets or drift_watch_targets; remove_whole_file_fallback_targets is optional for enrich_scope and may name only existing whole_file_fallback binding_targets on that decision; claim_lifecycle_update changes explicit claims only and keeps the parent DecisionRecord current; this apply path does not create binding decisions.",
					},
					{
						Field: "selection_review_response",
						Shape: `{"schema_version":1,"authority":"read_only_selection_review_not_apply_authority","apply_ready":false,"operator_approved":false,"document_authority":"report_only_selection_draft_not_operator_approval","required_authority":"operator_approved_reconciliation_selection","item_count":1,"validation_errors":["operator_approval_ref is required"],"items":[{"index":0,"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-old"],"apply_ready":true}],"mutation_boundary":["selection review is read-only","review does not create operator approval","review does not apply reconciliation selections"],"next_steps":["add operator_approval_ref that names the explicit approval event"]}`,
						Note:  "selection-review uses the same core validation as apply but never mutates; apply_command is emitted only when the document is already apply-ready.",
					},
				},
				Notes: []string{
					"This is an explicit lifecycle mutation path; run the report-only plan first and apply only reviewed selections.",
					"merge_through_successor requires an already-created successor DecisionRecord; this command does not create binding decisions.",
					"enrich_scope changes only DecisionRecord scope fields; it does not change status, lineage, evidence, baselines, or gates.",
					"enrich_scope may remove explicitly named existing whole-file fallback binding_targets when the operator-approved selection also provides exact governance_targets or drift_watch_targets.",
					"claim_lifecycle_update can supersede, deprecate, or mark explicit claims refresh_due without changing the parent DecisionRecord status.",
					"Validation covers the whole batch before mutation so a later invalid item cannot leave earlier items partially applied.",
					"selection-review is a read-only preflight for the approval packet; it cannot convert a draft into approval.",
				},
			},
			OutputVolume: []string{"default: compact applied operation list", "--json: DecisionReconciliationApplyResult"},
			Invariants: append(commonInterfaceInvariants(),
				"MCP has no reconciliation apply action in this slice.",
				"Operator approval ref is required in the selection document.",
				"Selection review is read-only and cannot create operator approval or apply mutations.",
				"Old decision IDs remain searchable; terminal status changes preserve lineage.",
				"Scope enrichment is additive: existing subjects cannot be retargeted by enrich_scope.",
				"Whole-file fallback removal is explicit and limited to named existing fallback binding_targets.",
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
						Shape: `{"kind":"haft_interface_contract_audit","schema_version":1,"authority":"read_only_contract_inventory_not_schema_generation","authority_boundary":{"inventory":"read_only_contract_inventory","schema_generation":"not_schema_generation","host_materialization":"not_host_materialization","evidence":"not_evidence","approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"},"summary":{"capabilities":34,"kernel_owned_contracts":34,"mcp_mirrored_actions":19,"cli_available_surfaces":26,"binding_authority_surfaces":2,"read_only_surfaces":19,"legacy_transport_exceptions":17,"schema_covered_surfaces":30,"schema_missing_surfaces":0,"schema_excluded_fields":15,"schema_required_covered_surfaces":30,"schema_required_missing_surfaces":0,"schema_missing_required_fields":0,"shape_covered_surfaces":30,"shape_missing_surfaces":0,"shape_skipped_fields":25,"shape_generator_targets":0,"shape_generator_target_fields":0,"validated_mcp_mirrors":30,"manual_cli_contracts":4,"unvalidated_host_fragments":0,"generated_target_fragments":0,"validated_fragments":30,"legacy_fragments":4,"unvalidated_fragments":0},"surfaces":[{"capability_id":"decision.decide","contract_sources":["kernel_interface_catalog"],"contract_fragment_posture":"validated_fragment","schema_posture":"mcp_schema_mirrored","authority_posture":"binding_denied_by_default_mcp","validation_refs":["internal/cli/interface_test.go","internal/fpf/server_test.go"],"legacy_exception":false,"schema_coverage":{"checked":true,"status":"covered","excluded_fields":["task_context"]},"shape_coverage":{"checked":true,"status":"covered"}}]}`,
						Note:  "The audit identifies contract fragments and validation posture; it does not generate schemas, materialize host descriptions, create evidence, approve binding actions, pass gates, create claim/global truth, publish, or change tool descriptions.",
					},
				},
				Notes: []string{
					"Use this before Phase F1 schema generation so host/schema drift is visible without inlining tool schemas into status.",
					"Read-only: schema visibility is not operator authorization and not binding authority.",
					"Contract audit is not evidence, approval, GateDecision, claim truth, global truth, publication, schema generation, or host materialization.",
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
			Purpose: "List generated preview fragments and remaining kernel-owned contract generator targets for MCP/host/skill/plugin/Pi synchronization without mutating host schemas.",
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
						Shape: `{"kind":"haft_interface_contract_generation_manifest","schema_version":1,"authority":"read_only_generation_manifest_not_host_materialization","source":"kernel_interface_catalog","source_digest":"sha256:...","validation_refs":["internal/cli/interface_test.go","internal/fpf/server_test.go"],"summary":{"capabilities":34,"generator_target_surfaces":0,"generator_target_fields":0,"generated_preview_fragments":34,"generated_schema_fragments":28,"binding_preview_fragments":2,"materialized_carriers":13,"digest_guarded_carriers":13,"authority_boundary_guarded_carriers":13},"surface_policy":{"default_status":"cue_or_count_only_never_inline_generation_manifest","default_code_context":"lane_index_only_never_inline_generated_descriptions","tools_list":"action_enum_and_compact_description_only_no_generated_schema_fragments","compact_cli":"summary_counts_only_field_targets_require_json","generated_descriptions":"drill_down_only_validate_with_carrier_semio_before_host_materialization","required_guards":["carrier_semio_authority_boundary","tools_list_context_budget","compact_status_no_manifest_inline","code_context_lane_index_default"]},"targets":[],"materialized_carriers":[{"carrier_path":"packages/haft-pi/extensions/haft/tools.ts","carrier_kind":"pi_tool_metadata","contract_role":"tool_schema_and_description_materialization","source_contract":"kernel_interface_catalog","expected_source_digest":"sha256:...","sync_posture":"digest_guarded_by_repo_regression","guard_posture":"source_digest_and_authority_boundary_guarded"}],"generated_fragments":[{"capability_id":"decision.decide","fragment_kind":"host_skill_plugin_description_preview","source_contract":"kernel_interface_catalog","source_digest":"sha256:...","authority_boundary":"binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts","generated_text":"...","input_fields":["choice_result","selected_title"]}],"generated_schema_fragments":[{"capability_id":"decision.decide","fragment_kind":"mcp_action_schema_fragment","schema_digest":"sha256:...","required_fields":["action"],"action_required_fields":["selected_title"],"handler_validated_fields":["selected_title"]}]}`,
						Note:  "The manifest is the kernel-owned generated-preview source plus any remaining generator queue; it does not materialize host schemas or authorize binding actions.",
					},
				},
				Notes: []string{
					"Use this after contract-audit: generated_fragments are the source for future host/skill/plugin/Pi wording; targets list remaining schema-generator work.",
					"Use `haft interface contract-generation --write-schema-fragments <path>` to materialize generated_schema_fragments as deterministic JSON validation carrier bytes.",
					"Use `haft interface contract-generation --write-description-fragments <path>` to materialize generated_fragments as deterministic JSON wording-sync carrier bytes.",
					"Use `--check-schema-fragments <path>` or `--check-description-fragments <path>` to fail when a materialized carrier drifted from the current kernel interface catalog.",
					"Use `--check-materialized-carriers` to validate every listed host/skill/plugin/Pi carrier for current source-digest and authority-boundary markers without rewriting files.",
					"Use `--sync-materialized-carriers` to rewrite only source-digest markers in listed repo carriers, then verify with the same marker check; it is not host runtime materialization.",
					"Read-only: generated fragments and schema generation targets are not operator authorization, evidence, or gate passage.",
				},
			},
			OutputVolume: []string{"default: compact generator target/fragment counts", "--json: generated fragments and field-level targets", "--write-schema-fragments/--write-description-fragments: deterministic generated carrier bytes; never in default status", "--check-schema-fragments/--check-description-fragments: compare generated fragment carriers without rewriting", "--check-materialized-carriers: compact repo carrier marker check", "--sync-materialized-carriers: explicit repo carrier source-digest marker sync plus check"},
			Invariants: append(commonInterfaceInvariants(),
				"Contract generation manifest is read-only.",
				"Generated fragments come from the kernel interface catalog.",
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
						Shape: `{"source_edition":{...},"baseline_currentness":{...},"current_authority":{"status":"clear|overlap_needs_review|conflict_requires_operator|unknown","authority_boundary":"read_only_current_authority_frontier_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication","decision_refs":[...],"target_refs":[...]},"admission":{...},"gate_decision":{"status":"not_applicable_no_operational_gate|passed|blocked","reason":"operational_gate_passed_for_declared_use|current_authority_conflict_requires_operator|...","authority_boundary":{"profile":"no_operational_gate_profile_not_gate_decision|read_only_derived_gate_evaluation","approval":"not_spec_approval","evidence":"not_evidence_creation","work_commission":"not_work_commission","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}}`,
						Note:  "Currentness, current-authority posture, admission, waiver expiry, and gate status are separate fields.",
					},
				},
				Notes: []string{
					"SpecificationUseRecord is read-only; it does not approve/rebaseline specs, create evidence, create WorkCommissions, or mutate an OperationalGate.",
					"Use policy=temporary_waiver only with waiver_expires_at; the waiver is represented in the response and is not global truth.",
					"current_authority is derived from the read-only governing-set frontier for the SpecSection target; conflicts and overlaps block OperationalGate passage.",
				},
			},
			OutputVolume: []string{"default: one JSON SpecificationUseRecord"},
			Invariants: append(commonInterfaceInvariants(),
				"Baseline currentness is not admission; admission policy is a distinct field.",
				"Current-authority conflict posture is not evidence, approval, GateDecision, claim truth, global truth, or publication; it is a fail-closed gate input when an OperationalGate is present.",
				"GateDecision passed/blocked is emitted only from an explicit OperationalGate profile.",
				"GateDecision remains a derived reading, not spec approval, evidence creation, WorkCommission, claim truth, global truth, publication, or execution authority.",
			),
		},
		{
			ID:      "spec.apply_change",
			Purpose: "Preview or apply one reviewed typed SpecSection carrier change into the SQL edition store.",
			CurrentExecution: interfaceExecution{
				CLIStatus:        "available",
				CLICommand:       "haft spec classify-change --before before.md --after after.md --section TS.x --json; haft spec apply-change --dry-run --before before.md --after after.md --section TS.x --json; haft spec apply-change --before before.md --after after.md --section TS.x --json",
				DiscoveryCommand: "haft interface spec.apply_change --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"before", "after", "section"},
				OptionalFields: []string{"kind", "dry_run", "json"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"authority_boundary":"sql_edition_update_not_approval_rebaseline_evidence_gate_claim_truth_global_truth_or_prose_authority","applied":false,"dry_run":true,"would_apply":true,"noop":false,"change":{"kind":"semantic_field_update|relationship_update|mixed_semantic_and_relationship_update|carrier_only|unknown_high_risk","import_posture":"recognized_update|no_semantic_mutation|abstain_block","scalar_fields":["title"],"relationship_fields":["depends_on"],"requires_operator_act":true},"planned_edition":{"source_kind":"markdown_sync_back","semantic_hash":"sha256..."}}`,
						Note:  "Dry-run runs the same typed parser and SQL freshness/conflict guard as apply but reports planned_edition instead of writing edition.",
					},
				},
				Notes: []string{
					"Use classify-change first for read-only field classification; use apply-change --dry-run for SQL-conflict-aware preview.",
					"Only typed fenced yaml spec-section fields participate; surrounding Markdown prose is never SQL truth.",
					"Recognized scalar, relationship, or mixed updates may write a markdown_sync_back SQL edition only through explicit apply-change.",
					"Carrier-only changes are no-op; unknown/high-risk changes fail closed.",
					"The command never approves, rebaselines, reopens, creates evidence, or mutates SpecSectionApprovalBaseline rows.",
				},
			},
			OutputVolume: []string{"default text summary; --json exact result; --dry-run reports planned_edition without edition write"},
			Invariants: append(commonInterfaceInvariants(),
				"SQL edition store remains the source of truth.",
				"Markdown prose is not authority; only typed spec-section fields can sync back.",
				"Sync-back mutation is not approval, rebaseline, evidence, GateDecision, claim truth, global truth, or prose authority.",
				"Default status must not inline apply-change contract details.",
			),
		},
		{
			ID:      "spec.export",
			Purpose: "Render one current SQL SpecSection edition as a deterministic Markdown publication projection.",
			CurrentExecution: interfaceExecution{
				CLIStatus:        "available",
				CLICommand:       "haft spec export TS.x --json; haft spec export TS.x --markdown",
				DiscoveryCommand: "haft interface spec.export --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"section_id"},
				OptionalFields: []string{"json", "markdown"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"authority_boundary":"publication_projection_only_not_approval_rebaseline_evidence_gate_claim_truth_or_global_truth","source_of_truth":"sql_project_graph","edition":{"section_id":"TS.x","semantic_hash":"sha256...","source_kind":"sql|carrier_import|markdown_sync_back","carrier_path":".haft/specs/target-system.md"},"publication":{"source_edition_hash":"sha256...","publication_hash":"sha256...","publication_projection":"typed_yaml_spec_section","carrier_path":".haft/specs/target-system.md","markdown":"yaml spec-section carrier bytes"},"audit":{"source_episteme":"sql_spec_section_edition","publication_projection":"typed_yaml_spec_section","carrier_bytes":".haft/specs/target-system.md","authority_boundary":"source_publication_carrier_audit_not_approval_rebaseline_evidence_gate_claim_truth_or_global_truth"}}`,
						Note:  "JSON separates source semantic edition, publication projection, and carrier bytes; --markdown prints carrier bytes only.",
					},
				},
				Notes: []string{
					"Run `haft spec sync` first when no current SQL edition exists.",
					"Export is read-only and never approves, rebaselines, reopens, creates evidence, or treats Markdown prose as authority.",
					"The renderer fails closed when the projected typed YAML would lose semantic identity on parse.",
					"`--markdown` intentionally omits audit fields so carrier bytes do not masquerade as authority metadata.",
				},
			},
			OutputVolume: []string{"default text summary; --json exact source/publication/audit record; --markdown carrier bytes only"},
			Invariants: append(commonInterfaceInvariants(),
				"SQL edition store remains the source of truth.",
				"Publication projection is not approval, rebaseline, evidence, GateDecision, claim truth, global truth, or prose authority.",
				"Carrier bytes are separated from source semantic edition and publication hash in JSON.",
				"Default status must not inline spec export publication details.",
			),
		},
		{
			ID:      "query.evidence_path",
			Purpose: "Build a read-only EvidencePath/RelianceDisposition record for one evidence item and declared attempted use.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "evidence_path",
				MCPCall:          `haft_query(action="evidence_path", artifact_ref="dec-...", evidence_ref="evid-...", attempted_use="verification reliance", requires_current_formality=true, method_ref="mpull-...")`,
				CLIStatus:        "available",
				CLICommand:       "haft evidence path ARTIFACT_REF EVIDENCE_REF --attempted-use ... --requires-current-formality --method-ref ... --json",
				DiscoveryCommand: "haft interface query.evidence_path --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"artifact_ref", "evidence_ref", "attempted_use"},
				OptionalFields: []string{"claim_ref", "requires_current_formality", "producer_ref", "method_ref", "work_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"attempted_use":{"requires_current_formality":true},"evidence":{"formality_level":7,"formality_scale":{"scale_id":"fpf-2026-f0-f9","level":7},"formality_bridge":{"loss":"source-scale-not-declared"},"formality_diagnostics":["legacy_formality_projection_lossy|unversioned_formality_source_scale_missing|current_f0_f9_formality"]},"claim_binding":{...},"trace_binding":{...},"currentness_window":{...},"reliance_disposition":{"disposition":"bounded_reliance|advisory_only|blocked"},"authority_boundary":{"approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}`,
						Note:  "Reliance is bounded to the declared use; formality scale/bridge/diagnostics are diagnostics and never create approval, gate passage, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"EvidencePathRecord is read-only and derived from an existing EvidenceItem.",
					"Missing attempted use, missing trace refs, expired evidence, refuting evidence, or an unbound requested claim cannot produce bounded reliance.",
					"`requires_current_formality=true` blocks bounded reliance when the evidence only carries legacy or undeclared/lossy formality.",
					"Evidence/formality diagnostics do not create approval, gate passage, claim truth, global truth, or publication.",
					"Exact/audit views must name formality scale and bridge/loss when evidence uses legacy or undeclared formality levels.",
				},
			},
			OutputVolume: []string{"default: one JSON EvidencePathRecord"},
			Invariants: append(commonInterfaceInvariants(),
				"Evidence presence is not approval, gate passage, claim truth, global truth, or publication.",
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
						Shape: `{"case_ref":"change-case:dec-...","problem_card_refs":[...],"transformation_refs":[...],"choice_result_ref":"...","evidence_item_refs":[...],"evidence_path_refs":[...],"authority_boundary":{"proof":"not_proof","approval":"not_approval","gate_decision":"not_gate_decision","work_occurrence":"not_work_occurrence","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}`,
						Note:  "The case is derived from existing artifacts; it is not a new root kind, proof, approval, GateDecision, performed work, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"EvidencePath records are included only when attempted_use is declared.",
					"Missing referenced ProblemCards remain visible as refs instead of being fabricated.",
					"EngineeringChangeCase projections do not create approval, gate passage, performed work, claim truth, global truth, or publication.",
				},
			},
			OutputVolume: []string{"default: one JSON EngineeringChangeCase projection"},
			Invariants: append(commonInterfaceInvariants(),
				"EngineeringChangeCase is a derived projection, not a mutation, new FPF root kind, approval, gate passage, performed work, claim truth, global truth, or publication.",
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
						Shape: `{"path_status":"graph_path_not_proof","expected_realization":[...],"observed_realization":[...],"edges":[{"relation_kind":"...","origin":"declared","path_status":"graph_path_not_proof"}],"gaps":[...],"authority_boundary":{"proof":"not_proof","evidence":"not_evidence","approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}`,
						Note:  "Edges are candidate correspondence paths; they are not evidence, proof, approval, GateDecision, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Expected nodes come from decision intent, claims, and TransformationRecord when present.",
					"Observed nodes come from affected_files and evidence items; missing bindings stay as gaps.",
					"Graph paths do not create evidence, proof, approval, gate passage, claim truth, global truth, or publication.",
				},
			},
			OutputVolume: []string{"default: one JSON QualifiedCorrespondenceGraph"},
			Invariants: append(commonInterfaceInvariants(),
				"Graph path is not proof.",
				"Correspondence edges are qualified by origin and source refs and never create evidence, approval, gate passage, claim truth, global truth, or publication.",
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
						Shape: `"carrier_only" | "carrier_drift" | "publication_faithfulness_drift" | "episteme_claim_drift" | "transformation_realization_drift" | "implementation_correspondence_drift" | "evidence_binding_drift" | ...`,
						Note:  "Unknown drift kinds fail closed with no_change/view and stronger-use block.",
					},
					{
						Field: "response",
						Shape: `{"drift_layer":"evidence|publication|episteme|...","candidate_repair_actions":[...],"language_state_move_kinds":[...],"authority_boundary":{"mutation":"not_mutation","evidence":"not_evidence","approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}`,
						Note:  "Routing is advisory and does not execute repair or create evidence, approval, GateDecision, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Haft does not route description/evidence/publication drift directly to code repair.",
					"Use the route as review input; mutation, evidence, approval, GateDecision, claim truth, global truth, and publication still need their own governing workflows.",
				},
			},
			OutputVolume: []string{"default: one JSON SemanticDriftRoute"},
			Invariants: append(commonInterfaceInvariants(),
				"Repair routing is read-only: candidate repair actions are not mutation, evidence, approval, GateDecision, claim truth, global truth, or publication.",
				"Candidate repair actions are not performed work.",
			),
		},
		{
			ID:      "query.drift_events",
			Purpose: "Group per-decision drift findings into read-only DriftEvents with fanout and impacted current decisions.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "drift_events",
				MCPCall:          `haft_query(action="drift_events", limit=5)`,
				CLIStatus:        "available",
				CLICommand:       "haft drift events --json; haft drift events resolve EVENT_ID --status resolved|waived_until --reason ...",
				DiscoveryCommand: "haft interface query.drift_events --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"limit", "full"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":2,"view":"compact","summary":{"unique_events":1,"impacted_decisions":7,"material_events":1,"audit_only_events":0,"needs_binding_resolution_events":1,"semantic_target_events":1,"file_fallback_events":0,"unknown_high_risk_events":0,"resolved_by_ledger_events":1,"waived_by_ledger_events":0,"max_fanout":7},"events":[{"event_id":"drift-event-...","changed_target_ref":"symbol:...","target_kind":"symbol|spec_section|api_contract|invariant|file|...","target_status":"modified|removed|renamed|retarget_candidate","trigger_kind":"file_hash","materiality":"material_symbol","root_cause":"semantic_target_changed|binding_target_missing|carrier_only_changed|generated_artifact_changed|target_deleted|target_renamed|retarget_candidate|implementation_footprint_churn|schema_changed|unknown_high_risk","root_cause_detail":"...","fallback_kind":"whole_file_fallback|edited_symbol_move_candidate","fallback_reason":"unsupported language","fanout":7,"impacted_decisions":[...],"omitted_impacted_decisions":2,"omitted_source_items":3,"resolution_status":"open|needs_scope_enrichment|needs_rebaseline|needs_reconcile|needs_operator_judgment|resolved|waived_until","resolution_record_posture":"applied|stale_event_binding|inactive_waiver|unsupported_status","resolution_record":{"event_id":"drift-event-...","status":"resolved|waived_until","reason":"...","evidence_refs":["..."],"waiver_expires_at":"2026-07-01","changed_target_ref":"symbol:...","target_kind":"symbol","target_status":"modified","materiality":"material_symbol","audit_only":false,"root_cause":"semantic_target_changed"},"suggested_next_command":"haft drift bindings --dry-run --json|haft decision reconcile --json|haft_refresh(action=\"review\")|..."}],"omitted_events":141,"omitted_compatibility_reports":34,"full_audit_command":"haft_query(action=\"drift_events\", full=true)"}`,
						Note:  "Default MCP response is compact. limit caps compact events and each event's inlined impacted_decisions; status/prompt-governor recommendations use limit=5. full=true restores source_items, compatibility_reports, and complete impacted_decisions. DriftEvents prefer symbol/semantic changed targets when available, fall back to file targets only when unresolved, keep source symbol details, expose fallback metadata, and can overlay non-binding resolution metadata from a local ledger.",
					},
				},
				Notes: []string{
					"Use this when a shared changed target creates several per-decision drift findings.",
					"One DriftEvent can carry many impacted decisions; file overlap alone is not a merge/supersede decision.",
					"needs_binding_resolution_events means fallback/imprecise binding must be resolved before treating the event as proven material authority drift.",
					"Binding-target-missing events route to ranked `haft drift bindings --dry-run --json` review before any reconciliation apply.",
					"root_cause, resolution_status, and suggested_next_command are computed review posture, not evidence, approval, or gate passage.",
					"`haft drift events resolve` writes only DriftEvent resolution metadata with authority=drift_event_resolution_metadata_not_decision_authority.",
					"Resolution ledgers do not change baseline, evidence, lineage, DecisionRecord status, gate decisions, or carrier authority.",
				},
			},
			OutputVolume: []string{"default: compact JSON DriftEventReport; full=true: complete audit payload with source_items and compatibility_reports"},
			Invariants: append(commonInterfaceInvariants(),
				"DriftEvent aggregation is read-only.",
				"Fanout is not independent debt count.",
				"Compatibility per-decision drift reports remain available.",
			),
		},
		{
			ID:      "drift.binding_review",
			Purpose: "Review legacy DecisionRecord file-scope bindings and propose precise binding targets without mutating authority.",
			CurrentExecution: interfaceExecution{
				CLIStatus:        "available",
				CLICommand:       "haft drift bindings --dry-run --json; haft drift bindings --json; haft drift bindings --apply-high-confidence --json; haft drift bindings --apply-selection selection.json --json",
				DiscoveryCommand: "haft interface drift.binding_review --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"dry_run", "json", "limit", "apply_high_confidence", "apply_selection", "include_clean"},
				FieldShapes: []fieldShape{
					{
						Field: "dry_run_response",
						Shape: `{"schema_version":"legacy_binding_report.v2","authority":"binding_target_review_proposal","view":"compact","summary":{"total_decisions":40,"already_precise":7,"missing_symbol_baseline":11,"missing_binding_targets":1,"ambiguous_file_scope":17,"high_confidence_proposals":0,"needs_operator_selection":29,"carrier_or_generated_only":4,"no_parseable_symbols":0},"items":[{"decision_id":"dec-...","posture":"ambiguous_file_scope|missing_symbol_baseline|missing_binding_targets|already_symbol_baselined|carrier_or_generated_only|no_parseable_symbols","recommended_action":"needs_operator_symbol_selection|propose_rebaseline_with_binding_targets|keep_legacy_file_scope|no_action","high_confidence":false,"ranking_policy":"review_only_title_file_kind_rank_not_binding_authority","candidate_symbol_preview":[{"file_path":"internal/cli/serve.go","symbol_name":"handleQuintQuery","symbol_kind":"func","review_rank":1,"review_score":14,"matched_terms":["query"],"ranking_signals":["symbol_title_match","source_file"]}],"candidate_symbols_omitted":15,"candidate_review_groups":[{"file_path":"internal/cli/serve.go","candidate_count":8,"candidate_symbol_preview":[...],"candidate_symbols_omitted":5,"best_review_score":14,"matched_terms":["query"],"ranking_signals":["symbol_title_match","source_file"]}],"candidate_review_groups_omitted":2,"diagnostic_preview":[{"file_path":"internal/cli/serve.go","kind":"needs_binding_resolution","severity":"block"}],"diagnostics_omitted":3,"binding_targets":[...],"full_candidate_audit_command":"haft drift bindings --json"}],"omitted_items":24,"full_audit_command":"haft drift bindings --json"}`,
						Note:  "Dry-run JSON is compact by default: candidate symbols are ranked and grouped for review using decision-title, file-locality, and symbol-kind cues; diagnostics are previewed with omitted counts, and long diagnostic messages stay in the full audit path. Ranking is review-only and not binding authority. --limit N changes the compact sample size; the full audit path is explicit and separate.",
					},
					{
						Field: "full_audit_response",
						Shape: `{"schema_version":"legacy_binding_report.v2","authority":"binding_target_review_proposal","summary":{...},"items":[...],"applied":[...]}`,
						Note:  "`haft drift bindings --json` keeps the complete review payload for audit and operator packet construction.",
					},
					{
						Field: "mutation_flags",
						Shape: `"--apply-high-confidence" | "--apply-selection selection.json"`,
						Note:  "Mutation flags are mutually exclusive with --dry-run and remain CLI-only binding-target repair paths.",
					},
				},
				Notes: []string{
					"Use this when status reports Binding resolution needed or old decisions still carry ambiguous whole-file scope.",
					"The report proposes binding_targets; it does not supersede, merge, retire, reopen, baseline, approve, or create evidence.",
					"`--dry-run` with apply flags fails closed before project DB access.",
					"Compact dry-run JSON is for token-bounded review and omits full diagnostic messages; use `haft drift bindings --json` for exact candidate and diagnostic audit.",
					"`ranking_policy`, `review_rank`, and `candidate_review_groups` are review aids only and do not create high-confidence apply authority.",
					"High-confidence apply only projects already-resolved precise targets; operator-selection apply requires an explicit reviewed selection file.",
				},
			},
			OutputVolume: []string{"--dry-run --json: compact JSON review with omitted count and full_audit_command", "--json without --dry-run: complete LegacyBindingReport audit payload"},
			Invariants: append(commonInterfaceInvariants(),
				"Binding review is not decision authority.",
				"Default dry-run output is compact with omitted counts, bounded candidate/diagnostic previews, and a full audit recovery command.",
				"Dry-run cannot be combined with binding mutation flags.",
				"Whole-file fallback remains unresolved until a precise binding target is selected or safely projected.",
			),
		},
		{
			ID:      "query.decision_reconcile",
			Purpose: "Build a read-only DecisionReconciliationPlan over current decisions.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "decision_reconcile",
				MCPCall:          `haft_query(action="decision_reconcile", limit=5)`,
				CLIStatus:        "available",
				CLICommand:       "haft decision reconcile --json --limit 5; haft decision reconcile --json; haft decision reconcile selection-draft --json; haft decision reconcile selection-draft --write-review-packet review.json --json; haft decision reconcile selection-draft --write-template selection.json --json; haft decision reconcile selection-draft --json --full; haft decision reconcile metrics --json",
				DiscoveryCommand: "haft interface query.decision_reconcile --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"limit", "full"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"authority":"report_only_not_binding_authority","view":"compact","file_overlap_policy":"affected_files are implementation-footprint hints; file overlap alone is never merge evidence","summary":{"reviewed_decisions":12,"merge_candidates":1,"conflict_requires_operator":0,"scope_enrichment_candidates":3},"compact_groups":[{"group_id":"reconcile-group-...","category":"merge_candidate|reopen_candidate|keep|...","subject_ref":"...","bounded_context":"...","scope_repair_hints":["use enrich_scope ..."],"decision_refs":[...],"fanout":7,"operator_required":true,"preview_operation":"merge_through_successor|supersede|retire_without_successor|reopen|enrich_scope|claim_lifecycle_update|keep|operator_judgment_required","apply_operation":"merge_through_successor|...","downstream_dependents":3,"downstream_migration_required":true,"successor_workflow_required":true}],"omitted_groups":42,"full_audit_command":"haft_query(action=\"decision_reconcile\", full=true)"}`,
						Note:  "Default MCP response is compact. limit caps compact groups; status/prompt-governor recommendations use limit=5. CLI `--json --limit N` returns the same compact projection; CLI `--json` without limit remains full audit JSON for backward compatibility. affected_files are footprint hints, not merge evidence. full=true restores groups[].preview with authority=report_only_preview_not_binding_authority, required_selection_fields including items[].successor_ref, validation_notes, lineage_relations labeled mergedFrom/supersedes/retiredWithSuccessor/retiredWithoutSuccessor, downstream_impact, downstream_migration_report, and consolidated_successor_workflow. preview is advisory and cannot authorize apply; downstream impact does not relink downstream artifacts.",
					},
					{
						Field: "metrics_response",
						Shape: `{"schema_version":1,"authority":"read_only_reconciliation_metrics_not_binding_authority","capture_policy":"capture_before_and_after_operator_approved_reconciliation_apply","reconciliation":{"reviewed_decisions":68,"whole_file_fallback_only":0,"missing_explicit_subject":68,"scope_enrichment_candidates":68,"conflict_requires_operator":0},"governing_set":{"current_decisions":68,"governing_sets":114,"fallback_target_sets":0,"scope_enrichment_sets":114,"conflict_sets":0,"overlap_review_sets":0,"terminal_history_refs":13},"drift_events":{"unique_events":161,"impacted_decisions":37,"material_events":107,"audit_only_events":54,"needs_binding_resolution_events":0,"semantic_target_events":81,"file_fallback_events":26,"unknown_high_risk_events":26,"max_fanout":28},"before_after_use":{"before_command":"haft decision reconcile metrics --json","apply_command":"haft decision reconcile apply SELECTION.json --json","after_command":"haft decision reconcile metrics --json","required_authority":"operator_approved_reconciliation_selection","mutation_boundary":["metrics capture is read-only","selection apply remains the only mutation step"]}}`,
						Note:  "Metrics packets are read-only before/after evidence for dogfood scope enrichment; they do not approve or apply a selection.",
					},
					{
						Field: "selection_draft_response",
						Shape: `{"schema_version":1,"authority":"report_only_selection_draft_not_operator_approval","operator_approved":false,"apply_authority_required":"operator_approved_reconciliation_selection","summary":{"reviewed_groups":69,"plan_scope_enrichment_candidates":7,"reviewable_scope_enrichment_candidates":6,"scope_enrichment_candidates":6,"operator_approval_candidates":6,"emitted_candidates":5,"omitted_candidates":1,"selected_candidates":0},"omitted_items":1,"full_audit_command":"haft decision reconcile selection-draft --json --full","current_metrics":{"schema_version":1,"authority":"read_only_reconciliation_metrics_not_binding_authority","reconciliation":{"reviewed_decisions":69,"groups":69,"scope_enrichment_candidates":7},"governing_set":{"current_decisions":69,"governing_sets":231,"fallback_target_sets":7,"scope_enrichment_sets":13,"conflict_sets":0,"terminal_history_refs":13},"drift_events":{"unique_events":246,"needs_binding_resolution_events":36,"max_fanout":29},"before_after_use":{"before_command":"haft decision reconcile metrics --json","apply_command":"haft decision reconcile apply SELECTION.json --json","after_command":"haft decision reconcile metrics --json","required_authority":"operator_approved_reconciliation_selection","mutation_boundary":["metrics capture is read-only"]}},"items":[{"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_ref":"dec-...","decision_carrier_hint":".haft/decisions/dec-....md","candidate_posture":"precise_target_prefilled_subject_needed|needs_subject_and_target_review|whole_file_fallback_target_repair_needed","confidence":"medium|low|not_applicable","decision_subject_ref_suggestions":["subject:bounded-context:title"],"review_commands":["sed -n '1,220p' .haft/decisions/dec-....md","haft decision reconcile selection-draft --decision-ref dec-... --json"],"suggested_review_action":"review decision carrier and fill exact decision_subject_ref","blocking_questions":["What exact object does this decision govern now?"],"approval_readiness":{"state":"operator_review_required","apply_ready":false,"not_apply_ready_reasons":["selection document still contains missing or placeholder fields"],"placeholder_fields":["operator_approval_ref","items[].decision_subject_ref","items[].reason"],"required_operator_checks":["confirm the exact decision_subject_ref from the decision carrier and current governing scope"],"selection_fields_to_confirm":["operator_approval_ref","items[].decision_subject_ref","items[].governance_targets","items[].remove_whole_file_fallback_targets","items[].reason"],"authority_boundary":["approval_readiness does not create operator approval"]},"selection_template":"{...}","proposed_selection":{"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-..."],"decision_subject_ref":"TODO_exact_decision_subject_ref","governance_targets":[{"kind":"symbol","ref":"symbol:..."}],"remove_whole_file_fallback_targets":["whole_file_fallback:.haft/solutions/sol-old.md"],"reason":"TODO_operator_reviewed_scope_enrichment_reason"}}],"selection_document_template":{"schema_version":1,"authority":"operator_approved_reconciliation_selection","operator_approval_ref":"","items":[{"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-..."],"decision_subject_ref":"TODO_exact_decision_subject_ref","governance_targets":[{"kind":"symbol","ref":"symbol:..."}],"remove_whole_file_fallback_targets":["whole_file_fallback:.haft/solutions/sol-old.md"],"reason":"TODO_operator_reviewed_scope_enrichment_reason"}]}}`,
						Note:  "Selection drafts are read-only review aids. Default output is bounded for review; use --limit N for a smaller compact slice or --full for the complete audit list. plan_scope_enrichment_candidates names the broader reconciliation-plan count; reviewable_scope_enrichment_candidates and the backwards-compatible scope_enrichment_candidates field name the draftable enrich_scope candidates. current_metrics is read-only before/after context copied from the reconciliation metrics surface; it is not copied into selection_document_template and does not create approval, evidence truth, gate passage, or apply authority. decision_subject_ref_suggestions, decision_carrier_hint, and review_commands are deterministic review hints only, discovery aids, not approval or apply authority, and are not copied into proposed_selection or apply templates. remove_whole_file_fallback_targets is a review hint copied only when it names existing top-level whole-file fallback binding_targets; the operator still must confirm it in the selection document. approval_readiness names missing placeholders, required operator checks, and fields to confirm; it is advisory review metadata, not approval. --write-review-packet review.json writes the bounded report-only draft with review hints for operator inspection; it is not an approval document. --write-template selection.json writes only the bounded selection_document_template to a file but does not create approval and leaves operator_approval_ref empty. proposed_selection is a structured copy of the per-item template, and together with selection_document_template removes JSON assembly work, but both remain not apply-ready until operator_approval_ref is filled; TODO_... placeholders are rejected and must be replaced before the selection can become apply-ready. apply still validates authority/current plan.",
					},
				},
				Notes: []string{
					"Use this after DriftEvents show high fanout or old decisions need authority-frontier cleanup.",
					"`haft decision reconcile metrics --json` captures fallback scope, DriftEvent fanout, and current-authority conflict counts before and after an operator-approved apply.",
					"`haft decision reconcile selection-draft --json` is compact by default and reports omitted candidates; `--full` restores the complete review list.",
					"`plan_scope_enrichment_candidates` can be larger than `reviewable_scope_enrichment_candidates` when the reconciliation plan has non-enrich_scope review work.",
					"`current_metrics` on selection drafts is read-only before/after context and is not copied into the operator-approved selection template.",
					"`haft decision reconcile selection-draft --json` emits report-only candidate posture, confidence, suggested_review_action, and blocking_questions so sparse candidates can be skipped instead of guessed.",
					"`decision_subject_ref_suggestions` are review hints only; they are not copied into selection_document_template authority fields.",
					"`decision_carrier_hint` and `review_commands` point back to source carriers for review; they are discovery aids, not approval or apply authority.",
					"`proposed_selection` is a structured copy of the per-item template for review ergonomics; it is not approval and still contains TODO placeholders until the operator fills it.",
					"`approval_readiness` is advisory review metadata; it names blockers and operator checks but does not change selection-review/apply authority.",
					"`remove_whole_file_fallback_targets` is emitted only for named existing whole-file fallback binding_targets and must be explicitly confirmed in an operator-approved selection.",
					"`haft decision reconcile selection-draft --write-review-packet review.json` writes the bounded report-only draft with hints for review; it is not an approval document.",
					"`haft decision reconcile selection-draft --write-template selection.json` writes the bounded selection document template for review but does not create approval.",
					"`selection_document_template` is a convenience packet, not approval; operator_approval_ref and TODO_... fields must be replaced before the selection can become apply-ready.",
					"Report-only: it does not supersede, merge, retire, reopen, baseline, or mutate evidence.",
					"scope_enrichment_candidates are repair prompts, not automatic mutations.",
					"lineage_relations are preview labels, not authority mutations; apply still requires an operator-approved selection document.",
					"preview shows current/proposed state and required approval fields; it is not an apply operation.",
					"downstream_impact shows links/backlinks that must be reviewed before apply; it does not relink dependencies.",
					"downstream_migration_report names dependent refs and review policy before apply; it never relinks automatically.",
					"consolidated_successor_workflow names the successor packet review contract; it never creates or approves a successor.",
					"claim_lifecycle_update is an operator-approved apply operation for explicit claims; it keeps the parent DecisionRecord current.",
					"Apply/lineage mutation is available only through an explicit operator-approved selection document.",
				},
			},
			OutputVolume: []string{"default: compact JSON DecisionReconciliationPlan; full=true: complete audit payload with groups[].preview"},
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
				MCPCall:          `haft_query(action="governing_set", query="symbol:...", limit=5)`,
				CLIStatus:        "available",
				CLICommand:       "haft decision governing-set --query symbol:... --json --limit 5; haft decision governing-set --query symbol:... --json; haft decision governing-set --write-snapshot .context/governing-set.json --json; haft decision governing-set --check-snapshot .context/governing-set.json --json",
				DiscoveryCommand: "haft interface query.governing_set --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"query", "bearer_ref", "source_refs", "limit", "full"},
				FieldShapes: []fieldShape{
					{
						Field: "filters",
						Shape: `{"query":"substring across set id, subject, target, decision refs, history refs, fallback targets, repair hints","bearer_ref":"exact subject_ref","source_refs":["exact target_ref"]}`,
						Note:  "CLI equivalents are --query, --subject-ref, and --target-ref. Filters are read-only drill-down constraints; they do not change the governing-set model.",
					},
					{
						Field: "response",
						Shape: `{"schema_version":1,"authority":"read_only_current_authority_frontier","view":"compact","snapshot":{"generated_at":"RFC3339","source":"artifact_store_decision_records","projection":"refreshable_current_governing_frontier","authority_boundary":"derived_read_only_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication","current_status_policy":["active","refresh_due"],"terminal_status_policy":["superseded","deprecated"],"terminal_history_policy":"terminal decisions stay searchable history and are excluded from current authority","filter_applied":true},"filter":{"query":"symbol:...","subject_ref":"subject:...","target_ref":"symbol:..."},"summary":{"current_decisions":7,"governing_sets":5,"conflict_sets":1,"overlap_review_sets":1,"fallback_target_sets":1,"scope_enrichment_sets":2,"terminal_history_refs":3},"authority_frontier":{"authority_boundary":"current_decision_refs_are_governing_authority_terminal_history_refs_are_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication","current_status_policy":["active","refresh_due"],"terminal_status_policy":["superseded","deprecated"],"current_decision_refs":["dec-current"],"terminal_history_refs":["dec-old"],"terminal_history_policy":"terminal decisions stay searchable history and are excluded from current authority"},"compact_sets":[{"set_id":"governing-set-...","subject_ref":"...","bounded_context":"...","target_ref":"...","answer_paths":[{"target_kind":"claim|spec_section|api_contract|invariant|symbol|whole_file_fallback|file_fallback|unscoped_decision","target_ref":"...","cli":"haft decision governing-set --target-ref ... --json","mcp_call":"haft_query(action=\"governing_set\", source_refs=[...])","exact_record_needed":"...","authority_boundary":"answer_path_is_read_only_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication"}],"target_resolution":"explicit_governance_or_watch_target|whole_file_fallback_requires_scope_enrichment","whole_file_fallback_targets":[...],"scope_repair_hints":["replace whole-file fallback ..."],"posture":"single_current_authority|overlap_needs_review|conflict_requires_operator","current_decision_refs":[...],"terminal_history_refs":[...],"current_decision_count":7,"operator_required":true}],"omitted_sets":42,"full_audit_command":"haft_query(action=\"governing_set\", full=true)"}`,
						Note:  "Default MCP response is compact. limit caps compact sets; status/prompt-governor recommendations use limit=5. CLI `--json --limit N` returns the same compact projection; CLI `--json` without limit remains full audit JSON for backward compatibility. full=true restores full sets[].current_decisions and basis and ignores the compact limit. Terminal decisions are history refs, not current authority; conflicts, overlaps, and fallback scope repair hints are review cues, not automatic lineage mutations.",
					},
					{
						Field: "snapshot_check_response",
						Shape: `{"schema_version":1,"authority":"read_only_current_governing_frontier_snapshot_check","snapshot_path":".context/governing-set.json","match":true,"current_snapshot_digest":"sha256:...","recorded_snapshot_digest":"sha256:...","mutation_boundary":["snapshot check is read-only","check does not create approval, evidence truth, gate passage, or decision authority"]}`,
						Note:  "CLI --write-snapshot writes a JSON CurrentGoverningSetReport carrier; --check-snapshot compares only snapshot_digest and fails on mismatch. Snapshot carriers are comparison aids, not authority artifacts.",
					},
				},
				Notes: []string{
					"Use this after reconciliation planning to ask what currently governs a subject/context/target.",
					"Use query/source_refs/bearer_ref for focused drill-down instead of expanding default status.",
					"The projection is derived from active/refresh_due decisions and effective governance/drift targets.",
					"snapshot is provenance for the refreshable projection; --write-snapshot persists it as a comparison carrier, not an authority artifact.",
					"--check-snapshot compares the current frontier digest to a persisted carrier and requires review on mismatch; it does not reconcile automatically.",
					"answer_paths give exact CLI/MCP drill-downs for claim/spec_section/API-contract/invariant/symbol/fallback/unscoped targets; they are read-only affordances, not evidence.",
					"fallback_target_sets and scope_enrichment_sets point to old decisions that need operator-approved scope enrichment before stronger use.",
					"Read-only: it does not supersede, merge, retire, reopen, baseline, create evidence, approve claims, publish truth, or create GateDecision records.",
				},
			},
			OutputVolume: []string{"default: compact JSON CurrentGoverningSetReport; full=true: complete audit payload with sets[].current_decisions and basis"},
			Invariants: append(commonInterfaceInvariants(),
				"CurrentGoverningSet is read-only: it is not evidence, approval, GateDecision, claim truth, global truth, publication, or reconciliation apply authority.",
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
						Shape: `{"object":{"bearer_ref":"...","entity_or_subject_label":"..."},"blocked_use":"...","source_return":{"status":"source_return_declared|exact_record_needed|missing_source_return","source_refs":[...]},"next_admissible_actions":[...],"authority_boundary":{"work_plan":"not_work_plan","evidence":"not_evidence","approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}`,
						Note:  "The item points back to exact source records; action invitations are not WorkPlans, evidence, approval, GateDecision, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Use this when a compact cockpit cue is insufficient and the agent needs the exact object/source return before stronger use.",
					"Missing source refs fail closed as missing_source_return and suggest recover_exact_source_record.",
				},
			},
			OutputVolume: []string{"default: one JSON BlockedUseAttentionItem"},
			Invariants: append(commonInterfaceInvariants(),
				"Attention items are read-only review inputs, not WorkPlans, evidence, approval, GateDecision, claim truth, global truth, or publication.",
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
						Shape: `{"score_policy":{"single_score":"no_single_haft_or_fpf_score","aggregation":"characteristic_space_only"},"characteristics":[{"bearer_ref":"...","method":"...","window":"...","denominator":"...","evidence_refs":[...],"reopen_condition":"..."}],"simplify_kill_criteria":[{"trigger":"...","review_action":"...","evidence_rule":"...","authority_boundary":"read_only_review_trigger_not_automatic_gate"}],"interpretation_rules":{"healthy_reopening":"healthy_reopening_not_counted_as_simple_failure"},"authority_boundary":{"score":"not_score","evidence":"not_evidence","approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"}}`,
						Note:  "Every characteristic names bearer, method, window, denominator, and evidence refs; simplify/kill criteria are visible read-only review triggers, not automatic gates.",
					},
				},
				Notes: []string{
					"Use source_refs for evidence refs in the compact MCP schema; CLI names them --evidence-ref.",
					"Healthy reopening is interpreted separately from avoidable rework or simple failure.",
					"Simplify/kill criteria require source-backed review before changing scope; they are not evidence, approval, GateDecision, claim truth, publication, or product-value proof.",
				},
			},
			OutputVolume: []string{"default: one JSON HaftEngineeringValueECS projection"},
			Invariants: append(commonInterfaceInvariants(),
				"No single Haft or FPF value score exists.",
				"Value characteristics are review inputs, not evidence, approval, GateDecision, claim truth, global truth, or publication.",
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
				CLICommand:       "haft overseer judgment --json --limit 20",
				DiscoveryCommand: "haft interface refresh.review --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"context"},
				Notes: []string{
					"Output groups only rung-3 needs-judgment tasks by recommendation, confidence, source, and category.",
					"CLI --limit returns a bounded JSON projection with omitted counts and a full_audit_command.",
					"Suggested commands are candidates for explicit operator approval; the packet is not evidence, approval, or mutation.",
				},
			},
			OutputVolume: []string{"default: compact grouped judgment packet", "CLI --json --limit N: bounded JSON with omitted counts", "CLI --json: full task list with source return and suggested commands"},
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
				CLICommand:       "haft overseer drain --dry-run --json; haft overseer drain --dry-run --json --full",
				DiscoveryCommand: "haft interface refresh.drain --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"dry_run", "context"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":"maintenance_drain.v1","view":"compact","dry_run":true,"authority_boundary":{"trigger":"explicit_h_verify_or_overseer_drain","mutation":"not_mutation|machine_safe_only","approval":"not_semantic_approval","evidence":"not_evidence|machine_evidence_only","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"},"summary":{"executed_actions":2,"needs_operator_tasks":72,"reconciliation_proposal_count":71},"executed":[{"kind":"auto_rebaseline","outcome":"proposed"}],"omitted_executed":1,"reconciliation_proposals":[{"kind":"high_fanout_reconciliation_review|fallback_scope_repair_review|fallback_governing_scope_review","suggested_command":"haft decision reconcile --json","authority_boundary":"read_only_reconciliation_proposal_not_binding_authority_not_mutation_not_evidence_not_approval_not_gate_decision_not_claim_truth_not_global_truth_not_publication"}],"omitted_reconciliation_proposals":70,"after_action":{"remaining_operator_judgment":[...],"authority_boundary":"after_action_report_only_not_binding_authority_not_mutation_not_evidence_not_approval_not_gate_decision_not_claim_truth_not_global_truth_not_publication"},"omitted_after_action_remaining_operator_judgment":67,"needs_operator":[...],"omitted_needs_operator_tasks":67,"full_audit_command":"haft overseer drain --dry-run --json --full"}`,
						Note:  "Default CLI JSON is compact and preserves summary counts while bounding executed, reconciliation_proposals, after_action.remaining_operator_judgment, and nested needs_operator tasks. Use --limit N to adjust compact output and --full for the complete audit payload. Drain reports, reconciliation proposals, and after_action are report-only for semantic authority; they do not supersede, retire, merge, approve, create decisions, create claim truth/global truth, publish, or apply reconciliation selections.",
					},
				},
				Notes: []string{
					"dry_run=true proposes machine-safe actions without mutating; dry_run=false executes only rung-1/rung-2 safe actions.",
					"Material drift, semantic uncertainty, reopen/supersede choices, and weak waivers are returned as needs_operator.",
					"CLI JSON is compact by default; summary counts are complete, omitted_* fields name truncated audit tails, and --full restores the complete report.",
					"reconciliation_proposals are read-only review batches for high-fanout/fallback groups; suggested commands are inspect-only.",
					"after_action lists auto-closed items, evidence refs, remaining operator judgment, and undo commands for autonomous mutations.",
					"Drain reports do not create claim truth, global truth, or publication authority.",
				},
			},
			OutputVolume: []string{"default: compact drain report", "CLI --json: compact executed/proposal/operator samples plus omitted counts", "CLI --json --full: complete audit payload"},
			Invariants: append(commonInterfaceInvariants(),
				"Drain is opt-in; default status and refresh.review remain read-only.",
				"Drain does not create semantic approval, GateDecision, claim truth, global truth, or publication.",
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
	Kind              string                                  `json:"kind"`
	SchemaVersion     int                                     `json:"schema_version"`
	Authority         string                                  `json:"authority"`
	AuthorityBoundary interfaceContractAuditAuthorityBoundary `json:"authority_boundary"`
	Summary           interfaceContractAuditSummary           `json:"summary"`
	Surfaces          []interfaceContractAuditSurface         `json:"surfaces"`
	Notes             []string                                `json:"notes"`
}

type interfaceContractAuditAuthorityBoundary struct {
	Inventory           string `json:"inventory"`
	SchemaGeneration    string `json:"schema_generation"`
	HostMaterialization string `json:"host_materialization"`
	Evidence            string `json:"evidence"`
	Approval            string `json:"approval"`
	GateDecision        string `json:"gate_decision"`
	ClaimTruth          string `json:"claim_truth"`
	GlobalTruth         string `json:"global_truth"`
	Publication         string `json:"publication"`
}

type interfaceContractAuditSummary struct {
	Capabilities                int `json:"capabilities"`
	KernelOwnedContracts        int `json:"kernel_owned_contracts"`
	MCPMirroredActions          int `json:"mcp_mirrored_actions"`
	CLIAvailableSurfaces        int `json:"cli_available_surfaces"`
	BindingAuthoritySurfaces    int `json:"binding_authority_surfaces"`
	ReadOnlySurfaces            int `json:"read_only_surfaces"`
	LegacyTransportExceptions   int `json:"legacy_transport_exceptions"`
	SchemaCoveredSurfaces       int `json:"schema_covered_surfaces"`
	SchemaMissingSurfaces       int `json:"schema_missing_surfaces"`
	SchemaExcludedFields        int `json:"schema_excluded_fields"`
	SchemaRequiredCovered       int `json:"schema_required_covered_surfaces"`
	SchemaRequiredMissing       int `json:"schema_required_missing_surfaces"`
	SchemaMissingRequiredFields int `json:"schema_missing_required_fields"`
	ShapeCoveredSurfaces        int `json:"shape_covered_surfaces"`
	ShapeMissingSurfaces        int `json:"shape_missing_surfaces"`
	ShapeSkippedFields          int `json:"shape_skipped_fields"`
	ShapeGeneratorTargets       int `json:"shape_generator_targets"`
	ShapeGeneratorTargetFields  int `json:"shape_generator_target_fields"`
	ValidatedMCPMirrors         int `json:"validated_mcp_mirrors"`
	ManualCLIContracts          int `json:"manual_cli_contracts"`
	UnvalidatedHostFragments    int `json:"unvalidated_host_fragments"`
	GeneratedTargetFragments    int `json:"generated_target_fragments"`
	ValidatedFragments          int `json:"validated_fragments"`
	LegacyFragments             int `json:"legacy_fragments"`
	UnvalidatedFragments        int `json:"unvalidated_fragments"`
}

type interfaceContractAuditSurface struct {
	CapabilityID            string                               `json:"capability_id"`
	MCPTool                 string                               `json:"mcp_tool"`
	MCPAction               string                               `json:"mcp_action,omitempty"`
	CLIStatus               string                               `json:"cli_status"`
	CLICommand              string                               `json:"cli_command,omitempty"`
	ContractSources         []string                             `json:"contract_sources"`
	ContractSourcePosture   string                               `json:"contract_source_posture"`
	ContractFragmentPosture string                               `json:"contract_fragment_posture"`
	HostSchemaPosture       string                               `json:"host_schema_posture"`
	SchemaPosture           string                               `json:"schema_posture"`
	AuthorityPosture        string                               `json:"authority_posture"`
	ValidationRefs          []string                             `json:"validation_refs"`
	LegacyException         bool                                 `json:"legacy_exception"`
	Notes                   []string                             `json:"notes,omitempty"`
	SchemaCoverage          interfaceContractAuditSchemaCoverage `json:"schema_coverage"`
	ShapeCoverage           interfaceContractAuditShapeCoverage  `json:"shape_coverage"`
}

type interfaceContractAuditSchemaCoverage struct {
	Checked               bool     `json:"checked"`
	Status                string   `json:"status"`
	MissingFields         []string `json:"missing_fields,omitempty"`
	ExcludedFields        []string `json:"excluded_fields,omitempty"`
	MCPRequiredFields     []string `json:"mcp_required_fields,omitempty"`
	MissingRequiredFields []string `json:"missing_required_fields,omitempty"`
	ActionRequiredFields  []string `json:"action_required_fields,omitempty"`
	RequiredPosture       string   `json:"required_posture,omitempty"`
}

type interfaceContractAuditShapeCoverage struct {
	Checked               bool     `json:"checked"`
	Status                string   `json:"status"`
	MissingShapeFields    []string `json:"missing_shape_fields,omitempty"`
	GeneratorTargetFields []string `json:"generator_target_fields,omitempty"`
	SkippedFields         []string `json:"skipped_fields,omitempty"`
}

type interfaceContractGenerationReport struct {
	Kind            string                                     `json:"kind"`
	SchemaVersion   int                                        `json:"schema_version"`
	Authority       string                                     `json:"authority"`
	Source          string                                     `json:"source"`
	SourceDigest    string                                     `json:"source_digest"`
	ValidationRefs  []string                                   `json:"validation_refs"`
	Summary         interfaceContractGenerationSummary         `json:"summary"`
	SurfacePolicy   interfaceContractGenerationPolicy          `json:"surface_policy"`
	RuntimeAudit    interfaceContractRuntimeSchemaAudit        `json:"runtime_schema_audit"`
	Targets         []interfaceContractGenerationTarget        `json:"targets"`
	Carriers        []interfaceContractMaterializedCarrier     `json:"materialized_carriers"`
	Fragments       []interfaceContractGeneratedFragment       `json:"generated_fragments"`
	SchemaFragments []interfaceContractGeneratedSchemaFragment `json:"generated_schema_fragments"`
	Notes           []string                                   `json:"notes"`
}

type interfaceContractSchemaFragmentsCarrier struct {
	Kind            string                                     `json:"kind"`
	SchemaVersion   int                                        `json:"schema_version"`
	Authority       string                                     `json:"authority"`
	Source          string                                     `json:"source"`
	SourceDigest    string                                     `json:"source_digest"`
	CarrierRole     string                                     `json:"carrier_role"`
	MaterializedBy  string                                     `json:"materialized_by"`
	Summary         interfaceContractSchemaCarrierSummary      `json:"summary"`
	ValidationRefs  []string                                   `json:"validation_refs"`
	SchemaFragments []interfaceContractGeneratedSchemaFragment `json:"generated_schema_fragments"`
	Notes           []string                                   `json:"notes"`
}

type interfaceContractSchemaCarrierSummary struct {
	GeneratedSchemaFragments int `json:"generated_schema_fragments"`
	Capabilities             int `json:"capabilities"`
	BindingPreviewFragments  int `json:"binding_preview_fragments"`
}

type interfaceContractSchemaMaterializationResult struct {
	Kind            string `json:"kind"`
	SchemaVersion   int    `json:"schema_version"`
	Authority       string `json:"authority"`
	Path            string `json:"path"`
	Source          string `json:"source"`
	SourceDigest    string `json:"source_digest"`
	CarrierDigest   string `json:"carrier_digest"`
	SchemaFragments int    `json:"schema_fragments"`
}

type interfaceContractDescriptionFragmentsCarrier struct {
	Kind                 string                               `json:"kind"`
	SchemaVersion        int                                  `json:"schema_version"`
	Authority            string                               `json:"authority"`
	Source               string                               `json:"source"`
	SourceDigest         string                               `json:"source_digest"`
	CarrierRole          string                               `json:"carrier_role"`
	MaterializedBy       string                               `json:"materialized_by"`
	Summary              interfaceContractDescriptionSummary  `json:"summary"`
	ValidationRefs       []string                             `json:"validation_refs"`
	DescriptionFragments []interfaceContractGeneratedFragment `json:"generated_description_fragments"`
	Notes                []string                             `json:"notes"`
}

type interfaceContractDescriptionSummary struct {
	GeneratedDescriptionFragments int `json:"generated_description_fragments"`
	Capabilities                  int `json:"capabilities"`
	BindingPreviewFragments       int `json:"binding_preview_fragments"`
}

type interfaceContractDescriptionMaterializationResult struct {
	Kind                 string `json:"kind"`
	SchemaVersion        int    `json:"schema_version"`
	Authority            string `json:"authority"`
	Path                 string `json:"path"`
	Source               string `json:"source"`
	SourceDigest         string `json:"source_digest"`
	CarrierDigest        string `json:"carrier_digest"`
	DescriptionFragments int    `json:"description_fragments"`
}

type interfaceContractCarrierCheckResult struct {
	Kind                  string `json:"kind"`
	SchemaVersion         int    `json:"schema_version"`
	Authority             string `json:"authority"`
	Path                  string `json:"path"`
	Source                string `json:"source"`
	SourceDigest          string `json:"source_digest"`
	ExpectedCarrierDigest string `json:"expected_carrier_digest"`
	ActualCarrierDigest   string `json:"actual_carrier_digest"`
	Match                 bool   `json:"match"`
}

type interfaceContractMaterializedCarrierCheckReport struct {
	Kind          string                                           `json:"kind"`
	SchemaVersion int                                              `json:"schema_version"`
	Authority     string                                           `json:"authority"`
	Source        string                                           `json:"source"`
	SourceDigest  string                                           `json:"source_digest"`
	Summary       interfaceContractMaterializedCarrierCheckSummary `json:"summary"`
	Carriers      []interfaceContractMaterializedCarrierCheckItem  `json:"carriers,omitempty"`
}

type interfaceContractMaterializedCarrierCheckSummary struct {
	MaterializedCarriers int  `json:"materialized_carriers"`
	CheckedCarriers      int  `json:"checked_carriers"`
	MissingCarrierFiles  int  `json:"missing_carrier_files"`
	MissingMarkers       int  `json:"missing_markers"`
	Match                bool `json:"match"`
}

type interfaceContractMaterializedCarrierCheckItem struct {
	CarrierPath    string   `json:"carrier_path"`
	CarrierKind    string   `json:"carrier_kind"`
	ContractRole   string   `json:"contract_role"`
	SyncPosture    string   `json:"sync_posture"`
	GuardPosture   string   `json:"guard_posture"`
	Match          bool     `json:"match"`
	MissingFile    bool     `json:"missing_file,omitempty"`
	MissingMarkers []string `json:"missing_markers,omitempty"`
	ValidationRefs []string `json:"validation_refs,omitempty"`
}

type interfaceContractMaterializedCarrierSyncReport struct {
	Kind          string                                           `json:"kind"`
	SchemaVersion int                                              `json:"schema_version"`
	Authority     string                                           `json:"authority"`
	Source        string                                           `json:"source"`
	SourceDigest  string                                           `json:"source_digest"`
	Summary       interfaceContractMaterializedCarrierSyncSummary  `json:"summary"`
	Carriers      []interfaceContractMaterializedCarrierSyncItem   `json:"carriers,omitempty"`
	Check         interfaceContractMaterializedCarrierCheckSummary `json:"post_sync_check"`
}

type interfaceContractMaterializedCarrierSyncSummary struct {
	MaterializedCarriers int  `json:"materialized_carriers"`
	SyncedCarriers       int  `json:"synced_carriers"`
	UnchangedCarriers    int  `json:"unchanged_carriers"`
	MissingCarrierFiles  int  `json:"missing_carrier_files"`
	UpdatedMarkers       int  `json:"updated_markers"`
	Match                bool `json:"match"`
}

type interfaceContractMaterializedCarrierSyncItem struct {
	CarrierPath       string   `json:"carrier_path"`
	ResolvedPath      string   `json:"resolved_path,omitempty"`
	CarrierKind       string   `json:"carrier_kind"`
	ContractRole      string   `json:"contract_role"`
	SyncPosture       string   `json:"sync_posture"`
	GuardPosture      string   `json:"guard_posture"`
	Synced            bool     `json:"synced"`
	MissingFile       bool     `json:"missing_file,omitempty"`
	UpdatedMarkers    int      `json:"updated_markers,omitempty"`
	ValidationRefs    []string `json:"validation_refs,omitempty"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

type interfaceContractGenerationSummary struct {
	Capabilities              int `json:"capabilities"`
	GeneratorTargetSurfaces   int `json:"generator_target_surfaces"`
	GeneratorTargetFields     int `json:"generator_target_fields"`
	GeneratedPreviewFragments int `json:"generated_preview_fragments"`
	GeneratedSchemaFragments  int `json:"generated_schema_fragments"`
	RuntimeSchemaMirrors      int `json:"runtime_schema_mirrors"`
	RuntimeSchemaDrift        int `json:"runtime_schema_drift"`
	BindingPreviewFragments   int `json:"binding_preview_fragments"`
	MaterializedCarriers      int `json:"materialized_carriers"`
	DigestGuardedCarriers     int `json:"digest_guarded_carriers"`
	AuthorityGuardedCarriers  int `json:"authority_boundary_guarded_carriers"`
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

type interfaceContractMaterializedCarrier struct {
	CarrierPath           string   `json:"carrier_path"`
	CarrierKind           string   `json:"carrier_kind"`
	ContractRole          string   `json:"contract_role"`
	SourceContract        string   `json:"source_contract"`
	ExpectedSourceDigest  string   `json:"expected_source_digest,omitempty"`
	SyncPosture           string   `json:"sync_posture"`
	GuardPosture          string   `json:"guard_posture"`
	AuthorityBoundary     string   `json:"authority_boundary"`
	RequiredMarkers       []string `json:"required_markers"`
	GeneratedFragmentRefs []string `json:"generated_fragment_refs"`
	ValidationRefs        []string `json:"validation_refs"`
}

type interfaceContractGeneratedFragment struct {
	CapabilityID      string   `json:"capability_id"`
	FragmentKind      string   `json:"fragment_kind"`
	SourceContract    string   `json:"source_contract"`
	SourceDigest      string   `json:"source_digest"`
	HostSchemaPosture string   `json:"host_schema_posture"`
	AuthorityPosture  string   `json:"authority_posture"`
	AuthorityBoundary string   `json:"authority_boundary"`
	MCPTool           string   `json:"mcp_tool,omitempty"`
	MCPAction         string   `json:"mcp_action,omitempty"`
	CLIStatus         string   `json:"cli_status,omitempty"`
	CLICommand        string   `json:"cli_command,omitempty"`
	GeneratedText     string   `json:"generated_text"`
	InputFields       []string `json:"input_fields,omitempty"`
	ValidationRefs    []string `json:"validation_refs"`
}

type interfaceContractGeneratedSchemaFragment struct {
	CapabilityID           string         `json:"capability_id"`
	FragmentKind           string         `json:"fragment_kind"`
	SourceContract         string         `json:"source_contract"`
	SourceDigest           string         `json:"source_digest"`
	AuthorityBoundary      string         `json:"authority_boundary"`
	MCPTool                string         `json:"mcp_tool"`
	MCPAction              string         `json:"mcp_action"`
	HostSchemaPosture      string         `json:"host_schema_posture"`
	RequiredFields         []string       `json:"required_fields"`
	AllowedTopLevelFields  []string       `json:"allowed_top_level_fields"`
	ActionRequiredFields   []string       `json:"action_required_fields"`
	HandlerValidatedFields []string       `json:"handler_validated_fields,omitempty"`
	Schema                 map[string]any `json:"schema"`
	SchemaDigest           string         `json:"schema_digest"`
	ValidationRefs         []string       `json:"validation_refs"`
}

type interfaceContractRuntimeSchemaAudit struct {
	Authority                   string   `json:"authority"`
	Source                      string   `json:"source"`
	Status                      string   `json:"status"`
	RuntimeTools                int      `json:"runtime_tools"`
	GeneratedSchemaFragments    int      `json:"generated_schema_fragments"`
	RuntimeSchemaMirrors        int      `json:"runtime_schema_mirrors"`
	RuntimeSchemaDrift          int      `json:"runtime_schema_drift"`
	MissingRuntimeTools         []string `json:"missing_runtime_tools,omitempty"`
	MissingRuntimeActionEnums   []string `json:"missing_runtime_action_enums,omitempty"`
	MissingRuntimeProperties    []string `json:"missing_runtime_properties,omitempty"`
	MissingRuntimeRequired      []string `json:"missing_runtime_required,omitempty"`
	SchemaDigestMismatches      []string `json:"schema_digest_mismatches,omitempty"`
	AdditionalPropertiesPosture string   `json:"additional_properties_posture"`
	ValidationRefs              []string `json:"validation_refs"`
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
	fragments := make([]interfaceContractGeneratedFragment, 0, len(audit.Surfaces))
	schemaFragments := make([]interfaceContractGeneratedSchemaFragment, 0, audit.Summary.MCPMirroredActions)
	toolSchemas := interfaceContractAuditToolSchemas()
	capabilitiesByID := make(map[string]interfaceCapability, len(catalog))
	for _, capability := range catalog {
		capabilitiesByID[capability.ID] = capability
	}

	for _, surface := range audit.Surfaces {
		capability := capabilitiesByID[surface.CapabilityID]
		fragments = append(fragments, interfaceContractGeneratedFragmentFor(surface, capability))
		if interfaceContractShouldGenerateSchemaFragment(surface) {
			schemaFragments = append(schemaFragments, interfaceContractGeneratedSchemaFragmentFor(surface, capability, toolSchemas[surface.MCPTool]))
		}

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
	sort.Slice(fragments, func(i, j int) bool {
		return fragments[i].CapabilityID < fragments[j].CapabilityID
	})
	sort.Slice(schemaFragments, func(i, j int) bool {
		return schemaFragments[i].CapabilityID < schemaFragments[j].CapabilityID
	})

	sourceDigest := interfaceContractGenerationDigest(catalog)
	carriers := interfaceContractMaterializedCarriers(sourceDigest)
	summary := interfaceContractGenerationSummary{
		Capabilities:              audit.Summary.Capabilities,
		GeneratorTargetSurfaces:   len(targets),
		GeneratorTargetFields:     countInterfaceContractGenerationFields(targets),
		GeneratedPreviewFragments: len(fragments),
		GeneratedSchemaFragments:  len(schemaFragments),
		BindingPreviewFragments:   countInterfaceContractBindingFragments(fragments),
		MaterializedCarriers:      len(carriers),
		DigestGuardedCarriers:     countInterfaceContractDigestGuardedCarriers(carriers),
		AuthorityGuardedCarriers:  countInterfaceContractAuthorityGuardedCarriers(carriers),
	}

	report := interfaceContractGenerationReport{
		Kind:            "haft_interface_contract_generation_manifest",
		SchemaVersion:   1,
		Authority:       "read_only_generation_manifest_not_host_materialization",
		Source:          "kernel_interface_catalog",
		SourceDigest:    sourceDigest,
		ValidationRefs:  interfaceContractGenerationValidationRefs(),
		Summary:         summary,
		SurfacePolicy:   interfaceContractGenerationSurfacePolicy(),
		Targets:         targets,
		Carriers:        carriers,
		Fragments:       fragments,
		SchemaFragments: schemaFragments,
		Notes: []string{
			"generated_fragments are derived from the kernel interface catalog; they are preview carriers for host/skill/plugin/Pi synchronization, not host materialization.",
			"generated_schema_fragments are read-only per-action MCP schema fragments for validation and future host materialization; they do not mutate tools/list.",
			"runtime_schema_audit validates generated_schema_fragments against the live MCP ToolCatalog action enum, required fields, properties, and schema digests.",
			"targets list schema fields that still need generator work; an empty target queue means current MCP mirror coverage is complete, not that generated fragments are absent.",
			"materialized_carriers names repo carriers that currently mirror generated contract fragments; sync_posture distinguishes source-digest guards from weaker authority-boundary-only guards.",
			"use `haft interface contract-generation --write-schema-fragments <path>` to materialize generated_schema_fragments as a deterministic JSON carrier for host/schema sync checks.",
			"use `haft interface contract-generation --write-description-fragments <path>` to materialize generated_fragments as a deterministic JSON carrier for host/skill/plugin/Pi wording sync checks.",
			"use `--check-schema-fragments <path>` or `--check-description-fragments <path>` to compare materialized carriers against the current source digest without rewriting them.",
			"use `--check-materialized-carriers` to validate every listed host/skill/plugin/Pi carrier for current source-digest and authority-boundary markers without rewriting files.",
			"use `--sync-materialized-carriers` to rewrite only source-digest markers in listed repo carriers, then verify with the same marker check; it is not host runtime materialization.",
			"validation_refs name tests that prove the manifest source, MCP mirror coverage, generated-fragment authority boundary, and default-output budget.",
			"Schema visibility is not operator authorization, binding authority, evidence, or gate passage.",
			"Default status must not inline this report; use haft interface contract-generation --json or haft_query(action=\"contract_generation\").",
		},
	}
	report.RuntimeAudit = interfaceContractRuntimeSchemaAuditFor(report)
	report.Summary.RuntimeSchemaMirrors = report.RuntimeAudit.RuntimeSchemaMirrors
	report.Summary.RuntimeSchemaDrift = report.RuntimeAudit.RuntimeSchemaDrift

	return report
}

func interfaceContractShouldGenerateSchemaFragment(surface interfaceContractAuditSurface) bool {
	if surface.MCPTool == "" || surface.MCPAction == "" {
		return false
	}
	if surface.SchemaCoverage.Status != "covered" {
		return false
	}
	if interfaceContractAllActionFieldsExcluded(surface.SchemaCoverage) {
		return false
	}
	if surface.HostSchemaPosture != "validated_mcp_mirror" {
		return false
	}
	return true
}

func interfaceContractSchemaFragmentsCarrierFor(report interfaceContractGenerationReport) interfaceContractSchemaFragmentsCarrier {
	return interfaceContractSchemaFragmentsCarrier{
		Kind:           "haft_interface_generated_mcp_schema_fragments",
		SchemaVersion:  1,
		Authority:      "generated_validation_carrier_not_runtime_schema_authority",
		Source:         report.Source,
		SourceDigest:   report.SourceDigest,
		CarrierRole:    "materialized_generated_contract_schema_fragments",
		MaterializedBy: "haft interface contract-generation --write-schema-fragments",
		Summary: interfaceContractSchemaCarrierSummary{
			GeneratedSchemaFragments: len(report.SchemaFragments),
			Capabilities:             report.Summary.Capabilities,
			BindingPreviewFragments:  report.Summary.BindingPreviewFragments,
		},
		ValidationRefs:  report.ValidationRefs,
		SchemaFragments: report.SchemaFragments,
		Notes: []string{
			"This generated carrier is deterministic validation material for host/schema synchronization.",
			"It is not runtime MCP schema authority, operator authorization, evidence truth, or gate passage.",
			"Regenerate with `haft interface contract-generation --write-schema-fragments <path>` after kernel interface catalog changes.",
		},
	}
}

func materializeInterfaceContractSchemaFragments(
	report interfaceContractGenerationReport,
	path string,
) (interfaceContractSchemaMaterializationResult, error) {
	carrier := interfaceContractSchemaFragmentsCarrierFor(report)
	data, err := marshalInterfaceContractCarrier(carrier)
	if err != nil {
		return interfaceContractSchemaMaterializationResult{}, fmt.Errorf("marshal generated schema fragments carrier: %w", err)
	}

	cleanPath := filepath.Clean(path)
	if dir := filepath.Dir(cleanPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return interfaceContractSchemaMaterializationResult{}, fmt.Errorf("create generated schema carrier directory: %w", err)
		}
	}
	if err := os.WriteFile(cleanPath, data, 0o644); err != nil {
		return interfaceContractSchemaMaterializationResult{}, fmt.Errorf("write generated schema carrier: %w", err)
	}

	return interfaceContractSchemaMaterializationResult{
		Kind:            "haft_interface_schema_fragments_materialization_result",
		SchemaVersion:   1,
		Authority:       "write_report_not_runtime_schema_authority",
		Path:            cleanPath,
		Source:          carrier.Source,
		SourceDigest:    carrier.SourceDigest,
		CarrierDigest:   interfaceContractGenerationDigestBytes(data),
		SchemaFragments: len(carrier.SchemaFragments),
	}, nil
}

func checkInterfaceContractSchemaFragments(
	report interfaceContractGenerationReport,
	path string,
) (interfaceContractCarrierCheckResult, error) {
	carrier := interfaceContractSchemaFragmentsCarrierFor(report)
	data, err := marshalInterfaceContractCarrier(carrier)
	if err != nil {
		return interfaceContractCarrierCheckResult{}, fmt.Errorf("marshal generated schema fragments carrier: %w", err)
	}
	return checkInterfaceContractCarrier(path, carrier.Source, carrier.SourceDigest, data)
}

func interfaceContractDescriptionFragmentsCarrierFor(report interfaceContractGenerationReport) interfaceContractDescriptionFragmentsCarrier {
	return interfaceContractDescriptionFragmentsCarrier{
		Kind:           "haft_interface_generated_description_fragments",
		SchemaVersion:  1,
		Authority:      "generated_text_carrier_not_operator_authorization",
		Source:         report.Source,
		SourceDigest:   report.SourceDigest,
		CarrierRole:    "materialized_generated_contract_description_fragments",
		MaterializedBy: "haft interface contract-generation --write-description-fragments",
		Summary: interfaceContractDescriptionSummary{
			GeneratedDescriptionFragments: len(report.Fragments),
			Capabilities:                  report.Summary.Capabilities,
			BindingPreviewFragments:       report.Summary.BindingPreviewFragments,
		},
		ValidationRefs:       report.ValidationRefs,
		DescriptionFragments: report.Fragments,
		Notes: []string{
			"This generated carrier is deterministic validation material for host/skill/plugin/Pi wording synchronization.",
			"It is not operator authorization, evidence truth, gate passage, runtime schema authority, or global truth.",
			"Regenerate with `haft interface contract-generation --write-description-fragments <path>` after kernel interface catalog changes.",
		},
	}
}

func materializeInterfaceContractDescriptionFragments(
	report interfaceContractGenerationReport,
	path string,
) (interfaceContractDescriptionMaterializationResult, error) {
	carrier := interfaceContractDescriptionFragmentsCarrierFor(report)
	data, err := marshalInterfaceContractCarrier(carrier)
	if err != nil {
		return interfaceContractDescriptionMaterializationResult{}, fmt.Errorf("marshal generated description fragments carrier: %w", err)
	}

	cleanPath := filepath.Clean(path)
	if dir := filepath.Dir(cleanPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return interfaceContractDescriptionMaterializationResult{}, fmt.Errorf("create generated description carrier directory: %w", err)
		}
	}
	if err := os.WriteFile(cleanPath, data, 0o644); err != nil {
		return interfaceContractDescriptionMaterializationResult{}, fmt.Errorf("write generated description carrier: %w", err)
	}

	return interfaceContractDescriptionMaterializationResult{
		Kind:                 "haft_interface_description_fragments_materialization_result",
		SchemaVersion:        1,
		Authority:            "write_report_not_text_authority",
		Path:                 cleanPath,
		Source:               carrier.Source,
		SourceDigest:         carrier.SourceDigest,
		CarrierDigest:        interfaceContractGenerationDigestBytes(data),
		DescriptionFragments: len(carrier.DescriptionFragments),
	}, nil
}

func checkInterfaceContractDescriptionFragments(
	report interfaceContractGenerationReport,
	path string,
) (interfaceContractCarrierCheckResult, error) {
	carrier := interfaceContractDescriptionFragmentsCarrierFor(report)
	data, err := marshalInterfaceContractCarrier(carrier)
	if err != nil {
		return interfaceContractCarrierCheckResult{}, fmt.Errorf("marshal generated description fragments carrier: %w", err)
	}
	return checkInterfaceContractCarrier(path, carrier.Source, carrier.SourceDigest, data)
}

func checkInterfaceContractMaterializedCarriers(
	report interfaceContractGenerationReport,
) (interfaceContractMaterializedCarrierCheckReport, error) {
	result := interfaceContractMaterializedCarrierCheckReport{
		Kind:          "haft_interface_materialized_carrier_check",
		SchemaVersion: 1,
		Authority:     "check_report_not_materialization_or_binding_authority",
		Source:        report.Source,
		SourceDigest:  report.SourceDigest,
		Summary: interfaceContractMaterializedCarrierCheckSummary{
			MaterializedCarriers: len(report.Carriers),
			Match:                true,
		},
		Carriers: make([]interfaceContractMaterializedCarrierCheckItem, 0, len(report.Carriers)),
	}

	for _, carrier := range report.Carriers {
		item := checkInterfaceContractMaterializedCarrier(carrier)
		result.Carriers = append(result.Carriers, item)
		result.Summary.CheckedCarriers++
		result.Summary.MissingMarkers += len(item.MissingMarkers)
		if item.MissingFile {
			result.Summary.MissingCarrierFiles++
		}
		if !item.Match {
			result.Summary.Match = false
		}
	}

	if !result.Summary.Match {
		return result, fmt.Errorf(
			"generated contract materialized carrier drift: missing_files=%d missing_markers=%d",
			result.Summary.MissingCarrierFiles,
			result.Summary.MissingMarkers,
		)
	}
	return result, nil
}

func checkInterfaceContractMaterializedCarrier(
	carrier interfaceContractMaterializedCarrier,
) interfaceContractMaterializedCarrierCheckItem {
	item := interfaceContractMaterializedCarrierCheckItem{
		CarrierPath:    carrier.CarrierPath,
		CarrierKind:    carrier.CarrierKind,
		ContractRole:   carrier.ContractRole,
		SyncPosture:    carrier.SyncPosture,
		GuardPosture:   carrier.GuardPosture,
		Match:          true,
		ValidationRefs: append([]string(nil), carrier.ValidationRefs...),
	}

	data, err := readInterfaceContractMaterializedCarrier(carrier.CarrierPath)
	if err != nil {
		item.Match = false
		item.MissingFile = true
		return item
	}

	text := string(data)
	for _, marker := range carrier.RequiredMarkers {
		if strings.Contains(text, marker) {
			continue
		}
		item.MissingMarkers = append(item.MissingMarkers, marker)
	}
	if len(item.MissingMarkers) != 0 {
		item.Match = false
	}
	return item
}

func syncInterfaceContractMaterializedCarriers(
	report interfaceContractGenerationReport,
) (interfaceContractMaterializedCarrierSyncReport, error) {
	result := interfaceContractMaterializedCarrierSyncReport{
		Kind:          "haft_interface_materialized_carrier_sync",
		SchemaVersion: 1,
		Authority:     "sync_report_not_host_runtime_materialization_binding_authority_evidence_approval_gate_decision_claim_truth_global_truth_or_publication",
		Source:        report.Source,
		SourceDigest:  report.SourceDigest,
		Summary: interfaceContractMaterializedCarrierSyncSummary{
			MaterializedCarriers: len(report.Carriers),
			Match:                false,
		},
		Carriers: make([]interfaceContractMaterializedCarrierSyncItem, 0, len(report.Carriers)),
	}

	for _, carrier := range report.Carriers {
		item := syncInterfaceContractMaterializedCarrier(carrier, report.SourceDigest)
		result.Carriers = append(result.Carriers, item)
		result.Summary.UpdatedMarkers += item.UpdatedMarkers
		if item.MissingFile {
			result.Summary.MissingCarrierFiles++
			continue
		}
		if item.Synced {
			result.Summary.SyncedCarriers++
			continue
		}
		result.Summary.UnchangedCarriers++
	}

	if result.Summary.MissingCarrierFiles != 0 {
		return result, fmt.Errorf(
			"generated contract materialized carrier sync failed: missing_files=%d",
			result.Summary.MissingCarrierFiles,
		)
	}

	check, err := checkInterfaceContractMaterializedCarriers(report)
	result.Check = check.Summary
	result.Summary.Match = check.Summary.Match
	if err != nil {
		return result, err
	}
	return result, nil
}

func syncInterfaceContractMaterializedCarrier(
	carrier interfaceContractMaterializedCarrier,
	sourceDigest string,
) interfaceContractMaterializedCarrierSyncItem {
	item := interfaceContractMaterializedCarrierSyncItem{
		CarrierPath:       carrier.CarrierPath,
		CarrierKind:       carrier.CarrierKind,
		ContractRole:      carrier.ContractRole,
		SyncPosture:       carrier.SyncPosture,
		GuardPosture:      carrier.GuardPosture,
		ValidationRefs:    append([]string(nil), carrier.ValidationRefs...),
		AuthorityBoundary: carrier.AuthorityBoundary,
	}

	resolvedPath, err := resolveInterfaceContractMaterializedCarrierPath(carrier.CarrierPath)
	if err != nil {
		item.MissingFile = true
		return item
	}
	item.ResolvedPath = resolvedPath

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		item.MissingFile = true
		return item
	}

	updated, updatedMarkers := syncInterfaceContractMaterializedCarrierData(data, sourceDigest)
	if updatedMarkers == 0 {
		return item
	}

	if err := os.WriteFile(resolvedPath, updated, 0o644); err != nil {
		item.MissingFile = true
		return item
	}
	item.Synced = true
	item.UpdatedMarkers = updatedMarkers
	return item
}

func syncInterfaceContractMaterializedCarrierData(data []byte, sourceDigest string) ([]byte, int) {
	lines := strings.SplitAfter(string(data), "\n")
	updatedMarkers := 0
	for index, line := range lines {
		if !interfaceContractMaterializedCarrierDigestLine(index, lines) {
			continue
		}
		lines[index] = interfaceContractDigestMarkerRE.ReplaceAllStringFunc(line, func(marker string) string {
			if marker == sourceDigest {
				return marker
			}
			updatedMarkers++
			return sourceDigest
		})
	}
	return []byte(strings.Join(lines, "")), updatedMarkers
}

func interfaceContractMaterializedCarrierDigestLine(index int, lines []string) bool {
	line := lines[index]
	if strings.Contains(line, "kernel_interface_catalog") {
		return true
	}
	if strings.Contains(line, "kernelInterfaceCatalogDigest") {
		return true
	}
	if index == 0 {
		return false
	}
	return strings.Contains(lines[index-1], "kernelInterfaceCatalogDigest")
}

func resolveInterfaceContractMaterializedCarrierPath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		if _, err := os.Stat(cleanPath); err != nil {
			return "", err
		}
		return cleanPath, nil
	}
	if _, err := os.Stat(cleanPath); err == nil {
		return cleanPath, nil
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", cwdErr
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, cleanPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("materialized carrier not found: %s", cleanPath)
}

func readInterfaceContractMaterializedCarrier(path string) ([]byte, error) {
	resolvedPath, err := resolveInterfaceContractMaterializedCarrierPath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(resolvedPath)
}

func marshalInterfaceContractCarrier(carrier any) ([]byte, error) {
	data, err := json.MarshalIndent(carrier, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func checkInterfaceContractCarrier(path string, source string, sourceDigest string, expected []byte) (interfaceContractCarrierCheckResult, error) {
	cleanPath := filepath.Clean(path)
	actual, err := os.ReadFile(cleanPath)
	if err != nil {
		return interfaceContractCarrierCheckResult{}, fmt.Errorf("read generated contract carrier: %w", err)
	}
	result := interfaceContractCarrierCheckResult{
		Kind:                  "haft_interface_generated_contract_carrier_check",
		SchemaVersion:         1,
		Authority:             "check_report_not_runtime_or_binding_authority",
		Path:                  cleanPath,
		Source:                source,
		SourceDigest:          sourceDigest,
		ExpectedCarrierDigest: interfaceContractGenerationDigestBytes(expected),
		ActualCarrierDigest:   interfaceContractGenerationDigestBytes(actual),
		Match:                 string(actual) == string(expected),
	}
	if !result.Match {
		return result, fmt.Errorf("generated contract carrier drift: path=%s expected=%s actual=%s", result.Path, result.ExpectedCarrierDigest, result.ActualCarrierDigest)
	}
	return result, nil
}

func interfaceContractAllActionFieldsExcluded(coverage interfaceContractAuditSchemaCoverage) bool {
	if len(coverage.ActionRequiredFields) == 0 || len(coverage.ExcludedFields) == 0 {
		return false
	}
	excluded := make(map[string]bool, len(coverage.ExcludedFields))
	for _, field := range coverage.ExcludedFields {
		excluded[field] = true
	}
	for _, field := range coverage.ActionRequiredFields {
		if !excluded[field] {
			return false
		}
	}
	return true
}

func interfaceContractGenerationValidationRefs() []string {
	return []string{
		"internal/cli/interface_test.go",
		"internal/cli/serve_parity_test.go",
		"internal/fpf/server_test.go",
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

func interfaceContractMaterializedCarriers(sourceDigest string) []interfaceContractMaterializedCarrier {
	bindingBoundary := "binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts"
	readOnlyBoundary := "read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization"
	carriers := []interfaceContractMaterializedCarrier{
		{
			CarrierPath:           "packages/haft-pi/extensions/haft/tools.ts",
			CarrierKind:           "pi_tool_metadata",
			ContractRole:          "tool_schema_and_description_materialization",
			SourceContract:        "kernel_interface_catalog",
			ExpectedSourceDigest:  sourceDigest,
			SyncPosture:           "digest_guarded_by_repo_regression",
			GuardPosture:          "source_digest_and_authority_boundary_guarded",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"kernelInterfaceCatalogDigest", sourceDigest, "contract_generation", bindingBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set", "decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiToolMetadataCarriesGeneratedContractAuthorityBoundaries", "internal/cli/interface_test.go:TestPiToolSchemasMirrorGeneratedSchemaFragments"},
		},
		{
			CarrierPath:           "packages/haft-pi/skills/h-status/SKILL.md",
			CarrierKind:           "pi_skill",
			ContractRole:          "status_drill_down_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     readOnlyBoundary,
			RequiredMarkers:       []string{"contract_generation", "generated-fragment", "drift fanout", "reconciliation", "current-authority drill-downs", readOnlyBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "packages/haft-pi/prompts/h-decide.md",
			CarrierKind:           "pi_manual_gate_prompt",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiPromptCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "packages/haft-pi/prompts/h-commission.md",
			CarrierKind:           "pi_manual_gate_prompt",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiPromptCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "packages/haft-pi/prompts/h-reason.md",
			CarrierKind:           "pi_workflow_prompt",
			ContractRole:          "binding_denial_routing_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "generated text, schema visibility, and model-supplied fields are not approval receipts"},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiPromptCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-status/SKILL.md",
			CarrierKind:           "bundled_skill",
			ContractRole:          "status_drill_down_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     readOnlyBoundary,
			RequiredMarkers:       []string{"contract_generation", "generated-fragment", "drift fanout", "reconciliation", "current-authority drill-downs", readOnlyBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-verify/SKILL.md",
			CarrierKind:           "bundled_skill",
			ContractRole:          "maintenance_drill_down_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     readOnlyBoundary,
			RequiredMarkers:       []string{"contract_generation", "generated-fragment", "drift fanout", "reconciliation", "current-authority drill-downs", readOnlyBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-reason/SKILL.md",
			CarrierKind:           "bundled_skill",
			ContractRole:          "workflow_routing_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "contract_generation", "generated-fragment", bindingBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set", "decision.decide"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries", "internal/cli/interface_test.go:TestBundledSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-decide/SKILL.md",
			CarrierKind:           "bundled_manual_skill",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-commission/SKILL.md",
			CarrierKind:           "bundled_manual_skill",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "internal/cli/claude_md_template.md",
			CarrierKind:           "host_instruction_template",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "AGENTS.md",
			CarrierKind:           "repo_agent_instruction",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "CLAUDE.md",
			CarrierKind:           "repo_agent_instruction",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_boundary_guarded_digest_missing",
			GuardPosture:          "authority_boundary_guarded_source_digest_missing",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries", "internal/cli/claude_md_test.go:TestClaudeMDTemplateInSyncWithRepoRoot"},
		},
	}
	for index := range carriers {
		if carriers[index].ExpectedSourceDigest == "" {
			carriers[index].ExpectedSourceDigest = sourceDigest
			carriers[index].RequiredMarkers = append(carriers[index].RequiredMarkers, sourceDigest)
		}
		if carriers[index].SyncPosture == "authority_boundary_guarded_digest_missing" {
			carriers[index].SyncPosture = "digest_guarded_by_repo_regression"
		}
		if carriers[index].GuardPosture == "authority_boundary_guarded_source_digest_missing" {
			carriers[index].GuardPosture = "source_digest_and_authority_boundary_guarded"
		}
	}
	return carriers
}

func countInterfaceContractDigestGuardedCarriers(carriers []interfaceContractMaterializedCarrier) int {
	total := 0
	for _, carrier := range carriers {
		if strings.Contains(carrier.GuardPosture, "source_digest") && !strings.Contains(carrier.GuardPosture, "source_digest_missing") {
			total++
		}
	}
	return total
}

func countInterfaceContractAuthorityGuardedCarriers(carriers []interfaceContractMaterializedCarrier) int {
	total := 0
	for _, carrier := range carriers {
		if carrier.AuthorityBoundary == "" {
			continue
		}
		total++
	}
	return total
}

func countInterfaceContractGenerationFields(targets []interfaceContractGenerationTarget) int {
	total := 0
	for _, target := range targets {
		total += len(target.Fields)
	}
	return total
}

func countInterfaceContractBindingFragments(fragments []interfaceContractGeneratedFragment) int {
	total := 0
	for _, fragment := range fragments {
		if interfaceContractAuditBindingSensitive(fragment.AuthorityPosture) {
			total++
		}
	}
	return total
}

func interfaceContractGeneratedFragmentFor(
	surface interfaceContractAuditSurface,
	capability interfaceCapability,
) interfaceContractGeneratedFragment {
	inputFields := topLevelInterfaceContractFields(capability.InputContract)
	inputFields = uniqueSortedStrings(inputFields)
	if len(inputFields) == 0 {
		inputFields = []string{surface.SchemaCoverage.Status, surface.ShapeCoverage.Status}
	}

	source := map[string]any{
		"capability_id":       surface.CapabilityID,
		"mcp_tool":            surface.MCPTool,
		"mcp_action":          surface.MCPAction,
		"cli_status":          surface.CLIStatus,
		"authority_posture":   surface.AuthorityPosture,
		"host_schema_posture": surface.HostSchemaPosture,
		"contract_fragment":   surface.ContractFragmentPosture,
		"input_fields":        inputFields,
	}

	return interfaceContractGeneratedFragment{
		CapabilityID:      surface.CapabilityID,
		FragmentKind:      "host_skill_plugin_description_preview",
		SourceContract:    "kernel_interface_catalog",
		SourceDigest:      interfaceContractGenerationDigest(source),
		HostSchemaPosture: surface.HostSchemaPosture,
		AuthorityPosture:  surface.AuthorityPosture,
		AuthorityBoundary: interfaceContractGeneratedFragmentAuthorityBoundary(surface),
		MCPTool:           surface.MCPTool,
		MCPAction:         surface.MCPAction,
		CLIStatus:         surface.CLIStatus,
		CLICommand:        surface.CLICommand,
		GeneratedText:     interfaceContractGeneratedFragmentText(surface),
		InputFields:       inputFields,
		ValidationRefs:    uniqueInterfaceContractAuditStrings(surface.ValidationRefs),
	}
}

func interfaceContractGeneratedSchemaFragmentFor(
	surface interfaceContractAuditSurface,
	capability interfaceCapability,
	toolSchema interfaceContractAuditToolSchema,
) interfaceContractGeneratedSchemaFragment {
	allowedFields := topLevelInterfaceContractFields(capability.InputContract)
	allowedFields = interfaceContractFilterExcludedSchemaFields(allowedFields, surface.SchemaCoverage.ExcludedFields)
	requiredFields := interfaceContractAuditExpectedMCPRequiredFields(capability)
	actionRequiredFields := topLevelInterfaceContractRequiredFields(capability.InputContract)
	actionRequiredFields = interfaceContractFilterExcludedSchemaFields(actionRequiredFields, surface.SchemaCoverage.ExcludedFields)
	handlerValidated := make([]string, 0, len(actionRequiredFields))
	requiredSet := make(map[string]bool, len(requiredFields))
	for _, field := range requiredFields {
		requiredSet[field] = true
	}
	for _, field := range actionRequiredFields {
		if requiredSet[field] {
			continue
		}
		handlerValidated = append(handlerValidated, field)
	}
	handlerValidated = uniqueSortedStrings(handlerValidated)

	schema := interfaceContractGeneratedSchemaFor(surface, allowedFields, requiredFields, toolSchema)
	source := map[string]any{
		"capability_id":            surface.CapabilityID,
		"mcp_tool":                 surface.MCPTool,
		"mcp_action":               surface.MCPAction,
		"allowed_top_level_fields": allowedFields,
		"required_fields":          requiredFields,
		"action_required_fields":   actionRequiredFields,
		"handler_validated":        handlerValidated,
		"schema":                   schema,
	}

	return interfaceContractGeneratedSchemaFragment{
		CapabilityID:           surface.CapabilityID,
		FragmentKind:           "mcp_action_schema_fragment",
		SourceContract:         "kernel_interface_catalog",
		SourceDigest:           interfaceContractGenerationDigest(source),
		AuthorityBoundary:      "schema fragment is read-only validation material, not operator authorization or host materialization",
		MCPTool:                surface.MCPTool,
		MCPAction:              surface.MCPAction,
		HostSchemaPosture:      surface.HostSchemaPosture,
		RequiredFields:         requiredFields,
		AllowedTopLevelFields:  allowedFields,
		ActionRequiredFields:   actionRequiredFields,
		HandlerValidatedFields: handlerValidated,
		Schema:                 schema,
		SchemaDigest:           interfaceContractGenerationDigest(schema),
		ValidationRefs:         uniqueInterfaceContractAuditStrings(surface.ValidationRefs),
	}
}

func interfaceContractFilterExcludedSchemaFields(fields []string, excludedFields []string) []string {
	if len(fields) == 0 || len(excludedFields) == 0 {
		return fields
	}
	excluded := make(map[string]bool, len(excludedFields))
	for _, field := range excludedFields {
		excluded[field] = true
	}
	filtered := make([]string, 0, len(fields))
	for _, field := range fields {
		if excluded[field] {
			continue
		}
		filtered = append(filtered, field)
	}
	return filtered
}

func interfaceContractGeneratedSchemaFor(
	surface interfaceContractAuditSurface,
	allowedFields []string,
	requiredFields []string,
	toolSchema interfaceContractAuditToolSchema,
) map[string]any {
	properties := make(map[string]any, len(allowedFields)+1)
	properties["action"] = map[string]any{
		"type":  "string",
		"const": surface.MCPAction,
	}
	for _, field := range allowedFields {
		if field == "action" {
			continue
		}
		propertySchema, ok := cloneJSONLikeMap(toolSchema.Properties[field])
		if !ok {
			propertySchema = map[string]any{
				"description": "shape validated by MCP schema mirror and kernel handler",
			}
		}
		properties[field] = propertySchema
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"required":             requiredFields,
		"properties":           properties,
	}
}

func interfaceContractRuntimeSchemaAuditFor(
	report interfaceContractGenerationReport,
) interfaceContractRuntimeSchemaAudit {
	toolSchemas := interfaceContractAuditToolSchemas()
	audit := interfaceContractRuntimeSchemaAudit{
		Authority:                   "read_only_runtime_schema_validation_not_generation_authority",
		Source:                      "live_mcp_tools_list_tool_catalog",
		Status:                      "clean",
		RuntimeTools:                len(toolSchemas),
		GeneratedSchemaFragments:    len(report.SchemaFragments),
		AdditionalPropertiesPosture: "preserved_from_runtime_mcp_schema; generated action fragments remain permissive for host compatibility",
		ValidationRefs: []string{
			"internal/cli/interface_test.go:TestInterfaceContractGenerationRuntimeSchemaAuditValidatesLiveToolCatalog",
			"internal/cli/interface_test.go:TestInterfaceContractGenerationRuntimeSchemaAuditDetectsFragmentDrift",
		},
	}

	for _, fragment := range report.SchemaFragments {
		schema, ok := toolSchemas[fragment.MCPTool]
		if !ok {
			audit.MissingRuntimeTools = append(audit.MissingRuntimeTools, fragment.CapabilityID)
			continue
		}
		if !interfaceContractRuntimeSchemaActionContains(schema, fragment.MCPAction) {
			audit.MissingRuntimeActionEnums = append(audit.MissingRuntimeActionEnums, fragment.CapabilityID)
		}
		audit.MissingRuntimeProperties = append(audit.MissingRuntimeProperties, interfaceContractRuntimeMissingProperties(fragment, schema)...)
		audit.MissingRuntimeRequired = append(audit.MissingRuntimeRequired, interfaceContractRuntimeMissingRequired(fragment, schema)...)
		if interfaceContractRuntimeSchemaDigest(fragment, schema) != fragment.SchemaDigest {
			audit.SchemaDigestMismatches = append(audit.SchemaDigestMismatches, fragment.CapabilityID)
		}
	}

	audit.MissingRuntimeTools = uniqueSortedStrings(audit.MissingRuntimeTools)
	audit.MissingRuntimeActionEnums = uniqueSortedStrings(audit.MissingRuntimeActionEnums)
	audit.MissingRuntimeProperties = uniqueSortedStrings(audit.MissingRuntimeProperties)
	audit.MissingRuntimeRequired = uniqueSortedStrings(audit.MissingRuntimeRequired)
	audit.SchemaDigestMismatches = uniqueSortedStrings(audit.SchemaDigestMismatches)
	audit.RuntimeSchemaDrift = len(audit.MissingRuntimeTools) +
		len(audit.MissingRuntimeActionEnums) +
		len(audit.MissingRuntimeProperties) +
		len(audit.MissingRuntimeRequired) +
		len(audit.SchemaDigestMismatches)
	audit.RuntimeSchemaMirrors = audit.GeneratedSchemaFragments - audit.RuntimeSchemaDrift
	if audit.RuntimeSchemaMirrors < 0 {
		audit.RuntimeSchemaMirrors = 0
	}
	if audit.RuntimeSchemaDrift != 0 {
		audit.Status = "drift"
	}

	return audit
}

func interfaceContractRuntimeMissingProperties(
	fragment interfaceContractGeneratedSchemaFragment,
	schema interfaceContractAuditToolSchema,
) []string {
	missing := make([]string, 0)
	for _, field := range fragment.AllowedTopLevelFields {
		if field == "action" {
			continue
		}
		if _, ok := schema.Properties[field]; ok {
			continue
		}
		missing = append(missing, fragment.CapabilityID+"."+field)
	}
	return missing
}

func interfaceContractRuntimeMissingRequired(
	fragment interfaceContractGeneratedSchemaFragment,
	schema interfaceContractAuditToolSchema,
) []string {
	missing := make([]string, 0)
	for _, field := range fragment.RequiredFields {
		if schema.Required[field] {
			continue
		}
		missing = append(missing, fragment.CapabilityID+"."+field)
	}
	return missing
}

func interfaceContractRuntimeSchemaDigest(
	fragment interfaceContractGeneratedSchemaFragment,
	schema interfaceContractAuditToolSchema,
) string {
	surface := interfaceContractAuditSurface{
		MCPAction: fragment.MCPAction,
	}
	generated := interfaceContractGeneratedSchemaFor(
		surface,
		fragment.AllowedTopLevelFields,
		fragment.RequiredFields,
		schema,
	)
	return interfaceContractGenerationDigest(generated)
}

func interfaceContractRuntimeSchemaActionContains(
	schema interfaceContractAuditToolSchema,
	action string,
) bool {
	actionSchema, ok := schema.Properties["action"]
	if !ok {
		return false
	}
	values := stringEnumFromSchemaProperty(actionSchema)
	for _, value := range values {
		if value == action {
			return true
		}
	}
	return false
}

func stringEnumFromSchemaProperty(value interface{}) []string {
	property, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	rawEnum, ok := property["enum"]
	if !ok {
		return nil
	}

	result := make([]string, 0)
	switch typed := rawEnum.(type) {
	case []string:
		result = append(result, typed...)
	case []interface{}:
		for _, item := range typed {
			text, ok := item.(string)
			if ok && text != "" {
				result = append(result, text)
			}
		}
	}
	return uniqueSortedStrings(result)
}

func cloneJSONLikeMap(value any) (map[string]any, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

func interfaceContractGeneratedFragmentText(surface interfaceContractAuditSurface) string {
	parts := []string{
		surface.CapabilityID,
		strings.TrimSpace(surface.CLICommand),
		strings.TrimSpace(surface.MCPTool),
		strings.TrimSpace(surface.MCPAction),
		surface.AuthorityPosture,
		interfaceContractGeneratedFragmentAuthorityBoundary(surface),
		"Generated from kernel_interface_catalog; do not edit host/skill/plugin/Pi wording by hand when this fragment is available.",
	}
	compact := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		compact = append(compact, part)
	}
	return strings.Join(compact, " | ")
}

func interfaceContractGeneratedFragmentAuthorityBoundary(surface interfaceContractAuditSurface) string {
	if interfaceContractAuditBindingSensitive(surface.AuthorityPosture) {
		return "binding actions require explicit operator/manual authorization; generated text, schema visibility, and model-supplied fields are not approval receipts"
	}
	return "read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization"
}

func interfaceContractGenerationDigest(source any) string {
	payload, err := json.Marshal(source)
	if err != nil {
		return "sha256:unavailable"
	}
	return interfaceContractGenerationDigestBytes(payload)
}

func interfaceContractGenerationDigestBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", sum)
}

func buildInterfaceContractAuditReport(catalog []interfaceCapability) interfaceContractAuditReport {
	surfaces := make([]interfaceContractAuditSurface, 0, len(catalog))
	summary := interfaceContractAuditSummary{Capabilities: len(catalog)}
	toolSchemas := interfaceContractAuditToolSchemas()
	exclusions := interfaceContractAuditSchemaFieldExclusions()

	for _, capability := range catalog {
		surface := buildInterfaceContractAuditSurface(capability, toolSchemas, exclusions)
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
		if surface.SchemaCoverage.Status == "missing_fields" ||
			surface.SchemaCoverage.Status == "missing_required_fields" ||
			surface.SchemaCoverage.Status == "missing_mcp_tool_schema" {
			summary.SchemaMissingSurfaces++
		}
		summary.SchemaExcludedFields += len(surface.SchemaCoverage.ExcludedFields)
		if len(surface.SchemaCoverage.MissingRequiredFields) == 0 &&
			surface.SchemaCoverage.Checked &&
			surface.SchemaCoverage.RequiredPosture != "" {
			summary.SchemaRequiredCovered++
		}
		if len(surface.SchemaCoverage.MissingRequiredFields) > 0 {
			summary.SchemaRequiredMissing++
		}
		summary.SchemaMissingRequiredFields += len(surface.SchemaCoverage.MissingRequiredFields)
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
		switch surface.ContractFragmentPosture {
		case "generated_target_fragment":
			summary.GeneratedTargetFragments++
		case "validated_fragment":
			summary.ValidatedFragments++
		case "legacy_fragment":
			summary.LegacyFragments++
		default:
			summary.UnvalidatedFragments++
		}
	}

	return interfaceContractAuditReport{
		Kind:              "haft_interface_contract_audit",
		SchemaVersion:     1,
		Authority:         "read_only_contract_inventory_not_schema_generation",
		AuthorityBoundary: interfaceContractAuditAuthorityBoundaryFor(),
		Summary:           summary,
		Surfaces:          surfaces,
		Notes: []string{
			"Kernel interface catalog is the audited contract source for this report.",
			"Host schema posture classifies each fragment as a validated mirror, manual CLI contract, or unvalidated host fragment.",
			"Contract fragment posture classifies every fragment as generated target, validated, legacy/manual, or unvalidated.",
			"MCP required-field coverage checks transport-level required fields such as action; action-specific required fields stay in the kernel contract and handler validation.",
			"Schema visibility is not operator authorization, binding authority, evidence, gate passage, claim truth, global truth, or publication.",
			"Generated MCP/host schema work remains a later phase and must validate against this inventory.",
			"Default status must not inline this report; use haft interface contract-audit --json or haft_query(action=\"contract_audit\").",
		},
	}
}

func interfaceContractAuditAuthorityBoundaryFor() interfaceContractAuditAuthorityBoundary {
	return interfaceContractAuditAuthorityBoundary{
		Inventory:           "read_only_contract_inventory",
		SchemaGeneration:    "not_schema_generation",
		HostMaterialization: "not_host_materialization",
		Evidence:            "not_evidence",
		Approval:            "not_approval",
		GateDecision:        "not_gate_decision",
		ClaimTruth:          "not_claim_truth",
		GlobalTruth:         "not_global_truth",
		Publication:         "not_publication",
	}
}

func buildInterfaceContractAuditSurface(
	capability interfaceCapability,
	toolSchemas map[string]interfaceContractAuditToolSchema,
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
		SchemaCoverage:   interfaceContractAuditSchemaCoverageFor(capability, toolSchemas, exclusions),
		ShapeCoverage:    interfaceContractAuditShapeCoverageFor(capability, toolSchemas, exclusions),
	}
	surface.ContractSourcePosture = interfaceContractAuditContractSourcePosture(surface)
	surface.HostSchemaPosture = interfaceContractAuditHostSchemaPosture(surface)
	surface.ContractFragmentPosture = interfaceContractAuditContractFragmentPosture(surface)

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

func interfaceContractAuditContractFragmentPosture(surface interfaceContractAuditSurface) string {
	switch surface.HostSchemaPosture {
	case "validated_mcp_mirror":
		return "validated_fragment"
	case "validated_mcp_mirror_with_generator_targets":
		return "generated_target_fragment"
	case "manual_cli_contract_not_generated":
		return "legacy_fragment"
	default:
		return "unvalidated_fragment"
	}
}

func interfaceContractAuditShapeCoverageFor(
	capability interfaceCapability,
	toolSchemas map[string]interfaceContractAuditToolSchema,
	exclusions map[string]map[string]bool,
) interfaceContractAuditShapeCoverage {
	execution := capability.CurrentExecution
	if execution.MCPTool == "" || execution.MCPAction == "" {
		return interfaceContractAuditShapeCoverage{
			Checked: false,
			Status:  "not_mcp_backed",
		}
	}

	toolSchema, ok := toolSchemas[execution.MCPTool]
	if !ok {
		return interfaceContractAuditShapeCoverage{
			Checked:            true,
			Status:             "missing_mcp_tool_schema",
			MissingShapeFields: []string{"action"},
		}
	}
	properties := toolSchema.Properties

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
	toolSchemas map[string]interfaceContractAuditToolSchema,
	exclusions map[string]map[string]bool,
) interfaceContractAuditSchemaCoverage {
	execution := capability.CurrentExecution
	if execution.MCPTool == "" || execution.MCPAction == "" {
		return interfaceContractAuditSchemaCoverage{
			Checked: false,
			Status:  "not_mcp_backed",
		}
	}

	toolSchema, ok := toolSchemas[execution.MCPTool]
	if !ok {
		return interfaceContractAuditSchemaCoverage{
			Checked:       true,
			Status:        "missing_mcp_tool_schema",
			MissingFields: []string{"action"},
		}
	}
	properties := toolSchema.Properties

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

	expectedRequired := interfaceContractAuditExpectedMCPRequiredFields(capability)
	missingRequired := make([]string, 0)
	for _, field := range expectedRequired {
		if !toolSchema.Required[field] {
			missingRequired = append(missingRequired, field)
		}
	}
	missingRequired = uniqueSortedStrings(missingRequired)

	status := "covered"
	if len(missing) > 0 {
		status = "missing_fields"
	} else if len(missingRequired) > 0 {
		status = "missing_required_fields"
	}

	return interfaceContractAuditSchemaCoverage{
		Checked:               true,
		Status:                status,
		MissingFields:         missing,
		ExcludedFields:        excluded,
		MCPRequiredFields:     sortedStringSetKeys(toolSchema.Required),
		MissingRequiredFields: missingRequired,
		ActionRequiredFields:  topLevelInterfaceContractRequiredFields(capability.InputContract),
		RequiredPosture:       interfaceContractAuditRequiredPosture(expectedRequired, missingRequired),
	}
}

type interfaceContractAuditToolSchema struct {
	Properties map[string]interface{}
	Required   map[string]bool
}

func interfaceContractAuditToolSchemas() map[string]interfaceContractAuditToolSchema {
	server := fpf.NewServer()
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})

	toolSchemas := make(map[string]interfaceContractAuditToolSchema)
	for _, tool := range server.ToolCatalog() {
		inputSchema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			continue
		}
		properties, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		toolSchemas[tool.Name] = interfaceContractAuditToolSchema{
			Properties: properties,
			Required:   stringSetFromSchemaRequired(inputSchema["required"]),
		}
	}
	return toolSchemas
}

func interfaceContractAuditExpectedMCPRequiredFields(capability interfaceCapability) []string {
	if capability.CurrentExecution.MCPTool == "" || capability.CurrentExecution.MCPAction == "" {
		return nil
	}
	return []string{"action"}
}

func interfaceContractAuditRequiredPosture(expected []string, missing []string) string {
	if len(expected) == 0 {
		return "no_transport_required_fields"
	}
	if len(missing) > 0 {
		return "missing_transport_required_fields"
	}
	return "transport_action_required_action_specific_fields_validated_by_handler"
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

func topLevelInterfaceContractRequiredFields(contract interfaceContract) []string {
	fields := make(map[string]bool)
	for _, field := range contract.RequiredFields {
		topLevel := topLevelInterfaceContractField(field)
		if topLevel == "" {
			continue
		}
		fields[topLevel] = true
	}
	return sortedStringSetKeys(fields)
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

func stringSetFromSchemaRequired(value interface{}) map[string]bool {
	result := make(map[string]bool)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if item != "" {
				result[item] = true
			}
		}
	case []interface{}:
		for _, item := range typed {
			text, ok := item.(string)
			if ok && text != "" {
				result[text] = true
			}
		}
	}
	return result
}

func sortedStringSetKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
	if capability.ID == "spec.apply_change" {
		return "sql_edition_sync_back_mutation_not_approval"
	}
	if capability.ID == "spec.export" {
		return "read_only_publication_projection"
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
	if capability.ID == "spec.apply_change" {
		refs = append(refs, "internal/cli/spec_apply_change_test.go")
	}
	if capability.ID == "spec.export" {
		refs = append(refs, "internal/cli/spec_export_test.go")
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
		"authority_boundary: inventory=%s schema_generation=%s host_materialization=%s evidence=%s approval=%s gate_decision=%s claim_truth=%s global_truth=%s publication=%s\n",
		report.AuthorityBoundary.Inventory,
		report.AuthorityBoundary.SchemaGeneration,
		report.AuthorityBoundary.HostMaterialization,
		report.AuthorityBoundary.Evidence,
		report.AuthorityBoundary.Approval,
		report.AuthorityBoundary.GateDecision,
		report.AuthorityBoundary.ClaimTruth,
		report.AuthorityBoundary.GlobalTruth,
		report.AuthorityBoundary.Publication,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: capabilities=%d kernel_owned=%d mcp_mirrored=%d cli_available=%d binding_sensitive=%d read_only=%d legacy_exceptions=%d schema_coverage=%d covered/%d missing excluded_fields=%d required_coverage=%d covered/%d missing missing_required_fields=%d shape_coverage=%d covered/%d missing generator_targets=%d fields=%d skipped_fields=%d host_fragments=%d validated_mcp/%d manual_cli/%d unvalidated fragment_posture=%d generated_targets/%d validated/%d legacy/%d unvalidated\n",
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
		report.Summary.SchemaRequiredCovered,
		report.Summary.SchemaRequiredMissing,
		report.Summary.SchemaMissingRequiredFields,
		report.Summary.ShapeCoveredSurfaces,
		report.Summary.ShapeMissingSurfaces,
		report.Summary.ShapeGeneratorTargets,
		report.Summary.ShapeGeneratorTargetFields,
		report.Summary.ShapeSkippedFields,
		report.Summary.ValidatedMCPMirrors,
		report.Summary.ManualCLIContracts,
		report.Summary.UnvalidatedHostFragments,
		report.Summary.GeneratedTargetFragments,
		report.Summary.ValidatedFragments,
		report.Summary.LegacyFragments,
		report.Summary.UnvalidatedFragments,
	); err != nil {
		return err
	}

	for _, surface := range report.Surfaces {
		if _, err := fmt.Fprintf(output, "- %s source=%s fragment=%s host_schema=%s schema=%s schema_coverage=%s shape_coverage=%s authority=%s cli=%s\n", surface.CapabilityID, surface.ContractSourcePosture, surface.ContractFragmentPosture, surface.HostSchemaPosture, surface.SchemaPosture, surface.SchemaCoverage.Status, surface.ShapeCoverage.Status, surface.AuthorityPosture, surface.CLIStatus); err != nil {
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
		"summary: capabilities=%d generator_target_surfaces=%d generator_target_fields=%d generated_preview_fragments=%d generated_schema_fragments=%d runtime_schema_mirrors=%d runtime_schema_drift=%d binding_preview_fragments=%d materialized_carriers=%d digest_guarded_carriers=%d authority_boundary_guarded_carriers=%d validation_refs=%d\n",
		report.Summary.Capabilities,
		report.Summary.GeneratorTargetSurfaces,
		report.Summary.GeneratorTargetFields,
		report.Summary.GeneratedPreviewFragments,
		report.Summary.GeneratedSchemaFragments,
		report.Summary.RuntimeSchemaMirrors,
		report.Summary.RuntimeSchemaDrift,
		report.Summary.BindingPreviewFragments,
		report.Summary.MaterializedCarriers,
		report.Summary.DigestGuardedCarriers,
		report.Summary.AuthorityGuardedCarriers,
		len(report.ValidationRefs),
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

func writeInterfaceContractSchemaMaterializationText(
	output io.Writer,
	result interfaceContractSchemaMaterializationResult,
) error {
	if _, err := fmt.Fprintf(output, "Haft interface schema fragments materialized v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "path: %s\n", result.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "carrier_digest: %s\n", result.CarrierDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "schema_fragments: %d\n", result.SchemaFragments); err != nil {
		return err
	}
	return nil
}

func writeInterfaceContractDescriptionMaterializationText(
	output io.Writer,
	result interfaceContractDescriptionMaterializationResult,
) error {
	if _, err := fmt.Fprintf(output, "Haft interface description fragments materialized v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "path: %s\n", result.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "carrier_digest: %s\n", result.CarrierDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "description_fragments: %d\n", result.DescriptionFragments); err != nil {
		return err
	}
	return nil
}

func writeInterfaceContractCarrierCheckText(
	output io.Writer,
	result interfaceContractCarrierCheckResult,
) error {
	if _, err := fmt.Fprintf(output, "Haft interface generated contract carrier check v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "path: %s\n", result.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "expected_carrier_digest: %s\n", result.ExpectedCarrierDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "actual_carrier_digest: %s\n", result.ActualCarrierDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "match: %t\n", result.Match); err != nil {
		return err
	}
	return nil
}

func writeInterfaceContractMaterializedCarrierCheckText(
	output io.Writer,
	result interfaceContractMaterializedCarrierCheckReport,
) error {
	if _, err := fmt.Fprintf(output, "Haft interface materialized carrier check v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: materialized_carriers=%d checked_carriers=%d missing_carrier_files=%d missing_markers=%d match=%t\n",
		result.Summary.MaterializedCarriers,
		result.Summary.CheckedCarriers,
		result.Summary.MissingCarrierFiles,
		result.Summary.MissingMarkers,
		result.Summary.Match,
	); err != nil {
		return err
	}
	for _, carrier := range result.Carriers {
		if carrier.Match {
			continue
		}
		if _, err := fmt.Fprintf(output, "- %s missing_file=%t missing_markers=%d\n", carrier.CarrierPath, carrier.MissingFile, len(carrier.MissingMarkers)); err != nil {
			return err
		}
	}
	return nil
}

func writeInterfaceContractMaterializedCarrierSyncText(
	output io.Writer,
	result interfaceContractMaterializedCarrierSyncReport,
) error {
	if _, err := fmt.Fprintf(output, "Haft interface materialized carrier sync v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: materialized_carriers=%d synced_carriers=%d unchanged_carriers=%d missing_carrier_files=%d updated_markers=%d match=%t\n",
		result.Summary.MaterializedCarriers,
		result.Summary.SyncedCarriers,
		result.Summary.UnchangedCarriers,
		result.Summary.MissingCarrierFiles,
		result.Summary.UpdatedMarkers,
		result.Summary.Match,
	); err != nil {
		return err
	}
	for _, carrier := range result.Carriers {
		if !carrier.Synced && !carrier.MissingFile {
			continue
		}
		status := "synced"
		if carrier.MissingFile {
			status = "missing"
		}
		if _, err := fmt.Fprintf(
			output,
			"- %s: %s updated_markers=%d\n",
			status,
			carrier.CarrierPath,
			carrier.UpdatedMarkers,
		); err != nil {
			return err
		}
	}
	return nil
}
