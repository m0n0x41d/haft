package fpf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
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
		fmt.Fprintf(os.Stderr, "Error: failed to marshal JSON-RPC response: %v\n", err)
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
					"query":         map[string]string{"type": "string", "description": "Search terms"},
					"scope":         map[string]string{"type": "string", "description": "Scope: 'holons', 'evidence', 'all' (default: 'all')"},
					"layer_filter":  map[string]string{"type": "string", "description": "Filter by layer: 'L0', 'L1', 'L2', or empty for all"},
					"status_filter": map[string]string{"type": "string", "description": "Filter decisions by status: 'open', 'implemented', 'abandoned', 'superseded'"},
					"limit":         map[string]interface{}{"type": "integer", "description": "Max results (default: 10, max: 50)"},
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
					"decision_id":   map[string]string{"type": "string", "description": "ID of the decision holon to resolve"},
					"resolution":    map[string]interface{}{"type": "string", "enum": []interface{}{"implemented", "abandoned", "superseded"}, "description": "Resolution type"},
					"reference":     map[string]string{"type": "string", "description": "Implementation reference (required for 'implemented'): commit:SHA, pr:NUM, file:PATH"},
					"superseded_by": map[string]string{"type": "string", "description": "ID of replacing decision (required for 'superseded')"},
					"notes":         map[string]string{"type": "string", "description": "Explanation or description (required for 'abandoned')"},
					"valid_until":   map[string]string{"type": "string", "description": "Optional: when to re-verify implementation (RFC3339 format)"},
				},
				"required": []string{"decision_id", "resolution"},
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
					"checks_json":   map[string]string{"type": "string", "description": "JSON of checks"},
					"verdict":       map[string]interface{}{"type": "string", "enum": []interface{}{"PASS", "FAIL", "REFINE"}},
				},
				"required": []string{"hypothesis_id", "checks_json", "verdict"},
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
					"result":        map[string]string{"type": "string", "description": "Test output/findings"},
					"verdict":       map[string]interface{}{"type": "string", "enum": []interface{}{"PASS", "FAIL", "REFINE"}},
				},
				"required": []string{"hypothesis_id", "test_type", "result", "verdict"},
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
		output, err = s.tools.Search(arg("query"), arg("scope"), arg("layer_filter"), arg("status_filter"), limit)

	case "quint_resolve":
		input := ResolveInput{
			DecisionID:   arg("decision_id"),
			Resolution:   arg("resolution"),
			Reference:    arg("reference"),
			SupersededBy: arg("superseded_by"),
			Notes:        arg("notes"),
			ValidUntil:   arg("valid_until"),
		}
		output, err = s.tools.Resolve(input)

	case "quint_propose":
		s.tools.FSM.State.Phase = PhaseAbduction
		if saveErr := s.tools.FSM.SaveState("default"); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", saveErr)
		}
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
		s.tools.FSM.State.Phase = PhaseDeduction
		if saveErr := s.tools.FSM.SaveState("default"); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", saveErr)
		}
		output, err = s.tools.VerifyHypothesis(arg("hypothesis_id"), arg("checks_json"), arg("verdict"))

	case "quint_test":
		s.tools.FSM.State.Phase = PhaseInduction
		if saveErr := s.tools.FSM.SaveState("default"); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", saveErr)
		}

		assLevel := "L2"
		if arg("verdict") != "PASS" {
			assLevel = "L1"
		}

		validUntil := computeValidUntil(arg("test_type"))
		output, err = s.tools.ManageEvidence(PhaseInduction, "add", arg("hypothesis_id"), arg("test_type"), arg("result"), arg("verdict"), assLevel, "test-runner", validUntil)

	case "quint_audit":
		output, err = s.tools.AuditEvidence(arg("hypothesis_id"), arg("risks"))

	case "quint_decide":
		s.tools.FSM.State.Phase = PhaseDecision
		var rejectedIDs []string
		if rids, ok := params.Arguments["rejected_ids"].([]interface{}); ok {
			for _, r := range rids {
				if s, ok := r.(string); ok {
					rejectedIDs = append(rejectedIDs, s)
				}
			}
		}
		output, err = s.tools.FinalizeDecision(arg("title"), arg("winner_id"), rejectedIDs, arg("context"), arg("decision"), arg("rationale"), arg("consequences"), arg("characteristics"))
		if err == nil {
			s.tools.FSM.State.Phase = PhaseIdle
			if saveErr := s.tools.FSM.SaveState("default"); saveErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", saveErr)
			}
		}

	case "quint_audit_tree":
		output, err = s.tools.VisualizeAudit(arg("holon_id"))

	case "quint_calculate_r":
		output, err = s.tools.CalculateR(arg("holon_id"))

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

// computeValidUntil returns evidence validity period based on test type.
// Internal tests (code/unit tests) are tied to codebase → 90 days.
// External research (docs/APIs) changes faster → 60 days.
func computeValidUntil(testType string) string {
	var days int
	switch testType {
	case "internal":
		days = 90
	case "external":
		days = 60
	default:
		days = 90
	}
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}
