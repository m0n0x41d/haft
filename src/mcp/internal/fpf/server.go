package fpf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/m0n0x41d/quint-code/logger"
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

type Server struct {
	tools *Tools
}

func NewServer(t *Tools) *Server {
	return &Server{tools: t}
}

func (s *Server) Start() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

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
}

func (s *Server) send(resp JSONRPCResponse) {
	bytes, err := json.Marshal(resp)
	if err != nil {
		logger.Error().Err(err).Msg("failed to marshal JSON-RPC response")
		return
	}
	fmt.Printf("%s\n", string(bytes))
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
	s.sendResult(req.ID, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    "quint-code",
			"version": "4.0.0",
		},
	})
}

func (s *Server) handleToolsList(req JSONRPCRequest) {
	tools := []Tool{
		{
			Name:        "quint_internalize",
			Description: "Unified entry point for FPF sessions. Initializes project if needed, checks for stale context, loads knowledge state, surfaces decaying evidence, and provides phase-appropriate guidance. Call this at the start of every session.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "quint_search",
			Description: "Full-text search across the knowledge base. Search holons and evidence by keywords.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":                 map[string]string{"type": "string", "description": "Search terms"},
					"scope":                 map[string]string{"type": "string", "description": "Scope: 'holons', 'evidence', 'all' (default: 'all')"},
					"layer_filter":          map[string]string{"type": "string", "description": "Filter by layer: 'L0', 'L1', 'L2', or empty for all"},
					"status_filter":         map[string]string{"type": "string", "description": "Filter decisions by status: 'open', 'implemented', 'abandoned', 'superseded'"},
					"affected_scope_filter": map[string]string{"type": "string", "description": "Filter DRRs by affected file path (matches against affected_scope patterns)"},
					"limit":                 map[string]interface{}{"type": "integer", "description": "Max results (default: 10, max: 50)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "quint_resolve",
			Description: "Resolve a decision (DRR) by recording its outcome: implemented, abandoned, or superseded.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"decision_id":       map[string]string{"type": "string", "description": "ID of the decision holon to resolve"},
					"resolution":        map[string]interface{}{"type": "string", "enum": []interface{}{"implemented", "abandoned", "superseded"}, "description": "Resolution type"},
					"reference":         map[string]string{"type": "string", "description": "Implementation reference (required for 'implemented'): commit:SHA, pr:NUM, file:PATH"},
					"superseded_by":     map[string]string{"type": "string", "description": "ID of replacing decision (required for 'superseded')"},
					"notes":             map[string]string{"type": "string", "description": "Explanation or description (required for 'abandoned')"},
					"valid_until":       map[string]string{"type": "string", "description": "Optional: when to re-verify implementation (RFC3339 format)"},
					"criteria_verified": map[string]interface{}{"type": "boolean", "description": "Set to true to confirm acceptance criteria are verified (required when DRR has acceptance_criteria)", "default": false},
				},
				"required": []string{"decision_id", "resolution"},
			},
		},
		{
			Name:        "quint_implement",
			Description: "Transform a finalized DRR into an implementation directive. Returns a structured prompt that programs your internal planning capabilities with invariants, constraints, and acceptance criteria from the decision and its dependencies.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"decision_id": map[string]string{"type": "string", "description": "ID of the DRR to implement"},
				},
				"required": []string{"decision_id"},
			},
		},
		{
			Name:        "quint_link",
			Description: "Add dependency between existing holons. Use after creating a hypothesis to link it to existing decisions/hypotheses. Creates ComponentOf (system) or ConstituentOf (episteme) relation. WLNK applies after linking.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_id": map[string]string{"type": "string", "description": "ID of the holon that DEPENDS on target"},
					"target_id": map[string]string{"type": "string", "description": "ID of the holon being depended upon"},
					"congruence_level": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"maximum":     3,
						"default":     3,
						"description": "CL3=same context, CL2=similar, CL1=different",
					},
				},
				"required": []string{"source_id", "target_id"},
			},
		},
		{
			Name:        "quint_propose",
			Description: "Propose a new hypothesis (L0). IMPORTANT: Consider depends_on for dependencies and decision_context for grouping alternatives.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":     map[string]string{"type": "string", "description": "Title"},
					"content":   map[string]string{"type": "string", "description": "Description"},
					"scope":     map[string]string{"type": "string", "description": "Scope (G) - where this hypothesis applies"},
					"kind":      map[string]interface{}{"type": "string", "enum": []interface{}{"system", "episteme"}, "description": "system=code/architecture, episteme=process/methodology"},
					"rationale": map[string]string{"type": "string", "description": "JSON: {anomaly, approach, alternatives_rejected}"},
					"decision_context": map[string]string{
						"type":        "string",
						"description": "Parent decision ID to GROUP competing alternatives. Does NOT affect R_eff. Use when multiple hypotheses solve the same problem. Example: 'caching-decision' groups 'redis-caching' and 'cdn-edge'. Creates MemberOf relation.",
					},
					"depends_on": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "IDs of holons this hypothesis REQUIRES to work. CRITICAL: Affects R_eff via WLNK - if dependency has low R, this inherits that ceiling. Use when: (1) builds on another hypothesis, (2) needs another to function, (3) dependency failure invalidates this. Leave empty for independent hypotheses. Creates ComponentOf/ConstituentOf.",
					},
					"dependency_cl": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"maximum":     3,
						"default":     3,
						"description": "Congruence level for dependencies. CL3=same context (no penalty), CL2=similar (10% penalty), CL1=different (30% penalty).",
					},
				},
				"required": []string{"title", "content", "scope", "kind", "rationale"},
			},
		},
		{
			Name:        "quint_verify",
			Description: "Record verification results (L0 -> L1).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hypothesis_id": map[string]string{"type": "string"},
					"verify_json": map[string]string{
						"type": "string",
						"description": `JSON object with structured verification result:
{
  "type_check": {"verdict": "PASS|FAIL", "evidence": ["ref1", "ref2"], "reasoning": "..."},
  "constraint_check": {"verdict": "PASS|FAIL", "evidence": [...], "reasoning": "..."},
  "logic_check": {"verdict": "PASS|FAIL", "evidence": [...], "reasoning": "..."},
  "overall_verdict": "PASS|FAIL",
  "risks": ["optional risk notes"]
}
ALL verdicts (PASS and FAIL) require at least one evidence reference.`,
					},
					"carrier_files": map[string]string{"type": "string", "description": "Comma-separated file paths (relative to repo root) that this verification is based on. These files will be tracked for changes - if they change, the evidence becomes stale. Extract from hypothesis scope or files you examined. Example: 'src/cache.py,src/api/routes.py'"},
				},
				"required": []string{"hypothesis_id", "verify_json"},
			},
		},
		{
			Name:        "quint_test",
			Description: "Record validation results (L1 -> L2).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hypothesis_id": map[string]string{"type": "string"},
					"test_type":     map[string]string{"type": "string", "description": "internal or research"},
					"test_json": map[string]string{
						"type": "string",
						"description": `JSON object with structured test result:
{
  "observations": [
    {"description": "what was observed", "evidence": ["ref1", "ref2"], "supports": true/false}
  ],
  "overall_verdict": "PASS|FAIL",
  "reasoning": "why this verdict"
}
Each observation requires at least one evidence reference.`,
					},
					"carrier_files": map[string]string{"type": "string", "description": "Comma-separated file paths (relative to repo root) that were tested. These files will be tracked for changes - if they change, the evidence becomes stale. For internal tests: files covered by tests. For external research: leave empty or use source URL. Example: 'src/database/repository.py,src/database/queries.py'"},
				},
				"required": []string{"hypothesis_id", "test_type", "test_json"},
			},
		},
		{
			Name:        "quint_audit",
			Description: "Record audit/trust score (R_eff).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hypothesis_id": map[string]string{"type": "string"},
					"risks":         map[string]string{"type": "string", "description": "Risk analysis"},
				},
				"required": []string{"hypothesis_id", "risks"},
			},
		},
		{
			Name:        "quint_decide",
			Description: "Finalize decision (DRR).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":     map[string]string{"type": "string"},
					"winner_id": map[string]string{"type": "string"},
					"rejected_ids": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "IDs of rejected L2 alternatives",
					},
					"context":         map[string]string{"type": "string"},
					"decision":        map[string]string{"type": "string"},
					"rationale":       map[string]string{"type": "string"},
					"consequences":    map[string]string{"type": "string"},
					"characteristics": map[string]string{"type": "string"},
					"contract": map[string]interface{}{
						"type":        "string",
						"description": "JSON object with implementation contract: {invariants: [], anti_patterns: [], acceptance_criteria: [], affected_scope: []}",
					},
				},
				"required": []string{"title", "winner_id", "context", "decision", "rationale", "consequences"},
			},
		},
		{
			Name:        "quint_audit_tree",
			Description: "Visualize the assurance tree for a holon, showing R scores, dependencies, and CL penalties.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"holon_id": map[string]string{"type": "string", "description": "ID of the holon to audit"},
				},
				"required": []string{"holon_id"},
			},
		},
		{
			Name:        "quint_calculate_r",
			Description: "Calculate the effective reliability (R_eff) for a holon with detailed breakdown.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"holon_id": map[string]string{"type": "string", "description": "ID of the holon"},
				},
				"required": []string{"holon_id"},
			},
		},
		{
			Name:        "quint_reset",
			Description: "Reset FPF cycle to IDLE state. Records session end in audit log without creating DRR. Use when ending a session without making a decision.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"reason": map[string]string{"type": "string", "description": "Why the cycle is being reset (e.g., 'pivoting to different problem', 'session complete')"},
				},
				"required": []string{},
			},
		},
		{
			Name:        "quint_compact",
			Description: "Compact old archived holons to reduce database size. Removes evidence, characteristics, and detailed content while preserving holon metadata and decision links for audit trail.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"mode":           map[string]string{"type": "string", "description": "Operation mode: 'preview' (default) shows what would be compacted, 'execute' performs compaction"},
					"retention_days": map[string]string{"type": "integer", "description": "Days after decision resolution before a holon is eligible for compaction (default: 90)"},
				},
				"required": []string{},
			},
		},
	}

	s.sendResult(req.ID, map[string]interface{}{
		"tools": tools,
	})
}

