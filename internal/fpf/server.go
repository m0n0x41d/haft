package fpf

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
	Description string      `json:"description"`
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

type Server struct {
	v5Handler    V5ToolHandler
	instructions string
}

// parityPlanMCPSchema delegates to the shared artifact.ParityPlanJSONSchema
// so the MCP-advertised schema and the standalone tool surface stay in
// lock-step on field shape, types, and missing_data_policy enum values.
func parityPlanMCPSchema(description string) map[string]interface{} {
	return artifact.ParityPlanJSONSchema(description)
}

func NewServer() *Server {
	return &Server{}
}

// SetV5Handler registers the handler for v5 tools (haft_note, haft_problem, etc).
func (s *Server) SetV5Handler(h V5ToolHandler) {
	s.v5Handler = h
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
			"version": "5.0.0",
		},
	}

	if s.instructions != "" {
		result["instructions"] = s.instructions
	}

	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	var tools []Tool

	// v5 tools only
	if s.v5Handler != nil {
		tools = append(tools,
			Tool{
				Name:        "haft_note",
				Description: "Record a micro-decision with rationale. Validates before recording: checks for missing rationale, conflicts with active decisions, and whether the scope is too large for a note. Use for quick engineering choices during coding.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title": map[string]string{
							"type":        "string",
							"description": "What was decided (e.g., 'RWMutex over channels for session cache')",
						},
						"rationale": map[string]string{
							"type":        "string",
							"description": "Why this choice — what alternatives existed, what evidence supports it",
						},
						"affected_files": map[string]interface{}{
							"type":        "array",
							"items":       map[string]string{"type": "string"},
							"description": "File paths affected by this decision",
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
					},
					"required": []string{"title", "rationale"},
				},
			},
			Tool{
				Name:        "haft_problem",
				Description: "Frame, characterize, and manage engineering problems. Actions: 'frame' creates a ProblemCard, 'characterize' adds comparison dimensions, 'select' lists active problems, 'close' marks a problem as addressed. Frame the problem BEFORE exploring solutions.",
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
							"description": "(frame) Problem type: optimization, diagnosis, search, or synthesis",
						},
						"signal": map[string]string{
							"type":        "string",
							"description": "(frame) What's anomalous, broken, or needs changing",
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
			Description: "Explore solution variants and compare them fairly. Actions: 'explore' creates a SolutionPortfolio with >=2 variants (each with weakest link and novelty marker), 'compare' runs parity check and identifies the Pareto front, 'similar' searches past solution portfolios.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"explore", "compare", "similar"},
						"description": "explore=create variants portfolio, compare=run parity comparison, similar=search past solutions",
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
							},
							"required": []string{"title", "weakest_link", "novelty_marker"},
						},
						"description": "(explore) Solution variants — at least 2, genuinely distinct",
					},
					"no_stepping_stone_rationale": map[string]string{
						"type":        "string",
						"description": "(explore) Required when no variant is a stepping stone — explain why",
					},
					"portfolio_ref": map[string]string{
						"type":        "string",
						"description": "(compare) SolutionPortfolio ID to add comparison results to. Auto-detected if only one active.",
					},
					"dimensions": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "(compare) Comparison dimension names",
					},
					"scores": map[string]interface{}{
						"type":        "object",
						"description": "(compare) Scores per variant: {\"V1\": {\"throughput\": \"100k/s\", \"cost\": \"$200\"}}",
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
						"description": "(compare) Advisory recommendation variant ID; the human still chooses",
					},
					"recommendation_rationale": map[string]string{
						"type":        "string",
						"description": "(compare) Why the advisory recommendation is preferred under the declared policy",
					},
					"context": map[string]string{
						"type":        "string",
						"description": "Optional context name",
					},
					"mode": map[string]string{
						"type":        "string",
						"description": "(explore/compare) Decision mode: tactical, standard (default), deep. Deep mode requires structured parity_plan.",
					},
				},
				"required": []string{"action"},
			},
		})
		tools = append(tools, Tool{
			Name:        "haft_decision",
			Description: "Manage the decision lifecycle. Actions: 'decide' creates a DecisionRecord, 'apply' generates implementation brief, 'measure' records post-implementation impact, 'evidence' attaches evidence to any artifact, 'baseline' snapshots affected files for drift detection.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"decide", "apply", "measure", "evidence", "baseline"},
						"description": "decide=create DRR, apply=impl brief, measure=record impact, evidence=attach evidence item, baseline=snapshot affected files for drift detection",
					},
					"selected_title": map[string]string{
						"type":        "string",
						"description": "(decide) Name of the selected variant",
					},
					"why_selected": map[string]string{
						"type":        "string",
						"description": "(decide) Why this variant was chosen",
					},
					"selection_policy": map[string]string{
						"type":        "string",
						"description": "(decide) Explicit policy used to choose the winning variant",
					},
					"counterargument": map[string]string{
						"type":        "string",
						"description": "(decide) Strongest argument against the chosen option",
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
					"predictions": map[string]interface{}{
						"type":        "array",
						"description": "(decide) Testable predictions — measure will check each one",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"claim":      map[string]string{"type": "string"},
								"observable": map[string]string{"type": "string"},
								"threshold":  map[string]string{"type": "string"},
							},
							"required": []string{"claim", "observable", "threshold"},
						},
					},
					"weakest_link": map[string]string{
						"type":        "string",
						"description": "(decide) Selected variant weakest link — what most plausibly breaks this choice",
					},
					"problem_ref": map[string]string{
						"type": "string", "description": "(decide) Single ProblemCard ID. Use problem_refs for multiple.",
					},
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
						"type": "string", "description": "(decide/evidence) Expiry date (RFC3339 or YYYY-MM-DD)",
					},
					"affected_files": map[string]interface{}{
						"type": "array", "items": map[string]string{"type": "string"},
						"description": "(decide/baseline) Files affected by this decision. For baseline: optional — if provided, replaces the file list before snapshotting.",
					},
					"search_keywords": map[string]string{
						"type":        "string",
						"description": "(decide) Space-separated synonyms and related terms for search enrichment",
					},
					"task_context": map[string]string{
						"type":        "string",
						"description": "(decide) Optional task/context text sanitized into the DecisionRecord ID filename",
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
					"context": map[string]string{"type": "string", "description": "Optional context name"},
					"mode":    map[string]string{"type": "string", "description": "(decide) tactical, standard (default), deep"},
				},
				"required": []string{"action"},
			},
		})
		tools = append(tools, Tool{
			Name:        "haft_refresh",
			Description: "Manage artifact lifecycle — detect stale items, extend validity, archive, replace, or find note-decision overlaps. Works on ALL artifact types. Actions: 'scan' finds expired and evidence-degraded artifacts, 'waive' extends validity, 'reopen' starts new problem cycle from a decision, 'supersede' replaces one artifact with another, 'deprecate' archives as no longer relevant, 'reconcile' finds notes that overlap with decisions.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"scan", "waive", "reopen", "supersede", "deprecate", "reconcile"},
						"description": "scan=find stale/degraded, waive=extend validity, reopen=new problem cycle, supersede=replace, deprecate=archive, reconcile=find note-decision overlaps",
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
				},
				"required": []string{"action"},
			},
		})

		tools = append(tools, Tool{
			Name:        "haft_query",
			Description: "Search past decisions, check status, find related artifacts, render audience projections, list all artifacts by kind, or show module coverage. Actions: 'search' does FTS5 search, 'status' shows compact dashboard, 'related' finds decisions affecting a file, 'projection' renders deterministic audience views, 'list' shows all artifacts of a given kind, 'coverage' shows module-level decision coverage.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []interface{}{"search", "status", "board", "related", "projection", "list", "coverage", "fpf"},
						"description": "search=FTS5 keyword search, status=compact dashboard, board=rich health dashboard (overview/decisions/problems/coverage/evidence/full), related=by file path, projection=audience-specific artifact view, list=all artifacts by kind, coverage=module-level decision coverage, fpf=search FPF methodology spec",
					},
					"query": map[string]string{
						"type":        "string",
						"description": "(search, fpf) Search terms",
					},
					"kind": map[string]string{
						"type":        "string",
						"description": "(list) Artifact kind: Note, ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, RefreshReport",
					},
					"file": map[string]string{
						"type":        "string",
						"description": "(related) File path to find linked decisions",
					},
					"context": map[string]string{
						"type":        "string",
						"description": "Optional context filter",
					},
					"view": map[string]string{
						"type":        "string",
						"description": "(projection) engineer | manager | audit | compare | delegated-agent | change-rationale",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": fmt.Sprintf("(search) Max results, default 20; (fpf) max results, default %d", DefaultSpecSearchLimit),
					},
					"full": map[string]interface{}{
						"type":        "boolean",
						"description": "(fpf) Show full section content instead of snippets",
					},
					"explain": map[string]interface{}{
						"type":        "boolean",
						"description": "(fpf) Show why each result matched",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "(fpf) Experimental retrieval mode; currently supports tree",
					},
				},
				"required": []string{"action"},
			},
		})

		tools = append(tools, haftCommissionTool())
	}

	s.sendResult(req.ID, map[string]interface{}{
		"tools": tools,
	})
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
