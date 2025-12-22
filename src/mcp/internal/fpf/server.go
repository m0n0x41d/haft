package fpf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
			Name:        "quint_status",
			Description: "Get current FPF phase and context.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "quint_init",
			Description: "Initialize FPF project structure.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "quint_record_context",
			Description: "Record the Bounded Context (A.1.1).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"vocabulary": map[string]string{"type": "string", "description": "Key terms"},
					"invariants": map[string]string{"type": "string", "description": "System rules"},
				},
				"required": []string{"vocabulary", "invariants"},
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
			Name:        "quint_actualize",
			Description: "Reconcile the project's FPF state with recent repository changes.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
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
			Name:        "quint_check_decay",
			Description: "Check evidence freshness and manage stale decisions. Without parameters: shows freshness report. With deprecate: downgrades hypothesis. With waive: records temporary risk acceptance.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"deprecate": map[string]string{
						"type":        "string",
						"description": "Hypothesis ID to deprecate (L2→L1 or L1→L0)",
					},
					"waive_id": map[string]string{
						"type":        "string",
						"description": "Evidence ID to waive",
					},
					"waive_until": map[string]string{
						"type":        "string",
						"description": "ISO date until which waiver is valid (required with waive_id)",
					},
					"waive_rationale": map[string]string{
						"type":        "string",
						"description": "Reason for accepting stale evidence (required with waive_id)",
					},
				},
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
	case "quint_status":
		st := s.tools.FSM.State.Phase
		output = string(st)

	case "quint_init":
		res := s.tools.InitProject()
		if res != nil {
			err = res
		} else {
			s.tools.FSM.State.Phase = PhaseAbduction
			if saveErr := s.tools.FSM.SaveState("default"); saveErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", saveErr)
			}
			output = "Initialized. Phase: ABDUCTION"
		}

	case "quint_actualize":
		output, err = s.tools.Actualize()

	case "quint_record_context":
		output, err = s.tools.RecordContext(arg("vocabulary"), arg("invariants"))

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

		output, err = s.tools.ManageEvidence(PhaseInduction, "add", arg("hypothesis_id"), arg("test_type"), arg("result"), arg("verdict"), assLevel, "test-runner", "")

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

	case "quint_check_decay":
		output, err = s.tools.CheckDecay(arg("deprecate"), arg("waive_id"), arg("waive_until"), arg("waive_rationale"))

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