func (s *Server) handleToolsCall(req JSONRPCRequest) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32700, "Invalid params")
		return
	}

	arg := func(k string) string {
		if v, ok := params.Arguments[k].(string); ok {
			return v
		}
		return ""
	}

	args := make(map[string]string)
	for k, v := range params.Arguments {
		if s, ok := v.(string); ok {
			args[k] = s
		}
	}

	if precondErr := s.tools.CheckPreconditions(params.Name, args); precondErr != nil {
		s.tools.AuditLog(params.Name, "precondition_failed", "agent", "", "BLOCKED", args, precondErr.Error())
		s.sendResult(req.ID, CallToolResult{
			Content: []ContentItem{{Type: "text", Text: precondErr.Error()}},
			IsError: true,
		})
		return
	}

	var output string
	var err error

	switch params.Name {
	case "quint_internalize":
		output, err = s.tools.Internalize()

	case "quint_search":
		limit := 10
		if l, ok := params.Arguments["limit"].(float64); ok {
			limit = int(l)
		}
		output, err = s.tools.Search(arg("query"), arg("scope"), arg("layer_filter"), arg("status_filter"), arg("affected_scope_filter"), limit)

	case "quint_resolve":
		criteriaVerified := false
		if cv, ok := params.Arguments["criteria_verified"].(bool); ok {
			criteriaVerified = cv
		}
		input := ResolveInput{
			DecisionID:       arg("decision_id"),
			Resolution:       arg("resolution"),
			Reference:        arg("reference"),
			SupersededBy:     arg("superseded_by"),
			Notes:            arg("notes"),
			ValidUntil:       arg("valid_until"),
			CriteriaVerified: criteriaVerified,
		}
		output, err = s.tools.Resolve(input)

	case "quint_implement":
		output, err = s.tools.Implement(arg("decision_id"))

	case "quint_link":
		cl := 3
		if clVal, ok := params.Arguments["congruence_level"].(float64); ok {
			cl = int(clVal)
		}
		output, err = s.tools.LinkHolons(arg("source_id"), arg("target_id"), cl)

	case "quint_propose":
		decisionContext := arg("decision_context")
		var dependsOn []string
		if deps, ok := params.Arguments["depends_on"].([]interface{}); ok {
			for _, d := range deps {
				if s, ok := d.(string); ok {
					dependsOn = append(dependsOn, s)
				}
			}
		}
		dependencyCL := 3
		if cl, ok := params.Arguments["dependency_cl"].(float64); ok {
			dependencyCL = int(cl)
		}
		output, err = s.tools.ProposeHypothesis(arg("title"), arg("content"), arg("scope"), arg("kind"), arg("rationale"), decisionContext, dependsOn, dependencyCL)

	case "quint_verify":
		output, err = s.tools.VerifyHypothesis(arg("hypothesis_id"), arg("verify_json"), arg("carrier_files"))

	case "quint_test":
		output, err = s.tools.ValidateHypothesis(arg("hypothesis_id"), arg("test_type"), arg("test_json"), arg("carrier_files"))

	case "quint_audit":
		output, err = s.tools.AuditEvidence(arg("hypothesis_id"), arg("risks"))

	case "quint_decide":
		var rejectedIDs []string
		if rids, ok := params.Arguments["rejected_ids"].([]interface{}); ok {
			for _, r := range rids {
				if s, ok := r.(string); ok {
					rejectedIDs = append(rejectedIDs, s)
				}
			}
		}
		output, err = s.tools.FinalizeDecision(arg("title"), arg("winner_id"), rejectedIDs, arg("context"), arg("decision"), arg("rationale"), arg("consequences"), arg("characteristics"), arg("contract"))

	case "quint_audit_tree":
		output, err = s.tools.VisualizeAudit(arg("holon_id"))

	case "quint_calculate_r":
		output, err = s.tools.CalculateR(arg("holon_id"))

	case "quint_reset":
		abandonAll := false
		if aa, ok := params.Arguments["abandon_all"].(bool); ok {
			abandonAll = aa
		}
		output, err = s.tools.ResetCycle(arg("reason"), arg("context_id"), abandonAll)

	case "quint_compact":
		retentionDays := int64(90)
		if rd, ok := params.Arguments["retention_days"].(float64); ok {
			retentionDays = int64(rd)
		}
		output, err = s.tools.Compact(arg("mode"), retentionDays)

	default:
		err = fmt.Errorf("unknown tool: %s", params.Name)
	}

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
