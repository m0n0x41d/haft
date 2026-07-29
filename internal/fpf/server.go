package fpf

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/logger"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema"`
}

type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// V5ToolHandler handles a v5 MCP tool call and returns the result text.
type V5ToolHandler func(ctx context.Context, toolName string, params json.RawMessage) (string, error)

// MemoryToolHandler handles the strict typed-memory wire contract. It receives
// the original tools/call arguments bytes so duplicate fields and other JSON
// boundary violations remain observable to the dedicated decoder.
type MemoryToolHandler func(ctx context.Context, arguments json.RawMessage) (string, error)

type Server struct {
	v5Handler         V5ToolHandler
	onboardHandler    MemoryToolHandler
	entityHandler     MemoryToolHandler
	memoryHandler     MemoryToolHandler
	memorySurface     memorySurfaceMode
	memoryReadHandler MemoryToolHandler
	instructions      string
	version           string
}

// parityPlanMCPSchema delegates to the shared artifact.ParityPlanJSONSchema
// so the MCP-advertised schema and the standalone tool surface stay in
// lock-step on field shape, types, and missing_data_policy enum values.
func parityPlanMCPSchema(description string) map[string]interface{} {
	return artifact.ParityPlanJSONSchema(description)
}

func stringArrayMCPSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":  "array",
		"items": map[string]interface{}{"type": "string"},
	}
}

func objectMCPSchema(properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
}

func objectArrayMCPSchema(properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":  "array",
		"items": objectMCPSchema(properties),
	}
}

func bindingTargetArrayMCPSchema() map[string]interface{} {
	stringField := func() map[string]interface{} {
		return map[string]interface{}{"type": "string"}
	}
	integerField := func() map[string]interface{} {
		return map[string]interface{}{"type": "integer"}
	}
	return map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"kind":              stringField(),
				"target_ref":        stringField(),
				"anchor_id":         stringField(),
				"anchor_version":    integerField(),
				"file_path":         stringField(),
				"language":          stringField(),
				"symbol_name":       stringField(),
				"symbol_kind":       stringField(),
				"receiver":          stringField(),
				"qualified_name":    stringField(),
				"signature_hash":    stringField(),
				"line":              integerField(),
				"end_line":          integerField(),
				"body_hash":         stringField(),
				"anchor_hash":       stringField(),
				"text_hash":         stringField(),
				"nearest_symbol":    stringField(),
				"module_path":       stringField(),
				"module_hash":       stringField(),
				"reason":            stringField(),
				"why_symbol_failed": stringField(),
				"why_range_failed":  stringField(),
				"language_support":  stringField(),
				"confidence":        stringField(),
				"resolution_source": stringField(),
			},
		},
	}
}

func NewServer(version string) *Server {
	return &Server{version: version}
}

// SetV5Handler registers the handler for v5 tools (haft_note, haft_problem, etc).
func (s *Server) SetV5Handler(h V5ToolHandler) {
	s.v5Handler = h
}

// SetOnboardHandler registers the readable task-level setup boundary.
// Detection is read-only; preparation can only materialize a non-binding
// review carrier and cannot apply a profile or enable project memory.
func (s *Server) SetOnboardHandler(h MemoryToolHandler) {
	s.onboardHandler = h
}

// SetEntityHandler registers the task-level EntityOfConcern establishment
// boundary. The handler derives all project-memory coordinates internally.
func (s *Server) SetEntityHandler(h MemoryToolHandler) {
	s.entityHandler = h
}

// SetMemoryHandler registers the dedicated validate-only typed-memory boundary.
// It is deliberately separate from the v5 handler, whose project hooks include
// audit and recall effects that are not part of haft_memory(validate).
func (s *Server) SetMemoryHandler(h MemoryToolHandler) {
	s.setMemorySurface(memorySurfaceValidateOnly, h)
}

// SetMemoryReadHandler registers the closed selected-project read variants
// behind haft_query(action="memory"). It does not widen the dedicated
// haft_memory(validate|admit) mutation boundary.
func (s *Server) SetMemoryReadHandler(h MemoryToolHandler) {
	s.memoryReadHandler = h
}

// SetMemoryFullHandler registers validate plus the non-binding exact-snapshot
// admission action. Project-memory reads remain a separate query capability.
func (s *Server) SetMemoryFullHandler(h MemoryToolHandler) {
	s.setMemorySurface(memorySurfaceFull, h)
}

func (s *Server) setMemorySurface(
	mode memorySurfaceMode,
	handler MemoryToolHandler,
) {
	if handler == nil {
		s.memoryHandler = nil
		s.memorySurface = memorySurfaceUnavailable
		return
	}
	s.memoryHandler = handler
	s.memorySurface = mode
}

func (s *Server) SetInstructions(instructions string) {
	s.instructions = strings.TrimSpace(instructions)
}

func (s *Server) Start() {
	logger.Info().Msg("MCP server starting")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			logger.Warn().Err(err).Int("line_len", len(line)).Msg("JSON-RPC parse error")
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		logger.Debug().Str("method", req.Method).Msg("request received")

		s.handleRequest(req)
	}

	// Scanner exited — log why
	if err := scanner.Err(); err != nil {
		logger.Error().Err(err).Msg("MCP server: scanner error (stdin read failure)")
	} else {
		logger.Info().Msg("MCP server: stdin closed (EOF)")
	}
}

func (s *Server) handleRequest(req JSONRPCRequest) {
	// Recover from panics — log and return error instead of crashing
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Interface("panic", r).Str("method", req.Method).Msg("MCP server: panic recovered")
			if req.ID != nil {
				s.sendError(req.ID, -32603, fmt.Sprintf("internal error: %v", r))
			}
		}
	}()

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "notifications/initialized":
		// No-op
	default:
		if req.ID != nil {
			s.sendError(req.ID, -32601, "Method not found")
		}
	}
}

func (s *Server) send(resp JSONRPCResponse) {
	bytes, err := json.Marshal(resp)
	if err != nil {
		logger.Error().Err(err).Msg("failed to marshal JSON-RPC response")
		return
	}
	if _, err := fmt.Printf("%s\n", string(bytes)); err != nil {
		logger.Error().Err(err).Msg("failed to write to stdout")
	}
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	s.send(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id interface{}, code int, message string) {
	s.send(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	})
}

func (s *Server) handleInitialize(req JSONRPCRequest) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    "haft",
			"version": s.version,
		},
	}

	if s.instructions != "" {
		result["instructions"] = s.instructions
	}

	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	// Several real MCP hosts register only the first tools/list response and do
	// not follow nextCursor. The catalog is therefore intentionally atomic: one
	// response advertises every currently available tool. Client-specific params
	// are ignored because they must never make part of the catalog undiscoverable.
	s.sendResult(req.ID, map[string]interface{}{
		"tools": s.ToolCatalog(),
	})
}

func (s *Server) ToolCatalog() []Tool {
	var tools []Tool

	// v5 tools only
	if s.v5Handler != nil {
		tools = append(tools,
			Tool{
				Name:        "haft_note",
				Description: "Record a project fact/observation carrier; not a decision.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]string{
							"type":        "string",
							"description": "What the fact is (e.g., 'MCP server is per-session, not a daemon')",
						},
						"observations": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "Atomic facts — one fact per entry. The core of a fact note.",
						},
						"anchors": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"type": map[string]string{"type": "string", "description": "Edge label: governs | about | relates_to | implements | supersedes"},
									"ref":  map[string]string{"type": "string", "description": "Target — an artifact ID (dec-/prob-/note-/sol-) OR a code symbol (Name, or Name@file to disambiguate). MUST exist — a dead anchor is rejected."},
								},
								"required": []string{"ref"},
							},
							"description": "Typed edges from this fact to decisions/problems/notes OR code symbols — surface in related/backlinks and code_context.",
						},
						"rationale": map[string]string{
							"type":        "string",
							"description": "OPTIONAL — why, if the fact carries a reasoned judgment. A bare fact needs none.",
						},
						"affected_files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "File paths this fact is about",
						},
						"evidence": map[string]string{
							"type":        "string",
							"description": "Supporting evidence (benchmarks, test results, references)",
						},
						"search_keywords": map[string]string{
							"type":        "string",
							"description": "Space-separated synonyms and related terms for search enrichment (e.g., 'redis cache caching in-memory key-value nosql')",
						},
						"context": map[string]string{
							"type":        "string",
							"description": "Optional context name for grouping (e.g., 'auth', 'payments')",
						},
						"entity_ref": memoryEntityReferenceSchema(),
						"bounded_context_ref": map[string]interface{}{
							"type":        "string",
							"description": "Exact typed-memory bounded context paired with entity_ref. Legacy context is only a grouping label and is never inferred as this coordinate.",
						},
					},
					"required": []string{"title"},
				},
			},
			Tool{
				Name:        "haft_problem",
				Description: "Frame, characterize, select, and close engineering problems.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"frame", "characterize", "select", "close"},
							"description": "frame=create ProblemCard, characterize=add comparison dimensions, select=list/filter active problems, close=mark problem as addressed",
						},
						"title": map[string]string{
							"type":        "string",
							"description": "(frame) Problem title",
						},
						"problem_type": map[string]string{
							"type":        "string",
							"description": "optimization",
						},
						"problem_profile": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"cue", "thin", "deep"},
							"description": "(frame) ProblemCard profile level: cue, thin, or deep. Only deep cards with explicit boundary/probe/freshness can be P2W-ready.",
						},
						"source_kind": map[string]interface{}{
							"type":        "string",
							"enum":        []interface{}{"observed_problem", "wish", "ticket", "chosen_method"},
							"description": "(frame) Source posture: observed_problem, wish, ticket, or chosen_method. Wish/ticket/chosen_method require explicit boundary before P2W readiness.",
						},
						"signal": map[string]string{
							"type":        "string",
							"description": "(frame) What's anomalous, broken, or needs changing",
						},
						"why_now": map[string]string{
							"type":        "string",
							"description": "(frame) Why this problem matters now, not just someday.",
						},
						"scope": map[string]string{
							"type":        "string",
							"description": "(frame) Explicit boundary/scope for what is inside and outside the problem.",
						},
						"acceptance_probe": map[string]string{
							"type":        "string",
							"description": "(frame) Probe that would show the problem is sufficiently bounded/solved.",
						},
						"freshness_disposition": map[string]string{
							"type":        "string",
							"description": "(frame) How fresh/current the problem signal is and when to revisit it.",
						},
						"constraints": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "(frame) Hard constraints that MUST hold",
						},
						"optimization_targets": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "(frame) What to improve (1-3 max)",
						},
						"observation_indicators": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "(frame) What to monitor but NOT optimize (Anti-Goodhart)",
						},
						"acceptance": map[string]string{
							"type":        "string",
							"description": "(frame) How we'll know the problem is solved",
						},
						"blast_radius": map[string]string{
							"type":        "string",
							"description": "(frame) What systems/teams are affected",
						},
						"seed_file": map[string]string{
							"type":        "string",
							"description": "(frame) Optional file context",
						},
						"reversibility": map[string]string{
							"type":        "string",
							"description": "(frame) How easy to undo — low/medium/high",
						},
						"problem_ref": map[string]string{
							"type":        "string",
							"description": "(characterize) ID of the ProblemCard to add dimensions to",
						},
						"dimensions": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name":           map[string]string{"type": "string", "description": "Dimension name (e.g., 'throughput', 'ops complexity')"},
									"scale_type":     map[string]string{"type": "string", "description": "ordinal, ratio, nominal"},
									"unit":           map[string]string{"type": "string", "description": "Measurement unit"},
									"polarity":       map[string]string{"type": "string", "description": "higher_better or lower_better"},
									"role":           map[string]string{"type": "string", "description": "Indicator role: constraint (hard limit), target (optimize), observation (watch, don't optimize). Default: target"},
									"proxy_for":      map[string]string{"type": "string", "description": "Intended value this dimension proxies (FPF E.13 value-before-proxy). Expected for target-role dimensions; kernel warns when absent"},
									"how_to_measure": map[string]string{"type": "string", "description": "How this dimension is measured"},
									"valid_until":    map[string]string{"type": "string", "description": "When this measurement expires (RFC3339). Compare warns on expired dimensions."},
								},
								"required": []string{"name"},
							},
							"description": "(characterize) Comparison dimensions for evaluating solutions",
						},
						"parity_rules": map[string]string{
							"type":        "string",
							"description": "(characterize) Prose rules for what must be equal across variants. Use parity_plan for the structured form required by deep mode.",
						},
						"parity_plan": parityPlanMCPSchema("(characterize) Structured parity plan that downstream compare can enforce. Object with baseline_set, window, budget, missing_data_policy and optional normalization / pinned_conditions per FPF G.9:4.2."),
						"context": map[string]string{
							"type":        "string",
							"description": "Optional context name for grouping",
						},
						"entity_ref": memoryEntityReferenceSchema(),
						"bounded_context_ref": map[string]interface{}{
							"type":        "string",
							"description": "(frame) Exact typed-memory bounded context paired with entity_ref. Legacy context remains only a grouping label.",
						},
						"mode": map[string]string{
							"type":        "string",
							"description": "(frame) Decision mode: tactical, standard (default), deep",
						},
					},
					"required": []string{"action"},
				},
			},
		)

		tools = append(tools, Tool{
			Name:        "haft_solution",
			Description: "Explore or compare solution variants. With exact EntityOfConcern coordinates and already-addressable option records, the task result is also projected into typed project memory without selecting a winner.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"explore", "compare", "similar"},
						"description": "explore=create variants, compare=parity comparison, similar=search",
					},
					"query": map[string]string{
						"type":        "string",
						"description": "(similar) Search query for past solution portfolios",
					},
					"problem_ref": map[string]string{
						"type":        "string",
						"description": "(explore) ProblemCard ID this portfolio solves. Auto-detected if only one active.",
					},
					"variants": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id":                   map[string]string{"type": "string", "description": "Explicit variant ID (auto-generated if omitted)"},
								"title":                map[string]string{"type": "string", "description": "Variant name"},
								"description":          map[string]string{"type": "string", "description": "What this option does"},
								"weakest_link":         map[string]string{"type": "string", "description": "What bounds this option's quality (WLNK)"},
								"novelty_marker":       map[string]string{"type": "string", "description": "How this variant differs from the others — state the genuine novelty"},
								"strengths":            map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
								"risks":                map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
								"stepping_stone":       map[string]interface{}{"type": "boolean", "description": "Opens future possibilities even if not optimal now"},
								"stepping_stone_basis": map[string]string{"type": "string", "description": "Why this is a stepping stone (required when stepping_stone=true)"},
								"diversity_role":       map[string]string{"type": "string", "description": "Role in portfolio diversity"},
								"assumption_notes":     map[string]string{"type": "string", "description": "Key assumptions this variant depends on"},
								"rollback_notes":       map[string]string{"type": "string"},
								"evidence_refs":        map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "References to supporting evidence"},
								"project_record_ref":   memoryProjectRecordReferenceSchema(),
							},
							"required": []string{"title", "weakest_link", "novelty_marker"},
						},
						"description": "(explore) at least 2 distinct variants",
					},
					"entity_ref": memoryEntityReferenceSchema(),
					"bounded_context_ref": map[string]interface{}{
						"type":        "string",
						"description": "(explore/compare) Exact typed-memory bounded context paired with entity_ref. Legacy context remains only a grouping label.",
					},
					"no_stepping_stone_rationale": map[string]string{
						"type":        "string",
						"description": "(explore) Required when no variant is a stepping stone — explain why",
					},
					"portfolio_ref": map[string]string{
						"type":        "string",
						"description": "(compare) SolutionPortfolio ID to add comparison results to. Its typed portfolio record and option records must already resolve for typed PortfolioComparison projection; otherwise the legacy comparison remains durable but typed projection abstains.",
					},
					"dimensions": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "(compare) Comparison dimension names",
					},
					"scores": map[string]interface{}{
						"type":        "object",
						"description": "(compare) Scores per variant: {\"V1\": {\"throughput\": \"100k/s\", \"cost\": \"$200\"}}",
						"additionalProperties": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": map[string]string{"type": "string"},
						},
					},
					"non_dominated_set": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "(compare) Advisory Pareto-front claim; runtime computes and stores the front from scores",
					},
					"incomparable": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
						"description": "(compare) Pairs that are intentionally incomparable",
					},
					"dominated_variants": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"variant": map[string]string{
									"type": "string",
								},
								"dominated_by": map[string]interface{}{
									"type":  "array",
									"items": map[string]string{"type": "string"},
								},
								"summary": map[string]string{
									"type": "string",
								},
							},
						},
						"description": "(compare) Persisted elimination notes for dominated variants",
					},
					"pareto_tradeoffs": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"variant": map[string]string{
									"type": "string",
								},
								"summary": map[string]string{
									"type": "string",
								},
							},
						},
						"description": "(compare) Persisted trade-off notes for Pareto-front variants",
					},
					"policy_applied": map[string]string{
						"type":        "string",
						"description": "(compare) Selection policy that was applied",
					},
					"parity_plan": parityPlanMCPSchema("(compare) Structured parity plan. REQUIRED for deep mode: baseline_set, window, budget, and missing_data_policy MUST be present. Standard/tactical modes accept any subset and warn on gaps. Per FPF G.9:4.2."),
					"selected_ref": map[string]string{
						"type":        "string",
						"description": "(compare) Legacy advisory variant ID. Excluded from typed PortfolioComparison; never a winner, ChoiceResult, or DecisionRecord.",
					},
					"legacy_recommendation_ref": map[string]string{
						"type":        "string",
						"description": "Advisory recommendation only; excluded from typed PortfolioComparison.",
					},
					"recommendation_rationale": map[string]string{
						"type":        "string",
						"description": "(compare) Why the advisory rec is preferred",
					},
					"context": map[string]string{
						"type":        "string",
						"description": "Optional context name",
					},
					"mode": map[string]string{
						"type":        "string",
						"description": "(explore/compare) tactical, standard, or deep",
					},
				},
				"required": []string{"action"},
			},
		})
		tools = append(tools, Tool{
			Name:        "haft_decision",
			Description: "Manage decisions; MCP binding creation fails closed.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"decide", "apply", "measure", "evidence", "baseline"},
						"description": "Decision lifecycle action",
					},
					"selected_title": map[string]string{
						"type": "string",
					},
					"why_selected": map[string]string{
						"type": "string",
					},
					"choice_result": map[string]interface{}{
						"properties": map[string]interface{}{
							"subject_ref":      map[string]interface{}{},
							"option_set":       map[string]interface{}{},
							"comparison_basis": map[string]interface{}{},
							"choice_rule":      map[string]interface{}{},
							"next_move": map[string]interface{}{
								"type": "string",
								"enum": []interface{}{"choose_now", "reject_current_set", "probe_again", "reroute"},
							},
							"variant_ref":      map[string]interface{}{},
							"problem_refs":     map[string]interface{}{},
							"portfolio_ref":    map[string]interface{}{},
							"reason":           map[string]interface{}{},
							"reversibility":    map[string]interface{}{},
							"reopen_condition": map[string]interface{}{},
						},
					},
					"transformation_record": map[string]interface{}{
						"properties": map[string]interface{}{
							"schema_version":     map[string]interface{}{},
							"transformed_entity": map[string]interface{}{},
							"initial_state":      map[string]interface{}{},
							"post_state":         map[string]interface{}{},
							"relation":           map[string]interface{}{},
							"context":            map[string]interface{}{},
							"window":             map[string]interface{}{},
							"method_refs":        map[string]interface{}{},
							"work_refs":          map[string]interface{}{},
							"evidence_refs":      map[string]interface{}{},
							"publication_refs":   map[string]interface{}{},
						},
					},
					"selection_policy": map[string]string{
						"type": "string",
					},
					"counterargument": map[string]string{
						"type": "string",
					},
					"why_not_others": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"variant": map[string]string{"type": "string"},
								"reason":  map[string]string{"type": "string"},
							},
						},
						"description": "(decide) At least one key rejected alternative and why it lost",
					},
					"invariants": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) What MUST hold at all times",
					},
					"pre_conditions": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) What MUST be true before implementation",
					},
					"post_conditions": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) What MUST be true after implementation",
					},
					"admissibility": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) What is NOT acceptable",
					},
					"evidence_requirements": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) What to measure/prove during implementation",
					},
					"rollback": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"triggers":     map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
							"steps":        map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
							"blast_radius": map[string]string{"type": "string"},
						},
						"description": "(decide) When and how to reverse; at least one trigger is required",
					},
					"refresh_triggers": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) When to re-evaluate this decision",
					},
					"section_refs": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide) SpecSection IDs governed by this DecisionRecord",
					},
					"spec_binding_preflight": map[string]interface{}{
						"properties": map[string]interface{}{
							"schema_version":        map[string]interface{}{},
							"record_kind":           map[string]interface{}{},
							"state":                 map[string]interface{}{},
							"selected_section_refs": map[string]interface{}{},
							"status_debt":           map[string]interface{}{},
						},
						"description": "(decide) spec_binding_preflight receipt; incompatible states fail closed.",
					},
					"spec_binding_preflight_required": map[string]interface{}{
						"type":        "boolean",
						"description": "(decide) Require compatible spec_binding_preflight without globally requiring section_refs.",
					},
					"predictions": map[string]interface{}{
						"type":        "array",
						"description": "(decide) Testable predictions — measure will check each one",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"claim":         map[string]string{"type": "string"},
								"observable":    map[string]string{"type": "string"},
								"threshold":     map[string]string{"type": "string"},
								"verify_after":  map[string]string{"type": "string", "description": "When to check (RFC3339 or YYYY-MM-DD) — for async claims"},
								"realizability": map[string]string{"type": "string"},
								"probability":   map[string]string{"type": "number", "description": "Optional p(claim holds) in [0,1]."},
								"command":       map[string]string{"type": "string", "description": "Optional allowlisted verification command."},
							},
							"required": []string{"claim", "observable", "threshold"},
						},
					},
					"weakest_link": map[string]string{
						"type": "string",
					},
					"problem_ref": map[string]string{
						"type": "string", "description": "(decide) Single ProblemCard ID. Use problem_refs for multiple.",
					},
					"problem_statement": map[string]interface{}{},
					"problem_refs": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "(decide) ProblemCard IDs this decision addresses — supports multiple problems",
					},
					"portfolio_ref": map[string]string{
						"type": "string", "description": "(decide) SolutionPortfolio ID",
					},
					"decision_ref": map[string]string{
						"type": "string", "description": "(apply) DecisionRecord ID to generate brief from",
					},
					"valid_until": map[string]string{
						"type": "string", "description": "Expiry date",
					},
					"affected_files": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide/baseline) Affected files.",
					},
					"decision_subject_ref": map[string]string{
						"type": "string",
					},
					"implementation_footprint": objectMCPSchema(map[string]interface{}{
						"files":     stringArrayMCPSchema(),
						"commits":   stringArrayMCPSchema(),
						"work_refs": stringArrayMCPSchema(),
					}),
					"governance_targets": objectArrayMCPSchema(map[string]interface{}{
						"kind":           map[string]interface{}{},
						"ref":            map[string]interface{}{},
						"binding_target": map[string]interface{}{},
					}),
					"drift_watch_targets": objectArrayMCPSchema(map[string]interface{}{
						"target_ref":     map[string]interface{}{},
						"trigger":        map[string]interface{}{},
						"binding_target": map[string]interface{}{},
					}),
					"claims": objectArrayMCPSchema(map[string]interface{}{
						"id":                     map[string]interface{}{},
						"claim":                  map[string]interface{}{},
						"observable":             map[string]interface{}{},
						"threshold":              map[string]interface{}{},
						"lifecycle_status":       map[string]interface{}{},
						"successor_ref":          map[string]interface{}{},
						"retired_reason":         map[string]interface{}{},
						"governance_target_refs": stringArrayMCPSchema(),
					}),
					"binding_hints": stringArrayMCPSchema(),
					"binding_scope": map[string]string{
						"type": "string",
					},
					"binding_fallback_reason": map[string]string{
						"type": "string",
					},
					"binding_targets": bindingTargetArrayMCPSchema(),
					"search_keywords": map[string]string{
						"type":        "string",
						"description": "(decide) Space-separated synonyms and related terms for search enrichment",
					},
					"task_context": map[string]string{
						"type":        "string",
						"description": "DecisionRecord ID filename",
					},
					"findings": map[string]string{
						"type": "string", "description": "(measure) What actually happened after implementation",
					},
					"criteria_met": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(measure) Acceptance criteria that were met",
					},
					"criteria_not_met": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(measure) Acceptance criteria NOT met",
					},
					"measurements": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(measure) Measured values (e.g., 'p99 latency: 42ms')",
					},
					"verdict": map[string]string{
						"type": "string", "description": "(measure) accepted, partial, or failed",
					},
					"artifact_ref": map[string]string{
						"type": "string", "description": "(evidence) Artifact ID to attach evidence to",
					},
					"evidence_content": map[string]string{
						"type": "string", "description": "(evidence) The evidence itself",
					},
					"evidence_type": map[string]string{
						"type": "string", "description": "(evidence) measurement, test, research, benchmark, audit",
					},
					"evidence_verdict": map[string]string{
						"type": "string", "description": "(evidence) supports, weakens, refutes",
					},
					"carrier_ref": map[string]string{
						"type": "string", "description": "(evidence) File path or URL of evidence source",
					},
					"congruence_level": map[string]interface{}{
						"type": "integer", "description": "(evidence) CL 0-3: 3=same context, 2=similar, 1=different, 0=opposed",
					},
					"formality_level": map[string]interface{}{
						"type": "integer", "description": "(evidence) F0-F9 current FPF; use formality_scale_id; display not approval/gate passage/claim truth/global truth/publication",
					},
					"formality_scale_id": map[string]string{
						"type": "string", "description": "(evidence) scale: fpf-2026-f0-f9, legacy bridge/loss, or unversioned-formality gap",
					},
					"claim_refs": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "(evidence) Exact decision claim IDs this evidence binds to when available",
					},
					"claim_scope": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "(evidence) Fallback claim scope labels for older artifacts or non-claim evidence",
					},
					"causal_support_basis": map[string]string{
						"type":        "string",
						"description": "C.28 simulation_only",
					},
					"_skips": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "_skip_reason",
					},
					"_skip": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "_skip_reason",
					},
					"_skip_reason": map[string]string{
						"type":        "string",
						"description": "_skip_reason!",
					},
					"context": map[string]string{"type": "string", "description": "Optional context name"},
					"mode":    map[string]string{"type": "string", "description": "(decide) tactical, standard (default), deep"},
				},
				"required": []string{"action"},
			},
		})
		tools = append(tools, Tool{
			Name:        "haft_refresh",
			Description: "Manage artifact lifecycle, stale scans, safe drain, and review packets.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"scan", "plan", "review", "drain", "waive", "reopen", "supersede", "deprecate", "reconcile"},
						"description": "scan stale, plan work order, review judgment packet, drain machine-safe actions, waive/reopen/supersede/deprecate/reconcile.",
					},
					"artifact_ref": map[string]string{
						"type":        "string",
						"description": "Artifact ID to act on — any kind: note, problem, decision, portfolio (required for waive/reopen/supersede/deprecate)",
					},
					"decision_ref": map[string]string{
						"type":        "string",
						"description": "Deprecated: use artifact_ref instead. Kept for backward compatibility.",
					},
					"reason": map[string]string{
						"type":        "string",
						"description": "Why this refresh action is being taken",
					},
					"new_valid_until": map[string]string{
						"type":        "string",
						"description": "(waive) New expiry date in RFC3339 format. Default: +90 days.",
					},
					"evidence": map[string]string{
						"type":        "string",
						"description": "(waive) Evidence supporting the extension",
					},
					"new_decision_ref": map[string]string{
						"type":        "string",
						"description": "(supersede) ID of the replacement artifact. Deprecated: use new_artifact_ref.",
					},
					"new_artifact_ref": map[string]string{
						"type":        "string",
						"description": "(supersede) ID of the artifact replacing this one",
					},
					"context": map[string]string{
						"type":        "string",
						"description": "Optional context filter for scan",
					},
					"verbose": map[string]string{
						"type":        "boolean",
						"description": "(scan) Include full per-file drift dump. Default false — drift is summarized as counts + top-5 modified paths per decision. Full mode can exceed context budget on repos with vendor subtrees or large added-files sets.",
					},
					"dry_run": map[string]string{
						"type":        "boolean",
						"description": "(drain) Propose machine-safe actions without mutating baselines, evidence, or stored maintenance runs.",
					},
				},
				"required": []string{"action"},
			},
		})

		tools = append(tools, Tool{
			Name:        "haft_query",
			Description: "Read project state.",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"search", "status", "board", "related", "code_context", "callees", "callers", "impact", "node", "explore", "ceremony", "projection", "list", "coverage", "fpf", "check", "carrier_manifest", "carrier_check", "contract_audit", "contract_generation", "spec_review", "spec_use", "spec_trace", "spec_binding_preflight", "spec_fit_probe", "change_case", "correspondence_graph", "drift_route", "drift_events", "decision_reconcile", "governing_set", "blocked_use", "value_space", "evidence_path", "resolve_term"},
					},
					"query": map[string]interface{}{"type": "string"},
					"identifier": map[string]interface{}{
						"type":        "string",
						"description": "FPF source identifier only for action=fpf mode=lookup or mode=inspect. A Haft artifact ID or code symbol is a wrong_identifier_namespace; use artifact_ref with action=related or symbol with action=node.",
					},
					"entity_of_concern": map[string]interface{}{"type": "string"},
					"scope_id": map[string]interface{}{
						"type":        "string",
						"description": "(status, coverage) Exact canonical project ScopeID. Required when the admitted profile has several scopes.",
					},
					"known_context": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"intended_use": map[string]interface{}{"type": "string"},
					"roles": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"max_candidates_per_role":     map[string]interface{}{"type": "integer", "minimum": 0},
					"max_total_candidates":        map[string]interface{}{"type": "integer", "minimum": 0},
					"max_excerpt_characters":      map[string]interface{}{"type": "integer", "minimum": 0},
					"max_relations_per_candidate": map[string]interface{}{"type": "integer", "minimum": 0},
					"max_candidates": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"maximum":     50,
						"description": "For action=explore with query, bounded canonical candidate retrieval before the working projection caps visible candidates at five.",
					},
					"term":           map[string]interface{}{},
					"section_id":     map[string]interface{}{},
					"decision_draft": map[string]interface{}{},
					"probe": map[string]interface{}{
						"type":        "object",
						"description": "(spec_fit_probe) Problem/scope/variant draft.",
						"properties": map[string]interface{}{
							"problem_signal":    map[string]interface{}{},
							"scope":             map[string]interface{}{},
							"mode":              map[string]interface{}{},
							"section_refs":      map[string]interface{}{},
							"affected_files":    map[string]interface{}{},
							"target_refs":       map[string]interface{}{},
							"conflict_refs":     map[string]interface{}{},
							"declared_relation": map[string]interface{}{},
						},
					},
					"problem_signal":    map[string]interface{}{},
					"scope":             map[string]interface{}{},
					"section_refs":      map[string]interface{}{},
					"affected_files":    map[string]interface{}{},
					"target_refs":       map[string]interface{}{},
					"conflict_refs":     map[string]interface{}{},
					"declared_relation": map[string]interface{}{},
					"variants": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"properties": map[string]interface{}{
								"id":                map[string]interface{}{},
								"title":             map[string]interface{}{},
								"description":       map[string]interface{}{},
								"section_refs":      map[string]interface{}{},
								"affected_files":    map[string]interface{}{},
								"target_refs":       map[string]interface{}{},
								"conflict_refs":     map[string]interface{}{},
								"declared_relation": map[string]interface{}{},
							},
						},
					},
					"use_context": map[string]interface{}{},
					"policy": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"documentary_only", "stronger_use_requires_current_source", "temporary_waiver"},
						"description": "(spec_use) Admission policy; currentness/waiver/gate stay separate.",
					},
					"waiver_expires_at": map[string]interface{}{},
					"operational_gate": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"schema_version":   map[string]interface{}{},
							"gate_ref":         map[string]interface{}{},
							"bearer_ref":       map[string]interface{}{},
							"use_context":      map[string]interface{}{},
							"rule":             map[string]interface{}{},
							"evidence_refs":    stringArrayMCPSchema(),
							"expires_at":       map[string]interface{}{},
							"reopen_condition": map[string]interface{}{},
						},
						"description": "(spec_use) Optional OperationalGate v1.",
					},
					"artifact_ref": map[string]interface{}{
						"type":        "string",
						"description": "Canonical Haft artifact ID for action=related exact recovery. Code symbols use symbol; FPF source IDs use identifier. A wrong_identifier_namespace response supplies the exact recovery_call.",
					},
					"ref": map[string]interface{}{
						"description": "(related) Backward-compatible alias for artifact_ref.",
					},
					"artifact_id": map[string]interface{}{
						"description": "(related) Backward-compatible alias for artifact_ref.",
					},
					"evidence_ref":               map[string]interface{}{},
					"claim_ref":                  map[string]interface{}{},
					"attempted_use":              map[string]interface{}{},
					"requires_current_formality": map[string]interface{}{},
					"producer_ref":               map[string]interface{}{},
					"method_ref":                 map[string]interface{}{},
					"work_ref":                   map[string]interface{}{},
					"drift_kind":                 map[string]interface{}{},
					"bearer_ref":                 map[string]interface{}{},
					"label":                      map[string]interface{}{},
					"blocked_use":                map[string]interface{}{},
					"source_refs":                map[string]interface{}{},
					"exact_record_needed":        map[string]interface{}{},
					"kind":                       map[string]interface{}{},
					"file":                       map[string]interface{}{},
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "For action=node, a code symbol only. A canonical Haft artifact ID returns wrong_identifier_namespace; recover it with haft_query(action=related, artifact_ref=<id>), not another node call.",
					},
					"anchor_id": map[string]interface{}{},
					"line":      map[string]interface{}{},
					"lane": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"index", "symbols", "decisions", "invariants", "notes", "problems", "portfolios", "all"},
					},
					"depth":   map[string]interface{}{},
					"profile": map[string]interface{}{},
					"files":   stringArrayMCPSchema(),
					"context": map[string]interface{}{},
					"view": map[string]interface{}{
						"type":        "string",
						"description": "For action=fpf and action=explore, publication projection views are working (default), trace, or diagnostic. Other haft_query actions retain their own view contracts, so this shared field is not a global enum.",
					},
					"trace_ref": map[string]interface{}{
						"type":        "string",
						"description": "For action=fpf or action=explore with view=trace or view=diagnostic, trace_ref is the opaque replay reference from an earlier response. FPF source-snapshot or typed-request drift returns replay_mismatch before retrieval; Explore index/request/result drift returns replay_mismatch. working view rejects trace_ref.",
					},
					"limit": map[string]interface{}{},
					"full": map[string]interface{}{
						"description": "(search) Exact artifact payload only.",
					},
					"mode": map[string]interface{}{
						"type": "string",
						"enum": []interface{}{"concern", "lookup", "inspect", "tactical", "standard", "deep"},
					},
				},
				"required": []string{"action"},
			},
		})

		tools = append(tools, haftMethodTool())
		tools = append(tools, haftCommissionTool())
		tools = append(tools, haftSpecSectionTool())
	}
	// These task-level and typed-memory contracts are stable discovery
	// surfaces. They remain advertised before project onboarding or memory
	// enablement; the configured handlers return a closed recovery result
	// instead of making the tools disappear from tools/list.
	tools = installMemoryQuerySchema(tools)
	tools = append(
		tools,
		haftOnboardTool(),
		haftEntityTool(),
		haftMemoryFullTool(),
	)

	return compactToolDescriptions(tools)
}

func compactToolDescriptions(tools []Tool) []Tool {
	compacted := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		tool.Description = compactToolDescription(tool.Name, tool.Description)
		if tool.Name != "haft_onboard" &&
			tool.Name != "haft_entity" {
			compactSchemaDescriptions(tool.InputSchema)
		}
		compacted = append(compacted, tool)
	}

	return compacted
}

func compactToolDescription(toolName, fallback string) string {
	if toolName == "haft_memory" {
		return strings.TrimSpace(fallback)
	}
	descriptions := map[string]string{
		"haft_note":         "Record a non-binding project fact.",
		"haft_problem":      "Frame or update an engineering problem.",
		"haft_solution":     "Explore or compare solution variants.",
		"haft_decision":     "Manage decisions; human-gated binding needs a full choice brief.",
		"haft_refresh":      "Inspect or update artifact lifecycle.",
		"haft_query":        "Read project state or FPF source; expand human gates into briefs.",
		"haft_onboard":      "Inspect setup or prepare a non-binding onboarding review.",
		"haft_entity":       "Establish one non-binding EntityOfConcern with atomic aliases.",
		"haft_method":       "Pull, inspect, or close a SWE MethodRun.",
		"haft_commission":   "Manage commissions; human-gated authority needs a full scope brief.",
		"haft_spec_section": "Spec lifecycle; FPF source fit is separate; human-gated acts need briefs",
	}
	description, ok := descriptions[toolName]
	if ok {
		return description
	}
	return strings.TrimSpace(fallback)
}

func compactSchemaDescriptions(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		if description, ok := typed["description"].(string); ok {
			if !isLoadBearingSchemaDescription(description) {
				delete(typed, "description")
			}
		}
		for _, child := range typed {
			compactSchemaDescriptions(child)
		}
	case map[string]string:
		if description, ok := typed["description"]; ok {
			if !isLoadBearingSchemaDescription(description) {
				delete(typed, "description")
			}
		}
	case []interface{}:
		for _, child := range typed {
			compactSchemaDescriptions(child)
		}
	}
}

func isLoadBearingSchemaDescription(description string) bool {
	for _, marker := range []string{
		"C.28",
		"DecisionRecord ID filename",
		"Expiry date",
		"_skip_reason",
		"tree mode",
		"projection views",
		"Advisory recommendation only",
		"Excluded from typed PortfolioComparison",
		"explore/compare",
		"Lane. Default index",
		"lane=index",
		"optimization",
		"simulation_only",
		"strict typed-memory decoder",
		"wrong_identifier_namespace",
		"publication projection views",
		"trace_ref",
	} {
		if strings.Contains(description, marker) {
			return true
		}
	}

	return false
}

func (s *Server) handleToolsCall(req JSONRPCRequest) {
	ctx := context.Background()

	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32700, "Invalid params")
		return
	}

	if params.Name == "haft_onboard" {
		s.handleDedicatedToolCall(
			req.ID,
			ctx,
			params.Arguments,
			s.onboardHandler,
			"Haft onboarding is unavailable",
		)
		return
	}

	if params.Name == "haft_entity" {
		s.handleDedicatedToolCall(
			req.ID,
			ctx,
			params.Arguments,
			s.entityHandler,
			"Haft EntityOfConcern establishment is unavailable",
		)
		return
	}

	if params.Name == "haft_query" {
		action, err := decodeMemoryToolAction(params.Arguments)
		if err != nil {
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{
					Type: "text",
					Text: "Invalid haft_query request: " + err.Error(),
				}},
				IsError: true,
			})
			return
		}
		if action == memoryQueryAction {
			if s.memoryReadHandler == nil {
				s.sendResult(req.ID, CallToolResult{
					Content: []ContentItem{{
						Type: "text",
						Text: "Haft project-memory reads are unavailable",
					}},
					IsError: true,
				})
				return
			}
			output, err := s.memoryReadHandler(ctx, params.Arguments)
			if err != nil {
				s.sendResult(req.ID, CallToolResult{
					Content: []ContentItem{{Type: "text", Text: err.Error()}},
					IsError: true,
				})
				return
			}
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{Type: "text", Text: output}},
			})
			return
		}
	}

	if params.Name == "haft_memory" {
		if s.memoryHandler == nil {
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{Type: "text", Text: "Haft memory validation is unavailable"}},
				IsError: true,
			})
			return
		}

		memoryRequest, err := unwrapMemoryToolRequest(params.Arguments)
		if err != nil {
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{
					Type: "text",
					Text: "Invalid haft_memory request: " + err.Error(),
				}},
				IsError: true,
			})
			return
		}
		action, err := decodeMemoryToolAction(memoryRequest)
		if err != nil {
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{
					Type: "text",
					Text: "Invalid haft_memory request: " + err.Error(),
				}},
				IsError: true,
			})
			return
		}
		if !s.memorySurface.allowsAction(action) {
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{
					Type: "text",
					Text: fmt.Sprintf(
						"Haft memory action %q is unavailable on the configured surface",
						action,
					),
				}},
				IsError: true,
			})
			return
		}

		output, err := s.memoryHandler(ctx, memoryRequest)
		if err != nil {
			s.sendResult(req.ID, CallToolResult{
				Content: []ContentItem{{Type: "text", Text: err.Error()}},
				IsError: true,
			})
			return
		}
		s.sendResult(req.ID, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: output}},
		})
		return
	}

	// All tools are handled by the v5 handler
	if s.v5Handler == nil {
		s.sendResult(req.ID, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: "Haft not initialized. Run: haft init"}},
			IsError: true,
		})
		return
	}

	output, err := s.v5Handler(ctx, params.Name, req.Params)
	if err != nil {
		s.sendResult(req.ID, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	} else {
		s.sendResult(req.ID, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: output}},
		})
	}
}

func (s *Server) handleDedicatedToolCall(
	id interface{},
	ctx context.Context,
	arguments json.RawMessage,
	handler MemoryToolHandler,
	unavailable string,
) {
	if handler == nil {
		s.sendResult(id, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: unavailable}},
			IsError: true,
		})
		return
	}
	output, err := handler(ctx, arguments)
	if err != nil {
		s.sendResult(id, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}
	s.sendResult(id, CallToolResult{
		Content: []ContentItem{{Type: "text", Text: output}},
	})
}

func unwrapMemoryToolRequest(
	arguments json.RawMessage,
) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("arguments must be a JSON object")
	}

	request := json.RawMessage(nil)
	requestPresent := false
	for decoder.More() {
		rawName, nameErr := decoder.Token()
		if nameErr != nil {
			return nil, fmt.Errorf("arguments are malformed")
		}
		name, nameOK := rawName.(string)
		if !nameOK {
			return nil, fmt.Errorf("arguments contain a non-string field name")
		}
		rawValue := json.RawMessage(nil)
		if decodeErr := decoder.Decode(&rawValue); decodeErr != nil {
			return nil, fmt.Errorf("arguments field %q is malformed", name)
		}
		if name != "request" {
			return nil, fmt.Errorf("arguments field %q is not allowed", name)
		}
		if requestPresent {
			return nil, fmt.Errorf("request field is duplicated")
		}
		request = append(json.RawMessage(nil), rawValue...)
		requestPresent = true
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, fmt.Errorf("arguments are malformed")
	}
	trailing := json.RawMessage(nil)
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("arguments contain trailing data")
	}
	if !requestPresent {
		return nil, fmt.Errorf("request is required")
	}
	return request, nil
}

func decodeMemoryToolAction(
	arguments json.RawMessage,
) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	start, err := decoder.Token()
	if err != nil {
		return "", fmt.Errorf("arguments must be a JSON object")
	}
	if start != json.Delim('{') {
		return "", fmt.Errorf("arguments must be a JSON object")
	}

	action := ""
	actionPresent := false
	for decoder.More() {
		rawName, nameErr := decoder.Token()
		if nameErr != nil {
			return "", fmt.Errorf("arguments are malformed")
		}
		name, nameOK := rawName.(string)
		if !nameOK {
			return "", fmt.Errorf("arguments contain a non-string field name")
		}
		rawValue := json.RawMessage(nil)
		if decodeErr := decoder.Decode(&rawValue); decodeErr != nil {
			return "", fmt.Errorf("arguments field %q is malformed", name)
		}
		if name != "action" {
			continue
		}
		if actionPresent {
			return "", fmt.Errorf("action field is duplicated")
		}
		if decodeErr := json.Unmarshal(rawValue, &action); decodeErr != nil {
			return "", fmt.Errorf("action must be a string")
		}
		actionPresent = true
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return "", fmt.Errorf("arguments are malformed")
	}
	trailing := json.RawMessage(nil)
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", fmt.Errorf("arguments contain trailing data")
	}
	if !actionPresent || action == "" {
		return "", fmt.Errorf("action is required")
	}
	return action, nil
}
