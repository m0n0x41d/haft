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
	"github.com/m0n0x41d/haft/internal/projectmemory"
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

const humanGateBriefRequirement = "Before requesting a human gate, present a self-contained Human Gate Brief in ordinary language: name the gate kind, the exact readable subject, the affected operation and why only it is blocked; list the real options available now and, for each, what changes, what stays unchanged, the immediate consequence or return condition, and the weakest link; summarize any existing comparison basis, parity, selection policy, and non-dominated or Pareto set, or explicitly say that no such comparison exists or is applicable; mark the agent recommendation as advisory; state evidence freshness or expiry; and ask for the human engineer's assessment of the options, trade-offs, and recommendation in natural language. A command, skill invocation, exact reply phrase, or resumption token must never substitute for that consultation. Accept ordinary language as the substantive answer. When that answer directly and unambiguously selects the exact effect, subject, option, and scope for DecisionRecord binding, manual profile application, or ProjectTypeEnvHead selection, route it as host_routed_operator_request without requiring a skill name or second confirmation; it is not reusable authority for another effect. A bare yes is usable only for one current unambiguous brief. WorkCommission creation remains a separately required manual authority act. Never end a blocking message with 'for resumption it is enough to...', 'reply exactly...', or an equivalent command-only instruction. The operator must not be expected to infer hidden state, alternatives, rationale, IDs, or hashes, and the brief itself is explanation rather than authority."

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
	OutputContract   any                `json:"output_contract,omitempty"`
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
	interfaceCmd.Flags().BoolVar(&interfaceCheckMaterializedCarriers, "check-materialized-carriers", false, "check required marker-string presence in listed repo carriers; does not compare or verify semantic bytes")
	interfaceCmd.Flags().BoolVar(&interfaceSyncMaterializedCarriers, "sync-materialized-carriers", false, "legacy name: refresh only kernel-interface source-digest marker lines in listed repo carriers; does not render or verify semantic bytes")
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
				OptionalFields: []string{"problem_type", "problem_profile", "source_kind", "why_now", "scope", "acceptance_probe", "freshness_disposition", "constraints", "optimization_targets", "observation_indicators", "acceptance", "blast_radius", "reversibility", "context", "mode", "task_context", "entity_ref", "bounded_context_ref"},
				FieldShapes: []fieldShape{{
					Field: "entity_ref",
					Shape: `{"ref_kind_id":"U.EntityRef","reference_id":"entity:authorization-service"}`,
					Note:  "Pair with bounded_context_ref for an exact typed ProblemCardAtConcern projection. Never derive this identity from the artifact ID.",
				}},
				Notes: []string{"Frame before exploring when the problem is fuzzy or a solution was proposed before acceptance criteria.", "P2W readiness is computed: wish/ticket/chosen_method sources cannot become ready without explicit scope and acceptance probe.", "Without exact EntityOfConcern coordinates the ProblemCard may persist while typed projection remains underdetermined."},
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
						Shape: `{"name":"latency","role":"target","polarity":"lower_better","scale_type":"ratio","unit":"ms","valid_until":"2026-08-12"}`,
						Note:  "One object per comparison dimension; name is required. Polarity, when present, is exactly higher_better or lower_better.",
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
									"polarity":       "lower_better",
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
				OptionalFields: []string{"problem_ref", "variants[].description", "variants[].strengths", "variants[].risks", "variants[].stepping_stone", "variants[].stepping_stone_basis", "variants[].project_record_ref", "no_stepping_stone_rationale", "context", "mode", "task_context", "entity_ref", "bounded_context_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "variants[]",
						Shape: `{"title":"...","weakest_link":"...","novelty_marker":"...","stepping_stone":true,"stepping_stone_basis":"...","project_record_ref":{"ref_kind_id":"Haft.ProjectRecordRef","reference_id":"record:note-..."}}`,
						Note:  "At least two variants; title, weakest_link, and novelty_marker are required. A typed durable portfolio also needs one exact independently admitted project_record_ref per variant.",
					},
					{
						Field: "entity_ref",
						Shape: `{"ref_kind_id":"U.EntityRef","reference_id":"entity:authorization-service"}`,
						Note:  "Pair with bounded_context_ref and exact option refs for SolutionPortfolioAtConcern projection.",
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
				Notes: []string{"At least one variant should be a stepping stone, or explain why no stepping-stone variant exists.", "Missing exact option refs retain the legacy portfolio but leave typed projection underdetermined."},
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
				OptionalFields: []string{"non_dominated_set", "incomparable", "dominated_variants", "pareto_tradeoffs", "policy_applied", "recommendation_rationale", "legacy_recommendation_ref", "selected_ref", "entity_ref", "bounded_context_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "dimensions",
						Shape: `["latency","cost"]`,
						Note:  "Required on every compare call. Characterization records candidate dimensions, but compare remains explicit and replayable; do not omit this field after characterize.",
					},
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
					{
						Field: "entity_ref",
						Shape: `{"ref_kind_id":"U.EntityRef","reference_id":"entity:authorization-service"}`,
						Note:  "Pair with bounded_context_ref; comparison option identities are recovered only from the exact typed portfolio relation.",
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
					"Always include top-level dimensions in the compare payload, even when the ProblemCard was already characterized.",
					"legacy_recommendation_ref is advisory and is the preferred alias for legacy selected_ref; it is not ChoiceResult.",
					"CLI input-file still accepts legacy results{...}, but agents should use the flat fields shown in flat_compare.",
				},
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "decision.decide",
			Purpose: "Route one direct, unambiguous operator request for an exact bounded choice to a binding DecisionRecord. A manual h-decide token remains a compatible shortcut, not an authorization receipt. MCP decide remains fail-closed until a verifiable host receipt exists.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_decision",
				MCPAction:        "decide",
				MCPCall:          `haft_decision(action="decide", ...) -> operator_confirmation_required`,
				CLIStatus:        "input_file_execution_shipped",
				CLICommand:       "haft artifact create decision.decide --input-file input.json --json",
				DiscoveryCommand: "haft interface decision.decide --json",
				InputFileFlow: []string{
					"haft interface decision.decide --json",
					"write input JSON with full DRR fields",
					"route only a direct operator request with one exact effect, subject, selected option, and scope; otherwise present one Human Gate Brief and accept its natural-language answer",
					"haft artifact create decision.decide --input-file input.json --json",
				},
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"selected_title", "why_selected", "selection_policy", "weakest_link", "counterargument", "why_not_others", "rollback", "predictions", "invariants", "affected_files", "valid_until"},
				OptionalFields: []string{"problem_ref", "problem_refs", "problem_statement", "portfolio_ref", "choice_result", "transformation_record", "decision_subject_ref", "implementation_footprint", "governance_targets", "drift_watch_targets", "claims", "evidence_requirements", "refresh_triggers", "context", "mode", "task_context", "section_refs", "spec_binding_preflight", "spec_binding_preflight_required", "search_keywords", "binding_hints", "binding_scope", "binding_fallback_reason", "binding_targets", "_skips", "_skip_reason"},
				FieldShapes: []fieldShape{
					{
						Field: "problem_basis",
						Shape: `{"problem_ref":"prob-..."} | {"problem_refs":["prob-...","prob-..."]} | {"portfolio_ref":"sol-..."} | {"problem_statement":"bounded problem this direct decision addresses"}`,
						Note:  "At least one resolvable problem/portfolio ref or non-empty problem_statement is required. Use problem_statement only for a direct decision without real linked problem provenance; inline alternatives belong in choice_result.option_set.",
					},
					{
						Field: "choice_result",
						Shape: `{"subject_ref":"operator","option_set":["V1","V2"],"comparison_basis":["selected V1: ...","rejected V2: ..."],"choice_rule":"declared selection policy","next_move":"choose_now","variant_ref":"V1","reason":"operator directly selected V1","reversibility":"two-week rollback","reopen_condition":"reopen if rollback triggers occur"}`,
						Note:  "Exact human choice outcome; compare never creates it and DecisionRecord remains a compatibility projection.",
					},
					{
						Field: "transformation_record",
						Shape: `{"schema_version":1,"transformed_entity":"ProblemCard profile","initial_state":"implicit prose","post_state":"typed profile/readiness object","relation":"makes explicit","context":"semantic-spine slice","window":"2026-Q3","method_refs":["mpull-..."],"work_refs":["wc-..."],"evidence_refs":["evid-..."],"publication_refs":["pub-..."]}`,
						Note:  "Describes the target transformation only; method/work/evidence/publication refs remain separate records and are not proof of occurrence, approval, or publication.",
					},
					{
						Field: "spec_binding_preflight",
						Shape: `{"schema_version":1,"record_kind":"spec_binding_preflight","state":"provided_refs_valid|bound_existing|no_specs|no_active_sections|out_of_spec","selected_section_refs":["TS.boundary.001"],"status_debt":{"severity":"none|low|high","message":"..."}}`,
						Note:  "Receipt from query.spec_binding_preflight; invalid_refs, conflict, ambiguous, and draft_section_needed block DecisionRecord creation.",
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
					"MCP schema discovery may show decide fields, but MCP decide is fail-closed because model-supplied arguments are not operator authorization and no verifiable host receipt exists.",
					humanGateBriefRequirement,
					"Use the input-file CLI as an internal effect sink after the host routes a direct, unambiguous operator request. Record host_routed_operator_request; do not claim independent proof of U.SpeechAct.",
					"A quotation, pasted third-party text, agent proposal or recommendation, hypothetical, and tool output are not operator requests. A bare yes is usable only for one current unambiguous Human Gate Brief.",
					"Tactical skips are accepted only in tactical mode and require _skip_reason.",
					"problem_statement is conditionally required when no problem_ref, problem_refs, or portfolio_ref resolves; a direct decision does not require manufacturing a ProblemCard or SolutionPortfolio.",
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
				OptionalFields: []string{"observations", "evidence", "rationale", "anchors", "affected_files", "context", "valid_until", "search_keywords", "task_context", "entity_ref", "bounded_context_ref"},
				FieldShapes: []fieldShape{{
					Field: "entity_ref",
					Shape: `{"ref_kind_id":"U.EntityRef","reference_id":"entity:authorization-service"}`,
					Note:  "Pair with bounded_context_ref for an exact typed NoteAtConcern projection.",
				}},
				Notes: []string{"A note is not a decision; use decision.decide for choices among alternatives.", "Without exact EntityOfConcern coordinates the note may persist while typed projection remains underdetermined."},
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
				OptionalFields: []string{"declared_task_kind", "change_intent", "intended_files", "risk_signals", "user_scope_constraints", "artifact_refs", "carry_through", "ceremony_request", "response_budget", "context", "scope_id"},
				FieldShapes: []fieldShape{
					{
						Field: "carry_through[]",
						Shape: `{"source_ref":"review:external","source_item_ref":"finding-1","acceptance_ref":"operator:accepted","acceptance_ref_kind":"operator_message","acceptance_ref_status":"externally_asserted"}`,
						Note:  "Use only for accepted review/instruction items that must be disposed before close; model text alone is not acceptance, and externally_asserted refs are weaker than verified local receipts.",
					},
				},
				Notes: []string{"Use for feature, bugfix/debug, refactor, external integration, governed files, cross-module edits, behavior changes, or failing tests.", "Mechanical edits should request low/none ceremony.", "Pass exact scope_id only after scope_choice_required for a multi-scope canonical profile; never pass task, thread, commission, or work IDs as selectors. A singleton profile ignores an unnecessary mismatched selector and reports the selected canonical scope.", "NotApplicable or Underdetermined SWE MethodPack applicability returns a typed no-MethodRun result and never creates an empty run.", "carry_through starts as pending accepted-basis inventory; close must apply/reject/defer/supersede or waive it.", "acceptance_ref_kind/status classify the acceptance receipt posture; external assertions are allowed but not evidence truth or approval by themselves."},
			},
			OutputVolume: []string{"default: max 3 method cards, max 3 hard gates per card, plus close_template JSON", "detail action: full definition for one method"},
			Invariants: append(commonInterfaceInvariants(),
				"No internal LLM classification; the agent declares task shape and risk signals.",
				"Pull persists a MethodRun carrier under .haft/method-runs.",
			),
		},
		{
			ID:      "method.close",
			Purpose: "Close an existing MethodRun by pull_id with changed files, hard-gate results, verification evidence, accepted-basis carry-through dispositions, and explicit waivers.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_method",
				MCPAction:        "close",
				MCPCall:          `haft_method(action="close", pull_id="mpull-...", changed_files=[...], gate_results=[...], verification={...}, waivers=[...])`,
				CLIStatus:        "mcp_only",
				DiscoveryCommand: "haft interface method.close --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"pull_id"},
				OptionalFields: []string{"changed_files", "gate_results", "verification", "waivers", "carry_through"},
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
					{
						Field: "carry_through[]",
						Shape: `{"source_ref":"review:external","source_item_ref":"finding-1","acceptance_ref":"operator:accepted","acceptance_ref_kind":"operator_message","acceptance_ref_status":"externally_asserted","disposition":"applied","target_refs":["internal/method/run.go::ValidateClose"],"evidence_refs":["go test ./internal/method"]}`,
						Note:  "applied requires target_refs; rejected/deferred/superseded require reason; pending blocks close unless waiver carry_through_disposition_recorded is explicit; missing/malformed acceptance_ref posture is a validation issue.",
					},
				},
				Notes: []string{
					`gate_results[] shape: {"gate_id":"<hard-gate-id>","status":"satisfied","evidence_refs":["<evidence-ref>"]}`,
					`waivers[] shape: {"gate_id":"<hard-gate-id>","reason":"<why waived>"}`,
					`verification shape: {"commands":["<command>"],"result":"<pass|partial|failed>","output_ref":"<optional>"}`,
					`carry_through[] shape: {"source_ref":"<source>","source_item_ref":"<item>","acceptance_ref":"<operator-or-review-acceptance>","acceptance_ref_kind":"operator_message|review_disposition|decision_record|manual_cli_receipt|external_unverified|unknown","acceptance_ref_status":"verified|externally_asserted|missing|malformed","disposition":"applied|rejected|deferred|superseded","target_refs":["<changed-target>"],"reason":"<why>"}`,
					"Derive changed_files, verification.commands, test status, and governed-decision intersections from git diff, terminal traces, and code_context before asking the operator.",
					"Ask the operator only for irreducible judgment: waivers, ambiguous authority, or acceptance of residual risk.",
					"Hard gates require either satisfied evidence_refs or an explicit waiver reason.",
					"Accepted carry-through items must be disposed before close; unresolved items require waiver gate carry_through_disposition_recorded with an operator reason.",
					"acceptance_ref_kind/status classify receipt posture; externally_asserted refs can be carried through but do not become approval, evidence truth, or gate passage.",
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
			ID:      "method.catalog",
			Purpose: "Read the MethodPack lifecycle/discovery catalog without creating process authority.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_method",
				MCPAction:        "catalog",
				MCPCall:          `haft_method(action="catalog", method_status="current")`,
				CLIStatus:        "available",
				CLICommand:       "haft method catalog --status current --json",
				DiscoveryCommand: "haft interface method.catalog --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"method_status", "scope_id"},
				FieldShapes: []fieldShape{
					{
						Field: "method_status",
						Shape: `"current" | "experimental" | "superseded" | "deprecated" | "all"`,
						Note:  "Default current. Superseded/deprecated methods remain discoverable history but are not eligible for normal pull matching.",
					},
					{
						Field: "response",
						Shape: `{"kind":"haft_method_catalog","schema_version":1,"filter_status":"current","authority_boundary":"read_only_method_catalog_not_processpattern_not_enforcement_authority","methods":[{"id":"verification-before-completion","lifecycle":{"status":"current"},"carrier_refs":["internal/method/builtin.go",".haft/methods/swe-core/verification-before-completion.yaml"],"source_pattern_refs":["fpf:A.10","fpf:B.3","fpf:A.15"]}]}`,
						Note:  "Catalog discovery is read-only; source_pattern_refs are documentary context only, not evidence, approval, gate passage, or FPF/DPF source authority.",
					},
				},
				Notes: []string{
					"Use this when an agent needs the current process-method catalog or successor/deprecation lineage.",
					"Pass exact scope_id only after scope_choice_required for a multi-scope canonical profile; never pass task, thread, commission, or work IDs as selectors. NotApplicable and Underdetermined return a typed no-catalog result without manufacturing an empty SWE catalog.",
					"Do not inline this catalog into default status; query it explicitly.",
					"source_pattern_refs cite source-pattern context for the method; they never satisfy hard gates or evidence requirements.",
				},
			},
			OutputVolume: []string{"default MCP: JSON MethodPack catalog report", "CLI text: compact method list", "CLI --json: full report"},
			Invariants: append(commonInterfaceInvariants(),
				"MethodPack remains the process-governance substrate; this does not create a ProcessPattern artifact kind.",
				"Only lifecycle current methods are eligible for normal pull matching.",
				"Skills may point to carrier_refs but do not become enforcement authority.",
			),
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
						Shape: `{"kind":"haft_interface_contract_audit","schema_version":1,"authority":"read_only_contract_inventory_not_schema_generation","authority_boundary":{"inventory":"read_only_contract_inventory","schema_generation":"not_schema_generation","host_materialization":"not_host_materialization","evidence":"not_evidence","approval":"not_approval","gate_decision":"not_gate_decision","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"},"summary":{"capabilities":50,"kernel_owned_contracts":50,"mcp_mirrored_actions":29,"cli_available_surfaces":37,"binding_authority_surfaces":2,"read_only_surfaces":31,"legacy_transport_exceptions":23,"schema_covered_surfaces":45,"schema_missing_surfaces":0,"schema_excluded_fields":12,"schema_required_covered_surfaces":45,"schema_required_missing_surfaces":0,"schema_missing_required_fields":0,"shape_covered_surfaces":45,"shape_missing_surfaces":0,"shape_skipped_fields":49,"shape_generator_targets":0,"shape_generator_target_fields":0,"validated_mcp_mirrors":45,"manual_cli_contracts":5,"unvalidated_host_fragments":0,"generated_target_fragments":0,"validated_fragments":45,"legacy_fragments":5,"unvalidated_fragments":0},"surfaces":[{"capability_id":"decision.decide","contract_sources":["kernel_interface_catalog"],"contract_fragment_posture":"validated_fragment","schema_posture":"mcp_schema_mirrored","authority_posture":"binding_denied_by_default_mcp","validation_refs":["internal/cli/interface_test.go","internal/fpf/server_test.go"],"legacy_exception":false,"schema_coverage":{"checked":true,"status":"covered"},"shape_coverage":{"checked":true,"status":"covered"}}]}`,
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
						Shape: `{"kind":"haft_interface_contract_generation_manifest","schema_version":1,"authority":"read_only_generation_manifest_not_host_materialization","source":"kernel_interface_catalog","source_digest":"sha256:...","validation_refs":["internal/cli/interface_test.go","internal/fpf/server_test.go"],"summary":{"capabilities":50,"generator_target_surfaces":0,"generator_target_fields":0,"generated_preview_fragments":50,"generated_schema_fragments":44,"runtime_schema_mirrors":44,"runtime_schema_drift":0,"binding_preview_fragments":2,"materialized_carriers":14,"digest_marker_guarded_carriers":14,"authority_boundary_guarded_carriers":14},"surface_policy":{"default_status":"cue_or_count_only_never_inline_generation_manifest","default_code_context":"lane_index_only_never_inline_generated_descriptions","tools_list":"action_enum_and_compact_description_only_no_generated_schema_fragments","compact_cli":"summary_counts_only_field_targets_require_json","generated_descriptions":"drill_down_only_validate_with_carrier_semio_before_host_materialization","required_guards":["carrier_semio_authority_boundary","tools_list_context_budget","compact_status_no_manifest_inline","code_context_lane_index_default"]},"targets":[],"materialized_carriers":[{"carrier_path":"packages/haft-pi/extensions/haft/tools.ts","carrier_kind":"pi_tool_metadata","contract_role":"tool_schema_and_description_materialization","source_contract":"kernel_interface_catalog","expected_digest_marker":"sha256:...","marker_refresh_posture":"digest_marker_presence_only_not_semantic_sync","marker_guard_posture":"required_marker_presence_only_not_semantic_bytes"}],"generated_fragments":[{"capability_id":"decision.decide","fragment_kind":"host_skill_plugin_description_preview","source_contract":"kernel_interface_catalog","source_digest":"sha256:...","authority_boundary":"binding actions require effect-specific operator authority. Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts","generated_text":"...","input_fields":["choice_result","selected_title"]}],"generated_schema_fragments":[{"capability_id":"decision.decide","fragment_kind":"mcp_action_schema_fragment","schema_digest":"sha256:...","required_fields":["action"],"action_required_fields":["selected_title"],"handler_validated_fields":["selected_title"]}]}`,
						Note:  "The manifest is the kernel-owned generated-preview source plus any remaining generator queue; it does not materialize host schemas or authorize binding actions.",
					},
				},
				Notes: []string{
					"Use this after contract-audit: generated_fragments are the source for future host/skill/plugin/Pi wording; targets list remaining schema-generator work.",
					"Use `haft interface contract-generation --write-schema-fragments <path>` to materialize generated_schema_fragments as deterministic JSON validation carrier bytes.",
					"Use `haft interface contract-generation --write-description-fragments <path>` to materialize generated_fragments as deterministic JSON wording-sync carrier bytes.",
					"Use `--check-schema-fragments <path>` or `--check-description-fragments <path>` to fail when a materialized carrier drifted from the current kernel interface catalog.",
					"Use `--check-materialized-carriers` only to check required marker-string presence in listed repo carriers. It does not compare or verify semantic bytes or establish currentness.",
					"`--sync-materialized-carriers` is a compatibility flag that refreshes only kernel-interface source-digest marker lines. It does not render, synchronize, compare, or verify semantic carrier bytes.",
					"Read-only: generated fragments and schema generation targets are not operator authorization, evidence, or gate passage.",
				},
			},
			OutputVolume: []string{"default: compact generator target/fragment counts", "--json: generated fragments and field-level targets", "--write-schema-fragments/--write-description-fragments: deterministic generated carrier bytes; never in default status", "--check-schema-fragments/--check-description-fragments: compare generated fragment carriers without rewriting", "--check-materialized-carriers: required marker-string presence only, not semantic bytes", "--sync-materialized-carriers: legacy compatibility flag for source-digest marker-line refresh only; no semantic synchronization or currentness claim"},
			Invariants: append(commonInterfaceInvariants(),
				"Contract generation manifest is read-only.",
				"Generated fragments come from the kernel interface catalog.",
				"Fields come from kernel interface catalog field_shapes and current MCP schema gaps.",
				"Schema visibility remains separate from binding authorization.",
			),
		},
		{
			ID:      "query.status",
			Purpose: "Show the compact operator cockpit; use full=true for detailed status and expand any real gate through its read-only basis into a self-contained Human Gate Brief before asking the operator.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "status",
				MCPCall:          `haft_query(action="status", full=false)`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.status --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"context", "full", "scope_id"},
				Notes: []string{
					"Default output is a compact operator cockpit; omitted detail is not evidence of absence.",
					"Pass full=true for detailed status, including shipped/pending/unassessed decision lists, addressed problems, recent notes, and full coverage when available.",
					"When the canonical project profile has several scopes, pass one exact scope_id; status never collapses mixed scopes by ordering.",
					"Compact status omits not-applicable SoftwareSystemSpec pressure. Full status shows the exact scope-local profile basis. Underdetermined applicability produces one neutral profile cue.",
					"Use haft_query(action=\"coverage\") for module coverage, haft_refresh(action=\"scan\", verbose=true) for drift/stale detail, haft_refresh(action=\"plan\") for the maintenance work order, haft_refresh(action=\"review\") for a read-only needs-judgment packet, and haft_refresh(action=\"drain\", dry_run=true) to preview safe closures.",
					"Cockpit attention is not a project-wide Work gate. Continue unrelated already-authorized Work; interrupt only an affected binding or authority mutation, an explicit human lifecycle gate, or a current use that relies on unresolved contradictory binding content.",
					humanGateBriefRequirement,
					"An operator question must name the exact semantic choice and why the affected operation cannot continue. Bare acknowledgement prompts for status, evidence, historicity, cleanup, or already-authorized continuation are invalid.",
				},
			},
			OutputVolume: []string{"default: compact cockpit plus one-line coverage cue", "full=true: detailed status plus complete coverage projection"},
			Invariants: append(
				commonInterfaceInvariants(),
				"Status attention does not imply project-wide interruption or mutation authority.",
				"Only an exact current-use binding, authority, or human lifecycle conflict may require interruption.",
			),
		},
		{
			ID:      "query.fpf",
			Purpose: "Retrieve source-native FPF units by concern or exact identifier, then publish the result through a bounded working, trace, or diagnostic view without selecting a pattern or inferring a work order.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "fpf",
				MCPCall:          `haft_query(action="fpf", mode="concern", query="What distinction governs this question?", view="working", max_total_candidates=25)`,
				CLIStatus:        "available",
				CLICommand:       "haft fpf query \"What distinction governs this question?\" --view working --max-total-candidates 25 --json",
				DiscoveryCommand: "haft interface query.fpf --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"mode"},
				OptionalFields: []string{"query", "identifier", "entity_of_concern", "known_context", "intended_use", "roles", "max_candidates_per_role", "max_total_candidates", "max_excerpt_characters", "max_relations_per_candidate", "view", "trace_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "request",
						Shape: `{"mode":"concern","query":"...","entity_of_concern":"...","known_context":["..."],"intended_use":"...","view":"working","max_candidates_per_role":5,"max_total_candidates":25,"max_excerpt_characters":1200,"max_relations_per_candidate":12} | {"mode":"lookup","identifier":"A.22.CGUS","roles":[...],"view":"working","max_candidates_per_role":5,"max_total_candidates":25,"max_excerpt_characters":1200,"max_relations_per_candidate":12} | {"mode":"inspect","identifier":"A.22.CGUS","roles":[...],"view":"working"}`,
						Note:  "mode=concern requires query and always returns the practical_use_card and toc_row navigation roles only; concern requests containing roles are rejected. mode=lookup and mode=inspect require identifier and allow explicit roles, including preface, pattern bodies, pattern sections, and named pattern_scope blocks. max_excerpt_characters is a strict per-candidate total across excerpt and practical-use cue text; for an exact working lookup the same field bounds its practical-use cues. max_relations_per_candidate also bounds exact working lookup relation projection. Exact inspect remains complete.",
					},
					{
						Field: "publication_view",
						Shape: `{"view":"working|trace|diagnostic","trace_ref":"fpf-query-trace:v1:<snapshot-digest>:<request-digest>:<result-digest>"}`,
						Note:  "view is independent from retrieval mode and defaults to working. MCP replays pass trace_ref only with view=trace or view=diagnostic; the equivalent CLI flag is --replay-ref. The generic haft_query view field is action-specific and is not a global enum because other query actions own different view contracts.",
					},
					{
						Field: "working_response",
						Shape: `{"view":"working","trace_ref":"fpf-query-trace:v1:...","kind":"exact_hit","identifier":"A.22.CGUS","unit":{"unit_id":"...","source_id":"...","source_role":"pattern_body","title":"...","pattern_id":"A.22","publication_status":"...","direct_refs":[...],"direct_refs_truncated":false,"relation_projection":{"relations":[{"kind":"...","target_pattern_id":"..."}],"truncated":false,"omitted_at_least":0},"use_cues":{...},"use_cues_truncated":false}} | {"view":"working","trace_ref":"fpf-query-trace:v1:...","kind":"exact_hit","identifier":"A.22.CGUS","unit":{"unit_id":"...","source_role":"pattern_body","title":"...","body":"complete exact inspect body","direct_refs":[...],"relations":[{"kind":"...","target_pattern_id":"..."}]}} | {"view":"working","trace_ref":"fpf-query-trace:v1:...","kind":"candidate_set","groups":[{"source_role":"toc_row","candidates":[{"source":{"unit_id":"...","source_role":"toc_row","title":"...","excerpt":"...","excerpt_truncated":false,"pattern_id":"A.22","publication_status":"...","use_cues":{...},"relation_projection":{"relations":[{"kind":"...","target_pattern_id":"..."}],"truncated":false,"omitted_at_least":0}}}]}],"truncation":{"applied":false,"budget":{...},"included_candidates":1,"omitted_at_least":0}} | {"view":"working","trace_ref":"fpf-query-trace:v1:...","kind":"abstained","reason":"...","missing_basis":["..."]}`,
						Note:  "Default working responses are closed public carriers. They omit repository paths, line spans, hashes, revisions, repeated provenance, raw match grounds, and producer diagnostics. Exact lookup returns a budgeted identity summary with explicit truncation posture; exact inspect alone retains the complete authoritative body, references, and relation semantics.",
					},
					{
						Field: "trace_response",
						Shape: `{"view":"trace","trace_ref":"fpf-query-trace:v1:...","kind":"exact_hit|candidate_set|abstained","trace":{"source_snapshot":{"index_schema_version":"...","source_revision":"...","readme_document_digest":"...","specification_document_digest":"..."},"provenance":[{"ref":"...","source_path":"...","start_line":1,"end_line":20,"content_hash":"..."}],"unit_bindings":[...],"relation_bindings":[...],"retrieval_evidence_bindings":[...]}}`,
						Note:  "Explicit trace view deduplicates response-wide source identity and provenance while retaining exact reconstruction bindings. It does not expose raw match grounds.",
					},
					{
						Field: "diagnostic_response",
						Shape: `{"view":"diagnostic","trace_ref":"fpf-query-trace:v1:...","kind":"candidate_set","groups":[{"source_role":"toc_row","candidates":[{"source":{...},"match_grounds":[{"tier":"authored_phrase|heading_keyword|role_local_fts","source_field":"...","matched_value":"..."}]}]}],"diagnostic":{"retrieval_mode":"concern|lookup|inspect","producer_ids":["exact_source","source_phrase","authored_phrase","heading_keyword","role_local_fts"]}}`,
						Note:  "Raw canonical retrieval internals, including match grounds and producer identities, are available only through an explicit diagnostic request.",
					},
					{
						Field: "replay_mismatch",
						Shape: `{"view":"trace|diagnostic","kind":"replay_mismatch","code":"source_snapshot_mismatch|query_request_mismatch|query_result_mismatch","expected_trace_ref":"...","current_trace_ref":"...","current_replay_basis_ref":"..."}`,
						Note:  "Snapshot and typed-request mismatches return before retrieval. Result drift returns the same typed family after canonical validation; replay never silently falls through to current source.",
					},
				},
				Notes: []string{
					"CLI modes are `haft fpf query`, `haft fpf lookup`, and `haft fpf inspect`; they map one-to-one to concern, lookup, and inspect.",
					"The omitted or explicit working view is the routine agent-facing result. Request trace only when provenance/replay is current, and diagnostic only when raw retrieval internals are current.",
					"Concern and lookup retain authored phrase, heading/keyword, and role-local FTS inside the canonical result, but producer grounds become public only in diagnostic view. Inspect is exact-only.",
					"FPF Query is read-only retrieval. It does not recommend or select a pattern, prescribe work order, apply a pattern, create an artifact, or provide approval.",
				},
			},
			OutputVolume: []string{
				"working (default): response-budgeted public carrier without internal provenance or raw match grounds",
				"trace: working semantics plus deduplicated replay/provenance bindings",
				"diagnostic: explicit canonical retrieval internals and producer diagnostics",
				"mode=inspect + exact_hit: complete authoritative source body in every view",
			},
			Invariants: append(commonInterfaceInvariants(),
				"Retrieval mode and publication view are independent closed dimensions.",
				"Source unit roles are publication roles, not ranking tiers.",
				"Candidate and group order does not imply applicability, causal order, temporal order, or work order.",
				"Retrieval never returns a selected or recommended pattern.",
				"Unknown exact identifiers abstain instead of fabricating a match.",
				"Working view never exposes source provenance, repository paths, line spans, hashes, revisions, raw match grounds, or producer diagnostics.",
				"Trace replay is opaque and fail-closed on source snapshot, typed request, or canonical result drift.",
			),
		},
		{
			ID:      "memory.validate",
			Purpose: "Validate one strict typed-memory change set without persistence, admission authority, or a fabricated project snapshot.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_memory",
				MCPAction:        "validate",
				MCPCall:          `haft_memory(request={"contract_version":"haft.memory.v2","action":"validate","basis":{"kind":"bundled_candidate_open_world"},"change_set":{...}})`,
				CLIStatus:        "available",
				CLICommand:       "haft memory validate --input-file request.json",
				DiscoveryCommand: "haft interface memory.validate --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"contract_version", "action", "basis", "change_set"},
				FieldShapes: []fieldShape{
					{
						Field: "basis",
						Shape: `{"kind":"bundled_candidate_open_world|project_current"} | {"kind":"exact_project","type_env_digest":"sha256:<64-hex>","graph_revision":17}`,
						Note:  "Selectors are untrusted requests. The server resolves the actual TypeEnv and snapshot; exact_project never falls back to another basis.",
					},
					{
						Field: "change_set",
						Shape: `{"changes":[{"kind":"declare_entity","entity_id":"...","local_ref":"...","context":"...","label":"...","provenance":"..."} | {"kind":"identity_change","change":{"kind":"admit_alias|supersede_alias",...}} | {"kind":"assert_relation","assertion_id":"...","signature_id":"...","context_slice":{"context":"...","standard_pins":[],"environment_selectors":[],"vocabulary_pins":[],"role_set_pins":[],"gamma_time":{"kind":"point","at":"2026-07-16T08:00:00Z"}},"modality":{"kind":"affirms_obtaining|denies_obtaining|obtaining_unknown"},"bindings":[{"slot_kind":"...","fillers":[{"kind":"by_reference","reference":{"kind":"persisted|local",...}} | {"kind":"by_value","value":{"value_kind":"...","value_shape":{"id":"...","digest":"sha256:..."},"codec":{"id":"...","version":"...","specification_digest":"sha256:..."},"input_base64":"...","asserted_digest":{"kind":"none|exact","digest":"sha256:..."}}}]}],"provenance":"..."} | {"kind":"retract_assertion","assertion_id":"...","reason":"...","provenance":"..."}]}`,
						Note:  "This v2 closed union is enforced by the strict byte-level decoder. assert_relation requires an explicit assertion modality and a complete context_slice with explicit gamma_time; neither occurrence nor relation obtaining is inferred. Current FPF IDs are resolved through the server-owned TypeEnv, not inlined as JSON Schema enums.",
					},
					{
						Field: "response",
						Shape: `{"contract_version":"haft.memory.v2","action":"validate","verdict":"valid|invalid|underdetermined","basis":{"requested_kind":"...","resolution_kind":"...","type_env_ref":"...","graph_revision":17},"persistence_disposition":{"mode":"validation_only_no_write","rows_written":0,"authority_granted":false},"diagnostics":[{"posture":"invalid|underdetermined","code":"...","path":"$....","witness":{"kind":"expected_actual|missing_basis",...},"governing_basis":{"kind":"...",...},"repair_candidates":[{"kind":"...","pointer":"...","target":{...},"human_choice_requirement":"..."}]}],"normalized_digest":"sha256:..."}`,
						Note:  "normalized_digest exists only for Valid under a server-resolved project TypeEnv plus immutable project snapshot. The bundled open-world candidate cannot produce project Valid.",
					},
				},
				Notes: []string{
					"Malformed JSON, duplicate fields, unknown fields, unsupported contract versions, and resource-limit violations are boundary errors rather than semantic verdicts.",
					"project_current and exact_project use the checked read-only selected-project runtime. A missing selected ProjectTypeEnvHead returns structured project_basis_unavailable; an old ledger reports that `haft init` is required and never migrates from a read call.",
					"CLI reads the exact bytes from --input-file; use --input-file - for stdin. CLI and MCP use the same decoder, service, and stable JSON presenter.",
				},
			},
			OutputVolume: []string{
				"one stable JSON semantic response",
				"malformed wire input: one boundary error with exact code and JSON path",
			},
			Invariants: append(commonInterfaceInvariants(),
				"Validation writes zero project rows, grants no authority, and exposes no AdmissionBatch.",
				"The bundled candidate is inactive and open-world; it never fabricates a project snapshot, graph revision, active project TypeEnv, or Valid verdict.",
				"Retrieval, schema visibility, a normalized digest, and validation are not admission, approval, evidence truth, or performed Work.",
			),
		},
		memoryAdmissionInterfaceCapability(),
		memoryBackfillInterfaceCapability(),
		memoryResolveInterfaceCapability(),
		memoryNeighborhoodInterfaceCapability(),
		memoryRecallInterfaceCapability(),
		{
			ID:      "query.explore",
			Purpose: "Explore one exact code symbol or a bounded concern through one fused code-and-reasoning result.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "explore",
				MCPCall:          `haft_query(action="explore", symbol="ExistingSymbol", view="working")`,
				CLIStatus:        "available",
				CLICommand:       `haft graph explore --symbol ExistingSymbol --view working --json`,
				DiscoveryCommand: "haft interface query.explore --json",
			},
			InputContract: interfaceContract{
				OptionalFields: []string{"symbol", "query", "file", "line", "anchor_id", "max_candidates", "view", "trace_ref"},
				FieldShapes: []fieldShape{
					{
						Field: "seed",
						Shape: `{"symbol":"ExistingSymbol","file":"optional/path.go","line":17} | {"query":"where is the index epoch published?","max_candidates":12}`,
						Note:  "Exactly one seed shape is accepted. symbol preserves exact and multi-symbol bag behavior; query returns an advisory candidate set and never auto-selects identity. anchor_id resolves to current symbol coordinates before the same exact execution path.",
					},
					{
						Field: "view",
						Shape: `"working" | "trace" | "diagnostic"`,
						Note:  "working is the default bounded projection and omits retrieval and edge provenance. trace adds bounded provenance plus the exact replay basis. diagnostic adds retrieval, resolution, and traversal internals. The vocabulary is action-scoped rather than a global haft_query enum.",
					},
					{
						Field: "trace_ref",
						Shape: `"code-explore-trace:v1:<index-digest>:<request-digest>:<result-digest>"`,
						Note:  "Opaque replay coordinate accepted only by trace or diagnostic view. Index, typed request, or canonical result drift produces a typed replay_mismatch instead of silently replaying a different result.",
					},
					{
						Field: "working_response",
						Shape: `{"contract_version":"haft.code_explore.v1","view":"working","kind":"resolved|candidate_set|disconnected|unresolved|incomplete|unavailable","trace_ref":"...","request_basis":{...},"index_coverage":{...},"seed_resolution":{...},"traversal_outcome":{...},"candidates":[...],"source_hops":[...],"reasoning_context":[...],"source":{"available":true,"truncated":false}}`,
						Note:  "Concern output exposes at most five candidates, no raw BM25/PPR vectors, and no retrieval or per-edge provenance. Ranking is advisory and identity_auto_selected is always false. Truncated source or incomplete traversal names a limiting reason and return view.",
					},
				},
				Notes: []string{
					"Both symbol and query are rejected; neither is rejected; blank or oversized concern strings fail before search.",
					"CandidateSet and unresolved outcomes cannot contain a fabricated traversal path.",
					"CLI and MCP call the same canonical application, pure projector, and compact JSON encoder.",
				},
			},
			OutputVolume: []string{
				"working: at most five concern candidates and at most 12000 encoded bytes",
				"trace: working semantics plus bounded retrieval/edge provenance and exact replay basis",
				"diagnostic: trace result plus bounded retrieval, resolution, and traversal internals",
			},
			Invariants: append(commonInterfaceInvariants(),
				"Retrieval rank and graph proximity are not identity selection, applicability, recommendation, or authority.",
				"Static absence is stated only under an exact complete index basis; bounded or unavailable traversal remains incomplete or unavailable.",
				"Working projection never exposes raw BM25 or PPR vectors.",
				"Working projection omits retrieval origin lanes, artifact support, edge provenance, and full exclusion details; trace recovers them under the same replay basis.",
			),
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
				OptionalFields: []string{"file", "files", "symbol", "anchor_id", "line", "context", "lane", "limit", "full"},
				Notes: []string{
					"Exactly one of file or files is required. files is a deduplicated file-level batch capped at 8 targets; symbol, anchor_id, line, and full=true remain single-target only.",
					"Default output is lane=index: target, governed/blind status, lane counts, hard risks, and exact next calls.",
					"Valid lanes: index, symbols, decisions, invariants, notes, problems, portfolios, all.",
					"Prefer one typed lane at a time; lane=all restores the compact all-lane view; full=true is audit/backcompat dump.",
				},
			},
			OutputVolume: []string{
				"default: lane=index under normal planning budget",
				"typed lanes: capped by limit, default 20",
				"files batch: at most 8 targets, default per-target limit 5, one navigation footer",
				"lane=all: compact all-lane recovery view",
				"full=true: complete audit dump",
			},
			Invariants: commonInterfaceInvariants(),
		},
		{
			ID:      "query.node",
			Purpose: "Resolve one code symbol or SymbolAnchor without coercing FPF, Haft-artifact, or typed-memory identifiers into the code namespace.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "node",
				MCPCall:          `haft_query(action="node", symbol="ExistingSymbol")`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.node --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"symbol", "anchor_id", "file", "line"},
				FieldShapes: []fieldShape{
					{
						Field: "exact_identifier_namespaces",
						Shape: `{"fpf_source":{"identifiers":"PatternID|SourceID|UnitID","call":"haft_query(action=fpf, mode=lookup|inspect, identifier=<id>)"},"haft_artifact":{"identifiers":"canonical ProblemCard|SolutionPortfolio|DecisionRecord|WorkCommission|MethodRun|EvidencePack|RefreshReport|Note ID","call":"haft_query(action=related, artifact_ref=<id>)"},"code":{"identifiers":"code symbol|SymbolAnchor","call":"haft_query(action=node, symbol=<name>)|haft_query(action=node, anchor_id=<anchor>)"},"typed_memory":{"identifiers":"EntityID|EntityAlias","call":"haft_query(action=memory, memory_request={mode:resolve,...})|haft memory resolve --input-file request.json"}}`,
						Note:  "Classify the identifier before choosing a field. Typed-memory identities use the exact read surface and remain distinct from code and Haft-artifact namespaces.",
					},
					{
						Field: "wrong_identifier_namespace",
						Shape: `{"code":"wrong_identifier_namespace","received_namespace":"haft_artifact_id","expected_namespace":"code_symbol","same_call_retryable":false,"recovery_call":{"tool":"haft_query","arguments":{"action":"related","artifact_ref":"<id>"}}}`,
						Note:  "The recovery call is executable guidance. Do not retry the same code action with another identifier from the rejected namespace.",
					},
				},
				Notes: []string{
					"Supply a code symbol in symbol or an exact SymbolAnchor in anchor_id; file and line only disambiguate the code lookup.",
					"FPF source identifiers use query.fpf. Canonical Haft artifact IDs use query.related. Neither belongs in symbol.",
					"An EntityID or EntityAlias remains a typed-memory identifier. Resolve it with the exact typed-memory read surface instead of sending it to node or related.",
				},
			},
			OutputVolume: []string{"one exact code-symbol view or one structured namespace error"},
			Invariants: append(commonInterfaceInvariants(),
				"The symbol field accepts only the code-symbol namespace.",
				"A canonical Haft artifact ID fails before code-index access with wrong_identifier_namespace, same_call_retryable=false, and one exact related(artifact_ref) recovery call.",
				"FPF source identifiers and typed-memory EntityIDs or aliases are never coerced into code symbols.",
				"The typed-memory exact-read action does not authorize fallback into another identifier namespace.",
			),
		},
		{
			ID:      "query.related",
			Purpose: "Recover one full artifact carrier by exact ref, including explicit ProblemCard semantic views when available.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "related",
				MCPCall:          `haft_query(action="related", artifact_ref="prob-...")`,
				CLIStatus:        "mcp_projection",
				DiscoveryCommand: "haft interface query.related --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"artifact_ref", "ref", "artifact_id", "file"},
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
					{
						Field: "decision.evidence.wlnk",
						Shape: `{"summary":"...","r_eff":0.8,"f_eff":7,"formality_scale_id":"fpf-2026-f0-f9|haft-legacy-f0-f3|unversioned-formality","formality_bridge_loss":"none|legacy-scale-has-fewer-buckets|source-scale-not-declared","weakest_cl":3,"authority_boundary":"evidence/formality diagnostics are not approval, gate passage, claim truth, global truth, or publication"}`,
						Note:  "DecisionRecord audit/evidence views name formality scale and bridge/loss; WLNK diagnostics do not create approval, GateDecision, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Provide artifact_ref (preferred), ref/artifact_id (aliases), or file; exact-artifact and file-related modes are distinct read-only projections.",
					"artifact_ref is canonical for exact single-artifact recovery; ref and artifact_id remain backward-compatible aliases.",
					"related(file=\"...\") remains the file-to-artifact discovery mode.",
					"For ProblemCard refs, the response preserves legacy keys and adds semantic + views.",
					"For DecisionRecord refs, the audit/evidence projection names WLNK formality scale and bridge/loss rather than showing a bare F ordinal.",
					"Persisted spec_binding_preflight is a binding-time receipt, not a current spec-health evaluation.",
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
				"DecisionRecord WLNK/formality projection remains diagnostics, not approval, GateDecision, claim truth, global truth, or publication.",
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
			ID:      "baseline.audit",
			Purpose: "Classify overloaded baseline terminology into spec approval, pre-work reference, verified-state snapshot, comparison, carrier, and legacy ambiguous categories.",
			CurrentExecution: interfaceExecution{
				CLIStatus:        "available",
				CLICommand:       "haft baseline audit --json",
				DiscoveryCommand: "haft interface baseline.audit --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				OptionalFields: []string{"json", "limit"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"kind":"haft_baseline_term_audit","schema_version":1,"authority":"read_only_term_audit_not_baseline_mutation","mutation_boundary":["read_only_term_audit","does_not_mutate_baselines_decisions_evidence_or_carriers","does_not_create_approval_gate_decision_claim_truth_global_truth_or_publication"],"summary":{"spec_section_approval_baseline":1,"pre_work_reference_snapshot":1,"verified_state_snapshot":1,"comparison_or_benchmark_baseline":1,"legacy_ambiguous_baseline":0},"diagnostics":[{"level":"warn","code":"legacy_ambiguous_baseline_terms","category":"legacy_ambiguous_baseline","next_action":"rename the usage to a typed baseline concept"}],"projection":{"view":"compact","omitted_findings":10,"full_audit_command":"haft baseline audit --json"}}`,
						Note:  "The audit classifies terminology only; diagnostics are review input and never permission to mutate baselines, decisions, evidence, gates, claims, global truth, or publication.",
					},
				},
				Notes: []string{
					"Use this when plan or code uses the word baseline without saying which kind of snapshot/currentness/comparison it means.",
					"The audit skips Open-Sleigh, node_modules, vendor, ignored planning carriers, and build output.",
					"`--limit` caps emitted findings without changing summary counts.",
					"Legacy ambiguous findings require a later typed rename or local wording clarification; the audit itself is read-only.",
				},
			},
			OutputVolume: []string{"default text summary; --json full term audit; --limit N compact JSON projection"},
			Invariants: append(commonInterfaceInvariants(),
				"Baseline audit is read-only.",
				"Baseline audit does not mutate SpecSectionApprovalBaseline rows, DecisionRecord baselines, evidence, carriers, gates, claims, global truth, or publication.",
				"Default status must not inline baseline audit findings.",
			),
		},
		{
			ID:      "spec.draft_contract",
			Purpose: "Publish the canonical profile-independent phases, fields, values, checks, and validation continuation needed to author specification carriers through public Haft surfaces.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_spec_section",
				MCPAction:        "draft_contract",
				MCPCall:          `haft_spec_section(action="draft_contract")`,
				CLIStatus:        "available",
				CLICommand:       "haft spec draft-contract --json",
				DiscoveryCommand: "haft interface spec.draft_contract --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"contract_version":"haft.spec-draft-contract/v1","authority":"read_only_design_time_contract_not_applicability_approval_or_evidence","applicability_effect":"none_contract_does_not_establish_profile_applicability","lifecycle_effect":"none_contract_does_not_activate_approve_rebaseline_or_reopen","phases":[{"phase_id":"target.boundary.draft","depends_on":[],"document_kind":"target-system","section_kind":"target.boundary","expected_fields":["boundary_perspectives"],"checks":["require_boundary_perspectives:min=4"]}],"spec_section":{"fence_info":"yaml spec-section","required_fields":["id","kind","statement_type","claim_layer","owner","status"]},"term_map":{"fence_info":"yaml term-map","container_field":"entries"},"validation_call":{"tool":"haft_query","arguments":{"action":"spec_validate"}}}`,
						Note:  "The response is shipped product grammar, not project applicability, lifecycle state, approval, evidence, or a profile mutation.",
					},
				},
				Notes: []string{
					"Use this contract when lifecycle applicability is underdetermined but draft-carrier work can continue safely.",
					"Follow validation_call for the canonical carrier-validation continuation.",
				},
			},
			OutputVolume: []string{"one bounded contract containing all registered phases and canonical carrier shapes"},
			Invariants: append(commonInterfaceInvariants(),
				"Draft contract is profile-independent read-only product knowledge.",
				"Draft contract does not establish applicability, activate or approve sections, mutate profiles, create evidence, or pass gates.",
				"Validation continuation is the existing query.spec_validate surface; no parallel validator is introduced.",
			),
		},
		{
			ID:      "query.spec_validate",
			Purpose: "Validate authored draft and active spec carriers structurally and semantically without profile-applicability filtering or lifecycle admission.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "spec_validate",
				MCPCall:          `haft_query(action="spec_validate")`,
				CLIStatus:        "available",
				CLICommand:       "haft spec validate --json",
				DiscoveryCommand: "haft interface query.spec_validate --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"validation_kind":"spec_carrier_validation","authority":"read_only_carrier_validation","source_basis":"authored_carriers_without_profile_applicability_filter","authority_boundary":{"applicability":"not_applicability_determination_or_admission","activation":"not_section_activation","approval":"not_approval_or_baseline","evidence":"not_evidence","stronger_use":"not_spec_use_admission","lifecycle_effect":"none_read_only","carrier_mutation":"none_read_only"},"summary":{"total_sections":10,"draft_sections":10,"active_sections":0,"checked_sections":10,"structural_findings":0,"semantic_findings":2,"lifecycle_observations":2},"structural":{"level":"L0/L1/L1.5","findings":[]},"semantic":{"profile":{"id":"spec_draft_semantic_review_v1"},"sections":[{"section_id":"TS.boundary.001","status":"draft"}]},"lifecycle_observations":[{"code":"spec_carrier_no_active_sections"}]}`,
						Note:  "Lifecycle observations remain visible but do not block structural or advisory semantic validation of draft carriers.",
					},
				},
				Notes: []string{
					"Validation reads authored target-system, software-system, and term-map carriers without profile-applicability filtering.",
					"Draft sections remain draft; validation does not activate, approve, baseline, admit applicability, create evidence, or authorize stronger use.",
					"Structural findings control the CLI failure status; semantic findings remain advisory review input.",
					"Validation does not establish compatibility with a newer FPF source revision or source-baseline currentness.",
				},
			},
			OutputVolume: []string{"default: compact validation summary; --json one structural plus advisory semantic carrier-validation report"},
			Invariants: append(commonInterfaceInvariants(),
				"Carrier validation is independent of canonical profile applicability.",
				"Carrier validation does not activate, approve, baseline, admit, mutate, create evidence, or authorize stronger use.",
				"Draft lifecycle status and validation findings remain separate readings.",
				"Existing spec check and active-only spec review semantics remain unchanged.",
			),
		},
		{
			ID:      "query.spec_review",
			Purpose: "Build a read-only advisory semantic review packet over current typed SpecSection editions; it does not compare their meaning with a newer FPF source.",
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
						Shape: `{"authority":"advisory_only","authority_boundary":{"evidence":"not_evidence","approval":"not_approval","rebaseline":"not_rebaseline","gate_decision":"not_gate_decision","spec_use_admission":"not_spec_use_admission","claim_truth":"not_claim_truth","global_truth":"not_global_truth","publication":"not_publication"},"review_kind":"spec_semantic","profile":{"id":"spec_semantic_review_v2","model_disposition":{...}},"summary":{"checked_sections":10,"explicit_claims":0,"blocked_for_stronger_use_findings":0},"sections":[{"system_frame":{...},"claim_register":{...},"state_reading":{...},"findings":[{"rule_id":"...","category":"claim_posture|publication_boundary|frame|unknown_abstain"}]}]}`,
						Note:  "Findings are review inputs and never evidence, approval, rebaseline, GateDecision, SpecUseAdmission, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Spec semantic review is advisory and read-only.",
					"The review evaluates the existing claim register and semantic profile only; a green result does not establish compatibility with a newer FPF source or FPF source-baseline currentness.",
					"It can abstain/block stronger use when the semantic profile lacks enough structure.",
					"Claim counts are profile/read findings until first-class ClaimRegister exists.",
				},
			},
			OutputVolume: []string{"default: one JSON spec semantic review packet; compact text via `haft spec review`"},
			Invariants: append(commonInterfaceInvariants(),
				"Spec review findings are not evidence, approval, rebaseline, GateDecision, SpecUseAdmission, claim truth, global truth, or publication.",
				"Spec semantic review and FPF source-currentness assessment are separate results.",
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
			ID:      "query.spec_binding_preflight",
			Purpose: "Classify the relationship between a DecisionRecord draft and the current ProjectSpecificationSet before binding.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "spec_binding_preflight",
				MCPCall:          `haft_query(action="spec_binding_preflight", decision_draft={"selected_title":"...","section_refs":["TS.role.001"],"affected_files":["internal/x.go"]})`,
				CLIStatus:        "MCP/read-only query only",
				DiscoveryCommand: "haft interface query.spec_binding_preflight --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"decision_draft"},
				OptionalFields: []string{},
				FieldShapes: []fieldShape{
					{
						Field: "decision_draft",
						Shape: `object: selected_title, mode, load_bearing_level, section_refs, affected_files, decision_subject_ref, binding_fallback_reason, declared_relation, and related draft context fields`,
						Note:  "Raw section_refs stay optional; the preflight classifies the relation instead of making the JSON field globally required.",
					},
					{
						Field: "response",
						Shape: `{"schema_version":1,"record_kind":"spec_binding_preflight","authority":"read_only_spec_binding_preflight","project_spec_state":"ready|partial|no_specs|no_active_sections","decision_mode":"tactical|standard|deep","load_bearing_level":"low|normal|load_bearing","state":"no_specs|no_active_sections|provided_refs_valid|invalid_refs|bound_existing|ambiguous|draft_section_needed|out_of_spec|conflict","selected_section_refs":["TS.boundary.001"],"candidate_section_refs":[{"section_ref":"TS.boundary.001","relation":"governs|context_only","confidence":"high|medium|low","basis":["matched target_refs"]}],"operator_action_required":"none|choose_section|draft_section|record_rationale|reopen_problem","status_debt":{"severity":"none|low|high","message":"..."}}`,
						Note:  "The response is admission/check metadata only; it is not approval, baseline, evidence, GateDecision, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Use before binding through h-decide in spec-enabled projects; if it was not run earlier, the host-routed decision path should treat the result as a late preflight.",
					"Provided refs fail closed when unknown, inactive, superseded, or draft-only.",
					"no_specs and no_active_sections allow ordinary decisions with explicit unbound status; they do not make specs required globally.",
					"bound_existing may auto-fill only for a single high-confidence existing active section match.",
					"ambiguous requires operator choice; conflict requires reopening/spec-changing path; out_of_spec requires explicit rationale and retained debt.",
				},
			},
			OutputVolume: []string{"default: one JSON SpecBindingPreflightResult"},
			Invariants: append(commonInterfaceInvariants(),
				"SpecBindingPreflight is read-only and cannot approve/rebaseline specs or create a DecisionRecord.",
				"Relation is required for spec-enabled load-bearing decisions; section_refs remains optional at raw schema level.",
				"Draft SpecSections may be proposed by follow-up workflows but never counted as active section_refs.",
				"Default status must not inline preflight candidates; status/overseer should surface unresolved binding debt compactly.",
			),
		},
		{
			ID:      "query.spec_trace",
			Purpose: "Trace one SpecSection through current decisions and code drill-down commands.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "spec_trace",
				MCPCall:          `haft_query(action="spec_trace", section_id="TS.boundary.001")`,
				CLIStatus:        "MCP/read-only query only",
				DiscoveryCommand: "haft interface query.spec_trace --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"section_id"},
				FieldShapes: []fieldShape{
					{
						Field: "response",
						Shape: `{"schema_version":1,"record_kind":"spec_trace","authority":"read_only_spec_trace_diagnostic","section_id":"TS.boundary.001","section":{"id":"TS.boundary.001","status":"active"},"baseline_currentness":{"status":"current|drifted|missing"},"current_authority":{"status":"clear|overlap_needs_review|conflict_requires_operator","explicit_decision_refs":[],"derived_from_section_refs":["dec-..."]},"code_bindings":[{"decision_ref":"dec-...","affected_files":["internal/x.go"],"code_context_drilldown":["haft_query(action=\"code_context\", file=\"internal/x.go\", lane=\"decisions\")"]}],"missing_links":[]}`,
						Note:  "Diagnostic trace only; it composes spec_use/governing_set/code_context pointers and does not create authority, evidence, approval, gate passage, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Use when the operator asks whether SpecSection -> DecisionRecord -> code_context linking works end to end.",
					"Explicit governance targets and derived section_refs are separated in current_authority.",
					"Missing links are diagnostics, not proof that no dynamic/reflection/callback code relation exists.",
				},
			},
			OutputVolume: []string{"default: one JSON SpecTrace record"},
			Invariants: append(commonInterfaceInvariants(),
				"SpecTrace is read-only and diagnostic.",
				"SpecTrace is not the authority mechanism; current-authority semantics remain in governing_set/spec_use.",
			),
		},
		{
			ID:      "query.spec_fit_probe",
			Purpose: "Classify problem or variant compatibility with active SpecSections before frame/explore/compare.",
			CurrentExecution: interfaceExecution{
				MCPTool:          "haft_query",
				MCPAction:        "spec_fit_probe",
				MCPCall:          `haft_query(action="spec_fit_probe", probe={"problem_signal":"...","scope":"...","variants":[{"id":"V1","title":"..."}]})`,
				CLIStatus:        "MCP/read-only query only",
				DiscoveryCommand: "haft interface query.spec_fit_probe --json",
			},
			InputContract: interfaceContract{
				RequiredFields: []string{"probe"},
				OptionalFields: []string{"variants"},
				FieldShapes: []fieldShape{
					{
						Field: "probe",
						Shape: `{"problem_signal":"...","scope":"...","section_refs":["TS.boundary.001"],"affected_files":["internal/x.go"],"target_refs":["symbol:internal/x.go::Run"],"conflict_refs":[],"declared_relation":"relates_existing"}`,
						Note:  "Read-only early compatibility probe; it does not create ProblemCards or SolutionPortfolios.",
					},
					{
						Field: "variants[]",
						Shape: `{"id":"V1","title":"fits existing","description":"...","section_refs":["TS.boundary.001"],"affected_files":["internal/x.go"],"target_refs":["symbol:internal/x.go::Run"],"conflict_refs":[],"declared_relation":"conflict"}`,
						Note:  "Optional candidate variants to classify before h-explore/h-compare; advisory only.",
					},
					{
						Field: "response",
						Shape: `{"schema_version":1,"record_kind":"spec_fit_probe","authority":"read_only_spec_fit_probe","state":"relates_existing|spec_gap|conflict|outside_current_spec|no_signal","candidate_section_refs":["TS.boundary.001"],"conflict_refs":[],"next_expected_action":"ordinary_explore|draft_section|explore_spec_delta","variant_spec_fit":[{"variant_ref":"V1","state":"relates_existing","section_refs":["TS.boundary.001"],"expected_action":"ordinary_explore"}]}`,
						Note:  "This is an advisory early-warning surface; it is not approval, baseline, evidence, GateDecision, claim truth, global truth, or publication.",
					},
				},
				Notes: []string{
					"Use before h-frame/h-explore/h-compare when a project has active specs and the work may be spec-bearing.",
					"spec_gap points to h-spec/draft-section work before a decision is bound.",
					"conflict and outside_current_spec should become comparison dimensions or explicit spec-delta variants, not hidden late h-decide surprises.",
				},
			},
			OutputVolume: []string{"default: one JSON SpecFitProbeResult"},
			Invariants: append(commonInterfaceInvariants(),
				"SpecFitProbe is advisory and read-only; it cannot approve specs, create artifacts, or bind decisions.",
				"Early probe results may inform frame/explore/compare but do not satisfy decision-time spec_binding_preflight by themselves.",
			),
		},
		{
			ID:      "spec.apply_change",
			Purpose: "Preview one reviewed typed SpecSection carrier change, present any required lifecycle act through a self-contained Human Gate Brief, or explicitly apply it into the SQL edition store.",
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
					humanGateBriefRequirement,
					"Classifying or applying a carrier delta does not establish that its terms match the current FPF source; h-spec recovers source meaning separately before stronger source-dependent use.",
					"Only typed fenced yaml spec-section fields participate; surrounding Markdown prose is never SQL truth.",
					"Recognized scalar, relationship, or mixed updates may write a markdown_sync_back SQL edition only through explicit apply-change.",
					"Carrier-only changes are no-op; unknown/high-risk changes fail closed.",
					"The command never approves, rebaselines, reopens, creates evidence, or mutates SpecSectionApprovalBaseline rows.",
				},
			},
			OutputVolume: []string{"default text summary; --json exact result; --dry-run reports planned_edition without edition write"},
			Invariants: append(commonInterfaceInvariants(),
				"SQL edition store remains the source of truth.",
				"SQL edition currentness and FPF source compatibility are separate postures.",
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
						Shape: `{"schema_version":1,"authority":"report_only_selection_draft_not_operator_approval","operator_approved":false,"apply_authority_required":"operator_approved_reconciliation_selection","summary":{"reviewed_groups":69,"plan_scope_enrichment_candidates":7,"reviewable_scope_enrichment_candidates":6,"scope_enrichment_candidates":6,"operator_approval_candidates":6,"review_required_candidates":6,"apply_ready_candidates":0,"emitted_candidates":5,"omitted_candidates":1,"selected_candidates":0,"template_items":5},"omitted_items":1,"full_audit_command":"haft decision reconcile selection-draft --json --full","current_metrics":{"schema_version":1,"authority":"read_only_reconciliation_metrics_not_binding_authority","reconciliation":{"reviewed_decisions":69,"groups":69,"scope_enrichment_candidates":7},"governing_set":{"current_decisions":69,"governing_sets":231,"fallback_target_sets":7,"scope_enrichment_sets":13,"conflict_sets":0,"terminal_history_refs":13},"drift_events":{"unique_events":246,"needs_binding_resolution_events":36,"max_fanout":29},"before_after_use":{"before_command":"haft decision reconcile metrics --json","apply_command":"haft decision reconcile apply SELECTION.json --json","after_command":"haft decision reconcile metrics --json","required_authority":"operator_approved_reconciliation_selection","mutation_boundary":["metrics capture is read-only"]}},"items":[{"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_ref":"dec-...","decision_carrier_hint":".haft/decisions/dec-....md","candidate_posture":"precise_target_prefilled_subject_needed|needs_subject_and_target_review|whole_file_fallback_target_repair_needed","confidence":"medium|low|not_applicable","decision_subject_ref_suggestions":["subject:bounded-context:title"],"review_commands":["sed -n '1,220p' .haft/decisions/dec-....md","haft decision reconcile selection-draft --decision-ref dec-... --json"],"suggested_review_action":"review decision carrier and fill exact decision_subject_ref","blocking_questions":["What exact object does this decision govern now?"],"approval_readiness":{"state":"operator_review_required","apply_ready":false,"not_apply_ready_reasons":["selection document still contains missing or placeholder fields"],"placeholder_fields":["operator_approval_ref","items[].decision_subject_ref","items[].reason"],"required_operator_checks":["confirm the exact decision_subject_ref from the decision carrier and current governing scope"],"selection_fields_to_confirm":["operator_approval_ref","items[].decision_subject_ref","items[].governance_targets","items[].remove_whole_file_fallback_targets","items[].reason"],"authority_boundary":["approval_readiness does not create operator approval"]},"selection_template":"{...}","proposed_selection":{"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-..."],"decision_subject_ref":"TODO_exact_decision_subject_ref","governance_targets":[{"kind":"symbol","ref":"symbol:..."}],"remove_whole_file_fallback_targets":["whole_file_fallback:.haft/solutions/sol-old.md"],"reason":"TODO_operator_reviewed_scope_enrichment_reason"}}],"selection_document_template":{"schema_version":1,"authority":"operator_approved_reconciliation_selection","operator_approval_ref":"","items":[{"operation":"enrich_scope","reviewed_group_id":"decision-reconcile-...","decision_refs":["dec-..."],"decision_subject_ref":"TODO_exact_decision_subject_ref","governance_targets":[{"kind":"symbol","ref":"symbol:..."}],"remove_whole_file_fallback_targets":["whole_file_fallback:.haft/solutions/sol-old.md"],"reason":"TODO_operator_reviewed_scope_enrichment_reason"}]},"selection_document_template_boundary":["selection_document_template items are emitted review candidates, not selected candidates","selection_document_template is not operator approval","operator_approval_ref must be filled only after explicit operator approval","TODO placeholders are rejected by selection-review and apply","selection-review and apply revalidate against the current reconciliation plan"]}`,
						Note:  "Selection drafts are read-only review aids. Default output is bounded for review; use --limit N for a smaller compact slice or --full for the complete audit list. plan_scope_enrichment_candidates names the broader reconciliation-plan count; reviewable_scope_enrichment_candidates and the backwards-compatible scope_enrichment_candidates field name the draftable enrich_scope candidates. review_required_candidates and apply_ready_candidates distinguish review work from candidates that can pass apply validation; template_items counts convenience-template rows and is not a selected/apply-ready count. current_metrics is read-only before/after context copied from the reconciliation metrics surface; it is not copied into selection_document_template and does not create approval, evidence truth, gate passage, or apply authority. decision_subject_ref_suggestions, decision_carrier_hint, and review_commands are deterministic review hints only, discovery aids, not approval or apply authority, and are not copied into proposed_selection or apply templates. remove_whole_file_fallback_targets is a review hint copied only when it names existing top-level whole-file fallback binding_targets; the operator still must confirm it in the selection document. approval_readiness names missing placeholders, required operator checks, and fields to confirm; it is advisory review metadata, not approval. --write-review-packet review.json writes the bounded report-only draft with review hints for operator inspection; it is not an approval document. --write-template selection.json writes only the bounded selection_document_template to a file but does not create approval and leaves operator_approval_ref empty. selection_document_template_boundary states that template items are emitted review candidates, not selected candidates. proposed_selection is a structured copy of the per-item template, and together with selection_document_template removes JSON assembly work, but both remain not apply-ready until operator_approval_ref is filled; TODO_... placeholders are rejected and must be replaced before the selection can become apply-ready. apply still validates authority/current plan.",
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
					"`selection_document_template_boundary` states that template items are emitted review candidates, not selected candidates.",
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
						Shape: `{"schema_version":1,"authority":"read_only_current_authority_frontier","view":"compact","snapshot":{"generated_at":"RFC3339","source":"artifact_store_decision_records","projection":"refreshable_current_governing_frontier","authority_boundary":"derived_read_only_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication","current_status_policy":["active","refresh_due"],"terminal_status_policy":["superseded","deprecated"],"terminal_history_policy":"terminal decisions stay searchable history and are excluded from current authority","filter_applied":true},"filter":{"query":"symbol:...","subject_ref":"subject:...","target_ref":"symbol:..."},"summary":{"current_decisions":7,"governing_sets":5,"conflict_sets":1,"overlap_review_sets":1,"fallback_target_sets":1,"scope_enrichment_sets":2,"terminal_history_refs":3},"authority_frontier":{"authority_boundary":"current_decision_refs_are_governing_authority_terminal_history_refs_are_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication","current_status_policy":["active","refresh_due"],"terminal_status_policy":["superseded","deprecated"],"current_decision_refs":["dec-current"],"terminal_history_refs":["dec-old"],"terminal_history_policy":"terminal decisions stay searchable history and are excluded from current authority"},"compact_sets":[{"set_id":"governing-set-...","subject_ref":"...","bounded_context":"...","target_ref":"...","answer_paths":[{"target_kind":"claim|spec_section|api_contract|invariant|symbol|whole_file_fallback|file_fallback|unscoped_decision","target_ref":"...","cli":"haft decision governing-set --target-ref ... --json","mcp_call":"haft_query(action=\"governing_set\", source_refs=[...])","exact_record_needed":"...","authority_boundary":"answer_path_is_read_only_not_evidence_approval_gate_decision_claim_truth_global_truth_or_publication"}],"target_resolution":"explicit_governance_or_watch_target|derived_from_section_refs|whole_file_fallback_requires_scope_enrichment","whole_file_fallback_targets":[...],"scope_repair_hints":["replace whole-file fallback ..."],"posture":"single_current_authority|overlap_needs_review|conflict_requires_operator","current_decision_refs":[...],"terminal_history_refs":[...],"current_decision_count":7,"operator_required":true}],"omitted_sets":42,"full_audit_command":"haft_query(action=\"governing_set\", full=true)"}`,
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
					"The projection is derived from active/refresh_due decisions, effective governance/drift targets, and canonical spec_section:<id> targets from section_refs.",
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

func memoryAdmissionInterfaceCapability() interfaceCapability {
	return interfaceCapability{
		ID:      "memory.admit",
		Purpose: "Validate and atomically admit one exact non-binding semantic change set into the selected project TypeEnv and graph snapshot.",
		CurrentExecution: interfaceExecution{
			MCPTool:          "haft_memory",
			MCPAction:        "admit",
			MCPCall:          `haft_memory(request={"contract_version":"haft.memory.v2","action":"admit","basis":{"kind":"exact_project","type_env_digest":"sha256:...","graph_revision":17},"authority_class":"non_binding_semantic_assertion","idempotency_key":"...","request_provenance_ref":"...","change_set":{...}})`,
			CLIStatus:        "available",
			CLICommand:       "haft memory admit --input-file request.json",
			DiscoveryCommand: "haft interface memory.admit --json",
		},
		InputContract: interfaceContract{
			RequiredFields: []string{
				"contract_version",
				"action",
				"basis",
				"authority_class",
				"idempotency_key",
				"request_provenance_ref",
				"change_set",
			},
			FieldShapes: []fieldShape{
				{
					Field: "basis",
					Shape: `{"kind":"exact_project","type_env_digest":"sha256:<64-hex>","graph_revision":17}`,
					Note:  "Admission accepts only the exact selected project basis. project_current, bundled candidates, stale revisions, and foreign TypeEnv digests fail closed.",
				},
				{
					Field: "authority_class",
					Shape: `"non_binding_semantic_assertion"`,
					Note:  "This class can add typed project-memory assertions. It cannot bind a DecisionRecord, approve a SpecSection, commission Work, or alter ProjectTypeEnvHead.",
				},
				{
					Field: "response",
					Shape: `{"contract_version":"haft.memory.v2","action":"admit","result":"not_admitted|committed|commit_outcome_unknown","authority_class":"non_binding_semantic_assertion","validation":{...},"persistence_disposition":{"mode":"...","rows_written":0,"authority_granted":false},"receipt":{"event_ref":"...","commit_ref":"...","graph_revision":18,"result_digest":"sha256:..."},"retry":{"kind":"same_request_replay","contract_version":"haft.memory.v2","project_id":"...","idempotency_key":"...","type_env_digest":"sha256:...","graph_revision":17,"request_identity_digest":"sha256:..."}}`,
					Note:  "commit_outcome_unknown is not success or rollback. Replay the unchanged request with the returned exact coordinates; do not invent a new key.",
				},
			},
			Notes: []string{
				"Fresh admission is haft.memory.v2 only. Its closed MemoryChangeSet uses assert_relation with an explicit affirms_obtaining, denies_obtaining, or obtaining_unknown modality; instantiate_relation belongs only to frozen v1 history.",
				"Historical haft.memory.v1 requests are exact replay-only at the storage boundary and are not selectable from the current MCP or CLI admission schema.",
				"The strict decoder owns the closed MemoryChangeSet union and rejects duplicate, unknown, cross-variant, oversized, or malformed fields before semantic work.",
				"The server re-resolves and revalidates project identity, selected TypeEnv, graph revision, references, codecs, context slice, constraints, and idempotency inside the transaction.",
				"CLI and MCP expose the same semantic admission service. The full MCP action is advertised only when the bound project has a current selected ProjectTypeEnvHead.",
			},
		},
		OutputVolume: []string{
			"one stable admission result and exact replay coordinates when outcome is unknown",
		},
		Invariants: append(
			commonInterfaceInvariants(),
			"Admission is non-binding semantic persistence, never decision, commission, specification approval, evidence truth, or performed Work.",
			"Only a server-sealed Valid candidate can commit; Invalid and Underdetermined write zero rows.",
			"One idempotency key denotes one exact request identity; replay cannot change project, TypeEnv, graph revision, provenance, or candidate.",
		),
	}
}

func memoryBackfillInterfaceCapability() interfaceCapability {
	return interfaceCapability{
		ID:      "memory.backfill",
		Purpose: "Dry-run or apply source-owned typed projections for an explicit inventory of already persisted Haft records.",
		CurrentExecution: interfaceExecution{
			CLIStatus:        "available_input_file",
			CLICommand:       "haft memory backfill --input-file request.json",
			DiscoveryCommand: "haft interface memory.backfill --json",
		},
		InputContract: interfaceContract{
			RequiredFields: []string{
				"contract_version",
				"mode",
				"request_provenance_ref",
				"items",
			},
			FieldShapes: []fieldShape{
				{
					Field: "request",
					Shape: `{"contract_version":"haft.memory.backfill.v1","mode":"dry_run|apply","request_provenance_ref":"operator:<exact receiving use>","items":[{"artifact_ref":"note-...","entity_ref":{"ref_kind_id":"U.EntityRef","reference_id":"entity:..."},"bounded_context_ref":"..."}]}`,
					Note:  "Each artifact is selected exactly once. EntityOfConcern coordinates are required only for routes that declare exact_entity_of_concern_and_bounded_context; they are never inferred from artifact IDs, titles, contexts, or rank.",
				},
				{
					Field: "response",
					Shape: `{"contract_version":"haft.memory.backfill.v1","mode":"dry_run|apply","project_id":"...","request_provenance_ref":"...","graph_revision_before":7,"graph_revision_after":7,"routes":[{"artifact_ref":"...","artifact_kind":"Note","version":1,"projection":"Haft.NoteAtConcern","requirements":["exact_entity_of_concern_and_bounded_context"],"result":"validated_only|already_projected|committed|unresolved|invalid|unavailable|outcome_unknown","projection_report":{...}}],"deferred":[{"artifact_ref":"...","artifact_kind":"EvidencePack","version":1,"reason":"evidence_carrier_needs_exact_work_and_evidence_source"}],"summary":{"selected_artifacts":2,"planned_routes":1,"validated_only":1,"already_projected":0,"committed":0,"unresolved":0,"invalid":0,"unavailable":0,"outcome_unknown":0,"deferred":1},"authority_boundary":{...}}`,
					Note:  "Dry-run opens the checked project ledger read-only and requires an unchanged graph revision. Apply commits only source-owned non-binding projections through the same task adapters and AdmissionService used at creation time.",
				},
			},
			Notes: []string{
				"The input is strict JSON from --input-file; use --input-file - for stdin.",
				"Only explicitly listed artifacts are inventoried. Empty or duplicate selections, unknown fields, trailing JSON, missing records, and unsupported contract versions fail closed.",
				"Note claims are recovered only from canonical Observations, Rationale, and Source sections; ProblemCard claims come from structured_data; other missing source meaning remains unresolved.",
				"SolutionPortfolio precedes PortfolioComparison, which precedes DecisionRecord. Dry-run may expose a planned dependency as unresolved without creating it.",
				"EvidencePack, WorkCommission, MethodRun, and RefreshReport remain deferred until a source-owned adapter can preserve their exact meaning.",
			},
		},
		OutputVolume: []string{
			"one deterministic JSON report over the explicitly selected inventory",
		},
		Invariants: append(
			commonInterfaceInvariants(),
			"Dry-run writes zero rows and cannot change graph revision.",
			"Apply is explicit non-binding projection admission, never automatic background migration.",
			"The operation cannot declare schema, select or change ProjectTypeEnvHead, bind or supersede a decision, approve a specification, establish evidence truth, or claim performed Work.",
			"Legacy carriers remain byte-preserved and unsupported or underdetermined meaning remains explicit.",
		),
	}
}

func memoryResolveInterfaceCapability() interfaceCapability {
	outputContract := projectmemory.MemoryReadOutputContractV1()
	return interfaceCapability{
		ID:      "memory.resolve",
		Purpose: "Resolve an exact or ambiguous EntityOfConcern identity inside one pinned project-memory snapshot without inferring project relations.",
		CurrentExecution: interfaceExecution{
			MCPTool:          "haft_query",
			MCPAction:        "memory",
			MCPCall:          `haft_query(action="memory", memory_request={"mode":"resolve","contract_version":"haft.memory.v1","basis":{"kind":"project_current"},"query":"...","bounded_context_ref":"...","max_candidates":8})`,
			CLIStatus:        "available",
			CLICommand:       "haft memory resolve --input-file request.json",
			DiscoveryCommand: "haft interface memory.resolve --json",
		},
		InputContract: interfaceContract{
			RequiredFields: []string{
				"action",
				"memory_request.mode",
				"memory_request.contract_version",
				"memory_request.basis",
				"memory_request.query",
				"memory_request.max_candidates",
			},
			OptionalFields: []string{
				"memory_request.bounded_context_ref",
			},
			FieldShapes: []fieldShape{
				{
					Field: "memory_request.basis",
					Shape: `{"kind":"project_current"} | {"kind":"exact_project","type_env_digest":"sha256:<64-hex>","graph_revision":17}`,
					Note:  "The response pins the resolved project, graph, TypeEnv, and index basis. exact_project never broadens to current.",
				},
			},
			Notes: []string{
				"MCP uses the public action=memory plus closed memory_request envelope. The dedicated CLI file uses the haft.memory.v1 semantic request with flat action=resolve and rejects MCP-only envelope fields.",
				"Exact identifiers and exact aliases bypass lexical ranking. Ambiguous identities remain a candidate set; known_absent requires named completeness.",
				"The machine-readable output_contract names every closed result variant and required wire field; it replaces approximate {...} response prose.",
			},
		},
		OutputContract: outputContract,
		OutputVolume: []string{
			"one exact result, bounded candidate set, explicit unsettled basis, known absence, or retry requirement",
		},
		Invariants: append(
			memoryReadInterfaceInvariants(),
			"Resolution never treats a candidate as the current EntityOfConcern and never adds a typed relation.",
		),
	}
}

func memoryNeighborhoodInterfaceCapability() interfaceCapability {
	outputContract := projectmemory.MemoryReadOutputContractV1()
	return interfaceCapability{
		ID:      "memory.neighborhood",
		Purpose: "Hydrate one exact EntityOfConcern-centered typed project-memory neighborhood under an explicit projection profile and dimensioned budget.",
		CurrentExecution: interfaceExecution{
			MCPTool:          "haft_query",
			MCPAction:        "memory",
			MCPCall:          `haft_query(action="memory", memory_request={"mode":"neighborhood","contract_version":"haft.memory.v1","basis":{"kind":"project_current"},"entity_ref":{"ref_kind_id":"...","reference_id":"..."},"bounded_context_ref":"...","view":{...},"read_budget":{...}})`,
			CLIStatus:        "available",
			CLICommand:       "haft memory neighborhood --input-file request.json",
			DiscoveryCommand: "haft interface memory.neighborhood --json",
		},
		InputContract: interfaceContract{
			RequiredFields: []string{
				"action",
				"memory_request.mode",
				"memory_request.contract_version",
				"memory_request.basis",
				"memory_request.entity_ref",
				"memory_request.bounded_context_ref",
				"memory_request.view",
				"memory_request.read_budget",
			},
			FieldShapes: []fieldShape{
				{
					Field: "memory_request.entity_ref",
					Shape: `{"ref_kind_id":"...","reference_id":"..."}`,
					Note:  "The exact selected TypeEnv resolves the reference kind. Call memory.resolve first when only a name or alias is known.",
				},
				{
					Field: "memory_request.view",
					Shape: `{"projection_profile_ref":"agent_orientation.v2|agent_orientation.v1|decision_rationale.v1|spec_impact.v1|evidence_currentness.v1|implementation_trace.v1","requested_facets":["epistemes","problems","alternatives","decisions","specifications","evidence","work","implementation","unresolved"],"detail":"overview|standard|evidence","include_history":false}`,
					Note:  "A projection profile changes inclusion and presentation, not truth, authority, applicability, or graph semantics.",
				},
			},
			Notes: []string{
				"MCP uses action=memory plus the closed neighborhood memory_request branch. The dedicated CLI file uses flat action=neighborhood and rejects MCP-only envelope fields.",
				"Read budgets are dimensioned: facets, items per facet, relation paths, carrier excerpt characters, and provenance depth.",
				"The machine-readable output_contract names every closed result variant, ProjectionBasis, facet issue/coverage, retry/abstention, interpretation, and budget field; it replaces approximate {...} response prose.",
			},
		},
		OutputContract: outputContract,
		OutputVolume: []string{
			"one budgeted exact neighborhood, explicit abstention, or snapshot-bound retry requirement",
		},
		Invariants: append(
			memoryReadInterfaceInvariants(),
			"Graph direction and inclusion paths do not imply causality, applicability, precedence, or project Work order.",
		),
	}
}

func memoryRecallInterfaceCapability() interfaceCapability {
	outputContract := projectmemory.MemoryReadOutputContractV1()
	return interfaceCapability{
		ID:      "memory.recall",
		Purpose: "Recall bounded lexical candidates only inside one exact EntityOfConcern neighborhood and pinned project snapshot.",
		CurrentExecution: interfaceExecution{
			MCPTool:          "haft_query",
			MCPAction:        "memory",
			MCPCall:          `haft_query(action="memory", memory_request={"mode":"recall","contract_version":"haft.memory.v1","basis":{"kind":"project_current"},"entity_ref":{"ref_kind_id":"...","reference_id":"..."},"bounded_context_ref":"...","view":{...},"read_budget":{...},"query":"...","candidate_budget":{"max_candidates":8}})`,
			CLIStatus:        "available",
			CLICommand:       "haft memory recall --input-file request.json",
			DiscoveryCommand: "haft interface memory.recall --json",
		},
		InputContract: interfaceContract{
			RequiredFields: []string{
				"action",
				"memory_request.mode",
				"memory_request.contract_version",
				"memory_request.basis",
				"memory_request.entity_ref",
				"memory_request.bounded_context_ref",
				"memory_request.view",
				"memory_request.read_budget",
				"memory_request.query",
				"memory_request.candidate_budget",
			},
			FieldShapes: []fieldShape{
				{
					Field: "memory_request.candidate_budget",
					Shape: `{"max_candidates":8}`,
					Note:  "This bounds presentation after exact EntityOfConcern/context/profile filtering. It does not widen the scope.",
				},
			},
			Notes: []string{
				"MCP uses action=memory plus the closed recall memory_request branch. The dedicated CLI file uses flat action=recall and rejects MCP-only envelope fields.",
				"V9 uses exact scope plus lexical retrieval. Dense/vector/PPR producers remain replaceable benchmark-gated extensions.",
				"No match or unusable producers return explicit abstention; stale snapshot/cursor returns retry_required without silently widening.",
				"The machine-readable output_contract names every closed result variant and required wire field; it replaces approximate {...} response prose.",
			},
		},
		OutputContract: outputContract,
		OutputVolume: []string{
			"one bounded scoped candidate set, explicit abstention, or snapshot-bound retry requirement",
		},
		Invariants: append(
			memoryReadInterfaceInvariants(),
			"Recall cannot cross the exact EntityOfConcern, bounded context, projection profile, graph revision, or selected TypeEnv basis.",
		),
	}
}

func memoryReadInterfaceInvariants() []string {
	return append(
		commonInterfaceInvariants(),
		"Project-memory reads write zero rows and expose no admission, Stage, ProjectTypeEnvHead selection, decision, commission, or specification-lifecycle capability.",
		"Candidate rank and projection inclusion are retrieval facts, not applicability, recommendation, evidence truth, or authorization.",
		"Snapshot mismatch returns retry_required; the runtime never silently moves a request to a newer graph or TypeEnv.",
	)
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
	Checked                     bool     `json:"checked"`
	Status                      string   `json:"status"`
	ActionSchemaPosture         string   `json:"action_schema_posture,omitempty"`
	AdditionalPropertiesPosture string   `json:"additional_properties_posture,omitempty"`
	MissingFields               []string `json:"missing_fields,omitempty"`
	ExcludedFields              []string `json:"excluded_fields,omitempty"`
	MCPRequiredFields           []string `json:"mcp_required_fields,omitempty"`
	MissingRequiredFields       []string `json:"missing_required_fields,omitempty"`
	ActionRequiredFields        []string `json:"action_required_fields,omitempty"`
	RequiredPosture             string   `json:"required_posture,omitempty"`
	ActionUnionDiagnostics      []string `json:"action_union_diagnostics,omitempty"`
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
	Kind                  string                                           `json:"kind"`
	SchemaVersion         int                                              `json:"schema_version"`
	Authority             string                                           `json:"authority"`
	CheckScope            string                                           `json:"check_scope"`
	Source                string                                           `json:"source"`
	SourceDigest          string                                           `json:"source_digest"`
	SemanticBytesVerified bool                                             `json:"semantic_bytes_verified"`
	Summary               interfaceContractMaterializedCarrierCheckSummary `json:"summary"`
	Carriers              []interfaceContractMaterializedCarrierCheckItem  `json:"carriers,omitempty"`
}

type interfaceContractMaterializedCarrierCheckSummary struct {
	MaterializedCarriers   int  `json:"materialized_carriers"`
	CheckedCarriers        int  `json:"checked_carriers"`
	MissingCarrierFiles    int  `json:"missing_carrier_files"`
	MissingMarkers         int  `json:"missing_markers"`
	RequiredMarkersPresent bool `json:"required_markers_present"`
	SemanticBytesVerified  bool `json:"semantic_bytes_verified"`
}

type interfaceContractMaterializedCarrierCheckItem struct {
	CarrierPath            string   `json:"carrier_path"`
	CarrierKind            string   `json:"carrier_kind"`
	ContractRole           string   `json:"contract_role"`
	MarkerRefreshPosture   string   `json:"marker_refresh_posture"`
	MarkerGuardPosture     string   `json:"marker_guard_posture"`
	RequiredMarkersPresent bool     `json:"required_markers_present"`
	SemanticBytesVerified  bool     `json:"semantic_bytes_verified"`
	MissingFile            bool     `json:"missing_file,omitempty"`
	MissingMarkers         []string `json:"missing_markers,omitempty"`
	ValidationRefs         []string `json:"validation_refs,omitempty"`
}

type interfaceContractMaterializedCarrierSyncReport struct {
	Kind                   string                                           `json:"kind"`
	SchemaVersion          int                                              `json:"schema_version"`
	Authority              string                                           `json:"authority"`
	MutationScope          string                                           `json:"mutation_scope"`
	Source                 string                                           `json:"source"`
	SourceDigest           string                                           `json:"source_digest"`
	SemanticBytesRewritten bool                                             `json:"semantic_bytes_rewritten"`
	SemanticBytesVerified  bool                                             `json:"semantic_bytes_verified"`
	Summary                interfaceContractMaterializedCarrierSyncSummary  `json:"summary"`
	Carriers               []interfaceContractMaterializedCarrierSyncItem   `json:"carriers,omitempty"`
	Check                  interfaceContractMaterializedCarrierCheckSummary `json:"post_refresh_marker_check"`
}

type interfaceContractMaterializedCarrierSyncSummary struct {
	MaterializedCarriers    int  `json:"materialized_carriers"`
	MarkerRefreshedCarriers int  `json:"marker_refreshed_carriers"`
	UnchangedMarkerCarriers int  `json:"unchanged_marker_carriers"`
	MissingCarrierFiles     int  `json:"missing_carrier_files"`
	UpdatedDigestMarkers    int  `json:"updated_digest_markers"`
	RequiredMarkersPresent  bool `json:"required_markers_present"`
	SemanticBytesRewritten  bool `json:"semantic_bytes_rewritten"`
	SemanticBytesVerified   bool `json:"semantic_bytes_verified"`
}

type interfaceContractMaterializedCarrierSyncItem struct {
	CarrierPath            string   `json:"carrier_path"`
	ResolvedPath           string   `json:"resolved_path,omitempty"`
	CarrierKind            string   `json:"carrier_kind"`
	ContractRole           string   `json:"contract_role"`
	MarkerRefreshPosture   string   `json:"marker_refresh_posture"`
	MarkerGuardPosture     string   `json:"marker_guard_posture"`
	MarkerUpdated          bool     `json:"marker_updated"`
	SemanticBytesRewritten bool     `json:"semantic_bytes_rewritten"`
	MissingFile            bool     `json:"missing_file,omitempty"`
	UpdatedDigestMarkers   int      `json:"updated_digest_markers,omitempty"`
	ValidationRefs         []string `json:"validation_refs,omitempty"`
	AuthorityBoundary      string   `json:"authority_boundary"`
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
	DigestGuardedCarriers     int `json:"digest_marker_guarded_carriers"`
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
	ExpectedSourceDigest  string   `json:"expected_digest_marker,omitempty"`
	SyncPosture           string   `json:"marker_refresh_posture"`
	GuardPosture          string   `json:"marker_guard_posture"`
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
			"materialized_carriers names repo paths expected to contain specific digest and authority marker strings; the list and marker checks do not establish byte-for-byte semantic synchronization or currentness.",
			"use `haft interface contract-generation --write-schema-fragments <path>` to materialize generated_schema_fragments as a deterministic JSON carrier for host/schema sync checks.",
			"use `haft interface contract-generation --write-description-fragments <path>` to materialize generated_fragments as a deterministic JSON carrier for host/skill/plugin/Pi wording sync checks.",
			"use `--check-schema-fragments <path>` or `--check-description-fragments <path>` to compare materialized carriers against the current source digest without rewriting them.",
			"use `--check-materialized-carriers` only to check required marker-string presence in listed repo carriers; it does not compare or verify semantic bytes.",
			"`--sync-materialized-carriers` is a compatibility flag that refreshes only kernel-interface source-digest marker lines; it does not render, synchronize, compare, or verify semantic carrier bytes.",
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
	if surface.MCPTool == "" {
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
		Kind:                  "haft_interface_materialized_carrier_marker_check",
		SchemaVersion:         2,
		Authority:             "required_marker_presence_check_only_not_semantic_byte_currentness_materialization_or_binding_authority",
		CheckScope:            "required_marker_string_presence_only",
		Source:                report.Source,
		SourceDigest:          report.SourceDigest,
		SemanticBytesVerified: false,
		Summary: interfaceContractMaterializedCarrierCheckSummary{
			MaterializedCarriers:   len(report.Carriers),
			RequiredMarkersPresent: true,
			SemanticBytesVerified:  false,
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
		if !item.RequiredMarkersPresent {
			result.Summary.RequiredMarkersPresent = false
		}
	}

	if !result.Summary.RequiredMarkersPresent {
		return result, fmt.Errorf(
			"required materialized-carrier marker check failed: missing_files=%d missing_markers=%d; semantic bytes were not compared",
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
		CarrierPath:            carrier.CarrierPath,
		CarrierKind:            carrier.CarrierKind,
		ContractRole:           carrier.ContractRole,
		MarkerRefreshPosture:   carrier.SyncPosture,
		MarkerGuardPosture:     carrier.GuardPosture,
		RequiredMarkersPresent: true,
		SemanticBytesVerified:  false,
		ValidationRefs:         append([]string(nil), carrier.ValidationRefs...),
	}

	data, err := readInterfaceContractMaterializedCarrier(carrier.CarrierPath)
	if err != nil {
		item.RequiredMarkersPresent = false
		item.MissingFile = true
		return item
	}

	text := normalizeInterfaceContractMarkerText(string(data))
	for _, marker := range carrier.RequiredMarkers {
		normalizedMarker := normalizeInterfaceContractMarkerText(marker)
		if strings.Contains(text, normalizedMarker) {
			continue
		}
		item.MissingMarkers = append(item.MissingMarkers, marker)
	}
	if len(item.MissingMarkers) != 0 {
		item.RequiredMarkersPresent = false
	}
	return item
}

func normalizeInterfaceContractMarkerText(value string) string {
	fields := strings.Fields(value)
	normalized := strings.Join(fields, " ")
	return strings.ToLower(normalized)
}

func syncInterfaceContractMaterializedCarriers(
	report interfaceContractGenerationReport,
) (interfaceContractMaterializedCarrierSyncReport, error) {
	result := interfaceContractMaterializedCarrierSyncReport{
		Kind:                   "haft_interface_materialized_carrier_marker_refresh",
		SchemaVersion:          2,
		Authority:              "digest_marker_refresh_only_not_semantic_sync_currentness_host_runtime_materialization_binding_authority_evidence_approval_gate_decision_claim_truth_global_truth_or_publication",
		MutationScope:          "kernel_interface_catalog_source_digest_marker_lines_only",
		Source:                 report.Source,
		SourceDigest:           report.SourceDigest,
		SemanticBytesRewritten: false,
		SemanticBytesVerified:  false,
		Summary: interfaceContractMaterializedCarrierSyncSummary{
			MaterializedCarriers:   len(report.Carriers),
			SemanticBytesRewritten: false,
			SemanticBytesVerified:  false,
		},
		Carriers: make([]interfaceContractMaterializedCarrierSyncItem, 0, len(report.Carriers)),
	}

	for _, carrier := range report.Carriers {
		item := syncInterfaceContractMaterializedCarrier(carrier, report.SourceDigest)
		result.Carriers = append(result.Carriers, item)
		result.Summary.UpdatedDigestMarkers += item.UpdatedDigestMarkers
		if item.MissingFile {
			result.Summary.MissingCarrierFiles++
			continue
		}
		if item.MarkerUpdated {
			result.Summary.MarkerRefreshedCarriers++
			continue
		}
		result.Summary.UnchangedMarkerCarriers++
	}

	if result.Summary.MissingCarrierFiles != 0 {
		return result, fmt.Errorf(
			"materialized-carrier digest marker refresh failed: missing_files=%d; semantic bytes were not rendered or verified",
			result.Summary.MissingCarrierFiles,
		)
	}

	check, err := checkInterfaceContractMaterializedCarriers(report)
	result.Check = check.Summary
	result.Summary.RequiredMarkersPresent = check.Summary.RequiredMarkersPresent
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
		CarrierPath:            carrier.CarrierPath,
		CarrierKind:            carrier.CarrierKind,
		ContractRole:           carrier.ContractRole,
		MarkerRefreshPosture:   carrier.SyncPosture,
		MarkerGuardPosture:     carrier.GuardPosture,
		SemanticBytesRewritten: false,
		ValidationRefs:         append([]string(nil), carrier.ValidationRefs...),
		AuthorityBoundary:      carrier.AuthorityBoundary,
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
	item.MarkerUpdated = true
	item.UpdatedDigestMarkers = updatedMarkers
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
	bindingBoundary := "binding actions require effect-specific operator authority. Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts"
	readOnlyBoundary := "read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization"
	carriers := []interfaceContractMaterializedCarrier{
		{
			CarrierPath:           "packages/haft-pi/extensions/haft/tools.ts",
			CarrierKind:           "pi_tool_metadata",
			ContractRole:          "tool_schema_and_description_materialization",
			SourceContract:        "kernel_interface_catalog",
			ExpectedSourceDigest:  sourceDigest,
			SyncPosture:           "digest_marker_presence_only_not_semantic_sync",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"kernelInterfaceCatalogDigest", sourceDigest, "contract_generation", "wrong_identifier_namespace", "artifact_ref", "symbol", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"query.node", "query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set", "decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiToolMetadataCarriesGeneratedContractAuthorityBoundaries", "internal/cli/interface_test.go:TestPiToolSchemasMirrorGeneratedSchemaFragments", "internal/fpf/server_test.go:TestHandleToolsList_QueryExactFieldsDeclareIdentifierNamespaces", "packages/haft-pi/tests/pure.test.ts:Pi haft_query schema mirrors current read-only drill-down actions"},
		},
		{
			CarrierPath:           "packages/haft-pi/skills/h-status/SKILL.md",
			CarrierKind:           "pi_skill",
			ContractRole:          "status_drill_down_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     readOnlyBoundary,
			RequiredMarkers:       []string{"contract_generation", "generated-fragment", "drift fanout", "reconciliation", "current-authority drill-downs", "Human Gate Brief", readOnlyBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "packages/haft-pi/prompts/h-decide.md",
			CarrierKind:           "pi_manual_gate_prompt",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "host_routed_operator_request", "direct, unambiguous operator request", "DecisionRecord", "stable host parity is not proven", "skill invocation", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiPromptCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "packages/haft-pi/prompts/h-commission.md",
			CarrierKind:           "pi_manual_gate_prompt",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiPromptCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "packages/haft-pi/prompts/h-reason.md",
			CarrierKind:           "pi_workflow_prompt",
			ContractRole:          "source_first_reasoning_entrypoint_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{`"action": "fpf"`, `"mode": "concern"`, "wrong_identifier_namespace", `"action": "related"`, `"artifact_ref"`, "Returned source material", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"query.fpf", "query.node", "decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiReasoningPromptUsesSourceNativeFPFQuery", "internal/cli/interface_test.go:TestPiReasoningCarriersTeachIdentifierNamespaces"},
		},
		{
			CarrierPath:           "packages/haft-pi/skills/h-reason/SKILL.md",
			CarrierKind:           "pi_skill",
			ContractRole:          "source_first_reasoning_entrypoint_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     "Retrieval and namespace recovery do not grant binding authority",
			RequiredMarkers:       []string{"wrong_identifier_namespace", `action="related"`, `artifact_ref="<id>"`, `action="memory"`, "memory_request", `"mode":"resolve"`, `haft_onboard(action="status")`, "haft_entity", "known_absent", "stable host parity is not", "Human Gate Brief"},
			GeneratedFragmentRefs: []string{"query.fpf", "query.node", "memory.resolve"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestPiReasoningCarriersTeachIdentifierNamespaces"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-status/SKILL.md",
			CarrierKind:           "bundled_skill",
			ContractRole:          "status_drill_down_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     "Generated text and read-only projections are discovery surfaces.",
			RequiredMarkers:       []string{"contract_generation", "drift_events", "decision_reconcile", "governing_set", "Human Gate Brief", "Generated text and read-only projections are discovery surfaces.", "evidence truth, gate passage, approval, authority, or work."},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-verify/SKILL.md",
			CarrierKind:           "bundled_skill",
			ContractRole:          "maintenance_drill_down_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     readOnlyBoundary,
			RequiredMarkers:       []string{"contract_generation", "generated-fragment", "drift fanout", "reconciliation", "current-authority drill-downs", readOnlyBoundary},
			GeneratedFragmentRefs: []string{"query.contract_generation", "query.drift_events", "query.decision_reconcile", "query.governing_set"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarrySelectedGeneratedQueryFragments"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-reason/SKILL.md",
			CarrierKind:           "bundled_skill",
			ContractRole:          "source_first_reasoning_entrypoint_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     "retrieval rank != applicability, recommendation, authorization, or work",
			RequiredMarkers:       []string{`action="fpf"`, `mode="concern"`, "wrong_identifier_namespace", `action="related"`, `artifact_ref="<id>"`, "retrieval rank != applicability, recommendation, authorization, or work", "ordinaryBounded", "Human Gate Brief", "h-decide may route a direct operator request; h-commission remains manual-only"},
			GeneratedFragmentRefs: []string{"query.fpf", "query.node", "decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledReasoningSkillUsesSourceNativeFPFQuery"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-decide/SKILL.md",
			CarrierKind:           "bundled_manual_skill",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "host_routed_operator_request", "direct, unambiguous", "compatible shortcut", "DecisionRecord route", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "internal/cli/skill/h-commission/SKILL.md",
			CarrierKind:           "bundled_manual_skill",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "internal/cli/claude_md_template.md",
			CarrierKind:           "host_instruction_template",
			ContractRole:          "project_discipline_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"Binding decisions and execution authority require effect-specific operator", "model-supplied fields are not", "operator requests.", "Human Gate Brief"},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "AGENTS.md",
			CarrierKind:           "repo_agent_instruction",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "host_routed_operator_request", "direct, unambiguous operator request", "skill token is not a receipt", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries"},
		},
		{
			CarrierPath:           "CLAUDE.md",
			CarrierKind:           "repo_agent_instruction",
			ContractRole:          "binding_authority_boundary_materialization",
			SourceContract:        "kernel_interface_catalog",
			SyncPosture:           "authority_marker_presence_only_digest_marker_pending",
			GuardPosture:          "required_marker_presence_only_not_semantic_bytes",
			AuthorityBoundary:     bindingBoundary,
			RequiredMarkers:       []string{"operator_confirmation_required", "host_routed_operator_request", "direct, unambiguous operator request", "skill token is not a receipt", "Human Gate Brief", bindingBoundary},
			GeneratedFragmentRefs: []string{"decision.decide", "commission.create"},
			ValidationRefs:        []string{"internal/cli/interface_test.go:TestBundledSkillCarriersCarryGeneratedContractAuthorityBoundaries", "internal/cli/claude_md_test.go:TestClaudeMDTemplateInSyncWithRepoRoot"},
		},
	}
	for index := range carriers {
		if carriers[index].ExpectedSourceDigest == "" {
			carriers[index].ExpectedSourceDigest = sourceDigest
			carriers[index].RequiredMarkers = append(carriers[index].RequiredMarkers, sourceDigest)
		}
		if carriers[index].SyncPosture == "authority_marker_presence_only_digest_marker_pending" {
			carriers[index].SyncPosture = "digest_marker_presence_only_not_semantic_sync"
		}
	}
	return carriers
}

func countInterfaceContractDigestGuardedCarriers(carriers []interfaceContractMaterializedCarrier) int {
	total := 0
	for _, carrier := range carriers {
		if carrier.ExpectedSourceDigest != "" {
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
	requiredFields := interfaceContractAuditExpectedMCPRequiredFieldsForSchema(
		capability,
		toolSchema,
	)
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
	actionSchema, _ := toolSchema.actionSchema(surface.MCPAction)
	properties := make(map[string]any, len(allowedFields)+1)
	if surface.MCPAction != "" {
		properties["action"] = map[string]any{
			"type":  "string",
			"const": surface.MCPAction,
		}
	}
	for _, field := range allowedFields {
		if field == "action" {
			continue
		}
		propertySchema, ok :=
			cloneJSONLikeMap(actionSchema.Properties[field])
		if !ok {
			propertySchema = map[string]any{
				"description": "shape validated by MCP schema mirror and kernel handler",
			}
		}
		properties[field] = propertySchema
	}
	additionalProperties := true
	if actionSchema.AdditionalPropertiesSet {
		additionalProperties = actionSchema.AdditionalProperties
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": additionalProperties,
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
		AdditionalPropertiesPosture: "preserved_per_action_from_runtime_mcp_schema; closed oneOf branches remain closed while shared legacy schemas retain their declared posture",
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
	actionSchema, ok := schema.actionSchema(fragment.MCPAction)
	if !ok {
		return []string{fragment.CapabilityID + ".action"}
	}
	missing := make([]string, 0)
	for _, field := range fragment.AllowedTopLevelFields {
		if field == "action" {
			continue
		}
		if _, ok := actionSchema.Properties[field]; ok {
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
	actionSchema, ok := schema.actionSchema(fragment.MCPAction)
	if !ok {
		return []string{fragment.CapabilityID + ".action"}
	}
	missing := make([]string, 0)
	for _, field := range fragment.RequiredFields {
		if actionSchema.Required[field] {
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
	if len(schema.ActionUnionDiagnostics) > 0 {
		return false
	}
	_, ok := schema.actionSchema(action)
	return ok
}

func stringEnumFromSchemaProperty(value interface{}) []string {
	property, ok := cloneJSONLikeMap(value)
	if !ok {
		return nil
	}
	if literal, ok := property["const"].(string); ok && literal != "" {
		return []string{literal}
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
	bindingSensitive := interfaceContractAuditBindingSensitive(surface.AuthorityPosture)
	humanGateBriefByPosture := map[bool]string{
		false: "",
		true:  humanGateBriefRequirement,
	}
	humanGateBrief := humanGateBriefByPosture[bindingSensitive]
	parts := []string{
		surface.CapabilityID,
		strings.TrimSpace(surface.CLICommand),
		strings.TrimSpace(surface.MCPTool),
		strings.TrimSpace(surface.MCPAction),
		surface.AuthorityPosture,
		interfaceContractGeneratedFragmentAuthorityBoundary(surface),
		"Generated from kernel_interface_catalog; do not edit host/skill/plugin/Pi wording by hand when this fragment is available.",
		humanGateBrief,
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
		return "binding actions require effect-specific operator authority. Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts"
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
	return buildInterfaceContractAuditReportWithToolSchemas(
		catalog,
		interfaceContractAuditToolSchemas(),
	)
}

func buildInterfaceContractAuditReportWithToolSchemas(
	catalog []interfaceCapability,
	toolSchemas map[string]interfaceContractAuditToolSchema,
) interfaceContractAuditReport {
	surfaces := make([]interfaceContractAuditSurface, 0, len(catalog))
	summary := interfaceContractAuditSummary{Capabilities: len(catalog)}
	exclusions := interfaceContractAuditSchemaFieldExclusions()
	toolSchemas = interfaceContractAuditToolSchemasAgainstCatalog(
		catalog,
		toolSchemas,
	)

	for _, capability := range catalog {
		surface := buildInterfaceContractAuditSurface(capability, toolSchemas, exclusions)
		surfaces = append(surfaces, surface)

		if interfaceContractAuditContainsString(surface.ContractSources, "kernel_interface_catalog") {
			summary.KernelOwnedContracts++
		}
		if surface.SchemaPosture == "mcp_schema_mirrored" &&
			surface.SchemaCoverage.Status == "covered" {
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
		if interfaceContractAuditSchemaCoverageMissing(
			surface.SchemaCoverage.Status,
		) {
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
			"Action-discriminated oneOf tools are audited branch by branch: each advertised action needs one exact const or singleton-enum discriminator, its own required fields, and explicit additionalProperties=false.",
			"Flat direct-object host schemas retain shared transport-required fields; action-specific required fields remain explicit handler contracts rather than fabricated top-level JSON-Schema requirements.",
			"A catalog action absent from the handler-installed schema, or a haft_memory handler action absent from the interface catalog, is an overstatement and remains an unvalidated fragment; multi-value, absent, non-string, duplicate, or open action branches are rejected.",
			"Schema visibility is not operator authorization, binding authority, evidence, gate passage, claim truth, global truth, or publication.",
			"Generated MCP/host schema work remains a later phase and must validate against this inventory.",
			"Default status must not inline this report; use haft interface contract-audit --json or haft_query(action=\"contract_audit\").",
		},
	}
}

func interfaceContractAuditSchemaCoverageMissing(status string) bool {
	switch status {
	case "missing_fields",
		"missing_required_fields",
		"missing_mcp_tool_schema",
		"missing_mcp_action",
		"malformed_action_union",
		"handler_action_surface_overexposed",
		"open_action_branch":
		return true
	default:
		return false
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
		surface.Notes = append(surface.Notes, "binding-sensitive surface remains governed by effect-specific operator authority")
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
	if surface.MCPTool == "" {
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
	if execution.MCPTool == "" {
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
	actionSchema, ok := toolSchema.actionSchema(execution.MCPAction)
	if !ok {
		return interfaceContractAuditShapeCoverage{
			Checked:            true,
			Status:             "missing_mcp_action",
			MissingShapeFields: []string{"action"},
		}
	}
	if len(toolSchema.ActionUnionDiagnostics) > 0 {
		return interfaceContractAuditShapeCoverage{
			Checked:            true,
			Status:             "malformed_action_union",
			MissingShapeFields: []string{"action"},
		}
	}
	if len(toolSchema.CatalogActionDiagnostics) > 0 {
		return interfaceContractAuditShapeCoverage{
			Checked:            true,
			Status:             "handler_action_surface_overexposed",
			MissingShapeFields: []string{"action"},
		}
	}
	if toolSchema.ActionUnion &&
		(!actionSchema.AdditionalPropertiesSet ||
			actionSchema.AdditionalProperties) {
		return interfaceContractAuditShapeCoverage{
			Checked:            true,
			Status:             "open_action_branch",
			MissingShapeFields: []string{"additionalProperties"},
		}
	}
	properties := actionSchema.Properties

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

		shapeSchema := nestedSchemaAtField(
			topLevel,
			shape.Field,
			propertySchema,
		)
		missing = append(
			missing,
			missingNestedShapeFields(shape.Field, shape.Shape, shapeSchema)...,
		)
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
	if items, ok := schemaMap["items"].(map[string]interface{}); ok {
		if properties := nestedSchemaProperties(items); len(properties) > 0 {
			return properties
		}
	}
	for _, unionKey := range []string{"oneOf", "anyOf"} {
		branches, ok := schemaMap[unionKey].([]interface{})
		if !ok {
			continue
		}
		merged := make(map[string]interface{})
		for _, branch := range branches {
			for field, fieldSchema := range nestedSchemaProperties(branch) {
				merged[field] = fieldSchema
			}
		}
		if len(merged) > 0 {
			return merged
		}
	}
	return nil
}

func nestedSchemaAtField(
	root string,
	field string,
	rootSchema interface{},
) interface{} {
	path := strings.TrimSpace(field)
	path = strings.TrimPrefix(path, root)
	path = strings.TrimPrefix(path, "[]")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return rootSchema
	}

	current := rootSchema
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSuffix(strings.TrimSpace(segment), "[]")
		if segment == "" {
			continue
		}
		properties := nestedSchemaProperties(current)
		next, ok := properties[segment]
		if !ok {
			return nil
		}
		current = next
	}
	return current
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
	if execution.MCPTool == "" {
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
	actionSchema, actionPresent :=
		toolSchema.actionSchema(execution.MCPAction)
	if len(toolSchema.ActionUnionDiagnostics) > 0 {
		return interfaceContractAuditSchemaCoverage{
			Checked:             true,
			Status:              "malformed_action_union",
			ActionSchemaPosture: "invalid_action_discriminated_union",
			MissingFields:       []string{"action"},
			ActionUnionDiagnostics: append(
				[]string(nil),
				toolSchema.ActionUnionDiagnostics...,
			),
		}
	}
	if len(toolSchema.CatalogActionDiagnostics) > 0 {
		return interfaceContractAuditSchemaCoverage{
			Checked:             true,
			Status:              "handler_action_surface_overexposed",
			ActionSchemaPosture: "handler_actions_exceed_interface_catalog",
			MissingFields:       []string{"catalog_action_contract"},
			ActionUnionDiagnostics: append(
				[]string(nil),
				toolSchema.CatalogActionDiagnostics...,
			),
		}
	}
	if !actionPresent {
		return interfaceContractAuditSchemaCoverage{
			Checked:             true,
			Status:              "missing_mcp_action",
			ActionSchemaPosture: "catalog_action_not_advertised",
			MissingFields:       []string{"action"},
		}
	}
	additionalPropertiesPosture :=
		interfaceContractAuditAdditionalPropertiesPosture(
			toolSchema.ActionUnion,
			actionSchema,
		)
	if toolSchema.ActionUnion &&
		additionalPropertiesPosture != "closed_action_branch" {
		return interfaceContractAuditSchemaCoverage{
			Checked:                     true,
			Status:                      "open_action_branch",
			ActionSchemaPosture:         actionSchema.DiscriminatorPosture,
			AdditionalPropertiesPosture: additionalPropertiesPosture,
			MissingFields:               []string{"additionalProperties:false"},
		}
	}
	properties := actionSchema.Properties

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

	expectedRequired := interfaceContractAuditExpectedMCPRequiredFieldsForSchema(
		capability,
		toolSchema,
	)
	missingRequired := make([]string, 0)
	for _, field := range expectedRequired {
		if !actionSchema.Required[field] {
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
		Checked:                     true,
		Status:                      status,
		ActionSchemaPosture:         actionSchema.DiscriminatorPosture,
		AdditionalPropertiesPosture: additionalPropertiesPosture,
		MissingFields:               missing,
		ExcludedFields:              excluded,
		MCPRequiredFields:           sortedStringSetKeys(actionSchema.Required),
		MissingRequiredFields:       missingRequired,
		ActionRequiredFields:        topLevelInterfaceContractRequiredFields(capability.InputContract),
		RequiredPosture:             interfaceContractAuditRequiredPosture(expectedRequired, missingRequired),
	}
}

func interfaceContractAuditAdditionalPropertiesPosture(
	actionUnion bool,
	actionSchema interfaceContractAuditActionSchema,
) string {
	if actionSchema.AdditionalPropertiesSet && !actionSchema.AdditionalProperties {
		if actionUnion {
			return "closed_action_branch"
		}
		return "closed_shared_object"
	}
	if actionSchema.AdditionalPropertiesSet {
		if actionUnion {
			return "open_action_branch"
		}
		return "open_shared_object"
	}
	if actionUnion {
		return "action_branch_additional_properties_unspecified"
	}
	return "shared_object_additional_properties_unspecified"
}

type interfaceContractAuditToolSchema struct {
	Properties               map[string]interface{}
	Required                 map[string]bool
	Actions                  map[string]bool
	AdditionalProperties     bool
	AdditionalPropertiesSet  bool
	ActionUnion              bool
	ActionBranches           map[string]interfaceContractAuditActionSchema
	ActionUnionDiagnostics   []string
	CatalogActionDiagnostics []string
}

type interfaceContractAuditActionSchema struct {
	Properties              map[string]interface{}
	Required                map[string]bool
	AdditionalProperties    bool
	AdditionalPropertiesSet bool
	DiscriminatorPosture    string
}

func interfaceContractAuditToolSchemas() map[string]interfaceContractAuditToolSchema {
	server := fpf.NewServer(Version)
	server.SetV5Handler(func(_ context.Context, _ string, _ json.RawMessage) (string, error) {
		return "", nil
	})
	server.SetMemoryFullHandler(func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", nil
	})
	server.SetMemoryReadHandler(func(_ context.Context, _ json.RawMessage) (string, error) {
		return "", nil
	})
	return interfaceContractAuditToolSchemasFromCatalog(server.ToolCatalog())
}

func interfaceContractAuditToolSchemasFromCatalog(
	catalog []fpf.Tool,
) map[string]interfaceContractAuditToolSchema {
	toolSchemas := make(map[string]interfaceContractAuditToolSchema)
	for _, tool := range catalog {
		inputSchema, ok := tool.InputSchema.(map[string]interface{})
		if !ok {
			continue
		}
		toolSchemas[tool.Name] = interfaceContractAuditToolSchemaFromInput(
			inputSchema,
		)
	}
	return toolSchemas
}

func interfaceContractAuditToolSchemasAgainstCatalog(
	catalog []interfaceCapability,
	toolSchemas map[string]interfaceContractAuditToolSchema,
) map[string]interfaceContractAuditToolSchema {
	catalogActions := make(map[string]map[string]bool)
	for _, capability := range catalog {
		tool := capability.CurrentExecution.MCPTool
		action := capability.CurrentExecution.MCPAction
		if tool == "" || action == "" {
			continue
		}
		if catalogActions[tool] == nil {
			catalogActions[tool] = make(map[string]bool)
		}
		catalogActions[tool][action] = true
	}

	checked := make(
		map[string]interfaceContractAuditToolSchema,
		len(toolSchemas),
	)
	for tool, schema := range toolSchemas {
		if schema.ActionUnion || tool == "haft_memory" {
			for action := range schema.Actions {
				if catalogActions[tool][action] {
					continue
				}
				schema.CatalogActionDiagnostics = append(
					schema.CatalogActionDiagnostics,
					"handler_action_not_cataloged:"+action,
				)
			}
			schema.CatalogActionDiagnostics =
				uniqueSortedStrings(schema.CatalogActionDiagnostics)
		}
		checked[tool] = schema
	}
	return checked
}

func interfaceContractAuditToolSchemaFromInput(
	inputSchema map[string]interface{},
) interfaceContractAuditToolSchema {
	properties, _ := inputSchema["properties"].(map[string]interface{})
	additionalProperties, additionalPropertiesSet :=
		inputSchema["additionalProperties"].(bool)
	result := interfaceContractAuditToolSchema{
		Properties:              properties,
		Required:                stringSetFromSchemaRequired(inputSchema["required"]),
		Actions:                 actionLiteralsFromSchemaProperties(properties),
		AdditionalProperties:    additionalProperties,
		AdditionalPropertiesSet: additionalPropertiesSet,
	}

	rawBranches, unionPath, union :=
		interfaceContractAuditActionUnionBranches(
			inputSchema,
			properties,
		)
	if !union {
		return result
	}

	result.ActionUnion = true
	result.ActionBranches =
		make(map[string]interfaceContractAuditActionSchema, len(rawBranches))
	if len(rawBranches) == 0 {
		result.ActionUnionDiagnostics = []string{
			unionPath + ":branches_missing",
		}
		return result
	}
	for index, rawBranch := range rawBranches {
		branchPath := fmt.Sprintf("%s[%d]", unionPath, index)
		branch, ok := rawBranch.(map[string]interface{})
		if !ok {
			result.ActionUnionDiagnostics = append(
				result.ActionUnionDiagnostics,
				branchPath+":branch_not_object_schema",
			)
			continue
		}
		if branch["type"] != "object" {
			result.ActionUnionDiagnostics = append(
				result.ActionUnionDiagnostics,
				branchPath+":branch_type_not_object",
			)
			continue
		}
		branchProperties, ok :=
			branch["properties"].(map[string]interface{})
		if !ok {
			result.ActionUnionDiagnostics = append(
				result.ActionUnionDiagnostics,
				branchPath+":properties_missing",
			)
			continue
		}
		action, discriminatorPosture, ok :=
			exactActionLiteralFromSchemaProperty(branchProperties["action"])
		if !ok {
			result.ActionUnionDiagnostics = append(
				result.ActionUnionDiagnostics,
				branchPath+":action_discriminator_not_exact_literal",
			)
			continue
		}
		if _, duplicate := result.ActionBranches[action]; duplicate {
			result.ActionUnionDiagnostics = append(
				result.ActionUnionDiagnostics,
				branchPath+":duplicate_action:"+action,
			)
			continue
		}
		branchAdditional, branchAdditionalSet :=
			branch["additionalProperties"].(bool)
		result.Actions[action] = true
		result.ActionBranches[action] = interfaceContractAuditActionSchema{
			Properties:              branchProperties,
			Required:                stringSetFromSchemaRequired(branch["required"]),
			AdditionalProperties:    branchAdditional,
			AdditionalPropertiesSet: branchAdditionalSet,
			DiscriminatorPosture:    discriminatorPosture,
		}
	}
	result.ActionUnionDiagnostics =
		uniqueSortedStrings(result.ActionUnionDiagnostics)
	return result
}

func interfaceContractAuditActionUnionBranches(
	inputSchema map[string]interface{},
	properties map[string]interface{},
) ([]interface{}, string, bool) {
	topLevel, topLevelUnion := schemaOneOfBranches(inputSchema["oneOf"])
	if topLevelUnion {
		return topLevel, "oneOf", true
	}
	if properties == nil {
		return nil, "", false
	}
	request, requestOK :=
		properties["request"].(map[string]interface{})
	if !requestOK {
		return nil, "", false
	}
	nested, nestedUnion := schemaOneOfBranches(request["oneOf"])
	if !nestedUnion {
		return nil, "", false
	}
	return nested, "properties.request.oneOf", true
}

func schemaOneOfBranches(value interface{}) ([]interface{}, bool) {
	switch typed := value.(type) {
	case []interface{}:
		return typed, true
	case nil:
		return nil, false
	default:
		return nil, true
	}
}

func actionLiteralsFromSchemaProperties(
	properties map[string]interface{},
) map[string]bool {
	result := make(map[string]bool)
	if properties == nil {
		return result
	}
	for _, action := range stringEnumFromSchemaProperty(properties["action"]) {
		result[action] = true
	}
	return result
}

func exactActionLiteralFromSchemaProperty(
	value interface{},
) (string, string, bool) {
	property, ok := cloneJSONLikeMap(value)
	if !ok {
		return "", "", false
	}
	rawConst, hasConst := property["const"]
	if hasConst {
		literal, ok := rawConst.(string)
		if !ok ||
			literal == "" ||
			literal != strings.TrimSpace(literal) {
			return "", "", false
		}
		return literal, "exact_const_discriminator", true
	}
	rawEnum, hasEnum := property["enum"]
	if !hasEnum {
		return "", "", false
	}
	literal, ok := exactSingletonStringEnum(rawEnum)
	if !ok {
		return "", "", false
	}
	return literal, "exact_singleton_enum_discriminator", true
}

func exactSingletonStringEnum(value interface{}) (string, bool) {
	values := make([]string, 0, 1)
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []interface{}:
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return "", false
			}
			values = append(values, text)
		}
	default:
		return "", false
	}
	if len(values) != 1 ||
		values[0] == "" ||
		values[0] != strings.TrimSpace(values[0]) {
		return "", false
	}
	return values[0], true
}

func (schema interfaceContractAuditToolSchema) actionSchema(
	action string,
) (interfaceContractAuditActionSchema, bool) {
	if schema.ActionUnion {
		branch, ok := schema.ActionBranches[action]
		return branch, ok
	}
	if action == "" && len(schema.Actions) == 0 {
		return interfaceContractAuditActionSchema{
			Properties:              schema.Properties,
			Required:                schema.Required,
			AdditionalProperties:    schema.AdditionalProperties,
			AdditionalPropertiesSet: schema.AdditionalPropertiesSet,
			DiscriminatorPosture:    "direct_tool_without_action",
		}, true
	}
	if !schema.Actions[action] {
		return interfaceContractAuditActionSchema{}, false
	}
	return interfaceContractAuditActionSchema{
		Properties:              schema.Properties,
		Required:                schema.Required,
		AdditionalProperties:    schema.AdditionalProperties,
		AdditionalPropertiesSet: schema.AdditionalPropertiesSet,
		DiscriminatorPosture:    "shared_action_enum",
	}, true
}

func interfaceContractAuditExpectedMCPRequiredFields(capability interfaceCapability) []string {
	if capability.CurrentExecution.MCPTool == "" {
		return nil
	}
	if capability.CurrentExecution.MCPAction == "" {
		return topLevelInterfaceContractRequiredFields(capability.InputContract)
	}
	if capability.CurrentExecution.MCPTool == "haft_memory" {
		return []string{"contract_version", "action"}
	}
	return []string{"action"}
}

func interfaceContractAuditExpectedMCPRequiredFieldsForSchema(
	capability interfaceCapability,
	toolSchema interfaceContractAuditToolSchema,
) []string {
	if toolSchema.ActionUnion {
		return topLevelInterfaceContractRequiredFields(capability.InputContract)
	}
	return interfaceContractAuditExpectedMCPRequiredFields(capability)
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
		"memory.validate": {
			"change_set": true,
		},
		"memory.admit": {
			"basis": true,
		},
		"memory.neighborhood": {
			"view": true,
		},
		"method.pull": {
			"carry_through[]": true,
		},
		"method.close": {
			"carry_through[]": true,
		},
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
	if execution.MCPTool != "" {
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
	if capability.ID == "baseline.audit" {
		return "read_only_term_audit"
	}
	if capability.ID == "memory.validate" {
		return "read_only_validation_no_persistence"
	}
	if capability.ID == "memory.admit" {
		return "non_binding_semantic_admission"
	}
	if capability.ID == "memory.backfill" {
		return "explicit_non_binding_existing_record_projection"
	}
	if strings.HasPrefix(capability.ID, "memory.") {
		return "read_only_project_memory"
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
	if capability.CurrentExecution.MCPTool == "haft_memory" {
		refs = append(
			refs,
			"internal/cli/interface_action_union_test.go",
			"internal/fpf/memory_schema_test.go",
		)
	}
	if strings.HasPrefix(capability.ID, "memory.") {
		refs = append(
			refs,
			"internal/cli/memory_full_server_test.go",
			"internal/cli/memory_read_server_test.go",
		)
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
	if capability.ID == "baseline.audit" {
		refs = append(refs, "internal/cli/baseline_audit_test.go")
	}

	return uniqueInterfaceContractAuditStrings(refs)
}

func interfaceContractAuditLegacyException(execution interfaceExecution) bool {
	if execution.MCPTool != "haft_query" {
		return false
	}

	switch execution.MCPAction {
	case "search", "status", "related", "projection", "fpf", "memory":
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
		"summary: capabilities=%d generator_target_surfaces=%d generator_target_fields=%d generated_preview_fragments=%d generated_schema_fragments=%d runtime_schema_mirrors=%d runtime_schema_drift=%d binding_preview_fragments=%d materialized_carriers=%d digest_marker_guarded_carriers=%d authority_boundary_guarded_carriers=%d validation_refs=%d\n",
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
	if _, err := fmt.Fprintf(output, "Haft interface materialized carrier required-marker check v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "check_scope: %s\n", result.CheckScope); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: materialized_carriers=%d checked_carriers=%d missing_carrier_files=%d missing_markers=%d required_markers_present=%t semantic_bytes_verified=%t\n",
		result.Summary.MaterializedCarriers,
		result.Summary.CheckedCarriers,
		result.Summary.MissingCarrierFiles,
		result.Summary.MissingMarkers,
		result.Summary.RequiredMarkersPresent,
		result.Summary.SemanticBytesVerified,
	); err != nil {
		return err
	}
	for _, carrier := range result.Carriers {
		if carrier.RequiredMarkersPresent {
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
	if _, err := fmt.Fprintf(output, "Haft interface materialized carrier digest-marker refresh v%d\n", result.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "authority: %s\n", result.Authority); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "mutation_scope: %s\n", result.MutationScope); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "source: %s %s\n", result.Source, result.SourceDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"summary: materialized_carriers=%d marker_refreshed_carriers=%d unchanged_marker_carriers=%d missing_carrier_files=%d updated_digest_markers=%d required_markers_present=%t semantic_bytes_rewritten=%t semantic_bytes_verified=%t\n",
		result.Summary.MaterializedCarriers,
		result.Summary.MarkerRefreshedCarriers,
		result.Summary.UnchangedMarkerCarriers,
		result.Summary.MissingCarrierFiles,
		result.Summary.UpdatedDigestMarkers,
		result.Summary.RequiredMarkersPresent,
		result.Summary.SemanticBytesRewritten,
		result.Summary.SemanticBytesVerified,
	); err != nil {
		return err
	}
	for _, carrier := range result.Carriers {
		if !carrier.MarkerUpdated && !carrier.MissingFile {
			continue
		}
		status := "marker-refreshed"
		if carrier.MissingFile {
			status = "missing"
		}
		if _, err := fmt.Fprintf(
			output,
			"- %s: %s updated_digest_markers=%d semantic_bytes_rewritten=%t\n",
			status,
			carrier.CarrierPath,
			carrier.UpdatedDigestMarkers,
			carrier.SemanticBytesRewritten,
		); err != nil {
			return err
		}
	}
	return nil
}
